/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package group

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

var (
	testOU = testutils.OrganizationUnit{
		Handle:      "test-group-ou",
		Name:        "Test Organization Unit for Groups",
		Description: "Organization unit created for group API testing",
		Parent:      nil,
	}

	testUserType = testutils.UserType{
		Name: "group-test-person",
		Schema: map[string]interface{}{
			"email": map[string]interface{}{
				"type": "string",
			},
			"given_name": map[string]interface{}{
				"type": "string",
			},
			"family_name": map[string]interface{}{
				"type": "string",
			},
			"password": map[string]interface{}{
				"type":       "string",
				"credential": true,
			},
		},
	}

	testUser = testutils.User{
		Type: "group-test-person",
		Attributes: json.RawMessage(`{
			"email": "testuser@example.com",
			"given_name": "Test",
			"family_name": "User",
			"password": "TestPassword123!"
		}`),
	}

	testGroup = CreateGroupRequest{
		Name:        "Test Group",
		Description: "Group created for API testing",
		Members:     []Member{}, // Will be populated with created user ID
	}
)

var (
	createdGroupID string
	testOUID       string
	testUserID     string
	entityTypeID   string
)

type GroupAPITestSuite struct {
	suite.Suite
}

func (suite *GroupAPITestSuite) SetupSuite() {
	// Create test organization unit
	ouID, err := testutils.CreateOrganizationUnit(testOU)
	if err != nil {
		suite.T().Fatalf("Failed to create test organization unit during setup: %v", err)
	}
	testOUID = ouID
	testUserType.OUID = testOUID

	// Create test user type
	schemaID, err := testutils.CreateUserType(testUserType)
	if err != nil {
		suite.T().Fatalf("Failed to create test user type during setup: %v", err)
	}
	entityTypeID = schemaID

	// Create test user with the created OU
	testUser := testUser
	testUser.OUID = testOUID
	userID, err := testutils.CreateUser(testUser)
	if err != nil {
		suite.T().Fatalf("Failed to create test user during setup: %v", err)
	}
	testUserID = userID

	// Create test group with the created OU and user
	groupToCreate := testGroup
	groupToCreate.OUID = testOUID
	groupToCreate.Members = []Member{
		{
			Id:   testUserID,
			Type: MemberTypeUser,
		},
	}

	id, err := createGroup(groupToCreate)
	if err != nil {
		suite.T().Fatalf("Failed to create group during setup: %v", err)
	}
	createdGroupID = id
}

func (suite *GroupAPITestSuite) TearDownSuite() {
	// Delete group first
	if createdGroupID != "" {
		err := deleteGroup(createdGroupID)
		if err != nil {
			suite.T().Logf("Failed to delete group during teardown: %v", err)
		}
	}

	// Delete test user
	if testUserID != "" {
		err := testutils.DeleteUser(testUserID)
		if err != nil {
			suite.T().Logf("Failed to delete test user during teardown: %v", err)
		}
	}

	// Delete test user type
	if entityTypeID != "" {
		err := testutils.DeleteUserType(entityTypeID)
		if err != nil {
			suite.T().Logf("Failed to delete user type during teardown: %v", err)
		}
	}

	// Delete test organization unit
	if testOUID != "" {
		err := testutils.DeleteOrganizationUnit(testOUID)
		if err != nil {
			suite.T().Logf("Failed to delete test organization unit during teardown: %v", err)
		}
	}
}

func (suite *GroupAPITestSuite) TestGetGroup() {
	if createdGroupID == "" {
		suite.T().Fatal("Group ID is not available for retrieval")
	}

	// Get the created group
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/groups/"+createdGroupID, nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			suite.T().Logf("Failed to close response body: %v", err)
		}
	}()

	suite.Equal(http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		suite.T().Fatalf("Failed to read response body: %v", err)
	}

	var retrievedGroup Group
	err = json.Unmarshal(body, &retrievedGroup)
	suite.Require().NoError(err)

	// Verify the retrieved group
	createdGroup := buildCreatedGroup()
	suite.Equal(createdGroup.Id, retrievedGroup.Id)
	suite.Equal(createdGroup.Name, retrievedGroup.Name)
	suite.Equal(createdGroup.OUID, retrievedGroup.OUID)
}

func (suite *GroupAPITestSuite) TestListGroups() {
	if createdGroupID == "" {
		suite.T().Fatal("Group ID is not available, group creation failed in setup")
	}

	// List groups
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/groups", nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		suite.T().Fatalf("Failed to read response body: %v", err)
	}

	var groupListResponse GroupListResponse
	err = json.Unmarshal(body, &groupListResponse)
	suite.Require().NoError(err)

	// Verify response structure
	suite.GreaterOrEqual(groupListResponse.TotalResults, 1, "Should have at least one group")
	suite.Equal(1, groupListResponse.StartIndex, "StartIndex should be 1 for non-paginated request")
	suite.Equal(groupListResponse.TotalResults, groupListResponse.Count, "Count should equal TotalResults for non-paginated request")
	suite.Equal(len(groupListResponse.Groups), groupListResponse.Count, "Groups array length should match Count")
	suite.Equal(0, len(groupListResponse.Links), "Links should be empty for non-paginated request")

	// Verify the list contains our created group
	found := false
	createdGroup := buildCreatedGroup()
	for _, group := range groupListResponse.Groups {
		if group.Id == createdGroup.Id {
			found = true
			suite.Equal(createdGroup.Name, group.Name)
			suite.Equal(createdGroup.OUID, group.OUID)
			break
		}
	}
	suite.True(found, "Created group should be in the list")
}

func (suite *GroupAPITestSuite) TestListGroupsWithPagination() {
	if createdGroupID == "" {
		suite.T().Fatal("Group ID is not available, group creation failed in setup")
	}

	// Test pagination with limit=1, offset=0
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/groups?limit=1&offset=0", nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var groupListResponse GroupListResponse
	err = json.Unmarshal(body, &groupListResponse)
	suite.Require().NoError(err)

	// Verify pagination structure
	suite.GreaterOrEqual(groupListResponse.TotalResults, 1, "Should have at least one group")
	suite.Equal(1, groupListResponse.StartIndex, "StartIndex should be 1 for offset=0")
	suite.LessOrEqual(groupListResponse.Count, 1, "Count should be at most 1 due to limit=1")
	suite.LessOrEqual(len(groupListResponse.Groups), 1, "Should return at most 1 group")

	// Verify links structure when there might be more pages
	if groupListResponse.TotalResults > 1 {
		suite.NotEmpty(groupListResponse.Links, "Should have pagination links when there are more results")

		// Check for next link
		hasNext := false
		for _, link := range groupListResponse.Links {
			if link.Rel == "next" {
				hasNext = true
				suite.Contains(link.Href, "offset=1", "Next link should have offset=1")
				suite.Contains(link.Href, "limit=1", "Next link should have limit=1")
			}
		}
		suite.True(hasNext, "Should have next link when there are more results")
	}
}

func (suite *GroupAPITestSuite) TestListGroupsWithInvalidPagination() {
	// Test with invalid limit parameter
	client := testutils.GetHTTPClient()

	// Test invalid limit (negative)
	req, err := http.NewRequest("GET", testServerURL+"/groups?limit=-1", nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusBadRequest, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var errorResp map[string]interface{}
	err = json.Unmarshal(body, &errorResp)
	suite.Require().NoError(err)

	suite.Equal("GRP-1011", errorResp["code"])
	suite.Equal("Invalid limit parameter", errorResp["message"].(map[string]interface{})["defaultValue"])

	// Test invalid offset (negative)
	req, err = http.NewRequest("GET", testServerURL+"/groups?offset=-1", nil)
	suite.Require().NoError(err)

	resp, err = client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusBadRequest, resp.StatusCode)

	body, err = io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	err = json.Unmarshal(body, &errorResp)
	suite.Require().NoError(err)

	suite.Equal("GRP-1012", errorResp["code"])
	suite.Equal("Invalid offset parameter", errorResp["message"].(map[string]interface{})["defaultValue"])

	// Test invalid limit (too large)
	req, err = http.NewRequest("GET", testServerURL+"/groups?limit=101", nil)
	suite.Require().NoError(err)

	resp, err = client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusBadRequest, resp.StatusCode)

	body, err = io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	err = json.Unmarshal(body, &errorResp)
	suite.Require().NoError(err)

	suite.Equal("GRP-1011", errorResp["code"])
	suite.Equal("Invalid limit parameter", errorResp["message"].(map[string]interface{})["defaultValue"])
}

func (suite *GroupAPITestSuite) TestListGroupsWithOnlyOffset() {
	// Test with only offset parameter provided (should use default limit=30)
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/groups?offset=0", nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var groupListResponse GroupListResponse
	err = json.Unmarshal(body, &groupListResponse)
	suite.Require().NoError(err)

	// Verify that pagination structure is present (should use default limit=30)
	suite.GreaterOrEqual(groupListResponse.TotalResults, 1, "Should have at least one group")
	suite.Equal(1, groupListResponse.StartIndex, "StartIndex should be 1 for offset=0")
	suite.LessOrEqual(groupListResponse.Count, 30, "Count should be at most 30 due to default limit")
}

func (suite *GroupAPITestSuite) TestUpdateGroup() {
	if createdGroupID == "" {
		suite.T().Fatal("Group ID is not available for update")
	}

	// Update the group
	updateRequest := UpdateGroupRequest{
		Name: "Updated Test Group",
		OUID: testOUID,
	}

	jsonData, err := json.Marshal(updateRequest)
	suite.Require().NoError(err)

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("PUT", testServerURL+"/groups/"+createdGroupID, bytes.NewBuffer(jsonData))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var updatedGroup Group
	err = json.Unmarshal(body, &updatedGroup)
	suite.Require().NoError(err)

	// Verify the update
	suite.Equal(createdGroupID, updatedGroup.Id)
	suite.Equal("Updated Test Group", updatedGroup.Name)
}

func (suite *GroupAPITestSuite) TestUpdateGroupPreservesMembers() {
	// Create a group with a member
	groupWithMember := CreateGroupRequest{
		Name: "Group for Preserve Members Test",
		OUID: testOUID,
		Members: []Member{
			{
				Id:   testUserID,
				Type: MemberTypeUser,
			},
		},
	}

	tempGroupID, err := createGroup(groupWithMember)
	suite.Require().NoError(err)
	defer func() {
		if deleteErr := deleteGroup(tempGroupID); deleteErr != nil {
			suite.T().Logf("Failed to clean up temp group: %v", deleteErr)
		}
	}()

	// Update only the group name via PUT (no members field)
	updateRequest := UpdateGroupRequest{
		Name: "Renamed Group",
		OUID: testOUID,
	}

	jsonData, err := json.Marshal(updateRequest)
	suite.Require().NoError(err)

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("PUT", testServerURL+"/groups/"+tempGroupID, bytes.NewBuffer(jsonData))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	// Verify the name was updated
	var updatedGroup Group
	err = json.NewDecoder(resp.Body).Decode(&updatedGroup)
	suite.Require().NoError(err)
	suite.Equal("Renamed Group", updatedGroup.Name)

	// Verify that the member is still present
	getReq, err := http.NewRequest("GET", testServerURL+"/groups/"+tempGroupID+"/members", nil)
	suite.Require().NoError(err)

	getResp, err := client.Do(getReq)
	suite.Require().NoError(err)
	defer getResp.Body.Close()

	suite.Equal(http.StatusOK, getResp.StatusCode)

	body, err := io.ReadAll(getResp.Body)
	suite.Require().NoError(err)

	var memberListResponse MemberListResponse
	err = json.Unmarshal(body, &memberListResponse)
	suite.Require().NoError(err)

	suite.Equal(1, memberListResponse.TotalResults, "Member should still be present after group update")

	found := false
	for _, member := range memberListResponse.Members {
		if member.Id == testUserID && member.Type == MemberTypeUser {
			found = true
			break
		}
	}
	suite.True(found, "Original member should be preserved after updating group metadata")
}

func (suite *GroupAPITestSuite) TestDeleteGroup() {
	// Create a temporary group for this test since we don't want to delete the main test group
	tempGroupToCreate := CreateGroupRequest{
		Name:    "Temp Test Group",
		OUID:    testOUID,
		Members: []Member{},
	}

	jsonData, err := json.Marshal(tempGroupToCreate)
	suite.Require().NoError(err)

	client := testutils.GetHTTPClient()

	// Create temporary group
	req, err := http.NewRequest("POST", testServerURL+"/groups", bytes.NewBuffer(jsonData))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusCreated, resp.StatusCode)

	var tempGroup Group
	err = json.NewDecoder(resp.Body).Decode(&tempGroup)
	suite.Require().NoError(err)

	// Delete the temporary group
	req, err = http.NewRequest("DELETE", testServerURL+"/groups/"+tempGroup.Id, nil)
	suite.Require().NoError(err)

	resp, err = client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusNoContent, resp.StatusCode)

	// Verify the group is deleted by trying to get it
	getReq, err := http.NewRequest("GET", testServerURL+"/groups/"+tempGroup.Id, nil)
	suite.Require().NoError(err)

	getResp, err := client.Do(getReq)
	if err != nil {
		suite.T().Fatalf("Failed to execute GET request: %v", err)
	}
	defer getResp.Body.Close()

	suite.Equal(http.StatusNotFound, getResp.StatusCode)
}

func (suite *GroupAPITestSuite) TestGetNonExistentGroup() {
	// Try to get a non-existent group
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/groups/non-existent-id", nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	if err != nil {
		suite.T().Fatalf("Failed to execute GET request: %v", err)
	}
	defer resp.Body.Close()

	suite.Equal(http.StatusNotFound, resp.StatusCode)
}

func (suite *GroupAPITestSuite) TestCreateGroupWithInvalidData() {
	// Try to create a group with invalid data (missing name)
	invalidGroup := map[string]interface{}{
		"ouId": testOUID,
	}

	jsonData, err := json.Marshal(invalidGroup)
	suite.Require().NoError(err)

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("POST", testServerURL+"/groups", bytes.NewBuffer(jsonData))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		suite.T().Fatalf("Failed to execute POST request: %v", err)
	}
	defer resp.Body.Close()

	suite.Equal(http.StatusBadRequest, resp.StatusCode)
}

func (suite *GroupAPITestSuite) TestCreateGroupWithInvalidUserID() {
	// Try to create a group with an invalid user ID
	invalidGroup := CreateGroupRequest{
		Name: "Group with Invalid User",
		OUID: testOUID,
		Members: []Member{
			{
				Id:   "invalid-user-id-12345",
				Type: MemberTypeUser,
			},
		},
	}

	jsonData, err := json.Marshal(invalidGroup)
	suite.Require().NoError(err)

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("POST", testServerURL+"/groups", bytes.NewBuffer(jsonData))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusBadRequest, resp.StatusCode)

	// Verify the error response
	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var errorResp map[string]interface{}
	err = json.Unmarshal(body, &errorResp)
	suite.Require().NoError(err)

	suite.Equal("GRP-1007", errorResp["code"])
	suite.Equal("Invalid member ID", errorResp["message"].(map[string]interface{})["defaultValue"])
	suite.Contains(errorResp["description"].(map[string]interface{})["defaultValue"], "One or more user or app member IDs in the request do not exist")
}

func (suite *GroupAPITestSuite) TestCreateGroupWithMixedValidInvalidUserIDs() {
	// Try to create a group with a mix of valid and invalid user IDs
	invalidGroup := CreateGroupRequest{
		Name: "Group with Mixed User IDs",
		OUID: testOUID,
		Members: []Member{
			{
				Id:   testUserID, // Use created test user
				Type: MemberTypeUser,
			},
			{
				Id:   "invalid-user-id-12345", // This is invalid
				Type: MemberTypeUser,
			},
			{
				Id:   "another-invalid-user-67890", // This is also invalid
				Type: MemberTypeUser,
			},
		},
	}

	jsonData, err := json.Marshal(invalidGroup)
	suite.Require().NoError(err)

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("POST", testServerURL+"/groups", bytes.NewBuffer(jsonData))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusBadRequest, resp.StatusCode)

	// Verify the error response
	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var errorResp map[string]interface{}
	err = json.Unmarshal(body, &errorResp)
	suite.Require().NoError(err)

	suite.Equal("GRP-1007", errorResp["code"])
	suite.Equal("Invalid member ID", errorResp["message"].(map[string]interface{})["defaultValue"])
}

func (suite *GroupAPITestSuite) TestCreateGroupWithEmptyUserList() {
	// Create a group with empty user list (should succeed)
	validGroup := CreateGroupRequest{
		Name:    "Group with Empty Users",
		OUID:    testOUID,
		Members: []Member{}, // Empty members list
	}

	jsonData, err := json.Marshal(validGroup)
	suite.Require().NoError(err)

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("POST", testServerURL+"/groups", bytes.NewBuffer(jsonData))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusCreated, resp.StatusCode)

	// Clean up: get the created group ID and delete it
	var createdGroup Group
	err = json.NewDecoder(resp.Body).Decode(&createdGroup)
	suite.Require().NoError(err)

	// Delete the temporary group
	deleteErr := deleteGroup(createdGroup.Id)
	suite.Require().NoError(deleteErr)
}

func (suite *GroupAPITestSuite) TestCreateGroupWithMultipleMembers() {
	// Create a temporary user for testing
	tempUser := testutils.User{
		OUID: testOUID,
		Type: "group-test-person",
		Attributes: json.RawMessage(`{
			"email": "testuser2@example.com",
			"given_name": "Test",
			"family_name": "User2",
			"password": "TestPassword123!"
		}`),
	}
	tempUserID, err := testutils.CreateUser(tempUser)
	if err != nil {
		suite.T().Fatalf("Failed to create test user: %v", err)
	}
	defer func() {
		if deleteErr := testutils.DeleteUser(tempUserID); deleteErr != nil {
			suite.T().Logf("Failed to clean up test user: %v", deleteErr)
		}
	}()

	// Create a group with multiple members (use both the main test user and the temporarily created user)
	groupWithMembers := CreateGroupRequest{
		Name: "Group with Multiple Members",
		OUID: testOUID,
		Members: []Member{
			{
				Id:   testUserID, // Main test user from SetupSuite
				Type: MemberTypeUser,
			},
			{
				Id:   tempUserID, // Temporary test user
				Type: MemberTypeUser,
			},
		},
	}

	jsonData, err := json.Marshal(groupWithMembers)
	suite.Require().NoError(err)

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("POST", testServerURL+"/groups", bytes.NewBuffer(jsonData))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusCreated, resp.StatusCode)

	var createdGroup Group
	err = json.NewDecoder(resp.Body).Decode(&createdGroup)
	suite.Require().NoError(err)

	// Verify the created group has the correct members
	suite.Equal(2, len(createdGroup.Members))

	// Verify member types and IDs
	memberIDs := make(map[string]MemberType)
	for _, member := range createdGroup.Members {
		memberIDs[member.Id] = member.Type
	}

	suite.Equal(MemberTypeUser, memberIDs[testUserID])
	suite.Equal(MemberTypeUser, memberIDs[tempUserID])

	// Clean up: delete the created group
	deleteErr := deleteGroup(createdGroup.Id)
	suite.Require().NoError(deleteErr)
}

func (suite *GroupAPITestSuite) TestCreateGroupWithGroupMember() {
	// First create a temporary group that will be used as a member
	tempGroup := CreateGroupRequest{
		Name:    "Temp Member Group",
		OUID:    testOUID,
		Members: []Member{},
	}

	jsonData, err := json.Marshal(tempGroup)
	suite.Require().NoError(err)

	client := testutils.GetHTTPClient()

	// Create the member group
	req, err := http.NewRequest("POST", testServerURL+"/groups", bytes.NewBuffer(jsonData))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusCreated, resp.StatusCode)

	var memberGroup Group
	err = json.NewDecoder(resp.Body).Decode(&memberGroup)
	suite.Require().NoError(err)

	// Clean up member group later
	defer func() {
		if deleteErr := deleteGroup(memberGroup.Id); deleteErr != nil {
			suite.T().Logf("Failed to clean up member group: %v", deleteErr)
		}
	}()

	// Now create a parent group that includes the first group as a member
	parentGroup := CreateGroupRequest{
		Name: "Parent Group with Group Member",
		OUID: testOUID,
		Members: []Member{
			{
				Id:   memberGroup.Id,
				Type: MemberTypeGroup,
			},
		},
	}

	jsonData, err = json.Marshal(parentGroup)
	suite.Require().NoError(err)

	req, err = http.NewRequest("POST", testServerURL+"/groups", bytes.NewBuffer(jsonData))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err = client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusCreated, resp.StatusCode)

	var createdParentGroup Group
	err = json.NewDecoder(resp.Body).Decode(&createdParentGroup)
	suite.Require().NoError(err)

	// Verify the parent group has the correct member
	suite.Equal(1, len(createdParentGroup.Members))
	suite.Equal(memberGroup.Id, createdParentGroup.Members[0].Id)
	suite.Equal(MemberTypeGroup, createdParentGroup.Members[0].Type)

	// Clean up: delete the parent group
	deleteErr := deleteGroup(createdParentGroup.Id)
	suite.Require().NoError(deleteErr)
}

func (suite *GroupAPITestSuite) TestCreateGroupWithInvalidGroupMember() {
	// Try to create a group with an invalid group member ID
	invalidGroup := CreateGroupRequest{
		Name: "Group with Invalid Group Member",
		OUID: testOUID,
		Members: []Member{
			{
				Id:   "invalid-group-id-12345",
				Type: MemberTypeGroup,
			},
		},
	}

	jsonData, err := json.Marshal(invalidGroup)
	suite.Require().NoError(err)

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("POST", testServerURL+"/groups", bytes.NewBuffer(jsonData))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusBadRequest, resp.StatusCode)

	// Verify the error response
	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var errorResp map[string]interface{}
	err = json.Unmarshal(body, &errorResp)
	suite.Require().NoError(err)

	// The error code might be different for invalid group IDs
	suite.Equal("GRP-1008", errorResp["code"])
	suite.Equal("Invalid group member ID", errorResp["message"].(map[string]interface{})["defaultValue"])
}

func createGroup(group CreateGroupRequest) (string, error) {
	jsonData, err := json.Marshal(group)
	if err != nil {
		return "", fmt.Errorf("failed to marshal group request: %w", err)
	}

	req, err := http.NewRequest("POST", testServerURL+"/groups", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("expected status 201, got %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var createdGroup Group
	err = json.NewDecoder(resp.Body).Decode(&createdGroup)
	if err != nil {
		return "", fmt.Errorf("failed to parse response body: %w", err)
	}

	return createdGroup.Id, nil
}

func deleteGroup(groupID string) error {
	req, err := http.NewRequest("DELETE", testServerURL+"/groups/"+groupID, nil)
	if err != nil {
		return err
	}

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("expected status 204, got %d", resp.StatusCode)
	}
	return nil
}

func buildCreatedGroup() Group {
	return Group{
		GroupBasic: GroupBasic{
			Id:   createdGroupID,
			Name: testGroup.Name,
			OUID: testOUID,
		},
		Members: []Member{
			{
				Id:   testUserID,
				Type: MemberTypeUser,
			},
		},
	}
}

func TestGroupAPITestSuite(t *testing.T) {
	suite.Run(t, new(GroupAPITestSuite))
}

func (suite *GroupAPITestSuite) TestAddGroupMembers() {
	if createdGroupID == "" {
		suite.T().Fatal("Group ID is not available for add members test")
	}

	// Create a temporary user to add as a member
	tempUser := testutils.User{
		OUID: testOUID,
		Type: "group-test-person",
		Attributes: json.RawMessage(`{
			"email": "addmember@example.com",
			"given_name": "Add",
			"family_name": "Member",
			"password": "TestPassword123!"
		}`),
	}
	tempUserID, err := testutils.CreateUser(tempUser)
	if err != nil {
		suite.T().Fatalf("Failed to create temp user: %v", err)
	}
	defer func() {
		if deleteErr := testutils.DeleteUser(tempUserID); deleteErr != nil {
			suite.T().Logf("Failed to clean up temp user: %v", deleteErr)
		}
	}()

	// Add the user as a member
	addRequest := MembersRequest{
		Members: []Member{
			{
				Id:   tempUserID,
				Type: MemberTypeUser,
			},
		},
	}

	jsonData, err := json.Marshal(addRequest)
	suite.Require().NoError(err)

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("POST", testServerURL+"/groups/"+createdGroupID+"/members/add", bytes.NewBuffer(jsonData))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	// Verify the response contains the updated group
	addRespBody, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)
	var addedGroup Group
	err = json.Unmarshal(addRespBody, &addedGroup)
	suite.Require().NoError(err)
	suite.Equal(createdGroupID, addedGroup.Id)

	// Verify the member was added by getting group members
	getReq, err := http.NewRequest("GET", testServerURL+"/groups/"+createdGroupID+"/members", nil)
	suite.Require().NoError(err)

	getResp, err := client.Do(getReq)
	suite.Require().NoError(err)
	defer getResp.Body.Close()

	suite.Equal(http.StatusOK, getResp.StatusCode)

	body, err := io.ReadAll(getResp.Body)
	suite.Require().NoError(err)

	var memberListResponse MemberListResponse
	err = json.Unmarshal(body, &memberListResponse)
	suite.Require().NoError(err)

	found := false
	for _, member := range memberListResponse.Members {
		if member.Id == tempUserID && member.Type == MemberTypeUser {
			found = true
			break
		}
	}
	suite.True(found, "Added member should be in the members list")

	// Clean up: remove the added member
	removeRequest := MembersRequest{
		Members: []Member{
			{
				Id:   tempUserID,
				Type: MemberTypeUser,
			},
		},
	}

	jsonData, err = json.Marshal(removeRequest)
	suite.Require().NoError(err)

	removeReq, err := http.NewRequest("POST", testServerURL+"/groups/"+createdGroupID+"/members/remove", bytes.NewBuffer(jsonData))
	suite.Require().NoError(err)
	removeReq.Header.Set("Content-Type", "application/json")

	removeResp, err := client.Do(removeReq)
	suite.Require().NoError(err)
	defer removeResp.Body.Close()

	suite.Equal(http.StatusOK, removeResp.StatusCode)
}

func (suite *GroupAPITestSuite) TestAddGroupMembersWithGroupMember() {
	if createdGroupID == "" {
		suite.T().Fatal("Group ID is not available for add group member test")
	}

	// Create a temporary group to add as a member
	tempGroupReq := CreateGroupRequest{
		Name:    "Temp Group for Add Member Test",
		OUID:    testOUID,
		Members: []Member{},
	}

	tempGroupID, err := createGroup(tempGroupReq)
	suite.Require().NoError(err)
	defer func() {
		if deleteErr := deleteGroup(tempGroupID); deleteErr != nil {
			suite.T().Logf("Failed to clean up temp group: %v", deleteErr)
		}
	}()

	// Add the group as a member
	addRequest := MembersRequest{
		Members: []Member{
			{
				Id:   tempGroupID,
				Type: MemberTypeGroup,
			},
		},
	}

	jsonData, err := json.Marshal(addRequest)
	suite.Require().NoError(err)

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("POST", testServerURL+"/groups/"+createdGroupID+"/members/add", bytes.NewBuffer(jsonData))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	// Verify the group member was added
	getReq, err := http.NewRequest("GET", testServerURL+"/groups/"+createdGroupID+"/members", nil)
	suite.Require().NoError(err)

	getResp, err := client.Do(getReq)
	suite.Require().NoError(err)
	defer getResp.Body.Close()

	suite.Equal(http.StatusOK, getResp.StatusCode)

	body, err := io.ReadAll(getResp.Body)
	suite.Require().NoError(err)

	var memberListResponse MemberListResponse
	err = json.Unmarshal(body, &memberListResponse)
	suite.Require().NoError(err)

	found := false
	for _, member := range memberListResponse.Members {
		if member.Id == tempGroupID && member.Type == MemberTypeGroup {
			found = true
			break
		}
	}
	suite.True(found, "Added group member should be in the members list")

	// Clean up: remove the added group member
	removeRequest := MembersRequest{
		Members: []Member{
			{
				Id:   tempGroupID,
				Type: MemberTypeGroup,
			},
		},
	}

	jsonData, err = json.Marshal(removeRequest)
	suite.Require().NoError(err)

	removeReq, err := http.NewRequest("POST", testServerURL+"/groups/"+createdGroupID+"/members/remove", bytes.NewBuffer(jsonData))
	suite.Require().NoError(err)
	removeReq.Header.Set("Content-Type", "application/json")

	removeResp, err := client.Do(removeReq)
	suite.Require().NoError(err)
	defer removeResp.Body.Close()

	suite.Equal(http.StatusOK, removeResp.StatusCode)
}

func (suite *GroupAPITestSuite) TestAddGroupMembersWithEmptyList() {
	if createdGroupID == "" {
		suite.T().Fatal("Group ID is not available for empty members test")
	}

	// Try to add empty members list
	addRequest := MembersRequest{
		Members: []Member{},
	}

	jsonData, err := json.Marshal(addRequest)
	suite.Require().NoError(err)

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("POST", testServerURL+"/groups/"+createdGroupID+"/members/add", bytes.NewBuffer(jsonData))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusBadRequest, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var errorResp map[string]interface{}
	err = json.Unmarshal(body, &errorResp)
	suite.Require().NoError(err)

	suite.Equal("GRP-1013", errorResp["code"])
	suite.Equal("Empty members list", errorResp["message"].(map[string]interface{})["defaultValue"])
}

func (suite *GroupAPITestSuite) TestAddGroupMembersWithInvalidUserID() {
	if createdGroupID == "" {
		suite.T().Fatal("Group ID is not available for invalid user test")
	}

	addRequest := MembersRequest{
		Members: []Member{
			{
				Id:   "invalid-user-id-for-add",
				Type: MemberTypeUser,
			},
		},
	}

	jsonData, err := json.Marshal(addRequest)
	suite.Require().NoError(err)

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("POST", testServerURL+"/groups/"+createdGroupID+"/members/add", bytes.NewBuffer(jsonData))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusBadRequest, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var errorResp map[string]interface{}
	err = json.Unmarshal(body, &errorResp)
	suite.Require().NoError(err)

	suite.Equal("GRP-1007", errorResp["code"])
	suite.Equal("Invalid member ID", errorResp["message"].(map[string]interface{})["defaultValue"])
}

func (suite *GroupAPITestSuite) TestAddGroupMembersWithInvalidGroupID() {
	if createdGroupID == "" {
		suite.T().Fatal("Group ID is not available for invalid group member test")
	}

	addRequest := MembersRequest{
		Members: []Member{
			{
				Id:   "invalid-group-id-for-add",
				Type: MemberTypeGroup,
			},
		},
	}

	jsonData, err := json.Marshal(addRequest)
	suite.Require().NoError(err)

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("POST", testServerURL+"/groups/"+createdGroupID+"/members/add", bytes.NewBuffer(jsonData))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusBadRequest, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var errorResp map[string]interface{}
	err = json.Unmarshal(body, &errorResp)
	suite.Require().NoError(err)

	suite.Equal("GRP-1008", errorResp["code"])
	suite.Equal("Invalid group member ID", errorResp["message"].(map[string]interface{})["defaultValue"])
}

func (suite *GroupAPITestSuite) TestAddGroupMembersToNonExistentGroup() {
	addRequest := MembersRequest{
		Members: []Member{
			{
				Id:   testUserID,
				Type: MemberTypeUser,
			},
		},
	}

	jsonData, err := json.Marshal(addRequest)
	suite.Require().NoError(err)

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("POST", testServerURL+"/groups/non-existent-id/members/add", bytes.NewBuffer(jsonData))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusNotFound, resp.StatusCode)
}

func (suite *GroupAPITestSuite) TestRemoveGroupMembers() {
	if createdGroupID == "" {
		suite.T().Fatal("Group ID is not available for remove members test")
	}

	// Create a temporary user and add it to the group first
	tempUser := testutils.User{
		OUID: testOUID,
		Type: "group-test-person",
		Attributes: json.RawMessage(`{
			"email": "removemember@example.com",
			"given_name": "Remove",
			"family_name": "Member",
			"password": "TestPassword123!"
		}`),
	}
	tempUserID, err := testutils.CreateUser(tempUser)
	if err != nil {
		suite.T().Fatalf("Failed to create temp user: %v", err)
	}
	defer func() {
		if deleteErr := testutils.DeleteUser(tempUserID); deleteErr != nil {
			suite.T().Logf("Failed to clean up temp user: %v", deleteErr)
		}
	}()

	// Add the user first
	addRequest := MembersRequest{
		Members: []Member{
			{
				Id:   tempUserID,
				Type: MemberTypeUser,
			},
		},
	}

	jsonData, err := json.Marshal(addRequest)
	suite.Require().NoError(err)

	client := testutils.GetHTTPClient()

	addReq, err := http.NewRequest("POST", testServerURL+"/groups/"+createdGroupID+"/members/add", bytes.NewBuffer(jsonData))
	suite.Require().NoError(err)
	addReq.Header.Set("Content-Type", "application/json")

	addResp, err := client.Do(addReq)
	suite.Require().NoError(err)
	defer addResp.Body.Close()
	suite.Equal(http.StatusOK, addResp.StatusCode)

	// Now remove the user
	removeRequest := MembersRequest{
		Members: []Member{
			{
				Id:   tempUserID,
				Type: MemberTypeUser,
			},
		},
	}

	jsonData, err = json.Marshal(removeRequest)
	suite.Require().NoError(err)

	removeReq, err := http.NewRequest("POST", testServerURL+"/groups/"+createdGroupID+"/members/remove", bytes.NewBuffer(jsonData))
	suite.Require().NoError(err)
	removeReq.Header.Set("Content-Type", "application/json")

	removeResp, err := client.Do(removeReq)
	suite.Require().NoError(err)
	defer removeResp.Body.Close()

	suite.Equal(http.StatusOK, removeResp.StatusCode)

	// Verify the member was removed
	getReq, err := http.NewRequest("GET", testServerURL+"/groups/"+createdGroupID+"/members", nil)
	suite.Require().NoError(err)

	getResp, err := client.Do(getReq)
	suite.Require().NoError(err)
	defer getResp.Body.Close()

	suite.Equal(http.StatusOK, getResp.StatusCode)

	body, err := io.ReadAll(getResp.Body)
	suite.Require().NoError(err)

	var memberListResponse MemberListResponse
	err = json.Unmarshal(body, &memberListResponse)
	suite.Require().NoError(err)

	for _, member := range memberListResponse.Members {
		suite.NotEqual(tempUserID, member.Id, "Removed member should not be in the members list")
	}
}

func (suite *GroupAPITestSuite) TestRemoveGroupMembersWithEmptyList() {
	if createdGroupID == "" {
		suite.T().Fatal("Group ID is not available for empty remove test")
	}

	removeRequest := MembersRequest{
		Members: []Member{},
	}

	jsonData, err := json.Marshal(removeRequest)
	suite.Require().NoError(err)

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("POST", testServerURL+"/groups/"+createdGroupID+"/members/remove", bytes.NewBuffer(jsonData))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusBadRequest, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var errorResp map[string]interface{}
	err = json.Unmarshal(body, &errorResp)
	suite.Require().NoError(err)

	suite.Equal("GRP-1013", errorResp["code"])
	suite.Equal("Empty members list", errorResp["message"].(map[string]interface{})["defaultValue"])
}

func (suite *GroupAPITestSuite) TestRemoveGroupMembersFromNonExistentGroup() {
	removeRequest := MembersRequest{
		Members: []Member{
			{
				Id:   testUserID,
				Type: MemberTypeUser,
			},
		},
	}

	jsonData, err := json.Marshal(removeRequest)
	suite.Require().NoError(err)

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("POST", testServerURL+"/groups/non-existent-id/members/remove", bytes.NewBuffer(jsonData))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusNotFound, resp.StatusCode)
}

func (suite *GroupAPITestSuite) TestAddAndRemoveMultipleMembers() {
	if createdGroupID == "" {
		suite.T().Fatal("Group ID is not available for multiple members test")
	}

	// Create two temporary users
	tempUser1 := testutils.User{
		OUID: testOUID,
		Type: "group-test-person",
		Attributes: json.RawMessage(`{
			"email": "multi1@example.com",
			"given_name": "Multi",
			"family_name": "User1",
			"password": "TestPassword123!"
		}`),
	}
	tempUser1ID, err := testutils.CreateUser(tempUser1)
	suite.Require().NoError(err)
	defer func() {
		if deleteErr := testutils.DeleteUser(tempUser1ID); deleteErr != nil {
			suite.T().Logf("Failed to clean up temp user 1: %v", deleteErr)
		}
	}()

	tempUser2 := testutils.User{
		OUID: testOUID,
		Type: "group-test-person",
		Attributes: json.RawMessage(`{
			"email": "multi2@example.com",
			"given_name": "Multi",
			"family_name": "User2",
			"password": "TestPassword123!"
		}`),
	}
	tempUser2ID, err := testutils.CreateUser(tempUser2)
	suite.Require().NoError(err)
	defer func() {
		if deleteErr := testutils.DeleteUser(tempUser2ID); deleteErr != nil {
			suite.T().Logf("Failed to clean up temp user 2: %v", deleteErr)
		}
	}()

	// Add both users at once
	addRequest := MembersRequest{
		Members: []Member{
			{
				Id:   tempUser1ID,
				Type: MemberTypeUser,
			},
			{
				Id:   tempUser2ID,
				Type: MemberTypeUser,
			},
		},
	}

	jsonData, err := json.Marshal(addRequest)
	suite.Require().NoError(err)

	client := testutils.GetHTTPClient()

	addReq, err := http.NewRequest("POST", testServerURL+"/groups/"+createdGroupID+"/members/add", bytes.NewBuffer(jsonData))
	suite.Require().NoError(err)
	addReq.Header.Set("Content-Type", "application/json")

	addResp, err := client.Do(addReq)
	suite.Require().NoError(err)
	defer addResp.Body.Close()

	suite.Equal(http.StatusOK, addResp.StatusCode)

	// Verify both members were added
	getReq, err := http.NewRequest("GET", testServerURL+"/groups/"+createdGroupID+"/members", nil)
	suite.Require().NoError(err)

	getResp, err := client.Do(getReq)
	suite.Require().NoError(err)
	defer getResp.Body.Close()

	body, err := io.ReadAll(getResp.Body)
	suite.Require().NoError(err)

	var memberListResponse MemberListResponse
	err = json.Unmarshal(body, &memberListResponse)
	suite.Require().NoError(err)

	foundUser1 := false
	foundUser2 := false
	for _, member := range memberListResponse.Members {
		if member.Id == tempUser1ID {
			foundUser1 = true
		}
		if member.Id == tempUser2ID {
			foundUser2 = true
		}
	}
	suite.True(foundUser1, "First added member should be in the list")
	suite.True(foundUser2, "Second added member should be in the list")

	// Remove both users at once
	removeRequest := MembersRequest{
		Members: []Member{
			{
				Id:   tempUser1ID,
				Type: MemberTypeUser,
			},
			{
				Id:   tempUser2ID,
				Type: MemberTypeUser,
			},
		},
	}

	jsonData, err = json.Marshal(removeRequest)
	suite.Require().NoError(err)

	removeReq, err := http.NewRequest("POST", testServerURL+"/groups/"+createdGroupID+"/members/remove", bytes.NewBuffer(jsonData))
	suite.Require().NoError(err)
	removeReq.Header.Set("Content-Type", "application/json")

	removeResp, err := client.Do(removeReq)
	suite.Require().NoError(err)
	defer removeResp.Body.Close()

	suite.Equal(http.StatusOK, removeResp.StatusCode)

	// Verify both were removed
	getReq2, err := http.NewRequest("GET", testServerURL+"/groups/"+createdGroupID+"/members", nil)
	suite.Require().NoError(err)

	getResp2, err := client.Do(getReq2)
	suite.Require().NoError(err)
	defer getResp2.Body.Close()

	body2, err := io.ReadAll(getResp2.Body)
	suite.Require().NoError(err)

	var memberListResponse2 MemberListResponse
	err = json.Unmarshal(body2, &memberListResponse2)
	suite.Require().NoError(err)

	for _, member := range memberListResponse2.Members {
		suite.NotEqual(tempUser1ID, member.Id, "First removed member should not be in the list")
		suite.NotEqual(tempUser2ID, member.Id, "Second removed member should not be in the list")
	}
}

func (suite *GroupAPITestSuite) TestAddGroupMembersWithMixedTypes() {
	if createdGroupID == "" {
		suite.T().Fatal("Group ID is not available for mixed types test")
	}

	// Create a temporary user
	tempUser := testutils.User{
		OUID: testOUID,
		Type: "group-test-person",
		Attributes: json.RawMessage(`{
			"email": "mixedtype@example.com",
			"given_name": "Mixed",
			"family_name": "Type",
			"password": "TestPassword123!"
		}`),
	}
	tempUserID, err := testutils.CreateUser(tempUser)
	suite.Require().NoError(err)
	defer func() {
		if deleteErr := testutils.DeleteUser(tempUserID); deleteErr != nil {
			suite.T().Logf("Failed to clean up temp user: %v", deleteErr)
		}
	}()

	// Create a temporary group
	tempGroupReq := CreateGroupRequest{
		Name:    "Temp Group for Mixed Type Test",
		OUID:    testOUID,
		Members: []Member{},
	}
	tempGroupID, err := createGroup(tempGroupReq)
	suite.Require().NoError(err)
	defer func() {
		if deleteErr := deleteGroup(tempGroupID); deleteErr != nil {
			suite.T().Logf("Failed to clean up temp group: %v", deleteErr)
		}
	}()

	// Add both user and group members at once
	addRequest := MembersRequest{
		Members: []Member{
			{
				Id:   tempUserID,
				Type: MemberTypeUser,
			},
			{
				Id:   tempGroupID,
				Type: MemberTypeGroup,
			},
		},
	}

	jsonData, err := json.Marshal(addRequest)
	suite.Require().NoError(err)

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("POST", testServerURL+"/groups/"+createdGroupID+"/members/add", bytes.NewBuffer(jsonData))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	// Verify both members exist
	getReq, err := http.NewRequest("GET", testServerURL+"/groups/"+createdGroupID+"/members", nil)
	suite.Require().NoError(err)

	getResp, err := client.Do(getReq)
	suite.Require().NoError(err)
	defer getResp.Body.Close()

	body, err := io.ReadAll(getResp.Body)
	suite.Require().NoError(err)

	var memberListResponse MemberListResponse
	err = json.Unmarshal(body, &memberListResponse)
	suite.Require().NoError(err)

	foundUser := false
	foundGroup := false
	for _, member := range memberListResponse.Members {
		if member.Id == tempUserID && member.Type == MemberTypeUser {
			foundUser = true
		}
		if member.Id == tempGroupID && member.Type == MemberTypeGroup {
			foundGroup = true
		}
	}
	suite.True(foundUser, "User member should be in the list")
	suite.True(foundGroup, "Group member should be in the list")

	// Clean up: remove both
	removeRequest := MembersRequest{
		Members: []Member{
			{
				Id:   tempUserID,
				Type: MemberTypeUser,
			},
			{
				Id:   tempGroupID,
				Type: MemberTypeGroup,
			},
		},
	}

	jsonData, err = json.Marshal(removeRequest)
	suite.Require().NoError(err)

	removeReq, err := http.NewRequest("POST", testServerURL+"/groups/"+createdGroupID+"/members/remove", bytes.NewBuffer(jsonData))
	suite.Require().NoError(err)
	removeReq.Header.Set("Content-Type", "application/json")

	removeResp, err := client.Do(removeReq)
	suite.Require().NoError(err)
	defer removeResp.Body.Close()

	suite.Equal(http.StatusOK, removeResp.StatusCode)
}

func (suite *GroupAPITestSuite) TestGetGroupMembers() {
	if createdGroupID == "" {
		suite.T().Fatal("Group ID is not available for member retrieval")
	}

	// Get the group members
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/groups/"+createdGroupID+"/members", nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var memberListResponse MemberListResponse
	err = json.Unmarshal(body, &memberListResponse)
	suite.Require().NoError(err)

	// Verify the response structure
	suite.GreaterOrEqual(memberListResponse.TotalResults, 1, "Should have at least one member")
	suite.Equal(1, memberListResponse.StartIndex, "StartIndex should be 1 for non-paginated request")
	suite.Equal(memberListResponse.TotalResults, memberListResponse.Count, "Count should equal TotalResults for non-paginated request")
	suite.Equal(len(memberListResponse.Members), memberListResponse.Count, "Members array length should match Count")

	// Verify we have the expected member
	found := false
	for _, member := range memberListResponse.Members {
		if member.Id == testUserID && member.Type == MemberTypeUser {
			found = true
			break
		}
	}
	suite.True(found, "Expected member should be in the list")
}

func (suite *GroupAPITestSuite) TestGetGroupMembersWithPagination() {
	if createdGroupID == "" {
		suite.T().Fatal("Group ID is not available for member retrieval")
	}

	// Test pagination with limit=1, offset=0
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/groups/"+createdGroupID+"/members?limit=1&offset=0", nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var memberListResponse MemberListResponse
	err = json.Unmarshal(body, &memberListResponse)
	suite.Require().NoError(err)

	// Verify pagination structure
	suite.GreaterOrEqual(memberListResponse.TotalResults, 1, "Should have at least one member")
	suite.Equal(1, memberListResponse.StartIndex, "StartIndex should be 1 for offset=0")
	suite.LessOrEqual(memberListResponse.Count, 1, "Count should be at most 1 due to limit=1")
	suite.LessOrEqual(len(memberListResponse.Members), 1, "Should return at most 1 member")
}

func (suite *GroupAPITestSuite) TestGetGroupMembersNotFound() {
	// Try to get members of a non-existent group
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/groups/non-existent-id/members", nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusNotFound, resp.StatusCode)
}
