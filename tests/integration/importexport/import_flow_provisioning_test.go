// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importexport

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// flowProvisioningRoleNotFoundCode is the flow save error for a provisioning step that assigns a missing role.
const flowProvisioningRoleNotFoundCode = "FLM-1050"

// provisioningFlowDocument builds a registration flow import document whose provisioning step assigns roleID.
func provisioningFlowDocument(id, handle, roleID string) string {
	return fmt.Sprintf("resource_type: flow\nid: %s\nhandle: %s\nname: Import Provisioning Flow\n"+
		"flowType: REGISTRATION\nouHandle: default\nnodes:\n"+
		"  - id: start\n    type: START\n    onSuccess: user_type_resolver\n"+
		"  - id: user_type_resolver\n    type: TASK_EXECUTION\n    executor:\n      name: UserTypeResolver\n"+
		"    onSuccess: provisioning\n"+
		"  - id: provisioning\n    type: TASK_EXECUTION\n    executor:\n      name: ProvisioningExecutor\n"+
		"    properties:\n      assignRole: %s\n    onSuccess: end\n"+
		"  - id: end\n    type: END\n", id, handle, roleID)
}

// An import creates roles before flows, so a flow may assign a role the same import creates.
func (suite *ImportExportErrorSuite) TestImportFlowAssigningARoleOfTheSameImport() {
	flowID := newImportUUID(suite)
	roleID := newImportUUID(suite)
	defer func() { _ = testutils.DeleteFlow(flowID) }()
	defer suite.deleteImportedRole(roleID)
	content := provisioningFlowDocument(flowID, "import-provisioning-same-import", roleID) + "---\n" +
		fmt.Sprintf("resource_type: role\nid: %s\nname: %q\nouHandle: default\npermissions: []\n",
			roleID, "Import Provisioning Role "+roleID[:8])

	payload := suite.importBody(testutils.ImportRequest{Content: content, Options: runtimeOptions()})
	status, body := suite.postJSON(suite.client, "/import", payload)

	suite.Require().Equal(http.StatusOK, status, "body: %s", body)
	var resp testutils.ImportResponse
	suite.Require().NoError(json.Unmarshal(body, &resp))
	suite.Require().Len(resp.Results, 2, "body: %s", body)
	for _, result := range resp.Results {
		suite.Equal("success", result.Status, "%s %s: %s", result.ResourceType, result.ResourceID, result.Message)
	}
}

func (suite *ImportExportErrorSuite) TestImportFlowAssigningAMissingRoleFails() {
	flowID := newImportUUID(suite)
	defer func() { _ = testutils.DeleteFlow(flowID) }()

	result := suite.importFlow(provisioningFlowDocument(flowID, "import-provisioning-missing-role",
		"00000000-0000-0000-0000-00000000c0de"), false, false)

	suite.Equal("failed", result.Status, "message: %s", result.Message)
	suite.Equal(flowProvisioningRoleNotFoundCode, result.Code)
}

// A role that fails to import does not exist, so a flow that assigns it is refused and not saved.
func (suite *ImportExportErrorSuite) TestImportFlowAssigningARoleThatFailsToImportFails() {
	flowID := newImportUUID(suite)
	roleID := newImportUUID(suite)
	defer func() { _ = testutils.DeleteFlow(flowID) }()
	defer suite.deleteImportedRole(roleID)
	// The role names an organization unit that does not exist, so its import fails.
	content := provisioningFlowDocument(flowID, "import-provisioning-failed-role", roleID) + "---\n" +
		fmt.Sprintf("resource_type: role\nid: %s\nname: %q\nouHandle: no-such-ou-for-provisioning\npermissions: []\n",
			roleID, "Import Provisioning Failed Role "+roleID[:8])

	payload := suite.importBody(testutils.ImportRequest{Content: content, Options: runtimeOptions()})
	status, body := suite.postJSON(suite.client, "/import", payload)

	suite.Require().Equal(http.StatusOK, status, "body: %s", body)
	var resp testutils.ImportResponse
	suite.Require().NoError(json.Unmarshal(body, &resp))
	suite.Require().Len(resp.Results, 2, "body: %s", body)
	for _, result := range resp.Results {
		suite.Equal("failed", result.Status, "%s %s should fail: %s", result.ResourceType, result.ResourceID, body)
		if result.ResourceType == "flow" {
			suite.Equal(flowProvisioningRoleNotFoundCode, result.Code)
		}
	}
	suite.Empty(suite.flowOwner(flowID), "The flow should not be saved")
}

// A role's assignments are added after the flows, so one that is refused fails the role but leaves it in place, and
// the flow that assigns it stays saved.
func (suite *ImportExportErrorSuite) TestImportRoleWhoseAssignmentIsRefusedKeepsTheRoleAndTheFlow() {
	flowID := newImportUUID(suite)
	roleID := newImportUUID(suite)
	defer func() { _ = testutils.DeleteFlow(flowID) }()
	defer suite.deleteImportedRole(roleID)
	// The assignee does not exist, so adding the assignment is refused.
	content := provisioningFlowDocument(flowID, "import-provisioning-refused-assignment", roleID) + "---\n" +
		fmt.Sprintf("resource_type: role\nid: %s\nname: %q\nouHandle: default\npermissions: []\n"+
			"assignments:\n  - id: %s\n    type: user\n", roleID, "Import Provisioning Refused Role "+roleID[:8],
			newImportUUID(suite))

	payload := suite.importBody(testutils.ImportRequest{Content: content, Options: runtimeOptions()})
	status, body := suite.postJSON(suite.client, "/import", payload)

	suite.Require().Equal(http.StatusOK, status, "body: %s", body)
	var resp testutils.ImportResponse
	suite.Require().NoError(json.Unmarshal(body, &resp))
	suite.Require().Len(resp.Results, 2, "body: %s", body)
	for _, result := range resp.Results {
		switch result.ResourceType {
		case "role":
			suite.Equal("failed", result.Status, "The refused assignment should fail the role: %s", body)
		case "flow":
			suite.Equal("success", result.Status, "The flow should stay saved: %s", body)
		}
	}
	suite.Equal(http.StatusOK, suite.getStatus("/roles/"+roleID), "The role should stay")
	suite.NotEmpty(suite.flowOwner(flowID), "The flow should be saved")
}

// A group's members and a role's assignments name an application and a user imported after the flows, so they are added
// once those exist. The group and role are still created before the flow that assigns the role.
func (suite *ImportExportErrorSuite) TestImportAddsMembersAndAssignmentsThatNameResourcesOfTheSameImport() {
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	ouID, err := testutils.DefaultOUID()
	suite.Require().NoError(err)
	typeHandle := "import-link-person-" + suffix
	typeID, err := testutils.CreateUserType(testutils.UserType{
		Handle: typeHandle, DisplayName: "Import link person", OUID: ouID,
		Schema: map[string]interface{}{"username": map[string]interface{}{"type": "string"}},
	})
	suite.Require().NoError(err)
	defer func() { _ = testutils.DeleteUserType(typeID) }()

	flowID, roleID, groupID := newImportUUID(suite), newImportUUID(suite), newImportUUID(suite)
	userID, appID := newImportUUID(suite), newImportUUID(suite)
	defer func() { _ = testutils.DeleteFlow(flowID) }()
	defer suite.deleteImportedRole(roleID)
	defer func() { _ = testutils.DeleteGroup(groupID) }()
	defer func() { _ = testutils.DeleteApplication(appID) }()
	defer func() { _ = testutils.DeleteUser(userID) }()

	// The group and role come first in the content, the user and application last.
	content := fmt.Sprintf("resource_type: group\nid: %s\nname: %q\nouHandle: default\nmembers:\n"+
		"  - id: %s\n    type: user\n  - id: %s\n    type: app\n", groupID, "Import Link Group "+suffix,
		userID, appID) + "---\n" +
		fmt.Sprintf("resource_type: role\nid: %s\nname: %q\nouHandle: default\npermissions: []\nassignments:\n"+
			"  - id: %s\n    type: user\n  - id: %s\n    type: app\n  - id: %s\n    type: group\n",
			roleID, "Import Link Role "+suffix, userID, appID, groupID) + "---\n" +
		provisioningFlowDocument(flowID, "import-provisioning-link-"+suffix, roleID) + "---\n" +
		fmt.Sprintf("resource_type: user\nid: %s\ntype: %s\nouId: %s\nattributes:\n  username: import-link-%s\n",
			userID, typeHandle, ouID, suffix) + "---\n" +
		fmt.Sprintf("resource_type: application\nid: %s\nname: %q\ntype: custom\nouHandle: default\n",
			appID, "Import Link App "+suffix)

	payload := suite.importBody(testutils.ImportRequest{Content: content, Options: runtimeOptions()})
	status, body := suite.postJSON(suite.client, "/import", payload)

	suite.Require().Equal(http.StatusOK, status, "body: %s", body)
	var resp testutils.ImportResponse
	suite.Require().NoError(json.Unmarshal(body, &resp))
	suite.Require().Equal(0, resp.Summary.Failed, "results: %+v", resp.Results)
	suite.Equal(5, resp.Summary.Imported, "body: %s", body)

	members, err := testutils.GetGroupMembers(groupID)
	suite.Require().NoError(err)
	memberIDs := make([]string, 0, len(members))
	for _, member := range members {
		memberIDs = append(memberIDs, member.ID)
	}
	suite.ElementsMatch([]string{userID, appID}, memberIDs, "The group should hold the user and the application")

	assigneeIDs := suite.assignmentIDsIn(roleID, ouID)
	for _, assignee := range []string{userID, appID, groupID} {
		suite.True(slices.Contains(assigneeIDs, assignee),
			"The role should be assigned to %s: %v", assignee, assigneeIDs)
	}
}
