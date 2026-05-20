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

package idp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
)

const (
	testIdpName = "Test IDP"
)

type IDPHandlerTestSuite struct {
	suite.Suite
	mockService *IDPServiceInterfaceMock
	handler     *idpHandler
}

func TestIDPHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(IDPHandlerTestSuite))
}

func (s *IDPHandlerTestSuite) SetupTest() {
	s.mockService = NewIDPServiceInterfaceMock(s.T())
	s.handler = newIDPHandler(s.mockService)
}

// TestHandleIDPPostRequest_Success tests successful IDP creation
func (s *IDPHandlerTestSuite) TestHandleIDPPostRequest_Success() {
	reqBody := idpRequest{
		Name:        testIdpName,
		Description: "Test Description",
		Type:        "OIDC",
		Properties: []cmodels.PropertyDTO{
			{Name: "client_id", Value: "test-client", IsSecret: false},
		},
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/identity-providers", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	prop1, _ := cmodels.NewProperty("client_id", "test-client", false)

	createdIDP := &IDPDTO{
		ID:          "idp-123",
		Name:        testIdpName,
		Description: "Test Description",
		Type:        IDPTypeOIDC,
		Properties:  []cmodels.Property{*prop1},
	}

	s.mockService.On("CreateIdentityProvider", mock.Anything, mock.MatchedBy(func(dto *IDPDTO) bool {
		return dto.Name == testIdpName && dto.Type == IDPTypeOIDC && len(dto.Properties) == 1
	})).Return(createdIDP, (*serviceerror.ServiceError)(nil))

	s.handler.HandleIDPPostRequest(rr, req)

	s.Equal(http.StatusCreated, rr.Code)
	var response idpResponse
	err := json.NewDecoder(rr.Body).Decode(&response)
	s.NoError(err)
	s.Equal("idp-123", response.ID)
	s.Equal(testIdpName, response.Name)
	s.Equal("OIDC", response.Type)
}

// TestHandleIDPPostRequest_InvalidJSON tests malformed JSON request
func (s *IDPHandlerTestSuite) TestHandleIDPPostRequest_InvalidJSON() {
	req := httptest.NewRequest(http.MethodPost, "/identity-providers", bytes.NewReader([]byte("invalid json")))
	rr := httptest.NewRecorder()

	s.handler.HandleIDPPostRequest(rr, req)

	s.Equal(http.StatusBadRequest, rr.Code)
	s.Contains(rr.Body.String(), ErrorInvalidRequestFormat.Code)
}

// TestHandleIDPPostRequest_ServiceError tests service error handling
func (s *IDPHandlerTestSuite) TestHandleIDPPostRequest_ServiceError() {
	testCases := []struct {
		name           string
		serviceError   serviceerror.ServiceError
		expectedStatus int
	}{
		{
			name:           "IDP already exists",
			serviceError:   ErrorIDPAlreadyExists,
			expectedStatus: http.StatusConflict,
		},
		{
			name:           "Invalid IDP name",
			serviceError:   ErrorInvalidIDPName,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Internal server error",
			serviceError:   serviceerror.InternalServerError,
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			reqBody := idpRequest{
				Name: testIdpName,
				Type: "OIDC",
			}
			body, _ := json.Marshal(reqBody)
			req := httptest.NewRequest(http.MethodPost, "/identity-providers", bytes.NewReader(body))
			rr := httptest.NewRecorder()

			mockService := NewIDPServiceInterfaceMock(s.T())
			handler := newIDPHandler(mockService)
			mockService.On("CreateIdentityProvider", mock.Anything, mock.MatchedBy(func(dto *IDPDTO) bool {
				return dto.Name == testIdpName && dto.Type == IDPTypeOIDC
			})).Return((*IDPDTO)(nil), &tc.serviceError)

			handler.HandleIDPPostRequest(rr, req)

			s.Equal(tc.expectedStatus, rr.Code)
			s.Contains(rr.Body.String(), tc.serviceError.Code)
		})
	}
}

// TestHandleIDPListRequest_Success tests successful IDP list retrieval
func (s *IDPHandlerTestSuite) TestHandleIDPListRequest_Success() {
	req := httptest.NewRequest(http.MethodGet, "/identity-providers", nil)
	rr := httptest.NewRecorder()

	idpList := []BasicIDPDTO{
		{ID: "idp-1", Name: "IDP 1", Type: IDPTypeOIDC},
		{ID: "idp-2", Name: "IDP 2", Type: IDPTypeGoogle},
	}

	s.mockService.On("GetIdentityProviderList", mock.Anything).Return(idpList, (*serviceerror.ServiceError)(nil))

	s.handler.HandleIDPListRequest(rr, req)

	s.Equal(http.StatusOK, rr.Code)
	var response []basicIDPResponse
	err := json.NewDecoder(rr.Body).Decode(&response)
	s.NoError(err)
	s.Len(response, 2)
	s.Equal("idp-1", response[0].ID)
	s.Equal("IDP 1", response[0].Name)
	s.False(response[0].IsReadOnly, "First IDP should be mutable")
	s.False(response[1].IsReadOnly, "Second IDP should be mutable")
}

// TestHandleIDPListRequest_WithReadOnlyIDPs tests IDP list retrieval with read-only IDPs
func (s *IDPHandlerTestSuite) TestHandleIDPListRequest_WithReadOnlyIDPs() {
	req := httptest.NewRequest(http.MethodGet, "/identity-providers", nil)
	rr := httptest.NewRecorder()

	idpList := []BasicIDPDTO{
		{ID: "idp-1", Name: "IDP 1", Type: IDPTypeOIDC, IsReadOnly: false},
		{ID: "idp-2", Name: "IDP 2", Type: IDPTypeGoogle, IsReadOnly: true},
		{ID: "idp-3", Name: "IDP 3", Type: IDPTypeOIDC, IsReadOnly: false},
	}

	s.mockService.On("GetIdentityProviderList", mock.Anything).Return(idpList, (*serviceerror.ServiceError)(nil))

	s.handler.HandleIDPListRequest(rr, req)

	s.Equal(http.StatusOK, rr.Code)
	var response []basicIDPResponse
	err := json.NewDecoder(rr.Body).Decode(&response)
	s.NoError(err)
	s.Len(response, 3)
	s.Equal("idp-1", response[0].ID)
	s.False(response[0].IsReadOnly)
	s.Equal("idp-2", response[1].ID)
	s.True(response[1].IsReadOnly)
	s.Equal("idp-3", response[2].ID)
	s.False(response[2].IsReadOnly)
}

// TestHandleIDPListRequest_EmptyList tests empty IDP list
func (s *IDPHandlerTestSuite) TestHandleIDPListRequest_EmptyList() {
	req := httptest.NewRequest(http.MethodGet, "/identity-providers", nil)
	rr := httptest.NewRecorder()

	s.mockService.On("GetIdentityProviderList", mock.Anything).
		Return([]BasicIDPDTO{}, (*serviceerror.ServiceError)(nil))

	s.handler.HandleIDPListRequest(rr, req)

	s.Equal(http.StatusOK, rr.Code)
	var response []basicIDPResponse
	err := json.NewDecoder(rr.Body).Decode(&response)
	s.NoError(err)
	s.Len(response, 0)
}

// TestHandleIDPListRequest_ServiceError tests service error handling
func (s *IDPHandlerTestSuite) TestHandleIDPListRequest_ServiceError() {
	req := httptest.NewRequest(http.MethodGet, "/identity-providers", nil)
	rr := httptest.NewRecorder()

	s.mockService.On("GetIdentityProviderList", mock.Anything).
		Return([]BasicIDPDTO(nil), &serviceerror.InternalServerError)

	s.handler.HandleIDPListRequest(rr, req)

	s.Equal(http.StatusInternalServerError, rr.Code)
	s.Contains(rr.Body.String(), serviceerror.InternalServerError.Code)
}

// TestHandleIDPGetRequest_Success tests successful IDP retrieval
func (s *IDPHandlerTestSuite) TestHandleIDPGetRequest_Success() {
	req := httptest.NewRequest(http.MethodGet, "/identity-providers/idp-123", nil)
	req.SetPathValue("id", "idp-123")
	rr := httptest.NewRecorder()

	idp := &IDPDTO{
		ID:          "idp-123",
		Name:        testIdpName,
		Description: "Test Description",
		Type:        IDPTypeOIDC,
	}

	s.mockService.On("GetIdentityProvider", mock.Anything, "idp-123").Return(idp, (*serviceerror.ServiceError)(nil))

	s.handler.HandleIDPGetRequest(rr, req)

	s.Equal(http.StatusOK, rr.Code)
	var response idpResponse
	err := json.NewDecoder(rr.Body).Decode(&response)
	s.NoError(err)
	s.Equal("idp-123", response.ID)
	s.Equal(testIdpName, response.Name)
}

// TestHandleIDPGetRequest_EmptyID tests empty IDP ID
func (s *IDPHandlerTestSuite) TestHandleIDPGetRequest_EmptyID() {
	req := httptest.NewRequest(http.MethodGet, "/identity-providers/", nil)
	req.SetPathValue("id", "")
	rr := httptest.NewRecorder()

	s.handler.HandleIDPGetRequest(rr, req)

	s.Equal(http.StatusBadRequest, rr.Code)
	s.Contains(rr.Body.String(), ErrorInvalidIDPID.Code)
}

// TestHandleIDPGetRequest_IDPNotFound tests IDP not found
func (s *IDPHandlerTestSuite) TestHandleIDPGetRequest_IDPNotFound() {
	req := httptest.NewRequest(http.MethodGet, "/identity-providers/non-existent", nil)
	req.SetPathValue("id", "non-existent")
	rr := httptest.NewRecorder()

	s.mockService.On("GetIdentityProvider", mock.Anything, "non-existent").Return((*IDPDTO)(nil), &ErrorIDPNotFound)

	s.handler.HandleIDPGetRequest(rr, req)

	s.Equal(http.StatusNotFound, rr.Code)
	s.Contains(rr.Body.String(), ErrorIDPNotFound.Code)
}

// TestHandleIDPPutRequest_Success tests successful IDP update
func (s *IDPHandlerTestSuite) TestHandleIDPPutRequest_Success() {
	reqBody := idpRequest{
		Name:        "Updated IDP",
		Description: "Updated Description",
		Type:        "OIDC",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPut, "/identity-providers/idp-123", bytes.NewReader(body))
	req.SetPathValue("id", "idp-123")
	rr := httptest.NewRecorder()

	updatedIDP := &IDPDTO{
		ID:          "idp-123",
		Name:        "Updated IDP",
		Description: "Updated Description",
		Type:        IDPTypeOIDC,
		Properties:  []cmodels.Property{},
	}

	s.mockService.On("UpdateIdentityProvider", mock.Anything, "idp-123", &IDPDTO{
		ID:          "idp-123",
		Name:        "Updated IDP",
		Description: "Updated Description",
		Type:        IDPTypeOIDC,
		Properties:  []cmodels.Property{},
	}).Return(updatedIDP, (*serviceerror.ServiceError)(nil))

	s.handler.HandleIDPPutRequest(rr, req)

	s.Equal(http.StatusOK, rr.Code)
	var response idpResponse
	err := json.NewDecoder(rr.Body).Decode(&response)
	s.NoError(err)
	s.Equal("Updated IDP", response.Name)
}

// TestHandleIDPPutRequest_EmptyID tests empty IDP ID
func (s *IDPHandlerTestSuite) TestHandleIDPPutRequest_EmptyID() {
	reqBody := idpRequest{Name: "Test", Type: "OIDC"}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPut, "/identity-providers/", bytes.NewReader(body))
	req.SetPathValue("id", "")
	rr := httptest.NewRecorder()

	s.handler.HandleIDPPutRequest(rr, req)

	s.Equal(http.StatusBadRequest, rr.Code)
	s.Contains(rr.Body.String(), ErrorInvalidIDPID.Code)
}

// TestHandleIDPPutRequest_InvalidJSON tests malformed JSON request
func (s *IDPHandlerTestSuite) TestHandleIDPPutRequest_InvalidJSON() {
	req := httptest.NewRequest(http.MethodPut, "/identity-providers/idp-123", bytes.NewReader([]byte("invalid")))
	req.SetPathValue("id", "idp-123")
	rr := httptest.NewRecorder()

	s.handler.HandleIDPPutRequest(rr, req)

	s.Equal(http.StatusBadRequest, rr.Code)
	s.Contains(rr.Body.String(), ErrorInvalidRequestFormat.Code)
}

// TestHandleIDPDeleteRequest_Success tests successful IDP deletion
func (s *IDPHandlerTestSuite) TestHandleIDPDeleteRequest_Success() {
	req := httptest.NewRequest(http.MethodDelete, "/identity-providers/idp-123", nil)
	req.SetPathValue("id", "idp-123")
	rr := httptest.NewRecorder()

	s.mockService.On("DeleteIdentityProvider", mock.Anything, "idp-123").Return((*serviceerror.ServiceError)(nil))

	s.handler.HandleIDPDeleteRequest(rr, req)

	s.Equal(http.StatusNoContent, rr.Code)
}

// TestHandleIDPDeleteRequest_EmptyID tests empty IDP ID
func (s *IDPHandlerTestSuite) TestHandleIDPDeleteRequest_EmptyID() {
	req := httptest.NewRequest(http.MethodDelete, "/identity-providers/", nil)
	req.SetPathValue("id", "")
	rr := httptest.NewRecorder()

	s.handler.HandleIDPDeleteRequest(rr, req)

	s.Equal(http.StatusBadRequest, rr.Code)
	s.Contains(rr.Body.String(), ErrorInvalidIDPID.Code)
}

// TestHandleIDPDeleteRequest_IDPNotFound tests IDP not found
func (s *IDPHandlerTestSuite) TestHandleIDPDeleteRequest_IDPNotFound() {
	req := httptest.NewRequest(http.MethodDelete, "/identity-providers/non-existent", nil)
	req.SetPathValue("id", "non-existent")
	rr := httptest.NewRecorder()

	s.mockService.On("DeleteIdentityProvider", mock.Anything, "non-existent").Return(&ErrorIDPNotFound)

	s.handler.HandleIDPDeleteRequest(rr, req)

	s.Equal(http.StatusNotFound, rr.Code)
	s.Contains(rr.Body.String(), ErrorIDPNotFound.Code)
}

// TestGetClientErrorStatusCode tests status code mapping
func (s *IDPHandlerTestSuite) TestGetClientErrorStatusCode() {
	testCases := []struct {
		name           string
		errorCode      string
		expectedStatus int
	}{
		{"IDP not found", ErrorIDPNotFound.Code, http.StatusNotFound},
		{"IDP already exists", ErrorIDPAlreadyExists.Code, http.StatusConflict},
		{"Other client error", "IDP-1099", http.StatusBadRequest},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			status := getClientErrorStatusCode(tc.errorCode)
			s.Equal(tc.expectedStatus, status)
		})
	}
}

// TestGetSanitizedProperties tests property sanitization
func (s *IDPHandlerTestSuite) TestGetSanitizedProperties() {
	testCases := []struct {
		name        string
		input       []cmodels.PropertyDTO
		expectError bool
	}{
		{
			name: "Valid non-secret properties",
			input: []cmodels.PropertyDTO{
				{Name: "client_id", Value: "test", IsSecret: false},
				{Name: "redirect_uri", Value: "https://example.com/callback", IsSecret: false},
			},
			expectError: false,
		},
		{
			name:        "Empty properties",
			input:       []cmodels.PropertyDTO{},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			properties, err := getSanitizedProperties(tc.input)
			if tc.expectError {
				s.Error(err)
			} else {
				s.NoError(err)
				s.Len(properties, len(tc.input))
			}
		})
	}
}

// TestGetIDPResponse tests IDP response construction
func (s *IDPHandlerTestSuite) TestGetIDPResponse() {
	testCases := []struct {
		name        string
		idp         IDPDTO
		expectError bool
	}{
		{
			name: "IDP with regular properties",
			idp: IDPDTO{
				ID:          "idp-1",
				Name:        "Test",
				Description: "Desc",
				Type:        IDPTypeOIDC,
				Properties:  []cmodels.Property{},
			},
			expectError: false,
		},
		{
			name: "IDP with non-secret properties",
			idp: func() IDPDTO {
				prop, _ := cmodels.NewProperty("client_id", "value", false)
				return IDPDTO{
					ID:         "idp-1",
					Name:       "Test",
					Type:       IDPTypeOIDC,
					Properties: []cmodels.Property{*prop},
				}
			}(),
			expectError: false,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			response, err := getIDPResponse(tc.idp)
			if tc.expectError {
				s.Error(err)
			} else {
				s.NoError(err)
				s.Equal(tc.idp.ID, response.ID)
				s.Equal(tc.idp.Name, response.Name)
			}
		})
	}
}

// TestWriteServiceErrorResponse tests error response writing
func (s *IDPHandlerTestSuite) TestWriteServiceErrorResponse() {
	testCases := []struct {
		name           string
		serviceError   serviceerror.ServiceError
		expectedStatus int
	}{
		{
			name:           "Client error",
			serviceError:   ErrorInvalidIDPName,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Server error",
			serviceError:   serviceerror.InternalServerError,
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			rr := httptest.NewRecorder()
			writeServiceErrorResponse(rr, &tc.serviceError)

			s.Equal(tc.expectedStatus, rr.Code)
		})
	}
}
