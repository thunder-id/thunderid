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
