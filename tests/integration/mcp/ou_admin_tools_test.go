// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const ouAdminPassword = "MCPOUAdmin@123"

// ouAdminToken signs in an administrator holding the org permission in a new organization unit under
// parentID, and returns its MCP token and its unit. Everything it creates is deleted when the test ends.
func (ts *MCPTestSuite) ouAdminToken(parentID string) (string, string) {
	suffix := time.Now().UnixNano()
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: fmt.Sprintf("mcp-ou-admin-%d", suffix), Name: "MCP OU Admin", Parent: &parentID})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteOrganizationUnit(ouID) })

	userType := testutils.UserType{
		Handle:      fmt.Sprintf("mcp-ou-admin-type-%d", suffix),
		DisplayName: "MCP OU Admin Type",
		OUID:        ouID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string", "unique": true},
			"password": map[string]interface{}{"type": "string", "credential": true},
		},
	}
	userTypeID, err := testutils.CreateUserType(userType)
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteUserType(userTypeID) })

	username := fmt.Sprintf("mcp-ou-admin-%d", suffix)
	userID, err := testutils.CreateUser(testutils.User{
		Type: userType.Handle,
		OUID: ouID,
		Attributes: json.RawMessage(fmt.Sprintf(`{"username": %q, "password": %q}`,
			username, ouAdminPassword)),
	})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteUser(userID) })

	roleID, err := testutils.CreateRole(testutils.Role{
		Name: fmt.Sprintf("mcp-ou-admin-role-%d", suffix),
		OUID: ouID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: ts.resourceServerID, Permissions: []string{orgActionHandle}},
		},
		Assignments: []testutils.Assignment{{ID: userID, Type: "user"}},
	})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteRole(roleID) })

	var signIn testutils.OUSignInApps
	ts.T().Cleanup(func() { _ = signIn.DeleteAll() })
	token, err := signIn.ObtainAccessToken(ouID, orgActionHandle, username, ouAdminPassword,
		mcpResourceIdentifier)
	ts.Require().NoError(err)
	ts.Require().NotEmpty(token.AccessToken)
	return token.AccessToken, ouID
}

// shareRoleInto shares a role with one organization unit, and removes the sharing when the test ends.
func (ts *MCPTestSuite) shareRoleInto(roleID, ouID string) {
	body, err := json.Marshal(map[string]interface{}{
		"childUnits": map[string]interface{}{
			"mode": "selected", "units": []map[string]string{{"ouId": ouID, "reach": "unit"}},
		},
	})
	ts.Require().NoError(err)
	path := testServerURL + "/roles/" + roleID + "/sharing-policies"
	req, err := http.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()
	ts.Require().Equal(http.StatusCreated, resp.StatusCode)
	var created struct {
		ID string `json:"id"`
	}
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&created))
	ts.T().Cleanup(func() {
		req, err := http.NewRequest(http.MethodDelete, path+"/"+created.ID, nil)
		if err == nil {
			if resp, err := testutils.GetHTTPClient().Do(req); err == nil {
				_ = resp.Body.Close()
			}
		}
	})
}

// An administrator of one organization unit finds by name a role shared into its unit by the unit above.
func (ts *MCPTestSuite) TestOUAdminTools_FindRoleSharedIntoUnit() {
	agentType, raw := ts.defaultAgentType(ts.mcpScopedAdminToken())
	parentID, _ := agentType["ouId"].(string)
	ts.Require().NotEmpty(parentID, raw)
	token, ouID := ts.ouAdminToken(parentID)

	name := fmt.Sprintf("mcp-shared-role-%d", time.Now().UnixNano())
	roleID, err := testutils.CreateRole(testutils.Role{Name: name, OUID: parentID})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteRole(roleID) })
	ts.shareRoleInto(roleID, ouID)

	result, raw := ts.callMCPTool(token, "list_roles", map[string]any{"name": name, "ouId": ouID})
	var listed struct {
		Roles []struct {
			ID string `json:"id"`
		} `json:"roles"`
	}
	ts.decodeToolResult(result, raw, &listed)
	ts.Require().Lenf(listed.Roles, 1, "the shared role must be found: %s", raw)
	ts.Equal(roleID, listed.Roles[0].ID)
}

// An administrator of one organization unit gives its agent approval with a flow owned by another unit,
// and the tool reads that flow to say why it can't be used.
func (ts *MCPTestSuite) TestOUAdminTools_ApprovalFlowOfAnotherUnit() {
	agentType, raw := ts.defaultAgentType(ts.mcpScopedAdminToken())
	parentID, _ := agentType["ouId"].(string)
	ts.Require().NotEmpty(parentID, raw)
	token, ouID := ts.ouAdminToken(parentID)

	flowID, err := testutils.GetFlowIDByHandle("default-flow", "AUTHENTICATION")
	ts.Require().NoError(err)
	agentID, err := testutils.CreateAgent(testutils.Agent{
		Type: "default", OUID: ouID,
		Attributes: map[string]interface{}{"name": fmt.Sprintf("mcp-ou-admin-agent-%d", time.Now().UnixNano())},
	})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteAgent(agentID) })

	refused, raw := ts.callMCPTool(token, "update_agent", map[string]any{
		"agentId": agentID, "actsForUsers": []string{"approval"}, "authFlowId": flowID})
	ts.Truef(refused.Result.IsError, "approval must be refused with the default flow: %s", raw)
	ts.Contains(raw, "can't reach a user who isn't there")
	ts.NotContains(raw, "names no flow")
}
