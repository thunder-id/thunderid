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
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/authz"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/tests/mocks/attributecachemock"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/authzmock"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/tokenservicemock"
	"github.com/thunder-id/thunderid/tests/mocks/resourcemock"
)

const (
	oidcReadWriteScopes         = "openid read write"
	testCodeChallenge           = "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	testCodeChallengeMethodS256 = "S256"
	testClientCallbackURL       = "https://client.example.com/callback"
	testCacheID                 = "test-cache-id"
)

// convertToStringSlice converts groups from various formats to []string for testing.
func convertToStringSlice(groups interface{}) []string {
	if groups == nil {
		return nil
	}
	switch v := groups.(type) {
	case []string:
		return v
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result
	default:
		return nil
	}
}

type AuthorizationCodeGrantHandlerTestSuite struct {
	suite.Suite
	handler              *authorizationCodeGrantHandler
	mockJWTService       *jwtmock.JWTServiceInterfaceMock
	mockTokenBuilder     *tokenservicemock.TokenBuilderInterfaceMock
	mockAuthzService     *authzmock.AuthorizeServiceInterfaceMock
	mockAttrCacheService *attributecachemock.AttributeCacheServiceInterfaceMock
	mockResourceService  *resourcemock.ResourceServiceInterfaceMock
	oauthApp             *inboundmodel.OAuthClient
	testAuthzCode        authz.AuthorizationCode
	testTokenReq         *model.TokenRequest
}

func TestAuthorizationCodeGrantHandlerSuite(t *testing.T) {
	suite.Run(t, new(AuthorizationCodeGrantHandlerTestSuite))
}

func (suite *AuthorizationCodeGrantHandlerTestSuite) SetupTest() {
	// Initialize Runtime config with basic test config
	testConfig := &config.Config{
		JWT: config.JWTConfig{
			ValidityPeriod: 3600,
		},
	}
	_ = config.InitializeServerRuntime("test", testConfig)

	suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())
	suite.mockTokenBuilder = tokenservicemock.NewTokenBuilderInterfaceMock(suite.T())
	suite.mockAuthzService = authzmock.NewAuthorizeServiceInterfaceMock(suite.T())
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
	suite.mockResourceService.On("FindResourceServersByPermissions", mock.Anything, mock.Anything).
		Return([]resource.ResourceServer{}, nil).Maybe()

	suite.handler = &authorizationCodeGrantHandler{
		tokenBuilder:    suite.mockTokenBuilder,
		authzService:    suite.mockAuthzService,
		attributeCache:  suite.mockAttrCacheService,
		resourceService: suite.mockResourceService,
	}

	suite.oauthApp = &inboundmodel.OAuthClient{
		ClientID: testClientID,

		RedirectURIs:            []string{"https://client.example.com/callback"},
		GrantTypes:              []constants.GrantType{constants.GrantTypeAuthorizationCode},
		ResponseTypes:           []constants.ResponseType{constants.ResponseTypeCode},
		TokenEndpointAuthMethod: constants.TokenEndpointAuthMethodClientSecretPost,
		Token: &inboundmodel.OAuthTokenConfig{
			AccessToken: &inboundmodel.AccessTokenConfig{
				UserAttributes: []string{"email", "username"},
			},
		},
	}

	suite.testTokenReq = &model.TokenRequest{
		GrantType:   string(constants.GrantTypeAuthorizationCode),
		ClientID:    testClientID,
		Code:        "test-auth-code",
		RedirectURI: "https://client.example.com/callback",
	}

	suite.testAuthzCode = authz.AuthorizationCode{
		CodeID:           "test-code-id",
		Code:             "test-auth-code",
		ClientID:         testClientID,
		RedirectURI:      "https://client.example.com/callback",
		AuthorizedUserID: testUserID,
		AttributeCacheID: "",
		TimeCreated:      time.Now().Add(-5 * time.Minute),
		ExpiryTime:       time.Now().Add(5 * time.Minute),
		Scopes:           "read write",
		State:            authz.AuthCodeStateActive,
	}
}

func (suite *AuthorizationCodeGrantHandlerTestSuite) TestNewAuthorizationCodeGrantHandler() {
	handler := newAuthorizationCodeGrantHandler(
		suite.mockAuthzService, suite.mockTokenBuilder, suite.mockAttrCacheService, suite.mockResourceService)
	assert.NotNil(suite.T(), handler)
	assert.Implements(suite.T(), (*GrantHandlerInterface)(nil), handler)
}

func (suite *AuthorizationCodeGrantHandlerTestSuite) TestValidateGrant_Success() {
	err := suite.handler.ValidateGrant(context.Background(), suite.testTokenReq, suite.oauthApp)
	assert.Nil(suite.T(), err)
}

func (suite *AuthorizationCodeGrantHandlerTestSuite) TestValidateGrant_MissingGrantType() {
	tokenReq := &model.TokenRequest{
		GrantType: "", // Missing grant type
		ClientID:  testClientID,
		Code:      "test-code",
	}

	err := suite.handler.ValidateGrant(context.Background(), tokenReq, suite.oauthApp)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, err.Error)
	assert.Equal(suite.T(), "Missing grant_type parameter", err.ErrorDescription)
}

func (suite *AuthorizationCodeGrantHandlerTestSuite) TestValidateGrant_UnsupportedGrantType() {
	tokenReq := &model.TokenRequest{
		GrantType: string(constants.GrantTypeClientCredentials), // Wrong grant type
		ClientID:  testClientID,
		Code:      "test-code",
	}

	err := suite.handler.ValidateGrant(context.Background(), tokenReq, suite.oauthApp)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorUnsupportedGrantType, err.Error)
	assert.Equal(suite.T(), "Unsupported grant type", err.ErrorDescription)
}

func (suite *AuthorizationCodeGrantHandlerTestSuite) TestValidateGrant_MissingAuthorizationCode() {
	tokenReq := &model.TokenRequest{
		GrantType: string(constants.GrantTypeAuthorizationCode),
		ClientID:  testClientID,
		Code:      "", // Missing authorization code
	}

	err := suite.handler.ValidateGrant(context.Background(), tokenReq, suite.oauthApp)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, err.Error)
	assert.Equal(suite.T(), "Authorization code is required", err.ErrorDescription)
}

func (suite *AuthorizationCodeGrantHandlerTestSuite) TestValidateGrant_MissingClientID() {
	tokenReq := &model.TokenRequest{
		GrantType: string(constants.GrantTypeAuthorizationCode),
		ClientID:  "", // Missing client ID
		Code:      "test-code",
	}

	err := suite.handler.ValidateGrant(context.Background(), tokenReq, suite.oauthApp)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorInvalidClient, err.Error)
	assert.Equal(suite.T(), "client_id is required", err.ErrorDescription)
}

func (suite *AuthorizationCodeGrantHandlerTestSuite) TestValidateGrant_MissingRedirectURI() {
	tokenReq := &model.TokenRequest{
		GrantType:   string(constants.GrantTypeAuthorizationCode),
		ClientID:    testClientID,
		Code:        "test-code",
		RedirectURI: "", // Missing redirect URI
	}

	err := suite.handler.ValidateGrant(context.Background(), tokenReq, suite.oauthApp)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, err.Error)
	assert.Equal(suite.T(), "Redirect URI is required", err.ErrorDescription)
}

func (suite *AuthorizationCodeGrantHandlerTestSuite) TestHandleGrant_Success() {
	// Create authorization code with resource
	authCodeWithResource := suite.testAuthzCode
	authCodeWithResource.Resources = []string{testResourceURL}

	// Mock authorization code store to return valid code with resource
	suite.mockAuthzService.On("GetAuthorizationCodeDetails", mock.Anything, testClientID, "test-auth-code").
		Return(&authCodeWithResource, nil)

	// Mock token builder to generate access token
	suite.mockTokenBuilder.On("BuildAccessToken", mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
		return ctx.Subject == testUserID &&
			len(ctx.Audiences) == 1 &&
			ctx.Audiences[0] == testResourceURL &&
			ctx.ClientID == testClientID &&
			ctx.GrantType == string(constants.GrantTypeAuthorizationCode)
	})).Return(&model.TokenDTO{
		Token:     "test-jwt-token",
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  time.Now().Unix(),
		ExpiresIn: 3600,
		Scopes:    []string{"read", "write"},
		ClientID:  testClientID,
		Subject:   testUserID,
		Audiences: []string{testResourceURL},
	}, nil)

	// Create token request with matching resource
	tokenReqWithResource := *suite.testTokenReq
	tokenReqWithResource.Resources = []string{testResourceURL}

	result, err := suite.handler.HandleGrant(context.Background(), &tokenReqWithResource, suite.oauthApp)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "test-jwt-token", result.AccessToken.Token)
	assert.Equal(suite.T(), constants.TokenTypeBearer, result.AccessToken.TokenType)
	assert.Equal(suite.T(), int64(3600), result.AccessToken.ExpiresIn)
	assert.Equal(suite.T(), []string{"read", "write"}, result.AccessToken.Scopes)
	assert.Equal(suite.T(), testClientID, result.AccessToken.ClientID)

	// Check token attributes
	assert.Equal(suite.T(), testUserID, result.AccessToken.Subject)
	assert.Contains(suite.T(), result.AccessToken.Audiences, testResourceURL)

	suite.mockAuthzService.AssertExpectations(suite.T())
	suite.mockTokenBuilder.AssertExpectations(suite.T())
}

func (suite *AuthorizationCodeGrantHandlerTestSuite) TestHandleGrant_InvalidAuthorizationCode() {
	// Mock authorization code store to return error
	suite.mockAuthzService.On("GetAuthorizationCodeDetails", mock.Anything, testClientID, "test-auth-code").
		Return(nil, errors.New("invalid authorization code"))

	// Create token request with matching resource
	tokenReqWithResource := *suite.testTokenReq
	tokenReqWithResource.Resources = []string{testResourceURL}

	result, err := suite.handler.HandleGrant(context.Background(), &tokenReqWithResource, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorInvalidGrant, err.Error)
	assert.Equal(suite.T(), "Invalid authorization code", err.ErrorDescription)

	suite.mockAuthzService.AssertExpectations(suite.T())
}

func (suite *AuthorizationCodeGrantHandlerTestSuite) TestHandleGrant_JWTGenerationError() {
	// Mock authorization code store to return valid code
	suite.mockAuthzService.On("GetAuthorizationCodeDetails", mock.Anything, testClientID, "test-auth-code").
		Return(&suite.testAuthzCode, nil)

	// Mock token builder to fail token generation
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything).Return(nil, errors.New("jwt generation failed"))

	// Create token request with matching resource
	tokenReqWithResource := *suite.testTokenReq
	tokenReqWithResource.Resources = []string{testResourceURL}

	result, err := suite.handler.HandleGrant(context.Background(), &tokenReqWithResource, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorServerError, err.Error)
	assert.Equal(suite.T(), "Failed to generate token", err.ErrorDescription)

	suite.mockAuthzService.AssertExpectations(suite.T())
	suite.mockTokenBuilder.AssertExpectations(suite.T())
}

func (suite *AuthorizationCodeGrantHandlerTestSuite) TestHandleGrant_EmptyScopes() {
	// Test with empty scopes
	authzCodeWithEmptyScopes := suite.testAuthzCode
	authzCodeWithEmptyScopes.Scopes = ""

	suite.mockAuthzService.On("GetAuthorizationCodeDetails", mock.Anything, testClientID, "test-auth-code").
		Return(&authzCodeWithEmptyScopes, nil)

	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything).Return(&model.TokenDTO{
		Token:     "test-jwt-token",
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  time.Now().Unix(),
		ExpiresIn: 3600,
		Scopes:    []string{},
		ClientID:  testClientID,
	}, nil)

	// Create token request with matching resource
	tokenReqWithResource := *suite.testTokenReq
	tokenReqWithResource.Resources = []string{testResourceURL}

	result, err := suite.handler.HandleGrant(context.Background(), &tokenReqWithResource, suite.oauthApp)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Empty(suite.T(), result.AccessToken.Scopes)

	suite.mockAuthzService.AssertExpectations(suite.T())
	suite.mockTokenBuilder.AssertExpectations(suite.T())
}

func (suite *AuthorizationCodeGrantHandlerTestSuite) TestHandleGrant_NilTokenAttributes() {
	// Test with nil token attributes
	suite.mockAuthzService.On("GetAuthorizationCodeDetails", mock.Anything, testClientID, "test-auth-code").
		Return(&suite.testAuthzCode, nil)

	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything).Return(&model.TokenDTO{
		Token:     "test-jwt-token",
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  time.Now().Unix(),
		ExpiresIn: 3600,
		Scopes:    []string{"read", "write"},
		ClientID:  testClientID,
		Subject:   testUserID,
		Audiences: []string{testClientID},
	}, nil)

	// Create token request with matching resource
	tokenReqWithResource := *suite.testTokenReq
	tokenReqWithResource.Resources = []string{testResourceURL}

	result, err := suite.handler.HandleGrant(context.Background(), &tokenReqWithResource, suite.oauthApp)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)

	// Check token attributes
	assert.Equal(suite.T(), testUserID, result.AccessToken.Subject)
	assert.Contains(suite.T(), result.AccessToken.Audiences, testClientID)

	suite.mockAuthzService.AssertExpectations(suite.T())
	suite.mockTokenBuilder.AssertExpectations(suite.T())
}

func (suite *AuthorizationCodeGrantHandlerTestSuite) TestValidateAuthorizationCode_Success() {
	err := validateAuthorizationCode(suite.testTokenReq, suite.testAuthzCode)
	assert.Nil(suite.T(), err)
}

func (suite *AuthorizationCodeGrantHandlerTestSuite) TestValidateAuthorizationCode_WrongClientID() {
	invalidTokenReq := &model.TokenRequest{
		ClientID: "wrong-client-id", // Wrong client ID
	}

	err := validateAuthorizationCode(invalidTokenReq, suite.testAuthzCode)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorInvalidGrant, err.Error)
	assert.Equal(suite.T(), "Invalid authorization code", err.ErrorDescription)
}

func (suite *AuthorizationCodeGrantHandlerTestSuite) TestValidateAuthorizationCode_WrongRedirectURI() {
	invalidTokenReq := &model.TokenRequest{
		ClientID:    testClientID,
		RedirectURI: "https://wrong.example.com/callback", // Wrong redirect URI
	}

	err := validateAuthorizationCode(invalidTokenReq, suite.testAuthzCode)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorInvalidGrant, err.Error)
	assert.Equal(suite.T(), "Invalid redirect URI", err.ErrorDescription)
}

func (suite *AuthorizationCodeGrantHandlerTestSuite) TestValidateAuthorizationCode_EmptyRedirectURIInCode() {
	authzCodeWithEmptyURI := suite.testAuthzCode
	authzCodeWithEmptyURI.RedirectURI = ""

	tokenReq := &model.TokenRequest{
		ClientID:    testClientID,
		RedirectURI: "https://any.example.com/callback",
	}

	err := validateAuthorizationCode(tokenReq, authzCodeWithEmptyURI)
	assert.Nil(suite.T(), err)
}

func (suite *AuthorizationCodeGrantHandlerTestSuite) TestValidateAuthorizationCode_ExpiredCode() {
	expiredCode := suite.testAuthzCode
	expiredCode.ExpiryTime = time.Now().Add(-5 * time.Minute) // Expired

	err := validateAuthorizationCode(suite.testTokenReq, expiredCode)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorInvalidGrant, err.Error)
	assert.Equal(suite.T(), "Expired authorization code", err.ErrorDescription)
}

func (suite *AuthorizationCodeGrantHandlerTestSuite) TestHandleGrant_WithGroups() {
	testCases := []struct {
		name                 string
		includeInAccessToken bool
		includeInIDToken     bool
		includeOpenIDScope   bool
		scopeClaimsForGroups bool
		expectedGroups       []string
		description          string
	}{
		{
			name:                 "Groups in access token with ID token config",
			includeInAccessToken: true,
			includeInIDToken:     true,
			includeOpenIDScope:   false,
			scopeClaimsForGroups: false,
			expectedGroups:       []string{"Admin", "Users"},
			description: "Should include groups in access token when configured (IDToken config " +
				"present but openid scope not requested)",
		},
		{
			name:                 "Groups in both access and ID tokens",
			includeInAccessToken: true,
			includeInIDToken:     true,
			includeOpenIDScope:   true,
			scopeClaimsForGroups: true,
			expectedGroups:       []string{"Admin", "Users"},
			description: "Should include groups in both tokens when configured with openid scope and scope" +
				"claims",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Reset mocks for each test case
			suite.mockAuthzService = authzmock.NewAuthorizeServiceInterfaceMock(suite.T())
			suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())
			suite.mockTokenBuilder = tokenservicemock.NewTokenBuilderInterfaceMock(suite.T())
			suite.mockAttrCacheService = attributecachemock.NewAttributeCacheServiceInterfaceMock(suite.T())
			suite.mockResourceService = resourcemock.NewResourceServiceInterfaceMock(suite.T())
			suite.mockResourceService.On("FindResourceServersByPermissions", mock.Anything, mock.Anything).
				Return([]resource.ResourceServer{}, nil).Maybe()
			suite.handler = &authorizationCodeGrantHandler{
				tokenBuilder:    suite.mockTokenBuilder,
				authzService:    suite.mockAuthzService,
				attributeCache:  suite.mockAttrCacheService,
				resourceService: suite.mockResourceService,
			}

			accessTokenAttrs := []string{"email", "username"}
			if tc.includeInAccessToken {
				accessTokenAttrs = append(accessTokenAttrs, constants.UserAttributeGroups)
			}
			var idTokenConfig *inboundmodel.IDTokenConfig
			var scopeClaims map[string][]string
			if tc.includeInIDToken {
				if tc.scopeClaimsForGroups {
					// Include groups in ID token config with scope claims mapping
					idTokenConfig = &inboundmodel.IDTokenConfig{
						UserAttributes: []string{"email", "username", constants.UserAttributeGroups},
					}
					scopeClaims = map[string][]string{
						"openid": {"email", "username", constants.UserAttributeGroups},
					}
				} else {
					idTokenConfig = &inboundmodel.IDTokenConfig{
						UserAttributes: []string{"email", "username"},
					}
				}
			}

			oauthAppWithGroups := &inboundmodel.OAuthClient{
				ClientID: testClientID,

				RedirectURIs:            []string{"https://client.example.com/callback"},
				GrantTypes:              []constants.GrantType{constants.GrantTypeAuthorizationCode},
				ResponseTypes:           []constants.ResponseType{constants.ResponseTypeCode},
				TokenEndpointAuthMethod: constants.TokenEndpointAuthMethodClientSecretPost,
				Token: &inboundmodel.OAuthTokenConfig{
					AccessToken: &inboundmodel.AccessTokenConfig{
						UserAttributes: accessTokenAttrs,
					},
					IDToken: idTokenConfig,
				},
				ScopeClaims: scopeClaims,
			}

			authzCode := suite.testAuthzCode
			if tc.includeOpenIDScope {
				authzCode.Scopes = oidcReadWriteScopes
			}

			// Add user attributes to authz code via attribute cache
			authzCode.AttributeCacheID = testCacheID
			expectedAttrs := map[string]interface{}{
				"email":    "test@example.com",
				"username": "testuser",
			}
			if len(tc.expectedGroups) > 0 {
				expectedAttrs[constants.UserAttributeGroups] = tc.expectedGroups
			}
			suite.mockAttrCacheService.On("GetAttributeCache", mock.Anything, testCacheID).
				Return(&attributecache.AttributeCache{
					ID:         testCacheID,
					Attributes: expectedAttrs,
				}, (*serviceerror.ServiceError)(nil)).Once()

			suite.mockAuthzService.On("GetAuthorizationCodeDetails", mock.Anything, testClientID, "test-auth-code").
				Return(&authzCode, nil)

			// Groups come from the authorization code (extracted from assertion during authorization)
			// No need to fetch from DB

			var capturedAccessTokenClaims map[string]interface{}
			var capturedIDTokenClaims map[string]interface{}

			// Mock access token generation - use function return to access context at call time
			suite.mockTokenBuilder.On("BuildAccessToken",
				mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
					// Capture user attributes and groups (simulate filtering that happens in BuildAccessToken)
					capturedAccessTokenClaims = make(map[string]interface{})
					for k, v := range ctx.UserAttributes {
						capturedAccessTokenClaims[k] = v
					}
					// Verify GrantType is authorization_code
					return ctx.GrantType == string(constants.GrantTypeAuthorizationCode)
				})).Return(func(ctx *tokenservice.AccessTokenBuildContext) (*model.TokenDTO, error) {
				// Simulate filtering that happens in BuildAccessToken
				userAttrs := make(map[string]interface{})
				for k, v := range ctx.UserAttributes {
					userAttrs[k] = v
				}
				return &model.TokenDTO{
					Token:          "test-jwt-token",
					TokenType:      constants.TokenTypeBearer,
					IssuedAt:       time.Now().Unix(),
					ExpiresIn:      3600,
					Scopes:         []string{"read", "write"},
					ClientID:       testClientID,
					UserAttributes: userAttrs,
				}, nil
			}).Once()

			// Mock ID token generation if openid scope is present
			if tc.includeOpenIDScope {
				suite.mockTokenBuilder.On("BuildIDToken",
					mock.MatchedBy(func(ctx *tokenservice.IDTokenBuildContext) bool {
						// Capture ID token claims
						capturedIDTokenClaims = make(map[string]interface{})
						for k, v := range ctx.UserAttributes {
							capturedIDTokenClaims[k] = v
						}
						// Groups are already in UserAttributes if configured
						// Token builder will extract and add them if needed
						return true
					})).Return(&model.TokenDTO{
					Token:     "test-id-token",
					TokenType: "",
					IssuedAt:  time.Now().Unix(),
					ExpiresIn: 3600,
					Scopes:    []string{"read", "write", "openid"},
					ClientID:  testClientID,
				}, nil).Once()
			}

			result, err := suite.handler.HandleGrant(context.Background(), suite.testTokenReq, oauthAppWithGroups)

			assert.Nil(suite.T(), err, tc.description)
			assert.NotNil(suite.T(), result, tc.description)

			// Verify access token groups
			if tc.includeInAccessToken {
				assert.NotNil(suite.T(), capturedAccessTokenClaims[constants.UserAttributeGroups], tc.description)
				groupsInClaims := convertToStringSlice(capturedAccessTokenClaims[constants.UserAttributeGroups])
				assert.Equal(suite.T(), tc.expectedGroups, groupsInClaims, tc.description)

				assert.NotNil(suite.T(),
					result.AccessToken.UserAttributes[constants.UserAttributeGroups], tc.description)
				groupsInAttrs := convertToStringSlice(result.AccessToken.UserAttributes[constants.UserAttributeGroups])
				assert.Equal(suite.T(), tc.expectedGroups, groupsInAttrs, tc.description)
			} else {
				assert.Nil(suite.T(), capturedAccessTokenClaims[constants.UserAttributeGroups], tc.description)
				assert.Nil(suite.T(), result.AccessToken.UserAttributes[constants.UserAttributeGroups], tc.description)
			}

			// Verify ID token groups
			if tc.includeInIDToken && tc.includeOpenIDScope && tc.scopeClaimsForGroups {
				assert.NotNil(suite.T(), result.IDToken.Token, tc.description)
				assert.NotNil(suite.T(), capturedIDTokenClaims[constants.UserAttributeGroups], tc.description)
				groupsInIDToken := convertToStringSlice(capturedIDTokenClaims[constants.UserAttributeGroups])
				assert.Equal(suite.T(), tc.expectedGroups, groupsInIDToken, tc.description)
			} else if tc.includeOpenIDScope {
				assert.NotNil(suite.T(), result.IDToken.Token, tc.description)
			} else {
				assert.Empty(suite.T(), result.IDToken.Token, tc.description)
			}

			suite.mockAuthzService.AssertExpectations(suite.T())
			suite.mockTokenBuilder.AssertExpectations(suite.T())
		})
	}
}

func (suite *AuthorizationCodeGrantHandlerTestSuite) TestHandleGrant_WithEmptyGroups() {
	testCases := []struct {
		name                 string
		includeInAccessToken bool
		includeInIDToken     bool
		includeOpenIDScope   bool
		scopeClaimsForGroups bool
		description          string
	}{
		{
			name:                 "Empty groups in access token",
			includeInAccessToken: true,
			includeInIDToken:     true,
			includeOpenIDScope:   false,
			scopeClaimsForGroups: false,
			description:          "Should not include groups claim in access token when user has no groups",
		},
		{
			name:                 "Empty groups with both tokens",
			includeInAccessToken: true,
			includeInIDToken:     true,
			includeOpenIDScope:   true,
			scopeClaimsForGroups: true,
			description:          "Should not include groups claim in either token when user has no groups",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.mockAuthzService = authzmock.NewAuthorizeServiceInterfaceMock(suite.T())
			suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())
			suite.mockTokenBuilder = tokenservicemock.NewTokenBuilderInterfaceMock(suite.T())
			suite.mockAttrCacheService = attributecachemock.NewAttributeCacheServiceInterfaceMock(suite.T())
			suite.mockResourceService = resourcemock.NewResourceServiceInterfaceMock(suite.T())
			suite.mockResourceService.On("FindResourceServersByPermissions", mock.Anything, mock.Anything).
				Return([]resource.ResourceServer{}, nil).Maybe()
			suite.handler = &authorizationCodeGrantHandler{
				tokenBuilder:    suite.mockTokenBuilder,
				authzService:    suite.mockAuthzService,
				attributeCache:  suite.mockAttrCacheService,
				resourceService: suite.mockResourceService,
			}

			accessTokenAttrs := []string{"email", "username"}
			if tc.includeInAccessToken {
				accessTokenAttrs = append(accessTokenAttrs, constants.UserAttributeGroups)
			}
			var idTokenConfig *inboundmodel.IDTokenConfig
			var scopeClaims map[string][]string
			if tc.includeInIDToken {
				if tc.scopeClaimsForGroups {
					idTokenConfig = &inboundmodel.IDTokenConfig{
						UserAttributes: []string{"email", "username", constants.UserAttributeGroups},
					}
					scopeClaims = map[string][]string{
						"openid": {"email", "username", constants.UserAttributeGroups},
					}
				} else {
					idTokenConfig = &inboundmodel.IDTokenConfig{
						UserAttributes: []string{"email", "username"},
					}
				}
			}

			oauthAppWithGroups := &inboundmodel.OAuthClient{
				ClientID: testClientID,

				RedirectURIs:            []string{"https://client.example.com/callback"},
				GrantTypes:              []constants.GrantType{constants.GrantTypeAuthorizationCode},
				ResponseTypes:           []constants.ResponseType{constants.ResponseTypeCode},
				TokenEndpointAuthMethod: constants.TokenEndpointAuthMethodClientSecretPost,
				Token: &inboundmodel.OAuthTokenConfig{
					AccessToken: &inboundmodel.AccessTokenConfig{
						UserAttributes: accessTokenAttrs,
					},
					IDToken: idTokenConfig,
				},
				ScopeClaims: scopeClaims,
			}

			authzCode := suite.testAuthzCode
			if tc.includeOpenIDScope {
				authzCode.Scopes = oidcReadWriteScopes
			}

			// Add user attributes to authz code via attribute cache (groups will be empty/not present)
			authzCode.AttributeCacheID = testCacheID
			suite.mockAttrCacheService.On("GetAttributeCache", mock.Anything, testCacheID).
				Return(&attributecache.AttributeCache{
					ID: testCacheID,
					Attributes: map[string]interface{}{
						"email":    "test@example.com",
						"username": "testuser",
					},
				}, (*serviceerror.ServiceError)(nil)).Once()
			// Empty groups - not added to Attributes (user has no groups)

			suite.mockAuthzService.On("GetAuthorizationCodeDetails", mock.Anything, testClientID, "test-auth-code").
				Return(&authzCode, nil)

			// Groups come from the authorization code (extracted from assertion during authorization)
			// If groups are not in auth code, user has no groups (empty array)
			// No need to fetch from DB

			var capturedAccessTokenClaims map[string]interface{}
			var capturedIDTokenClaims map[string]interface{}

			// Mock access token generation - use function return to access context at call time
			suite.mockTokenBuilder.On("BuildAccessToken",
				mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
					// Capture user attributes which contain groups
					capturedAccessTokenClaims = make(map[string]interface{})
					for k, v := range ctx.UserAttributes {
						capturedAccessTokenClaims[k] = v
					}
					// Verify GrantType is authorization_code
					return ctx.GrantType == string(constants.GrantTypeAuthorizationCode)
				})).Return(func(ctx *tokenservice.AccessTokenBuildContext) (*model.TokenDTO, error) {
				// Return user attributes from the actual call context
				userAttrs := make(map[string]interface{})
				for k, v := range ctx.UserAttributes {
					userAttrs[k] = v
				}
				return &model.TokenDTO{
					Token:          "test-jwt-token",
					TokenType:      constants.TokenTypeBearer,
					IssuedAt:       time.Now().Unix(),
					ExpiresIn:      3600,
					Scopes:         []string{"read", "write"},
					ClientID:       testClientID,
					UserAttributes: userAttrs,
				}, nil
			}).Once()

			// Mock ID token generation if openid scope is present
			if tc.includeOpenIDScope {
				suite.mockTokenBuilder.On("BuildIDToken",
					mock.MatchedBy(func(ctx *tokenservice.IDTokenBuildContext) bool {
						// Capture ID token claims from user attributes
						capturedIDTokenClaims = ctx.UserAttributes
						return true
					})).Return(&model.TokenDTO{
					Token:     "test-id-token",
					TokenType: "",
					IssuedAt:  time.Now().Unix(),
					ExpiresIn: 3600,
					Scopes:    []string{"read", "write", "openid"},
					ClientID:  testClientID,
				}, nil).Once()
			}

			result, err := suite.handler.HandleGrant(context.Background(), suite.testTokenReq, oauthAppWithGroups)

			assert.Nil(suite.T(), err, tc.description)
			assert.NotNil(suite.T(), result, tc.description)

			assert.Nil(suite.T(), capturedAccessTokenClaims[constants.UserAttributeGroups], tc.description)
			assert.Nil(suite.T(), result.AccessToken.UserAttributes[constants.UserAttributeGroups], tc.description)

			// Verify ID token
			if tc.includeOpenIDScope {
				assert.NotNil(suite.T(), result.IDToken.Token, tc.description)
				assert.Nil(suite.T(), capturedIDTokenClaims[constants.UserAttributeGroups], tc.description)
			} else {
				assert.Empty(suite.T(), result.IDToken.Token, tc.description)
			}

			suite.mockAuthzService.AssertExpectations(suite.T())
			suite.mockTokenBuilder.AssertExpectations(suite.T())
		})
	}
}

// Resource Parameter Tests (RFC 8707)

func (suite *AuthorizationCodeGrantHandlerTestSuite) TestHandleGrant_ResourceParameterMismatch() {
	// Set up auth code with different resource than token request
	authCodeWithResource := suite.testAuthzCode
	authCodeWithResource.Resources = []string{"https://api.example.com/resource"}

	suite.mockAuthzService.On("GetAuthorizationCodeDetails", mock.Anything, testClientID, "test-auth-code").
		Return(&authCodeWithResource, nil)

	// Create token request with different resource
	tokenReqWithResource := *suite.testTokenReq
	tokenReqWithResource.Resources = []string{testResourceURL}

	result, err := suite.handler.HandleGrant(context.Background(), &tokenReqWithResource, suite.oauthApp)

	assert.NotNil(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), constants.ErrorInvalidTarget, err.Error)
	assert.Equal(suite.T(), "Resource parameter mismatch", err.ErrorDescription)
}

func (suite *AuthorizationCodeGrantHandlerTestSuite) TestHandleGrant_ResourceParameterMatch() {
	// Set up auth code with resource parameter
	authCodeWithResource := suite.testAuthzCode
	authCodeWithResource.Resources = []string{testResourceURL}

	suite.mockAuthzService.On("GetAuthorizationCodeDetails", mock.Anything, testClientID, "test-auth-code").
		Return(&authCodeWithResource, nil)

	var capturedAudiences []string
	suite.mockTokenBuilder.On("BuildAccessToken", mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
		capturedAudiences = ctx.Audiences
		return true
	})).Return(&model.TokenDTO{
		Token:     "mock-jwt-token",
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  time.Now().Unix(),
		ExpiresIn: 3600,
		Scopes:    []string{"read", "write"},
		ClientID:  testClientID,
	}, nil)

	// Create token request with matching resource
	tokenReqWithResource := *suite.testTokenReq
	tokenReqWithResource.Resources = []string{testResourceURL}

	result, err := suite.handler.HandleGrant(context.Background(), &tokenReqWithResource, suite.oauthApp)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), []string{testResourceURL}, capturedAudiences)
}

func (suite *AuthorizationCodeGrantHandlerTestSuite) TestHandleGrant_NoResourceParameter() {
	// Auth code without resource parameter, token request also sends no resource.
	// Audience falls back to clientID (no RS contributes).
	suite.mockAuthzService.On("GetAuthorizationCodeDetails", mock.Anything, testClientID, "test-auth-code").
		Return(&suite.testAuthzCode, nil)

	var capturedAudiences []string
	suite.mockTokenBuilder.On("BuildAccessToken", mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
		capturedAudiences = ctx.Audiences
		return true
	})).Return(&model.TokenDTO{
		Token:     "mock-jwt-token",
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  time.Now().Unix(),
		ExpiresIn: 3600,
		Scopes:    []string{"read", "write"},
		ClientID:  testClientID,
	}, nil)

	// Token request sends no resource parameter.
	result, err := suite.handler.HandleGrant(context.Background(), suite.testTokenReq, suite.oauthApp)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)

	// Audience falls back to clientID when no RS contributes.
	assert.Equal(suite.T(), []string{testClientID}, capturedAudiences)
}

func (suite *AuthorizationCodeGrantHandlerTestSuite) TestHandleGrant_FetchUserOUFailed() {
	suite.T().Skip("OU service is no longer used - OU details are retrieved from authz code " +
		"which comes from JWT claims")
}

func (suite *AuthorizationCodeGrantHandlerTestSuite) TestHandleGrant_IDTokenGenerationFailed() {
	// Create auth code with openid scope
	authzCodeWithOpenID := suite.testAuthzCode
	authzCodeWithOpenID.Scopes = oidcReadWriteScopes

	suite.mockAuthzService.On("GetAuthorizationCodeDetails", mock.Anything, testClientID, "test-auth-code").
		Return(&authzCodeWithOpenID, nil)

	// Mock access token generation succeeds
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything).Return(&model.TokenDTO{
		Token:     "test-jwt-token",
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  time.Now().Unix(),
		ExpiresIn: 3600,
		Scopes:    []string{"openid", "read", "write"},
		ClientID:  testClientID,
	}, nil)

	// Mock ID token generation fails
	suite.mockTokenBuilder.On("BuildIDToken", mock.Anything).
		Return(nil, errors.New("failed to generate ID token"))

	result, err := suite.handler.HandleGrant(context.Background(), suite.testTokenReq, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorServerError, err.Error)
	assert.Equal(suite.T(), "Failed to generate token", err.ErrorDescription)

	suite.mockAuthzService.AssertExpectations(suite.T())
	suite.mockTokenBuilder.AssertExpectations(suite.T())
}

func (suite *AuthorizationCodeGrantHandlerTestSuite) TestValidateGrant_ResourceWithFragment() {
	// Test resource parameter with fragment component
	tokenReq := &model.TokenRequest{
		GrantType:   string(constants.GrantTypeAuthorizationCode),
		ClientID:    testClientID,
		Code:        "test-code",
		RedirectURI: "https://client.example.com/callback",
		Resources:   []string{"https://api.example.com/resource#fragment"}, // Fragment not allowed
	}

	err := suite.handler.ValidateGrant(context.Background(), tokenReq, suite.oauthApp)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorInvalidTarget, err.Error)
	assert.Contains(suite.T(), err.ErrorDescription, "fragment component")
}

func (suite *AuthorizationCodeGrantHandlerTestSuite) TestValidateGrant_ResourceParseError() {
	// Test resource parameter that fails to parse
	tokenReq := &model.TokenRequest{
		GrantType:   string(constants.GrantTypeAuthorizationCode),
		ClientID:    testClientID,
		Code:        "test-code",
		RedirectURI: "https://client.example.com/callback",
		Resources:   []string{"://invalid-uri"}, // Invalid URI format
	}

	err := suite.handler.ValidateGrant(context.Background(), tokenReq, suite.oauthApp)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorInvalidTarget, err.Error)
}

func (suite *AuthorizationCodeGrantHandlerTestSuite) TestHandleGrant_FetchUserGroupsError() {
	// Test that groups are retrieved from authorization code (not fetched from DB)
	// Create OAuth app with groups configured
	oauthAppWithGroups := &inboundmodel.OAuthClient{
		ClientID: testClientID,

		RedirectURIs:            []string{"https://client.example.com/callback"},
		GrantTypes:              []constants.GrantType{constants.GrantTypeAuthorizationCode},
		ResponseTypes:           []constants.ResponseType{constants.ResponseTypeCode},
		TokenEndpointAuthMethod: constants.TokenEndpointAuthMethodClientSecretPost,
		Token: &inboundmodel.OAuthTokenConfig{
			AccessToken: &inboundmodel.AccessTokenConfig{
				UserAttributes: []string{"email", "username", constants.UserAttributeGroups},
			},
		},
	}

	// Add groups to auth code via attribute cache (from assertion)
	authzCodeWithGroups := suite.testAuthzCode
	authzCodeWithGroups.AttributeCacheID = testCacheID
	suite.mockAttrCacheService.On("GetAttributeCache", mock.Anything, testCacheID).
		Return(&attributecache.AttributeCache{
			ID: testCacheID,
			Attributes: map[string]interface{}{
				"email":    "test@example.com",
				"username": "testuser",
				"groups":   []string{"Admin", "Users"},
			},
		}, (*serviceerror.ServiceError)(nil))

	suite.mockAuthzService.On("GetAuthorizationCodeDetails", mock.Anything, testClientID, "test-auth-code").
		Return(&authzCodeWithGroups, nil)

	// Groups come from the authorization code (extracted from assertion during authorization)
	// Token builder will extract groups from UserAttributes - verify groups are in UserAttributes
	suite.mockTokenBuilder.On("BuildAccessToken", mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
		// Verify groups are in UserAttributes (will be extracted by token builder)
		groupsValue, ok := ctx.UserAttributes[constants.UserAttributeGroups]
		if !ok {
			return false
		}
		groupsArray := convertToStringSlice(groupsValue)
		return len(groupsArray) == 2 &&
			groupsArray[0] == "Admin" &&
			groupsArray[1] == "Users"
	})).Return(&model.TokenDTO{
		Token:     "test-jwt-token",
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  time.Now().Unix(),
		ExpiresIn: 3600,
		Scopes:    []string{"read", "write"},
		ClientID:  testClientID,
	}, nil)

	result, err := suite.handler.HandleGrant(context.Background(), suite.testTokenReq, oauthAppWithGroups)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "test-jwt-token", result.AccessToken.Token)

	suite.mockAuthzService.AssertExpectations(suite.T())
	suite.mockTokenBuilder.AssertExpectations(suite.T())
}

func (suite *AuthorizationCodeGrantHandlerTestSuite) TestHandleGrant_AttributeCacheFetchError() {
	authzCodeWithCacheID := suite.testAuthzCode
	authzCodeWithCacheID.AttributeCacheID = testCacheID

	suite.mockAuthzService.On("GetAuthorizationCodeDetails", mock.Anything, testClientID, "test-auth-code").
		Return(&authzCodeWithCacheID, nil)

	suite.mockAttrCacheService.On("GetAttributeCache", mock.Anything, testCacheID).
		Return((*attributecache.AttributeCache)(nil), &serviceerror.ServiceError{
			Type:  serviceerror.ServerErrorType,
			Error: core.I18nMessage{DefaultValue: "cache error"},
		})

	result, err := suite.handler.HandleGrant(context.Background(), suite.testTokenReq, suite.oauthApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorServerError, err.Error)
}

// createPKCEApp creates a test OAuth app with PKCE required
func (suite *AuthorizationCodeGrantHandlerTestSuite) createPKCEApp() *inboundmodel.OAuthClient {
	return &inboundmodel.OAuthClient{
		ClientID: testClientID,

		RedirectURIs:            []string{testClientCallbackURL},
		GrantTypes:              []constants.GrantType{constants.GrantTypeAuthorizationCode},
		ResponseTypes:           []constants.ResponseType{constants.ResponseTypeCode},
		TokenEndpointAuthMethod: constants.TokenEndpointAuthMethodClientSecretPost,
		PKCERequired:            true,
	}
}

// createAuthCodeWithPKCE creates an authorization code with PKCE parameters
func (suite *AuthorizationCodeGrantHandlerTestSuite) createAuthCodeWithPKCE() authz.AuthorizationCode {
	authCodeWithPKCE := suite.testAuthzCode
	authCodeWithPKCE.CodeChallenge = testCodeChallenge
	authCodeWithPKCE.CodeChallengeMethod = testCodeChallengeMethodS256
	return authCodeWithPKCE
}

func (suite *AuthorizationCodeGrantHandlerTestSuite) TestRetrieveAndValidateAuthCode_PKCERequiredMissingVerifier() {
	// Test PKCE required but code_verifier missing
	pkceApp := suite.createPKCEApp()
	authCodeWithPKCE := suite.createAuthCodeWithPKCE()

	suite.mockAuthzService.On("GetAuthorizationCodeDetails", mock.Anything, testClientID, "test-auth-code").
		Return(&authCodeWithPKCE, nil)

	tokenReq := *suite.testTokenReq
	tokenReq.CodeVerifier = "" // Missing code verifier

	result, err := suite.handler.HandleGrant(context.Background(), &tokenReq, pkceApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, err.Error)
	assert.Contains(suite.T(), err.ErrorDescription, "code_verifier is required")

	suite.mockAuthzService.AssertExpectations(suite.T())
}

func (suite *AuthorizationCodeGrantHandlerTestSuite) TestRetrieveAndValidateAuthCode_PKCEValidationFailed() {
	// Test PKCE validation failure
	pkceApp := suite.createPKCEApp()
	authCodeWithPKCE := suite.createAuthCodeWithPKCE()

	suite.mockAuthzService.On("GetAuthorizationCodeDetails", mock.Anything, testClientID, "test-auth-code").
		Return(&authCodeWithPKCE, nil)

	tokenReq := *suite.testTokenReq
	tokenReq.CodeVerifier = "wrong-verifier-dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk" // Wrong verifier

	result, err := suite.handler.HandleGrant(context.Background(), &tokenReq, pkceApp)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorInvalidGrant, err.Error)
	assert.Contains(suite.T(), err.ErrorDescription, "Invalid code verifier")

	suite.mockAuthzService.AssertExpectations(suite.T())
}

func (suite *AuthorizationCodeGrantHandlerTestSuite) TestRetrieveAndValidateAuthCode_CodeChallengeNotRequired() {
	authCodeWithPKCE := suite.testAuthzCode
	authCodeWithPKCE.CodeChallenge = testCodeChallenge
	authCodeWithPKCE.CodeChallengeMethod = testCodeChallengeMethodS256

	suite.mockAuthzService.On("GetAuthorizationCodeDetails", mock.Anything, testClientID, "test-auth-code").
		Return(&authCodeWithPKCE, nil)

	// Mock token builder
	suite.mockTokenBuilder.On("BuildAccessToken", mock.Anything).Return(&model.TokenDTO{
		Token:     "test-jwt-token",
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  time.Now().Unix(),
		ExpiresIn: 3600,
		Scopes:    []string{"read", "write"},
		ClientID:  testClientID,
	}, nil)

	tokenReq := *suite.testTokenReq
	tokenReq.CodeVerifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk" // Valid verifier

	result, err := suite.handler.HandleGrant(context.Background(), &tokenReq, suite.oauthApp)

	// Should succeed even though PKCE is not required, because code challenge was provided
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)

	suite.mockAuthzService.AssertExpectations(suite.T())
	suite.mockTokenBuilder.AssertExpectations(suite.T())
}

func (suite *AuthorizationCodeGrantHandlerTestSuite) TestValidateAuthorizationCode_ResourceMismatch() {
	// Test resource parameter mismatch
	authCodeWithResource := suite.testAuthzCode
	authCodeWithResource.Resources = []string{"https://api.example.com/resource"}
	authCodeWithResource.RedirectURI = testClientCallbackURL

	tokenReq := &model.TokenRequest{
		ClientID:    testClientID,
		RedirectURI: "https://client.example.com/callback", // Must match auth code
		Resources:   []string{testResourceURL},             // Different resource
	}

	err := validateAuthorizationCode(tokenReq, authCodeWithResource)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), constants.ErrorInvalidTarget, err.Error)
	assert.Equal(suite.T(), "Resource parameter mismatch", err.ErrorDescription)
}

func (suite *AuthorizationCodeGrantHandlerTestSuite) TestValidateAuthorizationCode_ResourceMatch() {
	// Test resource parameter match
	authCodeWithResource := suite.testAuthzCode
	authCodeWithResource.Resources = []string{testResourceURL}
	authCodeWithResource.RedirectURI = testClientCallbackURL // Must match

	tokenReq := &model.TokenRequest{
		ClientID:    testClientID,
		RedirectURI: "https://client.example.com/callback", // Must match auth code
		Resources:   []string{testResourceURL},             // Matching resource
	}

	err := validateAuthorizationCode(tokenReq, authCodeWithResource)
	assert.Nil(suite.T(), err)
}

func (suite *AuthorizationCodeGrantHandlerTestSuite) TestValidateAuthorizationCode_EmptyResourceInCode() {
	authCodeWithEmptyResource := suite.testAuthzCode
	authCodeWithEmptyResource.Resources = nil
	authCodeWithEmptyResource.RedirectURI = "https://client.example.com/callback" // Must match

	tokenReq := &model.TokenRequest{
		ClientID:    testClientID,
		RedirectURI: "https://client.example.com/callback", // Must match auth code
		Resources:   []string{testResourceURL},             // Any resource should be OK
	}

	err := validateAuthorizationCode(tokenReq, authCodeWithEmptyResource)
	assert.Nil(suite.T(), err)
}

// §3 — token-request resource subset narrows the issued aud (RFC 8707 §2.1).

const testResourceURL2 = "https://api2.example.com/api"

func (suite *AuthorizationCodeGrantHandlerTestSuite) TestHandleGrant_TokenRequestNarrowsResourceSubset() {
	// Authz code has two resources; token request sends only one.
	// Issued aud must contain only the narrowed RS, not both.
	authCodeWithTwoResources := suite.testAuthzCode
	authCodeWithTwoResources.Resources = []string{testResourceURL, testResourceURL2}

	suite.mockResourceService.On("GetResourceServerByIdentifier", mock.Anything, testResourceURL2).
		Return(&resource.ResourceServer{ID: testResourceURL2, Identifier: testResourceURL2}, nil).Maybe()

	suite.mockAuthzService.On("GetAuthorizationCodeDetails", mock.Anything, testClientID, "test-auth-code").
		Return(&authCodeWithTwoResources, nil)

	var capturedAudiences []string
	suite.mockTokenBuilder.On("BuildAccessToken", mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
		capturedAudiences = ctx.Audiences
		return true
	})).Return(&model.TokenDTO{
		Token:     "mock-jwt-token",
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  time.Now().Unix(),
		ExpiresIn: 3600,
		Scopes:    []string{"read", "write"},
		ClientID:  testClientID,
	}, nil)

	tokenReq := *suite.testTokenReq
	tokenReq.Resources = []string{testResourceURL}

	result, err := suite.handler.HandleGrant(context.Background(), &tokenReq, suite.oauthApp)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), []string{testResourceURL}, capturedAudiences)
}

func (suite *AuthorizationCodeGrantHandlerTestSuite) TestHandleGrant_TokenRequestNoResourceUsesBothFromCode() {
	// Authz code has two resources; token request sends no resource.
	// Issued aud must contain both RS identifiers from the auth code.
	authCodeWithTwoResources := suite.testAuthzCode
	authCodeWithTwoResources.Resources = []string{testResourceURL, testResourceURL2}

	suite.mockResourceService.On("GetResourceServerByIdentifier", mock.Anything, testResourceURL2).
		Return(&resource.ResourceServer{ID: testResourceURL2, Identifier: testResourceURL2}, nil).Maybe()

	suite.mockAuthzService.On("GetAuthorizationCodeDetails", mock.Anything, testClientID, "test-auth-code").
		Return(&authCodeWithTwoResources, nil)

	var capturedAudiences []string
	suite.mockTokenBuilder.On("BuildAccessToken", mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
		capturedAudiences = ctx.Audiences
		return true
	})).Return(&model.TokenDTO{
		Token:     "mock-jwt-token",
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  time.Now().Unix(),
		ExpiresIn: 3600,
		Scopes:    []string{"read", "write"},
		ClientID:  testClientID,
	}, nil)

	result, err := suite.handler.HandleGrant(context.Background(), suite.testTokenReq, suite.oauthApp)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), 2, len(capturedAudiences))
	assert.Contains(suite.T(), capturedAudiences, testResourceURL)
	assert.Contains(suite.T(), capturedAudiences, testResourceURL2)
}

func (suite *AuthorizationCodeGrantHandlerTestSuite) TestHandleGrant_TokenRequestNarrows_ScopesDownscoped() {
	// Authz code has two resources [rs1, rs2] with scopes ["read", "write"].
	// Token request narrows to rs1 only; rs1 only supports "read" (ValidatePermissions returns "write" as invalid).
	// Access token must carry only "read" (scope downscoped) and aud=[rs1].
	// OriginalAudiences must carry both [rs1, rs2] for the refresh token.
	authCodeWithTwoResources := suite.testAuthzCode
	authCodeWithTwoResources.Resources = []string{testResourceURL, testResourceURL2}
	authCodeWithTwoResources.Scopes = "read write"

	suite.mockResourceService.ExpectedCalls = nil
	suite.mockResourceService.On("GetResourceServerByIdentifier", mock.Anything, testResourceURL).
		Return(&resource.ResourceServer{ID: testResourceURL, Identifier: testResourceURL}, nil)
	suite.mockResourceService.On("GetResourceServerByIdentifier", mock.Anything, testResourceURL2).
		Return(&resource.ResourceServer{ID: testResourceURL2, Identifier: testResourceURL2}, nil)
	// rs1 only supports "read"; "write" is invalid.
	suite.mockResourceService.On("ValidatePermissions", mock.Anything, testResourceURL, mock.Anything).
		Return([]string{"write"}, nil)
	suite.mockResourceService.On("FindResourceServersByPermissions", mock.Anything, mock.Anything).
		Return([]resource.ResourceServer{}, nil).Maybe()

	suite.mockAuthzService.On("GetAuthorizationCodeDetails", mock.Anything, testClientID, "test-auth-code").
		Return(&authCodeWithTwoResources, nil)

	var capturedAudiences []string
	var capturedScopes []string
	suite.mockTokenBuilder.On("BuildAccessToken", mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
		capturedAudiences = ctx.Audiences
		capturedScopes = ctx.Scopes
		return true
	})).Return(func(ctx *tokenservice.AccessTokenBuildContext) (*model.TokenDTO, error) {
		return &model.TokenDTO{
			Token:     "mock-jwt-token",
			TokenType: constants.TokenTypeBearer,
			IssuedAt:  time.Now().Unix(),
			ExpiresIn: 3600,
			Scopes:    ctx.Scopes,
			ClientID:  testClientID,
		}, nil
	})

	tokenReq := *suite.testTokenReq
	tokenReq.Resources = []string{testResourceURL}

	result, err := suite.handler.HandleGrant(context.Background(), &tokenReq, suite.oauthApp)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), []string{testResourceURL}, capturedAudiences)
	assert.Equal(suite.T(), []string{"read"}, capturedScopes)
	assert.Equal(suite.T(), []string{testResourceURL, testResourceURL2}, result.AccessToken.OriginalAudiences)
}

func (suite *AuthorizationCodeGrantHandlerTestSuite) TestHandleGrant_TokenRequestNarrowsFromImplicitAllRSSet() {
	// When the auth code carried no resources, all registered RSes are implicitly authorized.
	// The token request may then narrow to any registered RS; unknown RS identifiers are rejected
	// upstream by ResolveResourceServers with invalid_target.
	authCodeNoResources := suite.testAuthzCode
	authCodeNoResources.Resources = nil

	suite.mockAuthzService.On("GetAuthorizationCodeDetails", mock.Anything, testClientID, "test-auth-code").
		Return(&authCodeNoResources, nil)

	var capturedAudiences []string
	suite.mockTokenBuilder.On("BuildAccessToken", mock.MatchedBy(func(ctx *tokenservice.AccessTokenBuildContext) bool {
		capturedAudiences = ctx.Audiences
		return true
	})).Return(&model.TokenDTO{
		Token:     "mock-jwt-token",
		TokenType: constants.TokenTypeBearer,
		IssuedAt:  time.Now().Unix(),
		ExpiresIn: 3600,
		Scopes:    []string{"read", "write"},
		ClientID:  testClientID,
	}, nil)

	tokenReq := *suite.testTokenReq
	tokenReq.Resources = []string{testResourceURL}

	result, err := suite.handler.HandleGrant(context.Background(), &tokenReq, suite.oauthApp)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), []string{testResourceURL}, capturedAudiences)
}
