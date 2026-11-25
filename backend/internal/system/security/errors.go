// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package security

import (
	"errors"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

var (
	// errUnauthorized indicates that the request lacks valid authentication credentials.
	errUnauthorized = errors.New("unauthorized")

	// errForbidden indicates that the authenticated user lacks sufficient permissions.
	errForbidden = errors.New("forbidden")

	// errInsufficientPermissions indicates that the user's permissions are insufficient for the requested resource.
	errInsufficientPermissions = errors.New("insufficient permissions")

	// errNoHandlerFound indicates that no security handler could process the request.
	errNoHandlerFound = errors.New("no security handler found")

	// errInvalidToken indicates that the provided authentication token is invalid.
	errInvalidToken = errors.New("invalid token")

	// errMissingAuthHeader indicates that the Authorization header is missing.
	errMissingAuthHeader = errors.New("missing authorization header")
)

// ErrorRecentAuthenticationRequired refuses a self-service change whose authentication is older than the
// permission's maxAge. Its description carries the maxAge in the ErrorParamMaxAge param.
var ErrorRecentAuthenticationRequired = tidcommon.ServiceError{
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

// ErrorParamMaxAge is the ErrorRecentAuthenticationRequired param that carries the maxAge in seconds.
const ErrorParamMaxAge = "maxAge"
