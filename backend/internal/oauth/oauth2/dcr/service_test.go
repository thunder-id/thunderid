// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package dcr

import (
	"context"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/application"
	"github.com/thunder-id/thunderid/internal/application/model"
	"github.com/thunder-id/thunderid/internal/cert"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	i18nmgt "github.com/thunder-id/thunderid/internal/system/i18n/mgt"
	"github.com/thunder-id/thunderid/tests/mocks/applicationmock"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	i18nmock "github.com/thunder-id/thunderid/tests/mocks/i18n/mgtmock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
)

const (
	svcTestClientID = "svc-client-id"
	svcTestAppID    = "svc-app-id"
)

// DCRServiceTestSuite is the test suite for DCR service
type DCRServiceTestSuite struct {
	suite.Suite
	mockAppService *applicationmock.ApplicationServiceInterfaceMock
	mockOUService  *oumock.OrganizationUnitServiceInterfaceMock
	service        DCRServiceInterface
}

func TestDCRServiceTestSuite(t *testing.T) {
	suite.Run(t, new(DCRServiceTestSuite))
}

// MockTransactioner is a simple implementation of Transactioner for testing.
type MockTransactioner struct{}

func (m *MockTransactioner) Transact(ctx context.Context, txFunc func(context.Context) error) error {
	return txFunc(ctx)
}

func (s *DCRServiceTestSuite) SetupTest() {
	s.mockAppService = applicationmock.NewApplicationServiceInterfaceMock(s.T())
	s.mockOUService = oumock.NewOrganizationUnitServiceInterfaceMock(s.T())
	s.service = newDCRService(s.mockAppService, s.mockOUService, nil, nil, &MockTransactioner{})
}

// TestBuildIDTokenConfig verifies that the response type is derived from the algorithm fields, so
// a client requesting only a signing algorithm still gets a stored config.
func (s *DCRServiceTestSuite) TestBuildIDTokenConfig() {
	testCases := []struct {
		name         string
		request      *DCRRegistrationRequest
		expectNil    bool
		responseType providers.IDTokenResponseType
		signingAlg   string
	}{
		{
			name:      "NoAlgFieldsYieldsNilConfig",
			request:   &DCRRegistrationRequest{},
			expectNil: true,
		},
		{
			name:         "SigningOnlyYieldsJWT",
			request:      &DCRRegistrationRequest{IDTokenSignedResponseAlg: "ES256"},
			responseType: providers.IDTokenResponseTypeJWT,
			signingAlg:   "ES256",
		},
		{
			name:         "EncryptionOnlyYieldsJWE",
			request:      &DCRRegistrationRequest{IDTokenEncryptedResponseAlg: "RSA-OAEP"},
			responseType: providers.IDTokenResponseTypeJWE,
		},
		{
			name:         "EncryptionEncAloneStillYieldsJWE",
			request:      &DCRRegistrationRequest{IDTokenEncryptedResponseEnc: "A256GCM"},
			responseType: providers.IDTokenResponseTypeJWE,
		},
		{
			name: "SigningAndEncryptionYieldsNestedJWT",
			request: &DCRRegistrationRequest{
				IDTokenSignedResponseAlg:    "ES256",
				IDTokenEncryptedResponseAlg: "RSA-OAEP",
			},
			responseType: providers.IDTokenResponseTypeNESTEDJWT,
			signingAlg:   "ES256",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			cfg := buildIDTokenConfig(tc.request)
			if tc.expectNil {
				s.Nil(cfg)
				return
			}
			s.NotNil(cfg)
			s.Equal(tc.responseType, cfg.ResponseType)
			s.Equal(tc.signingAlg, cfg.SigningAlg)
		})
	}
}

// TestNewDCRService tests the service constructor
func (s *DCRServiceTestSuite) TestNewDCRService() {
	service := newDCRService(s.mockAppService, s.mockOUService, nil, nil, &MockTransactioner{})
	s.NotNil(service)
	s.Implements((*DCRServiceInterface)(nil), service)
}

// TestRegisterClient_NilRequest tests nil request handling
func (s *DCRServiceTestSuite) TestRegisterClient_NilRequest() {
	response, err := s.service.RegisterClient(context.Background(), nil)

	s.Nil(response)
	s.NotNil(err)
	s.Equal(ErrorInvalidRequestFormat.Code, err.Code)
}

// TestRegisterClient_JWKSConflict tests JWKS and JWKS_URI conflict
func (s *DCRServiceTestSuite) TestRegisterClient_JWKSConflict() {
	request := &DCRRegistrationRequest{
		RedirectURIs: []string{"https://client.example.com/callback"},
		GrantTypes:   []providers.GrantType{providers.GrantTypeAuthorizationCode},
		JWKSUri:      "https://client.example.com/.well-known/jwks.json",
		JWKS:         map[string]interface{}{"keys": []interface{}{}},
	}

	response, err := s.service.RegisterClient(context.Background(), request)

	s.Nil(response)
	s.NotNil(err)
	s.Equal(ErrorJWKSConfigurationConflict.Code, err.Code)
}

// TestRegisterClient_ClientNameProvided tests registration with provided client name
func (s *DCRServiceTestSuite) TestRegisterClient_ClientNameProvided() {
	request := &DCRRegistrationRequest{
		OUID:         "test-ou-1",
		RedirectURIs: []string{"https://client.example.com/callback"},
		GrantTypes:   []providers.GrantType{providers.GrantTypeAuthorizationCode},
		ClientName:   "Test Client",
	}

	appDTO := &model.ApplicationDTO{
		ID:   "app-id",
		Name: "Test Client",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID:     "client-id",
					ClientSecret: "client-secret",
					Scopes:       []string{},
				},
			},
		},
	}

	s.mockAppService.On(
		"CreateApplication", mock.Anything, mock.AnythingOfType("*model.ApplicationDTO"),
	).Return(appDTO, (*tidcommon.ServiceError)(nil))

	response, err := s.service.RegisterClient(context.Background(), request)

	s.NotNil(response)
	s.Nil(err)
	s.Equal("client-id", response.ClientID)
	s.Equal("Test Client", response.ClientName)
}

// TestRegisterClient_JWKSUriProvided tests registration with JWKS_URI
func (s *DCRServiceTestSuite) TestRegisterClient_JWKSUriProvided() {
	request := &DCRRegistrationRequest{
		OUID:         "test-ou-1",
		RedirectURIs: []string{"https://client.example.com/callback"},
		GrantTypes:   []providers.GrantType{providers.GrantTypeAuthorizationCode},
		ClientName:   "Test Client",
		JWKSUri:      "https://client.example.com/.well-known/jwks.json",
	}

	appDTO := &model.ApplicationDTO{
		ID:   "app-id",
		Name: "Test Client",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID:     "client-id",
					ClientSecret: "client-secret",
					Scopes:       []string{},
					Certificate: &inboundmodel.Certificate{
						Type:  cert.CertificateTypeJWKSURI,
						Value: "https://client.example.com/.well-known/jwks.json",
					},
				},
			},
		},
	}

	s.mockAppService.On(
		"CreateApplication", mock.Anything, mock.AnythingOfType("*model.ApplicationDTO"),
	).Return(appDTO, (*tidcommon.ServiceError)(nil))

	response, err := s.service.RegisterClient(context.Background(), request)

	s.NotNil(response)
	s.Nil(err)
	s.Equal("https://client.example.com/.well-known/jwks.json", response.JWKSUri)
}

// TestRegisterClient_ApplicationServiceError tests application service error handling
func (s *DCRServiceTestSuite) TestRegisterClient_ApplicationServiceError() {
	request := &DCRRegistrationRequest{
		OUID:         "test-ou-1",
		RedirectURIs: []string{"not-a-valid-uri"},
		GrantTypes:   []providers.GrantType{providers.GrantTypeAuthorizationCode},
	}

	appServiceErr := &tidcommon.ServiceError{
		Type:             tidcommon.ClientErrorType,
		Code:             "APP-1012",
		Error:            tidcommon.I18nMessage{DefaultValue: "Invalid redirect URI"},
		ErrorDescription: tidcommon.I18nMessage{DefaultValue: "The redirect URI is invalid"},
	}

	s.mockAppService.On("CreateApplication", mock.Anything, mock.AnythingOfType("*model.ApplicationDTO")).
		Return(nil, appServiceErr)

	response, err := s.service.RegisterClient(context.Background(), request)

	s.Nil(response)
	s.NotNil(err)
	s.Equal(ErrorInvalidRedirectURI.Code, err.Code)
}

// TestMapApplicationErrorToDCRError tests error mapping
func (s *DCRServiceTestSuite) TestMapApplicationErrorToDCRError() {
	testCases := []struct {
		name            string
		appErrCode      string
		expectedDCRCode string
	}{
		{
			name:            "Invalid Logo URL Error APP-1006",
			appErrCode:      "APP-1006",
			expectedDCRCode: ErrorInvalidClientMetadata.Code,
		},
		{
			name:            "Redirect URI Error APP-1012",
			appErrCode:      "APP-1012",
			expectedDCRCode: ErrorInvalidRedirectURI.Code,
		},
		{
			name:            "Certificate Type Error APP-1014",
			appErrCode:      "APP-1014",
			expectedDCRCode: ErrorInvalidClientMetadata.Code,
		},
		{
			name:            "Certificate Value Error APP-1015",
			appErrCode:      "APP-1015",
			expectedDCRCode: ErrorInvalidClientMetadata.Code,
		},
		{
			name:            "Internal Server Error SSE-5000",
			appErrCode:      tidcommon.InternalServerError.Code,
			expectedDCRCode: ErrorServerError.Code,
		},
		{
			name:            "Encoding Error SSE-5001",
			appErrCode:      tidcommon.ErrorEncodingError.Code,
			expectedDCRCode: ErrorServerError.Code,
		},
		{
			name:            "Default Client Error",
			appErrCode:      "APP-9999",
			expectedDCRCode: ErrorInvalidClientMetadata.Code,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			appErr := &tidcommon.ServiceError{
				Code: tc.appErrCode,
			}

			service := s.service.(*dcrService)
			dcrErr := service.mapApplicationErrorToDCRError(appErr)

			s.Equal(tc.expectedDCRCode, dcrErr.Code)
		})
	}
}

func (s *DCRServiceTestSuite) TestRegisterClient_ConvertDCRToApplicationError() {
	// A channel value cannot be JSON-marshaled, so JWKS serialization fails and
	// the request is rejected before reaching the application service.
	request := &DCRRegistrationRequest{
		OUID:         "test-ou-1",
		RedirectURIs: []string{"https://client.example.com/callback"},
		GrantTypes:   []providers.GrantType{providers.GrantTypeAuthorizationCode},
		JWKS:         map[string]interface{}{"keys": make(chan int)},
	}

	response, err := s.service.RegisterClient(context.Background(), request)

	s.Nil(response)
	s.NotNil(err)
	s.Equal(ErrorServerError.Code, err.Code)
}

func (s *DCRServiceTestSuite) TestRegisterClient_ConvertApplicationToDCRResponseError() {
	request := &DCRRegistrationRequest{
		OUID:         "test-ou-1",
		RedirectURIs: []string{"https://client.example.com/callback"},
		GrantTypes:   []providers.GrantType{providers.GrantTypeAuthorizationCode},
		ClientName:   "Test Client",
	}

	appDTO := &model.ApplicationDTO{
		ID:   "app-id",
		Name: "Test Client",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID:     "client-id",
					ClientSecret: "client-secret",
					Scopes:       []string{},
					Certificate: &inboundmodel.Certificate{
						Type:  cert.CertificateTypeJWKS,
						Value: "invalid json",
					},
				},
			},
		},
	}

	s.mockAppService.On(
		"CreateApplication", mock.Anything, mock.AnythingOfType("*model.ApplicationDTO"),
	).Return(appDTO, (*tidcommon.ServiceError)(nil))

	response, err := s.service.RegisterClient(context.Background(), request)

	s.Nil(response)
	s.NotNil(err)
	s.Equal(ErrorServerError.Code, err.Code)
}

func (s *DCRServiceTestSuite) TestRegisterClient_WithJWKS() {
	request := &DCRRegistrationRequest{
		OUID:         "test-ou-1",
		RedirectURIs: []string{"https://client.example.com/callback"},
		GrantTypes:   []providers.GrantType{providers.GrantTypeAuthorizationCode},
		ClientName:   "Test Client",
		JWKS:         map[string]interface{}{"keys": []interface{}{}},
	}

	appDTO := &model.ApplicationDTO{
		ID:   "app-id",
		Name: "Test Client",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID:     "client-id",
					ClientSecret: "client-secret",
					Scopes:       []string{},
					Certificate: &inboundmodel.Certificate{
						Type:  cert.CertificateTypeJWKS,
						Value: `{"keys":[]}`,
					},
				},
			},
		},
	}

	s.mockAppService.On(
		"CreateApplication", mock.Anything, mock.AnythingOfType("*model.ApplicationDTO"),
	).Return(appDTO, (*tidcommon.ServiceError)(nil))

	response, err := s.service.RegisterClient(context.Background(), request)

	s.NotNil(response)
	s.Nil(err)
	s.NotNil(response.JWKS)
}

func (s *DCRServiceTestSuite) TestRegisterClient_WithScope() {
	request := &DCRRegistrationRequest{
		OUID:         "test-ou-1",
		RedirectURIs: []string{"https://client.example.com/callback"},
		GrantTypes:   []providers.GrantType{providers.GrantTypeAuthorizationCode},
		ClientName:   "Test Client",
		Scope:        "read write admin",
	}

	appDTO := &model.ApplicationDTO{
		ID:   "app-id",
		Name: "Test Client",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID:     "client-id",
					ClientSecret: "client-secret",
					Scopes:       []string{"read", "write", "admin"},
				},
			},
		},
	}

	s.mockAppService.On(
		"CreateApplication", mock.Anything, mock.AnythingOfType("*model.ApplicationDTO"),
	).Return(appDTO, (*tidcommon.ServiceError)(nil))

	response, err := s.service.RegisterClient(context.Background(), request)

	s.NotNil(response)
	s.Nil(err)
	s.Equal("read write admin", response.Scope)
}

func (s *DCRServiceTestSuite) TestRegisterClient_RequirePushedAuthorizationRequests() {
	request := &DCRRegistrationRequest{
		OUID:                               "test-ou-1",
		RedirectURIs:                       []string{"https://client.example.com/callback"},
		GrantTypes:                         []providers.GrantType{providers.GrantTypeAuthorizationCode},
		ClientName:                         "Test Client",
		RequirePushedAuthorizationRequests: true,
	}

	appDTO := &model.ApplicationDTO{
		ID:   "app-id",
		Name: "Test Client",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID:                           "client-id",
					ClientSecret:                       "client-secret",
					Scopes:                             []string{},
					RequirePushedAuthorizationRequests: true,
				},
			},
		},
	}

	s.mockAppService.On(
		"CreateApplication", mock.Anything,
		mock.MatchedBy(func(dto *model.ApplicationDTO) bool {
			if len(dto.InboundAuthConfig) == 0 || dto.InboundAuthConfig[0].OAuthConfig == nil {
				return false
			}
			return dto.InboundAuthConfig[0].OAuthConfig.RequirePushedAuthorizationRequests
		}),
	).Return(appDTO, (*tidcommon.ServiceError)(nil))

	response, err := s.service.RegisterClient(context.Background(), request)

	s.NotNil(response)
	s.Nil(err)
	s.True(response.RequirePushedAuthorizationRequests)
}

// TestRegisterClient_EmptyInboundAuthConfig verifies that a created application returned by
// the application service without any OAuth inbound config is treated as a server-side
// invariant violation: the DCR endpoint must NOT silently respond 200 with an empty body.
func (s *DCRServiceTestSuite) TestRegisterClient_EmptyInboundAuthConfig() {
	request := &DCRRegistrationRequest{
		OUID:         "test-ou-1",
		RedirectURIs: []string{"https://client.example.com/callback"},
		GrantTypes:   []providers.GrantType{providers.GrantTypeAuthorizationCode},
		ClientName:   "Test Client",
	}

	appDTO := &model.ApplicationDTO{
		ID:                "app-id",
		Name:              "Test Client",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{},
	}

	s.mockAppService.On(
		"CreateApplication", mock.Anything, mock.AnythingOfType("*model.ApplicationDTO"),
	).Return(appDTO, (*tidcommon.ServiceError)(nil))

	response, err := s.service.RegisterClient(context.Background(), request)

	s.Nil(response)
	s.NotNil(err)
	s.Equal(ErrorServerError.Code, err.Code)
}

// TestRegisterClient_WithLocalizedVariants tests that localized fields are persisted and echoed,
// and that the non-tagged default is stored under SystemLanguage.
func (s *DCRServiceTestSuite) TestRegisterClient_WithLocalizedVariants() {
	mockI18n := i18nmock.NewI18nServiceInterfaceMock(s.T())
	svc := newDCRService(s.mockAppService, s.mockOUService, mockI18n, nil, &MockTransactioner{})

	request := &DCRRegistrationRequest{
		OUID:                "test-ou-1",
		ClientName:          "Test Client",
		LocalizedClientName: map[string]string{"fr": "Client FR", "de": "Client DE"},
		LocalizedLogoURI:    map[string]string{"fr": "https://example.fr/logo.png"},
	}

	appDTO := &model.ApplicationDTO{
		ID:   "app-id",
		Name: "Test Client",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID:     "client-id",
					ClientSecret: "client-secret",
					Scopes:       []string{},
				},
			},
		},
	}

	s.mockAppService.On(
		"CreateApplication", mock.Anything, mock.AnythingOfType("*model.ApplicationDTO"),
	).Return(appDTO, (*tidcommon.ServiceError)(nil))

	mockI18n.On(
		"SetTranslationOverridesForNamespace",
		mock.Anything,
		application.AppI18nNamespace(),
		mock.MatchedBy(func(entries map[string]map[string]string) bool {
			nameKey := application.AppI18nKey("app-id", "name")
			logoKey := application.AppI18nKey("app-id", "logo_uri")
			return entries[nameKey][i18nmgt.SystemLanguage] == "Test Client" &&
				entries[nameKey]["fr"] == "Client FR" &&
				entries[nameKey]["de"] == "Client DE" &&
				entries[logoKey]["fr"] == "https://example.fr/logo.png" &&
				entries[logoKey][i18nmgt.SystemLanguage] == ""
		}),
	).Return((*tidcommon.ServiceError)(nil))

	response, err := svc.RegisterClient(context.Background(), request)

	s.NotNil(response)
	s.Nil(err)
	s.Equal(map[string]string{"fr": "Client FR", "de": "Client DE"}, response.LocalizedClientName)
	s.Equal(map[string]string{"fr": "https://example.fr/logo.png"}, response.LocalizedLogoURI)
}

// TestRegisterClient_DefaultOnlyStoresSystemLanguage verifies that when only the non-tagged
// client_name is provided (no localized variants), it is stored under SystemLanguage.
func (s *DCRServiceTestSuite) TestRegisterClient_DefaultOnlyStoresSystemLanguage() {
	mockI18n := i18nmock.NewI18nServiceInterfaceMock(s.T())
	svc := newDCRService(s.mockAppService, s.mockOUService, mockI18n, nil, &MockTransactioner{})

	request := &DCRRegistrationRequest{
		OUID:       "test-ou-1",
		ClientName: "My App",
	}

	appDTO := &model.ApplicationDTO{
		ID:   "app-id",
		Name: "My App",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID: "client-id",
					Scopes:   []string{},
				},
			},
		},
	}

	s.mockAppService.On(
		"CreateApplication", mock.Anything, mock.AnythingOfType("*model.ApplicationDTO"),
	).Return(appDTO, (*tidcommon.ServiceError)(nil))

	mockI18n.On(
		"SetTranslationOverridesForNamespace",
		mock.Anything,
		application.AppI18nNamespace(),
		mock.MatchedBy(func(entries map[string]map[string]string) bool {
			nameKey := application.AppI18nKey("app-id", "name")
			return entries[nameKey][i18nmgt.SystemLanguage] == "My App"
		}),
	).Return((*tidcommon.ServiceError)(nil))

	response, err := svc.RegisterClient(context.Background(), request)

	s.NotNil(response)
	s.Nil(err)
}

// TestRegisterClient_TaggedSystemLanguageWinsOverDefault verifies that when both the non-tagged
// default and an explicit #SystemLanguage-tagged variant are provided, the tagged variant wins.
func (s *DCRServiceTestSuite) TestRegisterClient_TaggedSystemLanguageWinsOverDefault() {
	mockI18n := i18nmock.NewI18nServiceInterfaceMock(s.T())
	svc := newDCRService(s.mockAppService, s.mockOUService, mockI18n, nil, &MockTransactioner{})

	request := &DCRRegistrationRequest{
		OUID:                "test-ou-1",
		ClientName:          "My App",
		LocalizedClientName: map[string]string{i18nmgt.SystemLanguage: "My App US"},
	}

	appDTO := &model.ApplicationDTO{
		ID:   "app-id",
		Name: "My App",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID: "client-id",
					Scopes:   []string{},
				},
			},
		},
	}

	s.mockAppService.On(
		"CreateApplication", mock.Anything, mock.AnythingOfType("*model.ApplicationDTO"),
	).Return(appDTO, (*tidcommon.ServiceError)(nil))

	mockI18n.On(
		"SetTranslationOverridesForNamespace",
		mock.Anything,
		application.AppI18nNamespace(),
		mock.MatchedBy(func(entries map[string]map[string]string) bool {
			nameKey := application.AppI18nKey("app-id", "name")
			// Tagged variant wins — must be "My App US", not "My App".
			return entries[nameKey][i18nmgt.SystemLanguage] == "My App US"
		}),
	).Return((*tidcommon.ServiceError)(nil))

	response, err := svc.RegisterClient(context.Background(), request)

	s.NotNil(response)
	s.Nil(err)
	s.Equal(map[string]string{i18nmgt.SystemLanguage: "My App US"}, response.LocalizedClientName)
}

// TestRegisterClient_LocalizedVariantsWriteFailure tests that a failed i18n write triggers
// partial-row cleanup and app compensation delete.
func (s *DCRServiceTestSuite) TestRegisterClient_LocalizedVariantsWriteFailure() {
	mockI18n := i18nmock.NewI18nServiceInterfaceMock(s.T())
	svc := newDCRService(s.mockAppService, s.mockOUService, mockI18n, nil, &MockTransactioner{})

	request := &DCRRegistrationRequest{
		OUID:                "test-ou-1",
		ClientName:          "Test Client",
		LocalizedClientName: map[string]string{"fr": "Client FR"},
	}

	appDTO := &model.ApplicationDTO{
		ID:   "app-id",
		Name: "Test Client",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID: "client-id",
					Scopes:   []string{},
				},
			},
		},
	}

	i18nErr := &tidcommon.ServiceError{Code: "I18N-500"}

	s.mockAppService.On(
		"CreateApplication", mock.Anything, mock.AnythingOfType("*model.ApplicationDTO"),
	).Return(appDTO, (*tidcommon.ServiceError)(nil))
	mockI18n.On(
		"SetTranslationOverridesForNamespace",
		mock.Anything,
		application.AppI18nNamespace(),
		mock.Anything,
	).Return(i18nErr)
	mockI18n.On("DeleteTranslationsByKey", mock.Anything, application.AppI18nNamespace(), mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	s.mockAppService.On("DeleteApplication", mock.Anything, "app-id").
		Return((*tidcommon.ServiceError)(nil))

	response, err := svc.RegisterClient(context.Background(), request)

	s.Nil(response)
	s.NotNil(err)
	s.Equal(ErrorServerError.Code, err.Code)
	mockI18n.AssertExpectations(s.T())
	s.mockAppService.AssertExpectations(s.T())
}

// TestRegisterClient_InvalidLocalizedURI tests AC-13: a localized URI variant that fails URI
// validation must return ErrorInvalidClientMetadata and trigger the compensation rollback.
func (s *DCRServiceTestSuite) TestRegisterClient_InvalidLocalizedURI() {
	mockI18n := i18nmock.NewI18nServiceInterfaceMock(s.T())
	svc := newDCRService(s.mockAppService, s.mockOUService, mockI18n, nil, &MockTransactioner{})

	request := &DCRRegistrationRequest{
		OUID:             "test-ou-1",
		ClientName:       "Test Client",
		LocalizedLogoURI: map[string]string{"fr": "not-a-valid-uri"},
	}

	appDTO := &model.ApplicationDTO{
		ID:   "app-id",
		Name: "Test Client",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID: "client-id",
					Scopes:   []string{},
				},
			},
		},
	}

	s.mockAppService.On(
		"CreateApplication", mock.Anything, mock.AnythingOfType("*model.ApplicationDTO"),
	).Return(appDTO, (*tidcommon.ServiceError)(nil))
	// URI validation fails before any i18n writes; compensation still runs.
	mockI18n.On("DeleteTranslationsByKey", mock.Anything, application.AppI18nNamespace(), mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	s.mockAppService.On("DeleteApplication", mock.Anything, "app-id").
		Return((*tidcommon.ServiceError)(nil))

	response, err := svc.RegisterClient(context.Background(), request)

	s.Nil(response)
	s.NotNil(err)
	s.Equal(ErrorInvalidClientMetadata.Code, err.Code)
	mockI18n.AssertExpectations(s.T())
	s.mockAppService.AssertExpectations(s.T())
}

// TestBuildIDTokenConfig_NilWhenBothEmpty verifies that buildIDTokenConfig returns nil when both
// IDTokenEncryptedResponseAlg and IDTokenEncryptedResponseEnc are empty.
func (s *DCRServiceTestSuite) TestBuildIDTokenConfig_NilWhenBothEmpty() {
	req := &DCRRegistrationRequest{
		IDTokenEncryptedResponseAlg: "",
		IDTokenEncryptedResponseEnc: "",
	}
	s.Nil(buildIDTokenConfig(req))
}

// TestBuildIDTokenConfig_MapsAlgAndEnc verifies that buildIDTokenConfig maps the alg/enc fields.
func (s *DCRServiceTestSuite) TestBuildIDTokenConfig_MapsAlgAndEnc() {
	req := &DCRRegistrationRequest{
		IDTokenEncryptedResponseAlg: "RSA-OAEP-256",
		IDTokenEncryptedResponseEnc: "A256GCM",
	}
	cfg := buildIDTokenConfig(req)
	s.Require().NotNil(cfg)
	s.Equal("RSA-OAEP-256", cfg.EncryptionAlg)
	s.Equal("A256GCM", cfg.EncryptionEnc)
}

// TestRegisterClient_WithIDTokenEncryption verifies that DCR registration round-trips
// IDTokenEncryptedResponseAlg and IDTokenEncryptedResponseEnc correctly.
func (s *DCRServiceTestSuite) TestRegisterClient_WithIDTokenEncryption() {
	request := &DCRRegistrationRequest{
		OUID:                        "test-ou-1",
		ClientName:                  "IDToken Encryption Client",
		IDTokenEncryptedResponseAlg: "RSA-OAEP-256",
		IDTokenEncryptedResponseEnc: "A256GCM",
	}

	appDTO := &model.ApplicationDTO{
		ID:   "app-id",
		Name: "IDToken Encryption Client",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID: "client-id",
					Scopes:   []string{"openid"},
					Token: &providers.OAuthTokenConfig{
						IDToken: &providers.IDTokenConfig{
							EncryptionAlg: "RSA-OAEP-256",
							EncryptionEnc: "A256GCM",
						},
					},
				},
			},
		},
	}

	s.mockAppService.On("CreateApplication", mock.Anything,
		mock.MatchedBy(func(dto *model.ApplicationDTO) bool {
			for _, inbound := range dto.InboundAuthConfig {
				cfg := inbound.OAuthConfig
				if cfg != nil &&
					cfg.Token != nil &&
					cfg.Token.IDToken != nil &&
					cfg.Token.IDToken.EncryptionAlg == "RSA-OAEP-256" &&
					cfg.Token.IDToken.EncryptionEnc == "A256GCM" {
					return true
				}
			}
			return false
		}),
	).Return(appDTO, (*tidcommon.ServiceError)(nil))

	response, err := s.service.RegisterClient(context.Background(), request)

	s.Nil(err)
	s.Require().NotNil(response)
	s.Equal("RSA-OAEP-256", response.IDTokenEncryptedResponseAlg)
	s.Equal("A256GCM", response.IDTokenEncryptedResponseEnc)
	s.mockAppService.AssertExpectations(s.T())
}

// TestRegisterClient_LocalizedVariantsWriteFailure_ClientError tests that a ClientErrorType
// i18n error maps to ErrorServerError to avoid leaking internal details to external callers.
func (s *DCRServiceTestSuite) TestRegisterClient_LocalizedVariantsWriteFailure_ClientError() {
	mockI18n := i18nmock.NewI18nServiceInterfaceMock(s.T())
	svc := newDCRService(s.mockAppService, s.mockOUService, mockI18n, nil, &MockTransactioner{})

	request := &DCRRegistrationRequest{
		OUID:                "test-ou-1",
		ClientName:          "Test Client",
		LocalizedClientName: map[string]string{"fr": "Client FR"},
	}

	appDTO := &model.ApplicationDTO{
		ID:   "app-id",
		Name: "Test Client",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID: "client-id",
					Scopes:   []string{},
				},
			},
		},
	}

	i18nClientErr := &tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "I18N-4001",
	}

	s.mockAppService.On(
		"CreateApplication", mock.Anything, mock.AnythingOfType("*model.ApplicationDTO"),
	).Return(appDTO, (*tidcommon.ServiceError)(nil))
	mockI18n.On(
		"SetTranslationOverridesForNamespace",
		mock.Anything,
		application.AppI18nNamespace(),
		mock.Anything,
	).Return(i18nClientErr)
	mockI18n.On("DeleteTranslationsByKey", mock.Anything, application.AppI18nNamespace(), mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	s.mockAppService.On("DeleteApplication", mock.Anything, "app-id").
		Return((*tidcommon.ServiceError)(nil))

	response, err := svc.RegisterClient(context.Background(), request)

	s.Nil(response)
	s.NotNil(err)
	s.Equal(ErrorServerError.Code, err.Code)
	mockI18n.AssertExpectations(s.T())
	s.mockAppService.AssertExpectations(s.T())
}

func (s *DCRServiceTestSuite) existingOAuthClient() *providers.OAuthClient {
	return &providers.OAuthClient{
		ID:           svcTestAppID,
		ClientID:     svcTestClientID,
		OUID:         "ou-1",
		RedirectURIs: []string{"https://client.example.com/cb"},
		GrantTypes:   []providers.GrantType{providers.GrantTypeAuthorizationCode},
		Scopes:       []string{"openid"},
	}
}

func (s *DCRServiceTestSuite) existingApplication() *providers.Application {
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
func (s *DCRServiceTestSuite) registeredApplicationDTO() *model.ApplicationDTO {
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
func (s *DCRServiceTestSuite) expectUpdate() {
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
func (s *DCRServiceTestSuite) TestGetClient_ReturnsRegistration() {
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
func (s *DCRServiceTestSuite) TestGetClient_UnknownClient() {
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
func (s *DCRServiceTestSuite) TestUpdateClient_PreservesClientIdentity() {
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

	request := &DCRUpdateRequest{
		DCRRegistrationRequest: DCRRegistrationRequest{
			ClientName:   "Renamed Client",
			RedirectURIs: []string{"https://client.example.com/updated"},
		},
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
func (s *DCRServiceTestSuite) TestUpdateClient_InheritsNameWhenOmitted() {
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

	request := &DCRUpdateRequest{
		DCRRegistrationRequest: DCRRegistrationRequest{
			RedirectURIs: []string{"https://client.example.com/cb"},
		},
	}
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
func (s *DCRServiceTestSuite) TestUpdateClient_LocalizedNameRefUsesExistingAppID() {
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

	request := &DCRUpdateRequest{
		DCRRegistrationRequest: DCRRegistrationRequest{
			RedirectURIs:        []string{"https://client.example.com/cb"},
			LocalizedClientName: map[string]string{"fr": "Mon Application"},
		},
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
func (s *DCRServiceTestSuite) TestGetClient_ReturnsStoredLocalizedVariants() {
	i18nSvc := i18nmock.NewI18nServiceInterfaceMock(s.T())
	svc := newDCRService(s.mockAppService, s.mockOUService, i18nSvc, nil, &MockTransactioner{})

	s.mockAppService.On("GetOAuthApplication", mock.Anything, svcTestClientID).
		Return(s.existingOAuthClient(), (*tidcommon.ServiceError)(nil))
	s.mockAppService.On("GetApplication", mock.Anything, svcTestAppID).
		Return(s.existingApplication(), (*tidcommon.ServiceError)(nil))

	i18nSvc.On("GetTranslationsByKeys", mock.Anything, application.AppI18nNamespace(),
		mock.Anything).
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
func (s *DCRServiceTestSuite) TestGetClient_NoBaseValueResolvesToAVariant() {
	i18nSvc := i18nmock.NewI18nServiceInterfaceMock(s.T())
	svc := newDCRService(s.mockAppService, s.mockOUService, i18nSvc, nil, &MockTransactioner{})

	s.mockAppService.On("GetOAuthApplication", mock.Anything, svcTestClientID).
		Return(s.existingOAuthClient(), (*tidcommon.ServiceError)(nil))
	// The record delegates its name to i18n, which is the state a registration that supplied only
	// localized names leaves behind.
	localizedApp := s.existingApplication()
	localizedApp.Name = application.AppI18nRef(svcTestAppID, "name")
	s.mockAppService.On("GetApplication", mock.Anything, svcTestAppID).
		Return(localizedApp, (*tidcommon.ServiceError)(nil))
	i18nSvc.On("GetTranslationsByKeys", mock.Anything, application.AppI18nNamespace(),
		mock.Anything).
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

// The update response must report the name that was applied, not the i18n reference the record
// stores once the name is localized. A caller cannot meaningfully send a reference back.
func (s *DCRServiceTestSuite) TestUpdateClient_LocalizedUpdateResponseReportsAName() {
	cases := []struct {
		name     string
		request  *DCRRegistrationRequest
		expected string
	}{
		{
			// An explicit system-language variant is what gets written, so it is what is reported.
			name: "system language variant wins over client_name",
			request: &DCRRegistrationRequest{
				ClientName:          "Base Name",
				RedirectURIs:        []string{"https://client.example.com/cb"},
				LocalizedClientName: map[string]string{"en-US": "Explicit English", "fr": "Nom"},
			},
			expected: "Explicit English",
		},
		{
			name: "client_name is used when no system language variant is supplied",
			request: &DCRRegistrationRequest{
				ClientName:          "Base Name",
				RedirectURIs:        []string{"https://client.example.com/cb"},
				LocalizedClientName: map[string]string{"fr": "Nom"},
			},
			expected: "Base Name",
		},
		{
			// Nothing in the request names the client, so the value carried over from the cleared
			// variants is what the registration ends up with.
			name: "preserved base is used when the request supplies only variants",
			request: &DCRRegistrationRequest{
				RedirectURIs:        []string{"https://client.example.com/cb"},
				LocalizedClientName: map[string]string{"fr": "Nom"},
			},
			expected: "Preserved Base",
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			appSvc := applicationmock.NewApplicationServiceInterfaceMock(s.T())
			i18nSvc := i18nmock.NewI18nServiceInterfaceMock(s.T())
			svc := newDCRService(appSvc, s.mockOUService, i18nSvc, nil, &MockTransactioner{})

			appSvc.On("GetOAuthApplication", mock.Anything, svcTestClientID).
				Return(s.existingOAuthClient(), (*tidcommon.ServiceError)(nil))
			appSvc.On("GetApplication", mock.Anything, svcTestAppID).
				Return(s.existingApplication(), (*tidcommon.ServiceError)(nil))
			appSvc.On("UpdateApplication", mock.Anything, svcTestAppID,
				mock.AnythingOfType("*model.ApplicationDTO")).
				Return(&model.ApplicationDTO{
					ID:   svcTestAppID,
					Name: application.AppI18nRef(svcTestAppID, "name"),
					InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
						{
							Type:        providers.OAuthInboundAuthType,
							OAuthConfig: &providers.OAuthConfigWithSecret{ClientID: svcTestClientID},
						},
					},
				}, (*tidcommon.ServiceError)(nil))

			i18nSvc.On("GetTranslationsByKeys", mock.Anything, application.AppI18nNamespace(),
				mock.Anything).
				Return(map[string]map[string]string{
					application.AppI18nKey(svcTestAppID, "name"): {"en-US": "Preserved Base"},
				}, (*tidcommon.ServiceError)(nil))
			i18nSvc.On("DeleteTranslationsByKey", mock.Anything, application.AppI18nNamespace(),
				mock.Anything).
				Return((*tidcommon.ServiceError)(nil))
			i18nSvc.On("SetTranslationOverridesForNamespace", mock.Anything,
				application.AppI18nNamespace(), mock.Anything).
				Return((*tidcommon.ServiceError)(nil))

			response, err := svc.UpdateClient(context.Background(), svcTestClientID,
				&DCRUpdateRequest{DCRRegistrationRequest: *tc.request})

			s.Require().Nil(err)
			s.Require().NotNil(response)
			s.NotContains(response.ClientName, "{{t(")
			s.Equal(tc.expected, response.ClientName)
		})
	}
}

// A field the record holds as a literal value is reported as it stands. Translations left over from
// an earlier state of that field must not override the value the registration currently has, which
// is what an update setting a plain logo_uri leaves behind.
func (s *DCRServiceTestSuite) TestGetClient_LiteralFieldIsNotOverriddenByTranslations() {
	i18nSvc := i18nmock.NewI18nServiceInterfaceMock(s.T())
	svc := newDCRService(s.mockAppService, s.mockOUService, i18nSvc, nil, &MockTransactioner{})

	s.mockAppService.On("GetOAuthApplication", mock.Anything, svcTestClientID).
		Return(s.existingOAuthClient(), (*tidcommon.ServiceError)(nil))
	// LogoURL is a plain URL on the record, not an i18n reference.
	s.mockAppService.On("GetApplication", mock.Anything, svcTestAppID).
		Return(s.existingApplication(), (*tidcommon.ServiceError)(nil))
	i18nSvc.On("GetTranslationsByKeys", mock.Anything, application.AppI18nNamespace(),
		mock.Anything).
		Return(map[string]map[string]string{
			application.AppI18nKey(svcTestAppID, "logo_uri"): {
				"en-US": "https://stale.example.com/logo.png",
				"fr":    "https://stale.example.com/logo-fr.png",
			},
		}, (*tidcommon.ServiceError)(nil))

	response, err := svc.GetClient(context.Background(), svcTestClientID)

	s.Require().Nil(err)
	s.Require().NotNil(response)
	s.Equal("https://client.example.com/logo.png", response.LogoURI)
}

// The system-language entry holds the field's base value rather than a translation of it, so an
// update that changes only the variants of a field must not delete the base value along with them.
func (s *DCRServiceTestSuite) TestUpdateClient_VariantOnlyUpdatePreservesBaseName() {
	i18nSvc := i18nmock.NewI18nServiceInterfaceMock(s.T())
	svc := newDCRService(s.mockAppService, s.mockOUService, i18nSvc, nil, &MockTransactioner{})

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
	i18nSvc.On("GetTranslationsByKeys", mock.Anything, application.AppI18nNamespace(),
		mock.Anything).
		Return(map[string]map[string]string{
			nameKey: {"en-US": "My App", "fr": "Mon App", "ja": "マイアプリ"},
		}, (*tidcommon.ServiceError)(nil))
	i18nSvc.On("DeleteTranslationsByKey", mock.Anything, application.AppI18nNamespace(),
		mock.Anything).
		Return((*tidcommon.ServiceError)(nil))

	var written map[string]map[string]string
	i18nSvc.On("SetTranslationOverridesForNamespace", mock.Anything, application.AppI18nNamespace(),
		mock.Anything).
		Run(func(args mock.Arguments) {
			written = args.Get(2).(map[string]map[string]string)
		}).
		Return((*tidcommon.ServiceError)(nil))

	// Only a French variant: the request says nothing about the base name.
	_, err := svc.UpdateClient(context.Background(), svcTestClientID, &DCRUpdateRequest{
		DCRRegistrationRequest: DCRRegistrationRequest{
			RedirectURIs:        []string{"https://client.example.com/cb"},
			LocalizedClientName: map[string]string{"fr": "Mon App Renomme"},
		},
	})

	s.Require().Nil(err)
	s.Require().NotNil(written)
	s.Equal("Mon App Renomme", written[nameKey]["fr"])
	// The base name survives the variant change, while the variant the request dropped does not.
	s.Equal("My App", written[nameKey][i18nmgt.SystemLanguage])
	s.NotContains(written[nameKey], "ja")
}

// A client with no stored variants reads back without localized fields, rather than with empty maps.
func (s *DCRServiceTestSuite) TestGetClient_NoVariantsLeavesLocalizedFieldsUnset() {
	i18nSvc := i18nmock.NewI18nServiceInterfaceMock(s.T())
	svc := newDCRService(s.mockAppService, s.mockOUService, i18nSvc, nil, &MockTransactioner{})

	s.mockAppService.On("GetOAuthApplication", mock.Anything, svcTestClientID).
		Return(s.existingOAuthClient(), (*tidcommon.ServiceError)(nil))
	s.mockAppService.On("GetApplication", mock.Anything, svcTestAppID).
		Return(s.existingApplication(), (*tidcommon.ServiceError)(nil))
	i18nSvc.On("GetTranslationsByKeys", mock.Anything, application.AppI18nNamespace(),
		mock.Anything).
		Return(map[string]map[string]string{}, (*tidcommon.ServiceError)(nil))

	response, err := svc.GetClient(context.Background(), svcTestClientID)

	s.Require().Nil(err)
	s.Require().NotNil(response)
	s.Nil(response.LocalizedClientName)
	s.Equal("Existing Client", response.ClientName)
}

// RFC 7592 section 2.2 requires a client_secret carried in an update to match the issued secret. A
// mismatch must be rejected before anything is written, so the registration is left unchanged.
func (s *DCRServiceTestSuite) TestUpdateClient_MismatchedClientSecretIsRejected() {
	authn := managermock.NewAuthnProviderManagerMock(s.T())
	svc := newDCRService(s.mockAppService, s.mockOUService, nil, authn, &MockTransactioner{})

	s.mockAppService.On("GetOAuthApplication", mock.Anything, svcTestClientID).
		Return(s.existingOAuthClient(), (*tidcommon.ServiceError)(nil))
	s.mockAppService.On("GetApplication", mock.Anything, svcTestAppID).
		Return(s.existingApplication(), (*tidcommon.ServiceError)(nil))
	authn.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, providers.AuthenticatedClaims{},
			&tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "AUTH-1001"})

	request := &DCRUpdateRequest{
		ClientSecret: "wrong-secret",
		DCRRegistrationRequest: DCRRegistrationRequest{
			RedirectURIs: []string{"https://client.example.com/cb"},
		},
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
func (s *DCRServiceTestSuite) TestUpdateClient_MatchingClientSecretIsAccepted() {
	authn := managermock.NewAuthnProviderManagerMock(s.T())
	svc := newDCRService(s.mockAppService, s.mockOUService, nil, authn, &MockTransactioner{})

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

	request := &DCRUpdateRequest{
		ClientSecret: "correct-secret",
		DCRRegistrationRequest: DCRRegistrationRequest{
			RedirectURIs: []string{"https://client.example.com/cb"},
		},
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
func (s *DCRServiceTestSuite) TestUpdateClient_OmittedClientSecretSkipsVerification() {
	authn := managermock.NewAuthnProviderManagerMock(s.T())
	svc := newDCRService(s.mockAppService, s.mockOUService, nil, authn, &MockTransactioner{})

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
		&DCRUpdateRequest{
			DCRRegistrationRequest: DCRRegistrationRequest{
				RedirectURIs: []string{"https://client.example.com/cb"},
			},
		})

	s.Require().Nil(err)
	authn.AssertNotCalled(s.T(), "AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything)
}

// A client that does not already authenticate with a secret is issued one when it adopts a secret
// based method, and that value exists in plaintext only in this response. Keying the response off
// the client having been public would miss the private_key_jwt case, leaving a client that moved to
// client_secret_basic holding a credential nobody can produce.
func (s *DCRServiceTestSuite) TestUpdateClient_IssuedSecretIsReturnedWhenAdoptingASecret() {
	cases := []struct {
		name         string
		publicClient bool
		authMethod   providers.TokenEndpointAuthMethod
	}{
		{"public client becomes confidential", true, providers.TokenEndpointAuthMethodNone},
		{
			"confidential client leaves private_key_jwt",
			false,
			providers.TokenEndpointAuthMethodPrivateKeyJWT,
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			appSvc := applicationmock.NewApplicationServiceInterfaceMock(s.T())
			svc := newDCRService(appSvc, s.mockOUService, nil, nil, &MockTransactioner{})

			existingClient := s.existingOAuthClient()
			existingClient.PublicClient = tc.publicClient
			existingClient.TokenEndpointAuthMethod = tc.authMethod

			appSvc.On("GetOAuthApplication", mock.Anything, svcTestClientID).
				Return(existingClient, (*tidcommon.ServiceError)(nil))
			appSvc.On("GetApplication", mock.Anything, svcTestAppID).
				Return(s.existingApplication(), (*tidcommon.ServiceError)(nil))
			// The client did not authenticate with a secret before this update, so the application
			// service generates one and returns it on the updated application.
			appSvc.On("UpdateApplication", mock.Anything, svcTestAppID,
				mock.AnythingOfType("*model.ApplicationDTO")).
				Return(&model.ApplicationDTO{
					ID:   svcTestAppID,
					Name: "Existing Client",
					InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
						{
							Type: providers.OAuthInboundAuthType,
							OAuthConfig: &providers.OAuthConfigWithSecret{
								ClientID:     svcTestClientID,
								ClientSecret: "newly-issued-secret",
							},
						},
					},
				}, (*tidcommon.ServiceError)(nil))

			response, err := svc.UpdateClient(context.Background(), svcTestClientID,
				&DCRUpdateRequest{
					ClientID: svcTestClientID,
					DCRRegistrationRequest: DCRRegistrationRequest{
						RedirectURIs:            []string{"https://client.example.com/cb"},
						TokenEndpointAuthMethod: "client_secret_basic",
					},
				})

			s.Require().Nil(err)
			s.Require().NotNil(response)
			s.Equal("newly-issued-secret", response.ClientSecret)
		})
	}
}

// PKCE is derived from the client type at registration, and a public client is required to have it.
// An update that changes the type has to derive it afresh: carrying a disabled flag into a client
// becoming public would be rejected outright, leaving a legitimate transition with no way through
// the API, since RFC 7591 has no field for PKCE.
func (s *DCRServiceTestSuite) TestUpdateClient_PKCEFollowsAChangeOfClientType() {
	cases := []struct {
		name            string
		existingPublic  bool
		existingPKCE    bool
		requestedMethod providers.TokenEndpointAuthMethod
		wantPKCE        bool
	}{
		{"confidential without PKCE becomes public", false, false,
			providers.TokenEndpointAuthMethodNone, true},
		{"public becomes confidential", true, true,
			providers.TokenEndpointAuthMethodClientSecretBasic, false},
		{"confidential with PKCE stays confidential", false, true,
			providers.TokenEndpointAuthMethodClientSecretBasic, true},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			appSvc := applicationmock.NewApplicationServiceInterfaceMock(s.T())
			svc := newDCRService(appSvc, s.mockOUService, nil, nil, &MockTransactioner{})

			existingClient := s.existingOAuthClient()
			existingClient.PublicClient = tc.existingPublic
			existingClient.PKCERequired = tc.existingPKCE

			appSvc.On("GetOAuthApplication", mock.Anything, svcTestClientID).
				Return(existingClient, (*tidcommon.ServiceError)(nil))
			appSvc.On("GetApplication", mock.Anything, svcTestAppID).
				Return(s.existingApplication(), (*tidcommon.ServiceError)(nil))

			var captured *model.ApplicationDTO
			appSvc.On("UpdateApplication", mock.Anything, svcTestAppID,
				mock.AnythingOfType("*model.ApplicationDTO")).
				Run(func(args mock.Arguments) {
					captured = args.Get(2).(*model.ApplicationDTO)
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

			_, err := svc.UpdateClient(context.Background(), svcTestClientID, &DCRUpdateRequest{
				ClientID: svcTestClientID,
				DCRRegistrationRequest: DCRRegistrationRequest{
					RedirectURIs:            []string{"https://client.example.com/cb"},
					GrantTypes:              []providers.GrantType{providers.GrantTypeAuthorizationCode},
					TokenEndpointAuthMethod: tc.requestedMethod,
				},
			})

			s.Require().Nil(err)
			s.Require().NotNil(captured)
			s.Require().NotEmpty(captured.InboundAuthConfig)
			s.Equal(tc.wantPKCE, captured.InboundAuthConfig[0].OAuthConfig.PKCERequired)
		})
	}
}

// The response algorithms are RFC 7591 metadata, so an update that omits them clears them like any
// other omitted field. What the metadata cannot express, the ID token validity and the user
// attributes, is carried over rather than being dropped along with them.
func (s *DCRServiceTestSuite) TestUpdateClient_OmittedResponseAlgorithmsAreCleared() {
	appSvc := applicationmock.NewApplicationServiceInterfaceMock(s.T())
	svc := newDCRService(appSvc, s.mockOUService, nil, nil, &MockTransactioner{})

	existingClient := s.existingOAuthClient()
	existingClient.Token = &providers.OAuthTokenConfig{
		IDToken: &providers.IDTokenConfig{
			ResponseType:   providers.IDTokenResponseTypeJWE,
			SigningAlg:     "RS256",
			EncryptionAlg:  "RSA-OAEP",
			ValidityPeriod: 2345,
			UserAttributes: []string{"email"},
		},
	}
	existingClient.UserInfo = &providers.UserInfoConfig{
		ResponseType:   providers.UserInfoResponseTypeJWS,
		SigningAlg:     "RS256",
		UserAttributes: []string{"picture"},
	}

	appSvc.On("GetOAuthApplication", mock.Anything, svcTestClientID).
		Return(existingClient, (*tidcommon.ServiceError)(nil))
	appSvc.On("GetApplication", mock.Anything, svcTestAppID).
		Return(s.existingApplication(), (*tidcommon.ServiceError)(nil))

	var captured *model.ApplicationDTO
	appSvc.On("UpdateApplication", mock.Anything, svcTestAppID,
		mock.AnythingOfType("*model.ApplicationDTO")).
		Run(func(args mock.Arguments) {
			captured = args.Get(2).(*model.ApplicationDTO)
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

	// The request names no response algorithm at all.
	_, err := svc.UpdateClient(context.Background(), svcTestClientID, &DCRUpdateRequest{
		ClientID: svcTestClientID,
		DCRRegistrationRequest: DCRRegistrationRequest{
			RedirectURIs: []string{"https://client.example.com/cb"},
		},
	})

	s.Require().Nil(err)
	s.Require().NotNil(captured)
	s.Require().NotEmpty(captured.InboundAuthConfig)
	cfg := captured.InboundAuthConfig[0].OAuthConfig

	s.Require().NotNil(cfg.Token)
	s.Require().NotNil(cfg.Token.IDToken)
	s.Empty(cfg.Token.IDToken.SigningAlg, "an omitted id_token_signed_response_alg is cleared")
	s.Empty(cfg.Token.IDToken.EncryptionAlg, "an omitted id_token_encrypted_response_alg is cleared")
	// Not expressible through RFC 7591, so the update leaves them alone.
	s.Equal(int64(2345), cfg.Token.IDToken.ValidityPeriod)
	s.Equal([]string{"email"}, cfg.Token.IDToken.UserAttributes)

	s.Require().NotNil(cfg.UserInfo)
	s.Empty(cfg.UserInfo.SigningAlg, "an omitted userinfo_signed_response_alg is cleared")
	s.Equal([]string{"picture"}, cfg.UserInfo.UserAttributes)
}

// A client that was already confidential keeps its stored secret, which is hashed and not rotated by
// an update, so the response carries none rather than implying a new credential was issued.
func (s *DCRServiceTestSuite) TestUpdateClient_NoSecretReturnedForAnAlreadyConfidentialClient() {
	existingClient := s.existingOAuthClient()
	existingClient.PublicClient = false

	s.mockAppService.On("GetOAuthApplication", mock.Anything, svcTestClientID).
		Return(existingClient, (*tidcommon.ServiceError)(nil))
	s.mockAppService.On("GetApplication", mock.Anything, svcTestAppID).
		Return(s.existingApplication(), (*tidcommon.ServiceError)(nil))
	s.mockAppService.On("UpdateApplication", mock.Anything, svcTestAppID,
		mock.AnythingOfType("*model.ApplicationDTO")).
		Return(&model.ApplicationDTO{
			ID:   svcTestAppID,
			Name: "Existing Client",
			InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
				{
					Type: providers.OAuthInboundAuthType,
					// The application service issues no secret for a client that already
					// authenticates with one, so none comes back on the updated application.
					OAuthConfig: &providers.OAuthConfigWithSecret{
						ClientID: svcTestClientID,
					},
				},
			},
		}, (*tidcommon.ServiceError)(nil))

	response, err := s.service.UpdateClient(context.Background(), svcTestClientID,
		&DCRUpdateRequest{
			ClientID: svcTestClientID,
			DCRRegistrationRequest: DCRRegistrationRequest{
				RedirectURIs:            []string{"https://client.example.com/cb"},
				TokenEndpointAuthMethod: "client_secret_basic",
			},
		})

	s.Require().Nil(err)
	s.Require().NotNil(response)
	s.Empty(response.ClientSecret)
}

// RFC 7591 metadata is a subset of what an application holds. An update rebuilds the record from
// the request, so everything the protocol cannot express has to be carried forward: a caller reading
// a registration never sees these properties and so cannot resubmit them, and they are configured
// through the application API rather than here. Without this, writing back an unchanged read would
// reset whatever the Console had configured.
func (s *DCRServiceTestSuite) TestUpdateClient_PreservesSettingsDCRCannotExpress() {
	existingApp := s.existingApplication()
	existingApp.Description = "Configured in the Console"
	existingApp.Template = "custom-template"
	existingApp.Metadata = map[string]interface{}{"team": "identity"}

	existingClient := s.existingOAuthClient()
	existingClient.PKCERequired = true
	existingClient.IncludeActClaim = true
	existingClient.AcrValues = []string{"urn:acr:high"}
	existingClient.ScopeClaims = map[string][]string{"profile": {"name"}}
	existingClient.Token = &providers.OAuthTokenConfig{
		IDToken: &providers.IDTokenConfig{
			ValidityPeriod: 900,
			UserAttributes: []string{"email"},
			SigningAlg:     "RS256",
		},
		AccessToken: &providers.AccessTokenConfig{DefaultAudience: "https://api.example.com"},
	}
	existingClient.UserInfo = &providers.UserInfoConfig{
		ResponseType:   providers.UserInfoResponseTypeJSON,
		UserAttributes: []string{"email", "phone_number"},
	}

	s.mockAppService.On("GetOAuthApplication", mock.Anything, svcTestClientID).
		Return(existingClient, (*tidcommon.ServiceError)(nil))
	s.mockAppService.On("GetApplication", mock.Anything, svcTestAppID).
		Return(existingApp, (*tidcommon.ServiceError)(nil))

	var captured *model.ApplicationDTO
	s.mockAppService.On("UpdateApplication", mock.Anything, svcTestAppID,
		mock.AnythingOfType("*model.ApplicationDTO")).
		Run(func(args mock.Arguments) { captured = args.Get(2).(*model.ApplicationDTO) }).
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

	// A plain metadata update, carrying only what RFC 7591 can express.
	_, err := s.service.UpdateClient(context.Background(), svcTestClientID, &DCRUpdateRequest{
		ClientID: svcTestClientID,
		DCRRegistrationRequest: DCRRegistrationRequest{
			ClientName:   "Renamed",
			RedirectURIs: []string{"https://client.example.com/cb"},
		},
	})

	s.Require().Nil(err)
	s.Require().NotNil(captured)
	s.Equal("Configured in the Console", captured.Description)
	s.Equal("custom-template", captured.Template)
	s.Equal(map[string]interface{}{"team": "identity"}, captured.Metadata)

	s.Require().NotEmpty(captured.InboundAuthConfig)
	cfg := captured.InboundAuthConfig[0].OAuthConfig
	s.Require().NotNil(cfg)
	s.True(cfg.PKCERequired, "a confidential client deliberately requiring PKCE must keep it")
	s.True(cfg.IncludeActClaim)
	s.Equal([]string{"urn:acr:high"}, cfg.AcrValues)
	s.Equal(map[string][]string{"profile": {"name"}}, cfg.ScopeClaims)

	s.Require().NotNil(cfg.Token)
	s.Require().NotNil(cfg.Token.AccessToken)
	s.Equal("https://api.example.com", cfg.Token.AccessToken.DefaultAudience)
	s.Require().NotNil(cfg.Token.IDToken)
	s.Equal(int64(900), cfg.Token.IDToken.ValidityPeriod)
	s.Equal([]string{"email"}, cfg.Token.IDToken.UserAttributes)

	s.Require().NotNil(cfg.UserInfo)
	s.Equal([]string{"email", "phone_number"}, cfg.UserInfo.UserAttributes)
}

// An update replaces the registration in full, so localized variants the request omits must not
// survive it. The i18n store upserts, so without an explicit clear a translation the caller dropped
// would keep resolving with no way to remove it through this API.
func (s *DCRServiceTestSuite) TestUpdateClient_OmittedLocalizedVariantIsCleared() {
	i18nSvc := i18nmock.NewI18nServiceInterfaceMock(s.T())
	svc := newDCRService(s.mockAppService, s.mockOUService, i18nSvc, nil, &MockTransactioner{})

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
	i18nSvc.On("GetTranslationsByKeys", mock.Anything, application.AppI18nNamespace(),
		mock.Anything).
		Return(map[string]map[string]string{}, (*tidcommon.ServiceError)(nil))
	i18nSvc.On("DeleteTranslationsByKey", mock.Anything, application.AppI18nNamespace(),
		mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	i18nSvc.On("SetTranslationOverridesForNamespace", mock.Anything, application.AppI18nNamespace(),
		mock.Anything).Return((*tidcommon.ServiceError)(nil))

	request := &DCRUpdateRequest{
		DCRRegistrationRequest: DCRRegistrationRequest{
			RedirectURIs:        []string{"https://client.example.com/cb"},
			LocalizedClientName: map[string]string{"fr": "Mon Application"},
		},
	}
	_, err := svc.UpdateClient(context.Background(), svcTestClientID, request)

	s.Require().Nil(err)
	// The name key is cleared before the new variants are written, so a previously stored variant
	// the request omits does not survive the update.
	i18nSvc.AssertCalled(s.T(), "DeleteTranslationsByKey", mock.Anything,
		application.AppI18nNamespace(), nameKey)
}

// An update replaces the registration in full, so a localizable field the request leaves out is
// cleared from the record and its translations go with it. Keeping them would leave the field
// resolving to a translation while its base value is gone, and the caller could not remove them
// afterwards, because omitting the field is what created that state.
func (s *DCRServiceTestSuite) TestUpdateClient_UnmentionedLocalizedFieldIsCleared() {
	i18nSvc := i18nmock.NewI18nServiceInterfaceMock(s.T())
	svc := newDCRService(s.mockAppService, s.mockOUService, i18nSvc, nil, &MockTransactioner{})

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

	i18nSvc.On("GetTranslationsByKeys", mock.Anything, application.AppI18nNamespace(),
		mock.Anything).
		Return(map[string]map[string]string{}, (*tidcommon.ServiceError)(nil))
	i18nSvc.On("DeleteTranslationsByKey", mock.Anything, application.AppI18nNamespace(),
		mock.Anything).Return((*tidcommon.ServiceError)(nil))
	i18nSvc.On("SetTranslationOverridesForNamespace", mock.Anything, application.AppI18nNamespace(),
		mock.Anything).Return((*tidcommon.ServiceError)(nil))

	// The request names the client but says nothing about the logo, which the update therefore drops.
	request := &DCRUpdateRequest{
		DCRRegistrationRequest: DCRRegistrationRequest{
			ClientName:   "Renamed Client",
			RedirectURIs: []string{"https://client.example.com/cb"},
		},
	}
	_, err := svc.UpdateClient(context.Background(), svcTestClientID, request)

	s.Require().Nil(err)
	i18nSvc.AssertCalled(s.T(), "DeleteTranslationsByKey", mock.Anything,
		application.AppI18nNamespace(), application.AppI18nKey(svcTestAppID, "logo_uri"))
}

// The client name is the one field an update cannot drop, because a registration must always have a
// name. An update that mentions neither the name nor any variant of it leaves the stored name and
// its translations alone, rather than clearing them the way every other omitted field is cleared.
func (s *DCRServiceTestSuite) TestUpdateClient_UnmentionedClientNameKeepsItsVariants() {
	i18nSvc := i18nmock.NewI18nServiceInterfaceMock(s.T())
	svc := newDCRService(s.mockAppService, s.mockOUService, i18nSvc, nil, &MockTransactioner{})

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

	i18nSvc.On("GetTranslationsByKeys", mock.Anything, application.AppI18nNamespace(),
		mock.Anything).
		Return(map[string]map[string]string{}, (*tidcommon.ServiceError)(nil))
	i18nSvc.On("DeleteTranslationsByKey", mock.Anything, application.AppI18nNamespace(),
		mock.Anything).Return((*tidcommon.ServiceError)(nil))
	// No localizable field is supplied, so the update has nothing to write back and
	// SetTranslationOverridesForNamespace is never reached.

	// A metadata-only update: nothing in it names the client.
	request := &DCRUpdateRequest{
		DCRRegistrationRequest: DCRRegistrationRequest{
			RedirectURIs: []string{"https://client.example.com/cb"},
		},
	}
	_, err := svc.UpdateClient(context.Background(), svcTestClientID, request)

	s.Require().Nil(err)
	i18nSvc.AssertNotCalled(s.T(), "DeleteTranslationsByKey", mock.Anything,
		application.AppI18nNamespace(), application.AppI18nKey(svcTestAppID, "name"))
}

// Only the name is stored as an i18n reference; the logo, terms and policy URIs are stored as
// literal values on the application record. An update supplying just a language variant of one of
// those leaves its base value out of the request, which would otherwise clear it from the record and
// make a later read report an empty field while the preserved base sat unreachable in the i18n
// store. The existing value is carried forward instead, so only a request sending the field itself
// changes it.
func (s *DCRServiceTestSuite) TestUpdateClient_VariantOnlyUpdateKeepsBaseURIFields() {
	i18nSvc := i18nmock.NewI18nServiceInterfaceMock(s.T())
	svc := newDCRService(s.mockAppService, s.mockOUService, i18nSvc, nil, &MockTransactioner{})

	s.mockAppService.On("GetOAuthApplication", mock.Anything, svcTestClientID).
		Return(s.existingOAuthClient(), (*tidcommon.ServiceError)(nil))
	s.mockAppService.On("GetApplication", mock.Anything, svcTestAppID).
		Return(s.existingApplication(), (*tidcommon.ServiceError)(nil))

	var written *model.ApplicationDTO
	s.mockAppService.On("UpdateApplication", mock.Anything, svcTestAppID,
		mock.AnythingOfType("*model.ApplicationDTO")).
		Run(func(args mock.Arguments) {
			written = args.Get(2).(*model.ApplicationDTO)
		}).
		Return(&model.ApplicationDTO{
			ID:      svcTestAppID,
			Name:    "Existing Client",
			LogoURL: "https://client.example.com/logo.png",
			InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
				{
					Type:        providers.OAuthInboundAuthType,
					OAuthConfig: &providers.OAuthConfigWithSecret{ClientID: svcTestClientID},
				},
			},
		}, (*tidcommon.ServiceError)(nil))

	i18nSvc.On("GetTranslationsByKeys", mock.Anything, application.AppI18nNamespace(),
		mock.Anything).
		Return(map[string]map[string]string{}, (*tidcommon.ServiceError)(nil))
	i18nSvc.On("DeleteTranslationsByKey", mock.Anything, application.AppI18nNamespace(),
		mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	i18nSvc.On("SetTranslationOverridesForNamespace", mock.Anything, application.AppI18nNamespace(),
		mock.Anything).Return((*tidcommon.ServiceError)(nil))

	// The request localizes the logo without restating its base value.
	request := &DCRUpdateRequest{
		DCRRegistrationRequest: DCRRegistrationRequest{
			RedirectURIs:     []string{"https://client.example.com/cb"},
			LocalizedLogoURI: map[string]string{"fr": "https://client.example.fr/logo.png"},
		},
	}
	response, err := svc.UpdateClient(context.Background(), svcTestClientID, request)

	s.Require().Nil(err)
	s.Require().NotNil(written)
	s.Equal("https://client.example.com/logo.png", written.LogoURL,
		"a variant-only update must not clear the stored logo_uri")
	s.Require().NotNil(response)
	s.Equal("https://client.example.com/logo.png", response.LogoURI)
	s.Equal(map[string]string{"fr": "https://client.example.fr/logo.png"}, response.LocalizedLogoURI)
}

// An invalid localized URI must be rejected before the application is written. The localized
// variants are persisted in a separate store after the application is replaced, so validating them
// late would leave the registration updated while the caller is told the request was invalid.
func (s *DCRServiceTestSuite) TestUpdateClient_InvalidLocalizedURIRejectedBeforeWrite() {
	i18nSvc := i18nmock.NewI18nServiceInterfaceMock(s.T())
	svc := newDCRService(s.mockAppService, s.mockOUService, i18nSvc, nil, &MockTransactioner{})

	request := &DCRUpdateRequest{
		DCRRegistrationRequest: DCRRegistrationRequest{
			ClientName:      "Renamed Client",
			RedirectURIs:    []string{"https://client.example.com/cb"},
			LocalizedTosURI: map[string]string{"fr": "not a valid uri"},
		},
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
func (s *DCRServiceTestSuite) TestUpdateClient_UnknownClient() {
	s.mockAppService.On("GetOAuthApplication", mock.Anything, svcTestClientID).
		Return((*providers.OAuthClient)(nil), &tidcommon.ServiceError{
			Type: tidcommon.ClientErrorType, Code: "APP-1001",
		})

	response, err := s.service.UpdateClient(context.Background(), svcTestClientID,
		&DCRUpdateRequest{
			DCRRegistrationRequest: DCRRegistrationRequest{
				RedirectURIs: []string{"https://client.example.com/cb"},
			},
		})

	s.Nil(response)
	s.Require().NotNil(err)
	s.Equal(ErrorClientNotFound.Code, err.Code)
	s.mockAppService.AssertNotCalled(s.T(), "UpdateApplication", mock.Anything, mock.Anything,
		mock.Anything)
}

// Conflicting JWKS configuration is rejected on update, as it is on registration.
func (s *DCRServiceTestSuite) TestUpdateClient_JWKSConflict() {
	response, err := s.service.UpdateClient(context.Background(), svcTestClientID,
		&DCRUpdateRequest{
			DCRRegistrationRequest: DCRRegistrationRequest{
				RedirectURIs: []string{"https://client.example.com/cb"},
				JWKSUri:      "https://client.example.com/jwks.json",
				JWKS:         map[string]interface{}{"keys": []interface{}{}},
			},
		})

	s.Nil(response)
	s.Require().NotNil(err)
	s.Equal(ErrorJWKSConfigurationConflict.Code, err.Code)
}

// UC-4: deleting a registration removes the underlying application.
func (s *DCRServiceTestSuite) TestDeleteClient_RemovesApplication() {
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
func (s *DCRServiceTestSuite) TestDeleteClient_UnknownClient() {
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
func (s *DCRServiceTestSuite) TestRegisterClient_OmitsClientConfigurationFields() {
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
func (s *DCRServiceTestSuite) TestUpdateClient_OmitsClientConfigurationFields() {
	s.expectUpdate()

	response, err := s.service.UpdateClient(context.Background(), svcTestClientID,
		&DCRUpdateRequest{
			DCRRegistrationRequest: DCRRegistrationRequest{
				RedirectURIs: []string{"https://client.example.com/cb"},
			},
		})

	s.Require().Nil(err)
	s.Equal(svcTestClientID, response.ClientID)
	s.Empty(response.ClientSecret, "an update must not rotate or expose the client secret")
}

// A failure to look up the OAuth client that is not a client error is reported as a server error,
// so a storage outage is not mistaken for a missing registration.
func (s *DCRServiceTestSuite) TestGetClient_OAuthLookupServerError() {
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
func (s *DCRServiceTestSuite) TestGetClient_ApplicationLookupNotFound() {
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
func (s *DCRServiceTestSuite) TestGetClient_ApplicationLookupServerError() {
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
func (s *DCRServiceTestSuite) TestGetClient_EchoesJWKSURI() {
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
func (s *DCRServiceTestSuite) TestGetClient_EchoesInlineJWKS() {
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
func (s *DCRServiceTestSuite) TestGetClient_MalformedStoredJWKS() {
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
func (s *DCRServiceTestSuite) TestGetClient_EchoesResponseAlgorithms() {
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
func (s *DCRServiceTestSuite) TestDeleteClient_DeleteServerError() {
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
func (s *DCRServiceTestSuite) TestDeleteClient_DeleteClientError() {
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
func (s *DCRServiceTestSuite) TestUpdateClient_NilRequest() {
	response, err := s.service.UpdateClient(context.Background(), svcTestClientID, nil)

	s.Nil(response)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidRequestFormat.Code, err.Code)
	s.mockAppService.AssertNotCalled(s.T(), "GetOAuthApplication", mock.Anything, mock.Anything)
}

// A JWKS URI that is not an absolute HTTPS URL is rejected as invalid metadata.
func (s *DCRServiceTestSuite) TestUpdateClient_InsecureJWKSURI() {
	response, err := s.service.UpdateClient(context.Background(), svcTestClientID,
		&DCRUpdateRequest{
			DCRRegistrationRequest: DCRRegistrationRequest{
				JWKSUri: "http://client.example.com/jwks.json",
			},
		})

	s.Nil(response)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidClientMetadata.Code, err.Code)
	s.mockAppService.AssertNotCalled(s.T(), "GetOAuthApplication", mock.Anything, mock.Anything)
}

func (s *DCRServiceTestSuite) TestRegisterClient_BackchannelLogoutMetadataRoundTrip() {
	request := &DCRRegistrationRequest{
		OUID:                   "test-ou-1",
		RedirectURIs:           []string{"https://client.example.com/callback"},
		PostLogoutRedirectURIs: []string{"https://client.example.com/logged-out"},
		BackchannelLogoutURI:   "https://client.example.com/backchannel-logout",
		GrantTypes:             []providers.GrantType{providers.GrantTypeAuthorizationCode},
	}
	appDTO := &model.ApplicationDTO{
		ID: "app-id", Name: "client",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{{
			Type: providers.OAuthInboundAuthType,
			OAuthConfig: &providers.OAuthConfigWithSecret{
				ClientID:               "client-id",
				PostLogoutRedirectURIs: request.PostLogoutRedirectURIs,
				BackchannelLogoutURI:   request.BackchannelLogoutURI,
			},
		}},
	}
	s.mockAppService.On("CreateApplication", mock.Anything, mock.MatchedBy(func(dto *model.ApplicationDTO) bool {
		cfg := dto.InboundAuthConfig[0].OAuthConfig
		return cfg.BackchannelLogoutURI == request.BackchannelLogoutURI &&
			len(cfg.PostLogoutRedirectURIs) == 1
	})).Return(appDTO, (*tidcommon.ServiceError)(nil))

	response, err := s.service.RegisterClient(context.Background(), request)

	s.Nil(err)
	s.Require().NotNil(response)
	s.Equal(request.PostLogoutRedirectURIs, response.PostLogoutRedirectURIs)
	s.Equal(request.BackchannelLogoutURI, response.BackchannelLogoutURI)
}

// Reading a registration back returns the back-channel logout URI the client registered.
func (s *DCRServiceTestSuite) TestGetClient_ReturnsBackchannelLogoutURI() {
	client := s.existingOAuthClient()
	client.PostLogoutRedirectURIs = []string{"https://client.example.com/logged-out"}
	client.BackchannelLogoutURI = "https://client.example.com/backchannel-logout"
	s.mockAppService.On("GetOAuthApplication", mock.Anything, svcTestClientID).
		Return(client, (*tidcommon.ServiceError)(nil))
	s.mockAppService.On("GetApplication", mock.Anything, svcTestAppID).
		Return(s.existingApplication(), (*tidcommon.ServiceError)(nil))

	response, err := s.service.GetClient(context.Background(), svcTestClientID)

	s.Require().Nil(err)
	s.Equal(client.PostLogoutRedirectURIs, response.PostLogoutRedirectURIs)
	s.Equal(client.BackchannelLogoutURI, response.BackchannelLogoutURI)
}
