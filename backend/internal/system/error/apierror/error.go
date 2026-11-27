// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package apierror defines the error structures for the API.
package apierror

import (
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// ErrorResponse defines an API error response with i18n support.
type ErrorResponse struct {
	Code        string                `json:"code"`
	Message     tidcommon.I18nMessage `json:"message"`
	Description tidcommon.I18nMessage `json:"description"`
}

// Authentication and authorization error responses, returned by the security middleware.
var (
	// ErrUnauthorized is returned when authentication credentials are missing or invalid (HTTP 401).
	ErrUnauthorized = ErrorResponse{
		Code: "AUTH-4010",
		Message: tidcommon.I18nMessage{
			Key:          "error.auth.unauthorized",
			DefaultValue: "Unauthorized",
		},
		Description: tidcommon.I18nMessage{
			Key:          "error.auth.unauthorized_description",
			DefaultValue: "Authentication is required to access this resource",
		},
	}

	// ErrForbidden is returned when the caller is authenticated but lacks sufficient permissions (HTTP 403).
	ErrForbidden = ErrorResponse{
		Code: "AUTH-4030",
		Message: tidcommon.I18nMessage{
			Key:          "error.auth.forbidden",
			DefaultValue: "Forbidden",
		},
		Description: tidcommon.I18nMessage{
			Key:          "error.auth.forbidden_description",
			DefaultValue: "You do not have sufficient permissions to access this resource",
		},
	}
)

// ErrorParamMaxAge is the ErrRecentAuthenticationRequired param that carries the maximum age in seconds.
const ErrorParamMaxAge = "maxAge"

// ErrRecentAuthenticationRequired refuses a self-service change when the sign-in is older than the maximum
// authentication age (HTTP 401). It is a service error, since its description carries that age in the
// ErrorParamMaxAge param.
var ErrRecentAuthenticationRequired = tidcommon.ServiceError{
	Type: tidcommon.ClientErrorType,
	Code: "AUTH-4011",
	Error: tidcommon.I18nMessage{
		Key:          "error.security.recent_authentication_required",
		DefaultValue: "Recent authentication required",
	},
	ErrorDescription: tidcommon.I18nMessage{
		Key: "error.security.recent_authentication_required_description",
		DefaultValue: "Authenticate again: this change needs an authentication within the last " +
			"{{param(maxAge)}} seconds",
	},
}
