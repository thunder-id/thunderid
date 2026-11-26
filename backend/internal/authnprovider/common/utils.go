// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
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

// AuthenticationMethodOf returns the method a credentials map is checked under. It follows the order the default
// provider dispatches in, the order of InternalCredentialTypes. A provisioned entity ID is admitted as a
// federated sign-in, and any other credential is the credential method unless every key is a system credential.
func AuthenticationMethodOf(credentials map[string]interface{}) providers.AuthenticationMethod {
	for _, credentialType := range InternalCredentialTypes {
		if _, ok := credentials[credentialType]; !ok {
			continue
		}
		switch credentialType {
		case CredentialTypePasskey:
			return providers.AuthenticationMethodPasskey
		case CredentialTypeOTP:
			return providers.AuthenticationMethodOTP
		case CredentialTypeMagicLink:
			return providers.AuthenticationMethodMagicLink
		case CredentialTypeOpenID4VP:
			return providers.AuthenticationMethodOpenID4VP
		case CredentialTypeFederated, CredentialTypeProvisionedEntityID:
			return providers.AuthenticationMethodFederated
		}
	}
	for key := range credentials {
		if !slices.Contains(SystemCredentialTypes, key) {
			return providers.AuthenticationMethodCredential
		}
	}
	return providers.AuthenticationMethodSystemCredential
}
