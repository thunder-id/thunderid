// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type UtilsTestSuite struct {
	suite.Suite
}

func TestUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(UtilsTestSuite))
}

func (s *UtilsTestSuite) TestValidUntilOf() {
	cases := []struct {
		name        string
		attrs       string
		wantBounded bool
		want        time.Time
		wantErr     bool
	}{
		{name: "no attributes", attrs: ""},
		{name: "absent", attrs: `{"owner":"o-1"}`},
		{name: "unbounded sentinel", attrs: `{"validUntil":"9999-12-31T23:59:59Z"}`},
		{name: "bounded", attrs: `{"validUntil":"2027-03-31T05:30:00+05:30"}`, wantBounded: true,
			want: time.Date(2027, 3, 31, 0, 0, 0, 0, time.UTC)},
		{name: "unparsable blob", attrs: `not json`, wantErr: true},
		{name: "not a string", attrs: `{"validUntil":12}`, wantErr: true},
		{name: "not RFC 3339", attrs: `{"validUntil":"2027-03-31"}`, wantErr: true},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			got, bounded, err := ValidUntilOf(json.RawMessage(tc.attrs))
			if tc.wantErr {
				s.Require().Error(err)
				s.False(bounded, "a corrupt value never reads as a bound to skip")
				return
			}
			s.Require().NoError(err)
			s.Equal(tc.wantBounded, bounded)
			s.True(tc.want.Equal(got))
		})
	}
}
