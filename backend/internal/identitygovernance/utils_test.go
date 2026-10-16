// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernance

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type UtilsTestSuite struct {
	suite.Suite
	now time.Time
}

func TestUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(UtilsTestSuite))
}

func (s *UtilsTestSuite) SetupTest() {
	s.now = time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)
}

// fourDurationEscalation exercises an ordered list whose last duration repeats.
func fourDurationEscalation() lockoutPolicy {
	return lockoutPolicy{
		enabled:   true,
		threshold: 5,
		lockDurations: []time.Duration{
			5 * time.Minute, 15 * time.Minute, time.Hour, 24 * time.Hour,
		},
		lockDecay:     24 * time.Hour,
		failureWindow: 15 * time.Minute,
		granularity:   granularityAuthenticationMethod,
	}
}

func (s *UtilsTestSuite) at(offset time.Duration) string {
	return s.now.Add(offset).UTC().Format(time.RFC3339)
}

// Four lockouts use the four durations in order; the fifth repeats the last.
func (s *UtilsTestSuite) TestConsecutiveLockoutsWalkTheEscalationAndThenRepeat() {
	policy := fourDurationEscalation()

	cases := []struct {
		lockCount    int
		wantDuration time.Duration
	}{
		{0, 5 * time.Minute},
		{1, 15 * time.Minute},
		{2, time.Hour},
		{3, 24 * time.Hour},
		{4, 24 * time.Hour},
		{9, 24 * time.Hour},
	}

	for _, c := range cases {
		// Ended a minute ago, inside the decay window.
		observed := ScopeLock{LockCount: c.lockCount, UnlockAt: s.at(-time.Minute)}

		episode := nextLockEpisode(policy, observed, s.now)
		s.Equal(c.wantDuration, episode.Duration)
		s.Equal(c.lockCount+1, episode.LockCount,
			"lockCount %d must advance by one", c.lockCount)
		s.Equal(s.now.Add(c.wantDuration).UTC().Format(time.RFC3339), episode.UnlockAt,
			"lockCount %d must serve duration %s", c.lockCount, c.wantDuration)
	}
}

func (s *UtilsTestSuite) TestSingleDurationIsFixed() {
	policy := fourDurationEscalation()
	policy.lockDurations = []time.Duration{5 * time.Minute}

	for _, lockCount := range []int{0, 1, 5} {
		observed := ScopeLock{LockCount: lockCount, UnlockAt: s.at(-time.Minute)}
		episode := nextLockEpisode(policy, observed, s.now)
		s.Equal(s.now.Add(5*time.Minute).UTC().Format(time.RFC3339), episode.UnlockAt)
	}
}

// A zero duration stores the permanent sentinel.
func (s *UtilsTestSuite) TestZeroDurationIsPermanent() {
	policy := fourDurationEscalation()
	policy.lockDurations = []time.Duration{5 * time.Minute, 15 * time.Minute, 0}

	observed := ScopeLock{}
	for _, duration := range []time.Duration{5 * time.Minute, 15 * time.Minute} {
		episode := nextLockEpisode(policy, observed, s.now)
		s.Equal(s.now.Add(duration).Format(time.RFC3339), episode.UnlockAt)
		observed = ScopeLock{LockCount: episode.LockCount, UnlockAt: episode.UnlockAt}
		s.now = s.now.Add(duration + time.Second)
	}
	episode := nextLockEpisode(policy, observed, s.now)
	s.Equal(PermanentUnlockAt, episode.UnlockAt)
	s.True(isLockLive(episode.UnlockAt, s.now.Add(100*365*24*time.Hour)),
		"a permanent lock must still read as live a century later")
}

// After the decay window passes, the next lockout starts again at the first duration.
func (s *UtilsTestSuite) TestLockCountDecaysBackToTheFirstDuration() {
	policy := fourDurationEscalation()

	// The previous lock ended just over the decay window ago.
	observed := ScopeLock{LockCount: 3, UnlockAt: s.at(-25 * time.Hour)}

	episode := nextLockEpisode(policy, observed, s.now)
	s.Equal(1, episode.LockCount, "the consecutive count must return to zero after the decay")
	s.Equal(s.now.Add(5*time.Minute).UTC().Format(time.RFC3339), episode.UnlockAt,
		"and the duration served must be the first")
}

// Decay is measured from the end of the previous lock; one hour short of it is not decayed.
func (s *UtilsTestSuite) TestLockCountSurvivesInsideTheDecayWindow() {
	policy := fourDurationEscalation()
	observed := ScopeLock{LockCount: 3, UnlockAt: s.at(-23 * time.Hour)}

	episode := nextLockEpisode(policy, observed, s.now)
	s.Equal(4, episode.LockCount)
	s.Equal(s.now.Add(24*time.Hour).UTC().Format(time.RFC3339), episode.UnlockAt)
}

func (s *UtilsTestSuite) TestPermanentLockDoesNotDecay() {
	policy := fourDurationEscalation()
	observed := ScopeLock{LockCount: 3, UnlockAt: PermanentUnlockAt}

	episode := nextLockEpisode(policy, observed, s.now.Add(365*24*time.Hour))
	s.Equal(4, episode.LockCount, "a permanent lock keeps the escalation where it was")
}

func (s *UtilsTestSuite) TestFirstLockoutStartsAtTheFirstDuration() {
	episode := nextLockEpisode(fourDurationEscalation(), ScopeLock{}, s.now)
	s.Equal(1, episode.LockCount)
	s.Equal(s.now.Add(5*time.Minute).UTC().Format(time.RFC3339), episode.UnlockAt)
}

func (s *UtilsTestSuite) TestDisabledPolicyFormsNothing() {
	policy := fourDurationEscalation()
	policy.enabled = false

	s.False(shouldFormLock(policy, ScopeLock{FailureCount: 99}, s.now))
}

func (s *UtilsTestSuite) TestEmptyEscalationFormsNothing() {
	policy := fourDurationEscalation()
	policy.lockDurations = nil

	s.False(shouldFormLock(policy, ScopeLock{FailureCount: 99}, s.now))
}

func (s *UtilsTestSuite) TestShouldFormLockAtTheThreshold() {
	policy := fourDurationEscalation()

	s.False(shouldFormLock(policy, ScopeLock{FailureCount: 4}, s.now), "below the threshold")
	s.True(shouldFormLock(policy, ScopeLock{FailureCount: 5}, s.now), "at the threshold")
	s.True(shouldFormLock(policy, ScopeLock{FailureCount: 6}, s.now), "past it")
}

// While a lock is live, no new lock forms however many failures arrive.
func (s *UtilsTestSuite) TestNoNewLockFormsWhileOneIsLive() {
	policy := fourDurationEscalation()
	live := ScopeLock{FailureCount: 99, LockCount: 1, UnlockAt: s.at(5 * time.Minute)}

	s.False(shouldFormLock(policy, live, s.now),
		"a live episode must block a second one whatever the count says")

	// Once it lapses, the threshold forms a lock again.
	s.True(shouldFormLock(policy, live, s.now.Add(10*time.Minute)))
}

func (s *UtilsTestSuite) TestNoNewLockFormsUnderAPermanentOne() {
	policy := fourDurationEscalation()
	held := ScopeLock{FailureCount: 99, LockCount: 3, UnlockAt: PermanentUnlockAt}

	s.False(shouldFormLock(policy, held, s.now.Add(100*365*24*time.Hour)))
}

func (s *UtilsTestSuite) TestIsLockLive() {
	s.False(isLockLive("", s.now), "a scope that never locked is not locked")
	s.True(isLockLive(s.at(time.Minute), s.now), "an unlockAt in the future is live")
	s.False(isLockLive(s.at(-time.Minute), s.now), "one in the past is not")
	s.False(isLockLive(s.at(0), s.now), "the instant it expires, it is over")
	s.True(isLockLive(PermanentUnlockAt, s.now), "the sentinel is always live")
}

func (s *UtilsTestSuite) TestLockableScopes() {
	s.True(isLockable(providers.AccessScopeCredential))
	s.True(isLockable(providers.AccessScopeOTP))
	s.True(isLockable(AccessScopeEntity))
	s.False(isLockable(providers.AccessScopeSystemCredential),
		"a client secret is never counted, whatever the category's configuration")

	for _, scope := range []providers.AccessScope{
		providers.AccessScopePasskey,
		providers.AccessScopeMagicLink,
		providers.AccessScopeFederated,
		providers.AccessScopeOpenID4VP,
	} {
		s.False(isLockable(scope), "scope %s names no entity on failure", scope)
	}

	s.False(isLockable("unregistered"), "an unregistered scope locks nothing")
}

// A scope missing from lockableScopes would silently be unlockable whatever an operator configures.
func (s *UtilsTestSuite) TestEveryRegisteredScopeDeclaresWhetherItLocks() {
	for _, scope := range AllAccessScopes {
		s.True(IsAccessScope(scope), "%s must be registered", scope)
		_, declared := lockableScopes[scope]
		s.True(declared, "registered scope %s must declare whether it locks", scope)
	}
	s.Len(lockableScopes, len(AllAccessScopes))
}

func (s *UtilsTestSuite) TestNoUnregisteredScopeDeclaresLockability() {
	for scope := range lockableScopes {
		s.True(IsAccessScope(scope), "scope %q declares lockability but is not registered", scope)
	}
}

// A refusal names its hold only when the mode discloses it; either way it does not admit.
func (s *UtilsTestSuite) TestRefuseNamesTheHoldOnlyWhenDisclosed() {
	for _, reason := range []providers.AccessRefusalReason{
		providers.AccessRefusalReasonLocked, providers.AccessRefusalReasonSuspended,
	} {
		s.Equal(providers.AccessDecision{Reason: reason}, refuse(discloseAlways, reason))
		s.Equal(providers.AccessDecision{}, refuse(discloseNever, reason), "%s", reason)
		s.Equal(providers.AccessDecision{}, refuse("", reason), "an unset mode discloses nothing")
	}
}
