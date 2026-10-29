// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"context"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/sharing"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/sharingmock"
)

// RoleExporterTestSuite contains tests for the roleExporter.
type RoleExporterTestSuite struct {
	suite.Suite
	mockService           *RoleServiceInterfaceMock
	mockAssignmentService *RoleAssignmentServiceInterfaceMock
	mockSharingService    *sharingmock.SharingServiceInterfaceMock
	exporter              declarativeresource.ResourceExporter
	ctx                   context.Context
}

func TestRoleExporterTestSuite(t *testing.T) {
	suite.Run(t, new(RoleExporterTestSuite))
}

func (suite *RoleExporterTestSuite) SetupTest() {
	suite.mockService = NewRoleServiceInterfaceMock(suite.T())
	suite.mockAssignmentService = NewRoleAssignmentServiceInterfaceMock(suite.T())
	suite.mockSharingService = sharingmock.NewSharingServiceInterfaceMock(suite.T())
	// A role records no sharing policy unless a test says otherwise.
	suite.mockSharingService.EXPECT().ExportPolicies(mock.Anything, RoleSharingType, mock.Anything).
		Return(nil, nil).Maybe()
	suite.exporter = newRoleExporter(suite.mockService, suite.mockAssignmentService, suite.mockSharingService)
	suite.ctx = context.Background()
}

// Test GetResourceType
func (suite *RoleExporterTestSuite) TestGetResourceType() {
	assert.Equal(suite.T(), resourceTypeRole, suite.exporter.GetResourceType())
}

// Test GetParameterizerType
func (suite *RoleExporterTestSuite) TestGetParameterizerType() {
	assert.Equal(suite.T(), paramTypeRole, suite.exporter.GetParameterizerType())
}

// Test GetResourceRules
func (suite *RoleExporterTestSuite) TestGetResourceRules() {
	rules := suite.exporter.GetResourceRules()
	assert.NotNil(suite.T(), rules)
	assert.Empty(suite.T(), rules.Variables)
	assert.Empty(suite.T(), rules.ArrayVariables)
}

// Test GetAllResourceIDs - single page
func (suite *RoleExporterTestSuite) TestGetAllResourceIDs_SinglePage() {
	roleList := &RoleList{
		Roles: []Role{
			{ID: "role1", Name: "Admin", OUID: "ou1"},
			{ID: "role2", Name: "Viewer", OUID: "ou1"},
		},
		TotalResults: 2,
	}

	suite.mockService.On("GetRoleList", suite.ctx, serverconst.MaxPageSize, 0).Return(
		roleList, nil,
	)
	suite.mockService.On("GetRoleList", suite.ctx, serverconst.MaxPageSize, 2).Return(
		&RoleList{Roles: []Role{}, TotalResults: 2}, nil,
	)
	suite.mockService.On("IsRoleDeclarative", suite.ctx, "role1").Return(false, nil)
	suite.mockService.On("IsRoleDeclarative", suite.ctx, "role2").Return(false, nil)

	ids, err := suite.exporter.GetAllResourceIDs(suite.ctx)

	suite.Nil(err)
	assert.Len(suite.T(), ids, 2)
	assert.Contains(suite.T(), ids, "role1")
	assert.Contains(suite.T(), ids, "role2")
	suite.mockService.AssertExpectations(suite.T())
}

// Test GetAllResourceIDs - multiple pages
func (suite *RoleExporterTestSuite) TestGetAllResourceIDs_MultiplePages() {
	page1 := &RoleList{
		Roles: []Role{
			{ID: "role1", Name: "Admin", OUID: "ou1"},
		},
		TotalResults: 2,
	}
	page2 := &RoleList{
		Roles: []Role{
			{ID: "role2", Name: "Viewer", OUID: "ou1"},
		},
		TotalResults: 2,
	}
	emptyPage := &RoleList{
		Roles:        []Role{},
		TotalResults: 2,
	}

	suite.mockService.On("GetRoleList", suite.ctx, serverconst.MaxPageSize, 0).Return(page1, nil)
	suite.mockService.On("GetRoleList", suite.ctx, serverconst.MaxPageSize, 1).Return(page2, nil)
	suite.mockService.On("GetRoleList", suite.ctx, serverconst.MaxPageSize, 2).Return(emptyPage, nil)
	suite.mockService.On("IsRoleDeclarative", suite.ctx, "role1").Return(false, nil)
	suite.mockService.On("IsRoleDeclarative", suite.ctx, "role2").Return(false, nil)

	ids, err := suite.exporter.GetAllResourceIDs(suite.ctx)

	suite.Nil(err)
	assert.Len(suite.T(), ids, 2)
	suite.mockService.AssertExpectations(suite.T())
}

// Test GetAllResourceIDs - excludes declarative roles
func (suite *RoleExporterTestSuite) TestGetAllResourceIDs_ExcludesDeclarativeRoles() {
	roleList := &RoleList{
		Roles: []Role{
			{ID: "role1", Name: "Admin", OUID: "ou1"},
			{ID: "role-declarative", Name: "Declarative Role", OUID: "ou1"},
		},
		TotalResults: 2,
	}

	suite.mockService.On("GetRoleList", suite.ctx, serverconst.MaxPageSize, 0).Return(
		roleList, nil,
	)
	suite.mockService.On("GetRoleList", suite.ctx, serverconst.MaxPageSize, 2).Return(
		&RoleList{Roles: []Role{}, TotalResults: 2}, nil,
	)
	suite.mockService.On("IsRoleDeclarative", suite.ctx, "role1").Return(false, nil)
	suite.mockService.On("IsRoleDeclarative", suite.ctx, "role-declarative").Return(
		true, nil,
	)

	ids, err := suite.exporter.GetAllResourceIDs(suite.ctx)

	suite.Nil(err)
	assert.Len(suite.T(), ids, 1)
	assert.Equal(suite.T(), "role1", ids[0])
	suite.mockService.AssertExpectations(suite.T())
}

// Test GetAllResourceIDs - error on GetRoleList
func (suite *RoleExporterTestSuite) TestGetAllResourceIDs_ErrorOnGetRoleList() {
	serviceErr := &tidcommon.ServiceError{Code: "500"}
	suite.mockService.On("GetRoleList", suite.ctx, serverconst.MaxPageSize, 0).Return(nil, serviceErr)

	ids, err := suite.exporter.GetAllResourceIDs(suite.ctx)

	suite.NotNil(err)
	assert.Nil(suite.T(), ids)
	assert.Equal(suite.T(), serviceErr, err)
}

// Test GetAllResourceIDs - error on IsRoleDeclarative
func (suite *RoleExporterTestSuite) TestGetAllResourceIDs_ErrorOnIsRoleDeclarative() {
	roleList := &RoleList{
		Roles: []Role{
			{ID: "role1", Name: "Admin", OUID: "ou1"},
		},
		TotalResults: 1,
	}
	serviceErr := &tidcommon.ServiceError{Code: "500"}

	suite.mockService.On("GetRoleList", suite.ctx, serverconst.MaxPageSize, 0).Return(roleList, nil)
	suite.mockService.On("IsRoleDeclarative", suite.ctx, "role1").Return(false, serviceErr)

	ids, err := suite.exporter.GetAllResourceIDs(suite.ctx)

	suite.NotNil(err)
	assert.Nil(suite.T(), ids)
	assert.Equal(suite.T(), serviceErr, err)
}

// Test GetResourceByID - success
func (suite *RoleExporterTestSuite) TestGetResourceByID_Success() {
	roleWithPerms := &RoleWithPermissions{
		ID:          "role1",
		Name:        "Admin",
		Description: "Admin role",
		OUID:        "ou1",
		Permissions: []ResourcePermissions{
			{ResourceServerID: "rs1", Permissions: []string{"read", "write"}},
		},
	}

	suite.mockService.On("GetRoleWithPermissions", suite.ctx, "role1").Return(
		roleWithPerms, nil,
	)
	suite.mockAssignmentService.On(
		"GetRoleAssignments", suite.ctx, "role1", serverconst.MaxPageSize, 0, false,
	).Return(&AssignmentList{
		Assignments: []RoleAssignmentWithDisplay{
			{ID: "user1", Type: assigneeTypeEntity},
			{ID: "group1", Type: AssigneeTypeGroup},
		},
		TotalResults: 2,
	}, nil)
	suite.mockAssignmentService.On(
		"GetRoleAssignments", suite.ctx, "role1", serverconst.MaxPageSize, 2, false,
	).Return(&AssignmentList{
		Assignments:  []RoleAssignmentWithDisplay{},
		TotalResults: 2,
	}, nil)

	resource, name, err := suite.exporter.GetResourceByID(suite.ctx, "role1")

	suite.Nil(err)
	assert.Equal(suite.T(), "Admin", name)
	assert.NotNil(suite.T(), resource)

	role, ok := resource.(*roleDeclarativeResource)
	assert.True(suite.T(), ok)
	assert.Equal(suite.T(), "role1", role.ID)
	assert.Len(suite.T(), role.Assignments, 2)
}

// exportingPolicies rebuilds the exporter around a sharing service that answers the role's policies.
func (suite *RoleExporterTestSuite) exportingPolicies(
	policies []sharing.ReplayablePolicy, svcErr *tidcommon.ServiceError) {
	sharingService := sharingmock.NewSharingServiceInterfaceMock(suite.T())
	sharingService.EXPECT().ExportPolicies(mock.Anything, RoleSharingType, "role1").Return(policies, svcErr)
	suite.exporter = newRoleExporter(suite.mockService, suite.mockAssignmentService, sharingService)
	suite.mockService.On("GetRoleWithPermissions", suite.ctx, "role1").
		Return(&RoleWithPermissions{ID: "role1", Name: "Admin", OUID: "ou1"}, nil)
}

// The document carries the role's sharing policies, so it re-imports as a role shared as it was.
func (suite *RoleExporterTestSuite) TestGetResourceByID_ExportsTheSharingPolicies() {
	request := sharing.PolicyRequest{
		ID:         "role1-policy-ou1",
		ChildUnits: &sharing.ChildUnitsRequest{Mode: sharing.ChildUnitsAll},
	}
	suite.exportingPolicies([]sharing.ReplayablePolicy{{InitiatingOUID: "ou1", Request: request}}, nil)
	suite.mockAssignmentService.On(
		"GetRoleAssignments", suite.ctx, "role1", serverconst.MaxPageSize, 0, false,
	).Return(&AssignmentList{Assignments: []RoleAssignmentWithDisplay{}}, nil)

	resource, _, err := suite.exporter.GetResourceByID(suite.ctx, "role1")

	suite.Require().Nil(err)
	role, ok := resource.(*roleDeclarativeResource)
	suite.Require().True(ok)
	suite.Equal([]sharing.PolicyRequest{request}, role.SharingPolicies)
}

// A failure to read the policies fails the export rather than writing a document that silently
// shares with nobody, and it is answered in this package's own codes.
func (suite *RoleExporterTestSuite) TestGetResourceByID_SharingFailureFailsTheExport() {
	suite.exportingPolicies(nil, &tidcommon.ServiceError{Type: tidcommon.ServerErrorType, Code: "SHR-5000"})
	suite.mockAssignmentService.On(
		"GetRoleAssignments", suite.ctx, "role1", serverconst.MaxPageSize, 0, false,
	).Return(&AssignmentList{Assignments: []RoleAssignmentWithDisplay{}}, nil)

	resource, name, err := suite.exporter.GetResourceByID(suite.ctx, "role1")

	suite.Nil(resource)
	suite.Empty(name)
	suite.Require().NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

// Test GetResourceByID - error on GetRoleWithPermissions
func (suite *RoleExporterTestSuite) TestGetResourceByID_ErrorOnGetRoleWithPermissions() {
	serviceErr := &tidcommon.ServiceError{Code: "404"}
	suite.mockService.On("GetRoleWithPermissions", suite.ctx, "nonexistent").Return(nil, serviceErr)

	resource, name, err := suite.exporter.GetResourceByID(suite.ctx, "nonexistent")

	suite.NotNil(err)
	assert.Nil(suite.T(), resource)
	assert.Empty(suite.T(), name)
	assert.Equal(suite.T(), serviceErr, err)
}

// Test ValidateResource - success
func (suite *RoleExporterTestSuite) TestValidateResource_Success() {
	resource := &roleDeclarativeResource{
		ID:   "role1",
		Name: "Admin",
		OUID: "ou1",
	}
	logger := log.GetLogger()

	name, exportErr := suite.exporter.ValidateResource(context.Background(), resource, "role1", logger)

	suite.Nil(exportErr)
	assert.Equal(suite.T(), "Admin", name)
}

// Test ValidateResource - invalid type
func (suite *RoleExporterTestSuite) TestValidateResource_InvalidType() {
	logger := log.GetLogger()

	name, exportErr := suite.exporter.ValidateResource(context.Background(), "not a role", "role1", logger)

	suite.NotNil(exportErr)
	assert.Empty(suite.T(), name)
}

// Test parseToRole - valid YAML
func (suite *RoleExporterTestSuite) TestParseToRole_ValidYAML() {
	yamlData := []byte(`
id: role1
name: Admin
description: Admin role
ouId: ou1
permissions:
  - resourceServerId: rs1
    permissions:
      - read
      - write
assignments:
  - id: user1
    type: user
`)

	role, err := parseToRole(yamlData)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), role)
	assert.Equal(suite.T(), "role1", role.ID)
	assert.Equal(suite.T(), "Admin", role.Name)
	assert.Equal(suite.T(), "Admin role", role.Description)
	assert.Equal(suite.T(), "ou1", role.OUID)
	assert.Len(suite.T(), role.Permissions, 1)
	assert.Len(suite.T(), role.Assignments, 1)
}

// Test parseToRole - invalid YAML
func (suite *RoleExporterTestSuite) TestParseToRole_InvalidYAML() {
	yamlData := []byte(`
invalid: yaml: content:
`)

	role, err := parseToRole(yamlData)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), role)
}

// Test parseToRole - optional fields omitted
func (suite *RoleExporterTestSuite) TestParseToRole_OptionalFieldsOmitted() {
	yamlData := []byte(`
id: role1
name: Admin
ouId: ou1
`)

	role, err := parseToRole(yamlData)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), role)
	assert.Empty(suite.T(), role.Description)
	assert.Empty(suite.T(), role.Assignments)
	assert.Empty(suite.T(), role.Permissions)
}

// Test validateRoleWrapper - valid role
func (suite *RoleExporterTestSuite) TestValidateRoleWrapper_ValidRole() {
	role := &RoleWithPermissionsAndAssignments{
		ID:   "role1",
		Name: "Admin",
		OUID: "ou1",
		Permissions: []ResourcePermissions{
			{ResourceServerID: "rs1", Permissions: []string{"read"}},
		},
		Assignments: []RoleAssignment{
			{ID: "user1", Type: assigneeTypeEntity},
		},
	}

	// Pass nil for fileStore to skip duplicate check (for unit test purposes)
	err := validateRoleWrapper(role, nil, nil, nil)

	assert.NoError(suite.T(), err)
}

// Test validateRoleWrapper - missing ID
func (suite *RoleExporterTestSuite) TestValidateRoleWrapper_MissingID() {
	role := &RoleWithPermissionsAndAssignments{
		Name: "Admin",
		OUID: "ou1",
	}

	err := validateRoleWrapper(role, nil, nil, nil)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "role ID is required")
}

// Test validateRoleWrapper - missing name
func (suite *RoleExporterTestSuite) TestValidateRoleWrapper_MissingName() {
	role := &RoleWithPermissionsAndAssignments{
		ID:   "role1",
		OUID: "ou1",
	}

	err := validateRoleWrapper(role, nil, nil, nil)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "role name is required")
}

// Test validateRoleWrapper - missing organization unit ID
func (suite *RoleExporterTestSuite) TestValidateRoleWrapper_MissingOUID() {
	role := &RoleWithPermissionsAndAssignments{
		ID:   "role1",
		Name: "Admin",
	}

	err := validateRoleWrapper(role, nil, nil, nil)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "ouId or ouHandle is required for role 'Admin'")
}

// Test validateRoleWrapper - invalid assignment type
func (suite *RoleExporterTestSuite) TestValidateRoleWrapper_InvalidAssignmentType() {
	role := &RoleWithPermissionsAndAssignments{
		ID:   "role1",
		Name: "Admin",
		OUID: "ou1",
		Assignments: []RoleAssignment{
			{ID: "user1", Type: "invalid"},
		},
	}

	err := validateRoleWrapper(role, nil, nil, nil)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "invalid assignment type")
}

// Test validateRoleWrapper - missing assignment ID
func (suite *RoleExporterTestSuite) TestValidateRoleWrapper_MissingAssignmentID() {
	role := &RoleWithPermissionsAndAssignments{
		ID:   "role1",
		Name: "Admin",
		OUID: "ou1",
		Assignments: []RoleAssignment{
			{Type: assigneeTypeEntity},
		},
	}

	err := validateRoleWrapper(role, nil, nil, nil)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "assignment ID is required")
}

// Test validateRoleWrapper - missing resource server ID
func (suite *RoleExporterTestSuite) TestValidateRoleWrapper_MissingResourceServerID() {
	role := &RoleWithPermissionsAndAssignments{
		ID:   "role1",
		Name: "Admin",
		OUID: "ou1",
		Permissions: []ResourcePermissions{
			{Permissions: []string{"read"}},
		},
	}

	err := validateRoleWrapper(role, nil, nil, nil)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "resource server ID is required")
}

// Test toResourcePermissions
func (suite *RoleExporterTestSuite) TestToResourcePermissions() {
	perm := roleDeclarativePermission{
		ResourceServerID: "rs1",
		Permissions:      []string{"read", "write"},
	}

	result := toResourcePermissions(perm)

	assert.Equal(suite.T(), "rs1", result.ResourceServerID)
	assert.Len(suite.T(), result.Permissions, 2)
	assert.Contains(suite.T(), result.Permissions, "read")
	assert.Contains(suite.T(), result.Permissions, "write")
}

// Test parseToRoleWrapper
func (suite *RoleExporterTestSuite) TestParseToRoleWrapper() {
	yamlData := []byte(`
id: role1
name: Admin
ouId: ou1
`)

	result, err := parseToRoleWrapper(yamlData)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)

	role, ok := result.(*RoleWithPermissionsAndAssignments)
	assert.True(suite.T(), ok)
	assert.Equal(suite.T(), "role1", role.ID)
}

// Sharing

func (suite *RoleExporterTestSuite) TestMakeRoleSharingConfig() {
	cfg := makeRoleSharingConfig(suite.mockService)

	suite.Equal(RoleSharingType, cfg.ResourceType)
	suite.Equal("roles", cfg.DirectoryName)
	suite.NotNil(cfg.Parser)
}

func (suite *RoleExporterTestSuite) TestRoleSharingParser_DocumentWithoutPolicies() {
	declared, err := makeRoleSharingParser(suite.mockService)([]byte(`
id: role1
name: Support Agent
ouId: ou-owner
permissions: []
`))

	suite.Require().NoError(err)
	suite.Equal("role1", declared.ResourceID)
	suite.Empty(declared.Policies)
}

func (suite *RoleExporterTestSuite) TestRoleSharingParser_ResolvesTheOwnerByHandle() {
	suite.mockService.EXPECT().ResolveRoleOUHandle(context.Background(),
		&RoleWithPermissionsAndAssignments{ID: "role1", Name: "Support Agent", OUHandle: "acme"}).
		RunAndReturn(func(_ context.Context, role *RoleWithPermissionsAndAssignments) *tidcommon.ServiceError {
			role.OUID = "ou-owner"
			return nil
		})

	declared, err := makeRoleSharingParser(suite.mockService)([]byte(`
id: role1
name: Support Agent
ouHandle: acme
permissions: []
sharingPolicies:
  - id: role1-policy-owner
    childUnits:
      mode: all
      excludedOuIds: [ ou-sandbox ]
`))

	suite.Require().NoError(err)
	suite.Equal("role1", declared.ResourceID)
	suite.Equal("Support Agent", declared.ResourceName)
	suite.Equal("ou-owner", declared.OwningOUID)
	suite.Require().Len(declared.Policies, 1)
	policy := declared.Policies[0]
	suite.Equal("role1-policy-owner", policy.ID)
	suite.Require().NotNil(policy.ChildUnits)
	suite.Equal(sharing.ChildUnitsAll, policy.ChildUnits.Mode)
	suite.Equal([]string{"ou-sandbox"}, policy.ChildUnits.ExcludedOUIDs)
}

func (suite *RoleExporterTestSuite) TestRoleSharingParser_Errors() {
	parser := makeRoleSharingParser(suite.mockService)

	_, err := parser([]byte("id: [unterminated"))
	suite.ErrorContains(err, "failed to parse role YAML")

	suite.mockService.EXPECT().ResolveRoleOUHandle(context.Background(),
		&RoleWithPermissionsAndAssignments{ID: "role1", Name: "Support Agent", OUHandle: "nowhere"}).
		Return(&ErrorInvalidRequestFormat)
	_, err = parser([]byte(`
id: role1
name: Support Agent
ouHandle: nowhere
permissions: []
sharingPolicies:
  - id: role1-policy-owner
    childUnits: { mode: all }
`))
	suite.ErrorContains(err, `organization unit with handle "nowhere" not found`)
}
