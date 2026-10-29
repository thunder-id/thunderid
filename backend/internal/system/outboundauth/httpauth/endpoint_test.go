// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package httpauth

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type EndpointTestSuite struct {
	suite.Suite
}

func TestEndpointTestSuite(t *testing.T) {
	suite.Run(t, new(EndpointTestSuite))
}

func (s *EndpointTestSuite) TestValidHost() {
	for _, host := range []string{"risk.example.com", "localhost", "risk-engine", "my_service", "10.0.0.1",
		"::1", "risk.example.com."} {
		s.True(isValidHost(host), host)
	}
	for _, host := range []string{"", "risk..example.com", "-risk.com", "risk-.com", "risk example.com",
		"rïsk.com", "fe80::1%en0", ".risk.com"} {
		s.False(isValidHost(host), host)
	}
}

func (s *EndpointTestSuite) TestParseEndpointURL() {
	for _, raw := range []string{"https://risk.example.com/process", "http://localhost:8080", "https://[::1]:443"} {
		_, ok := ParseEndpointURL(raw)
		s.True(ok, raw)
	}
	for _, raw := range []string{"ftp://risk.example.com", "https://user:pw@risk.example.com", "https://risk.com:0",
		"https://risk.com:70000", "https://risk.com:port", "https://", "/relative", "https://bad host"} {
		_, ok := ParseEndpointURL(raw)
		s.False(ok, raw)
	}
}

func (s *EndpointTestSuite) TestIsLoopbackHost() {
	for _, host := range []string{"localhost", "LocalHost", "127.0.0.1", "127.1.2.3", "::1"} {
		s.True(IsLoopbackHost(host), host)
	}
	for _, host := range []string{"example.com", "10.0.0.1", ""} {
		s.False(IsLoopbackHost(host), host)
	}
}
