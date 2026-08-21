// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package scim

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestMissingRequiredAttrs_InvalidSchemaJSON tests Missing Required Attrs for Invalid Schema JSON.
func TestMissingRequiredAttrs_InvalidSchemaJSON(t *testing.T) {
	_, err := missingRequiredAttrs(map[string]json.RawMessage{}, json.RawMessage(`not json`), false)
	require.Error(t, err)
}

// TestMissingRequiredAttrs_AllPresent tests Missing Required Attrs for All Present.
func TestMissingRequiredAttrs_AllPresent(t *testing.T) {
	schema := json.RawMessage(`{"given_name":{"required":true}}`)
	attrs := map[string]json.RawMessage{"given_name": json.RawMessage(`"Alice"`)}
	missing, err := missingRequiredAttrs(attrs, schema, false)
	require.NoError(t, err)
	require.Empty(t, missing)
}

// TestMissingRequiredAttrs_ReportsMissingSorted tests Missing Required Attrs for Reports Missing Sorted.
func TestMissingRequiredAttrs_ReportsMissingSorted(t *testing.T) {
	schema := json.RawMessage(`{
		"given_name":{"required":true},
		"family_name":{"required":true},
		"nickname":{"required":false}
	}`)
	missing, err := missingRequiredAttrs(map[string]json.RawMessage{}, schema, false)
	require.NoError(t, err)
	require.Equal(t, []string{"family_name", "given_name"}, missing)
}

// TestMissingRequiredAttrs_SkipCredentialTrue_OmitsCredential tests Missing Required Attrs for Skip
// Credential True Omits Credential.
func TestMissingRequiredAttrs_SkipCredentialTrue_OmitsCredential(t *testing.T) {
	schema := json.RawMessage(`{"password":{"required":true,"credential":true}}`)
	missing, err := missingRequiredAttrs(map[string]json.RawMessage{}, schema, true)
	require.NoError(t, err)
	require.Empty(t, missing)
}

// TestMissingRequiredAttrs_SkipCredentialFalse_IncludesCredential tests Missing Required Attrs for Skip
// Credential False Includes Credential.
func TestMissingRequiredAttrs_SkipCredentialFalse_IncludesCredential(t *testing.T) {
	schema := json.RawMessage(`{"password":{"required":true,"credential":true}}`)
	missing, err := missingRequiredAttrs(map[string]json.RawMessage{}, schema, false)
	require.NoError(t, err)
	require.Equal(t, []string{"password"}, missing)
}

// TestUndeclaredAttrs_ReportsUndeclaredSorted tests Undeclared Attrs for Reports Undeclared Sorted.
func TestUndeclaredAttrs_ReportsUndeclaredSorted(t *testing.T) {
	schema := json.RawMessage(`{
		"given_name":{"required":true}
	}`)
	extAttrs := map[string]json.RawMessage{
		"given_name": json.RawMessage(`"Alice"`),
		"extra_one":  json.RawMessage(`"one"`),
		"another":    json.RawMessage(`"two"`),
	}
	undeclared, err := undeclaredAttrs(extAttrs, schema)
	require.NoError(t, err)
	require.Equal(t, []string{"another", "extra_one"}, undeclared)
}

// TestMissingRequiredAttrs_CaseInsensitive tests Missing Required Attrs for Case Insensitive.
func TestMissingRequiredAttrs_CaseInsensitive(t *testing.T) {
	schema := json.RawMessage(`{"given_name":{"required":true}}`)
	attrs := map[string]json.RawMessage{"Given_Name": json.RawMessage(`"Alice"`)}
	missing, err := missingRequiredAttrs(attrs, schema, false)
	require.NoError(t, err)
	require.Empty(t, missing)
}

// TestUndeclaredAttrs_CaseInsensitive tests Undeclared Attrs for Case Insensitive.
func TestUndeclaredAttrs_CaseInsensitive(t *testing.T) {
	schema := json.RawMessage(`{"given_name":{"required":true}}`)
	extAttrs := map[string]json.RawMessage{
		"Given_Name": json.RawMessage(`"Alice"`),
	}
	undeclared, err := undeclaredAttrs(extAttrs, schema)
	require.NoError(t, err)
	require.Empty(t, undeclared)
}

// TestMapRawPropertyToSCIMAttribute_CredentialArrayItems_PropagatesNeverReturned tests Map Raw Property To
// SCIM Attribute for Credential Array Items Propagates Never Returned.
func TestMapRawPropertyToSCIMAttribute_CredentialArrayItems_PropagatesNeverReturned(t *testing.T) {
	def := rawPropertyDef{
		Type: rawPropertyTypeArray,
		Items: &rawPropertyDef{
			Type:       rawPropertyTypeObject,
			Credential: true,
			Properties: map[string]rawPropertyDef{
				"secret": {Type: "string", Credential: true},
			},
		},
	}
	attr := mapRawPropertyToSCIMAttribute("recovery_codes", def)
	require.Equal(t, scimReturnedNever, attr.Returned)
	require.Equal(t, scimMutabilityWriteOnly, attr.Mutability)
}
