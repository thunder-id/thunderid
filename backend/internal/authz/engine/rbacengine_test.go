// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/tests/mocks/groupmock"
	"github.com/thunder-id/thunderid/tests/mocks/rolemock"
)

const (
	testUserID1 = "user1"
	testOUID    = "ou1"
)

type RBACEngineTestSuite struct {
	suite.Suite
	mockRoleService  *rolemock.RoleServiceInterfaceMock
	mockGroupService *groupmock.GroupServiceInterfaceMock
	engine           AuthorizationEngine
}

func TestRBACEngineTestSuite(t *testing.T) {
	suite.Run(t, new(RBACEngineTestSuite))
}

func (suite *RBACEngineTestSuite) SetupTest() {
	suite.mockRoleService = rolemock.NewRoleServiceInterfaceMock(suite.T())
	suite.mockGroupService = groupmock.NewGroupServiceInterfaceMock(suite.T())
	suite.engine = newRBACEngine(suite.mockRoleService, suite.mockGroupService)
}

func (suite *RBACEngineTestSuite) TestEvaluateAccessSuccess() {
	request := AccessEvaluationRequest{
		Subject:        Subject{ID: testUserID1, GroupIDs: []string{"group1"}},
		ResourceServer: ResourceServer{},
		Permission:     Permission{Name: "document:read"},
		OUID:           testOUID,
	}

	suite.mockRoleService.On("GetAuthorizedPermissionsByResourceServer", mock.Anything, testUserID1,
		[]string{"group1"}, mock.Anything, "", []string{"document:read"}, testOUID).
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
		OUID:           testOUID,
	}

	suite.mockRoleService.On("GetAuthorizedPermissionsByResourceServer", mock.Anything, testUserID1,
		[]string{"group1"}, mock.Anything, "", []string{"document:delete"}, testOUID).
		Return([]string{}, nil)

	result, err := suite.engine.EvaluateAccess(context.Background(), request)

	suite.Nil(err)
	suite.NotNil(result)
	suite.False(result.Decision)
}

func (suite *RBACEngineTestSuite) TestEvaluateAccessBatchBoundsRolesToEvaluationOU() {
	subject := Subject{ID: testUserID1, GroupIDs: []string{"group1"}}
	request := AccessEvaluationsRequest{
		Evaluations: []AccessEvaluationRequest{
			{Subject: subject, Permission: Permission{Name: "document:read"}, OUID: "ou1"},
			{Subject: subject, Permission: Permission{Name: "document:read"}, OUID: "ou2"},
		},
	}

	suite.mockRoleService.On("GetAuthorizedPermissionsByResourceServer", mock.Anything, testUserID1,
		[]string{"group1"}, mock.Anything, "", []string{"document:read"}, "ou1").
		Return([]string{"document:read"}, nil).Once()
	suite.mockRoleService.On("GetAuthorizedPermissionsByResourceServer", mock.Anything, testUserID1,
		[]string{"group1"}, mock.Anything, "", []string{"document:read"}, "ou2").
		Return([]string{}, nil).Once()

	result, err := suite.engine.EvaluateAccessBatch(context.Background(), request)

	suite.Nil(err)
	suite.True(result.Evaluations[0].Decision)
	suite.False(result.Evaluations[1].Decision)
}

func (suite *RBACEngineTestSuite) TestEvaluateAccessDeniesASubjectWithoutOU() {
	request := AccessEvaluationRequest{
		Subject:    Subject{ID: testUserID1, GroupIDs: []string{"group1"}, RoleIDs: []string{"mapped-role"}},
		Permission: Permission{Name: "document:read"},
	}

	result, err := suite.engine.EvaluateAccess(context.Background(), request)

	suite.Nil(err)
	suite.False(result.Decision)
	suite.mockRoleService.AssertNotCalled(suite.T(), "GetAuthorizedPermissionsByResourceServer")
	suite.mockGroupService.AssertNotCalled(suite.T(), "GetGroupsByIDs")
}

func (suite *RBACEngineTestSuite) TestEvaluateAccessBatchBoundsRolesAndGroupsToTheirOwnerOUWithoutSubject() {
	subject := Subject{GroupIDs: []string{"group-ou1", "group-ou2", "group-ou1b", "missing"},
		RoleIDs: []string{"mapped-role"}}
	request := AccessEvaluationsRequest{
		Evaluations: []AccessEvaluationRequest{
			{Subject: subject, ResourceServer: ResourceServer{ID: "rs1"}, Permission: Permission{Name: "read"}},
			{Subject: subject, ResourceServer: ResourceServer{ID: "rs1"}, Permission: Permission{Name: "write"}},
			{Subject: subject, ResourceServer: ResourceServer{ID: "rs1"}, Permission: Permission{Name: "delete"}},
			{Subject: subject, ResourceServer: ResourceServer{ID: "rs1"}, Permission: Permission{Name: "admin"}},
		},
	}
	permissions := []string{"read", "write", "delete", "admin"}

	suite.mockRoleService.On("GetAuthorizedPermissionsByResourceServer", mock.Anything, "",
		[]string(nil), []string{"mapped-role"}, "rs1", permissions, "").
		Return([]string{"read"}, nil).Once()
	suite.mockGroupService.On("GetGroupsByIDs", mock.Anything, subject.GroupIDs).
		Return(map[string]*group.Group{
			"group-ou1":  {ID: "group-ou1", OUID: "ou1"},
			"group-ou2":  {ID: "group-ou2", OUID: "ou2"},
			"group-ou1b": {ID: "group-ou1b", OUID: "ou1"},
		}, nil).Once()
	suite.mockRoleService.On("GetAuthorizedPermissionsByResourceServer", mock.Anything, "",
		[]string{"group-ou1", "group-ou1b"}, []string(nil), "rs1", permissions, "ou1").
		Return([]string{"read", "write"}, nil).Once()
	suite.mockRoleService.On("GetAuthorizedPermissionsByResourceServer", mock.Anything, "",
		[]string{"group-ou2"}, []string(nil), "rs1", permissions, "ou2").
		Return([]string{"delete"}, nil).Once()

	result, err := suite.engine.EvaluateAccessBatch(context.Background(), request)

	suite.Nil(err)
	suite.True(result.Evaluations[0].Decision)
	suite.True(result.Evaluations[1].Decision)
	suite.True(result.Evaluations[2].Decision)
	suite.False(result.Evaluations[3].Decision)
}

func (suite *RBACEngineTestSuite) TestEvaluateAccessWithoutSubjectEvaluatesOnlyItsRolesWhenItHasNoGroup() {
	request := AccessEvaluationRequest{
		Subject:    Subject{RoleIDs: []string{"mapped-role"}},
		Permission: Permission{Name: "document:read"},
	}

	suite.mockRoleService.On("GetAuthorizedPermissionsByResourceServer", mock.Anything, "",
		[]string(nil), []string{"mapped-role"}, "", []string{"document:read"}, "").
		Return([]string{"document:read"}, nil).Once()

	result, err := suite.engine.EvaluateAccess(context.Background(), request)

	suite.Nil(err)
	suite.True(result.Decision)
	suite.mockGroupService.AssertNotCalled(suite.T(), "GetGroupsByIDs")
}

func (suite *RBACEngineTestSuite) TestEvaluateAccessWithoutSubjectGroupServiceError() {
	request := AccessEvaluationRequest{
		Subject:    Subject{GroupIDs: []string{"group1"}},
		Permission: Permission{Name: "document:read"},
	}

	suite.mockGroupService.On("GetGroupsByIDs", mock.Anything, []string{"group1"}).
		Return(nil, &tidcommon.InternalServerError).Once()

	result, err := suite.engine.EvaluateAccess(context.Background(), request)

	suite.Nil(result)
	suite.ErrorContains(err, "group service error")
	suite.mockRoleService.AssertNotCalled(suite.T(), "GetAuthorizedPermissionsByResourceServer")
}

func (suite *RBACEngineTestSuite) TestEvaluateAccessWithoutSubjectRoleServiceError() {
	request := AccessEvaluationRequest{
		Subject:    Subject{GroupIDs: []string{"group1"}},
		Permission: Permission{Name: "document:read"},
	}

	suite.mockGroupService.On("GetGroupsByIDs", mock.Anything, []string{"group1"}).
		Return(map[string]*group.Group{"group1": {ID: "group1", OUID: testOUID}}, nil).Once()
	suite.mockRoleService.On("GetAuthorizedPermissionsByResourceServer", mock.Anything, "",
		[]string{"group1"}, []string(nil), "", []string{"document:read"}, testOUID).
		Return(nil, &tidcommon.InternalServerError).Once()

	result, err := suite.engine.EvaluateAccess(context.Background(), request)

	suite.Nil(result)
	suite.ErrorContains(err, "role service error")
}

func (suite *RBACEngineTestSuite) TestEvaluateAccessBatchPreservesOrder() {
	request := AccessEvaluationsRequest{
		Evaluations: []AccessEvaluationRequest{
			{
				Subject:        Subject{ID: testUserID1, GroupIDs: []string{"group1"}},
				ResourceServer: ResourceServer{},
				Permission:     Permission{Name: "document:read"},
				OUID:           testOUID,
			},
			{
				Subject:        Subject{ID: testUserID1, GroupIDs: []string{"group1"}},
				ResourceServer: ResourceServer{},
				Permission:     Permission{Name: "document:delete"},
				OUID:           testOUID,
			},
		},
	}

	suite.mockRoleService.On("GetAuthorizedPermissionsByResourceServer", mock.Anything, testUserID1,
		[]string{"group1"}, mock.Anything, "", []string{"document:read", "document:delete"}, testOUID).
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
				OUID:           testOUID,
			},
			{
				Subject:        Subject{ID: testUserID1, GroupIDs: []string{"group1"}},
				ResourceServer: ResourceServer{ID: "invoice-api"},
				Permission:     Permission{Name: "read"},
				OUID:           testOUID,
			},
		},
	}

	suite.mockRoleService.On("GetAuthorizedPermissionsByResourceServer", mock.Anything, testUserID1,
		[]string{"group1"}, mock.Anything, "booking-api", []string{"read"}, testOUID).
		Return([]string{"read"}, nil)
	suite.mockRoleService.On("GetAuthorizedPermissionsByResourceServer", mock.Anything, testUserID1,
		[]string{"group1"}, mock.Anything, "invoice-api", []string{"read"}, testOUID).
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
		OUID:           testOUID,
	}

	suite.mockRoleService.On("GetAuthorizedPermissionsByResourceServer", mock.Anything, testUserID1,
		[]string(nil), mock.Anything, "", []string{"document:read"}, testOUID).
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
		OUID:           testOUID,
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
		[]string{"group1"}, mock.Anything, "", []string{"document:read"}, testOUID).
		Return([]string(nil), roleServiceError)

	result, err := suite.engine.EvaluateAccess(context.Background(), request)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Contains(err.Error(), "role service error")
}
