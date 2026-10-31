// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ciba

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	cibaLifespanAgentClientID     = "ciba_lifespan_agent_client"
	cibaLifespanAgentClientSecret = "ciba_lifespan_agent_secret"
)

// cibaLifespanAgent returns the body of a CIBA-enabled agent whose lifespan ends at validUntil.
func (ts *CIBATestSuite) cibaLifespanAgent(validUntil string, withSecret bool) map[string]interface{} {
	config := map[string]interface{}{
		"clientId":                cibaLifespanAgentClientID,
		"grantTypes":              []string{cibaGrantType},
		"tokenEndpointAuthMethod": "client_secret_basic",
	}
	if withSecret {
		config["clientSecret"] = cibaLifespanAgentClientSecret
	}
	return map[string]interface{}{
		"type":              "default",
		"ouId":              ts.ouID,
		"attributes":        map[string]interface{}{"name": "CIBA Lifespan Test Agent"},
		"authFlowId":        ts.flowID,
		"allowedUserTypes":  []string{"ciba-test-person"},
		"validUntil":        validUntil,
		"inboundAuthConfig": []map[string]interface{}{{"type": "oauth2", "config": config}},
	}
}

func (ts *CIBATestSuite) sendAgent(method, path string, body map[string]interface{}) map[string]interface{} {
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
	var out map[string]interface{}
	ts.Require().NoError(json.Unmarshal(raw, &out))
	return out
}

func cibaLifespanEnd(in time.Duration) (string, time.Time) {
	end := time.Now().UTC().Add(in).Truncate(time.Second)
	return end.Format(time.RFC3339), end
}

// TestCIBAGrantFlow_AgentLifespanCapsAndEndsTokens verifies that a user's out-of-band approval for an
// agent yields tokens capped at the agent's validUntil, and none once its lifespan has ended.
func (ts *CIBATestSuite) TestCIBAGrantFlow_AgentLifespanCapsAndEndsTokens() {
	snapshot, err := testutils.SnapshotAgentType()
	ts.Require().NoError(err, "Failed to snapshot the default agent type")
	defer func() {
		ts.Require().NoError(testutils.RestoreAgentType(snapshot),
			"teardown: failed to restore the default agent type")
	}()
	_, err = testutils.CreateAgentType(testutils.UserType{
		Handle: "default", DisplayName: "Default", OUID: ts.ouID,
		Schema: map[string]interface{}{
			"name": map[string]interface{}{"type": "string", "required": true, "unique": true},
		},
	})
	ts.Require().NoError(err)

	validUntil, end := cibaLifespanEnd(2 * time.Minute)
	agentID := ts.sendAgent(http.MethodPost, "/agents", ts.cibaLifespanAgent(validUntil, true))["id"].(string)
	defer func() { _ = testutils.DeleteAgent(agentID) }()

	approve := func() cibaTokenResult {
		status, bcResp := ts.cibaBackchannelAuthorizeAs(cibaLifespanAgentClientID, cibaLifespanAgentClientSecret,
			cibaTestUsername, "openid")
		ts.Require().Equal(http.StatusOK, status, "bc-authorize should succeed")
		ts.completeCIBAFlow(bcResp.AuthReqID)
		return ts.pollCIBATokenUntilIssued(bcResp.AuthReqID, cibaLifespanAgentClientID,
			cibaLifespanAgentClientSecret)
	}

	tokenRes := approve()
	ts.Require().Equal(http.StatusOK, tokenRes.statusCode, tokenRes.errorCode)
	for what, token := range map[string]string{"access token": tokenRes.accessToken, "ID token": tokenRes.idToken} {
		claims, err := testutils.DecodeJWT(token)
		ts.Require().NoError(err, what)
		ts.LessOrEqual(int64(claims.Exp), end.Unix(), "%s outlives the agent", what)
		ts.GreaterOrEqual(int64(claims.Exp), end.Unix()-2, "%s is not capped at validUntil", what)
	}
	claims, _ := testutils.DecodeJWT(tokenRes.accessToken)
	ts.Equal(ts.userID, claims.Sub)
	act, _ := claims.Additional["act"].(map[string]interface{})
	ts.Equal(agentID, act["sub"], "the agent acts for the user")

	// A request the user approved while the agent was live yields no token once its lifespan ends, and
	// the expired agent cannot start a new one. Both endpoints refuse its client authentication.
	status, bcResp := ts.cibaBackchannelAuthorizeAs(cibaLifespanAgentClientID, cibaLifespanAgentClientSecret,
		cibaTestUsername, "openid")
	ts.Require().Equal(http.StatusOK, status, "bc-authorize should succeed")
	ts.completeCIBAFlow(bcResp.AuthReqID)

	validUntil, end = cibaLifespanEnd(3 * time.Second)
	ts.sendAgent(http.MethodPut, "/agents/"+agentID, ts.cibaLifespanAgent(validUntil, false))
	time.Sleep(time.Until(end.Add(time.Second)))

	tokenRes = ts.cibaPollTokenAs(bcResp.AuthReqID, "", cibaLifespanAgentClientID, cibaLifespanAgentClientSecret)
	ts.Equal(http.StatusUnauthorized, tokenRes.statusCode)
	ts.Equal("invalid_client", tokenRes.errorCode)

	status, bcResp = ts.cibaBackchannelAuthorizeAs(cibaLifespanAgentClientID, cibaLifespanAgentClientSecret,
		cibaTestUsername, "openid")
	ts.Equal(http.StatusUnauthorized, status)
	ts.Equal("invalid_client", bcResp.ErrorCode)
}
