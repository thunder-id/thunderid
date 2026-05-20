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

package flowmgt

import (
	"github.com/thunder-id/thunderid/internal/system/i18n/core"

	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	flowCommon "github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/mcp/tool"
)

type FlowToolsTestSuite struct {
	suite.Suite
}

func TestFlowToolsTestSuite(t *testing.T) {
	suite.Run(t, new(FlowToolsTestSuite))
}

func (suite *FlowToolsTestSuite) TestNewFlowTools() {
	mockService := NewFlowMgtServiceInterfaceMock(suite.T())
	tools := &flowTools{flowService: mockService}

	assert.NotNil(suite.T(), tools)
	assert.Equal(suite.T(), mockService, tools.flowService)
}

func (suite *FlowToolsTestSuite) TestListFlows_Success() {
	mockService := NewFlowMgtServiceInterfaceMock(suite.T())
	tools := &flowTools{flowService: mockService}

	expectedFlows := []BasicFlowDefinition{
		{
			ID:       "flow1",
			Handle:   "basic-login",
			Name:     "Basic Login",
			FlowType: flowCommon.FlowTypeAuthentication,
		},
		{
			ID:       "flow2",
			Handle:   "registration",
			Name:     "User Registration",
			FlowType: flowCommon.FlowTypeRegistration,
		},
	}

	mockService.On("ListFlows", mock.Anything, 30, 0, flowCommon.FlowType("")).Return(&FlowListResponse{
		TotalResults: 2,
		Flows:        expectedFlows,
	}, nil)

	ctx := context.Background()
	req := &mcp.CallToolRequest{}
	input := listFlowsInput{
		PaginationInput: tool.PaginationInput{Limit: 0, Offset: 0},
		FlowType:        "",
	}

	result, output, err := tools.listFlows(ctx, req, input)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), 2, output.TotalCount)
	assert.Len(suite.T(), output.Flows, 2)
	assert.Equal(suite.T(), "flow1", output.Flows[0].ID)

	mockService.AssertExpectations(suite.T())
}

func (suite *FlowToolsTestSuite) TestListFlows_WithFilter() {
	mockService := NewFlowMgtServiceInterfaceMock(suite.T())
	tools := &flowTools{flowService: mockService}

	expectedFlows := []BasicFlowDefinition{
		{
			ID:       "flow1",
			Handle:   "basic-login",
			Name:     "Basic Login",
			FlowType: flowCommon.FlowTypeAuthentication,
		},
	}

	mockService.On(
		"ListFlows",
		mock.Anything,
		30,
		0,
		flowCommon.FlowTypeAuthentication,
	).Return(&FlowListResponse{
		TotalResults: 1,
		Flows:        expectedFlows,
	}, nil)

	ctx := context.Background()
	req := &mcp.CallToolRequest{}
	input := listFlowsInput{
		PaginationInput: tool.PaginationInput{Limit: 0, Offset: 0},
		FlowType:        string(flowCommon.FlowTypeAuthentication),
	}

	result, output, err := tools.listFlows(ctx, req, input)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), 1, output.TotalCount)
	assert.Len(suite.T(), output.Flows, 1)

	mockService.AssertExpectations(suite.T())
}

func (suite *FlowToolsTestSuite) TestListFlows_Error() {
	mockService := NewFlowMgtServiceInterfaceMock(suite.T())
	tools := &flowTools{flowService: mockService}

	mockService.On("ListFlows", mock.Anything, 30, 0, flowCommon.FlowType("")).
		Return(nil, &serviceerror.ServiceError{
			ErrorDescription: core.I18nMessage{Key: "error.test.database_error", DefaultValue: "database error"},
		})

	ctx := context.Background()
	req := &mcp.CallToolRequest{}
	input := listFlowsInput{
		PaginationInput: tool.PaginationInput{Limit: 0, Offset: 0},
		FlowType:        "",
	}

	result, output, err := tools.listFlows(ctx, req, input)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), flowListOutput{}, output)
	assert.Contains(suite.T(), err.Error(), "failed to list flows")

	mockService.AssertExpectations(suite.T())
}

func (suite *FlowToolsTestSuite) TestGetFlowByHandle_Success() {
	mockService := NewFlowMgtServiceInterfaceMock(suite.T())
	tools := &flowTools{flowService: mockService}

	expectedFlow := &CompleteFlowDefinition{
		ID:       "flow123",
		Handle:   "basic-login",
		Name:     "Basic Login Flow",
		FlowType: flowCommon.FlowTypeAuthentication,
		Nodes: []NodeDefinition{
			{
				ID:   "start",
				Type: "START",
			},
			{
				ID:   "end",
				Type: "END",
			},
		},
	}

	mockService.On(
		"GetFlowByHandle",
		mock.Anything,
		"basic-login",
		flowCommon.FlowTypeAuthentication,
	).Return(expectedFlow, nil)

	ctx := context.Background()
	req := &mcp.CallToolRequest{}
	input := getFlowByHandleInput{
		Handle:   "basic-login",
		FlowType: string(flowCommon.FlowTypeAuthentication),
	}

	result, output, err := tools.getFlowByHandle(ctx, req, input)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), expectedFlow, output)

	mockService.AssertExpectations(suite.T())
}

func (suite *FlowToolsTestSuite) TestGetFlowByHandle_Error() {
	mockService := NewFlowMgtServiceInterfaceMock(suite.T())
	tools := &flowTools{flowService: mockService}

	mockService.On(
		"GetFlowByHandle",
		mock.Anything,
		"nonexistent",
		flowCommon.FlowTypeAuthentication,
	).Return(nil, &serviceerror.ServiceError{
		ErrorDescription: core.I18nMessage{Key: "error.test.flow_not_found", DefaultValue: "flow not found"},
	})

	ctx := context.Background()
	req := &mcp.CallToolRequest{}
	input := getFlowByHandleInput{
		Handle:   "nonexistent",
		FlowType: string(flowCommon.FlowTypeAuthentication),
	}

	result, output, err := tools.getFlowByHandle(ctx, req, input)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Nil(suite.T(), output)
	assert.Contains(suite.T(), err.Error(), "failed to get flow by handle")

	mockService.AssertExpectations(suite.T())
}

func (suite *FlowToolsTestSuite) TestGetFlowByID_Success() {
	mockService := NewFlowMgtServiceInterfaceMock(suite.T())
	tools := &flowTools{flowService: mockService}

	expectedFlow := &CompleteFlowDefinition{
		ID:       "flow123",
		Handle:   "basic-login",
		Name:     "Basic Login Flow",
		FlowType: flowCommon.FlowTypeAuthentication,
	}

	mockService.On("GetFlow", mock.Anything, "flow123").Return(expectedFlow, nil)

	ctx := context.Background()
	req := &mcp.CallToolRequest{}
	input := tool.IDInput{ID: "flow123"}

	result, output, err := tools.getFlowByID(ctx, req, input)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), expectedFlow, output)

	mockService.AssertExpectations(suite.T())
}

func (suite *FlowToolsTestSuite) TestGetFlowByID_Error() {
	mockService := NewFlowMgtServiceInterfaceMock(suite.T())
	tools := &flowTools{flowService: mockService}

	mockService.On("GetFlow", mock.Anything, "flow123").Return(nil, &serviceerror.ServiceError{
		ErrorDescription: core.I18nMessage{Key: "error.test.flow_not_found", DefaultValue: "flow not found"},
	})

	ctx := context.Background()
	req := &mcp.CallToolRequest{}
	input := tool.IDInput{ID: "flow123"}

	result, output, err := tools.getFlowByID(ctx, req, input)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Nil(suite.T(), output)
	assert.Contains(suite.T(), err.Error(), "failed to get flow")

	mockService.AssertExpectations(suite.T())
}

func (suite *FlowToolsTestSuite) TestCreateFlow_Success() {
	mockService := NewFlowMgtServiceInterfaceMock(suite.T())
	tools := &flowTools{flowService: mockService}

	inputFlow := FlowDefinition{
		Handle:   "new-flow",
		Name:     "New Flow",
		FlowType: flowCommon.FlowTypeAuthentication,
		Nodes: []NodeDefinition{
			{
				ID:   "start",
				Type: "START",
			},
			{
				ID:   "end",
				Type: "END",
			},
		},
	}

	createdFlow := &CompleteFlowDefinition{
		ID:       "new-flow-id",
		Handle:   "new-flow",
		Name:     "New Flow",
		FlowType: flowCommon.FlowTypeAuthentication,
		Nodes:    inputFlow.Nodes,
	}

	mockService.On("CreateFlow", mock.Anything, &inputFlow).Return(createdFlow, nil)

	ctx := context.Background()
	req := &mcp.CallToolRequest{}

	result, output, err := tools.createFlow(ctx, req, inputFlow)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), createdFlow, output)

	mockService.AssertExpectations(suite.T())
}

func (suite *FlowToolsTestSuite) TestCreateFlow_Error() {
	mockService := NewFlowMgtServiceInterfaceMock(suite.T())
	tools := &flowTools{flowService: mockService}

	inputFlow := FlowDefinition{
		Handle:   "invalid-flow",
		Name:     "Invalid Flow",
		FlowType: flowCommon.FlowTypeAuthentication,
	}

	mockService.On("CreateFlow", mock.Anything, &inputFlow).Return(nil, &serviceerror.ServiceError{
		ErrorDescription: core.I18nMessage{Key: "error.test.validation_error", DefaultValue: "validation error"},
	})

	ctx := context.Background()
	req := &mcp.CallToolRequest{}

	result, output, err := tools.createFlow(ctx, req, inputFlow)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Nil(suite.T(), output)
	assert.Contains(suite.T(), err.Error(), "failed to create flow")

	mockService.AssertExpectations(suite.T())
}

func (suite *FlowToolsTestSuite) TestUpdateFlow_Success() {
	mockService := NewFlowMgtServiceInterfaceMock(suite.T())
	tools := &flowTools{flowService: mockService}

	currentFlow := &CompleteFlowDefinition{
		ID:       "flow123",
		Handle:   "existing-flow",
		Name:     "Old Name",
		FlowType: flowCommon.FlowTypeAuthentication,
		Nodes: []NodeDefinition{
			{
				ID:   "start",
				Type: "START",
			},
		},
	}

	inputUpdate := updateFlowInput{
		ID:   "flow123",
		Name: "Updated Name",
		Nodes: []NodeDefinition{
			{
				ID:   "start",
				Type: "START",
			},
			{
				ID:   "end",
				Type: "END",
			},
		},
	}

	updatedFlow := &CompleteFlowDefinition{
		ID:       "flow123",
		Handle:   "existing-flow",
		Name:     "Updated Name",
		FlowType: flowCommon.FlowTypeAuthentication,
		Nodes:    inputUpdate.Nodes,
	}

	mockService.On("GetFlow", mock.Anything, "flow123").Return(currentFlow, nil)
	mockService.On("UpdateFlow", mock.Anything, "flow123", &FlowDefinition{
		Handle:   "existing-flow",
		FlowType: flowCommon.FlowTypeAuthentication,
		Name:     "Updated Name",
		Nodes:    inputUpdate.Nodes,
	}).Return(updatedFlow, nil)

	ctx := context.Background()
	req := &mcp.CallToolRequest{}

	result, output, err := tools.updateFlow(ctx, req, inputUpdate)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), updatedFlow, output)

	mockService.AssertExpectations(suite.T())
}

func (suite *FlowToolsTestSuite) TestUpdateFlow_GetFlowError() {
	mockService := NewFlowMgtServiceInterfaceMock(suite.T())
	tools := &flowTools{flowService: mockService}

	inputUpdate := updateFlowInput{
		ID:   "flow123",
		Name: "Updated Name",
		Nodes: []NodeDefinition{
			{
				ID:   "start",
				Type: "START",
			},
		},
	}

	mockService.On("GetFlow", mock.Anything, "flow123").Return(nil, &serviceerror.ServiceError{
		ErrorDescription: core.I18nMessage{Key: "error.test.flow_not_found", DefaultValue: "flow not found"},
	})

	ctx := context.Background()
	req := &mcp.CallToolRequest{}

	result, output, err := tools.updateFlow(ctx, req, inputUpdate)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Nil(suite.T(), output)
	assert.Contains(suite.T(), err.Error(), "failed to get flow")

	mockService.AssertExpectations(suite.T())
}

func (suite *FlowToolsTestSuite) TestUpdateFlow_UpdateError() {
	mockService := NewFlowMgtServiceInterfaceMock(suite.T())
	tools := &flowTools{flowService: mockService}

	currentFlow := &CompleteFlowDefinition{
		ID:       "flow123",
		Handle:   "existing-flow",
		Name:     "Old Name",
		FlowType: flowCommon.FlowTypeAuthentication,
	}

	inputUpdate := updateFlowInput{
		ID:   "flow123",
		Name: "Updated Name",
		Nodes: []NodeDefinition{
			{
				ID:   "start",
				Type: "START",
			},
		},
	}

	mockService.On("GetFlow", mock.Anything, "flow123").Return(currentFlow, nil)
	mockService.On("UpdateFlow", mock.Anything, "flow123", &FlowDefinition{
		Handle:   "existing-flow",
		FlowType: flowCommon.FlowTypeAuthentication,
		Name:     "Updated Name",
		Nodes:    inputUpdate.Nodes,
	}).Return(nil, &serviceerror.ServiceError{
		ErrorDescription: core.I18nMessage{Key: "error.test.validation_error", DefaultValue: "validation error"},
	})

	ctx := context.Background()
	req := &mcp.CallToolRequest{}

	result, output, err := tools.updateFlow(ctx, req, inputUpdate)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Nil(suite.T(), output)
	assert.Contains(suite.T(), err.Error(), "failed to update flow")

	mockService.AssertExpectations(suite.T())
}

func (suite *FlowToolsTestSuite) TestRegisterMCPTools() {
	mockService := NewFlowMgtServiceInterfaceMock(suite.T())
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "test-server",
		Version: "1.0.0",
	}, nil)

	// Register tools
	registerMCPTools(server, mockService)

	// Verify tools are registered by checking server has tools
	// Note: We can't directly verify the tools list without accessing internal server state,
	// but we can verify the function doesn't panic
	assert.NotNil(suite.T(), server)
}

func (suite *FlowToolsTestSuite) TestGetListFlowsSchema() {
	schema := getListFlowsSchema()

	assert.NotNil(suite.T(), schema)
}

func (suite *FlowToolsTestSuite) TestGetFlowByHandleSchema() {
	schema := getFlowByHandleSchema()

	assert.NotNil(suite.T(), schema)
}

func (suite *FlowToolsTestSuite) TestGetFlowByIDSchema() {
	schema := getFlowByIDSchema()

	assert.NotNil(suite.T(), schema)
}

func (suite *FlowToolsTestSuite) TestGetCreateFlowSchema() {
	schema := getCreateFlowSchema()

	assert.NotNil(suite.T(), schema)
}

func (suite *FlowToolsTestSuite) TestGetUpdateFlowSchema() {
	schema := getUpdateFlowSchema()

	assert.NotNil(suite.T(), schema)
}
