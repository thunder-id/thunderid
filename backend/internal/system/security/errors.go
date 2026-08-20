// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package security

import "errors"

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

	// errMissingDeploymentID indicates the instance resolves the deployment id from the token
	// (deployment_id_source: token) but the authenticated caller's token carries no deployment
	// claim. The request is rejected (401) rather than served against the wrong tenant.
	errMissingDeploymentID = errors.New("missing deployment id claim")
)
