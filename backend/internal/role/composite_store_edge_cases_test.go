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
	"errors"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

// CompositeRoleStoreEdgeCaseTestSuite contains edge case tests for the composite role store.
type CompositeRoleStoreEdgeCaseTestSuite struct {
	suite.Suite
	mockDBStore   *roleStoreInterfaceMock
	mockFileStore *roleStoreInterfaceMock
	store         roleStoreInterface
	ctx           context.Context
}

func TestCompositeRoleStoreEdgeCaseTestSuite(t *testing.T) {
	suite.Run(t, new(CompositeRoleStoreEdgeCaseTestSuite))
}

func (suite *CompositeRoleStoreEdgeCaseTestSuite) SetupTest() {
	suite.mockDBStore = newRoleStoreInterfaceMock(suite.T())
	suite.mockFileStore = newRoleStoreInterfaceMock(suite.T())
	suite.store = newCompositeRoleStore(suite.mockFileStore, suite.mockDBStore)
	suite.ctx = context.Background()
}

// Test CreateRole delegates to database store
func (suite *CompositeRoleStoreEdgeCaseTestSuite) TestCreateRole_DelegatesToDB() {
	suite.mockDBStore.On("CreateRole", suite.ctx, "role1", mock.Anything).Return(nil)

	err := suite.store.CreateRole(suite.ctx, "role1", RoleCreationDetail{
		Name: "Test",
		OUID: "ou1",
	})

	assert.NoError(suite.T(), err)
	suite.mockDBStore.AssertExpectations(suite.T())
	suite.mockFileStore.AssertNotCalled(suite.T(), "CreateRole")
}

// Test GetRole from DB when found
func (suite *CompositeRoleStoreEdgeCaseTestSuite) TestGetRole_FromDB() {
	expectedRole := RoleWithPermissions{
		ID:   "role1",
		Name: "Admin",
	}
	suite.mockDBStore.On("GetRole", suite.ctx, "role1").Return(expectedRole, nil)

	result, err := suite.store.GetRole(suite.ctx, "role1")

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), expectedRole, result)
	suite.mockDBStore.AssertExpectations(suite.T())
	suite.mockFileStore.AssertNotCalled(suite.T(), "GetRole")
}

// Test GetRole falls back to file store when not in DB
func (suite *CompositeRoleStoreEdgeCaseTestSuite) TestGetRole_FallbackToFile() {
	expectedRole := RoleWithPermissions{
		ID:   "role1",
		Name: "Admin",
	}
	suite.mockDBStore.On("GetRole", suite.ctx, "role1").Return(RoleWithPermissions{}, ErrRoleNotFound)
	suite.mockFileStore.On("GetRole", suite.ctx, "role1").Return(expectedRole, nil)

	result, err := suite.store.GetRole(suite.ctx, "role1")

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), expectedRole, result)
	suite.mockDBStore.AssertExpectations(suite.T())
	suite.mockFileStore.AssertExpectations(suite.T())
}

// Test GetRole returns DB error when not found in either store
func (suite *CompositeRoleStoreEdgeCaseTestSuite) TestGetRole_NotFound() {
	suite.mockDBStore.On("GetRole", suite.ctx, "nonexistent").Return(RoleWithPermissions{}, ErrRoleNotFound)
	suite.mockFileStore.On("GetRole", suite.ctx, "nonexistent").Return(RoleWithPermissions{}, ErrRoleNotFound)

	result, err := suite.store.GetRole(suite.ctx, "nonexistent")

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), RoleWithPermissions{}, result)
}

// Test GetRole returns DB error when DB has error other than not found
func (suite *CompositeRoleStoreEdgeCaseTestSuite) TestGetRole_DBError() {
	dbErr := errors.New("database connection error")
	suite.mockDBStore.On("GetRole", suite.ctx, "role1").Return(RoleWithPermissions{}, dbErr)

	result, err := suite.store.GetRole(suite.ctx, "role1")

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), dbErr, err)
	assert.Equal(suite.T(), RoleWithPermissions{}, result)
	suite.mockFileStore.AssertNotCalled(suite.T(), "GetRole")
}

// Test UpdateRole delegates to database store
func (suite *CompositeRoleStoreEdgeCaseTestSuite) TestUpdateRole_DelegatesToDB() {
	suite.mockDBStore.On("UpdateRole", suite.ctx, "role1", mock.Anything).Return(nil)

	err := suite.store.UpdateRole(suite.ctx, "role1", RoleUpdateDetail{
		Name: "Updated",
	})

	assert.NoError(suite.T(), err)
	suite.mockDBStore.AssertExpectations(suite.T())
	suite.mockFileStore.AssertNotCalled(suite.T(), "UpdateRole")
}

// Test DeleteRole delegates to database store
func (suite *CompositeRoleStoreEdgeCaseTestSuite) TestDeleteRole_DelegatesToDB() {
	suite.mockDBStore.On("DeleteRole", suite.ctx, "role1").Return(nil)

	err := suite.store.DeleteRole(suite.ctx, "role1")

	assert.NoError(suite.T(), err)
	suite.mockDBStore.AssertExpectations(suite.T())
	suite.mockFileStore.AssertNotCalled(suite.T(), "DeleteRole")
}

// Test AddAssignments delegates to database store
func (suite *CompositeRoleStoreEdgeCaseTestSuite) TestAddAssignments_DelegatesToDB() {
	suite.mockDBStore.On("AddAssignments", suite.ctx, "role1", mock.Anything).Return(nil)

	err := suite.store.AddAssignments(suite.ctx, "role1", []RoleAssignment{
		{ID: "user1", Type: assigneeTypeEntity},
	})

	assert.NoError(suite.T(), err)
	suite.mockDBStore.AssertExpectations(suite.T())
}

// Test RemoveAssignments delegates to database store
func (suite *CompositeRoleStoreEdgeCaseTestSuite) TestRemoveAssignments_DelegatesToDB() {
	suite.mockDBStore.On("RemoveAssignments", suite.ctx, "role1", mock.Anything).Return(nil)

	err := suite.store.RemoveAssignments(suite.ctx, "role1", []RoleAssignment{
		{ID: "user1", Type: assigneeTypeEntity},
	})

	assert.NoError(suite.T(), err)
	suite.mockDBStore.AssertExpectations(suite.T())
}

// Test CheckRoleNameExists checks file store first, returns true if found
func (suite *CompositeRoleStoreEdgeCaseTestSuite) TestCheckRoleNameExists_ChecksBothStores() {
	// CompositeBooleanCheckHelper checks fileStore first. If it returns true, it stops.
	suite.mockFileStore.On("CheckRoleNameExists", suite.ctx, "ou1", "Admin").Return(true, nil)
	// DBStore should not be called since fileStore returns true

	exists, err := suite.store.CheckRoleNameExists(suite.ctx, "ou1", "Admin")

	assert.NoError(suite.T(), err)
	assert.True(suite.T(), exists)
	suite.mockFileStore.AssertExpectations(suite.T())
	suite.mockDBStore.AssertNotCalled(suite.T(), "CheckRoleNameExists")
}

// Test CheckRoleNameExists returns true if found in DB
func (suite *CompositeRoleStoreEdgeCaseTestSuite) TestCheckRoleNameExists_FoundInDB() {
	suite.mockDBStore.On("CheckRoleNameExists", suite.ctx, "ou1", "Admin").Return(true, nil)
	suite.mockFileStore.On("CheckRoleNameExists", suite.ctx, "ou1", "Admin").Return(false, nil)

	exists, err := suite.store.CheckRoleNameExists(suite.ctx, "ou1", "Admin")

	assert.NoError(suite.T(), err)
	assert.True(suite.T(), exists)
}

// Test CheckRoleNameExists returns true if found in file store
func (suite *CompositeRoleStoreEdgeCaseTestSuite) TestCheckRoleNameExists_FoundInFile() {
	// FileStore returns true, so DBStore is not called
	suite.mockFileStore.On("CheckRoleNameExists", suite.ctx, "ou1", "Admin").Return(true, nil)

	exists, err := suite.store.CheckRoleNameExists(suite.ctx, "ou1", "Admin")

	assert.NoError(suite.T(), err)
	assert.True(suite.T(), exists)
	suite.mockFileStore.AssertExpectations(suite.T())
	suite.mockDBStore.AssertNotCalled(suite.T(), "CheckRoleNameExists")
}

// Test CheckRoleNameExistsExcludingID checks both stores
func (suite *CompositeRoleStoreEdgeCaseTestSuite) TestCheckRoleNameExistsExcludingID_ChecksBothStores() {
	suite.mockDBStore.On("CheckRoleNameExistsExcludingID", suite.ctx, "ou1", "Admin", "role1").Return(false, nil)
	suite.mockFileStore.On("CheckRoleNameExistsExcludingID", suite.ctx, "ou1", "Admin", "role1").Return(false, nil)

	exists, err := suite.store.CheckRoleNameExistsExcludingID(suite.ctx, "ou1", "Admin", "role1")

	assert.NoError(suite.T(), err)
	assert.False(suite.T(), exists)
}

// Test IsRoleExist checks file store first (uses CompositeBooleanCheckHelper)
func (suite *CompositeRoleStoreEdgeCaseTestSuite) TestIsRoleExist_ChecksBothStores() {
	// CompositeBooleanCheckHelper checks fileStore first. If true, returns without checking dbStore.
	suite.mockFileStore.On("IsRoleExist", suite.ctx, "role1").Return(true, nil)

	exists, err := suite.store.IsRoleExist(suite.ctx, "role1")

	assert.NoError(suite.T(), err)
	assert.True(suite.T(), exists)
	suite.mockFileStore.AssertExpectations(suite.T())
	suite.mockDBStore.AssertNotCalled(suite.T(), "IsRoleExist")
}

// Test IsRoleExist returns true if found in DB
func (suite *CompositeRoleStoreEdgeCaseTestSuite) TestIsRoleExist_FoundInDB() {
	suite.mockDBStore.On("IsRoleExist", suite.ctx, "role1").Return(true, nil)
	suite.mockFileStore.On("IsRoleExist", suite.ctx, "role1").Return(false, nil)

	exists, err := suite.store.IsRoleExist(suite.ctx, "role1")

	assert.NoError(suite.T(), err)
	assert.True(suite.T(), exists)
}

// Test IsRoleExist returns true if found in file store
func (suite *CompositeRoleStoreEdgeCaseTestSuite) TestIsRoleExist_FoundInFile() {
	// FileStore returns true, so DBStore is not called
	suite.mockFileStore.On("IsRoleExist", suite.ctx, "role1").Return(true, nil)

	exists, err := suite.store.IsRoleExist(suite.ctx, "role1")

	assert.NoError(suite.T(), err)
	assert.True(suite.T(), exists)
	suite.mockFileStore.AssertExpectations(suite.T())
	suite.mockDBStore.AssertNotCalled(suite.T(), "IsRoleExist")
}

// Test GetRoleListCount merges and deduplicates
func (suite *CompositeRoleStoreEdgeCaseTestSuite) TestGetRoleListCount_MergesAndDeduplicates() {
	dbRoles := []Role{
		{ID: "role1", Name: "Admin"},
		{ID: "role2", Name: "Editor"},
	}
	fileRoles := []Role{
		{ID: "role2", Name: "Editor"},
		{ID: "role3", Name: "Viewer"},
	}

	// GetRoleListCount first calls GetRoleListCount on both stores
	suite.mockDBStore.On("GetRoleListCount", suite.ctx).Return(2, nil)
	suite.mockFileStore.On("GetRoleListCount", suite.ctx).Return(3, nil)
	// Then calls GetRoleList with the counts as limits and 0 offset
	suite.mockDBStore.On("GetRoleList", suite.ctx, 2, 0).Return(dbRoles, nil)
	suite.mockFileStore.On("GetRoleList", suite.ctx, 3, 0).Return(fileRoles, nil)

	count, err := suite.store.GetRoleListCount(suite.ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), 3, count)
}

// Test GetRoleList merges and applies pagination
func (suite *CompositeRoleStoreEdgeCaseTestSuite) TestGetRoleList_MergesAndPaginates() {
	dbRoles := []Role{
		{ID: "role1", Name: "Admin"},
		{ID: "role2", Name: "Editor"},
	}
	fileRoles := []Role{
		{ID: "role3", Name: "Viewer"},
		{ID: "role4", Name: "Guest"},
	}

	// GetRoleList calls GetRoleListCount first, then GetRoleList with the counts
	suite.mockDBStore.On("GetRoleListCount", suite.ctx).Return(2, nil)
	suite.mockFileStore.On("GetRoleListCount", suite.ctx).Return(2, nil)
	suite.mockDBStore.On("GetRoleList", suite.ctx, 2, 0).Return(dbRoles, nil)
	suite.mockFileStore.On("GetRoleList", suite.ctx, 2, 0).Return(fileRoles, nil)

	// Test page 1
	result, err := suite.store.GetRoleList(suite.ctx, 2, 0)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), result, 2)

	// For the second page test, need fresh mock setup
	suite.mockDBStore.On("GetRoleListCount", suite.ctx).Return(2, nil)
	suite.mockFileStore.On("GetRoleListCount", suite.ctx).Return(2, nil)
	suite.mockDBStore.On("GetRoleList", suite.ctx, 2, 0).Return(dbRoles, nil)
	suite.mockFileStore.On("GetRoleList", suite.ctx, 2, 0).Return(fileRoles, nil)

	// Test page 2
	result, err = suite.store.GetRoleList(suite.ctx, 2, 2)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), result, 2)
}

// Test GetRoleList returns empty when offset exceeds results
func (suite *CompositeRoleStoreEdgeCaseTestSuite) TestGetRoleList_OffsetBeyondResults() {
	suite.mockDBStore.On("GetRoleListCount", suite.ctx).Return(1, nil)
	suite.mockFileStore.On("GetRoleListCount", suite.ctx).Return(0, nil)
	// When offset (100) exceeds effectiveTotal (1), the implementation short-circuits
	// and does not call GetRoleList on either store.

	result, err := suite.store.GetRoleList(suite.ctx, 10, 100)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), result, 0)
	suite.mockDBStore.AssertNotCalled(suite.T(), "GetRoleList", mock.Anything, mock.Anything, mock.Anything)
	suite.mockFileStore.AssertNotCalled(suite.T(), "GetRoleList", mock.Anything, mock.Anything, mock.Anything)
}

// Test GetRoleAssignmentsCount merges and deduplicates
func (suite *CompositeRoleStoreEdgeCaseTestSuite) TestGetRoleAssignmentsCount_MergesAndDeduplicates() {
	dbAssignments := []RoleAssignment{
		{ID: "user1", Type: assigneeTypeEntity},
		{ID: "group1", Type: AssigneeTypeGroup},
		{ID: "user2", Type: assigneeTypeEntity},
	}
	fileAssignments := []RoleAssignment{
		{ID: "user2", Type: assigneeTypeEntity},
		{ID: "group2", Type: AssigneeTypeGroup},
	}

	// GetRoleAssignmentsCount calls GetRoleAssignmentsCount first
	suite.mockDBStore.On("GetRoleAssignmentsCount", suite.ctx, "role1").Return(3, nil)
	suite.mockFileStore.On("GetRoleAssignmentsCount", suite.ctx, "role1").Return(2, nil)
	// Then calls GetRoleAssignments with the counts
	suite.mockDBStore.On("GetRoleAssignments", suite.ctx, "role1", 3, 0).Return(dbAssignments, nil)
	suite.mockFileStore.On("GetRoleAssignments", suite.ctx, "role1", 2, 0).Return(fileAssignments, nil)

	count, err := suite.store.GetRoleAssignmentsCount(suite.ctx, "role1")

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), 4, count)
}

// Test GetRoleAssignments merges and applies pagination
func (suite *CompositeRoleStoreEdgeCaseTestSuite) TestGetRoleAssignments_MergesAndPaginates() {
	dbAssignments := []RoleAssignment{
		{ID: "user1", Type: assigneeTypeEntity},
		{ID: "user2", Type: assigneeTypeEntity},
	}
	fileAssignments := []RoleAssignment{
		{ID: "group1", Type: AssigneeTypeGroup},
		{ID: "group2", Type: AssigneeTypeGroup},
	}

	// GetRoleAssignments calls GetRoleAssignmentsCount first
	suite.mockDBStore.On("GetRoleAssignmentsCount", suite.ctx, "role1").Return(2, nil)
	suite.mockFileStore.On("GetRoleAssignmentsCount", suite.ctx, "role1").Return(2, nil)
	// Then calls GetRoleAssignments with the counts
	suite.mockDBStore.On("GetRoleAssignments", suite.ctx, "role1", 2, 0).Return(dbAssignments, nil)
	suite.mockFileStore.On("GetRoleAssignments", suite.ctx, "role1", 2, 0).Return(fileAssignments, nil)

	result, err := suite.store.GetRoleAssignments(suite.ctx, "role1", 2, 1)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), result, 2)
}

// Test IsRoleDeclarative checks file store
func (suite *CompositeRoleStoreEdgeCaseTestSuite) TestIsRoleDeclarative_ChecksFileStore() {
	suite.mockFileStore.On("IsRoleExist", suite.ctx, "role1").Return(true, nil)

	isDeclarative, err := suite.store.IsRoleDeclarative(suite.ctx, "role1")

	assert.NoError(suite.T(), err)
	assert.True(suite.T(), isDeclarative)
}

// Test IsRoleDeclarative returns false for non-existent role
func (suite *CompositeRoleStoreEdgeCaseTestSuite) TestIsRoleDeclarative_NonExistent() {
	suite.mockFileStore.On("IsRoleExist", suite.ctx, "nonexistent").Return(false, nil)

	isDeclarative, err := suite.store.IsRoleDeclarative(suite.ctx, "nonexistent")

	assert.NoError(suite.T(), err)
	assert.False(suite.T(), isDeclarative)
}

// Test GetAuthorizedPermissions checks both stores
func (suite *CompositeRoleStoreEdgeCaseTestSuite) TestGetAuthorizedPermissions_ChecksBothStores() {
	perms := []string{"perm1", "perm2"}
	suite.mockDBStore.On(
		"GetAuthorizedPermissionsByResourceServer", suite.ctx, "user1", []string{"group1"}, "", perms,
	).Return([]string{"perm1"}, nil)
	suite.mockFileStore.On(
		"GetAuthorizedPermissionsByResourceServer", suite.ctx, "user1", []string{"group1"}, "", perms,
	).Return([]string{"perm1", "perm2"}, nil)
	// Cross-store lookup: no DB-recorded role IDs to fold in.
	suite.mockDBStore.On(
		"GetEntityRoleIDs", suite.ctx, "user1", []string{"group1"},
	).Return([]string{}, nil)

	result, err := suite.store.GetAuthorizedPermissionsByResourceServer(
		suite.ctx, "user1", []string{"group1"}, "",
		perms)

	assert.NoError(suite.T(), err)
	expected := []string{"perm1", "perm2"}
	// Sort both slices to make the comparison order-insensitive
	sort.Strings(expected)
	sort.Strings(result)
	assert.Equal(suite.T(), expected, result)
}

// Test GetAuthorizedPermissions merges permissions from both stores (union)
func (suite *CompositeRoleStoreEdgeCaseTestSuite) TestGetAuthorizedPermissions_CommonPermissions() {
	perms := []string{"p1", "p2", "p3"}
	suite.mockDBStore.On(
		"GetAuthorizedPermissionsByResourceServer", suite.ctx, "user1", []string{"group1"}, "", perms,
	).Return([]string{"p1", "p2"}, nil)
	suite.mockFileStore.On(
		"GetAuthorizedPermissionsByResourceServer", suite.ctx, "user1", []string{"group1"}, "", perms,
	).Return([]string{"p2", "p3"}, nil)
	suite.mockDBStore.On(
		"GetEntityRoleIDs", suite.ctx, "user1", []string{"group1"},
	).Return([]string{}, nil)

	result, err := suite.store.GetAuthorizedPermissionsByResourceServer(
		suite.ctx, "user1", []string{"group1"}, "",
		perms)

	assert.NoError(suite.T(), err)
	// mergePermissions returns union of all unique permissions
	assert.Len(suite.T(), result, 3)
	assert.Contains(suite.T(), result, "p1")
	assert.Contains(suite.T(), result, "p2")
	assert.Contains(suite.T(), result, "p3")
}

// Test GetAuthorizedPermissions with empty result
func (suite *CompositeRoleStoreEdgeCaseTestSuite) TestGetAuthorizedPermissions_EmptyResult() {
	perms := []string{"perm1"}
	suite.mockDBStore.On(
		"GetAuthorizedPermissionsByResourceServer", suite.ctx, "user1", []string{}, "", perms,
	).Return([]string{}, nil)
	suite.mockFileStore.On(
		"GetAuthorizedPermissionsByResourceServer", suite.ctx, "user1", []string{}, "", perms,
	).Return([]string{}, nil)
	suite.mockDBStore.On(
		"GetEntityRoleIDs", suite.ctx, "user1", []string{},
	).Return([]string{}, nil)

	result, err := suite.store.GetAuthorizedPermissionsByResourceServer(
		suite.ctx, "user1", []string{}, "",
		perms)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), result, 0)
}

// Test GetAuthorizedPermissions resolves a declarative role whose definition lives in the
// file store and whose assignment lives in the DB (the bug fixed by this change).
// dbStore.GetAuthorizedPermissions returns nothing because there are no DB-side ROLE_PERMISSION
// rows for the declarative role; fileStore.GetAuthorizedPermissions returns nothing because the
// YAML carries no static assignments. The composite must fold in the role's file-store
// permissions by following the DB-recorded role ID assignment.
func (suite *CompositeRoleStoreEdgeCaseTestSuite) TestGetAuthorizedPermissions_DeclarativeRoleWithDynamicAssignment() {
	perms := []string{"tenant_instance:system", "system:user:view"}
	declarativeRoleID := "a1c00000-0000-0000-0000-000000000004"

	suite.mockDBStore.On(
		"GetAuthorizedPermissionsByResourceServer", suite.ctx, "user1", []string{}, "", perms,
	).Return([]string{}, nil)
	suite.mockFileStore.On(
		"GetAuthorizedPermissionsByResourceServer", suite.ctx, "user1", []string{}, "", perms,
	).Return([]string{}, nil)
	suite.mockDBStore.On(
		"GetEntityRoleIDs", suite.ctx, "user1", []string{},
	).Return([]string{declarativeRoleID}, nil)
	suite.mockFileStore.On(
		"IsRoleExist", suite.ctx, declarativeRoleID,
	).Return(true, nil)
	suite.mockFileStore.On(
		"GetRole", suite.ctx, declarativeRoleID,
	).Return(RoleWithPermissions{
		ID: declarativeRoleID, Name: "TenantInstanceAdmin",
		Permissions: []ResourcePermissions{
			{ResourceServerID: "rs", Permissions: []string{"tenant_instance:system"}},
		},
	}, nil)

	result, err := suite.store.GetAuthorizedPermissionsByResourceServer(
		suite.ctx, "user1", []string{}, "",
		perms)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), []string{"tenant_instance:system"}, result)
}

// Test that the cross-store lookup ignores role IDs that exist only in the DB. Those
// roles' permissions are already covered by dbStore.GetAuthorizedPermissions, and looking
// them up in the file store would be wasted work.
func (suite *CompositeRoleStoreEdgeCaseTestSuite) TestGetAuthorizedPermissions_DBRoleNotDoubleResolved() {
	perms := []string{"perm1"}
	dbOnlyRoleID := "db-role-only"

	suite.mockDBStore.On(
		"GetAuthorizedPermissionsByResourceServer", suite.ctx, "user1", []string{}, "", perms,
	).Return([]string{"perm1"}, nil)
	suite.mockFileStore.On(
		"GetAuthorizedPermissionsByResourceServer", suite.ctx, "user1", []string{}, "", perms,
	).Return([]string{}, nil)
	suite.mockDBStore.On(
		"GetEntityRoleIDs", suite.ctx, "user1", []string{},
	).Return([]string{dbOnlyRoleID}, nil)
	suite.mockFileStore.On(
		"IsRoleExist", suite.ctx, dbOnlyRoleID,
	).Return(false, nil)
	// fileStore.GetRole MUST NOT be called for DB-only roles.

	result, err := suite.store.GetAuthorizedPermissionsByResourceServer(
		suite.ctx, "user1", []string{}, "",
		perms)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), []string{"perm1"}, result)
	suite.mockFileStore.AssertNotCalled(suite.T(), "GetRole", mock.Anything, dbOnlyRoleID)
}

// Test that an unexpected storage error from fileStore.GetRole in the cross-store path is
// propagated (wrapped) rather than silently swallowed. Silently dropping a real I/O error
// here would yield an empty permission set and could lead to under-authorization decisions.
func (suite *CompositeRoleStoreEdgeCaseTestSuite) TestGetAuthorizedPermissions_CrossStorePropagatesGetRoleError() {
	requested := []string{"perm1"}
	roleID := "declarative-role"
	storageErr := errors.New("disk read failure")

	suite.mockDBStore.On(
		"GetAuthorizedPermissionsByResourceServer", suite.ctx, "user1", []string{}, "", requested,
	).Return([]string{}, nil)
	suite.mockFileStore.On(
		"GetAuthorizedPermissionsByResourceServer", suite.ctx, "user1", []string{}, "", requested,
	).Return([]string{}, nil)
	suite.mockDBStore.On(
		"GetEntityRoleIDs", suite.ctx, "user1", []string{},
	).Return([]string{roleID}, nil)
	suite.mockFileStore.On(
		"IsRoleExist", suite.ctx, roleID,
	).Return(true, nil)
	suite.mockFileStore.On(
		"GetRole", suite.ctx, roleID,
	).Return(RoleWithPermissions{}, storageErr)

	result, err := suite.store.GetAuthorizedPermissionsByResourceServer(suite.ctx, "user1", []string{}, "", requested)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.ErrorIs(suite.T(), err, storageErr, "underlying storage error must be wrapped, not dropped")
}

// Test that benign GetRole errors from fileStore are silently skipped:
//   - ErrRoleNotFound: YAML removed between assignment-time and lookup-time (race).
//   - ErrRoleDataCorrupted: parse/type-assertion failure already logged by fileStore.
//
// Both cases must not bubble up; the cross-store path treats the role as absent and
// returns an empty result without error.
func (suite *CompositeRoleStoreEdgeCaseTestSuite) TestGetAuthorizedPermissions_CrossStoreSkipsBenignGetRoleErrors() {
	cases := []struct {
		name string
		err  error
	}{
		{"ErrRoleNotFound", ErrRoleNotFound},
		{"ErrRoleDataCorrupted", ErrRoleDataCorrupted},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			suite.SetupTest() // fresh mocks per subtest
			requested := []string{"perm1"}
			roleID := "role-" + tc.name

			suite.mockDBStore.On(
				"GetAuthorizedPermissionsByResourceServer", suite.ctx, "user1", []string{}, "", requested,
			).Return([]string{}, nil)
			suite.mockFileStore.On(
				"GetAuthorizedPermissionsByResourceServer", suite.ctx, "user1", []string{}, "", requested,
			).Return([]string{}, nil)
			suite.mockDBStore.On(
				"GetEntityRoleIDs", suite.ctx, "user1", []string{},
			).Return([]string{roleID}, nil)
			suite.mockFileStore.On(
				"IsRoleExist", suite.ctx, roleID,
			).Return(true, nil)
			suite.mockFileStore.On(
				"GetRole", suite.ctx, roleID,
			).Return(RoleWithPermissions{}, tc.err)

			result, err := suite.store.GetAuthorizedPermissionsByResourceServer(
				suite.ctx, "user1", []string{}, "",
				requested)

			assert.NoError(suite.T(), err)
			assert.Empty(suite.T(), result)
		})
	}
}

// Test that the cross-store lookup respects requestPermissions narrowing: permissions on
// the role that aren't in the request are silently dropped.
func (suite *CompositeRoleStoreEdgeCaseTestSuite) TestGetAuthorizedPermissions_CrossStoreRespectsRequestedSet() {
	requested := []string{"tenant_instance:system"}
	roleID := "declarative-role"

	suite.mockDBStore.On(
		"GetAuthorizedPermissionsByResourceServer", suite.ctx, "user1", []string{}, "", requested,
	).Return([]string{}, nil)
	suite.mockFileStore.On(
		"GetAuthorizedPermissionsByResourceServer", suite.ctx, "user1", []string{}, "", requested,
	).Return([]string{}, nil)
	suite.mockDBStore.On(
		"GetEntityRoleIDs", suite.ctx, "user1", []string{},
	).Return([]string{roleID}, nil)
	suite.mockFileStore.On(
		"IsRoleExist", suite.ctx, roleID,
	).Return(true, nil)
	suite.mockFileStore.On(
		"GetRole", suite.ctx, roleID,
	).Return(RoleWithPermissions{
		ID: roleID,
		Permissions: []ResourcePermissions{{
			ResourceServerID: "rs",
			Permissions:      []string{"tenant_instance:system", "system:user:view", "something_else"},
		}},
	}, nil)

	result, err := suite.store.GetAuthorizedPermissionsByResourceServer(suite.ctx, "user1", []string{}, "", requested)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), []string{"tenant_instance:system"}, result)
}

// Test DB precedence in deduplication
func (suite *CompositeRoleStoreEdgeCaseTestSuite) TestMergeAndDeduplicateRoles_DBPrecedence() {
	dbRoles := []Role{
		{ID: "role1", Name: "AdminDB"},
	}
	fileRoles := []Role{
		{ID: "role1", Name: "AdminFile"},
	}

	suite.mockDBStore.On("GetRoleListCount", suite.ctx).Return(1, nil)
	suite.mockFileStore.On("GetRoleListCount", suite.ctx).Return(1, nil)
	suite.mockDBStore.On("GetRoleList", suite.ctx, 1, 0).Return(dbRoles, nil)
	suite.mockFileStore.On("GetRoleList", suite.ctx, 1, 0).Return(fileRoles, nil)

	result, err := suite.store.GetRoleList(suite.ctx, 10, 0)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), result, 1)
	assert.Equal(suite.T(), "AdminDB", result[0].Name)
}

// Test DB error propagation in GetRoleList
func (suite *CompositeRoleStoreEdgeCaseTestSuite) TestGetRoleList_PropagatesDBError() {
	dbErr := errors.New("database error")
	suite.mockDBStore.On("GetRoleListCount", suite.ctx).Return(0, dbErr)

	result, err := suite.store.GetRoleList(suite.ctx, 10, 0)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), dbErr, err)
}

// Test file store error propagation in GetRoleList
func (suite *CompositeRoleStoreEdgeCaseTestSuite) TestGetRoleList_PropagatesFileError() {
	fileErr := errors.New("file store error")
	suite.mockDBStore.On("GetRoleListCount", suite.ctx).Return(1, nil)
	suite.mockFileStore.On("GetRoleListCount", suite.ctx).Return(0, fileErr)

	result, err := suite.store.GetRoleList(suite.ctx, 10, 0)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), fileErr, err)
}

// Test GetEntityRoleIDs delegates directly to the DB store; the file store is never
// consulted (assignments are never persisted in YAML).
func (suite *CompositeRoleStoreEdgeCaseTestSuite) TestGetEntityRoleIDs_DelegatesToDB() {
	expected := []string{"role-a", "role-b"}
	suite.mockDBStore.On(
		"GetEntityRoleIDs", suite.ctx, "user1", []string{"group1"},
	).Return(expected, nil)

	result, err := suite.store.GetEntityRoleIDs(suite.ctx, "user1", []string{"group1"})

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), expected, result)
	suite.mockFileStore.AssertNotCalled(suite.T(), "GetEntityRoleIDs", mock.Anything, mock.Anything, mock.Anything)
}

// Test GetEntityRoleIDs propagates DB errors.
func (suite *CompositeRoleStoreEdgeCaseTestSuite) TestGetEntityRoleIDs_PropagatesDBError() {
	dbErr := errors.New("db error")
	suite.mockDBStore.On(
		"GetEntityRoleIDs", suite.ctx, "user1", []string{},
	).Return(nil, dbErr)

	result, err := suite.store.GetEntityRoleIDs(suite.ctx, "user1", []string{})

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), dbErr, err)
}

// Test that GetAuthorizedPermissions short-circuits when no permissions are requested.
// Without this, every downstream store would be queried even though the result is
// guaranteed to be empty.
func (suite *CompositeRoleStoreEdgeCaseTestSuite) TestGetAuthorizedPermissions_EmptyRequestedShortCircuits() {
	result, err := suite.store.GetAuthorizedPermissionsByResourceServer(
		suite.ctx, "user1", []string{"group1"}, "", []string{})

	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), result)
	// Neither store should have been called.
	suite.mockDBStore.AssertNotCalled(
		suite.T(), "GetAuthorizedPermissionsByResourceServer",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	)
	suite.mockFileStore.AssertNotCalled(
		suite.T(), "GetAuthorizedPermissionsByResourceServer",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	)
	suite.mockDBStore.AssertNotCalled(
		suite.T(), "GetEntityRoleIDs", mock.Anything, mock.Anything, mock.Anything,
	)
}

// Test GetAuthorizedPermissions propagates a DB store error from the first call.
func (suite *CompositeRoleStoreEdgeCaseTestSuite) TestGetAuthorizedPermissions_PropagatesDBStoreError() {
	dbErr := errors.New("db unreachable")
	requested := []string{"perm1"}
	suite.mockDBStore.On(
		"GetAuthorizedPermissionsByResourceServer", suite.ctx, "user1", []string{}, "", requested,
	).Return(nil, dbErr)

	result, err := suite.store.GetAuthorizedPermissionsByResourceServer(suite.ctx, "user1", []string{}, "", requested)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), dbErr, err)
}

// Test GetAuthorizedPermissions propagates a file store error from the second call.
func (suite *CompositeRoleStoreEdgeCaseTestSuite) TestGetAuthorizedPermissions_PropagatesFileStoreError() {
	fileErr := errors.New("file read failure")
	requested := []string{"perm1"}
	suite.mockDBStore.On(
		"GetAuthorizedPermissionsByResourceServer", suite.ctx, "user1", []string{}, "", requested,
	).Return([]string{}, nil)
	suite.mockFileStore.On(
		"GetAuthorizedPermissionsByResourceServer", suite.ctx, "user1", []string{}, "", requested,
	).Return(nil, fileErr)

	result, err := suite.store.GetAuthorizedPermissionsByResourceServer(suite.ctx, "user1", []string{}, "", requested)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), fileErr, err)
}

// Test the cross-store path short-circuits when neither entity nor groups are supplied.
// The first two source paths might still return data based on store-internal logic, but
// the cross-store path itself has nothing to look up.
func (suite *CompositeRoleStoreEdgeCaseTestSuite) TestGetAuthorizedPermissions_CrossStoreNoEntityNoGroups() {
	requested := []string{"perm1"}
	suite.mockDBStore.On(
		"GetAuthorizedPermissionsByResourceServer", suite.ctx, "", []string{}, "", requested,
	).Return([]string{}, nil)
	suite.mockFileStore.On(
		"GetAuthorizedPermissionsByResourceServer", suite.ctx, "", []string{}, "", requested,
	).Return([]string{}, nil)
	// Cross-store path must not call GetEntityRoleIDs when there's no assignee to look up.

	result, err := suite.store.GetAuthorizedPermissionsByResourceServer(suite.ctx, "", []string{}, "", requested)

	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), result)
	suite.mockDBStore.AssertNotCalled(
		suite.T(), "GetEntityRoleIDs", mock.Anything, mock.Anything, mock.Anything,
	)
}

// Test cross-store path propagates GetEntityRoleIDs errors.
func (suite *CompositeRoleStoreEdgeCaseTestSuite) TestGetAuthorizedPermissions_CrossStoreRoleIDsErrorPropagates() {
	requested := []string{"perm1"}
	rolesErr := errors.New("assignment table unreachable")
	suite.mockDBStore.On(
		"GetAuthorizedPermissionsByResourceServer", suite.ctx, "user1", []string{}, "", requested,
	).Return([]string{}, nil)
	suite.mockFileStore.On(
		"GetAuthorizedPermissionsByResourceServer", suite.ctx, "user1", []string{}, "", requested,
	).Return([]string{}, nil)
	suite.mockDBStore.On(
		"GetEntityRoleIDs", suite.ctx, "user1", []string{},
	).Return(nil, rolesErr)

	result, err := suite.store.GetAuthorizedPermissionsByResourceServer(suite.ctx, "user1", []string{}, "", requested)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), rolesErr, err)
}

// Test cross-store path propagates a file-store IsRoleExist error so token issuance
// fails loudly instead of silently dropping a role.
func (suite *CompositeRoleStoreEdgeCaseTestSuite) TestGetAuthorizedPermissions_CrossStorePropagatesIsRoleExistError() {
	requested := []string{"perm1"}
	existErr := errors.New("file lookup failure")
	roleID := "some-role"
	suite.mockDBStore.On(
		"GetAuthorizedPermissionsByResourceServer", suite.ctx, "user1", []string{}, "", requested,
	).Return([]string{}, nil)
	suite.mockFileStore.On(
		"GetAuthorizedPermissionsByResourceServer", suite.ctx, "user1", []string{}, "", requested,
	).Return([]string{}, nil)
	suite.mockDBStore.On(
		"GetEntityRoleIDs", suite.ctx, "user1", []string{},
	).Return([]string{roleID}, nil)
	suite.mockFileStore.On(
		"IsRoleExist", suite.ctx, roleID,
	).Return(false, existErr)

	result, err := suite.store.GetAuthorizedPermissionsByResourceServer(suite.ctx, "user1", []string{}, "", requested)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), existErr, err)
}
