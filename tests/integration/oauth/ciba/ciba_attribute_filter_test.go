// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ciba

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
	cibaAttrFilterClientID     = "ciba_attr_filter_test_client"
	cibaAttrFilterClientSecret = "ciba_attr_filter_test_secret"
)

// createCIBAAppWithAttributes creates a CIBA-enabled application bound to the shared test auth flow,
// with the given token.accessToken.userConfig.attributes allow-list.
func (ts *CIBATestSuite) createCIBAAppWithAttributes(clientID, clientSecret string, allowedAttributes []string) string {
	app := map[string]interface{}{
		"name":                      "CIBAAttrFilterTestApp",
		"description":               "Application for CIBA attribute allow-list integration test",
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
					"grantTypes":              []string{cibaGrantType, "refresh_token"},
					"tokenEndpointAuthMethod": "client_secret_basic",
					"token": map[string]interface{}{
						"accessToken": map[string]interface{}{
							"userConfig": map[string]interface{}{
								"attributes": allowedAttributes,
							},
						},
					},
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
		ts.T().Fatalf("Failed to create CIBA attribute-filter application. Status: %d, Response: %s",
			resp.StatusCode, string(bodyBytes))
	}

	var respData map[string]interface{}
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&respData))
	return respData["id"].(string)
}

// cibaBackchannelAuthorizeAs is cibaBackchannelAuthorize parametrized by client credentials.
func (ts *CIBATestSuite) cibaBackchannelAuthorizeAs(
	clientID, clientSecret, loginHint, scope string,
) (int, cibaBackchannelResponse) {
	form := url.Values{}
	form.Set("login_hint", loginHint)
	form.Set("scope", scope)

	req, err := http.NewRequest("POST", testutils.TestServerURL+cibaBackchannelEndpoint,
		strings.NewReader(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	var body cibaBackchannelResponse
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return resp.StatusCode, body
}

// cibaPollTokenAs is cibaPollToken parametrized by client credentials.
func (ts *CIBATestSuite) cibaPollTokenAs(clientID, clientSecret, authReqID string) cibaTokenResult {
	form := url.Values{}
	form.Set("grant_type", cibaGrantType)
	form.Set("auth_req_id", authReqID)

	req, err := http.NewRequest("POST", testutils.TestServerURL+cibaTokenEndpoint,
		strings.NewReader(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	var raw map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&raw)

	res := cibaTokenResult{statusCode: resp.StatusCode}
	if v, ok := raw["access_token"].(string); ok {
		res.accessToken = v
	}
	if v, ok := raw["error"].(string); ok {
		res.errorCode = v
	}
	return res
}

// TestCIBAGrantFlow_AttributeAllowList verifies that the CIBA-issued access token only carries the
// user attributes selected by the app's token.accessToken.userConfig.attributes allow-list, mirroring
// the allow-list enforcement already proven for the authorization_code grant.
func (ts *CIBATestSuite) TestCIBAGrantFlow_AttributeAllowList() {
	appID := ts.createCIBAAppWithAttributes(cibaAttrFilterClientID, cibaAttrFilterClientSecret, []string{"email"})
	defer func() { _ = testutils.DeleteApplication(appID) }()

	status, bcResp := ts.cibaBackchannelAuthorizeAs(cibaAttrFilterClientID, cibaAttrFilterClientSecret,
		cibaTestUsername, "openid")
	ts.Require().Equal(http.StatusOK, status, "bc-authorize should succeed")
	ts.Require().NotEmpty(bcResp.AuthReqID)

	var executionID, inviteToken string
	ts.Require().Eventually(func() bool {
		msg := ts.mockServer.GetLastMessage()
		if msg == nil {
			return false
		}
		if extractCIBALinkParam(msg.Message, "auth_req_id") != bcResp.AuthReqID {
			return false
		}
		executionID = extractCIBALinkParam(msg.Message, "executionId")
		inviteToken = extractCIBALinkParam(msg.Message, "inviteToken")
		return executionID != "" && inviteToken != ""
	}, 5*time.Second, 100*time.Millisecond, "Expected CIBA notification carrying the executionId")

	resumeStep, err := testutils.ExecuteAuthenticationFlow(executionID,
		map[string]string{"inviteToken": inviteToken}, "")
	ts.Require().NoError(err)
	flowStep, err := testutils.ExecuteAuthenticationFlow(executionID, map[string]string{
		"username": cibaTestUsername,
		"password": cibaTestPassword,
	}, "action_001", resumeStep.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus)
	ts.Require().NotEmpty(flowStep.Assertion)

	ts.Require().Equal(http.StatusOK, ts.cibaPostCallback(bcResp.AuthReqID, flowStep.Assertion))

	tokenRes := ts.cibaPollTokenAs(cibaAttrFilterClientID, cibaAttrFilterClientSecret, bcResp.AuthReqID)
	if tokenRes.statusCode == http.StatusBadRequest && tokenRes.errorCode == "slow_down" {
		time.Sleep(cibaPollIntervalSeconds * time.Second)
		tokenRes = ts.cibaPollTokenAs(cibaAttrFilterClientID, cibaAttrFilterClientSecret, bcResp.AuthReqID)
	}
	ts.Require().Equal(http.StatusOK, tokenRes.statusCode, "AUTHENTICATED request should issue tokens")
	ts.Require().NotEmpty(tokenRes.accessToken)

	claims, err := testutils.DecodeJWT(tokenRes.accessToken)
	ts.Require().NoError(err)

	ts.Assert().Equal("ciba_test_user@example.com", claims.Additional["email"],
		"allow-listed attribute should be present")
	ts.Assert().NotContains(claims.Additional, "mobile_number",
		"non-allow-listed attribute must be excluded")
	ts.Assert().NotContains(claims.Additional, "username",
		"non-allow-listed attribute must be excluded")
}
