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
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/idp/idpmock"
	"github.com/thunder-id/thunderid/tests/mocks/notification/notificationmock"
)

type VonageTestSuite struct {
	suite.Suite
	handler   *handler
	mockIDP   *idpmock.IDPServiceInterfaceMock
	mockNotif *notificationmock.NotificationSenderMgtSvcInterfaceMock
}

func TestVonageSuite(t *testing.T) {
	suite.Run(t, new(VonageTestSuite))
}

func (s *VonageTestSuite) SetupTest() {
	s.handler, s.mockIDP, s.mockNotif = newConnectionTestHandler(s.T())
}

func (s *VonageTestSuite) TestToSenderDTOMapsFields() {
	dto, err := vonageToSenderDTO(vonageConnectionRequest{
		Name: "My Vonage", Description: "OTP over SMS",
		APIKey: "a1b2c3d4", APISecret: "sec", SenderID: "ThunderID",
	})
	s.Require().NoError(err)
	s.Equal(ncommon.NotificationSenderTypeMessage, dto.Type)
	s.Equal(ncommon.NotificationProviderTypeVonage, dto.Provider)
	s.Equal("OTP over SMS", dto.Description)

	values, err := propertyValues(dto.Properties)
	s.Require().NoError(err)
	s.Equal("a1b2c3d4", values[ncommon.VonagePropKeyAPIKey])
	s.Equal(maskedSecretValue, values[ncommon.VonagePropKeyAPISecret]) // secret is encrypted/masked
	s.Equal("ThunderID", values[ncommon.VonagePropKeySenderID])
}

//nolint:dupl // vonage's create/get tests mirror twilio's identical shape, kept distinct per vendor
func (s *VonageTestSuite) TestCreateMasksSecret() {
	s.mockNotif.On("CreateSender", mock.Anything, mock.Anything).
		Return(&ncommon.NotificationSenderDTO{
			ID:       "vo-1",
			Name:     "My Vonage",
			Type:     ncommon.NotificationSenderTypeMessage,
			Provider: ncommon.NotificationProviderTypeVonage,
			Properties: []cmodels.Property{
				mustProperty(s.T(), ncommon.VonagePropKeyAPIKey, "a1b2c3d4", false),
				mustProperty(s.T(), ncommon.VonagePropKeyAPISecret, "sec", true),
				mustProperty(s.T(), ncommon.VonagePropKeySenderID, "ThunderID", false),
			},
		}, (*tidcommon.ServiceError)(nil))

	body, _ := json.Marshal(vonageConnectionRequest{
		Name: "My Vonage", APIKey: "a1b2c3d4", APISecret: "sec", SenderID: "ThunderID",
	})
	req := httptest.NewRequest(http.MethodPost, "/connections/vonage", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	createSMSHandler(s.handler, vonageToSenderDTO, vonageFromSenderDTO)(rr, req)

	s.Equal(http.StatusCreated, rr.Code)
	var resp vonageConnectionResponse
	s.Require().NoError(json.NewDecoder(rr.Body).Decode(&resp))
	s.Equal("vo-1", resp.ID)
	s.Equal("vonage", resp.Type)
	s.Equal("a1b2c3d4", resp.APIKey)
	s.Equal(maskedSecretValue, resp.APISecret)
	s.Equal("ThunderID", resp.SenderID)
}

func (s *VonageTestSuite) TestGetRoundTrip() {
	s.mockNotif.On("GetSender", mock.Anything, "vo-1").
		Return(&ncommon.NotificationSenderDTO{
			ID:       "vo-1",
			Name:     "My Vonage",
			Type:     ncommon.NotificationSenderTypeMessage,
			Provider: ncommon.NotificationProviderTypeVonage,
			Properties: []cmodels.Property{
				mustProperty(s.T(), ncommon.VonagePropKeyAPIKey, "a1b2c3d4", false),
				mustProperty(s.T(), ncommon.VonagePropKeyAPISecret, "sec", true),
				mustProperty(s.T(), ncommon.VonagePropKeySenderID, "ThunderID", false),
			},
		}, (*tidcommon.ServiceError)(nil))

	req := httptest.NewRequest(http.MethodGet, "/connections/vonage/vo-1", nil)
	req.SetPathValue("id", "vo-1")
	rr := httptest.NewRecorder()
	getSMSHandler(s.handler, ncommon.NotificationProviderTypeVonage, vonageFromSenderDTO)(rr, req)

	s.Equal(http.StatusOK, rr.Code)
	var resp vonageConnectionResponse
	s.Require().NoError(json.NewDecoder(rr.Body).Decode(&resp))
	s.Equal("My Vonage", resp.Name)
	s.Equal("a1b2c3d4", resp.APIKey)
	s.Equal(maskedSecretValue, resp.APISecret)
	s.Equal("ThunderID", resp.SenderID)
}
