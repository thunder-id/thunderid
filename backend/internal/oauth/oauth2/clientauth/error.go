// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package clientauth

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
)

// authError represents an authentication error.
type authError struct {
	ErrorCode        string
	ErrorDescription string
	StatusCode       int
}

// newAuthError creates a new authentication error.
func newAuthError(errorCode, errorDescription string, statusCode int) *authError {
	return &authError{
		ErrorCode:        errorCode,
		ErrorDescription: errorDescription,
		StatusCode:       statusCode,
	}
}

// Common authentication errors
var (
	errInvalidAuthorizationHeader = newAuthError(
		constants.ErrorInvalidClient,
		"Invalid client credentials",
		http.StatusUnauthorized,
	)
	errInvalidClientCredentials = newAuthError(
		constants.ErrorInvalidClient,
		"Invalid client credentials",
		http.StatusUnauthorized,
	)
	errMultipleAuthMethods = newAuthError(
		constants.ErrorInvalidRequest,
		"Multiple client authentication methods were provided",
		http.StatusBadRequest,
	)
	errMissingClientID = newAuthError(
		constants.ErrorInvalidRequest,
		"Missing client_id parameter",
		http.StatusBadRequest,
	)
	errUnauthorizedAuthMethod = newAuthError(
		constants.ErrorUnauthorizedClient,
		"Client is not allowed to use the specified authentication method",
		http.StatusBadRequest,
	)
	errClientIDMismatch = newAuthError(
		constants.ErrorInvalidRequest,
		"client_id in request body does not match client_id from authentication credentials",
		http.StatusBadRequest,
	)
	errInvalidClientAssertion = newAuthError(
		constants.ErrorInvalidClient,
		"Invalid client assertion",
		http.StatusUnauthorized,
	)
	errClientAuthRequired = newAuthError(
		constants.ErrorInvalidClient,
		"Client authentication is required",
		http.StatusUnauthorized,
	)
	errOUAccessUnresolved = newAuthError(
		constants.ErrorServerError,
		"Failed to resolve the organization unit access",
		http.StatusInternalServerError,
	)
	errClientNotAuthorizedForOU = newAuthError(
		constants.ErrorUnauthorizedClient,
		constants.OUAccessRefusal,
		http.StatusBadRequest,
	)
)
