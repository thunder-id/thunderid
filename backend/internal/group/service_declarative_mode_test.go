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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
)

// DeclarativeModeGroupServiceTestSuite tests group service behavior in declarative mode.
type DeclarativeModeGroupServiceTestSuite struct {
	suite.Suite
	service       GroupServiceInterface
	store         *groupStoreInterfaceMock
	entityService *entitymock.EntityServiceInterfaceMock
	ctx           context.Context
}

func (suite *DeclarativeModeGroupServiceTestSuite) SetupTest() {
	config.ResetServerRuntime()
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{Enabled: true},
	}
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	suite.Require().NoError(err)

	suite.store = newGroupStoreInterfaceMock(suite.T())
	suite.entityService = entitymock.NewEntityServiceInterfaceMock(suite.T())
	mtx := new(stubTransactioner)
	suite.service = &groupService{
		groupStore:    suite.store,
		authzService:  newAllowAllAuthz(suite.T()),
		transactioner: mtx,
		entityService: suite.entityService,
	}
	suite.ctx = context.Background()
}

func (suite *DeclarativeModeGroupServiceTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func TestDeclarativeModeGroupServiceTestSuite(t *testing.T) {
	suite.Run(t, new(DeclarativeModeGroupServiceTestSuite))
}

// CreateGroup must be rejected immediately in declarative-only mode without touching the store.
func (suite *DeclarativeModeGroupServiceTestSuite) TestCreateGroup_FailsInDeclarativeMode() {
	request := CreateGroupRequest{Name: "test-group", OUID: "ou-1"}

	grp, err := suite.service.CreateGroup(suite.ctx, request)

	suite.NotNil(err)
	assert.Equal(suite.T(), ErrorDeclarativeModeGroupCreateNotAllowed.Code, err.Code)
	assert.Nil(suite.T(), grp)
	suite.store.AssertNotCalled(suite.T(), "CreateGroup", mock.Anything, mock.Anything)
}

// UpdateGroup must be rejected when the target group is declarative.
func (suite *DeclarativeModeGroupServiceTestSuite) TestUpdateGroup_FailsWhenDeclarative() {
	suite.store.On("IsGroupDeclarative", suite.ctx, "group-1").Return(true, nil).Once()

	request := UpdateGroupRequest{Name: "updated-name", OUID: "ou-1"}
	grp, err := suite.service.UpdateGroup(suite.ctx, "group-1", request)

	suite.NotNil(err)
	assert.Equal(suite.T(), ErrorImmutableGroup.Code, err.Code)
	assert.Nil(suite.T(), grp)
	suite.store.AssertNotCalled(suite.T(), "UpdateGroup", mock.Anything, mock.Anything)
}

// UpdateGroup must succeed (not be blocked) when the target group is mutable.
func (suite *DeclarativeModeGroupServiceTestSuite) TestUpdateGroup_ProceedsWhenMutable() {
	suite.store.On("IsGroupDeclarative", suite.ctx, "group-1").Return(false, nil).Once()
	suite.store.On("GetGroup", mock.Anything, "group-1").Return(GroupDAO{
		ID:   "group-1",
		Name: "original-name",
		OUID: "ou-1",
	}, nil)
	suite.store.On("CheckGroupNameConflictForUpdate", mock.Anything, "updated-name", "ou-1", "group-1").
		Return(nil).Maybe()
	suite.store.On("UpdateGroup", mock.Anything, mock.Anything).Return(nil).Once()

	request := UpdateGroupRequest{Name: "updated-name", OUID: "ou-1"}
	grp, err := suite.service.UpdateGroup(suite.ctx, "group-1", request)

	suite.Nil(err)
	assert.NotNil(suite.T(), grp)
}

// DeleteGroup must be rejected when the target group is declarative.
func (suite *DeclarativeModeGroupServiceTestSuite) TestDeleteGroup_FailsWhenDeclarative() {
	suite.store.On("IsGroupDeclarative", suite.ctx, "group-1").Return(true, nil).Once()

	err := suite.service.DeleteGroup(suite.ctx, "group-1")

	suite.NotNil(err)
	assert.Equal(suite.T(), ErrorImmutableGroup.Code, err.Code)
	suite.store.AssertNotCalled(suite.T(), "DeleteGroup", mock.Anything, mock.Anything)
}

// Add/RemoveGroupMembers must succeed for declarative groups — member assignments are always DB-backed.
func (suite *DeclarativeModeGroupServiceTestSuite) TestGroupMemberMutations_AllowedForDeclarativeGroup() {
	cases := []struct {
		name   string
		invoke func(members []Member) (*Group, *serviceerror.ServiceError)
		method string
	}{
		{
			name:   "AddGroupMembers",
			method: "AddGroupMembers",
			invoke: func(m []Member) (*Group, *serviceerror.ServiceError) {
				return suite.service.AddGroupMembers(suite.ctx, "group-1", m)
			},
		},
		{
			name:   "RemoveGroupMembers",
			method: "RemoveGroupMembers",
			invoke: func(m []Member) (*Group, *serviceerror.ServiceError) {
				return suite.service.RemoveGroupMembers(suite.ctx, "group-1", m)
			},
		},
	}

	for _, tc := range cases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			grpDAO := GroupDAO{ID: "group-1", Name: "Admins", OUID: "ou-1"}
			normalizedMembers := []Member{{ID: "user-1", Type: memberTypeEntity}}

			suite.store.On("GetGroup", mock.Anything, "group-1").Return(grpDAO, nil)
			suite.entityService.On("GetEntitiesByIDs", mock.Anything, []string{"user-1"}).
				Return([]entity.Entity{{ID: "user-1", Category: entity.EntityCategoryUser}}, nil)
			suite.store.On(tc.method, mock.Anything, "group-1", normalizedMembers).Return(nil)

			grp, err := tc.invoke([]Member{{ID: "user-1", Type: MemberTypeUser}})

			suite.Nil(err)
			assert.NotNil(suite.T(), grp)
			suite.store.AssertCalled(suite.T(), tc.method, mock.Anything, "group-1", normalizedMembers)
		})
	}
}
