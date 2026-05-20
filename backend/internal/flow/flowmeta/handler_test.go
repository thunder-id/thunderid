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

package flowmeta

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
)

// mockFlowMetaService is a manual mock for FlowMetaServiceInterface to avoid import cycles
type mockFlowMetaService struct {
	mock.Mock
}

func (m *mockFlowMetaService) GetFlowMetadata(
	ctx context.Context,
	metaType MetaType,
	id string,
	language *string,
	namespace *string,
) (*FlowMetadataResponse, *serviceerror.ServiceError) {
	args := m.Called(ctx, metaType, id, language, namespace)
	if args.Get(1) != nil {
		return nil, args.Get(1).(*serviceerror.ServiceError)
	}
	return args.Get(0).(*FlowMetadataResponse), nil
}

// Test Suite

type FlowMetaHandlerTestSuite struct {
	suite.Suite
	mockService *mockFlowMetaService
	handler     *flowMetaHandler
}

func TestFlowMetaHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(FlowMetaHandlerTestSuite))
}

func (suite *FlowMetaHandlerTestSuite) SetupTest() {
	suite.mockService = new(mockFlowMetaService)
	suite.handler = newFlowMetaHandler(suite.mockService)
}

func (suite *FlowMetaHandlerTestSuite) TearDownTest() {
	suite.mockService.AssertExpectations(suite.T())
}

func (suite *FlowMetaHandlerTestSuite) TestHandleGetFlowMetadata_Success_AppType() {
	// Arrange
	appID := "60a9b38b-6eba-9f9e-55f9-267067de4680"
	metaType := MetaTypeAPP
	language := "en"
	namespace := "auth"

	expectedResponse := &FlowMetadataResponse{
		IsRegistrationFlowEnabled: true,
		Application: &ApplicationMetadata{
			ID:        appID,
			Name:      "Test App",
			LogoURL:   "https://example.com/logo.png",
			URL:       "https://example.com",
			TosURI:    "https://example.com/tos",
			PolicyURI: "https://example.com/policy",
		},
		OU: &OUMetadata{
			ID:      "ou-123",
			Handle:  "default",
			Name:    "Default OU",
			LogoURL: "https://example.com/ou-logo.png",
		},
		Design: DesignMetadata{
			Theme:  json.RawMessage(`{"primary":"#000"}`),
			Layout: json.RawMessage(`{"header":"simple"}`),
		},
		I18n: I18nMetadata{
			Languages:    []string{"en", "es"},
			Language:     "en",
			TotalResults: 2,
			Translations: map[string]map[string]string{
				"auth": {
					"login.button": "Login",
					"login.title":  "Welcome",
				},
			},
		},
	}

	suite.mockService.On("GetFlowMetadata", mock.Anything, metaType, appID, &language, &namespace).
		Return(expectedResponse, nil)

	req := httptest.NewRequest(http.MethodGet, "/flow/meta?type=APP&id="+appID+"&language=en&namespace=auth", nil)
	w := httptest.NewRecorder()

	// Act
	suite.handler.HandleGetFlowMetadata(w, req)

	// Assert
	assert.Equal(suite.T(), http.StatusOK, w.Code)

	var response FlowMetadataResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), expectedResponse.IsRegistrationFlowEnabled, response.IsRegistrationFlowEnabled)
	assert.Equal(suite.T(), expectedResponse.Application.ID, response.Application.ID)
	assert.Equal(suite.T(), expectedResponse.OU.Handle, response.OU.Handle)
}

func (suite *FlowMetaHandlerTestSuite) TestHandleGetFlowMetadata_Success_OUType() {
	// Arrange
	ouID := "fe447a2f-29c5-4e33-ac8f-d77be15fdb32"
	metaType := MetaTypeOU

	expectedResponse := &FlowMetadataResponse{
		IsRegistrationFlowEnabled: false,
		OU: &OUMetadata{
			ID:              ouID,
			Handle:          "engineering",
			Name:            "Engineering OU",
			LogoURL:         "https://example.com/eng-logo.png",
			TosURI:          "https://example.com/tos",
			PolicyURI:       "https://example.com/policy",
			CookiePolicyURI: "https://example.com/cookies",
		},
		Design: DesignMetadata{
			Theme:  json.RawMessage(`{}`),
			Layout: json.RawMessage(`{}`),
		},
		I18n: I18nMetadata{
			Languages:    []string{"en"},
			Language:     "en",
			TotalResults: 0,
			Translations: map[string]map[string]string{},
		},
	}

	suite.mockService.On("GetFlowMetadata", mock.Anything, metaType, ouID, (*string)(nil), (*string)(nil)).
		Return(expectedResponse, nil)

	req := httptest.NewRequest(http.MethodGet, "/flow/meta?type=OU&id="+ouID, nil)
	w := httptest.NewRecorder()

	// Act
	suite.handler.HandleGetFlowMetadata(w, req)

	// Assert
	assert.Equal(suite.T(), http.StatusOK, w.Code)

	var response FlowMetadataResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.False(suite.T(), response.IsRegistrationFlowEnabled)
	assert.Nil(suite.T(), response.Application)
	assert.Equal(suite.T(), expectedResponse.OU.Handle, response.OU.Handle)
}

func (suite *FlowMetaHandlerTestSuite) TestHandleGetFlowMetadata_SystemFlow_NoParams() {
	// Arrange: no type or id — system flow, returns i18n only
	expectedResponse := &FlowMetadataResponse{
		IsRegistrationFlowEnabled: false,
		Design: DesignMetadata{
			Theme:  json.RawMessage(`{}`),
			Layout: json.RawMessage(`{}`),
		},
		I18n: I18nMetadata{
			Languages:    []string{"en-US"},
			Language:     "en-US",
			TotalResults: 5,
			Translations: map[string]map[string]string{
				"system": {"key": "value"},
			},
		},
	}

	suite.mockService.On("GetFlowMetadata", mock.Anything, MetaType(""), "", (*string)(nil), (*string)(nil)).
		Return(expectedResponse, nil)

	req := httptest.NewRequest(http.MethodGet, "/flow/meta", nil)
	w := httptest.NewRecorder()

	// Act
	suite.handler.HandleGetFlowMetadata(w, req)

	// Assert
	assert.Equal(suite.T(), http.StatusOK, w.Code)

	var response FlowMetadataResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.False(suite.T(), response.IsRegistrationFlowEnabled)
	assert.Nil(suite.T(), response.Application)
	assert.Nil(suite.T(), response.OU)
}

func (suite *FlowMetaHandlerTestSuite) TestHandleGetFlowMetadata_MissingID_WhenTypeProvided() {
	// Arrange: type is provided but id is missing — id is required when type is set
	req := httptest.NewRequest(http.MethodGet, "/flow/meta?type=APP", nil)
	w := httptest.NewRecorder()

	// Act
	suite.handler.HandleGetFlowMetadata(w, req)

	// Assert
	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

func (suite *FlowMetaHandlerTestSuite) TestHandleGetFlowMetadata_MissingType_WhenIDProvided() {
	// Arrange: id is provided but type is missing — type is required when id is set
	req := httptest.NewRequest(http.MethodGet, "/flow/meta?id=some-id", nil)
	w := httptest.NewRecorder()

	// Act
	suite.handler.HandleGetFlowMetadata(w, req)

	// Assert
	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)

	var errorResp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &errorResp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), ErrorMissingType.Code, errorResp["code"])
}

func (suite *FlowMetaHandlerTestSuite) TestHandleGetFlowMetadata_InvalidType() {
	// Arrange
	suite.mockService.On("GetFlowMetadata", mock.Anything,
		MetaType("INVALID"), "some-id", (*string)(nil), (*string)(nil)).
		Return(nil, &ErrorInvalidType)

	req := httptest.NewRequest(http.MethodGet, "/flow/meta?type=INVALID&id=some-id", nil)
	w := httptest.NewRecorder()

	// Act
	suite.handler.HandleGetFlowMetadata(w, req)

	// Assert
	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

func (suite *FlowMetaHandlerTestSuite) TestHandleGetFlowMetadata_ServiceError_NotFound() {
	// Arrange
	appID := "non-existent-id"
	metaType := MetaTypeAPP

	suite.mockService.On("GetFlowMetadata", mock.Anything, metaType, appID, (*string)(nil), (*string)(nil)).
		Return(nil, &ErrorApplicationNotFound)

	req := httptest.NewRequest(http.MethodGet, "/flow/meta?type=APP&id="+appID, nil)
	w := httptest.NewRecorder()

	// Act
	suite.handler.HandleGetFlowMetadata(w, req)

	// Assert
	assert.Equal(suite.T(), http.StatusNotFound, w.Code)
}

func (suite *FlowMetaHandlerTestSuite) TestHandleGetFlowMetadata_ServiceError_Internal() {
	// Arrange
	appID := "some-id"
	metaType := MetaTypeAPP

	suite.mockService.On("GetFlowMetadata", mock.Anything, metaType, appID, (*string)(nil), (*string)(nil)).
		Return(nil, &serviceerror.InternalServerError)

	req := httptest.NewRequest(http.MethodGet, "/flow/meta?type=APP&id="+appID, nil)
	w := httptest.NewRecorder()

	// Act
	suite.handler.HandleGetFlowMetadata(w, req)

	// Assert
	assert.Equal(suite.T(), http.StatusInternalServerError, w.Code)
}

func (suite *FlowMetaHandlerTestSuite) TestHandleGetFlowMetadata_WithLanguageParam() {
	// Arrange
	appID := testAppID
	metaType := MetaTypeAPP
	language := "es"

	expectedResponse := &FlowMetadataResponse{
		IsRegistrationFlowEnabled: true,
		Application: &ApplicationMetadata{
			ID:   appID,
			Name: "Test App",
		},
		OU: &OUMetadata{
			ID:     "ou-123",
			Handle: "default",
			Name:   "Default OU",
		},
		Design: DesignMetadata{
			Theme:  json.RawMessage(`{}`),
			Layout: json.RawMessage(`{}`),
		},
		I18n: I18nMetadata{
			Languages:    []string{"en", "es"},
			Language:     "es",
			TotalResults: 1,
			Translations: map[string]map[string]string{},
		},
	}

	suite.mockService.On("GetFlowMetadata", mock.Anything, metaType, appID, &language, (*string)(nil)).
		Return(expectedResponse, nil)

	req := httptest.NewRequest(http.MethodGet, "/flow/meta?type=APP&id="+appID+"&language=es", nil)
	w := httptest.NewRecorder()

	// Act
	suite.handler.HandleGetFlowMetadata(w, req)

	// Assert
	assert.Equal(suite.T(), http.StatusOK, w.Code)

	var response FlowMetadataResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "es", response.I18n.Language)
}

func (suite *FlowMetaHandlerTestSuite) TestHandleGetFlowMetadata_WithNamespaceParam() {
	// Arrange
	appID := testAppID
	metaType := MetaTypeAPP
	namespace := "errors"

	expectedResponse := &FlowMetadataResponse{
		IsRegistrationFlowEnabled: true,
		Application: &ApplicationMetadata{
			ID:   appID,
			Name: "Test App",
		},
		OU: &OUMetadata{
			ID:     "ou-123",
			Handle: "default",
			Name:   "Default OU",
		},
		Design: DesignMetadata{
			Theme:  json.RawMessage(`{}`),
			Layout: json.RawMessage(`{}`),
		},
		I18n: I18nMetadata{
			Languages:    []string{"en"},
			Language:     "en",
			TotalResults: 1,
			Translations: map[string]map[string]string{
				"errors": {
					"general.error": "An error occurred",
				},
			},
		},
	}

	suite.mockService.On("GetFlowMetadata", mock.Anything, metaType, appID, (*string)(nil), &namespace).
		Return(expectedResponse, nil)

	req := httptest.NewRequest(http.MethodGet, "/flow/meta?type=APP&id="+appID+"&namespace=errors", nil)
	w := httptest.NewRecorder()

	// Act
	suite.handler.HandleGetFlowMetadata(w, req)

	// Assert
	assert.Equal(suite.T(), http.StatusOK, w.Code)

	var response FlowMetadataResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), response.I18n.Translations, "errors")
}
