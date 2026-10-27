// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package userinfo

import (
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// UserInfo standard service error constants
var (
	// errorInvalidAccessToken is returned when the access token is invalid, expired, or malformed
	errorInvalidAccessToken = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "invalid_token",
		Error: tidcommon.I18nMessage{
			Key:          "error.userinfoservice.invalid_access_token",
			DefaultValue: "Invalid access token",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.userinfoservice.invalid_access_token_description",
			DefaultValue: "The access token is invalid, expired, or malformed",
		},
	}

	// errorMissingSubClaim is returned when the access token is missing or has an invalid 'sub' claim
	errorMissingSubClaim = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "invalid_token",
		Error: tidcommon.I18nMessage{
			Key:          "error.userinfoservice.missing_sub_claim",
			DefaultValue: "Invalid access token",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.userinfoservice.missing_sub_claim_description",
			DefaultValue: "The access token is missing or has an invalid 'sub' claim",
		},
	}

	// errorAudienceNotAccepted is returned when the access token's audience does not match the
	// client's own default audience, i.e. the token was minted for a different resource server via
	// the 'resource' parameter and must not be redeemable at the UserInfo endpoint.
	errorAudienceNotAccepted = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "invalid_token",
		Error: tidcommon.I18nMessage{
			Key:          "error.userinfoservice.audience_not_accepted",
			DefaultValue: "Invalid access token",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.userinfoservice.audience_not_accepted_description",
			DefaultValue: "The access token audience is not accepted by the UserInfo endpoint",
		},
	}

	// errorClientCredentialsNotSupported is returned when the access token was issued using client_credentials grant
	errorClientCredentialsNotSupported = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "invalid_token",
		Error: tidcommon.I18nMessage{
			Key:          "error.userinfoservice.client_credentials_not_supported",
			DefaultValue: "Invalid access token",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.userinfoservice.client_credentials_not_supported_description",
			DefaultValue: "UserInfo endpoint is not applicable for client_credentials grant type",
		},
	}

	// errorInsufficientScope is returned when the access token lacks the required 'openid' scope
	errorInsufficientScope = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "insufficient_scope",
		Error: tidcommon.I18nMessage{
			Key:          "error.userinfoservice.insufficient_scope",
			DefaultValue: "Insufficient scope",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.userinfoservice.insufficient_scope_description",
			DefaultValue: "The 'openid' scope is required for this request",
		},
	}

	// errorBearerDowngrade is returned when a DPoP-bound access token is presented
	// under the Bearer scheme.
	errorBearerDowngrade = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "invalid_token",
		Error: tidcommon.I18nMessage{
			Key:          "error.userinfoservice.dpop_bound_token_bearer_scheme",
			DefaultValue: "Invalid access token",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.userinfoservice.dpop_bound_token_bearer_scheme_description",
			DefaultValue: "DPoP-bound token must use DPoP scheme",
		},
	}

	// errorDPoPProofInvalid is returned when the DPoP-scheme request fails to bind:
	// access token not DPoP-bound, or proof verification fails.
	errorDPoPProofInvalid = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "invalid_token",
		Error: tidcommon.I18nMessage{
			Key:          "error.userinfoservice.invalid_dpop_proof",
			DefaultValue: "Invalid access token",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.userinfoservice.invalid_dpop_proof_description",
			DefaultValue: "DPoP proof verification failed",
		},
	}

	// errorRevocationUnavailable is returned when the token revocation deny list could not be
	// consulted. The validator fails closed, so the request is rejected with a server error rather
	// than served from a token whose revocation status is unknown.
	errorRevocationUnavailable = tidcommon.ServiceError{
		Type: tidcommon.ServerErrorType,
		Code: constants.ErrorServerError,
		Error: tidcommon.I18nMessage{
			Key:          "error.userinfoservice.revocation_unavailable",
			DefaultValue: "Token revocation status could not be verified",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.userinfoservice.revocation_unavailable_description",
			DefaultValue: "The token revocation status could not be verified",
		},
	}
)
