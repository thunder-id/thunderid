// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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

	// testAgentName is the display name of the agent assigned in the agent-type tests.
	testAgentName = "Role Test Agent"
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
	testAgentID  string
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

	// Create test agent (agent entity), using the bootstrapped `default` agent type
	agentID, err := testutils.CreateAgent(testutils.Agent{
		Type:        "default",
		Name:        testAgentName,
		Description: "Agent for role assignment testing",
		OUID:        testOUID,
	})
	suite.Require().NoError(err, "Failed to create test agent")
	testAgentID = agentID

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
	if testAgentID != "" {
		_ = testutils.DeleteAgent(testAgentID)
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
			expectedErr: "INVALID_INPUT_METADATA",
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
			expectedErr: "INVALID_INPUT_METADATA",
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

// TestListRoles_PaginationIsStableAcrossPages walks the role list one page at a time and checks that
// every role is returned exactly once. The list is ordered by CREATED_AT, and on SQLite the column
// defaults to datetime('now'), which only has second precision, so roles created in the same second
// share a timestamp. Without a unique tiebreaker in the ORDER BY the order of those rows is not
// stable between the separate queries that serve each page, so a row can come back on two pages
// while another is never returned at all.
func (suite *RoleAPITestSuite) TestListRoles_PaginationIsStableAcrossPages() {
	const roleCount = 6

	createdIDs := make(map[string]bool, roleCount)
	for i := 0; i < roleCount; i++ {
		role, err := suite.createRole(CreateRoleRequest{
			Name: fmt.Sprintf("Stable Pagination Role %d", i),
			OUID: testOUID,
			Permissions: []ResourcePermissions{
				{
					ResourceServerID: testResourceServer1ID,
					Permissions:      []string{testPermission1},
				},
			},
		})
		suite.Require().NoError(err, "failed to create role %d", i)
		defer suite.deleteRole(role.ID)
		createdIDs[role.ID] = true
	}

	first, err := suite.listRoles(0, 1)
	suite.Require().NoError(err)
	total := first.TotalResults
	suite.Require().GreaterOrEqual(total, roleCount, "the roles just created should be counted")

	// Page through the whole list one row at a time, which is the case that exposes the instability:
	// each page is its own query, so any reordering between them duplicates or drops a row.
	seen := make(map[string]int, total)
	for offset := 0; offset < total; offset++ {
		page, listErr := suite.listRoles(offset, 1)
		suite.Require().NoError(listErr, "failed to read page at offset %d", offset)
		suite.Require().Len(page.Roles, 1, "offset %d within totalResults should return a role", offset)
		seen[page.Roles[0].ID]++
	}

	for id, count := range seen {
		suite.Equalf(1, count, "role %s was returned on %d pages, so paging is not stable", id, count)
	}
	suite.Lenf(seen, total, "paging returned %d distinct roles but totalResults is %d", len(seen), total)

	for id := range createdIDs {
		suite.Containsf(seen, id, "role %s was never returned while paging the full list", id)
	}
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

// Deleting an action that a role grants must remove that permission from the role, rather than
// leaving a reference that can no longer be resolved. Refs #4806.
func (suite *RoleAPITestSuite) TestDeleteAction_CascadesToRolePermissions() {
	rs := testutils.ResourceServer{
		Name:        "Cascade Action Test System",
		Description: "Resource server for action cascade testing",
		Identifier:  "cascade-action-test-system",
		OUID:        testOUID,
	}
	rsID, err := testutils.CreateResourceServerWithActions(rs, nil)
	suite.Require().NoError(err)
	defer func() { _ = testutils.DeleteResourceServer(rsID) }()

	_, err = testutils.CreateAction(rsID, testutils.Action{
		Name:   "Kept Action",
		Handle: "kept",
	})
	suite.Require().NoError(err)
	deletedActionID, err := testutils.CreateAction(rsID, testutils.Action{
		Name:   "Deleted Action",
		Handle: "deleted",
	})
	suite.Require().NoError(err)

	role, err := suite.createRole(CreateRoleRequest{
		Name:        "Cascade Action Role",
		Description: "Role holding a permission that gets deleted",
		OUID:        testOUID,
		Permissions: []ResourcePermissions{
			{ResourceServerID: rsID, Permissions: []string{"kept", "deleted"}},
		},
	})
	suite.Require().NoError(err)
	defer func() { _ = suite.deleteRole(role.ID) }()

	suite.Require().NoError(testutils.DeleteAction(rsID, deletedActionID))

	// The deleted action's permission must be gone, the surviving one untouched.
	updated, err := suite.getRole(role.ID)
	suite.Require().NoError(err)
	suite.Require().Len(updated.Permissions, 1)
	suite.Equal(rsID, updated.Permissions[0].ResourceServerID)
	suite.Equal([]string{"kept"}, updated.Permissions[0].Permissions)

	// The role must remain editable. The Console edits a role by sending back the permissions it
	// read, so re-sending the fetched set must not trip permission validation.
	_, err = suite.updateRole(role.ID, UpdateRoleRequest{
		Name:        "Cascade Action Role Updated",
		Description: "Role updated after its permission was deleted",
		OUID:        testOUID,
		Permissions: updated.Permissions,
	})
	suite.NoError(err, "Role permissions should be updatable after an assigned action is deleted")
}

// Deleting a resource server must remove every role permission scoped to it, so that viewing the
// role afterwards does not fail to resolve the missing resource server. Refs #4806.
func (suite *RoleAPITestSuite) TestDeleteResourceServer_CascadesToRolePermissions() {
	rs := testutils.ResourceServer{
		Name:        "Cascade Server Test System",
		Description: "Resource server for server cascade testing",
		Identifier:  "cascade-server-test-system",
		OUID:        testOUID,
	}
	rsID, err := testutils.CreateResourceServerWithActions(rs, []testutils.Action{
		{Name: "Doomed Action", Handle: "doomed"},
	})
	suite.Require().NoError(err)

	role, err := suite.createRole(CreateRoleRequest{
		Name:        "Cascade Server Role",
		Description: "Role scoped to a resource server that gets deleted",
		OUID:        testOUID,
		Permissions: []ResourcePermissions{
			{ResourceServerID: rsID, Permissions: []string{"doomed"}},
			{ResourceServerID: testResourceServer2ID, Permissions: []string{testPermission3}},
		},
	})
	suite.Require().NoError(err)
	defer func() { _ = suite.deleteRole(role.ID) }()

	// Actions must go before the resource server, matching the reported reproduction steps.
	actions, err := testutils.GetActionsByResourceServer(rsID)
	suite.Require().NoError(err)
	for _, actionID := range actions {
		suite.Require().NoError(testutils.DeleteAction(rsID, actionID))
	}
	suite.Require().NoError(testutils.DeleteResourceServer(rsID))

	// Viewing the role must succeed and retain only the surviving resource server's permissions.
	updated, err := suite.getRole(role.ID)
	suite.Require().NoError(err, "Role should be viewable after its resource server is deleted")
	suite.Require().Len(updated.Permissions, 1)
	suite.Equal(testResourceServer2ID, updated.Permissions[0].ResourceServerID)
	suite.Equal([]string{testPermission3}, updated.Permissions[0].Permissions)
}

// Names and descriptions must survive a create -> read -> update cycle byte for byte. The Console
// echoes the returned name back on every save, so any encoding applied on input compounds per save.
func (suite *RoleAPITestSuite) TestRoleRoundTrip_SpecialCharactersPreserved() {
	const name = "Dean's Sub team"
	const description = `R&D <team> "core"`

	role, err := suite.createRole(CreateRoleRequest{
		Name:        name,
		Description: description,
		OUID:        testOUID,
		Permissions: []ResourcePermissions{
			{
				ResourceServerID: testResourceServer1ID,
				Permissions:      []string{testPermission1},
			},
		},
	})
	suite.Require().NoError(err)
	suite.Require().NotNil(role)
	defer func() { _ = suite.deleteRole(role.ID) }()

	suite.Equal(name, role.Name, "create response must echo the name verbatim")
	suite.Equal(description, role.Description, "create response must echo the description verbatim")

	fetched, err := suite.getRole(role.ID)
	suite.Require().NoError(err)
	suite.Equal(name, fetched.Name, "stored name must match what was sent")
	suite.Equal(description, fetched.Description, "stored description must match what was sent")

	// Replay the Console edit loop: echo the returned name back, change only the description.
	for i := 0; i < 3; i++ {
		updated, updateErr := suite.updateRole(role.ID, UpdateRoleRequest{
			Name:        fetched.Name,
			Description: description,
			OUID:        testOUID,
			Permissions: []ResourcePermissions{
				{
					ResourceServerID: testResourceServer1ID,
					Permissions:      []string{testPermission1},
				},
			},
		})
		suite.Require().NoError(updateErr)
		suite.Equal(name, updated.Name, "name must not drift on save %d", i+1)

		fetched, err = suite.getRole(role.ID)
		suite.Require().NoError(err)
		suite.Equal(name, fetched.Name, "stored name must not drift after save %d", i+1)
		suite.Equal(description, fetched.Description, "stored description must not drift after save %d", i+1)
	}
}

// Create Role - Name Conflict within the same organization unit
func (suite *RoleAPITestSuite) TestCreateRole_NameConflict() {
	roleRequest := CreateRoleRequest{
		Name: "Duplicate Name Role",
		OUID: testOUID,
		Permissions: []ResourcePermissions{
			{ResourceServerID: testResourceServer1ID, Permissions: []string{testPermission1}},
		},
	}

	role, err := suite.createRole(roleRequest)
	suite.Require().NoError(err)
	defer suite.deleteRole(role.ID)

	resp := suite.doRoleRequest(http.MethodPost, rolesBasePath, mustMarshal(suite.T(), roleRequest))
	defer resp.Body.Close()

	suite.Equal(http.StatusConflict, resp.StatusCode, "a duplicate role name must be rejected as a conflict")
	suite.Equal("ROL-1004", decodeErrorCode(suite.T(), resp))
}

// Update Role - Name Conflict with a sibling role in the same organization unit
func (suite *RoleAPITestSuite) TestUpdateRole_NameConflict() {
	existing, err := suite.createRole(CreateRoleRequest{
		Name: "Update Conflict Occupant",
		OUID: testOUID,
		Permissions: []ResourcePermissions{
			{ResourceServerID: testResourceServer1ID, Permissions: []string{testPermission1}},
		},
	})
	suite.Require().NoError(err)
	defer suite.deleteRole(existing.ID)

	target, err := suite.createRole(CreateRoleRequest{
		Name: "Update Conflict Target",
		OUID: testOUID,
		Permissions: []ResourcePermissions{
			{ResourceServerID: testResourceServer1ID, Permissions: []string{testPermission1}},
		},
	})
	suite.Require().NoError(err)
	defer suite.deleteRole(target.ID)

	updateRequest := UpdateRoleRequest{
		Name: existing.Name,
		OUID: testOUID,
		Permissions: []ResourcePermissions{
			{ResourceServerID: testResourceServer1ID, Permissions: []string{testPermission1}},
		},
	}

	resp := suite.doRoleRequest(http.MethodPut, rolesBasePath+"/"+target.ID, mustMarshal(suite.T(), updateRequest))
	defer resp.Body.Close()

	suite.Equal(http.StatusConflict, resp.StatusCode, "renaming onto a sibling's name must be rejected")
	suite.Equal("ROL-1004", decodeErrorCode(suite.T(), resp))
}

// Update Role - Organization Unit Not Found
func (suite *RoleAPITestSuite) TestUpdateRole_NonExistentOU() {
	role, err := suite.createRole(CreateRoleRequest{
		Name: "Role Moved To Missing OU",
		OUID: testOUID,
		Permissions: []ResourcePermissions{
			{ResourceServerID: testResourceServer1ID, Permissions: []string{testPermission1}},
		},
	})
	suite.Require().NoError(err)
	defer suite.deleteRole(role.ID)

	updated, err := suite.updateRole(role.ID, UpdateRoleRequest{
		Name:        "Role Moved To Missing OU",
		OUID:        "nonexistent-ou",
		Permissions: []ResourcePermissions{},
	})
	suite.Error(err)
	suite.Nil(updated)
	suite.Contains(err.Error(), "ROL-1005")
}

// Malformed JSON bodies are rejected on every write endpoint.
func (suite *RoleAPITestSuite) TestWriteRequests_MalformedJSON() {
	role, err := suite.createRole(CreateRoleRequest{
		Name: "Role For Malformed Bodies",
		OUID: testOUID,
		Permissions: []ResourcePermissions{
			{ResourceServerID: testResourceServer1ID, Permissions: []string{testPermission1}},
		},
	})
	suite.Require().NoError(err)
	defer suite.deleteRole(role.ID)

	testCases := []struct {
		name   string
		method string
		path   string
	}{
		{"Create", http.MethodPost, rolesBasePath},
		{"Update", http.MethodPut, rolesBasePath + "/" + role.ID},
		{"AddAssignments", http.MethodPost, rolesBasePath + "/" + role.ID + "/assignments/add"},
		{"RemoveAssignments", http.MethodPost, rolesBasePath + "/" + role.ID + "/assignments/remove"},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			resp := suite.doRoleRequest(tc.method, tc.path, []byte("{not json"))
			defer resp.Body.Close()

			suite.Equal(http.StatusBadRequest, resp.StatusCode, "a malformed body must be rejected")
			suite.Equal("ROL-1001", decodeErrorCode(suite.T(), resp))
		})
	}
}

// A structurally valid update body that violates the field constraints is reported as a
// field-level validation failure rather than a generic malformed-body error.
func (suite *RoleAPITestSuite) TestUpdateRole_ValidationErrors() {
	role, err := suite.createRole(CreateRoleRequest{
		Name: "Role For Update Validation",
		OUID: testOUID,
		Permissions: []ResourcePermissions{
			{ResourceServerID: testResourceServer1ID, Permissions: []string{testPermission1}},
		},
	})
	suite.Require().NoError(err)
	defer suite.deleteRole(role.ID)

	for _, tc := range []struct {
		name string
		body UpdateRoleRequest
	}{
		{"MissingName", UpdateRoleRequest{OUID: testOUID, Permissions: []ResourcePermissions{}}},
		{"MissingOUID", UpdateRoleRequest{Name: "Named", Permissions: []ResourcePermissions{}}},
	} {
		suite.Run(tc.name, func() {
			resp := suite.doRoleRequest(http.MethodPut, rolesBasePath+"/"+role.ID, mustMarshal(suite.T(), tc.body))
			defer resp.Body.Close()

			suite.Equal(http.StatusBadRequest, resp.StatusCode)
			suite.Equal("INVALID_INPUT_METADATA", decodeErrorCode(suite.T(), resp))
		})
	}
}

// Every role route answers a CORS preflight, so the Console can call them from a browser.
func (suite *RoleAPITestSuite) TestRoleRoutes_CORSPreflight() {
	suite.Require().NotEmpty(sharedRoleID, "Shared role must be created in SetupSuite")

	for _, path := range []string{
		rolesBasePath,
		rolesBasePath + "/" + sharedRoleID,
		rolesBasePath + "/" + sharedRoleID + "/assignments",
		rolesBasePath + "/" + sharedRoleID + "/assignments/add",
		rolesBasePath + "/" + sharedRoleID + "/assignments/remove",
	} {
		resp := suite.doRoleRequest(http.MethodOptions, path, nil)
		resp.Body.Close()
		suite.Equal(http.StatusNoContent, resp.StatusCode, "OPTIONS %s must be answered", path)
	}
}

// Assignment requests carrying no assignments are rejected.
func (suite *RoleAPITestSuite) TestAssignments_EmptyList() {
	role, err := suite.createRole(CreateRoleRequest{
		Name: "Role For Empty Assignment Lists",
		OUID: testOUID,
		Permissions: []ResourcePermissions{
			{ResourceServerID: testResourceServer1ID, Permissions: []string{testPermission1}},
		},
	})
	suite.Require().NoError(err)
	defer suite.deleteRole(role.ID)

	for _, action := range []string{"add", "remove"} {
		suite.Run(action, func() {
			body := mustMarshal(suite.T(), AssignmentsRequest{Assignments: []Assignment{}})
			resp := suite.doRoleRequest(http.MethodPost,
				rolesBasePath+"/"+role.ID+"/assignments/"+action, body)
			defer resp.Body.Close()

			suite.Equal(http.StatusBadRequest, resp.StatusCode, "an empty assignment list must be rejected")
			suite.Equal("ROL-1001", decodeErrorCode(suite.T(), resp))
		})
	}
}

// Add Assignments - claimed type does not match the assignee's actual category
func (suite *RoleAPITestSuite) TestAddAssignments_TypeMismatch() {
	role, err := suite.createRole(CreateRoleRequest{
		Name: "Role For Type Mismatch",
		OUID: testOUID,
		Permissions: []ResourcePermissions{
			{ResourceServerID: testResourceServer1ID, Permissions: []string{testPermission1}},
		},
	})
	suite.Require().NoError(err)
	defer suite.deleteRole(role.ID)

	// testUserID1 is a user; claiming it is an app must be rejected rather than silently stored.
	err = suite.addAssignments(role.ID, AssignmentsRequest{
		Assignments: []Assignment{{ID: testUserID1, Type: AssigneeTypeApp}},
	})
	suite.Error(err)
	suite.Contains(err.Error(), "ROL-1007")
}

// Add Assignments - the same ID claimed under two different types in one request
func (suite *RoleAPITestSuite) TestAddAssignments_ConflictingTypesForSameID() {
	role, err := suite.createRole(CreateRoleRequest{
		Name: "Role For Conflicting Assignment Types",
		OUID: testOUID,
		Permissions: []ResourcePermissions{
			{ResourceServerID: testResourceServer1ID, Permissions: []string{testPermission1}},
		},
	})
	suite.Require().NoError(err)
	defer suite.deleteRole(role.ID)

	err = suite.addAssignments(role.ID, AssignmentsRequest{
		Assignments: []Assignment{
			{ID: testUserID1, Type: AssigneeTypeUser},
			{ID: testUserID1, Type: AssigneeTypeApp},
		},
	})
	suite.Error(err)
	suite.Contains(err.Error(), "ROL-1007")
}

// Unknown sub-resources under a role are not routed.
func (suite *RoleAPITestSuite) TestGetRole_UnknownSubPath() {
	suite.Require().NotEmpty(sharedRoleID, "Shared role must be created in SetupSuite")

	for _, path := range []string{
		rolesBasePath + "/" + sharedRoleID + "/unknown",
		rolesBasePath + "/" + sharedRoleID + "/assignments/extra",
	} {
		resp := suite.doRoleRequest(http.MethodGet, path, nil)
		resp.Body.Close()
		suite.Equal(http.StatusNotFound, resp.StatusCode, "unknown sub-resource %q must not be routed", path)
	}
}

// Requests that omit the role id, or name a role that does not exist, are reported
// against the role rather than surfacing as a routing failure.
func (suite *RoleAPITestSuite) TestRoleRequests_MissingAndUnknownRole() {
	resp := suite.doRoleRequest(http.MethodGet, rolesBasePath+"/", nil)
	suite.Equal(http.StatusBadRequest, resp.StatusCode, "a request without a role id must be rejected")
	suite.Equal("ROL-1002", decodeErrorCode(suite.T(), resp))
	resp.Body.Close()

	const unknownRoleID = "nonexistent-role-for-assignments"
	assignments := mustMarshal(suite.T(), AssignmentsRequest{
		Assignments: []Assignment{{ID: testUserID1, Type: AssigneeTypeUser}},
	})

	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   []byte
	}{
		{"GetAssignments", http.MethodGet, "/assignments", nil},
		{"AddAssignments", http.MethodPost, "/assignments/add", assignments},
		{"RemoveAssignments", http.MethodPost, "/assignments/remove", assignments},
	} {
		suite.Run(tc.name, func() {
			resp := suite.doRoleRequest(tc.method, rolesBasePath+"/"+unknownRoleID+tc.path, tc.body)
			defer resp.Body.Close()

			suite.Equal(http.StatusNotFound, resp.StatusCode, "%s on an unknown role must report the role missing", tc.name)
			suite.Equal("ROL-1003", decodeErrorCode(suite.T(), resp))
		})
	}
}

// Pagination parameters are validated on both listing endpoints.
func (suite *RoleAPITestSuite) TestPagination_InvalidParameters() {
	suite.Require().NotEmpty(sharedRoleID, "Shared role must be created in SetupSuite")

	testCases := []struct {
		name         string
		query        string
		expectedCode string
	}{
		{"NonNumericLimit", "?limit=abc", "ROL-1008"},
		{"NonNumericOffset", "?offset=abc", "ROL-1009"},
		{"LimitAboveMaximum", "?limit=101", "ROL-1008"},
		{"NegativeLimit", "?limit=-1", "ROL-1008"},
		{"NegativeOffset", "?limit=10&offset=-1", "ROL-1009"},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			for _, base := range []string{
				rolesBasePath,
				rolesBasePath + "/" + sharedRoleID + "/assignments",
			} {
				resp := suite.doRoleRequest(http.MethodGet, base+tc.query, nil)
				suite.Equal(http.StatusBadRequest, resp.StatusCode,
					"%s%s must be rejected", base, tc.query)
				suite.Equal(tc.expectedCode, decodeErrorCode(suite.T(), resp))
				resp.Body.Close()
			}
		})
	}
}

// Agent assignments are stored and can be filtered by the agent type.
func (suite *RoleAPITestSuite) TestAssignments_AgentType() {
	role, err := suite.createRole(CreateRoleRequest{
		Name: "Role For Agent Assignment",
		OUID: testOUID,
		Permissions: []ResourcePermissions{
			{ResourceServerID: testResourceServer1ID, Permissions: []string{testPermission1}},
		},
		Assignments: []Assignment{
			{ID: testUserID1, Type: AssigneeTypeUser},
			{ID: testAgentID, Type: AssigneeTypeAgent},
		},
	})
	suite.Require().NoError(err)
	defer suite.deleteRole(role.ID)

	agentAssignments, err := suite.getRoleAssignmentsByType(role.ID, 0, 30, "agent")
	suite.Require().NoError(err)
	suite.Require().Equal(1, agentAssignments.TotalResults, "only the agent assignment should match")
	suite.Equal(testAgentID, agentAssignments.Assignments[0].ID)
	suite.Equal(AssigneeTypeAgent, agentAssignments.Assignments[0].Type)

	// The agent must not leak into the user-filtered page.
	userAssignments, err := suite.getRoleAssignmentsByType(role.ID, 0, 30, "user")
	suite.Require().NoError(err)
	suite.Require().Equal(1, userAssignments.TotalResults)
	suite.Equal(testUserID1, userAssignments.Assignments[0].ID)
}

// Display names resolve for app and agent assignees, not only users and groups.
func (suite *RoleAPITestSuite) TestGetRoleAssignments_AppAndAgentDisplay() {
	role, err := suite.createRole(CreateRoleRequest{
		Name: "Role For App And Agent Display",
		OUID: testOUID,
		Permissions: []ResourcePermissions{
			{ResourceServerID: testResourceServer1ID, Permissions: []string{testPermission1}},
		},
		Assignments: []Assignment{
			{ID: testAppID, Type: AssigneeTypeApp},
			{ID: testAgentID, Type: AssigneeTypeAgent},
		},
	})
	suite.Require().NoError(err)
	defer suite.deleteRole(role.ID)

	assignments, err := suite.getRoleAssignmentsWithInclude(role.ID, 0, 30, "display")
	suite.Require().NoError(err)
	suite.Require().Equal(2, assignments.TotalResults)

	displayByID := map[string]string{}
	for _, a := range assignments.Assignments {
		displayByID[a.ID] = a.Display
	}
	suite.Equal("Role Test App", displayByID[testAppID], "app display should be the application name")
	suite.Equal(testAgentName, displayByID[testAgentID], "agent display should be the agent name")
}

// A type-filtered page starting past the filtered total returns an empty page rather than
// an error, and still reports the full filtered count.
func (suite *RoleAPITestSuite) TestGetRoleAssignments_TypeFilterOffsetBeyondTotal() {
	role, err := suite.createRole(CreateRoleRequest{
		Name: "Role For Type Filter Overflow",
		OUID: testOUID,
		Permissions: []ResourcePermissions{
			{ResourceServerID: testResourceServer1ID, Permissions: []string{testPermission1}},
		},
		Assignments: []Assignment{
			{ID: testUserID1, Type: AssigneeTypeUser},
			{ID: testGroupID, Type: AssigneeTypeGroup},
		},
	})
	suite.Require().NoError(err)
	defer suite.deleteRole(role.ID)

	assignments, err := suite.getRoleAssignmentsByType(role.ID, 25, 30, "user")
	suite.Require().NoError(err)
	suite.Equal(1, assignments.TotalResults, "the filtered total is independent of the requested page")
	suite.Equal(0, assignments.Count)
	suite.Empty(assignments.Assignments)
}

// The group type filter paginates at the store level.
func (suite *RoleAPITestSuite) TestGetRoleAssignments_GroupTypeFilterPagination() {
	secondGroupID, err := testutils.CreateGroup(testutils.Group{
		Name:        "Second Role Test Group",
		Description: "Second group used for group-filter pagination",
		OUID:        testOUID,
	})
	suite.Require().NoError(err)
	defer func() { _ = testutils.DeleteGroup(secondGroupID) }()

	role, err := suite.createRole(CreateRoleRequest{
		Name: "Role For Group Filter Pagination",
		OUID: testOUID,
		Permissions: []ResourcePermissions{
			{ResourceServerID: testResourceServer1ID, Permissions: []string{testPermission1}},
		},
		Assignments: []Assignment{
			{ID: testGroupID, Type: AssigneeTypeGroup},
			{ID: testUserID1, Type: AssigneeTypeUser},
			{ID: secondGroupID, Type: AssigneeTypeGroup},
		},
	})
	suite.Require().NoError(err)
	defer suite.deleteRole(role.ID)

	page1, err := suite.getRoleAssignmentsByType(role.ID, 0, 1, "group")
	suite.Require().NoError(err)
	suite.Equal(2, page1.TotalResults, "TotalResults must reflect the group-filtered count")
	suite.Require().Equal(1, page1.Count)

	page2, err := suite.getRoleAssignmentsByType(role.ID, 1, 1, "group")
	suite.Require().NoError(err)
	suite.Equal(2, page2.TotalResults)
	suite.Require().Equal(1, page2.Count)
	suite.NotEqual(page1.Assignments[0].ID, page2.Assignments[0].ID,
		"the two pages must return different groups")
}

// Roles are listed under their organization unit.
func (suite *RoleAPITestSuite) TestListRolesByOU() {
	role1, err := suite.createRole(CreateRoleRequest{
		Name:        "OU Listing Role 1",
		OUID:        testOUID,
		Permissions: []ResourcePermissions{},
	})
	suite.Require().NoError(err)
	defer suite.deleteRole(role1.ID)

	role2, err := suite.createRole(CreateRoleRequest{
		Name:        "OU Listing Role 2",
		OUID:        testOUID,
		Permissions: []ResourcePermissions{},
	})
	suite.Require().NoError(err)
	defer suite.deleteRole(role2.ID)

	response, err := suite.listOURoles(fmt.Sprintf("/organization-units/%s/roles", testOUID), 0, 100)
	suite.Require().NoError(err)
	suite.Require().NotNil(response)
	suite.GreaterOrEqual(response.TotalResults, 2)

	ids := ouRoleIDs(response)
	suite.Contains(ids, role1.ID)
	suite.Contains(ids, role2.ID)
	for _, r := range response.Roles {
		suite.False(r.IsReadOnly, "database-backed roles must be reported as mutable")
	}

	// A single-item page must still report the full count.
	page, err := suite.listOURoles(fmt.Sprintf("/organization-units/%s/roles", testOUID), 0, 1)
	suite.Require().NoError(err)
	suite.Equal(1, page.Count)
	suite.Equal(response.TotalResults, page.TotalResults)
}

// Roles are listed under their organization unit addressed by handle path.
func (suite *RoleAPITestSuite) TestListRolesByOUPath() {
	role, err := suite.createRole(CreateRoleRequest{
		Name:        "OU Path Listing Role",
		OUID:        testOUID,
		Permissions: []ResourcePermissions{},
	})
	suite.Require().NoError(err)
	defer suite.deleteRole(role.ID)

	response, err := suite.listOURoles("/organization-units/tree/"+testOU.Handle+"/roles", 0, 100)
	suite.Require().NoError(err)
	suite.Require().NotNil(response)
	suite.Contains(ouRoleIDs(response), role.ID)
}

// The declarative organization unit lists its file-backed role as read-only. The role is
// declared with an ouHandle rather than an ouId, so this also covers the loader resolving the
// handle to the organization unit.
func (suite *RoleAPITestSuite) TestListRolesByOU_Declarative() {
	const declOUID = "decl-ou-1"
	const declRoleID = "decl-role-1"

	for _, path := range []string{
		"/organization-units/" + declOUID + "/roles",
		"/organization-units/tree/" + declOUID + "/roles",
	} {
		response, err := suite.listOURoles(path, 0, 100)
		suite.Require().NoError(err, "listing roles via %s should succeed", path)
		suite.Require().NotNil(response)

		var declRole *OURole
		for i := range response.Roles {
			if response.Roles[i].ID == declRoleID {
				declRole = &response.Roles[i]
				break
			}
		}
		suite.Require().NotNilf(declRole, "%s must list the declarative role", path)
		suite.True(declRole.IsReadOnly, "a file-backed role must be reported as read-only")
	}
}

// A declarative role cannot be updated through the API.
func (suite *RoleAPITestSuite) TestUpdateRole_DeclarativeImmutable() {
	updated, err := suite.updateRole("decl-role-1", UpdateRoleRequest{
		Name:        "Renamed Declarative Role",
		OUID:        "decl-ou-1",
		Permissions: []ResourcePermissions{},
	})
	suite.Error(err)
	suite.Nil(updated)
	suite.Contains(err.Error(), "ROL-1013")
}

// A declarative role cannot be deleted through the API.
func (suite *RoleAPITestSuite) TestDeleteRole_DeclarativeImmutable() {
	err := suite.deleteRole("decl-role-1")
	suite.Error(err)
	suite.Contains(err.Error(), "ROL-1013")

	// The role must survive the refused delete.
	role, err := suite.getRole("decl-role-1")
	suite.Require().NoError(err)
	suite.Equal("decl-role-1", role.ID)
}

// The type filters on a declarative role merge the assignments declared in the file with
// the ones added through the API, which live in the database.
func (suite *RoleAPITestSuite) TestGetRoleAssignments_DeclarativeRoleByType() {
	const declRoleID = "decl-role-1"
	const declUserID = "decl-user-1"
	const declGroupID = "decl-group-1"

	// Declared in the file: one user and one group. Both must be visible before anything is added.
	userAssignments, err := suite.getRoleAssignmentsByType(declRoleID, 0, 30, "user")
	suite.Require().NoError(err)
	suite.Require().Equal(1, userAssignments.TotalResults, "the declared user assignment must be listed")
	suite.Equal(declUserID, userAssignments.Assignments[0].ID)

	assignmentsRequest := AssignmentsRequest{
		Assignments: []Assignment{{ID: testGroupID, Type: AssigneeTypeGroup}},
	}
	suite.Require().NoError(suite.addAssignments(declRoleID, assignmentsRequest))
	defer func() {
		suite.NoError(suite.removeAssignments(declRoleID, assignmentsRequest))
	}()

	groupAssignments, err := suite.getRoleAssignmentsByType(declRoleID, 0, 30, "group")
	suite.Require().NoError(err)
	suite.Require().Equal(2, groupAssignments.TotalResults,
		"the declared group and the added group must both be listed")

	groupIDs := make([]string, 0, len(groupAssignments.Assignments))
	for _, a := range groupAssignments.Assignments {
		groupIDs = append(groupIDs, a.ID)
	}
	suite.Contains(groupIDs, declGroupID, "the file-declared group assignment must survive the merge")
	suite.Contains(groupIDs, testGroupID, "the database-backed group assignment must be included")

	// Adding a group must not change what the user filter returns.
	userAssignments, err = suite.getRoleAssignmentsByType(declRoleID, 0, 30, "user")
	suite.Require().NoError(err)
	suite.Equal(1, userAssignments.TotalResults, "a group assignment must not match the user filter")
}

// Permissions carried by a declarative role are honoured by access evaluation, whether
// the assignment is declared in the file or added at runtime through the API. The two bindings are
// stored in different places, so each is resolved by a different path through the composite store.
func (suite *RoleAPITestSuite) TestAccessEvaluation_DeclarativeRolePermissions() {
	const declRoleID = "decl-role-1"
	const declUserID = "decl-user-1"
	const declResourceServer = "https://localhost:8090/decl-rs-1"
	const grantedPermission = "test-resource:read"
	const withheldPermission = "test-resource:write"

	// The user declared in the role's YAML: resolved from the file store alone.
	suite.True(suite.evaluateAccess(declUserID, declResourceServer, grantedPermission),
		"the declared assignee must hold the declarative role's permission")
	suite.False(suite.evaluateAccess(declUserID, declResourceServer, withheldPermission),
		"a permission the declarative role does not carry must be denied")

	// A user assigned at runtime: the role definition lives in the file store while the assignment
	// lives in the database, so neither store can resolve this on its own.
	user := testUser1
	user.OUID = testOUID
	user.Attributes = json.RawMessage(`{
		"email": "declrolepermuser@example.com",
		"given_name": "Decl",
		"family_name": "PermUser",
		"password": "TestPassword123!"
	}`)
	runtimeUserID, err := testutils.CreateUser(user)
	suite.Require().NoError(err)
	defer func() { suite.NoError(testutils.DeleteUser(runtimeUserID)) }()

	assignmentsRequest := AssignmentsRequest{
		Assignments: []Assignment{{ID: runtimeUserID, Type: AssigneeTypeUser}},
	}
	suite.Require().NoError(suite.addAssignments(declRoleID, assignmentsRequest))
	defer func() { suite.NoError(suite.removeAssignments(declRoleID, assignmentsRequest)) }()

	suite.True(suite.evaluateAccess(runtimeUserID, declResourceServer, grantedPermission),
		"a runtime assignment to a declarative role must confer its permissions")
	suite.False(suite.evaluateAccess(runtimeUserID, declResourceServer, withheldPermission),
		"a permission the declarative role does not carry must be denied")

	// Removing the assignment must withdraw the permission.
	suite.Require().NoError(suite.removeAssignments(declRoleID, assignmentsRequest))
	suite.False(suite.evaluateAccess(runtimeUserID, declResourceServer, grantedPermission),
		"the permission must be withdrawn once the assignment is removed")
}

// A role is exported as a declarative document carrying its permissions and assignments.
func (suite *RoleAPITestSuite) TestExportRole() {
	role, err := suite.createRole(CreateRoleRequest{
		Name:        "Exportable Role",
		Description: "Role exercised by the export endpoint",
		OUID:        testOUID,
		Permissions: []ResourcePermissions{
			{ResourceServerID: testResourceServer1ID, Permissions: []string{testPermission1}},
		},
		Assignments: []Assignment{
			{ID: testUserID1, Type: AssigneeTypeUser},
		},
	})
	suite.Require().NoError(err)
	defer suite.deleteRole(role.ID)

	resources, err := suite.exportRoles([]string{role.ID})
	suite.Require().NoError(err)
	suite.Require().NotEmpty(resources)

	suite.Contains(resources, "resource_type: role")
	suite.Contains(resources, "name: Exportable Role")
	suite.Contains(resources, testResourceServer1ID)
	suite.Contains(resources, testUserID1, "the role's assignments must be part of the export")
}

// A wildcard export enumerates every role in the database store. Declarative roles are excluded:
// they already exist as YAML resources and are not re-emitted by the exporter.
func (suite *RoleAPITestSuite) TestExportAllRoles_Wildcard() {
	first, err := suite.createRole(CreateRoleRequest{
		Name:        "Wildcard Exportable Role One",
		Description: "First role picked up by the wildcard export",
		OUID:        testOUID,
		Permissions: []ResourcePermissions{
			{ResourceServerID: testResourceServer1ID, Permissions: []string{testPermission1}},
		},
	})
	suite.Require().NoError(err)
	defer suite.deleteRole(first.ID)

	second, err := suite.createRole(CreateRoleRequest{
		Name:        "Wildcard Exportable Role Two",
		Description: "Second role picked up by the wildcard export",
		OUID:        testOUID,
		Permissions: []ResourcePermissions{
			{ResourceServerID: testResourceServer2ID, Permissions: []string{testPermission3}},
		},
		Assignments: []Assignment{
			{ID: testGroupID, Type: AssigneeTypeGroup},
		},
	})
	suite.Require().NoError(err)
	defer suite.deleteRole(second.ID)

	resources, err := suite.exportRoles([]string{"*"})
	suite.Require().NoError(err)
	suite.Require().NotEmpty(resources)

	suite.Contains(resources, "name: Wildcard Exportable Role One")
	suite.Contains(resources, "name: Wildcard Exportable Role Two")
	suite.Contains(resources, testGroupID, "assignments must be carried through the wildcard export")
	// This behaviour may change based on #4966 issue, but for now the declarative role is excluded from the export.
	suite.NotContains(resources, "decl-role-1",
		"declarative roles must be excluded from the wildcard export")
	suite.NotContains(resources, "name: Declarative Test Role",
		"declarative roles must be excluded from the wildcard export")
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

// doRoleRequest issues a request against the test server and returns the raw response, so that
// tests can assert on status codes and error payloads the typed helpers hide.
func (suite *RoleAPITestSuite) doRoleRequest(method, path string, body []byte) *http.Response {
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

	resp, err := suite.client.Do(req)
	suite.Require().NoError(err)
	return resp
}

// listOURoles fetches a paginated role listing from one of the organization unit role endpoints.
func (suite *RoleAPITestSuite) listOURoles(path string, offset, limit int) (*OURoleListResponse, error) {
	url := fmt.Sprintf("%s%s?offset=%d&limit=%d", testServerURL, path, offset, limit)
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
		return nil, fmt.Errorf("failed to list organization unit roles: %s - %s", errResp.Code, errResp.Message)
	}

	var response OURoleListResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// exportRoles exports the given roles and returns the rendered declarative documents.
func (suite *RoleAPITestSuite) exportRoles(roleIDs []string) (string, error) {
	body, err := json.Marshal(ExportRequest{Roles: roleIDs})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", testServerURL+"/export", bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to export roles, status %d: %s", resp.StatusCode, string(respBody))
	}

	var response ExportResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return "", err
	}

	return response.Resources, nil
}

// evaluateAccess asks the access evaluation endpoint whether the entity holds the given permission
// on the given resource server, and returns the decision.
func (suite *RoleAPITestSuite) evaluateAccess(entityID, resourceServer, permission string) bool {
	suite.T().Helper()

	body, err := json.Marshal(EvaluationRequest{
		Subject:  EvaluationSubject{Type: "user", ID: entityID},
		Resource: EvaluationResource{Type: resourceServer, ID: "role-test-resource-instance"},
		Action:   EvaluationAction{Name: permission},
	})
	suite.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPost, testServerURL+"/access/v1/evaluation", bytes.NewBuffer(body))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)
	suite.Require().Equal(http.StatusOK, resp.StatusCode, "access evaluation failed: %s", string(respBody))

	var evaluation EvaluationResponse
	suite.Require().NoError(json.Unmarshal(respBody, &evaluation))
	return evaluation.Decision
}

// ouRoleIDs collects the role IDs from an organization unit role listing.
func ouRoleIDs(response *OURoleListResponse) []string {
	ids := make([]string, 0, len(response.Roles))
	for _, r := range response.Roles {
		ids = append(ids, r.ID)
	}
	return ids
}

// mustMarshal serializes a request body, failing the test rather than returning an error.
func mustMarshal(t *testing.T, v interface{}) []byte {
	t.Helper()

	body, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}
	return body
}

// decodeErrorCode reads the error code from an error response body. Only the code is decoded:
// message and description are localized objects rather than plain strings.
func decodeErrorCode(t *testing.T, resp *http.Response) string {
	t.Helper()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read error response body: %v", err)
	}

	var errResp struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(body, &errResp); err != nil {
		t.Fatalf("failed to decode error response %q: %v", string(body), err)
	}
	return errResp.Code
}
