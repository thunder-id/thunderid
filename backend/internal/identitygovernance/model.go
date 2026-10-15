// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernance

import (
	"time"

	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/notification"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// GovernedEntity is the entity data governance needs to resolve a policy and derive a hold.
type GovernedEntity struct {
	// ID is the entity's identifier.
	ID string
	// Category selects the policy. Category and Type come from the stored entity, never from a caller.
	Category providers.EntityCategory
	Type     string
	// OUID is the entity's organization unit.
	OUID string
	// State is the stored lifecycle state.
	State providers.EntityState
	// Revision is the runtime row's revision at read time, used by conditional writes.
	Revision int64
	// AccessState is the parsed access-state document.
	AccessState AccessState
	// ValidUntil is the UTC RFC 3339 instant the entity's lifespan ends, from its system attributes.
	// Empty when it does not end.
	ValidUntil string
	// ValidUntilErr is set when the stored lifespan end does not read. An access check fails on it, so a
	// corrupt value never admits; a lifecycle operation such as a suspension ignores it.
	ValidUntilErr error
}

// RuntimeAttributes is the runtime attribute document governance reads and writes.
type RuntimeAttributes struct {
	AccessState AccessState `json:"accessState"`
}

// AccessState is an entity's locks, suspension and last login.
type AccessState struct {
	// Lock holds the failure counters and lock entries.
	Lock LockState `json:"lock"`
	// Suspend is set while the entity is suspended. It is written together with the state column, and
	// either one marks the entity suspended.
	Suspend *SuspendState `json:"suspend"`
	// LastLoginAt is when access was last granted. A lock clear keeps it.
	LastLoginAt string `json:"lastLoginAt"`
}

// observedFor returns the entry recorded for a method, whether or not it is currently locked.
func (a AccessState) observedFor(method providers.AuthenticationMethod) LockEntry {
	if method == AuthenticationMethodAccount {
		if a.Lock.Account == nil {
			return LockEntry{}
		}
		return *a.Lock.Account
	}
	return a.Lock.AuthenticationMethods[method]
}

// LockState holds the account-wide entry and the per-method entries. A live account entry holds under
// either granularity; method entries are not read under account granularity.
type LockState struct {
	// Account is the account-wide entry, set under account granularity and after an unsuspend.
	Account *LockEntry `json:"account"`
	// AuthenticationMethods holds the per-method entries, keyed by authentication method.
	AuthenticationMethods map[providers.AuthenticationMethod]LockEntry `json:"authenticationMethods"`
}

// SuspendState is the suspension member. Enforcement checks only its presence.
type SuspendState struct {
	// SuspendedAt is when the hold was placed.
	SuspendedAt string `json:"suspendedAt"`
	// OperatorNote is an optional operator note. Returned on management reads, never on a refusal.
	OperatorNote string `json:"operatorNote"`
}

// LockEntry is one entry's stored failure and lock state, whichever member it sits under.
type LockEntry struct {
	// Revision is the runtime row's revision after the increment that returned this entry.
	Revision int64 `json:"-"`
	// FailureCount is the number of failures counted since the counter last reset.
	FailureCount int `json:"failureCount"`
	// LockCount is the number of consecutive lockouts, which selects the lock duration.
	LockCount int `json:"lockCount"`
	// UnlockAt is when the current lock ends, empty if never locked, or permanentUnlockAt.
	UnlockAt string `json:"unlockAt"`
	// LastFailedAt is when the last failure was recorded, empty if none.
	LastFailedAt string `json:"lastFailedAt"`
	// ConsecutiveFailures counts failures since the entry was last cleared. Neither the window nor a
	// lock resets it; only clearing the entry does.
	ConsecutiveFailures int `json:"consecutiveFailures"`
	// Reason is why an account-wide entry exists.
	Reason LockReason `json:"reason"`
}

// AccountHold is the account-wide hold written on unsuspend, in the same write that clears the
// suspension.
type AccountHold struct {
	// UnlockAt is permanentUnlockAt.
	UnlockAt string
	// Reason is descriptive only.
	Reason LockReason
}

// LockEpisode is a new lock computed by the service.
type LockEpisode struct {
	// Duration is the configured duration; zero means the lock does not expire.
	Duration time.Duration
	// LockCount is the new consecutive lockout count.
	LockCount int
	// UnlockAt is when the lock ends, or permanentUnlockAt.
	UnlockAt string
}

// AccessRead is what a management read derives from the access-state document: the effective status
// and the last login time.
type AccessRead struct {
	// Status is the effective status and what produced it.
	Status providers.AccessStatusReport
	// LastLoginAt is when access was last granted, empty when nothing has recorded one.
	LastLoginAt string
}

// NotificationDependencies supplies the services the lock email uses. All are required for the email,
// and omitting one disables it without disabling account locks.
type NotificationDependencies struct {
	// Templates renders the lock email.
	Templates notificationTemplateRenderer
	// Senders sends the email.
	Senders notification.NotificationSenderServiceInterface
	// EntityTypes supplies the user type's primary email, the only address the email goes to.
	EntityTypes entitytype.EntityTypeServiceInterface
}

// lockoutPolicy is the resolved lockout configuration for one category and authentication method.
type lockoutPolicy struct {
	// enabled turns automatic locking on or off.
	enabled bool
	// threshold is the number of counted failures that forms a lock.
	threshold int
	// failureWindow is an inactivity window: a failure more than this long after the previous one
	// restarts the count. Zero never restarts it on time; a successful sign-in or a lock still does.
	failureWindow time.Duration
	// lockDurations is the lock duration per consecutive lockout; the last one repeats. Zero means
	// locked until an operator unlocks.
	lockDurations []time.Duration
	// lockDecay is how long after a lock ends the consecutive lockout count resets.
	lockDecay time.Duration
	// maxConsecutiveFailures is how many failures with no proof of control in between lock the method
	// until a recovery or an unlock, whatever the window and durations say. Zero sets no ceiling.
	maxConsecutiveFailures int
	// granularity is what a lock under this policy holds.
	granularity granularity
	// discloseAccountHold says whether a refusal may state that the account is locked or suspended.
	discloseAccountHold disclosureMode
}

// activityPolicy is the resolved activity-recording configuration for one entity category.
type activityPolicy struct {
	// record says whether lastLoginAt is written when access is granted.
	record bool
	// resolution is how old the stored value may be before it is rewritten. Zero writes every time.
	resolution time.Duration
}

// activityPolicies holds the activity settings for each category.
type activityPolicies struct {
	user, agent activityPolicy
}

// enabled reports whether any category records activity.
func (p activityPolicies) enabled() bool {
	return p.user.record || p.agent.record
}

// forCategory returns the settings for an entity category.
func (p activityPolicies) forCategory(category providers.EntityCategory) activityPolicy {
	switch category {
	case providers.EntityCategoryUser:
		return p.user
	case providers.EntityCategoryAgent:
		return p.agent
	default:
		return activityPolicy{}
	}
}

// lifecycleAudit holds what one lifecycle operation records: event types, target and outcome.
type lifecycleAudit struct {
	succeeded, failed providers.EventType
	entityID          string
	category          providers.EntityCategory
	message           string
	// operatorNoteRecorded says whether a suspension carried a note. The note is never recorded.
	operatorNoteRecorded *bool
}

// lockNotice is one queued lock email. It holds no address; the address is read at delivery.
type lockNotice struct {
	// traceID and deploymentID are the failed sign-in's, so delivery reads the same deployment and
	// logs under the same trace ID.
	traceID      string
	deploymentID string
	entityID     string
	category     providers.EntityCategory
	method       providers.AuthenticationMethod
	episode      LockEpisode
}

// lockEnforcement says which of a category's recorded locks are still enforced. A lock whose method's
// policy is switched off is not, and under account granularity only the account entry is. The zero
// value enforces every lock.
type lockEnforcement struct {
	// unenforced is the set of methods whose recorded locks are not enforced.
	unenforced map[providers.AuthenticationMethod]struct{}
}

// enforces reports whether a recorded lock on this method is enforced.
func (e lockEnforcement) enforces(method providers.AuthenticationMethod) bool {
	_, off := e.unenforced[method]
	return !off
}

// namedCategory is one category of a section value, with its name for error messages.
type namedCategory struct {
	name  string
	value *accountAccessCategory
}

// accountAccessValue is the value of the accountAccess server-config section: the automatic lock
// policy, last-login recording and the lock email, per entity category. Every field is optional so a
// layer states only what it changes.
type accountAccessValue struct {
	User  *accountAccessCategory `json:"user,omitempty"`
	Agent *accountAccessCategory `json:"agent,omitempty"`
}

// isEmpty reports whether the value states nothing.
func (v accountAccessValue) isEmpty() bool {
	return v.User == nil && v.Agent == nil
}

// accountAccessCategory is one entity category's settings. A nil category inherits the layer below.
type accountAccessCategory struct {
	// LockGranularity is "authentication_method" or "account". Nil inherits.
	LockGranularity *granularity `json:"lockGranularity,omitempty"`
	// RecordLastLogin turns on the lastLoginAt write when access is granted.
	RecordLastLogin *bool `json:"recordLastLogin,omitempty"`
	// LastLoginResolutionSeconds skips the write when the stored value is newer than this. 0 writes
	// every time.
	LastLoginResolutionSeconds *int `json:"lastLoginResolutionSeconds,omitempty"`
	// Notifications configures the notices sent to the account holder.
	Notifications *accountAccessNotifications `json:"notifications,omitempty"`
	// Default is the policy for every authentication method in this category.
	Default *accountAccessPolicy `json:"default,omitempty"`
	// Methods holds per-method overrides, overlaid on Default field by field. Not exposed in the
	// section yet, so the decoder rejects the key and no layer sets it.
	Methods map[string]accountAccessPolicy `json:"-"`
}

// accountAccessNotifications is the notices one category sends, keyed by event.
type accountAccessNotifications struct {
	OnLock *accountAccessLockNotification `json:"onLock,omitempty"`
}

// lockEmail returns the on-lock email block, or nil if any level is unset.
func (n *accountAccessNotifications) lockEmail() *accountAccessLockEmail {
	if n == nil || n.OnLock == nil {
		return nil
	}
	return n.OnLock.Email
}

// accountAccessLockNotification is what is sent when an automatic lock forms, keyed by channel.
type accountAccessLockNotification struct {
	Email *accountAccessLockEmail `json:"email,omitempty"`
}

// accountAccessLockEmail is the automatic lock email for one category. It goes to the user type's
// primary email.
type accountAccessLockEmail struct {
	// Enabled turns the email on or off.
	Enabled *bool `json:"enabled,omitempty"`
	// Templates names the email templates. It is set only through the server config API; the bootstrap
	// sets the defaults.
	Templates *accountAccessLockTemplates `json:"templates,omitempty"`
}

// accountAccessLockTemplates names the default email templates and overrides for account-wide
// and single-method locks.
type accountAccessLockTemplates struct {
	lockEmailTemplatePair
	// Account is for a lock on every sign-in method.
	Account *lockEmailTemplatePair `json:"account,omitempty"`
	// Methods is for a lock on one method, keyed by a lockable method.
	Methods map[string]lockEmailTemplatePair `json:"methods,omitempty"`
}

// lockEmailTemplatePair names the temporary and permanent lock email templates.
type lockEmailTemplatePair struct {
	Temporary string `json:"temporary,omitempty"`
	Permanent string `json:"permanent,omitempty"`
}

// accountAccessPolicy is a lockout policy, used as a category default or a per-method override.
// Pointer and slice fields tell unset from zero. A zero window never resets failures on time, a zero
// duration locks until an operator unlocks, a zero decay never decays, and a zero
// maxConsecutiveFailures sets no ceiling.
type accountAccessPolicy struct {
	Enabled                *bool `json:"enabled,omitempty"`
	Threshold              *int  `json:"threshold,omitempty"`
	FailureWindowSeconds   *int  `json:"failureWindowSeconds,omitempty"`
	LockDurationsSeconds   []int `json:"lockDurationsSeconds,omitempty"`
	LockDecaySeconds       *int  `json:"lockDecaySeconds,omitempty"`
	MaxConsecutiveFailures *int  `json:"maxConsecutiveFailures,omitempty"`
}

// isEmpty reports whether the policy states nothing.
func (p accountAccessPolicy) isEmpty() bool {
	return p.Enabled == nil && p.Threshold == nil && p.FailureWindowSeconds == nil &&
		len(p.LockDurationsSeconds) == 0 && p.LockDecaySeconds == nil && p.MaxConsecutiveFailures == nil
}
