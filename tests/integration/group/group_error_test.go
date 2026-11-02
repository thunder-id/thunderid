// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package group

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// This file extends GroupAPITestSuite with the rejection paths of the group API: member validation,
// CRUD conflicts and lookups that fail, malformed or invalid request bodies, pagination parameters
// that are not numbers, and handle paths that do not resolve to a usable OU.

// doGroupRequest issues a request against the group API using the admin client.
func (suite *GroupAPITestSuite) doGroupRequest(method, path string, body []byte) *http.Response {
	suite.T().Helper()

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, testServerURL+path, bodyReader)
	suite.Require().NoError(err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := testutils.GetHTTPClient().Do(req)
	suite.Require().NoError(err)
	return resp
}

// expectErrorWithJSONBody asserts the response carries the given status and error code, and returns the body.
func (suite *GroupAPITestSuite) expectErrorWithJSONBody(resp *http.Response, status int, code string) ErrorResponse {
	suite.T().Helper()

	var errorResp ErrorResponse
	suite.Require().NoError(json.NewDecoder(resp.Body).Decode(&errorResp))
	suite.Equal(status, resp.StatusCode)
	suite.Equal(code, errorResp.Code)
	return errorResp
}

// marshal encodes a request body, failing the test on error.
func (suite *GroupAPITestSuite) marshal(v any) []byte {
	suite.T().Helper()
	payload, err := json.Marshal(v)
	suite.Require().NoError(err)
	return payload
}

// ---------------------------------------------------------------------------
// Member validation
// ---------------------------------------------------------------------------

// TestAddMemberWithMismatchedType verifies a member whose claimed type disagrees with the actual
// entity category is rejected, so a user cannot be smuggled in as an application.
func (suite *GroupAPITestSuite) TestAddMemberWithMismatchedType() {
	payload := suite.marshal(MembersRequest{
		Members: []Member{
			{Id: testUserID, Type: MemberTypeApp},
		},
	})

	resp := suite.doGroupRequest(http.MethodPost, "/groups/"+createdGroupID+"/members/add", payload)
	defer resp.Body.Close()

	errorResp := suite.expectErrorWithJSONBody(resp, http.StatusBadRequest, "GRP-1007")
	suite.Equal("Invalid member ID", errorResp.Message.DefaultValue)
}

// TestCreateGroupWithConflictingMemberTypes verifies the same member ID cannot be listed twice under
// two different types in one request.
func (suite *GroupAPITestSuite) TestCreateGroupWithConflictingMemberTypes() {
	payload := suite.marshal(CreateGroupRequest{
		Name: "Group with Conflicting Member Types",
		OUID: testOUID,
		Members: []Member{
			{Id: testUserID, Type: MemberTypeUser},
			{Id: testUserID, Type: MemberTypeApp},
		},
	})

	resp := suite.doGroupRequest(http.MethodPost, "/groups", payload)
	defer resp.Body.Close()

	errorResp := suite.expectErrorWithJSONBody(resp, http.StatusBadRequest, "GRP-1007")
	suite.Equal("Invalid member ID", errorResp.Message.DefaultValue)
}

// TestAddMemberWithEmptyID verifies a member entry carrying no ID is rejected.
func (suite *GroupAPITestSuite) TestAddMemberWithEmptyID() {
	payload := suite.marshal(MembersRequest{
		Members: []Member{
			{Id: "", Type: MemberTypeUser},
		},
	})

	resp := suite.doGroupRequest(http.MethodPost, "/groups/"+createdGroupID+"/members/add", payload)
	defer resp.Body.Close()

	suite.expectErrorWithJSONBody(resp, http.StatusBadRequest, "GRP-1001")
}

// TestAddMemberWithUnknownType verifies a member type outside user/app/agent/group is rejected.
func (suite *GroupAPITestSuite) TestAddMemberWithUnknownType() {
	payload := suite.marshal(map[string]any{
		"members": []map[string]string{{"id": testUserID, "type": "robot"}},
	})

	resp := suite.doGroupRequest(http.MethodPost, "/groups/"+createdGroupID+"/members/add", payload)
	defer resp.Body.Close()

	errorResp := suite.expectErrorWithJSONBody(resp, http.StatusBadRequest, "GRP-1014")
	suite.Equal("Invalid member type", errorResp.Message.DefaultValue)
}

// ---------------------------------------------------------------------------
// CRUD conflicts and failed lookups
// ---------------------------------------------------------------------------

// TestCreateGroupWithDuplicateName verifies two groups cannot share a name within one OU.
func (suite *GroupAPITestSuite) TestCreateGroupWithDuplicateName() {
	existingID, err := createGroup(CreateGroupRequest{
		Name:    "Duplicate Name Group",
		OUID:    testOUID,
		Members: []Member{},
	})
	suite.Require().NoError(err)
	defer func() {
		if deleteErr := deleteGroup(existingID); deleteErr != nil {
			suite.T().Logf("Failed to clean up group: %v", deleteErr)
		}
	}()

	payload := suite.marshal(CreateGroupRequest{
		Name:    "Duplicate Name Group",
		OUID:    testOUID,
		Members: []Member{},
	})

	resp := suite.doGroupRequest(http.MethodPost, "/groups", payload)
	defer resp.Body.Close()

	errorResp := suite.expectErrorWithJSONBody(resp, http.StatusConflict, "GRP-1004")
	suite.Equal("Group name conflict", errorResp.Message.DefaultValue)
}

// TestUpdateGroupToDuplicateName verifies a rename cannot collide with a sibling group's name.
func (suite *GroupAPITestSuite) TestUpdateGroupToDuplicateName() {
	occupantID, err := createGroup(CreateGroupRequest{
		Name:    "Occupied Name Group",
		OUID:    testOUID,
		Members: []Member{},
	})
	suite.Require().NoError(err)
	defer func() {
		if deleteErr := deleteGroup(occupantID); deleteErr != nil {
			suite.T().Logf("Failed to clean up occupant group: %v", deleteErr)
		}
	}()

	renamableID, err := createGroup(CreateGroupRequest{
		Name:    "Renamable Group",
		OUID:    testOUID,
		Members: []Member{},
	})
	suite.Require().NoError(err)
	defer func() {
		if deleteErr := deleteGroup(renamableID); deleteErr != nil {
			suite.T().Logf("Failed to clean up renamable group: %v", deleteErr)
		}
	}()

	payload := suite.marshal(UpdateGroupRequest{
		Name: "Occupied Name Group",
		OUID: testOUID,
	})

	resp := suite.doGroupRequest(http.MethodPut, "/groups/"+renamableID, payload)
	defer resp.Body.Close()

	errorResp := suite.expectErrorWithJSONBody(resp, http.StatusConflict, "GRP-1004")
	suite.Equal("Group name conflict", errorResp.Message.DefaultValue)
}

// TestUpdateNonExistentGroup verifies updating an unknown group reports it as not found.
func (suite *GroupAPITestSuite) TestUpdateNonExistentGroup() {
	payload := suite.marshal(UpdateGroupRequest{
		Name: "Ghost Group",
		OUID: testOUID,
	})

	resp := suite.doGroupRequest(http.MethodPut, "/groups/non-existent-group-id", payload)
	defer resp.Body.Close()

	errorResp := suite.expectErrorWithJSONBody(resp, http.StatusNotFound, "GRP-1003")
	suite.Equal("Group not found", errorResp.Message.DefaultValue)
}

// TestDeleteNonExistentGroup verifies deleting an unknown group reports it as not found.
func (suite *GroupAPITestSuite) TestDeleteNonExistentGroup() {
	resp := suite.doGroupRequest(http.MethodDelete, "/groups/non-existent-group-id", nil)
	defer resp.Body.Close()

	errorResp := suite.expectErrorWithJSONBody(resp, http.StatusNotFound, "GRP-1003")
	suite.Equal("Group not found", errorResp.Message.DefaultValue)
}

// TestCreateGroupWithUnknownOU verifies a group cannot be created under an OU that does not exist.
func (suite *GroupAPITestSuite) TestCreateGroupWithUnknownOU() {
	payload := suite.marshal(CreateGroupRequest{
		Name:    "Group in Unknown OU",
		OUID:    "non-existent-ou-id",
		Members: []Member{},
	})

	resp := suite.doGroupRequest(http.MethodPost, "/groups", payload)
	defer resp.Body.Close()

	errorResp := suite.expectErrorWithJSONBody(resp, http.StatusBadRequest, "GRP-1005")
	suite.Equal("Invalid OU ID", errorResp.Message.DefaultValue)
}

// TestUpdateGroupToUnknownOU verifies a group cannot be moved into an OU that does not exist.
func (suite *GroupAPITestSuite) TestUpdateGroupToUnknownOU() {
	groupID, err := createGroup(CreateGroupRequest{
		Name:    "Group for Unknown OU Move",
		OUID:    testOUID,
		Members: []Member{},
	})
	suite.Require().NoError(err)
	defer func() {
		if deleteErr := deleteGroup(groupID); deleteErr != nil {
			suite.T().Logf("Failed to clean up group: %v", deleteErr)
		}
	}()

	payload := suite.marshal(UpdateGroupRequest{
		Name: "Group for Unknown OU Move",
		OUID: "non-existent-ou-id",
	})

	resp := suite.doGroupRequest(http.MethodPut, "/groups/"+groupID, payload)
	defer resp.Body.Close()

	errorResp := suite.expectErrorWithJSONBody(resp, http.StatusBadRequest, "GRP-1005")
	suite.Equal("Invalid OU ID", errorResp.Message.DefaultValue)
}

// TestUpdateGroupMovesItToAnotherOU verifies a group can be relocated to a different OU the caller
// administers, and that the new OU is reflected on the group afterwards.
func (suite *GroupAPITestSuite) TestUpdateGroupMovesItToAnotherOU() {
	targetOUID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "test-group-move-target-ou",
		Name:        "Group Move Target OU",
		Description: "Destination OU for the group move test",
	})
	suite.Require().NoError(err)
	defer func() {
		if deleteErr := testutils.DeleteOrganizationUnit(targetOUID); deleteErr != nil {
			suite.T().Logf("Failed to clean up target OU: %v", deleteErr)
		}
	}()

	groupID, err := createGroup(CreateGroupRequest{
		Name:    "Movable Group",
		OUID:    testOUID,
		Members: []Member{},
	})
	suite.Require().NoError(err)
	defer func() {
		if deleteErr := deleteGroup(groupID); deleteErr != nil {
			suite.T().Logf("Failed to clean up group: %v", deleteErr)
		}
	}()

	payload := suite.marshal(UpdateGroupRequest{
		Name: "Movable Group",
		OUID: targetOUID,
	})

	resp := suite.doGroupRequest(http.MethodPut, "/groups/"+groupID, payload)
	defer resp.Body.Close()

	suite.Require().Equal(http.StatusOK, resp.StatusCode)

	var updated Group
	suite.Require().NoError(json.NewDecoder(resp.Body).Decode(&updated))
	suite.Equal(targetOUID, updated.OUID, "the update response should carry the new OU")

	getResp := suite.doGroupRequest(http.MethodGet, "/groups/"+groupID, nil)
	defer getResp.Body.Close()

	suite.Require().Equal(http.StatusOK, getResp.StatusCode)

	var fetched Group
	suite.Require().NoError(json.NewDecoder(getResp.Body).Decode(&fetched))
	suite.Equal(targetOUID, fetched.OUID, "the group should be persisted under the new OU")
}

// ---------------------------------------------------------------------------
// Malformed and invalid request bodies
// ---------------------------------------------------------------------------

// TestCreateGroupWithMalformedJSON verifies an unparsable create body is reported as a bad request.
func (suite *GroupAPITestSuite) TestCreateGroupWithMalformedJSON() {
	resp := suite.doGroupRequest(http.MethodPost, "/groups", []byte(`{"name": "Broken",`))
	defer resp.Body.Close()

	suite.expectErrorWithJSONBody(resp, http.StatusBadRequest, "GRP-1001")
}

// TestUpdateGroupWithMalformedJSON verifies an unparsable update body is reported as a bad request.
func (suite *GroupAPITestSuite) TestUpdateGroupWithMalformedJSON() {
	resp := suite.doGroupRequest(http.MethodPut, "/groups/"+createdGroupID, []byte(`{"name": "Broken",`))
	defer resp.Body.Close()

	suite.expectErrorWithJSONBody(resp, http.StatusBadRequest, "GRP-1001")
}

// TestUpdateGroupWithMissingName verifies the update body is validated against its declared
// constraints before reaching the service.
func (suite *GroupAPITestSuite) TestUpdateGroupWithMissingName() {
	payload := suite.marshal(map[string]any{"ouId": testOUID})

	resp := suite.doGroupRequest(http.MethodPut, "/groups/"+createdGroupID, payload)
	defer resp.Body.Close()

	suite.expectErrorWithJSONBody(resp, http.StatusBadRequest, "INVALID_INPUT_METADATA")
}

// TestAddMembersWithMalformedJSON verifies an unparsable add-members body is rejected.
func (suite *GroupAPITestSuite) TestAddMembersWithMalformedJSON() {
	resp := suite.doGroupRequest(http.MethodPost,
		"/groups/"+createdGroupID+"/members/add", []byte(`{"members": [`))
	defer resp.Body.Close()

	suite.expectErrorWithJSONBody(resp, http.StatusBadRequest, "GRP-1001")
}

// TestRemoveMembersWithMalformedJSON verifies an unparsable remove-members body is rejected.
func (suite *GroupAPITestSuite) TestRemoveMembersWithMalformedJSON() {
	resp := suite.doGroupRequest(http.MethodPost,
		"/groups/"+createdGroupID+"/members/remove", []byte(`{"members": [`))
	defer resp.Body.Close()

	suite.expectErrorWithJSONBody(resp, http.StatusBadRequest, "GRP-1001")
}

// ---------------------------------------------------------------------------
// Pagination parameters that are not numbers
// ---------------------------------------------------------------------------

// TestListGroupsWithNonNumericPagination verifies non-numeric pagination values are reported
// separately for limit and offset rather than silently falling back to defaults.
func (suite *GroupAPITestSuite) TestListGroupsWithNonNumericPagination() {
	limitResp := suite.doGroupRequest(http.MethodGet, "/groups?limit=abc", nil)
	defer limitResp.Body.Close()

	limitErr := suite.expectErrorWithJSONBody(limitResp, http.StatusBadRequest, "GRP-1011")
	suite.Equal("Invalid limit parameter", limitErr.Message.DefaultValue)

	offsetResp := suite.doGroupRequest(http.MethodGet, "/groups?offset=xyz", nil)
	defer offsetResp.Body.Close()

	offsetErr := suite.expectErrorWithJSONBody(offsetResp, http.StatusBadRequest, "GRP-1012")
	suite.Equal("Invalid offset parameter", offsetErr.Message.DefaultValue)
}

// TestGetGroupMembersWithNonNumericPagination verifies the members route validates pagination too.
func (suite *GroupAPITestSuite) TestGetGroupMembersWithNonNumericPagination() {
	resp := suite.doGroupRequest(http.MethodGet, "/groups/"+createdGroupID+"/members?limit=abc", nil)
	defer resp.Body.Close()

	suite.expectErrorWithJSONBody(resp, http.StatusBadRequest, "GRP-1011")
}

// TestGetGroupsByPathWithNonNumericPagination verifies the path-based route validates pagination
// before resolving the handle path.
func (suite *GroupAPITestSuite) TestGetGroupsByPathWithNonNumericPagination() {
	resp := suite.doGroupRequest(http.MethodGet, "/groups/tree/"+testOU.Handle+"?limit=abc", nil)
	defer resp.Body.Close()

	suite.expectErrorWithJSONBody(resp, http.StatusBadRequest, "GRP-1011")
}

// TestGetGroupsByPathWithOversizedLimit verifies the path-based route enforces the maximum page size.
func (suite *GroupAPITestSuite) TestGetGroupsByPathWithOversizedLimit() {
	resp := suite.doGroupRequest(http.MethodGet, "/groups/tree/"+testOU.Handle+"?limit=101", nil)
	defer resp.Body.Close()

	suite.expectErrorWithJSONBody(resp, http.StatusBadRequest, "GRP-1011")
}

// TestGetGroupWithoutID verifies requesting the group collection with a trailing slash is treated as
// a missing group ID rather than as a listing.
func (suite *GroupAPITestSuite) TestGetGroupWithoutID() {
	resp := suite.doGroupRequest(http.MethodGet, "/groups/", nil)
	defer resp.Body.Close()

	suite.expectErrorWithJSONBody(resp, http.StatusBadRequest, "GRP-1002")
}

// ---------------------------------------------------------------------------
// Handle paths that do not resolve
// ---------------------------------------------------------------------------

// TestGetGroupsByBlankPath verifies a path made only of whitespace is rejected as malformed rather
// than being looked up as an OU handle.
func (suite *GroupAPITestSuite) TestGetGroupsByBlankPath() {
	resp := suite.doGroupRequest(http.MethodGet, "/groups/tree/%20", nil)
	defer resp.Body.Close()

	suite.expectErrorWithJSONBody(resp, http.StatusBadRequest, "GRP-1001")
}

// TestGetGroupsByPathWithBlankSegment verifies a path carrying a blank segment between two handles
// is rejected as malformed. The segment has to sit in the middle: leading and trailing whitespace is
// stripped from the path as a whole before it is split.
func (suite *GroupAPITestSuite) TestGetGroupsByPathWithBlankSegment() {
	resp := suite.doGroupRequest(http.MethodGet, "/groups/tree/"+testOU.Handle+"/%20/child", nil)
	defer resp.Body.Close()

	suite.expectErrorWithJSONBody(resp, http.StatusBadRequest, "GRP-1001")
}
