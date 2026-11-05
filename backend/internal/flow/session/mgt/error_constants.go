// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sessionmgt

import tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

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

	// ErrorSessionNotFound is returned when the caller has no session with the given ID.
	ErrorSessionNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SSM-1007",
		Error: tidcommon.I18nMessage{
			Key:          "error.sessionmgt.session_not_found",
			DefaultValue: "Session not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.sessionmgt.session_not_found_description",
			DefaultValue: "No session with the given ID was found",
		},
	}
)
