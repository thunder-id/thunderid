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

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

// ServerConfigAPITestSuite validates the generic server-config store API: list and get-by-name, the
// validation and auth gates, and the always-present empty defaults. CORS is the example payload only
// because it is the single registered config consumer; the dynamic CORS *behavior* is covered by the
// cors integration package.
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

// --- GET (list) ---

func (suite *ServerConfigAPITestSuite) TestGetReturnsEmptyCorsArrayWhenNoOrigins() {
	status, body := suite.getJSON(suite.adminClient, serverConfigURL)

	suite.Equal(http.StatusOK, status)
	suite.Contains(string(body), `"cors"`)
	var resp serverConfigResponse
	suite.Require().NoError(json.Unmarshal(body, &resp))
	suite.Empty(resp.CORS)
}

// --- GET by name ---

func (suite *ServerConfigAPITestSuite) TestGetByNameReturnsCorsSection() {
	suite.putCORS(fmt.Sprintf(`[%q]`, sampleOrigin))

	status, body := suite.getJSON(suite.adminClient, serverConfigURL+"/cors")

	suite.Equal(http.StatusOK, status)
	var resp serverConfigResponse
	suite.Require().NoError(json.Unmarshal(body, &resp))
	suite.Require().Len(resp.CORS, 1)
	suite.Equal(fmt.Sprintf("%q", sampleOrigin), string(resp.CORS[0]))
}

func (suite *ServerConfigAPITestSuite) TestGetByNameReturnsEmptyArrayWhenNoOrigins() {
	status, body := suite.getJSON(suite.adminClient, serverConfigURL+"/cors")

	suite.Equal(http.StatusOK, status)
	var resp serverConfigResponse
	suite.Require().NoError(json.Unmarshal(body, &resp))
	suite.Empty(resp.CORS)
}

func (suite *ServerConfigAPITestSuite) TestGetByNameUnsupportedNameReturns400() {
	status, body := suite.getJSON(suite.adminClient, serverConfigURL+"/bogus")

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("SCF-1001", suite.errorCode(body))
}

// --- PUT ---

func (suite *ServerConfigAPITestSuite) TestPutInvalidValueReturns400AndDoesNotPersist() {
	status, body := suite.putRaw(suite.adminClient, `{"cors":["*"]}`)
	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("SCF-1003", suite.errorCode(body))

	// Nothing was persisted: the section is still empty.
	getStatus, getBody := suite.getJSON(suite.adminClient, serverConfigURL)
	suite.Equal(http.StatusOK, getStatus)
	var resp serverConfigResponse
	suite.Require().NoError(json.Unmarshal(getBody, &resp))
	suite.Empty(resp.CORS)
}

func (suite *ServerConfigAPITestSuite) TestPutMalformedBodyReturns400() {
	status, body := suite.putRaw(suite.adminClient, `{"cors":[`)

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("SCF-1004", suite.errorCode(body))
}

// --- Auth gate ---

func (suite *ServerConfigAPITestSuite) TestUnauthenticatedRequestsAreRejected() {
	getStatus, getBody := suite.getJSON(suite.plainClient, serverConfigURL)
	suite.Equal(http.StatusUnauthorized, getStatus)
	suite.Equal("AUTH-4010", suite.errorCode(getBody))

	putStatus, _ := suite.putRaw(suite.plainClient, `{"cors":[]}`)
	suite.Equal(http.StatusUnauthorized, putStatus)
}

// --- helpers ---

// putCORS sets the cors section to the given raw JSON array and requires a 200 response.
func (suite *ServerConfigAPITestSuite) putCORS(corsArray string) {
	status, _ := suite.putRaw(suite.adminClient, fmt.Sprintf(`{"cors":%s}`, corsArray))
	suite.Require().Equal(http.StatusOK, status)
}

// putRaw sends a PUT /server-config with the given body and returns the status and response body.
func (suite *ServerConfigAPITestSuite) putRaw(client *http.Client, body string) (int, []byte) {
	req, err := http.NewRequest(http.MethodPut, serverConfigURL, strings.NewReader(body))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer closeBodyQuietly(suite.T(), resp.Body)

	respBody, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)
	return resp.StatusCode, respBody
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
