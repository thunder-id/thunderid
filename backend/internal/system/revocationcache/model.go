// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package revocationcache

import "time"

// revokedEntry is one non-expired deny-list record returned by a syncSource and held in the cache.
type revokedEntry struct {
	// Value is the cache lookup key: the jti for a single-token entry, the tfid for a family entry.
	Value string
	// ExpiryTime is the revoked token's (or family's) original expiry; the entry is prunable once it
	// passes.
	ExpiryTime time.Time
	// RevokedAt is the latest establishment time affected by this criterion.
	RevokedAt time.Time
	// Boundary indicates that only artifacts established at or before RevokedAt are revoked.
	Boundary bool
}

// revokedSnapshot is one source read. Dimensions are separate so values of different types cannot collide.
type revokedSnapshot struct {
	// Tokens holds the revoked single-token entries (keyed by jti).
	Tokens []revokedEntry
	// Families holds the revoked token-family entries (keyed by tfid).
	Families []revokedEntry
	// Subjects holds revoked user-subject entries.
	Subjects []revokedEntry
	// AppKeys holds revoked OAuth client entries, written when an application is deleted or its
	// client secret regenerated.
	AppKeys []revokedEntry
}
