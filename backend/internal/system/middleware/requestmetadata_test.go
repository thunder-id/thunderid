// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"

	sysContext "github.com/thunder-id/thunderid/internal/system/context"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

type RequestMetadataMiddlewareTestSuite struct {
	suite.Suite
}

func TestRequestMetadataMiddlewareTestSuite(t *testing.T) {
	suite.Run(t, new(RequestMetadataMiddlewareTestSuite))
}

// metadata runs a request from remoteAddr with the given X-Forwarded-For through the middleware and
// returns the request metadata the handler sees on its context.
func (s *RequestMetadataMiddlewareTestSuite) metadata(proxies []engineconfig.TrustedProxyConfig, remoteAddr,
	forwardedFor string) sysContext.RequestMetadata {
	mw, err := RequestMetadataMiddleware(proxies)
	s.Require().NoError(err)

	var got sysContext.RequestMetadata
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = sysContext.GetRequestMetadata(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = remoteAddr
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh) Chrome/140.0")
	if forwardedFor != "" {
		req.Header.Set("X-Forwarded-For", forwardedFor)
	}
	handler.ServeHTTP(httptest.NewRecorder(), req)
	return got
}

func (s *RequestMetadataMiddlewareTestSuite) TestPutsMetadataOnContext() {
	got := s.metadata([]engineconfig.TrustedProxyConfig{{CIDR: "10.0.0.0/24"}}, "10.0.0.5:443", "203.0.113.10")
	s.Equal(sysContext.RequestMetadata{
		ClientIP:  "203.0.113.10",
		UserAgent: "Mozilla/5.0 (Macintosh) Chrome/140.0",
	}, got)
}

func (s *RequestMetadataMiddlewareTestSuite) TestSetsConnectingAddressWithoutProxies() {
	s.Equal("198.51.100.7", s.metadata(nil, "198.51.100.7:443", "203.0.113.10").ClientIP)
}

func (s *RequestMetadataMiddlewareTestSuite) TestFallsBackToConnectingAddressWhenHeaderMissing() {
	got := s.metadata([]engineconfig.TrustedProxyConfig{{CIDR: "10.0.0.0/24"}}, "10.0.0.5:443", "")
	s.Equal("10.0.0.5", got.ClientIP)
}

func (s *RequestMetadataMiddlewareTestSuite) TestInvalidConfig() {
	_, err := RequestMetadataMiddleware([]engineconfig.TrustedProxyConfig{{CIDR: "not-a-cidr"}})
	s.Error(err)
}
