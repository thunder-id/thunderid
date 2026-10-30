// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"encoding/json"
	"maps"
	"slices"

	entitytypemodel "github.com/thunder-id/thunderid/internal/entitytype/model"
	"github.com/thunder-id/thunderid/internal/identitygovernance"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// verificationValue returns the canonical form of a stored contact value, empty when it is absent or
// not a string.
func verificationValue(raw json.RawMessage, attributeType string) string {
	var value string
	if json.Unmarshal(raw, &value) != nil {
		return ""
	}
	return entitytypemodel.CanonicalContactValue(attributeType, value)
}

// invalidateVerification removes the verifications a write to the locked entity invalidates, and
// returns the system attributes to write with the attributes it cleared.
func invalidateVerification(current providers.Entity, nextType string, verifiableAttributes map[string]string,
	nextAttributes, systemAttributes json.RawMessage) (json.RawMessage, []clearedVerification, error) {
	if current.Category != providers.EntityCategoryUser {
		return systemAttributes, nil, nil
	}
	entries, err := identitygovernance.ReadVerifiedAttributes(current.SystemAttributes)
	if err != nil {
		return nil, nil, err
	}
	if len(entries) == 0 {
		return systemAttributes, nil, nil
	}
	var cleared []clearedVerification
	if current.Type != nextType {
		for _, name := range slices.Sorted(maps.Keys(entries)) {
			cleared = append(cleared, clearedVerification{attribute: name,
				reason: identitygovernance.VerificationUserTypeChanged})
		}
	} else {
		cleared, err = changedVerifiedValues(current, verifiableAttributes, nextAttributes, entries)
		if err != nil {
			return nil, nil, err
		}
	}
	if len(cleared) == 0 {
		return systemAttributes, nil, nil
	}
	for _, c := range cleared {
		delete(entries, c.attribute)
	}
	invalidated, err := identitygovernance.WithVerifiedAttributes(systemAttributes, entries)
	if err != nil {
		return nil, nil, err
	}
	return invalidated, cleared, nil
}

// changedVerifiedValues returns the verified attributes whose value the write changes or removes.
func changedVerifiedValues(current providers.Entity, verifiableAttributes map[string]string,
	nextAttributes json.RawMessage, entries map[string]providers.VerifiedAttribute) ([]clearedVerification, error) {
	var before, after map[string]json.RawMessage
	if len(current.Attributes) > 0 {
		if err := json.Unmarshal(current.Attributes, &before); err != nil {
			return nil, err
		}
	}
	if len(nextAttributes) > 0 {
		if err := json.Unmarshal(nextAttributes, &after); err != nil {
			return nil, err
		}
	}
	var cleared []clearedVerification
	for _, name := range slices.Sorted(maps.Keys(entries)) {
		newValue := verificationValue(after[name], verifiableAttributes[name])
		if verificationValue(before[name], verifiableAttributes[name]) == newValue {
			continue
		}
		reason := identitygovernance.VerificationValueChanged
		if newValue == "" {
			reason = identitygovernance.VerificationValueRemoved
		}
		cleared = append(cleared, clearedVerification{attribute: name, reason: reason})
	}
	return cleared, nil
}

// storedContactValue returns the canonical value an attribute holds, compared as invalidation compares
// it, or empty when the attribute is not one of verifiableAttributes or holds no string.
func storedContactValue(attributes json.RawMessage, verifiableAttributes map[string]string,
	attribute string) (string, error) {
	attributeType := verifiableAttributes[attribute]
	if attributeType == "" || len(attributes) == 0 {
		return "", nil
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(attributes, &values); err != nil {
		return "", err
	}
	return verificationValue(values[attribute], attributeType), nil
}
