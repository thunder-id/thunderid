// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernance

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type PolicyTestSuite struct {
	suite.Suite
}

func TestPolicyTestSuite(t *testing.T) {
	suite.Run(t, new(PolicyTestSuite))
}

// testAccountAccessConfig is a representative deployment: a user default and agent locking switched
// off.
func testAccountAccessConfig() accountAccessValue {
	return accountAccessValue{
		User: &accountAccessCategory{
			LockGranularity: new("authentication_method"),
			Default: &accountAccessPolicy{
				Enabled:              new(true),
				Threshold:            new(5),
				FailureWindowSeconds: new(900),
				LockDurationsSeconds: []int{300, 900, 3600, 86400},
				LockDecaySeconds:     new(86400),
			},
		},
		Agent: &accountAccessCategory{
			LockGranularity: new("authentication_method"),
			Default:         &accountAccessPolicy{Enabled: new(false)},
		},
	}
}

// newTestPolicyResolver resolves policies from a fixture, through a source with no server-config
// reader.
func newTestPolicyResolver(cfg accountAccessValue) policyResolverInterface {
	return newPolicyResolver(newAccountAccessSource(cfg, nil))
}

func (s *PolicyTestSuite) resolver() policyResolverInterface {
	return newTestPolicyResolver(testAccountAccessConfig())
}

func entityOf(category providers.EntityCategory) GovernedEntity {
	return GovernedEntity{ID: "e1", Category: category, State: providers.EntityStateActive}
}

// The configured policy is translated whole, with seconds becoming durations.
func (s *PolicyTestSuite) TestResolvesTheConfiguredPolicy() {
	policy := s.resolver().Resolve(
		context.Background(), entityOf(providers.EntityCategoryUser), providers.AccessScopeCredential)

	s.True(policy.enabled)
	s.Equal(5, policy.threshold)
	s.Equal(15*time.Minute, policy.failureWindow)
	s.Equal([]time.Duration{
		5 * time.Minute, 15 * time.Minute, time.Hour, 24 * time.Hour,
	}, policy.lockDurations)
	s.Equal(24*time.Hour, policy.lockDecay)
	s.Equal(granularityAuthenticationMethod, policy.granularity)
}

// Each category resolves its own policy; only the entity and the scope are inputs.
func (s *PolicyTestSuite) TestResolvesPerCategoryAndNothingElse() {
	resolver := s.resolver()

	user := resolver.Resolve(
		context.Background(), entityOf(providers.EntityCategoryUser), providers.AccessScopeCredential)
	agent := resolver.Resolve(
		context.Background(), entityOf(providers.EntityCategoryAgent), providers.AccessScopeCredential)

	s.True(user.enabled)
	s.False(agent.enabled, "this deployment explicitly disables agent locking")
}

func (s *PolicyTestSuite) TestUnconfiguredCategoryLocksNothing() {
	policy := s.resolver().Resolve(
		context.Background(), entityOf(providers.EntityCategoryApp), providers.AccessScopeCredential)

	s.False(policy.enabled)
	s.Equal(granularityAuthenticationMethod, policy.granularity,
		"and still defaults to per-authentication_method")
}

// withOTPOverride is the test configuration with a tighter one-time password policy that states
// only what differs from the user default.
func withOTPOverride() accountAccessValue {
	cfg := testAccountAccessConfig()
	cfg.User.Scopes = map[string]accountAccessPolicy{
		string(providers.AccessScopeOTP): {
			Threshold:            new(3),
			FailureWindowSeconds: new(600),
			LockDurationsSeconds: []int{300, 900},
		},
	}
	return cfg
}

// A method override is overlaid on the category default field by field.
func (s *PolicyTestSuite) TestAnOverrideIsOverlaidOnTheDefault() {
	otp := newTestPolicyResolver(withOTPOverride()).Resolve(
		context.Background(), entityOf(providers.EntityCategoryUser), providers.AccessScopeOTP)

	s.True(otp.enabled)
	s.Equal(3, otp.threshold, "the overridden threshold")
	s.Equal(10*time.Minute, otp.failureWindow, "the overridden window")
	s.Equal([]time.Duration{5 * time.Minute, 15 * time.Minute}, otp.lockDurations)
	s.Equal(24*time.Hour, otp.lockDecay, "the decay the override omits comes from the default")
}

// A method no override names gets the category default.
func (s *PolicyTestSuite) TestAnUnnamedMethodGetsTheDefault() {
	resolver := newTestPolicyResolver(withOTPOverride())
	user := entityOf(providers.EntityCategoryUser)

	credential := resolver.Resolve(context.Background(), user, providers.AccessScopeCredential)
	s.True(credential.enabled)
	s.Equal(5, credential.threshold)
	s.Equal(15*time.Minute, credential.failureWindow)
	s.Len(credential.lockDurations, 4)
}

// An override may switch one method off while the rest of the category stays protected.
func (s *PolicyTestSuite) TestAnOverrideMaySwitchOneMethodOff() {
	cfg := testAccountAccessConfig()
	cfg.User.Scopes = map[string]accountAccessPolicy{
		string(providers.AccessScopeOTP): {Enabled: new(false)},
	}
	resolver := newTestPolicyResolver(cfg)
	user := entityOf(providers.EntityCategoryUser)

	s.False(resolver.Resolve(context.Background(), user, providers.AccessScopeOTP).enabled)
	s.True(resolver.Resolve(context.Background(), user, providers.AccessScopeCredential).enabled,
		"switching one method off must not switch the category off")
}

// Under entity granularity every method resolves the entity scope's policy, not its own.
func (s *PolicyTestSuite) TestEntityGranularityUsesTheEntityScopePolicy() {
	cfg := testAccountAccessConfig()
	cfg.User.LockGranularity = new(string(granularityEntity))
	cfg.User.Scopes = map[string]accountAccessPolicy{
		string(AccessScopeEntity):        {Threshold: new(7)},
		string(providers.AccessScopeOTP): {Threshold: new(2)},
	}

	otp := newTestPolicyResolver(cfg).Resolve(
		context.Background(), entityOf(providers.EntityCategoryUser), providers.AccessScopeOTP)

	s.Equal(7, otp.threshold, "the entity scope's override applies, not the method's")
	s.Equal(15*time.Minute, otp.failureWindow, "over the category default")
	s.Equal(granularityEntity, otp.granularity)
}

// Zero is a value, not an omission: a zero decay never decays and a zero duration is permanent.
func (s *PolicyTestSuite) TestZeroIsAValueRatherThanAnOmission() {
	cfg := testAccountAccessConfig()
	cfg.User.Default.LockDecaySeconds = new(0)
	cfg.User.Default.LockDurationsSeconds = []int{300, 0}

	policy := newTestPolicyResolver(cfg).Resolve(
		context.Background(), entityOf(providers.EntityCategoryUser), providers.AccessScopeCredential)

	s.Equal(time.Duration(0), policy.lockDecay, "an explicit zero decay is kept")
	s.Equal([]time.Duration{5 * time.Minute, 0}, policy.lockDurations)
}

func (s *PolicyTestSuite) TestPolicyThatCannotLockIsNotEnabled() {
	user := entityOf(providers.EntityCategoryUser)
	cfg := testAccountAccessConfig()
	cfg.User.Default.Threshold = new(0)
	noThreshold := newTestPolicyResolver(cfg).Resolve(context.Background(), user,
		providers.AccessScopeCredential)

	cfg = testAccountAccessConfig()
	cfg.User.Default.LockDurationsSeconds = nil
	noDurations := newTestPolicyResolver(cfg).Resolve(context.Background(), user,
		providers.AccessScopeCredential)

	s.False(noThreshold.enabled, "a threshold of zero cannot form a lock")
	s.False(noDurations.enabled, "an empty escalation cannot form a lock")
}

// An omitted enabled flag keeps the default, which is on. A nil pointer means "not set".
func (s *PolicyTestSuite) TestOmittedEnabledFlagDefaultsToOn() {
	cfg := testAccountAccessConfig()
	cfg.User.Default.Enabled = nil

	policy := newTestPolicyResolver(cfg).Resolve(
		context.Background(), entityOf(providers.EntityCategoryUser), providers.AccessScopeCredential)
	s.True(policy.enabled)
}

// granularity defaults to per-method, and an unrecognized value takes the default.
func (s *PolicyTestSuite) TestGranularityDefaultsToAuthenticationMethod() {
	s.Equal(granularityAuthenticationMethod, parseGranularity(""))
	s.Equal(granularityAuthenticationMethod, parseGranularity("nonsense"))
	s.Equal(granularityAuthenticationMethod, parseGranularity("authentication_method"))
	s.Equal(granularityEntity, parseGranularity("entity"))
}

// A zero duration is kept as the permanent lock rather than dropped.
func (s *PolicyTestSuite) TestZeroDurationSurvivesTranslation() {
	s.Equal([]time.Duration{5 * time.Minute, 0}, toDurations([]int{300, 0}))
	s.Nil(toDurations(nil))
	s.Equal([]time.Duration{5 * time.Minute}, toDurations([]int{-1, 300}),
		"a negative duration is dropped")
}

// An oversized seconds value is clamped rather than wrapping to a negative duration.
func (s *PolicyTestSuite) TestADurationTooLargeToCarryIsClampedRatherThanWrapped() {
	const overflowing = maxDurationSeconds + 1

	cfg := testAccountAccessConfig()
	cfg.User.Default.FailureWindowSeconds = new(overflowing)
	cfg.User.Default.LockDecaySeconds = new(overflowing)
	cfg.User.Default.LockDurationsSeconds = []int{overflowing}
	cfg.User.RecordLastLogin = new(true)
	cfg.User.LastLoginResolutionSeconds = new(overflowing)

	resolver := newTestPolicyResolver(cfg)
	policy := resolver.Resolve(
		context.Background(), entityOf(providers.EntityCategoryUser), providers.AccessScopeCredential)

	ceiling := time.Duration(maxDurationSeconds) * time.Second
	s.Equal(ceiling, policy.failureWindow)
	s.Equal(ceiling, policy.lockDecay)
	s.Equal([]time.Duration{ceiling}, policy.lockDurations)
	s.True(policy.enabled, "a window that wrapped would have switched the policy off")
	s.Equal(ceiling,
		resolver.ActivityPolicies(context.Background()).forCategory(providers.EntityCategoryUser).resolution)
}

// A window of zero keeps the policy on and never resets the counter on time.
func (s *PolicyTestSuite) TestAZeroWindowKeepsThePolicyEnabled() {
	cfg := testAccountAccessConfig()
	cfg.User.Default.FailureWindowSeconds = new(0)

	policy := newTestPolicyResolver(cfg).Resolve(
		context.Background(), entityOf(providers.EntityCategoryUser), providers.AccessScopeCredential)

	s.True(policy.enabled)
	s.Zero(policy.failureWindow)
}

// A policy with no window at all cannot form a lock and is off.
func (s *PolicyTestSuite) TestAnAbsentWindowDisablesThePolicy() {
	cfg := testAccountAccessConfig()
	cfg.User.Default.FailureWindowSeconds = nil

	policy := newTestPolicyResolver(cfg).Resolve(
		context.Background(), entityOf(providers.EntityCategoryUser), providers.AccessScopeCredential)

	s.False(policy.enabled)
}

// A category the configuration does not mention resolves the same as one switched off, for every scope.
func (s *PolicyTestSuite) TestAbsentCategoryResolvesAsSwitchedOff() {
	off := false
	stubbed := accountAccessValue{
		Agent: &accountAccessCategory{
			LockGranularity: new(string(granularityAuthenticationMethod)),
			Default:         &accountAccessPolicy{Enabled: &off},
		},
	}
	absent := accountAccessValue{}

	for _, category := range []providers.EntityCategory{
		providers.EntityCategoryAgent, providers.EntityCategoryApp,
	} {
		entity := GovernedEntity{ID: "e1", Category: category, State: providers.EntityStateActive}

		for _, scope := range AllAccessScopes {
			ctx := context.Background()
			withStub := newTestPolicyResolver(stubbed).Resolve(ctx, entity, scope)
			withNone := newTestPolicyResolver(absent).Resolve(ctx, entity, scope)

			s.Equal(withStub, withNone,
				"category %s scope %s must resolve identically with and without the stub",
				category, scope)
			s.False(withNone.enabled,
				"category %s scope %s must resolve disabled", category, scope)
			s.Equal(granularityAuthenticationMethod, withNone.granularity,
				"an unconfigured category takes per-authentication_method granularity")
		}
	}
}

// Every policy discloses the hold; the mode is fixed in code, not read from configuration.
func (s *PolicyTestSuite) TestEveryPolicyDisclosesTheHold() {
	resolver := s.resolver()
	ctx := context.Background()

	for _, category := range []providers.EntityCategory{
		providers.EntityCategoryUser, providers.EntityCategoryAgent, providers.EntityCategoryApp,
	} {
		for _, scope := range AllAccessScopes {
			policy := resolver.Resolve(ctx, entityOf(category), scope)
			s.Equal(discloseAlways, policy.discloseAccountHold, "category %s scope %s", category, scope)
		}
	}
}

// Activity recording is resolved per category, separately from the lockout policy.
func (s *PolicyTestSuite) TestActivityPolicyIsResolvedPerCategory() {
	cfg := testAccountAccessConfig()
	cfg.User.RecordLastLogin = new(true)
	cfg.User.LastLoginResolutionSeconds = new(300)
	cfg.Agent.RecordLastLogin = new(false)
	resolver := newTestPolicyResolver(cfg)
	ctx := context.Background()

	user := resolver.ActivityPolicies(ctx).forCategory(providers.EntityCategoryUser)
	s.True(user.record)
	s.Equal(5*time.Minute, user.resolution)

	agent := resolver.ActivityPolicies(ctx).forCategory(providers.EntityCategoryAgent)
	s.False(agent.record, "a category may record nothing while its sibling records")
}

func (s *PolicyTestSuite) TestActivityRecordingIsOffWhenUnstated() {
	resolver := newTestPolicyResolver(testAccountAccessConfig())

	activity := resolver.ActivityPolicies(context.Background()).forCategory(providers.EntityCategoryUser)

	s.False(activity.record)
	s.Equal(time.Hour, activity.resolution,
		"the floor still has a default, so switching recording on alone cannot mean a write per sign-in")
}

// Resolution edges: zero writes every time, and a negative value clamps to zero.
func (s *PolicyTestSuite) TestActivityResolutionEdges() {
	cfg := testAccountAccessConfig()
	cfg.User.RecordLastLogin = new(true)

	cfg.User.LastLoginResolutionSeconds = new(0)
	activity := newTestPolicyResolver(cfg).ActivityPolicies(context.Background()).
		forCategory(providers.EntityCategoryUser)
	s.Zero(activity.resolution)

	cfg.User.LastLoginResolutionSeconds = new(-60)
	activity = newTestPolicyResolver(cfg).ActivityPolicies(context.Background()).
		forCategory(providers.EntityCategoryUser)
	s.Zero(activity.resolution,
		"a negative floor writes every time rather than never")
}

// activityPolicies is resolved from configuration alone, with no entity read.
func (s *PolicyTestSuite) TestActivityPoliciesNeedNoEntity() {
	cfg := testAccountAccessConfig()
	s.False(newTestPolicyResolver(cfg).ActivityPolicies(context.Background()).enabled(),
		"recording off must not need an entity read")

	cfg.Agent.RecordLastLogin = new(true)
	s.True(newTestPolicyResolver(cfg).ActivityPolicies(context.Background()).enabled(),
		"one category recording is enough to reach the read that decides the rest")
}

// The zero value enforces every recorded lock.
func (s *PolicyTestSuite) TestTheZeroValueEnforcesEveryLock() {
	var enforcement lockEnforcement
	for _, scope := range AllAccessScopes {
		s.True(enforcement.enforces(scope), "%s", scope)
	}
}

// Under entity granularity, entries under authentication methods are not enforced.
func (s *PolicyTestSuite) TestEntityGranularityUnenforcesEveryMethodScope() {
	cfg := testAccountAccessConfig()
	cfg.User.LockGranularity = new(string(granularityEntity))

	enforcement := lockEnforcementFor(cfg, providers.EntityCategoryUser)

	s.False(enforcement.enforces(providers.AccessScopeCredential))
	s.False(enforcement.enforces(providers.AccessScopeOTP))
	s.True(enforcement.enforces(AccessScopeEntity), "the entity entry is always enforced")
}

// A method whose policy is switched off no longer enforces its recorded lock.
func (s *PolicyTestSuite) TestADisabledMethodPolicyIsNotEnforced() {
	cfg := testAccountAccessConfig()
	cfg.User.Scopes = map[string]accountAccessPolicy{
		string(providers.AccessScopeOTP): {Enabled: new(false)},
	}

	enforcement := lockEnforcementFor(cfg, providers.EntityCategoryUser)

	s.False(enforcement.enforces(providers.AccessScopeOTP))
	s.True(enforcement.enforces(providers.AccessScopeCredential))
	s.True(enforcement.enforces(AccessScopeEntity))
}

// With locking off for a category, no failure lock is enforced, the entity entry included.
func (s *PolicyTestSuite) TestADisabledCategoryEnforcesNoFailureLock() {
	enforcement := lockEnforcementFor(testAccountAccessConfig(), providers.EntityCategoryAgent)

	s.False(enforcement.enforces(providers.AccessScopeCredential))
	s.False(enforcement.enforces(AccessScopeEntity))
}
