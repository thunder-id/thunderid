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

// This suite verifies the platform-attestation guard for POST /flow/execute for an iOS (public,
// authorization_code) application that configures Apple App Attest. Attestation takes precedence
// over the redirect classification, so the app may initiate a flow directly only by presenting a
// valid Attestation-Token header:
//   - a missing token is rejected with 401 (FES-1014);
//   - a malformed token is rejected with 401 (FES-1015). Unlike Play Integrity, App Attest is
//     verified entirely offline (no outbound API call), so a token that cannot be decoded is a
//     definitive rejection rather than a server-side condition — it never surfaces as a 500.
//
// A successful verification requires a genuine App Attest attestation object produced by a real
// Apple device, so that path is covered by unit tests and manual/E2E verification rather than here.

var appleAttestationGuardOU = testutils.OrganizationUnit{
	Handle:      "apple_attestation_guard_test_ou",
	Name:        "Apple Attestation Guard Test OU",
	Description: "Organization unit for Apple attestation guard flow testing",
	Parent:      nil,
}

var appleAttestationGuardUserType = testutils.UserType{
	Name: "apple_attestation_guard_person",
	Schema: map[string]interface{}{
		"username": map[string]interface{}{"type": "string"},
		"password": map[string]interface{}{"type": "string", "credential": true},
		"email":    map[string]interface{}{"type": "string"},
	},
}

var appleAttestationGuardFlow = testutils.Flow{
	Name:     "Apple Attestation Guard Auth Flow",
	FlowType: "AUTHENTICATION",
	Handle:   "auth_flow_apple_attestation_guard",
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

// appleAttestationMobileApp is a public, redirect-based (authorization_code) iOS application that
// configures Apple App Attest attestation. Attestation takes precedence over the redirect
// classification, so the app may initiate a flow directly — but only with a valid attestation token.
var appleAttestationMobileApp = testutils.Application{
	Name:                      "App Attest iOS App",
	Description:               "iOS application for Apple attestation guard testing",
	IsRegistrationFlowEnabled: false,
	ClientID:                  "apple_attestation_mobile_client",
	RedirectURIs:              []string{"myiosapp://callback"},
	AllowedUserTypes:          []string{"apple_attestation_guard_person"},
	// Attestation is a client-level setting, configured at the top level of the application
	// independent of the OAuth2 protocol config. Apple's config carries only non-secret identifiers.
	Attestation: map[string]interface{}{
		"apple": map[string]interface{}{
			"teamId":   "ABCDE12345",
			"bundleId": "com.example.myiosapp",
		},
	},
	InboundAuthConfig: []map[string]interface{}{
		{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId":                "apple_attestation_mobile_client",
				"redirectUris":            []string{"myiosapp://callback"},
				"grantTypes":              []string{"authorization_code"},
				"responseTypes":           []string{"code"},
				"tokenEndpointAuthMethod": "none",
				"publicClient":            true,
				"pkceRequired":            true,
			},
		},
	},
}

var (
	appleAttestationGuardOUID       string
	appleAttestationGuardUserTypeID string
	appleAttestationGuardFlowID     string
	appleAttestationMobileAppID     string
)

// AppleAttestationFlowTestSuite verifies the Apple App Attest guard on direct flow initiation.
type AppleAttestationFlowTestSuite struct {
	suite.Suite
	config *common.TestSuiteConfig
}

func TestAppleAttestationFlowTestSuite(t *testing.T) {
	suite.Run(t, new(AppleAttestationFlowTestSuite))
}

func (ts *AppleAttestationFlowTestSuite) SetupSuite() {
	ts.config = &common.TestSuiteConfig{}

	ouID, err := testutils.CreateOrganizationUnit(appleAttestationGuardOU)
	ts.Require().NoError(err, "failed to create OU")
	appleAttestationGuardOUID = ouID

	appleAttestationGuardUserType.OUID = appleAttestationGuardOUID
	schemaID, err := testutils.CreateUserType(appleAttestationGuardUserType)
	ts.Require().NoError(err, "failed to create user type")
	appleAttestationGuardUserTypeID = schemaID

	flowID, err := testutils.CreateFlow(appleAttestationGuardFlow)
	ts.Require().NoError(err, "failed to create auth flow")
	appleAttestationGuardFlowID = flowID
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)

	appleAttestationMobileApp.AuthFlowID = flowID
	appleAttestationMobileApp.OUID = appleAttestationGuardOUID
	appID, err := testutils.CreateApplication(appleAttestationMobileApp)
	ts.Require().NoError(err, "failed to create iOS application")
	appleAttestationMobileAppID = appID

	// A public / redirect-based mobile app is never issued a Flow Secret.
	ts.Require().Empty(testutils.GetFlowSecret(appleAttestationMobileAppID),
		"iOS app should not be issued a Flow Secret")
}

func (ts *AppleAttestationFlowTestSuite) TearDownSuite() {
	if appleAttestationMobileAppID != "" {
		if err := testutils.DeleteApplication(appleAttestationMobileAppID); err != nil {
			ts.T().Logf("failed to delete iOS application: %v", err)
		}
	}
	for _, id := range ts.config.CreatedFlowIDs {
		if err := testutils.DeleteFlow(id); err != nil {
			ts.T().Logf("failed to delete flow %s: %v", id, err)
		}
	}
	if appleAttestationGuardUserTypeID != "" {
		if err := testutils.DeleteUserType(appleAttestationGuardUserTypeID); err != nil {
			ts.T().Logf("failed to delete user type: %v", err)
		}
	}
	if appleAttestationGuardOUID != "" {
		if err := testutils.DeleteOrganizationUnit(appleAttestationGuardOUID); err != nil {
			ts.T().Logf("failed to delete OU: %v", err)
		}
	}
}

// executeNewFlowWithAttestation posts a new-flow INIT request, optionally presenting an attestation
// token via the Attestation-Token header, and returns the HTTP status code and parsed error body.
func (ts *AppleAttestationFlowTestSuite) executeNewFlowWithAttestation(
	body map[string]interface{}, attestationToken string) (int, *common.ErrorResponse) {
	reqBody, err := json.Marshal(body)
	ts.Require().NoError(err, "failed to marshal flow request")

	req, err := http.NewRequest("POST", testutils.TestServerURL+"/flow/execute", bytes.NewReader(reqBody))
	ts.Require().NoError(err, "failed to create flow request")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if attestationToken != "" {
		req.Header.Set("Attestation-Token", attestationToken)
	}

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err, "failed to send flow request")
	defer resp.Body.Close()

	var errResp common.ErrorResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&errResp),
		"flow error response should be valid JSON")
	return resp.StatusCode, &errResp
}

// An iOS app that omits the attestation token is rejected with 401 (FES-1014).
func (ts *AppleAttestationFlowTestSuite) TestIOSApp_MissingAttestationToken_Rejected() {
	status, errResp := ts.executeNewFlowWithAttestation(map[string]interface{}{
		"applicationId": appleAttestationMobileAppID,
		"flowType":      "AUTHENTICATION",
	}, "")

	ts.Require().Equal(http.StatusUnauthorized, status)
	ts.Require().Equal("FES-1014", errResp.Code)
}

// A malformed attestation object cannot be decoded. App Attest is verified offline, so this is a
// definitive token rejection (401, FES-1015), never a server error.
func (ts *AppleAttestationFlowTestSuite) TestIOSApp_MalformedAttestationToken_Rejected() {
	status, errResp := ts.executeNewFlowWithAttestation(map[string]interface{}{
		"applicationId": appleAttestationMobileAppID,
		"flowType":      "AUTHENTICATION",
	}, "not-a-valid-attestation-object")

	ts.Require().Equal(http.StatusUnauthorized, status)
	ts.Require().Equal("FES-1015", errResp.Code)
}
