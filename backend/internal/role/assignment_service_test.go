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
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
	"github.com/thunder-id/thunderid/tests/mocks/groupmock"
)

// RoleAssignmentServiceTestSuite tests the roleAssignmentService.
type RoleAssignmentServiceTestSuite struct {
	suite.Suite
	mockStore             *roleStoreInterfaceMock
	mockEntityService     *entitymock.EntityServiceInterfaceMock
	mockGroupService      *groupmock.GroupServiceInterfaceMock
	mockEntityTypeService *entitytypemock.EntityTypeServiceInterfaceMock
	transactioner         *fakeTransactioner
	service               RoleAssignmentServiceInterface
}

func TestRoleAssignmentServiceTestSuite(t *testing.T) {
	suite.Run(t, new(RoleAssignmentServiceTestSuite))
}

func (suite *RoleAssignmentServiceTestSuite) SetupTest() {
	suite.mockStore = newRoleStoreInterfaceMock(suite.T())
	suite.mockEntityService = entitymock.NewEntityServiceInterfaceMock(suite.T())
	suite.mockGroupService = groupmock.NewGroupServiceInterfaceMock(suite.T())
	suite.mockEntityTypeService = entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T())
	suite.transactioner = &fakeTransactioner{}
	suite.service = newRoleAssignmentService(
		suite.mockStore,
		suite.mockEntityService,
		suite.mockGroupService,
		suite.mockEntityTypeService,
		suite.transactioner,
	)
}

// GetRoleAssignments Tests

func (suite *RoleAssignmentServiceTestSuite) TestGetRoleAssignments_Success() {
	expectedAssignments := []RoleAssignment{
		{ID: testUserID1, Type: assigneeTypeEntity},
		{ID: "group1", Type: AssigneeTypeGroup},
	}

	suite.mockStore.On("IsRoleExist", mock.Anything,
		"role1").Return(true, nil)
	suite.mockStore.On("GetRoleAssignmentsCount", mock.Anything,
		"role1").Return(2, nil)
	suite.mockStore.On("GetRoleAssignments", mock.Anything,
		"role1", 10, 0).Return(expectedAssignments, nil)
	suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything,
		[]string{testUserID1}).Return([]entity.Entity{
		{ID: testUserID1, Category: entity.EntityCategoryUser},
	}, nil).Once()

	result, err := suite.service.GetRoleAssignments(context.Background(), "role1", 10, 0, false)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(2, result.TotalResults)
	suite.Equal(2, result.Count)
	suite.Equal(2, len(result.Assignments))
	suite.Equal(testUserID1, result.Assignments[0].ID)
	suite.Equal(AssigneeTypeUser, result.Assignments[0].Type)
	suite.Equal("group1", result.Assignments[1].ID)
	suite.Equal(AssigneeTypeGroup, result.Assignments[1].Type)
}

func (suite *RoleAssignmentServiceTestSuite) TestGetRoleAssignments_MissingID() {
	result, err := suite.service.GetRoleAssignments(context.Background(), "", 10, 0, false)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorMissingRoleID.Code, err.Code)
}

func (suite *RoleAssignmentServiceTestSuite) TestGetRoleAssignments_InvalidPagination() {
	result, err := suite.service.GetRoleAssignments(context.Background(), "role1", 0, 0, false)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidLimit.Code, err.Code)
}

func (suite *RoleAssignmentServiceTestSuite) TestGetRoleAssignments_RoleNotFound() {
	suite.mockStore.On("IsRoleExist", mock.Anything,
		"nonexistent").Return(false, nil)

	result, err := suite.service.GetRoleAssignments(context.Background(), "nonexistent", 10, 0, false)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorRoleNotFound.Code, err.Code)
}

func (suite *RoleAssignmentServiceTestSuite) TestGetRoleAssignments_GetRoleError() {
	suite.mockStore.On("IsRoleExist", mock.Anything,
		"role1").Return(false, errors.New("database error"))

	result, err := suite.service.GetRoleAssignments(context.Background(), "role1", 10, 0, false)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *RoleAssignmentServiceTestSuite) TestGetRoleAssignments_CountError() {
	suite.mockStore.On("IsRoleExist", mock.Anything,
		"role1").Return(true, nil)
	suite.mockStore.On("GetRoleAssignmentsCount", mock.Anything,
		"role1").Return(0, errors.New("count error"))

	result, err := suite.service.GetRoleAssignments(context.Background(), "role1", 10, 0, false)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *RoleAssignmentServiceTestSuite) TestGetRoleAssignments_GetListError() {
	suite.mockStore.On("IsRoleExist", mock.Anything,
		"role1").Return(true, nil)
	suite.mockStore.On("GetRoleAssignmentsCount", mock.Anything,
		"role1").Return(2, nil)
	suite.mockStore.On("GetRoleAssignments", mock.Anything,
		"role1", 10, 0).Return([]RoleAssignment{}, errors.New("list error"))

	result, err := suite.service.GetRoleAssignments(context.Background(), "role1", 10, 0, false)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *RoleAssignmentServiceTestSuite) TestGetRoleAssignments_WithDisplay_Success() {
	expectedAssignments := []RoleAssignment{
		{ID: testUserID1, Type: assigneeTypeEntity},
		{ID: "group1", Type: AssigneeTypeGroup},
	}

	suite.mockStore.On("IsRoleExist", mock.Anything,
		"role1").Return(true, nil)
	suite.mockStore.On("GetRoleAssignmentsCount", mock.Anything,
		"role1").Return(2, nil)
	suite.mockStore.On("GetRoleAssignments", mock.Anything,
		"role1", 10, 0).Return(expectedAssignments, nil)
	suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything,
		[]string{testUserID1}).Return([]entity.Entity{
		{
			ID:         testUserID1,
			Category:   entity.EntityCategoryUser,
			Type:       "employee",
			Attributes: json.RawMessage(`{"email":"alice@example.com"}`),
		},
	}, nil).Once()
	suite.mockGroupService.On("GetGroupsByIDs", mock.Anything,
		[]string{"group1"}).Return(map[string]*group.Group{
		"group1": {Name: "Test Group"},
	}, (*serviceerror.ServiceError)(nil)).Once()
	suite.mockEntityTypeService.On("GetDisplayAttributesByNames", mock.Anything, mock.Anything,
		[]string{"employee"}).Return(map[string]string{
		"employee": "email",
	}, (*serviceerror.ServiceError)(nil)).Once()

	result, err := suite.service.GetRoleAssignments(context.Background(), "role1", 10, 0, true)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(2, result.TotalResults)
	suite.Equal(2, result.Count)
	suite.Equal(AssigneeTypeUser, result.Assignments[0].Type)
	suite.Equal(AssigneeTypeGroup, result.Assignments[1].Type)
	suite.Equal("alice@example.com", result.Assignments[0].Display)
	suite.Equal("Test Group", result.Assignments[1].Display)
}

func (suite *RoleAssignmentServiceTestSuite) TestGetRoleAssignments_WithDisplay_FallbackToID() {
	expectedAssignments := []RoleAssignment{
		{ID: testUserID1, Type: assigneeTypeEntity},
	}

	suite.mockStore.On("IsRoleExist", mock.Anything,
		"role1").Return(true, nil)
	suite.mockStore.On("GetRoleAssignmentsCount", mock.Anything,
		"role1").Return(1, nil)
	suite.mockStore.On("GetRoleAssignments", mock.Anything,
		"role1", 10, 0).Return(expectedAssignments, nil)
	suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything,
		[]string{testUserID1}).Return([]entity.Entity{
		{ID: testUserID1},
	}, nil).Once()

	result, err := suite.service.GetRoleAssignments(context.Background(), "role1", 10, 0, true)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(testUserID1, result.Assignments[0].Display)
}

func (suite *RoleAssignmentServiceTestSuite) TestGetRoleAssignments_WithDisplay_FetchErrors() {
	suite.Run("User fetch error", func() {
		suite.mockStore.On("IsRoleExist", mock.Anything, "role1").Return(true, nil).Once()
		suite.mockStore.On("GetRoleAssignmentsCount", mock.Anything, "role1").Return(1, nil).Once()
		suite.mockStore.On("GetRoleAssignments", mock.Anything, "role1", 10, 0).
			Return([]RoleAssignment{{ID: testUserID1, Type: assigneeTypeEntity}}, nil).Once()
		suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything, []string{testUserID1}).
			Return([]entity.Entity(nil), errors.New("internal error")).Once()

		result, err := suite.service.GetRoleAssignments(context.Background(), "role1", 10, 0, true)

		// Entity service failure is a hard error — no silent fallback.
		suite.NotNil(err)
		suite.Nil(result)
	})

	suite.Run("Group fetch error", func() {
		suite.mockStore.On("IsRoleExist", mock.Anything, "role1").Return(true, nil).Once()
		suite.mockStore.On("GetRoleAssignmentsCount", mock.Anything, "role1").Return(1, nil).Once()
		suite.mockStore.On("GetRoleAssignments", mock.Anything, "role1", 10, 0).
			Return([]RoleAssignment{{ID: "group1", Type: AssigneeTypeGroup}}, nil).Once()
		suite.mockGroupService.On("GetGroupsByIDs", mock.Anything, []string{"group1"}).
			Return((map[string]*group.Group)(nil), &serviceerror.ServiceError{Code: "INTERNAL_ERROR"}).Once()

		result, err := suite.service.GetRoleAssignments(context.Background(), "role1", 10, 0, true)

		// Group display fetch error is a soft warning — response still returned, display falls back to ID.
		suite.Nil(err)
		suite.NotNil(result)
		suite.Equal(1, result.TotalResults)
		suite.Equal(1, result.Count)
		suite.Equal("group1", result.Assignments[0].Display)
	})
}

func (suite *RoleAssignmentServiceTestSuite) TestGetRoleAssignments_WithDisplay_PartialResults() {
	expectedAssignments := []RoleAssignment{
		{ID: testUserID1, Type: assigneeTypeEntity},
		{ID: "group1", Type: AssigneeTypeGroup},
	}

	suite.mockStore.On("IsRoleExist", mock.Anything,
		"role1").Return(true, nil)
	suite.mockStore.On("GetRoleAssignmentsCount", mock.Anything,
		"role1").Return(2, nil)
	suite.mockStore.On("GetRoleAssignments", mock.Anything,
		"role1", 10, 0).Return(expectedAssignments, nil)
	// Entity not found in service — orphaned assignment, skipped in output.
	suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything,
		[]string{testUserID1}).Return([]entity.Entity{}, nil).Once()
	// Group found but not in map — display falls back to ID.
	suite.mockGroupService.On("GetGroupsByIDs", mock.Anything,
		[]string{"group1"}).Return(map[string]*group.Group{}, (*serviceerror.ServiceError)(nil)).Once()

	result, err := suite.service.GetRoleAssignments(context.Background(), "role1", 10, 0, true)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(2, result.TotalResults)
	// Orphaned entity assignment is dropped; only the group remains.
	suite.Equal(1, result.Count)
	suite.Equal("group1", result.Assignments[0].Display)
}

func (suite *RoleAssignmentServiceTestSuite) TestGetRoleAssignments_WithDisplay_NestedDisplayAttribute() {
	expectedAssignments := []RoleAssignment{
		{ID: testUserID1, Type: assigneeTypeEntity},
	}

	suite.mockStore.On("IsRoleExist", mock.Anything,
		"role1").Return(true, nil)
	suite.mockStore.On("GetRoleAssignmentsCount", mock.Anything,
		"role1").Return(1, nil)
	suite.mockStore.On("GetRoleAssignments", mock.Anything,
		"role1", 10, 0).Return(expectedAssignments, nil)
	suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything,
		[]string{testUserID1}).Return([]entity.Entity{
		{
			ID:         testUserID1,
			Category:   entity.EntityCategoryUser,
			Type:       "employee",
			Attributes: json.RawMessage(`{"profile":{"fullName":"Alice Smith"}}`),
		},
	}, nil).Once()
	suite.mockEntityTypeService.On("GetDisplayAttributesByNames", mock.Anything, mock.Anything,
		[]string{"employee"}).Return(map[string]string{
		"employee": "profile.fullName",
	}, (*serviceerror.ServiceError)(nil)).Once()

	result, err := suite.service.GetRoleAssignments(context.Background(), "role1", 10, 0, true)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal("Alice Smith", result.Assignments[0].Display)
}

func (suite *RoleAssignmentServiceTestSuite) TestGetRoleAssignments_WithDisplay_SchemaServiceError() {
	expectedAssignments := []RoleAssignment{
		{ID: testUserID1, Type: assigneeTypeEntity},
	}

	suite.mockStore.On("IsRoleExist", mock.Anything,
		"role1").Return(true, nil)
	suite.mockStore.On("GetRoleAssignmentsCount", mock.Anything,
		"role1").Return(1, nil)
	suite.mockStore.On("GetRoleAssignments", mock.Anything,
		"role1", 10, 0).Return(expectedAssignments, nil)
	suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything,
		[]string{testUserID1}).Return([]entity.Entity{
		{
			ID:         testUserID1,
			Category:   entity.EntityCategoryUser,
			Type:       "employee",
			Attributes: json.RawMessage(`{"email":"alice@example.com"}`),
		},
	}, nil).Once()
	// Schema service fails — should fall back to user ID.
	suite.mockEntityTypeService.On("GetDisplayAttributesByNames", mock.Anything, mock.Anything,
		[]string{"employee"}).Return(
		(map[string]string)(nil), &serviceerror.ServiceError{Code: "INTERNAL_ERROR"},
	).Once()

	result, err := suite.service.GetRoleAssignments(context.Background(), "role1", 10, 0, true)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(testUserID1, result.Assignments[0].Display)
}

// GetRoleAssignmentsByType Tests

func (suite *RoleAssignmentServiceTestSuite) TestGetRoleAssignmentsByType_UserFilter_Success() {
	// Verifies that ?type=user fetches all entity assignments, filters to user-category
	// entities, paginates in memory, and returns public AssigneeTypeUser (not entity).
	suite.mockStore.On("IsRoleExist", mock.Anything, "role1").Return(true, nil).Once()
	suite.mockStore.On("GetRoleAssignmentsCountByType", mock.Anything, "role1",
		string(assigneeTypeEntity)).Return(2, nil).Once()
	suite.mockStore.On("GetRoleAssignmentsByType", mock.Anything, "role1", 2, 0,
		string(assigneeTypeEntity)).Return([]RoleAssignment{
		{ID: testUserID1, Type: assigneeTypeEntity},
		{ID: "app-001", Type: assigneeTypeEntity},
	}, nil).Once()
	suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything,
		mock.MatchedBy(func(ids []string) bool { return len(ids) == 2 })).
		Return([]entity.Entity{
			{ID: testUserID1, Category: entity.EntityCategoryUser},
			{ID: "app-001", Category: entity.EntityCategoryApp},
		}, nil).Once()
	// resolveAssignments fetches entity details for the filtered user page.
	suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything,
		[]string{testUserID1}).Return([]entity.Entity{
		{ID: testUserID1, Category: entity.EntityCategoryUser},
	}, nil).Once()

	result, err := suite.service.GetRoleAssignmentsByType(
		context.Background(), "role1", 10, 0, false, "user")

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(1, result.TotalResults)
	suite.Equal(1, result.Count)
	suite.Equal(1, len(result.Assignments))
	suite.Equal(testUserID1, result.Assignments[0].ID)
	suite.Equal(AssigneeTypeUser, result.Assignments[0].Type)
}

func (suite *RoleAssignmentServiceTestSuite) TestGetRoleAssignmentsByType_EntityServiceFailure() {
	// When entity service fails during category batch-fetch in getAssignmentsByEntityCategory,
	// the call should return a hard error (not a soft warn with empty results).
	suite.mockStore.On("IsRoleExist", mock.Anything, "role1").Return(true, nil).Once()
	suite.mockStore.On("GetRoleAssignmentsCountByType", mock.Anything, "role1",
		string(assigneeTypeEntity)).Return(1, nil).Once()
	suite.mockStore.On("GetRoleAssignmentsByType", mock.Anything, "role1", 1, 0,
		string(assigneeTypeEntity)).Return([]RoleAssignment{
		{ID: testUserID1, Type: assigneeTypeEntity},
	}, nil).Once()
	suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything,
		[]string{testUserID1}).Return([]entity.Entity(nil), errors.New("entity service down")).Once()

	result, err := suite.service.GetRoleAssignmentsByType(
		context.Background(), "role1", 10, 0, false, "user")

	suite.NotNil(err)
	suite.Nil(result)
}

// AddAssignments Tests

func (suite *RoleAssignmentServiceTestSuite) TestAddAssignments_MissingRoleID() {
	request := []RoleAssignment{
		{ID: testUserID1, Type: AssigneeTypeUser},
	}

	err := suite.service.AddAssignments(context.Background(), "", request)

	suite.NotNil(err)
	suite.Equal(ErrorMissingRoleID.Code, err.Code)
}

func (suite *RoleAssignmentServiceTestSuite) TestAddAssignments_EmptyAssignments() {
	err := suite.service.AddAssignments(context.Background(), "role1", []RoleAssignment{})

	suite.NotNil(err)
	suite.Equal(ErrorEmptyAssignments.Code, err.Code)
}

func (suite *RoleAssignmentServiceTestSuite) TestAddAssignments_InvalidAssignmentFormat() {
	testCases := []struct {
		name        string
		assignment  RoleAssignment
		expectedErr string
	}{
		{
			name:        "InvalidType",
			assignment:  RoleAssignment{ID: testUserID1, Type: "invalid_type"},
			expectedErr: ErrorInvalidAssigneeType.Code,
		},
		{
			name:        "EmptyID",
			assignment:  RoleAssignment{ID: "", Type: AssigneeTypeUser},
			expectedErr: ErrorInvalidRequestFormat.Code,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			err := suite.service.AddAssignments(context.Background(), "role1", []RoleAssignment{tc.assignment})
			suite.NotNil(err)
			suite.Equal(tc.expectedErr, err.Code)
		})
	}
}

func (suite *RoleAssignmentServiceTestSuite) TestAddAssignments_RoleNotFound() {
	request := []RoleAssignment{
		{ID: testUserID1, Type: AssigneeTypeUser},
	}

	suite.mockStore.On("IsRoleExist", mock.Anything,
		"nonexistent").Return(false, nil)

	err := suite.service.AddAssignments(context.Background(), "nonexistent", request)

	suite.NotNil(err)
	suite.Equal(ErrorRoleNotFound.Code, err.Code)
}

func (suite *RoleAssignmentServiceTestSuite) TestAddAssignments_GetRoleError() {
	request := []RoleAssignment{
		{ID: testUserID1, Type: AssigneeTypeUser},
	}

	suite.mockStore.On("IsRoleExist", mock.Anything,
		"role1").Return(false, errors.New("database error"))

	err := suite.service.AddAssignments(context.Background(), "role1", request)

	suite.NotNil(err)
	suite.Equal(ErrorInternalServerError.Code, err.Code)
}

func (suite *RoleAssignmentServiceTestSuite) TestAddAssignments_StoreError() {
	request := []RoleAssignment{
		{ID: testUserID1, Type: AssigneeTypeUser},
	}
	normalized := []RoleAssignment{
		{ID: testUserID1, Type: assigneeTypeEntity},
	}

	suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything,
		[]string{testUserID1}).Return([]entity.Entity{
		{ID: testUserID1, Category: entity.EntityCategoryUser},
	}, nil)
	suite.mockStore.On("IsRoleExist", mock.Anything,
		"role1").Return(true, nil)
	suite.mockStore.On("AddAssignments", mock.Anything,
		"role1", normalized).Return(errors.New("store error"))

	err := suite.service.AddAssignments(context.Background(), "role1", request)

	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *RoleAssignmentServiceTestSuite) TestAddAssignments_Success() {
	request := []RoleAssignment{
		{ID: testUserID1, Type: AssigneeTypeUser},
	}
	normalized := []RoleAssignment{
		{ID: testUserID1, Type: assigneeTypeEntity},
	}

	suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything,
		[]string{testUserID1}).Return([]entity.Entity{
		{ID: testUserID1, Category: entity.EntityCategoryUser},
	}, nil)
	suite.mockStore.On("IsRoleExist", mock.Anything,
		"role1").Return(true, nil)
	suite.mockStore.On("AddAssignments", mock.Anything,
		"role1", normalized).Return(nil)

	err := suite.service.AddAssignments(context.Background(), "role1", request)

	suite.Nil(err)
}

// RemoveAssignments Tests

func (suite *RoleAssignmentServiceTestSuite) TestRemoveAssignments_MissingRoleID() {
	request := []RoleAssignment{
		{ID: testUserID1, Type: AssigneeTypeUser},
	}

	err := suite.service.RemoveAssignments(context.Background(), "", request)

	suite.NotNil(err)
	suite.Equal(ErrorMissingRoleID.Code, err.Code)
}

func (suite *RoleAssignmentServiceTestSuite) TestRemoveAssignments_EmptyAssignments() {
	err := suite.service.RemoveAssignments(context.Background(), "role1", []RoleAssignment{})

	suite.NotNil(err)
	suite.Equal(ErrorEmptyAssignments.Code, err.Code)
}

func (suite *RoleAssignmentServiceTestSuite) TestRemoveAssignments_RoleNotFound() {
	request := []RoleAssignment{
		{ID: testUserID1, Type: AssigneeTypeUser},
	}

	suite.mockStore.On("IsRoleExist", mock.Anything,
		"nonexistent").Return(false, nil)

	err := suite.service.RemoveAssignments(context.Background(), "nonexistent", request)

	suite.NotNil(err)
	suite.Equal(ErrorRoleNotFound.Code, err.Code)
}

func (suite *RoleAssignmentServiceTestSuite) TestRemoveAssignments_GetRoleError() {
	request := []RoleAssignment{
		{ID: testUserID1, Type: AssigneeTypeUser},
	}

	suite.mockStore.On("IsRoleExist", mock.Anything,
		"role1").Return(false, errors.New("database error"))

	err := suite.service.RemoveAssignments(context.Background(), "role1", request)

	suite.NotNil(err)
	suite.Equal(ErrorInternalServerError.Code, err.Code)
}

func (suite *RoleAssignmentServiceTestSuite) TestRemoveAssignments_StoreError() {
	request := []RoleAssignment{
		{ID: testUserID1, Type: AssigneeTypeUser},
	}
	normalized := []RoleAssignment{
		{ID: testUserID1, Type: assigneeTypeEntity},
	}

	suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything,
		[]string{testUserID1}).Return([]entity.Entity{
		{ID: testUserID1, Category: entity.EntityCategoryUser},
	}, nil)
	suite.mockStore.On("IsRoleExist", mock.Anything,
		"role1").Return(true, nil)
	suite.mockStore.On("RemoveAssignments", mock.Anything,
		"role1", normalized).Return(errors.New("store error"))

	err := suite.service.RemoveAssignments(context.Background(), "role1", request)

	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *RoleAssignmentServiceTestSuite) TestRemoveAssignments_Success() {
	request := []RoleAssignment{
		{ID: testUserID1, Type: AssigneeTypeUser},
	}
	normalized := []RoleAssignment{
		{ID: testUserID1, Type: assigneeTypeEntity},
	}

	suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything,
		[]string{testUserID1}).Return([]entity.Entity{
		{ID: testUserID1, Category: entity.EntityCategoryUser},
	}, nil)
	suite.mockStore.On("IsRoleExist", mock.Anything,
		"role1").Return(true, nil)
	suite.mockStore.On("RemoveAssignments", mock.Anything,
		"role1", normalized).Return(nil)

	err := suite.service.RemoveAssignments(context.Background(), "role1", request)

	suite.Nil(err)
}
