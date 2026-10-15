// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/connection/httpconnection"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
)

type HTTPConnectionMappingTestSuite struct {
	suite.Suite
}

func TestHTTPConnectionMappingSuite(t *testing.T) {
	suite.Run(t, new(HTTPConnectionMappingTestSuite))
}

func (s *HTTPConnectionMappingTestSuite) TestRequestRoundTripsWithPasswordMasked() {
	headers := []httpconnection.Header{{Name: "Accept", Value: "application/json, text/plain"}}
	dto, err := httpToDTO(httpConnectionRequest{
		Name:           "Risk Engine",
		URL:            "https://risk.example.com/evaluate",
		TimeoutMS:      5000,
		Headers:        headers,
		Authentication: basicAuthRequest("svc", "s3cret"),
	})
	s.Require().NoError(err)

	response, err := httpFromDTO(*dto)
	s.Require().NoError(err)
	s.Equal(httpVendorName, response.Type)
	s.Equal(headers, response.Headers)
	s.Equal(string(outboundauth.TypeBasic), response.Authentication.Type)
	s.Equal("svc", response.Authentication.Properties[outboundauth.FieldBasicUsername])
	s.Equal(maskedSecretValue, response.Authentication.Properties[outboundauth.FieldBasicPassword])
}

func (s *HTTPConnectionMappingTestSuite) TestMissingAuthenticationAndHeadersReadAsNoneAndEmpty() {
	dto, err := httpToDTO(httpConnectionRequest{Name: "Risk Engine", URL: "https://risk.example.com"})
	s.Require().NoError(err)

	response, err := httpFromDTO(*dto)
	s.Require().NoError(err)
	s.Equal(string(outboundauth.TypeNone), response.Authentication.Type)
	s.NotNil(response.Headers)
	s.Empty(response.Headers)
}

func (s *HTTPConnectionMappingTestSuite) TestUnknownAuthenticationTypeIsRejected() {
	_, err := httpToDTO(httpConnectionRequest{
		Name:           "Risk Engine",
		URL:            "https://risk.example.com",
		Authentication: &outboundauth.Authentication{Type: "kerberos"},
	})
	s.ErrorIs(err, errUnsupportedAuthenticationType)
}
