// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ou

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
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
		Parent:          providerTestParentID,
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

func (suite *OUProviderTestSuite) TestDeleteOrganizationUnitDelegates() {
	suite.mockService.On("DeleteOrganizationUnit", mock.Anything, providerTestOUID).
		Return(&ErrorOrganizationUnitNotFound).Once()

	svcErr := suite.provider.DeleteOrganizationUnit(suite.T().Context(), providerTestOUID)

	suite.Equal(&ErrorOrganizationUnitNotFound, svcErr)
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

const (
	signInAppID  = "app-1"
	signInUserID = "user-1"
)

// eligibilityProvider returns a provider whose sign-in eligibility the returned resolvers answer.
func (suite *OUProviderTestSuite) eligibilityProvider() (providers.OrganizationUnitProvider,
	*entityEligibilityResolverMock, *applicationAccessResolverMock) {
	users := newEntityEligibilityResolverMock(suite.T())
	applications := newApplicationAccessResolverMock(suite.T())
	provider := NewOrganizationUnitProvider(suite.mockService)
	provider.SetSignInEligibilityResolvers(users, applications)
	return provider, users, applications
}

// boolPtr returns a pointer to b.
func boolPtr(b bool) *bool { return &b }

// signInCtx returns a context carrying the sign-in boundary of signInAppID with the given signed-in subject.
func (suite *OUProviderTestSuite) signInCtx(entityID string) context.Context {
	return authnprovidercm.WithSignInBoundary(suite.T().Context(),
		authnprovidercm.SignInBoundary{ApplicationID: signInAppID, EntityID: entityID})
}

// A provider whose resolvers were never injected refuses every eligibility question.
func (suite *OUProviderTestSuite) TestEligibilityIsRefusedUntilTheResolversAreSet() {
	provider := NewOrganizationUnitProvider(suite.mockService)
	ctx := suite.signInCtx(signInUserID)

	ouIDs, svcErr := provider.GetEligibleOUs(ctx)
	suite.Equal(&tidcommon.InternalServerError, svcErr)
	suite.Nil(ouIDs)

	eligible, svcErr := provider.IsEligibleOU(ctx, providerTestOUID)
	suite.Equal(&tidcommon.InternalServerError, svcErr)
	suite.False(eligible)

	shared, svcErr := provider.IsApplicationShared(ctx)
	suite.Equal(&tidcommon.InternalServerError, svcErr)
	suite.False(shared)
}

// The signed-in user's organization units are kept where the application is usable.
func (suite *OUProviderTestSuite) TestGetEligibleOUsListsTheSignedInUsersOUsTheApplicationIsUsableFrom() {
	provider, users, applications := suite.eligibilityProvider()
	ctx := suite.signInCtx(signInUserID)
	users.EXPECT().EntityOUs(ctx, signInUserID).Return([]string{"ou-a", "ou-b", "ou-c"}, true, nil).Once()
	applications.EXPECT().IsClientAccessibleFromOUByEntityID(ctx, signInAppID, "ou-a").Return(true, nil).Once()
	applications.EXPECT().IsClientAccessibleFromOUByEntityID(ctx, signInAppID, "ou-b").Return(false, nil).Once()
	applications.EXPECT().IsClientAccessibleFromOUByEntityID(ctx, signInAppID, "ou-c").
		Return(false, &ErrorOrganizationUnitNotFound).Once()

	ouIDs, svcErr := provider.GetEligibleOUs(ctx)

	suite.Nil(svcErr)
	suite.Equal([]string{"ou-a"}, ouIDs)
}

// A sign-in that fixed its organization unit lists that one alone, when the user is in it.
func (suite *OUProviderTestSuite) TestGetEligibleOUsKeepsTheFixedOU() {
	provider, users, applications := suite.eligibilityProvider()
	ctx := authnprovidercm.WithSignInBoundary(suite.T().Context(),
		authnprovidercm.SignInBoundary{OUID: "ou-b", ApplicationID: signInAppID, EntityID: signInUserID})
	users.EXPECT().EntityOUs(ctx, signInUserID).Return([]string{"ou-a", "ou-b"}, true, nil).Once()
	applications.EXPECT().IsClientAccessibleFromOUByEntityID(ctx, signInAppID, "ou-b").Return(true, nil).Once()

	ouIDs, svcErr := provider.GetEligibleOUs(ctx)

	suite.Nil(svcErr)
	suite.Equal([]string{"ou-b"}, ouIDs)
}

// Without a signed-in user the entity store holds, no organization unit is listed.
func (suite *OUProviderTestSuite) TestGetEligibleOUsListsNothingWithoutAUser() {
	cases := []struct {
		name    string
		ctx     func() context.Context
		arrange func(users *entityEligibilityResolverMock)
	}{
		{"no sign-in boundary", suite.T().Context, func(*entityEligibilityResolverMock) {}},
		{"no signed-in user", func() context.Context { return suite.signInCtx("") },
			func(*entityEligibilityResolverMock) {}},
		{"a user the entity store does not hold", func() context.Context { return suite.signInCtx(signInUserID) },
			func(users *entityEligibilityResolverMock) {
				users.EXPECT().EntityOUs(mock.Anything, signInUserID).Return(nil, false, nil).Once()
			}},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			provider, users, _ := suite.eligibilityProvider()
			tc.arrange(users)

			ouIDs, svcErr := provider.GetEligibleOUs(tc.ctx())

			suite.Equal(&ErrorSignedInUserRequired, svcErr)
			suite.Nil(ouIDs)
		})
	}
}

func (suite *OUProviderTestSuite) TestGetEligibleOUsResolverFailureIsAServerError() {
	provider, users, _ := suite.eligibilityProvider()
	users.EXPECT().EntityOUs(mock.Anything, signInUserID).Return(nil, false, errors.New("store down")).Once()

	ouIDs, svcErr := provider.GetEligibleOUs(suite.signInCtx(signInUserID))

	suite.Equal(&tidcommon.InternalServerError, svcErr)
	suite.Nil(ouIDs)
}

// With a signed-in user the entity store holds, the organization unit must be one the user is in and the
// application is usable from.
func (suite *OUProviderTestSuite) TestIsEligibleOUAsksTheUsersOUsAndTheApplication() {
	cases := []struct {
		name      string
		userOUIDs []string
		usable    *bool
		want      bool
	}{
		{"the user is in it and the application is usable", []string{"ou-other", providerTestOUID},
			boolPtr(true), true},
		{"the user is in it and the application is not usable", []string{providerTestOUID}, boolPtr(false), false},
		{"the user is not in it", []string{"ou-other"}, nil, false},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			provider, users, applications := suite.eligibilityProvider()
			users.EXPECT().EntityOUs(mock.Anything, signInUserID).Return(tc.userOUIDs, true, nil).Once()
			if tc.usable != nil {
				applications.EXPECT().IsClientAccessibleFromOUByEntityID(mock.Anything, signInAppID, providerTestOUID).
					Return(*tc.usable, nil).Once()
			}

			eligible, svcErr := provider.IsEligibleOU(suite.signInCtx(signInUserID), providerTestOUID)

			suite.Nil(svcErr)
			suite.Equal(tc.want, eligible)
		})
	}
}

// Before sign-in, or for a user the entity store does not hold, the organization unit must be one the
// application is usable from.
func (suite *OUProviderTestSuite) TestIsEligibleOUAsksTheApplicationWithoutAUserTheStoreHolds() {
	cases := []struct {
		name     string
		entityID string
	}{
		{"before sign-in", ""},
		{"a user the entity store does not hold", signInUserID},
	}
	for _, tc := range cases {
		for _, usable := range []bool{true, false} {
			suite.Run(fmt.Sprintf("%s, usable %t", tc.name, usable), func() {
				provider, users, applications := suite.eligibilityProvider()
				if tc.entityID != "" {
					users.EXPECT().EntityOUs(mock.Anything, tc.entityID).Return(nil, false, nil).Once()
				}
				applications.EXPECT().IsClientAccessibleFromOUByEntityID(mock.Anything, signInAppID, providerTestOUID).
					Return(usable, nil).Once()

				eligible, svcErr := provider.IsEligibleOU(suite.signInCtx(tc.entityID), providerTestOUID)

				suite.Nil(svcErr)
				suite.Equal(usable, eligible)
			})
		}
	}
}

// An organization unit the application check cannot resolve, such as one that does not exist, is not eligible.
func (suite *OUProviderTestSuite) TestIsEligibleOUClientErrorIsNotEligible() {
	provider, _, applications := suite.eligibilityProvider()
	applications.EXPECT().IsClientAccessibleFromOUByEntityID(mock.Anything, signInAppID, providerTestOUID).
		Return(false, &ErrorOrganizationUnitNotFound).Once()

	eligible, svcErr := provider.IsEligibleOU(suite.signInCtx(""), providerTestOUID)

	suite.Nil(svcErr)
	suite.False(eligible)
}

func (suite *OUProviderTestSuite) TestIsEligibleOUFailuresAreServerErrors() {
	suite.Run("user eligibility", func() {
		provider, users, _ := suite.eligibilityProvider()
		users.EXPECT().EntityOUs(mock.Anything, signInUserID).Return(nil, false, errors.New("store down")).Once()

		eligible, svcErr := provider.IsEligibleOU(suite.signInCtx(signInUserID), providerTestOUID)

		suite.Equal(&tidcommon.InternalServerError, svcErr)
		suite.False(eligible)
	})
	suite.Run("application access", func() {
		provider, _, applications := suite.eligibilityProvider()
		applications.EXPECT().IsClientAccessibleFromOUByEntityID(mock.Anything, signInAppID, providerTestOUID).
			Return(false, &tidcommon.InternalServerError).Once()

		eligible, svcErr := provider.IsEligibleOU(suite.signInCtx(""), providerTestOUID)

		suite.Equal(&tidcommon.InternalServerError, svcErr)
		suite.False(eligible)
	})
}

// No organization unit is not one a sign-in may land in, and nothing is asked.
func (suite *OUProviderTestSuite) TestIsEligibleOUWithoutAnOUIsNotEligible() {
	provider, _, _ := suite.eligibilityProvider()

	eligible, svcErr := provider.IsEligibleOU(suite.signInCtx(signInUserID), "")

	suite.Nil(svcErr)
	suite.False(eligible)
}

func (suite *OUProviderTestSuite) TestIsApplicationSharedAsksForTheSignInsApplication() {
	for _, tc := range []struct {
		shared bool
		svcErr *tidcommon.ServiceError
	}{
		{shared: true},
		{shared: false},
		{svcErr: &tidcommon.InternalServerError},
	} {
		provider, _, applications := suite.eligibilityProvider()
		applications.EXPECT().IsClientShared(mock.Anything, signInAppID).Return(tc.shared, tc.svcErr).Once()

		shared, svcErr := provider.IsApplicationShared(suite.signInCtx(signInUserID))

		suite.Equal(tc.svcErr, svcErr)
		suite.Equal(tc.shared, shared)
	}
}
