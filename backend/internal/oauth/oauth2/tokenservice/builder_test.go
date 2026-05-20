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

package tokenservice

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/thunder-id/thunderid/internal/system/i18n/core"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	certmodel "github.com/thunder-id/thunderid/internal/cert"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/jwksresolver"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/jose/jwe"
	"github.com/thunder-id/thunderid/tests/mocks/httpmock"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwemock"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
)

const (
	testAccessToken  = "test-access-token"  //nolint:gosec // Test token, not a real credential
	testRefreshToken = "test-refresh-token" //nolint:gosec // Test token, not a real credential
	testIDToken      = "test-id-token"      //nolint:gosec // Test token, not a real credential
	testUserName     = "John Doe"
	testAppID        = "app123"
	testCacheID      = "test-cache-id"
)

type TokenBuilderTestSuite struct {
	suite.Suite
	mockJWTService *jwtmock.JWTServiceInterfaceMock
	builder        *tokenBuilder
	oauthApp       *inboundmodel.OAuthClient
}

func TestTokenBuilderTestSuite(t *testing.T) {
	suite.Run(t, new(TokenBuilderTestSuite))
}

func (suite *TokenBuilderTestSuite) SetupTest() {
	// Initialize Runtime for tests
	testConfig := &config.Config{
		JWT: config.JWTConfig{
			Issuer:         "https://thunder.io",
			ValidityPeriod: 3600,
		},
	}
	_ = config.InitializeServerRuntime("test", testConfig)

	suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())
	suite.builder = &tokenBuilder{
		jwtService: suite.mockJWTService,
	}

	suite.oauthApp = &inboundmodel.OAuthClient{
		ClientID: "test-client",
		Token: &inboundmodel.OAuthTokenConfig{
			AccessToken: &inboundmodel.AccessTokenConfig{
				ValidityPeriod: 3600,
				UserAttributes: []string{"name"}, // Configure user attributes for tests
			},
		},
	}
}

func (suite *TokenBuilderTestSuite) TestNewTokenBuilder() {
	jwtService := jwtmock.NewJWTServiceInterfaceMock(suite.T())
	builder := newTokenBuilder(jwtService, nil, nil)

	assert.NotNil(suite.T(), builder)
	assert.Implements(suite.T(), (*TokenBuilderInterface)(nil), builder)
}

func (suite *TokenBuilderTestSuite) TestBuildAccessToken_Success_Basic() {
	ctx := &AccessTokenBuildContext{
		Subject:        "user123",
		Audiences:      []string{"app123"},
		ClientID:       "test-client",
		Scopes:         []string{"read", "write"},
		UserAttributes: map[string]interface{}{"name": testUserName},
		GrantType:      string(constants.GrantTypeAuthorizationCode),
		OAuthApp:       suite.oauthApp,
	}

	expectedToken := testAccessToken
	expectedIat := time.Now().Unix()

	suite.mockJWTService.On("GenerateJWT",
		mock.Anything,
		"user123",
		"https://thunder.io",
		int64(3600),
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			return claims["scope"] == "read write" &&
				claims["client_id"] == "test-client" &&
				claims["grant_type"] == string(constants.GrantTypeAuthorizationCode) &&
				claims["name"] == testUserName
		}), mock.Anything, mock.Anything,
	).Return(expectedToken, expectedIat, nil)

	result, err := suite.builder.BuildAccessToken(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), expectedToken, result.Token)
	assert.Equal(suite.T(), constants.TokenTypeBearer, result.TokenType)
	assert.Equal(suite.T(), expectedIat, result.IssuedAt)
	assert.Equal(suite.T(), int64(3600), result.ExpiresIn)
	assert.Equal(suite.T(), []string{"read", "write"}, result.Scopes)
	assert.Equal(suite.T(), "test-client", result.ClientID)
	assert.Equal(suite.T(), map[string]interface{}{"name": testUserName}, result.UserAttributes)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenBuilderTestSuite) TestBuildAccessToken_Success_WithActorClaim() {
	actorClaims := &SubjectTokenClaims{
		Sub:            "actor123",
		Iss:            "https://actor-issuer.com",
		Aud:            nil,
		UserAttributes: map[string]interface{}{},
		NestedAct:      nil,
	}

	ctx := &AccessTokenBuildContext{
		Subject:        "user123",
		Audiences:      []string{"app123"},
		ClientID:       "test-client",
		Scopes:         []string{"read"},
		UserAttributes: map[string]interface{}{},
		GrantType:      string(constants.GrantTypeTokenExchange),
		OAuthApp:       suite.oauthApp,
		ActorClaims:    actorClaims,
	}

	expectedToken := testAccessToken
	expectedIat := time.Now().Unix()

	suite.mockJWTService.On("GenerateJWT",
		mock.Anything,
		"user123",
		"https://thunder.io",
		int64(3600),
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			act, ok := claims["act"].(map[string]interface{})
			return ok && act["sub"] == "actor123" && act["iss"] == "https://actor-issuer.com"
		}), mock.Anything, mock.Anything,
	).Return(expectedToken, expectedIat, nil)

	result, err := suite.builder.BuildAccessToken(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenBuilderTestSuite) TestBuildAccessToken_Success_WithNestedActorClaim() {
	nestedActorClaims := &SubjectTokenClaims{
		Sub:            "nested-actor",
		Iss:            "https://nested-issuer.com",
		Aud:            nil,
		UserAttributes: map[string]interface{}{},
		NestedAct: map[string]interface{}{
			"sub": "original-actor",
		},
	}

	ctx := &AccessTokenBuildContext{
		Subject:        "user123",
		Audiences:      []string{"app123"},
		ClientID:       "test-client",
		Scopes:         []string{"read"},
		UserAttributes: map[string]interface{}{},
		GrantType:      string(constants.GrantTypeTokenExchange),
		OAuthApp:       suite.oauthApp,
		ActorClaims:    nestedActorClaims,
	}

	expectedToken := testAccessToken
	expectedIat := time.Now().Unix()

	suite.mockJWTService.On("GenerateJWT",
		mock.Anything,
		"user123",
		"https://thunder.io",
		int64(3600),
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			act, ok := claims["act"].(map[string]interface{})
			return ok && act["sub"] == "nested-actor" && act["act"] != nil
		}), mock.Anything, mock.Anything,
	).Return(expectedToken, expectedIat, nil)

	result, err := suite.builder.BuildAccessToken(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenBuilderTestSuite) TestBuildAccessToken_Success_EmptyScopes() {
	ctx := &AccessTokenBuildContext{
		Subject:        "user123",
		Audiences:      []string{"app123"},
		ClientID:       "test-client",
		Scopes:         []string{},
		UserAttributes: map[string]interface{}{},
		GrantType:      string(constants.GrantTypeAuthorizationCode),
		OAuthApp:       suite.oauthApp,
	}

	expectedToken := testAccessToken
	expectedIat := time.Now().Unix()

	suite.mockJWTService.On("GenerateJWT",
		mock.Anything,
		"user123",
		"https://thunder.io",
		int64(3600),
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			_, hasScope := claims["scope"]
			return !hasScope
		}), mock.Anything, mock.Anything,
	).Return(expectedToken, expectedIat, nil)

	result, err := suite.builder.BuildAccessToken(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenBuilderTestSuite) TestBuildAccessToken_Success_EmptyClientID() {
	ctx := &AccessTokenBuildContext{
		Subject:        "user123",
		Audiences:      []string{"app123"},
		ClientID:       "",
		Scopes:         []string{"read"},
		UserAttributes: map[string]interface{}{},
		GrantType:      string(constants.GrantTypeAuthorizationCode),
		OAuthApp:       suite.oauthApp,
	}

	expectedToken := testAccessToken
	expectedIat := time.Now().Unix()

	suite.mockJWTService.On("GenerateJWT",
		mock.Anything,
		"user123",
		"https://thunder.io",
		int64(3600),
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			_, hasClientID := claims["client_id"]
			return !hasClientID
		}), mock.Anything, mock.Anything,
	).Return(expectedToken, expectedIat, nil)

	result, err := suite.builder.BuildAccessToken(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenBuilderTestSuite) TestBuildAccessToken_Success_EmptyGrantType() {
	ctx := &AccessTokenBuildContext{
		Subject:        "user123",
		Audiences:      []string{"app123"},
		ClientID:       "test-client",
		Scopes:         []string{"read"},
		UserAttributes: map[string]interface{}{},
		GrantType:      "",
		OAuthApp:       suite.oauthApp,
	}

	expectedToken := testAccessToken
	expectedIat := time.Now().Unix()

	suite.mockJWTService.On("GenerateJWT",
		mock.Anything,
		"user123",
		"https://thunder.io",
		int64(3600),
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			_, hasGrantType := claims["grant_type"]
			return !hasGrantType
		}), mock.Anything, mock.Anything,
	).Return(expectedToken, expectedIat, nil)

	result, err := suite.builder.BuildAccessToken(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenBuilderTestSuite) TestBuildAccessToken_Success_CustomValidityPeriod() {
	customOAuthApp := &inboundmodel.OAuthClient{
		ClientID: "test-client",
		Token: &inboundmodel.OAuthTokenConfig{
			AccessToken: &inboundmodel.AccessTokenConfig{
				ValidityPeriod: 7200,
			},
		},
	}

	ctx := &AccessTokenBuildContext{
		Subject:        "user123",
		Audiences:      []string{"app123"},
		ClientID:       "test-client",
		Scopes:         []string{"read"},
		UserAttributes: map[string]interface{}{},
		GrantType:      string(constants.GrantTypeAuthorizationCode),
		OAuthApp:       customOAuthApp,
	}

	expectedToken := testAccessToken
	expectedIat := time.Now().Unix()

	suite.mockJWTService.On("GenerateJWT",
		mock.Anything,
		"user123",
		"https://thunder.io", // Server-level issuer always used
		int64(7200),
		mock.Anything, mock.Anything, mock.Anything,
	).Return(expectedToken, expectedIat, nil)

	result, err := suite.builder.BuildAccessToken(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), int64(7200), result.ExpiresIn)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenBuilderTestSuite) TestBuildAccessToken_Error_NilContext() {
	result, err := suite.builder.BuildAccessToken(nil)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "build context cannot be nil")
}

func (suite *TokenBuilderTestSuite) TestBuildAccessToken_Error_JWTGenerationFailed() {
	ctx := &AccessTokenBuildContext{
		Subject:        "user123",
		Audiences:      []string{"app123"},
		ClientID:       "test-client",
		Scopes:         []string{"read"},
		UserAttributes: map[string]interface{}{},
		GrantType:      string(constants.GrantTypeAuthorizationCode),
		OAuthApp:       suite.oauthApp,
	}

	suite.mockJWTService.On("GenerateJWT",
		mock.Anything,
		"user123",
		"https://thunder.io",
		int64(3600),
		mock.Anything, mock.Anything, mock.Anything,
	).Return("", int64(0), &serviceerror.ServiceError{
		Type: serviceerror.ServerErrorType,
		Code: "JWT_GENERATION_FAILED",
		Error: core.I18nMessage{
			Key: "error.test.jwt_generation_failed", DefaultValue: "JWT generation failed",
		},
		ErrorDescription: core.I18nMessage{
			Key: "error.test.failed_to_generate_jwt_token", DefaultValue: "Failed to generate JWT token",
		},
	})

	result, err := suite.builder.BuildAccessToken(ctx)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "failed to generate access token")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenBuilderTestSuite) TestBuildAccessToken_Success_WithClaimsLocales() {
	ctx := &AccessTokenBuildContext{
		Subject:        "user123",
		Audiences:      []string{"app123"},
		ClientID:       "test-client",
		Scopes:         []string{"openid", "profile"},
		UserAttributes: map[string]interface{}{"name": testUserName},
		GrantType:      string(constants.GrantTypeAuthorizationCode),
		OAuthApp:       suite.oauthApp,
		ClaimsLocales:  "en-US fr-CA ja",
	}

	expectedToken := testAccessToken
	expectedIat := time.Now().Unix()

	suite.mockJWTService.On("GenerateJWT",
		mock.Anything,
		"user123",
		"https://thunder.io",
		int64(3600),
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			return claims["scope"] == "openid profile" &&
				claims["client_id"] == "test-client" &&
				claims["grant_type"] == string(constants.GrantTypeAuthorizationCode) &&
				claims["name"] == testUserName &&
				claims["claims_locales"] == "en-US fr-CA ja"
		}), mock.Anything, mock.Anything,
	).Return(expectedToken, expectedIat, nil)

	result, err := suite.builder.BuildAccessToken(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), expectedToken, result.Token)
	assert.Equal(suite.T(), "en-US fr-CA ja", result.ClaimsLocales)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenBuilderTestSuite) TestBuildRefreshToken_Success_Basic() {
	// Create OAuth app with user attributes configured
	oauthAppWithUserAttrs := &inboundmodel.OAuthClient{
		ClientID: "test-client",
		Token: &inboundmodel.OAuthTokenConfig{
			AccessToken: &inboundmodel.AccessTokenConfig{
				ValidityPeriod: 3600,
				UserAttributes: []string{"name"}, // Configure user attributes
			},
		},
	}

	ctx := &RefreshTokenBuildContext{
		ClientID:             "test-client",
		Scopes:               []string{"read", "write"},
		GrantType:            string(constants.GrantTypeAuthorizationCode),
		AccessTokenSubject:   "user123",
		AccessTokenAudiences: []string{"app123"},
		AttributeCacheID:     testCacheID,
		OAuthApp:             oauthAppWithUserAttrs,
	}

	expectedToken := testRefreshToken
	expectedIat := time.Now().Unix()

	suite.mockJWTService.On("GenerateJWT",
		mock.Anything,
		"test-client",
		"https://thunder.io",
		int64(3600),
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			return claims["scope"] == "read write" &&
				claims["access_token_sub"] == "user123" &&
				reflect.DeepEqual(claims["access_token_aud"], []string{testAppID}) &&
				claims["grant_type"] == string(constants.GrantTypeAuthorizationCode) &&
				claims["aci"] == testCacheID
		}), mock.Anything, mock.Anything,
	).Return(expectedToken, expectedIat, nil)

	result, err := suite.builder.BuildRefreshToken(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), expectedToken, result.Token)
	assert.Equal(suite.T(), expectedIat, result.IssuedAt)
	assert.Equal(suite.T(), int64(3600), result.ExpiresIn)
	assert.Equal(suite.T(), []string{"read", "write"}, result.Scopes)
	assert.Equal(suite.T(), "test-client", result.ClientID)
	assert.Equal(suite.T(), []string{"https://thunder.io"}, result.Audiences)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenBuilderTestSuite) TestBuildRefreshToken_Success_WithoutUserAttributes() {
	ctx := &RefreshTokenBuildContext{
		ClientID:             "test-client",
		Scopes:               []string{"read"},
		GrantType:            string(constants.GrantTypeAuthorizationCode),
		AccessTokenSubject:   "user123",
		AccessTokenAudiences: []string{"app123"},
		AttributeCacheID:     "",
		OAuthApp:             suite.oauthApp,
	}

	expectedToken := testRefreshToken
	expectedIat := time.Now().Unix()

	suite.mockJWTService.On("GenerateJWT",
		mock.Anything,
		"test-client",
		"https://thunder.io",
		int64(3600),
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			_, hasAttrCacheID := claims["aci"]
			return !hasAttrCacheID
		}), mock.Anything, mock.Anything,
	).Return(expectedToken, expectedIat, nil)

	result, err := suite.builder.BuildRefreshToken(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenBuilderTestSuite) TestBuildRefreshToken_Success_WithNilOAuthApp() {
	ctx := &RefreshTokenBuildContext{
		ClientID:             "test-client",
		Scopes:               []string{"read"},
		GrantType:            string(constants.GrantTypeAuthorizationCode),
		AccessTokenSubject:   "user123",
		AccessTokenAudiences: []string{"app123"},
		AttributeCacheID:     testCacheID,
		OAuthApp:             nil,
	}

	expectedToken := testRefreshToken
	expectedIat := time.Now().Unix()

	suite.mockJWTService.On("GenerateJWT",
		mock.Anything,
		"test-client",
		"https://thunder.io",
		int64(3600),
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			return claims["aci"] == testCacheID
		}), mock.Anything, mock.Anything,
	).Return(expectedToken, expectedIat, nil)

	result, err := suite.builder.BuildRefreshToken(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenBuilderTestSuite) TestBuildRefreshToken_Success_EmptyScopes() {
	ctx := &RefreshTokenBuildContext{
		ClientID:             "test-client",
		Scopes:               []string{},
		GrantType:            string(constants.GrantTypeAuthorizationCode),
		AccessTokenSubject:   "user123",
		AccessTokenAudiences: []string{"app123"},
		AttributeCacheID:     "",
		OAuthApp:             suite.oauthApp,
	}

	expectedToken := testRefreshToken
	expectedIat := time.Now().Unix()

	suite.mockJWTService.On("GenerateJWT",
		mock.Anything,
		"test-client",
		"https://thunder.io",
		int64(3600),
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			_, hasScope := claims["scope"]
			return !hasScope
		}), mock.Anything, mock.Anything,
	).Return(expectedToken, expectedIat, nil)

	result, err := suite.builder.BuildRefreshToken(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenBuilderTestSuite) TestBuildRefreshToken_Success_WithTokenConfig() {
	customOAuthApp := &inboundmodel.OAuthClient{
		ClientID: "test-client",
		Token: &inboundmodel.OAuthTokenConfig{
			AccessToken: &inboundmodel.AccessTokenConfig{},
		},
	}

	ctx := &RefreshTokenBuildContext{
		ClientID:             "test-client",
		Scopes:               []string{"read"},
		GrantType:            string(constants.GrantTypeAuthorizationCode),
		AccessTokenSubject:   "user123",
		AccessTokenAudiences: []string{"app123"},
		AttributeCacheID:     "",
		OAuthApp:             customOAuthApp,
	}

	expectedToken := testRefreshToken
	expectedIat := time.Now().Unix()

	suite.mockJWTService.On("GenerateJWT",
		mock.Anything,
		"test-client",
		"https://thunder.io",
		int64(3600),
		mock.Anything, mock.Anything, mock.Anything,
	).Return(expectedToken, expectedIat, nil)

	result, err := suite.builder.BuildRefreshToken(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenBuilderTestSuite) TestBuildRefreshToken_Success_WithNilAccessToken() {
	oauthAppWithNilAccessToken := &inboundmodel.OAuthClient{
		ClientID: "test-client",
		Token: &inboundmodel.OAuthTokenConfig{
			// Token exists but AccessToken is nil
			AccessToken: nil,
		},
	}

	ctx := &RefreshTokenBuildContext{
		ClientID:             "test-client",
		Scopes:               []string{"read"},
		GrantType:            string(constants.GrantTypeAuthorizationCode),
		AccessTokenSubject:   "user123",
		AccessTokenAudiences: []string{"app123"},
		AttributeCacheID:     testCacheID,
		OAuthApp:             oauthAppWithNilAccessToken,
	}

	expectedToken := testRefreshToken
	expectedIat := time.Now().Unix()

	suite.mockJWTService.On("GenerateJWT",
		mock.Anything,
		"test-client",
		"https://thunder.io",
		int64(3600),
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			return claims["aci"] == testCacheID
		}), mock.Anything, mock.Anything,
	).Return(expectedToken, expectedIat, nil)

	result, err := suite.builder.BuildRefreshToken(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenBuilderTestSuite) TestBuildRefreshToken_Error_NilContext() {
	result, err := suite.builder.BuildRefreshToken(nil)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "build context cannot be nil")
}

func (suite *TokenBuilderTestSuite) TestBuildRefreshToken_Error_JWTGenerationFailed() {
	ctx := &RefreshTokenBuildContext{
		ClientID:             "test-client",
		Scopes:               []string{"read"},
		GrantType:            string(constants.GrantTypeAuthorizationCode),
		AccessTokenSubject:   "user123",
		AccessTokenAudiences: []string{"app123"},
		AttributeCacheID:     "",
		OAuthApp:             suite.oauthApp,
	}

	suite.mockJWTService.On("GenerateJWT",
		mock.Anything,
		"test-client",
		"https://thunder.io",
		int64(3600),
		mock.Anything, mock.Anything, mock.Anything,
	).Return("", int64(0), &serviceerror.ServiceError{
		Type: serviceerror.ServerErrorType,
		Code: "JWT_GENERATION_FAILED",
		Error: core.I18nMessage{
			Key: "error.test.jwt_generation_failed", DefaultValue: "JWT generation failed",
		},
		ErrorDescription: core.I18nMessage{
			Key: "error.test.failed_to_generate_jwt_token", DefaultValue: "Failed to generate JWT token",
		},
	})

	result, err := suite.builder.BuildRefreshToken(ctx)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "failed to generate refresh token")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenBuilderTestSuite) TestBuildRefreshToken_Success_WithClaimsLocales() {
	ctx := &RefreshTokenBuildContext{
		ClientID:             "test-client",
		Scopes:               []string{"openid", "profile"},
		GrantType:            string(constants.GrantTypeAuthorizationCode),
		AccessTokenSubject:   "user123",
		AccessTokenAudiences: []string{"app123"},
		AttributeCacheID:     "",
		OAuthApp:             suite.oauthApp,
		ClaimsLocales:        "en-US fr-CA ja",
	}

	expectedToken := testRefreshToken
	expectedIat := time.Now().Unix()

	suite.mockJWTService.On("GenerateJWT",
		mock.Anything,
		"test-client",
		"https://thunder.io",
		int64(3600),
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			return claims["scope"] == "openid profile" &&
				claims["access_token_sub"] == "user123" &&
				reflect.DeepEqual(claims["access_token_aud"], []string{testAppID}) &&
				claims["grant_type"] == string(constants.GrantTypeAuthorizationCode) &&
				claims["access_token_claims_locales"] == "en-US fr-CA ja"
		}), mock.Anything, mock.Anything,
	).Return(expectedToken, expectedIat, nil)

	result, err := suite.builder.BuildRefreshToken(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), expectedToken, result.Token)
	assert.Equal(suite.T(), "en-US fr-CA ja", result.ClaimsLocales)
	suite.mockJWTService.AssertExpectations(suite.T())
}

// ============================================================================
// BuildIDToken Tests - Success Cases
// ============================================================================

func (suite *TokenBuilderTestSuite) TestBuildIDToken_Success_Basic() {
	ctx := &IDTokenBuildContext{
		Subject:        "user123",
		Audience:       "app123",
		Scopes:         []string{"openid", "profile"},
		UserAttributes: map[string]interface{}{"sub": "user123", "name": testUserName},
		AuthTime:       time.Now().Unix(),
		OAuthApp:       suite.oauthApp,
	}

	expectedToken := testIDToken
	expectedIat := time.Now().Unix()

	suite.mockJWTService.On("GenerateJWT",
		mock.Anything,
		"user123",
		"https://thunder.io",
		int64(3600),
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			// sub is passed as first arg to GenerateJWT, not in claims map
			return claims["auth_time"] == ctx.AuthTime
		}), mock.Anything, mock.Anything,
	).Return(expectedToken, expectedIat, nil)

	result, err := suite.builder.BuildIDToken(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), expectedToken, result.Token)
	assert.Equal(suite.T(), "", result.TokenType) // ID tokens are not bearer tokens
	assert.Equal(suite.T(), expectedIat, result.IssuedAt)
	assert.Equal(suite.T(), int64(3600), result.ExpiresIn)
	assert.Equal(suite.T(), []string{"openid", "profile"}, result.Scopes)
	assert.Equal(suite.T(), "app123", result.ClientID)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenBuilderTestSuite) TestBuildIDToken_Success_WithNonce() {
	ctx := &IDTokenBuildContext{
		Subject:        "user123",
		Audience:       "app123",
		Scopes:         []string{"openid"},
		UserAttributes: map[string]interface{}{"sub": "user123"},
		AuthTime:       time.Now().Unix(),
		OAuthApp:       suite.oauthApp,
		Nonce:          "test-nonce-123",
	}

	expectedToken := testIDToken
	expectedIat := time.Now().Unix()

	suite.mockJWTService.On("GenerateJWT",
		mock.Anything,
		"user123",
		"https://thunder.io",
		int64(3600),
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			return claims["nonce"] == "test-nonce-123"
		}), mock.Anything, mock.Anything,
	).Return(expectedToken, expectedIat, nil)

	result, err := suite.builder.BuildIDToken(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenBuilderTestSuite) TestBuildIDToken_Success_WithoutNonce() {
	ctx := &IDTokenBuildContext{
		Subject:        "user123",
		Audience:       "app123",
		Scopes:         []string{"openid"},
		UserAttributes: map[string]interface{}{"sub": "user123"},
		AuthTime:       time.Now().Unix(),
		OAuthApp:       suite.oauthApp,
		Nonce:          "",
	}

	expectedToken := testIDToken
	expectedIat := time.Now().Unix()

	suite.mockJWTService.On("GenerateJWT",
		mock.Anything,
		"user123",
		"https://thunder.io",
		int64(3600),
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			_, exists := claims["nonce"]
			return !exists
		}), mock.Anything, mock.Anything,
	).Return(expectedToken, expectedIat, nil)

	result, err := suite.builder.BuildIDToken(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenBuilderTestSuite) TestBuildIDToken_Success_NoAuthTime() {
	ctx := &IDTokenBuildContext{
		Subject:        "user123",
		Audience:       "app123",
		Scopes:         []string{"openid"},
		UserAttributes: map[string]interface{}{"sub": "user123"},
		AuthTime:       0,
		OAuthApp:       suite.oauthApp,
	}

	expectedToken := testIDToken
	expectedIat := time.Now().Unix()

	suite.mockJWTService.On("GenerateJWT",
		mock.Anything,
		"user123",
		"https://thunder.io",
		int64(3600),
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			_, hasAuthTime := claims["auth_time"]
			return !hasAuthTime
		}), mock.Anything, mock.Anything,
	).Return(expectedToken, expectedIat, nil)

	result, err := suite.builder.BuildIDToken(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenBuilderTestSuite) TestBuildIDToken_Success_WithScopeClaims() {
	oauthAppWithScopeClaims := &inboundmodel.OAuthClient{
		ClientID: "test-client",
		Token: &inboundmodel.OAuthTokenConfig{
			IDToken: &inboundmodel.IDTokenConfig{
				ValidityPeriod: 3600,
				UserAttributes: []string{"name", "email"},
			},
		},
		ScopeClaims: map[string][]string{
			"profile": {"name", "email"},
		},
	}

	ctx := &IDTokenBuildContext{
		Subject:        "user123",
		Audience:       "app123",
		Scopes:         []string{"openid", "profile"},
		UserAttributes: map[string]interface{}{"sub": "user123", "name": testUserName, "email": "john@example.com"},
		AuthTime:       time.Now().Unix(),
		OAuthApp:       oauthAppWithScopeClaims,
	}

	expectedToken := testIDToken
	expectedIat := time.Now().Unix()

	suite.mockJWTService.On("GenerateJWT",
		mock.Anything,
		"user123",
		"https://thunder.io",
		int64(3600),
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			return claims["name"] == testUserName && claims["email"] == "john@example.com"
		}), mock.Anything, mock.Anything,
	).Return(expectedToken, expectedIat, nil)

	result, err := suite.builder.BuildIDToken(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenBuilderTestSuite) TestBuildIDToken_Success_WithStandardOIDCScopes() {
	oauthAppWithUserAttrs := &inboundmodel.OAuthClient{
		ClientID: "test-client",
		Token: &inboundmodel.OAuthTokenConfig{
			IDToken: &inboundmodel.IDTokenConfig{
				ValidityPeriod: 3600,
				UserAttributes: []string{"name", "email"},
			},
		},
	}

	ctx := &IDTokenBuildContext{
		Subject:        "user123",
		Audience:       "app123",
		Scopes:         []string{"openid", "profile", "email"}, // Added email scope
		UserAttributes: map[string]interface{}{"sub": "user123", "name": testUserName, "email": "john@example.com"},
		AuthTime:       time.Now().Unix(),
		OAuthApp:       oauthAppWithUserAttrs,
	}

	expectedToken := testIDToken
	expectedIat := time.Now().Unix()

	suite.mockJWTService.On("GenerateJWT",
		mock.Anything,
		"user123",
		"https://thunder.io",
		int64(3600),
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			// Check that both name (from profile scope) and email (from email scope) are present
			return claims["name"] == testUserName && claims["email"] == "john@example.com"
		}), mock.Anything, mock.Anything,
	).Return(expectedToken, expectedIat, nil)

	result, err := suite.builder.BuildIDToken(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenBuilderTestSuite) TestBuildIDToken_Success_NoUserAttributes() {
	ctx := &IDTokenBuildContext{
		Subject:        "user123",
		Audience:       "app123",
		Scopes:         []string{"openid"},
		UserAttributes: nil,
		AuthTime:       time.Now().Unix(),
		OAuthApp:       suite.oauthApp,
	}

	expectedToken := testIDToken
	expectedIat := time.Now().Unix()

	suite.mockJWTService.On("GenerateJWT",
		mock.Anything,
		"user123",
		"https://thunder.io",
		int64(3600),
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			return claims["auth_time"] == ctx.AuthTime
		}), mock.Anything, mock.Anything,
	).Return(expectedToken, expectedIat, nil)

	result, err := suite.builder.BuildIDToken(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenBuilderTestSuite) TestBuildIDToken_Success_EmptyUserAttributes() {
	oauthAppWithEmptyUserAttrs := &inboundmodel.OAuthClient{
		ClientID: "test-client",
		Token: &inboundmodel.OAuthTokenConfig{
			IDToken: &inboundmodel.IDTokenConfig{
				UserAttributes: []string{},
			},
		},
	}

	ctx := &IDTokenBuildContext{
		Subject:        "user123",
		Audience:       "app123",
		Scopes:         []string{"openid", "profile"},
		UserAttributes: map[string]interface{}{"name": testUserName},
		AuthTime:       time.Now().Unix(),
		OAuthApp:       oauthAppWithEmptyUserAttrs,
	}

	expectedToken := testIDToken
	expectedIat := time.Now().Unix()

	suite.mockJWTService.On("GenerateJWT",
		mock.Anything,
		"user123",
		"https://thunder.io",
		int64(3600),
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			_, hasName := claims["name"]
			return claims["auth_time"] == ctx.AuthTime &&
				!hasName // Should not include name if not in UserAttributes config
		}), mock.Anything, mock.Anything,
	).Return(expectedToken, expectedIat, nil)

	result, err := suite.builder.BuildIDToken(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenBuilderTestSuite) TestBuildIDToken_Success_CustomValidityPeriod() {
	oauthAppWithCustomValidity := &inboundmodel.OAuthClient{
		ClientID: "test-client",
		Token: &inboundmodel.OAuthTokenConfig{
			IDToken: &inboundmodel.IDTokenConfig{
				ValidityPeriod: 7200,
			},
		},
	}

	ctx := &IDTokenBuildContext{
		Subject:        "user123",
		Audience:       "app123",
		Scopes:         []string{"openid"},
		UserAttributes: map[string]interface{}{"sub": "user123"},
		AuthTime:       time.Now().Unix(),
		OAuthApp:       oauthAppWithCustomValidity,
	}

	expectedToken := testIDToken
	expectedIat := time.Now().Unix()

	suite.mockJWTService.On("GenerateJWT",
		mock.Anything,
		"user123",
		"https://thunder.io",
		int64(7200),
		mock.Anything, mock.Anything, mock.Anything,
	).Return(expectedToken, expectedIat, nil)

	result, err := suite.builder.BuildIDToken(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), int64(7200), result.ExpiresIn)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenBuilderTestSuite) TestBuildIDToken_Error_NilContext() {
	result, err := suite.builder.BuildIDToken(nil)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "build context cannot be nil")
}

func (suite *TokenBuilderTestSuite) TestBuildIDToken_Error_JWTGenerationFailed() {
	ctx := &IDTokenBuildContext{
		Subject:        "user123",
		Audience:       "app123",
		Scopes:         []string{"openid"},
		UserAttributes: map[string]interface{}{"sub": "user123"},
		AuthTime:       time.Now().Unix(),
		OAuthApp:       suite.oauthApp,
	}

	suite.mockJWTService.On("GenerateJWT",
		mock.Anything,
		"user123",
		"https://thunder.io",
		int64(3600),
		mock.Anything, mock.Anything, mock.Anything,
	).Return("", int64(0), &serviceerror.ServiceError{
		Type: serviceerror.ServerErrorType,
		Code: "JWT_GENERATION_FAILED",
		Error: core.I18nMessage{
			Key: "error.test.jwt_generation_failed", DefaultValue: "JWT generation failed",
		},
		ErrorDescription: core.I18nMessage{
			Key: "error.test.failed_to_generate_jwt_token", DefaultValue: "Failed to generate JWT token",
		},
	})

	result, err := suite.builder.BuildIDToken(ctx)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "failed to generate ID token")
	suite.mockJWTService.AssertExpectations(suite.T())
}

// ============================================================================
// BuildIDToken — JWE encryption tests
// ============================================================================

const testJWKSURIForBuilder = "https://rp.example.com/jwks" //nolint:gosec // test URI

// TestBuildIDToken_Success_WithEncryption_InlineJWKS verifies that BuildIDToken produces a JWE
// (5 dot-separated parts) when id_token encryption is configured with an inline JWKS cert.
// It also asserts that the payload passed to Encrypt is the signed JWS (3 dot-separated parts)
// and that the alg/enc/cty values are forwarded correctly.
func (suite *TokenBuilderTestSuite) TestBuildIDToken_Success_WithEncryption_InlineJWKS() {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	pubJWKS := testRSAPublicKeyToJWKS(&privateKey.PublicKey, "enc")

	mockJWE := jwemock.NewJWEServiceInterfaceMock(suite.T())
	const signedJWS = "header.payload.signature"
	const encryptedJWE = "a.b.c.d.e"

	mockJWE.On("Encrypt",
		mock.MatchedBy(func(payload []byte) bool {
			// Payload must be the signed JWS — three dot-separated parts.
			return strings.Count(string(payload), ".") == 2
		}),
		mock.Anything,
		jwe.KeyEncAlgorithm("RSA-OAEP-256"),
		jwe.ContentEncAlgorithm("A256GCM"),
		"JWT",
		mock.Anything,
	).Return(encryptedJWE, (*serviceerror.ServiceError)(nil))

	oauthApp := &inboundmodel.OAuthClient{
		ClientID: "test-client",
		Token: &inboundmodel.OAuthTokenConfig{
			IDToken: &inboundmodel.IDTokenConfig{
				ValidityPeriod: 3600,
				ResponseType:   inboundmodel.IDTokenResponseTypeJWE,
				EncryptionAlg:  "RSA-OAEP-256",
				EncryptionEnc:  "A256GCM",
			},
		},
		Certificate: &inboundmodel.Certificate{
			Type:  certmodel.CertificateTypeJWKS,
			Value: pubJWKS,
		},
	}

	suite.mockJWTService.On("GenerateJWT",
		mock.Anything, "user123", "https://thunder.io", int64(3600),
		mock.Anything, mock.Anything, mock.Anything,
	).Return(signedJWS, time.Now().Unix(), (*serviceerror.ServiceError)(nil))

	builder := &tokenBuilder{
		jwtService:   suite.mockJWTService,
		jweService:   mockJWE,
		jwksResolver: jwksresolver.Initialize(nil),
	}

	result, err := builder.BuildIDToken(&IDTokenBuildContext{
		Context:  context.Background(),
		Subject:  "user123",
		Audience: "test-client",
		Scopes:   []string{"openid"},
		OAuthApp: oauthApp,
	})

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), encryptedJWE, result.Token)
	mockJWE.AssertExpectations(suite.T())
	suite.mockJWTService.AssertExpectations(suite.T())
}

// TestBuildIDToken_Error_EncryptionKeyNotFound verifies that when the JWKS contains no enc-capable
// key (e.g. all keys have use=sig), BuildIDToken returns an error without calling Encrypt.
func (suite *TokenBuilderTestSuite) TestBuildIDToken_Error_EncryptionKeyNotFound() {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	sigOnlyJWKS := testRSAPublicKeyToJWKS(&privateKey.PublicKey, "sig")

	mockJWE := jwemock.NewJWEServiceInterfaceMock(suite.T())

	oauthApp := &inboundmodel.OAuthClient{
		ClientID: "test-client",
		Token: &inboundmodel.OAuthTokenConfig{
			IDToken: &inboundmodel.IDTokenConfig{
				ValidityPeriod: 3600,
				ResponseType:   inboundmodel.IDTokenResponseTypeJWE,
				EncryptionAlg:  "RSA-OAEP-256",
				EncryptionEnc:  "A256GCM",
			},
		},
		Certificate: &inboundmodel.Certificate{
			Type:  certmodel.CertificateTypeJWKS,
			Value: sigOnlyJWKS,
		},
	}

	suite.mockJWTService.On("GenerateJWT",
		mock.Anything, "user123", "https://thunder.io", int64(3600),
		mock.Anything, mock.Anything, mock.Anything,
	).Return("header.payload.signature", time.Now().Unix(), (*serviceerror.ServiceError)(nil))

	builder := &tokenBuilder{
		jwtService:   suite.mockJWTService,
		jweService:   mockJWE,
		jwksResolver: jwksresolver.Initialize(nil),
	}

	result, err := builder.BuildIDToken(&IDTokenBuildContext{
		Context:  context.Background(),
		Subject:  "user123",
		Audience: "test-client",
		Scopes:   []string{"openid"},
		OAuthApp: oauthApp,
	})

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "failed to resolve ID token encryption key")
	// Encrypt must not be called when key resolution fails.
	mockJWE.AssertNotCalled(suite.T(), "Encrypt")
	suite.mockJWTService.AssertExpectations(suite.T())
}

// TestBuildIDToken_Error_EncryptionFailed verifies that a JWE Encrypt failure is propagated.
func (suite *TokenBuilderTestSuite) TestBuildIDToken_Error_EncryptionFailed() {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	pubJWKS := testRSAPublicKeyToJWKS(&privateKey.PublicKey, "enc")

	encErr := &serviceerror.ServiceError{
		Type: serviceerror.ServerErrorType,
		Code: "JWE_ENCRYPT_FAILED",
		Error: core.I18nMessage{
			Key: "error.jwe.encrypt_failed", DefaultValue: "JWE encryption failed",
		},
	}
	mockJWE := jwemock.NewJWEServiceInterfaceMock(suite.T())
	mockJWE.On("Encrypt",
		mock.Anything, mock.Anything,
		jwe.KeyEncAlgorithm("RSA-OAEP-256"),
		jwe.ContentEncAlgorithm("A256GCM"),
		"JWT",
		mock.Anything,
	).Return("", encErr)

	oauthApp := &inboundmodel.OAuthClient{
		ClientID: "test-client",
		Token: &inboundmodel.OAuthTokenConfig{
			IDToken: &inboundmodel.IDTokenConfig{
				ValidityPeriod: 3600,
				ResponseType:   inboundmodel.IDTokenResponseTypeJWE,
				EncryptionAlg:  "RSA-OAEP-256",
				EncryptionEnc:  "A256GCM",
			},
		},
		Certificate: &inboundmodel.Certificate{
			Type:  certmodel.CertificateTypeJWKS,
			Value: pubJWKS,
		},
	}

	suite.mockJWTService.On("GenerateJWT",
		mock.Anything, "user123", "https://thunder.io", int64(3600),
		mock.Anything, mock.Anything, mock.Anything,
	).Return("header.payload.signature", time.Now().Unix(), (*serviceerror.ServiceError)(nil))

	builder := &tokenBuilder{
		jwtService:   suite.mockJWTService,
		jweService:   mockJWE,
		jwksResolver: jwksresolver.Initialize(nil),
	}

	result, err := builder.BuildIDToken(&IDTokenBuildContext{
		Context:  context.Background(),
		Subject:  "user123",
		Audience: "test-client",
		Scopes:   []string{"openid"},
		OAuthApp: oauthApp,
	})

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "failed to encrypt ID token")
	mockJWE.AssertExpectations(suite.T())
	suite.mockJWTService.AssertExpectations(suite.T())
}

// TestBuildIDToken_Success_WithEncryption_JWKSURI verifies that the builder fetches the JWKS
// via the HTTP client when the certificate type is JWKS_URI, then encrypts the ID token.
func (suite *TokenBuilderTestSuite) TestBuildIDToken_Success_WithEncryption_JWKSURI() {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	pubJWKS := testRSAPublicKeyToJWKS(&privateKey.PublicKey, "enc")
	const encryptedJWE = "a.b.c.d.e"

	mockHTTP := httpmock.NewHTTPClientInterfaceMock(suite.T())
	mockHTTP.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return req.URL.String() == testJWKSURIForBuilder && req.Method == http.MethodGet
	})).Return(
		&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(pubJWKS)),
		}, nil,
	)

	mockJWE := jwemock.NewJWEServiceInterfaceMock(suite.T())
	mockJWE.On("Encrypt",
		mock.Anything, mock.Anything,
		jwe.KeyEncAlgorithm("RSA-OAEP-256"),
		jwe.ContentEncAlgorithm("A256GCM"),
		"JWT",
		mock.Anything,
	).Return(encryptedJWE, (*serviceerror.ServiceError)(nil))

	oauthApp := &inboundmodel.OAuthClient{
		ClientID: "test-client",
		Token: &inboundmodel.OAuthTokenConfig{
			IDToken: &inboundmodel.IDTokenConfig{
				ValidityPeriod: 3600,
				ResponseType:   inboundmodel.IDTokenResponseTypeJWE,
				EncryptionAlg:  "RSA-OAEP-256",
				EncryptionEnc:  "A256GCM",
			},
		},
		Certificate: &inboundmodel.Certificate{
			Type:  certmodel.CertificateTypeJWKSURI,
			Value: testJWKSURIForBuilder,
		},
	}

	suite.mockJWTService.On("GenerateJWT",
		mock.Anything, "user123", "https://thunder.io", int64(3600),
		mock.Anything, mock.Anything, mock.Anything,
	).Return("header.payload.signature", time.Now().Unix(), (*serviceerror.ServiceError)(nil))

	builder := &tokenBuilder{
		jwtService:   suite.mockJWTService,
		jweService:   mockJWE,
		jwksResolver: jwksresolver.Initialize(mockHTTP),
	}

	result, err := builder.BuildIDToken(&IDTokenBuildContext{
		Context:  context.Background(),
		Subject:  "user123",
		Audience: "test-client",
		Scopes:   []string{"openid"},
		OAuthApp: oauthApp,
	})

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), encryptedJWE, result.Token)
	mockHTTP.AssertExpectations(suite.T())
	mockJWE.AssertExpectations(suite.T())
	suite.mockJWTService.AssertExpectations(suite.T())
}

// TestBuildIDToken_Error_NilJWEService verifies the nil-jweService guard returns an error
// instead of panicking when encryption is configured.
func (suite *TokenBuilderTestSuite) TestBuildIDToken_Error_NilJWEService() {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	pubJWKS := testRSAPublicKeyToJWKS(&privateKey.PublicKey, "enc")

	oauthApp := &inboundmodel.OAuthClient{
		ClientID: "test-client",
		Token: &inboundmodel.OAuthTokenConfig{
			IDToken: &inboundmodel.IDTokenConfig{
				ValidityPeriod: 3600,
				ResponseType:   inboundmodel.IDTokenResponseTypeJWE,
				EncryptionAlg:  "RSA-OAEP-256",
				EncryptionEnc:  "A256GCM",
			},
		},
		Certificate: &inboundmodel.Certificate{
			Type:  certmodel.CertificateTypeJWKS,
			Value: pubJWKS,
		},
	}

	suite.mockJWTService.On("GenerateJWT",
		mock.Anything, "user123", "https://thunder.io", int64(3600),
		mock.Anything, mock.Anything, mock.Anything,
	).Return("header.payload.signature", time.Now().Unix(), (*serviceerror.ServiceError)(nil))

	builder := &tokenBuilder{
		jwtService: suite.mockJWTService,
		jweService: nil,
	}

	result, err := builder.BuildIDToken(&IDTokenBuildContext{
		Context:  context.Background(),
		Subject:  "user123",
		Audience: "test-client",
		Scopes:   []string{"openid"},
		OAuthApp: oauthApp,
	})

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "JWE service is not configured")
	suite.mockJWTService.AssertExpectations(suite.T())
}

// TestBuildIDToken_NoEncryptionAlg verifies that the JWE block is skipped when EncryptionAlg is empty.
func (suite *TokenBuilderTestSuite) TestBuildIDToken_NoEncryptionAlg() {
	oauthApp := &inboundmodel.OAuthClient{
		ClientID: "test-client",
		Token: &inboundmodel.OAuthTokenConfig{
			IDToken: &inboundmodel.IDTokenConfig{
				ValidityPeriod: 3600,
				EncryptionAlg:  "", // no encryption
			},
		},
	}

	suite.mockJWTService.On("GenerateJWT",
		mock.Anything, "user123", "https://thunder.io", int64(3600),
		mock.Anything, mock.Anything, mock.Anything,
	).Return("header.payload.signature", time.Now().Unix(), (*serviceerror.ServiceError)(nil))

	builder := &tokenBuilder{
		jwtService: suite.mockJWTService,
		jweService: nil,
	}

	result, err := builder.BuildIDToken(&IDTokenBuildContext{
		Context:  context.Background(),
		Subject:  "user123",
		Audience: "test-client",
		Scopes:   []string{"openid"},
		OAuthApp: oauthApp,
	})

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "header.payload.signature", result.Token)
	suite.mockJWTService.AssertExpectations(suite.T())
}

// TestBuildIDToken_Error_UnsupportedCertType verifies error propagation when cert type is unsupported.
func (suite *TokenBuilderTestSuite) TestBuildIDToken_Error_UnsupportedCertType() {
	oauthApp := &inboundmodel.OAuthClient{
		ClientID: "test-client",
		Token: &inboundmodel.OAuthTokenConfig{
			IDToken: &inboundmodel.IDTokenConfig{
				ValidityPeriod: 3600,
				ResponseType:   inboundmodel.IDTokenResponseTypeJWE,
				EncryptionAlg:  "RSA-OAEP-256",
				EncryptionEnc:  "A256GCM",
			},
		},
		Certificate: &inboundmodel.Certificate{
			Type:  "UNKNOWN",
			Value: "{}",
		},
	}

	mockJWE := jwemock.NewJWEServiceInterfaceMock(suite.T())

	suite.mockJWTService.On("GenerateJWT",
		mock.Anything, "user123", "https://thunder.io", int64(3600),
		mock.Anything, mock.Anything, mock.Anything,
	).Return("header.payload.signature", time.Now().Unix(), (*serviceerror.ServiceError)(nil))

	builder := &tokenBuilder{
		jwtService:   suite.mockJWTService,
		jweService:   mockJWE,
		jwksResolver: jwksresolver.Initialize(nil),
	}

	result, err := builder.BuildIDToken(&IDTokenBuildContext{
		Context:  context.Background(),
		Subject:  "user123",
		Audience: "test-client",
		Scopes:   []string{"openid"},
		OAuthApp: oauthApp,
	})

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "failed to resolve ID token encryption key")
	mockJWE.AssertNotCalled(suite.T(), "Encrypt")
	suite.mockJWTService.AssertExpectations(suite.T())
}

// testRSAPublicKeyToJWKS builds a minimal RSA JWKS JSON string for tests.
// Pass use="" to omit the 'use' field.
func testRSAPublicKeyToJWKS(pub *rsa.PublicKey, use string) string {
	eBytes := big.NewInt(int64(pub.E)).Bytes()
	key := map[string]interface{}{
		"kty": "RSA",
		"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(eBytes),
	}
	if use != "" {
		key["use"] = use
	}
	b, _ := json.Marshal(map[string]interface{}{"keys": []interface{}{key}})
	return string(b)
}
