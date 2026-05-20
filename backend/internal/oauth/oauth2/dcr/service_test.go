/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package dcr

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/application"
	"github.com/thunder-id/thunderid/internal/application/model"
	"github.com/thunder-id/thunderid/internal/cert"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	i18ncore "github.com/thunder-id/thunderid/internal/system/i18n/core"
	i18nmgt "github.com/thunder-id/thunderid/internal/system/i18n/mgt"
	"github.com/thunder-id/thunderid/tests/mocks/applicationmock"
	i18nmock "github.com/thunder-id/thunderid/tests/mocks/i18n/mgtmock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
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
	s.service = newDCRService(s.mockAppService, s.mockOUService, nil, &MockTransactioner{})
}

// TestNewDCRService tests the service constructor
func (s *DCRServiceTestSuite) TestNewDCRService() {
	service := newDCRService(s.mockAppService, s.mockOUService, nil, &MockTransactioner{})
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
		GrantTypes:   []oauth2const.GrantType{oauth2const.GrantTypeAuthorizationCode},
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
		GrantTypes:   []oauth2const.GrantType{oauth2const.GrantTypeAuthorizationCode},
		ClientName:   "Test Client",
	}

	appDTO := &model.ApplicationDTO{
		ID:   "app-id",
		Name: "Test Client",
		InboundAuthConfig: []inboundmodel.InboundAuthConfigWithSecret{
			{
				Type: inboundmodel.OAuthInboundAuthType,
				OAuthConfig: &inboundmodel.OAuthConfigWithSecret{
					ClientID:     "client-id",
					ClientSecret: "client-secret",
					Scopes:       []string{},
				},
			},
		},
	}

	s.mockAppService.On(
		"CreateApplication", mock.Anything, mock.AnythingOfType("*model.ApplicationDTO"),
	).Return(appDTO, (*serviceerror.ServiceError)(nil))

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
		GrantTypes:   []oauth2const.GrantType{oauth2const.GrantTypeAuthorizationCode},
		ClientName:   "Test Client",
		JWKSUri:      "https://client.example.com/.well-known/jwks.json",
	}

	appDTO := &model.ApplicationDTO{
		ID:   "app-id",
		Name: "Test Client",
		InboundAuthConfig: []inboundmodel.InboundAuthConfigWithSecret{
			{
				Type: inboundmodel.OAuthInboundAuthType,
				OAuthConfig: &inboundmodel.OAuthConfigWithSecret{
					ClientID:     "client-id",
					ClientSecret: "client-secret",
					Scopes:       []string{},
				},
			},
		},
		InboundAuthProfile: inboundmodel.InboundAuthProfile{
			Certificate: &inboundmodel.Certificate{
				Type:  cert.CertificateTypeJWKSURI,
				Value: "https://client.example.com/.well-known/jwks.json",
			},
		},
	}

	s.mockAppService.On(
		"CreateApplication", mock.Anything, mock.AnythingOfType("*model.ApplicationDTO"),
	).Return(appDTO, (*serviceerror.ServiceError)(nil))

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
		GrantTypes:   []oauth2const.GrantType{oauth2const.GrantTypeAuthorizationCode},
	}

	appServiceErr := &serviceerror.ServiceError{
		Type:             serviceerror.ClientErrorType,
		Code:             "APP-1012",
		Error:            i18ncore.I18nMessage{DefaultValue: "Invalid redirect URI"},
		ErrorDescription: i18ncore.I18nMessage{DefaultValue: "The redirect URI is invalid"},
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
			name:            "Server Error APP-5001",
			appErrCode:      "APP-5001",
			expectedDCRCode: ErrorServerError.Code,
		},
		{
			name:            "Server Error APP-5002",
			appErrCode:      "APP-5002",
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
			appErr := &serviceerror.ServiceError{
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
		GrantTypes:   []oauth2const.GrantType{oauth2const.GrantTypeAuthorizationCode},
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
		GrantTypes:   []oauth2const.GrantType{oauth2const.GrantTypeAuthorizationCode},
		ClientName:   "Test Client",
	}

	appDTO := &model.ApplicationDTO{
		ID:   "app-id",
		Name: "Test Client",
		InboundAuthConfig: []inboundmodel.InboundAuthConfigWithSecret{
			{
				Type: inboundmodel.OAuthInboundAuthType,
				OAuthConfig: &inboundmodel.OAuthConfigWithSecret{
					ClientID:     "client-id",
					ClientSecret: "client-secret",
					Scopes:       []string{},
				},
			},
		},
		InboundAuthProfile: inboundmodel.InboundAuthProfile{
			Certificate: &inboundmodel.Certificate{
				Type:  cert.CertificateTypeJWKS,
				Value: "invalid json",
			},
		},
	}

	s.mockAppService.On(
		"CreateApplication", mock.Anything, mock.AnythingOfType("*model.ApplicationDTO"),
	).Return(appDTO, (*serviceerror.ServiceError)(nil))

	response, err := s.service.RegisterClient(context.Background(), request)

	s.Nil(response)
	s.NotNil(err)
	s.Equal(ErrorServerError.Code, err.Code)
}

func (s *DCRServiceTestSuite) TestRegisterClient_WithJWKS() {
	request := &DCRRegistrationRequest{
		OUID:         "test-ou-1",
		RedirectURIs: []string{"https://client.example.com/callback"},
		GrantTypes:   []oauth2const.GrantType{oauth2const.GrantTypeAuthorizationCode},
		ClientName:   "Test Client",
		JWKS:         map[string]interface{}{"keys": []interface{}{}},
	}

	appDTO := &model.ApplicationDTO{
		ID:   "app-id",
		Name: "Test Client",
		InboundAuthConfig: []inboundmodel.InboundAuthConfigWithSecret{
			{
				Type: inboundmodel.OAuthInboundAuthType,
				OAuthConfig: &inboundmodel.OAuthConfigWithSecret{
					ClientID:     "client-id",
					ClientSecret: "client-secret",
					Scopes:       []string{},
				},
			},
		},
		InboundAuthProfile: inboundmodel.InboundAuthProfile{
			Certificate: &inboundmodel.Certificate{
				Type:  cert.CertificateTypeJWKS,
				Value: `{"keys":[]}`,
			},
		},
	}

	s.mockAppService.On(
		"CreateApplication", mock.Anything, mock.AnythingOfType("*model.ApplicationDTO"),
	).Return(appDTO, (*serviceerror.ServiceError)(nil))

	response, err := s.service.RegisterClient(context.Background(), request)

	s.NotNil(response)
	s.Nil(err)
	s.NotNil(response.JWKS)
}

func (s *DCRServiceTestSuite) TestRegisterClient_WithScope() {
	request := &DCRRegistrationRequest{
		OUID:         "test-ou-1",
		RedirectURIs: []string{"https://client.example.com/callback"},
		GrantTypes:   []oauth2const.GrantType{oauth2const.GrantTypeAuthorizationCode},
		ClientName:   "Test Client",
		Scope:        "read write admin",
	}

	appDTO := &model.ApplicationDTO{
		ID:   "app-id",
		Name: "Test Client",
		InboundAuthConfig: []inboundmodel.InboundAuthConfigWithSecret{
			{
				Type: inboundmodel.OAuthInboundAuthType,
				OAuthConfig: &inboundmodel.OAuthConfigWithSecret{
					ClientID:     "client-id",
					ClientSecret: "client-secret",
					Scopes:       []string{"read", "write", "admin"},
				},
			},
		},
	}

	s.mockAppService.On(
		"CreateApplication", mock.Anything, mock.AnythingOfType("*model.ApplicationDTO"),
	).Return(appDTO, (*serviceerror.ServiceError)(nil))

	response, err := s.service.RegisterClient(context.Background(), request)

	s.NotNil(response)
	s.Nil(err)
	s.Equal("read write admin", response.Scope)
}

func (s *DCRServiceTestSuite) TestRegisterClient_RequirePushedAuthorizationRequests() {
	request := &DCRRegistrationRequest{
		OUID:                               "test-ou-1",
		RedirectURIs:                       []string{"https://client.example.com/callback"},
		GrantTypes:                         []oauth2const.GrantType{oauth2const.GrantTypeAuthorizationCode},
		ClientName:                         "Test Client",
		RequirePushedAuthorizationRequests: true,
	}

	appDTO := &model.ApplicationDTO{
		ID:   "app-id",
		Name: "Test Client",
		InboundAuthConfig: []inboundmodel.InboundAuthConfigWithSecret{
			{
				Type: inboundmodel.OAuthInboundAuthType,
				OAuthConfig: &inboundmodel.OAuthConfigWithSecret{
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
	).Return(appDTO, (*serviceerror.ServiceError)(nil))

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
		GrantTypes:   []oauth2const.GrantType{oauth2const.GrantTypeAuthorizationCode},
		ClientName:   "Test Client",
	}

	appDTO := &model.ApplicationDTO{
		ID:                "app-id",
		Name:              "Test Client",
		InboundAuthConfig: []inboundmodel.InboundAuthConfigWithSecret{},
	}

	s.mockAppService.On(
		"CreateApplication", mock.Anything, mock.AnythingOfType("*model.ApplicationDTO"),
	).Return(appDTO, (*serviceerror.ServiceError)(nil))

	response, err := s.service.RegisterClient(context.Background(), request)

	s.Nil(response)
	s.NotNil(err)
	s.Equal(ErrorServerError.Code, err.Code)
}

// TestRegisterClient_WithLocalizedVariants tests that localized fields are persisted and echoed,
// and that the non-tagged default is stored under SystemLanguage.
func (s *DCRServiceTestSuite) TestRegisterClient_WithLocalizedVariants() {
	mockI18n := i18nmock.NewI18nServiceInterfaceMock(s.T())
	svc := newDCRService(s.mockAppService, s.mockOUService, mockI18n, &MockTransactioner{})

	request := &DCRRegistrationRequest{
		OUID:                "test-ou-1",
		ClientName:          "Test Client",
		LocalizedClientName: map[string]string{"fr": "Client FR", "de": "Client DE"},
		LocalizedLogoURI:    map[string]string{"fr": "https://example.fr/logo.png"},
	}

	appDTO := &model.ApplicationDTO{
		ID:   "app-id",
		Name: "Test Client",
		InboundAuthConfig: []inboundmodel.InboundAuthConfigWithSecret{
			{
				Type: inboundmodel.OAuthInboundAuthType,
				OAuthConfig: &inboundmodel.OAuthConfigWithSecret{
					ClientID:     "client-id",
					ClientSecret: "client-secret",
					Scopes:       []string{},
				},
			},
		},
	}

	s.mockAppService.On(
		"CreateApplication", mock.Anything, mock.AnythingOfType("*model.ApplicationDTO"),
	).Return(appDTO, (*serviceerror.ServiceError)(nil))

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
	).Return((*serviceerror.ServiceError)(nil))

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
	svc := newDCRService(s.mockAppService, s.mockOUService, mockI18n, &MockTransactioner{})

	request := &DCRRegistrationRequest{
		OUID:       "test-ou-1",
		ClientName: "My App",
	}

	appDTO := &model.ApplicationDTO{
		ID:   "app-id",
		Name: "My App",
		InboundAuthConfig: []inboundmodel.InboundAuthConfigWithSecret{
			{
				Type: inboundmodel.OAuthInboundAuthType,
				OAuthConfig: &inboundmodel.OAuthConfigWithSecret{
					ClientID: "client-id",
					Scopes:   []string{},
				},
			},
		},
	}

	s.mockAppService.On(
		"CreateApplication", mock.Anything, mock.AnythingOfType("*model.ApplicationDTO"),
	).Return(appDTO, (*serviceerror.ServiceError)(nil))

	mockI18n.On(
		"SetTranslationOverridesForNamespace",
		mock.Anything,
		application.AppI18nNamespace(),
		mock.MatchedBy(func(entries map[string]map[string]string) bool {
			nameKey := application.AppI18nKey("app-id", "name")
			return entries[nameKey][i18nmgt.SystemLanguage] == "My App"
		}),
	).Return((*serviceerror.ServiceError)(nil))

	response, err := svc.RegisterClient(context.Background(), request)

	s.NotNil(response)
	s.Nil(err)
}

// TestRegisterClient_TaggedSystemLanguageWinsOverDefault verifies that when both the non-tagged
// default and an explicit #SystemLanguage-tagged variant are provided, the tagged variant wins.
func (s *DCRServiceTestSuite) TestRegisterClient_TaggedSystemLanguageWinsOverDefault() {
	mockI18n := i18nmock.NewI18nServiceInterfaceMock(s.T())
	svc := newDCRService(s.mockAppService, s.mockOUService, mockI18n, &MockTransactioner{})

	request := &DCRRegistrationRequest{
		OUID:                "test-ou-1",
		ClientName:          "My App",
		LocalizedClientName: map[string]string{i18nmgt.SystemLanguage: "My App US"},
	}

	appDTO := &model.ApplicationDTO{
		ID:   "app-id",
		Name: "My App",
		InboundAuthConfig: []inboundmodel.InboundAuthConfigWithSecret{
			{
				Type: inboundmodel.OAuthInboundAuthType,
				OAuthConfig: &inboundmodel.OAuthConfigWithSecret{
					ClientID: "client-id",
					Scopes:   []string{},
				},
			},
		},
	}

	s.mockAppService.On(
		"CreateApplication", mock.Anything, mock.AnythingOfType("*model.ApplicationDTO"),
	).Return(appDTO, (*serviceerror.ServiceError)(nil))

	mockI18n.On(
		"SetTranslationOverridesForNamespace",
		mock.Anything,
		application.AppI18nNamespace(),
		mock.MatchedBy(func(entries map[string]map[string]string) bool {
			nameKey := application.AppI18nKey("app-id", "name")
			// Tagged variant wins — must be "My App US", not "My App".
			return entries[nameKey][i18nmgt.SystemLanguage] == "My App US"
		}),
	).Return((*serviceerror.ServiceError)(nil))

	response, err := svc.RegisterClient(context.Background(), request)

	s.NotNil(response)
	s.Nil(err)
	s.Equal(map[string]string{i18nmgt.SystemLanguage: "My App US"}, response.LocalizedClientName)
}

// TestRegisterClient_LocalizedVariantsWriteFailure tests that a failed i18n write triggers
// partial-row cleanup and app compensation delete.
func (s *DCRServiceTestSuite) TestRegisterClient_LocalizedVariantsWriteFailure() {
	mockI18n := i18nmock.NewI18nServiceInterfaceMock(s.T())
	svc := newDCRService(s.mockAppService, s.mockOUService, mockI18n, &MockTransactioner{})

	request := &DCRRegistrationRequest{
		OUID:                "test-ou-1",
		ClientName:          "Test Client",
		LocalizedClientName: map[string]string{"fr": "Client FR"},
	}

	appDTO := &model.ApplicationDTO{
		ID:   "app-id",
		Name: "Test Client",
		InboundAuthConfig: []inboundmodel.InboundAuthConfigWithSecret{
			{
				Type: inboundmodel.OAuthInboundAuthType,
				OAuthConfig: &inboundmodel.OAuthConfigWithSecret{
					ClientID: "client-id",
					Scopes:   []string{},
				},
			},
		},
	}

	i18nErr := &serviceerror.ServiceError{Code: "I18N-500"}

	s.mockAppService.On(
		"CreateApplication", mock.Anything, mock.AnythingOfType("*model.ApplicationDTO"),
	).Return(appDTO, (*serviceerror.ServiceError)(nil))
	mockI18n.On(
		"SetTranslationOverridesForNamespace",
		mock.Anything,
		application.AppI18nNamespace(),
		mock.Anything,
	).Return(i18nErr)
	mockI18n.On("DeleteTranslationsByKey", mock.Anything, application.AppI18nNamespace(), mock.Anything).
		Return((*serviceerror.ServiceError)(nil))
	s.mockAppService.On("DeleteApplication", mock.Anything, "app-id").
		Return((*serviceerror.ServiceError)(nil))

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
	svc := newDCRService(s.mockAppService, s.mockOUService, mockI18n, &MockTransactioner{})

	request := &DCRRegistrationRequest{
		OUID:             "test-ou-1",
		ClientName:       "Test Client",
		LocalizedLogoURI: map[string]string{"fr": "not-a-valid-uri"},
	}

	appDTO := &model.ApplicationDTO{
		ID:   "app-id",
		Name: "Test Client",
		InboundAuthConfig: []inboundmodel.InboundAuthConfigWithSecret{
			{
				Type: inboundmodel.OAuthInboundAuthType,
				OAuthConfig: &inboundmodel.OAuthConfigWithSecret{
					ClientID: "client-id",
					Scopes:   []string{},
				},
			},
		},
	}

	s.mockAppService.On(
		"CreateApplication", mock.Anything, mock.AnythingOfType("*model.ApplicationDTO"),
	).Return(appDTO, (*serviceerror.ServiceError)(nil))
	// URI validation fails before any i18n writes; compensation still runs.
	mockI18n.On("DeleteTranslationsByKey", mock.Anything, application.AppI18nNamespace(), mock.Anything).
		Return((*serviceerror.ServiceError)(nil))
	s.mockAppService.On("DeleteApplication", mock.Anything, "app-id").
		Return((*serviceerror.ServiceError)(nil))

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
		InboundAuthConfig: []inboundmodel.InboundAuthConfigWithSecret{
			{
				Type: inboundmodel.OAuthInboundAuthType,
				OAuthConfig: &inboundmodel.OAuthConfigWithSecret{
					ClientID: "client-id",
					Scopes:   []string{"openid"},
					Token: &inboundmodel.OAuthTokenConfig{
						IDToken: &inboundmodel.IDTokenConfig{
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
	).Return(appDTO, (*serviceerror.ServiceError)(nil))

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
	svc := newDCRService(s.mockAppService, s.mockOUService, mockI18n, &MockTransactioner{})

	request := &DCRRegistrationRequest{
		OUID:                "test-ou-1",
		ClientName:          "Test Client",
		LocalizedClientName: map[string]string{"fr": "Client FR"},
	}

	appDTO := &model.ApplicationDTO{
		ID:   "app-id",
		Name: "Test Client",
		InboundAuthConfig: []inboundmodel.InboundAuthConfigWithSecret{
			{
				Type: inboundmodel.OAuthInboundAuthType,
				OAuthConfig: &inboundmodel.OAuthConfigWithSecret{
					ClientID: "client-id",
					Scopes:   []string{},
				},
			},
		},
	}

	i18nClientErr := &serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "I18N-4001",
	}

	s.mockAppService.On(
		"CreateApplication", mock.Anything, mock.AnythingOfType("*model.ApplicationDTO"),
	).Return(appDTO, (*serviceerror.ServiceError)(nil))
	mockI18n.On(
		"SetTranslationOverridesForNamespace",
		mock.Anything,
		application.AppI18nNamespace(),
		mock.Anything,
	).Return(i18nClientErr)
	mockI18n.On("DeleteTranslationsByKey", mock.Anything, application.AppI18nNamespace(), mock.Anything).
		Return((*serviceerror.ServiceError)(nil))
	s.mockAppService.On("DeleteApplication", mock.Anything, "app-id").
		Return((*serviceerror.ServiceError)(nil))

	response, err := svc.RegisterClient(context.Background(), request)

	s.Nil(response)
	s.NotNil(err)
	s.Equal(ErrorServerError.Code, err.Code)
	mockI18n.AssertExpectations(s.T())
	s.mockAppService.AssertExpectations(s.T())
}
