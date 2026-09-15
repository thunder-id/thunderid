// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package users

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	scim "github.com/thunder-id/thunderid/internal/scim/common"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const testAPIBaseURL = "https://api.example.com"

const testPersonExtensionURN = "urn:thunderid:params:scim:schemas:person:2.0:User"

// TestFilterAttrsBySchema tests Filter Attrs By Schema.
func TestFilterAttrsBySchema(t *testing.T) {
	testCases := []struct {
		name       string
		attributes json.RawMessage
		rawProps   map[string]scim.RawPropertyDef
		expected   json.RawMessage
	}{
		{
			name:       "Strips single credential",
			attributes: json.RawMessage(`{"name":"Alice","password":"secret"}`),
			rawProps:   map[string]scim.RawPropertyDef{"password": {Credential: true}},
			expected:   json.RawMessage(`{"name":"Alice"}`),
		},
		{
			name:       "Strips multiple credentials",
			attributes: json.RawMessage(`{"name":"Bob","password":"sec","pin":"123"}`),
			rawProps:   map[string]scim.RawPropertyDef{"password": {Credential: true}, "pin": {Credential: true}},
			expected:   json.RawMessage(`{"name":"Bob"}`),
		},
		{
			name:       "Case-insensitive sweep",
			attributes: json.RawMessage(`{"name":"Bob","PassWord":"sec","PIN":"123"}`),
			rawProps:   map[string]scim.RawPropertyDef{"password": {Credential: true}, "pin": {Credential: true}},
			expected:   json.RawMessage(`{"name":"Bob"}`),
		},
		{
			name:       "Non-credential schema property is retained",
			attributes: json.RawMessage(`{"name":"Charlie","password":"sec"}`),
			rawProps:   map[string]scim.RawPropertyDef{"name": {}, "password": {Credential: true}},
			expected:   json.RawMessage(`{"name":"Charlie"}`),
		},
		{
			name:       "Unknown key not present in schema is retained",
			attributes: json.RawMessage(`{"name":"Eve","unmapped":"kept"}`),
			rawProps:   map[string]scim.RawPropertyDef{"password": {Credential: true}},
			expected:   json.RawMessage(`{"name":"Eve","unmapped":"kept"}`),
		},
		{
			name:       "No credentials present",
			attributes: json.RawMessage(`{"name":"Charlie"}`),
			rawProps:   map[string]scim.RawPropertyDef{"password": {Credential: true}},
			expected:   json.RawMessage(`{"name":"Charlie"}`),
		},
		{
			name:       "Empty schema props",
			attributes: json.RawMessage(`{"name":"Dave","password":"sec"}`),
			rawProps:   map[string]scim.RawPropertyDef{},
			expected:   json.RawMessage(`{"name":"Dave","password":"sec"}`),
		},
		{
			name:       "Empty attributes",
			attributes: json.RawMessage(``),
			rawProps:   map[string]scim.RawPropertyDef{"password": {Credential: true}},
			expected:   json.RawMessage(`{}`), // Fails closed
		},
		{
			name:       "Invalid JSON",
			attributes: json.RawMessage(`{invalid`),
			rawProps:   map[string]scim.RawPropertyDef{"password": {Credential: true}},
			expected:   json.RawMessage(`{}`), // Fails closed
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := filterAttrsBySchema(context.Background(), *log.GetLogger(), tc.attributes, tc.rawProps)
			require.JSONEq(t, string(tc.expected), string(result))
		})
	}
}

// TestBuildSCIMUserResource tests Build SCIM User Resource.
func TestBuildSCIMUserResource(t *testing.T) {
	u := providers.User{
		ID:         "user123",
		Type:       "Person",
		Attributes: json.RawMessage(`{"name":"John","password":"pwd"}`),
	}
	baseURL := testAPIBaseURL
	extensionURN := testPersonExtensionURN
	rawProps := map[string]scim.RawPropertyDef{"password": {Credential: true}}

	scimUser := buildSCIMUserResource(context.Background(), *log.GetLogger(), u, extensionURN, baseURL, rawProps, true)

	require.Equal(t, "user123", scimUser.ID)
	require.Contains(t, scimUser.Schemas, scim.SCIMCoreUserSchemaURN)
	require.Contains(t, scimUser.Schemas, extensionURN)
	require.JSONEq(t, `{"name":"John"}`, string(scimUser.Attributes))
}

// TestBuildSCIMUserResource_IncludeCoreAttrsFalse_OmitsCoreAttrs tests Build SCIM User Resource for Include
// Core Attrs False Omits Core Attrs.
func TestBuildSCIMUserResource_IncludeCoreAttrsFalse_OmitsCoreAttrs(t *testing.T) {
	u := providers.User{
		ID:         "user123",
		Type:       "Person",
		Attributes: json.RawMessage(`{"username":"jdoe"}`),
	}
	baseURL := testAPIBaseURL
	extensionURN := testPersonExtensionURN

	scimUser := buildSCIMUserResource(context.Background(), *log.GetLogger(), u, extensionURN, baseURL, nil, false)

	require.Nil(t, scimUser.CoreAttrs)
}

// TestBuildSCIMUserListResponse_NilUsers tests Build SCIM User List Response for Nil Users.
func TestBuildSCIMUserListResponse_NilUsers(t *testing.T) {
	resp := buildSCIMUserListResponse(nil, 5, 1, 0)
	require.Equal(t, []string{scim.SCIMListResponseSchemaURN}, resp.Schemas)
	require.Equal(t, 5, resp.TotalResults)
	require.Equal(t, 1, resp.StartIndex)
	require.Equal(t, 0, resp.ItemsPerPage)
	require.NotNil(t, resp.Resources)
	require.Empty(t, resp.Resources)
}

// TestSCIMUser_MarshalJSON tests SCIM User for Marshal JSON.
func TestSCIMUser_MarshalJSON(t *testing.T) {
	// 1. Without extension attributes or extension URN
	u1 := SCIMUser{
		ID:      "user-1",
		Schemas: []string{scim.SCIMCoreUserSchemaURN},
		Meta: scim.SCIMMeta{
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
		Schemas:      []string{scim.SCIMCoreUserSchemaURN, "urn:thunderid:params:scim:schemas:employee:2.0:User"},
		ExtensionURN: "urn:thunderid:params:scim:schemas:employee:2.0:User",
		Attributes:   json.RawMessage(`{"department":"Engineering"}`),
		Meta: scim.SCIMMeta{
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

	// 3. With enterprise attributes
	u3 := SCIMUser{
		ID:              "user-3",
		Schemas:         []string{scim.SCIMCoreUserSchemaURN, scim.SCIMEnterpriseUserSchemaURN},
		EnterpriseAttrs: json.RawMessage(`{"department":"Engineering","employeeNumber":"123"}`),
		Meta: scim.SCIMMeta{
			ResourceType: "User",
			Location:     "https://api.example.com/scim/v2/Users/user-3",
		},
	}
	b3, err := u3.MarshalJSON()
	require.NoError(t, err)
	var map3 map[string]interface{}
	require.NoError(t, json.Unmarshal(b3, &map3))
	require.Equal(t, "user-3", map3["id"])
	ent3 := map3[scim.SCIMEnterpriseUserSchemaURN].(map[string]interface{})
	require.Equal(t, "Engineering", ent3["department"])
	require.Equal(t, "123", ent3["employeeNumber"])
}

// TestBuildSCIMUserResource_EnterpriseAttrs tests BuildSCIMUserResource with enterprise attributes.
func TestBuildSCIMUserResource_EnterpriseAttrs(t *testing.T) {
	u := providers.User{
		ID:         "user123",
		Type:       "Person",
		Attributes: json.RawMessage(`{"username":"jdoe","department":"Engineering","employee_number":"123"}`),
	}
	baseURL := testAPIBaseURL
	extensionURN := testPersonExtensionURN

	scimUser := buildSCIMUserResource(context.Background(), *log.GetLogger(), u, extensionURN, baseURL, nil, true)

	require.Contains(t, scimUser.Schemas, scim.SCIMEnterpriseUserSchemaURN)
	require.NotNil(t, scimUser.EnterpriseAttrs)
	var entMap map[string]interface{}
	require.NoError(t, json.Unmarshal(scimUser.EnterpriseAttrs, &entMap))
	require.Equal(t, "Engineering", entMap["department"])
	require.Equal(t, "123", entMap["employeeNumber"])
}

// TestProjectSCIMAttributes tests Project SCIM Attributes.
func TestProjectSCIMAttributes(t *testing.T) {
	const extURN = "urn:thunderid:params:scim:schemas:employee:2.0:User"
	resource := map[string]interface{}{
		"id":       "u1",
		"schemas":  []interface{}{scim.SCIMCoreUserSchemaURN, extURN},
		"meta":     map[string]interface{}{"resourceType": "User"},
		"userName": "alice",
		"emails":   []interface{}{"alice@example.com"},
		"active":   true,
		extURN: map[string]interface{}{
			"given_name": "Alice",
			"department": "Engineering",
		},
	}

	t.Run("attributes keeps requested plus always-returned", func(t *testing.T) {
		got := projectSCIMAttributes(resource, extURN, []string{"userName"}, nil)
		require.Equal(t, map[string]interface{}{
			"id":       "u1",
			"schemas":  []interface{}{scim.SCIMCoreUserSchemaURN, extURN},
			"meta":     map[string]interface{}{"resourceType": "User"},
			"userName": "alice",
		}, got)
	})

	t.Run("excludedAttributes drops requested but keeps always-returned", func(t *testing.T) {
		got := projectSCIMAttributes(resource, extURN, nil, []string{"emails", "active"})
		require.Equal(t, map[string]interface{}{
			"id":       "u1",
			"schemas":  []interface{}{scim.SCIMCoreUserSchemaURN, extURN},
			"meta":     map[string]interface{}{"resourceType": "User"},
			"userName": "alice",
			extURN: map[string]interface{}{
				"given_name": "Alice",
				"department": "Engineering",
			},
		}, got)
	})

	t.Run("attributes takes precedence when both supplied", func(t *testing.T) {
		got := projectSCIMAttributes(resource, extURN, []string{"userName"}, []string{"userName"})
		require.Equal(t, map[string]interface{}{
			"id":       "u1",
			"schemas":  []interface{}{scim.SCIMCoreUserSchemaURN, extURN},
			"meta":     map[string]interface{}{"resourceType": "User"},
			"userName": "alice",
		}, got)
	})

	t.Run("neither supplied is a pass-through", func(t *testing.T) {
		got := projectSCIMAttributes(resource, extURN, nil, nil)
		require.Equal(t, resource, got)
	})

	t.Run("bare name never matches inside the extension object", func(t *testing.T) {
		// A bare attribute name resolves against the core schema only (RFC 7643 §3.10
		// default schema). "given_name" is a custom/extension attribute, so it must not
		// match the extension object just because it happens to share a name with a
		// stored ThunderID attribute key.
		got := projectSCIMAttributes(resource, extURN, []string{"given_name"}, nil)
		require.Equal(t, map[string]interface{}{
			"id":      "u1",
			"schemas": []interface{}{scim.SCIMCoreUserSchemaURN, extURN},
			"meta":    map[string]interface{}{"resourceType": "User"},
		}, got)
	})

	t.Run("bare core attribute combines with a URN-qualified extension attribute", func(t *testing.T) {
		got := projectSCIMAttributes(resource, extURN, []string{"userName", extURN + ":given_name"}, nil)
		require.Equal(t, map[string]interface{}{
			"id":       "u1",
			"schemas":  []interface{}{scim.SCIMCoreUserSchemaURN, extURN},
			"meta":     map[string]interface{}{"resourceType": "User"},
			"userName": "alice",
			extURN: map[string]interface{}{
				"given_name": "Alice",
			},
		}, got)
	})

	t.Run("URN-qualified extension path selects only that key", func(t *testing.T) {
		got := projectSCIMAttributes(resource, extURN, []string{extURN + ":department"}, nil)
		require.Equal(t, map[string]interface{}{
			"id":      "u1",
			"schemas": []interface{}{scim.SCIMCoreUserSchemaURN, extURN},
			"meta":    map[string]interface{}{"resourceType": "User"},
			extURN: map[string]interface{}{
				"department": "Engineering",
			},
		}, got)
	})

	t.Run("URN-qualified core path selects only that top-level key", func(t *testing.T) {
		got := projectSCIMAttributes(resource, extURN, []string{scim.SCIMCoreUserSchemaURN + ":userName"}, nil)
		require.Equal(t, map[string]interface{}{
			"id":       "u1",
			"schemas":  []interface{}{scim.SCIMCoreUserSchemaURN, extURN},
			"meta":     map[string]interface{}{"resourceType": "User"},
			"userName": "alice",
		}, got)
	})

	t.Run("whole core URN path selects only core top-level keys", func(t *testing.T) {
		got := projectSCIMAttributes(resource, extURN, []string{scim.SCIMCoreUserSchemaURN}, nil)
		require.Equal(t, map[string]interface{}{
			"id":       "u1",
			"schemas":  []interface{}{scim.SCIMCoreUserSchemaURN, extURN},
			"meta":     map[string]interface{}{"resourceType": "User"},
			"userName": "alice",
			"emails":   []interface{}{"alice@example.com"},
			"active":   true,
		}, got)
	})

	t.Run("excludedAttributes whole core URN path drops all core top-level keys", func(t *testing.T) {
		got := projectSCIMAttributes(resource, extURN, nil, []string{scim.SCIMCoreUserSchemaURN})
		require.Equal(t, map[string]interface{}{
			"id":      "u1",
			"schemas": []interface{}{scim.SCIMCoreUserSchemaURN, extURN},
			"meta":    map[string]interface{}{"resourceType": "User"},
			extURN: map[string]interface{}{
				"given_name": "Alice",
				"department": "Engineering",
			},
		}, got)
	})

	t.Run("excludedAttributes bare name does not reach into the extension object", func(t *testing.T) {
		// Bare names in excludedAttributes resolve against the core schema only, same as
		// in attributes; "given_name" must not exclude anything from the extension object.
		got := projectSCIMAttributes(resource, extURN, nil, []string{"given_name"})
		require.Equal(t, map[string]interface{}{
			"id":       "u1",
			"schemas":  []interface{}{scim.SCIMCoreUserSchemaURN, extURN},
			"meta":     map[string]interface{}{"resourceType": "User"},
			"userName": "alice",
			"emails":   []interface{}{"alice@example.com"},
			"active":   true,
			extURN: map[string]interface{}{
				"given_name": "Alice",
				"department": "Engineering",
			},
		}, got)
	})

	t.Run("excludedAttributes URN-qualified extension keys drop the extension object entirely", func(t *testing.T) {
		got := projectSCIMAttributes(resource, extURN, nil, []string{extURN + ":given_name", extURN + ":department"})
		require.Equal(t, map[string]interface{}{
			"id":       "u1",
			"schemas":  []interface{}{scim.SCIMCoreUserSchemaURN, extURN},
			"meta":     map[string]interface{}{"resourceType": "User"},
			"userName": "alice",
			"emails":   []interface{}{"alice@example.com"},
			"active":   true,
		}, got)
	})

	t.Run("unresolvable path is silently omitted, not an error", func(t *testing.T) {
		got := projectSCIMAttributes(resource, extURN, []string{"no.such.path"}, nil)
		require.Equal(t, map[string]interface{}{
			"id":      "u1",
			"schemas": []interface{}{scim.SCIMCoreUserSchemaURN, extURN},
			"meta":    map[string]interface{}{"resourceType": "User"},
		}, got)
	})

	t.Run("URN-qualified enterprise path selects only that key", func(t *testing.T) {
		resWithEnt := map[string]interface{}{
			"id":      "u1",
			"schemas": []interface{}{scim.SCIMCoreUserSchemaURN, scim.SCIMEnterpriseUserSchemaURN},
			"meta":    map[string]interface{}{"resourceType": "User"},
			scim.SCIMEnterpriseUserSchemaURN: map[string]interface{}{
				"department":     "Engineering",
				"employeeNumber": "123",
			},
		}
		got := projectSCIMAttributes(resWithEnt, "", []string{scim.SCIMEnterpriseUserSchemaURN + ":department"}, nil)
		require.Equal(t, map[string]interface{}{
			"id":      "u1",
			"schemas": []interface{}{scim.SCIMCoreUserSchemaURN, scim.SCIMEnterpriseUserSchemaURN},
			"meta":    map[string]interface{}{"resourceType": "User"},
			scim.SCIMEnterpriseUserSchemaURN: map[string]interface{}{
				"department": "Engineering",
			},
		}, got)
	})

	t.Run("whole enterprise URN path selects full enterprise object", func(t *testing.T) {
		resWithEnt := map[string]interface{}{
			"id":      "u1",
			"schemas": []interface{}{scim.SCIMCoreUserSchemaURN, scim.SCIMEnterpriseUserSchemaURN},
			"meta":    map[string]interface{}{"resourceType": "User"},
			scim.SCIMEnterpriseUserSchemaURN: map[string]interface{}{
				"department":     "Engineering",
				"employeeNumber": "123",
			},
		}
		got := projectSCIMAttributes(resWithEnt, "", []string{scim.SCIMEnterpriseUserSchemaURN}, nil)
		require.Equal(t, map[string]interface{}{
			"id":      "u1",
			"schemas": []interface{}{scim.SCIMCoreUserSchemaURN, scim.SCIMEnterpriseUserSchemaURN},
			"meta":    map[string]interface{}{"resourceType": "User"},
			scim.SCIMEnterpriseUserSchemaURN: map[string]interface{}{
				"department":     "Engineering",
				"employeeNumber": "123",
			},
		}, got)
	})
}

// TestProjectSCIMUserListResponse tests Project SCIM User List Response.
func TestProjectSCIMUserListResponse(t *testing.T) {
	t.Run("no projection requested returns nil", func(t *testing.T) {
		got, err := projectSCIMUserListResponse(SCIMUserListResponse{}, nil, nil)
		require.NoError(t, err)
		require.Nil(t, got)
	})

	t.Run("projects each resource", func(t *testing.T) {
		listResp := SCIMUserListResponse{
			Schemas:      []string{scim.SCIMListResponseSchemaURN},
			TotalResults: 1,
			StartIndex:   1,
			ItemsPerPage: 1,
			Resources: []SCIMUser{
				{
					ID:      "u1",
					Schemas: []string{scim.SCIMCoreUserSchemaURN},
					Meta:    scim.SCIMMeta{ResourceType: "User"},
					CoreAttrs: map[string]json.RawMessage{
						"userName": json.RawMessage(`"alice"`),
						"active":   json.RawMessage(`true`),
					},
				},
			},
		}

		got, err := projectSCIMUserListResponse(listResp, []string{"userName"}, nil)
		require.NoError(t, err)
		require.NotNil(t, got)
		require.EqualValues(t, 1, got["totalResults"])

		resources, ok := got["Resources"].([]map[string]interface{})
		require.True(t, ok)
		require.Len(t, resources, 1)
		require.Equal(t, "u1", resources[0]["id"])
		require.Equal(t, "alice", resources[0]["userName"])
		require.NotContains(t, resources[0], "active")
	})
}

// TestProjectSCIMUserResource tests Project SCIM User Resource.
func TestProjectSCIMUserResource(t *testing.T) {
	const extURN = "urn:thunderid:params:scim:schemas:employee:2.0:User"
	u := SCIMUser{
		ID:           "u1",
		Schemas:      []string{scim.SCIMCoreUserSchemaURN, extURN},
		ExtensionURN: extURN,
		Attributes:   json.RawMessage(`{"department":"Engineering"}`),
		Meta:         scim.SCIMMeta{ResourceType: "User"},
	}

	t.Run("no projection requested returns nil", func(t *testing.T) {
		got, err := projectSCIMUserResource(u, nil, nil)
		require.NoError(t, err)
		require.Nil(t, got)
	})

	t.Run("projects the resource using its own extension URN", func(t *testing.T) {
		got, err := projectSCIMUserResource(u, []string{extURN + ":department"}, nil)
		require.NoError(t, err)
		require.NotNil(t, got)
		require.Equal(t, "u1", got["id"])

		ext, ok := got[extURN].(map[string]interface{})
		require.True(t, ok)
		require.Equal(t, "Engineering", ext["department"])
	})
}
