// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ciba

import (
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

// cibaIDTokenFor runs a full CIBA request as the given client and returns the issued ID token.
func (ts *CIBATestSuite) cibaIDTokenFor(clientID, clientSecret string) string {
	form := url.Values{}
	form.Set("login_hint", cibaTestUsername)
	form.Set("scope", "openid")
	status, bcResp := ts.cibaBackchannelAuthorizeForm(form, clientID, clientSecret)
	ts.Require().Equal(http.StatusOK, status, "bc-authorize should succeed")
	ts.Require().NotEmpty(bcResp.AuthReqID)

	ts.completeCIBAFlow(bcResp.AuthReqID)

	tokenRes := ts.cibaPollTokenAs(bcResp.AuthReqID, "", clientID, clientSecret)
	if tokenRes.statusCode == http.StatusBadRequest && tokenRes.errorCode == "slow_down" {
		time.Sleep(cibaPollIntervalSeconds * time.Second)
		tokenRes = ts.cibaPollTokenAs(bcResp.AuthReqID, "", clientID, clientSecret)
	}
	ts.Require().Equal(http.StatusOK, tokenRes.statusCode, "AUTHENTICATED request should issue tokens")
	ts.Require().NotEmpty(tokenRes.idToken, "openid scope should yield an ID token")
	return tokenRes.idToken
}

// TestCIBAIDTokenCarriesSidWhenFlowHasSession verifies the CIBA grant carries the SSO session id as
// sid when the authentication flow establishes a session, and omits it when the flow has none.
func (ts *CIBATestSuite) TestCIBAIDTokenCarriesSidWhenFlowHasSession() {
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

	withSession := ts.cibaIDTokenFor(cibaSessionClientID, cibaSessionClientSecret)
	claims, err := testutils.DecodeJWTPayloadMap(withSession)
	ts.Require().NoError(err)
	sid, _ := claims["sid"].(string)
	ts.NotEmpty(sid, "a CIBA flow with a Session node must issue an ID token carrying sid")

	// The suite's main flow has no Session node, so its ID token must not carry sid.
	withoutSession := ts.cibaIDTokenFor(cibaClientID, cibaClientSecret)
	claims, err = testutils.DecodeJWTPayloadMap(withoutSession)
	ts.Require().NoError(err)
	_, has := claims["sid"]
	ts.False(has, "a CIBA flow without a Session node must not issue a sid")
}
