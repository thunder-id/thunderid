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

package mgt

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
	flowUpdateBlockedErrorCode = "FLM-1026"
	registrationCalleeHandle   = "flowref-registration-callee"
	authCallerHandle           = "flowref-auth-caller"
)

// FlowReferenceValidationTestSuite exercises the flow-update side of cross-flow reference
// validation: updating a flow that is referenced by one or more applications must reject the
// update when it would introduce a genuine mismatch on any bound app, and must NOT reject when
// no bound app has an explicit conflicting binding.
type FlowReferenceValidationTestSuite struct {
	suite.Suite
	ouID        string
	regCalleeID string
	authFlowID  string
	createdApps []string
	extraFlows  []string
}

func TestFlowReferenceValidationTestSuite(t *testing.T) {
	suite.Run(t, new(FlowReferenceValidationTestSuite))
}

func (suite *FlowReferenceValidationTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "flowref_flow_ou",
		Name:        "FlowRef Flow-Update Test OU",
		Description: "OU for flow reference validation (flow-update) integration tests",
	})
	suite.Require().NoError(err, "failed to create test OU")
	suite.ouID = ouID

	regCallee := FlowDefinition{
		Name:     "FlowRef Registration Callee",
		Handle:   registrationCalleeHandle,
		FlowType: "REGISTRATION",
		Nodes: []NodeDefinition{
			{ID: "START", Type: "START", OnSuccess: "user_type_resolver"},
			{
				ID:        "user_type_resolver",
				Type:      "TASK_EXECUTION",
				Executor:  &ExecutorDefinition{Name: "UserTypeResolver"},
				OnSuccess: "provisioning",
			},
			{
				ID:        "provisioning",
				Type:      "TASK_EXECUTION",
				Executor:  &ExecutorDefinition{Name: "ProvisioningExecutor"},
				OnSuccess: "END",
			},
			{ID: "END", Type: "END"},
		},
	}
	suite.regCalleeID = suite.createFlowReturningID(regCallee)

	authCaller := FlowDefinition{
		Name:     "FlowRef Authentication Caller",
		Handle:   authCallerHandle,
		FlowType: "AUTHENTICATION",
		Nodes: []NodeDefinition{
			{ID: "START", Type: "START", OnSuccess: "auth_assert"},
			{
				ID:        "auth_assert",
				Type:      "TASK_EXECUTION",
				Executor:  &ExecutorDefinition{Name: "AuthAssertExecutor"},
				OnSuccess: "call_reg",
			},
			{
				ID:        "call_reg",
				Type:      "CALL",
				Flow:      &FlowReferenceDefinition{Ref: suite.regCalleeID},
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
	for _, flowID := range []string{suite.authFlowID, suite.regCalleeID} {
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

func (suite *FlowReferenceValidationTestSuite) TestUpdateFlow_IntroducingMismatchRejected() {
	// Create an app bound to authFlow + regCallee (a matching pair), then update authFlow to add
	// a CALL to a fresh registration flow that the app is not bound to. Since the app has a
	// concrete RegistrationFlowID set and the newly-reachable target does not match it, the
	// flow update must be rejected via the resource-dependency update validator.
	app := suite.baseApp("flowref_update_flow_mismatch")
	app.AuthFlowID = suite.authFlowID
	app.RegistrationFlowID = suite.regCalleeID
	app.IsRegistrationFlowEnabled = true
	appID, err := testutils.CreateApplication(app)
	suite.Require().NoError(err)
	suite.createdApps = append(suite.createdApps, appID)

	newReg := FlowDefinition{
		Name:     "FlowRef Newly Introduced Registration",
		Handle:   "flowref-newly-introduced-registration",
		FlowType: "REGISTRATION",
		Nodes: []NodeDefinition{
			{ID: "START", Type: "START", OnSuccess: "user_type_resolver"},
			{
				ID:        "user_type_resolver",
				Type:      "TASK_EXECUTION",
				Executor:  &ExecutorDefinition{Name: "UserTypeResolver"},
				OnSuccess: "provisioning",
			},
			{
				ID:        "provisioning",
				Type:      "TASK_EXECUTION",
				Executor:  &ExecutorDefinition{Name: "ProvisioningExecutor"},
				OnSuccess: "END",
			},
			{ID: "END", Type: "END"},
		},
	}
	newRegID := suite.createFlowReturningID(newReg)
	suite.extraFlows = append(suite.extraFlows, newRegID)

	brokenAuth := FlowDefinition{
		Name:     "FlowRef Authentication Caller",
		Handle:   authCallerHandle,
		FlowType: "AUTHENTICATION",
		Nodes: []NodeDefinition{
			{ID: "START", Type: "START", OnSuccess: "auth_assert"},
			{
				ID:        "auth_assert",
				Type:      "TASK_EXECUTION",
				Executor:  &ExecutorDefinition{Name: "AuthAssertExecutor"},
				OnSuccess: "call_reg",
			},
			{
				ID:        "call_reg",
				Type:      "CALL",
				Flow:      &FlowReferenceDefinition{Ref: suite.regCalleeID},
				OnSuccess: "call_new_reg",
				OnFailure: "call_new_reg",
			},
			{
				ID:        "call_new_reg",
				Type:      "CALL",
				Flow:      &FlowReferenceDefinition{Ref: newRegID},
				OnSuccess: "END",
				OnFailure: "END",
			},
			{ID: "END", Type: "END"},
		},
	}
	suite.updateFlowExpectFlowMismatch(suite.authFlowID, brokenAuth)
}

func (suite *FlowReferenceValidationTestSuite) TestUpdateFlow_UnboundAppIsNotBlocked() {
	// The app references authFlow but leaves RegistrationFlowID unset. Auto-fill happens on the
	// create path, so to keep the app unbound we start from a "quiet" auth flow that has no
	// reachable registration target, and later update a different, non-referenced flow to
	// introduce a call to a new registration. That update must NOT be rejected because no bound
	// app has a conflicting binding.
	quietAuth := FlowDefinition{
		Name:     "FlowRef Quiet Authentication",
		Handle:   "flowref-quiet-auth",
		FlowType: "AUTHENTICATION",
		Nodes: []NodeDefinition{
			{ID: "START", Type: "START", OnSuccess: "auth_assert"},
			{
				ID:        "auth_assert",
				Type:      "TASK_EXECUTION",
				Executor:  &ExecutorDefinition{Name: "AuthAssertExecutor"},
				OnSuccess: "END",
			},
			{ID: "END", Type: "END"},
		},
	}
	quietAuthID := suite.createFlowReturningID(quietAuth)
	suite.extraFlows = append(suite.extraFlows, quietAuthID)

	app := suite.baseApp("flowref_update_flow_unbound")
	app.AuthFlowID = quietAuthID
	appID, err := testutils.CreateApplication(app)
	suite.Require().NoError(err)
	suite.createdApps = append(suite.createdApps, appID)

	newReg := FlowDefinition{
		Name:     "FlowRef Unbound Introduced Registration",
		Handle:   "flowref-unbound-introduced-registration",
		FlowType: "REGISTRATION",
		Nodes: []NodeDefinition{
			{ID: "START", Type: "START", OnSuccess: "user_type_resolver"},
			{
				ID:        "user_type_resolver",
				Type:      "TASK_EXECUTION",
				Executor:  &ExecutorDefinition{Name: "UserTypeResolver"},
				OnSuccess: "provisioning",
			},
			{
				ID:        "provisioning",
				Type:      "TASK_EXECUTION",
				Executor:  &ExecutorDefinition{Name: "ProvisioningExecutor"},
				OnSuccess: "END",
			},
			{ID: "END", Type: "END"},
		},
	}
	newRegID := suite.createFlowReturningID(newReg)
	suite.extraFlows = append(suite.extraFlows, newRegID)

	// Rewrite quietAuth to call the new registration flow. The referenced app has no
	// RegistrationFlowID configured, so the flow-update revalidation must not reject.
	quietAuthUpdated := FlowDefinition{
		Name:     "FlowRef Quiet Authentication",
		Handle:   "flowref-quiet-auth",
		FlowType: "AUTHENTICATION",
		Nodes: []NodeDefinition{
			{ID: "START", Type: "START", OnSuccess: "auth_assert"},
			{
				ID:        "auth_assert",
				Type:      "TASK_EXECUTION",
				Executor:  &ExecutorDefinition{Name: "AuthAssertExecutor"},
				OnSuccess: "call_reg",
			},
			{
				ID:        "call_reg",
				Type:      "CALL",
				Flow:      &FlowReferenceDefinition{Ref: newRegID},
				OnSuccess: "END",
				OnFailure: "END",
			},
			{ID: "END", Type: "END"},
		},
	}
	suite.updateFlowExpectSuccess(quietAuthID, quietAuthUpdated)
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

func (suite *FlowReferenceValidationTestSuite) createFlowReturningID(flowDef FlowDefinition) string {
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

	var created CompleteFlowDefinition
	suite.Require().NoError(json.Unmarshal(bodyBytes, &created))
	return created.ID
}

func (suite *FlowReferenceValidationTestSuite) updateFlowExpectFlowMismatch(
	flowID string, flowDef FlowDefinition) {
	body, resp := suite.putFlow(flowID, flowDef)
	suite.Require().Equalf(http.StatusBadRequest, resp.StatusCode,
		"expected 400 updating flow, got %d: %s", resp.StatusCode, string(body))
	var errResp ErrorResponse
	suite.Require().NoError(json.Unmarshal(body, &errResp))
	suite.Equal(flowUpdateBlockedErrorCode, errResp.Code,
		fmt.Sprintf("expected %s, got %s: %s", flowUpdateBlockedErrorCode, errResp.Code, string(body)))
}

func (suite *FlowReferenceValidationTestSuite) updateFlowExpectSuccess(
	flowID string, flowDef FlowDefinition) {
	body, resp := suite.putFlow(flowID, flowDef)
	suite.Require().Equalf(http.StatusOK, resp.StatusCode,
		"expected 200 updating flow, got %d: %s", resp.StatusCode, string(body))
}

func (suite *FlowReferenceValidationTestSuite) putFlow(
	flowID string, flowDef FlowDefinition) ([]byte, *http.Response) {
	body, _ := json.Marshal(flowDef)
	req, _ := http.NewRequest(http.MethodPut, testServerURL+flowsEndpoint+"/"+flowID, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()
	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	return bodyBytes, resp
}
