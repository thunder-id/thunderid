// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package valuecompare compares values by a declared type for rule and policy operators.
// A value that does not parse as its declared type never satisfies a comparison.
package valuecompare

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Type is the declared type of a compared value.
type Type string

// Supported value types.
const (
	TypeString   Type = "string"
	TypeNumber   Type = "number"
	TypeBoolean  Type = "boolean"
	TypeDate     Type = "date"     // YYYY-MM-DD
	TypeDateTime Type = "dateTime" // RFC 3339
)

// IsOrdered reports whether values of the type can be compared by magnitude.
func (t Type) IsOrdered() bool {
	switch t {
	case TypeNumber, TypeDate, TypeDateTime:
		return true
	}
	return false
}

// Bounds on numbers compared exactly; numbers beyond them are compared as float64.
const (
	maxExactNumberLength   = 1024
	maxExactNumberExponent = 400
)

// Validate reports whether raw parses as a value of type t.
func Validate(raw string, t Type) error {
	switch t {
	case TypeString:
		return nil
	case TypeNumber:
		_, err := parseNumber(raw)
		return err
	case TypeBoolean:
		_, err := strconv.ParseBool(raw)
		return err
	case TypeDate, TypeDateTime:
		_, err := parseTime(raw, t)
		return err
	default:
		return fmt.Errorf("unsupported value type %q", t)
	}
}

// Compare returns -1, 0 or 1 as a is less than, equal to or greater than b for an ordered type.
func Compare(a, b string, t Type) (int, error) {
	switch t {
	case TypeNumber:
		aNum, errA := parseNumber(a)
		bNum, errB := parseNumber(b)
		if errA != nil || errB != nil {
			return 0, errors.New("value is not a number")
		}
		return aNum.compare(bNum)
	case TypeDate, TypeDateTime:
		aTime, errA := parseTime(a, t)
		bTime, errB := parseTime(b, t)
		if errA != nil || errB != nil {
			return 0, fmt.Errorf("value is not a %s", t)
		}
		return aTime.Compare(bTime), nil
	default:
		return 0, fmt.Errorf("values of type %q have no order", t)
	}
}

// Equal reports whether two values of type t are equal, comparing numbers by value.
func Equal(a, b string, t Type) (bool, error) {
	switch t {
	case TypeString:
		return a == b, nil
	case TypeBoolean:
		aBool, errA := strconv.ParseBool(a)
		bBool, errB := strconv.ParseBool(b)
		if errA != nil || errB != nil {
			return false, errors.New("value is not a boolean")
		}
		return aBool == bBool, nil
	default:
		order, err := Compare(a, b, t)
		return err == nil && order == 0, err
	}
}

// Evaluate reports whether actual satisfies the scalar operator op against expected as type t.
func Evaluate(op providers.AuthorizationOperator, actual, expected string, t Type) bool {
	if t == TypeNumber {
		return evaluateNumber(op, actual, expected)
	}
	switch op {
	case providers.AuthorizationOperatorEquals, providers.AuthorizationOperatorNotEquals:
		equal, err := Equal(actual, expected, t)
		if err != nil {
			return false
		}
		return equal == (op == providers.AuthorizationOperatorEquals)
	case providers.AuthorizationOperatorGreaterThan, providers.AuthorizationOperatorLessThan,
		providers.AuthorizationOperatorGreaterThanOrEqual, providers.AuthorizationOperatorLessThanOrEqual:
		order, err := Compare(actual, expected, t)
		return err == nil && orderSatisfies(op, order)
	default:
		return false
	}
}

// EvaluateSet reports whether a set of values satisfies a membership operator against expected.
func EvaluateSet(op providers.AuthorizationOperator, values []string, expected string) bool {
	switch op {
	case providers.AuthorizationOperatorIncludes:
		return slices.Contains(values, expected)
	case providers.AuthorizationOperatorNotIncludes:
		return !slices.Contains(values, expected)
	}
	return false
}

// orderSatisfies reports whether a comparison result satisfies an ordering operator.
func orderSatisfies(op providers.AuthorizationOperator, order int) bool {
	switch op {
	case providers.AuthorizationOperatorGreaterThan:
		return order > 0
	case providers.AuthorizationOperatorLessThan:
		return order < 0
	case providers.AuthorizationOperatorGreaterThanOrEqual:
		return order >= 0
	case providers.AuthorizationOperatorLessThanOrEqual:
		return order <= 0
	}
	return false
}

// number holds a parsed number exactly when possible, otherwise as a float64.
type number struct {
	exact *big.Rat
	float float64
}

// parseNumber accepts exactly what strconv.ParseFloat accepts.
func parseNumber(raw string) (number, error) {
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return number{}, errors.New("value is not a number")
	}
	n := number{float: f}
	if math.IsInf(f, 0) || math.IsNaN(f) || !exactlyRepresentable(raw) {
		return n, nil
	}
	if r, ok := new(big.Rat).SetString(raw); ok { //nolint:gosec // G113 is bounded by exactlyRepresentable
		n.exact = r
	}
	return n, nil
}

// exactlyRepresentable reports whether raw is plain decimal notation within the exact bounds.
func exactlyRepresentable(raw string) bool {
	if len(raw) > maxExactNumberLength {
		return false
	}
	unsigned := strings.TrimLeft(raw, "+-")
	if len(unsigned) > 1 && unsigned[0] == '0' && strings.ContainsAny(unsigned[1:2], "xXbBoO") {
		return false
	}
	if strings.Contains(raw, "_") {
		return false
	}
	if i := strings.IndexAny(raw, "eE"); i >= 0 {
		exponent, err := strconv.Atoi(raw[i+1:])
		if err != nil || exponent > maxExactNumberExponent || exponent < -maxExactNumberExponent {
			return false
		}
	}
	return true
}

// compare returns -1, 0 or 1 as n is less than, equal to or greater than other.
func (n number) compare(other number) (int, error) {
	if n.exact != nil && other.exact != nil {
		return n.exact.Cmp(other.exact), nil
	}
	switch {
	case math.IsNaN(n.float) || math.IsNaN(other.float):
		return 0, errors.New("NaN has no order")
	case n.float < other.float:
		return -1, nil
	case n.float > other.float:
		return 1, nil
	default:
		return 0, nil
	}
}

// evaluateNumber compares exactly when both numbers are exact, otherwise with float64 semantics.
func evaluateNumber(op providers.AuthorizationOperator, actual, expected string) bool {
	a, errA := parseNumber(actual)
	b, errB := parseNumber(expected)
	if errA != nil || errB != nil {
		return false
	}
	if a.exact != nil && b.exact != nil {
		order := a.exact.Cmp(b.exact)
		switch op {
		case providers.AuthorizationOperatorEquals:
			return order == 0
		case providers.AuthorizationOperatorNotEquals:
			return order != 0
		default:
			return orderSatisfies(op, order)
		}
	}
	x, y := a.float, b.float
	switch op {
	case providers.AuthorizationOperatorEquals:
		return x == y
	case providers.AuthorizationOperatorNotEquals:
		return x != y
	case providers.AuthorizationOperatorGreaterThan:
		return x > y
	case providers.AuthorizationOperatorLessThan:
		return x < y
	case providers.AuthorizationOperatorGreaterThanOrEqual:
		return x >= y
	case providers.AuthorizationOperatorLessThanOrEqual:
		return x <= y
	}
	return false
}

// parseTime parses a date or date-time value of the given type.
func parseTime(raw string, t Type) (time.Time, error) {
	layout := time.DateOnly
	if t == TypeDateTime {
		layout = time.RFC3339Nano
	}
	return time.Parse(layout, raw)
}
