// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ou

import (
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const (
	providerTestOUID     = "ou-id-123"
	providerTestParentID = "ou-id-parent"
	providerTestChildID  = "ou-id-child"
)

type OUProviderTestSuite struct {
	suite.Suite
	mockService *OrganizationUnitServiceInterfaceMock
	provider    providers.OrganizationUnitProvider
}

func (suite *OUProviderTestSuite) SetupTest() {
	suite.mockService = NewOrganizationUnitServiceInterfaceMock(suite.T())
	suite.provider = NewOrganizationUnitProvider(suite.mockService)
}

func TestOUProviderTestSuite(t *testing.T) {
	suite.Run(t, new(OUProviderTestSuite))
}

// managementOU is a fully populated management organization unit. Only the runtime subset of its
// fields may cross the provider contract.
func managementOU() OrganizationUnit {
	parent := providerTestParentID
	return OrganizationUnit{
		ID:                   providerTestOUID,
		Handle:               "engineering",
		Name:                 "Engineering",
		Description:          "Engineering unit",
		LogoURL:              "https://example.com/logo.png",
		TosURI:               "https://example.com/tos",
		PolicyURI:            "https://example.com/policy",
		CookiePolicyURI:      "https://example.com/cookies",
		Parent:               &parent,
		ThemeID:              "theme-1",
		LayoutID:             "layout-1",
		AuthFlowID:           "auth-flow-1",
		RegistrationFlowID:   "reg-flow-1",
		RecoveryFlowID:       "rec-flow-1",
		SignOutFlowID:        "signout-flow-1",
		UserOnboardingFlowID: "onboard-flow-1",
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}
}

// The adapter copies the eight runtime fields and drops the management-only ones.
func (suite *OUProviderTestSuite) TestGetOrganizationUnitMapsRuntimeFields() {
	suite.mockService.On("GetOrganizationUnit", mock.Anything, providerTestOUID).
		Return(managementOU(), (*tidcommon.ServiceError)(nil)).Once()

	got, svcErr := suite.provider.GetOrganizationUnit(suite.T().Context(), providerTestOUID)

	suite.Nil(svcErr)
	suite.Equal(providers.OrganizationUnit{
		ID:              providerTestOUID,
		Handle:          "engineering",
		Name:            "Engineering",
		Description:     "Engineering unit",
		LogoURL:         "https://example.com/logo.png",
		TosURI:          "https://example.com/tos",
		PolicyURI:       "https://example.com/policy",
		CookiePolicyURI: "https://example.com/cookies",
	}, got)
}

// Service errors are returned unchanged alongside a zero organization unit.
func (suite *OUProviderTestSuite) TestGetOrganizationUnitReturnsServiceError() {
	suite.mockService.On("GetOrganizationUnit", mock.Anything, providerTestOUID).
		Return(OrganizationUnit{}, &tidcommon.InternalServerError).Once()

	got, svcErr := suite.provider.GetOrganizationUnit(suite.T().Context(), providerTestOUID)

	suite.Equal(&tidcommon.InternalServerError, svcErr)
	suite.Equal(providers.OrganizationUnit{}, got)
}

// The runtime request reaches the service as the management request, and the response is mapped back.
func (suite *OUProviderTestSuite) TestCreateOrganizationUnitMapsRequestAndResponse() {
	parent := providerTestParentID
	suite.mockService.On("CreateOrganizationUnit", mock.Anything, OrganizationUnitRequestWithID{
		ID:          providerTestOUID,
		Handle:      "engineering",
		Name:        "Engineering",
		Description: "Engineering unit",
		Parent:      &parent,
	}).Return(managementOU(), (*tidcommon.ServiceError)(nil)).Once()

	got, svcErr := suite.provider.CreateOrganizationUnit(suite.T().Context(),
		providers.OrganizationUnitRequestWithID{
			ID:          providerTestOUID,
			Handle:      "engineering",
			Name:        "Engineering",
			Description: "Engineering unit",
			Parent:      &parent,
		})

	suite.Nil(svcErr)
	suite.Equal(providerTestOUID, got.ID)
	suite.Equal("engineering", got.Handle)
}

// A conflict from the service is surfaced unchanged so the flow engine can branch on its code.
func (suite *OUProviderTestSuite) TestCreateOrganizationUnitReturnsServiceError() {
	suite.mockService.On("CreateOrganizationUnit", mock.Anything, mock.Anything).
		Return(OrganizationUnit{}, &providers.ErrorOrganizationUnitHandleConflict).Once()

	got, svcErr := suite.provider.CreateOrganizationUnit(suite.T().Context(),
		providers.OrganizationUnitRequestWithID{Handle: "engineering"})

	suite.Equal(&providers.ErrorOrganizationUnitHandleConflict, svcErr)
	suite.Equal(providers.OrganizationUnit{}, got)
}

func (suite *OUProviderTestSuite) TestIsParentDelegates() {
	suite.mockService.On("IsParent", mock.Anything, providerTestParentID, providerTestChildID).
		Return(true, (*tidcommon.ServiceError)(nil)).Once()

	ok, svcErr := suite.provider.IsParent(suite.T().Context(), providerTestParentID, providerTestChildID)

	suite.Nil(svcErr)
	suite.True(ok)
}

func (suite *OUProviderTestSuite) TestIsOrganizationUnitExistsDelegates() {
	suite.mockService.On("IsOrganizationUnitExists", mock.Anything, providerTestOUID).
		Return(false, (*tidcommon.ServiceError)(nil)).Once()

	ok, svcErr := suite.provider.IsOrganizationUnitExists(suite.T().Context(), providerTestOUID)

	suite.Nil(svcErr)
	suite.False(ok)
}

// The children page is mapped to the runtime view; pagination links stay on the management side.
func (suite *OUProviderTestSuite) TestGetOrganizationUnitChildrenMapsPage() {
	suite.mockService.On("GetOrganizationUnitChildren", mock.Anything, providerTestOUID, 10, 0,
		(*tidcommon.FilterGroup)(nil)).
		Return(&OrganizationUnitListResponse{
			TotalResults: 2,
			StartIndex:   1,
			Count:        1,
			OrganizationUnits: []OrganizationUnitBasic{{
				ID: providerTestChildID, Handle: "child", Name: "Child", Description: "Child unit",
				LogoURL: "https://example.com/child.png", IsReadOnly: true,
			}},
			Links: []utils.Link{{Href: "/organization-units?offset=1&limit=1", Rel: "next"}},
		}, (*tidcommon.ServiceError)(nil)).Once()

	got, svcErr := suite.provider.GetOrganizationUnitChildren(
		suite.T().Context(), providerTestOUID, 10, 0, nil)

	suite.Nil(svcErr)
	suite.Equal(2, got.TotalResults)
	suite.Equal(1, got.StartIndex)
	suite.Equal(1, got.Count)
	suite.Len(got.OrganizationUnits, 1)
	suite.Equal(providers.OrganizationUnitBasic{
		ID: providerTestChildID, Handle: "child", Name: "Child", Description: "Child unit",
	}, got.OrganizationUnits[0])
}

func (suite *OUProviderTestSuite) TestGetOrganizationUnitChildrenReturnsServiceError() {
	suite.mockService.On("GetOrganizationUnitChildren", mock.Anything, providerTestOUID, 10, 0,
		(*tidcommon.FilterGroup)(nil)).
		Return(nil, &tidcommon.InternalServerError).Once()

	got, svcErr := suite.provider.GetOrganizationUnitChildren(
		suite.T().Context(), providerTestOUID, 10, 0, nil)

	suite.Equal(&tidcommon.InternalServerError, svcErr)
	suite.Nil(got)
}

// A nil page from the service maps to a nil page rather than an empty one.
func (suite *OUProviderTestSuite) TestGetOrganizationUnitChildrenHandlesNilPage() {
	suite.mockService.On("GetOrganizationUnitChildren", mock.Anything, providerTestOUID, 10, 0,
		(*tidcommon.FilterGroup)(nil)).
		Return(nil, (*tidcommon.ServiceError)(nil)).Once()

	got, svcErr := suite.provider.GetOrganizationUnitChildren(
		suite.T().Context(), providerTestOUID, 10, 0, nil)

	suite.Nil(svcErr)
	suite.Nil(got)
}

// NewOrganizationUnitProvider returns a provider backed by the given service.
func (suite *OUProviderTestSuite) TestNewProviderReturnsProvider() {
	suite.NotNil(NewOrganizationUnitProvider(suite.mockService))
}
