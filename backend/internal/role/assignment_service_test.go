// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/group"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/revocation"
	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
	"github.com/thunder-id/thunderid/tests/mocks/groupmock"
	"github.com/thunder-id/thunderid/tests/mocks/resourcemock"
	"github.com/thunder-id/thunderid/tests/mocks/sharingmock"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
	"github.com/thunder-id/thunderid/tests/mocks/transactionmock"
)

// RoleAssignmentServiceTestSuite tests the roleAssignmentService.
type RoleAssignmentServiceTestSuite struct {
	suite.Suite
	mockStore             *roleStoreInterfaceMock
	mockEntityService     *entitymock.EntityServiceInterfaceMock
	mockGroupService      *groupmock.GroupServiceInterfaceMock
	mockEntityTypeService *entitytypemock.EntityTypeServiceInterfaceMock
	mockSharingService    *sharingmock.SharingServiceInterfaceMock
	transactioner         *fakeTransactioner
	service               RoleAssignmentServiceInterface
}

func TestRoleAssignmentServiceTestSuite(t *testing.T) {
	suite.Run(t, new(RoleAssignmentServiceTestSuite))
}

// registerSharedCategories answers the shared resource type lookup as the server does: users and
// applications are shareable, agents are not.
func registerSharedCategories(entitySvc *entitymock.EntityServiceInterfaceMock) {
	entitySvc.On("GetSharedResourceType", providers.EntityCategoryUser).
		Return(sharing.ResourceType("user"), true).Maybe()
	entitySvc.On("GetSharedResourceType", providers.EntityCategoryApp).
		Return(sharing.ResourceType("application"), true).Maybe()
	entitySvc.On("GetSharedResourceType", providers.EntityCategoryAgent).
		Return(sharing.ResourceType(""), false).Maybe()
}

func (suite *RoleAssignmentServiceTestSuite) SetupTest() {
	suite.mockStore = newRoleStoreInterfaceMock(suite.T())
	suite.mockEntityService = entitymock.NewEntityServiceInterfaceMock(suite.T())
	registerSharedCategories(suite.mockEntityService)
	suite.mockGroupService = groupmock.NewGroupServiceInterfaceMock(suite.T())
	suite.mockEntityTypeService = entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T())
	suite.transactioner = &fakeTransactioner{}
	suite.mockSharingService = sharingmock.NewSharingServiceInterfaceMock(suite.T())
	suite.service = newRoleAssignmentService(
		suite.mockStore,
		suite.mockEntityService,
		suite.mockGroupService,
		suite.mockEntityTypeService,
		nil,
		suite.transactioner,
		newAllowAllRoleAuthz(suite.T()),
		suite.mockSharingService,
		discardingBus{},
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
		"role1", mock.Anything).Return(2, nil)
	suite.mockStore.On("GetRoleAssignments", mock.Anything,
		"role1", mock.Anything, 10, 0).Return(expectedAssignments, nil)
	suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything,
		[]string{testUserID1}).Return([]providers.Entity{
		{ID: testUserID1, Category: providers.EntityCategoryUser},
	}, nil).Once()

	result, err := suite.service.GetRoleAssignments(context.Background(), "role1", "", 10, 0, false)

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
	result, err := suite.service.GetRoleAssignments(context.Background(), "", "", 10, 0, false)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorMissingRoleID.Code, err.Code)
}

func (suite *RoleAssignmentServiceTestSuite) TestGetRoleAssignments_InvalidPagination() {
	result, err := suite.service.GetRoleAssignments(context.Background(), "role1", "", 0, 0, false)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidLimit.Code, err.Code)
}

func (suite *RoleAssignmentServiceTestSuite) TestGetRoleAssignments_RoleNotFound() {
	suite.mockStore.On("IsRoleExist", mock.Anything,
		"nonexistent").Return(false, nil)

	result, err := suite.service.GetRoleAssignments(context.Background(), "nonexistent", "", 10, 0, false)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorRoleNotFound.Code, err.Code)
}

func (suite *RoleAssignmentServiceTestSuite) TestGetRoleAssignments_GetRoleError() {
	suite.mockStore.On("IsRoleExist", mock.Anything,
		"role1").Return(false, errors.New("database error"))

	result, err := suite.service.GetRoleAssignments(context.Background(), "role1", "", 10, 0, false)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *RoleAssignmentServiceTestSuite) TestGetRoleAssignments_CountError() {
	suite.mockStore.On("IsRoleExist", mock.Anything,
		"role1").Return(true, nil)
	suite.mockStore.On("GetRoleAssignmentsCount", mock.Anything,
		"role1", mock.Anything).Return(0, errors.New("count error"))

	result, err := suite.service.GetRoleAssignments(context.Background(), "role1", "", 10, 0, false)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *RoleAssignmentServiceTestSuite) TestGetRoleAssignments_GetListError() {
	suite.mockStore.On("IsRoleExist", mock.Anything,
		"role1").Return(true, nil)
	suite.mockStore.On("GetRoleAssignmentsCount", mock.Anything,
		"role1", mock.Anything).Return(2, nil)
	suite.mockStore.On("GetRoleAssignments", mock.Anything,
		"role1", mock.Anything, 10, 0).Return([]RoleAssignment{}, errors.New("list error"))

	result, err := suite.service.GetRoleAssignments(context.Background(), "role1", "", 10, 0, false)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *RoleAssignmentServiceTestSuite) TestGetRoleAssignments_WithDisplay_Success() {
	expectedAssignments := []RoleAssignment{
		{ID: testUserID1, Type: assigneeTypeEntity},
		{ID: "group1", Type: AssigneeTypeGroup},
	}

	suite.mockStore.On("IsRoleExist", mock.Anything,
		"role1").Return(true, nil)
	suite.mockStore.On("GetRoleAssignmentsCount", mock.Anything,
		"role1", mock.Anything).Return(2, nil)
	suite.mockStore.On("GetRoleAssignments", mock.Anything,
		"role1", mock.Anything, 10, 0).Return(expectedAssignments, nil)
	suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything,
		[]string{testUserID1}).Return([]providers.Entity{
		{
			ID:         testUserID1,
			Category:   providers.EntityCategoryUser,
			Type:       "employee",
			Attributes: json.RawMessage(`{"email":"alice@example.com"}`),
		},
	}, nil).Once()
	suite.mockGroupService.On("GetGroupsByIDs", mock.Anything,
		[]string{"group1"}).Return(map[string]*group.Group{
		"group1": {Name: "Test Group"},
	}, (*tidcommon.ServiceError)(nil)).Once()
	suite.mockEntityTypeService.On("GetDisplayAttributesByHandles", mock.Anything, mock.Anything,
		[]string{"employee"}).Return(map[string]string{
		"employee": "email",
	}, (*tidcommon.ServiceError)(nil)).Once()

	result, err := suite.service.GetRoleAssignments(context.Background(), "role1", "", 10, 0, true)

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
		"role1", mock.Anything).Return(1, nil)
	suite.mockStore.On("GetRoleAssignments", mock.Anything,
		"role1", mock.Anything, 10, 0).Return(expectedAssignments, nil)
	suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything,
		[]string{testUserID1}).Return([]providers.Entity{
		{ID: testUserID1},
	}, nil).Once()

	result, err := suite.service.GetRoleAssignments(context.Background(), "role1", "", 10, 0, true)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(testUserID1, result.Assignments[0].Display)
}

func (suite *RoleAssignmentServiceTestSuite) TestGetRoleAssignments_WithDisplay_FetchErrors() {
	suite.Run("User fetch error", func() {
		suite.mockStore.On("IsRoleExist", mock.Anything, "role1").Return(true, nil).Once()
		suite.mockStore.On("GetRoleAssignmentsCount", mock.Anything, "role1", mock.Anything).Return(1, nil).Once()
		suite.mockStore.On("GetRoleAssignments", mock.Anything, "role1", mock.Anything, 10, 0).
			Return([]RoleAssignment{{ID: testUserID1, Type: assigneeTypeEntity}}, nil).Once()
		suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything, []string{testUserID1}).
			Return([]providers.Entity(nil), errors.New("internal error")).Once()

		result, err := suite.service.GetRoleAssignments(context.Background(), "role1", "", 10, 0, true)

		// Entity service failure is a hard error — no silent fallback.
		suite.NotNil(err)
		suite.Nil(result)
	})

	suite.Run("Group fetch error", func() {
		suite.mockStore.On("IsRoleExist", mock.Anything, "role1").Return(true, nil).Once()
		suite.mockStore.On("GetRoleAssignmentsCount", mock.Anything, "role1", mock.Anything).Return(1, nil).Once()
		suite.mockStore.On("GetRoleAssignments", mock.Anything, "role1", mock.Anything, 10, 0).
			Return([]RoleAssignment{{ID: "group1", Type: AssigneeTypeGroup}}, nil).Once()
		suite.mockGroupService.On("GetGroupsByIDs", mock.Anything, []string{"group1"}).
			Return((map[string]*group.Group)(nil), &tidcommon.ServiceError{Code: "INTERNAL_ERROR"}).Once()

		result, err := suite.service.GetRoleAssignments(context.Background(), "role1", "", 10, 0, true)

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
		"role1", mock.Anything).Return(2, nil)
	suite.mockStore.On("GetRoleAssignments", mock.Anything,
		"role1", mock.Anything, 10, 0).Return(expectedAssignments, nil)
	// Entity not found in service — orphaned assignment, skipped in output.
	suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything,
		[]string{testUserID1}).Return([]providers.Entity{}, nil).Once()
	// Group found but not in map — display falls back to ID.
	suite.mockGroupService.On("GetGroupsByIDs", mock.Anything,
		[]string{"group1"}).Return(map[string]*group.Group{}, (*tidcommon.ServiceError)(nil)).Once()

	result, err := suite.service.GetRoleAssignments(context.Background(), "role1", "", 10, 0, true)

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
		"role1", mock.Anything).Return(1, nil)
	suite.mockStore.On("GetRoleAssignments", mock.Anything,
		"role1", mock.Anything, 10, 0).Return(expectedAssignments, nil)
	suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything,
		[]string{testUserID1}).Return([]providers.Entity{
		{
			ID:         testUserID1,
			Category:   providers.EntityCategoryUser,
			Type:       "employee",
			Attributes: json.RawMessage(`{"profile":{"fullName":"Alice Smith"}}`),
		},
	}, nil).Once()
	suite.mockEntityTypeService.On("GetDisplayAttributesByHandles", mock.Anything, mock.Anything,
		[]string{"employee"}).Return(map[string]string{
		"employee": "profile.fullName",
	}, (*tidcommon.ServiceError)(nil)).Once()

	result, err := suite.service.GetRoleAssignments(context.Background(), "role1", "", 10, 0, true)

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
		"role1", mock.Anything).Return(1, nil)
	suite.mockStore.On("GetRoleAssignments", mock.Anything,
		"role1", mock.Anything, 10, 0).Return(expectedAssignments, nil)
	suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything,
		[]string{testUserID1}).Return([]providers.Entity{
		{
			ID:         testUserID1,
			Category:   providers.EntityCategoryUser,
			Type:       "employee",
			Attributes: json.RawMessage(`{"email":"alice@example.com"}`),
		},
	}, nil).Once()
	// Schema service fails — should fall back to user ID.
	suite.mockEntityTypeService.On("GetDisplayAttributesByHandles", mock.Anything, mock.Anything,
		[]string{"employee"}).Return(
		(map[string]string)(nil), &tidcommon.ServiceError{Code: "INTERNAL_ERROR"},
	).Once()

	result, err := suite.service.GetRoleAssignments(context.Background(), "role1", "", 10, 0, true)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(testUserID1, result.Assignments[0].Display)
}

func (suite *RoleAssignmentServiceTestSuite) TestGetRoleAssignments_IncludeDisplay_AgentUsesDisplayAttribute() {
	suite.mockStore.On("IsRoleExist", mock.Anything, "role1").Return(true, nil)
	suite.mockStore.On("GetRoleAssignmentsCount", mock.Anything, "role1", mock.Anything).Return(2, nil)
	suite.mockStore.On("GetRoleAssignments", mock.Anything, "role1", mock.Anything, 10, 0).Return([]RoleAssignment{
		{ID: "agent-1", Type: assigneeTypeEntity},
		{ID: "agent-2", Type: assigneeTypeEntity},
	}, nil)
	suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything,
		mock.MatchedBy(func(ids []string) bool { return len(ids) == 2 })).Return([]providers.Entity{
		{ID: "agent-1", Category: providers.EntityCategoryAgent, Type: "default",
			Attributes: json.RawMessage(`{"name":"Ledger Agent"}`)},
		{ID: "agent-2", Category: providers.EntityCategoryAgent, Type: "default",
			Attributes: json.RawMessage(`{"model":"x"}`)},
	}, nil).Once()
	suite.mockEntityTypeService.On("GetDisplayAttributesByHandles", mock.Anything,
		entitytype.TypeCategoryAgent, []string{"default"}).Return(
		map[string]string{"default": "name"}, (*tidcommon.ServiceError)(nil)).Once()

	result, err := suite.service.GetRoleAssignments(context.Background(), "role1", "", 10, 0, true)

	suite.Nil(err)
	suite.Require().Len(result.Assignments, 2)
	suite.Equal("Ledger Agent", result.Assignments[0].Display)
	suite.Equal("agent-2", result.Assignments[1].Display, "falls back to the ID when the name is absent")
}

// GetRoleAssignmentsByType Tests

func (suite *RoleAssignmentServiceTestSuite) TestGetRoleAssignmentsByType_UserFilter_Success() {
	// Verifies that ?type=user fetches all entity assignments, filters to user-category
	// entities, paginates in memory, and returns public AssigneeTypeUser (not entity).
	suite.mockStore.On("IsRoleExist", mock.Anything, "role1").Return(true, nil).Once()
	suite.mockStore.On("GetRoleAssignmentsCountByType", mock.Anything, "role1", mock.Anything,
		string(assigneeTypeEntity)).Return(2, nil).Once()
	suite.mockStore.On("GetRoleAssignmentsByType", mock.Anything, "role1", mock.Anything, 2, 0,
		string(assigneeTypeEntity)).Return([]RoleAssignment{
		{ID: testUserID1, Type: assigneeTypeEntity},
		{ID: "app-001", Type: assigneeTypeEntity},
	}, nil).Once()
	suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything,
		mock.MatchedBy(func(ids []string) bool { return len(ids) == 2 })).
		Return([]providers.Entity{
			{ID: testUserID1, Category: providers.EntityCategoryUser},
			{ID: "app-001", Category: providers.EntityCategoryApp},
		}, nil).Once()
	// resolveAssignments fetches entity details for the filtered user page.
	suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything,
		[]string{testUserID1}).Return([]providers.Entity{
		{ID: testUserID1, Category: providers.EntityCategoryUser},
	}, nil).Once()

	result, err := suite.service.GetRoleAssignmentsByType(
		context.Background(), "role1", "", 10, 0, false, "user")

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
	suite.mockStore.On("GetRoleAssignmentsCountByType", mock.Anything, "role1", mock.Anything,
		string(assigneeTypeEntity)).Return(1, nil).Once()
	suite.mockStore.On("GetRoleAssignmentsByType", mock.Anything, "role1", mock.Anything, 1, 0,
		string(assigneeTypeEntity)).Return([]RoleAssignment{
		{ID: testUserID1, Type: assigneeTypeEntity},
	}, nil).Once()
	suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything,
		[]string{testUserID1}).Return([]providers.Entity(nil), errors.New("entity service down")).Once()

	result, err := suite.service.GetRoleAssignmentsByType(
		context.Background(), "role1", "", 10, 0, false, "user")

	suite.NotNil(err)
	suite.Nil(result)
}

// AddAssignments Tests

func (suite *RoleAssignmentServiceTestSuite) TestAddAssignments_MissingRoleID() {
	request := []RoleAssignment{
		{ID: testUserID1, Type: AssigneeTypeUser},
	}

	err := suite.service.AddAssignments(context.Background(), "", "", request)

	suite.NotNil(err)
	suite.Equal(ErrorMissingRoleID.Code, err.Code)
}

func (suite *RoleAssignmentServiceTestSuite) TestAddAssignments_EmptyAssignments() {
	err := suite.service.AddAssignments(context.Background(), "role1", "", []RoleAssignment{})

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
			err := suite.service.AddAssignments(context.Background(), "role1", "", []RoleAssignment{tc.assignment})
			suite.NotNil(err)
			suite.Equal(tc.expectedErr, err.Code)
		})
	}
}

func (suite *RoleAssignmentServiceTestSuite) TestAddAssignments_RoleNotFound() {
	request := []RoleAssignment{
		{ID: testUserID1, Type: AssigneeTypeUser},
	}

	suite.mockStore.On("GetRole", mock.Anything, "nonexistent").Return(RoleWithPermissions{}, ErrRoleNotFound)

	err := suite.service.AddAssignments(context.Background(), "nonexistent", "", request)

	suite.NotNil(err)
	suite.Equal(ErrorRoleNotFound.Code, err.Code)
}

func (suite *RoleAssignmentServiceTestSuite) TestAddAssignments_GetRoleError() {
	request := []RoleAssignment{
		{ID: testUserID1, Type: AssigneeTypeUser},
	}

	suite.mockStore.On("GetRole", mock.Anything, "role1").Return(RoleWithPermissions{}, errors.New("database error"))

	err := suite.service.AddAssignments(context.Background(), "role1", "", request)

	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *RoleAssignmentServiceTestSuite) TestAddAssignments_Success() {
	request := []RoleAssignment{
		{ID: testUserID1, Type: AssigneeTypeUser},
	}
	normalized := []RoleAssignment{
		{ID: testUserID1, Type: assigneeTypeEntity, OUID: testOwnerOU},
	}

	suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything,
		[]string{testUserID1}).Return([]providers.Entity{
		ownedUser(),
	}, nil)
	suite.ownedRole("role1")
	suite.mockStore.On("AddAssignments", mock.Anything,
		"role1", normalized).Return(nil)

	err := suite.service.AddAssignments(context.Background(), "role1", "", request)

	suite.Nil(err)
}

// ownedRole stubs a role owned by testOwnerOU.
func (suite *RoleAssignmentServiceTestSuite) ownedRole(id string) *mock.Call {
	return suite.mockStore.On("GetRole", mock.Anything, id).Return(RoleWithPermissions{ID: id, OUID: testOwnerOU}, nil)
}

// ownedUser is testUserID1, a user owned by testOwnerOU.
func ownedUser() providers.Entity {
	return providers.Entity{ID: testUserID1, Category: providers.EntityCategoryUser, OUID: testOwnerOU}
}

// Organization unit scoped assignments

const testShareeOU = "ou-child"

// sharedWithChild stubs role1, owned by testOwnerOU, as shared with testShareeOU with its
// assignments open or turned off there.
func (suite *RoleAssignmentServiceTestSuite) sharedWithChild(assignmentsOpen bool) {
	suite.ownedRole("role1")
	suite.mockSharingService.EXPECT().ResolveOverlayRules(mock.Anything, RoleSharingType, "role1", testShareeOU).
		Return(sharing.ResolvedOverlay{
			OUID: testShareeOU, Visible: true,
			Rules: map[string]sharing.OverlayRule{roleAssignmentsFieldKey: {Editable: assignmentsOpen}},
		}, nil)
}

// An organization unit the role is shared with assigns it to its own users and to users shared
// with it, and the assignments are recorded there.
func (suite *RoleAssignmentServiceTestSuite) TestAddAssignments_InSharedOU() {
	suite.sharedWithChild(true)
	suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything, mock.Anything).Return([]providers.Entity{
		{ID: "own-user", Category: providers.EntityCategoryUser, OUID: testShareeOU},
		{ID: "shared-user", Category: providers.EntityCategoryUser, OUID: testOwnerOU},
	}, nil)
	suite.mockSharingService.EXPECT().IsVisible(mock.Anything, sharing.ResourceType("user"), "shared-user",
		testShareeOU).Return(true, nil)
	suite.mockStore.On("AddAssignments", mock.Anything, "role1", []RoleAssignment{
		{ID: "own-user", Type: assigneeTypeEntity, OUID: testShareeOU},
		{ID: "shared-user", Type: assigneeTypeEntity, OUID: testShareeOU},
	}).Return(nil)

	err := suite.service.AddAssignments(context.Background(), "role1", testShareeOU, []RoleAssignment{
		{ID: "own-user", Type: AssigneeTypeUser}, {ID: "shared-user", Type: AssigneeTypeUser},
	})

	suite.Nil(err)
}

func (suite *RoleAssignmentServiceTestSuite) TestAddAssignments_InSharedOU_AssignmentsTurnedOff() {
	suite.sharedWithChild(false)

	err := suite.service.AddAssignments(context.Background(), "role1", testShareeOU, []RoleAssignment{
		{ID: testUserID1, Type: AssigneeTypeUser},
	})

	suite.Require().NotNil(err)
	suite.Equal(ErrorSharedRoleFieldNotEditable.Code, err.Code)
	suite.mockStore.AssertNotCalled(suite.T(), "AddAssignments", mock.Anything, mock.Anything, mock.Anything)
}

// Taking an assignment away only narrows what someone holds, so it is allowed even where the
// role's assignments are turned off, and wherever the assignee lives.
func (suite *RoleAssignmentServiceTestSuite) TestRemoveAssignments_InSharedOU_AssignmentsTurnedOff() {
	suite.sharedWithChild(false)
	suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything, []string{testUserID1}).Return([]providers.Entity{
		{ID: testUserID1, Category: providers.EntityCategoryUser, OUID: "elsewhere"},
	}, nil)
	suite.mockStore.On("GetDirectAssignments", mock.Anything, "role1", testShareeOU, []string{testUserID1}).
		Return([]RoleAssignment{{ID: testUserID1, Type: assigneeTypeEntity, OUID: testShareeOU}}, nil)
	suite.mockStore.On("ListMandatedAssignmentsForRole", mock.Anything, "role1", testShareeOU).
		Return([]RoleAssignment{}, nil)
	suite.mockStore.On("RemoveAssignments", mock.Anything, "role1", []RoleAssignment{
		{ID: testUserID1, Type: assigneeTypeEntity, OUID: testShareeOU},
	}).Return(nil)

	err := suite.service.RemoveAssignments(context.Background(), "role1", testShareeOU, []RoleAssignment{
		{ID: testUserID1, Type: AssigneeTypeUser},
	})

	suite.Nil(err)
}

func (suite *RoleAssignmentServiceTestSuite) TestAddAssignments_InOUNotShared() {
	suite.ownedRole("role1")
	suite.mockSharingService.EXPECT().ResolveOverlayRules(mock.Anything, RoleSharingType, "role1", "stranger").
		Return(sharing.ResolvedOverlay{OUID: "stranger"}, nil)
	suite.mockSharingService.EXPECT().ResolveOverlayRules(mock.Anything, RoleSharingType, "role1", "gone").
		Return(sharing.ResolvedOverlay{}, &oupkg.ErrorOrganizationUnitNotFound)
	suite.mockSharingService.EXPECT().ResolveOverlayRules(mock.Anything, RoleSharingType, "role1", "broken").
		Return(sharing.ResolvedOverlay{}, &tidcommon.InternalServerError)

	for ouID, code := range map[string]string{
		"stranger": ErrorRoleNotSharedToOU.Code,
		"gone":     ErrorRoleNotSharedToOU.Code,
		"broken":   tidcommon.InternalServerError.Code,
	} {
		err := suite.service.AddAssignments(context.Background(), "role1", ouID, []RoleAssignment{
			{ID: testUserID1, Type: AssigneeTypeUser},
		})
		suite.Require().NotNil(err, ouID)
		suite.Equal(code, err.Code, ouID)
	}
}

// An assignee the organization unit neither owns nor has shared with it is refused. Agents are
// never shared, so one from elsewhere is refused without asking.
func (suite *RoleAssignmentServiceTestSuite) TestAddAssignments_AssigneeNotInOU() {
	suite.ownedRole("role1")

	suite.Run("unshared user", func() {
		suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything, []string{"stranger"}).Return(
			[]providers.Entity{{ID: "stranger", Category: providers.EntityCategoryUser, OUID: "elsewhere"}}, nil).Once()
		suite.mockSharingService.EXPECT().IsVisible(mock.Anything, sharing.ResourceType("user"), "stranger",
			testOwnerOU).Return(false, nil).Once()

		err := suite.service.AddAssignments(context.Background(), "role1", "", []RoleAssignment{
			{ID: "stranger", Type: AssigneeTypeUser},
		})
		suite.Require().NotNil(err)
		suite.Equal(ErrorAssigneeNotInOU.Code, err.Code)
	})
	suite.Run("agent from elsewhere", func() {
		suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything, []string{"agent"}).Return(
			[]providers.Entity{{ID: "agent", Category: providers.EntityCategoryAgent, OUID: "elsewhere"}}, nil).Once()

		err := suite.service.AddAssignments(context.Background(), "role1", "", []RoleAssignment{
			{ID: "agent", Type: AssigneeTypeAgent},
		})
		suite.Require().NotNil(err)
		suite.Equal(ErrorAssigneeNotInOU.Code, err.Code)
	})
	suite.Run("group from elsewhere", func() {
		suite.mockGroupService.On("ValidateGroupIDs", mock.Anything, []string{"grp"}).Return(nil).Once()
		suite.mockGroupService.On("GetGroupsByIDs", mock.Anything, []string{"grp"}).
			Return(map[string]*group.Group{"grp": {ID: "grp", OUID: "elsewhere"}}, nil).Once()

		err := suite.service.AddAssignments(context.Background(), "role1", "", []RoleAssignment{
			{ID: "grp", Type: AssigneeTypeGroup},
		})
		suite.Require().NotNil(err)
		suite.Equal(ErrorAssigneeNotInOU.Code, err.Code)
	})
	suite.Run("sharing lookup fails", func() {
		suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything, []string{"app"}).Return(
			[]providers.Entity{{ID: "app", Category: providers.EntityCategoryApp, OUID: "elsewhere"}}, nil).Once()
		suite.mockSharingService.EXPECT().IsVisible(mock.Anything, sharing.ResourceType("application"), "app",
			testOwnerOU).Return(false, &tidcommon.InternalServerError).Once()

		err := suite.service.AddAssignments(context.Background(), "role1", "", []RoleAssignment{
			{ID: "app", Type: AssigneeTypeApp},
		})
		suite.Require().NotNil(err)
		suite.Equal(tidcommon.InternalServerError.Code, err.Code)
	})
	suite.mockStore.AssertNotCalled(suite.T(), "AddAssignments", mock.Anything, mock.Anything, mock.Anything)
}

// A provisioned entity is assigned in the organization unit it lives in, which the role reaches
// through sharing.
func (suite *RoleAssignmentServiceTestSuite) TestAddAssigneesToRoles_AssignsInTheEntitysOU() {
	suite.sharedWithChild(true)
	suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything, []string{testUserID1}).Return(
		[]providers.Entity{{ID: testUserID1, Category: providers.EntityCategoryUser, OUID: testShareeOU}}, nil)
	suite.mockStore.On("AddAssignments", mock.Anything, "role1", []RoleAssignment{
		{ID: testUserID1, Type: assigneeTypeEntity, OUID: testShareeOU},
	}).Return(nil)

	err := suite.service.AddAssigneesToRoles(context.Background(),
		[]RoleAssignment{{ID: testUserID1, Type: AssigneeTypeUser}}, []string{"role1"})

	suite.Nil(err)
}

func (suite *RoleAssignmentServiceTestSuite) TestAddAssigneesToRoles_UnknownEntity() {
	suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything, []string{"ghost"}).Return([]providers.Entity{}, nil)

	err := suite.service.AddAssigneesToRoles(context.Background(),
		[]RoleAssignment{{ID: "ghost", Type: AssigneeTypeUser}}, []string{"role1"})

	suite.Require().NotNil(err)
	suite.Equal(ErrorInvalidAssignmentID.Code, err.Code)
}

// An assignment naming an organization unit is made there, and one naming none is made where its
// assignee lives.
func (suite *RoleAssignmentServiceTestSuite) TestAddAssigneesToRoles_NamedAndAssigneeOUs() {
	suite.sharedWithChild(true)
	suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything, []string{"child-user"}).Return(
		[]providers.Entity{{ID: "child-user", Category: providers.EntityCategoryUser, OUID: testShareeOU}}, nil)
	suite.mockGroupService.On("ValidateGroupIDs", mock.Anything, []string{"grp"}).Return(nil)
	suite.mockGroupService.On("GetGroupsByIDs", mock.Anything, []string{"grp"}).
		Return(map[string]*group.Group{"grp": {ID: "grp", OUID: testOwnerOU}}, nil)
	suite.mockStore.On("AddAssignments", mock.Anything, "role1", []RoleAssignment{
		{ID: "child-user", Type: assigneeTypeEntity, OUID: testShareeOU},
	}).Return(nil).Once()
	suite.mockStore.On("AddAssignments", mock.Anything, "role1", []RoleAssignment{
		{ID: "grp", Type: AssigneeTypeGroup, OUID: testOwnerOU},
	}).Return(nil).Once()

	err := suite.service.AddAssigneesToRoles(context.Background(), []RoleAssignment{
		{ID: "child-user", Type: AssigneeTypeUser, OUID: testShareeOU},
		{ID: "grp", Type: AssigneeTypeGroup},
	}, []string{"role1"})

	suite.Nil(err)
}

// An assignment in an organization unit the role is not shared with is refused, and none is added.
func (suite *RoleAssignmentServiceTestSuite) TestAddAssigneesToRoles_UnsharedOU() {
	suite.ownedRole("role1")
	suite.mockSharingService.EXPECT().ResolveOverlayRules(mock.Anything, RoleSharingType, "role1", "ou-other").
		Return(sharing.ResolvedOverlay{OUID: "ou-other"}, nil)

	err := suite.service.AddAssigneesToRoles(context.Background(), []RoleAssignment{
		{ID: testUserID1, Type: AssigneeTypeUser, OUID: "ou-other"},
	}, []string{"role1"})

	suite.Require().NotNil(err)
	suite.Equal(ErrorRoleNotSharedToOU.Code, err.Code)
	suite.mockStore.AssertNotCalled(suite.T(), "AddAssignments", mock.Anything, mock.Anything, mock.Anything)
}

func (suite *RoleAssignmentServiceTestSuite) TestAddAssigneesToRoles_NothingToAdd() {
	suite.Nil(suite.service.AddAssigneesToRoles(context.Background(), nil, []string{"role1"}))
}

// A filter on the organization unit lists the assignments made there, and its links keep it.
func (suite *RoleAssignmentServiceTestSuite) TestGetRoleAssignments_InOU() {
	suite.mockStore.On("IsRoleExist", mock.Anything, "role1").Return(true, nil)
	suite.mockStore.On("GetRoleAssignmentsCount", mock.Anything, "role1", testShareeOU).Return(3, nil)
	suite.mockStore.On("GetRoleAssignments", mock.Anything, "role1", testShareeOU, 1, 0).
		Return([]RoleAssignment{{ID: "grp", Type: AssigneeTypeGroup, OUID: testShareeOU}}, nil)

	result, err := suite.service.GetRoleAssignments(context.Background(), "role1", testShareeOU, 1, 0, false)

	suite.Require().Nil(err)
	suite.Require().Len(result.Assignments, 1)
	suite.Equal(testShareeOU, result.Assignments[0].OUID)
	suite.Require().NotEmpty(result.Links)
	suite.Contains(result.Links[0].Href, "filter=ouId+eq+%22ou-child%22")
}

// RemoveAssignments Tests

func (suite *RoleAssignmentServiceTestSuite) TestRemoveAssignments_MissingRoleID() {
	request := []RoleAssignment{
		{ID: testUserID1, Type: AssigneeTypeUser},
	}

	err := suite.service.RemoveAssignments(context.Background(), "", "", request)

	suite.NotNil(err)
	suite.Equal(ErrorMissingRoleID.Code, err.Code)
}

func (suite *RoleAssignmentServiceTestSuite) TestRemoveAssignments_EmptyAssignments() {
	err := suite.service.RemoveAssignments(context.Background(), "role1", "", []RoleAssignment{})

	suite.NotNil(err)
	suite.Equal(ErrorEmptyAssignments.Code, err.Code)
}

func (suite *RoleAssignmentServiceTestSuite) TestRemoveAssignments_RoleNotFound() {
	request := []RoleAssignment{
		{ID: testUserID1, Type: AssigneeTypeUser},
	}

	suite.mockStore.On("GetRole", mock.Anything, "nonexistent").Return(RoleWithPermissions{}, ErrRoleNotFound)

	err := suite.service.RemoveAssignments(context.Background(), "nonexistent", "", request)

	suite.NotNil(err)
	suite.Equal(ErrorRoleNotFound.Code, err.Code)
}

func (suite *RoleAssignmentServiceTestSuite) TestRemoveAssignments_GetRoleError() {
	request := []RoleAssignment{
		{ID: testUserID1, Type: AssigneeTypeUser},
	}

	suite.mockStore.On("GetRole", mock.Anything, "role1").Return(RoleWithPermissions{}, errors.New("database error"))

	err := suite.service.RemoveAssignments(context.Background(), "role1", "", request)

	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

// A store failure while adding or removing assignments is a server error.
func (suite *RoleAssignmentServiceTestSuite) TestModifyAssignments_StoreError() {
	request := []RoleAssignment{{ID: testUserID1, Type: AssigneeTypeUser}}
	normalized := []RoleAssignment{{ID: testUserID1, Type: assigneeTypeEntity, OUID: testOwnerOU}}
	suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything, []string{testUserID1}).
		Return([]providers.Entity{ownedUser()}, nil)
	suite.ownedRole("role1")
	suite.mockStore.On("AddAssignments", mock.Anything, "role1", normalized).Return(errors.New("store error"))
	suite.mockStore.On("GetDirectAssignments", mock.Anything, "role1", testOwnerOU, []string{testUserID1}).
		Return(normalized, nil)
	suite.mockStore.On("ListMandatedAssignmentsForRole", mock.Anything, "role1", testOwnerOU).
		Return([]RoleAssignment{}, nil)
	suite.mockStore.On("RemoveAssignments", mock.Anything, "role1", normalized).Return(errors.New("store error"))

	type modifyFunc func(context.Context, string, string, []RoleAssignment) *tidcommon.ServiceError
	for name, modify := range map[string]modifyFunc{
		"add":    suite.service.AddAssignments,
		"remove": suite.service.RemoveAssignments,
	} {
		err := modify(context.Background(), "role1", "", request)

		suite.Require().NotNil(err, name)
		suite.Equal(tidcommon.InternalServerError.Code, err.Code, name)
	}
}

func (suite *RoleAssignmentServiceTestSuite) TestRemoveAssignments_Success() {
	request := []RoleAssignment{
		{ID: testUserID1, Type: AssigneeTypeUser},
	}
	normalized := []RoleAssignment{
		{ID: testUserID1, Type: assigneeTypeEntity, OUID: testOwnerOU},
	}

	suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything,
		[]string{testUserID1}).Return([]providers.Entity{
		ownedUser(),
	}, nil)
	suite.ownedRole("role1")
	suite.mockStore.On("GetDirectAssignments", mock.Anything, "role1", testOwnerOU, []string{testUserID1}).
		Return(normalized, nil)
	suite.mockStore.On("ListMandatedAssignmentsForRole", mock.Anything, "role1", testOwnerOU).
		Return([]RoleAssignment{}, nil)
	suite.mockStore.On("RemoveAssignments", mock.Anything,
		"role1", normalized).Return(nil)

	err := suite.service.RemoveAssignments(context.Background(), "role1", "", request)

	suite.Nil(err)
}

func (suite *RoleAssignmentServiceTestSuite) TestGetRoleAssignments_IsRoleExist_DatabaseError() {
	suite.mockStore.On("IsRoleExist", mock.Anything, "role1").
		Return(false, errors.New("mock database tracking timeout")).Once()

	result, err := suite.service.GetRoleAssignments(context.Background(), "role1", "", 10, 0, false)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *RoleAssignmentServiceTestSuite) TestAddAssignments_Transaction_DatabaseError() {
	assignments := []RoleAssignment{{ID: "user1", Type: "user"}}

	suite.ownedRole("role1").Once()
	suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything, []string{"user1"}).
		Return([]providers.Entity{ownedUser()}, nil).Once()

	suite.transactioner.err = errors.New("failed to commit transaction block")

	err := suite.service.AddAssignments(context.Background(), "role1", "", assignments)

	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *RoleAssignmentServiceTestSuite) TestGetRoleAssignmentsByType_CountStoreError() {
	suite.mockStore.On("IsRoleExist", mock.Anything, "role-1").Return(true, nil).Once()

	suite.mockStore.On("GetRoleAssignmentsCountByType", mock.Anything, "role-1", mock.Anything, "entity").
		Return(0, errors.New("mock db counter crash")).Once()

	result, err := suite.service.GetRoleAssignmentsByType(context.Background(), "role-1", "", 10, 0, false, "user")
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *RoleAssignmentServiceTestSuite) TestGetRoleAssignmentsByType_ListStoreError() {
	suite.mockStore.On("IsRoleExist", mock.Anything, "role-1").Return(true, nil).Once()

	suite.mockStore.On("GetRoleAssignmentsCountByType", mock.Anything, "role-1", mock.Anything, "entity").
		Return(1, nil).Once()

	suite.mockStore.On("GetRoleAssignmentsByType", mock.Anything, "role-1", mock.Anything, 1, 0, "entity").
		Return([]RoleAssignment{}, errors.New("mock db row scan failure")).Once()

	result, err := suite.service.GetRoleAssignmentsByType(context.Background(), "role-1", "", 10, 0, false, "user")
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

// AddAssigneesToRoles Tests

func (suite *RoleAssignmentServiceTestSuite) TestAddAssigneesToRoles_EmptyRoleIDs() {
	assignments := []RoleAssignment{{ID: testUserID1, Type: AssigneeTypeUser}}
	err := suite.service.AddAssigneesToRoles(context.Background(), assignments, []string{})
	suite.Nil(err)
}

func (suite *RoleAssignmentServiceTestSuite) TestAddAssigneesToRoles_AddAssignmentsFails() {
	assignments := []RoleAssignment{{ID: testUserID1, Type: AssigneeTypeUser}}
	suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything, []string{testUserID1}).
		Return([]providers.Entity{ownedUser()}, nil).Once()
	suite.mockStore.On("GetRole", mock.Anything, "role1").Return(RoleWithPermissions{}, ErrRoleNotFound).Once()

	err := suite.service.AddAssigneesToRoles(context.Background(), assignments, []string{"role1"})

	suite.NotNil(err)
	suite.Equal(ErrorRoleNotFound.Code, err.Code)
}

func (suite *RoleAssignmentServiceTestSuite) TestAddAssigneesToRoles_TransactionerError() {
	assignments := []RoleAssignment{{ID: testUserID1, Type: AssigneeTypeUser}}
	suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything, []string{testUserID1}).
		Return([]providers.Entity{ownedUser()}, nil).Once()
	suite.transactioner.err = errors.New("tx begin failed")

	err := suite.service.AddAssigneesToRoles(context.Background(), assignments, []string{"role1"})

	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *RoleAssignmentServiceTestSuite) TestAddAssigneesToRoles_Success() {
	assignments := []RoleAssignment{{ID: testUserID1, Type: AssigneeTypeUser}}
	normalized := []RoleAssignment{{ID: testUserID1, Type: assigneeTypeEntity, OUID: testOwnerOU}}

	suite.ownedRole("role1").Once()
	suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything, []string{testUserID1}).
		Return([]providers.Entity{ownedUser()}, nil).Times(2)
	suite.mockStore.On("AddAssignments", mock.Anything, "role1", normalized).Return(nil).Once()

	err := suite.service.AddAssigneesToRoles(context.Background(), assignments, []string{"role1"})

	suite.Nil(err)
}

func (suite *RoleAssignmentServiceTestSuite) TestAddAssigneesToRoles_MultipleRoles_Success() {
	assignments := []RoleAssignment{{ID: testUserID1, Type: AssigneeTypeUser}}
	normalized := []RoleAssignment{{ID: testUserID1, Type: assigneeTypeEntity, OUID: testOwnerOU}}

	suite.ownedRole("role1").Once()
	suite.ownedRole("role2").Once()
	suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything, []string{testUserID1}).
		Return([]providers.Entity{ownedUser()}, nil).Times(3)
	suite.mockStore.On("AddAssignments", mock.Anything, "role1", normalized).Return(nil).Once()
	suite.mockStore.On("AddAssignments", mock.Anything, "role2", normalized).Return(nil).Once()

	err := suite.service.AddAssigneesToRoles(
		context.Background(), assignments, []string{"role1", "role2"})

	suite.Nil(err)
}

// --- Cascade / dependency provider ---

func (suite *RoleAssignmentServiceTestSuite) TestGetResourceDependencies_ReturnsEmpty() {
	result, err := suite.service.GetResourceDependencies(
		context.Background(), resourcedependency.ResourceTypeUser, "user-1")
	suite.NoError(err)
	suite.Empty(result)
}

func (suite *RoleAssignmentServiceTestSuite) TestDeleteAssignmentsInOU() {
	suite.mockStore.On("DeleteAssignmentsInOU", mock.Anything, "role1", "ou-child").Return(nil).Once()
	suite.NoError(suite.service.DeleteAssignmentsInOU(context.Background(), "role1", "ou-child"))

	suite.mockStore.On("DeleteAssignmentsInOU", mock.Anything, "role1", "ou-gone").
		Return(errors.New("db error")).Once()
	suite.Error(suite.service.DeleteAssignmentsInOU(context.Background(), "role1", "ou-gone"))
}

func (suite *RoleAssignmentServiceTestSuite) TestCascadeDeleteDependencies_User_DeletesAssignments() {
	suite.mockStore.On("DeleteAssignmentsByAssignee", mock.Anything, string(assigneeTypeEntity), "user-1").
		Return(int64(3), nil)

	deleted, err := suite.service.CascadeDeleteDependencies(
		context.Background(), resourcedependency.ResourceTypeUser, "user-1")

	suite.NoError(err)
	suite.Equal(3, deleted)
}

func (suite *RoleAssignmentServiceTestSuite) TestCascadeDeleteDependencies_UnknownType_NoOp() {
	deleted, err := suite.service.CascadeDeleteDependencies(context.Background(), "theme", "theme-1")

	suite.NoError(err)
	suite.Equal(0, deleted)
	suite.mockStore.AssertNotCalled(suite.T(), "DeleteAssignmentsByAssignee",
		mock.Anything, mock.Anything, mock.Anything)
}

func (suite *RoleAssignmentServiceTestSuite) TestCascadeDeleteDependencies_StoreError() {
	suite.mockStore.On("DeleteAssignmentsByAssignee", mock.Anything, string(assigneeTypeEntity), "user-1").
		Return(int64(0), errors.New("db error"))

	deleted, err := suite.service.CascadeDeleteDependencies(
		context.Background(), resourcedependency.ResourceTypeUser, "user-1")

	suite.Error(err)
	suite.Equal(0, deleted)
}

func (suite *RoleAssignmentServiceTestSuite) TestHasGroupRoleAssignments() {
	suite.Run("group assigned a role", func() {
		suite.mockStore.On("HasAssigneeAssignments", mock.Anything, string(AssigneeTypeGroup), "grp-1").
			Return(true, nil).Once()

		found, err := suite.service.HasGroupRoleAssignments(context.Background(), "grp-1")

		suite.Nil(err)
		suite.True(found)
	})
	suite.Run("group assigned no role", func() {
		suite.mockStore.On("HasAssigneeAssignments", mock.Anything, string(AssigneeTypeGroup), "grp-2").
			Return(false, nil).Once()

		found, err := suite.service.HasGroupRoleAssignments(context.Background(), "grp-2")

		suite.Nil(err)
		suite.False(found)
	})
	suite.Run("store error", func() {
		suite.mockStore.On("HasAssigneeAssignments", mock.Anything, string(AssigneeTypeGroup), "grp-3").
			Return(false, errors.New("db error")).Once()

		found, err := suite.service.HasGroupRoleAssignments(context.Background(), "grp-3")

		suite.Require().NotNil(err)
		suite.Equal(tidcommon.InternalServerError.Code, err.Code)
		suite.False(found)
	})
}

func (suite *RoleAssignmentServiceTestSuite) TestCascadeDeleteDependencies_Group_DeletesAssignments() {
	suite.mockStore.On("DeleteAssignmentsByAssignee", mock.Anything, string(AssigneeTypeGroup), "group-1").
		Return(int64(1), nil)

	deleted, err := suite.service.CascadeDeleteDependencies(
		context.Background(), resourcedependency.ResourceTypeGroup, "group-1")

	suite.NoError(err)
	suite.Equal(1, deleted)
}

// Mandated assignments

const testPolicyID = "policy-1"

// withRoleStore runs the test with roles in the given store mode.
func (suite *RoleAssignmentServiceTestSuite) withRoleStore(mode serverconst.StoreMode) {
	config.ResetServerRuntime()
	suite.Require().NoError(config.InitializeServerRuntime("", &config.Config{
		Role: config.RoleConfig{Store: string(mode)},
	}))
	suite.T().Cleanup(config.ResetServerRuntime)
}

// mandated is the row a sharing policy holds for testUserID1.
func mandated(roleID, ouID string) MandatedAssignment {
	return MandatedAssignment{RoleID: roleID, Assignment: RoleAssignment{
		ID: testUserID1, Type: assigneeTypeEntity, OUID: ouID, PolicyID: testPolicyID}}
}

// The policy's rows follow desired: a unit that gains a role gets only that role, a unit that loses one
// is rewritten, a unit no longer named loses its rows, and an unchanged unit is left alone.
func (suite *RoleAssignmentServiceTestSuite) TestApplyMandatedAssignments_SyncsThePolicysRows() {
	suite.withRoleStore(serverconst.StoreModeMutable)
	for _, roleID := range []string{"role-a", "role-b", "role-c"} {
		suite.ownedRole(roleID)
	}
	suite.mockSharingService.EXPECT().IsVisible(mock.Anything, RoleSharingType, mock.Anything, mock.Anything).
		Return(true, nil)
	suite.mockStore.On("ListMandatedAssignments", mock.Anything, testPolicyID).Return([]MandatedAssignment{
		mandated("role-a", "ou-grow"), mandated("role-b", "ou-grow"),
		mandated("role-a", "ou-narrow"), mandated("role-b", "ou-narrow"),
		mandated("role-a", "ou-dropped"),
		mandated("role-a", "ou-same"),
	}, nil).Once()
	suite.mockStore.On("AddMandatedAssignments", mock.Anything, testPolicyID, "role-c", "ou-grow", mandatedUser).
		Return(nil).Once()
	suite.mockStore.On("DeleteMandatedAssignments", mock.Anything, testPolicyID, MandatedFilter{OUID: "ou-narrow"}).
		Return(nil).Once()
	suite.mockStore.On("AddMandatedAssignments", mock.Anything, testPolicyID, "role-b", "ou-narrow", mandatedUser).
		Return(nil).Once()
	suite.mockStore.On("DeleteMandatedAssignments", mock.Anything, testPolicyID, MandatedFilter{OUID: "ou-dropped"}).
		Return(nil).Once()
	suite.mockStore.On("AddMandatedAssignments", mock.Anything, testPolicyID, "role-a", "ou-new", mandatedUser).
		Return(nil).Once()

	err := suite.service.ApplyMandatedAssignments(context.Background(), testPolicyID,
		RoleAssignment{ID: testUserID1, Type: AssigneeTypeUser}, map[string][]string{
			"ou-grow":   {"role-a", "role-b", "role-c", "role-c"},
			"ou-narrow": {"role-b"},
			"ou-same":   {"role-a"},
			"ou-new":    {"role-a"},
			"ou-empty":  {},
		})

	suite.Nil(err)
}

// withGrantChecks rebuilds the service around an authorization mock with no grants allowed unless a test
// stubs them.
func (suite *RoleAssignmentServiceTestSuite) withGrantChecks() *sysauthzmock.SystemAuthorizationServiceInterfaceMock {
	authz := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(suite.T())
	suite.service = newRoleAssignmentService(suite.mockStore, suite.mockEntityService, suite.mockGroupService,
		suite.mockEntityTypeService, nil, suite.transactioner, authz, suite.mockSharingService, discardingBus{})
	return authz
}

// Mandating a role conveys its permissions, so a caller who may not grant the role is refused with the
// error a direct assignment gets, and nothing is written.
func (suite *RoleAssignmentServiceTestSuite) TestApplyMandatedAssignments_RefusesARoleTheCallerCannotGrant() {
	suite.withRoleStore(serverconst.StoreModeMutable)
	authz := suite.withGrantChecks()
	suite.ownedRole("role-a")
	suite.mockStore.On("ListMandatedAssignments", mock.Anything, testPolicyID).Return([]MandatedAssignment{}, nil)
	authz.On("CanGrantMembership", mock.Anything, sysauthz.PrincipalTypeRole, "role-a").
		Return(&sysauthz.ErrorGrantNotPermitted).Once()

	err := suite.service.ApplyMandatedAssignments(context.Background(), testPolicyID,
		RoleAssignment{ID: testUserID1, Type: AssigneeTypeUser}, map[string][]string{testOwnerOU: {"role-a"}})

	suite.Require().NotNil(err)
	suite.Equal(sysauthz.ErrorGrantNotPermitted.Code, err.Code)
	suite.mockStore.AssertNotCalled(suite.T(), "AddMandatedAssignments",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	suite.mockStore.AssertNotCalled(suite.T(), "DeleteMandatedAssignments", mock.Anything, mock.Anything, mock.Anything)
}

// Only a role newly mandated in a unit is checked, once however many units it is new in. A role the policy
// already held there, and every removal, is not, so dropping a role or a whole share is never blocked.
func (suite *RoleAssignmentServiceTestSuite) TestApplyMandatedAssignments_ChecksOnlyNewlyMandatedRoles() {
	suite.withRoleStore(serverconst.StoreModeMutable)
	authz := suite.withGrantChecks()
	suite.ownedRole("role-b")
	suite.mockSharingService.EXPECT().IsVisible(mock.Anything, RoleSharingType, "role-b", mock.Anything).
		Return(true, nil)
	suite.mockStore.On("ListMandatedAssignments", mock.Anything, testPolicyID).Return([]MandatedAssignment{
		mandated("role-a", "ou-1"), mandated("role-b", "ou-1"), mandated("role-a", "ou-dropped"),
	}, nil)
	authz.On("CanGrantMembership", mock.Anything, sysauthz.PrincipalTypeRole, "role-b").Return(nil).Once()
	suite.mockStore.On("DeleteMandatedAssignments", mock.Anything, testPolicyID, MandatedFilter{OUID: "ou-1"}).
		Return(nil).Once()
	suite.mockStore.On("AddMandatedAssignments", mock.Anything, testPolicyID, "role-b", "ou-1", mandatedUser).
		Return(nil).Once()
	suite.mockStore.On("DeleteMandatedAssignments", mock.Anything, testPolicyID, MandatedFilter{OUID: "ou-dropped"}).
		Return(nil).Once()
	suite.mockStore.On("AddMandatedAssignments", mock.Anything, testPolicyID, "role-b", "ou-2", mandatedUser).
		Return(nil).Once()
	suite.mockStore.On("AddMandatedAssignments", mock.Anything, testPolicyID, "role-b", "ou-3", mandatedUser).
		Return(nil).Once()

	suite.Nil(suite.service.ApplyMandatedAssignments(context.Background(), testPolicyID,
		RoleAssignment{ID: testUserID1, Type: AssigneeTypeUser}, map[string][]string{
			"ou-1": {"role-b"},
			"ou-2": {"role-b"},
			"ou-3": {"role-b"},
		}))
	suite.mockStore.On("DeleteMandatedAssignments", mock.Anything, testPolicyID, MandatedFilter{}).Return(nil).Once()
	suite.Nil(suite.service.RemoveMandatedAssignments(context.Background(), testPolicyID))
}

// A declared policy is applied at startup under the runtime context, which the grant check lets through.
func (suite *RoleAssignmentServiceTestSuite) TestApplyMandatedAssignments_DeclaredAtStartup() {
	suite.withRoleStore(serverconst.StoreModeMutable)
	authz := suite.withGrantChecks()
	suite.ownedRole("role-a")
	suite.mockStore.On("ListMandatedAssignments", mock.Anything, testPolicyID).Return([]MandatedAssignment{}, nil)
	authz.On("CanGrantMembership", mock.MatchedBy(security.IsRuntimeContext), sysauthz.PrincipalTypeRole, "role-a").
		Return(nil).Once()
	suite.mockStore.On("AddMandatedAssignments", mock.Anything, testPolicyID, "role-a", testOwnerOU, mandatedUser).
		Return(nil).Once()

	suite.Nil(suite.service.ApplyMandatedAssignments(security.WithRuntimeContext(context.Background()),
		testPolicyID, RoleAssignment{ID: testUserID1, Type: AssigneeTypeUser},
		map[string][]string{testOwnerOU: {"role-a"}}))
}

// Any client error from the visibility check means the role is not usable in the unit.
func (suite *RoleAssignmentServiceTestSuite) TestApplyMandatedAssignments_VisibilityClientErrorIsNotInOU() {
	suite.withRoleStore(serverconst.StoreModeMutable)
	suite.ownedRole("role-a")
	suite.mockSharingService.EXPECT().IsVisible(mock.Anything, RoleSharingType, "role-a", testShareeOU).
		Return(false, &sharing.ErrorResourceTypeNotRegistered)

	err := suite.service.ApplyMandatedAssignments(context.Background(), testPolicyID,
		RoleAssignment{ID: testUserID1, Type: AssigneeTypeUser}, map[string][]string{testShareeOU: {"role-a"}})

	suite.Require().NotNil(err)
	suite.Equal(ErrorMandatedRoleNotInOU.Code, err.Code)
}

// Applying what the policy already holds writes nothing, so a declared policy re-applied at every
// startup changes nothing.
func (suite *RoleAssignmentServiceTestSuite) TestApplyMandatedAssignments_IsIdempotent() {
	suite.withRoleStore(serverconst.StoreModeMutable)
	suite.ownedRole("role-a")
	suite.mockSharingService.EXPECT().IsVisible(mock.Anything, RoleSharingType, "role-a", testShareeOU).
		Return(true, nil)
	suite.mockStore.On("ListMandatedAssignments", mock.Anything, testPolicyID).
		Return([]MandatedAssignment{mandated("role-a", testShareeOU)}, nil)

	err := suite.service.ApplyMandatedAssignments(context.Background(), testPolicyID,
		RoleAssignment{ID: testUserID1, Type: AssigneeTypeUser}, map[string][]string{testShareeOU: {"role-a"}})

	suite.Nil(err)
	suite.mockStore.AssertNotCalled(suite.T(), "AddMandatedAssignments",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	suite.mockStore.AssertNotCalled(suite.T(), "DeleteMandatedAssignments", mock.Anything, mock.Anything, mock.Anything)
}

// A role is mandated in the organization unit owning it without asking whether it is shared there.
func (suite *RoleAssignmentServiceTestSuite) TestApplyMandatedAssignments_InTheRolesOwnOU() {
	suite.withRoleStore(serverconst.StoreModeMutable)
	suite.ownedRole("role-a")
	suite.mockStore.On("ListMandatedAssignments", mock.Anything, testPolicyID).Return([]MandatedAssignment{}, nil)
	suite.mockStore.On("AddMandatedAssignments", mock.Anything, testPolicyID, "role-a", testOwnerOU, mandatedUser).
		Return(nil).Once()

	err := suite.service.ApplyMandatedAssignments(context.Background(), testPolicyID,
		RoleAssignment{ID: testUserID1, Type: AssigneeTypeUser}, map[string][]string{testOwnerOU: {"role-a"}})

	suite.Nil(err)
}

// A role neither owned by nor shared with the organization unit is refused there, and nothing is written.
func (suite *RoleAssignmentServiceTestSuite) TestApplyMandatedAssignments_RefusesARoleNotInTheOU() {
	cases := []struct {
		name  string
		stub  func()
		code  string
		ouID  string
		roles []string
	}{
		{"not shared", func() {
			suite.ownedRole("role-a")
			suite.mockSharingService.EXPECT().IsVisible(mock.Anything, RoleSharingType, "role-a", testShareeOU).
				Return(false, nil)
		}, ErrorMandatedRoleNotInOU.Code, testShareeOU, []string{"role-a"}},
		{"no such role", func() {
			suite.mockStore.On("GetRole", mock.Anything, "ghost").Return(RoleWithPermissions{}, ErrRoleNotFound)
		}, ErrorMandatedRoleNotInOU.Code, testShareeOU, []string{"ghost"}},
		{"no such organization unit", func() {
			suite.ownedRole("role-a")
			suite.mockSharingService.EXPECT().IsVisible(mock.Anything, RoleSharingType, "role-a", "gone").
				Return(false, &oupkg.ErrorOrganizationUnitNotFound)
		}, ErrorMandatedRoleNotInOU.Code, "gone", []string{"role-a"}},
		{"sharing fails", func() {
			suite.ownedRole("role-a")
			suite.mockSharingService.EXPECT().IsVisible(mock.Anything, RoleSharingType, "role-a", testShareeOU).
				Return(false, &tidcommon.InternalServerError)
		}, tidcommon.InternalServerError.Code, testShareeOU, []string{"role-a"}},
		{"role read fails", func() {
			suite.mockStore.On("GetRole", mock.Anything, "role-a").Return(RoleWithPermissions{}, errors.New("db down"))
		}, tidcommon.InternalServerError.Code, testShareeOU, []string{"role-a"}},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			suite.withRoleStore(serverconst.StoreModeMutable)
			tc.stub()

			err := suite.service.ApplyMandatedAssignments(context.Background(), testPolicyID,
				RoleAssignment{ID: testUserID1, Type: AssigneeTypeUser}, map[string][]string{tc.ouID: tc.roles})

			suite.Require().NotNil(err)
			suite.Equal(tc.code, err.Code)
			suite.mockStore.AssertNotCalled(suite.T(), "ListMandatedAssignments", mock.Anything, mock.Anything)
		})
	}
}

// The policy is written in the caller's transaction: the transactioner is handed the caller's context,
// which a real one joins, and every store call runs on it.
func (suite *RoleAssignmentServiceTestSuite) TestApplyMandatedAssignments_JoinsTheCallersTransaction() {
	type callerTx struct{}
	suite.withRoleStore(serverconst.StoreModeMutable)
	caller := context.WithValue(context.Background(), callerTx{}, true)
	inCallerTx := mock.MatchedBy(func(ctx context.Context) bool { return ctx.Value(callerTx{}) != nil })
	txr := transactionmock.NewTransactionerMock(suite.T())
	txr.EXPECT().Transact(inCallerTx, mock.Anything).
		RunAndReturn(func(ctx context.Context, txFunc func(context.Context) error) error {
			return txFunc(ctx)
		}).Once()
	suite.service = newRoleAssignmentService(suite.mockStore, suite.mockEntityService, suite.mockGroupService,
		suite.mockEntityTypeService, nil, txr, newAllowAllRoleAuthz(suite.T()), suite.mockSharingService,
		discardingBus{})
	suite.mockStore.On("GetRole", inCallerTx, "role-a").
		Return(RoleWithPermissions{ID: "role-a", OUID: testOwnerOU}, nil)
	suite.mockSharingService.EXPECT().IsVisible(inCallerTx, RoleSharingType, "role-a", testShareeOU).Return(true, nil)
	suite.mockStore.On("ListMandatedAssignments", inCallerTx, testPolicyID).
		Return([]MandatedAssignment{mandated("role-a", "ou-dropped")}, nil)
	suite.mockStore.On("DeleteMandatedAssignments", inCallerTx, testPolicyID, MandatedFilter{OUID: "ou-dropped"}).
		Return(nil).Once()
	suite.mockStore.On("AddMandatedAssignments", inCallerTx, testPolicyID, "role-a", testShareeOU, mandatedUser).
		Return(nil).Once()
	suite.mockStore.On("DeleteMandatedAssignments", inCallerTx, testPolicyID, MandatedFilter{}).Return(nil).Once()

	suite.Nil(suite.service.ApplyMandatedAssignments(caller, testPolicyID,
		RoleAssignment{ID: testUserID1, Type: AssigneeTypeUser}, map[string][]string{testShareeOU: {"role-a"}}))
	suite.Nil(suite.service.RemoveMandatedAssignments(caller, testPolicyID))
}

// A store failure while writing the policy's rows is a server error.
func (suite *RoleAssignmentServiceTestSuite) TestApplyMandatedAssignments_StoreError() {
	suite.withRoleStore(serverconst.StoreModeMutable)
	suite.ownedRole("role-a")
	suite.mockStore.On("ListMandatedAssignments", mock.Anything, testPolicyID).
		Return(nil, errors.New("db down"))

	err := suite.service.ApplyMandatedAssignments(context.Background(), testPolicyID,
		RoleAssignment{ID: testUserID1, Type: AssigneeTypeUser}, map[string][]string{testOwnerOU: {"role-a"}})

	suite.Require().NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *RoleAssignmentServiceTestSuite) TestApplyMandatedAssignments_InvalidAssignee() {
	suite.withRoleStore(serverconst.StoreModeMutable)

	err := suite.service.ApplyMandatedAssignments(context.Background(), testPolicyID,
		RoleAssignment{ID: testUserID1, Type: assigneeTypeEntity}, map[string][]string{testOwnerOU: {"role-a"}})
	suite.Require().NotNil(err)
	suite.Equal(ErrorInvalidAssigneeType.Code, err.Code)

	err = suite.service.ApplyMandatedAssignments(context.Background(), testPolicyID,
		RoleAssignment{Type: AssigneeTypeUser}, map[string][]string{testOwnerOU: {"role-a"}})
	suite.Require().NotNil(err)
	suite.Equal(ErrorInvalidRequestFormat.Code, err.Code)
}

// Roles that load only from resource files cannot hold a mandate, so one is refused, while a policy
// mandating nothing and a removal have nothing to do.
func (suite *RoleAssignmentServiceTestSuite) TestMandatedAssignments_DeclarativeRoleStore() {
	suite.withRoleStore(serverconst.StoreModeDeclarative)
	assignee := RoleAssignment{ID: testUserID1, Type: AssigneeTypeUser}

	err := suite.service.ApplyMandatedAssignments(context.Background(), testPolicyID, assignee,
		map[string][]string{testShareeOU: {"role-a"}})
	suite.Require().NotNil(err)
	suite.Equal(ErrorMandatedRolesNotSupported.Code, err.Code)

	suite.Nil(suite.service.ApplyMandatedAssignments(context.Background(), testPolicyID, assignee,
		map[string][]string{testShareeOU: {}}))
	suite.Nil(suite.service.RemoveMandatedAssignments(context.Background(), testPolicyID))
}

// A declared role is mandated like any other in composite mode, since its assignments live in the
// database.
func (suite *RoleAssignmentServiceTestSuite) TestApplyMandatedAssignments_CompositeRoleStore() {
	suite.withRoleStore(serverconst.StoreModeComposite)
	suite.ownedRole("declared-role")
	suite.mockSharingService.EXPECT().IsVisible(mock.Anything, RoleSharingType, "declared-role", testShareeOU).
		Return(true, nil)
	suite.mockStore.On("ListMandatedAssignments", mock.Anything, testPolicyID).Return([]MandatedAssignment{}, nil)
	suite.mockStore.On("AddMandatedAssignments", mock.Anything, testPolicyID, "declared-role", testShareeOU,
		mandatedUser).Return(nil).Once()

	suite.Nil(suite.service.ApplyMandatedAssignments(context.Background(), testPolicyID,
		RoleAssignment{ID: testUserID1, Type: AssigneeTypeUser},
		map[string][]string{testShareeOU: {"declared-role"}}))
}

func (suite *RoleAssignmentServiceTestSuite) TestRemoveMandatedAssignments() {
	suite.withRoleStore(serverconst.StoreModeMutable)
	suite.mockStore.On("DeleteMandatedAssignments", mock.Anything, testPolicyID, MandatedFilter{}).Return(nil).Once()
	suite.Nil(suite.service.RemoveMandatedAssignments(context.Background(), testPolicyID))

	suite.mockStore.On("DeleteMandatedAssignments", mock.Anything, "broken", MandatedFilter{}).
		Return(errors.New("db down")).Once()
	err := suite.service.RemoveMandatedAssignments(context.Background(), "broken")
	suite.Require().NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

// Where a role's sharing turns its assignments off, the organization unit cannot assign it directly,
// but a sharing policy can still mandate it there.
func (suite *RoleAssignmentServiceTestSuite) TestDirectAssignmentRespectsTheRuleWhileAMandateBypassesIt() {
	suite.withRoleStore(serverconst.StoreModeMutable)
	suite.ownedRole("role1")
	suite.mockSharingService.EXPECT().ResolveOverlayRules(mock.Anything, RoleSharingType, "role1", testShareeOU).
		Return(sharing.ResolvedOverlay{
			OUID: testShareeOU, Visible: true,
			Rules: map[string]sharing.OverlayRule{roleAssignmentsFieldKey: {Editable: false}},
		}, nil).Once()

	err := suite.service.AddAssignments(context.Background(), "role1", testShareeOU, []RoleAssignment{
		{ID: testUserID1, Type: AssigneeTypeUser},
	})
	suite.Require().NotNil(err)
	suite.Equal(ErrorSharedRoleFieldNotEditable.Code, err.Code)

	// The mandate asks only whether the role reaches the unit. Neither the role's rule nor where the
	// user lives is consulted: no other sharing or entity call is expected.
	suite.mockSharingService.EXPECT().IsVisible(mock.Anything, RoleSharingType, "role1", testShareeOU).
		Return(true, nil).Once()
	suite.mockStore.On("ListMandatedAssignments", mock.Anything, testPolicyID).Return([]MandatedAssignment{}, nil)
	suite.mockStore.On("AddMandatedAssignments", mock.Anything, testPolicyID, "role1", testShareeOU, mandatedUser).
		Return(nil).Once()

	suite.Nil(suite.service.ApplyMandatedAssignments(context.Background(), testPolicyID,
		RoleAssignment{ID: testUserID1, Type: AssigneeTypeUser}, map[string][]string{testShareeOU: {"role1"}}))
	suite.mockStore.AssertNotCalled(suite.T(), "AddAssignments", mock.Anything, mock.Anything, mock.Anything)
}

// removingInChild stubs a removal of testUserID1 from role1 in testShareeOU, where the user holds the
// given rows.
func (suite *RoleAssignmentServiceTestSuite) removingInChild(rows ...RoleAssignment) {
	suite.sharedWithChild(true)
	suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything, []string{testUserID1}).Return(
		[]providers.Entity{{ID: testUserID1, Category: providers.EntityCategoryUser, OUID: testOwnerOU}}, nil)
	var mandatedRows []RoleAssignment
	direct := []RoleAssignment{}
	for _, row := range rows {
		if row.PolicyID != "" {
			mandatedRows = append(mandatedRows, row)
		} else {
			direct = append(direct, row)
		}
	}
	suite.mockStore.On("ListMandatedAssignmentsForRole", mock.Anything, "role1", testShareeOU).
		Return(mandatedRows, nil)
	suite.mockStore.On("GetDirectAssignments", mock.Anything, "role1", testShareeOU, []string{testUserID1}).
		Return(direct, nil).Once()
}

// The organization unit cannot take away an assignment a sharing policy mandates.
func (suite *RoleAssignmentServiceTestSuite) TestRemoveAssignments_RefusesAMandatedOnlyAssignment() {
	suite.removingInChild(RoleAssignment{
		ID: testUserID1, Type: assigneeTypeEntity, OUID: testShareeOU, PolicyID: testPolicyID})

	err := suite.service.RemoveAssignments(context.Background(), "role1", testShareeOU, []RoleAssignment{
		{ID: testUserID1, Type: AssigneeTypeUser},
	})

	suite.Require().NotNil(err)
	suite.Equal(ErrorAssignmentMandated.Code, err.Code)
	suite.mockStore.AssertNotCalled(suite.T(), "RemoveAssignments", mock.Anything, mock.Anything, mock.Anything)
}

// A direct assignment beside a mandated one is still the organization unit's to remove.
func (suite *RoleAssignmentServiceTestSuite) TestRemoveAssignments_RemovesTheDirectRowBesideAMandatedOne() {
	suite.removingInChild(
		RoleAssignment{ID: testUserID1, Type: assigneeTypeEntity, OUID: testShareeOU, PolicyID: testPolicyID},
		RoleAssignment{ID: testUserID1, Type: assigneeTypeEntity, OUID: testShareeOU},
	)
	suite.mockStore.On("RemoveAssignments", mock.Anything, "role1", []RoleAssignment{
		{ID: testUserID1, Type: assigneeTypeEntity, OUID: testShareeOU},
	}).Return(nil).Once()

	suite.Nil(suite.service.RemoveAssignments(context.Background(), "role1", testShareeOU, []RoleAssignment{
		{ID: testUserID1, Type: AssigneeTypeUser},
	}))
}

// Assignments listed through the service carry the policy that mandates them.
func (suite *RoleAssignmentServiceTestSuite) TestGetRoleAssignments_CarriesTheMandatingPolicy() {
	suite.mockStore.On("IsRoleExist", mock.Anything, "role1").Return(true, nil)
	suite.mockStore.On("GetRoleAssignmentsCount", mock.Anything, "role1", testShareeOU).Return(2, nil)
	suite.mockStore.On("GetRoleAssignments", mock.Anything, "role1", testShareeOU, 10, 0).Return([]RoleAssignment{
		{ID: testUserID1, Type: assigneeTypeEntity, OUID: testShareeOU},
		{ID: testUserID1, Type: assigneeTypeEntity, OUID: testShareeOU, PolicyID: testPolicyID},
	}, nil)
	suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything, []string{testUserID1, testUserID1}).Return(
		[]providers.Entity{{ID: testUserID1, Category: providers.EntityCategoryUser}}, nil)

	list, err := suite.service.GetRoleAssignments(context.Background(), "role1", testShareeOU, 10, 0, false)

	suite.Require().Nil(err)
	suite.Equal([]string{"", testPolicyID}, []string{list.Assignments[0].PolicyID, list.Assignments[1].PolicyID})
}

func TestResolveAppDisplay_AppUsesSystemAttributesName(t *testing.T) {
	app := providers.Entity{
		ID:               "app-1",
		Category:         providers.EntityCategoryApp,
		Attributes:       json.RawMessage(`{"name":"ignored"}`),
		SystemAttributes: json.RawMessage(`{"name":"My App"}`),
	}
	require.Equal(t, "My App", resolveAppDisplay(app))
}

// RoleAssignmentRemovalValidationTestSuite tests ValidateRemoveAssignment.
type RoleAssignmentRemovalValidationTestSuite struct {
	suite.Suite
	store     *roleStoreInterfaceMock
	entities  *entitymock.EntityServiceInterfaceMock
	groups    *groupmock.GroupServiceInterfaceMock
	resources *resourcemock.ResourceServiceInterfaceMock
	authz     *sysauthzmock.SystemAuthorizationServiceInterfaceMock
	sharing   *sharingmock.SharingServiceInterfaceMock
	service   RoleAssignmentServiceInterface
}

func TestRoleAssignmentRemovalValidationTestSuite(t *testing.T) {
	suite.Run(t, new(RoleAssignmentRemovalValidationTestSuite))
}

func (s *RoleAssignmentRemovalValidationTestSuite) SetupTest() {
	s.store = newRoleStoreInterfaceMock(s.T())
	s.entities = entitymock.NewEntityServiceInterfaceMock(s.T())
	s.groups = groupmock.NewGroupServiceInterfaceMock(s.T())
	s.resources = resourcemock.NewResourceServiceInterfaceMock(s.T())
	s.authz = sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
	s.sharing = sharingmock.NewSharingServiceInterfaceMock(s.T())
	s.service = newRoleAssignmentService(
		s.store, s.entities, s.groups, nil, s.resources, &fakeTransactioner{}, s.authz, s.sharing, discardingBus{})
}

func (s *RoleAssignmentRemovalValidationTestSuite) mayChangeAssignments() {
	s.authz.On("CanGrantMembership", mock.Anything, sysauthz.PrincipalTypeRole, targetRoleID).Return(nil)
}

func (s *RoleAssignmentRemovalValidationTestSuite) roleGranting(permissions []ResourcePermissions) {
	s.store.On("GetRole", mock.Anything, targetRoleID).
		Return(RoleWithPermissions{ID: targetRoleID, OUID: testOwnerOU, Permissions: permissions}, nil)
}

// assigned stubs the role's assignments in any organization unit, each entity assignee of the given category.
func (s *RoleAssignmentRemovalValidationTestSuite) assigned(
	category providers.EntityCategory, assignments ...RoleAssignment) {
	s.directlyAssigned(mock.Anything, assignments)
	for _, assignment := range assignments {
		if assignment.Type == assigneeTypeEntity {
			s.entities.On("GetEntitiesByIDs", mock.Anything, []string{assignment.ID}).
				Return([]providers.Entity{{ID: assignment.ID, Category: category}}, nil).Maybe()
		}
	}
	s.store.On("ListMandatedAssignmentsForRole", mock.Anything, targetRoleID, mock.Anything).
		Return(mandatedRowsOf(assignments), nil).Maybe()
}

// directlyAssigned stubs the store's read of the named assignees' direct assignments in ouID.
func (s *RoleAssignmentRemovalValidationTestSuite) directlyAssigned(ouID any, assignments []RoleAssignment) {
	s.store.EXPECT().GetDirectAssignments(mock.Anything, targetRoleID, ouID, mock.Anything).RunAndReturn(
		func(_ context.Context, _, _ string, assigneeIDs []string) ([]RoleAssignment, error) {
			direct := []RoleAssignment{}
			for _, assignment := range assignments {
				if assignment.PolicyID == "" && slices.Contains(assigneeIDs, assignment.ID) {
					direct = append(direct, assignment)
				}
			}
			return direct, nil
		}).Maybe()
}

// mandatedRowsOf keeps the rows a sharing policy mandates.
func mandatedRowsOf(assignments []RoleAssignment) []RoleAssignment {
	rows := []RoleAssignment{}
	for _, assignment := range assignments {
		if assignment.PolicyID != "" {
			rows = append(rows, assignment)
		}
	}
	return rows
}

func (s *RoleAssignmentRemovalValidationTestSuite) TestPairsEachPermissionWithItsAudience() {
	s.mayChangeAssignments()
	s.roleGranting([]ResourcePermissions{
		{ResourceServerID: californiaRSID, Permissions: []string{"license"}},
		{ResourceServerID: ohioRSID, Permissions: []string{"license"}},
	})
	s.assigned(providers.EntityCategoryUser, RoleAssignment{ID: targetUserID, Type: assigneeTypeEntity})
	stubResourceServer(s.resources, californiaRSID, californiaAud)
	stubResourceServer(s.resources, ohioRSID, ohioAud)

	target, svcErr := s.service.ValidateRemoveAssignment(context.Background(), targetRoleID, "", targetUserID)

	s.Require().Nil(svcErr)
	s.Equal([]string{targetUserID}, target.EntityIDs)
	s.ElementsMatch([]revocation.AudienceScope{
		{Audience: californiaAud, Scope: "license"},
		{Audience: ohioAud, Scope: "license"},
	}, target.Scopes)
	s.Equal(string(AssigneeTypeUser), target.AssigneeType, "the public type, never the storage type")
}

func (s *RoleAssignmentRemovalValidationTestSuite) TestExpandsAGroupAssignee() {
	s.mayChangeAssignments()
	s.roleGranting([]ResourcePermissions{{ResourceServerID: californiaRSID, Permissions: []string{"license"}}})
	s.assigned("", RoleAssignment{ID: targetGroupID, Type: AssigneeTypeGroup})
	s.groups.On("ValidateGroupIDs", mock.Anything, []string{targetGroupID}).Return(nil)
	stubResourceServer(s.resources, californiaRSID, californiaAud)
	stubGroupEntities(s.groups, targetGroupID, "member-1", "member-2")

	target, svcErr := s.service.ValidateRemoveAssignment(context.Background(), targetRoleID, "", targetGroupID)

	s.Require().Nil(svcErr)
	s.ElementsMatch([]string{"member-1", "member-2"}, target.EntityIDs)
	s.Equal(string(AssigneeTypeGroup), target.AssigneeType)
}

func (s *RoleAssignmentRemovalValidationTestSuite) TestRoleWithoutPermissionsYieldsNoScopes() {
	s.mayChangeAssignments()
	s.roleGranting(nil)
	s.assigned(providers.EntityCategoryApp, RoleAssignment{ID: targetUserID, Type: assigneeTypeEntity})

	target, svcErr := s.service.ValidateRemoveAssignment(context.Background(), targetRoleID, "", targetUserID)

	s.Require().Nil(svcErr)
	s.Empty(target.Scopes)
	s.Equal(string(AssigneeTypeApp), target.AssigneeType)
}

func (s *RoleAssignmentRemovalValidationTestSuite) TestRefusesAnAssigneeThatDoesNotHoldTheRole() {
	s.mayChangeAssignments()
	s.roleGranting(nil)
	s.assigned(providers.EntityCategoryUser, RoleAssignment{ID: "someone-else", Type: assigneeTypeEntity})

	_, svcErr := s.service.ValidateRemoveAssignment(context.Background(), targetRoleID, "", targetUserID)

	s.Require().NotNil(svcErr)
	s.Equal(ErrorRoleAssignmentNotFound.Code, svcErr.Code)
}

// sharedWithSharee stubs the role as shared with testShareeOU, with its assignments open or turned
// off there.
func (s *RoleAssignmentRemovalValidationTestSuite) sharedWithSharee(assignmentsOpen bool) {
	s.sharing.EXPECT().ResolveOverlayRules(mock.Anything, RoleSharingType, targetRoleID, testShareeOU).
		Return(sharing.ResolvedOverlay{
			OUID: testShareeOU, Visible: true,
			Rules: map[string]sharing.OverlayRule{roleAssignmentsFieldKey: {Editable: assignmentsOpen}},
		}, nil)
}

// assignedIn stubs the assignments made in one organization unit, and only there.
func (s *RoleAssignmentRemovalValidationTestSuite) assignedIn(ouID string, assignments ...RoleAssignment) {
	s.directlyAssigned(ouID, assignments)
	s.store.On("ListMandatedAssignmentsForRole", mock.Anything, targetRoleID, ouID).
		Return(mandatedRowsOf(assignments), nil).Maybe()
}

// An assignment made in an organization unit the role is shared with is found there, not in the
// role's owner.
func (s *RoleAssignmentRemovalValidationTestSuite) TestFindsTheAssignmentInTheOUItWasMadeIn() {
	for _, tc := range []struct {
		name            string
		assignmentsOpen bool
	}{
		{"assignments open", true},
		{"assignments turned off, which stops only additions", false},
	} {
		s.Run(tc.name, func() {
			s.SetupTest()
			s.mayChangeAssignments()
			s.roleGranting([]ResourcePermissions{{ResourceServerID: californiaRSID, Permissions: []string{"license"}}})
			s.sharedWithSharee(tc.assignmentsOpen)
			s.assignedIn(testShareeOU, RoleAssignment{ID: targetUserID, Type: assigneeTypeEntity})
			s.entities.On("GetEntitiesByIDs", mock.Anything, []string{targetUserID}).
				Return([]providers.Entity{{ID: targetUserID, Category: providers.EntityCategoryUser}}, nil)
			stubResourceServer(s.resources, californiaRSID, californiaAud)

			target, svcErr := s.service.ValidateRemoveAssignment(
				context.Background(), targetRoleID, testShareeOU, targetUserID)

			s.Require().Nil(svcErr)
			s.Equal([]string{targetUserID}, target.EntityIDs)
			s.Equal(string(AssigneeTypeUser), target.AssigneeType)
			s.store.AssertNotCalled(s.T(), "GetDirectAssignments",
				mock.Anything, targetRoleID, testOwnerOU, mock.Anything)
		})
	}
}

// An organization unit the role does not reach holds no assignments of it to remove.
func (s *RoleAssignmentRemovalValidationTestSuite) TestRefusesAnOUTheRoleIsNotSharedWith() {
	s.mayChangeAssignments()
	s.roleGranting(nil)
	s.sharing.EXPECT().ResolveOverlayRules(mock.Anything, RoleSharingType, targetRoleID, "stranger").
		Return(sharing.ResolvedOverlay{OUID: "stranger"}, nil)

	_, svcErr := s.service.ValidateRemoveAssignment(context.Background(), targetRoleID, "stranger", targetUserID)

	s.Require().NotNil(svcErr)
	s.Equal(ErrorRoleNotSharedToOU.Code, svcErr.Code)
	s.store.AssertNotCalled(s.T(), "GetDirectAssignments",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// The assignment is read for the named assignee alone, never by paging through the role's assignments.
func (s *RoleAssignmentRemovalValidationTestSuite) TestReadsOnlyTheNamedAssigneesAssignment() {
	s.mayChangeAssignments()
	s.roleGranting(nil)
	s.store.EXPECT().GetDirectAssignments(mock.Anything, targetRoleID, testOwnerOU, []string{targetUserID}).
		Return([]RoleAssignment{{ID: targetUserID, Type: assigneeTypeEntity, OUID: testOwnerOU}}, nil).Once()
	s.entities.On("GetEntitiesByIDs", mock.Anything, []string{targetUserID}).
		Return([]providers.Entity{{ID: targetUserID, Category: providers.EntityCategoryUser}}, nil)
	s.store.On("ListMandatedAssignmentsForRole", mock.Anything, targetRoleID, testOwnerOU).
		Return([]RoleAssignment{}, nil)

	target, svcErr := s.service.ValidateRemoveAssignment(context.Background(), targetRoleID, "", targetUserID)

	s.Require().Nil(svcErr)
	s.Equal([]string{targetUserID}, target.EntityIDs)
	s.store.AssertNotCalled(s.T(), "GetRoleAssignments",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// A failed read of the assignment is a server error.
func (s *RoleAssignmentRemovalValidationTestSuite) TestAssignmentReadFailureIsAServerError() {
	s.mayChangeAssignments()
	s.roleGranting(nil)
	s.store.EXPECT().GetDirectAssignments(mock.Anything, targetRoleID, testOwnerOU, []string{targetUserID}).
		Return(nil, errors.New("db down")).Once()

	_, svcErr := s.service.ValidateRemoveAssignment(context.Background(), targetRoleID, "", targetUserID)

	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *RoleAssignmentRemovalValidationTestSuite) TestRunsTheRemovalsAssigneeChecks() {
	s.mayChangeAssignments()
	s.roleGranting(nil)
	s.assigned("", RoleAssignment{ID: targetGroupID, Type: AssigneeTypeGroup})
	s.groups.On("ValidateGroupIDs", mock.Anything, []string{targetGroupID}).
		Return(&group.ErrorInvalidGroupMemberID)

	_, svcErr := s.service.ValidateRemoveAssignment(context.Background(), targetRoleID, "", targetGroupID)

	s.Require().NotNil(svcErr)
	s.Equal(ErrorInvalidAssignmentID.Code, svcErr.Code)
}

func (s *RoleAssignmentRemovalValidationTestSuite) TestRefusesACallerWhoMayNotChangeAssignments() {
	s.authz.On("CanGrantMembership", mock.Anything, sysauthz.PrincipalTypeRole, targetRoleID).
		Return(&tidcommon.ErrorUnauthorized)

	_, svcErr := s.service.ValidateRemoveAssignment(context.Background(), targetRoleID, "", targetUserID)

	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
	s.store.AssertNotCalled(s.T(), "GetRole", mock.Anything, mock.Anything)
}

func (s *RoleAssignmentRemovalValidationTestSuite) TestRequiresBothIdentifiers() {
	s.authz.On("CanGrantMembership", mock.Anything, sysauthz.PrincipalTypeRole, "").Return(nil)

	_, svcErr := s.service.ValidateRemoveAssignment(context.Background(), "", "", targetUserID)
	s.Require().NotNil(svcErr)
	s.Equal(ErrorMissingRoleID.Code, svcErr.Code)

	_, svcErr = s.service.ValidateRemoveAssignment(context.Background(), targetRoleID, "", "")
	s.Require().NotNil(svcErr)
	s.Equal(ErrorMissingAssigneeID.Code, svcErr.Code)
}

func (s *RoleAssignmentRemovalValidationTestSuite) TestRoleNotFound() {
	s.mayChangeAssignments()
	s.store.On("GetRole", mock.Anything, targetRoleID).Return(RoleWithPermissions{}, ErrRoleNotFound)

	_, svcErr := s.service.ValidateRemoveAssignment(context.Background(), targetRoleID, "", targetUserID)

	s.Require().NotNil(svcErr)
	s.Equal(ErrorRoleNotFound.Code, svcErr.Code)
}

func (s *RoleAssignmentRemovalValidationTestSuite) TestRoleReadFailureIsAServerError() {
	s.mayChangeAssignments()
	s.store.On("GetRole", mock.Anything, targetRoleID).Return(RoleWithPermissions{}, errors.New("db down"))

	_, svcErr := s.service.ValidateRemoveAssignment(context.Background(), targetRoleID, "", targetUserID)

	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *RoleAssignmentRemovalValidationTestSuite) TestDeclarativeRoleListingAPermissionTwiceIsNotRefused() {
	declared, err := parseToRole([]byte(`
id: role-1
name: DMV Clerk
ouId: ou-1
permissions:
  - resourceServerId: rs-ca
    permissions:
      - license
      - license
  - resourceServerId: rs-ca
    permissions:
      - license
assignments:
  - id: user-1
    type: user
`))
	s.Require().NoError(err)
	store := &fileBasedStore{
		GenericFileBasedStore: declarativeresource.NewGenericFileBasedStoreForTest(entity.KeyTypeRole),
	}
	s.Require().NoError(store.Create(declared.ID, declared))
	service := newRoleAssignmentService(
		store, s.entities, s.groups, nil, s.resources, &fakeTransactioner{}, s.authz, nil, discardingBus{})
	s.mayChangeAssignments()
	s.entities.On("GetEntitiesByIDs", mock.Anything, []string{targetUserID}).
		Return([]providers.Entity{{ID: targetUserID, Category: providers.EntityCategoryUser}}, nil)
	stubResourceServer(s.resources, californiaRSID, californiaAud)

	target, svcErr := service.ValidateRemoveAssignment(context.Background(), targetRoleID, "", targetUserID)

	s.Require().Nil(svcErr)
	s.Require().NotEmpty(target.Scopes)
	for _, scope := range target.Scopes {
		s.Equal(revocation.AudienceScope{Audience: californiaAud, Scope: "license"}, scope)
	}
	s.Equal([]string{targetUserID}, target.EntityIDs)
}

// An assignee whose only assignment there is one a sharing policy mandates cannot be unassigned, so
// nothing is revoked.
func (s *RoleAssignmentRemovalValidationTestSuite) TestRefusesAMandatedOnlyAssignment() {
	s.mayChangeAssignments()
	s.roleGranting(nil)
	s.assigned(providers.EntityCategoryUser,
		RoleAssignment{ID: targetUserID, Type: assigneeTypeEntity, PolicyID: testPolicyID})

	_, svcErr := s.service.ValidateRemoveAssignment(context.Background(), targetRoleID, "", targetUserID)

	s.Require().NotNil(svcErr)
	s.Equal(ErrorAssignmentMandated.Code, svcErr.Code)
}

// A direct assignment beside a mandated one may still be removed, and since the mandate keeps the role,
// removing it revokes nothing.
func (s *RoleAssignmentRemovalValidationTestSuite) TestADirectAssignmentKeptByAMandateRevokesNothing() {
	s.mayChangeAssignments()
	s.roleGranting([]ResourcePermissions{{ResourceServerID: californiaRSID, Permissions: []string{"license"}}})
	s.assigned(providers.EntityCategoryUser,
		RoleAssignment{ID: targetUserID, Type: assigneeTypeEntity, PolicyID: testPolicyID},
		RoleAssignment{ID: targetUserID, Type: assigneeTypeEntity})
	s.entities.On("GetEntitiesByIDs", mock.Anything, []string{targetUserID}).
		Return([]providers.Entity{{ID: targetUserID, Category: providers.EntityCategoryUser}}, nil)

	target, svcErr := s.service.ValidateRemoveAssignment(context.Background(), targetRoleID, "", targetUserID)

	s.Require().Nil(svcErr)
	s.Equal(&revocation.AccessRevocationTarget{AssigneeType: string(AssigneeTypeUser)}, target,
		"no entity loses a scope, and the removal still knows the assignee's type")
	s.resources.AssertNotCalled(s.T(), "GetResourceServer", mock.Anything, mock.Anything)
}

// Applying the same desired twice writes the rows once: the second apply finds them already held.
func (suite *RoleAssignmentServiceTestSuite) TestApplyMandatedAssignments_SameDesiredTwiceWritesOnce() {
	suite.withRoleStore(serverconst.StoreModeMutable)
	suite.ownedRole("role-a")
	suite.mockSharingService.EXPECT().IsVisible(mock.Anything, RoleSharingType, "role-a", testShareeOU).
		Return(true, nil)
	suite.mockStore.On("ListMandatedAssignments", mock.Anything, testPolicyID).
		Return([]MandatedAssignment{}, nil).Once()
	suite.mockStore.On("AddMandatedAssignments", mock.Anything, testPolicyID, "role-a", testShareeOU, mandatedUser).
		Return(nil).Once()
	suite.mockStore.On("ListMandatedAssignments", mock.Anything, testPolicyID).
		Return([]MandatedAssignment{mandated("role-a", testShareeOU)}, nil).Once()
	desired := map[string][]string{testShareeOU: {"role-a"}}
	assignee := RoleAssignment{ID: testUserID1, Type: AssigneeTypeUser}

	suite.Nil(suite.service.ApplyMandatedAssignments(context.Background(), testPolicyID, assignee, desired))
	suite.Nil(suite.service.ApplyMandatedAssignments(context.Background(), testPolicyID, assignee, desired))

	suite.mockStore.AssertNumberOfCalls(suite.T(), "AddMandatedAssignments", 1)
	suite.mockStore.AssertNotCalled(suite.T(), "DeleteMandatedAssignments", mock.Anything, mock.Anything, mock.Anything)
}

// Reconciling removes the roles whose policy is gone, whose unit is gone, whose role is gone or no longer reaches the
// unit, and keeps the rest. Each policy and each role in a unit is checked once.
func (suite *RoleAssignmentServiceTestSuite) TestReconcileMandatedAssignments_RemovesWhatNoLongerHolds() {
	suite.withRoleStore(serverconst.StoreModeMutable)
	suite.mockStore.On("ListMandatedRoleUnits", mock.Anything).Return([]MandatedRoleUnit{
		{PolicyID: "policy-gone", RoleID: "role-a", OUID: "ou-1"},
		{PolicyID: "policy-gone", RoleID: "role-b", OUID: "ou-2"},
		{PolicyID: testPolicyID, RoleID: "role-a", OUID: testOwnerOU},
		{PolicyID: testPolicyID, RoleID: "role-a", OUID: "ou-unshared"},
		{PolicyID: testPolicyID, RoleID: "role-a", OUID: "ou-deleted"},
		{PolicyID: testPolicyID, RoleID: "role-a", OUID: testShareeOU},
		{PolicyID: "policy-2", RoleID: "role-a", OUID: testShareeOU},
		{PolicyID: testPolicyID, RoleID: "role-gone", OUID: testShareeOU},
	}, nil).Once()
	suite.mockSharingService.EXPECT().GetPolicy(mock.Anything, "policy-gone").
		Return(sharing.Policy{}, &sharing.ErrorPolicyNotFound).Once()
	suite.mockSharingService.EXPECT().GetPolicy(mock.Anything, testPolicyID).
		Return(sharing.Policy{ID: testPolicyID}, nil).Once()
	suite.mockSharingService.EXPECT().GetPolicy(mock.Anything, "policy-2").
		Return(sharing.Policy{ID: "policy-2"}, nil).Once()
	suite.ownedRole("role-a")
	suite.mockStore.On("GetRole", mock.Anything, "role-gone").Return(RoleWithPermissions{}, ErrRoleNotFound)
	suite.mockSharingService.EXPECT().IsVisible(mock.Anything, RoleSharingType, "role-a", "ou-unshared").
		Return(false, nil).Once()
	suite.mockSharingService.EXPECT().IsVisible(mock.Anything, RoleSharingType, "role-a", "ou-deleted").
		Return(false, &oupkg.ErrorOrganizationUnitNotFound).Once()
	suite.mockSharingService.EXPECT().IsVisible(mock.Anything, RoleSharingType, "role-a", testShareeOU).
		Return(true, nil).Once()
	for _, removed := range []MandatedRoleUnit{
		{PolicyID: "policy-gone", RoleID: "role-a", OUID: "ou-1"},
		{PolicyID: "policy-gone", RoleID: "role-b", OUID: "ou-2"},
		{PolicyID: testPolicyID, RoleID: "role-a", OUID: "ou-unshared"},
		{PolicyID: testPolicyID, RoleID: "role-a", OUID: "ou-deleted"},
		{PolicyID: testPolicyID, RoleID: "role-gone", OUID: testShareeOU},
	} {
		suite.mockStore.On("DeleteMandatedAssignments", mock.Anything, removed.PolicyID,
			MandatedFilter{OUID: removed.OUID, RoleID: removed.RoleID}).Return(nil).Once()
	}

	suite.NoError(suite.service.(*roleAssignmentService).reconcileMandatedAssignments(context.Background()))

	suite.mockStore.AssertNumberOfCalls(suite.T(), "DeleteMandatedAssignments", 5)
}

// A failure to tell whether a mandate still holds stops the reconciliation rather than removing it.
func (suite *RoleAssignmentServiceTestSuite) TestReconcileMandatedAssignments_Failures() {
	unit := MandatedRoleUnit{PolicyID: testPolicyID, RoleID: "role-a", OUID: testShareeOU}
	cases := []struct {
		name string
		stub func()
	}{
		{"listing fails", func() {
			suite.mockStore.On("ListMandatedRoleUnits", mock.Anything).Return(nil, errors.New("db down"))
		}},
		{"policy read fails", func() {
			suite.mockStore.On("ListMandatedRoleUnits", mock.Anything).Return([]MandatedRoleUnit{unit}, nil)
			suite.mockSharingService.EXPECT().GetPolicy(mock.Anything, testPolicyID).
				Return(sharing.Policy{}, &tidcommon.InternalServerError)
		}},
		{"role check fails", func() {
			suite.mockStore.On("ListMandatedRoleUnits", mock.Anything).Return([]MandatedRoleUnit{unit}, nil)
			suite.mockSharingService.EXPECT().GetPolicy(mock.Anything, testPolicyID).
				Return(sharing.Policy{ID: testPolicyID}, nil)
			suite.ownedRole("role-a")
			suite.mockSharingService.EXPECT().IsVisible(mock.Anything, RoleSharingType, "role-a", testShareeOU).
				Return(false, &tidcommon.InternalServerError)
		}},
		{"removal fails", func() {
			suite.mockStore.On("ListMandatedRoleUnits", mock.Anything).Return([]MandatedRoleUnit{unit}, nil)
			suite.mockSharingService.EXPECT().GetPolicy(mock.Anything, testPolicyID).
				Return(sharing.Policy{}, &sharing.ErrorPolicyNotFound)
			suite.mockStore.On("DeleteMandatedAssignments", mock.Anything, testPolicyID,
				MandatedFilter{OUID: testShareeOU, RoleID: "role-a"}).Return(errors.New("db down"))
		}},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			suite.withRoleStore(serverconst.StoreModeMutable)
			tc.stub()

			suite.Error(suite.service.(*roleAssignmentService).reconcileMandatedAssignments(context.Background()))
		})
	}
}

// Roles that load only from resource files hold no mandated assignments, so there is nothing to read.
func (suite *RoleAssignmentServiceTestSuite) TestReconcileMandatedAssignments_DeclarativeRolesHoldNone() {
	suite.withRoleStore(serverconst.StoreModeDeclarative)

	suite.NoError(suite.service.(*roleAssignmentService).reconcileMandatedAssignments(context.Background()))

	suite.mockStore.AssertNotCalled(suite.T(), "ListMandatedRoleUnits", mock.Anything)
}

// assignableIn stubs testShareeOU owning owned-1 and owned-ungrantable, with shared-open and shared-closed
// shared with it, only shared-open with its assignments editable there.
func (suite *RoleAssignmentServiceTestSuite) assignableIn() *sysauthzmock.SystemAuthorizationServiceInterfaceMock {
	authz := suite.withGrantChecks()
	suite.mockStore.On("GetRoleListCountByOUID", mock.Anything, testShareeOU).Return(2, nil)
	suite.mockStore.On("GetRoleListByOUID", mock.Anything, testShareeOU, 2, 0).Return([]Role{
		{ID: "owned-1", OUID: testShareeOU}, {ID: "owned-ungrantable", OUID: testShareeOU},
	}, nil)
	suite.mockSharingService.EXPECT().ListVisibleResourceIDs(mock.Anything, RoleSharingType, testShareeOU).
		Return([]string{"shared-closed", "shared-open"}, nil)
	for id, editable := range map[string]bool{"shared-open": true, "shared-closed": false} {
		suite.mockSharingService.EXPECT().ResolveOverlayRules(mock.Anything, RoleSharingType, id, testShareeOU).
			Return(sharing.ResolvedOverlay{
				OUID: testShareeOU, Visible: true,
				Rules: map[string]sharing.OverlayRule{roleAssignmentsFieldKey: {Editable: editable}},
			}, nil)
	}
	suite.mockStore.On("GetRolesByIDs", mock.Anything, []string{"shared-open"}).
		Return([]Role{{ID: "shared-open", OUID: testOwnerOU}}, nil)
	return authz
}

// A unit is offered the roles it owns and the shared ones whose assignments it may change, as far as the
// caller may grant them.
func (suite *RoleAssignmentServiceTestSuite) TestListAssignableRoles() {
	authz := suite.assignableIn()
	authz.On("GrantableMemberships", mock.Anything, sysauthz.PrincipalTypeRole,
		[]string{"owned-1", "owned-ungrantable", "shared-open"}).
		Return([]string{"owned-1", "shared-open"}, (*tidcommon.ServiceError)(nil)).Once()

	roles, svcErr := suite.service.ListAssignableRoles(context.Background(), testShareeOU)

	suite.Nil(svcErr)
	ids := make([]string, 0, len(roles))
	for _, r := range roles {
		ids = append(ids, r.ID)
	}
	suite.Equal([]string{"owned-1", "shared-open"}, ids)
}

func (suite *RoleAssignmentServiceTestSuite) TestListAssignableRoles_GrantCheckServerError() {
	authz := suite.assignableIn()
	authz.On("GrantableMemberships", mock.Anything, sysauthz.PrincipalTypeRole, mock.Anything).
		Return([]string(nil), &tidcommon.InternalServerError)

	roles, svcErr := suite.service.ListAssignableRoles(context.Background(), testShareeOU)

	suite.Nil(roles)
	suite.Require().NotNil(svcErr)
	suite.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

// A unit's roles are read once: a failed count, a failed page, and composite mode's cap each end the listing.
func (suite *RoleAssignmentServiceTestSuite) TestListAssignableRoles_StoreErrors() {
	cases := map[string]struct {
		countErr, listErr error
		want              string
	}{
		"count":     {countErr: errors.New("store down"), want: tidcommon.InternalServerError.Code},
		"page":      {listErr: errors.New("store down"), want: tidcommon.InternalServerError.Code},
		"composite": {countErr: errResultLimitExceededInCompositeMode, want: ResultLimitExceededInCompositeMode.Code},
	}
	for name, tc := range cases {
		suite.Run(name, func() {
			suite.SetupTest()
			suite.withGrantChecks()
			suite.mockStore.On("GetRoleListCountByOUID", mock.Anything, testShareeOU).Return(1, tc.countErr)
			suite.mockStore.On("GetRoleListByOUID", mock.Anything, testShareeOU, 1, 0).
				Return([]Role(nil), tc.listErr).Maybe()

			roles, svcErr := suite.service.ListAssignableRoles(context.Background(), testShareeOU)

			suite.Nil(roles)
			suite.Require().NotNil(svcErr)
			suite.Equal(tc.want, svcErr.Code)
		})
	}
}

// A unit that does not exist has nothing shared with it, so only what it owns is offered.
func (suite *RoleAssignmentServiceTestSuite) TestListAssignableRoles_UnknownUnitHasNothingShared() {
	authz := suite.withGrantChecks()
	suite.mockStore.On("GetRoleListCountByOUID", mock.Anything, testShareeOU).Return(1, nil)
	suite.mockStore.On("GetRoleListByOUID", mock.Anything, testShareeOU, 1, 0).
		Return([]Role{{ID: "owned-1", OUID: testShareeOU}}, nil)
	suite.mockSharingService.EXPECT().ListVisibleResourceIDs(mock.Anything, RoleSharingType, testShareeOU).
		Return(nil, &oupkg.ErrorOrganizationUnitNotFound)
	authz.On("GrantableMemberships", mock.Anything, sysauthz.PrincipalTypeRole, []string{"owned-1"}).
		Return([]string{"owned-1"}, (*tidcommon.ServiceError)(nil)).Once()

	roles, svcErr := suite.service.ListAssignableRoles(context.Background(), testShareeOU)

	suite.Nil(svcErr)
	suite.Equal([]Role{{ID: "owned-1", OUID: testShareeOU}}, roles)
}

func (suite *RoleAssignmentServiceTestSuite) TestListAssignableRoles_SharedRoleLookupErrors() {
	suite.Run("visible roles", func() {
		suite.SetupTest()
		suite.withGrantChecks()
		suite.mockStore.On("GetRoleListCountByOUID", mock.Anything, testShareeOU).Return(0, nil)
		suite.mockSharingService.EXPECT().ListVisibleResourceIDs(mock.Anything, RoleSharingType, testShareeOU).
			Return(nil, &tidcommon.InternalServerError)

		roles, svcErr := suite.service.ListAssignableRoles(context.Background(), testShareeOU)

		suite.Nil(roles)
		suite.Require().NotNil(svcErr)
		suite.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	})
	suite.Run("overlay rules", func() {
		suite.SetupTest()
		suite.withGrantChecks()
		suite.mockStore.On("GetRoleListCountByOUID", mock.Anything, testShareeOU).Return(0, nil)
		suite.mockSharingService.EXPECT().ListVisibleResourceIDs(mock.Anything, RoleSharingType, testShareeOU).
			Return([]string{"shared-open"}, nil)
		suite.mockSharingService.EXPECT().ResolveOverlayRules(mock.Anything, RoleSharingType, "shared-open",
			testShareeOU).Return(sharing.ResolvedOverlay{}, &tidcommon.InternalServerError)

		roles, svcErr := suite.service.ListAssignableRoles(context.Background(), testShareeOU)

		suite.Nil(roles)
		suite.Require().NotNil(svcErr)
		suite.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	})
	suite.Run("shared roles", func() {
		suite.SetupTest()
		suite.withGrantChecks()
		suite.mockStore.On("GetRoleListCountByOUID", mock.Anything, testShareeOU).Return(0, nil)
		suite.mockSharingService.EXPECT().ListVisibleResourceIDs(mock.Anything, RoleSharingType, testShareeOU).
			Return([]string{"shared-open"}, nil)
		suite.mockSharingService.EXPECT().ResolveOverlayRules(mock.Anything, RoleSharingType, "shared-open",
			testShareeOU).Return(sharing.ResolvedOverlay{OUID: testShareeOU, Visible: true,
			Rules: map[string]sharing.OverlayRule{roleAssignmentsFieldKey: {Editable: true}}}, nil)
		suite.mockStore.On("GetRolesByIDs", mock.Anything, []string{"shared-open"}).
			Return([]Role(nil), errors.New("store down"))

		roles, svcErr := suite.service.ListAssignableRoles(context.Background(), testShareeOU)

		suite.Nil(roles)
		suite.Require().NotNil(svcErr)
		suite.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	})
}
