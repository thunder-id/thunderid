// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package oauth

import (
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/authn/common"
	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/tests/mocks/httpmock"
	"github.com/thunder-id/thunderid/tests/mocks/idp/idpmock"
)

const (
	testIDPID         = "idp123"
	testSub           = "user_sub_123"
	testAuthURL       = "https://idp.com/authorize?client_id=test_client&redirect_uri=https%3A%2F%2Fapp.com%2Fcallback&response_type=code&scope=openid&state=random_state" //nolint:lll
	testTokenRespJSON = `{"access_token":"access123","token_type":"Bearer"}`
	testUserID        = "user123"
)

type OAuthAuthnServiceTestSuite struct {
	suite.Suite
	mockHTTPClient *httpmock.HTTPClientInterfaceMock
	mockIDPService *idpmock.IDPServiceInterfaceMock
	service        OAuthAuthnServiceInterface
	endpoints      OAuthEndpoints
}

func TestOAuthAuthnServiceTestSuite(t *testing.T) {
	suite.Run(t, new(OAuthAuthnServiceTestSuite))
}

func (suite *OAuthAuthnServiceTestSuite) SetupTest() {
	suite.mockHTTPClient = httpmock.NewHTTPClientInterfaceMock(suite.T())
	suite.mockIDPService = idpmock.NewIDPServiceInterfaceMock(suite.T())
	suite.endpoints = OAuthEndpoints{
		AuthorizationEndpoint: "https://localhost:8090/oauth/authorize",
		TokenEndpoint:         "https://localhost:8090/oauth/token",
		UserInfoEndpoint:      "https://localhost:8090/oauth/userinfo",
	}
	// Use the constructor to properly initialize the service including logger
	suite.service = newOAuthAuthnService(suite.mockHTTPClient, suite.mockIDPService)
}

func createTestIDPDTO() *providers.IDPDTO {
	clientIDProp, _ := cmodels.NewProperty("client_id", "test_client", false)
	clientSecretProp, _ := cmodels.NewProperty("client_secret", "test_secret", false)
	redirectURIProp, _ := cmodels.NewProperty("redirect_uri", "https://app.com/callback", false)
	scopesProp, _ := cmodels.NewProperty("scopes", "openid", false)
	tokenEndpointProp, _ := cmodels.NewProperty("token_endpoint", "https://idp.com/token", false)

	return &providers.IDPDTO{
		ID:   testIDPID,
		Name: "Test IDP",
		Type: providers.IDPTypeOAuth,
		Properties: []cmodels.Property{
			*clientIDProp, *clientSecretProp, *redirectURIProp, *scopesProp, *tokenEndpointProp,
		},
	}
}

func (suite *OAuthAuthnServiceTestSuite) TestGetOAuthClientConfigSuccess() {
	clientIDProp, _ := cmodels.NewProperty("client_id", "test_client_id", false)
	clientSecretProp, _ := cmodels.NewProperty("client_secret", "test_client_secret", false)
	redirectURIProp, _ := cmodels.NewProperty("redirect_uri", "https://app.example.com/callback", false)
	scopesProp, _ := cmodels.NewProperty("scopes", "openid profile email", false)
	authzEndpointProp, _ := cmodels.NewProperty("authorization_endpoint", "https://localhost:8090/authorize", false)
	tokenEndpointProp, _ := cmodels.NewProperty("token_endpoint", "https://localhost:8090/token", false)

	idpDTO := &providers.IDPDTO{
		ID:   testIDPID,
		Name: "Test OAuth Provider",
		Type: providers.IDPTypeOAuth,
		Properties: []cmodels.Property{
			*clientIDProp,
			*clientSecretProp,
			*redirectURIProp,
			*scopesProp,
			*authzEndpointProp,
			*tokenEndpointProp,
		},
	}
	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).Return(idpDTO, nil)

	config, err := suite.service.GetOAuthClientConfig(context.Background(), testIDPID)
	suite.Nil(err)
	suite.NotNil(config)
	suite.Equal("test_client_id", config.ClientID)
	suite.Equal("test_client_secret", config.ClientSecret)
	suite.Equal("https://app.example.com/callback", config.RedirectURI)
	suite.Equal([]string{"openid", "profile", "email"}, config.Scopes)
	suite.Equal("https://localhost:8090/authorize", config.OAuthEndpoints.AuthorizationEndpoint)
	suite.Equal("https://localhost:8090/token", config.OAuthEndpoints.TokenEndpoint)
}

func (suite *OAuthAuthnServiceTestSuite) TestGetOAuthClientConfigWithError() {
	tests := []struct {
		name            string
		idpID           string
		mockSetup       func(m *idpmock.IDPServiceInterfaceMock)
		expectedErrCode string
	}{
		{
			name:            "EmptyIdpID",
			idpID:           "",
			mockSetup:       nil,
			expectedErrCode: ErrorEmptyIdpID.Code,
		},
		{
			name:  "IdpNotFound",
			idpID: testIDPID,
			mockSetup: func(m *idpmock.IDPServiceInterfaceMock) {
				clientErr := &tidcommon.ServiceError{
					Type: tidcommon.ClientErrorType,
					Code: "IDP_NOT_FOUND",
					ErrorDescription: tidcommon.I18nMessage{
						Key: "error.test.identity_provider_not_found", DefaultValue: "Identity provider not found",
					},
				}
				m.On("GetIdentityProvider", mock.Anything, testIDPID).Return(nil, clientErr)
			},
			expectedErrCode: ErrorClientErrorWhileRetrievingIDP.Code,
		},
		{
			name:  "ServerError",
			idpID: testIDPID,
			mockSetup: func(m *idpmock.IDPServiceInterfaceMock) {
				serverErr := &tidcommon.ServiceError{
					Type: tidcommon.ServerErrorType,
					Code: "INTERNAL_ERROR",
					ErrorDescription: tidcommon.I18nMessage{
						Key: "error.test.database_unavailable", DefaultValue: "Database unavailable",
					},
				}
				m.On("GetIdentityProvider", mock.Anything, testIDPID).Return(nil, serverErr)
			},
			expectedErrCode: tidcommon.InternalServerError.Code,
		},
	}

	for _, tc := range tests {
		suite.Run(tc.name, func() {
			freshIDPMock := idpmock.NewIDPServiceInterfaceMock(suite.T())
			if svcImpl, ok := suite.service.(*oAuthAuthnService); ok {
				svcImpl.idpService = freshIDPMock
			}

			if tc.mockSetup != nil {
				tc.mockSetup(freshIDPMock)
			}

			config, err := suite.service.GetOAuthClientConfig(context.Background(), tc.idpID)
			suite.Nil(config)
			suite.NotNil(err)
			suite.Equal(tc.expectedErrCode, err.Code)
		})
	}
}

func (suite *OAuthAuthnServiceTestSuite) TestBuildAuthorizeURLSuccess() {
	clientIDProp, _ := cmodels.NewProperty("client_id", "test_client_id", false)
	clientSecretProp, _ := cmodels.NewProperty("client_secret", "test_client_secret", false)
	redirectURIProp, _ := cmodels.NewProperty("redirect_uri", "https://app.example.com/callback", false)
	scopesProp, _ := cmodels.NewProperty("scopes", "openid profile", false)
	authzEndpointProp, _ := cmodels.NewProperty("authorization_endpoint", "https://example.com/oauth/authorize", false)

	idpDTO := &providers.IDPDTO{
		ID:   testIDPID,
		Name: "Test OAuth Provider",
		Type: providers.IDPTypeOAuth,
		Properties: []cmodels.Property{
			*clientIDProp, *clientSecretProp, *redirectURIProp, *scopesProp, *authzEndpointProp,
		},
	}
	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).Return(idpDTO, nil)

	url, metadata, err := suite.service.BuildAuthorizeURL(context.Background(), testIDPID)
	suite.Nil(err)
	suite.NotNil(url)
	suite.Contains(url, "https://example.com/oauth/authorize?")
	suite.Contains(url, "response_type=code")
	suite.Contains(url, "client_id=test_client_id")
	suite.Contains(url, "redirect_uri=https%3A%2F%2Fapp.example.com%2Fcallback")
	suite.Contains(url, "scope=openid+profile")
	suite.Contains(url, "state=")
	suite.NotEmpty(metadata[oauth2const.RequestParamState])
}

func (suite *OAuthAuthnServiceTestSuite) TestBuildAuthorizeURLSuccessWithAdditionalParams() {
	tests := []struct {
		name         string
		extraProp    *cmodels.Property
		forbiddenStr string
	}{
		{
			name: "EmptyKey",
			extraProp: func() *cmodels.Property {
				p, _ := cmodels.NewProperty("", "should_be_ignored", false)
				return p
			}(),
			forbiddenStr: "should_be_ignored",
		},
		{
			name: "EmptyValue",
			extraProp: func() *cmodels.Property {
				p, _ := cmodels.NewProperty("custom_param", "", false)
				return p
			}(),
			forbiddenStr: "custom_param",
		},
	}

	for _, tc := range tests {
		suite.Run(tc.name, func() {
			if svcImpl, ok := suite.service.(*oAuthAuthnService); ok {
				svcImpl.idpService = suite.mockIDPService
			}

			clientIDProp, _ := cmodels.NewProperty("client_id", "test_client_id", false)
			clientSecretProp, _ := cmodels.NewProperty("client_secret", "test_client_secret", false)
			redirectURIProp, _ := cmodels.NewProperty("redirect_uri", "https://app.example.com/callback", false)
			scopesProp, _ := cmodels.NewProperty("scopes", "openid profile", false)
			authzEndpointProp, _ := cmodels.NewProperty("authorization_endpoint",
				"https://example.com/oauth/authorize", false)

			idpDTO := &providers.IDPDTO{
				ID:   testIDPID,
				Name: "Test OAuth Provider",
				Type: providers.IDPTypeOAuth,
				Properties: []cmodels.Property{
					*clientIDProp,
					*clientSecretProp,
					*redirectURIProp,
					*scopesProp,
					*authzEndpointProp,
					*tc.extraProp,
				},
			}
			suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).Return(idpDTO, nil)

			url, metadata, err := suite.service.BuildAuthorizeURL(context.Background(), testIDPID)
			suite.Nil(err)
			suite.NotNil(url)
			suite.Contains(url, "https://example.com/oauth/authorize?")
			suite.Contains(url, "client_id=test_client_id")
			suite.Contains(url, "redirect_uri=https%3A%2F%2Fapp.example.com%2Fcallback")
			suite.Contains(url, "state=")
			suite.NotEmpty(metadata[oauth2const.RequestParamState])

			// Ensure the forbidden string (empty-key value or empty-value key) is not present in the URL
			suite.NotContains(url, tc.forbiddenStr)
		})
	}
}

func (suite *OAuthAuthnServiceTestSuite) TestBuildAuthorizeURLWithError() {
	if svcImpl, ok := suite.service.(*oAuthAuthnService); ok {
		svcImpl.idpService = suite.mockIDPService
	}

	serverErr := &tidcommon.ServiceError{
		Type: tidcommon.ServerErrorType,
		Code: "INTERNAL_ERROR",
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.test.database_unavailable", DefaultValue: "Database unavailable",
		},
	}
	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).Return(nil, serverErr)

	url, metadata, err := suite.service.BuildAuthorizeURL(context.Background(), testIDPID)
	suite.Empty(url)
	suite.Nil(metadata)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *OAuthAuthnServiceTestSuite) TestExchangeCodeForTokenEmptyCode() {
	tokenResp, err := suite.service.ExchangeCodeForToken(context.Background(), testIDPID, "", false)
	suite.Nil(tokenResp)
	suite.NotNil(err)
	suite.Equal(ErrorEmptyAuthorizationCode.Code, err.Code)
}

func (suite *OAuthAuthnServiceTestSuite) TestExchangeCodeForTokenSuccess() {
	testCases := []struct {
		name             string
		code             string
		validateResponse bool
	}{
		{
			name:             "WithoutValidation",
			code:             "valid_auth_code",
			validateResponse: false,
		},
		{
			name:             "WithValidation",
			code:             "valid_auth_code",
			validateResponse: true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			clientIDProp, _ := cmodels.NewProperty("client_id", "test_client", false)
			clientSecretProp, _ := cmodels.NewProperty("client_secret", "test_secret", false)
			redirectURIProp, _ := cmodels.NewProperty("redirect_uri", "https://app.com/callback", false)
			scopesProp, _ := cmodels.NewProperty("scopes", "openid,profile", false)
			tokenEndpointProp, _ := cmodels.NewProperty(
				"token_endpoint", "https://idp.com/token", false)

			idpData := &providers.IDPDTO{
				ID:   testIDPID,
				Name: "Test IDP",
				Type: providers.IDPTypeOAuth,
				Properties: []cmodels.Property{
					*clientIDProp, *clientSecretProp, *redirectURIProp, *scopesProp, *tokenEndpointProp,
				},
			}

			tokenRespJSON := testTokenRespJSON
			resp := &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewReader([]byte(tokenRespJSON))),
			}

			suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).Return(idpData, nil).Once()
			suite.mockHTTPClient.On("Do", mock.Anything).Return(resp, nil).Once()

			result, err := suite.service.ExchangeCodeForToken(
				context.Background(), testIDPID, tc.code, tc.validateResponse)
			suite.Nil(err)
			suite.NotNil(result)
			suite.Equal("access123", result.AccessToken)
		})
	}
}

func (suite *OAuthAuthnServiceTestSuite) TestExchangeCodeForTokenWithFailure() {
	testCases := []struct {
		name          string
		setupMocks    func()
		expectedError string
	}{
		{
			name: "IDPNotFound",
			setupMocks: func() {
				svcErr := &tidcommon.ServiceError{
					Code: "IDP-001",
					Type: tidcommon.ClientErrorType,
				}
				suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).
					Return(nil, svcErr).Once()
			},
			expectedError: ErrorClientErrorWhileRetrievingIDP.Code,
		},
		{
			name: "HTTPRequestFailure",
			setupMocks: func() {
				clientIDProp, _ := cmodels.NewProperty("client_id", "test_client", false)
				clientSecretProp, _ := cmodels.NewProperty("client_secret", "test_secret", false)
				redirectURIProp, _ := cmodels.NewProperty("redirect_uri", "https://app.com/callback", false)
				scopesProp, _ := cmodels.NewProperty("scopes", "openid", false)
				tokenEndpointProp, _ := cmodels.NewProperty(
					"token_endpoint", "https://idp.com/token", false)

				idpData := &providers.IDPDTO{
					ID:   testIDPID,
					Name: "Test IDP",
					Type: providers.IDPTypeOAuth,
					Properties: []cmodels.Property{
						*clientIDProp, *clientSecretProp, *redirectURIProp, *scopesProp, *tokenEndpointProp,
					},
				}

				suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).Return(idpData, nil).Once()
				suite.mockHTTPClient.On("Do", mock.Anything).
					Return(nil, errors.New("network error")).Once()
			},
			expectedError: tidcommon.InternalServerError.Code,
		},
		{
			name: "Non200StatusCode",
			setupMocks: func() {
				idpData := createTestIDPDTO()
				resp := &http.Response{
					StatusCode: 401,
					Body:       io.NopCloser(bytes.NewReader([]byte(`{"error":"invalid_grant"}`))),
				}

				suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).Return(idpData, nil).Once()
				suite.mockHTTPClient.On("Do", mock.Anything).Return(resp, nil).Once()
			},
			expectedError: tidcommon.InternalServerError.Code,
		},
		{
			name: "InvalidJSONResponse",
			setupMocks: func() {
				idpData := createTestIDPDTO()
				resp := &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewReader([]byte(`invalid json`))),
				}

				suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).Return(idpData, nil).Once()
				suite.mockHTTPClient.On("Do", mock.Anything).Return(resp, nil).Once()
			},
			expectedError: tidcommon.InternalServerError.Code,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			tc.setupMocks()

			result, err := suite.service.ExchangeCodeForToken(context.Background(), testIDPID, "auth_code", false)
			suite.Nil(result)
			suite.NotNil(err)
			suite.Equal(tc.expectedError, err.Code)
		})
	}
}

func (suite *OAuthAuthnServiceTestSuite) TestFetchUserInfoEmptyAccessToken() {
	clientIDProp, _ := cmodels.NewProperty("client_id", "test_client_id", false)
	clientSecretProp, _ := cmodels.NewProperty("client_secret", "client_secret_value", false)
	redirectURIProp, _ := cmodels.NewProperty("redirect_uri", "https://app.example.com/callback", false)
	scopesProp, _ := cmodels.NewProperty("scopes", "openid profile", false)

	idpDTO := &providers.IDPDTO{
		ID:   testIDPID,
		Name: "Test OAuth Provider",
		Type: providers.IDPTypeOAuth,
		Properties: []cmodels.Property{
			*clientIDProp, *clientSecretProp, *redirectURIProp, *scopesProp,
		},
	}
	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).Return(idpDTO, nil)

	userInfo, err := suite.service.FetchUserInfo(context.Background(), testIDPID, "")
	suite.Nil(userInfo)
	suite.NotNil(err)
	suite.Equal(ErrorEmptyAccessToken.Code, err.Code)
}

func (suite *OAuthAuthnServiceTestSuite) TestFetchUserInfoWithClientConfigSuccess() {
	accessToken := "access_token_123"
	userInfoMap := map[string]interface{}{
		"sub":   "user_sub_123",
		"email": "user@example.com",
		"name":  "Test User",
	}
	userInfoJSON, _ := json.Marshal(userInfoMap)

	config := &OAuthClientConfig{
		ClientID:     "test_client_id",
		ClientSecret: "test_client_secret",
		RedirectURI:  "https://app.example.com/callback",
		Scopes:       []string{"openid", "profile"},
		OAuthEndpoints: OAuthEndpoints{
			UserInfoEndpoint: "https://localhost:8090/userinfo",
		},
	}

	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader(userInfoJSON)),
	}

	suite.mockHTTPClient.On("Do", mock.Anything).Return(resp, nil)

	userInfo, err := suite.service.FetchUserInfoWithClientConfig(context.Background(), config, accessToken)
	suite.Nil(err)
	suite.NotNil(userInfo)
	suite.Equal("user_sub_123", userInfo["sub"])
}

func (suite *OAuthAuthnServiceTestSuite) TestFetchUserInfoWithClientConfigEmptyAccessToken() {
	config := &OAuthClientConfig{
		ClientID:     "test_client_id",
		ClientSecret: "test_client_secret",
		OAuthEndpoints: OAuthEndpoints{
			UserInfoEndpoint: "https://localhost:8090/userinfo",
		},
	}

	userInfo, err := suite.service.FetchUserInfoWithClientConfig(context.Background(), config, "")
	suite.Nil(userInfo)
	suite.NotNil(err)
	suite.Equal(ErrorEmptyAccessToken.Code, err.Code)
}

func (suite *OAuthAuthnServiceTestSuite) TestValidateTokenResponseSuccess() {
	tokenResp := &TokenResponse{
		AccessToken: "access_token_123",
		TokenType:   "Bearer",
		ExpiresIn:   3600,
	}

	err := suite.service.ValidateTokenResponse(context.Background(), testIDPID, tokenResp)
	suite.Nil(err)
}

func (suite *OAuthAuthnServiceTestSuite) TestValidateTokenResponseWithError() {
	tests := []struct {
		name string
		resp *TokenResponse
	}{
		{
			name: "NilResponse",
			resp: nil,
		},
		{
			name: "EmptyAccessToken",
			resp: &TokenResponse{
				AccessToken: "",
				TokenType:   "Bearer",
			},
		},
	}

	for _, tc := range tests {
		suite.Run(tc.name, func() {
			err := suite.service.ValidateTokenResponse(context.Background(), testIDPID, tc.resp)
			suite.NotNil(err)
			suite.Equal(ErrorInvalidTokenResponse.Code, err.Code)
		})
	}
}

func (suite *OAuthAuthnServiceTestSuite) TestBuildAuthorizeURLErrors() {
	tests := []struct {
		name          string
		authzEndpoint string
	}{
		{name: "URIError", authzEndpoint: "://invalid-url"},
		{name: "MissingAuthorizationEndpoint", authzEndpoint: ""},
	}

	for _, tc := range tests {
		suite.Run(tc.name, func() {
			clientIDProp, _ := cmodels.NewProperty("client_id", "test_client", false)
			clientSecretProp, _ := cmodels.NewProperty("client_secret", "test_secret", false)
			redirectURIProp, _ := cmodels.NewProperty("redirect_uri", "https://app.com/callback", false)
			scopesProp, _ := cmodels.NewProperty("scopes", "openid", false)
			authzEndpointProp, _ := cmodels.NewProperty("authorization_endpoint", tc.authzEndpoint, false)

			idpDTO := &providers.IDPDTO{
				ID:   testIDPID,
				Name: "Test OAuth Provider",
				Type: providers.IDPTypeOAuth,
				Properties: []cmodels.Property{
					*clientIDProp, *clientSecretProp, *redirectURIProp, *scopesProp, *authzEndpointProp,
				},
			}
			suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).Return(idpDTO, nil)

			url, metadata, err := suite.service.BuildAuthorizeURL(context.Background(), testIDPID)
			suite.Empty(url)
			suite.Nil(metadata)
			suite.NotNil(err)
			suite.Equal(tidcommon.InternalServerError.Code, err.Code)
		})
	}
}

func (suite *OAuthAuthnServiceTestSuite) TestExchangeCodeForTokenWithValidationFailure() {
	// Prepare IDP data
	clientIDProp, _ := cmodels.NewProperty("client_id", "test_client", false)
	clientSecretProp, _ := cmodels.NewProperty("client_secret", "test_secret", false)
	redirectURIProp, _ := cmodels.NewProperty("redirect_uri", "https://app.com/callback", false)
	scopesProp, _ := cmodels.NewProperty("scopes", "openid", false)
	tokenEndpointProp, _ := cmodels.NewProperty("token_endpoint", "https://idp.com/token", false)

	idpData := &providers.IDPDTO{
		ID:   testIDPID,
		Name: "Test IDP",
		Type: providers.IDPTypeOAuth,
		Properties: []cmodels.Property{
			*clientIDProp, *clientSecretProp, *redirectURIProp, *scopesProp, *tokenEndpointProp,
		},
	}

	// token response with empty access_token to force validation failure
	tokenRespJSON := `{"access_token":"","token_type":"Bearer"}`
	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader([]byte(tokenRespJSON))),
	}

	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).Return(idpData, nil).Once()
	suite.mockHTTPClient.On("Do", mock.Anything).Return(resp, nil).Once()

	result, err := suite.service.ExchangeCodeForToken(context.Background(), testIDPID, "code123", true)
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidTokenResponse.Code, err.Code)
}

func (suite *OAuthAuthnServiceTestSuite) TestFetchUserInfoWithClientConfigMissingEndpoint() {
	config := &OAuthClientConfig{
		ClientID:     "test_client_id",
		ClientSecret: "test_client_secret",
		RedirectURI:  "https://app.example.com/callback",
		Scopes:       []string{"openid", "profile"},
		OAuthEndpoints: OAuthEndpoints{
			UserInfoEndpoint: "",
		},
	}

	userInfo, err := suite.service.FetchUserInfoWithClientConfig(context.Background(), config, "access_token")
	suite.Nil(userInfo)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *OAuthAuthnServiceTestSuite) TestExchangeCodeForTokenMissingTokenEndpoint() {
	clientIDProp, _ := cmodels.NewProperty("client_id", "test_client", false)
	clientSecretProp, _ := cmodels.NewProperty("client_secret", "test_secret", false)
	redirectURIProp, _ := cmodels.NewProperty("redirect_uri", "https://app.com/callback", false)
	scopesProp, _ := cmodels.NewProperty("scopes", "openid", false)
	tokenEndpointProp, _ := cmodels.NewProperty("token_endpoint", "", false)

	idpDTO := &providers.IDPDTO{
		ID:   testIDPID,
		Name: "Test OAuth Provider",
		Type: providers.IDPTypeOAuth,
		Properties: []cmodels.Property{
			*clientIDProp, *clientSecretProp, *redirectURIProp, *scopesProp, *tokenEndpointProp,
		},
	}
	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).Return(idpDTO, nil)

	token, err := suite.service.ExchangeCodeForToken(context.Background(), testIDPID, "code123", false)
	suite.Nil(token)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *OAuthAuthnServiceTestSuite) TestFetchUserInfoMissingUserInfoEndpoint() {
	clientIDProp, _ := cmodels.NewProperty("client_id", "test_client", false)
	clientSecretProp, _ := cmodels.NewProperty("client_secret", "test_secret", false)
	redirectURIProp, _ := cmodels.NewProperty("redirect_uri", "https://app.com/callback", false)
	scopesProp, _ := cmodels.NewProperty("scopes", "openid", false)
	userInfoEndpointProp, _ := cmodels.NewProperty("userinfo_endpoint", "", false)

	idpDTO := &providers.IDPDTO{
		ID:   testIDPID,
		Name: "Test OAuth Provider",
		Type: providers.IDPTypeOAuth,
		Properties: []cmodels.Property{
			*clientIDProp, *clientSecretProp, *redirectURIProp, *scopesProp, *userInfoEndpointProp,
		},
	}
	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).Return(idpDTO, nil)

	userInfo, err := suite.service.FetchUserInfo(context.Background(), testIDPID, "access_token")
	suite.Nil(userInfo)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *OAuthAuthnServiceTestSuite) TestAuthenticateSuccess() {
	clientIDProp, _ := cmodels.NewProperty("client_id", "test_client", false)
	clientSecretProp, _ := cmodels.NewProperty("client_secret", "test_secret", false)
	redirectURIProp, _ := cmodels.NewProperty("redirect_uri", "https://app.com/callback", false)
	scopesProp, _ := cmodels.NewProperty("scopes", "openid", false)
	tokenEndpointProp, _ := cmodels.NewProperty("token_endpoint", "https://idp.com/token", false)
	userInfoEndpointProp, _ := cmodels.NewProperty("userinfo_endpoint", "https://idp.com/userinfo", false)

	idpDTO := &providers.IDPDTO{
		ID:   testIDPID,
		Name: "Test IDP",
		Type: providers.IDPTypeOAuth,
		Properties: []cmodels.Property{
			*clientIDProp, *clientSecretProp, *redirectURIProp, *scopesProp, *tokenEndpointProp, *userInfoEndpointProp,
		},
	}

	tokenRespJSON := testTokenRespJSON
	tokenHTTPResp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader([]byte(tokenRespJSON))),
	}

	userInfoMap := map[string]interface{}{
		"sub":   "user_sub_123",
		"email": "user@example.com",
	}
	userInfoJSON, _ := json.Marshal(userInfoMap)
	userInfoHTTPResp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader(userInfoJSON)),
	}

	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).Return(idpDTO, nil)
	suite.mockHTTPClient.On("Do", mock.Anything).Return(tokenHTTPResp, nil).Once()
	suite.mockHTTPClient.On("Do", mock.Anything).Return(userInfoHTTPResp, nil).Once()

	result, err := suite.service.Authenticate(context.Background(), testIDPID,
		common.AuthorizationData{Code: "auth_code"})
	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal("user_sub_123", result.Token["sub"])
	suite.Equal("user@example.com", result.AuthenticatedClaims["email"])
}

func (suite *OAuthAuthnServiceTestSuite) TestAuthenticateTokenExchangeFailure() {
	result, err := suite.service.Authenticate(context.Background(), testIDPID, common.AuthorizationData{})
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorEmptyAuthorizationCode.Code, err.Code)
}

// oauthIDPProperties builds a generic OAuth IDP. An empty endpoint leaves the property out entirely,
// as the connection API does for an omitted value.
func oauthIDPProperties(userInfoEndpoint string) *providers.IDPDTO {
	clientIDProp, _ := cmodels.NewProperty("client_id", "test_client", false)
	clientSecretProp, _ := cmodels.NewProperty("client_secret", "test_secret", false)
	redirectURIProp, _ := cmodels.NewProperty("redirect_uri", "https://app.com/callback", false)
	scopesProp, _ := cmodels.NewProperty("scopes", "read", false)
	tokenEndpointProp, _ := cmodels.NewProperty("token_endpoint", "https://idp.com/token", false)

	props := []cmodels.Property{
		*clientIDProp, *clientSecretProp, *redirectURIProp, *scopesProp, *tokenEndpointProp,
	}
	if userInfoEndpoint != "" {
		userInfoEndpointProp, _ := cmodels.NewProperty("userinfo_endpoint", userInfoEndpoint, false)
		props = append(props, *userInfoEndpointProp)
	}

	return &providers.IDPDTO{
		ID:         testIDPID,
		Name:       "Test IDP",
		Type:       providers.IDPTypeOAuth,
		Properties: props,
	}
}

// jwtAccessToken builds an unsigned JWT-shaped access token carrying the given payload.
func jwtAccessToken(payload map[string]interface{}) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"at+jwt"}`))
	body, _ := json.Marshal(payload)
	return header + "." + base64.RawURLEncoding.EncodeToString(body) + ".signature"
}

// tokenHTTPResponse builds a token endpoint response returning the given access token.
func tokenHTTPResponse(accessToken string) *http.Response {
	body, _ := json.Marshal(map[string]string{"access_token": accessToken, "token_type": "Bearer"})
	return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(body))}
}

func (suite *OAuthAuthnServiceTestSuite) TestAuthenticateFetchUserInfoFailure() {
	userInfoHTTPResp := &http.Response{
		StatusCode: 403,
		Body: io.NopCloser(bytes.NewReader([]byte(
			`{"error":"insufficient_scope","error_description":"Access token does not have the openid scope"}`))),
	}

	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).
		Return(oauthIDPProperties("https://idp.com/userinfo"), nil)
	suite.mockHTTPClient.On("Do", mock.Anything).Return(tokenHTTPResponse("access123"), nil).Once()
	suite.mockHTTPClient.On("Do", mock.Anything).Return(userInfoHTTPResp, nil).Once()

	result, err := suite.service.Authenticate(context.Background(), testIDPID,
		common.AuthorizationData{Code: "auth_code"})
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorUserProfileRetrievalFailed.Code, err.Code)
	suite.Equal(tidcommon.ClientErrorType, err.Type)
}

func (suite *OAuthAuthnServiceTestSuite) TestAuthenticateWithoutProfileEndpointUsesAccessTokenSubject() {
	accessToken := jwtAccessToken(map[string]interface{}{
		"sub": testSub, "client_id": "test_client", "scope": "read", "jti": "token-id",
	})

	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).Return(oauthIDPProperties(""), nil)
	suite.mockHTTPClient.On("Do", mock.Anything).Return(tokenHTTPResponse(accessToken), nil).Once()

	result, err := suite.service.Authenticate(context.Background(), testIDPID,
		common.AuthorizationData{Code: "auth_code"})
	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(testSub, result.Token["sub"])
	// Only the subject is taken from the access token; its resource server claims are not user attributes.
	suite.Equal(map[string]interface{}{"sub": testSub}, result.AuthenticatedClaims)
}

func (suite *OAuthAuthnServiceTestSuite) TestAuthenticateWithoutProfileEndpointFailures() {
	tests := []struct {
		name        string
		accessToken string
	}{
		{
			name:        "OpaqueAccessToken",
			accessToken: "access123",
		},
		{
			name:        "JWTAccessTokenWithoutSub",
			accessToken: jwtAccessToken(map[string]interface{}{"client_id": "test_client", "scope": "read"}),
		},
		{
			name:        "JWTAccessTokenWithNumericSub",
			accessToken: jwtAccessToken(map[string]interface{}{"sub": 1}),
		},
		{
			name:        "JWTAccessTokenWithBooleanSub",
			accessToken: jwtAccessToken(map[string]interface{}{"sub": true}),
		},
		{
			name:        "JWTAccessTokenWithEmptySub",
			accessToken: jwtAccessToken(map[string]interface{}{"sub": ""}),
		},
	}

	for _, test := range tests {
		suite.Run(test.name, func() {
			suite.SetupTest()
			suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).
				Return(oauthIDPProperties(""), nil)
			suite.mockHTTPClient.On("Do", mock.Anything).Return(tokenHTTPResponse(test.accessToken), nil).Once()

			result, err := suite.service.Authenticate(context.Background(), testIDPID,
				common.AuthorizationData{Code: "auth_code"})
			suite.Nil(result)
			suite.NotNil(err)
			suite.Equal(ErrorNoUserProfileSource.Code, err.Code)
			suite.Equal(tidcommon.ClientErrorType, err.Type)
		})
	}
}

func (suite *OAuthAuthnServiceTestSuite) TestAuthenticateMissingSub() {
	tests := []struct {
		name     string
		userInfo map[string]interface{}
	}{
		{
			name:     "SubKeyMissing",
			userInfo: map[string]interface{}{"email": "user@example.com"},
		},
		{
			name:     "SubIsNil",
			userInfo: map[string]interface{}{"sub": nil, "email": "user@example.com"},
		},
		{
			name:     "SubIsEmptyString",
			userInfo: map[string]interface{}{"sub": "", "email": "user@example.com"},
		},
		{
			name:     "SubIsNonString",
			userInfo: map[string]interface{}{"sub": 12345, "email": "user@example.com"},
		},
	}

	for _, tc := range tests {
		suite.Run(tc.name, func() {
			freshIDPMock := idpmock.NewIDPServiceInterfaceMock(suite.T())
			freshHTTPMock := httpmock.NewHTTPClientInterfaceMock(suite.T())
			svcImpl := suite.service.(*oAuthAuthnService)
			svcImpl.idpService = freshIDPMock
			svcImpl.httpClient = freshHTTPMock

			clientIDProp, _ := cmodels.NewProperty("client_id", "test_client", false)
			clientSecretProp, _ := cmodels.NewProperty("client_secret", "test_secret", false)
			redirectURIProp, _ := cmodels.NewProperty("redirect_uri", "https://app.com/callback", false)
			scopesProp, _ := cmodels.NewProperty("scopes", "openid", false)
			tokenEndpointProp, _ := cmodels.NewProperty("token_endpoint", "https://idp.com/token", false)
			userInfoEndpointProp, _ := cmodels.NewProperty("userinfo_endpoint", "https://idp.com/userinfo", false)

			idpDTO := &providers.IDPDTO{
				ID:   testIDPID,
				Name: "Test IDP",
				Type: providers.IDPTypeOAuth,
				Properties: []cmodels.Property{
					*clientIDProp, *clientSecretProp, *redirectURIProp, *scopesProp,
					*tokenEndpointProp, *userInfoEndpointProp,
				},
			}

			tokenRespJSON := testTokenRespJSON
			tokenHTTPResp := &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewReader([]byte(tokenRespJSON))),
			}

			userInfoJSON, _ := json.Marshal(tc.userInfo)
			userInfoHTTPResp := &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewReader(userInfoJSON)),
			}

			freshIDPMock.On("GetIdentityProvider", mock.Anything, testIDPID).Return(idpDTO, nil)
			freshHTTPMock.On("Do", mock.Anything).Return(tokenHTTPResp, nil).Once()
			freshHTTPMock.On("Do", mock.Anything).Return(userInfoHTTPResp, nil).Once()

			result, err := suite.service.Authenticate(context.Background(), testIDPID,
				common.AuthorizationData{Code: "auth_code"})
			suite.Nil(result)
			suite.NotNil(err)
			suite.Equal(common.ErrorSubClaimNotFound.Code, err.Code)
		})
	}
}

func (suite *OAuthAuthnServiceTestSuite) TestBuildFederatedAuthResultAppliesMappings() {
	idpDTO := createTestIDPDTO()
	idpDTO.AttributeConfiguration = &providers.AttributeConfiguration{
		UserTypeResolution: &providers.UserTypeResolution{Default: "person"},
		UserTypeAttributeMappings: []providers.UserTypeAttributeMapping{{
			UserType: "person",
			Attributes: []providers.AttributeMapping{
				{ExternalAttribute: "given_name", LocalAttribute: "firstName"},
			},
		}},
	}
	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).Return(idpDTO, nil)

	result, svcErr := suite.service.BuildFederatedAuthResult(
		context.Background(), testIDPID, testSub, map[string]interface{}{"given_name": "Jane"})
	suite.Nil(svcErr)
	suite.Equal("Jane", result.AuthenticatedClaims["firstName"])
}

// A mapping onto the local attribute sub must not change the subject the flow links.
func (suite *OAuthAuthnServiceTestSuite) TestBuildFederatedAuthResultKeepsVerifiedSub() {
	idpDTO := createTestIDPDTO()
	idpDTO.AttributeConfiguration = &providers.AttributeConfiguration{
		UserTypeResolution: &providers.UserTypeResolution{Default: "person"},
		UserTypeAttributeMappings: []providers.UserTypeAttributeMapping{{
			UserType: "person",
			Attributes: []providers.AttributeMapping{
				{ExternalAttribute: "email", LocalAttribute: "sub"},
			},
		}},
	}
	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).Return(idpDTO, nil)

	result, svcErr := suite.service.BuildFederatedAuthResult(
		context.Background(), testIDPID, testSub, map[string]interface{}{"email": "victim@example.com"})
	suite.Nil(svcErr)
	suite.Equal(testSub, result.AuthenticatedClaims[authnprovidercm.UserAttributeSub])
}

func (suite *OAuthAuthnServiceTestSuite) TestBuildFederatedAuthResultNilClaimsCarrySub() {
	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).Return(createTestIDPDTO(), nil)

	result, svcErr := suite.service.BuildFederatedAuthResult(context.Background(), testIDPID, testSub, nil)
	suite.Nil(svcErr)
	suite.Equal(testSub, result.AuthenticatedClaims[authnprovidercm.UserAttributeSub])
}

// The token names the identity rather than an entity. Resolving it is the authn provider's job, so
// this service performs no lookup of its own.
func (suite *OAuthAuthnServiceTestSuite) TestBuildFederatedAuthResultNamesTheIdentity() {
	idpDTO := createTestIDPDTO()
	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).Return(idpDTO, nil)

	result, svcErr := suite.service.BuildFederatedAuthResult(
		context.Background(), testIDPID, testSub, map[string]interface{}{"email": "user@example.com"})
	suite.Nil(svcErr)
	suite.Equal(testIDPID, result.Token[authnprovidercm.UserAttributeFederatedIdpID])
	suite.Equal(testSub, result.Token[authnprovidercm.UserAttributeSub])
	suite.NotContains(result.Token, common.UserAttributeUserID)
}

// The linking lookups travel in the token under their own key so the Account Linking node can match
// on them.
func (suite *OAuthAuthnServiceTestSuite) TestBuildFederatedAuthResultCarriesLinkingFilters() {
	idpDTO := createTestIDPDTO()
	idpDTO.AttributeConfiguration = &providers.AttributeConfiguration{
		AccountLinking: &providers.AccountLinking{Attributes: []string{"email", "username"}},
	}
	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).Return(idpDTO, nil)

	result, svcErr := suite.service.BuildFederatedAuthResult(context.Background(), testIDPID, testSub,
		map[string]interface{}{"email": "user@example.com", "username": "jdoe"})
	suite.Nil(svcErr)
	suite.Equal([]map[string]interface{}{{"email": "user@example.com", "username": "jdoe"}},
		result.Token[authnprovidercm.AccountLinkingFiltersKey])
	suite.NotContains(result.Token, "email")
}

// An identity with nothing to match on carries no linking key at all.
func (suite *OAuthAuthnServiceTestSuite) TestBuildFederatedAuthResultOmitsLinkingWithoutValues() {
	idpDTO := createTestIDPDTO()
	idpDTO.AttributeConfiguration = &providers.AttributeConfiguration{
		AccountLinking: &providers.AccountLinking{Attributes: []string{"email"}},
	}
	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).Return(idpDTO, nil)

	result, svcErr := suite.service.BuildFederatedAuthResult(
		context.Background(), testIDPID, testSub, map[string]interface{}{"name": "Jane"})
	suite.Nil(svcErr)
	suite.NotContains(result.Token, authnprovidercm.AccountLinkingFiltersKey)
}

// A linking attribute named by its external claim matches on the local attribute the mapping declares
// and, since mappings copy, on the local attribute of its own name.
func (suite *OAuthAuthnServiceTestSuite) TestBuildFederatedAuthResultMatchesMappedAndSameNamedAttribute() {
	idpDTO := createTestIDPDTO()
	idpDTO.AttributeConfiguration = &providers.AttributeConfiguration{
		AccountLinking:     &providers.AccountLinking{Attributes: []string{"email"}},
		UserTypeResolution: &providers.UserTypeResolution{Default: "person"},
		UserTypeAttributeMappings: []providers.UserTypeAttributeMapping{{
			UserType: "person",
			Attributes: []providers.AttributeMapping{
				{ExternalAttribute: "email", LocalAttribute: "username"},
			},
		}},
	}
	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).Return(idpDTO, nil)

	result, svcErr := suite.service.BuildFederatedAuthResult(
		context.Background(), testIDPID, testSub, map[string]interface{}{"email": "user@example.com"})
	suite.Nil(svcErr)
	suite.Equal([]map[string]interface{}{{"username": "user@example.com"}, {"email": "user@example.com"}},
		result.Token[authnprovidercm.AccountLinkingFiltersKey])
}

// No linking value can take the place of the keys that name the identity or be read as an entity id.
func (suite *OAuthAuthnServiceTestSuite) TestBuildFederatedAuthResultLinkingCannotRenameTheIdentity() {
	idpDTO := createTestIDPDTO()
	idpDTO.AttributeConfiguration = &providers.AttributeConfiguration{
		AccountLinking:     &providers.AccountLinking{Attributes: []string{"userID", "email"}},
		UserTypeResolution: &providers.UserTypeResolution{Default: "person"},
		UserTypeAttributeMappings: []providers.UserTypeAttributeMapping{{
			UserType: "person",
			Attributes: []providers.AttributeMapping{
				{ExternalAttribute: "email", LocalAttribute: "sub"},
			},
		}},
	}
	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).Return(idpDTO, nil)

	result, svcErr := suite.service.BuildFederatedAuthResult(context.Background(), testIDPID, testSub,
		map[string]interface{}{"userID": "victim-id", "email": "user@example.com"})
	suite.Nil(svcErr)
	suite.Equal(testSub, result.Token[authnprovidercm.UserAttributeSub])
	suite.Equal(testIDPID, result.Token[authnprovidercm.UserAttributeFederatedIdpID])
	suite.NotContains(result.Token, authnprovidercm.UserAttributeUserID)
	suite.NotContains(result.Token, authnprovidercm.AccountLinkingFiltersKey)
}

func (suite *OAuthAuthnServiceTestSuite) TestBuildFederatedAuthResultClientError() {
	clientErr := &tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType, Code: "IDP-1001",
		ErrorDescription: tidcommon.I18nMessage{DefaultValue: "not found"},
	}
	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).Return(nil, clientErr)

	result, svcErr := suite.service.BuildFederatedAuthResult(context.Background(), testIDPID, testSub, nil)
	suite.Nil(result)
	suite.NotNil(svcErr)
	suite.Equal(ErrorClientErrorWhileRetrievingIDP.Code, svcErr.Code)
}

func (suite *OAuthAuthnServiceTestSuite) TestBuildFederatedAuthResultServerError() {
	serverErr := &tidcommon.ServiceError{
		Type: tidcommon.ServerErrorType, Code: "IDP-5000",
		ErrorDescription: tidcommon.I18nMessage{DefaultValue: "boom"},
	}
	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).Return(nil, serverErr)

	result, svcErr := suite.service.BuildFederatedAuthResult(context.Background(), testIDPID, testSub, nil)
	suite.Nil(result)
	suite.NotNil(svcErr)
	suite.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *OAuthAuthnServiceTestSuite) TestBuildFederatedAuthResultNilIDP() {
	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).Return(nil, nil)

	result, svcErr := suite.service.BuildFederatedAuthResult(context.Background(), testIDPID, testSub, nil)
	suite.Nil(result)
	suite.NotNil(svcErr)
	suite.Equal(ErrorInvalidIDP.Code, svcErr.Code)
}
