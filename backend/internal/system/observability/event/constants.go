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

package event

import "github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

// Component name constants for event sources.
// These identify which component/module is emitting the event.
const (
	// ComponentFlowEngine identifies events from the flow execution engine.
	ComponentFlowEngine = "FlowEngine"

	// ComponentAuthHandler identifies events from authentication handlers.
	ComponentAuthHandler = "AuthHandler"
)

// Authentication and Authorization Event Types
const (
	// Token Issuance Events

	// EventTypeTokenIssuanceStarted is triggered when token issuance begins.
	EventTypeTokenIssuanceStarted providers.EventType = "TOKEN_ISSUANCE_STARTED" //nolint:gosec

	// EventTypeTokenIssued is triggered when a token is successfully issued.
	EventTypeTokenIssued providers.EventType = "TOKEN_ISSUED"

	// EventTypeTokenIssuanceFailed is triggered when token issuance fails.
	EventTypeTokenIssuanceFailed providers.EventType = "TOKEN_ISSUANCE_FAILED" //nolint:gosec

	// EventTypeTokenRevoked is triggered when a token is revoked (RFC 7009).
	EventTypeTokenRevoked providers.EventType = "TOKEN_REVOKED" //nolint:gosec

	// EventTypeRuntimePersistentDBUnavailable is triggered when the runtime persistent database backing the
	// deny-list (revocation) check becomes unavailable and enforcement fails closed.
	EventTypeRuntimePersistentDBUnavailable providers.EventType = "RUNTIME_PERSISTENT_DB_UNAVAILABLE"

	// Flow Execution Events

	// EventTypeFlowStarted is triggered when a flow execution begins.
	EventTypeFlowStarted providers.EventType = "FLOW_STARTED"

	// EventTypeFlowNodeExecutionStarted is triggered when a flow node execution begins.
	EventTypeFlowNodeExecutionStarted providers.EventType = "FLOW_NODE_EXECUTION_STARTED"

	// EventTypeFlowNodeExecutionCompleted is triggered when a flow node completes.
	EventTypeFlowNodeExecutionCompleted providers.EventType = "FLOW_NODE_EXECUTION_COMPLETED"

	// EventTypeFlowNodeExecutionFailed is triggered when a flow node fails.
	EventTypeFlowNodeExecutionFailed providers.EventType = "FLOW_NODE_EXECUTION_FAILED"

	// EventTypeFlowUserInputRequired is triggered when flow requires user input.
	EventTypeFlowUserInputRequired providers.EventType = "FLOW_USER_INPUT_REQUIRED"

	// EventTypeFlowCompleted is triggered when flow execution succeeds.
	EventTypeFlowCompleted providers.EventType = "FLOW_COMPLETED"

	// EventTypeFlowFailed is triggered when flow execution fails.
	EventTypeFlowFailed providers.EventType = "FLOW_FAILED"
)
