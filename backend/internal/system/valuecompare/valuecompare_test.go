// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package valuecompare

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type ValueCompareTestSuite struct {
	suite.Suite
}

func TestValueCompareTestSuite(t *testing.T) {
	suite.Run(t, new(ValueCompareTestSuite))
}

func (s *ValueCompareTestSuite) TestTypes() {
	s.True(TypeDate.IsOrdered())
	s.False(TypeString.IsOrdered())
	s.False(TypeBoolean.IsOrdered())
}

func (s *ValueCompareTestSuite) TestValidate() {
	cases := []struct {
		raw   string
		t     Type
		valid bool
	}{
		{"anything", TypeString, true},
		{"45.00", TypeNumber, true},
		{"1e3", TypeNumber, true},
		{"Inf", TypeNumber, true},
		{"1e309", TypeNumber, false},
		{"45 ", TypeNumber, false},
		{"abc", TypeNumber, false},
		{"TRUE", TypeBoolean, true},
		{"yes", TypeBoolean, false},
		{"2026-01-31", TypeDate, true},
		{"2026-02-30", TypeDate, false},
		{"2026-01-31T09:00:00Z", TypeDateTime, true},
		{"2026-01-31", TypeDateTime, false},
		{"7", TypeNumber, true},
		{"x", Type("weight"), false},
	}
	for _, tc := range cases {
		err := Validate(tc.raw, tc.t)
		s.Equal(tc.valid, err == nil, "%q as %s", tc.raw, tc.t)
	}
}

func (s *ValueCompareTestSuite) TestCompare() {
	order := func(a, b string, typ Type) int {
		s.T().Helper()
		o, err := Compare(a, b, typ)
		s.Require().NoError(err, "%q vs %q as %s", a, b, typ)
		return o
	}
	s.Equal(0, order("45", "45.00", TypeNumber))
	s.Equal(-1, order("35.00", "45", TypeNumber))
	s.Equal(1, order("1e3", "999.99", TypeNumber))
	s.Equal(1, order("9007199254740993", "9007199254740992", TypeNumber), "exact beyond float precision")
	s.Equal(1, order("0.1000000000000000055511151231257827", "0.1", TypeNumber))
	s.Equal(1, order("Inf", "1e308", TypeNumber), "an infinity is compared as a float")
	s.Equal(1, order("0x1p1", "1.5", TypeNumber), "a hexadecimal float is compared as a float")
	s.Equal(-1, order("2026-01-30", "2026-01-31", TypeDate))
	s.Equal(0, order("2026-01-31T09:00:00Z", "2026-01-31T10:00:00+01:00", TypeDateTime))

	for _, tc := range []struct {
		a, b string
		t    Type
	}{
		{"NaN", "1", TypeNumber},
		{"x", "1", TypeNumber},
		{"2026-01-31", "soon", TypeDate},
		{"a", "b", TypeString},
		{"true", "false", TypeBoolean},
	} {
		_, err := Compare(tc.a, tc.b, tc.t)
		s.Error(err, "%q vs %q as %s", tc.a, tc.b, tc.t)
	}
}

func (s *ValueCompareTestSuite) TestEqual() {
	equal := func(a, b string, typ Type) bool {
		s.T().Helper()
		e, err := Equal(a, b, typ)
		s.Require().NoError(err)
		return e
	}
	s.True(equal("Admin", "Admin", TypeString))
	s.False(equal("Admin", "admin", TypeString))
	s.True(equal("45", "45.00", TypeNumber))
	s.True(equal("TRUE", "1", TypeBoolean))
	s.True(equal("2026-01-31", "2026-01-31", TypeDate))

	_, err := Equal("yes", "true", TypeBoolean)
	s.Error(err)
	_, err = Equal("x", "1", TypeNumber)
	s.Error(err)
}

func (s *ValueCompareTestSuite) TestEvaluate() {
	cases := []struct {
		op               providers.AuthorizationOperator
		actual, expected string
		t                Type
		want             bool
	}{
		{providers.AuthorizationOperatorEquals, "admin", "admin", TypeString, true},
		{providers.AuthorizationOperatorNotEquals, "admin", "user", TypeString, true},
		{providers.AuthorizationOperatorGreaterThan, "b", "a", TypeString, false},
		{providers.AuthorizationOperatorEquals, "true", "TRUE", TypeBoolean, true},
		{providers.AuthorizationOperatorNotEquals, "true", "false", TypeBoolean, true},
		{providers.AuthorizationOperatorLessThan, "false", "true", TypeBoolean, false},
		{providers.AuthorizationOperatorEquals, "yes", "true", TypeBoolean, false},
		{providers.AuthorizationOperatorEquals, "45", "45.00", TypeNumber, true},
		{providers.AuthorizationOperatorGreaterThanOrEqual, "45", "45.00", TypeNumber, true},
		{providers.AuthorizationOperatorLessThan, "35", "45", TypeNumber, true},
		{providers.AuthorizationOperatorGreaterThan, "x", "1", TypeNumber, false},
		{providers.AuthorizationOperatorNotEquals, "x", "1", TypeNumber, false},
		{providers.AuthorizationOperatorEquals, "NaN", "NaN", TypeNumber, false},
		{providers.AuthorizationOperatorNotEquals, "NaN", "1", TypeNumber, true},
		{providers.AuthorizationOperatorGreaterThan, "Inf", "1e308", TypeNumber, true},
		{providers.AuthorizationOperatorEquals, "9007199254740993", "9007199254740992", TypeNumber, false},
		{providers.AuthorizationOperatorLessThanOrEqual, "2026-01-30", "2026-01-31", TypeDate, true},
		{providers.AuthorizationOperatorGreaterThan, "2026-01-30", "soon", TypeDate, false},
		{providers.AuthorizationOperatorIncludes, "a", "a", TypeString, false},
	}
	for _, tc := range cases {
		s.Equal(tc.want, Evaluate(tc.op, tc.actual, tc.expected, tc.t),
			"%s %q %q as %s", tc.op, tc.actual, tc.expected, tc.t)
	}
}

func (s *ValueCompareTestSuite) TestEvaluateSet() {
	roles := []string{"admin", "editor"}
	s.True(EvaluateSet(providers.AuthorizationOperatorIncludes, roles, "admin"))
	s.False(EvaluateSet(providers.AuthorizationOperatorIncludes, roles, "Admin"))
	s.True(EvaluateSet(providers.AuthorizationOperatorNotIncludes, roles, "viewer"))
	s.False(EvaluateSet(providers.AuthorizationOperatorNotIncludes, roles, "editor"))
	s.False(EvaluateSet(providers.AuthorizationOperatorEquals, roles, "admin"), "only membership operators apply")
}

func (s *ValueCompareTestSuite) TestExactlyRepresentable() {
	s.True(exactlyRepresentable("-12.50e3"))
	s.False(exactlyRepresentable("1e-401"))
	s.False(exactlyRepresentable("0x10"))
	s.False(exactlyRepresentable("1_000"))
	s.False(exactlyRepresentable("1e+x"))
	s.False(exactlyRepresentable(strings.Repeat("9", maxExactNumberLength+1)))
}
