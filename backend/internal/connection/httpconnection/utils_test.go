// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package httpconnection

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type UtilsTestSuite struct {
	suite.Suite
}

func TestUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(UtilsTestSuite))
}

func (s *UtilsTestSuite) TestSameOrigin() {
	cases := []struct {
		name string
		a, b string
		want bool
	}{
		{"same", "https://a.example.com/x", "https://a.example.com/y?q=1", true},
		{"default https port", "https://a.example.com/x", "https://a.example.com:443/x", true},
		{"default http port", "http://a.example.com/x", "http://a.example.com:80/x", true},
		{"host case", "https://A.example.com/x", "https://a.EXAMPLE.com/x", true},
		{"scheme case", "HTTPS://a.example.com/x", "https://a.example.com/x", true},
		{"ipv6 default port", "https://[::1]/x", "https://[::1]:443/x", true},
		{"leading zero port", "https://a.example.com:0443/x", "https://a.example.com/x", true},
		{"host", "https://a.example.com/x", "https://b.example.com/x", false},
		{"subdomain", "https://a.example.com/x", "https://a.example.com.evil.net/x", false},
		{"port", "https://a.example.com/x", "https://a.example.com:8443/x", false},
		{"scheme", "http://a.example.com/x", "https://a.example.com/x", false},
		{"scheme with explicit ports", "http://a.example.com:443/x", "https://a.example.com:443/x", false},
		{"unparsable", "https://a.example.com/x", "://", false},
		{"both unparsable", "://", "://", false},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.Equal(tc.want, SameOrigin(tc.a, tc.b))
		})
	}
}
