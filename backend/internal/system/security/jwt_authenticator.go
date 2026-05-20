/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package security

import (
	"net/http"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/utils"
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
	// Step 1: Extract Bearer token
	authHeader := r.Header.Get(constants.AuthorizationHeaderName)
	token, err := extractToken(authHeader)
	if err != nil {
		return nil, err
	}

	if token == "" {
		return nil, errInvalidToken
	}

	// Step 2: Verify JWT.
	// If a trusted issuer is configured, the server delegates token issuance to it
	// and verifies tokens exclusively against its JWKS. Otherwise, verify with the
	// server's own signing key.
	if config.GetServerRuntime().Config.Server.SecurityConfig.TrustedIssuer.IsConfigured() {
		if !h.verifyFederatedToken(token) {
			return nil, errInvalidToken
		}
	} else {
		if err := h.jwtService.VerifyJWT(token, "", ""); err != nil {
			return nil, errInvalidToken
		}
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

	// Create immutable SecurityContext
	return newSecurityContext(subject, ouID, token, scopes, attributes), nil
}

// verifyFederatedToken checks if the token is from a trusted external issuer and verifies it via JWKS.
// Per RFC 9068 §2.2 and RFC 8707, this validates:
//   - iss: matches the configured trusted issuer
//   - aud: matches this server's own identifier (the resource server)
//   - signature: verified via the auth server's JWKS endpoint
//   - required_claims: each configured claim must match the expected value
func (h *jwtAuthenticator) verifyFederatedToken(token string) (verified bool) {
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
	if svcErr := h.jwtService.VerifyJWTWithJWKS(
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
