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

package openid4vp

import (
	"context"

	"github.com/thunder-id/thunderid/internal/system/cache"
)

const stateCacheName = "OpenID4VPRequestState"

// stateStore persists short-lived OpenID4VP request state keyed by State.
type stateStore interface {
	Save(ctx context.Context, st *RequestState) error
	Get(ctx context.Context, state string) (*RequestState, bool)
	Delete(ctx context.Context, state string) error
}

// cacheStateStore backs stateStore with the system cache. Callers must honor
// RequestState.ExpiresAt for the per-request window; the cache TTL is global.
type cacheStateStore struct {
	cache cache.CacheInterface[*RequestState]
}

func newCacheStateStore(cm cache.CacheManagerInterface) stateStore {
	return &cacheStateStore{cache: cache.GetCache[*RequestState](cm, stateCacheName)}
}

func (c *cacheStateStore) Save(ctx context.Context, st *RequestState) error {
	return c.cache.Set(ctx, cache.CacheKey{Key: st.State}, st)
}

func (c *cacheStateStore) Get(ctx context.Context, state string) (*RequestState, bool) {
	return c.cache.Get(ctx, cache.CacheKey{Key: state})
}

func (c *cacheStateStore) Delete(ctx context.Context, state string) error {
	return c.cache.Delete(ctx, cache.CacheKey{Key: state})
}

// memoryStateStore is a process-local stateStore for test wiring. Not safe for concurrent use.
type memoryStateStore struct {
	entries map[string]*RequestState
}

func newInMemoryStateStore() stateStore {
	return &memoryStateStore{entries: map[string]*RequestState{}}
}

func (m *memoryStateStore) Save(_ context.Context, st *RequestState) error {
	m.entries[st.State] = st
	return nil
}

func (m *memoryStateStore) Get(_ context.Context, state string) (*RequestState, bool) {
	st, ok := m.entries[state]
	return st, ok
}

func (m *memoryStateStore) Delete(_ context.Context, state string) error {
	delete(m.entries, state)
	return nil
}
