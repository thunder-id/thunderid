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
	cibaActClaimClientID     = "ciba_act_claim_test_client"
	cibaActClaimClientSecret = "ciba_act_claim_test_secret"
	cibaAgentClientID        = "ciba_agent_test_client"
	cibaAgentClientSecret    = "ciba_agent_test_secret"
)

// createCIBAAppWithActClaim creates a CIBA-enabled application bound to the shared test auth flow
// with includeActClaim turned on.
func (ts *CIBATestSuite) createCIBAAppWithActClaim(clientID, clientSecret string) string {
	app := map[string]interface{}{
		"name":                      "CIBAActClaimTestApp",
		"description":               "Application for CIBA act claim integration test",
		"ouId":                      ts.ouID,
		"type":                      "fullstack",
		"authFlowId":                ts.flowID,
		"isRegistrationFlowEnabled": false,
		"allowedUserTypes":          []string{"ciba-test-person"},
		"inboundAuthConfig": []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                clientID,
					"clientSecret":            clientSecret,
					"redirectUris":            []string{"https://localhost:3000"},
					"grantTypes":              []string{cibaGrantType},
					"tokenEndpointAuthMethod": "client_secret_basic",
					"includeActClaim":         true,
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
		ts.T().Fatalf("Failed to create CIBA act-claim application. Status: %d, Response: %s",
			resp.StatusCode, string(bodyBytes))
	}

	var respData map[string]interface{}
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&respData))
	return respData["id"].(string)
}

// createCIBAAgent creates a CIBA-enabled agent bound to the shared test auth flow. The agent carries
// no includeActClaim setting: an agent's act claim is implicit.
func (ts *CIBATestSuite) createCIBAAgent(clientID, clientSecret string) string {
	agent := map[string]interface{}{
		"name":             "CIBA Act Claim Test Agent",
		"type":             "default",
		"ouId":             ts.ouID,
		"authFlowId":       ts.flowID,
		"allowedUserTypes": []string{"ciba-test-person"},
		"inboundAuthConfig": []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                clientID,
					"clientSecret":            clientSecret,
					"grantTypes":              []string{cibaGrantType},
					"tokenEndpointAuthMethod": "client_secret_basic",
				},
			},
		},
	}

	jsonData, err := json.Marshal(agent)
	ts.Require().NoError(err)

	req, err := http.NewRequest("POST", testutils.TestServerURL+"/agents", bytes.NewBuffer(jsonData))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		ts.T().Fatalf("Failed to create CIBA agent. Status: %d, Response: %s",
			resp.StatusCode, string(bodyBytes))
	}

	var respData map[string]interface{}
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&respData))
	return respData["id"].(string)
}

// pollCIBATokenUntilIssued polls the token endpoint as the given client until the authenticated
// request yields tokens, backing off past the interval enforced while the request was pending.
func (ts *CIBATestSuite) pollCIBATokenUntilIssued(authReqID, clientID, clientSecret string) cibaTokenResult {
	var tokenRes cibaTokenResult
	deadline := time.Now().Add(30 * time.Second)
	delay := cibaPollIntervalSeconds * time.Second
	for {
		tokenRes = ts.cibaPollTokenAs(authReqID, "", clientID, clientSecret)
		if tokenRes.statusCode != http.StatusBadRequest ||
			(tokenRes.errorCode != "slow_down" && tokenRes.errorCode != "authorization_pending") {
			break
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(delay)
		delay += cibaPollIntervalSeconds * time.Second
	}
	return tokenRes
}

// TestCIBAGrantFlow_ActClaimOptIn verifies that an application that opts in through includeActClaim
// gets an on-behalf-of "act" claim naming itself on a CIBA-issued access token, matching the
// behaviour already proven for the authorization_code grant.
func (ts *CIBATestSuite) TestCIBAGrantFlow_ActClaimOptIn() {
	appID := ts.createCIBAAppWithActClaim(cibaActClaimClientID, cibaActClaimClientSecret)
	defer func() { _ = testutils.DeleteApplication(appID) }()

	status, bcResp := ts.cibaBackchannelAuthorizeAs(cibaActClaimClientID, cibaActClaimClientSecret,
		cibaTestUsername, "openid")
	ts.Require().Equal(http.StatusOK, status, "bc-authorize should succeed")
	ts.Require().NotEmpty(bcResp.AuthReqID)

	ts.completeCIBAFlow(bcResp.AuthReqID)

	tokenRes := ts.pollCIBATokenUntilIssued(bcResp.AuthReqID, cibaActClaimClientID, cibaActClaimClientSecret)
	ts.Require().Equal(http.StatusOK, tokenRes.statusCode, "AUTHENTICATED request should issue tokens")
	ts.Require().NotEmpty(tokenRes.accessToken)

	claims, err := testutils.DecodeJWT(tokenRes.accessToken)
	ts.Require().NoError(err)
	ts.Require().Equal(ts.userID, claims.Sub, "token subject should remain the CIBA user")

	act, ok := claims.Additional["act"].(map[string]interface{})
	ts.Require().True(ok, "an opted-in application should get an act claim")
	ts.Assert().Equal(appID, act["sub"], "act.sub should name the application entity")
}

// TestCIBAGrantFlow_AgentActClaim verifies that an agent polling the CIBA token endpoint always gets
// an act claim naming itself, without any opt-in. The user remains the token subject, so a resource
// server can tell which agent is acting on whose behalf.
func (ts *CIBATestSuite) TestCIBAGrantFlow_AgentActClaim() {
	// The `default` agent type is a singleton shared with every other suite. Snapshot it before
	// pointing it at this suite's OU, so teardown can put it back before that OU is deleted.
	snapshot, err := testutils.SnapshotAgentType()
	ts.Require().NoError(err, "Failed to snapshot the default agent type")
	defer func() {
		ts.Require().NoError(testutils.RestoreAgentType(snapshot),
			"teardown: failed to restore the default agent type")
	}()

	_, err = testutils.CreateAgentType(testutils.UserType{
		Name: "default",
		OUID: ts.ouID,
		Schema: map[string]interface{}{
			"description": map[string]interface{}{"type": "string"},
		},
	})
	ts.Require().NoError(err, "Failed to point the default agent type at the CIBA test OU")

	agentID := ts.createCIBAAgent(cibaAgentClientID, cibaAgentClientSecret)
	defer func() { _ = testutils.DeleteAgent(agentID) }()

	status, bcResp := ts.cibaBackchannelAuthorizeAs(cibaAgentClientID, cibaAgentClientSecret,
		cibaTestUsername, "openid")
	ts.Require().Equal(http.StatusOK, status, "bc-authorize should succeed for an agent client")
	ts.Require().NotEmpty(bcResp.AuthReqID)

	ts.completeCIBAFlow(bcResp.AuthReqID)

	tokenRes := ts.pollCIBATokenUntilIssued(bcResp.AuthReqID, cibaAgentClientID, cibaAgentClientSecret)
	ts.Require().Equal(http.StatusOK, tokenRes.statusCode, "AUTHENTICATED request should issue tokens")
	ts.Require().NotEmpty(tokenRes.accessToken)

	claims, err := testutils.DecodeJWT(tokenRes.accessToken)
	ts.Require().NoError(err)
	ts.Require().Equal(ts.userID, claims.Sub, "token subject should remain the CIBA user")

	act, ok := claims.Additional["act"].(map[string]interface{})
	ts.Require().True(ok, "an agent should always get an act claim")
	ts.Assert().Equal(agentID, act["sub"], "act.sub should name the agent entity")
}
