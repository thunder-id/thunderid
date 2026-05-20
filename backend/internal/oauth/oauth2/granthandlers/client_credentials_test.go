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
	"github.com/thunder-id/thunderid/internal/system/i18n/core"

	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/authz"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/tests/mocks/authzmock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/tokenservicemock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
	"github.com/thunder-id/thunderid/tests/mocks/resourcemock"
)

// nolint:gosec // Test token, not a real credential
const testJWTToken = "test-jwt-token-123"
const testResourceURL = "https://mcp.example.com/mcp"

type ClientCredentialsGrantHandlerTestSuite struct {
	suite.Suite
	mockJWTService      *jwtmock.JWTServiceInterfaceMock
	mockTokenBuilder    *tokenservicemock.TokenBuilderInterfaceMock
	mockOUService       *oumock.OrganizationUnitServiceInterfaceMock
	mockAuthzService    *authzmock.AuthorizationServiceInterfaceMock
	mockEntityProv      *entityprovidermock.EntityProviderInterfaceMock
	mockResourceService *resourcemock.ResourceServiceInterfaceMock
	handler             *clientCredentialsGrantHandler
	oauthApp            *inboundmodel.OAuthClient
}

func TestClientCredentialsGrantHandlerSuite(t *testing.T) {
	suite.Run(t, new(ClientCredentialsGrantHandlerTestSuite))
}

func (suite *ClientCredentialsGrantHandlerTestSuite) SetupTest() {
	// Initialize Runtime for tests
	testConfig := &config.Config{
		JWT: config.JWTConfig{
			Issuer:         "https://auth.example.com",
			ValidityPeriod: 3600,
		},
	}
	err := config.InitializeServerRuntime("", testConfig)
	assert.NoError(suite.T(), err)

	suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())
	suite.mockTokenBuilder = tokenservicemock.NewTokenBuilderInterfaceMock(suite.T())
	suite.mockOUService = oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())
	suite.mockAuthzService = authzmock.NewAuthorizationServiceInterfaceMock(suite.T())
	suite.mockEntityProv = entityprovidermock.NewEntityProviderInterfaceMock(suite.T())
	suite.mockResourceService = resourcemock.NewResourceServiceInterfaceMock(suite.T())
	suite.mockResourceService.On("GetResourceServerByIdentifier", mock.Anything, mock.Anything).
		Return(func(_ context.Context, identifier string) *resource.ResourceServer {
			return &resource.ResourceServer{ID: identifier, Identifier: identifier}
		}, func(_ context.Context, _ string) *serviceerror.ServiceError {
			return nil
		}).Maybe()
	suite.mockResourceService.On("ValidatePermissions", mock.Anything, mock.Anything, mock.Anything).
		Return([]string{}, nil).Maybe()
	suite.mockResourceService.On("FindResourceServersByPermissions", mock.Anything, mock.Anything).
		Return([]resource.ResourceServer{}, nil).Maybe()

	suite.handler = &clientCredentialsGrantHandler{
		tokenBuilder:    suite.mockTokenBuilder,
		ouService:       suite.mockOUService,
		authzService:    suite.mockAuthzService,
		entityProv:      suite.mockEntityProv,
		resourceService: suite.mockResourceService,
	}
	suite.mockEntityProv.On("GetTransitiveEntityGroups", mock.Anything).
		Return([]entityprovider.EntityGroup{}, nil).Maybe()

	suite.oauthApp = &inboundmodel.OAuthClient{
		ID:                      "app123",
		ClientID:                testClientID,
		RedirectURIs:            []string{"https://example.com/callback"},
		GrantTypes:              []constants.GrantType{constants.GrantTypeClientCredentials},
		ResponseTypes:           []constants.ResponseType{constants.ResponseTypeCode},
		TokenEndpointAuthMethod: constants.TokenEndpointAuthMethodClientSecretBasic,
	}
}

func (suite *ClientCredentialsGrantHandlerTestSuite) TestNewClientCredentialsGrantHandler() {
	handler := newClientCredentialsGrantHandler(
		suite.mockTokenBuilder, suite.mockOUService, suite.mockAuthzService,
		suite.mockEntityProv, suite.mockResourceService)
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
	}{
		{
			name:              "WithValidScope",
			scope:             "read write",
			expectedJWTClaims: map[string]interface{}{"scope": "read write"},
			expectedScopes:    []string{"read", "write"},
		},
		{
			name:              "WithoutScope",
			scope:             "",
			expectedJWTClaims: map[string]interface{}{},
			expectedScopes:    []string{},
		},
		{
			name:              "WithWhitespaceScope",
			scope:             "   ",
			expectedJWTClaims: map[string]interface{}{},
			expectedScopes:    []string{},
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

			// Mock authz service for non-OIDC scopes
			if len(tc.expectedScopes) > 0 {
				suite.mockAuthzService.On("GetAuthorizedPermissions", mock.Anything,
					authz.GetAuthorizedPermissionsRequest{
						EntityID:             suite.oauthApp.ID,
						RequestedPermissions: tc.expectedScopes,
					}).Return(&authz.GetAuthorizedPermissionsResponse{
					AuthorizedPermissions: tc.expectedScopes,
				}, nil)
			}

			expectedToken := testJWTToken
			suite.mockTokenBuilder.On("BuildAccessToken",
				mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
					return ctx.Subject == testClientID &&
						(len(ctx.Audiences) > 0 && ctx.Audiences[0] == testClientID) &&
						ctx.ClientID == testClientID &&
						tokenservice.JoinScopes(ctx.Scopes) == tokenservice.JoinScopes(tc.expectedScopes)
				})).Return(&model.TokenDTO{
				Token:     expectedToken,
				TokenType: constants.TokenTypeBearer,
				IssuedAt:  int64(1234567890),
				ExpiresIn: 3600,
				Scopes:    tc.expectedScopes,
				ClientID:  testClientID,
				Subject:   testClientID,
				Audiences: []string{testClientID},
			}, nil)

			result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

			assert.Nil(t, errResp)
			assert.NotNil(t, result)
			assert.Equal(t, expectedToken, result.AccessToken.Token)
			assert.Equal(t, constants.TokenTypeBearer, result.AccessToken.TokenType)
			assert.Equal(t, int64(3600), result.AccessToken.ExpiresIn)
			assert.Equal(t, tc.expectedScopes, result.AccessToken.Scopes)
			assert.Equal(t, testClientID, result.AccessToken.ClientID)

			// Verify token attributes
			assert.Equal(t, testClientID, result.AccessToken.Subject)
			assert.Contains(t, result.AccessToken.Audiences, testClientID)

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

	suite.mockAuthzService.On("GetAuthorizedPermissions", mock.Anything,
		authz.GetAuthorizedPermissionsRequest{
			EntityID:             suite.oauthApp.ID,
			RequestedPermissions: []string{"read"},
		}).Return(&authz.GetAuthorizedPermissionsResponse{
		AuthorizedPermissions: []string{"read"},
	}, nil)

	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything).
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

	suite.mockAuthzService.On("GetAuthorizedPermissions", mock.Anything,
		authz.GetAuthorizedPermissionsRequest{
			EntityID:             suite.oauthApp.ID,
			RequestedPermissions: []string{"read"},
		}).Return(&authz.GetAuthorizedPermissionsResponse{
		AuthorizedPermissions: []string{"read"},
	}, nil)

	expectedToken := testJWTToken
	suite.mockTokenBuilder.On("BuildAccessToken", mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
		return ctx.Subject == testClientID && (len(ctx.Audiences) > 0 && ctx.Audiences[0] == testClientID) &&
			tokenservice.JoinScopes(ctx.Scopes) == testScopeRead
	})).Return(&model.TokenDTO{
		Token:     expectedToken,
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  int64(1234567890),
		ExpiresIn: 3600,
		Scopes:    []string{"read"},
		ClientID:  "client123",
		Subject:   testClientID,
		Audiences: []string{testClientID},
	}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), expectedToken, result.AccessToken.Token)

	// Verify token attributes
	assert.Equal(suite.T(), testClientID, result.AccessToken.Subject)
	assert.Contains(suite.T(), result.AccessToken.Audiences, testClientID)

	suite.mockTokenBuilder.AssertExpectations(suite.T())
}

func (suite *ClientCredentialsGrantHandlerTestSuite) TestHandleGrant_TokenTimingValidation() {
	tokenRequest := &model.TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     testClientID,
		ClientSecret: "secret123",
		Scope:        "read",
	}

	suite.mockAuthzService.On("GetAuthorizedPermissions", mock.Anything,
		authz.GetAuthorizedPermissionsRequest{
			EntityID:             suite.oauthApp.ID,
			RequestedPermissions: []string{"read"},
		}).Return(&authz.GetAuthorizedPermissionsResponse{
		AuthorizedPermissions: []string{"read"},
	}, nil)

	expectedToken := testJWTToken
	now := time.Now().Unix()
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything).
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

	oauthAppWithOU := &inboundmodel.OAuthClient{
		ID:                      "app123",
		ClientID:                testClientID,
		OUID:                    "ou-456",
		GrantTypes:              []constants.GrantType{constants.GrantTypeClientCredentials},
		TokenEndpointAuthMethod: constants.TokenEndpointAuthMethodClientSecretBasic,
	}

	suite.mockAuthzService.On("GetAuthorizedPermissions", mock.Anything,
		authz.GetAuthorizedPermissionsRequest{
			EntityID:             oauthAppWithOU.ID,
			RequestedPermissions: []string{"read"},
		}).Return(&authz.GetAuthorizedPermissionsResponse{
		AuthorizedPermissions: []string{"read"},
	}, nil)

	suite.mockOUService.On("GetOrganizationUnit", context.Background(), "ou-456").Return(
		ou.OrganizationUnit{},
		&serviceerror.ServiceError{
			Code:  "OU-0001",
			Error: core.I18nMessage{Key: "error.test.not_found", DefaultValue: "not found"},
		},
	)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, oauthAppWithOU)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorServerError, errResp.Error)
	assert.Equal(suite.T(), "Failed to generate token", errResp.ErrorDescription)
}

// Resource Parameter Tests (RFC 8707) for Client Credentials Grant

func (suite *ClientCredentialsGrantHandlerTestSuite) TestHandleGrant_WithResourceParameter() {
	tokenRequest := &model.TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     testClientID,
		ClientSecret: "secret123",
		Scope:        "read",
		Resources:    []string{"https://mcp.example.com/mcp"},
	}

	suite.mockAuthzService.On("GetAuthorizedPermissions", mock.Anything,
		authz.GetAuthorizedPermissionsRequest{
			EntityID:             suite.oauthApp.ID,
			RequestedPermissions: []string{"read"},
		}).Return(&authz.GetAuthorizedPermissionsResponse{
		AuthorizedPermissions: []string{"read"},
	}, nil)

	var capturedAudiences []string
	suite.mockTokenBuilder.On("BuildAccessToken", mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
		capturedAudiences = ctx.Audiences
		return ctx.Subject == testClientID &&
			len(ctx.Audiences) == 1 &&
			ctx.Audiences[0] == "https://mcp.example.com/mcp"
	})).Return(&model.TokenDTO{
		Token:     testJWTToken,
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  int64(1234567890),
		ExpiresIn: 3600,
		Scopes:    []string{"read"},
		ClientID:  "client123",
		Subject:   testClientID,
		Audiences: []string{"https://mcp.example.com/mcp"},
	}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)

	// When RS contributes, clientID is NOT included; aud contains only the RS identifier.
	assert.Equal(suite.T(), []string{"https://mcp.example.com/mcp"}, capturedAudiences)
}

func (suite *ClientCredentialsGrantHandlerTestSuite) TestHandleGrant_WithoutResourceParameter() {
	tokenRequest := &model.TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     testClientID,
		ClientSecret: "secret123",
		Scope:        "read",
	}

	suite.mockAuthzService.On("GetAuthorizedPermissions", mock.Anything,
		authz.GetAuthorizedPermissionsRequest{
			EntityID:             suite.oauthApp.ID,
			RequestedPermissions: []string{"read"},
		}).Return(&authz.GetAuthorizedPermissionsResponse{
		AuthorizedPermissions: []string{"read"},
	}, nil)

	var capturedAudience string
	suite.mockTokenBuilder.On("BuildAccessToken", mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
		if len(ctx.Audiences) > 0 {
			capturedAudience = ctx.Audiences[0]
		}
		return ctx.Subject == testClientID && (len(ctx.Audiences) > 0 && ctx.Audiences[0] == testClientID)
	})).Return(&model.TokenDTO{
		Token:     testJWTToken,
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  int64(1234567890),
		ExpiresIn: 3600,
		Scopes:    []string{"read"},
		ClientID:  "client123",
		Subject:   testClientID,
		Audiences: []string{testClientID},
	}, nil)

	result, errResp := suite.handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)

	// Verify default audience (client_id) when no resource parameter
	assert.Equal(suite.T(), testClientID, capturedAudience)

	// Verify token attributes use client ID as audience when no resource
	assert.Contains(suite.T(), result.AccessToken.Audiences, testClientID)
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
	suite.mockAuthzService.On("GetAuthorizedPermissions", mock.Anything,
		authz.GetAuthorizedPermissionsRequest{
			EntityID:             suite.oauthApp.ID,
			RequestedPermissions: []string{"read", "write", "delete"},
		}).Return(&authz.GetAuthorizedPermissionsResponse{
		AuthorizedPermissions: []string{"read", "write"},
	}, nil)

	suite.mockTokenBuilder.On("BuildAccessToken",
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
	suite.mockAuthzService.On("GetAuthorizedPermissions", mock.Anything,
		authz.GetAuthorizedPermissionsRequest{
			EntityID:             suite.oauthApp.ID,
			RequestedPermissions: []string{"admin:full"},
		}).Return(&authz.GetAuthorizedPermissionsResponse{
		AuthorizedPermissions: []string{},
	}, nil)

	suite.mockTokenBuilder.On("BuildAccessToken",
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

	suite.mockAuthzService.On("GetAuthorizedPermissions", mock.Anything,
		authz.GetAuthorizedPermissionsRequest{
			EntityID:             suite.oauthApp.ID,
			RequestedPermissions: []string{"read"},
		}).Return((*authz.GetAuthorizedPermissionsResponse)(nil),
		&serviceerror.ServiceError{
			Code: "AUTHZ-0001",
			Error: core.I18nMessage{
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
	suite.mockAuthzService.AssertNotCalled(suite.T(), "GetAuthorizedPermissions", mock.Anything, mock.Anything)
}

// QA §4 — Implicit RS discovery: no resource param + scope maps to a registered RS.
//
// These tests use fresh mocks (not the suite defaults) so that FindResourceServersByPermissions
// can be configured to return a non-empty result without conflicting with the suite-level
// catch-all .Maybe() registration.

func (suite *ClientCredentialsGrantHandlerTestSuite) TestHandleGrant_ImplicitRSDiscovery_NoResourceScopeMapsToRS() {
	// When no resource parameter is supplied but the granted scope maps to a registered RS,
	// ComposeAudiences discovers it via FindResourceServersByPermissions and aud contains the RS
	// identifier rather than the clientID fallback.
	const rsIdentifier = "https://rs01.example.com"

	mockTokenBuilder := tokenservicemock.NewTokenBuilderInterfaceMock(suite.T())
	mockAuthzService := authzmock.NewAuthorizationServiceInterfaceMock(suite.T())
	mockResourceService := resourcemock.NewResourceServiceInterfaceMock(suite.T())
	mockEntityProv := entityprovidermock.NewEntityProviderInterfaceMock(suite.T())

	handler := &clientCredentialsGrantHandler{
		tokenBuilder:    mockTokenBuilder,
		ouService:       suite.mockOUService,
		authzService:    mockAuthzService,
		entityProv:      mockEntityProv,
		resourceService: mockResourceService,
	}

	mockEntityProv.On("GetTransitiveEntityGroups", mock.Anything).
		Return([]entityprovider.EntityGroup{}, nil).Maybe()

	// No resource param — ResolveResourceServers returns nil (no explicit identifiers).
	// GetResourceServerByIdentifier is not called.
	// ValidatePermissions is not called (no explicit RS).

	mockAuthzService.On("GetAuthorizedPermissions", mock.Anything,
		authz.GetAuthorizedPermissionsRequest{
			EntityID:             suite.oauthApp.ID,
			RequestedPermissions: []string{"r1:s1"},
		}).Return(&authz.GetAuthorizedPermissionsResponse{
		AuthorizedPermissions: []string{"r1:s1"},
	}, nil)

	mockResourceService.On("FindResourceServersByPermissions", mock.Anything, []string{"r1:s1"}).
		Return([]resource.ResourceServer{
			{ID: "rs01", Identifier: rsIdentifier},
		}, nil)

	var capturedAudiences []string
	mockTokenBuilder.On("BuildAccessToken", mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
		capturedAudiences = ctx.Audiences
		return true
	})).Return(&model.TokenDTO{
		Token:     testJWTToken,
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  int64(1234567890),
		ExpiresIn: 3600,
		Scopes:    []string{"r1:s1"},
		ClientID:  testClientID,
		Subject:   testClientID,
		Audiences: []string{rsIdentifier},
	}, nil)

	tokenRequest := &model.TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     testClientID,
		ClientSecret: "secret123",
		Scope:        "r1:s1",
	}

	result, errResp := handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), []string{rsIdentifier}, capturedAudiences)
}

func (suite *ClientCredentialsGrantHandlerTestSuite) TestHandleGrant_ImplicitRSDiscovery_MultipleRSes() {
	// When the granted scope maps to two registered RSes, both identifiers appear in aud (sorted).
	const rsIdentifier1 = "https://rs01.example.com"
	const rsIdentifier2 = "https://rs02.example.com"

	mockTokenBuilder := tokenservicemock.NewTokenBuilderInterfaceMock(suite.T())
	mockAuthzService := authzmock.NewAuthorizationServiceInterfaceMock(suite.T())
	mockResourceService := resourcemock.NewResourceServiceInterfaceMock(suite.T())
	mockEntityProv := entityprovidermock.NewEntityProviderInterfaceMock(suite.T())

	handler := &clientCredentialsGrantHandler{
		tokenBuilder:    mockTokenBuilder,
		ouService:       suite.mockOUService,
		authzService:    mockAuthzService,
		entityProv:      mockEntityProv,
		resourceService: mockResourceService,
	}

	mockEntityProv.On("GetTransitiveEntityGroups", mock.Anything).
		Return([]entityprovider.EntityGroup{}, nil).Maybe()

	mockAuthzService.On("GetAuthorizedPermissions", mock.Anything,
		authz.GetAuthorizedPermissionsRequest{
			EntityID:             suite.oauthApp.ID,
			RequestedPermissions: []string{"r1:s1"},
		}).Return(&authz.GetAuthorizedPermissionsResponse{
		AuthorizedPermissions: []string{"r1:s1"},
	}, nil)

	// Both RSes own the granted scope — ComposeAudiences includes both identifiers.
	mockResourceService.On("FindResourceServersByPermissions", mock.Anything, []string{"r1:s1"}).
		Return([]resource.ResourceServer{
			{ID: "rs01", Identifier: rsIdentifier1},
			{ID: "rs02", Identifier: rsIdentifier2},
		}, nil)

	var capturedAudiences []string
	mockTokenBuilder.On("BuildAccessToken", mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
		capturedAudiences = ctx.Audiences
		return true
	})).Return(&model.TokenDTO{
		Token:     testJWTToken,
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  int64(1234567890),
		ExpiresIn: 3600,
		Scopes:    []string{"r1:s1"},
		ClientID:  testClientID,
		Subject:   testClientID,
		Audiences: []string{rsIdentifier1, rsIdentifier2},
	}, nil)

	tokenRequest := &model.TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     testClientID,
		ClientSecret: "secret123",
		Scope:        "r1:s1",
	}

	result, errResp := handler.HandleGrant(context.Background(), tokenRequest, suite.oauthApp)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), result)
	assert.Len(suite.T(), capturedAudiences, 2)
	assert.Contains(suite.T(), capturedAudiences, rsIdentifier1)
	assert.Contains(suite.T(), capturedAudiences, rsIdentifier2)
}
