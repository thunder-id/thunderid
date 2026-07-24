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

// EnforcerInterface answers revocation checks for the Resource Server enforcement point. jti is the
// token's own identifier and tokenFamilyID is its grant's token family id (tfid); a token is rejected
// when either is on the cached deny list. Reads are served entirely from the in-memory cache, so the
// request hot path never touches the source.
type EnforcerInterface interface {
	// EnsureNotRevoked returns nil when the token may proceed and errTokenRevoked when its jti or its
	// token family id is present in the cached deny list. Empty jti and tokenFamilyID are each a no-op.
	EnsureNotRevoked(ctx context.Context, jti, tokenFamilyID string) error
}

// enforcer serves revocation checks from the in-memory cache. It holds no write capability.
type enforcer struct {
	cache *revokedCache
}

// newEnforcer creates an enforcer backed by the given cache.
func newEnforcer(cache *revokedCache) *enforcer {
	return &enforcer{cache: cache}
}

// EnsureNotRevoked returns errTokenRevoked when the token's jti or token family id is on the cached
// deny list, nil otherwise. Empty identifiers are treated as nothing to enforce.
func (e *enforcer) EnsureNotRevoked(_ context.Context, jti, tokenFamilyID string) error {
	if jti != "" && e.cache.isTokenRevoked(jti) {
		return errTokenRevoked
	}
	if tokenFamilyID != "" && e.cache.isTokenFamilyRevoked(tokenFamilyID) {
		return errTokenRevoked
	}
	return nil
}

// noopEnforcer is returned when RS revocation enforcement is disabled; it never rejects a token.
type noopEnforcer struct{}

// EnsureNotRevoked always returns nil.
func (noopEnforcer) EnsureNotRevoked(_ context.Context, _, _ string) error { return nil }
