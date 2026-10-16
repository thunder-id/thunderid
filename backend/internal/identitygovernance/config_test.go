// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernance

import (
	"context"
	"encoding/json"
	"testing"

	sysconfig "github.com/thunder-id/thunderid/internal/system/config"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/serverconfig"
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
	s.reader = serverconfigmock.NewServerConfigServiceMock(s.T())
}

func (s *ConfigTestSuite) section(raw string) accountAccessValue {
	value, err := accountAccessHandler{}.Decode(json.RawMessage(raw))
	s.Require().NoError(err)
	return value.(accountAccessValue)
}

// serves makes the reader return value for the accountAccess section.
func (s *ConfigTestSuite) serves(
	value any) *serverconfigmock.ServerConfigServiceMock_GetMergedConfig_Call {
	return s.reader.EXPECT().GetMergedConfig(mock.Anything, string(serverconfig.ConfigNameAccountAccess)).
		Return(value, nil)
}

// userPolicy resolves the user credential policy through the source.
func userPolicy(source accountAccessSourceInterface) lockoutPolicy {
	return newPolicyResolver(source).Resolve(context.Background(),
		GovernedEntity{Category: providers.EntityCategoryUser}, providers.AccessScopeCredential)
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

func (s *ConfigTestSuite) TestWithNoReaderAndNoBaseNoPolicyIsConfigured() {
	policy := userPolicy(newAccountAccessSource(accountAccessValue{}, nil))
	s.False(policy.enabled)
	s.Zero(policy.threshold)
}

func (s *ConfigTestSuite) TestWithNoReaderTheDeploymentPolicyApplies() {
	policy := userPolicy(newAccountAccessSource(deploymentBase(), nil))
	s.True(policy.enabled)
	s.Equal(4, policy.threshold)
}

func (s *ConfigTestSuite) TestTheReaderSuppliesThePolicy() {
	s.serves(s.section(`{"user":{"default":{"enabled":true,"threshold":3,"failureWindowSeconds":600,` +
		`"lockDurationsSeconds":[60]}}}`))

	s.Equal(3, userPolicy(newAccountAccessSource(deploymentBase(), s.reader)).threshold)
}

// A failed read falls back to the deployment policy, not to the last value read.
func (s *ConfigTestSuite) TestAFailedReadFallsBackToTheDeploymentPolicy() {
	source := newAccountAccessSource(deploymentBase(), s.reader)
	s.serves(s.section(`{"user":{"default":{"threshold":2}}}`)).Once()
	s.Equal(2, userPolicy(source).threshold)

	s.reader.EXPECT().GetMergedConfig(mock.Anything, mock.Anything).Return(nil, &common.InternalServerError).Once()

	s.Equal(4, userPolicy(source).threshold)
}

// The section is read on every call, so a write served by another node is seen.
func (s *ConfigTestSuite) TestTheSectionIsReadOnEveryCall() {
	source := newAccountAccessSource(accountAccessValue{}, s.reader)
	s.serves(s.section(`{"user":{"default":{"threshold":4}}}`)).Once()
	s.serves(s.section(`{"user":{"default":{"threshold":2}}}`)).Once()

	s.Equal(4, *source.Current(context.Background()).User.Default.Threshold)
	s.Equal(2, *source.Current(context.Background()).User.Default.Threshold)
	s.reader.AssertNumberOfCalls(s.T(), "GetMergedConfig", 2)
}

// A wrong-typed value is handled like a failed read.
func (s *ConfigTestSuite) TestAnUnexpectedSectionTypeFallsBackToTheDeploymentPolicy() {
	s.serves("not a section")

	s.Equal(4, userPolicy(newAccountAccessSource(deploymentBase(), s.reader)).threshold)
}

// The deployment configuration is served with the lock email recipient defaulted.
func (s *ConfigTestSuite) TestTheBaseCarriesTheDefaultRecipientAttribute() {
	base := deploymentBase()
	base.User.Notifications = &accountAccessNotifications{
		OnLock: &accountAccessLockNotification{
			Email: &accountAccessLockEmail{Enabled: new(true)},
		},
	}

	email := newAccountAccessSource(base, nil).Current(context.Background()).User.Notifications.LockEmail()

	s.Equal(defaultLockEmailRecipientAttribute, *email.RecipientAttribute)
	s.Nil(base.User.Notifications.LockEmail().RecipientAttribute, "the configuration itself is not changed")
}

// Removing the section removes the policy.
func (s *ConfigTestSuite) TestAnEmptySectionHasNoPolicy() {
	s.serves(accountAccessValue{})

	s.False(userPolicy(newAccountAccessSource(accountAccessValue{}, s.reader)).enabled)
}

// Activity policies come from one read of the section.
func (s *ConfigTestSuite) TestActivityPoliciesUseOneRead() {
	s.serves(s.section(`{"user":{"recordLastLogin":true,"lastLoginResolutionSeconds":300}}`)).Once()

	policies := newPolicyResolver(newAccountAccessSource(accountAccessValue{}, s.reader)).
		ActivityPolicies(context.Background())

	s.True(policies.enabled())
	s.True(policies.forCategory(providers.EntityCategoryUser).record)
	s.False(policies.forCategory(providers.EntityCategoryAgent).record)
}

// A category that states nothing converts to nil, so it inherits rather than overriding.
func (s *ConfigTestSuite) TestAnEmptyCategoryConvertsToNil() {
	value := accountAccessValueFrom(sysconfig.AccountAccessConfig{})

	s.Nil(value.User)
	s.Nil(value.Agent)
	s.True(value.IsEmpty())
}

func (s *ConfigTestSuite) TestTheDeploymentConfigurationConverts() {
	value := accountAccessValueFrom(sysconfig.AccountAccessConfig{User: sysconfig.CategoryAccessConfig{
		LockGranularity: "entity",
		RecordLastLogin: new(true),
		Default:         sysconfig.ScopeAccessConfig{Threshold: new(5), LockDurationsSeconds: []int{60}},
		Notifications: sysconfig.AccessNotificationsConfig{OnLock: sysconfig.LockNotificationConfig{
			Email: sysconfig.LockEmailConfig{Enabled: new(true), RecipientAttribute: new("workEmail")},
		}},
	}})

	s.Require().NotNil(value.User)
	s.Equal("entity", *value.User.LockGranularity)
	s.True(*value.User.RecordLastLogin)
	s.Equal(5, *value.User.Default.Threshold)
	s.Equal([]int{60}, value.User.Default.LockDurationsSeconds)
	email := value.User.Notifications.LockEmail()
	s.Require().NotNil(email)
	s.True(*email.Enabled)
	s.Equal("workEmail", *email.RecipientAttribute)
	s.Nil(value.Agent)
}

// A default that states nothing is left unset, so a lower layer's default is not replaced.
func (s *ConfigTestSuite) TestAnEmptyDefaultPolicyIsLeftUnset() {
	value := accountAccessValueFrom(sysconfig.AccountAccessConfig{User: sysconfig.CategoryAccessConfig{
		RecordLastLogin: new(false),
	}})

	s.Require().NotNil(value.User)
	s.Nil(value.User.Default)
	s.Nil(value.User.Notifications)
}

func (s *ConfigTestSuite) TestAnEmptyBaseIsValid() {
	s.NoError(Config{}.Validate())
}

func (s *ConfigTestSuite) TestAnUnknownGranularityFailsTheBase() {
	base := deploymentBase()
	base.User.LockGranularity = new("account")

	err := Config{AccountAccess: base}.Validate()

	s.Require().Error(err)
	s.Contains(err.Error(), "user.lockGranularity")
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

// A switched-off policy need not state how it would lock.
func (s *ConfigTestSuite) TestADisabledPolicyIsAValidBase() {
	base := accountAccessValue{
		User:  &accountAccessCategory{Default: &accountAccessPolicy{Enabled: new(false)}},
		Agent: &accountAccessCategory{Default: &accountAccessPolicy{Enabled: new(false), Threshold: new(3)}},
	}

	s.NoError(Config{AccountAccess: base}.Validate())
}
