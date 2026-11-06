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
// authorization server, with the MCP resource and the system scope, and the issuer publishes
// metadata for itself.
func (ts *ControlPlaneTestSuite) TestMCPMetadataNamesTheTrustedIssuer() {
	status, challenge, body := ts.listMCPTools("")
	ts.Require().Equalf(http.StatusUnauthorized, status, "response: %s", body)
	ts.Equal("system", challengeParam(challenge, "scope"))
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

// A token the trusted issuer signed with the system scope reaches the tools. One without the scope
// is refused with 403, and the challenge names the scope.
func (ts *ControlPlaneTestSuite) TestMCPNeedsTheSystemScope() {
	status, _, body := ts.listMCPTools(ts.harnessToken(testutils.MCPScopedTokenEnv))
	ts.Equalf(http.StatusOK, status, "response: %s", body)

	status, challenge, body := ts.listMCPTools(ts.harnessToken(testutils.MCPUnscopedTokenEnv))
	ts.Equalf(http.StatusForbidden, status, "response: %s", body)
	ts.Equal("system", challengeParam(challenge, "scope"))
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

// A Control Plane creates agents directly, so it offers onboard_agent and not execute_flow, which
// runs flows it does not have. The agent it creates gets a secret, and the token guide says the token
// endpoints are on the data planes.
func (ts *ControlPlaneTestSuite) TestMCPOnboardsAgentsDirectly() {
	token := ts.harnessToken(testutils.MCPScopedTokenEnv)
	status, _, body := ts.listMCPTools(token)
	ts.Require().Equalf(http.StatusOK, status, "response: %s", body)
	ts.Contains(string(body), `"onboard_agent"`)
	ts.NotContains(string(body), `"execute_flow"`)

	// The suite's administrator is a user of the trusted issuer, not of the Control Plane, so the
	// agent needs an owner of its own.
	userType := unique("cp-agent-owner")
	userTypeID, err := testutils.CreateUserType(testutils.UserType{
		Handle: userType, DisplayName: "Agent owner", OUID: ts.ouID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string", "required": true, "unique": true},
		},
	})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteUserType(userTypeID) }()
	attributes, err := json.Marshal(map[string]any{"username": unique("cp-agent-owner")})
	ts.Require().NoError(err)
	ownerID, err := testutils.CreateUser(testutils.User{OUID: ts.ouID, Type: userType, Attributes: attributes})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteUser(ownerID) }()

	message := ts.callMCPTool(token, "onboard_agent", map[string]any{
		"name": unique("cp-agent"), "ouId": ts.ouID, "owner": ownerID, "include": []string{"token_guide"}})
	var result struct {
		Result struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				Agent struct {
					ID    string `json:"id"`
					Owner string `json:"owner"`
				} `json:"agent"`
				ClientSecret string `json:"clientSecret"`
				Endpoints    string `json:"endpoints"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	ts.Require().NoErrorf(json.Unmarshal([]byte(message), &result), "onboard_agent returned: %s", message)
	ts.Require().Falsef(result.Result.IsError, "onboard_agent failed: %s", message)
	agent := result.Result.StructuredContent
	ts.Require().NotEmpty(agent.Agent.ID)
	defer func() { _ = testutils.DeleteAgent(agent.Agent.ID) }()
	ts.Equal(ownerID, agent.Agent.Owner)
	ts.NotEmpty(agent.ClientSecret)
	ts.Contains(agent.Endpoints, "control plane")

	// An agent holding a key that also signs users in and exchanges their tokens gets a guide for each
	// way of getting a token, each authenticating with a signed assertion.
	message = ts.callMCPTool(token, "onboard_agent", map[string]any{
		"name": unique("cp-keyed-agent"), "ouId": ts.ouID, "owner": ownerID,
		"credential": "private_key_jwt", "jwksUri": "https://agent.example.com/.well-known/jwks.json",
		"actsForUsers": []string{"sign_in", "token_exchange"}, "redirectUris": []string{"https://agent.example.com/cb"},
		"include": []string{"token_guide"}})
	var keyed struct {
		Result struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				Agent struct {
					ID string `json:"id"`
				} `json:"agent"`
				ClientSecret string `json:"clientSecret"`
				GetTokens    []struct {
					For                  string `json:"for"`
					ClientAuthentication string `json:"clientAuthentication"`
					Note                 string `json:"note"`
				} `json:"getTokens"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	ts.Require().NoErrorf(json.Unmarshal([]byte(message), &keyed), "onboard_agent returned: %s", message)
	ts.Require().Falsef(keyed.Result.IsError, "onboard_agent failed: %s", message)
	ts.Require().NotEmpty(keyed.Result.StructuredContent.Agent.ID)
	defer func() { _ = testutils.DeleteAgent(keyed.Result.StructuredContent.Agent.ID) }()
	ts.Empty(keyed.Result.StructuredContent.ClientSecret, "an agent with a key gets no secret")
	ways := []string{}
	for _, guide := range keyed.Result.StructuredContent.GetTokens {
		ways = append(ways, guide.For)
		ts.Contains(guide.ClientAuthentication, "client_assertion")
	}
	ts.Equalf([]string{"on_its_own", "sign_in", "token_exchange"}, ways, "onboard_agent returned: %s", message)
	ts.Contains(keyed.Result.StructuredContent.GetTokens[1].Note, "refresh_token",
		"signing users in comes with refresh tokens")
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

// callMCPTool sends a stateless tools/call request and returns the JSON-RPC message, with the data
// line of an SSE response unwrapped.
func (ts *ControlPlaneTestSuite) callMCPTool(token, name string, args map[string]any) string {
	ts.T().Helper()
	status, _, body := ts.postMCP(token, "tools/call", name, map[string]any{"name": name, "arguments": args})
	ts.Require().Equalf(http.StatusOK, status, "%s failed: %s", name, body)
	message := string(body)
	for _, line := range strings.Split(message, "\n") {
		if data, ok := strings.CutPrefix(line, "data: "); ok {
			message = data
		}
	}
	return message
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
