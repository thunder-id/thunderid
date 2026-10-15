// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package httpauth

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/outboundauth"
)

type HTTPAuthTestSuite struct {
	suite.Suite
}

func TestHTTPAuthTestSuite(t *testing.T) {
	suite.Run(t, new(HTTPAuthTestSuite))
}

func (s *HTTPAuthTestSuite) TestSupportedTypesFollowTheBindings() {
	s.Equal([]outboundauth.Type{outboundauth.TypeNone, outboundauth.TypeBasic}, SupportedTypes())
}

func (s *HTTPAuthTestSuite) TestBasicSetsTheAuthorizationHeader() {
	authenticator, err := New(outboundauth.Config{
		Type: outboundauth.TypeBasic,
		Properties: map[string]string{
			outboundauth.FieldBasicUsername: "svc-extension",
			outboundauth.FieldBasicPassword: "s3cret:with-colon",
		},
	})
	s.Require().NoError(err)

	header := http.Header{}
	s.Require().NoError(authenticator.Authenticate(context.Background(), header))

	request := &http.Request{Header: header}
	username, password, ok := request.BasicAuth()
	s.True(ok)
	s.Equal("svc-extension", username)
	s.Equal("s3cret:with-colon", password)
}

func (s *HTTPAuthTestSuite) TestBasicReplacesAnAuthorizationHeaderAlreadySet() {
	authenticator, err := New(outboundauth.Config{
		Type: outboundauth.TypeBasic,
		Properties: map[string]string{
			outboundauth.FieldBasicUsername: "svc", outboundauth.FieldBasicPassword: "pw",
		},
	})
	s.Require().NoError(err)

	header := http.Header{}
	header.Add("Authorization", "Bearer stale")
	s.Require().NoError(authenticator.Authenticate(context.Background(), header))

	s.Len(header.Values("Authorization"), 1)
	s.Equal("Basic c3ZjOnB3", header.Get("Authorization"))
}

func (s *HTTPAuthTestSuite) TestNoneAndEmptyTypeSendNothing() {
	for _, authConfig := range []outboundauth.Config{{Type: outboundauth.TypeNone}, {}} {
		authenticator, err := New(authConfig)
		s.Require().NoError(err)

		header := http.Header{"X-Tenant": {"acme"}}
		s.Require().NoError(authenticator.Authenticate(context.Background(), header))
		s.Equal(http.Header{"X-Tenant": {"acme"}}, header)
	}
}

func (s *HTTPAuthTestSuite) TestUnsupportedTypeIsRefused() {
	authenticator, err := New(outboundauth.Config{Type: "bearer"})
	s.Nil(authenticator)
	s.Error(err)
}
