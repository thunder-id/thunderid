// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

type UtilsTestSuite struct {
	suite.Suite
}

func TestUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(UtilsTestSuite))
}

func (s *UtilsTestSuite) TestReasonFromCode() {
	for code, reason := range map[string]string{
		"USR-1003":                    "usr_1003",
		"SSE-4030":                    "sse_4030",
		"SSE-5000":                    "sse_5000",
		"already_lowercase":           "already_lowercase",
		"FET 10.2/x":                  "fet_10_2_x",
		"Ünknown":                     reasonServerError,
		"Nötig":                       "n_tig",
		"":                            reasonServerError,
		"1001":                        reasonServerError,
		strings.Repeat("A", 65):       reasonServerError,
		"A" + strings.Repeat("b", 63): "a" + strings.Repeat("b", 63),
	} {
		s.Run(code, func() {
			got := reasonFromCode(code)
			s.Equal(reason, got)
			s.Regexp(reasonPattern, got)
		})
	}
}

// An ID is cut to its first 128 characters, counted as characters and not bytes.
func (s *UtilsTestSuite) TestCapIDCutsTo128Characters() {
	for name, tc := range map[string]struct {
		id   string
		want string
	}{
		"short":                  {"user-1", "user-1"},
		"exactly at the cap":     {strings.Repeat("a", maxIDLength), strings.Repeat("a", maxIDLength)},
		"multibyte at the cap":   {strings.Repeat("ü", maxIDLength), strings.Repeat("ü", maxIDLength)},
		"multibyte over the cap": {strings.Repeat("ü", maxIDLength+1), strings.Repeat("ü", maxIDLength)},
	} {
		s.Run(name, func() {
			s.Equal(tc.want, capID(tc.id))
		})
	}
}

// A list detail is cut after the last whole entry that fits 512 characters, counted as characters and not
// bytes, and an entry is never split.
func (s *UtilsTestSuite) TestJoinListCutsAtAWholeEntry() {
	for name, tc := range map[string]struct {
		entries []string
		want    string
	}{
		"no entries":       {nil, ""},
		"entries that fit": {[]string{"a", "b"}, "a,b"},
		"an entry landing on 512": {
			[]string{strings.Repeat("a", 255), strings.Repeat("b", 256)},
			strings.Repeat("a", 255) + "," + strings.Repeat("b", 256)},
		"an entry one over": {
			[]string{strings.Repeat("a", 255), strings.Repeat("b", 257)}, strings.Repeat("a", 255)},
		"a short entry after an overflow is not skipped to": {
			[]string{strings.Repeat("a", 300), strings.Repeat("b", 300), "c"}, strings.Repeat("a", 300)},
		"a first entry too long on its own": {[]string{strings.Repeat("a", 600)}, strings.Repeat("a", maxDetailLength)},
		"multibyte at 512 characters":       {[]string{strings.Repeat("ü", 512)}, strings.Repeat("ü", 512)},
		"multibyte over 512 characters": {
			[]string{strings.Repeat("ü", 300), strings.Repeat("ü", 300)}, strings.Repeat("ü", 300)},
	} {
		s.Run(name, func() {
			s.Equal(tc.want, JoinList(tc.entries))
		})
	}
}

// An entry escapes the separators and the escape character in each part, and cuts each part to the length of
// an ID, so a value from a request can neither add an entry nor split one.
func (s *UtilsTestSuite) TestListEntryEscapesAndCapsEachPart() {
	for name, tc := range map[string]struct {
		parts []string
		want  string
	}{
		"a type alone":                {[]string{"role"}, "role"},
		"a type and an id":            {[]string{"role", "role-1"}, "role:role-1"},
		"an id that reads as entries": {[]string{"role", "x,role:admin"}, `role:x\,role\:admin`},
		"an id with the escape":       {[]string{"role", `a\b`}, `role:a\\b`},
		"an id over the id cap": {
			[]string{"role", strings.Repeat("a", maxIDLength+1)}, "role:" + strings.Repeat("a", maxIDLength)},
		"a multibyte id at the id cap": {
			[]string{"role", strings.Repeat("ü", maxIDLength)}, "role:" + strings.Repeat("ü", maxIDLength)},
	} {
		s.Run(name, func() {
			s.Equal(tc.want, ListEntry(tc.parts...))
		})
	}
}

// An entry that has an ID of the longest length still fits on its own, so a list of failures is never empty.
func (s *UtilsTestSuite) TestAListOfTheLongestEntriesIsNeverEmpty() {
	entry := ListEntry("application", strings.Repeat(",", maxIDLength+10))

	list := JoinList([]string{entry})

	s.Equal(entry, list)
	s.LessOrEqual(len([]rune(list)), maxDetailLength)
}
