// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/application/model"
	"github.com/thunder-id/thunderid/internal/cert"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/sharing"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/log"
)

type HandlerTestSuite struct {
	suite.Suite
}

func TestHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(HandlerTestSuite))
}

func (suite *HandlerTestSuite) TestNewApplicationHandler() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	assert.NotNil(suite.T(), handler)
	assert.Equal(suite.T(), mockService, handler.service)
}

func (suite *HandlerTestSuite) TestHandleApplicationPostRequest_Success() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	appRequest := model.ApplicationRequest{
		OUID:        "ou-123",
		Name:        "TestApp",
		Description: "Test Description",
		Metadata:    map[string]interface{}{"key1": "val1"},
	}

	expectedApp := &model.ApplicationDTO{
		ID:          "test-app-id",
		OUID:        "ou-123",
		Name:        "TestApp",
		Description: "Test Description",
		Metadata:    map[string]interface{}{"key1": "val1"},
	}

	mockService.On("CreateApplication", mock.Anything, mock.AnythingOfType("*model.ApplicationDTO")).
		Return(expectedApp, nil)

	body, _ := json.Marshal(appRequest)
	req := httptest.NewRequest(http.MethodPost, "/applications", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleApplicationPostRequest(w, req)

	assert.Equal(suite.T(), http.StatusCreated, w.Code)
	assert.Equal(suite.T(), "application/json", w.Header().Get("Content-Type"))

	var response model.ApplicationCompleteResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "test-app-id", response.ID)
	assert.Equal(suite.T(), "TestApp", response.Name)
	assert.Equal(suite.T(), map[string]interface{}{"key1": "val1"}, response.Metadata)

	mockService.AssertExpectations(suite.T())
}

func (suite *HandlerTestSuite) TestHandleApplicationPostRequest_SuccessWithOAuth() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	appRequest := model.ApplicationRequest{
		OUID:        "ou-123",
		Name:        "TestApp",
		Description: "Test Description",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID:                "test-client-id",
					ClientSecret:            "test-secret",
					RedirectURIs:            []string{"https://example.com/callback"},
					GrantTypes:              []providers.GrantType{providers.GrantTypeAuthorizationCode},
					ResponseTypes:           []providers.ResponseType{providers.ResponseTypeCode},
					TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
				},
			},
		},
	}

	expectedApp := &model.ApplicationDTO{
		ID:          "test-app-id",
		OUID:        "ou-123",
		Name:        "TestApp",
		Description: "Test Description",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID:                "test-client-id",
					ClientSecret:            "test-secret",
					RedirectURIs:            []string{"https://example.com/callback"},
					GrantTypes:              []providers.GrantType{providers.GrantTypeAuthorizationCode},
					ResponseTypes:           []providers.ResponseType{providers.ResponseTypeCode},
					TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
				},
			},
		},
	}

	mockService.On("CreateApplication", mock.Anything, mock.AnythingOfType("*model.ApplicationDTO")).
		Return(expectedApp, nil)

	body, _ := json.Marshal(appRequest)
	req := httptest.NewRequest(http.MethodPost, "/applications", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleApplicationPostRequest(w, req)

	assert.Equal(suite.T(), http.StatusCreated, w.Code)
	assert.Equal(suite.T(), "application/json", w.Header().Get("Content-Type"))

	var response model.ApplicationCompleteResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "test-app-id", response.ID)
	assert.Equal(suite.T(), "TestApp", response.Name)
	assert.Equal(suite.T(), "test-client-id", response.ClientID)

	mockService.AssertExpectations(suite.T())
}

func (suite *HandlerTestSuite) TestHandleApplicationPostRequest_TemplateScenarios() {
	testCases := []struct {
		name             string
		template         string
		expectedTemplate string
	}{
		{
			name:             "with template",
			template:         "spa",
			expectedTemplate: "spa",
		},
		{
			name:             "with empty template",
			template:         "",
			expectedTemplate: "",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			mockService := NewApplicationServiceInterfaceMock(suite.T())
			handler := newApplicationHandler(mockService)

			appRequest := model.ApplicationRequest{
				OUID:        "ou-123",
				Name:        "TestApp",
				Description: "Test Description",
				Template:    tc.template,
			}

			expectedApp := &model.ApplicationDTO{
				ID:          "test-app-id",
				OUID:        "ou-123",
				Name:        "TestApp",
				Description: "Test Description",
				Template:    tc.expectedTemplate,
			}

			mockService.On("CreateApplication", mock.Anything, mock.AnythingOfType("*model.ApplicationDTO")).
				Return(expectedApp, nil)

			body, _ := json.Marshal(appRequest)
			req := httptest.NewRequest(http.MethodPost, "/applications", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.HandleApplicationPostRequest(w, req)

			assert.Equal(suite.T(), http.StatusCreated, w.Code)

			var response model.ApplicationCompleteResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(suite.T(), err)
			assert.Equal(suite.T(), "test-app-id", response.ID)
			assert.Equal(suite.T(), tc.expectedTemplate, response.Template)

			mockService.AssertExpectations(suite.T())
		})
	}
}

func (suite *HandlerTestSuite) TestHandleApplicationPostRequest_InvalidJSON() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	req := httptest.NewRequest(http.MethodPost, "/applications", bytes.NewBufferString("{invalid json}"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleApplicationPostRequest(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	assert.Equal(suite.T(), "application/json", w.Header().Get("Content-Type"))

	var errResp apierror.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &errResp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), ErrorInvalidRequestFormat.Code, errResp.Code)
}

func (suite *HandlerTestSuite) TestHandleApplicationPostRequest_ServiceError() {
	tests := []struct {
		name           string
		svcErr         *tidcommon.ServiceError
		expectedStatus int
		expectedCode   string
	}{
		{
			name:           "InvalidApplicationName",
			svcErr:         &ErrorInvalidApplicationName,
			expectedStatus: http.StatusBadRequest,
			expectedCode:   ErrorInvalidApplicationName.Code,
		},
		{
			name:           "InternalServerError",
			svcErr:         &tidcommon.InternalServerError,
			expectedStatus: http.StatusInternalServerError,
			expectedCode:   tidcommon.InternalServerError.Code,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			mockService := NewApplicationServiceInterfaceMock(suite.T())
			handler := newApplicationHandler(mockService)

			appRequest := model.ApplicationRequest{
				OUID: "ou-123",
				Name: "TestApp",
			}

			mockService.On("CreateApplication", mock.Anything, mock.AnythingOfType("*model.ApplicationDTO")).
				Return(nil, tt.svcErr)

			body, _ := json.Marshal(appRequest)
			req := httptest.NewRequest(http.MethodPost, "/applications", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.HandleApplicationPostRequest(w, req)

			assert.Equal(suite.T(), tt.expectedStatus, w.Code)
			assert.Equal(suite.T(), "application/json", w.Header().Get("Content-Type"))

			var errResp apierror.ErrorResponse
			err := json.Unmarshal(w.Body.Bytes(), &errResp)
			assert.NoError(suite.T(), err)
			assert.Equal(suite.T(), tt.expectedCode, errResp.Code)

			mockService.AssertExpectations(suite.T())
		})
	}
}

func (suite *HandlerTestSuite) TestHandleApplicationPostRequest_ProcessInboundAuthConfigError() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	appRequest := model.ApplicationRequest{
		OUID: "ou-123",
		Name: "TestApp",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type:        providers.OAuthInboundAuthType,
				OAuthConfig: nil, // This will cause processInboundAuthConfig to return empty
			},
		},
	}

	// Create app with inbound auth config that has unsupported type
	expectedApp := &model.ApplicationDTO{
		ID:   "test-app-id",
		OUID: "ou-123",
		Name: "TestApp",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type:        "unsupported",
				OAuthConfig: nil,
			},
		},
	}

	mockService.On("CreateApplication", mock.Anything, mock.AnythingOfType("*model.ApplicationDTO")).
		Return(expectedApp, nil)

	body, _ := json.Marshal(appRequest)
	req := httptest.NewRequest(http.MethodPost, "/applications", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleApplicationPostRequest(w, req)

	assert.Equal(suite.T(), http.StatusInternalServerError, w.Code)

	mockService.AssertExpectations(suite.T())
}

func (suite *HandlerTestSuite) TestHandleApplicationListRequest_Success() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	expectedList := &model.ApplicationListResponse{
		TotalResults: 2,
		Count:        2,
		Applications: []model.BasicApplicationResponse{
			{
				ID:          "app1",
				Name:        "App1",
				Description: "Description 1",
			},
			{
				ID:          "app2",
				Name:        "App2",
				Description: "Description 2",
			},
		},
	}

	mockService.On("GetApplicationList", mock.Anything).Return(expectedList, nil)

	req := httptest.NewRequest(http.MethodGet, "/applications", nil)
	w := httptest.NewRecorder()

	handler.HandleApplicationListRequest(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)
	assert.Equal(suite.T(), "application/json", w.Header().Get("Content-Type"))

	var response model.ApplicationListResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), 2, response.TotalResults)
	assert.Equal(suite.T(), 2, response.Count)
	assert.Len(suite.T(), response.Applications, 2)

	mockService.AssertExpectations(suite.T())
}

func (suite *HandlerTestSuite) TestHandleApplicationListRequest_WithTemplate() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	expectedList := &model.ApplicationListResponse{
		TotalResults: 2,
		Count:        2,
		Applications: []model.BasicApplicationResponse{
			{
				ID:          "app1",
				Name:        "App1",
				Description: "Description 1",
				Template:    "spa",
			},
			{
				ID:          "app2",
				Name:        "App2",
				Description: "Description 2",
				Template:    "mobile",
			},
		},
	}

	mockService.On("GetApplicationList", mock.Anything).Return(expectedList, nil)

	req := httptest.NewRequest(http.MethodGet, "/applications", nil)
	w := httptest.NewRecorder()

	handler.HandleApplicationListRequest(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)

	var response model.ApplicationListResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), 2, response.TotalResults)
	assert.Equal(suite.T(), "spa", response.Applications[0].Template)
	assert.Equal(suite.T(), "mobile", response.Applications[1].Template)

	mockService.AssertExpectations(suite.T())
}

func (suite *HandlerTestSuite) TestHandleApplicationListRequest_ServiceError() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	svcErr := &tidcommon.InternalServerError

	mockService.On("GetApplicationList", mock.Anything).Return(nil, svcErr)

	req := httptest.NewRequest(http.MethodGet, "/applications", nil)
	w := httptest.NewRecorder()

	handler.HandleApplicationListRequest(w, req)

	assert.Equal(suite.T(), http.StatusInternalServerError, w.Code)
	assert.Equal(suite.T(), "application/json", w.Header().Get("Content-Type"))

	var errResp apierror.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &errResp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, errResp.Code)

	mockService.AssertExpectations(suite.T())
}

func (suite *HandlerTestSuite) TestHandleApplicationGetRequest_Success() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	expectedApp := &providers.Application{
		ID:          "test-app-id",
		Name:        "TestApp",
		Description: "Test Description",
		Metadata:    map[string]interface{}{"key3": "val3"},
	}

	mockService.On("GetApplication", mock.Anything, "test-app-id").Return(expectedApp, nil)

	req := httptest.NewRequest(http.MethodGet, "/applications/test-app-id", nil)
	req.SetPathValue("id", "test-app-id")
	w := httptest.NewRecorder()

	handler.HandleApplicationGetRequest(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)
	assert.Equal(suite.T(), "application/json", w.Header().Get("Content-Type"))

	var response model.ApplicationGetResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "test-app-id", response.ID)
	assert.Equal(suite.T(), "TestApp", response.Name)
	assert.Equal(suite.T(), map[string]interface{}{"key3": "val3"}, response.Metadata)

	mockService.AssertExpectations(suite.T())
}

func (suite *HandlerTestSuite) TestHandleApplicationGetRequest_SuccessWithOAuth() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	expectedApp := &providers.Application{
		ID:          "test-app-id",
		Name:        "TestApp",
		Description: "Test Description",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID:                "test-client-id",
					RedirectURIs:            []string{"https://example.com/callback"},
					GrantTypes:              []providers.GrantType{providers.GrantTypeAuthorizationCode},
					ResponseTypes:           []providers.ResponseType{providers.ResponseTypeCode},
					TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
				},
			},
		},
	}

	mockService.On("GetApplication", mock.Anything, "test-app-id").Return(expectedApp, nil)

	req := httptest.NewRequest(http.MethodGet, "/applications/test-app-id", nil)
	req.SetPathValue("id", "test-app-id")
	w := httptest.NewRecorder()

	handler.HandleApplicationGetRequest(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)
	assert.Equal(suite.T(), "application/json", w.Header().Get("Content-Type"))

	var response model.ApplicationGetResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "test-app-id", response.ID)
	assert.Equal(suite.T(), "TestApp", response.Name)
	assert.Equal(suite.T(), "test-client-id", response.ClientID)

	mockService.AssertExpectations(suite.T())
}

func (suite *HandlerTestSuite) TestHandleApplicationGetRequest_WithTemplate() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	expectedApp := &providers.Application{
		ID:          "test-app-id",
		Name:        "TestApp",
		Description: "Test Description",
		InboundAuthProfile: providers.InboundAuthProfile{
			ThemeID:  "theme-123",
			LayoutID: "layout-456",
		},
		Template: "spa",
	}

	mockService.On("GetApplication", mock.Anything, "test-app-id").Return(expectedApp, nil)

	req := httptest.NewRequest(http.MethodGet, "/applications/test-app-id", nil)
	req.SetPathValue("id", "test-app-id")
	w := httptest.NewRecorder()

	handler.HandleApplicationGetRequest(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)

	var response model.ApplicationGetResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "test-app-id", response.ID)
	assert.Equal(suite.T(), "theme-123", response.ThemeID)
	assert.Equal(suite.T(), "layout-456", response.LayoutID)
	assert.Equal(suite.T(), "spa", response.Template)

	mockService.AssertExpectations(suite.T())
}

func (suite *HandlerTestSuite) TestHandleApplicationGetRequest_WithEmptyTemplate() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	expectedApp := &providers.Application{
		ID:          "test-app-id",
		Name:        "TestApp",
		Description: "Test Description",
		Template:    "",
	}

	mockService.On("GetApplication", mock.Anything, "test-app-id").Return(expectedApp, nil)

	req := httptest.NewRequest(http.MethodGet, "/applications/test-app-id", nil)
	req.SetPathValue("id", "test-app-id")
	w := httptest.NewRecorder()

	handler.HandleApplicationGetRequest(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)

	var response model.ApplicationGetResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "test-app-id", response.ID)
	assert.Equal(suite.T(), "", response.Template)

	mockService.AssertExpectations(suite.T())
}

func (suite *HandlerTestSuite) TestHandleApplicationGetRequest_InvalidID() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	req := httptest.NewRequest(http.MethodGet, "/applications/", nil)
	req.SetPathValue("id", "")
	w := httptest.NewRecorder()

	handler.HandleApplicationGetRequest(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	assert.Equal(suite.T(), "application/json", w.Header().Get("Content-Type"))

	var errResp apierror.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &errResp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), ErrorInvalidApplicationID.Code, errResp.Code)
}

//nolint:dupl // Testing different error scenarios
func (suite *HandlerTestSuite) TestHandleApplicationGetRequest_NotFound() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	svcErr := &ErrorApplicationNotFound

	mockService.On("GetApplication", mock.Anything, "non-existent-id").Return(nil, svcErr)

	req := httptest.NewRequest(http.MethodGet, "/applications/non-existent-id", nil)
	req.SetPathValue("id", "non-existent-id")
	w := httptest.NewRecorder()

	handler.HandleApplicationGetRequest(w, req)

	assert.Equal(suite.T(), http.StatusNotFound, w.Code)
	assert.Equal(suite.T(), "application/json", w.Header().Get("Content-Type"))

	var errResp apierror.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &errResp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), ErrorApplicationNotFound.Code, errResp.Code)

	mockService.AssertExpectations(suite.T())
}

//nolint:dupl // Testing different error scenarios
func (suite *HandlerTestSuite) TestHandleApplicationGetRequest_ServiceError() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	svcErr := &tidcommon.InternalServerError

	mockService.On("GetApplication", mock.Anything, "test-app-id").Return(nil, svcErr)

	req := httptest.NewRequest(http.MethodGet, "/applications/test-app-id", nil)
	req.SetPathValue("id", "test-app-id")
	w := httptest.NewRecorder()

	handler.HandleApplicationGetRequest(w, req)

	assert.Equal(suite.T(), http.StatusInternalServerError, w.Code)
	assert.Equal(suite.T(), "application/json", w.Header().Get("Content-Type"))

	var errResp apierror.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &errResp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, errResp.Code)

	mockService.AssertExpectations(suite.T())
}

func (suite *HandlerTestSuite) TestHandleApplicationGetRequest_UnsupportedInboundAuthType() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	expectedApp := &providers.Application{
		ID:   "test-app-id",
		Name: "TestApp",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type:        "unsupported",
				OAuthConfig: nil,
			},
		},
	}

	mockService.On("GetApplication", mock.Anything, "test-app-id").Return(expectedApp, nil)

	req := httptest.NewRequest(http.MethodGet, "/applications/test-app-id", nil)
	req.SetPathValue("id", "test-app-id")
	w := httptest.NewRecorder()

	handler.HandleApplicationGetRequest(w, req)

	assert.Equal(suite.T(), http.StatusInternalServerError, w.Code)

	mockService.AssertExpectations(suite.T())
}

func (suite *HandlerTestSuite) TestHandleApplicationGetRequest_NilOAuthConfig() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	expectedApp := &providers.Application{
		ID:   "test-app-id",
		Name: "TestApp",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type:        providers.OAuthInboundAuthType,
				OAuthConfig: nil,
			},
		},
	}

	mockService.On("GetApplication", mock.Anything, "test-app-id").Return(expectedApp, nil)

	req := httptest.NewRequest(http.MethodGet, "/applications/test-app-id", nil)
	req.SetPathValue("id", "test-app-id")
	w := httptest.NewRecorder()

	handler.HandleApplicationGetRequest(w, req)

	assert.Equal(suite.T(), http.StatusInternalServerError, w.Code)

	mockService.AssertExpectations(suite.T())
}

func (suite *HandlerTestSuite) TestHandleApplicationPutRequest_Success() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	appRequest := model.ApplicationRequest{
		OUID:        "ou-123",
		Name:        "UpdatedApp",
		Description: "Updated Description",
		Metadata:    map[string]interface{}{"key2": "val2"},
	}

	expectedApp := &model.ApplicationDTO{
		OUID:        "ou-123",
		ID:          "test-app-id",
		Name:        "UpdatedApp",
		Description: "Updated Description",
		Metadata:    map[string]interface{}{"key2": "val2"},
	}

	mockService.On("UpdateApplication", mock.Anything, "test-app-id",
		mock.AnythingOfType("*model.ApplicationDTO")).
		Return(expectedApp, nil)

	body, _ := json.Marshal(appRequest)
	req := httptest.NewRequest(http.MethodPut, "/applications/test-app-id", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "test-app-id")
	w := httptest.NewRecorder()

	handler.HandleApplicationPutRequest(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)
	assert.Equal(suite.T(), "application/json", w.Header().Get("Content-Type"))

	var response model.ApplicationCompleteResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "test-app-id", response.ID)
	assert.Equal(suite.T(), "UpdatedApp", response.Name)
	assert.Equal(suite.T(), map[string]interface{}{"key2": "val2"}, response.Metadata)

	mockService.AssertExpectations(suite.T())
}

func (suite *HandlerTestSuite) TestHandleApplicationPutRequest_WithTemplate() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	appRequest := model.ApplicationRequest{
		OUID:        "ou-123",
		Name:        "UpdatedApp",
		Description: "Updated Description",
		Template:    "mobile",
	}

	expectedApp := &model.ApplicationDTO{
		OUID:        "ou-123",
		ID:          "test-app-id",
		Name:        "UpdatedApp",
		Description: "Updated Description",
		Template:    "mobile",
	}

	mockService.On("UpdateApplication", mock.Anything, "test-app-id",
		mock.AnythingOfType("*model.ApplicationDTO")).
		Return(expectedApp, nil)

	body, _ := json.Marshal(appRequest)
	req := httptest.NewRequest(http.MethodPut, "/applications/test-app-id", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "test-app-id")
	w := httptest.NewRecorder()

	handler.HandleApplicationPutRequest(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)

	var response model.ApplicationCompleteResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "test-app-id", response.ID)
	assert.Equal(suite.T(), "mobile", response.Template)

	mockService.AssertExpectations(suite.T())
}

func (suite *HandlerTestSuite) TestHandleApplicationPutRequest_TemplateScenarios() {
	testCases := []struct {
		name             string
		template         string
		expectedTemplate string
	}{
		{
			name:             "update template",
			template:         "traditional_web_app",
			expectedTemplate: "traditional_web_app",
		},
		{
			name:             "clear template",
			template:         "",
			expectedTemplate: "",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			mockService := NewApplicationServiceInterfaceMock(suite.T())
			handler := newApplicationHandler(mockService)

			appRequest := model.ApplicationRequest{
				OUID:        "ou-123",
				Name:        "UpdatedApp",
				Description: "Updated Description",
				Template:    tc.template,
			}

			expectedApp := &model.ApplicationDTO{
				OUID:        "ou-123",
				ID:          "test-app-id",
				Name:        "UpdatedApp",
				Description: "Updated Description",
				Template:    tc.expectedTemplate,
			}

			mockService.On("UpdateApplication", mock.Anything, "test-app-id",
				mock.AnythingOfType("*model.ApplicationDTO")).
				Return(expectedApp, nil)

			body, _ := json.Marshal(appRequest)
			req := httptest.NewRequest(http.MethodPut, "/applications/test-app-id", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			req.SetPathValue("id", "test-app-id")
			w := httptest.NewRecorder()

			handler.HandleApplicationPutRequest(w, req)

			assert.Equal(suite.T(), http.StatusOK, w.Code)

			var response model.ApplicationCompleteResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(suite.T(), err)
			assert.Equal(suite.T(), tc.expectedTemplate, response.Template)

			mockService.AssertExpectations(suite.T())
		})
	}
}

func (suite *HandlerTestSuite) TestHandleApplicationPutRequest_InvalidID() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	appRequest := model.ApplicationRequest{
		OUID: "ou-123",
		Name: "UpdatedApp",
	}

	body, _ := json.Marshal(appRequest)
	req := httptest.NewRequest(http.MethodPut, "/applications/", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "")
	w := httptest.NewRecorder()

	handler.HandleApplicationPutRequest(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	assert.Equal(suite.T(), "application/json", w.Header().Get("Content-Type"))

	var errResp apierror.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &errResp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), ErrorInvalidApplicationID.Code, errResp.Code)
}

func (suite *HandlerTestSuite) TestHandleApplicationPutRequest_InvalidJSON() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	req := httptest.NewRequest(http.MethodPut, "/applications/test-app-id", bytes.NewBufferString("{invalid json}"))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "test-app-id")
	w := httptest.NewRecorder()

	handler.HandleApplicationPutRequest(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	assert.Equal(suite.T(), "application/json", w.Header().Get("Content-Type"))

	var errResp apierror.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &errResp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), ErrorInvalidRequestFormat.Code, errResp.Code)
}

func (suite *HandlerTestSuite) TestHandleApplicationPutRequest_ServiceError() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	appRequest := model.ApplicationRequest{
		OUID: "ou-123",
		Name: "UpdatedApp",
	}

	svcErr := &ErrorInvalidApplicationName

	mockService.On("UpdateApplication", mock.Anything, "test-app-id",
		mock.AnythingOfType("*model.ApplicationDTO")).
		Return(nil, svcErr)

	body, _ := json.Marshal(appRequest)
	req := httptest.NewRequest(http.MethodPut, "/applications/test-app-id", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "test-app-id")
	w := httptest.NewRecorder()

	handler.HandleApplicationPutRequest(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	assert.Equal(suite.T(), "application/json", w.Header().Get("Content-Type"))

	var errResp apierror.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &errResp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), ErrorInvalidApplicationName.Code, errResp.Code)

	mockService.AssertExpectations(suite.T())
}

func (suite *HandlerTestSuite) TestHandleApplicationPutRequest_NotFound() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	appRequest := model.ApplicationRequest{
		OUID: "ou-123",
		Name: "UpdatedApp",
	}

	svcErr := &ErrorApplicationNotFound

	mockService.On("UpdateApplication", mock.Anything, "non-existent-id", mock.AnythingOfType("*model.ApplicationDTO")).
		Return(nil, svcErr)

	body, _ := json.Marshal(appRequest)
	req := httptest.NewRequest(http.MethodPut, "/applications/non-existent-id", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "non-existent-id")
	w := httptest.NewRecorder()

	handler.HandleApplicationPutRequest(w, req)

	assert.Equal(suite.T(), http.StatusNotFound, w.Code)

	mockService.AssertExpectations(suite.T())
}

func (suite *HandlerTestSuite) TestHandleApplicationDeleteRequest_Success() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	mockService.On("DeleteApplication", mock.Anything, "test-app-id").Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/applications/test-app-id", nil)
	req.SetPathValue("id", "test-app-id")
	w := httptest.NewRecorder()

	handler.HandleApplicationDeleteRequest(w, req)

	assert.Equal(suite.T(), http.StatusNoContent, w.Code)

	mockService.AssertExpectations(suite.T())
}

func (suite *HandlerTestSuite) TestHandleApplicationDeleteRequest_InvalidID() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	req := httptest.NewRequest(http.MethodDelete, "/applications/", nil)
	req.SetPathValue("id", "")
	w := httptest.NewRecorder()

	handler.HandleApplicationDeleteRequest(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	assert.Equal(suite.T(), "application/json", w.Header().Get("Content-Type"))

	var errResp apierror.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &errResp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), ErrorInvalidApplicationID.Code, errResp.Code)
}

func (suite *HandlerTestSuite) TestHandleApplicationDeleteRequest_NotFound() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	svcErr := &ErrorApplicationNotFound

	mockService.On("DeleteApplication", mock.Anything, "non-existent-id").Return(svcErr)

	req := httptest.NewRequest(http.MethodDelete, "/applications/non-existent-id", nil)
	req.SetPathValue("id", "non-existent-id")
	w := httptest.NewRecorder()

	handler.HandleApplicationDeleteRequest(w, req)

	assert.Equal(suite.T(), http.StatusNotFound, w.Code)

	mockService.AssertExpectations(suite.T())
}

func (suite *HandlerTestSuite) TestHandleApplicationDeleteRequest_ServiceError() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	svcErr := &tidcommon.InternalServerError

	mockService.On("DeleteApplication", mock.Anything, "test-app-id").Return(svcErr)

	req := httptest.NewRequest(http.MethodDelete, "/applications/test-app-id", nil)
	req.SetPathValue("id", "test-app-id")
	w := httptest.NewRecorder()

	handler.HandleApplicationDeleteRequest(w, req)

	assert.Equal(suite.T(), http.StatusInternalServerError, w.Code)

	mockService.AssertExpectations(suite.T())
}

func (suite *HandlerTestSuite) TestProcessInboundAuthConfig_Success() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)
	logger := log.GetLogger()

	appDTO := &model.ApplicationDTO{
		ID:   "test-app-id",
		Name: "TestApp",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID:                "test-client-id",
					ClientSecret:            "test-secret",
					RedirectURIs:            []string{"https://example.com/callback"},
					GrantTypes:              []providers.GrantType{providers.GrantTypeAuthorizationCode},
					ResponseTypes:           []providers.ResponseType{providers.ResponseTypeCode},
					TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
				},
			},
		},
	}

	returnApp := &model.ApplicationCompleteResponse{
		ID:   "test-app-id",
		Name: "TestApp",
	}

	success := handler.processInboundAuthConfig(context.Background(), logger, appDTO, returnApp)

	assert.True(suite.T(), success)
	assert.NotNil(suite.T(), returnApp.InboundAuthConfig)
	assert.Len(suite.T(), returnApp.InboundAuthConfig, 1)
	assert.Equal(suite.T(), "test-client-id", returnApp.ClientID)
}

func (suite *HandlerTestSuite) TestProcessInboundAuthConfig_EmptyRedirectURIs() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)
	logger := log.GetLogger()

	appDTO := &model.ApplicationDTO{
		ID:   "test-app-id",
		Name: "TestApp",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID:                "test-client-id",
					ClientSecret:            "test-secret",
					RedirectURIs:            nil, // Empty redirect URIs
					GrantTypes:              []providers.GrantType{providers.GrantTypeAuthorizationCode},
					ResponseTypes:           []providers.ResponseType{providers.ResponseTypeCode},
					TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
				},
			},
		},
	}

	returnApp := &model.ApplicationCompleteResponse{
		ID:   "test-app-id",
		Name: "TestApp",
	}

	success := handler.processInboundAuthConfig(context.Background(), logger, appDTO, returnApp)

	assert.True(suite.T(), success)
	assert.NotNil(suite.T(), returnApp.InboundAuthConfig)
	assert.Empty(suite.T(), returnApp.InboundAuthConfig[0].OAuthConfig.RedirectURIs)
}

func (suite *HandlerTestSuite) TestProcessInboundAuthConfig_EmptyGrantTypes() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)
	logger := log.GetLogger()

	appDTO := &model.ApplicationDTO{
		ID:   "test-app-id",
		Name: "TestApp",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID:                "test-client-id",
					ClientSecret:            "test-secret",
					RedirectURIs:            []string{"https://example.com/callback"},
					GrantTypes:              nil, // Empty grant types
					ResponseTypes:           []providers.ResponseType{providers.ResponseTypeCode},
					TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
				},
			},
		},
	}

	returnApp := &model.ApplicationCompleteResponse{
		ID:   "test-app-id",
		Name: "TestApp",
	}

	success := handler.processInboundAuthConfig(context.Background(), logger, appDTO, returnApp)

	assert.True(suite.T(), success)
	assert.NotNil(suite.T(), returnApp.InboundAuthConfig)
	assert.Empty(suite.T(), returnApp.InboundAuthConfig[0].OAuthConfig.GrantTypes)
}

func (suite *HandlerTestSuite) TestProcessInboundAuthConfig_UnsupportedType() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)
	logger := log.GetLogger()

	appDTO := &model.ApplicationDTO{
		ID:   "test-app-id",
		Name: "TestApp",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type:        "unsupported",
				OAuthConfig: nil,
			},
		},
	}

	returnApp := &model.ApplicationCompleteResponse{
		ID:   "test-app-id",
		Name: "TestApp",
	}

	success := handler.processInboundAuthConfig(context.Background(), logger, appDTO, returnApp)

	assert.False(suite.T(), success)
}

func (suite *HandlerTestSuite) TestProcessInboundAuthConfig_NilOAuthConfig() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)
	logger := log.GetLogger()

	appDTO := &model.ApplicationDTO{
		ID:   "test-app-id",
		Name: "TestApp",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type:        providers.OAuthInboundAuthType,
				OAuthConfig: nil,
			},
		},
	}

	returnApp := &model.ApplicationCompleteResponse{
		ID:   "test-app-id",
		Name: "TestApp",
	}

	success := handler.processInboundAuthConfig(context.Background(), logger, appDTO, returnApp)

	assert.False(suite.T(), success)
}

func (suite *HandlerTestSuite) TestProcessInboundAuthConfig_EmptyInboundAuthConfig() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)
	logger := log.GetLogger()

	appDTO := &model.ApplicationDTO{
		ID:                "test-app-id",
		Name:              "TestApp",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{},
	}

	returnApp := &model.ApplicationCompleteResponse{
		ID:   "test-app-id",
		Name: "TestApp",
	}

	success := handler.processInboundAuthConfig(context.Background(), logger, appDTO, returnApp)

	assert.True(suite.T(), success)
}

func (suite *HandlerTestSuite) TestHandleError_ClientError() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/applications", nil)

	svcErr := &ErrorInvalidApplicationName

	handler.handleError(context.Background(), w, r, svcErr)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	assert.Equal(suite.T(), "application/json", w.Header().Get("Content-Type"))

	var errResp apierror.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &errResp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), ErrorInvalidApplicationName.Code, errResp.Code)
}

func (suite *HandlerTestSuite) TestHandleError_NotFoundError() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/applications", nil)

	svcErr := &ErrorApplicationNotFound

	handler.handleError(context.Background(), w, r, svcErr)

	assert.Equal(suite.T(), http.StatusNotFound, w.Code)
	assert.Equal(suite.T(), "application/json", w.Header().Get("Content-Type"))

	var errResp apierror.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &errResp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), ErrorApplicationNotFound.Code, errResp.Code)
}

func (suite *HandlerTestSuite) TestHandleError_ServerError() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/applications", nil)

	svcErr := &tidcommon.InternalServerError

	handler.handleError(context.Background(), w, r, svcErr)

	assert.Equal(suite.T(), http.StatusInternalServerError, w.Code)
	assert.Equal(suite.T(), "application/json", w.Header().Get("Content-Type"))

	var errResp apierror.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &errResp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, errResp.Code)
}

func (suite *HandlerTestSuite) TestProcessInboundAuthConfigFromRequest_Success() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	configs := []providers.InboundAuthConfigWithSecret{
		{
			Type: providers.OAuthInboundAuthType,
			OAuthConfig: &providers.OAuthConfigWithSecret{
				ClientID:                "test-client-id",
				ClientSecret:            "test-secret",
				RedirectURIs:            []string{"https://example.com/callback"},
				GrantTypes:              []providers.GrantType{providers.GrantTypeAuthorizationCode},
				ResponseTypes:           []providers.ResponseType{providers.ResponseTypeCode},
				TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
			},
		},
	}

	result := handler.processInboundAuthConfigFromRequest(configs)

	assert.NotNil(suite.T(), result)
	assert.Len(suite.T(), result, 1)
	assert.Equal(suite.T(), providers.OAuthInboundAuthType, result[0].Type)
	assert.Equal(suite.T(), "test-client-id", result[0].OAuthConfig.ClientID)
}

func (suite *HandlerTestSuite) TestProcessInboundAuthConfigFromRequest_EmptyConfigs() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	configs := []providers.InboundAuthConfigWithSecret{}

	result := handler.processInboundAuthConfigFromRequest(configs)

	assert.Nil(suite.T(), result)
}

func (suite *HandlerTestSuite) TestProcessInboundAuthConfigFromRequest_NilConfigs() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	result := handler.processInboundAuthConfigFromRequest(nil)

	assert.Nil(suite.T(), result)
}

func (suite *HandlerTestSuite) TestProcessInboundAuthConfigFromRequest_UnsupportedType() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	configs := []providers.InboundAuthConfigWithSecret{
		{
			Type:        "unsupported",
			OAuthConfig: nil,
		},
	}

	result := handler.processInboundAuthConfigFromRequest(configs)

	assert.NotNil(suite.T(), result)
	assert.Len(suite.T(), result, 0) // Should skip unsupported types
}

func (suite *HandlerTestSuite) TestProcessInboundAuthConfigFromRequest_NilOAuthConfig() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	configs := []providers.InboundAuthConfigWithSecret{
		{
			Type:        providers.OAuthInboundAuthType,
			OAuthConfig: nil,
		},
	}

	result := handler.processInboundAuthConfigFromRequest(configs)

	assert.NotNil(suite.T(), result)
	assert.Len(suite.T(), result, 0) // Should skip configs with nil OAuth config
}

func (suite *HandlerTestSuite) TestProcessInboundAuthConfigFromRequest_MultipleConfigs() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	configs := []providers.InboundAuthConfigWithSecret{
		{
			Type: providers.OAuthInboundAuthType,
			OAuthConfig: &providers.OAuthConfigWithSecret{
				ClientID:     "client-1",
				ClientSecret: "secret-1",
			},
		},
		{
			Type: providers.OAuthInboundAuthType,
			OAuthConfig: &providers.OAuthConfigWithSecret{
				ClientID:     "client-2",
				ClientSecret: "secret-2",
			},
		},
	}

	result := handler.processInboundAuthConfigFromRequest(configs)

	assert.NotNil(suite.T(), result)
	assert.Len(suite.T(), result, 2)
	assert.Equal(suite.T(), "client-1", result[0].OAuthConfig.ClientID)
	assert.Equal(suite.T(), "client-2", result[1].OAuthConfig.ClientID)
}

func (suite *HandlerTestSuite) TestProcessInboundAuthConfigFromRequest_WithTokenConfig() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	configs := []providers.InboundAuthConfigWithSecret{
		{
			Type: providers.OAuthInboundAuthType,
			OAuthConfig: &providers.OAuthConfigWithSecret{
				ClientID:     "test-client-id",
				ClientSecret: "test-secret",
				Token: &providers.OAuthTokenConfig{
					AccessToken: &providers.AccessTokenConfig{
						UserConfig: &providers.AccessTokenSubConfig{
							ValidityPeriod: 3600,
							Attributes:     []string{"email", "name"},
						},
					},
					IDToken: &providers.IDTokenConfig{
						ValidityPeriod: 3600,
						UserAttributes: []string{"email"},
					},
				},
			},
		},
	}

	result := handler.processInboundAuthConfigFromRequest(configs)

	assert.NotNil(suite.T(), result)
	assert.Len(suite.T(), result, 1)
	assert.NotNil(suite.T(), result[0].OAuthConfig.Token)
}

func (suite *HandlerTestSuite) TestProcessInboundAuthConfigFromRequest_WithScopes() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	configs := []providers.InboundAuthConfigWithSecret{
		{
			Type: providers.OAuthInboundAuthType,
			OAuthConfig: &providers.OAuthConfigWithSecret{
				ClientID:     "test-client-id",
				ClientSecret: "test-secret",
				Scopes:       []string{"openid", "profile", "email"},
			},
		},
	}

	result := handler.processInboundAuthConfigFromRequest(configs)

	assert.NotNil(suite.T(), result)
	assert.Len(suite.T(), result, 1)
	assert.Equal(suite.T(), []string{"openid", "profile", "email"}, result[0].OAuthConfig.Scopes)
}

func (suite *HandlerTestSuite) TestProcessInboundAuthConfigFromRequest_PublicClient() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	configs := []providers.InboundAuthConfigWithSecret{
		{
			Type: providers.OAuthInboundAuthType,
			OAuthConfig: &providers.OAuthConfigWithSecret{
				ClientID:                "test-client-id",
				PublicClient:            true,
				PKCERequired:            true,
				TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodNone,
			},
		},
	}

	result := handler.processInboundAuthConfigFromRequest(configs)

	assert.NotNil(suite.T(), result)
	assert.Len(suite.T(), result, 1)
	assert.True(suite.T(), result[0].OAuthConfig.PublicClient)
	assert.True(suite.T(), result[0].OAuthConfig.PKCERequired)
}

func (suite *HandlerTestSuite) TestHandleApplicationPostRequest_WithCertificate() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	appRequest := model.ApplicationRequest{
		OUID:        "ou-123",
		Name:        "TestApp",
		Description: "Test Description",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					Certificate: &inboundmodel.Certificate{
						Type:  cert.CertificateTypeJWKS,
						Value: `{"keys":[{"kty":"RSA","kid":"test"}]}`,
					},
				},
			},
		},
	}

	expectedApp := &model.ApplicationDTO{
		OUID:        "ou-123",
		ID:          "test-app-id",
		Name:        "TestApp",
		Description: "Test Description",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					Certificate: &inboundmodel.Certificate{
						Type:  cert.CertificateTypeJWKS,
						Value: `{"keys":[{"kty":"RSA","kid":"test"}]}`,
					},
				},
			},
		},
	}

	mockService.On("CreateApplication", mock.Anything, mock.AnythingOfType("*model.ApplicationDTO")).
		Return(expectedApp, nil)

	body, _ := json.Marshal(appRequest)
	req := httptest.NewRequest(http.MethodPost, "/applications", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleApplicationPostRequest(w, req)

	assert.Equal(suite.T(), http.StatusCreated, w.Code)

	mockService.AssertExpectations(suite.T())
}

func (suite *HandlerTestSuite) TestHandleApplicationGetRequest_WithEmptyArrays() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	expectedApp := &providers.Application{
		OUID:        "ou-123",
		ID:          "test-app-id",
		Name:        "TestApp",
		Description: "Test Description",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID:                "test-client-id",
					RedirectURIs:            []string{}, // Empty array
					GrantTypes:              []providers.GrantType{},
					ResponseTypes:           []providers.ResponseType{},
					TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
				},
			},
		},
	}

	mockService.On("GetApplication", mock.Anything, "test-app-id").Return(expectedApp, nil)

	req := httptest.NewRequest(http.MethodGet, "/applications/test-app-id", nil)
	req.SetPathValue("id", "test-app-id")
	w := httptest.NewRecorder()

	handler.HandleApplicationGetRequest(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)

	var response model.ApplicationGetResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), response.InboundAuthConfig)
	assert.Empty(suite.T(), response.InboundAuthConfig[0].OAuthConfig.RedirectURIs)
	assert.Empty(suite.T(), response.InboundAuthConfig[0].OAuthConfig.GrantTypes)
	assert.Empty(suite.T(), response.InboundAuthConfig[0].OAuthConfig.ResponseTypes)

	mockService.AssertExpectations(suite.T())
}

func (suite *HandlerTestSuite) TestHandleApplicationPutRequest_WithOAuth() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	appRequest := model.ApplicationRequest{
		OUID:        "ou-123",
		Name:        "UpdatedApp",
		Description: "Updated Description",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID:                "updated-client-id",
					ClientSecret:            "updated-secret",
					RedirectURIs:            []string{"https://example.com/callback"},
					GrantTypes:              []providers.GrantType{providers.GrantTypeAuthorizationCode},
					ResponseTypes:           []providers.ResponseType{providers.ResponseTypeCode},
					TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
				},
			},
		},
	}

	expectedApp := &model.ApplicationDTO{
		ID:          "test-app-id",
		OUID:        "ou-123",
		Name:        "UpdatedApp",
		Description: "Updated Description",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID:                "updated-client-id",
					ClientSecret:            "updated-secret",
					RedirectURIs:            []string{"https://example.com/callback"},
					GrantTypes:              []providers.GrantType{providers.GrantTypeAuthorizationCode},
					ResponseTypes:           []providers.ResponseType{providers.ResponseTypeCode},
					TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
				},
			},
		},
	}

	mockService.On("UpdateApplication", mock.Anything, "test-app-id",
		mock.AnythingOfType("*model.ApplicationDTO")).
		Return(expectedApp, nil)

	body, _ := json.Marshal(appRequest)
	req := httptest.NewRequest(http.MethodPut, "/applications/test-app-id", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "test-app-id")
	w := httptest.NewRecorder()

	handler.HandleApplicationPutRequest(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)

	var response model.ApplicationCompleteResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "updated-client-id", response.ClientID)

	mockService.AssertExpectations(suite.T())
}

// failingResponseWriter is a mock http.ResponseWriter that fails on Write
type failingResponseWriter struct {
	header        http.Header
	statusCode    int
	failOnce      bool
	writeCount    int
	headerWritten bool
}

func (f *failingResponseWriter) Header() http.Header {
	if f.header == nil {
		f.header = make(http.Header)
	}
	return f.header
}

func (f *failingResponseWriter) Write(b []byte) (int, error) {
	// Auto-set status code if not set
	if !f.headerWritten {
		f.statusCode = http.StatusOK
		f.headerWritten = true
	}

	f.writeCount++
	if f.failOnce || f.writeCount > 1 {
		return 0, assert.AnError
	}
	return len(b), nil
}

func (f *failingResponseWriter) WriteHeader(statusCode int) {
	// Only allow setting status code once
	if !f.headerWritten {
		f.statusCode = statusCode
		f.headerWritten = true
	}
}

func (suite *HandlerTestSuite) TestHandleApplicationListRequest_EncodeResponseError() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	listResponse := &model.ApplicationListResponse{
		Applications: []model.BasicApplicationResponse{
			{
				ID:          "test-app-1",
				Name:        "Test App 1",
				Description: "Test Description 1",
			},
		},
		TotalResults: 1,
		Count:        1,
	}

	mockService.On("GetApplicationList", mock.Anything).Return(listResponse, nil)

	req := httptest.NewRequest(http.MethodGet, "/applications", nil)
	w := &failingResponseWriter{failOnce: true}

	handler.HandleApplicationListRequest(w, req)

	// Should attempt to write but fail - verify service was called
	mockService.AssertExpectations(suite.T())
}

func (suite *HandlerTestSuite) TestHandleApplicationGetRequest_EncodeErrorResponseFails() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	// Test with empty id - should try to encode error response
	req := httptest.NewRequest(http.MethodGet, "/applications/", nil)
	req.SetPathValue("id", "")
	w := &failingResponseWriter{failOnce: true}

	handler.HandleApplicationGetRequest(w, req)

	// Error response encoding should fail
	assert.Equal(suite.T(), http.StatusBadRequest, w.statusCode)
}

func (suite *HandlerTestSuite) TestHandleApplicationPutRequest_EncodeErrorResponseFailsOnEmptyID() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	req := httptest.NewRequest(http.MethodPut, "/applications/", nil)
	req.SetPathValue("id", "")
	w := &failingResponseWriter{failOnce: true}

	handler.HandleApplicationPutRequest(w, req)

	// Error response encoding should fail
	assert.Equal(suite.T(), http.StatusBadRequest, w.statusCode)
}

func (suite *HandlerTestSuite) TestHandleApplicationPutRequest_EncodeErrorResponseFailsOnInvalidJSON() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	// Invalid JSON body
	req := httptest.NewRequest(http.MethodPut, "/applications/test-id", bytes.NewBufferString("{invalid json"))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "test-id")
	w := &failingResponseWriter{failOnce: true}

	handler.HandleApplicationPutRequest(w, req)

	// Error response encoding should fail
	assert.Equal(suite.T(), http.StatusBadRequest, w.statusCode)
}

func (suite *HandlerTestSuite) TestHandleApplicationPostRequest_EncodeResponseError() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	appRequest := model.ApplicationRequest{
		OUID:        "ou-123",
		Name:        "Test App",
		Description: "Test Description",
	}

	createdApp := &model.ApplicationDTO{
		ID:          "test-app-id",
		Name:        "Test App",
		Description: "Test Description",
		OUID:        "ou-123",
	}

	mockService.On("CreateApplication", mock.Anything, mock.AnythingOfType("*model.ApplicationDTO")).
		Return(createdApp, nil)

	body, _ := json.Marshal(appRequest)
	req := httptest.NewRequest(http.MethodPost, "/applications", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := &failingResponseWriter{failOnce: true}

	handler.HandleApplicationPostRequest(w, req)

	// Should attempt to write response but fail
	mockService.AssertExpectations(suite.T())
	assert.Equal(suite.T(), http.StatusCreated, w.statusCode)
}

func (suite *HandlerTestSuite) TestHandleApplicationPutRequest_EncodeResponseError() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	appRequest := model.ApplicationRequest{
		OUID:        "ou-123",
		Name:        "Updated App",
		Description: "Updated Description",
	}

	updatedApp := &model.ApplicationDTO{
		ID:          "test-app-id",
		OUID:        "ou-123",
		Name:        "Updated App",
		Description: "Updated Description",
	}

	mockService.On("UpdateApplication", mock.Anything, "test-app-id",
		mock.AnythingOfType("*model.ApplicationDTO")).
		Return(updatedApp, nil)

	body, _ := json.Marshal(appRequest)
	req := httptest.NewRequest(http.MethodPut, "/applications/test-app-id", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "test-app-id")
	w := &failingResponseWriter{failOnce: true}

	handler.HandleApplicationPutRequest(w, req)

	// Should attempt to write response but fail
	mockService.AssertExpectations(suite.T())
	assert.Equal(suite.T(), http.StatusOK, w.statusCode)
}

func (suite *HandlerTestSuite) TestHandleApplicationGetRequest_EncodeResponseError() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	expectedApp := &providers.Application{
		ID:          "test-app-id",
		Name:        "Test App",
		Description: "Test Description",
	}

	mockService.On("GetApplication", mock.Anything, "test-app-id").Return(expectedApp, nil)

	req := httptest.NewRequest(http.MethodGet, "/applications/test-app-id", nil)
	req.SetPathValue("id", "test-app-id")
	w := &failingResponseWriter{failOnce: true}

	handler.HandleApplicationGetRequest(w, req)

	// Should attempt to write response but fail
	mockService.AssertExpectations(suite.T())
	assert.Equal(suite.T(), http.StatusOK, w.statusCode)
}

func (suite *HandlerTestSuite) TestHandleApplicationPostRequest_MultipleInboundAuthConfigs() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	// Test with multiple OAuth configs (edge case - should only process first one properly)
	appRequest := model.ApplicationRequest{
		OUID:        "ou-123",
		Name:        "Multi Config App",
		Description: "App with multiple inbound auth configs",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID:                "client-1",
					ClientSecret:            "secret-1",
					RedirectURIs:            []string{"https://example1.com/callback"},
					GrantTypes:              []providers.GrantType{providers.GrantTypeAuthorizationCode},
					ResponseTypes:           []providers.ResponseType{providers.ResponseTypeCode},
					TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
				},
			},
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID:                "client-2",
					ClientSecret:            "secret-2",
					RedirectURIs:            []string{"https://example2.com/callback"},
					GrantTypes:              []providers.GrantType{providers.GrantTypeClientCredentials},
					ResponseTypes:           []providers.ResponseType{},
					TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretPost,
				},
			},
		},
	}

	createdApp := &model.ApplicationDTO{
		ID:          "multi-config-app-id",
		Name:        "Multi Config App",
		Description: "App with multiple inbound auth configs",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID:                "client-1",
					ClientSecret:            "secret-1",
					RedirectURIs:            []string{"https://example1.com/callback"},
					GrantTypes:              []providers.GrantType{providers.GrantTypeAuthorizationCode},
					ResponseTypes:           []providers.ResponseType{providers.ResponseTypeCode},
					TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
				},
			},
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID:                "client-2",
					ClientSecret:            "secret-2",
					RedirectURIs:            []string{"https://example2.com/callback"},
					GrantTypes:              []providers.GrantType{providers.GrantTypeClientCredentials},
					ResponseTypes:           []providers.ResponseType{},
					TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretPost,
				},
			},
		},
	}

	mockService.On("CreateApplication", mock.Anything, mock.AnythingOfType("*model.ApplicationDTO")).
		Return(createdApp, nil)

	body, _ := json.Marshal(appRequest)
	req := httptest.NewRequest(http.MethodPost, "/applications", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleApplicationPostRequest(w, req)

	assert.Equal(suite.T(), http.StatusCreated, w.Code)

	var response model.ApplicationCompleteResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "client-1", response.ClientID)
	// Should have both inbound auth configs in response
	assert.Len(suite.T(), response.InboundAuthConfig, 2)

	mockService.AssertExpectations(suite.T())
}

func (suite *HandlerTestSuite) TestHandleError_EncodeErrorResponseFails() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	mockService.On("GetApplication", mock.Anything, "test-id").Return(nil, &ErrorApplicationNotFound)

	req := httptest.NewRequest(http.MethodGet, "/applications/test-id", nil)
	req.SetPathValue("id", "test-id")
	w := &failingResponseWriter{failOnce: true}

	handler.HandleApplicationGetRequest(w, req)

	// Should try to encode error response but fail
	assert.Equal(suite.T(), http.StatusNotFound, w.statusCode)
	mockService.AssertExpectations(suite.T())
}

func (suite *HandlerTestSuite) TestHandleApplicationDeleteRequest_EncodeErrorResponseFails() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	// Test with empty id to trigger error encoding
	req := httptest.NewRequest(http.MethodDelete, "/applications/", nil)
	req.SetPathValue("id", "")
	w := &failingResponseWriter{failOnce: true}

	handler.HandleApplicationDeleteRequest(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.statusCode)
}

func (suite *HandlerTestSuite) TestHandleApplicationPostRequest_UnsupportedInboundAuthType() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	appRequest := model.ApplicationRequest{
		OUID:        "ou-123",
		Name:        "TestApp",
		Description: "Test Description",
	}

	// Service returns app with unsupported auth type
	createdApp := &model.ApplicationDTO{
		OUID:        "ou-123",
		ID:          "test-app-id",
		Name:        "TestApp",
		Description: "Test Description",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: "UNSUPPORTED_TYPE", // Not OAuthInboundAuthType
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID: "test-client-id",
				},
			},
		},
	}

	mockService.On("CreateApplication", mock.Anything, mock.AnythingOfType("*model.ApplicationDTO")).
		Return(createdApp, nil)

	body, _ := json.Marshal(appRequest)
	req := httptest.NewRequest(http.MethodPost, "/applications", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleApplicationPostRequest(w, req)

	// Should return 500 because processInboundAuthConfig returns false
	assert.Equal(suite.T(), http.StatusInternalServerError, w.Code)
	assert.Equal(suite.T(), "application/json", w.Header().Get("Content-Type"))

	var errResp apierror.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &errResp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, errResp.Code)

	mockService.AssertExpectations(suite.T())
}

func (suite *HandlerTestSuite) TestHandleApplicationPostRequest_NilOAuthConfig() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	appRequest := model.ApplicationRequest{
		OUID:        "ou-123",
		Name:        "TestApp",
		Description: "Test Description",
	}

	// Service returns app with OAuth auth type but nil config
	createdApp := &model.ApplicationDTO{
		OUID:        "ou-123",
		ID:          "test-app-id",
		Name:        "TestApp",
		Description: "Test Description",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type:        providers.OAuthInboundAuthType,
				OAuthConfig: nil, // Nil OAuth config
			},
		},
	}

	mockService.On("CreateApplication", mock.Anything, mock.AnythingOfType("*model.ApplicationDTO")).
		Return(createdApp, nil)

	body, _ := json.Marshal(appRequest)
	req := httptest.NewRequest(http.MethodPost, "/applications", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleApplicationPostRequest(w, req)

	// Should return 500 because processInboundAuthConfig returns false
	assert.Equal(suite.T(), http.StatusInternalServerError, w.Code)
	assert.Equal(suite.T(), "application/json", w.Header().Get("Content-Type"))

	var errResp apierror.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &errResp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, errResp.Code)

	mockService.AssertExpectations(suite.T())
}

func (suite *HandlerTestSuite) TestHandleApplicationPostRequest_ProcessInboundAuthConfigErrorEncodingFails() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	appRequest := model.ApplicationRequest{
		OUID:        "ou-123",
		Name:        "TestApp",
		Description: "Test Description",
	}

	// Service returns app with unsupported auth type
	createdApp := &model.ApplicationDTO{
		OUID:        "ou-123",
		ID:          "test-app-id",
		Name:        "TestApp",
		Description: "Test Description",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type:        "UNSUPPORTED_TYPE",
				OAuthConfig: &providers.OAuthConfigWithSecret{ClientID: "test"},
			},
		},
	}

	mockService.On("CreateApplication", mock.Anything, mock.AnythingOfType("*model.ApplicationDTO")).
		Return(createdApp, nil)

	body, _ := json.Marshal(appRequest)
	req := httptest.NewRequest(http.MethodPost, "/applications", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := &failingResponseWriter{failOnce: true}

	handler.HandleApplicationPostRequest(w, req)

	// Should have set status to 500
	assert.Equal(suite.T(), http.StatusInternalServerError, w.statusCode)
	mockService.AssertExpectations(suite.T())
}

func (suite *HandlerTestSuite) TestHandleApplicationPostRequest_SuccessResponseEncodingFails() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	appRequest := model.ApplicationRequest{
		OUID:        "ou-123",
		Name:        "TestApp",
		Description: "Test Description",
	}

	expectedApp := &model.ApplicationDTO{
		OUID:        "ou-123",
		ID:          "test-app-id",
		Name:        "TestApp",
		Description: "Test Description",
	}

	mockService.On("CreateApplication", mock.Anything, mock.AnythingOfType("*model.ApplicationDTO")).
		Return(expectedApp, nil)

	body, _ := json.Marshal(appRequest)
	req := httptest.NewRequest(http.MethodPost, "/applications", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := &failingResponseWriter{failOnce: true}

	handler.HandleApplicationPostRequest(w, req)

	// Should have set status to 201 before encoding fails
	assert.Equal(suite.T(), http.StatusCreated, w.statusCode)
	mockService.AssertExpectations(suite.T())
}

func (suite *HandlerTestSuite) TestHandleApplicationPutRequest_UnsupportedInboundAuthType() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	appRequest := model.ApplicationRequest{
		OUID:        "ou-123",
		Name:        "Updated App",
		Description: "Updated Description",
	}

	// Service returns app with unsupported auth type
	updatedApp := &model.ApplicationDTO{
		OUID:        "ou-123",
		ID:          "test-app-id",
		Name:        "Updated App",
		Description: "Updated Description",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: "UNSUPPORTED_TYPE",
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID: "test-client-id",
				},
			},
		},
	}

	mockService.On("UpdateApplication", mock.Anything, "test-app-id",
		mock.AnythingOfType("*model.ApplicationDTO")).
		Return(updatedApp, nil)

	body, _ := json.Marshal(appRequest)
	req := httptest.NewRequest(http.MethodPut, "/applications/test-app-id", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "test-app-id")
	w := httptest.NewRecorder()

	handler.HandleApplicationPutRequest(w, req)

	// Should return 500 because processInboundAuthConfig returns false
	assert.Equal(suite.T(), http.StatusInternalServerError, w.Code)
	assert.Equal(suite.T(), "application/json", w.Header().Get("Content-Type"))

	var errResp apierror.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &errResp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, errResp.Code)

	mockService.AssertExpectations(suite.T())
}

func (suite *HandlerTestSuite) TestHandleApplicationPutRequest_NilOAuthConfig() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	appRequest := model.ApplicationRequest{
		OUID:        "ou-123",
		Name:        "Updated App",
		Description: "Updated Description",
	}

	// Service returns app with OAuth auth type but nil config
	updatedApp := &model.ApplicationDTO{
		OUID:        "ou-123",
		ID:          "test-app-id",
		Name:        "Updated App",
		Description: "Updated Description",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type:        providers.OAuthInboundAuthType,
				OAuthConfig: nil,
			},
		},
	}

	mockService.On("UpdateApplication", mock.Anything, "test-app-id",
		mock.AnythingOfType("*model.ApplicationDTO")).
		Return(updatedApp, nil)

	body, _ := json.Marshal(appRequest)
	req := httptest.NewRequest(http.MethodPut, "/applications/test-app-id", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "test-app-id")
	w := httptest.NewRecorder()

	handler.HandleApplicationPutRequest(w, req)

	// Should return 500 because processInboundAuthConfig returns false
	assert.Equal(suite.T(), http.StatusInternalServerError, w.Code)
	assert.Equal(suite.T(), "application/json", w.Header().Get("Content-Type"))

	var errResp apierror.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &errResp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, errResp.Code)

	mockService.AssertExpectations(suite.T())
}

func (suite *HandlerTestSuite) TestHandleApplicationPutRequest_ProcessInboundAuthConfigErrorEncodingFails() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	appRequest := model.ApplicationRequest{
		OUID:        "ou-123",
		Name:        "Updated App",
		Description: "Updated Description",
	}

	updatedApp := &model.ApplicationDTO{
		OUID:        "ou-123",
		ID:          "test-app-id",
		Name:        "Updated App",
		Description: "Updated Description",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type:        "UNSUPPORTED_TYPE",
				OAuthConfig: &providers.OAuthConfigWithSecret{ClientID: "test"},
			},
		},
	}

	mockService.On("UpdateApplication", mock.Anything, "test-app-id",
		mock.AnythingOfType("*model.ApplicationDTO")).
		Return(updatedApp, nil)

	body, _ := json.Marshal(appRequest)
	req := httptest.NewRequest(http.MethodPut, "/applications/test-app-id", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "test-app-id")
	w := &failingResponseWriter{failOnce: true}

	handler.HandleApplicationPutRequest(w, req)

	// Should have set status to 500
	assert.Equal(suite.T(), http.StatusInternalServerError, w.statusCode)
	mockService.AssertExpectations(suite.T())
}

func (suite *HandlerTestSuite) TestHandleApplicationPutRequest_SuccessResponseEncodingFails() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	appRequest := model.ApplicationRequest{
		OUID:        "ou-123",
		Name:        "Updated App",
		Description: "Updated Description",
	}

	updatedApp := &model.ApplicationDTO{
		OUID:        "ou-123",
		ID:          "test-app-id",
		Name:        "Updated App",
		Description: "Updated Description",
	}

	mockService.On("UpdateApplication", mock.Anything, "test-app-id",
		mock.AnythingOfType("*model.ApplicationDTO")).
		Return(updatedApp, nil)

	body, _ := json.Marshal(appRequest)
	req := httptest.NewRequest(http.MethodPut, "/applications/test-app-id", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "test-app-id")
	w := &failingResponseWriter{failOnce: true}

	handler.HandleApplicationPutRequest(w, req)

	// Should have set status to 200 before encoding fails
	assert.Equal(suite.T(), http.StatusOK, w.statusCode)
	mockService.AssertExpectations(suite.T())
}

func (suite *HandlerTestSuite) TestHandleApplicationPutRequest_InvalidJSONErrorEncodingFails() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	req := httptest.NewRequest(http.MethodPut, "/applications/test-id", bytes.NewBufferString("{invalid json}"))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "test-id")
	w := &failingResponseWriter{failOnce: true}

	handler.HandleApplicationPutRequest(w, req)

	// Should have set status to 400
	assert.Equal(suite.T(), http.StatusBadRequest, w.statusCode)
}

func (suite *HandlerTestSuite) TestHandleApplicationPutRequest_EmptyIDErrorEncodingFails() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	appRequest := model.ApplicationRequest{
		OUID: "ou-123",
		Name: "Test",
	}

	body, _ := json.Marshal(appRequest)
	req := httptest.NewRequest(http.MethodPut, "/applications/", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "")
	w := &failingResponseWriter{failOnce: true}

	handler.HandleApplicationPutRequest(w, req)

	// Should have set status to 400
	assert.Equal(suite.T(), http.StatusBadRequest, w.statusCode)
}

func (suite *HandlerTestSuite) TestHandleApplicationPostRequest_InvalidJSONErrorEncodingFails() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	req := httptest.NewRequest(http.MethodPost, "/applications", bytes.NewBufferString("{invalid json}"))
	req.Header.Set("Content-Type", "application/json")
	w := &failingResponseWriter{failOnce: true}

	handler.HandleApplicationPostRequest(w, req)

	// Should have set status to 400 before encoding fails
	assert.Equal(suite.T(), http.StatusBadRequest, w.statusCode)
}

func (suite *HandlerTestSuite) TestHandleApplicationGetRequest_UnsupportedAuthTypeErrorEncodingFails() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	// Return app with unsupported auth type
	app := &providers.Application{
		ID:   "test-app-id",
		Name: "TestApp",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type:        "UNSUPPORTED_TYPE", // Not OAuthInboundAuthType
				OAuthConfig: &providers.OAuthConfigWithSecret{ClientID: "test-client-id"},
			},
		},
	}
	mockService.On("GetApplication", mock.Anything, "test-app-id").Return(app, nil)

	req := httptest.NewRequest(http.MethodGet, "/applications/test-app-id", nil)
	req.SetPathValue("id", "test-app-id")
	w := &failingResponseWriter{failOnce: true}

	handler.HandleApplicationGetRequest(w, req)

	// Should have set status to 500 before encoding fails
	assert.Equal(suite.T(), http.StatusInternalServerError, w.statusCode)
	mockService.AssertExpectations(suite.T())
}

func (suite *HandlerTestSuite) TestHandleApplicationGetRequest_NilOAuthConfigErrorEncodingFails() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	// Return app with nil OAuth config
	app := &providers.Application{
		ID:   "test-app-id",
		Name: "TestApp",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type:        providers.OAuthInboundAuthType,
				OAuthConfig: nil, // Nil OAuth config
			},
		},
	}
	mockService.On("GetApplication", mock.Anything, "test-app-id").Return(app, nil)

	req := httptest.NewRequest(http.MethodGet, "/applications/test-app-id", nil)
	req.SetPathValue("id", "test-app-id")
	w := &failingResponseWriter{failOnce: true}

	handler.HandleApplicationGetRequest(w, req)

	// Should have set status to 500 before encoding fails
	assert.Equal(suite.T(), http.StatusInternalServerError, w.statusCode)
	mockService.AssertExpectations(suite.T())
}

func (suite *HandlerTestSuite) TestHandleApplicationGetRequest_EmptyResponseTypes() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	// Return app with empty response types and grant types
	app := &providers.Application{
		ID:   "test-app-id",
		Name: "TestApp",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID:      "test-client-id",
					GrantTypes:    []providers.GrantType{},    // Empty grant types
					ResponseTypes: []providers.ResponseType{}, // Empty response types
				},
			},
		},
	}
	mockService.On("GetApplication", mock.Anything, "test-app-id").Return(app, nil)

	req := httptest.NewRequest(http.MethodGet, "/applications/test-app-id", nil)
	req.SetPathValue("id", "test-app-id")
	w := httptest.NewRecorder()

	handler.HandleApplicationGetRequest(w, req)

	// Should return 200 OK - handler properly handles empty arrays
	assert.Equal(suite.T(), http.StatusOK, w.Code)

	// Verify we got a valid JSON response
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), response)

	mockService.AssertExpectations(suite.T())
}

func (suite *HandlerTestSuite) TestHandleApplicationPostRequest_ForwardsPasskeyAllowedOrigins() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	origins := []string{"https://app.example.com", "https://other.example.com"}
	appRequest := model.ApplicationRequest{
		OUID: "ou-123",
		Name: "TestApp",
	}
	appRequest.PasskeyAllowedOrigins = origins

	expectedApp := &model.ApplicationDTO{ID: "test-app-id", Name: "TestApp"}
	expectedApp.PasskeyAllowedOrigins = origins

	mockService.On("CreateApplication", mock.Anything,
		mock.MatchedBy(func(dto *model.ApplicationDTO) bool {
			return assert.Equal(suite.T(), origins, dto.PasskeyAllowedOrigins)
		}),
	).Return(expectedApp, nil)

	body, _ := json.Marshal(appRequest)
	req := httptest.NewRequest(http.MethodPost, "/applications", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleApplicationPostRequest(w, req)

	assert.Equal(suite.T(), http.StatusCreated, w.Code)
	mockService.AssertExpectations(suite.T())
}

func (suite *HandlerTestSuite) TestHandleApplicationPutRequest_ForwardsPasskeyAllowedOrigins() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	origins := []string{"https://app.example.com"}
	appRequest := model.ApplicationRequest{
		OUID: "ou-123",
		Name: "UpdatedApp",
	}
	appRequest.PasskeyAllowedOrigins = origins

	expectedApp := &model.ApplicationDTO{ID: "test-app-id", Name: "UpdatedApp"}
	expectedApp.PasskeyAllowedOrigins = origins

	mockService.On("UpdateApplication", mock.Anything, "test-app-id",
		mock.MatchedBy(func(dto *model.ApplicationDTO) bool {
			return assert.Equal(suite.T(), origins, dto.PasskeyAllowedOrigins)
		}),
	).Return(expectedApp, nil)

	body, _ := json.Marshal(appRequest)
	req := httptest.NewRequest(http.MethodPut, "/applications/test-app-id", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "test-app-id")
	w := httptest.NewRecorder()

	handler.HandleApplicationPutRequest(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)
	mockService.AssertExpectations(suite.T())
}

// ----- Sharing policy handlers -----

// sharingRequest builds a request carrying the path values the mux would have set.
func sharingRequest(method, target string, body interface{}, pathValues map[string]string,
) *http.Request {
	var reader *bytes.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, target, reader)
	for k, v := range pathValues {
		req.SetPathValue(k, v)
	}
	return req
}

// The framework refuses a limit below one, so a listing that names none has to send a page size
// rather than a zero.
func (suite *HandlerTestSuite) TestSharingPaginationDefaultsToAPage() {
	limit, offset, svcErr := parseSharingPagination(url.Values{})

	suite.Require().Nil(svcErr)
	suite.Equal(serverconst.DefaultPageSize, limit)
	suite.Equal(0, offset)
}

// A value the caller did send is passed through, bounds included, because the bounds are the
// framework's to enforce and its refusal names which one was wrong.
func (suite *HandlerTestSuite) TestSharingPaginationPassesThroughWhatWasAsked() {
	limit, offset, svcErr := parseSharingPagination(url.Values{
		"limit": {"5"}, "offset": {"10"}})

	suite.Require().Nil(svcErr)
	suite.Equal(5, limit)
	suite.Equal(10, offset)
}

// A value that is not a number never reaches the framework, and the refusal names which parameter.
func (suite *HandlerTestSuite) TestSharingPaginationRefusesNonNumbers() {
	_, _, svcErr := parseSharingPagination(url.Values{"limit": {"all"}})
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorInvalidLimit.Code, svcErr.Code)

	_, _, svcErr = parseSharingPagination(url.Values{"offset": {"back"}})
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorInvalidOffset.Code, svcErr.Code)
}

// A page the handler cannot parse and a page the framework refuses are the same mistake, so the
// caller gets the same answer either way. Without the translation below, limit=0 would come back
// as APP-1051 and describe a policy the caller never sent.
func (suite *HandlerTestSuite) TestSharingPaginationAnswersAlikeFromEitherSide() {
	for _, tc := range []struct {
		name      string
		unparsed  url.Values
		fromStack *tidcommon.ServiceError
		want      tidcommon.ServiceError
	}{
		{
			name:      "limit",
			unparsed:  url.Values{"limit": {"all"}},
			fromStack: &sharing.ErrorInvalidLimit,
			want:      ErrorInvalidLimit,
		},
		{
			name:      "offset",
			unparsed:  url.Values{"offset": {"back"}},
			fromStack: &sharing.ErrorInvalidOffset,
			want:      ErrorInvalidOffset,
		},
	} {
		suite.Run(tc.name, func() {
			_, _, parseErr := parseSharingPagination(tc.unparsed)
			suite.Require().NotNil(parseErr)

			translated := translateSharingError(tc.fromStack)
			suite.Require().NotNil(translated)

			suite.Equal(tc.want.Code, parseErr.Code, "the handler answers in this package's namespace")
			suite.Equal(tc.want.Code, translated.Code, "and so does the framework's own refusal")
			suite.Equal(parseErr.Code, translated.Code)
			suite.NotEqual(ErrorInvalidSharingPolicy.Code, translated.Code,
				"a bad page is not an invalid policy")
		})
	}
}

// The status separates the three kinds of refusal a sharing call produces: a malformed request, a
// policy that is not there, and a conflict with state the caller has not seen.
func (suite *HandlerTestSuite) TestSharingErrorsCarryTheirOwnStatus() {
	for _, tc := range []struct {
		name       string
		svcErr     *tidcommon.ServiceError
		wantStatus int
	}{
		{"a policy the unit may not write", &ErrorInvalidSharingPolicy, http.StatusBadRequest},
		{"no such policy", &ErrorSharingPolicyNotFound, http.StatusNotFound},
		{"a second policy for one unit", &ErrorSharingPolicyExists, http.StatusConflict},
		{"a stale version", &ErrorSharingPolicyVersionMismatch, http.StatusConflict},
		{"a declared policy", &ErrorSharingPolicyDeclared, http.StatusBadRequest},
		{"an unshared organization unit", &ErrorApplicationNotSharedToOU, http.StatusNotFound},
		{"a framework failure", &tidcommon.InternalServerError, http.StatusInternalServerError},
	} {
		suite.Run(tc.name, func() {
			mockService := NewApplicationServiceInterfaceMock(suite.T())
			handler := newApplicationHandler(mockService)
			mockService.EXPECT().GetSharingPolicy(mock.Anything, "app-1", "p1").
				Return(sharing.Policy{}, tc.svcErr)

			w := httptest.NewRecorder()
			handler.HandleSharingPolicyGetRequest(w,
				sharingRequest(http.MethodGet, "/applications/app-1/sharing-policies/p1", nil,
					map[string]string{"id": "app-1", "policyId": "p1"}))

			suite.Equal(tc.wantStatus, w.Code)
			var body map[string]interface{}
			suite.Require().NoError(json.Unmarshal(w.Body.Bytes(), &body))
			suite.Equal(tc.svcErr.Code, body["code"])
		})
	}
}

// A create answers 201 with the recorded policy.
func (suite *HandlerTestSuite) TestSharingPolicyPostAnswersCreated() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)
	mockService.EXPECT().CreateSharingPolicy(mock.Anything, "app-1", mock.Anything).
		Return(sharing.Policy{ID: "p1", Version: 1,
			Targets: []sharing.Target{{ID: "t1", Scope: sharing.ScopeAllChildren, OUID: "root"}}}, nil)

	w := httptest.NewRecorder()
	handler.HandleSharingPolicyPostRequest(w, sharingRequest(http.MethodPost,
		"/applications/app-1/sharing-policies",
		map[string]interface{}{"targets": []map[string]string{{"scope": "allChildren"}}},
		map[string]string{"id": "app-1"}))

	suite.Require().Equal(http.StatusCreated, w.Code, "body: %s", w.Body.String())
	var body map[string]interface{}
	suite.Require().NoError(json.Unmarshal(w.Body.Bytes(), &body))
	suite.Equal("p1", body["id"])
}

// A body that is not a policy request is refused before the service is asked anything.
func (suite *HandlerTestSuite) TestSharingPolicyPostRefusesAMalformedBody() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/applications/app-1/sharing-policies",
		bytes.NewReader([]byte("{not json")))
	req.SetPathValue("id", "app-1")
	handler.HandleSharingPolicyPostRequest(w, req)

	suite.Equal(http.StatusBadRequest, w.Code)
}

// A field name the body gets wrong is refused rather than dropped. The spec marks every target
// additionalProperties: false, and nothing validates a request against the spec at runtime, so
// without this a misspelled excludedOuIds would read as a target with no exclusions and share the
// application with the organization unit the caller named to keep it from.
func (suite *HandlerTestSuite) TestSharingPolicyPostRefusesAnUnknownField() {
	for _, tc := range []struct {
		name string
		body map[string]interface{}
	}{
		{
			name: "a misspelled exclusion silently widens the share",
			body: map[string]interface{}{"targets": []map[string]interface{}{
				{"scope": "allChildren", "excludeOuIds": []string{"acme-trial"}}}},
		},
		{
			name: "an unknown field beside the targets",
			body: map[string]interface{}{
				"targets":       []map[string]string{{"scope": "allChildren"}},
				"totallyMadeUp": "x"},
		},
	} {
		suite.Run(tc.name, func() {
			mockService := NewApplicationServiceInterfaceMock(suite.T())
			handler := newApplicationHandler(mockService)

			w := httptest.NewRecorder()
			handler.HandleSharingPolicyPostRequest(w, sharingRequest(http.MethodPost,
				"/applications/app-1/sharing-policies", tc.body, map[string]string{"id": "app-1"}))

			// The service mock records no call: the body never gets that far.
			suite.Equal(http.StatusBadRequest, w.Code, "body: %s", w.Body.String())
		})
	}
}

// The spelling the spec gives still decodes, so the refusal above is about the unknown name and not
// about exclusions in general.
func (suite *HandlerTestSuite) TestSharingPolicyPostAcceptsExclusions() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)
	var recorded sharing.PolicyRequest
	mockService.EXPECT().CreateSharingPolicy(mock.Anything, "app-1", mock.Anything).
		Run(func(_ context.Context, _ string, req sharing.PolicyRequest) { recorded = req }).
		Return(sharing.Policy{ID: "p1", Version: 1}, nil)

	w := httptest.NewRecorder()
	handler.HandleSharingPolicyPostRequest(w, sharingRequest(http.MethodPost,
		"/applications/app-1/sharing-policies",
		map[string]interface{}{"targets": []map[string]interface{}{
			{"scope": "allChildren", "excludedOuIds": []string{"acme-trial"}}}},
		map[string]string{"id": "app-1"}))

	suite.Require().Equal(http.StatusCreated, w.Code, "body: %s", w.Body.String())
	suite.Equal([]string{"acme-trial"}, recorded.Targets[0].ExcludedOUIDs)
}

// A delete answers 204 with no body.
func (suite *HandlerTestSuite) TestSharingPolicyDeleteAnswersNoContent() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)
	mockService.EXPECT().DeleteSharingPolicy(mock.Anything, "app-1", "p1").Return(nil)

	w := httptest.NewRecorder()
	handler.HandleSharingPolicyDeleteRequest(w, sharingRequest(http.MethodDelete,
		"/applications/app-1/sharing-policies/p1", nil,
		map[string]string{"id": "app-1", "policyId": "p1"}))

	suite.Equal(http.StatusNoContent, w.Code)
	suite.Empty(w.Body.String())
}

// The organization unit to resolve for travels as a query parameter, and the empty collections
// come back as collections rather than as null, so a client can read them without a nil check.
func (suite *HandlerTestSuite) TestOverlayRulesReadsTheUnitFromTheQuery() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)
	mockService.EXPECT().ResolveSharingOverlay(mock.Anything, "app-1", "child-a").
		Return(sharing.ResolvedOverlay{OUID: "child-a", Visible: true}, nil)

	w := httptest.NewRecorder()
	handler.HandleOverlayRuleGetRequest(w, sharingRequest(http.MethodGet,
		"/applications/app-1/overlay-rules?ouId=child-a", nil, map[string]string{"id": "app-1"}))

	suite.Require().Equal(http.StatusOK, w.Code)
	var body map[string]interface{}
	suite.Require().NoError(json.Unmarshal(w.Body.Bytes(), &body))
	suite.Equal("child-a", body["ouId"])
	suite.Equal(true, body["visible"])
	suite.NotNil(body["rules"], "an absent rule set is an empty object, not null")
	suite.NotNil(body["policyIds"], "an absent policy list is an empty array, not null")
}

// A listing shapes every policy it returns and carries the page counts through.
func (suite *HandlerTestSuite) TestSharingPolicyListShapesThePage() {
	mockService := NewApplicationServiceInterfaceMock(suite.T())
	handler := newApplicationHandler(mockService)
	mockService.EXPECT().ListSharingPolicies(mock.Anything, "app-1", serverconst.DefaultPageSize, 0).
		Return(sharing.PolicyList{
			TotalResults: 1, StartIndex: 1, Count: 1,
			Policies: []sharing.Policy{{ID: "p1", Targets: []sharing.Target{
				{ID: "t1", Scope: sharing.ScopeChild, OUID: "child-a"}}}},
		}, nil)

	w := httptest.NewRecorder()
	handler.HandleSharingPolicyListRequest(w, sharingRequest(http.MethodGet,
		"/applications/app-1/sharing-policies", nil, map[string]string{"id": "app-1"}))

	suite.Require().Equal(http.StatusOK, w.Code)
	var body sharing.PolicyListResponse
	suite.Require().NoError(json.Unmarshal(w.Body.Bytes(), &body))
	suite.Equal(1, body.TotalResults)
	suite.Require().Len(body.Policies, 1)
	suite.Equal("child-a", body.Policies[0].Targets[0].OUID)
}
