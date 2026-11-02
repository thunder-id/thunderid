// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const mcpProtocolVersion = "2026-07-28"

type mcpToolResult struct {
	Result struct {
		IsError           bool           `json:"isError"`
		StructuredContent map[string]any `json:"structuredContent"`
	} `json:"result"`
}

// The project tools run the project service end to end: a project created through MCP gets its root
// organization unit, is listed and read back, and an update renames it.
func (ts *ProjectAPITestSuite) TestProjectMCPTools() {
	created, raw := ts.callMCPTool("create_project", map[string]any{
		"handle": ts.handle("mcp"), "displayName": "MCP Project", "description": "Created through MCP",
	})
	ts.Require().Falsef(created.Result.IsError, "create failed: %s", raw)
	id, _ := created.Result.StructuredContent["id"].(string)
	ts.Require().NotEmpty(id)
	ts.projects = append(ts.projects, id)
	ts.Equal(ts.handle("mcp"), created.Result.StructuredContent["handle"])
	ts.NotEmpty(created.Result.StructuredContent["rootOuId"])

	got, raw := ts.callMCPTool("get_project_by_id", map[string]any{"id": id})
	ts.Require().Falsef(got.Result.IsError, "get failed: %s", raw)
	ts.Equal("MCP Project", got.Result.StructuredContent["displayName"])

	listed, raw := ts.callMCPTool("list_projects", map[string]any{})
	ts.Require().Falsef(listed.Result.IsError, "list failed: %s", raw)
	ts.Contains(raw, id)

	updated, raw := ts.callMCPTool("update_project", map[string]any{
		"id": id, "displayName": "MCP Project Renamed",
	})
	ts.Require().Falsef(updated.Result.IsError, "update failed: %s", raw)
	ts.Equal("MCP Project Renamed", updated.Result.StructuredContent["displayName"])
	ts.Equal(ts.handle("mcp"), updated.Result.StructuredContent["handle"])

	duplicate, raw := ts.callMCPTool("create_project", map[string]any{
		"handle": ts.handle("mcp"), "displayName": "Duplicate",
	})
	ts.Truef(duplicate.Result.IsError, "a duplicate handle must be refused: %s", raw)
}

// callMCPTool sends a stateless tools/call request with the management token and returns the decoded
// result with the raw response body, which an SSE response carries on its data line.
func (ts *ProjectAPITestSuite) callMCPTool(name string, args map[string]any) (mcpToolResult, string) {
	payload, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{
			"name": name, "arguments": args,
			"_meta": map[string]any{
				"io.modelcontextprotocol/protocolVersion":    mcpProtocolVersion,
				"io.modelcontextprotocol/clientCapabilities": map[string]any{},
				"io.modelcontextprotocol/clientInfo": map[string]any{
					"name": "thunderid-integration-test", "version": "1.0.0"},
			},
		},
	})
	ts.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+"/mcp", bytes.NewReader(payload))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", mcpProtocolVersion)
	req.Header.Set("Mcp-Method", "tools/call")
	req.Header.Set("Mcp-Name", name)

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Require().Equalf(http.StatusOK, resp.StatusCode, "%s failed: %s", name, string(body))

	message := string(body)
	for _, line := range strings.Split(message, "\n") {
		if data, ok := strings.CutPrefix(line, "data: "); ok {
			message = data
		}
	}
	var result mcpToolResult
	ts.Require().NoErrorf(json.Unmarshal([]byte(message), &result), "%s returned: %s", name, string(body))
	return result, string(body)
}
