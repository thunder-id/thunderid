// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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

	// Back-Channel Logout Events
	// EventTypeBackchannelLogoutDelivered is triggered when a relying party accepts a logout token.
	EventTypeBackchannelLogoutDelivered providers.EventType = "BACKCHANNEL_LOGOUT_DELIVERED"
	// EventTypeBackchannelLogoutFailed is triggered when a logout token delivery attempt fails or is abandoned.
	EventTypeBackchannelLogoutFailed providers.EventType = "BACKCHANNEL_LOGOUT_FAILED"

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
