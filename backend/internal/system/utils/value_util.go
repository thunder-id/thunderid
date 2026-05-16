/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package utils

import "strconv"

// CompareValues performs a type-flexible equality comparison between two values.
// This is useful when comparing values that may have different numeric types
// (e.g., comparing int with float64 after JSON unmarshaling).
//
// Returns true if the values are equal, considering type conversions for:
//   - Strings: direct equality
//   - Numbers: compared after converting to float64
//   - Booleans: direct equality
//   - nil values: both nil are equal
//
// Returns false if the values are of incompatible types or not equal.
func CompareValues(a, b interface{}) bool {
	// Handle nil cases
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// For string comparison
	aStr, aIsStr := a.(string)
	bStr, bIsStr := b.(string)
	if aIsStr && bIsStr {
		return aStr == bStr
	}

	// For numeric comparison (JSON numbers are float64)
	aFloat, aIsFloat := ToFloat64(a)
	bFloat, bIsFloat := ToFloat64(b)
	if aIsFloat && bIsFloat {
		return aFloat == bFloat
	}

	// For boolean comparison
	aBool, aIsBool := a.(bool)
	bBool, bIsBool := b.(bool)
	if aIsBool && bIsBool {
		return aBool == bBool
	}

	return false
}

// ToFloat64 attempts to convert a value to float64.
// Supports conversion from all standard numeric types:
// - Floating-point: float32, float64
// - Signed integers: int, int8, int16, int32, int64
// - Unsigned integers: uint, uint8, uint16, uint32, uint64
// Returns the float64 value and true if successful, or 0 and false if not convertible.
func ToFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	default:
		return 0, false
	}
}

// ToInt64 attempts to convert a value to int64.
// Supports conversion from all standard numeric types:
// - Floating-point: float32, float64 (truncated toward zero)
// - Signed integers: int, int8, int16, int32, int64
// - Unsigned integers: uint, uint8, uint16, uint32, uint64
// Returns the int64 value and true if successful, or 0 and false if not convertible.
func ToInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case int:
		return int64(n), true
	case int8:
		return int64(n), true
	case int16:
		return int64(n), true
	case int32:
		return int64(n), true
	case uint:
		return int64(n), true
	case uint8:
		return int64(n), true
	case uint16:
		return int64(n), true
	case uint32:
		return int64(n), true
	case uint64:
		return int64(n), true
	case float32:
		return int64(n), true
	case float64:
		return int64(n), true
	default:
		return 0, false
	}
}

// SecondsToMinutes converts seconds to minutes and returns as a string.
func SecondsToMinutes(seconds int64) string {
	minutes := seconds / 60
	return strconv.FormatInt(minutes, 10)
}
