// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package bff

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// memStore is an in-memory runtime store with TTLs on an injected clock.
type memStore struct {
	mu      sync.Mutex
	now     func() time.Time
	entries map[string]memEntry
}

type memEntry struct {
	value   []byte
	expires time.Time
}

func newMemStore(now func() time.Time) *memStore {
	return &memStore{now: now, entries: map[string]memEntry{}}
}

func memKey(namespace providers.RuntimeStoreNamespace, key string) string {
	return string(namespace) + "|" + key
}

// live returns the entry when it exists and has not expired. The caller holds the lock.
func (m *memStore) live(k string) (memEntry, bool) {
	e, ok := m.entries[k]
	if !ok || !m.now().Before(e.expires) {
		return memEntry{}, false
	}
	return e, true
}

func (m *memStore) Put(_ context.Context, ns providers.RuntimeStoreNamespace, key string, value []byte,
	ttl int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries[memKey(ns, key)] = memEntry{value: value, expires: m.now().Add(time.Duration(ttl) * time.Second)}
	return nil
}

func (m *memStore) PutIfNotExists(_ context.Context, ns providers.RuntimeStoreNamespace, key string,
	value []byte, ttl int64) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.live(memKey(ns, key)); ok {
		return false, nil
	}
	m.entries[memKey(ns, key)] = memEntry{value: value, expires: m.now().Add(time.Duration(ttl) * time.Second)}
	return true, nil
}

func (m *memStore) Get(_ context.Context, ns providers.RuntimeStoreNamespace, key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.live(memKey(ns, key))
	if !ok {
		return nil, nil
	}
	return e.value, nil
}

func (m *memStore) Update(_ context.Context, ns providers.RuntimeStoreNamespace, key string,
	value []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e, ok := m.live(memKey(ns, key)); ok {
		e.value = value
		m.entries[memKey(ns, key)] = e
	}
	return nil
}

func (m *memStore) Delete(_ context.Context, ns providers.RuntimeStoreNamespace, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.entries, memKey(ns, key))
	return nil
}

func (m *memStore) Take(ctx context.Context, ns providers.RuntimeStoreNamespace, key string) ([]byte, error) {
	value, _ := m.Get(ctx, ns, key)
	_ = m.Delete(ctx, ns, key)
	return value, nil
}

func (m *memStore) ExtendTTL(_ context.Context, ns providers.RuntimeStoreNamespace, key string,
	ttl int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e, ok := m.live(memKey(ns, key)); ok {
		e.expires = m.now().Add(time.Duration(ttl) * time.Second)
		m.entries[memKey(ns, key)] = e
	}
	return nil
}

func (m *memStore) CompareFieldAndSwap(_ context.Context, ns providers.RuntimeStoreNamespace, key, field,
	expected string, newValue []byte) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.live(memKey(ns, key))
	if !ok {
		return false, nil
	}
	var doc map[string]any
	if err := json.Unmarshal(e.value, &doc); err != nil {
		return false, err
	}
	if current, _ := doc[field].(string); current != expected {
		return false, nil
	}
	e.value = newValue
	m.entries[memKey(ns, key)] = e
	return true, nil
}

// ttl returns how long an entry has left, or zero when it is gone.
func (m *memStore) ttl(ns providers.RuntimeStoreNamespace, key string) time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.live(memKey(ns, key))
	if !ok {
		return 0
	}
	return e.expires.Sub(m.now())
}

// count returns how many live entries a namespace holds.
func (m *memStore) count(ns providers.RuntimeStoreNamespace) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for k := range m.entries {
		if len(k) > len(ns) && k[:len(ns)+1] == string(ns)+"|" {
			if _, ok := m.live(k); ok {
				n++
			}
		}
	}
	return n
}
