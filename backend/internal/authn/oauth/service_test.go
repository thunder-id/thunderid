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

package oauth

import (
	"github.com/thunder-id/thunderid/internal/system/i18n/core"

	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/httpmock"
	"github.com/thunder-id/thunderid/tests/mocks/idp/idpmock"
)

const (
	testIDPID   = "idp123"
	testSub     = "user_sub_123"
	testAuthURL = "https://idp.com/authorize?client_id=test_client&redirect_uri=https%3A%2F%2Fapp.com%2Fcallback&response_type=code&scope=openid&state=random_state" //nolint:lll
)

type OAuthAuthnServiceTestSuite struct {
	suite.Suite
	mockHTTPClient     *httpmock.HTTPClientInterfaceMock
	mockIDPService     *idpmock.IDPServiceInterfaceMock
	mockEntityProvider *entityprovidermock.EntityProviderInterfaceMock
	service            OAuthAuthnServiceInterface
	endpoints          OAuthEndpoints
}

func TestOAuthAuthnServiceTestSuite(t *testing.T) {
	suite.Run(t, new(OAuthAuthnServiceTestSuite))
}

func (suite *OAuthAuthnServiceTestSuite) SetupTest() {
	suite.mockHTTPClient = httpmock.NewHTTPClientInterfaceMock(suite.T())
	suite.mockIDPService = idpmock.NewIDPServiceInterfaceMock(suite.T())
	suite.mockEntityProvider = entityprovidermock.NewEntityProviderInterfaceMock(suite.T())
	suite.endpoints = OAuthEndpoints{
		AuthorizationEndpoint: "https://localhost:8090/oauth/authorize",
		TokenEndpoint:         "https://localhost:8090/oauth/token",
		UserInfoEndpoint:      "https://localhost:8090/oauth/userinfo",
	}
	// Use the constructor to properly initialize the service including logger
	suite.service = newOAuthAuthnService(suite.mockHTTPClient, suite.mockIDPService, suite.mockEntityProvider)
}

func createTestIDPDTO(idpID string) *idp.IDPDTO {
	clientIDProp, _ := cmodels.NewProperty("client_id", "test_client", false)
	clientSecretProp, _ := cmodels.NewProperty("client_secret", "test_secret", false)
	redirectURIProp, _ := cmodels.NewProperty("redirect_uri", "https://app.com/callback", false)
	scopesProp, _ := cmodels.NewProperty("scopes", "openid", false)
	tokenEndpointProp, _ := cmodels.NewProperty("token_endpoint", "https://idp.com/token", false)

	return &idp.IDPDTO{
		ID:   idpID,
		Name: "Test IDP",
		Type: idp.IDPTypeOAuth,
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

	idpDTO := &idp.IDPDTO{
		ID:   testIDPID,
		Name: "Test OAuth Provider",
		Type: idp.IDPTypeOAuth,
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
				clientErr := &serviceerror.ServiceError{
					Type: serviceerror.ClientErrorType,
					Code: "IDP_NOT_FOUND",
					ErrorDescription: core.I18nMessage{
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
				serverErr := &serviceerror.ServiceError{
					Type: serviceerror.ServerErrorType,
					Code: "INTERNAL_ERROR",
					ErrorDescription: core.I18nMessage{
						Key: "error.test.database_unavailable", DefaultValue: "Database unavailable",
					},
				}
				m.On("GetIdentityProvider", mock.Anything, testIDPID).Return(nil, serverErr)
			},
			expectedErrCode: serviceerror.InternalServerError.Code,
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

	idpDTO := &idp.IDPDTO{
		ID:   testIDPID,
		Name: "Test OAuth Provider",
		Type: idp.IDPTypeOAuth,
		Properties: []cmodels.Property{
			*clientIDProp, *clientSecretProp, *redirectURIProp, *scopesProp, *authzEndpointProp,
		},
	}
	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).Return(idpDTO, nil)

	url, err := suite.service.BuildAuthorizeURL(context.Background(), testIDPID)
	suite.Nil(err)
	suite.NotNil(url)
	suite.Contains(url, "https://example.com/oauth/authorize?")
	suite.Contains(url, "response_type=code")
	suite.Contains(url, "client_id=test_client_id")
	suite.Contains(url, "redirect_uri=https%3A%2F%2Fapp.example.com%2Fcallback")
	suite.Contains(url, "scope=openid+profile")
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

			idpDTO := &idp.IDPDTO{
				ID:   testIDPID,
				Name: "Test OAuth Provider",
				Type: idp.IDPTypeOAuth,
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

			url, err := suite.service.BuildAuthorizeURL(context.Background(), testIDPID)
			suite.Nil(err)
			suite.NotNil(url)
			suite.Contains(url, "https://example.com/oauth/authorize?")
			suite.Contains(url, "client_id=test_client_id")
			suite.Contains(url, "redirect_uri=https%3A%2F%2Fapp.example.com%2Fcallback")

			// Ensure the forbidden string (empty-key value or empty-value key) is not present in the URL
			suite.NotContains(url, tc.forbiddenStr)
		})
	}
}

func (suite *OAuthAuthnServiceTestSuite) TestBuildAuthorizeURLWithError() {
	if svcImpl, ok := suite.service.(*oAuthAuthnService); ok {
		svcImpl.idpService = suite.mockIDPService
	}

	serverErr := &serviceerror.ServiceError{
		Type: serviceerror.ServerErrorType,
		Code: "INTERNAL_ERROR",
		ErrorDescription: core.I18nMessage{
			Key: "error.test.database_unavailable", DefaultValue: "Database unavailable",
		},
	}
	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).Return(nil, serverErr)

	url, err := suite.service.BuildAuthorizeURL(context.Background(), testIDPID)
	suite.Empty(url)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
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

			idpData := &idp.IDPDTO{
				ID:   testIDPID,
				Name: "Test IDP",
				Type: idp.IDPTypeOAuth,
				Properties: []cmodels.Property{
					*clientIDProp, *clientSecretProp, *redirectURIProp, *scopesProp, *tokenEndpointProp,
				},
			}

			tokenRespJSON := `{"access_token":"access123","token_type":"Bearer"}`
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
				svcErr := &serviceerror.ServiceError{
					Code: "IDP-001",
					Type: serviceerror.ClientErrorType,
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

				idpData := &idp.IDPDTO{
					ID:   testIDPID,
					Name: "Test IDP",
					Type: idp.IDPTypeOAuth,
					Properties: []cmodels.Property{
						*clientIDProp, *clientSecretProp, *redirectURIProp, *scopesProp, *tokenEndpointProp,
					},
				}

				suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).Return(idpData, nil).Once()
				suite.mockHTTPClient.On("Do", mock.Anything).
					Return(nil, errors.New("network error")).Once()
			},
			expectedError: serviceerror.InternalServerError.Code,
		},
		{
			name: "Non200StatusCode",
			setupMocks: func() {
				idpData := createTestIDPDTO(testIDPID)
				resp := &http.Response{
					StatusCode: 401,
					Body:       io.NopCloser(bytes.NewReader([]byte(`{"error":"invalid_grant"}`))),
				}

				suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).Return(idpData, nil).Once()
				suite.mockHTTPClient.On("Do", mock.Anything).Return(resp, nil).Once()
			},
			expectedError: serviceerror.InternalServerError.Code,
		},
		{
			name: "InvalidJSONResponse",
			setupMocks: func() {
				idpData := createTestIDPDTO(testIDPID)
				resp := &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewReader([]byte(`invalid json`))),
				}

				suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).Return(idpData, nil).Once()
				suite.mockHTTPClient.On("Do", mock.Anything).Return(resp, nil).Once()
			},
			expectedError: serviceerror.InternalServerError.Code,
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

	idpDTO := &idp.IDPDTO{
		ID:   testIDPID,
		Name: "Test OAuth Provider",
		Type: idp.IDPTypeOAuth,
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

	userInfo, err := suite.service.FetchUserInfoWithClientConfig(config, accessToken)
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

	userInfo, err := suite.service.FetchUserInfoWithClientConfig(config, "")
	suite.Nil(userInfo)
	suite.NotNil(err)
	suite.Equal(ErrorEmptyAccessToken.Code, err.Code)
}

func (suite *OAuthAuthnServiceTestSuite) TestGetInternalUserSuccess() {
	userID := "user123"
	user := &entityprovider.Entity{
		ID:   userID,
		Type: "person",
		OUID: "test-ou",
	}

	suite.mockEntityProvider.On("IdentifyEntity", mock.MatchedBy(
		func(filters map[string]interface{}) bool {
			return filters["sub"] == testSub
		}),
	).Return(&userID, nil)
	suite.mockEntityProvider.On("GetEntity", userID).Return(user, nil)

	result, err := suite.service.GetInternalUser(testSub)
	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(userID, result.ID)
}

func (suite *OAuthAuthnServiceTestSuite) TestGetInternalUserWithError_EmptySub() {
	result, err := suite.service.GetInternalUser("")
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorEmptySubClaim.Code, err.Code)
}

func (suite *OAuthAuthnServiceTestSuite) TestGetInternalUserWithError_UserNotFound() {
	upErr := &entityprovider.EntityProviderError{Code: entityprovider.ErrorCodeEntityNotFound}
	suite.mockEntityProvider.On("IdentifyEntity", mock.Anything).Return(nil, upErr)

	result, err := suite.service.GetInternalUser(testSub)
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(common.ErrorUserNotFound.Code, err.Code)
}

func (suite *OAuthAuthnServiceTestSuite) TestGetInternalUserWithServiceError() {
	tests := []struct {
		name            string
		mockSetup       func(m *entityprovidermock.EntityProviderInterfaceMock)
		expectedErrCode string
	}{
		{
			name: "IdentifyServerError",
			mockSetup: func(m *entityprovidermock.EntityProviderInterfaceMock) {
				serverErr := &entityprovider.EntityProviderError{
					Code:    entityprovider.ErrorCodeSystemError,
					Message: "Database unavailable",
				}
				m.On("IdentifyEntity", mock.Anything).Return(nil, serverErr)
			},
			expectedErrCode: serviceerror.InternalServerError.Code,
		},
		{
			name: "GetUserServerError",
			mockSetup: func(m *entityprovidermock.EntityProviderInterfaceMock) {
				userID := "user123"
				serverErr := &entityprovider.EntityProviderError{
					Code:    entityprovider.ErrorCodeSystemError,
					Message: "Database unavailable",
				}
				m.On("IdentifyEntity", mock.Anything).Return(&userID, nil)
				m.On("GetEntity", userID).Return(nil, serverErr)
			},
			expectedErrCode: serviceerror.InternalServerError.Code,
		},
		{
			name: "IdentifyNilUserID",
			mockSetup: func(m *entityprovidermock.EntityProviderInterfaceMock) {
				m.On("IdentifyEntity", mock.Anything).Return(nil, (*entityprovider.EntityProviderError)(nil))
			},
			expectedErrCode: common.ErrorUserNotFound.Code,
		},
	}

	for _, tc := range tests {
		suite.Run(tc.name, func() {
			freshUserMock := entityprovidermock.NewEntityProviderInterfaceMock(suite.T())
			if svcImpl, ok := suite.service.(*oAuthAuthnService); ok {
				svcImpl.entityProvider = freshUserMock
			}

			if tc.mockSetup != nil {
				tc.mockSetup(freshUserMock)
			}

			result, err := suite.service.GetInternalUser(testSub)
			suite.Nil(result)
			suite.NotNil(err)
			suite.Equal(tc.expectedErrCode, err.Code)
		})
	}
}

func (suite *OAuthAuthnServiceTestSuite) TestValidateTokenResponseSuccess() {
	tokenResp := &TokenResponse{
		AccessToken: "access_token_123",
		TokenType:   "Bearer",
		ExpiresIn:   3600,
	}

	err := suite.service.ValidateTokenResponse(testIDPID, tokenResp)
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
			err := suite.service.ValidateTokenResponse(testIDPID, tc.resp)
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

			idpDTO := &idp.IDPDTO{
				ID:   testIDPID,
				Name: "Test OAuth Provider",
				Type: idp.IDPTypeOAuth,
				Properties: []cmodels.Property{
					*clientIDProp, *clientSecretProp, *redirectURIProp, *scopesProp, *authzEndpointProp,
				},
			}
			suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).Return(idpDTO, nil)

			url, err := suite.service.BuildAuthorizeURL(context.Background(), testIDPID)
			suite.Empty(url)
			suite.NotNil(err)
			suite.Equal(serviceerror.InternalServerError.Code, err.Code)
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

	idpData := &idp.IDPDTO{
		ID:   testIDPID,
		Name: "Test IDP",
		Type: idp.IDPTypeOAuth,
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

	userInfo, err := suite.service.FetchUserInfoWithClientConfig(config, "access_token")
	suite.Nil(userInfo)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *OAuthAuthnServiceTestSuite) TestExchangeCodeForTokenMissingTokenEndpoint() {
	clientIDProp, _ := cmodels.NewProperty("client_id", "test_client", false)
	clientSecretProp, _ := cmodels.NewProperty("client_secret", "test_secret", false)
	redirectURIProp, _ := cmodels.NewProperty("redirect_uri", "https://app.com/callback", false)
	scopesProp, _ := cmodels.NewProperty("scopes", "openid", false)
	tokenEndpointProp, _ := cmodels.NewProperty("token_endpoint", "", false)

	idpDTO := &idp.IDPDTO{
		ID:   testIDPID,
		Name: "Test OAuth Provider",
		Type: idp.IDPTypeOAuth,
		Properties: []cmodels.Property{
			*clientIDProp, *clientSecretProp, *redirectURIProp, *scopesProp, *tokenEndpointProp,
		},
	}
	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).Return(idpDTO, nil)

	token, err := suite.service.ExchangeCodeForToken(context.Background(), testIDPID, "code123", false)
	suite.Nil(token)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *OAuthAuthnServiceTestSuite) TestFetchUserInfoMissingUserInfoEndpoint() {
	clientIDProp, _ := cmodels.NewProperty("client_id", "test_client", false)
	clientSecretProp, _ := cmodels.NewProperty("client_secret", "test_secret", false)
	redirectURIProp, _ := cmodels.NewProperty("redirect_uri", "https://app.com/callback", false)
	scopesProp, _ := cmodels.NewProperty("scopes", "openid", false)
	userInfoEndpointProp, _ := cmodels.NewProperty("userinfo_endpoint", "", false)

	idpDTO := &idp.IDPDTO{
		ID:   testIDPID,
		Name: "Test OAuth Provider",
		Type: idp.IDPTypeOAuth,
		Properties: []cmodels.Property{
			*clientIDProp, *clientSecretProp, *redirectURIProp, *scopesProp, *userInfoEndpointProp,
		},
	}
	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).Return(idpDTO, nil)

	userInfo, err := suite.service.FetchUserInfo(context.Background(), testIDPID, "access_token")
	suite.Nil(userInfo)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}
