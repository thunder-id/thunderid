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

package userinfo

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/attributecache"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/tests/mocks/attributecachemock"
	"github.com/thunder-id/thunderid/tests/mocks/inboundclientmock"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/tokenservicemock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
)

type UserInfoServiceTestSuite struct {
	suite.Suite
	mockJWTService            *jwtmock.JWTServiceInterfaceMock
	mockTokenValidator        *tokenservicemock.TokenValidatorInterfaceMock
	mockInboundClient         *inboundclientmock.InboundClientServiceInterfaceMock
	mockOUService             *oumock.OrganizationUnitServiceInterfaceMock
	mockAttributeCacheService *attributecachemock.AttributeCacheServiceInterfaceMock
	mockTransactioner         *MockTransactioner
	userInfoService           userInfoServiceInterface
	privateKey                *rsa.PrivateKey
}

// MockTransactioner is a simple implementation of Transactioner for testing.
type MockTransactioner struct{}

func (m *MockTransactioner) Transact(ctx context.Context, txFunc func(context.Context) error) error {
	return txFunc(ctx)
}

func TestUserInfoServiceTestSuite(t *testing.T) {
	suite.Run(t, new(UserInfoServiceTestSuite))
}

func (s *UserInfoServiceTestSuite) SetupTest() {
	s.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(s.T())
	s.mockTokenValidator = tokenservicemock.NewTokenValidatorInterfaceMock(s.T())
	s.mockInboundClient = inboundclientmock.NewInboundClientServiceInterfaceMock(s.T())
	s.mockOUService = oumock.NewOrganizationUnitServiceInterfaceMock(s.T())
	s.mockAttributeCacheService = attributecachemock.NewAttributeCacheServiceInterfaceMock(s.T())
	s.mockTransactioner = &MockTransactioner{}
	s.userInfoService = newUserInfoService(
		s.mockJWTService, nil, nil, s.mockTokenValidator,
		s.mockInboundClient, s.mockOUService,
		s.mockAttributeCacheService, s.mockTransactioner)

	// Initialize server runtime for tests
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime(
		"test-home",
		&config.Config{
			JWT: config.JWTConfig{
				Issuer:         "test-issuer",
				ValidityPeriod: 600,
			},
		},
	)

	// Create a private key for signing JWT tokens
	var err error
	s.privateKey, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		s.T().Fatal("Error generating RSA key:", err)
	}
}

// TestGetUserInfo_EmptyToken tests that empty token returns an error
func (s *UserInfoServiceTestSuite) TestGetUserInfo_EmptyToken() {
	response, svcErr := s.userInfoService.GetUserInfo(context.Background(), "")
	assert.NotNil(s.T(), svcErr)
	assert.Equal(s.T(), errorInvalidAccessToken.Code, svcErr.Code)
	assert.Nil(s.T(), response)
}

// TestGetUserInfo_InvalidTokenSignature tests that invalid token signature returns an error
func (s *UserInfoServiceTestSuite) TestGetUserInfo_InvalidTokenSignature() {
	token := "invalid.token.signature"
	s.mockTokenValidator.On("ValidateAccessToken", token).Return(
		nil, errors.New("invalid signature"))

	response, svcErr := s.userInfoService.GetUserInfo(context.Background(), token)
	assert.NotNil(s.T(), svcErr)
	assert.Equal(s.T(), errorInvalidAccessToken.Code, svcErr.Code)
	assert.Nil(s.T(), response)
	s.mockTokenValidator.AssertExpectations(s.T())
}

// createToken creates a JWT token with the given claims
func (s *UserInfoServiceTestSuite) createToken(claims map[string]interface{}) string {
	header := map[string]interface{}{
		"alg": "RS256",
		"typ": "JWT",
	}

	headerBytes, _ := json.Marshal(header)
	claimsBytes, _ := json.Marshal(claims)

	headerEncoded := base64.RawURLEncoding.EncodeToString(headerBytes)
	claimsEncoded := base64.RawURLEncoding.EncodeToString(claimsBytes)

	signingInput := headerEncoded + "." + claimsEncoded
	hashed := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, s.privateKey, crypto.SHA256, hashed[:])
	if err != nil {
		s.T().Fatal("Error signing token:", err)
	}
	signatureEncoded := base64.RawURLEncoding.EncodeToString(signature)

	return signingInput + "." + signatureEncoded
}

// TestGetUserInfo_InvalidTokenFormat tests that invalid token format returns an error
func (s *UserInfoServiceTestSuite) TestGetUserInfo_InvalidTokenFormat() {
	// nolint:gosec // This is a test token, not a real credential
	invalidToken := "not.a.valid.jwt"
	s.mockTokenValidator.On("ValidateAccessToken", invalidToken).Return(
		nil, errors.New("invalid token format"))

	response, svcErr := s.userInfoService.GetUserInfo(context.Background(), invalidToken)
	assert.NotNil(s.T(), svcErr)
	assert.Equal(s.T(), errorInvalidAccessToken.Code, svcErr.Code)
	assert.Nil(s.T(), response)
	s.mockTokenValidator.AssertExpectations(s.T())
}

// TestGetUserInfo_NoScopes tests that token with no scopes returns insufficient_scope error
func (s *UserInfoServiceTestSuite) TestGetUserInfo_NoScopes() {
	claims := map[string]interface{}{
		"exp": float64(time.Now().Add(time.Hour).Unix()),
		"nbf": float64(time.Now().Add(-time.Minute).Unix()),
		"sub": "user123",
	}
	token := s.createToken(claims)

	s.mockTokenValidator.On("ValidateAccessToken", token).Return(
		&tokenservice.AccessTokenClaims{Sub: "user123", Claims: claims}, nil)

	response, svcErr := s.userInfoService.GetUserInfo(context.Background(), token)
	assert.NotNil(s.T(), svcErr)
	assert.Equal(s.T(), "insufficient_scope", svcErr.Code)
	assert.Nil(s.T(), response)
	s.mockTokenValidator.AssertExpectations(s.T())
}

// TestGetUserInfo_NoScopesEmptyScopeString tests that empty scope string returns insufficient_scope error
func (s *UserInfoServiceTestSuite) TestGetUserInfo_NoScopesEmptyScopeString() {
	claims := map[string]interface{}{
		"exp":   float64(time.Now().Add(time.Hour).Unix()),
		"nbf":   float64(time.Now().Add(-time.Minute).Unix()),
		"sub":   "user123",
		"scope": "",
	}
	token := s.createToken(claims)

	s.mockTokenValidator.On("ValidateAccessToken", token).Return(
		&tokenservice.AccessTokenClaims{Sub: "user123", Claims: claims}, nil)

	response, svcErr := s.userInfoService.GetUserInfo(context.Background(), token)
	assert.NotNil(s.T(), svcErr)
	assert.Equal(s.T(), "insufficient_scope", svcErr.Code)
	assert.Nil(s.T(), response)
	s.mockTokenValidator.AssertExpectations(s.T())
}

// TestGetUserInfo_ErrorFetchingUserAttributes tests error when fetching user attributes fails
func (s *UserInfoServiceTestSuite) TestGetUserInfo_ErrorFetchingUserAttributes() {
	claims := map[string]interface{}{
		"exp":   float64(time.Now().Add(time.Hour).Unix()),
		"nbf":   float64(time.Now().Add(-time.Minute).Unix()),
		"sub":   "user123",
		"scope": "openid profile",
		"aci":   "cache-err-123",
	}
	token := s.createToken(claims)

	s.mockTokenValidator.On("ValidateAccessToken", token).Return(
		&tokenservice.AccessTokenClaims{Sub: "user123", Claims: claims}, nil)
	s.mockAttributeCacheService.On("GetAttributeCache", mock.Anything, "cache-err-123").Return(
		nil, &serviceerror.InternalServerError)

	response, svcErr := s.userInfoService.GetUserInfo(context.Background(), token)
	assert.NotNil(s.T(), svcErr)
	assert.Equal(s.T(), serviceerror.InternalServerError.Code, svcErr.Code)
	assert.Nil(s.T(), response)
	s.mockTokenValidator.AssertExpectations(s.T())
	s.mockAttributeCacheService.AssertExpectations(s.T())
}

// TestGetUserInfo_ErrorFetchingGroups tests error when the attribute cache (which contains groups) cannot be fetched
func (s *UserInfoServiceTestSuite) TestGetUserInfo_ErrorFetchingGroups() {
	claims := map[string]interface{}{
		"exp":       float64(time.Now().Add(time.Hour).Unix()),
		"nbf":       float64(time.Now().Add(-time.Minute).Unix()),
		"sub":       "user123",
		"scope":     "openid profile",
		"client_id": "client123",
		"aci":       "cache-groups-123",
	}
	token := s.createToken(claims)

	oauthApp := &inboundmodel.OAuthClient{
		UserInfo: &inboundmodel.UserInfoConfig{
			UserAttributes: []string{"name", constants.UserAttributeGroups},
		},
	}
	s.mockTokenValidator.On("ValidateAccessToken", token).Return(
		&tokenservice.AccessTokenClaims{Sub: "user123", Claims: claims}, nil)
	s.mockAttributeCacheService.On("GetAttributeCache", mock.Anything, "cache-groups-123").Return(
		nil, &serviceerror.InternalServerError)
	s.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, "client123").Return(oauthApp, nil)

	response, svcErr := s.userInfoService.GetUserInfo(context.Background(), token)
	assert.NotNil(s.T(), svcErr)
	assert.Equal(s.T(), serviceerror.InternalServerError.Code, svcErr.Code)
	assert.Nil(s.T(), response)
	s.mockTokenValidator.AssertExpectations(s.T())
	s.mockAttributeCacheService.AssertExpectations(s.T())
	s.mockInboundClient.AssertExpectations(s.T())
}

// TestGetUserInfo_Success_StandardScopes tests successful response with standard OIDC scopes
func (s *UserInfoServiceTestSuite) TestGetUserInfo_Success_StandardScopes() {
	claims := map[string]interface{}{
		"exp":       float64(time.Now().Add(time.Hour).Unix()),
		"nbf":       float64(time.Now().Add(-time.Minute).Unix()),
		"sub":       "user123",
		"scope":     "openid profile email",
		"client_id": "client123",
		"aci":       "cache-std-123",
	}
	token := s.createToken(claims)

	userAttrs := map[string]interface{}{
		"name":  "John Doe",
		"email": "john@example.com",
	}

	oauthApp := &inboundmodel.OAuthClient{
		Token: &inboundmodel.OAuthTokenConfig{
			IDToken: &inboundmodel.IDTokenConfig{
				UserAttributes: []string{"name", "email"},
			},
		},
		UserInfo: &inboundmodel.UserInfoConfig{
			UserAttributes: []string{"name", "email"},
		},
	}

	s.mockTokenValidator.On("ValidateAccessToken", token).Return(
		&tokenservice.AccessTokenClaims{Sub: "user123", Claims: claims}, nil)
	s.mockAttributeCacheService.On("GetAttributeCache", mock.Anything, "cache-std-123").Return(
		&attributecache.AttributeCache{ID: "cache-std-123", Attributes: userAttrs}, nil)
	s.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, "client123").Return(oauthApp, nil)

	response, svcErr := s.userInfoService.GetUserInfo(context.Background(), token)
	assert.Nil(s.T(), svcErr)
	assert.NotNil(s.T(), response)
	assert.Equal(s.T(), inboundmodel.UserInfoResponseTypeJSON, response.Type)
	assert.Equal(s.T(), "user123", response.JSONBody["sub"])
	assert.Equal(s.T(), "John Doe", response.JSONBody["name"])
	assert.Equal(s.T(), "john@example.com", response.JSONBody["email"])
	s.mockTokenValidator.AssertExpectations(s.T())
	s.mockAttributeCacheService.AssertExpectations(s.T())
	s.mockInboundClient.AssertExpectations(s.T())
}

// TestGetUserInfo_Success_WithGroups tests successful response with groups
func (s *UserInfoServiceTestSuite) TestGetUserInfo_Success_WithGroups() {
	claims := map[string]interface{}{
		"exp":       float64(time.Now().Add(time.Hour).Unix()),
		"nbf":       float64(time.Now().Add(-time.Minute).Unix()),
		"sub":       "user123",
		"scope":     "openid profile",
		"client_id": "client123",
		"aci":       "cache-grp-123",
	}
	token := s.createToken(claims)

	userAttrs := map[string]interface{}{
		"name":                        "John Doe",
		constants.UserAttributeGroups: []interface{}{"admin", "users"},
	}

	oauthApp := &inboundmodel.OAuthClient{
		Token: &inboundmodel.OAuthTokenConfig{
			IDToken: &inboundmodel.IDTokenConfig{
				UserAttributes: []string{"name", constants.UserAttributeGroups},
			},
		},
		UserInfo: &inboundmodel.UserInfoConfig{
			UserAttributes: []string{"name", constants.UserAttributeGroups},
		},
		ScopeClaims: map[string][]string{
			"profile": {"name", constants.UserAttributeGroups}, // Add groups to profile scope
		},
	}

	s.mockTokenValidator.On("ValidateAccessToken", token).Return(
		&tokenservice.AccessTokenClaims{Sub: "user123", Claims: claims}, nil)
	s.mockAttributeCacheService.On("GetAttributeCache", mock.Anything, "cache-grp-123").Return(
		&attributecache.AttributeCache{ID: "cache-grp-123", Attributes: userAttrs}, nil)
	s.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, "client123").Return(oauthApp, nil)

	response, svcErr := s.userInfoService.GetUserInfo(context.Background(), token)
	assert.Nil(s.T(), svcErr)
	assert.NotNil(s.T(), response)
	assert.Equal(s.T(), inboundmodel.UserInfoResponseTypeJSON, response.Type)
	assert.Equal(s.T(), "user123", response.JSONBody["sub"])
	assert.Equal(s.T(), "John Doe", response.JSONBody["name"])
	groupsValue := response.JSONBody[constants.UserAttributeGroups]
	assert.NotNil(s.T(), groupsValue, "groups should be present")
	groups, ok := groupsValue.([]interface{})
	assert.True(s.T(), ok, "groups should be []interface{}")
	assert.Len(s.T(), groups, 2)
	assert.Contains(s.T(), groups, "admin")
	assert.Contains(s.T(), groups, "users")
	s.mockTokenValidator.AssertExpectations(s.T())
	s.mockAttributeCacheService.AssertExpectations(s.T())
	s.mockInboundClient.AssertExpectations(s.T())
}

// TestGetUserInfo_Success_WithScopeClaimsMapping tests successful response with app-specific scope-to-claims mapping
func (s *UserInfoServiceTestSuite) TestGetUserInfo_Success_WithScopeClaimsMapping() {
	claims := map[string]interface{}{
		"exp":       float64(time.Now().Add(time.Hour).Unix()),
		"nbf":       float64(time.Now().Add(-time.Minute).Unix()),
		"sub":       "user123",
		"scope":     "openid custom_scope",
		"client_id": "client123",
		"aci":       "cache-scope-123",
	}
	token := s.createToken(claims)

	userAttrs := map[string]interface{}{
		"name":  "John Doe",
		"email": "john@example.com",
		"phone": "1234567890",
	}

	oauthApp := &inboundmodel.OAuthClient{
		Token: &inboundmodel.OAuthTokenConfig{
			IDToken: &inboundmodel.IDTokenConfig{
				UserAttributes: []string{"name", "email", "phone"},
			},
		},
		UserInfo: &inboundmodel.UserInfoConfig{
			UserAttributes: []string{"name", "email", "phone"},
		},
		ScopeClaims: map[string][]string{
			"custom_scope": {"name", "phone"},
		},
	}

	s.mockTokenValidator.On("ValidateAccessToken", token).Return(
		&tokenservice.AccessTokenClaims{Sub: "user123", Claims: claims}, nil)
	s.mockAttributeCacheService.On("GetAttributeCache", mock.Anything, "cache-scope-123").Return(
		&attributecache.AttributeCache{ID: "cache-scope-123", Attributes: userAttrs}, nil)
	s.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, "client123").Return(oauthApp, nil)

	response, svcErr := s.userInfoService.GetUserInfo(context.Background(), token)
	assert.Nil(s.T(), svcErr)
	assert.NotNil(s.T(), response)
	assert.Equal(s.T(), inboundmodel.UserInfoResponseTypeJSON, response.Type)
	assert.Equal(s.T(), "user123", response.JSONBody["sub"])
	assert.Equal(s.T(), "John Doe", response.JSONBody["name"])
	assert.Equal(s.T(), "1234567890", response.JSONBody["phone"])
	assert.NotContains(s.T(), response.JSONBody, "email") // email not in custom_scope mapping
	s.mockTokenValidator.AssertExpectations(s.T())
	s.mockAttributeCacheService.AssertExpectations(s.T())
	s.mockInboundClient.AssertExpectations(s.T())
}

// TestGetUserInfo_Success_NoAppConfig tests successful response without app config
func (s *UserInfoServiceTestSuite) TestGetUserInfo_Success_NoAppConfig() {
	claims := map[string]interface{}{
		"exp":   float64(time.Now().Add(time.Hour).Unix()),
		"nbf":   float64(time.Now().Add(-time.Minute).Unix()),
		"sub":   "user123",
		"scope": "openid profile",
		"aci":   "cache-noapp-123",
		// No client_id
	}
	token := s.createToken(claims)

	userAttrs := map[string]interface{}{
		"name":  "John Doe",
		"email": "john@example.com",
	}

	s.mockTokenValidator.On("ValidateAccessToken", token).Return(
		&tokenservice.AccessTokenClaims{Sub: "user123", Claims: claims}, nil)
	s.mockAttributeCacheService.On("GetAttributeCache", mock.Anything, "cache-noapp-123").Return(
		&attributecache.AttributeCache{ID: "cache-noapp-123", Attributes: userAttrs}, nil)

	// When no app config, BuildClaims returns empty (no allowedUserAttributes)
	response, svcErr := s.userInfoService.GetUserInfo(context.Background(), token)
	assert.Nil(s.T(), svcErr)
	assert.NotNil(s.T(), response)
	assert.Equal(s.T(), inboundmodel.UserInfoResponseTypeJSON, response.Type)
	assert.Equal(s.T(), "user123", response.JSONBody["sub"])
	// No other claims because allowedUserAttributes is empty
	assert.Len(s.T(), response.JSONBody, 1)
	s.mockTokenValidator.AssertExpectations(s.T())
	s.mockAttributeCacheService.AssertExpectations(s.T())
}

// TestGetUserInfo_AppNotFound_ReturnsInvalidToken tests that a stale/orphaned token returns an error
// when the referenced client application no longer exists.
func (s *UserInfoServiceTestSuite) TestGetUserInfo_AppNotFound_ReturnsInvalidToken() {
	claims := map[string]interface{}{
		"exp":       float64(time.Now().Add(time.Hour).Unix()),
		"nbf":       float64(time.Now().Add(-time.Minute).Unix()),
		"sub":       "user123",
		"scope":     "openid profile",
		"client_id": "client123",
		"aci":       "cache-anf-123",
	}
	token := s.createToken(claims)

	userAttrs := map[string]interface{}{}

	s.mockTokenValidator.On("ValidateAccessToken", token).Return(
		&tokenservice.AccessTokenClaims{Sub: "user123", Claims: claims}, nil)
	s.mockAttributeCacheService.On("GetAttributeCache", mock.Anything, "cache-anf-123").Return(
		&attributecache.AttributeCache{ID: "cache-anf-123", Attributes: userAttrs}, nil)
	s.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, "client123").
		Return(nil, errors.New("app not found"))

	response, svcErr := s.userInfoService.GetUserInfo(context.Background(), token)
	assert.Nil(s.T(), svcErr)
	assert.NotNil(s.T(), response)
	assert.Equal(s.T(), inboundmodel.UserInfoResponseTypeJSON, response.Type)
	assert.Equal(s.T(), "user123", response.JSONBody["sub"])
	// No other claims because allowedUserAttributes is empty
	assert.Len(s.T(), response.JSONBody, 1)
	s.mockTokenValidator.AssertExpectations(s.T())
	s.mockAttributeCacheService.AssertExpectations(s.T())
	s.mockInboundClient.AssertExpectations(s.T())
}

// TestGetUserInfo_Success_GroupsNotInAllowedAttributes tests that groups are not included if not in allowed attributes
func (s *UserInfoServiceTestSuite) TestGetUserInfo_Success_GroupsNotInAllowedAttributes() {
	claims := map[string]interface{}{
		"exp":       float64(time.Now().Add(time.Hour).Unix()),
		"nbf":       float64(time.Now().Add(-time.Minute).Unix()),
		"sub":       "user123",
		"scope":     "openid profile",
		"client_id": "client123",
		"aci":       "cache-gnaa-123",
	}
	token := s.createToken(claims)

	userAttrs := map[string]interface{}{
		"name": "John Doe",
	}

	oauthApp := &inboundmodel.OAuthClient{
		Token: &inboundmodel.OAuthTokenConfig{
			IDToken: &inboundmodel.IDTokenConfig{
				UserAttributes: []string{"name"}, // groups not in allowed attributes
			},
		},
		UserInfo: &inboundmodel.UserInfoConfig{
			UserAttributes: []string{"name"},
		},
	}

	s.mockTokenValidator.On("ValidateAccessToken", token).Return(
		&tokenservice.AccessTokenClaims{Sub: "user123", Claims: claims}, nil)
	s.mockAttributeCacheService.On("GetAttributeCache", mock.Anything, "cache-gnaa-123").Return(
		&attributecache.AttributeCache{ID: "cache-gnaa-123", Attributes: userAttrs}, nil)
	s.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, "client123").Return(oauthApp, nil)

	response, svcErr := s.userInfoService.GetUserInfo(context.Background(), token)
	assert.Nil(s.T(), svcErr)
	assert.NotNil(s.T(), response)
	assert.Equal(s.T(), inboundmodel.UserInfoResponseTypeJSON, response.Type)
	assert.Equal(s.T(), "user123", response.JSONBody["sub"])
	assert.Equal(s.T(), "John Doe", response.JSONBody["name"])
	assert.NotContains(s.T(), response.JSONBody, constants.UserAttributeGroups) // groups not included
	s.mockTokenValidator.AssertExpectations(s.T())
	s.mockAttributeCacheService.AssertExpectations(s.T())
	s.mockInboundClient.AssertExpectations(s.T())
}

// TestGetUserInfo_Success_EmptyUserAttributes tests successful response when no attribute cache key is present,
// meaning the user has no cached attributes.
func (s *UserInfoServiceTestSuite) TestGetUserInfo_Success_EmptyUserAttributes() {
	claims := map[string]interface{}{
		"exp":       float64(time.Now().Add(time.Hour).Unix()),
		"nbf":       float64(time.Now().Add(-time.Minute).Unix()),
		"sub":       "user123",
		"scope":     "openid profile",
		"client_id": "client123",
		// No "aci" claim — user has no cached attributes
	}
	token := s.createToken(claims)

	oauthApp := &inboundmodel.OAuthClient{
		Token: &inboundmodel.OAuthTokenConfig{
			IDToken: &inboundmodel.IDTokenConfig{
				UserAttributes: []string{"name", "email"},
			},
		},
		UserInfo: &inboundmodel.UserInfoConfig{
			UserAttributes: []string{"name", "email"},
		},
	}

	s.mockTokenValidator.On("ValidateAccessToken", token).Return(
		&tokenservice.AccessTokenClaims{Sub: "user123", Claims: claims}, nil)
	s.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, "client123").Return(oauthApp, nil)

	response, svcErr := s.userInfoService.GetUserInfo(context.Background(), token)
	assert.Nil(s.T(), svcErr)
	assert.NotNil(s.T(), response)
	assert.Equal(s.T(), inboundmodel.UserInfoResponseTypeJSON, response.Type)
	assert.Equal(s.T(), "user123", response.JSONBody["sub"])
	// No other claims because user has no cached attributes
	assert.Len(s.T(), response.JSONBody, 1)
	s.mockTokenValidator.AssertExpectations(s.T())
	s.mockAttributeCacheService.AssertExpectations(s.T())
	s.mockInboundClient.AssertExpectations(s.T())
}

// TestGetUserInfo_Success_ScopeAsNonString tests that non-string scope returns insufficient_scope error
func (s *UserInfoServiceTestSuite) TestGetUserInfo_Success_ScopeAsNonString() {
	claims := map[string]interface{}{
		"exp":   float64(time.Now().Add(time.Hour).Unix()),
		"nbf":   float64(time.Now().Add(-time.Minute).Unix()),
		"sub":   "user123",
		"scope": 123, // Invalid type
	}
	token := s.createToken(claims)

	s.mockTokenValidator.On("ValidateAccessToken", token).Return(
		&tokenservice.AccessTokenClaims{Sub: "user123", Claims: claims}, nil)

	response, svcErr := s.userInfoService.GetUserInfo(context.Background(), token)
	assert.NotNil(s.T(), svcErr)
	assert.Equal(s.T(), "insufficient_scope", svcErr.Code)
	assert.Nil(s.T(), response)
	s.mockTokenValidator.AssertExpectations(s.T())
}

// TestGetUserInfo_ScopeExistsButNotString tests when scope exists but is not a string returns insufficient_scope error
func (s *UserInfoServiceTestSuite) TestGetUserInfo_ScopeExistsButNotString() {
	claims := map[string]interface{}{
		"exp":   float64(time.Now().Add(time.Hour).Unix()),
		"nbf":   float64(time.Now().Add(-time.Minute).Unix()),
		"sub":   "user123",
		"scope": []string{"openid"}, // Scope as array instead of string
	}
	token := s.createToken(claims)

	s.mockTokenValidator.On("ValidateAccessToken", token).Return(
		&tokenservice.AccessTokenClaims{Sub: "user123", Claims: claims}, nil)

	response, svcErr := s.userInfoService.GetUserInfo(context.Background(), token)
	assert.NotNil(s.T(), svcErr)
	assert.Equal(s.T(), "insufficient_scope", svcErr.Code)
	assert.Nil(s.T(), response)
	s.mockTokenValidator.AssertExpectations(s.T())
}

// testGetUserInfoInvalidClientID is a helper function for testing invalid client_id scenarios
func (s *UserInfoServiceTestSuite) testGetUserInfoInvalidClientID(clientIDValue interface{}, description string) {
	claims := map[string]interface{}{
		"exp":       float64(time.Now().Add(time.Hour).Unix()),
		"nbf":       float64(time.Now().Add(-time.Minute).Unix()),
		"sub":       "user123",
		"scope":     "openid profile",
		"client_id": clientIDValue,
		"aci":       "cache-inv-cid-123",
	}
	token := s.createToken(claims)

	userAttrs := map[string]interface{}{
		"name": "John Doe",
	}

	s.mockTokenValidator.On("ValidateAccessToken", token).Return(
		&tokenservice.AccessTokenClaims{Sub: "user123", Claims: claims}, nil)
	s.mockAttributeCacheService.On("GetAttributeCache", mock.Anything, "cache-inv-cid-123").Return(
		&attributecache.AttributeCache{ID: "cache-inv-cid-123", Attributes: userAttrs}, nil)

	// When client_id is invalid, app lookup is skipped
	response, svcErr := s.userInfoService.GetUserInfo(context.Background(), token)
	assert.Nil(s.T(), svcErr, description)
	assert.NotNil(s.T(), response, description)
	assert.Equal(s.T(), inboundmodel.UserInfoResponseTypeJSON, response.Type, description)
	assert.Equal(s.T(), "user123", response.JSONBody["sub"], description)
	// No other claims because allowedUserAttributes is empty
	assert.Len(s.T(), response.JSONBody, 1, description)
	s.mockTokenValidator.AssertExpectations(s.T())
	s.mockAttributeCacheService.AssertExpectations(s.T())
}

// TestGetUserInfo_ClientIDNotString tests when client_id exists but is not a string
func (s *UserInfoServiceTestSuite) TestGetUserInfo_ClientIDNotString() {
	s.testGetUserInfoInvalidClientID(123, "When client_id is not a string, app lookup is skipped")
}

// TestGetUserInfo_ClientIDEmptyString tests when client_id is empty string
func (s *UserInfoServiceTestSuite) TestGetUserInfo_ClientIDEmptyString() {
	s.testGetUserInfoInvalidClientID("", "When client_id is empty, app lookup is skipped")
}

// TestGetUserInfo_GroupsWithNilOAuthApp tests groups when oauthApp is nil
func (s *UserInfoServiceTestSuite) TestGetUserInfo_GroupsWithNilOAuthApp() {
	claims := map[string]interface{}{
		"exp":   float64(time.Now().Add(time.Hour).Unix()),
		"nbf":   float64(time.Now().Add(-time.Minute).Unix()),
		"sub":   "user123",
		"scope": "openid profile",
		"aci":   "cache-nil-app-123",
		// No client_id
	}
	token := s.createToken(claims)

	userAttrs := map[string]interface{}{
		"name": "John Doe",
	}

	s.mockTokenValidator.On("ValidateAccessToken", token).Return(
		&tokenservice.AccessTokenClaims{Sub: "user123", Claims: claims}, nil)
	s.mockAttributeCacheService.On("GetAttributeCache", mock.Anything, "cache-nil-app-123").Return(
		&attributecache.AttributeCache{ID: "cache-nil-app-123", Attributes: userAttrs}, nil)
	response, svcErr := s.userInfoService.GetUserInfo(context.Background(), token)
	assert.Nil(s.T(), svcErr)
	assert.NotNil(s.T(), response)
	assert.Equal(s.T(), inboundmodel.UserInfoResponseTypeJSON, response.Type)
	assert.Equal(s.T(), "user123", response.JSONBody["sub"])
	// Groups not included because oauthApp is nil
	assert.NotContains(s.T(), response.JSONBody, constants.UserAttributeGroups)
	s.mockTokenValidator.AssertExpectations(s.T())
	s.mockAttributeCacheService.AssertExpectations(s.T())
}

// TestGetUserInfo_GroupsWithNilToken tests groups when Token is nil
func (s *UserInfoServiceTestSuite) TestGetUserInfo_GroupsWithNilToken() {
	claims := map[string]interface{}{
		"exp":       float64(time.Now().Add(time.Hour).Unix()),
		"nbf":       float64(time.Now().Add(-time.Minute).Unix()),
		"sub":       "user123",
		"scope":     "openid profile",
		"client_id": "client123",
		"aci":       "cache-nil-tok-123",
	}
	token := s.createToken(claims)

	userAttrs := map[string]interface{}{
		"name": "John Doe",
	}

	oauthApp := &inboundmodel.OAuthClient{
		Token: nil, // Token is nil
	}

	s.mockTokenValidator.On("ValidateAccessToken", token).Return(
		&tokenservice.AccessTokenClaims{Sub: "user123", Claims: claims}, nil)
	s.mockAttributeCacheService.On("GetAttributeCache", mock.Anything, "cache-nil-tok-123").Return(
		&attributecache.AttributeCache{ID: "cache-nil-tok-123", Attributes: userAttrs}, nil)
	s.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, "client123").Return(oauthApp, nil)

	// When Token is nil, groups are not added
	response, svcErr := s.userInfoService.GetUserInfo(context.Background(), token)
	assert.Nil(s.T(), svcErr)
	assert.NotNil(s.T(), response)
	assert.Equal(s.T(), inboundmodel.UserInfoResponseTypeJSON, response.Type)
	assert.Equal(s.T(), "user123", response.JSONBody["sub"])
	// Groups not included because Token is nil
	assert.NotContains(s.T(), response.JSONBody, constants.UserAttributeGroups)
	s.mockTokenValidator.AssertExpectations(s.T())
	s.mockAttributeCacheService.AssertExpectations(s.T())
	s.mockInboundClient.AssertExpectations(s.T())
}

// TestGetUserInfo_GroupsWithNilIDToken tests groups when IDToken is nil
func (s *UserInfoServiceTestSuite) TestGetUserInfo_GroupsWithNilIDToken() {
	claims := map[string]interface{}{
		"exp":       float64(time.Now().Add(time.Hour).Unix()),
		"nbf":       float64(time.Now().Add(-time.Minute).Unix()),
		"sub":       "user123",
		"scope":     "openid profile",
		"client_id": "client123",
		"aci":       "cache-nil-idt-123",
	}
	token := s.createToken(claims)

	userAttrs := map[string]interface{}{
		"name": "John Doe",
	}

	oauthApp := &inboundmodel.OAuthClient{
		Token: &inboundmodel.OAuthTokenConfig{
			IDToken: nil, // IDToken is nil
		},
	}

	s.mockTokenValidator.On("ValidateAccessToken", token).Return(
		&tokenservice.AccessTokenClaims{Sub: "user123", Claims: claims}, nil)
	s.mockAttributeCacheService.On("GetAttributeCache", mock.Anything, "cache-nil-idt-123").Return(
		&attributecache.AttributeCache{ID: "cache-nil-idt-123", Attributes: userAttrs}, nil)
	s.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, "client123").Return(oauthApp, nil)

	// When IDToken is nil, groups are not added
	response, svcErr := s.userInfoService.GetUserInfo(context.Background(), token)
	assert.Nil(s.T(), svcErr)
	assert.NotNil(s.T(), response)
	assert.Equal(s.T(), inboundmodel.UserInfoResponseTypeJSON, response.Type)
	assert.Equal(s.T(), "user123", response.JSONBody["sub"])
	// Groups not included because IDToken is nil
	assert.NotContains(s.T(), response.JSONBody, constants.UserAttributeGroups)
	s.mockTokenValidator.AssertExpectations(s.T())
	s.mockAttributeCacheService.AssertExpectations(s.T())
	s.mockInboundClient.AssertExpectations(s.T())
}

// TestGetUserInfo_GroupsWithEmptyGroups tests when groups list is empty
func (s *UserInfoServiceTestSuite) TestGetUserInfo_GroupsWithEmptyGroups() {
	claims := map[string]interface{}{
		"exp":       float64(time.Now().Add(time.Hour).Unix()),
		"nbf":       float64(time.Now().Add(-time.Minute).Unix()),
		"sub":       "user123",
		"scope":     "openid profile",
		"client_id": "client123",
		"aci":       "cache-eg-123",
	}
	token := s.createToken(claims)

	userAttrs := map[string]interface{}{
		"name": "John Doe",
	}

	oauthApp := &inboundmodel.OAuthClient{
		Token: &inboundmodel.OAuthTokenConfig{
			IDToken: &inboundmodel.IDTokenConfig{
				UserAttributes: []string{"name", constants.UserAttributeGroups},
			},
		},
		UserInfo: &inboundmodel.UserInfoConfig{
			UserAttributes: []string{"name", constants.UserAttributeGroups},
		},
		ScopeClaims: map[string][]string{
			"profile": {"name", constants.UserAttributeGroups},
		},
	}

	s.mockTokenValidator.On("ValidateAccessToken", token).Return(
		&tokenservice.AccessTokenClaims{Sub: "user123", Claims: claims}, nil)
	s.mockAttributeCacheService.On("GetAttributeCache", mock.Anything, "cache-eg-123").Return(
		&attributecache.AttributeCache{ID: "cache-eg-123", Attributes: userAttrs}, nil)
	s.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, "client123").Return(oauthApp, nil)

	// When the cache has no groups key, groups are not added to userAttributes
	response, svcErr := s.userInfoService.GetUserInfo(context.Background(), token)
	assert.Nil(s.T(), svcErr)
	assert.NotNil(s.T(), response)
	assert.Equal(s.T(), inboundmodel.UserInfoResponseTypeJSON, response.Type)
	assert.Equal(s.T(), "user123", response.JSONBody["sub"])
	assert.Equal(s.T(), "John Doe", response.JSONBody["name"])
	// Groups not included because the attribute cache has no groups entry
	assert.NotContains(s.T(), response.JSONBody, constants.UserAttributeGroups)
	s.mockTokenValidator.AssertExpectations(s.T())
	s.mockAttributeCacheService.AssertExpectations(s.T())
	s.mockInboundClient.AssertExpectations(s.T())
}

// TestGetUserInfo_ClientCredentialsGrant_Rejected tests that client_credentials grant is rejected
func (s *UserInfoServiceTestSuite) TestGetUserInfo_ClientCredentialsGrant_Rejected() {
	claims := map[string]interface{}{
		"exp":        float64(time.Now().Add(time.Hour).Unix()),
		"nbf":        float64(time.Now().Add(-time.Minute).Unix()),
		"sub":        "client123",
		"scope":      "read write",
		"grant_type": "client_credentials",
		"client_id":  "client123",
	}
	token := s.createToken(claims)

	s.mockTokenValidator.On("ValidateAccessToken", token).Return(
		&tokenservice.AccessTokenClaims{Sub: "client123", Claims: claims}, nil)

	response, svcErr := s.userInfoService.GetUserInfo(context.Background(), token)
	assert.NotNil(s.T(), svcErr)
	assert.Equal(s.T(), errorClientCredentialsNotSupported.Code, svcErr.Code)
	assert.Equal(s.T(), errorClientCredentialsNotSupported.ErrorDescription.DefaultValue,
		svcErr.ErrorDescription.DefaultValue)
	assert.Nil(s.T(), response)
	s.mockTokenValidator.AssertExpectations(s.T())
}

// testGetUserInfoAllowedGrantType is a helper function for testing allowed grant types
func (s *UserInfoServiceTestSuite) testGetUserInfoAllowedGrantType(grantTypeValue interface{}, description string) {
	claims := map[string]interface{}{
		"exp":       float64(time.Now().Add(time.Hour).Unix()),
		"nbf":       float64(time.Now().Add(-time.Minute).Unix()),
		"sub":       "user123",
		"scope":     "openid profile",
		"client_id": "client123",
		"aci":       "cache-agt-123",
	}
	if grantTypeValue != nil {
		claims["grant_type"] = grantTypeValue
	}
	token := s.createToken(claims)

	userAttrs := map[string]interface{}{
		"name": "John Doe",
	}

	oauthApp := &inboundmodel.OAuthClient{
		Token: &inboundmodel.OAuthTokenConfig{
			IDToken: &inboundmodel.IDTokenConfig{
				UserAttributes: []string{"name"},
			},
		},
		UserInfo: &inboundmodel.UserInfoConfig{
			UserAttributes: []string{"name"},
		},
	}

	s.mockTokenValidator.On("ValidateAccessToken", token).Return(
		&tokenservice.AccessTokenClaims{Sub: "user123", Claims: claims}, nil)
	s.mockAttributeCacheService.On("GetAttributeCache", mock.Anything, "cache-agt-123").Return(
		&attributecache.AttributeCache{ID: "cache-agt-123", Attributes: userAttrs}, nil)
	s.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, "client123").Return(oauthApp, nil)

	response, svcErr := s.userInfoService.GetUserInfo(context.Background(), token)
	assert.Nil(s.T(), svcErr, description)
	assert.NotNil(s.T(), response, description)
	assert.Equal(s.T(), inboundmodel.UserInfoResponseTypeJSON, response.Type, description)
	assert.Equal(s.T(), "user123", response.JSONBody["sub"], description)
	assert.Equal(s.T(), "John Doe", response.JSONBody["name"], description)
	s.mockTokenValidator.AssertExpectations(s.T())
	s.mockAttributeCacheService.AssertExpectations(s.T())
	s.mockInboundClient.AssertExpectations(s.T())
}

// TestGetUserInfo_AuthorizationCodeGrant_Allowed tests that authorization_code grant is allowed
func (s *UserInfoServiceTestSuite) TestGetUserInfo_AuthorizationCodeGrant_Allowed() {
	s.testGetUserInfoAllowedGrantType("authorization_code", "authorization_code grant should be allowed")
}

// TestGetUserInfo_RefreshTokenGrant_Allowed tests that refresh_token grant is allowed
func (s *UserInfoServiceTestSuite) TestGetUserInfo_RefreshTokenGrant_Allowed() {
	s.testGetUserInfoAllowedGrantType("refresh_token", "refresh_token grant should be allowed")
}

// TestGetUserInfo_TokenExchangeGrant_Allowed tests that token_exchange grant is allowed
func (s *UserInfoServiceTestSuite) TestGetUserInfo_TokenExchangeGrant_Allowed() {
	s.testGetUserInfoAllowedGrantType(
		"urn:ietf:params:oauth:grant-type:token-exchange",
		"token_exchange grant should be allowed")
}

// TestGetUserInfo_NoGrantType_Allowed tests that tokens without grant_type claim are allowed (backward compatibility)
func (s *UserInfoServiceTestSuite) TestGetUserInfo_NoGrantType_Allowed() {
	s.testGetUserInfoAllowedGrantType(nil, "tokens without grant_type should be allowed")
}

// TestGetUserInfo_GrantTypeNotString_Allowed tests that non-string grant_type is ignored and allowed
func (s *UserInfoServiceTestSuite) TestGetUserInfo_GrantTypeNotString_Allowed() {
	s.testGetUserInfoAllowedGrantType(123, "non-string grant_type should be ignored and allowed")
}

// TestGetUserInfo_MissingOpenIDScope_WithOtherScopes tests that missing openid scope returns insufficient_scope error
func (s *UserInfoServiceTestSuite) TestGetUserInfo_MissingOpenIDScope_WithOtherScopes() {
	claims := map[string]interface{}{
		"exp":   float64(time.Now().Add(time.Hour).Unix()),
		"nbf":   float64(time.Now().Add(-time.Minute).Unix()),
		"sub":   "user123",
		"scope": "profile email", // Missing 'openid' scope
	}
	token := s.createToken(claims)

	s.mockTokenValidator.On("ValidateAccessToken", token).Return(
		&tokenservice.AccessTokenClaims{Sub: "user123", Claims: claims}, nil)

	response, svcErr := s.userInfoService.GetUserInfo(context.Background(), token)
	assert.NotNil(s.T(), svcErr)
	assert.Equal(s.T(), "insufficient_scope", svcErr.Code)
	assert.Contains(s.T(), svcErr.ErrorDescription.DefaultValue, "openid")
	assert.Nil(s.T(), response)
	s.mockTokenValidator.AssertExpectations(s.T())
}

// TestGetUserInfo_OpenIDScope_CaseSensitive tests that scope matching is case-sensitive
func (s *UserInfoServiceTestSuite) TestGetUserInfo_OpenIDScope_CaseSensitive() {
	claims := map[string]interface{}{
		"exp":   float64(time.Now().Add(time.Hour).Unix()),
		"nbf":   float64(time.Now().Add(-time.Minute).Unix()),
		"sub":   "user123",
		"scope": "OpenID profile", // Wrong case - should fail
	}
	token := s.createToken(claims)

	s.mockTokenValidator.On("ValidateAccessToken", token).Return(
		&tokenservice.AccessTokenClaims{Sub: "user123", Claims: claims}, nil)

	response, svcErr := s.userInfoService.GetUserInfo(context.Background(), token)
	assert.NotNil(s.T(), svcErr)
	assert.Equal(s.T(), "insufficient_scope", svcErr.Code)
	assert.Nil(s.T(), response)
	s.mockTokenValidator.AssertExpectations(s.T())
}

// TestGetUserInfo_OnlyOpenIDScope_Success tests that only openid scope returns sub claim
func (s *UserInfoServiceTestSuite) TestGetUserInfo_OnlyOpenIDScope_Success() {
	claims := map[string]interface{}{
		"exp":   float64(time.Now().Add(time.Hour).Unix()),
		"nbf":   float64(time.Now().Add(-time.Minute).Unix()),
		"sub":   "user123",
		"scope": "openid", // Only openid scope
		"aci":   "cache-oid-only-123",
	}
	token := s.createToken(claims)

	s.mockTokenValidator.On("ValidateAccessToken", token).Return(
		&tokenservice.AccessTokenClaims{Sub: "user123", Claims: claims}, nil)
	s.mockAttributeCacheService.On("GetAttributeCache", mock.Anything, "cache-oid-only-123").Return(
		&attributecache.AttributeCache{ID: "cache-oid-only-123", Attributes: map[string]interface{}{}}, nil)

	response, svcErr := s.userInfoService.GetUserInfo(context.Background(), token)
	assert.Nil(s.T(), svcErr)
	assert.NotNil(s.T(), response)
	assert.Equal(s.T(), inboundmodel.UserInfoResponseTypeJSON, response.Type)
	assert.Equal(s.T(), "user123", response.JSONBody["sub"])
	// Only sub claim should be present
	assert.Len(s.T(), response.JSONBody, 1)
	s.mockTokenValidator.AssertExpectations(s.T())
	s.mockAttributeCacheService.AssertExpectations(s.T())
}

// TestGetUserInfo_OpenIDScope_InMiddleOfScopeString tests openid scope in middle position
func (s *UserInfoServiceTestSuite) TestGetUserInfo_OpenIDScope_InMiddleOfScopeString() {
	claims := map[string]interface{}{
		"exp":       float64(time.Now().Add(time.Hour).Unix()),
		"nbf":       float64(time.Now().Add(-time.Minute).Unix()),
		"sub":       "user123",
		"scope":     "profile openid email", // openid in middle
		"client_id": "client123",
		"aci":       "cache-mid-123",
	}
	token := s.createToken(claims)

	userAttrs := map[string]interface{}{
		"name":  "John Doe",
		"email": "john@example.com",
	}

	oauthApp := &inboundmodel.OAuthClient{
		UserInfo: &inboundmodel.UserInfoConfig{
			UserAttributes: []string{"name", "email"},
		},
	}

	s.mockTokenValidator.On("ValidateAccessToken", token).Return(
		&tokenservice.AccessTokenClaims{Sub: "user123", Claims: claims}, nil)
	s.mockAttributeCacheService.On("GetAttributeCache", mock.Anything, "cache-mid-123").Return(
		&attributecache.AttributeCache{ID: "cache-mid-123", Attributes: userAttrs}, nil)
	s.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, "client123").Return(oauthApp, nil)

	response, svcErr := s.userInfoService.GetUserInfo(context.Background(), token)
	assert.Nil(s.T(), svcErr)
	assert.NotNil(s.T(), response)
	assert.Equal(s.T(), inboundmodel.UserInfoResponseTypeJSON, response.Type)
	assert.Equal(s.T(), "user123", response.JSONBody["sub"])
	assert.Equal(s.T(), "John Doe", response.JSONBody["name"])
	assert.Equal(s.T(), "john@example.com", response.JSONBody["email"])
	s.mockTokenValidator.AssertExpectations(s.T())
	s.mockAttributeCacheService.AssertExpectations(s.T())
	s.mockInboundClient.AssertExpectations(s.T())
}

// TestGetUserInfo_OpenIDScope_AtEnd tests openid scope at end of scope string
func (s *UserInfoServiceTestSuite) TestGetUserInfo_OpenIDScope_AtEnd() {
	claims := map[string]interface{}{
		"exp":       float64(time.Now().Add(time.Hour).Unix()),
		"nbf":       float64(time.Now().Add(-time.Minute).Unix()),
		"sub":       "user123",
		"scope":     "profile email openid", // openid at end
		"client_id": "client123",
		"aci":       "cache-end-123",
	}
	token := s.createToken(claims)

	userAttrs := map[string]interface{}{
		"email": "john@example.com",
	}

	oauthApp := &inboundmodel.OAuthClient{
		UserInfo: &inboundmodel.UserInfoConfig{
			UserAttributes: []string{"email"},
		},
	}

	s.mockTokenValidator.On("ValidateAccessToken", token).Return(
		&tokenservice.AccessTokenClaims{Sub: "user123", Claims: claims}, nil)
	s.mockAttributeCacheService.On("GetAttributeCache", mock.Anything, "cache-end-123").Return(
		&attributecache.AttributeCache{ID: "cache-end-123", Attributes: userAttrs}, nil)
	s.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, "client123").Return(oauthApp, nil)

	response, svcErr := s.userInfoService.GetUserInfo(context.Background(), token)
	assert.Nil(s.T(), svcErr)
	assert.NotNil(s.T(), response)
	assert.Equal(s.T(), inboundmodel.UserInfoResponseTypeJSON, response.Type)
	assert.NotNil(s.T(), response.JSONBody)
	assert.Equal(s.T(), "user123", response.JSONBody["sub"])
	assert.Equal(s.T(), "john@example.com", response.JSONBody["email"])
	s.mockTokenValidator.AssertExpectations(s.T())
	s.mockAttributeCacheService.AssertExpectations(s.T())
	s.mockInboundClient.AssertExpectations(s.T())
}

// TestGetUserInfo_JWS_ResponseType tests that when the OAuth application
// is configured with UserInfo response_type as JWS, the service generates
// and returns a signed JWT response instead of JSON.
func (s *UserInfoServiceTestSuite) TestGetUserInfo_JWS_ResponseType() {
	claims := map[string]interface{}{
		"exp":       float64(time.Now().Add(time.Hour).Unix()),
		"nbf":       float64(time.Now().Add(-time.Minute).Unix()),
		"sub":       "user123",
		"scope":     "openid email",
		"client_id": "client123",
		"aci":       "cache-jws-123",
	}
	token := s.createToken(claims)

	userAttrs := map[string]interface{}{
		"email": "john@example.com",
	}

	oauthApp := &inboundmodel.OAuthClient{
		Token: &inboundmodel.OAuthTokenConfig{},
		UserInfo: &inboundmodel.UserInfoConfig{
			ResponseType:   inboundmodel.UserInfoResponseTypeJWS,
			SigningAlg:     "RS256",
			UserAttributes: []string{"email"},
		},
	}

	issuer := "test-issuer"

	// JWT verification
	s.mockTokenValidator.On("ValidateAccessToken", token).Return(
		&tokenservice.AccessTokenClaims{Sub: "user123", Claims: claims}, nil)

	// Attribute cache fetch
	s.mockAttributeCacheService.On("GetAttributeCache", mock.Anything, "cache-jws-123").Return(
		&attributecache.AttributeCache{ID: "cache-jws-123", Attributes: userAttrs}, nil)

	// App fetch
	s.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, "client123").Return(oauthApp, nil)

	// JWT generation
	s.mockJWTService.On(
		"GenerateJWT",
		mock.Anything,
		"user123",
		issuer,
		config.GetServerRuntime().Config.JWT.ValidityPeriod,
		mock.Anything,
		mock.Anything,
		"RS256",
	).Return("signed.jwt.token", int64(0), nil)

	response, svcErr := s.userInfoService.GetUserInfo(context.Background(), token)

	assert.Nil(s.T(), svcErr)
	assert.NotNil(s.T(), response)
	assert.Equal(s.T(), inboundmodel.UserInfoResponseTypeJWS, response.Type)
	assert.Equal(s.T(), "signed.jwt.token", response.JWTBody)
	assert.Nil(s.T(), response.JSONBody)

	s.mockTokenValidator.AssertExpectations(s.T())
	s.mockJWTService.AssertExpectations(s.T())
	s.mockAttributeCacheService.AssertExpectations(s.T())
	s.mockInboundClient.AssertExpectations(s.T())
}

// TestGetUserInfo_JWS_GenerateJWTFailure tests that
// an internal server error is returned when JWT generation fails.
func (s *UserInfoServiceTestSuite) TestGetUserInfo_JWS_GenerateJWTFailure() {
	claims := map[string]interface{}{
		"exp":       float64(time.Now().Add(time.Hour).Unix()),
		"nbf":       float64(time.Now().Add(-time.Minute).Unix()),
		"sub":       "user123",
		"scope":     "openid email",
		"client_id": "client123",
		"aci":       "cache-jws-fail-123",
	}
	token := s.createToken(claims)

	userAttrs := map[string]interface{}{
		"email": "john@example.com",
	}

	oauthApp := &inboundmodel.OAuthClient{
		Token: &inboundmodel.OAuthTokenConfig{},
		UserInfo: &inboundmodel.UserInfoConfig{
			ResponseType:   inboundmodel.UserInfoResponseTypeJWS,
			SigningAlg:     "RS256",
			UserAttributes: []string{"email"},
		},
	}
	issuer := "test-issuer"

	s.mockTokenValidator.On("ValidateAccessToken", token).Return(
		&tokenservice.AccessTokenClaims{Sub: "user123", Claims: claims}, nil)

	s.mockAttributeCacheService.On("GetAttributeCache", mock.Anything, "cache-jws-fail-123").Return(
		&attributecache.AttributeCache{ID: "cache-jws-fail-123", Attributes: userAttrs}, nil)

	s.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, "client123").Return(oauthApp, nil)

	// Simulate signing failure
	s.mockJWTService.On(
		"GenerateJWT",
		mock.Anything,
		"user123",
		issuer,
		config.GetServerRuntime().Config.JWT.ValidityPeriod,
		mock.Anything,
		mock.Anything,
		"RS256",
	).Return("", int64(0),
		&serviceerror.ServiceError{
			Type: serviceerror.ServerErrorType,
			Code: "JWT_SIGNING_FAILED",
			Error: core.I18nMessage{
				Key: "error.test.jwt_signing_failed", DefaultValue: "JWT signing failed",
			},
			ErrorDescription: core.I18nMessage{
				Key: "error.test.jwt_signing_failed", DefaultValue: "JWT signing failed",
			},
		})

	response, svcErr := s.userInfoService.GetUserInfo(context.Background(), token)

	assert.Nil(s.T(), response)
	assert.NotNil(s.T(), svcErr)
	assert.Equal(s.T(), serviceerror.InternalServerError.Code, svcErr.Code)

	s.mockTokenValidator.AssertExpectations(s.T())
	s.mockJWTService.AssertExpectations(s.T())
	s.mockAttributeCacheService.AssertExpectations(s.T())
}
