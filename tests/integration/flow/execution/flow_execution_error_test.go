// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// Error codes returned by the flow execution service. Asserting the code rather than the message
// keeps these tests independent of i18n wording.
const (
	errCodeInvalidAppID         = "FES-1003"
	errCodeInvalidExecutionID   = "FES-1004"
	errCodeInvalidFlowType      = "FES-1005"
	errCodeRegistrationDisabled = "FES-1006"
	errCodeRecoveryDisabled     = "FES-1009"
	errCodeAdminAuthRequired    = "FES-1017"
	errCodeFlowIDNotPermitted   = "FES-1018"
	// errCodeAdminPermissionNeeded (FES-1019) is not asserted yet: it needs a signed-in user whose
	// permissions omit the system scope. See TestExecuteByFlowID_ClientCredentialsTokenRejected.

	errorTestClientID     = "flow_execution_error_test_client"
	errorTestClientSecret = "flow_execution_error_test_secret" //nolint:gosec // test credential
)

var (
	errorTestOU = testutils.OrganizationUnit{
		Handle:      "flow_exec_error_test_ou",
		Name:        "Test OU for Flow Execution Errors",
		Description: "Organization unit created for flow execution error testing",
		Parent:      nil,
	}

	// A minimal authentication flow. These tests never complete it; they only need an application
	// bound to a real flow so that rejections come from the branch under test rather than from
	// missing configuration.
	errorTestFlow = testutils.Flow{
		Name:     "Flow Execution Error Test Auth Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_execution_error_test",
		Nodes: []map[string]interface{}{
			{
				"id":        "start",
				"type":      "START",
				"onSuccess": "prompt_credentials",
			},
			{
				"id":   "prompt_credentials",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{
								"ref":        "input_001",
								"identifier": "username",
								"type":       "TEXT_INPUT",
								"required":   true,
							},
						},
						"action": map[string]interface{}{
							"ref":      "action_001",
							"nextNode": "auth_assert",
						},
					},
				},
			},
			{
				// Flow validation requires an AuthAssertExecutor on every AUTHENTICATION flow, so this
				// node is here to make the definition valid rather than because the tests reach it.
				"id":   "auth_assert",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "AuthAssertExecutor",
				},
				"onSuccess": "end",
			},
			{
				"id":   "end",
				"type": "END",
			},
		},
	}

	// Registration and recovery are both left disabled so their "flow disabled" branches are
	// reachable from this one application.
	errorTestApp = testutils.Application{
		Name:                      "Flow Execution Error Test Application",
		Description:               "Application for testing flow execution error branches",
		IsRegistrationFlowEnabled: false,
		IsRecoveryFlowEnabled:     false,
		ClientID:                  errorTestClientID,
		ClientSecret:              errorTestClientSecret,
		RedirectURIs:              []string{"http://localhost:3000/callback"},
	}
)

type FlowExecutionErrorTestSuite struct {
	suite.Suite
	config *common.TestSuiteConfig
	ouID   string
	appID  string
	flowID string
}

func TestFlowExecutionErrorTestSuite(t *testing.T) {
	suite.Run(t, new(FlowExecutionErrorTestSuite))
}

func (ts *FlowExecutionErrorTestSuite) SetupSuite() {
	ts.config = &common.TestSuiteConfig{}

	ouID, err := testutils.CreateOrganizationUnit(errorTestOU)
	ts.Require().NoError(err, "Failed to create test organization unit")
	ts.ouID = ouID

	flowID, err := testutils.CreateFlow(errorTestFlow)
	ts.Require().NoError(err, "Failed to create test flow")
	ts.flowID = flowID
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)

	errorTestApp.OUID = ts.ouID
	errorTestApp.AuthFlowID = flowID
	appID, err := testutils.CreateApplication(errorTestApp)
	ts.Require().NoError(err, "Failed to create test application")
	ts.appID = appID
}

func (ts *FlowExecutionErrorTestSuite) TearDownSuite() {
	if ts.appID != "" {
		if err := testutils.DeleteApplication(ts.appID); err != nil {
			ts.T().Logf("Failed to delete test application during teardown: %v", err)
		}
	}
	for _, flowID := range ts.config.CreatedFlowIDs {
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("Failed to delete test flow during teardown: %v", err)
		}
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete test organization unit during teardown: %v", err)
		}
	}
}

// executeFlow posts an arbitrary body to /flow/execute and returns the status and decoded error.
//
// A raw client is used deliberately. The shared test clients wrap an auth transport that treats
// /flow/execute as a public endpoint and skips token injection entirely, so neither GetHTTPClient
// nor GetHTTPClientWithToken can authenticate against the administration entry point. Passing an
// empty token exercises the unauthenticated path; a non-empty one sets the header directly.
func (ts *FlowExecutionErrorTestSuite) executeFlow(
	body map[string]interface{}, bearerToken string) (int, common.ErrorResponse) {
	ts.T().Helper()

	reqBody, err := json.Marshal(body)
	ts.Require().NoError(err, "Failed to marshal flow request body")

	req, err := http.NewRequest(http.MethodPost, testServerURL+"/flow/execute", bytes.NewReader(reqBody))
	ts.Require().NoError(err, "Failed to build flow request")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
	}

	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err, "Failed to send flow request")
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err, "Failed to read flow response body")

	var errResp common.ErrorResponse
	if err := json.Unmarshal(bodyBytes, &errResp); err != nil {
		ts.T().Fatalf("Failed to decode error response (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	return resp.StatusCode, errResp
}

// assertFlowError runs the request and asserts both the status and the error code.
func (ts *FlowExecutionErrorTestSuite) assertFlowError(
	body map[string]interface{}, bearerToken string, wantStatus int, wantCode string) {
	ts.T().Helper()

	status, errResp := ts.executeFlow(body, bearerToken)
	ts.Equal(wantStatus, status, "Unexpected status for %s", wantCode)
	ts.Equal(wantCode, errResp.Code, "Unexpected error code")
}

// adminToken returns a bearer token carrying the system permission.
func (ts *FlowExecutionErrorTestSuite) adminToken() string {
	ts.T().Helper()

	token, err := testutils.GetAccessToken()
	ts.Require().NoError(err, "Failed to obtain admin access token")
	ts.Require().NotEmpty(token, "Admin access token is empty")

	return token
}

// nonAdminToken returns a client-credentials token for the test application: a valid token that
// carries no user subject and no system permission.
func (ts *FlowExecutionErrorTestSuite) nonAdminToken() string {
	ts.T().Helper()

	form := url.Values{}
	form.Set("grant_type", "client_credentials")

	req, err := http.NewRequest(http.MethodPost, testServerURL+"/oauth2/token", strings.NewReader(form.Encode()))
	ts.Require().NoError(err, "Failed to build token request")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(errorTestClientID, errorTestClientSecret)

	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err, "Failed to request client credentials token")
	defer resp.Body.Close()

	var body map[string]interface{}
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&body))
	ts.Require().Equal(http.StatusOK, resp.StatusCode, "Token request failed: %v", body)

	token, ok := body["access_token"].(string)
	ts.Require().True(ok, "Token response has no access_token: %v", body)

	return token
}

// A machine-to-machine client credentials token must not reach the administration entry point.
// /flow/execute is a public path, so this check is the only thing standing between any client
// holding a token and administration flow execution.
//
// The rejection is "authentication required" rather than "permission required": a client
// credentials token establishes no user subject, so the caller check refuses it before permissions
// are ever consulted. Reaching the permission branch (FES-1019) needs a user token whose
// permissions omit the system scope, which requires a signed-in non-administrator user and is not
// covered here.
func (ts *FlowExecutionErrorTestSuite) TestExecuteByFlowID_ClientCredentialsTokenRejected() {
	ts.assertFlowError(map[string]interface{}{
		"flowId": ts.flowID,
	}, ts.nonAdminToken(), http.StatusUnauthorized, errCodeAdminAuthRequired)
}

func (ts *FlowExecutionErrorTestSuite) TestInitiate_UnknownFlowTypeRejected() {
	ts.assertFlowError(map[string]interface{}{
		"applicationId": ts.appID,
		"flowType":      "NOT_A_REAL_FLOW_TYPE",
	}, "", http.StatusBadRequest, errCodeInvalidFlowType)
}

func (ts *FlowExecutionErrorTestSuite) TestInitiate_MissingFlowTypeRejected() {
	ts.assertFlowError(map[string]interface{}{
		"applicationId": ts.appID,
	}, "", http.StatusBadRequest, errCodeInvalidFlowType)
}

func (ts *FlowExecutionErrorTestSuite) TestInitiate_UnknownApplicationRejected() {
	ts.assertFlowError(map[string]interface{}{
		"applicationId": "01900000-0000-7000-8000-00000000dead",
		"flowType":      "AUTHENTICATION",
	}, "", http.StatusBadRequest, errCodeInvalidAppID)
}

// Registration is disabled on the test application, so the flow type is valid but unavailable.
func (ts *FlowExecutionErrorTestSuite) TestInitiate_RegistrationDisabledRejected() {
	ts.assertFlowError(map[string]interface{}{
		"applicationId": ts.appID,
		"flowType":      "REGISTRATION",
	}, "", http.StatusBadRequest, errCodeRegistrationDisabled)
}

func (ts *FlowExecutionErrorTestSuite) TestInitiate_RecoveryDisabledRejected() {
	ts.assertFlowError(map[string]interface{}{
		"applicationId": ts.appID,
		"flowType":      "RECOVERY",
	}, "", http.StatusBadRequest, errCodeRecoveryDisabled)
}

func (ts *FlowExecutionErrorTestSuite) TestResume_UnknownExecutionIDRejected() {
	ts.assertFlowError(map[string]interface{}{
		"executionId": "01900000-0000-7000-8000-00000000beef",
		"inputs":      map[string]string{"username": "someone"},
	}, "", http.StatusBadRequest, errCodeInvalidExecutionID)
}

func (ts *FlowExecutionErrorTestSuite) TestResume_MalformedExecutionIDRejected() {
	ts.assertFlowError(map[string]interface{}{
		"executionId": "not-a-uuid",
		"inputs":      map[string]string{"username": "someone"},
	}, "", http.StatusBadRequest, errCodeInvalidExecutionID)
}

// Initiating by flowId is the administration entry point. /flow/execute is a public path, so an
// unauthenticated caller must be rejected before the flow is even resolved, which also stops the
// response distinguishing a missing flow from an existing one.
func (ts *FlowExecutionErrorTestSuite) TestExecuteByFlowID_UnauthenticatedRejected() {
	ts.assertFlowError(map[string]interface{}{
		"flowId": ts.flowID,
	}, "", http.StatusUnauthorized, errCodeAdminAuthRequired)
}

func (ts *FlowExecutionErrorTestSuite) TestExecuteByFlowID_UnauthenticatedUnknownFlowRejectedIdentically() {
	// A flow id that does not exist must produce the same rejection as a real one, so the endpoint
	// does not leak which flows exist to an unauthenticated caller.
	ts.assertFlowError(map[string]interface{}{
		"flowId": "01900000-0000-7000-8000-00000000f00d",
	}, "", http.StatusUnauthorized, errCodeAdminAuthRequired)
}

// An authenticated administrator may only execute ADMINISTRATION flows by id. Pointing the entry
// point at an authentication flow must be refused, otherwise it becomes a way to start a login
// flow directly and bypass the application binding that type requires.
func (ts *FlowExecutionErrorTestSuite) TestExecuteByFlowID_NonAdministrationFlowRejected() {
	ts.assertFlowError(map[string]interface{}{
		"flowId": ts.flowID,
	}, ts.adminToken(), http.StatusForbidden, errCodeFlowIDNotPermitted)
}

func (ts *FlowExecutionErrorTestSuite) TestExecuteByFlowID_UnknownFlowRejectedForAdministrator() {
	status, errResp := ts.executeFlow(map[string]interface{}{
		"flowId": "01900000-0000-7000-8000-00000000f00d",
	}, ts.adminToken())

	// The caller is authorized, so the response may report either that the flow does not exist or
	// that execution by id is not permitted for it. Both are client errors; what matters is that an
	// authorized caller is not served a 401 and never reaches execution.
	ts.Contains([]int{http.StatusBadRequest, http.StatusForbidden, http.StatusNotFound}, status,
		fmt.Sprintf("Unexpected status for an unknown flow id: %d", status))
	ts.NotEmpty(errResp.Code, "Expected an error code for an unknown flow id")
}
