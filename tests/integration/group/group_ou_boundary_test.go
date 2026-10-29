// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package group

import (
	"encoding/json"
	"net/http"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// This file extends GroupAPITestSuite with the organization unit boundary on group membership: a
// group holds only members its OU owns or has shared with it, and a group moves to another OU only
// while it has no members and no other group contains it.

// createBoundaryOU creates an OU under the suite's test OU and registers its cleanup.
func (suite *GroupAPITestSuite) createBoundaryOU(handle string) string {
	suite.T().Helper()

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: handle,
		Name:   handle,
		Parent: &testOUID,
	})
	suite.Require().NoError(err)
	suite.T().Cleanup(func() {
		if deleteErr := testutils.DeleteOrganizationUnit(ouID); deleteErr != nil {
			suite.T().Logf("Failed to clean up organization unit: %v", deleteErr)
		}
	})
	return ouID
}

// createBoundaryGroup creates a group and registers its cleanup. Cleanups run in reverse order, so
// a group created after an OU is deleted before that OU.
func (suite *GroupAPITestSuite) createBoundaryGroup(name, ouID string, members []Member) string {
	suite.T().Helper()

	groupID, err := createGroup(CreateGroupRequest{Name: name, OUID: ouID, Members: members})
	suite.Require().NoError(err)
	suite.T().Cleanup(func() {
		if deleteErr := deleteGroup(groupID); deleteErr != nil {
			suite.T().Logf("Failed to clean up group: %v", deleteErr)
		}
	})
	return groupID
}

// TestCreateGroupWithMemberFromAnotherOU verifies a group cannot be created with a user from a
// different OU.
func (suite *GroupAPITestSuite) TestCreateGroupWithMemberFromAnotherOU() {
	otherOUID := suite.createBoundaryOU("group-boundary-create")
	otherUser := testutils.User{
		Type:       testUser.Type,
		OUID:       otherOUID,
		Attributes: json.RawMessage(`{"email": "boundary-create@example.com"}`),
	}
	otherUserID, err := testutils.CreateUser(otherUser)
	suite.Require().NoError(err)
	defer func() {
		if deleteErr := testutils.DeleteUser(otherUserID); deleteErr != nil {
			suite.T().Logf("Failed to clean up user: %v", deleteErr)
		}
	}()

	payload := suite.marshal(CreateGroupRequest{
		Name:    "Cross OU Member Group",
		OUID:    testOUID,
		Members: []Member{{Id: otherUserID, Type: MemberTypeUser}},
	})

	resp := suite.doGroupRequest(http.MethodPost, "/groups", payload)
	defer resp.Body.Close()

	suite.expectErrorWithJSONBody(resp, http.StatusBadRequest, "GRP-1019")
}

// TestAddNestedGroupFromAnotherOU verifies a group from a different OU cannot be nested in a group.
func (suite *GroupAPITestSuite) TestAddNestedGroupFromAnotherOU() {
	otherOUID := suite.createBoundaryOU("group-boundary-nested")
	otherGroupID := suite.createBoundaryGroup("Other OU Group", otherOUID, []Member{})

	payload := suite.marshal(MembersRequest{
		Members: []Member{{Id: otherGroupID, Type: MemberTypeGroup}},
	})

	resp := suite.doGroupRequest(http.MethodPost, "/groups/"+createdGroupID+"/members/add", payload)
	defer resp.Body.Close()

	suite.expectErrorWithJSONBody(resp, http.StatusBadRequest, "GRP-1019")
}

// TestMoveGroupWithMembers verifies a group that has members cannot move.
func (suite *GroupAPITestSuite) TestMoveGroupWithMembers() {
	targetOUID := suite.createBoundaryOU("group-boundary-move-member")
	groupID := suite.createBoundaryGroup("Move With Member Group", testOUID,
		[]Member{{Id: testUserID, Type: MemberTypeUser}})

	payload := suite.marshal(UpdateGroupRequest{Name: "Move With Member Group", OUID: targetOUID})

	resp := suite.doGroupRequest(http.MethodPut, "/groups/"+groupID, payload)
	defer resp.Body.Close()

	suite.expectErrorWithJSONBody(resp, http.StatusBadRequest, "GRP-1020")
}

// TestMoveGroupInsideAnotherGroup verifies a group that another group contains cannot move.
func (suite *GroupAPITestSuite) TestMoveGroupInsideAnotherGroup() {
	targetOUID := suite.createBoundaryOU("group-boundary-move-parent")
	childID := suite.createBoundaryGroup("Move Child Group", testOUID, []Member{})
	suite.createBoundaryGroup("Move Parent Group", testOUID, []Member{{Id: childID, Type: MemberTypeGroup}})

	payload := suite.marshal(UpdateGroupRequest{Name: "Move Child Group", OUID: targetOUID})

	resp := suite.doGroupRequest(http.MethodPut, "/groups/"+childID, payload)
	defer resp.Body.Close()

	suite.expectErrorWithJSONBody(resp, http.StatusBadRequest, "GRP-1020")
}

// TestMoveEmptyGroupToAnotherOU verifies a group with no memberships moves freely.
func (suite *GroupAPITestSuite) TestMoveEmptyGroupToAnotherOU() {
	targetOUID := suite.createBoundaryOU("group-boundary-move-empty")
	groupID := suite.createBoundaryGroup("Move Empty Group", testOUID, []Member{})

	payload := suite.marshal(UpdateGroupRequest{Name: "Move Empty Group", OUID: targetOUID})

	resp := suite.doGroupRequest(http.MethodPut, "/groups/"+groupID, payload)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)
	var group Group
	suite.Require().NoError(json.NewDecoder(resp.Body).Decode(&group))
	suite.Equal(targetOUID, group.OUID)
}

// TestAddMemberSharedWithGroupOU verifies a user owned by another OU becomes a valid member once a
// sharing policy reaches the group's OU with it.
func (suite *GroupAPITestSuite) TestAddMemberSharedWithGroupOU() {
	childOUID := suite.createBoundaryOU("group-boundary-shared")
	groupID := suite.createBoundaryGroup("Shared Member Group", childOUID, []Member{})
	addPayload := suite.marshal(MembersRequest{Members: []Member{{Id: testUserID, Type: MemberTypeUser}}})

	resp := suite.doGroupRequest(http.MethodPost, "/groups/"+groupID+"/members/add", addPayload)
	suite.expectErrorWithJSONBody(resp, http.StatusBadRequest, "GRP-1019")
	resp.Body.Close()

	policyResp := suite.doGroupRequest(http.MethodPost, "/users/"+testUserID+"/sharing-policies",
		suite.marshal(map[string]any{"childUnits": map[string]any{
			"mode":  "selected",
			"units": []map[string]string{{"ouId": childOUID, "reach": "unit"}},
		}}))
	suite.Require().Equal(http.StatusCreated, policyResp.StatusCode)
	var policy struct {
		ID string `json:"id"`
	}
	suite.Require().NoError(json.NewDecoder(policyResp.Body).Decode(&policy))
	policyResp.Body.Close()
	suite.T().Cleanup(func() {
		deleteResp := suite.doGroupRequest(http.MethodDelete,
			"/users/"+testUserID+"/sharing-policies/"+policy.ID, nil)
		deleteResp.Body.Close()
	})

	resp = suite.doGroupRequest(http.MethodPost, "/groups/"+groupID+"/members/add", addPayload)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)
}
