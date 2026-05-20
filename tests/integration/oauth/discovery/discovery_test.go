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

package discovery

import (
	"encoding/json"

	"net/http"
	"testing"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

const (
	oauth2DiscoveryEndpoint = "/.well-known/oauth-authorization-server"
	oidcDiscoveryEndpoint   = "/.well-known/openid-configuration"
	testServerURL           = testutils.TestServerURL
)

// OAuth2AuthorizationServerMetadata represents OAuth2 Authorization Server Metadata (RFC 8414)
type OAuth2AuthorizationServerMetadata struct {
	Issuer                                     string   `json:"issuer"`
	AuthorizationEndpoint                      string   `json:"authorization_endpoint"`
	TokenEndpoint                              string   `json:"token_endpoint"`
	UserInfoEndpoint                           string   `json:"userinfo_endpoint,omitempty"`
	JWKSUri                                    string   `json:"jwks_uri"`
	RevocationEndpoint                         string   `json:"revocation_endpoint,omitempty"`
	IntrospectionEndpoint                      string   `json:"introspection_endpoint,omitempty"`
	RegistrationEndpoint                       string   `json:"registration_endpoint,omitempty"`
	ScopesSupported                            []string `json:"scopes_supported"`
	ResponseTypesSupported                     []string `json:"response_types_supported"`
	GrantTypesSupported                        []string `json:"grant_types_supported"`
	TokenEndpointAuthMethodsSupported          []string `json:"token_endpoint_auth_methods_supported"`
	CodeChallengeMethodsSupported              []string `json:"code_challenge_methods_supported,omitempty"`
	AuthorizationResponseIssParameterSupported bool     `json:"authorization_response_iss_parameter_supported"`
}

// OIDCProviderMetadata represents OpenID Connect Provider Metadata (OIDC Discovery 1.0)
type OIDCProviderMetadata struct {
	OAuth2AuthorizationServerMetadata
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

	var metadata OAuth2AuthorizationServerMetadata
	err = json.NewDecoder(resp.Body).Decode(&metadata)
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

	// Verify userinfo endpoint is present
	ts.NotEmpty(metadata.UserInfoEndpoint, "UserInfoEndpoint should be present")
	ts.Contains(metadata.UserInfoEndpoint, "/oauth2/userinfo", "UserInfoEndpoint should contain correct path")

	// Verify not implemented endpoints are empty
	ts.Empty(metadata.RevocationEndpoint, "RevocationEndpoint should be empty (not implemented)")

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

	// Verify only S256 code challenge method is supported (plain is prohibited per OAuth 2.0 Security BCP)
	ts.Equal([]string{"S256"}, metadata.CodeChallengeMethodsSupported,
		"CodeChallengeMethodsSupported should contain exactly S256")

	// Verify supported scopes
	ts.NotEmpty(metadata.ScopesSupported, "ScopesSupported should not be empty")
	ts.Contains(metadata.ScopesSupported, "openid", "Should support openid scope")

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

	// Verify not implemented endpoints are empty
	ts.Empty(metadata.EndSessionEndpoint, "EndSessionEndpoint should be empty (not implemented)")

	// Verify RFC 9207 issuer identification support
	ts.True(metadata.AuthorizationResponseIssParameterSupported,
		"authorization_response_iss_parameter_supported must be true (RFC 9207)")
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
	// ScopesSupported order may differ, so we check that they contain the same scopes
	ts.ElementsMatch(oauth2Metadata.ScopesSupported, oidcMetadata.ScopesSupported, "ScopesSupported should contain the same scopes")
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
