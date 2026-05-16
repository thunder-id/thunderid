/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Per-client `dpopBoundAccessTokens` flag and global enforcement.
// When the flag is true, /oauth2/token MUST reject requests without a DPoP
// proof (invalid_dpop_proof) and MUST issue a DPoP-bound token when one is
// supplied. The flag itself MUST round-trip through the application API and
// the DCR endpoint so admins can read back the value they wrote.

package dpop

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// TestEnforcedClient_NoProof_Rejected — dpopBoundAccessTokens=true on
// the client; /token without a DPoP proof MUST fail with invalid_dpop_proof.
func (ts *DPoPTestSuite) TestEnforcedClient_NoProof_Rejected() {
	code, verifier := ts.obtainAuthorizationCode(enforcedClientID, nil)

	res, err := requestTokenWithDPoP(
		enforcedClientID, enforcedClientSecret, code, dpopRedirectURI, verifier, "", nil)
	ts.Require().NoError(err)
	ts.Equalf(http.StatusBadRequest, res.StatusCode, "body: %s", string(res.Body))
	ts.Require().NotNil(res.Error)
	ts.Equal("invalid_dpop_proof", res.Error.Error)
}

// TestEnforcedClient_WithProof_Succeeds — same client with a valid
// proof gets a DPoP-bound access token.
func (ts *DPoPTestSuite) TestEnforcedClient_WithProof_Succeeds() {
	key, err := testutils.GenerateDPoPKey("PS256")
	ts.Require().NoError(err)

	code, verifier := ts.obtainAuthorizationCode(enforcedClientID, nil)
	proof, err := key.CreateProof(http.MethodPost, tokenEndpoint, testutils.DPoPProofOptions{})
	ts.Require().NoError(err)

	res, err := requestTokenWithDPoP(
		enforcedClientID, enforcedClientSecret, code, dpopRedirectURI, verifier, proof, nil)
	ts.Require().NoError(err)
	ts.Require().Equalf(http.StatusOK, res.StatusCode, "token body: %s", string(res.Body))
	ts.Require().NotNil(res.Token)
	ts.Equal("DPoP", res.Token.TokenType)

	claims, err := testutils.DecodeJWTPayloadMap(res.Token.AccessToken)
	ts.Require().NoError(err)
	cnf, _ := claims["cnf"].(map[string]any)
	ts.Require().NotNil(cnf)
	ts.Equal(key.JKT, cnf["jkt"])
}

// TestVoluntaryClient_NoProof_StillSucceeds — the voluntary client
// (dpopBoundAccessTokens=false) MUST be unaffected by client-flag enforcement.
func (ts *DPoPTestSuite) TestVoluntaryClient_NoProof_StillSucceeds() {
	code, verifier := ts.obtainAuthorizationCode(voluntaryClientID, nil)
	res, err := requestTokenWithDPoP(
		voluntaryClientID, voluntaryClientSecret, code, dpopRedirectURI, verifier, "", nil)
	ts.Require().NoError(err)
	ts.Require().Equalf(http.StatusOK, res.StatusCode, "body: %s", string(res.Body))
	ts.Require().NotNil(res.Token)
	ts.Equal("Bearer", res.Token.TokenType)
}

// TestApplicationAPI_RoundTripsFlag — POST /applications with
// dpopBoundAccessTokens=true; GET echoes it; PUT to false; GET echoes false.
func (ts *DPoPTestSuite) TestApplicationAPI_RoundTripsFlag() {
	uniqueClientID := "dpop_app_roundtrip_client"
	body := map[string]any{
		"name":                      "DPoPRoundTripApp",
		"description":               "round-trip dpopBoundAccessTokens",
		"ouId":                      ts.ouID,
		"authFlowId":                ts.authFlowID,
		"isRegistrationFlowEnabled": false,
		"allowedUserTypes":          []string{"dpop-test-person"},
		"inboundAuthConfig": []map[string]any{
			{
				"type": "oauth2",
				"config": map[string]any{
					"clientId":                uniqueClientID,
					"clientSecret":            "dpop-app-roundtrip-secret",
					"redirectUris":            []string{dpopRedirectURI},
					"grantTypes":              []string{"authorization_code"},
					"responseTypes":           []string{"code"},
					"tokenEndpointAuthMethod": "client_secret_basic",
					"dpopBoundAccessTokens":   true,
				},
			},
		},
	}
	jsonData, err := json.Marshal(body)
	ts.Require().NoError(err)

	req, err := http.NewRequest("POST", testutils.TestServerURL+"/applications", bytes.NewBuffer(jsonData))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	created, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	ts.Require().Equalf(http.StatusCreated, resp.StatusCode, "create body: %s", string(created))

	var createdResp map[string]any
	ts.Require().NoError(json.Unmarshal(created, &createdResp))
	appID, _ := createdResp["id"].(string)
	ts.Require().NotEmpty(appID)
	defer func() { _ = testutils.DeleteApplication(appID) }()

	ts.True(extractDPoPBoundFlag(createdResp), "POST response must echo dpopBoundAccessTokens=true")

	// GET → expect true
	getReq, err := http.NewRequest("GET",
		fmt.Sprintf("%s/applications/%s", testutils.TestServerURL, appID), nil)
	ts.Require().NoError(err)
	getResp, err := ts.client.Do(getReq)
	ts.Require().NoError(err)
	getBody, _ := io.ReadAll(getResp.Body)
	getResp.Body.Close()
	ts.Require().Equalf(http.StatusOK, getResp.StatusCode, "get body: %s", string(getBody))
	var getJSON map[string]any
	ts.Require().NoError(json.Unmarshal(getBody, &getJSON))
	ts.True(extractDPoPBoundFlag(getJSON), "GET must echo dpopBoundAccessTokens=true")

	// PUT to false — full app payload, dpopBoundAccessTokens=false
	body["inboundAuthConfig"].([]map[string]any)[0]["config"].(map[string]any)["dpopBoundAccessTokens"] = false
	putData, err := json.Marshal(body)
	ts.Require().NoError(err)
	putReq, err := http.NewRequest("PUT",
		fmt.Sprintf("%s/applications/%s", testutils.TestServerURL, appID), bytes.NewBuffer(putData))
	ts.Require().NoError(err)
	putReq.Header.Set("Content-Type", "application/json")
	putResp, err := ts.client.Do(putReq)
	ts.Require().NoError(err)
	putBody, _ := io.ReadAll(putResp.Body)
	putResp.Body.Close()
	ts.Require().Equalf(http.StatusOK, putResp.StatusCode, "put body: %s", string(putBody))

	// GET again → expect false (omitempty may drop the field; treat absence as false).
	getReq2, err := http.NewRequest("GET",
		fmt.Sprintf("%s/applications/%s", testutils.TestServerURL, appID), nil)
	ts.Require().NoError(err)
	getResp2, err := ts.client.Do(getReq2)
	ts.Require().NoError(err)
	getBody2, _ := io.ReadAll(getResp2.Body)
	getResp2.Body.Close()
	ts.Require().Equal(http.StatusOK, getResp2.StatusCode)
	var getJSON2 map[string]any
	ts.Require().NoError(json.Unmarshal(getBody2, &getJSON2))
	ts.False(extractDPoPBoundFlag(getJSON2), "after PUT to false, GET must report false (or absent)")
}

// extractDPoPBoundFlag walks a serialised application payload (POST/GET/PUT
// response) to fish out inboundAuthConfig[0].config.dpopBoundAccessTokens.
// Returns false when the flag is absent — that matches `omitempty` semantics
// on the response and the documented default.
func extractDPoPBoundFlag(app map[string]any) bool {
	cfgs, ok := app["inboundAuthConfig"].([]any)
	if !ok || len(cfgs) == 0 {
		return false
	}
	first, ok := cfgs[0].(map[string]any)
	if !ok {
		return false
	}
	cfg, ok := first["config"].(map[string]any)
	if !ok {
		return false
	}
	v, _ := cfg["dpopBoundAccessTokens"].(bool)
	return v
}

// TestDCR_RoundTripsFlag — RFC 7591 dynamic client registration
// preserves the dpop_bound_access_tokens flag on register and read.
func (ts *DPoPTestSuite) TestDCR_RoundTripsFlag() {
	registration := map[string]any{
		"ou_id":                     "decl-ou-1",
		"redirect_uris":             []string{"https://dpop-dcr.example.com/callback"},
		"client_name":               "DPoP DCR Round Trip",
		"grant_types":               []string{"authorization_code", "refresh_token"},
		"response_types":            []string{"code"},
		"token_endpoint_auth_method": "client_secret_basic",
		"dpop_bound_access_tokens":  true,
	}

	body, err := json.Marshal(registration)
	ts.Require().NoError(err)

	req, err := http.NewRequest("POST",
		testutils.TestServerURL+"/oauth2/dcr/register", bytes.NewReader(body))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	token, err := testutils.GetAccessToken()
	ts.Require().NoError(err)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	ts.Require().Equalf(http.StatusCreated, resp.StatusCode, "DCR body: %s", string(respBody))

	var registerResp map[string]any
	ts.Require().NoError(json.Unmarshal(respBody, &registerResp))
	flag, _ := registerResp["dpop_bound_access_tokens"].(bool)
	ts.True(flag, "DCR response must echo dpop_bound_access_tokens=true")

	appID, _ := registerResp["app_id"].(string)
	ts.Require().NotEmpty(appID, "DCR response must include app_id")
	defer func() { _ = testutils.DeleteApplication(appID) }()

	// GET on the application API returns the same value (DCR persists into the
	// shared applications table).
	getReq, err := http.NewRequest("GET",
		fmt.Sprintf("%s/applications/%s", testutils.TestServerURL, appID), nil)
	ts.Require().NoError(err)
	getResp, err := ts.client.Do(getReq)
	ts.Require().NoError(err)
	getBody, _ := io.ReadAll(getResp.Body)
	getResp.Body.Close()
	ts.Require().Equalf(http.StatusOK, getResp.StatusCode, "GET body: %s", string(getBody))
	var getJSON map[string]any
	ts.Require().NoError(json.Unmarshal(getBody, &getJSON))
	ts.True(extractDPoPBoundFlag(getJSON),
		"after DCR with dpop_bound_access_tokens=true, application GET must report true")
}
