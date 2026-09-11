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

// ParseRawProperties parses a ThunderID entity type's raw JSON schema into its property
// definitions. Shared by discovery (schema declaration) and users (response filtering) so
// both read the same schema the same way.
func ParseRawProperties(schema json.RawMessage) (map[string]RawPropertyDef, error) {
	if len(schema) == 0 {
		return nil, nil
	}
	var rawProps map[string]RawPropertyDef
	if err := json.Unmarshal(schema, &rawProps); err != nil {
		return nil, err
	}
	return rawProps, nil
}

// CredentialCharacteristics returns the RFC 7643 §7 Returned/Mutability pair implied by
// whether a property is credential-flagged, the only schema signal that currently affects
// what's returned. Shared by discovery (declaring /Schemas) and users (deciding what to omit
// from responses), so the two can never independently drift.
func CredentialCharacteristics(credential bool) (returned Returned, mutability Mutability) {
	if credential {
		return ReturnedNever, MutabilityWriteOnly
	}
	return ReturnedDefault, MutabilityReadWrite
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
