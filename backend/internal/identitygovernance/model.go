// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernance

import (
	"context"
	"time"

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
}

// RuntimeAttributes is the stored shape of ENTITY_RUNTIME_DATA.RUNTIME_ATTRIBUTES.
type RuntimeAttributes struct {
	AccessState AccessState `json:"accessState"`
}

// AccessState is the parsed access-state document. Unknown members are ignored.
type AccessState struct {
	// Lock holds the failure counters and lock entries.
	Lock LockState `json:"lock"`
	// Suspend is set while the entity is suspended. It is written together with the state column, and
	// either one marks the entity suspended.
	Suspend *SuspendState `json:"suspend"`
	// LastLoginAt is when access was last granted. A lock clear keeps it.
	LastLoginAt string `json:"lastLoginAt"`
}

// observedFor returns the entry recorded for a scope, whether or not it is currently locked.
func (a AccessState) observedFor(scope providers.AccessScope) ScopeLock {
	if scope == AccessScopeEntity {
		if a.Lock.Entity == nil {
			return ScopeLock{}
		}
		return *a.Lock.Entity
	}
	return a.Lock.AuthenticationMethods[scope]
}

// LockState holds the entity-wide entry and the per-method entries. A live entity entry holds under
// either granularity; method entries are not read under entity granularity.
type LockState struct {
	// Entity is the entity-wide entry, set under entity granularity and after an unsuspend.
	Entity *ScopeLock `json:"entity"`
	// AuthenticationMethods holds the per-method entries, keyed by access scope.
	AuthenticationMethods map[providers.AccessScope]ScopeLock `json:"authenticationMethods"`
}

// SuspendState is the suspension member. Enforcement checks only its presence.
type SuspendState struct {
	// SuspendedAt is when the hold was placed.
	SuspendedAt string `json:"suspendedAt"`
	// OperatorNote is an optional operator note. Returned on management reads, never on a refusal.
	OperatorNote string `json:"operatorNote"`
}

// ScopeLock is one entry's stored failure and lock state, whichever member it sits under.
type ScopeLock struct {
	// Revision is the runtime row's revision after the increment that returned this entry.
	Revision int64 `json:"-"`
	// FailureCount is the number of failures counted since the counter last reset.
	FailureCount int `json:"failureCount"`
	// LockCount is the number of consecutive lockouts, which selects the lock duration.
	LockCount int `json:"lockCount"`
	// UnlockAt is when the current lock ends, empty if never locked, or PermanentUnlockAt.
	UnlockAt string `json:"unlockAt"`
	// LastFailedAt is when the last failure was recorded, empty if none.
	LastFailedAt string `json:"lastFailedAt"`
	// Reason is why an entity-wide entry exists.
	Reason LockReason `json:"reason"`
}

// EntityHold is the entity-wide hold written on unsuspend, in the same write that clears the
// suspension.
type EntityHold struct {
	// UnlockAt is PermanentUnlockAt.
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
	// UnlockAt is when the lock ends, or PermanentUnlockAt.
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

// NotificationDependencies supplies the template and email sender services the lock email uses.
// Omitting them disables the email without disabling account locks.
type NotificationDependencies struct {
	// Templates renders the lock email.
	Templates notificationTemplateRenderer
	// Senders sends the email.
	Senders notification.NotificationSenderServiceInterface
	// DefaultSender names the email sender the email goes through.
	DefaultSender notification.DefaultEmailSenderInterface
}

// lockoutPolicy is the resolved lockout configuration for one category and method scope.
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
	// ctx is the failed sign-in's context without its cancellation, so delivery reads the same
	// deployment and logs under the same trace ID.
	ctx      context.Context
	entityID string
	category providers.EntityCategory
	scope    providers.AccessScope
	episode  LockEpisode
}

// lockEnforcement says which of a category's recorded locks are still enforced. A lock whose scope's
// policy is switched off is not, and under entity granularity only the entity entry is. The zero
// value enforces every lock.
type lockEnforcement struct {
	// unenforced is the set of scopes whose recorded locks are not enforced.
	unenforced map[providers.AccessScope]struct{}
}

// enforces reports whether a recorded lock on this scope is enforced.
func (e lockEnforcement) enforces(scope providers.AccessScope) bool {
	_, off := e.unenforced[scope]
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

// IsEmpty reports whether the value states nothing.
func (v accountAccessValue) IsEmpty() bool {
	return v.User == nil && v.Agent == nil
}

// accountAccessCategory is one entity category's settings. A nil category inherits the layer below.
type accountAccessCategory struct {
	// LockGranularity is "authentication_method" or "entity". Empty inherits.
	LockGranularity *string `json:"lockGranularity,omitempty"`
	// RecordLastLogin turns on the lastLoginAt write when access is granted.
	RecordLastLogin *bool `json:"recordLastLogin,omitempty"`
	// LastLoginResolutionSeconds skips the write when the stored value is newer than this. 0 writes
	// every time.
	LastLoginResolutionSeconds *int `json:"lastLoginResolutionSeconds,omitempty"`
	// Notifications configures the notices sent to the account holder.
	Notifications *accountAccessNotifications `json:"notifications,omitempty"`
	// Default is the policy for every authentication method in this category.
	Default *accountAccessPolicy `json:"default,omitempty"`
	// Scopes holds per-method overrides, overlaid on Default field by field. Not exposed in the
	// section yet, so the decoder rejects the key and no layer sets it.
	Scopes map[string]accountAccessPolicy `json:"-"`
}

// accountAccessNotifications is the notices one category sends, keyed by event.
type accountAccessNotifications struct {
	OnLock *accountAccessLockNotification `json:"onLock,omitempty"`
}

// LockEmail returns the on-lock email block, or nil if any level is unset.
func (n *accountAccessNotifications) LockEmail() *accountAccessLockEmail {
	if n == nil || n.OnLock == nil {
		return nil
	}
	return n.OnLock.Email
}

// accountAccessLockNotification is what is sent when an automatic lock forms, keyed by channel.
type accountAccessLockNotification struct {
	Email *accountAccessLockEmail `json:"email,omitempty"`
}

// accountAccessLockEmail is the automatic lock email for one category.
type accountAccessLockEmail struct {
	// Enabled turns the email on or off.
	Enabled *bool `json:"enabled,omitempty"`
	// RecipientAttribute is the profile attribute holding the address. A user without it is skipped.
	RecipientAttribute *string `json:"recipientAttribute,omitempty"`
}

// accountAccessPolicy is a lockout policy, used as a category default or a per-method override.
// Pointer and slice fields tell unset from zero. A zero window never resets failures on time, a zero
// duration locks until an operator unlocks, and a zero decay never decays.
type accountAccessPolicy struct {
	Enabled              *bool `json:"enabled,omitempty"`
	Threshold            *int  `json:"threshold,omitempty"`
	FailureWindowSeconds *int  `json:"failureWindowSeconds,omitempty"`
	LockDurationsSeconds []int `json:"lockDurationsSeconds,omitempty"`
	LockDecaySeconds     *int  `json:"lockDecaySeconds,omitempty"`
}

// IsEmpty reports whether the policy states nothing.
func (p accountAccessPolicy) IsEmpty() bool {
	return p.Enabled == nil && p.Threshold == nil && p.FailureWindowSeconds == nil &&
		len(p.LockDurationsSeconds) == 0 && p.LockDecaySeconds == nil
}
