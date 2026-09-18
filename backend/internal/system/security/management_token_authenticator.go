// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package security

import (
	"crypto/subtle"
	"net/http"
	"regexp"

	"github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// managementTokenSubject names the principal a management token authenticates as. It is not a user
// and not an OAuth client, and the logs should not suggest it is either.
const managementTokenSubject = "management-token"

// managementTokenPaths are the APIs a configured management token may call.
//
// Deliberately a short list rather than the whole management surface. The token is one long-lived
// string in a configuration file: it cannot be scoped down per caller, rotated per client, or
// revoked without a restart. What it opens should therefore be only what a deployment pipeline
// needs — push configuration in, and manage the values that configuration refers to.
//
// Everything else stays behind OAuth, where a caller is a client with its own credentials and its
// own permissions. Widening this list gives every holder of the token the new surface too.
var managementTokenPaths = []string{
	"/import",
	"/import/**",
	"/variables",
	"/variables/**",
	"/secrets",
	"/secrets/**",
}

// managementTokenAuthenticator authenticates a caller presenting the deployment's configured
// management token, for the small set of APIs that token is trusted for.
//
// It exists so a deployment pipeline can push configuration without first obtaining an OAuth client.
// A pipeline is not a user and has no interactive step in which to complete a grant, so requiring
// one has meant creating a machine application whose credentials then have to be managed anyway.
type managementTokenAuthenticator struct {
	// token is the configured value. Empty means the mechanism is off, which is the default.
	token []byte
	paths []*regexp.Regexp
}

// newManagementTokenAuthenticator builds the authenticator for a configured token. An empty token
// yields an authenticator that handles nothing, so the feature is off unless deliberately turned on.
func newManagementTokenAuthenticator(token string, paths []string) (*managementTokenAuthenticator, error) {
	compiled, err := compilePathPatterns(paths)
	if err != nil {
		return nil, err
	}
	return &managementTokenAuthenticator{token: []byte(token), paths: compiled}, nil
}

// CanHandle reports whether this request presents the management token on a path that trusts it.
//
// The token is compared here rather than in Authenticate, and that is the point: a bearer value that
// is not this token is left for the JWT authenticator to handle as it always would. Claiming the
// request and then failing would turn every OAuth call to these paths into a 401.
func (a *managementTokenAuthenticator) CanHandle(r *http.Request) bool {
	if len(a.token) == 0 {
		return false
	}
	if !a.trustsPath(r.URL.Path) {
		return false
	}

	presented, err := extractToken(r.Header.Get(constants.AuthorizationHeaderName))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(presented), a.token) == 1
}

// Authenticate builds the security context for a verified management token.
//
// The token is not carried into the context. Nothing downstream needs it, and a credential that is
// never put somewhere is a credential that cannot be logged from there by accident.
func (a *managementTokenAuthenticator) Authenticate(r *http.Request) (*SecurityContext, error) {
	// CanHandle did the comparison, and the service calls it first. Repeating it here means this
	// cannot become an authentication bypass if it is ever called directly.
	if !a.CanHandle(r) {
		return nil, errInvalidToken
	}
	return newSecurityContext(
		managementTokenSubject, "", "", []string{GetSystemRootPermission()}, nil), nil
}

// trustsPath reports whether the path is one this mechanism is trusted for.
func (a *managementTokenAuthenticator) trustsPath(path string) bool {
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

// hasManagementToken reports whether a management token is configured at all.
func hasManagementToken(token string) bool {
	return utils.SanitizeString(token) != ""
}
