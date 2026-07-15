/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package revocationcache

import (
	"context"
	"sync"
	"time"

	"github.com/thunder-id/thunderid/internal/system/log"
)

const loggerComponentName = "RevocationStatusCache"

// statusValid is the Token Status List status for a token that has not been revoked (draft-ietf-oauth-
// status-list §7.1). A token whose index is absent from a list's entries has this status.
const statusValid = 0

// cachedList is one Token Status List held in the cache: its recorded (non-VALID) entries keyed by
// index, the list's capacity (for out-of-bounds rejection), and the instant after which the snapshot
// is stale and must be re-fetched.
type cachedList struct {
	statuses  map[int64]int
	capacity  int64
	expiresAt time.Time
}

// statusCache lazily caches Token Status Lists by URI and answers the status of an individual token.
// Lists are loaded on first reference and re-fetched once older than the refresh interval; a failed
// re-fetch keeps serving the last-known-good snapshot for a bounded grace window (maxStale past its
// refresh deadline) so a brief source outage does not drop revocations already known. Once that window
// elapses the status is treated as unknown and the caller fails closed, because a token revoked after
// the snapshot must not be admitted indefinitely during a prolonged outage (draft-ietf-oauth-status-
// list §8.3). There is no background loop: fetches happen on the lookup that needs a fresh list.
type statusCache struct {
	mu       sync.RWMutex
	lists    map[string]cachedList
	source   StatusListSource
	ttl      time.Duration
	maxStale time.Duration
	now      func() time.Time
	logger   *log.Logger
}

// newStatusCache creates an empty cache backed by the given source with the given refresh interval. A
// stale snapshot may be served for at most one further refresh interval past its deadline before the
// status is treated as unknown, bounding how long a source outage can mask a fresh revocation.
func newStatusCache(source StatusListSource, ttl time.Duration) *statusCache {
	return &statusCache{
		lists:    make(map[string]cachedList),
		source:   source,
		ttl:      ttl,
		maxStale: ttl,
		now:      time.Now,
		logger:   log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName)),
	}
}

// statusAt returns the status of the token referenced by (uri, idx). available is false when the
// status is not resolvable — the list could not be loaded and nothing is cached, the list does not
// exist, or idx is out of the list's bounds — so the caller fails closed (draft-ietf-oauth-status-list
// §8.3). An in-bounds index absent from a resolved list is VALID.
func (c *statusCache) statusAt(ctx context.Context, uri string, idx int64) (status int, available bool) {
	c.mu.RLock()
	entry, cached := c.lists[uri]
	c.mu.RUnlock()

	if cached && c.now().Before(entry.expiresAt) {
		return lookup(entry, idx)
	}

	statuses, capacity, found, err := c.source.Fetch(ctx, uri)
	if err != nil {
		if cached && c.now().Before(entry.expiresAt.Add(c.maxStale)) {
			// Within the bounded grace window: serve the last-known-good snapshot so a brief source
			// outage does not let a token we already know to be revoked slip through.
			c.logger.Warn(ctx, "Failed to refresh status list; serving last-known-good snapshot",
				log.String("uri", uri), log.Error(err))
			return lookup(entry, idx)
		}
		// No snapshot, or the snapshot is now too stale to trust: the latest status is unknown, so fail
		// closed rather than admit a token that may have been revoked since the snapshot was taken.
		c.logger.Warn(ctx, "Failed to load status list; token status is unavailable",
			log.String("uri", uri), log.Error(err))
		return 0, false
	}
	if !found {
		// The referenced list does not exist: the status is unresolvable, so fail closed rather than
		// caching an empty (all-VALID) list. Not cached — a genuinely missing list stays missing.
		c.logger.Warn(ctx, "Status list not found; token status is unresolvable", log.String("uri", uri))
		return 0, false
	}

	next := cachedList{statuses: statuses, capacity: capacity, expiresAt: c.now().Add(c.ttl)}
	c.mu.Lock()
	c.lists[uri] = next
	c.mu.Unlock()
	return lookup(next, idx)
}

// lookup reads idx from a resolved list, failing closed (available=false) when idx is out of the
// list's bounds; an in-bounds index absent from the entries is VALID.
func lookup(entry cachedList, idx int64) (status int, available bool) {
	if idx < 0 || idx >= entry.capacity {
		return 0, false
	}
	return entry.statuses[idx], true
}
