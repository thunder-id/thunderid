// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package actorprovider

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/application"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	syscontext "github.com/thunder-id/thunderid/internal/system/context"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/applicationmock"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/inboundclientmock"
)

const (
	admissionAppID    = "app-1"
	admissionClientID = "client-1"
	admissionOwnerOU  = "ou-owner"
	admissionOtherOU  = "ou-customer"
)

// AdmissionTestSuite covers the check that fuses client resolution with whether the client may be
// used for the organization unit the request named.
type AdmissionTestSuite struct {
	suite.Suite
	mockInbound *inboundclientmock.InboundClientServiceInterfaceMock
	mockAppOU   *applicationmock.ApplicationServiceInterfaceMock
}

func TestAdmissionTestSuite(t *testing.T) {
	suite.Run(t, new(AdmissionTestSuite))
}

func (s *AdmissionTestSuite) SetupTest() {
	s.mockInbound = inboundclientmock.NewInboundClientServiceInterfaceMock(s.T())
	s.mockAppOU = applicationmock.NewApplicationServiceInterfaceMock(s.T())
}

// resolve runs client resolution for a request naming accessingOU. The client is always owned by
// admissionOwnerOU; what varies is which organization unit the request asks to act for.
func (s *AdmissionTestSuite) resolve(
	accessingOU string, appService application.ApplicationServiceInterface,
) (*providers.OAuthClient, *tidcommon.ServiceError) {
	s.mockInbound.On("GetOAuthClientByClientID", mock.Anything, admissionClientID).
		Return(&providers.OAuthClient{
			ID: admissionAppID, ClientID: admissionClientID, OUID: admissionOwnerOU,
		}, nil)

	provider := Initialize(s.mockInbound, entityprovidermock.NewEntityProviderInterfaceMock(s.T()),
		managermock.NewAuthnProviderManagerMock(s.T()), nil, appService)

	ctx := context.Background()
	if accessingOU != "" {
		ctx = syscontext.WithAccessingOUID(ctx, accessingOU)
	}
	return provider.GetOAuthClientByClientID(ctx, admissionClientID)
}

// The bare token endpoint names no organization unit, so resolution stays exactly as it was.
func (s *AdmissionTestSuite) TestNoAccessingOULeavesResolutionUntouched() {
	client, svcErr := s.resolve("", s.mockAppOU)

	s.Require().Nil(svcErr)
	s.Require().NotNil(client)
	s.mockAppOU.AssertNotCalled(s.T(), "IsApplicationVisibleToOU", mock.Anything, mock.Anything, mock.Anything)
}

// An application's own organization unit needs no policy, and asking for one would put a sharing
// lookup on every token request an application makes in its own organization.
func (s *AdmissionTestSuite) TestOwnOrganizationUnitNeedsNoPolicy() {
	client, svcErr := s.resolve(admissionOwnerOU, s.mockAppOU)

	s.Require().Nil(svcErr)
	s.Require().NotNil(client)
	s.mockAppOU.AssertNotCalled(s.T(), "IsApplicationVisibleToOU", mock.Anything, mock.Anything, mock.Anything)
}

func (s *AdmissionTestSuite) TestReachedOrganizationUnitResolves() {
	s.mockAppOU.On("IsApplicationVisibleToOU", mock.Anything, admissionAppID, admissionOtherOU).
		Return(true, (*tidcommon.ServiceError)(nil))

	client, svcErr := s.resolve(admissionOtherOU, s.mockAppOU)

	s.Require().Nil(svcErr)
	s.Require().NotNil(client)
}

// The client exists and its credentials are fine; it simply may not act for this organization unit,
// so it does not resolve at all.
func (s *AdmissionTestSuite) TestUnreachedOrganizationUnitDoesNotResolve() {
	s.mockAppOU.On("IsApplicationVisibleToOU", mock.Anything, admissionAppID, admissionOtherOU).
		Return(false, (*tidcommon.ServiceError)(nil))

	client, svcErr := s.resolve(admissionOtherOU, s.mockAppOU)

	s.Nil(client)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
}

// Fail closed. Without a checker there is no way to establish a policy exists, and admitting on that
// basis would hand every organization unit every application.
func (s *AdmissionTestSuite) TestMissingCheckerRefusesEveryOtherOrganizationUnit() {
	client, svcErr := s.resolve(admissionOtherOU, nil)

	s.Nil(client)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
}

// A failed lookup is not a pass: an incomplete check must not read as an allowed one.
func (s *AdmissionTestSuite) TestLookupFailureRefuses() {
	s.mockAppOU.On("IsApplicationVisibleToOU", mock.Anything, admissionAppID, admissionOtherOU).
		Return(false, &tidcommon.InternalServerError)

	client, svcErr := s.resolve(admissionOtherOU, s.mockAppOU)

	s.Nil(client)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

// A client that does not exist is still a missing client, whatever organization unit was named.
func (s *AdmissionTestSuite) TestUnknownClientIsStillNotFound() {
	s.mockInbound.On("GetOAuthClientByClientID", mock.Anything, "missing").
		Return((*providers.OAuthClient)(nil), inboundclient.ErrInboundClientNotFound)

	provider := Initialize(s.mockInbound, entityprovidermock.NewEntityProviderInterfaceMock(s.T()),
		managermock.NewAuthnProviderManagerMock(s.T()), nil, s.mockAppOU)

	client, svcErr := provider.GetOAuthClientByClientID(
		syscontext.WithAccessingOUID(context.Background(), admissionOtherOU), "missing")

	s.Nil(client)
	s.Require().NotNil(svcErr)
	s.Equal(ErrorActorNotFound.Code, svcErr.Code)
}
