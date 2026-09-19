// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sharing

import (
	"fmt"

	"github.com/thunder-id/thunderid/internal/system/utils"
)

// newID returns an identifier for a policy, target or rule row.
func newID() string {
	id, err := utils.GenerateUUIDv7()
	if err != nil {
		// A time-ordered id is preferred but not load-bearing, so a random one is an acceptable
		// fallback rather than a reason to fail the write.
		return utils.GenerateUUID()
	}
	return id
}

// nullableString maps an empty string to a SQL null, so an absent parent or target organization
// unit stores as null rather than as an empty string a foreign key would reject.
func nullableString(v string) interface{} {
	if v == "" {
		return nil
	}
	return v
}

// dedupeStrings removes repeats while preserving order.
func dedupeStrings(in []string) []string {
	if len(in) < 2 {
		return in
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		if _, dup := seen[v]; dup {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

// stringField reads a column that may arrive as a string, as bytes, or as null.
func stringField(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return fmt.Sprintf("%v", t)
	}
}

// boolField reads a column that may arrive as a bool or, on engines without one, as an integer.
func boolField(v interface{}) bool {
	switch t := v.(type) {
	case bool:
		return t
	case int64:
		return t != 0
	case int:
		return t != 0
	case []byte:
		return len(t) == 1 && t[0] == 1
	default:
		return false
	}
}

// intField reads a numeric column across the integer types the drivers return.
func intField(v interface{}) int {
	switch t := v.(type) {
	case int:
		return t
	case int32:
		return int(t)
	case int64:
		return int(t)
	case float64:
		return int(t)
	default:
		return 0
	}
}

// maxInt returns the larger of two integers.
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// copyStrings returns a copy of a member list, so a stored rule cannot be mutated through a
// pointer the caller still holds.
func copyStrings(in *[]string) *[]string {
	if in == nil {
		return nil
	}
	out := append([]string{}, *in...)
	return &out
}
