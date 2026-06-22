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

package flowdef

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNodeDefinitionMarshalYAML_WithComplexMeta tests YAML marshaling with complex meta objects.
func TestNodeDefinitionMarshalYAML_WithComplexMeta(t *testing.T) {
	// Create a complex meta object similar to the test.yaml file
	complexMeta := map[string]interface{}{
		"components": []interface{}{
			map[string]interface{}{
				"id":      "text_001",
				"label":   "{{ t(signin:heading) }}",
				"type":    "TEXT",
				"variant": "HEADING_1",
			},
			map[string]interface{}{
				"id":   "block_001",
				"type": "BLOCK",
				"components": []interface{}{
					map[string]interface{}{
						"id":          "input_001",
						"label":       "{{ t(elements:fields.username.label) }}",
						"placeholder": "{{ t(elements:fields.username.placeholder) }}",
						"ref":         "username",
						"required":    true,
						"type":        "TEXT_INPUT",
					},
					map[string]interface{}{
						"id":          "input_002",
						"label":       "{{ t(elements:fields.password.label) }}",
						"placeholder": "{{ t(elements:fields.password.placeholder) }}",
						"ref":         "password",
						"required":    true,
						"type":        "PASSWORD_INPUT",
					},
					map[string]interface{}{
						"eventType": "SUBMIT",
						"id":        "action_001",
						"label":     "{{ t(elements:buttons.submit.text) }}",
						"type":      "ACTION",
						"variant":   "PRIMARY",
					},
				},
			},
		},
	}

	nodeDef := &NodeDefinition{
		ID:   "prompt_credentials",
		Type: "PROMPT",
		Meta: complexMeta,
		Prompts: []PromptDefinition{
			{
				Inputs: []InputDefinition{
					{Ref: "input_001", Type: "TEXT_INPUT", Identifier: "username", Required: true},
					{Ref: "input_002", Type: "PASSWORD_INPUT", Identifier: "password", Required: true},
				},
				Action: &ActionDefinition{Ref: "action_001", NextNode: "basic_auth"},
			},
		},
	}

	// Marshal to YAML
	result, err := nodeDef.MarshalYAML()
	require.NoError(t, err)

	// Verify the result
	alias, ok := result.(nodeDefinitionAlias)
	require.True(t, ok, "result should be nodeDefinitionAlias")

	// Verify Meta is now a JSON string
	metaStr, ok := alias.Meta.(string)
	require.True(t, ok, "Meta should be converted to string")
	assert.Contains(t, metaStr, "components")
	assert.Contains(t, metaStr, "text_001")
	assert.Contains(t, metaStr, "block_001")
}

// TestNodeDefinitionMarshalYAML_WithNilMeta tests YAML marshaling with nil meta.
func TestNodeDefinitionMarshalYAML_WithNilMeta(t *testing.T) {
	nodeDef := &NodeDefinition{
		ID:   "start",
		Type: "START",
		Meta: nil,
	}

	result, err := nodeDef.MarshalYAML()
	require.NoError(t, err)

	alias, ok := result.(nodeDefinitionAlias)
	require.True(t, ok)
	assert.Nil(t, alias.Meta)
}

// TestNodeDefinitionMarshalYAML_WithJSONMarshallableTypes tests various JSON-marshallable types.
func TestNodeDefinitionMarshalYAML_WithJSONMarshallableTypes(t *testing.T) {
	testCases := []struct {
		name        string
		meta        interface{}
		expectError bool
	}{
		{
			name: "nested arrays and maps",
			meta: []interface{}{
				map[string]interface{}{
					"nested": []interface{}{
						map[string]interface{}{"deep": "value"},
					},
				},
			},
			expectError: false,
		},
		{
			name: "numeric types",
			meta: map[string]interface{}{
				"int":      42,
				"float":    3.14,
				"negInt":   -100,
				"negFloat": -2.5,
				"zero":     0,
			},
			expectError: false,
		},
		{
			name: "boolean and null",
			meta: map[string]interface{}{
				"true":  true,
				"false": false,
				"null":  nil,
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			nodeDef := NodeDefinition{
				ID:   "test",
				Type: "TEST",
				Meta: tc.meta,
			}

			result, err := nodeDef.MarshalYAML()
			if tc.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				alias := result.(nodeDefinitionAlias)
				assert.IsType(t, "", alias.Meta)
			}
		})
	}
}
