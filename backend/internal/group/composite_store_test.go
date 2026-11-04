// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package group

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

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

func (suite *CompositeGroupStoreTestSuite) TestMemberGroupWrites_DelegateToDBStore() {
	ctx := context.Background()
	member := Member{ID: "user1", Type: memberTypeEntity}
	suite.mockDBStore.On("GetMemberGroupIDs", ctx, string(memberTypeEntity), "user1").
		Return([]string{"grp1"}, nil).Once()
	suite.mockDBStore.On("AddMemberToGroups", ctx, member, []string{"grp2"}).Return(nil).Once()
	suite.mockDBStore.On("RemoveMemberFromGroups", ctx, member, []string{"grp1"}).Return(nil).Once()

	groupIDs, err := suite.store.GetMemberGroupIDs(ctx, string(memberTypeEntity), "user1")
	suite.NoError(err)
	suite.Equal([]string{"grp1"}, groupIDs)
	suite.NoError(suite.store.AddMemberToGroups(ctx, member, []string{"grp2"}))
	suite.NoError(suite.store.RemoveMemberFromGroups(ctx, member, []string{"grp1"}))
	suite.mockFileStore.AssertNotCalled(suite.T(), "GetMemberGroupIDs", mock.Anything, mock.Anything, mock.Anything)
}
