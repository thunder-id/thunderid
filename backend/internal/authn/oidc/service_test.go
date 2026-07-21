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

package oidc

import (
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"context"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/stretchr/testify/suite"

	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/authn/oauth"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/tests/mocks/authn/oauthmock"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
)

const (
	testOIDCIDPID = "idp123"
)

type OIDCAuthnServiceTestSuite struct {
	suite.Suite
	mockOAuthService *oauthmock.OAuthAuthnServiceInterfaceMock
	mockJWTService   *jwtmock.JWTServiceInterfaceMock
	endpoints        oauth.OAuthEndpoints
	service          oidcAuthnService
}

func TestOIDCAuthnServiceTestSuite(t *testing.T) {
	suite.Run(t, new(OIDCAuthnServiceTestSuite))
}

func (suite *OIDCAuthnServiceTestSuite) SetupTest() {
	suite.mockOAuthService = oauthmock.NewOAuthAuthnServiceInterfaceMock(suite.T())
	suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())
	suite.endpoints = oauth.OAuthEndpoints{
		AuthorizationEndpoint: "https://localhost:8090/oauth/authorize",
		TokenEndpoint:         "https://localhost:8090/oauth/token",
		UserInfoEndpoint:      "https://localhost:8090/oauth/userinfo",
	}

	service := newOIDCAuthnService(suite.mockOAuthService, suite.mockJWTService)

	cast, ok := service.(*oidcAuthnService)
	suite.True(ok, "service is not of type *oidcAuthnService")
	suite.service = *cast
}

func (suite *OIDCAuthnServiceTestSuite) TestGetOAuthClientConfigWithOpenIDScope() {
	idpID := testOIDCIDPID
	config := &oauth.OAuthClientConfig{
		ClientID:     "client123",
		ClientSecret: "secret",
		Scopes:       []string{"openid", "profile", "email"},
	}
	suite.mockOAuthService.On("GetOAuthClientConfig", mock.Anything, idpID).Return(config, nil)

	result, err := suite.service.GetOAuthClientConfig(context.Background(), idpID)
	suite.Nil(err)
	suite.NotNil(result)

	suite.Contains(result.Scopes, "openid")
	suite.Contains(result.Scopes, "profile")
	suite.Contains(result.Scopes, "email")

	// Ensure openid is not duplicated
	suite.Equal(3, len(result.Scopes))
}

func (suite *OIDCAuthnServiceTestSuite) TestGetOAuthClientConfigWithoutOpenIDScope() {
	idpID := testOIDCIDPID
	config := &oauth.OAuthClientConfig{
		ClientID:     "client123",
		ClientSecret: "secret",
		Scopes:       []string{"profile"},
	}
	suite.mockOAuthService.On("GetOAuthClientConfig", mock.Anything, idpID).Return(config, nil)

	result, err := suite.service.GetOAuthClientConfig(context.Background(), idpID)
	suite.Nil(err)
	suite.NotNil(result)

	// Scopes come from IDP config as-is, no automatic addition of openid scope
	suite.NotContains(result.Scopes, "openid")
	suite.Contains(result.Scopes, "profile")
}

func (suite *OIDCAuthnServiceTestSuite) TestBuildAuthorizeURLSuccess() {
	expectedURL := "https://example.com/authorize?client_id=test"
	suite.mockOAuthService.On("BuildAuthorizeURL", mock.Anything, testOIDCIDPID).
		Return(expectedURL, map[string]string{oauth2const.RequestParamState: "test-state"}, nil)

	url, metadata, err := suite.service.BuildAuthorizeURL(context.Background(), testOIDCIDPID)
	suite.Nil(err)
	suite.Contains(url, expectedURL)
	suite.NotEmpty(metadata[oauth2const.RequestParamState])
	suite.NotEmpty(metadata[oauth2const.RequestParamNonce])
}

func (suite *OIDCAuthnServiceTestSuite) TestBuildAuthorizeURLError() {
	svcErr := &tidcommon.ServiceError{
		Code: "ERROR",
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.test.failed_to_build_url", DefaultValue: "Failed to build URL",
		},
	}
	suite.mockOAuthService.On("BuildAuthorizeURL", mock.Anything, testOIDCIDPID).
		Return("", (map[string]string)(nil), svcErr)

	url, metadata, err := suite.service.BuildAuthorizeURL(context.Background(), testOIDCIDPID)
	suite.Empty(url)
	suite.Nil(metadata)
	suite.NotNil(err)
	suite.Equal(svcErr.Code, err.Code)
}

func (suite *OIDCAuthnServiceTestSuite) TestExchangeCodeForTokenSuccess() {
	tests := []struct {
		name             string
		validateResponse bool
		setupMocks       func()
	}{
		{
			name:             "WithValidation",
			validateResponse: true,
			setupMocks: func() {
				code := "auth_code"
				tokenResp := &oauth.TokenResponse{
					AccessToken: "access_token",
					IDToken:     "id_token",
					TokenType:   "Bearer",
				}
				suite.mockOAuthService.On("ExchangeCodeForToken", mock.Anything, testOIDCIDPID, code, false).
					Return(tokenResp, nil)
				cfg := &oauth.OAuthClientConfig{
					OAuthEndpoints: oauth.OAuthEndpoints{JwksEndpoint: "https://example.com/jwks"},
				}
				suite.mockOAuthService.On("GetOAuthClientConfig", mock.Anything, testOIDCIDPID).
					Return(cfg, nil)
				suite.mockJWTService.On("VerifyJWTWithJWKS", mock.Anything, "id_token",
					"https://example.com/jwks", "", "").Return(nil)
			},
		},
		{
			name:             "WithoutValidation",
			validateResponse: false,
			setupMocks: func() {
				code := "auth_code"
				tokenResp := &oauth.TokenResponse{
					AccessToken: "access_token",
					IDToken:     "id_token",
					TokenType:   "Bearer",
				}
				suite.mockOAuthService.On("ExchangeCodeForToken", mock.Anything, testOIDCIDPID, code, false).
					Return(tokenResp, nil)
			},
		},
	}

	for _, tc := range tests {
		suite.Run(tc.name, func() {
			suite.mockOAuthService = oauthmock.NewOAuthAuthnServiceInterfaceMock(suite.T())
			suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())

			service := newOIDCAuthnService(suite.mockOAuthService, suite.mockJWTService)
			cast, ok := service.(*oidcAuthnService)
			suite.True(ok, "service is not of type *oidcAuthnService")
			suite.service = *cast

			tc.setupMocks()

			result, err := suite.service.ExchangeCodeForToken(context.Background(), testOIDCIDPID, "auth_code",
				tc.validateResponse)

			suite.Nil(err)
			suite.NotNil(result)
			suite.Equal("access_token", result.AccessToken)
		})
	}
}

func (suite *OIDCAuthnServiceTestSuite) TestValidateTokenResponseSuccess() {
	tests := []struct {
		name            string
		validateIDToken bool
		setupMocks      func()
	}{
		{
			name:            "WithIDTokenValidation",
			validateIDToken: true,
			setupMocks: func() {
				cfg := &oauth.OAuthClientConfig{
					OAuthEndpoints: oauth.OAuthEndpoints{JwksEndpoint: "https://example.com/jwks"},
				}
				suite.mockOAuthService.On("GetOAuthClientConfig", mock.Anything, testOIDCIDPID).
					Return(cfg, nil)
				suite.mockJWTService.On("VerifyJWTWithJWKS", mock.Anything, "id_token",
					"https://example.com/jwks", "", "").Return(nil)
			},
		},
		{
			name:            "WithoutIDTokenValidation",
			validateIDToken: false,
			setupMocks:      func() {},
		},
	}

	for _, tc := range tests {
		suite.Run(tc.name, func() {
			suite.mockOAuthService = oauthmock.NewOAuthAuthnServiceInterfaceMock(suite.T())
			suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())

			service := newOIDCAuthnService(suite.mockOAuthService, suite.mockJWTService)
			cast, ok := service.(*oidcAuthnService)
			suite.True(ok, "service is not of type *oidcAuthnService")
			suite.service = *cast

			tc.setupMocks()

			tokenResp := &oauth.TokenResponse{
				AccessToken: "access_token",
				IDToken:     "id_token",
				TokenType:   "Bearer",
			}
			err := suite.service.ValidateTokenResponse(
				context.Background(), testOIDCIDPID, tokenResp, tc.validateIDToken)
			suite.Nil(err)
		})
	}
}

func (suite *OIDCAuthnServiceTestSuite) TestValidateTokenResponseWithError() {
	tests := []struct {
		name string
		resp *oauth.TokenResponse
	}{
		{
			name: "NilResponse",
			resp: nil,
		},
		{
			name: "EmptyAccessToken",
			resp: &oauth.TokenResponse{AccessToken: "", IDToken: "id_token"},
		},
		{
			name: "EmptyIDToken",
			resp: &oauth.TokenResponse{AccessToken: "access_token", IDToken: ""},
		},
	}

	for _, tc := range tests {
		suite.Run(tc.name, func() {
			err := suite.service.ValidateTokenResponse(context.Background(), testOIDCIDPID, tc.resp, false)
			suite.NotNil(err)
			suite.Equal(oauth.ErrorInvalidTokenResponse.Code, err.Code)
		})
	}
}

func (suite *OIDCAuthnServiceTestSuite) TestValidateIDTokenSuccess() {
	tests := []struct {
		name       string
		setupMocks func()
	}{
		{
			name: "WithJWKSEndpoint",
			setupMocks: func() {
				cfg := &oauth.OAuthClientConfig{
					OAuthEndpoints: oauth.OAuthEndpoints{JwksEndpoint: "https://example.com/jwks"},
				}
				suite.mockOAuthService.On("GetOAuthClientConfig", mock.Anything, testOIDCIDPID).
					Return(cfg, nil)
				suite.mockJWTService.On("VerifyJWTWithJWKS", mock.Anything, "valid_id_token",
					"https://example.com/jwks", "", "").Return(nil)
			},
		},
		{
			name: "WithoutJWKSEndpoint",
			setupMocks: func() {
				cfg := &oauth.OAuthClientConfig{
					OAuthEndpoints: oauth.OAuthEndpoints{},
				}
				suite.mockOAuthService.On("GetOAuthClientConfig", mock.Anything, testOIDCIDPID).
					Return(cfg, nil)
			},
		},
	}

	for _, tc := range tests {
		suite.Run(tc.name, func() {
			suite.mockOAuthService = oauthmock.NewOAuthAuthnServiceInterfaceMock(suite.T())
			suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())

			service := newOIDCAuthnService(suite.mockOAuthService, suite.mockJWTService)
			cast, ok := service.(*oidcAuthnService)
			suite.True(ok, "service is not of type *oidcAuthnService")
			suite.service = *cast

			tc.setupMocks()

			err := suite.service.ValidateIDToken(context.Background(), testOIDCIDPID, "valid_id_token")
			suite.Nil(err)
		})
	}
}

func (suite *OIDCAuthnServiceTestSuite) TestValidateIDTokenEmptyToken() {
	err := suite.service.ValidateIDToken(context.Background(), testOIDCIDPID, "")
	suite.NotNil(err)
	suite.Equal(ErrorInvalidIDToken.Code, err.Code)
}

func (suite *OIDCAuthnServiceTestSuite) TestGetIDTokenClaimsSuccess() {
	// Create a valid JWT token (base64 encoded header.payload.signature)
	idToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9." +
		"eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ." +
		"SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"

	claims, err := suite.service.GetIDTokenClaims(context.Background(), idToken)
	suite.Nil(err)
	suite.NotNil(claims)
	suite.Equal("1234567890", claims["sub"])
}

func (suite *OIDCAuthnServiceTestSuite) TestGetIDTokenClaimsEmptyToken() {
	claims, err := suite.service.GetIDTokenClaims(context.Background(), "")
	suite.Nil(claims)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidIDToken.Code, err.Code)
}

func (suite *OIDCAuthnServiceTestSuite) TestFetchUserInfoSuccess() {
	accessToken := "access_token"
	userInfo := map[string]interface{}{
		"sub":   "user123",
		"email": "user@example.com",
	}
	suite.mockOAuthService.On("FetchUserInfo", mock.Anything, testOIDCIDPID, accessToken).Return(userInfo, nil)

	result, err := suite.service.FetchUserInfo(context.Background(), testOIDCIDPID, accessToken)
	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(userInfo["sub"], result["sub"])
}

func (suite *OIDCAuthnServiceTestSuite) TestExchangeCodeForTokenInternalError() {
	suite.mockOAuthService.On("ExchangeCodeForToken", mock.Anything, testOIDCIDPID, "auth_code", false).
		Return(nil, &tidcommon.ServiceError{Code: "INT-ERR"})

	result, err := suite.service.ExchangeCodeForToken(context.Background(), testOIDCIDPID, "auth_code", false)
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal("INT-ERR", err.Code)
}

func (suite *OIDCAuthnServiceTestSuite) TestValidateTokenResponseValidateIDTokenFailure() {
	// Setup: Token response valid but ValidateIDToken will fail due to signature verification error
	suite.mockOAuthService = oauthmock.NewOAuthAuthnServiceInterfaceMock(suite.T())
	suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())

	service := newOIDCAuthnService(suite.mockOAuthService, suite.mockJWTService)
	cast, ok := service.(*oidcAuthnService)
	suite.True(ok)
	suite.service = *cast

	// GetOAuthClientConfig returns a config with jwks endpoint
	suite.mockOAuthService.On("GetOAuthClientConfig", mock.Anything, testOIDCIDPID).Return(&oauth.OAuthClientConfig{
		OAuthEndpoints: oauth.OAuthEndpoints{JwksEndpoint: "https://example.com/jwks"},
	}, nil)

	// jwt service fails verification
	suite.mockJWTService.On("VerifyJWTWithJWKS", mock.Anything, "id_token", "https://example.com/jwks", "", "").
		Return(&tidcommon.ServiceError{
			Type: tidcommon.ServerErrorType,
			Code: "SIGNATURE_INVALID",
			Error: tidcommon.I18nMessage{
				Key: "error.test.signature_invalid", DefaultValue: "Signature invalid",
			},
			ErrorDescription: tidcommon.I18nMessage{
				Key: "error.test.signature_invalid", DefaultValue: "signature invalid",
			},
		})

	tokenResp := &oauth.TokenResponse{AccessToken: "access", IDToken: "id_token"}
	err := suite.service.ValidateTokenResponse(context.Background(), testOIDCIDPID, tokenResp, true)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidIDTokenSignature.Code, err.Code)
}

func (suite *OIDCAuthnServiceTestSuite) TestGetIDTokenClaimsMalformedToken() {
	// Malformed token (not three parts) should return invalid token error
	claims, err := suite.service.GetIDTokenClaims(context.Background(), "not.a.valid.token")
	suite.Nil(claims)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidIDToken.Code, err.Code)
}

func (suite *OIDCAuthnServiceTestSuite) TestValidateIDTokenWithJWKSEndpoint() {
	// Test that JWKS endpoint is used when configured
	idToken := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ1c2VyMTIzI" +
		"iwibmFtZSI6IlRlc3QgVXNlciIsImlhdCI6MTUxNjIzOTAyMn0.signature"

	config := &oauth.OAuthClientConfig{
		ClientID:     "test_client",
		ClientSecret: "test_secret",
		RedirectURI:  "https://app.com/callback",
		Scopes:       []string{"openid"},
		OAuthEndpoints: oauth.OAuthEndpoints{
			JwksEndpoint: "https://idp.com/jwks",
		},
	}

	suite.mockOAuthService.On("GetOAuthClientConfig", mock.Anything, testOIDCIDPID).Return(config, nil)
	suite.mockJWTService.On("VerifyJWTWithJWKS", mock.Anything, idToken, "https://idp.com/jwks", "", "").Return(nil)

	err := suite.service.ValidateIDToken(context.Background(), testOIDCIDPID, idToken)
	suite.Nil(err)
}

func (suite *OIDCAuthnServiceTestSuite) TestValidateIDTokenWithoutJWKSEndpoint() {
	// Test that validation succeeds when JWKS endpoint is not configured (skips signature validation)
	idToken := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ1c2VyMTIzIiw" +
		"ibmFtZSI6IlRlc3QgVXNlciIsImlhdCI6MTUxNjIzOTAyMn0.signature"

	config := &oauth.OAuthClientConfig{
		ClientID:     "test_client",
		ClientSecret: "test_secret",
		RedirectURI:  "https://app.com/callback",
		Scopes:       []string{"openid"},
		OAuthEndpoints: oauth.OAuthEndpoints{
			JwksEndpoint: "", // Empty JWKS endpoint
		},
	}

	suite.mockOAuthService.On("GetOAuthClientConfig", mock.Anything, testOIDCIDPID).Return(config, nil)
	// VerifyJWTWithJWKS should not be called when JWKS endpoint is empty

	err := suite.service.ValidateIDToken(context.Background(), testOIDCIDPID, idToken)
	suite.Nil(err)
}

func (suite *OIDCAuthnServiceTestSuite) TestAuthenticateSuccess() {
	suite.mockOAuthService = oauthmock.NewOAuthAuthnServiceInterfaceMock(suite.T())
	suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())

	service := newOIDCAuthnService(suite.mockOAuthService, suite.mockJWTService)
	cast, ok := service.(*oidcAuthnService)
	suite.True(ok)
	suite.service = *cast

	idToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9." +
		"eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0Ijox" +
		"NTE2MjM5MDIyLCJub25jZSI6InRlc3Qtbm9uY2UtdmFsdWUifQ." +
		"fake_sig"
	tokenResp := &oauth.TokenResponse{AccessToken: "access_token", IDToken: idToken, TokenType: "Bearer"}
	suite.mockOAuthService.On("ExchangeCodeForToken", mock.Anything, testOIDCIDPID, "auth_code", false).
		Return(tokenResp, nil)
	cfg := &oauth.OAuthClientConfig{
		OAuthEndpoints: oauth.OAuthEndpoints{JwksEndpoint: "https://example.com/jwks"},
		Scopes:         []string{"openid"},
	}
	suite.mockOAuthService.On("GetOAuthClientConfig", mock.Anything, testOIDCIDPID).Return(cfg, nil)
	suite.mockJWTService.On("VerifyJWTWithJWKS", mock.Anything, idToken, "https://example.com/jwks", "", "").Return(nil)
	suite.mockOAuthService.On("BuildFederatedAuthResult", mock.Anything, testOIDCIDPID, "1234567890", mock.Anything).
		Return(&authncm.AuthnResult{
			Token:               map[string]interface{}{"sub": "1234567890"},
			AuthenticatedClaims: map[string]interface{}{"sub": "1234567890", "name": "John Doe"},
		}, nil)

	result, svcErr := suite.service.Authenticate(context.Background(), testOIDCIDPID,
		authncm.AuthorizationData{Code: "auth_code", Nonce: "test-nonce-value"})
	suite.Nil(svcErr)
	suite.NotNil(result)
	suite.Equal("1234567890", result.Token["sub"])
	suite.Equal("John Doe", result.AuthenticatedClaims["name"])
}

func (suite *OIDCAuthnServiceTestSuite) TestAuthenticateWithUserInfoMerge() {
	suite.mockOAuthService = oauthmock.NewOAuthAuthnServiceInterfaceMock(suite.T())
	suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())

	service := newOIDCAuthnService(suite.mockOAuthService, suite.mockJWTService)
	cast, ok := service.(*oidcAuthnService)
	suite.True(ok)
	suite.service = *cast

	idToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9." +
		"eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0Ijox" +
		"NTE2MjM5MDIyLCJub25jZSI6InRlc3Qtbm9uY2UtdmFsdWUifQ." +
		"fake_sig"
	tokenResp := &oauth.TokenResponse{AccessToken: "access_token", IDToken: idToken, TokenType: "Bearer"}
	suite.mockOAuthService.On("ExchangeCodeForToken", mock.Anything, testOIDCIDPID, "auth_code", false).
		Return(tokenResp, nil)
	cfg := &oauth.OAuthClientConfig{
		OAuthEndpoints: oauth.OAuthEndpoints{JwksEndpoint: "https://example.com/jwks"},
		Scopes:         []string{"openid", "profile"},
	}
	suite.mockOAuthService.On("GetOAuthClientConfig", mock.Anything, testOIDCIDPID).Return(cfg, nil)
	suite.mockJWTService.On("VerifyJWTWithJWKS", mock.Anything, idToken, "https://example.com/jwks", "", "").Return(nil)
	suite.mockOAuthService.On("FetchUserInfo", mock.Anything, testOIDCIDPID, "access_token").Return(
		map[string]interface{}{"sub": "1234567890", "email": "john@example.com"}, nil)
	// The merged claims (ID token + userinfo) must be passed to BuildFederatedAuthResult.
	suite.mockOAuthService.On("BuildFederatedAuthResult", mock.Anything, testOIDCIDPID, "1234567890",
		mock.MatchedBy(func(c map[string]interface{}) bool {
			return c["email"] == "john@example.com" && c["name"] == "John Doe"
		})).
		Return(&authncm.AuthnResult{
			Token:               map[string]interface{}{"sub": "1234567890"},
			AuthenticatedClaims: map[string]interface{}{"email": "john@example.com", "name": "John Doe"},
		}, nil)

	result, svcErr := suite.service.Authenticate(context.Background(), testOIDCIDPID,
		authncm.AuthorizationData{Code: "auth_code", Nonce: "test-nonce-value"})
	suite.Nil(svcErr)
	suite.NotNil(result)
	suite.Equal("john@example.com", result.AuthenticatedClaims["email"])
	suite.Equal("John Doe", result.AuthenticatedClaims["name"])
}

func (suite *OIDCAuthnServiceTestSuite) TestAuthenticateUserInfoSubMismatch() {
	suite.mockOAuthService = oauthmock.NewOAuthAuthnServiceInterfaceMock(suite.T())
	suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())

	service := newOIDCAuthnService(suite.mockOAuthService, suite.mockJWTService)
	cast, ok := service.(*oidcAuthnService)
	suite.True(ok)
	suite.service = *cast

	idToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9." +
		"eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0Ijox" +
		"NTE2MjM5MDIyLCJub25jZSI6InRlc3Qtbm9uY2UtdmFsdWUifQ." +
		"fake_sig"
	tokenResp := &oauth.TokenResponse{AccessToken: "access_token", IDToken: idToken, TokenType: "Bearer"}
	suite.mockOAuthService.On("ExchangeCodeForToken", mock.Anything, testOIDCIDPID, "auth_code", false).
		Return(tokenResp, nil)
	cfg := &oauth.OAuthClientConfig{
		OAuthEndpoints: oauth.OAuthEndpoints{JwksEndpoint: "https://example.com/jwks"},
		Scopes:         []string{"openid", "profile"},
	}
	suite.mockOAuthService.On("GetOAuthClientConfig", mock.Anything, testOIDCIDPID).Return(cfg, nil)
	suite.mockJWTService.On("VerifyJWTWithJWKS", mock.Anything, idToken, "https://example.com/jwks", "", "").Return(nil)
	suite.mockOAuthService.On("FetchUserInfo", mock.Anything, testOIDCIDPID, "access_token").Return(
		map[string]interface{}{"sub": "different_user", "email": "other@example.com"}, nil)
	// UserInfo sub mismatch → merge skipped, so email must NOT reach BuildFederatedAuthResult.
	suite.mockOAuthService.On("BuildFederatedAuthResult", mock.Anything, testOIDCIDPID, "1234567890",
		mock.MatchedBy(func(c map[string]interface{}) bool { _, ok := c["email"]; return !ok })).
		Return(&authncm.AuthnResult{Token: map[string]interface{}{"sub": "1234567890"}}, nil)

	result, svcErr := suite.service.Authenticate(context.Background(), testOIDCIDPID,
		authncm.AuthorizationData{Code: "auth_code", Nonce: "test-nonce-value"})
	suite.Nil(svcErr)
	suite.NotNil(result)
	suite.Equal("1234567890", result.Token["sub"])
}

func (suite *OIDCAuthnServiceTestSuite) TestAuthenticateExchangeCodeError() {
	suite.mockOAuthService = oauthmock.NewOAuthAuthnServiceInterfaceMock(suite.T())
	suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())

	service := newOIDCAuthnService(suite.mockOAuthService, suite.mockJWTService)
	cast, ok := service.(*oidcAuthnService)
	suite.True(ok)
	suite.service = *cast

	suite.mockOAuthService.On("ExchangeCodeForToken", mock.Anything, testOIDCIDPID, "bad_code", false).
		Return(nil, &tidcommon.ServiceError{Code: "TOKEN-ERR"})

	result, svcErr := suite.service.Authenticate(context.Background(), testOIDCIDPID,
		authncm.AuthorizationData{Code: "bad_code"})
	suite.Nil(result)
	suite.NotNil(svcErr)
	suite.Equal("TOKEN-ERR", svcErr.Code)
}

func (suite *OIDCAuthnServiceTestSuite) TestAuthenticateSubClaimNotFound() {
	suite.mockOAuthService = oauthmock.NewOAuthAuthnServiceInterfaceMock(suite.T())
	suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())

	service := newOIDCAuthnService(suite.mockOAuthService, suite.mockJWTService)
	cast, ok := service.(*oidcAuthnService)
	suite.True(ok)
	suite.service = *cast

	idToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9." +
		"eyJuYW1lIjoiSm9obiBEb2UiLCJpYXQiOjE1MTYyMzkwMjIsIm5vbmNlIjoidGVzdC1ub25jZS12YWx1ZSJ9." +
		"fake_sig"
	tokenResp := &oauth.TokenResponse{AccessToken: "access_token", IDToken: idToken, TokenType: "Bearer"}
	suite.mockOAuthService.On("ExchangeCodeForToken", mock.Anything, testOIDCIDPID, "auth_code", false).
		Return(tokenResp, nil)
	cfg := &oauth.OAuthClientConfig{OAuthEndpoints: oauth.OAuthEndpoints{JwksEndpoint: "https://example.com/jwks"}}
	suite.mockOAuthService.On("GetOAuthClientConfig", mock.Anything, testOIDCIDPID).Return(cfg, nil)
	suite.mockJWTService.On("VerifyJWTWithJWKS", mock.Anything, idToken, "https://example.com/jwks", "", "").Return(nil)

	result, svcErr := suite.service.Authenticate(context.Background(), testOIDCIDPID,
		authncm.AuthorizationData{Code: "auth_code", Nonce: "test-nonce-value"})
	suite.Nil(result)
	suite.NotNil(svcErr)
	suite.Equal(authncm.ErrorSubClaimNotFound.Code, svcErr.Code)
}

func (suite *OIDCAuthnServiceTestSuite) TestAuthenticateUserInfoFetchError() {
	suite.mockOAuthService = oauthmock.NewOAuthAuthnServiceInterfaceMock(suite.T())
	suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())

	service := newOIDCAuthnService(suite.mockOAuthService, suite.mockJWTService)
	cast, ok := service.(*oidcAuthnService)
	suite.True(ok)
	suite.service = *cast

	idToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9." +
		"eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0Ijox" +
		"NTE2MjM5MDIyLCJub25jZSI6InRlc3Qtbm9uY2UtdmFsdWUifQ." +
		"fake_sig"
	tokenResp := &oauth.TokenResponse{AccessToken: "access_token", IDToken: idToken, TokenType: "Bearer"}
	suite.mockOAuthService.On("ExchangeCodeForToken", mock.Anything, testOIDCIDPID, "auth_code", false).
		Return(tokenResp, nil)
	cfg := &oauth.OAuthClientConfig{
		OAuthEndpoints: oauth.OAuthEndpoints{JwksEndpoint: "https://example.com/jwks"},
		Scopes:         []string{"openid", "profile"},
	}
	suite.mockOAuthService.On("GetOAuthClientConfig", mock.Anything, testOIDCIDPID).Return(cfg, nil)
	suite.mockJWTService.On("VerifyJWTWithJWKS", mock.Anything, idToken, "https://example.com/jwks", "", "").Return(nil)
	suite.mockOAuthService.On("FetchUserInfo", mock.Anything, testOIDCIDPID, "access_token").Return(
		nil, &tidcommon.ServiceError{Code: "USERINFO-ERR"})
	// UserInfo fetch failed → claims unchanged; the flow still proceeds with the ID token claims.
	suite.mockOAuthService.On("BuildFederatedAuthResult", mock.Anything, testOIDCIDPID, "1234567890", mock.Anything).
		Return(&authncm.AuthnResult{Token: map[string]interface{}{"sub": "1234567890"}}, nil)

	result, svcErr := suite.service.Authenticate(context.Background(), testOIDCIDPID,
		authncm.AuthorizationData{Code: "auth_code", Nonce: "test-nonce-value"})
	suite.Nil(svcErr)
	suite.NotNil(result)
	suite.Equal("1234567890", result.Token["sub"])
}

func (suite *OIDCAuthnServiceTestSuite) TestAuthenticateNonceValidation() {
	// Token with nonce "test-nonce-123".
	nonceTokenPayload := "eyJzdWIiOiAiMTIzNDU2Nzg5MCIsICJuYW1lIjogIkpvaG4gRG9lIiwg" +
		"ImlhdCI6IDE1MTYyMzkwMjIsICJub25jZSI6ICJ0ZXN0LW5vbmNlLTEyMyJ9"
	nonceToken := "eyJhbGciOiAiSFMyNTYiLCAidHlwIjogIkpXVCJ9." + nonceTokenPayload + ".fake_sig"

	// Token without nonce claim: {"sub":"1234567890","name":"John Doe","iat":1516239022}
	noNonceToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9." +
		"eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.fake_sig"

	// Token with empty nonce: {"sub":"1234567890","name":"John Doe","iat":1516239022,"nonce":""}
	emptyNonceToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9." +
		"eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyLCJub25jZSI6IiJ9.fake_sig"

	testCases := []struct {
		name          string
		idToken       string
		authzNonce    string
		expectSuccess bool
		expectedError string
	}{
		{
			name:          "MatchingNonce",
			idToken:       nonceToken,
			authzNonce:    "test-nonce-123",
			expectSuccess: true,
		},
		{
			name:          "NonceMissingInIDToken",
			idToken:       noNonceToken,
			authzNonce:    "test-nonce-123",
			expectedError: tidcommon.InternalServerError.Code,
		},
		{
			name:          "NonceEmptyInIDToken",
			idToken:       emptyNonceToken,
			authzNonce:    "test-nonce-123",
			expectedError: tidcommon.InternalServerError.Code,
		},
		{
			name:          "NonceMissingInAuthzData",
			idToken:       nonceToken,
			authzNonce:    "",
			expectedError: tidcommon.InternalServerError.Code,
		},
		{
			name:          "NonceMismatch",
			idToken:       nonceToken,
			authzNonce:    "wrong-nonce",
			expectedError: tidcommon.InternalServerError.Code,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.mockOAuthService = oauthmock.NewOAuthAuthnServiceInterfaceMock(suite.T())
			suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())

			service := newOIDCAuthnService(suite.mockOAuthService, suite.mockJWTService)
			cast, ok := service.(*oidcAuthnService)
			suite.True(ok)
			suite.service = *cast

			tokenResp := &oauth.TokenResponse{
				AccessToken: "access_token", IDToken: tc.idToken, TokenType: "Bearer",
			}
			suite.mockOAuthService.On("ExchangeCodeForToken",
				mock.Anything, testOIDCIDPID, "auth_code", false).Return(tokenResp, nil)
			cfg := &oauth.OAuthClientConfig{
				OAuthEndpoints: oauth.OAuthEndpoints{JwksEndpoint: "https://example.com/jwks"},
				Scopes:         []string{"openid"},
			}
			suite.mockOAuthService.On("GetOAuthClientConfig",
				mock.Anything, testOIDCIDPID).Return(cfg, nil)
			suite.mockJWTService.On("VerifyJWTWithJWKS",
				mock.Anything, tc.idToken, "https://example.com/jwks", "", "").Return(nil)

			if tc.expectSuccess {
				suite.mockOAuthService.On("BuildFederatedAuthResult",
					mock.Anything, testOIDCIDPID, "1234567890", mock.Anything).
					Return(&authncm.AuthnResult{
						Token: map[string]interface{}{"sub": "1234567890"},
					}, nil)
			}

			result, svcErr := suite.service.Authenticate(context.Background(), testOIDCIDPID,
				authncm.AuthorizationData{Code: "auth_code", Nonce: tc.authzNonce})

			if tc.expectSuccess {
				suite.Nil(svcErr)
				suite.NotNil(result)
				suite.Equal("1234567890", result.Token["sub"])
			} else {
				suite.Nil(result)
				suite.NotNil(svcErr)
				suite.Equal(tc.expectedError, svcErr.Code)
			}
		})
	}
}

func (suite *OIDCAuthnServiceTestSuite) TestBuildFederatedAuthResultDelegates() {
	expected := &authncm.AuthnResult{
		Token:               map[string]interface{}{"email": "user@example.com"},
		AuthenticatedClaims: map[string]interface{}{"email": "user@example.com"},
	}
	suite.mockOAuthService.On("BuildFederatedAuthResult", mock.Anything, testOIDCIDPID, "sub-1", mock.Anything).
		Return(expected, nil)

	result, svcErr := suite.service.BuildFederatedAuthResult(
		context.Background(), testOIDCIDPID, "sub-1", map[string]interface{}{"email": "user@example.com"})
	suite.Nil(svcErr)
	suite.Equal(expected, result)
}
