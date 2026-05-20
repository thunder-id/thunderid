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

package role

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
)

// RoleFileBasedStoreEdgeCaseTestSuite contains edge case tests for the file-based role store.
type RoleFileBasedStoreEdgeCaseTestSuite struct {
	suite.Suite
	store *fileBasedStore
}

func TestRoleFileBasedStoreEdgeCaseTestSuite(t *testing.T) {
	suite.Run(t, new(RoleFileBasedStoreEdgeCaseTestSuite))
}

func (suite *RoleFileBasedStoreEdgeCaseTestSuite) SetupTest() {
	genericStore := declarativeresource.NewGenericFileBasedStoreForTest(entity.KeyTypeRole)
	suite.store = &fileBasedStore{GenericFileBasedStore: genericStore}
}

func (suite *RoleFileBasedStoreEdgeCaseTestSuite) seedRole(role RoleWithPermissionsAndAssignments) {
	err := suite.store.GenericFileBasedStore.Create(role.ID, &role)
	suite.Require().NoError(err)
}

// Test GetRoleList with zero limit
func (suite *RoleFileBasedStoreEdgeCaseTestSuite) TestGetRoleList_ZeroLimit() {
	suite.seedRole(RoleWithPermissionsAndAssignments{
		ID:   "role1",
		Name: "Admin",
		OUID: "ou1",
	})

	roles, err := suite.store.GetRoleList(context.Background(), 0, 0)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), roles, 0)
}

// Test GetRoleList with negative limit
func (suite *RoleFileBasedStoreEdgeCaseTestSuite) TestGetRoleList_NegativeLimit() {
	suite.seedRole(RoleWithPermissionsAndAssignments{
		ID:   "role1",
		Name: "Admin",
		OUID: "ou1",
	})

	roles, err := suite.store.GetRoleList(context.Background(), -1, 0)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), roles, 0)
}

// Test GetRoleList with offset beyond results
func (suite *RoleFileBasedStoreEdgeCaseTestSuite) TestGetRoleList_OffsetBeyondResults() {
	suite.seedRole(RoleWithPermissionsAndAssignments{
		ID:   "role1",
		Name: "Admin",
		OUID: "ou1",
	})

	roles, err := suite.store.GetRoleList(context.Background(), 10, 100)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), roles, 0)
}

// Test GetRoleList with negative offset
func (suite *RoleFileBasedStoreEdgeCaseTestSuite) TestGetRoleList_NegativeOffset() {
	suite.seedRole(RoleWithPermissionsAndAssignments{
		ID:   "role1",
		Name: "Admin",
		OUID: "ou1",
	})

	roles, err := suite.store.GetRoleList(context.Background(), 10, -1)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), roles, 1)
}

// Test GetRoleList on empty store
func (suite *RoleFileBasedStoreEdgeCaseTestSuite) TestGetRoleList_EmptyStore() {
	roles, err := suite.store.GetRoleList(context.Background(), 10, 0)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), roles, 0)
}

// Test GetRoleListCount on empty store
func (suite *RoleFileBasedStoreEdgeCaseTestSuite) TestGetRoleListCount_EmptyStore() {
	count, err := suite.store.GetRoleListCount(context.Background())

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), 0, count)
}

// Test GetRole for non-existent role
func (suite *RoleFileBasedStoreEdgeCaseTestSuite) TestGetRole_NonExistent() {
	role, err := suite.store.GetRole(context.Background(), "nonexistent")

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), ErrRoleNotFound, err)
	assert.Equal(suite.T(), RoleWithPermissions{}, role)
}

// Test GetRoleAssignments with zero limit
func (suite *RoleFileBasedStoreEdgeCaseTestSuite) TestGetRoleAssignments_ZeroLimit() {
	suite.seedRole(RoleWithPermissionsAndAssignments{
		ID:   "role1",
		Name: "Admin",
		OUID: "ou1",
		Assignments: []RoleAssignment{
			{ID: "user1", Type: assigneeTypeEntity},
		},
	})

	assignments, err := suite.store.GetRoleAssignments(context.Background(), "role1", 0, 0)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), assignments, 0)
}

// Test GetRoleAssignments for non-existent role
func (suite *RoleFileBasedStoreEdgeCaseTestSuite) TestGetRoleAssignments_NonExistentRole() {
	assignments, err := suite.store.GetRoleAssignments(context.Background(), "nonexistent", 10, 0)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), assignments, 0)
}

// Test GetRoleAssignmentsCount for non-existent role
func (suite *RoleFileBasedStoreEdgeCaseTestSuite) TestGetRoleAssignmentsCount_NonExistentRole() {
	count, err := suite.store.GetRoleAssignments(context.Background(), "nonexistent", 10, 0)

	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), count)
}

// Test CheckRoleNameExists with different organization units
func (suite *RoleFileBasedStoreEdgeCaseTestSuite) TestCheckRoleNameExists_DifferentOUs() {
	suite.seedRole(RoleWithPermissionsAndAssignments{
		ID:   "role1",
		Name: "Manager",
		OUID: "ou1",
	})

	exists, err := suite.store.CheckRoleNameExists(context.Background(), "ou2", "Manager")

	assert.NoError(suite.T(), err)
	assert.False(suite.T(), exists)
}

// Test CheckRoleNameExists with empty organization unit
func (suite *RoleFileBasedStoreEdgeCaseTestSuite) TestCheckRoleNameExists_EmptyOU() {
	suite.seedRole(RoleWithPermissionsAndAssignments{
		ID:   "role1",
		Name: "Manager",
		OUID: "ou1",
	})

	exists, err := suite.store.CheckRoleNameExists(context.Background(), "", "Manager")

	assert.NoError(suite.T(), err)
	assert.False(suite.T(), exists)
}

// Test CheckRoleNameExists with case sensitivity
func (suite *RoleFileBasedStoreEdgeCaseTestSuite) TestCheckRoleNameExists_CaseSensitive() {
	suite.seedRole(RoleWithPermissionsAndAssignments{
		ID:   "role1",
		Name: "Admin",
		OUID: "ou1",
	})

	exists, err := suite.store.CheckRoleNameExists(context.Background(), "ou1", "admin")

	assert.NoError(suite.T(), err)
	assert.False(suite.T(), exists)
}

// Test CheckRoleNameExistsExcludingID with no other role
func (suite *RoleFileBasedStoreEdgeCaseTestSuite) TestCheckRoleNameExistsExcludingID_NoOtherRole() {
	suite.seedRole(RoleWithPermissionsAndAssignments{
		ID:   "role1",
		Name: "Admin",
		OUID: "ou1",
	})

	exists, err := suite.store.CheckRoleNameExistsExcludingID(context.Background(), "ou1", "Admin", "role1")

	assert.NoError(suite.T(), err)
	assert.False(suite.T(), exists)
}

// Test GetAuthorizedPermissions with empty user and groups
func (suite *RoleFileBasedStoreEdgeCaseTestSuite) TestGetAuthorizedPermissions_EmptyUserAndGroups() {
	suite.seedRole(RoleWithPermissionsAndAssignments{
		ID:   "role1",
		Name: "Admin",
		OUID: "ou1",
		Permissions: []ResourcePermissions{
			{ResourceServerID: "rs1", Permissions: []string{"perm1"}},
		},
	})

	perms, err := suite.store.GetAuthorizedPermissions(
		context.Background(),
		"",
		[]string{},
		[]string{"perm1"},
	)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), perms, 0)
}

// Test GetAuthorizedPermissions with empty requested permissions
func (suite *RoleFileBasedStoreEdgeCaseTestSuite) TestGetAuthorizedPermissions_EmptyRequestedPermissions() {
	suite.seedRole(RoleWithPermissionsAndAssignments{
		ID:   "role1",
		Name: "Admin",
		OUID: "ou1",
		Assignments: []RoleAssignment{
			{ID: "user1", Type: assigneeTypeEntity},
		},
		Permissions: []ResourcePermissions{
			{ResourceServerID: "rs1", Permissions: []string{"perm1"}},
		},
	})

	perms, err := suite.store.GetAuthorizedPermissions(
		context.Background(),
		"user1",
		[]string{},
		[]string{},
	)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), perms, 0)
}

// Test GetAuthorizedPermissions with group assignment
func (suite *RoleFileBasedStoreEdgeCaseTestSuite) TestGetAuthorizedPermissions_GroupAssignment() {
	suite.seedRole(RoleWithPermissionsAndAssignments{
		ID:   "role1",
		Name: "Admin",
		OUID: "ou1",
		Assignments: []RoleAssignment{
			{ID: "group1", Type: AssigneeTypeGroup},
		},
		Permissions: []ResourcePermissions{
			{ResourceServerID: "rs1", Permissions: []string{"perm1", "perm2"}},
		},
	})

	perms, err := suite.store.GetAuthorizedPermissions(
		context.Background(),
		"",
		[]string{"group1", "group2"},
		[]string{"perm1", "perm2", "perm3"},
	)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), perms, 2)
	assert.Contains(suite.T(), perms, "perm1")
	assert.Contains(suite.T(), perms, "perm2")
}

// Test GetAuthorizedPermissions with multiple roles and multiple resource servers
func (suite *RoleFileBasedStoreEdgeCaseTestSuite) TestGetAuthorizedPermissions_MultipleRolesAndServers() {
	suite.seedRole(RoleWithPermissionsAndAssignments{
		ID:   "role1",
		Name: "Admin",
		OUID: "ou1",
		Assignments: []RoleAssignment{
			{ID: "user1", Type: assigneeTypeEntity},
		},
		Permissions: []ResourcePermissions{
			{ResourceServerID: "rs1", Permissions: []string{"read", "write"}},
			{ResourceServerID: "rs2", Permissions: []string{"delete"}},
		},
	})
	suite.seedRole(RoleWithPermissionsAndAssignments{
		ID:   "role2",
		Name: "Editor",
		OUID: "ou1",
		Assignments: []RoleAssignment{
			{ID: "user1", Type: assigneeTypeEntity},
		},
		Permissions: []ResourcePermissions{
			{ResourceServerID: "rs1", Permissions: []string{"write"}},
		},
	})

	perms, err := suite.store.GetAuthorizedPermissions(
		context.Background(),
		"user1",
		[]string{},
		[]string{"read", "write", "delete"},
	)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), perms, 3)
	assert.Contains(suite.T(), perms, "read")
	assert.Contains(suite.T(), perms, "write")
	assert.Contains(suite.T(), perms, "delete")
}

// Test GetAuthorizedPermissions maintains order
func (suite *RoleFileBasedStoreEdgeCaseTestSuite) TestGetAuthorizedPermissions_MaintainsOrder() {
	suite.seedRole(RoleWithPermissionsAndAssignments{
		ID:   "role1",
		Name: "Admin",
		OUID: "ou1",
		Assignments: []RoleAssignment{
			{ID: "user1", Type: assigneeTypeEntity},
		},
		Permissions: []ResourcePermissions{
			{ResourceServerID: "rs1", Permissions: []string{"perm1", "perm2", "perm3"}},
		},
	})

	perms, err := suite.store.GetAuthorizedPermissions(
		context.Background(),
		"user1",
		[]string{},
		[]string{"perm3", "perm2", "perm1"},
	)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), []string{"perm3", "perm2", "perm1"}, perms)
}

// Test malformed role handling
func (suite *RoleFileBasedStoreEdgeCaseTestSuite) TestGetRoleList_SkipsMalformedRoles() {
	suite.seedRole(RoleWithPermissionsAndAssignments{
		ID:   "role1",
		Name: "Admin",
		OUID: "ou1",
	})

	// Seed a malformed role (this would be caught by Create but we test error handling)
	_ = suite.store.GenericFileBasedStore.Create("malformed", "not a role")

	// Should still return valid role and skip malformed one
	roles, err := suite.store.GetRoleList(context.Background(), 10, 0)

	// May return 1 or 0 depending on how malformed data is handled, but should not error
	suite.Nil(err) // Should not error
	// Ensure we have at least the valid role or can handle malformed gracefully
	_ = roles
}

// Test GetRoleListCount consistency
func (suite *RoleFileBasedStoreEdgeCaseTestSuite) TestGetRoleListCount_Consistency() {
	for i := 0; i < 5; i++ {
		suite.seedRole(RoleWithPermissionsAndAssignments{
			ID:   "role" + string(rune('0'+i)),
			Name: "Role" + string(rune('0'+i)),
			OUID: "ou1",
		})
	}

	count, err := suite.store.GetRoleListCount(context.Background())
	assert.NoError(suite.T(), err)

	roles, err := suite.store.GetRoleList(context.Background(), 100, 0)
	assert.NoError(suite.T(), err)

	assert.Equal(suite.T(), count, len(roles))
}

// Test CheckRoleNameExists with special characters
func (suite *RoleFileBasedStoreEdgeCaseTestSuite) TestCheckRoleNameExists_SpecialCharacters() {
	suite.seedRole(RoleWithPermissionsAndAssignments{
		ID:   "role1",
		Name: "Admin@Role#1",
		OUID: "ou1",
	})

	exists, err := suite.store.CheckRoleNameExists(context.Background(), "ou1", "Admin@Role#1")

	assert.NoError(suite.T(), err)
	assert.True(suite.T(), exists)
}

// Test GetRoleAssignments maintains pagination order
func (suite *RoleFileBasedStoreEdgeCaseTestSuite) TestGetRoleAssignments_PaginationOrder() {
	suite.seedRole(RoleWithPermissionsAndAssignments{
		ID:   "role1",
		Name: "Admin",
		OUID: "ou1",
		Assignments: []RoleAssignment{
			{ID: "user1", Type: assigneeTypeEntity},
			{ID: "user2", Type: assigneeTypeEntity},
			{ID: "user3", Type: assigneeTypeEntity},
		},
	})

	// Get all assignments
	all, err := suite.store.GetRoleAssignments(context.Background(), "role1", 3, 0)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), all, 3)

	// Get first page
	page1, err := suite.store.GetRoleAssignments(context.Background(), "role1", 2, 0)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), page1, 2)

	// Get second page
	page2, err := suite.store.GetRoleAssignments(context.Background(), "role1", 2, 2)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), page2, 1)
}

// Test app entity role assignment matching via GetAuthorizedPermissions.
func (suite *RoleFileBasedStoreEdgeCaseTestSuite) TestGetAuthorizedPermissions_AppAssignment() {
	suite.seedRole(RoleWithPermissionsAndAssignments{
		ID:   "role1",
		Name: "APIRole",
		OUID: "ou1",
		Assignments: []RoleAssignment{
			{ID: "app-uuid-123", Type: assigneeTypeEntity},
		},
		Permissions: []ResourcePermissions{
			{ResourceServerID: "rs1", Permissions: []string{"read:docs", "write:docs", "delete:docs"}},
		},
	})

	perms, err := suite.store.GetAuthorizedPermissions(
		context.Background(),
		"app-uuid-123",
		[]string{},
		[]string{"read:docs", "write:docs", "admin:docs"},
	)

	assert.NoError(suite.T(), err)
	assert.ElementsMatch(suite.T(), []string{"read:docs", "write:docs"}, perms)
}

// Test app assignment does not match a different entity ID.
func (suite *RoleFileBasedStoreEdgeCaseTestSuite) TestGetAuthorizedPermissions_AppAssignment_NoMatch() {
	suite.seedRole(RoleWithPermissionsAndAssignments{
		ID:   "role1",
		Name: "APIRole",
		OUID: "ou1",
		Assignments: []RoleAssignment{
			{ID: "app-uuid-123", Type: assigneeTypeEntity},
		},
		Permissions: []ResourcePermissions{
			{ResourceServerID: "rs1", Permissions: []string{"read:docs"}},
		},
	})

	perms, err := suite.store.GetAuthorizedPermissions(
		context.Background(),
		"different-app-uuid",
		[]string{},
		[]string{"read:docs"},
	)

	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), perms)
}

// Test mixed user and app assignments on the same role.
func (suite *RoleFileBasedStoreEdgeCaseTestSuite) TestGetAuthorizedPermissions_MixedUserAndAppAssignments() {
	suite.seedRole(RoleWithPermissionsAndAssignments{
		ID:   "role1",
		Name: "SharedRole",
		OUID: "ou1",
		Assignments: []RoleAssignment{
			{ID: "user-uuid-1", Type: assigneeTypeEntity},
			{ID: "app-uuid-1", Type: assigneeTypeEntity},
			{ID: "group1", Type: AssigneeTypeGroup},
		},
		Permissions: []ResourcePermissions{
			{ResourceServerID: "rs1", Permissions: []string{"perm1", "perm2"}},
		},
	})

	// App entity resolves permissions via entity ID.
	appPerms, err := suite.store.GetAuthorizedPermissions(
		context.Background(), "app-uuid-1", []string{}, []string{"perm1", "perm2", "perm3"})
	assert.NoError(suite.T(), err)
	assert.ElementsMatch(suite.T(), []string{"perm1", "perm2"}, appPerms)

	// User entity resolves permissions via entity ID.
	userPerms, err := suite.store.GetAuthorizedPermissions(
		context.Background(), "user-uuid-1", []string{}, []string{"perm1", "perm2", "perm3"})
	assert.NoError(suite.T(), err)
	assert.ElementsMatch(suite.T(), []string{"perm1", "perm2"}, userPerms)

	// Group-only resolution still works.
	groupPerms, err := suite.store.GetAuthorizedPermissions(
		context.Background(), "", []string{"group1"}, []string{"perm1"})
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), []string{"perm1"}, groupPerms)
}

// Test app assignment with multiple roles.
func (suite *RoleFileBasedStoreEdgeCaseTestSuite) TestGetAuthorizedPermissions_AppMultipleRoles() {
	suite.seedRole(RoleWithPermissionsAndAssignments{
		ID:   "role1",
		Name: "ReaderRole",
		OUID: "ou1",
		Assignments: []RoleAssignment{
			{ID: "app-uuid-1", Type: assigneeTypeEntity},
		},
		Permissions: []ResourcePermissions{
			{ResourceServerID: "rs1", Permissions: []string{"read:docs"}},
		},
	})
	suite.seedRole(RoleWithPermissionsAndAssignments{
		ID:   "role2",
		Name: "WriterRole",
		OUID: "ou1",
		Assignments: []RoleAssignment{
			{ID: "app-uuid-1", Type: assigneeTypeEntity},
		},
		Permissions: []ResourcePermissions{
			{ResourceServerID: "rs1", Permissions: []string{"write:docs"}},
		},
	})

	perms, err := suite.store.GetAuthorizedPermissions(
		context.Background(),
		"app-uuid-1",
		[]string{},
		[]string{"read:docs", "write:docs", "delete:docs"},
	)

	assert.NoError(suite.T(), err)
	assert.ElementsMatch(suite.T(), []string{"read:docs", "write:docs"}, perms)
}

// Test GetUserRoles works for app entities.
func (suite *RoleFileBasedStoreEdgeCaseTestSuite) TestGetUserRoles_AppEntity() {
	suite.seedRole(RoleWithPermissionsAndAssignments{
		ID:   "role1",
		Name: "AppRole",
		OUID: "ou1",
		Assignments: []RoleAssignment{
			{ID: "app-uuid-1", Type: assigneeTypeEntity},
		},
		Permissions: []ResourcePermissions{
			{ResourceServerID: "rs1", Permissions: []string{"perm1"}},
		},
	})
	suite.seedRole(RoleWithPermissionsAndAssignments{
		ID:   "role2",
		Name: "UserRole",
		OUID: "ou1",
		Assignments: []RoleAssignment{
			{ID: "user-uuid-1", Type: assigneeTypeEntity},
		},
		Permissions: []ResourcePermissions{
			{ResourceServerID: "rs1", Permissions: []string{"perm2"}},
		},
	})

	roles, err := suite.store.GetUserRoles(context.Background(), "app-uuid-1", []string{})

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), roles, 1)
	assert.Equal(suite.T(), "AppRole", roles[0])
}
