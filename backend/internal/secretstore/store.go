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
	"fmt"
	"sort"
	"sync"
	"time"
)

// Store holds the secrets this server serves and accepts writes for.
//
// Writes arrive from a control plane the moment a secret is created or updated, so they must take
// effect immediately rather than waiting for a configuration promotion: a credential that reaches a
// data plane late is a credential that rejects logins in between.
//
// Where the secrets actually live is the backend's business. The store keeps what it last read in
// memory so that resolving a reference during a request costs nothing, and reloads when the backend
// says that cache can go stale beneath it.
type Store struct {
	backend Backend
	ttl     time.Duration

	mu       sync.RWMutex
	secrets  map[string]Secret
	loadedAt time.Time
}

// NewStore opens a store over a backend, loading what it already holds.
//
// The initial load failing is reported rather than fatal: a vault that is briefly unreachable at
// startup should not stop the server coming up, and the next read reloads.
func NewStore(ctx context.Context, backend Backend) (*Store, error) {
	if backend == nil {
		return nil, fmt.Errorf("a secret store needs a backend")
	}
	s := &Store{backend: backend, ttl: backend.CacheTTL(), secrets: map[string]Secret{}}
	err := s.reload(ctx)
	return s, err
}

// Backend reports where this store keeps its secrets, for logging.
func (s *Store) Backend() string { return s.backend.Name() }

// Put stores a secret, replacing any entry of the same name.
func (s *Store) Put(ctx context.Context, secret Secret) error {
	if err := secret.Validate(); err != nil {
		return err
	}
	if err := s.backend.Put(ctx, secret); err != nil {
		return err
	}
	s.mu.Lock()
	s.secrets[secret.Name] = secret
	s.mu.Unlock()
	return nil
}

// Get returns a secret by name.
func (s *Store) Get(ctx context.Context, name string) (Secret, bool) {
	s.refreshIfStale(ctx)
	s.mu.RLock()
	defer s.mu.RUnlock()
	secret, ok := s.secrets[name]
	return secret, ok
}

// Delete removes a secret. Removing an absent name is not an error, so a repeated delete is safe.
func (s *Store) Delete(ctx context.Context, name string) error {
	if err := s.backend.Delete(ctx, name); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.secrets, name)
	s.mu.Unlock()
	return nil
}

// All returns every secret, keyed by name. This is what a data plane loads at startup.
func (s *Store) All(ctx context.Context) map[string]Secret {
	s.refreshIfStale(ctx)
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]Secret, len(s.secrets))
	for name, secret := range s.secrets {
		out[name] = secret
	}
	return out
}

// Names returns the stored names in order, for diagnostics that must not expose values.
func (s *Store) Names(ctx context.Context) []string {
	s.refreshIfStale(ctx)
	s.mu.RLock()
	defer s.mu.RUnlock()
	names := make([]string, 0, len(s.secrets))
	for name := range s.secrets {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Count reports how many secrets are held, without reloading. It is for startup logging, which must
// not log names or values.
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.secrets)
}

// Refresh reloads every secret from the backend.
func (s *Store) Refresh(ctx context.Context) error { return s.reload(ctx) }

// refreshIfStale reloads when the cache has outlived the backend's TTL.
//
// A failed reload is deliberately not propagated: the cache is still the best answer available, and a
// vault that has gone briefly unreachable should degrade to slightly stale secrets rather than to
// none. The load time advances either way, so a vault that stays down is retried once per TTL rather
// than on every single read.
func (s *Store) refreshIfStale(ctx context.Context) {
	if s.ttl <= 0 {
		return
	}
	s.mu.RLock()
	fresh := time.Since(s.loadedAt) < s.ttl
	s.mu.RUnlock()
	if fresh {
		return
	}
	_ = s.reload(ctx)
}

// reload replaces the cache with what the backend holds.
func (s *Store) reload(ctx context.Context) error {
	secrets, err := s.backend.Load(ctx)
	if err != nil {
		s.mu.Lock()
		s.loadedAt = time.Now()
		s.mu.Unlock()
		return err
	}
	s.mu.Lock()
	s.secrets = secrets
	s.loadedAt = time.Now()
	s.mu.Unlock()
	return nil
}
