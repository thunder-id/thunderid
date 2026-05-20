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

package role

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/group"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
	"github.com/thunder-id/thunderid/tests/mocks/groupmock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
	"github.com/thunder-id/thunderid/tests/mocks/resourcemock"
)

const (
	testUserID1 = "user1"
)

// fakeTransactioner is a light-weight test double to capture transaction usage.
type fakeTransactioner struct {
	transactCalls int
	err           error
}

func (f *fakeTransactioner) Transact(ctx context.Context, txFunc func(context.Context) error) error {
	f.transactCalls++
	if f.err != nil {
		return f.err
	}
	return txFunc(ctx)
}

// Test Suite
type RoleServiceTestSuite struct {
	suite.Suite
	mockStore             *roleStoreInterfaceMock
	mockEntityService     *entitymock.EntityServiceInterfaceMock
	mockGroupService      *groupmock.GroupServiceInterfaceMock
	mockOUService         *oumock.OrganizationUnitServiceInterfaceMock
	mockResourceService   *resourcemock.ResourceServiceInterfaceMock
	mockEntityTypeService *entitytypemock.EntityTypeServiceInterfaceMock
	transactioner         *fakeTransactioner
	service               RoleServiceInterface
}

func TestRoleServiceTestSuite(t *testing.T) {
	suite.Run(t, new(RoleServiceTestSuite))
}

func (suite *RoleServiceTestSuite) SetupTest() {
	// Initialize config runtime with default values
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	if err != nil {
		suite.Fail("Failed to initialize runtime", err)
	}

	suite.mockStore = newRoleStoreInterfaceMock(suite.T())
	suite.mockEntityService = entitymock.NewEntityServiceInterfaceMock(suite.T())
	suite.mockGroupService = groupmock.NewGroupServiceInterfaceMock(suite.T())
	suite.mockOUService = oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())
	suite.mockResourceService = resourcemock.NewResourceServiceInterfaceMock(suite.T())
	suite.mockEntityTypeService = entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T())
	suite.transactioner = &fakeTransactioner{}
	suite.service = newRoleService(
		suite.mockStore,
		suite.mockEntityService,
		suite.mockGroupService,
		suite.mockOUService,
		suite.mockResourceService,
		suite.transactioner,
	)
}

// TearDownTest cleans up after each test
func (suite *RoleServiceTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

// GetRoleList Tests
func (suite *RoleServiceTestSuite) TestGetRoleList_Success() {
	expectedRoles := []Role{
		{ID: "role1", Name: "Admin", OUID: "ou1"},
		{ID: "role2", Name: "User", OUID: "ou1"},
	}

	suite.mockStore.On("GetRoleListCount", mock.Anything).Return(2, nil)
	suite.mockStore.On("GetRoleList", mock.Anything, 10, 0).Return(expectedRoles, nil)
	suite.mockOUService.On("GetOrganizationUnitHandlesByIDs", mock.Anything,
		[]string{"ou1"}).Return(map[string]string{"ou1": "default"}, nil)

	result, err := suite.service.GetRoleList(context.Background(), 10, 0)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(2, result.TotalResults)
	suite.Equal(2, result.Count)
	suite.Equal(1, result.StartIndex)
	suite.Equal(2, len(result.Roles))
	suite.Equal("role1", result.Roles[0].ID)
	suite.Equal("Admin", result.Roles[0].Name)
	suite.Equal("default", result.Roles[0].OUHandle)
	suite.Equal("role2", result.Roles[1].ID)
	suite.Equal("User", result.Roles[1].Name)
	suite.Equal("default", result.Roles[1].OUHandle)
}

func (suite *RoleServiceTestSuite) TestGetRoleList_InvalidPagination() {
	testCases := []struct {
		name    string
		limit   int
		offset  int
		errCode string
	}{
		{"InvalidLimit_Zero", 0, 0, ErrorInvalidLimit.Code},
		{"InvalidLimit_TooLarge", serverconst.MaxPageSize + 1, 0, ErrorInvalidLimit.Code},
		{"InvalidOffset_Negative", 10, -1, ErrorInvalidOffset.Code},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			result, err := suite.service.GetRoleList(context.Background(), tc.limit, tc.offset)
			suite.Nil(result)
			suite.NotNil(err)
			suite.Equal(tc.errCode, err.Code)
		})
	}
}

func (suite *RoleServiceTestSuite) TestGetRoleList_StoreErrors() {
	testCases := []struct {
		name      string
		mockSetup func()
	}{
		{
			name: "CountError",
			mockSetup: func() {
				suite.mockStore.On("GetRoleListCount", mock.Anything).Return(0, errors.New("database error")).Once()
			},
		},
		{
			name: "GetListError",
			mockSetup: func() {
				suite.mockStore.On("GetRoleListCount", mock.Anything).Return(10, nil).Once()
				suite.mockStore.On("GetRoleList", mock.Anything,
					10, 0).
					Return([]Role{}, errors.New("database error")).Once()
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			tc.mockSetup()

			result, err := suite.service.GetRoleList(context.Background(), 10, 0)

			suite.Nil(result)
			suite.NotNil(err)
			suite.Equal(serviceerror.InternalServerError.Code, err.Code)
		})
	}
}

func (suite *RoleServiceTestSuite) TestGetRoleList_OUHandlesError() {
	expectedRoles := []Role{
		{ID: "role1", Name: "Admin", OUID: "ou1"},
	}

	suite.mockStore.On("GetRoleListCount", mock.Anything).Return(1, nil)
	suite.mockStore.On("GetRoleList", mock.Anything, 10, 0).Return(expectedRoles, nil)
	suite.mockOUService.On("GetOrganizationUnitHandlesByIDs", mock.Anything,
		[]string{"ou1"}).Return(nil, &serviceerror.ServiceError{Code: "INTERNAL_ERROR"})

	result, err := suite.service.GetRoleList(context.Background(), 10, 0)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(1, result.Count)
	suite.Equal("role1", result.Roles[0].ID)
	suite.Equal("", result.Roles[0].OUHandle)
}

// CreateRole Tests
func (suite *RoleServiceTestSuite) TestCreateRole_Success() {
	request := RoleCreationDetail{
		Name:        "Test Role",
		Description: "Test Description",
		OUID:        "ou1",
		Permissions: []ResourcePermissions{{ResourceServerID: "rs1", Permissions: []string{"perm1", "perm2"}}},
		Assignments: []RoleAssignment{
			{ID: testUserID1, Type: AssigneeTypeUser},
		},
	}

	ou := oupkg.OrganizationUnit{ID: "ou1", Name: "Test OU", Handle: "default"}
	suite.mockResourceService.On("ValidatePermissions", mock.Anything,
		"rs1", []string{"perm1", "perm2"}).Return([]string{}, nil)
	suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything,
		[]string{testUserID1}).Return([]entity.Entity{{ID: testUserID1, Category: entity.EntityCategoryUser}}, nil)
	suite.mockOUService.On("GetOrganizationUnit", mock.Anything, "ou1").Return(ou, nil)
	suite.mockStore.On("CheckRoleNameExists", mock.Anything,
		"ou1", "Test Role").Return(false, nil)
	suite.mockStore.On("CreateRole", mock.Anything,
		mock.AnythingOfType("string"),
		mock.AnythingOfType("RoleCreationDetail")).Return(nil)

	result, err := suite.service.CreateRole(context.Background(), request)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal("Test Role", result.Name)
	suite.Equal("Test Description", result.Description)
	suite.Equal("ou1", result.OUID)
	suite.Equal("default", result.OUHandle)
	suite.Equal(1, len(result.Permissions))
	suite.Equal(2, len(result.Permissions[0].Permissions))
	// Verify permission validation was called
	suite.mockResourceService.AssertCalled(suite.T(), "ValidatePermissions", mock.Anything,
		"rs1", []string{"perm1", "perm2"})
}

func (suite *RoleServiceTestSuite) TestCreateRole_ValidationErrors() {
	testCases := []struct {
		name    string
		request RoleCreationDetail
		errCode string
	}{
		{
			name: "MissingName",
			request: RoleCreationDetail{
				OUID: "ou1",
				Permissions: []ResourcePermissions{{
					ResourceServerID: "rs1",
					Permissions:      []string{"perm1"},
				}},
			},
			errCode: ErrorInvalidRequestFormat.Code,
		},
		{
			name: "MissingOrgUnit",
			request: RoleCreationDetail{
				Name: "Role",
				Permissions: []ResourcePermissions{{
					ResourceServerID: "rs1",
					Permissions:      []string{"perm1"},
				}},
			},
			errCode: ErrorInvalidRequestFormat.Code,
		},
		{
			name: "InvalidAssignmentType",
			request: RoleCreationDetail{
				Name:        "Role",
				OUID:        "ou1",
				Permissions: []ResourcePermissions{{ResourceServerID: "rs1", Permissions: []string{"perm1"}}},
				Assignments: []RoleAssignment{{ID: testUserID1, Type: "invalid"}},
			},
			errCode: ErrorInvalidAssigneeType.Code,
		},
		{
			name: "EmptyAssignmentID",
			request: RoleCreationDetail{
				Name:        "Role",
				OUID:        "ou1",
				Permissions: []ResourcePermissions{{ResourceServerID: "rs1", Permissions: []string{"perm1"}}},
				Assignments: []RoleAssignment{{ID: "", Type: AssigneeTypeUser}},
			},
			errCode: ErrorInvalidRequestFormat.Code,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			result, err := suite.service.CreateRole(context.Background(), tc.request)
			suite.Nil(result)
			suite.NotNil(err)
			suite.Equal(tc.errCode, err.Code)
		})
	}
}

func (suite *RoleServiceTestSuite) TestCreateRole_PermissionValidationErrors() {
	testCases := []struct {
		name          string
		request       RoleCreationDetail
		setupMocks    func()
		expectedError *serviceerror.ServiceError
	}{
		{
			name: "InvalidPermissions",
			request: RoleCreationDetail{
				Name:        "Test Role",
				OUID:        "ou1",
				Permissions: []ResourcePermissions{{ResourceServerID: "rs1", Permissions: []string{"perm1"}}},
			},
			setupMocks: func() {
				ou := oupkg.OrganizationUnit{ID: "ou1"}
				suite.mockOUService.On("GetOrganizationUnit", mock.Anything, "ou1").Return(ou, nil).Once()
				suite.mockResourceService.On("ValidatePermissions", mock.Anything,
					"rs1", []string{"perm1"}).
					Return([]string{"perm1"}, nil).Once()
			},
			expectedError: &ErrorInvalidPermissions,
		},
		{
			name: "PermissionValidationServiceError",
			request: RoleCreationDetail{
				Name:        "Test Role",
				OUID:        "ou1",
				Permissions: []ResourcePermissions{{ResourceServerID: "rs1", Permissions: []string{"perm1"}}},
			},
			setupMocks: func() {
				ou := oupkg.OrganizationUnit{ID: "ou1"}
				suite.mockOUService.On("GetOrganizationUnit", mock.Anything, "ou1").Return(ou, nil).Once()
				suite.mockResourceService.On("ValidatePermissions", mock.Anything,
					"rs1", []string{"perm1"}).
					Return([]string{}, &serviceerror.ServiceError{Code: "INTERNAL_ERROR"}).Once()
			},
			expectedError: &serviceerror.InternalServerError,
		},
		{
			name: "EmptyResourceServerID",
			request: RoleCreationDetail{
				Name:        "Test Role",
				OUID:        "ou1",
				Permissions: []ResourcePermissions{{ResourceServerID: "", Permissions: []string{"perm1"}}},
			},
			setupMocks: func() {
				ou := oupkg.OrganizationUnit{ID: "ou1"}
				suite.mockOUService.On("GetOrganizationUnit", mock.Anything, "ou1").Return(ou, nil).Once()
				// Resource service should not be called for empty resource server ID
			},
			expectedError: &ErrorInvalidPermissions,
		},
		{
			name: "EmptyPermissionsArray",
			request: RoleCreationDetail{
				Name:        "Test Role",
				Description: "Test Description",
				OUID:        "ou1",
				Permissions: []ResourcePermissions{},
			},
			setupMocks: func() {
				ou := oupkg.OrganizationUnit{ID: "ou1"}
				suite.mockOUService.On("GetOrganizationUnit", mock.Anything, "ou1").Return(ou, nil).Once()
				suite.mockStore.On("CheckRoleNameExists", mock.Anything,
					"ou1", "Test Role").Return(false, nil).Once()
				suite.mockStore.On("CreateRole", mock.Anything,
					mock.AnythingOfType("string"),
					mock.AnythingOfType("RoleCreationDetail")).Return(nil).Once()
				// Resource service should NOT be called for empty permissions
			},
			expectedError: nil, // Success case
		},
		{
			name: "MultipleResourceServers",
			request: RoleCreationDetail{
				Name:        "Test Role",
				Description: "Test Description",
				OUID:        "ou1",
				Permissions: []ResourcePermissions{
					{ResourceServerID: "rs1", Permissions: []string{"perm1"}},
					{ResourceServerID: "rs2", Permissions: []string{"perm2"}},
				},
			},
			setupMocks: func() {
				ou := oupkg.OrganizationUnit{ID: "ou1"}
				suite.mockOUService.On("GetOrganizationUnit", mock.Anything, "ou1").Return(ou, nil).Once()
				suite.mockResourceService.On("ValidatePermissions", mock.Anything,
					"rs1", []string{"perm1"}).
					Return([]string{}, nil).Once()
				suite.mockResourceService.On("ValidatePermissions", mock.Anything,
					"rs2", []string{"perm2"}).
					Return([]string{}, nil).Once()
				suite.mockStore.On("CheckRoleNameExists", mock.Anything,
					"ou1", "Test Role").Return(false, nil).Once()
				suite.mockStore.On("CreateRole", mock.Anything,
					mock.AnythingOfType("string"),
					mock.AnythingOfType("RoleCreationDetail")).Return(nil).Once()
			},
			expectedError: nil, // Success case
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			// Setup fresh mocks for this test case
			suite.SetupTest()
			tc.setupMocks()

			result, err := suite.service.CreateRole(context.Background(), tc.request)

			if tc.expectedError != nil {
				suite.Nil(result)
				suite.NotNil(err)
				suite.Equal(tc.expectedError.Code, err.Code)
			} else {
				suite.Nil(err)
				suite.NotNil(result)
			}
		})
	}
}

func (suite *RoleServiceTestSuite) TestCreateRole_OrganizationUnitNotFound() {
	request := RoleCreationDetail{
		Name:        "Test Role",
		OUID:        "nonexistent",
		Permissions: []ResourcePermissions{{ResourceServerID: "rs1", Permissions: []string{"perm1"}}},
	}

	suite.mockOUService.On("GetOrganizationUnit", mock.Anything, "nonexistent").
		Return(oupkg.OrganizationUnit{}, &oupkg.ErrorOrganizationUnitNotFound)

	result, err := suite.service.CreateRole(context.Background(), request)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorOrganizationUnitNotFound.Code, err.Code)
}

func (suite *RoleServiceTestSuite) TestCreateRole_InvalidUserID() {
	request := RoleCreationDetail{
		Name:        "Test Role",
		OUID:        "ou1",
		Permissions: []ResourcePermissions{{ResourceServerID: "rs1", Permissions: []string{"perm1"}}},
		Assignments: []RoleAssignment{{ID: "invalid_user", Type: AssigneeTypeUser}},
	}

	ou := oupkg.OrganizationUnit{ID: "ou1"}
	suite.mockOUService.On("GetOrganizationUnit", mock.Anything, "ou1").Return(ou, nil)
	suite.mockResourceService.On("ValidatePermissions", mock.Anything,
		"rs1", []string{"perm1"}).Return([]string{}, nil)
	suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything,
		[]string{"invalid_user"}).
		Return([]entity.Entity{}, nil)

	result, err := suite.service.CreateRole(context.Background(), request)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidAssignmentID.Code, err.Code)
}

func (suite *RoleServiceTestSuite) TestCreateRole_InvalidGroupID() {
	request := RoleCreationDetail{
		Name:        "Test Role",
		OUID:        "ou1",
		Permissions: []ResourcePermissions{{ResourceServerID: "rs1", Permissions: []string{"perm1"}}},
		Assignments: []RoleAssignment{{ID: "invalid_group", Type: AssigneeTypeGroup}},
	}

	ou := oupkg.OrganizationUnit{ID: "ou1"}
	suite.mockOUService.On("GetOrganizationUnit", mock.Anything, "ou1").Return(ou, nil)
	suite.mockResourceService.On("ValidatePermissions", mock.Anything,
		"rs1", []string{"perm1"}).Return([]string{}, nil)
	suite.mockGroupService.On("ValidateGroupIDs", mock.Anything,
		[]string{"invalid_group"}).
		Return(&group.ErrorInvalidGroupMemberID)

	result, err := suite.service.CreateRole(context.Background(), request)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidAssignmentID.Code, err.Code)
}

func (suite *RoleServiceTestSuite) TestCreateRole_StoreError() {
	request := RoleCreationDetail{
		Name:        "Test Role",
		OUID:        "ou1",
		Permissions: []ResourcePermissions{{ResourceServerID: "rs1", Permissions: []string{"perm1"}}},
	}

	ou := oupkg.OrganizationUnit{ID: "ou1"}
	suite.mockOUService.On("GetOrganizationUnit", mock.Anything, "ou1").Return(ou, nil)
	suite.mockResourceService.On("ValidatePermissions", mock.Anything,
		"rs1", []string{"perm1"}).Return([]string{}, nil)
	suite.mockStore.On("CheckRoleNameExists", mock.Anything,
		"ou1", "Test Role").Return(false, nil)
	suite.mockStore.On("CreateRole", mock.Anything,
		mock.AnythingOfType("string"),
		mock.AnythingOfType("RoleCreationDetail")).Return(errors.New("database error"))

	result, err := suite.service.CreateRole(context.Background(), request)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *RoleServiceTestSuite) TestCreateRole_NameConflict() {
	request := RoleCreationDetail{
		Name:        "Test Role",
		OUID:        "ou1",
		Permissions: []ResourcePermissions{{ResourceServerID: "rs1", Permissions: []string{"perm1"}}},
	}

	ou := oupkg.OrganizationUnit{ID: "ou1"}
	suite.mockOUService.On("GetOrganizationUnit", mock.Anything, "ou1").Return(ou, nil)
	suite.mockResourceService.On("ValidatePermissions", mock.Anything,
		"rs1", []string{"perm1"}).Return([]string{}, nil)
	suite.mockStore.On("CheckRoleNameExists", mock.Anything,
		"ou1", "Test Role").Return(true, nil)

	result, err := suite.service.CreateRole(context.Background(), request)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorRoleNameConflict.Code, err.Code)
}

func (suite *RoleServiceTestSuite) TestCreateRole_CheckNameExistsError() {
	request := RoleCreationDetail{
		Name:        "Test Role",
		OUID:        "ou1",
		Permissions: []ResourcePermissions{{ResourceServerID: "rs1", Permissions: []string{"perm1"}}},
	}

	ou := oupkg.OrganizationUnit{ID: "ou1"}
	suite.mockOUService.On("GetOrganizationUnit", mock.Anything, "ou1").Return(ou, nil)
	suite.mockResourceService.On("ValidatePermissions", mock.Anything,
		"rs1", []string{"perm1"}).Return([]string{}, nil)
	suite.mockStore.On("CheckRoleNameExists", mock.Anything,
		"ou1", "Test Role").
		Return(false, errors.New("database error"))

	result, err := suite.service.CreateRole(context.Background(), request)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

// CreateRole Declarative Mode Tests
func (suite *RoleServiceTestSuite) TestCreateRole_DeclarativeMode_Denied() {
	// Setup declarative-only mode
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: true,
		},
		Role: config.RoleConfig{
			Store: "declarative",
		},
	}
	config.ResetServerRuntime()
	initErr := config.InitializeServerRuntime("/tmp/test", testConfig)
	if initErr != nil {
		suite.Fail("Failed to initialize runtime", initErr)
	}
	defer config.ResetServerRuntime()

	request := RoleCreationDetail{
		Name: "Test Role",
		OUID: "ou1",
	}

	result, err := suite.service.CreateRole(context.Background(), request)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorDeclarativeModeCreateNotAllowed.Code, err.Code)
}

func (suite *RoleServiceTestSuite) TestUpdateRole_DeclarativeMode_Denied() {
	// Setup declarative-only mode
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: true,
		},
		Role: config.RoleConfig{
			Store: "declarative",
		},
	}
	config.ResetServerRuntime()
	initErr := config.InitializeServerRuntime("/tmp/test", testConfig)
	if initErr != nil {
		suite.Fail("Failed to initialize runtime", initErr)
	}
	defer config.ResetServerRuntime()

	request := RoleUpdateDetail{
		Name:        "Updated Role",
		OUID:        "ou1",
		Permissions: []ResourcePermissions{{ResourceServerID: "rs1", Permissions: []string{"perm1"}}},
	}

	suite.mockResourceService.On("ValidatePermissions", mock.Anything,
		"rs1", []string{"perm1"}).Return([]string{}, nil)
	suite.mockStore.On("IsRoleExist", mock.Anything, "role1").Return(true, nil)
	suite.mockStore.On("IsRoleDeclarative", mock.Anything, "role1").Return(true, nil)

	result, err := suite.service.UpdateRoleWithPermissions(context.Background(), "role1", request)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorImmutableRole.Code, err.Code)
}

// GetRoleWithPermissions Tests
func (suite *RoleServiceTestSuite) TestGetRole_Success() {
	expectedRole := RoleWithPermissions{
		ID:          "role1",
		Name:        "Admin",
		Description: "Administrator role",
		OUID:        "ou1",
		Permissions: []ResourcePermissions{{ResourceServerID: "rs1", Permissions: []string{"perm1", "perm2"}}},
	}

	suite.mockStore.On("GetRole", mock.Anything, "role1").Return(expectedRole, nil)
	suite.mockOUService.On("GetOrganizationUnit", mock.Anything,
		"ou1").Return(oupkg.OrganizationUnit{ID: "ou1", Handle: "default"}, nil)

	result, err := suite.service.GetRoleWithPermissions(context.Background(), "role1")

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(expectedRole.ID, result.ID)
	suite.Equal(expectedRole.Name, result.Name)
	suite.Equal("default", result.OUHandle)
}

func (suite *RoleServiceTestSuite) TestGetRole_OUHandleError() {
	expectedRole := RoleWithPermissions{
		ID:   "role1",
		Name: "Admin",
		OUID: "ou1",
	}

	suite.mockStore.On("GetRole", mock.Anything, "role1").Return(expectedRole, nil)
	suite.mockOUService.On("GetOrganizationUnit", mock.Anything,
		"ou1").Return(oupkg.OrganizationUnit{}, &serviceerror.ServiceError{Code: "INTERNAL_ERROR"})

	result, err := suite.service.GetRoleWithPermissions(context.Background(), "role1")

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal("role1", result.ID)
	suite.Equal("Admin", result.Name)
	suite.Equal("", result.OUHandle)
}

func (suite *RoleServiceTestSuite) TestGetRole_MissingID() {
	result, err := suite.service.GetRoleWithPermissions(context.Background(), "")

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorMissingRoleID.Code, err.Code)
}

func (suite *RoleServiceTestSuite) TestGetRole_NotFound() {
	suite.mockStore.On("GetRole", mock.Anything,
		"nonexistent").Return(RoleWithPermissions{}, ErrRoleNotFound)

	result, err := suite.service.GetRoleWithPermissions(context.Background(), "nonexistent")

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorRoleNotFound.Code, err.Code)
}

func (suite *RoleServiceTestSuite) TestGetRole_StoreError() {
	suite.mockStore.On("GetRole", mock.Anything,
		"role1").Return(RoleWithPermissions{}, errors.New("database error"))

	result, err := suite.service.GetRoleWithPermissions(context.Background(), "role1")

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

// UpdateRole Tests
func (suite *RoleServiceTestSuite) TestUpdateRole_MissingRoleID() {
	request := RoleUpdateDetail{
		Name:        "New Name",
		OUID:        "ou1",
		Permissions: []ResourcePermissions{{ResourceServerID: "rs1", Permissions: []string{"perm1"}}},
	}

	result, err := suite.service.UpdateRoleWithPermissions(context.Background(), "", request)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorMissingRoleID.Code, err.Code)
}

func (suite *RoleServiceTestSuite) TestUpdateRole_ValidationErrors() {
	testCases := []struct {
		name    string
		request RoleUpdateDetail
		errCode string
	}{
		{
			name: "MissingName",
			request: RoleUpdateDetail{
				OUID: "ou1",
				Permissions: []ResourcePermissions{{
					ResourceServerID: "rs1",
					Permissions:      []string{"perm1"},
				}},
			},
			errCode: ErrorInvalidRequestFormat.Code,
		},
		{
			name: "MissingOrgUnit",
			request: RoleUpdateDetail{
				Name: "Role",
				Permissions: []ResourcePermissions{{
					ResourceServerID: "rs1",
					Permissions:      []string{"perm1"},
				}},
			},
			errCode: ErrorInvalidRequestFormat.Code,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			result, err := suite.service.UpdateRoleWithPermissions(context.Background(), "role1", tc.request)
			suite.Nil(result)
			suite.NotNil(err)
			suite.Equal(tc.errCode, err.Code)
		})
	}
}

func (suite *RoleServiceTestSuite) TestUpdateRole_GetRoleError() {
	request := RoleUpdateDetail{
		Name:        "New Name",
		OUID:        "ou1",
		Permissions: []ResourcePermissions{{ResourceServerID: "rs1", Permissions: []string{"perm1"}}},
	}

	suite.mockResourceService.On("ValidatePermissions", mock.Anything,
		"rs1", []string{"perm1"}).Return([]string{}, nil)
	suite.mockStore.On("IsRoleExist", mock.Anything,
		"role1").Return(false, errors.New("database error"))

	result, err := suite.service.UpdateRoleWithPermissions(context.Background(), "role1", request)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *RoleServiceTestSuite) TestUpdateRole_OUNotFound() {
	request := RoleUpdateDetail{
		Name:        "New Name",
		OUID:        "nonexistent_ou",
		Permissions: []ResourcePermissions{{ResourceServerID: "rs1", Permissions: []string{"perm1"}}},
	}

	suite.mockResourceService.On("ValidatePermissions", mock.Anything,
		"rs1", []string{"perm1"}).Return([]string{}, nil)
	suite.mockStore.On("IsRoleExist", mock.Anything,
		"role1").Return(true, nil)
	suite.mockOUService.On("GetOrganizationUnit", mock.Anything, "nonexistent_ou").
		Return(oupkg.OrganizationUnit{}, &oupkg.ErrorOrganizationUnitNotFound)

	result, err := suite.service.UpdateRoleWithPermissions(context.Background(), "role1", request)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorOrganizationUnitNotFound.Code, err.Code)
}

func (suite *RoleServiceTestSuite) TestUpdateRole_OUServiceError() {
	request := RoleUpdateDetail{
		Name:        "New Name",
		OUID:        "ou1",
		Permissions: []ResourcePermissions{{ResourceServerID: "rs1", Permissions: []string{"perm1"}}},
	}

	suite.mockResourceService.On("ValidatePermissions", mock.Anything,
		"rs1", []string{"perm1"}).Return([]string{}, nil)
	suite.mockStore.On("IsRoleExist", mock.Anything,
		"role1").Return(true, nil)
	suite.mockOUService.On("GetOrganizationUnit", mock.Anything, "ou1").
		Return(oupkg.OrganizationUnit{}, &serviceerror.ServiceError{Code: "INTERNAL_ERROR"})

	result, err := suite.service.UpdateRoleWithPermissions(context.Background(), "role1", request)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *RoleServiceTestSuite) TestUpdateRole_UpdateStoreError() {
	request := RoleUpdateDetail{
		Name:        "New Name",
		OUID:        "ou1",
		Permissions: []ResourcePermissions{{ResourceServerID: "rs1", Permissions: []string{"perm1"}}},
	}

	ou := oupkg.OrganizationUnit{ID: "ou1"}
	suite.mockResourceService.On("ValidatePermissions", mock.Anything,
		"rs1", []string{"perm1"}).Return([]string{}, nil)
	suite.mockStore.On("IsRoleExist", mock.Anything,
		"role1").Return(true, nil)
	suite.mockOUService.On("GetOrganizationUnit", mock.Anything, "ou1").Return(ou, nil)
	suite.mockStore.On("CheckRoleNameExistsExcludingID", mock.Anything,
		"ou1", "New Name", "role1").Return(false, nil)
	suite.mockStore.On("UpdateRole", mock.Anything,
		mock.AnythingOfType("string"),
		mock.AnythingOfType("RoleUpdateDetail")).Return(errors.New("update error"))

	result, err := suite.service.UpdateRoleWithPermissions(context.Background(), "role1", request)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *RoleServiceTestSuite) TestUpdateRole_Success() {
	// This test also verifies permission validation is called correctly during update
	request := RoleUpdateDetail{
		Name:        "New Name",
		Description: "Updated description",
		OUID:        "ou1",
		Permissions: []ResourcePermissions{{ResourceServerID: "rs1", Permissions: []string{"perm1", "perm2"}}},
	}

	ou := oupkg.OrganizationUnit{ID: "ou1", Handle: "default"}
	suite.mockResourceService.On("ValidatePermissions", mock.Anything,
		"rs1", []string{"perm1", "perm2"}).Return([]string{}, nil)
	suite.mockStore.On("IsRoleExist", mock.Anything,
		"role1").Return(true, nil)
	suite.mockOUService.On("GetOrganizationUnit", mock.Anything, "ou1").Return(ou, nil)
	suite.mockStore.On("CheckRoleNameExistsExcludingID", mock.Anything,
		"ou1", "New Name", "role1").Return(false, nil)
	suite.mockStore.On("UpdateRole", mock.Anything,
		mock.AnythingOfType("string"),
		mock.AnythingOfType("RoleUpdateDetail")).Return(nil)

	result, err := suite.service.UpdateRoleWithPermissions(context.Background(), "role1", request)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal("New Name", result.Name)
	suite.Equal("Updated description", result.Description)
	suite.Equal("default", result.OUHandle)
	// Verify permission validation was called
	suite.mockResourceService.AssertCalled(suite.T(), "ValidatePermissions", mock.Anything,
		"rs1", []string{"perm1", "perm2"})
}

func (suite *RoleServiceTestSuite) TestUpdateRole_RoleNotFound() {
	request := RoleUpdateDetail{
		Name:        "New Name",
		OUID:        "ou1",
		Permissions: []ResourcePermissions{{ResourceServerID: "rs1", Permissions: []string{"perm1"}}},
	}

	suite.mockResourceService.On("ValidatePermissions", mock.Anything,
		"rs1", []string{"perm1"}).Return([]string{}, nil)
	suite.mockStore.On("IsRoleExist", mock.Anything,
		"nonexistent").Return(false, nil)

	result, err := suite.service.UpdateRoleWithPermissions(context.Background(), "nonexistent", request)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorRoleNotFound.Code, err.Code)
}

func (suite *RoleServiceTestSuite) TestUpdateRole_NameConflict() {
	request := RoleUpdateDetail{
		Name:        "Conflicting Name",
		OUID:        "ou1",
		Permissions: []ResourcePermissions{{ResourceServerID: "rs1", Permissions: []string{"perm1"}}},
	}

	ou := oupkg.OrganizationUnit{ID: "ou1"}
	suite.mockResourceService.On("ValidatePermissions", mock.Anything,
		"rs1", []string{"perm1"}).Return([]string{}, nil)
	suite.mockStore.On("IsRoleExist", mock.Anything,
		"role1").Return(true, nil)
	suite.mockOUService.On("GetOrganizationUnit", mock.Anything, "ou1").Return(ou, nil)
	suite.mockStore.On("CheckRoleNameExistsExcludingID", mock.Anything,
		"ou1", "Conflicting Name",
		"role1").Return(true, nil)

	result, err := suite.service.UpdateRoleWithPermissions(context.Background(), "role1", request)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorRoleNameConflict.Code, err.Code)
}

func (suite *RoleServiceTestSuite) TestUpdateRole_CheckNameExistsError() {
	request := RoleUpdateDetail{
		Name:        "New Name",
		OUID:        "ou1",
		Permissions: []ResourcePermissions{{ResourceServerID: "rs1", Permissions: []string{"perm1"}}},
	}

	ou := oupkg.OrganizationUnit{ID: "ou1"}
	suite.mockResourceService.On("ValidatePermissions", mock.Anything,
		"rs1", []string{"perm1"}).Return([]string{}, nil)
	suite.mockStore.On("IsRoleExist", mock.Anything,
		"role1").Return(true, nil)
	suite.mockOUService.On("GetOrganizationUnit", mock.Anything, "ou1").Return(ou, nil)
	suite.mockStore.On("CheckRoleNameExistsExcludingID", mock.Anything,
		"ou1", "New Name", "role1").
		Return(false, errors.New("database error"))

	result, err := suite.service.UpdateRoleWithPermissions(context.Background(), "role1", request)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *RoleServiceTestSuite) TestUpdateRole_PermissionValidationErrors() {
	testCases := []struct {
		name          string
		request       RoleUpdateDetail
		setupMocks    func()
		expectedError *serviceerror.ServiceError
	}{
		{
			name: "InvalidPermissionsOnUpdate",
			request: RoleUpdateDetail{
				Name:        "Updated Role",
				OUID:        "ou1",
				Permissions: []ResourcePermissions{{ResourceServerID: "rs1", Permissions: []string{"perm1"}}},
			},
			setupMocks: func() {
				// Permission validation happens before IsRoleExist check in UpdateRole
				suite.mockResourceService.On("ValidatePermissions", mock.Anything,
					"rs1", []string{"perm1"}).
					Return([]string{"perm1"}, nil).Once()
			},
			expectedError: &ErrorInvalidPermissions,
		},
		{
			name: "PermissionValidationServiceError",
			request: RoleUpdateDetail{
				Name:        "Updated Role",
				OUID:        "ou1",
				Permissions: []ResourcePermissions{{ResourceServerID: "rs1", Permissions: []string{"perm1"}}},
			},
			setupMocks: func() {
				// Permission validation happens before IsRoleExist check in UpdateRole
				suite.mockResourceService.On("ValidatePermissions", mock.Anything,
					"rs1", []string{"perm1"}).
					Return([]string{}, &serviceerror.ServiceError{Code: "INTERNAL_ERROR"}).Once()
			},
			expectedError: &serviceerror.InternalServerError,
		},
		{
			name: "EmptyResourceServerIDOnUpdate",
			request: RoleUpdateDetail{
				Name:        "Updated Role",
				OUID:        "ou1",
				Permissions: []ResourcePermissions{{ResourceServerID: "", Permissions: []string{"perm1"}}},
			},
			setupMocks: func() {
				// Resource service should not be called for empty resource server ID
				// Early validation should fail before any other calls
			},
			expectedError: &ErrorInvalidPermissions,
		},
		{
			name: "MultipleResourceServersOnUpdate",
			request: RoleUpdateDetail{
				Name:        "Updated Role",
				Description: "Updated description",
				OUID:        "ou1",
				Permissions: []ResourcePermissions{
					{ResourceServerID: "rs1", Permissions: []string{"perm1"}},
					{ResourceServerID: "rs2", Permissions: []string{"perm2"}},
				},
			},
			setupMocks: func() {
				ou := oupkg.OrganizationUnit{ID: "ou1"}
				suite.mockStore.On("IsRoleExist", mock.Anything,
					"role1").Return(true, nil).Once()
				suite.mockResourceService.On("ValidatePermissions", mock.Anything,
					"rs1", []string{"perm1"}).
					Return([]string{}, nil).Once()
				suite.mockResourceService.On("ValidatePermissions", mock.Anything,
					"rs2", []string{"perm2"}).
					Return([]string{}, nil).Once()
				suite.mockOUService.On("GetOrganizationUnit", mock.Anything, "ou1").Return(ou, nil).Once()
				suite.mockStore.On("CheckRoleNameExistsExcludingID", mock.Anything,
					"ou1",
					"Updated Role", "role1").Return(false, nil).Once()
				suite.mockStore.On("UpdateRole", mock.Anything,
					mock.AnythingOfType("string"),
					mock.AnythingOfType("RoleUpdateDetail")).Return(nil).Once()
			},
			expectedError: nil, // Success case
		},
		{
			name: "EmptyPermissionsArrayOnUpdate",
			request: RoleUpdateDetail{
				Name:        "Updated Role",
				Description: "Updated description",
				OUID:        "ou1",
				Permissions: []ResourcePermissions{},
			},
			setupMocks: func() {
				ou := oupkg.OrganizationUnit{ID: "ou1"}
				suite.mockStore.On("IsRoleExist", mock.Anything,
					"role1").Return(true, nil).Once()
				suite.mockOUService.On("GetOrganizationUnit", mock.Anything, "ou1").Return(ou, nil).Once()
				suite.mockStore.On("CheckRoleNameExistsExcludingID", mock.Anything,
					"ou1",
					"Updated Role", "role1").Return(false, nil).Once()
				suite.mockStore.On("UpdateRole", mock.Anything,
					mock.AnythingOfType("string"),
					mock.AnythingOfType("RoleUpdateDetail")).Return(nil).Once()
				// Resource service should NOT be called for empty permissions
			},
			expectedError: nil, // Success case
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			// Setup fresh mocks for this test case
			suite.SetupTest()
			tc.setupMocks()

			result, err := suite.service.UpdateRoleWithPermissions(context.Background(), "role1", tc.request)

			if tc.expectedError != nil {
				suite.Nil(result)
				suite.NotNil(err)
				suite.Equal(tc.expectedError.Code, err.Code)
			} else {
				suite.Nil(err)
				suite.NotNil(result)
			}
		})
	}
}

// DeleteRole Tests
func (suite *RoleServiceTestSuite) TestDeleteRole_Success() {
	suite.mockStore.On("IsRoleExist", mock.Anything,
		"role1").Return(true, nil)
	suite.mockStore.On("DeleteAssignmentsByRoleID", mock.Anything,
		"role1").Return(nil)
	suite.mockStore.On("DeleteRole", mock.Anything,
		"role1").Return(nil)

	err := suite.service.DeleteRole(context.Background(), "role1")

	suite.Nil(err)
}

func (suite *RoleServiceTestSuite) TestDeleteRole_WithAssignments() {
	suite.mockStore.On("IsRoleExist", mock.Anything,
		"role1").Return(true, nil)
	suite.mockStore.On("DeleteAssignmentsByRoleID", mock.Anything,
		"role1").Return(nil)
	suite.mockStore.On("DeleteRole", mock.Anything,
		"role1").Return(nil)

	err := suite.service.DeleteRole(context.Background(), "role1")

	suite.Nil(err)
}

func (suite *RoleServiceTestSuite) TestDeleteRole_NotFound_ReturnsNil() {
	suite.mockStore.On("IsRoleExist", mock.Anything,
		"nonexistent").Return(false, nil)

	err := suite.service.DeleteRole(context.Background(), "nonexistent")

	suite.Nil(err)
}

func (suite *RoleServiceTestSuite) TestDeleteRole_MissingID() {
	err := suite.service.DeleteRole(context.Background(), "")

	suite.NotNil(err)
	suite.Equal(ErrorMissingRoleID.Code, err.Code)
}

func (suite *RoleServiceTestSuite) TestDeleteRole_GetRoleError() {
	suite.mockStore.On("IsRoleExist", mock.Anything,
		"role1").Return(false, errors.New("database error"))

	err := suite.service.DeleteRole(context.Background(), "role1")

	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *RoleServiceTestSuite) TestDeleteRole_GetAssignmentsCountError() {
	suite.mockStore.On("IsRoleExist", mock.Anything,
		"role1").Return(true, nil)
	suite.mockStore.On("DeleteAssignmentsByRoleID", mock.Anything,
		"role1").Return(errors.New("database error"))

	err := suite.service.DeleteRole(context.Background(), "role1")

	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *RoleServiceTestSuite) TestDeleteRole_StoreError() {
	suite.mockStore.On("IsRoleExist", mock.Anything,
		"role1").Return(true, nil)
	suite.mockStore.On("DeleteAssignmentsByRoleID", mock.Anything,
		"role1").Return(nil)
	suite.mockStore.On("DeleteRole", mock.Anything,
		"role1").Return(errors.New("delete error"))

	err := suite.service.DeleteRole(context.Background(), "role1")

	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

// DeleteRole Declarative Mode Tests
func (suite *RoleServiceTestSuite) TestDeleteRole_DeclarativeMode_Denied() {
	// Setup declarative-only mode
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: true,
		},
		Role: config.RoleConfig{
			Store: "declarative",
		},
	}
	config.ResetServerRuntime()
	initErr := config.InitializeServerRuntime("/tmp/test", testConfig)
	if initErr != nil {
		suite.Fail("Failed to initialize runtime", initErr)
	}
	defer config.ResetServerRuntime()

	suite.mockStore.On("IsRoleExist", mock.Anything, "role1").Return(true, nil)
	suite.mockStore.On("IsRoleDeclarative", mock.Anything, "role1").Return(true, nil)

	err2 := suite.service.DeleteRole(context.Background(), "role1")

	suite.NotNil(err2)
	suite.Equal(ErrorImmutableRole.Code, err2.Code)
}

// validateAssignmentIDs Tests
func (suite *RoleServiceTestSuite) TestValidateAssignmentIDs_UserServiceError() {
	request := RoleCreationDetail{
		Name:        "Test Role",
		OUID:        "ou1",
		Permissions: []ResourcePermissions{{ResourceServerID: "rs1", Permissions: []string{"perm1"}}},
		Assignments: []RoleAssignment{{ID: "user1", Type: AssigneeTypeUser}},
	}

	ou := oupkg.OrganizationUnit{ID: "ou1"}
	suite.mockOUService.On("GetOrganizationUnit", mock.Anything, "ou1").Return(ou, nil)
	suite.mockResourceService.On("ValidatePermissions", mock.Anything,
		"rs1", []string{"perm1"}).Return([]string{}, nil)
	suite.mockEntityService.On("GetEntitiesByIDs", mock.Anything,
		[]string{"user1"}).
		Return([]entity.Entity{}, errors.New("internal error"))

	result, err := suite.service.CreateRole(context.Background(), request)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorInternalServerError.Code, err.Code)
}

func (suite *RoleServiceTestSuite) TestValidateAssignmentIDs_GroupServiceError() {
	request := RoleCreationDetail{
		Name:        "Test Role",
		OUID:        "ou1",
		Permissions: []ResourcePermissions{{ResourceServerID: "rs1", Permissions: []string{"perm1"}}},
		Assignments: []RoleAssignment{{ID: "group1", Type: AssigneeTypeGroup}},
	}

	ou := oupkg.OrganizationUnit{ID: "ou1"}
	suite.mockOUService.On("GetOrganizationUnit", mock.Anything, "ou1").Return(ou, nil)
	suite.mockResourceService.On("ValidatePermissions", mock.Anything,
		"rs1", []string{"perm1"}).Return([]string{}, nil)
	suite.mockGroupService.On("ValidateGroupIDs", mock.Anything,
		[]string{"group1"}).
		Return(&serviceerror.ServiceError{Code: "INTERNAL_ERROR"})

	result, err := suite.service.CreateRole(context.Background(), request)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

// Utility functions tests
func (suite *RoleServiceTestSuite) TestBuildPaginationLinks() {
	testCases := []struct {
		name        string
		base        string
		limit       int
		offset      int
		totalCount  int
		expectFirst bool
		expectPrev  bool
		expectNext  bool
		expectLast  bool
	}{
		{
			name:        "FirstPage",
			base:        "/roles",
			limit:       10,
			offset:      0,
			totalCount:  30,
			expectFirst: false,
			expectPrev:  false,
			expectNext:  true,
			expectLast:  true,
		},
		{
			name:        "MiddlePage",
			base:        "/roles",
			limit:       10,
			offset:      10,
			totalCount:  30,
			expectFirst: true,
			expectPrev:  true,
			expectNext:  true,
			expectLast:  true,
		},
		{
			name:        "LastPage",
			base:        "/roles",
			limit:       10,
			offset:      20,
			totalCount:  30,
			expectFirst: true,
			expectPrev:  true,
			expectNext:  false,
			expectLast:  false,
		},
		{
			name:        "SinglePage",
			base:        "/roles",
			limit:       10,
			offset:      0,
			totalCount:  5,
			expectFirst: false,
			expectPrev:  false,
			expectNext:  false,
			expectLast:  false,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			links := utils.BuildPaginationLinks(tc.base, tc.limit, tc.offset, tc.totalCount, "")

			hasFirst := false
			hasPrev := false
			hasNext := false
			hasLast := false

			for _, link := range links {
				switch link.Rel {
				case "first":
					hasFirst = true
				case "prev":
					hasPrev = true
				case "next":
					hasNext = true
				case "last":
					hasLast = true
				}
			}

			suite.Equal(tc.expectFirst, hasFirst, "first link mismatch")
			suite.Equal(tc.expectPrev, hasPrev, "prev link mismatch")
			suite.Equal(tc.expectNext, hasNext, "next link mismatch")
			suite.Equal(tc.expectLast, hasLast, "last link mismatch")
		})
	}
}

// GetAuthorizedPermissions Tests - Consolidated for efficiency while maintaining coverage
func (suite *RoleServiceTestSuite) TestGetAuthorizedPermissions() {
	testCases := []struct {
		name                 string
		userID               string
		groups               []string
		requestedPermissions []string
		mockReturn           []string
		mockError            error
		expectedPermissions  []string
		expectedError        *serviceerror.ServiceError
		skipMock             bool
	}{
		{
			name:                 "Success_UserAndGroups",
			userID:               testUserID1,
			groups:               []string{"group1", "group2"},
			requestedPermissions: []string{"perm1", "perm2", "perm3"},
			mockReturn:           []string{"perm1", "perm3"},
			expectedPermissions:  []string{"perm1", "perm3"},
		},
		{
			name:                 "Success_UserOnly_NilGroupsNormalized",
			userID:               testUserID1,
			groups:               nil, // Tests both nil and empty groups normalization
			requestedPermissions: []string{"perm1", "perm2"},
			mockReturn:           []string{"perm1"},
			expectedPermissions:  []string{"perm1"},
		},
		{
			name:                 "Success_GroupsOnly",
			userID:               "",
			groups:               []string{"group1", "group2"},
			requestedPermissions: []string{"perm1", "perm2"},
			mockReturn:           []string{"perm1"},
			expectedPermissions:  []string{"perm1"},
		},
		{
			name:                 "Success_NoAuthorizedPermissions",
			userID:               testUserID1,
			groups:               []string{"group1"},
			requestedPermissions: []string{"perm1", "perm2"},
			mockReturn:           []string{}, // User has no permissions
			expectedPermissions:  []string{},
		},
		{
			name:                 "Success_AllPermissionsAuthorized",
			userID:               testUserID1,
			groups:               []string{"group1"},
			requestedPermissions: []string{"perm1", "perm2"},
			mockReturn:           []string{"perm1", "perm2"}, // All permissions authorized
			expectedPermissions:  []string{"perm1", "perm2"},
		},
		{
			name:                 "EmptyAndNilRequestedPermissions_ReturnsEmpty",
			userID:               testUserID1,
			groups:               []string{"group1"},
			requestedPermissions: nil, // Also covers empty []string{} case
			expectedPermissions:  []string{},
			skipMock:             true, // No store call for empty permissions
		},
		{
			name:                 "MissingUserAndGroups_Error",
			userID:               "",
			groups:               nil, // Covers both nil and empty cases
			requestedPermissions: []string{"perm1", "perm2"},
			expectedError:        &ErrorMissingEntityOrGroups,
			skipMock:             true,
		},
		{
			name:                 "StoreError_ReturnsInternalError",
			userID:               testUserID1,
			groups:               []string{"group1"},
			requestedPermissions: []string{"perm1", "perm2"},
			mockError:            errors.New("database error"),
			expectedError:        &serviceerror.InternalServerError,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			if !tc.skipMock {
				normalizedGroups := tc.groups
				if normalizedGroups == nil {
					normalizedGroups = []string{}
				}
				suite.mockStore.On("GetAuthorizedPermissions", mock.Anything,
					tc.userID, normalizedGroups,
					tc.requestedPermissions).
					Return(tc.mockReturn, tc.mockError).Once()
			}

			result, err := suite.service.GetAuthorizedPermissions(
				context.Background(), tc.userID, tc.groups, tc.requestedPermissions)

			if tc.expectedError != nil {
				suite.NotNil(err)
				suite.Equal(tc.expectedError.Code, err.Code)
				suite.Nil(result)
			} else {
				suite.Nil(err)
				suite.NotNil(result)
				if len(tc.requestedPermissions) == 0 {
					suite.Equal(0, len(result))
				} else {
					suite.Equal(len(tc.expectedPermissions), len(result))
					suite.Equal(tc.expectedPermissions, result)
				}
			}
		})
	}
}

// Tests for IsRoleDeclarative (public method)
func (suite *RoleServiceTestSuite) TestIsRoleDeclarative_ReturnsTrue() {
	suite.mockStore.On("IsRoleDeclarative", mock.Anything, "declarative-role").Return(true, nil)

	isDeclarative, err := suite.service.IsRoleDeclarative(context.Background(), "declarative-role")

	suite.Nil(err)
	suite.True(isDeclarative)
	suite.mockStore.AssertCalled(suite.T(), "IsRoleDeclarative", mock.Anything, "declarative-role")
}

func (suite *RoleServiceTestSuite) TestIsRoleDeclarative_ReturnsFalse() {
	suite.mockStore.On("IsRoleDeclarative", mock.Anything, "mutable-role").Return(false, nil)

	isDeclarative, err := suite.service.IsRoleDeclarative(context.Background(), "mutable-role")

	suite.Nil(err)
	suite.False(isDeclarative)
}

func (suite *RoleServiceTestSuite) TestIsRoleDeclarative_StoreReturnsError() {
	storeErr := errors.New("store error")
	suite.mockStore.On("IsRoleDeclarative", mock.Anything, "role-id").Return(false, storeErr)

	isDeclarative, err := suite.service.IsRoleDeclarative(context.Background(), "role-id")

	suite.NotNil(err)
	suite.False(isDeclarative)
	suite.Equal(&serviceerror.InternalServerError, err)
}
