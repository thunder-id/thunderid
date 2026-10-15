// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package idjag

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	lifespanAgentClientID     = "idjag_lifespan_agent_client"
	lifespanAgentClientSecret = "idjag_lifespan_agent_secret"
)

// lifespanAgentBody returns the body of an agent that signs users in, requests ID-JAGs for this server,
// and redeems them through the JWT bearer grant, with its lifespan ending at validUntil.
func (ts *IDJAGConsumptionTestSuite) lifespanAgentBody(validUntil string, withSecret bool) map[string]any {
	config := map[string]any{
		"clientId":     lifespanAgentClientID,
		"redirectUris": []string{consumptionRedirectURI},
		"grantTypes": []string{
			"authorization_code",
			"urn:ietf:params:oauth:grant-type:token-exchange",
			"urn:ietf:params:oauth:grant-type:jwt-bearer",
		},
		"responseTypes":           []string{"code"},
		"tokenEndpointAuthMethod": "client_secret_basic",
		"scopes":                  []string{"openid", "profile", "email"},
		"token": map[string]any{
			"idJag": map[string]any{"enabled": true, "allowedAudiences": []string{"https://localhost:8095"}},
		},
	}
	if withSecret {
		config["clientSecret"] = lifespanAgentClientSecret
	}
	return map[string]any{
		"type": "default", "ouId": ts.ouID, "authFlowId": ts.authFlowID,
		"attributes":        map[string]any{"name": "IDJAG Lifespan Agent"},
		"allowedUserTypes":  []string{"idjag-consumption-person"},
		"validUntil":        validUntil,
		"inboundAuthConfig": []map[string]any{{"type": "oauth2", "config": config}},
	}
}

func (ts *IDJAGConsumptionTestSuite) sendLifespanAgent(method, path string, body map[string]any) string {
	data, err := json.Marshal(body)
	ts.Require().NoError(err)
	req, err := http.NewRequest(method, testutils.TestServerURL+path, bytes.NewBuffer(data))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	ts.Require().Less(resp.StatusCode, 300, string(raw))
	var out map[string]any
	ts.Require().NoError(json.Unmarshal(raw, &out))
	id, _ := out["id"].(string)
	return id
}

// agentIDToken signs the suite's user in to the agent and returns the ID token.
func (ts *IDJAGConsumptionTestSuite) agentIDToken() string {
	verifier, err := testutils.GenerateCodeVerifier()
	ts.Require().NoError(err)
	params := url.Values{
		"client_id": {lifespanAgentClientID}, "redirect_uri": {consumptionRedirectURI},
		"response_type": {"code"}, "scope": {"openid email"}, "state": {"idjag-lifespan"},
		"code_challenge": {testutils.GenerateCodeChallenge(verifier)}, "code_challenge_method": {"S256"},
	}
	req, err := http.NewRequest("GET", testutils.TestServerURL+"/oauth2/authorize?"+params.Encode(), nil)
	ts.Require().NoError(err)
	resp, err := testutils.GetNoRedirectHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusFound, resp.StatusCode)
	authID, executionID, err := testutils.ExtractAuthData(resp.Header.Get("Location"))
	ts.Require().NoError(err)

	initial, err := testutils.ExecuteAuthenticationFlow(executionID, nil, "")
	ts.Require().NoError(err)
	step, err := testutils.ExecuteAuthenticationFlow(executionID, map[string]string{
		"username": consumptionTestUser, "password": consumptionTestPassword,
	}, "action_001", initial.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("COMPLETE", step.FlowStatus)
	authzResp, err := testutils.CompleteAuthorization(authID, step.Assertion)
	ts.Require().NoError(err)
	code, err := testutils.ExtractAuthorizationCode(authzResp.RedirectURI)
	ts.Require().NoError(err)

	status, body := ts.lifespanAgentToken(url.Values{
		"grant_type": {"authorization_code"}, "code": {code},
		"redirect_uri": {consumptionRedirectURI}, "code_verifier": {verifier},
	})
	ts.Require().Equal(http.StatusOK, status, body)
	var tokens map[string]any
	ts.Require().NoError(json.Unmarshal([]byte(body), &tokens))
	idToken, _ := tokens["id_token"].(string)
	ts.Require().NotEmpty(idToken)
	return idToken
}

// lifespanAgentToken sends a token request authenticated as the agent and returns the status and body.
func (ts *IDJAGConsumptionTestSuite) lifespanAgentToken(form url.Values) (int, string) {
	req, err := http.NewRequest("POST", testutils.TestServerURL+"/oauth2/token", strings.NewReader(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(lifespanAgentClientID, lifespanAgentClientSecret)
	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(raw)
}

func idJagRequest(idToken string) url.Values {
	return url.Values{
		"grant_type":           {"urn:ietf:params:oauth:grant-type:token-exchange"},
		"subject_token":        {idToken},
		"subject_token_type":   {"urn:ietf:params:oauth:token-type:id_token"},
		"requested_token_type": {"urn:ietf:params:oauth:token-type:id-jag"},
		"audience":             {"https://localhost:8095"},
	}
}

func (ts *IDJAGConsumptionTestSuite) requireCappedAt(token string, end time.Time, what string) {
	claims, err := testutils.DecodeJWT(token)
	ts.Require().NoError(err, what)
	ts.LessOrEqual(int64(claims.Exp), end.Unix(), "%s outlives the agent", what)
	ts.GreaterOrEqual(int64(claims.Exp), end.Unix()-2, "%s is not capped at validUntil", what)
}

// TestIDJAG_AgentLifespanCapsAndEndsIssuanceAndRedemption verifies that an agent's ID token, the ID-JAG
// it requests, and the access token it redeems that ID-JAG for through the JWT bearer grant are each
// capped at the agent's validUntil, and that once the lifespan ends the agent can neither request an
// ID-JAG nor redeem one it already holds.
func (ts *IDJAGConsumptionTestSuite) TestIDJAG_AgentLifespanCapsAndEndsIssuanceAndRedemption() {
	snapshot, err := testutils.SnapshotAgentType()
	ts.Require().NoError(err)
	defer func() { ts.Require().NoError(testutils.RestoreAgentType(snapshot)) }()
	_, err = testutils.CreateAgentType(testutils.UserType{
		Handle: "default", DisplayName: "Default", OUID: ts.ouID,
		Schema: map[string]any{"name": map[string]any{"type": "string", "required": true, "unique": true}},
	})
	ts.Require().NoError(err)

	end := time.Now().UTC().Add(2 * time.Minute).Truncate(time.Second)
	agentID := ts.sendLifespanAgent(http.MethodPost, "/agents", ts.lifespanAgentBody(end.Format(time.RFC3339), true))
	defer func() { _ = testutils.DeleteAgent(agentID) }()

	idToken := ts.agentIDToken()
	ts.requireCappedAt(idToken, end, "ID token")

	status, body := ts.lifespanAgentToken(idJagRequest(idToken))
	ts.Require().Equal(http.StatusOK, status, body)
	var issued IDJAGTokenResponse
	ts.Require().NoError(json.Unmarshal([]byte(body), &issued))
	ts.requireCappedAt(issued.AccessToken, end, "ID-JAG")

	status, body = ts.lifespanAgentToken(url.Values{
		"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"}, "assertion": {issued.AccessToken},
	})
	ts.Require().Equal(http.StatusOK, status, body)
	var redeemed IDJAGTokenResponse
	ts.Require().NoError(json.Unmarshal([]byte(body), &redeemed))
	ts.requireCappedAt(redeemed.AccessToken, end, "JWT bearer access token")

	// An ID-JAG obtained while the agent is live, redeemed only after its lifespan ends.
	status, body = ts.lifespanAgentToken(idJagRequest(idToken))
	ts.Require().Equal(http.StatusOK, status, body)
	var held IDJAGTokenResponse
	ts.Require().NoError(json.Unmarshal([]byte(body), &held))

	end = time.Now().UTC().Add(3 * time.Second).Truncate(time.Second)
	ts.sendLifespanAgent(http.MethodPut, "/agents/"+agentID, ts.lifespanAgentBody(end.Format(time.RFC3339), false))
	time.Sleep(time.Until(end.Add(time.Second)))

	status, body = ts.lifespanAgentToken(idJagRequest(idToken))
	ts.Equal(http.StatusUnauthorized, status, body)
	ts.Contains(body, "invalid_client")

	status, body = ts.lifespanAgentToken(url.Values{
		"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"}, "assertion": {held.AccessToken},
	})
	ts.Equal(http.StatusUnauthorized, status, body)
	ts.Contains(body, "invalid_client")
}
