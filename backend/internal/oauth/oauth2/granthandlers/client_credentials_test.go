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
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

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
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/tests/mocks/actorprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/authzmock"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/tokenservicemock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
	"github.com/thunder-id/thunderid/tests/mocks/resourcemock"
	"github.com/thunder-id/thunderid/tests/mocks/serverconfigmock"
)

// nolint:gosec // Test token, not a real credential
const testJWTToken = "test-jwt-token-123"
const testResourceURL = "https://mcp.example.com/mcp"
const testEntityID = "agent-entity-123"

// Default target resource server resolved for requests without a resource parameter.
const defaultRSID = "rs-1"
const defaultRSIdentifier = "https://api.example.com"

type ClientCredentialsGrantHandlerTestSuite struct {
	suite.Suite
	mockJWTService      *jwtmock.JWTServiceInterfaceMock
	mockTokenBuilder    *tokenservicemock.TokenBuilderInterfaceMock
	mockOUService       *oumock.OrganizationUnitServiceInterfaceMock
	mockAuthzService    *authzmock.AuthorizationProviderMock
	mockEntityProvider  *actorprovidermock.ActorProviderMock
	mockResourceService *resourcemock.ResourceServiceInterfaceMock
	mockServerConfig    *serverconfigmock.ServerConfigServiceMock
	handler             *clientCredentialsGrantHandler
	oauthApp            *providers.OAuthClient
}

func TestClientCredentialsGrantHandlerSuite(t *testing.T) {
	suite.Run(t, new(ClientCredentialsGrantHandlerTestSuite))
}

func (suite *ClientCredentialsGrantHandlerTestSuite) SetupTest() {
	// Initialize Runtime for tests
	testConfig := &config.Config{
		JWT: engineconfig.JWTConfig{
			Issuer:         "https://auth.example.com",
			ValidityPeriod: 3600,
		},
	}
	err := config.InitializeServerRuntime("", testConfig)
	assert.NoError(suite.T(), err)

	suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())
	suite.mockTokenBuilder = tokenservicemock.NewTokenBuilderInterfaceMock(suite.T())
	suite.mockOUService = oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())
	suite.mockAuthzService = authzmock.NewAuthorizationProviderMock(suite.T())
	suite.mockEntityProvider = actorprovidermock.NewActorProviderMock(suite.T())
	suite.mockResourceService = resourcemock.NewResourceServiceInterfaceMock(suite.T())
	suite.mockServerConfig = serverconfigmock.NewServerConfigServiceMock(suite.T())

	// Explicit resource: resolve the identifier to an RS whose ID and Identifier are the identifier.
	suite.mockResourceService.On("GetResourceServerByIdentifier", mock.Anything, mock.Anything).
		Return(func(_ context.Context, identifier string) *providers.ResourceServer {
			return &providers.ResourceServer{ID: identifier, Identifier: identifier}
		}, func(_ context.Context, _ string) *tidcommon.ServiceError {
			return nil
		}).Maybe()
	// No resource: fall back to the configured default resource server.
	suite.mockServerConfig.On("GetMergedConfig", mock.Anything, "defaultResourceServer").
		Return(resource.DefaultResourceServerConfig{ResourceServerID: defaultRSID}, nil).Maybe()
	suite.mockResourceService.On("GetResourceServer", mock.Anything, defaultRSID).
		Return(&providers.ResourceServer{ID: defaultRSID, Identifier: defaultRSIdentifier}, nil).Maybe()
	suite.mockResourceService.On("ValidatePermissions", mock.Anything, mock.Anything, mock.Anything).
		Return([]string{}, nil).Maybe()

	suite.handler = &clientCredentialsGrantHandler{
		tokenBuilder:        suite.mockTokenBuilder,
		ouService:           suite.mockOUService,
		authzService:        suite.mockAuthzService,
		actorProvider:       suite.mockEntityProvider,
		resourceService:     suite.mockResourceService,
		serverConfigService: suite.mockServerConfig,
	}
	suite.mockEntityProvider.On("GetActorGroups", mock.Anything).
		Return([]providers.EntityGroup{}, nil).Maybe()

	suite.oauthApp = &providers.OAuthClient{
		ID:                      testEntityID,
		ClientID:                testClientID,
		RedirectURIs:            []string{"https://example.com/callback"},
		GrantTypes:              []providers.GrantType{providers.GrantTypeClientCredentials},
		ResponseTypes:           []providers.ResponseType{providers.ResponseTypeCode},
		TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
	}
}

// mockEvaluateAccessBatch stubs EvaluateAccessBatch, asserting each evaluation targets the given
// resource server ID and returning the given authorized scopes as allowed.
func mockEvaluateAccessBatch(
	authzService *authzmock.AuthorizationProviderMock,
	entityID string,
	resourceServerID string,
	requestedScopes []string,
	authorizedScopes []string,
) {
	authorizedScopeSet := make(map[string]bool, len(authorizedScopes))
	for _, scope := range authorizedScopes {
		authorizedScopeSet[scope] = true
	}

	evaluations := make([]providers.AccessEvaluationResponse, 0, len(requestedScopes))
	for _, scope := range requestedScopes {
		evaluations = append(evaluations, providers.AccessEvaluationResponse{
			Decision: authorizedScopeSet[scope],
		})
	}

	authzService.On("EvaluateAccessBatch", mock.Anything,
		mock.MatchedBy(func(req providers.AccessEvaluationsRequest) bool {
			if len(req.Evaluations) != len(requestedScopes) {
				return false
			}
			for i, scope := range requestedScopes {
				evaluation := req.Evaluations[i]
				if evaluation.Subject.ID != entityID ||
					len(evaluation.Subject.GroupIDs) != 0 ||
					evaluation.ResourceServer.ID != resourceServerID ||
					evaluation.Permission.Name != scope {
					return false
				}
			}
			return true
		})).
		Return(&providers.AccessEvaluationsResponse{Evaluations: evaluations}, nil)
}

func (suite *ClientCredentialsGrantHandlerTestSuite) TestNewClientCredentialsGrantHandler() {
	handler := newClientCredentialsGrantHandler(
		suite.mockTokenBuilder, suite.mockOUService, suite.mockAuthzService,
		suite.mockEntityProvider, suite.mockResourceService, suite.mockServerConfig)
	assert.NotNil(suite.T(), handler)
	assert.Implements(suite.T(), (*GrantHandlerInterface)(nil), handler)
}

func (suite *ClientCredentialsGrantHandlerTestSuite) TestValidateGrant_Success() {
	tokenRequest := &model.TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     testClientID,
		ClientSecret: "secret123",
		Scope:        "read",
	}

	result := suite.handler.ValidateGrant(context.Background(), tokenRequest, suite.oauthApp)
	assert.Nil(suite.T(), result)
}

func (suite *ClientCredentialsGrantHandlerTestSuite) TestValidateGrant_WrongGrantType() {
	tokenRequest := &model.TokenRequest{
		GrantType:    "authorization_code",
		ClientID:     testClientID,
		ClientSecret: "secret123",
	}

	result := suite.handler.ValidateGrant(context.Background(), tokenRequest, suite.oauthApp)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), constants.ErrorUnsupportedGrantType, result.Error)
	assert.Equal(suite.T(), "Unsupported grant type", result.ErrorDescription)
}

func (suite *ClientCredentialsGrantHandlerTestSuite) TestHandleGrant_Success() {
	testCases := []struct {
		name              string
		scope             string
		expectedJWTClaims map[string]interface{}
		expectedScopes    []string
		expectedAudience  string
	}{
		{
			name:              "WithValidScope",
			scope:             "read write",
			expectedJWTClaims: map[string]interface{}{"scope": "read write"},
			expectedScopes:    []string{"read", "write"},
			expectedAudience:  defaultRSIdentifier,
		},
		{
			// No scopes and no resource: not bound to a resource server, so aud is the client_id.
			name:              "WithoutScope",
			scope:             "",
			expectedJWTClaims: map[string]interface{}{},
			expectedScopes:    []string{},
			expectedAudience:  testClientID,
		},
		{
			name:              "WithWhitespaceScope",
			scope:             "   ",
			expectedJWTClaims: map[string]interface{}{},
			expectedScopes:    []string{},
			expectedAudience:  testClientID,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			// Reset mocks for each test case
			suite.mockJWTService.Mock = mock.Mock{}
			suite.mockAuthzService.Mock = mock.Mock{}

			tokenRequest := &model.TokenRequest{
				GrantType:    "client_credentials",
				ClientID:     testClientID,
				ClientSecret: "secret123",
				Scope:        tc.scope,
			}

			// Mock authz service for non-OIDC scopes. No resource param -> default RS.
			if len(tc.expectedScopes) > 0 {
				mockEvaluateAccessBatch(suite.mockAuthzService, suite.oauthApp.ID, defaultRSID,
					tc.expectedScopes, tc.expectedScopes)
			}

			expectedToken := testJWTToken
			suite.mockTokenBuilder.On("BuildAccessToken",
				mock.Anything,
				mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
					return ctx.Subject == testEntityID &&
						(len(ctx.Audiences) == 1 && ctx.Audiences[0] == tc.expectedAudience) &&
						ctx.ClientID == testClientID &&
						tokenservice.JoinScopes(ctx.Scopes) == tokenservice.JoinScopes(tc.expectedScopes)
				})).Return(&model.TokenDTO{
				Token:     expectedToken,
				TokenType: constants.TokenTypeBearer,
				IssuedAt:  int64(1234567890),
				ExpiresIn: 3600,
				Scopes:    tc.expectedScopes,
				ClientID:  testClientID,
				Subject:   testEntityID,
				Audiences: []string{tc.expectedAudience},
			}, nil)

			result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

			assert.Nil(t, errResp)
			assert.NotNil(t, result)
			assert.Equal(t, expectedToken, result.AccessToken.Token)
			assert.Equal(t, constants.TokenTypeBearer, result.AccessToken.TokenType)
			assert.Equal(t, int64(3600), result.AccessToken.ExpiresIn)
			assert.Equal(t, tc.expectedScopes, result.AccessToken.Scopes)
			assert.Equal(t, testClientID, result.AccessToken.ClientID)

			// The sub claim must be the resource entity ID, not the OAuth client_id.
			assert.Equal(t, testEntityID, result.AccessToken.Subject)
			assert.NotEqual(t, result.AccessToken.ClientID, result.AccessToken.Subject)
			// The token is bound to a single audience: the target resource server, or the client_id
			// when there are no scopes to bind to a resource server.
			assert.Equal(t, []string{tc.expectedAudience}, result.AccessToken.Audiences)

			suite.mockTokenBuilder.AssertExpectations(t)
		})
	}
}

func (suite *ClientCredentialsGrantHandlerTestSuite) TestHandleGrant_JWTGenerationError() {
	tokenRequest := &model.TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     testClientID,
		ClientSecret: "secret123",
		Scope:        "read",
	}

	mockEvaluateAccessBatch(suite.mockAuthzService, suite.oauthApp.ID, defaultRSID, []string{"read"}, []string{"read"})

	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything, mock.Anything).
		Return(nil, errors.New("JWT generation failed"))

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorServerError, errResp.Error)
	assert.Equal(suite.T(), "Failed to generate token", errResp.ErrorDescription)

	suite.mockTokenBuilder.AssertExpectations(suite.T())
}

func (suite *ClientCredentialsGrantHandlerTestSuite) TestHandleGrant_NilTokenAttributes() {
	tokenRequest := &model.TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     testClientID,
		ClientSecret: "secret123",
		Scope:        "read",
	}

	mockEvaluateAccessBatch(suite.mockAuthzService, suite.oauthApp.ID, defaultRSID, []string{"read"}, []string{"read"})

	expectedToken := testJWTToken
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return ctx.Subject == testEntityID &&
				(len(ctx.Audiences) == 1 && ctx.Audiences[0] == defaultRSIdentifier) &&
				tokenservice.JoinScopes(ctx.Scopes) == testScopeRead
		})).Return(&model.TokenDTO{
		Token:     expectedToken,
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  int64(1234567890),
		ExpiresIn: 3600,
		Scopes:    []string{"read"},
		ClientID:  "client123",
		Subject:   testEntityID,
		Audiences: []string{defaultRSIdentifier},
	}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), expectedToken, result.AccessToken.Token)

	// The sub claim must be the resource entity ID, not the OAuth client_id.
	assert.Equal(suite.T(), testEntityID, result.AccessToken.Subject)
	assert.Equal(suite.T(), []string{defaultRSIdentifier}, result.AccessToken.Audiences)

	suite.mockTokenBuilder.AssertExpectations(suite.T())
}

func (suite *ClientCredentialsGrantHandlerTestSuite) TestHandleGrant_TokenTimingValidation() {
	tokenRequest := &model.TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     testClientID,
		ClientSecret: "secret123",
		Scope:        "read",
	}

	mockEvaluateAccessBatch(suite.mockAuthzService, suite.oauthApp.ID, defaultRSID, []string{"read"}, []string{"read"})

	expectedToken := testJWTToken
	now := time.Now().Unix()
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything, mock.Anything).
		Return(&model.TokenDTO{
			Token:     expectedToken,
			TokenType: constants.TokenTypeBearer,
			IssuedAt:  now,
			ExpiresIn: 3600,
			Scopes:    []string{"read"},
			ClientID:  testClientID,
		}, nil)

	startTime := time.Now().Unix()
	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)
	endTime := time.Now().Unix()

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)

	// Verify the issued time is within reasonable bounds
	assert.GreaterOrEqual(suite.T(), result.AccessToken.IssuedAt, startTime)
	assert.LessOrEqual(suite.T(), result.AccessToken.IssuedAt, endTime)

	suite.mockTokenBuilder.AssertExpectations(suite.T())
}

func (suite *ClientCredentialsGrantHandlerTestSuite) TestHandleGrant_ClientAttributeError() {
	tokenRequest := &model.TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     testClientID,
		ClientSecret: "secret123",
		Scope:        "read",
	}

	oauthAppWithOU := &providers.OAuthClient{
		ID:                      "app123",
		ClientID:                testClientID,
		OUID:                    "ou-456",
		GrantTypes:              []providers.GrantType{providers.GrantTypeClientCredentials},
		TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
	}

	mockEvaluateAccessBatch(suite.mockAuthzService, oauthAppWithOU.ID, defaultRSID, []string{"read"}, []string{"read"})

	suite.mockOUService.On("GetOrganizationUnit", context.Background(), "ou-456").Return(
		providers.OrganizationUnit{},
		&tidcommon.ServiceError{
			Code:  "OU-0001",
			Error: tidcommon.I18nMessage{Key: "error.test.not_found", DefaultValue: "not found"},
		},
	)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, oauthAppWithOU)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorServerError, errResp.Error)
	assert.Equal(suite.T(), "Failed to generate token", errResp.ErrorDescription)
}

// ResourceServer Parameter Tests (RFC 8707) for Client Credentials Grant

func (suite *ClientCredentialsGrantHandlerTestSuite) TestHandleGrant_WithResourceParameter() {
	tokenRequest := &model.TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     testClientID,
		ClientSecret: "secret123",
		Scope:        "read",
		Resources:    []string{testResourceURL},
	}

	// The explicit resource resolves to an RS whose ID and Identifier equal the resource URL;
	// authz must be evaluated against that RS ID.
	mockEvaluateAccessBatch(suite.mockAuthzService, suite.oauthApp.ID, testResourceURL,
		[]string{"read"}, []string{"read"})

	var capturedAudiences []string
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			capturedAudiences = ctx.Audiences
			return ctx.Subject == testEntityID &&
				len(ctx.Audiences) == 1 &&
				ctx.Audiences[0] == testResourceURL
		})).Return(&model.TokenDTO{
		Token:     testJWTToken,
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  int64(1234567890),
		ExpiresIn: 3600,
		Scopes:    []string{"read"},
		ClientID:  "client123",
		Subject:   testEntityID,
		Audiences: []string{testResourceURL},
	}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)

	// aud contains only the resolved RS identifier (single-audience binding).
	assert.Equal(suite.T(), []string{testResourceURL}, capturedAudiences)
}

func (suite *ClientCredentialsGrantHandlerTestSuite) TestHandleGrant_WithoutResourceParameter() {
	tokenRequest := &model.TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     testClientID,
		ClientSecret: "secret123",
		Scope:        "read",
	}

	// No resource param -> default RS resolved via server-config; authz targets the default RS ID.
	mockEvaluateAccessBatch(suite.mockAuthzService, suite.oauthApp.ID, defaultRSID, []string{"read"}, []string{"read"})

	var capturedAudiences []string
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			capturedAudiences = ctx.Audiences
			return ctx.Subject == testEntityID &&
				len(ctx.Audiences) == 1 && ctx.Audiences[0] == defaultRSIdentifier
		})).Return(&model.TokenDTO{
		Token:     testJWTToken,
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  int64(1234567890),
		ExpiresIn: 3600,
		Scopes:    []string{"read"},
		ClientID:  "client123",
		Subject:   testEntityID,
		Audiences: []string{defaultRSIdentifier},
	}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)

	// With no resource param the token is bound to the configured default resource server.
	assert.Equal(suite.T(), []string{defaultRSIdentifier}, capturedAudiences)
	assert.Equal(suite.T(), []string{defaultRSIdentifier}, result.AccessToken.Audiences)
}

// App Authorization Integration Tests — verify scope filtering via RBAC roles

func (suite *ClientCredentialsGrantHandlerTestSuite) TestHandleGrant_PartialScopeAuthorization() {
	tokenRequest := &model.TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     testClientID,
		ClientSecret: "secret123",
		Scope:        "read write delete",
	}

	// App is only authorized for "read" and "write" via its role assignments.
	mockEvaluateAccessBatch(suite.mockAuthzService, suite.oauthApp.ID, defaultRSID,
		[]string{"read", "write", "delete"}, []string{"read", "write"})

	suite.mockTokenBuilder.On("BuildAccessToken",
		mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return tokenservice.JoinScopes(ctx.Scopes) == tokenservice.JoinScopes([]string{"read", "write"})
		})).Return(&model.TokenDTO{
		Token:     testJWTToken,
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  int64(1234567890),
		ExpiresIn: 3600,
		Scopes:    []string{"read", "write"},
		ClientID:  testClientID,
	}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), []string{"read", "write"}, result.AccessToken.Scopes)
}

func (suite *ClientCredentialsGrantHandlerTestSuite) TestHandleGrant_NoAuthorizedScopes() {
	tokenRequest := &model.TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     testClientID,
		ClientSecret: "secret123",
		Scope:        "admin:full",
	}

	// App has no role granting "admin:full".
	mockEvaluateAccessBatch(suite.mockAuthzService, suite.oauthApp.ID, defaultRSID,
		[]string{"admin:full"}, []string{})

	suite.mockTokenBuilder.On("BuildAccessToken",
		mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return len(ctx.Scopes) == 0
		})).Return(&model.TokenDTO{
		Token:     testJWTToken,
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  int64(1234567890),
		ExpiresIn: 3600,
		Scopes:    []string{},
		ClientID:  testClientID,
	}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
	assert.Empty(suite.T(), result.AccessToken.Scopes)
}

func (suite *ClientCredentialsGrantHandlerTestSuite) TestHandleGrant_AuthzServiceError() {
	tokenRequest := &model.TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     testClientID,
		ClientSecret: "secret123",
		Scope:        "read",
	}

	suite.mockAuthzService.On("EvaluateAccessBatch", mock.Anything, mock.Anything).
		Return((*providers.AccessEvaluationsResponse)(nil),
			&tidcommon.ServiceError{
				Code: "AUTHZ-0001",
				Error: tidcommon.I18nMessage{
					Key: "error.test.authorization_check_failed", DefaultValue: "authorization check failed",
				},
			})

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorServerError, errResp.Error)
}

func (suite *ClientCredentialsGrantHandlerTestSuite) TestHandleGrant_EmptyScope_SkipsAuthzCall() {
	tokenRequest := &model.TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     testClientID,
		ClientSecret: "secret123",
		Scope:        "",
	}

	suite.mockTokenBuilder.On("BuildAccessToken",
		mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return len(ctx.Scopes) == 0
		})).Return(&model.TokenDTO{
		Token:     testJWTToken,
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  int64(1234567890),
		ExpiresIn: 3600,
		Scopes:    []string{},
		ClientID:  testClientID,
	}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
	// Verify authz service was NOT called when no scopes requested.
	suite.mockAuthzService.AssertNotCalled(suite.T(), "EvaluateAccessBatch", mock.Anything, mock.Anything)
}

func (suite *ClientCredentialsGrantHandlerTestSuite) TestHandleGrant_AgentOwnAttributes_EmbeddedInClientAttributes() {
	tokenRequest := &model.TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     testClientID,
		ClientSecret: "secret123",
		Scope:        "",
	}

	agentApp := &providers.OAuthClient{
		ID:                      testEntityID,
		ClientID:                testClientID,
		EntityCategory:          providers.EntityCategoryAgent,
		GrantTypes:              []providers.GrantType{providers.GrantTypeClientCredentials},
		TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
		Token: &providers.OAuthTokenConfig{
			AccessToken: &providers.AccessTokenConfig{
				ClientConfig: &providers.AccessTokenSubConfig{Attributes: []string{"modelProvider"}},
			},
		},
	}

	suite.mockEntityProvider.On("GetActor", agentApp.ID).Return(&providers.Entity{
		ID:         agentApp.ID,
		Attributes: []byte(`{"modelProvider":"anthropic"}`),
	}, (*tidcommon.ServiceError)(nil))

	suite.mockTokenBuilder.On("BuildAccessToken",
		mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return ctx.SubjectAttributes["modelProvider"] == "anthropic"
		})).Return(&model.TokenDTO{
		Token:     testJWTToken,
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  int64(1234567890),
		ExpiresIn: 3600,
		Scopes:    []string{},
		ClientID:  testClientID,
	}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, agentApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
}

// Single-audience token binding tests (RFC 8707 + single-RS resolution).

// TestHandleGrant_ExplicitResource_PopulatesRSIDAndAudience verifies that an explicit resource
// binds both the authz evaluation (ResourceServer.ID) and the token audience to the resolved RS.
func (suite *ClientCredentialsGrantHandlerTestSuite) TestHandleGrant_ExplicitResource_PopulatesRSIDAndAudience() {
	const rsIdentifier = "https://rs01.example.com"

	tokenRequest := &model.TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     testClientID,
		ClientSecret: "secret123",
		Scope:        "r1:s1",
		Resources:    []string{rsIdentifier},
	}

	// The evaluation carries the resolved RS ID (equal to the identifier per the default stub).
	mockEvaluateAccessBatch(suite.mockAuthzService, suite.oauthApp.ID, rsIdentifier,
		[]string{"r1:s1"}, []string{"r1:s1"})

	var capturedAudiences []string
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			capturedAudiences = ctx.Audiences
			return true
		})).Return(&model.TokenDTO{
		Token:     testJWTToken,
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  int64(1234567890),
		ExpiresIn: 3600,
		Scopes:    []string{"r1:s1"},
		ClientID:  testClientID,
		Subject:   testEntityID,
		Audiences: []string{rsIdentifier},
	}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), []string{rsIdentifier}, capturedAudiences)
	suite.mockAuthzService.AssertExpectations(suite.T())
}

// TestHandleGrant_CollidingPermission_NotAuthorizedOnOtherRS verifies that when the same permission
// string is defined on RS-A and RS-B but the app is granted it only on RS-A, requesting RS-B does
// NOT authorize the colliding permission — the authz evaluation is scoped to RS-B's ID and returns
// Decision:false, so the scope is dropped.
func (suite *ClientCredentialsGrantHandlerTestSuite) TestHandleGrant_CollidingPermission_NotAuthorizedOnOtherRS() {
	const rsBIdentifier = "https://rs-b.example.com"

	tokenRequest := &model.TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     testClientID,
		ClientSecret: "secret123",
		Scope:        "shared:read",
		Resources:    []string{rsBIdentifier},
	}

	// "shared:read" is a valid permission on RS-B (ValidatePermissions returns no invalid),
	// but the app is not granted it there — the evaluation scoped to RS-B returns Decision:false.
	mockEvaluateAccessBatch(suite.mockAuthzService, suite.oauthApp.ID, rsBIdentifier,
		[]string{"shared:read"}, []string{})

	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return len(ctx.Scopes) == 0 &&
				len(ctx.Audiences) == 1 && ctx.Audiences[0] == rsBIdentifier
		})).Return(&model.TokenDTO{
		Token:     testJWTToken,
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  int64(1234567890),
		ExpiresIn: 3600,
		Scopes:    []string{},
		ClientID:  testClientID,
		Subject:   testEntityID,
		Audiences: []string{rsBIdentifier},
	}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
	// The colliding permission is not granted on RS-B, so it is dropped from the token.
	assert.Empty(suite.T(), result.AccessToken.Scopes)
	suite.mockAuthzService.AssertExpectations(suite.T())
}

// TestHandleGrant_NoResourceNoDefault_InvalidTarget verifies that when no resource parameter is
// supplied and no default resource server is configured, HandleGrant rejects with invalid_target.
func (suite *ClientCredentialsGrantHandlerTestSuite) TestHandleGrant_NoResourceNoDefault_InvalidTarget() {
	mockTokenBuilder := tokenservicemock.NewTokenBuilderInterfaceMock(suite.T())
	mockAuthzService := authzmock.NewAuthorizationProviderMock(suite.T())
	mockResourceService := resourcemock.NewResourceServiceInterfaceMock(suite.T())
	mockEntityProvider := actorprovidermock.NewActorProviderMock(suite.T())
	mockServerConfig := serverconfigmock.NewServerConfigServiceMock(suite.T())

	handler := &clientCredentialsGrantHandler{
		tokenBuilder:        mockTokenBuilder,
		ouService:           suite.mockOUService,
		authzService:        mockAuthzService,
		actorProvider:       mockEntityProvider,
		resourceService:     mockResourceService,
		serverConfigService: mockServerConfig,
	}

	// No default resource server configured.
	mockServerConfig.On("GetMergedConfig", mock.Anything, "defaultResourceServer").
		Return(resource.DefaultResourceServerConfig{}, nil)

	tokenRequest := &model.TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     testClientID,
		ClientSecret: "secret123",
		Scope:        "read",
	}

	result, errResp := handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorInvalidTarget, errResp.Error)
}

func (suite *ClientCredentialsGrantHandlerTestSuite) TestHandleGrant_DPoPProof_PropagatesJktToBuilder() {
	tokenRequest := &model.TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     testClientID,
		ClientSecret: "secret123",
		Scope:        "read",
	}

	mockEvaluateAccessBatch(suite.mockAuthzService, suite.oauthApp.ID, defaultRSID, []string{"read"}, []string{"read"})

	suite.mockTokenBuilder.On("BuildAccessToken",
		mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return ctx.DPoPJkt == "thumbprint-cc"
		})).Return(&model.TokenDTO{
		Token:     testJWTToken,
		TokenType: constants.TokenTypeDPoP,
		IssuedAt:  int64(1234567890),
		ExpiresIn: 3600,
		Scopes:    []string{"read"},
		ClientID:  testClientID,
	}, nil)

	ctx := dpop.WithJkt(context.Background(), "thumbprint-cc")
	result, errResp := suite.handler.HandleGrant(ctx, tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), constants.TokenTypeDPoP, result.AccessToken.TokenType)
}

func (suite *ClientCredentialsGrantHandlerTestSuite) TestHandleGrant_NoDPoPProof_EmptyJkt() {
	tokenRequest := &model.TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     testClientID,
		ClientSecret: "secret123",
		Scope:        "read",
	}

	mockEvaluateAccessBatch(suite.mockAuthzService, suite.oauthApp.ID, defaultRSID, []string{"read"}, []string{"read"})

	suite.mockTokenBuilder.On("BuildAccessToken",
		mock.Anything,
		mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return ctx.DPoPJkt == ""
		})).Return(&model.TokenDTO{
		Token:     testJWTToken,
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  int64(1234567890),
		ExpiresIn: 3600,
		Scopes:    []string{"read"},
		ClientID:  testClientID,
	}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), constants.TokenTypeBearer, result.AccessToken.TokenType)
}
