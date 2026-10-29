// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"bytes"
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	testServerURL = "https://localhost:8095"

	// mcpResourceIdentifier matches the MCP resource identifier the running integration server
	// itself computes (baseURL + "/mcp", derived from the server's own configured hostname/port —
	// see tests/integration/resources/deployment.yaml). It intentionally does NOT reuse
	// testutils.SystemResourceIdentifier ("https://localhost:8090/mcp"), which reflects the
	// product's generic bootstrapped default rather than this harness's actual port (8095) — a
	// token audience-bound to that default would fail mcp.DefaultGuard's RFC 8707 audience check
	// against this server.
	mcpResourceIdentifier = testServerURL + "/mcp"

	mcpEndpoint    = testServerURL + "/mcp"
	revokeEndpoint = testServerURL + "/oauth2/revoke"

	mcpProtocolVersion = "2025-06-18"

	// mcpProtocolVersion20260728 is the MCP stateless core (SEP-2567/2575) protocol version: no
	// initialize handshake or Mcp-Session-Id, per-request _meta instead.
	mcpProtocolVersion20260728 = "2026-07-28"

	// adminGroupID is the bootstrapped "Administrator" role's group assignment
	// (backend/cmd/server/bootstrap/01-default-resources.yaml, resource_type: role, id
	// 01900000-0000-7000-8000-000000000050, assignments[0].id) — the same well-known, fixed
	// bootstrap ID the product's own default Administrator role targets to grant the "system"
	// permission on the default System resource server. Reused here so the admin ends up in that
	// same group for this suite's own resource server too.
	adminGroupID = "01900000-0000-7000-8000-000000000040"

	systemActionHandle = "system"
)

// MCPTestSuite exercises the MCP server's authentication end-to-end: a real signed token, the
// shared security.BearerAuthenticator path, and real revocation enforcement — as opposed to the
// mocked unit tests in backend/internal/system/mcp/auth, which don't exercise the real JWT service,
// database-backed revocation cache, or the actual /mcp route wiring.
//
// The product's bootstrapped "System" resource server (testutils.SystemResourceIdentifier) that
// normally grants admin users "system" scope has a fixed identifier
// ("https://localhost:8090/mcp") baked into backend/cmd/server/bootstrap/01-default-resources.yaml,
// which does not match this integration harness's actual server URL (port 8095, see
// tests/integration/resources/deployment.yaml) — a token bound to it fails mcp.DefaultGuard's
// audience check here (see TestInitialize_WrongAudienceToken_Rejected, which exercises exactly
// that mismatch). So this suite provisions its own resource server at the harness's real MCP
// identifier, plus a role granting the "system" permission on it to the same bootstrapped admin
// group, to obtain a token that is genuinely valid for this server's own /mcp endpoint.
type MCPTestSuite struct {
	suite.Suite
	client           *http.Client
	ouID             string
	resourceServerID string
	roleID           string
}

func TestMCPTestSuite(t *testing.T) {
	suite.Run(t, new(MCPTestSuite))
}

func (ts *MCPTestSuite) SetupSuite() {
	ts.client = testutils.GetRawHTTPClient()

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "mcp-auth-test-ou",
		Name:        "MCP Auth Test OU",
		Description: "Organization unit for MCP authentication integration tests",
		Parent:      nil,
	})
	ts.Require().NoError(err, "failed to create test organization unit")
	ts.ouID = ouID

	// Registers the resource server whose identifier matches this server's real mcpURL, so a token
	// requested with resource=mcpResourceIdentifier carries a matching "aud" claim. The "system"
	// action mirrors the handle the bootstrapped System resource server defines, so a role can
	// grant the same "system" permission string against this resource server too.
	resourceServerID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:        "MCP Auth Test Resource Server",
		Description: "Resource server matching this server's own MCP endpoint identifier",
		Identifier:  mcpResourceIdentifier,
		OUID:        ts.ouID,
	}, []testutils.Action{
		{Name: "System", Handle: systemActionHandle, Description: "System resource"},
	})
	ts.Require().NoError(err, "failed to create resource server")
	ts.resourceServerID = resourceServerID

	// Grants the "system" permission on this resource server to the same group the bootstrapped
	// admin user belongs to, so the admin token request below can obtain "system" scope bound to
	// this resource server's identifier instead of only the default one.
	roleID, err := testutils.CreateRole(testutils.Role{
		Name:        "MCP Auth Test System Role",
		Description: "Grants system permission on the MCP auth test resource server",
		OUID:        ts.ouID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: ts.resourceServerID, Permissions: []string{systemActionHandle}},
		},
		Assignments: []testutils.Assignment{
			{ID: adminGroupID, Type: "group"},
		},
	})
	ts.Require().NoError(err, "failed to create role granting system permission")
	ts.roleID = roleID
}

func (ts *MCPTestSuite) TearDownSuite() {
	if ts.roleID != "" {
		if err := testutils.DeleteRole(ts.roleID); err != nil {
			ts.T().Logf("Failed to delete role: %v", err)
		}
	}
	if ts.resourceServerID != "" {
		// The suite creates a "system" action under this resource server (see SetupSuite), which
		// blocks a plain delete until it's removed first — DeleteResourceServerWithChildren handles
		// that ordering.
		if err := testutils.DeleteResourceServerWithChildren(ts.resourceServerID); err != nil {
			ts.T().Logf("Failed to delete resource server: %v", err)
		}
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete test organization unit: %v", err)
		}
	}
}

// mcpScopedAdminToken obtains a fresh admin access token (password grant against the public
// CONSOLE client, matching testutils.ObtainAdminAccessToken's pattern) bound to
// mcpResourceIdentifier via the RFC 8707 "resource" parameter, carrying the "system" scope
// mcp.DefaultGuard requires.
func (ts *MCPTestSuite) mcpScopedAdminToken() string {
	tokenResp, err := testutils.ObtainAccessTokenWithPassword(
		"CONSOLE",
		testServerURL+"/console",
		"system",
		testutils.AdminUsername,
		testutils.AdminPassword,
		true,
		"", // no client secret — CONSOLE is a public client
		mcpResourceIdentifier,
	)
	ts.Require().NoError(err, "failed to obtain MCP-scoped admin token")
	ts.Require().NotEmpty(tokenResp.AccessToken, "no access token returned")
	return tokenResp.AccessToken
}

// initializeMCPSession sends the JSON-RPC "initialize" request that starts an MCP Streamable HTTP
// session — the first request any real MCP client sends, and (per RequireBearerToken's wrapping in
// mcp.DefaultGuard) authenticated the same way as every subsequent request on that session. Using a
// bare GET or an empty body here would fail at the MCP transport layer regardless of token
// validity, proving nothing about authentication.
func (ts *MCPTestSuite) initializeMCPSession(token string) *http.Response {
	resp, err := ts.tryInitializeMCPSession(token)
	ts.Require().NoError(err)
	return resp
}

// tryInitializeMCPSession is initializeMCPSession without failing the test, for polling callbacks.
func (ts *MCPTestSuite) tryInitializeMCPSession(token string) (*http.Response, error) {
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{` +
		`"protocolVersion":"` + mcpProtocolVersion + `",` +
		`"capabilities":{},` +
		`"clientInfo":{"name":"thunderid-integration-test","version":"1.0.0"}}}`

	req, err := http.NewRequest(http.MethodPost, mcpEndpoint, bytes.NewBufferString(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	return ts.client.Do(req)
}

func (ts *MCPTestSuite) revoke(token string) {
	form := "token=" + token + "&client_id=CONSOLE"
	req, err := http.NewRequest(http.MethodPost, revokeEndpoint, strings.NewReader(form))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode, "revoke request failed")
}

// A token minted for the MCP resource, carrying the required "system" scope, and not revoked
// authenticates successfully — exercising the full shared BearerAuthenticator path (verification,
// audience check, no-revocation) through the real /mcp route and its guard wiring, not a mock.
func (ts *MCPTestSuite) TestInitialize_ValidMCPScopedToken_Succeeds() {
	token := ts.mcpScopedAdminToken()

	resp := ts.initializeMCPSession(token)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	ts.Assert().Equalf(http.StatusOK, resp.StatusCode, "initialize failed: %s", string(body))
	// mcp.Initialize mounts the MCP server in stateless mode (StreamableHTTPOptions.Stateless), so
	// per the MCP 2026-07-28 stateless core (SEP-2567/2575) no Mcp-Session-Id is minted, even for a
	// legacy "initialize" request such as this one.
	ts.Assert().Empty(resp.Header.Get("Mcp-Session-Id"),
		"a stateless server must not establish a session")
}

// discoverMCP sends a JSON-RPC "server/discover" request using the MCP 2026-07-28 stateless core's
// per-request shape: protocol version, client info, and capabilities travel in the request body's
// _meta instead of a prior "initialize" handshake, and the Streamable HTTP transport mirrors the
// method into the required Mcp-Method header (see mcp.Initialize's Stateless: true wiring).
func (ts *MCPTestSuite) discoverMCP(token string) *http.Response {
	body := `{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{` +
		`"_meta":{` +
		`"io.modelcontextprotocol/protocolVersion":"` + mcpProtocolVersion20260728 + `",` +
		`"io.modelcontextprotocol/clientCapabilities":{},` +
		`"io.modelcontextprotocol/clientInfo":{"name":"thunderid-integration-test","version":"1.0.0"}` +
		`}}}`

	req, err := http.NewRequest(http.MethodPost, mcpEndpoint, bytes.NewBufferString(body))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", mcpProtocolVersion20260728)
	req.Header.Set("Mcp-Method", "server/discover")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	return resp
}

// A valid, MCP-scoped, non-revoked token authenticates a stateless 2026-07-28 request the same way
// it authenticates a legacy "initialize" request: mcp.DefaultGuard's bearer-token guard wraps the
// Streamable HTTP handler regardless of protocol era, so per-request auth must not regress when the
// server negotiates the new stateless core instead of a session-based one.
func (ts *MCPTestSuite) TestDiscover_ValidMCPScopedToken_StatelessSucceeds() {
	token := ts.mcpScopedAdminToken()

	resp := ts.discoverMCP(token)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	ts.Assert().Equalf(http.StatusOK, resp.StatusCode, "server/discover failed: %s", string(body))
	ts.Assert().Contains(string(body), mcpProtocolVersion20260728,
		"server/discover should advertise support for the requested protocol version")
	ts.Assert().Empty(resp.Header.Get("Mcp-Session-Id"),
		"a stateless server must not establish a session")
}

// A required standard header (Mcp-Method) missing from an otherwise well-formed 2026-07-28 request
// is rejected with HeaderMismatch (-32020), proving the header/body mirroring this transport
// requires at the new protocol version is actually enforced, not silently ignored.
func (ts *MCPTestSuite) TestDiscover_MissingMcpMethodHeader_Rejected() {
	token := ts.mcpScopedAdminToken()

	body := `{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{` +
		`"_meta":{` +
		`"io.modelcontextprotocol/protocolVersion":"` + mcpProtocolVersion20260728 + `",` +
		`"io.modelcontextprotocol/clientCapabilities":{}` +
		`}}}`

	req, err := http.NewRequest(http.MethodPost, mcpEndpoint, bytes.NewBufferString(body))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", mcpProtocolVersion20260728)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	ts.Assert().Equalf(http.StatusBadRequest, resp.StatusCode, "response: %s", string(respBody))
	ts.Assert().Contains(string(respBody), "-32020")
}

// A token that is otherwise valid but has been revoked is rejected — proving revocation is enforced
// on the MCP path via the same security.RevocationEnforcerInterface the REST gate uses, not just
// verified in isolation against a mock as in the unit tests.
//
// Per-request enforcement (BearerAuthenticator.Authenticate -> EnsureNotRevoked) is served from an
// in-process cache synced on token_revocation.sync_interval_seconds (tests/integration/resources/
// deployment.yaml sets it to 2s for this reason), not a direct store read on every request — unlike
// RFC 7009 introspection, which the existing oauth/revocation suite proves reflects revocation
// immediately. Retrying past that interval, rather than sleeping the full duration up front, keeps
// this test fast in the common case and only pays the wait when the first attempt is too early.
func (ts *MCPTestSuite) TestInitialize_RevokedToken_Rejected() {
	token := ts.mcpScopedAdminToken()
	ts.revoke(token)

	ts.Require().Eventually(func() bool {
		resp := ts.initializeMCPSession(token)
		defer resp.Body.Close()
		return resp.StatusCode == http.StatusUnauthorized
	}, 5*time.Second, 250*time.Millisecond,
		"a revoked token must be rejected once the revocation cache has synced")
}

// A suspended operator's MCP token is rejected once the revocation copy syncs: suspension revokes by
// entity ID, which the MCP gate checks as the REST gate does. The operator is this test's own, so the
// shared admin is never suspended.
func (ts *MCPTestSuite) TestInitialize_SuspendedOperatorToken_Rejected() {
	const (
		typeHandle = "mcp-suspend-operator"
		username   = "mcp-suspend-operator"
		password   = "McpSuspend@123"
	)
	typeID, err := testutils.CreateUserType(testutils.UserType{
		Handle: typeHandle, DisplayName: typeHandle, OUID: ts.ouID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
			"password": map[string]interface{}{"type": "string", "credential": true},
		},
	})
	ts.Require().NoError(err, "failed to create the operator user type")
	defer func() { _ = testutils.DeleteUserType(typeID) }()
	userID, err := testutils.CreateUser(testutils.User{
		Type: typeHandle, OUID: ts.ouID,
		Attributes: []byte(`{"username": "` + username + `", "password": "` + password + `"}`),
	})
	ts.Require().NoError(err, "failed to create the operator")
	defer func() { _ = testutils.DeleteUser(userID) }()
	roleID, err := testutils.CreateRole(testutils.Role{
		Name: "MCP Suspended Operator Role", OUID: ts.ouID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: ts.resourceServerID, Permissions: []string{systemActionHandle}},
		},
		Assignments: []testutils.Assignment{{ID: userID, Type: "user"}},
	})
	ts.Require().NoError(err, "failed to grant the operator the system permission")
	defer func() { _ = testutils.DeleteRole(roleID) }()

	tokenResp, err := testutils.ObtainAccessTokenWithPassword("CONSOLE", testServerURL+"/console", "system",
		username, password, true, "", mcpResourceIdentifier)
	ts.Require().NoError(err, "the operator must sign in")
	resp := ts.initializeMCPSession(tokenResp.AccessToken)
	resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode, "the token must work before the suspension")

	suspendFlowID, err := testutils.GetFlowIDByHandle("default-user-suspend-flow", "ADMINISTRATION")
	ts.Require().NoError(err, "the suspend flow must ship")
	result, err := common.ExecuteAdministrationFlow(suspendFlowID, map[string]string{"subject": userID}, true)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, result.StatusCode, "the suspend flow was refused: %+v", result.Error)
	ts.Require().Equal("COMPLETE", result.Step.FlowStatus, "the suspension must be placed")

	ts.Require().Eventually(func() bool {
		resp, err := ts.tryInitializeMCPSession(tokenResp.AccessToken)
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode == http.StatusUnauthorized
	}, 10*time.Second, 250*time.Millisecond,
		"a suspended operator's token must be rejected once the revocation cache has synced")
}

// A token that is valid and carries the required scope, but was minted for a different — still
// real, registered — resource (testutils.SystemResourceIdentifier, the product's generic
// bootstrapped default, distinct from this harness's actual mcpResourceIdentifier), is rejected.
// This is also the exact scenario testutils.ObtainAdminAccessToken's default token is in: without
// an explicit resource override it is bound to SystemResourceIdentifier, not this server's real
// mcpURL, so it would fail this same check. Proves the RFC 8707 audience requirement restored in
// mcp.DefaultGuard is enforced end-to-end, not just unit-tested against a mock JWT service.
func (ts *MCPTestSuite) TestInitialize_WrongAudienceToken_Rejected() {
	tokenResp, err := testutils.ObtainAccessTokenWithPassword(
		"CONSOLE",
		testServerURL+"/console",
		"system",
		testutils.AdminUsername,
		testutils.AdminPassword,
		true,
		// no optional resource override -> defaults to testutils.SystemResourceIdentifier, which
		// does not match mcpResourceIdentifier for this test harness (see its doc comment above).
	)
	ts.Require().NoError(err, "failed to obtain admin token for the default system resource")
	ts.Require().NotEmpty(tokenResp.AccessToken)

	resp := ts.initializeMCPSession(tokenResp.AccessToken)
	defer resp.Body.Close()

	ts.Assert().Equal(http.StatusUnauthorized, resp.StatusCode,
		"a token not scoped to the MCP resource must be rejected")
}

// No Authorization header at all is rejected, and does not panic or hang the MCP transport.
func (ts *MCPTestSuite) TestInitialize_NoToken_Rejected() {
	resp := ts.initializeMCPSession("")
	defer resp.Body.Close()

	ts.Assert().Equal(http.StatusUnauthorized, resp.StatusCode)
}

const (
	appRevocationClientID    = "mcp-app-revocation-client"
	appRevocationRedirectURI = "https://localhost:3000/mcp-app-revocation/callback"

	applicationDeletionFlowHandle = "default-application-deletion-flow"
	administrationFlowType        = "ADMINISTRATION"
)

// A token issued through an application is rejected at /mcp once that application is deleted, as it is
// at the REST gate. Deletion revokes the application's OAuth client, so the token's subject is still
// valid and only the client check refuses it.
func (ts *MCPTestSuite) TestInitialize_DeletedApplicationToken_Rejected() {
	appID, err := testutils.CreateApplication(testutils.Application{
		Name:        "MCP App Revocation Test App",
		Description: "Application whose token is used at /mcp before and after deletion",
		OUID:        ts.ouID,
		Type:        "fullstack",
		InboundAuthConfig: []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                appRevocationClientID,
					"redirectUris":            []string{appRevocationRedirectURI},
					"grantTypes":              []string{"authorization_code"},
					"responseTypes":           []string{"code"},
					"tokenEndpointAuthMethod": "none",
					"publicClient":            true,
					"pkceRequired":            true,
				},
			},
		},
	})
	ts.Require().NoError(err, "failed to create application")
	deleted := false
	defer func() {
		if !deleted {
			_ = testutils.DeleteApplication(appID)
		}
	}()

	tokenResp, err := testutils.ObtainAccessTokenWithPassword(appRevocationClientID, appRevocationRedirectURI,
		"system", testutils.AdminUsername, testutils.AdminPassword, true, "", mcpResourceIdentifier)
	ts.Require().NoError(err, "failed to obtain a token through the application")
	token := tokenResp.AccessToken

	resp := ts.initializeMCPSession(token)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	ts.Require().Equalf(http.StatusOK, resp.StatusCode, "the token must work before deletion: %s", string(body))
	ts.Require().Equal(http.StatusOK, ts.restStatus(token), "the token must work at REST before deletion")

	ts.runApplicationDeletionFlow(appID)
	deleted = true

	ts.Require().Eventually(func() bool {
		resp := ts.initializeMCPSession(token)
		defer resp.Body.Close()
		return resp.StatusCode == http.StatusUnauthorized
	}, 5*time.Second, 250*time.Millisecond,
		"a token of a deleted application must be rejected at /mcp once the revocation cache has synced")
	ts.Equal(http.StatusUnauthorized, ts.restStatus(token),
		"the REST gate must reject the same token")
}

// restStatus calls a REST endpoint the "system" scope permits with the token.
func (ts *MCPTestSuite) restStatus(token string) int {
	req, err := http.NewRequest(http.MethodGet, testServerURL+"/organization-units/"+ts.ouID, nil)
	ts.Require().NoError(err)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode
}

// runApplicationDeletionFlow deletes the application through the shipped deletion flow, which revokes
// its OAuth client before removing the record.
func (ts *MCPTestSuite) runApplicationDeletionFlow(appID string) {
	flowID, err := testutils.GetFlowIDByHandle(applicationDeletionFlowHandle, administrationFlowType)
	ts.Require().NoError(err, "failed to resolve the application deletion flow")

	adminToken, err := testutils.GetAccessToken()
	ts.Require().NoError(err, "failed to obtain the admin token")

	reqBody := `{"flowId":"` + flowID + `","inputs":{"targetApplicationId":"` + appID + `"}}`
	req, err := http.NewRequest(http.MethodPost, testServerURL+"/flow/execute", bytes.NewBufferString(reqBody))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	ts.Require().Equalf(http.StatusOK, resp.StatusCode, "deletion flow failed: %s", string(body))
	ts.Require().Containsf(string(body), `"COMPLETE"`, "deletion flow must complete: %s", string(body))
}

// mcpToolResult is the part of a tools/call response the tool tests read.
type mcpToolResult struct {
	Result struct {
		IsError           bool           `json:"isError"`
		StructuredContent map[string]any `json:"structuredContent"`
	} `json:"result"`
}

// postStatelessMCP sends a stateless 2026-07-28 request and returns the JSON-RPC message with the
// raw response body. An SSE response carries the message on its data line. A non-empty mcpName is
// mirrored into the Mcp-Name header, as tools/call requires.
func (ts *MCPTestSuite) postStatelessMCP(token, method, mcpName string, params map[string]any) (string, string) {
	withMeta := map[string]any{
		"_meta": map[string]any{
			"io.modelcontextprotocol/protocolVersion":    mcpProtocolVersion20260728,
			"io.modelcontextprotocol/clientCapabilities": map[string]any{},
			"io.modelcontextprotocol/clientInfo": map[string]any{
				"name": "thunderid-integration-test", "version": "1.0.0"},
		},
	}
	maps.Copy(withMeta, params)
	payload, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": withMeta})
	ts.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPost, mcpEndpoint, bytes.NewReader(payload))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", mcpProtocolVersion20260728)
	req.Header.Set("Mcp-Method", method)
	if mcpName != "" {
		req.Header.Set("Mcp-Name", mcpName)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	ts.Require().Equalf(http.StatusOK, resp.StatusCode, "%s failed: %s",
		strings.TrimSpace(method+" "+mcpName), string(body))

	message := string(body)
	for _, line := range strings.Split(message, "\n") {
		if data, ok := strings.CutPrefix(line, "data: "); ok {
			message = data
		}
	}
	return message, string(body)
}

// callMCPTool sends a stateless 2026-07-28 tools/call request and returns the decoded result with
// the raw response body.
func (ts *MCPTestSuite) callMCPTool(token, name string, args map[string]any) (mcpToolResult, string) {
	message, body := ts.postStatelessMCP(token, "tools/call", name, map[string]any{"name": name, "arguments": args})
	var result mcpToolResult
	ts.Require().NoErrorf(json.Unmarshal([]byte(message), &result), "%s returned: %s", name, body)
	return result, body
}

// tools/list advertises every tool without the product name in its name, and no title, description,
// or schema refers to a tool by its old prefixed name. Clients that aggregate several servers
// namespace tool names themselves, so the server does not prefix them (#6105).
func (ts *MCPTestSuite) TestListTools_NamesDoNotCarryProductName() {
	message, body := ts.postStatelessMCP(ts.mcpScopedAdminToken(), "tools/list", "", map[string]any{})
	var listed struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	ts.Require().NoErrorf(json.Unmarshal([]byte(message), &listed), "tools/list returned: %s", body)
	ts.Require().NotEmptyf(listed.Result.Tools, "tools/list returned no tools: %s", body)

	names := make([]string, 0, len(listed.Result.Tools))
	for _, tool := range listed.Result.Tools {
		ts.NotContainsf(strings.ToLower(tool.Name), "thunderid", "tool %q carries the product name", tool.Name)
		names = append(names, tool.Name)
	}
	ts.Contains(names, "list_applications")
	ts.NotContains(message, "thunderid_")
}

// The React SDK guide tells the agent which tools to call, so it names them without the prefix.
func (ts *MCPTestSuite) TestIntegrateReactSDK_GuideNamesToolsWithoutPrefix() {
	result, raw := ts.callMCPTool(ts.mcpScopedAdminToken(), "integrate_react_sdk", map[string]any{})
	ts.Require().Falsef(result.Result.IsError, "integrate_react_sdk failed: %s", raw)

	instructions, _ := result.Result.StructuredContent["instructions"].(string)
	ts.Require().NotEmptyf(instructions, "integrate_react_sdk returned no instructions: %s", raw)
	ts.NotContains(instructions, "thunderid_")
	ts.Contains(instructions, "`update_application`")
}

// The HTTP connection tools run the /connections/http path end to end: the password is never
// returned, is kept on an update that omits it while the URL origin is unchanged, and is required
// again once the origin changes.
func (ts *MCPTestSuite) TestHTTPConnectionTools_MaskAndRetainPassword() {
	const password = "mcp-http-connection-secret"
	token := ts.mcpScopedAdminToken()

	created, raw := ts.callMCPTool(token, "create_http_connection", map[string]any{
		"ouId": ts.ouID, "name": "MCP Tools HTTP Connection", "url": "https://risk.example.com/evaluate",
		"authentication": map[string]any{"type": "basic",
			"properties": map[string]any{"username": "svc-user", "password": password}},
	})
	ts.Require().Falsef(created.Result.IsError, "create failed: %s", raw)
	ts.NotContains(raw, password)
	id, _ := created.Result.StructuredContent["id"].(string)
	ts.Require().NotEmpty(id)
	ts.T().Cleanup(func() { ts.deleteHTTPConnection(id) })

	got, raw := ts.callMCPTool(token, "get_http_connection_by_id", map[string]any{"id": id})
	ts.Require().Falsef(got.Result.IsError, "get failed: %s", raw)
	ts.NotContains(raw, password)
	ts.Contains(raw, `"password":"******"`)

	listed, raw := ts.callMCPTool(token, "list_http_connections", map[string]any{})
	ts.Require().Falsef(listed.Result.IsError, "list failed: %s", raw)
	ts.Contains(raw, id)
	ts.NotContains(raw, password)

	sameOrigin, raw := ts.callMCPTool(token, "update_http_connection", map[string]any{
		"id": id, "name": "MCP Tools HTTP Connection", "url": "https://risk.example.com/v2/evaluate",
		"authentication": map[string]any{"type": "basic", "properties": map[string]any{"username": "svc-user"}},
	})
	ts.Falsef(sameOrigin.Result.IsError, "an update keeping the origin must keep the stored password: %s", raw)
	ts.NotContains(raw, password)

	otherOrigin, raw := ts.callMCPTool(token, "update_http_connection", map[string]any{
		"id": id, "name": "MCP Tools HTTP Connection", "url": "https://elsewhere.example.net/evaluate",
		"authentication": map[string]any{"type": "basic", "properties": map[string]any{"username": "svc-user"}},
	})
	ts.Truef(otherOrigin.Result.IsError, "an update moving the origin must require the password again: %s", raw)
	ts.Contains(raw, "failed to update HTTP connection")
	ts.Contains(raw, "must give every field its method requires")
}

// The reviewer's case (AC11.3): an agent reads a connection through the get tool and sends the whole
// object back through the update tool, so the password arrives as the mask. The update is refused,
// and the stored password is unchanged: a later update that omits it, at the same origin, still
// succeeds, which it could not if the password had been dropped or replaced by the mask.
func (ts *MCPTestSuite) TestHTTPConnectionTools_MaskedPasswordReadBackIsRefused() {
	const password = "mcp-http-connection-readback-secret"
	const name = "MCP Tools HTTP Connection Read Back"
	token := ts.mcpScopedAdminToken()

	created, raw := ts.callMCPTool(token, "create_http_connection", map[string]any{
		"ouId": ts.ouID, "name": name, "url": "https://risk.example.com/evaluate",
		"authentication": map[string]any{"type": "basic",
			"properties": map[string]any{"username": "svc-user", "password": password}},
	})
	ts.Require().Falsef(created.Result.IsError, "create failed: %s", raw)
	id, _ := created.Result.StructuredContent["id"].(string)
	ts.Require().NotEmpty(id)
	ts.T().Cleanup(func() { ts.deleteHTTPConnection(id) })

	got, raw := ts.callMCPTool(token, "get_http_connection_by_id", map[string]any{"id": id})
	ts.Require().Falsef(got.Result.IsError, "get failed: %s", raw)
	readBack := got.Result.StructuredContent
	// type is part of the output only; the update workflow tells the agent to leave it out.
	delete(readBack, "type")
	readBack["description"] = "changed by the read-back update"

	refused, raw := ts.callMCPTool(token, "update_http_connection", readBack)
	ts.Truef(refused.Result.IsError, "a password sent as the mask must be refused: %s", raw)
	ts.Contains(raw, "failed to update HTTP connection")
	ts.Contains(raw, "a password or token cannot be ******")
	ts.NotContains(raw, password)

	got, raw = ts.callMCPTool(token, "get_http_connection_by_id", map[string]any{"id": id})
	ts.Require().Falsef(got.Result.IsError, "get failed: %s", raw)
	ts.Nil(got.Result.StructuredContent["description"], "the refused update must not be applied")
	ts.Contains(raw, `"password":"******"`)

	kept, raw := ts.callMCPTool(token, "update_http_connection", map[string]any{
		"id": id, "name": name, "url": "https://risk.example.com/v2/evaluate",
		"authentication": map[string]any{"type": "basic", "properties": map[string]any{"username": "svc-user"}},
	})
	ts.Falsef(kept.Result.IsError, "the stored password must survive the refused update: %s", raw)
	ts.NotContains(raw, password)
}

func (ts *MCPTestSuite) deleteHTTPConnection(id string) {
	adminToken, err := testutils.GetAccessToken()
	ts.Require().NoError(err, "failed to obtain the admin token")
	req, err := http.NewRequest(http.MethodDelete, testServerURL+"/connections/http/"+id, nil)
	ts.Require().NoError(err)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Equal(http.StatusNoContent, resp.StatusCode)
}
