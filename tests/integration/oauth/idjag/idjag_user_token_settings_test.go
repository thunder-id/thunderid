// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package idjag

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	userTokenMockIssuerPort = 8106
	userTokenRSIdentifier   = "https://idjag-user-token-rs.example.com"
	userTokenSubject        = "external-idjag-user"

	userTokenAgentClientID     = "idjag_user_token_agent_client"
	userTokenAgentClientSecret = "idjag_user_token_agent_secret"
	userTokenAgentValidity     = 1800

	userTokenActAppClientID     = "idjag_user_token_act_app_client"
	userTokenActAppClientSecret = "idjag_user_token_act_app_secret"
	userTokenActAppValidity     = 900

	userTokenPlainAppClientID     = "idjag_user_token_plain_app_client"
	userTokenPlainAppClientSecret = "idjag_user_token_plain_app_secret"
	userTokenPlainAppValidity     = 1200

	// userTokenClientValidity is set on every client's client_credentials token settings, which the
	// jwt-bearer grant must not use.
	userTokenClientValidity = 600

	jwtBearerGrantType = "urn:ietf:params:oauth:grant-type:jwt-bearer"
)

// IDJAGUserTokenSettingsTestSuite covers redeeming an ID-JAG with the jwt-bearer grant as a delegated
// agent and as applications with and without includeActClaim. The redeemed token names a user, so it
// carries the client's user access token validity and, where the client acts for the user, an act claim.
type IDJAGUserTokenSettingsTestSuite struct {
	suite.Suite
	client            *http.Client
	ouID              string
	agentTypeSnapshot *testutils.AgentTypeSnapshot
	mockIssuer        *testutils.MockOIDCServer
	idpID             string
	resourceServerID  string
	agentID           string
	actAppID          string
	plainAppID        string
}

func TestIDJAGUserTokenSettingsTestSuite(t *testing.T) {
	suite.Run(t, new(IDJAGUserTokenSettingsTestSuite))
}

func (ts *IDJAGUserTokenSettingsTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "idjag-user-token-ou",
		Name:        "IDJAG User Token OU",
		Description: "OU for ID-JAG user token settings testing",
	})
	ts.Require().NoError(err, "create test OU")
	ts.ouID = ouID

	// The `default` agent type is a singleton shared with every other suite. Snapshot it before
	// pointing it at this suite's OU, so teardown can put it back before that OU is deleted.
	snapshot, err := testutils.SnapshotAgentType()
	ts.Require().NoError(err, "snapshot the default agent type")
	ts.agentTypeSnapshot = snapshot
	_, err = testutils.CreateAgentType(testutils.UserType{
		Handle:      "default",
		DisplayName: "Default",
		OUID:        ts.ouID,
		Schema: map[string]interface{}{
			"name": map[string]interface{}{"type": "string", "required": true, "unique": true},
		},
	})
	ts.Require().NoError(err, "point the default agent type at the test OU")

	mockIssuer, err := testutils.NewMockOIDCServer(
		userTokenMockIssuerPort, "idjag-user-token-client", "idjag-user-token-secret")
	ts.Require().NoError(err, "create mock ID-JAG issuer")
	ts.Require().NoError(mockIssuer.Start(), "start mock ID-JAG issuer")
	ts.mockIssuer = mockIssuer

	idpID, err := testutils.CreateIDP(testutils.IDP{
		Name:        fmt.Sprintf("IDJAG User Token IdP %d", time.Now().UnixNano()),
		Description: "Mock ID-JAG issuer for user token settings testing",
		Type:        "OIDC",
		Properties: []testutils.IDPProperty{
			{Name: "issuer", Value: mockIssuer.GetURL()},
			{Name: "jwks_endpoint", Value: mockIssuer.GetJWKSURL()},
			{Name: "id_jag_enabled", Value: "true"},
		},
	})
	ts.Require().NoError(err, "create trusted ID-JAG issuer")
	ts.idpID = idpID

	rsID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name: "IDJAG User Token RS", Identifier: userTokenRSIdentifier, OUID: ts.ouID,
	}, []testutils.Action{{Name: "Read", Handle: "read", Description: "Read permission"}})
	ts.Require().NoError(err, "create test resource server")
	ts.resourceServerID = rsID

	ts.agentID = ts.create("/agents", map[string]any{
		"ouId":       ts.ouID,
		"type":       "default",
		"attributes": map[string]any{"name": "IDJAG User Token Agent"},
		"delegated":  true,
		"inboundAuthConfig": []map[string]any{{
			"type": "oauth2",
			"config": ts.clientConfig(userTokenAgentClientID, userTokenAgentClientSecret,
				userTokenAgentValidity, []string{"client_credentials", jwtBearerGrantType}),
		}},
	})

	ts.actAppID = ts.createApp("IDJAGUserTokenActApp", userTokenActAppClientID, userTokenActAppClientSecret,
		userTokenActAppValidity, true)
	ts.plainAppID = ts.createApp("IDJAGUserTokenPlainApp", userTokenPlainAppClientID,
		userTokenPlainAppClientSecret, userTokenPlainAppValidity, false)
}

func (ts *IDJAGUserTokenSettingsTestSuite) TearDownSuite() {
	for _, appID := range []string{ts.actAppID, ts.plainAppID} {
		if appID != "" {
			_ = testutils.DeleteApplication(appID)
		}
	}
	if ts.agentID != "" {
		_ = testutils.DeleteAgent(ts.agentID)
	}
	if ts.resourceServerID != "" {
		ts.NoError(testutils.DeleteResourceServerWithChildren(ts.resourceServerID),
			"teardown: delete resource server and its actions")
	}
	if ts.idpID != "" {
		_ = testutils.DeleteIDP(ts.idpID)
	}
	if ts.mockIssuer != nil {
		_ = ts.mockIssuer.Stop()
	}
	// Restore the shared agent type before deleting the OU it points at, or the singleton is left
	// referencing a deleted OU and a later suite's restore fails.
	agentTypeRestored := true
	if ts.agentTypeSnapshot != nil {
		if err := testutils.RestoreAgentType(ts.agentTypeSnapshot); err != nil {
			ts.T().Errorf("teardown: failed to restore the default agent type: %v", err)
			agentTypeRestored = false
		}
	}
	if ts.ouID != "" && agentTypeRestored {
		_ = testutils.DeleteOrganizationUnit(ts.ouID)
	}
}

// clientConfig builds a confidential OAuth client config whose user and client access tokens have
// different validity periods.
func (ts *IDJAGUserTokenSettingsTestSuite) clientConfig(
	clientID, clientSecret string, userValidity int, grantTypes []string) map[string]any {
	return map[string]any{
		"clientId":                clientID,
		"clientSecret":            clientSecret,
		"grantTypes":              grantTypes,
		"tokenEndpointAuthMethod": "client_secret_basic",
		"scopes":                  []string{"read"},
		"token": map[string]any{
			"accessToken": map[string]any{
				"userConfig":   map[string]any{"validityPeriod": userValidity},
				"clientConfig": map[string]any{"validityPeriod": userTokenClientValidity},
			},
		},
	}
}

// createApp creates an application that redeems ID-JAGs, with or without includeActClaim.
func (ts *IDJAGUserTokenSettingsTestSuite) createApp(
	name, clientID, clientSecret string, userValidity int, includeActClaim bool) string {
	config := ts.clientConfig(clientID, clientSecret, userValidity, []string{jwtBearerGrantType})
	config["includeActClaim"] = includeActClaim
	return ts.create("/applications", map[string]any{
		"name":                      name,
		"ouId":                      ts.ouID,
		"type":                      "fullstack",
		"isRegistrationFlowEnabled": false,
		"inboundAuthConfig":         []map[string]any{{"type": "oauth2", "config": config}},
	})
}

// create POSTs a resource and returns its ID, failing the suite unless it is created.
func (ts *IDJAGUserTokenSettingsTestSuite) create(path string, body map[string]any) string {
	ts.T().Helper()
	payload, err := json.Marshal(body)
	ts.Require().NoError(err)
	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+path, bytes.NewReader(payload))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Require().Equalf(http.StatusCreated, resp.StatusCode, "create %s failed: %s", path, string(raw))
	var created map[string]any
	ts.Require().NoError(json.Unmarshal(raw, &created))
	id, _ := created["id"].(string)
	ts.Require().NotEmpty(id)
	return id
}

// redeem signs an ID-JAG for the given client, redeems it with the jwt-bearer grant, and returns the
// decoded access token and the response's expires_in.
func (ts *IDJAGUserTokenSettingsTestSuite) redeem(
	clientID, clientSecret string, extraClaims map[string]any) (*testutils.JWTClaims, int64) {
	ts.T().Helper()
	now := time.Now()
	claims := map[string]interface{}{
		"iss":       ts.mockIssuer.GetURL(),
		"sub":       userTokenSubject,
		"aud":       testutils.TestServerURL,
		"client_id": clientID,
		"exp":       now.Add(5 * time.Minute).Unix(),
		"iat":       now.Unix(),
		"jti":       fmt.Sprintf("user-token-jti-%d", now.UnixNano()),
		"scope":     "read",
		"resource":  userTokenRSIdentifier,
	}
	for k, v := range extraClaims {
		claims[k] = v
	}
	assertion, err := ts.mockIssuer.SignJWT(
		map[string]interface{}{"typ": "oauth-id-jag+jwt", "alg": "RS256", "kid": "oidc-key-1"}, claims)
	ts.Require().NoError(err)

	form := url.Values{}
	form.Set("grant_type", jwtBearerGrantType)
	form.Set("assertion", assertion)
	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+"/oauth2/token",
		strings.NewReader(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Basic "+basicAuth(clientID, clientSecret))
	httpResp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer httpResp.Body.Close()
	var resp IDJAGTokenResponse
	ts.Require().NoError(json.NewDecoder(httpResp.Body).Decode(&resp))
	ts.Require().Equalf(http.StatusOK, httpResp.StatusCode, "%s: %s", resp.Error, resp.ErrorDescription)

	token, err := testutils.DecodeJWT(resp.AccessToken)
	ts.Require().NoError(err)
	ts.Equal(userTokenSubject, token.Sub, "the token names the asserted user")
	return token, resp.ExpiresIn
}

// assertValidity checks that both the token's lifetime and expires_in equal the expected validity.
func (ts *IDJAGUserTokenSettingsTestSuite) assertValidity(token *testutils.JWTClaims, expiresIn int64,
	validity int) {
	ts.T().Helper()
	ts.Equal(float64(validity), token.Exp-token.Iat, "exp - iat must be the user access token validity")
	ts.Equal(int64(validity), expiresIn)
}

func (ts *IDJAGUserTokenSettingsTestSuite) TestAgentRedeemsAnIDJAGForAUser() {
	token, expiresIn := ts.redeem(userTokenAgentClientID, userTokenAgentClientSecret, nil)

	act, ok := token.Additional["act"].(map[string]interface{})
	ts.Require().True(ok, "an agent's token must carry an act claim")
	ts.Equal(ts.agentID, act["sub"], "act.sub must be the agent's ID")
	ts.NotContains(act, "act")
	ts.assertValidity(token, expiresIn, userTokenAgentValidity)
}

// An act claim inside the ID-JAG is not carried into the redeemed token: the redeeming client is the
// only actor recorded.
func (ts *IDJAGUserTokenSettingsTestSuite) TestAgentRedemptionDoesNotChainTheAssertionsActClaim() {
	token, _ := ts.redeem(userTokenAgentClientID, userTokenAgentClientSecret,
		map[string]any{"act": map[string]any{"sub": "upstream-actor"}})

	act, ok := token.Additional["act"].(map[string]interface{})
	ts.Require().True(ok, "an agent's token must carry an act claim")
	ts.Equal(ts.agentID, act["sub"])
	ts.NotContains(act, "act")
}

func (ts *IDJAGUserTokenSettingsTestSuite) TestApplicationWithIncludeActClaimRedeemsAnIDJAG() {
	token, expiresIn := ts.redeem(userTokenActAppClientID, userTokenActAppClientSecret, nil)

	act, ok := token.Additional["act"].(map[string]interface{})
	ts.Require().True(ok, "an application that includes the act claim must get one")
	ts.Equal(ts.actAppID, act["sub"], "act.sub must be the application's ID")
	ts.assertValidity(token, expiresIn, userTokenActAppValidity)
}

func (ts *IDJAGUserTokenSettingsTestSuite) TestApplicationWithoutIncludeActClaimRedeemsAnIDJAG() {
	token, expiresIn := ts.redeem(userTokenPlainAppClientID, userTokenPlainAppClientSecret, nil)

	ts.NotContains(token.Additional, "act")
	ts.assertValidity(token, expiresIn, userTokenPlainAppValidity)
}
