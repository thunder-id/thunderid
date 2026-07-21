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

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"

	"github.com/stretchr/testify/suite"
)

// RoleFileBasedStoreTestSuite contains tests for the file-based role store.
type RoleFileBasedStoreTestSuite struct {
	suite.Suite
	store *fileBasedStore
}

func TestRoleFileBasedStoreTestSuite(t *testing.T) {
	suite.Run(t, new(RoleFileBasedStoreTestSuite))
}

func (suite *RoleFileBasedStoreTestSuite) SetupTest() {
	genericStore := declarativeresource.NewGenericFileBasedStoreForTest(entity.KeyTypeRole)
	suite.store = &fileBasedStore{GenericFileBasedStore: genericStore}
}

func (suite *RoleFileBasedStoreTestSuite) seedRole(role RoleWithPermissionsAndAssignments) {
	err := suite.store.GenericFileBasedStore.Create(role.ID, &role)
	suite.Require().NoError(err)
}

func (suite *RoleFileBasedStoreTestSuite) TestGetRoleListCountAndList() {
	suite.seedRole(RoleWithPermissionsAndAssignments{
		ID:   "role1",
		Name: "Admin",
		OUID: "ou1",
	})
	suite.seedRole(RoleWithPermissionsAndAssignments{
		ID:   "role2",
		Name: "Viewer",
		OUID: "ou1",
	})

	count, err := suite.store.GetRoleListCount(context.Background())

	suite.NoError(err)
	suite.Equal(2, count)

	roles, err := suite.store.GetRoleList(context.Background(), 10, 0)

	suite.NoError(err)
	suite.Len(roles, 2)
	roleIDs := map[string]bool{}
	for _, role := range roles {
		roleIDs[role.ID] = true
	}
	suite.True(roleIDs["role1"])
	suite.True(roleIDs["role2"])

	pagedRoles, err := suite.store.GetRoleList(context.Background(), 1, 1)

	suite.NoError(err)
	suite.Len(pagedRoles, 1)
}

func (suite *RoleFileBasedStoreTestSuite) TestGetRoleListCountByOUIDAndListByOUID() {
	suite.seedRole(RoleWithPermissionsAndAssignments{
		ID:   "role1",
		Name: "Admin",
		OUID: "ou1",
	})
	suite.seedRole(RoleWithPermissionsAndAssignments{
		ID:   "role2",
		Name: "Viewer",
		OUID: "ou1",
	})
	suite.seedRole(RoleWithPermissionsAndAssignments{
		ID:   "role3",
		Name: "OtherOUAdmin",
		OUID: "ou2",
	})

	count, err := suite.store.GetRoleListCountByOUID(context.Background(), "ou1")

	suite.NoError(err)
	suite.Equal(2, count)

	roles, err := suite.store.GetRoleListByOUID(context.Background(), "ou1", 10, 0)

	suite.NoError(err)
	suite.Len(roles, 2)
	roleIDs := map[string]bool{}
	for _, role := range roles {
		roleIDs[role.ID] = true
		suite.Equal("ou1", role.OUID)
	}
	suite.True(roleIDs["role1"])
	suite.True(roleIDs["role2"])

	pagedRoles, err := suite.store.GetRoleListByOUID(context.Background(), "ou1", 1, 1)

	suite.NoError(err)
	suite.Len(pagedRoles, 1)

	otherOUCount, err := suite.store.GetRoleListCountByOUID(context.Background(), "ou2")

	suite.NoError(err)
	suite.Equal(1, otherOUCount)
}

func (suite *RoleFileBasedStoreTestSuite) TestGetRoleListByOUID_NoMatches() {
	suite.seedRole(RoleWithPermissionsAndAssignments{
		ID:   "role1",
		Name: "Admin",
		OUID: "ou1",
	})

	roles, err := suite.store.GetRoleListByOUID(context.Background(), "nonexistent-ou", 10, 0)

	suite.NoError(err)
	suite.Empty(roles)
}

func (suite *RoleFileBasedStoreTestSuite) TestGetRoleAndExistence() {
	suite.seedRole(RoleWithPermissionsAndAssignments{
		ID:          "role1",
		Name:        "Admin",
		OUID:        "ou1",
		Permissions: []ResourcePermissions{{ResourceServerID: "rs1", Permissions: []string{"perm1"}}},
	})

	role, err := suite.store.GetRole(context.Background(), "role1")

	suite.NoError(err)
	suite.Equal("role1", role.ID)
	suite.Len(role.Permissions, 1)

	exists, err := suite.store.IsRoleExist(context.Background(), "role1")
	suite.NoError(err)
	suite.True(exists)

	exists, err = suite.store.IsRoleExist(context.Background(), "missing")
	suite.NoError(err)
	suite.False(exists)
}

func (suite *RoleFileBasedStoreTestSuite) TestGetRoleAssignmentsAndCount() {
	suite.seedRole(RoleWithPermissionsAndAssignments{
		ID:   "role1",
		Name: "Admin",
		OUID: "ou1",
		Assignments: []RoleAssignment{
			{ID: "user1", Type: assigneeTypeEntity},
			{ID: "group1", Type: AssigneeTypeGroup},
		},
	})

	count, err := suite.store.GetRoleAssignmentsCount(context.Background(), "role1")

	suite.NoError(err)
	suite.Equal(2, count)

	assignments, err := suite.store.GetRoleAssignments(context.Background(), "role1", 1, 1)

	suite.NoError(err)
	suite.Len(assignments, 1)
	suite.Equal("group1", assignments[0].ID)
}

func (suite *RoleFileBasedStoreTestSuite) TestCheckRoleNameExists() {
	suite.seedRole(RoleWithPermissionsAndAssignments{
		ID:   "role1",
		Name: "Admin",
		OUID: "ou1",
	})
	suite.seedRole(RoleWithPermissionsAndAssignments{
		ID:   "role2",
		Name: "Admin",
		OUID: "ou2",
	})

	exists, err := suite.store.CheckRoleNameExists(context.Background(), "ou1", "Admin")

	suite.NoError(err)
	suite.True(exists)

	exists, err = suite.store.CheckRoleNameExists(context.Background(), "ou1", "Missing")

	suite.NoError(err)
	suite.False(exists)
}

func (suite *RoleFileBasedStoreTestSuite) TestCheckRoleNameExistsExcludingID() {
	suite.seedRole(RoleWithPermissionsAndAssignments{
		ID:   "role1",
		Name: "Admin",
		OUID: "ou1",
	})
	suite.seedRole(RoleWithPermissionsAndAssignments{
		ID:   "role2",
		Name: "Admin",
		OUID: "ou1",
	})

	exists, err := suite.store.CheckRoleNameExistsExcludingID(context.Background(), "ou1", "Admin", "role1")

	suite.NoError(err)
	suite.True(exists)

	exists, err = suite.store.CheckRoleNameExistsExcludingID(context.Background(), "ou1", "Admin", "role2")

	suite.NoError(err)
	suite.True(exists)

	// Test case where the only matching role is the excluded ID (should return false)
	suite.seedRole(RoleWithPermissionsAndAssignments{
		ID:   "role3",
		Name: "Admin",
		OUID: "ou3",
	})

	exists, err = suite.store.CheckRoleNameExistsExcludingID(context.Background(), "ou3", "Admin", "role3")

	suite.NoError(err)
	suite.False(exists)
}

func (suite *RoleFileBasedStoreTestSuite) TestGetAuthorizedPermissions() {
	suite.seedRole(RoleWithPermissionsAndAssignments{
		ID:   "role1",
		Name: "Admin",
		OUID: "ou1",
		Assignments: []RoleAssignment{
			{ID: "user1", Type: assigneeTypeEntity},
			{ID: "group1", Type: AssigneeTypeGroup},
		},
		Permissions: []ResourcePermissions{
			{ResourceServerID: "rs1", Permissions: []string{"perm1", "perm2"}},
		},
	})

	perms, err := suite.store.GetAuthorizedPermissionsByResourceServer(
		context.Background(),
		"user1",
		[]string{"group1"}, "",

		[]string{"perm2", "perm3"})

	suite.NoError(err)
	suite.Equal([]string{"perm2"}, perms)
}

func (suite *RoleFileBasedStoreTestSuite) TestImmutability() {
	// Seed a role for testing
	suite.seedRole(RoleWithPermissionsAndAssignments{
		ID:   "immutable-role",
		Name: "Test Role",
		OUID: "ou1",
	})

	// Test CreateRole returns error
	err := suite.store.CreateRole(context.Background(), "new-role", RoleCreationDetail{
		Name: "New Role",
		OUID: "ou1",
	})
	suite.Error(err)

	// Test UpdateRole returns error
	err = suite.store.UpdateRole(context.Background(), "immutable-role", RoleUpdateDetail{
		Description: "Updated description",
	})
	suite.Error(err)

	// Test DeleteRole returns error
	err = suite.store.DeleteRole(context.Background(), "immutable-role")
	suite.Error(err)

	// Test AddAssignments returns error
	err = suite.store.AddAssignments(context.Background(), "immutable-role", []RoleAssignment{
		{ID: "user1", Type: assigneeTypeEntity},
	})
	suite.Error(err)

	// Test RemoveAssignments returns error
	err = suite.store.RemoveAssignments(context.Background(), "immutable-role", []RoleAssignment{
		{ID: "user1", Type: assigneeTypeEntity},
	})
	suite.Error(err)
}

func (suite *RoleFileBasedStoreTestSuite) TestIsRoleDeclarative() {
	// Seed a role
	suite.seedRole(RoleWithPermissionsAndAssignments{
		ID:   "declarative-role",
		Name: "Declarative Role",
		OUID: "ou1",
	})

	// Test IsRoleDeclarative returns true for seeded role
	isDeclarative, err := suite.store.IsRoleDeclarative(context.Background(), "declarative-role")
	suite.NoError(err)
	suite.True(isDeclarative)

	// Test IsRoleDeclarative returns false for nonexistent role
	isDeclarative, err = suite.store.IsRoleDeclarative(context.Background(), "nonexistent")
	suite.NoError(err)
	suite.False(isDeclarative)
}

// TestCreate_ImplementsStorer tests the Create method from the declarativeresource.Storer interface
func (suite *RoleFileBasedStoreTestSuite) TestCreate_ImplementsStorer() {
	role := &RoleWithPermissionsAndAssignments{
		ID:          "test-role-create",
		Name:        "Test Role Create",
		Description: "Test create implementation",
		OUID:        "ou1",
	}

	err := suite.store.Create("test-role-create", role)
	suite.NoError(err)

	// Verify the role was created
	retrievedRole, err := suite.store.GetRole(context.Background(), "test-role-create")
	suite.NoError(err)
	suite.Equal("test-role-create", retrievedRole.ID)
	suite.Equal("Test Role Create", retrievedRole.Name)
}

// TestCreate_InvalidData tests Create with invalid data type
func (suite *RoleFileBasedStoreTestSuite) TestCreate_InvalidData() {
	err := suite.store.Create("invalid-role", "invalid string data")
	suite.Error(err)
	suite.Equal("role data corrupted", err.Error())
}

// TestCreate_SetsIDFromParameter tests Create sets ID from parameter if not in data
func (suite *RoleFileBasedStoreTestSuite) TestCreate_SetsIDFromParameter() {
	role := &RoleWithPermissionsAndAssignments{
		Name: "Role with ID from param",
		OUID: "ou1",
		// ID is empty, should be set from parameter
	}

	err := suite.store.Create("param-role-id", role)
	suite.NoError(err)

	// Verify the role ID was set from parameter
	retrievedRole, err := suite.store.GetRole(context.Background(), "param-role-id")
	suite.NoError(err)
	suite.Equal("param-role-id", retrievedRole.ID)
}

// GetEntityRoleIDs on the file-based store is a deliberate no-op: API-added role
// assignments are persisted in the DB, never in YAML, so the file store has no record
// to surface. Composite callers rely on this returning an empty (non-nil) slice so
// the union of (db, file) sources stays correct.
func (suite *RoleFileBasedStoreTestSuite) TestGetEntityRoleIDs_AlwaysEmpty() {
	suite.seedRole(RoleWithPermissionsAndAssignments{
		ID: "r1", Name: "R1", OUID: "ou1",
		Assignments: []RoleAssignment{
			{ID: "user-x", Type: assigneeTypeEntity},
			{ID: "group-y", Type: AssigneeTypeGroup},
		},
	})

	cases := []struct {
		name     string
		entityID string
		groupIDs []string
	}{
		{"populated entity matches YAML", "user-x", []string{"group-y"}},
		{"populated entity no match", "user-z", []string{"group-z"}},
		{"empty entity, populated groups", "", []string{"group-y"}},
		{"empty both", "", nil},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			roleIDs, err := suite.store.GetEntityRoleIDs(context.Background(), tc.entityID, tc.groupIDs)
			suite.NoError(err)
			suite.Empty(roleIDs)
			suite.NotNil(roleIDs, "must return [] not nil for safe composite union")
		})
	}
}
