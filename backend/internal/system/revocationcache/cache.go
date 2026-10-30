// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package revocationcache

import (
	"sync"
	"time"

	"github.com/thunder-id/thunderid/internal/revocation"
)

// pruneInterval is how often apply drops lapsed entries, which a full replace would otherwise do.
const pruneInterval = time.Minute

// revokedCache is the concurrent in-memory deny-list snapshot. It maps each revoked jti and each
// revoked token-family id (tfid) to its original expiry, so a lookup can ignore entries whose token
// has already expired (and is rejected by time-claim validation anyway) even between syncs. The two
// dimensions are kept separate so a jti is never matched against a tfid.
type revokedCache struct {
	mu       sync.RWMutex
	tokens   map[string]time.Time
	families map[string]time.Time
	subjects map[string]revokedEntry
	appKeys  map[string]revokedEntry
	// entityScopes and scopes are keyed by criterion digest.
	entityScopes map[string]revokedEntry
	scopes       map[string]revokedEntry
	lastPruned   time.Time
}

// newRevokedCache creates an empty cache. It holds nothing until the first snapshot is loaded.
func newRevokedCache() *revokedCache {
	return &revokedCache{
		tokens:       make(map[string]time.Time),
		families:     make(map[string]time.Time),
		subjects:     make(map[string]revokedEntry),
		appKeys:      make(map[string]revokedEntry),
		entityScopes: make(map[string]revokedEntry),
		scopes:       make(map[string]revokedEntry),
	}
}

// replace atomically swaps the snapshot for the given entries. It is called by the syncer after each
// successful source read; a failed read leaves the previous snapshot in place (last-known-good).
func (c *revokedCache) replace(snapshot revokedSnapshot) {
	tokens := indexByValue(snapshot.Tokens)
	families := indexByValue(snapshot.Families)
	subjects := indexEntriesByValue(snapshot.Subjects)
	appKeys := indexEntriesByValue(snapshot.AppKeys)
	entityScopes := indexEntriesByValue(snapshot.EntityScopes)
	scopes := indexEntriesByValue(snapshot.Scopes)
	c.mu.Lock()
	c.tokens = tokens
	c.families = families
	c.subjects = subjects
	c.appKeys = appKeys
	c.entityScopes = entityScopes
	c.scopes = scopes
	c.mu.Unlock()
}

// apply adds one pushed revocation to the snapshot, merging it with what is already held the way the
// deny-list write does: a terminal revocation is never weakened to a bounded one, a cutoff never moves
// back, and an expiry is never shortened. Criterion types the cache does not enforce are ignored.
func (c *revokedCache) apply(event revocation.Event) {
	incoming := revokedEntry{
		Value:      event.Criterion.Value,
		ExpiryTime: event.ExpiryTime,
		RevokedAt:  event.RevokedAt,
		Boundary:   revocation.IsBoundaryReason(event.Reason),
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if event.JTI != "" {
		c.tokens[event.JTI] = laterOf(c.tokens[event.JTI], event.ExpiryTime)
	} else {
		switch event.Criterion.Type {
		case revocation.CriterionTypeTokenFamily:
			c.families[incoming.Value] = laterOf(c.families[incoming.Value], incoming.ExpiryTime)
		case revocation.CriterionTypeSubject:
			mergeEntry(c.subjects, incoming)
		case revocation.CriterionTypeApplicationKey:
			mergeEntry(c.appKeys, incoming)
		case revocation.CriterionTypeEntityScope:
			mergeEntry(c.entityScopes, incoming)
		case revocation.CriterionTypeScope:
			mergeEntry(c.scopes, incoming)
		}
	}
	c.pruneLocked()
}

// mergeEntry stores the incoming entry over any held one without weakening it.
func mergeEntry(entries map[string]revokedEntry, incoming revokedEntry) {
	if held, ok := entries[incoming.Value]; ok {
		incoming.Boundary = incoming.Boundary && held.Boundary
		incoming.ExpiryTime = laterOf(held.ExpiryTime, incoming.ExpiryTime)
		if held.RevokedAt.After(incoming.RevokedAt) {
			incoming.RevokedAt = held.RevokedAt
		}
	}
	entries[incoming.Value] = incoming
}

// laterOf returns the later of two times.
func laterOf(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

// pruneLocked drops entries whose expiry has passed, at most once per pruneInterval. The caller
// holds the write lock.
func (c *revokedCache) pruneLocked() {
	now := time.Now()
	if now.Sub(c.lastPruned) < pruneInterval {
		return
	}
	c.lastPruned = now
	pruneTimes(c.tokens, now)
	pruneTimes(c.families, now)
	pruneEntries(c.subjects, now)
	pruneEntries(c.appKeys, now)
	pruneEntries(c.entityScopes, now)
	pruneEntries(c.scopes, now)
}

func pruneTimes(m map[string]time.Time, now time.Time) {
	for k, expiry := range m {
		if !now.Before(expiry) {
			delete(m, k)
		}
	}
}

func pruneEntries(m map[string]revokedEntry, now time.Time) {
	for k, entry := range m {
		if !now.Before(entry.ExpiryTime) {
			delete(m, k)
		}
	}
}

func (c *revokedCache) isSubjectRevoked(subject string, establishedAt time.Time) bool {
	c.mu.RLock()
	entry, ok := c.subjects[subject]
	c.mu.RUnlock()
	return matchesEntry(entry, ok, establishedAt)
}

// isAppKeyRevoked reports whether the OAuth client the token was issued to is revoked, honoring the
// establishment cutoff so a token minted after a secret regeneration still passes.
func (c *revokedCache) isAppKeyRevoked(appKey string, establishedAt time.Time) bool {
	c.mu.RLock()
	entry, ok := c.appKeys[appKey]
	c.mu.RUnlock()
	return matchesEntry(entry, ok, establishedAt)
}

// isEntityScopeRevoked reports whether the scope is revoked for one entity, honoring the cutoff.
func (c *revokedCache) isEntityScopeRevoked(digest string, establishedAt time.Time) bool {
	c.mu.RLock()
	entry, ok := c.entityScopes[digest]
	c.mu.RUnlock()
	return matchesEntry(entry, ok, establishedAt)
}

// isScopeRevoked reports whether the scope is revoked deployment-wide, honoring the cutoff.
func (c *revokedCache) isScopeRevoked(digest string, establishedAt time.Time) bool {
	c.mu.RLock()
	entry, ok := c.scopes[digest]
	c.mu.RUnlock()
	return matchesEntry(entry, ok, establishedAt)
}

// matchesEntry applies the shared terminal-versus-bounded decision. A terminal entry rejects on
// membership alone; a bounded entry rejects only artifacts established at or before its cutoff. An
// unknown establishment time is treated as revoked, keeping the decision fail-closed.
func matchesEntry(entry revokedEntry, found bool, establishedAt time.Time) bool {
	return found && time.Now().Before(entry.ExpiryTime) &&
		(!entry.Boundary || establishedAt.IsZero() || !establishedAt.After(entry.RevokedAt))
}

// indexEntriesByValue builds a value-to-entry map while preserving criterion metadata.
func indexEntriesByValue(entries []revokedEntry) map[string]revokedEntry {
	indexed := make(map[string]revokedEntry, len(entries))
	for _, entry := range entries {
		indexed[entry.Value] = entry
	}
	return indexed
}

// isTokenRevoked reports whether jti is on the single-token deny list and has not yet expired.
func (c *revokedCache) isTokenRevoked(jti string) bool {
	c.mu.RLock()
	expiry, ok := c.tokens[jti]
	c.mu.RUnlock()
	return ok && time.Now().Before(expiry)
}

// isTokenFamilyRevoked reports whether tfid is on the family deny list and has not yet expired.
func (c *revokedCache) isTokenFamilyRevoked(tfid string) bool {
	c.mu.RLock()
	expiry, ok := c.families[tfid]
	c.mu.RUnlock()
	return ok && time.Now().Before(expiry)
}

// indexByValue builds a value -> expiry map from a slice of entries.
func indexByValue(entries []revokedEntry) map[string]time.Time {
	m := make(map[string]time.Time, len(entries))
	for _, e := range entries {
		m[e.Value] = e.ExpiryTime
	}
	return m
}
