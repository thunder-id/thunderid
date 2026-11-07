// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	syscontext "github.com/thunder-id/thunderid/internal/system/context"
)

// publicPeer is a peer address no special-purpose range covers, which the middleware records.
const publicPeer = "8.8.8.8:52814"

type contextProbe struct{}

type ClientMiddlewareTestSuite struct {
	suite.Suite
}

func TestClientMiddlewareTestSuite(t *testing.T) {
	suite.Run(t, new(ClientMiddlewareTestSuite))
}

// serve runs r through the middleware and returns the client the handler saw.
func (s *ClientMiddlewareTestSuite) serve(r *http.Request) (address, userAgent string) {
	reached := false
	ClientMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		reached = true
		address, userAgent = syscontext.GetClient(r.Context())
	})).ServeHTTP(httptest.NewRecorder(), r)
	s.Require().True(reached, "the request is passed on")
	return address, userAgent
}

// A public peer is recorded without its port, an IPv4 address mapped into IPv6 as IPv4.
func (s *ClientMiddlewareTestSuite) TestAPublicPeerIsRecordedWithoutItsPort() {
	for remoteAddr, want := range map[string]string{
		"8.8.8.8:52814":                "8.8.8.8",
		"[2001:4860:4860::8888]:52814": "2001:4860:4860::8888",
		"1.1.1.1:443":                  "1.1.1.1",
		"[::ffff:8.8.4.4]:5000":        "8.8.4.4",
		"8.8.8.8":                      "8.8.8.8",
		"2001:4860:4860::8888":         "2001:4860:4860::8888",
		"198.17.255.255:52814":         "198.17.255.255",
		"198.20.0.0:52814":             "198.20.0.0",
	} {
		s.Run(remoteAddr, func() {
			r := httptest.NewRequest(http.MethodGet, "/users", nil)
			r.RemoteAddr = remoteAddr
			r.Header.Set("User-Agent", "audit-test/1.0")

			address, userAgent := s.serve(r)

			s.Equal(want, address)
			s.Equal("audit-test/1.0", userAgent)
		})
	}
}

// Behind a proxy, an ingress or a load balancer, and in a local run, the peer is a private or loopback
// address that names no client, so neither the address nor the user agent is recorded.
func (s *ClientMiddlewareTestSuite) TestANonPublicPeerIsNotRecorded() {
	for _, remoteAddr := range []string{
		"127.0.0.1:52814",
		"[::1]:52814",
		"10.1.2.3:52814",
		"172.16.0.1:52814",
		"172.31.255.254:52814",
		"192.168.1.20:52814",
		"169.254.169.254:80",
		"[fe80::1]:52814",
		"[fc00::1]:52814",
		"[fd12:3456::1]:52814",
		"100.64.0.1:52814",
		"100.127.255.254:52814",
		"[::ffff:10.0.0.1]:52814",
		"224.0.0.1:52814",
		"[ff02::1]:52814",
		"0.0.0.0:52814",
		"[::]:52814",
		"255.255.255.255:52814",
		"0.1.2.3:52814",
		"192.0.0.8:52814",
		"192.0.2.10:52814",
		"198.51.100.4:52814",
		"203.0.113.7:52814",
		"192.88.99.1:52814",
		"198.18.0.1:52814",
		"198.19.255.254:52814",
		"240.0.0.1:52814",
		"[100::1]:52814",
		"[2001:db8::1]:52814",
		"[3fff::1]:52814",
		"[::ffff:192.0.2.10]:52814",
		"not-an-address:52814",
		"10.0.0.1",
		"@",
		"",
	} {
		s.Run(remoteAddr, func() {
			r := httptest.NewRequest(http.MethodGet, "/users", nil)
			r.RemoteAddr = remoteAddr
			r.Header.Set("User-Agent", "audit-test/1.0")

			address, userAgent := s.serve(r)

			s.Empty(address)
			s.Empty(userAgent, "the user agent is recorded only with a public address")
		})
	}
}

// The client is the TCP peer: a forwarded header is chosen by the caller and is never read, neither to
// replace a public peer nor to make a private one public.
func (s *ClientMiddlewareTestSuite) TestForwardedHeadersAreIgnored() {
	for remoteAddr, want := range map[string]string{
		"8.8.8.8:52814":   "8.8.8.8",
		"127.0.0.1:52814": "",
		"10.0.0.5:52814":  "",
	} {
		s.Run(remoteAddr, func() {
			r := httptest.NewRequest(http.MethodGet, "/users", nil)
			r.RemoteAddr = remoteAddr
			r.Header.Set("X-Forwarded-For", "203.0.113.7")
			r.Header.Set("X-Real-IP", "203.0.113.7")
			r.Header.Set("Forwarded", "for=203.0.113.7")

			address, _ := s.serve(r)

			s.Equal(want, address)
		})
	}
}

func (s *ClientMiddlewareTestSuite) TestTheUserAgentIsRecorded() {
	r := httptest.NewRequest(http.MethodGet, "/users", nil)
	r.RemoteAddr = publicPeer
	r.Header.Set("User-Agent", "audit-test/1.0")

	_, userAgent := s.serve(r)

	s.Equal("audit-test/1.0", userAgent)
}

// The cap counts characters, so a multibyte user agent is cut on a character boundary.
func (s *ClientMiddlewareTestSuite) TestTheUserAgentIsCutTo256Characters() {
	for _, tc := range []struct{ name, userAgent, want string }{
		{"multibyte", strings.Repeat("ü", 2*maxUserAgentLength), strings.Repeat("ü", maxUserAgentLength)},
		{"mixed", strings.Repeat("aü", maxUserAgentLength), strings.Repeat("aü", maxUserAgentLength/2)},
		{"multibyte at the cap", strings.Repeat("ü", maxUserAgentLength), strings.Repeat("ü", maxUserAgentLength)},
	} {
		s.Run(tc.name, func() {
			r := httptest.NewRequest(http.MethodGet, "/users", nil)
			r.RemoteAddr = publicPeer
			r.Header.Set("User-Agent", tc.userAgent)

			_, userAgent := s.serve(r)

			s.Equal(tc.want, userAgent)
		})
	}
}

func (s *ClientMiddlewareTestSuite) TestAMissingUserAgentIsEmpty() {
	r := httptest.NewRequest(http.MethodGet, "/users", nil)
	r.RemoteAddr = publicPeer
	r.Header.Del("User-Agent")

	address, userAgent := s.serve(r)

	s.Equal("8.8.8.8", address)
	s.Empty(userAgent)
}

// Only the client is added: the handler gets the same method, path, headers and context values.
func (s *ClientMiddlewareTestSuite) TestTheRequestIsPassedOnUnchanged() {
	r := httptest.NewRequest(http.MethodPost, "/users?x=1", strings.NewReader("body"))
	r.Header.Set("X-Custom", "kept")
	r = r.WithContext(context.WithValue(r.Context(), contextProbe{}, "kept"))

	var seen *http.Request
	recorder := httptest.NewRecorder()
	ClientMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r
		w.WriteHeader(http.StatusTeapot)
	})).ServeHTTP(recorder, r)

	s.Require().NotNil(seen)
	s.Equal(http.StatusTeapot, recorder.Code)
	s.Equal(http.MethodPost, seen.Method)
	s.Equal("/users?x=1", seen.URL.String())
	s.Equal("kept", seen.Header.Get("X-Custom"))
	s.Equal(r.Body, seen.Body)
	s.Equal("kept", seen.Context().Value(contextProbe{}))
}
