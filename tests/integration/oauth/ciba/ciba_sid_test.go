// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ciba

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	cibaSessionClientID     = "ciba_test_client_session"
	cibaSessionClientSecret = "ciba_test_secret_session"
)

// cibaSessionFlowNodes is the suite's CIBA flow with a Session node between credential
// authentication and the assertion, so the authentication establishes an SSO session. The flow
// validator requires an SSO-check node to reference it; for a CIBA request there is no session
// cookie, so that check always falls through to the credential prompt.
func cibaSessionFlowNodes(senderID string) []map[string]interface{} {
	nodes := cibaAuthFlowNodes("username", senderID, false)
	for _, node := range nodes {
		// The check sits just before the credential prompt, so its failure path (the normal CIBA
		// path) lands on a PROMPT node as the validator requires.
		if node["onSuccess"] == "prompt_credentials" {
			node["onSuccess"] = "sso_check"
		}
		executor, _ := node["executor"].(map[string]interface{})
		if executor != nil && executor["name"] == "CredentialsAuthExecutor" {
			node["onSuccess"] = "session_main"
		}
	}
	return append(nodes,
		map[string]interface{}{
			"id":         "sso_check",
			"type":       "TASK_EXECUTION",
			"executor":   map[string]interface{}{"name": "SSOCheckExecutor"},
			"properties": map[string]interface{}{"checkpointRef": "session_main"},
			"onSuccess":  "session_main",
			"onFailure":  "prompt_credentials",
		},
		map[string]interface{}{
			"id":        "session_main",
			"type":      "TASK_EXECUTION",
			"executor":  map[string]interface{}{"name": "SessionExecutor"},
			"onSuccess": "auth_assert",
		})
}

// cibaTokensFor runs a full CIBA request as the given client and returns the issued tokens and the sessions its
// approval started, as the approving browser holds them.
func (ts *CIBATestSuite) cibaTokensFor(clientID, clientSecret string) (
	cibaTokenResult, []testutils.FlowSessionData) {
	form := url.Values{}
	form.Set("login_hint", cibaTestUsername)
	form.Set("scope", "openid")
	status, bcResp := ts.cibaBackchannelAuthorizeForm(form, clientID, clientSecret)
	ts.Require().Equal(http.StatusOK, status, "bc-authorize should succeed")
	ts.Require().NotEmpty(bcResp.AuthReqID)

	executionID, inviteToken := ts.recoverCIBAInvite(bcResp.AuthReqID)
	approval := ts.driveCIBAFlowStep(executionID, inviteToken, bcResp.AuthReqID)
	sessions := make([]testutils.FlowSessionData, 0, len(approval.SessionData))
	for _, issued := range approval.SessionData {
		sessions = append(sessions, issued.FlowSessionData)
	}

	tokenRes := ts.cibaPollTokenAs(bcResp.AuthReqID, "", clientID, clientSecret)
	if tokenRes.statusCode == http.StatusBadRequest && tokenRes.errorCode == "slow_down" {
		time.Sleep(cibaPollIntervalSeconds * time.Second)
		tokenRes = ts.cibaPollTokenAs(bcResp.AuthReqID, "", clientID, clientSecret)
	}
	ts.Require().Equal(http.StatusOK, tokenRes.statusCode, "AUTHENTICATED request should issue tokens")
	ts.Require().NotEmpty(tokenRes.idToken, "openid scope should yield an ID token")
	return tokenRes, sessions
}

// createCIBASessionApp creates the CIBA application on a flow with a Session node, removed when the test ends.
func (ts *CIBATestSuite) createCIBASessionApp() {
	flowID, err := testutils.CreateFlow(testutils.Flow{
		Name:     "CIBA Session Test Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_ciba_session_test",
		Nodes:    cibaSessionFlowNodes(ts.senderID),
	})
	ts.Require().NoError(err, "Failed to create the CIBA session test flow")
	ts.T().Cleanup(func() { _ = testutils.DeleteFlow(flowID) })
	appID := ts.createCIBAApp("CIBASessionTestApp", cibaSessionClientID, cibaSessionClientSecret,
		[]string{cibaGrantType, "refresh_token"}, flowID)
	ts.T().Cleanup(func() { _ = testutils.DeleteApplication(appID) })
}

// assertCarriesTheParticipantsFamily checks the tokens name a session and carry the token family its participant
// holds, so ending the session ends them. It returns the session id.
func (ts *CIBATestSuite) assertCarriesTheParticipantsFamily(tokens cibaTokenResult) string {
	idClaims, err := testutils.DecodeJWTPayloadMap(tokens.idToken)
	ts.Require().NoError(err)
	sid, _ := idClaims["sid"].(string)
	ts.Require().NotEmpty(sid, "the ID token must name the session")
	accessClaims, err := testutils.DecodeJWTPayloadMap(tokens.accessToken)
	ts.Require().NoError(err)
	tfid, _ := accessClaims["tfid"].(string)
	ts.Require().NotEmpty(tfid, "the access token must carry its token family")

	participant, err := testutils.QueryRuntimeDB(fmt.Sprintf(
		`SELECT TFID FROM "SESSION_PARTICIPANT" WHERE SESSION_ID = '%s';`, sid))
	ts.Require().NoError(err)
	ts.Equal(tfid, participant, "the tokens must carry the participant's token family")
	return sid
}

// TestCIBATokensCarryTheirSessionAndFamily verifies CIBA tokens name the session and carry its participant's token
// family: the SSO session's when the flow establishes one, and the direct session's when it has none.
func (ts *CIBATestSuite) TestCIBATokensCarryTheirSessionAndFamily() {
	ts.createCIBASessionApp()

	tokens, _ := ts.cibaTokensFor(cibaSessionClientID, cibaSessionClientSecret)
	sid := ts.assertCarriesTheParticipantsFamily(tokens)

	// The suite's main flow has no Session node, so its tokens name the direct session the login recorded.
	directTokens, _ := ts.cibaTokensFor(cibaClientID, cibaClientSecret)
	directSid := ts.assertCarriesTheParticipantsFamily(directTokens)
	ts.NotEqual(sid, directSid, "the two logins record different sessions")
}

// TestSigningOutRevokesCIBATokens verifies that signing out with the session a CIBA approval started revokes the
// request's tokens, since they carry that session's token family.
func (ts *CIBATestSuite) TestSigningOutRevokesCIBATokens() {
	ts.createCIBASessionApp()
	tokens, sessions := ts.cibaTokensFor(cibaSessionClientID, cibaSessionClientSecret)
	ts.Require().Len(sessions, 1, "the approval started one session")
	active, err := testutils.IntrospectActive(tokens.accessToken, cibaSessionClientID, cibaSessionClientSecret)
	ts.Require().NoError(err)
	ts.Require().True(active, "the access token should be active before sign-out")

	ts.Require().NoError(testutils.SignOutWithSessions(tokens.idToken, sessions))

	active, err = testutils.IntrospectActive(tokens.accessToken, cibaSessionClientID, cibaSessionClientSecret)
	ts.Require().NoError(err)
	ts.False(active, "signing out must revoke the CIBA tokens")
}
