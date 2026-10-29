// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importexport

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// The organization units the declarative test resources define: a root and a direct child of it.
const (
	roleSharingRootOU  = "decl-m2m-root"
	roleSharingChildOU = "decl-m2m-child-a"
)

// TestImportRoleWithSharingPolicies covers a role document carrying sharing policies: the import
// records the policies, and importing the same document again updates the role in place.
func (suite *ImportExportErrorSuite) TestImportRoleWithSharingPolicies() {
	id := newImportUUID(suite)
	defer suite.deleteImportedRole(id)
	document := roleSharingDocument(id, roleSharingChildOU)

	result := suite.importRole(document)
	suite.Require().Equal("success", result.Status, "message: %s", result.Message)
	suite.Equal("create", result.Operation)
	suite.Equal(1, suite.sharingPolicyCount(id))
	suite.Equal(http.StatusOK, suite.getStatus("/roles/"+id+"/overlay-rules?ouId="+roleSharingChildOU),
		"the imported policy reaches the child")

	result = suite.importRole(document)
	suite.Require().Equal("success", result.Status, "message: %s", result.Message)
	suite.Equal("update", result.Operation)
	suite.Equal(1, suite.sharingPolicyCount(id), "the replayed policy replaces the one already recorded")
}

// TestImportRoleWithRefusedSharingPolicy covers a role document whose policy is refused: the import
// fails and the role it created is removed, so a retry does not meet a half-imported role.
func (suite *ImportExportErrorSuite) TestImportRoleWithRefusedSharingPolicy() {
	id := newImportUUID(suite)
	defer suite.deleteImportedRole(id)

	// decl-ou-1 is the root of another tree, not a child of the role's organization unit.
	result := suite.importRole(roleSharingDocument(id, "decl-ou-1"))

	suite.Equal("failed", result.Status)
	suite.Contains(result.Message, "sharing policy 1")
	suite.Equal(http.StatusNotFound, suite.getStatus("/roles/"+id))
}

// TestImportRoleWithMisspelledPolicyField covers a policy field the format does not declare: the
// document fails rather than sharing the role with the unit it meant to exclude.
func (suite *ImportExportErrorSuite) TestImportRoleWithMisspelledPolicyField() {
	id := newImportUUID(suite)
	defer suite.deleteImportedRole(id)
	document := fmt.Sprintf("resource_type: role\nid: %s\nname: %q\nouId: %s\npermissions: []\n"+
		"sharingPolicies:\n  - childUnits:\n      mode: all\n      excludeOuIds: [%s]\n",
		id, "Misspelled Shared Role "+id[:8], roleSharingRootOU, roleSharingChildOU)

	result := suite.importRole(document)

	suite.Equal("failed", result.Status)
	suite.Contains(result.Message, "excludeOuIds")
	suite.Equal(http.StatusNotFound, suite.getStatus("/roles/"+id))
}

// importRole imports one role document and returns its result.
func (suite *ImportExportErrorSuite) importRole(content string) testutils.ImportItem {
	payload := suite.importBody(testutils.ImportRequest{Content: content, Options: runtimeOptions()})

	status, body := suite.postJSON(suite.client, "/import", payload)
	suite.Require().Equal(http.StatusOK, status, "body: %s", body)

	var resp testutils.ImportResponse
	suite.Require().NoError(json.Unmarshal(body, &resp))
	suite.Require().Len(resp.Results, 1, "body: %s", body)
	return resp.Results[0]
}

// sharingPolicyCount returns how many sharing policies a role holds.
func (suite *ImportExportErrorSuite) sharingPolicyCount(roleID string) int {
	resp, err := suite.client.Get(testutils.TestServerURL + "/roles/" + roleID + "/sharing-policies")
	suite.Require().NoError(err)
	defer resp.Body.Close()
	suite.Require().Equal(http.StatusOK, resp.StatusCode)

	var list struct {
		TotalResults int `json:"totalResults"`
	}
	suite.Require().NoError(json.NewDecoder(resp.Body).Decode(&list))
	return list.TotalResults
}

// getStatus returns the status a GET on a path answers with.
func (suite *ImportExportErrorSuite) getStatus(path string) int {
	resp, err := suite.client.Get(testutils.TestServerURL + path)
	suite.Require().NoError(err)
	defer resp.Body.Close()
	return resp.StatusCode
}

// deleteImportedRole removes a role created by an import.
func (suite *ImportExportErrorSuite) deleteImportedRole(id string) {
	if err := testutils.DeleteRole(id); err != nil {
		suite.T().Logf("Failed to delete imported role %s: %v", id, err)
	}
}

// roleSharingDocument builds a role import document owned by the test root, sharing the role with one
// named organization unit.
func roleSharingDocument(id, sharedOUID string) string {
	return fmt.Sprintf("resource_type: role\nid: %s\nname: %q\nouId: %s\npermissions: []\n"+
		"sharingPolicies:\n  - childUnits:\n      mode: selected\n      units:\n"+
		"        - ouId: %s\n          reach: unit\n",
		id, "Imported Shared Role "+id[:8], roleSharingRootOU, sharedOUID)
}
