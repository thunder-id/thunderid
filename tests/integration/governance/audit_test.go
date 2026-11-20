// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

// Audit event names, target types and outcome keys the audit assertions in GovernanceEventsTestSuite
// use. They mirror the server's audit package, which a separate module cannot import.
const (
	auditUserLock      = "user.lock"
	auditUserUnlock    = "user.unlock"
	auditUserSuspend   = "user.suspend"
	auditUserUnsuspend = "user.unsuspend"

	auditAgentLock      = "agent.lock"
	auditAgentUnlock    = "agent.unlock"
	auditAgentSuspend   = "agent.suspend"
	auditAgentUnsuspend = "agent.unsuspend"

	auditTargetUser  = "user"
	auditTargetAgent = "agent"

	auditResultSucceeded = "succeeded"
	auditResultDenied    = "denied"
	auditResultFailed    = "failed"
)
