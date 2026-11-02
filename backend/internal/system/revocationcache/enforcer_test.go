// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package revocationcache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/revocation"
	"github.com/thunder-id/thunderid/internal/system/security"
)

func TestEnforcer_EnsureNotRevoked(t *testing.T) {
	cache := newRevokedCache()
	cache.replace(revokedSnapshot{
		Tokens:   []revokedEntry{{Value: "revoked-jti", ExpiryTime: time.Now().Add(time.Hour)}},
		Families: []revokedEntry{{Value: "revoked-tfid", ExpiryTime: time.Now().Add(time.Hour)}},
		Subjects: []revokedEntry{{Value: "revoked-user", ExpiryTime: time.Now().Add(time.Hour)}},
		AppKeys:  []revokedEntry{{Value: "deleted-client", ExpiryTime: time.Now().Add(time.Hour)}},
	})
	e := newEnforcer(cache)

	assert.NoError(t, e.EnsureNotRevoked(context.Background(), security.RevocationIdentity{}),
		"empty ids are a no-op")
	assert.NoError(t, e.EnsureNotRevoked(context.Background(), security.RevocationIdentity{
		JTI: "active-jti", TokenFamilyID: "active-tfid", Subject: "active-user", AppKey: "active-client",
	}),
		"a token with a clean jti and family may proceed")
	assert.ErrorIs(t, e.EnsureNotRevoked(context.Background(),
		security.RevocationIdentity{JTI: "revoked-jti"}), errTokenRevoked,
		"a jti on the deny list is rejected")
	assert.ErrorIs(t, e.EnsureNotRevoked(context.Background(), security.RevocationIdentity{
		JTI: "active-jti", TokenFamilyID: "revoked-tfid",
	}), errTokenRevoked,
		"a token whose family is revoked is rejected even with a clean jti")
	assert.ErrorIs(t, e.EnsureNotRevoked(context.Background(), security.RevocationIdentity{
		Subject: "revoked-user",
	}), errTokenRevoked, "a token whose subject is revoked is rejected")
	assert.ErrorIs(t, e.EnsureNotRevoked(context.Background(), security.RevocationIdentity{
		JTI: "active-jti", AppKey: "deleted-client",
	}), errTokenRevoked,
		"a token issued to a revoked application is rejected even with a clean jti")
}

// TestEnforcer_AppKeyBoundary covers the secret-regeneration case: the entry is bounded, so a token
// established before the rotation is rejected while one minted after it passes.
func TestEnforcer_AppKeyBoundary(t *testing.T) {
	rotatedAt := time.Now().Add(-time.Minute)
	cache := newRevokedCache()
	cache.replace(revokedSnapshot{
		AppKeys: []revokedEntry{{
			Value:      "rotated-client",
			ExpiryTime: time.Now().Add(time.Hour),
			RevokedAt:  rotatedAt,
			Boundary:   true,
		}},
	})
	e := newEnforcer(cache)

	assert.ErrorIs(t, e.EnsureNotRevoked(context.Background(), security.RevocationIdentity{
		AppKey: "rotated-client", EstablishedAt: rotatedAt.Add(-time.Second),
	}), errTokenRevoked, "a token issued before the secret rotation is rejected")
	assert.ErrorIs(t, e.EnsureNotRevoked(context.Background(), security.RevocationIdentity{
		AppKey: "rotated-client", EstablishedAt: rotatedAt,
	}), errTokenRevoked, "a token established in the same instant as the cutoff is rejected")
	assert.NoError(t, e.EnsureNotRevoked(context.Background(), security.RevocationIdentity{
		AppKey: "rotated-client", EstablishedAt: rotatedAt.Add(time.Second),
	}), "a token minted with the new secret passes")
	assert.ErrorIs(t, e.EnsureNotRevoked(context.Background(), security.RevocationIdentity{
		AppKey: "rotated-client",
	}), errTokenRevoked, "an unknown establishment time fails closed")
}

// TestEnforcer_AppKeyTerminalIgnoresEstablishment covers application deletion: the entry is terminal, so
// even a token minted after the revocation is rejected.
func TestEnforcer_AppKeyTerminalIgnoresEstablishment(t *testing.T) {
	deletedAt := time.Now().Add(-time.Minute)
	cache := newRevokedCache()
	cache.replace(revokedSnapshot{
		AppKeys: []revokedEntry{{
			Value:      "deleted-client",
			ExpiryTime: time.Now().Add(time.Hour),
			RevokedAt:  deletedAt,
			Boundary:   false,
		}},
	})
	e := newEnforcer(cache)

	assert.ErrorIs(t, e.EnsureNotRevoked(context.Background(), security.RevocationIdentity{
		AppKey: "deleted-client", EstablishedAt: deletedAt.Add(time.Hour),
	}), errTokenRevoked, "a terminal application revocation ignores establishment time")
}

// TestEnforcer_AppKeyExpiredEntry confirms an entry past its expiry stops matching, so the deny list
// does not outlive the artifacts it governs.
func TestEnforcer_AppKeyExpiredEntry(t *testing.T) {
	cache := newRevokedCache()
	cache.replace(revokedSnapshot{
		AppKeys: []revokedEntry{{Value: "old-client", ExpiryTime: time.Now().Add(-time.Second)}},
	})
	e := newEnforcer(cache)

	assert.NoError(t, e.EnsureNotRevoked(context.Background(), security.RevocationIdentity{
		AppKey: "old-client",
	}), "an expired app-key entry no longer rejects")
}

func TestNoopEnforcer_AlwaysAllows(t *testing.T) {
	var e EnforcerInterface = noopEnforcer{}
	assert.NoError(t, e.EnsureNotRevoked(context.Background(), security.RevocationIdentity{JTI: "anything"}))
	assert.NoError(t, e.EnsureNotRevoked(context.Background(), security.RevocationIdentity{}))
}

type EnforcerScopeTestSuite struct {
	suite.Suite
}

func TestEnforcerScopeTestSuite(t *testing.T) {
	suite.Run(t, new(EnforcerScopeTestSuite))
}

func (s *EnforcerScopeTestSuite) TestScopeRevocationIsScopedToItsAudience() {
	const (
		user       = "user-1"
		california = "https://api.dmv.ca.gov"
		ohio       = "https://api.dmv.oh.gov"
	)
	revokedAt := time.Now().Add(-time.Hour)
	cache := newRevokedCache()
	cache.replace(revokedSnapshot{
		EntityScopes: []revokedEntry{{
			Value:      revocation.EntityScopeCriterionValue(user, ohio, "license"),
			ExpiryTime: time.Now().Add(time.Hour),
			RevokedAt:  revokedAt,
			Boundary:   true,
		}},
	})
	e := newEnforcer(cache)

	ohioToken := security.RevocationIdentity{
		Subject: user, Audiences: []string{ohio}, Scopes: []string{"openid", "license"},
		EstablishedAt: revokedAt.Add(-time.Minute),
	}
	s.ErrorIs(e.EnsureNotRevoked(context.Background(), ohioToken), errTokenRevoked,
		"the token bound to the audience the scope was revoked on is rejected")

	californiaToken := ohioToken
	californiaToken.Audiences = []string{california}
	s.NoError(e.EnsureNotRevoked(context.Background(), californiaToken),
		"the same scope on a different resource server is a different scope and must survive")

	bothAudiences := ohioToken
	bothAudiences.Audiences = []string{california, ohio}
	s.ErrorIs(e.EnsureNotRevoked(context.Background(), bothAudiences), errTokenRevoked,
		"a scope revoked on any audience the token names is rejected")

	otherUser := ohioToken
	otherUser.Subject = "user-2"
	s.NoError(e.EnsureNotRevoked(context.Background(), otherUser),
		"another principal holding the same scope is unaffected")

	later := ohioToken
	later.EstablishedAt = revokedAt.Add(time.Minute)
	s.NoError(e.EnsureNotRevoked(context.Background(), later),
		"a token established after a boundary revocation may proceed")
}

func (s *EnforcerScopeTestSuite) TestScopeDimensionAppliesToEveryPrincipal() {
	const california = "https://api.dmv.ca.gov"
	cache := newRevokedCache()
	cache.replace(revokedSnapshot{
		Scopes: []revokedEntry{{
			Value:      revocation.ScopeCriterionValue(california, "vehicle-registration"),
			ExpiryTime: time.Now().Add(time.Hour),
		}},
	})
	e := newEnforcer(cache)

	s.ErrorIs(e.EnsureNotRevoked(context.Background(), security.RevocationIdentity{
		Subject: "anyone", Audiences: []string{california}, Scopes: []string{"vehicle-registration"},
	}), errTokenRevoked)
	s.NoError(e.EnsureNotRevoked(context.Background(), security.RevocationIdentity{
		Subject: "anyone", Audiences: []string{california}, Scopes: []string{"license"},
	}), "an unrelated scope on the same server is unaffected")
}

func (s *EnforcerScopeTestSuite) TestTokenWithoutAudienceCarriesNoScopeDimensions() {
	cache := newRevokedCache()
	cache.replace(revokedSnapshot{
		Scopes: []revokedEntry{{
			Value:      revocation.ScopeCriterionValue("", "openid"),
			ExpiryTime: time.Now().Add(time.Hour),
		}},
	})
	e := newEnforcer(cache)

	s.NoError(e.EnsureNotRevoked(context.Background(), security.RevocationIdentity{
		Subject: "user-1", Scopes: []string{"openid"},
	}))
}
