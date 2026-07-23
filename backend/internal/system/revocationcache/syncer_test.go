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
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// fakeSource is a test syncSource whose snapshot and error are settable between reads.
type fakeSource struct {
	mu      sync.Mutex
	entries []revokedEntry
	err     error
	calls   int
}

func (f *fakeSource) Snapshot(context.Context) (revokedSnapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.err != nil {
		return revokedSnapshot{}, f.err
	}
	return revokedSnapshot{Tokens: f.entries}, nil
}

func (f *fakeSource) set(entries []revokedEntry, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.entries = entries
	f.err = err
}

func (f *fakeSource) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func futureEntry(jti string) revokedEntry {
	return revokedEntry{Value: jti, ExpiryTime: time.Now().Add(time.Hour)}
}

func TestSyncer_RefreshSuccessUpdatesCache(t *testing.T) {
	source := &fakeSource{entries: []revokedEntry{futureEntry("jti-1")}}
	cache := newRevokedCache()
	s := newSyncer(source, cache, time.Minute)

	assert.NoError(t, s.refresh(context.Background()))
	assert.True(t, cache.isTokenRevoked("jti-1"))
}

func TestSyncer_RefreshErrorKeepsLastKnownGood(t *testing.T) {
	source := &fakeSource{entries: []revokedEntry{futureEntry("jti-1")}}
	cache := newRevokedCache()
	s := newSyncer(source, cache, time.Minute)

	assert.NoError(t, s.refresh(context.Background()))
	assert.True(t, cache.isTokenRevoked("jti-1"))

	source.set(nil, errors.New("source unavailable"))
	assert.Error(t, s.refresh(context.Background()))
	assert.True(t, cache.isTokenRevoked("jti-1"), "a failed refresh must not empty the deny list")
}

func TestSyncer_StartRefreshesPeriodicallyThenStops(t *testing.T) {
	source := &fakeSource{entries: []revokedEntry{futureEntry("jti-1")}}
	cache := newRevokedCache()
	s := newSyncer(source, cache, 5*time.Millisecond)

	s.Start(context.Background())
	assert.Eventually(t, func() bool { return cache.isTokenRevoked("jti-1") }, time.Second, 5*time.Millisecond,
		"periodic refresh should load the snapshot into the cache")

	source.set([]revokedEntry{futureEntry("jti-2")}, nil)
	assert.Eventually(t, func() bool { return cache.isTokenRevoked("jti-2") }, time.Second, 5*time.Millisecond,
		"periodic refresh should pick up source changes")

	s.Stop()
	callsAfterStop := source.callCount()
	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, callsAfterStop, source.callCount(), "no refresh should occur after Stop")
}

func TestSyncer_StopIsIdempotent(t *testing.T) {
	source := &fakeSource{}
	s := newSyncer(source, newRevokedCache(), 5*time.Millisecond)
	s.Start(context.Background())
	s.Stop()
	assert.NotPanics(t, s.Stop, "Stop is safe to call more than once")
}

func TestSyncer_StartStopsOnContextCancel(t *testing.T) {
	source := &fakeSource{}
	s := newSyncer(source, newRevokedCache(), 5*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())

	s.Start(ctx)
	cancel()

	done := make(chan struct{})
	go func() {
		s.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Stop did not return after context cancellation")
	}
}

// blockingSource.Snapshot blocks until its context is canceled, simulating a hung source read.
type blockingSource struct {
	entered   chan struct{}
	enterOnce sync.Once
}

func (b *blockingSource) Snapshot(ctx context.Context) (revokedSnapshot, error) {
	b.enterOnce.Do(func() { close(b.entered) })
	<-ctx.Done()
	return revokedSnapshot{}, ctx.Err()
}

func TestSyncer_StopAbortsInFlightRefresh(t *testing.T) {
	source := &blockingSource{entered: make(chan struct{})}
	s := newSyncer(source, newRevokedCache(), 5*time.Millisecond)

	s.Start(context.Background())
	<-source.entered // a refresh is now blocked inside Snapshot

	done := make(chan struct{})
	go func() {
		s.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Stop did not return; the in-flight refresh was not canceled")
	}
}

func TestNoopSyncer_DoesNothing(t *testing.T) {
	var s Syncer = noopSyncer{}
	assert.NotPanics(t, func() {
		s.Start(context.Background())
		s.Stop()
	})
}
