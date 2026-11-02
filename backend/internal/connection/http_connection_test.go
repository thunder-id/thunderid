// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/connection/httpconnection"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	"github.com/thunder-id/thunderid/internal/system/outboundauth/httpauth"
)

type HTTPConnectionMappingTestSuite struct {
	suite.Suite
}

func TestHTTPConnectionMappingSuite(t *testing.T) {
	suite.Run(t, new(HTTPConnectionMappingTestSuite))
}

func (s *HTTPConnectionMappingTestSuite) SetupTest() {
	initConfigWithTestCryptoKey(s.T())
	s.T().Cleanup(config.ResetServerRuntime)
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

func (s *HTTPConnectionMappingTestSuite) TestBearerAndAPIKeyReadWithSecretsMasked() {
	cases := map[string]struct {
		auth *outboundauth.Authentication
		want map[string]string
	}{
		"bearer": {
			auth: &outboundauth.Authentication{Type: string(outboundauth.TypeBearer),
				Properties: map[string]string{outboundauth.FieldBearerToken: "bearer-secret"}},
			want: map[string]string{outboundauth.FieldBearerToken: maskedSecretValue},
		},
		"api_key": {
			auth: &outboundauth.Authentication{Type: string(outboundauth.TypeAPIKey),
				Properties: map[string]string{"x-tenant-key": "api-key-secret", "X-Region-Key": "region-secret"}},
			want: map[string]string{"X-Tenant-Key": maskedSecretValue, "X-Region-Key": maskedSecretValue},
		},
	}
	for name, tc := range cases {
		s.Run(name, func() {
			dto, err := httpToDTO(httpConnectionRequest{Name: "Risk", URL: "https://risk.example.com",
				Authentication: tc.auth})
			s.Require().NoError(err)
			for _, property := range dto.AuthProperties {
				if property.GetName() != outboundauth.PropertyKeyType {
					s.True(property.IsSecret(), "%s must be stored as a secret", property.GetName())
				}
			}

			response, err := httpFromDTO(*dto)
			s.Require().NoError(err)
			s.Equal(name, response.Authentication.Type)
			s.Equal(tc.want, response.Authentication.Properties)
		})
	}
}

// The mask is what every read returns, so a client that sends a read back unchanged must not have it
// stored as the password or token, nor have it read as "keep the stored one" (AC2.11).
func (s *HTTPConnectionMappingTestSuite) TestMaskedPasswordOrTokenIsRefused() {
	for name, auth := range map[string]*outboundauth.Authentication{
		"basic": basicAuthRequest("svc", maskedSecretValue),
		"bearer": {Type: string(outboundauth.TypeBearer),
			Properties: map[string]string{outboundauth.FieldBearerToken: maskedSecretValue}},
	} {
		for operation, mapRequest := range map[string]func(httpConnectionRequest) (
			*httpconnection.HTTPConnection, error){"create": httpToDTO, "update": httpUpdateToDTO} {
			s.Run(name+" "+operation, func() {
				_, err := mapRequest(httpConnectionRequest{Name: "Risk", URL: "https://risk.example.com",
					Authentication: auth})
				s.Require().ErrorIs(err, errInvalidHTTPAuthentication)
				s.Equal(httpconnection.ErrorInvalidAuthentication.Code, senderMappingServiceError(err).Code)
			})
		}
	}
}

// An API-key header value sent as the mask keeps that header's stored value on update, as the shared
// HTTP binding defines; on create there is nothing to keep.
func (s *HTTPConnectionMappingTestSuite) TestMaskedAPIKeyValueIsAcceptedOnlyOnUpdate() {
	request := httpConnectionRequest{Name: "Risk", URL: "https://risk.example.com",
		Authentication: &outboundauth.Authentication{Type: string(outboundauth.TypeAPIKey),
			Properties: map[string]string{"X-Tenant-Key": maskedSecretValue}}}

	_, err := httpToDTO(request)
	s.ErrorIs(err, errInvalidHTTPAuthentication)

	_, err = httpUpdateToDTO(request)
	s.NoError(err)
}

func (s *HTTPConnectionMappingTestSuite) TestEveryHTTPBindingMethodIsAccepted() {
	for _, authType := range httpauth.SupportedTypes() {
		s.Run(string(authType), func() {
			_, err := httpUpdateToDTO(httpConnectionRequest{Name: "Risk", URL: "https://risk.example.com",
				Authentication: &outboundauth.Authentication{Type: string(authType)}})
			s.NotErrorIs(err, errUnsupportedAuthenticationType)
		})
	}
}

func (s *HTTPConnectionMappingTestSuite) TestOUCarriedThroughMapping() {
	dto, err := httpRequestToDTO(httpConnectionRequest{
		Name: "n", OUID: "ou-1", URL: "https://risk.example.com",
		Authentication: &outboundauth.Authentication{Type: string(outboundauth.TypeNone)},
	}, false)
	s.Require().NoError(err)
	s.Equal("ou-1", dto.OUID)

	resp, err := httpFromDTO(*dto)
	s.Require().NoError(err)
	s.Equal("ou-1", resp.OUID)
}
