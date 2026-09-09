// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package tokenservice

import "errors"

// Reasons a token failed validation, discriminated by callers with errors.Is to pick a specific error_description.
var (
	// ErrTokenExpired indicates the token's exp claim is in the past.
	ErrTokenExpired = errors.New("token has expired")

	// ErrIssuerNotTrusted indicates the token's iss claim resolves to neither this server nor an
	// identity provider registered as a trusted issuer for the grant being used.
	ErrIssuerNotTrusted = errors.New("token issuer is not trusted")

	// ErrAudienceNotAccepted indicates the token's aud claim names none of the audiences the grant
	// accepts, such as this server's issuer or an identity provider's trusted token audience.
	ErrAudienceNotAccepted = errors.New("token audience is not accepted")

	// ErrAssertionReplayed indicates the assertion's jti has already been recorded in the replay cache.
	ErrAssertionReplayed = errors.New("assertion has already been used")

	// ErrAuthorizationMappingUnavailable indicates authorization mapping resolution failed because a
	// dependency (the role, group, or resource service) was unavailable, rather than because the
	// token's issuer is untrusted. Callers must map this to server_error, not to an issuer-trust or
	// request-validation failure.
	ErrAuthorizationMappingUnavailable = errors.New("authorization mapping resolution unavailable")
)
