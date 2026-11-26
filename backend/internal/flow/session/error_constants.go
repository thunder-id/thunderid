// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"errors"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

var (
	// errVersionConflict is returned by Update when the optimistic-lock version no longer
	// matches (the row was updated concurrently or no longer exists).
	errVersionConflict = errors.New("session version conflict")

	// errSessionContextTooLarge is returned when a serialized session context exceeds
	// MaxSessionContextBytes. The bounded snapshot keeps the sibling row small.
	errSessionContextTooLarge = errors.New("session context exceeds maximum size")
)

var (
	// ErrorInvalidListFilter is returned unless exactly one of userId and appId is given.
	ErrorInvalidListFilter = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SSM-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.sessionmgt.invalid_filter",
			DefaultValue: "Invalid session filter",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.sessionmgt.invalid_filter_description",
			DefaultValue: "Exactly one of the userId or appId query parameters is required",
		},
	}

	// ErrorInvalidLimit is returned when limit is not an integer between 1 and the maximum page size.
	ErrorInvalidLimit = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SSM-1002",
		Error: tidcommon.I18nMessage{
			Key:          "error.sessionmgt.invalid_limit",
			DefaultValue: "Invalid limit parameter",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.sessionmgt.invalid_limit_description",
			DefaultValue: "The limit parameter must be an integer between 1 and 100",
		},
	}

	// ErrorInvalidOffset is returned when offset is not a non-negative integer.
	ErrorInvalidOffset = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SSM-1003",
		Error: tidcommon.I18nMessage{
			Key:          "error.sessionmgt.invalid_offset",
			DefaultValue: "Invalid offset parameter",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.sessionmgt.invalid_offset_description",
			DefaultValue: "The offset parameter must be a non-negative integer",
		},
	}

	// ErrorMissingUserID is returned when revoking a user's sessions without the userId query parameter.
	ErrorMissingUserID = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SSM-1004",
		Error: tidcommon.I18nMessage{
			Key:          "error.sessionmgt.missing_user_id",
			DefaultValue: "Missing user ID",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.sessionmgt.missing_user_id_description",
			DefaultValue: "The userId query parameter is required",
		},
	}

	// ErrorInvalidExceptCurrent is returned when exceptCurrent is not true or false.
	ErrorInvalidExceptCurrent = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SSM-1005",
		Error: tidcommon.I18nMessage{
			Key:          "error.sessionmgt.invalid_except_current",
			DefaultValue: "Invalid exceptCurrent parameter",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.sessionmgt.invalid_except_current_description",
			DefaultValue: "The exceptCurrent parameter must be true or false",
		},
	}

	// ErrorCurrentSessionUnknown is returned when exceptCurrent is true but the caller's access token
	// does not identify the session it was issued in.
	ErrorCurrentSessionUnknown = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SSM-1006",
		Error: tidcommon.I18nMessage{
			Key:          "error.sessionmgt.current_session_unknown",
			DefaultValue: "Current session unknown",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.sessionmgt.current_session_unknown_description",
			DefaultValue: "The access token does not identify the current session, so it cannot be kept",
		},
	}

	// ErrorDelegatedTokenNotAllowed is returned when a delegated token, issued to a client acting for the user,
	// tries to revoke the user's sessions.
	ErrorDelegatedTokenNotAllowed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SSM-1007",
		Error: tidcommon.I18nMessage{
			Key:          "error.sessionmgt.delegated_token_not_allowed",
			DefaultValue: "Delegated token not allowed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.sessionmgt.delegated_token_not_allowed_description",
			DefaultValue: "A token issued to a client acting for the user cannot revoke the user's sessions",
		},
	}
)
