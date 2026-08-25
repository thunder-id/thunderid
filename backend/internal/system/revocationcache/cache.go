// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package revocationcache

import (
	"sync"
	"time"
)

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
}

// newRevokedCache creates an empty cache. It holds nothing until the first snapshot is loaded.
func newRevokedCache() *revokedCache {
	return &revokedCache{
		tokens:   make(map[string]time.Time),
		families: make(map[string]time.Time),
		subjects: make(map[string]revokedEntry),
		appKeys:  make(map[string]revokedEntry),
	}
}

// replace atomically swaps the snapshot for the given entries. It is called by the syncer after each
// successful source read; a failed read leaves the previous snapshot in place (last-known-good).
func (c *revokedCache) replace(snapshot revokedSnapshot) {
	tokens := indexByValue(snapshot.Tokens)
	families := indexByValue(snapshot.Families)
	subjects := indexEntriesByValue(snapshot.Subjects)
	appKeys := indexEntriesByValue(snapshot.AppKeys)
	c.mu.Lock()
	c.tokens = tokens
	c.families = families
	c.subjects = subjects
	c.appKeys = appKeys
	c.mu.Unlock()
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
