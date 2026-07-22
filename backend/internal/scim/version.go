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

package scim

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// generateVersion produces a stable weak ETag (RFC 7232) for the given resource
// state. Callers pass a struct capturing only the fields that should affect the
// version, so unrelated field changes don't bump the ETag.
func generateVersion(state any) string {
	b, err := json.Marshal(state)
	if err != nil {
		return `W/"0000000000000000"`
	}
	h := sha256.Sum256(b)
	return fmt.Sprintf("W/%q", hex.EncodeToString(h[:8]))
}

// checkIfMatch enforces RFC 7232 §3.1 / RFC 7644 §3.14 optimistic concurrency.
// ifMatch is the raw value of an incoming If-Match header — may be empty (no
// precondition requested), "*", a single ETag, or a comma-separated list.
// currentVersion is the resource's current weak ETag as produced by generateVersion.
//
// KNOWN LIMITATION: this check and the mutation callers apply afterward are not
// atomic (TOCTOU window). Two concurrent requests can both read the same current
// version, both pass this check, and both write, so the last write silently wins
// instead of the second one failing with 412. Closing this requires either a
// persisted version column with a conditional UPDATE ... WHERE version=? at the
// store layer, or a transaction with a row lock spanning the check and the
// mutation; group.GroupServiceInterface (and the equivalent for users) has
// neither today.
func checkIfMatch(ifMatch, currentVersion string) *tidcommon.ServiceError {
	ifMatch = strings.TrimSpace(ifMatch)
	if ifMatch == "" || ifMatch == "*" {
		return nil
	}
	for _, tag := range strings.Split(ifMatch, ",") {
		if normalizeETag(tag) == normalizeETag(currentVersion) {
			return nil
		}
	}
	return &ErrorPreconditionFailed
}

// normalizeETag strips the weak-validator prefix ("W/") and surrounding quotes
// so weak ETags compare equal regardless of formatting differences.
func normalizeETag(tag string) string {
	tag = strings.TrimSpace(tag)
	tag = strings.TrimPrefix(tag, "W/")
	return strings.Trim(tag, `"`)
}
