// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// agentRegenerationResult is what an agent secret regeneration request answered.
type agentRegenerationResult struct {
	status       int
	cacheControl string
	clientSecret string
	errorCode    string
	body         string
}

// regenerateAgentSecret asks for a new client secret for the agent, as the admin unless a client is given.
func (ts *AgentOAuthFlowsTestSuite) regenerateAgentSecret(client *http.Client, agentID string) agentRegenerationResult {
	ts.T().Helper()
	if client == nil {
		client = testutils.GetHTTPClient()
	}
	req, err := http.NewRequest(http.MethodPost,
		testutils.TestServerURL+"/agents/"+agentID+"/client-secret/regenerate", nil)
	ts.Require().NoError(err)
	resp, err := client.Do(req)
	ts.Require().NoError(err, "Failed to call the regeneration endpoint")
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)

	result := agentRegenerationResult{status: resp.StatusCode, cacheControl: resp.Header.Get("Cache-Control"),
		body: string(body)}
	var decoded struct {
		ClientSecret string `json:"clientSecret"`
		Code         string `json:"code"`
	}
	_ = json.Unmarshal(body, &decoded)
	result.clientSecret = decoded.ClientSecret
	result.errorCode = decoded.Code
	return result
}

// ccAgent creates an agent that requests client_credentials tokens with the client and secret.
func (ts *AgentOAuthFlowsTestSuite) ccAgent(name, clientID, clientSecret, authMethod string) string {
	ts.T().Helper()
	agentID, err := createAgent(Agent{
		OUID:       ts.ouID,
		Type:       "default",
		Attributes: agentAttrs(name, nil),
		AuthFlowID: ts.authFlowID,
		InboundAuthConfig: []InboundAuthConfig{{
			Type: "oauth2",
			Config: &OAuthAgentConfig{
				ClientID:                clientID,
				ClientSecret:            clientSecret,
				GrantTypes:              []string{"client_credentials"},
				TokenEndpointAuthMethod: authMethod,
			},
		}},
	})
	ts.Require().NoError(err)
	return agentID
}

// agentTokenStatus requests a client_credentials token for the agent and returns the status and token.
func (ts *AgentOAuthFlowsTestSuite) agentTokenStatus(clientID, clientSecret string) (int, string) {
	ts.T().Helper()
	result, err := testutils.RequestTokenWithResource(clientID, clientSecret, "", "", "client_credentials",
		agentOAuthDefaultRSIdentifier)
	ts.Require().NoError(err)
	if result.Token == nil {
		return result.StatusCode, ""
	}
	return result.StatusCode, result.Token.AccessToken
}

// A regeneration returns the new secret once, and the change is real: the old secret stops issuing tokens,
// the new one starts, and a token the old secret issued is revoked.
func (ts *AgentOAuthFlowsTestSuite) TestRegenerateAgentSecret_ReplacesTheSecretAndRevokes() {
	const (
		clientID       = "regen_agent_client"
		oldSecret      = "regen_agent_secret" // #nosec G101 -- test fixture
		observerID     = "regen_agent_observer_client"
		observerSecret = "regen_agent_observer_secret" // #nosec G101 -- test fixture
	)
	agentID := ts.ccAgent("regen-agent", clientID, oldSecret, "client_secret_basic")
	defer func() { _ = deleteAgent(agentID) }()
	observerAppID, err := testutils.CreateApplication(testutils.Application{
		Name:         "regen_agent_observer",
		Description:  "introspects the agent's tokens",
		OUID:         ts.ouID,
		AuthFlowID:   ts.authFlowID,
		ClientID:     observerID,
		ClientSecret: observerSecret,
	})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteApplication(observerAppID) }()

	status, token := ts.agentTokenStatus(clientID, oldSecret)
	ts.Require().Equal(http.StatusOK, status, "The agent should issue a token before the regeneration")
	active, err := testutils.IntrospectToken(token, observerID, observerSecret)
	ts.Require().NoError(err)
	ts.Require().True(active, "The token should be active before the secret is regenerated")

	result := ts.regenerateAgentSecret(nil, agentID)

	ts.Require().Equal(http.StatusOK, result.status, "Regeneration failed: %s", result.body)
	ts.Equal("no-store", result.cacheControl, "A response that carries a secret must not be cached")
	ts.Require().NotEmpty(result.clientSecret, "The response should carry the new secret: %s", result.body)
	ts.NotEqual(oldSecret, result.clientSecret)
	status, _ = ts.agentTokenStatus(clientID, oldSecret)
	ts.Equal(http.StatusUnauthorized, status, "The old secret was not rejected")
	status, _ = ts.agentTokenStatus(clientID, result.clientSecret)
	ts.Equal(http.StatusOK, status, "The new secret did not issue tokens")
	active, err = testutils.IntrospectToken(token, observerID, observerSecret)
	ts.Require().NoError(err)
	ts.False(active, "A token issued with the regenerated secret was still active")
}

// An agent that authenticates without a secret has none to regenerate.
func (ts *AgentOAuthFlowsTestSuite) TestRegenerateAgentSecret_RefusesAgentWithoutASecret() {
	agentID, err := createAgent(Agent{
		OUID:       ts.ouID,
		Type:       "default",
		Attributes: agentAttrs("regen-entity-only-agent", nil),
	})
	ts.Require().NoError(err)
	defer func() { _ = deleteAgent(agentID) }()

	result := ts.regenerateAgentSecret(nil, agentID)

	ts.Equal(http.StatusBadRequest, result.status, result.body)
	ts.Equal("AGT-1052", result.errorCode, result.body)
	ts.Empty(result.clientSecret)
}

// An agent that does not exist is answered as not found.
func (ts *AgentOAuthFlowsTestSuite) TestRegenerateAgentSecret_UnknownAgent() {
	result := ts.regenerateAgentSecret(nil, "01900000-0000-7000-8000-0000000000fe")

	ts.Equal(http.StatusNotFound, result.status, result.body)
	ts.Equal("AGT-1004", result.errorCode, result.body)
	ts.Empty(result.clientSecret)
}

// A caller without a token is refused, and the secret stays as it was.
func (ts *AgentOAuthFlowsTestSuite) TestRegenerateAgentSecret_RejectsAnonymousCaller() {
	const (
		clientID = "regen_anon_agent_client"
		secret   = "regen_anon_agent_secret" // #nosec G101 -- test fixture
	)
	agentID := ts.ccAgent("regen-anon-agent", clientID, secret, "client_secret_basic")
	defer func() { _ = deleteAgent(agentID) }()

	result := ts.regenerateAgentSecret(testutils.GetRawHTTPClient(), agentID)

	ts.Equal(http.StatusUnauthorized, result.status, result.body)
	ts.Empty(result.clientSecret)
	status, _ := ts.agentTokenStatus(clientID, secret)
	ts.Equal(http.StatusOK, status, "The secret changed for an anonymous caller")
}

// An ID that names an application rather than an agent is answered as an agent that does not exist.
func (ts *AgentOAuthFlowsTestSuite) TestRegenerateAgentSecret_RefusesAnApplication() {
	appID, err := testutils.CreateApplication(testutils.Application{
		Name:         "regen_agent_not_an_agent",
		Description:  "an application, not an agent",
		OUID:         ts.ouID,
		AuthFlowID:   ts.authFlowID,
		ClientID:     "regen_agent_not_an_agent_client",
		ClientSecret: "regen_agent_not_an_agent_secret", // #nosec G101 -- test fixture
	})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteApplication(appID) }()

	result := ts.regenerateAgentSecret(nil, appID)

	ts.Equal(http.StatusNotFound, result.status, result.body)
	ts.Equal("AGT-1004", result.errorCode, result.body)
	ts.Empty(result.clientSecret)
}

// The route answers its own CORS preflight, so a browser may send the POST: the preflight of the
// agent's other routes does not allow it.
func (ts *AgentOAuthFlowsTestSuite) TestRegenerateAgentSecret_AnswersThePreflight() {
	req, err := http.NewRequest(http.MethodOptions,
		testutils.TestServerURL+"/agents/01900000-0000-7000-8000-0000000000fe/client-secret/regenerate", nil)
	ts.Require().NoError(err)
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Equal(http.StatusNoContent, resp.StatusCode)
}
