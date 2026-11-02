// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/connection/httpconnection"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/notification"
	ncommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/connection/httpconnectionmock"
	"github.com/thunder-id/thunderid/tests/mocks/idp/idpmock"
	"github.com/thunder-id/thunderid/tests/mocks/notification/notificationmock"
)

// HandlerTestSuite covers the shared HTTP plumbing (decode, status mapping, empty-id and
// list/delete handlers), exercised through Google as the representative vendor.
type HandlerTestSuite struct {
	suite.Suite
	handler   *handler
	mockIDP   *idpmock.IDPServiceInterfaceMock
	mockNotif *notificationmock.NotificationSenderMgtSvcInterfaceMock
}

func TestHandlerSuite(t *testing.T) {
	suite.Run(t, new(HandlerTestSuite))
}

func (s *HandlerTestSuite) SetupTest() {
	s.handler, s.mockIDP, s.mockNotif = newConnectionTestHandler(s.T())
}

// mockListFixtures registers the shared list fixtures: 2 google + 1 oidc IdPs, a twilio and a
// custom message sender, and an SMTP email sender.
func (s *HandlerTestSuite) mockListFixtures() {
	s.mockIDP.On("GetIdentityProviderList", mock.Anything).Return([]idp.BasicIDPDTO{
		{ID: "1", Name: "Google One", Type: providers.IDPTypeGoogle},
		{ID: "2", Name: "Google Two", Type: providers.IDPTypeGoogle},
		{ID: "3", Name: "Corp OIDC", Description: "corp", Type: providers.IDPTypeOIDC},
	}, (*tidcommon.ServiceError)(nil))
	s.mockNotif.On("ListSendersByType", mock.Anything, ncommon.NotificationSenderTypeMessage).
		Return([]ncommon.NotificationSenderDTO{
			{ID: "s1", Name: "Twilio SMS", Type: ncommon.NotificationSenderTypeMessage,
				Provider: ncommon.NotificationProviderTypeTwilio},
			{ID: "s2", Name: "Gateway", Type: ncommon.NotificationSenderTypeMessage,
				Provider: ncommon.NotificationProviderTypeCustom},
		}, (*tidcommon.ServiceError)(nil))
	s.mockNotif.On("ListSendersByType", mock.Anything, ncommon.NotificationSenderTypeEmail).
		Return([]ncommon.NotificationSenderDTO{
			{ID: "e1", Name: "Corp SMTP", Type: ncommon.NotificationSenderTypeEmail,
				Provider: ncommon.NotificationProviderTypeSMTP},
		}, (*tidcommon.ServiceError)(nil))
}

func (s *HandlerTestSuite) listConnections(target string) (*httptest.ResponseRecorder, connectionListResponse) {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rr := httptest.NewRecorder()
	s.handler.handleListConnections(rr, req)
	var resp connectionListResponse
	if rr.Code == http.StatusOK {
		s.Require().NoError(json.NewDecoder(rr.Body).Decode(&resp))
	}
	return rr, resp
}

func (s *HandlerTestSuite) TestListConnections() {
	s.mockListFixtures()

	rr, resp := s.listConnections("/connections")

	s.Equal(http.StatusOK, rr.Code)
	s.Require().Len(resp.Connections, 6)
	s.Equal(6, resp.TotalResults)
	s.Equal(1, resp.StartIndex)
	s.Equal(6, resp.Count)
	// Sorted by type, then name, then ID.
	ids := make([]string, 0, len(resp.Connections))
	for _, c := range resp.Connections {
		ids = append(ids, c.ID)
	}
	s.Equal([]string{"e1", "1", "2", "3", "s2", "s1"}, ids)
	s.Equal(emailSMTPVendorName, resp.Connections[0].Type)
	s.Equal([]connectionCategory{categoryEmailProvider}, resp.Connections[0].Categories)
	s.Equal("google", resp.Connections[1].Type)
	s.Equal([]connectionCategory{categoryIdentityProvider}, resp.Connections[1].Categories)
	s.Equal("google", resp.Connections[2].Type)
	s.Equal([]connectionCategory{categoryIdentityProvider}, resp.Connections[2].Categories)
	s.Equal("oidc", resp.Connections[3].Type)
	s.Equal("corp", resp.Connections[3].Description)
	s.Equal("sms-gateway", resp.Connections[4].Type)
	s.Equal([]connectionCategory{categorySMSProvider}, resp.Connections[4].Categories)
	s.Equal("twilio", resp.Connections[5].Type)
	s.Empty(resp.Links)
}

func (s *HandlerTestSuite) TestListConnectionsPagination() {
	s.mockListFixtures()

	rr, resp := s.listConnections("/connections?limit=2&offset=1")

	s.Equal(http.StatusOK, rr.Code)
	s.Equal(6, resp.TotalResults)
	s.Equal(2, resp.StartIndex)
	s.Equal(2, resp.Count)
	s.Require().Len(resp.Connections, 2)
	s.Equal("1", resp.Connections[0].ID)
	s.Equal("2", resp.Connections[1].ID)

	rels := make([]string, 0, len(resp.Links))
	for _, link := range resp.Links {
		rels = append(rels, link.Rel)
	}
	s.Contains(rels, "next")
	s.Contains(rels, "prev")
}

func (s *HandlerTestSuite) TestListConnectionsOffsetPastEnd() {
	s.mockListFixtures()

	rr, resp := s.listConnections("/connections?offset=100")

	s.Equal(http.StatusOK, rr.Code)
	s.Equal(6, resp.TotalResults)
	s.Equal(101, resp.StartIndex)
	s.Equal(0, resp.Count)
	s.NotNil(resp.Connections)
	s.Empty(resp.Connections)
}

func (s *HandlerTestSuite) TestListConnectionsInvalidLimit() {
	targets := []string{
		"/connections?limit=abc",
		"/connections?limit=0",
		"/connections?limit=-5",
		"/connections?limit=101",
	}
	for _, target := range targets {
		rr, _ := s.listConnections(target)
		s.Equal(http.StatusBadRequest, rr.Code, target)
		s.Contains(rr.Body.String(), "CON-1002", target)
	}
	s.mockIDP.AssertNotCalled(s.T(), "GetIdentityProviderList", mock.Anything)
	s.mockNotif.AssertNotCalled(s.T(), "ListSendersByType", mock.Anything, ncommon.NotificationSenderTypeMessage)
}

func (s *HandlerTestSuite) TestListConnectionsInvalidOffset() {
	for _, target := range []string{"/connections?offset=abc", "/connections?offset=-1"} {
		rr, _ := s.listConnections(target)
		s.Equal(http.StatusBadRequest, rr.Code, target)
		s.Contains(rr.Body.String(), "CON-1003", target)
	}
	s.mockIDP.AssertNotCalled(s.T(), "GetIdentityProviderList", mock.Anything)
	s.mockNotif.AssertNotCalled(s.T(), "ListSendersByType", mock.Anything, ncommon.NotificationSenderTypeMessage)
}

func (s *HandlerTestSuite) TestListConnectionsIdentityProviderCategory() {
	s.mockIDP.On("GetIdentityProviderList", mock.Anything).Return([]idp.BasicIDPDTO{
		{ID: "1", Name: "Google One", Type: providers.IDPTypeGoogle},
		{ID: "3", Name: "Corp OIDC", Type: providers.IDPTypeOIDC},
	}, (*tidcommon.ServiceError)(nil))

	rr, resp := s.listConnections("/connections?category=identity-provider")

	s.Equal(http.StatusOK, rr.Code)
	s.Require().Len(resp.Connections, 2)
	s.Equal("google", resp.Connections[0].Type)
	s.Equal("oidc", resp.Connections[1].Type)
	s.mockNotif.AssertNotCalled(s.T(), "ListSendersByType", mock.Anything, ncommon.NotificationSenderTypeMessage)
}

func (s *HandlerTestSuite) TestListConnectionsIDJagEnabledField() {
	trusted := true
	s.mockIDP.On("GetIdentityProviderList", mock.Anything).Return([]idp.BasicIDPDTO{
		{ID: "1", Name: "Trusted Issuer", Type: providers.IDPTypeOIDC, IDJagEnabled: &trusted},
		{ID: "2", Name: "Plain Federation", Type: providers.IDPTypeOIDC},
	}, (*tidcommon.ServiceError)(nil))

	req := httptest.NewRequest(http.MethodGet, "/connections?category=identity-provider", nil)
	rr := httptest.NewRecorder()
	s.handler.handleListConnections(rr, req)
	s.Equal(http.StatusOK, rr.Code)

	var raw struct {
		Connections []map[string]interface{} `json:"connections"`
	}
	s.Require().NoError(json.NewDecoder(rr.Body).Decode(&raw))
	s.Require().Len(raw.Connections, 2)

	byID := make(map[string]map[string]interface{}, len(raw.Connections))
	for _, c := range raw.Connections {
		byID[c["id"].(string)] = c
	}

	s.Require().Contains(byID["1"], "idJagEnabled")
	s.Equal(true, byID["1"]["idJagEnabled"])
	s.NotContains(byID["2"], "idJagEnabled")
}

func (s *HandlerTestSuite) TestListConnectionsSMSProviderCategory() {
	s.mockNotif.On("ListSendersByType", mock.Anything, ncommon.NotificationSenderTypeMessage).
		Return([]ncommon.NotificationSenderDTO{
			{ID: "s1", Name: "Twilio SMS", Type: ncommon.NotificationSenderTypeMessage,
				Provider: ncommon.NotificationProviderTypeTwilio},
			{ID: "s2", Name: "Gateway", Type: ncommon.NotificationSenderTypeMessage,
				Provider: ncommon.NotificationProviderTypeCustom},
		}, (*tidcommon.ServiceError)(nil))

	rr, resp := s.listConnections("/connections?category=sms-provider")

	s.Equal(http.StatusOK, rr.Code)
	s.Require().Len(resp.Connections, 2)
	s.Equal("sms-gateway", resp.Connections[0].Type)
	s.Equal("twilio", resp.Connections[1].Type)
	s.mockIDP.AssertNotCalled(s.T(), "GetIdentityProviderList", mock.Anything)
}

func (s *HandlerTestSuite) TestListConnectionsEmptyCategory() {
	s.mockListFixtures()

	rr, resp := s.listConnections("/connections?category=")

	s.Equal(http.StatusOK, rr.Code)
	s.Len(resp.Connections, 6)
}

func (s *HandlerTestSuite) TestListConnectionsEmailCategorySkipsIdPsAndSMS() {
	s.mockNotif.On("ListSendersByType", mock.Anything, ncommon.NotificationSenderTypeEmail).
		Return([]ncommon.NotificationSenderDTO{
			{ID: "e1", Name: "Corp SMTP", Type: ncommon.NotificationSenderTypeEmail,
				Provider: ncommon.NotificationProviderTypeSMTP},
		}, (*tidcommon.ServiceError)(nil))

	rr, resp := s.listConnections("/connections?category=email-provider")

	s.Equal(http.StatusOK, rr.Code)
	s.Require().Len(resp.Connections, 1)
	s.Equal("e1", resp.Connections[0].ID)
	s.Equal(emailSMTPVendorName, resp.Connections[0].Type)
	s.Equal([]connectionCategory{categoryEmailProvider}, resp.Connections[0].Categories)
	s.mockIDP.AssertNotCalled(s.T(), "GetIdentityProviderList", mock.Anything)
	s.mockNotif.AssertNotCalled(s.T(), "ListSendersByType", mock.Anything,
		ncommon.NotificationSenderTypeMessage)
}

func (s *HandlerTestSuite) TestListConnectionsInvalidCategory() {
	for _, target := range []string{"/connections?category=bogus", "/connections?category=idp"} {
		rr, _ := s.listConnections(target)
		s.Equal(http.StatusBadRequest, rr.Code, target)
		s.Contains(rr.Body.String(), "CON-1001", target)
	}
	s.mockIDP.AssertNotCalled(s.T(), "GetIdentityProviderList", mock.Anything)
	s.mockNotif.AssertNotCalled(s.T(), "ListSendersByType", mock.Anything, ncommon.NotificationSenderTypeMessage)
}

func (s *HandlerTestSuite) TestListConnectionsServiceError() {
	s.mockIDP.On("GetIdentityProviderList", mock.Anything).
		Return(([]idp.BasicIDPDTO)(nil), &tidcommon.InternalServerError)

	rr, _ := s.listConnections("/connections")

	s.Equal(http.StatusInternalServerError, rr.Code)
}

func (s *HandlerTestSuite) TestListConnectionsSenderServiceError() {
	s.mockNotif.On("ListSendersByType", mock.Anything, ncommon.NotificationSenderTypeMessage).
		Return(([]ncommon.NotificationSenderDTO)(nil), &tidcommon.InternalServerError)

	rr, _ := s.listConnections("/connections?category=sms-provider")

	s.Equal(http.StatusInternalServerError, rr.Code)
}

func (s *HandlerTestSuite) googleBody() []byte {
	body, _ := json.Marshal(googleConnectionRequest{
		Name: "g", ClientID: "c", ClientSecret: "s", RedirectURI: "https://app/cb",
	})
	return body
}

func (s *HandlerTestSuite) TestCreateInvalidBody() {
	req := httptest.NewRequest(http.MethodPost, "/connections/google", bytes.NewReader([]byte("{bad")))
	rr := httptest.NewRecorder()
	createHandler(s.handler, googleToIDPDTO, googleFromIDPDTO)(rr, req)
	s.Equal(http.StatusBadRequest, rr.Code)
}

func (s *HandlerTestSuite) TestCreateServiceErrorConflict() {
	s.mockIDP.On("CreateIdentityProvider", mock.Anything, mock.Anything).
		Return((*providers.IDPDTO)(nil), &idp.ErrorIDPAlreadyExists)

	req := httptest.NewRequest(http.MethodPost, "/connections/google", bytes.NewReader(s.googleBody()))
	rr := httptest.NewRecorder()
	createHandler(s.handler, googleToIDPDTO, googleFromIDPDTO)(rr, req)
	s.Equal(http.StatusConflict, rr.Code)
}

// Mapping a request onto a sender DTO fails for two unrelated reasons, and only one of them is
// the server's fault. Naming an authentication method that does not exist is the caller's, so it
// must not be reported as a 500 the caller cannot act on.
func (s *HandlerTestSuite) TestCreateSenderUnsupportedAuthenticationTypeIsBadRequest() {
	body, err := json.Marshal(emailSMTPConnectionRequest{
		Name: "Corp SMTP", Host: "smtp.example.com", Port: 587, FromAddress: "noreply@example.com",
		TLS:            string(ncommon.TLSModeSTARTTLS),
		Authentication: &outboundauth.Authentication{Type: "bearer"},
	})
	s.Require().NoError(err)

	req := httptest.NewRequest(http.MethodPost, "/connections/"+emailSMTPVendorName, bytes.NewReader(body))
	rr := httptest.NewRecorder()
	createSenderHandler(s.handler, emailSMTPToSenderDTO, emailSMTPFromSenderDTO)(rr, req)

	s.Equal(http.StatusBadRequest, rr.Code)
	s.Contains(rr.Body.String(), ErrorInvalidAuthenticationType.Code)
	// The request never reached the service: it was rejected while being mapped.
	s.mockNotif.AssertNotCalled(s.T(), "CreateSender", mock.Anything, mock.Anything)
}

func (s *HandlerTestSuite) TestUpdateSenderUnsupportedAuthenticationTypeIsBadRequest() {
	body, err := json.Marshal(emailSMTPConnectionRequest{
		Name: "Corp SMTP", Host: "smtp.example.com", Port: 587, FromAddress: "noreply@example.com",
		TLS:            string(ncommon.TLSModeSTARTTLS),
		Authentication: &outboundauth.Authentication{Type: "bearer"},
	})
	s.Require().NoError(err)

	req := httptest.NewRequest(http.MethodPut, "/connections/"+emailSMTPVendorName+"/sm-1", bytes.NewReader(body))
	req.SetPathValue("id", "sm-1")
	rr := httptest.NewRecorder()
	updateSenderHandler(s.handler, ncommon.NotificationSenderTypeEmail, ncommon.NotificationProviderTypeSMTP,
		emailSMTPToSenderDTO, emailSMTPFromSenderDTO)(rr, req)

	s.Equal(http.StatusBadRequest, rr.Code)
	s.Contains(rr.Body.String(), ErrorInvalidAuthenticationType.Code)
}

func (s *HandlerTestSuite) TestGetEmptyID() {
	req := httptest.NewRequest(http.MethodGet, "/connections/google/", nil)
	rr := httptest.NewRecorder()
	getHandler(s.handler, providers.IDPTypeGoogle, googleFromIDPDTO)(rr, req)
	s.Equal(http.StatusBadRequest, rr.Code)
}

func (s *HandlerTestSuite) TestGetServiceErrorNotFound() {
	s.mockIDP.On("GetIdentityProvider", mock.Anything, "missing").
		Return((*providers.IDPDTO)(nil), &idp.ErrorIDPNotFound)

	req := httptest.NewRequest(http.MethodGet, "/connections/google/missing", nil)
	req.SetPathValue("id", "missing")
	rr := httptest.NewRecorder()
	getHandler(s.handler, providers.IDPTypeGoogle, googleFromIDPDTO)(rr, req)
	s.Equal(http.StatusNotFound, rr.Code)
}

func (s *HandlerTestSuite) TestUpdateEmptyID() {
	req := httptest.NewRequest(http.MethodPut, "/connections/google/", bytes.NewReader(s.googleBody()))
	rr := httptest.NewRecorder()
	updateHandler(s.handler, providers.IDPTypeGoogle, googleToIDPDTO, googleFromIDPDTO)(rr, req)
	s.Equal(http.StatusBadRequest, rr.Code)
}

func (s *HandlerTestSuite) TestUpdateInvalidBody() {
	req := httptest.NewRequest(http.MethodPut, "/connections/google/g-1", bytes.NewReader([]byte("{bad")))
	req.SetPathValue("id", "g-1")
	rr := httptest.NewRecorder()
	updateHandler(s.handler, providers.IDPTypeGoogle, googleToIDPDTO, googleFromIDPDTO)(rr, req)
	s.Equal(http.StatusBadRequest, rr.Code)
}

func (s *HandlerTestSuite) TestListInstancesServiceError() {
	s.mockIDP.On("GetIdentityProviderList", mock.Anything).
		Return(([]idp.BasicIDPDTO)(nil), &tidcommon.InternalServerError)

	req := httptest.NewRequest(http.MethodGet, "/connections/google", nil)
	rr := httptest.NewRecorder()
	s.handler.listInstances(providers.IDPTypeGoogle)(rr, req)
	s.Equal(http.StatusInternalServerError, rr.Code)
}

func (s *HandlerTestSuite) TestDeleteEmptyID() {
	req := httptest.NewRequest(http.MethodDelete, "/connections/google/", nil)
	rr := httptest.NewRecorder()
	s.handler.deleteInstance(providers.IDPTypeGoogle)(rr, req)
	s.Equal(http.StatusBadRequest, rr.Code)
}

func (s *HandlerTestSuite) TestUsagesEmptyID() {
	req := httptest.NewRequest(http.MethodGet, "/connections/google//usages", nil)
	rr := httptest.NewRecorder()
	s.handler.usagesInstance(providers.IDPTypeGoogle)(rr, req)
	s.Equal(http.StatusBadRequest, rr.Code)
	s.mockIDP.AssertNotCalled(s.T(), "GetIDPUsages", mock.Anything, mock.Anything)
}

func (s *HandlerTestSuite) TestUsagesServiceError() {
	s.mockIDP.On("GetIdentityProvider", mock.Anything, "missing").
		Return((*providers.IDPDTO)(nil), &idp.ErrorIDPNotFound)

	req := httptest.NewRequest(http.MethodGet, "/connections/google/missing/usages", nil)
	req.SetPathValue("id", "missing")
	rr := httptest.NewRecorder()
	s.handler.usagesInstance(providers.IDPTypeGoogle)(rr, req)
	s.Equal(http.StatusNotFound, rr.Code)
}

func (s *HandlerTestSuite) TestUsagesSuccess() {
	total := 1
	usages := &resourcedependency.DependenciesResponse{
		TotalResults: &total,
		Count:        1,
		Summary:      map[string]int{"flow": 1},
		Usages: []resourcedependency.ResourceDependency{
			{ResourceType: "flow", ID: "flow-1", DisplayName: "Login Flow", BehaviorOnDelete: "restrict"},
		},
	}
	s.mockIDP.On("GetIdentityProvider", mock.Anything, "g-1").
		Return(&providers.IDPDTO{ID: "g-1", Type: providers.IDPTypeGoogle}, (*tidcommon.ServiceError)(nil))
	s.mockIDP.On("GetIDPUsages", mock.Anything, "g-1").Return(usages, (*tidcommon.ServiceError)(nil))

	req := httptest.NewRequest(http.MethodGet, "/connections/google/g-1/usages", nil)
	req.SetPathValue("id", "g-1")
	rr := httptest.NewRecorder()
	s.handler.usagesInstance(providers.IDPTypeGoogle)(rr, req)

	s.Equal(http.StatusOK, rr.Code)
	var resp resourcedependency.DependenciesResponse
	s.Require().NoError(json.NewDecoder(rr.Body).Decode(&resp))
	s.Require().NotNil(resp.TotalResults)
	s.Equal(1, *resp.TotalResults)
	s.Require().Len(resp.Usages, 1)
	s.Equal("flow-1", resp.Usages[0].ID)
	s.Equal("restrict", resp.Usages[0].BehaviorOnDelete)
}

const stubProvider = ncommon.NotificationProviderTypeTwilio

type smsStubReq struct {
	Name string `json:"name"`
}

type smsStubResp struct {
	ID string `json:"id"`
}

var errStubMapper = errors.New("stub mapper failure")

func stubToDTO(smsStubReq) (*ncommon.NotificationSenderDTO, error) {
	return &ncommon.NotificationSenderDTO{
		Type:     ncommon.NotificationSenderTypeMessage,
		Provider: stubProvider,
	}, nil
}

func stubToDTOErr(smsStubReq) (*ncommon.NotificationSenderDTO, error) {
	return nil, errStubMapper
}

func stubFromDTO(dto ncommon.NotificationSenderDTO) (smsStubResp, error) {
	return smsStubResp{ID: dto.ID}, nil
}

func stubFromDTOErr(ncommon.NotificationSenderDTO) (smsStubResp, error) {
	return smsStubResp{}, errStubMapper
}

// SMSHandlerTestSuite covers the generic SMS connection handlers' branches (decode, mapper and
// service errors, empty-id guards, list/delete) independently of any specific vendor's mappers.
type SMSHandlerTestSuite struct {
	suite.Suite
	handler   *handler
	mockIDP   *idpmock.IDPServiceInterfaceMock
	mockNotif *notificationmock.NotificationSenderMgtSvcInterfaceMock
}

func TestSMSHandlerSuite(t *testing.T) {
	suite.Run(t, new(SMSHandlerTestSuite))
}

func (s *SMSHandlerTestSuite) SetupTest() {
	s.handler, s.mockIDP, s.mockNotif = newConnectionTestHandler(s.T())
}

func (s *SMSHandlerTestSuite) stubBody() []byte {
	body, _ := json.Marshal(smsStubReq{Name: "x"})
	return body
}

func (s *SMSHandlerTestSuite) TestCreateInvalidBody() {
	req := httptest.NewRequest(http.MethodPost, "/connections/twilio", bytes.NewReader([]byte("{bad")))
	rr := httptest.NewRecorder()
	createSenderConnection(s.handler, rr, req, stubToDTO, stubFromDTO)
	s.Equal(http.StatusBadRequest, rr.Code)
}

func (s *SMSHandlerTestSuite) TestCreateToDTOError() {
	req := httptest.NewRequest(http.MethodPost, "/connections/twilio", bytes.NewReader(s.stubBody()))
	rr := httptest.NewRecorder()
	createSenderConnection(s.handler, rr, req, stubToDTOErr, stubFromDTO)
	s.Equal(http.StatusInternalServerError, rr.Code)
}

func (s *SMSHandlerTestSuite) TestCreateServiceError() {
	s.mockNotif.On("CreateSender", mock.Anything, mock.Anything).
		Return((*ncommon.NotificationSenderDTO)(nil), &tidcommon.InternalServerError)

	req := httptest.NewRequest(http.MethodPost, "/connections/twilio", bytes.NewReader(s.stubBody()))
	rr := httptest.NewRecorder()
	createSenderConnection(s.handler, rr, req, stubToDTO, stubFromDTO)
	s.Equal(http.StatusInternalServerError, rr.Code)
}

func (s *SMSHandlerTestSuite) TestCreateFromDTOError() {
	s.mockNotif.On("CreateSender", mock.Anything, mock.Anything).
		Return(&ncommon.NotificationSenderDTO{ID: "tw-1"}, (*tidcommon.ServiceError)(nil))

	req := httptest.NewRequest(http.MethodPost, "/connections/twilio", bytes.NewReader(s.stubBody()))
	rr := httptest.NewRecorder()
	createSenderConnection(s.handler, rr, req, stubToDTO, stubFromDTOErr)
	s.Equal(http.StatusInternalServerError, rr.Code)
}

func (s *SMSHandlerTestSuite) TestGetEmptyID() {
	req := httptest.NewRequest(http.MethodGet, "/connections/twilio/", nil)
	rr := httptest.NewRecorder()
	getSenderConnection(s.handler, rr, req, ncommon.NotificationSenderTypeMessage, stubProvider, stubFromDTO)
	s.Equal(http.StatusBadRequest, rr.Code)
}

func (s *SMSHandlerTestSuite) TestGetServiceError() {
	s.mockNotif.On("GetSender", mock.Anything, "missing").
		Return((*ncommon.NotificationSenderDTO)(nil), &notification.ErrorSenderNotFound)

	req := httptest.NewRequest(http.MethodGet, "/connections/twilio/missing", nil)
	req.SetPathValue("id", "missing")
	rr := httptest.NewRecorder()
	getSenderConnection(s.handler, rr, req, ncommon.NotificationSenderTypeMessage, stubProvider, stubFromDTO)
	s.Equal(http.StatusNotFound, rr.Code)
}

func (s *SMSHandlerTestSuite) TestGetFromDTOError() {
	s.mockNotif.On("GetSender", mock.Anything, "tw-1").Return(&ncommon.NotificationSenderDTO{
		ID: "tw-1", Type: ncommon.NotificationSenderTypeMessage, Provider: stubProvider,
	}, (*tidcommon.ServiceError)(nil))

	req := httptest.NewRequest(http.MethodGet, "/connections/twilio/tw-1", nil)
	req.SetPathValue("id", "tw-1")
	rr := httptest.NewRecorder()
	getSenderConnection(s.handler, rr, req, ncommon.NotificationSenderTypeMessage, stubProvider, stubFromDTOErr)
	s.Equal(http.StatusInternalServerError, rr.Code)
}

func (s *SMSHandlerTestSuite) TestUpdateEmptyID() {
	req := httptest.NewRequest(http.MethodPut, "/connections/twilio/", bytes.NewReader(s.stubBody()))
	rr := httptest.NewRecorder()
	updateSenderConnection(s.handler, rr, req, ncommon.NotificationSenderTypeMessage, stubProvider,
		stubToDTO, stubFromDTO)
	s.Equal(http.StatusBadRequest, rr.Code)
}

func (s *SMSHandlerTestSuite) TestUpdateInvalidBody() {
	req := httptest.NewRequest(http.MethodPut, "/connections/twilio/tw-1", bytes.NewReader([]byte("{bad")))
	req.SetPathValue("id", "tw-1")
	rr := httptest.NewRecorder()
	updateSenderConnection(s.handler, rr, req, ncommon.NotificationSenderTypeMessage, stubProvider,
		stubToDTO, stubFromDTO)
	s.Equal(http.StatusBadRequest, rr.Code)
}

func (s *SMSHandlerTestSuite) TestUpdateToDTOError() {
	req := httptest.NewRequest(http.MethodPut, "/connections/twilio/tw-1", bytes.NewReader(s.stubBody()))
	req.SetPathValue("id", "tw-1")
	rr := httptest.NewRecorder()
	updateSenderConnection(s.handler, rr, req, ncommon.NotificationSenderTypeMessage,
		stubProvider, stubToDTOErr, stubFromDTO)
	s.Equal(http.StatusInternalServerError, rr.Code)
}

func (s *SMSHandlerTestSuite) TestUpdateServiceError() {
	s.mockNotif.On("GetSender", mock.Anything, "tw-1").Return(&ncommon.NotificationSenderDTO{
		ID: "tw-1", Type: ncommon.NotificationSenderTypeMessage, Provider: stubProvider,
	}, (*tidcommon.ServiceError)(nil))
	s.mockNotif.On("UpdateSender", mock.Anything, "tw-1", mock.Anything).
		Return((*ncommon.NotificationSenderDTO)(nil), &tidcommon.InternalServerError)

	req := httptest.NewRequest(http.MethodPut, "/connections/twilio/tw-1", bytes.NewReader(s.stubBody()))
	req.SetPathValue("id", "tw-1")
	rr := httptest.NewRecorder()
	updateSenderConnection(s.handler, rr, req, ncommon.NotificationSenderTypeMessage, stubProvider,
		stubToDTO, stubFromDTO)
	s.Equal(http.StatusInternalServerError, rr.Code)
}

func (s *SMSHandlerTestSuite) TestUpdateFromDTOError() {
	s.mockNotif.On("GetSender", mock.Anything, "tw-1").Return(&ncommon.NotificationSenderDTO{
		ID: "tw-1", Type: ncommon.NotificationSenderTypeMessage, Provider: stubProvider,
	}, (*tidcommon.ServiceError)(nil))
	s.mockNotif.On("UpdateSender", mock.Anything, "tw-1", mock.Anything).
		Return(&ncommon.NotificationSenderDTO{ID: "tw-1"}, (*tidcommon.ServiceError)(nil))

	req := httptest.NewRequest(http.MethodPut, "/connections/twilio/tw-1", bytes.NewReader(s.stubBody()))
	req.SetPathValue("id", "tw-1")
	rr := httptest.NewRecorder()
	updateSenderConnection(s.handler, rr, req, ncommon.NotificationSenderTypeMessage,
		stubProvider, stubToDTO, stubFromDTOErr)
	s.Equal(http.StatusInternalServerError, rr.Code)
}

func (s *SMSHandlerTestSuite) TestListInstancesServiceError() {
	s.mockNotif.On("ListSendersByType", mock.Anything, ncommon.NotificationSenderTypeMessage).
		Return(([]ncommon.NotificationSenderDTO)(nil), &tidcommon.InternalServerError)

	req := httptest.NewRequest(http.MethodGet, "/connections/twilio", nil)
	rr := httptest.NewRecorder()
	s.handler.listSenderInstances(ncommon.NotificationSenderTypeMessage, stubProvider)(rr, req)
	s.Equal(http.StatusInternalServerError, rr.Code)
}

func (s *SMSHandlerTestSuite) TestListInstancesSuccess() {
	s.mockNotif.On("ListSendersByType", mock.Anything, ncommon.NotificationSenderTypeMessage).
		Return([]ncommon.NotificationSenderDTO{
			{
				ID: "tw-1", Name: "A", Description: "d",
				Type: ncommon.NotificationSenderTypeMessage, Provider: stubProvider,
			},
			{ID: "vo-1", Type: ncommon.NotificationSenderTypeMessage, Provider: ncommon.NotificationProviderTypeVonage},
		}, (*tidcommon.ServiceError)(nil))

	req := httptest.NewRequest(http.MethodGet, "/connections/twilio", nil)
	rr := httptest.NewRecorder()
	s.handler.listSenderInstances(ncommon.NotificationSenderTypeMessage, stubProvider)(rr, req)

	s.Equal(http.StatusOK, rr.Code)
	var summaries []connectionInstanceSummary
	s.Require().NoError(json.NewDecoder(rr.Body).Decode(&summaries))
	s.Require().Len(summaries, 1)
	s.Equal("tw-1", summaries[0].ID)
	s.Equal("A", summaries[0].Name)
}

func (s *SMSHandlerTestSuite) TestDeleteEmptyID() {
	req := httptest.NewRequest(http.MethodDelete, "/connections/twilio/", nil)
	rr := httptest.NewRecorder()
	s.handler.deleteSenderInstance(ncommon.NotificationSenderTypeMessage, stubProvider)(rr, req)
	s.Equal(http.StatusBadRequest, rr.Code)
}

func (s *SMSHandlerTestSuite) TestDeleteMissingReturnsNoContent() {
	s.mockNotif.On("GetSender", mock.Anything, "missing").
		Return((*ncommon.NotificationSenderDTO)(nil), &notification.ErrorSenderNotFound)

	req := httptest.NewRequest(http.MethodDelete, "/connections/twilio/missing", nil)
	req.SetPathValue("id", "missing")
	rr := httptest.NewRecorder()
	s.handler.deleteSenderInstance(ncommon.NotificationSenderTypeMessage, stubProvider)(rr, req)
	s.Equal(http.StatusNoContent, rr.Code)
	s.mockNotif.AssertNotCalled(s.T(), "DeleteSender", mock.Anything, mock.Anything)
}

func (s *SMSHandlerTestSuite) TestDeleteServiceError() {
	s.mockNotif.On("GetSender", mock.Anything, "tw-1").
		Return((*ncommon.NotificationSenderDTO)(nil), &tidcommon.InternalServerError)

	req := httptest.NewRequest(http.MethodDelete, "/connections/twilio/tw-1", nil)
	req.SetPathValue("id", "tw-1")
	rr := httptest.NewRecorder()
	s.handler.deleteSenderInstance(ncommon.NotificationSenderTypeMessage, stubProvider)(rr, req)
	s.Equal(http.StatusInternalServerError, rr.Code)
}

func (s *SMSHandlerTestSuite) TestDeleteSuccess() {
	s.mockNotif.On("GetSender", mock.Anything, "tw-1").Return(&ncommon.NotificationSenderDTO{
		ID: "tw-1", Type: ncommon.NotificationSenderTypeMessage, Provider: stubProvider,
	}, (*tidcommon.ServiceError)(nil))
	s.mockNotif.On("DeleteSender", mock.Anything, "tw-1").Return((*tidcommon.ServiceError)(nil))

	req := httptest.NewRequest(http.MethodDelete, "/connections/twilio/tw-1", nil)
	req.SetPathValue("id", "tw-1")
	rr := httptest.NewRecorder()
	s.handler.deleteSenderInstance(ncommon.NotificationSenderTypeMessage, stubProvider)(rr, req)
	s.Equal(http.StatusNoContent, rr.Code)
}

func (s *SMSHandlerTestSuite) TestUsagesEmptyID() {
	req := httptest.NewRequest(http.MethodGet, "/connections/twilio//usages", nil)
	rr := httptest.NewRecorder()
	s.handler.usagesSenderInstance(ncommon.NotificationSenderTypeMessage, stubProvider)(rr, req)
	s.Equal(http.StatusBadRequest, rr.Code)
	s.mockNotif.AssertNotCalled(s.T(), "GetSenderUsages", mock.Anything, mock.Anything)
}

func (s *SMSHandlerTestSuite) TestUsagesServiceError() {
	s.mockNotif.On("GetSender", mock.Anything, "missing").
		Return((*ncommon.NotificationSenderDTO)(nil), &notification.ErrorSenderNotFound)

	req := httptest.NewRequest(http.MethodGet, "/connections/twilio/missing/usages", nil)
	req.SetPathValue("id", "missing")
	rr := httptest.NewRecorder()
	s.handler.usagesSenderInstance(ncommon.NotificationSenderTypeMessage, stubProvider)(rr, req)
	s.Equal(http.StatusNotFound, rr.Code)
}

func (s *SMSHandlerTestSuite) TestUsagesSuccess() {
	total := 1
	usages := &resourcedependency.DependenciesResponse{
		TotalResults: &total,
		Count:        1,
		Summary:      map[string]int{"flow": 1},
		Usages: []resourcedependency.ResourceDependency{
			{ResourceType: "flow", ID: "flow-1", DisplayName: "SMS OTP", BehaviorOnDelete: "restrict"},
		},
	}
	s.mockNotif.On("GetSender", mock.Anything, "tw-1").Return(&ncommon.NotificationSenderDTO{
		ID: "tw-1", Type: ncommon.NotificationSenderTypeMessage, Provider: stubProvider,
	}, (*tidcommon.ServiceError)(nil))
	s.mockNotif.On("GetSenderUsages", mock.Anything, "tw-1").Return(usages, (*tidcommon.ServiceError)(nil))

	req := httptest.NewRequest(http.MethodGet, "/connections/twilio/tw-1/usages", nil)
	req.SetPathValue("id", "tw-1")
	rr := httptest.NewRecorder()
	s.handler.usagesSenderInstance(ncommon.NotificationSenderTypeMessage, stubProvider)(rr, req)

	s.Equal(http.StatusOK, rr.Code)
	var resp resourcedependency.DependenciesResponse
	s.Require().NoError(json.NewDecoder(rr.Body).Decode(&resp))
	s.Require().NotNil(resp.TotalResults)
	s.Equal(1, *resp.TotalResults)
	s.Require().Len(resp.Usages, 1)
	s.Equal("flow-1", resp.Usages[0].ID)
	s.Equal("restrict", resp.Usages[0].BehaviorOnDelete)
}

const testHTTPConnectionPassword = "risk-engine-password"

type HTTPConnectionHandlerTestSuite struct {
	suite.Suite
	mux      *http.ServeMux
	mockHTTP *httpconnectionmock.HTTPConnectionServiceInterfaceMock
}

func TestHTTPConnectionHandlerSuite(t *testing.T) {
	suite.Run(t, new(HTTPConnectionHandlerTestSuite))
}

func (s *HTTPConnectionHandlerTestSuite) SetupTest() {
	initConfigWithTestCryptoKey(s.T())
	s.T().Cleanup(config.ResetServerRuntime)
	s.mockHTTP = httpconnectionmock.NewHTTPConnectionServiceInterfaceMock(s.T())
	h := newHandler(newService(idpmock.NewIDPServiceInterfaceMock(s.T()),
		notificationmock.NewNotificationSenderMgtSvcInterfaceMock(s.T()), &testResourceServerLister{},
		newAuthZENPDPServiceMock(s.T()), s.mockHTTP))
	s.mux = http.NewServeMux()
	registerRoutes(s.mux, h)
}

func (s *HTTPConnectionHandlerTestSuite) basicConnection() *httpconnection.HTTPConnection {
	auth, err := outboundauth.ToProperties(outboundauth.Config{Type: outboundauth.TypeBasic,
		Properties: map[string]string{outboundauth.FieldBasicUsername: "svc-user",
			outboundauth.FieldBasicPassword: testHTTPConnectionPassword}})
	s.Require().NoError(err)
	return &httpconnection.HTTPConnection{
		ID: "http-1", Name: "Risk", Description: "Risk engine", OUID: "ou-1",
		URL: "https://risk.example.com/evaluate", TimeoutMS: 5000,
		Headers:        []httpconnection.Header{{Name: "X-Tenant", Value: "acme"}},
		AuthProperties: auth,
	}
}

func (s *HTTPConnectionHandlerTestSuite) serve(method, path, body string) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, httptest.NewRequest(method, path, strings.NewReader(body)))
	return rr
}

func (s *HTTPConnectionHandlerTestSuite) errorCode(rr *httptest.ResponseRecorder) string {
	var resp apierror.ErrorResponse
	s.Require().NoError(json.NewDecoder(rr.Body).Decode(&resp))
	return resp.Code
}

func (s *HTTPConnectionHandlerTestSuite) TestCreateMapsAuthenticationAndMasksTheResponse() {
	var received httpconnection.HTTPConnection
	s.mockHTTP.On("CreateHTTPConnection", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { received = args.Get(1).(httpconnection.HTTPConnection) }).
		Return(s.basicConnection(), (*tidcommon.ServiceError)(nil))

	rr := s.serve(http.MethodPost, "/connections/http", `{"name":"Risk","description":"Risk engine",
		"url":"https://risk.example.com/evaluate","timeoutMs":5000,"headers":[{"name":"X-Tenant","value":"acme"}],
		"authentication":{"type":"basic","properties":{"username":"svc-user","password":"`+
		testHTTPConnectionPassword+`"}}}`)

	s.Equal(http.StatusCreated, rr.Code)
	s.NotContains(rr.Body.String(), testHTTPConnectionPassword)
	s.JSONEq(`{"id":"http-1","name":"Risk","description":"Risk engine","ouId":"ou-1","type":"http",
		"url":"https://risk.example.com/evaluate","timeoutMs":5000,
		"headers":[{"name":"X-Tenant","value":"acme"}],
		"authentication":{"type":"basic","properties":{"username":"svc-user","password":"******"}}}`,
		rr.Body.String())
	s.Equal("https://risk.example.com/evaluate", received.URL)
	s.Equal([]httpconnection.Header{{Name: "X-Tenant", Value: "acme"}}, received.Headers)
	authConfig, err := outboundauth.FromProperties(received.AuthProperties)
	s.Require().NoError(err)
	s.Equal(outboundauth.Config{Type: outboundauth.TypeBasic, Properties: map[string]string{
		outboundauth.FieldBasicUsername: "svc-user", outboundauth.FieldBasicPassword: testHTTPConnectionPassword,
	}}, authConfig)
}

func (s *HTTPConnectionHandlerTestSuite) TestOmittedAuthenticationIsNone() {
	none := &httpconnection.HTTPConnection{ID: "http-1", Name: "Risk", OUID: "ou-1", URL: "http://localhost:9000",
		TimeoutMS: 10000}
	none.AuthProperties, _ = outboundauth.ToProperties(outboundauth.Config{Type: outboundauth.TypeNone})
	s.mockHTTP.On("CreateHTTPConnection", mock.Anything, mock.MatchedBy(func(c httpconnection.HTTPConnection) bool {
		authConfig, err := outboundauth.FromProperties(c.AuthProperties)
		return err == nil && authConfig.Type == outboundauth.TypeNone
	})).Return(none, (*tidcommon.ServiceError)(nil))

	rr := s.serve(http.MethodPost, "/connections/http", `{"name":"Risk","url":"http://localhost:9000"}`)

	s.Equal(http.StatusCreated, rr.Code)
	s.JSONEq(`{"id":"http-1","name":"Risk","ouId":"ou-1","type":"http","url":"http://localhost:9000","timeoutMs":10000,
		"headers":[],"authentication":{"type":"none"}}`, rr.Body.String())
}

func (s *HTTPConnectionHandlerTestSuite) TestUnsupportedAuthenticationTypeIsRejected() {
	rr := s.serve(http.MethodPost, "/connections/http",
		`{"name":"Risk","url":"https://risk.example.com","authentication":{"type":"digest"}}`)

	s.Equal(http.StatusBadRequest, rr.Code)
	s.Equal(ErrorInvalidAuthenticationType.Code, s.errorCode(rr))
}

func (s *HTTPConnectionHandlerTestSuite) TestGetMasksTheSecret() {
	s.mockHTTP.On("GetHTTPConnection", mock.Anything, "http-1").
		Return(s.basicConnection(), (*tidcommon.ServiceError)(nil))

	rr := s.serve(http.MethodGet, "/connections/http/http-1", "")

	s.Equal(http.StatusOK, rr.Code)
	s.NotContains(rr.Body.String(), testHTTPConnectionPassword)
	s.Contains(rr.Body.String(), `"password":"******"`)
}

func (s *HTTPConnectionHandlerTestSuite) TestUpdateReturnsTheMaskedConnection() {
	s.mockHTTP.On("GetHTTPConnection", mock.Anything, "http-1").
		Return(s.basicConnection(), (*tidcommon.ServiceError)(nil))
	s.mockHTTP.On("UpdateHTTPConnection", mock.Anything, "http-1", mock.Anything).
		Return(s.basicConnection(), (*tidcommon.ServiceError)(nil))

	rr := s.serve(http.MethodPut, "/connections/http/http-1", `{"name":"Risk","url":"https://risk.example.com"}`)

	s.Equal(http.StatusOK, rr.Code)
	s.NotContains(rr.Body.String(), testHTTPConnectionPassword)
}

// A client that sends a read back unchanged sends the mask; it is refused on both routes, before the
// stored connection is touched (AC2.11).
func (s *HTTPConnectionHandlerTestSuite) TestMaskedPasswordOrTokenIsRefused() {
	for _, auth := range []string{
		`{"type":"basic","properties":{"username":"svc-user","password":"******"}}`,
		`{"type":"bearer","properties":{"token":"******"}}`,
	} {
		for _, route := range []struct{ method, path string }{
			{http.MethodPost, "/connections/http"}, {http.MethodPut, "/connections/http/http-1"},
		} {
			rr := s.serve(route.method, route.path,
				`{"name":"Risk","url":"https://risk.example.com/evaluate","authentication":`+auth+`}`)

			s.Equal(http.StatusBadRequest, rr.Code, route.method+" "+auth)
			s.Equal(httpconnection.ErrorInvalidAuthentication.Code, s.errorCode(rr), route.method+" "+auth)
		}
	}
	s.mockHTTP.AssertNotCalled(s.T(), "GetHTTPConnection", mock.Anything, mock.Anything)
	s.mockHTTP.AssertNotCalled(s.T(), "UpdateHTTPConnection", mock.Anything, mock.Anything, mock.Anything)
	s.mockHTTP.AssertNotCalled(s.T(), "CreateHTTPConnection", mock.Anything, mock.Anything)
}

// An update that omits the password, at the same origin and for the same user, keeps the stored one
// through the REST route, so the route must map the body as an update rather than as a create.
func (s *HTTPConnectionHandlerTestSuite) TestUpdateKeepsAnOmittedPassword() {
	s.mockHTTP.On("GetHTTPConnection", mock.Anything, "http-1").
		Return(s.basicConnection(), (*tidcommon.ServiceError)(nil))
	var received httpconnection.HTTPConnection
	s.mockHTTP.On("UpdateHTTPConnection", mock.Anything, "http-1", mock.Anything).
		Run(func(args mock.Arguments) { received = args.Get(2).(httpconnection.HTTPConnection) }).
		Return(s.basicConnection(), (*tidcommon.ServiceError)(nil))

	rr := s.serve(http.MethodPut, "/connections/http/http-1", `{"name":"Risk",
		"url":"https://risk.example.com/v2","authentication":{"type":"basic","properties":{"username":"svc-user"}}}`)

	s.Require().Equal(http.StatusOK, rr.Code, rr.Body.String())
	authConfig, err := outboundauth.FromProperties(received.AuthProperties)
	s.Require().NoError(err)
	s.Equal(testHTTPConnectionPassword, authConfig.Get(outboundauth.FieldBasicPassword))
}

func (s *HTTPConnectionHandlerTestSuite) TestDelete() {
	s.mockHTTP.On("GetHTTPConnection", mock.Anything, "http-1").
		Return(&httpconnection.HTTPConnection{ID: "http-1"}, (*tidcommon.ServiceError)(nil))
	s.mockHTTP.On("DeleteHTTPConnection", mock.Anything, "http-1").Return((*tidcommon.ServiceError)(nil))

	s.Equal(http.StatusNoContent, s.serve(http.MethodDelete, "/connections/http/http-1", "").Code)
}

func (s *HTTPConnectionHandlerTestSuite) TestDeleteOfAConnectionInUseIsAConflict() {
	s.mockHTTP.On("GetHTTPConnection", mock.Anything, "http-1").
		Return(&httpconnection.HTTPConnection{ID: "http-1"}, (*tidcommon.ServiceError)(nil))
	s.mockHTTP.On("DeleteHTTPConnection", mock.Anything, "http-1").
		Return(&httpconnection.ErrorHasBlockingDependencies)

	rr := s.serve(http.MethodDelete, "/connections/http/http-1", "")

	s.Equal(http.StatusConflict, rr.Code)
	s.Equal(httpconnection.ErrorHasBlockingDependencies.Code, s.errorCode(rr))
}

func (s *HTTPConnectionHandlerTestSuite) TestUsagesListTheReferencingFlows() {
	total := 1
	s.mockHTTP.On("GetHTTPConnectionUsages", mock.Anything, "http-1").
		Return(&resourcedependency.DependenciesResponse{
			TotalResults: &total, Count: 1, Summary: map[string]int{resourcedependency.ResourceTypeFlow: 1},
			Usages: []resourcedependency.ResourceDependency{{
				ResourceType: resourcedependency.ResourceTypeFlow, ID: "flow-1", DisplayName: "Default sign-in",
				BehaviorOnDelete: resourcedependency.BehaviorRestrict,
			}},
		}, (*tidcommon.ServiceError)(nil))

	rr := s.serve(http.MethodGet, "/connections/http/http-1/usages", "")

	s.Equal(http.StatusOK, rr.Code)
	s.JSONEq(`{"totalResults":1,"count":1,"summary":{"flow":1},"usages":[{"resourceType":"flow",
		"id":"flow-1","displayName":"Default sign-in","behaviorOnDelete":"restrict"}]}`, rr.Body.String())
}

func (s *HTTPConnectionHandlerTestSuite) TestUsagesOfAMissingConnectionIsNotFound() {
	s.mockHTTP.On("GetHTTPConnectionUsages", mock.Anything, "missing").
		Return((*resourcedependency.DependenciesResponse)(nil), &httpconnection.ErrorNotFound)

	rr := s.serve(http.MethodGet, "/connections/http/missing/usages", "")

	s.Equal(http.StatusNotFound, rr.Code)
	s.Equal(httpconnection.ErrorNotFound.Code, s.errorCode(rr))
}

func (s *HTTPConnectionHandlerTestSuite) TestListReturnsSummaries() {
	s.mockHTTP.On("ListHTTPConnections", mock.Anything).
		Return([]httpconnection.HTTPConnection{*s.basicConnection()}, (*tidcommon.ServiceError)(nil))

	rr := s.serve(http.MethodGet, "/connections/http", "")

	s.Equal(http.StatusOK, rr.Code)
	s.JSONEq(`[{"id":"http-1","name":"Risk","description":"Risk engine"}]`, rr.Body.String())
}

func (s *HTTPConnectionHandlerTestSuite) TestServiceErrorsMapToStatus() {
	cases := []struct {
		err  tidcommon.ServiceError
		want int
	}{
		{httpconnection.ErrorAlreadyExists, http.StatusConflict},
		{httpconnection.ErrorHasBlockingDependencies, http.StatusConflict},
		{httpconnection.ErrorInvalidName, http.StatusBadRequest},
		{httpconnection.ErrorInvalidDescription, http.StatusBadRequest},
		{httpconnection.ErrorInvalidURL, http.StatusBadRequest},
		{httpconnection.ErrorInvalidTimeout, http.StatusBadRequest},
		{httpconnection.ErrorInvalidHeaders, http.StatusBadRequest},
		{httpconnection.ErrorInvalidAuthentication, http.StatusBadRequest},
		{tidcommon.InternalServerError, http.StatusInternalServerError},
	}
	for _, tc := range cases {
		s.Run(tc.err.Code, func() {
			s.SetupTest()
			s.mockHTTP.On("CreateHTTPConnection", mock.Anything, mock.Anything).
				Return((*httpconnection.HTTPConnection)(nil), &tc.err)

			rr := s.serve(http.MethodPost, "/connections/http", `{"name":"Risk"}`)

			s.Equal(tc.want, rr.Code)
			s.Equal(tc.err.Code, s.errorCode(rr))
		})
	}
}

func (s *HTTPConnectionHandlerTestSuite) TestMissingConnectionIsNotFound() {
	s.mockHTTP.On("GetHTTPConnection", mock.Anything, "missing").
		Return((*httpconnection.HTTPConnection)(nil), &httpconnection.ErrorNotFound)

	for _, method := range []string{http.MethodGet, http.MethodPut} {
		rr := s.serve(method, "/connections/http/missing", `{"name":"Risk"}`)
		s.Equal(http.StatusNotFound, rr.Code, method)
		s.Equal(httpconnection.ErrorNotFound.Code, s.errorCode(rr), method)
	}
}

func (s *HTTPConnectionHandlerTestSuite) TestMalformedBodyIsRejected() {
	for _, path := range []string{"/connections/http", "/connections/http/http-1"} {
		method := http.MethodPost
		if path != "/connections/http" {
			method = http.MethodPut
		}
		rr := s.serve(method, path, `{"name":`)

		s.Equal(http.StatusBadRequest, rr.Code, method)
		s.Equal(ErrorInvalidRequestFormat.Code, s.errorCode(rr), method)
	}
}

func (s *HTTPConnectionHandlerTestSuite) TestConnectionListingFiltersByHTTPCategory() {
	s.mockHTTP.On("ListHTTPConnections", mock.Anything).
		Return([]httpconnection.HTTPConnection{*s.basicConnection()}, (*tidcommon.ServiceError)(nil))

	rr := s.serve(http.MethodGet, "/connections?category=http", "")

	s.Equal(http.StatusOK, rr.Code)
	var resp connectionListResponse
	s.Require().NoError(json.NewDecoder(rr.Body).Decode(&resp))
	s.Equal([]connectionInstance{{ID: "http-1", Name: "Risk", Description: "Risk engine", Type: httpVendorName,
		OUID: "ou-1", Categories: []connectionCategory{categoryHTTP}, IsOwnedByOU: new(true)}}, resp.Connections)
}

func (s *HTTPConnectionHandlerTestSuite) TestConnectionListingRefusesAFilterItCannotApply() {
	for _, expr := range []string{
		"ouId eq ou-1",
		`ouId eq ""`,
		`ouId sw "ou"`,
		`name eq "x"`,
		`ouId eq "ou-1" and name eq "x"`,
	} {
		rr := s.serve(http.MethodGet, "/connections?filter="+url.QueryEscape(expr), "")

		s.Equal(http.StatusBadRequest, rr.Code, expr)
		s.Contains(rr.Body.String(), ErrorInvalidFilter.Code, expr)
	}
}

func (s *HTTPConnectionHandlerTestSuite) TestConnectionListingFailsWhenHTTPConnectionsCannotBeListed() {
	s.mockHTTP.On("ListHTTPConnections", mock.Anything).
		Return(([]httpconnection.HTTPConnection)(nil), &tidcommon.InternalServerError)

	s.Equal(http.StatusInternalServerError, s.serve(http.MethodGet, "/connections?category=http", "").Code)
}
