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

package revocation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	testServerURL = "https://localhost:8095"

	clientIDOwner                             = "revoke_owner_client"
	secretOwner                               = "revoke_owner_secret"
	clientIDOther                             = "revoke_other_client"
	secretOther                               = "revoke_other_secret"
	revocationDefaultResourceServerIdentifier = "https://revocation-default.example.com"
	revokeEndpoint                            = testServerURL + "/oauth2/revoke"
	tokenEndpoint                             = testServerURL + "/oauth2/token"
	introspectEndpoint                        = testServerURL + "/oauth2/introspect"
)

// RevocationTestSuite exercises the RFC 7009 POST /oauth2/revoke endpoint end-to-end:
// real client authentication, real signed tokens, and the runtime persistent database.
type RevocationTestSuite struct {
	suite.Suite
	client           *http.Client
	ouID             string
	appIDOwner       string
	appIDOther       string
	resourceServerID string
}

func TestRevocationTestSuite(t *testing.T) {
	suite.Run(t, new(RevocationTestSuite))
}

func (ts *RevocationTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "revocation-test-ou",
		Name:        "Revocation Test OU",
		Description: "Organization unit for token revocation integration tests",
		Parent:      nil,
	})
	ts.Require().NoError(err, "failed to create test organization unit")
	ts.ouID = ouID

	resourceServerID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:        "Revocation Default Resource Server",
		Description: "Resource server for revocation integration tests",
		Identifier:  revocationDefaultResourceServerIdentifier,
		OUID:        ts.ouID,
	}, []testutils.Action{})
	ts.Require().NoError(err, "failed to create resource server")
	ts.resourceServerID = resourceServerID

	ts.appIDOwner = ts.createApp("RevokeOwnerApp", clientIDOwner, secretOwner)
	ts.appIDOther = ts.createApp("RevokeOtherApp", clientIDOther, secretOther)
}

func (ts *RevocationTestSuite) TearDownSuite() {
	if ts.appIDOwner != "" {
		ts.deleteApp(ts.appIDOwner)
	}
	if ts.appIDOther != "" {
		ts.deleteApp(ts.appIDOther)
	}
	if ts.resourceServerID != "" {
		if err := testutils.DeleteResourceServer(ts.resourceServerID); err != nil {
			ts.T().Logf("Failed to delete resource server: %v", err)
		}
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete test organization unit: %v", err)
		}
	}
}

func (ts *RevocationTestSuite) createApp(name, clientID, clientSecret string) string {
	app := map[string]interface{}{
		"name":                      name,
		"description":               "Application for token revocation integration tests",
		"ouId":                      ts.ouID,
		"isRegistrationFlowEnabled": false,
		"inboundAuthConfig": []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                clientID,
					"clientSecret":            clientSecret,
					"redirectUris":            []string{"https://localhost:3000"},
					"grantTypes":              []string{"client_credentials", "urn:ietf:params:oauth:grant-type:token-exchange"},
					"tokenEndpointAuthMethod": "client_secret_basic",
				},
			},
		},
	}

	jsonData, err := json.Marshal(app)
	ts.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPost, testServerURL+"/applications", bytes.NewBuffer(jsonData))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	ts.Require().Equalf(http.StatusCreated, resp.StatusCode, "create app failed: %s", string(body))

	var created map[string]interface{}
	ts.Require().NoError(json.Unmarshal(body, &created))
	id, ok := created["id"].(string)
	ts.Require().True(ok, "application id not found in response")
	return id
}

func (ts *RevocationTestSuite) deleteApp(appID string) {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/applications/%s", testServerURL, appID), nil)
	if err != nil {
		ts.T().Logf("Failed to build delete request: %v", err)
		return
	}
	resp, err := ts.client.Do(req)
	if err != nil {
		ts.T().Logf("Failed to delete application: %v", err)
		return
	}
	defer resp.Body.Close()
}

// getAccessToken obtains a fresh client_credentials access token for the given client.
func (ts *RevocationTestSuite) getAccessToken(clientID, clientSecret string) string {
	req, err := http.NewRequest(http.MethodPost, tokenEndpoint,
		strings.NewReader("grant_type=client_credentials&resource="+url.QueryEscape(
			revocationDefaultResourceServerIdentifier)))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&body))
	token, ok := body["access_token"].(string)
	ts.Require().True(ok, "access_token not found")
	return token
}

// revoke posts a revocation request. When auth is true, client_secret_basic credentials are attached.
func (ts *RevocationTestSuite) revoke(token, clientID, clientSecret string, auth bool) *http.Response {
	form := ""
	if token != "" {
		form = "token=" + token
	}
	req, err := http.NewRequest(http.MethodPost, revokeEndpoint, strings.NewReader(form))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if auth {
		req.SetBasicAuth(clientID, clientSecret)
	}
	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	return resp
}

// introspectActive posts an introspection request for the token and returns its "active" status.
func (ts *RevocationTestSuite) introspectActive(token, clientID, clientSecret string) bool {
	req, err := http.NewRequest(http.MethodPost, introspectEndpoint, strings.NewReader("token="+token))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&body))
	active, _ := body["active"].(bool)
	return active
}

// exchange performs an RFC 8693 token exchange using the given self-issued access token as the
// subject_token, authenticated with client_secret_basic.
func (ts *RevocationTestSuite) exchange(subjectToken, clientID, clientSecret string) *http.Response {
	form := url.Values{}
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	form.Set("subject_token", subjectToken)
	form.Set("subject_token_type", "urn:ietf:params:oauth:token-type:access_token")
	form.Set("resource", revocationDefaultResourceServerIdentifier)

	req, err := http.NewRequest(http.MethodPost, tokenEndpoint, strings.NewReader(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	return resp
}

func decodeError(resp *http.Response) string {
	var body map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if e, ok := body["error"].(string); ok {
		return e
	}
	return ""
}

// A client can revoke its own token; RFC 7009 success is HTTP 200.
func (ts *RevocationTestSuite) TestRevoke_OwnTokenSucceeds() {
	token := ts.getAccessToken(clientIDOwner, secretOwner)
	resp := ts.revoke(token, clientIDOwner, secretOwner, true)
	defer resp.Body.Close()
	ts.Assert().Equal(http.StatusOK, resp.StatusCode)
}

// Revocation is idempotent — revoking the same token again still returns 200.
func (ts *RevocationTestSuite) TestRevoke_IsIdempotent() {
	token := ts.getAccessToken(clientIDOwner, secretOwner)

	resp1 := ts.revoke(token, clientIDOwner, secretOwner, true)
	resp1.Body.Close()
	ts.Assert().Equal(http.StatusOK, resp1.StatusCode)

	resp2 := ts.revoke(token, clientIDOwner, secretOwner, true)
	resp2.Body.Close()
	ts.Assert().Equal(http.StatusOK, resp2.StatusCode)
}

// An invalid/unknown token is a successful no-op (RFC 7009 §2.2) — HTTP 200.
func (ts *RevocationTestSuite) TestRevoke_InvalidTokenIsNoOp() {
	resp := ts.revoke("not-a-real-jwt", clientIDOwner, secretOwner, true)
	defer resp.Body.Close()
	ts.Assert().Equal(http.StatusOK, resp.StatusCode)
}

// A missing token parameter is a protocol error — 400 invalid_request.
func (ts *RevocationTestSuite) TestRevoke_MissingTokenReturnsInvalidRequest() {
	resp := ts.revoke("", clientIDOwner, secretOwner, true)
	defer resp.Body.Close()
	ts.Assert().Equal(http.StatusBadRequest, resp.StatusCode)
	ts.Assert().Equal("invalid_request", decodeError(resp))
}

// Invalid client credentials are rejected — 401 invalid_client (RFC 7009).
func (ts *RevocationTestSuite) TestRevoke_InvalidClientAuthReturnsInvalidClient() {
	token := ts.getAccessToken(clientIDOwner, secretOwner)
	resp := ts.revoke(token, clientIDOwner, "wrong_secret", true)
	defer resp.Body.Close()
	ts.Assert().Equal(http.StatusUnauthorized, resp.StatusCode)
	ts.Assert().Equal("invalid_client", decodeError(resp))
}

// A request with no client authentication cannot identify the client, so the shared clientauth
// middleware rejects it with 400 invalid_request — before any client is identified. (A request that
// identifies a client but presents the wrong secret returns 401 invalid_client; see above.)
func (ts *RevocationTestSuite) TestRevoke_MissingClientAuthReturnsInvalidRequest() {
	token := ts.getAccessToken(clientIDOwner, secretOwner)
	resp := ts.revoke(token, "", "", false)
	defer resp.Body.Close()
	ts.Assert().Equal(http.StatusBadRequest, resp.StatusCode)
	ts.Assert().Equal("invalid_request", decodeError(resp))
}

// M2 deny-list enforcement: a token is active before revocation and inactive afterwards when
// introspected on the AS hot path.
func (ts *RevocationTestSuite) TestIntrospect_ReflectsRevocation() {
	token := ts.getAccessToken(clientIDOwner, secretOwner)

	ts.Assert().True(ts.introspectActive(token, clientIDOwner, secretOwner),
		"token should be active before revocation")

	resp := ts.revoke(token, clientIDOwner, secretOwner, true)
	resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode)

	ts.Assert().False(ts.introspectActive(token, clientIDOwner, secretOwner),
		"token should be inactive after revocation")
}

// M2 deny-list enforcement on token exchange: a self-issued subject token is accepted for exchange
// before revocation and rejected with invalid_request afterwards.
func (ts *RevocationTestSuite) TestTokenExchange_RejectsRevokedSubjectToken() {
	subject := ts.getAccessToken(clientIDOwner, secretOwner)

	// Before revocation the subject token is accepted for exchange.
	resp := ts.exchange(subject, clientIDOwner, secretOwner)
	resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode, "exchange should succeed before revocation")

	revokeResp := ts.revoke(subject, clientIDOwner, secretOwner, true)
	revokeResp.Body.Close()
	ts.Require().Equal(http.StatusOK, revokeResp.StatusCode)

	// After revocation the subject token is rejected on the exchange hot path.
	resp2 := ts.exchange(subject, clientIDOwner, secretOwner)
	defer resp2.Body.Close()
	ts.Assert().Equal(http.StatusBadRequest, resp2.StatusCode)
	ts.Assert().Equal("invalid_request", decodeError(resp2))
}

// A client cannot revoke a token issued to a different client — 400 invalid_grant.
func (ts *RevocationTestSuite) TestRevoke_OtherClientsTokenReturnsInvalidGrant() {
	token := ts.getAccessToken(clientIDOwner, secretOwner)
	resp := ts.revoke(token, clientIDOther, secretOther, true)
	defer resp.Body.Close()
	ts.Assert().Equal(http.StatusBadRequest, resp.StatusCode)
	ts.Assert().Equal("invalid_grant", decodeError(resp))
}
