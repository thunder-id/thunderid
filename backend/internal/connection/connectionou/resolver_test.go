// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connectionou

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
)

var errServer = &tidcommon.ServiceError{Type: tidcommon.ServerErrorType, Code: "SSE-5000"}

type ResolverTestSuite struct {
	suite.Suite
	ouService *oumock.OrganizationUnitServiceInterfaceMock
	authz     *sysauthzmock.SystemAuthorizationServiceInterfaceMock
	resolver  ResolverInterface
}

func TestResolverTestSuite(t *testing.T) {
	suite.Run(t, new(ResolverTestSuite))
}

func (s *ResolverTestSuite) SetupTest() {
	s.ouService = oumock.NewOrganizationUnitServiceInterfaceMock(s.T())
	s.authz = sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
	s.resolver = Initialize(s.ouService, s.authz, Errors{
		Required: ErrorOURequired,
		NotFound: ErrorInvalidOU,
		Changed:  ErrorOUUpdateNotAllowed,
	})
}

func (s *ResolverTestSuite) TestResolve_RequiresAnIDOrAHandle() {
	_, svcErr := s.resolver.Resolve(context.Background(), "", "")
	s.Equal(&ErrorOURequired, svcErr)
}

func (s *ResolverTestSuite) TestResolve_UsesAnIDThatExists() {
	s.ouService.On("IsOrganizationUnitExists", mock.Anything, "ou-1").Return(true, nil)

	id, svcErr := s.resolver.Resolve(context.Background(), "ou-1", "")

	s.Nil(svcErr)
	s.Equal("ou-1", id)
}

func (s *ResolverTestSuite) TestResolve_LetsTheIDWinWhenBothAreGiven() {
	s.ouService.On("IsOrganizationUnitExists", mock.Anything, "ou-1").Return(true, nil)

	id, svcErr := s.resolver.Resolve(context.Background(), "ou-1", "acme")

	s.Nil(svcErr)
	s.Equal("ou-1", id)
}

func (s *ResolverTestSuite) TestResolve_RefusesAnIDThatDoesNotExist() {
	s.ouService.On("IsOrganizationUnitExists", mock.Anything, "missing").Return(false, nil)

	_, svcErr := s.resolver.Resolve(context.Background(), "missing", "")

	s.Equal(&ErrorInvalidOU, svcErr)
}

func (s *ResolverTestSuite) TestResolve_ReportsAFailedIDLookup() {
	s.ouService.On("IsOrganizationUnitExists", mock.Anything, "ou-1").Return(false, errServer)

	_, svcErr := s.resolver.Resolve(context.Background(), "ou-1", "")

	s.Equal(&tidcommon.InternalServerError, svcErr)
}

func (s *ResolverTestSuite) TestResolve_ResolvesAHandle() {
	s.ouService.On("GetOrganizationUnitByPath", mock.Anything, "acme").
		Return(ou.OrganizationUnit{ID: "ou-acme"}, nil)

	id, svcErr := s.resolver.Resolve(context.Background(), "", "acme")

	s.Nil(svcErr)
	s.Equal("ou-acme", id)
}

func (s *ResolverTestSuite) TestResolve_RefusesAHandleThatDoesNotExist() {
	s.ouService.On("GetOrganizationUnitByPath", mock.Anything, "nowhere").
		Return(ou.OrganizationUnit{}, &ou.ErrorOrganizationUnitNotFound)

	_, svcErr := s.resolver.Resolve(context.Background(), "", "nowhere")

	s.Equal(&ErrorInvalidOU, svcErr)
}

func (s *ResolverTestSuite) TestResolve_DoesNotTakeAFailedHandleLookupForAMissingUnit() {
	s.ouService.On("GetOrganizationUnitByPath", mock.Anything, "acme").
		Return(ou.OrganizationUnit{}, errServer)

	_, svcErr := s.resolver.Resolve(context.Background(), "", "acme")

	s.Equal(&tidcommon.InternalServerError, svcErr)
}

func (s *ResolverTestSuite) TestResolveForUpdate_KeepsTheCurrentOwnerWhenNoneIsRequested() {
	id, svcErr := s.resolver.ResolveForUpdate(context.Background(), "", "", "ou-1")

	s.Nil(svcErr)
	s.Equal("ou-1", id)
}

func (s *ResolverTestSuite) TestResolveForUpdate_AcceptsTheCurrentOwnerByID() {
	id, svcErr := s.resolver.ResolveForUpdate(context.Background(), "ou-1", "", "ou-1")

	s.Nil(svcErr)
	s.Equal("ou-1", id)
}

func (s *ResolverTestSuite) TestResolveForUpdate_AcceptsTheCurrentOwnerByHandle() {
	s.ouService.On("GetOrganizationUnitByPath", mock.Anything, "acme").
		Return(ou.OrganizationUnit{ID: "ou-1"}, nil)

	id, svcErr := s.resolver.ResolveForUpdate(context.Background(), "", "acme", "ou-1")

	s.Nil(svcErr)
	s.Equal("ou-1", id)
}

func (s *ResolverTestSuite) TestResolveForUpdate_RefusesAnotherOwner() {
	_, svcErr := s.resolver.ResolveForUpdate(context.Background(), "ou-2", "", "ou-1")

	s.Equal(&ErrorOUUpdateNotAllowed, svcErr)
}

func (s *ResolverTestSuite) TestResolveForUpdate_RefusesAHandleThatCannotBeResolved() {
	s.ouService.On("GetOrganizationUnitByPath", mock.Anything, "nowhere").
		Return(ou.OrganizationUnit{}, &ou.ErrorOrganizationUnitNotFound)

	_, svcErr := s.resolver.ResolveForUpdate(context.Background(), "", "nowhere", "ou-1")

	s.Equal(&ErrorInvalidOU, svcErr)
}

func (s *ResolverTestSuite) TestAuthorize() {
	actionCtx := &sysauthz.ActionContext{
		ResourceType: security.ResourceTypeConnection, OUID: "ou-1", ResourceID: "conn-1",
	}
	cases := []struct {
		name     string
		allowed  bool
		authzErr *tidcommon.ServiceError
		wantCode string
	}{
		{"Allowed", true, nil, ""},
		{"Refused", false, nil, tidcommon.ErrorUnauthorized.Code},
		{"AuthorizationFailure", false, errServer, tidcommon.InternalServerError.Code},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.SetupTest()
			s.authz.On("IsActionAllowed", mock.Anything, security.ActionUpdateConnection, actionCtx).
				Return(tc.allowed, tc.authzErr).Once()

			svcErr := s.resolver.Authorize(context.Background(), security.ActionUpdateConnection, "ou-1", "conn-1")

			if tc.wantCode == "" {
				s.Nil(svcErr)
				return
			}
			s.Require().NotNil(svcErr)
			s.Equal(tc.wantCode, svcErr.Code)
		})
	}
}
