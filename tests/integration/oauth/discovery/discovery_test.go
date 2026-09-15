// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"encoding/json"
	"io"

	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	oauth2DiscoveryEndpoint = "/.well-known/oauth-authorization-server"
	oidcDiscoveryEndpoint   = "/.well-known/openid-configuration"
	testServerURL           = testutils.TestServerURL
	// oidcCIBAGrantType is the OpenID Connect CIBA grant type identifier (providers.GrantTypeCIBA).
	oidcCIBAGrantType = "urn:openid:params:grant-type:ciba"
	// cibaBackchannelAuthEndpointPath is the backchannel authentication endpoint path
	// (oauth2const.OAuth2BackchannelAuthEndpoint).
	cibaBackchannelAuthEndpointPath = "/oauth2/bc-authorize"
)

// OAuth2AuthorizationServerMetadata represents OAuth2 Authorization Server Metadata (RFC 8414)
type OAuth2AuthorizationServerMetadata struct {
	Issuer                                     string   `json:"issuer"`
	AuthorizationEndpoint                      string   `json:"authorization_endpoint"`
	TokenEndpoint                              string   `json:"token_endpoint"`
	JWKSUri                                    string   `json:"jwks_uri"`
	RevocationEndpoint                         string   `json:"revocation_endpoint,omitempty"`
	IntrospectionEndpoint                      string   `json:"introspection_endpoint,omitempty"`
	RegistrationEndpoint                       string   `json:"registration_endpoint,omitempty"`
	BackchannelAuthenticationEndpoint          string   `json:"backchannel_authentication_endpoint,omitempty"`
	BackchannelTokenDeliveryModesSupported     []string `json:"backchannel_token_delivery_modes_supported,omitempty"`
	BackchannelUserCodeParameterSupported      bool     `json:"backchannel_user_code_parameter_supported"`
	ResponseTypesSupported                     []string `json:"response_types_supported"`
	GrantTypesSupported                        []string `json:"grant_types_supported"`
	TokenEndpointAuthMethodsSupported          []string `json:"token_endpoint_auth_methods_supported"`
	TokenEndpointAuthSigningAlgValuesSupported []string `json:"token_endpoint_auth_signing_alg_values_supported"`
	CodeChallengeMethodsSupported              []string `json:"code_challenge_methods_supported,omitempty"`
	AuthorizationResponseIssParameterSupported bool     `json:"authorization_response_iss_parameter_supported"`
}

// OIDCProviderMetadata represents OpenID Connect Provider Metadata (OIDC Discovery 1.0)
type OIDCProviderMetadata struct {
	OAuth2AuthorizationServerMetadata
	UserInfoEndpoint                 string   `json:"userinfo_endpoint"`
	ScopesSupported                  []string `json:"scopes_supported"`
	SubjectTypesSupported            []string `json:"subject_types_supported"`
	IDTokenSigningAlgValuesSupported []string `json:"id_token_signing_alg_values_supported"`
	ClaimsSupported                  []string `json:"claims_supported"`
	AcrValuesSupported               []string `json:"acr_values_supported,omitempty"`
	EndSessionEndpoint               string   `json:"end_session_endpoint,omitempty"`
}

type DiscoveryTestSuite struct {
	suite.Suite
	client *http.Client
}

func TestDiscoveryTestSuite(t *testing.T) {
	suite.Run(t, new(DiscoveryTestSuite))
}

func (ts *DiscoveryTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()
}

// TestOAuth2AuthorizationServerMetadata_GET_Success tests successful retrieval of OAuth2 Authorization Server Metadata
func (ts *DiscoveryTestSuite) TestOAuth2AuthorizationServerMetadata_GET_Success() {
	req, err := http.NewRequest("GET", testServerURL+oauth2DiscoveryEndpoint, nil)
	ts.Require().NoError(err)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Equal(http.StatusOK, resp.StatusCode)
	ts.Equal("application/json", resp.Header.Get("Content-Type"))

	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)

	var metadata OAuth2AuthorizationServerMetadata
	err = json.Unmarshal(body, &metadata)
	ts.Require().NoError(err)

	var rawMetadata map[string]json.RawMessage
	err = json.Unmarshal(body, &rawMetadata)
	ts.Require().NoError(err)

	// Verify required fields are present
	ts.NotEmpty(metadata.Issuer, "Issuer should be present")
	ts.NotEmpty(metadata.AuthorizationEndpoint, "AuthorizationEndpoint should be present")
	ts.NotEmpty(metadata.TokenEndpoint, "TokenEndpoint should be present")
	ts.NotEmpty(metadata.JWKSUri, "JWKSUri should be present")
	ts.NotEmpty(metadata.RegistrationEndpoint, "RegistrationEndpoint should be present")
	ts.NotEmpty(metadata.IntrospectionEndpoint, "IntrospectionEndpoint should be present")

	// Verify endpoints are correctly formatted
	ts.Contains(metadata.AuthorizationEndpoint, "/oauth2/authorize", "AuthorizationEndpoint should contain correct path")
	ts.Contains(metadata.TokenEndpoint, "/oauth2/token", "TokenEndpoint should contain correct path")
	ts.Contains(metadata.JWKSUri, "/oauth2/jwks", "JWKSUri should contain correct path")
	ts.Contains(metadata.RegistrationEndpoint, "/oauth2/dcr/register", "RegistrationEndpoint should contain correct path")
	ts.Contains(metadata.IntrospectionEndpoint, "/oauth2/introspect", "IntrospectionEndpoint should contain correct path")

	// Verify OIDC-specific fields are not present
	ts.NotContains(rawMetadata, "userinfo_endpoint", "OAuth metadata should not include the UserInfo endpoint")
	ts.NotContains(rawMetadata, "scopes_supported", "OAuth metadata should not include OIDC scopes")

	// Verify revocation endpoint is present
	ts.NotEmpty(metadata.RevocationEndpoint, "RevocationEndpoint should be present")
	ts.Contains(metadata.RevocationEndpoint, "/oauth2/revoke", "RevocationEndpoint should contain correct path")

	// Verify supported grant types
	ts.NotEmpty(metadata.GrantTypesSupported, "GrantTypesSupported should not be empty")
	ts.Contains(metadata.GrantTypesSupported, "authorization_code", "Should support authorization_code grant type")
	ts.Contains(metadata.GrantTypesSupported, "client_credentials", "Should support client_credentials grant type")
	ts.Contains(metadata.GrantTypesSupported, "refresh_token", "Should support refresh_token grant type")
	ts.NotContains(metadata.GrantTypesSupported, "password", "Should not support password grant type")
	ts.NotContains(metadata.GrantTypesSupported, "implicit", "Should not support implicit grant type")

	// Verify supported response types
	ts.NotEmpty(metadata.ResponseTypesSupported, "ResponseTypesSupported should not be empty")
	ts.Equal([]string{"code"}, metadata.ResponseTypesSupported, "Should only support 'code' response type")

	// Verify supported token endpoint auth methods
	ts.NotEmpty(metadata.TokenEndpointAuthMethodsSupported, "TokenEndpointAuthMethodsSupported should not be empty")
	ts.Contains(metadata.TokenEndpointAuthMethodsSupported, "client_secret_basic", "Should support client_secret_basic")
	ts.Contains(metadata.TokenEndpointAuthMethodsSupported, "client_secret_post", "Should support client_secret_post")
	ts.Contains(metadata.TokenEndpointAuthMethodsSupported, "none", "Should support none")

	// Verify token endpoint auth signing algs are advertised with FAPI 2.0 permitted algorithms (RFC 8414)
	ts.NotEmpty(metadata.TokenEndpointAuthSigningAlgValuesSupported,
		"token_endpoint_auth_signing_alg_values_supported should be present (FAPI 2.0)")
	ts.Contains(metadata.TokenEndpointAuthSigningAlgValuesSupported, "PS256", "Should advertise PS256")
	ts.Contains(metadata.TokenEndpointAuthSigningAlgValuesSupported, "ES256", "Should advertise ES256")
	ts.Contains(metadata.TokenEndpointAuthSigningAlgValuesSupported, "EdDSA", "Should advertise EdDSA")

	// Verify only S256 code challenge method is supported (plain is prohibited per OAuth 2.0 Security BCP)
	ts.Equal([]string{"S256"}, metadata.CodeChallengeMethodsSupported,
		"CodeChallengeMethodsSupported should contain exactly S256")

	// Verify RFC 9207 issuer identification support
	ts.True(metadata.AuthorizationResponseIssParameterSupported,
		"authorization_response_iss_parameter_supported must be true (RFC 9207)")
}

// TestOAuth2AuthorizationServerMetadata_OPTIONS_Success tests OPTIONS request for CORS
func (ts *DiscoveryTestSuite) TestOAuth2AuthorizationServerMetadata_OPTIONS_Success() {
	req, err := http.NewRequest("OPTIONS", testServerURL+oauth2DiscoveryEndpoint, nil)
	ts.Require().NoError(err)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Equal(http.StatusNoContent, resp.StatusCode)
}

// TestOIDCDiscovery_GET_Success tests successful retrieval of OIDC Provider Metadata
func (ts *DiscoveryTestSuite) TestOIDCDiscovery_GET_Success() {
	req, err := http.NewRequest("GET", testServerURL+oidcDiscoveryEndpoint, nil)
	ts.Require().NoError(err)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Equal(http.StatusOK, resp.StatusCode)
	ts.Equal("application/json", resp.Header.Get("Content-Type"))

	var metadata OIDCProviderMetadata
	err = json.NewDecoder(resp.Body).Decode(&metadata)
	ts.Require().NoError(err)

	// Verify all OAuth2 fields are inherited
	ts.NotEmpty(metadata.Issuer, "Issuer should be present")
	ts.NotEmpty(metadata.AuthorizationEndpoint, "AuthorizationEndpoint should be present")
	ts.NotEmpty(metadata.TokenEndpoint, "TokenEndpoint should be present")
	ts.NotEmpty(metadata.JWKSUri, "JWKSUri should be present")
	ts.NotEmpty(metadata.RegistrationEndpoint, "RegistrationEndpoint should be present")
	ts.NotEmpty(metadata.IntrospectionEndpoint, "IntrospectionEndpoint should be present")

	// Verify OIDC-specific fields
	ts.NotEmpty(metadata.SubjectTypesSupported, "SubjectTypesSupported should not be empty")
	ts.Contains(metadata.SubjectTypesSupported, "public", "Should support public subject type")

	ts.NotEmpty(metadata.IDTokenSigningAlgValuesSupported, "IDTokenSigningAlgValuesSupported should not be empty")
	ts.Contains(metadata.IDTokenSigningAlgValuesSupported, "RS256", "Should support RS256 signing algorithm")

	ts.NotEmpty(metadata.ClaimsSupported, "ClaimsSupported should not be empty")
	ts.NotEmpty(metadata.UserInfoEndpoint, "UserInfoEndpoint should be present")
	ts.Contains(metadata.UserInfoEndpoint, "/oauth2/userinfo", "UserInfoEndpoint should contain correct path")
	ts.NotEmpty(metadata.ScopesSupported, "ScopesSupported should not be empty")
	ts.Contains(metadata.ScopesSupported, "openid", "Should support openid scope")
	// Verify standard JWT claims
	ts.Contains(metadata.ClaimsSupported, "sub", "Should support sub claim")
	ts.Contains(metadata.ClaimsSupported, "iss", "Should support iss claim")
	ts.Contains(metadata.ClaimsSupported, "aud", "Should support aud claim")
	ts.Contains(metadata.ClaimsSupported, "exp", "Should support exp claim")
	ts.Contains(metadata.ClaimsSupported, "iat", "Should support iat claim")
	ts.Contains(metadata.ClaimsSupported, "auth_time", "Should support auth_time claim")

	// Verify OIDC scope claims are included
	ts.Contains(metadata.ClaimsSupported, "name", "Should support name claim (from profile scope)")
	ts.Contains(metadata.ClaimsSupported, "email", "Should support email claim (from email scope)")
	ts.Contains(metadata.ClaimsSupported, "phone_number", "Should support phone_number claim (from phone scope)")

	// Verify RP-initiated logout endpoint is advertised
	ts.NotEmpty(metadata.EndSessionEndpoint, "EndSessionEndpoint should be present")
	ts.Contains(metadata.EndSessionEndpoint, "/oauth2/logout", "EndSessionEndpoint should contain correct path")

	// Verify RFC 9207 issuer identification support
	ts.True(metadata.AuthorizationResponseIssParameterSupported,
		"authorization_response_iss_parameter_supported must be true (RFC 9207)")
}

func (ts *DiscoveryTestSuite) TestOIDCDiscovery_RequestObjectParametersNotSupported() {
	req, err := http.NewRequest("GET", testServerURL+oidcDiscoveryEndpoint, nil)
	ts.Require().NoError(err)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Equal(http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)

	var doc map[string]any
	ts.Require().NoError(json.Unmarshal(body, &doc))

	value, present := doc["request_uri_parameter_supported"]
	ts.True(present, "request_uri_parameter_supported must be present (its default when omitted is true)")
	ts.Equal(false, value, "request_uri_parameter_supported must be false")

	value, present = doc["request_parameter_supported"]
	ts.True(present, "request_parameter_supported must be present")
	ts.Equal(false, value, "request_parameter_supported must be false")
}

func (ts *DiscoveryTestSuite) TestOIDCDiscovery_AcrValuesSupported() {
	req, err := http.NewRequest("GET", testServerURL+oidcDiscoveryEndpoint, nil)
	ts.Require().NoError(err)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Equal(http.StatusOK, resp.StatusCode)

	var metadata OIDCProviderMetadata
	err = json.NewDecoder(resp.Body).Decode(&metadata)
	ts.Require().NoError(err)

	expectedACRs := []string{
		"urn:thunder:acr:password",
		"urn:thunder:acr:generated-code",
		"urn:thunder:acr:biometrics",
	}
	ts.ElementsMatch(expectedACRs, metadata.AcrValuesSupported,
		"acr_values_supported must contain exactly the ACR values from the ACR-AMR config")
}

// TestOIDCDiscovery_CIBAMetadata verifies the OIDC discovery document advertises CIBA support in
// poll-only mode: the backchannel authentication endpoint, poll-only delivery mode, no user_code
// parameter support, and the CIBA grant type in grant_types_supported. Ping/push delivery is not
// implemented, so only "poll" must appear.
func (ts *DiscoveryTestSuite) TestOIDCDiscovery_CIBAMetadata() {
	req, err := http.NewRequest("GET", testServerURL+oidcDiscoveryEndpoint, nil)
	ts.Require().NoError(err)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Equal(http.StatusOK, resp.StatusCode)

	var metadata OIDCProviderMetadata
	err = json.NewDecoder(resp.Body).Decode(&metadata)
	ts.Require().NoError(err)

	ts.Contains(metadata.GrantTypesSupported, oidcCIBAGrantType,
		"grant_types_supported must include the CIBA grant type")
	ts.NotEmpty(metadata.BackchannelAuthenticationEndpoint, "BackchannelAuthenticationEndpoint should be present")
	ts.True(strings.HasSuffix(metadata.BackchannelAuthenticationEndpoint, cibaBackchannelAuthEndpointPath),
		"BackchannelAuthenticationEndpoint should end with the bc-authorize path")
	ts.Equal([]string{"poll"}, metadata.BackchannelTokenDeliveryModesSupported,
		"only poll-mode delivery is implemented")
	ts.False(metadata.BackchannelUserCodeParameterSupported, "user_code is not supported")
}

// TestOIDCDiscovery_OPTIONS_Success tests OPTIONS request for CORS
func (ts *DiscoveryTestSuite) TestOIDCDiscovery_OPTIONS_Success() {
	req, err := http.NewRequest("OPTIONS", testServerURL+oidcDiscoveryEndpoint, nil)
	ts.Require().NoError(err)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Equal(http.StatusNoContent, resp.StatusCode)
}

// TestOAuth2MetadataConsistency tests that OAuth2 metadata is consistent between direct call and OIDC response
func (ts *DiscoveryTestSuite) TestOAuth2MetadataConsistency() {
	// Get OAuth2 metadata directly
	oauth2Req, err := http.NewRequest("GET", testServerURL+oauth2DiscoveryEndpoint, nil)
	ts.Require().NoError(err)

	oauth2Resp, err := ts.client.Do(oauth2Req)
	ts.Require().NoError(err)
	defer oauth2Resp.Body.Close()

	var oauth2Metadata OAuth2AuthorizationServerMetadata
	err = json.NewDecoder(oauth2Resp.Body).Decode(&oauth2Metadata)
	ts.Require().NoError(err)

	// Get OIDC metadata
	oidcReq, err := http.NewRequest("GET", testServerURL+oidcDiscoveryEndpoint, nil)
	ts.Require().NoError(err)

	oidcResp, err := ts.client.Do(oidcReq)
	ts.Require().NoError(err)
	defer oidcResp.Body.Close()

	var oidcMetadata OIDCProviderMetadata
	err = json.NewDecoder(oidcResp.Body).Decode(&oidcMetadata)
	ts.Require().NoError(err)

	// Verify OAuth2 fields are consistent
	ts.Equal(oauth2Metadata.Issuer, oidcMetadata.Issuer, "Issuer should match")
	ts.Equal(oauth2Metadata.AuthorizationEndpoint, oidcMetadata.AuthorizationEndpoint, "AuthorizationEndpoint should match")
	ts.Equal(oauth2Metadata.TokenEndpoint, oidcMetadata.TokenEndpoint, "TokenEndpoint should match")
	ts.Equal(oauth2Metadata.JWKSUri, oidcMetadata.JWKSUri, "JWKSUri should match")
	ts.Equal(oauth2Metadata.RegistrationEndpoint, oidcMetadata.RegistrationEndpoint, "RegistrationEndpoint should match")
	ts.Equal(oauth2Metadata.IntrospectionEndpoint, oidcMetadata.IntrospectionEndpoint, "IntrospectionEndpoint should match")
	ts.Equal(oauth2Metadata.GrantTypesSupported, oidcMetadata.GrantTypesSupported, "GrantTypesSupported should match")
	ts.Equal(oauth2Metadata.ResponseTypesSupported, oidcMetadata.ResponseTypesSupported, "ResponseTypesSupported should match")
	ts.Equal(oauth2Metadata.TokenEndpointAuthMethodsSupported, oidcMetadata.TokenEndpointAuthMethodsSupported, "TokenEndpointAuthMethodsSupported should match")
	ts.Equal(oauth2Metadata.CodeChallengeMethodsSupported, oidcMetadata.CodeChallengeMethodsSupported, "CodeChallengeMethodsSupported should match")
}

// TestDiscoveryEndpointsAccessibility tests that discovery endpoints are accessible without authentication
func (ts *DiscoveryTestSuite) TestDiscoveryEndpointsAccessibility() {
	endpoints := []string{oauth2DiscoveryEndpoint, oidcDiscoveryEndpoint}

	for _, endpoint := range endpoints {
		req, err := http.NewRequest("GET", testServerURL+endpoint, nil)
		ts.Require().NoError(err)

		// Don't set any authentication headers
		resp, err := ts.client.Do(req)
		ts.Require().NoError(err)
		defer resp.Body.Close()

		ts.Equal(http.StatusOK, resp.StatusCode, "Endpoint %s should be accessible without authentication", endpoint)
		ts.Equal("application/json", resp.Header.Get("Content-Type"), "Content-Type should be application/json for %s", endpoint)
	}
}
