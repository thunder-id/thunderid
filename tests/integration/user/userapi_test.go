// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

var testServerURL = testutils.TestServerURL

const (
	groupMemberTypeUser = "user"
)

type groupMember struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

type groupCreateRequest struct {
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	OUID        string        `json:"ouId"`
	Members     []groupMember `json:"members,omitempty"`
}

type groupCreateResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

var (
	entityType = testutils.UserType{
		Handle:      "test-user-person",
		DisplayName: "Test User Person",
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
	createdUserID          string
	testOUID               string
	createdGroupID         string
	entityTypeID           string
	credentialEntityTypeID string
	credentialUserID       string
	credentialUsername     string
	credentialPassword     string
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

	// Create user type with credential fields for credential update tests.
	credentialUsername = "admin.cred.user"
	credentialPassword = "InitialP@ssw0rd!"

	credentialType := testutils.UserType{
		Handle:      "admin-cred-update-type",
		DisplayName: "Admin Cred Update Type",
		OUID:        testOUID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string", "required": true, "unique": true},
			"email":    map[string]interface{}{"type": "string", "required": true, "unique": true},
			"password": map[string]interface{}{"type": "string", "credential": true, "regex": "^.{8,}$"},
		},
	}
	credTypeID, err := testutils.CreateUserType(credentialType)
	if err != nil {
		ts.T().Fatalf("Failed to create credential user type during setup: %v", err)
	}
	credentialEntityTypeID = credTypeID

	credAttrs, err := json.Marshal(map[string]interface{}{
		"username": credentialUsername,
		"email":    "admin.cred.user@example.com",
		"password": credentialPassword,
	})
	if err != nil {
		ts.T().Fatalf("Failed to marshal credential user attributes: %v", err)
	}

	credUserID, err := testutils.CreateUser(testutils.User{
		OUID:       testOUID,
		Type:       "admin-cred-update-type",
		Attributes: credAttrs,
	})
	if err != nil {
		ts.T().Fatalf("Failed to create credential user during setup: %v", err)
	}
	credentialUserID = credUserID

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

	// Delete the credential test user and type
	if credentialUserID != "" {
		if err := deleteUser(credentialUserID); err != nil {
			ts.T().Logf("Failed to delete credential user during teardown: %v", err)
		}
	}
	if credentialEntityTypeID != "" {
		if err := testutils.DeleteUserType(credentialEntityTypeID); err != nil {
			ts.T().Logf("Failed to delete credential user type during teardown: %v", err)
		}
	}

	// Delete the test user first
	if createdUserID != "" {
		err := deleteUser(createdUserID)
		if err != nil {
			ts.T().Logf("Failed to delete user during teardown: %v", err)
		}
	}

	if entityTypeID != "" {
		if err := testutils.DeleteUserType(entityTypeID); err != nil {
			ts.T().Logf("Failed to delete user type during teardown: %v", err)
		}
	}

	// Delete the test organization unit
	if testOUID != "" {
		err := deleteOrganizationUnit(testOUID)
		if err != nil {
			ts.T().Logf("Failed to delete organization unit during teardown: %v", err)
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
		ID:         createdUserID,
		OUID:       testOUID,
		Type:       testUser.Type,
		Attributes: testUser.Attributes,
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
		ID:         createdUserID,
		OUID:       testOUID,
		Type:       testUser.Type,
		Attributes: testUser.Attributes,
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
		ID:         createdUserID,
		OUID:       userToUpdate.OUID,
		Type:       userToUpdate.Type,
		Attributes: userToUpdate.Attributes,
	})
}

func (ts *UserAPITestSuite) TestUserUpdateReportsTheDerivedStatus() {
	ts.Require().NotEmpty(createdUserID, "User ID is not available for update")
	userToUpdate := userUpdate
	userToUpdate.OUID = testOUID
	encoded, err := json.Marshal(userToUpdate)
	ts.Require().NoError(err)
	var body map[string]interface{}
	ts.Require().NoError(json.Unmarshal(encoded, &body))
	body["status"] = map[string]interface{}{"value": "SUSPENDED"}
	payload, err := json.Marshal(body)
	ts.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPut, testServerURL+"/users/"+createdUserID, bytes.NewReader(payload))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, resp.StatusCode, string(raw))

	var updated struct {
		Status json.RawMessage `json:"status"`
	}
	ts.Require().NoError(json.Unmarshal(raw, &updated))
	ts.JSONEq(`{"value":"ACTIVE"}`, string(updated.Status), "the update must report the derived status")
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

func (ts *UserAPITestSuite) TestCredentialUpdateByAdmin() {
	newPassword := credentialPassword + "Updated!"

	payload, err := json.Marshal(map[string]interface{}{
		"credentials": map[string]interface{}{
			"password": newPassword,
		},
	})
	ts.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/users/%s/update-credentials", testServerURL, credentialUserID),
		bytes.NewReader(payload))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Require().Equal(http.StatusNoContent, resp.StatusCode)

	// Verify the new password works by authenticating.
	authenticated, err := testutils.AuthenticateWithCredential(
		"username", credentialUsername, "password", newPassword)
	ts.Require().NoError(err)
	ts.True(authenticated)

	// Verify the old password no longer works.
	authenticated, err = testutils.AuthenticateWithCredential(
		"username", credentialUsername, "password", credentialPassword)
	ts.Require().NoError(err)
	ts.False(authenticated)

	credentialPassword = newPassword
}

func (ts *UserAPITestSuite) TestCredentialUpdateByAdmin_PasswordFailsSchemaRegex() {
	payload, err := json.Marshal(map[string]interface{}{
		"credentials": map[string]interface{}{
			"password": "1",
		},
	})
	ts.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/users/%s/update-credentials", testServerURL, credentialUserID),
		bytes.NewReader(payload))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Require().Equal(http.StatusBadRequest, resp.StatusCode)
	var errorResp testutils.ErrorResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&errorResp))
	ts.Equal("USR-1041", errorResp.Code)

	// The current password still works, since the rejected one was not stored.
	authenticated, err := testutils.AuthenticateWithCredential(
		"username", credentialUsername, "password", credentialPassword)
	ts.Require().NoError(err)
	ts.True(authenticated)
}

func (ts *UserAPITestSuite) TestCredentialUpdateByAdmin_EmptyCredentials() {
	payload, err := json.Marshal(map[string]interface{}{
		"credentials": map[string]interface{}{},
	})
	ts.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/users/%s/update-credentials", testServerURL, credentialUserID),
		bytes.NewReader(payload))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Require().Equal(http.StatusBadRequest, resp.StatusCode)
}

func (ts *UserAPITestSuite) TestCredentialUpdateByAdmin_MissingCredentialsField() {
	payload, err := json.Marshal(map[string]interface{}{})
	ts.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/users/%s/update-credentials", testServerURL, credentialUserID),
		bytes.NewReader(payload))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Require().Equal(http.StatusBadRequest, resp.StatusCode)
}

func (ts *UserAPITestSuite) TestCredentialUpdateByAdmin_NonExistentUser() {
	payload, err := json.Marshal(map[string]interface{}{
		"credentials": map[string]interface{}{
			"password": "newpassword",
		},
	})
	ts.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/users/%s/update-credentials", testServerURL, "non-existent-user-id"),
		bytes.NewReader(payload))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Require().Equal(http.StatusNotFound, resp.StatusCode)
}

func (ts *UserAPITestSuite) TestCredentialUpdateByAdmin_SystemManagedCredential() {
	payload, err := json.Marshal(map[string]interface{}{
		"credentials": map[string]interface{}{
			"passkey": "some-passkey-value",
		},
	})
	ts.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/users/%s/update-credentials", testServerURL, credentialUserID),
		bytes.NewReader(payload))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Require().Equal(http.StatusBadRequest, resp.StatusCode)
}

func (ts *UserAPITestSuite) TestCredentialUpdateByAdmin_PutUserRejectsCredentials() {
	attrs, err := json.Marshal(map[string]interface{}{
		"username": credentialUsername,
		"email":    "admin.cred.user@example.com",
		"password": "ShouldBeRejected!",
	})
	ts.Require().NoError(err)

	payload, err := json.Marshal(map[string]interface{}{
		"ouId":       testOUID,
		"type":       "admin-cred-update-type",
		"attributes": json.RawMessage(attrs),
	})
	ts.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPut,
		fmt.Sprintf("%s/users/%s", testServerURL, credentialUserID),
		bytes.NewReader(payload))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Require().Equal(http.StatusBadRequest, resp.StatusCode)
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

// verificationContactUser is a user whose type declares email as a contact, with its token client.
type verificationContactUser struct {
	id, typeHandle, username, email, password string
	client                                    *http.Client
}

// newVerificationContactUser creates, with supplied verification, a user whose type declares email as a
// contact, and removes both when the test ends. Supplied verification could only take effect on such an
// attribute.
func (ts *UserAPITestSuite) newVerificationContactUser(name string) verificationContactUser {
	ts.T().Helper()

	user := verificationContactUser{typeHandle: "verification-contact-" + name,
		username: "verification." + name, email: "verification." + name + "@example.com",
		password: "Verification-P@ssw0rd1"}
	typeID, err := testutils.CreateUserType(testutils.UserType{
		Handle: user.typeHandle, DisplayName: "Verification Contact " + name, OUID: testOUID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string", "required": true, "unique": true},
			"password": map[string]interface{}{"type": "string", "credential": true},
			"email":    map[string]interface{}{"type": "email"},
		},
	})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { ts.NoError(testutils.DeleteUserType(typeID)) })

	status, body := sendUserRequest(ts.T(), testutils.GetHTTPClient(), http.MethodPost, "/users",
		map[string]interface{}{
			"ouId": testOUID, "type": user.typeHandle,
			"attributes": map[string]interface{}{"username": user.username, "password": user.password,
				"email": user.email},
			"verifiedAttributes":    suppliedVerification(),
			"attributeVerification": suppliedVerification(),
			"systemAttributes":      map[string]interface{}{"attributeVerification": suppliedVerification()},
		})
	ts.Require().Equal(http.StatusCreated, status, "%s", body)
	var created testutils.User
	ts.Require().NoError(json.Unmarshal(body, &created))
	user.id = created.ID
	ts.T().Cleanup(func() { ts.NoError(deleteUser(user.id)) })
	ts.Empty(verifiedAttributesIn(ts.T(), body), "the create response must report nothing verified")

	// The user lives outside the default OU, and a sign-in is bounded to its application's OU, so it signs in
	// through an application in its own OU rather than the Console.
	var signIn testutils.OUSignInApps
	ts.T().Cleanup(func() { ts.NoError(signIn.DeleteAll()) })
	user.client, err = signIn.HTTPClientForUser(testOUID, user.username, user.password)
	ts.Require().NoError(err)
	return user
}

// requireNothingVerified checks that every read of the user, by ID, in a filtered list and the user's
// own, reports verifiedAttributes as an empty object.
func (ts *UserAPITestSuite) requireNothingVerified(user verificationContactUser) {
	ts.T().Helper()

	_, byID := sendUserRequest(ts.T(), testutils.GetHTTPClient(), http.MethodGet, "/users/"+user.id, nil)
	ts.Empty(verifiedAttributesIn(ts.T(), byID), "GET /users/{id} must report nothing verified")

	filter := url.QueryEscape(fmt.Sprintf(`username eq "%s"`, user.username))
	status, list := sendUserRequest(ts.T(), testutils.GetHTTPClient(), http.MethodGet, "/users?filter="+filter,
		nil)
	ts.Require().Equal(http.StatusOK, status, "%s", list)
	var listed struct {
		Users []json.RawMessage `json:"users"`
	}
	ts.Require().NoError(json.Unmarshal(list, &listed))
	ts.Require().Len(listed.Users, 1, "the filter must find exactly the user: %s", list)
	ts.Empty(verifiedAttributesIn(ts.T(), listed.Users[0]), "GET /users must report nothing verified")

	_, own := sendUserRequest(ts.T(), user.client, http.MethodGet, "/users/me", nil)
	ts.Empty(verifiedAttributesIn(ts.T(), own), "GET /users/me must report nothing verified")
}

// TestUserCreateCannotSetVerification checks that verification supplied with a create is ignored.
func (ts *UserAPITestSuite) TestUserCreateCannotSetVerification() {
	ts.requireNothingVerified(ts.newVerificationContactUser("create"))
}

// TestUserUpdateCannotSetVerification checks that verification supplied with an administrator's or the
// user's own update is ignored.
func (ts *UserAPITestSuite) TestUserUpdateCannotSetVerification() {
	user := ts.newVerificationContactUser("update")
	attributes := map[string]interface{}{"username": user.username, "email": user.email}

	status, body := sendUserRequest(ts.T(), testutils.GetHTTPClient(), http.MethodPut, "/users/"+user.id,
		map[string]interface{}{
			"ouId": testOUID, "type": user.typeHandle, "attributes": attributes,
			"verifiedAttributes": suppliedVerification(),
			"systemAttributes":   map[string]interface{}{"attributeVerification": suppliedVerification()},
		})
	ts.Require().Equal(http.StatusOK, status, "%s", body)
	ts.Empty(verifiedAttributesIn(ts.T(), body), "the administrator's update must report nothing verified")

	status, body = sendUserRequest(ts.T(), user.client, http.MethodPut, "/users/me",
		map[string]interface{}{"attributes": attributes, "verifiedAttributes": suppliedVerification()})
	ts.Require().Equal(http.StatusOK, status, "%s", body)
	ts.Empty(verifiedAttributesIn(ts.T(), body), "the self update must report nothing verified")

	ts.requireNothingVerified(user)
}

// suppliedVerification is what a client might send to claim a verified email.
func suppliedVerification() map[string]interface{} {
	return map[string]interface{}{"email": map[string]interface{}{"verifiedAt": "2026-01-01T00:00:00Z"}}
}

// verifiedAttributesIn decodes the verifiedAttributes object of a user response, requiring it present
// and an object, not null.
func verifiedAttributesIn(t *testing.T, body []byte) map[string]testutils.VerifiedAttribute {
	t.Helper()

	var response struct {
		VerifiedAttributes *map[string]testutils.VerifiedAttribute `json:"verifiedAttributes"`
	}
	require.NoError(t, json.Unmarshal(body, &response), "%s", body)
	require.NotNil(t, response.VerifiedAttributes, "a user response must carry verifiedAttributes: %s", body)
	require.NotNil(t, *response.VerifiedAttributes, "verifiedAttributes must be an object: %s", body)
	return *response.VerifiedAttributes
}

// sendUserRequest sends a JSON request and returns the status and body.
func sendUserRequest(t *testing.T, client *http.Client, method, path string,
	payload interface{}) (int, []byte) {
	t.Helper()

	var reader io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		require.NoError(t, err)
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequest(method, testServerURL+path, reader)
	require.NoError(t, err)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, body
}
