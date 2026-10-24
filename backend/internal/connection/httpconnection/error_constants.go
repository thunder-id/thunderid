// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package httpconnection

import (
	"errors"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

var (
	// errNameConflict reports that the unique index refused a write because another connection in
	// the deployment has the same name.
	errNameConflict = errors.New("an HTTP connection with the same name already exists")
	// errConnectionNotFound reports that a write matched no connection.
	errConnectionNotFound = errors.New("HTTP connection not found")
)

var (
	// ErrorNotFound is returned when an HTTP connection does not exist.
	ErrorNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CON-1015",
		Error: tidcommon.I18nMessage{
			Key:          "error.connectionservice.http_connection_not_found",
			DefaultValue: "HTTP connection not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.connectionservice.http_connection_not_found_description",
			DefaultValue: "No HTTP connection exists for the supplied identifier",
		},
	}
	// ErrorAlreadyExists is returned when another HTTP connection has the same name.
	ErrorAlreadyExists = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CON-1016",
		Error: tidcommon.I18nMessage{
			Key:          "error.connectionservice.http_connection_already_exists",
			DefaultValue: "An HTTP connection with the same name already exists",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.connectionservice.http_connection_already_exists_description",
			DefaultValue: "Choose a different name for the HTTP connection",
		},
	}
	// ErrorInvalidName is returned when an HTTP connection name is missing or too long.
	ErrorInvalidName = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CON-1017",
		Error: tidcommon.I18nMessage{
			Key:          "error.connectionservice.invalid_http_connection_name",
			DefaultValue: "Invalid HTTP connection name",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.connectionservice.invalid_http_connection_name_description",
			DefaultValue: "The HTTP connection name is required and must be at most 255 characters",
		},
	}
	// ErrorInvalidURL is returned when an HTTP connection URL is not an absolute http or https URL with
	// a valid host and port and no userinfo, or is not https while the connection sends credentials.
	ErrorInvalidURL = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CON-1018",
		Error: tidcommon.I18nMessage{
			Key:          "error.connectionservice.invalid_http_connection_url",
			DefaultValue: "Invalid HTTP connection URL",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.connectionservice.invalid_http_connection_url_description",
			DefaultValue: "The URL must be an absolute http or https URL with a valid host and port and " +
				"no user name or password, and must use https when the connection sends credentials",
		},
	}
	// ErrorInvalidTimeout is returned when an HTTP connection timeout is out of range.
	ErrorInvalidTimeout = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CON-1019",
		Error: tidcommon.I18nMessage{
			Key:          "error.connectionservice.invalid_http_connection_timeout",
			DefaultValue: "Invalid HTTP connection timeout",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.connectionservice.invalid_http_connection_timeout_description",
			DefaultValue: "The timeout must be between 1 and 20000 milliseconds",
		},
	}
	// ErrorInvalidHeaders is returned when an HTTP connection header is malformed, repeated, reserved,
	// or set by the connection's authentication.
	ErrorInvalidHeaders = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CON-1020",
		Error: tidcommon.I18nMessage{
			Key:          "error.connectionservice.invalid_http_connection_headers",
			DefaultValue: "Invalid HTTP connection headers",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.connectionservice.invalid_http_connection_headers_description",
			DefaultValue: "Each header needs a valid name that is not repeated and a value without line " +
				"breaks. Credentials go in authentication, so names that carry them, such as Authorization or " +
				"X-Api-Key, headers the HTTP client manages, such as Host or Content-Type, and headers the " +
				"authentication sets cannot be set.",
		},
	}
	// ErrorInvalidAuthentication is returned when an HTTP connection's authentication is not valid for
	// its method, such as a basic method without a password or with the mask as its password.
	ErrorInvalidAuthentication = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CON-1021",
		Error: tidcommon.I18nMessage{
			Key:          "error.connectionservice.invalid_http_connection_authentication",
			DefaultValue: "Invalid HTTP connection authentication",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.connectionservice.invalid_http_connection_authentication_description",
			DefaultValue: "The authentication must give every field its method requires, and a password " +
				"or token cannot be ******: omit it to keep the stored one. A changed method, username or URL " +
				"origin needs each secret again.",
		},
	}
	// ErrorInvalidDescription is returned when an HTTP connection description is too long.
	ErrorInvalidDescription = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CON-1022",
		Error: tidcommon.I18nMessage{
			Key:          "error.connectionservice.invalid_http_connection_description",
			DefaultValue: "Invalid HTTP connection description",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.connectionservice.invalid_http_connection_description_description",
			DefaultValue: "The HTTP connection description must be at most 500 characters",
		},
	}
)
