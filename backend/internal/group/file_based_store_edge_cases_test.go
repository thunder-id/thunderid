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
	"testing"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// GroupFileBasedStoreEdgeCaseTestSuite contains edge case tests for the file-based group store.
type GroupFileBasedStoreEdgeCaseTestSuite struct {
	suite.Suite
	store *fileBasedGroupStore
}

func TestGroupFileBasedStoreEdgeCaseTestSuite(t *testing.T) {
	suite.Run(t, new(GroupFileBasedStoreEdgeCaseTestSuite))
}

func (suite *GroupFileBasedStoreEdgeCaseTestSuite) SetupTest() {
	genericStore := declarativeresource.NewGenericFileBasedStoreForTest(entity.KeyTypeGroup)
	suite.store = &fileBasedGroupStore{GenericFileBasedStore: genericStore}
}

func (suite *GroupFileBasedStoreEdgeCaseTestSuite) seedGroup(grp groupDeclarativeResource) {
	err := suite.store.GenericFileBasedStore.Create(grp.ID, &grp)
	suite.Require().NoError(err)
}

// Test GetGroupList with zero limit returns empty.
func (suite *GroupFileBasedStoreEdgeCaseTestSuite) TestGetGroupList_ZeroLimit() {
	suite.seedGroup(groupDeclarativeResource{ID: "grp1", Name: "Admins", OUID: "ou1"})

	groups, err := suite.store.GetGroupList(context.Background(), 0, 0)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), groups, 0)
}

// Test GetGroupList with negative limit returns empty.
func (suite *GroupFileBasedStoreEdgeCaseTestSuite) TestGetGroupList_NegativeLimit() {
	suite.seedGroup(groupDeclarativeResource{ID: "grp1", Name: "Admins", OUID: "ou1"})

	groups, err := suite.store.GetGroupList(context.Background(), -1, 0)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), groups, 0)
}

// Test GetGroupList with offset beyond total returns empty.
func (suite *GroupFileBasedStoreEdgeCaseTestSuite) TestGetGroupList_OffsetBeyondResults() {
	suite.seedGroup(groupDeclarativeResource{ID: "grp1", Name: "Admins", OUID: "ou1"})

	groups, err := suite.store.GetGroupList(context.Background(), 10, 100)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), groups, 0)
}

// Test GetGroupList with negative offset treats it as zero.
func (suite *GroupFileBasedStoreEdgeCaseTestSuite) TestGetGroupList_NegativeOffset() {
	suite.seedGroup(groupDeclarativeResource{ID: "grp1", Name: "Admins", OUID: "ou1"})

	groups, err := suite.store.GetGroupList(context.Background(), 10, -1)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), groups, 1)
}

// Test GetGroupList on empty store returns empty slice.
func (suite *GroupFileBasedStoreEdgeCaseTestSuite) TestGetGroupList_EmptyStore() {
	groups, err := suite.store.GetGroupList(context.Background(), 10, 0)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), groups, 0)
}

// Test GetGroupListCount on empty store returns zero.
func (suite *GroupFileBasedStoreEdgeCaseTestSuite) TestGetGroupListCount_EmptyStore() {
	count, err := suite.store.GetGroupListCount(context.Background())

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), 0, count)
}

// Test GetGroupListCount consistency with GetGroupList.
func (suite *GroupFileBasedStoreEdgeCaseTestSuite) TestGetGroupListCount_Consistency() {
	for i := 0; i < 5; i++ {
		suite.seedGroup(groupDeclarativeResource{
			ID:   "grp" + string(rune('0'+i)),
			Name: "Group" + string(rune('0'+i)),
			OUID: "ou1",
		})
	}

	count, err := suite.store.GetGroupListCount(context.Background())
	assert.NoError(suite.T(), err)

	groups, err := suite.store.GetGroupList(context.Background(), 100, 0)
	assert.NoError(suite.T(), err)

	assert.Equal(suite.T(), count, len(groups))
}

// Test GetGroupMembers with zero limit returns empty.
func (suite *GroupFileBasedStoreEdgeCaseTestSuite) TestGetGroupMembers_ZeroLimit() {
	suite.seedGroup(groupDeclarativeResource{
		ID:   "grp1",
		Name: "Admins",
		OUID: "ou1",
		Members: []Member{
			{ID: "user1", Type: memberTypeEntity},
		},
	})

	members, err := suite.store.GetGroupMembers(context.Background(), "grp1", 0, 0)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), members, 0)
}

// Test GetGroupMembers with offset beyond members returns empty.
func (suite *GroupFileBasedStoreEdgeCaseTestSuite) TestGetGroupMembers_OffsetBeyondResults() {
	suite.seedGroup(groupDeclarativeResource{
		ID:   "grp1",
		Name: "Admins",
		OUID: "ou1",
		Members: []Member{
			{ID: "user1", Type: memberTypeEntity},
		},
	})

	members, err := suite.store.GetGroupMembers(context.Background(), "grp1", 10, 100)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), members, 0)
}

// Test GetGroupMembers pagination order is stable.
func (suite *GroupFileBasedStoreEdgeCaseTestSuite) TestGetGroupMembers_PaginationOrder() {
	suite.seedGroup(groupDeclarativeResource{
		ID:   "grp1",
		Name: "Admins",
		OUID: "ou1",
		Members: []Member{
			{ID: "user1", Type: memberTypeEntity},
			{ID: "user2", Type: memberTypeEntity},
			{ID: "user3", Type: memberTypeEntity},
		},
	})

	all, err := suite.store.GetGroupMembers(context.Background(), "grp1", 3, 0)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), all, 3)
	allIDs := []string{all[0].ID, all[1].ID, all[2].ID}

	page1, err := suite.store.GetGroupMembers(context.Background(), "grp1", 2, 0)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), page1, 2)
	assert.Equal(suite.T(), allIDs[:2], []string{page1[0].ID, page1[1].ID})

	page2, err := suite.store.GetGroupMembers(context.Background(), "grp1", 2, 2)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), page2, 1)
	assert.Equal(suite.T(), allIDs[2:], []string{page2[0].ID})
}

// Test GetGroupListByOUIDs with empty OU list returns empty.
func (suite *GroupFileBasedStoreEdgeCaseTestSuite) TestGetGroupListByOUIDs_EmptyOUList() {
	suite.seedGroup(groupDeclarativeResource{ID: "grp1", Name: "Admins", OUID: "ou1"})

	groups, err := suite.store.GetGroupListByOUIDs(context.Background(), []string{}, 10, 0)

	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), groups)
}

// Test GetGroupListCountByOUIDs with empty OU list returns zero.
func (suite *GroupFileBasedStoreEdgeCaseTestSuite) TestGetGroupListCountByOUIDs_EmptyOUList() {
	suite.seedGroup(groupDeclarativeResource{ID: "grp1", Name: "Admins", OUID: "ou1"})

	count, err := suite.store.GetGroupListCountByOUIDs(context.Background(), []string{})

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), 0, count)
}

// Test GetGroupListByOUIDs with zero limit returns empty.
func (suite *GroupFileBasedStoreEdgeCaseTestSuite) TestGetGroupListByOUIDs_ZeroLimit() {
	suite.seedGroup(groupDeclarativeResource{ID: "grp1", Name: "Admins", OUID: "ou1"})

	groups, err := suite.store.GetGroupListByOUIDs(context.Background(), []string{"ou1"}, 0, 0)

	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), groups)
}

// Test GetGroupListByOUIDs filters correctly by OU.
func (suite *GroupFileBasedStoreEdgeCaseTestSuite) TestGetGroupListByOUIDs_OUFiltering() {
	suite.seedGroup(groupDeclarativeResource{ID: "grp1", Name: "Admins", OUID: "ou1"})
	suite.seedGroup(groupDeclarativeResource{ID: "grp2", Name: "Finance", OUID: "ou2"})
	suite.seedGroup(groupDeclarativeResource{ID: "grp3", Name: "HR", OUID: "ou3"})

	groups, err := suite.store.GetGroupListByOUIDs(context.Background(), []string{"ou1", "ou2"}, 10, 0)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), groups, 2)
	for _, g := range groups {
		assert.NotEqual(suite.T(), "ou3", g.OUID)
	}
}

// Test GetGroupsByOrganizationUnit with zero limit returns empty.
func (suite *GroupFileBasedStoreEdgeCaseTestSuite) TestGetGroupsByOrganizationUnit_ZeroLimit() {
	suite.seedGroup(groupDeclarativeResource{ID: "grp1", Name: "Admins", OUID: "ou1"})

	groups, err := suite.store.GetGroupsByOrganizationUnit(context.Background(), "ou1", 0, 0)

	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), groups)
}

// Test GetGroupsByOrganizationUnit with offset beyond results returns empty.
func (suite *GroupFileBasedStoreEdgeCaseTestSuite) TestGetGroupsByOrganizationUnit_OffsetBeyondResults() {
	suite.seedGroup(groupDeclarativeResource{ID: "grp1", Name: "Admins", OUID: "ou1"})

	groups, err := suite.store.GetGroupsByOrganizationUnit(context.Background(), "ou1", 10, 100)

	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), groups)
}

// Test GetGroupsByOrganizationUnit with negative offset treated as zero.
func (suite *GroupFileBasedStoreEdgeCaseTestSuite) TestGetGroupsByOrganizationUnit_NegativeOffset() {
	suite.seedGroup(groupDeclarativeResource{ID: "grp1", Name: "Admins", OUID: "ou1"})

	groups, err := suite.store.GetGroupsByOrganizationUnit(context.Background(), "ou1", 10, -1)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), groups, 1)
}

// Test CheckGroupNameConflictForCreate is case-sensitive.
func (suite *GroupFileBasedStoreEdgeCaseTestSuite) TestCheckGroupNameConflictForCreate_CaseSensitive() {
	suite.seedGroup(groupDeclarativeResource{ID: "grp1", Name: "Admins", OUID: "ou1"})

	err := suite.store.CheckGroupNameConflictForCreate(context.Background(), "admins", "ou1")

	assert.NoError(suite.T(), err)
}

// Test CheckGroupNameConflictForCreate does not conflict across OUs.
func (suite *GroupFileBasedStoreEdgeCaseTestSuite) TestCheckGroupNameConflictForCreate_DifferentOUs() {
	suite.seedGroup(groupDeclarativeResource{ID: "grp1", Name: "Admins", OUID: "ou1"})

	err := suite.store.CheckGroupNameConflictForCreate(context.Background(), "Admins", "ou2")

	assert.NoError(suite.T(), err)
}

// Test CheckGroupNameConflictForCreate with special characters.
func (suite *GroupFileBasedStoreEdgeCaseTestSuite) TestCheckGroupNameConflictForCreate_SpecialCharacters() {
	suite.seedGroup(groupDeclarativeResource{ID: "grp1", Name: "Admin@Group#1", OUID: "ou1"})

	err := suite.store.CheckGroupNameConflictForCreate(context.Background(), "Admin@Group#1", "ou1")

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), ErrGroupNameConflict, err)
}

// Test CheckGroupNameConflictForUpdate no conflict when only matching group is excluded.
func (suite *GroupFileBasedStoreEdgeCaseTestSuite) TestCheckGroupNameConflictForUpdate_NoConflictWhenExcluded() {
	suite.seedGroup(groupDeclarativeResource{ID: "grp1", Name: "Admins", OUID: "ou1"})

	err := suite.store.CheckGroupNameConflictForUpdate(context.Background(), "Admins", "ou1", "grp1")

	assert.NoError(suite.T(), err)
}

// Test GetGroupMemberCount on group with no members.
func (suite *GroupFileBasedStoreEdgeCaseTestSuite) TestGetGroupMemberCount_NoMembers() {
	suite.seedGroup(groupDeclarativeResource{ID: "grp1", Name: "Admins", OUID: "ou1"})

	count, err := suite.store.GetGroupMemberCount(context.Background(), "grp1")

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), 0, count)
}

// Test malformed data in store is skipped gracefully.
func (suite *GroupFileBasedStoreEdgeCaseTestSuite) TestGetGroupList_SkipsMalformedEntries() {
	suite.seedGroup(groupDeclarativeResource{ID: "grp1", Name: "Valid", OUID: "ou1"})

	_ = suite.store.GenericFileBasedStore.Create("malformed", "not a group")

	groups, err := suite.store.GetGroupList(context.Background(), 10, 0)

	assert.Nil(suite.T(), err)
	assert.Len(suite.T(), groups, 1)
	assert.Equal(suite.T(), "grp1", groups[0].ID)
	assert.Equal(suite.T(), "Valid", groups[0].Name)
}

// Test GetGroupsByIDs with empty input returns empty slice.
func (suite *GroupFileBasedStoreEdgeCaseTestSuite) TestGetGroupsByIDs_EmptyInput() {
	groups, err := suite.store.GetGroupsByIDs(context.Background(), []string{})

	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), groups)
}

// Test ValidateGroupIDs with empty input returns empty slice.
func (suite *GroupFileBasedStoreEdgeCaseTestSuite) TestValidateGroupIDs_EmptyInput() {
	invalid, err := suite.store.ValidateGroupIDs(context.Background(), []string{})

	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), invalid)
}
