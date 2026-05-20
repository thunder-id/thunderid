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

package granthandlers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/attributecache"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/attributecachemock"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/tokenservicemock"
	"github.com/thunder-id/thunderid/tests/mocks/resourcemock"
)

// testUserID and testAudience are declared in tokenexchange_test.go
const testRefreshTokenUserID = "test-user-id"
const testRefreshTokenAudience = "test-audience"
const testRefreshTokenClientID = "test-client-id"
const testRS01URI = "https://rs01.example.com"
const testRS02URI = "https://rs02.example.com"

type RefreshTokenGrantHandlerTestSuite struct {
	suite.Suite
	handler              *refreshTokenGrantHandler
	mockJWTService       *jwtmock.JWTServiceInterfaceMock
	mockTokenBuilder     *tokenservicemock.TokenBuilderInterfaceMock
	mockTokenValidator   *tokenservicemock.TokenValidatorInterfaceMock
	mockAttrCacheService *attributecachemock.AttributeCacheServiceInterfaceMock
	mockResourceService  *resourcemock.ResourceServiceInterfaceMock
	oauthApp             *inboundmodel.OAuthClient
	validRefreshToken    string
	validClaims          map[string]interface{}
	testTokenReq         *model.TokenRequest
}

func TestRefreshTokenGrantHandlerSuite(t *testing.T) {
	suite.Run(t, new(RefreshTokenGrantHandlerTestSuite))
}

func (suite *RefreshTokenGrantHandlerTestSuite) SetupTest() {
	// Reset server runtime before initializing with test config
	config.ResetServerRuntime()

	// Initialize Runtime config with basic test config
	testConfig := &config.Config{
		JWT: config.JWTConfig{
			ValidityPeriod: 3600,
		},
		OAuth: config.OAuthConfig{
			RefreshToken: config.RefreshTokenConfig{
				ValidityPeriod: 86400,
				RenewOnGrant:   false,
			},
		},
	}
	_ = config.InitializeServerRuntime("test", testConfig)

	suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())
	suite.mockTokenBuilder = tokenservicemock.NewTokenBuilderInterfaceMock(suite.T())
	suite.mockTokenValidator = tokenservicemock.NewTokenValidatorInterfaceMock(suite.T())
	suite.mockAttrCacheService = attributecachemock.NewAttributeCacheServiceInterfaceMock(suite.T())
	suite.mockResourceService = resourcemock.NewResourceServiceInterfaceMock(suite.T())

	suite.mockResourceService.On("GetResourceServerByIdentifier", mock.Anything, mock.Anything).
		Return(func(_ context.Context, identifier string) *resource.ResourceServer {
			return &resource.ResourceServer{ID: identifier, Identifier: identifier}
		}, func(_ context.Context, _ string) *serviceerror.ServiceError {
			return nil
		}).Maybe()
	suite.mockResourceService.On("ValidatePermissions", mock.Anything, mock.Anything, mock.Anything).
		Return([]string{}, nil).Maybe()

	suite.handler = &refreshTokenGrantHandler{
		jwtService:       suite.mockJWTService,
		tokenBuilder:     suite.mockTokenBuilder,
		tokenValidator:   suite.mockTokenValidator,
		attrCacheService: suite.mockAttrCacheService,
		resourceService:  suite.mockResourceService,
	}

	suite.oauthApp = &inboundmodel.OAuthClient{
		ClientID:                testRefreshTokenClientID,
		GrantTypes:              []constants.GrantType{constants.GrantTypeRefreshToken},
		TokenEndpointAuthMethod: constants.TokenEndpointAuthMethodClientSecretPost,
		Token: &inboundmodel.OAuthTokenConfig{
			AccessToken: &inboundmodel.AccessTokenConfig{
				UserAttributes: []string{"email", "username"},
			},
		},
	}

	suite.validRefreshToken = "valid.refresh.token"
	now := time.Now().Unix()
	suite.validClaims = map[string]interface{}{
		"iat":              float64(now - 3600),
		"exp":              float64(now + 86400),
		"client_id":        testRefreshTokenClientID,
		"grant_type":       "authorization_code",
		"scopes":           "read write",
		"access_token_sub": testRefreshTokenUserID,
		"access_token_aud": testRefreshTokenAudience,
	}

	suite.testTokenReq = &model.TokenRequest{
		GrantType:    string(constants.GrantTypeRefreshToken),
		ClientID:     testRefreshTokenClientID,
		RefreshToken: suite.validRefreshToken,
		Scope:        "read",
	}
}

func (suite *RefreshTokenGrantHandlerTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestNewRefreshTokenGrantHandler() {
	handler := newRefreshTokenGrantHandler(
		suite.mockJWTService,
		suite.mockTokenBuilder,
		suite.mockTokenValidator,
		suite.mockAttrCacheService,
		suite.mockResourceService,
	)
	assert.NotNil(suite.T(), handler)
	assert.Implements(suite.T(), (*RefreshTokenGrantHandlerInterface)(nil), handler)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestValidateGrant_Success() {
	err := suite.handler.ValidateGrant(context.Background(), suite.testTokenReq, suite.oauthApp)
	assert.Nil(suite.T(), err)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestValidateGrant_InvalidGrantType() {
	tokenReq := &model.TokenRequest{
		GrantType:    "invalid_grant",
		ClientID:     testRefreshTokenClientID,
		RefreshToken: "token",
	}

	err := suite.handler.ValidateGrant(context.Background(), tokenReq, suite.oauthApp)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorUnsupportedGrantType, err.Error)
	assert.Equal(suite.T(), "Unsupported grant type", err.ErrorDescription)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestValidateGrant_MissingRefreshToken() {
	tokenReq := &model.TokenRequest{
		GrantType: string(constants.GrantTypeRefreshToken),
		ClientID:  testRefreshTokenClientID,
	}

	err := suite.handler.ValidateGrant(context.Background(), tokenReq, suite.oauthApp)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, err.Error)
	assert.Equal(suite.T(), "Refresh token is required", err.ErrorDescription)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestValidateGrant_MissingClientID() {
	tokenReq := &model.TokenRequest{
		GrantType:    string(constants.GrantTypeRefreshToken),
		RefreshToken: "token",
	}

	err := suite.handler.ValidateGrant(context.Background(), tokenReq, suite.oauthApp)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, err.Error)
	assert.Equal(suite.T(), "Client ID is required", err.ErrorDescription)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_InvalidSignature() {
	// Mock token validator to return error (simulating signature verification failure)
	suite.mockTokenValidator.On("ValidateRefreshToken", suite.validRefreshToken, testRefreshTokenClientID).
		Return(nil, errors.New("public key not available"))

	response, err := suite.handler.HandleGrant(context.Background(), suite.testTokenReq, suite.oauthApp)

	assert.Nil(suite.T(), response)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorInvalidGrant, err.Error)
	assert.Equal(suite.T(), "Invalid refresh token", err.ErrorDescription)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestIssueRefreshToken_Success() {
	// Mock token builder for refresh token generation
	suite.mockTokenBuilder.On("BuildRefreshToken", mock.MatchedBy(
		func(ctx *tokenservice.RefreshTokenBuildContext) bool {
			return ctx.ClientID == testRefreshTokenClientID &&
				ctx.GrantType == "authorization_code" &&
				ctx.AccessTokenSubject == testRefreshTokenUserID &&
				len(ctx.AccessTokenAudiences) == 1 && ctx.AccessTokenAudiences[0] == testRefreshTokenAudience
		})).Return(&model.TokenDTO{
		Token:     "new.refresh.token",
		TokenType: "",
		IssuedAt:  int64(1234567890),
		ExpiresIn: 3600,
		Scopes:    []string{"read", "write"},
		ClientID:  testRefreshTokenClientID,
	}, nil)

	tokenResponse := &model.TokenResponseDTO{}

	err := suite.handler.IssueRefreshToken(context.Background(), tokenResponse, suite.oauthApp,
		testRefreshTokenUserID, []string{testRefreshTokenAudience},
		"authorization_code", []string{"read", "write"}, nil, "", "")

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), tokenResponse.RefreshToken)
	assert.Equal(suite.T(), "new.refresh.token", tokenResponse.RefreshToken.Token)
	assert.Equal(suite.T(), "", tokenResponse.RefreshToken.TokenType)
	assert.Equal(suite.T(), int64(1234567890), tokenResponse.RefreshToken.IssuedAt)
	assert.Equal(suite.T(), int64(3600), tokenResponse.RefreshToken.ExpiresIn)
	assert.Equal(suite.T(), []string{"read", "write"}, tokenResponse.RefreshToken.Scopes)
	assert.Equal(suite.T(), testRefreshTokenClientID, tokenResponse.RefreshToken.ClientID)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestIssueRefreshToken_JWTGenerationError() {
	// Mock token builder to return error
	suite.mockTokenBuilder.On("BuildRefreshToken", mock.Anything).
		Return(nil, errors.New("JWT generation failed"))

	tokenResponse := &model.TokenResponseDTO{}

	err := suite.handler.IssueRefreshToken(context.Background(), tokenResponse, suite.oauthApp, "", nil,
		"authorization_code", []string{"read"}, nil, "", "")

	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorServerError, err.Error)
	assert.Equal(suite.T(), "Failed to generate refresh token", err.ErrorDescription)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestIssueRefreshToken_WithEmptyTokenAttributes() {
	// Mock token builder with matcher that checks for empty sub and aud
	suite.mockTokenBuilder.On("BuildRefreshToken", mock.MatchedBy(
		func(ctx *tokenservice.RefreshTokenBuildContext) bool {
			return ctx.AccessTokenSubject == "" && len(ctx.AccessTokenAudiences) == 0
		})).Return(&model.TokenDTO{
		Token:    "new.refresh.token",
		IssuedAt: int64(1234567890),
	}, nil)

	tokenResponse := &model.TokenResponseDTO{}

	err := suite.handler.IssueRefreshToken(context.Background(), tokenResponse, suite.oauthApp, "", nil,
		"authorization_code", []string{"read"}, nil, "", "")

	assert.Nil(suite.T(), err)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestIssueRefreshToken_WithClaimsLocales() {
	// Mock token builder with matcher that checks claimsLocales is propagated
	suite.mockTokenBuilder.On("BuildRefreshToken", mock.MatchedBy(
		func(ctx *tokenservice.RefreshTokenBuildContext) bool {
			return ctx.ClientID == testRefreshTokenClientID &&
				ctx.GrantType == "authorization_code" &&
				ctx.AccessTokenSubject == testRefreshTokenUserID &&
				len(ctx.AccessTokenAudiences) == 1 && ctx.AccessTokenAudiences[0] == testRefreshTokenAudience &&
				ctx.ClaimsLocales == "en-US fr-CA ja"
		})).Return(&model.TokenDTO{
		Token:         "new.refresh.token",
		IssuedAt:      int64(1234567890),
		ExpiresIn:     3600,
		ClaimsLocales: "en-US fr-CA ja",
	}, nil)

	tokenResponse := &model.TokenResponseDTO{}

	err := suite.handler.IssueRefreshToken(context.Background(), tokenResponse, suite.oauthApp,
		testRefreshTokenUserID, []string{testRefreshTokenAudience},
		"authorization_code", []string{"read"}, nil, "en-US fr-CA ja", "")

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), tokenResponse.RefreshToken)
	assert.Equal(suite.T(), "en-US fr-CA ja", tokenResponse.RefreshToken.ClaimsLocales)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_Success_WithRenewOnGrantDisabled() {
	// Mock successful refresh token validation
	suite.mockTokenValidator.On("ValidateRefreshToken", suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:              testRefreshTokenUserID,
			Audiences:        []string{testRefreshTokenAudience},
			Scopes:           []string{"read", "write"},
			GrantType:        "authorization_code",
			AttributeCacheID: "",
			Iat:              int64(suite.validClaims["iat"].(float64)),
		}, nil)

	// Mock successful access token generation
	suite.mockTokenBuilder.On("BuildAccessToken", mock.MatchedBy(
		func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return ctx.Subject == testRefreshTokenUserID &&
				ctx.ClientID == testRefreshTokenClientID &&
				len(ctx.Scopes) == 1 && ctx.Scopes[0] == testScopeRead
		})).Return(&model.TokenDTO{
		Token:     "new.access.token",
		IssuedAt:  time.Now().Unix(),
		ExpiresIn: 3600,
		Scopes:    []string{"read"},
	}, nil)

	response, err := suite.handler.HandleGrant(context.Background(), suite.testTokenReq, suite.oauthApp)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), response)
	assert.Equal(suite.T(), "new.access.token", response.AccessToken.Token)
	assert.Equal(suite.T(), suite.validRefreshToken, response.RefreshToken.Token)
	assert.Equal(suite.T(), []string{"read", "write"}, response.RefreshToken.Scopes)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_Success_WithRenewOnGrantEnabled() {
	// Enable RenewOnGrant in config
	config.GetServerRuntime().Config.OAuth.RefreshToken.RenewOnGrant = true

	// Mock successful refresh token validation
	suite.mockTokenValidator.On("ValidateRefreshToken", suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:              testRefreshTokenUserID,
			Audiences:        []string{testRefreshTokenAudience},
			Scopes:           []string{"read", "write"},
			GrantType:        "authorization_code",
			AttributeCacheID: "",
			Iat:              int64(suite.validClaims["iat"].(float64)),
		}, nil)

	// Mock successful access token generation
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything).Return(&model.TokenDTO{
		Token:          "new.access.token",
		IssuedAt:       time.Now().Unix(),
		ExpiresIn:      3600,
		Scopes:         []string{"read"},
		UserAttributes: map[string]interface{}{"email": "test@example.com"},
	}, nil)

	// Mock successful refresh token generation
	suite.mockTokenBuilder.On("BuildRefreshToken", mock.MatchedBy(
		func(ctx *tokenservice.RefreshTokenBuildContext) bool {
			return ctx.AccessTokenSubject == testRefreshTokenUserID &&
				len(ctx.AccessTokenAudiences) == 1 && ctx.AccessTokenAudiences[0] == testRefreshTokenAudience
		})).Return(&model.TokenDTO{
		Token:     "new.refresh.token",
		IssuedAt:  time.Now().Unix(),
		ExpiresIn: 86400,
		Scopes:    []string{"read"},
	}, nil)

	response, err := suite.handler.HandleGrant(context.Background(), suite.testTokenReq, suite.oauthApp)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), response)
	assert.Equal(suite.T(), "new.access.token", response.AccessToken.Token)
	assert.Equal(suite.T(), "new.refresh.token", response.RefreshToken.Token)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_GetAttributeCacheError() {
	suite.mockTokenValidator.On("ValidateRefreshToken", suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:              testRefreshTokenUserID,
			Audiences:        []string{testRefreshTokenAudience},
			Scopes:           []string{"read", "write"},
			GrantType:        "authorization_code",
			AttributeCacheID: testCacheID,
			Iat:              int64(suite.validClaims["iat"].(float64)),
		}, nil)

	cacheErr := &serviceerror.ServiceError{
		Type: serviceerror.ServerErrorType,
		Code: "ACS-2001",
		Error: core.I18nMessage{
			Key:          "error.attributecache.internal_server_error",
			DefaultValue: "Internal server error",
		},
	}
	suite.mockAttrCacheService.On("GetAttributeCache", mock.Anything, testCacheID).
		Return((*attributecache.AttributeCache)(nil), cacheErr).Once()

	response, err := suite.handler.HandleGrant(context.Background(), suite.testTokenReq, suite.oauthApp)

	assert.Nil(suite.T(), response)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorServerError, err.Error)
	assert.Equal(suite.T(), "Failed to get user attributes from attribute cache", err.ErrorDescription)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_BuildAccessTokenError() {
	// Mock successful refresh token validation
	suite.mockTokenValidator.On("ValidateRefreshToken", suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:              testRefreshTokenUserID,
			Audiences:        []string{testRefreshTokenAudience},
			Scopes:           []string{"read"},
			GrantType:        "authorization_code",
			AttributeCacheID: "",
			Iat:              int64(suite.validClaims["iat"].(float64)),
		}, nil)

	// Mock failed access token generation
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything).
		Return(nil, errors.New("failed to sign JWT"))

	response, err := suite.handler.HandleGrant(context.Background(), suite.testTokenReq, suite.oauthApp)

	assert.Nil(suite.T(), response)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorServerError, err.Error)
	assert.Equal(suite.T(), "Failed to generate access token", err.ErrorDescription)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_IssueRefreshTokenError() {
	// Enable RenewOnGrant in config
	config.GetServerRuntime().Config.OAuth.RefreshToken.RenewOnGrant = true

	// Mock successful refresh token validation
	suite.mockTokenValidator.On("ValidateRefreshToken", suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:              testRefreshTokenUserID,
			Audiences:        []string{testRefreshTokenAudience},
			Scopes:           []string{"read"},
			GrantType:        "authorization_code",
			AttributeCacheID: "",
			Iat:              int64(suite.validClaims["iat"].(float64)),
		}, nil)

	// Mock successful access token generation
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything).Return(&model.TokenDTO{
		Token:          "new.access.token",
		IssuedAt:       time.Now().Unix(),
		ExpiresIn:      3600,
		Scopes:         []string{"read"},
		UserAttributes: map[string]interface{}{},
	}, nil)

	// Mock failed refresh token generation
	suite.mockTokenBuilder.On("BuildRefreshToken", mock.Anything).
		Return(nil, errors.New("refresh token generation failed"))

	response, err := suite.handler.HandleGrant(context.Background(), suite.testTokenReq, suite.oauthApp)

	assert.Nil(suite.T(), response)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorServerError, err.Error)
	assert.Equal(suite.T(), "Failed to generate refresh token", err.ErrorDescription)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_ExtractIatClaimError() {
	// RenewOnGrant is disabled by default in SetupTest

	// Mock validator to return error when iat is missing (validation fails)
	suite.mockTokenValidator.On("ValidateRefreshToken", suite.validRefreshToken, testRefreshTokenClientID).
		Return(nil, errors.New("missing or invalid 'iat' claim"))

	response, err := suite.handler.HandleGrant(context.Background(), suite.testTokenReq, suite.oauthApp)

	assert.Nil(suite.T(), response)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorInvalidGrant, err.Error)
	assert.Equal(suite.T(), "Invalid refresh token", err.ErrorDescription)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestValidateAndApplyScopes_NoScopesRequested() {
	refreshTokenScopes := []string{"read", "write", "delete"}
	logger := log.GetLogger()

	result, errResp := suite.handler.validateAndApplyScopes("", refreshTokenScopes, logger)

	assert.Nil(suite.T(), errResp)
	assert.Equal(suite.T(), refreshTokenScopes, result)
	assert.Len(suite.T(), result, 3)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestValidateAndApplyScopes_RequestedScopesSubset() {
	refreshTokenScopes := []string{"read", "write", "delete"}
	logger := log.GetLogger()

	result, errResp := suite.handler.validateAndApplyScopes("read write", refreshTokenScopes, logger)

	assert.Nil(suite.T(), errResp)
	assert.Len(suite.T(), result, 2)
	assert.Contains(suite.T(), result, "read")
	assert.Contains(suite.T(), result, "write")
	assert.NotContains(suite.T(), result, "delete")
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestValidateAndApplyScopes_SomeRequestedScopesNotInRefreshToken() {
	refreshTokenScopes := []string{"read", "write"}
	logger := log.GetLogger()

	result, errResp := suite.handler.validateAndApplyScopes("read write delete admin", refreshTokenScopes, logger)

	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorInvalidScope, errResp.Error)
	assert.Nil(suite.T(), result)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestValidateAndApplyScopes_NoMatchingScopes() {
	refreshTokenScopes := []string{"read", "write"}
	logger := log.GetLogger()

	result, errResp := suite.handler.validateAndApplyScopes("admin delete", refreshTokenScopes, logger)

	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorInvalidScope, errResp.Error)
	assert.Nil(suite.T(), result)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_IDTokenGenerated_WhenOpenIDScopePresent() {
	// Mock successful refresh token validation with openid scope
	suite.mockTokenValidator.On("ValidateRefreshToken", suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:              testRefreshTokenUserID,
			Audiences:        []string{testRefreshTokenAudience},
			Scopes:           []string{"openid", "read"},
			GrantType:        "authorization_code",
			AttributeCacheID: "",
			Iat:              int64(suite.validClaims["iat"].(float64)),
		}, nil)

	// Mock successful access token generation
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything).Return(&model.TokenDTO{
		Token:          "new.access.token",
		IssuedAt:       time.Now().Unix(),
		ExpiresIn:      3600,
		Scopes:         []string{"openid", "read"},
		UserAttributes: map[string]interface{}{"email": "test@example.com"},
	}, nil)

	// Mock successful ID token generation
	suite.mockTokenBuilder.On("BuildIDToken", mock.MatchedBy(
		func(ctx *tokenservice.IDTokenBuildContext) bool {
			return ctx.Subject == testRefreshTokenUserID &&
				ctx.Audience == testRefreshTokenClientID &&
				len(ctx.Scopes) == 2 &&
				ctx.AuthTime == 0 // auth_time is not set during refresh (per OIDC spec)
		})).Return(&model.TokenDTO{
		Token:     "new.id.token",
		IssuedAt:  time.Now().Unix(),
		ExpiresIn: 3600,
		Scopes:    []string{"openid", "read"},
		ClientID:  testRefreshTokenClientID,
		Subject:   testRefreshTokenUserID,
	}, nil)

	tokenReq := &model.TokenRequest{
		GrantType:    string(constants.GrantTypeRefreshToken),
		ClientID:     testRefreshTokenClientID,
		RefreshToken: suite.validRefreshToken,
		Scope:        "openid read",
	}

	response, err := suite.handler.HandleGrant(context.Background(), tokenReq, suite.oauthApp)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), response)
	assert.Equal(suite.T(), "new.access.token", response.AccessToken.Token)
	assert.Equal(suite.T(), "new.id.token", response.IDToken.Token)
	assert.Equal(suite.T(), testRefreshTokenUserID, response.IDToken.Subject)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_NoIDToken_WhenOpenIDScopeAbsent() {
	// Mock successful refresh token validation without openid scope
	suite.mockTokenValidator.On("ValidateRefreshToken", suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:              testRefreshTokenUserID,
			Audiences:        []string{testRefreshTokenAudience},
			Scopes:           []string{"read", "write"},
			GrantType:        "authorization_code",
			AttributeCacheID: "",
			Iat:              int64(suite.validClaims["iat"].(float64)),
		}, nil)

	// Mock successful access token generation
	suite.mockTokenBuilder.On("BuildAccessToken", mock.MatchedBy(
		func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return ctx.Subject == testRefreshTokenUserID &&
				ctx.ClientID == testRefreshTokenClientID &&
				len(ctx.Scopes) == 1 && ctx.Scopes[0] == testScopeRead
		})).Return(&model.TokenDTO{
		Token:     "new.access.token",
		IssuedAt:  time.Now().Unix(),
		ExpiresIn: 3600,
		Scopes:    []string{"read"},
	}, nil)

	response, err := suite.handler.HandleGrant(context.Background(), suite.testTokenReq, suite.oauthApp)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), response)
	assert.Equal(suite.T(), "new.access.token", response.AccessToken.Token)
	// ID token should be empty when openid scope is not present
	assert.Empty(suite.T(), response.IDToken.Token)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_IDTokenGenerationError() {
	// Mock successful refresh token validation with openid scope
	suite.mockTokenValidator.On("ValidateRefreshToken", suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:              testRefreshTokenUserID,
			Audiences:        []string{testRefreshTokenAudience},
			Scopes:           []string{"openid", "read"},
			GrantType:        "authorization_code",
			AttributeCacheID: "",
			Iat:              int64(suite.validClaims["iat"].(float64)),
		}, nil)

	// Mock successful access token generation
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything).Return(&model.TokenDTO{
		Token:     "new.access.token",
		IssuedAt:  time.Now().Unix(),
		ExpiresIn: 3600,
		Scopes:    []string{"openid", "read"},
	}, nil)

	// Mock failed ID token generation
	suite.mockTokenBuilder.On("BuildIDToken", mock.Anything).
		Return(nil, errors.New("failed to generate ID token"))

	tokenReq := &model.TokenRequest{
		GrantType:    string(constants.GrantTypeRefreshToken),
		ClientID:     testRefreshTokenClientID,
		RefreshToken: suite.validRefreshToken,
		Scope:        "openid read",
	}

	response, err := suite.handler.HandleGrant(context.Background(), tokenReq, suite.oauthApp)

	assert.Nil(suite.T(), response)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorServerError, err.Error)
	assert.Equal(suite.T(), "Failed to generate token", err.ErrorDescription)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_NoRenewOnGrant_ReusesExistingRefreshToken() {
	// RenewOnGrant is false by default in SetupTest.
	// iat = now-3600, refreshValidity = 86400 → refresh remaining ≈ 82800 s.
	// Access token ExpiresIn is 0 (< 82800), so the cache is extended to the refresh
	// token's remaining lifetime (~82800 s).

	suite.mockTokenValidator.On("ValidateRefreshToken", suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:              testRefreshTokenUserID,
			Audiences:        []string{testRefreshTokenAudience},
			Scopes:           []string{"read", "write"},
			GrantType:        "authorization_code",
			AttributeCacheID: testCacheID,
			Iat:              int64(suite.validClaims["iat"].(float64)),
		}, nil)

	suite.mockAttrCacheService.On("GetAttributeCache", mock.Anything, testCacheID).
		Return(&attributecache.AttributeCache{ID: testCacheID, Attributes: map[string]interface{}{}},
			(*serviceerror.ServiceError)(nil)).Once()

	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything).Return(&model.TokenDTO{
		Token:    "new.access.token",
		IssuedAt: time.Now().Unix(),
		Scopes:   []string{"read"},
	}, nil)

	// TTL ≈ 82800 + buffer(60) = 82860; allow ±2 s for execution time between test setup and handler call.
	suite.mockAttrCacheService.On("ExtendAttributeCacheTTL", mock.Anything, testCacheID,
		mock.MatchedBy(func(ttl int) bool { return ttl >= 82858 && ttl <= 82862 })).
		Return((*serviceerror.ServiceError)(nil))

	response, err := suite.handler.HandleGrant(context.Background(), suite.testTokenReq, suite.oauthApp)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), response)
	assert.Equal(suite.T(), suite.validRefreshToken, response.RefreshToken.Token)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_ExtendsCache_WhenAccessTokenOutlivesRefreshToken() {
	// iat = now-83000, refreshValidity = 86400 → refresh remaining = 3400 s.
	// Access token ExpiresIn = 7200 > 3400, so the cache must be extended to 7200 s.

	now := time.Now().Unix()
	suite.mockTokenValidator.On("ValidateRefreshToken", suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:              testRefreshTokenUserID,
			Audiences:        []string{testRefreshTokenAudience},
			Scopes:           []string{"read", "write"},
			GrantType:        "authorization_code",
			AttributeCacheID: testCacheID,
			Iat:              now - 83000,
		}, nil)

	suite.mockAttrCacheService.On("GetAttributeCache", mock.Anything, testCacheID).
		Return(&attributecache.AttributeCache{ID: testCacheID, Attributes: map[string]interface{}{}},
			(*serviceerror.ServiceError)(nil)).Once()

	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything).Return(&model.TokenDTO{
		Token:     "new.access.token",
		IssuedAt:  now,
		ExpiresIn: 7200,
		Scopes:    []string{"read"},
	}, nil)

	// Access token expiry (now+7200) > refresh token expiry (now+3400) → TTL = 7200 + buffer(60) = 7260.
	suite.mockAttrCacheService.On("ExtendAttributeCacheTTL", mock.Anything, testCacheID, 7260).
		Return((*serviceerror.ServiceError)(nil))

	response, err := suite.handler.HandleGrant(context.Background(), suite.testTokenReq, suite.oauthApp)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), response)
	assert.Equal(suite.T(), suite.validRefreshToken, response.RefreshToken.Token)
	suite.mockAttrCacheService.AssertCalled(suite.T(), "ExtendAttributeCacheTTL", mock.Anything, testCacheID, 7260)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_NoRenewOnGrant_ExtendCacheTTLError() {
	// Same near-expiry scenario: access token outlives the refresh token, but
	// ExtendAttributeCacheTTL fails → handler returns a server error.

	now := time.Now().Unix()
	suite.mockTokenValidator.On("ValidateRefreshToken", suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:              testRefreshTokenUserID,
			Audiences:        []string{testRefreshTokenAudience},
			Scopes:           []string{"read", "write"},
			GrantType:        "authorization_code",
			AttributeCacheID: testCacheID,
			Iat:              now - 83000,
		}, nil)

	suite.mockAttrCacheService.On("GetAttributeCache", mock.Anything, testCacheID).
		Return(&attributecache.AttributeCache{ID: testCacheID, Attributes: map[string]interface{}{}},
			(*serviceerror.ServiceError)(nil)).Once()

	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything).Return(&model.TokenDTO{
		Token:     "new.access.token",
		IssuedAt:  now,
		ExpiresIn: 7200,
		Scopes:    []string{"read"},
	}, nil)

	extendErr := &serviceerror.ServiceError{
		Type: serviceerror.ServerErrorType,
		Code: "ACS-2001",
		Error: core.I18nMessage{
			Key:          "error.attributecache.internal_server_error",
			DefaultValue: "Internal server error",
		},
	}
	suite.mockAttrCacheService.On("ExtendAttributeCacheTTL", mock.Anything, testCacheID, 7260).
		Return(extendErr)

	response, err := suite.handler.HandleGrant(context.Background(), suite.testTokenReq, suite.oauthApp)

	assert.Nil(suite.T(), response)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorServerError, err.Error)
	assert.Equal(suite.T(), "Failed to extend attribute cache TTL", err.ErrorDescription)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_RenewOnGrant_ExtendsAttributeCacheTTL() {
	config.GetServerRuntime().Config.OAuth.RefreshToken.RenewOnGrant = true

	suite.mockTokenValidator.On("ValidateRefreshToken", suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:              testRefreshTokenUserID,
			Audiences:        []string{testRefreshTokenAudience},
			Scopes:           []string{"read", "write"},
			GrantType:        "authorization_code",
			AttributeCacheID: testCacheID,
			Iat:              int64(suite.validClaims["iat"].(float64)),
		}, nil)

	suite.mockAttrCacheService.On("GetAttributeCache", mock.Anything, testCacheID).
		Return(&attributecache.AttributeCache{ID: testCacheID, Attributes: map[string]interface{}{}},
			(*serviceerror.ServiceError)(nil)).Once()

	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything).Return(&model.TokenDTO{
		Token:          "new.access.token",
		IssuedAt:       time.Now().Unix(),
		ExpiresIn:      3600,
		Scopes:         []string{"read"},
		UserAttributes: map[string]interface{}{"aci": testCacheID},
	}, nil)

	suite.mockTokenBuilder.On("BuildRefreshToken", mock.Anything).Return(&model.TokenDTO{
		Token:     "new.refresh.token",
		IssuedAt:  time.Now().Unix(),
		ExpiresIn: 86400,
		Scopes:    []string{"read"},
	}, nil)

	// Expect TTL to be extended to the refresh token validity period (86400 from config) + buffer(60) = 86460.
	suite.mockAttrCacheService.On("ExtendAttributeCacheTTL", mock.Anything, testCacheID, 86460).
		Return((*serviceerror.ServiceError)(nil))

	response, err := suite.handler.HandleGrant(context.Background(), suite.testTokenReq, suite.oauthApp)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), response)
	assert.Equal(suite.T(), "new.access.token", response.AccessToken.Token)
	assert.Equal(suite.T(), "new.refresh.token", response.RefreshToken.Token)
	suite.mockAttrCacheService.AssertCalled(suite.T(), "ExtendAttributeCacheTTL", mock.Anything, testCacheID, 86460)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_RenewOnGrant_ExtendAttributeCacheTTLError() {
	config.GetServerRuntime().Config.OAuth.RefreshToken.RenewOnGrant = true

	suite.mockTokenValidator.On("ValidateRefreshToken", suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:              testRefreshTokenUserID,
			Audiences:        []string{testRefreshTokenAudience},
			Scopes:           []string{"read", "write"},
			GrantType:        "authorization_code",
			AttributeCacheID: testCacheID,
			Iat:              int64(suite.validClaims["iat"].(float64)),
		}, nil)

	suite.mockAttrCacheService.On("GetAttributeCache", mock.Anything, testCacheID).
		Return(&attributecache.AttributeCache{ID: testCacheID, Attributes: map[string]interface{}{}},
			(*serviceerror.ServiceError)(nil)).Once()

	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything).Return(&model.TokenDTO{
		Token:          "new.access.token",
		IssuedAt:       time.Now().Unix(),
		ExpiresIn:      3600,
		Scopes:         []string{"read"},
		UserAttributes: map[string]interface{}{"aci": testCacheID},
	}, nil)

	suite.mockTokenBuilder.On("BuildRefreshToken", mock.Anything).Return(&model.TokenDTO{
		Token:     "new.refresh.token",
		IssuedAt:  time.Now().Unix(),
		ExpiresIn: 86400,
		Scopes:    []string{"read"},
	}, nil)

	extendErr := &serviceerror.ServiceError{
		Type: serviceerror.ServerErrorType,
		Code: "ACS-2001",
		Error: core.I18nMessage{
			Key:          "error.attributecache.internal_server_error",
			DefaultValue: "Internal server error",
		},
	}
	suite.mockAttrCacheService.On("ExtendAttributeCacheTTL", mock.Anything, testCacheID, 86460).
		Return(extendErr)

	response, err := suite.handler.HandleGrant(context.Background(), suite.testTokenReq, suite.oauthApp)

	assert.Nil(suite.T(), response)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorServerError, err.Error)
	assert.Equal(suite.T(), "Failed to extend attribute cache TTL", err.ErrorDescription)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_SkipsCacheExtend_WhenCurrentTTLAlreadySufficient() {
	// cacheEntry.TTLSeconds (100000) already exceeds the computed desiredTTL
	// (max of refresh remaining ≈ 82800 and access ExpiresIn 3600, plus buffer 60 = ≈ 82860), so
	// ExtendAttributeCacheTTL must not be called.

	suite.mockTokenValidator.On("ValidateRefreshToken", suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:              testRefreshTokenUserID,
			Audiences:        []string{testRefreshTokenAudience},
			Scopes:           []string{"read", "write"},
			GrantType:        "authorization_code",
			AttributeCacheID: testCacheID,
			Iat:              int64(suite.validClaims["iat"].(float64)),
		}, nil)

	suite.mockAttrCacheService.On("GetAttributeCache", mock.Anything, testCacheID).
		Return(&attributecache.AttributeCache{
			ID:         testCacheID,
			Attributes: map[string]interface{}{},
			TTLSeconds: 100000,
		}, (*serviceerror.ServiceError)(nil)).Once()

	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything).Return(&model.TokenDTO{
		Token:     "new.access.token",
		IssuedAt:  time.Now().Unix(),
		ExpiresIn: 3600,
		Scopes:    []string{"read"},
	}, nil)

	response, err := suite.handler.HandleGrant(context.Background(), suite.testTokenReq, suite.oauthApp)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), response)
	suite.mockAttrCacheService.AssertNotCalled(suite.T(), "ExtendAttributeCacheTTL")
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestExtendCacheTTL_NilCacheEntry_NoOp() {
	result := suite.handler.extendCacheTTL(
		context.Background(), nil, suite.oauthApp,
		time.Now().Unix()-3600, 3600, false, testCacheID, log.GetLogger(),
	)

	assert.Nil(suite.T(), result)
	suite.mockAttrCacheService.AssertNotCalled(suite.T(), "ExtendAttributeCacheTTL")
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestExtendCacheTTL_CurrentTTLSufficient_NoExtension() {
	// TTLSeconds (200000) > computed desiredTTL (≈82860) → no extend call.
	cacheEntry := &attributecache.AttributeCache{ID: testCacheID, TTLSeconds: 200000}

	result := suite.handler.extendCacheTTL(
		context.Background(), cacheEntry, suite.oauthApp,
		time.Now().Unix()-3600, 3600, false, testCacheID, log.GetLogger(),
	)

	assert.Nil(suite.T(), result)
	suite.mockAttrCacheService.AssertNotCalled(suite.T(), "ExtendAttributeCacheTTL")
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestExtendCacheTTL_RefreshOutlivesAccess_ExtendsToRefreshExpiry() {
	// iat=now-3600, validity=86400 → remaining≈82800. accessExpiresIn=3600 < 82800.
	// desiredTTL ≈ 82800 + 60 = 82860 (±1 for clock drift).
	cacheEntry := &attributecache.AttributeCache{ID: testCacheID, TTLSeconds: 0}

	suite.mockAttrCacheService.On("ExtendAttributeCacheTTL", mock.Anything, testCacheID,
		mock.MatchedBy(func(ttl int) bool { return ttl >= 82858 && ttl <= 82862 })).
		Return((*serviceerror.ServiceError)(nil))

	result := suite.handler.extendCacheTTL(
		context.Background(), cacheEntry, suite.oauthApp,
		time.Now().Unix()-3600, 3600, false, testCacheID, log.GetLogger(),
	)

	assert.Nil(suite.T(), result)
	suite.mockAttrCacheService.AssertCalled(suite.T(), "ExtendAttributeCacheTTL", mock.Anything, testCacheID,
		mock.MatchedBy(func(ttl int) bool { return ttl >= 82858 && ttl <= 82862 }))
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestExtendCacheTTL_AccessOutlivesRefresh_ExtendsToAccessExpiry() {
	// iat=now-83000, validity=86400 → refresh remaining=3400. accessExpiresIn=7200 > 3400.
	// desiredTTL = 7200 + 60 = 7260.
	cacheEntry := &attributecache.AttributeCache{ID: testCacheID, TTLSeconds: 0}

	suite.mockAttrCacheService.On("ExtendAttributeCacheTTL", mock.Anything, testCacheID, 7260).
		Return((*serviceerror.ServiceError)(nil))

	result := suite.handler.extendCacheTTL(
		context.Background(), cacheEntry, suite.oauthApp,
		time.Now().Unix()-83000, 7200, false, testCacheID, log.GetLogger(),
	)

	assert.Nil(suite.T(), result)
	suite.mockAttrCacheService.AssertCalled(suite.T(), "ExtendAttributeCacheTTL", mock.Anything, testCacheID, 7260)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestExtendCacheTTL_RenewOnGrant_UsesNowAsRefreshIat() {
	// renewRefreshToken=true → refreshIat overridden to now.
	// refreshValidity=86400, accessExpiresIn=3600 < 86400 → desiredTTL = 86400 + 60 = 86460.
	cacheEntry := &attributecache.AttributeCache{ID: testCacheID, TTLSeconds: 0}

	suite.mockAttrCacheService.On("ExtendAttributeCacheTTL", mock.Anything, testCacheID, 86460).
		Return((*serviceerror.ServiceError)(nil))

	result := suite.handler.extendCacheTTL(
		context.Background(), cacheEntry, suite.oauthApp,
		time.Now().Unix()-3600, // stale iat — ignored when renewRefreshToken=true
		3600, true, testCacheID, log.GetLogger(),
	)

	assert.Nil(suite.T(), result)
	suite.mockAttrCacheService.AssertCalled(suite.T(), "ExtendAttributeCacheTTL", mock.Anything, testCacheID, 86460)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestExtendCacheTTL_ExtendFails_ReturnsServerError() {
	cacheEntry := &attributecache.AttributeCache{ID: testCacheID, TTLSeconds: 0}

	extendErr := &serviceerror.ServiceError{
		Type: serviceerror.ServerErrorType,
		Code: "ACS-2001",
		Error: core.I18nMessage{
			Key:          "error.attributecache.internal_server_error",
			DefaultValue: "Internal server error",
		},
	}
	suite.mockAttrCacheService.On("ExtendAttributeCacheTTL", mock.Anything, testCacheID, mock.Anything).
		Return(extendErr)

	result := suite.handler.extendCacheTTL(
		context.Background(), cacheEntry, suite.oauthApp,
		time.Now().Unix()-83000, 7200, false, testCacheID, log.GetLogger(),
	)

	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), constants.ErrorServerError, result.Error)
	assert.Equal(suite.T(), "Failed to extend attribute cache TTL", result.ErrorDescription)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_IDTokenWithRenewOnGrant() {
	// Enable RenewOnGrant in config
	config.GetServerRuntime().Config.OAuth.RefreshToken.RenewOnGrant = true

	// Mock successful refresh token validation with openid scope
	suite.mockTokenValidator.On("ValidateRefreshToken", suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:              testRefreshTokenUserID,
			Audiences:        []string{testRefreshTokenAudience},
			Scopes:           []string{"openid", "read"},
			GrantType:        "authorization_code",
			AttributeCacheID: "",
			Iat:              int64(suite.validClaims["iat"].(float64)),
		}, nil)

	// Mock successful access token generation
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything).Return(&model.TokenDTO{
		Token:          "new.access.token",
		IssuedAt:       time.Now().Unix(),
		ExpiresIn:      3600,
		Scopes:         []string{"openid", "read"},
		UserAttributes: map[string]interface{}{"email": "test@example.com"},
	}, nil)

	// Mock successful ID token generation
	suite.mockTokenBuilder.On("BuildIDToken", mock.MatchedBy(
		func(ctx *tokenservice.IDTokenBuildContext) bool {
			return ctx.Subject == testRefreshTokenUserID &&
				ctx.Audience == testRefreshTokenClientID
		})).Return(&model.TokenDTO{
		Token:     "new.id.token",
		IssuedAt:  time.Now().Unix(),
		ExpiresIn: 3600,
		Scopes:    []string{"openid", "read"},
		ClientID:  testRefreshTokenClientID,
		Subject:   testRefreshTokenUserID,
	}, nil)

	// Mock successful refresh token generation
	suite.mockTokenBuilder.On("BuildRefreshToken", mock.Anything).Return(&model.TokenDTO{
		Token:     "new.refresh.token",
		IssuedAt:  time.Now().Unix(),
		ExpiresIn: 86400,
		Scopes:    []string{"openid", "read"},
	}, nil)

	tokenReq := &model.TokenRequest{
		GrantType:    string(constants.GrantTypeRefreshToken),
		ClientID:     testRefreshTokenClientID,
		RefreshToken: suite.validRefreshToken,
		Scope:        "openid read",
	}

	response, err := suite.handler.HandleGrant(context.Background(), tokenReq, suite.oauthApp)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), response)
	assert.Equal(suite.T(), "new.access.token", response.AccessToken.Token)
	assert.Equal(suite.T(), "new.id.token", response.IDToken.Token)
	assert.Equal(suite.T(), "new.refresh.token", response.RefreshToken.Token)
}

// ============================================================================
// §5 — resource parameter in refresh_token grant (RFC 8707 §2.1)
// ============================================================================

func (suite *RefreshTokenGrantHandlerTestSuite) TestValidateGrant_MalformedResourceURI_ReturnsInvalidTarget() {
	tokenReq := &model.TokenRequest{
		GrantType:    string(constants.GrantTypeRefreshToken),
		ClientID:     testRefreshTokenClientID,
		RefreshToken: suite.validRefreshToken,
		Resources:    []string{"not-an-absolute-uri"},
	}

	err := suite.handler.ValidateGrant(context.Background(), tokenReq, suite.oauthApp)

	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorInvalidTarget, err.Error)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_ResourceNarrowing_KnownResource_NarrowsAud() {
	// Original aud=[rs01, rs02]; request resource=[rs01] → issued aud=[rs01].
	suite.mockTokenValidator.On("ValidateRefreshToken", suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:              testRefreshTokenUserID,
			Audiences:        []string{testRS01URI, testRS02URI},
			Scopes:           []string{"read"},
			GrantType:        "authorization_code",
			AttributeCacheID: "",
			Iat:              int64(suite.validClaims["iat"].(float64)),
		}, nil)

	suite.mockTokenBuilder.On("BuildAccessToken", mock.MatchedBy(
		func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return len(ctx.Audiences) == 1 && ctx.Audiences[0] == testRS01URI
		})).Return(&model.TokenDTO{
		Token:     "new.access.token",
		IssuedAt:  time.Now().Unix(),
		ExpiresIn: 3600,
		Scopes:    []string{"read"},
	}, nil)

	tokenReq := &model.TokenRequest{
		GrantType:    string(constants.GrantTypeRefreshToken),
		ClientID:     testRefreshTokenClientID,
		RefreshToken: suite.validRefreshToken,
		Scope:        "read",
		Resources:    []string{testRS01URI},
	}

	response, err := suite.handler.HandleGrant(context.Background(), tokenReq, suite.oauthApp)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), response)
	assert.Equal(suite.T(), "new.access.token", response.AccessToken.Token)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_ResourceNarrowing_UnknownResource_InvalidTarget() {
	// Original aud=[rs01, rs02]; request resource=[rs99] (unknown) → empty intersection → invalid_target.
	suite.mockTokenValidator.On("ValidateRefreshToken", suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:              testRefreshTokenUserID,
			Audiences:        []string{testRS01URI, testRS02URI},
			Scopes:           []string{"read"},
			GrantType:        "authorization_code",
			AttributeCacheID: "",
			Iat:              int64(suite.validClaims["iat"].(float64)),
		}, nil)

	tokenReq := &model.TokenRequest{
		GrantType:    string(constants.GrantTypeRefreshToken),
		ClientID:     testRefreshTokenClientID,
		RefreshToken: suite.validRefreshToken,
		Scope:        "read",
		Resources:    []string{"https://rs99.example.com"},
	}

	response, err := suite.handler.HandleGrant(context.Background(), tokenReq, suite.oauthApp)

	assert.Nil(suite.T(), response)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorInvalidTarget, err.Error)
	assert.Equal(suite.T(), "Requested resources do not match any audience in the original grant", err.ErrorDescription)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_ResourceNarrowing_MixedResources_DropsUnknown() {
	// Original aud=[rs01, rs02]; request resource=[rs99, rs01] → issued aud=[rs01] (rs99 dropped).
	suite.mockTokenValidator.On("ValidateRefreshToken", suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:              testRefreshTokenUserID,
			Audiences:        []string{testRS01URI, testRS02URI},
			Scopes:           []string{"read"},
			GrantType:        "authorization_code",
			AttributeCacheID: "",
			Iat:              int64(suite.validClaims["iat"].(float64)),
		}, nil)

	suite.mockTokenBuilder.On("BuildAccessToken", mock.MatchedBy(
		func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return len(ctx.Audiences) == 1 && ctx.Audiences[0] == testRS01URI
		})).Return(&model.TokenDTO{
		Token:     "new.access.token",
		IssuedAt:  time.Now().Unix(),
		ExpiresIn: 3600,
		Scopes:    []string{"read"},
	}, nil)

	tokenReq := &model.TokenRequest{
		GrantType:    string(constants.GrantTypeRefreshToken),
		ClientID:     testRefreshTokenClientID,
		RefreshToken: suite.validRefreshToken,
		Scope:        "read",
		Resources:    []string{"https://rs99.example.com", testRS01URI},
	}

	response, err := suite.handler.HandleGrant(context.Background(), tokenReq, suite.oauthApp)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), response)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_NoResourceParam_AudUnchanged() {
	// No resource param → issued aud equals original aud (regression guard).
	suite.mockTokenValidator.On("ValidateRefreshToken", suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:              testRefreshTokenUserID,
			Audiences:        []string{testRS01URI, testRS02URI},
			Scopes:           []string{"read"},
			GrantType:        "authorization_code",
			AttributeCacheID: "",
			Iat:              int64(suite.validClaims["iat"].(float64)),
		}, nil)

	suite.mockTokenBuilder.On("BuildAccessToken", mock.MatchedBy(
		func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return len(ctx.Audiences) == 2 &&
				ctx.Audiences[0] == testRS01URI &&
				ctx.Audiences[1] == testRS02URI
		})).Return(&model.TokenDTO{
		Token:     "new.access.token",
		IssuedAt:  time.Now().Unix(),
		ExpiresIn: 3600,
		Scopes:    []string{"read"},
	}, nil)

	tokenReq := &model.TokenRequest{
		GrantType:    string(constants.GrantTypeRefreshToken),
		ClientID:     testRefreshTokenClientID,
		RefreshToken: suite.validRefreshToken,
		Scope:        "read",
	}

	response, err := suite.handler.HandleGrant(context.Background(), tokenReq, suite.oauthApp)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), response)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_RenewOnGrant_OriginalAudPreservedInNewRefreshToken() {
	// RFC 8707 §5: when renewRefreshToken=true and narrowing occurs, the new refresh token must
	// carry the ORIGINAL (un-narrowed) audiences so future refreshes can recover dropped resources.
	config.GetServerRuntime().Config.OAuth.RefreshToken.RenewOnGrant = true

	suite.mockTokenValidator.On("ValidateRefreshToken", suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:              testRefreshTokenUserID,
			Audiences:        []string{testRS01URI, testRS02URI},
			Scopes:           []string{"read"},
			GrantType:        "authorization_code",
			AttributeCacheID: "",
			Iat:              int64(suite.validClaims["iat"].(float64)),
		}, nil)

	suite.mockTokenBuilder.On("BuildAccessToken", mock.MatchedBy(
		func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return len(ctx.Audiences) == 1 && ctx.Audiences[0] == testRS01URI
		})).Return(&model.TokenDTO{
		Token:     "new.access.token",
		IssuedAt:  time.Now().Unix(),
		ExpiresIn: 3600,
		Scopes:    []string{"read"},
	}, nil)

	// New refresh token must carry the original full aud [rs01, rs02], not only the narrowed rs01.
	suite.mockTokenBuilder.On("BuildRefreshToken", mock.MatchedBy(
		func(ctx *tokenservice.RefreshTokenBuildContext) bool {
			return len(ctx.AccessTokenAudiences) == 2 &&
				ctx.AccessTokenAudiences[0] == testRS01URI &&
				ctx.AccessTokenAudiences[1] == testRS02URI
		})).Return(&model.TokenDTO{
		Token:     "new.refresh.token",
		IssuedAt:  time.Now().Unix(),
		ExpiresIn: 86400,
		Scopes:    []string{"read"},
	}, nil)

	tokenReq := &model.TokenRequest{
		GrantType:    string(constants.GrantTypeRefreshToken),
		ClientID:     testRefreshTokenClientID,
		RefreshToken: suite.validRefreshToken,
		Scope:        "read",
		Resources:    []string{testRS01URI},
	}

	response, err := suite.handler.HandleGrant(context.Background(), tokenReq, suite.oauthApp)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), response)
	assert.Equal(suite.T(), "new.refresh.token", response.RefreshToken.Token)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_ResourceNarrowing_EmptyIntersection_InvalidTarget() {
	// All requested resources are outside the original grant → invalid_target (Issue 2).
	suite.mockTokenValidator.On("ValidateRefreshToken", suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:              testRefreshTokenUserID,
			Audiences:        []string{testRS01URI},
			Scopes:           []string{"read"},
			GrantType:        "authorization_code",
			AttributeCacheID: "",
			Iat:              int64(suite.validClaims["iat"].(float64)),
		}, nil)

	tokenReq := &model.TokenRequest{
		GrantType:    string(constants.GrantTypeRefreshToken),
		ClientID:     testRefreshTokenClientID,
		RefreshToken: suite.validRefreshToken,
		Scope:        "read",
		Resources:    []string{testRS02URI},
	}

	response, err := suite.handler.HandleGrant(context.Background(), tokenReq, suite.oauthApp)

	assert.Nil(suite.T(), response)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorInvalidTarget, err.Error)
	assert.Equal(suite.T(), "Requested resources do not match any audience in the original grant", err.ErrorDescription)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_ResourceNarrowing_ScopeDownscoped() {
	// Original aud=[rs01, rs02] with scopes [read write]; narrow to rs01 only.
	// ValidatePermissions returns "write" as invalid for rs01, so access token must carry only "read".
	suite.mockResourceService.ExpectedCalls = nil
	suite.mockResourceService.On("GetResourceServerByIdentifier", mock.Anything, testRS01URI).
		Return(&resource.ResourceServer{ID: testRS01URI, Identifier: testRS01URI}, nil)
	suite.mockResourceService.On("ValidatePermissions", mock.Anything, testRS01URI, mock.Anything).
		Return([]string{"write"}, nil)

	suite.mockTokenValidator.On("ValidateRefreshToken", suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:              testRefreshTokenUserID,
			Audiences:        []string{testRS01URI, testRS02URI},
			Scopes:           []string{"read", "write"},
			GrantType:        "authorization_code",
			AttributeCacheID: "",
			Iat:              int64(suite.validClaims["iat"].(float64)),
		}, nil)

	suite.mockTokenBuilder.On("BuildAccessToken", mock.MatchedBy(
		func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return len(ctx.Audiences) == 1 && ctx.Audiences[0] == testRS01URI &&
				len(ctx.Scopes) == 1 && ctx.Scopes[0] == testScopeRead
		})).Return(&model.TokenDTO{
		Token:     "new.access.token",
		IssuedAt:  time.Now().Unix(),
		ExpiresIn: 3600,
		Scopes:    []string{"read"},
	}, nil)

	tokenReq := &model.TokenRequest{
		GrantType:    string(constants.GrantTypeRefreshToken),
		ClientID:     testRefreshTokenClientID,
		RefreshToken: suite.validRefreshToken,
		Scope:        "read write",
		Resources:    []string{testRS01URI},
	}

	response, err := suite.handler.HandleGrant(context.Background(), tokenReq, suite.oauthApp)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), response)
	assert.Equal(suite.T(), "new.access.token", response.AccessToken.Token)
}
