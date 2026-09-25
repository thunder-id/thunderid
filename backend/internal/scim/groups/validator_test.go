// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package groups

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	scim "github.com/thunder-id/thunderid/internal/scim/common"
)

// ValidatorTestSuite groups the tests in validator_test.go.
type ValidatorTestSuite struct {
	suite.Suite
}

// TestValidatorTestSuite runs ValidatorTestSuite.
func TestValidatorTestSuite(t *testing.T) {
	suite.Run(t, new(ValidatorTestSuite))
}

// ---------------------------------------------------------------------------
// Groups — POST / PUT validation tests
// ---------------------------------------------------------------------------

// TestValidateSCIMGroupWriteRequest_InvalidJSON tests Validate SCIM Group Write Request for Invalid JSON.
func (suite *ValidatorTestSuite) TestValidateSCIMGroupWriteRequest_InvalidJSON() {
	t := suite.T()
	_, err := parseAndValidateSCIMGroupWriteRequest([]byte(`not json`))
	require.Equal(t, scim.ErrorInvalidRequestBody.Code, err.Code)
}

// TestValidateSCIMGroupWriteRequest_MissingDisplayName tests Validate SCIM Group Write Request for Missing
// Display Name.
func (suite *ValidatorTestSuite) TestValidateSCIMGroupWriteRequest_MissingDisplayName() {
	t := suite.T()
	body := `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:Group"],"displayName":""}`
	_, err := parseAndValidateSCIMGroupWriteRequest([]byte(body))
	require.Equal(t, scim.ErrorInvalidRequestBody.Code, err.Code)
}

// TestValidateSCIMGroupWriteRequest_MissingCoreGroupSchema tests Validate SCIM Group Write Request for
// Missing Core Group Schema.
func (suite *ValidatorTestSuite) TestValidateSCIMGroupWriteRequest_MissingCoreGroupSchema() {
	t := suite.T()
	body := `{"schemas":[],"displayName":"Eng"}`
	_, err := parseAndValidateSCIMGroupWriteRequest([]byte(body))
	require.Equal(t, scim.ErrorMissingCoreGroupSchema.Code, err.Code)
}

// TestValidateSCIMGroupWriteRequest_Valid tests Validate SCIM Group Write Request for Valid.
func (suite *ValidatorTestSuite) TestValidateSCIMGroupWriteRequest_Valid() {
	t := suite.T()
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
func (suite *ValidatorTestSuite) TestValidateSCIMGroupWriteRequest_NoMembers() {
	t := suite.T()
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
func (suite *ValidatorTestSuite) TestValidateSCIMGroupPatchRequest_MissingSchema() {
	t := suite.T()
	body := `{"Operations":[{"op":"replace","path":"displayName","value":"X"}]}`
	_, err := parseAndValidateSCIMGroupPatchRequest([]byte(body))
	require.Equal(t, scim.ErrorMissingSchemas.Code, err.Code)
}

// TestValidateSCIMGroupPatchRequest_InvalidJSON tests Validate SCIM Group Patch Request for Invalid JSON.
func (suite *ValidatorTestSuite) TestValidateSCIMGroupPatchRequest_InvalidJSON() {
	t := suite.T()
	_, err := parseAndValidateSCIMGroupPatchRequest([]byte(`not json`))
	require.Equal(t, scim.ErrorInvalidRequestBody.Code, err.Code)
}

// TestValidateSCIMGroupPatchOp_DisplayNameReplace tests Validate SCIM Group Patch Op for Display Name Replace.
func (suite *ValidatorTestSuite) TestValidateSCIMGroupPatchOp_DisplayNameReplace() {
	t := suite.T()
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
func (suite *ValidatorTestSuite) TestValidateSCIMGroupPatchOp_DisplayNameRemove_Rejected() {
	t := suite.T()
	body := `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
		"Operations": [{"op": "remove", "path": "displayName"}]
	}`
	_, err := parseAndValidateSCIMGroupPatchRequest([]byte(body))
	require.Equal(t, scim.ErrorInvalidPatchPath.Code, err.Code)
}

// TestValidateSCIMGroupPatchOp_DisplayNameEmptyValue_Rejected tests Validate SCIM Group Patch Op for Display
// Name Empty Value Rejected.
func (suite *ValidatorTestSuite) TestValidateSCIMGroupPatchOp_DisplayNameEmptyValue_Rejected() {
	t := suite.T()
	body := `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
		"Operations": [{"op": "replace", "path": "displayName", "value": ""}]
	}`
	_, err := parseAndValidateSCIMGroupPatchRequest([]byte(body))
	require.Equal(t, scim.ErrorInvalidPatchValue.Code, err.Code)
}

// TestValidateSCIMGroupPatchOp_AddMembers tests Validate SCIM Group Patch Op for Add Members.
func (suite *ValidatorTestSuite) TestValidateSCIMGroupPatchOp_AddMembers() {
	t := suite.T()
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
func (suite *ValidatorTestSuite) TestValidateSCIMGroupPatchOp_AddMembers_EmptyValue_Rejected() {
	t := suite.T()
	body := `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
		"Operations": [{"op": "add", "path": "members", "value": []}]
	}`
	_, err := parseAndValidateSCIMGroupPatchRequest([]byte(body))
	require.Equal(t, scim.ErrorInvalidPatchValue.Code, err.Code)
}

// TestValidateSCIMGroupPatchOp_RemoveMembers_NoPath tests Validate SCIM Group Patch Op for Remove Members No Path.
func (suite *ValidatorTestSuite) TestValidateSCIMGroupPatchOp_RemoveMembers_NoPath() {
	t := suite.T()
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
func (suite *ValidatorTestSuite) TestValidateSCIMGroupPatchOp_RemoveMembers_FilteredPath() {
	t := suite.T()
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
func (suite *ValidatorTestSuite) TestValidateSCIMGroupPatchOp_RemoveMembers_FilteredPathWithValue_Rejected() {
	t := suite.T()
	body := `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
		"Operations": [{"op": "remove", "path": "members[value eq \"user-1\"]",
			"value": [{"value": "user-1"}]}]
	}`
	_, err := parseAndValidateSCIMGroupPatchRequest([]byte(body))
	require.Equal(t, scim.ErrorInvalidPatchValue.Code, err.Code)
}

// TestValidateSCIMGroupPatchOp_MalformedFilterPath tests Validate SCIM Group Patch Op for Malformed Filter Path.
func (suite *ValidatorTestSuite) TestValidateSCIMGroupPatchOp_MalformedFilterPath() {
	t := suite.T()
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
func (suite *ValidatorTestSuite) TestValidateSCIMGroupPatchOp_FilteredPath_AddRejected() {
	t := suite.T()
	body := `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
		"Operations": [{"op": "add", "path": "members[value eq \"user-1\"]",
			"value": [{"value": "user-1"}]}]
	}`
	_, err := parseAndValidateSCIMGroupPatchRequest([]byte(body))
	require.Equal(t, scim.ErrorInvalidPatchPath.Code, err.Code)
}

// TestValidateSCIMGroupPatchOp_UnknownPath_Rejected tests Validate SCIM Group Patch Op for Unknown Path Rejected.
func (suite *ValidatorTestSuite) TestValidateSCIMGroupPatchOp_UnknownPath_Rejected() {
	t := suite.T()
	body := `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
		"Operations": [{"op": "replace", "path": "externalId", "value": "x"}]
	}`
	_, err := parseAndValidateSCIMGroupPatchRequest([]byte(body))
	require.Equal(t, scim.ErrorInvalidPatchPath.Code, err.Code)
}

// TestValidateSCIMGroupPatchOp_Pathless tests Validate SCIM Group Patch Op for pathless add/replace.
func (suite *ValidatorTestSuite) TestValidateSCIMGroupPatchOp_Pathless() {
	t := suite.T()
	wrap := func(op, value string) []byte {
		return []byte(`{
			"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
			"Operations": [{"op": "` + op + `", "value": ` + value + `}]
		}`)
	}

	t.Run("displayName replace", func(t *testing.T) {
		actions, err := parseAndValidateSCIMGroupPatchRequest(wrap("replace", `{"displayName": "New Name"}`))
		require.Nil(t, err)
		require.Len(t, actions, 1)
		require.Equal(t, scimGroupPatchTargetDisplayName, actions[0].Target)
		require.Equal(t, "New Name", actions[0].DisplayName)
	})

	t.Run("displayName and members, case-insensitive keys", func(t *testing.T) {
		actions, err := parseAndValidateSCIMGroupPatchRequest(
			wrap("add", `{"DisplayName": "N", "members": [{"value": "user-1", "type": "User"}]}`))
		require.Nil(t, err)
		require.Len(t, actions, 2)
		require.Equal(t, scimGroupPatchTargetDisplayName, actions[0].Target)
		require.Equal(t, scimGroupPatchTargetMembers, actions[1].Target)
		require.Len(t, actions[1].Members, 1)
	})

	t.Run("unknown attribute", func(t *testing.T) {
		_, err := parseAndValidateSCIMGroupPatchRequest(wrap("replace", `{"externalId": "x"}`))
		require.Equal(t, scim.ErrorInvalidPatchPath.Code, err.Code)
	})

	t.Run("invalid nested value", func(t *testing.T) {
		_, err := parseAndValidateSCIMGroupPatchRequest(wrap("replace", `{"displayName": ""}`))
		require.Equal(t, scim.ErrorInvalidPatchValue.Code, err.Code)
	})

	t.Run("non-object or empty value", func(t *testing.T) {
		for _, value := range []string{`"x"`, `{}`, `null`} {
			_, err := parseAndValidateSCIMGroupPatchRequest(wrap("replace", value))
			require.Equal(t, scim.ErrorInvalidPatchValue.Code, err.Code, "value: %s", value)
		}
	})

	t.Run("missing value", func(t *testing.T) {
		body := `{
			"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
			"Operations": [{"op": "add"}]
		}`
		_, err := parseAndValidateSCIMGroupPatchRequest([]byte(body))
		require.Equal(t, scim.ErrorInvalidPatchValue.Code, err.Code)
	})

	t.Run("remove has no target", func(t *testing.T) {
		body := `{
			"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
			"Operations": [{"op": "remove"}]
		}`
		_, err := parseAndValidateSCIMGroupPatchRequest([]byte(body))
		require.Equal(t, scim.ErrorNoTarget.Code, err.Code)
	})
}

// TestValidateSCIMGroupPatchOp_InvalidOp_Rejected tests Validate SCIM Group Patch Op for Invalid Op Rejected.
func (suite *ValidatorTestSuite) TestValidateSCIMGroupPatchOp_InvalidOp_Rejected() {
	t := suite.T()
	body := `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
		"Operations": [{"op": "bogus", "path": "displayName", "value": "x"}]
	}`
	_, err := parseAndValidateSCIMGroupPatchRequest([]byte(body))
	require.Equal(t, scim.ErrorInvalidPatchOp.Code, err.Code)
}

// TestValidateSCIMGroupPatchOp_CaseInsensitiveOpAndPath tests Validate SCIM Group Patch Op for Case
// Insensitive Op And Path.
func (suite *ValidatorTestSuite) TestValidateSCIMGroupPatchOp_CaseInsensitiveOpAndPath() {
	t := suite.T()
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
func (suite *ValidatorTestSuite) TestValidateSCIMGroupPatchOp_RemoveMembersWithUnexpectedValue_Rejected() {
	t := suite.T()
	body := `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
		"Operations": [{"op": "remove", "path": "members", "value": [{"value": "user-1"}]}]
	}`
	_, err := parseAndValidateSCIMGroupPatchRequest([]byte(body))
	require.Equal(t, scim.ErrorInvalidPatchValue.Code, err.Code)
}

// TestValidateSCIMGroupPatchOp_AddMembersWithInvalidJSONValue_Rejected tests Validate SCIM Group Patch Op for
// Add Members With Invalid JSON Value Rejected.
func (suite *ValidatorTestSuite) TestValidateSCIMGroupPatchOp_AddMembersWithInvalidJSONValue_Rejected() {
	t := suite.T()
	body := `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
		"Operations": [{"op": "add", "path": "members", "value": "not-an-array"}]
	}`
	_, err := parseAndValidateSCIMGroupPatchRequest([]byte(body))
	require.Equal(t, scim.ErrorInvalidPatchValue.Code, err.Code)
}
