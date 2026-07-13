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

type TwilioTestSuite struct {
	suite.Suite
	handler   *handler
	mockIDP   *idpmock.IDPServiceInterfaceMock
	mockNotif *notificationmock.NotificationSenderMgtSvcInterfaceMock
}

func TestTwilioSuite(t *testing.T) {
	suite.Run(t, new(TwilioTestSuite))
}

func (s *TwilioTestSuite) SetupTest() {
	s.handler, s.mockIDP, s.mockNotif = newConnectionTestHandler(s.T())
}

func (s *TwilioTestSuite) TestToSenderDTOMapsFields() {
	dto, err := twilioToSenderDTO(twilioConnectionRequest{
		Name: "My Twilio", Description: "OTP over SMS",
		AccountSID: "AC00000000000000000000000000000000", AuthToken: "tok", SenderID: "+15005550006",
	})
	s.Require().NoError(err)
	s.Equal(ncommon.NotificationSenderTypeMessage, dto.Type)
	s.Equal(ncommon.MessageProviderTypeTwilio, dto.Provider)
	s.Equal("OTP over SMS", dto.Description)

	values, err := propertyValues(dto.Properties)
	s.Require().NoError(err)
	s.Equal("AC00000000000000000000000000000000", values[ncommon.TwilioPropKeyAccountSID])
	s.Equal(maskedSecretValue, values[ncommon.TwilioPropKeyAuthToken]) // secret is encrypted/masked
	s.Equal("+15005550006", values[ncommon.TwilioPropKeySenderID])
}

//nolint:dupl // twilio's create/get tests mirror vonage's identical shape, kept distinct per vendor
func (s *TwilioTestSuite) TestCreateMasksSecret() {
	s.mockNotif.On("CreateSender", mock.Anything, mock.Anything).
		Return(&ncommon.NotificationSenderDTO{
			ID:       "tw-1",
			Name:     "My Twilio",
			Type:     ncommon.NotificationSenderTypeMessage,
			Provider: ncommon.MessageProviderTypeTwilio,
			Properties: []cmodels.Property{
				mustProperty(s.T(), ncommon.TwilioPropKeyAccountSID, "AC00000000000000000000000000000000", false),
				mustProperty(s.T(), ncommon.TwilioPropKeyAuthToken, "s3cret", true),
				mustProperty(s.T(), ncommon.TwilioPropKeySenderID, "+15005550006", false),
			},
		}, (*tidcommon.ServiceError)(nil))

	body, _ := json.Marshal(twilioConnectionRequest{
		Name: "My Twilio", AccountSID: "AC00000000000000000000000000000000",
		AuthToken: "s3cret", SenderID: "+15005550006",
	})
	req := httptest.NewRequest(http.MethodPost, "/connections/twilio", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	createSMSHandler(s.handler, twilioToSenderDTO, twilioFromSenderDTO)(rr, req)

	s.Equal(http.StatusCreated, rr.Code)
	var resp twilioConnectionResponse
	s.Require().NoError(json.NewDecoder(rr.Body).Decode(&resp))
	s.Equal("tw-1", resp.ID)
	s.Equal("twilio", resp.Type)
	s.Equal("AC00000000000000000000000000000000", resp.AccountSID)
	s.Equal(maskedSecretValue, resp.AuthToken)
	s.Equal("+15005550006", resp.SenderID)
}

func (s *TwilioTestSuite) TestGetRoundTrip() {
	s.mockNotif.On("GetSender", mock.Anything, "tw-1").
		Return(&ncommon.NotificationSenderDTO{
			ID:       "tw-1",
			Name:     "My Twilio",
			Type:     ncommon.NotificationSenderTypeMessage,
			Provider: ncommon.MessageProviderTypeTwilio,
			Properties: []cmodels.Property{
				mustProperty(s.T(), ncommon.TwilioPropKeyAccountSID, "AC00000000000000000000000000000000", false),
				mustProperty(s.T(), ncommon.TwilioPropKeyAuthToken, "s3cret", true),
				mustProperty(s.T(), ncommon.TwilioPropKeySenderID, "+15005550006", false),
			},
		}, (*tidcommon.ServiceError)(nil))

	req := httptest.NewRequest(http.MethodGet, "/connections/twilio/tw-1", nil)
	req.SetPathValue("id", "tw-1")
	rr := httptest.NewRecorder()
	getSMSHandler(s.handler, ncommon.MessageProviderTypeTwilio, twilioFromSenderDTO)(rr, req)

	s.Equal(http.StatusOK, rr.Code)
	var resp twilioConnectionResponse
	s.Require().NoError(json.NewDecoder(rr.Body).Decode(&resp))
	s.Equal("My Twilio", resp.Name)
	s.Equal("AC00000000000000000000000000000000", resp.AccountSID)
	s.Equal(maskedSecretValue, resp.AuthToken)
	s.Equal("+15005550006", resp.SenderID)
}

func (s *TwilioTestSuite) TestGetProviderMismatchReturnsNotFound() {
	s.mockNotif.On("GetSender", mock.Anything, "vo-1").
		Return(&ncommon.NotificationSenderDTO{
			ID: "vo-1", Type: ncommon.NotificationSenderTypeMessage, Provider: ncommon.MessageProviderTypeVonage,
		}, (*tidcommon.ServiceError)(nil))

	req := httptest.NewRequest(http.MethodGet, "/connections/twilio/vo-1", nil)
	req.SetPathValue("id", "vo-1")
	rr := httptest.NewRecorder()
	getSMSHandler(s.handler, ncommon.MessageProviderTypeTwilio, twilioFromSenderDTO)(rr, req)

	s.Equal(http.StatusNotFound, rr.Code)
}
