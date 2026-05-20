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
	hostLabelInternalClientID     = "hw_label_internal_client"
	hostLabelInternalClientSecret = "hw_label_internal_secret"
	hostMultiStarClientID         = "hw_multi_star_client"
	hostMultiStarClientSecret     = "hw_multi_star_secret"
	hostMixedPathClientID         = "hw_mixed_path_client"
	hostMixedPathClientSecret     = "hw_mixed_path_secret"
)

// HostWildcardRedirectURITestSuite covers wildcards in the host component of redirect URIs.
// It requires the server to be started with oauth.allow_wildcard_redirect_uri: true.
type HostWildcardRedirectURITestSuite struct {
	suite.Suite
	client *http.Client
	ouID   string
	appIDs []string
}

func TestHostWildcardRedirectURITestSuite(t *testing.T) {
	suite.Run(t, new(HostWildcardRedirectURITestSuite))
}

func (ts *HostWildcardRedirectURITestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "host-wildcard-redirect-uri-test-ou",
		Name:        "Host Wildcard Redirect URI Test OU",
		Description: "Organization unit for host wildcard redirect URI integration tests",
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
			name:         "hw-label-internal-app",
			clientID:     hostLabelInternalClientID,
			clientSecret: hostLabelInternalClientSecret,
			redirectURIs: []string{"https://app-*.gateway.example.com/cb"},
		},
		{
			name:         "hw-multi-star-app",
			clientID:     hostMultiStarClientID,
			clientSecret: hostMultiStarClientSecret,
			redirectURIs: []string{"https://tenant-app-*-*.gateway.example.com/cb"},
		},
		{
			name:         "hw-mixed-path-app",
			clientID:     hostMixedPathClientID,
			clientSecret: hostMixedPathClientSecret,
			redirectURIs: []string{"https://app-*.example.com/cb/*"},
		},
	}

	for _, app := range authzApps {
		appID, createErr := ts.createApp(app.name, app.clientID, app.clientSecret, app.redirectURIs)
		ts.Require().NoError(createErr, "Failed to create app %s", app.name)
		ts.appIDs = append(ts.appIDs, appID)
	}
}

func (ts *HostWildcardRedirectURITestSuite) TearDownSuite() {
	for _, id := range ts.appIDs {
		_ = testutils.DeleteApplication(id)
	}
	if ts.ouID != "" {
		_ = testutils.DeleteOrganizationUnit(ts.ouID)
	}
}

// createApp posts an application with the given redirect URIs and returns its ID.
func (ts *HostWildcardRedirectURITestSuite) createApp(name, clientID, clientSecret string, redirectURIs []string) (string, error) {
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
	return ts.postAppExpectCreated(payload)
}

// postApplication sends a POST /applications request and returns the raw response.
// The caller is responsible for closing the response body.
func (ts *HostWildcardRedirectURITestSuite) postApplication(name string, redirectURIs []string) (*http.Response, error) {
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

func (ts *HostWildcardRedirectURITestSuite) postAppExpectCreated(payload map[string]interface{}) (string, error) {
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

// ---------------------------------------------------------------------------
// Registration validation tests
// ---------------------------------------------------------------------------

// TestHWAC01_HostWildcardLabelInternal_Accepted verifies that a host wildcard placed
// inside a label between literal characters is accepted at registration.
func (ts *HostWildcardRedirectURITestSuite) TestHWAC01_HostWildcardLabelInternal_Accepted() {
	resp, err := ts.postApplication("hw-ac01", []string{"https://app-*.example.com/cb"})
	ts.Require().NoError(err)
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	ts.Equal(http.StatusCreated, resp.StatusCode, "body=%s", string(respBody))

	var result map[string]interface{}
	ts.Require().NoError(json.Unmarshal(respBody, &result))
	if id, _ := result["id"].(string); id != "" {
		ts.appIDs = append(ts.appIDs, id)
	}
}

// TestHWAC02_HostWholeLabelWildcard_Rejected verifies that a whole-label host wildcard
// (a label that is exactly "*") is rejected at registration.
func (ts *HostWildcardRedirectURITestSuite) TestHWAC02_HostWholeLabelWildcard_Rejected() {
	resp, err := ts.postApplication("hw-ac02", []string{"https://*.example.com/cb"})
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Equal(http.StatusBadRequest, resp.StatusCode)
}

// TestHWAC03_HostWildcardInPort_Rejected verifies that a wildcard in the port portion
// of host:port is rejected at registration.
func (ts *HostWildcardRedirectURITestSuite) TestHWAC03_HostWildcardInPort_Rejected() {
	resp, err := ts.postApplication("hw-ac03", []string{"https://app.example.com:80*0/cb"})
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Equal(http.StatusBadRequest, resp.StatusCode)
}

// TestHWAC04_HostWildcardWithExplicitPort_Accepted verifies that a host wildcard pattern
// with an explicit literal port is accepted at registration.
func (ts *HostWildcardRedirectURITestSuite) TestHWAC04_HostWildcardWithExplicitPort_Accepted() {
	resp, err := ts.postApplication("hw-ac04", []string{"https://app-*.example.com:8443/cb"})
	ts.Require().NoError(err)
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	ts.Equal(http.StatusCreated, resp.StatusCode, "body=%s", string(respBody))

	var result map[string]interface{}
	ts.Require().NoError(json.Unmarshal(respBody, &result))
	if id, _ := result["id"].(string); id != "" {
		ts.appIDs = append(ts.appIDs, id)
	}
}

// TestHWAC05_HostWildcardMultipleStars_Accepted verifies that multiple wildcards within
// a single label, separated by literal characters, are accepted.
func (ts *HostWildcardRedirectURITestSuite) TestHWAC05_HostWildcardMultipleStars_Accepted() {
	resp, err := ts.postApplication("hw-ac05",
		[]string{"https://tenant-app-*-*.gateway.example.com/cb"})
	ts.Require().NoError(err)
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	ts.Equal(http.StatusCreated, resp.StatusCode, "body=%s", string(respBody))

	var result map[string]interface{}
	ts.Require().NoError(json.Unmarshal(respBody, &result))
	if id, _ := result["id"].(string); id != "" {
		ts.appIDs = append(ts.appIDs, id)
	}
}

// ---------------------------------------------------------------------------
// Authorization request matching tests
// ---------------------------------------------------------------------------

// TestHWAC06_LabelInternalWildcard_Matches verifies that a label-internal '*' matches
// an alphanumeric run within the corresponding incoming label.
func (ts *HostWildcardRedirectURITestSuite) TestHWAC06_LabelInternalWildcard_Matches() {
	resp, err := testutils.InitiateAuthorizationFlow(
		hostLabelInternalClientID,
		"https://app-prod.gateway.example.com/cb",
		"code", "openid", "state_hwac06",
	)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Equal(http.StatusFound, resp.StatusCode)
	ts.True(isLoginPageRedirect(resp.Header.Get("Location")))
}

// TestHWAC07_LabelInternalWildcard_RejectsHyphen verifies that hyphens inside the dynamic
// portion are not matched by '*' (since '*' matches [0-9a-zA-Z]+, no hyphen).
func (ts *HostWildcardRedirectURITestSuite) TestHWAC07_LabelInternalWildcard_RejectsHyphen() {
	resp, err := testutils.InitiateAuthorizationFlow(
		hostLabelInternalClientID,
		"https://app-foo-bar.gateway.example.com/cb",
		"code", "openid", "state_hwac07",
	)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Equal(http.StatusFound, resp.StatusCode)
	ts.True(isErrorPageRedirect(resp.Header.Get("Location")))
}

// TestHWAC08_LabelInternalWildcard_RejectsDotInsideLabel verifies that '*' does not cross
// label boundaries — a dot inside the dynamic portion forces the match to fail.
func (ts *HostWildcardRedirectURITestSuite) TestHWAC08_LabelInternalWildcard_RejectsDotInsideLabel() {
	resp, err := testutils.InitiateAuthorizationFlow(
		hostLabelInternalClientID,
		"https://app-foo.bar.gateway.example.com/cb",
		"code", "openid", "state_hwac08",
	)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Equal(http.StatusFound, resp.StatusCode)
	ts.True(isErrorPageRedirect(resp.Header.Get("Location")))
}

// TestHWAC09_MultipleStarsInLabel_Matches verifies that two wildcards in the same label
// match alphanumeric runs separated by the literal hyphen.
func (ts *HostWildcardRedirectURITestSuite) TestHWAC09_MultipleStarsInLabel_Matches() {
	resp, err := testutils.InitiateAuthorizationFlow(
		hostMultiStarClientID,
		"https://tenant-app-019dfc78-f19ab4f2.gateway.example.com/cb",
		"code", "openid", "state_hwac09",
	)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Equal(http.StatusFound, resp.StatusCode)
	ts.True(isLoginPageRedirect(resp.Header.Get("Location")))
}

// TestHWAC10_HostWildcardCaseInsensitive_Matches verifies that host comparison remains
// case-insensitive when wildcards are involved.
func (ts *HostWildcardRedirectURITestSuite) TestHWAC10_HostWildcardCaseInsensitive_Matches() {
	resp, err := testutils.InitiateAuthorizationFlow(
		hostLabelInternalClientID,
		"https://APP-PROD.GATEWAY.EXAMPLE.com/cb",
		"code", "openid", "state_hwac10",
	)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Equal(http.StatusFound, resp.StatusCode)
	ts.True(isLoginPageRedirect(resp.Header.Get("Location")))
}

// TestHWAC11_HostAndPathWildcards_Matches verifies that host and path wildcards
// compose correctly within a single registered URI.
func (ts *HostWildcardRedirectURITestSuite) TestHWAC11_HostAndPathWildcards_Matches() {
	resp, err := testutils.InitiateAuthorizationFlow(
		hostMixedPathClientID,
		"https://app-staging.example.com/cb/v3",
		"code", "openid", "state_hwac11",
	)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Equal(http.StatusFound, resp.StatusCode)
	ts.True(isLoginPageRedirect(resp.Header.Get("Location")))
}

// TestHWAC12_LabelCountMismatch_RedirectsToErrorPage verifies that an incoming host with
// a different number of labels than the pattern is rejected.
func (ts *HostWildcardRedirectURITestSuite) TestHWAC12_LabelCountMismatch_RedirectsToErrorPage() {
	resp, err := testutils.InitiateAuthorizationFlow(
		hostLabelInternalClientID,
		"https://app-prod.dev.gateway.example.com/cb",
		"code", "openid", "state_hwac12",
	)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Equal(http.StatusFound, resp.StatusCode)
	ts.True(isErrorPageRedirect(resp.Header.Get("Location")))
}

// TestHWAC13_EmptyDynamicPart_RedirectsToErrorPage verifies that '*' requires at least one
// alphanumeric character — an empty dynamic portion does not match.
func (ts *HostWildcardRedirectURITestSuite) TestHWAC13_EmptyDynamicPart_RedirectsToErrorPage() {
	resp, err := testutils.InitiateAuthorizationFlow(
		hostLabelInternalClientID,
		"https://app-.gateway.example.com/cb",
		"code", "openid", "state_hwac13",
	)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	// url.Parse may reject hostnames with consecutive dots; if the request is locally
	// rejected the server still responds with an error page redirect.
	ts.Equal(http.StatusFound, resp.StatusCode)
	loc := resp.Header.Get("Location")
	ts.True(isErrorPageRedirect(loc) || strings.Contains(loc, "error"))
}
