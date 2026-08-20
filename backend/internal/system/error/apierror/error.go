// Copyright 2025 The ThunderID Authors
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

	// ErrNotFound is returned when the requested resource or endpoint is not available on this
	// instance (HTTP 404), for example a management route on a data-plane instance.
	ErrNotFound = ErrorResponse{
		Code: "AUTH-4040",
		Message: tidcommon.I18nMessage{
			Key:          "error.auth.not_found",
			DefaultValue: "Not Found",
		},
		Description: tidcommon.I18nMessage{
			Key:          "error.auth.not_found_description",
			DefaultValue: "The requested endpoint is not available on this instance",
		},
	}
)
