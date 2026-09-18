// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package dcr

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/application/model"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/applicationmock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
	"github.com/thunder-id/thunderid/tests/testhelpers"
)

const (
	svcTestClientID = "svc-client-id"
	svcTestAppID    = "svc-app-id"
)

// ClientConfigurationServiceTestSuite covers the RFC 7592 service operations.
type ClientConfigurationServiceTestSuite struct {
	suite.Suite
	mockAppService *applicationmock.ApplicationServiceInterfaceMock
	mockOUService  *oumock.OrganizationUnitServiceInterfaceMock
	service        DCRServiceInterface
}

func TestClientConfigurationServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ClientConfigurationServiceTestSuite))
}

func (s *ClientConfigurationServiceTestSuite) SetupTest() {
	s.mockAppService = applicationmock.NewApplicationServiceInterfaceMock(s.T())
	s.mockOUService = oumock.NewOrganizationUnitServiceInterfaceMock(s.T())
	s.service = newDCRService(s.mockAppService, s.mockOUService, nil, &MockTransactioner{},
		testhelpers.OAuthConfig())
}

func (s *ClientConfigurationServiceTestSuite) existingOAuthClient() *providers.OAuthClient {
	return &providers.OAuthClient{
		ID:           svcTestAppID,
		ClientID:     svcTestClientID,
		OUID:         "ou-1",
		RedirectURIs: []string{"https://client.example.com/cb"},
		GrantTypes:   []providers.GrantType{providers.GrantTypeAuthorizationCode},
		Scopes:       []string{"openid"},
	}
}

func (s *ClientConfigurationServiceTestSuite) existingApplication() *providers.Application {
	return &providers.Application{
		ID:      svcTestAppID,
		OUID:    "ou-1",
		Name:    "Existing Client",
		Type:    "custom",
		URL:     "https://client.example.com",
		LogoURL: "https://client.example.com/logo.png",
	}
}

// registeredApplicationDTO is the application the application service returns for a newly created
// registration.
func (s *ClientConfigurationServiceTestSuite) registeredApplicationDTO() *model.ApplicationDTO {
	return &model.ApplicationDTO{
		ID:   svcTestAppID,
		Name: "New Client",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID:     svcTestClientID,
					ClientSecret: "client-secret",
					RedirectURIs: []string{"https://client.example.com/cb"},
				},
			},
		},
	}
}

// expectUpdate primes the lookups and the write that a successful update performs, for tests whose
// subject is the response shape rather than the metadata being written.
func (s *ClientConfigurationServiceTestSuite) expectUpdate() {
	s.mockAppService.On("GetOAuthApplication", mock.Anything, svcTestClientID).
		Return(s.existingOAuthClient(), (*tidcommon.ServiceError)(nil))
	s.mockAppService.On("GetApplication", mock.Anything, svcTestAppID).
		Return(s.existingApplication(), (*tidcommon.ServiceError)(nil))
	s.mockAppService.On("UpdateApplication", mock.Anything, svcTestAppID,
		mock.AnythingOfType("*model.ApplicationDTO")).
		Return(&model.ApplicationDTO{
			ID:   svcTestAppID,
			Name: "Existing Client",
			InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
				{
					Type:        providers.OAuthInboundAuthType,
					OAuthConfig: &providers.OAuthConfigWithSecret{ClientID: svcTestClientID},
				},
			},
		}, (*tidcommon.ServiceError)(nil))
}

// UC-2: reading a registration returns the stored metadata.
func (s *ClientConfigurationServiceTestSuite) TestGetClient_ReturnsRegistration() {
	s.mockAppService.On("GetOAuthApplication", mock.Anything, svcTestClientID).
		Return(s.existingOAuthClient(), (*tidcommon.ServiceError)(nil))
	s.mockAppService.On("GetApplication", mock.Anything, svcTestAppID).
		Return(s.existingApplication(), (*tidcommon.ServiceError)(nil))

	response, err := s.service.GetClient(context.Background(), svcTestClientID)

	s.Require().Nil(err)
	s.Equal(svcTestClientID, response.ClientID)
	s.Equal("Existing Client", response.ClientName)
	s.Equal("https://client.example.com", response.ClientURI)
	s.Equal("openid", response.Scope)
	// The stored client secret cannot be read back, so it is never echoed.
	s.Empty(response.ClientSecret)
}

// A deleted client no longer resolves, so reading it reports not found.
func (s *ClientConfigurationServiceTestSuite) TestGetClient_UnknownClient() {
	s.mockAppService.On("GetOAuthApplication", mock.Anything, svcTestClientID).
		Return((*providers.OAuthClient)(nil), &tidcommon.ServiceError{
			Type: tidcommon.ClientErrorType, Code: "APP-1001",
		})

	response, err := s.service.GetClient(context.Background(), svcTestClientID)

	s.Nil(response)
	s.Require().NotNil(err)
	s.Equal(ErrorClientNotFound.Code, err.Code)
}

// UC-3: an update preserves the client identity while replacing the metadata.
func (s *ClientConfigurationServiceTestSuite) TestUpdateClient_PreservesClientIdentity() {
	s.mockAppService.On("GetOAuthApplication", mock.Anything, svcTestClientID).
		Return(s.existingOAuthClient(), (*tidcommon.ServiceError)(nil))
	s.mockAppService.On("GetApplication", mock.Anything, svcTestAppID).
		Return(s.existingApplication(), (*tidcommon.ServiceError)(nil))

	var capturedDTO *model.ApplicationDTO
	s.mockAppService.On("UpdateApplication", mock.Anything, svcTestAppID,
		mock.AnythingOfType("*model.ApplicationDTO")).
		Run(func(args mock.Arguments) {
			capturedDTO = args.Get(2).(*model.ApplicationDTO)
		}).
		Return(&model.ApplicationDTO{
			ID:   svcTestAppID,
			Name: "Renamed Client",
			InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
				{
					Type: providers.OAuthInboundAuthType,
					OAuthConfig: &providers.OAuthConfigWithSecret{
						ClientID:     svcTestClientID,
						RedirectURIs: []string{"https://client.example.com/updated"},
					},
				},
			},
		}, (*tidcommon.ServiceError)(nil))

	request := &DCRRegistrationRequest{
		ClientName:   "Renamed Client",
		RedirectURIs: []string{"https://client.example.com/updated"},
	}
	response, err := s.service.UpdateClient(context.Background(), svcTestClientID, request)

	s.Require().Nil(err)
	s.Equal(svcTestClientID, response.ClientID)
	s.Empty(response.ClientSecret, "an update must not rotate or expose the client secret")

	// The update carries the existing identity forward, so the application service preserves the
	// client_id, the client secret and the owning organization unit.
	s.Require().NotNil(capturedDTO)
	s.Equal(svcTestAppID, capturedDTO.ID)
	s.Equal("ou-1", capturedDTO.OUID)
	s.Empty(capturedDTO.Type, "the application type is immutable and must be inherited")
	s.Require().Len(capturedDTO.InboundAuthConfig, 1)
	s.Equal(svcTestClientID, capturedDTO.InboundAuthConfig[0].OAuthConfig.ClientID)
	s.Empty(capturedDTO.InboundAuthConfig[0].OAuthConfig.ClientSecret)
}

// An update that omits the client name keeps the registered one, since a name is required.
func (s *ClientConfigurationServiceTestSuite) TestUpdateClient_InheritsNameWhenOmitted() {
	s.mockAppService.On("GetOAuthApplication", mock.Anything, svcTestClientID).
		Return(s.existingOAuthClient(), (*tidcommon.ServiceError)(nil))
	s.mockAppService.On("GetApplication", mock.Anything, svcTestAppID).
		Return(s.existingApplication(), (*tidcommon.ServiceError)(nil))

	var capturedDTO *model.ApplicationDTO
	s.mockAppService.On("UpdateApplication", mock.Anything, svcTestAppID,
		mock.AnythingOfType("*model.ApplicationDTO")).
		Run(func(args mock.Arguments) {
			capturedDTO = args.Get(2).(*model.ApplicationDTO)
		}).
		Return(&model.ApplicationDTO{
			ID:   svcTestAppID,
			Name: "Existing Client",
			InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
				{
					Type:        providers.OAuthInboundAuthType,
					OAuthConfig: &providers.OAuthConfigWithSecret{ClientID: svcTestClientID},
				},
			},
		}, (*tidcommon.ServiceError)(nil))

	request := &DCRRegistrationRequest{RedirectURIs: []string{"https://client.example.com/cb"}}
	response, err := s.service.UpdateClient(context.Background(), svcTestClientID, request)

	s.Require().Nil(err)
	s.Require().NotNil(capturedDTO)
	s.Equal("Existing Client", capturedDTO.Name)

	// The response must report the name that was actually applied. Reporting the client ID here
	// would tell the caller the registration has a name it does not have.
	s.Require().NotNil(response)
	s.Equal("Existing Client", response.ClientName)
	s.NotEqual(svcTestClientID, response.ClientName)
}

// Updating an unknown client reports not found rather than creating one.
func (s *ClientConfigurationServiceTestSuite) TestUpdateClient_UnknownClient() {
	s.mockAppService.On("GetOAuthApplication", mock.Anything, svcTestClientID).
		Return((*providers.OAuthClient)(nil), &tidcommon.ServiceError{
			Type: tidcommon.ClientErrorType, Code: "APP-1001",
		})

	response, err := s.service.UpdateClient(context.Background(), svcTestClientID,
		&DCRRegistrationRequest{RedirectURIs: []string{"https://client.example.com/cb"}})

	s.Nil(response)
	s.Require().NotNil(err)
	s.Equal(ErrorClientNotFound.Code, err.Code)
	s.mockAppService.AssertNotCalled(s.T(), "UpdateApplication", mock.Anything, mock.Anything,
		mock.Anything)
}

// Conflicting JWKS configuration is rejected on update, as it is on registration.
func (s *ClientConfigurationServiceTestSuite) TestUpdateClient_JWKSConflict() {
	response, err := s.service.UpdateClient(context.Background(), svcTestClientID,
		&DCRRegistrationRequest{
			RedirectURIs: []string{"https://client.example.com/cb"},
			JWKSUri:      "https://client.example.com/jwks.json",
			JWKS:         map[string]interface{}{"keys": []interface{}{}},
		})

	s.Nil(response)
	s.Require().NotNil(err)
	s.Equal(ErrorJWKSConfigurationConflict.Code, err.Code)
}

// UC-4: deleting a registration removes the underlying application.
func (s *ClientConfigurationServiceTestSuite) TestDeleteClient_RemovesApplication() {
	s.mockAppService.On("GetOAuthApplication", mock.Anything, svcTestClientID).
		Return(s.existingOAuthClient(), (*tidcommon.ServiceError)(nil))
	s.mockAppService.On("GetApplication", mock.Anything, svcTestAppID).
		Return(s.existingApplication(), (*tidcommon.ServiceError)(nil))
	s.mockAppService.On("DeleteApplication", mock.Anything, svcTestAppID).
		Return((*tidcommon.ServiceError)(nil))

	err := s.service.DeleteClient(context.Background(), svcTestClientID)

	s.Nil(err)
	s.mockAppService.AssertCalled(s.T(), "DeleteApplication", mock.Anything, svcTestAppID)
}

// Deleting an unknown client reports not found.
func (s *ClientConfigurationServiceTestSuite) TestDeleteClient_UnknownClient() {
	s.mockAppService.On("GetOAuthApplication", mock.Anything, svcTestClientID).
		Return((*providers.OAuthClient)(nil), &tidcommon.ServiceError{
			Type: tidcommon.ClientErrorType, Code: "APP-1001",
		})

	err := s.service.DeleteClient(context.Background(), svcTestClientID)

	s.Require().NotNil(err)
	s.Equal(ErrorClientNotFound.Code, err.Code)
	s.mockAppService.AssertNotCalled(s.T(), "DeleteApplication", mock.Anything, mock.Anything)
}

// Registration returns no client configuration fields. Management is administrative, so no durable
// per-client credential is minted and no client configuration URI is advertised to the registrant.
func (s *ClientConfigurationServiceTestSuite) TestRegisterClient_OmitsClientConfigurationFields() {
	s.mockAppService.On("CreateApplication", mock.Anything,
		mock.AnythingOfType("*model.ApplicationDTO")).
		Return(s.registeredApplicationDTO(), (*tidcommon.ServiceError)(nil))

	response, err := s.service.RegisterClient(context.Background(), &DCRRegistrationRequest{
		OUID:         "ou-1",
		ClientName:   "New Client",
		RedirectURIs: []string{"https://client.example.com/cb"},
	})

	s.Require().Nil(err)
	s.NotEmpty(response.ClientID)
	s.NotZero(response.ClientIDIssuedAt)
}

// An update returns the replaced metadata without minting or rotating any management credential.
func (s *ClientConfigurationServiceTestSuite) TestUpdateClient_OmitsClientConfigurationFields() {
	s.expectUpdate()

	response, err := s.service.UpdateClient(context.Background(), svcTestClientID,
		&DCRRegistrationRequest{RedirectURIs: []string{"https://client.example.com/cb"}})

	s.Require().Nil(err)
	s.Equal(svcTestClientID, response.ClientID)
	s.Empty(response.ClientSecret, "an update must not rotate or expose the client secret")
}

// A failure to look up the OAuth client that is not a client error is reported as a server error,
// so a storage outage is not mistaken for a missing registration.
func (s *ClientConfigurationServiceTestSuite) TestGetClient_OAuthLookupServerError() {
	s.mockAppService.On("GetOAuthApplication", mock.Anything, svcTestClientID).
		Return((*providers.OAuthClient)(nil), &tidcommon.ServiceError{
			Type: tidcommon.ServerErrorType, Code: "SSE-5000",
		})

	response, err := s.service.GetClient(context.Background(), svcTestClientID)

	s.Nil(response)
	s.Require().NotNil(err)
	s.Equal(ErrorServerError.Code, err.Code)
}

// The application carrying the human-readable metadata is resolved separately, so its absence also
// reports the registration as not found.
func (s *ClientConfigurationServiceTestSuite) TestGetClient_ApplicationLookupNotFound() {
	s.mockAppService.On("GetOAuthApplication", mock.Anything, svcTestClientID).
		Return(s.existingOAuthClient(), (*tidcommon.ServiceError)(nil))
	s.mockAppService.On("GetApplication", mock.Anything, svcTestAppID).
		Return((*providers.Application)(nil), &tidcommon.ServiceError{
			Type: tidcommon.ClientErrorType, Code: "APP-1001",
		})

	response, err := s.service.GetClient(context.Background(), svcTestClientID)

	s.Nil(response)
	s.Require().NotNil(err)
	s.Equal(ErrorClientNotFound.Code, err.Code)
}

// A storage failure resolving the application is a server error, not a missing registration.
func (s *ClientConfigurationServiceTestSuite) TestGetClient_ApplicationLookupServerError() {
	s.mockAppService.On("GetOAuthApplication", mock.Anything, svcTestClientID).
		Return(s.existingOAuthClient(), (*tidcommon.ServiceError)(nil))
	s.mockAppService.On("GetApplication", mock.Anything, svcTestAppID).
		Return((*providers.Application)(nil), &tidcommon.ServiceError{
			Type: tidcommon.ServerErrorType, Code: "SSE-5000",
		})

	response, err := s.service.GetClient(context.Background(), svcTestClientID)

	s.Nil(response)
	s.Require().NotNil(err)
	s.Equal(ErrorServerError.Code, err.Code)
}

// A client registered with a JWKS URI has it echoed back on a read.
func (s *ClientConfigurationServiceTestSuite) TestGetClient_EchoesJWKSURI() {
	oauthClient := s.existingOAuthClient()
	oauthClient.Certificate = &providers.Certificate{
		Type:  providers.CertificateTypeJWKSURI,
		Value: "https://client.example.com/jwks.json",
	}
	s.mockAppService.On("GetOAuthApplication", mock.Anything, svcTestClientID).
		Return(oauthClient, (*tidcommon.ServiceError)(nil))
	s.mockAppService.On("GetApplication", mock.Anything, svcTestAppID).
		Return(s.existingApplication(), (*tidcommon.ServiceError)(nil))

	response, err := s.service.GetClient(context.Background(), svcTestClientID)

	s.Require().Nil(err)
	s.Equal("https://client.example.com/jwks.json", response.JWKSUri)
	s.Empty(response.JWKS)
}

// A client registered with an inline JWKS has it echoed back as a decoded object.
func (s *ClientConfigurationServiceTestSuite) TestGetClient_EchoesInlineJWKS() {
	oauthClient := s.existingOAuthClient()
	oauthClient.Certificate = &providers.Certificate{
		Type:  providers.CertificateTypeJWKS,
		Value: `{"keys":[{"kty":"RSA","kid":"k1"}]}`,
	}
	s.mockAppService.On("GetOAuthApplication", mock.Anything, svcTestClientID).
		Return(oauthClient, (*tidcommon.ServiceError)(nil))
	s.mockAppService.On("GetApplication", mock.Anything, svcTestAppID).
		Return(s.existingApplication(), (*tidcommon.ServiceError)(nil))

	response, err := s.service.GetClient(context.Background(), svcTestClientID)

	s.Require().Nil(err)
	s.NotEmpty(response.JWKS)
	s.Empty(response.JWKSUri)
}

// A stored JWKS that is not valid JSON cannot be rendered, so the read reports a server error
// rather than returning a half-built registration.
func (s *ClientConfigurationServiceTestSuite) TestGetClient_MalformedStoredJWKS() {
	oauthClient := s.existingOAuthClient()
	oauthClient.Certificate = &providers.Certificate{
		Type:  providers.CertificateTypeJWKS,
		Value: "not-json",
	}
	s.mockAppService.On("GetOAuthApplication", mock.Anything, svcTestClientID).
		Return(oauthClient, (*tidcommon.ServiceError)(nil))
	s.mockAppService.On("GetApplication", mock.Anything, svcTestAppID).
		Return(s.existingApplication(), (*tidcommon.ServiceError)(nil))

	response, err := s.service.GetClient(context.Background(), svcTestClientID)

	s.Nil(response)
	s.Require().NotNil(err)
	s.Equal(ErrorServerError.Code, err.Code)
}

// The signing and encryption algorithms registered for the userinfo and ID token responses are
// echoed back, so a read reflects the full protocol configuration.
func (s *ClientConfigurationServiceTestSuite) TestGetClient_EchoesResponseAlgorithms() {
	oauthClient := s.existingOAuthClient()
	oauthClient.UserInfo = &providers.UserInfoConfig{
		SigningAlg:    "RS256",
		EncryptionAlg: "RSA-OAEP",
		EncryptionEnc: "A256GCM",
	}
	oauthClient.Token = &providers.OAuthTokenConfig{
		IDToken: &providers.IDTokenConfig{
			SigningAlg:    "ES256",
			EncryptionAlg: "ECDH-ES",
			EncryptionEnc: "A128GCM",
		},
	}
	s.mockAppService.On("GetOAuthApplication", mock.Anything, svcTestClientID).
		Return(oauthClient, (*tidcommon.ServiceError)(nil))
	s.mockAppService.On("GetApplication", mock.Anything, svcTestAppID).
		Return(s.existingApplication(), (*tidcommon.ServiceError)(nil))

	response, err := s.service.GetClient(context.Background(), svcTestClientID)

	s.Require().Nil(err)
	s.Equal("RS256", response.UserInfoSignedResponseAlg)
	s.Equal("RSA-OAEP", response.UserInfoEncryptedResponseAlg)
	s.Equal("A256GCM", response.UserInfoEncryptedResponseEnc)
	s.Equal("ES256", response.IDTokenSignedResponseAlg)
	s.Equal("ECDH-ES", response.IDTokenEncryptedResponseAlg)
	s.Equal("A128GCM", response.IDTokenEncryptedResponseEnc)
}

// A storage failure deleting the application is reported as a server error, so the caller can tell
// a failed delete from a registration that was already gone.
func (s *ClientConfigurationServiceTestSuite) TestDeleteClient_DeleteServerError() {
	s.mockAppService.On("GetOAuthApplication", mock.Anything, svcTestClientID).
		Return(s.existingOAuthClient(), (*tidcommon.ServiceError)(nil))
	s.mockAppService.On("GetApplication", mock.Anything, svcTestAppID).
		Return(s.existingApplication(), (*tidcommon.ServiceError)(nil))
	s.mockAppService.On("DeleteApplication", mock.Anything, svcTestAppID).
		Return(&tidcommon.ServiceError{Type: tidcommon.ServerErrorType, Code: "SSE-5000"})

	err := s.service.DeleteClient(context.Background(), svcTestClientID)

	s.Require().NotNil(err)
	s.Equal(ErrorServerError.Code, err.Code)
}

// A client error refusing the delete is surfaced through the DCR error mapping.
func (s *ClientConfigurationServiceTestSuite) TestDeleteClient_DeleteClientError() {
	s.mockAppService.On("GetOAuthApplication", mock.Anything, svcTestClientID).
		Return(s.existingOAuthClient(), (*tidcommon.ServiceError)(nil))
	s.mockAppService.On("GetApplication", mock.Anything, svcTestAppID).
		Return(s.existingApplication(), (*tidcommon.ServiceError)(nil))
	s.mockAppService.On("DeleteApplication", mock.Anything, svcTestAppID).
		Return(&tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "APP-1030"})

	err := s.service.DeleteClient(context.Background(), svcTestClientID)

	s.Require().NotNil(err)
	s.Equal(tidcommon.ClientErrorType, err.Type)
}

// An update with no body at all is rejected before any lookup.
func (s *ClientConfigurationServiceTestSuite) TestUpdateClient_NilRequest() {
	response, err := s.service.UpdateClient(context.Background(), svcTestClientID, nil)

	s.Nil(response)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidRequestFormat.Code, err.Code)
	s.mockAppService.AssertNotCalled(s.T(), "GetOAuthApplication", mock.Anything, mock.Anything)
}

// A JWKS URI that is not an absolute HTTPS URL is rejected as invalid metadata.
func (s *ClientConfigurationServiceTestSuite) TestUpdateClient_InsecureJWKSURI() {
	response, err := s.service.UpdateClient(context.Background(), svcTestClientID,
		&DCRRegistrationRequest{JWKSUri: "http://client.example.com/jwks.json"})

	s.Nil(response)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidClientMetadata.Code, err.Code)
	s.mockAppService.AssertNotCalled(s.T(), "GetOAuthApplication", mock.Anything, mock.Anything)
}
