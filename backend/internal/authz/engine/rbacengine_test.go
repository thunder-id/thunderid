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

package engine

import (
	"github.com/thunder-id/thunderid/internal/system/i18n/core"

	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/tests/mocks/rolemock"
)

const (
	testUserID1 = "user1"
)

// RBACEngineTestSuite is the test suite for RBAC engine.
type RBACEngineTestSuite struct {
	suite.Suite
	mockRoleService *rolemock.RoleServiceInterfaceMock
	engine          AuthorizationEngine
}

func TestRBACEngineTestSuite(t *testing.T) {
	suite.Run(t, new(RBACEngineTestSuite))
}

func (suite *RBACEngineTestSuite) SetupTest() {
	suite.mockRoleService = rolemock.NewRoleServiceInterfaceMock(suite.T())
	suite.engine = NewRBACEngine(suite.mockRoleService)
}

func (suite *RBACEngineTestSuite) TestGetAuthorizedPermissions_Success() {
	userID := testUserID1
	groupIDs := []string{"group1", "group2"}
	requestedPermissions := []string{"perm1", "perm2", "perm3"}
	authorizedPermissions := []string{"perm1", "perm3"}

	suite.mockRoleService.On("GetAuthorizedPermissions", mock.Anything, userID, groupIDs, requestedPermissions).
		Return(authorizedPermissions, nil)

	result, err := suite.engine.GetAuthorizedPermissions(context.Background(), userID, groupIDs, requestedPermissions)

	suite.Nil(err)
	suite.Equal(authorizedPermissions, result)
}

func (suite *RBACEngineTestSuite) TestGetAuthorizedPermissions_UserOnly() {
	userID := testUserID1
	groupIDs := []string{}
	requestedPermissions := []string{"perm1", "perm2"}
	authorizedPermissions := []string{"perm1"}

	suite.mockRoleService.On("GetAuthorizedPermissions", mock.Anything, userID, groupIDs, requestedPermissions).
		Return(authorizedPermissions, nil)

	result, err := suite.engine.GetAuthorizedPermissions(context.Background(), userID, groupIDs, requestedPermissions)

	suite.Nil(err)
	suite.Equal(authorizedPermissions, result)
}

func (suite *RBACEngineTestSuite) TestGetAuthorizedPermissions_GroupsOnly() {
	userID := ""
	groupIDs := []string{"group1", "group2"}
	requestedPermissions := []string{"perm1", "perm2"}
	authorizedPermissions := []string{"perm2"}

	suite.mockRoleService.On("GetAuthorizedPermissions", mock.Anything, userID, groupIDs, requestedPermissions).
		Return(authorizedPermissions, nil)

	result, err := suite.engine.GetAuthorizedPermissions(context.Background(), userID, groupIDs, requestedPermissions)

	suite.Nil(err)
	suite.Equal(authorizedPermissions, result)
}

func (suite *RBACEngineTestSuite) TestGetAuthorizedPermissions_NoAuthorizedPermissions() {
	userID := testUserID1
	groupIDs := []string{"group1"}
	requestedPermissions := []string{"perm1", "perm2"}
	authorizedPermissions := []string{}

	suite.mockRoleService.On("GetAuthorizedPermissions", mock.Anything, userID, groupIDs, requestedPermissions).
		Return(authorizedPermissions, nil)

	result, err := suite.engine.GetAuthorizedPermissions(context.Background(), userID, groupIDs, requestedPermissions)

	suite.Nil(err)
	suite.Empty(result)
}

func (suite *RBACEngineTestSuite) TestGetAuthorizedPermissions_RoleServiceError() {
	userID := testUserID1
	groupIDs := []string{"group1"}
	requestedPermissions := []string{"perm1", "perm2"}

	roleServiceError := &serviceerror.ServiceError{
		Type: serviceerror.ServerErrorType,
		Code: "ROL-5000",
		Error: core.I18nMessage{
			Key: "error.test.internal_server_error", DefaultValue: "Internal server error",
		},
		ErrorDescription: core.I18nMessage{
			Key: "error.test.an_unexpected_error_occurred", DefaultValue: "An unexpected error occurred",
		},
	}

	suite.mockRoleService.On("GetAuthorizedPermissions", mock.Anything, userID, groupIDs, requestedPermissions).
		Return([]string(nil), roleServiceError)

	result, err := suite.engine.GetAuthorizedPermissions(context.Background(), userID, groupIDs, requestedPermissions)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Contains(err.Error(), "role service error")
}

func (suite *RBACEngineTestSuite) TestGetAuthorizedPermissions_AllPermissionsAuthorized() {
	userID := testUserID1
	groupIDs := []string{"group1"}
	requestedPermissions := []string{"perm1", "perm2"}

	suite.mockRoleService.On("GetAuthorizedPermissions", mock.Anything, userID, groupIDs, requestedPermissions).
		Return(requestedPermissions, nil)

	result, err := suite.engine.GetAuthorizedPermissions(context.Background(), userID, groupIDs, requestedPermissions)

	suite.Nil(err)
	suite.Equal(requestedPermissions, result)
}
