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

package github

import (
	"github.com/thunder-id/thunderid/internal/system/i18n/core"

	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/authn/oauth"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/authn/oauthmock"
	"github.com/thunder-id/thunderid/tests/mocks/httpmock"
)

const (
	testGithubIDPID         = "github_idp"
	testAccessToken         = "access_token"
	githubUserInfoEndpoint  = "https://api.github.com/user"
	githubUserEmailEndpoint = "https://api.github.com/user/emails"
)

type GithubOAuthAuthnServiceTestSuite struct {
	suite.Suite
	mockOAuthService *oauthmock.OAuthAuthnServiceInterfaceMock
	mockHTTPClient   *httpmock.HTTPClientInterfaceMock
	service          GithubOAuthAuthnServiceInterface
}

func TestGithubOAuthAuthnServiceTestSuite(t *testing.T) {
	suite.Run(t, new(GithubOAuthAuthnServiceTestSuite))
}

func (suite *GithubOAuthAuthnServiceTestSuite) SetupTest() {
	suite.mockOAuthService = oauthmock.NewOAuthAuthnServiceInterfaceMock(suite.T())
	suite.mockHTTPClient = httpmock.NewHTTPClientInterfaceMock(suite.T())

	service := &githubOAuthAuthnService{
		internal:   suite.mockOAuthService,
		httpClient: suite.mockHTTPClient,
		logger:     log.GetLogger().With(log.String(log.LoggerKeyComponentName, "GithubAuthnService")),
	}
	suite.service = service

	thunderConfig := &config.Config{
		TLS: config.TLSConfig{
			MinVersion: "1.3",
		},
	}
	err := config.InitializeServerRuntime("", thunderConfig)
	assert.NoError(suite.T(), err)
}

func (suite *GithubOAuthAuthnServiceTestSuite) TestBuildAuthorizeURLSuccess() {
	expectedURL := "https://github.com/login/oauth/authorize?client_id=test"
	suite.mockOAuthService.On("BuildAuthorizeURL", mock.Anything, testGithubIDPID).Return(expectedURL, nil)

	url, err := suite.service.BuildAuthorizeURL(context.Background(), testGithubIDPID)
	suite.Nil(err)
	suite.Equal(expectedURL, url)
}

func (suite *GithubOAuthAuthnServiceTestSuite) TestBuildAuthorizeURLError() {
	svcErr := &serviceerror.ServiceError{
		Code:             "ERROR",
		ErrorDescription: core.I18nMessage{Key: "error.test.failed_to_build_url", DefaultValue: "Failed to build URL"},
	}
	suite.mockOAuthService.On("BuildAuthorizeURL", mock.Anything, testGithubIDPID).Return("", svcErr)

	url, err := suite.service.BuildAuthorizeURL(context.Background(), testGithubIDPID)
	suite.Empty(url)
	suite.NotNil(err)
	suite.Equal(svcErr.Code, err.Code)
}

func (suite *GithubOAuthAuthnServiceTestSuite) TestExchangeCodeForTokenSuccess() {
	code := "auth_code"
	tokenResp := &oauth.TokenResponse{
		AccessToken: testAccessToken,
		TokenType:   "Bearer",
	}
	suite.mockOAuthService.On("ExchangeCodeForToken", mock.Anything, testGithubIDPID, code, false).
		Return(tokenResp, nil)

	result, err := suite.service.ExchangeCodeForToken(context.Background(), testGithubIDPID, code, false)
	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(tokenResp.AccessToken, result.AccessToken)
}

func (suite *GithubOAuthAuthnServiceTestSuite) TestExchangeCodeForTokenError() {
	code := "auth_code"
	svcErr := &serviceerror.ServiceError{
		Code: "TOKEN_ERROR",
		ErrorDescription: core.I18nMessage{
			Key: "error.test.failed_to_exchange_token", DefaultValue: "Failed to exchange token",
		},
	}
	suite.mockOAuthService.On("ExchangeCodeForToken", mock.Anything, testGithubIDPID, code, false).Return(nil, svcErr)

	result, err := suite.service.ExchangeCodeForToken(context.Background(), testGithubIDPID, code, false)
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(svcErr.Code, err.Code)
}

func (suite *GithubOAuthAuthnServiceTestSuite) TestFetchUserInfoSuccess() {
	accessToken := testAccessToken
	userInfo := map[string]interface{}{
		"id":    float64(12345),
		"login": "testuser",
		"email": "test@example.com",
	}

	config := &oauth.OAuthClientConfig{
		Scopes: []string{"user", "user:email"},
		OAuthEndpoints: oauth.OAuthEndpoints{
			UserInfoEndpoint: githubUserInfoEndpoint,
		},
	}

	suite.mockOAuthService.On("GetOAuthClientConfig", mock.Anything, testGithubIDPID).Return(config, nil)
	suite.mockOAuthService.On("FetchUserInfoWithClientConfig", config, accessToken).
		Return(userInfo, nil)

	result, err := suite.service.FetchUserInfo(context.Background(), testGithubIDPID, accessToken)
	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal("12345", result["sub"])
}

func (suite *GithubOAuthAuthnServiceTestSuite) TestFetchUserInfoSuccessWithEmailFetch() {
	accessToken := testAccessToken
	userInfo := map[string]interface{}{
		"id":    float64(12345),
		"login": "testuser",
	}
	emailData := []map[string]interface{}{
		{
			"email":    "test@example.com",
			"primary":  true,
			"verified": true,
		},
	}
	emailJSON, _ := json.Marshal(emailData)

	config := &oauth.OAuthClientConfig{
		Scopes: []string{"user:email"},
		OAuthEndpoints: oauth.OAuthEndpoints{
			UserInfoEndpoint:  githubUserInfoEndpoint,
			UserEmailEndpoint: githubUserEmailEndpoint,
		},
	}

	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader(emailJSON)),
	}

	// Mock GetOAuthClientConfig once for FetchUserInfo (config is passed to fetchPrimaryEmail)
	suite.mockOAuthService.On("GetOAuthClientConfig", mock.Anything, testGithubIDPID).Return(config, nil).Once()
	suite.mockOAuthService.On("FetchUserInfoWithClientConfig", config, accessToken).Return(userInfo, nil)
	suite.mockHTTPClient.On("Do", mock.Anything).Return(resp, nil)

	result, err := suite.service.FetchUserInfo(context.Background(), testGithubIDPID, accessToken)
	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal("test@example.com", result["email"])
}

func (suite *GithubOAuthAuthnServiceTestSuite) TestFetchUserInfoWithFailure() {
	testCases := []struct {
		name      string
		setupMock func()
		errCode   string
	}{
		{
			name: "ConfigRetrievalFailure",
			setupMock: func() {
				suite.mockOAuthService.On("GetOAuthClientConfig", mock.Anything, testGithubIDPID).
					Return(nil, &serviceerror.ServiceError{Code: "CONFIG-001"}).Once()
			},
			errCode: "CONFIG-001",
		},
		{
			name: "UserInfoFetchFailure",
			setupMock: func() {
				config := &oauth.OAuthClientConfig{
					Scopes: []string{"user"},
					OAuthEndpoints: oauth.OAuthEndpoints{
						UserInfoEndpoint: githubUserInfoEndpoint,
					},
				}
				suite.mockOAuthService.On("GetOAuthClientConfig", mock.Anything, testGithubIDPID).
					Return(config, nil).Once()
				suite.mockOAuthService.On("FetchUserInfoWithClientConfig", config, testAccessToken).
					Return(nil, &serviceerror.ServiceError{Code: "FETCH-001"}).Once()
			},
			errCode: "FETCH-001",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			tc.setupMock()

			result, err := suite.service.FetchUserInfo(context.Background(), testGithubIDPID, testAccessToken)
			suite.Nil(result)
			suite.NotNil(err)
			suite.Equal(tc.errCode, err.Code)
		})
	}
}

func (suite *GithubOAuthAuthnServiceTestSuite) TestFetchUserInfoWithEmailFetchFailure() {
	testCases := []struct {
		name      string
		setupMock func()
		errCode   string
	}{
		{
			name: "EmailFetchHTTPError",
			setupMock: func() {
				userInfo := map[string]interface{}{
					"id":    float64(12345),
					"login": "testuser",
				}
				config := &oauth.OAuthClientConfig{
					Scopes: []string{"user:email"},
					OAuthEndpoints: oauth.OAuthEndpoints{
						UserInfoEndpoint:  githubUserInfoEndpoint,
						UserEmailEndpoint: githubUserEmailEndpoint,
					},
				}

				suite.mockOAuthService.On("GetOAuthClientConfig", mock.Anything, testGithubIDPID).
					Return(config, nil).Once()
				suite.mockOAuthService.On("FetchUserInfoWithClientConfig", config, testAccessToken).
					Return(userInfo, nil).Once()
				suite.mockHTTPClient.On("Do", mock.Anything).
					Return(nil, errors.New("http error")).Once()
			},
			errCode: serviceerror.InternalServerError.Code,
		},
		{
			name: "EmailFetchNon200Status",
			setupMock: func() {
				userInfo := map[string]interface{}{
					"id":    float64(12345),
					"login": "testuser",
				}
				config := &oauth.OAuthClientConfig{
					Scopes: []string{"user:email"},
					OAuthEndpoints: oauth.OAuthEndpoints{
						UserInfoEndpoint:  githubUserInfoEndpoint,
						UserEmailEndpoint: githubUserEmailEndpoint,
					},
				}

				resp := &http.Response{
					StatusCode: 403,
					Body:       io.NopCloser(bytes.NewReader([]byte(`{"error":"forbidden"}`))),
				}

				suite.mockOAuthService.On("GetOAuthClientConfig", mock.Anything, testGithubIDPID).
					Return(config, nil).Once()
				suite.mockOAuthService.On("FetchUserInfoWithClientConfig", config, testAccessToken).
					Return(userInfo, nil).Once()
				suite.mockHTTPClient.On("Do", mock.Anything).Return(resp, nil).Once()
			},
			errCode: serviceerror.InternalServerError.Code,
		},
		{
			name: "EmailFetchInvalidJSON",
			setupMock: func() {
				userInfo := map[string]interface{}{
					"id":    float64(12345),
					"login": "testuser",
				}
				config := &oauth.OAuthClientConfig{
					Scopes: []string{"user:email"},
					OAuthEndpoints: oauth.OAuthEndpoints{
						UserInfoEndpoint:  githubUserInfoEndpoint,
						UserEmailEndpoint: githubUserEmailEndpoint,
					},
				}

				resp := &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewReader([]byte(`invalid json`))),
				}

				suite.mockOAuthService.On("GetOAuthClientConfig", mock.Anything, testGithubIDPID).
					Return(config, nil).Once()
				suite.mockOAuthService.On("FetchUserInfoWithClientConfig", config, testAccessToken).
					Return(userInfo, nil).Once()
				suite.mockHTTPClient.On("Do", mock.Anything).Return(resp, nil).Once()
			},
			errCode: serviceerror.InternalServerError.Code,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			tc.setupMock()

			result, err := suite.service.FetchUserInfo(context.Background(), testGithubIDPID, testAccessToken)
			suite.Nil(result)
			suite.NotNil(err)
			suite.Equal(tc.errCode, err.Code)
		})
	}
}

func (suite *GithubOAuthAuthnServiceTestSuite) TestGetInternalUserSuccess() {
	sub := "user123"
	user := &entityprovider.Entity{
		ID:   "user123",
		Type: "person",
	}
	suite.mockOAuthService.On("GetInternalUser", sub).Return(user, nil)

	result, err := suite.service.GetInternalUser(sub)
	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(user.ID, result.ID)
}

func (suite *GithubOAuthAuthnServiceTestSuite) TestGetInternalUserError() {
	sub := "user123"
	svcErr := &serviceerror.ServiceError{
		Code:             "USER_NOT_FOUND",
		ErrorDescription: core.I18nMessage{Key: "error.test.user_not_found", DefaultValue: "User not found"},
	}
	suite.mockOAuthService.On("GetInternalUser", sub).Return(nil, svcErr)

	result, err := suite.service.GetInternalUser(sub)
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(svcErr.Code, err.Code)
}

func (suite *GithubOAuthAuthnServiceTestSuite) TestShouldFetchEmailAndGetMetadata() {
	// type assert to concrete implementation
	gsvc, ok := suite.service.(*githubOAuthAuthnService)
	suite.True(ok)

	// shouldFetchEmail true cases
	suite.True(gsvc.shouldFetchEmail([]string{UserScope}))
	suite.True(gsvc.shouldFetchEmail([]string{UserEmailScope}))

	// shouldFetchEmail false
	suite.False(gsvc.shouldFetchEmail([]string{"openid", "profile"}))
}

func (suite *GithubOAuthAuthnServiceTestSuite) TestGetOAuthClientConfig() {
	expectedConfig := &oauth.OAuthClientConfig{
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Scopes:       []string{"user"},
	}
	suite.mockOAuthService.On("GetOAuthClientConfig", mock.Anything, testGithubIDPID).Return(expectedConfig, nil)

	config, err := suite.service.GetOAuthClientConfig(context.Background(), testGithubIDPID)
	suite.Nil(err)
	suite.NotNil(config)
	suite.Equal("test-client", config.ClientID)
}

func (suite *GithubOAuthAuthnServiceTestSuite) TestFetchPrimaryEmailEdgeCases() {
	// no primary present
	emailData := []map[string]interface{}{
		{"email": "a@example.com", "primary": false},
		{"email": "b@example.com", "primary": false},
	}
	emailJSON, _ := json.Marshal(emailData)

	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader(emailJSON)),
	}

	config := &oauth.OAuthClientConfig{
		OAuthEndpoints: oauth.OAuthEndpoints{
			UserEmailEndpoint: githubUserEmailEndpoint,
		},
	}

	suite.mockHTTPClient.On("Do", mock.Anything).Return(resp, nil).Once()

	gsvc, ok := suite.service.(*githubOAuthAuthnService)
	suite.True(ok)

	email, svcErr := gsvc.fetchPrimaryEmail(config, testAccessToken)
	suite.Nil(svcErr)
	suite.Equal("", email)

	// primary present but email not string
	badData := []map[string]interface{}{{"email": 12345, "primary": true}}
	badJSON, _ := json.Marshal(badData)
	resp2 := &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(badJSON))}

	suite.mockHTTPClient.On("Do", mock.Anything).Return(resp2, nil).Once()
	email2, svcErr2 := gsvc.fetchPrimaryEmail(config, testAccessToken)
	suite.Nil(svcErr2)
	suite.Equal("", email2)
}

func (suite *GithubOAuthAuthnServiceTestSuite) TestConstructorAndInjectInternal() {
	svcInterface := newGithubOAuthAuthnService(suite.mockOAuthService, suite.mockHTTPClient)
	gsvc, ok := svcInterface.(*githubOAuthAuthnService)
	suite.True(ok)

	// test BuildAuthorizeURL delegation
	expectedURL := "https://github.com/login/oauth/authorize?client_id=test"
	suite.mockOAuthService.On("BuildAuthorizeURL", mock.Anything, testGithubIDPID).Return(expectedURL, nil)
	url, err := gsvc.BuildAuthorizeURL(context.Background(), testGithubIDPID)
	suite.Nil(err)
	suite.Equal(expectedURL, url)

	// test ExchangeCodeForToken delegation
	tokenResp := &oauth.TokenResponse{AccessToken: "atoken", TokenType: "Bearer"}
	suite.mockOAuthService.On("ExchangeCodeForToken", mock.Anything, testGithubIDPID, "code", false).
		Return(tokenResp, nil)
	tr, err2 := gsvc.ExchangeCodeForToken(context.Background(), testGithubIDPID, "code", false)
	suite.Nil(err2)
	suite.Equal("atoken", tr.AccessToken)
}

func (suite *GithubOAuthAuthnServiceTestSuite) TestFetchUserInfoNoEmailScope() {
	accessToken := testAccessToken
	userInfo := map[string]interface{}{
		"id":    float64(12345),
		"login": "testuser",
	}

	config := &oauth.OAuthClientConfig{
		Scopes: []string{"profile"},
		OAuthEndpoints: oauth.OAuthEndpoints{
			UserInfoEndpoint: githubUserInfoEndpoint,
		},
	}

	suite.mockOAuthService.On("GetOAuthClientConfig", mock.Anything, testGithubIDPID).Return(config, nil)
	suite.mockOAuthService.On("FetchUserInfoWithClientConfig", config, accessToken).Return(userInfo, nil)

	result, err := suite.service.FetchUserInfo(context.Background(), testGithubIDPID, accessToken)
	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal("12345", result["sub"])
}

func (suite *GithubOAuthAuthnServiceTestSuite) TestFetchPrimaryEmailWithEmptyEndpoint() {
	accessToken := testAccessToken
	userInfo := map[string]interface{}{
		"id":    float64(12345),
		"login": "testuser",
	}

	config := &oauth.OAuthClientConfig{
		Scopes: []string{"user:email"},
		OAuthEndpoints: oauth.OAuthEndpoints{
			UserInfoEndpoint:  githubUserInfoEndpoint,
			UserEmailEndpoint: "", // Empty - should return error
		},
	}

	suite.mockOAuthService.On("GetOAuthClientConfig", mock.Anything, testGithubIDPID).Return(config, nil).Once()
	suite.mockOAuthService.On("FetchUserInfoWithClientConfig", config, accessToken).Return(userInfo, nil)

	result, err := suite.service.FetchUserInfo(context.Background(), testGithubIDPID, accessToken)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
	suite.Nil(result)
}

func (suite *GithubOAuthAuthnServiceTestSuite) TestFetchUserInfoWithEmptyPrimaryEmail() {
	accessToken := testAccessToken
	userInfo := map[string]interface{}{
		"id":    float64(12345),
		"login": "testuser",
	}
	emailData := []map[string]interface{}{
		{
			"email":    "test@example.com",
			"primary":  false, // Not primary
			"verified": true,
		},
	}
	emailJSON, _ := json.Marshal(emailData)

	config := &oauth.OAuthClientConfig{
		Scopes: []string{"user:email"},
		OAuthEndpoints: oauth.OAuthEndpoints{
			UserInfoEndpoint:  githubUserInfoEndpoint,
			UserEmailEndpoint: githubUserEmailEndpoint,
		},
	}

	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader(emailJSON)),
	}

	suite.mockOAuthService.On("GetOAuthClientConfig", mock.Anything, testGithubIDPID).Return(config, nil).Once()
	suite.mockOAuthService.On("FetchUserInfoWithClientConfig", config, accessToken).Return(userInfo, nil)
	suite.mockHTTPClient.On("Do", mock.Anything).Return(resp, nil)

	result, err := suite.service.FetchUserInfo(context.Background(), testGithubIDPID, accessToken)
	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal("12345", result["sub"])
	// Email should not be added since no primary email found
	_, hasEmail := result["email"]
	suite.False(hasEmail)
}
