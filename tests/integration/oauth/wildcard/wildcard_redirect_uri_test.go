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

package wildcard

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

const (
	singleStarClientID     = "wc_single_star_client"
	singleStarClientSecret = "wc_single_star_secret"
	doubleStarClientID     = "wc_double_star_client"
	doubleStarClientSecret = "wc_double_star_secret"
	exactClientID          = "wc_exact_client"
	exactClientSecret      = "wc_exact_secret"
	queryClientID          = "wc_query_client"
	queryClientSecret      = "wc_query_secret"
	multiClientID          = "wc_multi_client"
	multiClientSecret      = "wc_multi_secret"
)

// WildcardRedirectURITestSuite tests wildcard redirect URI registration validation and authorization matching.
// It requires the server to be started with oauth.allow_wildcard_redirect_uri: true.
type WildcardRedirectURITestSuite struct {
	suite.Suite
	client *http.Client
	ouID   string
	appIDs []string
}

// TestWildcardRedirectURITestSuite is the entry point for the test suite.
func TestWildcardRedirectURITestSuite(t *testing.T) {
	suite.Run(t, new(WildcardRedirectURITestSuite))
}

func (ts *WildcardRedirectURITestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "wildcard-redirect-uri-test-ou",
		Name:        "Wildcard Redirect URI Test OU",
		Description: "Organization unit for wildcard redirect URI integration tests",
	})
	ts.Require().NoError(err, "Failed to create organization unit")
	ts.ouID = ouID

	authzApps := []struct {
		name         string
		clientID     string
		clientSecret string
		redirectURIs []string
	}{
		{
			name:         "wc-single-star-app",
			clientID:     singleStarClientID,
			clientSecret: singleStarClientSecret,
			redirectURIs: []string{"https://client.example.com/cb/*"},
		},
		{
			name:         "wc-double-star-app",
			clientID:     doubleStarClientID,
			clientSecret: doubleStarClientSecret,
			redirectURIs: []string{"https://client.example.com/app/**/cb"},
		},
		{
			name:         "wc-exact-app",
			clientID:     exactClientID,
			clientSecret: exactClientSecret,
			redirectURIs: []string{"https://client.example.com/callback"},
		},
		{
			name:         "wc-query-app",
			clientID:     queryClientID,
			clientSecret: queryClientSecret,
			redirectURIs: []string{"https://client.example.com/cb?foo=bar"},
		},
		{
			name:         "wc-multi-app",
			clientID:     multiClientID,
			clientSecret: multiClientSecret,
			redirectURIs: []string{
				"https://client.example.com/a/*",
				"https://client.example.com/b/*",
			},
		},
	}

	for _, app := range authzApps {
		appID, createErr := ts.createApp(app.name, app.clientID, app.clientSecret, app.redirectURIs)
		ts.Require().NoError(createErr, "Failed to create app %s", app.name)
		ts.appIDs = append(ts.appIDs, appID)
	}
}

func (ts *WildcardRedirectURITestSuite) TearDownSuite() {
	for _, id := range ts.appIDs {
		_ = testutils.DeleteApplication(id)
	}
	if ts.ouID != "" {
		_ = testutils.DeleteOrganizationUnit(ts.ouID)
	}
}

// createApp posts an application with the given redirect URIs and returns its ID.
// It hard-codes clientId/clientSecret so authorization requests can use known values.
func (ts *WildcardRedirectURITestSuite) createApp(name, clientID, clientSecret string, redirectURIs []string) (string, error) {
	payload := map[string]interface{}{
		"name": name,
		"ouId": ts.ouID,
		"inboundAuthConfig": []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                clientID,
					"clientSecret":            clientSecret,
					"redirectUris":            redirectURIs,
					"grantTypes":              []string{"authorization_code"},
					"responseTypes":           []string{"code"},
					"tokenEndpointAuthMethod": "client_secret_basic",
					"pkceRequired":            false,
					"publicClient":            false,
				},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", testutils.TestServerURL+"/applications", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("expected 201, got %d: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	id, ok := result["id"].(string)
	if !ok {
		return "", fmt.Errorf("response missing string id field")
	}
	return id, nil
}

// postApplication sends a POST /applications request and returns the raw response.
// The caller is responsible for closing the response body.
func (ts *WildcardRedirectURITestSuite) postApplication(name string, redirectURIs []string) (*http.Response, error) {
	payload := map[string]interface{}{
		"name": name,
		"ouId": ts.ouID,
		"inboundAuthConfig": []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"redirectUris":            redirectURIs,
					"grantTypes":              []string{"authorization_code"},
					"responseTypes":           []string{"code"},
					"tokenEndpointAuthMethod": "client_secret_basic",
				},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", testutils.TestServerURL+"/applications", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	return ts.client.Do(req)
}

// isErrorPageRedirect returns true when the Location header indicates server's own error page.
func isErrorPageRedirect(location string) bool {
	return strings.Contains(location, "errorCode=")
}

// isLoginPageRedirect returns true when the Location header leads to the login/flow page.
func isLoginPageRedirect(location string) bool {
	return location != "" && !strings.Contains(location, "errorCode=")
}

// ---------------------------------------------------------------------------
// Registration Validation Tests
// ---------------------------------------------------------------------------

// TestAC01_WildcardInScheme_Rejected verifies that a redirect URI with a wildcard
// in the scheme component is rejected with 400.
func (ts *WildcardRedirectURITestSuite) TestAC01_WildcardInScheme_Rejected() {
	resp, err := ts.postApplication("wc-ac01", []string{"http*://example.com/callback"})
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Equal(http.StatusBadRequest, resp.StatusCode)
}

// TestAC02_WildcardInHost_Rejected verifies that a redirect URI with a wildcard
// in the host component is rejected with 400.
func (ts *WildcardRedirectURITestSuite) TestAC02_WildcardInHost_Rejected() {
	resp, err := ts.postApplication("wc-ac02", []string{"https://*.example.com/callback"})
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Equal(http.StatusBadRequest, resp.StatusCode)
}

// TestAC03_WildcardInQuery_Rejected verifies that a redirect URI with a wildcard
// in the query string is rejected with 400.
func (ts *WildcardRedirectURITestSuite) TestAC03_WildcardInQuery_Rejected() {
	resp, err := ts.postApplication("wc-ac03", []string{"https://example.com/callback?foo=*"})
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Equal(http.StatusBadRequest, resp.StatusCode)
}

// TestAC04_WildcardInPath_Accepted verifies that a redirect URI with a wildcard
// only in the path component is accepted (201) and stored as-is.
func (ts *WildcardRedirectURITestSuite) TestAC04_WildcardInPath_Accepted() {
	resp, err := ts.postApplication("wc-ac04", []string{"https://example.com/callback/*"})
	ts.Require().NoError(err)
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	ts.Equal(http.StatusCreated, resp.StatusCode)

	var result map[string]interface{}
	ts.Require().NoError(json.Unmarshal(respBody, &result))

	appID, _ := result["id"].(string)
	if appID != "" {
		ts.appIDs = append(ts.appIDs, appID)
	}

	inbound, _ := result["inboundAuthConfig"].([]interface{})
	ts.Require().NotEmpty(inbound)
	cfg, _ := inbound[0].(map[string]interface{})["config"].(map[string]interface{})
	uris, _ := cfg["redirectUris"].([]interface{})
	ts.Contains(uris, "https://example.com/callback/*")
}

// TestAC05_RegexInPath_Rejected verifies that a redirect URI with regex metacharacters
// in the path is rejected with 400.
func (ts *WildcardRedirectURITestSuite) TestAC05_RegexInPath_Rejected() {
	resp, err := ts.postApplication("wc-ac05", []string{"https://example.com/callback/[a-z]+"})
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Equal(http.StatusBadRequest, resp.StatusCode)
}

// TestAC13_DeeplinkWildcardInPath_Accepted verifies that a deeplink redirect URI with
// a wildcard in the path is accepted.
func (ts *WildcardRedirectURITestSuite) TestAC13_DeeplinkWildcardInPath_Accepted() {
	resp, err := ts.postApplication("wc-ac13", []string{"myapp://callback/*"})
	ts.Require().NoError(err)
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	ts.Equal(http.StatusCreated, resp.StatusCode)

	var result map[string]interface{}
	ts.Require().NoError(json.Unmarshal(respBody, &result))
	if id, _ := result["id"].(string); id != "" {
		ts.appIDs = append(ts.appIDs, id)
	}
}

// TestAC14_DeeplinkWildcardInScheme_Rejected verifies that a deeplink redirect URI with
// a wildcard in the scheme is rejected with 400.
func (ts *WildcardRedirectURITestSuite) TestAC14_DeeplinkWildcardInScheme_Rejected() {
	resp, err := ts.postApplication("wc-ac14", []string{"my*app://callback"})
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Equal(http.StatusBadRequest, resp.StatusCode)
}

// ---------------------------------------------------------------------------
// Authorization Request Matching Tests
// ---------------------------------------------------------------------------

// TestAC06_SingleStar_MatchesOneSegment verifies that * matches exactly one path segment.
func (ts *WildcardRedirectURITestSuite) TestAC06_SingleStar_MatchesOneSegment() {
	resp, err := testutils.InitiateAuthorizationFlow(
		singleStarClientID,
		"https://client.example.com/cb/v1",
		"code", "openid", "state_ac06a",
	)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Equal(http.StatusFound, resp.StatusCode)
	ts.True(isLoginPageRedirect(resp.Header.Get("Location")))
}

// TestAC06_SingleStar_RejectsTwoSegments verifies that * does not match two path segments.
func (ts *WildcardRedirectURITestSuite) TestAC06_SingleStar_RejectsTwoSegments() {
	resp, err := testutils.InitiateAuthorizationFlow(
		singleStarClientID,
		"https://client.example.com/cb/v1/extra",
		"code", "openid", "state_ac06b",
	)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Equal(http.StatusFound, resp.StatusCode)
	ts.True(isErrorPageRedirect(resp.Header.Get("Location")))
}

// TestAC07_DoubleStar_MatchesMultipleSegments verifies that ** matches multiple path segments.
func (ts *WildcardRedirectURITestSuite) TestAC07_DoubleStar_MatchesMultipleSegments() {
	resp, err := testutils.InitiateAuthorizationFlow(
		doubleStarClientID,
		"https://client.example.com/app/tenant/region/cb",
		"code", "openid", "state_ac07a",
	)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Equal(http.StatusFound, resp.StatusCode)
	ts.True(isLoginPageRedirect(resp.Header.Get("Location")))
}

// TestAC07_DoubleStar_MatchesZeroSegments verifies that ** matches zero path segments.
func (ts *WildcardRedirectURITestSuite) TestAC07_DoubleStar_MatchesZeroSegments() {
	resp, err := testutils.InitiateAuthorizationFlow(
		doubleStarClientID,
		"https://client.example.com/app/cb",
		"code", "openid", "state_ac07b",
	)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Equal(http.StatusFound, resp.StatusCode)
	ts.True(isLoginPageRedirect(resp.Header.Get("Location")))
}

// TestAC08_ExactMatch_Accepted verifies that an exact (non-wildcard) redirect URI is accepted.
func (ts *WildcardRedirectURITestSuite) TestAC08_ExactMatch_Accepted() {
	resp, err := testutils.InitiateAuthorizationFlow(
		exactClientID,
		"https://client.example.com/callback",
		"code", "openid", "state_ac08",
	)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Equal(http.StatusFound, resp.StatusCode)
	ts.True(isLoginPageRedirect(resp.Header.Get("Location")))
}

// TestAC09_NoMatch_RedirectsToErrorPage verifies that an unmatched redirect URI causes
// the server to redirect to its own error page.
func (ts *WildcardRedirectURITestSuite) TestAC09_NoMatch_RedirectsToErrorPage() {
	resp, err := testutils.InitiateAuthorizationFlow(
		singleStarClientID,
		"https://client.example.com/other",
		"code", "openid", "state_ac09",
	)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Equal(http.StatusFound, resp.StatusCode)
	ts.True(isErrorPageRedirect(resp.Header.Get("Location")))
}

// TestAC10_QueryMismatch_RedirectsToErrorPage verifies that a redirect URI whose query
// string does not match the registered pattern is rejected.
func (ts *WildcardRedirectURITestSuite) TestAC10_QueryMismatch_RedirectsToErrorPage() {
	resp, err := testutils.InitiateAuthorizationFlow(
		queryClientID,
		"https://client.example.com/cb?foo=baz",
		"code", "openid", "state_ac10",
	)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Equal(http.StatusFound, resp.StatusCode)
	ts.True(isErrorPageRedirect(resp.Header.Get("Location")))
}

// TestAC11_MultipleURIs_MatchesAny verifies that when multiple redirect URIs are
// registered, a URI matching any of the registered patterns is accepted.
func (ts *WildcardRedirectURITestSuite) TestAC11_MultipleURIs_MatchesAny() {
	resp, err := testutils.InitiateAuthorizationFlow(
		multiClientID,
		"https://client.example.com/b/x",
		"code", "openid", "state_ac11",
	)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Equal(http.StatusFound, resp.StatusCode)
	ts.True(isLoginPageRedirect(resp.Header.Get("Location")))
}

// TestAC12_WildcardRegistered_OmittedRedirectURI_Rejected verifies that omitting the
// redirect_uri in the authorization request is rejected when only wildcard URIs are registered
// (RFC 6749 §3.1.2.3 — when multiple or wildcard URIs are registered, the client must include
// a redirect_uri; a wildcard pattern has no single concrete URI to fall back to).
func (ts *WildcardRedirectURITestSuite) TestAC12_WildcardRegistered_OmittedRedirectURI_Rejected() {
	resp, err := testutils.InitiateAuthorizationFlow(
		singleStarClientID,
		"",
		"code", "openid", "state_ac12",
	)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Equal(http.StatusFound, resp.StatusCode)
	ts.True(isErrorPageRedirect(resp.Header.Get("Location")))
}
