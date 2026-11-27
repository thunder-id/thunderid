// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package security

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
)

// Initialize creates and returns the security middleware with necessary authenticators. The
// revocationEnforcer is consulted after authentication to reject revoked tokens; expectedAud, if
// non-empty, is the required audience.
//
// managementAPIKeyHash, when configured, admits a second mechanism for the import and the
// variable store APIs only. It is registered ahead of the JWT authenticator and claims every request on
// those paths that carries an API-Key header: a key that does not hash to the digest is refused, not
// passed on. A request without the header falls through to JWT unchanged.
func Initialize(jwtService jwt.JWTServiceInterface, revocationEnforcer RevocationEnforcerInterface,
	expectedAud string, managementAPIKeyHash string,
) (func(http.Handler) http.Handler, error) {
	authenticators := make([]AuthenticatorInterface, 0, 2)

	if digest := managementAPIKeyDigest(managementAPIKeyHash); digest != "" {
		managementAuthenticator, err := newManagementAPIKeyAuthenticator(digest, managementAPIKeyPaths)
		if err != nil {
			return nil, err
		}
		authenticators = append(authenticators, managementAuthenticator)
	}
	authenticators = append(authenticators, newJWTAuthenticator(jwtService, expectedAud))

	securityService, err := newSecurityService(
		authenticators, revocationEnforcer, publicPaths, apiPermissionEntries)
	if err != nil {
		return nil, err
	}
	return middleware(securityService)
}
