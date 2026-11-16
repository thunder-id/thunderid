// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package group

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// CompositeGroupStoreTestSuite contains tests for the composite group store.
type CompositeGroupStoreTestSuite struct {
	suite.Suite
	mockDBStore   *groupStoreInterfaceMock
	mockFileStore *groupStoreInterfaceMock
	store         groupStoreInterface
}

func TestCompositeGroupStoreTestSuite(t *testing.T) {
	suite.Run(t, new(CompositeGroupStoreTestSuite))
}

func (suite *CompositeGroupStoreTestSuite) SetupTest() {
	suite.mockDBStore = newGroupStoreInterfaceMock(suite.T())
	suite.mockFileStore = newGroupStoreInterfaceMock(suite.T())
	suite.store = newCompositeGroupStore(suite.mockFileStore, suite.mockDBStore)
}

func (suite *CompositeGroupStoreTestSuite) TestGetGroupListCount_Deduplicates() {
	dbGroups := []GroupBasicDAO{{ID: "grp1"}, {ID: "grp2"}}
	fileGroups := []GroupBasicDAO{{ID: "grp2"}, {ID: "grp3"}}

	suite.mockDBStore.On("GetGroupListCount", mock.Anything).Return(2, nil)
	suite.mockFileStore.On("GetGroupListCount", mock.Anything).Return(2, nil)
	suite.mockDBStore.On("GetGroupList", mock.Anything, 2, 0).Return(dbGroups, nil)
	suite.mockFileStore.On("GetGroupList", mock.Anything, 2, 0).Return(fileGroups, nil)

	count, err := suite.store.GetGroupListCount(context.Background())

	suite.NoError(err)
	suite.Equal(3, count)
}

func (suite *CompositeGroupStoreTestSuite) TestGetGroupList_Pagination() {
	dbGroups := []GroupBasicDAO{{ID: "grp1"}, {ID: "grp2"}}
	fileGroups := []GroupBasicDAO{{ID: "grp2"}, {ID: "grp3"}}

	suite.mockDBStore.On("GetGroupListCount", mock.Anything).Return(2, nil)
	suite.mockFileStore.On("GetGroupListCount", mock.Anything).Return(2, nil)
	suite.mockDBStore.On("GetGroupList", mock.Anything, 2, 0).Return(dbGroups, nil)
	suite.mockFileStore.On("GetGroupList", mock.Anything, 2, 0).Return(fileGroups, nil)

	groups, err := suite.store.GetGroupList(context.Background(), 2, 1)

	suite.NoError(err)
	suite.Len(groups, 2)
}

func (suite *CompositeGroupStoreTestSuite) TestGetGroupListCountByOUIDs_Deduplicates() {
	ouIDs := []string{"ou1"}
	dbGroups := []GroupBasicDAO{{ID: "grp1", OUID: "ou1"}, {ID: "grp2", OUID: "ou1"}}
	fileGroups := []GroupBasicDAO{{ID: "grp2", OUID: "ou1"}, {ID: "grp3", OUID: "ou1"}}

	suite.mockDBStore.On("GetGroupListCountByOUIDs", mock.Anything, ouIDs).Return(2, nil)
	suite.mockFileStore.On("GetGroupListCountByOUIDs", mock.Anything, ouIDs).Return(2, nil)
	suite.mockDBStore.On("GetGroupListByOUIDs", mock.Anything, ouIDs, 2, 0).Return(dbGroups, nil)
	suite.mockFileStore.On("GetGroupListByOUIDs", mock.Anything, ouIDs, 2, 0).Return(fileGroups, nil)

	count, err := suite.store.GetGroupListCountByOUIDs(context.Background(), ouIDs)

	suite.NoError(err)
	suite.Equal(3, count)
}

func (suite *CompositeGroupStoreTestSuite) TestGetGroupListByOUIDs_Pagination() {
	ouIDs := []string{"ou1"}
	dbGroups := []GroupBasicDAO{{ID: "grp1", OUID: "ou1"}, {ID: "grp2", OUID: "ou1"}}
	fileGroups := []GroupBasicDAO{{ID: "grp2", OUID: "ou1"}, {ID: "grp3", OUID: "ou1"}}

	suite.mockDBStore.On("GetGroupListCountByOUIDs", mock.Anything, ouIDs).Return(2, nil)
	suite.mockFileStore.On("GetGroupListCountByOUIDs", mock.Anything, ouIDs).Return(2, nil)
	suite.mockDBStore.On("GetGroupListByOUIDs", mock.Anything, ouIDs, 2, 0).Return(dbGroups, nil)
	suite.mockFileStore.On("GetGroupListByOUIDs", mock.Anything, ouIDs, 2, 0).Return(fileGroups, nil)

	groups, err := suite.store.GetGroupListByOUIDs(context.Background(), ouIDs, 2, 1)

	suite.NoError(err)
	suite.Len(groups, 2)
}

// Error path tests for GetGroupListCount

func (suite *CompositeGroupStoreTestSuite) TestGetGroupListCount_DBStoreError() {
	testErr := errors.New("test error")
	suite.mockDBStore.On("GetGroupListCount", mock.Anything).Return(0, testErr)

	_, err := suite.store.GetGroupListCount(context.Background())

	suite.Error(err)
	suite.Equal(testErr, err)
}

func (suite *CompositeGroupStoreTestSuite) TestGetGroupListCount_FileStoreCountError() {
	testErr := errors.New("test error")
	suite.mockDBStore.On("GetGroupListCount", mock.Anything).Return(2, nil)
	suite.mockFileStore.On("GetGroupListCount", mock.Anything).Return(0, testErr)

	_, err := suite.store.GetGroupListCount(context.Background())

	suite.Error(err)
	suite.Equal(testErr, err)
}

func (suite *CompositeGroupStoreTestSuite) TestGetGroupListCount_DBListError() {
	testErr := errors.New("test error")
	suite.mockDBStore.On("GetGroupListCount", mock.Anything).Return(2, nil)
	suite.mockFileStore.On("GetGroupListCount", mock.Anything).Return(2, nil)
	suite.mockDBStore.On("GetGroupList", mock.Anything, 2, 0).Return(nil, testErr)

	_, err := suite.store.GetGroupListCount(context.Background())

	suite.Error(err)
	suite.Equal(testErr, err)
}

func (suite *CompositeGroupStoreTestSuite) TestGetGroupListCount_FileListError() {
	testErr := errors.New("test error")
	dbGroups := []GroupBasicDAO{{ID: "grp1"}}
	suite.mockDBStore.On("GetGroupListCount", mock.Anything).Return(1, nil)
	suite.mockFileStore.On("GetGroupListCount", mock.Anything).Return(2, nil)
	suite.mockDBStore.On("GetGroupList", mock.Anything, 1, 0).Return(dbGroups, nil)
	suite.mockFileStore.On("GetGroupList", mock.Anything, 2, 0).Return(nil, testErr)

	_, err := suite.store.GetGroupListCount(context.Background())

	suite.Error(err)
	suite.Equal(testErr, err)
}

// Error path tests for GetGroupList

func (suite *CompositeGroupStoreTestSuite) TestGetGroupList_DBStoreError() {
	testErr := errors.New("test error")
	suite.mockDBStore.On("GetGroupListCount", mock.Anything).Return(0, testErr)

	_, err := suite.store.GetGroupList(context.Background(), 10, 0)

	suite.Error(err)
	suite.Equal(testErr, err)
}

func (suite *CompositeGroupStoreTestSuite) TestGetGroupList_FileStoreCountError() {
	testErr := errors.New("test error")
	suite.mockDBStore.On("GetGroupListCount", mock.Anything).Return(2, nil)
	suite.mockFileStore.On("GetGroupListCount", mock.Anything).Return(0, testErr)

	_, err := suite.store.GetGroupList(context.Background(), 10, 0)

	suite.Error(err)
	suite.Equal(testErr, err)
}

func (suite *CompositeGroupStoreTestSuite) TestGetGroupList_DBListError() {
	testErr := errors.New("test error")
	suite.mockDBStore.On("GetGroupListCount", mock.Anything).Return(2, nil)
	suite.mockFileStore.On("GetGroupListCount", mock.Anything).Return(2, nil)
	suite.mockDBStore.On("GetGroupList", mock.Anything, 2, 0).Return(nil, testErr)

	_, err := suite.store.GetGroupList(context.Background(), 10, 0)

	suite.Error(err)
	suite.Equal(testErr, err)
}

func (suite *CompositeGroupStoreTestSuite) TestGetGroupList_FileListError() {
	testErr := errors.New("test error")
	dbGroups := []GroupBasicDAO{{ID: "grp1"}}
	suite.mockDBStore.On("GetGroupListCount", mock.Anything).Return(1, nil)
	suite.mockFileStore.On("GetGroupListCount", mock.Anything).Return(2, nil)
	suite.mockDBStore.On("GetGroupList", mock.Anything, 1, 0).Return(dbGroups, nil)
	suite.mockFileStore.On("GetGroupList", mock.Anything, 2, 0).Return(nil, testErr)

	_, err := suite.store.GetGroupList(context.Background(), 10, 0)

	suite.Error(err)
	suite.Equal(testErr, err)
}

// --- GetTransitiveGroupsForEntity ---

func (suite *CompositeGroupStoreTestSuite) TestGetGroupMember_PrefersTheDBStore() {
	suite.mockDBStore.On("GetGroupMember", mock.Anything, "grp1", "usr1").
		Return(&Member{ID: "usr1", Type: memberTypeEntity}, nil)

	member, err := suite.store.GetGroupMember(context.Background(), "grp1", "usr1")

	suite.NoError(err)
	suite.Equal(&Member{ID: "usr1", Type: memberTypeEntity}, member)
	suite.mockFileStore.AssertNotCalled(suite.T(), "GetGroupMember", mock.Anything, mock.Anything, mock.Anything)
}

func (suite *CompositeGroupStoreTestSuite) TestGetGroupMember_FallsBackToTheFileStore() {
	suite.mockDBStore.On("GetGroupMember", mock.Anything, "grp1", "grp2").Return(nil, nil)
	suite.mockFileStore.On("GetGroupMember", mock.Anything, "grp1", "grp2").
		Return(&Member{ID: "grp2", Type: MemberTypeGroup}, nil)

	member, err := suite.store.GetGroupMember(context.Background(), "grp1", "grp2")

	suite.NoError(err)
	suite.Equal(&Member{ID: "grp2", Type: MemberTypeGroup}, member)
}

func (suite *CompositeGroupStoreTestSuite) TestGetGroupMember_NilWhenNeitherStoreHasIt() {
	suite.mockDBStore.On("GetGroupMember", mock.Anything, "grp1", "usr1").Return(nil, nil)
	suite.mockFileStore.On("GetGroupMember", mock.Anything, "grp1", "usr1").Return(nil, nil)

	member, err := suite.store.GetGroupMember(context.Background(), "grp1", "usr1")

	suite.NoError(err)
	suite.Nil(member)
}

func (suite *CompositeGroupStoreTestSuite) TestGetGroupMember_DBStoreError() {
	suite.mockDBStore.On("GetGroupMember", mock.Anything, "grp1", "usr1").Return(nil, errors.New("db error"))

	member, err := suite.store.GetGroupMember(context.Background(), "grp1", "usr1")

	suite.Error(err)
	suite.Nil(member)
	suite.mockFileStore.AssertNotCalled(suite.T(), "GetGroupMember", mock.Anything, mock.Anything, mock.Anything)
}

func (suite *CompositeGroupStoreTestSuite) TestGetGroupMember_FileStoreError() {
	suite.mockDBStore.On("GetGroupMember", mock.Anything, "grp1", "usr1").Return(nil, nil)
	suite.mockFileStore.On("GetGroupMember", mock.Anything, "grp1", "usr1").Return(nil, errors.New("file error"))

	member, err := suite.store.GetGroupMember(context.Background(), "grp1", "usr1")

	suite.Error(err)
	suite.Nil(member)
}

// Only the file store declares members, so the database store is not asked.
func (suite *CompositeGroupStoreTestSuite) TestIsGroupMemberDeclarative_AsksTheFileStore() {
	suite.mockFileStore.On("IsGroupMemberDeclarative", mock.Anything, "grp1", "usr1").Return(true, nil)

	declared, err := suite.store.IsGroupMemberDeclarative(context.Background(), "grp1", "usr1")

	suite.NoError(err)
	suite.True(declared)
	suite.mockDBStore.AssertNotCalled(suite.T(), "IsGroupMemberDeclarative", mock.Anything, mock.Anything,
		mock.Anything)
}

func (suite *CompositeGroupStoreTestSuite) TestGetTransitiveGroupsForEntity_MergesBothStores() {
	dbGroups := []providers.EntityGroup{
		{ID: "grp1", Name: "Administrators", OUID: "ou1"},
	}
	fileGroups := []providers.EntityGroup{
		{ID: "grp2", Name: "Declarative Group", OUID: "ou1"},
	}

	suite.mockDBStore.On("GetTransitiveGroupsForEntity", mock.Anything, "user1").Return(dbGroups, nil)
	suite.mockFileStore.On("GetTransitiveGroupsForEntity", mock.Anything, "user1").Return(fileGroups, nil)

	groups, err := suite.store.GetTransitiveGroupsForEntity(context.Background(), "user1")

	suite.NoError(err)
	suite.Len(groups, 2)
	ids := map[string]bool{}
	for _, g := range groups {
		ids[g.ID] = true
	}
	suite.True(ids["grp1"])
	suite.True(ids["grp2"])
}

func (suite *CompositeGroupStoreTestSuite) TestGetTransitiveGroupsForEntity_DeduplicatesOverlap() {
	shared := providers.EntityGroup{ID: "grp1", Name: "Shared Group", OUID: "ou1"}
	dbGroups := []providers.EntityGroup{shared}
	fileGroups := []providers.EntityGroup{shared, {ID: "grp2", Name: "File Only", OUID: "ou1"}}

	suite.mockDBStore.On("GetTransitiveGroupsForEntity", mock.Anything, "user1").Return(dbGroups, nil)
	suite.mockFileStore.On("GetTransitiveGroupsForEntity", mock.Anything, "user1").Return(fileGroups, nil)

	groups, err := suite.store.GetTransitiveGroupsForEntity(context.Background(), "user1")

	suite.NoError(err)
	suite.Len(groups, 2)
}

func (suite *CompositeGroupStoreTestSuite) TestGetTransitiveGroupsForEntity_DBOnlyResult() {
	dbGroups := []providers.EntityGroup{
		{ID: "grp1", Name: "DB Group", OUID: "ou1"},
	}

	suite.mockDBStore.On("GetTransitiveGroupsForEntity", mock.Anything, "user1").Return(dbGroups, nil)
	suite.mockFileStore.On("GetTransitiveGroupsForEntity", mock.Anything, "user1").
		Return([]providers.EntityGroup{}, nil)

	groups, err := suite.store.GetTransitiveGroupsForEntity(context.Background(), "user1")

	suite.NoError(err)
	suite.Len(groups, 1)
	suite.Equal("grp1", groups[0].ID)
}

func (suite *CompositeGroupStoreTestSuite) TestGetTransitiveGroupsForEntity_FileOnlyResult() {
	fileGroups := []providers.EntityGroup{
		{ID: "grp1", Name: "Declarative Group", OUID: "ou1"},
	}

	suite.mockDBStore.On("GetTransitiveGroupsForEntity", mock.Anything, "user1").Return([]providers.EntityGroup{}, nil)
	suite.mockFileStore.On("GetTransitiveGroupsForEntity", mock.Anything, "user1").Return(fileGroups, nil)

	groups, err := suite.store.GetTransitiveGroupsForEntity(context.Background(), "user1")

	suite.NoError(err)
	suite.Len(groups, 1)
	suite.Equal("grp1", groups[0].ID)
}

func (suite *CompositeGroupStoreTestSuite) TestGetTransitiveGroupsForEntity_BothEmpty() {
	suite.mockDBStore.On("GetTransitiveGroupsForEntity", mock.Anything, "user1").Return([]providers.EntityGroup{}, nil)
	suite.mockFileStore.On("GetTransitiveGroupsForEntity", mock.Anything, "user1").
		Return([]providers.EntityGroup{}, nil)

	groups, err := suite.store.GetTransitiveGroupsForEntity(context.Background(), "user1")

	suite.NoError(err)
	suite.Empty(groups)
}

func (suite *CompositeGroupStoreTestSuite) TestGetTransitiveGroupsForEntity_DBStoreError() {
	testErr := errors.New("db error")
	suite.mockDBStore.On("GetTransitiveGroupsForEntity", mock.Anything, "user1").Return(nil, testErr)

	_, err := suite.store.GetTransitiveGroupsForEntity(context.Background(), "user1")

	suite.Error(err)
	suite.Equal(testErr, err)
}

func (suite *CompositeGroupStoreTestSuite) TestGetTransitiveGroupsForEntity_FileStoreError() {
	testErr := errors.New("file store error")
	dbGroups := []providers.EntityGroup{{ID: "grp1", Name: "DB Group", OUID: "ou1"}}

	suite.mockDBStore.On("GetTransitiveGroupsForEntity", mock.Anything, "user1").Return(dbGroups, nil)
	suite.mockFileStore.On("GetTransitiveGroupsForEntity", mock.Anything, "user1").Return(nil, testErr)

	_, err := suite.store.GetTransitiveGroupsForEntity(context.Background(), "user1")

	suite.Error(err)
	suite.Equal(testErr, err)
}

func (suite *CompositeGroupStoreTestSuite) TestGetGroupListByProject_MergesBothStores() {
	ouIDs := []string{"ou1"}
	suite.mockDBStore.On("GetGroupListCountByProject", mock.Anything, "project-1", ouIDs).Return(1, nil)
	suite.mockFileStore.On("GetGroupListCountByProject", mock.Anything, "project-1", ouIDs).Return(1, nil)
	suite.mockDBStore.On("GetGroupListByProject", mock.Anything, "project-1", ouIDs, 1, 0).
		Return([]GroupBasicDAO{{ID: "grp1", ProjectID: "project-1"}}, nil)
	suite.mockFileStore.On("GetGroupListByProject", mock.Anything, "project-1", ouIDs, 1, 0).
		Return([]GroupBasicDAO{{ID: "grp2", ProjectID: "project-1"}}, nil)

	count, err := suite.store.GetGroupListCountByProject(context.Background(), "project-1", ouIDs)
	suite.NoError(err)
	suite.Equal(2, count)

	groups, err := suite.store.GetGroupListByProject(context.Background(), "project-1", ouIDs, 10, 0)
	suite.NoError(err)
	suite.Len(groups, 2)
}

func (suite *CompositeGroupStoreTestSuite) TestGetGroupListCountByProject_Errors() {
	suite.mockDBStore.On("GetGroupListCountByProject", mock.Anything, "project-1", []string(nil)).
		Return(0, errors.New("db fail")).Once()
	_, err := suite.store.GetGroupListCountByProject(context.Background(), "project-1", nil)
	suite.Error(err)

	suite.mockDBStore.On("GetGroupListCountByProject", mock.Anything, "project-2", []string(nil)).Return(1, nil)
	suite.mockFileStore.On("GetGroupListCountByProject", mock.Anything, "project-2", []string(nil)).
		Return(0, errors.New("file fail"))
	_, err = suite.store.GetGroupListCountByProject(context.Background(), "project-2", nil)
	suite.Error(err)

	suite.mockDBStore.On("GetGroupListCountByProject", mock.Anything, "project-3", []string(nil)).
		Return(serverconst.MaxCompositeStoreRecords, nil)
	suite.mockFileStore.On("GetGroupListCountByProject", mock.Anything, "project-3", []string(nil)).Return(1, nil)
	_, err = suite.store.GetGroupListCountByProject(context.Background(), "project-3", nil)
	suite.ErrorIs(err, errResultLimitExceededInCompositeMode)
}

func (suite *CompositeGroupStoreTestSuite) TestGetGroupListByProject_Errors() {
	suite.mockDBStore.On("GetGroupListCountByProject", mock.Anything, "project-1", []string(nil)).
		Return(0, errors.New("db fail"))
	_, err := suite.store.GetGroupListByProject(context.Background(), "project-1", nil, 10, 0)
	suite.Error(err)

	suite.mockDBStore.On("GetGroupListCountByProject", mock.Anything, "project-2", []string(nil)).
		Return(serverconst.MaxCompositeStoreRecords, nil)
	suite.mockFileStore.On("GetGroupListCountByProject", mock.Anything, "project-2", []string(nil)).Return(1, nil)
	_, err = suite.store.GetGroupListByProject(context.Background(), "project-2", nil, 10, 0)
	suite.ErrorIs(err, errResultLimitExceededInCompositeMode)
}

func (suite *CompositeGroupStoreTestSuite) TestGetMemberGroupIDs_DelegatesToDBStore() {
	ctx := context.Background()
	suite.mockDBStore.On("GetMemberGroupIDs", ctx, string(memberTypeEntity), "user1").
		Return([]string{"grp1"}, nil).Once()

	groupIDs, err := suite.store.GetMemberGroupIDs(ctx, string(memberTypeEntity), "user1")
	suite.NoError(err)
	suite.Equal([]string{"grp1"}, groupIDs)
	suite.mockFileStore.AssertNotCalled(suite.T(), "GetMemberGroupIDs", mock.Anything, mock.Anything, mock.Anything)
}
