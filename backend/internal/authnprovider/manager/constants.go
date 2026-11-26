// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package manager

import oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"

// maxLinkingCandidates bounds how many accounts one identity's account-linking lookups can match.
const maxLinkingCandidates = 10

// tokenMetadataClaims describe the token rather than the user, so they never fill an attribute.
var tokenMetadataClaims = []string{
	oauth2const.ClaimAud, oauth2const.ClaimExp, oauth2const.ClaimIat, oauth2const.ClaimNbf,
	oauth2const.ClaimIss, oauth2const.ClaimJTI, oauth2const.ClaimAtHash, oauth2const.ClaimCHash,
	oauth2const.ClaimAzp, oauth2const.ClaimNonce,
}

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
