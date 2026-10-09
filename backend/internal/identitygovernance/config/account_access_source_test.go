// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/thunder-id/thunderid/internal/identitygovernance/model"

	"github.com/stretchr/testify/suite"

	sysconfig "github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// stubConfigReader stands in for the server-config service and counts reads.
type stubConfigReader struct {
	value any
	err   *common.ServiceError
	reads int
}

func (r *stubConfigReader) GetMergedConfig(_ context.Context, _ string) (any, *common.ServiceError) {
	r.reads++
	if r.err != nil {
		return nil, r.err
	}
	return r.value, nil
}

type AccountAccessSourceTestSuite struct {
	suite.Suite
}

func TestAccountAccessSourceTestSuite(t *testing.T) {
	suite.Run(t, new(AccountAccessSourceTestSuite))
}

func (s *AccountAccessSourceTestSuite) newResolver() *policyResolver {
	return NewPolicyResolver(NewAccountAccessSource(sysconfig.AccountAccessConfig{}, log.GetLogger()))
}

func (s *AccountAccessSourceTestSuite) section(raw string) AccountAccessValue {
	value, err := AccountAccessHandler{}.Decode(json.RawMessage(raw))
	s.Require().NoError(err)
	return value.(AccountAccessValue)
}

// merged overlays a writable value onto representative declarative defaults.
func (s *AccountAccessSourceTestSuite) merged(raw string) AccountAccessValue {
	return AccountAccessHandler{}.Merge(testAccountAccessConfig(), s.section(raw)).(AccountAccessValue)
}

func (s *AccountAccessSourceTestSuite) resolveUserCredential(
	resolver *policyResolver) LockoutPolicy {
	return resolver.Resolve(context.Background(),
		model.GovernedEntity{Category: providers.EntityCategoryUser},
		model.AccessScopeCredential)
}

// deploymentBase is a deployment policy with a threshold of 4.
func deploymentBase() sysconfig.AccountAccessConfig {
	enabled, threshold, window := true, 4, 900
	return sysconfig.AccountAccessConfig{User: sysconfig.CategoryAccessConfig{
		Default: sysconfig.ScopeAccessConfig{
			Enabled: &enabled, Threshold: &threshold, FailureWindowSeconds: &window,
			LockDurationsSeconds: []int{300},
		},
	}}
}

func (s *AccountAccessSourceTestSuite) TestWithNoReaderNoPolicyIsConfigured() {
	resolver := s.newResolver()

	policy := s.resolveUserCredential(resolver)
	s.False(policy.Enabled)
	s.Zero(policy.Threshold)
}

// Until a reader is installed the deployment policy applies.
func (s *AccountAccessSourceTestSuite) TestWithNoReaderTheDeploymentPolicyApplies() {
	resolver := NewPolicyResolver(NewAccountAccessSource(deploymentBase(), log.GetLogger()))

	policy := s.resolveUserCredential(resolver)
	s.True(policy.Enabled)
	s.Equal(4, policy.Threshold)
}

// A failed read falls back to the deployment policy, not to the last value read.
func (s *AccountAccessSourceTestSuite) TestAFailedReadFallsBackToTheDeploymentPolicy() {
	resolver := NewPolicyResolver(NewAccountAccessSource(deploymentBase(), log.GetLogger()))
	reader := &stubConfigReader{value: s.section(`{"user":{"default":{"enabled":true,"threshold":2}}}`)}
	resolver.section.install(reader)
	s.Equal(2, s.resolveUserCredential(resolver).Threshold)

	reader.err = &common.InternalServerError

	s.Equal(4, s.resolveUserCredential(resolver).Threshold)
}

func (s *AccountAccessSourceTestSuite) TestAnInstalledReaderSuppliesThePolicy() {
	resolver := s.newResolver()
	reader := &stubConfigReader{value: s.section(
		`{"user":{"default":{"enabled":true,"threshold":3,"failureWindowSeconds":600,` +
			`"lockDurationsSeconds":[60]}}}`)}

	resolver.section.install(reader)

	policy := s.resolveUserCredential(resolver)
	s.Equal(3, policy.Threshold)
}

// The section is read on every resolve.
func (s *AccountAccessSourceTestSuite) TestResolveReadsTheSectionEveryTime() {
	resolver := s.newResolver()
	reader := &stubConfigReader{value: s.section(`{"user":{"default":{"threshold":3}}}`)}
	resolver.section.install(reader)
	s.Zero(reader.reads, "installing the reader does not read")

	for i := 0; i < 10; i++ {
		s.resolveUserCredential(resolver)
	}
	s.Equal(10, reader.reads)
}

// A write served by another node is seen without any notification.
func (s *AccountAccessSourceTestSuite) TestAWriteOnAnotherNodeIsVisibleWithoutBeingToldAboutIt() {
	resolver := s.newResolver()
	reader := &stubConfigReader{value: s.section(`{"user":{"default":{"threshold":4}}}`)}
	resolver.section.install(reader)

	policy := s.resolveUserCredential(resolver)
	s.Equal(4, policy.Threshold)

	reader.value = s.section(`{"user":{"default":{"threshold":2}}}`)

	policy = s.resolveUserCredential(resolver)
	s.Equal(2, policy.Threshold)
}

// A failed read applies no policy, like every other server-config section.
func (s *AccountAccessSourceTestSuite) TestAFailedReadHasNoPolicy() {
	resolver := s.newResolver()
	reader := &stubConfigReader{value: s.section(`{"user":{"default":{"enabled":true,"threshold":2}}}`)}
	resolver.section.install(reader)
	s.Equal(2, s.resolveUserCredential(resolver).Threshold)

	reader.err = &common.InternalServerError

	policy := s.resolveUserCredential(resolver)
	s.False(policy.Enabled)
	s.Zero(policy.Threshold)
}

// A wrong-typed value is handled like a failed read.
func (s *AccountAccessSourceTestSuite) TestAnUnexpectedSectionTypeHasNoPolicy() {
	resolver := s.newResolver()
	reader := &stubConfigReader{value: "not a section"}
	resolver.section.install(reader)

	policy := s.resolveUserCredential(resolver)
	s.False(policy.Enabled)
	s.Zero(policy.Threshold)
}

// Writable omissions retain the declarative defaults.
func (s *AccountAccessSourceTestSuite) TestWritableOmittedCategoryKeepsDeclarativePolicy() {
	resolver := s.newResolver()
	reader := &stubConfigReader{value: s.merged(`{"agent":{"default":{"enabled":false}}}`)}
	resolver.section.install(reader)

	policy := s.resolveUserCredential(resolver)
	s.True(policy.Enabled)
	s.Equal(5, policy.Threshold)
}

func (s *AccountAccessSourceTestSuite) TestEmptyWritableLayerKeepsDeclarativePolicy() {
	resolver := s.newResolver()
	reader := &stubConfigReader{value: s.merged(`{}`)}
	resolver.section.install(reader)

	policy := s.resolveUserCredential(resolver)
	s.True(policy.Enabled)
	s.Equal(5, policy.Threshold)
}

func (s *AccountAccessSourceTestSuite) TestANilReaderLeavesPolicyUnconfigured() {
	resolver := s.newResolver()
	InstallConfigReader(nil)

	s.False(s.resolveUserCredential(resolver).Enabled)
}

func (s *AccountAccessSourceTestSuite) TestTheSectionCanSwitchACategoryOff() {
	resolver := s.newResolver()
	reader := &stubConfigReader{value: s.section(`{"user":{"default":{"enabled":false}}}`)}
	resolver.section.install(reader)

	policy := s.resolveUserCredential(resolver)
	s.False(policy.Enabled)
}

func (s *AccountAccessSourceTestSuite) TestActivityPoliciesUseOneFreshSnapshot() {
	resolver := s.newResolver()
	reader := &stubConfigReader{value: s.section(`{"user":{"recordLastLogin":true,"lastLoginResolutionSeconds":300}}`)}
	resolver.section.install(reader)
	reader.reads = 0

	policies := resolver.ActivityPolicies(context.Background())
	s.Equal(1, reader.reads)
	s.True(policies.Enabled())
	s.True(policies.For(providers.EntityCategoryUser).Record)
	s.False(policies.For(providers.EntityCategoryAgent).Record)

	reader.value = s.section(`{"user":{"recordLastLogin":false},"agent":{"recordLastLogin":true}}`)
	s.True(policies.For(providers.EntityCategoryUser).Record, "the current operation keeps its snapshot")
	updated := resolver.ActivityPolicies(context.Background())
	s.Equal(2, reader.reads)
	s.False(updated.For(providers.EntityCategoryUser).Record)
	s.True(updated.For(providers.EntityCategoryAgent).Record)
}

func (s *AccountAccessSourceTestSuite) TestRemovingSectionRemovesPolicy() {
	resolver := s.newResolver()
	reader := &stubConfigReader{value: testAccountAccessConfig()}
	resolver.section.install(reader)
	s.True(s.resolveUserCredential(resolver).Enabled)
	reader.value = AccountAccessValue{}
	s.False(s.resolveUserCredential(resolver).Enabled)
}
