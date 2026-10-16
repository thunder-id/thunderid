// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package identitygovernance decides account access: admission checks, lock formation, lifecycle
// operations on identities, the accountAccess server-config section and the lock email.
package identitygovernance

import (
	"context"
	"errors"
	"time"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// IdentityGovernanceServiceInterface is the governance authority. Methods take an entity ID and read
// the entity themselves.
type IdentityGovernanceServiceInterface interface {
	// AdmitAuthenticationStep admits one verified step and, when admitted, clears that method's entry.
	AdmitAuthenticationStep(ctx context.Context, entityID string,
		scope providers.AccessScope) (providers.AccessDecision, *tidcommon.ServiceError)

	// RecordFailure records one failed authentication step against a scope.
	RecordFailure(ctx context.Context, entityID string, scope providers.AccessScope) *tidcommon.ServiceError

	// EvaluateAccess decides whether an identity may be used at a plane for an authentication method
	// scope. An empty entityID admits; an unreadable entity refuses.
	EvaluateAccess(ctx context.Context, entityID string, plane providers.AccessPlane,
		scope providers.AccessScope) (providers.AccessDecision, *tidcommon.ServiceError)

	// AdmitSignIn admits a completed sign-in at the session plane and, when admitted, clears an expired
	// entity-wide entry.
	AdmitSignIn(ctx context.Context, entityID string) (providers.AccessDecision, *tidcommon.ServiceError)

	// Unlock releases every automatic lock in every scope and resets the escalation.
	Unlock(ctx context.Context, entityID string) *tidcommon.ServiceError

	// Suspend places an administrative hold. It writes state only and does not revoke sessions or
	// tokens. The operator note is optional. On an already suspended identity it writes nothing and
	// succeeds, keeping the first placement time and note.
	Suspend(ctx context.Context, entityID, operatorNote string) *tidcommon.ServiceError

	// ValidateSuspend runs Suspend's checks without writing.
	ValidateSuspend(ctx context.Context, entityID string) *tidcommon.ServiceError

	// Unsuspend releases the hold. A user is left under a permanent entity-wide lock; an agent or an
	// application becomes active.
	Unsuspend(ctx context.Context, entityID string) *tidcommon.ServiceError

	// RecordLogin stamps when access was last granted, if enabled for the entity's category.
	RecordLogin(ctx context.Context, entityID string) *tidcommon.ServiceError

	// NewAccessReader returns a reader for management reads of one category, with the lockout policy
	// resolved once.
	NewAccessReader(ctx context.Context, category providers.EntityCategory) AccessReaderInterface
}

// EntityStateProvider reads and writes the governed state. The entity service implements it.
type EntityStateProvider interface {
	// GetGovernedEntity reads the entity with its runtime state. It returns ErrEntityNotFound when no
	// identity has the ID, which access checks admit as having nothing to govern. Any other failure
	// must refuse, never admit.
	GetGovernedEntity(ctx context.Context, entityID string) (GovernedEntity, error)

	// GetEntityProfile reads the profile without the runtime state. An unknown identity is an error.
	GetEntityProfile(ctx context.Context, entityID string) (*providers.Entity, error)

	// IncrementFailure records one failure against a scope and returns the resulting counters. It
	// returns false when nothing was written.
	IncrementFailure(ctx context.Context, entityID string, scope providers.AccessScope,
		now, windowStart time.Time) (ScopeLock, bool, error)

	// FormLock opens a lock episode if the counters still match observed. It returns false, not an
	// error, when they changed.
	FormLock(ctx context.Context, entityID string, scope providers.AccessScope, observed ScopeLock,
		episode LockEpisode, now time.Time) (bool, error)

	// ClearAccessState removes the lock entries for the given scopes. Suspension and activity are kept.
	ClearAccessState(ctx context.Context, entityID string, scopes []providers.AccessScope) error

	// ClearAccessStateIfUnchanged clears a scope's lock entry only if the revision is unchanged.
	ClearAccessStateIfUnchanged(ctx context.Context, entityID string, scope providers.AccessScope,
		revision int64) (bool, error)

	// RecordLogin stamps lastLoginAt unless the stored stamp is within [notBefore, at]. A missing row
	// and a skipped write both succeed.
	RecordLogin(ctx context.Context, entityID string, at, notBefore time.Time) error

	// SetSuspension records the suspension time and the operator note, which may be empty. It
	// returns false, not an error, when the entity is already suspended.
	SetSuspension(ctx context.Context, entityID string, at time.Time, operatorNote string) (bool, error)

	// ClearSuspension removes the suspension placed at suspendedAt and writes the given hold, if any,
	// in the same statement. It returns false, not an error, when that suspension is no longer there.
	ClearSuspension(ctx context.Context, entityID string, hold *EntityHold, suspendedAt string) (bool, error)
}

// service is the governance authority.
type service struct {
	entityState EntityStateProvider
	source      accountAccessSourceInterface
	policies    policyResolverInterface
	// notifier is optional; locks form the same without it.
	notifier lockNotifierInterface
	// authorizer is optional; nil permits.
	authorizer sysauthz.SystemAuthorizationServiceInterface
	// observability receives lifecycle audit events. Optional.
	observability providers.ObservabilityProvider
	logger        *log.Logger
	now           func() time.Time
}

// newService builds the governance service.
func newService(entityState EntityStateProvider, source accountAccessSourceInterface,
	notifier lockNotifierInterface, authorizer sysauthz.SystemAuthorizationServiceInterface,
	observability providers.ObservabilityProvider) *service {
	return &service{
		entityState:   entityState,
		source:        source,
		policies:      newPolicyResolver(source),
		notifier:      notifier,
		authorizer:    authorizer,
		observability: observability,
		logger:        log.GetLogger().With(log.String(log.LoggerKeyComponentName, "IdentityGovernanceService")),
		now:           func() time.Time { return time.Now().UTC() },
	}
}

// AdmitAuthenticationStep clears the step's own method entry only after admission. A clear failure is
// logged and does not change the decision.
func (s *service) AdmitAuthenticationStep(ctx context.Context, entityID string,
	scope providers.AccessScope) (providers.AccessDecision, *tidcommon.ServiceError) {
	entity, decided, ok, svcErr := s.readForCheck(ctx, entityID)
	if !ok {
		return decided, svcErr
	}
	decision := s.decide(ctx, entity, providers.AccessPlaneAuthentication, scope)
	if !decision.Admitted {
		return decision, nil
	}
	_, recorded := entity.AccessState.Lock.AuthenticationMethods[scope]
	if recorded && scope != AccessScopeEntity {
		cleared, err := s.entityState.ClearAccessStateIfUnchanged(ctx, entityID, scope, entity.Revision)
		if err != nil {
			s.logger.Warn(ctx, "Failed to clear account access state after an admitted authentication",
				log.MaskedString("entityId", entityID), log.String("scope", string(scope)), log.Error(err))
		} else if !cleared {
			s.logger.Debug(ctx, "Skipped clearing account access state: it changed after the admission read",
				log.MaskedString("entityId", entityID), log.String("scope", string(scope)))
		}
	}
	return decision, nil
}

// RecordFailure counts a failure and forms a lock when the threshold is reached. The policy is
// resolved from the stored entity, not from the caller.
func (s *service) RecordFailure(ctx context.Context, entityID string,
	scope providers.AccessScope) *tidcommon.ServiceError {
	if entityID == "" || !isLockable(scope) {
		return nil
	}

	entity, err := s.entityState.GetGovernedEntity(ctx, entityID)
	if errors.Is(err, ErrEntityNotFound) {
		return nil
	}
	if err != nil {
		return s.serverError(ctx, "Failed to read the identity to record a failed attempt", entityID, err)
	}

	policy := s.policies.Resolve(ctx, entity, scope)
	if !policy.enabled {
		return nil
	}
	effective := effectiveScopeFor(policy.granularity, scope)
	now := s.now()

	// Do not count while a live lock already refuses this scope. Skipping the write also keeps a wrong
	// secret from being distinguishable by timing.
	if _, held := lockFor(entity.AccessState, effective, now); held {
		return nil
	}

	// A zero window never resets the counter on time, so the window starts at the zero time.
	var windowStart time.Time
	if policy.failureWindow > 0 {
		windowStart = now.Add(-policy.failureWindow)
	}
	observed, recorded, err := s.entityState.IncrementFailure(ctx, entity.ID, effective, now, windowStart)
	if err != nil {
		return s.serverError(ctx, "Failed to record a failed attempt", entityID, err)
	}
	if !recorded {
		// No row, or the store's hold guard skipped the write.
		return nil
	}

	return s.formLockIfEligible(ctx, entity, effective, policy, observed, now)
}

// EvaluateAccess returns the access decision at a plane.
func (s *service) EvaluateAccess(ctx context.Context, entityID string, plane providers.AccessPlane,
	scope providers.AccessScope) (providers.AccessDecision, *tidcommon.ServiceError) {
	entity, decided, ok, svcErr := s.readForCheck(ctx, entityID)
	if !ok {
		return decided, svcErr
	}
	return s.decide(ctx, entity, plane, scope), nil
}

// AdmitSignIn checks a completed sign-in and, when admitted, clears an expired entity-wide entry. A
// live entity-wide lock is never cleared.
func (s *service) AdmitSignIn(ctx context.Context,
	entityID string) (providers.AccessDecision, *tidcommon.ServiceError) {
	entity, decided, ok, svcErr := s.readForCheck(ctx, entityID)
	if !ok {
		return decided, svcErr
	}

	decision := s.decide(ctx, entity, providers.AccessPlaneSession, "")
	entry := entity.AccessState.Lock.Entity
	if decision.Admitted && entry != nil && !isLockLive(entry.UnlockAt, s.now()) {
		cleared, err := s.entityState.ClearAccessStateIfUnchanged(ctx, entityID, AccessScopeEntity,
			entity.Revision)
		if err != nil {
			s.logger.Warn(ctx, "Failed to clear the entity access state after a completed sign-in",
				log.MaskedString("entityId", entityID), log.Error(err))
		} else if !cleared {
			s.logger.Debug(ctx, "Skipped clearing the entity access state: it changed after the admission read",
				log.MaskedString("entityId", entityID))
		}
	}
	return decision, nil
}

// Unlock clears lock entries in every scope, including the entity scope. It does not release a
// suspension. A runtime-context caller is a recovery flow, so its unlock is recorded as self-service.
func (s *service) Unlock(ctx context.Context, entityID string) (svcErr *tidcommon.ServiceError) {
	record := lifecycleAudit{succeeded: event.EventTypeIdentityUnlocked,
		failed: event.EventTypeIdentityUnlockFailed, entityID: entityID}
	if security.IsRuntimeContext(ctx) {
		record.succeeded, record.failed = event.EventTypeIdentitySelfUnlocked, event.EventTypeIdentitySelfUnlockFailed
	}
	defer func() { s.audit(ctx, &record, svcErr) }()

	if _, svcErr = s.governedForLifecycle(ctx, entityID, &record); svcErr != nil {
		return svcErr
	}
	if err := s.entityState.ClearAccessState(ctx, entityID, AllAccessScopes); err != nil {
		return s.serverError(ctx, "Failed to release account locks", entityID, err)
	}
	record.message = "Every automatic lock released"
	return nil
}

// Suspend places an administrative hold. See IdentityGovernanceServiceInterface.
func (s *service) Suspend(ctx context.Context, entityID, operatorNote string) (svcErr *tidcommon.ServiceError) {
	noteRecorded := operatorNote != ""
	record := lifecycleAudit{succeeded: event.EventTypeIdentitySuspended,
		failed: event.EventTypeIdentitySuspensionFailed, entityID: entityID, operatorNoteRecorded: &noteRecorded}
	defer func() { s.audit(ctx, &record, svcErr) }()

	entity, svcErr := s.governedForLifecycle(ctx, entityID, &record)
	if svcErr != nil {
		return svcErr
	}
	// Already suspended: write nothing and keep the first placement time and note.
	if isSuspended(entity.State, entity.AccessState) {
		record.message = "Suspension already in place"
		return nil
	}
	placed, err := s.entityState.SetSuspension(ctx, entityID, s.now(), operatorNote)
	if err != nil {
		return s.serverError(ctx, "Failed to place a suspension", entityID, err)
	}
	// A concurrent suspend placed it first; that placement and its note are kept.
	record.message = "Suspension placed"
	if !placed {
		record.message = "Suspension already in place"
	}
	return nil
}

// ValidateSuspend runs Suspend's checks without writing. Only a refusal is audited.
func (s *service) ValidateSuspend(ctx context.Context, entityID string) (svcErr *tidcommon.ServiceError) {
	record := lifecycleAudit{failed: event.EventTypeIdentitySuspensionFailed, entityID: entityID}
	defer func() {
		if svcErr != nil {
			s.audit(ctx, &record, svcErr)
		}
	}()

	_, svcErr = s.governedForLifecycle(ctx, entityID, &record)
	return svcErr
}

// Unsuspend releases the hold and, for a user, writes a permanent entity-wide lock in the same
// statement. See IdentityGovernanceServiceInterface.
func (s *service) Unsuspend(ctx context.Context, entityID string) (svcErr *tidcommon.ServiceError) {
	record := lifecycleAudit{succeeded: event.EventTypeIdentityUnsuspended,
		failed: event.EventTypeIdentityUnsuspensionFailed, entityID: entityID}
	defer func() { s.audit(ctx, &record, svcErr) }()

	entity, svcErr := s.governedForLifecycle(ctx, entityID, &record)
	if svcErr != nil {
		return svcErr
	}
	if !isSuspended(entity.State, entity.AccessState) {
		return &ErrorNotApplicable
	}

	var hold *EntityHold
	if entity.Category == providers.EntityCategoryUser {
		// The permanent sentinel: this hold does not expire.
		hold = &EntityHold{UnlockAt: PermanentUnlockAt, Reason: LockReasonPostSuspension}
	}

	observedSuspension := ""
	if entity.AccessState.Suspend != nil {
		observedSuspension = entity.AccessState.Suspend.SuspendedAt
	}
	cleared, err := s.entityState.ClearSuspension(ctx, entityID, hold, observedSuspension)
	if err != nil {
		return s.serverError(ctx, "Failed to release a suspension", entityID, err)
	}
	if !cleared {
		// A concurrent unsuspend released it, or a new suspension replaced it, after the read.
		return &ErrorNotApplicable
	}

	record.message = "Suspension released"
	if hold != nil {
		record.message = "Suspension released onto a post-suspension lock"
	}
	return nil
}

// RecordLogin stamps the last login when the entity's category has recording enabled. The store
// skips the write when the stored stamp is within the resolution window.
func (s *service) RecordLogin(ctx context.Context, entityID string) *tidcommon.ServiceError {
	if entityID == "" {
		return nil
	}
	policies := s.policies.ActivityPolicies(ctx)
	if !policies.enabled() {
		return nil
	}

	profile, err := s.entityState.GetEntityProfile(ctx, entityID)
	if err != nil {
		// The profile read does not name a missing identity, so the governed read tells one with no
		// local record, which has nothing to stamp, apart from a failed read.
		if _, govErr := s.entityState.GetGovernedEntity(ctx, entityID); errors.Is(govErr, ErrEntityNotFound) {
			return nil
		}
		return s.serverError(ctx, "Failed to read the identity to record a login", entityID, err)
	}

	policy := policies.forCategory(profile.Category)
	if !policy.record {
		return nil
	}

	now := s.now()
	if err := s.entityState.RecordLogin(ctx, entityID, now, now.Add(-policy.resolution)); err != nil {
		return s.serverError(ctx, "Failed to record a login", entityID, err)
	}
	return nil
}

// NewAccessReader resolves the lockout policy for one entity category, once.
func (s *service) NewAccessReader(ctx context.Context, category providers.EntityCategory) AccessReaderInterface {
	return accessReader{enforcement: lockEnforcementFor(s.source.Current(ctx), category), now: s.now}
}

// readForCheck reads the subject of a check. ok is false when the check is already decided: an empty
// subject, or one with no local identity, admits, since there is no hold to apply; any other read
// failure refuses.
func (s *service) readForCheck(ctx context.Context,
	entityID string) (GovernedEntity, providers.AccessDecision, bool, *tidcommon.ServiceError) {
	if entityID == "" {
		return GovernedEntity{}, admit(), false, nil
	}
	entity, err := s.entityState.GetGovernedEntity(ctx, entityID)
	if errors.Is(err, ErrEntityNotFound) {
		// For example an identity an external authentication provider asserts with no local record.
		s.logger.Debug(ctx, "No local identity to govern; admitted", log.MaskedString("entityId", entityID))
		return GovernedEntity{}, admit(), false, nil
	}
	if err != nil {
		return GovernedEntity{}, providers.AccessDecision{}, false,
			s.serverError(ctx, "Failed to read the identity for an access check", entityID, err)
	}
	return entity, providers.AccessDecision{}, true, nil
}

// decide returns the decision for an entity at a plane and scope.
func (s *service) decide(ctx context.Context, entity GovernedEntity, plane providers.AccessPlane,
	scope providers.AccessScope) providers.AccessDecision {
	// A suspension refuses at every plane. The policy is read only for disclosure.
	if isSuspended(entity.State, entity.AccessState) {
		policy := s.policies.Resolve(ctx, entity, scope)
		return refuse(policy.discloseAccountHold, providers.AccessRefusalReasonSuspended)
	}

	// Only a suspension refuses an application.
	if entity.Category == providers.EntityCategoryApp {
		return admit()
	}

	// No lock entries: admit without reading the policy.
	if entity.AccessState.Lock.Entity == nil && len(entity.AccessState.Lock.AuthenticationMethods) == 0 {
		return admit()
	}

	policy := s.policies.Resolve(ctx, entity, scope)
	effective := effectiveScopeFor(policy.granularity, scope)
	entityWide, held := lockFor(entity.AccessState, effective, s.now())
	if !held {
		return admit()
	}

	// An entity-wide entry holds every method. A method-scoped entry holds only a lockable method.
	if !entityWide && !isLockable(effective) {
		return admit()
	}

	postSuspension := entityWide && entity.AccessState.Lock.Entity.Reason == LockReasonPostSuspension
	if !holdsAuthenticationPlane(policy, plane, postSuspension) {
		return admit()
	}

	return refuse(policy.discloseAccountHold, providers.AccessRefusalReasonLocked)
}

// formLockIfEligible opens a lock episode when the counters reach the threshold, re-reading up to
// formationRetries times when a concurrent writer wins the guard.
func (s *service) formLockIfEligible(ctx context.Context, entity GovernedEntity, effective providers.AccessScope,
	policy lockoutPolicy, observed ScopeLock, now time.Time) *tidcommon.ServiceError {
	for range formationRetries {
		if !shouldFormLock(policy, observed, now) {
			return nil
		}

		episode := nextLockEpisode(policy, observed, now)

		formed, err := s.entityState.FormLock(ctx, entity.ID, effective, observed, episode, now)
		if err != nil {
			return s.serverError(ctx, "Failed to form an account lock", entity.ID, err)
		}
		if formed {
			s.logger.Debug(ctx, "Account access lock formed",
				log.MaskedString("entityId", entity.ID),
				log.String("category", string(entity.Category)),
				log.String("scope", string(effective)),
				log.Int("lockCount", episode.LockCount))

			// Only the guard's winner reaches here, so one lock sends one notice. Scheduling never blocks.
			if s.notifier != nil {
				s.notifier.NotifyLockFormed(ctx, entity, effective, episode)
			}
			return nil
		}

		// The row changed. Re-read and decide again.
		refreshed, err := s.entityState.GetGovernedEntity(ctx, entity.ID)
		if err != nil {
			return s.serverError(ctx, "Failed to re-read the identity to form an account lock", entity.ID, err)
		}
		observed = refreshed.AccessState.observedFor(effective)
		observed.Revision = refreshed.Revision
	}

	return nil
}

// governedForLifecycle reads the subject of a lifecycle operation, records its category on the audit
// and authorizes the caller.
func (s *service) governedForLifecycle(ctx context.Context, entityID string,
	record *lifecycleAudit) (GovernedEntity, *tidcommon.ServiceError) {
	if entityID == "" {
		return GovernedEntity{}, &ErrorMissingSubject
	}
	entity, err := s.entityState.GetGovernedEntity(ctx, entityID)
	if errors.Is(err, ErrEntityNotFound) {
		return GovernedEntity{}, &ErrorEntityNotFound
	}
	if err != nil {
		return GovernedEntity{}, s.serverError(ctx, "Failed to read the identity for a lifecycle operation",
			entityID, err)
	}
	record.category = entity.Category
	return entity, s.authorize(ctx, entity)
}

// serverError logs a store or read failure and returns the internal server error.
func (s *service) serverError(ctx context.Context, message, entityID string,
	err error) *tidcommon.ServiceError {
	s.logger.Error(ctx, message, log.MaskedString("entityId", entityID), log.Error(err))
	return &tidcommon.InternalServerError
}
