// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// createLifespanAgent creates a client credentials agent whose lifespan ends at validUntil, or does
// not end when validUntil is nil.
func (ts *AgentOAuthFlowsTestSuite) createLifespanAgent(name, clientID string, validUntil *string) string {
	agentID, err := createAgent(Agent{
		OUID:       ts.ouID,
		Type:       "default",
		Attributes: agentAttrs(name, nil),
		AuthFlowID: ts.authFlowID,
		ValidUntil: validUntil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				Config: &OAuthAgentConfig{
					ClientID:                clientID,
					ClientSecret:            clientID + "_secret",
					GrantTypes:              []string{"client_credentials"},
					TokenEndpointAuthMethod: "client_secret_basic",
				},
			},
		},
	})
	ts.Require().NoError(err)
	return agentID
}

func (ts *AgentOAuthFlowsTestSuite) getAgent(agentID string) Agent {
	resp, err := doGet(testServerURL + agentBasePath + "/" + agentID)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode, readBody(resp))
	var agent Agent
	ts.Require().NoError(json.Unmarshal([]byte(readBody(resp)), &agent))
	return agent
}

func lifespanEnd(in time.Duration) *string {
	end := time.Now().UTC().Add(in).Truncate(time.Second).Format(time.RFC3339)
	return &end
}

// TestAgentLifespan_SetChangeAndRemove covers registering an agent with a lifespan, an update that
// leaves it out, removing the expiry with null, and the values the API refuses.
func (ts *AgentOAuthFlowsTestSuite) TestAgentLifespan_SetChangeAndRemove() {
	validUntil := lifespanEnd(48 * time.Hour)
	agentID := ts.createLifespanAgent("lifespan-crud-agent", "lifespan_crud_client", validUntil)
	defer func() { _ = deleteAgent(agentID) }()

	agent := ts.getAgent(agentID)
	ts.Require().NotNil(agent.ValidUntil)
	ts.Equal(*validUntil, *agent.ValidUntil)
	ts.Require().NotNil(agent.Lifespan)
	ts.Equal("ACTIVE", agent.Lifespan.Phase)

	// An update that does not carry validUntil leaves the lifespan unchanged.
	resp, err := putAgentRaw(agentID, map[string]interface{}{
		"type": "default", "ouId": ts.ouID, "attributes": map[string]interface{}{"name": "lifespan-crud-agent"},
	})
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, resp.StatusCode, readBody(resp))
	_ = resp.Body.Close()
	ts.Equal(*validUntil, *ts.getAgent(agentID).ValidUntil)

	// A past value is refused and changes nothing.
	resp, err = putAgentRaw(agentID, map[string]interface{}{
		"type": "default", "ouId": ts.ouID, "attributes": map[string]interface{}{"name": "renamed-agent"},
		"validUntil": "2020-01-01T00:00:00Z",
	})
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, resp.StatusCode)
	ts.Equal("AGT-1044", errorCode(resp))
	_ = resp.Body.Close()
	ts.Equal("lifespan-crud-agent", ts.getAgent(agentID).AttrName(), "a refused lifespan applies no field")

	// null removes the expiry.
	resp, err = putAgentRaw(agentID, map[string]interface{}{
		"type": "default", "ouId": ts.ouID, "attributes": map[string]interface{}{"name": "lifespan-crud-agent"},
		"validUntil": nil,
	})
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, resp.StatusCode, readBody(resp))
	_ = resp.Body.Close()
	agent = ts.getAgent(agentID)
	ts.Nil(agent.ValidUntil, "an unbounded lifespan omits validUntil")
	ts.Equal("UNBOUNDED", agent.Lifespan.Phase)
}

func (ts *AgentOAuthFlowsTestSuite) TestAgentLifespan_RegistrationRefusesInvalidValues() {
	for value, code := range map[string]string{
		"2027-03-31":           "AGT-1043",
		"2020-01-01T00:00:00Z": "AGT-1044",
	} {
		resp, err := doPost(testServerURL+agentBasePath, map[string]interface{}{
			"ouId": ts.ouID, "type": "default", "validUntil": value,
			"attributes": map[string]interface{}{"name": "lifespan-invalid-agent"},
		})
		ts.Require().NoError(err)
		ts.Equal(http.StatusBadRequest, resp.StatusCode, value)
		ts.Equal(code, errorCode(resp), value)
		_ = resp.Body.Close()
	}
}

// TestAgentLifespan_TokenCappedAtValidUntil verifies an access token never outlives the agent.
func (ts *AgentOAuthFlowsTestSuite) TestAgentLifespan_TokenCappedAtValidUntil() {
	agentID := ts.createLifespanAgent("lifespan-cap-agent", "lifespan_cap_client", lifespanEnd(2*time.Minute))
	defer func() { _ = deleteAgent(agentID) }()

	tokenResult, err := testutils.RequestTokenWithResource(
		"lifespan_cap_client", "lifespan_cap_client_secret", "", "", "client_credentials",
		agentOAuthDefaultRSIdentifier)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, tokenResult.StatusCode, string(tokenResult.Body))
	ts.Greater(tokenResult.Token.ExpiresIn, float64(0))
	ts.LessOrEqual(tokenResult.Token.ExpiresIn, float64(120))
}

// TestAgentLifespan_ExpiredAgentObtainsNoToken verifies that the agent obtains tokens until validUntil
// and is refused with invalid_grant once it passes, and that the agent is kept, reported as expired.
func (ts *AgentOAuthFlowsTestSuite) TestAgentLifespan_ExpiredAgentObtainsNoToken() {
	// Long enough that creating the agent and its first token request finish well before the final
	// second, in which issuance already stops, even on a slow runner.
	validUntil := lifespanEnd(10 * time.Second)
	agentID := ts.createLifespanAgent("lifespan-expiry-agent", "lifespan_expiry_client", validUntil)
	defer func() { _ = deleteAgent(agentID) }()

	requestToken := func() *testutils.TokenHTTPResult {
		result, err := testutils.RequestTokenWithResource(
			"lifespan_expiry_client", "lifespan_expiry_client_secret", "", "", "client_credentials",
			agentOAuthDefaultRSIdentifier)
		ts.Require().NoError(err)
		return result
	}

	tokenResult := requestToken()
	ts.Require().Equal(http.StatusOK, tokenResult.StatusCode, string(tokenResult.Body))

	end, err := time.Parse(time.RFC3339, *validUntil)
	ts.Require().NoError(err)
	deadline := end.Add(5 * time.Second)
	for tokenResult.StatusCode == http.StatusOK && time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
		tokenResult = requestToken()
	}
	ts.Equal(http.StatusBadRequest, tokenResult.StatusCode, string(tokenResult.Body))
	ts.Contains(string(tokenResult.Body), "invalid_grant")
	// The builder holds back the final second, in which a token could only be issued with an exp
	// after validUntil, so refusal starts at most one second early.
	ts.False(time.Now().Before(end.Add(-time.Second)), "refused more than a second before validUntil")

	time.Sleep(time.Until(end))
	ts.Equal("EXPIRED", ts.getAgent(agentID).Lifespan.Phase)
}
