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
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

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
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
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

var errEntityNotFound = &entityprovider.EntityProviderError{Code: entityprovider.ErrorCodeEntityNotFound}

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

	serverErr := &tidcommon.ServiceError{
		Type: tidcommon.ServerErrorType,
		Code: "INTERNAL_ERROR",
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.test.database_unavailable", DefaultValue: "Database unavailable",
		},
	}
	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).Return(nil, serverErr)

	url, err := suite.service.BuildAuthorizeURL(context.Background(), testIDPID)
	suite.Empty(url)
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

func (suite *OAuthAuthnServiceTestSuite) TestGetInternalUserSuccess() {
	svcImpl := suite.service.(*oAuthAuthnService)

	userID := testUserID
	user := &providers.Entity{
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

	result, err := svcImpl.GetInternalUser(context.Background(), testSub)
	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(userID, result.ID)
}

func (suite *OAuthAuthnServiceTestSuite) TestGetInternalUserWithError_EmptySub() {
	svcImpl := suite.service.(*oAuthAuthnService)

	result, err := svcImpl.GetInternalUser(context.Background(), "")
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorEmptySubClaim.Code, err.Code)
}

func (suite *OAuthAuthnServiceTestSuite) TestGetInternalUserWithError_UserNotFound() {
	svcImpl := suite.service.(*oAuthAuthnService)

	upErr := &entityprovider.EntityProviderError{Code: entityprovider.ErrorCodeEntityNotFound}
	suite.mockEntityProvider.On("IdentifyEntity", mock.Anything).Return(nil, upErr)

	result, err := svcImpl.GetInternalUser(context.Background(), testSub)
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(common.ErrorUserNotFound.Code, err.Code)
}

func (suite *OAuthAuthnServiceTestSuite) TestGetInternalUserWithError_AmbiguousUser() {
	svcImpl := suite.service.(*oAuthAuthnService)

	upErr := &entityprovider.EntityProviderError{Code: entityprovider.ErrorCodeAmbiguousEntity}
	suite.mockEntityProvider.On("IdentifyEntity", mock.Anything).Return(nil, upErr)

	result, err := svcImpl.GetInternalUser(context.Background(), testSub)
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(common.ErrorAmbiguousUser.Code, err.Code)
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
			expectedErrCode: tidcommon.InternalServerError.Code,
		},
		{
			name: "GetUserServerError",
			mockSetup: func(m *entityprovidermock.EntityProviderInterfaceMock) {
				userID := testUserID
				serverErr := &entityprovider.EntityProviderError{
					Code:    entityprovider.ErrorCodeSystemError,
					Message: "Database unavailable",
				}
				m.On("IdentifyEntity", mock.Anything).Return(&userID, nil)
				m.On("GetEntity", userID).Return(nil, serverErr)
			},
			expectedErrCode: tidcommon.InternalServerError.Code,
		},
		{
			name: "GetUserNotFound",
			mockSetup: func(m *entityprovidermock.EntityProviderInterfaceMock) {
				userID := testUserID
				notFoundErr := &entityprovider.EntityProviderError{
					Code: entityprovider.ErrorCodeEntityNotFound,
				}
				m.On("IdentifyEntity", mock.Anything).Return(&userID, nil)
				m.On("GetEntity", userID).Return(nil, notFoundErr)
			},
			expectedErrCode: common.ErrorUserNotFound.Code,
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
			svcImpl := suite.service.(*oAuthAuthnService)
			svcImpl.entityProvider = freshUserMock

			if tc.mockSetup != nil {
				tc.mockSetup(freshUserMock)
			}

			result, err := svcImpl.GetInternalUser(context.Background(), testSub)
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

			url, err := suite.service.BuildAuthorizeURL(context.Background(), testIDPID)
			suite.Empty(url)
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

	result, err := suite.service.Authenticate(context.Background(), testIDPID, "auth_code")
	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal("user_sub_123", result.Token["sub"])
	suite.Equal("user@example.com", result.AuthenticatedClaims["email"])
}

func (suite *OAuthAuthnServiceTestSuite) TestAuthenticateTokenExchangeFailure() {
	result, err := suite.service.Authenticate(context.Background(), testIDPID, "")
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorEmptyAuthorizationCode.Code, err.Code)
}

func (suite *OAuthAuthnServiceTestSuite) TestAuthenticateFetchUserInfoFailure() {
	clientIDProp, _ := cmodels.NewProperty("client_id", "test_client", false)
	clientSecretProp, _ := cmodels.NewProperty("client_secret", "test_secret", false)
	redirectURIProp, _ := cmodels.NewProperty("redirect_uri", "https://app.com/callback", false)
	scopesProp, _ := cmodels.NewProperty("scopes", "openid", false)
	tokenEndpointProp, _ := cmodels.NewProperty("token_endpoint", "https://idp.com/token", false)

	idpDTO := &providers.IDPDTO{
		ID:   testIDPID,
		Name: "Test IDP",
		Type: providers.IDPTypeOAuth,
		Properties: []cmodels.Property{
			*clientIDProp, *clientSecretProp, *redirectURIProp, *scopesProp, *tokenEndpointProp,
		},
	}

	tokenRespJSON := testTokenRespJSON
	tokenHTTPResp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader([]byte(tokenRespJSON))),
	}

	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).Return(idpDTO, nil)
	suite.mockHTTPClient.On("Do", mock.Anything).Return(tokenHTTPResp, nil).Once()

	result, err := suite.service.Authenticate(context.Background(), testIDPID, "auth_code")
	suite.Nil(result)
	suite.NotNil(err)
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

			result, err := suite.service.Authenticate(context.Background(), testIDPID, "auth_code")
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
			UserType:   "person",
			Attributes: []providers.AttributeMapping{{ExternalAttribute: "given_name", LocalAttribute: "firstName"}},
		}},
	}
	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).Return(idpDTO, nil)

	result, svcErr := suite.service.BuildFederatedAuthResult(
		context.Background(), testIDPID, testSub, map[string]interface{}{"given_name": "Jane", "sub": testSub})
	suite.Nil(svcErr)
	suite.Equal("Jane", result.AuthenticatedClaims["firstName"])
	suite.NotContains(result.AuthenticatedClaims, "given_name")
	// No account linking configured, so the lookup falls back to sub without a query.
	suite.Equal(testSub, result.Token["sub"])
}

func (suite *OAuthAuthnServiceTestSuite) TestBuildFederatedAuthResultLinksByAttribute() {
	// sub does not resolve, so the configured account-linking attribute is returned as the filter,
	// deferring the actual lookup to the caller.
	idpDTO := createTestIDPDTO()
	idpDTO.AttributeConfiguration = &providers.AttributeConfiguration{
		AccountLinking: &providers.AccountLinking{Attributes: []string{"email"}},
	}
	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).Return(idpDTO, nil)
	suite.mockEntityProvider.On("IdentifyEntity",
		map[string]interface{}{"sub": testSub}).Return(nil, errEntityNotFound)

	result, svcErr := suite.service.BuildFederatedAuthResult(
		context.Background(), testIDPID, testSub, map[string]interface{}{"email": "user@example.com"})
	suite.Nil(svcErr)
	suite.Equal("user@example.com", result.Token["email"])
	suite.NotContains(result.Token, "sub")
}

func (suite *OAuthAuthnServiceTestSuite) TestBuildFederatedAuthResultPrefersSubWhenResolved() {
	// When sub resolves an existing user, the configured account-linking attributes are not consulted.
	idpDTO := createTestIDPDTO()
	idpDTO.AttributeConfiguration = &providers.AttributeConfiguration{
		AccountLinking: &providers.AccountLinking{Attributes: []string{"email"}},
	}
	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).Return(idpDTO, nil)
	resolvedID := testUserID
	suite.mockEntityProvider.On("IdentifyEntity",
		map[string]interface{}{"sub": testSub}).Return(&resolvedID, nil)

	result, svcErr := suite.service.BuildFederatedAuthResult(
		context.Background(), testIDPID, testSub, map[string]interface{}{"email": "user@example.com"})
	suite.Nil(svcErr)
	suite.Equal(testUserID, result.Token[common.UserAttributeUserID])
	suite.mockEntityProvider.AssertNotCalled(suite.T(), "IdentifyEntity",
		map[string]interface{}{"email": "user@example.com"})
}

func (suite *OAuthAuthnServiceTestSuite) TestBuildFederatedAuthResultCombinesLinkedAttributes() {
	// All configured account-linking attributes with a value are combined into a single filter, so
	// the caller's lookup resolves a unique user by all of them together.
	idpDTO := createTestIDPDTO()
	idpDTO.AttributeConfiguration = &providers.AttributeConfiguration{
		AccountLinking: &providers.AccountLinking{Attributes: []string{"email", "username"}},
	}
	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).Return(idpDTO, nil)
	suite.mockEntityProvider.On("IdentifyEntity",
		map[string]interface{}{"sub": testSub}).Return(nil, errEntityNotFound)

	result, svcErr := suite.service.BuildFederatedAuthResult(context.Background(), testIDPID, testSub,
		map[string]interface{}{"email": "user@example.com", "username": "jdoe"})
	suite.Nil(svcErr)
	suite.Equal("user@example.com", result.Token["email"])
	suite.Equal("jdoe", result.Token["username"])
	suite.NotContains(result.Token, "sub")
}

func (suite *OAuthAuthnServiceTestSuite) TestBuildFederatedAuthResultResolvesExternalToLocalAttribute() {
	// The account-linking attribute is an external name mapped to a different local attribute; both the
	// mapped claim and the lookup key must be the local attribute.
	idpDTO := createTestIDPDTO()
	idpDTO.AttributeConfiguration = &providers.AttributeConfiguration{
		UserTypeResolution: &providers.UserTypeResolution{Default: "Person"},
		UserTypeAttributeMappings: []providers.UserTypeAttributeMapping{
			{UserType: "Person", Attributes: []providers.AttributeMapping{
				{ExternalAttribute: "email", LocalAttribute: "family_name"},
			}},
		},
		AccountLinking: &providers.AccountLinking{Attributes: []string{"email"}},
	}
	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).Return(idpDTO, nil)
	suite.mockEntityProvider.On("IdentifyEntity",
		map[string]interface{}{"sub": testSub}).Return(nil, errEntityNotFound)

	result, svcErr := suite.service.BuildFederatedAuthResult(
		context.Background(), testIDPID, testSub, map[string]interface{}{"email": "sadil@wso2.com"})
	suite.Nil(svcErr)
	suite.Equal("sadil@wso2.com", result.AuthenticatedClaims["family_name"])
	suite.Equal("sadil@wso2.com", result.Token["family_name"])
	suite.NotContains(result.Token, "sub")
}

func (suite *OAuthAuthnServiceTestSuite) TestBuildFederatedAuthResultAmbiguousSubFallsBackToSubWhenNoAttributeValue() {
	// An ambiguous sub match does not short-circuit; account-linking attributes are still tried. With
	// none of them having a value here, the original sub filter is returned, so the ambiguity is
	// surfaced when the caller looks it up.
	idpDTO := createTestIDPDTO()
	idpDTO.AttributeConfiguration = &providers.AttributeConfiguration{
		AccountLinking: &providers.AccountLinking{Attributes: []string{"email"}},
	}
	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).Return(idpDTO, nil)
	ambiguousErr := &entityprovider.EntityProviderError{Code: entityprovider.ErrorCodeAmbiguousEntity}
	suite.mockEntityProvider.On("IdentifyEntity",
		map[string]interface{}{"sub": testSub}).Return(nil, ambiguousErr)

	result, svcErr := suite.service.BuildFederatedAuthResult(context.Background(), testIDPID, testSub, nil)
	suite.Nil(svcErr)
	suite.Equal(testSub, result.Token["sub"])
	suite.NotContains(result.Token, common.UserAttributeUserID)
}

func (suite *OAuthAuthnServiceTestSuite) TestBuildFederatedAuthResultAmbiguousSubFallsThroughToAccountLinking() {
	// An ambiguous sub match must not skip account-linking resolution: a configured attribute that
	// would uniquely identify the user still gets a chance to resolve the login.
	idpDTO := createTestIDPDTO()
	idpDTO.AttributeConfiguration = &providers.AttributeConfiguration{
		AccountLinking: &providers.AccountLinking{Attributes: []string{"email"}},
	}
	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).Return(idpDTO, nil)
	ambiguousErr := &entityprovider.EntityProviderError{Code: entityprovider.ErrorCodeAmbiguousEntity}
	suite.mockEntityProvider.On("IdentifyEntity",
		map[string]interface{}{"sub": testSub}).Return(nil, ambiguousErr)

	result, svcErr := suite.service.BuildFederatedAuthResult(
		context.Background(), testIDPID, testSub, map[string]interface{}{"email": "user@example.com"})
	suite.Nil(svcErr)
	suite.Equal("user@example.com", result.Token["email"])
	suite.NotContains(result.Token, "sub")
}

func (suite *OAuthAuthnServiceTestSuite) TestBuildFederatedAuthResultSurfacesServerErrorOnSubLookup() {
	// A real (non not-found/ambiguous) entity provider error while resolving the sub must be surfaced,
	// not silently treated as "not found".
	idpDTO := createTestIDPDTO()
	idpDTO.AttributeConfiguration = &providers.AttributeConfiguration{
		AccountLinking: &providers.AccountLinking{Attributes: []string{"email"}},
	}
	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).Return(idpDTO, nil)
	serverErr := &entityprovider.EntityProviderError{Code: entityprovider.ErrorCodeSystemError}
	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{"sub": testSub}).Return(nil, serverErr)

	result, svcErr := suite.service.BuildFederatedAuthResult(
		context.Background(), testIDPID, testSub, map[string]interface{}{"email": "user@example.com"})
	suite.Nil(result)
	suite.NotNil(svcErr)
	suite.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *OAuthAuthnServiceTestSuite) TestBuildFederatedAuthResultFallsBackToSubWhenNotConfigured() {
	// No account linking configured: the sub filter is returned as-is, with no lookup performed here
	// (original, pre-account-linking behavior).
	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).Return(createTestIDPDTO(), nil)

	result, svcErr := suite.service.BuildFederatedAuthResult(context.Background(), testIDPID, testSub, nil)
	suite.Nil(svcErr)
	suite.Equal(testSub, result.Token["sub"])
}

func (suite *OAuthAuthnServiceTestSuite) TestBuildFederatedAuthResultFallsBackToSubWhenAttributeMissing() {
	idpDTO := createTestIDPDTO()
	idpDTO.AttributeConfiguration = &providers.AttributeConfiguration{
		AccountLinking: &providers.AccountLinking{Attributes: []string{"email"}},
	}
	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, testIDPID).Return(idpDTO, nil)
	suite.mockEntityProvider.On("IdentifyEntity", mock.Anything).Return(nil, errEntityNotFound)

	result, svcErr := suite.service.BuildFederatedAuthResult(
		context.Background(), testIDPID, testSub, map[string]interface{}{"name": "no-email"})
	suite.Nil(svcErr)
	suite.Equal(testSub, result.Token["sub"])
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
