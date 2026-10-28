// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package clientip

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ClientIPTestSuite struct {
	suite.Suite
}

func TestClientIPTestSuite(t *testing.T) {
	suite.Run(t, new(ClientIPTestSuite))
}

// request builds a request from remoteAddr carrying each header line in headers.
func request(remoteAddr string, headers map[string][]string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/flow/execute", nil)
	r.RemoteAddr = remoteAddr
	for name, values := range headers {
		for _, v := range values {
			r.Header.Add(name, v)
		}
	}
	return r
}

func (s *ClientIPTestSuite) newResolver(proxies ...TrustedProxy) *Resolver {
	r, err := NewResolver(proxies, nil)
	s.Require().NoError(err)
	return r
}

func (s *ClientIPTestSuite) TestContextRoundTrip() {
	s.Equal("203.0.113.10", FromContext(WithIP(context.Background(), "203.0.113.10")))
	s.Empty(FromContext(context.Background()))
}

func (s *ClientIPTestSuite) TestResolve_NoTrustedProxies() {
	r := s.newResolver()
	tests := []struct {
		name       string
		remoteAddr string
		want       string
	}{
		{"IPv4 with port", "203.0.113.10:54321", "203.0.113.10"},
		{"IPv6 with port", "[2001:db8::1]:443", "2001:db8::1"},
		{"IPv4 without port", "203.0.113.10", "203.0.113.10"},
		{"IPv6 without port", "[2001:db8::1]", "2001:db8::1"},
		{"IPv4-mapped IPv6 becomes IPv4", "[::ffff:203.0.113.10]:443", "203.0.113.10"},
		{"IPv6 zone is dropped", "[fe80::1%eth0]:443", "fe80::1"},
		{"not an IP address", "not-an-ip", ""},
		{"no address", "", ""},
	}
	for _, tc := range tests {
		s.Run(tc.name, func() {
			req := request(tc.remoteAddr, map[string][]string{"X-Forwarded-For": {"198.51.100.7"}})
			s.Equal(tc.want, r.Resolve(req), "headers are never read without trusted proxies")
		})
	}
}

func (s *ClientIPTestSuite) TestResolve_ForwardedFor() {
	r := s.newResolver(
		TrustedProxy{CIDR: "10.0.0.0/24"},
		TrustedProxy{CIDR: "172.16.0.0/12"},
	)
	tests := []struct {
		name       string
		remoteAddr string
		xff        []string
		want       string
	}{
		{"untrusted caller ignores the header", "198.51.100.7:443", []string{"203.0.113.10"}, "198.51.100.7"},
		{"trusted proxy reports the client", "10.0.0.5:443", []string{"203.0.113.10"}, "203.0.113.10"},
		{"IPv4-mapped trusted proxy matches its IPv4 range", "[::ffff:172.17.0.2]:41234",
			[]string{"203.0.113.10"}, "203.0.113.10"},
		{"trusted hops are skipped from the right", "10.0.0.5:443",
			[]string{"203.0.113.10, 172.20.0.3, 10.0.0.9"}, "203.0.113.10"},
		{"an address left of the client is never read", "10.0.0.5:443",
			[]string{"1.2.3.4, 203.0.113.10"}, "203.0.113.10"},
		{"garbage left of the client is never read", "10.0.0.5:443",
			[]string{"garbage, 203.0.113.10"}, "203.0.113.10"},
		{"header lines are joined", "10.0.0.5:443", []string{"203.0.113.10", "10.0.0.9"}, "203.0.113.10"},
		{"an address with a port is normalised", "10.0.0.5:443", []string{"[2001:db8::1]:443"}, "2001:db8::1"},
		{"all trusted uses the leftmost", "10.0.0.5:443", []string{"172.20.0.3, 10.0.0.9"}, "172.20.0.3"},
		{"an invalid address before the client rejects the header", "10.0.0.5:443",
			[]string{"203.0.113.10, garbage"}, "10.0.0.5"},
		{"an empty element before the client rejects the header", "10.0.0.5:443",
			[]string{"203.0.113.10,"}, "10.0.0.5"},
		{"a missing header falls back", "10.0.0.5:443", nil, "10.0.0.5"},
	}
	for _, tc := range tests {
		s.Run(tc.name, func() {
			req := request(tc.remoteAddr, map[string][]string{"X-Forwarded-For": tc.xff})
			s.Equal(tc.want, r.Resolve(req))
		})
	}
}

func (s *ClientIPTestSuite) TestResolve_SingleValueHeader() {
	r := s.newResolver(TrustedProxy{CIDR: "10.0.0.0/24", Header: "x-real-ip"})
	tests := []struct {
		name    string
		headers map[string][]string
		want    string
	}{
		{"reads the header", map[string][]string{"X-Real-IP": {"203.0.113.10"}}, "203.0.113.10"},
		{"normalises the value", map[string][]string{"X-Real-IP": {" ::ffff:203.0.113.10 "}}, "203.0.113.10"},
		{"X-Forwarded-For is not read", map[string][]string{"X-Forwarded-For": {"203.0.113.10"}}, "10.0.0.5"},
		{"a list is invalid", map[string][]string{"X-Real-IP": {"1.2.3.4, 203.0.113.10"}}, "10.0.0.5"},
		{"a repeated header is invalid", map[string][]string{"X-Real-IP": {"1.2.3.4", "203.0.113.10"}}, "10.0.0.5"},
		{"a missing header falls back", nil, "10.0.0.5"},
	}
	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.Equal(tc.want, r.Resolve(request("10.0.0.5:443", tc.headers)))
		})
	}
}

func (s *ClientIPTestSuite) TestResolve_FirstMatchingProxyDecidesTheHeader() {
	r := s.newResolver(
		TrustedProxy{CIDR: "10.0.1.0/24", Header: "x-real-ip"},
		TrustedProxy{CIDR: "10.0.0.0/16"},
	)
	headers := map[string][]string{"X-Real-IP": {"203.0.113.10"}, "X-Forwarded-For": {"198.51.100.7"}}

	s.Equal("203.0.113.10", r.Resolve(request("10.0.1.5:443", headers)))
	s.Equal("198.51.100.7", r.Resolve(request("10.0.2.5:443", headers)))
}

func (s *ClientIPTestSuite) TestResolve_ReportsFallbackOncePerProxy() {
	var reported []string
	r, err := NewResolver([]TrustedProxy{
		{CIDR: "10.0.1.0/24", Header: "x-real-ip"},
		{CIDR: "10.0.2.0/24"},
	}, func(_ context.Context, cidr netip.Prefix, header string) {
		reported = append(reported, cidr.String()+" "+header)
	})
	s.Require().NoError(err)

	for range 3 {
		s.Equal("10.0.1.5", r.Resolve(request("10.0.1.5:443", nil)))
		s.Equal("10.0.2.5", r.Resolve(request("10.0.2.5:443", nil)))
	}
	s.Equal("203.0.113.10", r.Resolve(request("10.0.1.5:443",
		map[string][]string{"X-Real-IP": {"203.0.113.10"}})))

	s.Equal([]string{"10.0.1.0/24 X-Real-Ip", "10.0.2.0/24 X-Forwarded-For"}, reported)
}

func (s *ClientIPTestSuite) TestNewResolver_InvalidCIDR() {
	_, err := NewResolver([]TrustedProxy{{CIDR: "10.0.0.5"}}, nil)
	s.Error(err)
}

func (s *ClientIPTestSuite) TestResolve_IPv4MappedTrustedProxy() {
	r := s.newResolver(TrustedProxy{CIDR: "::ffff:10.0.1.0/120"})
	headers := map[string][]string{"X-Forwarded-For": {"203.0.113.10"}}

	s.Equal("203.0.113.10", r.Resolve(request("10.0.1.5:443", headers)))
	s.Equal("203.0.113.10", r.Resolve(request("[::ffff:10.0.1.5]:443", headers)))
	s.Equal("10.0.2.5", r.Resolve(request("10.0.2.5:443", headers)))
}

func (s *ClientIPTestSuite) TestNewResolver_IPv4MappedRangeShorterThan96() {
	_, err := NewResolver([]TrustedProxy{{CIDR: "::ffff:0.0.0.0/95"}}, nil)
	s.ErrorContains(err, "shorter than /96")
}
