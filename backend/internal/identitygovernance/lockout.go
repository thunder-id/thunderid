// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernance

import (
	"time"

	governanceconfig "github.com/thunder-id/thunderid/internal/identitygovernance/config"
	"github.com/thunder-id/thunderid/internal/identitygovernance/model"
)

// Lock escalation: repeated lockouts use longer durations from an ordered list whose last entry
// repeats.

// isLockLive reports whether a stored unlockAt is permanent or still in the future.
func isLockLive(unlockAt string, now time.Time) bool {
	if unlockAt == "" {
		return false
	}
	if unlockAt == model.PermanentUnlockAt {
		return true
	}
	return now.UTC().Format(time.RFC3339) < unlockAt
}

// nextLockEpisode returns the lock count and unlockAt for a lock about to form. A zero duration means
// a permanent lock. Call it only after shouldFormLock returns true, which guarantees a duration.
func nextLockEpisode(policy governanceconfig.LockoutPolicy, observed model.ScopeLock,
	now time.Time) model.LockEpisode {
	lockCount := nextLockCount(policy, observed, now)

	// lockCount is one-based; the last duration repeats.
	index := lockCount - 1
	if index >= len(policy.LockDurations) {
		index = len(policy.LockDurations) - 1
	}
	duration := policy.LockDurations[index]

	unlockAt := model.PermanentUnlockAt
	if duration > 0 {
		unlockAt = now.UTC().Add(duration).Format(time.RFC3339)
	}

	return model.LockEpisode{LockCount: lockCount, UnlockAt: unlockAt, Duration: duration}
}

// nextLockCount returns the consecutive lockout number. It resets to 1 once LockDecay has passed
// since the previous lock ended.
func nextLockCount(policy governanceconfig.LockoutPolicy, observed model.ScopeLock, now time.Time) int {
	if observed.UnlockAt == "" {
		return 1
	}
	// A permanent lock never decays.
	if observed.UnlockAt == model.PermanentUnlockAt {
		return observed.LockCount + 1
	}
	if policy.LockDecay > 0 {
		decayedBy := now.UTC().Add(-policy.LockDecay).Format(time.RFC3339)
		if observed.UnlockAt < decayedBy {
			return 1
		}
	}
	if observed.LockCount <= 0 {
		return 1
	}
	return observed.LockCount + 1
}

// shouldFormLock reports whether the observed counters reach the threshold. No lock forms while one is
// live, so failures during a lock cannot escalate it.
func shouldFormLock(policy governanceconfig.LockoutPolicy, observed model.ScopeLock,
	now time.Time) bool {
	if !policy.Enabled || policy.Threshold <= 0 || len(policy.LockDurations) == 0 {
		return false
	}
	if isLockLive(observed.UnlockAt, now) {
		return false
	}
	return observed.FailureCount >= policy.Threshold
}

// lockFor reports whether anything holds scope, ignoring configured granularity:
//
//	lock.entity live                              -> every authentication method is held
//	else lock.authenticationMethods.<scope> live  -> that scope is held
//	else                                          -> nothing is held
//
// The first return is true when the entity-wide entry holds.
func lockFor(a model.AccessState, scope model.AccessScope, now time.Time) (bool, bool) {
	if a.Lock.Entity != nil && isLockLive(a.Lock.Entity.UnlockAt, now) {
		return true, true
	}
	entry, recorded := a.Lock.AuthenticationMethods[scope]
	if recorded && isLockLive(entry.UnlockAt, now) {
		return false, true
	}
	return false, false
}
