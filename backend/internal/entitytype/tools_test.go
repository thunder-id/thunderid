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

func (suite *EntityTypeToolsTestSuite) TestListAgentTypes_Success() {
	mockService := NewEntityTypeServiceInterfaceMock(suite.T())
	tools := &entityTypeTools{entityTypeService: mockService}

	mockService.On("GetEntityTypeList", mock.Anything, TypeCategoryAgent, mock.Anything, mock.Anything, false).
		Return(&EntityTypeListResponse{
			TotalResults: 1,
			Types:        []EntityTypeListItem{{ID: "at1", Handle: "assistant", DisplayName: "Assistant", OUID: "ou1"}},
		}, nil)

	result, output, err := tools.listAgentTypes(context.Background(), &mcp.CallToolRequest{}, nil)

	suite.NoError(err)
	suite.Nil(result)
	suite.Require().NotNil(output)
	suite.Equal(1, output.TotalResults)
	suite.Require().Len(output.Types, 1)
	suite.Equal("assistant", output.Types[0].Handle)
	suite.Equal("ou1", output.Types[0].OUID)
}

func (suite *EntityTypeToolsTestSuite) TestListAgentTypes_ServiceError() {
	mockService := NewEntityTypeServiceInterfaceMock(suite.T())
	tools := &entityTypeTools{entityTypeService: mockService}

	mockService.On("GetEntityTypeList", mock.Anything, TypeCategoryAgent, mock.Anything, mock.Anything, false).
		Return(nil, &tidcommon.ServiceError{
			ErrorDescription: tidcommon.I18nMessage{DefaultValue: "database error"},
		})

	result, output, err := tools.listAgentTypes(context.Background(), &mcp.CallToolRequest{}, nil)

	suite.Nil(result)
	suite.Nil(output)
	suite.ErrorContains(err, "failed to list agent types: database error")
}
