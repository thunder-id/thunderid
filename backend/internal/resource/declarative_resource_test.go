// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"context"
	"errors"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/log"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"
)

func TestProcessResourceServerAuthorizationEngine(t *testing.T) {
	for _, engineType := range []string{"", providers.AuthorizationEngineTypeRBAC} {
		t.Run("default_"+engineType, func(t *testing.T) {
			rs := &providers.ResourceServer{AuthorizationEngine: providers.AuthorizationEngineConfig{
				Type:       engineType,
				Properties: providers.AuthorizationEngineProperties{PDPConnectionID: "old-pdp"},
			}}
			assert.NoError(t, ProcessResourceServer(rs))
			assert.Equal(t, providers.AuthorizationEngineTypeRBAC, rs.AuthorizationEngine.Type)
			assert.Empty(t, rs.AuthorizationEngine.Properties.PDPConnectionID)
		})
	}
	rs := &providers.ResourceServer{AuthorizationEngine: providers.AuthorizationEngineConfig{Type: "invalid"}}
	assert.Error(t, ProcessResourceServer(rs))

	t.Run("authzen_pdp requires connection ID", func(t *testing.T) {
		rs := &providers.ResourceServer{AuthorizationEngine: providers.AuthorizationEngineConfig{
			Type: providers.AuthorizationEngineTypeAuthZENPDP,
		}}
		assert.Error(t, ProcessResourceServer(rs))
	})

	t.Run("authzen_pdp trims connection ID", func(t *testing.T) {
		rs := &providers.ResourceServer{AuthorizationEngine: providers.AuthorizationEngineConfig{
			Type: providers.AuthorizationEngineTypeAuthZENPDP,
			Properties: providers.AuthorizationEngineProperties{
				PDPConnectionID: " pdp-1 ",
			},
		}}
		assert.NoError(t, ProcessResourceServer(rs))
		assert.Equal(t, "pdp-1", rs.AuthorizationEngine.Properties.PDPConnectionID)
	})
}

// ResourceServerExporterTestSuite tests the resourceServerExporter.
type ResourceServerExporterTestSuite struct {
	suite.Suite
	mockService *ResourceServiceInterfaceMock
	exporter    *resourceServerExporter
	logger      *log.Logger
}

func TestResourceServerExporterTestSuite(t *testing.T) {
	suite.Run(t, new(ResourceServerExporterTestSuite))
}

func (s *ResourceServerExporterTestSuite) SetupTest() {
	s.mockService = NewResourceServiceInterfaceMock(s.T())
	s.exporter = newResourceServerExporterForTest(s.mockService)
	s.logger = log.GetLogger()
}

func (s *ResourceServerExporterTestSuite) TestNewResourceServerExporter() {
	assert.NotNil(s.T(), s.exporter)
}

func (s *ResourceServerExporterTestSuite) TestGetResourceType() {
	assert.Equal(s.T(), "resource_server", s.exporter.GetResourceType())
}

func (s *ResourceServerExporterTestSuite) TestGetParameterizerType() {
	assert.Equal(s.T(), "ResourceServer", s.exporter.GetParameterizerType())
}

func (s *ResourceServerExporterTestSuite) TestGetAllResourceIDs_Success() {
	ctx := context.Background()
	expectedList := &ResourceServerList{
		TotalResults: 2,
		ResourceServers: []providers.ResourceServer{
			{ID: "rs1", Name: "Resource Server 1"},
			{ID: "rs2", Name: "Resource Server 2"},
		},
	}

	s.mockService.EXPECT().GetResourceServerList(ctx, serverconst.MaxPageSize, 0).Return(expectedList, nil)
	s.mockService.EXPECT().IsResourceServerDeclarative("rs1").Return(false)
	s.mockService.EXPECT().IsResourceServerDeclarative("rs2").Return(false)

	ids, err := s.exporter.GetAllResourceIDs(ctx)

	assert.Nil(s.T(), err)
	assert.Len(s.T(), ids, 2)
	assert.Equal(s.T(), "rs1", ids[0])
	assert.Equal(s.T(), "rs2", ids[1])
}

func (s *ResourceServerExporterTestSuite) TestGetAllResourceIDs_FilterDeclarative() {
	ctx := context.Background()
	expectedList := &ResourceServerList{
		TotalResults: 3,
		ResourceServers: []providers.ResourceServer{
			{ID: "rs1", Name: "Mutable Server"},
			{ID: "rs2", Name: "Declarative Server"},
			{ID: "rs3", Name: "Another Mutable Server"},
		},
	}

	s.mockService.EXPECT().GetResourceServerList(ctx, serverconst.MaxPageSize, 0).Return(expectedList, nil)
	s.mockService.EXPECT().IsResourceServerDeclarative("rs1").Return(false)
	s.mockService.EXPECT().IsResourceServerDeclarative("rs2").Return(true)
	s.mockService.EXPECT().IsResourceServerDeclarative("rs3").Return(false)

	ids, err := s.exporter.GetAllResourceIDs(ctx)

	assert.Nil(s.T(), err)
	assert.Len(s.T(), ids, 2)
	assert.Equal(s.T(), "rs1", ids[0])
	assert.Equal(s.T(), "rs3", ids[1])
}

func (s *ResourceServerExporterTestSuite) TestGetAllResourceIDs_Error() {
	ctx := context.Background()
	expectedError := &tidcommon.ServiceError{
		Code:  "ERR_CODE",
		Error: tidcommon.I18nMessage{DefaultValue: "test error"},
	}

	s.mockService.EXPECT().GetResourceServerList(ctx, serverconst.MaxPageSize, 0).Return(nil, expectedError)

	ids, err := s.exporter.GetAllResourceIDs(ctx)

	assert.Nil(s.T(), ids)
	assert.Equal(s.T(), expectedError, err)
}

func (s *ResourceServerExporterTestSuite) TestGetResourceByID_Success() {
	ctx := context.Background()
	serverID := "rs1"

	server := &providers.ResourceServer{
		ID:          serverID,
		Name:        "Test Server",
		Description: "A test server",
		Identifier:  "test-server",
		OUID:        "ou1",
		Delimiter:   ":",
		AuthorizationEngine: providers.AuthorizationEngineConfig{
			Type: providers.AuthorizationEngineTypeAuthZENPDP,
			Properties: providers.AuthorizationEngineProperties{
				PDPConnectionID: "pdp-1",
			},
		},
	}

	resources := []providers.Resource{
		{
			ID:           "res1",
			Name:         "Resource 1",
			Handle:       "resource1",
			Description:  "First resource",
			Parent:       nil,
			ParentHandle: "",
			Permission:   "test-server:resource1",
		},
	}

	actions := &ActionList{
		TotalResults: 1,
		Actions: []providers.Action{
			{
				ID:          "act1",
				Name:        "Action 1",
				Handle:      "read",
				Description: "Read action",
				Permission:  "test-server:resource1:read",
				Kind:        providers.ActionKindTool,
			},
		},
	}

	resourceID := "res1"
	s.mockService.EXPECT().GetResourceServer(ctx, serverID).Return(server, nil)
	s.mockService.EXPECT().GetAllResourceList(ctx, serverID).Return(resources, nil)
	s.mockService.EXPECT().
		GetActionList(ctx, serverID, &resourceID, providers.ActionKind(""), serverconst.MaxPageSize, 0).
		Return(actions, nil)

	result, name, err := s.exporter.GetResourceByID(ctx, serverID)

	assert.Nil(s.T(), err)
	assert.Equal(s.T(), "Test Server", name)
	assert.NotNil(s.T(), result)

	dto, ok := result.(*providers.ResourceServer)
	assert.True(s.T(), ok)
	assert.Equal(s.T(), serverID, dto.ID)
	assert.Equal(s.T(), "Test Server", dto.Name)
	assert.Equal(s.T(), server.AuthorizationEngine, dto.AuthorizationEngine)
	assert.Len(s.T(), dto.Resources, 1)
	assert.Len(s.T(), dto.Resources[0].Actions, 1)
	assert.Equal(s.T(), providers.ActionKindTool, dto.Resources[0].Actions[0].Kind)

	yamlBytes, marshalErr := yaml.Marshal(dto)
	assert.NoError(s.T(), marshalErr)
	assert.Contains(s.T(), string(yamlBytes), "type: authzen_pdp")
	assert.Contains(s.T(), string(yamlBytes), "pdpConnectionId: pdp-1")

	imported, parseErr := parseToResourceServer(yamlBytes)
	s.Require().NoError(parseErr)
	s.Require().NotNil(imported)
	assert.Equal(s.T(), server.AuthorizationEngine, imported.AuthorizationEngine)
}

func (s *ResourceServerExporterTestSuite) TestGetResourceByID_MCPExportImportRoundTrip() {
	ctx := context.Background()
	serverID := "rs-mcp"

	server := &providers.ResourceServer{
		ID:         serverID,
		Name:       "Booking MCP",
		Identifier: "booking-mcp",
		Type:       providers.ResourceServerTypeMCP,
		OUID:       "ou1",
		Delimiter:  ":",
	}

	resources := []providers.Resource{
		{
			ID:         "res1",
			Name:       "User Management",
			Handle:     "user-mgmt",
			Parent:     nil,
			Permission: "booking-mcp:user-mgmt",
		},
	}

	actions := &ActionList{
		TotalResults: 1,
		Actions: []providers.Action{
			{
				ID:         "act1",
				Name:       "Create User",
				Handle:     "create_user",
				Permission: "booking-mcp:user-mgmt:create_user",
				Kind:       providers.ActionKindTool,
			},
		},
	}

	resourceID := "res1"
	s.mockService.EXPECT().GetResourceServer(ctx, serverID).Return(server, nil)
	s.mockService.EXPECT().GetAllResourceList(ctx, serverID).Return(resources, nil)
	s.mockService.EXPECT().
		GetActionList(ctx, serverID, &resourceID, providers.ActionKind(""), serverconst.MaxPageSize, 0).
		Return(actions, nil)

	result, _, err := s.exporter.GetResourceByID(ctx, serverID)
	assert.Nil(s.T(), err)

	dto, ok := result.(*providers.ResourceServer)
	assert.True(s.T(), ok)

	// Marshal the exported DTO to YAML, then re-import it. This guards the export->import
	// lossless guarantee: type must survive so the kind-vs-type import validation
	// accepts the nested action carrying a kind.
	yamlBytes, marshalErr := yaml.Marshal(dto)
	assert.NoError(s.T(), marshalErr)
	assert.NotContains(s.T(), string(yamlBytes), "authorizationEngine:")

	imported, parseErr := parseToResourceServer(yamlBytes)
	s.Require().NoError(parseErr)
	s.Require().NotNil(imported)
	assert.Equal(s.T(), providers.ResourceServerTypeMCP, imported.Type)
	assert.Len(s.T(), imported.Resources, 1)
	assert.Len(s.T(), imported.Resources[0].Actions, 1)
	assert.Equal(s.T(), providers.ActionKindTool, imported.Resources[0].Actions[0].Kind)
}

func (s *ResourceServerExporterTestSuite) TestGetResourceByID_SharedHandleExportsParentPath() {
	ctx := context.Background()
	serverID := "rs1"

	server := &providers.ResourceServer{ID: serverID, Name: "Repro", Identifier: "urn:example:repro", Delimiter: ":"}
	appID, projectID, teamID := "res-app", "res-project", "res-team"
	projectMemberID, teamMemberID, leadID := "res-project-member", "res-team-member", "res-lead"
	resources := []providers.Resource{
		{ID: appID, Name: "App", Handle: "app", Permission: "app"},
		{ID: projectID, Name: "Project", Handle: "project", Parent: &appID, Permission: "app:project"},
		{ID: projectMemberID, Name: "Member", Handle: "member", Parent: &projectID, Permission: "app:project:member"},
		{ID: teamID, Name: "Team", Handle: "team", Parent: &appID, Permission: "app:team"},
		{ID: teamMemberID, Name: "Member", Handle: "member", Parent: &teamID, Permission: "app:team:member"},
		{ID: leadID, Name: "Lead", Handle: "lead", Parent: &teamMemberID, Permission: "app:team:member:lead"},
	}

	s.mockService.EXPECT().GetResourceServer(ctx, serverID).Return(server, nil)
	s.mockService.EXPECT().GetAllResourceList(ctx, serverID).Return(resources, nil)
	s.mockService.EXPECT().
		GetActionList(ctx, serverID, mock.Anything, providers.ActionKind(""), serverconst.MaxPageSize, 0).
		Return(&ActionList{}, nil)

	result, _, err := s.exporter.GetResourceByID(ctx, serverID)

	s.Require().Nil(err)
	dto, ok := result.(*providers.ResourceServer)
	s.Require().True(ok)
	s.Require().Len(dto.Resources, 6)
	assert.Equal(s.T(), "app", dto.Resources[1].ParentHandle)
	assert.Equal(s.T(), "project", dto.Resources[2].ParentHandle)
	assert.Equal(s.T(), "team", dto.Resources[4].ParentHandle)
	assert.Equal(s.T(), "app:team:member", dto.Resources[5].ParentHandle)

	yamlBytes, marshalErr := yaml.Marshal(dto)
	s.Require().NoError(marshalErr)
	imported, parseErr := parseToResourceServer(yamlBytes)
	s.Require().NoError(parseErr)
	s.Require().NoError(ProcessResourceServer(imported))
	assert.Equal(s.T(), "app:project:member", imported.Resources[2].Permission)
	assert.Equal(s.T(), "app:team:member:lead", imported.Resources[5].Permission)
}

func (s *ResourceServerExporterTestSuite) TestGetResourceByID_ServerNotFound() {
	ctx := context.Background()
	serverID := "rs-nonexistent"

	expectedError := &ErrorResourceServerNotFound

	s.mockService.EXPECT().GetResourceServer(ctx, serverID).Return(nil, expectedError)

	result, name, err := s.exporter.GetResourceByID(ctx, serverID)

	assert.Nil(s.T(), result)
	assert.Equal(s.T(), "", name)
	assert.Equal(s.T(), expectedError, err)
}

func (s *ResourceServerExporterTestSuite) TestValidateResource_Success() {
	dto := &providers.ResourceServer{
		ID:          "rs1",
		Name:        "Test Server",
		Description: "A test server",
		OUID:        "ou1",
	}

	name, err := s.exporter.ValidateResource(context.Background(), dto, "rs1", s.logger)

	assert.Nil(s.T(), err)
	assert.Equal(s.T(), "Test Server", name)
}

func (s *ResourceServerExporterTestSuite) TestValidateResource_InvalidType() {
	invalidData := "not a resource server dto"

	name, err := s.exporter.ValidateResource(context.Background(), invalidData, "rs1", s.logger)

	assert.Equal(s.T(), "", name)
	assert.NotNil(s.T(), err)
}

func (s *ResourceServerExporterTestSuite) TestValidateResource_EmptyName() {
	dto := &providers.ResourceServer{
		ID:   "rs1",
		Name: "",
		OUID: "ou1",
	}

	name, err := s.exporter.ValidateResource(context.Background(), dto, "rs1", s.logger)

	assert.Equal(s.T(), "", name)
	assert.NotNil(s.T(), err)
}

func (s *ResourceServerExporterTestSuite) TestGetResourceRules() {
	rules := s.exporter.GetResourceRules()

	assert.Nil(s.T(), rules)
}

func TestParseToResourceServer(t *testing.T) {
	yamlData := []byte(`
id: "rs1"
name: "Test Server"
description: "Test description"
identifier: "test-server"
ouId: "ou1"
delimiter: ":"
resources:
  - name: "Users"
    handle: "users"
    description: "User resources"
    actions:
      - name: "Read"
        handle: "read"
`)

	dto, err := parseToResourceServer(yamlData)

	assert.NoError(t, err)
	assert.NotNil(t, dto)
	assert.Equal(t, "rs1", dto.ID)
	assert.Equal(t, "Test Server", dto.Name)
	assert.Equal(t, "test-server", dto.Identifier)
	assert.Equal(t, "ou1", dto.OUID)
	assert.Equal(t, ":", dto.Delimiter)
	assert.Len(t, dto.Resources, 1)
	assert.Equal(t, "users", dto.Resources[0].Handle)
	assert.Len(t, dto.Resources[0].Actions, 1)
}

func TestParseToResourceServer_MissingID(t *testing.T) {
	yamlData := []byte(`
name: "Test Server"
ouId: "ou1"
`)

	dto, err := parseToResourceServer(yamlData)

	assert.Error(t, err)
	assert.Nil(t, dto)
	assert.Contains(t, err.Error(), "ID cannot be empty")
}

func TestParseToResourceServer_MissingName(t *testing.T) {
	yamlData := []byte(`
id: "rs1"
ouId: "ou1"
`)

	dto, err := parseToResourceServer(yamlData)

	assert.Error(t, err)
	assert.Nil(t, dto)
	assert.Contains(t, err.Error(), "name cannot be empty")
}

func TestParseToResourceServer_TypeMCP(t *testing.T) {
	yamlData := []byte(`
id: "rs1"
name: "Test Server"
type: "MCP"
ouId: "ou1"
`)

	dto, err := parseToResourceServer(yamlData)

	assert.NoError(t, err)
	assert.NotNil(t, dto)
	assert.Equal(t, providers.ResourceServerTypeMCP, dto.Type)
}

func TestParseToResourceServer_TypeDefaultsToCustom(t *testing.T) {
	yamlData := []byte(`
id: "rs1"
name: "Test Server"
ouId: "ou1"
`)

	dto, err := parseToResourceServer(yamlData)

	assert.NoError(t, err)
	assert.NotNil(t, dto)
	assert.Equal(t, providers.ResourceServerTypeCustom, dto.Type)
}

func TestParseToResourceServer_InvalidType(t *testing.T) {
	yamlData := []byte(`
id: "rs1"
name: "Test Server"
type: "BOGUS"
ouId: "ou1"
`)

	dto, err := parseToResourceServer(yamlData)

	assert.Error(t, err)
	assert.Nil(t, dto)
	assert.Contains(t, err.Error(), "invalid type")
}

func TestParseToResourceServer_MCPActionDefaultsKindToTool(t *testing.T) {
	yamlData := []byte(`
id: "rs1"
name: "Test Server"
type: "MCP"
ouId: "ou1"
resources:
  - name: "Users"
    handle: "users"
    actions:
      - name: "Read"
        handle: "read"
`)

	dto, err := parseToResourceServer(yamlData)

	assert.NoError(t, err)
	assert.NotNil(t, dto)
	assert.Equal(t, providers.ActionKindTool, dto.Resources[0].Actions[0].Kind)
}

func TestParseToResourceServer_MCPActionWithKindSucceeds(t *testing.T) {
	yamlData := []byte(`
id: "rs1"
name: "Test Server"
type: "MCP"
ouId: "ou1"
resources:
  - name: "Users"
    handle: "users"
    actions:
      - name: "Read"
        handle: "read"
        kind: "resource"
      - name: "Create"
        handle: "create"
        kind: "tool"
`)

	dto, err := parseToResourceServer(yamlData)

	assert.NoError(t, err)
	assert.NotNil(t, dto)
	assert.Equal(t, providers.ActionKindResource, dto.Resources[0].Actions[0].Kind)
	assert.Equal(t, providers.ActionKindTool, dto.Resources[0].Actions[1].Kind)
}

func TestParseToResourceServer_NonMCPActionAllowsKind(t *testing.T) {
	yamlData := []byte(`
id: "rs1"
name: "Test Server"
type: "API"
ouId: "ou1"
resources:
  - name: "Users"
    handle: "users"
    actions:
      - name: "Read"
        handle: "read"
        kind: "tool"
`)

	dto, err := parseToResourceServer(yamlData)

	assert.NoError(t, err)
	assert.NotNil(t, dto)
	assert.Equal(t, providers.ActionKindTool, dto.Resources[0].Actions[0].Kind)
}

func TestParseToResourceServer_NonMCPActionNoKindStaysEmpty(t *testing.T) {
	yamlData := []byte(`
id: "rs1"
name: "Test Server"
type: "CUSTOM"
ouId: "ou1"
resources:
  - name: "Users"
    handle: "users"
    actions:
      - name: "Read"
        handle: "read"
`)

	dto, err := parseToResourceServer(yamlData)

	assert.NoError(t, err)
	assert.NotNil(t, dto)
	assert.Equal(t, providers.ActionKind(""), dto.Resources[0].Actions[0].Kind)
}

func TestParseToResourceServer_ActionInvalidKindRejected(t *testing.T) {
	yamlData := []byte(`
id: "rs1"
name: "Test Server"
type: "MCP"
ouId: "ou1"
resources:
  - name: "Users"
    handle: "users"
    actions:
      - name: "Read"
        handle: "read"
        kind: "prompt"
`)

	dto, err := parseToResourceServer(yamlData)

	assert.Error(t, err)
	assert.Nil(t, dto)
	assert.Contains(t, err.Error(), "read")
	assert.Contains(t, err.Error(), "invalid kind")
}

func TestParseAndValidateResourceServerWrapper_TypeMCP(t *testing.T) {
	yamlData := []byte(`
id: "rs1"
name: "Test Server"
handle: "test-api"
type: "MCP"
ouId: "ou1"
`)

	parser := parseAndValidateResourceServerWrapper(nil)
	result, err := parser(yamlData)

	assert.NoError(t, err)
	rs, ok := result.(*providers.ResourceServer)
	assert.True(t, ok)
	assert.Equal(t, providers.ResourceServerTypeMCP, rs.Type)
}

func TestParseAndValidateResourceServerWrapper_InvalidType(t *testing.T) {
	yamlData := []byte(`
id: "rs1"
name: "Test Server"
handle: "test-api"
type: "BOGUS"
ouId: "ou1"
`)

	parser := parseAndValidateResourceServerWrapper(nil)
	result, err := parser(yamlData)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "invalid type")
}

func TestResolveResourcePaths(t *testing.T) {
	resources := []providers.Resource{
		{Handle: "users"},
		{Handle: "profile", ParentHandle: "users"},
		{Handle: "avatar", ParentHandle: "users:profile"},
	}

	paths, err := resolveResourcePaths(resources, ":")

	assert.NoError(t, err)
	assert.Equal(t, []string{"users", "users:profile", "users:profile:avatar"}, paths)
}

func TestProcessResourceServer_SetsPermissionsAndDelimiter(t *testing.T) {
	rs := &providers.ResourceServer{
		ID:         "rs1",
		Name:       "Test Server",
		OUID:       "ou1",
		Identifier: "api",
		Resources: []providers.Resource{
			{
				Name:   "Users",
				Handle: "users",
				Actions: []providers.Action{
					{Name: "Read", Handle: "read"},
				},
			},
			{
				Name:         "Profile",
				Handle:       "profile",
				ParentHandle: "users",
			},
		},
	}

	err := ProcessResourceServer(rs)

	assert.NoError(t, err)
	assert.Equal(t, ":", rs.Delimiter)
	assert.Equal(t, "users", rs.Resources[0].Permission)
	assert.Equal(t, "users:read", rs.Resources[0].Actions[0].Permission)
	assert.Equal(t, "users:profile", rs.Resources[1].Permission)
}

func TestProcessResourceServer_DuplicateHandle(t *testing.T) {
	rs := &providers.ResourceServer{
		ID:   "rs1",
		Name: "Test Server",
		OUID: "ou1",
		Resources: []providers.Resource{
			{Name: "Users", Handle: "users"},
			{Name: "Users Duplicate", Handle: "users"},
		},
	}

	err := ProcessResourceServer(rs)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate resource handle")
}

func TestProcessResourceServer_MCPActionCollidesWithGroupPermission(t *testing.T) {
	// An MCP resource server where a group (RESOURCE) nested under "ops" shares its handle with a
	// tool (ACTION) nested under the same "ops" group: both derive "ops:deploy".
	rs := &providers.ResourceServer{
		ID:   "rs-mcp",
		Name: "Booking MCP",
		OUID: "ou1",
		Type: providers.ResourceServerTypeMCP,
		Resources: []providers.Resource{
			{
				Name:   "Ops",
				Handle: "ops",
				Actions: []providers.Action{
					{Name: "Deploy", Handle: "deploy", Kind: providers.ActionKindTool},
				},
			},
			{
				Name:         "Deploy Group",
				Handle:       "deploy",
				ParentHandle: "ops",
			},
		},
	}

	err := ProcessResourceServer(rs)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate permission")
	assert.Contains(t, err.Error(), "ops:deploy")
}

func TestProcessResourceServer_MCPActionCollidesWithNestedGroupPermission(t *testing.T) {
	// A tool nested under group "a" derives "a:b". A child group "b" nested under "a" derives the
	// same "a:b". The cross-entity collision is caught even though the two collide via different
	// nesting paths rather than at the same level.
	rs := &providers.ResourceServer{
		ID:   "rs-mcp",
		Name: "Booking MCP",
		OUID: "ou1",
		Type: providers.ResourceServerTypeMCP,
		Resources: []providers.Resource{
			{
				Name:   "Group A",
				Handle: "a",
				Actions: []providers.Action{
					{Name: "Tool B", Handle: "b", Kind: providers.ActionKindTool},
				},
			},
			{
				Name:         "Group B",
				Handle:       "b",
				ParentHandle: "a",
			},
		},
	}

	err := ProcessResourceServer(rs)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate permission")
	assert.Contains(t, err.Error(), "a:b")
}

func TestProcessResourceServer_MCPNoCollisionSucceeds(t *testing.T) {
	rs := &providers.ResourceServer{
		ID:   "rs-mcp",
		Name: "Booking MCP",
		OUID: "ou1",
		Type: providers.ResourceServerTypeMCP,
		Resources: []providers.Resource{
			{
				Name:   "Ops",
				Handle: "ops",
				Actions: []providers.Action{
					{Name: "Deploy", Handle: "deploy", Kind: providers.ActionKindTool},
					{Name: "Status", Handle: "status", Kind: providers.ActionKindResource},
				},
			},
			{
				Name:   "Users",
				Handle: "users",
				Actions: []providers.Action{
					{Name: "Create", Handle: "create", Kind: providers.ActionKindTool},
				},
			},
		},
	}

	err := ProcessResourceServer(rs)

	assert.NoError(t, err)
	assert.Equal(t, "ops:deploy", rs.Resources[0].Actions[0].Permission)
	assert.Equal(t, "ops:status", rs.Resources[0].Actions[1].Permission)
	assert.Equal(t, "users:create", rs.Resources[1].Actions[0].Permission)
}

func TestProcessResourceServer_NonMCPSkipsPermissionCollisionCheck(t *testing.T) {
	// An API resource server with the same structure that collides for MCP must still succeed,
	// since Rule 6 (cross-entity permission collision) applies only to MCP-type resource servers.
	rs := &providers.ResourceServer{
		ID:   "rs-api",
		Name: "Booking API",
		OUID: "ou1",
		Type: providers.ResourceServerTypeAPI,
		Resources: []providers.Resource{
			{
				Name:   "Ops",
				Handle: "ops",
				Actions: []providers.Action{
					{Name: "Deploy", Handle: "deploy"},
				},
			},
			{
				Name:         "Deploy Group",
				Handle:       "deploy",
				ParentHandle: "ops",
			},
		},
	}

	err := ProcessResourceServer(rs)

	assert.NoError(t, err)
}

func TestProcessResource_SetsPermissions(t *testing.T) {
	resource := &providers.Resource{
		Handle:       "child",
		ParentHandle: "root",
		Actions: []providers.Action{
			{Name: "Read", Handle: "read"},
		},
	}

	processResource(resource, "root:child", ":")

	assert.Equal(t, "root:child", resource.Permission)
	assert.Equal(t, "root:child:read", resource.Actions[0].Permission)
}

func TestProcessResourceServer_MissingParent(t *testing.T) {
	rs := &providers.ResourceServer{
		ID: "rs1",
		Resources: []providers.Resource{
			{Name: "Child", Handle: "child", ParentHandle: "missing"},
		},
	}

	err := ProcessResourceServer(rs)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parent resource handle 'missing' not found")
}

func TestProcessResourceServer_MissingParentPath(t *testing.T) {
	rs := &providers.ResourceServer{
		ID: "rs1",
		Resources: []providers.Resource{
			{Name: "App", Handle: "app"},
			{Name: "Member", Handle: "member", ParentHandle: "app:project"},
		},
	}

	err := ProcessResourceServer(rs)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parent resource path 'app:project' not found")
}

func TestProcessResourceServer_SameHandleUnderDifferentParents(t *testing.T) {
	rs := &providers.ResourceServer{
		ID:        "rs1",
		Delimiter: ":",
		Resources: []providers.Resource{
			{Name: "App", Handle: "app"},
			{Name: "Project", Handle: "project", ParentHandle: "app"},
			{
				Name:         "Member",
				Handle:       "member",
				ParentHandle: "app:project",
				Actions:      []providers.Action{{Name: "Read", Handle: "read"}},
			},
			{Name: "Team", Handle: "team", ParentHandle: "app"},
			{
				Name:         "Member",
				Handle:       "member",
				ParentHandle: "app:team",
				Actions:      []providers.Action{{Name: "Read", Handle: "read"}},
			},
			{Name: "Lead", Handle: "lead", ParentHandle: "app:team:member"},
		},
	}

	err := ProcessResourceServer(rs)

	assert.NoError(t, err)
	assert.Equal(t, "app:project:member", rs.Resources[2].Permission)
	assert.Equal(t, "app:project:member:read", rs.Resources[2].Actions[0].Permission)
	assert.Equal(t, "app:team:member", rs.Resources[4].Permission)
	assert.Equal(t, "app:team:member:read", rs.Resources[4].Actions[0].Permission)
	assert.Equal(t, "app:team:member:lead", rs.Resources[5].Permission)
}

func TestProcessResourceServer_PathParentWithCustomDelimiter(t *testing.T) {
	rs := &providers.ResourceServer{
		ID:        "rs1",
		Delimiter: ".",
		Resources: []providers.Resource{
			{Name: "App", Handle: "app"},
			{Name: "Project", Handle: "project", ParentHandle: "app"},
			{Name: "Member", Handle: "member", ParentHandle: "app.project"},
		},
	}

	err := ProcessResourceServer(rs)

	assert.NoError(t, err)
	assert.Equal(t, "app.project.member", rs.Resources[2].Permission)
}

func TestProcessResourceServer_DuplicateHandleUnderSameParent(t *testing.T) {
	rs := &providers.ResourceServer{
		ID: "rs1",
		Resources: []providers.Resource{
			{Name: "App", Handle: "app"},
			{Name: "Member", Handle: "member", ParentHandle: "app"},
			{Name: "Member Duplicate", Handle: "member", ParentHandle: "app"},
		},
	}

	err := ProcessResourceServer(rs)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate resource handle 'member' found under the same parent")
	assert.Contains(t, err.Error(), "app:member")
}

func TestProcessResourceServer_AmbiguousBareParentHandle(t *testing.T) {
	rs := &providers.ResourceServer{
		ID: "rs1",
		Resources: []providers.Resource{
			{Name: "Project", Handle: "project"},
			{Name: "Team", Handle: "team"},
			{Name: "Member", Handle: "member", ParentHandle: "project"},
			{Name: "Member", Handle: "member", ParentHandle: "team"},
			{Name: "Lead", Handle: "lead", ParentHandle: "member"},
		},
	}

	err := ProcessResourceServer(rs)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parent resource handle 'member' is ambiguous for resource 'lead'")
}

func TestProcessResourceServer_BareParentHandlePrefersRootResource(t *testing.T) {
	rs := &providers.ResourceServer{
		ID: "rs1",
		Resources: []providers.Resource{
			{Name: "App", Handle: "app"},
			{Name: "Member", Handle: "member", ParentHandle: "app"},
			{Name: "Member", Handle: "member"},
			{Name: "Lead", Handle: "lead", ParentHandle: "member"},
		},
	}

	err := ProcessResourceServer(rs)

	assert.NoError(t, err)
	assert.Equal(t, "app:member", rs.Resources[1].Permission)
	assert.Equal(t, "member", rs.Resources[2].Permission)
	assert.Equal(t, "member:lead", rs.Resources[3].Permission)
}

func TestProcessResourceServer_CircularParentReference(t *testing.T) {
	rs := &providers.ResourceServer{
		ID: "rs1",
		Resources: []providers.Resource{
			{Name: "A", Handle: "a", ParentHandle: "b"},
			{Name: "B", Handle: "b", ParentHandle: "a"},
		},
	}

	err := ProcessResourceServer(rs)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circular parent reference detected")
}

func TestParseAndValidateResourceServerWrapper_Success(t *testing.T) {
	yamlData := []byte(`
id: "rs1"
name: "Test Server"
identifier: "api"
ouId: "ou1"
resources:
  - name: "Users"
    handle: "users"
    actions:
      - name: "Read"
        handle: "read"
  - name: "Profile"
    handle: "profile"
    parent: "users"
`)

	parser := parseAndValidateResourceServerWrapper(nil)
	result, err := parser(yamlData)

	assert.NoError(t, err)
	rs, ok := result.(*providers.ResourceServer)
	assert.True(t, ok)
	assert.Equal(t, "users", rs.Resources[0].Permission)
	assert.Equal(t, "users:read", rs.Resources[0].Actions[0].Permission)
	assert.Equal(t, "users:profile", rs.Resources[1].Permission)
}

func TestParseAndValidateResourceServerWrapper_NormalizesParentPath(t *testing.T) {
	yamlData := []byte(`
id: "rs1"
name: "Test Server"
identifier: "api"
ouId: "ou1"
resources:
  - name: "App"
    handle: "app"
  - name: "Project"
    handle: "project"
    parent: "app"
  - name: "Member"
    handle: "member"
    parent: "app:project"
`)

	parser := parseAndValidateResourceServerWrapper(nil)
	result, err := parser(yamlData)

	assert.NoError(t, err)
	rs, ok := result.(*providers.ResourceServer)
	assert.True(t, ok)
	assert.Equal(t, "app:project:member", rs.Resources[2].Permission)
	assert.Equal(t, "project", rs.Resources[2].ParentHandle)
}

func TestParseAndValidateResourceServerWrapper_DuplicateHandleUnderDifferentParentsRejected(t *testing.T) {
	yamlData := []byte(`
id: "rs1"
name: "Test Server"
identifier: "api"
ouId: "ou1"
resources:
  - name: "Project"
    handle: "project"
  - name: "Team"
    handle: "team"
  - name: "Member"
    handle: "member"
    parent: "project"
  - name: "Member"
    handle: "member"
    parent: "team"
`)

	parser := parseAndValidateResourceServerWrapper(nil)
	result, err := parser(yamlData)

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate resource handle 'member'")
	assert.Contains(t, err.Error(), "file-based resource servers require resource handles to be unique")
}

func TestParseAndValidateResourceServerWrapper_MCPPermissionCollisionRejected(t *testing.T) {
	yamlData := []byte(`
id: "rs-mcp"
name: "Booking MCP"
type: "MCP"
ouId: "ou1"
resources:
  - name: "Ops"
    handle: "ops"
    actions:
      - name: "Deploy"
        handle: "deploy"
        kind: "tool"
  - name: "Deploy Group"
    handle: "deploy"
    parent: "ops"
`)

	parser := parseAndValidateResourceServerWrapper(nil)
	result, err := parser(yamlData)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "duplicate permission")
	assert.Contains(t, err.Error(), "ops:deploy")
}

func TestParseAndValidateResourceServerWrapper_MCPCleanSucceeds(t *testing.T) {
	yamlData := []byte(`
id: "rs-mcp"
name: "Booking MCP"
type: "MCP"
ouId: "ou1"
resources:
  - name: "Ops"
    handle: "ops"
    actions:
      - name: "Deploy"
        handle: "deploy"
        kind: "tool"
      - name: "Status"
        handle: "status"
        kind: "resource"
`)

	parser := parseAndValidateResourceServerWrapper(nil)
	result, err := parser(yamlData)

	assert.NoError(t, err)
	rs, ok := result.(*providers.ResourceServer)
	assert.True(t, ok)
	assert.Equal(t, "ops:deploy", rs.Resources[0].Actions[0].Permission)
	assert.Equal(t, "ops:status", rs.Resources[0].Actions[1].Permission)
}

func TestParseAndValidateResourceServerWrapper_InvalidYAML(t *testing.T) {
	yamlData := []byte(`::invalid`)

	parser := parseAndValidateResourceServerWrapper(nil)
	result, err := parser(yamlData)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to parse")
}

func TestValidateResourceServerWrapper_InvalidType(t *testing.T) {
	err := validateResourceServerWrapper("invalid", newResourceStoreInterfaceMock(t), nil, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid type")
}

func TestValidateResourceServerWrapper_EmptyName(t *testing.T) {
	fileStore := newResourceStoreInterfaceMock(t)

	err := validateResourceServerWrapper(&providers.ResourceServer{ID: "rs1"}, fileStore, nil, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "name cannot be empty")
}

func TestValidateResourceServerWrapper_EmptyIdentifier(t *testing.T) {
	fileStore := newResourceStoreInterfaceMock(t)

	err := validateResourceServerWrapper(&providers.ResourceServer{ID: "rs1", Name: "Server"}, fileStore, nil, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "identifier cannot be empty")
}

func TestValidateResourceServerWrapper_DuplicateInFileStore(t *testing.T) {
	fileStore := newResourceStoreInterfaceMock(t)
	fileStore.On("GetResourceServer", mock.Anything, "rs1").Return(providers.ResourceServer{ID: "rs1"}, nil)

	err := validateResourceServerWrapper(
		&providers.ResourceServer{ID: "rs1", Name: "Server", Identifier: "test-server", OUID: "ou1"},
		fileStore, nil, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate resource server ID")
}

func TestValidateResourceServerWrapper_FileStoreError(t *testing.T) {
	fileStore := newResourceStoreInterfaceMock(t)
	fileStore.On("GetResourceServer", mock.Anything, "rs1").Return(providers.ResourceServer{}, errors.New("file error"))

	err := validateResourceServerWrapper(
		&providers.ResourceServer{ID: "rs1", Name: "Server", Identifier: "test-server", OUID: "ou1"},
		fileStore, nil, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to check")
}

func TestValidateResourceServerWrapper_DuplicateInDBStore(t *testing.T) {
	fileStore := newResourceStoreInterfaceMock(t)
	dbStore := newResourceStoreInterfaceMock(t)
	fileStore.On("GetResourceServer", mock.Anything, "rs1").
		Return(providers.ResourceServer{}, errResourceServerNotFound)
	dbStore.On("GetResourceServer", mock.Anything, "rs1").Return(providers.ResourceServer{ID: "rs1"}, nil)

	err := validateResourceServerWrapper(
		&providers.ResourceServer{ID: "rs1", Name: "Server", Identifier: "test-server", OUID: "ou1"},
		fileStore, dbStore, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database store")
}

func TestValidateResourceServerWrapper_DBStoreError(t *testing.T) {
	fileStore := newResourceStoreInterfaceMock(t)
	dbStore := newResourceStoreInterfaceMock(t)
	fileStore.On("GetResourceServer", mock.Anything, "rs1").
		Return(providers.ResourceServer{}, errResourceServerNotFound)
	dbStore.On("GetResourceServer", mock.Anything, "rs1").Return(providers.ResourceServer{}, errors.New("db error"))

	err := validateResourceServerWrapper(
		&providers.ResourceServer{ID: "rs1", Name: "Server", Identifier: "test-server", OUID: "ou1"},
		fileStore, dbStore, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database store")
}

func TestValidateResourceServerWrapper_Success(t *testing.T) {
	fileStore := newResourceStoreInterfaceMock(t)
	fileStore.On("GetResourceServer", mock.Anything, "rs1").
		Return(providers.ResourceServer{}, errResourceServerNotFound)

	err := validateResourceServerWrapper(
		&providers.ResourceServer{ID: "rs1", Name: "Server", Identifier: "test-server", OUID: "ou1"},
		fileStore, nil, nil)

	assert.NoError(t, err)
}

func TestLoadDeclarativeResources_CompositeFileStoreTypeError(t *testing.T) {
	fileStore := newResourceStoreInterfaceMock(t)
	dbStore := newResourceStoreInterfaceMock(t)
	compositeStore := newCompositeResourceStore(fileStore, dbStore)

	err := loadDeclarativeResources(compositeStore, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to assert fileStore")
}

func TestLoadDeclarativeResources_InvalidStoreType(t *testing.T) {
	invalidStore := newResourceStoreInterfaceMock(t)

	err := loadDeclarativeResources(invalidStore, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid store type")
}
