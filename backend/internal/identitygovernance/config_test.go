// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernance

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/serverconfig"
	sysconfig "github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/serverconfigmock"
)

type ConfigTestSuite struct {
	suite.Suite
	reader *serverconfigmock.ServerConfigServiceMock
}

func TestConfigTestSuite(t *testing.T) {
	suite.Run(t, new(ConfigTestSuite))
}

func (s *ConfigTestSuite) SetupTest() {
	sysconfig.ResetServerRuntime()
	s.reader = serverconfigmock.NewServerConfigServiceMock(s.T())
}

func (s *ConfigTestSuite) TearDownTest() {
	sysconfig.ResetServerRuntime()
}

func (s *ConfigTestSuite) section(raw string) accountAccessValue {
	value, err := accountAccessHandler{}.Decode(json.RawMessage(raw))
	s.Require().NoError(err)
	return value.(accountAccessValue)
}

// readingSource builds a source over base that reads the section through the suite's reader.
func (s *ConfigTestSuite) readingSource(base accountAccessValue) accountAccessSourceInterface {
	return newAccountAccessSource(base, s.reader)
}

// serves makes the reader return value for the accountAccess section.
func (s *ConfigTestSuite) serves(
	value any) *serverconfigmock.ServerConfigServiceMock_GetMergedConfig_Call {
	return s.reader.EXPECT().GetMergedConfig(mock.Anything, string(serverconfig.ConfigNameAccountAccess)).
		Return(value, nil)
}

// userThreshold returns the user default threshold the source serves.
func userThreshold(source accountAccessSourceInterface) int {
	return *source.Current(context.Background()).User.Default.Threshold
}

// deploymentBase is a deployment policy with a threshold of 4.
func deploymentBase() accountAccessValue {
	return accountAccessValue{User: &accountAccessCategory{
		Default: &accountAccessPolicy{
			Enabled: new(true), Threshold: new(4), FailureWindowSeconds: new(900),
			LockDurationsSeconds: []int{300},
		},
	}}
}

func (s *ConfigTestSuite) TestTheReaderSuppliesThePolicy() {
	s.serves(s.section(`{"user":{"default":{"enabled":true,"threshold":3,"failureWindowSeconds":600,` +
		`"lockDurationsSeconds":[60]}}}`))

	s.Equal(3, userThreshold(s.readingSource(deploymentBase())))
}

// A failed read falls back to the deployment policy, not to the last value read.
func (s *ConfigTestSuite) TestAFailedReadFallsBackToTheDeploymentPolicy() {
	source := s.readingSource(deploymentBase())
	s.serves(s.section(`{"user":{"default":{"threshold":2}}}`)).Once()
	s.Equal(2, userThreshold(source))

	s.reader.EXPECT().GetMergedConfig(mock.Anything, mock.Anything).Return(nil, &common.InternalServerError).Once()

	s.Equal(4, userThreshold(source))
}

// The section is read on every call, so a write served by another node is seen.
func (s *ConfigTestSuite) TestTheSectionIsReadOnEveryCall() {
	source := s.readingSource(accountAccessValue{})
	s.serves(s.section(`{"user":{"default":{"threshold":4}}}`)).Once()
	s.serves(s.section(`{"user":{"default":{"threshold":2}}}`)).Once()

	s.Equal(4, userThreshold(source))
	s.Equal(2, userThreshold(source))
	s.reader.AssertNumberOfCalls(s.T(), "GetMergedConfig", 2)
}

// A wrong-typed value is handled like a failed read.
func (s *ConfigTestSuite) TestAnUnexpectedSectionTypeFallsBackToTheDeploymentPolicy() {
	s.serves("not a section")

	s.Equal(4, userThreshold(s.readingSource(deploymentBase())))
}

// Removing the section removes the policy.
func (s *ConfigTestSuite) TestAnEmptySectionHasNoPolicy() {
	s.serves(accountAccessValue{})

	s.True(s.readingSource(deploymentBase()).Current(context.Background()).isEmpty())
}

// The server runtime's account access block becomes the configuration.
func (s *ConfigTestSuite) TestFromServerRuntimeReadsTheDeploymentBlock() {
	s.Require().NoError(sysconfig.InitializeServerRuntime("", &sysconfig.Config{
		AccountAccess: sysconfig.AccountAccessConfig{User: sysconfig.CategoryAccessConfig{
			LockGranularity: "account",
			Default:         sysconfig.LockPolicyConfig{Threshold: new(7)},
		}},
	}))

	cfg := FromServerRuntime()

	s.Require().NotNil(cfg.AccountAccess.User)
	s.Equal(granularityAccount, *cfg.AccountAccess.User.LockGranularity)
	s.Equal(7, *cfg.AccountAccess.User.Default.Threshold)
	s.Nil(cfg.AccountAccess.Agent, "a category the deployment does not state is absent")
}

func (s *ConfigTestSuite) TestAnEmptyBaseIsValid() {
	s.NoError(Config{}.Validate())
}

// An enabled base policy must form a lock on its own; no later layer completes it.
func (s *ConfigTestSuite) TestAnIncompleteEnabledPolicyFailsTheBase() {
	cases := map[string]func(policy *accountAccessPolicy){
		"threshold":            func(policy *accountAccessPolicy) { policy.Threshold = nil },
		"failureWindowSeconds": func(policy *accountAccessPolicy) { policy.FailureWindowSeconds = nil },
		"lockDurationsSeconds": func(policy *accountAccessPolicy) { policy.LockDurationsSeconds = nil },
	}
	for missing, drop := range cases {
		s.Run(missing, func() {
			base := deploymentBase()
			drop(base.User.Default)

			err := Config{AccountAccess: base}.Validate()

			s.Require().Error(err)
			s.Contains(err.Error(), missing)
		})
	}
}

func (s *ConfigTestSuite) TestANegativeBaseValueFailsTheBase() {
	base := deploymentBase()
	base.User.Default.LockDurationsSeconds = []int{-1}

	s.Error(Config{AccountAccess: base}.Validate())
}

// The base is refused at startup when its consecutive ceiling is negative or below its threshold of 4.
func (s *ConfigTestSuite) TestAnInvalidBaseCeilingFailsTheBase() {
	for _, ceiling := range []int{-1, 3} {
		base := deploymentBase()
		base.User.Default.MaxConsecutiveFailures = new(ceiling)
		s.Error(Config{AccountAccess: base}.Validate(), "ceiling %d", ceiling)
	}
	base := deploymentBase()
	base.User.Default.MaxConsecutiveFailures = new(0)
	s.NoError(Config{AccountAccess: base}.Validate(), "zero sets no ceiling")
}

// A switched-off policy need not state how it would lock.
func (s *ConfigTestSuite) TestADisabledPolicyIsAValidBase() {
	base := accountAccessValue{
		User:  &accountAccessCategory{Default: &accountAccessPolicy{Enabled: new(false)}},
		Agent: &accountAccessCategory{Default: &accountAccessPolicy{Enabled: new(false), Threshold: new(3)}},
	}

	s.NoError(Config{AccountAccess: base}.Validate())
}

func (s *ConfigTestSuite) TestTheShippedDefaultsAreAValidBase() {
	data, err := os.ReadFile("../../cmd/server/config/default.json")
	s.Require().NoError(err)
	var defaults struct {
		AccountAccess sysconfig.AccountAccessConfig `json:"account_access"`
	}
	s.Require().NoError(json.Unmarshal(data, &defaults))

	s.NoError(Config{AccountAccess: accountAccessValueFrom(defaults.AccountAccess)}.Validate())
}

// The shipped defaults validate and resolve to the documented policy, with the lock email on.
func (s *ConfigTestSuite) TestTheShippedDefaultsResolveToTheDocumentedPolicy() {
	data, err := os.ReadFile("../../cmd/server/config/default.json")
	s.Require().NoError(err)
	var defaults struct {
		AccountAccess sysconfig.AccountAccessConfig `json:"account_access"`
	}
	s.Require().NoError(json.Unmarshal(data, &defaults))
	s.Require().NoError(sysconfig.InitializeServerRuntime("", &sysconfig.Config{AccountAccess: defaults.AccountAccess}))
	cfg := FromServerRuntime()

	handler := accountAccessHandler{base: cfg.AccountAccess}
	s.Require().NoError(handler.Validate(accountAccessValue{}, nil, nil))
	s.serves(handler.Merge(nil, accountAccessValue{}))
	source := s.readingSource(cfg.AccountAccess)
	svc := newIdentityGovernanceService(nil, source, nil, nil, nil)
	ctx := context.Background()

	user := svc.resolvePolicy(ctx, GovernedEntity{Category: providers.EntityCategoryUser},
		providers.AuthenticationMethodCredential)
	s.True(user.enabled)
	s.Equal(5, user.threshold)
	s.Equal(time.Hour, user.failureWindow)
	s.Equal([]time.Duration{5 * time.Minute, 15 * time.Minute, 30 * time.Minute, time.Hour},
		user.lockDurations)
	s.Equal(24*time.Hour, user.lockDecay)
	s.Equal(50, user.maxConsecutiveFailures)
	agent := svc.resolvePolicy(ctx, GovernedEntity{Category: providers.EntityCategoryAgent},
		providers.AuthenticationMethodCredential)
	s.True(agent.enabled)
	s.Equal(5, agent.threshold)
	s.Equal(15*time.Minute, agent.failureWindow)
	s.Equal([]time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute}, agent.lockDurations,
		"no agent rung is permanent by default")
	s.Equal(time.Hour, agent.lockDecay)
	s.Equal(50, agent.maxConsecutiveFailures, "only the consecutive ceiling locks an agent permanently")

	activity := svc.resolveActivityPolicies(ctx)
	s.False(activity.enabled())
	s.Equal(time.Hour, activity.forCategory(providers.EntityCategoryUser).resolution)
	s.Equal(time.Hour, activity.forCategory(providers.EntityCategoryAgent).resolution)
	effective := source.Current(ctx)
	s.False(*effective.User.RecordLastLogin)
	s.False(*effective.Agent.RecordLastLogin)
	s.Equal(3600, *effective.Agent.LastLoginResolutionSeconds)
	email := effective.User.Notifications.lockEmail()
	s.True(*email.Enabled)
}
