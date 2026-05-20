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

package group

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"

	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

type GroupStoreTestSuite struct {
	suite.Suite
}

func TestGroupStoreTestSuite(t *testing.T) {
	suite.Run(t, new(GroupStoreTestSuite))
}

const queryGroupExistsID = "GRQ-GROUP_MGT-18"
const testDeploymentID = "test-deployment-id"

type validateGroupIDsSetupFn func(
	*providermock.DBProviderInterfaceMock,
	*providermock.DBClientInterfaceMock,
)

type validateGroupIDsOverrideFn func(*bool) func()

type validateGroupIDsPostAssertFn func(
	*testing.T,
	*providermock.DBProviderInterfaceMock,
	*providermock.DBClientInterfaceMock,
	bool,
)

func assertBuilderErrorPostconditions(
	t *testing.T,
	providerMock *providermock.DBProviderInterfaceMock,
	dbClientMock *providermock.DBClientInterfaceMock,
	builderCalled bool,
) {
	require.True(t, builderCalled)
	dbClientMock.AssertNotCalled(t, "QueryContext", mock.Anything, mock.Anything, mock.Anything)
}

func assertEmptyInputPostconditions(
	t *testing.T,
	providerMock *providermock.DBProviderInterfaceMock,
	dbClientMock *providermock.DBClientInterfaceMock,
	builderCalled bool,
) {
	require.False(t, builderCalled)
	providerMock.AssertNotCalled(t, "GetUserDBClient", mock.Anything)
	dbClientMock.AssertNotCalled(t, "QueryContext", mock.Anything, mock.Anything, mock.Anything)
}

type groupConflictTestCase struct {
	name          string
	setupDB       func(*providermock.DBClientInterfaceMock)
	setupProvider func(*providermock.DBProviderInterfaceMock, *providermock.DBClientInterfaceMock)
	invoke        func(*groupStore, *providermock.DBClientInterfaceMock) error
	expectErr     string
	expectErrIs   error
}

func (suite *GroupStoreTestSuite) runGroupNameConflictTestCases(testCases []groupConflictTestCase) {
	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			providerMock := providermock.NewDBProviderInterfaceMock(suite.T())
			dbClientMock := providermock.NewDBClientInterfaceMock(suite.T())
			store := &groupStore{dbProvider: providerMock, deploymentID: testDeploymentID}

			if tc.setupDB != nil {
				tc.setupDB(dbClientMock)
			}
			if tc.setupProvider != nil {
				tc.setupProvider(providerMock, dbClientMock)
			}

			err := tc.invoke(store, dbClientMock)

			switch {
			case tc.expectErrIs != nil:
				suite.Require().ErrorIs(err, tc.expectErrIs)
			case tc.expectErr != "":
				suite.Require().Error(err)
				suite.Require().Contains(err.Error(), tc.expectErr)
			default:
				suite.Require().NoError(err)
			}

			providerMock.AssertExpectations(suite.T())
			dbClientMock.AssertExpectations(suite.T())
		})
	}
}

func (suite *GroupStoreTestSuite) TestGroupStore_GetGroupListCount() {
	testCases := []struct {
		name      string
		setup     func(*providermock.DBProviderInterfaceMock, *providermock.DBClientInterfaceMock)
		wantErr   string
		wantCount int
	}{
		{
			name: "success",
			setup: func(
				providerMock *providermock.DBProviderInterfaceMock,
				dbClientMock *providermock.DBClientInterfaceMock,
			) {
				providerMock.
					On("GetUserDBClient").
					Return(dbClientMock, nil).
					Once()

				dbClientMock.
					On("QueryContext", mock.Anything, QueryGetGroupListCount, testDeploymentID).
					Return([]map[string]interface{}{{"total": int64(7)}}, nil).
					Once()
			},
			wantCount: 7,
		},
		{
			name: "client error",
			setup: func(
				providerMock *providermock.DBProviderInterfaceMock,
				_ *providermock.DBClientInterfaceMock,
			) {
				providerMock.
					On("GetUserDBClient").
					Return(nil, errors.New("no client")).
					Once()
			},
			wantErr:   "failed to get database client",
			wantCount: 0,
		},
		{
			name: "query error",
			setup: func(
				providerMock *providermock.DBProviderInterfaceMock,
				dbClientMock *providermock.DBClientInterfaceMock,
			) {
				providerMock.
					On("GetUserDBClient").
					Return(dbClientMock, nil).
					Once()

				dbClientMock.
					On("QueryContext", mock.Anything, QueryGetGroupListCount, testDeploymentID).
					Return(nil, errors.New("boom")).
					Once()
			},
			wantErr:   "boom",
			wantCount: 0,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			providerMock := providermock.NewDBProviderInterfaceMock(suite.T())
			dbClientMock := providermock.NewDBClientInterfaceMock(suite.T())
			store := &groupStore{dbProvider: providerMock, deploymentID: testDeploymentID}

			if tc.setup != nil {
				tc.setup(providerMock, dbClientMock)
			}

			count, err := store.GetGroupListCount(context.Background())

			if tc.wantErr != "" {
				suite.Require().Error(err)
				suite.Require().Contains(err.Error(), tc.wantErr)
			} else {
				suite.Require().NoError(err)
			}
			suite.Require().Equal(tc.wantCount, count)

			providerMock.AssertExpectations(suite.T())
			dbClientMock.AssertExpectations(suite.T())
		})
	}
}

func (suite *GroupStoreTestSuite) TestGroupStore_GetGroupList() {
	type expectedGroup struct {
		id   string
		name string
		ouID string
	}

	testCases := []struct {
		name       string
		limit      int
		offset     int
		setup      func(*providermock.DBProviderInterfaceMock, *providermock.DBClientInterfaceMock)
		wantErr    string
		wantGroups []expectedGroup
	}{
		{
			name:   "success",
			limit:  5,
			offset: 0,
			setup: func(
				providerMock *providermock.DBProviderInterfaceMock,
				dbClientMock *providermock.DBClientInterfaceMock,
			) {
				providerMock.
					On("GetUserDBClient").
					Return(dbClientMock, nil).
					Once()

				rows := []map[string]interface{}{
					{
						"id":          "g1",
						"name":        "Group 1",
						"description": "Desc 1",
						"ou_id":       "ou-1",
					},
					{
						"id":          "g2",
						"name":        "Group 2",
						"description": "Desc 2",
						"ou_id":       "ou-2",
					},
				}

				dbClientMock.
					On("QueryContext", mock.Anything, QueryGetGroupList, 5, 0, testDeploymentID).
					Return(rows, nil).
					Once()
			},
			wantGroups: []expectedGroup{
				{id: "g1", name: "Group 1", ouID: "ou-1"},
				{id: "g2", name: "Group 2", ouID: "ou-2"},
			},
		},
		{
			name:   "provider error",
			limit:  1,
			offset: 0,
			setup: func(
				providerMock *providermock.DBProviderInterfaceMock,
				_ *providermock.DBClientInterfaceMock,
			) {
				providerMock.
					On("GetUserDBClient").
					Return(nil, errors.New("boom")).
					Once()
			},
			wantErr: "failed to get database client",
		},
		{
			name:   "query error",
			limit:  1,
			offset: 0,
			setup: func(
				providerMock *providermock.DBProviderInterfaceMock,
				dbClientMock *providermock.DBClientInterfaceMock,
			) {
				providerMock.
					On("GetUserDBClient").
					Return(dbClientMock, nil).
					Once()

				dbClientMock.
					On("QueryContext", mock.Anything, QueryGetGroupList, 1, 0, testDeploymentID).
					Return(nil, errors.New("query fail")).
					Once()
			},
			wantErr: "failed to execute group list query",
		},
		{
			name:   "invalid row",
			limit:  1,
			offset: 0,
			setup: func(
				providerMock *providermock.DBProviderInterfaceMock,
				dbClientMock *providermock.DBClientInterfaceMock,
			) {
				providerMock.
					On("GetUserDBClient").
					Return(dbClientMock, nil).
					Once()

				dbClientMock.
					On("QueryContext", mock.Anything, QueryGetGroupList, 1, 0, testDeploymentID).
					Return([]map[string]interface{}{
						{
							"id":   "g1",
							"name": "Group 1",
							// Missing description to trigger validation error
							"ou_id": "ou-1",
						},
					}, nil).
					Once()
			},
			wantErr: "failed to build group from result row",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			providerMock := providermock.NewDBProviderInterfaceMock(suite.T())
			dbClientMock := providermock.NewDBClientInterfaceMock(suite.T())
			store := &groupStore{dbProvider: providerMock, deploymentID: testDeploymentID}

			if tc.setup != nil {
				tc.setup(providerMock, dbClientMock)
			}

			groups, err := store.GetGroupList(context.Background(), tc.limit, tc.offset)

			if tc.wantErr != "" {
				suite.Require().Error(err)
				suite.Require().Contains(err.Error(), tc.wantErr)
				suite.Require().Nil(groups)
			} else {
				suite.Require().NoError(err)
				suite.Require().Len(groups, len(tc.wantGroups))
				for idx, expected := range tc.wantGroups {
					suite.Require().Equal(expected.id, groups[idx].ID)
					suite.Require().Equal(expected.name, groups[idx].Name)
					suite.Require().Equal(expected.ouID, groups[idx].OUID)
				}
			}

			providerMock.AssertExpectations(suite.T())
			dbClientMock.AssertExpectations(suite.T())
		})
	}
}

func (suite *GroupStoreTestSuite) TestGroupStore_CreateGroup() {
}

func (suite *GroupStoreTestSuite) TestGroupStore_GetGroup() {
	type groupAssertion func(GroupDAO)

	testCases := []struct {
		name        string
		groupID     string
		setup       func(*providermock.DBProviderInterfaceMock, *providermock.DBClientInterfaceMock)
		expectErr   string
		expectErrIs error
		assertGroup groupAssertion
	}{
		{
			name:    "success",
			groupID: "grp-001",
			setup: func(
				providerMock *providermock.DBProviderInterfaceMock,
				dbClientMock *providermock.DBClientInterfaceMock,
			) {
				providerMock.
					On("GetUserDBClient").
					Return(dbClientMock, nil).
					Once()

				dbClientMock.
					On("QueryContext", mock.Anything, QueryGetGroupByID, "grp-001", testDeploymentID).
					Return([]map[string]interface{}{
						{
							"id":          "grp-001",
							"name":        "Engineering",
							"description": "Core team",
							"ou_id":       "ou-1",
						},
					}, nil).
					Once()
			},
			assertGroup: func(group GroupDAO) {
				suite.Require().Equal("Engineering", group.Name)
				suite.Require().Equal("ou-1", group.OUID)
			},
		},
		{
			name:    "database client error",
			groupID: "grp-001",
			setup: func(
				providerMock *providermock.DBProviderInterfaceMock,
				_ *providermock.DBClientInterfaceMock,
			) {
				providerMock.
					On("GetUserDBClient").
					Return(nil, errors.New("client fail")).
					Once()
			},
			expectErr: "failed to get database client",
		},
		{
			name:    "result build error",
			groupID: "grp-001",
			setup: func(
				providerMock *providermock.DBProviderInterfaceMock,
				dbClientMock *providermock.DBClientInterfaceMock,
			) {
				providerMock.
					On("GetUserDBClient").
					Return(dbClientMock, nil).
					Once()

				dbClientMock.
					On("QueryContext", mock.Anything, QueryGetGroupByID, "grp-001", testDeploymentID).
					Return([]map[string]interface{}{{"name": "group"}}, nil).
					Once()
			},
			expectErr: "failed to parse id",
		},
		{
			name:    "query error",
			groupID: "grp-001",
			setup: func(
				providerMock *providermock.DBProviderInterfaceMock,
				dbClientMock *providermock.DBClientInterfaceMock,
			) {
				providerMock.
					On("GetUserDBClient").
					Return(dbClientMock, nil).
					Once()

				dbClientMock.
					On("QueryContext", mock.Anything, QueryGetGroupByID, "grp-001", testDeploymentID).
					Return(nil, errors.New("query fail")).
					Once()
			},
			expectErr: "failed to execute query",
		},
		{
			name:    "unexpected multiple results",
			groupID: "grp-001",
			setup: func(
				providerMock *providermock.DBProviderInterfaceMock,
				dbClientMock *providermock.DBClientInterfaceMock,
			) {
				providerMock.
					On("GetUserDBClient").
					Return(dbClientMock, nil).
					Once()

				dbClientMock.
					On("QueryContext", mock.Anything, QueryGetGroupByID, "grp-001", testDeploymentID).
					Return([]map[string]interface{}{
						{"id": "grp-001"},
						{"id": "grp-002"},
					}, nil).
					Once()
			},
			expectErr: "unexpected number of results",
		},
		{
			name:    "group not found",
			groupID: "grp-404",
			setup: func(
				providerMock *providermock.DBProviderInterfaceMock,
				dbClientMock *providermock.DBClientInterfaceMock,
			) {
				providerMock.
					On("GetUserDBClient").
					Return(dbClientMock, nil).
					Once()

				dbClientMock.
					On("QueryContext", mock.Anything, QueryGetGroupByID, "grp-404", testDeploymentID).
					Return([]map[string]interface{}{}, nil).
					Once()
			},
			expectErrIs: ErrGroupNotFound,
			assertGroup: func(group GroupDAO) {
				suite.Require().Empty(group.ID)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			providerMock := providermock.NewDBProviderInterfaceMock(suite.T())
			dbClientMock := providermock.NewDBClientInterfaceMock(suite.T())
			store := &groupStore{dbProvider: providerMock, deploymentID: testDeploymentID}

			if tc.setup != nil {
				tc.setup(providerMock, dbClientMock)
			}

			group, err := store.GetGroup(context.Background(), tc.groupID)

			switch {
			case tc.expectErrIs != nil:
				suite.Require().ErrorIs(err, tc.expectErrIs)
			case tc.expectErr != "":
				suite.Require().Error(err)
				suite.Require().Contains(err.Error(), tc.expectErr)
			default:
				suite.Require().NoError(err)
			}

			if tc.assertGroup != nil {
				tc.assertGroup(group)
			}

			providerMock.AssertExpectations(suite.T())
			dbClientMock.AssertExpectations(suite.T())
		})
	}
}

func (suite *GroupStoreTestSuite) TestGroupStore_GetGroupMembers() {
	testCases := []struct {
		name       string
		groupID    string
		limit      int
		offset     int
		setup      func(*providermock.DBProviderInterfaceMock, *providermock.DBClientInterfaceMock)
		expectErr  string
		assertList func([]Member)
	}{
		{
			name:    "success",
			groupID: "grp-001",
			limit:   2,
			offset:  0,
			setup: func(
				providerMock *providermock.DBProviderInterfaceMock,
				dbClientMock *providermock.DBClientInterfaceMock,
			) {
				providerMock.
					On("GetUserDBClient").
					Return(dbClientMock, nil).
					Once()

				dbClientMock.
					On("QueryContext", mock.Anything, QueryGetGroupMembers, "grp-001", 2, 0, testDeploymentID).
					Return([]map[string]interface{}{
						{"member_id": "usr-1", "member_type": "entity"},
						{"member_id": "grp-2", "member_type": "group"},
					}, nil).
					Once()
			},
			assertList: func(members []Member) {
				suite.Require().Len(members, 2)
				suite.Require().Equal(memberTypeEntity, members[0].Type)
				suite.Require().Equal("grp-2", members[1].ID)
			},
		},
		{
			name:    "query error",
			groupID: "grp-001",
			limit:   2,
			offset:  0,
			setup: func(
				providerMock *providermock.DBProviderInterfaceMock,
				dbClientMock *providermock.DBClientInterfaceMock,
			) {
				providerMock.
					On("GetUserDBClient").
					Return(dbClientMock, nil).
					Once()

				dbClientMock.
					On("QueryContext", mock.Anything, QueryGetGroupMembers, "grp-001", 2, 0, testDeploymentID).
					Return(nil, errors.New("query failed")).
					Once()
			},
			expectErr: "failed to get group members",
		},
		{
			name:    "database client error",
			groupID: "grp-001",
			setup: func(
				providerMock *providermock.DBProviderInterfaceMock,
				_ *providermock.DBClientInterfaceMock,
			) {
				providerMock.
					On("GetUserDBClient").
					Return(nil, errors.New("client fail")).
					Once()
			},
			expectErr: "failed to get database client",
		},
	}

	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			providerMock := providermock.NewDBProviderInterfaceMock(suite.T())
			dbClientMock := providermock.NewDBClientInterfaceMock(suite.T())
			store := &groupStore{dbProvider: providerMock, deploymentID: testDeploymentID}

			if tc.setup != nil {
				tc.setup(providerMock, dbClientMock)
			}

			members, err := store.GetGroupMembers(context.Background(), tc.groupID, tc.limit, tc.offset)

			if tc.expectErr != "" {
				suite.Require().Error(err)
				suite.Require().Contains(err.Error(), tc.expectErr)
				suite.Require().Nil(members)
			} else {
				suite.Require().NoError(err)
				tc.assertList(members)
			}

			providerMock.AssertExpectations(suite.T())
			dbClientMock.AssertExpectations(suite.T())
		})
	}
}

func (suite *GroupStoreTestSuite) TestGroupStore_GetGroupMemberCount() {
	testCases := []struct {
		name      string
		groupID   string
		setup     func(*providermock.DBProviderInterfaceMock, *providermock.DBClientInterfaceMock)
		expectErr string
		expect    int
	}{
		{
			name:    "success",
			groupID: "grp-001",
			setup: func(
				providerMock *providermock.DBProviderInterfaceMock,
				dbClientMock *providermock.DBClientInterfaceMock,
			) {
				providerMock.
					On("GetUserDBClient").
					Return(dbClientMock, nil).
					Once()

				dbClientMock.
					On("QueryContext", mock.Anything, QueryGetGroupMemberCount, "grp-001", testDeploymentID).
					Return([]map[string]interface{}{
						{"total": int64(3)},
					}, nil).
					Once()
			},
			expect: 3,
		},
		{
			name:    "query error",
			groupID: "grp-001",
			setup: func(
				providerMock *providermock.DBProviderInterfaceMock,
				dbClientMock *providermock.DBClientInterfaceMock,
			) {
				providerMock.
					On("GetUserDBClient").
					Return(dbClientMock, nil).
					Once()

				dbClientMock.
					On("QueryContext", mock.Anything, QueryGetGroupMemberCount, "grp-001", testDeploymentID).
					Return(nil, errors.New("query fail")).
					Once()
			},
			expectErr: "failed to get group member count",
		},
		{
			name:    "database client error",
			groupID: "grp-001",
			setup: func(
				providerMock *providermock.DBProviderInterfaceMock,
				_ *providermock.DBClientInterfaceMock,
			) {
				providerMock.
					On("GetUserDBClient").
					Return(nil, errors.New("client fail")).
					Once()
			},
			expectErr: "failed to get database client",
		},
		{
			name:    "empty result",
			groupID: "grp-001",
			setup: func(
				providerMock *providermock.DBProviderInterfaceMock,
				dbClientMock *providermock.DBClientInterfaceMock,
			) {
				providerMock.
					On("GetUserDBClient").
					Return(dbClientMock, nil).
					Once()

				dbClientMock.
					On("QueryContext", mock.Anything, QueryGetGroupMemberCount, "grp-001", testDeploymentID).
					Return([]map[string]interface{}{}, nil).
					Once()
			},
			expect: 0,
		},
		{
			name:    "invalid result format",
			groupID: "grp-001",
			setup: func(
				providerMock *providermock.DBProviderInterfaceMock,
				dbClientMock *providermock.DBClientInterfaceMock,
			) {
				providerMock.
					On("GetUserDBClient").
					Return(dbClientMock, nil).
					Once()

				dbClientMock.
					On("QueryContext", mock.Anything, QueryGetGroupMemberCount, "grp-001", testDeploymentID).
					Return([]map[string]interface{}{{"total": "invalid"}}, nil).
					Once()
			},
			expect: 0,
		},
	}

	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			providerMock := providermock.NewDBProviderInterfaceMock(suite.T())
			dbClientMock := providermock.NewDBClientInterfaceMock(suite.T())
			store := &groupStore{dbProvider: providerMock, deploymentID: testDeploymentID}

			if tc.setup != nil {
				tc.setup(providerMock, dbClientMock)
			}

			count, err := store.GetGroupMemberCount(context.Background(), tc.groupID)

			if tc.expectErr != "" {
				suite.Require().Error(err)
				suite.Require().Contains(err.Error(), tc.expectErr)
			} else {
				suite.Require().NoError(err)
				suite.Require().Equal(tc.expect, count)
			}

			providerMock.AssertExpectations(suite.T())
			dbClientMock.AssertExpectations(suite.T())
		})
	}
}

func (suite *GroupStoreTestSuite) TestGroupStore_UpdateGroup() {
	groupDAO := GroupDAO{
		ID:          "grp-001",
		Name:        "Engineering",
		Description: "Core",
		OUID:        "ou-1",
	}

	groupMinimal := GroupDAO{ID: "grp-001"}

	testCases := []struct {
		name        string
		group       GroupDAO
		setup       func(*providermock.DBProviderInterfaceMock, *providermock.DBClientInterfaceMock)
		expectErr   string
		expectErrIs error
	}{
		{
			name:  "rows affected zero",
			group: groupDAO,
			setup: func(
				providerMock *providermock.DBProviderInterfaceMock,
				dbClientMock *providermock.DBClientInterfaceMock,
			) {
				providerMock.
					On("GetUserDBClient").
					Return(dbClientMock, nil).
					Once()
				dbClientMock.
					On(
						"ExecuteContext",
						mock.Anything,
						QueryUpdateGroup,
						groupDAO.ID,
						groupDAO.OUID,
						groupDAO.Name,
						groupDAO.Description,
						mock.Anything,
						testDeploymentID,
					).
					Return(int64(0), nil).
					Once()
			},
			expectErrIs: ErrGroupNotFound,
		},
		{
			name:  "database client error",
			group: groupMinimal,
			setup: func(
				providerMock *providermock.DBProviderInterfaceMock,
				_ *providermock.DBClientInterfaceMock,
			) {
				providerMock.
					On("GetUserDBClient").
					Return(nil, errors.New("client fail")).
					Once()
			},
			expectErr: "failed to get database client",
		},
		{
			name:  "update error",
			group: groupMinimal,
			setup: func(
				providerMock *providermock.DBProviderInterfaceMock,
				dbClientMock *providermock.DBClientInterfaceMock,
			) {
				providerMock.
					On("GetUserDBClient").
					Return(dbClientMock, nil).
					Once()
				dbClientMock.
					On(
						"ExecuteContext",
						mock.Anything,
						QueryUpdateGroup,
						groupMinimal.ID,
						groupMinimal.OUID,
						groupMinimal.Name,
						groupMinimal.Description,
						mock.Anything,
						testDeploymentID,
					).
					Return(int64(0), errors.New("update fail")).
					Once()
			},
			expectErr: "failed to execute query",
		},
		{
			name:  "success",
			group: groupDAO,
			setup: func(
				providerMock *providermock.DBProviderInterfaceMock,
				dbClientMock *providermock.DBClientInterfaceMock,
			) {
				providerMock.
					On("GetUserDBClient").
					Return(dbClientMock, nil).
					Once()
				dbClientMock.
					On(
						"ExecuteContext",
						mock.Anything,
						QueryUpdateGroup,
						groupDAO.ID,
						groupDAO.OUID,
						groupDAO.Name,
						groupDAO.Description,
						mock.Anything,
						testDeploymentID,
					).
					Return(int64(1), nil).
					Once()
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			providerMock := providermock.NewDBProviderInterfaceMock(suite.T())
			dbClientMock := providermock.NewDBClientInterfaceMock(suite.T())

			store := &groupStore{dbProvider: providerMock, deploymentID: testDeploymentID}

			if tc.setup != nil {
				tc.setup(providerMock, dbClientMock)
			}

			err := store.UpdateGroup(context.Background(), tc.group)

			switch {
			case tc.expectErrIs != nil:
				suite.Require().ErrorIs(err, tc.expectErrIs)
			case tc.expectErr != "":
				suite.Require().Error(err)
				suite.Require().Contains(err.Error(), tc.expectErr)
			default:
				suite.Require().NoError(err)
			}

			providerMock.AssertExpectations(suite.T())
			dbClientMock.AssertExpectations(suite.T())
		})
	}
}

func (suite *GroupStoreTestSuite) TestGroupStore_DeleteGroup() {
	testCases := []struct {
		name      string
		groupID   string
		setup     func(*providermock.DBProviderInterfaceMock, *providermock.DBClientInterfaceMock)
		expectErr string
	}{
		{
			name:    "database client error",
			groupID: "grp-001",
			setup: func(
				providerMock *providermock.DBProviderInterfaceMock,
				_ *providermock.DBClientInterfaceMock,
			) {
				providerMock.
					On("GetUserDBClient").
					Return(nil, errors.New("client fail")).
					Once()
			},
			expectErr: "failed to get database client",
		},
		{
			name:    "delete members error",
			groupID: "grp-001",
			setup: func(
				providerMock *providermock.DBProviderInterfaceMock,
				dbClientMock *providermock.DBClientInterfaceMock,
			) {
				providerMock.
					On("GetUserDBClient").
					Return(dbClientMock, nil).
					Once()
				dbClientMock.
					On(
						"ExecuteContext",
						mock.Anything,
						QueryDeleteGroupMembers,
						"grp-001",
						testDeploymentID,
					).
					Return(int64(0), errors.New("delete fail")).
					Once()
			},
			expectErr: "failed to delete group members",
		},
		{
			name:    "delete group exec error",
			groupID: "grp-001",
			setup: func(
				providerMock *providermock.DBProviderInterfaceMock,
				dbClientMock *providermock.DBClientInterfaceMock,
			) {
				providerMock.
					On("GetUserDBClient").
					Return(dbClientMock, nil).
					Once()
				dbClientMock.
					On(
						"ExecuteContext",
						mock.Anything,
						QueryDeleteGroupMembers,
						"grp-001",
						testDeploymentID,
					).
					Return(int64(1), nil).
					Once()
				dbClientMock.
					On(
						"ExecuteContext",
						mock.Anything,
						QueryDeleteGroup,
						"grp-001",
						testDeploymentID,
					).
					Return(int64(0), errors.New("delete fail")).
					Once()
			},
			expectErr: "failed to execute query",
		},
		{
			name:    "success",
			groupID: "grp-001",
			setup: func(
				providerMock *providermock.DBProviderInterfaceMock,
				dbClientMock *providermock.DBClientInterfaceMock,
			) {
				providerMock.
					On("GetUserDBClient").
					Return(dbClientMock, nil).
					Once()
				dbClientMock.
					On(
						"ExecuteContext",
						mock.Anything,
						QueryDeleteGroupMembers,
						"grp-001",
						testDeploymentID,
					).
					Return(int64(1), nil).
					Once()
				dbClientMock.
					On(
						"ExecuteContext",
						mock.Anything,
						QueryDeleteGroup,
						"grp-001",
						testDeploymentID,
					).
					Return(int64(1), nil).
					Once()
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			providerMock := providermock.NewDBProviderInterfaceMock(suite.T())
			dbClientMock := providermock.NewDBClientInterfaceMock(suite.T())

			store := &groupStore{dbProvider: providerMock, deploymentID: testDeploymentID}

			if tc.setup != nil {
				tc.setup(providerMock, dbClientMock)
			}

			err := store.DeleteGroup(context.Background(), tc.groupID)

			if tc.expectErr != "" {
				suite.Require().Error(err)
				suite.Require().Contains(err.Error(), tc.expectErr)
			} else {
				suite.Require().NoError(err)
			}

			providerMock.AssertExpectations(suite.T())
			dbClientMock.AssertExpectations(suite.T())
		})
	}
}

func (suite *GroupStoreTestSuite) TestGroupStore_ValidateGroupIDs() {
	t := suite.T()
	type testCase struct {
		name            string
		groupIDs        []string
		setup           validateGroupIDsSetupFn
		overrideBuilder validateGroupIDsOverrideFn
		wantInvalid     []string
		wantErr         string
		postAssert      validateGroupIDsPostAssertFn
	}

	queryMatcher := func() interface{} {
		return mock.MatchedBy(func(q dbmodel.DBQuery) bool { return q.ID == queryGroupExistsID })
	}

	testCases := []testCase{
		{
			name:     "returns missing IDs",
			groupIDs: []string{"grp-1", "grp-2"},
			setup: func(
				providerMock *providermock.DBProviderInterfaceMock,
				dbClientMock *providermock.DBClientInterfaceMock,
			) {
				providerMock.
					On("GetUserDBClient").
					Return(dbClientMock, nil).
					Once()

				dbClientMock.
					On("QueryContext", mock.Anything, queryMatcher(), "grp-1", "grp-2", testDeploymentID).
					Return([]map[string]interface{}{{"id": "grp-1"}}, nil).
					Once()
			},
			wantInvalid: []string{"grp-2"},
		},
		{
			name:     "preserves invalid order including empty IDs",
			groupIDs: []string{"grp-miss", "", "grp-hit"},
			setup: func(
				providerMock *providermock.DBProviderInterfaceMock,
				dbClientMock *providermock.DBClientInterfaceMock,
			) {
				providerMock.
					On("GetUserDBClient").
					Return(dbClientMock, nil).
					Once()

				dbClientMock.
					On("QueryContext", mock.Anything, queryMatcher(), "grp-miss", "", "grp-hit", testDeploymentID).
					Return([]map[string]interface{}{{"id": "grp-hit"}}, nil).
					Once()
			},
			wantInvalid: []string{"grp-miss", ""},
		},
		{
			name:     "query error",
			groupIDs: []string{"grp-1"},
			setup: func(
				providerMock *providermock.DBProviderInterfaceMock,
				dbClientMock *providermock.DBClientInterfaceMock,
			) {
				providerMock.
					On("GetUserDBClient").
					Return(dbClientMock, nil).
					Once()

				dbClientMock.
					On("QueryContext", mock.Anything, queryMatcher(), "grp-1", testDeploymentID).
					Return(nil, errors.New("query fail")).
					Once()
			},
			wantErr: "failed to execute query",
		},
		{
			name:     "builder error",
			groupIDs: []string{"grp-1"},
			setup: func(
				providerMock *providermock.DBProviderInterfaceMock,
				dbClientMock *providermock.DBClientInterfaceMock,
			) {
				providerMock.
					On("GetUserDBClient").
					Return(dbClientMock, nil).
					Once()
			},
			overrideBuilder: func(builderCalled *bool) func() {
				originalBuilder := buildBulkGroupExistsQueryFunc
				buildBulkGroupExistsQueryFunc = func(
					groupIDs []string, deploymentID string,
				) (dbmodel.DBQuery, []interface{}, error) {
					if builderCalled != nil {
						*builderCalled = true
					}
					return dbmodel.DBQuery{}, nil, errors.New("builder fail")
				}
				return func() { buildBulkGroupExistsQueryFunc = originalBuilder }
			},
			wantErr:    "failed to build bulk group exists query",
			postAssert: assertBuilderErrorPostconditions,
		},
		{
			name:     "db client error",
			groupIDs: []string{"grp-1"},
			setup: func(
				providerMock *providermock.DBProviderInterfaceMock,
				_ *providermock.DBClientInterfaceMock,
			) {
				providerMock.
					On("GetUserDBClient").
					Return(nil, errors.New("client fail")).
					Once()
			},
			wantErr: "failed to get database client",
		},
		{
			name:        "empty input returns immediately",
			groupIDs:    []string{},
			wantInvalid: []string{},
			overrideBuilder: func(builderCalled *bool) func() {
				originalBuilder := buildBulkGroupExistsQueryFunc
				buildBulkGroupExistsQueryFunc = func(
					groupIDs []string, deploymentID string,
				) (dbmodel.DBQuery, []interface{}, error) {
					if builderCalled != nil {
						*builderCalled = true
					}
					return originalBuilder(groupIDs, deploymentID)
				}
				return func() { buildBulkGroupExistsQueryFunc = originalBuilder }
			},
			postAssert: assertEmptyInputPostconditions,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			providerMock := providermock.NewDBProviderInterfaceMock(t)
			dbClientMock := providermock.NewDBClientInterfaceMock(t)

			var builderCalled bool
			if tc.overrideBuilder != nil {
				restore := tc.overrideBuilder(&builderCalled)
				t.Cleanup(restore)
			}

			if tc.setup != nil {
				tc.setup(providerMock, dbClientMock)
			}

			store := &groupStore{dbProvider: providerMock, deploymentID: testDeploymentID}
			invalid, err := store.ValidateGroupIDs(context.Background(), tc.groupIDs)

			if tc.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantErr)
				require.Nil(t, invalid)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantInvalid, invalid)
			}

			if tc.postAssert != nil {
				tc.postAssert(t, providerMock, dbClientMock, builderCalled)
			}

			providerMock.AssertExpectations(t)
			dbClientMock.AssertExpectations(t)
		})
	}
}

func (suite *GroupStoreTestSuite) TestGroupStore_GetGroupsByOrganizationUnitCount() {
	testCases := []struct {
		name      string
		setup     func(*providermock.DBProviderInterfaceMock, *providermock.DBClientInterfaceMock)
		expectErr string
		expected  int
	}{
		{
			name: "database client error",
			setup: func(
				providerMock *providermock.DBProviderInterfaceMock,
				_ *providermock.DBClientInterfaceMock,
			) {
				providerMock.
					On("GetUserDBClient").
					Return(nil, errors.New("client fail")).
					Once()
			},
			expectErr: "failed to get database client",
		},
		{
			name: "query error",
			setup: func(
				providerMock *providermock.DBProviderInterfaceMock,
				dbClientMock *providermock.DBClientInterfaceMock,
			) {
				providerMock.
					On("GetUserDBClient").
					Return(dbClientMock, nil).
					Once()

				dbClientMock.
					On("QueryContext", mock.Anything, QueryGetGroupsByOrganizationUnitCount, "ou-1", testDeploymentID).
					Return(nil, errors.New("query fail")).
					Once()
			},
			expectErr: "failed to get group count by organization unit",
		},
		{
			name: "empty result",
			setup: func(
				providerMock *providermock.DBProviderInterfaceMock,
				dbClientMock *providermock.DBClientInterfaceMock,
			) {
				providerMock.
					On("GetUserDBClient").
					Return(dbClientMock, nil).
					Once()

				dbClientMock.
					On("QueryContext", mock.Anything, QueryGetGroupsByOrganizationUnitCount, "ou-1", testDeploymentID).
					Return([]map[string]interface{}{}, nil).
					Once()
			},
			expected: 0,
		},
		{
			name: "unexpected format",
			setup: func(
				providerMock *providermock.DBProviderInterfaceMock,
				dbClientMock *providermock.DBClientInterfaceMock,
			) {
				providerMock.
					On("GetUserDBClient").
					Return(dbClientMock, nil).
					Once()

				dbClientMock.
					On("QueryContext", mock.Anything, QueryGetGroupsByOrganizationUnitCount, "ou-1", testDeploymentID).
					Return([]map[string]interface{}{{"total": "not-number"}}, nil).
					Once()
			},
			expectErr: "unexpected response format",
		},
	}

	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			providerMock := providermock.NewDBProviderInterfaceMock(suite.T())
			dbClientMock := providermock.NewDBClientInterfaceMock(suite.T())
			store := &groupStore{dbProvider: providerMock, deploymentID: testDeploymentID}

			if tc.setup != nil {
				tc.setup(providerMock, dbClientMock)
			}

			count, err := store.GetGroupsByOrganizationUnitCount(context.Background(), "ou-1")

			if tc.expectErr != "" {
				suite.Require().Error(err)
				suite.Require().Contains(err.Error(), tc.expectErr)
			} else {
				suite.Require().NoError(err)
				suite.Require().Equal(tc.expected, count)
			}

			providerMock.AssertExpectations(suite.T())
			dbClientMock.AssertExpectations(suite.T())
		})
	}
}

func (suite *GroupStoreTestSuite) TestGroupStore_GetGroupsByOrganizationUnit() {
	testCases := []struct {
		name      string
		setup     func(*providermock.DBProviderInterfaceMock, *providermock.DBClientInterfaceMock)
		expectErr string
		assert    func([]GroupBasicDAO)
	}{
		{
			name: "database client error",
			setup: func(
				providerMock *providermock.DBProviderInterfaceMock,
				_ *providermock.DBClientInterfaceMock,
			) {
				providerMock.
					On("GetUserDBClient").
					Return(nil, errors.New("client fail")).
					Once()
			},
			expectErr: "failed to get database client",
		},
		{
			name: "query error",
			setup: func(
				providerMock *providermock.DBProviderInterfaceMock,
				dbClientMock *providermock.DBClientInterfaceMock,
			) {
				providerMock.
					On("GetUserDBClient").
					Return(dbClientMock, nil).
					Once()

				dbClientMock.
					On(
						"QueryContext",
						mock.Anything,
						QueryGetGroupsByOrganizationUnit,
						"ou-1",
						10,
						0,
						testDeploymentID,
					).
					Return(nil, errors.New("query fail")).
					Once()
			},
			expectErr: "failed to get groups by organization unit",
		},
		{
			name: "success",
			setup: func(
				providerMock *providermock.DBProviderInterfaceMock,
				dbClientMock *providermock.DBClientInterfaceMock,
			) {
				providerMock.
					On("GetUserDBClient").
					Return(dbClientMock, nil).
					Once()

				dbClientMock.
					On(
						"QueryContext",
						mock.Anything,
						QueryGetGroupsByOrganizationUnit,
						"ou-1",
						10,
						0,
						testDeploymentID,
					).
					Return([]map[string]interface{}{
						{"id": "grp-1", "ou_id": "ou-1", "name": "g1", "description": "desc"},
					}, nil).
					Once()
			},
			assert: func(groups []GroupBasicDAO) {
				suite.Require().Len(groups, 1)
				suite.Require().Equal("g1", groups[0].Name)
				suite.Require().Equal("desc", groups[0].Description)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			providerMock := providermock.NewDBProviderInterfaceMock(suite.T())
			dbClientMock := providermock.NewDBClientInterfaceMock(suite.T())
			store := &groupStore{dbProvider: providerMock, deploymentID: testDeploymentID}

			if tc.setup != nil {
				tc.setup(providerMock, dbClientMock)
			}

			groups, err := store.GetGroupsByOrganizationUnit(context.Background(), "ou-1", 10, 0)

			if tc.expectErr != "" {
				suite.Require().Error(err)
				suite.Require().Contains(err.Error(), tc.expectErr)
				suite.Require().Nil(groups)
			} else {
				suite.Require().NoError(err)
				tc.assert(groups)
			}

			providerMock.AssertExpectations(suite.T())
			dbClientMock.AssertExpectations(suite.T())
		})
	}
}

func (suite *GroupStoreTestSuite) TestGroupStore_CheckGroupNameConflictForCreate() {
	testCases := []groupConflictTestCase{
		{
			name: "conflict detected",
			setupDB: func(dbClientMock *providermock.DBClientInterfaceMock) {
				dbClientMock.
					On(
						"QueryContext",
						mock.Anything,
						QueryCheckGroupNameConflict,
						"engineering",
						"ou-1",
						testDeploymentID,
					).
					Return([]map[string]interface{}{{"count": int64(1)}}, nil).
					Once()
			},
			invoke: func(_ *groupStore, dbClientMock *providermock.DBClientInterfaceMock) error {
				return checkGroupNameConflictForCreate(
					context.Background(),
					dbClientMock,
					"engineering",
					"ou-1",
					testDeploymentID,
				)
			},
			expectErrIs: ErrGroupNameConflict,
		},
		{
			name: "query error",
			setupDB: func(dbClientMock *providermock.DBClientInterfaceMock) {
				dbClientMock.
					On(
						"QueryContext",
						mock.Anything,
						QueryCheckGroupNameConflict,
						"engineering",
						"ou-1",
						testDeploymentID,
					).
					Return(nil, errors.New("query fail")).
					Once()
			},
			invoke: func(_ *groupStore, dbClientMock *providermock.DBClientInterfaceMock) error {
				return checkGroupNameConflictForCreate(
					context.Background(),
					dbClientMock,
					"engineering",
					"ou-1",
					testDeploymentID,
				)
			},
			expectErr: "failed to check group name conflict",
		},
		{
			name: "no conflict",
			setupDB: func(dbClientMock *providermock.DBClientInterfaceMock) {
				dbClientMock.
					On(
						"QueryContext",
						mock.Anything,
						QueryCheckGroupNameConflict,
						"engineering",
						"ou-1",
						testDeploymentID,
					).
					Return([]map[string]interface{}{{"count": int64(0)}}, nil).
					Once()
			},
			invoke: func(_ *groupStore, dbClientMock *providermock.DBClientInterfaceMock) error {
				return checkGroupNameConflictForCreate(
					context.Background(),
					dbClientMock,
					"engineering",
					"ou-1",
					testDeploymentID,
				)
			},
		},
		{
			name: "database client error",
			setupProvider: func(
				providerMock *providermock.DBProviderInterfaceMock,
				_ *providermock.DBClientInterfaceMock,
			) {
				providerMock.
					On("GetUserDBClient").
					Return(nil, errors.New("client fail")).
					Once()
			},
			invoke: func(store *groupStore, _ *providermock.DBClientInterfaceMock) error {
				return store.CheckGroupNameConflictForCreate(
					context.Background(),
					"engineering",
					"ou-1",
				)
			},
			expectErr: "failed to get database client",
		},
	}

	suite.runGroupNameConflictTestCases(testCases)
}

func (suite *GroupStoreTestSuite) TestGroupStore_CheckGroupNameConflictForUpdate() {
	testCases := []groupConflictTestCase{
		{
			name: "success",
			setupDB: func(dbClientMock *providermock.DBClientInterfaceMock) {
				dbClientMock.
					On(
						"QueryContext",
						mock.Anything,
						QueryCheckGroupNameConflictForUpdate,
						"engineering",
						"ou-1",
						"grp-1",
						testDeploymentID,
					).
					Return([]map[string]interface{}{{"count": int64(0)}}, nil).
					Once()
			},
			invoke: func(_ *groupStore, dbClientMock *providermock.DBClientInterfaceMock) error {
				return checkGroupNameConflictForUpdate(
					context.Background(),
					dbClientMock,
					"engineering",
					"ou-1",
					"grp-1",
					testDeploymentID,
				)
			},
		},
		{
			name: "conflict detected",
			setupDB: func(dbClientMock *providermock.DBClientInterfaceMock) {
				dbClientMock.
					On(
						"QueryContext",
						mock.Anything,
						QueryCheckGroupNameConflictForUpdate,
						"engineering",
						"ou-1",
						"grp-1",
						testDeploymentID,
					).
					Return([]map[string]interface{}{{"count": int64(1)}}, nil).
					Once()
			},
			invoke: func(_ *groupStore, dbClientMock *providermock.DBClientInterfaceMock) error {
				return checkGroupNameConflictForUpdate(
					context.Background(),
					dbClientMock,
					"engineering",
					"ou-1",
					"grp-1",
					testDeploymentID,
				)
			},
			expectErrIs: ErrGroupNameConflict,
		},
		{
			name: "query error",
			setupDB: func(dbClientMock *providermock.DBClientInterfaceMock) {
				dbClientMock.
					On(
						"QueryContext",
						mock.Anything,
						QueryCheckGroupNameConflictForUpdate,
						"engineering",
						"ou-1",
						"grp-1",
						testDeploymentID,
					).
					Return(nil, errors.New("query fail")).
					Once()
			},
			invoke: func(_ *groupStore, dbClientMock *providermock.DBClientInterfaceMock) error {
				return checkGroupNameConflictForUpdate(
					context.Background(),
					dbClientMock,
					"engineering",
					"ou-1",
					"grp-1",
					testDeploymentID,
				)
			},
			expectErr: "failed to check group name conflict",
		},
		{
			name: "database client error",
			setupProvider: func(
				providerMock *providermock.DBProviderInterfaceMock,
				_ *providermock.DBClientInterfaceMock,
			) {
				providerMock.
					On("GetUserDBClient").
					Return(nil, errors.New("client fail")).
					Once()
			},
			invoke: func(store *groupStore, _ *providermock.DBClientInterfaceMock) error {
				return store.CheckGroupNameConflictForUpdate(
					context.Background(),
					"engineering",
					"ou-1",
					"grp-1",
				)
			},
			expectErr: "failed to get database client",
		},
	}

	suite.runGroupNameConflictTestCases(testCases)
}

func (suite *GroupStoreTestSuite) TestGroupStore_BuildGroupFromResultRowValidationErrors() {
	testCases := []struct {
		name    string
		row     map[string]interface{}
		wantErr string
	}{
		{
			name:    "missing group ID",
			row:     map[string]interface{}{},
			wantErr: "id",
		},
		{
			name: "missing name",
			row: map[string]interface{}{
				"id": "grp-1",
			},
			wantErr: "name",
		},
		{
			name: "missing description",
			row: map[string]interface{}{
				"id":   "grp-1",
				"name": "group",
			},
			wantErr: "description",
		},
		{
			name: "missing organization unit ID",
			row: map[string]interface{}{
				"id":          "grp-1",
				"name":        "group",
				"description": "desc",
			},
			wantErr: "ou_id",
		},
	}

	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			_, err := buildGroupFromResultRow(tc.row)
			suite.Require().Error(err)
			suite.Require().Contains(err.Error(), tc.wantErr)
		})
	}
}

func (suite *GroupStoreTestSuite) TestGroupStore_BuildBulkGroupExistsQueryEmpty() {
	t := suite.T()
	_, _, err := buildBulkGroupExistsQuery([]string{}, testDeploymentID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "groupIDs list cannot be empty")
}

func (suite *GroupStoreTestSuite) TestGroupStore_GetGroupsByIDs() {
	t := suite.T()

	queryMatcher := func() interface{} {
		return mock.MatchedBy(func(q dbmodel.DBQuery) bool { return q.ID == "GRQ-GROUP_MGT-19" })
	}

	type testCase struct {
		name      string
		groupIDs  []string
		setup     func(*providermock.DBProviderInterfaceMock, *providermock.DBClientInterfaceMock)
		wantCount int
		wantErr   string
	}

	testCases := []testCase{
		{
			name:      "empty input returns empty slice",
			groupIDs:  []string{},
			wantCount: 0,
		},
		{
			name:     "success with multiple groups",
			groupIDs: []string{"grp-1", "grp-2"},
			setup: func(
				providerMock *providermock.DBProviderInterfaceMock,
				dbClientMock *providermock.DBClientInterfaceMock,
			) {
				providerMock.On("GetUserDBClient").Return(dbClientMock, nil).Once()
				dbClientMock.On("QueryContext", mock.Anything, queryMatcher(), "grp-1", "grp-2", testDeploymentID).
					Return([]map[string]interface{}{
						{"id": "grp-1", "ou_id": "ou-1", "name": "Group One", "description": "First group"},
						{"id": "grp-2", "ou_id": "ou-1", "name": "Group Two", "description": "Second group"},
					}, nil).Once()
			},
			wantCount: 2,
		},
		{
			name:     "partial results - some IDs not found",
			groupIDs: []string{"grp-1", "grp-missing"},
			setup: func(
				providerMock *providermock.DBProviderInterfaceMock,
				dbClientMock *providermock.DBClientInterfaceMock,
			) {
				providerMock.On("GetUserDBClient").Return(dbClientMock, nil).Once()
				dbClientMock.On(
					"QueryContext", mock.Anything, queryMatcher(), "grp-1", "grp-missing", testDeploymentID,
				).
					Return([]map[string]interface{}{
						{"id": "grp-1", "ou_id": "ou-1", "name": "Group One", "description": "First group"},
					}, nil).Once()
			},
			wantCount: 1,
		},
		{
			name:     "query error",
			groupIDs: []string{"grp-1"},
			setup: func(
				providerMock *providermock.DBProviderInterfaceMock,
				dbClientMock *providermock.DBClientInterfaceMock,
			) {
				providerMock.On("GetUserDBClient").Return(dbClientMock, nil).Once()
				dbClientMock.On("QueryContext", mock.Anything, queryMatcher(), "grp-1", testDeploymentID).
					Return(nil, errors.New("query fail")).Once()
			},
			wantErr: "failed to execute query",
		},
		{
			name:     "db client error",
			groupIDs: []string{"grp-1"},
			setup: func(
				providerMock *providermock.DBProviderInterfaceMock,
				_ *providermock.DBClientInterfaceMock,
			) {
				providerMock.On("GetUserDBClient").Return(nil, errors.New("client fail")).Once()
			},
			wantErr: "failed to get database client",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			providerMock := providermock.NewDBProviderInterfaceMock(t)
			dbClientMock := providermock.NewDBClientInterfaceMock(t)

			if tc.setup != nil {
				tc.setup(providerMock, dbClientMock)
			}

			store := &groupStore{dbProvider: providerMock, deploymentID: testDeploymentID}
			groups, err := store.GetGroupsByIDs(context.Background(), tc.groupIDs)

			if tc.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantErr)
				require.Nil(t, groups)
			} else {
				require.NoError(t, err)
				require.Len(t, groups, tc.wantCount)
			}

			providerMock.AssertExpectations(t)
			dbClientMock.AssertExpectations(t)
		})
	}
}

func (suite *GroupStoreTestSuite) TestGroupStore_AddMembersToGroupReturnsError() {
	t := suite.T()
	dbClientMock := providermock.NewDBClientInterfaceMock(t)

	dbClientMock.
		On(
			"ExecuteContext",
			mock.Anything,
			QueryAddMemberToGroup,
			"grp-001",
			mock.Anything, // MemberType to avoid type mismatch
			"usr-1",
			testDeploymentID,
			mock.Anything, // CREATED_AT
			mock.Anything, // UPDATED_AT
		).
		Return(int64(0), errors.New("insert fail")).
		Once()

	err := addMembersToGroup(
		context.Background(),
		dbClientMock,
		"grp-001",
		[]Member{{ID: "usr-1", Type: memberTypeEntity}},
		testDeploymentID,
	)

	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to add member to group")
}
