// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package common provides error constants for the common module.
package common

// ServiceErrorType defines the type of service error.
type ServiceErrorType string

const (
	// ClientErrorType denotes the client error type.
	ClientErrorType ServiceErrorType = "client_error"
	// ServerErrorType denotes the server error type.
	ServerErrorType ServiceErrorType = "server_error"
)

// Authorization errors
var (
	// ErrorUnauthorized is the error returned when the caller is not authorized to perform the operation.
	ErrorUnauthorized = ServiceError{
		Type: ClientErrorType,
		Code: "SSE-4030",
		Error: I18nMessage{
			Key:          "error.unauthorized",
			DefaultValue: "Unauthorized",
		},
		ErrorDescription: I18nMessage{
			Key:          "error.unauthorized_description",
			DefaultValue: "The caller is not authorized to perform this operation",
		},
	}
)

// Resource server binding errors
var (
	// ErrorAmbiguousPermissions is the error a ResourceServerProvider returns when the requested
	// permissions are defined on more than one resource server.
	ErrorAmbiguousPermissions = ServiceError{
		Type: ClientErrorType,
		Code: "SSE-4220",
		Error: I18nMessage{
			Key:          "error.ambiguous_permissions",
			DefaultValue: "Ambiguous permissions",
		},
		ErrorDescription: I18nMessage{
			Key:          "error.ambiguous_permissions_description",
			DefaultValue: "The requested permissions are defined on more than one resource server",
		},
	}
)

// Server errors
var (
	// InternalServerError is the error returned for unexpected server errors.
	InternalServerError = ServiceError{
		Type: ServerErrorType,
		Code: "SSE-5000",
		Error: I18nMessage{
			Key:          "error.internal_server_error",
			DefaultValue: "Internal server error",
		},
		ErrorDescription: I18nMessage{
			Key:          "error.internal_server_error_description",
			DefaultValue: "An unexpected error occurred while processing the request",
		},
	}

	// ErrorEncodingError is the error returned when encoding the response fails.
	ErrorEncodingError = ServiceError{
		Type: ServerErrorType,
		Code: "SSE-5001",
		Error: I18nMessage{
			Key:          "error.encoding_error",
			DefaultValue: "Encoding error",
		},
		ErrorDescription: I18nMessage{
			Key:          "error.encoding_error_description",
			DefaultValue: "An error occurred while encoding the response",
		},
	}
)
