// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

const testKeyModified = "modified"

type SliceUtilTestSuite struct {
	suite.Suite
}

func TestSliceUtilTestSuite(t *testing.T) {
	suite.Run(t, new(SliceUtilTestSuite))
}

func (suite *SliceUtilTestSuite) TestUniqueStrings() {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "Empty slice",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "Nil slice",
			input:    nil,
			expected: nil,
		},
		{
			name:     "No duplicates",
			input:    []string{"a", "b", "c"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "With duplicates",
			input:    []string{"a", "b", "a", "c", "b"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "All duplicates",
			input:    []string{"a", "a", "a"},
			expected: []string{"a"},
		},
		{
			name:     "Single element",
			input:    []string{"a"},
			expected: []string{"a"},
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			result := UniqueStrings(tt.input)

			if tt.expected == nil {
				assert.Nil(suite.T(), result)
				return
			}

			// Check length matches
			assert.Equal(suite.T(), len(tt.expected), len(result))

			// Convert result to map for order-independent comparison
			resultMap := make(map[string]bool)
			for _, v := range result {
				resultMap[v] = true
			}

			// Verify all expected values are present
			for _, v := range tt.expected {
				assert.True(suite.T(), resultMap[v], "Expected value %s not found in result", v)
			}
		})
	}
}

func (suite *SliceUtilTestSuite) TestUniqueNonEmptyStrings() {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "Normal with duplicates",
			input:    []string{"a", "b", "a", "c"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "Empty strings filtered",
			input:    []string{"a", "", "b", ""},
			expected: []string{"a", "b"},
		},
		{
			name:     "All empty strings",
			input:    []string{"", ""},
			expected: nil,
		},
		{
			name:     "Empty slice",
			input:    []string{},
			expected: nil,
		},
		{
			name:     "Single element",
			input:    []string{"x"},
			expected: []string{"x"},
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			result := UniqueNonEmptyStrings(tt.input)
			assert.Equal(suite.T(), tt.expected, result)
		})
	}
}

func (suite *SliceUtilTestSuite) TestMergeUniqueStrings() {
	suite.Equal([]string{"a", "b", "c"}, MergeUniqueStrings([]string{"a", "b"}, []string{"b", "c"}))
	suite.Equal([]string{"a"}, MergeUniqueStrings([]string{"a"}, nil))
	var nilSlice []string
	suite.Equal(nilSlice, MergeUniqueStrings(nil, nil))
}

func (suite *SliceUtilTestSuite) TestDeepCopyMapOfStrings() {
	tests := []struct {
		name     string
		input    map[string]string
		expected map[string]string
	}{
		{
			name:     "Nil map",
			input:    nil,
			expected: nil,
		},
		{
			name:     "Empty map",
			input:    map[string]string{},
			expected: map[string]string{},
		},
		{
			name: "Map with values",
			input: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
			expected: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			result := DeepCopyMapOfStrings(tt.input)

			if tt.expected == nil {
				assert.Nil(suite.T(), result)
				return
			}

			assert.Equal(suite.T(), tt.expected, result)

			// Verify it's a deep copy (modifying original doesn't affect copy)
			if len(tt.input) > 0 {
				for k := range tt.input {
					tt.input[k] = testKeyModified
					assert.NotEqual(suite.T(), testKeyModified, result[k])
					break
				}
			}
		})
	}
}

func (suite *SliceUtilTestSuite) TestDeepCopyMapOfStringSlices() {
	tests := []struct {
		name     string
		input    map[string][]string
		expected map[string][]string
	}{
		{
			name:     "Nil map",
			input:    nil,
			expected: nil,
		},
		{
			name:     "Empty map",
			input:    map[string][]string{},
			expected: map[string][]string{},
		},
		{
			name: "Map with values",
			input: map[string][]string{
				"key1": {"value1", "value2"},
				"key2": {"value3"},
			},
			expected: map[string][]string{
				"key1": {"value1", "value2"},
				"key2": {"value3"},
			},
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			result := DeepCopyMapOfStringSlices(tt.input)

			if tt.expected == nil {
				assert.Nil(suite.T(), result)
				return
			}

			assert.Equal(suite.T(), tt.expected, result)

			// Verify it's a deep copy (modifying original doesn't affect copy)
			if len(tt.input) > 0 {
				for k := range tt.input {
					if len(tt.input[k]) > 0 {
						tt.input[k][0] = testKeyModified
						assert.NotEqual(suite.T(), testKeyModified, result[k][0])
					}
					break
				}
			}
		})
	}
}

func (suite *SliceUtilTestSuite) TestMergeInterfaceMaps() {
	tests := []struct {
		name     string
		dst      map[string]interface{}
		src      map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name:     "Both nil",
			dst:      nil,
			src:      nil,
			expected: map[string]interface{}{},
		},
		{
			name:     "Dst nil, src with values",
			dst:      nil,
			src:      map[string]interface{}{"key1": "value1"},
			expected: map[string]interface{}{"key1": "value1"},
		},
		{
			name:     "Dst with values, src nil",
			dst:      map[string]interface{}{"key1": "value1"},
			src:      nil,
			expected: map[string]interface{}{"key1": "value1"},
		},
		{
			name:     "Both with non-overlapping keys",
			dst:      map[string]interface{}{"key1": "value1"},
			src:      map[string]interface{}{"key2": "value2"},
			expected: map[string]interface{}{"key1": "value1", "key2": "value2"},
		},
		{
			name:     "Both with overlapping keys - src overrides dst",
			dst:      map[string]interface{}{"key1": "value1", "key2": "value2"},
			src:      map[string]interface{}{"key2": "newValue2", "key3": "value3"},
			expected: map[string]interface{}{"key1": "value1", "key2": "newValue2", "key3": "value3"},
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			result := MergeInterfaceMaps(tt.dst, tt.src)
			assert.Equal(suite.T(), tt.expected, result)
		})
	}
}

func (suite *SliceUtilTestSuite) TestGetNestedValue() {
	data := map[string]interface{}{
		"scope":  "read",
		"a.b":    "literal-dotted-key",
		"nested": map[string]interface{}{"child": "value"},
	}

	value, ok := GetNestedValue(data, "scope")
	suite.True(ok)
	suite.Equal("read", value)

	value, ok = GetNestedValue(data, "a.b")
	suite.True(ok, "an exact key match wins over dot-notation traversal")
	suite.Equal("literal-dotted-key", value)

	value, ok = GetNestedValue(data, "nested.child")
	suite.True(ok)
	suite.Equal("value", value)

	_, ok = GetNestedValue(data, "nested.missing")
	suite.False(ok)

	_, ok = GetNestedValue(data, "missing")
	suite.False(ok)
}

func (suite *SliceUtilTestSuite) TestDeepCopyMap() {
	tests := []struct {
		name     string
		input    map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name:     "Nil map",
			input:    nil,
			expected: nil,
		},
		{
			name:     "Empty map",
			input:    map[string]interface{}{},
			expected: map[string]interface{}{},
		},
		{
			name: "Map with primitive types",
			input: map[string]interface{}{
				"string": "value",
				"int":    42,
				"float":  3.14,
				"bool":   true,
			},
			expected: map[string]interface{}{
				"string": "value",
				"int":    42,
				"float":  3.14,
				"bool":   true,
			},
		},
		{
			name: "Map with nil value",
			input: map[string]interface{}{
				"key1": "value1",
				"key2": nil,
			},
			expected: map[string]interface{}{
				"key1": "value1",
				"key2": nil,
			},
		},
		{
			name: "Map with nested map",
			input: map[string]interface{}{
				"outer": map[string]interface{}{
					"inner": "value",
					"count": 10,
				},
			},
			expected: map[string]interface{}{
				"outer": map[string]interface{}{
					"inner": "value",
					"count": 10,
				},
			},
		},
		{
			name: "Map with slice of interfaces",
			input: map[string]interface{}{
				"list": []interface{}{"a", "b", "c"},
			},
			expected: map[string]interface{}{
				"list": []interface{}{"a", "b", "c"},
			},
		},
		{
			name: "Map with slice of strings",
			input: map[string]interface{}{
				"strings": []string{"x", "y", "z"},
			},
			expected: map[string]interface{}{
				"strings": []string{"x", "y", "z"},
			},
		},
		{
			name: "Map with complex nested structure",
			input: map[string]interface{}{
				"level1": map[string]interface{}{
					"level2": map[string]interface{}{
						"level3": []interface{}{
							"value1",
							map[string]interface{}{
								"nested": "deep",
							},
							[]string{"a", "b"},
						},
					},
				},
			},
			expected: map[string]interface{}{
				"level1": map[string]interface{}{
					"level2": map[string]interface{}{
						"level3": []interface{}{
							"value1",
							map[string]interface{}{
								"nested": "deep",
							},
							[]string{"a", "b"},
						},
					},
				},
			},
		},
		{
			name: "Map with mixed types in slice",
			input: map[string]interface{}{
				"mixed": []interface{}{
					"string",
					42,
					true,
					3.14,
					nil,
					map[string]interface{}{"key": "value"},
					[]string{"nested", "slice"},
				},
			},
			expected: map[string]interface{}{
				"mixed": []interface{}{
					"string",
					42,
					true,
					3.14,
					nil,
					map[string]interface{}{"key": "value"},
					[]string{"nested", "slice"},
				},
			},
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			result := DeepCopyMap(tt.input)

			if tt.expected == nil {
				assert.Nil(suite.T(), result)
				return
			}

			assert.Equal(suite.T(), tt.expected, result)

			// Verify it's a deep copy by modifying nested structures
			if len(tt.input) > 0 {
				// Test modifying nested map
				if nestedMap, ok := tt.input["outer"].(map[string]interface{}); ok {
					nestedMap[testKeyModified] = "test"
					resultNested := result["outer"].(map[string]interface{})
					_, exists := resultNested[testKeyModified]
					assert.False(suite.T(), exists,
						"Modifying original nested map should not affect copy")
				}

				// Test modifying nested slice
				if nestedSlice, ok := tt.input["list"].([]interface{}); ok && len(nestedSlice) > 0 {
					nestedSlice[0] = testKeyModified
					resultSlice := result["list"].([]interface{})
					assert.NotEqual(suite.T(), testKeyModified, resultSlice[0],
						"Modifying original slice should not affect copy")
				}

				// Test modifying string slice
				if stringSlice, ok := tt.input["strings"].([]string); ok && len(stringSlice) > 0 {
					stringSlice[0] = testKeyModified
					resultSlice := result["strings"].([]string)
					assert.NotEqual(suite.T(), testKeyModified, resultSlice[0],
						"Modifying original string slice should not affect copy")
				}
			}
		})
	}
}

func (suite *SliceUtilTestSuite) TestDeepCopyInterface() {
	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{
			name:     "Nil value",
			input:    nil,
			expected: nil,
		},
		{
			name:     "String value",
			input:    "test string",
			expected: "test string",
		},
		{
			name:     "Integer value",
			input:    42,
			expected: 42,
		},
		{
			name:     "Float value",
			input:    3.14159,
			expected: 3.14159,
		},
		{
			name:     "Boolean value",
			input:    true,
			expected: true,
		},
		{
			name: "Map value",
			input: map[string]interface{}{
				"key": "value",
			},
			expected: map[string]interface{}{
				"key": "value",
			},
		},
		{
			name:     "Slice of interfaces",
			input:    []interface{}{"a", 1, true},
			expected: []interface{}{"a", 1, true},
		},
		{
			name:     "Slice of strings",
			input:    []string{"x", "y", "z"},
			expected: []string{"x", "y", "z"},
		},
		{
			name: "Nested map in slice",
			input: []interface{}{
				map[string]interface{}{
					"nested": "value",
				},
			},
			expected: []interface{}{
				map[string]interface{}{
					"nested": "value",
				},
			},
		},
		{
			name:     "Empty slice of interfaces",
			input:    []interface{}{},
			expected: []interface{}{},
		},
		{
			name:     "Empty slice of strings",
			input:    []string{},
			expected: []string(nil), // append returns nil for empty slices
		},
		{
			name:     "Empty map",
			input:    map[string]interface{}{},
			expected: map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			result := DeepCopyInterface(tt.input)
			assert.Equal(suite.T(), tt.expected, result)

			// Verify deep copy for complex types
			switch v := tt.input.(type) {
			case map[string]interface{}:
				if len(v) > 0 {
					v[testKeyModified] = "test"
					resultMap := result.(map[string]interface{})
					_, exists := resultMap[testKeyModified]
					assert.False(suite.T(), exists,
						"Modifying original map should not affect copy")
				}
			case []interface{}:
				if len(v) > 0 {
					v[0] = testKeyModified
					resultSlice := result.([]interface{})
					assert.NotEqual(suite.T(), testKeyModified, resultSlice[0],
						"Modifying original slice should not affect copy")
				}
			case []string:
				if len(v) > 0 {
					v[0] = testKeyModified
					resultSlice := result.([]string)
					assert.NotEqual(suite.T(), testKeyModified, resultSlice[0],
						"Modifying original string slice should not affect copy")
				}
			}
		})
	}
}
