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

package application

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	appsEndpoint             = "/applications"
	flowsEndpoint            = "/flows"
	flowMismatchErrorCode    = "APP-1039"
	registrationCalleeHandle = "flowref-registration-callee"
	recoveryCalleeHandle     = "flowref-recovery-callee"
	authCallerHandle         = "flowref-auth-caller"
)

// FlowReferenceValidationTestSuite exercises the app-side cross-flow reference behavior:
// on app create/update, if the app's AuthFlow (or another starting flow) transitively invokes a
// REGISTRATION / RECOVERY flow via a CALL node, the app either has to declare the matching binding
// or leave it unset (in which case the binding is auto-filled in a disabled state). Genuine
// mismatches still reject.
type FlowReferenceValidationTestSuite struct {
	suite.Suite
	ouID        string
	regCalleeID string
	recCalleeID string
	authFlowID  string
	createdApps []string
	extraFlows  []string
}

func TestFlowReferenceValidationTestSuite(t *testing.T) {
	suite.Run(t, new(FlowReferenceValidationTestSuite))
}

// ----- flow model (minimal subset used by these tests) -----

type flowDefinition struct {
	Name     string           `json:"name"`
	Handle   string           `json:"handle,omitempty"`
	FlowType string           `json:"flowType"`
	Nodes    []nodeDefinition `json:"nodes"`
}

type nodeDefinition struct {
	ID        string              `json:"id"`
	Type      string              `json:"type"`
	Executor  *executorDefinition `json:"executor,omitempty"`
	Flow      *flowRefDefinition  `json:"flow,omitempty"`
	OnSuccess string              `json:"onSuccess,omitempty"`
	OnFailure string              `json:"onFailure,omitempty"`
}

type executorDefinition struct {
	Name string `json:"name"`
}

type flowRefDefinition struct {
	Ref string `json:"ref"`
}

type createdFlowResponse struct {
	ID string `json:"id"`
}

type errorResponse struct {
	Code             string `json:"code"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// ----- setup / teardown -----

func (suite *FlowReferenceValidationTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "flowref_ou",
		Name:        "FlowRef Test OU",
		Description: "OU for flow reference validation integration tests",
	})
	suite.Require().NoError(err, "failed to create test OU")
	suite.ouID = ouID

	regCallee := flowDefinition{
		Name:     "FlowRef Registration Callee",
		Handle:   registrationCalleeHandle,
		FlowType: "REGISTRATION",
		Nodes: []nodeDefinition{
			{ID: "START", Type: "START", OnSuccess: "user_type_resolver"},
			{
				ID:        "user_type_resolver",
				Type:      "TASK_EXECUTION",
				Executor:  &executorDefinition{Name: "UserTypeResolver"},
				OnSuccess: "provisioning",
			},
			{
				ID:        "provisioning",
				Type:      "TASK_EXECUTION",
				Executor:  &executorDefinition{Name: "ProvisioningExecutor"},
				OnSuccess: "END",
			},
			{ID: "END", Type: "END"},
		},
	}
	suite.regCalleeID = suite.createFlowReturningID(regCallee)

	recCallee := flowDefinition{
		Name:     "FlowRef Recovery Callee",
		Handle:   recoveryCalleeHandle,
		FlowType: "RECOVERY",
		Nodes: []nodeDefinition{
			{ID: "START", Type: "START", OnSuccess: "identify"},
			{
				ID:        "identify",
				Type:      "TASK_EXECUTION",
				Executor:  &executorDefinition{Name: "IdentifyingExecutor"},
				OnSuccess: "END",
			},
			{ID: "END", Type: "END"},
		},
	}
	suite.recCalleeID = suite.createFlowReturningID(recCallee)

	// Authentication caller that invokes the registration callee via a CALL node.
	authCaller := flowDefinition{
		Name:     "FlowRef Authentication Caller",
		Handle:   authCallerHandle,
		FlowType: "AUTHENTICATION",
		Nodes: []nodeDefinition{
			{ID: "START", Type: "START", OnSuccess: "auth_assert"},
			{
				ID:        "auth_assert",
				Type:      "TASK_EXECUTION",
				Executor:  &executorDefinition{Name: "AuthAssertExecutor"},
				OnSuccess: "call_reg",
			},
			{
				ID:        "call_reg",
				Type:      "CALL",
				Flow:      &flowRefDefinition{Ref: suite.regCalleeID},
				OnSuccess: "END",
				OnFailure: "END",
			},
			{ID: "END", Type: "END"},
		},
	}
	suite.authFlowID = suite.createFlowReturningID(authCaller)
}

func (suite *FlowReferenceValidationTestSuite) TearDownSuite() {
	for _, appID := range suite.createdApps {
		if err := testutils.DeleteApplication(appID); err != nil {
			suite.T().Logf("failed to delete app %s: %v", appID, err)
		}
	}
	for _, flowID := range suite.extraFlows {
		if err := testutils.DeleteFlow(flowID); err != nil {
			suite.T().Logf("failed to delete flow %s: %v", flowID, err)
		}
	}
	for _, flowID := range []string{suite.authFlowID, suite.regCalleeID, suite.recCalleeID} {
		if flowID != "" {
			if err := testutils.DeleteFlow(flowID); err != nil {
				suite.T().Logf("failed to delete flow %s: %v", flowID, err)
			}
		}
	}
	if suite.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(suite.ouID); err != nil {
			suite.T().Logf("failed to delete ou %s: %v", suite.ouID, err)
		}
	}
}

// ----- create scenarios -----

func (suite *FlowReferenceValidationTestSuite) TestCreateApp_MatchingRegistrationTargetPasses() {
	app := suite.baseApp("flowref_match")
	app.AuthFlowID = suite.authFlowID
	app.RegistrationFlowID = suite.regCalleeID
	app.IsRegistrationFlowEnabled = true

	appID, err := testutils.CreateApplication(app)
	suite.Require().NoError(err)
	suite.createdApps = append(suite.createdApps, appID)
}

func (suite *FlowReferenceValidationTestSuite) TestCreateApp_MissingRegistrationBindingAutoFilled() {
	app := suite.baseApp("flowref_autofill_reg")
	app.AuthFlowID = suite.authFlowID
	// RegistrationFlowID intentionally omitted — the auth flow calls a REGISTRATION target, so the
	// server must auto-fill RegistrationFlowID with the reg-callee ID and force
	// IsRegistrationFlowEnabled to false.
	app.IsRegistrationFlowEnabled = true

	appID, err := testutils.CreateApplication(app)
	suite.Require().NoError(err)
	suite.createdApps = append(suite.createdApps, appID)

	persisted := suite.getApp(appID)
	suite.Equal(suite.regCalleeID, persisted["registrationFlowId"],
		"auto-fill must populate registrationFlowId from the reachable target")
	suite.Equal(false, persisted["isRegistrationFlowEnabled"],
		"auto-fill must force isRegistrationFlowEnabled to false regardless of the caller's value")
}

func (suite *FlowReferenceValidationTestSuite) TestCreateApp_MissingRecoveryBindingAutoFilled() {
	// Build a fresh auth flow that calls the RECOVERY callee (rather than reusing the shared
	// authFlow which calls the registration callee).
	authCallingRec := flowDefinition{
		Name:     "FlowRef Authentication Calling Recovery",
		Handle:   "flowref-auth-calling-rec",
		FlowType: "AUTHENTICATION",
		Nodes: []nodeDefinition{
			{ID: "START", Type: "START", OnSuccess: "auth_assert"},
			{
				ID:        "auth_assert",
				Type:      "TASK_EXECUTION",
				Executor:  &executorDefinition{Name: "AuthAssertExecutor"},
				OnSuccess: "call_rec",
			},
			{
				ID:        "call_rec",
				Type:      "CALL",
				Flow:      &flowRefDefinition{Ref: suite.recCalleeID},
				OnSuccess: "END",
				OnFailure: "END",
			},
			{ID: "END", Type: "END"},
		},
	}
	authCallingRecID := suite.createFlowReturningID(authCallingRec)
	suite.extraFlows = append(suite.extraFlows, authCallingRecID)

	app := suite.baseApp("flowref_autofill_rec")
	app.AuthFlowID = authCallingRecID
	app.IsRecoveryFlowEnabled = true

	appID, err := testutils.CreateApplication(app)
	suite.Require().NoError(err)
	suite.createdApps = append(suite.createdApps, appID)

	persisted := suite.getApp(appID)
	suite.Equal(suite.recCalleeID, persisted["recoveryFlowId"])
	suite.Equal(false, persisted["isRecoveryFlowEnabled"])
}

func (suite *FlowReferenceValidationTestSuite) TestCreateApp_MismatchedRegistrationRejected() {
	altReg := flowDefinition{
		Name:     "FlowRef Alternate Registration",
		Handle:   "flowref-alt-registration",
		FlowType: "REGISTRATION",
		Nodes: []nodeDefinition{
			{ID: "START", Type: "START", OnSuccess: "user_type_resolver"},
			{
				ID:        "user_type_resolver",
				Type:      "TASK_EXECUTION",
				Executor:  &executorDefinition{Name: "UserTypeResolver"},
				OnSuccess: "provisioning",
			},
			{
				ID:        "provisioning",
				Type:      "TASK_EXECUTION",
				Executor:  &executorDefinition{Name: "ProvisioningExecutor"},
				OnSuccess: "END",
			},
			{ID: "END", Type: "END"},
		},
	}
	altRegID := suite.createFlowReturningID(altReg)
	suite.extraFlows = append(suite.extraFlows, altRegID)

	app := suite.baseApp("flowref_mismatch_reg")
	app.AuthFlowID = suite.authFlowID
	app.RegistrationFlowID = altRegID // differs from the reg-callee the auth flow calls
	suite.createApplicationExpectFlowMismatch(app)
}

func (suite *FlowReferenceValidationTestSuite) TestCreateApp_ReverseAuthReferenceMismatchRejected() {
	// A REGISTRATION flow that calls an AUTHENTICATION flow different from the app's AuthFlowID
	// must be rejected. Auth has no auto-fill because it lacks a disable toggle.
	regCallingAuth := flowDefinition{
		Name:     "FlowRef Registration Calling Authentication",
		Handle:   "flowref-reg-calling-auth",
		FlowType: "REGISTRATION",
		Nodes: []nodeDefinition{
			{ID: "START", Type: "START", OnSuccess: "user_type_resolver"},
			{
				ID:        "user_type_resolver",
				Type:      "TASK_EXECUTION",
				Executor:  &executorDefinition{Name: "UserTypeResolver"},
				OnSuccess: "provisioning",
			},
			{
				ID:        "provisioning",
				Type:      "TASK_EXECUTION",
				Executor:  &executorDefinition{Name: "ProvisioningExecutor"},
				OnSuccess: "call_auth",
			},
			{
				ID:        "call_auth",
				Type:      "CALL",
				Flow:      &flowRefDefinition{Ref: suite.authFlowID},
				OnSuccess: "END",
				OnFailure: "END",
			},
			{ID: "END", Type: "END"},
		},
	}
	regCallingAuthID := suite.createFlowReturningID(regCallingAuth)
	suite.extraFlows = append(suite.extraFlows, regCallingAuthID)

	loneAuth := flowDefinition{
		Name:     "FlowRef Lone Authentication",
		Handle:   "flowref-lone-auth",
		FlowType: "AUTHENTICATION",
		Nodes: []nodeDefinition{
			{ID: "START", Type: "START", OnSuccess: "auth_assert"},
			{
				ID:        "auth_assert",
				Type:      "TASK_EXECUTION",
				Executor:  &executorDefinition{Name: "AuthAssertExecutor"},
				OnSuccess: "END",
			},
			{ID: "END", Type: "END"},
		},
	}
	loneAuthID := suite.createFlowReturningID(loneAuth)
	suite.extraFlows = append(suite.extraFlows, loneAuthID)

	app := suite.baseApp("flowref_reverse_auth")
	app.AuthFlowID = loneAuthID
	app.RegistrationFlowID = regCallingAuthID
	suite.createApplicationExpectFlowMismatch(app)
}

func (suite *FlowReferenceValidationTestSuite) TestCreateApp_AuthReferencedByRegistrationMatching() {
	// Positive reverse-direction case: a REGISTRATION flow that calls the app's configured
	// AuthFlowID must be accepted. The app's AuthFlowID uses a fresh lone auth flow (no CALL nodes)
	// so the auth-side transitive walk doesn't add any conflicting registration targets.
	loneAuth := flowDefinition{
		Name:     "FlowRef Reverse-Match Lone Authentication",
		Handle:   "flowref-reverse-match-lone-auth",
		FlowType: "AUTHENTICATION",
		Nodes: []nodeDefinition{
			{ID: "START", Type: "START", OnSuccess: "auth_assert"},
			{
				ID:        "auth_assert",
				Type:      "TASK_EXECUTION",
				Executor:  &executorDefinition{Name: "AuthAssertExecutor"},
				OnSuccess: "END",
			},
			{ID: "END", Type: "END"},
		},
	}
	loneAuthID := suite.createFlowReturningID(loneAuth)
	suite.extraFlows = append(suite.extraFlows, loneAuthID)

	regCallingAuth := flowDefinition{
		Name:     "FlowRef Registration Calling Matching Authentication",
		Handle:   "flowref-reg-calling-match-auth",
		FlowType: "REGISTRATION",
		Nodes: []nodeDefinition{
			{ID: "START", Type: "START", OnSuccess: "user_type_resolver"},
			{
				ID:        "user_type_resolver",
				Type:      "TASK_EXECUTION",
				Executor:  &executorDefinition{Name: "UserTypeResolver"},
				OnSuccess: "provisioning",
			},
			{
				ID:        "provisioning",
				Type:      "TASK_EXECUTION",
				Executor:  &executorDefinition{Name: "ProvisioningExecutor"},
				OnSuccess: "call_auth",
			},
			{
				ID:        "call_auth",
				Type:      "CALL",
				Flow:      &flowRefDefinition{Ref: loneAuthID},
				OnSuccess: "END",
				OnFailure: "END",
			},
			{ID: "END", Type: "END"},
		},
	}
	regCallingAuthID := suite.createFlowReturningID(regCallingAuth)
	suite.extraFlows = append(suite.extraFlows, regCallingAuthID)

	app := suite.baseApp("flowref_reverse_auth_match")
	app.AuthFlowID = loneAuthID
	app.RegistrationFlowID = regCallingAuthID
	app.IsRegistrationFlowEnabled = true
	appID, err := testutils.CreateApplication(app)
	suite.Require().NoError(err)
	suite.createdApps = append(suite.createdApps, appID)
}

// ----- update scenarios -----

func (suite *FlowReferenceValidationTestSuite) TestUpdateApp_IntroducingMismatchRejected() {
	altReg := flowDefinition{
		Name:     "FlowRef Update Alternate Registration",
		Handle:   "flowref-update-alt-registration",
		FlowType: "REGISTRATION",
		Nodes: []nodeDefinition{
			{ID: "START", Type: "START", OnSuccess: "user_type_resolver"},
			{
				ID:        "user_type_resolver",
				Type:      "TASK_EXECUTION",
				Executor:  &executorDefinition{Name: "UserTypeResolver"},
				OnSuccess: "provisioning",
			},
			{
				ID:        "provisioning",
				Type:      "TASK_EXECUTION",
				Executor:  &executorDefinition{Name: "ProvisioningExecutor"},
				OnSuccess: "END",
			},
			{ID: "END", Type: "END"},
		},
	}
	altRegID := suite.createFlowReturningID(altReg)
	suite.extraFlows = append(suite.extraFlows, altRegID)

	app := suite.baseApp("flowref_update_app")
	app.AuthFlowID = suite.authFlowID
	app.RegistrationFlowID = suite.regCalleeID
	app.IsRegistrationFlowEnabled = true
	appID, err := testutils.CreateApplication(app)
	suite.Require().NoError(err)
	suite.createdApps = append(suite.createdApps, appID)

	// Point RegistrationFlowID at a flow the auth flow does not call — must be rejected.
	suite.updateApplicationExpectFlowMismatch(appID, map[string]interface{}{
		"registrationFlowId": altRegID,
	})
}

func (suite *FlowReferenceValidationTestSuite) TestUpdateApp_MissingRegistrationBindingAutoFilled() {
	// Onboard-like path via update: create the app with only AuthFlowID set (which triggers
	// auto-fill at create time already). Then clear the field via an update payload that omits
	// registrationFlowId, and verify the update path also reconciles it. To make this test
	// meaningful, create an app with an auth flow that has NO reachable registration flow first,
	// then update the app's AuthFlowID to one that does — the update must auto-fill.
	quietAuth := flowDefinition{
		Name:     "FlowRef Quiet Authentication",
		Handle:   "flowref-quiet-auth",
		FlowType: "AUTHENTICATION",
		Nodes: []nodeDefinition{
			{ID: "START", Type: "START", OnSuccess: "auth_assert"},
			{
				ID:        "auth_assert",
				Type:      "TASK_EXECUTION",
				Executor:  &executorDefinition{Name: "AuthAssertExecutor"},
				OnSuccess: "END",
			},
			{ID: "END", Type: "END"},
		},
	}
	quietAuthID := suite.createFlowReturningID(quietAuth)
	suite.extraFlows = append(suite.extraFlows, quietAuthID)

	app := suite.baseApp("flowref_update_autofill")
	app.AuthFlowID = quietAuthID
	app.IsRegistrationFlowEnabled = true
	appID, err := testutils.CreateApplication(app)
	suite.Require().NoError(err)
	suite.createdApps = append(suite.createdApps, appID)

	// Sanity: nothing was auto-filled on create since quietAuth has no calls.
	initial := suite.getApp(appID)
	suite.Empty(initial["registrationFlowId"])

	// Switch AuthFlowID to the caller that invokes the registration callee — update must auto-fill.
	suite.updateApplicationExpectSuccess(appID, map[string]interface{}{
		"authFlowId": suite.authFlowID,
	})

	updated := suite.getApp(appID)
	suite.Equal(suite.regCalleeID, updated["registrationFlowId"],
		"update must auto-fill registrationFlowId from the reachable target")
	suite.Equal(false, updated["isRegistrationFlowEnabled"],
		"update auto-fill must force isRegistrationFlowEnabled to false")
}

// ----- helpers -----

func (suite *FlowReferenceValidationTestSuite) baseApp(nameSuffix string) testutils.Application {
	return testutils.Application{
		OUID:         suite.ouID,
		Name:         "FlowRef App " + nameSuffix,
		Description:  "Application used for flow reference validation integration tests",
		ClientID:     "flowref_" + nameSuffix + "_client",
		ClientSecret: "flowref_" + nameSuffix + "_secret",
		RedirectURIs: []string{"http://localhost:3000/callback"},
	}
}

func (suite *FlowReferenceValidationTestSuite) createFlowReturningID(flowDef flowDefinition) string {
	body, _ := json.Marshal(flowDef)
	req, _ := http.NewRequest(http.MethodPost, testServerURL+flowsEndpoint, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()
	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	suite.Require().Equalf(http.StatusCreated, resp.StatusCode,
		"expected 201 creating flow, got %d: %s", resp.StatusCode, string(bodyBytes))

	var created createdFlowResponse
	suite.Require().NoError(json.Unmarshal(bodyBytes, &created))
	return created.ID
}

func (suite *FlowReferenceValidationTestSuite) getApp(appID string) map[string]interface{} {
	client := testutils.GetHTTPClient()
	req, _ := http.NewRequest(http.MethodGet, testServerURL+appsEndpoint+"/"+appID, nil)
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	suite.Require().Equalf(http.StatusOK, resp.StatusCode,
		"expected 200 fetching app, got %d: %s", resp.StatusCode, string(body))

	var payload map[string]interface{}
	suite.Require().NoError(json.Unmarshal(body, &payload))
	return payload
}

func (suite *FlowReferenceValidationTestSuite) createApplicationExpectFlowMismatch(
	app testutils.Application) {
	body, _ := json.Marshal(suite.applicationRequestBody(app))
	req, _ := http.NewRequest(http.MethodPost, testServerURL+appsEndpoint, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()
	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	suite.Require().Equalf(http.StatusBadRequest, resp.StatusCode,
		"expected 400 for flow mismatch, got %d: %s", resp.StatusCode, string(bodyBytes))

	var errResp errorResponse
	suite.Require().NoError(json.Unmarshal(bodyBytes, &errResp))
	suite.Equal(flowMismatchErrorCode, errResp.Code,
		fmt.Sprintf("expected %s, got %s: %s", flowMismatchErrorCode, errResp.Code, string(bodyBytes)))
}

func (suite *FlowReferenceValidationTestSuite) updateApplicationExpectFlowMismatch(
	appID string, overrides map[string]interface{}) {
	putBody, putResp := suite.putApplicationOverrides(appID, overrides)
	suite.Require().Equalf(http.StatusBadRequest, putResp.StatusCode,
		"expected 400 on update, got %d: %s", putResp.StatusCode, string(putBody))
	var errResp errorResponse
	suite.Require().NoError(json.Unmarshal(putBody, &errResp))
	suite.Equal(flowMismatchErrorCode, errResp.Code,
		fmt.Sprintf("expected %s, got %s: %s", flowMismatchErrorCode, errResp.Code, string(putBody)))
}

func (suite *FlowReferenceValidationTestSuite) updateApplicationExpectSuccess(
	appID string, overrides map[string]interface{}) {
	putBody, putResp := suite.putApplicationOverrides(appID, overrides)
	suite.Require().Equalf(http.StatusOK, putResp.StatusCode,
		"expected 200 on update, got %d: %s", putResp.StatusCode, string(putBody))
}

func (suite *FlowReferenceValidationTestSuite) putApplicationOverrides(
	appID string, overrides map[string]interface{}) ([]byte, *http.Response) {
	client := testutils.GetHTTPClient()

	getReq, _ := http.NewRequest(http.MethodGet, testServerURL+appsEndpoint+"/"+appID, nil)
	getReq.Header.Set("Accept", "application/json")
	getResp, err := client.Do(getReq)
	suite.Require().NoError(err)
	getBody, _ := io.ReadAll(getResp.Body)
	getResp.Body.Close()
	suite.Require().Equalf(http.StatusOK, getResp.StatusCode,
		"expected 200 fetching app, got %d: %s", getResp.StatusCode, string(getBody))

	var appPayload map[string]interface{}
	suite.Require().NoError(json.Unmarshal(getBody, &appPayload))
	for k, v := range overrides {
		appPayload[k] = v
	}
	// Client secret is required on update because the server doesn't return it on GET.
	appPayload["clientSecret"] = "secret123"

	body, _ := json.Marshal(appPayload)
	putReq, _ := http.NewRequest(http.MethodPut, testServerURL+appsEndpoint+"/"+appID, bytes.NewBuffer(body))
	putReq.Header.Set("Content-Type", "application/json")
	putResp, err := client.Do(putReq)
	suite.Require().NoError(err)
	defer putResp.Body.Close()

	putBody, _ := io.ReadAll(putResp.Body)
	return putBody, putResp
}

// applicationRequestBody mirrors testutils.CreateApplication's payload assembly but returns a
// raw map so the caller can hit the endpoint directly and inspect the error response.
func (suite *FlowReferenceValidationTestSuite) applicationRequestBody(
	app testutils.Application) map[string]interface{} {
	inboundAuthConfig := []map[string]interface{}{
		{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId":     app.ClientID,
				"clientSecret": app.ClientSecret,
				"redirectUris": app.RedirectURIs,
				"grantTypes":   []string{"client_credentials", "urn:ietf:params:oauth:grant-type:token-exchange"},
			},
		},
	}
	return map[string]interface{}{
		"ouId":                      app.OUID,
		"name":                      app.Name,
		"description":               app.Description,
		"isRegistrationFlowEnabled": app.IsRegistrationFlowEnabled,
		"isRecoveryFlowEnabled":     app.IsRecoveryFlowEnabled,
		"authFlowId":                app.AuthFlowID,
		"registrationFlowId":        app.RegistrationFlowID,
		"recoveryFlowId":            app.RecoveryFlowID,
		"inboundAuthConfig":         inboundAuthConfig,
	}
}
