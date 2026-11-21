// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package manager

// maxLinkingCandidates bounds how many accounts one identity's account-linking lookups can match.
const maxLinkingCandidates = 10

// Audit reasons of a credential check, and of an enrollment refused by a suspension of the identity.
const (
	auditReasonAccountSuspended   = "account_suspended"
	auditReasonAccountLocked      = "account_locked"
	auditReasonSubjectNotAllowed  = "subject_not_allowed"
	auditReasonUserNotFound       = "user_not_found"
	auditReasonInvalidRequest     = "invalid_request"
	auditReasonAmbiguousUser      = "ambiguous_user"
	auditReasonInvalidCredentials = "invalid_credentials"
)
