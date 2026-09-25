// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package dcr

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/security"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/testhelpers"
)

const (
	testClientID   = "test-client-id"
	testConfigPath = "/oauth2/dcr/register/" + testClientID
)

// ClientConfigurationHandlerTestSuite covers the RFC 7592 client configuration endpoint handlers.
type ClientConfigurationHandlerTestSuite struct {
	suite.Suite
	mockService *DCRServiceInterfaceMock
	handler     *dcrHandler
}

func TestClientConfigurationHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(ClientConfigurationHandlerTestSuite))
}

func (s *ClientConfigurationHandlerTestSuite) SetupTest() {
	s.mockService = NewDCRServiceInterfaceMock(s.T())
	_ = config.InitializeServerRuntime("test", &config.Config{
		OAuth: config.OAuthConfig{DCR: engineconfig.DCRConfig{Insecure: true}},
	})
	security.InitSystemPermissions("")
	s.handler = newDCRHandler(s.mockService, testhelpers.OAuthConfig())
}

func (s *ClientConfigurationHandlerTestSuite) TearDownTest() {
	config.ResetServerRuntime()
	security.InitSystemPermissions("")
}

// newRequest builds a client configuration request authorized by an administrative caller, with the
// client_id path value populated as the router would.
func (s *ClientConfigurationHandlerTestSuite) newRequest(
	method string, body []byte) *http.Request {
	return s.newRequestWithPermissions(method, body, []string{"system"})
}

// newRequestWithPermissions builds a client configuration request whose caller holds the given
// permissions, so a test can exercise an unprivileged or unauthenticated caller.
func (s *ClientConfigurationHandlerTestSuite) newRequestWithPermissions(
	method string, body []byte, permissions []string) *http.Request {
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, testConfigPath, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, testConfigPath, nil)
	}
	if permissions != nil {
		secCtx := security.NewSecurityContextForTest("admin", "ou1", "tok", permissions, nil)
		req = req.WithContext(security.WithSecurityContextTest(context.Background(), secCtx))
	}
	req.SetPathValue("client_id", testClientID)
	return req
}

// UC-2: an administrative caller reads a registration.
func (s *ClientConfigurationHandlerTestSuite) TestGet_ReturnsRegistration() {
	s.mockService.On("GetClient", mock.Anything, testClientID).
		Return(&DCRRegistrationResponse{ClientID: testClientID, ClientName: "Test Client"},
			(*tidcommon.ServiceError)(nil))

	rr := httptest.NewRecorder()
	s.handler.HandleGetClientConfiguration(rr, s.newRequest(http.MethodGet, nil))

	s.Equal(http.StatusOK, rr.Code)
	var response DCRRegistrationResponse
	s.Require().NoError(json.Unmarshal(rr.Body.Bytes(), &response))
	s.Equal(testClientID, response.ClientID)
	s.Empty(response.ClientSecret, "the client secret is not readable and must not be returned")
}

// A caller holding no system permission is rejected, and the registration is not exposed.
func (s *ClientConfigurationHandlerTestSuite) TestGet_WithoutSystemPermissionIsUnauthorized() {
	rr := httptest.NewRecorder()
	s.handler.HandleGetClientConfiguration(rr,
		s.newRequestWithPermissions(http.MethodGet, nil, []string{"some:other:scope"}))

	s.Equal(http.StatusUnauthorized, rr.Code)
	s.Equal(wwwAuthenticateInvalidToken, rr.Header().Get(wwwAuthenticateHeaderName))
	s.mockService.AssertNotCalled(s.T(), "GetClient", mock.Anything, mock.Anything)
}

// An unauthenticated request carries no security context at all, and is rejected.
func (s *ClientConfigurationHandlerTestSuite) TestGet_UnauthenticatedIsUnauthorized() {
	rr := httptest.NewRecorder()
	s.handler.HandleGetClientConfiguration(rr, s.newRequestWithPermissions(http.MethodGet, nil, nil))

	s.Equal(http.StatusUnauthorized, rr.Code)
	s.mockService.AssertNotCalled(s.T(), "GetClient", mock.Anything, mock.Anything)
}

// A deleted client no longer resolves, so managing it reports not found.
func (s *ClientConfigurationHandlerTestSuite) TestGet_UnknownClientIsNotFound() {
	s.mockService.On("GetClient", mock.Anything, testClientID).
		Return((*DCRRegistrationResponse)(nil), &ErrorClientNotFound)

	rr := httptest.NewRecorder()
	s.handler.HandleGetClientConfiguration(rr, s.newRequest(http.MethodGet, nil))

	s.Equal(http.StatusNotFound, rr.Code)
}

// UC-3: a client updates its registered metadata.
func (s *ClientConfigurationHandlerTestSuite) TestPut_UpdatesRegistration() {
	s.mockService.On("UpdateClient", mock.Anything, testClientID,
		mock.AnythingOfType("*dcr.DCRRegistrationRequest")).
		Return(&DCRRegistrationResponse{ClientID: testClientID, ClientName: "Renamed"},
			(*tidcommon.ServiceError)(nil))

	body := []byte(`{"client_id":"` + testClientID + `","client_name":"Renamed",
		"redirect_uris":["https://client.example.com/cb"]}`)
	rr := httptest.NewRecorder()
	s.handler.HandleUpdateClientConfiguration(rr, s.newRequest(http.MethodPut, body))

	s.Equal(http.StatusOK, rr.Code)
	var response DCRRegistrationResponse
	s.Require().NoError(json.Unmarshal(rr.Body.Bytes(), &response))
	s.Equal(testClientID, response.ClientID, "the client_id must be preserved across an update")
	s.Equal("Renamed", response.ClientName)
}

// A client_id in the body that contradicts the path is rejected.
func (s *ClientConfigurationHandlerTestSuite) TestPut_ClientIDMismatchIsRejected() {
	body := []byte(`{"client_id":"some-other-client","redirect_uris":["https://client.example.com/cb"]}`)
	rr := httptest.NewRecorder()
	s.handler.HandleUpdateClientConfiguration(rr, s.newRequest(http.MethodPut, body))

	s.Equal(http.StatusBadRequest, rr.Code)
	s.mockService.AssertNotCalled(s.T(), "UpdateClient", mock.Anything, mock.Anything, mock.Anything)
}

// RFC 7592 section 2.2 requires the update request to carry its client_id, so omitting it is
// rejected the same way a mismatched one is.
func (s *ClientConfigurationHandlerTestSuite) TestPut_MissingClientIDIsRejected() {
	body := []byte(`{"redirect_uris":["https://client.example.com/cb"]}`)
	rr := httptest.NewRecorder()
	s.handler.HandleUpdateClientConfiguration(rr, s.newRequest(http.MethodPut, body))

	s.Equal(http.StatusBadRequest, rr.Code)
	s.mockService.AssertNotCalled(s.T(), "UpdateClient", mock.Anything, mock.Anything, mock.Anything)
}

// A malformed body is rejected before reaching the service.
func (s *ClientConfigurationHandlerTestSuite) TestPut_InvalidBodyIsRejected() {
	rr := httptest.NewRecorder()
	s.handler.HandleUpdateClientConfiguration(rr,
		s.newRequest(http.MethodPut, []byte(`{"invalid": json}`)))

	s.Equal(http.StatusBadRequest, rr.Code)
	s.mockService.AssertNotCalled(s.T(), "UpdateClient", mock.Anything, mock.Anything, mock.Anything)
}

// Invalid client metadata on update is surfaced as an RFC 7592 client error.
func (s *ClientConfigurationHandlerTestSuite) TestPut_InvalidMetadataIsRejected() {
	s.mockService.On("UpdateClient", mock.Anything, testClientID,
		mock.AnythingOfType("*dcr.DCRRegistrationRequest")).
		Return((*DCRRegistrationResponse)(nil), &ErrorInvalidRedirectURI)

	body := []byte(`{"client_id":"test-client-id","redirect_uris":["not-a-uri"]}`)
	rr := httptest.NewRecorder()
	s.handler.HandleUpdateClientConfiguration(rr, s.newRequest(http.MethodPut, body))

	s.Equal(http.StatusBadRequest, rr.Code)
	var errResponse DCRErrorResponse
	s.Require().NoError(json.Unmarshal(rr.Body.Bytes(), &errResponse))
	s.Equal(ErrorInvalidRedirectURI.Code, errResponse.Error)
}

// UC-4: a client deletes its registration.
func (s *ClientConfigurationHandlerTestSuite) TestDelete_RemovesRegistration() {
	s.mockService.On("DeleteClient", mock.Anything, testClientID).
		Return((*tidcommon.ServiceError)(nil))

	rr := httptest.NewRecorder()
	s.handler.HandleDeleteClientConfiguration(rr, s.newRequest(http.MethodDelete, nil))

	s.Equal(http.StatusNoContent, rr.Code)
	s.Empty(rr.Body.Bytes())
}

// Deleting an unknown client reports not found.
func (s *ClientConfigurationHandlerTestSuite) TestDelete_UnknownClientIsNotFound() {
	s.mockService.On("DeleteClient", mock.Anything, testClientID).Return(&ErrorClientNotFound)

	rr := httptest.NewRecorder()
	s.handler.HandleDeleteClientConfiguration(rr, s.newRequest(http.MethodDelete, nil))

	s.Equal(http.StatusNotFound, rr.Code)
}

// A server failure during deletion is reported as a server error.
func (s *ClientConfigurationHandlerTestSuite) TestDelete_ServerErrorIsReported() {
	s.mockService.On("DeleteClient", mock.Anything, testClientID).Return(&ErrorServerError)

	rr := httptest.NewRecorder()
	s.handler.HandleDeleteClientConfiguration(rr, s.newRequest(http.MethodDelete, nil))

	s.Equal(http.StatusInternalServerError, rr.Code)
}

// A request without a client ID in the path cannot identify a registration.
func (s *ClientConfigurationHandlerTestSuite) TestGet_MissingClientIDIsNotFound() {
	req := httptest.NewRequest(http.MethodGet, "/oauth2/dcr/register/", nil)
	secCtx := security.NewSecurityContextForTest("admin", "ou1", "tok", []string{"system"}, nil)
	req = req.WithContext(security.WithSecurityContextTest(context.Background(), secCtx))

	rr := httptest.NewRecorder()
	s.handler.HandleGetClientConfiguration(rr, req)

	s.Equal(http.StatusNotFound, rr.Code)
}
