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

import "context"

// EnforcerInterface answers revocation checks for the Resource Server enforcement point. It is
// token-format agnostic: id is the token's revocation identifier (the jti for JWTs today, an opaque
// token handle in future). Reads are served entirely from the in-memory cache, so the request hot
// path never touches the source.
type EnforcerInterface interface {
	// EnsureNotRevoked returns nil when the token identified by id may proceed and errTokenRevoked
	// when id is present in the cached deny list. An empty id is a no-op (nothing to enforce).
	EnsureNotRevoked(ctx context.Context, id string) error
}

// enforcer serves revocation checks from the in-memory cache. It holds no write capability.
type enforcer struct {
	cache *revokedCache
}

// newEnforcer creates an enforcer backed by the given cache.
func newEnforcer(cache *revokedCache) *enforcer {
	return &enforcer{cache: cache}
}

// EnsureNotRevoked returns errTokenRevoked when id is present in the cached deny list, nil otherwise.
// An empty id is treated as nothing to enforce.
func (e *enforcer) EnsureNotRevoked(_ context.Context, id string) error {
	if id == "" {
		return nil
	}
	if e.cache.isRevoked(id) {
		return errTokenRevoked
	}
	return nil
}

// noopEnforcer is returned when RS revocation enforcement is disabled; it never rejects a token.
type noopEnforcer struct{}

// EnsureNotRevoked always returns nil.
func (noopEnforcer) EnsureNotRevoked(_ context.Context, _ string) error { return nil }
