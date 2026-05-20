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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/attributecache"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/tests/mocks/attributecachemock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
)

type UtilsTestSuite struct {
	suite.Suite
}

func TestUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(UtilsTestSuite))
}

func (suite *UtilsTestSuite) SetupTest() {
	config.ResetServerRuntime()

	testConfig := &config.Config{
		JWT: config.JWTConfig{
			Issuer:         "https://thunder.io",
			ValidityPeriod: 3600,
		},
	}
	_ = config.InitializeServerRuntime("test", testConfig)
}

// ============================================================================
// isSelfIssuer Tests
// ============================================================================

func (suite *UtilsTestSuite) TestisSelfIssuer_WithValidDeploymentIssuer() {
	result := isSelfIssuer("https://thunder.io")

	assert.True(suite.T(), result)
}

func (suite *UtilsTestSuite) TestisSelfIssuer_WithInvalidIssuer() {
	result := isSelfIssuer("https://evil.example.com")

	assert.False(suite.T(), result)
}

func (suite *UtilsTestSuite) TestisSelfIssuer_WithEmptyIssuer() {
	result := isSelfIssuer("")

	assert.False(suite.T(), result)
}

func (suite *UtilsTestSuite) TestJoinScopes_WithMultipleScopes() {
	scopes := []string{"read", "write", "admin"}
	result := JoinScopes(scopes)

	assert.Equal(suite.T(), "read write admin", result)
}

func (suite *UtilsTestSuite) TestJoinScopes_WithSingleScope() {
	scopes := []string{"read"}
	result := JoinScopes(scopes)

	assert.Equal(suite.T(), "read", result)
}

func (suite *UtilsTestSuite) TestJoinScopes_WithEmptySlice() {
	scopes := []string{}
	result := JoinScopes(scopes)

	assert.Equal(suite.T(), "", result)
}

func (suite *UtilsTestSuite) TestJoinScopes_WithNilSlice() {
	scopes := []string(nil)
	result := JoinScopes(scopes)

	assert.Equal(suite.T(), "", result)
}

// ============================================================================
// getStandardJWTClaims Tests
// ============================================================================

func (suite *UtilsTestSuite) TestgetStandardJWTClaims_ContainsAllStandardClaims() {
	claims := getStandardJWTClaims()

	assert.True(suite.T(), claims["sub"])
	assert.True(suite.T(), claims["iss"])
	assert.True(suite.T(), claims["aud"])
	assert.True(suite.T(), claims["exp"])
	assert.True(suite.T(), claims["nbf"])
	assert.True(suite.T(), claims["iat"])
	assert.True(suite.T(), claims["jti"])
	assert.True(suite.T(), claims["scope"])
	assert.True(suite.T(), claims["client_id"])
	assert.True(suite.T(), claims["act"])
}

func (suite *UtilsTestSuite) TestgetStandardJWTClaims_ReturnsNewMap() {
	claims1 := getStandardJWTClaims()
	claims2 := getStandardJWTClaims()

	// Should be independent - modifying one shouldn't affect the other
	claims1["test"] = true
	assert.NotContains(suite.T(), claims2, "test")
}

func (suite *UtilsTestSuite) TestExtractUserAttributes_WithStandardClaimsOnly() {
	claims := map[string]interface{}{
		"sub":   "user123",
		"iss":   "https://thunder.io",
		"aud":   "app123",
		"exp":   1234567890,
		"scope": "read write",
	}

	result := ExtractUserAttributes(claims)

	assert.Empty(suite.T(), result)
}

func (suite *UtilsTestSuite) TestExtractUserAttributes_WithCustomClaims() {
	claims := map[string]interface{}{
		"sub":    "user123",
		"iss":    "https://thunder.io",
		"aud":    "app123",
		"exp":    1234567890,
		"scope":  "read write",
		"name":   "John Doe",
		"email":  "john@example.com",
		"groups": []string{"admin", "user"},
	}

	result := ExtractUserAttributes(claims)

	assert.Equal(suite.T(), "John Doe", result["name"])
	assert.Equal(suite.T(), "john@example.com", result["email"])
	assert.Equal(suite.T(), []string{"admin", "user"}, result["groups"])
	assert.NotContains(suite.T(), result, "sub")
	assert.NotContains(suite.T(), result, "iss")
	assert.NotContains(suite.T(), result, "aud")
	assert.NotContains(suite.T(), result, "exp")
	assert.NotContains(suite.T(), result, "scope")
}

func (suite *UtilsTestSuite) TestExtractUserAttributes_WithRefreshTokenSpecificClaims() {
	claims := map[string]interface{}{
		"sub":                          "user123",
		"iss":                          "https://thunder.io",
		"aud":                          "app123",
		"exp":                          1234567890,
		"scope":                        "read write",
		"grant_type":                   "authorization_code",
		"access_token_sub":             "user123",
		"access_token_aud":             "app123",
		"access_token_user_attributes": map[string]interface{}{"name": "John"},
		"name":                         "John Doe",
		"email":                        "john@example.com",
	}

	result := ExtractUserAttributes(claims)

	// Should include refresh token specific claims as they're not standard JWT claims
	assert.Equal(suite.T(), "John Doe", result["name"])
	assert.Equal(suite.T(), "john@example.com", result["email"])
	assert.Equal(suite.T(), "authorization_code", result["grant_type"])
	assert.Equal(suite.T(), "user123", result["access_token_sub"])
	assert.Equal(suite.T(), "app123", result["access_token_aud"])
}

func (suite *UtilsTestSuite) TestExtractUserAttributes_EmptyClaims() {
	claims := map[string]interface{}{}

	result := ExtractUserAttributes(claims)

	assert.Empty(suite.T(), result)
}

func (suite *UtilsTestSuite) TestExtractUserAttributes_NilClaims() {
	claims := map[string]interface{}(nil)

	result := ExtractUserAttributes(claims)

	assert.Empty(suite.T(), result)
}

func (suite *UtilsTestSuite) TestextractInt64Claim_WithIntType() {
	claims := map[string]interface{}{
		"iat": int(1234567890),
	}

	result, err := extractInt64Claim(claims, "iat")

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(1234567890), result)
}

func (suite *UtilsTestSuite) TestextractInt64Claim_WithInt64Type() {
	claims := map[string]interface{}{
		"iat": int64(1234567890),
	}

	result, err := extractInt64Claim(claims, "iat")

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(1234567890), result)
}

func (suite *UtilsTestSuite) TestextractInt64Claim_WithInvalidType() {
	claims := map[string]interface{}{
		"iat": "not-a-number",
	}

	result, err := extractInt64Claim(claims, "iat")

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), int64(0), result)
	assert.Contains(suite.T(), err.Error(), "not a number")
}

func (suite *UtilsTestSuite) TestParseScopes_WithMultipleSpaces() {
	scopeString := "read  write   admin"
	result := ParseScopes(scopeString)

	assert.Equal(suite.T(), []string{"read", "write", "admin"}, result)
}

func (suite *UtilsTestSuite) TestParseScopes_WithLeadingTrailingSpaces() {
	scopeString := "  read write  "
	result := ParseScopes(scopeString)

	assert.Equal(suite.T(), []string{"read", "write"}, result)
}

func (suite *UtilsTestSuite) TestParseScopes_WithSingleScope() {
	scopeString := "read"
	result := ParseScopes(scopeString)

	assert.Equal(suite.T(), []string{"read"}, result)
}

func (suite *UtilsTestSuite) TestextractScopesFromClaims_WithValidScope() {
	claims := map[string]interface{}{
		"scope": "read write admin",
	}

	result := extractScopesFromClaims(claims, false)

	assert.Equal(suite.T(), []string{"read", "write", "admin"}, result)
}

func (suite *UtilsTestSuite) TestextractScopesFromClaims_WithEmptyScopeString() {
	claims := map[string]interface{}{
		"scope": "", // Empty string
	}

	result := extractScopesFromClaims(claims, false)

	assert.Empty(suite.T(), result)
}

func (suite *UtilsTestSuite) TestextractScopesFromClaims_WithInvalidScopeType() {
	claims := map[string]interface{}{
		"scope": 12345, // Invalid type (not string)
	}

	result := extractScopesFromClaims(claims, false)

	assert.Empty(suite.T(), result)
}

func (suite *UtilsTestSuite) TestextractScopesFromClaims_WithNoScopeButAuthorizedPermissions_IsAuthAssertion() {
	claims := map[string]interface{}{
		"authorized_permissions": "read:documents write:documents",
	}

	result := extractScopesFromClaims(claims, true)

	assert.Equal(suite.T(), []string{"read:documents", "write:documents"}, result)
}

func (suite *UtilsTestSuite) TestextractScopesFromClaims_WithNoScopeButAuthorizedPermissions_NotAuthAssertion() {
	claims := map[string]interface{}{
		"authorized_permissions": "read:documents write:documents",
	}

	result := extractScopesFromClaims(claims, false)

	assert.Empty(suite.T(), result) // Should not use authorized_permissions when not auth assertion
}

func (suite *UtilsTestSuite) TestextractScopesFromClaims_WithEmptyScopeButAuthorizedPermissions_IsAuthAssertion() {
	claims := map[string]interface{}{
		"scope":                  "", // Empty scope
		"authorized_permissions": "read write",
	}

	result := extractScopesFromClaims(claims, true)

	assert.Equal(suite.T(), []string{"read", "write"}, result)
}

func (suite *UtilsTestSuite) TestextractScopesFromClaims_WithEmptyAuthorizedPermissions_IsAuthAssertion() {
	claims := map[string]interface{}{
		"authorized_permissions": "", // Empty string
	}

	result := extractScopesFromClaims(claims, true)

	assert.Empty(suite.T(), result)
}

func (suite *UtilsTestSuite) TestextractScopesFromClaims_WithInvalidAuthorizedPermissionsType_IsAuthAssertion() {
	claims := map[string]interface{}{
		"authorized_permissions": 12345, // Invalid type (not string)
	}

	result := extractScopesFromClaims(claims, true)

	assert.Empty(suite.T(), result)
}

func (suite *UtilsTestSuite) TestextractScopesFromClaims_WithNoScopeAndNoAuthorizedPermissions() {
	claims := map[string]interface{}{
		// No scope or authorized_permissions
	}

	result := extractScopesFromClaims(claims, true)

	assert.Empty(suite.T(), result)
}

func (suite *UtilsTestSuite) TestextractScopesFromClaims_ScopeTakesPriorityOverAuthorizedPermissions() {
	claims := map[string]interface{}{
		"scope":                  "openid profile",
		"authorized_permissions": "read:documents write:documents",
	}

	result := extractScopesFromClaims(claims, true)

	// Scope should take priority
	assert.Equal(suite.T(), []string{"openid", "profile"}, result)
}

func (suite *UtilsTestSuite) TestFetchUserAttributes_GetAttributeCacheError() {
	mockAttrCacheService := attributecachemock.NewAttributeCacheServiceInterfaceMock(suite.T())

	// Mock GetAttributeCache to return error
	serverErr := &serviceerror.ServiceError{
		Type: serviceerror.ServerErrorType,
		Code: "CACHE_NOT_FOUND",
		Error: core.I18nMessage{
			Key:          "cache_not_found",
			DefaultValue: "Cache not found",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "cache_not_found_desc",
			DefaultValue: "cache not found",
		},
	}
	mockAttrCacheService.On("GetAttributeCache", mock.Anything, "cache-key-123").
		Return(nil, serverErr)

	_, err := FetchUserAttributes(context.Background(), mockAttrCacheService,
		[]string{constants.ClaimUserType}, "cache-key-123")

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to fetch attribute cache")

	mockAttrCacheService.AssertExpectations(suite.T())
}

func (suite *UtilsTestSuite) TestFetchUserAttributes_EmptyCacheKey() {
	mockAttrCacheService := attributecachemock.NewAttributeCacheServiceInterfaceMock(suite.T())

	// When cache key is empty, no cache lookup should happen
	attrs, err := FetchUserAttributes(context.Background(), mockAttrCacheService,
		[]string{constants.ClaimUserType}, "")

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), attrs)
	assert.Empty(suite.T(), attrs) // No attributes when cache key is empty and no claims allowed

	// Verify GetAttributeCache was NOT called
	mockAttrCacheService.AssertNotCalled(suite.T(), "GetAttributeCache", mock.Anything, mock.Anything)
}

func (suite *UtilsTestSuite) TestFetchUserAttributes_NilCacheAttributes() {
	mockAttrCacheService := attributecachemock.NewAttributeCacheServiceInterfaceMock(suite.T())

	// Mock GetAttributeCache to return cache with nil attributes — must be treated as an error
	mockAttrCacheService.On("GetAttributeCache", mock.Anything, "cache-key-123").
		Return(&attributecache.AttributeCache{
			ID:         "cache-key-123",
			Attributes: nil,
		}, nil)

	allowedClaims := []string{constants.ClaimUserType}
	_, err := FetchUserAttributes(context.Background(), mockAttrCacheService,
		allowedClaims, "cache-key-123")

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "attribute cache not found for key")

	mockAttrCacheService.AssertExpectations(suite.T())
}

func (suite *UtilsTestSuite) TestFetchUserAttributes_EmptyAllowedClaims() {
	mockAttrCacheService := attributecachemock.NewAttributeCacheServiceInterfaceMock(suite.T())

	// Mock GetAttributeCache to return cached attributes
	mockAttrCacheService.On("GetAttributeCache", mock.Anything, "cache-key-123").
		Return(&attributecache.AttributeCache{
			ID: "cache-key-123",
			Attributes: map[string]interface{}{
				"email":                 "test@example.com",
				constants.ClaimUserType: "local",
				constants.ClaimOUID:     "ou-123",
			},
		}, nil)

	// Empty allowedClaims - no claims should be returned
	attrs, err := FetchUserAttributes(context.Background(), mockAttrCacheService,
		[]string{}, "cache-key-123")

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), attrs)
	// No attributes should be present when allowedClaims is empty
	assert.Empty(suite.T(), attrs)

	mockAttrCacheService.AssertExpectations(suite.T())
}

func (suite *UtilsTestSuite) TestFetchUserAttributes_NilAllowedClaims() {
	mockAttrCacheService := attributecachemock.NewAttributeCacheServiceInterfaceMock(suite.T())

	// Mock GetAttributeCache to return cached attributes
	mockAttrCacheService.On("GetAttributeCache", mock.Anything, "cache-key-123").
		Return(&attributecache.AttributeCache{
			ID: "cache-key-123",
			Attributes: map[string]interface{}{
				"email":                 "test@example.com",
				constants.ClaimUserType: "local",
				constants.ClaimOUID:     "ou-123",
			},
		}, nil)

	// Nil allowedClaims - no claims should be returned
	attrs, err := FetchUserAttributes(context.Background(), mockAttrCacheService,
		nil, "cache-key-123")

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), attrs)
	// No attributes should be present when allowedClaims is nil
	assert.Empty(suite.T(), attrs)

	mockAttrCacheService.AssertExpectations(suite.T())
}

func (suite *UtilsTestSuite) TestFetchUserAttributes_CacheWithoutUserType() {
	mockAttrCacheService := attributecachemock.NewAttributeCacheServiceInterfaceMock(suite.T())

	// Mock GetAttributeCache to return cache without userType
	mockAttrCacheService.On("GetAttributeCache", mock.Anything, "cache-key-123").
		Return(&attributecache.AttributeCache{
			ID: "cache-key-123",
			Attributes: map[string]interface{}{
				"email":             "test@example.com",
				constants.ClaimOUID: "ou-123",
			},
		}, nil)

	allowedClaims := []string{constants.ClaimUserType, constants.ClaimOUID}
	attrs, err := FetchUserAttributes(context.Background(), mockAttrCacheService,
		allowedClaims, "cache-key-123")

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), attrs)
	// userType should not be present since it's not in cache
	assert.Nil(suite.T(), attrs[constants.ClaimUserType])
	// ouId should be present
	assert.Equal(suite.T(), "ou-123", attrs[constants.ClaimOUID])

	mockAttrCacheService.AssertExpectations(suite.T())
}

//nolint:dupl // Similar test structure but different scenario (cache without OUID)
func (suite *UtilsTestSuite) TestFetchUserAttributes_CacheWithoutOUID() {
	mockAttrCacheService := attributecachemock.NewAttributeCacheServiceInterfaceMock(suite.T())

	// Mock GetAttributeCache to return cache without OUID
	mockAttrCacheService.On("GetAttributeCache", mock.Anything, "cache-key-123").
		Return(&attributecache.AttributeCache{
			ID: "cache-key-123",
			Attributes: map[string]interface{}{
				"email":                 "test@example.com",
				constants.ClaimUserType: "local",
			},
		}, nil)

	allowedClaims := []string{constants.ClaimUserType, constants.ClaimOUID}
	attrs, err := FetchUserAttributes(context.Background(), mockAttrCacheService,
		allowedClaims, "cache-key-123")

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), attrs)
	// userType should be present
	assert.Equal(suite.T(), "local", attrs[constants.ClaimUserType])
	// ouId should not be present since it's not in cache
	assert.Nil(suite.T(), attrs[constants.ClaimOUID])

	mockAttrCacheService.AssertExpectations(suite.T())
}

func (suite *UtilsTestSuite) TestResolveTokenConfig_RefreshToken_WithServerLevelConfig() {
	// Reset and initialize config with refresh token validity period
	config.ResetServerRuntime()
	testConfig := &config.Config{
		JWT: config.JWTConfig{
			Issuer:         "https://thunder.io",
			ValidityPeriod: 3600,
		},
		OAuth: config.OAuthConfig{
			RefreshToken: config.RefreshTokenConfig{
				ValidityPeriod: 86400, // 24 hours
			},
		},
	}
	_ = config.InitializeServerRuntime("test", testConfig)

	oauthApp := &inboundmodel.OAuthClient{
		ClientID: "test-client",
	}

	result := ResolveTokenConfig(oauthApp, TokenTypeRefresh)

	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), int64(86400), result.ValidityPeriod)
	assert.Equal(suite.T(), "https://thunder.io", result.Issuer)
}

func (suite *UtilsTestSuite) TestResolveTokenConfig_RefreshToken_WithoutServerLevelConfig() {
	// Reset and initialize config without refresh token validity period (zero value)
	config.ResetServerRuntime()
	testConfig := &config.Config{
		JWT: config.JWTConfig{
			Issuer:         "https://thunder.io",
			ValidityPeriod: 3600,
		},
		OAuth: config.OAuthConfig{
			RefreshToken: config.RefreshTokenConfig{
				ValidityPeriod: 0, // Not set
			},
		},
	}
	_ = config.InitializeServerRuntime("test", testConfig)

	oauthApp := &inboundmodel.OAuthClient{
		ClientID: "test-client",
	}

	result := ResolveTokenConfig(oauthApp, TokenTypeRefresh)

	assert.NotNil(suite.T(), result)
	// Should fallback to default JWT validity period
	assert.Equal(suite.T(), int64(3600), result.ValidityPeriod)
}

func (suite *UtilsTestSuite) TestResolveTokenConfig_RefreshToken_WithNilOAuthApp() {
	// Reset and initialize config with refresh token validity period
	config.ResetServerRuntime()
	testConfig := &config.Config{
		JWT: config.JWTConfig{
			Issuer:         "https://thunder.io",
			ValidityPeriod: 3600,
		},
		OAuth: config.OAuthConfig{
			RefreshToken: config.RefreshTokenConfig{
				ValidityPeriod: 604800, // 7 days
			},
		},
	}
	_ = config.InitializeServerRuntime("test", testConfig)

	// oauthApp is nil
	result := ResolveTokenConfig(nil, TokenTypeRefresh)

	assert.NotNil(suite.T(), result)
	// Should still use server-level refresh token config
	assert.Equal(suite.T(), int64(604800), result.ValidityPeriod)
	assert.Equal(suite.T(), "https://thunder.io", result.Issuer)
}

func (suite *UtilsTestSuite) TestResolveTokenConfig_RefreshToken_WithTokenConfig() {
	// Refresh token always uses server-level issuer from config
	config.ResetServerRuntime()
	testConfig := &config.Config{
		JWT: config.JWTConfig{
			Issuer:         "https://thunder.io",
			ValidityPeriod: 3600,
		},
		OAuth: config.OAuthConfig{
			RefreshToken: config.RefreshTokenConfig{
				ValidityPeriod: 86400,
			},
		},
	}
	_ = config.InitializeServerRuntime("test", testConfig)

	oauthApp := &inboundmodel.OAuthClient{
		ClientID: "test-client",
		Token:    &inboundmodel.OAuthTokenConfig{},
	}

	result := ResolveTokenConfig(oauthApp, TokenTypeRefresh)

	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), int64(86400), result.ValidityPeriod)
	assert.Equal(suite.T(), "https://thunder.io", result.Issuer)
}

func (suite *UtilsTestSuite) TestResolveTokenConfig_AccessToken_WithNilOAuthApp() {
	config.ResetServerRuntime()
	testConfig := &config.Config{
		JWT: config.JWTConfig{
			Issuer:         "https://thunder.io",
			ValidityPeriod: 3600,
		},
	}
	_ = config.InitializeServerRuntime("test", testConfig)

	// oauthApp is nil - should use default config
	result := ResolveTokenConfig(nil, TokenTypeAccess)

	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), int64(3600), result.ValidityPeriod)
	assert.Equal(suite.T(), "https://thunder.io", result.Issuer)
}

func (suite *UtilsTestSuite) TestResolveTokenConfig_AccessToken_WithNilToken() {
	config.ResetServerRuntime()
	testConfig := &config.Config{
		JWT: config.JWTConfig{
			Issuer:         "https://thunder.io",
			ValidityPeriod: 3600,
		},
	}
	_ = config.InitializeServerRuntime("test", testConfig)

	// oauthApp.Token is nil - should use default config
	oauthApp := &inboundmodel.OAuthClient{
		ClientID: "test-client",
		Token:    nil,
	}

	result := ResolveTokenConfig(oauthApp, TokenTypeAccess)

	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), int64(3600), result.ValidityPeriod)
	assert.Equal(suite.T(), "https://thunder.io", result.Issuer)
}

func (suite *UtilsTestSuite) TestResolveTokenConfig_AccessToken_WithAppLevelConfig() {
	config.ResetServerRuntime()
	testConfig := &config.Config{
		JWT: config.JWTConfig{
			Issuer:         "https://thunder.io",
			ValidityPeriod: 3600,
		},
	}
	_ = config.InitializeServerRuntime("test", testConfig)

	oauthApp := &inboundmodel.OAuthClient{
		ClientID: "test-client",
		Token: &inboundmodel.OAuthTokenConfig{
			AccessToken: &inboundmodel.AccessTokenConfig{
				ValidityPeriod: 7200,
			},
		},
	}

	result := ResolveTokenConfig(oauthApp, TokenTypeAccess)

	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), int64(7200), result.ValidityPeriod)
}

func (suite *UtilsTestSuite) TestResolveTokenConfig_IDToken_WithNilOAuthApp() {
	config.ResetServerRuntime()
	testConfig := &config.Config{
		JWT: config.JWTConfig{
			Issuer:         "https://thunder.io",
			ValidityPeriod: 3600,
		},
	}
	_ = config.InitializeServerRuntime("test", testConfig)

	// oauthApp is nil - should use default config
	result := ResolveTokenConfig(nil, TokenTypeID)

	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), int64(3600), result.ValidityPeriod)
	assert.Equal(suite.T(), "https://thunder.io", result.Issuer)
}

func (suite *UtilsTestSuite) TestResolveTokenConfig_IDToken_WithNilToken() {
	config.ResetServerRuntime()
	testConfig := &config.Config{
		JWT: config.JWTConfig{
			Issuer:         "https://thunder.io",
			ValidityPeriod: 3600,
		},
	}
	_ = config.InitializeServerRuntime("test", testConfig)

	// oauthApp.Token is nil - should use default config
	oauthApp := &inboundmodel.OAuthClient{
		ClientID: "test-client",
		Token:    nil,
	}

	result := ResolveTokenConfig(oauthApp, TokenTypeID)

	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), int64(3600), result.ValidityPeriod)
	assert.Equal(suite.T(), "https://thunder.io", result.Issuer)
}

func (suite *UtilsTestSuite) TestResolveTokenConfig_IDToken_WithAppLevelConfig() {
	config.ResetServerRuntime()
	testConfig := &config.Config{
		JWT: config.JWTConfig{
			Issuer:         "https://thunder.io",
			ValidityPeriod: 3600,
		},
	}
	_ = config.InitializeServerRuntime("test", testConfig)

	oauthApp := &inboundmodel.OAuthClient{
		ClientID: "test-client",
		Token: &inboundmodel.OAuthTokenConfig{
			IDToken: &inboundmodel.IDTokenConfig{
				ValidityPeriod: 1800,
			},
		},
	}

	result := ResolveTokenConfig(oauthApp, TokenTypeID)

	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), int64(1800), result.ValidityPeriod)
}

func (suite *UtilsTestSuite) TestResolveTokenConfig_WithCustomIssuer_NilOAuthApp() {
	config.ResetServerRuntime()
	testConfig := &config.Config{
		JWT: config.JWTConfig{
			Issuer:         "https://thunder.io",
			ValidityPeriod: 3600,
		},
	}
	_ = config.InitializeServerRuntime("test", testConfig)

	// With nil oauthApp, should use default issuer
	result := ResolveTokenConfig(nil, TokenTypeAccess)

	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "https://thunder.io", result.Issuer)
}

func (suite *UtilsTestSuite) TestResolveTokenConfig_WithTokenConfig_UsesServerIssuer() {
	config.ResetServerRuntime()
	testConfig := &config.Config{
		JWT: config.JWTConfig{
			Issuer:         "https://thunder.io",
			ValidityPeriod: 3600,
		},
	}
	_ = config.InitializeServerRuntime("test", testConfig)

	// OAuthApp with token config always uses server-level issuer from config
	oauthApp := &inboundmodel.OAuthClient{
		ClientID: "test-client",
		Token:    &inboundmodel.OAuthTokenConfig{},
	}

	result := ResolveTokenConfig(oauthApp, TokenTypeAccess)

	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "https://thunder.io", result.Issuer)
}

const (
	testBCCAppID = "app-123"
	testBCCOUID  = "ou-456"
)

func newOAuthAppForClientAttributes(ouID string) *inboundmodel.OAuthClient {
	return &inboundmodel.OAuthClient{
		ID:   testBCCAppID,
		OUID: ouID,
	}
}

func (suite *UtilsTestSuite) TestBuildClientAttributes_NoOUID_ReturnsNil() {
	ous := oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())

	app := newOAuthAppForClientAttributes("")
	claims, err := BuildClientAttributes(context.Background(), app, ous)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), claims)
}

func (suite *UtilsTestSuite) TestBuildClientAttributes_NilOAuthApp_ReturnsNil() {
	ous := oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())

	claims, err := BuildClientAttributes(context.Background(), nil, ous)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), claims)
}

func (suite *UtilsTestSuite) TestBuildClientAttributes_HappyPath() {
	ous := oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())

	ous.On("GetOrganizationUnit", context.Background(), testBCCOUID).Return(ou.OrganizationUnit{
		ID:     testBCCOUID,
		Name:   "Engineering",
		Handle: "eng",
	}, (*serviceerror.ServiceError)(nil))

	app := newOAuthAppForClientAttributes(testBCCOUID)
	claims, err := BuildClientAttributes(context.Background(), app, ous)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), claims)
	assert.Equal(suite.T(), testBCCOUID, claims[constants.ClaimOUID])
	assert.Equal(suite.T(), "Engineering", claims[constants.ClaimOUName])
	assert.Equal(suite.T(), "eng", claims[constants.ClaimOUHandle])
}

func (suite *UtilsTestSuite) TestBuildClientAttributes_OULookupError_ReturnsError() {
	ous := oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())

	ous.On("GetOrganizationUnit", context.Background(), testBCCOUID).Return(
		ou.OrganizationUnit{},
		&serviceerror.ServiceError{
			Code:  "OU-0001",
			Error: core.I18nMessage{Key: "error.test.not_found", DefaultValue: "not found"},
		},
	)

	app := newOAuthAppForClientAttributes(testBCCOUID)
	claims, err := BuildClientAttributes(context.Background(), app, ous)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), claims)
}

func (suite *UtilsTestSuite) TestBuildClientAttributes_NilOUService_ReturnsNil() {
	app := newOAuthAppForClientAttributes(testBCCOUID)
	claims, err := BuildClientAttributes(context.Background(), app, nil)
	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), claims)
}

// ============================================================================
// §1 — extractAudiences direct unit tests
// ============================================================================

func (suite *UtilsTestSuite) TestExtractAudiences_StringValue() {
	claims := map[string]interface{}{"aud": "x"}
	auds, err := extractAudiences(claims)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), []string{"x"}, auds)
}

func (suite *UtilsTestSuite) TestExtractAudiences_StringSlice() {
	claims := map[string]interface{}{"aud": []interface{}{"x", "y"}}
	auds, err := extractAudiences(claims)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), []string{"x", "y"}, auds)
}

func (suite *UtilsTestSuite) TestExtractAudiences_SingleElementSlice() {
	claims := map[string]interface{}{"aud": []interface{}{"x"}}
	auds, err := extractAudiences(claims)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), []string{"x"}, auds)
}

func (suite *UtilsTestSuite) TestExtractAudiences_EmptyString_ReturnsError() {
	claims := map[string]interface{}{"aud": ""}
	auds, err := extractAudiences(claims)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), auds)
}

func (suite *UtilsTestSuite) TestExtractAudiences_EmptySlice_ReturnsError() {
	claims := map[string]interface{}{"aud": []interface{}{}}
	auds, err := extractAudiences(claims)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), auds)
}

func (suite *UtilsTestSuite) TestExtractAudiences_NilValue_ReturnsError() {
	claims := map[string]interface{}{"aud": nil}
	auds, err := extractAudiences(claims)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), auds)
}

func (suite *UtilsTestSuite) TestExtractAudiences_MissingKey_ReturnsError() {
	claims := map[string]interface{}{}
	auds, err := extractAudiences(claims)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), auds)
}

func (suite *UtilsTestSuite) TestExtractAudiences_WrongType_ReturnsError() {
	claims := map[string]interface{}{"aud": 123}
	auds, err := extractAudiences(claims)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), auds)
}

func (suite *UtilsTestSuite) TestExtractAudiences_MixedSliceNonString_ReturnsError() {
	claims := map[string]interface{}{"aud": []interface{}{"x", 42}}
	auds, err := extractAudiences(claims)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), auds)
}

func (suite *UtilsTestSuite) TestExtractAudiences_SliceWithEmptyString_ReturnsError() {
	claims := map[string]interface{}{"aud": []interface{}{"x", ""}}
	auds, err := extractAudiences(claims)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), auds)
}
