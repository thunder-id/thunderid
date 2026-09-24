// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package security

import (
	"net/http"
	"regexp"

	"github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// managementAPIKeySubject names the principal a management API key authenticates as: not a user
// and not an OAuth client.
const managementAPIKeySubject = "management-api-key"

// managementAPIKeyPaths are the APIs a configured management API key may call.
//
// Deliberately short. The key is one long-lived credential that cannot be scoped per caller or
// revoked without a restart, so widening this list hands every holder of it the new surface too.
var managementAPIKeyPaths = []string{
	"/import",
	"/import/**",
	"/variables",
	"/variables/**",
	"/secrets",
	"/secrets/**",
}

// managementAPIKeyAuthenticator authenticates a caller presenting the deployment's management API
// key, for the small set of APIs that key is trusted for. It exists so a deployment pipeline can
// push configuration without first obtaining an OAuth client.
type managementAPIKeyAuthenticator struct {
	// keyHash is the configured SHA-256 hex digest of the key; empty means off. The key itself is
	// never held here, so this plane has no way to present one.
	keyHash string
	paths   []*regexp.Regexp
}

// newManagementAPIKeyAuthenticator builds the authenticator for a configured key digest.
func newManagementAPIKeyAuthenticator(keyHash string, paths []string) (*managementAPIKeyAuthenticator, error) {
	compiled, err := compilePathPatterns(paths)
	if err != nil {
		return nil, err
	}
	return &managementAPIKeyAuthenticator{keyHash: keyHash, paths: compiled}, nil
}

// CanHandle reports whether this request presents a management API key on a path that trusts it.
//
// Presence only. Whether the key is the right one is Authenticate's to decide, so a request that
// presents one and gets it wrong is refused rather than falling through to another mechanism.
func (a *managementAPIKeyAuthenticator) CanHandle(r *http.Request) bool {
	if a.keyHash == "" {
		return false
	}
	if !a.trustsPath(r.URL.Path) {
		return false
	}
	return utils.SanitizeString(r.Header.Get(constants.APIKeyHeaderName)) != ""
}

// Authenticate verifies the presented key and builds the security context for it. The key is not
// carried into that context: nothing downstream needs it, and it cannot be logged from where it is
// not.
func (a *managementAPIKeyAuthenticator) Authenticate(r *http.Request) (*SecurityContext, error) {
	if a.keyHash == "" {
		return nil, errInvalidToken
	}
	presented := utils.SanitizeString(r.Header.Get(constants.APIKeyHeaderName))
	if presented == "" || !cryptolib.ValidateTokenHash(presented, a.keyHash) {
		return nil, errInvalidToken
	}
	return newSecurityContext(
		managementAPIKeySubject, "", "", []string{GetSystemRootPermission()}, nil), nil
}

// trustsPath reports whether the path is one this mechanism is trusted for.
func (a *managementAPIKeyAuthenticator) trustsPath(path string) bool {
	if len(path) > maxPublicPathLength {
		return false
	}
	for _, pattern := range a.paths {
		if pattern.MatchString(path) {
			return true
		}
	}
	return false
}

// hasManagementAPIKey reports whether a management API key digest is configured at all.
func hasManagementAPIKey(keyHash string) bool {
	return utils.SanitizeString(keyHash) != ""
}
