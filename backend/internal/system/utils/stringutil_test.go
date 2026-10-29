// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type StringUtilTestSuite struct {
	suite.Suite
}

func TestStringUtilSuite(t *testing.T) {
	suite.Run(t, new(StringUtilTestSuite))
}

func (suite *StringUtilTestSuite) TestParseStringArray() {
	testCases := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "EmptyString",
			input:    "",
			expected: []string{},
		},
		{
			name:     "SingleValue",
			input:    "value1",
			expected: []string{"value1"},
		},
		{
			name:     "MultipleValues",
			input:    "value1,value2,value3",
			expected: []string{"value1", "value2", "value3"},
		},
		{
			name:     "ValuesWithSpaces",
			input:    "value1, value2, value3",
			expected: []string{"value1", "value2", "value3"},
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			result := ParseStringArray(tc.input, ",")
			assert.Equal(t, tc.expected, result)
		})
	}
}

func (suite *StringUtilTestSuite) TestParseTypedStringArray() {
	type MyString string
	testCases := []struct {
		name     string
		input    string
		sep      string
		expected []MyString
	}{
		{
			name:     "EmptyString",
			input:    "",
			sep:      ",",
			expected: []MyString{},
		},
		{
			name:     "SingleValue",
			input:    "value1",
			sep:      ",",
			expected: []MyString{"value1"},
		},
		{
			name:     "MultipleValues",
			input:    "value1,value2,value3",
			sep:      ",",
			expected: []MyString{"value1", "value2", "value3"},
		},
		{
			name:     "ValuesWithSpaces",
			input:    "value1, value2, value3",
			sep:      ",",
			expected: []MyString{"value1", "value2", "value3"},
		},
		{
			name:     "CustomSeparator",
			input:    "a|b|c",
			sep:      "|",
			expected: []MyString{"a", "b", "c"},
		},
		{
			name:     "EmptySeparatorDefaultsToComma",
			input:    "a,b,c",
			sep:      "",
			expected: []MyString{"a", "b", "c"},
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			result := ParseTypedStringArray[MyString](tc.input, tc.sep)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func (suite *StringUtilTestSuite) TestConvertInterfaceMapToStringMap() {
	testCases := []struct {
		name     string
		input    map[string]interface{}
		expected map[string]string
	}{
		{
			name:     "EmptyMap",
			input:    map[string]interface{}{},
			expected: map[string]string{},
		},
		{
			name:     "NilMap",
			input:    nil,
			expected: nil,
		},
		{
			name: "StringValues",
			input: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
			expected: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
		},
		{
			name: "MixedTypeValues",
			input: map[string]interface{}{
				"string": "value",
				"int":    42,
				"bool":   true,
				"float":  3.14,
				"slice":  []string{"a", "b", "c"},
				"nil":    nil,
			},
			expected: map[string]string{
				"string": "value",
				"int":    "42",
				"bool":   "true",
				"float":  "3.14",
				"slice":  "a,b,c",
				"nil":    "",
			},
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			result := ConvertInterfaceMapToStringMap(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

type testStringer struct{}

// Implement the Stringer interface for the test struct
func (s testStringer) String() string {
	return "test-stringer"
}

func (suite *StringUtilTestSuite) TestConvertInterfaceValueToString() {
	testCases := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "NilValue",
			input:    nil,
			expected: "",
		},
		{
			name:     "StringValue",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "BoolValue_True",
			input:    true,
			expected: "true",
		},
		{
			name:     "BoolValue_False",
			input:    false,
			expected: "false",
		},
		{
			name:     "IntValue",
			input:    42,
			expected: "42",
		},
		{
			name:     "Int64Value",
			input:    int64(9223372036854775807),
			expected: "9223372036854775807",
		},
		{
			name:     "UintValue",
			input:    uint(42),
			expected: "42",
		},
		{
			name:     "Float32Value",
			input:    float32(3.14),
			expected: "3.14",
		},
		{
			name:     "Float64Value",
			input:    float64(3.14159265359),
			expected: "3.14159265359",
		},
		{
			name:     "ByteSlice",
			input:    []byte("hello"),
			expected: "hello",
		},
		{
			name:     "StringSlice",
			input:    []string{"a", "b", "c"},
			expected: "a,b,c",
		},
		{
			name:     "IntSlice",
			input:    []int{1, 2, 3},
			expected: "1,2,3",
		},
		{
			name:     "MixedSlice",
			input:    []interface{}{1, "two", true},
			expected: "1,two,true",
		},
		{
			name:     "EmptySlice",
			input:    []string{},
			expected: "",
		},
		{
			name:     "StringerInterface",
			input:    testStringer{},
			expected: "test-stringer",
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			result := ConvertInterfaceValueToString(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func (suite *StringUtilTestSuite) TestMergeStringMaps() {
	testCases := []struct {
		name     string
		dst      map[string]string
		src      map[string]string
		expected map[string]string
	}{
		{
			name:     "BothEmpty",
			dst:      map[string]string{},
			src:      map[string]string{},
			expected: map[string]string{},
		},
		{
			name:     "EmptyDst",
			dst:      map[string]string{},
			src:      map[string]string{"key1": "value1", "key2": "value2"},
			expected: map[string]string{"key1": "value1", "key2": "value2"},
		},
		{
			name:     "EmptySrc",
			dst:      map[string]string{"key1": "value1", "key2": "value2"},
			src:      map[string]string{},
			expected: map[string]string{"key1": "value1", "key2": "value2"},
		},
		{
			name:     "NilDst",
			dst:      nil,
			src:      map[string]string{"key1": "value1", "key2": "value2"},
			expected: map[string]string{"key1": "value1", "key2": "value2"},
		},
		{
			name:     "NilSrc",
			dst:      map[string]string{"key1": "value1", "key2": "value2"},
			src:      nil,
			expected: map[string]string{"key1": "value1", "key2": "value2"},
		},
		{
			name:     "NoOverlap",
			dst:      map[string]string{"key1": "value1", "key2": "value2"},
			src:      map[string]string{"key3": "value3", "key4": "value4"},
			expected: map[string]string{"key1": "value1", "key2": "value2", "key3": "value3", "key4": "value4"},
		},
		{
			name:     "WithOverlap",
			dst:      map[string]string{"key1": "value1", "key2": "value2"},
			src:      map[string]string{"key2": "updated", "key3": "value3"},
			expected: map[string]string{"key1": "value1", "key2": "updated", "key3": "value3"},
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			result := MergeStringMaps(tc.dst, tc.src)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func (suite *StringUtilTestSuite) TestBoolToNumString() {
	testCases := []struct {
		name     string
		input    bool
		expected string
	}{
		{
			name:     "True",
			input:    true,
			expected: "1",
		},
		{
			name:     "False",
			input:    false,
			expected: "0",
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			result := BoolToNumString(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func (suite *StringUtilTestSuite) TestNumStringToBool() {
	testCases := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "One",
			input:    "1",
			expected: true,
		},
		{
			name:     "Zero",
			input:    "0",
			expected: false,
		},
		{
			name:     "EmptyString",
			input:    "",
			expected: false,
		},
		{
			name:     "OtherValue",
			input:    "any other value",
			expected: false,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			result := NumStringToBool(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func (suite *StringUtilTestSuite) TestStringifyStringArray() {
	testCases := []struct {
		name     string
		input    []string
		sep      string
		expected string
	}{
		{
			name:     "EmptySlice",
			input:    []string{},
			sep:      ",",
			expected: "",
		},
		{
			name:     "SingleValue",
			input:    []string{"a"},
			sep:      ",",
			expected: "a",
		},
		{
			name:     "MultipleValues",
			input:    []string{"a", "b", "c"},
			sep:      ",",
			expected: "a,b,c",
		},
		{
			name:     "CustomSeparator",
			input:    []string{"a", "b", "c"},
			sep:      "|",
			expected: "a|b|c",
		},
		{
			name:     "EmptySeparator",
			input:    []string{"a", "b"},
			sep:      "",
			expected: "a,b",
		},
	}
	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			result := StringifyStringArray(tc.input, tc.sep)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func (suite *StringUtilTestSuite) TestParseStringArray_CustomSeparator() {
	testCases := []struct {
		name     string
		input    string
		sep      string
		expected []string
	}{
		{
			name:     "PipeSeparator",
			input:    "a|b|c",
			sep:      "|",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "EmptySeparatorDefaultsToComma",
			input:    "a,b,c",
			sep:      "",
			expected: []string{"a", "b", "c"},
		},
	}
	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			result := ParseStringArray(tc.input, tc.sep)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func (suite *StringUtilTestSuite) TestConvertInterfaceValueToString_ExtraCases() {
	testCases := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "StructValue",
			input:    struct{ X int }{X: 5},
			expected: "{5}",
		},
		{
			name:     "MapValue",
			input:    map[string]int{"a": 1},
			expected: "map[a:1]",
		},
		{
			name:     "NilSlice",
			input:    ([]string)(nil),
			expected: "",
		},
	}
	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			result := ConvertInterfaceValueToString(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func (suite *StringUtilTestSuite) TestMergeStringMaps_OverlapNilValue() {
	dst := map[string]string{"a": "1", "b": "2"}
	src := map[string]string{"b": "", "c": "3"}
	expected := map[string]string{"a": "1", "b": "", "c": "3"}
	result := MergeStringMaps(dst, src)
	assert.Equal(suite.T(), expected, result)
}

func (suite *StringUtilTestSuite) TestHasPrefixFold() {
	tests := []struct {
		name     string
		s        string
		prefix   string
		expected bool
	}{
		{name: "Exact match", s: "Bearer token", prefix: "Bearer ", expected: true},
		{name: "Lowercase", s: "bearer token", prefix: "Bearer ", expected: true},
		{name: "Uppercase", s: "BEARER token", prefix: "Bearer ", expected: true},
		{name: "Mixed case", s: "bEaReR token", prefix: "Bearer ", expected: true},
		{name: "No match", s: "Basic token", prefix: "Bearer ", expected: false},
		{name: "Empty string", s: "", prefix: "Bearer ", expected: false},
		{name: "Empty prefix", s: "anything", prefix: "", expected: true},
		{name: "String shorter than prefix", s: "Be", prefix: "Bearer ", expected: false},
		{name: "Exact prefix only", s: "Bearer ", prefix: "Bearer ", expected: true},
		{name: "Basic exact", s: "Basic dXNlcjpwYXNz", prefix: "Basic ", expected: true},
		{name: "Basic lowercase", s: "basic dXNlcjpwYXNz", prefix: "Basic ", expected: true},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			assert.Equal(suite.T(), tt.expected, HasPrefixFold(tt.s, tt.prefix))
		})
	}
}

func (suite *StringUtilTestSuite) TestContainsFold() {
	list := []string{"google", "github"}
	tests := []struct {
		name     string
		list     []string
		v        string
		expected bool
	}{
		{name: "Exact match", list: list, v: "google", expected: true},
		{name: "Different case", list: list, v: "GitHub", expected: true},
		{name: "No match", list: list, v: "oidc", expected: false},
		{name: "Empty list", list: nil, v: "google", expected: false},
		{name: "Empty value", list: list, v: "", expected: false},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			assert.Equal(suite.T(), tt.expected, ContainsFold(tt.list, tt.v))
		})
	}
}

func (suite *StringUtilTestSuite) TestTrimPrefixFold() {
	tests := []struct {
		name     string
		s        string
		prefix   string
		expected string
	}{
		{name: "Exact match", s: "Bearer token", prefix: "Bearer ", expected: "token"},
		{name: "Lowercase", s: "bearer token", prefix: "Bearer ", expected: "token"},
		{name: "Uppercase", s: "BEARER token", prefix: "Bearer ", expected: "token"},
		{name: "No match", s: "Basic token", prefix: "Bearer ", expected: "Basic token"},
		{name: "Empty string", s: "", prefix: "Bearer ", expected: ""},
		{name: "Empty prefix", s: "anything", prefix: "", expected: "anything"},
		{name: "Basic exact", s: "Basic dXNlcjpwYXNz", prefix: "Basic ", expected: "dXNlcjpwYXNz"},
		{name: "Basic uppercase", s: "BASIC dXNlcjpwYXNz", prefix: "Basic ", expected: "dXNlcjpwYXNz"},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			assert.Equal(suite.T(), tt.expected, TrimPrefixFold(tt.s, tt.prefix))
		})
	}
}

func (suite *StringUtilTestSuite) TestIsScalar() {
	suite.True(IsScalar("hello"))
	suite.True(IsScalar(""))
	suite.True(IsScalar(float64(3.14)))
	suite.True(IsScalar(float64(0)))
	suite.True(IsScalar(true))
	suite.True(IsScalar(false))

	suite.False(IsScalar(nil))
	suite.False(IsScalar(42))
	suite.False(IsScalar(map[string]interface{}{"key": "val"}))
	suite.False(IsScalar([]string{"a", "b"}))
	suite.False(IsScalar(struct{}{}))
}
