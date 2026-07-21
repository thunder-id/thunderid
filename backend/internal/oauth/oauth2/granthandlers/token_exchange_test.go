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
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/dpop"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/revocation"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/tests/mocks/actorprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/authzmock"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/tokenservicemock"
	"github.com/thunder-id/thunderid/tests/mocks/resourcemock"
	"github.com/thunder-id/thunderid/tests/mocks/serverconfigmock"
)

const (
	testTokenExchangeJWT = "test-token-exchange-jwt" //nolint:gosec
	testScopeReadWrite   = "read write"
	testCustomIssuer     = "https://custom.issuer.com"
	testUserEmail        = "user@example.com"
	testClientID         = "client123"
	testUserID           = "user123"
	testScopeRead        = "read"
	// testTokenExchangeDefaultRSID / testTokenExchangeDefaultRSAudience model the
	// deployment-configured default resource server used when a request carries no explicit
	// resource parameter.
	testTokenExchangeDefaultRSID       = "rs-1"
	testTokenExchangeDefaultRSAudience = "https://default-rs.example.com" // #nosec G101 -- resource identifier URL.
)

type TokenExchangeGrantHandlerTestSuite struct {
	suite.Suite
	mockJWTService      *jwtmock.JWTServiceInterfaceMock
	mockTokenBuilder    *tokenservicemock.TokenBuilderInterfaceMock
	mockTokenValidator  *tokenservicemock.TokenValidatorInterfaceMock
	mockAuthzService    *authzmock.AuthorizationProviderMock
	mockActorProvider   *actorprovidermock.ActorProviderMock
	mockResourceService *resourcemock.ResourceServiceInterfaceMock
	mockServerConfigSvc *serverconfigmock.ServerConfigServiceMock
	handler             *tokenExchangeGrantHandler
	oauthApp            *providers.OAuthClient
}

func TestTokenExchangeGrantHandlerSuite(t *testing.T) {
	suite.Run(t, new(TokenExchangeGrantHandlerTestSuite))
}

func (suite *TokenExchangeGrantHandlerTestSuite) SetupTest() {
	testConfig := &config.Config{
		JWT: engineconfig.JWTConfig{
			Issuer:         "https://auth.example.com",
			ValidityPeriod: 3600,
			Audience:       "application", // Default audience for tests
		},
	}
	err := config.InitializeServerRuntime("", testConfig)
	assert.NoError(suite.T(), err)

	suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())
	suite.mockTokenBuilder = tokenservicemock.NewTokenBuilderInterfaceMock(suite.T())
	suite.mockTokenValidator = tokenservicemock.NewTokenValidatorInterfaceMock(suite.T())
	suite.mockAuthzService = authzmock.NewAuthorizationProviderMock(suite.T())
	suite.mockActorProvider = actorprovidermock.NewActorProviderMock(suite.T())
	suite.mockResourceService = resourcemock.NewResourceServiceInterfaceMock(suite.T())
	suite.mockServerConfigSvc = serverconfigmock.NewServerConfigServiceMock(suite.T())
	suite.mockActorProvider.On("GetActorGroups", mock.Anything).
		Return([]providers.EntityGroup{}, nil).Maybe()
	suite.mockAuthzService.On("EvaluateAccessBatch", mock.Anything, mock.Anything).
		Return(func(_ context.Context,
			request providers.AccessEvaluationsRequest,
		) *providers.AccessEvaluationsResponse {
			evaluations := make([]providers.AccessEvaluationResponse, 0, len(request.Evaluations))
			for range request.Evaluations {
				evaluations = append(evaluations, providers.AccessEvaluationResponse{
					Decision: true,
				})
			}
			return &providers.AccessEvaluationsResponse{Evaluations: evaluations}
		}, nil).Maybe()
	// Explicit resource parameter resolves to an RS whose identifier equals the resource URI.
	suite.mockResourceService.On("GetResourceServerByIdentifier", mock.Anything, mock.Anything).
		Return(func(_ context.Context, identifier string) *providers.ResourceServer {
			return &providers.ResourceServer{ID: identifier, Identifier: identifier}
		}, func(_ context.Context, _ string) *tidcommon.ServiceError {
			return nil
		}).Maybe()
	suite.mockResourceService.On("ValidatePermissions", mock.Anything, mock.Anything, mock.Anything).
		Return([]string{}, nil).Maybe()
	// No explicit resource -> deployment default RS.
	suite.mockServerConfigSvc.On("GetMergedConfig", mock.Anything, "defaultResourceServer").
		Return(resource.DefaultResourceServerConfig{ResourceServerID: testTokenExchangeDefaultRSID}, nil).Maybe()
	suite.mockResourceService.On("GetResourceServer", mock.Anything, testTokenExchangeDefaultRSID).
		Return(&providers.ResourceServer{
			ID:         testTokenExchangeDefaultRSID,
			Identifier: testTokenExchangeDefaultRSAudience,
		}, nil).Maybe()
	suite.handler = &tokenExchangeGrantHandler{
		tokenBuilder:        suite.mockTokenBuilder,
		tokenValidator:      suite.mockTokenValidator,
		authzService:        suite.mockAuthzService,
		actorProvider:       suite.mockActorProvider,
		resourceService:     suite.mockResourceService,
		serverConfigService: suite.mockServerConfigSvc,
	}

	suite.oauthApp = &providers.OAuthClient{
		ID:                      "app123",
		ClientID:                testClientID,
		RedirectURIs:            []string{"https://example.com/callback"},
		GrantTypes:              []providers.GrantType{providers.GrantTypeTokenExchange},
		ResponseTypes:           []providers.ResponseType{providers.ResponseTypeCode},
		TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
		Token: &providers.OAuthTokenConfig{
			AccessToken: &providers.AccessTokenConfig{
				UserConfig: &providers.AccessTokenSubConfig{
					ValidityPeriod: 7200,
					Attributes:     []string{"email", "name", "given_name", "family_name", "organization"},
				},
			},
		},
	}
}

// getDefaultAudience is a helper function to get the configured default audience from runtime.
func (suite *TokenExchangeGrantHandlerTestSuite) getDefaultAudience() string {
	runtime := config.GetServerRuntime()
	if runtime == nil {
		suite.T().Skip("Server runtime not initialized")
		return ""
	}
	defaultAudience := runtime.Config.JWT.Audience
	if defaultAudience == "" {
		suite.T().Skip("Default audience not configured in runtime")
		return ""
	}
	return defaultAudience
}

// Helper function to create a test JWT token
func (suite *TokenExchangeGrantHandlerTestSuite) createTestJWT(claims map[string]interface{}) string {
	header := map[string]interface{}{
		"alg": "RS256",
		"typ": "JWT",
	}

	headerJSON, _ := json.Marshal(header)
	claimsJSON, _ := json.Marshal(claims)

	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)

	return fmt.Sprintf("%s.%s.signature", headerB64, claimsB64)
}

// Helper function to create a basic token request for testing
func (suite *TokenExchangeGrantHandlerTestSuite) createBasicTokenRequest(subjectToken string) *model.TokenRequest {
	return &model.TokenRequest{
		GrantType:        string(providers.GrantTypeTokenExchange),
		ClientID:         testClientID,
		SubjectToken:     subjectToken,
		SubjectTokenType: string(constants.TokenTypeIdentifierAccessToken),
	}
}

// Helper function to setup token validator and token builder mocks for successful token generation with audience check
func (suite *TokenExchangeGrantHandlerTestSuite) setupSuccessfulJWTMock(
	subjectToken string,
	expectedAudience string,
	now int64,
) {
	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub:            testUserID,
			Iss:            testCustomIssuer,
			Scopes:         []string{"read", "write"},
			UserAttributes: map[string]interface{}{},
			NestedAct:      nil,
		}, nil)
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			// Token is bound to exactly one resource server; audience is a single value.
			return ctx.Subject == testUserID &&
				len(ctx.Audiences) == 1 && ctx.Audiences[0] == expectedAudience
		})).Return(&model.TokenDTO{
		Token:     testTokenExchangeJWT,
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  now,
		ExpiresIn: 7200,
		Scopes:    []string{"read", "write"},
		ClientID:  testClientID,
	}, nil)
}

// Helper function to setup token validator and token builder mocks for successful token generation with scope check
func (suite *TokenExchangeGrantHandlerTestSuite) setupSuccessfulJWTMockWithScope(
	subjectToken string,
	expectedAudience string,
	expectedScope string,
	now int64,
) {
	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub:            testUserID,
			Iss:            testCustomIssuer,
			Scopes:         tokenservice.ParseScopes(expectedScope),
			UserAttributes: map[string]interface{}{},
			NestedAct:      nil,
		}, nil)
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return ctx.Subject == testUserID && (len(ctx.Audiences) > 0 && ctx.Audiences[0] == expectedAudience) &&
				tokenservice.JoinScopes(ctx.Scopes) == expectedScope
		})).Return(&model.TokenDTO{
		Token:     testTokenExchangeJWT,
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  now,
		ExpiresIn: 7200,
		Scopes:    tokenservice.ParseScopes(expectedScope),
		ClientID:  testClientID,
	}, nil)
}

// TestNewTokenExchangeGrantHandler tests the constructor
func (suite *TokenExchangeGrantHandlerTestSuite) TestNewTokenExchangeGrantHandler() {
	handler := newTokenExchangeGrantHandler(suite.mockTokenBuilder, suite.mockTokenValidator,
		suite.mockAuthzService, suite.mockActorProvider, suite.mockResourceService, suite.mockServerConfigSvc)
	assert.NotNil(suite.T(), handler)
	assert.Implements(suite.T(), (*GrantHandlerInterface)(nil), handler)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestValidateGrant_Success() {
	tokenRequest := &model.TokenRequest{
		GrantType:        string(providers.GrantTypeTokenExchange),
		ClientID:         testClientID,
		ClientSecret:     "secret123",
		SubjectToken:     "subject-token",
		SubjectTokenType: string(constants.TokenTypeIdentifierAccessToken),
	}

	result := suite.handler.ValidateGrant(context.Background(), tokenRequest, suite.oauthApp)
	assert.Nil(suite.T(), result)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestValidateGrant_WrongGrantType() {
	tokenRequest := &model.TokenRequest{
		GrantType:        "authorization_code",
		ClientID:         testClientID,
		SubjectToken:     "subject-token",
		SubjectTokenType: string(constants.TokenTypeIdentifierAccessToken),
	}

	result := suite.handler.ValidateGrant(context.Background(), tokenRequest, suite.oauthApp)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), constants.ErrorUnsupportedGrantType, result.Error)
	assert.Equal(suite.T(), "Unsupported grant type", result.ErrorDescription)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestValidateGrant_MissingSubjectToken() {
	tokenRequest := &model.TokenRequest{
		GrantType:        string(providers.GrantTypeTokenExchange),
		ClientID:         testClientID,
		SubjectToken:     "",
		SubjectTokenType: string(constants.TokenTypeIdentifierAccessToken),
	}

	result := suite.handler.ValidateGrant(context.Background(), tokenRequest, suite.oauthApp)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, result.Error)
	assert.Equal(suite.T(), "Missing required parameter: subject_token", result.ErrorDescription)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestValidateGrant_MissingSubjectTokenType() {
	tokenRequest := &model.TokenRequest{
		GrantType:        string(providers.GrantTypeTokenExchange),
		ClientID:         testClientID,
		SubjectToken:     "subject-token",
		SubjectTokenType: "",
	}

	result := suite.handler.ValidateGrant(context.Background(), tokenRequest, suite.oauthApp)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, result.Error)
	assert.Equal(suite.T(), "Missing required parameter: subject_token_type", result.ErrorDescription)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestValidateGrant_UnsupportedSubjectTokenType() {
	tokenRequest := &model.TokenRequest{
		GrantType:        string(providers.GrantTypeTokenExchange),
		ClientID:         testClientID,
		SubjectToken:     "subject-token",
		SubjectTokenType: "urn:ietf:params:oauth:token-type:saml2",
	}

	result := suite.handler.ValidateGrant(context.Background(), tokenRequest, suite.oauthApp)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, result.Error)
	assert.Contains(suite.T(), result.ErrorDescription, "Unsupported subject_token_type")
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestValidateGrant_MissingActorTokenType() {
	tokenRequest := &model.TokenRequest{
		GrantType:        string(providers.GrantTypeTokenExchange),
		ClientID:         testClientID,
		SubjectToken:     "subject-token",
		SubjectTokenType: string(constants.TokenTypeIdentifierAccessToken),
		ActorToken:       "actor-token",
		ActorTokenType:   "",
	}

	result := suite.handler.ValidateGrant(context.Background(), tokenRequest, suite.oauthApp)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, result.Error)
	assert.Equal(suite.T(), "actor_token_type is required when actor_token is provided", result.ErrorDescription)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestValidateGrant_UnsupportedActorTokenType() {
	tokenRequest := &model.TokenRequest{
		GrantType:        string(providers.GrantTypeTokenExchange),
		ClientID:         testClientID,
		SubjectToken:     "subject-token",
		SubjectTokenType: string(constants.TokenTypeIdentifierAccessToken),
		ActorToken:       "actor-token",
		ActorTokenType:   "urn:ietf:params:oauth:token-type:saml1",
	}

	result := suite.handler.ValidateGrant(context.Background(), tokenRequest, suite.oauthApp)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, result.Error)
	assert.Contains(suite.T(), result.ErrorDescription, "Unsupported actor_token_type")
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestValidateGrant_ActorTokenTypeWithoutActorToken() {
	tokenRequest := &model.TokenRequest{
		GrantType:        string(providers.GrantTypeTokenExchange),
		ClientID:         testClientID,
		SubjectToken:     "subject-token",
		SubjectTokenType: string(constants.TokenTypeIdentifierAccessToken),
		ActorToken:       "",
		ActorTokenType:   string(constants.TokenTypeIdentifierAccessToken),
	}

	result := suite.handler.ValidateGrant(context.Background(), tokenRequest, suite.oauthApp)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, result.Error)
	assert.Equal(suite.T(), "actor_token_type must not be provided without actor_token", result.ErrorDescription)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestValidateGrant_InvalidResourceURI() {
	tokenRequest := &model.TokenRequest{
		GrantType:        string(providers.GrantTypeTokenExchange),
		ClientID:         testClientID,
		SubjectToken:     "subject-token",
		SubjectTokenType: string(constants.TokenTypeIdentifierAccessToken),
		Resources:        []string{"not-a-valid-uri"},
	}

	result := suite.handler.ValidateGrant(context.Background(), tokenRequest, suite.oauthApp)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), constants.ErrorInvalidTarget, result.Error)
	assert.Contains(suite.T(), result.ErrorDescription, "Invalid resource parameter")
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestValidateGrant_ResourceURIWithFragment() {
	tokenRequest := &model.TokenRequest{
		GrantType:        string(providers.GrantTypeTokenExchange),
		ClientID:         testClientID,
		SubjectToken:     "subject-token",
		SubjectTokenType: string(constants.TokenTypeIdentifierAccessToken),
		Resources:        []string{"https://api.example.com/resource#fragment"},
	}

	result := suite.handler.ValidateGrant(context.Background(), tokenRequest, suite.oauthApp)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), constants.ErrorInvalidTarget, result.Error)
	assert.Contains(suite.T(), result.ErrorDescription, "must not contain a fragment component")
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestValidateGrant_ValidResourceURI() {
	tokenRequest := &model.TokenRequest{
		GrantType:        string(providers.GrantTypeTokenExchange),
		ClientID:         testClientID,
		SubjectToken:     "subject-token",
		SubjectTokenType: string(constants.TokenTypeIdentifierAccessToken),
		Resources:        []string{"https://api.example.com/resource"},
	}

	result := suite.handler.ValidateGrant(context.Background(), tokenRequest, suite.oauthApp)
	assert.Nil(suite.T(), result)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestValidateGrant_UnsupportedRequestedTokenType() {
	tokenRequest := &model.TokenRequest{
		GrantType:          string(providers.GrantTypeTokenExchange),
		ClientID:           testClientID,
		SubjectToken:       "subject-token",
		SubjectTokenType:   string(constants.TokenTypeIdentifierAccessToken),
		RequestedTokenType: "urn:ietf:params:oauth:token-type:saml2",
	}

	result := suite.handler.ValidateGrant(context.Background(), tokenRequest, suite.oauthApp)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, result.Error)
	assert.Contains(suite.T(), result.ErrorDescription, "Unsupported requested_token_type")
}

// ============================================================================
// HandleGrant Tests - Success Cases
// ============================================================================

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_Success_Basic() {
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub":   "user123",
		"iss":   testCustomIssuer,
		"aud":   "app123",
		"exp":   float64(now + 3600),
		"nbf":   float64(now - 60),
		"scope": "read write",
		"email": "user@example.com",
	})

	tokenRequest := &model.TokenRequest{
		GrantType:        string(providers.GrantTypeTokenExchange),
		ClientID:         testClientID,
		SubjectToken:     subjectToken,
		SubjectTokenType: string(constants.TokenTypeIdentifierAccessToken),
	}

	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub:            testUserID,
			Iss:            testCustomIssuer,
			Scopes:         []string{"read", "write"},
			UserAttributes: map[string]interface{}{"email": testUserEmail},
			NestedAct:      nil,
		}, nil)
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return ctx.Subject == testUserID &&
				// No resource parameter → token is bound to the configured default resource server.
				len(ctx.Audiences) == 1 && ctx.Audiences[0] == testTokenExchangeDefaultRSAudience &&
				ctx.ClientID == testClientID &&
				ctx.SubjectAttributes["email"] == testUserEmail &&
				tokenservice.JoinScopes(ctx.Scopes) == testScopeReadWrite
		})).Return(&model.TokenDTO{
		Token:     testTokenExchangeJWT,
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  now,
		ExpiresIn: 7200,
		Scopes:    []string{"read", "write"},
		ClientID:  testClientID,
		UserAttributes: map[string]interface{}{
			"email": "user@example.com",
		},
	}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), testTokenExchangeJWT, result.AccessToken.Token)
	assert.Equal(suite.T(), constants.TokenTypeBearer, result.AccessToken.TokenType)
	assert.Equal(suite.T(), int64(7200), result.AccessToken.ExpiresIn)
	assert.Equal(suite.T(), []string{"read", "write"}, result.AccessToken.Scopes)
}

// A revoked subject token is rejected (RFC 7009 deny-list enforcement).
func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_RevokedSubjectToken() {
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub":   "user123",
		"iss":   "https://auth.example.com",
		"exp":   float64(now + 3600),
		"nbf":   float64(now - 60),
		"scope": "read",
	})
	tokenRequest := suite.createBasicTokenRequest(subjectToken)

	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(nil, revocation.ErrTokenRevoked)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, errResp.Error)
	assert.Contains(suite.T(), errResp.ErrorDescription, "subject_token")
}

// A revoked actor token is rejected even when the subject token is valid — enforcement runs on
// both tokens in a delegation exchange.
func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_RevokedActorToken() {
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub": "user123", "iss": "https://auth.example.com",
		"exp": float64(now + 3600), "nbf": float64(now - 60), "scope": "read",
	})
	actorToken := suite.createTestJWT(map[string]interface{}{
		"sub": "svc123", "iss": "https://auth.example.com",
		"exp": float64(now + 3600), "nbf": float64(now - 60),
	})
	tokenRequest := suite.createBasicTokenRequest(subjectToken)
	tokenRequest.ActorToken = actorToken
	tokenRequest.ActorTokenType = string(constants.TokenTypeIdentifierAccessToken)

	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub: testUserID, Iss: "https://auth.example.com", Scopes: []string{"read"},
			JTI: "subject-jti-ok",
		}, nil)
	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, actorToken, suite.oauthApp).
		Return(nil, revocation.ErrTokenRevoked)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, errResp.Error)
	assert.Contains(suite.T(), errResp.ErrorDescription, "actor_token")
}

// When the deny list cannot be consulted for the subject token, the exchange fails closed with
// server_error rather than issuing a token whose revocation status is unknown.
func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_SubjectTokenEnforcementUnavailableFailsClosed() {
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub":   "user123",
		"iss":   "https://auth.example.com",
		"exp":   float64(now + 3600),
		"nbf":   float64(now - 60),
		"scope": "read",
	})
	tokenRequest := suite.createBasicTokenRequest(subjectToken)

	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(nil, revocation.ErrEnforcementUnavailable)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorServerError, errResp.Error)
}

// The actor token path also fails closed with server_error when the deny list is unavailable, even
// though the subject token validates.
func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_ActorTokenEnforcementUnavailableFailsClosed() {
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub": "user123", "iss": "https://auth.example.com",
		"exp": float64(now + 3600), "nbf": float64(now - 60), "scope": "read",
	})
	actorToken := suite.createTestJWT(map[string]interface{}{
		"sub": "svc123", "iss": "https://auth.example.com",
		"exp": float64(now + 3600), "nbf": float64(now - 60),
	})
	tokenRequest := suite.createBasicTokenRequest(subjectToken)
	tokenRequest.ActorToken = actorToken
	tokenRequest.ActorTokenType = string(constants.TokenTypeIdentifierAccessToken)

	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub: testUserID, Iss: "https://auth.example.com", Scopes: []string{"read"},
			JTI: "subject-jti-ok",
		}, nil)
	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, actorToken, suite.oauthApp).
		Return(nil, revocation.ErrEnforcementUnavailable)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorServerError, errResp.Error)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_Success_WithScopeDownscoping() {
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub":   "user123",
		"iss":   testCustomIssuer,
		"exp":   float64(now + 3600),
		"nbf":   float64(now - 60),
		"scope": "read write delete",
	})

	tokenRequest := suite.createBasicTokenRequest(subjectToken)
	tokenRequest.Scope = testScopeReadWrite

	suite.setupSuccessfulJWTMockWithScope(subjectToken, testTokenExchangeDefaultRSAudience, testScopeReadWrite, now)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), []string{"read", "write"}, result.AccessToken.Scopes)
}

// TestHandleGrant_PreservesOIDCScopes_DownscopesPermissions verifies token exchange keeps OIDC
// scopes (governed by the app's OIDC scope configuration) while downscoping non-OIDC permission
// scopes to the target resource server. Regression: a resource server that defines no OIDC scope
// must not cause openid/profile to be dropped from the exchanged token.
func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_PreservesOIDCScopes_DownscopesPermissions() {
	now := time.Now().Unix()
	resourceURI := "https://rs.example.com"
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub":   testUserID,
		"iss":   testCustomIssuer,
		"exp":   float64(now + 3600),
		"nbf":   float64(now - 60),
		"scope": "openid profile read write",
	})

	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub:            testUserID,
			Iss:            testCustomIssuer,
			Scopes:         []string{"openid", "profile", "read", "write"},
			UserAttributes: map[string]interface{}{},
		}, nil)

	// The target RS defines only "read" (no OIDC scopes, no "write"). DownscopeToResourceServer must
	// drop "write" and must never be asked to validate openid/profile.
	suite.mockResourceService.ExpectedCalls = nil
	suite.mockResourceService.On("GetResourceServerByIdentifier", mock.Anything, resourceURI).
		Return(&providers.ResourceServer{ID: resourceURI, Identifier: resourceURI}, (*tidcommon.ServiceError)(nil))
	rsPermissions := map[string]struct{}{"read": {}}
	suite.mockResourceService.On("ValidatePermissions", mock.Anything, resourceURI, mock.Anything).
		Return(func(_ context.Context, _ string, permissions []string) []string {
			invalid := []string{}
			for _, p := range permissions {
				if _, ok := rsPermissions[p]; !ok {
					invalid = append(invalid, p)
				}
			}
			return invalid
		}, func(_ context.Context, _ string, _ []string) *tidcommon.ServiceError { return nil })

	// OIDC scopes (openid, profile) are preserved; only "read" survives resource downscoping.
	const expectedScope = "openid profile read"
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return len(ctx.Audiences) == 1 && ctx.Audiences[0] == resourceURI &&
				tokenservice.JoinScopes(ctx.Scopes) == expectedScope
		})).Return(&model.TokenDTO{
		Token:     testTokenExchangeJWT,
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  now,
		ExpiresIn: 7200,
		Scopes:    tokenservice.ParseScopes(expectedScope),
		ClientID:  testClientID,
	}, nil)

	tokenRequest := suite.createBasicTokenRequest(subjectToken)
	tokenRequest.Resources = []string{resourceURI}

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), []string{"openid", "profile", "read"}, result.AccessToken.Scopes)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_Success_WithActorToken() {
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub": "user123",
		"iss": testCustomIssuer,
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
	})

	actorToken := suite.createTestJWT(map[string]interface{}{
		"sub": "service456",
		"iss": testCustomIssuer,
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
	})

	tokenRequest := &model.TokenRequest{
		GrantType:        string(providers.GrantTypeTokenExchange),
		ClientID:         testClientID,
		SubjectToken:     subjectToken,
		SubjectTokenType: string(constants.TokenTypeIdentifierAccessToken),
		ActorToken:       actorToken,
		ActorTokenType:   string(constants.TokenTypeIdentifierAccessToken),
	}

	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub:            testUserID,
			Iss:            testCustomIssuer,
			UserAttributes: map[string]interface{}{},
			NestedAct:      nil,
		}, nil)
	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, actorToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub:            "service456",
			Iss:            testCustomIssuer,
			UserAttributes: map[string]interface{}{},
			NestedAct:      nil,
		}, nil)
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return ctx.Subject == testUserID &&
				len(ctx.Audiences) == 1 && ctx.Audiences[0] == testClientID &&
				ctx.ActorClaims != nil &&
				ctx.ActorClaims.Sub == "service456" &&
				ctx.ActorClaims.Iss == testCustomIssuer
		})).Return(&model.TokenDTO{
		Token:     testTokenExchangeJWT,
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  now,
		ExpiresIn: 7200,
		Scopes:    []string{},
		ClientID:  testClientID,
	}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_Success_WithActorChaining() {
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub": "user123",
		"iss": testCustomIssuer,
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
		"act": map[string]interface{}{
			"sub": "service789",
			"iss": "https://existing-actor.com",
		},
	})

	actorToken := suite.createTestJWT(map[string]interface{}{
		"sub": "service456",
		"iss": testCustomIssuer,
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
	})

	tokenRequest := &model.TokenRequest{
		GrantType:        string(providers.GrantTypeTokenExchange),
		ClientID:         testClientID,
		SubjectToken:     subjectToken,
		SubjectTokenType: string(constants.TokenTypeIdentifierAccessToken),
		ActorToken:       actorToken,
		ActorTokenType:   string(constants.TokenTypeIdentifierAccessToken),
	}

	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub:            "user123",
			Iss:            testCustomIssuer,
			UserAttributes: map[string]interface{}{},
			NestedAct: map[string]interface{}{
				"sub": "service789",
				"iss": "https://existing-actor.com",
			},
		}, nil)
	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, actorToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub:            "service456",
			Iss:            testCustomIssuer,
			UserAttributes: map[string]interface{}{},
			NestedAct:      nil,
		}, nil)
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			if ctx.ActorClaims == nil {
				return false
			}
			// Check new actor (from actor token)
			return ctx.ActorClaims.Sub == "service456" && ctx.ActorClaims.Iss == testCustomIssuer
		})).Return(&model.TokenDTO{
		Token:     testTokenExchangeJWT,
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  now,
		ExpiresIn: 7200,
		Scopes:    []string{},
		ClientID:  testClientID,
	}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
}

// The RFC 8693 audience parameter is ignored: with no resource parameter, the token is bound to
// the configured default resource server, not the requested audience.
func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_AudienceParameterIgnored() {
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub": "user123",
		"iss": testCustomIssuer,
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
	})

	tokenRequest := suite.createBasicTokenRequest(subjectToken)
	tokenRequest.Audiences = []string{"https://api.example.com"}

	suite.setupSuccessfulJWTMock(subjectToken, testTokenExchangeDefaultRSAudience, now)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_Success_WithResourceParameter() {
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub": "user123",
		"iss": testCustomIssuer,
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
	})

	tokenRequest := suite.createBasicTokenRequest(subjectToken)
	tokenRequest.Resources = []string{"https://resource.example.com"}

	suite.setupSuccessfulJWTMock(subjectToken, "https://resource.example.com", now)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_Success_WithMultipleSpacesInScope() {
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub":   "user123",
		"iss":   testCustomIssuer,
		"exp":   float64(now + 3600),
		"nbf":   float64(now - 60),
		"scope": "read write",
	})

	tokenRequest := suite.createBasicTokenRequest(subjectToken)
	tokenRequest.Scope = "  read    write  "

	suite.setupSuccessfulJWTMockWithScope(subjectToken, testTokenExchangeDefaultRSAudience, testScopeReadWrite, now)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), []string{"read", "write"}, result.AccessToken.Scopes)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_Success_PreservesUserAttributes() {
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub":   "user123",
		"iss":   testCustomIssuer,
		"exp":   float64(now + 3600),
		"nbf":   float64(now - 60),
		"email": "user@example.com",
		"name":  "Test User",
		"roles": []string{"admin", "user"},
	})

	tokenRequest := &model.TokenRequest{
		GrantType:        string(providers.GrantTypeTokenExchange),
		ClientID:         testClientID,
		SubjectToken:     subjectToken,
		SubjectTokenType: string(constants.TokenTypeIdentifierAccessToken),
	}

	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub:    testUserID,
			Iss:    testCustomIssuer,
			Scopes: []string{},
			UserAttributes: map[string]interface{}{
				"email": testUserEmail,
				"name":  "Test User",
				"roles": []string{"admin", "user"},
			},
			NestedAct: nil,
		}, nil)
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return ctx.Subject == testUserID &&
				len(ctx.Audiences) == 1 && ctx.Audiences[0] == testClientID &&
				ctx.SubjectAttributes["email"] == "user@example.com" &&
				ctx.SubjectAttributes["name"] == "Test User"
		})).Return(&model.TokenDTO{
		Token:     testTokenExchangeJWT,
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  now,
		ExpiresIn: 7200,
		Scopes:    []string{},
		ClientID:  testClientID,
		UserAttributes: map[string]interface{}{
			"email": "user@example.com",
			"name":  "Test User",
		},
	}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), testUserEmail, result.AccessToken.UserAttributes["email"])
	assert.Equal(suite.T(), "Test User", result.AccessToken.UserAttributes["name"])
}

// ============================================================================
// HandleGrant Tests - Error Cases
// ============================================================================

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_InvalidSubjectToken_SignatureError() {
	now := time.Now().Unix()
	// Create a token that decodes successfully and has valid issuer, but invalid signature
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub": "user123",
		"iss": testCustomIssuer,
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
	})

	tokenRequest := &model.TokenRequest{
		GrantType:        string(providers.GrantTypeTokenExchange),
		ClientID:         testClientID,
		SubjectToken:     subjectToken,
		SubjectTokenType: string(constants.TokenTypeIdentifierAccessToken),
	}

	// Token will pass issuer validation but fail signature verification
	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(nil, errors.New("invalid subject token signature: invalid signature"))

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, errResp.Error)
	assert.Equal(suite.T(), "Invalid subject_token", errResp.ErrorDescription)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_InvalidSubjectToken_MissingSubClaim() {
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"iss": testCustomIssuer,
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
	})

	tokenRequest := &model.TokenRequest{
		GrantType:        string(providers.GrantTypeTokenExchange),
		ClientID:         testClientID,
		SubjectToken:     subjectToken,
		SubjectTokenType: string(constants.TokenTypeIdentifierAccessToken),
	}

	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(nil, errors.New("missing or invalid 'sub' claim"))

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, errResp.Error)
	assert.Equal(suite.T(), "Invalid subject_token", errResp.ErrorDescription)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_InvalidSubjectToken_DecodeError() {
	tokenRequest := &model.TokenRequest{
		GrantType:        string(providers.GrantTypeTokenExchange),
		ClientID:         testClientID,
		SubjectToken:     "invalid.jwt.format",
		SubjectTokenType: string(constants.TokenTypeIdentifierAccessToken),
	}

	// Mock token validator to return decode error
	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, "invalid.jwt.format", suite.oauthApp).
		Return(nil, errors.New("invalid token format"))

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, errResp.Error)
	assert.Equal(suite.T(), "Invalid subject_token", errResp.ErrorDescription)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_InvalidSubjectToken_Expired() {
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub": "user123",
		"iss": testCustomIssuer,
		"exp": float64(now - 3600),
		"nbf": float64(now - 7200),
	})

	tokenRequest := suite.createBasicTokenRequest(subjectToken)
	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(nil, errors.New("token has expired"))

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, errResp.Error)
	assert.Equal(suite.T(), "Invalid subject_token", errResp.ErrorDescription)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_InvalidSubjectToken_NotYetValid() {
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub": "user123",
		"iss": testCustomIssuer,
		"exp": float64(now + 3600),
		"nbf": float64(now + 1800),
	})

	tokenRequest := suite.createBasicTokenRequest(subjectToken)
	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(nil, errors.New("token not yet valid"))

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, errResp.Error)
	assert.Equal(suite.T(), "Invalid subject_token", errResp.ErrorDescription)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_InvalidActorToken() {
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub": "user123",
		"iss": testCustomIssuer,
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
	})

	// Create a valid JWT format actor token that passes issuer validation but fails signature verification
	actorToken := suite.createTestJWT(map[string]interface{}{
		"sub": "service456",
		"iss": testCustomIssuer,
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
	})

	tokenRequest := &model.TokenRequest{
		GrantType:        string(providers.GrantTypeTokenExchange),
		ClientID:         testClientID,
		SubjectToken:     subjectToken,
		SubjectTokenType: string(constants.TokenTypeIdentifierAccessToken),
		ActorToken:       actorToken,
		ActorTokenType:   string(constants.TokenTypeIdentifierAccessToken),
	}

	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub:            testUserID,
			Iss:            testCustomIssuer,
			UserAttributes: map[string]interface{}{},
			NestedAct:      nil,
		}, nil)
	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, actorToken, suite.oauthApp).
		Return(nil, errors.New("invalid subject token signature: invalid signature"))

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, errResp.Error)
	assert.Equal(suite.T(), "Invalid actor_token", errResp.ErrorDescription)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_InvalidScope() {
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub":   "user123",
		"iss":   testCustomIssuer,
		"exp":   float64(now + 3600),
		"nbf":   float64(now - 60),
		"scope": "read write",
	})

	tokenRequest := &model.TokenRequest{
		GrantType:        string(providers.GrantTypeTokenExchange),
		ClientID:         testClientID,
		SubjectToken:     subjectToken,
		SubjectTokenType: string(constants.TokenTypeIdentifierAccessToken),
		Scope:            "read write delete", // "delete" is not in subject token
	}

	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub:            testUserID,
			Iss:            testCustomIssuer,
			Scopes:         []string{"read", "write"},
			UserAttributes: map[string]interface{}{},
			NestedAct:      nil,
		}, nil)
	// Expect token generation with only valid scopes ("read write", filtering out "delete")
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			// Verify only valid scopes are included (filtering out "delete")
			return tokenservice.JoinScopes(ctx.Scopes) == testScopeReadWrite
		})).Return(&model.TokenDTO{
		Token:     testTokenExchangeJWT,
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  now,
		ExpiresIn: 7200,
		Scopes:    []string{"read", "write"},
		ClientID:  testClientID,
	}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	// Should succeed with only valid scopes filtered in
	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), []string{"read", "write"}, result.AccessToken.Scopes)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_ScopeEscalationPrevention() {
	now := time.Now().Unix()
	// Subject token has NO scopes
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub": "user123",
		"iss": testCustomIssuer,
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
	})

	// Request tries to add scopes
	tokenRequest := &model.TokenRequest{
		GrantType:        string(providers.GrantTypeTokenExchange),
		ClientID:         testClientID,
		SubjectToken:     subjectToken,
		SubjectTokenType: string(constants.TokenTypeIdentifierAccessToken),
		Scope:            "read write",
	}

	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub:            testUserID,
			Iss:            testCustomIssuer,
			Scopes:         []string{}, // No scopes in subject token
			UserAttributes: map[string]interface{}{},
			NestedAct:      nil,
		}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorInvalidScope, errResp.Error)
	assert.Contains(suite.T(), errResp.ErrorDescription, "Cannot request scopes when the subject token has no scopes")
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_JWTGenerationError() {
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub": "user123",
		"iss": testCustomIssuer,
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
	})

	tokenRequest := &model.TokenRequest{
		GrantType:        string(providers.GrantTypeTokenExchange),
		ClientID:         testClientID,
		SubjectToken:     subjectToken,
		SubjectTokenType: string(constants.TokenTypeIdentifierAccessToken),
	}

	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub:            testUserID,
			Iss:            testCustomIssuer,
			Scopes:         []string{},
			UserAttributes: map[string]interface{}{},
			NestedAct:      nil,
		}, nil)
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything, mock.Anything).
		Return(nil, errors.New("failed to sign token"))

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorServerError, errResp.Error)
	assert.Equal(suite.T(), "Failed to generate token", errResp.ErrorDescription)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_UsesDefaultConfig() {
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub": "user123",
		"iss": "https://auth.example.com", // Use default config issuer since oauthApp has no Token config
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
	})

	tokenRequest := &model.TokenRequest{
		GrantType:        string(providers.GrantTypeTokenExchange),
		ClientID:         testClientID,
		SubjectToken:     subjectToken,
		SubjectTokenType: string(constants.TokenTypeIdentifierAccessToken),
	}

	// Use app without custom token config
	oauthAppNoConfig := &providers.OAuthClient{
		ClientID:   testClientID,
		GrantTypes: []providers.GrantType{providers.GrantTypeTokenExchange},
	}

	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, oauthAppNoConfig).
		Return(&tokenservice.SubjectTokenClaims{
			Sub:            testUserID,
			Iss:            testCustomIssuer,
			Scopes:         []string{},
			UserAttributes: map[string]interface{}{},
			NestedAct:      nil,
		}, nil)
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything, mock.Anything).
		Return(&model.TokenDTO{
			Token:     testTokenExchangeJWT,
			TokenType: constants.TokenTypeBearer,
			IssuedAt:  now,
			ExpiresIn: 3600,
			Scopes:    []string{},
			ClientID:  testClientID,
		}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, oauthAppNoConfig)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_Success_WithJWTTokenType() {
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub": "user123",
		"iss": testCustomIssuer,
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
	})

	tokenRequest := &model.TokenRequest{
		GrantType:          string(providers.GrantTypeTokenExchange),
		ClientID:           testClientID,
		SubjectToken:       subjectToken,
		SubjectTokenType:   string(constants.TokenTypeIdentifierAccessToken),
		RequestedTokenType: string(constants.TokenTypeIdentifierJWT),
	}

	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub:            testUserID,
			Iss:            testCustomIssuer,
			Scopes:         []string{},
			UserAttributes: map[string]interface{}{},
			NestedAct:      nil,
		}, nil)
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything, mock.Anything).
		Return(&model.TokenDTO{
			Token:     testTokenExchangeJWT,
			TokenType: constants.TokenTypeBearer,
			IssuedAt:  now,
			ExpiresIn: 7200,
			Scopes:    []string{},
			ClientID:  testClientID,
		}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
	assert.NotEmpty(suite.T(), result.AccessToken.Token)
	// IssuedTokenType is determined at the token handler level, not the grant handler level
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestValidateGrant_UnsupportedIDTokenType() {
	tokenRequest := &model.TokenRequest{
		GrantType:          string(providers.GrantTypeTokenExchange),
		ClientID:           testClientID,
		SubjectToken:       "subject-token",
		SubjectTokenType:   string(constants.TokenTypeIdentifierAccessToken),
		RequestedTokenType: string(constants.TokenTypeIdentifierIDToken),
	}

	errResp := suite.handler.ValidateGrant(context.Background(), tokenRequest, suite.oauthApp)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, errResp.Error)
	assert.Contains(suite.T(), errResp.ErrorDescription, "Unsupported requested_token_type")
	assert.Contains(suite.T(), errResp.ErrorDescription, "Only access tokens, JWT tokens, and ID-JAGs are supported")
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestValidateGrant_UnsupportedRefreshTokenType() {
	tokenRequest := &model.TokenRequest{
		GrantType:          string(providers.GrantTypeTokenExchange),
		ClientID:           testClientID,
		SubjectToken:       "subject-token",
		SubjectTokenType:   string(constants.TokenTypeIdentifierAccessToken),
		RequestedTokenType: string(constants.TokenTypeIdentifierRefreshToken),
	}

	// Test ValidateGrant first (which is called before HandleGrant in production)
	errResp := suite.handler.ValidateGrant(context.Background(), tokenRequest, suite.oauthApp)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, errResp.Error)
	assert.Contains(suite.T(), errResp.ErrorDescription, "Unsupported requested_token_type")
	assert.Contains(suite.T(), errResp.ErrorDescription, "Only access tokens, JWT tokens, and ID-JAGs are supported")
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestRFC8693_CompleteTokenExchangeFlow() {
	// RFC 8693 Section 2.2: Verify all required response parameters
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub":   "user123",
		"iss":   testCustomIssuer,
		"aud":   "original-audience",
		"exp":   float64(now + 3600),
		"nbf":   float64(now - 60),
		"scope": "read write",
		"email": "user@example.com",
		"name":  "John Doe",
	})

	tokenRequest := &model.TokenRequest{
		GrantType:          string(providers.GrantTypeTokenExchange),
		ClientID:           testClientID,
		SubjectToken:       subjectToken,
		SubjectTokenType:   string(constants.TokenTypeIdentifierAccessToken),
		RequestedTokenType: string(constants.TokenTypeIdentifierAccessToken),
		Audiences:          []string{"https://target-service.com"},
		Scope:              "read",
	}

	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub:    testUserID,
			Iss:    testCustomIssuer,
			Scopes: []string{"read", "write"},
			UserAttributes: map[string]interface{}{
				"email": testUserEmail,
				"name":  "John Doe",
			},
			NestedAct: nil,
		}, nil)
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			// The audience parameter is ignored; with no resource parameter the token is bound to
			// the configured default resource server.
			return ctx.Subject == testUserID &&
				len(ctx.Audiences) == 1 &&
				ctx.Audiences[0] == testTokenExchangeDefaultRSAudience &&
				ctx.ClientID == testClientID &&
				tokenservice.JoinScopes(ctx.Scopes) == testScopeRead &&
				ctx.SubjectAttributes["email"] == "user@example.com" &&
				ctx.SubjectAttributes["name"] == "John Doe"
		})).Return(&model.TokenDTO{
		Token:     testTokenExchangeJWT,
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  now,
		ExpiresIn: 7200,
		Scopes:    []string{"read"},
		ClientID:  testClientID,
		UserAttributes: map[string]interface{}{
			"email": "user@example.com",
			"name":  "John Doe",
		},
	}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	// RFC 8693 Section 2.2: Verify required response parameters
	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
	assert.NotEmpty(suite.T(), result.AccessToken.Token)                             // access_token - REQUIRED
	assert.Equal(suite.T(), constants.TokenTypeBearer, result.AccessToken.TokenType) // token_type - REQUIRED
	assert.NotZero(suite.T(), result.AccessToken.ExpiresIn)                          // expires_in - RECOMMENDED
	assert.Equal(suite.T(), []string{"read"}, result.AccessToken.Scopes)
	// issued_token_type - REQUIRED
	// IssuedTokenType is determined at the token handler level, not the grant handler level
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_AudienceIgnoredWhenResourcePresent() {
	// The audience parameter is ignored even when combined with a resource parameter: the token is
	// bound to the single resolved resource server only.
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub": "user123",
		"iss": testCustomIssuer,
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
		"aud": "token-audience",
	})

	tokenRequest := &model.TokenRequest{
		GrantType:        string(providers.GrantTypeTokenExchange),
		ClientID:         testClientID,
		SubjectToken:     subjectToken,
		SubjectTokenType: string(constants.TokenTypeIdentifierAccessToken),
		Audiences:        []string{"request-audience"},
		Resources:        []string{"https://resource.example.com"},
	}

	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub:            testUserID,
			Iss:            testCustomIssuer,
			Scopes:         []string{},
			UserAttributes: map[string]interface{}{},
			NestedAct:      nil,
		}, nil)
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			// audience parameter dropped; aud is the resolved resource server only.
			return ctx.Subject == testUserID &&
				len(ctx.Audiences) == 1 &&
				ctx.Audiences[0] == "https://resource.example.com"
		})).Return(&model.TokenDTO{
		Token:     testTokenExchangeJWT,
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  now,
		ExpiresIn: 7200,
		Scopes:    []string{},
		ClientID:  testClientID,
	}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestRFC8693_ActorDelegationChain() {
	// RFC 8693 Section 4.1: Test nested actor delegation chains
	now := time.Now().Unix()

	// Subject token with existing actor
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub": "user123",
		"iss": testCustomIssuer,
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
		"act": map[string]interface{}{
			"sub": "previous-actor",
			"iss": "https://previous-issuer.com",
		},
	})

	// Actor token with its own actor chain
	actorToken := suite.createTestJWT(map[string]interface{}{
		"sub": "current-actor",
		"iss": testCustomIssuer,
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
		"act": map[string]interface{}{
			"sub": "actor-of-actor",
			"iss": "https://nested-issuer.com",
		},
	})

	tokenRequest := &model.TokenRequest{
		GrantType:        string(providers.GrantTypeTokenExchange),
		ClientID:         testClientID,
		SubjectToken:     subjectToken,
		SubjectTokenType: string(constants.TokenTypeIdentifierAccessToken),
		ActorToken:       actorToken,
		ActorTokenType:   string(constants.TokenTypeIdentifierAccessToken),
	}

	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub:            "user123",
			Iss:            testCustomIssuer,
			UserAttributes: map[string]interface{}{},
			NestedAct: map[string]interface{}{
				"sub": "previous-actor",
				"iss": "https://previous-issuer.com",
			},
		}, nil)
	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, actorToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub:            "current-actor",
			Iss:            testCustomIssuer,
			UserAttributes: map[string]interface{}{},
			NestedAct: map[string]interface{}{
				"sub": "actor-of-actor",
				"iss": "https://nested-issuer.com",
			},
		}, nil)
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			// Verify nested delegation chain per RFC 8693
			if ctx.ActorClaims == nil {
				return false
			}
			// Current actor
			if ctx.ActorClaims.Sub != "current-actor" || ctx.ActorClaims.Iss != testCustomIssuer {
				return false
			}
			// Check that actor token's act claim is preserved
			if len(ctx.ActorClaims.NestedAct) > 0 {
				return ctx.ActorClaims.NestedAct["sub"] == "actor-of-actor"
			}
			return false
		})).Return(&model.TokenDTO{
		Token:     testTokenExchangeJWT,
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  now,
		ExpiresIn: 7200,
		Scopes:    []string{},
		ClientID:  testClientID,
	}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_Success_WithActorTokenHasActButSubjectHasNoAct() {
	// Test case: Actor token has its own act claim, but subject token has no act claim
	now := time.Now().Unix()

	// Subject token WITHOUT act claim
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub": "user123",
		"iss": testCustomIssuer,
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
	})

	// Actor token WITH its own act claim
	actorToken := suite.createTestJWT(map[string]interface{}{
		"sub": "current-actor",
		"iss": testCustomIssuer,
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
		"act": map[string]interface{}{
			"sub": "actor-of-actor",
			"iss": "https://nested-issuer.com",
		},
	})

	tokenRequest := &model.TokenRequest{
		GrantType:        string(providers.GrantTypeTokenExchange),
		ClientID:         testClientID,
		SubjectToken:     subjectToken,
		SubjectTokenType: string(constants.TokenTypeIdentifierAccessToken),
		ActorToken:       actorToken,
		ActorTokenType:   string(constants.TokenTypeIdentifierAccessToken),
	}

	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub:            testUserID,
			Iss:            testCustomIssuer,
			UserAttributes: map[string]interface{}{},
			NestedAct:      nil,
		}, nil)
	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, actorToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub:            "current-actor",
			Iss:            testCustomIssuer,
			UserAttributes: map[string]interface{}{},
			NestedAct: map[string]interface{}{
				"sub": "actor-of-actor",
				"iss": "https://nested-issuer.com",
			},
		}, nil)
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			// Verify actor claim structure
			if ctx.ActorClaims == nil {
				return false
			}
			// Current actor should be present
			if ctx.ActorClaims.Sub != "current-actor" || ctx.ActorClaims.Iss != testCustomIssuer {
				return false
			}
			// Actor's act claim should be preserved directly
			if len(ctx.ActorClaims.NestedAct) > 0 {
				nestedAct := ctx.ActorClaims.NestedAct
				nestedSub := nestedAct["sub"] == "actor-of-actor"
				nestedIss := nestedAct["iss"] == "https://nested-issuer.com"
				if !nestedSub || !nestedIss {
					return false
				}
				// Subject has no act claim, so it should not be nested
				_, hasFurtherNesting := ctx.ActorClaims.NestedAct["act"]
				return !hasFurtherNesting
			}
			return false
		})).Return(&model.TokenDTO{
		Token:     testTokenExchangeJWT,
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  now,
		ExpiresIn: 7200,
		Scopes:    []string{},
		ClientID:  testClientID,
	}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestRFC8693_ScopeDownscopingEnforcement() {
	// RFC 8693 Section 5: Verify scope downscoping (security consideration)
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub":   "user123",
		"iss":   testCustomIssuer,
		"exp":   float64(now + 3600),
		"nbf":   float64(now - 60),
		"scope": "read write delete",
	})

	// Test 1: Valid downscoping (subset of scopes)
	tokenRequest := &model.TokenRequest{
		GrantType:        string(providers.GrantTypeTokenExchange),
		ClientID:         testClientID,
		SubjectToken:     subjectToken,
		SubjectTokenType: string(constants.TokenTypeIdentifierAccessToken),
		Scope:            "read",
	}

	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub:            testUserID,
			Iss:            testCustomIssuer,
			Scopes:         []string{"read", "write", "delete"},
			UserAttributes: map[string]interface{}{},
			NestedAct:      nil,
		}, nil)
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return tokenservice.JoinScopes(ctx.Scopes) == testScopeRead
		})).Return(&model.TokenDTO{
		Token:     testTokenExchangeJWT,
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  now,
		ExpiresIn: 7200,
		Scopes:    []string{"read"},
		ClientID:  testClientID,
	}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), []string{"read"}, result.AccessToken.Scopes)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestRFC8693_ResourceParameterValidation() {
	// RFC 8693 Section 2.1: Resource must be absolute URI without fragment
	tokenRequest := &model.TokenRequest{
		GrantType:        string(providers.GrantTypeTokenExchange),
		ClientID:         testClientID,
		SubjectToken:     "subject-token",
		SubjectTokenType: string(constants.TokenTypeIdentifierAccessToken),
		Resources:        []string{"https://api.example.com/v1/resource"},
	}

	result := suite.handler.ValidateGrant(context.Background(), tokenRequest, suite.oauthApp)
	assert.Nil(suite.T(), result)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestRFC8693_NoTokenLinkage() {
	// RFC 8693 Section 2.1: "exchange has no impact on the validity of the subject token"
	// This is a design verification test - token exchange should not invalidate input tokens
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub": "user123",
		"iss": testCustomIssuer,
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
	})

	tokenRequest := &model.TokenRequest{
		GrantType:        string(providers.GrantTypeTokenExchange),
		ClientID:         testClientID,
		SubjectToken:     subjectToken,
		SubjectTokenType: string(constants.TokenTypeIdentifierAccessToken),
	}

	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub:            testUserID,
			Iss:            testCustomIssuer,
			Scopes:         []string{},
			UserAttributes: map[string]interface{}{},
			NestedAct:      nil,
		}, nil)
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything, mock.Anything).
		Return(&model.TokenDTO{
			Token:     testTokenExchangeJWT,
			TokenType: constants.TokenTypeBearer,
			IssuedAt:  now,
			ExpiresIn: 7200,
			Scopes:    []string{},
			ClientID:  testClientID,
		}, nil)

	result1, errResp1 := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp1)
	assert.NotNil(suite.T(), result1)

	// Use same subject token again - should succeed (no linkage/invalidation)
	result2, errResp2 := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp2)
	assert.NotNil(suite.T(), result2)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestRFC8693_ClaimPreservation() {
	// Verify non-standard claims are preserved through token exchange
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub":          "user123",
		"iss":          testCustomIssuer,
		"exp":          float64(now + 3600),
		"nbf":          float64(now - 60),
		"email":        "user@example.com",
		"given_name":   "John",
		"family_name":  "Doe",
		"roles":        []interface{}{"admin", "user"},
		"organization": "ACME Corp",
	})

	tokenRequest := &model.TokenRequest{
		GrantType:        string(providers.GrantTypeTokenExchange),
		ClientID:         testClientID,
		SubjectToken:     subjectToken,
		SubjectTokenType: string(constants.TokenTypeIdentifierAccessToken),
	}

	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub:    testUserID,
			Iss:    testCustomIssuer,
			Scopes: []string{},
			UserAttributes: map[string]interface{}{
				"email":        "user@example.com",
				"given_name":   "John",
				"family_name":  "Doe",
				"roles":        []interface{}{"admin", "user"},
				"organization": "ACME Corp",
			},
			NestedAct: nil,
		}, nil)
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			// Verify all custom claims are preserved in user attributes
			return ctx.SubjectAttributes["email"] == testUserEmail &&
				ctx.SubjectAttributes["given_name"] == "John" &&
				ctx.SubjectAttributes["family_name"] == "Doe" &&
				ctx.SubjectAttributes["organization"] == "ACME Corp"
		})).Return(&model.TokenDTO{
		Token:     testTokenExchangeJWT,
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  now,
		ExpiresIn: 7200,
		Scopes:    []string{},
		ClientID:  testClientID,
		UserAttributes: map[string]interface{}{
			"email":        "user@example.com",
			"given_name":   "John",
			"family_name":  "Doe",
			"roles":        []interface{}{"admin", "user"},
			"organization": "ACME Corp",
		},
	}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)

	// Verify user attributes in response
	assert.Equal(suite.T(), testUserEmail, result.AccessToken.UserAttributes["email"])
	assert.Equal(suite.T(), "John", result.AccessToken.UserAttributes["given_name"])
	assert.Equal(suite.T(), "Doe", result.AccessToken.UserAttributes["family_name"])
	assert.Equal(suite.T(), "ACME Corp", result.AccessToken.UserAttributes["organization"])
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestIsSupportedTokenType() {
	assert.True(suite.T(), constants.TokenTypeIdentifierAccessToken.IsValid())
	assert.True(suite.T(), constants.TokenTypeIdentifierRefreshToken.IsValid())
	assert.True(suite.T(), constants.TokenTypeIdentifierIDToken.IsValid())
	assert.True(suite.T(), constants.TokenTypeIdentifierJWT.IsValid())
	assert.False(suite.T(), constants.TokenTypeIdentifier("urn:ietf:params:oauth:token-type:saml2").IsValid())
	assert.False(suite.T(), constants.TokenTypeIdentifier("invalid").IsValid())
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestExtractUserAttributes() {
	claims := map[string]interface{}{
		"sub":       testUserID,
		"iss":       "issuer",
		"aud":       "audience",
		"exp":       float64(123456789),
		"nbf":       float64(123456789),
		"iat":       float64(123456789),
		"jti":       "jwt-id",
		"scope":     "read write",
		"client_id": testClientID,
		"act":       map[string]interface{}{"sub": "actor"},
		"email":     testUserEmail,
		"name":      "Test User",
		"custom":    "value",
	}

	// Use the utility function from tokenservice
	userAttrs := tokenservice.ExtractUserAttributes(claims)

	assert.Equal(suite.T(), 3, len(userAttrs))
	assert.Equal(suite.T(), testUserEmail, userAttrs["email"])
	assert.Equal(suite.T(), "Test User", userAttrs["name"])
	assert.Equal(suite.T(), "value", userAttrs["custom"])
	assert.NotContains(suite.T(), userAttrs, "sub")
	assert.NotContains(suite.T(), userAttrs, "iss")
	assert.NotContains(suite.T(), userAttrs, "scope")
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_ServerAuthAssertion_Success() {
	defaultAudience := suite.getDefaultAudience()
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub":                    testUserID,
		"iss":                    testCustomIssuer,
		"aud":                    defaultAudience, // Match default audience
		"exp":                    float64(now + 3600),
		"nbf":                    float64(now - 60),
		"assurance":              map[string]interface{}{"aal": "AAL1", "ial": "IAL1"}, // Make it an auth assertion
		"authorized_permissions": "read:documents write:documents",
		"userType":               "person",
	}
	subjectToken := suite.createTestJWT(claims)

	tokenRequest := &model.TokenRequest{
		GrantType:        string(providers.GrantTypeTokenExchange),
		ClientID:         testClientID,
		SubjectToken:     subjectToken,
		SubjectTokenType: string(constants.TokenTypeIdentifierJWT),
	}

	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub:            testUserID,
			Iss:            testCustomIssuer,
			Aud:            []string{defaultAudience},
			Scopes:         []string{"read:documents", "write:documents"}, // Mapped from authorized_permissions
			UserAttributes: map[string]interface{}{"userType": "person"},
			NestedAct:      nil,
		}, nil)

	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return ctx.Subject == testUserID &&
				len(ctx.Scopes) == 2 &&
				ctx.Scopes[0] == "read:documents" &&
				ctx.Scopes[1] == "write:documents"
		})).Return(&model.TokenDTO{
		Token:     testTokenExchangeJWT,
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  now,
		ExpiresIn: 7200,
		Scopes:    []string{"read:documents", "write:documents"},
		ClientID:  testClientID,
	}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), testTokenExchangeJWT, result.AccessToken.Token)
	assert.Equal(suite.T(), []string{"read:documents", "write:documents"}, result.AccessToken.Scopes)
	suite.mockTokenValidator.AssertExpectations(suite.T())
	suite.mockTokenBuilder.AssertExpectations(suite.T())
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_ServerAuthAssertion_AudienceMismatch() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub":       testUserID,
		"iss":       testCustomIssuer,
		"aud":       "different-audience", // Doesn't match default audience or client app_id
		"exp":       float64(now + 3600),
		"nbf":       float64(now - 60),
		"assurance": map[string]interface{}{"aal": "AAL1", "ial": "IAL1"}, // Make it an auth assertion
	}
	subjectToken := suite.createTestJWT(claims)

	tokenRequest := &model.TokenRequest{
		GrantType:        string(providers.GrantTypeTokenExchange),
		ClientID:         testClientID,
		SubjectToken:     subjectToken,
		SubjectTokenType: string(constants.TokenTypeIdentifierJWT),
	}

	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(nil, fmt.Errorf("auth assertion audience mismatch"))

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, errResp.Error)
	assert.Equal(suite.T(), "Invalid subject_token", errResp.ErrorDescription)
	suite.mockTokenValidator.AssertExpectations(suite.T())
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_ServerAuthAssertion_MissingAudience() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub": testUserID,
		"iss": testCustomIssuer,
		// Missing aud claim
		"exp":       float64(now + 3600),
		"nbf":       float64(now - 60),
		"assurance": map[string]interface{}{"aal": "AAL1", "ial": "IAL1"}, // Make it an auth assertion
	}
	subjectToken := suite.createTestJWT(claims)

	tokenRequest := &model.TokenRequest{
		GrantType:        string(providers.GrantTypeTokenExchange),
		ClientID:         testClientID,
		SubjectToken:     subjectToken,
		SubjectTokenType: string(constants.TokenTypeIdentifierJWT),
	}

	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(nil, fmt.Errorf("server auth assertion is missing 'aud' claim"))

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, errResp.Error)
	assert.Equal(suite.T(), "Invalid subject_token", errResp.ErrorDescription)
	suite.mockTokenValidator.AssertExpectations(suite.T())
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_ServerAuthAssertion_ClientIDMatch() {
	defaultAudience := suite.getDefaultAudience()
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub":                    testUserID,
		"iss":                    testCustomIssuer,
		"aud":                    defaultAudience, // Match default audience
		"exp":                    float64(now + 3600),
		"nbf":                    float64(now - 60),
		"assurance":              map[string]interface{}{"aal": "AAL1", "ial": "IAL1"}, // Make it an auth assertion
		"authorized_permissions": "read write",
	}
	subjectToken := suite.createTestJWT(claims)

	tokenRequest := &model.TokenRequest{
		GrantType:        string(providers.GrantTypeTokenExchange),
		ClientID:         testClientID,
		SubjectToken:     subjectToken,
		SubjectTokenType: string(constants.TokenTypeIdentifierJWT),
	}

	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub:            testUserID,
			Iss:            testCustomIssuer,
			Aud:            []string{defaultAudience},
			Scopes:         []string{"read", "write"},
			UserAttributes: map[string]interface{}{},
			NestedAct:      nil,
		}, nil)

	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything, mock.Anything).Return(&model.TokenDTO{
		Token:     testTokenExchangeJWT,
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  now,
		ExpiresIn: 7200,
		Scopes:    []string{"read", "write"},
		ClientID:  testClientID,
	}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
	suite.mockTokenValidator.AssertExpectations(suite.T())
	suite.mockTokenBuilder.AssertExpectations(suite.T())
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_ServerAuthAssertion_WithClientAppID() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub":                    testUserID,
		"iss":                    testCustomIssuer,
		"aud":                    suite.oauthApp.ID, // Match client app_id
		"exp":                    float64(now + 3600),
		"nbf":                    float64(now - 60),
		"assurance":              map[string]interface{}{"aal": "AAL1", "ial": "IAL1"}, // Make it an auth assertion
		"authorized_permissions": "read:documents write:documents",
		"userType":               "person",
	}
	subjectToken := suite.createTestJWT(claims)

	tokenRequest := &model.TokenRequest{
		GrantType:        string(providers.GrantTypeTokenExchange),
		ClientID:         testClientID,
		SubjectToken:     subjectToken,
		SubjectTokenType: string(constants.TokenTypeIdentifierJWT),
	}

	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub:            testUserID,
			Iss:            testCustomIssuer,
			Aud:            []string{suite.oauthApp.ID},
			Scopes:         []string{"read:documents", "write:documents"},
			UserAttributes: map[string]interface{}{"userType": "person"},
			NestedAct:      nil,
		}, nil)

	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return ctx.Subject == testUserID &&
				len(ctx.Scopes) == 2 &&
				ctx.Scopes[0] == "read:documents" &&
				ctx.Scopes[1] == "write:documents"
		})).Return(&model.TokenDTO{
		Token:     testTokenExchangeJWT,
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  now,
		ExpiresIn: 7200,
		Scopes:    []string{"read:documents", "write:documents"},
		ClientID:  testClientID,
	}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), testTokenExchangeJWT, result.AccessToken.Token)
	assert.Equal(suite.T(), []string{"read:documents", "write:documents"}, result.AccessToken.Scopes)
	suite.mockTokenValidator.AssertExpectations(suite.T())
	suite.mockTokenBuilder.AssertExpectations(suite.T())
}

// ============================================================================
// Single-audience token binding — the token is bound to exactly one resource server
// (RFC 8707 resource parameter or the configured default). The RFC 8693 audience parameter
// is ignored.
// ============================================================================

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_SingleResource_AudIsResolvedRS() { //nolint:dupl
	// One resource=https://rs01 (registered) → aud = [testRS01URI].
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub": testUserID,
		"iss": testCustomIssuer,
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
	})

	tokenRequest := suite.createBasicTokenRequest(subjectToken)
	tokenRequest.Resources = []string{testRS01URI}

	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub:            testUserID,
			Scopes:         []string{},
			UserAttributes: map[string]interface{}{},
		}, nil)
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return len(ctx.Audiences) == 1 && ctx.Audiences[0] == testRS01URI
		})).Return(&model.TokenDTO{Token: testTokenExchangeJWT, IssuedAt: now, ExpiresIn: 7200}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_MultipleResources_InvalidTarget() {
	// More than one resource parameter → invalid_target (a token binds to a single RS).
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub": testUserID,
		"iss": testCustomIssuer,
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
	})

	tokenRequest := suite.createBasicTokenRequest(subjectToken)
	tokenRequest.Resources = []string{testRS01URI, testRS02URI}

	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub:            testUserID,
			Scopes:         []string{},
			UserAttributes: map[string]interface{}{},
		}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorInvalidTarget, errResp.Error)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_NoResource_DefaultRSAud() { //nolint:dupl
	// Permission scope, no resource parameter, and a default RS configured use the default RS as aud.
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub": testUserID,
		"iss": testCustomIssuer,
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
	})

	tokenRequest := suite.createBasicTokenRequest(subjectToken)

	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub:            testUserID,
			Scopes:         []string{"read"},
			UserAttributes: map[string]interface{}{},
		}, nil)
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return len(ctx.Audiences) == 1 && ctx.Audiences[0] == testTokenExchangeDefaultRSAudience
		})).Return(&model.TokenDTO{Token: testTokenExchangeJWT, IssuedAt: now, ExpiresIn: 7200}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_NoResource_NoDefaultConfigured_InvalidTarget() {
	// Permission scope, no resource parameter, and no default RS configured → invalid_target.
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub": testUserID,
		"iss": testCustomIssuer,
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
	})

	tokenRequest := suite.createBasicTokenRequest(subjectToken)

	// Fresh mocks so the default GetMergedConfig / GetResourceServer stubs from SetupTest are not
	// used; the default RS ID is empty here.
	rsvc := resourcemock.NewResourceServiceInterfaceMock(suite.T())
	scfg := serverconfigmock.NewServerConfigServiceMock(suite.T())
	scfg.On("GetMergedConfig", mock.Anything, "defaultResourceServer").
		Return(resource.DefaultResourceServerConfig{ResourceServerID: ""}, nil)
	h := &tokenExchangeGrantHandler{
		tokenBuilder:        suite.mockTokenBuilder,
		tokenValidator:      suite.mockTokenValidator,
		authzService:        suite.mockAuthzService,
		actorProvider:       suite.mockActorProvider,
		resourceService:     rsvc,
		serverConfigService: scfg,
	}

	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub:            testUserID,
			Scopes:         []string{"read"},
			UserAttributes: map[string]interface{}{},
		}, nil)

	result, errResp := h.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorInvalidTarget, errResp.Error)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_AudienceParamIgnored_NoResource() { //nolint:dupl
	// audience parameter supplied, no resource → audience ignored, aud = [testTokenExchangeDefaultRSAudience].
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub": testUserID,
		"iss": testCustomIssuer,
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
	})

	tokenRequest := suite.createBasicTokenRequest(subjectToken)
	tokenRequest.Audiences = []string{"https://other-service.example.com"}

	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub:            testUserID,
			Scopes:         []string{"read"},
			UserAttributes: map[string]interface{}{},
		}, nil)
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return len(ctx.Audiences) == 1 && ctx.Audiences[0] == testTokenExchangeDefaultRSAudience
		})).Return(&model.TokenDTO{Token: testTokenExchangeJWT, IssuedAt: now, ExpiresIn: 7200}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_AudienceParamIgnored_WithResource() { //nolint:dupl
	// audience parameter supplied with a resource → audience ignored, aud = [testRS01URI].
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub": testUserID,
		"iss": testCustomIssuer,
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
	})

	tokenRequest := suite.createBasicTokenRequest(subjectToken)
	tokenRequest.Audiences = []string{"https://other-service.example.com"}
	tokenRequest.Resources = []string{testRS01URI}

	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub:            testUserID,
			Scopes:         []string{},
			UserAttributes: map[string]interface{}{},
		}, nil)
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return len(ctx.Audiences) == 1 && ctx.Audiences[0] == testRS01URI
		})).Return(&model.TokenDTO{Token: testTokenExchangeJWT, IssuedAt: now, ExpiresIn: 7200}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
}

// ============================================================================
// RFC 8707 §2.2 — per-RS scope downscoping in token_exchange
// ============================================================================

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_RFC8707_Resource_ScopesNarrowedToRSPermissions() {
	// Subject token has [read, write, admin]; RS defines [read, write].
	// Expect token scopes = [read, write] (admin dropped by RS intersection).
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub":   testUserID,
		"iss":   testCustomIssuer,
		"exp":   float64(now + 3600),
		"nbf":   float64(now - 60),
		"scope": "read write admin",
	})

	tokenRequest := suite.createBasicTokenRequest(subjectToken)
	tokenRequest.Resources = []string{testRS01URI}

	// Use a fresh resource service mock so the catch-all from SetupTest does not shadow
	// the specific ValidatePermissions expectation.
	rsvc := resourcemock.NewResourceServiceInterfaceMock(suite.T())
	rsvc.On("GetResourceServerByIdentifier", mock.Anything, testRS01URI).
		Return(&providers.ResourceServer{ID: testRS01URI, Identifier: testRS01URI}, nil)
	// RS only defines [read, write]; ValidatePermissions returns the invalid one (admin).
	rsvc.On("ValidatePermissions", mock.Anything, testRS01URI, []string{"read", "write", "admin"}).
		Return([]string{"admin"}, nil)
	h := &tokenExchangeGrantHandler{
		tokenBuilder:    suite.mockTokenBuilder,
		tokenValidator:  suite.mockTokenValidator,
		authzService:    suite.mockAuthzService,
		actorProvider:   suite.mockActorProvider,
		resourceService: rsvc,
	}

	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub:            testUserID,
			Scopes:         []string{"read", "write", "admin"},
			UserAttributes: map[string]interface{}{},
		}, nil)
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return tokenservice.JoinScopes(ctx.Scopes) == testScopeReadWrite
		})).Return(&model.TokenDTO{
		Token:     testTokenExchangeJWT,
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  now,
		ExpiresIn: 7200,
		Scopes:    []string{"read", "write"},
		ClientID:  testClientID,
	}, nil)

	result, errResp := h.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), []string{"read", "write"}, result.AccessToken.Scopes)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_RFC8707_ScopeNotOnRS_Dropped() {
	// Subject token [read, write, admin], explicit scope=read write, RS defines [read].
	// getScopes gives [read, write] (admin not in scope param); RS intersection drops write → [read].
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub":   testUserID,
		"iss":   testCustomIssuer,
		"exp":   float64(now + 3600),
		"nbf":   float64(now - 60),
		"scope": "read write admin",
	})

	tokenRequest := suite.createBasicTokenRequest(subjectToken)
	tokenRequest.Resources = []string{testRS01URI}
	tokenRequest.Scope = "read write"

	// Use a fresh resource service mock so the catch-all from SetupTest does not shadow
	// the specific ValidatePermissions expectation.
	rsvc := resourcemock.NewResourceServiceInterfaceMock(suite.T())
	rsvc.On("GetResourceServerByIdentifier", mock.Anything, testRS01URI).
		Return(&providers.ResourceServer{ID: testRS01URI, Identifier: testRS01URI}, nil)
	// RS defines [read] only; ValidatePermissions returns [write] as invalid.
	rsvc.On("ValidatePermissions", mock.Anything, testRS01URI, []string{"read", "write"}).
		Return([]string{"write"}, nil)
	h := &tokenExchangeGrantHandler{
		tokenBuilder:    suite.mockTokenBuilder,
		tokenValidator:  suite.mockTokenValidator,
		authzService:    suite.mockAuthzService,
		actorProvider:   suite.mockActorProvider,
		resourceService: rsvc,
	}

	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub:            testUserID,
			Scopes:         []string{"read", "write", "admin"},
			UserAttributes: map[string]interface{}{},
		}, nil)
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return tokenservice.JoinScopes(ctx.Scopes) == testScopeRead
		})).Return(&model.TokenDTO{
		Token:     testTokenExchangeJWT,
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  now,
		ExpiresIn: 7200,
		Scopes:    []string{"read"},
		ClientID:  testClientID,
	}, nil)

	result, errResp := h.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), []string{"read"}, result.AccessToken.Scopes)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_RFC8707_ScopesNarrowedToIssuingAppPermissions() {
	// Subject token has [read, write], target RS defines both, but the issuing OAuth app is only
	// authorized for [read]. The exchanged token must not carry write.
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub":   testUserID,
		"iss":   testCustomIssuer,
		"exp":   float64(now + 3600),
		"nbf":   float64(now - 60),
		"scope": "read write",
	})

	tokenRequest := suite.createBasicTokenRequest(subjectToken)
	tokenRequest.Resources = []string{testRS01URI}

	rsvc := resourcemock.NewResourceServiceInterfaceMock(suite.T())
	rsvc.On("GetResourceServerByIdentifier", mock.Anything, testRS01URI).
		Return(&providers.ResourceServer{ID: testRS01URI, Identifier: testRS01URI}, nil)
	rsvc.On("ValidatePermissions", mock.Anything, testRS01URI, []string{"read", "write"}).
		Return([]string{}, nil)

	authzSvc := authzmock.NewAuthorizationProviderMock(suite.T())
	authzSvc.On("EvaluateAccessBatch", mock.Anything,
		mock.MatchedBy(func(req providers.AccessEvaluationsRequest) bool {
			if len(req.Evaluations) != 2 {
				return false
			}
			return req.Evaluations[0].Subject.ID == suite.oauthApp.ID &&
				req.Evaluations[0].ResourceServer.ID == testRS01URI &&
				req.Evaluations[0].Permission.Name == "read" &&
				req.Evaluations[1].Permission.Name == "write"
		})).
		Return(&providers.AccessEvaluationsResponse{
			Evaluations: []providers.AccessEvaluationResponse{
				{Decision: true},
				{Decision: false},
			},
		}, nil)

	h := &tokenExchangeGrantHandler{
		tokenBuilder:    suite.mockTokenBuilder,
		tokenValidator:  suite.mockTokenValidator,
		authzService:    authzSvc,
		actorProvider:   suite.mockActorProvider,
		resourceService: rsvc,
	}

	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub:            testUserID,
			Scopes:         []string{"read", "write"},
			UserAttributes: map[string]interface{}{},
		}, nil)
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return tokenservice.JoinScopes(ctx.Scopes) == testScopeRead
		})).Return(&model.TokenDTO{
		Token:     testTokenExchangeJWT,
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  now,
		ExpiresIn: 7200,
		Scopes:    []string{"read"},
		ClientID:  testClientID,
	}, nil)

	result, errResp := h.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), []string{"read"}, result.AccessToken.Scopes)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_RFC8707_ResourceOmitted_ScopesUnchanged() {
	// No resource param → ComputeRSValidScopes is never called; scopes come straight from getScopes.
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub":   testUserID,
		"iss":   testCustomIssuer,
		"exp":   float64(now + 3600),
		"nbf":   float64(now - 60),
		"scope": "read write",
	})

	tokenRequest := suite.createBasicTokenRequest(subjectToken)

	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub:            testUserID,
			Scopes:         []string{"read", "write"},
			UserAttributes: map[string]interface{}{},
		}, nil)
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return tokenservice.JoinScopes(ctx.Scopes) == testScopeReadWrite
		})).Return(&model.TokenDTO{
		Token:     testTokenExchangeJWT,
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  now,
		ExpiresIn: 7200,
		Scopes:    []string{"read", "write"},
		ClientID:  testClientID,
	}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), []string{"read", "write"}, result.AccessToken.Scopes)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_DPoPProof_PropagatesJktToBuilder() {
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub":   testUserID,
		"iss":   testCustomIssuer,
		"exp":   float64(now + 3600),
		"scope": "read",
	})

	tokenRequest := suite.createBasicTokenRequest(subjectToken)

	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub:    testUserID,
			Iss:    testCustomIssuer,
			Scopes: []string{"read"},
		}, nil)
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return ctx.DPoPJkt == "thumbprint-tx"
		})).Return(&model.TokenDTO{
		Token:     testTokenExchangeJWT,
		TokenType: constants.TokenTypeDPoP,
		IssuedAt:  now,
		ExpiresIn: 7200,
		Scopes:    []string{"read"},
		ClientID:  testClientID,
	}, nil)

	ctx := dpop.WithJkt(context.Background(), "thumbprint-tx")
	result, errResp := suite.handler.HandleGrant(ctx, tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), constants.TokenTypeDPoP, result.AccessToken.TokenType)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_BoundSubjectToken_ProofMatches_Success() {
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub":   testUserID,
		"iss":   testCustomIssuer,
		"exp":   float64(now + 3600),
		"scope": "read",
		"cnf":   map[string]interface{}{"jkt": "thumbprint-tx"},
	})

	tokenRequest := suite.createBasicTokenRequest(subjectToken)

	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub:    testUserID,
			Iss:    testCustomIssuer,
			Scopes: []string{"read"},
			CnfJkt: "thumbprint-tx",
		}, nil)
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return ctx.DPoPJkt == "thumbprint-tx"
		})).Return(&model.TokenDTO{
		Token:     testTokenExchangeJWT,
		TokenType: constants.TokenTypeDPoP,
		IssuedAt:  now,
		ExpiresIn: 7200,
		Scopes:    []string{"read"},
		ClientID:  testClientID,
	}, nil)

	ctx := dpop.WithJkt(context.Background(), "thumbprint-tx")
	result, errResp := suite.handler.HandleGrant(ctx, tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), constants.TokenTypeDPoP, result.AccessToken.TokenType)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_BoundSubjectToken_NoProof_InvalidDPoPProof() {
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub":   testUserID,
		"iss":   testCustomIssuer,
		"exp":   float64(now + 3600),
		"scope": "read",
		"cnf":   map[string]interface{}{"jkt": "thumbprint-tx"},
	})

	tokenRequest := suite.createBasicTokenRequest(subjectToken)

	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub:    testUserID,
			Iss:    testCustomIssuer,
			Scopes: []string{"read"},
			CnfJkt: "thumbprint-tx",
		}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorInvalidGrant, errResp.Error)
	assert.Contains(suite.T(), errResp.ErrorDescription, "DPoP proof required")
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_BoundSubjectToken_ProofMismatch_InvalidDPoPProof() {
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub":   testUserID,
		"iss":   testCustomIssuer,
		"exp":   float64(now + 3600),
		"scope": "read",
		"cnf":   map[string]interface{}{"jkt": "thumbprint-tx"},
	})

	tokenRequest := suite.createBasicTokenRequest(subjectToken)

	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub:    testUserID,
			Iss:    testCustomIssuer,
			Scopes: []string{"read"},
			CnfJkt: "thumbprint-tx",
		}, nil)

	ctx := dpop.WithJkt(context.Background(), "thumbprint-other")
	result, errResp := suite.handler.HandleGrant(ctx, tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorInvalidGrant, errResp.Error)
	assert.Contains(suite.T(), errResp.ErrorDescription, "does not match")
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_RFC8707_MultipleResources_InvalidTarget() {
	// A token binds to exactly one resource server, so multiple resource indicators are rejected.
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub":   testUserID,
		"iss":   testCustomIssuer,
		"exp":   float64(now + 3600),
		"nbf":   float64(now - 60),
		"scope": "read write admin",
	})

	const testRS02URI = "https://rs02.example.com"

	tokenRequest := suite.createBasicTokenRequest(subjectToken)
	tokenRequest.Resources = []string{testRS01URI, testRS02URI}

	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub:            testUserID,
			Scopes:         []string{"read", "write", "admin"},
			UserAttributes: map[string]interface{}{},
		}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorInvalidTarget, errResp.Error)
	suite.mockTokenBuilder.AssertNotCalled(suite.T(), "BuildAccessToken", mock.Anything, mock.Anything)
}

// filterScopesAuthorizedForApp error paths.

func (suite *TokenExchangeGrantHandlerTestSuite) TestFilterScopesAuthorizedForApp_NilAuthzService_ReturnsServerError() {
	handler := &tokenExchangeGrantHandler{}

	scopes, errResp := handler.filterScopesAuthorizedForApp(
		context.Background(), suite.oauthApp, "rs-1", []string{"read"})

	assert.Nil(suite.T(), scopes)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorServerError, errResp.Error)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestFilterScopesAuthorizedForApp_ActorGroupsError() {
	actorProvider := actorprovidermock.NewActorProviderMock(suite.T())
	actorProvider.On("GetActorGroups", mock.Anything).
		Return([]providers.EntityGroup(nil), &tidcommon.ServiceError{
			Code:  "AZ-0001",
			Error: tidcommon.I18nMessage{DefaultValue: "group lookup failed"},
		})
	handler := &tokenExchangeGrantHandler{
		authzService:  suite.mockAuthzService,
		actorProvider: actorProvider,
	}

	scopes, errResp := handler.filterScopesAuthorizedForApp(
		context.Background(), suite.oauthApp, "rs-1", []string{"read"})

	assert.Nil(suite.T(), scopes)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorServerError, errResp.Error)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestFilterScopesAuthorizedForApp_EvaluateError_ReturnsServerError() {
	authzService := authzmock.NewAuthorizationProviderMock(suite.T())
	authzService.On("EvaluateAccessBatch", mock.Anything, mock.Anything).
		Return((*providers.AccessEvaluationsResponse)(nil), &tidcommon.ServiceError{
			Code:  "AZ-0002",
			Error: tidcommon.I18nMessage{DefaultValue: "evaluation failed"},
		})
	// actorProvider is nil, so app group resolution is skipped and evaluation runs directly.
	handler := &tokenExchangeGrantHandler{authzService: authzService}

	scopes, errResp := handler.filterScopesAuthorizedForApp(
		context.Background(), suite.oauthApp, "rs-1", []string{"read"})

	assert.Nil(suite.T(), scopes)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorServerError, errResp.Error)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestFilterScopesAuthorizedForApp_WithAppGroups() {
	actorProvider := actorprovidermock.NewActorProviderMock(suite.T())
	actorProvider.On("GetActorGroups", mock.Anything).
		Return([]providers.EntityGroup{{ID: "group-1"}}, nil)
	authzService := authzmock.NewAuthorizationProviderMock(suite.T())
	authzService.On("EvaluateAccessBatch", mock.Anything, mock.Anything).
		Return(
			func(_ context.Context, request providers.AccessEvaluationsRequest) *providers.AccessEvaluationsResponse {
				evaluations := make([]providers.AccessEvaluationResponse, 0, len(request.Evaluations))
				for range request.Evaluations {
					evaluations = append(evaluations, providers.AccessEvaluationResponse{Decision: true})
				}
				return &providers.AccessEvaluationsResponse{Evaluations: evaluations}
			},
			nil,
		)
	handler := &tokenExchangeGrantHandler{authzService: authzService, actorProvider: actorProvider}

	scopes, errResp := handler.filterScopesAuthorizedForApp(
		context.Background(), suite.oauthApp, "rs-1", []string{"read"})

	assert.Nil(suite.T(), errResp)
	assert.Equal(suite.T(), []string{"read"}, scopes)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_DownscopeValidationError() {
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub": testUserID,
		"iss": testCustomIssuer,
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
	})
	tokenRequest := suite.createBasicTokenRequest(subjectToken)
	tokenRequest.Resources = []string{"https://rs.example.com"}

	rsvc := resourcemock.NewResourceServiceInterfaceMock(suite.T())
	rsvc.On("GetResourceServerByIdentifier", mock.Anything, "https://rs.example.com").
		Return(&providers.ResourceServer{ID: "rs-x", Identifier: "https://rs.example.com"}, nil)
	rsvc.On("ValidatePermissions", mock.Anything, mock.Anything, mock.Anything).
		Return([]string(nil), &tidcommon.ServiceError{Type: tidcommon.ServerErrorType, Code: "RES-5001"})
	handler := &tokenExchangeGrantHandler{
		tokenBuilder:        suite.mockTokenBuilder,
		tokenValidator:      suite.mockTokenValidator,
		authzService:        suite.mockAuthzService,
		actorProvider:       suite.mockActorProvider,
		resourceService:     rsvc,
		serverConfigService: suite.mockServerConfigSvc,
	}
	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub:            testUserID,
			Scopes:         []string{"read"},
			UserAttributes: map[string]interface{}{},
		}, nil)

	result, errResp := handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorServerError, errResp.Error)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_AppAuthorizationError() {
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub": testUserID,
		"iss": testCustomIssuer,
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
	})
	tokenRequest := suite.createBasicTokenRequest(subjectToken)
	tokenRequest.Resources = []string{"https://rs.example.com"}

	rsvc := resourcemock.NewResourceServiceInterfaceMock(suite.T())
	rsvc.On("GetResourceServerByIdentifier", mock.Anything, "https://rs.example.com").
		Return(&providers.ResourceServer{ID: "rs-x", Identifier: "https://rs.example.com"}, nil)
	rsvc.On("ValidatePermissions", mock.Anything, mock.Anything, mock.Anything).
		Return([]string{}, nil)
	authzService := authzmock.NewAuthorizationProviderMock(suite.T())
	authzService.On("EvaluateAccessBatch", mock.Anything, mock.Anything).
		Return((*providers.AccessEvaluationsResponse)(nil), &tidcommon.ServiceError{
			Type: tidcommon.ServerErrorType,
			Code: "AZ-5000",
		})
	// actorProvider nil so app group resolution is skipped and evaluation runs directly.
	handler := &tokenExchangeGrantHandler{
		tokenBuilder:        suite.mockTokenBuilder,
		tokenValidator:      suite.mockTokenValidator,
		authzService:        authzService,
		resourceService:     rsvc,
		serverConfigService: suite.mockServerConfigSvc,
	}
	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub:            testUserID,
			Scopes:         []string{"read"},
			UserAttributes: map[string]interface{}{},
		}, nil)

	result, errResp := handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorServerError, errResp.Error)
}

// ============================================================================
// ID-JAG Issuance Tests (draft-ietf-oauth-identity-assertion-authz-grant)
// The server issuer configured in SetupTest is testIDJAGServerIssuer.
// ============================================================================

const testIDJAGServerIssuer = "https://auth.example.com"
const testIDJAGAudience = "https://rs.example.com"

func (suite *TokenExchangeGrantHandlerTestSuite) TestValidateGrant_IDJAGRequestedTokenType() {
	tokenRequest := &model.TokenRequest{
		GrantType:          string(providers.GrantTypeTokenExchange),
		ClientID:           testClientID,
		SubjectToken:       "subject-token",
		SubjectTokenType:   string(constants.TokenTypeIdentifierIDToken),
		RequestedTokenType: string(constants.TokenTypeIdentifierIDJAG),
	}

	result := suite.handler.ValidateGrant(context.Background(), tokenRequest, suite.oauthApp)
	assert.Nil(suite.T(), result)
}

// An ID-JAG request must present an ID token as the subject_token; any other subject_token_type is
// rejected as invalid_request per the draft's restriction to identity assertions.
func (suite *TokenExchangeGrantHandlerTestSuite) TestValidateGrant_IDJAGWrongSubjectTokenType() {
	tokenRequest := &model.TokenRequest{
		GrantType:          string(providers.GrantTypeTokenExchange),
		ClientID:           testClientID,
		SubjectToken:       "subject-token",
		SubjectTokenType:   string(constants.TokenTypeIdentifierAccessToken),
		RequestedTokenType: string(constants.TokenTypeIdentifierIDJAG),
	}

	result := suite.handler.ValidateGrant(context.Background(), tokenRequest, suite.oauthApp)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, result.Error)
	assert.Contains(suite.T(), result.ErrorDescription,
		"ID-JAG requests require subject_token_type urn:ietf:params:oauth:token-type:id_token")
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_IDJAG_Success() {
	suite.oauthApp.Token.IDJAG = &providers.IDJAGConfig{
		Enabled:          true,
		AllowedAudiences: []string{testIDJAGAudience},
	}
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub": testUserID,
		"iss": testIDJAGServerIssuer,
		"aud": testClientID,
		"exp": float64(now + 3600),
	})

	tokenRequest := &model.TokenRequest{
		GrantType:          string(providers.GrantTypeTokenExchange),
		ClientID:           testClientID,
		SubjectToken:       subjectToken,
		SubjectTokenType:   string(constants.TokenTypeIdentifierIDToken),
		RequestedTokenType: string(constants.TokenTypeIdentifierIDJAG),
		Audiences:          []string{testIDJAGAudience},
		Scope:              testScopeRead,
	}

	suite.mockTokenValidator.On("ValidateIDJAGSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub: testUserID,
			Iss: testIDJAGServerIssuer,
			Aud: []string{testClientID},
		}, nil)
	suite.mockTokenBuilder.On("BuildIDJAG", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.IDJAGBuildContext) bool {
			return ctx.Subject == testUserID &&
				ctx.Audience == testIDJAGAudience &&
				ctx.ClientID == testClientID &&
				tokenservice.JoinScopes(ctx.Scopes) == testScopeRead
		})).Return(&model.TokenDTO{
		Token:     "test-id-jag",
		TokenType: constants.TokenTypeNA,
		IssuedAt:  now,
		ExpiresIn: 300,
		Scopes:    []string{testScopeRead},
		ClientID:  testClientID,
		Subject:   testUserID,
		Audiences: []string{testIDJAGAudience},
	}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "test-id-jag", result.AccessToken.Token)
	assert.Equal(suite.T(), constants.TokenTypeNA, result.AccessToken.TokenType)
	assert.Equal(suite.T(), testClientID, result.AccessToken.ClientID)
	assert.Equal(suite.T(), testUserID, result.AccessToken.Subject)
	assert.Equal(suite.T(), []string{testIDJAGAudience}, result.AccessToken.Audiences)
	assert.Equal(suite.T(), []string{testScopeRead}, result.AccessToken.Scopes)
	// No refresh token is issued for ID-JAGs.
	assert.Empty(suite.T(), result.RefreshToken.Token)
}

// RFC 8707: a resource parameter present on an ID-JAG request is embedded in the issued ID-JAG's
// resource claim, threaded through IDJAGBuildContext.Resources.
func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_IDJAG_ResourcePresent_ThreadedToBuildContext() {
	suite.oauthApp.Token.IDJAG = &providers.IDJAGConfig{
		Enabled:          true,
		AllowedAudiences: []string{testIDJAGAudience},
	}
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub": testUserID,
		"iss": testIDJAGServerIssuer,
		"aud": testClientID,
		"exp": float64(now + 3600),
	})

	tokenRequest := &model.TokenRequest{
		GrantType:          string(providers.GrantTypeTokenExchange),
		ClientID:           testClientID,
		SubjectToken:       subjectToken,
		SubjectTokenType:   string(constants.TokenTypeIdentifierIDToken),
		RequestedTokenType: string(constants.TokenTypeIdentifierIDJAG),
		Audiences:          []string{testIDJAGAudience},
		Resources:          []string{testRS01URI},
	}

	suite.mockTokenValidator.On("ValidateIDJAGSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub: testUserID,
			Iss: testIDJAGServerIssuer,
			Aud: []string{testClientID},
		}, nil)
	suite.mockTokenBuilder.On("BuildIDJAG", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.IDJAGBuildContext) bool {
			return len(ctx.Resources) == 1 && ctx.Resources[0] == testRS01URI
		})).Return(&model.TokenDTO{
		Token:     "test-id-jag",
		TokenType: constants.TokenTypeNA,
		IssuedAt:  now,
		ExpiresIn: 300,
		ClientID:  testClientID,
		Subject:   testUserID,
		Audiences: []string{testIDJAGAudience},
	}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
}

// RFC 8707: when no resource parameter is present, the ID-JAG is still valid and no Resources are
// threaded to the build context.
func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_IDJAG_ResourceAbsent_BuildContextEmpty() {
	suite.oauthApp.Token.IDJAG = &providers.IDJAGConfig{
		Enabled:          true,
		AllowedAudiences: []string{testIDJAGAudience},
	}
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub": testUserID,
		"iss": testIDJAGServerIssuer,
		"aud": testClientID,
		"exp": float64(now + 3600),
	})

	tokenRequest := &model.TokenRequest{
		GrantType:          string(providers.GrantTypeTokenExchange),
		ClientID:           testClientID,
		SubjectToken:       subjectToken,
		SubjectTokenType:   string(constants.TokenTypeIdentifierIDToken),
		RequestedTokenType: string(constants.TokenTypeIdentifierIDJAG),
		Audiences:          []string{testIDJAGAudience},
	}

	suite.mockTokenValidator.On("ValidateIDJAGSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub: testUserID,
			Iss: testIDJAGServerIssuer,
			Aud: []string{testClientID},
		}, nil)
	suite.mockTokenBuilder.On("BuildIDJAG", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.IDJAGBuildContext) bool {
			return len(ctx.Resources) == 0
		})).Return(&model.TokenDTO{
		Token:     "test-id-jag",
		TokenType: constants.TokenTypeNA,
		IssuedAt:  now,
		ExpiresIn: 300,
		ClientID:  testClientID,
		Subject:   testUserID,
		Audiences: []string{testIDJAGAudience},
	}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
}

// RFC 8707 §2: an invalid resource URI (not an absolute URI) is rejected as invalid_target before any
// subject token validation occurs.
func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_IDJAG_InvalidResourceURIRejected() {
	suite.oauthApp.Token.IDJAG = &providers.IDJAGConfig{
		Enabled:          true,
		AllowedAudiences: []string{testIDJAGAudience},
	}
	tokenRequest := &model.TokenRequest{
		GrantType:          string(providers.GrantTypeTokenExchange),
		ClientID:           testClientID,
		SubjectToken:       "subject-token",
		SubjectTokenType:   string(constants.TokenTypeIdentifierIDToken),
		RequestedTokenType: string(constants.TokenTypeIdentifierIDJAG),
		Audiences:          []string{testIDJAGAudience},
		Resources:          []string{"not-a-valid-uri"},
	}

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorInvalidTarget, errResp.Error)
	suite.mockTokenValidator.AssertNotCalled(suite.T(), "ValidateIDJAGSubjectToken",
		mock.Anything, mock.Anything, mock.Anything)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_IDJAG_AudienceNotAllowed() {
	suite.oauthApp.Token.IDJAG = &providers.IDJAGConfig{
		Enabled:          true,
		AllowedAudiences: []string{testIDJAGAudience},
	}
	tokenRequest := &model.TokenRequest{
		GrantType:          string(providers.GrantTypeTokenExchange),
		ClientID:           testClientID,
		SubjectToken:       "subject-token",
		SubjectTokenType:   string(constants.TokenTypeIdentifierAccessToken),
		RequestedTokenType: string(constants.TokenTypeIdentifierIDJAG),
		Audiences:          []string{"https://evil.example.com"},
	}

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorInvalidTarget, errResp.Error)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_IDJAG_MissingAudience() {
	suite.oauthApp.Token.IDJAG = &providers.IDJAGConfig{
		Enabled:          true,
		AllowedAudiences: []string{testIDJAGAudience},
	}
	tokenRequest := &model.TokenRequest{
		GrantType:          string(providers.GrantTypeTokenExchange),
		ClientID:           testClientID,
		SubjectToken:       "subject-token",
		SubjectTokenType:   string(constants.TokenTypeIdentifierAccessToken),
		RequestedTokenType: string(constants.TokenTypeIdentifierIDJAG),
	}

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorInvalidTarget, errResp.Error)
	assert.Contains(suite.T(), errResp.ErrorDescription, "audience parameter is required")
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_IDJAG_SubjectTokenValidationFailedRejected() {
	suite.oauthApp.Token.IDJAG = &providers.IDJAGConfig{
		Enabled:          true,
		AllowedAudiences: []string{testIDJAGAudience},
	}
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub": testUserID,
		"iss": testCustomIssuer,
		"exp": float64(now + 3600),
	})

	tokenRequest := &model.TokenRequest{
		GrantType:          string(providers.GrantTypeTokenExchange),
		ClientID:           testClientID,
		SubjectToken:       subjectToken,
		SubjectTokenType:   string(constants.TokenTypeIdentifierAccessToken),
		RequestedTokenType: string(constants.TokenTypeIdentifierIDJAG),
		Audiences:          []string{testIDJAGAudience},
	}

	// ValidateIDJAGSubjectToken rejects external-issuer (and other invalid) subject tokens itself; the
	// handler maps any validation failure to a single invalid_request response.
	suite.mockTokenValidator.On("ValidateIDJAGSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(nil, fmt.Errorf("subject_token must be issued by this server, got issuer %q", testCustomIssuer))

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, errResp.Error)
	assert.Contains(suite.T(), errResp.ErrorDescription, "subject_token must be an ID token issued to this client")
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_IDJAG_NoIDJAGConfigRejected() {
	// suite.oauthApp has no IDJAG config configured in SetupTest.
	tokenRequest := &model.TokenRequest{
		GrantType:          string(providers.GrantTypeTokenExchange),
		ClientID:           testClientID,
		SubjectToken:       "subject-token",
		SubjectTokenType:   string(constants.TokenTypeIdentifierAccessToken),
		RequestedTokenType: string(constants.TokenTypeIdentifierIDJAG),
		Audiences:          []string{testIDJAGAudience},
	}

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorInvalidTarget, errResp.Error)
	assert.Contains(suite.T(), errResp.ErrorDescription, "not permitted to request ID-JAGs")
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_IDJAG_DisabledRejected() {
	suite.oauthApp.Token.IDJAG = &providers.IDJAGConfig{
		Enabled:          false,
		AllowedAudiences: []string{testIDJAGAudience},
	}
	tokenRequest := &model.TokenRequest{
		GrantType:          string(providers.GrantTypeTokenExchange),
		ClientID:           testClientID,
		SubjectToken:       "subject-token",
		SubjectTokenType:   string(constants.TokenTypeIdentifierAccessToken),
		RequestedTokenType: string(constants.TokenTypeIdentifierIDJAG),
		Audiences:          []string{testIDJAGAudience},
	}

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorInvalidTarget, errResp.Error)
	assert.Contains(suite.T(), errResp.ErrorDescription, "not permitted to request ID-JAGs")
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_IDJAG_InvalidSubjectToken() {
	suite.oauthApp.Token.IDJAG = &providers.IDJAGConfig{
		Enabled:          true,
		AllowedAudiences: []string{testIDJAGAudience},
	}
	tokenRequest := &model.TokenRequest{
		GrantType:          string(providers.GrantTypeTokenExchange),
		ClientID:           testClientID,
		SubjectToken:       "subject-token",
		SubjectTokenType:   string(constants.TokenTypeIdentifierAccessToken),
		RequestedTokenType: string(constants.TokenTypeIdentifierIDJAG),
		Audiences:          []string{testIDJAGAudience},
	}

	suite.mockTokenValidator.On("ValidateIDJAGSubjectToken", mock.Anything, "subject-token", suite.oauthApp).
		Return(nil, errors.New("invalid signature"))

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, errResp.Error)
	assert.Contains(suite.T(), errResp.ErrorDescription,
		"subject_token must be an ID token issued to this client")
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_IDJAG_SubjectTokenEnforcementUnavailable() {
	suite.oauthApp.Token.IDJAG = &providers.IDJAGConfig{
		Enabled:          true,
		AllowedAudiences: []string{testIDJAGAudience},
	}
	tokenRequest := &model.TokenRequest{
		GrantType:          string(providers.GrantTypeTokenExchange),
		ClientID:           testClientID,
		SubjectToken:       "subject-token",
		SubjectTokenType:   string(constants.TokenTypeIdentifierAccessToken),
		RequestedTokenType: string(constants.TokenTypeIdentifierIDJAG),
		Audiences:          []string{testIDJAGAudience},
	}

	suite.mockTokenValidator.On("ValidateIDJAGSubjectToken", mock.Anything, "subject-token", suite.oauthApp).
		Return(nil, revocation.ErrEnforcementUnavailable)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorServerError, errResp.Error)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_IDJAG_BuildError() {
	suite.oauthApp.Token.IDJAG = &providers.IDJAGConfig{
		Enabled:          true,
		AllowedAudiences: []string{testIDJAGAudience},
	}
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub": testUserID,
		"iss": testIDJAGServerIssuer,
		"aud": testClientID,
		"exp": float64(time.Now().Unix() + 3600),
	})
	tokenRequest := &model.TokenRequest{
		GrantType:          string(providers.GrantTypeTokenExchange),
		ClientID:           testClientID,
		SubjectToken:       subjectToken,
		SubjectTokenType:   string(constants.TokenTypeIdentifierIDToken),
		RequestedTokenType: string(constants.TokenTypeIdentifierIDJAG),
		Audiences:          []string{testIDJAGAudience},
	}

	suite.mockTokenValidator.On("ValidateIDJAGSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub: testUserID,
			Iss: testIDJAGServerIssuer,
			Aud: []string{testClientID},
		}, nil)
	suite.mockTokenBuilder.On("BuildIDJAG", mock.Anything, mock.Anything).
		Return(nil, errors.New("signing failed"))

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorServerError, errResp.Error)
}

// A public (none-auth) client cannot request ID-JAGs; the grant requires a confidential client.
func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_IDJAG_PublicClientRejected() {
	suite.oauthApp.Token.IDJAG = &providers.IDJAGConfig{
		Enabled:          true,
		AllowedAudiences: []string{testIDJAGAudience},
	}
	suite.oauthApp.TokenEndpointAuthMethod = providers.TokenEndpointAuthMethodNone
	tokenRequest := &model.TokenRequest{
		GrantType:          string(providers.GrantTypeTokenExchange),
		ClientID:           testClientID,
		SubjectToken:       "subject-token",
		SubjectTokenType:   string(constants.TokenTypeIdentifierAccessToken),
		RequestedTokenType: string(constants.TokenTypeIdentifierIDJAG),
		Audiences:          []string{testIDJAGAudience},
	}

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorInvalidClient, errResp.Error)
	assert.Contains(suite.T(), errResp.ErrorDescription, "confidential client")
}

// An ID-JAG must target a single RS; supplying more than one audience is rejected as invalid_target.
func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_IDJAG_MultipleAudiencesRejected() {
	suite.oauthApp.Token.IDJAG = &providers.IDJAGConfig{
		Enabled:          true,
		AllowedAudiences: []string{testIDJAGAudience},
	}
	tokenRequest := &model.TokenRequest{
		GrantType:          string(providers.GrantTypeTokenExchange),
		ClientID:           testClientID,
		SubjectToken:       "subject-token",
		SubjectTokenType:   string(constants.TokenTypeIdentifierAccessToken),
		RequestedTokenType: string(constants.TokenTypeIdentifierIDJAG),
		Audiences:          []string{testIDJAGAudience, "https://rs2.example.com"},
	}

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorInvalidTarget, errResp.Error)
	assert.Contains(suite.T(), errResp.ErrorDescription, "Exactly one audience")
}

// The subject token must be bound to the authenticated client: its aud must contain the client_id.
func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_IDJAG_SubjectTokenAudMismatchRejected() {
	suite.oauthApp.Token.IDJAG = &providers.IDJAGConfig{
		Enabled:          true,
		AllowedAudiences: []string{testIDJAGAudience},
	}
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub": testUserID,
		"iss": testIDJAGServerIssuer,
		"aud": "another-client",
		"exp": float64(time.Now().Unix() + 3600),
	})
	tokenRequest := &model.TokenRequest{
		GrantType:          string(providers.GrantTypeTokenExchange),
		ClientID:           testClientID,
		SubjectToken:       subjectToken,
		SubjectTokenType:   string(constants.TokenTypeIdentifierIDToken),
		RequestedTokenType: string(constants.TokenTypeIdentifierIDJAG),
		Audiences:          []string{testIDJAGAudience},
	}

	suite.mockTokenValidator.On("ValidateIDJAGSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub: testUserID,
			Iss: testIDJAGServerIssuer,
			Aud: []string{"another-client"},
		}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, errResp.Error)
	assert.Contains(suite.T(), errResp.ErrorDescription,
		"subject_token audience does not match the authenticated client")
}

// An empty subject-token audience does not satisfy the client binding and is rejected.
func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_IDJAG_SubjectTokenEmptyAudRejected() {
	suite.oauthApp.Token.IDJAG = &providers.IDJAGConfig{
		Enabled:          true,
		AllowedAudiences: []string{testIDJAGAudience},
	}
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub": testUserID,
		"iss": testIDJAGServerIssuer,
		"exp": float64(time.Now().Unix() + 3600),
	})
	tokenRequest := &model.TokenRequest{
		GrantType:          string(providers.GrantTypeTokenExchange),
		ClientID:           testClientID,
		SubjectToken:       subjectToken,
		SubjectTokenType:   string(constants.TokenTypeIdentifierIDToken),
		RequestedTokenType: string(constants.TokenTypeIdentifierIDJAG),
		Audiences:          []string{testIDJAGAudience},
	}

	suite.mockTokenValidator.On("ValidateIDJAGSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub: testUserID,
			Iss: testIDJAGServerIssuer,
		}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, errResp.Error)
	assert.Contains(suite.T(), errResp.ErrorDescription,
		"subject_token audience does not match the authenticated client")
}

// A subject token whose multi-valued aud includes the authenticated client satisfies the binding.
func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_IDJAG_SubjectTokenMultiAudContainingClientAccepted() {
	suite.oauthApp.Token.IDJAG = &providers.IDJAGConfig{
		Enabled:          true,
		AllowedAudiences: []string{testIDJAGAudience},
	}
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub": testUserID,
		"iss": testIDJAGServerIssuer,
		"aud": []string{"another-client", testClientID},
		"exp": float64(now + 3600),
	})
	tokenRequest := &model.TokenRequest{
		GrantType:          string(providers.GrantTypeTokenExchange),
		ClientID:           testClientID,
		SubjectToken:       subjectToken,
		SubjectTokenType:   string(constants.TokenTypeIdentifierIDToken),
		RequestedTokenType: string(constants.TokenTypeIdentifierIDJAG),
		Audiences:          []string{testIDJAGAudience},
		Scope:              testScopeRead,
	}

	suite.mockTokenValidator.On("ValidateIDJAGSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub: testUserID,
			Iss: testIDJAGServerIssuer,
			Aud: []string{"another-client", testClientID},
		}, nil)
	suite.mockTokenBuilder.On("BuildIDJAG", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.IDJAGBuildContext) bool {
			return ctx.Subject == testUserID && ctx.Audience == testIDJAGAudience
		})).Return(&model.TokenDTO{
		Token:     "test-id-jag",
		TokenType: constants.TokenTypeNA,
		IssuedAt:  now,
		ExpiresIn: 300,
		Scopes:    []string{testScopeRead},
		ClientID:  testClientID,
		Subject:   testUserID,
		Audiences: []string{testIDJAGAudience},
	}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "test-id-jag", result.AccessToken.Token)
}

// All requested scopes are granted as-is; ID-JAG no longer applies a per-application scope allowlist.
func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_IDJAG_ScopesPassthrough() {
	suite.oauthApp.Token.IDJAG = &providers.IDJAGConfig{
		Enabled:          true,
		AllowedAudiences: []string{testIDJAGAudience},
	}
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub": testUserID,
		"iss": testIDJAGServerIssuer,
		"aud": testClientID,
		"exp": float64(now + 3600),
	})
	tokenRequest := &model.TokenRequest{
		GrantType:          string(providers.GrantTypeTokenExchange),
		ClientID:           testClientID,
		SubjectToken:       subjectToken,
		SubjectTokenType:   string(constants.TokenTypeIdentifierIDToken),
		RequestedTokenType: string(constants.TokenTypeIdentifierIDJAG),
		Audiences:          []string{testIDJAGAudience},
		Scope:              "read delete",
	}

	suite.mockTokenValidator.On("ValidateIDJAGSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub: testUserID,
			Iss: testIDJAGServerIssuer,
			Aud: []string{testClientID},
		}, nil)
	suite.mockTokenBuilder.On("BuildIDJAG", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.IDJAGBuildContext) bool {
			return tokenservice.JoinScopes(ctx.Scopes) == "read delete"
		})).Return(&model.TokenDTO{
		Token:     "test-id-jag",
		TokenType: constants.TokenTypeNA,
		IssuedAt:  now,
		ExpiresIn: 300,
		Scopes:    []string{"read", "delete"},
		ClientID:  testClientID,
		Subject:   testUserID,
		Audiences: []string{testIDJAGAudience},
	}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), []string{"read", "delete"}, result.AccessToken.Scopes)
}

func (suite *TokenExchangeGrantHandlerTestSuite) TestHandleGrant_OIDCOnly_NoResource_AudIsClientID() {
	// Only OIDC scopes and no resource: the exchanged token is not bound to a resource server, so
	// its audience is the client_id and it carries the OIDC scopes.
	now := time.Now().Unix()
	subjectToken := suite.createTestJWT(map[string]interface{}{
		"sub": testUserID,
		"iss": testCustomIssuer,
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
	})

	tokenRequest := suite.createBasicTokenRequest(subjectToken)

	suite.mockTokenValidator.On("ValidateSubjectToken", mock.Anything, subjectToken, suite.oauthApp).
		Return(&tokenservice.SubjectTokenClaims{
			Sub:            testUserID,
			Scopes:         []string{"openid"},
			UserAttributes: map[string]interface{}{},
		}, nil)
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return len(ctx.Audiences) == 1 && ctx.Audiences[0] == testClientID &&
				tokenservice.JoinScopes(ctx.Scopes) == "openid"
		})).Return(&model.TokenDTO{Token: testTokenExchangeJWT, IssuedAt: now, ExpiresIn: 7200}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
}
