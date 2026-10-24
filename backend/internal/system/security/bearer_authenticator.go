// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package security

import (
	"context"

	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
)

// BearerAuthenticator verifies bearer tokens and enforces revocation — the single authentication
// implementation shared by every HTTP surface that accepts this server's tokens (the REST API gate
// and the MCP server). A token rejected by one is rejected by the other, for the same reason.
type BearerAuthenticator struct {
	jwtService         jwt.JWTServiceInterface
	revocationEnforcer RevocationEnforcerInterface
	expectedAud        string
}

// NewBearerAuthenticator creates a new BearerAuthenticator. expectedAud, if non-empty, is required
// as the audience (RFC 8707 resource indicator) of a self-issued token and of a trusted-issuer
// token alike. Pass "" to skip the self-issued check; a trusted-issuer token is then checked
// against trusted_issuer.audience.
func NewBearerAuthenticator(
	jwtService jwt.JWTServiceInterface, revocationEnforcer RevocationEnforcerInterface, expectedAud string,
) *BearerAuthenticator {
	return &BearerAuthenticator{
		jwtService: jwtService, revocationEnforcer: revocationEnforcer, expectedAud: expectedAud,
	}
}

// Authenticate verifies token — routing on issuer exactly as the REST gate does (self-issued, or a
// configured trusted issuer verified via JWKS), and against expectedAud if this authenticator was
// constructed with one, for trusted-issuer tokens too — and rejects it if revocationEnforcer
// reports it as revoked. It returns the resulting SecurityContext on success.
func (a *BearerAuthenticator) Authenticate(ctx context.Context, token string) (*SecurityContext, error) {
	securityCtx, err := AuthenticateBearerToken(ctx, a.jwtService, token, a.expectedAud)
	if err != nil {
		return nil, err
	}

	// Revoked tokens are rejected as invalid, not disclosed as specifically revoked — same as the
	// REST gate's securityService.Process.
	if err := a.revocationEnforcer.EnsureNotRevoked(ctx, RevocationIdentity{
		JTI:           securityCtx.revocationID,
		TokenFamilyID: securityCtx.tokenFamilyID,
		Subject:       securityCtx.revocationSubject,
		EstablishedAt: securityCtx.establishedAt,
	}); err != nil {
		return nil, errInvalidToken
	}

	return securityCtx, nil
}
