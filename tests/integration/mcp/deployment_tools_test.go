// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// selfManagementKey is the management API key the integration server trusts, so the server can be
// registered as a gateway of its own, as the gateway suite does.
const selfManagementKey = "integration-management-api-key"

// registerSelf registers the integration server as a gateway at baseURL and returns its ID and what
// removes it. The server is reachable by more than one name, so it can be registered once per name.
func (ts *MCPTestSuite) registerSelf(baseURL string) (string, func()) {
	ts.T().Helper()
	certificate, err := os.ReadFile(filepath.Join(testutils.GetExtractedProductHome(), "config", "certs",
		"server.cert"))
	ts.Require().NoError(err)
	body, err := json.Marshal(map[string]any{
		"name": fmt.Sprintf("mcp-self-%d", time.Now().UnixNano()), "baseUrl": baseURL,
		"key": selfManagementKey, "caCertificate": string(certificate),
	})
	ts.Require().NoError(err)
	req, err := http.NewRequest(http.MethodPost, testServerURL+"/gateways", bytes.NewReader(body))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()
	ts.Require().Equal(http.StatusCreated, resp.StatusCode)
	var registered struct {
		ID string `json:"id"`
	}
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&registered))
	return registered.ID, func() {
		remove, err := http.NewRequest(http.MethodDelete, testServerURL+"/gateways/"+registered.ID, nil)
		if err == nil {
			if resp, err := testutils.GetHTTPClient().Do(remove); err == nil {
				_ = resp.Body.Close()
			}
		}
	}
}

// tool calls a tool that must succeed and returns its structured answer.
func (ts *MCPTestSuite) tool(token, name string, args map[string]any) map[string]any {
	ts.T().Helper()
	result, raw := ts.callMCPTool(token, name, args)
	ts.Require().Falsef(result.Result.IsError, "%s failed: %s", name, raw)
	return result.Result.StructuredContent
}

// refused calls a tool that must fail and returns the raw answer, which names why.
func (ts *MCPTestSuite) refused(token, name string, args map[string]any) string {
	ts.T().Helper()
	result, raw := ts.callMCPTool(token, name, args)
	ts.Require().Truef(result.Result.IsError, "%s should have failed: %s", name, raw)
	return raw
}

// The deployment tools work through the MCP server as their REST endpoints do: a version is captured,
// read, compared with what a gateway runs, deployed, promoted to another gateway and rolled back.
func (ts *MCPTestSuite) TestDeploymentTools_DeployPromoteAndRollBack() {
	token := ts.mcpScopedAdminToken()
	source, removeSource := ts.registerSelf(testServerURL)
	defer removeSource()
	target, removeTarget := ts.registerSelf("https://127.0.0.1:8095")
	defer removeTarget()

	listed := ts.tool(token, "thunderid_list_gateways", map[string]any{})
	ts.Contains(fmt.Sprint(listed["gateways"]), source)

	captured := ts.tool(token, "thunderid_capture_configuration_version", map[string]any{"name": "mcp tools"})
	version, _ := captured["version"].(string)
	ts.Require().NotEmpty(version)
	versions := ts.tool(token, "thunderid_list_configuration_versions", map[string]any{})
	ts.Contains(fmt.Sprint(versions["versions"]), version)
	read := ts.tool(token, "thunderid_get_configuration_version", map[string]any{"version": version})
	ts.NotEmpty(read["resources"])
	ts.Contains(ts.refused(token, "thunderid_get_configuration_version", map[string]any{"version": "0000000"}),
		"GTW-")

	diff := ts.tool(token, "thunderid_diff_version", map[string]any{"gatewayId": source, "version": version})
	ts.Equal(version, diff["toVersion"])

	// Nothing runs on the source yet, so there is nothing to promote or roll back.
	ts.Contains(ts.refused(token, "thunderid_promote_gateway",
		map[string]any{"sourceGatewayId": source, "targetGatewayId": target}), "runs no version yet")
	ts.Contains(ts.refused(token, "thunderid_promote_gateway",
		map[string]any{"sourceGatewayId": source, "targetGatewayId": source}), "same gateway")

	dryRun := ts.tool(token, "thunderid_deploy_version",
		map[string]any{"gatewayId": source, "version": version, "dryRun": true})
	ts.Equal(true, dryRun["dryRun"])
	ts.NotEqual(true, dryRun["recorded"])
	deployed := ts.tool(token, "thunderid_deploy_version", map[string]any{"gatewayId": source, "version": version})
	ts.Equalf(true, deployed["recorded"], "the deploy was not recorded: %v", deployed["import"])

	applied := ts.tool(token, "thunderid_get_gateway_applied_version", map[string]any{"gatewayId": source})
	ts.Equal(version, applied["appliedVersion"])
	configuration := ts.tool(token, "thunderid_get_gateway_configuration", map[string]any{"gatewayId": source})
	ts.Equal(version, configuration["version"])

	promoted := ts.tool(token, "thunderid_promote_gateway",
		map[string]any{"sourceGatewayId": source, "targetGatewayId": target, "dryRun": true})
	ts.Equal(true, promoted["dryRun"])

	// The source held no version before this one, so there is nothing to roll back to.
	ts.Contains(ts.refused(token, "thunderid_rollback_gateway", map[string]any{"gatewayId": source, "dryRun": true}),
		"GTW-")
	ts.Contains(ts.refused(token, "thunderid_deploy_version", map[string]any{"gatewayId": "no-such-gateway"}), "GTW-")
}

// A gateway's variables and secrets are listed, set and removed through the MCP server, and a
// secret's value is never given back.
func (ts *MCPTestSuite) TestDeploymentTools_GatewayValues() {
	token := ts.mcpScopedAdminToken()
	gatewayID, remove := ts.registerSelf(testServerURL)
	defer remove()
	name := fmt.Sprintf("MCP_TOOLS_VALUE_%d", time.Now().UnixNano())

	set := ts.tool(token, "thunderid_set_gateway_variable",
		map[string]any{"gatewayId": gatewayID, "name": name, "value": "one"})
	ts.Equal("one", set["value"])
	variables := ts.tool(token, "thunderid_list_gateway_variables",
		map[string]any{"gatewayId": gatewayID, "names": name, "limit": 10, "offset": 0})
	ts.Contains(fmt.Sprint(variables["variables"]), "one")
	ts.tool(token, "thunderid_delete_gateway_variable", map[string]any{"gatewayId": gatewayID, "name": name})

	ts.tool(token, "thunderid_set_gateway_secret",
		map[string]any{"gatewayId": gatewayID, "name": name, "value": "s3cret-value"})
	secrets := ts.tool(token, "thunderid_list_gateway_secrets", map[string]any{"gatewayId": gatewayID, "names": name})
	ts.Contains(fmt.Sprint(secrets["secrets"]), name)
	ts.NotContains(fmt.Sprint(secrets), "s3cret-value")
	ts.tool(token, "thunderid_delete_gateway_secret", map[string]any{"gatewayId": gatewayID, "name": name})

	// The gateway's store refuses a name it cannot hold, and the refusal is passed on.
	ts.refused(token, "thunderid_set_gateway_variable",
		map[string]any{"gatewayId": gatewayID, "name": "not a name", "value": "x"})
	ts.Contains(ts.refused(token, "thunderid_list_gateway_variables", map[string]any{"gatewayId": "no-such-gateway"}),
		"GTW-")
}

// Export and import work through the MCP server as POST /export and POST /import do.
func (ts *MCPTestSuite) TestDeploymentTools_ExportAndImport() {
	token := ts.mcpScopedAdminToken()

	exported := ts.tool(token, "thunderid_export_configuration",
		map[string]any{"organizationUnits": []string{ts.ouID}})
	resources, _ := exported["resources"].(string)
	ts.Contains(resources, ts.ouID)

	imported := ts.tool(token, "thunderid_import_configuration", map[string]any{"content": resources, "dryRun": true})
	ts.NotNil(imported["summary"])

	ts.Contains(ts.refused(token, "thunderid_import_configuration",
		map[string]any{"content": "resource_type: organization_unit\nname: {{.MCP_TOOLS_MISSING}}\n"}), "IMP-")
	ts.Contains(ts.refused(token, "thunderid_export_configuration",
		map[string]any{"organizationUnits": []string{"00000000-0000-0000-0000-000000000000"}}), "EXP-1002")
}
