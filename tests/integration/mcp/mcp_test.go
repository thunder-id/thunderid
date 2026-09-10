// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

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
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{` +
		`"protocolVersion":"` + mcpProtocolVersion + `",` +
		`"capabilities":{},` +
		`"clientInfo":{"name":"thunderid-integration-test","version":"1.0.0"}}}`

	req, err := http.NewRequest(http.MethodPost, mcpEndpoint, bytes.NewBufferString(body))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	return resp
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
	ts.Assert().NotEmpty(resp.Header.Get("Mcp-Session-Id"),
		"a successful initialize should establish an MCP session")
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
