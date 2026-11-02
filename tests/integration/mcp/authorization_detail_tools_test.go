// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const authorizationDetailTypesPath = "/authorization-detail-types"

// The authorization details type tools run the registry end to end: the processor credential is never
// returned, a definition read through the get tool and sent back through the update tool keeps it, and a
// processor moved to another endpoint needs it again.
func (ts *MCPTestSuite) TestAuthorizationDetailTypeTools_MaskAndRetainProcessorCredential() {
	const credential = "mcp-authorization-detail-processor-token"
	token := ts.mcpScopedAdminToken()
	value := fmt.Sprintf("https://types.example.com/mcp/%d", time.Now().UnixNano())

	created, raw := ts.callMCPTool(token, "create_authorization_detail_type", map[string]any{
		"type": value, "name": "MCP Flight", "consentTemplate": "Book {{route}}",
		"schema": map[string]any{"type": "object", "required": []any{"route"},
			"properties": map[string]any{"route": map[string]any{"type": "string", "title": "Route"}}},
		"processor": map[string]any{"endpoint": "https://processor.example.com/rar",
			"authentication": map[string]any{"type": "bearer",
				"properties": map[string]any{"token": credential}}},
	})
	ts.Require().Falsef(created.Result.IsError, "create failed: %s", raw)
	ts.NotContains(raw, credential)
	id, _ := created.Result.StructuredContent["id"].(string)
	ts.Require().NotEmpty(id)
	ts.T().Cleanup(func() { ts.deleteAuthorizationDetailType(id) })

	got, raw := ts.callMCPTool(token, "get_authorization_detail_type_by_id", map[string]any{"id": id})
	ts.Require().Falsef(got.Result.IsError, "get failed: %s", raw)
	ts.NotContains(raw, credential)
	ts.Contains(raw, `"token":"******"`)
	ts.Equal(value, got.Result.StructuredContent["type"])

	listed, raw := ts.callMCPTool(token, "list_authorization_detail_types", map[string]any{})
	ts.Require().Falsef(listed.Result.IsError, "list failed: %s", raw)
	ts.Contains(raw, id)
	ts.Contains(raw, value)
	ts.NotContains(raw, credential)

	// The definition the get tool returned is sent back whole, with the credential as the mask.
	readBack := got.Result.StructuredContent
	readBack["name"] = "MCP Flight Renamed"
	updated, raw := ts.callMCPTool(token, "update_authorization_detail_type", readBack)
	ts.Require().Falsef(updated.Result.IsError, "a masked credential at the same endpoint must be kept: %s", raw)
	ts.Equal("MCP Flight Renamed", updated.Result.StructuredContent["name"])
	ts.Equal(got.Result.StructuredContent["version"], updated.Result.StructuredContent["version"],
		"a rename keeps the version")
	ts.NotContains(raw, credential)
	ts.Contains(ts.exportedEnvironment(id), "="+credential+"\n", "the stored credential is still the original")

	moved, raw := ts.callMCPTool(token, "update_authorization_detail_type", map[string]any{
		"id": id, "type": value, "name": "MCP Flight Renamed", "consentTemplate": "Book {{route}}",
		"schema":    readBack["schema"],
		"processor": map[string]any{"endpoint": "https://elsewhere.example.net/rar"},
	})
	ts.Truef(moved.Result.IsError, "a processor moved to another endpoint must need its credential again: %s", raw)
	ts.Contains(raw, "failed to update authorization details type")
	ts.NotContains(raw, credential)
}

// The authorization details type tools refuse what the management API refuses, and say which tool failed.
func (ts *MCPTestSuite) TestAuthorizationDetailTypeTools_RefuseInvalidInput() {
	token := ts.mcpScopedAdminToken()
	value := fmt.Sprintf("https://types.example.com/mcp-refused/%d", time.Now().UnixNano())
	schema := map[string]any{"type": "object",
		"properties": map[string]any{"route": map[string]any{"type": "string"}}}

	misspelled, raw := ts.callMCPTool(token, "create_authorization_detail_type", map[string]any{
		"type": value, "name": "MCP Misspelled", "schema": schema,
		"procesor": map[string]any{"endpoint": "https://processor.example.com/rar"},
	})
	ts.Truef(misspelled.Result.IsError, "a misspelled key must be refused: %s", raw)
	ts.Contains(raw, "invalid arguments")

	invalid, raw := ts.callMCPTool(token, "create_authorization_detail_type", map[string]any{
		"type": value, "name": "MCP Invalid",
		"schema": map[string]any{"type": "object", "properties": map[string]any{"actions": map[string]any{
			"type": "array"}}},
	})
	ts.Truef(invalid.Result.IsError, "an invalid schema must be refused: %s", raw)
	ts.Contains(raw, "failed to create authorization details type")

	badAuthentication, raw := ts.callMCPTool(token, "create_authorization_detail_type",
		map[string]any{"type": value, "name": "MCP Bad Authentication", "schema": schema,
			"processor": map[string]any{"endpoint": "https://processor.example.com/rar",
				"authentication": map[string]any{"type": "kerberos"}}})
	ts.Truef(badAuthentication.Result.IsError, "an unknown authentication must be refused: %s", raw)
	ts.Contains(raw, "failed to create authorization details type")

	missing, raw := ts.callMCPTool(token, "get_authorization_detail_type_by_id",
		map[string]any{"id": "00000000-0000-7000-8000-00000000dead"})
	ts.Truef(missing.Result.IsError, "an unknown id must not be found: %s", raw)
	ts.Contains(raw, "failed to get authorization details type")

	missingUpdate, raw := ts.callMCPTool(token, "update_authorization_detail_type", map[string]any{
		"id": "00000000-0000-7000-8000-00000000dead", "type": value, "name": "MCP Missing", "schema": schema,
		"processor": map[string]any{"endpoint": "https://processor.example.com/rar",
			"authentication": map[string]any{"type": "kerberos"}},
	})
	ts.Truef(missingUpdate.Result.IsError, "an update of an unknown id must fail: %s", raw)
	ts.Contains(raw, "failed to update authorization details type")
}

// exportedEnvironment exports an authorization details type and returns its environment variables file,
// which carries the processor credential in clear.
func (ts *MCPTestSuite) exportedEnvironment(id string) string {
	payload, err := json.Marshal(map[string]any{"authorizationDetailTypes": []string{id}})
	ts.Require().NoError(err)
	resp, err := testutils.GetHTTPClient().Post(testServerURL+"/export", "application/json",
		bytes.NewReader(payload))
	ts.Require().NoError(err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	ts.Require().Equalf(http.StatusOK, resp.StatusCode, "export failed: %s", string(body))
	var exported struct {
		EnvironmentVariables string `json:"environment_variables"`
	}
	ts.Require().NoError(json.Unmarshal(body, &exported))
	return exported.EnvironmentVariables
}

func (ts *MCPTestSuite) deleteAuthorizationDetailType(id string) {
	adminToken, err := testutils.GetAccessToken()
	ts.Require().NoError(err, "failed to obtain the admin token")
	req, err := http.NewRequest(http.MethodDelete, testServerURL+authorizationDetailTypesPath+"/"+id, nil)
	ts.Require().NoError(err)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Equal(http.StatusNoContent, resp.StatusCode)
}
