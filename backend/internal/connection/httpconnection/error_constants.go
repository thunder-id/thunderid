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
	// errIDConflict reports a create whose ID another connection, stored or declared, already has.
	errIDConflict = errors.New("an HTTP connection with the same ID already exists")
	// errConnectionNotFound reports that a write matched no connection.
	errConnectionNotFound = errors.New("HTTP connection not found")
	// errHTTPConnectionIsImmutable reports a write to a connection defined in a declarative file.
	errHTTPConnectionIsImmutable = errors.New("HTTP connection is immutable")
	// errInvalidDeclarativeData reports a file store entry that does not hold an HTTP connection.
	errInvalidDeclarativeData = errors.New("invalid HTTP connection data")
	// errResultLimitExceededInCompositeMode reports that a composite list exceeds its record cap.
	errResultLimitExceededInCompositeMode = errors.New("result limit exceeded in composite mode")
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
	// ErrorHasBlockingDependencies is returned when an HTTP connection cannot be deleted because other
	// resources use it.
	ErrorHasBlockingDependencies = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CON-1026",
		Error: tidcommon.I18nMessage{
			Key:          "error.connectionservice.http_connection_has_blocking_dependencies",
			DefaultValue: "HTTP connection cannot be deleted",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.connectionservice.http_connection_has_blocking_dependencies_description",
			DefaultValue: "The HTTP connection cannot be deleted because {{param(dependencies)}} depend on it. " +
				"Remove or reassign them first.",
		},
	}
	// ErrorResultLimitExceededInCompositeMode is returned when the database and declarative HTTP
	// connections together exceed the composite-mode record limit.
	ErrorResultLimitExceededInCompositeMode = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CON-1027",
		Error: tidcommon.I18nMessage{
			Key:          "error.connectionservice.http_connection_result_limit_exceeded",
			DefaultValue: "Result limit exceeded in composite mode",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.connectionservice.http_connection_result_limit_exceeded_description",
			DefaultValue: "The total number of HTTP connections exceeds the maximum limit in composite mode",
		},
	}
	// ErrorInvalidOU is the error returned when the organization unit given for an HTTP connection does not exist.
	ErrorInvalidOU = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CON-1028",
		Error: tidcommon.I18nMessage{
			Key:          "error.connectionservice.invalid_http_connection_ou",
			DefaultValue: "Invalid organization unit",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.connectionservice.invalid_http_connection_ou_description",
			DefaultValue: "The organization unit given for the HTTP connection does not exist",
		},
	}
	// ErrorOUUpdateNotAllowed is the error returned when attempting to change the
	// organization unit of an HTTP connection.
	ErrorOUUpdateNotAllowed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CON-1029",
		Error: tidcommon.I18nMessage{
			Key:          "error.connectionservice.http_connection_ou_update_not_allowed",
			DefaultValue: "Organization unit update not allowed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.connectionservice.http_connection_ou_update_not_allowed_description",
			DefaultValue: "The organization unit of an HTTP connection cannot be changed",
		},
	}
	// ErrorOURequired is the error returned when an HTTP connection is created without an organization unit.
	ErrorOURequired = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CON-1030",
		Error: tidcommon.I18nMessage{
			Key:          "error.connectionservice.http_connection_ou_required",
			DefaultValue: "Organization unit required",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.connectionservice.http_connection_ou_required_description",
			DefaultValue: "An HTTP connection needs an organization unit. Give its ouId in the request",
		},
	}
)
