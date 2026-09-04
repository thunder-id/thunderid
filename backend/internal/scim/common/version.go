// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// GenerateVersion produces a stable weak ETag (RFC 7232) for the given resource
// state. Callers pass a struct capturing only the fields that should affect the
// version, so unrelated field changes don't bump the ETag.
func GenerateVersion(state any) string {
	b, err := json.Marshal(state)
	if err != nil {
		return `W/"0000000000000000"`
	}
	h := sha256.Sum256(b)
	return fmt.Sprintf("W/%q", hex.EncodeToString(h[:8]))
}

// CheckIfMatch enforces RFC 7232 §3.1 / RFC 7644 §3.14 optimistic concurrency.
// ifMatch is the raw value of an incoming If-Match header — may be empty (no
// precondition requested), "*", a single ETag, or a comma-separated list.
// currentVersion is the resource's current weak ETag as produced by GenerateVersion.
//
// KNOWN LIMITATION: this check and the mutation callers apply afterward are not
// atomic (TOCTOU window). Two concurrent requests can both read the same current
// version, both pass this check, and both write, so the last write silently wins
// instead of the second one failing with 412. Closing this requires either a
// persisted version column with a conditional UPDATE ... WHERE version=? at the
// store layer, or a transaction with a row lock spanning the check and the
// mutation; group.GroupServiceInterface (and the equivalent for users) has
// neither today.
func CheckIfMatch(ifMatch, currentVersion string) *tidcommon.ServiceError {
	ifMatch = strings.TrimSpace(ifMatch)
	// Per RFC 7232 §3.1, an If-Match header value of "*" matches any existing representation.
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
