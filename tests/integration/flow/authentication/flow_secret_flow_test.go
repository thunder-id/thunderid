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

package authentication

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// This suite verifies the direct flow-initiation guard for POST /flow/execute:
//   - flow-native apps (non-public, non-redirect, token-exchange capable) must present a valid Flow Secret;
//   - redirect (authorization_code) and machine-to-machine (client_credentials only) apps are blocked;
//   - flow continuation (executionId) is unaffected by the guard.

var flowSecretGuardOU = testutils.OrganizationUnit{
	Handle:      "flow_secret_guard_test_ou",
	Name:        "Flow Secret Guard Test OU",
	Description: "Organization unit for Flow Secret guard flow testing",
	Parent:      nil,
}

var flowSecretGuardUserType = testutils.UserType{
	Name: "flow_secret_guard_person",
	Schema: map[string]interface{}{
		"username": map[string]interface{}{"type": "string"},
		"password": map[string]interface{}{"type": "string", "credential": true},
		"email":    map[string]interface{}{"type": "string"},
	},
}

var flowSecretGuardUser = testutils.User{
	Type: flowSecretGuardUserType.Name,
	Attributes: json.RawMessage(`{
		"username": "appsecretguarduser",
		"password": "testpassword",
		"email": "appsecretguarduser@example.com"
	}`),
}

var flowSecretGuardFlow = testutils.Flow{
	Name:     "Flow Secret Guard Auth Flow",
	FlowType: "AUTHENTICATION",
	Handle:   "auth_flow_flow_secret_guard",
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
						{"ref": "input_001", "identifier": "username", "type": "TEXT_INPUT", "required": true},
						{"ref": "input_002", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
					},
					"action": map[string]interface{}{"ref": "action_001", "nextNode": "credentials_auth"},
				},
			},
		},
		{
			"id":   "credentials_auth",
			"type": "TASK_EXECUTION",
			"executor": map[string]interface{}{
				"name": "CredentialsAuthExecutor",
				"inputs": []map[string]interface{}{
					{"ref": "input_001", "identifier": "username", "type": "TEXT_INPUT", "required": true},
					{"ref": "input_002", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
				},
			},
			"onSuccess":    "auth_assert",
			"onIncomplete": "prompt_credentials",
		},
		{
			"id":        "auth_assert",
			"type":      "TASK_EXECUTION",
			"executor":  map[string]interface{}{"name": "AuthAssertExecutor"},
			"onSuccess": "end",
		},
		{
			"id":   "end",
			"type": "END",
		},
	},
}

// flowSecretBackendApp is a flow-native confidential application: non-public, non-redirect, with the
// token-exchange grant so it can consume a flow assertion. It is issued a Flow Secret and must
// present it to initiate a flow directly.
var flowSecretBackendApp = testutils.Application{
	Name:                      "Flow Secret Backend App",
	Description:               "Backend application for Flow Secret guard testing",
	IsRegistrationFlowEnabled: false,
	ClientID:                  "flow_secret_backend_client",
	ClientSecret:              "flow_secret_backend_secret",
	AllowedUserTypes:          []string{"flow_secret_guard_person"},
	InboundAuthConfig: []map[string]interface{}{
		{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId":                "flow_secret_backend_client",
				"clientSecret":            "flow_secret_backend_secret",
				"grantTypes":              []string{"client_credentials", "urn:ietf:params:oauth:grant-type:token-exchange"},
				"tokenEndpointAuthMethod": "client_secret_basic",
			},
		},
	},
}

// flowSecretM2MApp is a machine-to-machine application (client_credentials only). It obtains tokens
// directly and cannot consume a flow assertion, so it is not issued a Flow Secret and is blocked
// from initiating a flow directly.
var flowSecretM2MApp = testutils.Application{
	Name:                      "Flow Secret M2M App",
	Description:               "Machine-to-machine application for Flow Secret guard testing",
	IsRegistrationFlowEnabled: false,
	ClientID:                  "flow_secret_m2m_client",
	ClientSecret:              "flow_secret_m2m_secret",
	AllowedUserTypes:          []string{"flow_secret_guard_person"},
	InboundAuthConfig: []map[string]interface{}{
		{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId":                "flow_secret_m2m_client",
				"clientSecret":            "flow_secret_m2m_secret",
				"grantTypes":              []string{"client_credentials"},
				"tokenEndpointAuthMethod": "client_secret_basic",
			},
		},
	},
}

// flowSecretRedirectApp is a confidential application using the redirect-based authorization_code
// grant. It is blocked from initiating a flow directly and is not issued an Flow Secret.
var flowSecretRedirectApp = testutils.Application{
	Name:                      "Flow Secret Redirect App",
	Description:               "Redirect-based application for Flow Secret guard testing",
	IsRegistrationFlowEnabled: false,
	ClientID:                  "flow_secret_redirect_client",
	ClientSecret:              "flow_secret_redirect_secret",
	RedirectURIs:              []string{"https://localhost:3000/app-secret-callback"},
	AllowedUserTypes:          []string{"flow_secret_guard_person"},
	InboundAuthConfig: []map[string]interface{}{
		{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId":                "flow_secret_redirect_client",
				"clientSecret":            "flow_secret_redirect_secret",
				"redirectUris":            []string{"https://localhost:3000/app-secret-callback"},
				"grantTypes":              []string{"authorization_code"},
				"responseTypes":           []string{"code"},
				"tokenEndpointAuthMethod": "client_secret_basic",
			},
		},
	},
}

var (
	flowSecretGuardOUID       string
	flowSecretGuardUserTypeID string
	flowSecretGuardFlowID     string
	flowSecretBackendAppID    string
	flowSecretM2MAppID        string
	flowSecretRedirectAppID   string
)

// FlowSecretFlowTestSuite verifies the Flow Secret guard on direct flow initiation.
type FlowSecretFlowTestSuite struct {
	suite.Suite
	config *common.TestSuiteConfig
}

func TestFlowSecretFlowTestSuite(t *testing.T) {
	suite.Run(t, new(FlowSecretFlowTestSuite))
}

func (ts *FlowSecretFlowTestSuite) SetupSuite() {
	ts.config = &common.TestSuiteConfig{}

	ouID, err := testutils.CreateOrganizationUnit(flowSecretGuardOU)
	ts.Require().NoError(err, "failed to create OU")
	flowSecretGuardOUID = ouID

	flowSecretGuardUserType.OUID = flowSecretGuardOUID
	schemaID, err := testutils.CreateUserType(flowSecretGuardUserType)
	ts.Require().NoError(err, "failed to create user type")
	flowSecretGuardUserTypeID = schemaID

	user := flowSecretGuardUser
	user.OUID = flowSecretGuardOUID
	userIDs, err := testutils.CreateMultipleUsers(user)
	ts.Require().NoError(err, "failed to create test user")
	ts.config.CreatedUserIDs = userIDs

	flowID, err := testutils.CreateFlow(flowSecretGuardFlow)
	ts.Require().NoError(err, "failed to create auth flow")
	flowSecretGuardFlowID = flowID
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)

	flowSecretBackendApp.AuthFlowID = flowID
	flowSecretBackendApp.OUID = flowSecretGuardOUID
	backendAppID, err := testutils.CreateApplication(flowSecretBackendApp)
	ts.Require().NoError(err, "failed to create backend application")
	flowSecretBackendAppID = backendAppID

	flowSecretM2MApp.AuthFlowID = flowID
	flowSecretM2MApp.OUID = flowSecretGuardOUID
	m2mAppID, err := testutils.CreateApplication(flowSecretM2MApp)
	ts.Require().NoError(err, "failed to create M2M application")
	flowSecretM2MAppID = m2mAppID

	flowSecretRedirectApp.AuthFlowID = flowID
	flowSecretRedirectApp.OUID = flowSecretGuardOUID
	redirectAppID, err := testutils.CreateApplication(flowSecretRedirectApp)
	ts.Require().NoError(err, "failed to create redirect application")
	flowSecretRedirectAppID = redirectAppID

	// The flow-native backend app is eligible for a Flow Secret, so creation must have issued one.
	ts.Require().NotEmpty(testutils.GetFlowSecret(flowSecretBackendAppID),
		"backend app should be issued a Flow Secret at creation")
	// M2M and redirect apps must never be issued a Flow Secret.
	ts.Require().Empty(testutils.GetFlowSecret(flowSecretM2MAppID),
		"M2M app should not be issued a Flow Secret")
	ts.Require().Empty(testutils.GetFlowSecret(flowSecretRedirectAppID),
		"redirect app should not be issued a Flow Secret")
}

func (ts *FlowSecretFlowTestSuite) TearDownSuite() {
	if err := testutils.CleanupUsers(ts.config.CreatedUserIDs); err != nil {
		ts.T().Logf("failed to cleanup users: %v", err)
	}
	if flowSecretBackendAppID != "" {
		if err := testutils.DeleteApplication(flowSecretBackendAppID); err != nil {
			ts.T().Logf("failed to delete backend application: %v", err)
		}
	}
	if flowSecretM2MAppID != "" {
		if err := testutils.DeleteApplication(flowSecretM2MAppID); err != nil {
			ts.T().Logf("failed to delete M2M application: %v", err)
		}
	}
	if flowSecretRedirectAppID != "" {
		if err := testutils.DeleteApplication(flowSecretRedirectAppID); err != nil {
			ts.T().Logf("failed to delete redirect application: %v", err)
		}
	}
	for _, id := range ts.config.CreatedFlowIDs {
		if err := testutils.DeleteFlow(id); err != nil {
			ts.T().Logf("failed to delete flow %s: %v", id, err)
		}
	}
	if flowSecretGuardUserTypeID != "" {
		if err := testutils.DeleteUserType(flowSecretGuardUserTypeID); err != nil {
			ts.T().Logf("failed to delete user type: %v", err)
		}
	}
	if flowSecretGuardOUID != "" {
		if err := testutils.DeleteOrganizationUnit(flowSecretGuardOUID); err != nil {
			ts.T().Logf("failed to delete OU: %v", err)
		}
	}
}

// executeNewFlow posts a new-flow INIT request and returns the HTTP status code and parsed error
// body. It does not inject a registered Flow Secret, so flowSecret controls exactly what is sent
// in the Flow-Secret header (omitted entirely when empty).
func (ts *FlowSecretFlowTestSuite) executeNewFlow(body map[string]interface{}, flowSecret string) (
	int, *common.ErrorResponse) {
	reqBody, err := json.Marshal(body)
	ts.Require().NoError(err, "failed to marshal flow request")

	req, err := http.NewRequest("POST", testutils.TestServerURL+"/flow/execute", bytes.NewReader(reqBody))
	ts.Require().NoError(err, "failed to create flow request")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if flowSecret != "" {
		req.Header.Set(testutils.FlowSecretHeaderName, flowSecret)
	}

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err, "failed to send flow request")
	defer resp.Body.Close()

	var errResp common.ErrorResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&errResp),
		"flow error response should be valid JSON")
	return resp.StatusCode, &errResp
}

// A backend app that presents its valid Flow Secret may initiate a flow.
func (ts *FlowSecretFlowTestSuite) TestBackendApp_ValidFlowSecret_Allowed() {
	// InitiateAuthenticationFlow injects the Flow Secret registered for the app at creation.
	flowStep, err := common.InitiateAuthenticationFlow(flowSecretBackendAppID, false, nil, "")
	ts.Require().NoError(err, "flow initiation with a valid Flow Secret should succeed")
	ts.Require().NotEmpty(flowStep.ExecutionID, "a new flow should return an execution ID")
}

// A backend app that omits the Flow Secret is rejected with 401 (FES-1011).
func (ts *FlowSecretFlowTestSuite) TestBackendApp_MissingFlowSecret_Rejected() {
	status, errResp := ts.executeNewFlow(map[string]interface{}{
		"applicationId": flowSecretBackendAppID,
		"flowType":      "AUTHENTICATION",
	}, "")

	ts.Require().Equal(http.StatusUnauthorized, status)
	ts.Require().Equal("FES-1011", errResp.Code)
}

// A backend app that presents an incorrect Flow Secret is rejected with 401 (FES-1012).
func (ts *FlowSecretFlowTestSuite) TestBackendApp_InvalidFlowSecret_Rejected() {
	status, errResp := ts.executeNewFlow(map[string]interface{}{
		"applicationId": flowSecretBackendAppID,
		"flowType":      "AUTHENTICATION",
	}, "wrong-secret")

	ts.Require().Equal(http.StatusUnauthorized, status)
	ts.Require().Equal("FES-1012", errResp.Code)
}

// A machine-to-machine (client_credentials only) app cannot initiate a flow directly; it is blocked
// with 403 (FES-1010) and holds no Flow Secret.
func (ts *FlowSecretFlowTestSuite) TestM2MApp_DirectInitiation_Forbidden() {
	status, errResp := ts.executeNewFlow(map[string]interface{}{
		"applicationId": flowSecretM2MAppID,
		"flowType":      "AUTHENTICATION",
	}, "")

	ts.Require().Equal(http.StatusForbidden, status)
	ts.Require().Equal("FES-1010", errResp.Code)
}

// A redirect-based (authorization_code) app cannot initiate a flow directly; it is blocked with
// 403 (FES-1010) regardless of any Flow Secret value supplied.
func (ts *FlowSecretFlowTestSuite) TestRedirectApp_DirectInitiation_Forbidden() {
	status, errResp := ts.executeNewFlow(map[string]interface{}{
		"applicationId": flowSecretRedirectAppID,
		"flowType":      "AUTHENTICATION",
	}, "")

	ts.Require().Equal(http.StatusForbidden, status)
	ts.Require().Equal("FES-1010", errResp.Code)
}

// Flow continuation (carrying an executionId) is not subject to the guard: once a backend app has
// initiated a flow with its Flow Secret, subsequent steps complete without resupplying it.
func (ts *FlowSecretFlowTestSuite) TestBackendApp_Continuation_NoFlowSecretRequired() {
	flowStep, err := common.InitiateAuthenticationFlow(flowSecretBackendAppID, false, nil, "")
	ts.Require().NoError(err, "flow initiation should succeed")
	ts.Require().NotEmpty(flowStep.ExecutionID)

	completed, err := common.CompleteFlow(
		flowStep.ExecutionID,
		map[string]string{"username": "appsecretguarduser", "password": "testpassword"},
		"action_001",
		flowStep.ChallengeToken,
	)
	ts.Require().NoError(err, "flow continuation should not require an Flow Secret")
	ts.Require().Equal("COMPLETE", completed.FlowStatus)
	ts.Require().NotEmpty(completed.Assertion, "a completed authentication flow returns an assertion")
}
