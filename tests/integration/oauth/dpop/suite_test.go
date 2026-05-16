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

// Package dpop holds end-to-end integration tests for DPoP support.
//
// These tests run against the live test server (see testutils.TestServerURL)
// and exercise the same observable HTTP behaviour an external client would
// see — they intentionally do not import the server-side dpop package, so
// the assertions are independent of the implementation's internal helpers.
package dpop

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

const (
	// Voluntary client — DPoP is not required, used to exercise the
	// token/auth-code/resource binding flows.
	voluntaryClientID     = "dpop_voluntary_client_123"
	voluntaryClientSecret = "dpop_voluntary_secret_123"
	voluntaryAppName      = "DPoPVoluntaryTestApp"
	// Enforced client — dpopBoundAccessTokens=true, used for the
	// per-client enforcement flow.
	enforcedClientID     = "dpop_enforced_client_456"
	enforcedClientSecret = "dpop_enforced_secret_456"
	enforcedAppName      = "DPoPEnforcedTestApp"

	dpopRedirectURI = "https://localhost:3000"

	// Endpoint URLs — these MUST equal the canonicalized form the server uses
	// for DPoP htu comparison. The server canonicalizes, so as long as we send
	// a syntactically equivalent URL we are fine.
	tokenEndpoint    = testutils.TestServerURL + "/oauth2/token"
	parEndpoint      = testutils.TestServerURL + "/oauth2/par"
	userInfoEndpoint = testutils.TestServerURL + "/oauth2/userinfo"
	authzEndpoint    = testutils.TestServerURL + "/oauth2/authorize"

	// Test user
	dpopTestUsername = "dpop_test_user"
	dpopTestPassword = "DPoPTest123!"
)

var (
	dpopTestUserSchema = testutils.UserType{
		Name: "dpop-test-person",
		Schema: map[string]any{
			"username": map[string]any{"type": "string"},
			"password": map[string]any{"type": "string", "credential": true},
			"email":    map[string]any{"type": "string"},
		},
	}

	dpopTestOU = testutils.OrganizationUnit{
		Handle:      "dpop-test-ou",
		Name:        "DPoP Test OU",
		Description: "Organization unit for DPoP integration testing",
		Parent:      nil,
	}

	dpopTestAuthFlow = testutils.Flow{
		Name:     "DPoP Test Auth Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_dpop_test",
		Nodes: []map[string]any{
			{
				"id":        "start",
				"type":      "START",
				"onSuccess": "prompt_credentials",
			},
			{
				"id":   "prompt_credentials",
				"type": "PROMPT",
				"prompts": []map[string]any{
					{
						"inputs": []map[string]any{
							{"ref": "input_001", "identifier": "username", "type": "TEXT_INPUT", "required": true},
							{"ref": "input_002", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
						},
						"action": map[string]any{"ref": "action_001", "nextNode": "basic_auth"},
					},
				},
			},
			{
				"id":   "basic_auth",
				"type": "TASK_EXECUTION",
				"executor": map[string]any{
					"name": "BasicAuthExecutor",
					"inputs": []map[string]any{
						{"ref": "input_001", "identifier": "username", "type": "TEXT_INPUT", "required": true},
						{"ref": "input_002", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
					},
				},
				"onSuccess": "auth_assert",
			},
			{
				"id":        "auth_assert",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]any{"name": "AuthAssertExecutor"},
				"onSuccess": "end",
			},
			{"id": "end", "type": "END"},
		},
	}
)

// DPoPTestSuite is the umbrella test suite shared by every phase's test file.
type DPoPTestSuite struct {
	suite.Suite
	ouID             string
	userSchemaID     string
	authFlowID       string
	userID           string
	voluntaryAppID   string
	enforcedAppID    string
	client           *http.Client
}

// TestDPoPTestSuite is the single entrypoint that runs every Test* method
// declared on the suite across all phase files in this package.
func TestDPoPTestSuite(t *testing.T) {
	suite.Run(t, new(DPoPTestSuite))
}

func (ts *DPoPTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()

	ouID, err := testutils.CreateOrganizationUnit(dpopTestOU)
	ts.Require().NoError(err, "create test OU")
	ts.ouID = ouID

	dpopTestUserSchema.OUID = ouID
	schemaID, err := testutils.CreateUserType(dpopTestUserSchema)
	ts.Require().NoError(err, "create test user schema")
	ts.userSchemaID = schemaID

	flowID, err := testutils.CreateFlow(dpopTestAuthFlow)
	ts.Require().NoError(err, "create auth flow")
	ts.authFlowID = flowID

	user := testutils.User{
		OUID: ts.ouID,
		Type: "dpop-test-person",
		Attributes: json.RawMessage(fmt.Sprintf(`{
			"username": %q,
			"password": %q,
			"email": "dpop_test_user@example.com"
		}`, dpopTestUsername, dpopTestPassword)),
	}
	userID, err := testutils.CreateUser(user)
	ts.Require().NoError(err, "create test user")
	ts.userID = userID

	ts.voluntaryAppID = ts.createApp(voluntaryAppName, voluntaryClientID, voluntaryClientSecret, false)
	ts.enforcedAppID = ts.createApp(enforcedAppName, enforcedClientID, enforcedClientSecret, true)
}

func (ts *DPoPTestSuite) TearDownSuite() {
	if ts.voluntaryAppID != "" {
		_ = testutils.DeleteApplication(ts.voluntaryAppID)
	}
	if ts.enforcedAppID != "" {
		_ = testutils.DeleteApplication(ts.enforcedAppID)
	}
	if ts.userID != "" {
		_ = testutils.DeleteUser(ts.userID)
	}
	if ts.authFlowID != "" {
		_ = testutils.DeleteFlow(ts.authFlowID)
	}
	if ts.userSchemaID != "" {
		_ = testutils.DeleteUserType(ts.userSchemaID)
	}
	if ts.ouID != "" {
		_ = testutils.DeleteOrganizationUnit(ts.ouID)
	}
}

// createApp creates an OAuth2 application bound to the shared flow + OU.
// Setting dpopBound=true exercises per-client DPoP enforcement at /oauth2/token.
func (ts *DPoPTestSuite) createApp(name, clientID, clientSecret string, dpopBound bool) string {
	app := map[string]any{
		"name":                      name,
		"description":               "DPoP integration test app",
		"ouId":                      ts.ouID,
		"authFlowId":                ts.authFlowID,
		"isRegistrationFlowEnabled": false,
		"allowedUserTypes":          []string{"dpop-test-person"},
		"inboundAuthConfig": []map[string]any{
			{
				"type": "oauth2",
				"config": map[string]any{
					"clientId":                clientID,
					"clientSecret":            clientSecret,
					"redirectUris":            []string{dpopRedirectURI},
					"grantTypes":              []string{"authorization_code", "refresh_token"},
					"responseTypes":           []string{"code"},
					"tokenEndpointAuthMethod": "client_secret_basic",
					"scopes":                  []string{"openid", "profile", "email"},
					"dpopBoundAccessTokens":   dpopBound,
					"userInfo": map[string]any{
						"responseType":   "JSON",
						"userAttributes": []string{"email"},
					},
					"scopeClaims": map[string][]string{
						"profile": {"given_name", "family_name", "name"},
						"email":   {"email"},
					},
				},
			},
		},
	}

	jsonData, err := json.Marshal(app)
	ts.Require().NoError(err)

	req, err := http.NewRequest("POST", testutils.TestServerURL+"/applications", bytes.NewBuffer(jsonData))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	ts.Require().Equalf(http.StatusCreated, resp.StatusCode, "create app failed: %s", string(body))

	var respData map[string]any
	ts.Require().NoError(json.Unmarshal(body, &respData))
	id, _ := respData["id"].(string)
	ts.Require().NotEmpty(id, "application id missing in response")
	return id
}

// obtainAuthorizationCode runs the shared OU/flow login and returns
// (code, codeVerifier). The caller is responsible for exchanging the code at
// /oauth2/token (with or without DPoP).
//
// extraAuthzParams is merged into the /authorize query string; it is intended
// for the dpop_jkt parameter but tests can use it for anything.
func (ts *DPoPTestSuite) obtainAuthorizationCode(
	clientID string, extraAuthzParams map[string]string,
) (code, codeVerifier string) {
	verifier, err := testutils.GenerateCodeVerifier()
	ts.Require().NoError(err, "generate PKCE verifier")
	challenge := testutils.GenerateCodeChallenge(verifier)

	params := url.Values{}
	params.Set("client_id", clientID)
	params.Set("redirect_uri", dpopRedirectURI)
	params.Set("response_type", "code")
	params.Set("scope", "openid email")
	params.Set("state", "dpop-test-state")
	params.Set("code_challenge", challenge)
	params.Set("code_challenge_method", "S256")
	for k, v := range extraAuthzParams {
		params.Set(k, v)
	}

	noRedirect := testutils.GetNoRedirectHTTPClient()
	req, err := http.NewRequest("GET", authzEndpoint+"?"+params.Encode(), nil)
	ts.Require().NoError(err)
	resp, err := noRedirect.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusFound, resp.StatusCode)
	location := resp.Header.Get("Location")

	authID, executionID, err := testutils.ExtractAuthData(location)
	ts.Require().NoError(err)

	initial, err := testutils.ExecuteAuthenticationFlow(executionID, nil, "")
	ts.Require().NoError(err)

	step, err := testutils.ExecuteAuthenticationFlow(executionID, map[string]string{
		"username": dpopTestUsername,
		"password": dpopTestPassword,
	}, "action_001", initial.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("COMPLETE", step.FlowStatus, "auth flow should complete")

	authzResp, err := testutils.CompleteAuthorization(authID, step.Assertion)
	ts.Require().NoError(err)

	code, err = testutils.ExtractAuthorizationCode(authzResp.RedirectURI)
	ts.Require().NoError(err)
	return code, verifier
}

// obtainAuthorizationCodeViaPAR pushes a PAR request (optionally with a DPoP
// header) and follows the request_uri through the auth flow to the code.
// dpopProof may be empty.
func (ts *DPoPTestSuite) obtainAuthorizationCodeViaPAR(
	clientID, clientSecret, dpopProof string,
) (code, codeVerifier string) {
	verifier, err := testutils.GenerateCodeVerifier()
	ts.Require().NoError(err)
	challenge := testutils.GenerateCodeChallenge(verifier)

	parResult, err := submitPARWithDPoP(clientID, clientSecret, dpopProof, map[string]string{
		"response_type":         "code",
		"redirect_uri":          dpopRedirectURI,
		"scope":                 "openid email",
		"state":                 "dpop-par-state",
		"code_challenge":        challenge,
		"code_challenge_method": "S256",
	})
	ts.Require().NoError(err)
	ts.Require().Equalf(http.StatusCreated, parResult.StatusCode,
		"PAR submission expected 201, got %d body=%s", parResult.StatusCode, string(parResult.Body))
	ts.Require().NotNil(parResult.PAR)

	noRedirect := testutils.GetNoRedirectHTTPClient()
	q := url.Values{}
	q.Set("client_id", clientID)
	q.Set("request_uri", parResult.PAR.RequestURI)
	req, err := http.NewRequest("GET", authzEndpoint+"?"+q.Encode(), nil)
	ts.Require().NoError(err)
	resp, err := noRedirect.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusFound, resp.StatusCode)

	authID, executionID, err := testutils.ExtractAuthData(resp.Header.Get("Location"))
	ts.Require().NoError(err)

	initial, err := testutils.ExecuteAuthenticationFlow(executionID, nil, "")
	ts.Require().NoError(err)
	step, err := testutils.ExecuteAuthenticationFlow(executionID, map[string]string{
		"username": dpopTestUsername,
		"password": dpopTestPassword,
	}, "action_001", initial.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("COMPLETE", step.FlowStatus)

	authzResp, err := testutils.CompleteAuthorization(authID, step.Assertion)
	ts.Require().NoError(err)
	code, err = testutils.ExtractAuthorizationCode(authzResp.RedirectURI)
	ts.Require().NoError(err)
	return code, verifier
}
