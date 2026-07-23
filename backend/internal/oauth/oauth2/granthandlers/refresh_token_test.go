/*
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/attributecache"
	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/dpop"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/revocation"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/attributecachemock"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/revocationmock"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/tokenservicemock"
	"github.com/thunder-id/thunderid/tests/mocks/resourcemock"
	"github.com/thunder-id/thunderid/tests/mocks/serverconfigmock"
	"github.com/thunder-id/thunderid/tests/testhelpers"
)

// testUserID and testAudience are declared in tokenexchange_test.go
const testRefreshTokenUserID = "test-user-id"
const testRefreshTokenAudience = "test-audience"
const testRefreshTokenClientID = "test-client-id"
const testRS01URI = "https://rs01.example.com"
const testRS02URI = "https://rs02.example.com"

type RefreshTokenGrantHandlerTestSuite struct {
	testCfg oauthconfig.Config
	suite.Suite
	handler              *refreshTokenGrantHandler
	mockJWTService       *jwtmock.JWTServiceInterfaceMock
	mockTokenBuilder     *tokenservicemock.TokenBuilderInterfaceMock
	mockTokenValidator   *tokenservicemock.TokenValidatorInterfaceMock
	mockAttrCacheService *attributecachemock.AttributeCacheServiceInterfaceMock
	mockResourceService  *resourcemock.ResourceServiceInterfaceMock
	mockServerConfigSvc  *serverconfigmock.ServerConfigServiceMock
	mockRefreshRevoker   *revocationmock.RefreshTokenRevokerInterfaceMock
	oauthApp             *providers.OAuthClient
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
		JWT: engineconfig.JWTConfig{
			ValidityPeriod: 3600,
		},
		OAuth: engineconfig.OAuthConfig{
			RefreshToken: engineconfig.RefreshTokenConfig{
				ValidityPeriod: 86400,
				RenewOnGrant:   false,
			},
		},
	}
	_ = config.InitializeServerRuntime("test", testConfig)

	suite.testCfg = testhelpers.OAuthConfig()
	suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())
	suite.mockTokenBuilder = tokenservicemock.NewTokenBuilderInterfaceMock(suite.T())
	suite.mockTokenValidator = tokenservicemock.NewTokenValidatorInterfaceMock(suite.T())
	suite.mockAttrCacheService = attributecachemock.NewAttributeCacheServiceInterfaceMock(suite.T())
	suite.mockResourceService = resourcemock.NewResourceServiceInterfaceMock(suite.T())
	suite.mockServerConfigSvc = serverconfigmock.NewServerConfigServiceMock(suite.T())
	suite.mockRefreshRevoker = revocationmock.NewRefreshTokenRevokerInterfaceMock(suite.T())

	suite.mockResourceService.On("GetResourceServerByIdentifier", mock.Anything, mock.Anything).
		Return(func(_ context.Context, identifier string) *providers.ResourceServer {
			return &providers.ResourceServer{ID: identifier, Identifier: identifier}
		}, func(_ context.Context, _ string) *tidcommon.ServiceError {
			return nil
		}).Maybe()
	suite.mockResourceService.On("ValidatePermissions", mock.Anything, mock.Anything, mock.Anything).
		Return([]string{}, nil).Maybe()

	suite.rebuildHandlerWithConfig()

	suite.oauthApp = &providers.OAuthClient{
		ClientID:                testRefreshTokenClientID,
		GrantTypes:              []providers.GrantType{providers.GrantTypeRefreshToken},
		TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretPost,
		Token: &providers.OAuthTokenConfig{
			AccessToken: &providers.AccessTokenConfig{
				UserConfig: &providers.AccessTokenSubConfig{
					Attributes: []string{"email", "username"},
				},
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
		GrantType:    string(providers.GrantTypeRefreshToken),
		ClientID:     testRefreshTokenClientID,
		RefreshToken: suite.validRefreshToken,
		Scope:        "read",
	}
}

func (suite *RefreshTokenGrantHandlerTestSuite) rebuildHandlerWithConfig() {
	suite.handler = newRefreshTokenGrantHandler(
		suite.mockJWTService,
		suite.mockTokenBuilder,
		suite.mockTokenValidator,
		suite.mockAttrCacheService,
		suite.mockResourceService,
		suite.mockServerConfigSvc,
		suite.mockRefreshRevoker,
		suite.testCfg,
	).(*refreshTokenGrantHandler)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestNewRefreshTokenGrantHandler() {
	handler := newRefreshTokenGrantHandler(suite.mockJWTService,
		suite.mockTokenBuilder,
		suite.mockTokenValidator,
		suite.mockAttrCacheService,
		suite.mockResourceService, suite.mockServerConfigSvc, suite.mockRefreshRevoker, testhelpers.OAuthConfig())
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
		GrantType: string(providers.GrantTypeRefreshToken),
		ClientID:  testRefreshTokenClientID,
	}

	err := suite.handler.ValidateGrant(context.Background(), tokenReq, suite.oauthApp)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, err.Error)
	assert.Equal(suite.T(), "Refresh token is required", err.ErrorDescription)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestValidateGrant_MissingClientID() {
	tokenReq := &model.TokenRequest{
		GrantType:    string(providers.GrantTypeRefreshToken),
		RefreshToken: "token",
	}

	err := suite.handler.ValidateGrant(context.Background(), tokenReq, suite.oauthApp)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, err.Error)
	assert.Equal(suite.T(), "Client ID is required", err.ErrorDescription)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_InvalidSignature() {
	// Mock token validator to return error (simulating signature verification failure)
	suite.mockTokenValidator.
		On("ValidateRefreshToken", mock.Anything, suite.validRefreshToken, testRefreshTokenClientID).
		Return(nil, errors.New("public key not available"))

	response, err := suite.handler.HandleGrant(context.Background(), suite.testTokenReq, suite.oauthApp)

	assert.Nil(suite.T(), response)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorInvalidGrant, err.Error)
	assert.Equal(suite.T(), "Invalid refresh token", err.ErrorDescription)
}

// A revoked refresh token is rejected with invalid_grant. The validator enforces the RFC 7009 deny
// list and surfaces ErrTokenRevoked, which the grant handler maps to invalid_grant.
func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_RevokedRefreshToken() {
	suite.mockTokenValidator.
		On("ValidateRefreshToken", mock.Anything, suite.validRefreshToken, testRefreshTokenClientID).
		Return(nil, revocation.ErrTokenRevoked)

	response, err := suite.handler.HandleGrant(context.Background(), suite.testTokenReq, suite.oauthApp)

	assert.Nil(suite.T(), response)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorInvalidGrant, err.Error)
}

// When the deny list cannot be consulted, the validator surfaces ErrEnforcementUnavailable and the
// refresh grant fails closed with server_error.
func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_EnforcementUnavailableFailsClosed() {
	suite.mockTokenValidator.
		On("ValidateRefreshToken", mock.Anything, suite.validRefreshToken, testRefreshTokenClientID).
		Return(nil, revocation.ErrEnforcementUnavailable)

	response, err := suite.handler.HandleGrant(context.Background(), suite.testTokenReq, suite.oauthApp)

	assert.Nil(suite.T(), response)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorServerError, err.Error)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestIssueRefreshToken_Success() {
	// Mock token builder for refresh token generation
	suite.mockTokenBuilder.On("BuildRefreshToken", mock.Anything, mock.MatchedBy(
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
	suite.mockTokenBuilder.On("BuildRefreshToken", mock.Anything, mock.Anything).
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
	suite.mockTokenBuilder.On("BuildRefreshToken", mock.Anything, mock.MatchedBy(
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
	suite.mockTokenBuilder.On("BuildRefreshToken", mock.Anything, mock.MatchedBy(
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

func (suite *RefreshTokenGrantHandlerTestSuite) TestIssueRefreshToken_AgentClientFreezesActorSub() {
	const actAppID = "act-entity-id"
	agentApp := &providers.OAuthClient{
		ID:                      actAppID,
		ClientID:                testRefreshTokenClientID,
		EntityCategory:          "agent",
		GrantTypes:              []providers.GrantType{providers.GrantTypeRefreshToken},
		TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretPost,
	}

	var capturedActorSub string
	suite.mockTokenBuilder.On("BuildRefreshToken", mock.Anything, mock.MatchedBy(
		func(ctx *tokenservice.RefreshTokenBuildContext) bool {
			capturedActorSub = ctx.ActorSub
			return true
		})).Return(&model.TokenDTO{
		Token:    "new.refresh.token",
		IssuedAt: int64(1234567890),
	}, nil)

	tokenResponse := &model.TokenResponseDTO{}
	err := suite.handler.IssueRefreshToken(context.Background(), tokenResponse, agentApp,
		testRefreshTokenUserID, []string{testRefreshTokenAudience},
		"authorization_code", []string{"read"}, nil, "", "")

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), actAppID, capturedActorSub)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestIssueRefreshToken_AppClientWithoutFlagOmitsActorSub() {
	appApp := &providers.OAuthClient{
		ID:                      "app-entity-id",
		ClientID:                testRefreshTokenClientID,
		EntityCategory:          "app",
		IncludeActClaim:         false,
		GrantTypes:              []providers.GrantType{providers.GrantTypeRefreshToken},
		TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretPost,
	}

	var capturedActorSub string
	suite.mockTokenBuilder.On("BuildRefreshToken", mock.Anything, mock.MatchedBy(
		func(ctx *tokenservice.RefreshTokenBuildContext) bool {
			capturedActorSub = ctx.ActorSub
			return true
		})).Return(&model.TokenDTO{
		Token:    "new.refresh.token",
		IssuedAt: int64(1234567890),
	}, nil)

	tokenResponse := &model.TokenResponseDTO{}
	err := suite.handler.IssueRefreshToken(context.Background(), tokenResponse, appApp,
		testRefreshTokenUserID, []string{testRefreshTokenAudience},
		"authorization_code", []string{"read"}, nil, "", "")

	assert.Nil(suite.T(), err)
	assert.Empty(suite.T(), capturedActorSub)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_ReplaysActorSubFromStoredMarker() {
	const actAppID = "act-entity-id"
	suite.mockTokenValidator.
		On("ValidateRefreshToken", mock.Anything, suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:              testRefreshTokenUserID,
			Audiences:        []string{testRefreshTokenAudience},
			Scopes:           []string{"read", "write"},
			GrantType:        "authorization_code",
			AttributeCacheID: "",
			Iat:              int64(suite.validClaims["iat"].(float64)),
			ActorSub:         actAppID,
		}, nil)

	var capturedActor *tokenservice.SubjectTokenClaims
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything, mock.MatchedBy(
		func(ctx *tokenservice.AccessTokenBuildContext) bool {
			capturedActor = ctx.ActorClaims
			return ctx.Subject == testRefreshTokenUserID
		})).Return(&model.TokenDTO{
		Token:     "new.access.token",
		IssuedAt:  time.Now().Unix(),
		ExpiresIn: 3600,
		Scopes:    []string{"read"},
	}, nil)

	response, err := suite.handler.HandleGrant(context.Background(), suite.testTokenReq, suite.oauthApp)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), response)
	assert.NotNil(suite.T(), capturedActor)
	assert.Equal(suite.T(), actAppID, capturedActor.Sub)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_NoActorSubMarker_OmitsActorEvenWhenFlagOn() {
	// Freeze-at-issuance: a refresh token issued without the marker must not gain an act claim
	// even if the client now opts into act claims.
	appWithFlagOn := &providers.OAuthClient{
		ID:                      "app-entity-id",
		ClientID:                testRefreshTokenClientID,
		EntityCategory:          "app",
		IncludeActClaim:         true,
		GrantTypes:              []providers.GrantType{providers.GrantTypeRefreshToken},
		TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretPost,
	}

	suite.mockTokenValidator.
		On("ValidateRefreshToken", mock.Anything, suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:              testRefreshTokenUserID,
			Audiences:        []string{testRefreshTokenAudience},
			Scopes:           []string{"read", "write"},
			GrantType:        "authorization_code",
			AttributeCacheID: "",
			Iat:              int64(suite.validClaims["iat"].(float64)),
		}, nil)

	var capturedActor *tokenservice.SubjectTokenClaims
	hadActor := false
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything, mock.MatchedBy(
		func(ctx *tokenservice.AccessTokenBuildContext) bool {
			capturedActor = ctx.ActorClaims
			hadActor = true
			return ctx.Subject == testRefreshTokenUserID
		})).Return(&model.TokenDTO{
		Token:     "new.access.token",
		IssuedAt:  time.Now().Unix(),
		ExpiresIn: 3600,
		Scopes:    []string{"read"},
	}, nil)

	response, err := suite.handler.HandleGrant(context.Background(), suite.testTokenReq, appWithFlagOn)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), response)
	assert.True(suite.T(), hadActor)
	assert.Nil(suite.T(), capturedActor)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_Success_WithRenewOnGrantDisabled() {
	// Mock successful refresh token validation
	suite.mockTokenValidator.
		On("ValidateRefreshToken", mock.Anything, suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:              testRefreshTokenUserID,
			Audiences:        []string{testRefreshTokenAudience},
			Scopes:           []string{"read", "write"},
			GrantType:        "authorization_code",
			AttributeCacheID: "",
			Iat:              int64(suite.validClaims["iat"].(float64)),
		}, nil)

	// Mock successful access token generation
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything, mock.MatchedBy(
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

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_RenewRevokesConsumedRefreshToken() {
	suite.testCfg.OAuth.RefreshToken.RenewOnGrant = true
	suite.testCfg.OAuth.RefreshToken.RevokePreviousOnRenew = true
	suite.rebuildHandlerWithConfig()

	consumedJTI := "consumed-rt-jti"
	exp := int64(suite.validClaims["exp"].(float64))
	suite.mockTokenValidator.
		On("ValidateRefreshToken", mock.Anything, suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:       testRefreshTokenUserID,
			Audiences: []string{testRefreshTokenAudience},
			Scopes:    []string{"read", "write"},
			GrantType: "authorization_code",
			Iat:       int64(suite.validClaims["iat"].(float64)),
			JTI:       consumedJTI,
			Exp:       exp,
		}, nil)
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything, mock.Anything).Return(&model.TokenDTO{
		Token: "new.access.token", IssuedAt: time.Now().Unix(), ExpiresIn: 3600, Scopes: []string{"read"},
	}, nil)
	suite.mockTokenBuilder.On("BuildRefreshToken", mock.Anything, mock.Anything).Return(&model.TokenDTO{
		Token: "new.refresh.token", IssuedAt: time.Now().Unix(), ExpiresIn: 86400, Scopes: []string{"read", "write"},
	}, nil)
	// Single-use: the consumed refresh token is revoked by its own jti and original expiry.
	suite.mockRefreshRevoker.
		On("RevokeRefreshToken", mock.Anything, consumedJTI, time.Unix(exp, 0).UTC()).
		Return(nil)

	response, err := suite.handler.HandleGrant(context.Background(), suite.testTokenReq, suite.oauthApp)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), response)
	assert.Equal(suite.T(), "new.refresh.token", response.RefreshToken.Token)
	suite.mockRefreshRevoker.AssertCalled(suite.T(), "RevokeRefreshToken",
		mock.Anything, consumedJTI, time.Unix(exp, 0).UTC())
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_RenewRevokeFailureFailsClosed() {
	suite.testCfg.OAuth.RefreshToken.RenewOnGrant = true
	suite.testCfg.OAuth.RefreshToken.RevokePreviousOnRenew = true
	suite.rebuildHandlerWithConfig()

	suite.mockTokenValidator.
		On("ValidateRefreshToken", mock.Anything, suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:       testRefreshTokenUserID,
			Audiences: []string{testRefreshTokenAudience},
			Scopes:    []string{"read", "write"},
			GrantType: "authorization_code",
			Iat:       int64(suite.validClaims["iat"].(float64)),
			JTI:       "consumed-rt-jti",
			Exp:       int64(suite.validClaims["exp"].(float64)),
		}, nil)
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything, mock.Anything).Return(&model.TokenDTO{
		Token: "new.access.token", IssuedAt: time.Now().Unix(), ExpiresIn: 3600, Scopes: []string{"read"},
	}, nil)
	suite.mockTokenBuilder.On("BuildRefreshToken", mock.Anything, mock.Anything).Return(&model.TokenDTO{
		Token: "new.refresh.token", IssuedAt: time.Now().Unix(), ExpiresIn: 86400, Scopes: []string{"read", "write"},
	}, nil)
	// The deny-list write fails; the rotation must fail closed rather than leave the old token usable.
	suite.mockRefreshRevoker.
		On("RevokeRefreshToken", mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("runtime persistent database unavailable"))

	response, err := suite.handler.HandleGrant(context.Background(), suite.testTokenReq, suite.oauthApp)

	assert.Nil(suite.T(), response)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorServerError, err.Error)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_RevokePreviousOnRenew_NilRefreshRevoker() {
	// When the token_revocation feature is disabled, refreshRevoker is nil even though
	// renew_on_grant/revoke_previous_on_renew are independently configured. The handler must
	// skip revocation rather than dereference the nil revoker.
	suite.testCfg.OAuth.RefreshToken.RenewOnGrant = true
	suite.testCfg.OAuth.RefreshToken.RevokePreviousOnRenew = true
	suite.handler = newRefreshTokenGrantHandler(
		suite.mockJWTService,
		suite.mockTokenBuilder,
		suite.mockTokenValidator,
		suite.mockAttrCacheService,
		suite.mockResourceService,
		suite.mockServerConfigSvc,
		nil,
		suite.testCfg,
	).(*refreshTokenGrantHandler)

	suite.mockTokenValidator.
		On("ValidateRefreshToken", mock.Anything, suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:       testRefreshTokenUserID,
			Audiences: []string{testRefreshTokenAudience},
			Scopes:    []string{"read", "write"},
			GrantType: "authorization_code",
			Iat:       int64(suite.validClaims["iat"].(float64)),
			JTI:       "consumed-rt-jti",
			Exp:       int64(suite.validClaims["exp"].(float64)),
		}, nil)
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything, mock.Anything).Return(&model.TokenDTO{
		Token: "new.access.token", IssuedAt: time.Now().Unix(), ExpiresIn: 3600, Scopes: []string{"read"},
	}, nil)
	suite.mockTokenBuilder.On("BuildRefreshToken", mock.Anything, mock.Anything).Return(&model.TokenDTO{
		Token: "new.refresh.token", IssuedAt: time.Now().Unix(), ExpiresIn: 86400, Scopes: []string{"read", "write"},
	}, nil)

	response, err := suite.handler.HandleGrant(context.Background(), suite.testTokenReq, suite.oauthApp)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), response)
	assert.Equal(suite.T(), "new.refresh.token", response.RefreshToken.Token)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_Success_WithRenewOnGrantEnabled() {
	// Enable RenewOnGrant in config
	suite.testCfg.OAuth.RefreshToken.RenewOnGrant = true
	suite.rebuildHandlerWithConfig()

	// Mock successful refresh token validation
	suite.mockTokenValidator.
		On("ValidateRefreshToken", mock.Anything, suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:              testRefreshTokenUserID,
			Audiences:        []string{testRefreshTokenAudience},
			Scopes:           []string{"read", "write"},
			GrantType:        "authorization_code",
			AttributeCacheID: "",
			Iat:              int64(suite.validClaims["iat"].(float64)),
		}, nil)

	// Mock successful access token generation
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything, mock.Anything).Return(&model.TokenDTO{
		Token:          "new.access.token",
		IssuedAt:       time.Now().Unix(),
		ExpiresIn:      3600,
		Scopes:         []string{"read"},
		UserAttributes: map[string]interface{}{"email": "test@example.com"},
	}, nil)

	// Mock successful refresh token generation
	suite.mockTokenBuilder.On("BuildRefreshToken", mock.Anything, mock.MatchedBy(
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
	suite.mockTokenValidator.
		On("ValidateRefreshToken", mock.Anything, suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:              testRefreshTokenUserID,
			Audiences:        []string{testRefreshTokenAudience},
			Scopes:           []string{"read", "write"},
			GrantType:        "authorization_code",
			AttributeCacheID: testCacheID,
			Iat:              int64(suite.validClaims["iat"].(float64)),
		}, nil)

	cacheErr := &tidcommon.ServiceError{
		Type: tidcommon.ServerErrorType,
		Code: "ACS-2001",
		Error: tidcommon.I18nMessage{
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
	suite.mockTokenValidator.
		On("ValidateRefreshToken", mock.Anything, suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:              testRefreshTokenUserID,
			Audiences:        []string{testRefreshTokenAudience},
			Scopes:           []string{"read"},
			GrantType:        "authorization_code",
			AttributeCacheID: "",
			Iat:              int64(suite.validClaims["iat"].(float64)),
		}, nil)

	// Mock failed access token generation
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything, mock.Anything).
		Return(nil, errors.New("failed to sign JWT"))

	response, err := suite.handler.HandleGrant(context.Background(), suite.testTokenReq, suite.oauthApp)

	assert.Nil(suite.T(), response)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorServerError, err.Error)
	assert.Equal(suite.T(), "Failed to generate access token", err.ErrorDescription)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_IssueRefreshTokenError() {
	// Enable RenewOnGrant in config
	suite.testCfg.OAuth.RefreshToken.RenewOnGrant = true
	suite.rebuildHandlerWithConfig()

	// Mock successful refresh token validation
	suite.mockTokenValidator.
		On("ValidateRefreshToken", mock.Anything, suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:              testRefreshTokenUserID,
			Audiences:        []string{testRefreshTokenAudience},
			Scopes:           []string{"read"},
			GrantType:        "authorization_code",
			AttributeCacheID: "",
			Iat:              int64(suite.validClaims["iat"].(float64)),
		}, nil)

	// Mock successful access token generation
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything, mock.Anything).Return(&model.TokenDTO{
		Token:          "new.access.token",
		IssuedAt:       time.Now().Unix(),
		ExpiresIn:      3600,
		Scopes:         []string{"read"},
		UserAttributes: map[string]interface{}{},
	}, nil)

	// Mock failed refresh token generation
	suite.mockTokenBuilder.On("BuildRefreshToken", mock.Anything, mock.Anything).
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
	suite.mockTokenValidator.
		On("ValidateRefreshToken", mock.Anything, suite.validRefreshToken, testRefreshTokenClientID).
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

	result, errResp := suite.handler.validateAndApplyScopes(context.Background(), "", refreshTokenScopes, logger)

	assert.Nil(suite.T(), errResp)
	assert.Equal(suite.T(), refreshTokenScopes, result)
	assert.Len(suite.T(), result, 3)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestValidateAndApplyScopes_RequestedScopesSubset() {
	refreshTokenScopes := []string{"read", "write", "delete"}
	logger := log.GetLogger()

	result, errResp := suite.handler.validateAndApplyScopes(
		context.Background(),
		"read write",
		refreshTokenScopes,
		logger)

	assert.Nil(suite.T(), errResp)
	assert.Len(suite.T(), result, 2)
	assert.Contains(suite.T(), result, "read")
	assert.Contains(suite.T(), result, "write")
	assert.NotContains(suite.T(), result, "delete")
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestValidateAndApplyScopes_SomeRequestedScopesNotInRefreshToken() {
	refreshTokenScopes := []string{"read", "write"}
	logger := log.GetLogger()

	result, errResp := suite.handler.validateAndApplyScopes(
		context.Background(),
		"read write delete admin",
		refreshTokenScopes,
		logger)

	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorInvalidScope, errResp.Error)
	assert.Nil(suite.T(), result)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestValidateAndApplyScopes_NoMatchingScopes() {
	refreshTokenScopes := []string{"read", "write"}
	logger := log.GetLogger()

	result, errResp := suite.handler.validateAndApplyScopes(
		context.Background(),
		"admin delete",
		refreshTokenScopes,
		logger)

	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorInvalidScope, errResp.Error)
	assert.Nil(suite.T(), result)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_IDTokenGenerated_WhenOpenIDScopePresent() {
	// Mock successful refresh token validation with openid scope
	suite.mockTokenValidator.
		On("ValidateRefreshToken", mock.Anything, suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:              testRefreshTokenUserID,
			Audiences:        []string{testRefreshTokenAudience},
			Scopes:           []string{"openid", "read"},
			GrantType:        "authorization_code",
			AttributeCacheID: "",
			Iat:              int64(suite.validClaims["iat"].(float64)),
		}, nil)

	// Mock successful access token generation
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything, mock.Anything).Return(&model.TokenDTO{
		Token:          "new.access.token",
		IssuedAt:       time.Now().Unix(),
		ExpiresIn:      3600,
		Scopes:         []string{"openid", "read"},
		UserAttributes: map[string]interface{}{"email": "test@example.com"},
	}, nil)

	// Mock successful ID token generation
	suite.mockTokenBuilder.On("BuildIDToken", mock.Anything, mock.MatchedBy(
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
		GrantType:    string(providers.GrantTypeRefreshToken),
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
	suite.mockTokenValidator.
		On("ValidateRefreshToken", mock.Anything, suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:              testRefreshTokenUserID,
			Audiences:        []string{testRefreshTokenAudience},
			Scopes:           []string{"read", "write"},
			GrantType:        "authorization_code",
			AttributeCacheID: "",
			Iat:              int64(suite.validClaims["iat"].(float64)),
		}, nil)

	// Mock successful access token generation
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything, mock.MatchedBy(
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
	suite.mockTokenValidator.
		On("ValidateRefreshToken", mock.Anything, suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:              testRefreshTokenUserID,
			Audiences:        []string{testRefreshTokenAudience},
			Scopes:           []string{"openid", "read"},
			GrantType:        "authorization_code",
			AttributeCacheID: "",
			Iat:              int64(suite.validClaims["iat"].(float64)),
		}, nil)

	// Mock successful access token generation
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything, mock.Anything).Return(&model.TokenDTO{
		Token:     "new.access.token",
		IssuedAt:  time.Now().Unix(),
		ExpiresIn: 3600,
		Scopes:    []string{"openid", "read"},
	}, nil)

	// Mock failed ID token generation
	suite.mockTokenBuilder.On("BuildIDToken", mock.Anything, mock.Anything).
		Return(nil, errors.New("failed to generate ID token"))

	tokenReq := &model.TokenRequest{
		GrantType:    string(providers.GrantTypeRefreshToken),
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

	suite.mockTokenValidator.
		On("ValidateRefreshToken", mock.Anything, suite.validRefreshToken, testRefreshTokenClientID).
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
			(*tidcommon.ServiceError)(nil)).Once()

	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything, mock.Anything).Return(&model.TokenDTO{
		Token:    "new.access.token",
		IssuedAt: time.Now().Unix(),
		Scopes:   []string{"read"},
	}, nil)

	// TTL ≈ 82800 + buffer(60) = 82860; allow ±2 s for execution time between test setup and handler call.
	suite.mockAttrCacheService.On("ExtendAttributeCacheTTL", mock.Anything, testCacheID,
		mock.MatchedBy(func(ttl int) bool { return ttl >= 82858 && ttl <= 82862 })).
		Return((*tidcommon.ServiceError)(nil))

	response, err := suite.handler.HandleGrant(context.Background(), suite.testTokenReq, suite.oauthApp)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), response)
	assert.Equal(suite.T(), suite.validRefreshToken, response.RefreshToken.Token)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_ExtendsCache_WhenAccessTokenOutlivesRefreshToken() {
	// iat = now-83000, refreshValidity = 86400 → refresh remaining = 3400 s.
	// Access token ExpiresIn = 7200 > 3400, so the cache must be extended to 7200 s.

	now := time.Now().Unix()
	suite.mockTokenValidator.
		On("ValidateRefreshToken", mock.Anything, suite.validRefreshToken, testRefreshTokenClientID).
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
			(*tidcommon.ServiceError)(nil)).Once()

	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything, mock.Anything).Return(&model.TokenDTO{
		Token:     "new.access.token",
		IssuedAt:  now,
		ExpiresIn: 7200,
		Scopes:    []string{"read"},
	}, nil)

	// Access token expiry (now+7200) > refresh token expiry (now+3400) → TTL = 7200 + buffer(60) = 7260.
	suite.mockAttrCacheService.On("ExtendAttributeCacheTTL", mock.Anything, testCacheID, 7260).
		Return((*tidcommon.ServiceError)(nil))

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
	suite.mockTokenValidator.
		On("ValidateRefreshToken", mock.Anything, suite.validRefreshToken, testRefreshTokenClientID).
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
			(*tidcommon.ServiceError)(nil)).Once()

	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything, mock.Anything).Return(&model.TokenDTO{
		Token:     "new.access.token",
		IssuedAt:  now,
		ExpiresIn: 7200,
		Scopes:    []string{"read"},
	}, nil)

	extendErr := &tidcommon.ServiceError{
		Type: tidcommon.ServerErrorType,
		Code: "ACS-2001",
		Error: tidcommon.I18nMessage{
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
	suite.testCfg.OAuth.RefreshToken.RenewOnGrant = true
	suite.rebuildHandlerWithConfig()

	suite.mockTokenValidator.
		On("ValidateRefreshToken", mock.Anything, suite.validRefreshToken, testRefreshTokenClientID).
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
			(*tidcommon.ServiceError)(nil)).Once()

	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything, mock.Anything).Return(&model.TokenDTO{
		Token:          "new.access.token",
		IssuedAt:       time.Now().Unix(),
		ExpiresIn:      3600,
		Scopes:         []string{"read"},
		UserAttributes: map[string]interface{}{"aci": testCacheID},
	}, nil)

	suite.mockTokenBuilder.On("BuildRefreshToken", mock.Anything, mock.Anything).Return(&model.TokenDTO{
		Token:     "new.refresh.token",
		IssuedAt:  time.Now().Unix(),
		ExpiresIn: 86400,
		Scopes:    []string{"read"},
	}, nil)

	// Expect TTL to be extended to the refresh token validity period (86400 from config) + buffer(60) = 86460.
	suite.mockAttrCacheService.On("ExtendAttributeCacheTTL", mock.Anything, testCacheID, 86460).
		Return((*tidcommon.ServiceError)(nil))

	response, err := suite.handler.HandleGrant(context.Background(), suite.testTokenReq, suite.oauthApp)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), response)
	assert.Equal(suite.T(), "new.access.token", response.AccessToken.Token)
	assert.Equal(suite.T(), "new.refresh.token", response.RefreshToken.Token)
	suite.mockAttrCacheService.AssertCalled(suite.T(), "ExtendAttributeCacheTTL", mock.Anything, testCacheID, 86460)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_RenewOnGrant_ExtendAttributeCacheTTLError() {
	suite.testCfg.OAuth.RefreshToken.RenewOnGrant = true
	suite.rebuildHandlerWithConfig()

	suite.mockTokenValidator.
		On("ValidateRefreshToken", mock.Anything, suite.validRefreshToken, testRefreshTokenClientID).
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
			(*tidcommon.ServiceError)(nil)).Once()

	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything, mock.Anything).Return(&model.TokenDTO{
		Token:          "new.access.token",
		IssuedAt:       time.Now().Unix(),
		ExpiresIn:      3600,
		Scopes:         []string{"read"},
		UserAttributes: map[string]interface{}{"aci": testCacheID},
	}, nil)

	suite.mockTokenBuilder.On("BuildRefreshToken", mock.Anything, mock.Anything).Return(&model.TokenDTO{
		Token:     "new.refresh.token",
		IssuedAt:  time.Now().Unix(),
		ExpiresIn: 86400,
		Scopes:    []string{"read"},
	}, nil)

	extendErr := &tidcommon.ServiceError{
		Type: tidcommon.ServerErrorType,
		Code: "ACS-2001",
		Error: tidcommon.I18nMessage{
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

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_ExtendsCache_EvenWhenCurrentTTLAlreadySufficient() {
	// extendCacheTTL does not currently inspect cacheEntry.TTLSeconds, so ExtendAttributeCacheTTL
	// is called unconditionally regardless of the current TTL (100000).

	suite.mockTokenValidator.
		On("ValidateRefreshToken", mock.Anything, suite.validRefreshToken, testRefreshTokenClientID).
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
		}, (*tidcommon.ServiceError)(nil)).Once()

	suite.mockAttrCacheService.On("ExtendAttributeCacheTTL", mock.Anything, testCacheID, mock.Anything).
		Return((*tidcommon.ServiceError)(nil)).Once()

	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything, mock.Anything).Return(&model.TokenDTO{
		Token:     "new.access.token",
		IssuedAt:  time.Now().Unix(),
		ExpiresIn: 3600,
		Scopes:    []string{"read"},
	}, nil)

	response, err := suite.handler.HandleGrant(context.Background(), suite.testTokenReq, suite.oauthApp)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), response)
	suite.mockAttrCacheService.AssertCalled(suite.T(), "ExtendAttributeCacheTTL",
		mock.Anything, testCacheID, mock.Anything)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestExtendCacheTTL_NilCacheEntry_NoOp() {
	result := suite.handler.extendCacheTTL(
		context.Background(), nil, suite.oauthApp,
		time.Now().Unix()-3600, 3600, false, testCacheID, log.GetLogger(),
	)

	assert.Nil(suite.T(), result)
	suite.mockAttrCacheService.AssertNotCalled(suite.T(), "ExtendAttributeCacheTTL")
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestExtendCacheTTL_ExtendsRegardlessOfCurrentTTL() {
	// extendCacheTTL does not currently inspect cacheEntry.TTLSeconds (200000), so the extend
	// call is always made.
	cacheEntry := &attributecache.AttributeCache{ID: testCacheID, TTLSeconds: 200000}

	suite.mockAttrCacheService.On("ExtendAttributeCacheTTL", mock.Anything, testCacheID, mock.Anything).
		Return((*tidcommon.ServiceError)(nil)).Once()

	result := suite.handler.extendCacheTTL(
		context.Background(), cacheEntry, suite.oauthApp,
		time.Now().Unix()-3600, 3600, false, testCacheID, log.GetLogger(),
	)

	assert.Nil(suite.T(), result)
	suite.mockAttrCacheService.AssertCalled(suite.T(), "ExtendAttributeCacheTTL",
		mock.Anything, testCacheID, mock.Anything)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestExtendCacheTTL_RefreshOutlivesAccess_ExtendsToRefreshExpiry() {
	// iat=now-3600, validity=86400 → remaining≈82800. accessExpiresIn=3600 < 82800.
	// desiredTTL ≈ 82800 + 60 = 82860 (±1 for clock drift).
	cacheEntry := &attributecache.AttributeCache{ID: testCacheID, TTLSeconds: 0}

	suite.mockAttrCacheService.On("ExtendAttributeCacheTTL", mock.Anything, testCacheID,
		mock.MatchedBy(func(ttl int) bool { return ttl >= 82858 && ttl <= 82862 })).
		Return((*tidcommon.ServiceError)(nil))

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
		Return((*tidcommon.ServiceError)(nil))

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
		Return((*tidcommon.ServiceError)(nil))

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

	extendErr := &tidcommon.ServiceError{
		Type: tidcommon.ServerErrorType,
		Code: "ACS-2001",
		Error: tidcommon.I18nMessage{
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
	suite.testCfg.OAuth.RefreshToken.RenewOnGrant = true
	suite.rebuildHandlerWithConfig()

	// Mock successful refresh token validation with openid scope
	suite.mockTokenValidator.
		On("ValidateRefreshToken", mock.Anything, suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:              testRefreshTokenUserID,
			Audiences:        []string{testRefreshTokenAudience},
			Scopes:           []string{"openid", "read"},
			GrantType:        "authorization_code",
			AttributeCacheID: "",
			Iat:              int64(suite.validClaims["iat"].(float64)),
		}, nil)

	// Mock successful access token generation
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything, mock.Anything).Return(&model.TokenDTO{
		Token:          "new.access.token",
		IssuedAt:       time.Now().Unix(),
		ExpiresIn:      3600,
		Scopes:         []string{"openid", "read"},
		UserAttributes: map[string]interface{}{"email": "test@example.com"},
	}, nil)

	// Mock successful ID token generation
	suite.mockTokenBuilder.On("BuildIDToken", mock.Anything, mock.MatchedBy(
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
	suite.mockTokenBuilder.On("BuildRefreshToken", mock.Anything, mock.Anything).Return(&model.TokenDTO{
		Token:     "new.refresh.token",
		IssuedAt:  time.Now().Unix(),
		ExpiresIn: 86400,
		Scopes:    []string{"openid", "read"},
	}, nil)

	tokenReq := &model.TokenRequest{
		GrantType:    string(providers.GrantTypeRefreshToken),
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
		GrantType:    string(providers.GrantTypeRefreshToken),
		ClientID:     testRefreshTokenClientID,
		RefreshToken: suite.validRefreshToken,
		Resources:    []string{"not-an-absolute-uri"},
	}

	err := suite.handler.ValidateGrant(context.Background(), tokenReq, suite.oauthApp)

	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorInvalidTarget, err.Error)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_MatchingResource_ReusesBoundAudience() {
	// Refresh token is bound to a single audience (rs01); request resource=[rs01] matches → issued aud=[rs01].
	suite.mockTokenValidator.
		On("ValidateRefreshToken", mock.Anything, suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:              testRefreshTokenUserID,
			Audiences:        []string{testRS01URI},
			Scopes:           []string{"read"},
			GrantType:        "authorization_code",
			AttributeCacheID: "",
			Iat:              int64(suite.validClaims["iat"].(float64)),
		}, nil)

	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything, mock.MatchedBy(
		func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return len(ctx.Audiences) == 1 && ctx.Audiences[0] == testRS01URI
		})).Return(&model.TokenDTO{
		Token:     "new.access.token",
		IssuedAt:  time.Now().Unix(),
		ExpiresIn: 3600,
		Scopes:    []string{"read"},
	}, nil)

	tokenReq := &model.TokenRequest{
		GrantType:    string(providers.GrantTypeRefreshToken),
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

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_DifferentResource_InvalidTarget() {
	// Refresh token is bound to rs01; request resource=[rs02] does not match → invalid_target.
	suite.mockTokenValidator.
		On("ValidateRefreshToken", mock.Anything, suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:              testRefreshTokenUserID,
			Audiences:        []string{testRS01URI},
			Scopes:           []string{"read"},
			GrantType:        "authorization_code",
			AttributeCacheID: "",
			Iat:              int64(suite.validClaims["iat"].(float64)),
		}, nil)

	tokenReq := &model.TokenRequest{
		GrantType:    string(providers.GrantTypeRefreshToken),
		ClientID:     testRefreshTokenClientID,
		RefreshToken: suite.validRefreshToken,
		Scope:        "read",
		Resources:    []string{testRS02URI},
	}

	response, err := suite.handler.HandleGrant(context.Background(), tokenReq, suite.oauthApp)

	assert.Nil(suite.T(), response)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorInvalidTarget, err.Error)
	assert.Equal(suite.T(), "Requested resource does not match the refresh token audience", err.ErrorDescription)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_MultipleResources_InvalidTarget() {
	// More than one resource parameter is not supported → invalid_target.
	suite.mockTokenValidator.
		On("ValidateRefreshToken", mock.Anything, suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:              testRefreshTokenUserID,
			Audiences:        []string{testRS01URI},
			Scopes:           []string{"read"},
			GrantType:        "authorization_code",
			AttributeCacheID: "",
			Iat:              int64(suite.validClaims["iat"].(float64)),
		}, nil)

	tokenReq := &model.TokenRequest{
		GrantType:    string(providers.GrantTypeRefreshToken),
		ClientID:     testRefreshTokenClientID,
		RefreshToken: suite.validRefreshToken,
		Scope:        "read",
		Resources:    []string{testRS01URI, testRS02URI},
	}

	response, err := suite.handler.HandleGrant(context.Background(), tokenReq, suite.oauthApp)

	assert.Nil(suite.T(), response)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorInvalidTarget, err.Error)
	assert.Equal(suite.T(), "Only a single resource parameter is supported", err.ErrorDescription)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_NoResourceParam_ReusesBoundAudience() {
	// No resource param → issued aud equals the single bound audience (rs01).
	suite.mockTokenValidator.
		On("ValidateRefreshToken", mock.Anything, suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:              testRefreshTokenUserID,
			Audiences:        []string{testRS01URI},
			Scopes:           []string{"read"},
			GrantType:        "authorization_code",
			AttributeCacheID: "",
			Iat:              int64(suite.validClaims["iat"].(float64)),
		}, nil)

	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything, mock.MatchedBy(
		func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return len(ctx.Audiences) == 1 && ctx.Audiences[0] == testRS01URI
		})).Return(&model.TokenDTO{
		Token:     "new.access.token",
		IssuedAt:  time.Now().Unix(),
		ExpiresIn: 3600,
		Scopes:    []string{"read"},
	}, nil)

	tokenReq := &model.TokenRequest{
		GrantType:    string(providers.GrantTypeRefreshToken),
		ClientID:     testRefreshTokenClientID,
		RefreshToken: suite.validRefreshToken,
		Scope:        "read",
	}

	response, err := suite.handler.HandleGrant(context.Background(), tokenReq, suite.oauthApp)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), response)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_NonSingleAudience_InvalidGrant() {
	// A refresh token that is not bound to exactly one audience is rejected as invalid_grant.
	suite.mockTokenValidator.
		On("ValidateRefreshToken", mock.Anything, suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:              testRefreshTokenUserID,
			Audiences:        []string{testRS01URI, testRS02URI},
			Scopes:           []string{"read"},
			GrantType:        "authorization_code",
			AttributeCacheID: "",
			Iat:              int64(suite.validClaims["iat"].(float64)),
		}, nil)

	response, err := suite.handler.HandleGrant(context.Background(), suite.testTokenReq, suite.oauthApp)

	assert.Nil(suite.T(), response)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorInvalidGrant, err.Error)
	assert.Equal(suite.T(), "Refresh token is not bound to a single resource server", err.ErrorDescription)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_BoundResourceServerGone_InvalidTarget() {
	// The resource server bound to the refresh token no longer exists → invalid_target.
	suite.mockResourceService.ExpectedCalls = nil
	suite.mockResourceService.On("GetResourceServerByIdentifier", mock.Anything, testRS01URI).
		Return((*providers.ResourceServer)(nil), &tidcommon.ServiceError{
			Type: tidcommon.ClientErrorType,
			Code: "RS-1001",
			Error: tidcommon.I18nMessage{
				Key:          "error.resource.not_found",
				DefaultValue: "Resource server not found",
			},
		})

	suite.mockTokenValidator.
		On("ValidateRefreshToken", mock.Anything, suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:              testRefreshTokenUserID,
			Audiences:        []string{testRS01URI},
			Scopes:           []string{"read"},
			GrantType:        "authorization_code",
			AttributeCacheID: "",
			Iat:              int64(suite.validClaims["iat"].(float64)),
		}, nil)

	response, err := suite.handler.HandleGrant(context.Background(), suite.testTokenReq, suite.oauthApp)

	assert.Nil(suite.T(), response)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorInvalidTarget, err.Error)
	assert.Equal(suite.T(), "The resource server bound to the refresh token no longer exists", err.ErrorDescription)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_RenewOnGrant_PreservesSingleAudience() {
	// When renewRefreshToken=true, the new refresh token must carry the SAME single bound audience.
	suite.testCfg.OAuth.RefreshToken.RenewOnGrant = true
	suite.rebuildHandlerWithConfig()

	suite.mockTokenValidator.
		On("ValidateRefreshToken", mock.Anything, suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:              testRefreshTokenUserID,
			Audiences:        []string{testRS01URI},
			Scopes:           []string{"read"},
			GrantType:        "authorization_code",
			AttributeCacheID: "",
			Iat:              int64(suite.validClaims["iat"].(float64)),
		}, nil)

	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything, mock.MatchedBy(
		func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return len(ctx.Audiences) == 1 && ctx.Audiences[0] == testRS01URI
		})).Return(&model.TokenDTO{
		Token:     "new.access.token",
		IssuedAt:  time.Now().Unix(),
		ExpiresIn: 3600,
		Scopes:    []string{"read"},
	}, nil)

	// The new refresh token carries the same single audience [rs01].
	suite.mockTokenBuilder.On("BuildRefreshToken", mock.Anything, mock.MatchedBy(
		func(ctx *tokenservice.RefreshTokenBuildContext) bool {
			return len(ctx.AccessTokenAudiences) == 1 && ctx.AccessTokenAudiences[0] == testRS01URI
		})).Return(&model.TokenDTO{
		Token:     "new.refresh.token",
		IssuedAt:  time.Now().Unix(),
		ExpiresIn: 86400,
		Scopes:    []string{"read"},
	}, nil)

	tokenReq := &model.TokenRequest{
		GrantType:    string(providers.GrantTypeRefreshToken),
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

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_ScopeDownscopedToBoundResourceServer() {
	// Refresh token bound to rs01 with scopes [read write]. ValidatePermissions reports "write"
	// as invalid for rs01, so the non-OIDC scope is downscoped and the access token carries only "read".
	suite.mockResourceService.ExpectedCalls = nil
	suite.mockResourceService.On("GetResourceServerByIdentifier", mock.Anything, testRS01URI).
		Return(&providers.ResourceServer{ID: "rs-1", Identifier: testRS01URI}, nil)
	suite.mockResourceService.On("ValidatePermissions", mock.Anything, "rs-1", mock.Anything).
		Return([]string{"write"}, nil)

	suite.mockTokenValidator.
		On("ValidateRefreshToken", mock.Anything, suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:              testRefreshTokenUserID,
			Audiences:        []string{testRS01URI},
			Scopes:           []string{"read", "write"},
			GrantType:        "authorization_code",
			AttributeCacheID: "",
			Iat:              int64(suite.validClaims["iat"].(float64)),
		}, nil)

	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything, mock.MatchedBy(
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
		GrantType:    string(providers.GrantTypeRefreshToken),
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

const testRefreshTokenJkt = "0ZcOCORZNYy-DWpqq30jZyJGHTN0d2HglBV3uiguA4I" // #nosec G101

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_DPoPBoundRT_MissingProof_Rejected() {
	suite.mockTokenValidator.
		On("ValidateRefreshToken", mock.Anything, suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:       testRefreshTokenUserID,
			Audiences: []string{testRefreshTokenAudience},
			Scopes:    []string{"read"},
			GrantType: "authorization_code",
			Iat:       int64(suite.validClaims["iat"].(float64)),
			DPoPJkt:   testRefreshTokenJkt,
		}, nil)

	response, err := suite.handler.HandleGrant(context.Background(), suite.testTokenReq, suite.oauthApp)

	assert.Nil(suite.T(), response)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorInvalidGrant, err.Error)
	assert.Equal(suite.T(), "DPoP proof required for this refresh token", err.ErrorDescription)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_DPoPBoundRT_WrongKey_Rejected() {
	suite.mockTokenValidator.
		On("ValidateRefreshToken", mock.Anything, suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:       testRefreshTokenUserID,
			Audiences: []string{testRefreshTokenAudience},
			Scopes:    []string{"read"},
			GrantType: "authorization_code",
			Iat:       int64(suite.validClaims["iat"].(float64)),
			DPoPJkt:   testRefreshTokenJkt,
		}, nil)

	ctx := dpop.WithJkt(context.Background(), "different-jkt")
	response, err := suite.handler.HandleGrant(ctx, suite.testTokenReq, suite.oauthApp)

	assert.Nil(suite.T(), response)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorInvalidGrant, err.Error)
	assert.Equal(suite.T(), "DPoP proof key does not match refresh token binding", err.ErrorDescription)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_DPoPBoundRT_ValidProof_AccessTokenBound() {
	suite.mockTokenValidator.
		On("ValidateRefreshToken", mock.Anything, suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:       testRefreshTokenUserID,
			Audiences: []string{testRefreshTokenAudience},
			Scopes:    []string{"read", "write"},
			GrantType: "authorization_code",
			Iat:       int64(suite.validClaims["iat"].(float64)),
			DPoPJkt:   testRefreshTokenJkt,
		}, nil)

	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything, mock.MatchedBy(
		func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return ctx.DPoPJkt == testRefreshTokenJkt
		})).Return(&model.TokenDTO{
		Token:     "new.access.token",
		IssuedAt:  time.Now().Unix(),
		ExpiresIn: 3600,
		Scopes:    []string{"read"},
		TokenType: constants.TokenTypeDPoP,
	}, nil)

	ctx := dpop.WithJkt(context.Background(), testRefreshTokenJkt)
	response, err := suite.handler.HandleGrant(ctx, suite.testTokenReq, suite.oauthApp)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), response)
	assert.Equal(suite.T(), constants.TokenTypeDPoP, response.AccessToken.TokenType)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_UnboundRT_NoProof_Succeeds() {
	suite.mockTokenValidator.
		On("ValidateRefreshToken", mock.Anything, suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:       testRefreshTokenUserID,
			Audiences: []string{testRefreshTokenAudience},
			Scopes:    []string{"read"},
			GrantType: "authorization_code",
			Iat:       int64(suite.validClaims["iat"].(float64)),
		}, nil)

	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything, mock.MatchedBy(
		func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return ctx.DPoPJkt == ""
		})).Return(&model.TokenDTO{
		Token:     "new.access.token",
		IssuedAt:  time.Now().Unix(),
		ExpiresIn: 3600,
		Scopes:    []string{"read"},
	}, nil)

	response, err := suite.handler.HandleGrant(context.Background(), suite.testTokenReq, suite.oauthApp)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), response)
	assert.Empty(suite.T(), response.AccessToken.TokenType)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_UnboundRT_VoluntaryProof_AccessTokenBound() {
	suite.mockTokenValidator.
		On("ValidateRefreshToken", mock.Anything, suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:       testRefreshTokenUserID,
			Audiences: []string{testRefreshTokenAudience},
			Scopes:    []string{"read"},
			GrantType: "authorization_code",
			Iat:       int64(suite.validClaims["iat"].(float64)),
		}, nil)

	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything, mock.MatchedBy(
		func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return ctx.DPoPJkt == testRefreshTokenJkt
		})).Return(&model.TokenDTO{
		Token:     "new.access.token",
		IssuedAt:  time.Now().Unix(),
		ExpiresIn: 3600,
		Scopes:    []string{"read"},
		TokenType: constants.TokenTypeDPoP,
	}, nil)

	ctx := dpop.WithJkt(context.Background(), testRefreshTokenJkt)
	response, err := suite.handler.HandleGrant(ctx, suite.testTokenReq, suite.oauthApp)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), response)
	assert.Equal(suite.T(), constants.TokenTypeDPoP, response.AccessToken.TokenType)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_DPoPBoundRT_RenewOnGrant_RotatesJkt_PublicClient() {
	suite.testCfg.OAuth.RefreshToken.RenewOnGrant = true
	suite.rebuildHandlerWithConfig()

	suite.oauthApp.PublicClient = true
	suite.mockTokenValidator.
		On("ValidateRefreshToken", mock.Anything, suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:       testRefreshTokenUserID,
			Audiences: []string{testRefreshTokenAudience},
			Scopes:    []string{"read"},
			GrantType: "authorization_code",
			Iat:       int64(suite.validClaims["iat"].(float64)),
			DPoPJkt:   testRefreshTokenJkt,
		}, nil)

	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything, mock.Anything).Return(&model.TokenDTO{
		Token:     "new.access.token",
		IssuedAt:  time.Now().Unix(),
		ExpiresIn: 3600,
		Scopes:    []string{"read"},
		TokenType: constants.TokenTypeDPoP,
	}, nil)

	suite.mockTokenBuilder.On("BuildRefreshToken", mock.Anything, mock.MatchedBy(
		func(ctx *tokenservice.RefreshTokenBuildContext) bool {
			return ctx.DPoPJkt == testRefreshTokenJkt
		})).Return(&model.TokenDTO{
		Token:    "new.refresh.token",
		IssuedAt: time.Now().Unix(),
	}, nil)

	ctx := dpop.WithJkt(context.Background(), testRefreshTokenJkt)
	response, err := suite.handler.HandleGrant(ctx, suite.testTokenReq, suite.oauthApp)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), response)
	assert.Equal(suite.T(), "new.refresh.token", response.RefreshToken.Token)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_RenewOnGrant_ConfidentialClient_RTNotBound() {
	// Confidential clients never receive a bound refresh token, even when a DPoP
	// proof is presented at /token.
	suite.testCfg.OAuth.RefreshToken.RenewOnGrant = true
	suite.rebuildHandlerWithConfig()

	suite.oauthApp.PublicClient = false
	suite.mockTokenValidator.
		On("ValidateRefreshToken", mock.Anything, suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:       testRefreshTokenUserID,
			Audiences: []string{testRefreshTokenAudience},
			Scopes:    []string{"read"},
			GrantType: "authorization_code",
			Iat:       int64(suite.validClaims["iat"].(float64)),
		}, nil)

	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything, mock.Anything).Return(&model.TokenDTO{
		Token:     "new.access.token",
		IssuedAt:  time.Now().Unix(),
		ExpiresIn: 3600,
		Scopes:    []string{"read"},
	}, nil)

	suite.mockTokenBuilder.On("BuildRefreshToken", mock.Anything, mock.MatchedBy(
		func(ctx *tokenservice.RefreshTokenBuildContext) bool {
			return ctx.DPoPJkt == ""
		})).Return(&model.TokenDTO{
		Token:    "new.refresh.token",
		IssuedAt: time.Now().Unix(),
	}, nil)

	ctx := dpop.WithJkt(context.Background(), testRefreshTokenJkt)
	response, err := suite.handler.HandleGrant(ctx, suite.testTokenReq, suite.oauthApp)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), response)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestIssueRefreshToken_PublicClient_BindsJkt() {
	suite.oauthApp.PublicClient = true

	suite.mockTokenBuilder.On("BuildRefreshToken", mock.Anything, mock.MatchedBy(
		func(ctx *tokenservice.RefreshTokenBuildContext) bool {
			return ctx.DPoPJkt == testRefreshTokenJkt
		})).Return(&model.TokenDTO{
		Token:    "rotated.refresh.token",
		IssuedAt: time.Now().Unix(),
	}, nil)

	ctx := dpop.WithJkt(context.Background(), testRefreshTokenJkt)
	tokenResponse := &model.TokenResponseDTO{}

	err := suite.handler.IssueRefreshToken(ctx, tokenResponse, suite.oauthApp,
		testRefreshTokenUserID, []string{testRefreshTokenAudience},
		"authorization_code", []string{"read"}, nil, "", "")

	assert.Nil(suite.T(), err)
	suite.mockTokenBuilder.AssertExpectations(suite.T())
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestIssueRefreshToken_ConfidentialClient_DoesNotBindJkt() {
	suite.oauthApp.PublicClient = false

	suite.mockTokenBuilder.On("BuildRefreshToken", mock.Anything, mock.MatchedBy(
		func(ctx *tokenservice.RefreshTokenBuildContext) bool {
			return ctx.DPoPJkt == ""
		})).Return(&model.TokenDTO{
		Token:    "rotated.refresh.token",
		IssuedAt: time.Now().Unix(),
	}, nil)

	ctx := dpop.WithJkt(context.Background(), testRefreshTokenJkt)
	tokenResponse := &model.TokenResponseDTO{}

	err := suite.handler.IssueRefreshToken(ctx, tokenResponse, suite.oauthApp,
		testRefreshTokenUserID, []string{testRefreshTokenAudience},
		"authorization_code", []string{"read"}, nil, "", "")

	assert.Nil(suite.T(), err)
	suite.mockTokenBuilder.AssertExpectations(suite.T())
}

// Resource-server binding error paths on refresh.

func (suite *RefreshTokenGrantHandlerTestSuite) refreshClaimsValid() {
	suite.mockTokenValidator.
		On("ValidateRefreshToken", mock.Anything, suite.validRefreshToken, testRefreshTokenClientID).
		Return(&tokenservice.RefreshTokenClaims{
			Sub:       testRefreshTokenUserID,
			Audiences: []string{testRefreshTokenAudience},
			Scopes:    []string{"read", "write"},
			GrantType: "authorization_code",
			Iat:       int64(suite.validClaims["iat"].(float64)),
		}, nil)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_BoundResourceServerLookupServerError() {
	suite.refreshClaimsValid()

	rsvc := resourcemock.NewResourceServiceInterfaceMock(suite.T())
	rsvc.On("GetResourceServerByIdentifier", mock.Anything, testRefreshTokenAudience).
		Return((*providers.ResourceServer)(nil), &tidcommon.ServiceError{
			Type: tidcommon.ServerErrorType,
			Code: "RES-5000",
		})
	suite.mockResourceService = rsvc
	suite.rebuildHandlerWithConfig()

	response, err := suite.handler.HandleGrant(context.Background(), suite.testTokenReq, suite.oauthApp)

	assert.Nil(suite.T(), response)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorServerError, err.Error)
}

func (suite *RefreshTokenGrantHandlerTestSuite) TestHandleGrant_DownscopeValidationError() {
	suite.refreshClaimsValid()

	rsvc := resourcemock.NewResourceServiceInterfaceMock(suite.T())
	rsvc.On("GetResourceServerByIdentifier", mock.Anything, testRefreshTokenAudience).
		Return(&providers.ResourceServer{ID: testRefreshTokenAudience, Identifier: testRefreshTokenAudience}, nil)
	rsvc.On("ValidatePermissions", mock.Anything, mock.Anything, mock.Anything).
		Return([]string(nil), &tidcommon.ServiceError{Type: tidcommon.ServerErrorType, Code: "RES-5001"})
	suite.mockResourceService = rsvc
	suite.rebuildHandlerWithConfig()

	response, err := suite.handler.HandleGrant(context.Background(), suite.testTokenReq, suite.oauthApp)

	assert.Nil(suite.T(), response)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorServerError, err.Error)
}
