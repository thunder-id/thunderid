// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"math"
	"strconv"
)

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
		if uint64(n) > math.MaxInt64 {
			return 0, false
		}
		return int64(n), true // #nosec G115 -- bounds checked above
	case uint8:
		return int64(n), true
	case uint16:
		return int64(n), true
	case uint32:
		return int64(n), true
	case uint64:
		if n > math.MaxInt64 {
			return 0, false
		}
		return int64(n), true // #nosec G115 -- bounds checked above
	case float32:
		return int64(n), true
	case float64:
		return int64(n), true
	default:
		return 0, false
	}
}

// ToBool attempts to convert a value to bool.
// Supports bool and string types ("true"/"false"/"1"/"0" etc. via strconv.ParseBool).
// Returns the bool value and true if successful, or false and false if not convertible.
func ToBool(v any) (bool, bool) {
	switch n := v.(type) {
	case bool:
		return n, true
	case string:
		b, err := strconv.ParseBool(n)
		if err != nil {
			return false, false
		}
		return b, true
	default:
		return false, false
	}
}

// FormatExpiryDuration converts a duration in seconds into a human readable string using the
// largest unit that divides the value evenly, so the rendered duration is never rounded.
func FormatExpiryDuration(seconds int64) string {
	if seconds <= 0 {
		return "0 seconds"
	}

	value, unit := seconds, "second"
	switch {
	case seconds%3600 == 0:
		value, unit = seconds/3600, "hour"
	case seconds%60 == 0:
		value, unit = seconds/60, "minute"
	}

	if value > 1 {
		unit += "s"
	}
	return strconv.FormatInt(value, 10) + " " + unit
}
