// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"context"
	"encoding/json"
	"errors"
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
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
	"github.com/thunder-id/thunderid/tests/mocks/groupmock"
	"github.com/thunder-id/thunderid/tests/mocks/resourcemock"
	"github.com/thunder-id/thunderid/tests/mocks/sharingmock"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
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

func (suite *RoleAssignmentServiceTestSuite) SetupTest() {
	suite.mockStore = newRoleStoreInterfaceMock(suite.T())
	suite.mockEntityService = entitymock.NewEntityServiceInterfaceMock(suite.T())
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

func (suite *RoleAssignmentServiceTestSuite) TestCascadeDeleteDependencies_Group_DeletesAssignments() {
	suite.mockStore.On("DeleteAssignmentsByAssignee", mock.Anything, string(AssigneeTypeGroup), "group-1").
		Return(int64(1), nil)

	deleted, err := suite.service.CascadeDeleteDependencies(
		context.Background(), resourcedependency.ResourceTypeGroup, "group-1")

	suite.NoError(err)
	suite.Equal(1, deleted)
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
		s.store, s.entities, s.groups, nil, s.resources, &fakeTransactioner{}, s.authz, s.sharing)
}

func (s *RoleAssignmentRemovalValidationTestSuite) mayChangeAssignments() {
	s.authz.On("CanGrantMembership", mock.Anything, sysauthz.PrincipalTypeRole, targetRoleID).Return(nil)
}

func (s *RoleAssignmentRemovalValidationTestSuite) roleGranting(permissions []ResourcePermissions) {
	s.store.On("GetRole", mock.Anything, targetRoleID).
		Return(RoleWithPermissions{ID: targetRoleID, OUID: testOwnerOU, Permissions: permissions}, nil)
}

func (s *RoleAssignmentRemovalValidationTestSuite) assignedOnPage(total, offset int,
	category providers.EntityCategory, assignments ...RoleAssignment) {
	s.store.On("GetRoleAssignments", mock.Anything, targetRoleID, mock.Anything, serverconst.MaxPageSize, offset).
		Return(assignments, nil)
	entities := []providers.Entity{}
	entityIDs := []string{}
	for _, assignment := range assignments {
		if assignment.Type == assigneeTypeEntity {
			entityIDs = append(entityIDs, assignment.ID)
			entities = append(entities, providers.Entity{ID: assignment.ID, Category: category})
		}
	}
	if len(entityIDs) > 0 {
		s.entities.On("GetEntitiesByIDs", mock.Anything, entityIDs).Return(entities, nil)
	}
	s.store.On("IsRoleExist", mock.Anything, targetRoleID).Return(true, nil)
	s.store.On("GetRoleAssignmentsCount", mock.Anything, targetRoleID, mock.Anything).Return(total, nil)
}

func (s *RoleAssignmentRemovalValidationTestSuite) TestPairsEachPermissionWithItsAudience() {
	s.mayChangeAssignments()
	s.roleGranting([]ResourcePermissions{
		{ResourceServerID: californiaRSID, Permissions: []string{"license"}},
		{ResourceServerID: ohioRSID, Permissions: []string{"license"}},
	})
	s.assignedOnPage(1, 0, providers.EntityCategoryUser, RoleAssignment{ID: targetUserID, Type: assigneeTypeEntity})
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
	s.assignedOnPage(1, 0, "", RoleAssignment{ID: targetGroupID, Type: AssigneeTypeGroup})
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
	s.assignedOnPage(1, 0, providers.EntityCategoryApp, RoleAssignment{ID: targetUserID, Type: assigneeTypeEntity})

	target, svcErr := s.service.ValidateRemoveAssignment(context.Background(), targetRoleID, "", targetUserID)

	s.Require().Nil(svcErr)
	s.Empty(target.Scopes)
	s.Equal(string(AssigneeTypeApp), target.AssigneeType)
}

func (s *RoleAssignmentRemovalValidationTestSuite) TestRefusesAnAssigneeThatDoesNotHoldTheRole() {
	s.mayChangeAssignments()
	s.roleGranting(nil)
	s.assignedOnPage(1, 0, providers.EntityCategoryUser, RoleAssignment{ID: "someone-else", Type: assigneeTypeEntity})

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
	s.store.On("GetRoleAssignments", mock.Anything, targetRoleID, ouID, serverconst.MaxPageSize, 0).
		Return(assignments, nil)
	s.store.On("GetRoleAssignmentsCount", mock.Anything, targetRoleID, ouID).Return(len(assignments), nil)
	s.store.On("IsRoleExist", mock.Anything, targetRoleID).Return(true, nil)
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
			s.store.AssertNotCalled(s.T(), "GetRoleAssignments",
				mock.Anything, targetRoleID, testOwnerOU, mock.Anything, mock.Anything)
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
	s.store.AssertNotCalled(s.T(), "GetRoleAssignments",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *RoleAssignmentRemovalValidationTestSuite) TestFindsAnAssignmentOnALaterPage() {
	s.mayChangeAssignments()
	s.roleGranting(nil)
	s.assignedOnPage(serverconst.MaxPageSize+1, 0, providers.EntityCategoryUser,
		RoleAssignment{ID: "someone-else", Type: assigneeTypeEntity})
	s.assignedOnPage(serverconst.MaxPageSize+1, serverconst.MaxPageSize, providers.EntityCategoryUser,
		RoleAssignment{ID: targetUserID, Type: assigneeTypeEntity})

	target, svcErr := s.service.ValidateRemoveAssignment(context.Background(), targetRoleID, "", targetUserID)

	s.Require().Nil(svcErr)
	s.Equal([]string{targetUserID}, target.EntityIDs)
}

func (s *RoleAssignmentRemovalValidationTestSuite) TestRunsTheRemovalsAssigneeChecks() {
	s.mayChangeAssignments()
	s.roleGranting(nil)
	s.assignedOnPage(1, 0, "", RoleAssignment{ID: targetGroupID, Type: AssigneeTypeGroup})
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
		store, s.entities, s.groups, nil, s.resources, &fakeTransactioner{}, s.authz, nil)
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
