// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package revocationcache

import (
	"context"

	"github.com/thunder-id/thunderid/internal/revocation"
	"github.com/thunder-id/thunderid/internal/system/security"
)

// EnforcerInterface answers revocation checks for the Resource Server enforcement point. A token is
// rejected when its JTI, token family, ThunderID subject, owning OAuth client, or any scope it carries
// is cached as revoked.
type EnforcerInterface interface {
	// EnsureNotRevoked returns nil when the token may proceed.
	EnsureNotRevoked(ctx context.Context, identity security.RevocationIdentity) error
}

// enforcer serves revocation checks from the in-memory cache. It holds no write capability.
type enforcer struct {
	cache *revokedCache
}

// newEnforcer creates an enforcer backed by the given cache.
func newEnforcer(cache *revokedCache) *enforcer {
	return &enforcer{cache: cache}
}

// EnsureNotRevoked returns errTokenRevoked when the identity matches a cached deny-list entry.
func (e *enforcer) EnsureNotRevoked(_ context.Context, identity security.RevocationIdentity) error {
	if identity.JTI != "" && e.cache.isTokenRevoked(identity.JTI) {
		return errTokenRevoked
	}
	if identity.TokenFamilyID != "" && e.cache.isTokenFamilyRevoked(identity.TokenFamilyID) {
		return errTokenRevoked
	}
	if identity.Subject != "" && e.cache.isSubjectRevoked(identity.Subject, identity.EstablishedAt) {
		return errTokenRevoked
	}
	if identity.AppKey != "" && e.cache.isAppKeyRevoked(identity.AppKey, identity.EstablishedAt) {
		return errTokenRevoked
	}
	if e.isScopeRevoked(identity) {
		return errTokenRevoked
	}
	return nil
}

// isScopeRevoked reports whether the token has lost any scope it carries.
//
// A permission string is unique only within its resource server, so both dimensions are keyed by a
// digest that includes the audience. A token with no audience carries no permission scopes either,
// and is not checked. The digests are derived through the same helpers the write path uses, so the
// two sides cannot disagree about what a row denies.
func (e *enforcer) isScopeRevoked(identity security.RevocationIdentity) bool {
	if identity.Audience == "" {
		return false
	}
	for _, scope := range identity.Scopes {
		if identity.Subject != "" && e.cache.isEntityScopeRevoked(
			revocation.EntityScopeCriterionValue(identity.Subject, identity.Audience, scope),
			identity.EstablishedAt) {
			return true
		}
		if e.cache.isScopeRevoked(
			revocation.ScopeCriterionValue(identity.Audience, scope), identity.EstablishedAt) {
			return true
		}
	}
	return false
}

// noopEnforcer is returned when RS revocation enforcement is disabled; it never rejects a token.
type noopEnforcer struct{}

// EnsureNotRevoked always returns nil.
func (noopEnforcer) EnsureNotRevoked(_ context.Context, _ security.RevocationIdentity) error {
	return nil
}
