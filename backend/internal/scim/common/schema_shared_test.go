// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestParseRawProperties_ValidJSON_ReturnsPropertyMap tests that a well-formed schema
// property map unmarshals into RawPropertyDef entries keyed by property name.
func TestParseRawProperties_ValidJSON_ReturnsPropertyMap(t *testing.T) {
	schema := json.RawMessage(`{
		"username": {"type": "string", "required": true},
		"password": {"type": "string", "credential": true}
	}`)

	props, err := ParseRawProperties(schema)

	require.NoError(t, err)
	require.Len(t, props, 2)
	require.True(t, props["username"].Required)
	require.True(t, props["password"].Credential)
}

// TestParseRawProperties_EmptySchema_ReturnsNil tests that an empty/absent schema
// returns a nil map with no error, matching the zero-value contract callers rely on.
func TestParseRawProperties_EmptySchema_ReturnsNil(t *testing.T) {
	props, err := ParseRawProperties(nil)

	require.NoError(t, err)
	require.Nil(t, props)
}

// TestParseRawProperties_InvalidJSON_ReturnsError tests that malformed schema JSON
// surfaces a parse error instead of silently returning an empty/partial map.
func TestParseRawProperties_InvalidJSON_ReturnsError(t *testing.T) {
	props, err := ParseRawProperties(json.RawMessage(`{not-json`))

	require.Error(t, err)
	require.Nil(t, props)
}

// TestCredentialCharacteristics_Credential_ReturnsNeverWriteOnly tests that a
// credential-flagged property is declared never-returned and write-only, per RFC 7643 §7.
func TestCredentialCharacteristics_Credential_ReturnsNeverWriteOnly(t *testing.T) {
	returned, mutability := CredentialCharacteristics(true)

	require.Equal(t, ReturnedNever, returned)
	require.Equal(t, MutabilityWriteOnly, mutability)
}

// TestCredentialCharacteristics_NonCredential_ReturnsDefaultReadWrite tests that a
// non-credential property gets the RFC 7643 §7 baseline characteristics.
func TestCredentialCharacteristics_NonCredential_ReturnsDefaultReadWrite(t *testing.T) {
	returned, mutability := CredentialCharacteristics(false)

	require.Equal(t, ReturnedDefault, returned)
	require.Equal(t, MutabilityReadWrite, mutability)
}
