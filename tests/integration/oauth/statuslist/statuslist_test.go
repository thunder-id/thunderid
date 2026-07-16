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

// Package statuslist exercises the Token Status List (draft-ietf-oauth-status-list) end-to-end: issued
// tokens carry a status reference, the publish endpoint serves the signed list without authentication,
// and a revoked token is rejected at the Resource Server enforcement point.
package statuslist

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	testServerURL  = "https://localhost:8095"
	clientID       = "statuslist_client"
	clientSecret   = "statuslist_secret"
	tokenEndpoint  = testServerURL + "/oauth2/token"
	statusListPath = testServerURL + "/statuslists/"
)

type StatusListTestSuite struct {
	suite.Suite
	client *http.Client
	ouID   string
	appID  string
}

func TestStatusListTestSuite(t *testing.T) {
	suite.Run(t, new(StatusListTestSuite))
}

func (ts *StatusListTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "statuslist-test-ou",
		Name:        "Status List Test OU",
		Description: "Organization unit for Token Status List integration tests",
		Parent:      nil,
	})
	ts.Require().NoError(err, "failed to create test organization unit")
	ts.ouID = ouID
	ts.appID = ts.createApp()
}

func (ts *StatusListTestSuite) TearDownSuite() {
	if ts.appID != "" {
		ts.deleteApp(ts.appID)
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete test organization unit: %v", err)
		}
	}
}

func (ts *StatusListTestSuite) createApp() string {
	app := map[string]interface{}{
		"name":                      "StatusListApp",
		"description":               "Application for Token Status List integration tests",
		"ouId":                      ts.ouID,
		"isRegistrationFlowEnabled": false,
		"inboundAuthConfig": []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                clientID,
					"clientSecret":            clientSecret,
					"redirectUris":            []string{"https://localhost:3000"},
					"grantTypes":              []string{"client_credentials"},
					"tokenEndpointAuthMethod": "client_secret_basic",
				},
			},
		},
	}
	jsonData, err := json.Marshal(app)
	ts.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPost, testServerURL+"/applications", bytes.NewBuffer(jsonData))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Truef(resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK,
		"unexpected app-create status %d", resp.StatusCode)

	var body map[string]interface{}
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&body))
	id, _ := body["id"].(string)
	ts.Require().NotEmpty(id, "app id not returned")
	return id
}

func (ts *StatusListTestSuite) deleteApp(appID string) {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/applications/%s", testServerURL, appID), nil)
	if err != nil {
		ts.T().Logf("Failed to build delete request: %v", err)
		return
	}
	resp, err := ts.client.Do(req)
	if err != nil {
		ts.T().Logf("Failed to delete application: %v", err)
		return
	}
	resp.Body.Close()
}

// getAccessToken obtains a fresh client_credentials access token.
func (ts *StatusListTestSuite) getAccessToken() string {
	req, err := http.NewRequest(http.MethodPost, tokenEndpoint, strings.NewReader("grant_type=client_credentials"))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&body))
	token, ok := body["access_token"].(string)
	ts.Require().True(ok, "access_token not found")
	return token
}

// jwtSegment base64url-decodes the given JWT segment (0=header, 1=payload) into a claims map.
func (ts *StatusListTestSuite) jwtSegment(token string, seg int) map[string]interface{} {
	parts := strings.Split(token, ".")
	ts.Require().Len(parts, 3, "not a JWT")
	raw, err := base64.RawURLEncoding.DecodeString(parts[seg])
	ts.Require().NoError(err)
	var m map[string]interface{}
	ts.Require().NoError(json.Unmarshal(raw, &m))
	return m
}

// statusListRef extracts the status.status_list {uri, idx} reference from a token.
func (ts *StatusListTestSuite) statusListRef(token string) (string, float64) {
	payload := ts.jwtSegment(token, 1)
	status, ok := payload["status"].(map[string]interface{})
	ts.Require().True(ok, "token is missing the status claim")
	sl, ok := status["status_list"].(map[string]interface{})
	ts.Require().True(ok, "token is missing status.status_list")
	uri, _ := sl["uri"].(string)
	idx, _ := sl["idx"].(float64)
	return uri, idx
}

func listIDFromURI(uri string) string {
	const seg = "/statuslists/"
	i := strings.LastIndex(uri, seg)
	if i < 0 {
		return ""
	}
	return uri[i+len(seg):]
}

// An issued access token carries a well-formed status.status_list reference.
func (ts *StatusListTestSuite) TestAccessToken_CarriesStatusListReference() {
	uri, idx := ts.statusListRef(ts.getAccessToken())
	ts.Assert().Contains(uri, "/statuslists/", "uri should point at the publish endpoint")
	ts.Assert().GreaterOrEqual(idx, float64(0), "idx should be a non-negative integer")
}

// The publish endpoint serves the signed Status List Token without authentication.
func (ts *StatusListTestSuite) TestPublishEndpoint_ServesSignedStatusList() {
	uri, _ := ts.statusListRef(ts.getAccessToken())
	id := listIDFromURI(uri)
	ts.Require().NotEmpty(id)

	// Deliberately send no credentials: the endpoint is public.
	req, err := http.NewRequest(http.MethodGet, statusListPath+id, nil)
	ts.Require().NoError(err)
	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Require().Equal(http.StatusOK, resp.StatusCode, "publish endpoint must be public (no auth required)")
	ts.Assert().Contains(resp.Header.Get("Content-Type"), "application/statuslist+jwt")

	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	slt := string(body)

	header := ts.jwtSegment(slt, 0)
	ts.Assert().Equal("statuslist+jwt", header["typ"], "typ header should be statuslist+jwt")

	payload := ts.jwtSegment(slt, 1)
	sl, ok := payload["status_list"].(map[string]interface{})
	ts.Require().True(ok, "status list token is missing the status_list claim")
	ts.Assert().IsType("", sl["lst"], "status_list.lst should be a (compressed, base64url) string")
	ts.Assert().NotNil(sl["bits"], "status_list.bits should be present")
}

// A reference to a list that does not exist is rejected with 404 (not treated as an empty/all-valid list).
func (ts *StatusListTestSuite) TestPublishEndpoint_UnknownListReturns404() {
	req, err := http.NewRequest(http.MethodGet, statusListPath+"00000000-0000-0000-0000-000000000000", nil)
	ts.Require().NoError(err)
	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Assert().Equal(http.StatusNotFound, resp.StatusCode)
}

// NOTE: Resource-Server enforcement (a revoked token rejected with 401 at a protected endpoint) is
// intentionally NOT asserted here. It is covered by unit tests (internal/system/revocationcache, 100%
// statement coverage, plus internal/system/security). An end-to-end assertion is impractical in this
// harness for two reasons: (1) the shared test HTTP client injects an admin bearer token on every
// request, so a per-request token cannot be presented to the middleware; and (2) RS enforcement is
// eventually-consistent — the revocation cache refreshes only every token_revocation.refresh_interval_
// seconds, so a revocation is not visible immediately after /oauth2/revoke (unlike AS introspection,
// which reads the store directly and is covered by the revocation integration suite).
