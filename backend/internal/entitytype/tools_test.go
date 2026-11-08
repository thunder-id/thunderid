// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entitytype

import (
	"context"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/security"
)

type EntityTypeToolsTestSuite struct {
	suite.Suite
}

func TestEntityTypeToolsTestSuite(t *testing.T) {
	suite.Run(t, new(EntityTypeToolsTestSuite))
}

func (suite *EntityTypeToolsTestSuite) TestListUserTypes_Success() {
	mockService := NewEntityTypeServiceInterfaceMock(suite.T())
	tools := &entityTypeTools{entityTypeService: mockService}

	expectedTypes := []EntityTypeListItem{
		{
			ID:                    "et1",
			Handle:                "employee",
			DisplayName:           "Employee",
			OUID:                  "ou1",
			OUHandle:              "engineering",
			AllowSelfRegistration: false,
			IsReadOnly:            false,
		},
		{
			ID:                    "et2",
			Handle:                "customer",
			DisplayName:           "Customer",
			OUID:                  "ou2",
			OUHandle:              "sales",
			AllowSelfRegistration: true,
			IsReadOnly:            true,
		},
	}

	mockService.On("GetEntityTypeList", mock.Anything, TypeCategoryUser, mock.Anything, mock.Anything, false).
		Return(&EntityTypeListResponse{
			TotalResults: 2,
			Types:        expectedTypes,
		}, nil)

	ctx := context.Background()
	req := &mcp.CallToolRequest{}

	result, output, err := tools.listUserTypes(ctx, req, nil)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), output)
	assert.Equal(suite.T(), 2, output.TotalResults)
	assert.Len(suite.T(), output.Types, 2)

	item := output.Types[0]
	assert.Equal(suite.T(), "et1", item.ID)
	assert.Equal(suite.T(), "employee", item.Handle)
	assert.Equal(suite.T(), "Employee", item.DisplayName)
	assert.Equal(suite.T(), "ou1", item.OUID)
	assert.Equal(suite.T(), "engineering", item.OUHandle)
	assert.False(suite.T(), item.AllowSelfRegistration)
	assert.False(suite.T(), item.IsReadOnly)

	item2 := output.Types[1]
	assert.Equal(suite.T(), "et2", item2.ID)
	assert.True(suite.T(), item2.AllowSelfRegistration)
	assert.True(suite.T(), item2.IsReadOnly)

	mockService.AssertExpectations(suite.T())
}

func (suite *EntityTypeToolsTestSuite) TestListUserTypes_ServiceError() {
	mockService := NewEntityTypeServiceInterfaceMock(suite.T())
	tools := &entityTypeTools{entityTypeService: mockService}

	mockService.On("GetEntityTypeList", mock.Anything, TypeCategoryUser, mock.Anything, mock.Anything, false).
		Return(nil, &tidcommon.ServiceError{
			ErrorDescription: tidcommon.I18nMessage{DefaultValue: "database error"},
		})

	ctx := context.Background()
	req := &mcp.CallToolRequest{}

	result, output, err := tools.listUserTypes(ctx, req, nil)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Nil(suite.T(), output)
	assert.Contains(suite.T(), err.Error(), "failed to list user types")

	mockService.AssertExpectations(suite.T())
}

// TestRegisteredToolsRequireUserTypePermissions verifies that every entity type MCP tool is mapped to a user
// type or agent type catalog permission, so none falls back to root by accident.
func (suite *EntityTypeToolsTestSuite) TestRegisteredToolsRequireUserTypePermissions() {
	security.InitSystemPermissions("")
	p := security.GetSystemPermissions()
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "1.0.0"}, nil)
	registerMCPTools(server, NewEntityTypeServiceInterfaceMock(suite.T()))

	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	_, err := server.Connect(ctx, serverTransport, nil)
	suite.Require().NoError(err)
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil).
		Connect(ctx, clientTransport, nil)
	suite.Require().NoError(err)
	defer func() { _ = session.Close() }()
	tools, err := session.ListTools(ctx, nil)
	suite.Require().NoError(err)

	suite.Require().NotEmpty(tools.Tools)
	for _, registered := range tools.Tools {
		suite.Contains([]string{p.UserTypeView, p.UserTypeCreate, p.UserTypeUpdate, p.UserTypeDelete,
			p.AgentTypeView}, security.ResolveMCPToolPermission(registered.Name),
			"tool %q must require a user type or agent type permission", registered.Name)
	}
}

func (suite *EntityTypeToolsTestSuite) TestListAgentTypes_Success() {
	mockService := NewEntityTypeServiceInterfaceMock(suite.T())
	tools := &entityTypeTools{entityTypeService: mockService}

	listCall := mockService.On("GetEntityTypeList", mock.Anything, TypeCategoryAgent, mock.Anything, 0, false).
		Return(&EntityTypeListResponse{TotalResults: 1, Types: []EntityTypeListItem{{
			ID: "at1", Handle: DefaultAgentTypeHandle, OUID: "ou1",
			SystemAttributes: &SystemAttributes{Display: "name"},
		}}}, nil).Once()
	mockService.On("GetAttributes", mock.Anything, TypeCategoryAgent, DefaultAgentTypeHandle,
		AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]AttributeInfo{
			{Attribute: "name", Type: "string", Required: true, Unique: true},
			{Attribute: "function", Type: "string", Enum: []string{"support", "sales"}},
			{Attribute: "apiKey", Type: "string", Credential: true},
		}, nil).NotBefore(listCall).Once()

	result, output, err := tools.listAgentTypes(context.Background(), &mcp.CallToolRequest{}, nil)

	suite.Require().NoError(err)
	assert.Nil(suite.T(), result)
	suite.Require().Len(output.Types, 1)
	agentType := output.Types[0]
	assert.Equal(suite.T(), "at1", agentType.ID)
	assert.Equal(suite.T(), "name", agentType.DisplayAttribute)
	assert.Equal(suite.T(), []agentTypeAttributeMCPItem{
		{Name: "apiKey", Type: "string", Credential: true},
		{Name: "function", Type: "string", Enum: []string{"support", "sales"}},
		{Name: "name", Type: "string", Required: true, Unique: true},
	}, agentType.Attributes)
}

func (suite *EntityTypeToolsTestSuite) TestListAgentTypes_ServiceErrors() {
	failure := &tidcommon.ServiceError{ErrorDescription: tidcommon.I18nMessage{DefaultValue: "database error"}}

	suite.Run("list", func() {
		mockService := NewEntityTypeServiceInterfaceMock(suite.T())
		tools := &entityTypeTools{entityTypeService: mockService}
		mockService.On("GetEntityTypeList", mock.Anything, TypeCategoryAgent, mock.Anything, 0, false).
			Return(nil, failure).Once()

		_, output, err := tools.listAgentTypes(context.Background(), &mcp.CallToolRequest{}, nil)

		assert.ErrorContains(suite.T(), err, "failed to list agent types: database error")
		assert.Nil(suite.T(), output)
		mockService.AssertNotCalled(suite.T(), "GetAttributes", mock.Anything, mock.Anything, mock.Anything,
			mock.Anything)
	})

	suite.Run("attributes", func() {
		mockService := NewEntityTypeServiceInterfaceMock(suite.T())
		tools := &entityTypeTools{entityTypeService: mockService}
		mockService.On("GetEntityTypeList", mock.Anything, TypeCategoryAgent, mock.Anything, 0, false).
			Return(&EntityTypeListResponse{Types: []EntityTypeListItem{{Handle: DefaultAgentTypeHandle}}}, nil).Once()
		mockService.On("GetAttributes", mock.Anything, TypeCategoryAgent, DefaultAgentTypeHandle, mock.Anything).
			Return(nil, failure).Once()

		_, output, err := tools.listAgentTypes(context.Background(), &mcp.CallToolRequest{}, nil)

		assert.ErrorContains(suite.T(), err, "failed to list agent types: database error")
		assert.Nil(suite.T(), output)
	})
}
