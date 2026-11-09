// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// The Control Plane's MCP endpoint with a trusted issuer. Tokens the trusted issuer signed come from
// the harness (main.go), which holds the issuer's key.

const mcpProtocolVersion = "2026-07-28"

var trustedIssuerURL = fmt.Sprintf("http://localhost:%d", testutils.TrustedIssuerPort)

// rawClient sends this file's requests with only the credentials each one sets. Its timeout fails a
// request the server never answers, instead of leaving it to the go test timeout.
var rawClient = func() *http.Client {
	client := testutils.GetRawHTTPClient()
	client.Timeout = 30 * time.Second
	return client
}()

// A client that follows the challenge is sent to the trusted issuer: the metadata names it as the
// authorization server, with the MCP resource and the scope a full administrator requests, and the
// issuer publishes metadata for itself.
func (ts *ControlPlaneTestSuite) TestMCPMetadataNamesTheTrustedIssuer() {
	status, challenge, body := ts.listMCPTools("")
	ts.Require().Equalf(http.StatusUnauthorized, status, "response: %s", body)
	ts.Empty(challengeParam(challenge, "scope"))
	metadataURL := challengeParam(challenge, "resource_metadata")
	ts.Require().Equal(testutils.TestServerURL+"/.well-known/oauth-protected-resource", metadataURL)

	var metadata struct {
		Resource             string   `json:"resource"`
		AuthorizationServers []string `json:"authorization_servers"`
		ScopesSupported      []string `json:"scopes_supported"`
	}
	ts.getJSON(metadataURL, &metadata)
	ts.Equal(testutils.TestServerURL+"/mcp", metadata.Resource)
	ts.Equal([]string{trustedIssuerURL}, metadata.AuthorizationServers)
	ts.Equal([]string{"system"}, metadata.ScopesSupported)

	// The suite's issuer is an OpenID provider, so it publishes OpenID Connect discovery metadata.
	var issuer struct {
		Issuer string `json:"issuer"`
	}
	ts.getJSON(trustedIssuerURL+"/.well-known/openid-configuration", &issuer)
	ts.Equal(trustedIssuerURL, issuer.Issuer)
}

// A token the trusted issuer signed with a system permission reaches the tools. One without the scope
// is refused with 403. Any system permission admits the caller, so the insufficient_scope challenge
// names no single scope.
func (ts *ControlPlaneTestSuite) TestMCPNeedsTheSystemScope() {
	status, _, body := ts.listMCPTools(ts.harnessToken(testutils.MCPScopedTokenEnv))
	ts.Equalf(http.StatusOK, status, "response: %s", body)

	status, challenge, body := ts.listMCPTools(ts.harnessToken(testutils.MCPUnscopedTokenEnv))
	ts.Equalf(http.StatusForbidden, status, "response: %s", body)
	ts.Equal("insufficient_scope", challengeParam(challenge, "error"))
	ts.Empty(challengeParam(challenge, "scope"))
	ts.Equal(testutils.TestServerURL+"/.well-known/oauth-protected-resource",
		challengeParam(challenge, "resource_metadata"))
}

// A token is refused with 401 when it is not a JWT, when the trusted issuer did not sign it, when it
// comes from another issuer, when it has expired, and when it was issued for another resource, the
// management API included: the MCP server accepts only tokens issued for the resource it advertises
// (thunder-id/thunderid#6037).
func (ts *ControlPlaneTestSuite) TestMCPRefusesATokenItDoesNotTrust() {
	management, err := testutils.GetAccessToken()
	ts.Require().NoError(err)

	forger, err := testutils.NewMockOIDCServer("", "")
	ts.Require().NoError(err)
	defer func() { _ = forger.Stop() }()
	sign := func(issuer string) string {
		now := time.Now()
		token, err := forger.SignJWT(map[string]interface{}{"typ": "at+jwt"}, map[string]interface{}{
			"iss":   issuer,
			"aud":   testutils.MCPResource(),
			"sub":   testutils.TrustedIssuerAdminSubject,
			"scope": "system",
			"iat":   now.Unix(),
			"exp":   now.Add(time.Hour).Unix(),
		})
		ts.Require().NoError(err)
		return token
	}

	cases := []struct {
		name  string
		token string
	}{
		{name: "not a JWT", token: "not-a-jwt"},
		{name: "signed with another key", token: sign(trustedIssuerURL)},
		{name: "from another issuer", token: sign(forger.GetURL())},
		{name: "expired", token: ts.harnessToken(testutils.MCPExpiredTokenEnv)},
		{name: "for another resource", token: ts.harnessToken(testutils.OtherResourceTokenEnv)},
		{name: "for the management API", token: management},
	}
	for _, tc := range cases {
		ts.Run(tc.name, func() {
			status, _, body := ts.listMCPTools(tc.token)
			ts.Equalf(http.StatusUnauthorized, status, "response: %s", body)
		})
	}
}

// harnessToken returns the token the harness exported in env.
func (ts *ControlPlaneTestSuite) harnessToken(env string) string {
	token := os.Getenv(env)
	ts.Require().NotEmptyf(token, "the harness exports %s", env)
	return token
}

// listMCPTools sends a stateless tools/list request to /mcp with the given token, or none when it is
// empty, and returns the status, the WWW-Authenticate challenge and the body.
func (ts *ControlPlaneTestSuite) listMCPTools(token string) (int, string, []byte) {
	ts.T().Helper()
	return ts.postMCP(token, "tools/list", "", map[string]any{})
}

// postMCP sends a stateless request to /mcp with the given token, or none when it is empty, and
// returns the status, the WWW-Authenticate challenge and the body. A non-empty mcpName is mirrored
// into the Mcp-Name header, as tools/call requires.
func (ts *ControlPlaneTestSuite) postMCP(token, method, mcpName string, params map[string]any) (
	int, string, []byte) {
	ts.T().Helper()
	params["_meta"] = map[string]any{
		"io.modelcontextprotocol/protocolVersion":    mcpProtocolVersion,
		"io.modelcontextprotocol/clientCapabilities": map[string]any{},
		"io.modelcontextprotocol/clientInfo": map[string]any{
			"name": "thunderid-integration-test", "version": "1.0.0"},
	}
	payload, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	ts.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+"/mcp", bytes.NewReader(payload))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", mcpProtocolVersion)
	req.Header.Set("Mcp-Method", method)
	if mcpName != "" {
		req.Header.Set("Mcp-Name", mcpName)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := rawClient.Do(req)
	ts.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	return resp.StatusCode, resp.Header.Get("WWW-Authenticate"), body
}

// challengeParam returns the value of the named quoted parameter in a WWW-Authenticate challenge.
func challengeParam(challenge, name string) string {
	_, rest, found := strings.Cut(challenge, name+`="`)
	if !found {
		return ""
	}
	value, _, _ := strings.Cut(rest, `"`)
	return value
}

// getJSON fetches url without credentials and decodes its JSON body into out.
func (ts *ControlPlaneTestSuite) getJSON(url string, out any) {
	ts.T().Helper()
	resp, err := rawClient.Get(url)
	ts.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Require().Equalf(http.StatusOK, resp.StatusCode, "GET %s: %s", url, string(body))
	ts.Require().NoErrorf(json.Unmarshal(body, out), "GET %s: %s", url, string(body))
}
