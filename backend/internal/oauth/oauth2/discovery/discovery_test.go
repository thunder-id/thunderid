// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package discovery provides tests for the OAuth2 and OIDC discovery endpoints.
package discovery

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	joseconfig "github.com/thunder-id/thunderid/internal/system/jose/config"
	"github.com/thunder-id/thunderid/internal/system/jose/jwe"
	"github.com/thunder-id/thunderid/tests/mocks/crypto/cryptomock"
)

// newTestJWEService builds a real jwe service backed by the given crypto provider, for tests that
// need discoveryService to read the alg/enc lists it exposes.
func newTestJWEService(cryptoProvider providers.RuntimeCryptoProvider) jwe.JWEServiceInterface {
	svc, _ := jwe.Initialize(cryptoProvider, joseconfig.Config{})
	return svc
}

type DiscoveryTestSuite struct {
	suite.Suite
	cryptoMock       *cryptomock.RuntimeCryptoProviderMock
	discoveryService DiscoveryServiceInterface
	handler          discoveryHandlerInterface
	oauthCfg         oauthconfig.Config
}

// oauthCfgFromServerConfig initializes the global server runtime with the supplied config
// and returns the derived oauth config (with OIDC defaults seeded via FromServerRuntime).
func (suite *DiscoveryTestSuite) oauthCfgFromServerConfig(cfg *config.Config) oauthconfig.Config {
	config.ResetServerRuntime()
	suite.Require().NoError(config.InitializeServerRuntime("/tmp/test-discovery", cfg))
	return oauthconfig.FromServerRuntime()
}

func TestDiscoverySuite(t *testing.T) {
	suite.Run(t, new(DiscoveryTestSuite))
}

func (suite *DiscoveryTestSuite) SetupTest() {
	testConfig := &config.Config{
		Server: engineconfig.ServerConfig{
			Hostname: "localhost",
			Port:     8080,
			HTTPOnly: false,
		},
		JWT: engineconfig.JWTConfig{
			Issuer:         "https://auth.example.com",
			ValidityPeriod: 3600,
		},
		OAuth: config.OAuthConfig{
			DPoP: engineconfig.DPoPConfig{
				Required:     false,
				IatWindow:    60,
				Leeway:       5,
				AllowedAlgs:  []string{"ES256", "PS256", "ES384", "ES512", "EdDSA", "RS256"},
				MaxJTILength: 256,
			},
			AuthClass: engineconfig.AuthClassConfig{
				Amrs: []string{"PWD", "OTP"},
				AcrAMR: map[string][]string{
					"urn:thunder:acr:password":       {"PWD"},
					"urn:thunder:acr:generated-code": {"OTP"},
				},
			},
			DCR:             engineconfig.DCRConfig{Enabled: boolPtr(true)},
			TokenRevocation: engineconfig.OAuthTokenRevocationConfig{Enabled: boolPtr(true)},
			Logout:          engineconfig.LogoutConfig{Enabled: boolPtr(true)},
		},
	}
	_ = config.InitializeServerRuntime("test", testConfig)

	suite.oauthCfg = suite.oauthCfgFromServerConfig(testConfig)
	suite.cryptoMock = cryptomock.NewRuntimeCryptoProviderMock(suite.T())
	suite.cryptoMock.EXPECT().GetSupportedSigningAlgorithms().
		Return(testConfig.OAuth.DPoP.AllowedAlgs).Maybe()
	suite.cryptoMock.EXPECT().GetSupportedEncryptionAlgorithms().
		Return([]string{string(cryptolib.AlgorithmRSAOAEP256)}).Maybe()
	suite.discoveryService = newDiscoveryService(suite.cryptoMock, newTestJWEService(suite.cryptoMock), suite.oauthCfg)
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
	assert.NotEmpty(suite.T(), metadata.RevocationEndpoint)

	body, err := json.Marshal(metadata)
	assert.NoError(suite.T(), err)
	assert.NotContains(suite.T(), string(body), "userinfo_endpoint")
	assert.NotContains(suite.T(), string(body), "scopes_supported")

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

func (suite *DiscoveryTestSuite) TestCIBAMetadataAdvertised() {
	metadata := suite.discoveryService.GetOAuth2AuthorizationServerMetadata(context.Background())

	assert.Contains(suite.T(), metadata.GrantTypesSupported, string(providers.GrantTypeCIBA))
	assert.NotEmpty(suite.T(), metadata.BackchannelAuthenticationEndpoint)
	assert.Contains(suite.T(), metadata.BackchannelAuthenticationEndpoint, constants.OAuth2BackchannelAuthEndpoint)
	assert.Equal(suite.T(), []string{"poll"}, metadata.BackchannelTokenDeliveryModesSupported)
	assert.False(suite.T(), metadata.BackchannelUserCodeParameterSupported)
}

func (suite *DiscoveryTestSuite) TestOIDCDiscovery() {
	suite.cryptoMock.EXPECT().GetPublicKeys(mock.Anything, providers.PublicKeyFilter{}).
		Return([]providers.PublicKeyInfo{{KeyID: "k1", Algorithm: string(cryptolib.AlgorithmRS256)}}, nil)

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
	assert.NotEmpty(suite.T(), metadata.UserInfoEndpoint)
	assert.NotEmpty(suite.T(), metadata.ScopesSupported)
	assert.Contains(suite.T(), metadata.ScopesSupported, "openid")
	assert.NotEmpty(suite.T(), metadata.EndSessionEndpoint)

	// Verify OIDC-specific fields
	assert.Contains(suite.T(), metadata.SubjectTypesSupported, constants.SubjectTypePublic)
	assert.Contains(suite.T(), metadata.IDTokenSigningAlgValuesSupported, "RS256")
	assert.Contains(suite.T(), metadata.ClaimsSupported, constants.ClaimSub)
	assert.Contains(suite.T(), metadata.ClaimsSupported, constants.ClaimIss)
	assert.Contains(suite.T(), metadata.ClaimsSupported, constants.ClaimAud)

	// Verify claims parameter support
	assert.True(suite.T(), metadata.ClaimsParameterSupported, "claims_parameter_supported should be true")

	// JAR (RFC 9101) is not implemented, so neither request object form is advertised.
	assert.False(suite.T(), metadata.RequestParameterSupported,
		"request_parameter_supported should be false")
	assert.False(suite.T(), metadata.RequestURIParameterSupported,
		"request_uri_parameter_supported should be false")

	// Verify RFC 9207 advertisement (inherited from embedded OAuth2AuthorizationServerMetadata)
	assert.True(suite.T(), metadata.AuthorizationResponseIssParameterSupported)
	assert.Contains(suite.T(), metadata.AcrValuesSupported, "urn:thunder:acr:password")
	assert.Contains(suite.T(), metadata.AcrValuesSupported, "urn:thunder:acr:generated-code")
}

func (suite *DiscoveryTestSuite) TestDPoPSigningAlgValuesAdvertised() {
	expected := []string{"ES256", "PS256", "ES384", "ES512", "EdDSA", "RS256"}

	oauth2Meta := suite.discoveryService.GetOAuth2AuthorizationServerMetadata(context.Background())
	assert.Equal(suite.T(), expected, oauth2Meta.DPoPSigningAlgValuesSupported)

	suite.cryptoMock.EXPECT().GetPublicKeys(mock.Anything, providers.PublicKeyFilter{}).
		Return([]providers.PublicKeyInfo{{KeyID: "k1", Algorithm: string(cryptolib.AlgorithmRS256)}}, nil)

	oidcMeta, err := suite.discoveryService.GetOIDCMetadata(context.Background())
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), expected, oidcMeta.DPoPSigningAlgValuesSupported)
}

// TestTokenEndpointAuthSigningAlgValuesAdvertised verifies the discovery metadata publishes
// token_endpoint_auth_signing_alg_values_supported with FAPI 2.0 permitted algorithms (RFC 8414).
func (suite *DiscoveryTestSuite) TestTokenEndpointAuthSigningAlgValuesAdvertised() {
	oauth2Meta := suite.discoveryService.GetOAuth2AuthorizationServerMetadata(context.Background())
	assert.NotEmpty(suite.T(), oauth2Meta.TokenEndpointAuthSigningAlgValuesSupported)
	assert.Contains(suite.T(), oauth2Meta.TokenEndpointAuthSigningAlgValuesSupported, "ES256")
	assert.Contains(suite.T(), oauth2Meta.TokenEndpointAuthSigningAlgValuesSupported, "PS256")
	assert.Contains(suite.T(), oauth2Meta.TokenEndpointAuthSigningAlgValuesSupported, "EdDSA")
}

func (suite *DiscoveryTestSuite) TestDPoPSigningAlgValuesOmittedWhenUnconfigured() {
	config.ResetServerRuntime()
	testConfig := &config.Config{
		Server: engineconfig.ServerConfig{Hostname: "localhost", Port: 8080},
		JWT:    engineconfig.JWTConfig{Issuer: "https://auth.example.com"},
	}
	_ = config.InitializeServerRuntime("test", testConfig)
	defer config.ResetServerRuntime()

	cryptoMock := cryptomock.NewRuntimeCryptoProviderMock(suite.T())
	cryptoMock.EXPECT().GetSupportedSigningAlgorithms().Return(nil)

	svc := newDiscoveryService(cryptoMock, newTestJWEService(cryptoMock), suite.oauthCfgFromServerConfig(testConfig))
	oauth2Meta := svc.GetOAuth2AuthorizationServerMetadata(context.Background())
	assert.Nil(suite.T(), oauth2Meta.DPoPSigningAlgValuesSupported)

	body, err := json.Marshal(oauth2Meta)
	assert.NoError(suite.T(), err)
	assert.NotContains(suite.T(), string(body), "dpop_signing_alg_values_supported")
}

func (suite *DiscoveryTestSuite) TestDCRRevocationLogoutEndpointsOmittedWhenDisabled() {
	config.ResetServerRuntime()
	testConfig := &config.Config{
		Server: engineconfig.ServerConfig{Hostname: "localhost", Port: 8080},
		JWT:    engineconfig.JWTConfig{Issuer: "https://auth.example.com"},
	}
	_ = config.InitializeServerRuntime("test", testConfig)
	defer config.ResetServerRuntime()

	svc := newDiscoveryService(
		suite.cryptoMock, newTestJWEService(suite.cryptoMock), suite.oauthCfgFromServerConfig(testConfig))
	oauth2Meta := svc.GetOAuth2AuthorizationServerMetadata(context.Background())
	assert.Empty(suite.T(), oauth2Meta.RegistrationEndpoint)
	assert.Empty(suite.T(), oauth2Meta.RevocationEndpoint)

	suite.cryptoMock.EXPECT().GetPublicKeys(mock.Anything, providers.PublicKeyFilter{}).
		Return([]providers.PublicKeyInfo{{KeyID: "k1", Algorithm: string(cryptolib.AlgorithmRS256)}}, nil)
	oidcMeta, err := svc.GetOIDCMetadata(context.Background())
	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), oidcMeta.EndSessionEndpoint)

	body, err := json.Marshal(oauth2Meta)
	assert.NoError(suite.T(), err)
	assert.NotContains(suite.T(), string(body), "registration_endpoint")
	assert.NotContains(suite.T(), string(body), "revocation_endpoint")
}

// TestGrantTypeIsValid tests the GrantType.IsValid() method
// This is a standalone test for constants - doesn't require discovery service setup
func TestGrantTypeIsValid(t *testing.T) {
	// Test valid grant types
	assert.True(t, providers.GrantTypeAuthorizationCode.IsValid())
	assert.True(t, providers.GrantTypeClientCredentials.IsValid())
	assert.True(t, providers.GrantTypeRefreshToken.IsValid())

	// Test invalid grant types
	assert.False(t, providers.GrantType("invalid").IsValid())
	assert.False(t, providers.GrantType("password").IsValid())
	assert.False(t, providers.GrantType("").IsValid())
	assert.False(t, providers.GrantType("implicit").IsValid())
}

// TestResponseTypeIsValid tests the ResponseType.IsValid() method
// This is a standalone test for constants - doesn't require discovery service setup
func TestResponseTypeIsValid(t *testing.T) {
	// Test valid response types
	assert.True(t, providers.ResponseTypeCode.IsValid())

	// Test invalid response types
	assert.False(t, providers.ResponseType("invalid").IsValid())
	assert.False(t, providers.ResponseType("token").IsValid())
	assert.False(t, providers.ResponseType("id_token").IsValid())
	assert.False(t, providers.ResponseType("").IsValid())
}

// TestTokenEndpointAuthMethodIsValid tests the TokenEndpointAuthMethod.IsValid() method
// This is a standalone test for constants - doesn't require discovery service setup
func TestTokenEndpointAuthMethodIsValid(t *testing.T) {
	// Test valid authentication methods
	assert.True(t, providers.TokenEndpointAuthMethodClientSecretBasic.IsValid())
	assert.True(t, providers.TokenEndpointAuthMethodClientSecretPost.IsValid())
	assert.True(t, providers.TokenEndpointAuthMethodNone.IsValid())
	assert.True(t, providers.TokenEndpointAuthMethodPrivateKeyJWT.IsValid())

	// Test invalid authentication methods
	assert.False(t, providers.TokenEndpointAuthMethod("invalid").IsValid())
	assert.False(t, providers.TokenEndpointAuthMethod("client_secret_jwt").IsValid())
	assert.False(t, providers.TokenEndpointAuthMethod("").IsValid())
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
	suite.cryptoMock.EXPECT().GetPublicKeys(mock.Anything, providers.PublicKeyFilter{}).
		Return([]providers.PublicKeyInfo{{KeyID: "k1", Algorithm: string(cryptolib.AlgorithmRS256)}}, nil)

	mux := http.NewServeMux()
	service := Initialize(mux, suite.cryptoMock, newTestJWEService(suite.cryptoMock), suite.oauthCfg)

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
		Server: engineconfig.ServerConfig{
			PublicURL: "https://public.thunder.io",
			Hostname:  "localhost",
			Port:      8080,
		},
		JWT: engineconfig.JWTConfig{
			Issuer: "https://auth.example.com",
		},
	}
	_ = config.InitializeServerRuntime("test", testConfig)

	service := newDiscoveryService(
		suite.cryptoMock, newTestJWEService(suite.cryptoMock), suite.oauthCfgFromServerConfig(testConfig))
	metadata := service.GetOAuth2AuthorizationServerMetadata(context.Background())
	assert.Contains(suite.T(), metadata.AuthorizationEndpoint, "public.thunder.io")
	config.ResetServerRuntime()
}

func (suite *DiscoveryTestSuite) TestGetBaseURL_WithHTTPOnly() {
	config.ResetServerRuntime()
	testConfig := &config.Config{
		Server: engineconfig.ServerConfig{
			Hostname: "localhost",
			Port:     8080,
			HTTPOnly: true,
		},
		JWT: engineconfig.JWTConfig{
			Issuer: "https://auth.example.com",
		},
	}
	_ = config.InitializeServerRuntime("test", testConfig)

	service := newDiscoveryService(
		suite.cryptoMock, newTestJWEService(suite.cryptoMock), suite.oauthCfgFromServerConfig(testConfig))
	metadata := service.GetOAuth2AuthorizationServerMetadata(context.Background())
	assert.Contains(suite.T(), metadata.AuthorizationEndpoint, "http://")
	config.ResetServerRuntime()
}

func (suite *DiscoveryTestSuite) TestOIDCDiscovery_MultipleKeyAlgorithms() {
	cryptoMock := cryptomock.NewRuntimeCryptoProviderMock(suite.T())
	cryptoMock.EXPECT().GetSupportedSigningAlgorithms().Return(suite.oauthCfg.OAuth.DPoP.AllowedAlgs)
	cryptoMock.EXPECT().GetSupportedEncryptionAlgorithms().Return([]string{string(cryptolib.AlgorithmRSAOAEP256)})
	cryptoMock.EXPECT().GetPublicKeys(mock.Anything, providers.PublicKeyFilter{}).
		Return([]providers.PublicKeyInfo{
			{KeyID: "k1", Algorithm: string(cryptolib.AlgorithmRS256)},
			{KeyID: "k2", Algorithm: string(cryptolib.AlgorithmES256)},
			{KeyID: "k3", Algorithm: string(cryptolib.AlgorithmEdDSA)},
		}, nil)
	svc := newDiscoveryService(cryptoMock, newTestJWEService(cryptoMock), suite.oauthCfg)
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
	cryptoMock.EXPECT().GetSupportedSigningAlgorithms().Return(suite.oauthCfg.OAuth.DPoP.AllowedAlgs)
	cryptoMock.EXPECT().GetSupportedEncryptionAlgorithms().Return([]string{string(cryptolib.AlgorithmRSAOAEP256)})
	cryptoMock.EXPECT().GetPublicKeys(mock.Anything, providers.PublicKeyFilter{}).
		Return([]providers.PublicKeyInfo{
			{KeyID: "k1", Algorithm: string(cryptolib.AlgorithmRS256)},
			{KeyID: "k2", Algorithm: string(cryptolib.AlgorithmRS256)},
		}, nil)
	svc := newDiscoveryService(cryptoMock, newTestJWEService(cryptoMock), suite.oauthCfg)
	meta, err := svc.GetOIDCMetadata(context.Background())
	assert.NoError(suite.T(), err)
	algs := meta.IDTokenSigningAlgValuesSupported

	assert.Equal(suite.T(), 1, len(algs))
	assert.Contains(suite.T(), algs, "RS256")
}

func (suite *DiscoveryTestSuite) TestAuthorizationGrantProfilesSupported_JWTBearerEnabled() {
	suite.cryptoMock.EXPECT().GetPublicKeys(mock.Anything, providers.PublicKeyFilter{}).
		Return([]providers.PublicKeyInfo{{KeyID: "k1", Algorithm: string(cryptolib.AlgorithmRS256)}}, nil)

	meta, err := suite.discoveryService.GetOIDCMetadata(context.Background())
	assert.NoError(suite.T(), err)

	// Default config allows the JWT Bearer grant type, so the ID-JAG profile should be advertised.
	assert.Contains(suite.T(), meta.GrantTypesSupported, string(providers.GrantTypeJWTBearer))
	assert.Contains(
		suite.T(), meta.AuthorizationGrantProfilesSupported, constants.SupportedAuthorizationGrantProfileIDJAG)
}

func (suite *DiscoveryTestSuite) TestAuthorizationGrantProfilesSupported_JWTBearerDisabled() {
	testConfig := &config.Config{
		Server: engineconfig.ServerConfig{Hostname: "localhost", Port: 8080},
		JWT:    engineconfig.JWTConfig{Issuer: "https://auth.example.com"},
		OAuth: config.OAuthConfig{
			AllowedGrantTypes: []string{"client_credentials", "refresh_token"},
		},
	}
	cryptoMock := cryptomock.NewRuntimeCryptoProviderMock(suite.T())
	cryptoMock.EXPECT().GetSupportedSigningAlgorithms().Return(suite.oauthCfg.OAuth.DPoP.AllowedAlgs).Maybe()
	cryptoMock.EXPECT().GetSupportedEncryptionAlgorithms().
		Return([]string{string(cryptolib.AlgorithmRSAOAEP256)}).Maybe()
	cryptoMock.EXPECT().GetPublicKeys(mock.Anything, providers.PublicKeyFilter{}).
		Return([]providers.PublicKeyInfo{{KeyID: "k1", Algorithm: string(cryptolib.AlgorithmRS256)}}, nil)

	svc := newDiscoveryService(cryptoMock, newTestJWEService(cryptoMock), suite.oauthCfgFromServerConfig(testConfig))
	meta, err := svc.GetOIDCMetadata(context.Background())
	assert.NoError(suite.T(), err)

	assert.NotContains(suite.T(), meta.GrantTypesSupported, string(providers.GrantTypeJWTBearer))
	assert.Empty(suite.T(), meta.AuthorizationGrantProfilesSupported)

	body, err := json.Marshal(meta)
	assert.NoError(suite.T(), err)
	assert.NotContains(suite.T(), string(body), "authorization_grant_profiles_supported")
}

func (suite *DiscoveryTestSuite) TestAuthorizationGrantProfilesSupported_AdvertisedOverHTTP() {
	suite.cryptoMock.EXPECT().GetPublicKeys(mock.Anything, providers.PublicKeyFilter{}).
		Return([]providers.PublicKeyInfo{{KeyID: "k1", Algorithm: string(cryptolib.AlgorithmRS256)}}, nil)

	req := httptest.NewRequest("GET", "/.well-known/openid-configuration", nil)
	w := httptest.NewRecorder()
	suite.handler.HandleOIDCDiscovery(w, req)
	assert.Equal(suite.T(), http.StatusOK, w.Code)

	var metadata OIDCProviderMetadata
	err := json.NewDecoder(w.Body).Decode(&metadata)
	assert.NoError(suite.T(), err)
	assert.Equal(
		suite.T(),
		[]string{constants.SupportedAuthorizationGrantProfileIDJAG},
		metadata.AuthorizationGrantProfilesSupported,
	)
}

func (suite *DiscoveryTestSuite) TestAuthorizationGrantProfilesSupported_OnOAuth2AuthorizationServerMetadata() {
	req := httptest.NewRequest("GET", "/.well-known/oauth-authorization-server", nil)
	w := httptest.NewRecorder()

	suite.handler.HandleOAuth2AuthorizationServerMetadata(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)

	var metadata OAuth2AuthorizationServerMetadata
	err := json.NewDecoder(w.Body).Decode(&metadata)
	assert.NoError(suite.T(), err)

	// Default config allows the JWT Bearer grant type, so the ID-JAG profile should be
	// advertised on the OAuth 2.0 Authorization Server Metadata endpoint too.
	assert.Contains(suite.T(), metadata.GrantTypesSupported, string(providers.GrantTypeJWTBearer))
	assert.Equal(
		suite.T(),
		[]string{constants.SupportedAuthorizationGrantProfileIDJAG},
		metadata.AuthorizationGrantProfilesSupported,
	)
}

func (suite *DiscoveryTestSuite) TestAuthorizationGrantProfilesSupported_NotOnOAuth2ServerMetadataWhenUnsupported() {
	testConfig := &config.Config{
		Server: engineconfig.ServerConfig{Hostname: "localhost", Port: 8080},
		JWT:    engineconfig.JWTConfig{Issuer: "https://auth.example.com"},
		OAuth: config.OAuthConfig{
			AllowedGrantTypes: []string{"client_credentials", "refresh_token"},
		},
	}
	svc := newDiscoveryService(
		suite.cryptoMock, newTestJWEService(suite.cryptoMock), suite.oauthCfgFromServerConfig(testConfig))
	handler := newDiscoveryHandler(svc)

	req := httptest.NewRequest("GET", "/.well-known/oauth-authorization-server", nil)
	w := httptest.NewRecorder()
	handler.HandleOAuth2AuthorizationServerMetadata(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)
	assert.NotContains(suite.T(), w.Body.String(), "authorization_grant_profiles_supported")
}

func (suite *DiscoveryTestSuite) TestOIDCDiscovery_EngineOverridesLandInWellKnown() {
	suite.cryptoMock.EXPECT().GetPublicKeys(mock.Anything, providers.PublicKeyFilter{}).
		Return([]providers.PublicKeyInfo{{KeyID: "k1", Algorithm: string(cryptolib.AlgorithmRS256)}}, nil)

	cfg := suite.oauthCfg
	cfg.OAuth.AllowedScopes = []string{"openid", "profile", "test"}
	cfg.OAuth.AllowedClaims = []string{"sub", "iss", "aud", "exp", "iat", "auth_time", "test_name"}
	cfg.OAuth.DefaultScopeClaimsMapping = map[string][]string{
		"openid":  {"sub"},
		"profile": {"name"},
		"test":    {"test_name"},
	}
	cfg.OAuth.AllowedSubjectTypes = []string{"public", "pairwise"}

	svc := newDiscoveryService(suite.cryptoMock, newTestJWEService(suite.cryptoMock), cfg)
	meta, err := svc.GetOIDCMetadata(context.Background())
	assert.NoError(suite.T(), err)

	assert.ElementsMatch(suite.T(), cfg.OAuth.AllowedScopes, meta.ScopesSupported)
	assert.ElementsMatch(suite.T(), cfg.OAuth.AllowedClaims, meta.ClaimsSupported)
	assert.ElementsMatch(suite.T(), cfg.OAuth.AllowedSubjectTypes, meta.SubjectTypesSupported)
}

func boolPtr(b bool) *bool { return &b }

// Both request object booleans must appear in the serialized document. request_parameter_supported
// defaults to false when omitted, but request_uri_parameter_supported defaults to true
// (OIDC Discovery 3), so omitting it would advertise support that does not exist.
func (suite *DiscoveryTestSuite) TestRequestObjectParametersSerializedExplicitly() {
	suite.cryptoMock.EXPECT().GetPublicKeys(mock.Anything, providers.PublicKeyFilter{}).
		Return([]providers.PublicKeyInfo{{KeyID: "k1", Algorithm: string(cryptolib.AlgorithmRS256)}}, nil)

	metadata, err := suite.discoveryService.GetOIDCMetadata(context.Background())
	assert.NoError(suite.T(), err)

	raw, err := json.Marshal(metadata)
	assert.NoError(suite.T(), err)

	var doc map[string]any
	assert.NoError(suite.T(), json.Unmarshal(raw, &doc))

	value, present := doc["request_uri_parameter_supported"]
	assert.True(suite.T(), present, "request_uri_parameter_supported must be present")
	assert.Equal(suite.T(), false, value)

	value, present = doc["request_parameter_supported"]
	assert.True(suite.T(), present, "request_parameter_supported must be present")
	assert.Equal(suite.T(), false, value)
}
