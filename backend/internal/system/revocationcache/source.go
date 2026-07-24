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

// syncSource supplies the current deny-list snapshot to the cache. It is the pluggable seam that lets
// the Resource Server sync from the runtime persistent DB today and from another DB, endpoint, or event stream
// in future without changing the cache, enforcer, or syncer.
type syncSource interface {
	// Snapshot returns all currently-revoked, non-expired entries for this deployment: the revoked
	// single-token jtis and the revoked token-family ids.
	Snapshot(ctx context.Context) (revokedSnapshot, error)
}
