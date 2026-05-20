/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package authz

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

const (
	declPublicClientID       = "decl-public-client-1"
	declConfidentialClientID = "decl-conf-client-1"
	declConfidentialSecret   = "decl-conf-secret-1"
	declRedirectURI          = "https://localhost:3000"
)

// DeclarativeAuthzTestSuite tests that declarative YAML-loaded applications are
// correctly indexed by clientId and resolvable at the authorize endpoint.
// This is a regression suite for issue #2269 (public clients without a
// client_secret had their clientId omitted from SystemAttributes, causing
// invalid_request on every authorize call).
//
// No SetupSuite/TearDownSuite are needed: the declarative fixtures are loaded
// by the integration bootstrap before the test suite runs.
type DeclarativeAuthzTestSuite struct {
	suite.Suite
	client *http.Client
}

func TestDeclarativeAuthzTestSuite(t *testing.T) {
	suite.Run(t, new(DeclarativeAuthzTestSuite))
}

func (ts *DeclarativeAuthzTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()
}

// TestPublicClientAuthorize verifies that a declarative public OAuth client
// (no client_secret, token_endpoint_auth_method=none) can initiate an
// authorization code flow. A valid redirect with authId and flowId in the
// Location header is the expected outcome.
func (ts *DeclarativeAuthzTestSuite) TestPublicClientAuthorize() {
	codeVerifier := "decl-public-pkce-verifier-1234567890-abcdefghijklmnopqrstuvwxyz"
	codeChallenge := buildS256CodeChallenge(codeVerifier)

	resp, err := testutils.InitiateAuthorizationFlowWithPKCE(
		declPublicClientID,
		declRedirectURI,
		"code",
		"openid",
		"decl_state_public",
		"",
		codeChallenge,
		"S256",
	)
	ts.Require().NoError(err, "failed to initiate authorization flow for declarative public client")
	defer resp.Body.Close()

	ts.Equal(http.StatusFound, resp.StatusCode)

	location := resp.Header.Get("Location")
	ts.NotEmpty(location, "expected redirect location header")
	ts.assertNotOAuthErrorRedirect(location)
}

// TestConfidentialClientAuthorize verifies that a declarative confidential OAuth
// client (with client_secret, token_endpoint_auth_method=client_secret_basic)
// can initiate an authorization code flow at the authorize endpoint.
func (ts *DeclarativeAuthzTestSuite) TestConfidentialClientAuthorize() {
	resp, err := testutils.InitiateAuthorizationFlow(
		declConfidentialClientID, declRedirectURI, "code", "openid", "decl_state_conf",
	)
	ts.Require().NoError(err, "failed to initiate authorization flow for declarative confidential client")
	defer resp.Body.Close()

	ts.Equal(http.StatusFound, resp.StatusCode)

	location := resp.Header.Get("Location")
	ts.NotEmpty(location, "expected redirect location header")
	ts.assertNotOAuthErrorRedirect(location)
}

// TestConfidentialClientClientCredentialsGrant verifies that a declarative
// confidential client can authenticate at the token endpoint using
// client_secret_basic for the client_credentials grant.
func (ts *DeclarativeAuthzTestSuite) TestConfidentialClientClientCredentialsGrant() {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")

	req, err := http.NewRequest(
		http.MethodPost,
		testutils.TestServerURL+"/oauth2/token",
		bytes.NewBufferString(form.Encode()),
	)
	ts.Require().NoError(err, "failed to create token request")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(declConfidentialClientID, declConfidentialSecret)

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	resp, err := client.Do(req)
	ts.Require().NoError(err, "failed to execute token request")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err, "failed to read token response")

	ts.Equal(http.StatusOK, resp.StatusCode, "unexpected token endpoint response: %s", string(body))

	var tokenResp map[string]interface{}
	err = json.Unmarshal(body, &tokenResp)
	ts.Require().NoError(err, "failed to parse token response body: %s", string(body))
	ts.NotEmpty(tokenResp["access_token"], "expected access_token in token response")
	ts.Equal("Bearer", tokenResp["token_type"])
}

// assertNotOAuthErrorRedirect validates the authorize redirect is not an OAuth
// error redirect. This keeps the test focused on client resolution success
// without coupling to a specific interaction redirect shape.
func (ts *DeclarativeAuthzTestSuite) assertNotOAuthErrorRedirect(location string) {
	parsedURL, err := url.Parse(location)
	ts.Require().NoError(err, "failed to parse redirect URL")

	queryParams := parsedURL.Query()
	if queryParams.Get("error") != "" || queryParams.Get("errorCode") != "" {
		ts.Failf("unexpected OAuth error redirect", "location: %s", location)
	}
}

func buildS256CodeChallenge(codeVerifier string) string {
	hash := sha256.Sum256([]byte(codeVerifier))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

// TestUnknownClientIDReturnsInvalidRequest verifies that an authorize request
// with an unknown clientId is rejected with an invalid_request error redirect.
func (ts *DeclarativeAuthzTestSuite) TestUnknownClientIDReturnsInvalidRequest() {
	resp, err := testutils.InitiateAuthorizationFlow(
		"unknown-declarative-client", declRedirectURI, "code", "openid", "decl_state_invalid",
	)
	ts.Require().NoError(err, "failed to send authorization request")
	defer resp.Body.Close()

	ts.Equal(http.StatusFound, resp.StatusCode)

	location := resp.Header.Get("Location")
	ts.NotEmpty(location, "expected redirect location header")

	err = testutils.ValidateOAuth2ErrorRedirect(location, "invalid_request", "Invalid client_id")
	ts.NoError(err, "expected invalid_request error for unknown clientId")
}

// TestMissingClientIDReturnsInvalidRequest verifies that an authorize request
// with no clientId is rejected with an invalid_request error redirect.
func (ts *DeclarativeAuthzTestSuite) TestMissingClientIDReturnsInvalidRequest() {
	resp, err := testutils.InitiateAuthorizationFlow(
		"", declRedirectURI, "code", "openid", "decl_state_missing_client",
	)
	ts.Require().NoError(err, "failed to send authorization request")
	defer resp.Body.Close()

	ts.Equal(http.StatusFound, resp.StatusCode)

	location := resp.Header.Get("Location")
	ts.NotEmpty(location, "expected redirect location header")

	err = testutils.ValidateOAuth2ErrorRedirect(location, "invalid_request", "Missing client_id parameter")
	ts.NoError(err, "expected invalid_request error for missing clientId")
}

// TestInvalidRedirectURIReturnsInvalidRequest verifies that an authorize request
// with a redirect_uri not registered for the declarative public client is
// rejected with an invalid_request error redirect.
func (ts *DeclarativeAuthzTestSuite) TestInvalidRedirectURIReturnsInvalidRequest() {
	resp, err := testutils.InitiateAuthorizationFlow(
		declPublicClientID, "https://evil.example.com/callback", "code", "openid", "decl_state_bad_redirect",
	)
	ts.Require().NoError(err, "failed to send authorization request")
	defer resp.Body.Close()

	ts.Equal(http.StatusFound, resp.StatusCode)

	location := resp.Header.Get("Location")
	ts.NotEmpty(location, "expected redirect location header")

	err = testutils.ValidateOAuth2ErrorRedirect(location, "invalid_request", "Invalid redirect URI")
	ts.NoError(err, "expected invalid_request error for unregistered redirect URI")
}

// TestInvalidRedirectURIForConfidentialClientReturnsInvalidRequest verifies that
// a redirect_uri not registered for the declarative confidential client is
// also rejected with an invalid_request error redirect.
func (ts *DeclarativeAuthzTestSuite) TestInvalidRedirectURIForConfidentialClientReturnsInvalidRequest() {
	resp, err := testutils.InitiateAuthorizationFlow(
		declConfidentialClientID, "https://evil.example.com/callback", "code", "openid", "decl_state_conf_bad_redirect",
	)
	ts.Require().NoError(err, "failed to send authorization request")
	defer resp.Body.Close()

	ts.Equal(http.StatusFound, resp.StatusCode)

	location := resp.Header.Get("Location")
	ts.NotEmpty(location, "expected redirect location header")

	err = testutils.ValidateOAuth2ErrorRedirect(location, "invalid_request", "Invalid redirect URI")
	ts.NoError(err, "expected invalid_request error for unregistered redirect URI on confidential client")
}
