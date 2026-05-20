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

// Package security provides authentication and authorization for server APIs.
package security

import (
	"context"
	"net/http"
	"os"
	"regexp"

	"github.com/thunder-id/thunderid/internal/system/log"
)

const loggerComponentName = "SecurityService"

// SecurityServiceInterface defines the contract for security processing services.
type SecurityServiceInterface interface {
	Process(r *http.Request) (context.Context, error)
}

// securityService orchestrates authentication and authorization for HTTP requests.
type securityService struct {
	authenticators         []AuthenticatorInterface
	logger                 *log.Logger
	compiledPaths          []*regexp.Regexp
	compiledAPIPermissions []compiledAPIPermission
	skipSecurity           bool
}

// newSecurityService creates a new instance of the security service.
//
// Parameters:
//   - authenticators: A slice of AuthenticatorInterface implementations to handle request authentication.
//   - publicPaths: A slice of string patterns representing paths that are exempt from authentication.
//   - apiPermissions: An ordered slice of API permission entries used for authorization.
//
// Returns:
//   - *securityService: A pointer to the created securityService instance.
//   - error: An error if any of the provided path patterns are invalid and cannot be compiled.
func newSecurityService(authenticators []AuthenticatorInterface, publicPaths []string,
	apiPermissions []apiPermissionEntry) (*securityService, error) {
	compiledPaths, err := compilePathPatterns(publicPaths)
	if err != nil {
		return nil, err
	}

	compiledPerms, err := compileAPIPermissions(apiPermissions)
	if err != nil {
		return nil, err
	}

	// Check if security enforcement should be skipped via environment variable
	skipSecurity := os.Getenv("SKIP_SECURITY") == "true"

	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	if skipSecurity {
		logger.Warn("============================================================")
		logger.Warn("|       WARNING: SECURITY ENFORCEMENT DISABLED             |")
		logger.Warn("|                                                          |")
		logger.Warn("|        SKIP_SECURITY is set to 'true'            |")
		logger.Warn("|  This is NOT RECOMMENDED for production environments!    |")
		logger.Warn("| Endpoints accessible without auth, but tokens processed  |")
		logger.Warn("|                                                          |")
		logger.Warn("============================================================")
	}

	return &securityService{
		authenticators:         authenticators,
		logger:                 logger,
		compiledPaths:          compiledPaths,
		compiledAPIPermissions: compiledPerms,
		skipSecurity:           skipSecurity,
	}, nil
}

// Process handles the complete security flow: authentication and authorization.
// Returns an enriched context on success, or an error if authentication or authorization fails.
func (s *securityService) Process(r *http.Request) (context.Context, error) {
	isPublic := s.isPublicPath(r.URL.Path)

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
		return s.handleAuthError(r.Context(), r.URL.Path, errNoHandlerFound, isPublic, s.skipSecurity)
	}

	// Authenticate the request
	securityCtx, err := authenticator.Authenticate(r)
	if err != nil {
		return s.handleAuthError(r.Context(), r.URL.Path, err, isPublic, s.skipSecurity)
	}

	// Add authentication context to request context if available
	ctx := r.Context()
	if securityCtx != nil {
		ctx = withSecurityContext(ctx, securityCtx)
	}

	// Authorize the authenticated principal based on the permissions carried in the security context.
	if err := s.authorize(r.WithContext(ctx)); err != nil {
		return s.handleAuthError(ctx, r.URL.Path, err, isPublic, s.skipSecurity)
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
	if sysPerms != nil {
		return sysPerms.Root
	}
	return UninitializedPermissionSentinel
}

// isPublicPath checks if the given request path matches any of the configured public path patterns.
func (s *securityService) isPublicPath(requestPath string) bool {
	if len(requestPath) > maxPublicPathLength {
		s.logger.Warn("Path length exceeds maximum allowed length",
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

// handleAuthError handles authentication/authorization errors based on whether
// the path is public or security is skipped.
func (s *securityService) handleAuthError(
	ctx context.Context,
	path string,
	err error,
	isPublic bool,
	skipSecurity bool,
) (context.Context, error) {
	if isPublic {
		// Mark the context as a runtime caller so that the authorization layer can grant access.
		return WithRuntimeContext(ctx), nil
	}

	if skipSecurity {
		s.logger.Debug(
			"Proceeding without authentication/authorization enforcement as skipSecurity is enabled",
			log.Error(err),
			log.String("path", path))
		return withSecuritySkipped(ctx), nil
	}

	return nil, err
}
