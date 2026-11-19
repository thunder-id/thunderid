// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type ValueTestSuite struct {
	suite.Suite
}

func TestValueTestSuite(t *testing.T) {
	suite.Run(t, new(ValueTestSuite))
}

func (s *ValueTestSuite) TestParseDecimal() {
	cases := []struct {
		in        string
		canonical string
		ok        bool
	}{
		{"30", "30", true},
		{"-2", "-2", true},
		{"3.5", "3.5", true},
		{"3.50", "3.5", true},
		{"3.0", "3", true},
		{"007", "7", true},
		{"0", "0", true},
		{"-0", "0", true},
		{"-0.000", "0", true},
		{"0.25", "0.25", true},
		{"1e3", "", false},
		{"+3", "", false},
		{".5", "", false},
		{"5.", "", false},
		{"", "", false},
		{"-", "", false},
		{" 5", "", false},
		{"5 ", "", false},
		{"thirty", "", false},
		{"1.2.3", "", false},
	}
	for _, tc := range cases {
		s.Run(tc.in, func() {
			d, ok := parseDecimal(tc.in)

			s.Equal(tc.ok, ok)
			if tc.ok {
				s.Equal(tc.canonical, d.String())
			}
		})
	}
}

func (s *ValueTestSuite) TestCompareDecimal() {
	cases := []struct {
		a, b string
		want int
	}{
		{"30", "18", 1},
		{"18", "30", -1},
		{"3.0", "3", 0},
		{"10", "9", 1},
		{"-10", "-9", -1},
		{"-1", "1", -1},
		{"1", "-1", 1},
		{"0.5", "0.25", 1},
		{"0.25", "0.250", 0},
		{"-0.5", "-0.25", -1},
		{"100", "99.999", 1},
		{"0", "-0", 0},
	}
	for _, tc := range cases {
		s.Run(tc.a+" vs "+tc.b, func() {
			a, _ := parseDecimal(tc.a)
			b, _ := parseDecimal(tc.b)

			s.Equal(tc.want, compareDecimal(a, b))
		})
	}
}

func (s *ValueTestSuite) TestParseInstant() {
	cases := []struct {
		in   string
		want time.Time
		ok   bool
	}{
		{"2026-01-05", time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC), true},
		{"2026-01-05T10:00:00Z", time.Date(2026, 1, 5, 10, 0, 0, 0, time.UTC), true},
		{"2026-01-05T10:00:00.5Z", time.Date(2026, 1, 5, 10, 0, 0, 500000000, time.UTC), true},
		{"2026-01-05T15:30:00+05:30", time.Date(2026, 1, 5, 10, 0, 0, 0, time.UTC), true},
		{"2026-01-05T10:00:00", time.Time{}, false},
		{"2026-13-05", time.Time{}, false},
		{"2026", time.Time{}, false},
		{"05/01/2026", time.Time{}, false},
		{"1730000000", time.Time{}, false},
	}
	for _, tc := range cases {
		s.Run(tc.in, func() {
			got, ok := parseInstant(tc.in)

			s.Equal(tc.ok, ok)
			if tc.ok {
				s.True(tc.want.Equal(got))
			}
		})
	}
}

func (s *ValueTestSuite) TestParseDuration() {
	cases := []struct {
		in      string
		want    isoDuration
		ok      bool
		nominal int64
	}{
		{"P30D", isoDuration{days: 30}, true, 30 * secondsPerDay},
		{"PT15M", isoDuration{seconds: 900}, true, 900},
		{"P2W", isoDuration{days: 14}, true, 14 * secondsPerDay},
		{"P1Y2M3DT4H5M6S", isoDuration{years: 1, months: 2, days: 3, seconds: 4*3600 + 5*60 + 6}, true,
			(365+60+3)*secondsPerDay + 4*3600 + 5*60 + 6},
		{"PT1H", isoDuration{seconds: 3600}, true, 3600},
		{"P", isoDuration{}, false, 0},
		{"PT", isoDuration{}, false, 0},
		{"P1DT", isoDuration{}, false, 0},
		{"P1H", isoDuration{}, false, 0},
		{"PT1D", isoDuration{}, false, 0},
		{"P1D1Y", isoDuration{}, false, 0},
		{"P1.5D", isoDuration{}, false, 0},
		{"P1234567890D", isoDuration{}, false, 0},
		{"30D", isoDuration{}, false, 0},
		{"PD", isoDuration{}, false, 0},
		{"P1DT2", isoDuration{}, false, 0},
		{"P1DTT1H", isoDuration{}, false, 0},
	}
	for _, tc := range cases {
		s.Run(tc.in, func() {
			got, ok := parseDuration(tc.in)

			s.Equal(tc.ok, ok)
			if tc.ok {
				s.Equal(tc.want, got)
				s.Equal(tc.nominal, got.nominalSeconds())
			}
		})
	}
}

func (s *ValueTestSuite) TestDurationBeforeUsesCalendar() {
	now := time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC)
	d, _ := parseDuration("P1MT1H")

	s.Equal(time.Date(2026, 3, 3, 11, 0, 0, 0, time.UTC), d.before(now))
}

func (s *ValueTestSuite) TestDurationBeyondDurationRangeStaysInThePast() {
	now := time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC)
	d, ok := parseDuration("PT3000000H")
	s.Require().True(ok)

	s.Equal(now.AddDate(0, 0, -125000), d.before(now))
}

func (s *ValueTestSuite) TestNewAtomClassifies() {
	cases := []struct {
		in   string
		want comparatorMask
	}{
		{"30", maskOf(cmpNumber) | maskOf(cmpText)},
		{"2026-01-05", maskOf(cmpInstant) | maskOf(cmpText)},
		{"P30D", maskOf(cmpDuration) | maskOf(cmpText)},
		{"true", maskOf(cmpBoolean) | maskOf(cmpText)},
		{"True", maskOf(cmpText)},
		{"payments-approver", maskOf(cmpText)},
		{"", maskOf(cmpText)},
	}
	for _, tc := range cases {
		s.Run(tc.in, func() {
			s.Equal(tc.want, newAtom(tc.in).classes)
		})
	}
	s.Equal("abc", newAtom("AbC").lower)
}

func (s *ValueTestSuite) TestIsBlank() {
	for _, tc := range []struct {
		in    string
		blank bool
	}{{"", true}, {" \t\n", true}, {" a ", false}} {
		a := newAtom(tc.in)
		s.Equal(tc.blank, a.isBlank())
	}
}
