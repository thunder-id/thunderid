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

package flowexec

import (
	"testing"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/mock"

	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
	"github.com/thunder-id/thunderid/tests/mocks/observability/observabilitymock"
)

// setupMockObservability creates a mock observability service for testing
func setupMockObservability(t *testing.T) *observabilitymock.ObservabilityServiceInterfaceMock {
	t.Helper()

	// Initialize runtime with observability enabled
	config.ResetServerRuntime()
	testConfig := &config.Config{
		Observability: engineconfig.ObservabilityConfig{
			Enabled: true,
			Output: engineconfig.ObservabilityOutputConfig{
				Console: engineconfig.ObservabilityConsoleConfig{
					Enabled: true,
					Format:  "json",
				},
			},
		},
	}

	err := config.InitializeServerRuntime("/tmp/test-events", testConfig)
	if err != nil {
		t.Fatalf("Failed to initialize server runtime: %v", err)
	}

	// Create mockery-generated mock
	mockObs := &observabilitymock.ObservabilityServiceInterfaceMock{}

	// Setup common expectations - allow any number of calls
	mockObs.On("IsEnabled").Return(true).Maybe()
	mockObs.On("PublishEvent", mock.Anything, mock.Anything).Return().Maybe()
	mockObs.On("Shutdown").Return().Maybe()

	return mockObs
}

// TestPublishFlowStartedEvent tests the flow started event publishing
func TestPublishFlowStartedEvent(t *testing.T) {
	mockObs := setupMockObservability(t)
	defer mockObs.Shutdown()
	defer config.ResetServerRuntime()

	t.Run("with_authenticated_user", func(t *testing.T) {
		ctx := &EngineContext{
			ExecutionID: "flow-001",
			FlowType:    providers.FlowTypeAuthentication,
			AppID:       "app-001",
			AuthenticatedUser: authncm.AuthenticatedUser{
				IsAuthenticated: true,
				UserID:          "user-123",
			},
			ExecutionHistory: make(map[string]*providers.NodeExecutionRecord),
		}

		// Call the actual function to get code coverage
		publishFlowStartedEvent(ctx, mockObs)

		// Verify mock was called
		mockObs.AssertCalled(t, "IsEnabled")
		mockObs.AssertCalled(t, "PublishEvent", mock.Anything, mock.Anything)
	})

	t.Run("without_authenticated_user", func(t *testing.T) {
		ctx := &EngineContext{
			ExecutionID:      "flow-002",
			FlowType:         providers.FlowTypeRegistration,
			AppID:            "app-002",
			ExecutionHistory: make(map[string]*providers.NodeExecutionRecord),
		}

		// Call the actual function to get code coverage
		publishFlowStartedEvent(ctx, mockObs)

		// Verify mock was called
		mockObs.AssertCalled(t, "IsEnabled")
		mockObs.AssertCalled(t, "PublishEvent", mock.Anything, mock.Anything)
	})
}

// TestPublishFlowCompletedEvent tests the flow completed event publishing
func TestPublishFlowCompletedEvent(t *testing.T) {
	mockObs := setupMockObservability(t)
	defer mockObs.Shutdown()
	defer config.ResetServerRuntime()

	ctx := &EngineContext{
		ExecutionID: "flow-003",
		FlowType:    providers.FlowTypeAuthentication,
		AppID:       "app-003",
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated: true,
			UserID:          "user-456",
		},
		ExecutionHistory: make(map[string]*providers.NodeExecutionRecord),
	}

	flowStartTime := int64(1000)
	flowEndTime := int64(2000)

	// Call the actual function to get code coverage
	publishFlowCompletedEvent(ctx, flowStartTime, flowEndTime, mockObs)

	// Verify mock was called
	mockObs.AssertCalled(t, "IsEnabled")
	mockObs.AssertCalled(t, "PublishEvent", mock.Anything, mock.Anything)
}

// TestPublishFlowFailedEvent tests the flow failed event publishing
func TestPublishFlowFailedEvent(t *testing.T) {
	mockObs := setupMockObservability(t)
	defer mockObs.Shutdown()
	defer config.ResetServerRuntime()

	t.Run("with_error_description", func(t *testing.T) {
		ctx := &EngineContext{
			ExecutionID:      "flow-004",
			FlowType:         providers.FlowTypeAuthentication,
			AppID:            "app-004",
			ExecutionHistory: make(map[string]*providers.NodeExecutionRecord),
		}

		svcErr := &tidcommon.ServiceError{
			Error:            tidcommon.I18nMessage{DefaultValue: "flow_execution_failed"},
			Code:             "FLOW_ERR_001",
			ErrorDescription: tidcommon.I18nMessage{DefaultValue: "Authentication failed due to invalid credentials"},
		}

		flowStartTime := int64(1000)
		flowEndTime := int64(1500)

		// Call the actual function to get code coverage
		publishFlowFailedEvent(ctx, svcErr, flowStartTime, flowEndTime, mockObs)

		// Verify mock was called
		mockObs.AssertCalled(t, "IsEnabled")
		mockObs.AssertCalled(t, "PublishEvent", mock.Anything, mock.Anything)
	})

	t.Run("without_error_description", func(t *testing.T) {
		ctx := &EngineContext{
			ExecutionID:      "flow-005",
			FlowType:         providers.FlowTypeAuthentication,
			AppID:            "app-005",
			ExecutionHistory: make(map[string]*providers.NodeExecutionRecord),
		}

		svcErr := &tidcommon.ServiceError{
			Error: tidcommon.I18nMessage{DefaultValue: "generic_error"},
			Code:  "ERR_002",
		}

		flowStartTime := int64(1000)
		flowEndTime := int64(1300)

		// Call the actual function to get code coverage
		publishFlowFailedEvent(ctx, svcErr, flowStartTime, flowEndTime, mockObs)

		// Verify mock was called
		mockObs.AssertCalled(t, "IsEnabled")
		mockObs.AssertCalled(t, "PublishEvent", mock.Anything, mock.Anything)
	})
}

// TestPublishNodeExecutionStartedEvent tests the node execution started event publishing
func TestPublishNodeExecutionStartedEvent(t *testing.T) {
	mockObs := setupMockObservability(t)
	defer mockObs.Shutdown()
	defer config.ResetServerRuntime()

	t.Run("new_node_execution", func(t *testing.T) {
		node := coremock.NewNodeInterfaceMock(t)
		node.On("GetID").Return("node-001")
		node.On("GetType").Return(common.NodeTypePrompt)

		ctx := &EngineContext{
			ExecutionID:      "flow-006",
			FlowType:         providers.FlowTypeAuthentication,
			AppID:            "app-006",
			ExecutionHistory: make(map[string]*providers.NodeExecutionRecord),
		}

		// Call the actual function to get code coverage
		publishNodeExecutionStartedEvent(ctx, node, mockObs)

		// Verify mock was called
		mockObs.AssertCalled(t, "IsEnabled")
		mockObs.AssertCalled(t, "PublishEvent", mock.Anything, mock.Anything)
	})

	t.Run("retry_node_execution", func(t *testing.T) {
		node := coremock.NewNodeInterfaceMock(t)
		node.On("GetID").Return("node-002")
		node.On("GetType").Return(common.NodeTypeTaskExecution)

		ctx := &EngineContext{
			ExecutionID:      "flow-007",
			FlowType:         providers.FlowTypeAuthentication,
			AppID:            "app-007",
			ExecutionHistory: make(map[string]*providers.NodeExecutionRecord),
		}

		// Simulate retry scenario
		ctx.ExecutionHistory[node.GetID()] = &providers.NodeExecutionRecord{
			NodeID:     node.GetID(),
			NodeType:   string(node.GetType()),
			Step:       1,
			Status:     providers.FlowStatusIncomplete,
			Executions: []providers.ExecutionAttempt{{Attempt: 1, Status: providers.FlowStatusIncomplete}},
			StartTime:  1000,
		}

		// Call the actual function to get code coverage
		publishNodeExecutionStartedEvent(ctx, node, mockObs)

		// Verify mock was called
		mockObs.AssertCalled(t, "IsEnabled")
		mockObs.AssertCalled(t, "PublishEvent", mock.Anything, mock.Anything)
	})
}

// TestPublishNodeExecutionCompletedEvent tests the node execution completed event publishing
func TestPublishNodeExecutionCompletedEvent(t *testing.T) {
	mockObs := setupMockObservability(t)
	defer mockObs.Shutdown()
	defer config.ResetServerRuntime()

	t.Run("node_completed_successfully", func(t *testing.T) {
		node := coremock.NewNodeInterfaceMock(t)
		node.On("GetID").Return("node-003")
		node.On("GetType").Return(common.NodeTypePrompt)

		ctx := &EngineContext{
			ExecutionID: "flow-008",
			FlowType:    providers.FlowTypeAuthentication,
			AppID:       "app-008",
			AuthenticatedUser: authncm.AuthenticatedUser{
				IsAuthenticated: true,
				UserID:          "user-789",
			},
			ExecutionHistory: make(map[string]*providers.NodeExecutionRecord),
		}

		ctx.ExecutionHistory[node.GetID()] = &providers.NodeExecutionRecord{
			NodeID:     node.GetID(),
			NodeType:   string(node.GetType()),
			Step:       1,
			Status:     providers.FlowStatusComplete,
			Executions: []providers.ExecutionAttempt{{Attempt: 1, Status: providers.FlowStatusComplete}},
			StartTime:  1000,
		}

		nodeResp := &common.NodeResponse{Status: common.NodeStatusComplete}
		executionStartTime := int64(1000)
		executionEndTime := int64(1100)

		// Call the actual function to get code coverage
		publishNodeExecutionCompletedEvent(ctx, node, nodeResp, nil, executionStartTime, executionEndTime, mockObs)

		// Verify mock was called
		mockObs.AssertCalled(t, "IsEnabled")
		mockObs.AssertCalled(t, "PublishEvent", mock.Anything, mock.Anything)
	})

	t.Run("node_failed_with_error", func(t *testing.T) {
		node := coremock.NewNodeInterfaceMock(t)
		node.On("GetID").Return("node-004")
		node.On("GetType").Return(common.NodeTypeTaskExecution)

		ctx := &EngineContext{
			ExecutionID:      "flow-009",
			FlowType:         providers.FlowTypeAuthentication,
			AppID:            "app-009",
			ExecutionHistory: make(map[string]*providers.NodeExecutionRecord),
		}

		ctx.ExecutionHistory[node.GetID()] = &providers.NodeExecutionRecord{
			NodeID:     node.GetID(),
			NodeType:   string(node.GetType()),
			Step:       1,
			Status:     providers.FlowStatusError,
			Executions: []providers.ExecutionAttempt{{Attempt: 1, Status: providers.FlowStatusError}},
			StartTime:  1000,
		}

		svcErr := &tidcommon.ServiceError{
			Error:            tidcommon.I18nMessage{DefaultValue: "node_execution_failed"},
			Code:             "NODE_ERR_001",
			ErrorDescription: tidcommon.I18nMessage{DefaultValue: "Task execution failed"},
		}

		executionStartTime := int64(1000)
		executionEndTime := int64(1050)

		// Call the actual function to get code coverage
		publishNodeExecutionCompletedEvent(ctx, node, nil, svcErr, executionStartTime, executionEndTime, mockObs)

		// Verify mock was called
		mockObs.AssertCalled(t, "IsEnabled")
		mockObs.AssertCalled(t, "PublishEvent", mock.Anything, mock.Anything)
	})

	t.Run("node_incomplete_status", func(t *testing.T) {
		node := coremock.NewNodeInterfaceMock(t)
		node.On("GetID").Return("node-005")
		node.On("GetType").Return(common.NodeTypePrompt)

		ctx := &EngineContext{
			ExecutionID:      "flow-010",
			FlowType:         providers.FlowTypeAuthentication,
			AppID:            "app-010",
			ExecutionHistory: make(map[string]*providers.NodeExecutionRecord),
		}

		ctx.ExecutionHistory[node.GetID()] = &providers.NodeExecutionRecord{
			NodeID:     node.GetID(),
			NodeType:   string(node.GetType()),
			Step:       1,
			Status:     providers.FlowStatusIncomplete,
			Executions: []providers.ExecutionAttempt{{Attempt: 1, Status: providers.FlowStatusIncomplete}},
			StartTime:  1000,
		}

		nodeResp := &common.NodeResponse{Status: common.NodeStatusIncomplete}
		executionStartTime := int64(1000)
		executionEndTime := int64(1075)

		// Call the actual function to get code coverage
		publishNodeExecutionCompletedEvent(ctx, node, nodeResp, nil, executionStartTime, executionEndTime, mockObs)

		// Verify mock was called
		mockObs.AssertCalled(t, "IsEnabled")
		mockObs.AssertCalled(t, "PublishEvent", mock.Anything, mock.Anything)
	})
}

// TestObservabilityDisabled verifies that no events are published when observability is disabled
func TestObservabilityDisabled(t *testing.T) {
	config.ResetServerRuntime()
	defer config.ResetServerRuntime()

	testConfig := &config.Config{
		Observability: engineconfig.ObservabilityConfig{
			Enabled: false,
		},
	}

	err := config.InitializeServerRuntime("/tmp/test-disabled", testConfig)
	if err != nil {
		t.Fatalf("Failed to initialize server runtime: %v", err)
	}

	mockObs := &observabilitymock.ObservabilityServiceInterfaceMock{}
	mockObs.On("IsEnabled").Return(false).Maybe()
	mockObs.On("PublishEvent", mock.Anything, mock.Anything).Return().Maybe()

	// Try to publish an event
	ctx := &EngineContext{
		ExecutionID:      "test-flow",
		FlowType:         providers.FlowTypeAuthentication,
		AppID:            "test-app",
		ExecutionHistory: make(map[string]*providers.NodeExecutionRecord),
	}

	publishFlowStartedEvent(ctx, mockObs)

	// Verify IsEnabled was called but PublishEvent was NOT called
	mockObs.AssertCalled(t, "IsEnabled")
	mockObs.AssertNotCalled(t, "PublishEvent", mock.Anything, mock.Anything)
}
