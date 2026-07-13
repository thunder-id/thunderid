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

package authn

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// DirectAuthSecretTestSuite verifies the Direct Auth Secret gate on the Direct API endpoints.
// The integration server runs with server.security.direct_auth_secret configured (see
// resources/deployment.yaml), so these endpoints require the Direct-Auth-Secret header. This suite
// uses a raw HTTP client (no automatic header injection) to control the header per request.
type DirectAuthSecretTestSuite struct {
	suite.Suite
	client     *http.Client
	ouID       string
	userTypeID string
	userID     string
}

func TestDirectAuthSecretTestSuite(t *testing.T) {
	suite.Run(t, new(DirectAuthSecretTestSuite))
}

func (ts *DirectAuthSecretTestSuite) SetupSuite() {
	ts.client = testutils.GetRawHTTPClient()

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "direct-auth-secret-test-ou",
		Name:        "Direct Auth Secret Test Organization Unit",
		Description: "Organization unit for Direct Auth secret testing",
		Parent:      nil,
	})
	ts.Require().NoError(err, "failed to create test organization unit")
	ts.ouID = ouID

	userType := testutils.UserType{
		Name: "direct_auth_secret_person",
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
			"password": map[string]interface{}{"type": "string", "credential": true},
		},
		OUID: ts.ouID,
	}
	userTypeID, err := testutils.CreateUserType(userType)
	ts.Require().NoError(err, "failed to create test user type")
	ts.userTypeID = userTypeID

	userID, err := testutils.CreateUser(testutils.User{
		Type:       userType.Name,
		OUID:       ts.ouID,
		Attributes: json.RawMessage(`{"username": "directauthsecretuser", "password": "TestPassword123!"}`),
	})
	ts.Require().NoError(err, "failed to create test user")
	ts.userID = userID
}

func (ts *DirectAuthSecretTestSuite) TearDownSuite() {
	if ts.userID != "" {
		if err := testutils.DeleteUser(ts.userID); err != nil {
			ts.T().Logf("teardown: failed to delete user: %v", err)
		}
	}
	if ts.userTypeID != "" {
		if err := testutils.DeleteUserType(ts.userTypeID); err != nil {
			ts.T().Logf("teardown: failed to delete user type: %v", err)
		}
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("teardown: failed to delete organization unit: %v", err)
		}
	}
}

// TestMissingSecretRejected verifies an Direct API request without the header is rejected.
func (ts *DirectAuthSecretTestSuite) TestMissingSecretRejected() {
	errResp, status := ts.sendCredentialsAuth("")
	ts.Equal(http.StatusUnauthorized, status, "expected 401 when the Direct Auth secret header is missing")
	ts.Equal("AUTH-4010", errResp.Code)
}

// TestWrongSecretRejected verifies an Direct API request with an incorrect header value is rejected.
func (ts *DirectAuthSecretTestSuite) TestWrongSecretRejected() {
	errResp, status := ts.sendCredentialsAuth("wrong-secret")
	ts.Equal(http.StatusUnauthorized, status, "expected 401 when the Direct Auth secret header is incorrect")
	ts.Equal("AUTH-4010", errResp.Code)
}

// TestValidSecretAdmitted verifies an Direct API request with the correct header authenticates.
func (ts *DirectAuthSecretTestSuite) TestValidSecretAdmitted() {
	authRequest := map[string]interface{}{
		"identifiers": map[string]interface{}{"username": "directauthsecretuser"},
		"credentials": map[string]interface{}{"password": "TestPassword123!"},
	}
	requestJSON, err := json.Marshal(authRequest)
	ts.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+credentialsAuthEndpoint,
		bytes.NewReader(requestJSON))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(testutils.DirectAuthHeaderName, testutils.DirectAuthHeaderValue)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	ts.Require().Equal(http.StatusOK, resp.StatusCode, "expected 200 with a valid Direct Auth secret; body: %s", bodyBytes)

	var response testutils.AuthenticationResponse
	ts.Require().NoError(json.Unmarshal(bodyBytes, &response))
	ts.Equal(ts.userID, response.ID, "authenticated user ID should match")
}

// TestRegisterPasskeyPathGated verifies the second gated prefix (/register/passkey/**) is enforced,
// independently of the /auth/** prefix. It asserts the gate itself, not the passkey handler, so it
// does not run a full WebAuthn ceremony.
func (ts *DirectAuthSecretTestSuite) TestRegisterPasskeyPathGated() {
	const registerPasskeyEndpoint = "/register/passkey/start"

	// Missing header → rejected by the gate before the handler runs.
	errResp, status := ts.postJSON(registerPasskeyEndpoint, "{}", "")
	ts.Equal(http.StatusUnauthorized, status, "expected 401 when the Direct Auth secret header is missing")
	ts.Equal("AUTH-4010", errResp.Code)

	// Valid header → passes the gate (any non-gate response is fine here; it must not be AUTH-4010).
	errResp, status = ts.postJSON(registerPasskeyEndpoint, "{}", testutils.DirectAuthHeaderValue)
	ts.NotEqual("AUTH-4010", errResp.Code,
		"a valid Direct Auth secret must pass the gate; got %d / %s", status, errResp.Code)
}

// TestNonDirectAuthPublicPathNotGated verifies the secret does not affect non-direct public endpoints.
func (ts *DirectAuthSecretTestSuite) TestNonDirectAuthPublicPathNotGated() {
	req, err := http.NewRequest(http.MethodGet, testutils.TestServerURL+"/health/liveness", nil)
	ts.Require().NoError(err)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.NotEqual(http.StatusUnauthorized, resp.StatusCode,
		"non-Direct-API public paths must not require the Direct Auth secret")
}

// sendCredentialsAuth posts a fixed credentials request with the given secret header value (a value
// of "" omits the header) and returns the decoded error response and status code.
func (ts *DirectAuthSecretTestSuite) sendCredentialsAuth(secret string) (*testutils.ErrorResponse, int) {
	body := `{"identifiers":{"username":"directauthsecretuser"},"credentials":{"password":"TestPassword123!"}}`
	return ts.postJSON(credentialsAuthEndpoint, body, secret)
}

// postJSON posts a raw JSON body to the given endpoint with the given secret header value (a value
// of "" omits the header) and returns the decoded error response and status code.
func (ts *DirectAuthSecretTestSuite) postJSON(endpoint, body, secret string) (*testutils.ErrorResponse, int) {
	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+endpoint, bytes.NewReader([]byte(body)))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	if secret != "" {
		req.Header.Set(testutils.DirectAuthHeaderName, secret)
	}

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	var errResp testutils.ErrorResponse
	bodyBytes, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(bodyBytes, &errResp)
	return &errResp, resp.StatusCode
}
