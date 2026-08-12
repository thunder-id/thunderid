// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
// REGISTRATION / RECOVERY flow via a CALL node, the app must either declare a matching binding
// (with the corresponding enable flag on) or leave it disabled — in the disabled case the server
// persists an empty binding regardless of what the auth flow calls. Sign-out still auto-fills
// because it has no enable toggle. Genuine mismatches with an enabled binding still reject.
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

func (suite *FlowReferenceValidationTestSuite) TestCreateApp_DisabledRegistrationLeftEmpty() {
	// Auth flow calls a REGISTRATION target, but the caller has left IsRegistrationFlowEnabled
	// unset (false). The server must persist an empty registration binding.
	app := suite.baseApp("flowref_disabled_reg")
	app.AuthFlowID = suite.authFlowID

	appID, err := testutils.CreateApplication(app)
	suite.Require().NoError(err)
	suite.createdApps = append(suite.createdApps, appID)

	persisted := suite.getApp(appID)
	suite.Empty(persisted["registrationFlowId"],
		"disabled registration binding must not be auto-filled from the auth flow's reachable target")
	suite.Equal(false, persisted["isRegistrationFlowEnabled"])
}

func (suite *FlowReferenceValidationTestSuite) TestCreateApp_DisabledRecoveryLeftEmpty() {
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

	app := suite.baseApp("flowref_disabled_rec")
	app.AuthFlowID = authCallingRecID

	appID, err := testutils.CreateApplication(app)
	suite.Require().NoError(err)
	suite.createdApps = append(suite.createdApps, appID)

	persisted := suite.getApp(appID)
	suite.Empty(persisted["recoveryFlowId"],
		"disabled recovery binding must not be auto-filled from the auth flow's reachable target")
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
	app.IsRegistrationFlowEnabled = true
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
	app.IsRegistrationFlowEnabled = true
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

func (suite *FlowReferenceValidationTestSuite) TestUpdateApp_DisabledRegistrationStaysEmptyOnAuthFlowSwitch() {
	// Create an app with a "quiet" auth flow that has no CALL nodes, then switch its AuthFlowID
	// to one that transitively references a REGISTRATION target. The caller has left the
	// registration binding disabled, so the update must NOT auto-fill it.
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

	app := suite.baseApp("flowref_update_no_autofill")
	app.AuthFlowID = quietAuthID
	appID, err := testutils.CreateApplication(app)
	suite.Require().NoError(err)
	suite.createdApps = append(suite.createdApps, appID)

	initial := suite.getApp(appID)
	suite.Empty(initial["registrationFlowId"])

	// Switch AuthFlowID to the caller that invokes the registration callee. The registration
	// binding must remain empty since the caller kept isRegistrationFlowEnabled=false.
	suite.updateApplicationExpectSuccess(appID, map[string]interface{}{
		"authFlowId": suite.authFlowID,
	})

	updated := suite.getApp(appID)
	suite.Empty(updated["registrationFlowId"],
		"disabled registration binding must remain empty after an auth-flow switch")
	suite.Equal(false, updated["isRegistrationFlowEnabled"])
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
		"type":                      "fullstack",
		"isRegistrationFlowEnabled": app.IsRegistrationFlowEnabled,
		"isRecoveryFlowEnabled":     app.IsRecoveryFlowEnabled,
		"authFlowId":                app.AuthFlowID,
		"registrationFlowId":        app.RegistrationFlowID,
		"recoveryFlowId":            app.RecoveryFlowID,
		"inboundAuthConfig":         inboundAuthConfig,
	}
}
