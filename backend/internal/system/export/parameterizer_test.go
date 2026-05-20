/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package export

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/internal/resource"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
)

// Helper to convert local resourceRules to declarativeresource.ResourceRules for testing
func toDeclarativeResourceRules(r *resourceRules) *declarativeresource.ResourceRules {
	if r == nil {
		return nil
	}
	return &declarativeresource.ResourceRules{
		Variables:             r.Variables,
		ArrayVariables:        r.ArrayVariables,
		DynamicPropertyFields: r.DynamicPropertyFields,
	}
}

// Test struct with omitempty tags
type TestApp struct {
	Name        string     `yaml:"name"`
	ClientID    string     `yaml:"clientId,omitempty"`
	RedirectURI string     `yaml:"redirectUri,omitempty"`
	OAuth       *TestOAuth `yaml:"oauth,omitempty"`
	Scopes      []string   `yaml:"scopes,omitempty"`
}

type TestOAuth struct {
	GrantTypes   []string `yaml:"grantTypes,omitempty"`
	ClientSecret string   `yaml:"clientSecret,omitempty"`
}

func TestToParameterizedYAML_WithOmitemptyFields(t *testing.T) {
	// Test struct with empty/nil fields that have omitempty tags
	app := &TestApp{
		Name:        "TestApp",
		ClientID:    "", // Empty - would normally be omitted
		RedirectURI: "", // Empty - would normally be omitted
		OAuth: &TestOAuth{
			GrantTypes:   nil, // Nil slice - would normally be omitted
			ClientSecret: "",  // Empty - would normally be omitted
		},
		Scopes: nil, // Nil slice - would normally be omitted
	}

	rules := templatingRules{
		Application: &resourceRules{
			Variables: []string{
				"ClientID",
				"RedirectURI",
				"OAuth.ClientSecret",
			},
			ArrayVariables: []string{
				"OAuth.GrantTypes",
				"Scopes",
			},
		},
	}

	parameterizer := newParameterizer(rules)
	result, _, err := parameterizer.ToParameterizedYAML(
		app, "Application", "TestApp", toDeclarativeResourceRules(rules.Application))

	require.NoError(t, err)
	require.NotEmpty(t, result)

	// Verify that omitempty fields are included and parameterized
	assert.Contains(t, result, "clientId:", "ClientID field should be present even though it's empty")
	assert.Contains(t, result, "{{.TEST_APP_CLIENT_ID}}", "ClientID should be parameterized")

	assert.Contains(t, result, "redirectUri:", "RedirectURI field should be present even though it's empty")
	assert.Contains(t, result, "{{.TEST_APP_REDIRECT_URI}}", "RedirectURI should be parameterized")

	assert.Contains(t, result, "clientSecret:", "ClientSecret field should be present even though it's empty")
	assert.Contains(t, result, "{{.TEST_APP_CLIENT_SECRET}}", "ClientSecret should be parameterized")

	// Verify array parameterization
	assert.Contains(t, result, "grantTypes:", "GrantTypes field should be present")
	assert.Contains(t, result, "{{- range .TEST_APP_GRANT_TYPES}}", "GrantTypes should have range template")

	assert.Contains(t, result, "scopes:", "Scopes field should be present")
	assert.Contains(t, result, "{{- range .TEST_APP_SCOPES}}", "Scopes should have range template")
}

func TestToParameterizedYAML_WithPopulatedFields(t *testing.T) {
	// Test struct with populated fields
	app := &TestApp{
		Name:        "TestApp",
		ClientID:    "test-client-id",
		RedirectURI: "https://example.com/callback",
		OAuth: &TestOAuth{
			GrantTypes:   []string{"authorization_code", "refresh_token"},
			ClientSecret: "secret123",
		},
		Scopes: []string{"openid", "profile"},
	}

	rules := templatingRules{
		Application: &resourceRules{
			Variables: []string{
				"ClientID",
				"RedirectURI",
				"OAuth.ClientSecret",
			},
			ArrayVariables: []string{
				"OAuth.GrantTypes",
				"Scopes",
			},
		},
	}

	parameterizer := newParameterizer(rules)
	result, _, err := parameterizer.ToParameterizedYAML(
		app, "Application", "TestApp", toDeclarativeResourceRules(rules.Application))

	require.NoError(t, err)
	require.NotEmpty(t, result)

	// Verify parameterization replaced actual values
	assert.Contains(t, result, "{{.TEST_APP_CLIENT_ID}}", "ClientID should be parameterized")
	assert.NotContains(t, result, "test-client-id", "Original ClientID value should be replaced")

	assert.Contains(t, result, "{{.TEST_APP_REDIRECT_URI}}", "RedirectURI should be parameterized")
	assert.NotContains(t, result, "https://example.com/callback", "Original RedirectURI should be replaced")

	assert.Contains(t, result, "{{.TEST_APP_CLIENT_SECRET}}", "ClientSecret should be parameterized")
	assert.NotContains(t, result, "secret123", "Original ClientSecret should be replaced")

	// Verify array parameterization
	assert.Contains(t, result, "{{- range .TEST_APP_GRANT_TYPES}}", "GrantTypes should have range template")
	assert.NotContains(t, result, "authorization_code", "Original grant type should be replaced")

	assert.Contains(t, result, "{{- range .TEST_APP_SCOPES}}", "Scopes should have range template")
	assert.NotContains(t, result, "openid", "Original scope should be replaced")
}

func TestToParameterizedYAML_MixedEmptyAndPopulated(t *testing.T) {
	// Test with mix of empty and populated fields
	app := &TestApp{
		Name:        "TestApp",
		ClientID:    "populated-client-id",
		RedirectURI: "", // Empty
		OAuth: &TestOAuth{
			GrantTypes:   []string{"authorization_code"}, // Populated
			ClientSecret: "",                             // Empty
		},
		Scopes: nil, // Nil
	}

	rules := templatingRules{
		Application: &resourceRules{
			Variables: []string{
				"ClientID",
				"RedirectURI",
				"OAuth.ClientSecret",
			},
			ArrayVariables: []string{
				"OAuth.GrantTypes",
				"Scopes",
			},
		},
	}

	parameterizer := newParameterizer(rules)
	result, _, err := parameterizer.ToParameterizedYAML(
		app, "Application", "TestApp", toDeclarativeResourceRules(rules.Application))

	require.NoError(t, err)
	require.NotEmpty(t, result)

	// All fields should be parameterized regardless of whether they were empty
	assert.Contains(t, result, "clientId:", "ClientID field should be present")
	assert.Contains(t, result, "{{.TEST_APP_CLIENT_ID}}", "ClientID should be parameterized")

	assert.Contains(t, result, "redirectUri:", "RedirectURI field should be present even though empty")
	assert.Contains(t, result, "{{.TEST_APP_REDIRECT_URI}}", "RedirectURI should be parameterized")

	assert.Contains(t, result, "clientSecret:", "ClientSecret field should be present even though empty")
	assert.Contains(t, result, "{{.TEST_APP_CLIENT_SECRET}}", "ClientSecret should be parameterized")

	assert.Contains(t, result, "grantTypes:", "GrantTypes should be present")
	assert.Contains(t, result, "{{- range .TEST_APP_GRANT_TYPES}}", "GrantTypes should have range template")

	assert.Contains(t, result, "scopes:", "Scopes should be present even though nil")
	assert.Contains(t, result, "{{- range .TEST_APP_SCOPES}}", "Scopes should have range template")
}

func TestStructToMapIgnoringOmitempty(t *testing.T) {
	app := &TestApp{
		Name:        "TestApp",
		ClientID:    "",
		RedirectURI: "",
		OAuth: &TestOAuth{
			GrantTypes:   nil,
			ClientSecret: "",
		},
		Scopes: nil,
	}

	parameterizer := newParameterizer(templatingRules{})
	result, err := parameterizer.structToMapIgnoringOmitempty(app)

	require.NoError(t, err)
	require.NotNil(t, result)

	// All fields should be present in the map, even if empty
	assert.Contains(t, result, "name")
	assert.Contains(t, result, "clientId")
	assert.Contains(t, result, "redirectUri")
	assert.Contains(t, result, "oauth")
	assert.Contains(t, result, "scopes")

	// Check nested struct
	oauth, ok := result["oauth"].(map[string]interface{})
	require.True(t, ok, "oauth should be a map")
	assert.Contains(t, oauth, "grantTypes")
	assert.Contains(t, oauth, "clientSecret")
}

func TestConvertFieldToInterface_NestedStructs(t *testing.T) {
	app := &TestApp{
		Name: "TestApp",
		OAuth: &TestOAuth{
			GrantTypes:   []string{"code"},
			ClientSecret: "secret",
		},
	}

	parameterizer := newParameterizer(templatingRules{})
	result, err := parameterizer.structToMapIgnoringOmitempty(app)

	require.NoError(t, err)

	oauth, ok := result["oauth"].(map[string]interface{})
	require.True(t, ok)

	grantTypes, ok := oauth["grantTypes"].([]interface{})
	require.True(t, ok)
	assert.Len(t, grantTypes, 1)
	assert.Equal(t, "code", grantTypes[0])

	clientSecret, ok := oauth["clientSecret"].(string)
	require.True(t, ok)
	assert.Equal(t, "secret", clientSecret)
}

func TestPathToVariableName(t *testing.T) {
	parameterizer := newParameterizer(templatingRules{})

	tests := []struct {
		appName  string
		path     string
		expected string
	}{
		{"TestApp", "ClientID", "TEST_APP_CLIENT_ID"},
		{"TestApp", "RedirectURI", "TEST_APP_REDIRECT_URI"},
		{"TestApp", "OAuth.ClientSecret", "TEST_APP_CLIENT_SECRET"},
		{"TestApp", "OAuth.GrantTypes", "TEST_APP_GRANT_TYPES"},
		{"TestApp", "InboundAuthConfig.OAuth.ClientID", "TEST_APP_CLIENT_ID"},
		{"TestApp", "simpleField", "TEST_APP_SIMPLE_FIELD"},
		{"TestApp", "ALLCAPS", "TEST_APP_ALLCAPS"},
		{"My App", "ClientID", "MY_APP_CLIENT_ID"},
		{"My Test App", "RedirectURI", "MY_TEST_APP_REDIRECT_URI"},
	}

	for _, tt := range tests {
		t.Run(tt.appName+"_"+tt.path, func(t *testing.T) {
			result := parameterizer.pathToVariableName(tt.appName, tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// =============================================================================
// Phase 2: Omitempty Behavior Tests
// =============================================================================

// TestOmitempty_EmptyFieldsWithoutRules tests that empty fields with omitempty
// are omitted when they're NOT in parameterization rules
func TestOmitempty_EmptyFieldsWithoutRules(t *testing.T) {
	app := &TestApp{
		Name:        "TestApp",
		ClientID:    "", // Empty with omitempty - should be omitted
		RedirectURI: "", // Empty with omitempty - should be omitted
		OAuth: &TestOAuth{
			GrantTypes:   nil, // Nil slice with omitempty - should be omitted
			ClientSecret: "",  // Empty with omitempty - should be omitted
		},
		Scopes: nil, // Nil slice with omitempty - should be omitted
	}

	// No parameterization rules - omitempty should work normally
	parameterizer := newParameterizer(templatingRules{})
	result, _, err := parameterizer.ToParameterizedYAML(app, "Application", "TestApp", nil)

	require.NoError(t, err)
	require.NotEmpty(t, result)

	// Verify omitempty fields are NOT present
	assert.NotContains(t, result, "clientId:", "Empty ClientID should be omitted")
	assert.NotContains(t, result, "redirectUri:", "Empty RedirectURI should be omitted")
	assert.NotContains(t, result, "grantTypes:", "Nil GrantTypes should be omitted")
	assert.NotContains(t, result, "clientSecret:", "Empty ClientSecret should be omitted")
	assert.NotContains(t, result, "scopes:", "Nil Scopes should be omitted")

	// Verify non-omitempty fields are present
	assert.Contains(t, result, "name: TestApp", "Name field should be present")
}

// TestOmitempty_EmptyArraysOmitted verifies empty arrays with omitempty are not in output
func TestOmitempty_EmptyArraysOmitted(t *testing.T) {
	app := &TestApp{
		Name: "TestApp",
		OAuth: &TestOAuth{
			GrantTypes: []string{}, // Empty array - should be omitted
		},
		Scopes: []string{}, // Empty array - should be omitted
	}

	parameterizer := newParameterizer(templatingRules{})
	result, _, err := parameterizer.ToParameterizedYAML(app, "Application", "TestApp", nil)

	require.NoError(t, err)
	assert.NotContains(t, result, "grantTypes:", "Empty GrantTypes array should be omitted")
	assert.NotContains(t, result, "scopes:", "Empty Scopes array should be omitted")
}

// TestOmitempty_NilSlicesOmitted verifies nil slices with omitempty are not in output
func TestOmitempty_NilSlicesOmitted(t *testing.T) {
	app := &TestApp{
		Name: "TestApp",
		OAuth: &TestOAuth{
			GrantTypes: nil, // Nil slice
		},
		Scopes: nil, // Nil slice
	}

	parameterizer := newParameterizer(templatingRules{})
	result, _, err := parameterizer.ToParameterizedYAML(app, "Application", "TestApp", nil)

	require.NoError(t, err)
	assert.NotContains(t, result, "grantTypes:", "Nil GrantTypes should be omitted")
	assert.NotContains(t, result, "scopes:", "Nil Scopes should be omitted")
}

// TestOmitempty_NilPointersOmitted verifies nil pointers with omitempty are not in output
func TestOmitempty_NilPointersOmitted(t *testing.T) {
	app := &TestApp{
		Name:  "TestApp",
		OAuth: nil, // Nil pointer - should be omitted
	}

	parameterizer := newParameterizer(templatingRules{})
	result, _, err := parameterizer.ToParameterizedYAML(app, "Application", "TestApp", nil)

	require.NoError(t, err)
	assert.NotContains(t, result, "oauth:", "Nil OAuth pointer should be omitted")
	assert.Contains(t, result, "name: TestApp")
}

// =============================================================================
// Phase 3: Parameterization Override Tests
// =============================================================================

// TestParameterization_OverridesOmitemptyForVariables verifies that empty string
// fields in parameterization rules are included despite omitempty
func TestParameterization_OverridesOmitemptyForVariables(t *testing.T) {
	app := &TestApp{
		Name:        "TestApp",
		ClientID:    "", // Empty but in rules - should be included and parameterized
		RedirectURI: "", // Empty but in rules - should be included and parameterized
	}

	rules := templatingRules{
		Application: &resourceRules{
			Variables: []string{
				"ClientID",
				"RedirectURI",
			},
		},
	}

	parameterizer := newParameterizer(rules)
	result, _, err := parameterizer.ToParameterizedYAML(
		app, "Application", "TestApp", toDeclarativeResourceRules(rules.Application))

	require.NoError(t, err)

	// These fields should be present and parameterized despite being empty
	assert.Contains(t, result, "clientId:", "ClientID should be present (in rules)")
	assert.Contains(t, result, "{{.TEST_APP_CLIENT_ID}}", "ClientID should be parameterized")

	assert.Contains(t, result, "redirectUri:", "RedirectURI should be present (in rules)")
	assert.Contains(t, result, "{{.TEST_APP_REDIRECT_URI}}", "RedirectURI should be parameterized")
}

// TestParameterization_OverridesOmitemptyForArrays verifies that empty array
// fields in parameterization rules are included despite omitempty
func TestParameterization_OverridesOmitemptyForArrays(t *testing.T) {
	app := &TestApp{
		Name:   "TestApp",
		Scopes: nil, // Nil but in rules - should be included and parameterized
	}

	rules := templatingRules{
		Application: &resourceRules{
			ArrayVariables: []string{
				"Scopes",
			},
		},
	}

	parameterizer := newParameterizer(rules)
	result, _, err := parameterizer.ToParameterizedYAML(
		app, "Application", "TestApp", toDeclarativeResourceRules(rules.Application))

	require.NoError(t, err)

	// Scopes should be present and parameterized despite being nil
	assert.Contains(t, result, "scopes:", "Scopes should be present (in rules)")
	assert.Contains(t, result, "{{- range .TEST_APP_SCOPES}}", "Scopes should be parameterized")
}

// TestParameterization_NestedFieldsWithOmitempty verifies nested empty fields
// in parameterization are included
func TestParameterization_NestedFieldsWithOmitempty(t *testing.T) {
	app := &TestApp{
		Name: "TestApp",
		OAuth: &TestOAuth{
			ClientSecret: "",  // Empty but in rules
			GrantTypes:   nil, // Nil but in rules
		},
	}

	rules := templatingRules{
		Application: &resourceRules{
			Variables: []string{
				"OAuth.ClientSecret",
			},
			ArrayVariables: []string{
				"OAuth.GrantTypes",
			},
		},
	}

	parameterizer := newParameterizer(rules)
	result, _, err := parameterizer.ToParameterizedYAML(
		app, "Application", "TestApp", toDeclarativeResourceRules(rules.Application))

	require.NoError(t, err)

	// Nested fields should be present and parameterized
	assert.Contains(t, result, "oauth:", "OAuth should be present")
	assert.Contains(t, result, "clientSecret:", "ClientSecret should be present (in rules)")
	assert.Contains(t, result, "{{.TEST_APP_CLIENT_SECRET}}", "ClientSecret should be parameterized")
	assert.Contains(t, result, "grantTypes:", "GrantTypes should be present (in rules)")
	assert.Contains(t, result, "{{- range .TEST_APP_GRANT_TYPES}}", "GrantTypes should be parameterized")
}

// TestParameterization_MixedRulesAndOmitempty verifies correct behavior when
// some fields are in rules and some are not
func TestParameterization_MixedRulesAndOmitempty(t *testing.T) {
	app := &TestApp{
		Name:        "TestApp",
		ClientID:    "Id", // Empty AND in rules - should be included
		RedirectURI: "",   // Empty but NOT in rules - should be omitted
		OAuth: &TestOAuth{
			ClientSecret: "",  // Empty AND in rules - should be included
			GrantTypes:   nil, // Nil but NOT in rules - should be omitted
		},
		Scopes: nil, // Nil but NOT in rules - should be omitted
	}

	rules := templatingRules{
		Application: &resourceRules{
			Variables: []string{
				"ClientID",
				"oauth.ClientSecret",
			},
		},
	}

	parameterizer := newParameterizer(rules)
	result, _, err := parameterizer.ToParameterizedYAML(
		app, "Application", "TestApp", toDeclarativeResourceRules(rules.Application))

	require.NoError(t, err)

	// Fields in rules should be present
	assert.Contains(t, result, "clientId:", "ClientID should be present (in rules)")
	assert.Contains(t, result, "{{.TEST_APP_CLIENT_ID}}", "ClientID should be parameterized")
	assert.Contains(t, result, "clientSecret:", "ClientSecret should be present (in rules)")
	assert.Contains(t, result, "{{.TEST_APP_CLIENT_SECRET}}", "ClientSecret should be parameterized")

	// Fields NOT in rules should be omitted
	assert.NotContains(t, result, "redirectUri:", "RedirectURI should be omitted (not in rules)")
	assert.NotContains(t, result, "grantTypes:", "GrantTypes should be omitted (not in rules)")
	assert.NotContains(t, result, "scopes:", "Scopes should be omitted (not in rules)")
}

// =============================================================================
// Phase 4: Field Order Preservation Tests
// =============================================================================

type OrderTestStruct struct {
	FieldA string `yaml:"fieldA"`
	FieldB string `yaml:"fieldB"`
	FieldC string `yaml:"fieldC,omitempty"`
	FieldD string `yaml:"fieldD"`
	FieldE string `yaml:"fieldE,omitempty"`
}

// TestFieldOrder_TopLevelFieldsPreserved verifies top-level fields maintain struct order
func TestFieldOrder_TopLevelFieldsPreserved(t *testing.T) {
	obj := &OrderTestStruct{
		FieldA: "valueA",
		FieldB: "valueB",
		FieldC: "valueC",
		FieldD: "valueD",
		FieldE: "valueE",
	}

	parameterizer := newParameterizer(templatingRules{})
	result, _, err := parameterizer.ToParameterizedYAML(obj, "Application", "TestApp", nil)

	require.NoError(t, err)

	// Check that fields appear in order
	indexA := indexOf(result, "fieldA:")
	indexB := indexOf(result, "fieldB:")
	indexC := indexOf(result, "fieldC:")
	indexD := indexOf(result, "fieldD:")
	indexE := indexOf(result, "fieldE:")

	assert.True(t, indexA < indexB, "FieldA should come before FieldB")
	assert.True(t, indexB < indexC, "FieldB should come before FieldC")
	assert.True(t, indexC < indexD, "FieldC should come before FieldD")
	assert.True(t, indexD < indexE, "FieldD should come before FieldE")
}

// TestFieldOrder_WithOmittedFields verifies remaining fields maintain relative order after omission
func TestFieldOrder_WithOmittedFields(t *testing.T) {
	obj := &OrderTestStruct{
		FieldA: "valueA",
		FieldB: "valueB",
		FieldC: "", // Empty with omitempty - will be omitted
		FieldD: "valueD",
		FieldE: "", // Empty with omitempty - will be omitted
	}

	parameterizer := newParameterizer(templatingRules{})
	result, _, err := parameterizer.ToParameterizedYAML(obj, "Application", "TestApp", nil)

	require.NoError(t, err)

	// Check that remaining fields maintain order
	indexA := indexOf(result, "fieldA:")
	indexB := indexOf(result, "fieldB:")
	indexD := indexOf(result, "fieldD:")

	assert.True(t, indexA < indexB, "FieldA should come before FieldB")
	assert.True(t, indexB < indexD, "FieldB should come before FieldD")

	// Verify omitted fields are not present
	assert.NotContains(t, result, "fieldC:")
	assert.NotContains(t, result, "fieldE:")
}

type NestedOrderStruct struct {
	Name   string           `yaml:"name"`
	Config *NestedConfigObj `yaml:"config"`
	Extra  string           `yaml:"extra"`
}

type NestedConfigObj struct {
	Setting1 string `yaml:"setting1"`
	Setting2 string `yaml:"setting2"`
	Setting3 string `yaml:"setting3"`
}

// TestFieldOrder_NestedFieldsPreserved verifies nested struct fields maintain order
func TestFieldOrder_NestedFieldsPreserved(t *testing.T) {
	obj := &NestedOrderStruct{
		Name: "test",
		Config: &NestedConfigObj{
			Setting1: "val1",
			Setting2: "val2",
			Setting3: "val3",
		},
		Extra: "extra",
	}

	parameterizer := newParameterizer(templatingRules{})
	result, _, err := parameterizer.ToParameterizedYAML(obj, "Application", "TestApp", nil)

	require.NoError(t, err)

	// Check top-level order
	indexName := indexOf(result, "name:")
	indexConfig := indexOf(result, "config:")
	indexExtra := indexOf(result, "extra:")

	assert.True(t, indexName < indexConfig, "name should come before config")
	assert.True(t, indexConfig < indexExtra, "config should come before extra")

	// Check nested order
	indexSetting1 := indexOf(result, "setting1:")
	indexSetting2 := indexOf(result, "setting2:")
	indexSetting3 := indexOf(result, "setting3:")

	assert.True(t, indexSetting1 < indexSetting2, "setting1 should come before setting2")
	assert.True(t, indexSetting2 < indexSetting3, "setting2 should come before setting3")
}

// =============================================================================
// Phase 5: Edge Case Tests
// =============================================================================

type DeeplyNestedStruct struct {
	Level1 *Level1Struct `yaml:"level1,omitempty"`
}

type Level1Struct struct {
	Name   string        `yaml:"name"`
	Level2 *Level2Struct `yaml:"level2,omitempty"`
}

type Level2Struct struct {
	Name   string        `yaml:"name"`
	Level3 *Level3Struct `yaml:"level3,omitempty"`
}

type Level3Struct struct {
	Name  string   `yaml:"name"`
	Items []string `yaml:"items,omitempty"`
}

// TestEdgeCase_DeeplyNestedStructures tests 3+ level nested structures
func TestEdgeCase_DeeplyNestedStructures(t *testing.T) {
	obj := &DeeplyNestedStruct{
		Level1: &Level1Struct{
			Name: "L1",
			Level2: &Level2Struct{
				Name: "L2",
				Level3: &Level3Struct{
					Name:  "L3",
					Items: []string{"item1", "item2"},
				},
			},
		},
	}

	parameterizer := newParameterizer(templatingRules{})
	result, _, err := parameterizer.ToParameterizedYAML(obj, "Application", "TestApp", nil)

	require.NoError(t, err)
	assert.Contains(t, result, "level1:")
	assert.Contains(t, result, "level2:")
	assert.Contains(t, result, "level3:")
	assert.Contains(t, result, "items:")
	assert.Contains(t, result, "- item1")
	assert.Contains(t, result, "- item2")
}

type ArrayOfStructsTest struct {
	Name  string       `yaml:"name"`
	Items []ItemStruct `yaml:"items"`
}

type ItemStruct struct {
	ID    string `yaml:"id"`
	Value string `yaml:"value,omitempty"`
}

// TestEdgeCase_ArraysOfStructs tests arrays containing struct elements
func TestEdgeCase_ArraysOfStructs(t *testing.T) {
	obj := &ArrayOfStructsTest{
		Name: "TestArray",
		Items: []ItemStruct{
			{ID: "1", Value: "val1"},
			{ID: "2", Value: ""},
		},
	}

	parameterizer := newParameterizer(templatingRules{})
	result, _, err := parameterizer.ToParameterizedYAML(obj, "Application", "TestApp", nil)

	require.NoError(t, err)
	assert.Contains(t, result, "items:")
	assert.Contains(t, result, "id: \"1\"")
	assert.Contains(t, result, "id: \"2\"")
	assert.Contains(t, result, "value: val1")
	// Second item's value should be omitted due to omitempty
	// Count occurrences of "value:" - should only be 1
	valueCount := countOccurrences(result, "value:")
	assert.Equal(t, 1, valueCount, "Only first item should have value field")
}

// TestEdgeCase_EmptyStructWithOmitempty tests struct with all empty fields
func TestEdgeCase_EmptyStructWithOmitempty(t *testing.T) {
	app := &TestApp{
		Name:  "TestApp",
		OAuth: &TestOAuth{
			// All fields empty with omitempty
		},
	}

	parameterizer := newParameterizer(templatingRules{})
	result, _, err := parameterizer.ToParameterizedYAML(app, "Application", "TestApp", nil)

	require.NoError(t, err)

	// OAuth should still be present (not nil), but its fields should be omitted
	assert.Contains(t, result, "oauth:")
	assert.NotContains(t, result, "grantTypes:")
	assert.NotContains(t, result, "clientSecret:")
}

// =============================================================================
// Helper Functions
// =============================================================================

// indexOf returns the index of substr in str, or -1 if not found
func indexOf(str, substr string) int {
	for i := 0; i <= len(str)-len(substr); i++ {
		if str[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// countOccurrences counts how many times substr appears in str
func countOccurrences(str, substr string) int {
	count := 0
	for i := 0; i <= len(str)-len(substr); i++ {
		if str[i:i+len(substr)] == substr {
			count++
			i += len(substr) - 1
		}
	}
	return count
}

// =============================================================================
// Additional Coverage Tests for isEmptyValue
// =============================================================================

type NumericTestStruct struct {
	IntField     int     `yaml:"intField,omitempty"`
	Int8Field    int8    `yaml:"int8Field,omitempty"`
	Int16Field   int16   `yaml:"int16Field,omitempty"`
	Int32Field   int32   `yaml:"int32Field,omitempty"`
	Int64Field   int64   `yaml:"int64Field,omitempty"`
	UintField    uint    `yaml:"uintField,omitempty"`
	Uint8Field   uint8   `yaml:"uint8Field,omitempty"`
	Uint16Field  uint16  `yaml:"uint16Field,omitempty"`
	Uint32Field  uint32  `yaml:"uint32Field,omitempty"`
	Uint64Field  uint64  `yaml:"uint64Field,omitempty"`
	Float32Field float32 `yaml:"float32Field,omitempty"`
	Float64Field float64 `yaml:"float64Field,omitempty"`
	BoolField    bool    `yaml:"boolField,omitempty"`
}

// TestIsEmptyValue_IntegerTypes tests integer type detection
func TestIsEmptyValue_IntegerTypes(t *testing.T) {
	obj := &NumericTestStruct{
		IntField:   0,
		Int8Field:  0,
		Int16Field: 0,
		Int32Field: 0,
		Int64Field: 0,
	}

	parameterizer := newParameterizer(templatingRules{})
	result, _, err := parameterizer.ToParameterizedYAML(obj, "Application", "TestApp", nil)

	require.NoError(t, err)

	// All zero integer fields should be omitted
	assert.NotContains(t, result, "intField:")
	assert.NotContains(t, result, "int8Field:")
	assert.NotContains(t, result, "int16Field:")
	assert.NotContains(t, result, "int32Field:")
	assert.NotContains(t, result, "int64Field:")
}

// TestIsEmptyValue_UnsignedIntegerTypes tests unsigned integer type detection
func TestIsEmptyValue_UnsignedIntegerTypes(t *testing.T) {
	obj := &NumericTestStruct{
		UintField:   0,
		Uint8Field:  0,
		Uint16Field: 0,
		Uint32Field: 0,
		Uint64Field: 0,
	}

	parameterizer := newParameterizer(templatingRules{})
	result, _, err := parameterizer.ToParameterizedYAML(obj, "Application", "TestApp", nil)

	require.NoError(t, err)

	// All zero unsigned integer fields should be omitted
	assert.NotContains(t, result, "uintField:")
	assert.NotContains(t, result, "uint8Field:")
	assert.NotContains(t, result, "uint16Field:")
	assert.NotContains(t, result, "uint32Field:")
	assert.NotContains(t, result, "uint64Field:")
}

// TestIsEmptyValue_FloatTypes tests float type detection
func TestIsEmptyValue_FloatTypes(t *testing.T) {
	obj := &NumericTestStruct{
		Float32Field: 0.0,
		Float64Field: 0.0,
	}

	parameterizer := newParameterizer(templatingRules{})
	result, _, err := parameterizer.ToParameterizedYAML(obj, "Application", "TestApp", nil)

	require.NoError(t, err)

	// All zero float fields should be omitted
	assert.NotContains(t, result, "float32Field:")
	assert.NotContains(t, result, "float64Field:")
}

// TestIsEmptyValue_BoolType tests bool type detection
func TestIsEmptyValue_BoolType(t *testing.T) {
	obj := &NumericTestStruct{
		BoolField: false,
	}

	parameterizer := newParameterizer(templatingRules{})
	result, _, err := parameterizer.ToParameterizedYAML(obj, "Application", "TestApp", nil)

	require.NoError(t, err)

	// False bool field should be omitted
	assert.NotContains(t, result, "boolField:")
}

// TestIsEmptyValue_NonZeroValues tests that non-zero values are NOT omitted
func TestIsEmptyValue_NonZeroValues(t *testing.T) {
	obj := &NumericTestStruct{
		IntField:     42,
		UintField:    99,
		Float32Field: 3.14,
		Float64Field: 2.718,
		BoolField:    true,
	}

	parameterizer := newParameterizer(templatingRules{})
	result, _, err := parameterizer.ToParameterizedYAML(obj, "Application", "TestApp", nil)

	require.NoError(t, err)

	// Non-zero fields should be present
	assert.Contains(t, result, "intField: 42")
	assert.Contains(t, result, "uintField: 99")
	assert.Contains(t, result, "float32Field: 3.14")
	assert.Contains(t, result, "float64Field: 2.718")
	assert.Contains(t, result, "boolField: true")
}

// =============================================================================
// Additional Coverage Tests for convertFieldToInterface
// =============================================================================

type MapTestStruct struct {
	StringMap map[string]string      `yaml:"stringMap"`
	IntMap    map[string]int         `yaml:"intMap"`
	NestedMap map[string]interface{} `yaml:"nestedMap,omitempty"`
}

// TestConvertFieldToInterface_Maps tests map conversion
func TestConvertFieldToInterface_Maps(t *testing.T) {
	obj := &MapTestStruct{
		StringMap: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
		IntMap: map[string]int{
			"count": 42,
		},
	}

	parameterizer := newParameterizer(templatingRules{})
	result, err := parameterizer.structToMapIgnoringOmitempty(obj)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Check maps are converted
	stringMap, ok := result["stringMap"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "value1", stringMap["key1"])
	assert.Equal(t, "value2", stringMap["key2"])

	intMap, ok := result["intMap"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, 42, intMap["count"])
}

// TestConvertFieldToInterface_EmptyMap tests empty map handling
func TestConvertFieldToInterface_EmptyMap(t *testing.T) {
	obj := &MapTestStruct{
		StringMap: map[string]string{},
		IntMap:    nil,
	}

	parameterizer := newParameterizer(templatingRules{})
	result, err := parameterizer.structToMapIgnoringOmitempty(obj)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Empty map should still be in result
	stringMap, ok := result["stringMap"].(map[string]interface{})
	require.True(t, ok)
	assert.Empty(t, stringMap)
}

type PrimitiveArrayStruct struct {
	IntArray     []int     `yaml:"intArray"`
	BoolArray    []bool    `yaml:"boolArray"`
	Float64Array []float64 `yaml:"float64Array"`
}

// TestConvertFieldToInterface_PrimitiveArrays tests arrays of primitive types
func TestConvertFieldToInterface_PrimitiveArrays(t *testing.T) {
	obj := &PrimitiveArrayStruct{
		IntArray:     []int{1, 2, 3},
		BoolArray:    []bool{true, false, true},
		Float64Array: []float64{1.1, 2.2, 3.3},
	}

	parameterizer := newParameterizer(templatingRules{})
	result, err := parameterizer.structToMapIgnoringOmitempty(obj)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Check int array
	intArray, ok := result["intArray"].([]interface{})
	require.True(t, ok)
	assert.Len(t, intArray, 3)
	assert.Equal(t, 1, intArray[0])
	assert.Equal(t, 2, intArray[1])
	assert.Equal(t, 3, intArray[2])

	// Check bool array
	boolArray, ok := result["boolArray"].([]interface{})
	require.True(t, ok)
	assert.Len(t, boolArray, 3)
	assert.Equal(t, true, boolArray[0])
	assert.Equal(t, false, boolArray[1])

	// Check float array
	floatArray, ok := result["float64Array"].([]interface{})
	require.True(t, ok)
	assert.Len(t, floatArray, 3)
	assert.Equal(t, 1.1, floatArray[0])
}

// =============================================================================
// Additional Coverage Tests for renderNode edge cases
// =============================================================================

type ComplexRenderStruct struct {
	Name           string                `yaml:"name"`
	TemplateField  string                `yaml:"templateField"`
	NestedTemplate *NestedTemplateStruct `yaml:"nestedTemplate"`
	ArrayOfMaps    []map[string]string   `yaml:"arrayOfMaps,omitempty"`
}

type NestedTemplateStruct struct {
	Value       string   `yaml:"value"`
	TemplateArr []string `yaml:"templateArr"`
}

// TestRenderNode_ComplexTemplateStructures tests complex rendering scenarios
func TestRenderNode_ComplexTemplateStructures(t *testing.T) {
	obj := &ComplexRenderStruct{
		Name:          "ComplexTest",
		TemplateField: "original_value",
		NestedTemplate: &NestedTemplateStruct{
			Value:       "nested_value",
			TemplateArr: []string{"item1", "item2"},
		},
	}

	rules := templatingRules{
		Application: &resourceRules{
			Variables: []string{
				"TemplateField",
				"NestedTemplate.Value",
			},
			ArrayVariables: []string{
				"NestedTemplate.TemplateArr",
			},
		},
	}

	parameterizer := newParameterizer(rules)
	result, _, err := parameterizer.ToParameterizedYAML(
		obj, "Application", "TestApp", toDeclarativeResourceRules(rules.Application))

	require.NoError(t, err)

	// Check template replacements
	assert.Contains(t, result, "{{.TEST_APP_TEMPLATE_FIELD}}")
	assert.Contains(t, result, "{{.TEST_APP_VALUE}}")
	assert.Contains(t, result, "{{- range .TEST_APP_TEMPLATE_ARR}}")
	assert.Contains(t, result, "{{- end}}")
}

// TestRenderNode_ArrayOfMaps tests rendering arrays containing maps
func TestRenderNode_ArrayOfMaps(t *testing.T) {
	obj := &ComplexRenderStruct{
		Name: "MapArrayTest",
		ArrayOfMaps: []map[string]string{
			{"key1": "value1", "key2": "value2"},
			{"key3": "value3"},
		},
	}

	parameterizer := newParameterizer(templatingRules{})
	result, _, err := parameterizer.ToParameterizedYAML(obj, "Application", "TestApp", nil)

	require.NoError(t, err)

	// Check array of maps is rendered
	assert.Contains(t, result, "arrayOfMaps:")
	assert.Contains(t, result, "key1: value1")
	assert.Contains(t, result, "key2: value2")
	assert.Contains(t, result, "key3: value3")
}

// TestToParameterizedYAML_NilRules tests behavior with nil rules
func TestToParameterizedYAML_NilRules(t *testing.T) {
	obj := &TestApp{
		Name:     "TestApp",
		ClientID: "test-client-id",
	}

	parameterizer := newParameterizer(templatingRules{})
	result, _, err := parameterizer.ToParameterizedYAML(obj, "Application", "TestApp", nil)

	require.NoError(t, err)
	require.NotEmpty(t, result)

	// With nil rules, fields should just be serialized normally without parameterization
	assert.Contains(t, result, "name: TestApp")
	assert.Contains(t, result, "clientId: test-client-id")
	assert.NotContains(t, result, "{{.") // No template variables
}

// TestToParameterizedYAML_NilRulesWithOmitempty tests behavior with nil rules and omitempty fields
func TestToParameterizedYAML_NilRulesWithOmitempty(t *testing.T) {
	obj := &TestApp{
		Name:     "TestApp",
		ClientID: "test-client",
	}

	rules := templatingRules{
		Application: nil, // Nil rules
	}

	parameterizer := newParameterizer(rules)
	result, _, err := parameterizer.ToParameterizedYAML(
		obj, "Application", "TestApp", toDeclarativeResourceRules(rules.Application))

	require.NoError(t, err)
	require.NotEmpty(t, result)

	// Without rules, should just output normal YAML
	assert.Contains(t, result, "name: TestApp")
	assert.Contains(t, result, "clientId: test-client")
	assert.NotContains(t, result, "{{.")
}

// TestStructToMapIgnoringOmitempty_NonStructInput tests error handling for non-struct input
func TestStructToMapIgnoringOmitempty_NonStructInput(t *testing.T) {
	parameterizer := newParameterizer(templatingRules{})

	// Test with string
	_, err := parameterizer.structToMapIgnoringOmitempty("not a struct")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected struct")

	// Test with int
	_, err = parameterizer.structToMapIgnoringOmitempty(123)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected struct")
}

type ComplexNestedArrayStruct struct {
	Name  string                `yaml:"name"`
	Items []NestedItemWithArray `yaml:"items"`
}

type NestedItemWithArray struct {
	ID     string   `yaml:"id"`
	Values []string `yaml:"values"`
}

// TestRenderNode_NestedArraysInSequence tests rendering nested arrays in sequence items
func TestRenderNode_NestedArraysInSequence(t *testing.T) {
	obj := &ComplexNestedArrayStruct{
		Name: "NestedArrayTest",
		Items: []NestedItemWithArray{
			{
				ID:     "item1",
				Values: []string{"val1", "val2"},
			},
		},
	}

	parameterizer := newParameterizer(templatingRules{})
	result, _, err := parameterizer.ToParameterizedYAML(obj, "Application", "TestApp", nil)

	require.NoError(t, err)

	// Check nested array rendering
	assert.Contains(t, result, "items:")
	assert.Contains(t, result, "id: item1")
	assert.Contains(t, result, "values:")
	assert.Contains(t, result, "- val1")
	assert.Contains(t, result, "- val2")
}

// TestFieldToNode_NilPointer tests nil pointer handling in fieldToNode
func TestFieldToNode_NilPointer(t *testing.T) {
	obj := &TestApp{
		Name:  "TestApp",
		OAuth: nil, // Nil pointer
	}

	parameterizer := newParameterizer(templatingRules{})
	result, _, err := parameterizer.ToParameterizedYAML(obj, "Application", "TestApp", nil)

	require.NoError(t, err)

	// Nil OAuth pointer should be omitted due to omitempty
	assert.NotContains(t, result, "oauth:")
}

// TestConvertPathToYAMLPath_NonStructType tests path conversion with non-struct types
func TestConvertPathToYAMLPath_NonStructType(t *testing.T) {
	obj := &TestApp{
		Name:   "TestApp",
		Scopes: []string{"scope1"}, // Scopes is a slice, not a struct
	}

	rules := templatingRules{
		Application: &resourceRules{
			Variables: []string{
				"Scopes.InvalidPath", // This path tries to access a field on a slice
			},
		},
	}

	parameterizer := newParameterizer(rules)
	result, _, err := parameterizer.ToParameterizedYAML(
		obj, "Application", "TestApp", toDeclarativeResourceRules(rules.Application))

	require.NoError(t, err)
	// Should still generate output, even though path doesn't fully resolve
	assert.Contains(t, result, "name: TestApp")
}

// TestFindFieldByNameCaseInsensitive_NotFound tests field lookup when field doesn't exist
func TestFindFieldByNameCaseInsensitive_NotFound(t *testing.T) {
	obj := &TestApp{
		Name: "TestApp",
	}

	rules := templatingRules{
		Application: &resourceRules{
			Variables: []string{
				"NonExistentField", // Field that doesn't exist
			},
		},
	}

	parameterizer := newParameterizer(rules)
	result, _, err := parameterizer.ToParameterizedYAML(
		obj, "Application", "TestApp", toDeclarativeResourceRules(rules.Application))

	require.NoError(t, err)
	// Should generate output without the non-existent field
	assert.Contains(t, result, "name: TestApp")
}

// TestToParameterizedYAML_InvalidStruct tests error when passing non-struct type.
func TestToParameterizedYAML_InvalidStruct(t *testing.T) {
	p := newParameterizer(templatingRules{
		Application: &resourceRules{
			Variables: []string{"Name"},
		},
	})

	// Pass non-struct type
	var notAStruct int = 42

	_, _, err := p.ToParameterizedYAML(notAStruct, "Application", "Test", nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to convert object to node")
	assert.Contains(t, err.Error(), "expected struct")
}

// TestHandleInterfaceValue_WithComplexObject tests handleInterfaceValue with complex nested objects
func TestHandleInterfaceValue_WithComplexObject(t *testing.T) {
	p := newParameterizer(templatingRules{})

	// Create a complex interface{} value
	complexData := map[string]interface{}{
		"title":       "Login Page",
		"description": "Please enter your credentials",
		"components": []interface{}{
			map[string]interface{}{
				"id":       "username_field",
				"type":     "TEXT_INPUT",
				"label":    "Username",
				"required": true,
			},
			map[string]interface{}{
				"id":       "password_field",
				"type":     "PASSWORD_INPUT",
				"label":    "Password",
				"required": true,
			},
		},
		"theme": map[string]interface{}{
			"primaryColor":   "#0066cc",
			"secondaryColor": "#6c757d",
			"fonts": map[string]interface{}{
				"heading": "Arial",
				"body":    "Helvetica",
			},
		},
	}

	// Wrap in interface{} to get the right reflect type
	var interfaceValue interface{} = complexData
	reflectValue := reflect.ValueOf(&interfaceValue).Elem()

	node := p.handleInterfaceValue(reflectValue)

	require.NotNil(t, node)
	assert.Equal(t, yaml.ScalarNode, node.Kind)
	assert.Equal(t, "!!str", node.Tag)

	// Verify it's valid JSON
	var parsed interface{}
	err := json.Unmarshal([]byte(node.Value), &parsed)
	require.NoError(t, err)

	// Verify structure is preserved
	parsedMap, ok := parsed.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Login Page", parsedMap["title"])
	assert.Equal(t, "Please enter your credentials", parsedMap["description"])

	components, ok := parsedMap["components"].([]interface{})
	require.True(t, ok)
	assert.Len(t, components, 2)

	theme, ok := parsedMap["theme"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "#0066cc", theme["primaryColor"])
}

// TestHandleInterfaceValue_WithNilInterface tests handleInterfaceValue with nil interface
func TestHandleInterfaceValue_WithNilInterface(t *testing.T) {
	p := newParameterizer(templatingRules{})

	var nilInterface interface{}
	reflectValue := reflect.ValueOf(&nilInterface).Elem()

	node := p.handleInterfaceValue(reflectValue)

	require.NotNil(t, node)
	assert.Equal(t, yaml.ScalarNode, node.Kind)
	assert.Equal(t, "!!null", node.Tag)
	assert.Equal(t, "null", node.Value)
}

// TestHandleInterfaceValue_WithPrimitiveTypes tests handleInterfaceValue with various primitive types
func TestHandleInterfaceValue_WithPrimitiveTypes(t *testing.T) {
	p := newParameterizer(templatingRules{})

	testCases := []struct {
		name          string
		value         interface{}
		expectedValue string
	}{
		{
			name:          "string value",
			value:         "test string",
			expectedValue: `"test string"`,
		},
		{
			name:          "integer value",
			value:         42,
			expectedValue: "42",
		},
		{
			name:          "float value",
			value:         3.14,
			expectedValue: "3.14",
		},
		{
			name:          "boolean true",
			value:         true,
			expectedValue: "true",
		},
		{
			name:          "boolean false",
			value:         false,
			expectedValue: "false",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Wrap in interface{} to get the right reflect type
			var interfaceValue interface{} = tc.value
			reflectValue := reflect.ValueOf(&interfaceValue).Elem()

			node := p.handleInterfaceValue(reflectValue)

			require.NotNil(t, node)
			assert.Equal(t, yaml.ScalarNode, node.Kind)
			assert.Equal(t, "!!str", node.Tag)
			assert.Equal(t, tc.expectedValue, node.Value)
		})
	}
}

// TestHandleInterfaceValue_WithArrays tests handleInterfaceValue with arrays
func TestHandleInterfaceValue_WithArrays(t *testing.T) {
	p := newParameterizer(templatingRules{})

	testCases := []struct {
		name  string
		value interface{}
	}{
		{
			name:  "string array",
			value: []string{"one", "two", "three"},
		},
		{
			name:  "integer array",
			value: []int{1, 2, 3, 4, 5},
		},
		{
			name: "mixed array",
			value: []interface{}{
				"string",
				42,
				true,
				map[string]interface{}{"key": "value"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Wrap in interface{} to get the right reflect type
			var interfaceValue interface{} = tc.value
			reflectValue := reflect.ValueOf(&interfaceValue).Elem()

			node := p.handleInterfaceValue(reflectValue)

			require.NotNil(t, node)
			assert.Equal(t, yaml.ScalarNode, node.Kind)
			assert.Equal(t, "!!str", node.Tag)

			// Verify it's valid JSON
			var parsed interface{}
			err := json.Unmarshal([]byte(node.Value), &parsed)
			require.NoError(t, err, "Should produce valid JSON")
		})
	}
}

// TestHandleInterfaceValue_WithNestedStructures tests handleInterfaceValue with deeply nested structures
func TestHandleInterfaceValue_WithNestedStructures(t *testing.T) {
	p := newParameterizer(templatingRules{})

	deeplyNested := map[string]interface{}{
		"level1": map[string]interface{}{
			"level2": map[string]interface{}{
				"level3": map[string]interface{}{
					"level4": map[string]interface{}{
						"level5": "deep value",
						"array": []interface{}{
							map[string]interface{}{
								"nested_key": "nested_value",
							},
						},
					},
				},
			},
		},
	}

	// Wrap in interface{} to get the right reflect type
	var interfaceValue interface{} = deeplyNested
	reflectValue := reflect.ValueOf(&interfaceValue).Elem()

	node := p.handleInterfaceValue(reflectValue)

	require.NotNil(t, node)
	assert.Equal(t, yaml.ScalarNode, node.Kind)
	assert.Equal(t, "!!str", node.Tag)

	// Verify it's valid JSON and structure is preserved
	var parsed map[string]interface{}
	err := json.Unmarshal([]byte(node.Value), &parsed)
	require.NoError(t, err)

	// Navigate to the deep value
	level1, ok := parsed["level1"].(map[string]interface{})
	require.True(t, ok)
	level2, ok := level1["level2"].(map[string]interface{})
	require.True(t, ok)
	level3, ok := level2["level3"].(map[string]interface{})
	require.True(t, ok)
	level4, ok := level3["level4"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "deep value", level4["level5"])
}

// TestHandleInterfaceValue_WithEmptyStructures tests handleInterfaceValue with empty containers
func TestHandleInterfaceValue_WithEmptyStructures(t *testing.T) {
	p := newParameterizer(templatingRules{})

	testCases := []struct {
		name  string
		value interface{}
	}{
		{
			name:  "empty map",
			value: map[string]interface{}{},
		},
		{
			name:  "empty slice",
			value: []interface{}{},
		},
		{
			name:  "empty string slice",
			value: []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Wrap in interface{} to get the right reflect type
			var interfaceValue interface{} = tc.value
			reflectValue := reflect.ValueOf(&interfaceValue).Elem()

			node := p.handleInterfaceValue(reflectValue)

			require.NotNil(t, node)
			assert.Equal(t, yaml.ScalarNode, node.Kind)
			assert.Equal(t, "!!str", node.Tag)

			// Verify it's valid JSON
			var parsed interface{}
			err := json.Unmarshal([]byte(node.Value), &parsed)
			require.NoError(t, err)
		})
	}
}

// TestHandleInterfaceValue_WithUnmarshallableType tests fallback to fmt.Sprintf when JSON marshaling fails
func TestHandleInterfaceValue_WithUnmarshallableType(t *testing.T) {
	p := newParameterizer(templatingRules{})

	// Create a type that json.Marshal cannot handle but doesn't panic
	// Use a map with non-string keys (json.Marshal will fail for this)
	type customKey struct {
		ID int
	}

	problemMap := map[customKey]string{
		{ID: 1}: "value1",
		{ID: 2}: "value2",
	}

	// Wrap in interface{} to get the right reflect type
	var interfaceValue interface{} = problemMap
	reflectValue := reflect.ValueOf(&interfaceValue).Elem()

	node := p.handleInterfaceValue(reflectValue)

	require.NotNil(t, node)
	assert.Equal(t, yaml.ScalarNode, node.Kind)
	assert.Equal(t, "!!str", node.Tag)

	// Verify it uses the fmt.Sprintf fallback (should contain "map[")
	assert.Contains(t, node.Value, "map[")

	// Verify it's NOT valid JSON (because it used the fallback)
	var parsed interface{}
	err := json.Unmarshal([]byte(node.Value), &parsed)
	assert.Error(t, err, "Should not be valid JSON since it used fmt.Sprintf fallback")
}

// TestFieldToNode_JSONRawMessage tests that json.RawMessage fields are properly converted to JSON strings.
func TestFieldToNode_JSONRawMessage(t *testing.T) {
	p := newParameterizer(templatingRules{})

	// Test with a JSON object - should be exported as a JSON string
	jsonObj := json.RawMessage(
		`{"username":{"type":"string","required":true},"email":{"type":"string","required":true}}`)
	v := reflect.ValueOf(jsonObj)
	node, err := p.fieldToNode(v, nil, "", "TestSchema")

	require.NoError(t, err)
	require.NotNil(t, node)
	assert.Equal(t, yaml.ScalarNode, node.Kind)
	assert.Equal(t, "!!str", node.Tag)

	// Verify it's a valid JSON string
	assert.JSONEq(t,
		`{"username":{"type":"string","required":true},"email":{"type":"string","required":true}}`, node.Value)
}

// TestFieldToNode_JSONRawMessageArray tests that json.RawMessage with JSON array is properly converted to JSON string.
func TestFieldToNode_JSONRawMessageArray(t *testing.T) {
	p := newParameterizer(templatingRules{})

	// Test with a JSON array - should be exported as a JSON string
	jsonArr := json.RawMessage(`["item1","item2","item3"]`)
	v := reflect.ValueOf(jsonArr)
	node, err := p.fieldToNode(v, nil, "", "TestSchema")

	require.NoError(t, err)
	require.NotNil(t, node)
	assert.Equal(t, yaml.ScalarNode, node.Kind)
	assert.Equal(t, "!!str", node.Tag)

	// Verify it's a valid JSON string
	assert.JSONEq(t, `["item1","item2","item3"]`, node.Value)
}

// TestFieldToNode_JSONRawMessageEmpty tests that empty json.RawMessage is handled correctly.
func TestFieldToNode_JSONRawMessageEmpty(t *testing.T) {
	p := newParameterizer(templatingRules{})

	// Test with empty JSON
	jsonEmpty := json.RawMessage(``)
	v := reflect.ValueOf(jsonEmpty)
	node, err := p.fieldToNode(v, nil, "", "TestSchema")

	require.NoError(t, err)
	require.NotNil(t, node)
	assert.Equal(t, yaml.ScalarNode, node.Kind)
	// Empty json.RawMessage should return null
	assert.Equal(t, "!!null", node.Tag)
	assert.Equal(t, "null", node.Value)
}

// TestFieldToNode_JSONRawMessageNil tests that nil json.RawMessage is handled correctly.
func TestFieldToNode_JSONRawMessageNil(t *testing.T) {
	p := newParameterizer(templatingRules{})

	// Test with nil json.RawMessage (nil slice)
	var jsonNil json.RawMessage // nil slice
	v := reflect.ValueOf(jsonNil)
	node, err := p.fieldToNode(v, nil, "", "TestSchema")

	require.NoError(t, err)
	require.NotNil(t, node)
	assert.Equal(t, yaml.ScalarNode, node.Kind)
	// Nil json.RawMessage (nil slice with Len() == 0) should return null
	assert.Equal(t, "!!null", node.Tag)
	assert.Equal(t, "null", node.Value)
}

// TestFieldToNode_JSONRawMessageInvalid tests that invalid JSON in RawMessage returns an error.
func TestFieldToNode_JSONRawMessageInvalid(t *testing.T) {
	p := newParameterizer(templatingRules{})

	// Test with invalid JSON
	jsonInvalid := json.RawMessage(`{invalid json}`)
	v := reflect.ValueOf(jsonInvalid)
	node, err := p.fieldToNode(v, nil, "", "TestSchema")

	// Should return an error for invalid JSON
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid JSON in RawMessage")
	assert.Nil(t, node)
}

// TestToParameterizedYAML_JSONRawMessageInvalid tests that ToParameterizedYAML propagates
// errors when a struct contains invalid json.RawMessage
func TestToParameterizedYAML_JSONRawMessageInvalid(t *testing.T) {
	type TestSchema struct {
		ID     string          `yaml:"id"`
		Name   string          `yaml:"name"`
		Schema json.RawMessage `yaml:"schema"`
	}

	p := newParameterizer(templatingRules{})

	// Create a schema with invalid JSON
	schema := TestSchema{
		ID:     "test-id",
		Name:   "TestSchema",
		Schema: json.RawMessage(`{invalid json}`),
	}

	// Should return an error
	result, _, err := p.ToParameterizedYAML(schema, "EntityType", "TestSchema", nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid JSON in RawMessage")
	assert.Empty(t, result)
}

// TestToParameterizedYAML_NestedStructWithInvalidJSON tests error propagation
// when a nested struct contains invalid json.RawMessage
func TestToParameterizedYAML_NestedStructWithInvalidJSON(t *testing.T) {
	type NestedConfig struct {
		Name   string          `yaml:"name"`
		Config json.RawMessage `yaml:"config"`
	}

	type OuterStruct struct {
		ID     string        `yaml:"id"`
		Nested *NestedConfig `yaml:"nested"`
	}

	p := newParameterizer(templatingRules{})

	// Create a struct with nested invalid JSON
	obj := OuterStruct{
		ID: "test-id",
		Nested: &NestedConfig{
			Name:   "NestedConfig",
			Config: json.RawMessage(`{corrupted: json without quotes}`),
		},
	}

	// Should return an error from the nested structure
	result, _, err := p.ToParameterizedYAML(obj, "Config", "TestConfig", nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid JSON in RawMessage")
	assert.Empty(t, result)
}

// =============================================================================
// User Type Import/Export Symmetry Tests
// =============================================================================

// TestEntityTypeImportExportSymmetry verifies that exported entity types can be re-imported
// This test ensures alignment between the export format and the import format used in
// entitytype.EntityTypeRequestWithID
func TestEntityTypeImportExportSymmetry(t *testing.T) {
	// Define the export format (EntityType struct)
	type EntityType struct {
		ID                    string          `yaml:"id"`
		Name                  string          `yaml:"name"`
		OUID                  string          `yaml:"organization_unit_id"`
		AllowSelfRegistration bool            `yaml:"allow_self_registration,omitempty"`
		Schema                json.RawMessage `yaml:"schema"`
	}

	// Define the import format (EntityTypeRequestWithID struct)
	type EntityTypeRequestWithID struct {
		ID                    string `yaml:"id"`
		Name                  string `yaml:"name"`
		OUID                  string `yaml:"organization_unit_id"`
		AllowSelfRegistration bool   `yaml:"allow_self_registration,omitempty"`
		Schema                string `yaml:"schema"` // Note: This is a string, not json.RawMessage
	}

	// Original schema with json.RawMessage
	originalSchema := EntityType{
		ID:                    "93e861d5-531a-4495-b373-e3db5250e76a",
		Name:                  "Person",
		OUID:                  "14abcc09-4a7f-417e-be47-88e332148a82",
		AllowSelfRegistration: true,
		Schema: json.RawMessage(
			`{"username":{"type":"string","required":true,"unique":true},"email":{"type":"string","required":true}}`),
	}

	// Export using the parameterizer
	p := newParameterizer(templatingRules{})
	yamlOutput, _, err := p.ToParameterizedYAML(originalSchema, "EntityType", "Person", nil)
	require.NoError(t, err)

	t.Logf("Exported YAML:\n%s", yamlOutput)

	// Parse the exported YAML back as if importing (using EntityTypeRequestWithID format)
	var importedSchema EntityTypeRequestWithID
	err = yaml.Unmarshal([]byte(yamlOutput), &importedSchema)
	require.NoError(t, err)

	// Verify the imported values match the original
	assert.Equal(t, originalSchema.ID, importedSchema.ID)
	assert.Equal(t, originalSchema.Name, importedSchema.Name)
	assert.Equal(t, originalSchema.OUID, importedSchema.OUID)
	assert.Equal(t, originalSchema.AllowSelfRegistration, importedSchema.AllowSelfRegistration)

	// Verify the schema is a string (as required by import format)
	assert.IsType(t, "", importedSchema.Schema)

	// Verify the schema string contains valid JSON
	assert.True(t, json.Valid([]byte(importedSchema.Schema)), "Schema should be valid JSON")

	// Verify the JSON content is equivalent
	var originalJSON, importedJSON map[string]interface{}
	err = json.Unmarshal(originalSchema.Schema, &originalJSON)
	require.NoError(t, err)
	err = json.Unmarshal([]byte(importedSchema.Schema), &importedJSON)
	require.NoError(t, err)

	assert.Equal(t, originalJSON, importedJSON, "Schema JSON content should match")

	// Verify the schema is NOT an array (the original bug)
	var yamlParsed map[string]interface{}
	err = yaml.Unmarshal([]byte(yamlOutput), &yamlParsed)
	require.NoError(t, err)

	_, isArray := yamlParsed["schema"].([]interface{})
	assert.False(t, isArray, "Schema should NOT be exported as an array of bytes")

	_, isString := yamlParsed["schema"].(string)
	assert.True(t, isString, "Schema should be exported as a string")
}

// TestEntityTypeExportFormat verifies the exact export format
// TestRenderNode_I18nRefsInSequenceItemAreQuoted tests that i18n template references
// (e.g. {{ t(key) }}) in sequence-item mapping values are quoted in the exported YAML
// so that the output is valid YAML and can be re-imported without parse errors.
// Regression test for: message field in flow PROMPT nodes exported without quotes.
func TestRenderNode_I18nRefsInSequenceItemAreQuoted(t *testing.T) {
	type Node struct {
		ID      string `yaml:"id"`
		Type    string `yaml:"type"`
		Message string `yaml:"message,omitempty"`
		Next    string `yaml:"next,omitempty"`
	}
	type Flow struct {
		Name  string `yaml:"name"`
		Nodes []Node `yaml:"nodes"`
	}

	obj := &Flow{
		Name: "User Onboarding Flow",
		Nodes: []Node{
			{
				ID:      "invite_email_service_unavailable",
				Type:    "PROMPT",
				Message: "{{ t(onboarding:forms.invite_email_unavailable.title) }}",
				Next:    "invite_link_status",
			},
			{
				ID:      "invite_link_status",
				Type:    "PROMPT",
				Message: "The invite link is ready to share",
				Next:    "invite_verify",
			},
		},
	}

	p := newParameterizer(templatingRules{})
	// Use empty (non-nil) rules to exercise the custom renderNode path (same as flows)
	result, _, err := p.ToParameterizedYAML(
		obj, "Flow", "User Onboarding Flow",
		toDeclarativeResourceRules(&resourceRules{}))
	require.NoError(t, err)

	// The i18n reference must be single-quoted so the output is valid YAML.
	assert.Contains(t, result, `message: '{{ t(onboarding:forms.invite_email_unavailable.title) }}'`,
		"i18n reference in message field should be quoted in exported YAML")

	// Plain string messages must remain unquoted.
	assert.Contains(t, result, "message: The invite link is ready to share",
		"plain string message should remain unquoted")

	// Verify the output parses as valid YAML and round-trips correctly.
	var parsed Flow
	require.NoError(t, yaml.Unmarshal([]byte(result), &parsed),
		"exported YAML must be parseable")
	assert.Equal(t, "{{ t(onboarding:forms.invite_email_unavailable.title) }}",
		parsed.Nodes[0].Message)
	assert.Equal(t, "The invite link is ready to share", parsed.Nodes[1].Message)
}

// TestRenderMappingValue_I18nRefsAreQuoted tests that i18n refs at the mapping level
// are also quoted, distinguishing them from real parameterization variables.
func TestRenderMappingValue_I18nRefsAreQuoted(t *testing.T) {
	type Resource struct {
		Name    string `yaml:"name"`
		Message string `yaml:"message,omitempty"`
		// A real parameterization variable placeholder (written unquoted)
		Callback string `yaml:"callbackUrl,omitempty"`
	}

	obj := &Resource{
		Name:     "Test",
		Message:  "{{ t(signup:forms.credentials.title) }}",
		Callback: "{{.TEST_CALLBACK_URL}}",
	}

	p := newParameterizer(templatingRules{})
	result, _, err := p.ToParameterizedYAML(
		obj, "Application", "Test",
		toDeclarativeResourceRules(&resourceRules{Variables: []string{"Callback"}}))
	require.NoError(t, err)

	// i18n reference must be single-quoted.
	assert.Contains(t, result, `message: '{{ t(signup:forms.credentials.title) }}'`)
	// Real parameterization variable must remain unquoted (Go template syntax).
	assert.Contains(t, result, `callbackUrl: {{.TEST_CALLBACK_URL}}`)
}

func TestEntityTypeExportFormat(t *testing.T) {
	type EntityType struct {
		ID                    string          `yaml:"id"`
		Name                  string          `yaml:"name"`
		OUID                  string          `yaml:"organization_unit_id"`
		AllowSelfRegistration bool            `yaml:"allow_self_registration,omitempty"`
		Schema                json.RawMessage `yaml:"schema"`
	}

	schema := EntityType{
		ID:                    "test-id",
		Name:                  "TestSchema",
		OUID:                  "test-ou",
		AllowSelfRegistration: false,
		Schema:                json.RawMessage(`{"field1":"value1"}`),
	}

	p := newParameterizer(templatingRules{})
	yamlOutput, _, err := p.ToParameterizedYAML(schema, "EntityType", "TestSchema", nil)
	require.NoError(t, err)

	// Verify the schema field is a plain string in the YAML, not a structured object
	// This ensures it can be imported using the string type in EntityTypeRequestWithID
	assert.Contains(t, yamlOutput, `schema: '{"field1":"value1"}'`,
		"Schema should be exported as a quoted JSON string")
	assert.NotContains(t, yamlOutput, "schema:\n  field1:",
		"Schema should NOT be exported as a nested YAML structure")
	assert.NotContains(t, yamlOutput, "schema:\n  - ",
		"Schema should NOT be exported as a YAML array")
}

// =============================================================================
// Resource Server Export Scenario Tests
// =============================================================================

// TestResourceServerExport_IdentifierAndOUIDNotParameterized verifies that when
// GetResourceRules() returns nil, the identifier and ou_id fields are emitted as
// literal values and NOT replaced with Go template placeholders ({{.…}}).
func TestResourceServerExport_IdentifierAndOUIDNotParameterized(t *testing.T) {
	rs := &resource.ResourceServer{
		ID:         "019ddcf3-c67c-7521-a3a1-6744abb241a7",
		Name:       "System",
		Identifier: "system",
		OUID:       "019ddcf3-c5d8-7375-80e3-c5bf524257c8",
		Delimiter:  ":",
	}

	p := newParameterizer(templatingRules{})
	// nil rules mirrors what GetResourceRules() returns for resource servers
	result, vars, err := p.ToParameterizedYAML(rs, "ResourceServer", "System", nil)
	require.NoError(t, err)

	// identifier must be the literal value, not a template variable
	assert.Contains(t, result, "identifier: system",
		"identifier should be emitted as a literal value")
	assert.NotContains(t, result, "{{.SYSTEM_IDENTIFIER}}",
		"identifier must not be parameterized")

	// ou_id must be the literal value, not a template variable
	assert.Contains(t, result, "ou_id: 019ddcf3-c5d8-7375-80e3-c5bf524257c8",
		"ou_id should be emitted as a literal value")
	assert.NotContains(t, result, "{{.SYSTEM_OU_ID}}",
		"ou_id must not be parameterized")

	// no variables should be extracted since rules are nil
	assert.Empty(t, vars)
}

// TestResourceServerExport_DelimiterIsQuoted verifies that the delimiter field is
// wrapped in double quotes in the exported YAML (yamlfmt:"quoted" tag).
func TestResourceServerExport_DelimiterIsQuoted(t *testing.T) {
	rs := &resource.ResourceServer{
		ID:         "019ddcf3-c67c-7521-a3a1-6744abb241a7",
		Name:       "System",
		Identifier: "system",
		OUID:       "019ddcf3-c5d8-7375-80e3-c5bf524257c8",
		Delimiter:  ":",
	}

	p := newParameterizer(templatingRules{})
	result, _, err := p.ToParameterizedYAML(rs, "ResourceServer", "System", nil)
	require.NoError(t, err)

	// delimiter must be quoted so bare ":" is not parsed as a YAML mapping indicator
	assert.Contains(t, result, `delimiter: ":"`,
		"delimiter should be wrapped in double quotes")
	assert.NotContains(t, result, "delimiter: :",
		"bare unquoted colon must not appear as the delimiter value")

	// Verify the output round-trips: the parsed delimiter value must equal ":"
	var parsed resource.ResourceServer
	require.NoError(t, yaml.Unmarshal([]byte(result), &parsed))
	assert.Equal(t, ":", parsed.Delimiter,
		"round-tripped delimiter should equal \":\"")
}

// --- Tests for `yaml:",inline"` handling in the parameterizer ---

type inlineInner struct {
	A string   `yaml:"a,omitempty"`
	B string   `yaml:"b,omitempty"`
	C []string `yaml:"c,omitempty"`
}

type inlineTopParent struct {
	Name  string      `yaml:"name"`
	Inner inlineInner `yaml:",inline"`
	Tail  string      `yaml:"tail,omitempty"`
}

// TestInlineEmbed_TopLevelFlattensFields verifies a top-level `yaml:",inline"` embedded
// struct has its fields flattened into the parent mapping (not nested under an empty key)
// and that per-field omitempty is still applied to each flattened field.
func TestInlineEmbed_TopLevelFlattensFields(t *testing.T) {
	obj := &inlineTopParent{
		Name:  "MyApp",
		Inner: inlineInner{A: "alpha", B: "", C: []string{"x", "y"}},
		Tail:  "end",
	}

	p := newParameterizer(templatingRules{})
	out, _, err := p.ToParameterizedYAML(obj, "Application", "MyApp", nil)
	require.NoError(t, err)

	// Bug signature: a bare-colon line (empty key holding the embedded struct).
	assert.NotContains(t, out, "\n:\n", "embedded fields must not nest under an empty key")
	// Inline fields appear at the parent's indent level.
	assert.Contains(t, out, "name: MyApp")
	assert.Contains(t, out, "\na: alpha")
	assert.Contains(t, out, "\nc:")
	assert.Contains(t, out, "\ntail: end")
	// B is empty + omitempty + not in rules -> omitted.
	assert.NotContains(t, out, "\nb:")
}

type inlinePtrParent struct {
	Name  string       `yaml:"name"`
	Inner *inlineInner `yaml:",inline"`
}

// TestInlineEmbed_NilPointerSkipped verifies a nil `yaml:",inline"` pointer is skipped
// without emitting anything for it.
func TestInlineEmbed_NilPointerSkipped(t *testing.T) {
	obj := &inlinePtrParent{Name: "App", Inner: nil}
	p := newParameterizer(templatingRules{})
	out, _, err := p.ToParameterizedYAML(obj, "Application", "App", nil)
	require.NoError(t, err)

	assert.Contains(t, out, "name: App")
	assert.NotContains(t, out, "\na:")
	assert.NotContains(t, out, "\nb:")
	assert.NotContains(t, out, "\nc:")
}

// TestInlineEmbed_PointerDereferenced verifies a non-nil `yaml:",inline"` pointer is
// dereferenced and its fields flattened.
func TestInlineEmbed_PointerDereferenced(t *testing.T) {
	obj := &inlinePtrParent{Name: "App", Inner: &inlineInner{A: "alpha"}}
	p := newParameterizer(templatingRules{})
	out, _, err := p.ToParameterizedYAML(obj, "Application", "App", nil)
	require.NoError(t, err)

	assert.NotContains(t, out, "\n:\n")
	assert.Contains(t, out, "name: App")
	assert.Contains(t, out, "a: alpha")
}

type recInner struct {
	Deep string `yaml:"deep,omitempty"`
}

type recMiddle struct {
	Inner recInner `yaml:",inline"`
	Mid   string   `yaml:"mid,omitempty"`
}

type recParent struct {
	Name string    `yaml:"name"`
	Mid  recMiddle `yaml:",inline"`
}

// TestInlineEmbed_RecursivelyFlattensNestedInline verifies an inline embed that itself
// contains an inline embed is recursively flattened into the parent mapping.
func TestInlineEmbed_RecursivelyFlattensNestedInline(t *testing.T) {
	obj := &recParent{Name: "App", Mid: recMiddle{Inner: recInner{Deep: "value"}, Mid: "middle"}}
	p := newParameterizer(templatingRules{})
	out, _, err := p.ToParameterizedYAML(obj, "Application", "App", nil)
	require.NoError(t, err)

	assert.NotContains(t, out, "\n:\n")
	assert.Contains(t, out, "name: App")
	assert.Contains(t, out, "deep: value")
	assert.Contains(t, out, "mid: middle")
}

// TestInlineEmbed_RuleOverridesOmitemptyForInlineField verifies that an empty
// inline-embedded field referenced by parameterization rules is still emitted, exercising
// the rule-based omitempty bypass in appendInlineStructFields.
func TestInlineEmbed_RuleOverridesOmitemptyForInlineField(t *testing.T) {
	obj := &inlineTopParent{Name: "App", Inner: inlineInner{A: "", B: "", C: nil}}
	rules := &resourceRules{Variables: []string{"A"}}

	p := newParameterizer(templatingRules{Application: rules})
	out, _, err := p.ToParameterizedYAML(obj, "Application", "App", toDeclarativeResourceRules(rules))
	require.NoError(t, err)

	// A is empty but referenced by rules -> emitted (omitempty bypassed).
	assert.Contains(t, out, "\na:")
	// B is empty + omitempty + not in rules -> omitted.
	assert.NotContains(t, out, "\nb:")
}

type inlineQuotedInner struct {
	Delim string `yaml:"delim,omitempty" yamlfmt:"quoted"`
}

type inlineQuotedParent struct {
	Name  string            `yaml:"name"`
	Inner inlineQuotedInner `yaml:",inline"`
}

// TestInlineEmbed_QuotedFieldHonored verifies that `yamlfmt:"quoted"` on a field of
// an inline embed is honored (the value is wrapped in double quotes in the output).
func TestInlineEmbed_QuotedFieldHonored(t *testing.T) {
	obj := &inlineQuotedParent{Name: "App", Inner: inlineQuotedInner{Delim: ":"}}
	p := newParameterizer(templatingRules{})
	out, _, err := p.ToParameterizedYAML(obj, "Application", "App", nil)
	require.NoError(t, err)

	assert.Contains(t, out, `delim: ":"`)
}

type bogusInlineParent struct {
	Name string `yaml:"name"`
	// Tagging a non-struct field with `yaml:",inline"` is invalid; the parameterizer
	// must skip it gracefully rather than panic.
	Foo int `yaml:",inline"`
}

// TestInlineEmbed_NonStructValueSkipped verifies that a non-struct field marked
// `yaml:",inline"` is skipped without panicking.
func TestInlineEmbed_NonStructValueSkipped(t *testing.T) {
	obj := &bogusInlineParent{Name: "App", Foo: 42}
	p := newParameterizer(templatingRules{})
	out, _, err := p.ToParameterizedYAML(obj, "Application", "App", nil)
	require.NoError(t, err)

	assert.Contains(t, out, "name: App")
	assert.NotContains(t, out, "42")
}

type skipFieldInner struct {
	Visible string `yaml:"visible,omitempty"`
	Skipped string `yaml:"-"`
	NoTag   string
}

type skipFieldParent struct {
	Name  string         `yaml:"name"`
	Inner skipFieldInner `yaml:",inline"`
}

// TestInlineEmbed_HiddenAndUntaggedFieldsSkipped verifies that within an inline embed,
// fields with `yaml:"-"` or no yaml tag at all are skipped.
func TestInlineEmbed_HiddenAndUntaggedFieldsSkipped(t *testing.T) {
	obj := &skipFieldParent{
		Name:  "App",
		Inner: skipFieldInner{Visible: "yes", Skipped: "secret", NoTag: "untagged"},
	}
	p := newParameterizer(templatingRules{})
	out, _, err := p.ToParameterizedYAML(obj, "Application", "App", nil)
	require.NoError(t, err)

	assert.Contains(t, out, "visible: yes")
	assert.NotContains(t, out, "secret")
	assert.NotContains(t, out, "untagged")
}

type nestedInlineChild struct {
	Y     string      `yaml:"y,omitempty"`
	Inner inlineInner `yaml:",inline"`
}

type nestedInlineParent struct {
	Name  string            `yaml:"name"`
	Child nestedInlineChild `yaml:"child"`
}

// TestInlineEmbed_InsideNestedStructFlattened verifies the inline detection in the
// nested-struct walker (handleStructNode): an inline embed inside a non-inline child
// struct is also flattened into the child's mapping.
func TestInlineEmbed_InsideNestedStructFlattened(t *testing.T) {
	obj := &nestedInlineParent{
		Name:  "App",
		Child: nestedInlineChild{Y: "outer", Inner: inlineInner{A: "alpha"}},
	}
	p := newParameterizer(templatingRules{})
	out, _, err := p.ToParameterizedYAML(obj, "Application", "App", nil)
	require.NoError(t, err)

	// No empty-key nesting inside the child mapping.
	assert.NotContains(t, out, "\n  :\n")
	assert.Contains(t, out, "child:")
	assert.Contains(t, out, "y: outer")
	assert.Contains(t, out, "a: alpha")
}
