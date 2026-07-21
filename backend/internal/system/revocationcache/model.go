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

import "time"

// revokedEntry is one non-expired deny-list record returned by a syncSource and held in the cache.
type revokedEntry struct {
	// JTI is the token identifier and the cache lookup key.
	JTI string
	// ExpiryTime is the revoked token's original expiry; the entry is prunable once it passes.
	ExpiryTime time.Time
}
