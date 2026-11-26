// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package extensionauth

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/connection/httpconnection"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/authn/oauthmock"
	"github.com/thunder-id/thunderid/tests/mocks/idpprovidermock"
)

const testConnectionID = "conn-1"

type ExtensionAuthnServiceTestSuite struct {
	suite.Suite
	idpService *idpprovidermock.IDPProviderMock
	oauth      *oauthmock.OAuthAuthnCoreServiceInterfaceMock
	service    ExtensionAuthnServiceInterface
}

func TestExtensionAuthnServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ExtensionAuthnServiceTestSuite))
}

func (s *ExtensionAuthnServiceTestSuite) SetupTest() {
	s.idpService = idpprovidermock.NewIDPProviderMock(s.T())
	s.oauth = oauthmock.NewOAuthAuthnCoreServiceInterfaceMock(s.T())
	s.service = Initialize(s.idpService, s.oauth)
}

func (s *ExtensionAuthnServiceTestSuite) property(name, value string) cmodels.Property {
	prop, err := cmodels.NewProperty(name, value, false)
	s.Require().NoError(err)
	return *prop
}

func (s *ExtensionAuthnServiceTestSuite) expectConnection(props ...cmodels.Property) {
	s.idpService.EXPECT().GetIdentityProvider(mock.Anything, testConnectionID).
		Return(&providers.IDPDTO{ID: testConnectionID, Type: providers.IDPTypeExtensionAuthentication,
			Properties: props}, nil)
}

func (s *ExtensionAuthnServiceTestSuite) TestResolvedConnectionCarriesEndpointTimeoutAndCredential() {
	s.expectConnection(
		s.property(idp.PropEndpointURL, "https://auth.example.com/verify"),
		s.property(idp.PropTimeoutMS, "2500"),
		s.property(idp.PropAuthUsername, "svc"),
		s.property(idp.PropAuthPassword, "s3cret"))

	resolved, svcErr := s.service.GetResolvedConnection(context.Background(), testConnectionID)

	s.Require().Nil(svcErr)
	s.Equal("https://auth.example.com/verify", resolved.URL)
	s.Equal(2500*time.Millisecond, resolved.Timeout)
	request := &http.Request{Header: resolved.Headers}
	username, password, ok := request.BasicAuth()
	s.True(ok)
	s.Equal("svc", username)
	s.Equal("s3cret", password)
}

func (s *ExtensionAuthnServiceTestSuite) TestResolvedConnectionDefaultsTheTimeoutAndSendsNoCredential() {
	s.expectConnection(s.property(idp.PropEndpointURL, "https://auth.example.com/verify"))

	resolved, svcErr := s.service.GetResolvedConnection(context.Background(), testConnectionID)

	s.Require().Nil(svcErr)
	s.Equal(10*time.Second, resolved.Timeout)
	s.Empty(resolved.Headers.Get("Authorization"))
}

func (s *ExtensionAuthnServiceTestSuite) TestResolvedConnectionRefusesWhatIsNotAnAuthenticationEndpoint() {
	s.idpService.EXPECT().GetIdentityProvider(mock.Anything, testConnectionID).
		Return(&providers.IDPDTO{ID: testConnectionID, Type: providers.IDPTypeGoogle}, nil)

	resolved, svcErr := s.service.GetResolvedConnection(context.Background(), testConnectionID)

	s.Nil(resolved)
	s.Equal(httpconnection.ErrorNotFound.Code, svcErr.Code)
}

func (s *ExtensionAuthnServiceTestSuite) TestResolvedConnectionReportsMissingAndFailedLookups() {
	s.idpService.EXPECT().GetIdentityProvider(mock.Anything, "missing").
		Return(nil, &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "IDP-1003"})
	s.idpService.EXPECT().GetIdentityProvider(mock.Anything, "broken").
		Return(nil, &tidcommon.ServiceError{Type: tidcommon.ServerErrorType, Code: "IDP-5000"})

	_, svcErr := s.service.GetResolvedConnection(context.Background(), "missing")
	s.Equal(httpconnection.ErrorNotFound.Code, svcErr.Code)
	_, svcErr = s.service.GetResolvedConnection(context.Background(), "broken")
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *ExtensionAuthnServiceTestSuite) TestAuthenticateBuildsTheResultThroughTheFederatedPath() {
	s.expectConnection(s.property(idp.PropEndpointURL, "https://auth.example.com/verify"))
	claims := map[string]interface{}{"email": "alice@example.com"}
	expected := &common.AuthnResult{Token: map[string]interface{}{"sub": "ext-42"}}
	s.oauth.EXPECT().BuildFederatedAuthResult(mock.Anything, testConnectionID, "ext-42", claims).
		Return(expected, (*tidcommon.ServiceError)(nil))

	result, svcErr := s.service.Authenticate(context.Background(), &common.ExtensionVerifiedCredential{
		ConnectionID: testConnectionID, Subject: "ext-42", Claims: claims})

	s.Nil(svcErr)
	s.Same(expected, result)
}

func (s *ExtensionAuthnServiceTestSuite) TestAuthenticateRefusesAnIncompleteCredential() {
	for _, credential := range []*common.ExtensionVerifiedCredential{
		nil,
		{Subject: "ext-42"},
		{ConnectionID: testConnectionID, Subject: " "},
	} {
		result, svcErr := s.service.Authenticate(context.Background(), credential)

		s.Nil(result)
		s.NotNil(svcErr)
	}
}
