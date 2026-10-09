// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernance

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	governanceconfig "github.com/thunder-id/thunderid/internal/identitygovernance/config"
	"github.com/thunder-id/thunderid/internal/identitygovernance/model"
)

type LockoutTestSuite struct {
	suite.Suite
	now time.Time
}

func TestLockoutTestSuite(t *testing.T) {
	suite.Run(t, new(LockoutTestSuite))
}

func (s *LockoutTestSuite) SetupTest() {
	s.now = time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)
}

// fourDurationEscalation exercises an ordered list whose last duration repeats.
func fourDurationEscalation() governanceconfig.LockoutPolicy {
	return governanceconfig.LockoutPolicy{
		Enabled:   true,
		Threshold: 5,
		LockDurations: []time.Duration{
			5 * time.Minute, 15 * time.Minute, time.Hour, 24 * time.Hour,
		},
		LockDecay:     24 * time.Hour,
		FailureWindow: 15 * time.Minute,
		Granularity:   governanceconfig.GranularityAuthenticationMethod,
	}
}

func (s *LockoutTestSuite) at(offset time.Duration) string {
	return s.now.Add(offset).UTC().Format(time.RFC3339)
}

// Four lockouts use the four durations in order; the fifth repeats the last.
func (s *LockoutTestSuite) TestConsecutiveLockoutsWalkTheEscalationAndThenRepeat() {
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
		observed := model.ScopeLock{LockCount: c.lockCount, UnlockAt: s.at(-time.Minute)}

		episode := nextLockEpisode(policy, observed, s.now)
		s.Equal(c.wantDuration, episode.Duration)
		s.Equal(c.lockCount+1, episode.LockCount,
			"lockCount %d must advance by one", c.lockCount)
		s.Equal(s.now.Add(c.wantDuration).UTC().Format(time.RFC3339), episode.UnlockAt,
			"lockCount %d must serve duration %s", c.lockCount, c.wantDuration)
	}
}

func (s *LockoutTestSuite) TestSingleDurationIsFixed() {
	policy := fourDurationEscalation()
	policy.LockDurations = []time.Duration{5 * time.Minute}

	for _, lockCount := range []int{0, 1, 5} {
		observed := model.ScopeLock{LockCount: lockCount, UnlockAt: s.at(-time.Minute)}
		episode := nextLockEpisode(policy, observed, s.now)
		s.Equal(s.now.Add(5*time.Minute).UTC().Format(time.RFC3339), episode.UnlockAt)
	}
}

// A zero duration stores the permanent sentinel.
func (s *LockoutTestSuite) TestZeroDurationIsPermanent() {
	policy := fourDurationEscalation()
	policy.LockDurations = []time.Duration{5 * time.Minute, 15 * time.Minute, 0}

	observed := model.ScopeLock{}
	for _, duration := range []time.Duration{5 * time.Minute, 15 * time.Minute} {
		episode := nextLockEpisode(policy, observed, s.now)
		s.Equal(s.now.Add(duration).Format(time.RFC3339), episode.UnlockAt)
		observed = model.ScopeLock{LockCount: episode.LockCount, UnlockAt: episode.UnlockAt}
		s.now = s.now.Add(duration + time.Second)
	}
	episode := nextLockEpisode(policy, observed, s.now)
	s.Equal(model.PermanentUnlockAt, episode.UnlockAt)
	s.True(isLockLive(episode.UnlockAt, s.now.Add(100*365*24*time.Hour)),
		"a permanent lock must still read as live a century later")
}

// After the decay window passes, the next lockout starts again at the first duration.
func (s *LockoutTestSuite) TestLockCountDecaysBackToTheFirstDuration() {
	policy := fourDurationEscalation()

	// The previous lock ended just over the decay window ago.
	observed := model.ScopeLock{LockCount: 3, UnlockAt: s.at(-25 * time.Hour)}

	episode := nextLockEpisode(policy, observed, s.now)
	s.Equal(1, episode.LockCount, "the consecutive count must return to zero after the decay")
	s.Equal(s.now.Add(5*time.Minute).UTC().Format(time.RFC3339), episode.UnlockAt,
		"and the duration served must be the first")
}

// Decay is measured from the end of the previous lock; one hour short of it is not decayed.
func (s *LockoutTestSuite) TestLockCountSurvivesInsideTheDecayWindow() {
	policy := fourDurationEscalation()
	observed := model.ScopeLock{LockCount: 3, UnlockAt: s.at(-23 * time.Hour)}

	episode := nextLockEpisode(policy, observed, s.now)
	s.Equal(4, episode.LockCount)
	s.Equal(s.now.Add(24*time.Hour).UTC().Format(time.RFC3339), episode.UnlockAt)
}

func (s *LockoutTestSuite) TestPermanentLockDoesNotDecay() {
	policy := fourDurationEscalation()
	observed := model.ScopeLock{LockCount: 3, UnlockAt: model.PermanentUnlockAt}

	episode := nextLockEpisode(policy, observed, s.now.Add(365*24*time.Hour))
	s.Equal(4, episode.LockCount, "a permanent lock keeps the escalation where it was")
}

func (s *LockoutTestSuite) TestFirstLockoutStartsAtTheFirstDuration() {
	episode := nextLockEpisode(fourDurationEscalation(), model.ScopeLock{}, s.now)
	s.Equal(1, episode.LockCount)
	s.Equal(s.now.Add(5*time.Minute).UTC().Format(time.RFC3339), episode.UnlockAt)
}

func (s *LockoutTestSuite) TestDisabledPolicyFormsNothing() {
	policy := fourDurationEscalation()
	policy.Enabled = false

	s.False(shouldFormLock(policy, model.ScopeLock{FailureCount: 99}, s.now))
}

func (s *LockoutTestSuite) TestEmptyEscalationFormsNothing() {
	policy := fourDurationEscalation()
	policy.LockDurations = nil

	s.False(shouldFormLock(policy, model.ScopeLock{FailureCount: 99}, s.now))
}

func (s *LockoutTestSuite) TestShouldFormLockAtTheThreshold() {
	policy := fourDurationEscalation()

	s.False(shouldFormLock(policy, model.ScopeLock{FailureCount: 4}, s.now), "below the threshold")
	s.True(shouldFormLock(policy, model.ScopeLock{FailureCount: 5}, s.now), "at the threshold")
	s.True(shouldFormLock(policy, model.ScopeLock{FailureCount: 6}, s.now), "past it")
}

// While a lock is live, no new lock forms however many failures arrive.
func (s *LockoutTestSuite) TestNoNewLockFormsWhileOneIsLive() {
	policy := fourDurationEscalation()
	live := model.ScopeLock{FailureCount: 99, LockCount: 1, UnlockAt: s.at(5 * time.Minute)}

	s.False(shouldFormLock(policy, live, s.now),
		"a live episode must block a second one whatever the count says")

	// Once it lapses, the threshold forms a lock again.
	s.True(shouldFormLock(policy, live, s.now.Add(10*time.Minute)))
}

func (s *LockoutTestSuite) TestNoNewLockFormsUnderAPermanentOne() {
	policy := fourDurationEscalation()
	held := model.ScopeLock{FailureCount: 99, LockCount: 3, UnlockAt: model.PermanentUnlockAt}

	s.False(shouldFormLock(policy, held, s.now.Add(100*365*24*time.Hour)))
}

func (s *LockoutTestSuite) TestIsLockLive() {
	s.False(isLockLive("", s.now), "a scope that never locked is not locked")
	s.True(isLockLive(s.at(time.Minute), s.now), "an unlockAt in the future is live")
	s.False(isLockLive(s.at(-time.Minute), s.now), "one in the past is not")
	s.False(isLockLive(s.at(0), s.now), "the instant it expires, it is over")
	s.True(isLockLive(model.PermanentUnlockAt, s.now), "the sentinel is always live")
}
