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

// Package revocationcache answers the Resource Server's token-revocation checks from Token Status
// Lists (draft-ietf-oauth-status-list). It is read-only: it lazily caches lists by URI from a
// pluggable source and reads a token's status bit; it never writes revocations. The Status List write
// path lives in internal/oauth/oauth2/revocation and the list store in internal/tokenstatus, neither of
// which is imported here — the source is wired at the composition root.
package revocationcache

import "time"

// defaultRefreshInterval bounds cache staleness when cfg.RefreshInterval is not a positive duration.
const defaultRefreshInterval = time.Minute

// Initialize builds the RS revocation enforcer from cfg and the given status list source. It returns a
// no-op enforcer when enforcement is disabled OR when no source is wired: revocation status is carried
// only by the Token Status List, so with the list feature off there is nothing to enforce and the
// enforcer must not reject tokens. When enabled with a source it returns an enforcer backed by a
// lazily-populated, TTL-refreshed cache — there is no background sync.
func Initialize(cfg Config, source StatusListSource) EnforcerInterface {
	if !cfg.Enabled || source == nil {
		return noopEnforcer{}
	}

	interval := cfg.RefreshInterval
	if interval <= 0 {
		interval = defaultRefreshInterval
	}

	return newEnforcer(newStatusCache(source, interval))
}
