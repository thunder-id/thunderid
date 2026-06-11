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

package group

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

// CompositeGroupStoreEdgeCaseTestSuite contains edge case tests for the composite group store.
type CompositeGroupStoreEdgeCaseTestSuite struct {
	suite.Suite
	mockDBStore   *groupStoreInterfaceMock
	mockFileStore *groupStoreInterfaceMock
	store         groupStoreInterface
	ctx           context.Context
}

func TestCompositeGroupStoreEdgeCaseTestSuite(t *testing.T) {
	suite.Run(t, new(CompositeGroupStoreEdgeCaseTestSuite))
}

func (suite *CompositeGroupStoreEdgeCaseTestSuite) SetupTest() {
	suite.mockDBStore = newGroupStoreInterfaceMock(suite.T())
	suite.mockFileStore = newGroupStoreInterfaceMock(suite.T())
	suite.store = newCompositeGroupStore(suite.mockFileStore, suite.mockDBStore)
	suite.ctx = context.Background()
}

// Test CreateGroup delegates to database store only.
func (suite *CompositeGroupStoreEdgeCaseTestSuite) TestCreateGroup_DelegatesToDB() {
	group := GroupDAO{ID: "grp1", Name: "Admins", OUID: "ou1"}
	suite.mockDBStore.On("CreateGroup", suite.ctx, group).Return(nil)

	err := suite.store.CreateGroup(suite.ctx, group)

	assert.NoError(suite.T(), err)
	suite.mockDBStore.AssertExpectations(suite.T())
	suite.mockFileStore.AssertNotCalled(suite.T(), "CreateGroup")
}

// Test GetGroup retrieves from DB when found.
func (suite *CompositeGroupStoreEdgeCaseTestSuite) TestGetGroup_FromDB() {
	expected := GroupDAO{ID: "grp1", Name: "Admins", OUID: "ou1"}
	suite.mockDBStore.On("GetGroup", suite.ctx, "grp1").Return(expected, nil)

	result, err := suite.store.GetGroup(suite.ctx, "grp1")

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), expected, result)
	suite.mockFileStore.AssertNotCalled(suite.T(), "GetGroup")
}

// Test GetGroup falls back to file store when not found in DB.
func (suite *CompositeGroupStoreEdgeCaseTestSuite) TestGetGroup_FallbackToFile() {
	expected := GroupDAO{ID: "grp1", Name: "Admins", OUID: "ou1", IsReadOnly: true}
	suite.mockDBStore.On("GetGroup", suite.ctx, "grp1").Return(GroupDAO{}, ErrGroupNotFound)
	suite.mockFileStore.On("GetGroup", suite.ctx, "grp1").Return(expected, nil)

	result, err := suite.store.GetGroup(suite.ctx, "grp1")

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), expected, result)
}

// Test GetGroup returns not-found when missing from both stores.
func (suite *CompositeGroupStoreEdgeCaseTestSuite) TestGetGroup_NotFound() {
	suite.mockDBStore.On("GetGroup", suite.ctx, "missing").Return(GroupDAO{}, ErrGroupNotFound)
	suite.mockFileStore.On("GetGroup", suite.ctx, "missing").Return(GroupDAO{}, ErrGroupNotFound)

	result, err := suite.store.GetGroup(suite.ctx, "missing")

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), GroupDAO{}, result)
}

// Test GetGroup propagates non-not-found DB errors without consulting the file store.
func (suite *CompositeGroupStoreEdgeCaseTestSuite) TestGetGroup_DBError() {
	dbErr := errors.New("database connection error")
	suite.mockDBStore.On("GetGroup", suite.ctx, "grp1").Return(GroupDAO{}, dbErr)

	result, err := suite.store.GetGroup(suite.ctx, "grp1")

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), dbErr, err)
	assert.Equal(suite.T(), GroupDAO{}, result)
	suite.mockFileStore.AssertNotCalled(suite.T(), "GetGroup")
}

// Test UpdateGroup delegates to database store only.
func (suite *CompositeGroupStoreEdgeCaseTestSuite) TestUpdateGroup_DelegatesToDB() {
	group := GroupDAO{ID: "grp1", Name: "Updated", OUID: "ou1"}
	suite.mockDBStore.On("UpdateGroup", suite.ctx, group).Return(nil)

	err := suite.store.UpdateGroup(suite.ctx, group)

	assert.NoError(suite.T(), err)
	suite.mockDBStore.AssertExpectations(suite.T())
	suite.mockFileStore.AssertNotCalled(suite.T(), "UpdateGroup")
}

// Test DeleteGroup delegates to database store only.
func (suite *CompositeGroupStoreEdgeCaseTestSuite) TestDeleteGroup_DelegatesToDB() {
	suite.mockDBStore.On("DeleteGroup", suite.ctx, "grp1").Return(nil)

	err := suite.store.DeleteGroup(suite.ctx, "grp1")

	assert.NoError(suite.T(), err)
	suite.mockDBStore.AssertExpectations(suite.T())
	suite.mockFileStore.AssertNotCalled(suite.T(), "DeleteGroup")
}

// Test AddGroupMembers delegates to database store only.
func (suite *CompositeGroupStoreEdgeCaseTestSuite) TestAddGroupMembers_DelegatesToDB() {
	members := []Member{{ID: "user1", Type: MemberTypeUser}}
	suite.mockDBStore.On("AddGroupMembers", suite.ctx, "grp1", members).Return(nil)

	err := suite.store.AddGroupMembers(suite.ctx, "grp1", members)

	assert.NoError(suite.T(), err)
	suite.mockDBStore.AssertExpectations(suite.T())
	suite.mockFileStore.AssertNotCalled(suite.T(), "AddGroupMembers")
}

// Test RemoveGroupMembers delegates to database store only.
func (suite *CompositeGroupStoreEdgeCaseTestSuite) TestRemoveGroupMembers_DelegatesToDB() {
	members := []Member{{ID: "user1", Type: MemberTypeUser}}
	suite.mockDBStore.On("RemoveGroupMembers", suite.ctx, "grp1", members).Return(nil)

	err := suite.store.RemoveGroupMembers(suite.ctx, "grp1", members)

	assert.NoError(suite.T(), err)
	suite.mockDBStore.AssertExpectations(suite.T())
	suite.mockFileStore.AssertNotCalled(suite.T(), "RemoveGroupMembers")
}

// Test GetGroupMembers merges members from both stores.
func (suite *CompositeGroupStoreEdgeCaseTestSuite) TestGetGroupMembers_MergesBothStores() {
	dbMembers := []Member{{ID: "user1", Type: MemberTypeUser}}
	fileMembers := []Member{{ID: "user2", Type: MemberTypeUser}}

	suite.mockDBStore.On("GetGroupMemberCount", suite.ctx, "grp1").Return(1, nil)
	suite.mockFileStore.On("GetGroupMemberCount", suite.ctx, "grp1").Return(1, nil)
	suite.mockDBStore.On("GetGroupMembers", suite.ctx, "grp1", 1, 0).Return(dbMembers, nil)
	suite.mockFileStore.On("GetGroupMembers", suite.ctx, "grp1", 1, 0).Return(fileMembers, nil)

	result, err := suite.store.GetGroupMembers(suite.ctx, "grp1", 10, 0)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), result, 2)
}

// Test GetGroupMembers deduplicates members present in both stores.
func (suite *CompositeGroupStoreEdgeCaseTestSuite) TestGetGroupMembers_DeduplicatesSameIDAndType() {
	member := Member{ID: "user1", Type: MemberTypeUser}

	suite.mockDBStore.On("GetGroupMemberCount", suite.ctx, "grp1").Return(1, nil)
	suite.mockFileStore.On("GetGroupMemberCount", suite.ctx, "grp1").Return(1, nil)
	suite.mockDBStore.On("GetGroupMembers", suite.ctx, "grp1", 1, 0).Return([]Member{member}, nil)
	suite.mockFileStore.On("GetGroupMembers", suite.ctx, "grp1", 1, 0).Return([]Member{member}, nil)

	result, err := suite.store.GetGroupMembers(suite.ctx, "grp1", 10, 0)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), result, 1)
}

// Test GetGroupMembers returns only file members when DB has none.
func (suite *CompositeGroupStoreEdgeCaseTestSuite) TestGetGroupMembers_OnlyFileMembers() {
	fileMembers := []Member{{ID: "user1", Type: MemberTypeUser}}

	suite.mockDBStore.On("GetGroupMemberCount", suite.ctx, "grp1").Return(0, nil)
	suite.mockFileStore.On("GetGroupMemberCount", suite.ctx, "grp1").Return(1, nil)
	suite.mockDBStore.On("GetGroupMembers", suite.ctx, "grp1", 0, 0).Return([]Member{}, nil)
	suite.mockFileStore.On("GetGroupMembers", suite.ctx, "grp1", 1, 0).Return(fileMembers, nil)

	result, err := suite.store.GetGroupMembers(suite.ctx, "grp1", 10, 0)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), fileMembers, result)
}

// Test GetGroupMembers propagates DB count error.
func (suite *CompositeGroupStoreEdgeCaseTestSuite) TestGetGroupMembers_DBCountError() {
	dbErr := errors.New("db error")
	suite.mockDBStore.On("GetGroupMemberCount", suite.ctx, "grp1").Return(0, dbErr)

	_, err := suite.store.GetGroupMembers(suite.ctx, "grp1", 10, 0)

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), dbErr, err)
}

// Test GetGroupMembers propagates file store count error.
func (suite *CompositeGroupStoreEdgeCaseTestSuite) TestGetGroupMembers_FileCountError() {
	fileErr := errors.New("file error")
	suite.mockDBStore.On("GetGroupMemberCount", suite.ctx, "grp1").Return(1, nil)
	suite.mockFileStore.On("GetGroupMemberCount", suite.ctx, "grp1").Return(0, fileErr)

	_, err := suite.store.GetGroupMembers(suite.ctx, "grp1", 10, 0)

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), fileErr, err)
}

// Test GetGroupMemberCount returns merged deduplicated count.
func (suite *CompositeGroupStoreEdgeCaseTestSuite) TestGetGroupMemberCount_MergesAndDeduplicates() {
	dbMembers := []Member{{ID: "user1", Type: MemberTypeUser}, {ID: "user2", Type: MemberTypeUser}}
	fileMembers := []Member{{ID: "user2", Type: MemberTypeUser}, {ID: "user3", Type: MemberTypeUser}}

	suite.mockDBStore.On("GetGroupMemberCount", suite.ctx, "grp1").Return(2, nil)
	suite.mockFileStore.On("GetGroupMemberCount", suite.ctx, "grp1").Return(2, nil)
	suite.mockDBStore.On("GetGroupMembers", suite.ctx, "grp1", 2, 0).Return(dbMembers, nil)
	suite.mockFileStore.On("GetGroupMembers", suite.ctx, "grp1", 2, 0).Return(fileMembers, nil)

	count, err := suite.store.GetGroupMemberCount(suite.ctx, "grp1")

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), 3, count)
}

// Test GetGroupMemberCount returns zero when both stores are empty.
func (suite *CompositeGroupStoreEdgeCaseTestSuite) TestGetGroupMemberCount_BothEmpty() {
	suite.mockDBStore.On("GetGroupMemberCount", suite.ctx, "grp1").Return(0, nil)
	suite.mockFileStore.On("GetGroupMemberCount", suite.ctx, "grp1").Return(0, nil)

	count, err := suite.store.GetGroupMemberCount(suite.ctx, "grp1")

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), 0, count)
	suite.mockDBStore.AssertNotCalled(suite.T(), "GetGroupMembers",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	suite.mockFileStore.AssertNotCalled(suite.T(), "GetGroupMembers",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// Test GetGroupMemberCount propagates DB count error.
func (suite *CompositeGroupStoreEdgeCaseTestSuite) TestGetGroupMemberCount_DBCountError() {
	dbErr := errors.New("db error")
	suite.mockDBStore.On("GetGroupMemberCount", suite.ctx, "grp1").Return(0, dbErr)

	_, err := suite.store.GetGroupMemberCount(suite.ctx, "grp1")

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), dbErr, err)
}

// Test GetGroupMemberCount propagates file store count error.
func (suite *CompositeGroupStoreEdgeCaseTestSuite) TestGetGroupMemberCount_FileCountError() {
	fileErr := errors.New("file error")
	suite.mockDBStore.On("GetGroupMemberCount", suite.ctx, "grp1").Return(1, nil)
	suite.mockFileStore.On("GetGroupMemberCount", suite.ctx, "grp1").Return(0, fileErr)

	_, err := suite.store.GetGroupMemberCount(suite.ctx, "grp1")

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), fileErr, err)
}

// Test ValidateGroupIDs returns empty when all IDs found in DB.
func (suite *CompositeGroupStoreEdgeCaseTestSuite) TestValidateGroupIDs_AllFoundInDB() {
	groupIDs := []string{"grp1", "grp2"}
	suite.mockDBStore.On("ValidateGroupIDs", suite.ctx, groupIDs).Return([]string{}, nil)

	invalid, err := suite.store.ValidateGroupIDs(suite.ctx, groupIDs)

	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), invalid)
	suite.mockFileStore.AssertNotCalled(suite.T(), "ValidateGroupIDs")
}

// Test ValidateGroupIDs re-checks missing IDs against file store.
func (suite *CompositeGroupStoreEdgeCaseTestSuite) TestValidateGroupIDs_FallbackToFile() {
	groupIDs := []string{"grp1", "grp-declarative"}
	suite.mockDBStore.On("ValidateGroupIDs", suite.ctx, groupIDs).Return([]string{"grp-declarative"}, nil)
	suite.mockFileStore.On("ValidateGroupIDs", suite.ctx, []string{"grp-declarative"}).Return([]string{}, nil)

	invalid, err := suite.store.ValidateGroupIDs(suite.ctx, groupIDs)

	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), invalid)
}

// Test ValidateGroupIDs returns IDs not found in either store.
func (suite *CompositeGroupStoreEdgeCaseTestSuite) TestValidateGroupIDs_NotFoundInEither() {
	groupIDs := []string{"grp1", "grp-missing"}
	suite.mockDBStore.On("ValidateGroupIDs", suite.ctx, groupIDs).Return([]string{"grp-missing"}, nil)
	suite.mockFileStore.On("ValidateGroupIDs", suite.ctx, []string{"grp-missing"}).Return([]string{"grp-missing"}, nil)

	invalid, err := suite.store.ValidateGroupIDs(suite.ctx, groupIDs)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), []string{"grp-missing"}, invalid)
}

// Test ValidateGroupIDs propagates DB error.
func (suite *CompositeGroupStoreEdgeCaseTestSuite) TestValidateGroupIDs_DBError() {
	dbErr := errors.New("db error")
	suite.mockDBStore.On("ValidateGroupIDs", suite.ctx, []string{"grp1"}).Return(nil, dbErr)

	_, err := suite.store.ValidateGroupIDs(suite.ctx, []string{"grp1"})

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), dbErr, err)
}

// Test CheckGroupNameConflictForCreate checks DB first, then file store.
func (suite *CompositeGroupStoreEdgeCaseTestSuite) TestCheckGroupNameConflictForCreate_ChecksBothStores() {
	suite.mockDBStore.On("CheckGroupNameConflictForCreate", suite.ctx, "Admins", "ou1").Return(nil)
	suite.mockFileStore.On("CheckGroupNameConflictForCreate", suite.ctx, "Admins", "ou1").Return(nil)

	err := suite.store.CheckGroupNameConflictForCreate(suite.ctx, "Admins", "ou1")

	assert.NoError(suite.T(), err)
}

// Test CheckGroupNameConflictForCreate returns conflict from DB.
func (suite *CompositeGroupStoreEdgeCaseTestSuite) TestCheckGroupNameConflictForCreate_ConflictInDB() {
	suite.mockDBStore.On("CheckGroupNameConflictForCreate", suite.ctx, "Admins", "ou1").
		Return(ErrGroupNameConflict)

	err := suite.store.CheckGroupNameConflictForCreate(suite.ctx, "Admins", "ou1")

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), ErrGroupNameConflict, err)
	suite.mockFileStore.AssertNotCalled(suite.T(), "CheckGroupNameConflictForCreate")
}

// Test CheckGroupNameConflictForCreate returns conflict from file store.
func (suite *CompositeGroupStoreEdgeCaseTestSuite) TestCheckGroupNameConflictForCreate_ConflictInFile() {
	suite.mockDBStore.On("CheckGroupNameConflictForCreate", suite.ctx, "Admins", "ou1").Return(nil)
	suite.mockFileStore.On("CheckGroupNameConflictForCreate", suite.ctx, "Admins", "ou1").
		Return(ErrGroupNameConflict)

	err := suite.store.CheckGroupNameConflictForCreate(suite.ctx, "Admins", "ou1")

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), ErrGroupNameConflict, err)
}

// Test CheckGroupNameConflictForUpdate checks both stores.
func (suite *CompositeGroupStoreEdgeCaseTestSuite) TestCheckGroupNameConflictForUpdate_ChecksBothStores() {
	suite.mockDBStore.On("CheckGroupNameConflictForUpdate", suite.ctx, "Admins", "ou1", "grp1").Return(nil)
	suite.mockFileStore.On("CheckGroupNameConflictForUpdate", suite.ctx, "Admins", "ou1", "grp1").Return(nil)

	err := suite.store.CheckGroupNameConflictForUpdate(suite.ctx, "Admins", "ou1", "grp1")

	assert.NoError(suite.T(), err)
}

// Test GetGroupsByOrganizationUnit deduplicates across stores.
func (suite *CompositeGroupStoreEdgeCaseTestSuite) TestGetGroupsByOrganizationUnit_Deduplicates() {
	dbGroups := []GroupBasicDAO{{ID: "grp1", OUID: "ou1"}, {ID: "grp2", OUID: "ou1"}}
	fileGroups := []GroupBasicDAO{{ID: "grp2", OUID: "ou1"}, {ID: "grp3", OUID: "ou1"}}

	suite.mockDBStore.On("GetGroupsByOrganizationUnitCount", suite.ctx, "ou1").Return(2, nil)
	suite.mockFileStore.On("GetGroupsByOrganizationUnitCount", suite.ctx, "ou1").Return(2, nil)
	suite.mockDBStore.On("GetGroupsByOrganizationUnit", suite.ctx, "ou1", 2, 0).Return(dbGroups, nil)
	suite.mockFileStore.On("GetGroupsByOrganizationUnit", suite.ctx, "ou1", 2, 0).Return(fileGroups, nil)

	groups, err := suite.store.GetGroupsByOrganizationUnit(suite.ctx, "ou1", 10, 0)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), groups, 3)
}

// Test GetGroupsByOrganizationUnitCount returns deduplicated count.
func (suite *CompositeGroupStoreEdgeCaseTestSuite) TestGetGroupsByOrganizationUnitCount_Deduplicates() {
	dbGroups := []GroupBasicDAO{{ID: "grp1", OUID: "ou1"}, {ID: "grp2", OUID: "ou1"}}
	fileGroups := []GroupBasicDAO{{ID: "grp2", OUID: "ou1"}, {ID: "grp3", OUID: "ou1"}}

	suite.mockDBStore.On("GetGroupsByOrganizationUnitCount", suite.ctx, "ou1").Return(2, nil)
	suite.mockFileStore.On("GetGroupsByOrganizationUnitCount", suite.ctx, "ou1").Return(2, nil)
	suite.mockDBStore.On("GetGroupsByOrganizationUnit", suite.ctx, "ou1", 2, 0).Return(dbGroups, nil)
	suite.mockFileStore.On("GetGroupsByOrganizationUnit", suite.ctx, "ou1", 2, 0).Return(fileGroups, nil)

	count, err := suite.store.GetGroupsByOrganizationUnitCount(suite.ctx, "ou1")

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), 3, count)
}

// Test GetGroupsByIDs returns all groups when all found in DB.
func (suite *CompositeGroupStoreEdgeCaseTestSuite) TestGetGroupsByIDs_AllFoundInDB() {
	groupIDs := []string{"grp1", "grp2"}
	dbGroups := []GroupBasicDAO{{ID: "grp1"}, {ID: "grp2"}}
	suite.mockDBStore.On("GetGroupsByIDs", suite.ctx, groupIDs).Return(dbGroups, nil)

	result, err := suite.store.GetGroupsByIDs(suite.ctx, groupIDs)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), result, 2)
	suite.mockFileStore.AssertNotCalled(suite.T(), "GetGroupsByIDs")
}

// Test GetGroupsByIDs fetches missing IDs from file store.
func (suite *CompositeGroupStoreEdgeCaseTestSuite) TestGetGroupsByIDs_FallbackToFile() {
	groupIDs := []string{"grp1", "grp-declarative"}
	dbGroups := []GroupBasicDAO{{ID: "grp1"}}
	fileGroups := []GroupBasicDAO{{ID: "grp-declarative", IsReadOnly: true}}

	suite.mockDBStore.On("GetGroupsByIDs", suite.ctx, groupIDs).Return(dbGroups, nil)
	suite.mockFileStore.On("GetGroupsByIDs", suite.ctx, []string{"grp-declarative"}).Return(fileGroups, nil)

	result, err := suite.store.GetGroupsByIDs(suite.ctx, groupIDs)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), result, 2)
}

// Test GetGroupsByIDs propagates DB error.
func (suite *CompositeGroupStoreEdgeCaseTestSuite) TestGetGroupsByIDs_DBError() {
	dbErr := errors.New("db error")
	suite.mockDBStore.On("GetGroupsByIDs", suite.ctx, []string{"grp1"}).Return(nil, dbErr)

	_, err := suite.store.GetGroupsByIDs(suite.ctx, []string{"grp1"})

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), dbErr, err)
}

// Test IsGroupDeclarative delegates to file store.
func (suite *CompositeGroupStoreEdgeCaseTestSuite) TestIsGroupDeclarative_ChecksFileStore() {
	suite.mockFileStore.On("IsGroupDeclarative", suite.ctx, "grp1").Return(true, nil)

	isDeclarative, err := suite.store.IsGroupDeclarative(suite.ctx, "grp1")

	assert.NoError(suite.T(), err)
	assert.True(suite.T(), isDeclarative)
	suite.mockDBStore.AssertNotCalled(suite.T(), "IsGroupDeclarative")
}

// Test IsGroupDeclarative returns false for non-declarative group.
func (suite *CompositeGroupStoreEdgeCaseTestSuite) TestIsGroupDeclarative_NonDeclarative() {
	suite.mockFileStore.On("IsGroupDeclarative", suite.ctx, "grp1").Return(false, nil)

	isDeclarative, err := suite.store.IsGroupDeclarative(suite.ctx, "grp1")

	assert.NoError(suite.T(), err)
	assert.False(suite.T(), isDeclarative)
}

// Test mergeGroupBasicDAOs gives DB groups precedence over file groups with same ID.
func (suite *CompositeGroupStoreEdgeCaseTestSuite) TestGetGroupList_DBPrecedence() {
	dbGroups := []GroupBasicDAO{{ID: "grp1", Name: "AdminsDB"}}
	fileGroups := []GroupBasicDAO{{ID: "grp1", Name: "AdminsFile"}}

	suite.mockDBStore.On("GetGroupListCount", suite.ctx).Return(1, nil)
	suite.mockFileStore.On("GetGroupListCount", suite.ctx).Return(1, nil)
	suite.mockDBStore.On("GetGroupList", suite.ctx, 1, 0).Return(dbGroups, nil)
	suite.mockFileStore.On("GetGroupList", suite.ctx, 1, 0).Return(fileGroups, nil)

	result, err := suite.store.GetGroupList(suite.ctx, 10, 0)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), result, 1)
	assert.Equal(suite.T(), "AdminsDB", result[0].Name)
	assert.False(suite.T(), result[0].IsReadOnly)
}

// Test that file-only groups are marked IsReadOnly=true.
func (suite *CompositeGroupStoreEdgeCaseTestSuite) TestGetGroupList_FileGroupsMarkedReadOnly() {
	suite.mockDBStore.On("GetGroupListCount", suite.ctx).Return(0, nil)
	suite.mockFileStore.On("GetGroupListCount", suite.ctx).Return(1, nil)
	suite.mockDBStore.On("GetGroupList", suite.ctx, 0, 0).Return([]GroupBasicDAO{}, nil)
	suite.mockFileStore.On("GetGroupList", suite.ctx, 1, 0).Return([]GroupBasicDAO{{ID: "grp1", Name: "Admins"}}, nil)

	result, err := suite.store.GetGroupList(suite.ctx, 10, 0)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), result, 1)
	assert.True(suite.T(), result[0].IsReadOnly)
}

// Test GetGroupList returns empty when offset exceeds total.
func (suite *CompositeGroupStoreEdgeCaseTestSuite) TestGetGroupList_OffsetBeyondResults() {
	suite.mockDBStore.On("GetGroupListCount", suite.ctx).Return(1, nil)
	suite.mockFileStore.On("GetGroupListCount", suite.ctx).Return(0, nil)

	result, err := suite.store.GetGroupList(suite.ctx, 10, 100)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), result, 0)
	suite.mockDBStore.AssertNotCalled(suite.T(), "GetGroupList", mock.Anything, mock.Anything, mock.Anything)
	suite.mockFileStore.AssertNotCalled(suite.T(), "GetGroupList", mock.Anything, mock.Anything, mock.Anything)
}

// Test GetGroupList propagates DB error.
func (suite *CompositeGroupStoreEdgeCaseTestSuite) TestGetGroupList_PropagatesDBError() {
	dbErr := errors.New("database error")
	suite.mockDBStore.On("GetGroupListCount", suite.ctx).Return(0, dbErr)

	result, err := suite.store.GetGroupList(suite.ctx, 10, 0)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), dbErr, err)
}

// Test GetGroupList propagates file store error.
func (suite *CompositeGroupStoreEdgeCaseTestSuite) TestGetGroupList_PropagatesFileError() {
	fileErr := errors.New("file store error")
	suite.mockDBStore.On("GetGroupListCount", suite.ctx).Return(1, nil)
	suite.mockFileStore.On("GetGroupListCount", suite.ctx).Return(0, fileErr)

	result, err := suite.store.GetGroupList(suite.ctx, 10, 0)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), fileErr, err)
}
