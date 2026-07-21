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
	"sync"
	"time"
)

// revokedCache is the concurrent in-memory deny-list snapshot. It maps a token's revocation
// identifier to the revoked token's original expiry, so a lookup can ignore entries whose token has
// already expired (and is rejected by time-claim validation anyway) even between syncs.
type revokedCache struct {
	mu      sync.RWMutex
	entries map[string]time.Time
}

// newRevokedCache creates an empty cache. It holds nothing until the first snapshot is loaded.
func newRevokedCache() *revokedCache {
	return &revokedCache{entries: make(map[string]time.Time)}
}

// replace atomically swaps the snapshot for the given entries. It is called by the syncer after each
// successful source read; a failed read leaves the previous snapshot in place (last-known-good).
func (c *revokedCache) replace(entries []revokedEntry) {
	next := make(map[string]time.Time, len(entries))
	for _, e := range entries {
		next[e.JTI] = e.ExpiryTime
	}
	c.mu.Lock()
	c.entries = next
	c.mu.Unlock()
}

// isRevoked reports whether id is in the deny list and its token has not yet expired.
func (c *revokedCache) isRevoked(id string) bool {
	c.mu.RLock()
	expiry, ok := c.entries[id]
	c.mu.RUnlock()
	return ok && time.Now().Before(expiry)
}
