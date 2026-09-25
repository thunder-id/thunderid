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
)

// errClientNotAuthorizedForOU is returned when the request names an organization unit the client may
// not act for.
//
// It is byte-identical to what an organization unit that does not exist receives, so the two cannot
// be told apart. It stays distinct from invalid_client, though: a wrong secret is a different
// failure, and collapsing that one too would leave a caller unable to tell a bad credential from a
// missing policy.
func errClientNotAuthorizedForOU(ouID string) *authError {
	return newAuthError(
		constants.ErrorInvalidRequest,
		constants.OUAccessRefusalDescription(ouID),
		http.StatusBadRequest,
	)
}
