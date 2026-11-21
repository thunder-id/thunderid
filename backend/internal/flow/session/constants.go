// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package session

// maxUserAgentLength is the most characters of a User-Agent a session's properties record.
const maxUserAgentLength = 512

const (
	// auditReasonSubjectMismatch is the audit reason of a session join refused because the authenticated
	// subject is not the subject of the session.
	auditReasonSubjectMismatch = "subject_mismatch"
	// auditReasonOUNotEligible is the audit reason of a session reuse refused because the sign-in can no
	// longer land in the organization unit it needs.
	auditReasonOUNotEligible = "ou_not_eligible"
	// auditReasonSessionNotFound is the audit reason of a session reuse that found its session gone.
	auditReasonSessionNotFound = "session_not_found"
)

// terminationReasonApplicationDetached is the end_reason of a session deleted because its last participating
// application was detached. It is audited only: no termination listener is notified of it.
const terminationReasonApplicationDetached TerminationReason = "application_detached"
