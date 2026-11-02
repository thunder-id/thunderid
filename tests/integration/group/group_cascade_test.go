// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package group

import (
	"encoding/json"
	"net/http"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// This file extends GroupAPITestSuite with coverage for the dependency cascade that runs when a
// principal is deleted. Group memberships are cleaned up rather than blocking the delete, so a
// deleted user must not linger as a member, and a deleted group must disappear both from the groups
// that contain it and from the roles assigned to it.

// TestDeletingUserRemovesGroupMembership verifies deleting a user cascades to the memberships it
// held, leaving no stale member behind.
func (suite *GroupAPITestSuite) TestDeletingUserRemovesGroupMembership() {
	tempUserID, err := testutils.CreateUser(testutils.User{
		OUID: testOUID,
		Type: testUserType.Handle,
		Attributes: json.RawMessage(`{
			"email": "cascade-user@example.com",
			"given_name": "Cascade",
			"family_name": "User",
			"password": "TestPassword123!"
		}`),
	})
	suite.Require().NoError(err)
	// The delete below is the action under test; this only covers the paths that abort before it.
	defer func() { _ = testutils.DeleteUser(tempUserID) }()

	groupID, err := createGroup(CreateGroupRequest{
		Name: "Cascade Membership Group",
		OUID: testOUID,
		Members: []Member{
			{Id: tempUserID, Type: MemberTypeUser},
		},
	})
	suite.Require().NoError(err)
	defer func() {
		if deleteErr := deleteGroup(groupID); deleteErr != nil {
			suite.T().Logf("Failed to clean up group: %v", deleteErr)
		}
	}()

	before := suite.memberList(groupID)
	suite.Require().Equal(1, before.TotalResults, "the group should hold one member before deletion")
	suite.Require().Len(before.Members, 1, "the user should be a member before deletion")

	suite.Require().NoError(testutils.DeleteUser(tempUserID))

	// TotalResults comes from the stored member rows, while Members is the resolved view that drops
	// members whose entity no longer exists. Both have to reach zero: asserting only on Members would
	// pass even if the cascade left the membership row behind.
	after := suite.memberList(groupID)
	suite.Equal(0, after.TotalResults, "deleting a user should remove the stored membership row")
	suite.Empty(after.Members, "the deleted user should not remain a member")
}

// TestDeletingGroupRemovesNestingAndRoleAssignment verifies deleting a group cascades to both the
// groups that nest it and the roles assigned to it.
func (suite *GroupAPITestSuite) TestDeletingGroupRemovesNestingAndRoleAssignment() {
	nestedGroupID, err := createGroup(CreateGroupRequest{
		Name:    "Cascade Nested Group",
		OUID:    testOUID,
		Members: []Member{},
	})
	suite.Require().NoError(err)

	nestedGroupDeleted := false
	defer func() {
		if nestedGroupDeleted {
			return
		}
		if deleteErr := deleteGroup(nestedGroupID); deleteErr != nil {
			suite.T().Logf("Failed to clean up nested group: %v", deleteErr)
		}
	}()

	parentGroupID, err := createGroup(CreateGroupRequest{
		Name: "Cascade Parent Group",
		OUID: testOUID,
		Members: []Member{
			{Id: nestedGroupID, Type: MemberTypeGroup},
		},
	})
	suite.Require().NoError(err)
	defer func() {
		if deleteErr := deleteGroup(parentGroupID); deleteErr != nil {
			suite.T().Logf("Failed to clean up parent group: %v", deleteErr)
		}
	}()

	roleID, err := testutils.CreateRole(testutils.Role{
		Name: "Cascade Test Role",
		OUID: testOUID,
		Assignments: []testutils.Assignment{
			{ID: nestedGroupID, Type: "group"},
		},
	})
	suite.Require().NoError(err)
	defer func() {
		if deleteErr := testutils.DeleteRole(roleID); deleteErr != nil {
			suite.T().Logf("Failed to clean up role: %v", deleteErr)
		}
	}()

	assignments, err := testutils.GetRoleAssignments(roleID)
	suite.Require().NoError(err)
	suite.Require().Len(assignments, 1, "the group should be assigned to the role before deletion")

	suite.Require().NoError(deleteGroup(nestedGroupID))
	nestedGroupDeleted = true

	after := suite.memberList(parentGroupID)
	suite.Equal(0, after.TotalResults, "deleting a group should remove the stored nesting row")
	for _, member := range after.Members {
		suite.NotEqual(nestedGroupID, member.Id,
			"the deleted group should no longer be a member of its parent group")
	}

	assignmentsAfter, err := testutils.GetRoleAssignments(roleID)
	suite.Require().NoError(err)
	for _, assignment := range assignmentsAfter {
		suite.NotEqual(nestedGroupID, assignment.ID,
			"the deleted group should no longer hold a role assignment")
	}
}

// memberList returns the member listing reported for the given group.
func (suite *GroupAPITestSuite) memberList(groupID string) MemberListResponse {
	suite.T().Helper()

	req, err := http.NewRequest("GET", testServerURL+"/groups/"+groupID+"/members?limit=100", nil)
	suite.Require().NoError(err)

	resp, err := testutils.GetHTTPClient().Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Require().Equal(http.StatusOK, resp.StatusCode)

	var memberList MemberListResponse
	suite.Require().NoError(json.NewDecoder(resp.Body).Decode(&memberList))
	return memberList
}
