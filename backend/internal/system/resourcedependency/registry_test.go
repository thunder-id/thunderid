// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resourcedependency

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// providerCascadeMock combines the generated ProviderMock and CascadeDeleterMock so a single value
// satisfies both interfaces — needed because Registry uses a type assertion to select cascade-capable
// providers.
type providerCascadeMock struct {
	*ProviderMock
	*CascadeDeleterMock
}

// providerValidatorMock combines ProviderMock and UpdateValidatorMock so a single value satisfies
// both interfaces for the update-validation type assertion.
type providerValidatorMock struct {
	*ProviderMock
	*UpdateValidatorMock
}

// providerDependentsMock combines ProviderMock and SuspensionDependentsListerMock so a single value satisfies both
// interfaces for the suspension dependents type assertion.
type providerDependentsMock struct {
	*ProviderMock
	*SuspensionDependentsListerMock
}

type RegistryTestSuite struct {
	suite.Suite
}

func TestRegistryTestSuite(t *testing.T) {
	suite.Run(t, new(RegistryTestSuite))
}

func (suite *RegistryTestSuite) newProvider(usages []ResourceDependency, err error) *ProviderMock {
	p := NewProviderMock(suite.T())
	p.EXPECT().GetResourceDependencies(mock.Anything, mock.Anything, mock.Anything).
		Return(usages, err).Maybe()
	return p
}

// ----- GetDependencies -----

func (suite *RegistryTestSuite) TestGetDependenciesAggregatesAcrossProviders() {
	reg := newRegistry()
	reg.RegisterProvider(suite.newProvider([]ResourceDependency{
		{ResourceType: "application", ID: "a1", DisplayName: "App 1", BehaviorOnDelete: BehaviorFallback},
		{ResourceType: "application", ID: "a2", DisplayName: "App 2", BehaviorOnDelete: BehaviorFallback},
	}, nil))
	reg.RegisterProvider(suite.newProvider([]ResourceDependency{
		{ResourceType: "agent", ID: "g1", DisplayName: "Agent 1", BehaviorOnDelete: BehaviorFallback},
	}, nil))

	resp, err := reg.GetDependencies(context.Background(), "theme", "t1")

	suite.Require().NoError(err)
	suite.Require().NotNil(resp.TotalResults)
	suite.Equal(3, *resp.TotalResults)
	suite.Equal(3, resp.Count)
	suite.Len(resp.Usages, 3)
	suite.Equal(map[string]int{"application": 2, "agent": 1}, resp.Summary)
}

func (suite *RegistryTestSuite) TestGetDependenciesProviderErrorReturnsUnknown() {
	reg := newRegistry()
	reg.RegisterProvider(suite.newProvider([]ResourceDependency{
		{ResourceType: "application", ID: "a1", DisplayName: "App 1", BehaviorOnDelete: BehaviorFallback},
	}, nil))
	reg.RegisterProvider(suite.newProvider(nil, errors.New("lookup failed")))

	resp, err := reg.GetDependencies(context.Background(), "theme", "t1")

	suite.Require().NoError(err)
	suite.Nil(resp.TotalResults)
	suite.Nil(resp.Summary)
	suite.Equal(0, resp.Count)
	suite.Empty(resp.Usages)
}

func (suite *RegistryTestSuite) TestRegisterProviderIgnoresNil() {
	reg := newRegistry()
	reg.RegisterProvider(nil)
	reg.RegisterProvider(suite.newProvider([]ResourceDependency{
		{ResourceType: "application", ID: "a1", DisplayName: "App 1", BehaviorOnDelete: BehaviorFallback},
	}, nil))

	resp, err := reg.GetDependencies(context.Background(), "theme", "t1")

	suite.Require().NoError(err)
	suite.Require().NotNil(resp.TotalResults)
	suite.Equal(1, *resp.TotalResults)
	suite.Len(resp.Usages, 1)
}

func (suite *RegistryTestSuite) TestGetDependenciesNoProvidersReturnsEmpty() {
	reg := newRegistry()

	resp, err := reg.GetDependencies(context.Background(), "theme", "t1")

	suite.Require().NoError(err)
	suite.Require().NotNil(resp.TotalResults)
	suite.Equal(0, *resp.TotalResults)
	suite.Empty(resp.Usages)
	suite.Empty(resp.Summary)
}

// ----- CascadeDelete -----

func (suite *RegistryTestSuite) newCascadeProvider(deleted int, err error) providerCascadeMock {
	p := NewProviderMock(suite.T())
	p.EXPECT().GetResourceDependencies(mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil).Maybe()
	c := NewCascadeDeleterMock(suite.T())
	c.EXPECT().CascadeDeleteDependencies(mock.Anything, mock.Anything, mock.Anything).
		Return(deleted, err).Maybe()
	return providerCascadeMock{ProviderMock: p, CascadeDeleterMock: c}
}

func (suite *RegistryTestSuite) TestCascadeDeleteSumsAcrossProvidersAndSkipsNonCascaders() {
	reg := newRegistry()
	// A plain provider (no CascadeDeleter) must be skipped, not fail.
	reg.RegisterProvider(suite.newProvider(nil, nil))
	reg.RegisterProvider(suite.newCascadeProvider(2, nil))
	reg.RegisterProvider(suite.newCascadeProvider(3, nil))

	deleted, err := reg.CascadeDelete(context.Background(), "user", "u1")

	suite.Require().NoError(err)
	suite.Equal(5, deleted)
}

func (suite *RegistryTestSuite) TestCascadeDeleteStopsOnProviderError() {
	reg := newRegistry()
	reg.RegisterProvider(suite.newCascadeProvider(1, nil))
	reg.RegisterProvider(suite.newCascadeProvider(0, errors.New("delete failed")))

	deleted, err := reg.CascadeDelete(context.Background(), "user", "u1")

	suite.Require().Error(err)
	suite.Equal(1, deleted)
}

func (suite *RegistryTestSuite) TestCascadeDeleteNoProvidersReturnsZero() {
	reg := newRegistry()
	reg.RegisterProvider(suite.newProvider(nil, nil))

	deleted, err := reg.CascadeDelete(context.Background(), "user", "u1")

	suite.Require().NoError(err)
	suite.Equal(0, deleted)
}

// ----- ValidateReferenceUpdate -----

func (suite *RegistryTestSuite) newValidatorProvider(
	validateErr *tidcommon.ServiceError) providerValidatorMock {
	p := NewProviderMock(suite.T())
	p.EXPECT().GetResourceDependencies(mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil).Maybe()
	v := NewUpdateValidatorMock(suite.T())
	v.EXPECT().ValidateReferenceUpdate(mock.Anything, mock.Anything, mock.Anything).
		Return(validateErr).Maybe()
	return providerValidatorMock{ProviderMock: p, UpdateValidatorMock: v}
}

func (suite *RegistryTestSuite) TestValidateReferenceUpdateNoValidatorsReturnsNil() {
	reg := newRegistry()
	reg.RegisterProvider(suite.newProvider(nil, nil))

	svcErr := reg.ValidateReferenceUpdate(context.Background(), "flow", "f1")

	suite.Nil(svcErr)
}

func (suite *RegistryTestSuite) TestValidateReferenceUpdateAllPassReturnsNil() {
	reg := newRegistry()
	reg.RegisterProvider(suite.newValidatorProvider(nil))
	reg.RegisterProvider(suite.newValidatorProvider(nil))

	svcErr := reg.ValidateReferenceUpdate(context.Background(), "flow", "f1")

	suite.Nil(svcErr)
}

func (suite *RegistryTestSuite) TestValidateReferenceUpdateStopsOnFirstError() {
	reg := newRegistry()
	failure := &tidcommon.ServiceError{Code: "X", Type: tidcommon.ClientErrorType}
	first := suite.newValidatorProvider(failure)
	second := suite.newValidatorProvider(nil)
	reg.RegisterProvider(first)
	reg.RegisterProvider(second)

	svcErr := reg.ValidateReferenceUpdate(context.Background(), "flow", "f1")

	suite.Equal(failure, svcErr)
	// The second provider's validator must not be invoked once the first returned an error.
	second.UpdateValidatorMock.AssertNotCalled(suite.T(), "ValidateReferenceUpdate",
		mock.Anything, mock.Anything, mock.Anything)
}

func (suite *RegistryTestSuite) TestSummarizeBlockingUsagesHumanizesTypes() {
	usages := []ResourceDependency{
		{ResourceType: ResourceTypeFlow},
		{ResourceType: ResourceTypeFlow},
		// A camelCase / non-shared type must not leak its raw identifier into the summary.
		{ResourceType: "systemEndpoint"},
		{ResourceType: ResourceTypeNotificationSender},
	}

	summary := SummarizeBlockingUsages(usages)

	// Sorted by raw type; rendered with humanized, lowercase nouns.
	suite.Equal("2 flow(s), 1 notification sender(s), 1 system endpoint(s)", summary)
}

// Only restrict blocks a deletion: a fallback reference survives it and a cascade dependent is removed with it.
func (suite *RegistryTestSuite) TestBlockingUsagesKeepsOnlyRestrict() {
	blocking := BlockingUsages(&DependenciesResponse{Usages: []ResourceDependency{
		{ID: "restrict", BehaviorOnDelete: BehaviorRestrict},
		{ID: "fallback", BehaviorOnDelete: BehaviorFallback},
		{ID: "cascade", BehaviorOnDelete: BehaviorCascade},
	}})

	suite.Equal([]ResourceDependency{{ID: "restrict", BehaviorOnDelete: BehaviorRestrict}}, blocking)
}

// SuspensionDependents collects the dependents named by every provider that lists them, skipping the others.
func (suite *RegistryTestSuite) TestSuspensionDependentsCollectsFromListers() {
	first := NewSuspensionDependentsListerMock(suite.T())
	first.EXPECT().SuspensionDependents(mock.Anything, ResourceTypeUser, "u1").Return([]string{"a1"}, nil).Once()
	second := NewSuspensionDependentsListerMock(suite.T())
	second.EXPECT().SuspensionDependents(mock.Anything, ResourceTypeUser, "u1").Return([]string{"a2", "a3"}, nil).Once()
	registry := Initialize(
		providerDependentsMock{ProviderMock: NewProviderMock(suite.T()), SuspensionDependentsListerMock: first},
		NewProviderMock(suite.T()),
		providerDependentsMock{ProviderMock: NewProviderMock(suite.T()), SuspensionDependentsListerMock: second},
	)

	dependents, err := registry.SuspensionDependents(context.Background(), ResourceTypeUser, "u1")

	suite.Require().NoError(err)
	suite.Equal([]string{"a1", "a2", "a3"}, dependents)
}

// A provider failure stops the lookup and is returned.
func (suite *RegistryTestSuite) TestSuspensionDependentsStopsAtTheFirstFailure() {
	failing := NewSuspensionDependentsListerMock(suite.T())
	failing.EXPECT().SuspensionDependents(mock.Anything, ResourceTypeUser, "u1").Return(nil, errors.New("down")).Once()
	never := NewSuspensionDependentsListerMock(suite.T())
	registry := Initialize(
		providerDependentsMock{ProviderMock: NewProviderMock(suite.T()), SuspensionDependentsListerMock: failing},
		providerDependentsMock{ProviderMock: NewProviderMock(suite.T()), SuspensionDependentsListerMock: never},
	)

	dependents, err := registry.SuspensionDependents(context.Background(), ResourceTypeUser, "u1")

	suite.Require().Error(err)
	suite.Nil(dependents)
}
