// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package dcr

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/tests/testhelpers"
)

const (
	testClientID   = "test-client-id"
	testConfigPath = "/oauth2/dcr/register/" + testClientID
)

// DCRHandlerTestSuite is the test suite for DCR handler
type DCRHandlerTestSuite struct {
	suite.Suite
	mockService *DCRServiceInterfaceMock
	handler     *dcrHandler
}

func TestDCRHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(DCRHandlerTestSuite))
}

func (s *DCRHandlerTestSuite) SetupTest() {
	s.mockService = NewDCRServiceInterfaceMock(s.T())
	_ = config.InitializeServerRuntime("test", &config.Config{
		OAuth: config.OAuthConfig{DCR: engineconfig.DCRConfig{Insecure: true}},
	})
	security.InitSystemPermissions("")
	cfg := testhelpers.OAuthConfig()
	cfg.OAuth.DCR.Insecure = true
	s.handler = newDCRHandler(s.mockService, cfg)
}

func (s *DCRHandlerTestSuite) TearDownTest() {
	config.ResetServerRuntime()
	security.InitSystemPermissions("")
}

// TestHandleDCRRegistration_InvalidRequestFormat tests handling of invalid JSON in request body
func (s *DCRHandlerTestSuite) TestHandleDCRRegistration_InvalidRequestFormat() {
	// Create a request with invalid JSON
	invalidJSON := `{"invalid": json}`
	req := httptest.NewRequest(http.MethodPost, "/oauth2/dcr", bytes.NewReader([]byte(invalidJSON)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	s.handler.HandleDCRRegistration(rr, req)

	assert.Equal(s.T(), http.StatusBadRequest, rr.Code)
	var errorResponse map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &errorResponse)
	s.NoError(err)
	assert.Contains(s.T(), errorResponse, "error")
}

// TestHandleDCRRegistration_ServiceError tests handling of service errors
func (s *DCRHandlerTestSuite) TestHandleDCRRegistration_ServiceError() {
	request := &DCRRegistrationRequest{
		RedirectURIs: []string{"https://client.example.com/callback"},
		GrantTypes:   []providers.GrantType{providers.GrantTypeAuthorizationCode},
	}

	serviceErr := &ErrorInvalidRedirectURI
	s.mockService.On("RegisterClient", mock.Anything, request).Return(nil, serviceErr)

	requestJSON, _ := json.Marshal(request)
	req := httptest.NewRequest(http.MethodPost, "/oauth2/dcr", bytes.NewReader(requestJSON))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	s.handler.HandleDCRRegistration(rr, req)

	assert.Equal(s.T(), http.StatusBadRequest, rr.Code)
	s.mockService.AssertExpectations(s.T())
}

// TestHandleDCRRegistration_ClientError tests handling of client errors
func (s *DCRHandlerTestSuite) TestHandleDCRRegistration_ClientError() {
	request := &DCRRegistrationRequest{
		RedirectURIs: []string{"not-a-valid-uri"},
		GrantTypes:   []providers.GrantType{providers.GrantTypeAuthorizationCode},
	}

	serviceErr := &tidcommon.ServiceError{
		Type:             tidcommon.ClientErrorType,
		Code:             "invalid_client_metadata",
		Error:            tidcommon.I18nMessage{DefaultValue: "Invalid client metadata"},
		ErrorDescription: tidcommon.I18nMessage{DefaultValue: "Invalid grant type"},
	}
	s.mockService.On("RegisterClient", mock.Anything, request).Return(nil, serviceErr)

	requestJSON, _ := json.Marshal(request)
	req := httptest.NewRequest(http.MethodPost, "/oauth2/dcr", bytes.NewReader(requestJSON))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	s.handler.HandleDCRRegistration(rr, req)

	assert.Equal(s.T(), http.StatusBadRequest, rr.Code)
	var errorResponse map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &errorResponse)
	s.NoError(err)
	assert.Equal(s.T(), "invalid_client_metadata", errorResponse["error"])
	s.mockService.AssertExpectations(s.T())
}

// TestHandleDCRRegistration_ServerError tests handling of server errors
func (s *DCRHandlerTestSuite) TestHandleDCRRegistration_ServerError() {
	request := &DCRRegistrationRequest{
		RedirectURIs: []string{"https://client.example.com/callback"},
		GrantTypes:   []providers.GrantType{providers.GrantTypeAuthorizationCode},
	}

	serviceErr := &ErrorServerError
	s.mockService.On("RegisterClient", mock.Anything, request).Return(nil, serviceErr)

	requestJSON, _ := json.Marshal(request)
	req := httptest.NewRequest(http.MethodPost, "/oauth2/dcr", bytes.NewReader(requestJSON))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	s.handler.HandleDCRRegistration(rr, req)

	assert.Equal(s.T(), http.StatusInternalServerError, rr.Code)
	var errorResponse map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &errorResponse)
	s.NoError(err)
	assert.Equal(s.T(), "server_error", errorResponse["error"])
	s.mockService.AssertExpectations(s.T())
}

// TestHandleDCRRegistration_UnknownErrorType tests handling of unknown error types (defaults to BadRequest)
func (s *DCRHandlerTestSuite) TestHandleDCRRegistration_UnknownErrorType() {
	request := &DCRRegistrationRequest{
		RedirectURIs: []string{"https://client.example.com/callback"},
		GrantTypes:   []providers.GrantType{providers.GrantTypeAuthorizationCode},
	}

	serviceErr := &tidcommon.ServiceError{
		Type:             "UnknownErrorType",
		Code:             "unknown_error",
		Error:            tidcommon.I18nMessage{DefaultValue: "Unknown error"},
		ErrorDescription: tidcommon.I18nMessage{DefaultValue: "An unknown error occurred"},
	}
	s.mockService.On("RegisterClient", mock.Anything, request).Return(nil, serviceErr)

	requestJSON, _ := json.Marshal(request)
	req := httptest.NewRequest(http.MethodPost, "/oauth2/dcr", bytes.NewReader(requestJSON))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	s.handler.HandleDCRRegistration(rr, req)

	// Unknown error type should default to BadRequest
	assert.Equal(s.T(), http.StatusBadRequest, rr.Code)
	s.mockService.AssertExpectations(s.T())
}

// TestHandleDCRRegistration_Success tests successful registration
func (s *DCRHandlerTestSuite) TestHandleDCRRegistration_Success() {
	request := &DCRRegistrationRequest{
		RedirectURIs: []string{"https://client.example.com/callback"},
		GrantTypes:   []providers.GrantType{providers.GrantTypeAuthorizationCode},
		ClientName:   "Test Client",
	}

	response := &DCRRegistrationResponse{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		ClientName:   "Test Client",
		RedirectURIs: []string{"https://client.example.com/callback"},
		GrantTypes:   []providers.GrantType{providers.GrantTypeAuthorizationCode},
	}

	s.mockService.On("RegisterClient", mock.Anything, request).Return(response, (*tidcommon.ServiceError)(nil))

	requestJSON, _ := json.Marshal(request)
	req := httptest.NewRequest(http.MethodPost, "/oauth2/dcr", bytes.NewReader(requestJSON))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	s.handler.HandleDCRRegistration(rr, req)

	assert.Equal(s.T(), http.StatusCreated, rr.Code)
	var responseBody DCRRegistrationResponse
	err := json.Unmarshal(rr.Body.Bytes(), &responseBody)
	s.NoError(err)
	assert.Equal(s.T(), "test-client-id", responseBody.ClientID)
	assert.Equal(s.T(), "test-client-secret", responseBody.ClientSecret)
	assert.Equal(s.T(), "Test Client", responseBody.ClientName)
	s.mockService.AssertExpectations(s.T())
}

// TestHandleDCRRegistration_EmptyBody tests handling of empty request body
func (s *DCRHandlerTestSuite) TestHandleDCRRegistration_EmptyBody() {
	req := httptest.NewRequest(http.MethodPost, "/oauth2/dcr", bytes.NewReader([]byte("")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	s.handler.HandleDCRRegistration(rr, req)

	assert.Equal(s.T(), http.StatusBadRequest, rr.Code)
	var errorResponse map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &errorResponse)
	s.NoError(err)
	assert.Contains(s.T(), errorResponse, "error")
}

// TestNewDCRHandler tests the handler constructor
func TestNewDCRHandler(t *testing.T) {
	mockService := NewDCRServiceInterfaceMock(t)
	handler := newDCRHandler(mockService, testhelpers.OAuthConfig())

	assert.NotNil(t, handler)
	assert.Equal(t, mockService, handler.dcrService)
}

// TestWriteServiceErrorResponse_DirectCall tests the writeServiceErrorResponse function directly
func TestWriteServiceErrorResponse_DirectCall(t *testing.T) {
	mockService := NewDCRServiceInterfaceMock(t)
	handler := newDCRHandler(mockService, testhelpers.OAuthConfig())

	testCases := []struct {
		name           string
		serviceError   *tidcommon.ServiceError
		expectedStatus int
	}{
		{
			name: "Client Error",
			serviceError: &tidcommon.ServiceError{
				Type:             tidcommon.ClientErrorType,
				Code:             "test_code",
				Error:            tidcommon.I18nMessage{DefaultValue: "Test error"},
				ErrorDescription: tidcommon.I18nMessage{DefaultValue: "Test description"},
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Server Error",
			serviceError: &tidcommon.ServiceError{
				Type:             tidcommon.ServerErrorType,
				Code:             "test_code",
				Error:            tidcommon.I18nMessage{DefaultValue: "Test error"},
				ErrorDescription: tidcommon.I18nMessage{DefaultValue: "Test description"},
			},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name: "Unknown Error Type",
			serviceError: &tidcommon.ServiceError{
				Type:             "UnknownType",
				Code:             "test_code",
				Error:            tidcommon.I18nMessage{DefaultValue: "Test error"},
				ErrorDescription: tidcommon.I18nMessage{DefaultValue: "Test description"},
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			handler.writeServiceErrorResponse(context.Background(), rr, tc.serviceError)

			assert.Equal(t, tc.expectedStatus, rr.Code)
			var errorResponse map[string]interface{}
			err := json.Unmarshal(rr.Body.Bytes(), &errorResponse)
			assert.NoError(t, err)
			assert.Equal(t, tc.serviceError.Code, errorResponse["error"])
		})
	}
}

// TestHandleDCRRegistration_ClosedDCR_NoToken tests that a missing token is rejected when insecure=false.
// Uses the default config where Insecure defaults to false (secure by default).
func TestHandleDCRRegistration_ClosedDCR_NoToken(t *testing.T) {
	_ = config.InitializeServerRuntime("test", &config.Config{})
	defer config.ResetServerRuntime()

	mockService := NewDCRServiceInterfaceMock(t)
	handler := newDCRHandler(mockService, testhelpers.OAuthConfig())

	req := httptest.NewRequest(http.MethodPost, "/oauth2/dcr/register", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.HandleDCRRegistration(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	var errResp map[string]interface{}
	assert.NoError(t, json.Unmarshal(rr.Body.Bytes(), &errResp))
	assert.Equal(t, "unauthorized_client", errResp["error"])
	mockService.AssertNotCalled(t, "RegisterClient")
}

// TestHandleDCRRegistration_ClosedDCR_InsufficientPermissions tests that a token without 'system'
// permission is rejected when insecure=false.
// Uses the default config where Insecure defaults to false (secure by default).
func TestHandleDCRRegistration_ClosedDCR_InsufficientPermissions(t *testing.T) {
	_ = config.InitializeServerRuntime("test", &config.Config{})
	defer config.ResetServerRuntime()

	mockService := NewDCRServiceInterfaceMock(t)
	handler := newDCRHandler(mockService, testhelpers.OAuthConfig())

	secCtx := security.NewSecurityContextForTest("user1", "ou1", "tok", []string{"openid", "profile"}, nil)
	ctx := security.WithSecurityContextTest(context.Background(), secCtx)

	req := httptest.NewRequest(http.MethodPost, "/oauth2/dcr/register", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.HandleDCRRegistration(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	var errResp map[string]interface{}
	assert.NoError(t, json.Unmarshal(rr.Body.Bytes(), &errResp))
	assert.Equal(t, "unauthorized_client", errResp["error"])
	mockService.AssertNotCalled(t, "RegisterClient")
}

// TestHandleDCRRegistration_ClosedDCR_WithSystemPermission tests that a token with the 'system'
// permission is accepted when insecure=false.
// Uses the default config where Insecure defaults to false (secure by default).
func TestHandleDCRRegistration_ClosedDCR_WithSystemPermission(t *testing.T) {
	_ = config.InitializeServerRuntime("test", &config.Config{})
	defer config.ResetServerRuntime()
	security.InitSystemPermissions("")
	defer security.InitSystemPermissions("")

	mockService := NewDCRServiceInterfaceMock(t)
	handler := newDCRHandler(mockService, testhelpers.OAuthConfig())

	secCtx := security.NewSecurityContextForTest("admin", "ou1", "tok", []string{"system"}, nil)
	ctx := security.WithSecurityContextTest(context.Background(), secCtx)

	request := &DCRRegistrationRequest{
		RedirectURIs: []string{"https://client.example.com/callback"},
		GrantTypes:   []providers.GrantType{providers.GrantTypeAuthorizationCode},
	}
	response := &DCRRegistrationResponse{ClientID: "new-client"}
	mockService.On("RegisterClient", mock.Anything, request).Return(response, (*tidcommon.ServiceError)(nil))

	requestJSON, _ := json.Marshal(request)
	req := httptest.NewRequest(http.MethodPost, "/oauth2/dcr/register", bytes.NewReader(requestJSON))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.HandleDCRRegistration(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	mockService.AssertExpectations(t)
}

// newRequest builds a client configuration request authorized by an administrative caller, with the
// client_id path value populated as the router would.
func (s *DCRHandlerTestSuite) newRequest(
	method string, body []byte) *http.Request {
	return s.newRequestWithPermissions(method, body, []string{"system"})
}

// newRequestWithPermissions builds a client configuration request whose caller holds the given
// permissions, so a test can exercise an unprivileged or unauthenticated caller.
func (s *DCRHandlerTestSuite) newRequestWithPermissions(
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
func (s *DCRHandlerTestSuite) TestGet_ReturnsRegistration() {
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
func (s *DCRHandlerTestSuite) TestGet_WithoutSystemPermissionIsUnauthorized() {
	rr := httptest.NewRecorder()
	s.handler.HandleGetClientConfiguration(rr,
		s.newRequestWithPermissions(http.MethodGet, nil, []string{"some:other:scope"}))

	s.Equal(http.StatusUnauthorized, rr.Code)
	s.Equal(wwwAuthenticateInvalidToken, rr.Header().Get(serverconst.WWWAuthenticateHeaderName))
	s.mockService.AssertNotCalled(s.T(), "GetClient", mock.Anything, mock.Anything)
}

// The management endpoints and the registration endpoint share a status code but not a message. A
// caller refused here is told it cannot manage a registration, not that it cannot register a client,
// which is what the registration endpoint says.
func (s *DCRHandlerTestSuite) TestGet_UnauthorizedReportsTheManagementMessage() {
	rr := httptest.NewRecorder()
	s.handler.HandleGetClientConfiguration(rr,
		s.newRequestWithPermissions(http.MethodGet, nil, []string{"some:other:scope"}))

	s.Equal(http.StatusUnauthorized, rr.Code)
	s.Contains(rr.Body.String(), ErrorUnauthorizedManagement.ErrorDescription.DefaultValue)
	s.NotContains(rr.Body.String(), ErrorUnauthorized.ErrorDescription.DefaultValue)
}

// An unauthenticated request carries no security context at all, and is rejected.
func (s *DCRHandlerTestSuite) TestGet_UnauthenticatedIsUnauthorized() {
	rr := httptest.NewRecorder()
	s.handler.HandleGetClientConfiguration(rr, s.newRequestWithPermissions(http.MethodGet, nil, nil))

	s.Equal(http.StatusUnauthorized, rr.Code)
	s.mockService.AssertNotCalled(s.T(), "GetClient", mock.Anything, mock.Anything)
}

// A deleted client no longer resolves, so managing it reports not found.
func (s *DCRHandlerTestSuite) TestGet_UnknownClientIsNotFound() {
	s.mockService.On("GetClient", mock.Anything, testClientID).
		Return((*DCRRegistrationResponse)(nil), &ErrorClientNotFound)

	rr := httptest.NewRecorder()
	s.handler.HandleGetClientConfiguration(rr, s.newRequest(http.MethodGet, nil))

	s.Equal(http.StatusNotFound, rr.Code)
}

// UC-3: a client updates its registered metadata.
func (s *DCRHandlerTestSuite) TestPut_UpdatesRegistration() {
	s.mockService.On("UpdateClient", mock.Anything, testClientID,
		mock.AnythingOfType("*dcr.DCRUpdateRequest")).
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
func (s *DCRHandlerTestSuite) TestPut_ClientIDMismatchIsRejected() {
	body := []byte(`{"client_id":"some-other-client","redirect_uris":["https://client.example.com/cb"]}`)
	rr := httptest.NewRecorder()
	s.handler.HandleUpdateClientConfiguration(rr, s.newRequest(http.MethodPut, body))

	s.Equal(http.StatusBadRequest, rr.Code)
	s.mockService.AssertNotCalled(s.T(), "UpdateClient", mock.Anything, mock.Anything, mock.Anything)
}

// RFC 7592 section 2.2 requires the update request to carry its client_id, so omitting it is
// rejected the same way a mismatched one is.
func (s *DCRHandlerTestSuite) TestPut_MissingClientIDIsRejected() {
	body := []byte(`{"redirect_uris":["https://client.example.com/cb"]}`)
	rr := httptest.NewRecorder()
	s.handler.HandleUpdateClientConfiguration(rr, s.newRequest(http.MethodPut, body))

	s.Equal(http.StatusBadRequest, rr.Code)
	s.mockService.AssertNotCalled(s.T(), "UpdateClient", mock.Anything, mock.Anything, mock.Anything)
}

// A malformed body is rejected before reaching the service.
func (s *DCRHandlerTestSuite) TestPut_InvalidBodyIsRejected() {
	rr := httptest.NewRecorder()
	s.handler.HandleUpdateClientConfiguration(rr,
		s.newRequest(http.MethodPut, []byte(`{"invalid": json}`)))

	s.Equal(http.StatusBadRequest, rr.Code)
	s.mockService.AssertNotCalled(s.T(), "UpdateClient", mock.Anything, mock.Anything, mock.Anything)
}

// Invalid client metadata on update is surfaced as an RFC 7592 client error.
func (s *DCRHandlerTestSuite) TestPut_InvalidMetadataIsRejected() {
	s.mockService.On("UpdateClient", mock.Anything, testClientID,
		mock.AnythingOfType("*dcr.DCRUpdateRequest")).
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
func (s *DCRHandlerTestSuite) TestDelete_RemovesRegistration() {
	s.mockService.On("DeleteClient", mock.Anything, testClientID).
		Return((*tidcommon.ServiceError)(nil))

	rr := httptest.NewRecorder()
	s.handler.HandleDeleteClientConfiguration(rr, s.newRequest(http.MethodDelete, nil))

	s.Equal(http.StatusNoContent, rr.Code)
	s.Empty(rr.Body.Bytes())
}

// Deleting an unknown client reports not found.
func (s *DCRHandlerTestSuite) TestDelete_UnknownClientIsNotFound() {
	s.mockService.On("DeleteClient", mock.Anything, testClientID).Return(&ErrorClientNotFound)

	rr := httptest.NewRecorder()
	s.handler.HandleDeleteClientConfiguration(rr, s.newRequest(http.MethodDelete, nil))

	s.Equal(http.StatusNotFound, rr.Code)
}

// A server failure during deletion is reported as a server error.
func (s *DCRHandlerTestSuite) TestDelete_ServerErrorIsReported() {
	s.mockService.On("DeleteClient", mock.Anything, testClientID).Return(&ErrorServerError)

	rr := httptest.NewRecorder()
	s.handler.HandleDeleteClientConfiguration(rr, s.newRequest(http.MethodDelete, nil))

	s.Equal(http.StatusInternalServerError, rr.Code)
}

// A request without a client ID in the path cannot identify a registration.
func (s *DCRHandlerTestSuite) TestGet_MissingClientIDIsNotFound() {
	req := httptest.NewRequest(http.MethodGet, "/oauth2/dcr/register/", nil)
	secCtx := security.NewSecurityContextForTest("admin", "ou1", "tok", []string{"system"}, nil)
	req = req.WithContext(security.WithSecurityContextTest(context.Background(), secCtx))

	rr := httptest.NewRecorder()
	s.handler.HandleGetClientConfiguration(rr, req)

	s.Equal(http.StatusNotFound, rr.Code)
}
