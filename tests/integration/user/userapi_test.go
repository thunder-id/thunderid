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

package user

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

const (
	testServerURL = "https://localhost:8095"
)

const (
	groupMemberTypeUser = "user"
)

type groupMember struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

type groupCreateRequest struct {
	Name               string        `json:"name"`
	Description        string        `json:"description,omitempty"`
	OUID               string        `json:"ouId"`
	Members            []groupMember `json:"members,omitempty"`
}

type groupCreateResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

var (
	entityType = testutils.UserType{
		Name: "test-user-person",
		Schema: map[string]interface{}{
			"age": map[string]interface{}{"type": "number"},
			"roles": map[string]interface{}{
				"type":  "array",
				"items": map[string]interface{}{"type": "string"},
			},
			"address": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"city": map[string]interface{}{"type": "string"},
					"zip":  map[string]interface{}{"type": "string"},
				},
			},
		},
	}

	testUser = testutils.User{
		Type:       "test-user-person",
		Attributes: json.RawMessage(`{"age": 25, "roles": ["viewer"], "address": {"city": "Seattle", "zip": "98101"}}`),
	}

	userUpdate = testutils.User{
		Type:       "test-user-person",
		Attributes: json.RawMessage(`{"age": 35, "roles": ["admin"], "address": {"city": "Colombo", "zip": "10300"}}`),
	}

	testOU = OUCreateRequest{
		Handle:      "test-ou-users",
		Name:        "Test Organization Unit for Users",
		Description: "Organization unit created for user API testing",
		Parent:      nil,
	}

	testGroup = groupCreateRequest{
		Name:        "User API Test Group",
		Description: "Group created for validating user groups endpoint",
	}
)

var (
	createdUserID  string
	testOUID       string
	createdGroupID string
	entityTypeID   string
)

type UserAPITestSuite struct {
	suite.Suite
}

func TestUserAPITestSuite(t *testing.T) {

	suite.Run(t, new(UserAPITestSuite))
}

// SetupSuite creates test organization unit and user via API
func (ts *UserAPITestSuite) SetupSuite() {
	// First create the organization unit
	ouID, err := createOrganizationUnit(testOU)
	if err != nil {
		ts.T().Fatalf("Failed to create organization unit during setup: %v", err)
	}
	testOUID = ouID

	entityType.OUID = testOUID
	schemaID, err := testutils.CreateUserType(entityType)
	if err != nil {
		ts.T().Fatalf("Failed to create user type during setup: %v", err)
	}
	entityTypeID = schemaID

	// Update user template with the created OU ID
	testUser := testUser
	testUser.OUID = testOUID

	// Create the test user
	userID, err := createUser(testUser)
	if err != nil {
		ts.T().Fatalf("Failed to create user during setup: %v", err)
	}
	createdUserID = userID

	// Create a group and add the created user as a member
	groupToCreate := testGroup
	groupToCreate.OUID = testOUID
	groupToCreate.Members = []groupMember{
		{
			ID:   createdUserID,
			Type: groupMemberTypeUser,
		},
	}

	groupID, err := createGroup(groupToCreate)
	if err != nil {
		ts.T().Fatalf("Failed to create group during setup: %v", err)
	}
	createdGroupID = groupID
}

// TearDownSuite cleans up test user and organization unit
func (ts *UserAPITestSuite) TearDownSuite() {
	// Delete the test group
	if createdGroupID != "" {
		err := deleteGroup(createdGroupID)
		if err != nil {
			ts.T().Logf("Failed to delete group during teardown: %v", err)
		}
	}

	// Delete the test user first
	if createdUserID != "" {
		err := deleteUser(createdUserID)
		if err != nil {
			ts.T().Logf("Failed to delete user during teardown: %v", err)
		}
	}

	// Delete the test organization unit
	if testOUID != "" {
		err := deleteOrganizationUnit(testOUID)
		if err != nil {
			ts.T().Logf("Failed to delete organization unit during teardown: %v", err)
		}
	}

	if entityTypeID != "" {
		if err := testutils.DeleteUserType(entityTypeID); err != nil {
			ts.T().Logf("Failed to delete user type during teardown: %v", err)
		}
	}
}

// Test user listing
func (ts *UserAPITestSuite) TestUserListing() {

	req, err := http.NewRequest("GET", testServerURL+"/users", nil)
	if err != nil {
		ts.T().Fatalf("Failed to create request: %v", err)
	}

	client := testutils.GetHTTPClient()

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		ts.T().Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Validate the response
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		ts.T().Fatalf("Expected status 200, got %d. Response body: %s", resp.StatusCode, string(body))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		ts.T().Fatalf("Failed to read response body: %v", err)
	}

	var userListResponse testutils.UserListResponse
	err = json.Unmarshal(bodyBytes, &userListResponse)
	if err != nil {
		ts.T().Fatalf("Failed to parse response body: %v. Raw body: %s", err, string(bodyBytes))
	}

	if userListResponse.TotalResults <= 0 {
		ts.T().Fatalf("Expected TotalResults > 0, got %d", userListResponse.TotalResults)
	}

	if userListResponse.StartIndex != 1 {
		ts.T().Fatalf("Expected StartIndex 1, got %d", userListResponse.StartIndex)
	}

	if userListResponse.Count != len(userListResponse.Users) {
		ts.T().Fatalf("Count field (%d) doesn't match actual users length (%d)", userListResponse.Count, len(userListResponse.Users))
	}

	users := userListResponse.Users
	userListLength := len(users)
	if userListLength == 0 {
		ts.T().Fatalf("Response does not contain any users")
	}

	var foundCreatedUser bool
	expectedUser := testutils.User{
		ID:               createdUserID,
		OUID:             testOUID,
		Type:             testUser.Type,
		Attributes:       testUser.Attributes,
	}
	for _, user := range users {
		if Equals(user, expectedUser) {
			foundCreatedUser = true
			break
		}
	}

	if !foundCreatedUser {
		ts.T().Fatalf("Created user not found in user list. Expected %+v", expectedUser)
	}
}

// Test user pagination
func (ts *UserAPITestSuite) TestUserPagination() {
	req, err := http.NewRequest("GET", testServerURL+"/users?limit=1&offset=0", nil)
	if err != nil {
		ts.T().Fatalf("Failed to create request: %v", err)
	}

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		ts.T().Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		ts.T().Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	var userListResponse testutils.UserListResponse
	err = json.NewDecoder(resp.Body).Decode(&userListResponse)
	if err != nil {
		ts.T().Fatalf("Failed to parse response body: %v", err)
	}

	if userListResponse.Count != 1 {
		ts.T().Fatalf("Expected count 1 with limit=1, got %d", userListResponse.Count)
	}

	if len(userListResponse.Users) != 1 {
		ts.T().Fatalf("Expected 1 user with limit=1, got %d", len(userListResponse.Users))
	}

	if userListResponse.StartIndex != 1 {
		ts.T().Fatalf("Expected StartIndex 1 with offset=0, got %d", userListResponse.StartIndex)
	}

	req2, err := http.NewRequest("GET", testServerURL+"/users?limit=1&offset=1", nil)
	if err != nil {
		ts.T().Fatalf("Failed to create request: %v", err)
	}

	resp2, err := client.Do(req2)
	if err != nil {
		ts.T().Fatalf("Failed to send request: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		ts.T().Fatalf("Expected status 200, got %d", resp2.StatusCode)
	}

	var userListResponse2 testutils.UserListResponse
	err = json.NewDecoder(resp2.Body).Decode(&userListResponse2)
	if err != nil {
		ts.T().Fatalf("Failed to parse response body: %v", err)
	}

	if userListResponse2.StartIndex != 2 {
		ts.T().Fatalf("Expected StartIndex 2 with offset=1, got %d", userListResponse2.StartIndex)
	}

	req3, err := http.NewRequest("GET", testServerURL+"/users?limit=invalid", nil)
	if err != nil {
		ts.T().Fatalf("Failed to create request: %v", err)
	}

	resp3, err := client.Do(req3)
	if err != nil {
		ts.T().Fatalf("Failed to send request: %v", err)
	}
	defer resp3.Body.Close()

	if resp3.StatusCode != http.StatusBadRequest {
		ts.T().Fatalf("Expected status 400 for invalid limit, got %d", resp3.StatusCode)
	}
}

// Test user get by ID
func (ts *UserAPITestSuite) TestUserGetByID() {

	if createdUserID == "" {
		ts.T().Fatal("user ID is not available for retrieval")
	}
	expectedUser := testutils.User{
		ID:               createdUserID,
		OUID:             testOUID,
		Type:             testUser.Type,
		Attributes:       testUser.Attributes,
	}
	retrieveAndValidateUserDetails(ts, expectedUser)
}

// Test user update
func (ts *UserAPITestSuite) TestUserUpdate() {

	if createdUserID == "" {
		ts.T().Fatal("User ID is not available for update")
	}

	// Update user template with the created OU ID
	userToUpdate := userUpdate
	userToUpdate.OUID = testOUID

	userJSON, err := json.Marshal(userToUpdate)
	if err != nil {
		ts.T().Fatalf("Failed to marshal userToUpdate: %v", err)
	}

	reqBody := bytes.NewReader(userJSON)
	req, err := http.NewRequest("PUT", testServerURL+"/users/"+createdUserID, reqBody)
	if err != nil {
		ts.T().Fatalf("Failed to create update request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		ts.T().Fatalf("Failed to send update request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		ts.T().Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	// Validate the update by retrieving the user
	retrieveAndValidateUserDetails(ts, testutils.User{
		ID:               createdUserID,
		OUID:             userToUpdate.OUID,
		Type:             userToUpdate.Type,
		Attributes:       userToUpdate.Attributes,
	})
}

// Test user groups listing
func (ts *UserAPITestSuite) TestUserGroupsListing() {

	if createdUserID == "" {
		ts.T().Fatal("user ID is not available for group listing")
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/users/%s/groups", testServerURL, createdUserID), nil)
	if err != nil {
		ts.T().Fatalf("Failed to create user groups request: %v", err)
	}

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		ts.T().Fatalf("Failed to send user groups request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		ts.T().Fatalf("Expected status 200, got %d. Response body: %s", resp.StatusCode, string(body))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		ts.T().Fatalf("Failed to read user groups response: %v", err)
	}

	var groupListResponse UserGroupListResponse
	err = json.Unmarshal(bodyBytes, &groupListResponse)
	if err != nil {
		ts.T().Fatalf("Failed to parse user groups response: %v. Raw body: %s", err, string(bodyBytes))
	}

	if groupListResponse.TotalResults < 1 {
		ts.T().Fatalf("Expected at least one group for the user, got %d", groupListResponse.TotalResults)
	}

	if groupListResponse.StartIndex != 1 {
		ts.T().Fatalf("Expected StartIndex 1, got %d", groupListResponse.StartIndex)
	}

	if groupListResponse.Count != len(groupListResponse.Groups) {
		ts.T().Fatalf("Count field (%d) doesn't match groups length (%d)", groupListResponse.Count, len(groupListResponse.Groups))
	}

	var foundCreatedGroup bool
	for _, group := range groupListResponse.Groups {
		if group.ID == createdGroupID {
			foundCreatedGroup = true
			if group.Name != testGroup.Name {
				ts.T().Fatalf("Expected group name %s, got %s", testGroup.Name, group.Name)
			}
			if group.OUID != "" && group.OUID != testOUID {
				ts.T().Fatalf("Expected group OU %s, got %s", testOUID, group.OUID)
			}
			break
		}
	}

	if !foundCreatedGroup {
		ts.T().Fatalf("Expected to find group %s in user groups list", createdGroupID)
	}
}

// Test user groups listing for non-existing user
func (ts *UserAPITestSuite) TestUserGroupsListingNonExistingUser() {

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/users/%s/groups",
		testServerURL, "00000000-0000-0000-0000-000000000999"), nil)
	if err != nil {
		ts.T().Fatalf("Failed to create request for non-existing user groups: %v", err)
	}

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		ts.T().Fatalf("Failed to send request for non-existing user groups: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		ts.T().Fatalf("Expected status 404 for non-existing user, got %d. Response body: %s", resp.StatusCode, string(body))
	}

	var errorResp testutils.ErrorResponse
	err = json.NewDecoder(resp.Body).Decode(&errorResp)
	if err != nil {
		ts.T().Fatalf("Failed to parse error response: %v", err)
	}

	if errorResp.Code != "USR-1003" {
		ts.T().Fatalf("Expected error code USR-1003, got %s", errorResp.Code)
	}
}

func retrieveAndValidateUserDetails(ts *UserAPITestSuite, expectedUser testutils.User) {

	req, err := http.NewRequest("GET", testServerURL+"/users/"+expectedUser.ID, nil)
	if err != nil {
		ts.T().Fatalf("Failed to create get request: %v", err)
	}

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		ts.T().Fatalf("Failed to send get request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		ts.T().Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	// Check if the response Content-Type is application/json
	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		rawBody, _ := io.ReadAll(resp.Body)
		ts.T().Fatalf("Unexpected Content-Type: %s. Raw body: %s", contentType, string(rawBody))
	}

	var user testutils.User
	err = json.NewDecoder(resp.Body).Decode(&user)
	if err != nil {
		ts.T().Fatalf("Failed to parse response body: %v", err)
	}

	if !Equals(user, expectedUser) {
		ts.T().Fatalf("User mismatch, expected %+v, got %+v", expectedUser, user)
	}
}

func createUser(user testutils.User) (string, error) {

	userJSON, err := json.Marshal(user)
	if err != nil {
		return "", fmt.Errorf("failed to marshal user template: %w", err)
	}

	reqBody := bytes.NewReader(userJSON)
	req, err := http.NewRequest("POST", testServerURL+"/users", reqBody)
	if err != nil {
		// print error
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
		return "", fmt.Errorf("expected status 201, got %d", resp.StatusCode)
	}

	var respBody map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	if err != nil {
		return "", fmt.Errorf("failed to parse response body: %w", err)
	}

	id, ok := respBody["id"].(string)
	if !ok {
		return "", fmt.Errorf("response does not contain id")
	}
	createdUserID = id
	return id, nil
}

func deleteUser(userId string) error {

	req, err := http.NewRequest("DELETE", testServerURL+"/users/"+userId, nil)
	if err != nil {
		return err
	}

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return err
	}
	return nil
}

func createOrganizationUnit(ouRequest OUCreateRequest) (string, error) {
	ouJSON, err := json.Marshal(ouRequest)
	if err != nil {
		return "", fmt.Errorf("failed to marshal OU request: %w", err)
	}

	reqBody := bytes.NewReader(ouJSON)
	req, err := http.NewRequest("POST", testServerURL+"/organization-units", reqBody)
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
		responseBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("expected status 201, got %d. Response: %s", resp.StatusCode, string(responseBody))
	}

	var createdOU OUResponse
	err = json.NewDecoder(resp.Body).Decode(&createdOU)
	if err != nil {
		return "", fmt.Errorf("failed to parse response body: %w", err)
	}

	return createdOU.ID, nil
}

func deleteOrganizationUnit(ouID string) error {
	req, err := http.NewRequest("DELETE", testServerURL+"/organization-units/"+ouID, nil)
	if err != nil {
		return err
	}

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("expected status 200 or 204, got %d", resp.StatusCode)
	}
	return nil
}

func createGroup(groupReq groupCreateRequest) (string, error) {
	groupJSON, err := json.Marshal(groupReq)
	if err != nil {
		return "", fmt.Errorf("failed to marshal group request: %w", err)
	}

	req, err := http.NewRequest("POST", testServerURL+"/groups", bytes.NewReader(groupJSON))
	if err != nil {
		return "", fmt.Errorf("failed to create group request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send group create request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("expected status 201 for group creation, got %d. Response: %s",
			resp.StatusCode, string(bodyBytes))
	}

	var created groupCreateResponse
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		return "", fmt.Errorf("failed to parse group creation response: %w", err)
	}

	if created.ID == "" {
		return "", fmt.Errorf("group creation response does not contain id")
	}

	return created.ID, nil
}

func deleteGroup(groupID string) error {
	req, err := http.NewRequest("DELETE", testServerURL+"/groups/"+groupID, nil)
	if err != nil {
		return fmt.Errorf("failed to create group delete request: %w", err)
	}

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send group delete request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("expected status 204 for group deletion, got %d. Response: %s",
			resp.StatusCode, string(bodyBytes))
	}

	return nil
}
