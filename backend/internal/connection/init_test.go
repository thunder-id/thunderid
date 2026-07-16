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

// Package connection provides tests for the connections API. This file holds the shared
// test fixtures plus the route-registration (Initialize) integration test.
package connection

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/idp"
	ncommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/idp/idpmock"
	"github.com/thunder-id/thunderid/tests/mocks/notification/notificationmock"
)

// testCryptoKey is the shared key used so secret property encryption works in tests.
const testCryptoKey = "0579f866ac7c9273580d0ff163fa01a7b2401a7ff3ddc3e3b14ae3136fa6025e"

// initConfigWithTestCryptoKey initializes the server runtime with the test crypto key.
func initConfigWithTestCryptoKey() {
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/tmp/test", &config.Config{
		Crypto: config.CryptoConfig{Encryption: engineconfig.EncryptionConfig{Key: testCryptoKey}},
	})
}

// newConnectionTestHandler returns a connection handler over fresh mock IdP and
// notification-sender services, with the config crypto key initialized and cleaned up
// automatically.
func newConnectionTestHandler(t *testing.T) (*handler, *idpmock.IDPServiceInterfaceMock,
	*notificationmock.NotificationSenderMgtSvcInterfaceMock) {
	t.Helper()
	initConfigWithTestCryptoKey()
	t.Cleanup(config.ResetServerRuntime)
	mockIDP := idpmock.NewIDPServiceInterfaceMock(t)
	mockNotif := notificationmock.NewNotificationSenderMgtSvcInterfaceMock(t)
	return newHandler(newService(mockIDP, mockNotif)), mockIDP, mockNotif
}

// mustProperty builds a property, failing the test on error.
func mustProperty(t *testing.T, name, value string, isSecret bool) cmodels.Property {
	t.Helper()
	prop, err := cmodels.NewProperty(name, value, isSecret)
	require.NoError(t, err)
	return *prop
}

func boolPtr(b bool) *bool { return &b }

// InitTestSuite drives the full route table registered by Initialize through a real
// ServeMux, exercising route registration, CORS/OPTIONS handling, and path-value extraction.
type InitTestSuite struct {
	suite.Suite
	mux       *http.ServeMux
	mockIDP   *idpmock.IDPServiceInterfaceMock
	mockNotif *notificationmock.NotificationSenderMgtSvcInterfaceMock
}

func TestInitSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

func (s *InitTestSuite) SetupTest() {
	initConfigWithTestCryptoKey()
	s.mockIDP = idpmock.NewIDPServiceInterfaceMock(s.T())
	s.mockNotif = notificationmock.NewNotificationSenderMgtSvcInterfaceMock(s.T())
	s.mux = http.NewServeMux()
	Initialize(s.mux, s.mockIDP, s.mockNotif)
}

func (s *InitTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (s *InitTestSuite) TestRouteTable() {
	githubDTO := &providers.IDPDTO{ID: "gh-1", Name: "GH", Type: providers.IDPTypeGitHub}
	s.mockIDP.On("GetIdentityProviderList", mock.Anything).
		Return([]idp.BasicIDPDTO{{ID: "gh-1", Name: "GH", Type: providers.IDPTypeGitHub}},
			(*tidcommon.ServiceError)(nil))
	s.mockIDP.On("CreateIdentityProvider", mock.Anything, mock.Anything).
		Return(githubDTO, (*tidcommon.ServiceError)(nil))
	s.mockIDP.On("GetIdentityProvider", mock.Anything, "gh-1").
		Return(githubDTO, (*tidcommon.ServiceError)(nil))
	s.mockIDP.On("UpdateIdentityProvider", mock.Anything, "gh-1", mock.Anything).
		Return(githubDTO, (*tidcommon.ServiceError)(nil))
	s.mockIDP.On("DeleteIdentityProvider", mock.Anything, "gh-1").
		Return((*tidcommon.ServiceError)(nil))

	twilioDTO := &ncommon.NotificationSenderDTO{
		ID: "tw-1", Name: "TW", Type: ncommon.NotificationSenderTypeMessage,
		Provider: ncommon.MessageProviderTypeTwilio,
	}
	s.mockNotif.On("ListSendersByType", mock.Anything, ncommon.NotificationSenderTypeMessage).
		Return([]ncommon.NotificationSenderDTO{*twilioDTO}, (*tidcommon.ServiceError)(nil))
	s.mockNotif.On("CreateSender", mock.Anything, mock.Anything).
		Return(twilioDTO, (*tidcommon.ServiceError)(nil))
	s.mockNotif.On("GetSender", mock.Anything, "tw-1").
		Return(twilioDTO, (*tidcommon.ServiceError)(nil))
	s.mockNotif.On("UpdateSender", mock.Anything, "tw-1", mock.Anything).
		Return(twilioDTO, (*tidcommon.ServiceError)(nil))
	s.mockNotif.On("DeleteSender", mock.Anything, "tw-1").
		Return((*tidcommon.ServiceError)(nil))

	body, _ := json.Marshal(githubConnectionRequest{
		Name: "GH", ClientID: "c", ClientSecret: "s", RedirectURI: "https://app/cb",
	})
	twilioBody, _ := json.Marshal(twilioConnectionRequest{
		Name: "TW", AccountSID: "AC00000000000000000000000000000000", AuthToken: "tok", SenderID: "+15005550006",
	})

	cases := []struct {
		method, path string
		body         []byte
		want         int
	}{
		{http.MethodGet, "/connections", nil, http.StatusOK},
		{http.MethodOptions, "/connections", nil, http.StatusNoContent},
		{http.MethodPost, "/connections/github", body, http.StatusCreated},
		{http.MethodGet, "/connections/github", nil, http.StatusOK},
		{http.MethodOptions, "/connections/github", nil, http.StatusNoContent},
		{http.MethodGet, "/connections/github/gh-1", nil, http.StatusOK},
		{http.MethodPut, "/connections/github/gh-1", body, http.StatusOK},
		{http.MethodDelete, "/connections/github/gh-1", nil, http.StatusNoContent},
		{http.MethodOptions, "/connections/github/gh-1", nil, http.StatusNoContent},
		{http.MethodPost, "/connections/twilio", twilioBody, http.StatusCreated},
		{http.MethodGet, "/connections/twilio", nil, http.StatusOK},
		{http.MethodOptions, "/connections/twilio", nil, http.StatusNoContent},
		{http.MethodGet, "/connections/twilio/tw-1", nil, http.StatusOK},
		{http.MethodPut, "/connections/twilio/tw-1", twilioBody, http.StatusOK},
		{http.MethodDelete, "/connections/twilio/tw-1", nil, http.StatusNoContent},
		{http.MethodOptions, "/connections/twilio/tw-1", nil, http.StatusNoContent},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, bytes.NewReader(tc.body))
		rr := httptest.NewRecorder()
		s.mux.ServeHTTP(rr, req)
		s.Equal(tc.want, rr.Code, "%s %s", tc.method, tc.path)
	}
}
