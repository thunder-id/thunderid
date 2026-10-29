// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"context"
	"errors"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/log"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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

	serverActions := &ActionList{
		TotalResults: 1,
		Actions: []providers.Action{
			{ID: "act0", Name: "Read All", Handle: "read_all", Description: "Read everything", Permission: "read_all"},
		},
	}

	resourceID := "res1"
	s.mockService.EXPECT().GetResourceServer(ctx, serverID).Return(server, nil)
	s.mockService.EXPECT().
		GetActionList(ctx, serverID, (*string)(nil), providers.ActionKind(""), serverconst.MaxPageSize, 0).
		Return(serverActions, nil)
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
	assert.Equal(s.T(), []providers.Action{
		{Name: "Read All", Handle: "read_all", Description: "Read everything"},
	}, dto.Actions)
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
	assert.Equal(s.T(), dto.Actions, imported.Actions)
}

// TestGetResourceByID_PagesServerActions verifies that every page of the resource server's own
// actions is exported, and that a server with no resources still exports them.
func (s *ResourceServerExporterTestSuite) TestGetResourceByID_PagesServerActions() {
	ctx := context.Background()
	serverID := testRS1ID
	server := &providers.ResourceServer{
		ID: serverID, Name: "Test Server", Identifier: "test-server", OUID: "ou1", Delimiter: ":",
	}

	s.mockService.EXPECT().GetResourceServer(ctx, serverID).Return(server, nil)
	s.mockService.EXPECT().
		GetActionList(ctx, serverID, (*string)(nil), providers.ActionKind(""), serverconst.MaxPageSize, 0).
		Return(&ActionList{TotalResults: 2, Actions: []providers.Action{{Name: "Read", Handle: "read"}}}, nil)
	s.mockService.EXPECT().
		GetActionList(ctx, serverID, (*string)(nil), providers.ActionKind(""), serverconst.MaxPageSize, 1).
		Return(&ActionList{TotalResults: 2, Actions: []providers.Action{{Name: "Write", Handle: "write"}}}, nil)
	s.mockService.EXPECT().GetAllResourceList(ctx, serverID).Return([]providers.Resource{}, nil)

	result, _, err := s.exporter.GetResourceByID(ctx, serverID)

	s.Require().Nil(err)
	dto, ok := result.(*providers.ResourceServer)
	s.Require().True(ok)
	assert.Equal(s.T(), []providers.Action{{Name: "Read", Handle: "read"}, {Name: "Write", Handle: "write"}},
		dto.Actions)
	assert.Empty(s.T(), dto.Resources)
}

func (s *ResourceServerExporterTestSuite) TestGetResourceByID_ServerActionsError() {
	ctx := context.Background()
	serverID := testRS1ID
	server := &providers.ResourceServer{ID: serverID, Name: "Test Server", Delimiter: ":"}

	s.mockService.EXPECT().GetResourceServer(ctx, serverID).Return(server, nil)
	s.mockService.EXPECT().
		GetActionList(ctx, serverID, (*string)(nil), providers.ActionKind(""), serverconst.MaxPageSize, 0).
		Return(nil, &tidcommon.InternalServerError)

	result, name, err := s.exporter.GetResourceByID(ctx, serverID)

	assert.Nil(s.T(), result)
	assert.Equal(s.T(), "", name)
	assert.Equal(s.T(), &tidcommon.InternalServerError, err)
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

	serverActions := &ActionList{
		TotalResults: 1,
		Actions: []providers.Action{
			{ID: "act0", Name: "List Bookings", Handle: "list_bookings", Permission: "list_bookings",
				Kind: providers.ActionKindResource},
		},
	}

	resourceID := "res1"
	s.mockService.EXPECT().GetResourceServer(ctx, serverID).Return(server, nil)
	s.mockService.EXPECT().
		GetActionList(ctx, serverID, (*string)(nil), providers.ActionKind(""), serverconst.MaxPageSize, 0).
		Return(serverActions, nil)
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
	s.Require().Len(imported.Actions, 1)
	assert.Equal(s.T(), "list_bookings", imported.Actions[0].Handle)
	assert.Equal(s.T(), providers.ActionKindResource, imported.Actions[0].Kind)
	assert.Len(s.T(), imported.Resources, 1)
	assert.Len(s.T(), imported.Resources[0].Actions, 1)
	assert.Equal(s.T(), providers.ActionKindTool, imported.Resources[0].Actions[0].Kind)
}

// TestGetResourceByID_ExportsParentPath verifies that a parent is exported as its full path.
func (s *ResourceServerExporterTestSuite) TestGetResourceByID_ExportsParentPath() {
	ctx := context.Background()
	serverID := "rs1"
	parent := func(id string) *string { return &id }

	server := &providers.ResourceServer{
		ID: serverID, Name: "Test Server", Identifier: "test-server", OUID: "ou1", Delimiter: ":",
	}
	resources := []providers.Resource{
		{ID: "app", Name: "App", Handle: "app", Permission: "app"},
		{ID: "project", Name: "Project", Handle: "project", Parent: parent("app"), Permission: "app:project"},
		{ID: "pm", Name: "Member", Handle: "member", Parent: parent("project"), Permission: "app:project:member"},
		{ID: "team", Name: "Team", Handle: "team", Parent: parent("app"), Permission: "app:team"},
		{ID: "tm", Name: "Member", Handle: "member", Parent: parent("team"), Permission: "app:team:member"},
		{ID: "lead", Name: "Lead", Handle: "lead", Parent: parent("tm"), Permission: "app:team:member:lead"},
	}

	s.mockService.EXPECT().GetResourceServer(ctx, serverID).Return(server, nil)
	s.mockService.EXPECT().GetAllResourceList(ctx, serverID).Return(resources, nil)
	s.mockService.EXPECT().
		GetActionList(ctx, serverID, mock.Anything, providers.ActionKind(""), serverconst.MaxPageSize, 0).
		Return(&ActionList{}, nil)

	result, _, err := s.exporter.GetResourceByID(ctx, serverID)

	assert.Nil(s.T(), err)
	dto, ok := result.(*providers.ResourceServer)
	assert.True(s.T(), ok)
	parents := make([]string, len(dto.Resources))
	for i, res := range dto.Resources {
		parents[i] = res.ParentHandle
	}
	assert.Equal(s.T(), []string{"", "app", "app:project", "app", "app:team", "app:team:member"}, parents)

	assert.NoError(s.T(), ProcessResourceServer(dto))
	for i, res := range dto.Resources {
		assert.Equal(s.T(), resources[i].Permission, res.Permission)
	}
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

	s.Require().NotNil(rules)
	assert.Equal(s.T(), []string{"Identifier"}, rules.Variables)
	assert.Empty(s.T(), rules.SecretVariables)
	assert.Empty(s.T(), rules.ArrayVariables)
	assert.Empty(s.T(), rules.DynamicPropertyFields)
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

// processResourceServerYAML parses and processes a resource server with the given resources.
func processResourceServerYAML(t *testing.T, delimiter, resources string) (*providers.ResourceServer, error) {
	t.Helper()
	data := "id: rs1\nname: Test Server\nidentifier: api\nouId: ou1\n"
	if delimiter != "" {
		data += "delimiter: \"" + delimiter + "\"\n"
	}
	rs, err := parseToResourceServer([]byte(data + "resources:\n" + resources))
	if err != nil {
		t.Fatalf("failed to parse resource server: %v", err)
	}
	return rs, ProcessResourceServer(rs)
}

// TestProcessResourceServer_ResolvesParents verifies parent resolution by bare handle and by full path.
func TestProcessResourceServer_ResolvesParents(t *testing.T) {
	tests := []struct {
		name        string
		delimiter   string
		resources   string
		permissions []string
		parents     []string
	}{
		{
			name: "handle repeated under different parents",
			resources: `
  - { name: App, handle: app }
  - { name: Project, handle: project, parent: app }
  - { name: Member, handle: member, parent: project, actions: [{ name: Read, handle: read }] }
  - { name: Team, handle: team, parent: app }
  - { name: Member, handle: member, parent: team, actions: [{ name: Read, handle: read }] }
`,
			permissions: []string{"app", "app:project", "app:project:member", "app:team", "app:team:member"},
			parents:     []string{"", "app", "app:project", "app", "app:team"},
		},
		{
			name: "full path names a parent whose handle repeats",
			resources: `
  - { name: Product A, handle: product-a }
  - { name: Org A, handle: org, parent: product-a }
  - { name: Product B, handle: product-b }
  - { name: Org B, handle: org, parent: product-b }
  - { name: Billing, handle: billing, parent: "product-b:org" }
`,
			permissions: []string{"product-a", "product-a:org", "product-b", "product-b:org", "product-b:org:billing"},
			parents:     []string{"", "product-a", "", "product-b", "product-b:org"},
		},
		{
			name: "children declared before their parents",
			resources: `
  - { name: Lead, handle: lead, parent: "app:team:member" }
  - { name: Member, handle: member, parent: "app:team" }
  - { name: Team, handle: team, parent: app }
  - { name: App, handle: app }
`,
			permissions: []string{"app:team:member:lead", "app:team:member", "app:team", "app"},
			parents:     []string{"app:team:member", "app:team", "app", ""},
		},
		{
			name:      "path uses the resource server delimiter",
			delimiter: "/",
			resources: `
  - { name: App, handle: app }
  - { name: Team, handle: team, parent: app }
  - { name: Member, handle: member, parent: "app/team" }
`,
			permissions: []string{"app", "app/team", "app/team/member"},
			parents:     []string{"", "app", "app/team"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs, err := processResourceServerYAML(t, tt.delimiter, tt.resources)

			assert.NoError(t, err)
			for i, res := range rs.Resources {
				assert.Equal(t, tt.permissions[i], res.Permission)
				assert.Equal(t, tt.parents[i], res.ParentHandle)
				for _, action := range res.Actions {
					assert.Equal(t, res.Permission+rs.Delimiter+action.Handle, action.Permission)
				}
			}
		})
	}
}

// TestProcessResourceServer_RejectsInvalidParents verifies that unresolvable parents are rejected.
func TestProcessResourceServer_RejectsInvalidParents(t *testing.T) {
	tests := []struct {
		name      string
		resources string
		errSubstr string
	}{
		{
			name: "bare handle shared at different depths",
			resources: `
  - { name: Product A, handle: product-a }
  - { name: Org A, handle: org, parent: product-a }
  - { name: Product B, handle: product-b }
  - { name: X, handle: x, parent: product-b }
  - { name: Org B, handle: org, parent: "product-b:x" }
  - { name: Billing, handle: billing, parent: org }
`,
			errSubstr: "parent resource handle 'org' is ambiguous for resource 'billing'",
		},
		{
			name: "bare handle shared by a top-level and a nested resource",
			resources: `
  - { name: Member, handle: member }
  - { name: App, handle: app }
  - { name: App Member, handle: member, parent: app }
  - { name: Child, handle: child, parent: member }
`,
			errSubstr: "parent resource handle 'member' is ambiguous for resource 'child'",
		},
		{
			name: "same handle under the same parent",
			resources: `
  - { name: App, handle: app }
  - { name: Member, handle: member, parent: app }
  - { name: Member Again, handle: member, parent: app }
`,
			errSubstr: "duplicate resource handle 'member' found under parent 'app'",
		},
		{
			name: "duplicate handle also used as a parent",
			resources: `
  - { name: A, handle: a }
  - { name: A Again, handle: a }
  - { name: C, handle: c, parent: a }
`,
			errSubstr: "duplicate resource handle 'a' found at the top level",
		},
		{
			name: "duplicate handle under one parent named in different forms",
			resources: `
  - { name: X, handle: x }
  - { name: A, handle: a, parent: x }
  - { name: A Again, handle: a, parent: "x" }
  - { name: C, handle: c, parent: a }
`,
			errSubstr: "duplicate resource handle 'a' found under parent 'x'",
		},
		{
			name: "missing bare parent",
			resources: `
  - { name: Child, handle: child, parent: missing }
`,
			errSubstr: "parent resource handle 'missing' not found for resource 'child'",
		},
		{
			name: "path naming no resource",
			resources: `
  - { name: App, handle: app }
  - { name: X, handle: x }
  - { name: Project, handle: project, parent: app }
  - { name: Member, handle: member, parent: "x:project" }
`,
			errSubstr: "parent resource path 'x:project' not found for resource 'member'",
		},
		{
			name: "cycle",
			resources: `
  - { name: A, handle: a, parent: b }
  - { name: B, handle: b, parent: a }
`,
			errSubstr: "circular parent reference detected",
		},
		{
			name: "path into a cycle reports the cycle",
			resources: `
  - { name: C, handle: c, parent: "a:b" }
  - { name: A, handle: a, parent: b }
  - { name: B, handle: b, parent: a }
`,
			errSubstr: "circular parent reference detected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := processResourceServerYAML(t, "", tt.resources)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errSubstr)
		})
	}
}

// TestProcessResourceServer_ValidatesHandlesAndDelimiter verifies the REST API's delimiter and handle
// rules.
func TestProcessResourceServer_ValidatesHandlesAndDelimiter(t *testing.T) {
	tests := []struct {
		name      string
		delimiter string
		resources string
		errSubstr string
	}{
		{
			name:      "resource handle holding the delimiter",
			resources: "  - { name: AB, handle: \"a:b\" }\n",
			errSubstr: "invalid handle 'a:b' for resource 'AB'",
		},
		{
			name:      "action handle holding the delimiter",
			delimiter: "/",
			resources: "  - { name: App, handle: app, actions: [{ name: Read, handle: \"read/all\" }] }\n",
			errSubstr: "invalid handle 'read/all' for action 'Read'",
		},
		{
			name:      "handle with an invalid character",
			resources: "  - { name: App, handle: \"my app\" }\n",
			errSubstr: "invalid handle 'my app' for resource 'App'",
		},
		{
			name:      "invalid delimiter",
			delimiter: "::",
			resources: "  - { name: App, handle: app }\n",
			errSubstr: "invalid delimiter '::'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := processResourceServerYAML(t, tt.delimiter, tt.resources)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errSubstr)
		})
	}
}

// TestProcessResourceServer_DefaultsToConfiguredDelimiter verifies that a missing delimiter takes the
// configured default.
func TestProcessResourceServer_DefaultsToConfiguredDelimiter(t *testing.T) {
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", &config.Config{
		Resource: engineconfig.ResourceConfig{DefaultDelimiter: "/"},
	})
	assert.NoError(t, err)
	defer config.ResetServerRuntime()

	rs, err := processResourceServerYAML(t, "", `
  - { name: App, handle: app }
  - { name: Team, handle: team, parent: app }
`)

	assert.NoError(t, err)
	assert.Equal(t, "/", rs.Delimiter)
	assert.Equal(t, "app/team", rs.Resources[1].Permission)
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

func TestProcessResourceServer_ServerActions(t *testing.T) {
	t.Run("an action of the resource server has its handle as its permission", func(t *testing.T) {
		rs := &providers.ResourceServer{
			ID: "rs1", Name: "Test Server", OUID: "ou1",
			Actions:   []providers.Action{{Name: "Read All", Handle: "read_all"}},
			Resources: []providers.Resource{{Name: "Users", Handle: "users"}},
		}

		assert.NoError(t, ProcessResourceServer(rs))
		assert.Equal(t, "read_all", rs.Actions[0].Permission)
		assert.Equal(t, "users", rs.Resources[0].Permission)
	})

	t.Run("an action handle holding the delimiter is refused", func(t *testing.T) {
		rs := &providers.ResourceServer{
			ID: "rs1", Name: "Test Server", OUID: "ou1",
			Actions: []providers.Action{{Name: "Read All", Handle: "read:all"}},
		}

		err := ProcessResourceServer(rs)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid handle 'read:all' for action 'Read All'")
	})

	t.Run("an MCP action sharing a top-level group's permission is refused", func(t *testing.T) {
		rs := &providers.ResourceServer{
			ID: "rs-mcp", Name: "Booking MCP", OUID: "ou1", Type: providers.ResourceServerTypeMCP,
			Actions:   []providers.Action{{Name: "Ops Tool", Handle: "ops", Kind: providers.ActionKindTool}},
			Resources: []providers.Resource{{Name: "Ops", Handle: "ops"}},
		}

		err := ProcessResourceServer(rs)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate permission 'ops' derived for handles 'ops' and 'ops'")
	})

	t.Run("two MCP actions of the resource server with one handle are refused", func(t *testing.T) {
		rs := &providers.ResourceServer{
			ID: "rs-mcp", Name: "Booking MCP", OUID: "ou1", Type: providers.ResourceServerTypeMCP,
			Actions: []providers.Action{
				{Name: "Search", Handle: "search", Kind: providers.ActionKindTool},
				{Name: "Search Again", Handle: "search", Kind: providers.ActionKindTool},
			},
		}

		err := ProcessResourceServer(rs)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate permission 'search'")
	})
}

func TestParseToResourceServer_ServerActionKinds(t *testing.T) {
	t.Run("an MCP action of the resource server defaults to the tool kind", func(t *testing.T) {
		dto, err := parseToResourceServer([]byte(`
id: "rs1"
name: "Test Server"
type: "MCP"
ouId: "ou1"
actions:
  - { name: "Search", handle: "search" }
  - { name: "Docs", handle: "docs", kind: "resource" }
`))

		require.NoError(t, err)
		assert.Equal(t, providers.ActionKindTool, dto.Actions[0].Kind)
		assert.Equal(t, providers.ActionKindResource, dto.Actions[1].Kind)
	})

	t.Run("an action of the resource server with an invalid kind is refused", func(t *testing.T) {
		dto, err := parseToResourceServer([]byte(`
id: "rs1"
name: "Test Server"
type: "MCP"
ouId: "ou1"
actions:
  - { name: "Prompt", handle: "prompt", kind: "prompt" }
`))

		assert.Error(t, err)
		assert.Nil(t, dto)
		assert.Contains(t, err.Error(), `action "prompt" in resource server 'Test Server' has invalid kind "prompt"`)
	})
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

func TestValidateResourceServerWrapper_InvalidMCPIdentifier(t *testing.T) {
	fileStore := newResourceStoreInterfaceMock(t)

	err := validateResourceServerWrapper(&providers.ResourceServer{
		ID: "rs1", Name: "Server", Identifier: "my-mcp-server", Type: providers.ResourceServerTypeMCP, OUID: "ou1",
	}, fileStore, nil, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be an http or https URL")
}

func TestValidateResourceServerWrapper_MCPWithURIIdentifier(t *testing.T) {
	fileStore := newResourceStoreInterfaceMock(t)
	fileStore.On("GetResourceServer", mock.Anything, "rs1").
		Return(providers.ResourceServer{}, errResourceServerNotFound)

	err := validateResourceServerWrapper(&providers.ResourceServer{
		ID: "rs1", Name: "Server", Identifier: "https://mcp.example.com/mcp",
		Type: providers.ResourceServerTypeMCP, OUID: "ou1",
	}, fileStore, nil, nil)

	assert.NoError(t, err)
}

func TestValidateResourceServerWrapper_MCPIdentifierReferenceOnControlPlane(t *testing.T) {
	fileStore := newResourceStoreInterfaceMock(t)
	fileStore.On("GetResourceServer", mock.Anything, "rs1").
		Return(providers.ResourceServer{}, errResourceServerNotFound)

	err := validateResourceServerWrapper(&providers.ResourceServer{
		ID: "rs1", Name: "Server", Identifier: "var:RESOURCE_SERVER_SERVER_IDENTIFIER",
		Type: providers.ResourceServerTypeMCP, OUID: "ou1",
	}, fileStore, nil, &resourceService{keepsReferences: true})

	assert.NoError(t, err)
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
