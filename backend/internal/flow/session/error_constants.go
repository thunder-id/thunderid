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

	// errSessionNotFound is returned when the session a load resolved no longer exists, as after a concurrent
	// sign-out.
	errSessionNotFound = errors.New("resolved session no longer exists")

	// errOUNotEligible is returned when the sign-in can no longer reuse the session because of the
	// organization unit it needs.
	errOUNotEligible = errors.New("the sign-in can no longer reuse the session")

	// errNilEventBus is returned when the service is initialized without the event bus it audits on.
	errNilEventBus = errors.New("the SSO session service needs an event bus")
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
)
