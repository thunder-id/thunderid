package scim

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateSCIMGroupPatchRequest_MissingSchema(t *testing.T) {
	body := `{"Operations":[{"op":"replace","path":"displayName","value":"X"}]}`
	_, err := ValidateSCIMGroupPatchRequest([]byte(body))
	require.Equal(t, ErrorMissingSchemas.Code, err.Code)
}

func TestValidateSCIMGroupPatchRequest_InvalidJSON(t *testing.T) {
	_, err := ValidateSCIMGroupPatchRequest([]byte(`not json`))
	require.Equal(t, ErrorInvalidRequestBody.Code, err.Code)
}

func TestValidateSCIMGroupPatchOp_DisplayNameReplace(t *testing.T) {
	body := `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
		"Operations": [{"op": "replace", "path": "displayName", "value": "New Name"}]
	}`
	actions, err := ValidateSCIMGroupPatchRequest([]byte(body))
	require.Nil(t, err)
	require.Len(t, actions, 1)
	require.Equal(t, scimGroupPatchTargetDisplayName, actions[0].Target)
	require.Equal(t, "New Name", actions[0].DisplayName)
}

func TestValidateSCIMGroupPatchOp_DisplayNameRemove_Rejected(t *testing.T) {
	body := `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
		"Operations": [{"op": "remove", "path": "displayName"}]
	}`
	_, err := ValidateSCIMGroupPatchRequest([]byte(body))
	require.Equal(t, ErrorInvalidPatchPath.Code, err.Code)
}

func TestValidateSCIMGroupPatchOp_DisplayNameEmptyValue_Rejected(t *testing.T) {
	body := `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
		"Operations": [{"op": "replace", "path": "displayName", "value": ""}]
	}`
	_, err := ValidateSCIMGroupPatchRequest([]byte(body))
	require.Equal(t, ErrorInvalidPatchValue.Code, err.Code)
}

func TestValidateSCIMGroupPatchOp_AddMembers(t *testing.T) {
	body := `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
		"Operations": [{"op": "add", "path": "members",
			"value": [{"value": "user-1", "type": "User"}]}]
	}`
	actions, err := ValidateSCIMGroupPatchRequest([]byte(body))
	require.Nil(t, err)
	require.Equal(t, scimGroupPatchTargetMembers, actions[0].Target)
	require.Len(t, actions[0].Members, 1)
}

func TestValidateSCIMGroupPatchOp_AddMembers_EmptyValue_Rejected(t *testing.T) {
	body := `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
		"Operations": [{"op": "add", "path": "members", "value": []}]
	}`
	_, err := ValidateSCIMGroupPatchRequest([]byte(body))
	require.Equal(t, ErrorInvalidPatchValue.Code, err.Code)
}

func TestValidateSCIMGroupPatchOp_RemoveMembers_NoPath(t *testing.T) {
	body := `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
		"Operations": [{"op": "remove", "path": "members"}]
	}`
	actions, err := ValidateSCIMGroupPatchRequest([]byte(body))
	require.Nil(t, err)
	require.Empty(t, actions[0].FilterValue)
}

func TestValidateSCIMGroupPatchOp_RemoveMembers_FilteredPath(t *testing.T) {
	body := `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
		"Operations": [{"op": "remove", "path": "members[value eq \"user-1\"]"}]
	}`
	actions, err := ValidateSCIMGroupPatchRequest([]byte(body))
	require.Nil(t, err)
	require.Equal(t, "user-1", actions[0].FilterValue)
}

func TestValidateSCIMGroupPatchOp_RemoveMembers_FilteredPathWithValue_Rejected(t *testing.T) {
	body := `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
		"Operations": [{"op": "remove", "path": "members[value eq \"user-1\"]",
			"value": [{"value": "user-1"}]}]
	}`
	_, err := ValidateSCIMGroupPatchRequest([]byte(body))
	require.Equal(t, ErrorInvalidPatchValue.Code, err.Code)
}

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
		_, err := ValidateSCIMGroupPatchRequest([]byte(body))
		require.Equal(t, ErrorInvalidPatchPath.Code, err.Code, "path: %s", path)
	}
}

func TestValidateSCIMGroupPatchOp_FilteredPath_AddRejected(t *testing.T) {
	body := `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
		"Operations": [{"op": "add", "path": "members[value eq \"user-1\"]",
			"value": [{"value": "user-1"}]}]
	}`
	_, err := ValidateSCIMGroupPatchRequest([]byte(body))
	require.Equal(t, ErrorInvalidPatchPath.Code, err.Code)
}

func TestValidateSCIMGroupPatchOp_UnknownPath_Rejected(t *testing.T) {
	body := `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
		"Operations": [{"op": "replace", "path": "externalId", "value": "x"}]
	}`
	_, err := ValidateSCIMGroupPatchRequest([]byte(body))
	require.Equal(t, ErrorInvalidPatchPath.Code, err.Code)
}

func TestValidateSCIMGroupPatchOp_InvalidOp_Rejected(t *testing.T) {
	body := `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
		"Operations": [{"op": "bogus", "path": "displayName", "value": "x"}]
	}`
	_, err := ValidateSCIMGroupPatchRequest([]byte(body))
	require.Equal(t, ErrorInvalidPatchOp.Code, err.Code)
}

func TestValidateSCIMGroupPatchOp_CaseInsensitiveOpAndPath(t *testing.T) {
	body := `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
		"Operations": [{"op": "REPLACE", "path": "DisplayName", "value": "X"}]
	}`
	actions, err := ValidateSCIMGroupPatchRequest([]byte(body))
	require.Nil(t, err)
	require.Equal(t, scimGroupPatchTargetDisplayName, actions[0].Target)
}

func TestValidateSCIMGroupPatchOp_RemoveMembersWithUnexpectedValue_Rejected(t *testing.T) {
	body := `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
		"Operations": [{"op": "remove", "path": "members", "value": [{"value": "user-1"}]}]
	}`
	_, err := ValidateSCIMGroupPatchRequest([]byte(body))
	require.Equal(t, ErrorInvalidPatchValue.Code, err.Code)
}

func TestValidateSCIMGroupPatchOp_AddMembersWithInvalidJSONValue_Rejected(t *testing.T) {
	body := `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
		"Operations": [{"op": "add", "path": "members", "value": "not-an-array"}]
	}`
	_, err := ValidateSCIMGroupPatchRequest([]byte(body))
	require.Equal(t, ErrorInvalidPatchValue.Code, err.Code)
}
