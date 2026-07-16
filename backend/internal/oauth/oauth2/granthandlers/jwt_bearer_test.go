/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/dpop"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/tokenservicemock"
	"github.com/thunder-id/thunderid/tests/mocks/resourcemock"
)

const testAssertion = "test-id-jag-assertion" //nolint:gosec // Test assertion, not a real credential

type JWTBearerGrantHandlerTestSuite struct {
	suite.Suite
	mockTokenBuilder    *tokenservicemock.TokenBuilderInterfaceMock
	mockTokenValidator  *tokenservicemock.TokenValidatorInterfaceMock
	mockResourceService *resourcemock.ResourceServiceInterfaceMock
	handler             *jwtBearerGrantHandler
	oauthApp            *providers.OAuthClient
}

func TestJWTBearerGrantHandlerSuite(t *testing.T) {
	suite.Run(t, new(JWTBearerGrantHandlerTestSuite))
}

func (suite *JWTBearerGrantHandlerTestSuite) SetupTest() {
	suite.mockTokenBuilder = tokenservicemock.NewTokenBuilderInterfaceMock(suite.T())
	suite.mockTokenValidator = tokenservicemock.NewTokenValidatorInterfaceMock(suite.T())
	suite.mockResourceService = resourcemock.NewResourceServiceInterfaceMock(suite.T())
	suite.mockResourceService.On("FindResourceServersByPermissions", mock.Anything, mock.Anything).
		Return([]providers.ResourceServer{}, nil).Maybe()
	suite.handler = &jwtBearerGrantHandler{
		tokenBuilder:    suite.mockTokenBuilder,
		tokenValidator:  suite.mockTokenValidator,
		resourceService: suite.mockResourceService,
	}

	suite.oauthApp = &providers.OAuthClient{
		ID:         "app123",
		ClientID:   testClientID,
		GrantTypes: []providers.GrantType{providers.GrantTypeJWTBearer},
		Scopes:     []string{"read", "write"},
		Token: &providers.OAuthTokenConfig{
			AccessToken: &providers.AccessTokenConfig{
				UserConfig: &providers.AccessTokenSubConfig{
					ValidityPeriod: 3600,
				},
			},
		},
	}
}

func (suite *JWTBearerGrantHandlerTestSuite) TestNewJWTBearerGrantHandler() {
	handler := newJWTBearerGrantHandler(suite.mockTokenBuilder, suite.mockTokenValidator,
		suite.mockResourceService)
	assert.NotNil(suite.T(), handler)
	assert.Implements(suite.T(), (*GrantHandlerInterface)(nil), handler)
}

func (suite *JWTBearerGrantHandlerTestSuite) TestValidateGrant_Success() {
	tokenRequest := &model.TokenRequest{
		GrantType: string(providers.GrantTypeJWTBearer),
		ClientID:  testClientID,
		Assertion: testAssertion,
	}

	result := suite.handler.ValidateGrant(context.Background(), tokenRequest, suite.oauthApp)
	assert.Nil(suite.T(), result)
}

func (suite *JWTBearerGrantHandlerTestSuite) TestValidateGrant_WrongGrantType() {
	tokenRequest := &model.TokenRequest{
		GrantType: string(providers.GrantTypeTokenExchange),
		ClientID:  testClientID,
		Assertion: testAssertion,
	}

	result := suite.handler.ValidateGrant(context.Background(), tokenRequest, suite.oauthApp)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), constants.ErrorUnsupportedGrantType, result.Error)
}

func (suite *JWTBearerGrantHandlerTestSuite) TestValidateGrant_MissingAssertion() {
	tokenRequest := &model.TokenRequest{
		GrantType: string(providers.GrantTypeJWTBearer),
		ClientID:  testClientID,
	}

	result := suite.handler.ValidateGrant(context.Background(), tokenRequest, suite.oauthApp)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, result.Error)
	assert.Contains(suite.T(), result.ErrorDescription, "Missing required parameter: assertion")
}

// A public (none-auth) client cannot present an ID-JAG because the grant's only anti-theft protection
// is the client_id binding, which a public client cannot safeguard.
func (suite *JWTBearerGrantHandlerTestSuite) TestValidateGrant_PublicClientRejected() {
	suite.oauthApp.TokenEndpointAuthMethod = providers.TokenEndpointAuthMethodNone
	tokenRequest := &model.TokenRequest{
		GrantType: string(providers.GrantTypeJWTBearer),
		ClientID:  testClientID,
		Assertion: testAssertion,
	}

	result := suite.handler.ValidateGrant(context.Background(), tokenRequest, suite.oauthApp)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), constants.ErrorInvalidClient, result.Error)
	assert.Contains(suite.T(), result.ErrorDescription, "confidential client")
}

func (suite *JWTBearerGrantHandlerTestSuite) TestHandleGrant_Success() {
	now := time.Now().Unix()
	tokenRequest := &model.TokenRequest{
		GrantType: string(providers.GrantTypeJWTBearer),
		ClientID:  testClientID,
		Assertion: testAssertion,
	}

	suite.mockTokenValidator.On("ValidateIDJAGAssertion", mock.Anything, testAssertion, testClientID).
		Return(&tokenservice.IDJAGAssertionClaims{
			Sub:    testUserID,
			Iss:    testCustomIssuer,
			Scopes: []string{"read", "write"},
		}, nil)
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return ctx.Subject == testUserID &&
				ctx.ClientID == testClientID &&
				ctx.GrantType == string(providers.GrantTypeJWTBearer) &&
				ctx.SourceIDP == testCustomIssuer &&
				len(ctx.Audiences) == 1 && ctx.Audiences[0] == testClientID &&
				tokenservice.JoinScopes(ctx.Scopes) == testScopeReadWrite &&
				ctx.DPoPJkt == ""
		})).Return(&model.TokenDTO{
		Token:     testTokenExchangeJWT,
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  now,
		ExpiresIn: 3600,
		Scopes:    []string{"read", "write"},
		ClientID:  testClientID,
		Subject:   testUserID,
	}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), testTokenExchangeJWT, result.AccessToken.Token)
	assert.Equal(suite.T(), testUserID, result.AccessToken.Subject)
	assert.Empty(suite.T(), result.RefreshToken.Token)
}

// The issued access token is DPoP-bound when the request carries a DPoP proof, like every other grant;
// only the inbound ID-JAG assertion itself is exempt from sender-constraining.
func (suite *JWTBearerGrantHandlerTestSuite) TestHandleGrant_DPoPProof_PropagatesJktToBuilder() {
	now := time.Now().Unix()
	tokenRequest := &model.TokenRequest{
		GrantType: string(providers.GrantTypeJWTBearer),
		ClientID:  testClientID,
		Assertion: testAssertion,
	}

	suite.mockTokenValidator.On("ValidateIDJAGAssertion", mock.Anything, testAssertion, testClientID).
		Return(&tokenservice.IDJAGAssertionClaims{
			Sub:    testUserID,
			Iss:    testCustomIssuer,
			Scopes: []string{"read", "write"},
		}, nil)
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return ctx.DPoPJkt == "thumbprint-jb"
		})).Return(&model.TokenDTO{
		Token:     testTokenExchangeJWT,
		TokenType: constants.TokenTypeDPoP,
		IssuedAt:  now,
		ExpiresIn: 3600,
		Scopes:    []string{"read", "write"},
		ClientID:  testClientID,
		Subject:   testUserID,
	}, nil)

	ctx := dpop.WithJkt(context.Background(), "thumbprint-jb")
	result, errResp := suite.handler.HandleGrant(ctx, tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), constants.TokenTypeDPoP, result.AccessToken.TokenType)
}

func (suite *JWTBearerGrantHandlerTestSuite) TestHandleGrant_InvalidAssertion() {
	tokenRequest := &model.TokenRequest{
		GrantType: string(providers.GrantTypeJWTBearer),
		ClientID:  testClientID,
		Assertion: testAssertion,
	}

	suite.mockTokenValidator.On("ValidateIDJAGAssertion", mock.Anything, testAssertion, testClientID).
		Return(nil, errors.New("assertion audience does not match server issuer"))

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorInvalidGrant, errResp.Error)
	assert.Equal(suite.T(), "Invalid assertion", errResp.ErrorDescription)
}

// Granted scopes are the intersection of the assertion scopes and the request scope parameter. The
// app's registered scopes are not consulted.
func (suite *JWTBearerGrantHandlerTestSuite) TestHandleGrant_ScopeIntersection() {
	now := time.Now().Unix()
	tokenRequest := &model.TokenRequest{
		GrantType: string(providers.GrantTypeJWTBearer),
		ClientID:  testClientID,
		Assertion: testAssertion,
		Scope:     "read admin",
	}

	// Assertion carries [read write], request narrows to [read admin]. Only "read" survives the
	// intersection; "write" was not requested, "admin" was not asserted.
	suite.mockTokenValidator.On("ValidateIDJAGAssertion", mock.Anything, testAssertion, testClientID).
		Return(&tokenservice.IDJAGAssertionClaims{
			Sub:    testUserID,
			Iss:    testCustomIssuer,
			Scopes: []string{"read", "write"},
		}, nil)
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return tokenservice.JoinScopes(ctx.Scopes) == testScopeRead
		})).Return(&model.TokenDTO{
		Token:     testTokenExchangeJWT,
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  now,
		ExpiresIn: 3600,
		Scopes:    []string{"read"},
		ClientID:  testClientID,
		Subject:   testUserID,
	}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), []string{"read"}, result.AccessToken.Scopes)
}

// Regression test: previously the granted scopes were intersected with oauthApp.Scopes, so an app with
// no registered scopes silently produced an empty-scope token. The app's registered scopes are no
// longer consulted, so the assertion's scopes are granted in full when no request scope narrows them.
func (suite *JWTBearerGrantHandlerTestSuite) TestHandleGrant_EmptyAppScopes_AssertionScopesGranted() {
	now := time.Now().Unix()
	suite.oauthApp.Scopes = []string{}
	tokenRequest := &model.TokenRequest{
		GrantType: string(providers.GrantTypeJWTBearer),
		ClientID:  testClientID,
		Assertion: testAssertion,
	}

	suite.mockTokenValidator.On("ValidateIDJAGAssertion", mock.Anything, testAssertion, testClientID).
		Return(&tokenservice.IDJAGAssertionClaims{
			Sub:    testUserID,
			Iss:    testCustomIssuer,
			Scopes: []string{"read", "write"},
		}, nil)
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return tokenservice.JoinScopes(ctx.Scopes) == testScopeReadWrite
		})).Return(&model.TokenDTO{
		Token:     testTokenExchangeJWT,
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  now,
		ExpiresIn: 3600,
		Scopes:    []string{"read", "write"},
		ClientID:  testClientID,
		Subject:   testUserID,
	}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), []string{"read", "write"}, result.AccessToken.Scopes)
}

// RFC 8707: when the assertion carries a resource claim, the resource AS resolves it to registered
// Resource Servers and uses their identifiers as the access token audience instead of the clientID
// fallback.
func (suite *JWTBearerGrantHandlerTestSuite) TestHandleGrant_AssertionResource_AudienceFromResolvedRS() {
	now := time.Now().Unix()
	tokenRequest := &model.TokenRequest{
		GrantType: string(providers.GrantTypeJWTBearer),
		ClientID:  testClientID,
		Assertion: testAssertion,
	}

	suite.mockTokenValidator.On("ValidateIDJAGAssertion", mock.Anything, testAssertion, testClientID).
		Return(&tokenservice.IDJAGAssertionClaims{
			Sub:       testUserID,
			Iss:       testCustomIssuer,
			Scopes:    []string{"read", "write"},
			Resources: []string{testRS01URI},
		}, nil)
	suite.mockResourceService.On("GetResourceServerByIdentifier", mock.Anything, testRS01URI).
		Return(&providers.ResourceServer{ID: testRS01URI, Identifier: testRS01URI}, nil)
	// RS defines only "read"; "write" is dropped by scope narrowing.
	suite.mockResourceService.On("ValidatePermissions", mock.Anything, testRS01URI, []string{"read", "write"}).
		Return([]string{"write"}, nil)
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return len(ctx.Audiences) == 1 && ctx.Audiences[0] == testRS01URI &&
				tokenservice.JoinScopes(ctx.Scopes) == testScopeRead
		})).Return(&model.TokenDTO{
		Token:     testTokenExchangeJWT,
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  now,
		ExpiresIn: 3600,
		Scopes:    []string{"read"},
		ClientID:  testClientID,
		Subject:   testUserID,
	}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), []string{"read"}, result.AccessToken.Scopes)
}

// RFC 8707: when the assertion carries no resource claim, behavior is unchanged — audience composed
// from the granted scopes via the client_credentials pattern.
func (suite *JWTBearerGrantHandlerTestSuite) TestHandleGrant_AssertionNoResource_CurrentBehaviorUnchanged() {
	now := time.Now().Unix()
	tokenRequest := &model.TokenRequest{
		GrantType: string(providers.GrantTypeJWTBearer),
		ClientID:  testClientID,
		Assertion: testAssertion,
	}

	suite.mockTokenValidator.On("ValidateIDJAGAssertion", mock.Anything, testAssertion, testClientID).
		Return(&tokenservice.IDJAGAssertionClaims{
			Sub:    testUserID,
			Iss:    testCustomIssuer,
			Scopes: []string{"read", "write"},
		}, nil)
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return len(ctx.Audiences) == 1 && ctx.Audiences[0] == testClientID &&
				tokenservice.JoinScopes(ctx.Scopes) == testScopeReadWrite
		})).Return(&model.TokenDTO{
		Token:     testTokenExchangeJWT,
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  now,
		ExpiresIn: 3600,
		Scopes:    []string{"read", "write"},
		ClientID:  testClientID,
		Subject:   testUserID,
	}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), []string{"read", "write"}, result.AccessToken.Scopes)
}

// RFC 8707: a request resource parameter that is a subset of the assertion's resources narrows the
// audience to the requested resource server only.
func (suite *JWTBearerGrantHandlerTestSuite) TestHandleGrant_RequestResourceNarrowsAssertionResource_Accepted() {
	now := time.Now().Unix()
	const testRS02URI = "https://rs02.example.com"
	tokenRequest := &model.TokenRequest{
		GrantType: string(providers.GrantTypeJWTBearer),
		ClientID:  testClientID,
		Assertion: testAssertion,
		Resources: []string{testRS01URI},
	}

	suite.mockTokenValidator.On("ValidateIDJAGAssertion", mock.Anything, testAssertion, testClientID).
		Return(&tokenservice.IDJAGAssertionClaims{
			Sub:       testUserID,
			Iss:       testCustomIssuer,
			Scopes:    []string{"read", "write"},
			Resources: []string{testRS01URI, testRS02URI},
		}, nil)
	suite.mockResourceService.On("GetResourceServerByIdentifier", mock.Anything, testRS01URI).
		Return(&providers.ResourceServer{ID: testRS01URI, Identifier: testRS01URI}, nil)
	suite.mockResourceService.On("ValidatePermissions", mock.Anything, testRS01URI, []string{"read", "write"}).
		Return([]string{}, nil)
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return len(ctx.Audiences) == 1 && ctx.Audiences[0] == testRS01URI
		})).Return(&model.TokenDTO{
		Token:     testTokenExchangeJWT,
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  now,
		ExpiresIn: 3600,
		Scopes:    []string{"read", "write"},
		ClientID:  testClientID,
		Subject:   testUserID,
	}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
}

// RFC 8707: a request resource parameter not present in the assertion's resources is rejected as
// invalid_target — the request can narrow the assertion's resources but not widen them.
func (suite *JWTBearerGrantHandlerTestSuite) TestHandleGrant_RequestResourceNotInAssertionResource_InvalidTarget() {
	tokenRequest := &model.TokenRequest{
		GrantType: string(providers.GrantTypeJWTBearer),
		ClientID:  testClientID,
		Assertion: testAssertion,
		Resources: []string{"https://not-granted.example.com"},
	}

	suite.mockTokenValidator.On("ValidateIDJAGAssertion", mock.Anything, testAssertion, testClientID).
		Return(&tokenservice.IDJAGAssertionClaims{
			Sub:       testUserID,
			Iss:       testCustomIssuer,
			Scopes:    []string{"read", "write"},
			Resources: []string{testRS01URI},
		}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorInvalidTarget, errResp.Error)
}

func (suite *JWTBearerGrantHandlerTestSuite) TestHandleGrant_TokenBuildError() {
	tokenRequest := &model.TokenRequest{
		GrantType: string(providers.GrantTypeJWTBearer),
		ClientID:  testClientID,
		Assertion: testAssertion,
	}

	suite.mockTokenValidator.On("ValidateIDJAGAssertion", mock.Anything, testAssertion, testClientID).
		Return(&tokenservice.IDJAGAssertionClaims{
			Sub:    testUserID,
			Iss:    testCustomIssuer,
			Scopes: []string{"read"},
		}, nil)
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything, mock.Anything).
		Return(nil, errors.New("token generation failed"))

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorServerError, errResp.Error)
}
