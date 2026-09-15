// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package groups

import (
	"testing"

	"github.com/stretchr/testify/require"

	scim "github.com/thunder-id/thunderid/internal/scim/common"
)

// ---------------------------------------------------------------------------
// Groups — POST / PUT validation tests
// ---------------------------------------------------------------------------

// TestValidateSCIMGroupWriteRequest_InvalidJSON tests Validate SCIM Group Write Request for Invalid JSON.
func TestValidateSCIMGroupWriteRequest_InvalidJSON(t *testing.T) {
	_, err := parseAndValidateSCIMGroupWriteRequest([]byte(`not json`))
	require.Equal(t, scim.ErrorInvalidRequestBody.Code, err.Code)
}

// TestValidateSCIMGroupWriteRequest_MissingDisplayName tests Validate SCIM Group Write Request for Missing
// Display Name.
func TestValidateSCIMGroupWriteRequest_MissingDisplayName(t *testing.T) {
	body := `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:Group"],"displayName":""}`
	_, err := parseAndValidateSCIMGroupWriteRequest([]byte(body))
	require.Equal(t, scim.ErrorInvalidRequestBody.Code, err.Code)
}

// TestValidateSCIMGroupWriteRequest_MissingCoreGroupSchema tests Validate SCIM Group Write Request for
// Missing Core Group Schema.
func TestValidateSCIMGroupWriteRequest_MissingCoreGroupSchema(t *testing.T) {
	body := `{"schemas":[],"displayName":"Eng"}`
	_, err := parseAndValidateSCIMGroupWriteRequest([]byte(body))
	require.Equal(t, scim.ErrorMissingCoreGroupSchema.Code, err.Code)
}

// TestValidateSCIMGroupWriteRequest_Valid tests Validate SCIM Group Write Request for Valid.
func TestValidateSCIMGroupWriteRequest_Valid(t *testing.T) {
	body := `{
		"schemas":["urn:ietf:params:scim:schemas:core:2.0:Group"],
		"displayName":"Engineering",
		"members":[{"value":"user-1","type":"User"}]
	}`
	payload, err := parseAndValidateSCIMGroupWriteRequest([]byte(body))
	require.Nil(t, err)
	require.Equal(t, "Engineering", payload.DisplayName)
	require.Len(t, payload.Members, 1)
}

// TestValidateSCIMGroupWriteRequest_NoMembers tests Validate SCIM Group Write Request for No Members.
func TestValidateSCIMGroupWriteRequest_NoMembers(t *testing.T) {
	body := `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:Group"],"displayName":"Empty"}`
	payload, err := parseAndValidateSCIMGroupWriteRequest([]byte(body))
	require.Nil(t, err)
	require.Equal(t, "Empty", payload.DisplayName)
	require.Empty(t, payload.Members)
}

// ---------------------------------------------------------------------------
// Groups — PATCH validation tests
// ---------------------------------------------------------------------------

// TestValidateSCIMGroupPatchRequest_MissingSchema tests Validate SCIM Group Patch Request for Missing Schema.
func TestValidateSCIMGroupPatchRequest_MissingSchema(t *testing.T) {
	body := `{"Operations":[{"op":"replace","path":"displayName","value":"X"}]}`
	_, err := parseAndValidateSCIMGroupPatchRequest([]byte(body))
	require.Equal(t, scim.ErrorMissingSchemas.Code, err.Code)
}

// TestValidateSCIMGroupPatchRequest_InvalidJSON tests Validate SCIM Group Patch Request for Invalid JSON.
func TestValidateSCIMGroupPatchRequest_InvalidJSON(t *testing.T) {
	_, err := parseAndValidateSCIMGroupPatchRequest([]byte(`not json`))
	require.Equal(t, scim.ErrorInvalidRequestBody.Code, err.Code)
}

// TestValidateSCIMGroupPatchOp_DisplayNameReplace tests Validate SCIM Group Patch Op for Display Name Replace.
func TestValidateSCIMGroupPatchOp_DisplayNameReplace(t *testing.T) {
	body := `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
		"Operations": [{"op": "replace", "path": "displayName", "value": "New Name"}]
	}`
	actions, err := parseAndValidateSCIMGroupPatchRequest([]byte(body))
	require.Nil(t, err)
	require.Len(t, actions, 1)
	require.Equal(t, scimGroupPatchTargetDisplayName, actions[0].Target)
	require.Equal(t, "New Name", actions[0].DisplayName)
}

// TestValidateSCIMGroupPatchOp_DisplayNameRemove_Rejected tests Validate SCIM Group Patch Op for Display Name
// Remove Rejected.
func TestValidateSCIMGroupPatchOp_DisplayNameRemove_Rejected(t *testing.T) {
	body := `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
		"Operations": [{"op": "remove", "path": "displayName"}]
	}`
	_, err := parseAndValidateSCIMGroupPatchRequest([]byte(body))
	require.Equal(t, scim.ErrorInvalidPatchPath.Code, err.Code)
}

// TestValidateSCIMGroupPatchOp_DisplayNameEmptyValue_Rejected tests Validate SCIM Group Patch Op for Display
// Name Empty Value Rejected.
func TestValidateSCIMGroupPatchOp_DisplayNameEmptyValue_Rejected(t *testing.T) {
	body := `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
		"Operations": [{"op": "replace", "path": "displayName", "value": ""}]
	}`
	_, err := parseAndValidateSCIMGroupPatchRequest([]byte(body))
	require.Equal(t, scim.ErrorInvalidPatchValue.Code, err.Code)
}

// TestValidateSCIMGroupPatchOp_AddMembers tests Validate SCIM Group Patch Op for Add Members.
func TestValidateSCIMGroupPatchOp_AddMembers(t *testing.T) {
	body := `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
		"Operations": [{"op": "add", "path": "members",
			"value": [{"value": "user-1", "type": "User"}]}]
	}`
	actions, err := parseAndValidateSCIMGroupPatchRequest([]byte(body))
	require.Nil(t, err)
	require.Equal(t, scimGroupPatchTargetMembers, actions[0].Target)
	require.Len(t, actions[0].Members, 1)
}

// TestValidateSCIMGroupPatchOp_AddMembers_EmptyValue_Rejected tests Validate SCIM Group Patch Op for Add
// Members Empty Value Rejected.
func TestValidateSCIMGroupPatchOp_AddMembers_EmptyValue_Rejected(t *testing.T) {
	body := `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
		"Operations": [{"op": "add", "path": "members", "value": []}]
	}`
	_, err := parseAndValidateSCIMGroupPatchRequest([]byte(body))
	require.Equal(t, scim.ErrorInvalidPatchValue.Code, err.Code)
}

// TestValidateSCIMGroupPatchOp_RemoveMembers_NoPath tests Validate SCIM Group Patch Op for Remove Members No Path.
func TestValidateSCIMGroupPatchOp_RemoveMembers_NoPath(t *testing.T) {
	body := `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
		"Operations": [{"op": "remove", "path": "members"}]
	}`
	actions, err := parseAndValidateSCIMGroupPatchRequest([]byte(body))
	require.Nil(t, err)
	require.Empty(t, actions[0].FilterValue)
}

// TestValidateSCIMGroupPatchOp_RemoveMembers_FilteredPath tests Validate SCIM Group Patch Op for Remove
// Members Filtered Path.
func TestValidateSCIMGroupPatchOp_RemoveMembers_FilteredPath(t *testing.T) {
	body := `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
		"Operations": [{"op": "remove", "path": "members[value eq \"user-1\"]"}]
	}`
	actions, err := parseAndValidateSCIMGroupPatchRequest([]byte(body))
	require.Nil(t, err)
	require.Equal(t, "user-1", actions[0].FilterValue)
}

// TestValidateSCIMGroupPatchOp_RemoveMembers_FilteredPathWithValue_Rejected tests Validate SCIM Group Patch
// Op for Remove Members Filtered Path With Value Rejected.
func TestValidateSCIMGroupPatchOp_RemoveMembers_FilteredPathWithValue_Rejected(t *testing.T) {
	body := `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
		"Operations": [{"op": "remove", "path": "members[value eq \"user-1\"]",
			"value": [{"value": "user-1"}]}]
	}`
	_, err := parseAndValidateSCIMGroupPatchRequest([]byte(body))
	require.Equal(t, scim.ErrorInvalidPatchValue.Code, err.Code)
}

// TestValidateSCIMGroupPatchOp_MalformedFilterPath tests Validate SCIM Group Patch Op for Malformed Filter Path.
func TestValidateSCIMGroupPatchOp_MalformedFilterPath(t *testing.T) {
	cases := []string{
		`members[value \"user-1\"]`,   // missing "eq"
		`members[id eq \"user-1\"]`,   // wrong attribute
		`members[value eq ]`,          // empty value
		`members[value eq \"\"]`,      // empty string value
		`members[value eq \"user-1\"`, // unterminated bracket
	}
	for _, path := range cases {
		body := `{
			"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
			"Operations": [{"op": "remove", "path": "` + path + `"}]
		}`
		_, err := parseAndValidateSCIMGroupPatchRequest([]byte(body))
		require.Equal(t, scim.ErrorInvalidPatchPath.Code, err.Code, "path: %s", path)
	}
}

// TestValidateSCIMGroupPatchOp_FilteredPath_AddRejected tests Validate SCIM Group Patch Op for Filtered Path
// Add Rejected.
func TestValidateSCIMGroupPatchOp_FilteredPath_AddRejected(t *testing.T) {
	body := `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
		"Operations": [{"op": "add", "path": "members[value eq \"user-1\"]",
			"value": [{"value": "user-1"}]}]
	}`
	_, err := parseAndValidateSCIMGroupPatchRequest([]byte(body))
	require.Equal(t, scim.ErrorInvalidPatchPath.Code, err.Code)
}

// TestValidateSCIMGroupPatchOp_UnknownPath_Rejected tests Validate SCIM Group Patch Op for Unknown Path Rejected.
func TestValidateSCIMGroupPatchOp_UnknownPath_Rejected(t *testing.T) {
	body := `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
		"Operations": [{"op": "replace", "path": "externalId", "value": "x"}]
	}`
	_, err := parseAndValidateSCIMGroupPatchRequest([]byte(body))
	require.Equal(t, scim.ErrorInvalidPatchPath.Code, err.Code)
}

// TestValidateSCIMGroupPatchOp_InvalidOp_Rejected tests Validate SCIM Group Patch Op for Invalid Op Rejected.
func TestValidateSCIMGroupPatchOp_InvalidOp_Rejected(t *testing.T) {
	body := `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
		"Operations": [{"op": "bogus", "path": "displayName", "value": "x"}]
	}`
	_, err := parseAndValidateSCIMGroupPatchRequest([]byte(body))
	require.Equal(t, scim.ErrorInvalidPatchOp.Code, err.Code)
}

// TestValidateSCIMGroupPatchOp_CaseInsensitiveOpAndPath tests Validate SCIM Group Patch Op for Case
// Insensitive Op And Path.
func TestValidateSCIMGroupPatchOp_CaseInsensitiveOpAndPath(t *testing.T) {
	body := `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
		"Operations": [{"op": "REPLACE", "path": "DisplayName", "value": "X"}]
	}`
	actions, err := parseAndValidateSCIMGroupPatchRequest([]byte(body))
	require.Nil(t, err)
	require.Equal(t, scimGroupPatchTargetDisplayName, actions[0].Target)
}

// TestValidateSCIMGroupPatchOp_RemoveMembersWithUnexpectedValue_Rejected tests Validate SCIM Group Patch Op
// for Remove Members With Unexpected Value Rejected.
func TestValidateSCIMGroupPatchOp_RemoveMembersWithUnexpectedValue_Rejected(t *testing.T) {
	body := `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
		"Operations": [{"op": "remove", "path": "members", "value": [{"value": "user-1"}]}]
	}`
	_, err := parseAndValidateSCIMGroupPatchRequest([]byte(body))
	require.Equal(t, scim.ErrorInvalidPatchValue.Code, err.Code)
}

// TestValidateSCIMGroupPatchOp_AddMembersWithInvalidJSONValue_Rejected tests Validate SCIM Group Patch Op for
// Add Members With Invalid JSON Value Rejected.
func TestValidateSCIMGroupPatchOp_AddMembersWithInvalidJSONValue_Rejected(t *testing.T) {
	body := `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
		"Operations": [{"op": "add", "path": "members", "value": "not-an-array"}]
	}`
	_, err := parseAndValidateSCIMGroupPatchRequest([]byte(body))
	require.Equal(t, scim.ErrorInvalidPatchValue.Code, err.Code)
}
