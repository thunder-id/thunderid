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

package serverconfig

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// ServerConfigAPITestSuite validates the generic server-config store API: list, get-by-name, the layered
// read (read-only declarative + writable + merged), PUT editing only the writable layer, and the
// validation/auth gates. CORS is the example payload only because it is the single registered config
// consumer; the dynamic CORS *behavior* is covered by the cors integration package.
type ServerConfigAPITestSuite struct {
	suite.Suite
	adminClient *http.Client
	plainClient *http.Client
}

func TestServerConfigAPITestSuite(t *testing.T) {
	suite.Run(t, new(ServerConfigAPITestSuite))
}

func (suite *ServerConfigAPITestSuite) SetupSuite() {
	suite.adminClient = testutils.GetHTTPClient()
	suite.plainClient = &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
}

// SetupTest clears the cors section so each test starts from a known empty state.
func (suite *ServerConfigAPITestSuite) SetupTest() {
	suite.putCORS(`[]`)
}

// TearDownSuite leaves the cors section empty so the shared server is not left mutated.
func (suite *ServerConfigAPITestSuite) TearDownSuite() {
	suite.putCORS(`[]`)
}

// --- GET /list ---

func (suite *ServerConfigAPITestSuite) TestListReturnsCorsSection() {
	status, body := suite.getJSON(suite.adminClient, serverConfigURL)

	suite.Equal(http.StatusOK, status)
	var names []string
	suite.Require().NoError(json.Unmarshal(body, &names))
	suite.Contains(names, "cors")
}

// --- GET by name (layered: read-only declarative + writable + merged) ---

func (suite *ServerConfigAPITestSuite) TestGetByNameMergesDeclarativeAndWritable() {
	suite.putCORS(fmt.Sprintf(`[%q]`, sampleOrigin))

	layers := suite.getLayers(corsConfigURL)
	suite.Equal([]string{quoted(declarativeOrigin)}, rawStrings(layers.ReadOnly.AllowedOrigins))
	suite.Equal([]string{quoted(sampleOrigin)}, rawStrings(layers.Writable.AllowedOrigins))
	// merged = readOnly (declarative) ∪ writable, declarative first.
	suite.Equal([]string{quoted(declarativeOrigin), quoted(sampleOrigin)}, rawStrings(layers.Merged.AllowedOrigins))
}

func (suite *ServerConfigAPITestSuite) TestGetByNameReturnsDeclarativeWhenWritableEmpty() {
	// SetupTest cleared the writable layer; the read-only declarative layer is always present.
	layers := suite.getLayers(corsConfigURL)
	suite.Empty(layers.Writable.AllowedOrigins)
	suite.Equal([]string{quoted(declarativeOrigin)}, rawStrings(layers.ReadOnly.AllowedOrigins))
	suite.Equal([]string{quoted(declarativeOrigin)}, rawStrings(layers.Merged.AllowedOrigins))
}

func (suite *ServerConfigAPITestSuite) TestMergeDeduplicatesOverlap() {
	// Writing the origin the declarative layer already declares must not duplicate it in merged.
	suite.putCORS(fmt.Sprintf(`[%q]`, declarativeOrigin))

	layers := suite.getLayers(corsConfigURL)
	suite.Equal([]string{quoted(declarativeOrigin)}, rawStrings(layers.Writable.AllowedOrigins))
	suite.Equal([]string{quoted(declarativeOrigin)}, rawStrings(layers.Merged.AllowedOrigins))
}

func (suite *ServerConfigAPITestSuite) TestGetByNameUnsupportedNameReturns400() {
	status, body := suite.getJSON(suite.adminClient, serverConfigURL+"/bogus")

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("SCF-1001", suite.errorCode(body))
}

// --- PUT edits only the writable layer; the declarative layer is read-only ---

func (suite *ServerConfigAPITestSuite) TestPutReplacesWritableLeavingDeclarativeIntact() {
	suite.putCORS(fmt.Sprintf(`[%q]`, sampleOrigin))
	suite.putCORS(fmt.Sprintf(`[%q]`, otherOrigin)) // replaces, does not append

	layers := suite.getLayers(corsConfigURL)
	suite.Equal([]string{quoted(otherOrigin)}, rawStrings(layers.Writable.AllowedOrigins))
	suite.Equal([]string{quoted(declarativeOrigin)}, rawStrings(layers.ReadOnly.AllowedOrigins))

	suite.putCORS(`[]`) // clearing the writable layer leaves the declarative layer untouched
	layers = suite.getLayers(corsConfigURL)
	suite.Empty(layers.Writable.AllowedOrigins)
	suite.Equal([]string{quoted(declarativeOrigin)}, rawStrings(layers.ReadOnly.AllowedOrigins))
	suite.Equal([]string{quoted(declarativeOrigin)}, rawStrings(layers.Merged.AllowedOrigins))
}

func (suite *ServerConfigAPITestSuite) TestPutInvalidValueReturns400AndDoesNotPersist() {
	status, body := suite.putRaw(suite.adminClient, corsBody(`["*"]`))
	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("SCF-1003", suite.errorCode(body))

	// Nothing persisted: the writable layer is still empty and the declarative layer is intact.
	layers := suite.getLayers(corsConfigURL)
	suite.Empty(layers.Writable.AllowedOrigins)
	suite.Equal([]string{quoted(declarativeOrigin)}, rawStrings(layers.ReadOnly.AllowedOrigins))
}

func (suite *ServerConfigAPITestSuite) TestPutMalformedBodyReturns400() {
	status, body := suite.putRaw(suite.adminClient, `[`)

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("SCF-1004", suite.errorCode(body))
}

// --- Auth gate ---

func (suite *ServerConfigAPITestSuite) TestUnauthenticatedRequestsAreRejected() {
	getStatus, getBody := suite.getJSON(suite.plainClient, corsConfigURL)
	suite.Equal(http.StatusUnauthorized, getStatus)
	suite.Equal("AUTH-4010", suite.errorCode(getBody))

	putStatus, _ := suite.putRaw(suite.plainClient, `[]`)
	suite.Equal(http.StatusUnauthorized, putStatus)
}

// --- helpers ---

// putCORS sets the cors writable layer to the given allowed-origins array and requires a 200 response.
func (suite *ServerConfigAPITestSuite) putCORS(allowedOrigins string) {
	status, _ := suite.putRaw(suite.adminClient, corsBody(allowedOrigins))
	suite.Require().Equal(http.StatusOK, status)
}

// corsBody wraps an allowed-origins array literal in the object-shaped section value the API expects.
func corsBody(allowedOrigins string) string {
	return `{"allowedOrigins":` + allowedOrigins + `}`
}

// putRaw sends a PUT /server-config/cors with the given body and returns the status and response body.
func (suite *ServerConfigAPITestSuite) putRaw(client *http.Client, body string) (int, []byte) {
	req, err := http.NewRequest(http.MethodPut, corsConfigURL, strings.NewReader(body))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer closeBodyQuietly(suite.T(), resp.Body)

	respBody, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)
	return resp.StatusCode, respBody
}

// getLayers GETs a section by URL, requires a 200 response, and decodes the three layers.
func (suite *ServerConfigAPITestSuite) getLayers(url string) serverConfigLayers {
	status, body := suite.getJSON(suite.adminClient, url)
	suite.Require().Equal(http.StatusOK, status)
	var layers serverConfigLayers
	suite.Require().NoError(json.Unmarshal(body, &layers))
	return layers
}

// rawStrings renders raw JSON layer elements as strings for order-sensitive comparison; a literal origin
// element is the quoted form "https://app.example.com".
func rawStrings(items []json.RawMessage) []string {
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = string(item)
	}
	return out
}

// quoted returns the JSON encoding of a literal origin string, matching how it appears as a layer element.
func quoted(s string) string {
	return fmt.Sprintf("%q", s)
}

// getJSON sends a GET to the given URL and returns the status and response body.
func (suite *ServerConfigAPITestSuite) getJSON(client *http.Client, url string) (int, []byte) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer closeBodyQuietly(suite.T(), resp.Body)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)
	return resp.StatusCode, body
}

// errorCode decodes the error code from an API error response body.
func (suite *ServerConfigAPITestSuite) errorCode(body []byte) string {
	var errResp apiErrorResponse
	suite.Require().NoError(json.Unmarshal(body, &errResp))
	return errResp.Code
}

func closeBodyQuietly(t *testing.T, body io.ReadCloser) {
	if body != nil {
		if err := body.Close(); err != nil {
			t.Logf("Failed to close response body: %v", err)
		}
	}
}
