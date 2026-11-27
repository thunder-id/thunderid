// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	entitytypemodel "github.com/thunder-id/thunderid/internal/entitytype/model"
	"github.com/thunder-id/thunderid/internal/system/valueref"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// AgentOwnerVerificationAttribute names an agent's owner among the entity's recorded verifications. The
// owner is a system attribute, not a schema attribute, so the name is the owner's system attribute key.
const AgentOwnerVerificationAttribute = "owner"

// AgentOwnerContact returns the canonical value of the owner recorded in an agent's system attributes and its
// contact type, as AgentOwnerContactOf does. It returns empty strings for system attributes that name no owner
// or do not decode.
func AgentOwnerContact(systemAttributes json.RawMessage) (string, string) {
	var stored struct {
		Owner *providers.AgentOwner `json:"owner"`
	}
	if len(systemAttributes) == 0 || json.Unmarshal(systemAttributes, &stored) != nil {
		return "", ""
	}

	return AgentOwnerContactOf(stored.Owner)
}

// AgentOwnerContactOf returns the canonical value of an agent's owner and its contact type when the owner is
// managed in another identity system and named by an email address or a mobile number. It returns empty
// strings for no owner, a local owner, an owner named by a subject, or an owner that holds a reference.
func AgentOwnerContactOf(owner *providers.AgentOwner) (string, string) {
	if owner == nil {
		return "", ""
	}

	// A control plane keeps references for each deployment to resolve, which name no contact yet.
	if local, stated := owner.IsLocal.Bool(); local || !stated || valueref.IsWellFormedReference(owner.ID) {
		return "", ""
	}

	var contactType string
	switch owner.IDType {
	case providers.AgentOwnerIDTypeEmail:
		contactType = entitytypemodel.TypeEmail
	case providers.AgentOwnerIDTypeMobile:
		contactType = entitytypemodel.TypeMobile
	default:
		return "", ""
	}

	value := entitytypemodel.CanonicalContactValue(contactType, strings.TrimSpace(owner.ID))
	if value == "" {
		return "", ""
	}

	return value, contactType
}

// AgentOwnerVerifiedAt returns when an agent's owner was verified, or "" when it is not, and whether the owner can
// be verified at all. A recorded verification that does not read is not one.
func AgentOwnerVerifiedAt(systemAttributes json.RawMessage) (verifiedAt string, verifiable bool) {
	if value, _ := AgentOwnerContact(systemAttributes); value == "" {
		return "", false
	}
	entries, err := ReadRecordedAttributeVerifications(systemAttributes)
	if err != nil {
		return "", true
	}
	return entries[AgentOwnerVerificationAttribute].VerifiedAt, true
}

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
