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

package thememgt

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
)

// Test Suite
type ThemeHandlerTestSuite struct {
	suite.Suite
}

func TestThemeHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(ThemeHandlerTestSuite))
}

// mockThemeService implements ThemeMgtServiceInterface for handler tests
type mockThemeService struct {
	getThemeListFunc func(limit, offset int) (*ThemeList, *serviceerror.ServiceError)
	createThemeFunc  func(theme CreateThemeRequestWithID) (*Theme, *serviceerror.ServiceError)
	getThemeFunc     func(id string) (*Theme, *serviceerror.ServiceError)
	updateThemeFunc  func(id string, theme UpdateThemeRequest) (*Theme, *serviceerror.ServiceError)
	deleteThemeFunc  func(id string) *serviceerror.ServiceError
	isThemeExistFunc func(id string) (bool, *serviceerror.ServiceError)
}

func (m *mockThemeService) GetThemeList(limit, offset int) (*ThemeList, *serviceerror.ServiceError) {
	return m.getThemeListFunc(limit, offset)
}

func (m *mockThemeService) CreateTheme(theme CreateThemeRequestWithID) (*Theme, *serviceerror.ServiceError) {
	return m.createThemeFunc(theme)
}

func (m *mockThemeService) GetTheme(id string) (*Theme, *serviceerror.ServiceError) {
	return m.getThemeFunc(id)
}

func (m *mockThemeService) UpdateTheme(id string, theme UpdateThemeRequest) (*Theme, *serviceerror.ServiceError) {
	return m.updateThemeFunc(id, theme)
}

func (m *mockThemeService) DeleteTheme(id string) *serviceerror.ServiceError {
	return m.deleteThemeFunc(id)
}

func (m *mockThemeService) IsThemeExist(id string) (bool, *serviceerror.ServiceError) {
	return m.isThemeExistFunc(id)
}

// Test HandleThemeListRequest - Success
func (suite *ThemeHandlerTestSuite) TestHandleThemeListRequest_Success() {
	themeList := &ThemeList{
		TotalResults: 2,
		StartIndex:   1,
		Count:        2,
		Themes: []Theme{
			{ID: "theme-1", DisplayName: "Theme 1", Description: "Desc 1"},
			{ID: "theme-2", DisplayName: "Theme 2", Description: "Desc 2"},
		},
		Links: []Link{},
	}

	mockSvc := &mockThemeService{
		getThemeListFunc: func(limit, offset int) (*ThemeList, *serviceerror.ServiceError) {
			return themeList, nil
		},
	}

	handler := newThemeMgtHandler(mockSvc)
	req := httptest.NewRequest(http.MethodGet, "/design/themes?limit=10&offset=0", nil)
	w := httptest.NewRecorder()

	handler.HandleThemeListRequest(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)

	var response ThemeListResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), 2, response.TotalResults)
	assert.Len(suite.T(), response.Themes, 2)
}

// Test HandleThemeListRequest - color fields are populated from theme JSON
func (suite *ThemeHandlerTestSuite) TestHandleThemeListRequest_ColorFieldsPopulated() {
	themeList := &ThemeList{
		TotalResults: 1,
		StartIndex:   1,
		Count:        1,
		Themes: []Theme{
			{
				ID:          "theme-1",
				DisplayName: "Theme 1",
				Description: "Desc 1",
				Theme: json.RawMessage(`{
					"defaultColorScheme": "light",
					"colorSchemes": {
						"light": {"palette": {"primary": {"main": "#ff7300"}}}
					}
				}`),
			},
		},
		Links: []Link{},
	}

	mockSvc := &mockThemeService{
		getThemeListFunc: func(limit, offset int) (*ThemeList, *serviceerror.ServiceError) {
			return themeList, nil
		},
	}

	handler := newThemeMgtHandler(mockSvc)
	req := httptest.NewRequest(http.MethodGet, "/design/themes", nil)
	w := httptest.NewRecorder()

	handler.HandleThemeListRequest(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)

	var response ThemeListResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), response.Themes, 1)
	assert.Equal(suite.T(), "light", response.Themes[0].DefaultColorScheme)
	assert.Equal(suite.T(), "#ff7300", response.Themes[0].PrimaryColor)
}

// Test HandleThemeListRequest - color fields are empty when theme JSON is absent
func (suite *ThemeHandlerTestSuite) TestHandleThemeListRequest_EmptyColorFieldsWhenNoThemeJSON() {
	themeList := &ThemeList{
		TotalResults: 1,
		StartIndex:   1,
		Count:        1,
		Themes: []Theme{
			{ID: "theme-1", DisplayName: "Theme 1", Description: "Desc 1"},
		},
		Links: []Link{},
	}

	mockSvc := &mockThemeService{
		getThemeListFunc: func(limit, offset int) (*ThemeList, *serviceerror.ServiceError) {
			return themeList, nil
		},
	}

	handler := newThemeMgtHandler(mockSvc)
	req := httptest.NewRequest(http.MethodGet, "/design/themes", nil)
	w := httptest.NewRecorder()

	handler.HandleThemeListRequest(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)

	var response ThemeListResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), response.Themes, 1)
	assert.Equal(suite.T(), "", response.Themes[0].DefaultColorScheme)
	assert.Equal(suite.T(), "", response.Themes[0].PrimaryColor)
}

// Test HandleThemeListRequest - Invalid pagination
func (suite *ThemeHandlerTestSuite) TestHandleThemeListRequest_InvalidLimit() {
	handler := newThemeMgtHandler(&mockThemeService{})
	req := httptest.NewRequest(http.MethodGet, "/design/themes?limit=abc", nil)
	w := httptest.NewRecorder()

	handler.HandleThemeListRequest(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

// Test HandleThemeListRequest - Invalid offset
func (suite *ThemeHandlerTestSuite) TestHandleThemeListRequest_InvalidOffset() {
	handler := newThemeMgtHandler(&mockThemeService{})
	req := httptest.NewRequest(http.MethodGet, "/design/themes?offset=abc", nil)
	w := httptest.NewRecorder()

	handler.HandleThemeListRequest(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

// Test HandleThemeListRequest - Service error
func (suite *ThemeHandlerTestSuite) TestHandleThemeListRequest_ServiceError() {
	mockSvc := &mockThemeService{
		getThemeListFunc: func(limit, offset int) (*ThemeList, *serviceerror.ServiceError) {
			return nil, &serviceerror.InternalServerError
		},
	}

	handler := newThemeMgtHandler(mockSvc)
	req := httptest.NewRequest(http.MethodGet, "/design/themes", nil)
	w := httptest.NewRecorder()

	handler.HandleThemeListRequest(w, req)

	assert.Equal(suite.T(), http.StatusInternalServerError, w.Code)
}

// Test HandleThemePostRequest - Success
func (suite *ThemeHandlerTestSuite) TestHandleThemePostRequest_Success() {
	createdTheme := &Theme{
		ID:          "theme-new",
		DisplayName: "New Theme",
		Description: "A new theme",
		Theme:       json.RawMessage(`{"colors": {"primary": "#ff0000"}}`),
	}

	mockSvc := &mockThemeService{
		createThemeFunc: func(theme CreateThemeRequestWithID) (*Theme, *serviceerror.ServiceError) {
			return createdTheme, nil
		},
	}

	handler := newThemeMgtHandler(mockSvc)
	body, _ := json.Marshal(CreateThemeRequest{
		DisplayName: "New Theme",
		Description: "A new theme",
		Theme:       json.RawMessage(`{"colors": {"primary": "#ff0000"}}`),
	})
	req := httptest.NewRequest(http.MethodPost, "/design/themes", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleThemePostRequest(w, req)

	assert.Equal(suite.T(), http.StatusCreated, w.Code)

	var response Theme
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "theme-new", response.ID)
}

// Test HandleThemePostRequest - Invalid JSON body
func (suite *ThemeHandlerTestSuite) TestHandleThemePostRequest_InvalidJSON() {
	handler := newThemeMgtHandler(&mockThemeService{})
	req := httptest.NewRequest(http.MethodPost, "/design/themes", bytes.NewBufferString("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleThemePostRequest(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

// Test HandleThemePostRequest - Service error
func (suite *ThemeHandlerTestSuite) TestHandleThemePostRequest_ServiceError() {
	mockSvc := &mockThemeService{
		createThemeFunc: func(theme CreateThemeRequestWithID) (*Theme, *serviceerror.ServiceError) {
			return nil, &serviceerror.InternalServerError
		},
	}

	handler := newThemeMgtHandler(mockSvc)
	body, _ := json.Marshal(CreateThemeRequest{
		DisplayName: "Theme",
		Theme:       json.RawMessage(`{"colors": {}}`),
	})
	req := httptest.NewRequest(http.MethodPost, "/design/themes", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleThemePostRequest(w, req)

	assert.Equal(suite.T(), http.StatusInternalServerError, w.Code)
}

// Test HandleThemeGetRequest - Success
func (suite *ThemeHandlerTestSuite) TestHandleThemeGetRequest_Success() {
	theme := &Theme{
		ID:          "theme-123",
		DisplayName: "Test Theme",
		Description: "A test theme",
		Theme:       json.RawMessage(`{"colors": {"primary": "#007bff"}}`),
	}

	mockSvc := &mockThemeService{
		getThemeFunc: func(id string) (*Theme, *serviceerror.ServiceError) {
			return theme, nil
		},
	}

	handler := newThemeMgtHandler(mockSvc)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /design/themes/{id}", handler.HandleThemeGetRequest)

	req := httptest.NewRequest(http.MethodGet, "/design/themes/theme-123", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)

	var response Theme
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "theme-123", response.ID)
}

// Test HandleThemeGetRequest - Not found
func (suite *ThemeHandlerTestSuite) TestHandleThemeGetRequest_NotFound() {
	mockSvc := &mockThemeService{
		getThemeFunc: func(id string) (*Theme, *serviceerror.ServiceError) {
			return nil, &ErrorThemeNotFound
		},
	}

	handler := newThemeMgtHandler(mockSvc)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /design/themes/{id}", handler.HandleThemeGetRequest)

	req := httptest.NewRequest(http.MethodGet, "/design/themes/non-existent", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusNotFound, w.Code)
}

// Test HandleThemePutRequest - Success
func (suite *ThemeHandlerTestSuite) TestHandleThemePutRequest_Success() {
	updatedTheme := &Theme{
		ID:          "theme-123",
		DisplayName: "Updated Theme",
		Description: "Updated desc",
		Theme:       json.RawMessage(`{"colors": {"primary": "#00ff00"}}`),
	}

	mockSvc := &mockThemeService{
		updateThemeFunc: func(id string, theme UpdateThemeRequest) (*Theme, *serviceerror.ServiceError) {
			return updatedTheme, nil
		},
	}

	handler := newThemeMgtHandler(mockSvc)
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /design/themes/{id}", handler.HandleThemePutRequest)

	body, _ := json.Marshal(UpdateThemeRequest{
		DisplayName: "Updated Theme",
		Description: "Updated desc",
		Theme:       json.RawMessage(`{"colors": {"primary": "#00ff00"}}`),
	})
	req := httptest.NewRequest(http.MethodPut, "/design/themes/theme-123", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)
}

// Test HandleThemePutRequest - Invalid JSON
func (suite *ThemeHandlerTestSuite) TestHandleThemePutRequest_InvalidJSON() {
	handler := newThemeMgtHandler(&mockThemeService{})
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /design/themes/{id}", handler.HandleThemePutRequest)

	req := httptest.NewRequest(http.MethodPut, "/design/themes/theme-123", bytes.NewBufferString("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

// Test HandleThemeDeleteRequest - Success
func (suite *ThemeHandlerTestSuite) TestHandleThemeDeleteRequest_Success() {
	mockSvc := &mockThemeService{
		deleteThemeFunc: func(id string) *serviceerror.ServiceError {
			return nil
		},
	}

	handler := newThemeMgtHandler(mockSvc)
	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /design/themes/{id}", handler.HandleThemeDeleteRequest)

	req := httptest.NewRequest(http.MethodDelete, "/design/themes/theme-123", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusNoContent, w.Code)
}

// Test HandleThemeDeleteRequest - Not found (idempotent delete returns 204)
func (suite *ThemeHandlerTestSuite) TestHandleThemeDeleteRequest_NotFound() {
	mockSvc := &mockThemeService{
		deleteThemeFunc: func(id string) *serviceerror.ServiceError {
			return nil
		},
	}

	handler := newThemeMgtHandler(mockSvc)
	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /design/themes/{id}", handler.HandleThemeDeleteRequest)

	req := httptest.NewRequest(http.MethodDelete, "/design/themes/non-existent", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusNoContent, w.Code)
}

// Test HandleThemeDeleteRequest - Conflict (theme in use)
func (suite *ThemeHandlerTestSuite) TestHandleThemeDeleteRequest_Conflict() {
	mockSvc := &mockThemeService{
		deleteThemeFunc: func(id string) *serviceerror.ServiceError {
			return &ErrorThemeInUse
		},
	}

	handler := newThemeMgtHandler(mockSvc)
	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /design/themes/{id}", handler.HandleThemeDeleteRequest)

	req := httptest.NewRequest(http.MethodDelete, "/design/themes/theme-123", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusConflict, w.Code)
}

// Test parsePaginationParams
func (suite *ThemeHandlerTestSuite) TestParsePaginationParams_Defaults() {
	query := url.Values{}
	limit, offset, err := parsePaginationParams(query)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), 0, offset)
	assert.Greater(suite.T(), limit, 0)
}

func (suite *ThemeHandlerTestSuite) TestParsePaginationParams_ValidValues() {
	query := url.Values{}
	query.Set("limit", "20")
	query.Set("offset", "5")
	limit, offset, err := parsePaginationParams(query)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), 20, limit)
	assert.Equal(suite.T(), 5, offset)
}

func (suite *ThemeHandlerTestSuite) TestParsePaginationParams_InvalidLimit() {
	query := url.Values{}
	query.Set("limit", "abc")
	_, _, err := parsePaginationParams(query)

	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), "THM-1010", err.Code)
}

func (suite *ThemeHandlerTestSuite) TestParsePaginationParams_InvalidOffset() {
	query := url.Values{}
	query.Set("offset", "xyz")
	_, _, err := parsePaginationParams(query)

	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), "THM-1011", err.Code)
}

// Test toHTTPLinks
func (suite *ThemeHandlerTestSuite) TestToHTTPLinks() {
	links := []Link{
		{Href: "/design/themes?limit=10&offset=0", Rel: "previous"},
		{Href: "/design/themes?limit=10&offset=20", Rel: "next"},
	}

	result := toHTTPLinks(links)

	assert.Len(suite.T(), result, 2)
	assert.Equal(suite.T(), "/design/themes?limit=10&offset=0", result[0].Href)
	assert.Equal(suite.T(), "previous", result[0].Rel)
}

func (suite *ThemeHandlerTestSuite) TestToHTTPLinks_Empty() {
	result := toHTTPLinks([]Link{})
	assert.Len(suite.T(), result, 0)
}

// Test handleError - status code mapping
func (suite *ThemeHandlerTestSuite) TestHandleError_StatusCodeMapping() {
	tests := []struct {
		name           string
		svcErr         *serviceerror.ServiceError
		expectedStatus int
	}{
		{
			name:           "ThemeNotFound",
			svcErr:         &ErrorThemeNotFound,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "ThemeInUse",
			svcErr:         &ErrorThemeInUse,
			expectedStatus: http.StatusConflict,
		},
		{
			name:           "InvalidThemeID",
			svcErr:         &ErrorInvalidThemeID,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "InvalidThemeData",
			svcErr:         &ErrorInvalidThemeData,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "InternalServerError",
			svcErr:         &serviceerror.InternalServerError,
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name: "UnknownClientError",
			svcErr: &serviceerror.ServiceError{
				Type: serviceerror.ClientErrorType,
				Code: "UNKNOWN",
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		suite.Run(tc.name, func() {
			w := httptest.NewRecorder()
			handleError(w, tc.svcErr)
			assert.Equal(suite.T(), tc.expectedStatus, w.Code)
		})
	}
}
