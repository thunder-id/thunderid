// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package security

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
)

// Initialize creates and returns the security middleware with necessary authenticators. The
// revocationEnforcer is consulted after authentication to reject revoked tokens.
//
// managementToken, when configured, admits a second mechanism for the import and variable store
// APIs only. It is registered ahead of the JWT authenticator because it claims a request only when
// the presented bearer value is that exact token; anything else falls through to JWT unchanged.
func Initialize(jwtService jwt.JWTServiceInterface, revocationEnforcer RevocationEnforcerInterface,
	managementToken string,
) (func(http.Handler) http.Handler, error) {
	authenticators := make([]AuthenticatorInterface, 0, 2)

	if hasManagementToken(managementToken) {
		managementAuthenticator, err := newManagementTokenAuthenticator(managementToken, managementTokenPaths)
		if err != nil {
			return nil, err
		}
		authenticators = append(authenticators, managementAuthenticator)
	}
	authenticators = append(authenticators, newJWTAuthenticator(jwtService))

	securityService, err := newSecurityService(
		authenticators, revocationEnforcer, publicPaths, apiPermissionEntries)
	if err != nil {
		return nil, err
	}
	return middleware(securityService)
}
