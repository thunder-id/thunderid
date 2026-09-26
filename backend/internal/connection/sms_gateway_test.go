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
	"github.com/thunder-id/thunderid/internal/system/outboundauthn"
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
		APIKeyHeaders: []outboundauthn.APIKeyHeader{
			{Name: "X-API-Key", Value: "secret"},
			{Name: "X-Tenant", Value: "tenant-1"},
		},
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
	s.Equal(maskedSecretValue, values[ncommon.CustomPropKeyAPIKeyHeaders])
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
	s.NotContains(values, ncommon.CustomPropKeyAPIKeyHeaders)
	s.NotContains(values, ncommon.CustomPropKeyContentType)
}

func (s *SMSGatewayTestSuite) TestToSenderDTORejectsInvalidAPIKeyHeader() {
	_, err := smsGatewayToSenderDTO(smsGatewayConnectionRequest{
		Name: "Prod SMS", URL: "https://sms.example.com/send",
		APIKeyHeaders: []outboundauthn.APIKeyHeader{{
			Name: "Content-Type", Value: "application/json",
		}},
	})

	s.Require().Error(err)
}

func (s *SMSGatewayTestSuite) TestCreateRejectsMaskedAPIKeyHeaderValue() {
	_, err := smsGatewayToSenderDTO(smsGatewayConnectionRequest{
		Name: "Prod SMS", URL: "https://sms.example.com/send",
		APIKeyHeaders: []outboundauthn.APIKeyHeader{{Name: "X-API-Key", Value: maskedSecretValue}},
	})

	s.Require().Error(err)
}

func (s *SMSGatewayTestSuite) TestMergeStoredAPIKeyHeadersSupportsPartialDeletion() {
	existingProperties := []cmodels.Property{
		mustProperty(s.T(), legacyHTTPHeadersProperty,
			"X-First-Key: first-secret, X-Second-Key: second-secret", false),
	}
	incomingDTO, err := smsGatewayUpdateToSenderDTO(smsGatewayConnectionRequest{
		Name: "Prod SMS", URL: "https://sms.example.com/send",
		APIKeyHeaders: []outboundauthn.APIKeyHeader{
			{Name: "X-Second-Key", Value: maskedSecretValue},
			{Name: "X-Third-Key", Value: "third-secret"},
		},
	})
	s.Require().NoError(err)

	properties, err := mergeStoredSMSGatewayAPIKeyHeaders(incomingDTO.Properties, existingProperties)
	s.Require().NoError(err)
	headers, err := smsGatewayAPIKeyHeaders(properties, false)
	s.Require().NoError(err)
	s.Equal([]outboundauthn.APIKeyHeader{
		{Name: "X-Second-Key", Value: "second-secret"},
		{Name: "X-Third-Key", Value: "third-secret"},
	}, headers)
	for _, property := range properties {
		if property.GetName() == ncommon.CustomPropKeyAPIKeyHeaders {
			s.True(property.IsSecret())
		}
	}
}

func (s *SMSGatewayTestSuite) TestFromSenderDTOMasksLegacyHTTPHeaders() {
	response, err := smsGatewayFromSenderDTO(ncommon.NotificationSenderDTO{
		ID: "sg-1", Name: "Legacy SMS", Provider: ncommon.NotificationProviderTypeCustom,
		Properties: []cmodels.Property{
			mustProperty(s.T(), ncommon.CustomPropKeyURL, "https://sms.example.com/send", false),
			mustProperty(s.T(), legacyHTTPHeadersProperty,
				"X-API-Key: legacy-secret, X-Tenant: tenant-1", false),
		},
	})

	s.Require().NoError(err)
	s.Equal([]outboundauthn.APIKeyHeader{
		{Name: "X-Api-Key", Value: maskedSecretValue},
		{Name: "X-Tenant", Value: maskedSecretValue},
	}, response.APIKeyHeaders)
}

func (s *SMSGatewayTestSuite) TestMergeStoredAPIKeyHeadersMigratesOmittedLegacyHeaders() {
	existingProperties := []cmodels.Property{
		mustProperty(s.T(), legacyHTTPHeadersProperty, "X-API-Key: legacy-secret", false),
	}

	properties, err := mergeStoredSMSGatewayAPIKeyHeaders(nil, existingProperties)
	s.Require().NoError(err)
	s.Require().Len(properties, 1)
	s.True(properties[0].IsSecret())
	headers, err := smsGatewayAPIKeyHeaders(properties, false)
	s.Require().NoError(err)
	s.Equal([]outboundauthn.APIKeyHeader{{Name: "X-Api-Key", Value: "legacy-secret"}}, headers)
}

func (s *SMSGatewayTestSuite) TestMergeStoredAPIKeyHeadersSupportsDeletingAll() {
	existingDTO, err := smsGatewayToSenderDTO(smsGatewayConnectionRequest{
		Name: "Prod SMS", URL: "https://sms.example.com/send",
		APIKeyHeaders: []outboundauthn.APIKeyHeader{{Name: "X-API-Key", Value: "secret"}},
	})
	s.Require().NoError(err)
	incomingDTO, err := smsGatewayUpdateToSenderDTO(smsGatewayConnectionRequest{
		Name: "Prod SMS", URL: "https://sms.example.com/send",
		APIKeyHeaders: []outboundauthn.APIKeyHeader{},
	})
	s.Require().NoError(err)

	properties, err := mergeStoredSMSGatewayAPIKeyHeaders(incomingDTO.Properties, existingDTO.Properties)
	s.Require().NoError(err)
	headers, err := smsGatewayAPIKeyHeaders(properties, false)
	s.Require().NoError(err)
	s.Empty(headers)
	s.True(hasProperty(properties, ncommon.CustomPropKeyAPIKeyHeaders))
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
				mustProperty(s.T(), ncommon.CustomPropKeyAPIKeyHeaders,
					`[{"name":"X-API-Key","value":"secret"},{"name":"X-Tenant","value":"tenant-1"}]`, true),
				mustProperty(s.T(), ncommon.CustomPropKeyContentType, "JSON", false),
			},
		}, (*tidcommon.ServiceError)(nil))

	body, _ := json.Marshal(smsGatewayConnectionRequest{
		Name: "Prod SMS", URL: "https://sms.example.com/send", HTTPMethod: "POST",
		APIKeyHeaders: []outboundauthn.APIKeyHeader{
			{Name: "X-API-Key", Value: "secret"},
			{Name: "X-Tenant", Value: "tenant-1"},
		},
		ContentType: "JSON",
	})
	req := httptest.NewRequest(http.MethodPost, "/connections/sms-gateway", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	createSMSHandler(s.handler, smsGatewayToSenderDTO, smsGatewayFromSenderDTO)(rr, req)

	s.Equal(http.StatusCreated, rr.Code)
	var resp smsGatewayConnectionResponse
	s.Require().NoError(json.NewDecoder(rr.Body).Decode(&resp))
	s.Equal("sg-1", resp.ID)
	s.Equal("sms-gateway", resp.Type)
	s.Equal("https://sms.example.com/send", resp.URL)
	s.Equal("POST", resp.HTTPMethod)
	s.Equal([]outboundauthn.APIKeyHeader{
		{Name: "X-Api-Key", Value: maskedSecretValue},
		{Name: "X-Tenant", Value: maskedSecretValue},
	}, resp.APIKeyHeaders)
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
	getSMSHandler(s.handler, ncommon.NotificationProviderTypeCustom, smsGatewayFromSenderDTO)(rr, req)

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
	getSMSHandler(s.handler, ncommon.NotificationProviderTypeCustom, smsGatewayFromSenderDTO)(rr, req)

	s.Equal(http.StatusNotFound, rr.Code)
}
