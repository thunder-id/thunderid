// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authz

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	cbNegClientID     = "authz_callback_neg_client"
	cbNegClientSecret = "authz_callback_neg_secret" //nolint:gosec // test credential
	cbNegRedirectURI  = "https://localhost:3000"
	cbNegUsername     = "callback_neg_user"
	cbNegPassword     = "testpass123"
	cbNegUserType     = "authz-callback-neg-person"

	// callbackPath is the single flow-callback endpoint every grant type is dispatched through.
	callbackPath = "/oauth2/auth/callback"
)

// CallbackNegativeTestSuite covers the rejection paths of the flow-callback dispatcher, the endpoint
// the Gate UI posts a terminal flow assertion to. Its happy path is exercised by every other
// authorization-code suite, so what is pinned here is the opposite: requests that are structurally
// invalid, name an unknown callback type, carry an assertion the server did not sign, or replay an
// authId that was already spent. None of them may yield an authorization code.
type CallbackNegativeTestSuite struct {
	suite.Suite
	ouID         string
	entityTypeID string
	userID       string
	flowID       string
	appID        string
	client       *http.Client
}

func TestCallbackNegativeTestSuite(t *testing.T) {
	suite.Run(t, new(CallbackNegativeTestSuite))
}

func (ts *CallbackNegativeTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "authz-callback-neg-ou",
		Name:        "Authz Callback Negative OU",
		Description: "Organization unit for the flow callback dispatcher negative tests",
		Parent:      nil,
	})
	ts.Require().NoError(err, "Failed to create test organization unit")
	ts.ouID = ouID

	entityTypeID, err := testutils.CreateUserType(testutils.UserType{
		Name: cbNegUserType,
		OUID: ouID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
			"password": map[string]interface{}{"type": "string", "credential": true},
			"email":    map[string]interface{}{"type": "string"},
		},
	})
	ts.Require().NoError(err, "Failed to create test user type")
	ts.entityTypeID = entityTypeID

	userID, err := testutils.CreateUser(testutils.User{
		OUID: ouID,
		Type: cbNegUserType,
		Attributes: json.RawMessage(fmt.Sprintf(`{
			"username": "%s",
			"password": "%s",
			"email": "%s@example.com"
		}`, cbNegUsername, cbNegPassword, cbNegUsername)),
	})
	ts.Require().NoError(err, "Failed to create test user")
	ts.userID = userID

	flowID, err := testutils.CreateFlow(testutils.Flow{
		Name:     "Authz Callback Negative Auth Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_authz_callback_negative",
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "prompt_credentials"},
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
				"onSuccess":    "authorization_check",
				"onIncomplete": "prompt_credentials",
			},
			{
				"id":        "authorization_check",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "AuthorizationExecutor"},
				"onSuccess": "auth_assert",
			},
			{
				"id":        "auth_assert",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "AuthAssertExecutor"},
				"onSuccess": "end",
			},
			{"id": "end", "type": "END"},
		},
	})
	ts.Require().NoError(err, "Failed to create callback negative flow")
	ts.flowID = flowID

	ts.appID = ts.createCallbackNegativeApplication(flowID)
}

func (ts *CallbackNegativeTestSuite) TearDownSuite() {
	if ts.appID != "" {
		_ = testutils.DeleteApplication(ts.appID)
	}
	if ts.flowID != "" {
		_ = testutils.DeleteFlow(ts.flowID)
	}
	if ts.userID != "" {
		if err := testutils.DeleteUser(ts.userID); err != nil {
			ts.T().Logf("Failed to delete test user: %v", err)
		}
	}
	if ts.entityTypeID != "" {
		if err := testutils.DeleteUserType(ts.entityTypeID); err != nil {
			ts.T().Logf("Failed to delete test user type: %v", err)
		}
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete test organization unit: %v", err)
		}
	}
}

// TestCallback_MissingAuthIDIsRejected verifies the dispatcher rejects a body with no authId before
// it looks at anything else, so an assertion can never be processed without a request to bind it to.
func (ts *CallbackNegativeTestSuite) TestCallback_MissingAuthIDIsRejected() {
	status, body := ts.postCallbackJSON(map[string]interface{}{"assertion": "some.assertion.value"})

	ts.Require().Equal(http.StatusBadRequest, status, "a body without authId must be rejected")
	ts.Equal("invalid_request", body["error"])
	ts.Equal("authId is required", body["error_description"])
}

// TestCallback_MissingAssertionIsRejected verifies that an authId alone is not enough: without an
// assertion there is nothing signed to authorize the request, so it is rejected.
func (ts *CallbackNegativeTestSuite) TestCallback_MissingAssertionIsRejected() {
	status, body := ts.postCallbackJSON(map[string]interface{}{"authId": "some-auth-id"})

	ts.Require().Equal(http.StatusBadRequest, status, "a body without an assertion must be rejected")
	ts.Equal("invalid_request", body["error"])
	ts.Equal("assertion is required", body["error_description"])
}

// TestCallback_MalformedJSONBodyIsRejected verifies that a body which is not valid JSON is reported as
// an invalid request rather than surfacing a decoder failure as a server error.
func (ts *CallbackNegativeTestSuite) TestCallback_MalformedJSONBodyIsRejected() {
	status, body := ts.postCallbackRaw([]byte(`{"authId": "abc", "assertion":`))

	ts.Require().Equal(http.StatusBadRequest, status, "an unparseable body must be rejected")
	ts.Equal("invalid_request", body["error"])
	ts.NotEmpty(body["error_description"], "the rejection should describe why the body was refused")
}

// TestCallback_UnsupportedTypeIsRejected verifies that the dispatcher only routes callback types it
// has a handler for. An unknown type falls through to the default branch instead of being silently
// treated as the authorization_code default.
func (ts *CallbackNegativeTestSuite) TestCallback_UnsupportedTypeIsRejected() {
	status, body := ts.postCallbackJSON(map[string]interface{}{
		"authId":    "some-auth-id",
		"assertion": "some.assertion.value",
		"type":      "urn:unsupported",
	})

	ts.Require().Equal(http.StatusBadRequest, status, "an unknown callback type must be rejected")
	ts.Equal("invalid_request", body["error"])
	ts.Equal("Unsupported callback type", body["error_description"])
}

// TestCallback_ForeignSignedAssertionIsRejected verifies that a well-formed assertion carrying the
// right claims but signed with a key the server does not trust is refused. Signature verification
// runs before the authorization request is loaded, so no authorization code is issued.
func (ts *CallbackNegativeTestSuite) TestCallback_ForeignSignedAssertionIsRejected() {
	authID, _ := ts.initiateAuthorization("cb_neg_foreign_state")

	forged := ts.signForeignAssertion(authID)

	status, body := ts.postCallbackJSON(map[string]interface{}{"authId": authID, "assertion": forged})
	ts.Require().Equal(http.StatusOK, status, "the dispatcher answers with a redirect target: %v", body)

	redirect, ok := body["redirect_uri"].(string)
	ts.Require().True(ok, "the response should carry a redirect target, got %v", body)

	query := ts.redirectQuery(redirect)
	ts.Equal("invalid_request", query.Get("errorCode"),
		"an assertion the server did not sign must be reported as an invalid request")
	ts.Empty(query.Get("code"), "no authorization code may be issued for an untrusted assertion")
}

// TestCallback_ReplayedAuthIDIsRejected verifies that an authId is single-use: once it has been
// exchanged for an authorization code, posting the same authId and assertion again is refused and
// yields no second code.
func (ts *CallbackNegativeTestSuite) TestCallback_ReplayedAuthIDIsRejected() {
	authID, executionID := ts.initiateAuthorization("cb_neg_replay_state")

	initialStep, err := testutils.ExecuteAuthenticationFlow(executionID, nil, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")

	step, err := testutils.ExecuteAuthenticationFlow(executionID,
		map[string]string{"username": cbNegUsername, "password": cbNegPassword},
		"action_001", initialStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to complete authentication flow")
	ts.Require().Equal("COMPLETE", step.FlowStatus, "credential login should complete the flow")
	ts.Require().NotEmpty(step.Assertion, "login should yield an assertion")

	// First callback: the genuine exchange, which spends the authId.
	first, err := testutils.CompleteAuthorization(authID, step.Assertion)
	ts.Require().NoError(err, "the first callback should succeed")
	code, err := testutils.ExtractAuthorizationCode(first.RedirectURI)
	ts.Require().NoError(err, "the first callback should issue an authorization code")
	ts.Require().NotEmpty(code)

	// Second callback: the exact same request replayed.
	status, body := ts.postCallbackJSON(map[string]interface{}{"authId": authID, "assertion": step.Assertion})
	ts.Require().Equal(http.StatusOK, status, "the dispatcher answers with a redirect target: %v", body)

	redirect, ok := body["redirect_uri"].(string)
	ts.Require().True(ok, "the response should carry a redirect target, got %v", body)

	query := ts.redirectQuery(redirect)
	ts.Equal("invalid_request", query.Get("errorCode"),
		"a spent authId must be reported as an invalid authorization request")
	ts.Empty(query.Get("code"), "replaying a spent authId must not issue a second authorization code")
}

// initiateAuthorization starts an authorization code request and returns the authId and executionId
// handed to the gate.
func (ts *CallbackNegativeTestSuite) initiateAuthorization(state string) (string, string) {
	resp, err := testutils.InitiateAuthorizationFlow(cbNegClientID, cbNegRedirectURI, "code", "openid", state)
	ts.Require().NoError(err, "Failed to initiate authorization flow")
	defer resp.Body.Close()

	ts.Require().Equal(http.StatusFound, resp.StatusCode, "authorize should redirect to the gate")
	authID, executionID, err := testutils.ExtractAuthData(resp.Header.Get("Location"))
	ts.Require().NoError(err, "Failed to extract auth data from redirect")
	return authID, executionID
}

// signForeignAssertion mints an RS256 JWT with the claim shape of a genuine authentication assertion,
// signed by a throwaway key that is not in the server's trust set.
func (ts *CallbackNegativeTestSuite) signForeignAssertion(authID string) string {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	ts.Require().NoError(err, "Failed to generate the foreign signing key")

	now := time.Now().UTC()
	header := map[string]interface{}{"alg": "RS256", "typ": "JWT", "kid": "foreign-key-1"}
	claims := map[string]interface{}{
		"sub":                      ts.userID,
		"aud":                      cbNegClientID,
		"iss":                      "https://foreign-issuer.example.com",
		"iat":                      now.Unix(),
		"nbf":                      now.Unix(),
		"exp":                      now.Add(5 * time.Minute).Unix(),
		"authorization_request_id": authID,
		"auth_time":                now.Unix(),
	}

	headerJSON, err := json.Marshal(header)
	ts.Require().NoError(err)
	claimsJSON, err := json.Marshal(claims)
	ts.Require().NoError(err)

	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." +
		base64.RawURLEncoding.EncodeToString(claimsJSON)
	digest := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	ts.Require().NoError(err, "Failed to sign the foreign assertion")

	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)
}

// redirectQuery parses the query parameters of a redirect target returned by the dispatcher.
func (ts *CallbackNegativeTestSuite) redirectQuery(redirectURI string) url.Values {
	parsed, err := url.Parse(redirectURI)
	ts.Require().NoError(err, "Failed to parse the redirect target")
	return parsed.Query()
}

// postCallbackJSON posts a JSON-encoded body to the flow callback endpoint.
func (ts *CallbackNegativeTestSuite) postCallbackJSON(payload map[string]interface{}) (
	int, map[string]interface{}) {
	raw, err := json.Marshal(payload)
	ts.Require().NoError(err, "Failed to marshal the callback body")
	return ts.postCallbackRaw(raw)
}

// postCallbackRaw posts an arbitrary byte body to the flow callback endpoint and returns the status
// along with the decoded response. A plain client is used so the request carries exactly what the
// test sets, matching what the Gate UI would send.
func (ts *CallbackNegativeTestSuite) postCallbackRaw(raw []byte) (int, map[string]interface{}) {
	req, err := http.NewRequest("POST", testutils.TestServerURL+callbackPath, bytes.NewReader(raw))
	ts.Require().NoError(err, "Failed to build the callback request")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout:   30 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	}
	resp, err := client.Do(req)
	ts.Require().NoError(err, "Callback request failed")
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err, "Failed to read the callback response")

	var decoded map[string]interface{}
	ts.Require().NoError(json.Unmarshal(respBody, &decoded),
		"Failed to decode the callback response %q", string(respBody))
	return resp.StatusCode, decoded
}

// createCallbackNegativeApplication registers the OAuth application bound to the callback flow.
func (ts *CallbackNegativeTestSuite) createCallbackNegativeApplication(authFlowID string) string {
	app := map[string]interface{}{
		"name":                      "AuthzCallbackNegativeApp",
		"description":               "Application for the flow callback dispatcher negative tests",
		"ouId":                      ts.ouID,
		"type":                      "browser",
		"authFlowId":                authFlowID,
		"isRegistrationFlowEnabled": false,
		"allowedUserTypes":          []string{cbNegUserType},
		"inboundAuthConfig": []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                cbNegClientID,
					"clientSecret":            cbNegClientSecret,
					"redirectUris":            []string{cbNegRedirectURI},
					"grantTypes":              []string{"authorization_code"},
					"responseTypes":           []string{"code"},
					"tokenEndpointAuthMethod": "client_secret_basic",
					"scopes":                  []string{"openid"},
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

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		ts.T().Fatalf("Failed to create application. Status: %d, Response: %s",
			resp.StatusCode, string(bodyBytes))
	}

	var respData map[string]interface{}
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&respData))
	return respData["id"].(string)
}
