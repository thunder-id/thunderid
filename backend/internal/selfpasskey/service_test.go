// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package selfpasskey

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/authn/passkey"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/tests/mocks/authn/passkeymock"
	"github.com/thunder-id/thunderid/tests/mocks/inboundclientmock"
)

const (
	testUserID   = "user-1"
	testClientID = "client-1"
	testAppID    = "app-1"
)

type ServiceTestSuite struct {
	suite.Suite
	passkeyService *passkeymock.PasskeyServiceInterfaceMock
	inboundClient  *inboundclientmock.InboundClientServiceInterfaceMock
	service        selfPasskeyServiceInterface
}

func TestServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ServiceTestSuite))
}

func (s *ServiceTestSuite) SetupTest() {
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime("", &config.Config{
		Passkey: config.PasskeyConfig{AllowedOrigins: []string{"https://server.example.org"}},
	}))
	s.passkeyService = passkeymock.NewPasskeyServiceInterfaceMock(s.T())
	s.inboundClient = inboundclientmock.NewInboundClientServiceInterfaceMock(s.T())
	s.service = newSelfPasskeyService(s.passkeyService, s.inboundClient)
}

func (s *ServiceTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (s *ServiceTestSuite) expectAppOrigins(origins []string) {
	s.expectApp(&providers.InboundClient{PasskeyAllowedOrigins: origins})
}

func (s *ServiceTestSuite) expectApp(app *providers.InboundClient) {
	s.inboundClient.On("GetOAuthClientByClientID", mock.Anything, testClientID).
		Return(&providers.OAuthClient{ID: testAppID, ClientID: testClientID}, nil).Once()
	s.inboundClient.On("GetInboundClientByEntityID", mock.Anything, testAppID).Return(app, nil).Once()
}

func (s *ServiceTestSuite) TestStartRegistration_UsesAppOrigins() {
	origins := []string{"https://login.example.com", "android:apk-key-hash:abc"}
	s.expectAppOrigins(origins)
	expected := &passkey.PasskeyRegistrationStartData{SessionToken: "token"}
	s.passkeyService.On("StartRegistration", mock.Anything, &passkey.PasskeyRegistrationStartRequest{
		UserID:         testUserID,
		RelyingPartyID: "example.com",
		AllowedOrigins: origins,
	}).Return(expected, nil).Once()

	data, svcErr := s.service.StartRegistration(context.Background(), testUserID, testClientID, " example.com ")

	s.Nil(svcErr)
	s.Equal(expected, data)
}

func (s *ServiceTestSuite) TestStartRegistration_FallsBackToServerOrigins() {
	s.expectAppOrigins(nil)
	s.passkeyService.On("StartRegistration", mock.Anything, mock.MatchedBy(
		func(req *passkey.PasskeyRegistrationStartRequest) bool {
			return assert.ObjectsAreEqual([]string{"https://server.example.org"}, req.AllowedOrigins)
		})).Return(&passkey.PasskeyRegistrationStartData{}, nil).Once()

	_, svcErr := s.service.StartRegistration(context.Background(), testUserID, testClientID, "example.org")

	s.Nil(svcErr)
}

func (s *ServiceTestSuite) TestStartRegistration_UnknownAppFallsBackToServerOrigins() {
	s.inboundClient.On("GetOAuthClientByClientID", mock.Anything, testClientID).Return(nil, nil).Once()
	s.passkeyService.On("StartRegistration", mock.Anything, mock.Anything).
		Return(&passkey.PasskeyRegistrationStartData{}, nil).Once()

	_, svcErr := s.service.StartRegistration(context.Background(), testUserID, testClientID, "server.example.org")

	s.Nil(svcErr)
}

func (s *ServiceTestSuite) TestStartRegistration_MissingInboundClientFallsBackToServerOrigins() {
	s.inboundClient.On("GetOAuthClientByClientID", mock.Anything, testClientID).
		Return(&providers.OAuthClient{ID: testAppID}, nil).Once()
	s.inboundClient.On("GetInboundClientByEntityID", mock.Anything, testAppID).
		Return(nil, inboundclient.ErrInboundClientNotFound).Once()
	s.passkeyService.On("StartRegistration", mock.Anything, mock.Anything).
		Return(&passkey.PasskeyRegistrationStartData{}, nil).Once()

	_, svcErr := s.service.StartRegistration(context.Background(), testUserID, testClientID, "example.org")

	s.Nil(svcErr)
}

func (s *ServiceTestSuite) TestStartRegistration_RelyingPartyNotAllowed() {
	cases := []string{"evil.com", "ample.com", "login.example.com.evil.com"}
	for _, rpID := range cases {
		s.expectAppOrigins([]string{"https://login.example.com", "android:apk-key-hash:abc", "::bad"})

		data, svcErr := s.service.StartRegistration(context.Background(), testUserID, testClientID, rpID)

		s.Nil(data, rpID)
		s.Equal(ErrorRelyingPartyNotAllowed.Code, svcErr.Code, rpID)
	}
}

func (s *ServiceTestSuite) TestStartRegistration_EmptyRelyingParty() {
	s.expectAppOrigins([]string{"https://login.example.com"})

	data, svcErr := s.service.StartRegistration(context.Background(), testUserID, testClientID, " ")

	s.Nil(data)
	s.Equal(passkey.ErrorEmptyRelyingPartyID.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestStartRegistration_ClientLookupError() {
	s.inboundClient.On("GetOAuthClientByClientID", mock.Anything, testClientID).
		Return(nil, assert.AnError).Once()

	data, svcErr := s.service.StartRegistration(context.Background(), testUserID, testClientID, "example.com")

	s.Nil(data)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestStartRegistration_AppLookupError() {
	s.inboundClient.On("GetOAuthClientByClientID", mock.Anything, testClientID).
		Return(&providers.OAuthClient{ID: testAppID}, nil).Once()
	s.inboundClient.On("GetInboundClientByEntityID", mock.Anything, testAppID).
		Return(nil, assert.AnError).Once()

	data, svcErr := s.service.StartRegistration(context.Background(), testUserID, testClientID, "example.com")

	s.Nil(data)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestFinishRegistration_Success() {
	origins := []string{"https://login.example.com"}
	s.expectAppOrigins(origins)
	req := &registrationFinishRequest{
		PublicKeyCredential: publicKeyCredential{
			ID:   "cred-1",
			Type: "public-key",
			Response: attestationResponse{
				ClientDataJSON:    "client-data",
				AttestationObject: "attestation",
			},
		},
		SessionToken: "token",
		Name:         " Work laptop ",
	}
	s.passkeyService.On("FinishRegistration", mock.Anything, &passkey.PasskeyRegistrationFinishRequest{
		CredentialID:      "cred-1",
		CredentialType:    "public-key",
		ClientDataJSON:    "client-data",
		AttestationObject: "attestation",
		SessionToken:      "token",
		AllowedOrigins:    origins,
		UserID:            testUserID,
		Name:              "Work laptop",
	}).Return(nil, nil).Once()

	saved, svcErr := s.service.FinishRegistration(context.Background(), testUserID, testClientID, req)

	s.Nil(svcErr)
	s.Equal(&passkey.PasskeyInfo{ID: "cred-1", Name: "Work laptop"}, saved)
}

func (s *ServiceTestSuite) TestFinishRegistration_PasskeyServiceError() {
	s.expectAppOrigins([]string{"https://login.example.com"})
	s.passkeyService.On("FinishRegistration", mock.Anything, mock.Anything).
		Return(nil, &passkey.ErrorInvalidSessionToken).Once()

	saved, svcErr := s.service.FinishRegistration(context.Background(), testUserID, testClientID,
		&registrationFinishRequest{})

	s.Nil(saved)
	s.Equal(passkey.ErrorInvalidSessionToken.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestFinishRegistration_OriginLookupError() {
	s.inboundClient.On("GetOAuthClientByClientID", mock.Anything, testClientID).
		Return(nil, assert.AnError).Once()

	saved, svcErr := s.service.FinishRegistration(context.Background(), testUserID, testClientID,
		&registrationFinishRequest{})

	s.Nil(saved)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestListPasskeys() {
	expected := []passkey.PasskeyInfo{{ID: "cred-1", Name: "Work laptop"}}
	s.passkeyService.On("ListPasskeys", mock.Anything, testUserID).Return(expected, nil).Once()

	passkeys, svcErr := s.service.ListPasskeys(context.Background(), testUserID)

	s.Nil(svcErr)
	s.Equal(expected, passkeys)
}

func (s *ServiceTestSuite) TestDeletePasskey() {
	s.passkeyService.On("DeletePasskey", mock.Anything, testUserID, "cred-1").Return(nil).Once()

	s.Nil(s.service.DeletePasskey(context.Background(), testUserID, "cred-1"))
}

func (s *ServiceTestSuite) TestStartRegistration_DefaultsToAppRelyingParty() {
	origins := []string{"android:apk-key-hash:abc"}
	s.expectApp(&providers.InboundClient{PasskeyAllowedOrigins: origins, PasskeyRelyingPartyID: "example.com"})
	s.passkeyService.On("StartRegistration", mock.Anything, &passkey.PasskeyRegistrationStartRequest{
		UserID:         testUserID,
		RelyingPartyID: "example.com",
		AllowedOrigins: origins,
	}).Return(&passkey.PasskeyRegistrationStartData{}, nil).Once()

	_, svcErr := s.service.StartRegistration(context.Background(), testUserID, testClientID, "")

	s.Nil(svcErr)
}

func (s *ServiceTestSuite) TestStartRegistration_AcceptsAppRelyingPartyWithoutWebOrigin() {
	s.expectApp(&providers.InboundClient{
		PasskeyAllowedOrigins: []string{"android:apk-key-hash:abc"},
		PasskeyRelyingPartyID: "example.com",
	})
	s.passkeyService.On("StartRegistration", mock.Anything, mock.Anything).
		Return(&passkey.PasskeyRegistrationStartData{}, nil).Once()

	_, svcErr := s.service.StartRegistration(context.Background(), testUserID, testClientID, "example.com")

	s.Nil(svcErr)
}

func (s *ServiceTestSuite) TestStartRegistration_RejectsOtherRelyingPartyWhenAppSetsOne() {
	s.expectApp(&providers.InboundClient{
		PasskeyAllowedOrigins: []string{"android:apk-key-hash:abc"},
		PasskeyRelyingPartyID: "example.com",
	})

	data, svcErr := s.service.StartRegistration(context.Background(), testUserID, testClientID, "evil.com")

	s.Nil(data)
	s.Equal(ErrorRelyingPartyNotAllowed.Code, svcErr.Code)
}
