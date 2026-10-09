// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"time"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// The access vocabulary is aliased from the SDK providers package so the two cannot drift.
type (
	// HoldLevel is how much a hold withholds. Enforcement reads the level, not the stored state.
	HoldLevel = providers.AccessHoldLevel

	// Plane is the point at which a request is checked. This package decides which levels apply there.
	Plane = providers.AccessPlane

	// Decision is the result of an access check.
	Decision = providers.AccessDecision
)

const (
	// HoldNone withholds nothing.
	HoldNone = providers.AccessHoldNone

	// HoldAuthentication is L1, set automatically by failed attempts. It withholds new authentication
	// and challenge dispatch through the held method (or every method under entity granularity).
	// Existing sessions, grants, refresh and self-service recovery are not affected.
	HoldAuthentication = providers.AccessHoldAuthentication

	// HoldIdentity is L2, an administrative hold. It withholds authentication, existing sessions, new
	// tokens from an existing grant, refresh and self-service. Administrators can still read and write
	// the record.
	HoldIdentity = providers.AccessHoldIdentity
)

const (
	// PlaneAuthentication is P1, right after an authentication method verifies its input. Both levels
	// apply, and method-scoped holds are evaluated only here.
	PlaneAuthentication = providers.AccessPlaneAuthentication

	// PlaneDispatch is checked before a challenge (one-time password, magic link, user-bound passkey
	// request) is sent. Both levels apply on the challenge's scope. Sending a code never moves a counter.
	PlaneDispatch = providers.AccessPlaneDispatch

	// PlaneRecovery is self-service recovery: identification, the credential write, and challenges
	// sent during recovery. L2 only, in both granularity modes, so a locked account can still recover.
	PlaneRecovery = providers.AccessPlaneRecovery

	// PlaneSession is P2, a subject resolved at the grant from a session snapshot or a flow. L2 only,
	// so a lock (which anyone knowing a username can trigger) never ends live sessions.
	PlaneSession = providers.AccessPlaneSession

	// PlaneIssuance is P3, checked by the refresh grant. L2 only, as for PlaneSession.
	PlaneIssuance = providers.AccessPlaneIssuance

	// PlaneApplication is P4, an application resolved at runtime. L2 only: applications do not lock
	// automatically.
	PlaneApplication = providers.AccessPlaneApplication
)

// The disclosable refusal reasons. Kept coarse: they never name the locked method or the time left.
const (
	// ReasonIdentityHeld says an administrator has placed a hold on the identity.
	ReasonIdentityHeld = "identity_held"

	// ReasonCredentialLocked says failed attempts have locked an authentication method.
	ReasonCredentialLocked = "credential_locked" // #nosec G101 -- a refusal reason, not a secret
)

// PermanentUnlockAt is the stored unlock time for a permanent lock (duration zero). It compares
// correctly both as a string and as a timestamp.
const PermanentUnlockAt = "9999-12-31T23:59:59Z"

// GovernedEntity is the entity data governance needs to resolve a policy and derive a hold.
type GovernedEntity struct {
	// ID is the entity's identifier.
	ID string
	// Category and Type select the policy. They come from the stored entity, never from a caller.
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
	// either one marks the entity suspended (see isSuspended).
	Suspend *SuspendState `json:"suspend"`
	// LastLoginAt is when access was last granted. Written only by the runtime; a lock clear keeps it.
	LastLoginAt string `json:"lastLoginAt"`
}

// LockState holds the entity-wide entry and the per-method entries. A live entity entry holds under
// either granularity; method entries are not read under entity granularity.
type LockState struct {
	// Entity is the entity-wide entry, set under entity granularity and after an unsuspend.
	Entity *ScopeLock `json:"entity"`
	// AuthenticationMethods holds the per-method entries, keyed by access scope.
	AuthenticationMethods map[AccessScope]ScopeLock `json:"authenticationMethods"`
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
	// FailureCount is the number of failures in the current window.
	FailureCount int `json:"failureCount"`
	// LockCount is the number of consecutive lockouts, which selects the lock duration.
	LockCount int `json:"lockCount"`
	// UnlockAt is when the current lock ends, empty if never locked, or PermanentUnlockAt.
	UnlockAt string `json:"unlockAt"`
	// LastFailedAt is when the last failure was recorded, empty if none.
	LastFailedAt string `json:"lastFailedAt"`
	// Reason is why an entity-wide entry exists. Descriptive only; UnlockAt decides expiry.
	Reason string `json:"reason"`
}

// ReasonPostSuspension marks the entity-wide hold written on unsuspend.
const ReasonPostSuspension = "POST_SUSPENDED"

// ObservedFor returns the entry recorded for a scope, whether or not it is currently locked.
func (a AccessState) ObservedFor(scope AccessScope) ScopeLock {
	if scope == AccessScopeEntity {
		if a.Lock.Entity == nil {
			return ScopeLock{}
		}
		return *a.Lock.Entity
	}
	return a.Lock.AuthenticationMethods[scope]
}

// EntityHold is the entity-wide hold written on unsuspend, in the same write that clears the suspension.
type EntityHold struct {
	// UnlockAt is PermanentUnlockAt.
	UnlockAt string
	// Reason is descriptive only.
	Reason string
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
