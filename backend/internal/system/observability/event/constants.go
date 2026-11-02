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
	// ComponentBackchannelLogout identifies the back-channel logout dispatcher.
	ComponentBackchannelLogout = "BackchannelLogout"
	// ComponentExtensionExecutor identifies the flow step that calls an organization's service.
	ComponentExtensionExecutor = "ExtensionExecutor"
	// ComponentIdentityGovernance identifies the identity governance service.
	ComponentIdentityGovernance = "IdentityGovernance"
	// ComponentServerConfig identifies the server-config service.
	ComponentServerConfig = "ServerConfig"
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

	// EventTypeTokenRevocationFailed is triggered when a criteria revocation cannot be stored.
	EventTypeTokenRevocationFailed providers.EventType = "TOKEN_REVOCATION_FAILED" //nolint:gosec

	// EventTypeRuntimePersistentDBUnavailable is triggered when the runtime persistent database backing the
	// deny-list (revocation) check becomes unavailable and enforcement fails closed.
	EventTypeRuntimePersistentDBUnavailable providers.EventType = "RUNTIME_PERSISTENT_DB_UNAVAILABLE"

	// EventTypeIdentityLocked is triggered when failed authentication attempts form an automatic lock.
	EventTypeIdentityLocked providers.EventType = "IDENTITY_LOCKED"

	// EventTypeIdentitySelfUnlocked is triggered when a recovery flow releases the holder's own locks.
	EventTypeIdentitySelfUnlocked providers.EventType = "IDENTITY_SELF_UNLOCKED"

	// EventTypeIdentitySelfUnlockFailed is triggered when a recovery flow's unlock is refused or fails.
	EventTypeIdentitySelfUnlockFailed providers.EventType = "IDENTITY_SELF_UNLOCK_FAILED"

	// Administration Events. Each carries the operator as act_sub and the target identity as sub.

	// EventTypeIdentitySuspended is triggered when a suspension is placed or repeated.
	EventTypeIdentitySuspended providers.EventType = "IDENTITY_SUSPENDED"
	// EventTypeIdentitySuspensionFailed is triggered when a suspension is refused or cannot be written.
	EventTypeIdentitySuspensionFailed providers.EventType = "IDENTITY_SUSPENSION_FAILED"
	// EventTypeIdentityUnsuspended is triggered when a suspension is released.
	EventTypeIdentityUnsuspended providers.EventType = "IDENTITY_UNSUSPENDED"
	// EventTypeIdentityUnsuspensionFailed is triggered when a release is refused or cannot be written.
	EventTypeIdentityUnsuspensionFailed providers.EventType = "IDENTITY_UNSUSPENSION_FAILED"
	// EventTypeIdentityUnlocked is triggered when every automatic lock on an identity is released.
	EventTypeIdentityUnlocked providers.EventType = "IDENTITY_UNLOCKED"
	// EventTypeIdentityUnlockFailed is triggered when an unlock is refused or cannot be written.
	EventTypeIdentityUnlockFailed providers.EventType = "IDENTITY_UNLOCK_FAILED"
	// EventTypeServerConfigUpdated is triggered when a server-config section's writable layer is replaced.
	EventTypeServerConfigUpdated providers.EventType = "SERVER_CONFIG_UPDATED"
	// EventTypeServerConfigUpdateFailed is triggered when a server-config write is rejected or fails.
	EventTypeServerConfigUpdateFailed providers.EventType = "SERVER_CONFIG_UPDATE_FAILED"

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

	// EventTypeExtensionCall is triggered once for every execution of an extension step, whatever its
	// outcome.
	EventTypeExtensionCall providers.EventType = "EXTENSION_CALL"
)
