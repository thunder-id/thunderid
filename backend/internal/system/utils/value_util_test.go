// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type ValueUtilTestSuite struct {
	suite.Suite
}

func TestValueUtilTestSuite(t *testing.T) {
	suite.Run(t, new(ValueUtilTestSuite))
}

func (suite *ValueUtilTestSuite) TestCompareValues_Strings() {
	assert.True(suite.T(), CompareValues("hello", "hello"))
	assert.False(suite.T(), CompareValues("hello", "world"))
	assert.False(suite.T(), CompareValues("123", 123))
}

func (suite *ValueUtilTestSuite) TestCompareValues_Numbers() {
	// Same type
	assert.True(suite.T(), CompareValues(123, 123))
	assert.True(suite.T(), CompareValues(float64(123.45), float64(123.45)))

	// Different numeric types (should be equal after conversion)
	assert.True(suite.T(), CompareValues(123, float64(123)))
	assert.True(suite.T(), CompareValues(int32(456), int64(456)))
	assert.True(suite.T(), CompareValues(float32(789), float64(789)))

	// Not equal
	assert.False(suite.T(), CompareValues(123, 456))
	assert.False(suite.T(), CompareValues(float64(123.45), float64(123.46)))
}

func (suite *ValueUtilTestSuite) TestCompareValues_Booleans() {
	assert.True(suite.T(), CompareValues(true, true))
	assert.True(suite.T(), CompareValues(false, false))
	assert.False(suite.T(), CompareValues(true, false))
	assert.False(suite.T(), CompareValues(false, true))
}

func (suite *ValueUtilTestSuite) TestCompareValues_Nil() {
	assert.True(suite.T(), CompareValues(nil, nil))
	assert.False(suite.T(), CompareValues(nil, "value"))
	assert.False(suite.T(), CompareValues("value", nil))
	assert.False(suite.T(), CompareValues(nil, 123))
}

func (suite *ValueUtilTestSuite) TestCompareValues_IncompatibleTypes() {
	assert.False(suite.T(), CompareValues("hello", 123))
	assert.False(suite.T(), CompareValues(true, "true"))
	assert.False(suite.T(), CompareValues(123, true))
	assert.False(suite.T(), CompareValues([]string{"a"}, "a"))
}

func (suite *ValueUtilTestSuite) TestToFloat64_Success() {
	testCases := []struct {
		name     string
		input    interface{}
		expected float64
	}{
		{"float64", float64(123.45), 123.45},
		{"float32", float32(123.5), 123.5},
		{"int", 123, 123.0},
		{"int8", int8(127), 127.0},
		{"int16", int16(32767), 32767.0},
		{"int32", int32(456), 456.0},
		{"int64", int64(789), 789.0},
		{"uint", uint(100), 100.0},
		{"uint8", uint8(255), 255.0},
		{"uint16", uint16(65535), 65535.0},
		{"uint32", uint32(999), 999.0},
		{"uint64", uint64(1000), 1000.0},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			result, ok := ToFloat64(tc.input)
			assert.True(suite.T(), ok)
			assert.Equal(suite.T(), tc.expected, result)
		})
	}
}

func (suite *ValueUtilTestSuite) TestToFloat64_Failure() {
	testCases := []struct {
		name  string
		input interface{}
	}{
		{"string", "123"},
		{"bool", true},
		{"nil", nil},
		{"slice", []int{1, 2, 3}},
		{"map", map[string]int{"a": 1}},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			result, ok := ToFloat64(tc.input)
			assert.False(suite.T(), ok)
			assert.Equal(suite.T(), float64(0), result)
		})
	}
}

func (suite *ValueUtilTestSuite) TestToBool_Success() {
	testCases := []struct {
		name     string
		input    interface{}
		expected bool
	}{
		{"bool true", true, true},
		{"bool false", false, false},
		{"string true", "true", true},
		{"string false", "false", false},
		{"string 1", "1", true},
		{"string 0", "0", false},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			result, ok := ToBool(tc.input)
			assert.True(suite.T(), ok)
			assert.Equal(suite.T(), tc.expected, result)
		})
	}
}

func (suite *ValueUtilTestSuite) TestToBool_Failure() {
	testCases := []struct {
		name  string
		input interface{}
	}{
		{"invalid string", "not-a-bool"},
		{"int", 123},
		{"float64", float64(1.0)},
		{"nil", nil},
		{"slice", []int{1, 2, 3}},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			result, ok := ToBool(tc.input)
			assert.False(suite.T(), ok)
			assert.False(suite.T(), result)
		})
	}
}

func (suite *ValueUtilTestSuite) TestFormatExpiryDuration() {
	testCases := []struct {
		name     string
		seconds  int64
		expected string
	}{
		{"Zero seconds", 0, "0 seconds"},
		{"Negative seconds", -30, "0 seconds"},
		{"One second is singular", 1, "1 second"},
		{"Sub minute stays in seconds", 30, "30 seconds"},
		{"59 seconds stays in seconds", 59, "59 seconds"},
		{"60 seconds is singular minute", 60, "1 minute"},
		{"90 seconds is not a whole minute", 90, "90 seconds"},
		{"120 seconds", 120, "2 minutes"},
		{"300 seconds", 300, "5 minutes"},
		{"600 seconds", 600, "10 minutes"},
		{"3600 seconds is singular hour", 3600, "1 hour"},
		{"5400 seconds is not a whole hour", 5400, "90 minutes"},
		{"7200 seconds", 7200, "2 hours"},
		{"86400 seconds", 86400, "24 hours"},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			result := FormatExpiryDuration(tc.seconds)
			assert.Equal(suite.T(), tc.expected, result)
		})
	}
}
