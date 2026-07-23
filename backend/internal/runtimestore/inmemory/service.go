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

package inmemory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// keyFormat is the format string used to build in-memory store keys.
const keyFormat = "runtime:%s:%s:%s"

type entry struct {
	value     []byte
	expiresAt time.Time // zero value means no expiry
}

func (e *entry) isExpired() bool {
	return !e.expiresAt.IsZero() && time.Now().After(e.expiresAt)
}

// inMemoryStore implements the RuntimeStoreProvider interface using an in-process map.
type inMemoryStore struct {
	mu           sync.RWMutex
	data         map[string]*entry
	deploymentID string
	logger       *log.Logger
}

func newInMemoryStore(deploymentID string) providers.RuntimeStoreProvider {
	return &inMemoryStore{
		data:         make(map[string]*entry),
		deploymentID: deploymentID,
		logger:       log.GetLogger().With(log.String(log.LoggerKeyComponentName, "InMemoryStore")),
	}
}

// Put stores a value in the in-memory store with the specified TTL.
func (s *inMemoryStore) Put(ctx context.Context, namespace providers.RuntimeStoreNamespace,
	key string, value []byte, ttlSeconds int64) error {
	e := &entry{value: value}
	if ttlSeconds > 0 {
		e.expiresAt = time.Now().Add(time.Duration(ttlSeconds) * time.Second)
	}

	s.mu.Lock()
	s.data[s.getFormattedKey(namespace, key)] = e
	s.mu.Unlock()

	s.logger.Debug(ctx, "Stored in memory", log.String("key", key))
	return nil
}

// PutIfNotExists atomically stores a value only if the key does not already hold a non-expired value.
func (s *inMemoryStore) PutIfNotExists(ctx context.Context, namespace providers.RuntimeStoreNamespace,
	key string, value []byte, ttlSeconds int64) (bool, error) {
	e := &entry{value: value}
	if ttlSeconds > 0 {
		e.expiresAt = time.Now().Add(time.Duration(ttlSeconds) * time.Second)
	}

	fk := s.getFormattedKey(namespace, key)

	s.mu.Lock()
	existing, ok := s.data[fk]
	if ok && !existing.isExpired() {
		s.mu.Unlock()
		return false, nil
	}
	s.data[fk] = e
	s.mu.Unlock()

	s.logger.Debug(ctx, "Stored in memory", log.String("key", key))
	return true, nil
}

// Get retrieves a value from the in-memory store by its key.
func (s *inMemoryStore) Get(_ context.Context, namespace providers.RuntimeStoreNamespace,
	key string) ([]byte, error) {
	s.mu.RLock()
	e, ok := s.data[s.getFormattedKey(namespace, key)]
	s.mu.RUnlock()

	if !ok || e.isExpired() {
		return nil, nil
	}
	return e.value, nil
}

// Update updates the value associated with a key in the in-memory store.
func (s *inMemoryStore) Update(_ context.Context, namespace providers.RuntimeStoreNamespace,
	key string, value []byte) error {
	fk := s.getFormattedKey(namespace, key)

	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.data[fk]
	if !ok || e.isExpired() {
		return providers.ErrRuntimeStoreKeyNotFound
	}
	s.data[fk] = &entry{value: value, expiresAt: e.expiresAt}
	return nil
}

// Delete removes a value from the in-memory store by its key.
func (s *inMemoryStore) Delete(_ context.Context, namespace providers.RuntimeStoreNamespace,
	key string) error {
	s.mu.Lock()
	delete(s.data, s.getFormattedKey(namespace, key))
	s.mu.Unlock()
	return nil
}

// Take retrieves and removes a value from the in-memory store by its key.
func (s *inMemoryStore) Take(ctx context.Context, namespace providers.RuntimeStoreNamespace,
	key string) ([]byte, error) {
	fk := s.getFormattedKey(namespace, key)

	s.mu.Lock()
	e, ok := s.data[fk]
	if ok && !e.isExpired() {
		delete(s.data, fk)
	} else {
		e = nil
	}
	s.mu.Unlock()

	if e == nil {
		return nil, nil
	}

	s.logger.Debug(ctx, "Taken from memory", log.String("key", key))
	return e.value, nil
}

// ExtendTTL extends the TTL of an existing entry in the in-memory store.
func (s *inMemoryStore) ExtendTTL(_ context.Context, namespace providers.RuntimeStoreNamespace,
	key string, ttlSeconds int64) error {
	fk := s.getFormattedKey(namespace, key)

	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.data[fk]
	if !ok || e.isExpired() {
		return providers.ErrRuntimeStoreKeyNotFound
	}
	e.expiresAt = time.Now().Add(time.Duration(ttlSeconds) * time.Second)
	return nil
}

// getFormattedKey builds the in-memory key.
func (s *inMemoryStore) getFormattedKey(namespace providers.RuntimeStoreNamespace, key string) string {
	return fmt.Sprintf(keyFormat, s.deploymentID, namespace, key)
}
