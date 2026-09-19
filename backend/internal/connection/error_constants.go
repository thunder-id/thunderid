// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// Client errors for connection operations.
var (
	// ErrorInvalidConnectionCategory is the error returned when the category query parameter
	// on GET /connections is not a recognized category.
	ErrorInvalidConnectionCategory = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CON-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.connectionservice.invalid_category",
			DefaultValue: "Invalid connection category",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.connectionservice.invalid_category_description",
			DefaultValue: "The category must be one of: identity-provider, sms-provider, authorization-pdp",
		},
	}
	// ErrorInvalidLimit is the error returned when an invalid limit query parameter is provided.
	ErrorInvalidLimit = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CON-1002",
		Error: tidcommon.I18nMessage{
			Key:          "error.connectionservice.invalid_limit_parameter",
			DefaultValue: "Invalid limit parameter",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.connectionservice.invalid_limit_parameter_description",
			DefaultValue: "The limit parameter must be a positive integer",
		},
	}
	// ErrorInvalidOffset is the error returned when an invalid offset query parameter is provided.
	ErrorInvalidOffset = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CON-1003",
		Error: tidcommon.I18nMessage{
			Key:          "error.connectionservice.invalid_offset_parameter",
			DefaultValue: "Invalid offset parameter",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.connectionservice.invalid_offset_parameter_description",
			DefaultValue: "The offset parameter must be a non-negative integer",
		},
	}
	// ErrorInvalidRequestFormat is returned when an AuthZEN PDP request body is malformed.
	ErrorInvalidRequestFormat = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CON-1007",
		Error: tidcommon.I18nMessage{
			Key:          "error.connectionservice.invalid_request_format",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.connectionservice.invalid_request_format_description",
			DefaultValue: "The request body is malformed or contains invalid data",
		},
	}
)
