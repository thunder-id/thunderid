/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package usertype

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

// DisplayAttributeTestSuite tests the display attribute feature across schema CRUD,
// user listing, OU user listing, group members listing, and role assignments.
type DisplayAttributeTestSuite struct {
	suite.Suite
	client           *http.Client
	oUID             string
	createdSchemas   []string
	createdUsers     []string
	createdGroups    []string
	createdRoles     []string
	resourceServerID string
}

var displayTestOU = testutils.OrganizationUnit{
	Handle:      "test-display-attr-ou",
	Name:        "Test Organization Unit for Display Attribute",
	Description: "Organization unit created for display attribute testing",
	Parent:      nil,
}

func TestDisplayAttributeTestSuite(t *testing.T) {
	suite.Run(t, new(DisplayAttributeTestSuite))
}

func (ts *DisplayAttributeTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()

	ouID, err := testutils.CreateOrganizationUnit(displayTestOU)
	if err != nil {
		ts.T().Fatalf("Failed to create test organization unit: %v", err)
	}
	ts.oUID = ouID

	// Create resource server for role tests
	rs := testutils.ResourceServer{
		Name:        "Display Attr Test RS",
		Description: "Resource server for display attribute testing",
		Identifier:  "display-attr-test-rs",
		OUID:        ts.oUID,
	}
	action := testutils.Action{
		Name:        "Read",
		Handle:      "read",
		Description: "Read access",
	}
	rsID, err := testutils.CreateResourceServerWithActions(rs, []testutils.Action{action})
	if err != nil {
		ts.T().Fatalf("Failed to create resource server: %v", err)
	}
	ts.resourceServerID = rsID
}

func (ts *DisplayAttributeTestSuite) TearDownSuite() {
	for _, roleID := range ts.createdRoles {
		if err := testutils.DeleteRole(roleID); err != nil {
			ts.T().Logf("Failed to delete role %s: %v", roleID, err)
		}
	}
	for _, groupID := range ts.createdGroups {
		if err := testutils.DeleteGroup(groupID); err != nil {
			ts.T().Logf("Failed to delete group %s: %v", groupID, err)
		}
	}
	for _, userID := range ts.createdUsers {
		if err := testutils.DeleteUser(userID); err != nil {
			ts.T().Logf("Failed to delete user %s: %v", userID, err)
		}
	}
	for _, schemaID := range ts.createdSchemas {
		if err := testutils.DeleteUserType(schemaID); err != nil {
			ts.T().Logf("Failed to delete user type %s: %v", schemaID, err)
		}
	}
	if ts.resourceServerID != "" {
		if err := testutils.DeleteResourceServer(ts.resourceServerID); err != nil {
			ts.T().Logf("Failed to delete resource server %s: %v", ts.resourceServerID, err)
		}
	}
	if ts.oUID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.oUID); err != nil {
			ts.T().Logf("Failed to delete organization unit %s: %v", ts.oUID, err)
		}
	}
}

// --- Schema CRUD Validation Tests ---

// TestCreateSchemaWithDisplayAttribute_SingleEligible tests that a single eligible string
// attribute can be set as the display attribute.
func (ts *DisplayAttributeTestSuite) TestCreateSchemaWithDisplayAttribute_SingleEligible() {
	schema := CreateUserTypeRequest{
		Name:             "display-single-eligible",
		OUID:             ts.oUID,
		SystemAttributes: &SystemAttributes{Display: "email"},
		Schema:           json.RawMessage(`{"email": {"type": "string"}}`),
	}

	createdSchema := ts.createSchemaExpectSuccess(schema)
	ts.createdSchemas = append(ts.createdSchemas, createdSchema.ID)

	ts.Assert().NotNil(createdSchema.SystemAttributes)
	ts.Assert().Equal("email", createdSchema.SystemAttributes.Display)
}

// TestCreateSchemaWithDisplayAttribute_MultipleEligible tests that when multiple eligible
// attributes exist, an explicit displayAttribute can be set.
func (ts *DisplayAttributeTestSuite) TestCreateSchemaWithDisplayAttribute_MultipleEligible() {
	schema := CreateUserTypeRequest{
		Name:             "display-multiple-eligible",
		OUID:             ts.oUID,
		SystemAttributes: &SystemAttributes{Display: "given_name"},
		Schema: json.RawMessage(`{
			"email": {"type": "string"},
			"given_name": {"type": "string"},
			"family_name": {"type": "string"}
		}`),
	}

	createdSchema := ts.createSchemaExpectSuccess(schema)
	ts.createdSchemas = append(ts.createdSchemas, createdSchema.ID)

	ts.Assert().NotNil(createdSchema.SystemAttributes)
	ts.Assert().Equal("given_name", createdSchema.SystemAttributes.Display)
}

// TestCreateSchemaWithDisplayAttribute_NonExistent tests that setting a non-existent
// attribute as display returns 400.
func (ts *DisplayAttributeTestSuite) TestCreateSchemaWithDisplayAttribute_NonExistent() {
	schema := CreateUserTypeRequest{
		Name:             "display-nonexistent-attr",
		OUID:             ts.oUID,
		SystemAttributes: &SystemAttributes{Display: "nonexistent_field"},
		Schema:           json.RawMessage(`{"email": {"type": "string"}}`),
	}

	ts.createSchemaExpectError(schema, http.StatusBadRequest)
}

// TestCreateSchemaWithDisplayAttribute_NonString tests that setting a non-displayable
// type (boolean) as display returns 400.
func (ts *DisplayAttributeTestSuite) TestCreateSchemaWithDisplayAttribute_NonString() {
	schema := CreateUserTypeRequest{
		Name:             "display-non-string",
		OUID:             ts.oUID,
		SystemAttributes: &SystemAttributes{Display: "is_active"},
		Schema:           json.RawMessage(`{"is_active": {"type": "boolean"}, "email": {"type": "string"}}`),
	}

	ts.createSchemaExpectError(schema, http.StatusBadRequest)
}

// TestCreateSchemaWithDisplayAttribute_Credential tests that setting a credential
// attribute as display returns 400.
func (ts *DisplayAttributeTestSuite) TestCreateSchemaWithDisplayAttribute_Credential() {
	schema := CreateUserTypeRequest{
		Name:             "display-credential-attr",
		OUID:             ts.oUID,
		SystemAttributes: &SystemAttributes{Display: "password"},
		Schema: json.RawMessage(`{
			"email": {"type": "string"},
			"password": {"type": "string", "credential": true}
		}`),
	}

	ts.createSchemaExpectError(schema, http.StatusBadRequest)
}

// TestCreateSchemaWithDisplayAttribute_NumberType tests that a number type attribute
// can be set as display (numbers are displayable).
func (ts *DisplayAttributeTestSuite) TestCreateSchemaWithDisplayAttribute_NumberType() {
	schema := CreateUserTypeRequest{
		Name:             "display-number-type",
		OUID:             ts.oUID,
		SystemAttributes: &SystemAttributes{Display: "employee_id"},
		Schema:           json.RawMessage(`{"employee_id": {"type": "number"}, "email": {"type": "string"}}`),
	}

	createdSchema := ts.createSchemaExpectSuccess(schema)
	ts.createdSchemas = append(ts.createdSchemas, createdSchema.ID)

	ts.Assert().NotNil(createdSchema.SystemAttributes)
	ts.Assert().Equal("employee_id", createdSchema.SystemAttributes.Display)
}

// TestCreateSchemaWithDisplayAttribute_NestedAttribute tests that a nested attribute
// using dot notation can be set as display.
func (ts *DisplayAttributeTestSuite) TestCreateSchemaWithDisplayAttribute_NestedAttribute() {
	schema := CreateUserTypeRequest{
		Name:             "display-nested-attr",
		OUID:             ts.oUID,
		SystemAttributes: &SystemAttributes{Display: "profile.name"},
		Schema: json.RawMessage(`{
			"profile": {
				"type": "object",
				"properties": {
					"name": {"type": "string"},
					"age": {"type": "number"}
				}
			}
		}`),
	}

	createdSchema := ts.createSchemaExpectSuccess(schema)
	ts.createdSchemas = append(ts.createdSchemas, createdSchema.ID)

	ts.Assert().NotNil(createdSchema.SystemAttributes)
	ts.Assert().Equal("profile.name", createdSchema.SystemAttributes.Display)
}

// TestUpdateSchemaDisplayAttribute tests updating the display attribute on an existing schema.
func (ts *DisplayAttributeTestSuite) TestUpdateSchemaDisplayAttribute() {
	// Create schema with email as display
	createReq := CreateUserTypeRequest{
		Name:             "display-update-test",
		OUID:             ts.oUID,
		SystemAttributes: &SystemAttributes{Display: "email"},
		Schema: json.RawMessage(`{
			"email": {"type": "string"},
			"given_name": {"type": "string"}
		}`),
	}

	createdSchema := ts.createSchemaExpectSuccess(createReq)
	ts.createdSchemas = append(ts.createdSchemas, createdSchema.ID)
	ts.Assert().Equal("email", createdSchema.SystemAttributes.Display)

	// Update to given_name as display
	updateReq := UpdateUserTypeRequest{
		Name:             "display-update-test",
		OUID:             ts.oUID,
		SystemAttributes: &SystemAttributes{Display: "given_name"},
		Schema: json.RawMessage(`{
			"email": {"type": "string"},
			"given_name": {"type": "string"}
		}`),
	}

	jsonData, err := json.Marshal(updateReq)
	ts.Require().NoError(err)

	req, err := http.NewRequest("PUT", testServerURL+"/user-types/"+createdSchema.ID, bytes.NewBuffer(jsonData))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Assert().Equal(http.StatusOK, resp.StatusCode, "Should return 200 OK")

	bodyBytes, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)

	var updatedSchema UserType
	err = json.Unmarshal(bodyBytes, &updatedSchema)
	ts.Require().NoError(err)

	ts.Assert().NotNil(updatedSchema.SystemAttributes)
	ts.Assert().Equal("given_name", updatedSchema.SystemAttributes.Display)
}

// --- User Listing with Display Tests ---

// TestUserListingWithDisplay tests GET /users?include=display returns the display value
// and without the param returns the existing format.
func (ts *DisplayAttributeTestSuite) TestUserListingWithDisplay() {
	// Create schema with display attribute set to email
	schemaReq := CreateUserTypeRequest{
		Name:             "display-user-listing",
		OUID:             ts.oUID,
		SystemAttributes: &SystemAttributes{Display: "email"},
		Schema: json.RawMessage(`{
			"email": {"type": "string"},
			"given_name": {"type": "string"},
			"password": {"type": "string", "credential": true}
		}`),
	}

	createdSchema := ts.createSchemaExpectSuccess(schemaReq)
	ts.createdSchemas = append(ts.createdSchemas, createdSchema.ID)

	// Create a user with this schema
	user := testutils.User{
		OUID:       ts.oUID,
		Type:       "display-user-listing",
		Attributes: json.RawMessage(`{"email": "display-test@example.com", "given_name": "DisplayTest", "password": "TestPass123!"}`),
	}
	userID, err := testutils.CreateUser(user)
	ts.Require().NoError(err)
	ts.createdUsers = append(ts.createdUsers, userID)

	// Test: GET /users without include=display
	usersWithout := ts.getUserList("")
	foundUser := ts.findUserInList(usersWithout, userID)
	ts.Require().NotNil(foundUser, "User should be in list")
	ts.Assert().Empty(foundUser.Display, "Display should be empty without include=display")

	// Test: GET /users?include=display
	usersWith := ts.getUserList("display")
	foundUserDisplay := ts.findUserInList(usersWith, userID)
	ts.Require().NotNil(foundUserDisplay, "User should be in list")
	ts.Assert().Equal("display-test@example.com", foundUserDisplay.Display,
		"Display should be the email attribute value")
}

// --- OU User Listing with Display Tests ---

// TestOUUserListingWithDisplay tests GET /organization-units/{id}/users?include=display
// returns display values.
func (ts *DisplayAttributeTestSuite) TestOUUserListingWithDisplay() {
	// Create schema with display attribute
	schemaReq := CreateUserTypeRequest{
		Name:             "display-ou-listing",
		OUID:             ts.oUID,
		SystemAttributes: &SystemAttributes{Display: "given_name"},
		Schema: json.RawMessage(`{
			"email": {"type": "string"},
			"given_name": {"type": "string"},
			"password": {"type": "string", "credential": true}
		}`),
	}

	createdSchema := ts.createSchemaExpectSuccess(schemaReq)
	ts.createdSchemas = append(ts.createdSchemas, createdSchema.ID)

	// Create a user
	user := testutils.User{
		OUID:       ts.oUID,
		Type:       "display-ou-listing",
		Attributes: json.RawMessage(`{"email": "ou-display@example.com", "given_name": "OUDisplay", "password": "TestPass123!"}`),
	}
	userID, err := testutils.CreateUser(user)
	ts.Require().NoError(err)
	ts.createdUsers = append(ts.createdUsers, userID)

	// Test: GET /organization-units/{id}/users without include=display
	ouUsersWithout := ts.getOUUserList(ts.oUID, "")
	foundWithout := ts.findOUUserInList(ouUsersWithout, userID)
	ts.Require().NotNil(foundWithout, "User should be in OU user list")
	ts.Assert().Empty(foundWithout.Display, "Display should be empty without include=display")

	// Test: GET /organization-units/{id}/users?include=display
	ouUsersWith := ts.getOUUserList(ts.oUID, "display")
	foundWith := ts.findOUUserInList(ouUsersWith, userID)
	ts.Require().NotNil(foundWith, "User should be in OU user list")
	ts.Assert().Equal("OUDisplay", foundWith.Display,
		"Display should be the given_name attribute value")
}

// --- Group Members Listing with Display Tests ---

// TestGroupMembersWithDisplay tests GET /groups/{id}/members?include=display
// returns display values.
func (ts *DisplayAttributeTestSuite) TestGroupMembersWithDisplay() {
	// Create schema with display attribute
	schemaReq := CreateUserTypeRequest{
		Name:             "display-group-member",
		OUID:             ts.oUID,
		SystemAttributes: &SystemAttributes{Display: "email"},
		Schema: json.RawMessage(`{
			"email": {"type": "string"},
			"given_name": {"type": "string"},
			"password": {"type": "string", "credential": true}
		}`),
	}

	createdSchema := ts.createSchemaExpectSuccess(schemaReq)
	ts.createdSchemas = append(ts.createdSchemas, createdSchema.ID)

	// Create a user
	user := testutils.User{
		OUID:       ts.oUID,
		Type:       "display-group-member",
		Attributes: json.RawMessage(`{"email": "group-display@example.com", "given_name": "GroupUser", "password": "TestPass123!"}`),
	}
	userID, err := testutils.CreateUser(user)
	ts.Require().NoError(err)
	ts.createdUsers = append(ts.createdUsers, userID)

	// Create a group with this user as member
	groupID := ts.createGroupWithMembers("Display Test Group", []groupMemberReq{
		{ID: userID, Type: "user"},
	})
	ts.createdGroups = append(ts.createdGroups, groupID)

	// Test: GET /groups/{id}/members without include=display
	membersWithout := ts.getGroupMembers(groupID, "")
	ts.Require().NotEmpty(membersWithout, "Should have at least one member")
	memberWithout := ts.findMemberInList(membersWithout, userID)
	ts.Require().NotNil(memberWithout, "User should be in members list")
	ts.Assert().Empty(memberWithout.Display, "Display should be empty without include=display")

	// Test: GET /groups/{id}/members?include=display
	membersWith := ts.getGroupMembers(groupID, "display")
	ts.Require().NotEmpty(membersWith, "Should have at least one member")
	memberWith := ts.findMemberInList(membersWith, userID)
	ts.Require().NotNil(memberWith, "User should be in members list")
	ts.Assert().Equal("group-display@example.com", memberWith.Display,
		"Display should be the email attribute value")
}

// --- Role Assignments with Display Tests ---

// TestRoleAssignmentsWithDisplayAttribute tests GET /roles/{id}/assignments?include=display
// returns the actual display attribute value instead of user ID.
func (ts *DisplayAttributeTestSuite) TestRoleAssignmentsWithDisplayAttribute() {
	// Create schema with display attribute
	schemaReq := CreateUserTypeRequest{
		Name:             "display-role-assign",
		OUID:             ts.oUID,
		SystemAttributes: &SystemAttributes{Display: "email"},
		Schema: json.RawMessage(`{
			"email": {"type": "string"},
			"given_name": {"type": "string"},
			"password": {"type": "string", "credential": true}
		}`),
	}

	createdSchema := ts.createSchemaExpectSuccess(schemaReq)
	ts.createdSchemas = append(ts.createdSchemas, createdSchema.ID)

	// Create a user
	user := testutils.User{
		OUID:       ts.oUID,
		Type:       "display-role-assign",
		Attributes: json.RawMessage(`{"email": "role-display@example.com", "given_name": "RoleUser", "password": "TestPass123!"}`),
	}
	userID, err := testutils.CreateUser(user)
	ts.Require().NoError(err)
	ts.createdUsers = append(ts.createdUsers, userID)

	// Create a group
	group := testutils.Group{
		Name:        "Display Role Test Group",
		Description: "Group for display attribute role testing",
		OUID:        ts.oUID,
	}
	groupID, err := testutils.CreateGroup(group)
	ts.Require().NoError(err)
	ts.createdGroups = append(ts.createdGroups, groupID)

	// Create a role with both user and group assignments
	roleReq := roleCreateRequest{
		Name: "Display Attr Test Role",
		OUID: ts.oUID,
		Permissions: []resourcePermissions{
			{
				ResourceServerID: ts.resourceServerID,
				Permissions:      []string{"read"},
			},
		},
		Assignments: []roleAssignment{
			{ID: userID, Type: "user"},
			{ID: groupID, Type: "group"},
		},
	}
	roleID := ts.createRole(roleReq)
	ts.createdRoles = append(ts.createdRoles, roleID)

	// Test: GET /roles/{id}/assignments without include=display
	assignmentsWithout := ts.getRoleAssignments(roleID, "")
	ts.Assert().Equal(2, assignmentsWithout.TotalResults)
	for _, a := range assignmentsWithout.Assignments {
		ts.Assert().Empty(a.Display, "Display should be empty without include=display")
	}

	// Test: GET /roles/{id}/assignments?include=display
	assignmentsWith := ts.getRoleAssignments(roleID, "display")
	ts.Assert().Equal(2, assignmentsWith.TotalResults)

	userFound := false
	groupFound := false
	for _, a := range assignmentsWith.Assignments {
		ts.Assert().NotEmpty(a.Display, "Display should be populated with include=display")

		if a.Type == "user" && a.ID == userID {
			userFound = true
			ts.Assert().Equal("role-display@example.com", a.Display,
				"User display should be the email attribute value, not the user ID")
		}
		if a.Type == "group" && a.ID == groupID {
			groupFound = true
			ts.Assert().Equal("Display Role Test Group", a.Display,
				"Group display should be the group name")
		}
	}
	ts.Assert().True(userFound, "User assignment should be found")
	ts.Assert().True(groupFound, "Group assignment should be found")
}

// --- Helper Types ---

type userListResponse struct {
	TotalResults int        `json:"totalResults"`
	StartIndex   int        `json:"startIndex"`
	Count        int        `json:"count"`
	Users        []userItem `json:"users"`
}

type userItem struct {
	ID         string          `json:"id"`
	OUID       string          `json:"ouId,omitempty"`
	Type       string          `json:"type,omitempty"`
	Attributes json.RawMessage `json:"attributes,omitempty"`
	Display    string          `json:"display,omitempty"`
}

type ouUserItem struct {
	ID      string `json:"id"`
	Type    string `json:"type,omitempty"`
	Display string `json:"display,omitempty"`
}

type ouUserListResponse struct {
	TotalResults int          `json:"totalResults"`
	StartIndex   int          `json:"startIndex"`
	Count        int          `json:"count"`
	Users        []ouUserItem `json:"users"`
}

type groupMemberItem struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Display string `json:"display,omitempty"`
}

type groupMemberListResponse struct {
	TotalResults int               `json:"totalResults"`
	StartIndex   int               `json:"startIndex"`
	Count        int               `json:"count"`
	Members      []groupMemberItem `json:"members"`
}

type groupMemberReq struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

type roleAssignment struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Display string `json:"display,omitempty"`
}

type resourcePermissions struct {
	ResourceServerID string   `json:"resourceServerId"`
	Permissions      []string `json:"permissions"`
}

type roleCreateRequest struct {
	Name        string                `json:"name"`
	OUID        string                `json:"ouId"`
	Permissions []resourcePermissions `json:"permissions"`
	Assignments []roleAssignment      `json:"assignments,omitempty"`
}

type roleCreateResponse struct {
	ID string `json:"id"`
}

type assignmentListResponse struct {
	TotalResults int              `json:"totalResults"`
	StartIndex   int              `json:"startIndex"`
	Count        int              `json:"count"`
	Assignments  []roleAssignment `json:"assignments"`
}

// --- Helper Functions ---

func (ts *DisplayAttributeTestSuite) createSchemaExpectSuccess(schema CreateUserTypeRequest) UserType {
	ts.T().Helper()

	jsonData, err := json.Marshal(schema)
	ts.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+"/user-types", bytes.NewBuffer(jsonData))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)

	ts.Require().Equal(http.StatusCreated, resp.StatusCode,
		"Expected 201 Created, got %d. Response: %s", resp.StatusCode, string(bodyBytes))

	var createdSchema UserType
	err = json.Unmarshal(bodyBytes, &createdSchema)
	ts.Require().NoError(err)

	return createdSchema
}

func (ts *DisplayAttributeTestSuite) createSchemaExpectError(schema CreateUserTypeRequest, expectedStatus int) {
	ts.T().Helper()

	jsonData, err := json.Marshal(schema)
	ts.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+"/user-types", bytes.NewBuffer(jsonData))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)

	ts.Assert().Equal(expectedStatus, resp.StatusCode,
		"Expected status %d, got %d. Response: %s", expectedStatus, resp.StatusCode, string(bodyBytes))

	var errorResp ErrorResponse
	err = json.Unmarshal(bodyBytes, &errorResp)
	ts.Require().NoError(err)
	ts.Assert().NotEmpty(errorResp.Code, "Error should have code")
	ts.Assert().NotEmpty(errorResp.Message.DefaultValue, "Error should have message")
}

func (ts *DisplayAttributeTestSuite) getUserList(include string) []userItem {
	ts.T().Helper()

	url := testServerURL + "/users?limit=100"
	if include != "" {
		url += "&include=" + include
	}

	req, err := http.NewRequest("GET", url, nil)
	ts.Require().NoError(err)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Require().Equal(http.StatusOK, resp.StatusCode)

	var listResp userListResponse
	err = json.NewDecoder(resp.Body).Decode(&listResp)
	ts.Require().NoError(err)

	return listResp.Users
}

func (ts *DisplayAttributeTestSuite) findUserInList(users []userItem, userID string) *userItem {
	for i := range users {
		if users[i].ID == userID {
			return &users[i]
		}
	}
	return nil
}

func (ts *DisplayAttributeTestSuite) getOUUserList(ouID, include string) []ouUserItem {
	ts.T().Helper()

	url := fmt.Sprintf("%s/organization-units/%s/users?limit=100", testServerURL, ouID)
	if include != "" {
		url += "&include=" + include
	}

	req, err := http.NewRequest("GET", url, nil)
	ts.Require().NoError(err)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Require().Equal(http.StatusOK, resp.StatusCode)

	var listResp ouUserListResponse
	err = json.NewDecoder(resp.Body).Decode(&listResp)
	ts.Require().NoError(err)

	return listResp.Users
}

func (ts *DisplayAttributeTestSuite) findOUUserInList(users []ouUserItem, userID string) *ouUserItem {
	for i := range users {
		if users[i].ID == userID {
			return &users[i]
		}
	}
	return nil
}

func (ts *DisplayAttributeTestSuite) getGroupMembers(groupID, include string) []groupMemberItem {
	ts.T().Helper()

	url := fmt.Sprintf("%s/groups/%s/members?limit=100", testServerURL, groupID)
	if include != "" {
		url += "&include=" + include
	}

	req, err := http.NewRequest("GET", url, nil)
	ts.Require().NoError(err)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Require().Equal(http.StatusOK, resp.StatusCode)

	var listResp groupMemberListResponse
	err = json.NewDecoder(resp.Body).Decode(&listResp)
	ts.Require().NoError(err)

	return listResp.Members
}

func (ts *DisplayAttributeTestSuite) findMemberInList(members []groupMemberItem, memberID string) *groupMemberItem {
	for i := range members {
		if members[i].ID == memberID {
			return &members[i]
		}
	}
	return nil
}

func (ts *DisplayAttributeTestSuite) createGroupWithMembers(name string, members []groupMemberReq) string {
	ts.T().Helper()

	reqBody := map[string]interface{}{
		"name":        name,
		"description": "Group for display attribute testing",
		"ouId":        ts.oUID,
		"members":     members,
	}

	jsonData, err := json.Marshal(reqBody)
	ts.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+"/groups", bytes.NewReader(jsonData))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusCreated, resp.StatusCode,
		"Expected 201, got %d. Response: %s", resp.StatusCode, string(bodyBytes))

	var created map[string]interface{}
	err = json.Unmarshal(bodyBytes, &created)
	ts.Require().NoError(err)

	return created["id"].(string)
}

func (ts *DisplayAttributeTestSuite) createRole(roleReq roleCreateRequest) string {
	ts.T().Helper()

	jsonData, err := json.Marshal(roleReq)
	ts.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+"/roles", bytes.NewReader(jsonData))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusCreated, resp.StatusCode,
		"Expected 201, got %d. Response: %s", resp.StatusCode, string(bodyBytes))

	var created roleCreateResponse
	err = json.Unmarshal(bodyBytes, &created)
	ts.Require().NoError(err)

	return created.ID
}

func (ts *DisplayAttributeTestSuite) getRoleAssignments(roleID, include string) assignmentListResponse {
	ts.T().Helper()

	url := fmt.Sprintf("%s/roles/%s/assignments?offset=0&limit=100", testServerURL, roleID)
	if include != "" {
		url += "&include=" + include
	}

	req, err := http.NewRequest("GET", url, nil)
	ts.Require().NoError(err)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, resp.StatusCode,
		"Expected 200, got %d. Response: %s", resp.StatusCode, string(bodyBytes))

	var listResp assignmentListResponse
	err = json.Unmarshal(bodyBytes, &listResp)
	ts.Require().NoError(err)

	return listResp
}
