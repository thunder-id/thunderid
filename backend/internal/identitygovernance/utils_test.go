// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernance

import (
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	sysconfig "github.com/thunder-id/thunderid/internal/system/config"
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
		observed := LockEntry{LockCount: c.lockCount, UnlockAt: s.at(-time.Minute)}

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
		observed := LockEntry{LockCount: lockCount, UnlockAt: s.at(-time.Minute)}
		episode := nextLockEpisode(policy, observed, s.now)
		s.Equal(s.now.Add(5*time.Minute).UTC().Format(time.RFC3339), episode.UnlockAt)
	}
}

// A zero duration stores the permanent sentinel.
func (s *UtilsTestSuite) TestZeroDurationIsPermanent() {
	policy := fourDurationEscalation()
	policy.lockDurations = []time.Duration{5 * time.Minute, 15 * time.Minute, 0}

	observed := LockEntry{}
	for _, duration := range []time.Duration{5 * time.Minute, 15 * time.Minute} {
		episode := nextLockEpisode(policy, observed, s.now)
		s.Equal(s.now.Add(duration).Format(time.RFC3339), episode.UnlockAt)
		observed = LockEntry{LockCount: episode.LockCount, UnlockAt: episode.UnlockAt}
		s.now = s.now.Add(duration + time.Second)
	}
	episode := nextLockEpisode(policy, observed, s.now)
	s.Equal(permanentUnlockAt, episode.UnlockAt)
	s.True(isLockLive(episode.UnlockAt, s.now.Add(100*365*24*time.Hour)),
		"a permanent lock must still read as live a century later")
}

// After the decay window passes, the next lockout starts again at the first duration.
func (s *UtilsTestSuite) TestLockCountDecaysBackToTheFirstDuration() {
	policy := fourDurationEscalation()

	// The previous lock ended just over the decay window ago.
	observed := LockEntry{LockCount: 3, UnlockAt: s.at(-25 * time.Hour)}

	episode := nextLockEpisode(policy, observed, s.now)
	s.Equal(1, episode.LockCount, "the consecutive count must return to zero after the decay")
	s.Equal(s.now.Add(5*time.Minute).UTC().Format(time.RFC3339), episode.UnlockAt,
		"and the duration served must be the first")
}

// Decay is measured from the end of the previous lock; one hour short of it is not decayed.
func (s *UtilsTestSuite) TestLockCountSurvivesInsideTheDecayWindow() {
	policy := fourDurationEscalation()
	observed := LockEntry{LockCount: 3, UnlockAt: s.at(-23 * time.Hour)}

	episode := nextLockEpisode(policy, observed, s.now)
	s.Equal(4, episode.LockCount)
	s.Equal(s.now.Add(24*time.Hour).UTC().Format(time.RFC3339), episode.UnlockAt)
}

func (s *UtilsTestSuite) TestPermanentLockDoesNotDecay() {
	policy := fourDurationEscalation()
	observed := LockEntry{LockCount: 3, UnlockAt: permanentUnlockAt}

	episode := nextLockEpisode(policy, observed, s.now.Add(365*24*time.Hour))
	s.Equal(4, episode.LockCount, "a permanent lock keeps the escalation where it was")
}

func (s *UtilsTestSuite) TestFirstLockoutStartsAtTheFirstDuration() {
	episode := nextLockEpisode(fourDurationEscalation(), LockEntry{}, s.now)
	s.Equal(1, episode.LockCount)
	s.Equal(s.now.Add(5*time.Minute).UTC().Format(time.RFC3339), episode.UnlockAt)
}

func (s *UtilsTestSuite) TestDisabledPolicyFormsNothing() {
	policy := fourDurationEscalation()
	policy.enabled = false

	s.False(shouldFormLock(policy, LockEntry{FailureCount: 99}, s.now))
}

func (s *UtilsTestSuite) TestEmptyEscalationFormsNothing() {
	policy := fourDurationEscalation()
	policy.lockDurations = nil

	s.False(shouldFormLock(policy, LockEntry{FailureCount: 99}, s.now))
}

func (s *UtilsTestSuite) TestShouldFormLockAtTheThreshold() {
	policy := fourDurationEscalation()

	s.False(shouldFormLock(policy, LockEntry{FailureCount: 4}, s.now), "below the threshold")
	s.True(shouldFormLock(policy, LockEntry{FailureCount: 5}, s.now), "at the threshold")
	s.True(shouldFormLock(policy, LockEntry{FailureCount: 6}, s.now), "past it")
}

// The consecutive ceiling forms a lock below the threshold, and that lock is permanent whatever rung
// the escalation has reached.
func (s *UtilsTestSuite) TestTheConsecutiveCeilingFormsAPermanentLock() {
	policy := fourDurationEscalation()
	policy.maxConsecutiveFailures = 12

	s.False(shouldFormLock(policy, LockEntry{FailureCount: 1, ConsecutiveFailures: 11}, s.now), "below the ceiling")
	atCeiling := LockEntry{FailureCount: 2, LockCount: 2, UnlockAt: s.at(-time.Minute), ConsecutiveFailures: 12}
	s.Require().True(shouldFormLock(policy, atCeiling, s.now), "the ceiling forms a lock below the threshold")

	episode := nextLockEpisode(policy, atCeiling, s.now)
	s.Equal(permanentUnlockAt, episode.UnlockAt, "a lock at the ceiling must not lapse")
	s.Equal(time.Duration(0), episode.Duration)
	s.Equal(3, episode.LockCount, "the escalation still advances")

	below := nextLockEpisode(policy, LockEntry{FailureCount: 5, ConsecutiveFailures: 5}, s.now)
	s.NotEqual(permanentUnlockAt, below.UnlockAt, "below the ceiling the rung decides")
}

// A ceiling of zero sets no ceiling.
func (s *UtilsTestSuite) TestAZeroCeilingSetsNoCeiling() {
	policy := fourDurationEscalation()
	policy.maxConsecutiveFailures = 0
	observed := LockEntry{FailureCount: 1, ConsecutiveFailures: 1000}

	s.False(shouldFormLock(policy, observed, s.now))
	s.NotEqual(permanentUnlockAt, nextLockEpisode(policy, observed, s.now).UnlockAt)
}

// While a lock is live, no new lock forms however many failures arrive.
func (s *UtilsTestSuite) TestNoNewLockFormsWhileOneIsLive() {
	policy := fourDurationEscalation()
	live := LockEntry{FailureCount: 99, LockCount: 1, UnlockAt: s.at(5 * time.Minute)}

	s.False(shouldFormLock(policy, live, s.now),
		"a live episode must block a second one whatever the count says")

	// Once it lapses, the threshold forms a lock again.
	s.True(shouldFormLock(policy, live, s.now.Add(10*time.Minute)))
}

func (s *UtilsTestSuite) TestNoNewLockFormsUnderAPermanentOne() {
	policy := fourDurationEscalation()
	held := LockEntry{FailureCount: 99, LockCount: 3, UnlockAt: permanentUnlockAt}

	s.False(shouldFormLock(policy, held, s.now.Add(100*365*24*time.Hour)))
}

func (s *UtilsTestSuite) TestIsLockLive() {
	s.False(isLockLive("", s.now), "a method that never locked is not locked")
	s.True(isLockLive(s.at(time.Minute), s.now), "an unlockAt in the future is live")
	s.False(isLockLive(s.at(-time.Minute), s.now), "one in the past is not")
	s.False(isLockLive(s.at(0), s.now), "the instant it expires, it is over")
	s.True(isLockLive(permanentUnlockAt, s.now), "the sentinel is always live")
}

func (s *UtilsTestSuite) TestLockableMethods() {
	s.True(isLockable(providers.AuthenticationMethodCredential))
	s.True(isLockable(providers.AuthenticationMethodOTP))
	s.True(isLockable(AuthenticationMethodAccount))
	s.False(isLockable(providers.AuthenticationMethodSystemCredential),
		"a client secret is never counted, whatever the category's configuration")

	for _, method := range []providers.AuthenticationMethod{
		providers.AuthenticationMethodPasskey,
		providers.AuthenticationMethodMagicLink,
		providers.AuthenticationMethodFederated,
		providers.AuthenticationMethodOpenID4VP,
	} {
		s.False(isLockable(method), "method %s names no entity on failure", method)
	}

	s.False(isLockable("unregistered"), "an unregistered method locks nothing")
}

// A method missing from lockableMethods would silently be unlockable whatever an operator configures.
func (s *UtilsTestSuite) TestEveryRegisteredMethodDeclaresWhetherItLocks() {
	for _, method := range AllAuthenticationMethods {
		_, declared := lockableMethods[method]
		s.True(declared, "registered method %s must declare whether it locks", method)
	}
	s.Len(lockableMethods, len(AllAuthenticationMethods))
}

func (s *UtilsTestSuite) TestNoUnregisteredMethodDeclaresLockability() {
	for method := range lockableMethods {
		s.True(slices.Contains(AllAuthenticationMethods, method),
			"method %q declares lockability but is not registered", method)
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

// granularity defaults to per-method, and an unrecognized value takes the default.
func (s *UtilsTestSuite) TestGranularityDefaultsToAuthenticationMethod() {
	s.Equal(granularityAuthenticationMethod, parseGranularity(""))
	s.Equal(granularityAuthenticationMethod, parseGranularity("nonsense"))
	s.Equal(granularityAuthenticationMethod, parseGranularity(granularityAuthenticationMethod))
	s.Equal(granularityAccount, parseGranularity(granularityAccount))
}

// A zero duration is kept as the permanent lock rather than dropped.
func (s *UtilsTestSuite) TestZeroDurationSurvivesTranslation() {
	s.Equal([]time.Duration{5 * time.Minute, 0}, toDurations([]int{300, 0}))
	s.Nil(toDurations(nil))
	s.Equal([]time.Duration{5 * time.Minute}, toDurations([]int{-1, 300}), "a negative duration is dropped")
}

// The zero value enforces every recorded lock.
func (s *UtilsTestSuite) TestTheZeroValueEnforcesEveryLock() {
	var enforcement lockEnforcement
	for _, method := range AllAuthenticationMethods {
		s.True(enforcement.enforces(method), "%s", method)
	}
}

// Under account granularity, entries under authentication methods are not enforced.
func (s *UtilsTestSuite) TestAccountGranularityUnenforcesEveryMethodMethod() {
	cfg := testAccountAccessConfig()
	cfg.User.LockGranularity = new(granularityAccount)

	enforcement := lockEnforcementFor(cfg, providers.EntityCategoryUser)

	s.False(enforcement.enforces(providers.AuthenticationMethodCredential))
	s.False(enforcement.enforces(providers.AuthenticationMethodOTP))
	s.True(enforcement.enforces(AuthenticationMethodAccount), "the account entry is always enforced")
}

// A method whose policy is switched off no longer enforces its recorded lock.
func (s *UtilsTestSuite) TestADisabledMethodPolicyIsNotEnforced() {
	cfg := testAccountAccessConfig()
	cfg.User.Methods = map[string]accountAccessPolicy{string(providers.AuthenticationMethodOTP): {Enabled: new(false)}}

	enforcement := lockEnforcementFor(cfg, providers.EntityCategoryUser)

	s.False(enforcement.enforces(providers.AuthenticationMethodOTP))
	s.True(enforcement.enforces(providers.AuthenticationMethodCredential))
	s.True(enforcement.enforces(AuthenticationMethodAccount))
}

// With locking off for a category, no failure lock is enforced, the account entry included.
func (s *UtilsTestSuite) TestADisabledCategoryEnforcesNoFailureLock() {
	enforcement := lockEnforcementFor(testAccountAccessConfig(), providers.EntityCategoryAgent)

	s.False(enforcement.enforces(providers.AuthenticationMethodCredential))
	s.False(enforcement.enforces(AuthenticationMethodAccount))
}

// newTestLockEnforcement returns an enforcement that skips the given methods.
func newTestLockEnforcement(unenforced map[providers.AuthenticationMethod]struct{}) lockEnforcement {
	return lockEnforcement{unenforced: unenforced}
}

func allMethods() map[providers.AuthenticationMethod]struct{} {
	methods := make(map[providers.AuthenticationMethod]struct{})
	for _, method := range AllAuthenticationMethods {
		if method != AuthenticationMethodAccount {
			methods[method] = struct{}{}
		}
	}
	return methods
}

// SUSPENDED takes precedence over LOCKED, which takes precedence over ACTIVE.
func (s *UtilsTestSuite) TestEffectiveStatusPrecedence() {
	locked := AccessState{}
	putLockEntry(&locked, testMethod, LockEntry{UnlockAt: s.at(5 * time.Minute)})

	suspended := AccessState{Suspend: &SuspendState{SuspendedAt: s.at(-time.Hour)}}

	suspendedAndLocked := suspended
	putLockEntry(&suspendedAndLocked, testMethod, LockEntry{UnlockAt: s.at(5 * time.Minute)})

	lapsed := AccessState{}
	putLockEntry(&lapsed, testMethod, LockEntry{UnlockAt: s.at(-time.Second)})

	postSuspension := AccessState{}
	putLockEntry(&postSuspension, AuthenticationMethodAccount,
		LockEntry{UnlockAt: permanentUnlockAt, Reason: lockReasonPostSuspension})

	cases := []struct {
		name  string
		state providers.EntityState
		doc   AccessState
		want  providers.AccessStatus
	}{
		{"nothing held", providers.EntityStateActive, AccessState{}, providers.AccessStatusActive},
		{"lock only", providers.EntityStateActive, locked, providers.AccessStatusLocked},
		// Either store alone reports the suspension.
		{"the document alone", providers.EntityStateActive, suspended, providers.AccessStatusSuspended},
		{"the column alone", providers.EntityStateSuspended, AccessState{}, providers.AccessStatusSuspended},
		{"a suspension beats a lock", providers.EntityStateSuspended, suspendedAndLocked,
			providers.AccessStatusSuspended},
		{"an expired lock is not a lock", providers.EntityStateActive, lapsed, providers.AccessStatusActive},
		// Only SUSPENDED on the state column holds.
		{"an unrecognized state is not a suspension", providers.EntityState("SOME_FUTURE_STATE"),
			AccessState{}, providers.AccessStatusActive},
		{"an empty state is not a suspension", providers.EntityState(""), AccessState{}, providers.AccessStatusActive},
		{"an unrecognized state still reports its lock", providers.EntityState("SOME_FUTURE_STATE"),
			locked, providers.AccessStatusLocked},
		{"an unrecognized state with a suspension member is suspended",
			providers.EntityState("SOME_FUTURE_STATE"), suspended, providers.AccessStatusSuspended},
		{"the post-suspension hold reads LOCKED", providers.EntityStateActive, postSuspension,
			providers.AccessStatusLocked},
	}

	for _, c := range cases {
		s.Equal(c.want, reportFor(c.state, c.doc, s.now, lockEnforcement{}).Value, c.name)
	}
}

// Each case asserts both the status and its details.
func (s *UtilsTestSuite) TestStatusDetailsDescribeTheHold() {
	automatic := AccessState{}
	putLockEntry(&automatic, providers.AuthenticationMethodOTP, LockEntry{LockCount: 1,
		UnlockAt: s.at(5 * time.Minute)})

	postSuspension := AccessState{}
	putLockEntry(&postSuspension, AuthenticationMethodAccount,
		LockEntry{UnlockAt: permanentUnlockAt, Reason: lockReasonPostSuspension})

	suspended := AccessState{Suspend: &SuspendState{
		SuspendedAt: s.at(-time.Hour), OperatorNote: "Credential exposure",
	}}

	suspendedAndLocked := suspended
	putLockEntry(&suspendedAndLocked, providers.AuthenticationMethodOTP, LockEntry{UnlockAt: s.at(5 * time.Minute)})

	noteless := AccessState{Suspend: &SuspendState{SuspendedAt: s.at(-time.Hour)}}

	suspension := &providers.AccessStatusDetails{
		Reason:       providers.AccessHoldReasonAdministrativeSuspension,
		Since:        s.at(-time.Hour),
		OperatorNote: "Credential exposure",
	}
	cases := []struct {
		name   string
		state  providers.EntityState
		doc    AccessState
		status providers.AccessStatus
		want   *providers.AccessStatusDetails
	}{
		{"nothing held", providers.EntityStateActive, AccessState{}, providers.AccessStatusActive, nil},
		{"an automatic lock names its authentication_method and when it lifts", providers.EntityStateActive,
			automatic, providers.AccessStatusLocked, &providers.AccessStatusDetails{
				Reason: providers.AccessHoldReasonFailedAttempts,
				LockedMethods: []providers.AccessLockedMethod{
					{Method: string(providers.AuthenticationMethodOTP), ExpiresAt: s.at(5 * time.Minute)},
				},
			}},
		{"the post-suspension hold reports no expiry", providers.EntityStateActive, postSuspension,
			providers.AccessStatusLocked, &providers.AccessStatusDetails{
				Reason:        providers.AccessHoldReasonSuspensionReleaseLock,
				LockedMethods: []providers.AccessLockedMethod{{Method: string(AuthenticationMethodAccount)}},
			}},
		{"a suspension carries when it was placed and why", providers.EntityStateSuspended, suspended,
			providers.AccessStatusSuspended, suspension},
		{"the details follow the same precedence as the status", providers.EntityStateSuspended,
			suspendedAndLocked, providers.AccessStatusSuspended, suspension},
		{"a suspension known only from the column reports no circumstances", providers.EntityStateSuspended,
			AccessState{}, providers.AccessStatusSuspended,
			&providers.AccessStatusDetails{Reason: providers.AccessHoldReasonAdministrativeSuspension}},
		{"an operator who gave no note produces none", providers.EntityStateSuspended, noteless,
			providers.AccessStatusSuspended, &providers.AccessStatusDetails{
				Reason: providers.AccessHoldReasonAdministrativeSuspension,
				Since:  s.at(-time.Hour),
			}},
	}

	for _, c := range cases {
		report := reportFor(c.state, c.doc, s.now, lockEnforcement{})
		s.Equal(c.status, report.Value, c.name)
		s.Equal(c.want, report.Details, c.name)
	}
}

func (s *UtilsTestSuite) TestEveryLiveAuthenticationMethodLockIsReported() {
	both := AccessState{}
	putLockEntry(&both, providers.AuthenticationMethodOTP, LockEntry{LockCount: 1, UnlockAt: s.at(10 * time.Minute)})
	putLockEntry(&both, providers.AuthenticationMethodCredential, LockEntry{LockCount: 2,
		UnlockAt: s.at(5 * time.Minute)})

	report := reportFor(providers.EntityStateActive, both, s.now, lockEnforcement{})
	s.Equal(providers.AccessStatusLocked, report.Value)
	s.Require().NotNil(report.Details)
	s.Equal(providers.AccessHoldReasonFailedAttempts, report.Details.Reason)
	s.Equal([]providers.AccessLockedMethod{
		{Method: string(providers.AuthenticationMethodCredential), ExpiresAt: s.at(5 * time.Minute)},
		{Method: string(providers.AuthenticationMethodOTP), ExpiresAt: s.at(10 * time.Minute)},
	}, report.Details.LockedMethods, "in the registry's order, so two reads agree")

	// A lapsed entry stays in the document but is not reported.
	oneLapsed := AccessState{}
	putLockEntry(&oneLapsed, providers.AuthenticationMethodOTP, LockEntry{LockCount: 1,
		UnlockAt: s.at(10 * time.Minute)})
	putLockEntry(&oneLapsed, providers.AuthenticationMethodCredential, LockEntry{LockCount: 2,
		UnlockAt: s.at(-time.Minute)})

	report = reportFor(providers.EntityStateActive, oneLapsed, s.now, lockEnforcement{})
	s.Require().NotNil(report.Details)
	s.Equal([]providers.AccessLockedMethod{
		{Method: string(providers.AuthenticationMethodOTP), ExpiresAt: s.at(10 * time.Minute)},
	}, report.Details.LockedMethods)
}

// The account-wide entry reports as the single account method, not as a list of methods.
func (s *UtilsTestSuite) TestTheAccountEntryReportsAsOneMethod() {
	doc := AccessState{}
	putLockEntry(&doc, AuthenticationMethodAccount, LockEntry{LockCount: 1, UnlockAt: s.at(30 * time.Minute)})
	putLockEntry(&doc, providers.AuthenticationMethodOTP, LockEntry{LockCount: 1, UnlockAt: s.at(5 * time.Minute)})

	report := reportFor(providers.EntityStateActive, doc, s.now, lockEnforcement{})
	s.Require().NotNil(report.Details)
	s.Equal([]providers.AccessLockedMethod{
		{Method: string(AuthenticationMethodAccount), ExpiresAt: s.at(30 * time.Minute)},
	}, report.Details.LockedMethods)
}

// A permanent lock reports no expiry.
func (s *UtilsTestSuite) TestTheSentinelIsNeverReportedAsADate() {
	held := AccessState{}
	putLockEntry(&held, providers.AuthenticationMethodCredential, LockEntry{LockCount: 4, UnlockAt: permanentUnlockAt})

	details := reportFor(providers.EntityStateActive, held, s.now, lockEnforcement{}).Details
	s.Require().NotNil(details)
	s.Require().Len(details.LockedMethods, 1)
	s.Empty(details.LockedMethods[0].ExpiresAt, "a hold that does not lift reports no expiry")
	s.Equal(providers.AccessHoldReasonFailedAttempts, details.Reason,
		"a permanent lock is still the escalation's doing, not a suspension release")
}

// An entry under a method whose class is not lockable is not reported.
func (s *UtilsTestSuite) TestAnUnlockableMethodIsNotReported() {
	doc := AccessState{}
	putLockEntry(&doc, providers.AuthenticationMethodPasskey, LockEntry{UnlockAt: s.at(5 * time.Minute)})

	report := reportFor(providers.EntityStateActive, doc, s.now, lockEnforcement{})
	s.Equal(providers.AccessStatusActive, report.Value)
	s.Nil(report.Details)
}

// A lock whose policy is switched off is not reported, since the runtime admits past it.
func (s *UtilsTestSuite) TestAnUnenforcedMethodIsNotReported() {
	var doc AccessState
	putLockEntry(&doc, providers.AuthenticationMethodCredential, LockEntry{UnlockAt: s.at(5 * time.Minute)})
	off := newTestLockEnforcement(map[providers.AuthenticationMethod]struct{}{
		providers.AuthenticationMethodCredential: {}})

	s.Equal(providers.AccessStatusLocked, reportFor(providers.EntityStateActive, doc, s.now, lockEnforcement{}).Value,
		"the zero value enforces everything")
	s.Equal(providers.AccessStatusActive, reportFor(providers.EntityStateActive, doc, s.now, off).Value,
		"a method the runtime admits past must not read as locked")
}

func (s *UtilsTestSuite) TestOnlyTheUnenforcedMethodIsDropped() {
	var doc AccessState
	putLockEntry(&doc, providers.AuthenticationMethodCredential, LockEntry{UnlockAt: s.at(5 * time.Minute)})
	putLockEntry(&doc, providers.AuthenticationMethodOTP, LockEntry{UnlockAt: s.at(5 * time.Minute)})

	report := reportFor(providers.EntityStateActive, doc, s.now, newTestLockEnforcement(
		map[providers.AuthenticationMethod]struct{}{providers.AuthenticationMethodCredential: {}}))

	s.Equal(providers.AccessStatusLocked, report.Value)
	s.Require().Len(report.Details.LockedMethods, 1)
	s.Equal(string(providers.AuthenticationMethodOTP), report.Details.LockedMethods[0].Method)
}

// The post-suspension hold is reported regardless of enforcement, since it is enforced regardless of
// the lockout setting.
func (s *UtilsTestSuite) TestAnAccountWideHoldIsReportedEvenWhenNoMethodMethodIs() {
	doc := AccessState{Lock: LockState{Account: &LockEntry{
		UnlockAt: permanentUnlockAt, Reason: lockReasonPostSuspension,
	}}}

	report := reportFor(providers.EntityStateActive, doc, s.now, newTestLockEnforcement(allMethods()))

	s.Equal(providers.AccessStatusLocked, report.Value)
	s.Equal(providers.AccessHoldReasonSuspensionReleaseLock, report.Details.Reason)
}

// An account-wide failure lock follows enforcement, as the check does.
func (s *UtilsTestSuite) TestAnUnenforcedAccountWideFailureLockIsNotReported() {
	doc := AccessState{Lock: LockState{Account: &LockEntry{LockCount: 3, UnlockAt: permanentUnlockAt}}}

	report := reportFor(providers.EntityStateActive, doc, s.now, newTestLockEnforcement(
		map[providers.AuthenticationMethod]struct{}{AuthenticationMethodAccount: {}}))

	s.Equal(providers.AccessStatusActive, report.Value)
}

// Last-login settings overlay field by field across layers.
func (s *UtilsTestSuite) TestLastLoginSettingsOverlayFieldByField() {
	readOnly := accountAccessValue{User: &accountAccessCategory{RecordLastLogin: new(false),
		LastLoginResolutionSeconds: new(3600)}}
	writable := accountAccessValue{User: &accountAccessCategory{RecordLastLogin: new(true)}}

	merged := mergeAccountAccessValue(readOnly, writable)

	s.True(*merged.User.RecordLastLogin, "the writable layer wins where it states a value")
	s.Equal(3600, *merged.User.LastLoginResolutionSeconds, "and the layer beneath supplies what it does not state")
}

// A default named in both layers is overlaid field by field.
func (s *UtilsTestSuite) TestADefaultInBothLayersIsOverlaid() {
	writable := accountAccessValue{User: &accountAccessCategory{Default: &accountAccessPolicy{Threshold: new(4)}}}

	merged := mergeAccountAccessValue(testAccountAccessConfig(), writable).User.Default

	s.Equal(4, *merged.Threshold)
	s.Equal(900, *merged.FailureWindowSeconds, "an unstated field keeps the read-only value")
}

// A category absent from either layer is taken from the other.
func (s *UtilsTestSuite) TestACategoryAbsentFromOneLayerIsTakenFromTheOther() {
	readOnly := testAccountAccessConfig()

	merged := mergeAccountAccessValue(readOnly, accountAccessValue{})
	s.Same(readOnly.User, merged.User)
	s.Same(readOnly.Agent, merged.Agent)

	merged = mergeAccountAccessValue(accountAccessValue{}, readOnly)
	s.Same(readOnly.User, merged.User)
	s.Same(readOnly.Agent, merged.Agent)
}

// lockEmailNotifications builds a notification block holding only the lock email.
func lockEmailNotifications(email accountAccessLockEmail) *accountAccessNotifications {
	return &accountAccessNotifications{OnLock: &accountAccessLockNotification{Email: &email}}
}

// Each category field the writable layer states replaces the read-only one; every other field is kept.
func (s *UtilsTestSuite) TestMergeAccountAccessCategoryOverlaysEachField() {
	readOnly := func() *accountAccessCategory {
		return &accountAccessCategory{
			LockGranularity:            new(granularityAuthenticationMethod),
			RecordLastLogin:            new(false),
			LastLoginResolutionSeconds: new(3600),
			Default:                    &accountAccessPolicy{Threshold: new(5), FailureWindowSeconds: new(900)},
			Notifications:              lockEmailNotifications(accountAccessLockEmail{Enabled: new(true)}),
		}
	}
	for name, tc := range map[string]struct {
		writable accountAccessCategory
		edit     func(want *accountAccessCategory)
	}{
		"nothing stated": {accountAccessCategory{}, func(*accountAccessCategory) {}},
		"lockGranularity": {accountAccessCategory{LockGranularity: new(granularityAccount)},
			func(want *accountAccessCategory) { want.LockGranularity = new(granularityAccount) }},
		"recordLastLogin": {accountAccessCategory{RecordLastLogin: new(true)},
			func(want *accountAccessCategory) { want.RecordLastLogin = new(true) }},
		"lastLoginResolutionSeconds": {accountAccessCategory{LastLoginResolutionSeconds: new(60)},
			func(want *accountAccessCategory) { want.LastLoginResolutionSeconds = new(60) }},
		"default": {accountAccessCategory{Default: &accountAccessPolicy{Threshold: new(3)}},
			func(want *accountAccessCategory) { want.Default.Threshold = new(3) }},
		"notifications": {accountAccessCategory{
			Notifications: lockEmailNotifications(accountAccessLockEmail{Enabled: new(false)}),
		}, func(want *accountAccessCategory) {
			want.Notifications = lockEmailNotifications(accountAccessLockEmail{Enabled: new(false)})
		}},
	} {
		want := readOnly()
		tc.edit(want)

		s.Equal(want, mergeAccountAccessCategory(readOnly(), &tc.writable), name)
	}
}

// Either side of a lock email merge may be absent, and a stated field replaces the other side's.
func (s *UtilsTestSuite) TestMergeAccountAccessNotificationsOverlaysEachField() {
	base := lockEmailNotifications(accountAccessLockEmail{Enabled: new(true)})
	override := lockEmailNotifications(accountAccessLockEmail{Enabled: new(false)})

	s.Same(base, mergeAccountAccessNotifications(base, nil), "no override keeps the read-only block")
	s.Same(base, mergeAccountAccessNotifications(base, &accountAccessNotifications{}),
		"an override with no email keeps the read-only block")
	s.Same(override, mergeAccountAccessNotifications(nil, override), "no read-only block takes the override")
	s.Same(override, mergeAccountAccessNotifications(&accountAccessNotifications{}, override),
		"a read-only block with no email takes the override")

	for name, tc := range map[string]struct {
		override accountAccessLockEmail
		want     accountAccessLockEmail
	}{
		"nothing stated": {accountAccessLockEmail{}, accountAccessLockEmail{Enabled: new(true)}},
		"enabled":        {accountAccessLockEmail{Enabled: new(false)}, accountAccessLockEmail{Enabled: new(false)}},
	} {
		s.Equal(lockEmailNotifications(tc.want),
			mergeAccountAccessNotifications(base, lockEmailNotifications(tc.override)), name)
	}
}

// A template the writable layer states replaces the read-only one; the account and method overrides
// merge field by field, method by method, and the read-only block is not changed.
func (s *UtilsTestSuite) TestMergeAccountAccessNotificationsOverlaysTheTemplates() {
	readOnly := func() *accountAccessNotifications {
		return lockEmailNotifications(accountAccessLockEmail{Enabled: new(true), Templates: &accountAccessLockTemplates{
			lockEmailTemplatePair: lockEmailTemplatePair{Temporary: "ro", Permanent: "ro-permanent"},
			Account: &lockEmailTemplatePair{
				Temporary: "ro-account", Permanent: "ro-account-permanent"},
			Methods: map[string]lockEmailTemplatePair{
				"credential": {Temporary: "ro-credential", Permanent: "ro-credential-permanent"}},
		}})
	}
	base := readOnly()
	writable := lockEmailNotifications(accountAccessLockEmail{Templates: &accountAccessLockTemplates{
		lockEmailTemplatePair: lockEmailTemplatePair{Temporary: "wr"},
		Account:               &lockEmailTemplatePair{Permanent: "wr-account-permanent"},
		Methods: map[string]lockEmailTemplatePair{
			"credential": {Temporary: "wr-credential"}, "otp": {Temporary: "wr-otp"}},
	}})

	s.Equal(lockEmailNotifications(accountAccessLockEmail{Enabled: new(true), Templates: &accountAccessLockTemplates{
		lockEmailTemplatePair: lockEmailTemplatePair{Temporary: "wr", Permanent: "ro-permanent"},
		Account: &lockEmailTemplatePair{
			Temporary: "ro-account", Permanent: "wr-account-permanent"},
		Methods: map[string]lockEmailTemplatePair{
			"credential": {Temporary: "wr-credential", Permanent: "ro-credential-permanent"},
			"otp":        {Temporary: "wr-otp"},
		},
	}}), mergeAccountAccessNotifications(base, writable))
	s.Equal(readOnly(), base)

	only := &accountAccessLockTemplates{lockEmailTemplatePair: lockEmailTemplatePair{Temporary: "t"}}
	s.Same(only, mergeLockTemplates(nil, only), "no read-only block takes the writable one")
	s.Same(only, mergeLockTemplates(only, nil), "no writable block keeps the read-only one")
}

// Each policy field the override states replaces the base's; every other field is kept.
func (s *UtilsTestSuite) TestOverlayAccountAccessPolicyOverlaysEachField() {
	base := func() *accountAccessPolicy {
		return &accountAccessPolicy{Enabled: new(true), Threshold: new(5), FailureWindowSeconds: new(900),
			LockDurationsSeconds: []int{300}, LockDecaySeconds: new(86400), MaxConsecutiveFailures: new(20)}
	}
	override := base()
	s.Same(override, overlayAccountAccessPolicy(nil, override), "no base takes the override")
	s.Same(override, overlayAccountAccessPolicy(override, nil), "no override keeps the base")

	for name, tc := range map[string]struct {
		override accountAccessPolicy
		edit     func(want *accountAccessPolicy)
	}{
		"nothing stated": {accountAccessPolicy{}, func(*accountAccessPolicy) {}},
		"an empty duration list": {accountAccessPolicy{LockDurationsSeconds: []int{}},
			func(*accountAccessPolicy) {}},
		"enabled": {accountAccessPolicy{Enabled: new(false)},
			func(want *accountAccessPolicy) { want.Enabled = new(false) }},
		"threshold": {accountAccessPolicy{Threshold: new(3)},
			func(want *accountAccessPolicy) { want.Threshold = new(3) }},
		"failureWindowSeconds": {accountAccessPolicy{FailureWindowSeconds: new(0)},
			func(want *accountAccessPolicy) { want.FailureWindowSeconds = new(0) }},
		"lockDurationsSeconds": {accountAccessPolicy{LockDurationsSeconds: []int{60, 0}},
			func(want *accountAccessPolicy) { want.LockDurationsSeconds = []int{60, 0} }},
		"lockDecaySeconds": {accountAccessPolicy{LockDecaySeconds: new(0)},
			func(want *accountAccessPolicy) { want.LockDecaySeconds = new(0) }},
		"maxConsecutiveFailures": {accountAccessPolicy{MaxConsecutiveFailures: new(0)},
			func(want *accountAccessPolicy) { want.MaxConsecutiveFailures = new(0) }},
	} {
		want := base()
		tc.edit(want)
		original := base()

		s.Equal(want, overlayAccountAccessPolicy(original, &tc.override), name)
		s.Equal(base(), original, "%s: the base is not changed", name)
	}
}

func (s *UtilsTestSuite) TestLockEmailToleratesMissingLevels() {
	var notifications *accountAccessNotifications
	s.Nil(notifications.lockEmail())
	s.Nil((&accountAccessNotifications{}).lockEmail())
}

func (s *UtilsTestSuite) TestPolicyIsEmpty() {
	s.True(accountAccessPolicy{}.isEmpty())
	s.False(accountAccessPolicy{Enabled: new(false)}.isEmpty())
	s.False(accountAccessPolicy{MaxConsecutiveFailures: new(0)}.isEmpty())
}

// A category that states nothing converts to nil, so it inherits rather than overriding.
func (s *UtilsTestSuite) TestAnEmptyCategoryConvertsToNil() {
	value := accountAccessValueFrom(sysconfig.AccountAccessConfig{})

	s.Nil(value.User)
	s.Nil(value.Agent)
	s.True(value.isEmpty())
}

func (s *UtilsTestSuite) TestTheDeploymentConfigurationConverts() {
	value := accountAccessValueFrom(sysconfig.AccountAccessConfig{User: sysconfig.CategoryAccessConfig{
		LockGranularity: "account",
		RecordLastLogin: new(true),
		Default: sysconfig.LockPolicyConfig{Threshold: new(5), LockDurationsSeconds: []int{60},
			MaxConsecutiveFailures: new(50)},
		Notifications: sysconfig.AccessNotificationsConfig{OnLock: sysconfig.LockNotificationConfig{
			Email: sysconfig.LockEmailConfig{Enabled: new(true)},
		}},
	}})

	s.Require().NotNil(value.User)
	s.Equal(granularityAccount, *value.User.LockGranularity)
	s.True(*value.User.RecordLastLogin)
	s.Equal(5, *value.User.Default.Threshold)
	s.Equal([]int{60}, value.User.Default.LockDurationsSeconds)
	s.Equal(50, *value.User.Default.MaxConsecutiveFailures)
	email := value.User.Notifications.lockEmail()
	s.Require().NotNil(email)
	s.True(*email.Enabled)
	s.Nil(value.Agent)
}

// A default that states nothing is left unset, so a lower layer's default is not replaced.
func (s *UtilsTestSuite) TestAnEmptyDefaultPolicyIsLeftUnset() {
	value := accountAccessValueFrom(sysconfig.AccountAccessConfig{User: sysconfig.CategoryAccessConfig{
		RecordLastLogin: new(false),
	}})

	s.Require().NotNil(value.User)
	s.Nil(value.User.Default)
	s.Nil(value.User.Notifications)
	s.Nil(value.User.LockGranularity)
}

func (s *UtilsTestSuite) TestLockEmailIsOffUnlessEnabled() {
	s.Nil(lockEmailFor(lockEmailConfig(accountAccessLockEmail{}), providers.EntityCategoryUser))
	s.Nil(lockEmailFor(accountAccessValue{}, providers.EntityCategoryUser))
	s.Nil(lockEmailFor(accountAccessValue{User: &accountAccessCategory{}}, providers.EntityCategoryUser))

	email := enabledLockEmail()
	s.Equal(&email, lockEmailFor(lockEmailConfig(email), providers.EntityCategoryUser))
}

// Agents and applications are not notified.
func (s *UtilsTestSuite) TestOnlyUsersAreNotified() {
	cfg := lockEmailConfig(enabledLockEmail())
	for _, category := range []providers.EntityCategory{providers.EntityCategoryAgent, providers.EntityCategoryApp} {
		s.Nil(lockEmailFor(cfg, category), "%s", category)
	}
}

// testLockTemplates supplies the default method pair and the account-wide pair.
func testLockTemplates() *accountAccessLockEmail {
	return &accountAccessLockEmail{Templates: &accountAccessLockTemplates{
		lockEmailTemplatePair: lockEmailTemplatePair{
			Temporary: "authentication-method-locked", Permanent: "authentication-method-locked-permanent"},
		Account: &lockEmailTemplatePair{Temporary: "account-locked", Permanent: "account-locked-permanent"},
	}}
}

// A lock that expires renders its expiring template with the method, its duration and its unlock time
// in UTC. The account-wide lock names no method.
func (s *UtilsTestSuite) TestATemporaryLockFillsItsFacts() {
	for method, want := range map[providers.AuthenticationMethod]struct{ handle, method string }{
		providers.AuthenticationMethodCredential: {"authentication-method-locked", "credential"},
		providers.AuthenticationMethodOTP:        {"authentication-method-locked", "otp"},
		AuthenticationMethodAccount:              {"account-locked", ""},
	} {
		handle, data := lockTemplateFor(testLockTemplates(), method, testLockEpisode())
		s.Equal(want.handle, handle, "method %s", method)
		s.Equal(map[string]string{
			templateKeyLockedAuthenticationMethod: want.method,
			templateKeyLockDuration:               "5 minutes",
			templateKeyUnlockTime:                 "2026-10-08 10:05",
		}, data, "method %s", method)
	}

	_, data := lockTemplateFor(testLockTemplates(), providers.AuthenticationMethodCredential,
		LockEpisode{Duration: time.Minute, UnlockAt: "2026-10-08T15:35:00+05:30"})
	s.Equal("2026-10-08 10:05", data[templateKeyUnlockTime], "an offset unlock time is written in UTC")
}

// The duration is the committed episode's, formatted like other template durations.
func (s *UtilsTestSuite) TestTheDurationIsTheCommittedEpisodes() {
	for duration, want := range map[time.Duration]string{
		15 * time.Minute: "15 minutes", time.Hour: "1 hour", 24 * time.Hour: "24 hours", 90 * time.Second: "90 seconds",
	} {
		_, data := lockTemplateFor(testLockTemplates(), providers.AuthenticationMethodCredential,
			LockEpisode{Duration: duration, UnlockAt: "2026-10-08T10:05:00Z"})
		s.Equal(want, data[templateKeyLockDuration])
	}
}

// A lock that does not expire, or whose unlock time cannot be read, renders the permanent template with
// no duration or unlock time. An override that sets only the expiring template keeps the default
// permanent one.
func (s *UtilsTestSuite) TestAPermanentLockRendersItsOwnTemplate() {
	for name, episode := range map[string]LockEpisode{
		"the permanent sentinel": {UnlockAt: permanentUnlockAt},
		"no duration":            {UnlockAt: "2026-10-08T10:05:00Z"},
		"an unreadable time":     {Duration: time.Minute, UnlockAt: "soon"},
	} {
		for method, want := range map[providers.AuthenticationMethod]string{
			providers.AuthenticationMethodCredential: "authentication-method-locked-permanent",
			providers.AuthenticationMethodOTP:        "authentication-method-locked-permanent",
			AuthenticationMethodAccount:              "account-locked-permanent",
		} {
			handle, data := lockTemplateFor(testLockTemplates(), method, episode)

			s.Equal(want, handle, "%s, method %s", name, method)
			s.Empty(data[templateKeyLockDuration], name)
			s.Empty(data[templateKeyUnlockTime], name)
		}
	}

	email := testLockTemplates()
	email.Templates.Account.Permanent = ""
	handle, _ := lockTemplateFor(email, AuthenticationMethodAccount, LockEpisode{UnlockAt: permanentUnlockAt})
	s.Equal("authentication-method-locked-permanent", handle, "an omitted account override keeps the default")
}

// A method's override replaces the defaults field by field.
func (s *UtilsTestSuite) TestAMethodOverrideReplacesTheDefaults() {
	email := testLockTemplates()
	email.Templates.Methods = map[string]lockEmailTemplatePair{
		string(providers.AuthenticationMethodOTP): {Temporary: "otp-locked", Permanent: "otp-locked-permanent"}}

	handle, data := lockTemplateFor(email, providers.AuthenticationMethodOTP, testLockEpisode())
	s.Equal("otp-locked", handle)
	s.Equal("otp", data[templateKeyLockedAuthenticationMethod])

	handle, _ = lockTemplateFor(email, providers.AuthenticationMethodOTP, LockEpisode{UnlockAt: permanentUnlockAt})
	s.Equal("otp-locked-permanent", handle)

	handle, _ = lockTemplateFor(email, providers.AuthenticationMethodCredential, testLockEpisode())
	s.Equal("authentication-method-locked", handle, "another method keeps the defaults")
}

// With no template set, the handle is empty and every placeholder is still filled.
func (s *UtilsTestSuite) TestNoTemplateLeavesTheHandleEmpty() {
	for name, email := range map[string]*accountAccessLockEmail{
		"no templates block": {},
		"an empty templates": {Templates: &accountAccessLockTemplates{}},
	} {
		handle, data := lockTemplateFor(email, providers.AuthenticationMethodCredential, testLockEpisode())

		s.Empty(handle, name)
		s.Len(data, 3, name)
	}
}

// readOf derives the management read of a hydrated entity with the given runtime attributes, with every
// recorded lock enforced.
func (s *UtilsTestSuite) readOf(state providers.EntityState, runtimeAttrs string) AccessRead {
	entity := providers.Entity{ID: "e1", Category: providers.EntityCategoryUser, State: state}
	if runtimeAttrs != "" {
		entity.RuntimeAttributes = json.RawMessage(runtimeAttrs)
	}
	return accessReadFor(&entity, lockEnforcement{}, s.now)
}

func (s *UtilsTestSuite) TestAccessReadReportsTheSuspensionWithItsNote() {
	read := s.readOf(providers.EntityStateSuspended, `{"accessState":{"suspend":`+
		`{"suspendedAt":"2026-09-10T11:02:00Z","operatorNote":"Credential exposure"},`+
		`"lastLoginAt":"2026-09-09T18:22:04Z"}}`)

	s.Equal(providers.AccessStatusSuspended, read.Status.Value)
	s.Require().NotNil(read.Status.Details)
	s.Equal("Credential exposure", read.Status.Details.OperatorNote)
	s.Equal("2026-09-09T18:22:04Z", read.LastLoginAt)
}

// The document holds while the column says ACTIVE; the column alone reports no circumstances.
func (s *UtilsTestSuite) TestAccessReadHoldsOnEitherSuspensionStore() {
	s.Equal(providers.AccessStatusSuspended, s.readOf(providers.EntityStateActive,
		`{"accessState":{"suspend":{"suspendedAt":"2026-09-10T11:02:00Z"}}}`).Status.Value)

	column := s.readOf(providers.EntityStateSuspended, `{"accessState":{"lastLoginAt":"2026-09-09T18:22:04Z"}}`)
	s.Equal(providers.AccessStatusSuspended, column.Status.Value)
	s.Require().NotNil(column.Status.Details)
	s.Empty(column.Status.Details.Since)
}

// The account-wide entry is not read as a method named "account".
func (s *UtilsTestSuite) TestAccessReadReportsTheAccountHoldAsLocked() {
	read := s.readOf(providers.EntityStateActive,
		`{"accessState":{"lock":{"account":{"unlockAt":"9999-12-31T23:59:59Z","reason":"POST_SUSPENDED"}}}}`)
	s.Equal(providers.AccessStatusLocked, read.Status.Value)
}

func (s *UtilsTestSuite) TestAccessReadReportsAnUnparsableDocumentAsSuspended() {
	read := s.readOf(providers.EntityStateActive, `not json`)
	s.Equal(providers.AccessStatusSuspended, read.Status.Value)
	s.Require().NotNil(read.Status.Details)
	s.Equal(providers.AccessHoldReasonAdministrativeSuspension, read.Status.Details.Reason)
}

func (s *UtilsTestSuite) TestAccessReadWithNothingRecordedIsActive() {
	s.Equal(providers.AccessStatusActive, s.readOf(providers.EntityStateActive, "").Status.Value)
	s.Equal(providers.AccessStatusActive, s.readOf(providers.EntityStateActive, `{"other":1}`).Status.Value)
}

// A lock is reported by the time the read is made at.
func (s *UtilsTestSuite) TestAccessReadReportsByTheGivenTime() {
	entity := providers.Entity{ID: "e1", State: providers.EntityStateActive, RuntimeAttributes: json.RawMessage(
		`{"accessState":{"lock":{"authenticationMethods":{"credential":{"unlockAt":"2026-09-08T10:05:00Z"}}}}}`)}

	s.Equal(providers.AccessStatusLocked, accessReadFor(&entity, lockEnforcement{}, s.now).Status.Value)
	s.Equal(providers.AccessStatusActive,
		accessReadFor(&entity, lockEnforcement{}, s.now.Add(10*time.Minute)).Status.Value, "the lock has ended")
}
