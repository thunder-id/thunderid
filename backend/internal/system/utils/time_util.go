// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"fmt"
	"strings"
	"time"
)

const dbTimeFormat = "2006-01-02 15:04:05.999999999"

// ParseDBTimeField parses a time value read from a database column.
// Accepts a time.Time (returned as-is) or a string in SQLite datetime format,
// normalising to UTC. Falls back to ISO 8601 if the primary format does not match.
func ParseDBTimeField(field interface{}, fieldName string) (time.Time, error) {
	switch v := field.(type) {
	case string:
		parts := strings.SplitN(v, " ", 3)
		trimmed := v
		if len(parts) >= 2 {
			trimmed = parts[0] + " " + parts[1]
		}
		if t, err := time.Parse(dbTimeFormat, trimmed); err == nil {
			return t.UTC(), nil
		}
		t, err := time.Parse("2006-01-02T15:04:05Z07:00", v)
		if err != nil {
			return time.Time{}, fmt.Errorf("error parsing %s: %w", fieldName, err)
		}
		return t.UTC(), nil
	case time.Time:
		return v.UTC(), nil
	default:
		return time.Time{}, fmt.Errorf("unexpected type for %s", fieldName)
	}
}

// FormatRFC3339 renders t as RFC 3339 in UTC, or as an empty string for the zero time, such as a deadline
// that isn't set.
func FormatRFC3339(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
