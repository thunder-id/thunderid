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

// This suite verifies the platform-attestation guard for POST /flow/execute. A mobile
// (public, authorization_code) application that configures Google Play Integrity attestation may
// initiate a flow directly only by presenting a valid Attestation-Token header:
//   - a missing token is rejected with 401 (FES-1014);
//   - a token that cannot be verified because the outbound Play Integrity call cannot complete
//     (here, unusable service account credentials) surfaces as a 500 server error, not a 401 — a
//     provider/configuration failure must not be reported to the client as an auth failure.
//
// A definitive token rejection (identity mismatch → 401 FES-1015) and a successful verification both
// require a genuine Google-issued Play Integrity token and a real Google Cloud service account, so
// those paths are covered by unit tests and manual/E2E verification rather than here. To keep this
// suite deterministic and offline, the application is configured with unusable service account
// credentials so the Play Integrity API call can never succeed.

var attestationGuardOU = testutils.OrganizationUnit{
	Handle:      "attestation_guard_test_ou",
	Name:        "Attestation Guard Test OU",
	Description: "Organization unit for attestation guard flow testing",
	Parent:      nil,
}

var attestationGuardUserType = testutils.UserType{
	Name: "attestation_guard_person",
	Schema: map[string]interface{}{
		"username": map[string]interface{}{"type": "string"},
		"password": map[string]interface{}{"type": "string", "credential": true},
		"email":    map[string]interface{}{"type": "string"},
	},
}

var attestationGuardFlow = testutils.Flow{
	Name:     "Attestation Guard Auth Flow",
	FlowType: "AUTHENTICATION",
	Handle:   "auth_flow_attestation_guard",
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

// attestationMobileApp is a public, redirect-based (authorization_code) mobile application that
// configures Google Play Integrity attestation. Attestation takes precedence over the redirect
// classification, so the app may initiate a flow directly — but only with a valid attestation
// token. The service account credentials are intentionally unusable so verification always fails.
var attestationMobileApp = testutils.Application{
	Name:                      "Play Integrity Mobile App",
	Description:               "Mobile application for attestation guard testing",
	IsRegistrationFlowEnabled: false,
	ClientID:                  "attestation_mobile_client",
	RedirectURIs:              []string{"myapp://callback"},
	AllowedUserTypes:          []string{"attestation_guard_person"},
	// Attestation is a client-level setting, configured at the top level of the application
	// independent of the OAuth2 protocol config.
	Attestation: map[string]interface{}{
		"android": map[string]interface{}{
			"packageName":               "com.example.myapp",
			"certificateSha256Digests":  []string{"AA:BB:CC"},
			"serviceAccountCredentials": "not-a-valid-service-account",
		},
	},
	InboundAuthConfig: []map[string]interface{}{
		{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId":                "attestation_mobile_client",
				"redirectUris":            []string{"myapp://callback"},
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
	attestationGuardOUID       string
	attestationGuardUserTypeID string
	attestationGuardFlowID     string
	attestationMobileAppID     string
)

// AttestationFlowTestSuite verifies the attestation guard on direct flow initiation.
type AttestationFlowTestSuite struct {
	suite.Suite
	config *common.TestSuiteConfig
}

func TestAttestationFlowTestSuite(t *testing.T) {
	suite.Run(t, new(AttestationFlowTestSuite))
}

func (ts *AttestationFlowTestSuite) SetupSuite() {
	ts.config = &common.TestSuiteConfig{}

	ouID, err := testutils.CreateOrganizationUnit(attestationGuardOU)
	ts.Require().NoError(err, "failed to create OU")
	attestationGuardOUID = ouID

	attestationGuardUserType.OUID = attestationGuardOUID
	schemaID, err := testutils.CreateUserType(attestationGuardUserType)
	ts.Require().NoError(err, "failed to create user type")
	attestationGuardUserTypeID = schemaID

	flowID, err := testutils.CreateFlow(attestationGuardFlow)
	ts.Require().NoError(err, "failed to create auth flow")
	attestationGuardFlowID = flowID
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)

	attestationMobileApp.AuthFlowID = flowID
	attestationMobileApp.OUID = attestationGuardOUID
	appID, err := testutils.CreateApplication(attestationMobileApp)
	ts.Require().NoError(err, "failed to create mobile application")
	attestationMobileAppID = appID

	// A public / redirect-based mobile app is never issued a Flow Secret.
	ts.Require().Empty(testutils.GetFlowSecret(attestationMobileAppID),
		"mobile app should not be issued a Flow Secret")
}

func (ts *AttestationFlowTestSuite) TearDownSuite() {
	if attestationMobileAppID != "" {
		if err := testutils.DeleteApplication(attestationMobileAppID); err != nil {
			ts.T().Logf("failed to delete mobile application: %v", err)
		}
	}
	for _, id := range ts.config.CreatedFlowIDs {
		if err := testutils.DeleteFlow(id); err != nil {
			ts.T().Logf("failed to delete flow %s: %v", id, err)
		}
	}
	if attestationGuardUserTypeID != "" {
		if err := testutils.DeleteUserType(attestationGuardUserTypeID); err != nil {
			ts.T().Logf("failed to delete user type: %v", err)
		}
	}
	if attestationGuardOUID != "" {
		if err := testutils.DeleteOrganizationUnit(attestationGuardOUID); err != nil {
			ts.T().Logf("failed to delete OU: %v", err)
		}
	}
}

// executeNewFlowWithAttestation posts a new-flow INIT request, optionally presenting an
// attestation token via the Attestation-Token header, and returns the HTTP status code and parsed
// error body.
func (ts *AttestationFlowTestSuite) executeNewFlowWithAttestation(
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

// A mobile app that omits the attestation token is rejected with 401 (FES-1014).
func (ts *AttestationFlowTestSuite) TestMobileApp_MissingAttestationToken_Rejected() {
	status, errResp := ts.executeNewFlowWithAttestation(map[string]interface{}{
		"applicationId": attestationMobileAppID,
		"flowType":      "AUTHENTICATION",
	}, "")

	ts.Require().Equal(http.StatusUnauthorized, status)
	ts.Require().Equal("FES-1014", errResp.Code)
}

// A mobile app whose stored service account credentials cannot reach Google's Play Integrity API
// cannot complete verification. This is a server-side condition rather than a rejected token, so it
// surfaces as a 500 server error rather than a 401.
func (ts *AttestationFlowTestSuite) TestMobileApp_AttestationVerificationUnavailable_ServerError() {
	status, errResp := ts.executeNewFlowWithAttestation(map[string]interface{}{
		"applicationId": attestationMobileAppID,
		"flowType":      "AUTHENTICATION",
	}, "some-play-integrity-token")

	ts.Require().Equal(http.StatusInternalServerError, status)
	ts.Require().Equal("SSE-5000", errResp.Code)
}
