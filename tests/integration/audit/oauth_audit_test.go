// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	clientCredentialsGrant = "client_credentials"
	tokenExchangeGrant     = "urn:ietf:params:oauth:grant-type:token-exchange"
	accessTokenType        = "urn:ietf:params:oauth:token-type:access_token"

	auditClientSecret      = "AuditOAuthClientSecret0123456789"
	auditOtherClientSecret = "AuditOAuthOtherSecret0123456789"
	auditWrongSecret       = "AuditOAuthWrongSecret0123456789"
)

// auditClient is an OAuth client the suite created, with the ID of the application it belongs to.
type auditClient struct {
	appID    string
	clientID string
	secret   string
}

// oauthLine is the part of an audit line only the OAuth events fill.
type oauthLine struct {
	Details map[string]string `json:"details"`
}

// A token issued to a client writes one token.issue line by the client, named by the ID of its
// application and never by its client ID, with the grant type and no subject. A token exchanged for one
// the admin holds names the admin as subject. No line carries a token, a secret or the client ID.
func (ts *AuditLogTestSuite) TestATokenIssuedToAClientWritesOneLine() {
	client := ts.createClient("issue", auditClientSecret, clientCredentialsGrant, tokenExchangeGrant)
	adminToken, err := testutils.GetAccessToken()
	ts.Require().NoError(err)

	issued, body := ts.oauthCall("/oauth2/token", url.Values{"grant_type": {clientCredentialsGrant}}, client, nil)
	ts.Require().Equal(http.StatusOK, issued.StatusCode, string(body))
	accessToken := ts.accessTokenOf(body)
	exchanged, body := ts.oauthCall("/oauth2/token", url.Values{
		"grant_type":         {tokenExchangeGrant},
		"subject_token":      {adminToken},
		"subject_token_type": {accessTokenType},
		"resource":           {testutils.SystemResourceIdentifier},
	}, client, nil)
	ts.Require().Equal(http.StatusOK, exchanged.StatusCode, string(body))
	exchangedToken := ts.accessTokenOf(body)

	lines := ts.linesOf(issued, exchanged)
	for i, tc := range []struct {
		grantType string
		subject   testutils.AuditRef
	}{
		{clientCredentialsGrant, testutils.AuditRef{}},
		{tokenExchangeGrant, testutils.AuditRef{Type: "user", ID: ts.adminUserID()}},
	} {
		line := lines[i]
		ts.True(strings.HasPrefix(line.Raw, auditLinePrefix), line.Raw)
		ts.Equal("token.issue", line.Event)
		ts.Equal("issue", line.Action)
		ts.Equal(testutils.AuditRef{Type: "application", ID: client.appID}, line.Actor, tc.grantType)
		ts.Equal(testutils.AuditRef{}, line.OnBehalfOf, tc.grantType)
		ts.Equal(testutils.AuditRef{Type: "token"}, line.Target, tc.grantType)
		ts.Equal(tc.subject, line.Subject, tc.grantType)
		ts.Equal(map[string]string{"result": "succeeded"}, line.Outcome, tc.grantType)
		ts.Equal(map[string]string{"grant_type": tc.grantType}, ts.detailsOf(line), tc.grantType)
		ts.Equal(testUserAgent, line.Client["user_agent"], tc.grantType)
		ts.requireNoCredential(line, client, accessToken, exchangedToken, adminToken)
	}
}

// A request whose client never authenticates writes one client.authenticate line and nothing else, on
// every endpoint that authenticates a client: anonymous, with the client ID the request named as the
// target, denied when the client is not valid and failed when the request is malformed. No line
// carries the secret it presented.
func (ts *AuditLogTestSuite) TestEveryRefusedClientAuthenticationWritesOnlyItsLine() {
	client := ts.createClient("refused", auditClientSecret, clientCredentialsGrant)
	unknown := ts.prefix + "unknown-client"
	wrongSecret := auditClient{clientID: client.clientID, secret: auditWrongSecret}
	cases := []struct {
		name     string
		path     string
		form     url.Values
		client   auditClient
		clientID string
		outcome  map[string]string
	}{
		{
			name: "a wrong secret at the token endpoint", path: "/oauth2/token",
			form: url.Values{"grant_type": {clientCredentialsGrant}}, client: wrongSecret, clientID: client.clientID,
			outcome: map[string]string{"result": "denied", "reason": "invalid_client"},
		},
		{
			name: "an unknown client at the token endpoint", path: "/oauth2/token",
			form:     url.Values{"grant_type": {clientCredentialsGrant}, "client_id": {unknown}, "client_secret": {auditWrongSecret}},
			clientID: unknown, outcome: map[string]string{"result": "denied", "reason": "invalid_client"},
		},
		{
			name: "no client at the token endpoint", path: "/oauth2/token",
			form:    url.Values{"grant_type": {clientCredentialsGrant}},
			outcome: map[string]string{"result": "failed", "reason": "invalid_request"},
		},
		{
			name: "a wrong secret at the revocation endpoint", path: "/oauth2/revoke",
			form: url.Values{"token": {"not-a-token"}}, client: wrongSecret, clientID: client.clientID,
			outcome: map[string]string{"result": "denied", "reason": "invalid_client"},
		},
		{
			name: "a wrong secret at the introspection endpoint", path: "/oauth2/introspect",
			form: url.Values{"token": {"not-a-token"}}, client: wrongSecret, clientID: client.clientID,
			outcome: map[string]string{"result": "denied", "reason": "invalid_client"},
		},
		{
			name: "a wrong secret at the pushed authorization request endpoint", path: "/oauth2/par",
			form: url.Values{"response_type": {"code"}}, client: wrongSecret, clientID: client.clientID,
			outcome: map[string]string{"result": "denied", "reason": "invalid_client"},
		},
		{
			name: "a wrong secret at the device authorization endpoint", path: "/oauth2/device-authorize",
			form: url.Values{"scope": {"openid"}}, client: wrongSecret, clientID: client.clientID,
			outcome: map[string]string{"result": "denied", "reason": "invalid_client"},
		},
		{
			name: "a wrong secret at the backchannel authentication endpoint", path: "/oauth2/bc-authorize",
			form: url.Values{"scope": {"openid"}}, client: wrongSecret, clientID: client.clientID,
			outcome: map[string]string{"result": "denied", "reason": "invalid_client"},
		},
	}

	responses := make([]*http.Response, len(cases))
	for i, tc := range cases {
		response, body := ts.oauthCall(tc.path, tc.form, tc.client, nil)
		ts.Require().Contains([]int{http.StatusBadRequest, http.StatusUnauthorized}, response.StatusCode,
			"%s: %s", tc.name, string(body))
		responses[i] = response
	}

	lines := ts.linesOf(responses...)
	for i, tc := range cases {
		line := lines[i]
		ts.Equal("client.authenticate", line.Event, tc.name)
		ts.Equal("authenticate", line.Action, tc.name)
		ts.Equal(testutils.AuditRef{Type: "anonymous"}, line.Actor, tc.name)
		ts.Equal(testutils.AuditRef{Type: "application", ID: tc.clientID}, line.Target, tc.name)
		ts.Equal(testutils.AuditRef{}, line.Subject, tc.name)
		ts.Equal(tc.outcome, line.Outcome, tc.name)
		ts.Empty(ts.detailsOf(line), tc.name)
		ts.NotContains(line.Raw, auditWrongSecret, "%s: a presented secret is never recorded", tc.name)
		ts.NotContains(line.Raw, auditClientSecret, "%s: a client secret is never recorded", tc.name)
		ts.NotContains(line.Raw, client.appID, "%s: a refusal names only what the request presented", tc.name)
	}
}

// A token request the authenticated client may not make writes one token.issue line by the client:
// denied when the client is refused, failed when the request is malformed. A grant type the server does
// not know is not recorded, since it is text the caller chose.
func (ts *AuditLogTestSuite) TestEveryRefusedTokenRequestRecordsItsOutcome() {
	client := ts.createClient("token-refused", auditClientSecret, clientCredentialsGrant)
	unknownGrant := "urn:example:" + ts.prefix + "grant"
	cases := []struct {
		name    string
		form    url.Values
		header  http.Header
		outcome map[string]string
		details map[string]string
	}{
		{
			name: "a grant type the client may not use", form: url.Values{"grant_type": {tokenExchangeGrant}},
			outcome: map[string]string{"result": "denied", "reason": "unauthorized_client"},
			details: map[string]string{"grant_type": tokenExchangeGrant},
		},
		{
			name: "a grant type the server does not know", form: url.Values{"grant_type": {unknownGrant}},
			outcome: map[string]string{"result": "failed", "reason": "unsupported_grant_type"},
		},
		{
			name: "no grant type", form: url.Values{"scope": {"openid"}},
			outcome: map[string]string{"result": "failed", "reason": "invalid_request"},
		},
		{
			name: "two DPoP proofs", form: url.Values{"grant_type": {clientCredentialsGrant}},
			header:  http.Header{"Dpop": {"proof-one", "proof-two"}},
			outcome: map[string]string{"result": "failed", "reason": "invalid_dpop_proof"},
			details: map[string]string{"grant_type": clientCredentialsGrant},
		},
	}

	responses := make([]*http.Response, len(cases))
	for i, tc := range cases {
		response, body := ts.oauthCall("/oauth2/token", tc.form, client, tc.header)
		ts.Require().Contains([]int{http.StatusBadRequest, http.StatusUnauthorized}, response.StatusCode,
			"%s: %s", tc.name, string(body))
		responses[i] = response
	}

	lines := ts.linesOf(responses...)
	for i, tc := range cases {
		line := lines[i]
		ts.Equal("token.issue", line.Event, tc.name)
		ts.Equal(testutils.AuditRef{Type: "application", ID: client.appID}, line.Actor, tc.name)
		ts.Equal(testutils.AuditRef{Type: "token"}, line.Target, tc.name)
		ts.Equal(testutils.AuditRef{}, line.Subject, tc.name)
		ts.Equal(tc.outcome, line.Outcome, tc.name)
		if tc.details == nil {
			ts.Empty(ts.detailsOf(line), tc.name)
		} else {
			ts.Equal(tc.details, ts.detailsOf(line), tc.name)
		}
		ts.NotContains(line.Raw, unknownGrant, "%s: text the caller chose is never recorded", tc.name)
		ts.NotContains(line.Raw, "proof-one", "%s: a DPoP proof is never recorded", tc.name)
		ts.requireNoCredential(line, client)
	}
}

// Every revocation request writes one token.revoke line by the client. A revoked token and one the
// server never issued succeed alike, so the log never tells whether a token existed; another client's
// token is denied. No line carries the token.
func (ts *AuditLogTestSuite) TestEveryRevocationRecordsItsOutcome() {
	owner := ts.createClient("revoke-owner", auditClientSecret, clientCredentialsGrant)
	other := ts.createClient("revoke-other", auditOtherClientSecret, clientCredentialsGrant)
	issued, body := ts.oauthCall("/oauth2/token", url.Values{"grant_type": {clientCredentialsGrant}}, owner, nil)
	ts.Require().Equal(http.StatusOK, issued.StatusCode, string(body))
	ownerToken := ts.accessTokenOf(body)
	const unknownToken = "eyJhbGciOiJSUzI1NiJ9.eyJqdGkiOiJ1bmtub3duIn0.dW5rbm93bi1zaWduYXR1cmU"

	cases := []struct {
		name    string
		client  auditClient
		form    url.Values
		status  int
		outcome map[string]string
	}{
		{
			name: "another client's token", client: other, form: url.Values{"token": {ownerToken}},
			status: http.StatusBadRequest, outcome: map[string]string{"result": "denied", "reason": "invalid_grant"},
		},
		{
			name: "the client's own token", client: owner, form: url.Values{"token": {ownerToken}},
			status: http.StatusOK, outcome: map[string]string{"result": "succeeded"},
		},
		{
			name: "a token the server never issued", client: owner, form: url.Values{"token": {unknownToken}},
			status: http.StatusOK, outcome: map[string]string{"result": "succeeded"},
		},
		{
			name: "no token", client: owner, form: url.Values{"token_type_hint": {"access_token"}},
			status: http.StatusBadRequest, outcome: map[string]string{"result": "failed", "reason": "invalid_request"},
		},
	}

	responses := make([]*http.Response, len(cases))
	for i, tc := range cases {
		response, body := ts.oauthCall("/oauth2/revoke", tc.form, tc.client, nil)
		ts.Require().Equal(tc.status, response.StatusCode, "%s: %s", tc.name, string(body))
		responses[i] = response
	}

	lines := ts.linesOf(responses...)
	for i, tc := range cases {
		line := lines[i]
		ts.Equal("token.revoke", line.Event, tc.name)
		ts.Equal("revoke", line.Action, tc.name)
		ts.Equal(testutils.AuditRef{Type: "application", ID: tc.client.appID}, line.Actor, tc.name)
		ts.Equal(testutils.AuditRef{Type: "token"}, line.Target, tc.name)
		ts.Equal(testutils.AuditRef{}, line.Subject, tc.name)
		ts.Equal(tc.outcome, line.Outcome, tc.name)
		ts.Empty(ts.detailsOf(line), tc.name)
		ts.NotContains(line.Raw, unknownToken, "%s: a token is never recorded", tc.name)
		ts.requireNoCredential(line, owner, ownerToken)
		ts.requireNoCredential(line, other)
	}
}

// createClient creates an application in an organization unit of its own whose OAuth client
// authenticates with secret in Basic credentials and may use grantTypes, removed when the test ends.
func (ts *AuditLogTestSuite) createClient(name, secret string, grantTypes ...string) auditClient {
	ts.T().Helper()
	clientID := ts.prefix + name
	appID, err := testutils.CreateApplication(testutils.Application{
		OUID: ts.createOrganizationUnit(name),
		Name: "Audit OAuth " + ts.prefix + name,
		InboundAuthConfig: []map[string]interface{}{{"type": "oauth2", "config": map[string]interface{}{
			"clientId":                clientID,
			"clientSecret":            secret,
			"grantTypes":              grantTypes,
			"tokenEndpointAuthMethod": "client_secret_basic",
		}}},
	})
	ts.Require().NoError(err, "failed to create the application of client %s", name)
	ts.T().Cleanup(func() { _ = testutils.DeleteApplication(appID) })
	return auditClient{appID: appID, clientID: clientID, secret: secret}
}

// oauthCall posts form to an OAuth endpoint at path, with the Basic credentials of client when it has a
// client ID, and returns the response and its body.
func (ts *AuditLogTestSuite) oauthCall(path string, form url.Values, client auditClient, header http.Header) (
	*http.Response, []byte) {
	ts.T().Helper()
	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+path, strings.NewReader(form.Encode()))
	ts.Require().NoError(err)
	for key, values := range header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", testUserAgent)
	if client.clientID != "" {
		req.SetBasicAuth(client.clientID, client.secret)
	}
	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err)
	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Require().NoError(resp.Body.Close())
	return resp, body
}

// accessTokenOf returns the access token of a token response.
func (ts *AuditLogTestSuite) accessTokenOf(body []byte) string {
	ts.T().Helper()
	var token struct {
		AccessToken string `json:"access_token"`
	}
	ts.Require().NoError(json.Unmarshal(body, &token))
	ts.Require().NotEmpty(token.AccessToken)
	return token.AccessToken
}

// detailsOf returns the details of an audit line, nil when it has none.
func (ts *AuditLogTestSuite) detailsOf(line testutils.AuditLine) map[string]string {
	ts.T().Helper()
	var parsed oauthLine
	ts.Require().NoError(json.Unmarshal([]byte(line.Raw), &parsed))
	return parsed.Details
}

// requireNoCredential checks that line carries no secret or client ID of client, and none of tokens.
func (ts *AuditLogTestSuite) requireNoCredential(line testutils.AuditLine, client auditClient, tokens ...string) {
	ts.T().Helper()
	ts.NotContains(line.Raw, client.secret, "a client secret is never recorded")
	ts.NotContains(line.Raw, client.clientID, "a client is named by its application, never its client ID")
	for _, token := range tokens {
		ts.NotContains(line.Raw, token, "a token is never recorded")
	}
}
