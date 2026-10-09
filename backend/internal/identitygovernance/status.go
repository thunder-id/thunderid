// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernance

import (
	"context"
	"encoding/json"
	"time"

	governanceconfig "github.com/thunder-id/thunderid/internal/identitygovernance/config"
	"github.com/thunder-id/thunderid/internal/identitygovernance/model"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// AccessRead is what a management read derives from the access-state document: the effective status
// and the last login time.
type AccessRead struct {
	// Status is the effective status and what produced it.
	Status providers.AccessStatusReport
	// LastLoginAt is when access was last granted, empty when nothing has recorded one.
	LastLoginAt string
}

// AccessReader derives the access facts for management reads, with the lockout policy resolved once.
// Its zero value reports every recorded lock.
type AccessReader struct {
	enforcement governanceconfig.LockEnforcement
}

// NewAccessReader resolves the lockout policy for one entity category, once.
func NewAccessReader(ctx context.Context, category providers.EntityCategory) AccessReader {
	return AccessReader{enforcement: governanceconfig.LockEnforcementFor(ctx, category)}
}

// Read derives the access facts for one entity. An unreadable document reports suspended.
func (r AccessReader) Read(entity providers.Entity) AccessRead {
	var stored model.RuntimeAttributes
	if len(entity.RuntimeAttributes) > 0 {
		if err := json.Unmarshal(entity.RuntimeAttributes, &stored); err != nil {
			return AccessRead{Status: providers.AccessStatusReport{
				Value:   providers.AccessStatusSuspended,
				Details: &providers.AccessStatusDetails{Reason: providers.AccessHoldReasonAdministrativeSuspension},
			}}
		}
	}
	return AccessRead{
		Status:      ReportFor(entity.State, stored.AccessState, time.Now().UTC(), r.enforcement),
		LastLoginAt: stored.AccessState.LastLoginAt,
	}
}

// ReportFor derives the status a management read reports, and what produced it. Precedence is
// SUSPENDED, then LOCKED, then ACTIVE. Method locks that enforcement no longer applies are skipped.
func ReportFor(state providers.EntityState, accessState model.AccessState, now time.Time,
	enforcement governanceconfig.LockEnforcement) providers.AccessStatusReport {
	if isSuspended(state, accessState) {
		return suspendedReport(accessState.Suspend)
	}

	if entry := accessState.Lock.Entity; entry != nil && isLockLive(entry.UnlockAt, now) {
		return lockedReport(entityHoldReason(entry.Reason), []providers.AccessLockedScope{{
			Scope:     string(model.AccessScopeEntity),
			ExpiresAt: expiryOf(entry.UnlockAt),
		}})
	}

	if locked := liveAuthenticationMethodLocks(accessState, now, enforcement); len(locked) > 0 {
		return lockedReport(providers.AccessHoldReasonFailedAttempts, locked)
	}

	return providers.AccessStatusReport{Value: providers.AccessStatusActive}
}

// suspendedReport builds the SUSPENDED report with any recorded details.
func suspendedReport(suspend *model.SuspendState) providers.AccessStatusReport {
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
func liveAuthenticationMethodLocks(accessState model.AccessState, now time.Time,
	enforcement governanceconfig.LockEnforcement) []providers.AccessLockedScope {
	locked := make([]providers.AccessLockedScope, 0, len(accessState.Lock.AuthenticationMethods))
	for _, scope := range model.AllAccessScopes {
		entry, recorded := accessState.Lock.AuthenticationMethods[scope]
		if !recorded || model.ClassOf(scope) != model.ClassLockable || !isLockLive(entry.UnlockAt, now) {
			continue
		}
		if !enforcement.Enforces(scope) {
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
func entityHoldReason(stored string) providers.AccessHoldReason {
	if stored == model.ReasonPostSuspension {
		return providers.AccessHoldReasonSuspensionReleaseLock
	}
	return providers.AccessHoldReasonFailedAttempts
}

// expiryOf returns unlockAt, or empty for a permanent hold.
func expiryOf(unlockAt string) string {
	if unlockAt == model.PermanentUnlockAt {
		return ""
	}
	return unlockAt
}

// isSuspended reports a suspension recorded in either the state column or the access-state document.
func isSuspended(state providers.EntityState, accessState model.AccessState) bool {
	return state == providers.EntityStateSuspended || accessState.Suspend != nil
}
