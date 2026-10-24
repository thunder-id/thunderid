// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	ncommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/idp/idpmock"
	"github.com/thunder-id/thunderid/tests/mocks/notification/notificationmock"
)

type SMSGatewayTestSuite struct {
	suite.Suite
	handler   *handler
	mockIDP   *idpmock.IDPServiceInterfaceMock
	mockNotif *notificationmock.NotificationSenderMgtSvcInterfaceMock
}

const legacyHTTPHeadersProperty = "http_headers"

func apiKeyAuthentication(headers map[string]string) *outboundauth.Authentication {
	return &outboundauth.Authentication{Type: string(outboundauth.TypeAPIKey), Properties: headers}
}

func TestSMSGatewaySuite(t *testing.T) {
	suite.Run(t, new(SMSGatewayTestSuite))
}

func (s *SMSGatewayTestSuite) SetupTest() {
	s.handler, s.mockIDP, s.mockNotif = newConnectionTestHandler(s.T())
}

func (s *SMSGatewayTestSuite) TestToSenderDTOMapsFields() {
	dto, err := smsGatewayToSenderDTO(smsGatewayConnectionRequest{
		Name: "Prod SMS", Description: "Custom webhook sender",
		URL: "https://sms.example.com/send", HTTPMethod: "POST",
		Authentication: apiKeyAuthentication(map[string]string{
			"X-API-Key": "secret",
			"X-Tenant":  "tenant-1",
		}),
		ContentType: "JSON",
	})
	s.Require().NoError(err)
	s.Equal(ncommon.NotificationSenderTypeMessage, dto.Type)
	s.Equal(ncommon.NotificationProviderTypeCustom, dto.Provider)
	s.Equal("Custom webhook sender", dto.Description)

	values, err := propertyValues(dto.Properties)
	s.Require().NoError(err)
	s.Equal("https://sms.example.com/send", values[ncommon.CustomPropKeyURL])
	s.Equal("POST", values[ncommon.CustomPropKeyHTTPMethod])
	s.Equal(maskedSecretValue, values["authentication_X-Api-Key"])
	s.Equal(maskedSecretValue, values["authentication_X-Tenant"])
	s.Equal("JSON", values[ncommon.CustomPropKeyContentType])
}

func (s *SMSGatewayTestSuite) TestToSenderDTOOmitsEmptyOptionalFields() {
	dto, err := smsGatewayToSenderDTO(smsGatewayConnectionRequest{
		Name: "Minimal", URL: "https://sms.example.com/send",
	})
	s.Require().NoError(err)

	values, err := propertyValues(dto.Properties)
	s.Require().NoError(err)
	s.Equal("https://sms.example.com/send", values[ncommon.CustomPropKeyURL])
	s.NotContains(values, ncommon.CustomPropKeyHTTPMethod)
	s.NotContains(values, "authentication_X-Api-Key")
	s.NotContains(values, ncommon.CustomPropKeyContentType)
}

func (s *SMSGatewayTestSuite) TestToSenderDTORejectsInvalidAPIKeyHeader() {
	_, err := smsGatewayToSenderDTO(smsGatewayConnectionRequest{
		Name: "Prod SMS", URL: "https://sms.example.com/send",
		Authentication: apiKeyAuthentication(map[string]string{"Content-Type": "application/json"}),
	})

	s.Require().Error(err)
}

func (s *SMSGatewayTestSuite) TestSupportsHTTPAuthenticationMethods() {
	cases := []struct {
		name           string
		authentication *outboundauth.Authentication
	}{
		{"none", &outboundauth.Authentication{Type: string(outboundauth.TypeNone)}},
		{"bearer", &outboundauth.Authentication{Type: string(outboundauth.TypeBearer),
			Properties: map[string]string{outboundauth.FieldBearerToken: "token"}}},
		{"basic", &outboundauth.Authentication{Type: string(outboundauth.TypeBasic),
			Properties: map[string]string{outboundauth.FieldBasicUsername: "user",
				outboundauth.FieldBasicPassword: "password"}}},
		{"api_key", apiKeyAuthentication(map[string]string{"X-API-Key": "secret"})},
	}
	for _, test := range cases {
		s.Run(test.name, func() {
			dto, err := smsGatewayToSenderDTO(smsGatewayConnectionRequest{
				URL: "https://sms.example.com/send", Authentication: test.authentication,
			})
			s.Require().NoError(err)
			config, err := outboundauth.FromProperties(dto.Properties)
			s.Require().NoError(err)
			s.Equal(test.authentication.Type, string(config.Type))
		})
	}
}

func (s *SMSGatewayTestSuite) TestAuthenticatedURLRequiresHTTPSUnlessLoopback() {
	for _, authType := range []outboundauth.Type{
		outboundauth.TypeBearer, outboundauth.TypeBasic, outboundauth.TypeAPIKey,
	} {
		s.Run(string(authType), func() {
			properties := map[string]string{}
			switch authType {
			case outboundauth.TypeBearer:
				properties[outboundauth.FieldBearerToken] = "token"
			case outboundauth.TypeBasic:
				properties[outboundauth.FieldBasicUsername] = "user"
				properties[outboundauth.FieldBasicPassword] = "password"
			case outboundauth.TypeAPIKey:
				properties["X-API-Key"] = "secret"
			}
			authentication := &outboundauth.Authentication{Type: string(authType), Properties: properties}
			_, err := smsGatewayToSenderDTO(smsGatewayConnectionRequest{
				URL: "http://sms.example.com/send", Authentication: authentication,
			})
			s.ErrorContains(err, "HTTPS is required")
			_, err = smsGatewayToSenderDTO(smsGatewayConnectionRequest{
				URL: "http://localhost:8080/send", Authentication: authentication,
			})
			s.NoError(err)
		})
	}
	_, err := smsGatewayToSenderDTO(smsGatewayConnectionRequest{URL: "http://sms.example.com/send"})
	s.NoError(err)
}

func (s *SMSGatewayTestSuite) TestCreateRejectsMaskedAPIKeyHeaderValue() {
	_, err := smsGatewayToSenderDTO(smsGatewayConnectionRequest{
		Name: "Prod SMS", URL: "https://sms.example.com/send",
		Authentication: apiKeyAuthentication(map[string]string{"X-API-Key": maskedSecretValue}),
	})

	s.Require().Error(err)
}

func (s *SMSGatewayTestSuite) TestCreateRejectsInvalidAuthenticationWithBadRequest() {
	body, err := json.Marshal(smsGatewayConnectionRequest{
		Name: "Invalid headers", URL: "https://sms.example.com/send", HTTPMethod: "POST",
		Authentication: apiKeyAuthentication(map[string]string{"Content-Type": "application/json"}),
	})
	s.Require().NoError(err)
	request := httptest.NewRequest(http.MethodPost, "/connections/sms-gateway", bytes.NewReader(body))
	response := httptest.NewRecorder()
	createSenderHandler(s.handler, smsGatewayToSenderDTO, smsGatewayFromSenderDTO)(response, request)
	s.Equal(http.StatusBadRequest, response.Code)
	s.mockNotif.AssertNotCalled(s.T(), "CreateSender", mock.Anything, mock.Anything)
}

func (s *SMSGatewayTestSuite) TestUpdateRetainsMaskedHeaderAndDeletesOmittedHeader() {
	existing, err := smsGatewayToSenderDTO(smsGatewayConnectionRequest{
		Name: "Prod SMS", URL: "https://sms.example.com/send",
		Authentication: apiKeyAuthentication(map[string]string{
			"X-First-Key": "first-secret", "X-Second-Key": "second-secret",
		}),
	})
	s.Require().NoError(err)
	incoming, err := smsGatewayUpdateToSenderDTO(smsGatewayConnectionRequest{
		Name: "Prod SMS", URL: "https://sms.example.com/send",
		Authentication: apiKeyAuthentication(map[string]string{
			"X-Second-Key": maskedSecretValue, "X-Third-Key": "third-secret",
		}),
	})
	s.Require().NoError(err)

	merged, err := mergeSMSGatewayProperties(incoming.Properties, existing.Properties)
	s.Require().NoError(err)
	config, err := outboundauth.FromProperties(merged)
	s.Require().NoError(err)
	s.Equal(map[string]string{
		"X-Second-Key": "second-secret", "X-Third-Key": "third-secret",
	}, config.Properties)
}

func (s *SMSGatewayTestSuite) TestUpdateRejectsRetainingHeaderAfterURLChange() {
	existing, err := smsGatewayToSenderDTO(smsGatewayConnectionRequest{
		URL:            "https://sms.example.com/send",
		Authentication: apiKeyAuthentication(map[string]string{"X-API-Key": "secret"}),
	})
	s.Require().NoError(err)
	incoming, err := smsGatewayUpdateToSenderDTO(smsGatewayConnectionRequest{
		URL:            "https://other.example.com/send",
		Authentication: apiKeyAuthentication(map[string]string{"X-API-Key": maskedSecretValue}),
	})
	s.Require().NoError(err)
	_, err = mergeSMSGatewayProperties(incoming.Properties, existing.Properties)
	s.Require().Error(err)
}

func (s *SMSGatewayTestSuite) TestUpdateRejectsOmittedAuthenticationAfterURLChange() {
	existing, err := smsGatewayToSenderDTO(smsGatewayConnectionRequest{
		URL:            "https://sms.example.com/send",
		Authentication: apiKeyAuthentication(map[string]string{"X-API-Key": "secret"}),
	})
	s.Require().NoError(err)
	incoming, err := smsGatewayUpdateToSenderDTO(smsGatewayConnectionRequest{
		URL: "https://other.example.com/send",
	})
	s.Require().NoError(err)
	_, err = mergeSMSGatewayProperties(incoming.Properties, existing.Properties)
	s.Require().Error(err)
}

func (s *SMSGatewayTestSuite) TestFromSenderDTOReturnsLegacyHTTPHeaders() {
	for _, test := range []struct {
		name     string
		isSecret bool
	}{{name: "plaintext"}, {name: "encrypted", isSecret: true}} {
		s.Run(test.name, func() {
			response, err := smsGatewayFromSenderDTO(ncommon.NotificationSenderDTO{
				ID: "sg-1", Name: "Legacy SMS", Provider: ncommon.NotificationProviderTypeCustom,
				Properties: []cmodels.Property{
					mustProperty(s.T(), ncommon.CustomPropKeyURL, "https://sms.example.com/send", false),
					mustProperty(s.T(), legacyHTTPHeadersProperty,
						"X-API-Key: legacy-secret, X-Tenant: tenant-1", test.isSecret),
				},
			})

			s.Require().NoError(err)
			s.Equal(string(outboundauth.TypeNone), response.Authentication.Type)
			s.Equal("X-API-Key: legacy-secret, X-Tenant: tenant-1", response.HTTPHeaders)
		})
	}
}

func (s *SMSGatewayTestSuite) TestGeneralHeadersCoexistWithAuthentication() {
	headers := "X-Campaign: campaign-1"
	dto, err := smsGatewayToSenderDTO(smsGatewayConnectionRequest{
		Name: "SMS", URL: "https://sms.example.com/send", HTTPHeaders: &headers,
		Authentication: apiKeyAuthentication(map[string]string{"X-API-Key": "secret"}),
	})
	s.Require().NoError(err)
	stored, err := smsGatewayHeaderValue(dto.Properties)
	s.Require().NoError(err)
	s.Equal(headers, stored)
	for _, property := range dto.Properties {
		if property.GetName() == ncommon.CustomPropKeyHTTPHeaders {
			s.False(property.IsSecret())
		}
	}
	response, err := smsGatewayFromSenderDTO(*dto)
	s.Require().NoError(err)
	s.Equal(headers, response.HTTPHeaders)
	s.Equal(maskedSecretValue, response.Authentication.Properties["X-Api-Key"])
}

func (s *SMSGatewayTestSuite) TestGeneralHeaderUpdateReplacesString() {
	old := "X-First: one, X-Second: two"
	existing, err := smsGatewayToSenderDTO(smsGatewayConnectionRequest{
		URL: "https://sms.example.com/send", HTTPHeaders: &old,
	})
	s.Require().NoError(err)
	update := "X-Second: two"
	incoming, err := smsGatewayUpdateToSenderDTO(smsGatewayConnectionRequest{
		URL: "https://sms.example.com/send", HTTPHeaders: &update,
	})
	s.Require().NoError(err)
	merged, err := mergeSMSGatewayProperties(incoming.Properties, existing.Properties)
	s.Require().NoError(err)
	stored, err := smsGatewayHeaderValue(merged)
	s.Require().NoError(err)
	s.Equal(update, stored)
}

func (s *SMSGatewayTestSuite) TestGeneralHeaderUpdateEmptyStringRemovesHeaders() {
	old := "X-Campaign: campaign-1"
	existing, err := smsGatewayToSenderDTO(smsGatewayConnectionRequest{
		URL: "https://sms.example.com/send", HTTPHeaders: &old,
	})
	s.Require().NoError(err)
	empty := ""
	incoming, err := smsGatewayUpdateToSenderDTO(smsGatewayConnectionRequest{
		URL: "https://sms.example.com/send", HTTPHeaders: &empty,
	})
	s.Require().NoError(err)
	merged, err := mergeSMSGatewayProperties(incoming.Properties, existing.Properties)
	s.Require().NoError(err)
	value, err := smsGatewayHeaderValue(merged)
	s.Require().NoError(err)
	s.Empty(value)
}

func (s *SMSGatewayTestSuite) TestGeneralHeadersRejectAuthenticationOverlap() {
	headers := "x-api-key: other"
	_, err := smsGatewayToSenderDTO(smsGatewayConnectionRequest{
		URL: "https://sms.example.com/send", HTTPHeaders: &headers,
		Authentication: apiKeyAuthentication(map[string]string{"X-API-Key": "secret"}),
	})
	s.Require().ErrorContains(err, "also used for authentication")
}

func (s *SMSGatewayTestSuite) TestGeneralHeadersRejectAuthorizationOverlap() {
	headers := "authorization: legacy-token"
	for _, authentication := range []*outboundauth.Authentication{
		{Type: string(outboundauth.TypeBearer),
			Properties: map[string]string{outboundauth.FieldBearerToken: "token"}},
		{Type: string(outboundauth.TypeBasic),
			Properties: map[string]string{outboundauth.FieldBasicUsername: "user",
				outboundauth.FieldBasicPassword: "password"}},
	} {
		_, err := smsGatewayToSenderDTO(smsGatewayConnectionRequest{
			URL: "https://sms.example.com/send", HTTPHeaders: &headers, Authentication: authentication,
		})
		s.ErrorContains(err, "also used for authentication")
	}
}

func (s *SMSGatewayTestSuite) TestUpdateRejectsAuthorizationOverlapWithStoredGeneralHeaders() {
	headers := "Authorization: legacy-token"
	existing, err := smsGatewayToSenderDTO(smsGatewayConnectionRequest{
		URL: "https://sms.example.com/send", HTTPHeaders: &headers,
	})
	s.Require().NoError(err)
	incoming, err := smsGatewayUpdateToSenderDTO(smsGatewayConnectionRequest{
		URL: "https://sms.example.com/send",
		Authentication: &outboundauth.Authentication{Type: string(outboundauth.TypeBearer),
			Properties: map[string]string{outboundauth.FieldBearerToken: "token"}},
	})
	s.Require().NoError(err)
	_, err = mergeSMSGatewayProperties(incoming.Properties, existing.Properties)
	s.ErrorContains(err, "also used for authentication")
}

func (s *SMSGatewayTestSuite) TestUpdateValidatesAuthenticationTransport() {
	_, err := smsGatewayUpdateToSenderDTO(smsGatewayConnectionRequest{
		URL: "http://sms.example.com/send",
		Authentication: &outboundauth.Authentication{Type: string(outboundauth.TypeBearer),
			Properties: map[string]string{outboundauth.FieldBearerToken: "new-token"}},
	})
	s.ErrorContains(err, "HTTPS is required")
}

func (s *SMSGatewayTestSuite) TestGeneralHeadersRemainAfterURLChange() {
	headers := "X-Campaign: old"
	existing, err := smsGatewayToSenderDTO(smsGatewayConnectionRequest{
		URL: "https://sms.example.com/send", HTTPHeaders: &headers,
	})
	s.Require().NoError(err)
	incoming, err := smsGatewayUpdateToSenderDTO(smsGatewayConnectionRequest{
		URL: "https://other.example.com/send",
	})
	s.Require().NoError(err)
	merged, err := mergeSMSGatewayProperties(incoming.Properties, existing.Properties)
	s.Require().NoError(err)
	value, err := smsGatewayHeaderValue(merged)
	s.Require().NoError(err)
	s.Equal(headers, value)
}

func (s *SMSGatewayTestSuite) TestGeneralHeadersAllowLegacyEmptyAndReservedValues() {
	headers := "Content-Type: text/plain, X-Empty: "
	dto, err := smsGatewayToSenderDTO(smsGatewayConnectionRequest{
		URL: "https://sms.example.com/send", HTTPHeaders: &headers,
		Authentication: apiKeyAuthentication(map[string]string{"X-API-Key": "secret"}),
	})
	s.Require().NoError(err)
	value, err := smsGatewayHeaderValue(dto.Properties)
	s.Require().NoError(err)
	s.Equal(headers, value)
}

func (s *SMSGatewayTestSuite) TestCreateReturnsPlaintextNonSecretFields() {
	s.mockNotif.On("CreateSender", mock.Anything, mock.Anything).
		Return(&ncommon.NotificationSenderDTO{
			ID:       "sg-1",
			Name:     "Prod SMS",
			Type:     ncommon.NotificationSenderTypeMessage,
			Provider: ncommon.NotificationProviderTypeCustom,
			Properties: []cmodels.Property{
				mustProperty(s.T(), ncommon.CustomPropKeyURL, "https://sms.example.com/send", false),
				mustProperty(s.T(), ncommon.CustomPropKeyHTTPMethod, "POST", false),
				mustProperty(s.T(), "authentication_type", "api_key", false),
				mustProperty(s.T(), "authentication_X-Api-Key", "secret", true),
				mustProperty(s.T(), "authentication_X-Tenant", "tenant-1", true),
				mustProperty(s.T(), ncommon.CustomPropKeyContentType, "JSON", false),
			},
		}, (*tidcommon.ServiceError)(nil))

	body, _ := json.Marshal(smsGatewayConnectionRequest{
		Name: "Prod SMS", URL: "https://sms.example.com/send", HTTPMethod: "POST",
		Authentication: apiKeyAuthentication(map[string]string{
			"X-API-Key": "secret",
			"X-Tenant":  "tenant-1",
		}),
		ContentType: "JSON",
	})
	req := httptest.NewRequest(http.MethodPost, "/connections/sms-gateway", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	createSenderHandler(s.handler, smsGatewayToSenderDTO, smsGatewayFromSenderDTO)(rr, req)

	s.Equal(http.StatusCreated, rr.Code)
	var resp smsGatewayConnectionResponse
	s.Require().NoError(json.NewDecoder(rr.Body).Decode(&resp))
	s.Equal("sg-1", resp.ID)
	s.Equal("sms-gateway", resp.Type)
	s.Equal("https://sms.example.com/send", resp.URL)
	s.Equal("POST", resp.HTTPMethod)
	s.Equal(map[string]string{
		"X-Api-Key": maskedSecretValue,
		"X-Tenant":  maskedSecretValue,
	}, resp.Authentication.Properties)
	s.Equal("JSON", resp.ContentType)
}

func (s *SMSGatewayTestSuite) TestGetRoundTrip() {
	s.mockNotif.On("GetSender", mock.Anything, "sg-1").
		Return(&ncommon.NotificationSenderDTO{
			ID:       "sg-1",
			Name:     "Prod SMS",
			Type:     ncommon.NotificationSenderTypeMessage,
			Provider: ncommon.NotificationProviderTypeCustom,
			Properties: []cmodels.Property{
				mustProperty(s.T(), ncommon.CustomPropKeyURL, "https://sms.example.com/send", false),
			},
		}, (*tidcommon.ServiceError)(nil))

	req := httptest.NewRequest(http.MethodGet, "/connections/sms-gateway/sg-1", nil)
	req.SetPathValue("id", "sg-1")
	rr := httptest.NewRecorder()
	getSenderHandler(s.handler, ncommon.NotificationSenderTypeMessage,
		ncommon.NotificationProviderTypeCustom, smsGatewayFromSenderDTO)(rr, req)

	s.Equal(http.StatusOK, rr.Code)
	var resp smsGatewayConnectionResponse
	s.Require().NoError(json.NewDecoder(rr.Body).Decode(&resp))
	s.Equal("Prod SMS", resp.Name)
	s.Equal("https://sms.example.com/send", resp.URL)
}

func (s *SMSGatewayTestSuite) TestGetProviderMismatchReturnsNotFound() {
	s.mockNotif.On("GetSender", mock.Anything, "tw-1").
		Return(&ncommon.NotificationSenderDTO{
			ID: "tw-1", Type: ncommon.NotificationSenderTypeMessage, Provider: ncommon.NotificationProviderTypeTwilio,
		}, (*tidcommon.ServiceError)(nil))

	req := httptest.NewRequest(http.MethodGet, "/connections/sms-gateway/tw-1", nil)
	req.SetPathValue("id", "tw-1")
	rr := httptest.NewRecorder()
	getSenderHandler(s.handler, ncommon.NotificationSenderTypeMessage,
		ncommon.NotificationProviderTypeCustom, smsGatewayFromSenderDTO)(rr, req)

	s.Equal(http.StatusNotFound, rr.Code)
}
