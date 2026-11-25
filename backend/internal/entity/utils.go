// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// ReadRecordedAttributeVerifications reads stored evidence without applying verification policies.
func ReadRecordedAttributeVerifications(systemAttributes json.RawMessage) (
	map[string]providers.VerifiedAttribute, error) {
	var attributes map[string]json.RawMessage
	if len(systemAttributes) > 0 {
		if err := json.Unmarshal(systemAttributes, &attributes); err != nil {
			return nil, fmt.Errorf("failed to read verification metadata: %w", err)
		}
	}

	raw, exists := attributes[systemAttrAttributeVerification]
	if !exists {
		return map[string]providers.VerifiedAttribute{}, nil
	}

	var entries map[string]providers.VerifiedAttribute
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("failed to read verification entries: %w", err)
	}

	if entries == nil {
		return nil, errors.New("invalid verification entries")
	}

	for name, entry := range entries {
		if _, err := time.Parse(time.RFC3339Nano, entry.VerifiedAt); err != nil {
			delete(entries, name)
		}
	}

	return entries, nil
}
