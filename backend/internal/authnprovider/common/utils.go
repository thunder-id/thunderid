// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"encoding/json"
	"fmt"
	"time"
)

// ValidUntilOf returns the instant the lifespan held in the given system attributes ends. bounded
// is false when the lifespan never ends: the key is absent, or holds the unbounded sentinel. A
// stored value that does not parse is an error rather than an unbounded lifespan, so a corrupt value
// never lifts the bound.
func ValidUntilOf(systemAttributes json.RawMessage) (validUntil time.Time, bounded bool, err error) {
	if len(systemAttributes) == 0 {
		return time.Time{}, false, nil
	}
	var attrs map[string]json.RawMessage
	if err := json.Unmarshal(systemAttributes, &attrs); err != nil {
		return time.Time{}, false, fmt.Errorf("failed to unmarshal system attributes: %w", err)
	}
	raw, ok := attrs[SystemAttrValidUntil]
	if !ok {
		return time.Time{}, false, nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return time.Time{}, false, fmt.Errorf("invalid %s system attribute: %w", SystemAttrValidUntil, err)
	}
	if value == UnboundedValidUntil {
		return time.Time{}, false, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("invalid %s system attribute: %w", SystemAttrValidUntil, err)
	}
	return parsed.UTC(), true, nil
}
