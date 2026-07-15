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

// EnforcerInterface answers revocation checks for the Resource Server enforcement point using the
// Token Status List reference carried by the token. Reads are served from the in-memory cache, so the
// request hot path only touches the source when a referenced list is missing or stale.
type EnforcerInterface interface {
	// EnsureNotRevoked returns errTokenRevoked when the token whose status-list reference is
	// (statusURI, statusIdx) has a non-VALID status, and nil when it is VALID or carries no reference
	// (empty statusURI). It fails closed with errStatusUnavailable whenever the status is not resolvable
	// — the referenced list does not exist, the index is out of the list's bounds, or the list has
	// never been cached and cannot be fetched — so an unknown status never admits a possibly-revoked
	// token (draft-ietf-oauth-status-list §8.3). A transient outage on an already-cached list is masked
	// by the last-known-good snapshot.
	EnsureNotRevoked(ctx context.Context, statusURI string, statusIdx int64) error
}

// enforcer serves revocation checks from the status cache. It holds no write capability.
type enforcer struct {
	cache *statusCache
}

// newEnforcer creates an enforcer backed by the given cache.
func newEnforcer(cache *statusCache) *enforcer {
	return &enforcer{cache: cache}
}

// EnsureNotRevoked returns errTokenRevoked when the referenced token has a non-VALID status,
// errStatusUnavailable when the status cannot be resolved (fail closed), and nil when the token is
// valid or carries no reference.
func (e *enforcer) EnsureNotRevoked(ctx context.Context, statusURI string, statusIdx int64) error {
	if statusURI == "" {
		return nil
	}
	status, available := e.cache.statusAt(ctx, statusURI, statusIdx)
	if !available {
		return errStatusUnavailable
	}
	if status != statusValid {
		return errTokenRevoked
	}
	return nil
}

// noopEnforcer is returned when RS revocation enforcement is disabled; it never rejects a token.
type noopEnforcer struct{}

// EnsureNotRevoked always returns nil.
func (noopEnforcer) EnsureNotRevoked(context.Context, string, int64) error { return nil }
