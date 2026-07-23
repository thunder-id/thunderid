/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

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
