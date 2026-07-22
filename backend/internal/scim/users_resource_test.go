package scim

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/user"
)

func TestStripCredentialFields(t *testing.T) {
	testCases := []struct {
		name       string
		attributes json.RawMessage
		credKeys   map[string]struct{}
		expected   json.RawMessage
	}{
		{
			name:       "Strips single credential",
			attributes: json.RawMessage(`{"name":"Alice","password":"secret"}`),
			credKeys:   map[string]struct{}{"password": {}},
			expected:   json.RawMessage(`{"name":"Alice"}`),
		},
		{
			name:       "Strips multiple credentials",
			attributes: json.RawMessage(`{"name":"Bob","password":"sec","pin":"123"}`),
			credKeys:   map[string]struct{}{"password": {}, "pin": {}},
			expected:   json.RawMessage(`{"name":"Bob"}`),
		},
		{
			name:       "Case-insensitive sweep",
			attributes: json.RawMessage(`{"name":"Bob","PassWord":"sec","PIN":"123"}`),
			credKeys:   map[string]struct{}{"password": {}, "pin": {}},
			expected:   json.RawMessage(`{"name":"Bob"}`),
		},
		{
			name:       "No credentials present",
			attributes: json.RawMessage(`{"name":"Charlie"}`),
			credKeys:   map[string]struct{}{"password": {}},
			expected:   json.RawMessage(`{"name":"Charlie"}`),
		},
		{
			name:       "Empty credentials keys list",
			attributes: json.RawMessage(`{"name":"Dave","password":"sec"}`),
			credKeys:   map[string]struct{}{},
			expected:   json.RawMessage(`{"name":"Dave","password":"sec"}`),
		},
		{
			name:       "Empty attributes",
			attributes: json.RawMessage(``),
			credKeys:   map[string]struct{}{"password": {}},
			expected:   json.RawMessage(`{}`), // Fails closed
		},
		{
			name:       "Invalid JSON",
			attributes: json.RawMessage(`{invalid`),
			credKeys:   map[string]struct{}{"password": {}},
			expected:   json.RawMessage(`{}`), // Fails closed
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := stripCredentialFields(context.Background(), tc.attributes, tc.credKeys)
			require.JSONEq(t, string(tc.expected), string(result))
		})
	}
}

func TestBuildSCIMUserResource(t *testing.T) {
	u := user.User{
		ID:         "user123",
		Type:       "Person",
		Attributes: json.RawMessage(`{"name":"John","password":"pwd"}`),
	}
	baseURL := "https://api.example.com"
	extensionURN := "urn:thunderid:params:scim:schemas:person:2.0:User"
	credKeys := map[string]struct{}{"password": {}}

	scimUser := buildSCIMUserResource(context.Background(), u, extensionURN, baseURL, credKeys)

	require.Equal(t, "user123", scimUser.ID)
	require.Contains(t, scimUser.Schemas, SCIMCoreUserSchemaURN)
	require.Contains(t, scimUser.Schemas, extensionURN)
	require.JSONEq(t, `{"name":"John"}`, string(scimUser.Attributes))
}

func TestBuildSCIMUserListResponse_NilUsers(t *testing.T) {
	resp := buildSCIMUserListResponse(nil, 5, 1, 0)
	require.Equal(t, []string{SCIMListResponseSchemaURN}, resp.Schemas)
	require.Equal(t, 5, resp.TotalResults)
	require.Equal(t, 1, resp.StartIndex)
	require.Equal(t, 0, resp.ItemsPerPage)
	require.NotNil(t, resp.Resources)
	require.Empty(t, resp.Resources)
}

func TestSCIMUser_MarshalJSON(t *testing.T) {
	// 1. Without extension attributes or extension URN
	u1 := SCIMUser{
		ID:      "user-1",
		Schemas: []string{SCIMCoreUserSchemaURN},
		Meta: SCIMMeta{
			ResourceType: "User",
			Location:     "https://api.example.com/scim/v2/Users/user-1",
		},
	}
	b1, err := u1.MarshalJSON()
	require.NoError(t, err)
	var map1 map[string]interface{}
	require.NoError(t, json.Unmarshal(b1, &map1))
	require.Equal(t, "user-1", map1["id"])
	require.Nil(t, map1["urn:thunderid:params:scim:schemas:employee:2.0:User"])

	// 2. With extension attributes and extension URN
	u2 := SCIMUser{
		ID:           "user-2",
		Schemas:      []string{SCIMCoreUserSchemaURN, "urn:thunderid:params:scim:schemas:employee:2.0:User"},
		ExtensionURN: "urn:thunderid:params:scim:schemas:employee:2.0:User",
		Attributes:   json.RawMessage(`{"department":"Engineering"}`),
		Meta: SCIMMeta{
			ResourceType: "User",
			Location:     "https://api.example.com/scim/v2/Users/user-2",
		},
	}
	b2, err := u2.MarshalJSON()
	require.NoError(t, err)
	var map2 map[string]interface{}
	require.NoError(t, json.Unmarshal(b2, &map2))
	require.Equal(t, "user-2", map2["id"])
	require.NotNil(t, map2["urn:thunderid:params:scim:schemas:employee:2.0:User"])

	ext := map2["urn:thunderid:params:scim:schemas:employee:2.0:User"].(map[string]interface{})
	require.Equal(t, "Engineering", ext["department"])
}
