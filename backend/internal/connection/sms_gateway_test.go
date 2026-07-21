/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

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
		HTTPHeaders: "Authorization: Bearer abc123", ContentType: "JSON",
	})
	s.Require().NoError(err)
	s.Equal(ncommon.NotificationSenderTypeMessage, dto.Type)
	s.Equal(ncommon.MessageProviderTypeCustom, dto.Provider)
	s.Equal("Custom webhook sender", dto.Description)

	values, err := propertyValues(dto.Properties)
	s.Require().NoError(err)
	s.Equal("https://sms.example.com/send", values[ncommon.CustomPropKeyURL])
	s.Equal("POST", values[ncommon.CustomPropKeyHTTPMethod])
	s.Equal("Authorization: Bearer abc123", values[ncommon.CustomPropKeyHTTPHeaders])
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
	s.NotContains(values, ncommon.CustomPropKeyHTTPHeaders)
	s.NotContains(values, ncommon.CustomPropKeyContentType)
}

func (s *SMSGatewayTestSuite) TestCreateReturnsPlaintextNonSecretFields() {
	s.mockNotif.On("CreateSender", mock.Anything, mock.Anything).
		Return(&ncommon.NotificationSenderDTO{
			ID:       "sg-1",
			Name:     "Prod SMS",
			Type:     ncommon.NotificationSenderTypeMessage,
			Provider: ncommon.MessageProviderTypeCustom,
			Properties: []cmodels.Property{
				mustProperty(s.T(), ncommon.CustomPropKeyURL, "https://sms.example.com/send", false),
				mustProperty(s.T(), ncommon.CustomPropKeyHTTPMethod, "POST", false),
				mustProperty(s.T(), ncommon.CustomPropKeyHTTPHeaders, "Authorization: Bearer abc123", false),
				mustProperty(s.T(), ncommon.CustomPropKeyContentType, "JSON", false),
			},
		}, (*tidcommon.ServiceError)(nil))

	body, _ := json.Marshal(smsGatewayConnectionRequest{
		Name: "Prod SMS", URL: "https://sms.example.com/send", HTTPMethod: "POST",
		HTTPHeaders: "Authorization: Bearer abc123", ContentType: "JSON",
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
	s.Equal("Authorization: Bearer abc123", resp.HTTPHeaders)
	s.Equal("JSON", resp.ContentType)
}

func (s *SMSGatewayTestSuite) TestGetRoundTrip() {
	s.mockNotif.On("GetSender", mock.Anything, "sg-1").
		Return(&ncommon.NotificationSenderDTO{
			ID:       "sg-1",
			Name:     "Prod SMS",
			Type:     ncommon.NotificationSenderTypeMessage,
			Provider: ncommon.MessageProviderTypeCustom,
			Properties: []cmodels.Property{
				mustProperty(s.T(), ncommon.CustomPropKeyURL, "https://sms.example.com/send", false),
			},
		}, (*tidcommon.ServiceError)(nil))

	req := httptest.NewRequest(http.MethodGet, "/connections/sms-gateway/sg-1", nil)
	req.SetPathValue("id", "sg-1")
	rr := httptest.NewRecorder()
	getSMSHandler(s.handler, ncommon.MessageProviderTypeCustom, smsGatewayFromSenderDTO)(rr, req)

	s.Equal(http.StatusOK, rr.Code)
	var resp smsGatewayConnectionResponse
	s.Require().NoError(json.NewDecoder(rr.Body).Decode(&resp))
	s.Equal("Prod SMS", resp.Name)
	s.Equal("https://sms.example.com/send", resp.URL)
}

func (s *SMSGatewayTestSuite) TestGetProviderMismatchReturnsNotFound() {
	s.mockNotif.On("GetSender", mock.Anything, "tw-1").
		Return(&ncommon.NotificationSenderDTO{
			ID: "tw-1", Type: ncommon.NotificationSenderTypeMessage, Provider: ncommon.MessageProviderTypeTwilio,
		}, (*tidcommon.ServiceError)(nil))

	req := httptest.NewRequest(http.MethodGet, "/connections/sms-gateway/tw-1", nil)
	req.SetPathValue("id", "tw-1")
	rr := httptest.NewRecorder()
	getSMSHandler(s.handler, ncommon.MessageProviderTypeCustom, smsGatewayFromSenderDTO)(rr, req)

	s.Equal(http.StatusNotFound, rr.Code)
}
