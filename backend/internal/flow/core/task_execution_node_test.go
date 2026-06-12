/*
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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

package core

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	i18ncore "github.com/thunder-id/thunderid/internal/system/i18n/core"
)

type TaskExecutionNodeTestSuite struct {
	suite.Suite
	mockExecutor *ExecutorInterfaceMock
}

func TestTaskExecutionNodeTestSuite(t *testing.T) {
	suite.Run(t, new(TaskExecutionNodeTestSuite))
}

func (s *TaskExecutionNodeTestSuite) SetupTest() {
	s.mockExecutor = NewExecutorInterfaceMock(s.T())
}

func (s *TaskExecutionNodeTestSuite) TestNewTaskExecutionNode() {
	node := newTaskExecutionNode("task-1", map[string]interface{}{"key": "value"}, true, false)

	s.NotNil(node)
	s.Equal("task-1", node.GetID())
	s.Equal(common.NodeTypeTaskExecution, node.GetType())
	s.True(node.IsStartNode())
	s.False(node.IsFinalNode())
}

func (s *TaskExecutionNodeTestSuite) TestExecutorMethods() {
	node := newTaskExecutionNode("task-1", map[string]interface{}{}, false, false)
	execNode, ok := node.(ExecutorBackedNodeInterface)
	s.True(ok)

	s.Empty(execNode.GetExecutorName())
	s.Nil(execNode.GetExecutor())

	execNode.SetExecutorName("test-executor")
	s.Equal("test-executor", execNode.GetExecutorName())

	s.mockExecutor.On("GetName").Return("mock-executor")
	execNode.SetExecutor(s.mockExecutor)
	s.NotNil(execNode.GetExecutor())
	s.Equal("mock-executor", execNode.GetExecutorName())
}

func (s *TaskExecutionNodeTestSuite) TestExecuteNoExecutor() {
	node := newTaskExecutionNode("task-1", map[string]interface{}{}, false, false)
	ctx := &NodeContext{ExecutionID: "test-flow"}

	resp, err := node.Execute(ctx)

	s.NotNil(err)
	s.Nil(resp)
}

func (s *TaskExecutionNodeTestSuite) TestExecuteSuccess() {
	tests := []struct {
		name           string
		setupMock      func(*ExecutorInterfaceMock)
		expectedStatus common.NodeStatus
		expectedType   common.NodeResponseType
	}{
		{
			name: "Complete execution",
			setupMock: func(m *ExecutorInterfaceMock) {
				m.On("GetName").Return("test-executor").Once()
				m.On("Execute", mock.Anything).Return(
					&common.ExecutorResponse{
						Status:         common.ExecComplete,
						AdditionalData: map[string]string{"key": "value"},
						RuntimeData:    map[string]string{"runtime": "data"},
					}, nil,
				).Once()
			},
			expectedStatus: common.NodeStatusComplete,
			expectedType:   "",
		},
		{
			name: "User input required",
			setupMock: func(m *ExecutorInterfaceMock) {
				m.On("GetName").Return("test-executor").Once()
				m.On("Execute", mock.Anything).Return(
					&common.ExecutorResponse{
						Status: common.ExecUserInputRequired,
						Inputs: []common.Input{{Identifier: "username", Required: true}},
					}, nil,
				).Once()
			},
			expectedStatus: common.NodeStatusIncomplete,
			expectedType:   common.NodeResponseTypeView,
		},
		{
			name: "External redirection",
			setupMock: func(m *ExecutorInterfaceMock) {
				m.On("GetName").Return("test-executor").Once()
				m.On("Execute", mock.Anything).Return(
					&common.ExecutorResponse{
						Status:      common.ExecExternalRedirection,
						RedirectURL: "https://example.com/auth",
					}, nil,
				).Once()
			},
			expectedStatus: common.NodeStatusIncomplete,
			expectedType:   common.NodeResponseTypeRedirection,
		},
		{
			name: "Retry execution",
			setupMock: func(m *ExecutorInterfaceMock) {
				m.On("GetName").Return("test-executor").Once()
				m.On("Execute", mock.Anything).Return(
					&common.ExecutorResponse{Status: common.ExecRetry},
					nil,
				).Once()
			},
			expectedStatus: common.NodeStatusIncomplete,
			expectedType:   common.NodeResponseTypeRetry,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			mockExec := NewExecutorInterfaceMock(s.T())
			node := newTaskExecutionNode("task-1", map[string]interface{}{}, false, false)
			execNode, _ := node.(ExecutorBackedNodeInterface)
			tt.setupMock(mockExec)
			execNode.SetExecutor(mockExec)

			ctx := &NodeContext{ExecutionID: "test-flow"}
			resp, err := node.Execute(ctx)

			s.Nil(err)
			s.NotNil(resp)
			s.Equal(tt.expectedStatus, resp.Status)
			s.Equal(tt.expectedType, resp.Type)
		})
	}
}

func (s *TaskExecutionNodeTestSuite) TestExecuteFailure() {
	s.mockExecutor.On("GetName").Return("test-executor").Once()
	s.mockExecutor.On("Execute", mock.Anything).Return(
		&common.ExecutorResponse{
			Status: common.ExecFailure,
			Error:  &serviceerror.ServiceError{Error: i18ncore.I18nMessage{DefaultValue: "AUTH_FAILED"}},
		},
		nil,
	).Once()

	node := newTaskExecutionNode("task-1", map[string]interface{}{}, false, false)
	execNode, _ := node.(ExecutorBackedNodeInterface)
	execNode.SetExecutor(s.mockExecutor)

	ctx := &NodeContext{ExecutionID: "test-flow"}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusFailure, resp.Status)
	s.Equal("AUTH_FAILED", resp.Error.Error.DefaultValue)
}

func (s *TaskExecutionNodeTestSuite) TestExecuteFailureWithOnFailureHandler() {
	s.mockExecutor.On("GetName").Return("test-executor").Once()
	s.mockExecutor.On("Execute", mock.Anything).Return(
		&common.ExecutorResponse{
			Status: common.ExecFailure,
			Error:  &serviceerror.ServiceError{Error: i18ncore.I18nMessage{DefaultValue: "AUTH_FAILED"}},
		},
		nil,
	).Once()

	node := newTaskExecutionNode("task-1", map[string]interface{}{}, false, false)
	execNode, _ := node.(ExecutorBackedNodeInterface)
	execNode.SetOnFailure("error-prompt")
	execNode.SetExecutor(s.mockExecutor)

	ctx := &NodeContext{ExecutionID: "test-flow"}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusForward, resp.Status)
	s.Equal("error-prompt", resp.NextNodeID)
	s.Equal("AUTH_FAILED", resp.Error.Error.DefaultValue)
	s.NotNil(resp.RuntimeData)
	var svcErr217 serviceerror.ServiceError
	s.NoError(json.Unmarshal([]byte(resp.RuntimeData["failureReasonJSON"]), &svcErr217))
	s.Equal("AUTH_FAILED", svcErr217.Error.DefaultValue)
}

func (s *TaskExecutionNodeTestSuite) TestExecuteExecutorError() {
	s.mockExecutor.On("GetName").Return("test-executor").Once()
	s.mockExecutor.On("Execute", mock.Anything).Return(nil, assert.AnError).Once()

	node := newTaskExecutionNode("task-1", map[string]interface{}{}, false, false)
	execNode, _ := node.(ExecutorBackedNodeInterface)
	execNode.SetExecutor(s.mockExecutor)

	ctx := &NodeContext{ExecutionID: "test-flow"}
	resp, err := node.Execute(ctx)

	s.NotNil(err)
	s.Nil(resp)
}

func (s *TaskExecutionNodeTestSuite) TestExecuteNilExecutorResponse() {
	node := newTaskExecutionNode("task-1", map[string]interface{}{}, false, false)
	execNode, _ := node.(ExecutorBackedNodeInterface)

	s.mockExecutor.On("GetName").Return("test-executor").Once()
	s.mockExecutor.On("Execute", mock.Anything).Return(nil, nil).Once()
	execNode.SetExecutor(s.mockExecutor)

	ctx := &NodeContext{ExecutionID: "test-flow"}
	resp, err := node.Execute(ctx)

	s.NotNil(err)
	s.Nil(resp)
}

func (s *TaskExecutionNodeTestSuite) TestExecutePopulatedNodeProperties() {
	mockExec := NewExecutorInterfaceMock(s.T())

	props := map[string]interface{}{"k": "v"}
	node := newTaskExecutionNode("task-props", props, false, false)
	execNode, _ := node.(ExecutorBackedNodeInterface)

	mockExec.On("GetName").Return("test-executor").Once()
	mockExec.On("Execute", mock.Anything).Return(
		&common.ExecutorResponse{Status: common.ExecComplete}, nil,
	).Once()

	execNode.SetExecutor(mockExec)

	ctx := &NodeContext{ExecutionID: "test-flow"}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(props, ctx.NodeProperties)
}

func (s *TaskExecutionNodeTestSuite) TestBuildNodeResponse() {
	tests := []struct {
		name         string
		execStatus   common.ExecutorStatus
		nodeStatus   common.NodeStatus
		responseType common.NodeResponseType
	}{
		{"ExecComplete", common.ExecComplete, common.NodeStatusComplete, ""},
		{"ExecUserInputRequired", common.ExecUserInputRequired, common.NodeStatusIncomplete,
			common.NodeResponseTypeView},
		{"ExecExternalRedirection", common.ExecExternalRedirection, common.NodeStatusIncomplete,
			common.NodeResponseTypeRedirection},
		{"ExecRetry", common.ExecRetry, common.NodeStatusIncomplete, common.NodeResponseTypeRetry},
		{"ExecFailure", common.ExecFailure, common.NodeStatusFailure, ""},
		{"Unknown status", common.ExecutorStatus("UNKNOWN"), common.NodeStatusIncomplete, ""},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			node := newTaskExecutionNode("task-1", map[string]interface{}{}, false, false).(*taskExecutionNode)
			execResp := &common.ExecutorResponse{Status: tt.execStatus}
			nodeResp := node.buildNodeResponse(execResp)

			s.NotNil(nodeResp)
			s.Equal(tt.nodeStatus, nodeResp.Status)
			s.Equal(tt.responseType, nodeResp.Type)
			s.NotNil(nodeResp.AdditionalData)
			s.NotNil(nodeResp.RuntimeData)
			s.NotNil(nodeResp.Inputs)
			s.NotNil(nodeResp.Actions)
		})
	}
}

func (s *TaskExecutionNodeTestSuite) TestModeMethods() {
	node := newTaskExecutionNode("task-1", map[string]interface{}{}, false, false)
	execNode, ok := node.(ExecutorBackedNodeInterface)
	s.True(ok)

	// Test default mode is empty
	s.Empty(execNode.GetMode())

	// Test setting mode
	execNode.SetMode("send")
	s.Equal("send", execNode.GetMode())

	// Test updating mode
	execNode.SetMode("verify")
	s.Equal("verify", execNode.GetMode())
}

func (s *TaskExecutionNodeTestSuite) TestExecuteWithMode() {
	mockExec := NewExecutorInterfaceMock(s.T())
	node := newTaskExecutionNode("task-1", map[string]interface{}{}, false, false)
	execNode, _ := node.(ExecutorBackedNodeInterface)

	// Set mode
	execNode.SetMode("send")

	var capturedCtx *NodeContext
	mockExec.On("GetName").Return("test-executor").Once()
	mockExec.On("Execute", mock.Anything).Run(func(args mock.Arguments) {
		capturedCtx = args.Get(0).(*NodeContext)
	}).Return(
		&common.ExecutorResponse{Status: common.ExecComplete}, nil,
	).Once()

	execNode.SetExecutor(mockExec)

	ctx := &NodeContext{ExecutionID: "test-flow"}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.NotNil(capturedCtx)
	s.Equal("send", capturedCtx.ExecutorMode, "Mode should be set in context before calling executor")
}

func (s *TaskExecutionNodeTestSuite) TestExecuteEnrichesRuntimeData() {
	mockExec := NewExecutorInterfaceMock(s.T())
	props := map[string]interface{}{
		"idpId":    "idp-123",
		"senderId": "sender-456",
	}
	node := newTaskExecutionNode("task-1", props, false, false)
	execNode, _ := node.(ExecutorBackedNodeInterface)

	var capturedCtx *NodeContext
	mockExec.On("GetName").Return("test-executor").Once()
	mockExec.On("Execute", mock.Anything).Run(func(args mock.Arguments) {
		capturedCtx = args.Get(0).(*NodeContext)
	}).Return(
		&common.ExecutorResponse{Status: common.ExecComplete}, nil,
	).Once()

	execNode.SetExecutor(mockExec)

	ctx := &NodeContext{
		ExecutionID: "test-flow",
		EntityID:    "app-789",
		RuntimeData: map[string]string{"existing": "value"},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.NotNil(capturedCtx)
	s.Equal("value", capturedCtx.RuntimeData["existing"])
	s.Equal("app-789", capturedCtx.RuntimeData["applicationId"])
	s.Equal("idp-123", capturedCtx.RuntimeData["idpId"])
	s.Equal("sender-456", capturedCtx.RuntimeData["senderId"])
	s.Equal("app-789", ctx.RuntimeData["applicationId"])
}

func (s *TaskExecutionNodeTestSuite) TestOnSuccessMethods() {
	node := newTaskExecutionNode("task-1", map[string]interface{}{}, false, false)
	execNode, ok := node.(ExecutorBackedNodeInterface)
	s.True(ok)

	// Test default onSuccess is empty
	s.Empty(execNode.GetOnSuccess())

	// Test setting onSuccess
	execNode.SetOnSuccess("success-node")
	s.Equal("success-node", execNode.GetOnSuccess())

	// Test updating onSuccess
	execNode.SetOnSuccess("another-success-node")
	s.Equal("another-success-node", execNode.GetOnSuccess())
}

func (s *TaskExecutionNodeTestSuite) TestOnFailureMethods() {
	node := newTaskExecutionNode("task-1", map[string]interface{}{}, false, false)
	execNode, ok := node.(ExecutorBackedNodeInterface)
	s.True(ok)

	// Test default onFailure is empty
	s.Empty(execNode.GetOnFailure())

	// Test setting onFailure
	execNode.SetOnFailure("failure-node")
	s.Equal("failure-node", execNode.GetOnFailure())

	// Test updating onFailure
	execNode.SetOnFailure("another-failure-node")
	s.Equal("another-failure-node", execNode.GetOnFailure())
}

func (s *TaskExecutionNodeTestSuite) TestExecuteWithOnSuccess() {
	mockExec := NewExecutorInterfaceMock(s.T())
	node := newTaskExecutionNode("task-1", map[string]interface{}{}, false, false)
	execNode, _ := node.(ExecutorBackedNodeInterface)

	// Set onSuccess handler
	execNode.SetOnSuccess("success-node")

	mockExec.On("GetName").Return("test-executor").Once()
	mockExec.On("Execute", mock.Anything).Return(
		&common.ExecutorResponse{Status: common.ExecComplete}, nil,
	).Once()

	execNode.SetExecutor(mockExec)

	ctx := &NodeContext{ExecutionID: "test-flow"}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusComplete, resp.Status)
	s.Equal("success-node", resp.NextNodeID, "OnSuccess node should be set as next node")
}

func (s *TaskExecutionNodeTestSuite) TestExecuteWithEmptyNodeProperties() {
	mockExec := NewExecutorInterfaceMock(s.T())
	node := newTaskExecutionNode("task-1", nil, false, false)
	execNode, _ := node.(ExecutorBackedNodeInterface)

	var capturedCtx *NodeContext
	mockExec.On("GetName").Return("test-executor").Once()
	mockExec.On("Execute", mock.Anything).Run(func(args mock.Arguments) {
		capturedCtx = args.Get(0).(*NodeContext)
	}).Return(
		&common.ExecutorResponse{Status: common.ExecComplete}, nil,
	).Once()

	execNode.SetExecutor(mockExec)

	ctx := &NodeContext{ExecutionID: "test-flow"}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.NotNil(capturedCtx)
	s.NotNil(capturedCtx.NodeProperties, "NodeProperties should be initialized even if empty")
	s.Empty(capturedCtx.NodeProperties)
}

func (s *TaskExecutionNodeTestSuite) TestExecuteFailureWithoutOnFailureHandler() {
	mockExec := NewExecutorInterfaceMock(s.T())
	node := newTaskExecutionNode("task-1", map[string]interface{}{}, false, false)
	execNode, _ := node.(ExecutorBackedNodeInterface)

	mockExec.On("GetName").Return("test-executor").Once()
	mockExec.On("Execute", mock.Anything).Return(
		&common.ExecutorResponse{
			Status: common.ExecFailure,
			Error:  &serviceerror.ServiceError{Error: i18ncore.I18nMessage{DefaultValue: "AUTH_FAILED"}},
		},
		nil,
	).Once()

	execNode.SetExecutor(mockExec)

	ctx := &NodeContext{ExecutionID: "test-flow"}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusFailure, resp.Status, "Status should remain failure without onFailure handler")
	s.Empty(resp.NextNodeID, "NextNodeID should not be set without onFailure handler")
	s.Equal("AUTH_FAILED", resp.Error.Error.DefaultValue)
}

func (s *TaskExecutionNodeTestSuite) TestExecuteCompleteWithoutOnSuccess() {
	mockExec := NewExecutorInterfaceMock(s.T())
	node := newTaskExecutionNode("task-1", map[string]interface{}{}, false, false)
	execNode, _ := node.(ExecutorBackedNodeInterface)

	mockExec.On("GetName").Return("test-executor").Once()
	mockExec.On("Execute", mock.Anything).Return(
		&common.ExecutorResponse{Status: common.ExecComplete}, nil,
	).Once()

	execNode.SetExecutor(mockExec)

	ctx := &NodeContext{ExecutionID: "test-flow"}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusComplete, resp.Status)
	s.Empty(resp.NextNodeID, "NextNodeID should not be set without onSuccess handler")
}

func (s *TaskExecutionNodeTestSuite) TestBuildNodeResponseWithNilMaps() {
	node := newTaskExecutionNode("task-1", map[string]interface{}{}, false, false).(*taskExecutionNode)
	execResp := &common.ExecutorResponse{
		Status:         common.ExecComplete,
		AdditionalData: nil,
		RuntimeData:    nil,
		Inputs:         nil,
	}

	nodeResp := node.buildNodeResponse(execResp)

	s.NotNil(nodeResp)
	s.NotNil(nodeResp.AdditionalData, "AdditionalData should be initialized")
	s.NotNil(nodeResp.RuntimeData, "RuntimeData should be initialized")
	s.NotNil(nodeResp.Inputs, "Inputs should be initialized")
	s.NotNil(nodeResp.Actions, "Actions should be initialized")
	s.Empty(nodeResp.AdditionalData)
	s.Empty(nodeResp.RuntimeData)
	s.Empty(nodeResp.Inputs)
	s.Empty(nodeResp.Actions)
}

func (s *TaskExecutionNodeTestSuite) TestBuildNodeResponsePreservesExecutorData() {
	node := newTaskExecutionNode("task-1", map[string]interface{}{}, false, false).(*taskExecutionNode)
	execResp := &common.ExecutorResponse{
		Status:         common.ExecComplete,
		Error:          &serviceerror.ServiceError{Error: i18ncore.I18nMessage{DefaultValue: "TEST_FAILURE"}},
		Inputs:         []common.Input{{Identifier: "email", Required: true}},
		AdditionalData: map[string]string{"key1": "value1"},
		RedirectURL:    "https://example.com",
		RuntimeData:    map[string]string{"runtime": "data"},
		Assertion:      "assertion-token",
	}

	nodeResp := node.buildNodeResponse(execResp)

	s.NotNil(nodeResp)
	s.Equal("TEST_FAILURE", nodeResp.Error.Error.DefaultValue)
	s.Equal(1, len(nodeResp.Inputs))
	s.Equal("email", nodeResp.Inputs[0].Identifier)
	s.Equal("value1", nodeResp.AdditionalData["key1"])
	s.Equal("https://example.com", nodeResp.RedirectURL)
	s.Equal("data", nodeResp.RuntimeData["runtime"])
	s.Equal("assertion-token", nodeResp.Assertion)
}

func (s *TaskExecutionNodeTestSuite) TestExecuteFailureWithOnFailureStoresFailureReason() {
	mockExec := NewExecutorInterfaceMock(s.T())
	node := newTaskExecutionNode("task-1", map[string]interface{}{}, false, false)
	execNode, _ := node.(ExecutorBackedNodeInterface)

	execNode.SetOnFailure("error-handler")

	mockExec.On("GetName").Return("test-executor").Once()
	mockExec.On("Execute", mock.Anything).Return(
		&common.ExecutorResponse{
			Status:      common.ExecFailure,
			Error:       &serviceerror.ServiceError{Error: i18ncore.I18nMessage{DefaultValue: "CUSTOM_ERROR"}},
			RuntimeData: map[string]string{"existing": "data"},
		},
		nil,
	).Once()

	execNode.SetExecutor(mockExec)

	ctx := &NodeContext{ExecutionID: "test-flow"}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusForward, resp.Status)
	s.Equal("error-handler", resp.NextNodeID)
	s.Equal("CUSTOM_ERROR", resp.Error.Error.DefaultValue)
	var svcErr598 serviceerror.ServiceError
	s.NoError(json.Unmarshal([]byte(resp.RuntimeData["failureReasonJSON"]), &svcErr598))
	s.Equal("CUSTOM_ERROR", svcErr598.Error.DefaultValue, "Failure reason should be stored in RuntimeData")
	s.Equal("data", resp.RuntimeData["existing"], "Existing runtime data should be preserved")
}

func (s *TaskExecutionNodeTestSuite) TestExecuteFailureWithOnFailureInitializesRuntimeData() {
	mockExec := NewExecutorInterfaceMock(s.T())
	node := newTaskExecutionNode("task-1", map[string]interface{}{}, false, false)
	execNode, _ := node.(ExecutorBackedNodeInterface)

	execNode.SetOnFailure("error-handler")

	mockExec.On("GetName").Return("test-executor").Once()
	mockExec.On("Execute", mock.Anything).Return(
		&common.ExecutorResponse{
			Status:      common.ExecFailure,
			Error:       &serviceerror.ServiceError{Error: i18ncore.I18nMessage{DefaultValue: "CUSTOM_ERROR"}},
			RuntimeData: nil, // RuntimeData is nil
		},
		nil,
	).Once()

	execNode.SetExecutor(mockExec)

	ctx := &NodeContext{ExecutionID: "test-flow"}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusForward, resp.Status)
	s.Equal("error-handler", resp.NextNodeID)
	s.Equal("CUSTOM_ERROR", resp.Error.Error.DefaultValue)
	s.NotNil(resp.RuntimeData, "RuntimeData should be initialized if nil")
	var svcErr630 serviceerror.ServiceError
	s.NoError(json.Unmarshal([]byte(resp.RuntimeData["failureReasonJSON"]), &svcErr630))
	s.Equal("CUSTOM_ERROR", svcErr630.Error.DefaultValue, "Failure reason should be stored in RuntimeData")
}

func (s *TaskExecutionNodeTestSuite) TestExecuteFailureWithEmptyFailureReasonAndOnFailure() {
	mockExec := NewExecutorInterfaceMock(s.T())
	node := newTaskExecutionNode("task-1", map[string]interface{}{}, false, false)
	execNode, _ := node.(ExecutorBackedNodeInterface)

	execNode.SetOnFailure("error-handler")

	mockExec.On("GetName").Return("test-executor").Once()
	mockExec.On("Execute", mock.Anything).Return(
		&common.ExecutorResponse{
			Status: common.ExecFailure,
			Error:  nil, // No error — onFailure handler should NOT be triggered
		},
		nil,
	).Once()

	execNode.SetExecutor(mockExec)

	ctx := &NodeContext{ExecutionID: "test-flow"}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	// When FailureReason is empty, onFailure handler should NOT be triggered
	s.Equal(common.NodeStatusFailure, resp.Status, "Status should remain failure when FailureReason is empty")
	s.Empty(resp.NextNodeID, "NextNodeID should not be set when FailureReason is empty")
}

func (s *TaskExecutionNodeTestSuite) TestExecuteFailureWithOnFailureClearsNodeInputs() {
	mockExec := NewExecutorInterfaceMock(s.T())

	inputs := []common.Input{
		{Identifier: "email", Required: true},
	}

	node := newTaskExecutionNode("task-1", map[string]interface{}{}, false, false)
	execNode, _ := node.(ExecutorBackedNodeInterface)

	execNode.SetOnFailure("prompt-email")
	execNode.(*taskExecutionNode).inputs = inputs

	mockExec.On("GetName").Return("test-executor").Once()
	mockExec.On("Execute", mock.Anything).Return(
		&common.ExecutorResponse{
			Status: common.ExecFailure,
			Error: &serviceerror.ServiceError{
				Error: i18ncore.I18nMessage{DefaultValue: "A user with this email already exists"},
			},
		}, nil,
	).Once()

	execNode.SetExecutor(mockExec)

	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs: map[string]string{
			"email": "existing@example.com",
		},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusForward, resp.Status)
	s.Equal("prompt-email", resp.NextNodeID)
	var svcErr698 serviceerror.ServiceError
	s.NoError(json.Unmarshal([]byte(resp.RuntimeData["failureReasonJSON"]), &svcErr698))
	s.Equal("A user with this email already exists", svcErr698.Error.DefaultValue)
	s.Empty(ctx.UserInputs["email"], "Email should be cleared from UserInputs on onFailure")
}

func (s *TaskExecutionNodeTestSuite) TestExecuteFailureWithOnFailureNoNodeInputsPreservesUserInputs() {
	mockExec := NewExecutorInterfaceMock(s.T())

	node := newTaskExecutionNode("task-1", map[string]interface{}{}, false, false)
	execNode, _ := node.(ExecutorBackedNodeInterface)

	execNode.SetOnFailure("error-handler")
	// No inputs configured on the node

	mockExec.On("GetName").Return("test-executor").Once()
	mockExec.On("Execute", mock.Anything).Return(
		&common.ExecutorResponse{
			Status: common.ExecFailure,
			Error:  &serviceerror.ServiceError{Error: i18ncore.I18nMessage{DefaultValue: "SOME_ERROR"}},
		}, nil,
	).Once()

	execNode.SetExecutor(mockExec)

	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs: map[string]string{
			"email": "user@example.com",
		},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusForward, resp.Status)
	s.Equal("error-handler", resp.NextNodeID)
	s.Equal("user@example.com", ctx.UserInputs["email"],
		"UserInputs should be preserved when no node inputs are configured")
}

func (s *TaskExecutionNodeTestSuite) TestOnIncomplete() {
	node := newTaskExecutionNode("task-1", map[string]interface{}{}, false, false)
	execNode, ok := node.(ExecutorBackedNodeInterface)
	s.True(ok)

	// Test default onIncomplete is empty
	s.Empty(execNode.GetOnIncomplete())

	// Test setting onIncomplete
	execNode.SetOnIncomplete("prompt-node")
	s.Equal("prompt-node", execNode.GetOnIncomplete())

	// Test updating onIncomplete
	execNode.SetOnIncomplete("another-prompt-node")
	s.Equal("another-prompt-node", execNode.GetOnIncomplete())
}

func (s *TaskExecutionNodeTestSuite) TestExecuteIncompleteWithOnIncompleteHandler() {
	mockExec := NewExecutorInterfaceMock(s.T())
	node := newTaskExecutionNode("task-1", map[string]interface{}{}, false, false)
	execNode, _ := node.(ExecutorBackedNodeInterface)

	execNode.SetOnIncomplete("prompt-credentials")

	mockExec.On("GetName").Return("test-executor").Once()
	mockExec.On("Execute", mock.Anything).Return(
		&common.ExecutorResponse{
			Status: common.ExecUserInputRequired,
			Inputs: []common.Input{{Identifier: "username", Required: true}},
		}, nil,
	).Once()

	execNode.SetExecutor(mockExec)

	ctx := &NodeContext{ExecutionID: "test-flow"}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusForward, resp.Status, "Should forward to onIncomplete node")
	s.Equal("prompt-credentials", resp.NextNodeID)
}

func (s *TaskExecutionNodeTestSuite) TestExecuteIncompleteWithOnIncompleteAndFailureReason() {
	mockExec := NewExecutorInterfaceMock(s.T())

	inputs := []common.Input{
		{Identifier: "username", Required: true},
		{Identifier: "password", Required: true},
	}

	node := newTaskExecutionNode("task-1", map[string]interface{}{}, false, false)
	execNode, _ := node.(ExecutorBackedNodeInterface)

	execNode.SetOnIncomplete("prompt-credentials")
	execNode.(*taskExecutionNode).inputs = inputs

	mockExec.On("GetName").Return("test-executor").Once()
	mockExec.On("Execute", mock.Anything).Return(
		&common.ExecutorResponse{
			Status: common.ExecUserInputRequired,
			Inputs: inputs,
			Error: &serviceerror.ServiceError{
				Error: i18ncore.I18nMessage{DefaultValue: "Invalid credentials provided"},
			},
			RuntimeData: map[string]string{"existing": "data"},
		}, nil,
	).Once()

	execNode.SetExecutor(mockExec)

	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs: map[string]string{
			"username": "testuser",
			"password": "wrongpassword",
		},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusForward, resp.Status, "Should forward to onIncomplete node")
	s.Equal("prompt-credentials", resp.NextNodeID)
	s.Equal("Invalid credentials provided", resp.Error.Error.DefaultValue)
	var svcErr822 serviceerror.ServiceError
	s.NoError(json.Unmarshal([]byte(resp.RuntimeData["failureReasonJSON"]), &svcErr822))
	s.Equal("Invalid credentials provided", svcErr822.Error.DefaultValue,
		"Failure reason should be propagated to RuntimeData")
	s.Equal("data", resp.RuntimeData["existing"], "Existing runtime data should be preserved")

	// Verify user inputs consumed by the executor are cleared
	s.Empty(ctx.UserInputs["username"], "Username should be cleared from UserInputs")
	s.Empty(ctx.UserInputs["password"], "Password should be cleared from UserInputs")
}

func (s *TaskExecutionNodeTestSuite) TestExecuteIncompleteWithOnIncompleteAndFailureReasonNilRuntimeData() {
	mockExec := NewExecutorInterfaceMock(s.T())

	inputs := []common.Input{
		{Identifier: "username", Required: true},
	}

	node := newTaskExecutionNode("task-1", map[string]interface{}{}, false, false)
	execNode, _ := node.(ExecutorBackedNodeInterface)

	execNode.SetOnIncomplete("prompt-credentials")
	execNode.(*taskExecutionNode).inputs = inputs

	mockExec.On("GetName").Return("test-executor").Once()
	mockExec.On("Execute", mock.Anything).Return(
		&common.ExecutorResponse{
			Status:      common.ExecUserInputRequired,
			Inputs:      inputs,
			Error:       &serviceerror.ServiceError{Error: i18ncore.I18nMessage{DefaultValue: "User not found"}},
			RuntimeData: nil, // nil RuntimeData
		}, nil,
	).Once()

	execNode.SetExecutor(mockExec)

	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs: map[string]string{
			"username": "nonexistent",
		},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusForward, resp.Status)
	s.Equal("prompt-credentials", resp.NextNodeID)
	s.NotNil(resp.RuntimeData, "RuntimeData should be initialized when nil")
	var svcErr869 serviceerror.ServiceError
	s.NoError(json.Unmarshal([]byte(resp.RuntimeData["failureReasonJSON"]), &svcErr869))
	s.Equal("User not found", svcErr869.Error.DefaultValue,
		"Failure reason should be stored even when RuntimeData was nil")
	s.Empty(ctx.UserInputs["username"], "Username should be cleared from UserInputs")
}

func (s *TaskExecutionNodeTestSuite) TestExecuteIncompleteWithOnIncompleteNoFailureReason() {
	mockExec := NewExecutorInterfaceMock(s.T())
	node := newTaskExecutionNode("task-1", map[string]interface{}{}, false, false)
	execNode, _ := node.(ExecutorBackedNodeInterface)

	execNode.SetOnIncomplete("prompt-credentials")
	execNode.(*taskExecutionNode).inputs = []common.Input{
		{Identifier: "username", Required: true},
	}

	mockExec.On("GetName").Return("test-executor").Once()
	mockExec.On("Execute", mock.Anything).Return(
		&common.ExecutorResponse{
			Status: common.ExecUserInputRequired,
			Inputs: []common.Input{{Identifier: "username", Required: true}},
			// No FailureReason
		}, nil,
	).Once()

	execNode.SetExecutor(mockExec)

	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs: map[string]string{
			"username": "testuser",
		},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusForward, resp.Status)
	s.Equal("prompt-credentials", resp.NextNodeID)
	s.Nil(resp.Error, "No failure error should be set")
	// UserInputs should NOT be cleared when there's no failure reason
	s.Equal("testuser", ctx.UserInputs["username"],
		"UserInputs should not be cleared without failure reason")
}

func (s *TaskExecutionNodeTestSuite) TestExecuteUserInputRequiredWithNoInputsReturnsServerError() {
	mockExec := NewExecutorInterfaceMock(s.T())
	node := newTaskExecutionNode("task-1", map[string]interface{}{}, false, false)
	execNode, _ := node.(ExecutorBackedNodeInterface)

	mockExec.On("GetName").Return("test-executor").Once()
	mockExec.On("Execute", mock.Anything).Return(
		&common.ExecutorResponse{
			Status: common.ExecUserInputRequired,
			// No Inputs — broken executor implementation
		}, nil,
	).Once()

	execNode.SetExecutor(mockExec)

	ctx := &NodeContext{ExecutionID: "test-flow"}
	resp, err := node.Execute(ctx)

	s.NotNil(err, "Should return a server error when executor returns VIEW with no inputs")
	s.Nil(resp)
}

func (s *TaskExecutionNodeTestSuite) TestGetExecutionPolicy_NoExecutorReturnsNil() {
	node := newTaskExecutionNode("task-1", make(map[string]interface{}), false, false)

	policy := node.GetExecutionPolicy()
	s.Nil(policy)
}

func (s *TaskExecutionNodeTestSuite) TestGetExecutionPolicy_DelegatedToExecutor() {
	node := newTaskExecutionNode("task-1", make(map[string]interface{}), false, false)
	execNode, _ := node.(ExecutorBackedNodeInterface)

	mockExecutor := NewExecutorInterfaceMock(s.T())
	expectedPolicy := &ExecutionPolicy{SkipChallengeValidation: true}

	mockExecutor.On("GetName").Return("test-executor")
	mockExecutor.On("GetExecutionPolicy", "verify").Return(expectedPolicy)

	execNode.SetMode("verify")
	execNode.SetExecutor(mockExecutor)

	policy := node.GetExecutionPolicy()
	s.NotNil(policy)
	s.True(policy.SkipChallengeValidation)
}

func (s *TaskExecutionNodeTestSuite) TestGetExecutionPolicy_ExecutorReturnsNil() {
	node := newTaskExecutionNode("task-1", make(map[string]interface{}), false, false)
	execNode, _ := node.(ExecutorBackedNodeInterface)

	mockExecutor := NewExecutorInterfaceMock(s.T())

	mockExecutor.On("GetName").Return("test-executor")
	mockExecutor.On("GetExecutionPolicy", "process").Return(nil)

	execNode.SetMode("process")
	execNode.SetExecutor(mockExecutor)

	policy := node.GetExecutionPolicy()
	s.Nil(policy)
}

func (s *TaskExecutionNodeTestSuite) TestGetExecutionPolicy_DifferentModes() {
	testCases := []struct {
		mode     string
		expected *ExecutionPolicy
	}{
		{
			mode:     "generate",
			expected: nil,
		},
		{
			mode:     "verify",
			expected: &ExecutionPolicy{SkipChallengeValidation: true},
		},
		{
			mode:     "validate",
			expected: &ExecutionPolicy{SkipChallengeValidation: false},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.mode, func() {
			node := newTaskExecutionNode("task-1", make(map[string]interface{}), false, false)
			execNode, _ := node.(ExecutorBackedNodeInterface)

			mockExecutor := NewExecutorInterfaceMock(s.T())
			mockExecutor.On("GetName").Return("test-executor")
			mockExecutor.On("GetExecutionPolicy", tc.mode).Return(tc.expected)

			execNode.SetMode(tc.mode)
			execNode.SetExecutor(mockExecutor)

			policy := node.GetExecutionPolicy()

			if tc.expected == nil {
				s.Nil(policy)
			} else {
				s.NotNil(policy)
				s.Equal(tc.expected.SkipChallengeValidation, policy.SkipChallengeValidation)
			}
		})
	}
}

func (s *TaskExecutionNodeTestSuite) TestGetExecutionPolicy_WithEmptyMode() {
	node := newTaskExecutionNode("task-1", make(map[string]interface{}), false, false)
	execNode, _ := node.(ExecutorBackedNodeInterface)

	mockExecutor := NewExecutorInterfaceMock(s.T())
	expectedPolicy := &ExecutionPolicy{SkipChallengeValidation: false}

	mockExecutor.On("GetName").Return("test-executor")
	mockExecutor.On("GetExecutionPolicy", "").Return(expectedPolicy)

	execNode.SetExecutor(mockExecutor)
	// SetMode is not called, so mode will be empty string

	policy := node.GetExecutionPolicy()
	s.NotNil(policy)
	s.False(policy.SkipChallengeValidation)
}
