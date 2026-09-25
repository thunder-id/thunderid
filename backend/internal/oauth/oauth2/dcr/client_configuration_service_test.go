// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package dcr

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/application"
	"github.com/thunder-id/thunderid/internal/application/model"
	i18nmgt "github.com/thunder-id/thunderid/internal/system/i18n/mgt"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/applicationmock"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/i18n/mgtmock"
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
	s.service = newDCRService(s.mockAppService, s.mockOUService, nil, nil, &MockTransactioner{},
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

// An update carrying localized names must key its i18n reference off the existing application ID,
// because the variants are written under that ID. A reference built from a freshly generated ID
// would never resolve.
func (s *ClientConfigurationServiceTestSuite) TestUpdateClient_LocalizedNameRefUsesExistingAppID() {
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

	request := &DCRRegistrationRequest{
		RedirectURIs:        []string{"https://client.example.com/cb"},
		LocalizedClientName: map[string]string{"fr": "Mon Application"},
	}
	_, err := s.service.UpdateClient(context.Background(), svcTestClientID, request)

	s.Require().Nil(err)
	s.Require().NotNil(capturedDTO)
	s.Equal(application.AppI18nRef(svcTestAppID, "name"), capturedDTO.Name)
}

// An update replaces the localized variants of every field it supplies, so a caller reading a
// registration in order to modify and write it back must be able to see the variants it would be
// replacing. Registration only echoes the variants from its own request, so a read has to resolve
// them from the i18n store.
func (s *ClientConfigurationServiceTestSuite) TestGetClient_ReturnsStoredLocalizedVariants() {
	i18nSvc := mgtmock.NewI18nServiceInterfaceMock(s.T())
	svc := newDCRService(s.mockAppService, s.mockOUService, i18nSvc, nil, &MockTransactioner{},
		testhelpers.OAuthConfig())

	s.mockAppService.On("GetOAuthApplication", mock.Anything, svcTestClientID).
		Return(s.existingOAuthClient(), (*tidcommon.ServiceError)(nil))
	s.mockAppService.On("GetApplication", mock.Anything, svcTestAppID).
		Return(s.existingApplication(), (*tidcommon.ServiceError)(nil))

	i18nSvc.On("GetTranslationsByNamespace", mock.Anything, application.AppI18nNamespace()).
		Return(map[string]map[string]string{
			application.AppI18nKey(svcTestAppID, "name"): {
				"en-US": "Existing Client",
				"fr":    "Client Actuel",
				"ja":    "既存のクライアント",
			},
		}, (*tidcommon.ServiceError)(nil))

	response, err := svc.GetClient(context.Background(), svcTestClientID)

	s.Require().Nil(err)
	s.Require().NotNil(response)
	s.Equal(map[string]string{"fr": "Client Actuel", "ja": "既存のクライアント"},
		response.LocalizedClientName)
	// The system-language entry is the base value, reported in client_name rather than as a variant,
	// and it replaces the i18n reference the record stores once the field is localized.
	s.Equal("Existing Client", response.ClientName)
	s.NotContains(response.ClientName, "{{t(")
}

// A registration that supplied only localized variants has no system-language entry, so the stored
// field is an i18n reference with no base text behind it. A read must still report a name: returning
// the reference would hand the caller a value that becomes a literal name if it is sent back.
func (s *ClientConfigurationServiceTestSuite) TestGetClient_NoBaseValueResolvesToAVariant() {
	i18nSvc := mgtmock.NewI18nServiceInterfaceMock(s.T())
	svc := newDCRService(s.mockAppService, s.mockOUService, i18nSvc, nil, &MockTransactioner{},
		testhelpers.OAuthConfig())

	s.mockAppService.On("GetOAuthApplication", mock.Anything, svcTestClientID).
		Return(s.existingOAuthClient(), (*tidcommon.ServiceError)(nil))
	s.mockAppService.On("GetApplication", mock.Anything, svcTestAppID).
		Return(s.existingApplication(), (*tidcommon.ServiceError)(nil))
	i18nSvc.On("GetTranslationsByNamespace", mock.Anything, application.AppI18nNamespace()).
		Return(map[string]map[string]string{
			application.AppI18nKey(svcTestAppID, "name"): {
				"fr": "Application Sans Base",
				"ja": "ベースなしアプリ",
			},
		}, (*tidcommon.ServiceError)(nil))

	response, err := svc.GetClient(context.Background(), svcTestClientID)

	s.Require().Nil(err)
	s.Require().NotNil(response)
	s.NotContains(response.ClientName, "{{t(")
	// Lowest language tag, so the resolved name does not change between reads.
	s.Equal("Application Sans Base", response.ClientName)
	s.Equal(map[string]string{"fr": "Application Sans Base", "ja": "ベースなしアプリ"},
		response.LocalizedClientName)
}

// The system-language entry holds the field's base value rather than a translation of it, so an
// update that changes only the variants of a field must not delete the base value along with them.
func (s *ClientConfigurationServiceTestSuite) TestUpdateClient_VariantOnlyUpdatePreservesBaseName() {
	i18nSvc := mgtmock.NewI18nServiceInterfaceMock(s.T())
	svc := newDCRService(s.mockAppService, s.mockOUService, i18nSvc, nil, &MockTransactioner{},
		testhelpers.OAuthConfig())

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

	nameKey := application.AppI18nKey(svcTestAppID, "name")
	i18nSvc.On("GetTranslationsByNamespace", mock.Anything, application.AppI18nNamespace()).
		Return(map[string]map[string]string{
			nameKey: {"en-US": "My App", "fr": "Mon App", "ja": "マイアプリ"},
		}, (*tidcommon.ServiceError)(nil))
	i18nSvc.On("DeleteTranslationsByKey", mock.Anything, application.AppI18nNamespace(), nameKey).
		Return((*tidcommon.ServiceError)(nil))

	var written map[string]map[string]string
	i18nSvc.On("SetTranslationOverridesForNamespace", mock.Anything, application.AppI18nNamespace(),
		mock.Anything).
		Run(func(args mock.Arguments) {
			written = args.Get(2).(map[string]map[string]string)
		}).
		Return((*tidcommon.ServiceError)(nil))

	// Only a French variant: the request says nothing about the base name.
	_, err := svc.UpdateClient(context.Background(), svcTestClientID, &DCRRegistrationRequest{
		RedirectURIs:        []string{"https://client.example.com/cb"},
		LocalizedClientName: map[string]string{"fr": "Mon App Renomme"},
	})

	s.Require().Nil(err)
	s.Require().NotNil(written)
	s.Equal("Mon App Renomme", written[nameKey]["fr"])
	// The base name survives the variant change, while the variant the request dropped does not.
	s.Equal("My App", written[nameKey][i18nmgt.SystemLanguage])
	s.NotContains(written[nameKey], "ja")
}

// A client with no stored variants reads back without localized fields, rather than with empty maps.
func (s *ClientConfigurationServiceTestSuite) TestGetClient_NoVariantsLeavesLocalizedFieldsUnset() {
	i18nSvc := mgtmock.NewI18nServiceInterfaceMock(s.T())
	svc := newDCRService(s.mockAppService, s.mockOUService, i18nSvc, nil, &MockTransactioner{},
		testhelpers.OAuthConfig())

	s.mockAppService.On("GetOAuthApplication", mock.Anything, svcTestClientID).
		Return(s.existingOAuthClient(), (*tidcommon.ServiceError)(nil))
	s.mockAppService.On("GetApplication", mock.Anything, svcTestAppID).
		Return(s.existingApplication(), (*tidcommon.ServiceError)(nil))
	i18nSvc.On("GetTranslationsByNamespace", mock.Anything, application.AppI18nNamespace()).
		Return(map[string]map[string]string{}, (*tidcommon.ServiceError)(nil))

	response, err := svc.GetClient(context.Background(), svcTestClientID)

	s.Require().Nil(err)
	s.Require().NotNil(response)
	s.Nil(response.LocalizedClientName)
	s.Equal("Existing Client", response.ClientName)
}

// RFC 7592 section 2.2 requires a client_secret carried in an update to match the issued secret. A
// mismatch must be rejected before anything is written, so the registration is left unchanged.
func (s *ClientConfigurationServiceTestSuite) TestUpdateClient_MismatchedClientSecretIsRejected() {
	authn := managermock.NewAuthnProviderManagerMock(s.T())
	svc := newDCRService(s.mockAppService, s.mockOUService, nil, authn, &MockTransactioner{},
		testhelpers.OAuthConfig())

	s.mockAppService.On("GetOAuthApplication", mock.Anything, svcTestClientID).
		Return(s.existingOAuthClient(), (*tidcommon.ServiceError)(nil))
	s.mockAppService.On("GetApplication", mock.Anything, svcTestAppID).
		Return(s.existingApplication(), (*tidcommon.ServiceError)(nil))
	authn.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, providers.AuthenticatedClaims{},
			&tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "AUTH-1001"})

	request := &DCRRegistrationRequest{
		ClientSecret: "wrong-secret",
		RedirectURIs: []string{"https://client.example.com/cb"},
	}
	response, err := svc.UpdateClient(context.Background(), svcTestClientID, request)

	s.Nil(response)
	s.Require().NotNil(err)
	s.Equal(ErrorClientSecretMismatch.Code, err.Code)
	// Rejected before the write, so the registration is untouched.
	s.mockAppService.AssertNotCalled(s.T(), "UpdateApplication",
		mock.Anything, mock.Anything, mock.Anything)
}

// A client_secret that matches the issued secret is accepted, and the update proceeds. The
// submitted value is only ever compared, never written, so the stored secret is preserved.
func (s *ClientConfigurationServiceTestSuite) TestUpdateClient_MatchingClientSecretIsAccepted() {
	authn := managermock.NewAuthnProviderManagerMock(s.T())
	svc := newDCRService(s.mockAppService, s.mockOUService, nil, authn, &MockTransactioner{},
		testhelpers.OAuthConfig())

	s.mockAppService.On("GetOAuthApplication", mock.Anything, svcTestClientID).
		Return(s.existingOAuthClient(), (*tidcommon.ServiceError)(nil))
	s.mockAppService.On("GetApplication", mock.Anything, svcTestAppID).
		Return(s.existingApplication(), (*tidcommon.ServiceError)(nil))
	authn.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, providers.AuthenticatedClaims{}, (*tidcommon.ServiceError)(nil))

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

	request := &DCRRegistrationRequest{
		ClientSecret: "correct-secret",
		RedirectURIs: []string{"https://client.example.com/cb"},
	}
	response, err := svc.UpdateClient(context.Background(), svcTestClientID, request)

	s.Require().Nil(err)
	s.Require().NotNil(response)
	// The submitted secret is never written: an empty secret on the replacement is what preserves
	// the stored one.
	s.Require().NotNil(capturedDTO)
	s.Require().NotEmpty(capturedDTO.InboundAuthConfig)
	s.Empty(capturedDTO.InboundAuthConfig[0].OAuthConfig.ClientSecret)
}

// The field is optional. An update that omits it must not consult the authentication provider at
// all, so a caller that simply does not echo the secret is unaffected.
func (s *ClientConfigurationServiceTestSuite) TestUpdateClient_OmittedClientSecretSkipsVerification() {
	authn := managermock.NewAuthnProviderManagerMock(s.T())
	svc := newDCRService(s.mockAppService, s.mockOUService, nil, authn, &MockTransactioner{},
		testhelpers.OAuthConfig())

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

	_, err := svc.UpdateClient(context.Background(), svcTestClientID,
		&DCRRegistrationRequest{RedirectURIs: []string{"https://client.example.com/cb"}})

	s.Require().Nil(err)
	authn.AssertNotCalled(s.T(), "AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything)
}

// An update replaces the registration in full, so localized variants the request omits must not
// survive it. The i18n store upserts, so without an explicit clear a translation the caller dropped
// would keep resolving with no way to remove it through this API.
func (s *ClientConfigurationServiceTestSuite) TestUpdateClient_OmittedLocalizedVariantIsCleared() {
	i18nSvc := mgtmock.NewI18nServiceInterfaceMock(s.T())
	svc := newDCRService(s.mockAppService, s.mockOUService, i18nSvc, nil, &MockTransactioner{},
		testhelpers.OAuthConfig())

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

	nameKey := application.AppI18nKey(svcTestAppID, "name")
	i18nSvc.On("GetTranslationsByNamespace", mock.Anything, application.AppI18nNamespace()).
		Return(map[string]map[string]string{}, (*tidcommon.ServiceError)(nil))
	i18nSvc.On("DeleteTranslationsByKey", mock.Anything, application.AppI18nNamespace(), nameKey).
		Return((*tidcommon.ServiceError)(nil))
	i18nSvc.On("SetTranslationOverridesForNamespace", mock.Anything, application.AppI18nNamespace(),
		mock.Anything).Return((*tidcommon.ServiceError)(nil))

	request := &DCRRegistrationRequest{
		RedirectURIs:        []string{"https://client.example.com/cb"},
		LocalizedClientName: map[string]string{"fr": "Mon Application"},
	}
	_, err := svc.UpdateClient(context.Background(), svcTestClientID, request)

	s.Require().Nil(err)
	// The name key is cleared before the new variants are written, so a previously stored variant
	// the request omits does not survive the update.
	i18nSvc.AssertCalled(s.T(), "DeleteTranslationsByKey", mock.Anything,
		application.AppI18nNamespace(), nameKey)
}

// Clearing is scoped to the fields the request actually supplies. An update that does not mention a
// localizable field at all must leave its translations alone, because clearing them would make an
// unrelated update silently drop every translation the client had.
func (s *ClientConfigurationServiceTestSuite) TestUpdateClient_UnmentionedLocalizedFieldIsNotCleared() {
	i18nSvc := mgtmock.NewI18nServiceInterfaceMock(s.T())
	svc := newDCRService(s.mockAppService, s.mockOUService, i18nSvc, nil, &MockTransactioner{},
		testhelpers.OAuthConfig())

	s.mockAppService.On("GetOAuthApplication", mock.Anything, svcTestClientID).
		Return(s.existingOAuthClient(), (*tidcommon.ServiceError)(nil))
	s.mockAppService.On("GetApplication", mock.Anything, svcTestAppID).
		Return(s.existingApplication(), (*tidcommon.ServiceError)(nil))
	s.mockAppService.On("UpdateApplication", mock.Anything, svcTestAppID,
		mock.AnythingOfType("*model.ApplicationDTO")).
		Return(&model.ApplicationDTO{
			ID:   svcTestAppID,
			Name: "Renamed Client",
			InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
				{
					Type:        providers.OAuthInboundAuthType,
					OAuthConfig: &providers.OAuthConfigWithSecret{ClientID: svcTestClientID},
				},
			},
		}, (*tidcommon.ServiceError)(nil))

	nameKey := application.AppI18nKey(svcTestAppID, "name")
	i18nSvc.On("GetTranslationsByNamespace", mock.Anything, application.AppI18nNamespace()).
		Return(map[string]map[string]string{}, (*tidcommon.ServiceError)(nil))
	i18nSvc.On("DeleteTranslationsByKey", mock.Anything, application.AppI18nNamespace(), nameKey).
		Return((*tidcommon.ServiceError)(nil))
	i18nSvc.On("SetTranslationOverridesForNamespace", mock.Anything, application.AppI18nNamespace(),
		mock.Anything).Return((*tidcommon.ServiceError)(nil))

	// The request names the client but says nothing about the logo, so only the name key is cleared.
	request := &DCRRegistrationRequest{
		ClientName:   "Renamed Client",
		RedirectURIs: []string{"https://client.example.com/cb"},
	}
	_, err := svc.UpdateClient(context.Background(), svcTestClientID, request)

	s.Require().Nil(err)
	i18nSvc.AssertNotCalled(s.T(), "DeleteTranslationsByKey", mock.Anything,
		application.AppI18nNamespace(), application.AppI18nKey(svcTestAppID, "logo_uri"))
}

// An invalid localized URI must be rejected before the application is written. The localized
// variants are persisted in a separate store after the application is replaced, so validating them
// late would leave the registration updated while the caller is told the request was invalid.
func (s *ClientConfigurationServiceTestSuite) TestUpdateClient_InvalidLocalizedURIRejectedBeforeWrite() {
	i18nSvc := mgtmock.NewI18nServiceInterfaceMock(s.T())
	svc := newDCRService(s.mockAppService, s.mockOUService, i18nSvc, nil, &MockTransactioner{},
		testhelpers.OAuthConfig())

	request := &DCRRegistrationRequest{
		ClientName:      "Renamed Client",
		RedirectURIs:    []string{"https://client.example.com/cb"},
		LocalizedTosURI: map[string]string{"fr": "not a valid uri"},
	}
	response, err := svc.UpdateClient(context.Background(), svcTestClientID, request)

	s.Nil(response)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidClientMetadata.Code, err.Code)
	// No lookup and no write: the registration is untouched. AssertNotCalled matches on the method
	// name and its arguments, so all three matchers are required for the check to mean anything.
	s.mockAppService.AssertNotCalled(s.T(), "UpdateApplication",
		mock.Anything, mock.Anything, mock.Anything)
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
