// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package selfpasskey

import (
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

var (
	// ErrorAuthenticationRequired is returned when the request carries no signed-in user.
	ErrorAuthenticationRequired = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SPK-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.selfpasskeyservice.authentication_required",
			DefaultValue: "Authentication required",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.selfpasskeyservice.authentication_required_description",
			DefaultValue: "Sign in to manage your passkeys",
		},
	}
	// ErrorDelegatedTokenNotAllowed is returned when the token acts on behalf of the user.
	ErrorDelegatedTokenNotAllowed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SPK-1002",
		Error: tidcommon.I18nMessage{
			Key:          "error.selfpasskeyservice.delegated_token_not_allowed",
			DefaultValue: "Delegated token not allowed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.selfpasskeyservice.delegated_token_not_allowed_description",
			DefaultValue: "Passkeys can only be managed with a token issued to the user directly",
		},
	}
	// ErrorRelyingPartyNotAllowed is returned when the relying party ID does not match the allowed origins.
	ErrorRelyingPartyNotAllowed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SPK-1003",
		Error: tidcommon.I18nMessage{
			Key:          "error.selfpasskeyservice.relying_party_not_allowed",
			DefaultValue: "Relying party not allowed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.selfpasskeyservice.relying_party_not_allowed_description",
			DefaultValue: "The relying party ID must be the application's passkey relying party ID, or match the " +
				"host, or a parent domain, of an allowed passkey origin",
		},
	}
	// ErrorInvalidRequestFormat is returned when the request body cannot be read.
	ErrorInvalidRequestFormat = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SPK-1004",
		Error: tidcommon.I18nMessage{
			Key:          "error.selfpasskeyservice.invalid_request_format",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.selfpasskeyservice.invalid_request_format_description",
			DefaultValue: "The request body is malformed or contains invalid data",
		},
	}
)
