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

// An address without a port, as a test or a unix socket gives, is recorded as it is.
func (s *ClientMiddlewareTestSuite) TestThePeerAddressIsRecordedWithoutItsPort() {
	for remoteAddr, want := range map[string]string{
		"192.0.2.10:52814":    "192.0.2.10",
		"[2001:db8::1]:52814": "2001:db8::1",
		"192.0.2.10":          "192.0.2.10",
		"2001:db8::1":         "2001:db8::1",
		"@":                   "@",
		"":                    "",
	} {
		s.Run(remoteAddr, func() {
			r := httptest.NewRequest(http.MethodGet, "/users", nil)
			r.RemoteAddr = remoteAddr

			address, _ := s.serve(r)

			s.Equal(want, address)
		})
	}
}

// The client is the TCP peer: a forwarded header is chosen by the caller and is never read.
func (s *ClientMiddlewareTestSuite) TestForwardedHeadersAreIgnored() {
	r := httptest.NewRequest(http.MethodGet, "/users", nil)
	r.RemoteAddr = "192.0.2.10:52814"
	r.Header.Set("X-Forwarded-For", "203.0.113.7")
	r.Header.Set("X-Real-IP", "203.0.113.7")
	r.Header.Set("Forwarded", "for=203.0.113.7")

	address, _ := s.serve(r)

	s.Equal("192.0.2.10", address)
}

func (s *ClientMiddlewareTestSuite) TestTheUserAgentIsRecorded() {
	r := httptest.NewRequest(http.MethodGet, "/users", nil)
	r.Header.Set("User-Agent", "audit-test/1.0")

	_, userAgent := s.serve(r)

	s.Equal("audit-test/1.0", userAgent)
}

func (s *ClientMiddlewareTestSuite) TestTheUserAgentIsCutTo256Characters() {
	r := httptest.NewRequest(http.MethodGet, "/users", nil)
	r.Header.Set("User-Agent", strings.Repeat("ü", maxUserAgentLength+50))

	_, userAgent := s.serve(r)

	s.Equal(strings.Repeat("ü", maxUserAgentLength), userAgent)
}

func (s *ClientMiddlewareTestSuite) TestAMissingUserAgentIsEmpty() {
	r := httptest.NewRequest(http.MethodGet, "/users", nil)
	r.Header.Del("User-Agent")

	_, userAgent := s.serve(r)

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
