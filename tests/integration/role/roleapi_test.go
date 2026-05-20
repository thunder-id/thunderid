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

package role

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	testServerURL = "https://localhost:8095"
	rolesBasePath = "/roles"
)

var (
	testOU = testutils.OrganizationUnit{
		Handle:      "test-role-ou",
		Name:        "Test Organization Unit for Roles",
		Description: "Organization unit created for role API testing",
		Parent:      nil,
	}

	testUserType = testutils.UserType{
		Name: "role-person",
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

	testUser1 = testutils.User{
		Type: "role-person",
		Attributes: json.RawMessage(`{
			"email": "roleuser1@example.com",
			"given_name": "Role",
			"family_name": "User1",
			"password": "TestPassword123!"
		}`),
	}

	testUser2 = testutils.User{
		Type: "role-person",
		Attributes: json.RawMessage(`{
			"email": "roleuser2@example.com",
			"given_name": "Role",
			"family_name": "User2",
			"password": "TestPassword123!"
		}`),
	}

	testGroup = testutils.Group{
		Name:        "Test Role Group",
		Description: "Group created for role API testing",
	}
)

var (
	testOUID     string
	testUserID1  string
	testUserID2  string
	testGroupID  string
	testAppID    string
	sharedRoleID string // Shared role created in SetupSuite for tests that need a pre-existing role
	entityTypeID string

	// Resource servers for permission testing
	testResourceServer1ID string
	testResourceServer2ID string

	// Permission strings derived from actions
	testPermission1 = "read"
	testPermission2 = "write"
	testPermission3 = "process"
)

type RoleAPITestSuite struct {
	suite.Suite
	client *http.Client
}

func TestRoleAPITestSuite(t *testing.T) {
	suite.Run(t, new(RoleAPITestSuite))
}

func (suite *RoleAPITestSuite) SetupSuite() {
	// Create HTTP client that skips TLS verification for testing
	suite.client = testutils.GetHTTPClient()

	// Create test organization unit
	ouID, err := testutils.CreateOrganizationUnit(testOU)
	suite.Require().NoError(err, "Failed to create test organization unit")
	testOUID = ouID
	testUserType.OUID = testOUID

	// Create user type
	schemaID, err := testutils.CreateUserType(testUserType)
	suite.Require().NoError(err, "Failed to create user type")
	entityTypeID = schemaID

	// Create test users
	user1 := testUser1
	user1.OUID = testOUID
	userID1, err := testutils.CreateUser(user1)
	suite.Require().NoError(err, "Failed to create test user 1")
	testUserID1 = userID1

	user2 := testUser2
	user2.OUID = testOUID
	userID2, err := testutils.CreateUser(user2)
	suite.Require().NoError(err, "Failed to create test user 2")
	testUserID2 = userID2

	// Create test group
	groupToCreate := testGroup
	groupToCreate.OUID = testOUID
	groupID, err := testutils.CreateGroup(groupToCreate)
	suite.Require().NoError(err, "Failed to create test group")
	testGroupID = groupID

	// Create test application (app entity)
	appID, err := testutils.CreateApplication(testutils.Application{
		Name:         "Role Test App",
		Description:  "Application for role assignment testing",
		OUID:         testOUID,
		ClientID:     "role-test-app-client",
		ClientSecret: "role-test-app-secret",
	})
	suite.Require().NoError(err, "Failed to create test application")
	testAppID = appID

	// Create test resource servers
	rs1 := testutils.ResourceServer{
		Name:        "Test Booking System",
		Description: "Resource server for testing role permissions",
		Identifier:  "test-booking-system",
		OUID:        testOUID,
	}
	// Create actions on resource server 1
	action1 := testutils.Action{
		Name:        "Read Bookings",
		Handle:      testPermission1,
		Description: "Read booking information",
	}
	action2 := testutils.Action{
		Name:        "Write Bookings",
		Handle:      testPermission2,
		Description: "Create and modify bookings",
	}
	rsID1, err := testutils.CreateResourceServerWithActions(rs1, []testutils.Action{action1, action2})
	suite.Require().NoError(err, "Failed to create test resource server 1")
	testResourceServer1ID = rsID1

	rs2 := testutils.ResourceServer{
		Name:        "Test Payment System",
		Description: "Second resource server for multi-server testing",
		Identifier:  "test-payment-system",
		OUID:        testOUID,
	}
	action3 := testutils.Action{
		Name:        "Process Payments",
		Handle:      testPermission3,
		Description: "Handle payment processing",
	}
	rsID2, err := testutils.CreateResourceServerWithActions(rs2, []testutils.Action{action3})
	suite.Require().NoError(err, "Failed to create test resource server 2")
	testResourceServer2ID = rsID2

	// Create a shared role that can be used by multiple tests
	sharedRole := CreateRoleRequest{
		Name:        "Test Admin Role",
		Description: "Admin role for testing",
		OUID:        testOUID,
		Permissions: []ResourcePermissions{
			{
				ResourceServerID: testResourceServer1ID,
				Permissions:      []string{testPermission1, testPermission2},
			},
		},
	}
	role, err := suite.createRole(sharedRole)
	suite.Require().NoError(err, "Failed to create shared role")
	sharedRoleID = role.ID
}

func (suite *RoleAPITestSuite) TearDownSuite() {
	// Cleanup in reverse order - roles first
	if sharedRoleID != "" {
		_ = suite.deleteRole(sharedRoleID)
	}

	// Then group and users
	if testGroupID != "" {
		_ = testutils.DeleteGroup(testGroupID)
	}
	if testAppID != "" {
		_ = testutils.DeleteApplication(testAppID)
	}
	if testUserID2 != "" {
		_ = testutils.DeleteUser(testUserID2)
	}
	if testUserID1 != "" {
		_ = testutils.DeleteUser(testUserID1)
	}

	// Then resource servers (actions deleted via cascade)
	if testResourceServer2ID != "" {
		_ = testutils.DeleteResourceServer(testResourceServer2ID)
	}
	if testResourceServer1ID != "" {
		_ = testutils.DeleteResourceServer(testResourceServer1ID)
	}

	// Finally schema and OU
	if entityTypeID != "" {
		_ = testutils.DeleteUserType(entityTypeID)
	}
	if testOUID != "" {
		_ = testutils.DeleteOrganizationUnit(testOUID)
	}
}

// Test 1: Create Role
func (suite *RoleAPITestSuite) TestCreateRole_Success() {
	roleRequest := CreateRoleRequest{
		Name:        "Test Create Role Success",
		Description: "Test role created in TestCreateRole_Success",
		OUID:        testOUID,
		Permissions: []ResourcePermissions{
			{
				ResourceServerID: testResourceServer1ID,
				Permissions:      []string{testPermission1, testPermission2},
			},
		},
	}

	role, err := suite.createRole(roleRequest)
	suite.Require().NoError(err)
	suite.Require().NotNil(role)

	suite.NotEmpty(role.ID)
	suite.Equal(roleRequest.Name, role.Name)
	suite.Equal(roleRequest.Description, role.Description)
	suite.Equal(roleRequest.OUID, role.OUID)
	suite.Equal(1, len(role.Permissions))
	suite.Equal(testResourceServer1ID, role.Permissions[0].ResourceServerID)
	suite.Equal(2, len(role.Permissions[0].Permissions))

	// Cleanup
	_ = suite.deleteRole(role.ID)
}

// Test 2: Create Role with Assignments
func (suite *RoleAPITestSuite) TestCreateRole_WithAssignments() {
	roleRequest := CreateRoleRequest{
		Name:        "Test Role With Assignments",
		Description: "Role with initial assignments",
		OUID:        testOUID,
		Permissions: []ResourcePermissions{
			{
				ResourceServerID: testResourceServer1ID,
				Permissions:      []string{testPermission1},
			},
		},
		Assignments: []Assignment{
			{ID: testUserID1, Type: AssigneeTypeUser},
		},
	}

	role, err := suite.createRole(roleRequest)
	suite.Require().NoError(err)
	suite.Require().NotNil(role)

	suite.Equal(1, len(role.Assignments))
	suite.Equal(testUserID1, role.Assignments[0].ID)
	suite.Equal(AssigneeTypeUser, role.Assignments[0].Type)

	// Cleanup
	_ = suite.deleteRole(role.ID)
}

// Test 3: Create Role without Permissions
func (suite *RoleAPITestSuite) TestCreateRole_WithoutPermissions() {
	roleRequest := CreateRoleRequest{
		Name:        "Test Role Without Permissions",
		Description: "Role without permissions",
		OUID:        testOUID,
		Permissions: []ResourcePermissions{},
		Assignments: []Assignment{
			{ID: testUserID1, Type: AssigneeTypeUser},
		},
	}

	role, err := suite.createRole(roleRequest)
	suite.Require().NoError(err)
	suite.Require().NotNil(role)

	suite.Equal(1, len(role.Assignments))
	suite.Equal(testUserID1, role.Assignments[0].ID)
	suite.Equal(AssigneeTypeUser, role.Assignments[0].Type)

	// Cleanup
	_ = suite.deleteRole(role.ID)
}

// Test 4: Create Role - Validation Errors
func (suite *RoleAPITestSuite) TestCreateRole_ValidationErrors() {
	testCases := []struct {
		name        string
		roleRequest CreateRoleRequest
		expectedErr string
	}{
		{
			name: "Missing Name",
			roleRequest: CreateRoleRequest{
				OUID: testOUID,
				Permissions: []ResourcePermissions{
					{
						ResourceServerID: testResourceServer1ID,
						Permissions:      []string{testPermission1},
					},
				},
			},
			expectedErr: "ROL-1001",
		},
		{
			name: "Missing OUID",
			roleRequest: CreateRoleRequest{
				Name: "Test Role",
				Permissions: []ResourcePermissions{
					{
						ResourceServerID: testResourceServer1ID,
						Permissions:      []string{testPermission1},
					},
				},
			},
			expectedErr: "ROL-1001",
		},
		{
			name: "Invalid Organization Unit",
			roleRequest: CreateRoleRequest{
				Name: "Test Role",
				OUID: "nonexistent-ou",
				Permissions: []ResourcePermissions{
					{
						ResourceServerID: testResourceServer1ID,
						Permissions:      []string{testPermission1},
					},
				},
			},
			expectedErr: "ROL-1005",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			role, err := suite.createRole(tc.roleRequest)
			suite.Error(err)
			suite.Nil(role)
			suite.Contains(err.Error(), tc.expectedErr)
		})
	}
}

// Test 5: Get Role
func (suite *RoleAPITestSuite) TestGetRole_Success() {
	suite.Require().NotEmpty(sharedRoleID, "Shared role must be created in SetupSuite")

	role, err := suite.getRole(sharedRoleID)
	suite.Require().NoError(err)
	suite.Require().NotNil(role)

	suite.Equal(sharedRoleID, role.ID)
	suite.Equal("Test Admin Role", role.Name)
	suite.Equal("Admin role for testing", role.Description)
	suite.Equal(1, len(role.Permissions))
	suite.Equal(testResourceServer1ID, role.Permissions[0].ResourceServerID)
}

// Test 6: Get Role - Not Found
func (suite *RoleAPITestSuite) TestGetRole_NotFound() {
	role, err := suite.getRole("nonexistent-role-id")
	suite.Error(err)
	suite.Nil(role)
	suite.Contains(err.Error(), "ROL-1003")
}

// Test 7: List Roles
func (suite *RoleAPITestSuite) TestListRoles_Success() {
	suite.Require().NotEmpty(sharedRoleID, "Shared role must be created in SetupSuite")

	response, err := suite.listRoles(0, 30)
	suite.Require().NoError(err)
	suite.Require().NotNil(response)

	suite.GreaterOrEqual(response.TotalResults, 1)
	suite.GreaterOrEqual(response.Count, 1)
	suite.NotEmpty(response.Roles)

	// Verify our shared role is in the list
	found := false
	for _, role := range response.Roles {
		if role.ID == sharedRoleID {
			found = true
			suite.Equal("Test Admin Role", role.Name)
			break
		}
	}
	suite.True(found, "Shared role should be in the list")
}

// Test 8: List Roles - Pagination
func (suite *RoleAPITestSuite) TestListRoles_Pagination() {
	// Create additional roles for pagination testing
	role1Request := CreateRoleRequest{
		Name: "Pagination Test Role 1",
		OUID: testOUID,
		Permissions: []ResourcePermissions{
			{
				ResourceServerID: testResourceServer1ID,
				Permissions:      []string{testPermission1},
			},
		},
	}
	role2Request := CreateRoleRequest{
		Name: "Pagination Test Role 2",
		OUID: testOUID,
		Permissions: []ResourcePermissions{
			{
				ResourceServerID: testResourceServer1ID,
				Permissions:      []string{testPermission2},
			},
		},
	}

	role1, err := suite.createRole(role1Request)
	suite.Require().NoError(err)
	defer suite.deleteRole(role1.ID)

	role2, err := suite.createRole(role2Request)
	suite.Require().NoError(err)
	defer suite.deleteRole(role2.ID)

	// Test pagination with limit
	response, err := suite.listRoles(0, 2)
	suite.Require().NoError(err)
	suite.LessOrEqual(response.Count, 2)

	// Test with offset
	response2, err := suite.listRoles(1, 2)
	suite.Require().NoError(err)
	suite.NotNil(response2)
}

// Test 9: Update Role
func (suite *RoleAPITestSuite) TestUpdateRole_Success() {
	suite.Require().NotEmpty(sharedRoleID, "Shared role must be created in SetupSuite")

	updateRequest := UpdateRoleRequest{
		Name:        "Updated Admin Role",
		Description: "Updated description",
		OUID:        testOUID,
		Permissions: []ResourcePermissions{
			{
				ResourceServerID: testResourceServer1ID,
				Permissions:      []string{testPermission1, testPermission2},
			},
			{
				ResourceServerID: testResourceServer2ID,
				Permissions:      []string{testPermission3},
			},
		},
	}

	role, err := suite.updateRole(sharedRoleID, updateRequest)
	suite.Require().NoError(err)
	suite.Require().NotNil(role)

	suite.Equal(sharedRoleID, role.ID)
	suite.Equal(updateRequest.Name, role.Name)
	suite.Equal(updateRequest.Description, role.Description)
	suite.Equal(2, len(role.Permissions))
}

// Test 10: Update Role - Not Found
func (suite *RoleAPITestSuite) TestUpdateRole_NotFound() {
	updateRequest := UpdateRoleRequest{
		Name: "Updated Role",
		OUID: testOUID,
		Permissions: []ResourcePermissions{
			{
				ResourceServerID: testResourceServer1ID,
				Permissions:      []string{testPermission1},
			},
		},
	}

	role, err := suite.updateRole("nonexistent-role-id", updateRequest)
	suite.Error(err)
	suite.Nil(role)
	suite.Contains(err.Error(), "ROL-1003")
}

// Test 11: Add Assignments - User
func (suite *RoleAPITestSuite) TestAddAssignments_User() {
	// Create a role for this test
	roleRequest := CreateRoleRequest{
		Name: "Test Role for User Assignment",
		OUID: testOUID,
		Permissions: []ResourcePermissions{
			{
				ResourceServerID: testResourceServer1ID,
				Permissions:      []string{testPermission1},
			},
		},
	}
	role, err := suite.createRole(roleRequest)
	suite.Require().NoError(err)
	defer suite.deleteRole(role.ID)

	assignmentsRequest := AssignmentsRequest{
		Assignments: []Assignment{
			{ID: testUserID1, Type: AssigneeTypeUser},
		},
	}

	err = suite.addAssignments(role.ID, assignmentsRequest)
	suite.Require().NoError(err)

	// Verify assignments were added
	assignments, err := suite.getRoleAssignments(role.ID, 0, 30)
	suite.Require().NoError(err)
	suite.Equal(1, assignments.TotalResults)
	suite.Equal(testUserID1, assignments.Assignments[0].ID)
	suite.Equal(AssigneeTypeUser, assignments.Assignments[0].Type)
}

// Test 12: Add Assignments - Group
func (suite *RoleAPITestSuite) TestAddAssignments_Group() {
	// Create a role for this test
	roleRequest := CreateRoleRequest{
		Name: "Test Role for Group Assignment",
		OUID: testOUID,
		Permissions: []ResourcePermissions{
			{
				ResourceServerID: testResourceServer1ID,
				Permissions:      []string{testPermission1},
			},
		},
	}
	role, err := suite.createRole(roleRequest)
	suite.Require().NoError(err)
	defer suite.deleteRole(role.ID)

	assignmentsRequest := AssignmentsRequest{
		Assignments: []Assignment{
			{ID: testGroupID, Type: AssigneeTypeGroup},
		},
	}

	err = suite.addAssignments(role.ID, assignmentsRequest)
	suite.Require().NoError(err)

	// Verify assignments
	assignments, err := suite.getRoleAssignments(role.ID, 0, 30)
	suite.Require().NoError(err)
	suite.Equal(1, assignments.TotalResults) // Group only

	// Check group assignment exists
	groupFound := false
	for _, assignment := range assignments.Assignments {
		if assignment.ID == testGroupID && assignment.Type == AssigneeTypeGroup {
			groupFound = true
			break
		}
	}
	suite.True(groupFound, "Group assignment should exist")
}

// Test 13: Add Assignments - Multiple
func (suite *RoleAPITestSuite) TestAddAssignments_Multiple() {
	// Create a new role for this test
	roleRequest := CreateRoleRequest{
		Name: "Multi Assignment Role",
		OUID: testOUID,
		Permissions: []ResourcePermissions{
			{
				ResourceServerID: testResourceServer1ID,
				Permissions:      []string{testPermission1},
			},
		},
	}
	role, err := suite.createRole(roleRequest)
	suite.Require().NoError(err)
	defer suite.deleteRole(role.ID)

	assignmentsRequest := AssignmentsRequest{
		Assignments: []Assignment{
			{ID: testUserID1, Type: AssigneeTypeUser},
			{ID: testUserID2, Type: AssigneeTypeUser},
			{ID: testGroupID, Type: AssigneeTypeGroup},
		},
	}

	err = suite.addAssignments(role.ID, assignmentsRequest)
	suite.Require().NoError(err)

	// Verify all assignments
	assignments, err := suite.getRoleAssignments(role.ID, 0, 30)
	suite.Require().NoError(err)
	suite.Equal(3, assignments.TotalResults)
}

// Test 14: Add Assignments - Invalid User
func (suite *RoleAPITestSuite) TestAddAssignments_InvalidUser() {
	// Create a role for this test
	roleRequest := CreateRoleRequest{
		Name: "Test Role for Invalid Assignment",
		OUID: testOUID,
		Permissions: []ResourcePermissions{
			{
				ResourceServerID: testResourceServer1ID,
				Permissions:      []string{testPermission1},
			},
		},
	}
	role, err := suite.createRole(roleRequest)
	suite.Require().NoError(err)
	defer suite.deleteRole(role.ID)

	assignmentsRequest := AssignmentsRequest{
		Assignments: []Assignment{
			{ID: "nonexistent-user-id", Type: AssigneeTypeUser},
		},
	}

	err = suite.addAssignments(role.ID, assignmentsRequest)
	suite.Error(err)
	suite.Contains(err.Error(), "ROL-1007")
}

// Test 15: Get Role Assignments
func (suite *RoleAPITestSuite) TestGetRoleAssignments_Success() {
	// Create a role with an assignment for this test
	roleRequest := CreateRoleRequest{
		Name: "Test Role for Get Assignments",
		OUID: testOUID,
		Permissions: []ResourcePermissions{
			{
				ResourceServerID: testResourceServer1ID,
				Permissions:      []string{testPermission1},
			},
		},
		Assignments: []Assignment{
			{ID: testUserID1, Type: AssigneeTypeUser},
		},
	}
	role, err := suite.createRole(roleRequest)
	suite.Require().NoError(err)
	defer suite.deleteRole(role.ID)

	assignments, err := suite.getRoleAssignments(role.ID, 0, 30)
	suite.Require().NoError(err)
	suite.Require().NotNil(assignments)
	suite.GreaterOrEqual(assignments.TotalResults, 0)
}

// Test 16: Get Role Assignments - Pagination
func (suite *RoleAPITestSuite) TestGetRoleAssignments_Pagination() {
	// Create a role with multiple assignments for pagination testing
	roleRequest := CreateRoleRequest{
		Name: "Test Role for Pagination",
		OUID: testOUID,
		Permissions: []ResourcePermissions{
			{
				ResourceServerID: testResourceServer1ID,
				Permissions:      []string{testPermission1},
			},
		},
		Assignments: []Assignment{
			{ID: testUserID1, Type: AssigneeTypeUser},
			{ID: testUserID2, Type: AssigneeTypeUser},
		},
	}
	role, err := suite.createRole(roleRequest)
	suite.Require().NoError(err)
	defer suite.deleteRole(role.ID)

	// Test with small page size
	assignments, err := suite.getRoleAssignments(role.ID, 0, 1)
	suite.Require().NoError(err)
	suite.LessOrEqual(assignments.Count, 1)

	// Test with offset
	if assignments.TotalResults > 1 {
		assignments2, err := suite.getRoleAssignments(role.ID, 1, 1)
		suite.Require().NoError(err)
		suite.NotNil(assignments2)
	}
}

// Test 17: Remove Assignments
func (suite *RoleAPITestSuite) TestRemoveAssignments_Success() {
	// Create a role with assignments for this test
	roleRequest := CreateRoleRequest{
		Name: "Test Role for Remove Assignments",
		OUID: testOUID,
		Permissions: []ResourcePermissions{
			{
				ResourceServerID: testResourceServer1ID,
				Permissions:      []string{testPermission1},
			},
		},
		Assignments: []Assignment{
			{ID: testUserID1, Type: AssigneeTypeUser},
			{ID: testUserID2, Type: AssigneeTypeUser},
		},
	}
	role, err := suite.createRole(roleRequest)
	suite.Require().NoError(err)
	defer suite.deleteRole(role.ID)

	// Get current assignments
	beforeAssignments, err := suite.getRoleAssignments(role.ID, 0, 30)
	suite.Require().NoError(err)
	initialCount := beforeAssignments.TotalResults

	suite.Require().Greater(initialCount, 0, "Should have assignments to remove")

	// Remove first assignment
	assignmentToRemove := beforeAssignments.Assignments[0]
	removeRequest := AssignmentsRequest{
		Assignments: []Assignment{assignmentToRemove},
	}

	err = suite.removeAssignments(role.ID, removeRequest)
	suite.Require().NoError(err)

	// Verify assignment was removed
	afterAssignments, err := suite.getRoleAssignments(role.ID, 0, 30)
	suite.Require().NoError(err)
	suite.Equal(initialCount-1, afterAssignments.TotalResults)
}

// Test 18: Delete Role with Assignments
func (suite *RoleAPITestSuite) TestDeleteRole_WithAssignments() {
	// Create a role with assignments
	roleRequest := CreateRoleRequest{
		Name: "Role to Delete with Assignments",
		OUID: testOUID,
		Permissions: []ResourcePermissions{
			{
				ResourceServerID: testResourceServer1ID,
				Permissions:      []string{testPermission1},
			},
		},
		Assignments: []Assignment{
			{ID: testUserID1, Type: AssigneeTypeUser},
		},
	}
	role, err := suite.createRole(roleRequest)
	suite.Require().NoError(err)

	// Delete should succeed - assignments are cascade deleted automatically
	err = suite.deleteRole(role.ID)
	suite.NoError(err, "Delete should succeed and cascade delete assignments")

	// Verify the role is gone
	_, err = suite.getRole(role.ID)
	suite.Require().Error(err, "Role should no longer exist after deletion")
}

// Test 19: Delete Role - Success
func (suite *RoleAPITestSuite) TestDeleteRole_Success() {
	// Create a role without assignments
	roleRequest := CreateRoleRequest{
		Name: "Role to Delete",
		OUID: testOUID,
		Permissions: []ResourcePermissions{
			{
				ResourceServerID: testResourceServer1ID,
				Permissions:      []string{testPermission1},
			},
		},
	}
	role, err := suite.createRole(roleRequest)
	suite.Require().NoError(err)

	// Delete the role
	err = suite.deleteRole(role.ID)
	suite.NoError(err)

	// Verify role is deleted
	deletedRole, err := suite.getRole(role.ID)
	suite.Error(err)
	suite.Nil(deletedRole)
	suite.Contains(err.Error(), "ROL-1003")
}

// Test 20: Delete Role - Not Found (Should return success for idempotency)
func (suite *RoleAPITestSuite) TestDeleteRole_NotFound() {
	err := suite.deleteRole("nonexistent-role-id")
	// As per service implementation, delete returns nil for non-existent roles
	suite.NoError(err)
}

// Test 21: Get Role Assignments with Display Names
func (suite *RoleAPITestSuite) TestGetRoleAssignments_WithDisplay() {
	// Create a role with both user and group assignments
	roleRequest := CreateRoleRequest{
		Name: "Test Role for Display Names",
		OUID: testOUID,
		Permissions: []ResourcePermissions{
			{
				ResourceServerID: testResourceServer1ID,
				Permissions:      []string{testPermission1},
			},
		},
		Assignments: []Assignment{
			{ID: testUserID1, Type: AssigneeTypeUser},
			{ID: testGroupID, Type: AssigneeTypeGroup},
		},
	}
	role, err := suite.createRole(roleRequest)
	suite.Require().NoError(err)
	defer suite.deleteRole(role.ID)

	// Get assignments without display parameter
	assignmentsWithoutDisplay, err := suite.getRoleAssignmentsWithInclude(role.ID, 0, 30, "")
	suite.Require().NoError(err)
	suite.Require().NotNil(assignmentsWithoutDisplay)
	suite.Equal(2, assignmentsWithoutDisplay.TotalResults)

	// Verify display names are not included
	for _, assignment := range assignmentsWithoutDisplay.Assignments {
		suite.Empty(assignment.Display, "Display field should be empty without include=display parameter")
	}

	// Get assignments with include=display parameter
	assignmentsWithDisplay, err := suite.getRoleAssignmentsWithInclude(role.ID, 0, 30, "display")
	suite.Require().NoError(err)
	suite.Require().NotNil(assignmentsWithDisplay)
	suite.Equal(2, assignmentsWithDisplay.TotalResults)

	// Verify display names are included
	userFound := false
	groupFound := false
	for _, assignment := range assignmentsWithDisplay.Assignments {
		suite.NotEmpty(assignment.Display, "Display field should be populated with include=display parameter")

		if assignment.Type == AssigneeTypeUser && assignment.ID == testUserID1 {
			userFound = true
			// Display name for user should be the user ID (as per implementation)
			suite.Equal(testUserID1, assignment.Display)
		}

		if assignment.Type == AssigneeTypeGroup && assignment.ID == testGroupID {
			groupFound = true
			// Display name for group should be the group name
			suite.Equal(testGroup.Name, assignment.Display)
		}
	}

	suite.True(userFound, "User assignment should be found")
	suite.True(groupFound, "Group assignment should be found")
}

// Test 22: Create Role - Invalid Resource Server ID
func (suite *RoleAPITestSuite) TestCreateRole_InvalidResourceServerID() {
	roleRequest := CreateRoleRequest{
		Name: "Role With Invalid Resource Server",
		OUID: testOUID,
		Permissions: []ResourcePermissions{
			{
				ResourceServerID: "00000000-0000-0000-0000-000000000000",
				Permissions:      []string{"some:permission"},
			},
		},
	}

	role, err := suite.createRole(roleRequest)
	suite.Error(err, "Should fail with invalid resource server ID")
	suite.Nil(role)
	suite.Contains(err.Error(), "ROL-1012", "Should return invalid permissions error")
}

// Test 23: Create Role - Invalid Permissions for Valid Resource Server
func (suite *RoleAPITestSuite) TestCreateRole_InvalidPermissionsForValidResourceServer() {
	roleRequest := CreateRoleRequest{
		Name: "Role With Invalid Permissions",
		OUID: testOUID,
		Permissions: []ResourcePermissions{
			{
				ResourceServerID: testResourceServer1ID,
				Permissions:      []string{"nonexistent:permission"},
			},
		},
	}

	role, err := suite.createRole(roleRequest)
	suite.Error(err, "Should fail with invalid permissions")
	suite.Nil(role)
	suite.Contains(err.Error(), "ROL-1012")
}

// Test 24: Create Role - Empty Permissions Array for Resource Server
func (suite *RoleAPITestSuite) TestCreateRole_EmptyPermissionsArrayForResourceServer() {
	roleRequest := CreateRoleRequest{
		Name: "Role With Empty Permissions Array",
		OUID: testOUID,
		Permissions: []ResourcePermissions{
			{
				ResourceServerID: testResourceServer1ID,
				Permissions:      []string{},
			},
		},
	}

	role, err := suite.createRole(roleRequest)
	suite.Require().NoError(err, "Empty permissions array should be allowed")
	suite.Require().NotNil(role)
	defer suite.deleteRole(role.ID)

	suite.Equal(1, len(role.Permissions))
	suite.Equal(0, len(role.Permissions[0].Permissions))
}

// Test 25: Create Role - Multiple Resource Servers
func (suite *RoleAPITestSuite) TestCreateRole_MultipleResourceServers() {
	roleRequest := CreateRoleRequest{
		Name:        "Multi-Server Role",
		Description: "Role with permissions from multiple resource servers",
		OUID:        testOUID,
		Permissions: []ResourcePermissions{
			{
				ResourceServerID: testResourceServer1ID,
				Permissions:      []string{testPermission1, testPermission2},
			},
			{
				ResourceServerID: testResourceServer2ID,
				Permissions:      []string{testPermission3},
			},
		},
	}

	role, err := suite.createRole(roleRequest)
	suite.Require().NoError(err)
	suite.Require().NotNil(role)
	defer suite.deleteRole(role.ID)

	suite.Equal(2, len(role.Permissions))

	// Verify each resource server
	var foundRS1, foundRS2 bool
	for _, rp := range role.Permissions {
		if rp.ResourceServerID == testResourceServer1ID {
			foundRS1 = true
			suite.Equal(2, len(rp.Permissions))
		}
		if rp.ResourceServerID == testResourceServer2ID {
			foundRS2 = true
			suite.Equal(1, len(rp.Permissions))
		}
	}
	suite.True(foundRS1 && foundRS2, "Should find both resource servers")
}

// Test 26: Create Role - Multiple Resource Servers with One Invalid
func (suite *RoleAPITestSuite) TestCreateRole_MultipleResourceServers_OneInvalid() {
	roleRequest := CreateRoleRequest{
		Name: "Multi-Server Role With Invalid",
		OUID: testOUID,
		Permissions: []ResourcePermissions{
			{
				ResourceServerID: testResourceServer1ID,
				Permissions:      []string{testPermission1},
			},
			{
				ResourceServerID: "invalid-id",
				Permissions:      []string{"some:permission"},
			},
		},
	}

	role, err := suite.createRole(roleRequest)
	suite.Error(err)
	suite.Nil(role)
	suite.Contains(err.Error(), "ROL-1012")
}

// Test 27: Update Role - Invalid Permissions
func (suite *RoleAPITestSuite) TestUpdateRole_InvalidPermissions() {
	// Create valid role first
	roleRequest := CreateRoleRequest{
		Name: "Role to Update with Invalid Permissions",
		OUID: testOUID,
		Permissions: []ResourcePermissions{
			{
				ResourceServerID: testResourceServer1ID,
				Permissions:      []string{testPermission1},
			},
		},
	}
	role, err := suite.createRole(roleRequest)
	suite.Require().NoError(err)
	defer suite.deleteRole(role.ID)

	// Update with invalid permissions
	updateRequest := UpdateRoleRequest{
		Name: "Updated Role Name",
		OUID: testOUID,
		Permissions: []ResourcePermissions{
			{
				ResourceServerID: testResourceServer1ID,
				Permissions:      []string{"invalid:permission"},
			},
		},
	}

	updatedRole, err := suite.updateRole(role.ID, updateRequest)
	suite.Error(err)
	suite.Nil(updatedRole)
	suite.Contains(err.Error(), "ROL-1012")
}

// Test 28: Update Role - Add Second Resource Server
func (suite *RoleAPITestSuite) TestUpdateRole_AddSecondResourceServer() {
	// Create role with one resource server
	roleRequest := CreateRoleRequest{
		Name: "Role to Expand",
		OUID: testOUID,
		Permissions: []ResourcePermissions{
			{
				ResourceServerID: testResourceServer1ID,
				Permissions:      []string{testPermission1},
			},
		},
	}
	role, err := suite.createRole(roleRequest)
	suite.Require().NoError(err)
	defer suite.deleteRole(role.ID)

	// Update to add second resource server
	updateRequest := UpdateRoleRequest{
		Name: "Expanded Role",
		OUID: testOUID,
		Permissions: []ResourcePermissions{
			{
				ResourceServerID: testResourceServer1ID,
				Permissions:      []string{testPermission1, testPermission2},
			},
			{
				ResourceServerID: testResourceServer2ID,
				Permissions:      []string{testPermission3},
			},
		},
	}

	updatedRole, err := suite.updateRole(role.ID, updateRequest)
	suite.Require().NoError(err)
	suite.Require().NotNil(updatedRole)
	suite.Equal(2, len(updatedRole.Permissions))
}

// Test 29: Create Role - Mix of Valid and Invalid Permissions
func (suite *RoleAPITestSuite) TestCreateRole_MixedValidInvalidPermissions() {
	roleRequest := CreateRoleRequest{
		Name: "Role With Mixed Permissions",
		OUID: testOUID,
		Permissions: []ResourcePermissions{
			{
				ResourceServerID: testResourceServer1ID,
				Permissions:      []string{testPermission1, "invalid:permission"},
			},
		},
	}

	role, err := suite.createRole(roleRequest)
	suite.Error(err, "Should fail when any permission is invalid")
	suite.Nil(role)
	suite.Contains(err.Error(), "ROL-1012")
}

// Test 30: Create Role - Missing Resource Server ID
func (suite *RoleAPITestSuite) TestCreateRole_MissingResourceServerID() {
	roleRequest := CreateRoleRequest{
		Name: "Role With Missing Resource Server ID",
		OUID: testOUID,
		Permissions: []ResourcePermissions{
			{
				ResourceServerID: "",
				Permissions:      []string{"some:permission"},
			},
		},
	}

	role, err := suite.createRole(roleRequest)
	suite.Error(err)
	suite.Nil(role)
	// May return ROL-1012 or ROL-1001 depending on validation
}

// Test 31: Get Role Assignments - Filter by Type
func (suite *RoleAPITestSuite) TestGetRoleAssignments_FilterByType() {
	// Create a role with both user and group assignments
	roleRequest := CreateRoleRequest{
		Name: "Test Role for Type Filtering",
		OUID: testOUID,
		Permissions: []ResourcePermissions{
			{
				ResourceServerID: testResourceServer1ID,
				Permissions:      []string{testPermission1},
			},
		},
		Assignments: []Assignment{
			{ID: testUserID1, Type: AssigneeTypeUser},
			{ID: testUserID2, Type: AssigneeTypeUser},
			{ID: testGroupID, Type: AssigneeTypeGroup},
		},
	}
	role, err := suite.createRole(roleRequest)
	suite.Require().NoError(err)
	defer suite.deleteRole(role.ID)

	// Verify no filter returns all assignments
	allAssignments, err := suite.getRoleAssignments(role.ID, 0, 30)
	suite.Require().NoError(err)
	suite.Require().NotNil(allAssignments)
	suite.Equal(3, allAssignments.TotalResults, "Should return all 3 assignments without type filter")
	suite.Equal(3, allAssignments.Count)

	// Filter by user type
	userAssignments, err := suite.getRoleAssignmentsByType(role.ID, 0, 30, "user")
	suite.Require().NoError(err)
	suite.Require().NotNil(userAssignments)
	suite.Equal(2, userAssignments.TotalResults, "Should return 2 user assignments")
	suite.Equal(2, userAssignments.Count)
	for _, assignment := range userAssignments.Assignments {
		suite.Equal(AssigneeTypeUser, assignment.Type, "All assignments should be of type 'user'")
	}

	// Filter by group type
	groupAssignments, err := suite.getRoleAssignmentsByType(role.ID, 0, 30, "group")
	suite.Require().NoError(err)
	suite.Require().NotNil(groupAssignments)
	suite.Equal(1, groupAssignments.TotalResults, "Should return 1 group assignment")
	suite.Equal(1, groupAssignments.Count)
	for _, assignment := range groupAssignments.Assignments {
		suite.Equal(AssigneeTypeGroup, assignment.Type, "All assignments should be of type 'group'")
	}
	suite.Equal(testGroupID, groupAssignments.Assignments[0].ID)
}

// Test 32: Get Role Assignments - Filter by Type with Pagination
func (suite *RoleAPITestSuite) TestGetRoleAssignments_FilterByTypeWithPagination() {
	// Interleave group between users so a "paginate-then-filter" bug is caught.
	// Wrong impl: offset=1,limit=1 on the raw list [user1, group, user2] gives [group] → filter → []
	// Correct impl: filter first → [user1, user2], then offset=1,limit=1 → [user2]
	roleRequest := CreateRoleRequest{
		Name: "Test Role for Type Filter Pagination",
		OUID: testOUID,
		Permissions: []ResourcePermissions{
			{
				ResourceServerID: testResourceServer1ID,
				Permissions:      []string{testPermission1},
			},
		},
		Assignments: []Assignment{
			{ID: testUserID1, Type: AssigneeTypeUser},
			{ID: testGroupID, Type: AssigneeTypeGroup},
			{ID: testUserID2, Type: AssigneeTypeUser},
		},
	}
	role, err := suite.createRole(roleRequest)
	suite.Require().NoError(err)
	defer suite.deleteRole(role.ID)

	// Get first page of user assignments (limit=1)
	page1, err := suite.getRoleAssignmentsByType(role.ID, 0, 1, "user")
	suite.Require().NoError(err)
	suite.Require().NotNil(page1)
	suite.Equal(2, page1.TotalResults, "TotalResults should reflect filtered count")
	suite.Require().Equal(1, page1.Count, "Should return only 1 assignment per page")

	// Get second page — must return a different user than page 1
	page2, err := suite.getRoleAssignmentsByType(role.ID, 1, 1, "user")
	suite.Require().NoError(err)
	suite.Require().NotNil(page2)
	suite.Equal(2, page2.TotalResults, "TotalResults should still be 2")
	suite.Require().Equal(1, page2.Count, "Should return 1 assignment on second page")
	suite.NotEqual(page1.Assignments[0].ID, page2.Assignments[0].ID,
		"Page 1 and page 2 must return different user assignments")
}

// Test 33: Get Role Assignments - Invalid Type Parameter
func (suite *RoleAPITestSuite) TestGetRoleAssignments_InvalidType() {
	// Create a role
	roleRequest := CreateRoleRequest{
		Name: "Test Role for Invalid Type",
		OUID: testOUID,
		Permissions: []ResourcePermissions{
			{
				ResourceServerID: testResourceServer1ID,
				Permissions:      []string{testPermission1},
			},
		},
	}
	role, err := suite.createRole(roleRequest)
	suite.Require().NoError(err)
	defer suite.deleteRole(role.ID)

	// Request with invalid type should return error
	_, err = suite.getRoleAssignmentsByType(role.ID, 0, 30, "invalid")
	suite.Require().Error(err)
	suite.Contains(err.Error(), "ROL-1016", "Should return invalid assignee type error")
}

// Test 34: Create Role with App Assignment
func (suite *RoleAPITestSuite) TestCreateRole_WithAppAssignment() {
	roleRequest := CreateRoleRequest{
		Name: "Test Role With App Assignment",
		OUID: testOUID,
		Permissions: []ResourcePermissions{
			{
				ResourceServerID: testResourceServer1ID,
				Permissions:      []string{testPermission1},
			},
		},
		Assignments: []Assignment{
			{ID: testAppID, Type: AssigneeTypeApp},
		},
	}

	role, err := suite.createRole(roleRequest)
	suite.Require().NoError(err)
	suite.Require().NotNil(role)
	defer suite.deleteRole(role.ID)

	suite.Equal(1, len(role.Assignments))
	suite.Equal(testAppID, role.Assignments[0].ID)
	suite.Equal(AssigneeTypeApp, role.Assignments[0].Type)
}

// Test 35: Add App Assignment to Role
func (suite *RoleAPITestSuite) TestAddAssignments_App() {
	roleRequest := CreateRoleRequest{
		Name: "Test Role for App Assignment",
		OUID: testOUID,
		Permissions: []ResourcePermissions{
			{
				ResourceServerID: testResourceServer1ID,
				Permissions:      []string{testPermission1},
			},
		},
	}
	role, err := suite.createRole(roleRequest)
	suite.Require().NoError(err)
	defer suite.deleteRole(role.ID)

	assignmentsRequest := AssignmentsRequest{
		Assignments: []Assignment{
			{ID: testAppID, Type: AssigneeTypeApp},
		},
	}

	err = suite.addAssignments(role.ID, assignmentsRequest)
	suite.Require().NoError(err)

	// Verify assignments were added
	assignments, err := suite.getRoleAssignments(role.ID, 0, 30)
	suite.Require().NoError(err)
	suite.Equal(1, assignments.TotalResults)
	suite.Equal(testAppID, assignments.Assignments[0].ID)
	suite.Equal(AssigneeTypeApp, assignments.Assignments[0].Type)
}

// Test 36: Mixed User, Group, and App Assignments
func (suite *RoleAPITestSuite) TestAddAssignments_MixedUserGroupApp() {
	roleRequest := CreateRoleRequest{
		Name: "Mixed Assignment Role",
		OUID: testOUID,
		Permissions: []ResourcePermissions{
			{
				ResourceServerID: testResourceServer1ID,
				Permissions:      []string{testPermission1},
			},
		},
	}
	role, err := suite.createRole(roleRequest)
	suite.Require().NoError(err)
	defer suite.deleteRole(role.ID)

	assignmentsRequest := AssignmentsRequest{
		Assignments: []Assignment{
			{ID: testUserID1, Type: AssigneeTypeUser},
			{ID: testGroupID, Type: AssigneeTypeGroup},
			{ID: testAppID, Type: AssigneeTypeApp},
		},
	}

	err = suite.addAssignments(role.ID, assignmentsRequest)
	suite.Require().NoError(err)

	// Verify all assignments
	assignments, err := suite.getRoleAssignments(role.ID, 0, 30)
	suite.Require().NoError(err)
	suite.Equal(3, assignments.TotalResults)

	// Verify each type exists
	typeFound := map[AssigneeType]bool{}
	for _, a := range assignments.Assignments {
		typeFound[a.Type] = true
	}
	suite.True(typeFound[AssigneeTypeUser], "User assignment should exist")
	suite.True(typeFound[AssigneeTypeGroup], "Group assignment should exist")
	suite.True(typeFound[AssigneeTypeApp], "App assignment should exist")
}

// Test 37: Filter Assignments by App Type
func (suite *RoleAPITestSuite) TestGetRoleAssignments_FilterByAppType() {
	roleRequest := CreateRoleRequest{
		Name: "Test Role for App Type Filter",
		OUID: testOUID,
		Permissions: []ResourcePermissions{
			{
				ResourceServerID: testResourceServer1ID,
				Permissions:      []string{testPermission1},
			},
		},
		Assignments: []Assignment{
			{ID: testUserID1, Type: AssigneeTypeUser},
			{ID: testAppID, Type: AssigneeTypeApp},
		},
	}
	role, err := suite.createRole(roleRequest)
	suite.Require().NoError(err)
	defer suite.deleteRole(role.ID)

	// Filter by app type
	appAssignments, err := suite.getRoleAssignmentsByType(role.ID, 0, 30, "app")
	suite.Require().NoError(err)
	suite.Equal(1, appAssignments.TotalResults)
	suite.Equal(AssigneeTypeApp, appAssignments.Assignments[0].Type)
	suite.Equal(testAppID, appAssignments.Assignments[0].ID)
}

// Test 38: Remove App Assignment
func (suite *RoleAPITestSuite) TestRemoveAssignments_App() {
	roleRequest := CreateRoleRequest{
		Name: "Test Role for App Removal",
		OUID: testOUID,
		Permissions: []ResourcePermissions{
			{
				ResourceServerID: testResourceServer1ID,
				Permissions:      []string{testPermission1},
			},
		},
		Assignments: []Assignment{
			{ID: testAppID, Type: AssigneeTypeApp},
		},
	}
	role, err := suite.createRole(roleRequest)
	suite.Require().NoError(err)
	defer suite.deleteRole(role.ID)

	// Verify the assignment exists
	assignments, err := suite.getRoleAssignments(role.ID, 0, 30)
	suite.Require().NoError(err)
	suite.Equal(1, assignments.TotalResults)

	// Remove the app assignment
	err = suite.removeAssignments(role.ID, AssignmentsRequest{
		Assignments: []Assignment{
			{ID: testAppID, Type: AssigneeTypeApp},
		},
	})
	suite.Require().NoError(err)

	// Verify it was removed
	assignments, err = suite.getRoleAssignments(role.ID, 0, 30)
	suite.Require().NoError(err)
	suite.Equal(0, assignments.TotalResults)
}

// Test 39: Add App Assignment with Invalid App ID
func (suite *RoleAPITestSuite) TestAddAssignments_InvalidApp() {
	roleRequest := CreateRoleRequest{
		Name: "Test Role for Invalid App Assignment",
		OUID: testOUID,
		Permissions: []ResourcePermissions{
			{
				ResourceServerID: testResourceServer1ID,
				Permissions:      []string{testPermission1},
			},
		},
	}
	role, err := suite.createRole(roleRequest)
	suite.Require().NoError(err)
	defer suite.deleteRole(role.ID)

	assignmentsRequest := AssignmentsRequest{
		Assignments: []Assignment{
			{ID: "nonexistent-app-id", Type: AssigneeTypeApp},
		},
	}

	err = suite.addAssignments(role.ID, assignmentsRequest)
	suite.Error(err)
	suite.Contains(err.Error(), "ROL-1007")
}

// Test 20: Add Assignment to Declarative Role
func (suite *RoleAPITestSuite) TestAddAssignments_DeclarativeRole() {
	// The declarative role 'decl-role-1' is loaded from the file store.
	// Create a user via API, assign them to the declarative role, then verify and clean up.
	const declRoleID = "decl-role-1"
	const declOUID = "decl-ou-1"

	// Step 1: Verify the declarative role is accessible via the API.
	declRole, err := suite.getRole(declRoleID)
	suite.Require().NoError(err, "Declarative role should be accessible via API")
	suite.Require().NotNil(declRole)
	suite.Equal(declRoleID, declRole.ID)

	// Step 2: Create a user in the declarative OU via API.
	user := testutils.User{
		OUID: declOUID,
		Type: "Declarative Test Schema",
		Attributes: json.RawMessage(`{
			"email": "decl-role-assign-user@example.com",
			"username": "declroleassignuser"
		}`),
	}
	userID, err := testutils.CreateUser(user)
	suite.Require().NoError(err, "Failed to create user for declarative role assignment test")
	defer testutils.DeleteUser(userID)

	// Step 3: Assign the user to the declarative role via API.
	assignmentsRequest := AssignmentsRequest{
		Assignments: []Assignment{
			{ID: userID, Type: AssigneeTypeUser},
		},
	}
	err = suite.addAssignments(declRoleID, assignmentsRequest)
	suite.Require().NoError(err, "Should be able to assign a user to a declarative role")
	defer func() {
		_ = suite.removeAssignments(declRoleID, assignmentsRequest)
	}()

	// Step 4: Verify the assignment appears in the role's assignment list.
	assignments, err := suite.getRoleAssignments(declRoleID, 0, 10)
	suite.Require().NoError(err)
	suite.Require().NotNil(assignments)

	var found bool
	for _, a := range assignments.Assignments {
		if a.ID == userID {
			found = true
			break
		}
	}
	suite.True(found, "Assigned user should appear in the declarative role's assignment list")
}

// Helper methods

func (suite *RoleAPITestSuite) createRole(request CreateRoleRequest) (*Role, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", testServerURL+rolesBasePath, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated {
		var errResp ErrorResponse
		json.Unmarshal(respBody, &errResp)
		return nil, fmt.Errorf("failed to create role: %s - %s", errResp.Code, errResp.Message)
	}

	var role Role
	if err := json.Unmarshal(respBody, &role); err != nil {
		return nil, err
	}

	return &role, nil
}

func (suite *RoleAPITestSuite) getRole(roleID string) (*Role, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s%s/%s", testServerURL, rolesBasePath, roleID), nil)
	if err != nil {
		return nil, err
	}

	resp, err := suite.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		json.Unmarshal(respBody, &errResp)
		return nil, fmt.Errorf("failed to get role: %s - %s", errResp.Code, errResp.Message)
	}

	var role Role
	if err := json.Unmarshal(respBody, &role); err != nil {
		return nil, err
	}

	return &role, nil
}

func (suite *RoleAPITestSuite) listRoles(offset, limit int) (*RoleListResponse, error) {
	url := fmt.Sprintf("%s%s?offset=%d&limit=%d", testServerURL, rolesBasePath, offset, limit)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := suite.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		json.Unmarshal(respBody, &errResp)
		return nil, fmt.Errorf("failed to list roles: %s - %s", errResp.Code, errResp.Message)
	}

	var response RoleListResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

func (suite *RoleAPITestSuite) updateRole(roleID string, request UpdateRoleRequest) (*Role, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PUT", fmt.Sprintf("%s%s/%s", testServerURL, rolesBasePath, roleID),
		bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		json.Unmarshal(respBody, &errResp)
		return nil, fmt.Errorf("failed to update role: %s - %s", errResp.Code, errResp.Message)
	}

	var role Role
	if err := json.Unmarshal(respBody, &role); err != nil {
		return nil, err
	}

	return &role, nil
}

func (suite *RoleAPITestSuite) deleteRole(roleID string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s%s/%s", testServerURL, rolesBasePath, roleID), nil)
	if err != nil {
		return err
	}

	resp, err := suite.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		var errResp ErrorResponse
		json.Unmarshal(respBody, &errResp)
		return fmt.Errorf("failed to delete role: %s - %s", errResp.Code, errResp.Message)
	}

	return nil
}

func (suite *RoleAPITestSuite) addAssignments(roleID string, request AssignmentsRequest) error {
	body, err := json.Marshal(request)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s%s/%s/assignments/add", testServerURL, rolesBasePath, roleID),
		bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		var errResp ErrorResponse
		json.Unmarshal(respBody, &errResp)
		return fmt.Errorf("failed to add assignments: %s - %s", errResp.Code, errResp.Message)
	}

	return nil
}

func (suite *RoleAPITestSuite) removeAssignments(roleID string, request AssignmentsRequest) error {
	body, err := json.Marshal(request)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s%s/%s/assignments/remove", testServerURL, rolesBasePath, roleID),
		bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		var errResp ErrorResponse
		json.Unmarshal(respBody, &errResp)
		return fmt.Errorf("failed to remove assignments: %s - %s", errResp.Code, errResp.Message)
	}

	return nil
}

func (suite *RoleAPITestSuite) getRoleAssignments(roleID string, offset, limit int) (*AssignmentListResponse, error) {
	return suite.getRoleAssignmentsWithInclude(roleID, offset, limit, "")
}

func (suite *RoleAPITestSuite) getRoleAssignmentsWithInclude(roleID string, offset, limit int,
	include string) (*AssignmentListResponse, error) {
	return suite.getRoleAssignmentsWithParams(roleID, offset, limit, include, "")
}

func (suite *RoleAPITestSuite) getRoleAssignmentsByType(roleID string, offset, limit int,
	assigneeType string) (*AssignmentListResponse, error) {
	return suite.getRoleAssignmentsWithParams(roleID, offset, limit, "", assigneeType)
}

func (suite *RoleAPITestSuite) getRoleAssignmentsWithParams(roleID string, offset, limit int,
	include, assigneeType string) (*AssignmentListResponse, error) {
	url := fmt.Sprintf("%s%s/%s/assignments?offset=%d&limit=%d", testServerURL, rolesBasePath, roleID, offset, limit)
	if include != "" {
		url = fmt.Sprintf("%s&include=%s", url, include)
	}
	if assigneeType != "" {
		url = fmt.Sprintf("%s&type=%s", url, assigneeType)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := suite.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		json.Unmarshal(respBody, &errResp)
		return nil, fmt.Errorf("failed to get role assignments: %s - %s", errResp.Code, errResp.Message)
	}

	var response AssignmentListResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, err
	}

	return &response, nil
}
