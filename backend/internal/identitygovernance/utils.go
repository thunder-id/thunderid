// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernance

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/thunder-id/thunderid/internal/serverconfig"
	sysconfig "github.com/thunder-id/thunderid/internal/system/config"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// isLockable reports whether failures at a method are counted and may form a lock. An unregistered
// method is not lockable.
func isLockable(method providers.AuthenticationMethod) bool {
	return lockableMethods[method]
}

// isSuspended reports a suspension by the organization, by the identity's owner, or carried from the identity it
// depends on.
func isSuspended(state providers.EntityState, accessState AccessState) bool {
	return isSuspendedByOrganization(state, accessState) || accessState.OwnerSuspend != nil ||
		accessState.CascadeSuspend != nil
}

// isSuspendedByOrganization reports the organization's suspension, recorded in either the state column or
// the access-state document.
func isSuspendedByOrganization(state providers.EntityState, accessState AccessState) bool {
	return state == providers.EntityStateSuspended || accessState.Suspend != nil
}

type ownerApplierContextKey struct{}

// WithOwnerApplier marks a suspend or unsuspend call as made by the owner of the identity, such as the
// owner of a personal agent, rather than by the organization. The owner's suspension is held apart from
// the organization's: each party places and lifts only its own.
func WithOwnerApplier(ctx context.Context) context.Context {
	return context.WithValue(ctx, ownerApplierContextKey{}, true)
}

// suspensionApplier returns who makes a suspend or unsuspend call: the owner when the call is marked
// with WithOwnerApplier, otherwise the organization.
func suspensionApplier(ctx context.Context) providers.SuspensionAppliedBy {
	if owner, _ := ctx.Value(ownerApplierContextKey{}).(bool); owner {
		return providers.SuspensionAppliedByOwner
	}
	return providers.SuspensionAppliedByOrganization
}

// hasLockEntries reports whether the entity has any recorded lock entry, live or lapsed.
func hasLockEntries(entity GovernedEntity) bool {
	return entity.AccessState.Lock.Entity != nil || len(entity.AccessState.Lock.AuthenticationMethods) > 0
}

// isLockLive reports whether a stored unlockAt is permanent or still in the future.
func isLockLive(unlockAt string, now time.Time) bool {
	if unlockAt == "" {
		return false
	}
	if unlockAt == permanentUnlockAt {
		return true
	}
	return now.UTC().Format(time.RFC3339) < unlockAt
}

// lockFor reports whether anything holds method, ignoring configured granularity: a live entity-wide
// lock holds every method, else a live lock on method holds it. The first return is true when the
// entity-wide entry holds.
func lockFor(a AccessState, method providers.AuthenticationMethod, now time.Time) (bool, bool) {
	if a.Lock.Entity != nil && isLockLive(a.Lock.Entity.UnlockAt, now) {
		return true, true
	}
	entry, recorded := a.Lock.AuthenticationMethods[method]
	if recorded && isLockLive(entry.UnlockAt, now) {
		return false, true
	}
	return false, false
}

// shouldFormLock reports whether the observed counters reach the threshold or the consecutive-failure
// ceiling. No lock forms while one is live, so failures during a lock cannot escalate it.
func shouldFormLock(policy lockoutPolicy, observed LockEntry, now time.Time) bool {
	if !policy.enabled || policy.threshold <= 0 || len(policy.lockDurations) == 0 {
		return false
	}
	if isLockLive(observed.UnlockAt, now) {
		return false
	}
	return observed.FailureCount >= policy.threshold || reachesCeiling(policy, observed)
}

// reachesCeiling reports whether the failures since the entry was last cleared reach the policy's
// consecutive-failure ceiling.
func reachesCeiling(policy lockoutPolicy, observed LockEntry) bool {
	return policy.maxConsecutiveFailures > 0 && observed.ConsecutiveFailures >= policy.maxConsecutiveFailures
}

// nextLockEpisode returns the lock count and unlockAt for a lock about to form. A zero duration means
// a permanent lock. Call it only after shouldFormLock returns true, which guarantees a duration.
func nextLockEpisode(policy lockoutPolicy, observed LockEntry, now time.Time) LockEpisode {
	lockCount := nextLockCount(policy, observed, now)

	// lockCount is one-based; the last duration repeats.
	index := lockCount - 1
	if index >= len(policy.lockDurations) {
		index = len(policy.lockDurations) - 1
	}
	duration := policy.lockDurations[index]
	// At the ceiling the lock stays until a recovery or an unlock clears the entry.
	if reachesCeiling(policy, observed) {
		duration = 0
	}

	unlockAt := permanentUnlockAt
	if duration > 0 {
		unlockAt = now.UTC().Add(duration).Format(time.RFC3339)
	}

	return LockEpisode{LockCount: lockCount, UnlockAt: unlockAt, Duration: duration}
}

// nextLockCount returns the consecutive lockout number. It resets to 1 once lockDecay has passed
// since the previous lock ended.
func nextLockCount(policy lockoutPolicy, observed LockEntry, now time.Time) int {
	if observed.UnlockAt == "" {
		return 1
	}
	// A permanent lock never decays.
	if observed.UnlockAt == permanentUnlockAt {
		return observed.LockCount + 1
	}
	if policy.lockDecay > 0 {
		decayedBy := now.UTC().Add(-policy.lockDecay).Format(time.RFC3339)
		if observed.UnlockAt < decayedBy {
			return 1
		}
	}
	if observed.LockCount <= 0 {
		return 1
	}
	return observed.LockCount + 1
}

// holdsAuthenticationPlane reports whether an automatic lock is enforced at this plane for a method.
func holdsAuthenticationPlane(policy lockoutPolicy, plane providers.AccessPlane, method providers.AuthenticationMethod,
	postSuspension bool) bool {
	if !policy.enabled && !postSuspension {
		// Disabled lockout ignores failure locks. The post-suspension hold still applies, since
		// Unsuspend writes one regardless of the policy.
		return false
	}
	return lockAppliesAt(plane, method)
}

// lockAppliesAt reports whether an automatic lock applies to a method at a plane. A failure is counted
// only where the lock applies, so a count never locks a method where the lock cannot stop guessing.
func lockAppliesAt(plane providers.AccessPlane, method providers.AuthenticationMethod) bool {
	switch plane {
	case providers.AccessPlaneAuthentication, providers.AccessPlaneDispatch:
		return true
	case providers.AccessPlaneRecovery:
		return heldAtRecovery[method]
	default:
		// Locks never apply at the session, issuance or application planes, so failed logins cannot
		// end sessions. Unknown planes do not enforce locks.
		return false
	}
}

// admit returns the decision that allows access.
func admit() providers.AccessDecision {
	return providers.AccessDecision{Admitted: true}
}

// refuse returns the decision that withholds access, naming the hold only when the mode discloses it.
func refuse(mode disclosureMode, reason providers.AccessRefusalReason) providers.AccessDecision {
	if mode != discloseAlways {
		return providers.AccessDecision{}
	}
	return providers.AccessDecision{Reason: reason}
}

// isOperatorNoteTooLong reports whether a suspension's operator note exceeds the limit.
func isOperatorNoteTooLong(note string) bool {
	return utf8.RuneCountInString(note) > maxOperatorNoteLength
}

// failureClass maps a lifecycle error to its failure class, without store detail.
func failureClass(svcErr *tidcommon.ServiceError) lifecycleFailure {
	switch svcErr.Code {
	case tidcommon.ErrorUnauthorized.Code:
		return failureNotAuthorized
	case ErrorNotApplicable.Code:
		return failureNotApplicable
	case ErrorMissingSubject.Code:
		return failureNoSubject
	case ErrorEntityNotFound.Code:
		return failureNotFound
	case ErrorOperatorNoteTooLong.Code:
		return failureNoteTooLong
	default:
		return failureOperationFailed
	}
}

// effectiveMethodFor returns the access-state key a method maps to under the given granularity.
func effectiveMethodFor(g granularity, method providers.AuthenticationMethod) providers.AuthenticationMethod {
	if g == granularityEntity {
		return AuthenticationMethodEntity
	}
	return method
}

// toDuration converts seconds to a duration, clamped to [0, maxDurationSeconds].
func toDuration(seconds int) time.Duration {
	return time.Duration(min(max(seconds, 0), maxDurationSeconds)) * time.Second
}

// activityPolicyFor reads one category's activity settings. Recording is off unless enabled.
func activityPolicyFor(category accountAccessCategory) activityPolicy {
	return activityPolicy{
		record: sysutils.DerefOr(category.RecordLastLogin, false),
		// A negative resolution clamps to zero (write every time).
		resolution: toDuration(sysutils.DerefOr(category.LastLoginResolutionSeconds,
			defaultLastLoginResolutionSeconds)),
	}
}

// policyFor builds the policy a category applies at a method.
func policyFor(category accountAccessCategory, g granularity, method providers.AuthenticationMethod) lockoutPolicy {
	effective := effectiveMethodFor(g, method)

	// A method block overlays the category default; an unnamed method gets the default.
	var merged accountAccessPolicy
	if methodPolicy, ok := category.Methods[string(effective)]; ok {
		merged = *overlayAccountAccessPolicy(category.Default, &methodPolicy)
	} else if category.Default != nil {
		merged = *category.Default
	}

	policy := lockoutPolicy{
		enabled:                sysutils.DerefOr(merged.Enabled, true),
		threshold:              sysutils.DerefOr(merged.Threshold, 0),
		failureWindow:          toDuration(sysutils.DerefOr(merged.FailureWindowSeconds, 0)),
		lockDurations:          toDurations(merged.LockDurationsSeconds),
		lockDecay:              toDuration(sysutils.DerefOr(merged.LockDecaySeconds, 0)),
		maxConsecutiveFailures: sysutils.DerefOr(merged.MaxConsecutiveFailures, 0),
		granularity:            g,
		discloseAccountHold:    defaultDisclosureMode,
	}

	// A policy that cannot form a lock (no threshold, durations or window) is disabled.
	if policy.threshold <= 0 || len(policy.lockDurations) == 0 || merged.FailureWindowSeconds == nil {
		policy.enabled = false
	}

	return policy
}

// categoryConfig returns the configuration block for an entity category, or an empty block (a
// disabled policy) if none is set.
func categoryConfig(cfg accountAccessValue, category providers.EntityCategory) accountAccessCategory {
	var block *accountAccessCategory
	switch category {
	case providers.EntityCategoryUser:
		block = cfg.User
	case providers.EntityCategoryAgent:
		block = cfg.Agent
	}
	if block == nil {
		return accountAccessCategory{}
	}
	return *block
}

// parseGranularity returns the granularity, defaulting an unknown value to per-method.
func parseGranularity(value granularity) granularity {
	if value == granularityEntity {
		return granularityEntity
	}
	return granularityAuthenticationMethod
}

// toDurations converts seconds to durations, keeping zero (permanent lock) and dropping negatives.
func toDurations(seconds []int) []time.Duration {
	if len(seconds) == 0 {
		return nil
	}
	durations := make([]time.Duration, 0, len(seconds))
	for _, s := range seconds {
		if s < 0 {
			continue
		}
		durations = append(durations, toDuration(s))
	}
	return durations
}

// lockEnforcementFor resolves which of a category's recorded locks are enforced.
func lockEnforcementFor(cfg accountAccessValue, category providers.EntityCategory) lockEnforcement {
	block := categoryConfig(cfg, category)
	g := parseGranularity(sysutils.DerefOr(block.LockGranularity, ""))

	unenforced := make(map[providers.AuthenticationMethod]struct{}, len(AllAuthenticationMethods))
	for _, method := range AllAuthenticationMethods {
		// Under entity granularity only the entity entry is enforced. The post-suspension hold is
		// exempt from this in reportFor.
		if (g == granularityEntity && method != AuthenticationMethodEntity) || !policyFor(block, g, method).enabled {
			unenforced[method] = struct{}{}
		}
	}
	return lockEnforcement{unenforced: unenforced}
}

// accessReadFor derives what a management read reports for one entity at now. An unreadable document
// reports suspended.
func accessReadFor(entity *providers.Entity, enforcement lockEnforcement, now time.Time) AccessRead {
	var stored RuntimeAttributes
	if len(entity.RuntimeAttributes) > 0 {
		if err := json.Unmarshal(entity.RuntimeAttributes, &stored); err != nil {
			return AccessRead{Status: providers.AccessStatusReport{
				Value: providers.AccessStatusSuspended,
				Details: &providers.AccessStatusDetails{Reason: providers.AccessHoldReasonAdministrativeSuspension,
					AppliedBy: []providers.SuspensionAppliedBy{providers.SuspensionAppliedByOrganization}},
			}}
		}
	}
	return AccessRead{
		Status:      reportFor(entity.State, stored.AccessState, now, enforcement),
		LastLoginAt: stored.AccessState.LastLoginAt,
	}
}

// reportFor derives the status a management read reports, and what produced it. Precedence is
// SUSPENDED, then LOCKED, then ACTIVE. Method locks that enforcement no longer applies are skipped.
func reportFor(state providers.EntityState, accessState AccessState, now time.Time,
	enforcement lockEnforcement) providers.AccessStatusReport {
	if isSuspended(state, accessState) {
		return suspendedReport(state, accessState)
	}

	if entry := accessState.Lock.Entity; entry != nil && isLockLive(entry.UnlockAt, now) &&
		(entry.Reason == lockReasonPostSuspension || enforcement.enforces(AuthenticationMethodEntity)) {
		return lockedReport(entityHoldReason(entry.Reason), []providers.AccessLockedMethod{{
			Method:    string(AuthenticationMethodEntity),
			ExpiresAt: expiryOf(entry.UnlockAt),
		}})
	}

	if locked := liveAuthenticationMethodLocks(accessState, now, enforcement); len(locked) > 0 {
		return lockedReport(providers.AccessHoldReasonFailedAttempts, locked)
	}

	return providers.AccessStatusReport{Value: providers.AccessStatusActive}
}

// suspendedReport builds the SUSPENDED report with any recorded details. It names every party holding the
// identity suspended; the reason, time and note are the organization's when it holds one, otherwise the
// owner's, otherwise the cascaded one's.
func suspendedReport(state providers.EntityState, accessState AccessState) providers.AccessStatusReport {
	details := providers.AccessStatusDetails{}
	if isSuspendedByOrganization(state, accessState) {
		details.Reason = providers.AccessHoldReasonAdministrativeSuspension
		details.AppliedBy = append(details.AppliedBy, providers.SuspensionAppliedByOrganization)
		if suspend := accessState.Suspend; suspend != nil {
			details.Since = suspend.SuspendedAt
			// Reported only to callers authorized to read the record; never on a refusal.
			details.OperatorNote = suspend.OperatorNote
		}
	}
	if owner := accessState.OwnerSuspend; owner != nil {
		details.AppliedBy = append(details.AppliedBy, providers.SuspensionAppliedByOwner)
		if details.Reason == "" {
			details.Reason = providers.AccessHoldReasonOwnerSuspension
			details.Since = owner.SuspendedAt
		}
	}
	if cascade := accessState.CascadeSuspend; cascade != nil {
		details.AppliedBy = append(details.AppliedBy, providers.SuspensionAppliedByCascade)
		if details.Reason == "" {
			details.Reason = providers.AccessHoldReasonCascadedSuspension
			details.Since = cascade.SuspendedAt
		}
	}
	return providers.AccessStatusReport{Value: providers.AccessStatusSuspended, Details: &details}
}

// lockedReport builds the LOCKED report.
func lockedReport(reason providers.AccessHoldReason,
	locked []providers.AccessLockedMethod) providers.AccessStatusReport {
	return providers.AccessStatusReport{
		Value:   providers.AccessStatusLocked,
		Details: &providers.AccessStatusDetails{Reason: reason, LockedMethods: locked},
	}
}

// liveAuthenticationMethodLocks returns every live, enforced lock on a lockable method, in
// AllAuthenticationMethods order.
func liveAuthenticationMethodLocks(accessState AccessState, now time.Time,
	enforcement lockEnforcement) []providers.AccessLockedMethod {
	locked := make([]providers.AccessLockedMethod, 0, len(accessState.Lock.AuthenticationMethods))
	for _, method := range AllAuthenticationMethods {
		entry, recorded := accessState.Lock.AuthenticationMethods[method]
		if !recorded || !isLockable(method) || !isLockLive(entry.UnlockAt, now) {
			continue
		}
		if !enforcement.enforces(method) {
			continue
		}
		locked = append(locked, providers.AccessLockedMethod{
			Method:    string(method),
			ExpiresAt: expiryOf(entry.UnlockAt),
		})
	}
	return locked
}

// entityHoldReason maps the stored reason to the published one. Anything other than the
// post-suspension hold reports as failed attempts.
func entityHoldReason(stored LockReason) providers.AccessHoldReason {
	if stored == lockReasonPostSuspension {
		return providers.AccessHoldReasonSuspensionReleaseLock
	}
	return providers.AccessHoldReasonFailedAttempts
}

// expiryOf returns unlockAt, or empty for a permanent hold.
func expiryOf(unlockAt string) string {
	if unlockAt == permanentUnlockAt {
		return ""
	}
	return unlockAt
}

// categoriesOf lists a section's categories in a fixed order.
func categoriesOf(cfg accountAccessValue) []namedCategory {
	return []namedCategory{{"user", cfg.User}, {"agent", cfg.Agent}}
}

// validateStructure checks a value's fields without regard to other layers.
func validateStructure(cfg accountAccessValue) error {
	for _, category := range categoriesOf(cfg) {
		if category.value == nil {
			continue
		}
		if err := validateGranularity(category.name, category.value.LockGranularity); err != nil {
			return err
		}
		if err := validateLastLoginResolution(category.name,
			category.value.LastLoginResolutionSeconds); err != nil {
			return err
		}
		if err := validatePolicyStructure(category.name+".default", category.value.Default); err != nil {
			return err
		}
		if err := validateLockEmail(category.name+".notifications.onLock.email",
			category.value.Notifications.lockEmail()); err != nil {
			return err
		}
	}
	return nil
}

// validateLockEmail checks the lock email's template handles, and that each method override names a
// lockable method.
func validateLockEmail(path string, email *accountAccessLockEmail) error {
	if email == nil || email.Templates == nil {
		return nil
	}
	path += ".templates"
	templates := email.Templates
	if err := validateTemplatePair(path, templates.lockEmailTemplatePair); err != nil {
		return err
	}
	if templates.Account != nil {
		if err := validateTemplatePair(path+".account", *templates.Account); err != nil {
			return err
		}
	}
	allowed := lockTemplateMethods()
	for _, method := range slices.Sorted(maps.Keys(templates.Methods)) {
		methodPath := path + ".methods." + method
		if !slices.Contains(allowed, method) {
			return fmt.Errorf("%s: %s is not a lockable authentication method; use %s",
				serverconfig.ConfigNameAccountAccess, methodPath, strings.Join(allowed, " or "))
		}
		if err := validateTemplatePair(methodPath, templates.Methods[method]); err != nil {
			return err
		}
	}
	return nil
}

// lockTemplateMethods lists supported method-specific lock email overrides in a fixed order.
func lockTemplateMethods() []string {
	var methods []string
	for method, lockable := range lockableMethods {
		if lockable && method != AuthenticationMethodEntity {
			methods = append(methods, string(method))
		}
	}
	slices.Sort(methods)
	return methods
}

// validateTemplatePair rejects a template handle that is set and not a valid handle.
func validateTemplatePair(path string, pair lockEmailTemplatePair) error {
	for _, field := range []struct{ name, handle string }{
		{"temporary", pair.Temporary}, {"permanent", pair.Permanent},
	} {
		if field.handle != "" && !sysutils.IsValidHandle(field.handle) {
			return fmt.Errorf("%s: %s.%s is not a valid handle", serverconfig.ConfigNameAccountAccess, path,
				field.name)
		}
	}
	return nil
}

// validateLastLoginResolution checks the last-login resolution is within range.
func validateLastLoginResolution(category string, value *int) error {
	if value == nil {
		return nil
	}
	return validateSeconds(category+".lastLoginResolutionSeconds", *value)
}

// validateSeconds rejects a negative seconds value or one above maxDurationSeconds. The resolver
// clamps such values; a write is refused instead.
func validateSeconds(path string, value int) error {
	if value < 0 {
		return fmt.Errorf("%s: %s must be >= 0", serverconfig.ConfigNameAccountAccess, path)
	}
	if value > maxDurationSeconds {
		return fmt.Errorf("%s: %s must be <= %d", serverconfig.ConfigNameAccountAccess, path, maxDurationSeconds)
	}
	return nil
}

// validateGranularity rejects an unknown granularity, which the resolver would read as the default.
func validateGranularity(category string, value *granularity) error {
	if value == nil {
		return nil
	}
	switch *value {
	case granularityAuthenticationMethod, granularityEntity:
		return nil
	default:
		return fmt.Errorf("%s: %s.lockGranularity must be %q or %q",
			serverconfig.ConfigNameAccountAccess, category, granularityAuthenticationMethod, granularityEntity)
	}
}

// validatePolicyStructure checks one policy's fields in isolation.
func validatePolicyStructure(path string, policy *accountAccessPolicy) error {
	if policy == nil {
		return nil
	}
	if policy.Threshold != nil && *policy.Threshold < 0 {
		return fmt.Errorf("%s: %s.threshold must be >= 0", serverconfig.ConfigNameAccountAccess, path)
	}
	if policy.MaxConsecutiveFailures != nil && *policy.MaxConsecutiveFailures < 0 {
		return fmt.Errorf("%s: %s.maxConsecutiveFailures must be >= 0", serverconfig.ConfigNameAccountAccess, path)
	}
	if policy.FailureWindowSeconds != nil {
		if err := validateSeconds(path+".failureWindowSeconds", *policy.FailureWindowSeconds); err != nil {
			return err
		}
	}
	if policy.LockDecaySeconds != nil {
		if err := validateSeconds(path+".lockDecaySeconds", *policy.LockDecaySeconds); err != nil {
			return err
		}
	}
	for i, duration := range policy.LockDurationsSeconds {
		// Zero means locked until an operator unlocks.
		if err := validateSeconds(fmt.Sprintf("%s.lockDurationsSeconds[%d]", path, i), duration); err != nil {
			return err
		}
	}
	return nil
}

// validateCoherence rejects a merged policy that claims to be on and cannot form a lock.
func validateCoherence(merged accountAccessValue) error {
	for _, category := range categoriesOf(merged) {
		if category.value == nil {
			continue
		}
		if err := validateMergedPolicy(category.name+".default", category.value.Default); err != nil {
			return err
		}
	}
	return nil
}

// validateMergedPolicy rejects an enabled policy that cannot form a lock. A policy is enabled when it
// says so, or when it states any lock field without saying it is off. A policy that states nothing
// configures no lock and is not checked.
func validateMergedPolicy(path string, policy *accountAccessPolicy) error {
	if policy == nil || !sysutils.DerefOr(policy.Enabled, true) {
		return nil
	}
	statesLock := policy.Threshold != nil || policy.FailureWindowSeconds != nil ||
		len(policy.LockDurationsSeconds) > 0 || policy.LockDecaySeconds != nil ||
		policy.MaxConsecutiveFailures != nil
	if policy.Enabled == nil && !statesLock {
		return nil
	}

	var missing []string
	if policy.Threshold == nil || *policy.Threshold <= 0 {
		missing = append(missing, "threshold")
	}
	if policy.FailureWindowSeconds == nil {
		missing = append(missing, "failureWindowSeconds")
	}
	if len(policy.LockDurationsSeconds) == 0 {
		missing = append(missing, "lockDurationsSeconds")
	}
	if len(missing) > 0 {
		return fmt.Errorf(
			"%s: %s is enabled but cannot form a lock; %s must be set, or set enabled to false",
			serverconfig.ConfigNameAccountAccess, path, strings.Join(missing, ", "))
	}
	// A ceiling below the threshold would make the first lock permanent.
	if ceiling := sysutils.DerefOr(policy.MaxConsecutiveFailures, 0); ceiling > 0 && ceiling < *policy.Threshold {
		return fmt.Errorf("%s: %s.maxConsecutiveFailures must be 0 or at least the threshold",
			serverconfig.ConfigNameAccountAccess, path)
	}
	return nil
}

// mergeAccountAccessValue overlays the writable layer onto the read-only one, category by category.
func mergeAccountAccessValue(readOnly, writable accountAccessValue) accountAccessValue {
	return accountAccessValue{
		User:  mergeAccountAccessCategory(readOnly.User, writable.User),
		Agent: mergeAccountAccessCategory(readOnly.Agent, writable.Agent),
	}
}

// mergeAccountAccessCategory overlays one category. If either side is nil, the other is returned.
func mergeAccountAccessCategory(readOnly, writable *accountAccessCategory) *accountAccessCategory {
	if writable == nil {
		return readOnly
	}
	if readOnly == nil {
		return writable
	}

	merged := &accountAccessCategory{
		LockGranularity:            readOnly.LockGranularity,
		RecordLastLogin:            readOnly.RecordLastLogin,
		LastLoginResolutionSeconds: readOnly.LastLoginResolutionSeconds,
		Default:                    overlayAccountAccessPolicy(readOnly.Default, writable.Default),
	}
	if writable.LockGranularity != nil {
		merged.LockGranularity = writable.LockGranularity
	}
	if writable.RecordLastLogin != nil {
		merged.RecordLastLogin = writable.RecordLastLogin
	}
	if writable.LastLoginResolutionSeconds != nil {
		merged.LastLoginResolutionSeconds = writable.LastLoginResolutionSeconds
	}

	merged.Notifications = mergeAccountAccessNotifications(readOnly.Notifications, writable.Notifications)
	return merged
}

// mergeAccountAccessNotifications overlays one notification block onto another, field by field.
func mergeAccountAccessNotifications(readOnly, writable *accountAccessNotifications) *accountAccessNotifications {
	base, override := readOnly.lockEmail(), writable.lockEmail()
	if override == nil {
		return readOnly
	}
	if base == nil {
		return writable
	}

	merged := &accountAccessLockEmail{
		Enabled:   base.Enabled,
		Templates: mergeLockTemplates(base.Templates, override.Templates),
	}
	if override.Enabled != nil {
		merged.Enabled = override.Enabled
	}
	return &accountAccessNotifications{
		OnLock: &accountAccessLockNotification{Email: merged},
	}
}

// mergeLockTemplates overlays one templates block on another, handle by handle and method by method.
// If either side is nil, the other is returned.
func mergeLockTemplates(readOnly, writable *accountAccessLockTemplates) *accountAccessLockTemplates {
	if writable == nil {
		return readOnly
	}
	if readOnly == nil {
		return writable
	}
	merged := &accountAccessLockTemplates{
		lockEmailTemplatePair: overlayTemplatePair(readOnly.lockEmailTemplatePair,
			writable.lockEmailTemplatePair),
		Account: readOnly.Account,
		Methods: maps.Clone(readOnly.Methods),
	}
	if writable.Account != nil {
		account := overlayTemplatePair(sysutils.DerefOr(readOnly.Account, lockEmailTemplatePair{}),
			*writable.Account)
		merged.Account = &account
	}
	for method, pair := range writable.Methods {
		if merged.Methods == nil {
			merged.Methods = make(map[string]lockEmailTemplatePair)
		}
		merged.Methods[method] = overlayTemplatePair(merged.Methods[method], pair)
	}
	return merged
}

// overlayTemplatePair overlays the handles override sets on base. Used both for layer merges and for
// an override over the default pair.
func overlayTemplatePair(base, override lockEmailTemplatePair) lockEmailTemplatePair {
	if override.Temporary != "" {
		base.Temporary = override.Temporary
	}
	if override.Permanent != "" {
		base.Permanent = override.Permanent
	}
	return base
}

// overlayAccountAccessPolicy overlays one policy on another, field by field. Used both for layer
// merges and for a method over its category default.
func overlayAccountAccessPolicy(base, override *accountAccessPolicy) *accountAccessPolicy {
	if override == nil {
		return base
	}
	if base == nil {
		return override
	}
	merged := *base
	if override.Enabled != nil {
		merged.Enabled = override.Enabled
	}
	if override.Threshold != nil {
		merged.Threshold = override.Threshold
	}
	if override.FailureWindowSeconds != nil {
		merged.FailureWindowSeconds = override.FailureWindowSeconds
	}
	if len(override.LockDurationsSeconds) > 0 {
		merged.LockDurationsSeconds = override.LockDurationsSeconds
	}
	if override.LockDecaySeconds != nil {
		merged.LockDecaySeconds = override.LockDecaySeconds
	}
	if override.MaxConsecutiveFailures != nil {
		merged.MaxConsecutiveFailures = override.MaxConsecutiveFailures
	}
	return &merged
}

// lockEmailFor returns the category's lock email when it is switched on, or nil. Only users are
// notified.
func lockEmailFor(cfg accountAccessValue, category providers.EntityCategory) *accountAccessLockEmail {
	if category != providers.EntityCategoryUser || cfg.User == nil {
		return nil
	}
	email := cfg.User.Notifications.lockEmail()
	if email == nil || !sysutils.DerefOr(email.Enabled, false) {
		return nil
	}
	return email
}

// lockTemplateFor selects the lock email template and its render context.
// Account-wide locks use the account override and omit the authentication method.
func lockTemplateFor(email *accountAccessLockEmail, method providers.AuthenticationMethod,
	episode LockEpisode) (string, map[string]string) {
	var pair lockEmailTemplatePair
	if templates := email.Templates; templates != nil {
		override := templates.Methods[string(method)]
		if method == AuthenticationMethodEntity {
			override = sysutils.DerefOr(templates.Account, lockEmailTemplatePair{})
		}
		pair = overlayTemplatePair(templates.lockEmailTemplatePair, override)
	}
	data := map[string]string{
		templateKeyAuthenticationMethod: string(method),
		templateKeyLockDuration:         "",
		templateKeyUnlockTime:           "",
	}
	if method == AuthenticationMethodEntity {
		data[templateKeyAuthenticationMethod] = ""
	}

	ends, err := time.Parse(time.RFC3339, episode.UnlockAt)
	if episode.UnlockAt == permanentUnlockAt || episode.Duration <= 0 || err != nil {
		return pair.Permanent, data
	}
	data[templateKeyLockDuration] = sysutils.FormatExpiryDuration(int64(episode.Duration / time.Second))
	data[templateKeyUnlockTime] = ends.UTC().Format(unlockTimeLayout)
	return pair.Temporary, data
}

// accountAccessValueFrom converts the deployment configuration into its section form.
func accountAccessValueFrom(cfg sysconfig.AccountAccessConfig) accountAccessValue {
	return accountAccessValue{
		User:  accountAccessCategoryFrom(cfg.User),
		Agent: accountAccessCategoryFrom(cfg.Agent),
	}
}

// accountAccessCategoryFrom converts a deployment category. A category that states nothing converts
// to nil, so absence survives the conversion.
func accountAccessCategoryFrom(c sysconfig.CategoryAccessConfig) *accountAccessCategory {
	out := &accountAccessCategory{
		RecordLastLogin:            c.RecordLastLogin,
		LastLoginResolutionSeconds: c.LastLoginResolutionSeconds,
	}
	if c.LockGranularity != "" {
		lockGranularity := granularity(c.LockGranularity)
		out.LockGranularity = &lockGranularity
	}
	if policy := accountAccessPolicyFrom(c.Default); !policy.isEmpty() {
		out.Default = &policy
	}
	out.Notifications = accountAccessNotificationsFrom(c.Notifications)
	if out.LockGranularity == nil && out.Default == nil && out.Notifications == nil &&
		out.RecordLastLogin == nil && out.LastLoginResolutionSeconds == nil {
		return nil
	}
	return out
}

// accountAccessPolicyFrom converts a deployment policy.
func accountAccessPolicyFrom(c sysconfig.LockPolicyConfig) accountAccessPolicy {
	return accountAccessPolicy{
		Enabled:                c.Enabled,
		Threshold:              c.Threshold,
		FailureWindowSeconds:   c.FailureWindowSeconds,
		LockDurationsSeconds:   c.LockDurationsSeconds,
		LockDecaySeconds:       c.LockDecaySeconds,
		MaxConsecutiveFailures: c.MaxConsecutiveFailures,
	}
}

// accountAccessNotificationsFrom converts a deployment notification block, or returns nil when it
// states nothing.
func accountAccessNotificationsFrom(n sysconfig.AccessNotificationsConfig) *accountAccessNotifications {
	email := n.OnLock.Email
	if email.Enabled == nil {
		return nil
	}
	return &accountAccessNotifications{OnLock: &accountAccessLockNotification{Email: &accountAccessLockEmail{
		Enabled: email.Enabled,
	}}}
}
