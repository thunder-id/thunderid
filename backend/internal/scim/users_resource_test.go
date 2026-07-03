package scim

import (
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
			result := stripCredentialFields(tc.attributes, tc.credKeys)
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

	scimUser := buildSCIMUserResource(u, extensionURN, baseURL, credKeys)

	require.Equal(t, "user123", scimUser.ID)
	require.Contains(t, scimUser.Schemas, SCIMCoreUserSchemaURN)
	require.Contains(t, scimUser.Schemas, extensionURN)
	require.JSONEq(t, `{"name":"John"}`, string(scimUser.Attributes))
}
