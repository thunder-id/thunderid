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

package secretstore

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// countingBackend records how often it is read, so a test can tell a cache hit from a reload.
type countingBackend struct {
	ttl time.Duration

	mu      sync.Mutex
	secrets map[string]Secret
	loads   int
	loadErr error
}

func newCountingBackend(ttl time.Duration) *countingBackend {
	return &countingBackend{ttl: ttl, secrets: map[string]Secret{}}
}

func (b *countingBackend) Name() string            { return "counting" }
func (b *countingBackend) CacheTTL() time.Duration { return b.ttl }

func (b *countingBackend) Load(context.Context) (map[string]Secret, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.loads++
	if b.loadErr != nil {
		return nil, b.loadErr
	}
	out := make(map[string]Secret, len(b.secrets))
	for name, secret := range b.secrets {
		out[name] = secret
	}
	return out, nil
}

func (b *countingBackend) Put(_ context.Context, secret Secret) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.secrets[secret.Name] = secret
	return nil
}

func (b *countingBackend) Delete(_ context.Context, name string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.secrets, name)
	return nil
}

// loadCount reports how many times the backend has been read.
func (b *countingBackend) loadCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.loads
}

// setBehindTheStore changes the backend without going through the store, standing in for another
// instance of the same deployment writing to a shared vault.
func (b *countingBackend) setBehindTheStore(secret Secret) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.secrets[secret.Name] = secret
}

func TestStorePutIsReadableBack(t *testing.T) {
	ctx := context.Background()
	store, err := NewStore(ctx, newCountingBackend(0))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	if err := store.Put(ctx, Secret{Name: "A", Kind: KindValue, Value: "v"}); err != nil {
		t.Fatalf("put: %v", err)
	}

	got, ok := store.Get(ctx, "A")
	if !ok || got.Value != "v" {
		t.Fatalf("expected the secret back, got %+v %v", got, ok)
	}
}

// An invalid secret must not reach the backend: a hash with no algorithm could never be verified
// against, so storing it would leave a credential that silently rejects every attempt.
func TestStoreRefusesAnInvalidSecretWithoutWritingIt(t *testing.T) {
	ctx := context.Background()
	backend := newCountingBackend(0)
	store, err := NewStore(ctx, backend)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	err = store.Put(ctx, Secret{Name: "A", Kind: KindHash, Value: "h"})

	if !errors.Is(err, ErrInvalidSecret) {
		t.Fatalf("expected an invalid secret error, got %v", err)
	}
	if len(backend.secrets) != 0 {
		t.Fatal("an invalid secret must not be written to the backend")
	}
}

func TestStoreDeleteRemovesFromTheBackendAndTheCache(t *testing.T) {
	ctx := context.Background()
	backend := newCountingBackend(0)
	store, err := NewStore(ctx, backend)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	if err := store.Put(ctx, Secret{Name: "A", Kind: KindValue, Value: "v"}); err != nil {
		t.Fatalf("put: %v", err)
	}

	if err := store.Delete(ctx, "A"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if _, ok := store.Get(ctx, "A"); ok {
		t.Fatal("expected the secret to be gone from the store")
	}
	if _, held := backend.secrets["A"]; held {
		t.Fatal("expected the secret to be gone from the backend")
	}
}

func TestStoreNamesAreSortedAndValuesAreNotExposed(t *testing.T) {
	ctx := context.Background()
	store, err := NewStore(ctx, newCountingBackend(0))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	for _, name := range []string{"ZED", "ALPHA", "MIKE"} {
		if err := store.Put(ctx, Secret{Name: name, Kind: KindValue, Value: "v"}); err != nil {
			t.Fatalf("put %s: %v", name, err)
		}
	}

	names := store.Names(ctx)

	want := []string{"ALPHA", "MIKE", "ZED"}
	if len(names) != len(want) {
		t.Fatalf("expected %d names, got %v", len(want), names)
	}
	for i, name := range want {
		if names[i] != name {
			t.Fatalf("expected %v, got %v", want, names)
		}
	}
}

// A backend nothing else writes to is read once. Re-reading it on every resolution would turn each
// request into a file read for no gain.
func TestStoreWithNoCacheTTLReadsTheBackendOnlyOnce(t *testing.T) {
	ctx := context.Background()
	backend := newCountingBackend(0)
	store, err := NewStore(ctx, backend)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	store.Get(ctx, "A")
	store.All(ctx)
	store.Names(ctx)

	if got := backend.loadCount(); got != 1 {
		t.Fatalf("expected the single startup load, got %d", got)
	}
}

// A shared backend is re-read once its TTL passes, which is how one instance picks up a secret
// another instance of the same deployment has written.
func TestStoreRereadsASharedBackendOnceTheTTLPasses(t *testing.T) {
	ctx := context.Background()
	backend := newCountingBackend(10 * time.Millisecond)
	store, err := NewStore(ctx, backend)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	if _, ok := store.Get(ctx, "WRITTEN_ELSEWHERE"); ok {
		t.Fatal("expected the store to start without it")
	}

	backend.setBehindTheStore(Secret{Name: "WRITTEN_ELSEWHERE", Kind: KindValue, Value: "v"})
	time.Sleep(15 * time.Millisecond)

	if _, ok := store.Get(ctx, "WRITTEN_ELSEWHERE"); !ok {
		t.Fatal("expected the store to pick up a secret written to the shared backend")
	}
}

// Within the TTL the cache answers, so resolving a reference during a request costs no outbound call.
func TestStoreServesFromTheCacheWithinTheTTL(t *testing.T) {
	ctx := context.Background()
	backend := newCountingBackend(time.Hour)
	store, err := NewStore(ctx, backend)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	for range 5 {
		store.Get(ctx, "A")
	}

	if got := backend.loadCount(); got != 1 {
		t.Fatalf("expected reads to be served from the cache, got %d loads", got)
	}
}

// A vault that goes unreachable should leave the store serving what it last read. Dropping the cache
// would turn a brief outage into every credential failing at once.
func TestStoreKeepsServingWhenAReloadFails(t *testing.T) {
	ctx := context.Background()
	backend := newCountingBackend(10 * time.Millisecond)
	backend.setBehindTheStore(Secret{Name: "A", Kind: KindValue, Value: "v"})
	store, err := NewStore(ctx, backend)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	backend.loadErr = errors.New("the vault is unreachable")
	time.Sleep(15 * time.Millisecond)

	got, ok := store.Get(ctx, "A")
	if !ok || got.Value != "v" {
		t.Fatalf("expected the last known secret to still be served, got %+v %v", got, ok)
	}
}

// A store whose backing cannot be read at startup is still returned, so the server comes up and
// recovers rather than losing its secrets for the life of the process.
func TestNewStoreReturnsAUsableStoreWhenTheFirstLoadFails(t *testing.T) {
	backend := newCountingBackend(0)
	backend.loadErr = errors.New("the vault is unreachable")

	store, err := NewStore(context.Background(), backend)

	if err == nil {
		t.Fatal("expected the load failure to be reported")
	}
	if store == nil {
		t.Fatal("expected a usable store despite the failed load")
	}
	if store.Count() != 0 {
		t.Fatalf("expected an empty store, got %d", store.Count())
	}
}

func TestNewStoreRefusesANilBackend(t *testing.T) {
	if _, err := NewStore(context.Background(), nil); err == nil {
		t.Fatal("expected a store with no backend to be refused")
	}
}
