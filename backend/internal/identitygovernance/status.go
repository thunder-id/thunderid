// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernance

import (
	"encoding/json"
	"time"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// AccessReaderInterface derives the access facts for management reads.
type AccessReaderInterface interface {
	Read(entity providers.Entity) AccessRead
}

// accessReader is an AccessReaderInterface with the lockout policy resolved once. Its zero value
// reports every recorded lock.
type accessReader struct {
	enforcement lockEnforcement
	now         func() time.Time
}

// NewDefaultAccessReader returns a reader for callers with no lockout policy; it reports every
// recorded lock.
func NewDefaultAccessReader() AccessReaderInterface {
	return accessReader{}
}

// Read derives the access facts for one entity. An unreadable document reports suspended.
func (r accessReader) Read(entity providers.Entity) AccessRead {
	var stored RuntimeAttributes
	if len(entity.RuntimeAttributes) > 0 {
		if err := json.Unmarshal(entity.RuntimeAttributes, &stored); err != nil {
			return AccessRead{Status: providers.AccessStatusReport{
				Value:   providers.AccessStatusSuspended,
				Details: &providers.AccessStatusDetails{Reason: providers.AccessHoldReasonAdministrativeSuspension},
			}}
		}
	}
	return AccessRead{
		Status:      reportFor(entity.State, stored.AccessState, r.clock(), r.enforcement),
		LastLoginAt: stored.AccessState.LastLoginAt,
	}
}

// clock returns the reader's time, or the current time for the zero value.
func (r accessReader) clock() time.Time {
	if r.now == nil {
		return time.Now().UTC()
	}
	return r.now()
}

// reportFor derives the status a management read reports, and what produced it. Precedence is
// SUSPENDED, then LOCKED, then ACTIVE. Method locks that enforcement no longer applies are skipped.
func reportFor(state providers.EntityState, accessState AccessState, now time.Time,
	enforcement lockEnforcement) providers.AccessStatusReport {
	if isSuspended(state, accessState) {
		return suspendedReport(accessState.Suspend)
	}

	if entry := accessState.Lock.Entity; entry != nil && isLockLive(entry.UnlockAt, now) &&
		(entry.Reason == LockReasonPostSuspension || enforcement.enforces(AccessScopeEntity)) {
		return lockedReport(entityHoldReason(entry.Reason), []providers.AccessLockedScope{{
			Scope:     string(AccessScopeEntity),
			ExpiresAt: expiryOf(entry.UnlockAt),
		}})
	}

	if locked := liveAuthenticationMethodLocks(accessState, now, enforcement); len(locked) > 0 {
		return lockedReport(providers.AccessHoldReasonFailedAttempts, locked)
	}

	return providers.AccessStatusReport{Value: providers.AccessStatusActive}
}

// suspendedReport builds the SUSPENDED report with any recorded details.
func suspendedReport(suspend *SuspendState) providers.AccessStatusReport {
	details := providers.AccessStatusDetails{Reason: providers.AccessHoldReasonAdministrativeSuspension}
	if suspend != nil {
		details.Since = suspend.SuspendedAt
		// Reported only to callers authorized to read the record; never on a refusal.
		details.OperatorNote = suspend.OperatorNote
	}
	return providers.AccessStatusReport{Value: providers.AccessStatusSuspended, Details: &details}
}

// lockedReport builds the LOCKED report.
func lockedReport(reason providers.AccessHoldReason,
	locked []providers.AccessLockedScope) providers.AccessStatusReport {
	return providers.AccessStatusReport{
		Value:   providers.AccessStatusLocked,
		Details: &providers.AccessStatusDetails{Reason: reason, LockedScopes: locked},
	}
}

// liveAuthenticationMethodLocks returns every live, enforced lock on a lockable method, in
// AllAccessScopes order.
func liveAuthenticationMethodLocks(accessState AccessState, now time.Time,
	enforcement lockEnforcement) []providers.AccessLockedScope {
	locked := make([]providers.AccessLockedScope, 0, len(accessState.Lock.AuthenticationMethods))
	for _, scope := range AllAccessScopes {
		entry, recorded := accessState.Lock.AuthenticationMethods[scope]
		if !recorded || !isLockable(scope) || !isLockLive(entry.UnlockAt, now) {
			continue
		}
		if !enforcement.enforces(scope) {
			continue
		}
		locked = append(locked, providers.AccessLockedScope{
			Scope:     string(scope),
			ExpiresAt: expiryOf(entry.UnlockAt),
		})
	}
	return locked
}

// entityHoldReason maps the stored reason to the published one. Anything other than the
// post-suspension hold reports as failed attempts.
func entityHoldReason(stored LockReason) providers.AccessHoldReason {
	if stored == LockReasonPostSuspension {
		return providers.AccessHoldReasonSuspensionReleaseLock
	}
	return providers.AccessHoldReasonFailedAttempts
}

// expiryOf returns unlockAt, or empty for a permanent hold.
func expiryOf(unlockAt string) string {
	if unlockAt == PermanentUnlockAt {
		return ""
	}
	return unlockAt
}
