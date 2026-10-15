// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authentication

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	lifespanSignInAgentUsername = "agent_lifespan_signin_bot"
	lifespanSignInAgentPassword = "AgentLifespanBot1!"
	lifespanSignInClientID      = "agent_lifespan_signin_app"
	lifespanSignInClientSecret  = "agent_lifespan_signin_secret"
	lifespanSignInRedirectURI   = "https://localhost:3000"
)

func (ts *AgentSubjectTypeTestSuite) sendLifespanAgent(method, path string, validUntil string) string {
	attributes := map[string]interface{}{
		"name": "agent-lifespan-signin-bot", "agent_username": lifespanSignInAgentUsername,
	}
	if method == http.MethodPost {
		attributes["password"] = lifespanSignInAgentPassword
	}
	data, err := json.Marshal(map[string]interface{}{
		"type": agentSubjectAgentType, "ouId": ts.ouID, "attributes": attributes, "validUntil": validUntil,
	})
	ts.Require().NoError(err)
	req, err := http.NewRequest(method, testutils.TestServerURL+path, bytes.NewReader(data))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	ts.Require().Less(resp.StatusCode, 300, string(raw))
	var out map[string]interface{}
	ts.Require().NoError(json.Unmarshal(raw, &out))
	return out["id"].(string)
}

// submitLifespanAgentCredentials starts the application's authorization code flow and submits the agent's
// credentials, returning the authorization request and the flow's answer.
func (ts *AgentSubjectTypeTestSuite) submitLifespanAgentCredentials() (string, *testutils.FlowStep) {
	resp, err := testutils.InitiateAuthorizationFlow(lifespanSignInClientID, lifespanSignInRedirectURI, "code",
		"openid", "lifespan-state")
	ts.Require().NoError(err)
	location := resp.Header.Get("Location")
	_ = resp.Body.Close()
	authID, executionID, err := testutils.ExtractAuthData(location)
	ts.Require().NoError(err, location)

	initial, err := testutils.ExecuteAuthenticationFlow(executionID, nil, "")
	ts.Require().NoError(err)
	step, err := testutils.ExecuteAuthenticationFlow(executionID, map[string]string{
		"agent_username": lifespanSignInAgentUsername, "password": lifespanSignInAgentPassword,
	}, "action_001", initial.ChallengeToken)
	ts.Require().NoError(err)
	return authID, step
}

// signInToApp runs the authorization code flow of the application with the agent as the subject and
// returns the token endpoint's answer.
func (ts *AgentSubjectTypeTestSuite) signInToApp() *testutils.TokenHTTPResult {
	authID, step := ts.submitLifespanAgentCredentials()
	ts.Require().Equal("COMPLETE", step.FlowStatus, "the agent signs in")

	authz, err := testutils.CompleteAuthorization(authID, step.Assertion)
	ts.Require().NoError(err)
	code, err := testutils.ExtractAuthorizationCode(authz.RedirectURI)
	ts.Require().NoError(err)
	result, err := testutils.RequestToken(lifespanSignInClientID, lifespanSignInClientSecret, code,
		lifespanSignInRedirectURI, "authorization_code")
	ts.Require().NoError(err)
	return result
}

// TestAgentSignInTokensEndWithTheAgentLifespan verifies that when an agent signs in to an application
// as its subject, the application's tokens are capped at the agent's validUntil, and the agent's sign
// in is refused once its lifespan has ended.
func (ts *AgentSubjectTypeTestSuite) TestAgentSignInTokensEndWithTheAgentLifespan() {
	appID, err := testutils.CreateApplication(testutils.Application{
		Name: "Agent Lifespan Sign-In App", Description: "Application an agent with a lifespan signs in to",
		OUID: ts.ouID, Type: "fullstack", AuthFlowID: ts.flowID, AllowedAgentTypes: []string{agentSubjectAgentType},
		InboundAuthConfig: []map[string]interface{}{{"type": "oauth2", "config": map[string]interface{}{
			"clientId": lifespanSignInClientID, "clientSecret": lifespanSignInClientSecret,
			"redirectUris": []string{lifespanSignInRedirectURI}, "grantTypes": []string{"authorization_code"},
			"responseTypes": []string{"code"}, "tokenEndpointAuthMethod": "client_secret_basic",
		}}},
	})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteApplication(appID) }()

	end := time.Now().UTC().Add(2 * time.Minute).Truncate(time.Second)
	agentID := ts.sendLifespanAgent(http.MethodPost, "/agents", end.Format(time.RFC3339))
	defer func() { _ = testutils.DeleteAgent(agentID) }()

	result := ts.signInToApp()
	ts.Require().Equal(http.StatusOK, result.StatusCode, string(result.Body))
	for what, token := range map[string]string{
		"access token": result.Token.AccessToken, "ID token": result.Token.IDToken,
	} {
		claims, err := testutils.DecodeJWT(token)
		ts.Require().NoError(err, what)
		ts.Equal(agentID, claims.Sub, "the agent is the subject of the %s", what)
		ts.LessOrEqual(int64(claims.Exp), end.Unix(), "%s outlives the agent", what)
		ts.GreaterOrEqual(int64(claims.Exp), end.Unix()-2, "%s is not capped at validUntil", what)
	}

	end = time.Now().UTC().Add(3 * time.Second).Truncate(time.Second)
	ts.sendLifespanAgent(http.MethodPut, "/agents/"+agentID, end.Format(time.RFC3339))
	time.Sleep(time.Until(end))

	// Governance refuses the expired agent at authentication, so the sign-in stays on the credentials
	// prompt with the refusal stated and yields no assertion to redeem.
	_, step := ts.submitLifespanAgentCredentials()
	ts.Equal("INCOMPLETE", step.FlowStatus, "the expired agent must not sign in")
	ts.Empty(step.Assertion)
	ts.Require().NotNil(step.Error, "the prompt must state the refusal: %+v", step)
	ts.T().Logf("expired agent sign-in refused with %s", step.Error.Code)
}
