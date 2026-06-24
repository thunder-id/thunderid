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

package cors

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

// DynamicCORSTestSuite validates the dynamic CORS behavior driven by the server-config store: an
// origin set at runtime via PUT /server-config takes effect with no restart, the static deployment
// baseline is preserved (union), regex origins match, and replacing or clearing the section revokes
// access. CORS is asserted at the HTTP level via the preflight Access-Control-Allow-Origin header.
type DynamicCORSTestSuite struct {
	suite.Suite
	adminClient *http.Client
	plainClient *http.Client
}

func TestDynamicCORSTestSuite(t *testing.T) {
	suite.Run(t, new(DynamicCORSTestSuite))
}

func (suite *DynamicCORSTestSuite) SetupSuite() {
	suite.adminClient = testutils.GetHTTPClient()
	suite.plainClient = &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
}

// SetupTest clears the dynamic cors section so each test starts from the static-only baseline.
func (suite *DynamicCORSTestSuite) SetupTest() {
	suite.putCORS(`[]`)
}

// TearDownSuite leaves the dynamic cors section empty so the shared server is not left mutated.
func (suite *DynamicCORSTestSuite) TearDownSuite() {
	suite.putCORS(`[]`)
}

func (suite *DynamicCORSTestSuite) TestDynamicOriginAllowedAfterPutWithoutRestart() {
	// Before it is configured, the dynamic origin is not allowed.
	suite.Empty(suite.preflightACAO(dynamicOrigin))

	suite.putCORS(fmt.Sprintf(`[%q]`, dynamicOrigin))

	// It is allowed immediately, with no server restart.
	suite.Equal(dynamicOrigin, suite.preflightACAO(dynamicOrigin))
	// An unconfigured origin stays disallowed.
	suite.Empty(suite.preflightACAO(unknownOrigin))
}

func (suite *DynamicCORSTestSuite) TestCORSUnionPreservesStaticBaseline() {
	suite.putCORS(fmt.Sprintf(`[%q]`, dynamicOrigin))

	// The deployment.yaml (static) origin stays allowed alongside the dynamic one.
	suite.Equal(staticOrigin, suite.preflightACAO(staticOrigin))
	suite.Equal(dynamicOrigin, suite.preflightACAO(dynamicOrigin))
}

func (suite *DynamicCORSTestSuite) TestDynamicRegexOriginAllowedAfterPut() {
	suite.Empty(suite.preflightACAO(regexOrigin))

	// JSON-escaped so the stored regex is ^https://[a-z0-9-]+\.regex\.example$ (literal dots).
	suite.putCORS(`[{"regex":"^https://[a-z0-9-]+\\.regex\\.example$"}]`)

	suite.Equal(regexOrigin, suite.preflightACAO(regexOrigin))
	// A host that does not match the pattern stays disallowed.
	suite.Empty(suite.preflightACAO(unknownOrigin))
}

func (suite *DynamicCORSTestSuite) TestPutMultipleOriginsAllowsAll() {
	suite.putCORS(fmt.Sprintf(`[%q,%q]`, dynamicOrigin, secondDynamicOrigin))

	suite.Equal(dynamicOrigin, suite.preflightACAO(dynamicOrigin))
	suite.Equal(secondDynamicOrigin, suite.preflightACAO(secondDynamicOrigin))
}

func (suite *DynamicCORSTestSuite) TestPutReplacesPreviousOrigins() {
	suite.putCORS(fmt.Sprintf(`[%q]`, dynamicOrigin))
	suite.Equal(dynamicOrigin, suite.preflightACAO(dynamicOrigin))

	// Replacing the value de-registers the old origin and registers the new one, with no restart.
	suite.putCORS(fmt.Sprintf(`[%q]`, secondDynamicOrigin))
	suite.Equal(secondDynamicOrigin, suite.preflightACAO(secondDynamicOrigin))
	suite.Empty(suite.preflightACAO(dynamicOrigin))
}

func (suite *DynamicCORSTestSuite) TestClearingOriginsRevokesDynamicAccess() {
	suite.putCORS(fmt.Sprintf(`[%q]`, dynamicOrigin))
	suite.Equal(dynamicOrigin, suite.preflightACAO(dynamicOrigin))

	// Clearing the section revokes the dynamic origin; the static baseline remains.
	suite.putCORS(`[]`)
	suite.Empty(suite.preflightACAO(dynamicOrigin))
	suite.Equal(staticOrigin, suite.preflightACAO(staticOrigin))
}

// --- helpers ---

// putCORS sets the dynamic cors section to the given raw JSON array and requires a 200 response.
func (suite *DynamicCORSTestSuite) putCORS(corsArray string) {
	status, _ := suite.putRaw(suite.adminClient, fmt.Sprintf(`{"cors":%s}`, corsArray))
	suite.Require().Equal(http.StatusOK, status)
}

// putRaw sends a PUT /server-config with the given body and returns the status and response body.
func (suite *DynamicCORSTestSuite) putRaw(client *http.Client, body string) (int, []byte) {
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

// preflightACAO sends a CORS preflight for the origin and returns the echoed
// Access-Control-Allow-Origin header (empty when the origin is not allowed).
func (suite *DynamicCORSTestSuite) preflightACAO(origin string) string {
	req, err := http.NewRequest(http.MethodOptions, serverConfigURL, nil)
	suite.Require().NoError(err)
	req.Header.Set("Origin", origin)
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)

	resp, err := suite.plainClient.Do(req)
	suite.Require().NoError(err)
	defer closeBodyQuietly(suite.T(), resp.Body)

	suite.Equal(http.StatusNoContent, resp.StatusCode)
	return resp.Header.Get("Access-Control-Allow-Origin")
}

func closeBodyQuietly(t *testing.T, body io.ReadCloser) {
	if body != nil {
		if err := body.Close(); err != nil {
			t.Logf("Failed to close response body: %v", err)
		}
	}
}
