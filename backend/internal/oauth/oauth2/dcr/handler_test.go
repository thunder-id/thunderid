/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/tests/testhelpers"
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
		OAuth: engineconfig.OAuthConfig{DCR: engineconfig.DCRConfig{Insecure: true}},
	})
	cfg := testhelpers.OAuthConfig()
	cfg.OAuth.DCR.Insecure = true
	s.handler = newDCRHandler(s.mockService, cfg)
}

func (s *DCRHandlerTestSuite) TearDownTest() {
	config.ResetServerRuntime()
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
