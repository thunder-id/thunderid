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
	"errors"
	"testing"
	"time"
)

// fakeSource is a hand-written StatusListSource for exercising the cache without the Status List
// subsystem. It records the number of Fetch calls so tests can assert cache hits vs re-fetches.
type fakeSource struct {
	statuses map[int64]int
	capacity int64
	found    bool
	err      error
	calls    int
}

func (f *fakeSource) Fetch(context.Context, string) (map[int64]int, int64, bool, error) {
	f.calls++
	return f.statuses, f.capacity, f.found, f.err
}

func TestStatusCacheLoadsLazilyAndServesWithinTTL(t *testing.T) {
	src := &fakeSource{statuses: map[int64]int{7: 1}, capacity: 100, found: true}
	cache := newStatusCache(src, time.Hour)

	status, available := cache.statusAt(context.Background(), "uri", 7)
	if !available || status != 1 {
		t.Fatalf("first lookup = (%d, %v), want (1, true)", status, available)
	}
	// A second lookup within the TTL is served from the cache without re-fetching.
	if _, _ = cache.statusAt(context.Background(), "uri", 7); src.calls != 1 {
		t.Fatalf("source Fetch called %d times, want 1", src.calls)
	}
}

func TestStatusCacheMissingIndexIsValid(t *testing.T) {
	// idx 99 is within capacity 100 but has no entry: the bit is genuinely VALID.
	cache := newStatusCache(&fakeSource{statuses: map[int64]int{7: 1}, capacity: 100, found: true}, time.Hour)

	status, available := cache.statusAt(context.Background(), "uri", 99)
	if !available || status != statusValid {
		t.Fatalf("missing index = (%d, %v), want (%d, true)", status, available, statusValid)
	}
}

func TestStatusCacheOutOfBoundsIndexFailsClosed(t *testing.T) {
	// idx 100 is at/beyond capacity 100: out of the list's bounds, so the status is unresolvable.
	cache := newStatusCache(&fakeSource{statuses: map[int64]int{}, capacity: 100, found: true}, time.Hour)

	if status, available := cache.statusAt(context.Background(), "uri", 100); available || status != 0 {
		t.Fatalf("out-of-bounds index = (%d, %v), want (0, false)", status, available)
	}
}

func TestStatusCacheNotFoundListFailsClosed(t *testing.T) {
	// A referenced list that does not exist is unresolvable and must not be treated as all-VALID.
	src := &fakeSource{found: false}
	cache := newStatusCache(src, time.Hour)

	if status, available := cache.statusAt(context.Background(), "uri", 7); available || status != 0 {
		t.Fatalf("not-found list = (%d, %v), want (0, false)", status, available)
	}
	// A genuinely missing list is not cached, so a later lookup re-fetches.
	if _, _ = cache.statusAt(context.Background(), "uri", 7); src.calls != 2 {
		t.Fatalf("source Fetch called %d times, want 2 (not-found is not cached)", src.calls)
	}
}

func TestStatusCacheKeepsLastKnownGoodWithinGraceOnRefreshError(t *testing.T) {
	src := &fakeSource{statuses: map[int64]int{7: 1}, capacity: 100, found: true}
	cache := newStatusCache(src, time.Minute) // ttl and maxStale are both one minute
	base := time.Now()
	cache.now = func() time.Time { return base }

	if status, available := cache.statusAt(context.Background(), "uri", 7); !available || status != 1 {
		t.Fatalf("initial load = (%d, %v), want (1, true)", status, available)
	}

	// Past the refresh deadline but within the grace window, a failed re-fetch keeps serving the
	// last-known-good snapshot so a brief outage does not drop a revocation we already know.
	cache.now = func() time.Time { return base.Add(90 * time.Second) }
	src.err = errors.New("db down")
	status, available := cache.statusAt(context.Background(), "uri", 7)
	if !available || status != 1 {
		t.Fatalf("within grace = (%d, %v), want (1, true)", status, available)
	}
}

func TestStatusCacheStaleBeyondGraceFailsClosed(t *testing.T) {
	src := &fakeSource{statuses: map[int64]int{7: 1}, capacity: 100, found: true}
	cache := newStatusCache(src, time.Minute)
	base := time.Now()
	cache.now = func() time.Time { return base }

	if status, available := cache.statusAt(context.Background(), "uri", 7); !available || status != 1 {
		t.Fatalf("initial load = (%d, %v), want (1, true)", status, available)
	}

	// Beyond expiry plus the grace window the snapshot is too stale to trust: a token revoked since
	// then must not slip through, so the status is unavailable and the caller fails closed.
	cache.now = func() time.Time { return base.Add(3 * time.Minute) }
	src.err = errors.New("db down")
	status, available := cache.statusAt(context.Background(), "uri", 7)
	if available || status != 0 {
		t.Fatalf("beyond grace = (%d, %v), want (0, false)", status, available)
	}
}

func TestStatusCacheUnavailableWhenFirstLoadFails(t *testing.T) {
	cache := newStatusCache(&fakeSource{err: errors.New("db down")}, time.Hour)

	if status, available := cache.statusAt(context.Background(), "uri", 7); available || status != 0 {
		t.Fatalf("failed load = (%d, %v), want (0, false)", status, available)
	}
}
