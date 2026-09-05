// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package revocationcache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

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
