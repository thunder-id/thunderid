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
	"context"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/mocks/rolemock"
)

const testUserID1 = "user1"

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

func (suite *RBACEngineTestSuite) TestEvaluateAccessSuccess() {
	request := AccessEvaluationRequest{
		Subject:        Subject{ID: testUserID1, GroupIDs: []string{"group1"}},
		ResourceServer: ResourceServer{},
		Permission:     Permission{Name: "document:read"},
	}

	suite.mockRoleService.On("GetAuthorizedPermissionsByResourceServer", mock.Anything, testUserID1,
		[]string{"group1"}, "", []string{"document:read"}).
		Return([]string{"document:read"}, nil)

	result, err := suite.engine.EvaluateAccess(context.Background(), request)

	suite.Nil(err)
	suite.NotNil(result)
	suite.True(result.Decision)
}

func (suite *RBACEngineTestSuite) TestEvaluateAccessDenied() {
	request := AccessEvaluationRequest{
		Subject:        Subject{ID: testUserID1, GroupIDs: []string{"group1"}},
		ResourceServer: ResourceServer{},
		Permission:     Permission{Name: "document:delete"},
	}

	suite.mockRoleService.On("GetAuthorizedPermissionsByResourceServer", mock.Anything, testUserID1,
		[]string{"group1"}, "", []string{"document:delete"}).
		Return([]string{}, nil)

	result, err := suite.engine.EvaluateAccess(context.Background(), request)

	suite.Nil(err)
	suite.NotNil(result)
	suite.False(result.Decision)
}

func (suite *RBACEngineTestSuite) TestEvaluateAccessBatchPreservesOrder() {
	request := AccessEvaluationsRequest{
		Evaluations: []AccessEvaluationRequest{
			{
				Subject:        Subject{ID: testUserID1, GroupIDs: []string{"group1"}},
				ResourceServer: ResourceServer{},
				Permission:     Permission{Name: "document:read"},
			},
			{
				Subject:        Subject{ID: testUserID1, GroupIDs: []string{"group1"}},
				ResourceServer: ResourceServer{},
				Permission:     Permission{Name: "document:delete"},
			},
		},
	}

	suite.mockRoleService.On("GetAuthorizedPermissionsByResourceServer", mock.Anything, testUserID1,
		[]string{"group1"}, "", []string{"document:read", "document:delete"}).
		Return([]string{"document:read"}, nil)

	result, err := suite.engine.EvaluateAccessBatch(context.Background(), request)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Len(result.Evaluations, 2)
	suite.True(result.Evaluations[0].Decision)
	suite.False(result.Evaluations[1].Decision)
	suite.mockRoleService.AssertNumberOfCalls(suite.T(), "GetAuthorizedPermissionsByResourceServer", 1)
}

func (suite *RBACEngineTestSuite) TestEvaluateAccessBatchScopesByResourceServerID() {
	request := AccessEvaluationsRequest{
		Evaluations: []AccessEvaluationRequest{
			{
				Subject:        Subject{ID: testUserID1, GroupIDs: []string{"group1"}},
				ResourceServer: ResourceServer{ID: "booking-api"},
				Permission:     Permission{Name: "read"},
			},
			{
				Subject:        Subject{ID: testUserID1, GroupIDs: []string{"group1"}},
				ResourceServer: ResourceServer{ID: "invoice-api"},
				Permission:     Permission{Name: "read"},
			},
		},
	}

	suite.mockRoleService.On("GetAuthorizedPermissionsByResourceServer", mock.Anything, testUserID1,
		[]string{"group1"}, "booking-api", []string{"read"}).
		Return([]string{"read"}, nil)
	suite.mockRoleService.On("GetAuthorizedPermissionsByResourceServer", mock.Anything, testUserID1,
		[]string{"group1"}, "invoice-api", []string{"read"}).
		Return([]string{}, nil)

	result, err := suite.engine.EvaluateAccessBatch(context.Background(), request)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Len(result.Evaluations, 2)
	suite.True(result.Evaluations[0].Decision)
	suite.False(result.Evaluations[1].Decision)
	suite.mockRoleService.AssertNumberOfCalls(suite.T(), "GetAuthorizedPermissionsByResourceServer", 2)
}

func (suite *RBACEngineTestSuite) TestEvaluateAccessUsesActionNameAsPermission() {
	request := AccessEvaluationRequest{
		Subject:        Subject{ID: testUserID1},
		ResourceServer: ResourceServer{},
		Permission:     Permission{Name: "document:read"},
	}

	suite.mockRoleService.On("GetAuthorizedPermissionsByResourceServer", mock.Anything, testUserID1,
		[]string(nil), "", []string{"document:read"}).
		Return([]string{"document:read"}, nil)

	result, err := suite.engine.EvaluateAccess(context.Background(), request)

	suite.Nil(err)
	suite.NotNil(result)
	suite.True(result.Decision)
}

func (suite *RBACEngineTestSuite) TestEvaluateAccessBatchEmpty() {
	result, err := suite.engine.EvaluateAccessBatch(context.Background(), AccessEvaluationsRequest{})

	suite.Nil(err)
	suite.NotNil(result)
	suite.Empty(result.Evaluations)
	suite.mockRoleService.AssertNotCalled(suite.T(), "GetAuthorizedPermissionsByResourceServer")
}

func (suite *RBACEngineTestSuite) TestEvaluateAccessRoleServiceError() {
	request := AccessEvaluationRequest{
		Subject:        Subject{ID: testUserID1, GroupIDs: []string{"group1"}},
		ResourceServer: ResourceServer{},
		Permission:     Permission{Name: "document:read"},
	}
	roleServiceError := &tidcommon.ServiceError{
		Type: tidcommon.ServerErrorType,
		Code: "ROL-5000",
		Error: tidcommon.I18nMessage{
			Key: "error.test.internal_server_error", DefaultValue: "Internal server error",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.test.an_unexpected_error_occurred", DefaultValue: "An unexpected error occurred",
		},
	}

	suite.mockRoleService.On("GetAuthorizedPermissionsByResourceServer", mock.Anything, testUserID1,
		[]string{"group1"}, "", []string{"document:read"}).
		Return([]string(nil), roleServiceError)

	result, err := suite.engine.EvaluateAccess(context.Background(), request)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Contains(err.Error(), "role service error")
}
