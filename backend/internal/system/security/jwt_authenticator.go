// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package security

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// claimJTI is the JWT ID claim (RFC 7519 §4.1.7); its value is recorded as the token's revocation
// identifier for the enforcement step.
const claimJTI = "jti"

// claimTokenFamilyID is the token family id claim (tfid); its value lets the enforcement step reject a
// token whose whole authorization grant has been revoked. It is defined here (not imported from the
// OAuth package) to keep the security layer decoupled from OAuth internals.
const claimTokenFamilyID = "tfid"

// The remaining claims consulted during authentication. Like the two above, they are declared here
// rather than imported from the OAuth package so the security layer stays decoupled from it.
const (
	// claimIssuer is the issuer claim (RFC 7519 §4.1.1), used to tell a self-issued token from a
	// federated one before any of its subject values are trusted for revocation.
	claimIssuer = "iss"

	// claimIssuedAt is the issued-at claim (RFC 7519 §4.1.6); it establishes when the token was minted
	// so boundary revocations can decide whether it predates the revoked action.
	claimIssuedAt = "iat"

	// claimAccessTokenSubject carries the end user behind a token minted for a delegated call. When
	// present it, not sub, is the subject the deny list is evaluated against.
	claimAccessTokenSubject = "access_token_sub"

	// claimClientID names the OAuth client an access token was issued to.
	claimClientID = "client_id"
)

// jwtAuthenticator handles authentication and authorization using JWT Bearer tokens.
type jwtAuthenticator struct {
	jwtService jwt.JWTServiceInterface
}

// newJWTAuthenticator creates a new JWT authenticator.
func newJWTAuthenticator(jwtService jwt.JWTServiceInterface) *jwtAuthenticator {
	return &jwtAuthenticator{
		jwtService: jwtService,
	}
}

// CanHandle checks if the request contains a Bearer token in the Authorization header.
// RFC 7235 §2.1: The authentication scheme token is case-insensitive.
func (h *jwtAuthenticator) CanHandle(r *http.Request) bool {
	authHeader := r.Header.Get(constants.AuthorizationHeaderName)
	return utils.HasPrefixFold(authHeader, constants.AuthSchemeBearer)
}

// Authenticate validates the JWT token and builds a SecurityContext.
func (h *jwtAuthenticator) Authenticate(r *http.Request) (*SecurityContext, error) {
	ctx := r.Context()
	// Step 1: Extract Bearer token
	authHeader := r.Header.Get(constants.AuthorizationHeaderName)
	token, err := extractToken(authHeader)
	if err != nil {
		return nil, err
	}

	if token == "" {
		return nil, errInvalidToken
	}

	// Step 2: Verify the JWT, routing on its issuer. Tokens this server issued
	// are verified with its own signing key; Additionally when a trusted issuer is
	// configured, tokens from that issuer are verified against its JWKS.
	if err := h.verifyToken(ctx, token); err != nil {
		return nil, err
	}

	// Step 3: Decode JWT payload to extract attributes
	attributes, err := jwt.DecodeJWTPayload(token)
	if err != nil {
		return nil, errInvalidToken
	}

	// Step 4: Extract subject information and build SecurityContext
	subject := ""
	if sub, ok := attributes["sub"].(string); ok && sub != "" {
		subject = sub
	}

	ouID := extractAttribute(attributes, "ouId")

	// Step 5: Extract scopes from JWT claims
	scopes := extractScopes(attributes)

	// Create immutable SecurityContext. The jti claim is recorded as the revocation identifier for
	// the enforcement step; it may be empty.
	securityCtx := newSecurityContext(subject, ouID, token, scopes, attributes)
	securityCtx.revocationID = extractAttribute(attributes, claimJTI)
	securityCtx.tokenFamilyID = extractAttribute(attributes, claimTokenFamilyID)
	// Subject-dimension revocation is only meaningful for subjects this deployment issued, so a
	// federated token contributes no revocation subject and is enforced on jti and tfid alone.
	if issuer, _ := attributes[claimIssuer].(string); issuer == config.GetServerRuntime().Config.JWT.Issuer {
		securityCtx.revocationSubject = subject
		accessTokenSubject, _ := attributes[claimAccessTokenSubject].(string)
		if accessTokenSubject != "" {
			securityCtx.revocationSubject = accessTokenSubject
		}
		// The app.key dimension is revoked by the OAuth client the token was issued to. Access tokens
		// carry it in client_id; refresh tokens carry no client_id and hold the owning client in sub,
		// which is only safe to read when access_token_sub marks the token as a refresh token.
		if clientID, _ := attributes[claimClientID].(string); clientID != "" {
			securityCtx.revocationAppKey = clientID
		} else if accessTokenSubject != "" {
			securityCtx.revocationAppKey = subject
		}
		if issuedAt, ok := attributes[claimIssuedAt].(float64); ok {
			securityCtx.establishedAt = time.Unix(int64(issuedAt), 0).UTC()
		}
	}
	return securityCtx, nil
}

// verifyToken verifies the bearer token by routing on its iss claim against
// an explicit allowlist of accepted issuers. Tokens from the configured
// trusted issuer (when set) are verified against its JWKS. Tokens whose iss
// matches this server's own JWT issuer are verified with the local signing
// key. Any other iss is rejected. There is no cross-issuer fallback.
func (h *jwtAuthenticator) verifyToken(ctx context.Context, token string) error {
	trustedIssuer := config.GetServerRuntime().Config.Server.SecurityConfig.TrustedIssuer
	iss := extractIssuer(token)
	switch {
	case trustedIssuer.IsConfigured() && iss == trustedIssuer.Issuer:
		if !h.verifyFederatedToken(ctx, token) {
			return errInvalidToken
		}
	case iss == config.GetServerRuntime().Config.JWT.Issuer:
		if err := h.jwtService.VerifyJWT(ctx, token, "", ""); err != nil {
			return errInvalidToken
		}
	default:
		return errInvalidToken
	}
	return nil
}

// verifyFederatedToken checks if the token is from a trusted external issuer and verifies it via JWKS.
// Per RFC 9068 §2.2 and RFC 8707, this validates:
//   - iss: matches the configured trusted issuer
//   - aud: matches this server's own identifier (the resource server)
//   - signature: verified via the auth server's JWKS endpoint
//   - required_claims: each configured claim must match the expected value
func (h *jwtAuthenticator) verifyFederatedToken(ctx context.Context, token string) (verified bool) {
	trustedIssuer := config.GetServerRuntime().Config.Server.SecurityConfig.TrustedIssuer
	if !trustedIssuer.IsConfigured() {
		return false
	}

	attributes, err := jwt.DecodeJWTPayload(token)
	if err != nil {
		return false
	}

	iss, ok := attributes["iss"].(string)
	if !ok || iss != trustedIssuer.Issuer {
		return false
	}

	// VerifyJWTWithJWKS validates signature, aud (resource server identity), iss, and time claims.
	if svcErr := h.jwtService.VerifyJWTWithJWKS(ctx,
		token, trustedIssuer.JWKSURL, trustedIssuer.Audience, trustedIssuer.Issuer,
	); svcErr != nil {
		return false
	}

	// Validate required claims — each configured claim must be present with the expected value.
	for _, rc := range trustedIssuer.RequiredClaims {
		val, ok := attributes[rc.Claim].(string)
		if !ok || val != rc.Value {
			return false
		}
	}

	return true
}

// extractToken extracts the Bearer token from the Authorization header.
func extractToken(authHeader string) (string, error) {
	if !utils.HasPrefixFold(authHeader, constants.AuthSchemeBearer) {
		return "", errMissingAuthHeader
	}
	token := strings.TrimSpace(utils.TrimPrefixFold(authHeader, constants.AuthSchemeBearer))
	return token, nil
}

// extractIssuer returns the iss claim of the token, or an empty string if the
// token payload cannot be decoded or carries no string iss claim.
func extractIssuer(token string) string {
	attributes, err := jwt.DecodeJWTPayload(token)
	if err != nil {
		return ""
	}
	iss, _ := attributes["iss"].(string)
	return iss
}

// extractScopes extracts permissions from JWT claims.
// Permissions can be in "scope" (string with space-separated values), "scopes" (array) claim,
// or "authorized_permissions" (server-specific) claim.
func extractScopes(attributes map[string]interface{}) []string {
	// Try "scope" claim (OAuth2 standard - space-separated string)
	if scopeStr, ok := attributes["scope"].(string); ok && scopeStr != "" {
		return strings.Fields(scopeStr)
	}

	// Try "scopes" claim (array format)
	if scopesRaw, ok := attributes["scopes"]; ok {
		switch scopes := scopesRaw.(type) {
		case []interface{}:
			result := make([]string, 0, len(scopes))
			for _, s := range scopes {
				if str, ok := s.(string); ok {
					result = append(result, str)
				}
			}
			return result
		case []string:
			return scopes
		}
	}

	// Try "authorized_permissions" from the server assertion
	if permsStr, ok := attributes["authorized_permissions"].(string); ok && permsStr != "" {
		return strings.Fields(permsStr)
	}

	return []string{}
}

// extractAttribute extracts a string claim from JWT claims map.
func extractAttribute(attributes map[string]interface{}, key string) string {
	if value, ok := attributes[key].(string); ok {
		return value
	}
	return ""
}
