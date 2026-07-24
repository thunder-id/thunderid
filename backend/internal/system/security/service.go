/*
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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

// Package security provides authentication and authorization for server APIs.
package security

import (
	"context"
	"net/http"
	"regexp"

	"github.com/thunder-id/thunderid/internal/system/log"
)

const loggerComponentName = "SecurityService"

// SecurityServiceInterface defines the contract for security processing services.
type SecurityServiceInterface interface {
	Process(r *http.Request) (context.Context, error)
}

// RevocationEnforcerInterface rejects tokens whose jti or token family id is on the deny list. It is
// the read-only seam the security layer uses to consult the Resource Server's revocation cache
// without depending on its implementation.
type RevocationEnforcerInterface interface {
	// EnsureNotRevoked returns a non-nil error when the token's jti or its token family id has been
	// revoked. Empty identifiers are each a no-op.
	EnsureNotRevoked(ctx context.Context, jti, tokenFamilyID string) error
}

// securityService orchestrates authentication and authorization for HTTP requests.
type securityService struct {
	authenticators         []AuthenticatorInterface
	revocationEnforcer     RevocationEnforcerInterface
	logger                 *log.Logger
	compiledPaths          []*regexp.Regexp
	compiledAPIPermissions []compiledAPIPermission
}

// newSecurityService creates a new instance of the security service.
//
// Parameters:
//   - authenticators: A slice of AuthenticatorInterface implementations to handle request authentication.
//   - revocationEnforcer: Consulted after authentication to reject revoked tokens.
//   - publicPaths: A slice of string patterns representing paths that are exempt from authentication.
//   - apiPermissions: An ordered slice of API permission entries used for authorization.
//
// Returns:
//   - *securityService: A pointer to the created securityService instance.
//   - error: An error if any of the provided path patterns are invalid and cannot be compiled.
func newSecurityService(authenticators []AuthenticatorInterface, revocationEnforcer RevocationEnforcerInterface,
	publicPaths []string, apiPermissions []apiPermissionEntry) (*securityService, error) {
	compiledPaths, err := compilePathPatterns(publicPaths)
	if err != nil {
		return nil, err
	}

	compiledPerms, err := compileAPIPermissions(apiPermissions)
	if err != nil {
		return nil, err
	}

	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	return &securityService{
		authenticators:         authenticators,
		revocationEnforcer:     revocationEnforcer,
		logger:                 logger,
		compiledPaths:          compiledPaths,
		compiledAPIPermissions: compiledPerms,
	}, nil
}

// Process handles the complete security flow: authentication and authorization.
// Returns an enriched context on success, or an error if authentication or authorization fails.
func (s *securityService) Process(r *http.Request) (context.Context, error) {
	isPublic := s.isPublicPath(r.Context(), r.URL.Path)

	// Check if the request is options (CORS preflight)
	if r.Method == http.MethodOptions {
		return r.Context(), nil
	}

	// Find an authenticator that can process this request
	var authenticator AuthenticatorInterface
	for _, a := range s.authenticators {
		if a.CanHandle(r) {
			authenticator = a
			break
		}
	}

	// If no authenticator found
	if authenticator == nil {
		return s.handleAuthError(r.Context(), isPublic, errNoHandlerFound)
	}

	// Authenticate the request
	securityCtx, err := authenticator.Authenticate(r)
	if err != nil {
		return s.handleAuthError(r.Context(), isPublic, err)
	}

	// Add authentication context to request context if available
	ctx := r.Context()
	if securityCtx != nil {
		ctx = withSecurityContext(ctx, securityCtx)

		// Reject the request when the presented token has been revoked. This runs after successful
		// authentication and is format-agnostic: it enforces on the token's jti and its token family
		// id. A revoked token is surfaced as an invalid token (RFC 6750 §3.1) so the response does not
		// disclose that the token was specifically revoked.
		if err := s.revocationEnforcer.EnsureNotRevoked(ctx, securityCtx.revocationID,
			securityCtx.tokenFamilyID); err != nil {
			return s.handleAuthError(ctx, isPublic, errInvalidToken)
		}
	}

	// Authorize the authenticated principal based on the permissions carried in the security context.
	if err := s.authorize(r.WithContext(ctx)); err != nil {
		return s.handleAuthError(ctx, isPublic, err)
	}

	return ctx, nil
}

// authorize checks whether the permissions stored in the request context satisfy
// the requirements for the requested path using hierarchical scope matching.
func (s *securityService) authorize(r *http.Request) error {
	required := s.getRequiredPermissionForAPI(r.Method, r.URL.Path)
	// Empty required means any authenticated user may access the path.
	if required == "" {
		return nil
	}
	permissions := GetPermissions(r.Context())
	if !HasSufficientPermission(permissions, required) {
		return errInsufficientPermissions
	}
	return nil
}

// getRequiredPermissionForAPI returns the minimum permission required to access the
// given HTTP method + path combination. Returns an empty string for self-service paths
// that any authenticated user may access. Falls back to the root system permission for paths not
// covered by any entry in compiledAPIPermissions.
//
// Matching uses pre-compiled regular expressions evaluated in declaration order;
// the first matching pattern wins. More specific patterns (exact paths, named
// sub-resources) are listed before broader wildcards in apiPermissionEntries to
// ensure correct precedence — no manual prefix arithmetic is required.
func (s *securityService) getRequiredPermissionForAPI(method, path string) string {
	key := method + " " + path
	for _, entry := range s.compiledAPIPermissions {
		if entry.re.MatchString(key) {
			return entry.permission
		}
	}
	return GetSystemRootPermission()
}

// isPublicPath checks if the given request path matches any of the configured public path patterns.
func (s *securityService) isPublicPath(ctx context.Context, requestPath string) bool {
	if len(requestPath) > maxPublicPathLength {
		s.logger.Warn(ctx, "Path length exceeds maximum allowed length",
			log.Int("limit", maxPublicPathLength),
			log.Int("length", len(requestPath)))
		return false
	}

	for _, regex := range s.compiledPaths {
		if regex.MatchString(requestPath) {
			return true
		}
	}

	return false
}

// handleAuthError grants access to public paths (as an internal runtime caller)
// and otherwise propagates the authentication/authorization error.
func (s *securityService) handleAuthError(
	ctx context.Context,
	isPublic bool,
	err error,
) (context.Context, error) {
	if isPublic {
		// Mark the context as a runtime caller so that the authorization layer can grant access.
		return WithRuntimeContext(ctx), nil
	}

	return nil, err
}
