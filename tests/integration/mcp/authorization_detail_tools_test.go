// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// authorizationDetailTypeResourceType is the import resource type of an authorization details type.
const authorizationDetailTypeResourceType = "authorization_detail_type"

// The authorization details type tools read the registry end to end without returning the processor
// credential. A data plane takes a type only from an import with the management API key, so the type is
// imported.
func (ts *MCPTestSuite) TestAuthorizationDetailTypeTools_MaskTheProcessorCredential() {
	const credential = "mcp-authorization-detail-processor-token"
	token := ts.mcpScopedAdminToken()
	value := fmt.Sprintf("https://types.example.com/mcp/%d", time.Now().UnixNano())

	id, err := testutils.ImportResource(authorizationDetailTypeResourceType, map[string]any{
		"type": value, "name": "MCP Flight", "consentTemplate": "Book {{route}}",
		"schema": map[string]any{"type": "object", "required": []any{"route"},
			"properties": map[string]any{"route": map[string]any{"type": "string", "title": "Route"}}},
		"processor": map[string]any{"endpoint": "https://processor.example.com/rar",
			"authentication": map[string]any{"type": "bearer",
				"properties": map[string]any{"token": credential}}},
	}, false)
	ts.Require().NoError(err)
	ts.T().Cleanup(func() {
		ts.NoError(testutils.DeleteImportedResource(authorizationDetailTypeResourceType, id))
	})

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
}

// A data plane takes no authorization details type from a token, and an MCP caller always presents one, so
// the data plane offers only the read tools.
func (ts *MCPTestSuite) TestAuthorizationDetailTypeTools_DataPlaneOffersOnlyReads() {
	message, body := ts.postStatelessMCP(ts.mcpScopedAdminToken(), "tools/list", "", map[string]any{})
	var listed struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	ts.Require().NoErrorf(json.Unmarshal([]byte(message), &listed), "tools/list returned: %s", body)
	names := make([]string, 0, len(listed.Result.Tools))
	for _, tool := range listed.Result.Tools {
		names = append(names, tool.Name)
	}
	ts.Contains(names, "list_authorization_detail_types")
	ts.Contains(names, "get_authorization_detail_type_by_id")
	ts.NotContains(names, "create_authorization_detail_type")
	ts.NotContains(names, "update_authorization_detail_type")
}

// The get tool refuses an unknown id and says which tool failed.
func (ts *MCPTestSuite) TestAuthorizationDetailTypeTools_RefuseAnUnknownID() {
	missing, raw := ts.callMCPTool(ts.mcpScopedAdminToken(), "get_authorization_detail_type_by_id",
		map[string]any{"id": "00000000-0000-7000-8000-00000000dead"})
	ts.Truef(missing.Result.IsError, "an unknown id must not be found: %s", raw)
	ts.Contains(raw, "failed to get authorization details type")
}
