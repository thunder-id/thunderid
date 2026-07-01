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

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	testServerURL = testutils.TestServerURL
	corsConfigURL = testServerURL + "/server-config/cors"

	// declarativeOrigin is the read-only declarative cors.yaml fixture; it is the dynamic matcher's
	// read-only layer and survives clearing the writable layer.
	declarativeOrigin = "https://declarative.example.com"
	// runtimeOrigin / replacementOrigin exercise runtime PUTs (the dynamic matcher's writable layer).
	runtimeOrigin     = "https://runtime.example.com"
	replacementOrigin = "https://replacement.example.com"
	// disallowedOrigin is never configured in any layer.
	disallowedOrigin = "https://evil.example.com"
)

// CORSIntegrationTestSuite validates live CORS behavior from the server-config cors section. The generic
// server-config API is covered by the serverconfig integration package; this suite asserts Origin matching.
type CORSIntegrationTestSuite struct {
	suite.Suite
	adminClient *http.Client
	plainClient *http.Client
}

func TestCORSIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(CORSIntegrationTestSuite))
}

func (suite *CORSIntegrationTestSuite) SetupSuite() {
	suite.adminClient = testutils.GetHTTPClient()
	suite.plainClient = &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
}

// SetupTest clears the cors writable layer before each test.
func (suite *CORSIntegrationTestSuite) SetupTest() {
	suite.putCORS(`[]`)
}

// TearDownSuite leaves the cors writable layer empty so the shared server is not left mutated.
func (suite *CORSIntegrationTestSuite) TearDownSuite() {
	suite.putCORS(`[]`)
}

// --- declarative (read-only) baseline ---

func (suite *CORSIntegrationTestSuite) TestDeclarativeOriginAllowed() {
	suite.Equal(declarativeOrigin, suite.allowedOrigin(declarativeOrigin))
}

func (suite *CORSIntegrationTestSuite) TestDisallowedOriginRejected() {
	suite.Empty(suite.allowedOrigin(disallowedOrigin))
}

// --- runtime (writable) layer, no restart ---

func (suite *CORSIntegrationTestSuite) TestRuntimeOriginAllowedWithoutRestart() {
	suite.Empty(suite.allowedOrigin(runtimeOrigin))

	suite.putCORS(fmt.Sprintf(`[%q]`, runtimeOrigin))

	suite.Equal(runtimeOrigin, suite.allowedOrigin(runtimeOrigin))
}

func (suite *CORSIntegrationTestSuite) TestDeclarativeAndRuntimeUnioned() {
	suite.putCORS(fmt.Sprintf(`[%q]`, runtimeOrigin))

	// The declarative read-only origin and the runtime writable origin are both allowed at once.
	suite.Equal(declarativeOrigin, suite.allowedOrigin(declarativeOrigin))
	suite.Equal(runtimeOrigin, suite.allowedOrigin(runtimeOrigin))
}

func (suite *CORSIntegrationTestSuite) TestReplaceDeRegistersOldOrigin() {
	suite.putCORS(fmt.Sprintf(`[%q]`, runtimeOrigin))
	suite.Require().Equal(runtimeOrigin, suite.allowedOrigin(runtimeOrigin))

	suite.putCORS(fmt.Sprintf(`[%q]`, replacementOrigin)) // replaces, does not append

	suite.Empty(suite.allowedOrigin(runtimeOrigin))
	suite.Equal(replacementOrigin, suite.allowedOrigin(replacementOrigin))
	// The read-only declarative origin is unaffected by writable replacement.
	suite.Equal(declarativeOrigin, suite.allowedOrigin(declarativeOrigin))
}

func (suite *CORSIntegrationTestSuite) TestClearRevokesRuntimeOrigin() {
	suite.putCORS(fmt.Sprintf(`[%q]`, runtimeOrigin))
	suite.Require().Equal(runtimeOrigin, suite.allowedOrigin(runtimeOrigin))

	suite.putCORS(`[]`) // clear the writable layer

	suite.Empty(suite.allowedOrigin(runtimeOrigin))
	// Clearing the writable layer leaves the read-only declarative origin enforced.
	suite.Equal(declarativeOrigin, suite.allowedOrigin(declarativeOrigin))
}

func (suite *CORSIntegrationTestSuite) TestRuntimeRegexOriginAllowed() {
	suite.putCORS(`[{"regex":"^https://[a-z0-9-]+\\.tenant\\.example\\.com$"}]`)

	suite.Equal("https://acme.tenant.example.com",
		suite.allowedOrigin("https://acme.tenant.example.com"))
	suite.Empty(suite.allowedOrigin("https://acme.other.example.com"))
}

// --- helpers ---

// allowedOrigin sends a CORS preflight (OPTIONS + Access-Control-Request-Method) for the given Origin to a
// CORS-enabled, auth-free route and returns the echoed Access-Control-Allow-Origin (empty when rejected).
func (suite *CORSIntegrationTestSuite) allowedOrigin(origin string) string {
	req, err := http.NewRequest(http.MethodOptions, corsConfigURL, nil)
	suite.Require().NoError(err)
	req.Header.Set("Origin", origin)
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)

	resp, err := suite.plainClient.Do(req)
	suite.Require().NoError(err)
	defer closeBodyQuietly(suite.T(), resp.Body)

	return resp.Header.Get("Access-Control-Allow-Origin")
}

// putCORS sets the cors writable layer to the given allowed-origins array (object-shaped body) using the
// admin client and requires a 200 response.
func (suite *CORSIntegrationTestSuite) putCORS(allowedOrigins string) {
	body := `{"allowedOrigins":` + allowedOrigins + `}`
	req, err := http.NewRequest(http.MethodPut, corsConfigURL, strings.NewReader(body))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.adminClient.Do(req)
	suite.Require().NoError(err)
	defer closeBodyQuietly(suite.T(), resp.Body)

	suite.Require().Equal(http.StatusOK, resp.StatusCode)
}

func closeBodyQuietly(t *testing.T, body io.ReadCloser) {
	if body != nil {
		if err := body.Close(); err != nil {
			t.Logf("Failed to close response body: %v", err)
		}
	}
}
