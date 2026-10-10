// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package token

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	rtBranchClientID       = "rt_branches_test_client"
	rtBranchClientSecret   = "rt_branches_test_secret"
	rtBranchPublicClientID = "rt_branches_public_client"
	rtBranchRedirectURI    = "https://localhost:3000"
	rtBranchUsername       = "rt_branches_test_user"
	rtBranchPassword       = "RtBranchesPass1!"
	rtBranchResourceA      = "https://rt-branches-a.example.com"
	rtBranchResourceB      = "https://rt-branches-b.example.com"
)

var rtBranchUserType = testutils.UserType{
	Handle:      "rt-branches-person",
	DisplayName: "Rt Branches Person",
	Schema: map[string]interface{}{
		"username": map[string]interface{}{"type": "string"},
		"password": map[string]interface{}{"type": "string", "credential": true},
	},
}

var rtBranchAuthFlow = testutils.Flow{
	Name:     "Refresh Token Branches Auth Flow",
	FlowType: "AUTHENTICATION",
	Handle:   "auth_flow_rt_branches_test",
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
}

// RefreshTokenBranchesTestSuite covers refresh-token grant branches beyond the happy path already
// proven elsewhere: rejecting a resource parameter that does not match the token's bound audience,
// refresh-token rotation and replay-detection (oauth.refresh_token.renew_on_grant is enabled
// server-wide by default), and DPoP binding continuity across refresh for a public client.
//
// Reuse with rotation explicitly disabled is not covered here: oauth.refresh_token.renew_on_grant is
// a plain bool (unlike its sibling settings, which use *bool for exactly this reason), so the config
// merge cannot distinguish an explicit `false` in deployment.yaml from "not set" and silently keeps
// the default enabled. A test exercising renew_on_grant: false would currently just document that
// bug rather than the intended reuse behavior.
type RefreshTokenBranchesTestSuite struct {
	suite.Suite
	client            *http.Client
	ouID              string
	entityTypeID      string
	authFlowID        string
	appID             string
	publicAppID       string
	resourceServerAID string
	resourceServerBID string
	userID            string
}

// TestRefreshTokenBranchesTestSuite runs the RefreshTokenBranchesTestSuite.
func TestRefreshTokenBranchesTestSuite(t *testing.T) {
	suite.Run(t, new(RefreshTokenBranchesTestSuite))
}

// SetupSuite creates the shared organization unit, user type, auth flow, two resource servers, a
// confidential app, a public (PKCE) app, and the test user.
func (ts *RefreshTokenBranchesTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "rt-branches-test-ou",
		Name:        "Refresh Token Branches Test OU",
		Description: "Organization unit for refresh-token grant branch integration tests",
	})
	ts.Require().NoError(err)
	ts.ouID = ouID

	rtBranchUserType.OUID = ouID
	entityTypeID, err := testutils.CreateUserType(rtBranchUserType)
	ts.Require().NoError(err)
	ts.entityTypeID = entityTypeID

	flowID, err := testutils.CreateFlow(rtBranchAuthFlow)
	ts.Require().NoError(err)
	ts.authFlowID = flowID

	rsAID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:       "Refresh Token Branches RS A",
		Identifier: rtBranchResourceA,
		OUID:       ts.ouID,
	}, []testutils.Action{})
	ts.Require().NoError(err)
	ts.resourceServerAID = rsAID

	rsBID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:       "Refresh Token Branches RS B",
		Identifier: rtBranchResourceB,
		OUID:       ts.ouID,
	}, []testutils.Action{})
	ts.Require().NoError(err)
	ts.resourceServerBID = rsBID

	ts.appID = ts.createApplication(map[string]interface{}{
		"clientId":                rtBranchClientID,
		"clientSecret":            rtBranchClientSecret,
		"redirectUris":            []string{rtBranchRedirectURI},
		"grantTypes":              []string{"authorization_code", "refresh_token"},
		"responseTypes":           []string{"code"},
		"tokenEndpointAuthMethod": "client_secret_basic",
	})

	ts.publicAppID = ts.createApplication(map[string]interface{}{
		"clientId":                rtBranchPublicClientID,
		"redirectUris":            []string{rtBranchRedirectURI},
		"grantTypes":              []string{"authorization_code", "refresh_token"},
		"responseTypes":           []string{"code"},
		"tokenEndpointAuthMethod": "none",
		"publicClient":            true,
		"pkceRequired":            true,
	})

	attributesJSON, err := json.Marshal(map[string]interface{}{
		"username": rtBranchUsername,
		"password": rtBranchPassword,
	})
	ts.Require().NoError(err)
	userID, err := testutils.CreateUser(testutils.User{
		OUID:       ouID,
		Type:       "rt-branches-person",
		Attributes: json.RawMessage(attributesJSON),
	})
	ts.Require().NoError(err)
	ts.userID = userID
}

// TearDownSuite deletes the resources created in SetupSuite.
func (ts *RefreshTokenBranchesTestSuite) TearDownSuite() {
	if ts.userID != "" {
		_ = testutils.DeleteUser(ts.userID)
	}
	if ts.publicAppID != "" {
		_ = testutils.DeleteApplication(ts.publicAppID)
	}
	if ts.appID != "" {
		_ = testutils.DeleteApplication(ts.appID)
	}
	if ts.resourceServerBID != "" {
		_ = testutils.DeleteResourceServer(ts.resourceServerBID)
	}
	if ts.resourceServerAID != "" {
		_ = testutils.DeleteResourceServer(ts.resourceServerAID)
	}
	if ts.authFlowID != "" {
		_ = testutils.DeleteFlow(ts.authFlowID)
	}
	if ts.entityTypeID != "" {
		_ = testutils.DeleteUserType(ts.entityTypeID)
	}
	if ts.ouID != "" {
		_ = testutils.DeleteOrganizationUnit(ts.ouID)
	}
}

// createApplication creates an application bound to the shared auth flow with the given OAuth2
// inbound config.
func (ts *RefreshTokenBranchesTestSuite) createApplication(oauthConfig map[string]interface{}) string {
	clientID, _ := oauthConfig["clientId"].(string)
	app := map[string]interface{}{
		"name":                      "RefreshTokenBranchesApp-" + clientID,
		"description":               "Application for refresh-token grant branch integration testing",
		"ouId":                      ts.ouID,
		"type":                      "fullstack",
		"authFlowId":                ts.authFlowID,
		"isRegistrationFlowEnabled": false,
		"allowedUserTypes":          []string{"rt-branches-person"},
		"inboundAuthConfig": []map[string]interface{}{
			{"type": "oauth2", "config": oauthConfig},
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

	bodyBytes, _ := io.ReadAll(resp.Body)
	ts.Require().Equal(http.StatusCreated, resp.StatusCode, string(bodyBytes))

	var respData map[string]interface{}
	ts.Require().NoError(json.Unmarshal(bodyBytes, &respData))
	return respData["id"].(string)
}

// obtainTokensForResource runs the confidential app's authorization_code flow, requesting the given
// resource, and returns the resulting token response.
func (ts *RefreshTokenBranchesTestSuite) obtainTokensForResource(resource string) *testutils.TokenHTTPResult {
	resp, err := testutils.InitiateAuthorizationFlowWithResource(
		rtBranchClientID, rtBranchRedirectURI, "code", "openid", "test-state", resource)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusFound, resp.StatusCode)

	authID, executionID, err := testutils.ExtractAuthData(resp.Header.Get("Location"))
	ts.Require().NoError(err)

	initialStep, err := testutils.ExecuteAuthenticationFlow(executionID, nil, "")
	ts.Require().NoError(err)

	flowStep, err := testutils.ExecuteAuthenticationFlow(executionID, map[string]string{
		"username": rtBranchUsername,
		"password": rtBranchPassword,
	}, "action_001", initialStep.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus)

	authzResp, err := testutils.CompleteAuthorization(authID, flowStep.Assertion)
	ts.Require().NoError(err)

	code, err := testutils.ExtractAuthorizationCode(authzResp.RedirectURI)
	ts.Require().NoError(err)

	tokenResult, err := testutils.RequestTokenWithResource(
		rtBranchClientID, rtBranchClientSecret, code, rtBranchRedirectURI, "authorization_code", resource)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, tokenResult.StatusCode, string(tokenResult.Body))
	ts.Require().NotNil(tokenResult.Token)
	ts.Require().NotEmpty(tokenResult.Token.RefreshToken)

	return tokenResult
}

// refreshWithResource submits a refresh_token grant request, optionally scoping it to a resource
// parameter, and returns the parsed response.
func (ts *RefreshTokenBranchesTestSuite) refreshWithResource(refreshToken, resource string) (int, map[string]interface{}) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	if resource != "" {
		form.Set("resource", resource)
	}

	req, err := http.NewRequest("POST", testutils.TestServerURL+"/oauth2/token",
		bytes.NewBufferString(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(rtBranchClientID, rtBranchClientSecret)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	var body map[string]interface{}
	bodyBytes, _ := io.ReadAll(resp.Body)
	ts.Require().NoError(json.Unmarshal(bodyBytes, &body), "body: %s", string(bodyBytes))
	return resp.StatusCode, body
}

// TestRefreshToken_ResourceMismatch_InvalidTarget verifies that requesting a resource different from
// the one the refresh token is bound to is rejected as invalid_target, and the mismatched resource
// server's existence is irrelevant to the outcome.
func (ts *RefreshTokenBranchesTestSuite) TestRefreshToken_ResourceMismatch_InvalidTarget() {
	tokenResult := ts.obtainTokensForResource(rtBranchResourceA)

	status, body := ts.refreshWithResource(tokenResult.Token.RefreshToken, rtBranchResourceB)
	ts.Require().Equal(http.StatusBadRequest, status, "%v", body)
	ts.Equal("invalid_target", body["error"])
	ts.Contains(body["error_description"], "does not match the refresh token audience")
}

// TestRefreshToken_RotationAndReplayDetection verifies that with renew_on_grant enabled (the
// server-wide default), a refresh consumes and revokes the presented refresh token, issuing a new one
// in its place (rotation); and that replaying the now-revoked original token is rejected as
// invalid_grant and additionally revokes the whole token family, so the freshly rotated token dies too
// (RFC 9700 4.14.2).
func (ts *RefreshTokenBranchesTestSuite) TestRefreshToken_RotationAndReplayDetection() {
	tokenResult := ts.obtainTokensForResource(rtBranchResourceA)
	originalRefreshToken := tokenResult.Token.RefreshToken

	// Rotation: the refresh response must carry a new refresh token, distinct from the original.
	status, body := ts.refreshWithResource(originalRefreshToken, "")
	ts.Require().Equal(http.StatusOK, status, "%v", body)
	rotatedRefreshToken, ok := body["refresh_token"].(string)
	ts.Require().True(ok, "Rotation should issue a new refresh_token")
	ts.NotEqual(originalRefreshToken, rotatedRefreshToken, "Rotated refresh token should differ from the original")

	// Replay: reusing the now-revoked original token must fail.
	status, body = ts.refreshWithResource(originalRefreshToken, "")
	ts.Require().Equal(http.StatusBadRequest, status, "%v", body)
	ts.Equal("invalid_grant", body["error"])

	// Family revocation: the replay must have revoked the rotated token too, since it shares the
	// same token family as the replayed original.
	status, body = ts.refreshWithResource(rotatedRefreshToken, "")
	ts.Require().Equal(http.StatusBadRequest, status, "%v", body)
	ts.Equal("invalid_grant", body["error"], "Replay should revoke the whole token family, including the rotated token")
}

// requestWithDPoP submits a token request with the given form parameters and an optional DPoP proof
// header, returning the parsed JSON response.
func (ts *RefreshTokenBranchesTestSuite) requestWithDPoP(
	form url.Values, dpopProof string,
) (int, map[string]interface{}) {
	req, err := http.NewRequest("POST", testutils.TestServerURL+"/oauth2/token",
		bytes.NewBufferString(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if dpopProof != "" {
		req.Header.Set("DPoP", dpopProof)
	}

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	var body map[string]interface{}
	bodyBytes, _ := io.ReadAll(resp.Body)
	ts.Require().NoError(json.Unmarshal(bodyBytes, &body), "body: %s", string(bodyBytes))
	return resp.StatusCode, body
}

// TestRefreshToken_PublicClientDPoP_RequiresMatchingProof verifies that a public client's DPoP-bound
// refresh token requires a matching DPoP proof on every refresh: no proof and a proof from a
// different key are both rejected, and the correct key continues to work across repeated refreshes
// (binding continuity, not a one-off match).
func (ts *RefreshTokenBranchesTestSuite) TestRefreshToken_PublicClientDPoP_RequiresMatchingProof() {
	key, err := testutils.GenerateDPoPKey("ES256")
	ts.Require().NoError(err)
	otherKey, err := testutils.GenerateDPoPKey("ES256")
	ts.Require().NoError(err)

	codeVerifier, err := testutils.GenerateCodeVerifier()
	ts.Require().NoError(err)
	codeChallenge := testutils.GenerateCodeChallenge(codeVerifier)

	resp, err := testutils.InitiateAuthorizationFlowWithPKCE(
		rtBranchPublicClientID, rtBranchRedirectURI, "code", "openid", "test-state", "", codeChallenge, "S256")
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusFound, resp.StatusCode)

	authID, executionID, err := testutils.ExtractAuthData(resp.Header.Get("Location"))
	ts.Require().NoError(err)

	initialStep, err := testutils.ExecuteAuthenticationFlow(executionID, nil, "")
	ts.Require().NoError(err)
	flowStep, err := testutils.ExecuteAuthenticationFlow(executionID, map[string]string{
		"username": rtBranchUsername,
		"password": rtBranchPassword,
	}, "action_001", initialStep.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus)

	authzResp, err := testutils.CompleteAuthorization(authID, flowStep.Assertion)
	ts.Require().NoError(err)
	code, err := testutils.ExtractAuthorizationCode(authzResp.RedirectURI)
	ts.Require().NoError(err)

	tokenForm := url.Values{}
	tokenForm.Set("grant_type", "authorization_code")
	tokenForm.Set("code", code)
	tokenForm.Set("redirect_uri", rtBranchRedirectURI)
	tokenForm.Set("client_id", rtBranchPublicClientID)
	tokenForm.Set("code_verifier", codeVerifier)

	authCodeProof, err := key.CreateProof(http.MethodPost, testutils.TestServerURL+"/oauth2/token", testutils.DPoPProofOptions{})
	ts.Require().NoError(err)

	status, body := ts.requestWithDPoP(tokenForm, authCodeProof)
	ts.Require().Equal(http.StatusOK, status, "%v", body)
	refreshToken, ok := body["refresh_token"].(string)
	ts.Require().True(ok, "Public client should receive a refresh token")

	refreshForm := func() url.Values {
		form := url.Values{}
		form.Set("grant_type", "refresh_token")
		form.Set("refresh_token", refreshToken)
		form.Set("client_id", rtBranchPublicClientID)
		return form
	}

	// No DPoP proof at all: refresh must fail, this refresh token requires proof of possession.
	status, body = ts.requestWithDPoP(refreshForm(), "")
	ts.Require().Equal(http.StatusBadRequest, status, "%v", body)
	ts.Equal("invalid_grant", body["error"])
	ts.Contains(body["error_description"], "DPoP proof required")

	// A proof from a different key: refresh must fail, the key does not match the binding.
	mismatchedProof, err := otherKey.CreateProof(http.MethodPost, testutils.TestServerURL+"/oauth2/token", testutils.DPoPProofOptions{})
	ts.Require().NoError(err)
	status, body = ts.requestWithDPoP(refreshForm(), mismatchedProof)
	ts.Require().Equal(http.StatusBadRequest, status, "%v", body)
	ts.Equal("invalid_grant", body["error"])
	ts.Contains(body["error_description"], "does not match")

	// The correct key succeeds, and continues to succeed on a second refresh with a fresh proof from
	// the same key, proving the binding persists rather than being a one-off match. Rotation is on by
	// default, so the first refresh issues a new refresh token; that is what the second refresh must use.
	correctProof1, err := key.CreateProof(http.MethodPost, testutils.TestServerURL+"/oauth2/token", testutils.DPoPProofOptions{})
	ts.Require().NoError(err)
	status, body = ts.requestWithDPoP(refreshForm(), correctProof1)
	ts.Require().Equal(http.StatusOK, status, "%v", body)
	if rotated, ok := body["refresh_token"].(string); ok && rotated != "" {
		refreshToken = rotated
	}

	correctProof2, err := key.CreateProof(http.MethodPost, testutils.TestServerURL+"/oauth2/token", testutils.DPoPProofOptions{})
	ts.Require().NoError(err)
	status, body = ts.requestWithDPoP(refreshForm(), correctProof2)
	ts.Require().Equal(http.StatusOK, status, "%v", body)
}
