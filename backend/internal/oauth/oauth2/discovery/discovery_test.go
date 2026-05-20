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

// Package discovery provides tests for the OAuth2 and OIDC discovery endpoints.
package discovery

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/cryptolab"
	"github.com/thunder-id/thunderid/internal/system/kmprovider"
	"github.com/thunder-id/thunderid/tests/mocks/crypto/cryptomock"
)

type DiscoveryTestSuite struct {
	suite.Suite
	cryptoMock       *cryptomock.RuntimeCryptoProviderMock
	discoveryService DiscoveryServiceInterface
	handler          discoveryHandlerInterface
}

func TestDiscoverySuite(t *testing.T) {
	suite.Run(t, new(DiscoveryTestSuite))
}

func (suite *DiscoveryTestSuite) SetupTest() {
	testConfig := &config.Config{
		Server: config.ServerConfig{
			Hostname: "localhost",
			Port:     8080,
			HTTPOnly: false,
		},
		JWT: config.JWTConfig{
			Issuer:         "https://auth.example.com",
			ValidityPeriod: 3600,
		},
		OAuth: config.OAuthConfig{
			AuthClass: config.AuthClassConfig{
				Amrs: []string{"PWD", "OTP"},
				AcrAMR: map[string][]string{
					"urn:thunder:acr:password":       {"PWD"},
					"urn:thunder:acr:generated-code": {"OTP"},
				},
			},
		},
	}
	_ = config.InitializeServerRuntime("test", testConfig)

	suite.cryptoMock = cryptomock.NewRuntimeCryptoProviderMock(suite.T())
	suite.discoveryService = newDiscoveryService(suite.cryptoMock)
	suite.handler = newDiscoveryHandler(suite.discoveryService)
}

func (suite *DiscoveryTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (suite *DiscoveryTestSuite) TestOAuth2AuthorizationServerMetadata() {
	req := httptest.NewRequest("GET", "/.well-known/oauth-authorization-server", nil)
	w := httptest.NewRecorder()

	suite.handler.HandleOAuth2AuthorizationServerMetadata(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)
	assert.Equal(suite.T(), "application/json", w.Header().Get("Content-Type"))

	var metadata OAuth2AuthorizationServerMetadata
	err := json.NewDecoder(w.Body).Decode(&metadata)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), metadata.Issuer)
	assert.NotEmpty(suite.T(), metadata.AuthorizationEndpoint)
	assert.NotEmpty(suite.T(), metadata.TokenEndpoint)
	assert.NotEmpty(suite.T(), metadata.JWKSUri)
	assert.NotEmpty(suite.T(), metadata.RegistrationEndpoint)
	assert.NotEmpty(suite.T(), metadata.IntrospectionEndpoint)
	assert.NotEmpty(suite.T(), metadata.UserInfoEndpoint)

	// Verify only implemented endpoints are present
	assert.Empty(suite.T(), metadata.RevocationEndpoint) // Not implemented

	// Verify only implemented grant types are present
	assert.Contains(suite.T(), metadata.GrantTypesSupported, "authorization_code")
	assert.Contains(suite.T(), metadata.GrantTypesSupported, "client_credentials")
	assert.Contains(suite.T(), metadata.GrantTypesSupported, "refresh_token")
	assert.NotContains(suite.T(), metadata.GrantTypesSupported, "password") // Not implemented
	assert.NotContains(suite.T(), metadata.GrantTypesSupported, "implicit") // Not implemented

	// Verify only implemented response types are present
	assert.Equal(suite.T(), []string{"code"}, metadata.ResponseTypesSupported)

	// Verify RFC 9207 advertisement
	assert.True(suite.T(), metadata.AuthorizationResponseIssParameterSupported)
}

func (suite *DiscoveryTestSuite) TestOIDCDiscovery() {
	suite.cryptoMock.EXPECT().GetPublicKeys(mock.Anything, kmprovider.PublicKeyFilter{}).
		Return([]kmprovider.PublicKeyInfo{{KeyID: "k1", Algorithm: cryptolab.AlgorithmRS256}}, nil)

	req := httptest.NewRequest("GET", "/.well-known/openid-configuration", nil)
	w := httptest.NewRecorder()

	suite.handler.HandleOIDCDiscovery(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)
	assert.Equal(suite.T(), "application/json", w.Header().Get("Content-Type"))

	var metadata OIDCProviderMetadata
	err := json.NewDecoder(w.Body).Decode(&metadata)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), metadata.Issuer)
	assert.NotEmpty(suite.T(), metadata.SubjectTypesSupported)
	assert.NotEmpty(suite.T(), metadata.ClaimsSupported)
	assert.NotEmpty(suite.T(), metadata.IDTokenSigningAlgValuesSupported)

	// Verify OIDC-specific fields
	assert.Contains(suite.T(), metadata.SubjectTypesSupported, constants.SubjectTypePublic)
	assert.Contains(suite.T(), metadata.IDTokenSigningAlgValuesSupported, "RS256")
	assert.Contains(suite.T(), metadata.ClaimsSupported, constants.ClaimSub)
	assert.Contains(suite.T(), metadata.ClaimsSupported, constants.ClaimIss)
	assert.Contains(suite.T(), metadata.ClaimsSupported, constants.ClaimAud)

	// Verify claims parameter support
	assert.True(suite.T(), metadata.ClaimsParameterSupported, "claims_parameter_supported should be true")

	// Verify RFC 9207 advertisement (inherited from embedded OAuth2AuthorizationServerMetadata)
	assert.True(suite.T(), metadata.AuthorizationResponseIssParameterSupported)
	assert.Contains(suite.T(), metadata.AcrValuesSupported, "urn:thunder:acr:password")
	assert.Contains(suite.T(), metadata.AcrValuesSupported, "urn:thunder:acr:generated-code")
}

// TestGrantTypeIsValid tests the GrantType.IsValid() method
// This is a standalone test for constants - doesn't require discovery service setup
func TestGrantTypeIsValid(t *testing.T) {
	// Test valid grant types
	assert.True(t, constants.GrantTypeAuthorizationCode.IsValid())
	assert.True(t, constants.GrantTypeClientCredentials.IsValid())
	assert.True(t, constants.GrantTypeRefreshToken.IsValid())

	// Test invalid grant types
	assert.False(t, constants.GrantType("invalid").IsValid())
	assert.False(t, constants.GrantType("password").IsValid())
	assert.False(t, constants.GrantType("").IsValid())
	assert.False(t, constants.GrantType("implicit").IsValid())
}

// TestResponseTypeIsValid tests the ResponseType.IsValid() method
// This is a standalone test for constants - doesn't require discovery service setup
func TestResponseTypeIsValid(t *testing.T) {
	// Test valid response types
	assert.True(t, constants.ResponseTypeCode.IsValid())

	// Test invalid response types
	assert.False(t, constants.ResponseType("invalid").IsValid())
	assert.False(t, constants.ResponseType("token").IsValid())
	assert.False(t, constants.ResponseType("id_token").IsValid())
	assert.False(t, constants.ResponseType("").IsValid())
}

// TestTokenEndpointAuthMethodIsValid tests the TokenEndpointAuthMethod.IsValid() method
// This is a standalone test for constants - doesn't require discovery service setup
func TestTokenEndpointAuthMethodIsValid(t *testing.T) {
	// Test valid authentication methods
	assert.True(t, constants.TokenEndpointAuthMethodClientSecretBasic.IsValid())
	assert.True(t, constants.TokenEndpointAuthMethodClientSecretPost.IsValid())
	assert.True(t, constants.TokenEndpointAuthMethodNone.IsValid())
	assert.True(t, constants.TokenEndpointAuthMethodPrivateKeyJWT.IsValid())

	// Test invalid authentication methods
	assert.False(t, constants.TokenEndpointAuthMethod("invalid").IsValid())
	assert.False(t, constants.TokenEndpointAuthMethod("client_secret_jwt").IsValid())
	assert.False(t, constants.TokenEndpointAuthMethod("").IsValid())
}

// TestGetSupportedResponseTypes tests the GetSupportedResponseTypes function
// This is a standalone test for constants - doesn't require discovery service setup
func TestGetSupportedResponseTypes(t *testing.T) {
	supported := constants.GetSupportedResponseTypes()

	assert.NotNil(t, supported)
	assert.Equal(t, 1, len(supported))
	assert.Contains(t, supported, "code")
	assert.Equal(t, []string{"code"}, supported)
}

// TestGetSupportedGrantTypes tests the GetSupportedGrantTypes function
// This is a standalone test for constants - doesn't require discovery service setup
func TestGetSupportedGrantTypes(t *testing.T) {
	supported := constants.GetSupportedGrantTypes()

	assert.NotNil(t, supported)
	assert.Equal(t, 4, len(supported))
	assert.Contains(t, supported, "authorization_code")
	assert.Contains(t, supported, "client_credentials")
	assert.Contains(t, supported, "refresh_token")
	assert.Contains(t, supported, "urn:ietf:params:oauth:grant-type:token-exchange")
	assert.NotContains(t, supported, "password")
	assert.NotContains(t, supported, "implicit")
}

// TestGetSupportedTokenEndpointAuthMethods tests the GetSupportedTokenEndpointAuthMethods function
// This is a standalone test for constants - doesn't require discovery service setup
func TestGetSupportedTokenEndpointAuthMethods(t *testing.T) {
	supported := constants.GetSupportedTokenEndpointAuthMethods()

	assert.NotNil(t, supported)
	assert.Equal(t, 4, len(supported))
	assert.Contains(t, supported, "client_secret_basic")
	assert.Contains(t, supported, "client_secret_post")
	assert.Contains(t, supported, "none")
	assert.Contains(t, supported, "private_key_jwt")
	assert.NotContains(t, supported, "client_secret_jwt")
}

// TestGetSupportedSubjectTypes tests the GetSupportedSubjectTypes function
// This is a standalone test for constants - doesn't require discovery service setup
func TestGetSupportedSubjectTypes(t *testing.T) {
	supported := constants.GetSupportedSubjectTypes()

	assert.NotNil(t, supported)
	assert.Equal(t, 1, len(supported))
	assert.Contains(t, supported, constants.SubjectTypePublic)
	assert.Equal(t, []string{"public"}, supported)
}

// TestGetStandardClaims tests the GetStandardClaims function
// This is a standalone test for constants - doesn't require discovery service setup
func TestGetStandardClaims(t *testing.T) {
	claims := constants.GetStandardClaims()

	assert.NotNil(t, claims)
	assert.GreaterOrEqual(t, len(claims), 6)
	assert.Contains(t, claims, constants.ClaimSub)
	assert.Contains(t, claims, constants.ClaimIss)
	assert.Contains(t, claims, constants.ClaimAud)
	assert.Contains(t, claims, constants.ClaimExp)
	assert.Contains(t, claims, constants.ClaimIat)
	assert.Contains(t, claims, constants.ClaimAuthTime)
}

func (suite *DiscoveryTestSuite) TestInitialize() {
	suite.cryptoMock.EXPECT().GetPublicKeys(mock.Anything, kmprovider.PublicKeyFilter{}).
		Return([]kmprovider.PublicKeyInfo{{KeyID: "k1", Algorithm: cryptolab.AlgorithmRS256}}, nil)

	mux := http.NewServeMux()
	service := Initialize(mux, suite.cryptoMock)

	assert.NotNil(suite.T(), service)
	assert.Implements(suite.T(), (*DiscoveryServiceInterface)(nil), service)

	// Test that routes are registered by making requests
	req := httptest.NewRequest("GET", "/.well-known/oauth-authorization-server", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	assert.Equal(suite.T(), http.StatusOK, w.Code)

	req = httptest.NewRequest("GET", "/.well-known/openid-configuration", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	assert.Equal(suite.T(), http.StatusOK, w.Code)

	// Test OPTIONS requests
	req = httptest.NewRequest("OPTIONS", "/.well-known/oauth-authorization-server", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	assert.Equal(suite.T(), http.StatusNoContent, w.Code)

	req = httptest.NewRequest("OPTIONS", "/.well-known/openid-configuration", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	assert.Equal(suite.T(), http.StatusNoContent, w.Code)
}

func (suite *DiscoveryTestSuite) TestGetBaseURL_WithPublicHostname() {
	config.ResetServerRuntime()
	testConfig := &config.Config{
		Server: config.ServerConfig{
			PublicURL: "https://public.thunder.io",
			Hostname:  "localhost",
			Port:      8080,
		},
		JWT: config.JWTConfig{
			Issuer: "https://auth.example.com",
		},
	}
	_ = config.InitializeServerRuntime("test", testConfig)

	service := newDiscoveryService(suite.cryptoMock)
	metadata := service.GetOAuth2AuthorizationServerMetadata(context.Background())
	assert.Contains(suite.T(), metadata.AuthorizationEndpoint, "public.thunder.io")
	config.ResetServerRuntime()
}

func (suite *DiscoveryTestSuite) TestGetBaseURL_WithHTTPOnly() {
	config.ResetServerRuntime()
	testConfig := &config.Config{
		Server: config.ServerConfig{
			Hostname: "localhost",
			Port:     8080,
			HTTPOnly: true,
		},
		JWT: config.JWTConfig{
			Issuer: "https://auth.example.com",
		},
	}
	_ = config.InitializeServerRuntime("test", testConfig)

	service := newDiscoveryService(suite.cryptoMock)
	metadata := service.GetOAuth2AuthorizationServerMetadata(context.Background())
	assert.Contains(suite.T(), metadata.AuthorizationEndpoint, "http://")
	config.ResetServerRuntime()
}

func (suite *DiscoveryTestSuite) TestOIDCDiscovery_MultipleKeyAlgorithms() {
	cryptoMock := cryptomock.NewRuntimeCryptoProviderMock(suite.T())
	cryptoMock.EXPECT().GetPublicKeys(mock.Anything, kmprovider.PublicKeyFilter{}).
		Return([]kmprovider.PublicKeyInfo{
			{KeyID: "k1", Algorithm: cryptolab.AlgorithmRS256},
			{KeyID: "k2", Algorithm: cryptolab.AlgorithmES256},
			{KeyID: "k3", Algorithm: cryptolab.AlgorithmEdDSA},
		}, nil)
	svc := newDiscoveryService(cryptoMock)
	meta, err := svc.GetOIDCMetadata(context.Background())
	assert.NoError(suite.T(), err)
	algs := meta.IDTokenSigningAlgValuesSupported

	assert.Equal(suite.T(), 3, len(algs))
	assert.Contains(suite.T(), algs, "RS256")
	assert.Contains(suite.T(), algs, "ES256")
	assert.Contains(suite.T(), algs, "EdDSA")
}

func (suite *DiscoveryTestSuite) TestOIDCDiscovery_DeduplicatesAlgorithms() {
	cryptoMock := cryptomock.NewRuntimeCryptoProviderMock(suite.T())
	cryptoMock.EXPECT().GetPublicKeys(mock.Anything, kmprovider.PublicKeyFilter{}).
		Return([]kmprovider.PublicKeyInfo{
			{KeyID: "k1", Algorithm: cryptolab.AlgorithmRS256},
			{KeyID: "k2", Algorithm: cryptolab.AlgorithmRS256},
		}, nil)
	svc := newDiscoveryService(cryptoMock)
	meta, err := svc.GetOIDCMetadata(context.Background())
	assert.NoError(suite.T(), err)
	algs := meta.IDTokenSigningAlgValuesSupported

	assert.Equal(suite.T(), 1, len(algs))
	assert.Contains(suite.T(), algs, "RS256")
}
