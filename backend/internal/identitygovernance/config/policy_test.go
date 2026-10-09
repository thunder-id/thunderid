// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"context"
	"testing"
	"time"

	"github.com/thunder-id/thunderid/internal/identitygovernance/model"

	"github.com/stretchr/testify/suite"

	sysconfig "github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type PolicyTestSuite struct {
	suite.Suite
}

func TestPolicyTestSuite(t *testing.T) {
	suite.Run(t, new(PolicyTestSuite))
}

func boolPtr(v bool) *bool { return &v }
func intPtr(v int) *int    { return &v }

// testAccountAccessConfig is a representative deployment: a user default, a tighter OTP override,
// and agent locking switched off. The password uses the category default.
func testAccountAccessConfig() AccountAccessValue {
	return AccountAccessValue{
		User: &AccountAccessCategory{
			LockGranularity: stringPtr("authentication_method"),
			Default: &AccountAccessPolicy{
				Enabled:              boolPtr(true),
				Threshold:            intPtr(5),
				FailureWindowSeconds: intPtr(900),
				LockDurationsSeconds: []int{300, 900, 3600, 86400},
				LockDecaySeconds:     intPtr(86400),
			},
			Scopes: map[string]AccountAccessPolicy{
				// States only what differs from the default.
				string(model.AccessScopeOTP): {
					Threshold:            intPtr(3),
					FailureWindowSeconds: intPtr(600),
					LockDurationsSeconds: []int{300, 900},
					LockDecaySeconds:     intPtr(3600),
				},
			},
		},
		Agent: &AccountAccessCategory{
			LockGranularity: stringPtr("authentication_method"),
			Default:         &AccountAccessPolicy{Enabled: boolPtr(false)},
		},
	}
}

// newTestPolicyResolver reads a fixture through the server-config source.
func newTestPolicyResolver(cfg AccountAccessValue) *policyResolver {
	source := NewAccountAccessSource(sysconfig.AccountAccessConfig{}, log.GetLogger())
	source.install(&stubConfigReader{value: cfg})
	return NewPolicyResolver(source)
}

func (s *PolicyTestSuite) resolver() *policyResolver {
	return newTestPolicyResolver(testAccountAccessConfig())
}

func entityOf(category providers.EntityCategory) model.GovernedEntity {
	return model.GovernedEntity{ID: "e1", Category: category, State: providers.EntityStateActive}
}

// The configured policy is translated whole, with seconds becoming durations.
func (s *PolicyTestSuite) TestResolvesTheConfiguredPolicy() {
	policy := s.resolver().Resolve(
		context.Background(), entityOf(providers.EntityCategoryUser), model.AccessScopeCredential)

	s.True(policy.Enabled)
	s.Equal(5, policy.Threshold)
	s.Equal(15*time.Minute, policy.FailureWindow)
	s.Equal([]time.Duration{
		5 * time.Minute, 15 * time.Minute, time.Hour, 24 * time.Hour,
	}, policy.LockDurations)
	s.Equal(24*time.Hour, policy.LockDecay)
	s.Equal(GranularityAuthenticationMethod, policy.Granularity)
}

// A one-time password resolves a tighter policy than a password.
func (s *PolicyTestSuite) TestResolvesPerScope() {
	resolver := s.resolver()
	user := entityOf(providers.EntityCategoryUser)

	credential := resolver.Resolve(context.Background(), user, model.AccessScopeCredential)
	otp := resolver.Resolve(context.Background(), user, model.AccessScopeOTP)

	s.Equal(5, credential.Threshold)
	s.Equal(3, otp.Threshold, "the one-time password scope carries its own threshold")
	s.Equal(10*time.Minute, otp.FailureWindow)
	s.Len(otp.LockDurations, 2)
}

// Each category resolves its own policy; only the entity and the scope are inputs.
func (s *PolicyTestSuite) TestResolvesPerCategoryAndNothingElse() {
	resolver := s.resolver()

	user := resolver.Resolve(
		context.Background(), entityOf(providers.EntityCategoryUser), model.AccessScopeCredential)
	agent := resolver.Resolve(
		context.Background(), entityOf(providers.EntityCategoryAgent), model.AccessScopeCredential)

	s.True(user.Enabled)
	s.False(agent.Enabled, "this deployment explicitly disables agent locking")
}

func (s *PolicyTestSuite) TestUnconfiguredCategoryLocksNothing() {
	policy := s.resolver().Resolve(
		context.Background(), entityOf(providers.EntityCategoryApp), model.AccessScopeCredential)

	s.False(policy.Enabled)
	s.Equal(GranularityAuthenticationMethod, policy.Granularity,
		"and still defaults to per-authentication_method")
}

func (s *PolicyTestSuite) TestUnconfiguredScopeInheritsTheCategoryDefault() {
	resolver := s.resolver()
	user := entityOf(providers.EntityCategoryUser)

	// The password has no block at all in the test configuration.
	credential := resolver.Resolve(context.Background(), user, model.AccessScopeCredential)
	s.True(credential.Enabled)
	s.Equal(5, credential.Threshold)
	s.Equal(15*time.Minute, credential.FailureWindow)

	// A non-lockable method still resolves a policy; its class prevents locking.
	passkey := resolver.Resolve(context.Background(), user, model.AccessScopePasskey)
	s.True(passkey.Enabled, "the policy is inherited")
	s.Equal(model.ClassNotLockable, model.ClassOf(model.AccessScopePasskey),
		"and the class is what keeps it from ever locking")
}

func (s *PolicyTestSuite) TestAnOverrideInheritsTheFieldsItOmits() {
	cfg := testAccountAccessConfig()
	cfg.User.Scopes[string(model.AccessScopeOTP)] = AccountAccessPolicy{
		Threshold: intPtr(2),
	}

	policy := newTestPolicyResolver(cfg).Resolve(
		context.Background(), entityOf(providers.EntityCategoryUser), model.AccessScopeOTP)

	s.Equal(2, policy.Threshold, "the overridden field")
	s.Equal(15*time.Minute, policy.FailureWindow, "and everything else from the default")
	s.Equal(24*time.Hour, policy.LockDecay)
	s.Len(policy.LockDurations, 4)
}

// An override may switch one authentication method off while the rest of the category stays protected.
func (s *PolicyTestSuite) TestAnOverrideMaySwitchOneScopeOff() {
	cfg := testAccountAccessConfig()
	cfg.User.Scopes[string(model.AccessScopeOTP)] = AccountAccessPolicy{
		Enabled: boolPtr(false),
	}
	resolver := newTestPolicyResolver(cfg)
	user := entityOf(providers.EntityCategoryUser)

	otp := resolver.Resolve(context.Background(), user, model.AccessScopeOTP)
	credential := resolver.Resolve(context.Background(), user, model.AccessScopeCredential)

	s.False(otp.Enabled)
	s.True(credential.Enabled, "switching one authentication_method off must not switch the category off")
}

// Zero is a value, not an omission: a zero decay never decays and a zero duration is permanent.
func (s *PolicyTestSuite) TestZeroIsAValueRatherThanAnOmission() {
	cfg := testAccountAccessConfig()
	cfg.User.Scopes[string(model.AccessScopeOTP)] = AccountAccessPolicy{
		LockDecaySeconds:     intPtr(0),
		LockDurationsSeconds: []int{300, 0},
	}

	policy := newTestPolicyResolver(cfg).Resolve(
		context.Background(), entityOf(providers.EntityCategoryUser), model.AccessScopeOTP)

	s.Equal(time.Duration(0), policy.LockDecay, "an explicit zero decay must not inherit 24h")
	s.Equal([]time.Duration{5 * time.Minute, 0}, policy.LockDurations)
}

func (s *PolicyTestSuite) TestPolicyThatCannotLockIsNotEnabled() {
	cfg := testAccountAccessConfig()
	cfg.User.Default = &AccountAccessPolicy{Enabled: boolPtr(true)}
	cfg.User.Scopes["no-threshold"] = AccountAccessPolicy{
		Threshold:            intPtr(0),
		LockDurationsSeconds: []int{300},
	}
	cfg.User.Scopes["no-durations"] = AccountAccessPolicy{
		Threshold: intPtr(5),
	}
	resolver := newTestPolicyResolver(cfg)
	user := entityOf(providers.EntityCategoryUser)

	noThreshold := resolver.Resolve(context.Background(), user, "no-threshold")
	noDurations := resolver.Resolve(context.Background(), user, "no-durations")

	s.False(noThreshold.Enabled, "a threshold of zero cannot form a lock")
	s.False(noDurations.Enabled, "an empty escalation cannot form a lock")
}

// An omitted enabled flag keeps the default, which is on. A nil pointer means "not set".
func (s *PolicyTestSuite) TestOmittedEnabledFlagDefaultsToOn() {
	cfg := testAccountAccessConfig()
	cfg.User.Default.Enabled = nil

	policy := newTestPolicyResolver(cfg).Resolve(
		context.Background(), entityOf(providers.EntityCategoryUser), model.AccessScopeCredential)
	s.True(policy.Enabled)
}

// Granularity defaults to per-method, and an unrecognized value takes the default.
func (s *PolicyTestSuite) TestGranularityDefaultsToAuthenticationMethod() {
	s.Equal(GranularityAuthenticationMethod, parseGranularity(""))
	s.Equal(GranularityAuthenticationMethod, parseGranularity("nonsense"))
	s.Equal(GranularityAuthenticationMethod, parseGranularity("authentication_method"))
	s.Equal(GranularityEntity, parseGranularity("entity"))
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
	cfg.User.Default.FailureWindowSeconds = intPtr(overflowing)
	cfg.User.Default.LockDecaySeconds = intPtr(overflowing)
	cfg.User.Default.LockDurationsSeconds = []int{overflowing}
	cfg.User.RecordLastLogin = boolPtr(true)
	cfg.User.LastLoginResolutionSeconds = intPtr(overflowing)

	resolver := newTestPolicyResolver(cfg)
	policy := resolver.Resolve(
		context.Background(), entityOf(providers.EntityCategoryUser), model.AccessScopeCredential)

	ceiling := time.Duration(maxDurationSeconds) * time.Second
	s.Equal(ceiling, policy.FailureWindow)
	s.Equal(ceiling, policy.LockDecay)
	s.Equal([]time.Duration{ceiling}, policy.LockDurations)
	s.True(policy.Enabled, "a window that wrapped would have switched the policy off")
	s.Equal(ceiling, resolver.ActivityPolicies(context.Background()).For(providers.EntityCategoryUser).Resolution)
}

// A category the configuration does not mention resolves the same as one switched off, for every scope.
func (s *PolicyTestSuite) TestAbsentCategoryResolvesAsSwitchedOff() {
	off := false
	stubbed := AccountAccessValue{
		Agent: &AccountAccessCategory{
			LockGranularity: stringPtr(string(GranularityAuthenticationMethod)),
			Default:         &AccountAccessPolicy{Enabled: &off},
		},
	}
	absent := AccountAccessValue{}

	for _, category := range []providers.EntityCategory{
		providers.EntityCategoryAgent, providers.EntityCategoryApp,
	} {
		entity := model.GovernedEntity{ID: "e1", Category: category, State: providers.EntityStateActive}

		for _, scope := range model.AllAccessScopes {
			ctx := context.Background()
			withStub := newTestPolicyResolver(stubbed).Resolve(ctx, entity, scope)
			withNone := newTestPolicyResolver(absent).Resolve(ctx, entity, scope)

			s.Equal(withStub, withNone,
				"category %s scope %s must resolve identically with and without the stub",
				category, scope)
			s.False(withNone.Enabled,
				"category %s scope %s must resolve disabled", category, scope)
			s.Equal(GranularityAuthenticationMethod, withNone.Granularity,
				"an unconfigured category takes per-authentication_method granularity")
			s.Equal(DiscloseNever, withNone.DiscloseAccountHold,
				"an unconfigured category discloses nothing")
		}
	}
}

// Activity recording is resolved per category, separately from the lockout policy.
func (s *PolicyTestSuite) TestActivityPolicyIsResolvedPerCategory() {
	cfg := testAccountAccessConfig()
	cfg.User.RecordLastLogin = boolPtr(true)
	cfg.User.LastLoginResolutionSeconds = intPtr(300)
	cfg.Agent.RecordLastLogin = boolPtr(false)
	resolver := newTestPolicyResolver(cfg)
	ctx := context.Background()

	user := resolver.ActivityPolicies(ctx).For(providers.EntityCategoryUser)
	s.True(user.Record)
	s.Equal(5*time.Minute, user.Resolution)

	agent := resolver.ActivityPolicies(ctx).For(providers.EntityCategoryAgent)
	s.False(agent.Record, "a category may record nothing while its sibling records")
}

func (s *PolicyTestSuite) TestActivityRecordingIsOffWhenUnstated() {
	resolver := newTestPolicyResolver(testAccountAccessConfig())

	activity := resolver.ActivityPolicies(context.Background()).For(providers.EntityCategoryUser)

	s.False(activity.Record)
	s.Equal(time.Hour, activity.Resolution,
		"the floor still has a default, so switching recording on alone cannot mean a write per sign-in")
}

// Resolution edges: zero writes every time, and a negative value clamps to zero.
func (s *PolicyTestSuite) TestActivityResolutionEdges() {
	cfg := testAccountAccessConfig()
	cfg.User.RecordLastLogin = boolPtr(true)

	cfg.User.LastLoginResolutionSeconds = intPtr(0)
	activity := newTestPolicyResolver(cfg).ActivityPolicies(context.Background()).
		For(providers.EntityCategoryUser)
	s.Zero(activity.Resolution)

	cfg.User.LastLoginResolutionSeconds = intPtr(-60)
	activity = newTestPolicyResolver(cfg).ActivityPolicies(context.Background()).For(providers.EntityCategoryUser)
	s.Zero(activity.Resolution,
		"a negative floor writes every time rather than never")
}

// Disclosure is resolved per category and per method.
func (s *PolicyTestSuite) TestDisclosureIsResolvedPerCategory() {
	always, never := string(DiscloseAlways), string(DiscloseNever)
	cfg := testAccountAccessConfig()
	cfg.User.Default.DiscloseAccountHold = &never
	cfg.Agent.Default.DiscloseAccountHold = &always
	resolver := newTestPolicyResolver(cfg)
	ctx := context.Background()

	user := resolver.Resolve(ctx, entityOf(providers.EntityCategoryUser),
		model.AccessScopeCredential)
	agent := resolver.Resolve(ctx, entityOf(providers.EntityCategoryAgent),
		model.AccessScopeCredential)

	s.Equal(DiscloseNever, user.DiscloseAccountHold)
	s.Equal(DiscloseAlways, agent.DiscloseAccountHold,
		"one category may disclose while the other does not")

	// Per-method override.
	cfg.User.Scopes[string(model.AccessScopeOTP)] = AccountAccessPolicy{
		DiscloseAccountHold: &always,
	}
	otp := newTestPolicyResolver(cfg).Resolve(ctx, entityOf(providers.EntityCategoryUser),
		model.AccessScopeOTP)
	s.Equal(DiscloseAlways, otp.DiscloseAccountHold)
}

// ActivityPolicies is resolved from configuration alone, with no entity read.
func (s *PolicyTestSuite) TestActivityPoliciesNeedNoEntity() {
	cfg := testAccountAccessConfig()
	s.False(newTestPolicyResolver(cfg).ActivityPolicies(context.Background()).Enabled(),
		"recording off must not need an entity read")

	cfg.Agent.RecordLastLogin = boolPtr(true)
	s.True(newTestPolicyResolver(cfg).ActivityPolicies(context.Background()).Enabled(),
		"one category recording is enough to reach the read that decides the rest")
}
