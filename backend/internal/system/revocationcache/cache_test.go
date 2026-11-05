// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package revocationcache

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/thunder-id/thunderid/internal/revocation"
)

func TestRevokedCache_ReplaceAndIsRevoked(t *testing.T) {
	c := newRevokedCache()
	future := time.Now().Add(time.Hour)

	assert.False(t, c.isTokenRevoked("jti-1"), "empty cache reports nothing revoked")

	c.replace(revokedSnapshot{
		Tokens: []revokedEntry{
			{Value: "jti-1", ExpiryTime: future},
			{Value: "jti-2", ExpiryTime: future},
		},
		Families: []revokedEntry{{Value: "tfid-1", ExpiryTime: future}},
		Subjects: []revokedEntry{{Value: "user-1", ExpiryTime: future}},
	})

	assert.True(t, c.isTokenRevoked("jti-1"))
	assert.True(t, c.isTokenRevoked("jti-2"))
	assert.False(t, c.isTokenRevoked("jti-3"))
	assert.True(t, c.isTokenFamilyRevoked("tfid-1"))
	assert.False(t, c.isTokenFamilyRevoked("jti-1"), "a jti must not match the family dimension")
	assert.False(t, c.isTokenRevoked("tfid-1"), "a tfid must not match the token dimension")
	assert.True(t, c.isSubjectRevoked("user-1", time.Time{}))
	assert.False(t, c.isSubjectRevoked("jti-1", time.Time{}), "a jti must not match the subject dimension")
}

func TestRevokedCache_ReplaceSwapsSnapshot(t *testing.T) {
	c := newRevokedCache()
	future := time.Now().Add(time.Hour)

	c.replace(revokedSnapshot{Tokens: []revokedEntry{{Value: "old", ExpiryTime: future}}})
	assert.True(t, c.isTokenRevoked("old"))

	c.replace(revokedSnapshot{Tokens: []revokedEntry{{Value: "new", ExpiryTime: future}}})
	assert.False(t, c.isTokenRevoked("old"), "prior entries are dropped on replace")
	assert.True(t, c.isTokenRevoked("new"))
}

func TestRevokedCache_ExpiredEntryNotRevoked(t *testing.T) {
	c := newRevokedCache()
	c.replace(revokedSnapshot{Tokens: []revokedEntry{{Value: "expired", ExpiryTime: time.Now().Add(-time.Second)}}})

	assert.False(t, c.isTokenRevoked("expired"), "an entry past its expiry is treated as not revoked")
}

func TestRevokedCache_SubjectBoundary(t *testing.T) {
	now := time.Now().UTC()
	c := newRevokedCache()
	c.replace(revokedSnapshot{Subjects: []revokedEntry{
		{Value: "bounded-user", ExpiryTime: now.Add(time.Hour), RevokedAt: now, Boundary: true},
		{Value: "deleted-user", ExpiryTime: now.Add(time.Hour), RevokedAt: now},
	}})

	assert.True(t, c.isSubjectRevoked("bounded-user", now.Add(-time.Minute)))
	assert.False(t, c.isSubjectRevoked("bounded-user", now.Add(time.Minute)))
	assert.True(t, c.isSubjectRevoked("deleted-user", now.Add(time.Minute)))
}

func TestRevokedCache_ConcurrentAccess(t *testing.T) {
	c := newRevokedCache()
	future := time.Now().Add(time.Hour)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			c.replace(revokedSnapshot{Tokens: []revokedEntry{{Value: "jti", ExpiryTime: future}}})
		}()
		go func() {
			defer wg.Done()
			_ = c.isTokenRevoked("jti")
		}()
	}
	wg.Wait()

	assert.True(t, c.isTokenRevoked("jti"))
}

func criterionEvent(criterionType revocation.CriterionType, value string, reason revocation.Reason,
	revokedAt time.Time, expiry time.Time) revocation.Event {
	return revocation.Event{
		Criterion: revocation.Criterion{Type: criterionType, Value: value},
		Reason:    reason, RevokedAt: revokedAt, ExpiryTime: expiry,
	}
}

func TestRevokedCache_ApplyEachDimension(t *testing.T) {
	cache := newRevokedCache()
	expiry := time.Now().Add(time.Hour)

	cache.apply(revocation.Event{JTI: "jti-1", Reason: revocation.ReasonExplicit, RevokedAt: time.Now(),
		ExpiryTime: expiry})
	cache.apply(criterionEvent(revocation.CriterionTypeTokenFamily, "tfid-1", revocation.ReasonSessionLogout,
		time.Now(), expiry))
	cache.apply(criterionEvent(revocation.CriterionTypeSubject, "user-1", revocation.ReasonUserDeleted,
		time.Now(), expiry))
	cache.apply(criterionEvent(revocation.CriterionTypeApplicationKey, "client-1",
		revocation.ReasonApplicationDeleted, time.Now(), expiry))
	cache.apply(criterionEvent(revocation.CriterionTypeEntityScope, "digest-1", revocation.ReasonRoleDeleted,
		time.Now(), expiry))
	cache.apply(criterionEvent(revocation.CriterionTypeScope, "digest-2", revocation.ReasonActionDeleted,
		time.Now(), expiry))
	cache.apply(criterionEvent(revocation.CriterionTypeOrganizationUnit, "ou-1",
		revocation.ReasonOrganizationUnitChanged, time.Now(), expiry))

	assert.True(t, cache.isTokenRevoked("jti-1"))
	assert.True(t, cache.isTokenFamilyRevoked("tfid-1"))
	assert.True(t, cache.isSubjectRevoked("user-1", time.Time{}))
	assert.True(t, cache.isAppKeyRevoked("client-1", time.Time{}))
	assert.True(t, cache.isEntityScopeRevoked("digest-1", time.Time{}))
	assert.True(t, cache.isScopeRevoked("digest-2", time.Time{}))
	assert.False(t, cache.isSubjectRevoked("someone-else", time.Time{}))
}

func TestRevokedCache_ApplyNeverWeakensAnEntry(t *testing.T) {
	now := time.Now()
	established := now.Add(-time.Minute)

	t.Run("a terminal entry stays terminal when a bounded one follows", func(t *testing.T) {
		cache := newRevokedCache()
		cache.apply(criterionEvent(revocation.CriterionTypeSubject, "user-1", revocation.ReasonUserDeleted,
			now, now.Add(time.Hour)))
		cache.apply(criterionEvent(revocation.CriterionTypeSubject, "user-1", revocation.ReasonIdentitySuspended,
			now.Add(-time.Hour), now.Add(2*time.Hour)))

		assert.True(t, cache.isSubjectRevoked("user-1", now.Add(time.Minute)),
			"a token minted after the bounded cutoff is still rejected by the terminal entry")
		assert.Equal(t, now.Add(2*time.Hour).Unix(), cache.subjects["user-1"].ExpiryTime.Unix(),
			"the expiry is the later one")
	})

	t.Run("a bounded cutoff never moves back", func(t *testing.T) {
		cache := newRevokedCache()
		cache.apply(criterionEvent(revocation.CriterionTypeSubject, "user-1", revocation.ReasonIdentitySuspended,
			now, now.Add(time.Hour)))
		cache.apply(criterionEvent(revocation.CriterionTypeSubject, "user-1", revocation.ReasonIdentitySuspended,
			now.Add(-time.Hour), now.Add(time.Minute)))

		assert.True(t, cache.isSubjectRevoked("user-1", established), "established before the later cutoff")
		assert.Equal(t, now.Add(time.Hour).Unix(), cache.subjects["user-1"].ExpiryTime.Unix(),
			"the expiry is never shortened")
	})

	t.Run("a terminal event hardens a bounded entry", func(t *testing.T) {
		cache := newRevokedCache()
		cache.apply(criterionEvent(revocation.CriterionTypeSubject, "user-1", revocation.ReasonIdentitySuspended,
			now.Add(-time.Hour), now.Add(time.Hour)))
		assert.False(t, cache.isSubjectRevoked("user-1", now), "minted after the cutoff")

		cache.apply(criterionEvent(revocation.CriterionTypeSubject, "user-1", revocation.ReasonUserDeleted,
			now, now.Add(time.Hour)))

		assert.True(t, cache.isSubjectRevoked("user-1", now))
	})
}

func TestRevokedCache_ApplyPrunesLapsedEntries(t *testing.T) {
	cache := newRevokedCache()
	cache.apply(revocation.Event{JTI: "old", Reason: revocation.ReasonExplicit, RevokedAt: time.Now(),
		ExpiryTime: time.Now().Add(-time.Minute)})
	cache.apply(criterionEvent(revocation.CriterionTypeSubject, "old-user", revocation.ReasonUserDeleted,
		time.Now(), time.Now().Add(-time.Minute)))
	cache.lastPruned = time.Now().Add(-2 * pruneInterval)

	cache.apply(revocation.Event{JTI: "fresh", Reason: revocation.ReasonExplicit, RevokedAt: time.Now(),
		ExpiryTime: time.Now().Add(time.Hour)})

	assert.NotContains(t, cache.tokens, "old")
	assert.NotContains(t, cache.subjects, "old-user")
	assert.Contains(t, cache.tokens, "fresh")
}
