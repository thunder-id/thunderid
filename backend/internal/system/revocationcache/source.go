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

// StatusListSource resolves a Token Status List's revocation entries by the list's public URI. It is
// the pluggable seam that lets the Resource Server read lists in-process from the local Status List
// subsystem today and fetch signed Status List Tokens from a remote Status Provider in future without
// changing the cache or enforcer. It is wired at the composition root, so this package never imports
// the Status List subsystem.
type StatusListSource interface {
	// Fetch returns the recorded (non-VALID) entries of the list identified by uri, keyed by index,
	// together with the list's capacity. An in-bounds index absent from the map has status VALID.
	// found is false when no such list exists, so the cache can fail closed on an unresolvable
	// reference rather than treating an empty result as an all-VALID list.
	Fetch(ctx context.Context, uri string) (statuses map[int64]int, capacity int64, found bool, err error)
}
