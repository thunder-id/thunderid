// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package users

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	scim "github.com/thunder-id/thunderid/internal/scim/common"
)

// ---------------------------------------------------------------------------
// Users — POST / PUT validation tests
// ---------------------------------------------------------------------------

// TestValidateSCIMUserRequest tests Validate SCIM User Request.
func TestValidateSCIMUserRequest(t *testing.T) {
	validURN := "urn:thunderid:params:scim:schemas:employee:2.0:User"

	tests := []struct {
		name         string
		body         []byte
		wantErrCode  string
		wantUserType string
		wantExtURN   string
	}{
		{
			name:        "InvalidJSON",
			body:        []byte(`not json`),
			wantErrCode: scim.ErrorInvalidRequestBody.Code,
		},
		{
			name:        "MissingSchemas",
			body:        []byte(`{"userName":"alice"}`),
			wantErrCode: scim.ErrorMissingSchemas.Code,
		},
		{
			name:        "EmptySchemas",
			body:        []byte(`{"schemas":[],"` + validURN + `":{}}`),
			wantErrCode: scim.ErrorMissingSchemas.Code,
		},
		{
			name:        "DuplicateSchemas",
			body:        []byte(`{"schemas":["` + validURN + `","` + validURN + `"],"` + validURN + `":{}}`),
			wantErrCode: scim.ErrorDuplicateSchemas.Code,
		},
		{
			name:        "MissingThunderIDURN",
			body:        []byte(`{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"]}`),
			wantErrCode: scim.ErrorMissingCustomSchema.Code,
		},
		{
			name:         "CoreOnly_NoThunderIDURN_ParsesWithEmptyUserType",
			body:         []byte(`{"schemas":["` + scim.SCIMCoreUserSchemaURN + `"],"userName":"alice"}`),
			wantErrCode:  "",
			wantUserType: "",
			wantExtURN:   "",
		},
		{
			name: "MultipleThunderIDURNs",
			body: []byte(`{` +
				`"schemas":["urn:thunderid:params:scim:schemas:employee:2.0:User",` +
				`"urn:thunderid:params:scim:schemas:person:2.0:User"],` +
				`"urn:thunderid:params:scim:schemas:employee:2.0:User":{},` +
				`"urn:thunderid:params:scim:schemas:person:2.0:User":{}}`),
			wantErrCode: scim.ErrorMultipleCustomSchemas.Code,
		},
		{
			name: "MalformedCustomSchemaURN_WrongSuffix",
			body: []byte(
				`{"schemas":["urn:thunderid:params:scim:schemas:employee:2.0:Group"],` +
					`"urn:thunderid:params:scim:schemas:employee:2.0:Group":{}}`),
			wantErrCode: scim.ErrorInvalidCustomSchemaURN.Code,
		},
		{
			name: "MalformedCustomSchemaURN_EmptyUserType",
			body: []byte(
				`{"schemas":["urn:thunderid:params:scim:schemas::2.0:User"],` +
					`"urn:thunderid:params:scim:schemas::2.0:User":{}}`),
			wantErrCode: scim.ErrorInvalidCustomSchemaURN.Code,
		},
		{
			name:         "OmittedExtensionObject_DefaultsToEmpty",
			body:         []byte(`{"schemas":["` + scim.SCIMCoreUserSchemaURN + `","` + validURN + `"],"userName":"alice"}`),
			wantErrCode:  "",
			wantUserType: "employee",
			wantExtURN:   validURN,
		},
		{
			name:        "InvalidExtensionObjectJSON",
			body:        []byte(`{"schemas":["` + validURN + `"],"` + validURN + `":"not-an-object"}`),
			wantErrCode: scim.ErrorMissingCustomSchemaObject.Code,
		},
		{
			name: "ValidPayload",
			body: []byte(`{
				"schemas":["` + scim.SCIMCoreUserSchemaURN + `","` + validURN + `"],
				"` + validURN + `":{"department":"engineering"},
				"userName":"alice"
			}`),
			wantErrCode:  "",
			wantUserType: "employee",
			wantExtURN:   validURN,
		},
		{
			name:         "ValidPayload_ExtensionOnly_NoCoreAttrs_CoreSchemaOmitted",
			body:         []byte(`{"schemas":["` + validURN + `"],"` + validURN + `":{"given_name":"alice"}}`),
			wantErrCode:  "",
			wantUserType: "employee",
			wantExtURN:   validURN,
		},
		{
			name:        "CoreAttrsPresent_CoreUserSchemaMissing",
			body:        []byte(`{"schemas":["` + validURN + `"],"` + validURN + `":{}, "userName":"alice"}`),
			wantErrCode: scim.ErrorMissingCoreUserSchema.Code,
		},
		{
			name: "UndeclaredThunderIDExtensionKey_NoThunderIDURNInSchemas",
			body: []byte(`{"schemas":["` + scim.SCIMCoreUserSchemaURN + `"],` +
				`"` + validURN + `":{"department":"engineering"},"userName":"alice"}`),
			wantErrCode: scim.ErrorUndeclaredCustomSchemaObject.Code,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			payload, svcErr := parseAndValidateSCIMUserRequest(tc.body)
			if tc.wantErrCode != "" {
				require.NotNil(t, svcErr, "expected a ServiceError")
				require.Equal(t, tc.wantErrCode, svcErr.Code)
				require.Nil(t, payload)
				return
			}
			require.Nil(t, svcErr)
			require.NotNil(t, payload)
			require.Equal(t, tc.wantUserType, payload.UserTypeName)
			require.Equal(t, tc.wantExtURN, payload.ExtensionURN)
		})
	}
}

// ---------------------------------------------------------------------------
// Schema-content validation tests (missingRequiredAttrs / undeclaredAttrs)
// ---------------------------------------------------------------------------

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
