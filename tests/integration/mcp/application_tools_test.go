// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// regenerate_application_secret replaces the application's secret and revokes the tokens the old one issued,
// and its notice says so. The application introspects with its new secret, since the old one no longer works.
func (ts *MCPTestSuite) TestApplicationTools_RegenerateSecretRevokesItsTokens() {
	token := ts.mcpScopedAdminToken()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	clientID, secret := "mcp-regen-client-"+suffix, "mcp-regen-secret-"+suffix
	appID, err := testutils.CreateApplication(testutils.Application{
		Name: "mcp_regen_app_" + suffix,
		OUID: ts.ouID,
		Type: "fullstack",
		InboundAuthConfig: []map[string]interface{}{{"type": "oauth2", "config": map[string]interface{}{
			"clientId":                clientID,
			"clientSecret":            secret,
			"redirectUris":            []string{"https://localhost:3000"},
			"grantTypes":              []string{"client_credentials"},
			"tokenEndpointAuthMethod": "client_secret_basic",
		}}},
	})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteApplication(appID) })
	issued, err := testutils.RequestClientCredentialsToken(clientID, secret, "", "")
	ts.Require().NoError(err)
	ts.Require().Equalf(http.StatusOK, issued.StatusCode, "token request failed: %s", issued.Body)
	oldToken := issued.Token.AccessToken

	regenerated, raw := ts.callMCPTool(token, "regenerate_application_secret", map[string]any{"id": appID})
	ts.Require().Falsef(regenerated.Result.IsError, "regenerate_application_secret failed: %s", raw)
	newSecret, _ := regenerated.Result.StructuredContent["clientSecret"].(string)
	ts.Require().NotEmpty(newSecret)
	ts.NotEqual(secret, newSecret)
	ts.Nil(regenerated.Result.StructuredContent["revocationIncomplete"], "the revocation must be complete")
	notice, _ := regenerated.Result.StructuredContent["notice"].(string)
	ts.Contains(notice, "the tokens it issued are revoked")

	active, err := testutils.IntrospectToken(oldToken, clientID, newSecret)
	ts.Require().NoError(err)
	ts.False(active, "a token the old secret issued was still active")
	issued, err = testutils.RequestClientCredentialsToken(clientID, secret, "", "")
	ts.Require().NoError(err)
	ts.Equal(http.StatusUnauthorized, issued.StatusCode, "the old secret must stop working")
	// A token's iat has second granularity while the cutoff is sub-second, and the comparison is at or before.
	time.Sleep(1100 * time.Millisecond)
	issued, err = testutils.RequestClientCredentialsToken(clientID, newSecret, "", "")
	ts.Require().NoError(err)
	ts.Require().Equalf(http.StatusOK, issued.StatusCode, "the new secret must work: %s", issued.Body)
	active, err = testutils.IntrospectToken(issued.Token.AccessToken, clientID, newSecret)
	ts.Require().NoError(err)
	ts.True(active, "a token the new secret issued was not active")
}

// create_application and update_application take the same inbound authentication profile as the
// application API: subjectAttribute is not in their input schema, a call that carries it is refused,
// and the same call without it succeeds.
func (ts *MCPTestSuite) TestApplicationTools_CreateAndUpdateTakeTheAPIProfile() {
	token := ts.mcpScopedAdminToken()

	message, body := ts.postStatelessMCP(token, "tools/list", "", map[string]any{})
	var listed struct {
		Result struct {
			Tools []struct {
				Name        string `json:"name"`
				InputSchema struct {
					Properties map[string]any `json:"properties"`
				} `json:"inputSchema"`
			} `json:"tools"`
		} `json:"result"`
	}
	ts.Require().NoErrorf(json.Unmarshal([]byte(message), &listed), "tools/list returned: %s", body)
	checked := 0
	for _, tool := range listed.Result.Tools {
		if tool.Name != "create_application" && tool.Name != "update_application" {
			continue
		}
		checked++
		ts.Contains(tool.InputSchema.Properties, "allowedUserTypes", "tool %q", tool.Name)
		ts.NotContains(tool.InputSchema.Properties, "subjectAttribute", "tool %q", tool.Name)
	}
	ts.Equal(2, checked, "tools/list must list create_application and update_application")

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	args := map[string]any{
		"name":                      "mcp_profile_app_" + suffix,
		"ouId":                      ts.ouID,
		"type":                      "fullstack",
		"isRegistrationFlowEnabled": false,
		"isRecoveryFlowEnabled":     false,
	}
	withMapping := func(in map[string]any) map[string]any {
		out := maps.Clone(in)
		out["subjectAttribute"] = map[string]string{"customer": "email"}
		return out
	}

	refused, raw := ts.callMCPTool(token, "create_application", withMapping(args))
	ts.Require().Truef(refused.Result.IsError, "create_application with subjectAttribute must be refused: %s", raw)
	ts.Contains(raw, "subjectAttribute")

	created, raw := ts.callMCPTool(token, "create_application", args)
	ts.Require().Falsef(created.Result.IsError, "create_application failed: %s", raw)
	appID, _ := created.Result.StructuredContent["id"].(string)
	ts.Require().NotEmpty(appID)
	ts.T().Cleanup(func() { _ = testutils.DeleteApplication(appID) })

	update := maps.Clone(args)
	update["id"] = appID
	update["description"] = "updated"
	refused, raw = ts.callMCPTool(token, "update_application", withMapping(update))
	ts.Require().Truef(refused.Result.IsError, "update_application with subjectAttribute must be refused: %s", raw)
	ts.Contains(raw, "subjectAttribute")

	updated, raw := ts.callMCPTool(token, "update_application", update)
	ts.Require().Falsef(updated.Result.IsError, "update_application failed: %s", raw)
	ts.Equal("updated", updated.Result.StructuredContent["description"])
}

// get_application_by_id returns the application with its read-only status, and sending that result back to
// update_application with one field changed updates the application and leaves the status as it was.
func (ts *MCPTestSuite) TestApplicationTools_UpdateAcceptsGetResult() {
	token := ts.mcpScopedAdminToken()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	appID, err := testutils.CreateApplication(testutils.Application{
		Name: "mcp_round_trip_app_" + suffix,
		OUID: ts.ouID,
		Type: "fullstack",
		InboundAuthConfig: []map[string]interface{}{{"type": "oauth2", "config": map[string]interface{}{
			"clientId":                "mcp-round-trip-client-" + suffix,
			"clientSecret":            "mcp-round-trip-secret-" + suffix,
			"redirectUris":            []string{"https://localhost:3000/callback"},
			"grantTypes":              []string{"authorization_code", "refresh_token"},
			"responseTypes":           []string{"code"},
			"tokenEndpointAuthMethod": "client_secret_basic",
			"scopes":                  []string{"openid", "profile"},
		}}},
	})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteApplication(appID) })

	got, raw := ts.callMCPTool(token, "get_application_by_id", map[string]any{"id": appID})
	ts.Require().Falsef(got.Result.IsError, "get_application_by_id failed: %s", raw)
	app := got.Result.StructuredContent
	ts.Require().Containsf(app, "status", "get_application_by_id returned no status: %s", raw)
	status := app["status"]
	app["description"] = "Updated through the MCP tools"

	updated, raw := ts.callMCPTool(token, "update_application", app)
	ts.Require().Falsef(updated.Result.IsError, "update_application refused the get result: %s", raw)

	reread, raw := ts.callMCPTool(token, "get_application_by_id", map[string]any{"id": appID})
	ts.Require().Falsef(reread.Result.IsError, "get_application_by_id failed: %s", raw)
	ts.Equal("Updated through the MCP tools", reread.Result.StructuredContent["description"])
	ts.Equal(status, reread.Result.StructuredContent["status"])
	before, after := ts.oauthConfig(app), ts.oauthConfig(reread.Result.StructuredContent)
	for _, key := range []string{"clientId", "redirectUris", "grantTypes", "responseTypes", "scopes"} {
		ts.Equalf(before[key], after[key], "inboundAuthConfig %s changed", key)
	}
}

// oauthConfig returns the OAuth config of an application tool result.
func (ts *MCPTestSuite) oauthConfig(app map[string]any) map[string]any {
	inbound, _ := app["inboundAuthConfig"].([]any)
	ts.Require().Len(inbound, 1)
	entry, _ := inbound[0].(map[string]any)
	config, _ := entry["config"].(map[string]any)
	ts.Require().NotNil(config)
	return config
}
