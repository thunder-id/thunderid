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

package google

import (
	"github.com/thunder-id/thunderid/internal/system/i18n/core"

	"context"
	"encoding/base64"
	"encoding/json"
	"testing"

	"time"

	"github.com/stretchr/testify/mock"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/authn/oauth"
	"github.com/thunder-id/thunderid/internal/authn/oidc"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/authn/oidcmock"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
)

const (
	testGoogleIDPID = "google_idp"
	testClientID    = "test-client-id"
	testAuthCode    = "auth_code"
)

type GoogleOIDCAuthnServiceTestSuite struct {
	suite.Suite
	mockOIDCService *oidcmock.OIDCAuthnServiceInterfaceMock
	mockJWTService  *jwtmock.JWTServiceInterfaceMock
	service         *googleOIDCAuthnService
}

func TestGoogleOIDCAuthnServiceTestSuite(t *testing.T) {
	suite.Run(t, new(GoogleOIDCAuthnServiceTestSuite))
}

func (suite *GoogleOIDCAuthnServiceTestSuite) SetupTest() {
	suite.mockOIDCService = oidcmock.NewOIDCAuthnServiceInterfaceMock(suite.T())
	suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())
	suite.service = &googleOIDCAuthnService{
		internal:   suite.mockOIDCService,
		jwtService: suite.mockJWTService,
		logger:     log.GetLogger().With(log.String(log.LoggerKeyComponentName, "GoogleOIDCAuthnService")),
	}

	// Initialize config with leeway for tests
	testConfig := &config.Config{
		JWT: config.JWTConfig{
			Leeway: 30, // 30 seconds leeway for clock skew
		},
	}
	_ = config.InitializeServerRuntime("test", testConfig)
}

func (suite *GoogleOIDCAuthnServiceTestSuite) TestBuildAuthorizeURLSuccess() {
	expectedURL := "https://accounts.google.com/o/oauth2/v2/auth?client_id=test"
	suite.mockOIDCService.On("BuildAuthorizeURL", mock.Anything, testGoogleIDPID).Return(expectedURL, nil)

	url, err := suite.service.BuildAuthorizeURL(context.Background(), testGoogleIDPID)
	suite.Nil(err)
	suite.Equal(expectedURL, url)
}

func (suite *GoogleOIDCAuthnServiceTestSuite) TestExchangeCodeForTokenSuccess() {
	tokenResp := &oauth.TokenResponse{
		AccessToken: "access_token",
		IDToken:     "id_token",
		TokenType:   "Bearer",
	}
	suite.mockOIDCService.On("ExchangeCodeForToken", mock.Anything, testGoogleIDPID, testAuthCode, false).
		Return(tokenResp, nil)

	result, err := suite.service.ExchangeCodeForToken(context.Background(), testGoogleIDPID, testAuthCode, false)
	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(tokenResp.AccessToken, result.AccessToken)
}

func (suite *GoogleOIDCAuthnServiceTestSuite) TestExchangeCodeForTokenWithValidation() {
	now := time.Now()
	validClaims := map[string]interface{}{
		"iss": Issuer1,
		"aud": testClientID,
		"sub": "user123",
		"exp": float64(now.Add(1 * time.Hour).Unix()),
		"iat": float64(now.Add(-1 * time.Minute).Unix()),
	}
	idToken := generateTestJWT(validClaims)

	tokenResp := &oauth.TokenResponse{
		AccessToken: "access_token",
		IDToken:     idToken,
		TokenType:   "Bearer",
	}

	oAuthConfig := &oauth.OAuthClientConfig{
		ClientID:       testClientID,
		ClientSecret:   "test-secret",
		OAuthEndpoints: oauth.OAuthEndpoints{},
	}

	suite.mockOIDCService.On("ExchangeCodeForToken", mock.Anything, testGoogleIDPID, testAuthCode, false).
		Return(tokenResp, nil)
	suite.mockOIDCService.On("ValidateTokenResponse", mock.Anything, testGoogleIDPID, tokenResp, false).
		Return(nil)
	suite.mockOIDCService.On("GetOAuthClientConfig", mock.Anything, testGoogleIDPID).Return(oAuthConfig, nil)

	result, err := suite.service.ExchangeCodeForToken(context.Background(), testGoogleIDPID, testAuthCode, true)
	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(tokenResp.AccessToken, result.AccessToken)
}

func (suite *GoogleOIDCAuthnServiceTestSuite) TestExchangeCodeForTokenFailure() {
	suite.mockOIDCService.On("ExchangeCodeForToken", mock.Anything, testGoogleIDPID, testAuthCode, false).
		Return(nil, &serviceerror.ServiceError{Code: "TOKEN-001"})

	result, err := suite.service.ExchangeCodeForToken(context.Background(), testGoogleIDPID, testAuthCode, false)
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal("TOKEN-001", err.Code)
}

func (suite *GoogleOIDCAuthnServiceTestSuite) TestExchangeCodeForTokenValidationFailure() {
	now := time.Now()
	invalidClaims := map[string]interface{}{
		"iss": "invalid-issuer",
		"aud": testClientID,
		"exp": float64(now.Add(1 * time.Hour).Unix()),
		"iat": float64(now.Add(-1 * time.Minute).Unix()),
	}
	idToken := generateTestJWT(invalidClaims)

	tokenResp := &oauth.TokenResponse{
		AccessToken: "access_token",
		IDToken:     idToken,
		TokenType:   "Bearer",
	}

	oAuthConfig := &oauth.OAuthClientConfig{
		ClientID:       testClientID,
		ClientSecret:   "test-secret",
		OAuthEndpoints: oauth.OAuthEndpoints{},
	}

	suite.mockOIDCService.On("ExchangeCodeForToken", mock.Anything, testGoogleIDPID, testAuthCode, false).
		Return(tokenResp, nil)
	suite.mockOIDCService.On("ValidateTokenResponse", mock.Anything, testGoogleIDPID, tokenResp, false).
		Return(nil)
	suite.mockOIDCService.On("GetOAuthClientConfig", mock.Anything, testGoogleIDPID).Return(oAuthConfig, nil)

	result, err := suite.service.ExchangeCodeForToken(context.Background(), testGoogleIDPID, testAuthCode, true)
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(oidc.ErrorInvalidIDToken.Code, err.Code)
}

func (suite *GoogleOIDCAuthnServiceTestSuite) TestValidateTokenResponseSuccess() {
	now := time.Now()
	validClaims := map[string]interface{}{
		"iss": Issuer1,
		"aud": testClientID,
		"sub": "user123",
		"exp": float64(now.Add(1 * time.Hour).Unix()),
		"iat": float64(now.Add(-1 * time.Minute).Unix()),
	}
	idToken := generateTestJWT(validClaims)

	tokenResp := &oauth.TokenResponse{
		AccessToken: "access_token",
		IDToken:     idToken,
		TokenType:   "Bearer",
	}

	oAuthConfig := &oauth.OAuthClientConfig{
		ClientID:       testClientID,
		ClientSecret:   "test-secret",
		OAuthEndpoints: oauth.OAuthEndpoints{},
	}

	suite.mockOIDCService.On("ValidateTokenResponse", mock.Anything, testGoogleIDPID, tokenResp, false).
		Return(nil)
	suite.mockOIDCService.On("GetOAuthClientConfig", mock.Anything, testGoogleIDPID).Return(oAuthConfig, nil)

	err := suite.service.ValidateTokenResponse(context.Background(), testGoogleIDPID, tokenResp)
	suite.Nil(err)
}

func (suite *GoogleOIDCAuthnServiceTestSuite) TestValidateTokenResponseInternalValidationFailure() {
	tokenResp := &oauth.TokenResponse{
		AccessToken: "access_token",
		IDToken:     "id_token",
		TokenType:   "Bearer",
	}

	suite.mockOIDCService.On("ValidateTokenResponse", mock.Anything, testGoogleIDPID, tokenResp, false).
		Return(&serviceerror.ServiceError{Code: "VALIDATION-001"})

	err := suite.service.ValidateTokenResponse(context.Background(), testGoogleIDPID, tokenResp)
	suite.NotNil(err)
	suite.Equal("VALIDATION-001", err.Code)
}

func (suite *GoogleOIDCAuthnServiceTestSuite) TestValidateTokenResponseIDTokenValidationFailure() {
	now := time.Now()
	invalidClaims := map[string]interface{}{
		"iss": "invalid-issuer",
		"aud": testClientID,
		"exp": float64(now.Add(1 * time.Hour).Unix()),
		"iat": float64(now.Add(-1 * time.Minute).Unix()),
	}
	idToken := generateTestJWT(invalidClaims)

	tokenResp := &oauth.TokenResponse{
		AccessToken: "access_token",
		IDToken:     idToken,
		TokenType:   "Bearer",
	}

	oAuthConfig := &oauth.OAuthClientConfig{
		ClientID:       testClientID,
		ClientSecret:   "test-secret",
		OAuthEndpoints: oauth.OAuthEndpoints{},
	}

	suite.mockOIDCService.On("ValidateTokenResponse", mock.Anything, testGoogleIDPID, tokenResp, false).
		Return(nil)
	suite.mockOIDCService.On("GetOAuthClientConfig", mock.Anything, testGoogleIDPID).Return(oAuthConfig, nil)

	err := suite.service.ValidateTokenResponse(context.Background(), testGoogleIDPID, tokenResp)
	suite.NotNil(err)
	suite.Equal(oidc.ErrorInvalidIDToken.Code, err.Code)
}

func (suite *GoogleOIDCAuthnServiceTestSuite) TestValidateIDTokenSuccess() {
	now := time.Now()

	testCases := []struct {
		name        string
		claims      map[string]interface{}
		oAuthConfig *oauth.OAuthClientConfig
		setupMocks  func(idToken string, config *oauth.OAuthClientConfig)
	}{
		{
			name: "BasicValidToken",
			claims: map[string]interface{}{
				"iss": Issuer1,
				"aud": testClientID,
				"sub": "user123",
				"exp": float64(now.Add(1 * time.Hour).Unix()),
				"iat": float64(now.Add(-1 * time.Minute).Unix()),
			},
			oAuthConfig: &oauth.OAuthClientConfig{
				ClientID:       testClientID,
				ClientSecret:   "test-secret",
				OAuthEndpoints: oauth.OAuthEndpoints{},
			},
			setupMocks: func(idToken string, config *oauth.OAuthClientConfig) {
				suite.mockOIDCService.On("GetOAuthClientConfig", mock.Anything, testGoogleIDPID).
					Return(config, nil).Once()
			},
		},
		{
			name: "ValidTokenWithIssuer2",
			claims: map[string]interface{}{
				"iss": Issuer2,
				"aud": testClientID,
				"sub": "user123",
				"exp": float64(now.Add(1 * time.Hour).Unix()),
				"iat": float64(now.Add(-1 * time.Minute).Unix()),
			},
			oAuthConfig: &oauth.OAuthClientConfig{
				ClientID:       testClientID,
				ClientSecret:   "test-secret",
				OAuthEndpoints: oauth.OAuthEndpoints{},
			},
			setupMocks: func(idToken string, config *oauth.OAuthClientConfig) {
				suite.mockOIDCService.On("GetOAuthClientConfig", mock.Anything, testGoogleIDPID).
					Return(config, nil).Once()
			},
		},
		{
			name: "WithJWKSEndpoint",
			claims: map[string]interface{}{
				"iss": Issuer1,
				"aud": testClientID,
				"sub": "user123",
				"exp": float64(now.Add(1 * time.Hour).Unix()),
				"iat": float64(now.Add(-1 * time.Minute).Unix()),
			},
			oAuthConfig: &oauth.OAuthClientConfig{
				ClientID:     testClientID,
				ClientSecret: "test-secret",
				OAuthEndpoints: oauth.OAuthEndpoints{
					JwksEndpoint: "https://www.googleapis.com/oauth2/v3/certs",
				},
			},
			setupMocks: func(idToken string, config *oauth.OAuthClientConfig) {
				suite.mockOIDCService.On("GetOAuthClientConfig", mock.Anything, testGoogleIDPID).
					Return(config, nil).Once()
				suite.mockJWTService.On("VerifyJWTSignatureWithJWKS", idToken, config.OAuthEndpoints.JwksEndpoint).
					Return(nil).Once()
			},
		},
		{
			name: "WithValidHostedDomain",
			claims: map[string]interface{}{
				"iss": Issuer1,
				"aud": testClientID,
				"sub": "user123",
				"exp": float64(now.Add(1 * time.Hour).Unix()),
				"iat": float64(now.Add(-1 * time.Minute).Unix()),
				"hd":  "example.com",
			},
			oAuthConfig: &oauth.OAuthClientConfig{
				ClientID:     testClientID,
				ClientSecret: "test-secret",
				OAuthEndpoints: oauth.OAuthEndpoints{
					JwksEndpoint: "https://www.googleapis.com/oauth2/v3/certs",
				},
				AdditionalParams: map[string]string{
					"hd": "example.com",
				},
			},
			setupMocks: func(idToken string, config *oauth.OAuthClientConfig) {
				suite.mockOIDCService.On("GetOAuthClientConfig", mock.Anything, testGoogleIDPID).
					Return(config, nil).Once()
				suite.mockJWTService.On("VerifyJWTSignatureWithJWKS", idToken, config.OAuthEndpoints.JwksEndpoint).
					Return(nil).Once()
			},
		},
		{
			name: "HostedDomainPresentButNotRequired",
			claims: map[string]interface{}{
				"iss": Issuer1,
				"aud": testClientID,
				"exp": float64(now.Add(1 * time.Hour).Unix()),
				"iat": float64(now.Add(-1 * time.Minute).Unix()),
				"hd":  "example.com",
			},
			oAuthConfig: &oauth.OAuthClientConfig{
				ClientID:       testClientID,
				ClientSecret:   "test-secret",
				OAuthEndpoints: oauth.OAuthEndpoints{},
			},
			setupMocks: func(idToken string, config *oauth.OAuthClientConfig) {
				suite.mockOIDCService.On("GetOAuthClientConfig", mock.Anything, testGoogleIDPID).
					Return(config, nil).Once()
			},
		},
		{
			name: "HostedDomainEmptyInConfig",
			claims: map[string]interface{}{
				"iss": Issuer1,
				"aud": testClientID,
				"exp": float64(now.Add(1 * time.Hour).Unix()),
				"iat": float64(now.Add(-1 * time.Minute).Unix()),
				"hd":  "example.com",
			},
			oAuthConfig: &oauth.OAuthClientConfig{
				ClientID:       testClientID,
				ClientSecret:   "test-secret",
				OAuthEndpoints: oauth.OAuthEndpoints{},
				AdditionalParams: map[string]string{
					"hd": "",
				},
			},
			setupMocks: func(idToken string, config *oauth.OAuthClientConfig) {
				suite.mockOIDCService.On("GetOAuthClientConfig", mock.Anything, testGoogleIDPID).
					Return(config, nil).Once()
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			idToken := generateTestJWT(tc.claims)
			tc.setupMocks(idToken, tc.oAuthConfig)

			err := suite.service.ValidateIDToken(context.Background(), testGoogleIDPID, idToken)
			suite.Nil(err)
		})
	}
}

func (suite *GoogleOIDCAuthnServiceTestSuite) TestValidateIDTokenWithFailure() {
	now := time.Now()

	hostedDomainConfig := &oauth.OAuthClientConfig{
		ClientID:     testClientID,
		ClientSecret: "test-secret",
		OAuthEndpoints: oauth.OAuthEndpoints{
			JwksEndpoint: "https://www.googleapis.com/oauth2/v3/certs",
		},
		AdditionalParams: map[string]string{
			"hd": "example.com",
		},
	}
	hostedDomainSetupMocks := func(idToken string, config *oauth.OAuthClientConfig) {
		suite.mockOIDCService.On("GetOAuthClientConfig", mock.Anything, testGoogleIDPID).
			Return(config, nil).Once()
		suite.mockJWTService.On("VerifyJWTSignatureWithJWKS", idToken, config.OAuthEndpoints.JwksEndpoint).
			Return(nil).Once()
	}

	testCases := []struct {
		name                string
		idToken             string
		claims              map[string]interface{}
		oAuthConfig         *oauth.OAuthClientConfig
		setupMocks          func(idToken string, config *oauth.OAuthClientConfig)
		expectedErrorCode   string
		expectedErrContains string
	}{
		{
			name:              "EmptyToken",
			idToken:           "",
			expectedErrorCode: oidc.ErrorInvalidIDToken.Code,
			setupMocks:        func(idToken string, config *oauth.OAuthClientConfig) {},
		},
		{
			name:              "WhitespaceOnlyToken",
			idToken:           "   ",
			expectedErrorCode: oidc.ErrorInvalidIDToken.Code,
			setupMocks:        func(idToken string, config *oauth.OAuthClientConfig) {},
		},
		{
			name: "GetConfigFailure",
			claims: map[string]interface{}{
				"iss": Issuer1,
				"aud": testClientID,
				"exp": float64(now.Add(1 * time.Hour).Unix()),
				"iat": float64(now.Add(-1 * time.Minute).Unix()),
			},
			expectedErrorCode: "CONFIG-001",
			setupMocks: func(idToken string, config *oauth.OAuthClientConfig) {
				suite.mockOIDCService.On("GetOAuthClientConfig", mock.Anything, testGoogleIDPID).
					Return(nil, &serviceerror.ServiceError{Code: "CONFIG-001"}).Once()
			},
		},
		{
			name: "SignatureVerificationFailure",
			claims: map[string]interface{}{
				"iss": Issuer1,
				"aud": testClientID,
				"exp": float64(now.Add(1 * time.Hour).Unix()),
				"iat": float64(now.Add(-1 * time.Minute).Unix()),
			},
			oAuthConfig: &oauth.OAuthClientConfig{
				ClientID:     testClientID,
				ClientSecret: "test-secret",
				OAuthEndpoints: oauth.OAuthEndpoints{
					JwksEndpoint: "https://www.googleapis.com/oauth2/v3/certs",
				},
			},
			expectedErrorCode: oidc.ErrorInvalidIDTokenSignature.Code,
			setupMocks: func(idToken string, config *oauth.OAuthClientConfig) {
				suite.mockOIDCService.On("GetOAuthClientConfig", mock.Anything, testGoogleIDPID).
					Return(config, nil).Once()
				suite.mockJWTService.On("VerifyJWTSignatureWithJWKS", idToken, config.OAuthEndpoints.JwksEndpoint).
					Return(&serviceerror.ServiceError{
						Type: serviceerror.ServerErrorType,
						Code: "SIGNATURE_VERIFICATION_FAILED",
						Error: core.I18nMessage{
							Key:          "error.test.signature_verification_failed",
							DefaultValue: "Signature verification failed",
						},
						ErrorDescription: core.I18nMessage{
							Key:          "error.test.signature_verification_failed",
							DefaultValue: "signature verification failed",
						},
					}).Once()
			},
		},
		{
			name:    "InvalidJWTFormat",
			idToken: "not.a.valid.jwt.token",
			oAuthConfig: &oauth.OAuthClientConfig{
				ClientID:       testClientID,
				ClientSecret:   "test-secret",
				OAuthEndpoints: oauth.OAuthEndpoints{},
			},
			expectedErrorCode: oidc.ErrorInvalidIDToken.Code,
			setupMocks: func(idToken string, config *oauth.OAuthClientConfig) {
				suite.mockOIDCService.On("GetOAuthClientConfig", mock.Anything, testGoogleIDPID).
					Return(config, nil).Once()
			},
		},
		{
			name: "InvalidIssuer",
			claims: map[string]interface{}{
				"iss": "invalid-issuer.com",
				"aud": testClientID,
				"exp": float64(now.Add(1 * time.Hour).Unix()),
				"iat": float64(now.Add(-1 * time.Minute).Unix()),
			},
			oAuthConfig: &oauth.OAuthClientConfig{
				ClientID:       testClientID,
				ClientSecret:   "test-secret",
				OAuthEndpoints: oauth.OAuthEndpoints{},
			},
			expectedErrorCode:   oidc.ErrorInvalidIDToken.Code,
			expectedErrContains: "issuer",
			setupMocks: func(idToken string, config *oauth.OAuthClientConfig) {
				suite.mockOIDCService.On("GetOAuthClientConfig", mock.Anything, testGoogleIDPID).
					Return(config, nil).Once()
			},
		},
		{
			name: "MissingIssuer",
			claims: map[string]interface{}{
				"aud": testClientID,
				"exp": float64(now.Add(1 * time.Hour).Unix()),
				"iat": float64(now.Add(-1 * time.Minute).Unix()),
			},
			oAuthConfig: &oauth.OAuthClientConfig{
				ClientID:       testClientID,
				ClientSecret:   "test-secret",
				OAuthEndpoints: oauth.OAuthEndpoints{},
			},
			expectedErrorCode: oidc.ErrorInvalidIDToken.Code,
			setupMocks: func(idToken string, config *oauth.OAuthClientConfig) {
				suite.mockOIDCService.On("GetOAuthClientConfig", mock.Anything, testGoogleIDPID).
					Return(config, nil).Once()
			},
		},
		{
			name: "InvalidAudience",
			claims: map[string]interface{}{
				"iss": Issuer1,
				"aud": "wrong-client-id",
				"exp": float64(now.Add(1 * time.Hour).Unix()),
				"iat": float64(now.Add(-1 * time.Minute).Unix()),
			},
			oAuthConfig: &oauth.OAuthClientConfig{
				ClientID:       testClientID,
				ClientSecret:   "test-secret",
				OAuthEndpoints: oauth.OAuthEndpoints{},
			},
			expectedErrorCode:   oidc.ErrorInvalidIDToken.Code,
			expectedErrContains: "audience",
			setupMocks: func(idToken string, config *oauth.OAuthClientConfig) {
				suite.mockOIDCService.On("GetOAuthClientConfig", mock.Anything, testGoogleIDPID).
					Return(config, nil).Once()
			},
		},
		{
			name: "MissingAudience",
			claims: map[string]interface{}{
				"iss": Issuer1,
				"exp": float64(now.Add(1 * time.Hour).Unix()),
				"iat": float64(now.Add(-1 * time.Minute).Unix()),
			},
			oAuthConfig: &oauth.OAuthClientConfig{
				ClientID:       testClientID,
				ClientSecret:   "test-secret",
				OAuthEndpoints: oauth.OAuthEndpoints{},
			},
			expectedErrorCode: oidc.ErrorInvalidIDToken.Code,
			setupMocks: func(idToken string, config *oauth.OAuthClientConfig) {
				suite.mockOIDCService.On("GetOAuthClientConfig", mock.Anything, testGoogleIDPID).
					Return(config, nil).Once()
			},
		},
		{
			name: "ExpiredToken",
			claims: map[string]interface{}{
				"iss": Issuer1,
				"aud": testClientID,
				"exp": float64(now.Add(-1 * time.Hour).Unix()),
				"iat": float64(now.Add(-2 * time.Hour).Unix()),
			},
			oAuthConfig: &oauth.OAuthClientConfig{
				ClientID:       testClientID,
				ClientSecret:   "test-secret",
				OAuthEndpoints: oauth.OAuthEndpoints{},
			},
			expectedErrorCode:   oidc.ErrorInvalidIDToken.Code,
			expectedErrContains: "expired",
			setupMocks: func(idToken string, config *oauth.OAuthClientConfig) {
				suite.mockOIDCService.On("GetOAuthClientConfig", mock.Anything, testGoogleIDPID).
					Return(config, nil).Once()
			},
		},
		{
			name: "MissingExpiration",
			claims: map[string]interface{}{
				"iss": Issuer1,
				"aud": testClientID,
				"iat": float64(now.Add(-1 * time.Minute).Unix()),
			},
			oAuthConfig: &oauth.OAuthClientConfig{
				ClientID:       testClientID,
				ClientSecret:   "test-secret",
				OAuthEndpoints: oauth.OAuthEndpoints{},
			},
			expectedErrorCode:   oidc.ErrorInvalidIDToken.Code,
			expectedErrContains: "expiration",
			setupMocks: func(idToken string, config *oauth.OAuthClientConfig) {
				suite.mockOIDCService.On("GetOAuthClientConfig", mock.Anything, testGoogleIDPID).
					Return(config, nil).Once()
			},
		},
		{
			name: "InvalidExpirationFormat",
			claims: map[string]interface{}{
				"iss": Issuer1,
				"aud": testClientID,
				"exp": "invalid-exp",
				"iat": float64(now.Add(-1 * time.Minute).Unix()),
			},
			oAuthConfig: &oauth.OAuthClientConfig{
				ClientID:       testClientID,
				ClientSecret:   "test-secret",
				OAuthEndpoints: oauth.OAuthEndpoints{},
			},
			expectedErrorCode:   oidc.ErrorInvalidIDToken.Code,
			expectedErrContains: "expiration",
			setupMocks: func(idToken string, config *oauth.OAuthClientConfig) {
				suite.mockOIDCService.On("GetOAuthClientConfig", mock.Anything, testGoogleIDPID).
					Return(config, nil).Once()
			},
		},
		{
			name: "IssuedInFuture",
			claims: map[string]interface{}{
				"iss": Issuer1,
				"aud": testClientID,
				"exp": float64(now.Add(2 * time.Hour).Unix()),
				"iat": float64(now.Add(1 * time.Hour).Unix()),
			},
			oAuthConfig: &oauth.OAuthClientConfig{
				ClientID:       testClientID,
				ClientSecret:   "test-secret",
				OAuthEndpoints: oauth.OAuthEndpoints{},
			},
			expectedErrorCode:   oidc.ErrorInvalidIDToken.Code,
			expectedErrContains: "future",
			setupMocks: func(idToken string, config *oauth.OAuthClientConfig) {
				suite.mockOIDCService.On("GetOAuthClientConfig", mock.Anything, testGoogleIDPID).
					Return(config, nil).Once()
			},
		},
		{
			name: "MissingIssuedAt",
			claims: map[string]interface{}{
				"iss": Issuer1,
				"aud": testClientID,
				"exp": float64(now.Add(1 * time.Hour).Unix()),
			},
			oAuthConfig: &oauth.OAuthClientConfig{
				ClientID:       testClientID,
				ClientSecret:   "test-secret",
				OAuthEndpoints: oauth.OAuthEndpoints{},
			},
			expectedErrorCode:   oidc.ErrorInvalidIDToken.Code,
			expectedErrContains: "iat",
			setupMocks: func(idToken string, config *oauth.OAuthClientConfig) {
				suite.mockOIDCService.On("GetOAuthClientConfig", mock.Anything, testGoogleIDPID).
					Return(config, nil).Once()
			},
		},
		{
			name: "InvalidIssuedAtFormat",
			claims: map[string]interface{}{
				"iss": Issuer1,
				"aud": testClientID,
				"exp": float64(now.Add(1 * time.Hour).Unix()),
				"iat": "invalid-iat",
			},
			oAuthConfig: &oauth.OAuthClientConfig{
				ClientID:       testClientID,
				ClientSecret:   "test-secret",
				OAuthEndpoints: oauth.OAuthEndpoints{},
			},
			expectedErrorCode:   oidc.ErrorInvalidIDToken.Code,
			expectedErrContains: "iat",
			setupMocks: func(idToken string, config *oauth.OAuthClientConfig) {
				suite.mockOIDCService.On("GetOAuthClientConfig", mock.Anything, testGoogleIDPID).
					Return(config, nil).Once()
			},
		},
		{
			name: "InvalidHostedDomain",
			claims: map[string]interface{}{
				"iss": Issuer1,
				"aud": testClientID,
				"exp": float64(now.Add(1 * time.Hour).Unix()),
				"iat": float64(now.Add(-1 * time.Minute).Unix()),
				"hd":  "wrongdomain.com",
			},
			oAuthConfig:         hostedDomainConfig,
			expectedErrorCode:   oidc.ErrorInvalidIDToken.Code,
			expectedErrContains: "hosted domain",
			setupMocks:          hostedDomainSetupMocks,
		},
		{
			name: "HostedDomainWrongType",
			claims: map[string]interface{}{
				"iss": Issuer1,
				"aud": testClientID,
				"exp": float64(now.Add(1 * time.Hour).Unix()),
				"iat": float64(now.Add(-1 * time.Minute).Unix()),
				"hd":  123,
			},
			oAuthConfig:         hostedDomainConfig,
			expectedErrorCode:   oidc.ErrorInvalidIDToken.Code,
			expectedErrContains: "hosted domain",
			setupMocks:          hostedDomainSetupMocks,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Generate token if claims are provided
			var idToken string
			if tc.idToken != "" {
				idToken = tc.idToken
			} else if tc.claims != nil {
				idToken = generateTestJWT(tc.claims)
			}

			// Setup mocks
			tc.setupMocks(idToken, tc.oAuthConfig)

			// Execute test
			err := suite.service.ValidateIDToken(context.Background(), testGoogleIDPID, idToken)

			// Assertions
			suite.NotNil(err)
			suite.Equal(tc.expectedErrorCode, err.Code)
			if tc.expectedErrContains != "" {
				suite.Contains(err.ErrorDescription.DefaultValue, tc.expectedErrContains)
			}
		})
	}
}

func (suite *GoogleOIDCAuthnServiceTestSuite) TestGetIDTokenClaimsSuccess() {
	// #nosec G101 - This is a test JWT token, not a hardcoded credential
	idToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI" +
		"6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
	claims := map[string]interface{}{
		"sub":  "1234567890",
		"name": "John Doe",
	}
	suite.mockOIDCService.On("GetIDTokenClaims", idToken).Return(claims, nil)

	result, err := suite.service.GetIDTokenClaims(idToken)
	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal("1234567890", result["sub"])
}

func (suite *GoogleOIDCAuthnServiceTestSuite) TestFetchUserInfoSuccess() {
	accessToken := "access_token"
	userInfo := map[string]interface{}{
		"sub":   "user123",
		"email": "user@gmail.com",
	}
	suite.mockOIDCService.On("FetchUserInfo", mock.Anything, testGoogleIDPID, accessToken).Return(userInfo, nil)

	result, err := suite.service.FetchUserInfo(context.Background(), testGoogleIDPID, accessToken)
	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(userInfo["sub"], result["sub"])
}

func (suite *GoogleOIDCAuthnServiceTestSuite) TestGetInternalUserSuccess() {
	sub := "user123"
	user := &entityprovider.Entity{
		ID:   "user123",
		Type: "person",
	}
	suite.mockOIDCService.On("GetInternalUser", sub).Return(user, nil)

	result, err := suite.service.GetInternalUser(sub)
	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(user.ID, result.ID)
}

// generateTestJWT creates a valid JWT token with the specified claims.
func generateTestJWT(claims map[string]interface{}) string {
	header := map[string]interface{}{
		"alg": "RS256",
		"typ": "JWT",
	}

	headerBytes, _ := json.Marshal(header)
	claimsBytes, _ := json.Marshal(claims)

	encodedHeader := base64.RawURLEncoding.EncodeToString(headerBytes)
	encodedClaims := base64.RawURLEncoding.EncodeToString(claimsBytes)
	signature := base64.RawURLEncoding.EncodeToString([]byte("fake-signature"))

	return encodedHeader + "." + encodedClaims + "." + signature
}

// ============================================================================
// Leeway Tests - Time-based claim validation with clock skew tolerance
// ============================================================================

func (suite *GoogleOIDCAuthnServiceTestSuite) TestValidateIDToken_Leeway_ExpiredWithinLeeway_ShouldPass() {
	now := time.Now()
	// Token expired 10 seconds ago, but leeway is 30 seconds - should pass
	claims := map[string]interface{}{
		"iss": Issuer1,
		"aud": testClientID,
		"sub": "user123",
		"exp": float64(now.Add(-10 * time.Second).Unix()), // Expired 10 seconds ago
		"iat": float64(now.Add(-1 * time.Hour).Unix()),
	}
	idToken := generateTestJWT(claims)

	oAuthConfig := &oauth.OAuthClientConfig{
		ClientID:       testClientID,
		ClientSecret:   "test-secret",
		OAuthEndpoints: oauth.OAuthEndpoints{},
	}

	suite.mockOIDCService.On("GetOAuthClientConfig", mock.Anything, testGoogleIDPID).Return(oAuthConfig, nil).Once()

	err := suite.service.ValidateIDToken(context.Background(), testGoogleIDPID, idToken)
	suite.Nil(err)
}

func (suite *GoogleOIDCAuthnServiceTestSuite) TestValidateIDToken_Leeway_ExpiredBeyondLeeway_ShouldFail() {
	now := time.Now()
	// Token expired 60 seconds ago, leeway is 30 seconds - should fail
	claims := map[string]interface{}{
		"iss": Issuer1,
		"aud": testClientID,
		"sub": "user123",
		"exp": float64(now.Add(-60 * time.Second).Unix()), // Expired 60 seconds ago
		"iat": float64(now.Add(-2 * time.Hour).Unix()),
	}
	idToken := generateTestJWT(claims)

	oAuthConfig := &oauth.OAuthClientConfig{
		ClientID:       testClientID,
		ClientSecret:   "test-secret",
		OAuthEndpoints: oauth.OAuthEndpoints{},
	}

	suite.mockOIDCService.On("GetOAuthClientConfig", mock.Anything, testGoogleIDPID).Return(oAuthConfig, nil).Once()

	err := suite.service.ValidateIDToken(context.Background(), testGoogleIDPID, idToken)
	suite.NotNil(err)
	suite.Equal(oidc.ErrorInvalidIDToken.Code, err.Code)
	suite.Contains(err.ErrorDescription.DefaultValue, "expired")
}

func (suite *GoogleOIDCAuthnServiceTestSuite) TestValidateIDToken_Leeway_IssuedInFutureWithinLeeway_ShouldPass() {
	now := time.Now()
	// Token iat is 10 seconds in future, but leeway is 30 seconds - should pass
	claims := map[string]interface{}{
		"iss": Issuer1,
		"aud": testClientID,
		"sub": "user123",
		"exp": float64(now.Add(1 * time.Hour).Unix()),
		"iat": float64(now.Add(10 * time.Second).Unix()), // Issued 10 seconds in future
	}
	idToken := generateTestJWT(claims)

	oAuthConfig := &oauth.OAuthClientConfig{
		ClientID:       testClientID,
		ClientSecret:   "test-secret",
		OAuthEndpoints: oauth.OAuthEndpoints{},
	}

	suite.mockOIDCService.On("GetOAuthClientConfig", mock.Anything, testGoogleIDPID).Return(oAuthConfig, nil).Once()

	err := suite.service.ValidateIDToken(context.Background(), testGoogleIDPID, idToken)
	suite.Nil(err)
}

func (suite *GoogleOIDCAuthnServiceTestSuite) TestValidateIDToken_Leeway_IssuedInFutureBeyondLeeway_ShouldFail() {
	now := time.Now()
	// Token iat is 60 seconds in future, leeway is 30 seconds - should fail
	claims := map[string]interface{}{
		"iss": Issuer1,
		"aud": testClientID,
		"sub": "user123",
		"exp": float64(now.Add(2 * time.Hour).Unix()),
		"iat": float64(now.Add(60 * time.Second).Unix()), // Issued 60 seconds in future
	}
	idToken := generateTestJWT(claims)

	oAuthConfig := &oauth.OAuthClientConfig{
		ClientID:       testClientID,
		ClientSecret:   "test-secret",
		OAuthEndpoints: oauth.OAuthEndpoints{},
	}

	suite.mockOIDCService.On("GetOAuthClientConfig", mock.Anything, testGoogleIDPID).Return(oAuthConfig, nil).Once()

	err := suite.service.ValidateIDToken(context.Background(), testGoogleIDPID, idToken)
	suite.NotNil(err)
	suite.Equal(oidc.ErrorInvalidIDToken.Code, err.Code)
	suite.Contains(err.ErrorDescription.DefaultValue, "future")
}

func (suite *GoogleOIDCAuthnServiceTestSuite) TestValidateIDToken_Leeway_ZeroLeeway_ExpiredShouldFail() {
	// Reset and reinitialize with zero leeway
	config.ResetServerRuntime()
	testConfig := &config.Config{
		JWT: config.JWTConfig{
			Leeway: 0, // No leeway
		},
	}
	_ = config.InitializeServerRuntime("test", testConfig)

	now := time.Now()
	// Token expired 1 second ago - should fail with zero leeway
	claims := map[string]interface{}{
		"iss": Issuer1,
		"aud": testClientID,
		"sub": "user123",
		"exp": float64(now.Add(-1 * time.Second).Unix()), // Expired 1 second ago
		"iat": float64(now.Add(-1 * time.Hour).Unix()),
	}
	idToken := generateTestJWT(claims)

	oAuthConfig := &oauth.OAuthClientConfig{
		ClientID:       testClientID,
		ClientSecret:   "test-secret",
		OAuthEndpoints: oauth.OAuthEndpoints{},
	}

	suite.mockOIDCService.On("GetOAuthClientConfig", mock.Anything, testGoogleIDPID).Return(oAuthConfig, nil).Once()

	err := suite.service.ValidateIDToken(context.Background(), testGoogleIDPID, idToken)
	suite.NotNil(err)
	suite.Equal(oidc.ErrorInvalidIDToken.Code, err.Code)
	suite.Contains(err.ErrorDescription.DefaultValue, "expired")
}

func (suite *GoogleOIDCAuthnServiceTestSuite) TestValidateIDToken_Leeway_IatExactlyAtBoundary_ShouldPass() {
	// Reset and reinitialize with 30 second leeway
	config.ResetServerRuntime()
	testConfig := &config.Config{
		JWT: config.JWTConfig{
			Leeway: 30, // 30 seconds leeway
		},
	}
	_ = config.InitializeServerRuntime("test", testConfig)

	now := time.Now()
	// Token iat is exactly at leeway boundary (now + 30 seconds)
	// Condition: time.Now().Unix() < int64(iat) - leeway
	// = now < (now + 30) - 30 = now < now = FALSE (should pass)
	claims := map[string]interface{}{
		"iss": Issuer1,
		"aud": testClientID,
		"sub": "user123",
		"exp": float64(now.Add(1 * time.Hour).Unix()),
		"iat": float64(now.Add(30 * time.Second).Unix()), // Exactly at boundary
	}
	idToken := generateTestJWT(claims)

	oAuthConfig := &oauth.OAuthClientConfig{
		ClientID:       testClientID,
		ClientSecret:   "test-secret",
		OAuthEndpoints: oauth.OAuthEndpoints{},
	}

	suite.mockOIDCService.On("GetOAuthClientConfig", mock.Anything, testGoogleIDPID).Return(oAuthConfig, nil).Once()

	err := suite.service.ValidateIDToken(context.Background(), testGoogleIDPID, idToken)
	suite.Nil(err)
}

func (suite *GoogleOIDCAuthnServiceTestSuite) TestValidateIDToken_Leeway_IatJustBeyondBoundary_ShouldFail() {
	// Reset and reinitialize with 30 second leeway
	config.ResetServerRuntime()
	testConfig := &config.Config{
		JWT: config.JWTConfig{
			Leeway: 30, // 30 seconds leeway
		},
	}
	_ = config.InitializeServerRuntime("test", testConfig)

	now := time.Now()
	// Token iat is just beyond leeway boundary (now + 31 seconds)
	// Condition: time.Now().Unix() < int64(iat) - leeway
	// = now < (now + 31) - 30 = now < now + 1 = TRUE (should fail)
	claims := map[string]interface{}{
		"iss": Issuer1,
		"aud": testClientID,
		"sub": "user123",
		"exp": float64(now.Add(2 * time.Hour).Unix()),
		"iat": float64(now.Add(31 * time.Second).Unix()), // Just beyond boundary
	}
	idToken := generateTestJWT(claims)

	oAuthConfig := &oauth.OAuthClientConfig{
		ClientID:       testClientID,
		ClientSecret:   "test-secret",
		OAuthEndpoints: oauth.OAuthEndpoints{},
	}

	suite.mockOIDCService.On("GetOAuthClientConfig", mock.Anything, testGoogleIDPID).Return(oAuthConfig, nil).Once()

	err := suite.service.ValidateIDToken(context.Background(), testGoogleIDPID, idToken)
	suite.NotNil(err)
	suite.Equal(oidc.ErrorInvalidIDToken.Code, err.Code)
	suite.Contains(err.ErrorDescription.DefaultValue, "future")
}
