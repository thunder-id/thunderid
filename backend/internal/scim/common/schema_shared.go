// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"encoding/json"
	"strings"
)

// RawPropertyDef is the internal representation of a single property from a
// ThunderID EntityType JSON schema. Used for unmarshalling during SCIM schema
// mapping and validation.
type RawPropertyDef struct {
	Type        string                    `json:"type"`
	Required    bool                      `json:"required"`
	Unique      bool                      `json:"unique"`
	Credential  bool                      `json:"credential"`
	DisplayName string                    `json:"displayName"`
	Enum        []json.RawMessage         `json:"enum"`       // string: ["a","b"] / number: [1,2]
	Regex       string                    `json:"regex"`      // string type only; mutually exclusive with Pattern
	Pattern     string                    `json:"pattern"`    // alias for Regex; model rejects both being set
	Properties  map[string]RawPropertyDef `json:"properties"` // for type=object
	Items       *RawPropertyDef           `json:"items"`      // for type=array
}

// BuildSchemaURN returns the canonical lowercase SCIM extension URN for a ThunderID user type.
// Format: urn:thunderid:params:scim:schemas:<userTypeName>:2.0:User
func BuildSchemaURN(userTypeName string) string {
	return ThunderIDURNPrefix + strings.ToLower(userTypeName) + ThunderIDURNSuffix
}

// ParseUserTypeFromSchemaURN extracts the user type name from a ThunderID extension URN.
// Matching is case-insensitive per the proposal decision.
// Returns the name and true on success; empty string and false if the URN is not a
// well-formed ThunderID extension URN.
func ParseUserTypeFromSchemaURN(schemaURN string) (string, bool) {
	lower := strings.ToLower(strings.TrimSpace(schemaURN))

	lowerPrefix := strings.ToLower(ThunderIDURNPrefix)
	lowerSuffix := strings.ToLower(ThunderIDURNSuffix)

	withoutPrefix, ok := strings.CutPrefix(lower, lowerPrefix)
	if !ok {
		return "", false
	}

	name, ok := strings.CutSuffix(withoutPrefix, lowerSuffix)
	if !ok {
		return "", false
	}

	if name == "" {
		return "", false
	}

	return name, true
}

// HasSchemaURN checks if a target schema URN exists in a list of schemas (case-insensitive).
func HasSchemaURN(schemas []string, targetURN string) bool {
	for _, urn := range schemas {
		if strings.EqualFold(strings.TrimSpace(urn), targetURN) {
			return true
		}
	}
	return false
}
