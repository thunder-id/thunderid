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
//   - backend (non-public, non-redirect) apps must present a valid App Secret;
//   - apps using the redirect-based authorization_code grant are blocked outright;
//   - flow continuation (executionId) is unaffected by the guard.

var appSecretGuardOU = testutils.OrganizationUnit{
	Handle:      "app_secret_guard_test_ou",
	Name:        "App Secret Guard Test OU",
	Description: "Organization unit for App Secret guard flow testing",
	Parent:      nil,
}

var appSecretGuardUserType = testutils.UserType{
	Name: "app_secret_guard_person",
	Schema: map[string]interface{}{
		"username": map[string]interface{}{"type": "string"},
		"password": map[string]interface{}{"type": "string", "credential": true},
		"email":    map[string]interface{}{"type": "string"},
	},
}

var appSecretGuardUser = testutils.User{
	Type: appSecretGuardUserType.Name,
	Attributes: json.RawMessage(`{
		"username": "appsecretguarduser",
		"password": "testpassword",
		"email": "appsecretguarduser@example.com"
	}`),
}

var appSecretGuardFlow = testutils.Flow{
	Name:     "App Secret Guard Auth Flow",
	FlowType: "AUTHENTICATION",
	Handle:   "auth_flow_app_secret_guard",
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

// appSecretBackendApp is a backend (client_credentials, confidential) application. It is not a
// public client and does not use authorization_code, so it is issued an App Secret and must
// present it to initiate a flow directly.
var appSecretBackendApp = testutils.Application{
	Name:                      "App Secret Backend App",
	Description:               "Backend application for App Secret guard testing",
	IsRegistrationFlowEnabled: false,
	ClientID:                  "app_secret_backend_client",
	ClientSecret:              "app_secret_backend_secret",
	AllowedUserTypes:          []string{"app_secret_guard_person"},
	InboundAuthConfig: []map[string]interface{}{
		{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId":                "app_secret_backend_client",
				"clientSecret":            "app_secret_backend_secret",
				"grantTypes":              []string{"client_credentials"},
				"tokenEndpointAuthMethod": "client_secret_basic",
			},
		},
	},
}

// appSecretRedirectApp is a confidential application using the redirect-based authorization_code
// grant. It is blocked from initiating a flow directly and is not issued an App Secret.
var appSecretRedirectApp = testutils.Application{
	Name:                      "App Secret Redirect App",
	Description:               "Redirect-based application for App Secret guard testing",
	IsRegistrationFlowEnabled: false,
	ClientID:                  "app_secret_redirect_client",
	ClientSecret:              "app_secret_redirect_secret",
	RedirectURIs:              []string{"https://localhost:3000/app-secret-callback"},
	AllowedUserTypes:          []string{"app_secret_guard_person"},
	InboundAuthConfig: []map[string]interface{}{
		{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId":                "app_secret_redirect_client",
				"clientSecret":            "app_secret_redirect_secret",
				"redirectUris":            []string{"https://localhost:3000/app-secret-callback"},
				"grantTypes":              []string{"authorization_code"},
				"responseTypes":           []string{"code"},
				"tokenEndpointAuthMethod": "client_secret_basic",
			},
		},
	},
}

var (
	appSecretGuardOUID       string
	appSecretGuardUserTypeID string
	appSecretGuardFlowID     string
	appSecretBackendAppID    string
	appSecretRedirectAppID   string
)

// AppSecretFlowTestSuite verifies the App Secret guard on direct flow initiation.
type AppSecretFlowTestSuite struct {
	suite.Suite
	config *common.TestSuiteConfig
}

func TestAppSecretFlowTestSuite(t *testing.T) {
	suite.Run(t, new(AppSecretFlowTestSuite))
}

func (ts *AppSecretFlowTestSuite) SetupSuite() {
	ts.config = &common.TestSuiteConfig{}

	ouID, err := testutils.CreateOrganizationUnit(appSecretGuardOU)
	ts.Require().NoError(err, "failed to create OU")
	appSecretGuardOUID = ouID

	appSecretGuardUserType.OUID = appSecretGuardOUID
	schemaID, err := testutils.CreateUserType(appSecretGuardUserType)
	ts.Require().NoError(err, "failed to create user type")
	appSecretGuardUserTypeID = schemaID

	user := appSecretGuardUser
	user.OUID = appSecretGuardOUID
	userIDs, err := testutils.CreateMultipleUsers(user)
	ts.Require().NoError(err, "failed to create test user")
	ts.config.CreatedUserIDs = userIDs

	flowID, err := testutils.CreateFlow(appSecretGuardFlow)
	ts.Require().NoError(err, "failed to create auth flow")
	appSecretGuardFlowID = flowID
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)

	appSecretBackendApp.AuthFlowID = flowID
	appSecretBackendApp.OUID = appSecretGuardOUID
	backendAppID, err := testutils.CreateApplication(appSecretBackendApp)
	ts.Require().NoError(err, "failed to create backend application")
	appSecretBackendAppID = backendAppID

	appSecretRedirectApp.AuthFlowID = flowID
	appSecretRedirectApp.OUID = appSecretGuardOUID
	redirectAppID, err := testutils.CreateApplication(appSecretRedirectApp)
	ts.Require().NoError(err, "failed to create redirect application")
	appSecretRedirectAppID = redirectAppID

	// The backend app is eligible for an App Secret, so creation must have issued one.
	ts.Require().NotEmpty(testutils.GetAppSecret(appSecretBackendAppID),
		"backend app should be issued an App Secret at creation")
	// The redirect app must never be issued an App Secret.
	ts.Require().Empty(testutils.GetAppSecret(appSecretRedirectAppID),
		"redirect app should not be issued an App Secret")
}

func (ts *AppSecretFlowTestSuite) TearDownSuite() {
	if err := testutils.CleanupUsers(ts.config.CreatedUserIDs); err != nil {
		ts.T().Logf("failed to cleanup users: %v", err)
	}
	if appSecretBackendAppID != "" {
		if err := testutils.DeleteApplication(appSecretBackendAppID); err != nil {
			ts.T().Logf("failed to delete backend application: %v", err)
		}
	}
	if appSecretRedirectAppID != "" {
		if err := testutils.DeleteApplication(appSecretRedirectAppID); err != nil {
			ts.T().Logf("failed to delete redirect application: %v", err)
		}
	}
	for _, id := range ts.config.CreatedFlowIDs {
		if err := testutils.DeleteFlow(id); err != nil {
			ts.T().Logf("failed to delete flow %s: %v", id, err)
		}
	}
	if appSecretGuardUserTypeID != "" {
		if err := testutils.DeleteUserType(appSecretGuardUserTypeID); err != nil {
			ts.T().Logf("failed to delete user type: %v", err)
		}
	}
	if appSecretGuardOUID != "" {
		if err := testutils.DeleteOrganizationUnit(appSecretGuardOUID); err != nil {
			ts.T().Logf("failed to delete OU: %v", err)
		}
	}
}

// executeNewFlow posts a new-flow INIT request and returns the HTTP status code and parsed error
// body. It does not inject a registered App Secret, so the body controls exactly what is sent.
func (ts *AppSecretFlowTestSuite) executeNewFlow(body map[string]interface{}) (int, *common.ErrorResponse) {
	reqBody, err := json.Marshal(body)
	ts.Require().NoError(err, "failed to marshal flow request")

	req, err := http.NewRequest("POST", testutils.TestServerURL+"/flow/execute", bytes.NewReader(reqBody))
	ts.Require().NoError(err, "failed to create flow request")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err, "failed to send flow request")
	defer resp.Body.Close()

	var errResp common.ErrorResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&errResp),
		"flow error response should be valid JSON")
	return resp.StatusCode, &errResp
}

// A backend app that presents its valid App Secret may initiate a flow.
func (ts *AppSecretFlowTestSuite) TestBackendApp_ValidAppSecret_Allowed() {
	// InitiateAuthenticationFlow injects the App Secret registered for the app at creation.
	flowStep, err := common.InitiateAuthenticationFlow(appSecretBackendAppID, false, nil, "")
	ts.Require().NoError(err, "flow initiation with a valid App Secret should succeed")
	ts.Require().NotEmpty(flowStep.ExecutionID, "a new flow should return an execution ID")
}

// A backend app that omits the App Secret is rejected with 401 (FES-1012).
func (ts *AppSecretFlowTestSuite) TestBackendApp_MissingAppSecret_Rejected() {
	status, errResp := ts.executeNewFlow(map[string]interface{}{
		"applicationId": appSecretBackendAppID,
		"flowType":      "AUTHENTICATION",
	})

	ts.Require().Equal(http.StatusUnauthorized, status)
	ts.Require().Equal("FES-1012", errResp.Code)
}

// A backend app that presents an incorrect App Secret is rejected with 401 (FES-1013).
func (ts *AppSecretFlowTestSuite) TestBackendApp_InvalidAppSecret_Rejected() {
	status, errResp := ts.executeNewFlow(map[string]interface{}{
		"applicationId": appSecretBackendAppID,
		"appSecret":     "wrong-secret",
		"flowType":      "AUTHENTICATION",
	})

	ts.Require().Equal(http.StatusUnauthorized, status)
	ts.Require().Equal("FES-1013", errResp.Code)
}

// A redirect-based (authorization_code) app cannot initiate a flow directly; it is blocked with
// 403 (FES-1011) regardless of any App Secret value supplied.
func (ts *AppSecretFlowTestSuite) TestRedirectApp_DirectInitiation_Forbidden() {
	status, errResp := ts.executeNewFlow(map[string]interface{}{
		"applicationId": appSecretRedirectAppID,
		"flowType":      "AUTHENTICATION",
	})

	ts.Require().Equal(http.StatusForbidden, status)
	ts.Require().Equal("FES-1011", errResp.Code)
}

// Flow continuation (carrying an executionId) is not subject to the guard: once a backend app has
// initiated a flow with its App Secret, subsequent steps complete without resupplying it.
func (ts *AppSecretFlowTestSuite) TestBackendApp_Continuation_NoAppSecretRequired() {
	flowStep, err := common.InitiateAuthenticationFlow(appSecretBackendAppID, false, nil, "")
	ts.Require().NoError(err, "flow initiation should succeed")
	ts.Require().NotEmpty(flowStep.ExecutionID)

	completed, err := common.CompleteFlow(
		flowStep.ExecutionID,
		map[string]string{"username": "appsecretguarduser", "password": "testpassword"},
		"action_001",
		flowStep.ChallengeToken,
	)
	ts.Require().NoError(err, "flow continuation should not require an App Secret")
	ts.Require().Equal("COMPLETE", completed.FlowStatus)
	ts.Require().NotEmpty(completed.Assertion, "a completed authentication flow returns an assertion")
}
