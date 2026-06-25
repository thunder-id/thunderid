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
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

// CompositeRoleStoreTestSuite contains tests for the composite role store.
type CompositeRoleStoreTestSuite struct {
	suite.Suite
	mockDBStore   *roleStoreInterfaceMock
	mockFileStore *roleStoreInterfaceMock
	store         roleStoreInterface
}

func TestCompositeRoleStoreTestSuite(t *testing.T) {
	suite.Run(t, new(CompositeRoleStoreTestSuite))
}

func (suite *CompositeRoleStoreTestSuite) SetupTest() {
	suite.mockDBStore = newRoleStoreInterfaceMock(suite.T())
	suite.mockFileStore = newRoleStoreInterfaceMock(suite.T())
	suite.store = newCompositeRoleStore(suite.mockFileStore, suite.mockDBStore)
}

func (suite *CompositeRoleStoreTestSuite) TestGetRoleListCount_Deduplicates() {
	dbRoles := []Role{{ID: "role1"}, {ID: "role2"}}
	fileRoles := []Role{{ID: "role2"}, {ID: "role3"}}

	suite.mockDBStore.On("GetRoleListCount", mock.Anything).Return(2, nil)
	suite.mockFileStore.On("GetRoleListCount", mock.Anything).Return(2, nil)
	suite.mockDBStore.On("GetRoleList", mock.Anything, 2, 0).Return(dbRoles, nil)
	suite.mockFileStore.On("GetRoleList", mock.Anything, 2, 0).Return(fileRoles, nil)

	count, err := suite.store.GetRoleListCount(context.Background())

	suite.NoError(err)
	suite.Equal(3, count)
}

func (suite *CompositeRoleStoreTestSuite) TestGetRoleList_Pagination() {
	dbRoles := []Role{{ID: "role1"}, {ID: "role2"}}
	fileRoles := []Role{{ID: "role2"}, {ID: "role3"}}

	suite.mockDBStore.On("GetRoleListCount", mock.Anything).Return(2, nil)
	suite.mockFileStore.On("GetRoleListCount", mock.Anything).Return(2, nil)
	suite.mockDBStore.On("GetRoleList", mock.Anything, 2, 0).Return(dbRoles, nil)
	suite.mockFileStore.On("GetRoleList", mock.Anything, 2, 0).Return(fileRoles, nil)

	roles, err := suite.store.GetRoleList(context.Background(), 2, 1)

	suite.NoError(err)
	suite.Len(roles, 2)
}

func (suite *CompositeRoleStoreTestSuite) TestGetRoleListCountByOUID_Deduplicates() {
	dbRoles := []Role{{ID: "role1"}, {ID: "role2"}}
	fileRoles := []Role{{ID: "role2"}, {ID: "role3"}}

	suite.mockDBStore.On("GetRoleListCountByOUID", mock.Anything, "ou-1").Return(2, nil)
	suite.mockFileStore.On("GetRoleListCountByOUID", mock.Anything, "ou-1").Return(2, nil)
	suite.mockDBStore.On("GetRoleListByOUID", mock.Anything, "ou-1", 2, 0).Return(dbRoles, nil)
	suite.mockFileStore.On("GetRoleListByOUID", mock.Anything, "ou-1", 2, 0).Return(fileRoles, nil)

	count, err := suite.store.GetRoleListCountByOUID(context.Background(), "ou-1")

	suite.NoError(err)
	suite.Equal(3, count)
}

func (suite *CompositeRoleStoreTestSuite) TestGetRoleListByOUID_Pagination() {
	dbRoles := []Role{{ID: "role1"}, {ID: "role2"}}
	fileRoles := []Role{{ID: "role2"}, {ID: "role3"}}

	suite.mockDBStore.On("GetRoleListCountByOUID", mock.Anything, "ou-1").Return(2, nil)
	suite.mockFileStore.On("GetRoleListCountByOUID", mock.Anything, "ou-1").Return(2, nil)
	suite.mockDBStore.On("GetRoleListByOUID", mock.Anything, "ou-1", 2, 0).Return(dbRoles, nil)
	suite.mockFileStore.On("GetRoleListByOUID", mock.Anything, "ou-1", 2, 0).Return(fileRoles, nil)

	roles, err := suite.store.GetRoleListByOUID(context.Background(), "ou-1", 2, 1)

	suite.NoError(err)
	suite.Len(roles, 2)
}

func (suite *CompositeRoleStoreTestSuite) TestGetRoleListCountByOUID_DBStoreError() {
	suite.mockDBStore.On("GetRoleListCountByOUID", mock.Anything, "ou-1").Return(0, errors.New("db error"))

	count, err := suite.store.GetRoleListCountByOUID(context.Background(), "ou-1")

	suite.Error(err)
	suite.Equal(0, count)
}

func (suite *CompositeRoleStoreTestSuite) TestGetRoleListByOUID_DBStoreError() {
	suite.mockDBStore.On("GetRoleListCountByOUID", mock.Anything, "ou-1").Return(0, errors.New("db error"))

	roles, err := suite.store.GetRoleListByOUID(context.Background(), "ou-1", 5, 0)

	suite.Error(err)
	suite.Nil(roles)
}

func (suite *CompositeRoleStoreTestSuite) TestGetRoleAssignmentsCount_Deduplicates() {
	dbAssignments := []RoleAssignment{
		{ID: "user1", Type: assigneeTypeEntity},
		{ID: "group1", Type: AssigneeTypeGroup},
	}
	fileAssignments := []RoleAssignment{
		{ID: "user1", Type: assigneeTypeEntity},
		{ID: "group2", Type: AssigneeTypeGroup},
	}

	suite.mockDBStore.On("GetRoleAssignmentsCount", mock.Anything, "role1").Return(2, nil)
	suite.mockFileStore.On("GetRoleAssignmentsCount", mock.Anything, "role1").Return(2, nil)
	suite.mockDBStore.On("GetRoleAssignments", mock.Anything, "role1", 2, 0).Return(dbAssignments, nil)
	suite.mockFileStore.On("GetRoleAssignments", mock.Anything, "role1", 2, 0).Return(fileAssignments, nil)

	count, err := suite.store.GetRoleAssignmentsCount(context.Background(), "role1")

	suite.NoError(err)
	suite.Equal(3, count)
}

func (suite *CompositeRoleStoreTestSuite) TestGetRoleAssignments_Pagination() {
	dbAssignments := []RoleAssignment{
		{ID: "user1", Type: assigneeTypeEntity},
		{ID: "group1", Type: AssigneeTypeGroup},
	}
	fileAssignments := []RoleAssignment{
		{ID: "user1", Type: assigneeTypeEntity},
		{ID: "group2", Type: AssigneeTypeGroup},
	}

	suite.mockDBStore.On("GetRoleAssignmentsCount", mock.Anything, "role1").Return(2, nil)
	suite.mockFileStore.On("GetRoleAssignmentsCount", mock.Anything, "role1").Return(2, nil)
	suite.mockDBStore.On("GetRoleAssignments", mock.Anything, "role1", 2, 0).Return(dbAssignments, nil)
	suite.mockFileStore.On("GetRoleAssignments", mock.Anything, "role1", 2, 0).Return(fileAssignments, nil)

	assignments, err := suite.store.GetRoleAssignments(context.Background(), "role1", 1, 2)

	suite.NoError(err)
	suite.Len(assignments, 1)
}

// Error path tests for composite store
func (suite *CompositeRoleStoreTestSuite) TestGetRoleListCount_DBStoreError() {
	testErr := errors.New("test error")
	suite.mockDBStore.On("GetRoleListCount", mock.Anything).Return(0, testErr)

	_, err := suite.store.GetRoleListCount(context.Background())

	suite.Error(err)
	suite.Equal(testErr, err)
}

func (suite *CompositeRoleStoreTestSuite) TestGetRoleListCount_FileStoreCountError() {
	testErr := errors.New("test error")
	suite.mockDBStore.On("GetRoleListCount", mock.Anything).Return(2, nil)
	suite.mockFileStore.On("GetRoleListCount", mock.Anything).Return(0, testErr)

	_, err := suite.store.GetRoleListCount(context.Background())

	suite.Error(err)
	suite.Equal(testErr, err)
}

func (suite *CompositeRoleStoreTestSuite) TestGetRoleListCount_DBRolesListError() {
	testErr := errors.New("test error")
	suite.mockDBStore.On("GetRoleListCount", mock.Anything).Return(2, nil)
	suite.mockFileStore.On("GetRoleListCount", mock.Anything).Return(2, nil)
	suite.mockDBStore.On("GetRoleList", mock.Anything, 2, 0).Return(nil, testErr)

	_, err := suite.store.GetRoleListCount(context.Background())

	suite.Error(err)
	suite.Equal(testErr, err)
}

func (suite *CompositeRoleStoreTestSuite) TestGetRoleListCount_FileRolesListError() {
	testErr := errors.New("test error")
	dbRoles := []Role{{ID: "role1"}}
	suite.mockDBStore.On("GetRoleListCount", mock.Anything).Return(1, nil)
	suite.mockFileStore.On("GetRoleListCount", mock.Anything).Return(2, nil)
	suite.mockDBStore.On("GetRoleList", mock.Anything, 1, 0).Return(dbRoles, nil)
	suite.mockFileStore.On("GetRoleList", mock.Anything, 2, 0).Return(nil, testErr)

	_, err := suite.store.GetRoleListCount(context.Background())

	suite.Error(err)
	suite.Equal(testErr, err)
}

func (suite *CompositeRoleStoreTestSuite) TestGetRoleList_DBStoreError() {
	testErr := errors.New("test error")
	suite.mockDBStore.On("GetRoleListCount", mock.Anything).Return(0, testErr)

	_, err := suite.store.GetRoleList(context.Background(), 10, 0)

	suite.Error(err)
	suite.Equal(testErr, err)
}

func (suite *CompositeRoleStoreTestSuite) TestGetRoleList_FileStoreCountError() {
	testErr := errors.New("test error")
	suite.mockDBStore.On("GetRoleListCount", mock.Anything).Return(2, nil)
	suite.mockFileStore.On("GetRoleListCount", mock.Anything).Return(0, testErr)

	_, err := suite.store.GetRoleList(context.Background(), 10, 0)

	suite.Error(err)
	suite.Equal(testErr, err)
}

func (suite *CompositeRoleStoreTestSuite) TestGetRoleList_DBRolesListError() {
	testErr := errors.New("test error")
	suite.mockDBStore.On("GetRoleListCount", mock.Anything).Return(2, nil)
	suite.mockFileStore.On("GetRoleListCount", mock.Anything).Return(2, nil)
	suite.mockDBStore.On("GetRoleList", mock.Anything, 2, 0).Return(nil, testErr)

	_, err := suite.store.GetRoleList(context.Background(), 10, 0)

	suite.Error(err)
	suite.Equal(testErr, err)
}

func (suite *CompositeRoleStoreTestSuite) TestGetRoleList_FileRolesListError() {
	testErr := errors.New("test error")
	dbRoles := []Role{{ID: "role1"}}
	suite.mockDBStore.On("GetRoleListCount", mock.Anything).Return(1, nil)
	suite.mockFileStore.On("GetRoleListCount", mock.Anything).Return(2, nil)
	suite.mockDBStore.On("GetRoleList", mock.Anything, 1, 0).Return(dbRoles, nil)
	suite.mockFileStore.On("GetRoleList", mock.Anything, 2, 0).Return(nil, testErr)

	_, err := suite.store.GetRoleList(context.Background(), 10, 0)

	suite.Error(err)
	suite.Equal(testErr, err)
}

func (suite *CompositeRoleStoreTestSuite) TestGetRoleAssignmentsCount_DBStoreError() {
	testErr := errors.New("test error")
	suite.mockDBStore.On("GetRoleAssignmentsCount", mock.Anything, "role1").Return(0, testErr)

	_, err := suite.store.GetRoleAssignmentsCount(context.Background(), "role1")

	suite.Error(err)
	suite.Equal(testErr, err)
}

func (suite *CompositeRoleStoreTestSuite) TestGetRoleAssignmentsCount_FileStoreCountError() {
	testErr := errors.New("test error")
	suite.mockDBStore.On("GetRoleAssignmentsCount", mock.Anything, "role1").Return(2, nil)
	suite.mockFileStore.On("GetRoleAssignmentsCount", mock.Anything, "role1").Return(0, testErr)

	_, err := suite.store.GetRoleAssignmentsCount(context.Background(), "role1")

	suite.Error(err)
	suite.Equal(testErr, err)
}

func (suite *CompositeRoleStoreTestSuite) TestGetRoleAssignmentsCount_DBAssignmentsListError() {
	testErr := errors.New("test error")
	suite.mockDBStore.On("GetRoleAssignmentsCount", mock.Anything, "role1").Return(2, nil)
	suite.mockFileStore.On("GetRoleAssignmentsCount", mock.Anything, "role1").Return(2, nil)
	suite.mockDBStore.On("GetRoleAssignments", mock.Anything, "role1", 2, 0).Return(nil, testErr)

	_, err := suite.store.GetRoleAssignmentsCount(context.Background(), "role1")

	suite.Error(err)
	suite.Equal(testErr, err)
}

func (suite *CompositeRoleStoreTestSuite) TestGetRoleAssignmentsCount_FileAssignmentsListError() {
	testErr := errors.New("test error")
	dbAssignments := []RoleAssignment{{ID: "user1", Type: assigneeTypeEntity}}
	suite.mockDBStore.On("GetRoleAssignmentsCount", mock.Anything, "role1").Return(1, nil)
	suite.mockFileStore.On("GetRoleAssignmentsCount", mock.Anything, "role1").Return(2, nil)
	suite.mockDBStore.On("GetRoleAssignments", mock.Anything, "role1", 1, 0).Return(dbAssignments, nil)
	suite.mockFileStore.On("GetRoleAssignments", mock.Anything, "role1", 2, 0).Return(nil, testErr)

	_, err := suite.store.GetRoleAssignmentsCount(context.Background(), "role1")

	suite.Error(err)
	suite.Equal(testErr, err)
}

func (suite *CompositeRoleStoreTestSuite) TestGetRoleAssignments_DBStoreError() {
	testErr := errors.New("test error")
	suite.mockDBStore.On("GetRoleAssignmentsCount", mock.Anything, "role1").Return(0, testErr)

	_, err := suite.store.GetRoleAssignments(context.Background(), "role1", 10, 0)

	suite.Error(err)
	suite.Equal(testErr, err)
}

func (suite *CompositeRoleStoreTestSuite) TestGetRoleAssignments_FileStoreCountError() {
	testErr := errors.New("test error")
	suite.mockDBStore.On("GetRoleAssignmentsCount", mock.Anything, "role1").Return(2, nil)
	suite.mockFileStore.On("GetRoleAssignmentsCount", mock.Anything, "role1").Return(0, testErr)

	_, err := suite.store.GetRoleAssignments(context.Background(), "role1", 10, 0)

	suite.Error(err)
	suite.Equal(testErr, err)
}

func (suite *CompositeRoleStoreTestSuite) TestGetRoleAssignments_DBAssignmentsListError() {
	testErr := errors.New("test error")
	suite.mockDBStore.On("GetRoleAssignmentsCount", mock.Anything, "role1").Return(2, nil)
	suite.mockFileStore.On("GetRoleAssignmentsCount", mock.Anything, "role1").Return(2, nil)
	suite.mockDBStore.On("GetRoleAssignments", mock.Anything, "role1", 2, 0).Return(nil, testErr)

	_, err := suite.store.GetRoleAssignments(context.Background(), "role1", 10, 0)

	suite.Error(err)
	suite.Equal(testErr, err)
}

func (suite *CompositeRoleStoreTestSuite) TestGetRoleAssignments_FileAssignmentsListError() {
	testErr := errors.New("test error")
	dbAssignments := []RoleAssignment{{ID: "user1", Type: assigneeTypeEntity}}
	suite.mockDBStore.On("GetRoleAssignmentsCount", mock.Anything, "role1").Return(1, nil)
	suite.mockFileStore.On("GetRoleAssignmentsCount", mock.Anything, "role1").Return(2, nil)
	suite.mockDBStore.On("GetRoleAssignments", mock.Anything, "role1", 1, 0).Return(dbAssignments, nil)
	suite.mockFileStore.On("GetRoleAssignments", mock.Anything, "role1", 2, 0).Return(nil, testErr)

	_, err := suite.store.GetRoleAssignments(context.Background(), "role1", 10, 0)

	suite.Error(err)
	suite.Equal(testErr, err)
}
