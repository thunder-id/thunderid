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

package layoutmgt

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
type LayoutHandlerTestSuite struct {
	suite.Suite
}

func TestLayoutHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(LayoutHandlerTestSuite))
}

// mockLayoutService implements LayoutMgtServiceInterface for handler tests
type mockLayoutService struct {
	getLayoutListFunc func(limit, offset int) (*LayoutList, *serviceerror.ServiceError)
	createLayoutFunc  func(layout CreateLayoutRequest) (*Layout, *serviceerror.ServiceError)
	getLayoutFunc     func(id string) (*Layout, *serviceerror.ServiceError)
	updateLayoutFunc  func(id string, layout UpdateLayoutRequest) (*Layout, *serviceerror.ServiceError)
	deleteLayoutFunc  func(id string) *serviceerror.ServiceError
	isLayoutExistFunc func(id string) (bool, *serviceerror.ServiceError)
}

func (m *mockLayoutService) GetLayoutList(limit, offset int) (*LayoutList, *serviceerror.ServiceError) {
	return m.getLayoutListFunc(limit, offset)
}

func (m *mockLayoutService) CreateLayout(layout CreateLayoutRequest) (*Layout, *serviceerror.ServiceError) {
	return m.createLayoutFunc(layout)
}

func (m *mockLayoutService) GetLayout(id string) (*Layout, *serviceerror.ServiceError) {
	return m.getLayoutFunc(id)
}

func (m *mockLayoutService) UpdateLayout(
	id string, layout UpdateLayoutRequest) (*Layout, *serviceerror.ServiceError) {
	return m.updateLayoutFunc(id, layout)
}

func (m *mockLayoutService) DeleteLayout(id string) *serviceerror.ServiceError {
	return m.deleteLayoutFunc(id)
}

func (m *mockLayoutService) IsLayoutExist(id string) (bool, *serviceerror.ServiceError) {
	return m.isLayoutExistFunc(id)
}

// Test HandleLayoutListRequest - Success
func (suite *LayoutHandlerTestSuite) TestHandleLayoutListRequest_Success() {
	layoutList := &LayoutList{
		TotalResults: 2,
		StartIndex:   1,
		Count:        2,
		Layouts: []Layout{
			{ID: "layout-1", DisplayName: "Layout 1", Description: "Desc 1"},
			{ID: "layout-2", DisplayName: "Layout 2", Description: "Desc 2"},
		},
		Links: []Link{},
	}

	mockSvc := &mockLayoutService{
		getLayoutListFunc: func(limit, offset int) (*LayoutList, *serviceerror.ServiceError) {
			return layoutList, nil
		},
	}

	handler := newLayoutMgtHandler(mockSvc)
	req := httptest.NewRequest(http.MethodGet, "/design/layouts?limit=10&offset=0", nil)
	w := httptest.NewRecorder()

	handler.HandleLayoutListRequest(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)

	var response LayoutListResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), 2, response.TotalResults)
	assert.Len(suite.T(), response.Layouts, 2)
}

// Test HandleLayoutListRequest - Invalid pagination
func (suite *LayoutHandlerTestSuite) TestHandleLayoutListRequest_InvalidLimit() {
	handler := newLayoutMgtHandler(&mockLayoutService{})
	req := httptest.NewRequest(http.MethodGet, "/design/layouts?limit=abc", nil)
	w := httptest.NewRecorder()

	handler.HandleLayoutListRequest(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

// Test HandleLayoutListRequest - Invalid offset
func (suite *LayoutHandlerTestSuite) TestHandleLayoutListRequest_InvalidOffset() {
	handler := newLayoutMgtHandler(&mockLayoutService{})
	req := httptest.NewRequest(http.MethodGet, "/design/layouts?offset=abc", nil)
	w := httptest.NewRecorder()

	handler.HandleLayoutListRequest(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

// Test HandleLayoutListRequest - Service error
func (suite *LayoutHandlerTestSuite) TestHandleLayoutListRequest_ServiceError() {
	mockSvc := &mockLayoutService{
		getLayoutListFunc: func(limit, offset int) (*LayoutList, *serviceerror.ServiceError) {
			return nil, &serviceerror.InternalServerError
		},
	}

	handler := newLayoutMgtHandler(mockSvc)
	req := httptest.NewRequest(http.MethodGet, "/design/layouts", nil)
	w := httptest.NewRecorder()

	handler.HandleLayoutListRequest(w, req)

	assert.Equal(suite.T(), http.StatusInternalServerError, w.Code)
}

// Test HandleLayoutPostRequest - Success
func (suite *LayoutHandlerTestSuite) TestHandleLayoutPostRequest_Success() {
	createdLayout := &Layout{
		ID:          "layout-new",
		DisplayName: "New Layout",
		Description: "A new layout",
		Layout:      json.RawMessage(`{"structure": "grid"}`),
		IsReadOnly:  true,
	}

	mockSvc := &mockLayoutService{
		createLayoutFunc: func(layout CreateLayoutRequest) (*Layout, *serviceerror.ServiceError) {
			return createdLayout, nil
		},
	}

	handler := newLayoutMgtHandler(mockSvc)
	body, _ := json.Marshal(CreateLayoutRequest{
		DisplayName: "New Layout",
		Description: "A new layout",
		Layout:      json.RawMessage(`{"structure": "grid"}`),
	})
	req := httptest.NewRequest(http.MethodPost, "/design/layouts", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleLayoutPostRequest(w, req)

	assert.Equal(suite.T(), http.StatusCreated, w.Code)

	var response Layout
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "layout-new", response.ID)
	assert.True(suite.T(), response.IsReadOnly)
}

// Test HandleLayoutPostRequest - Invalid JSON body
func (suite *LayoutHandlerTestSuite) TestHandleLayoutPostRequest_InvalidJSON() {
	handler := newLayoutMgtHandler(&mockLayoutService{})
	req := httptest.NewRequest(http.MethodPost, "/design/layouts", bytes.NewBufferString("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleLayoutPostRequest(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

// Test HandleLayoutPostRequest - Service error
func (suite *LayoutHandlerTestSuite) TestHandleLayoutPostRequest_ServiceError() {
	mockSvc := &mockLayoutService{
		createLayoutFunc: func(layout CreateLayoutRequest) (*Layout, *serviceerror.ServiceError) {
			return nil, &serviceerror.InternalServerError
		},
	}

	handler := newLayoutMgtHandler(mockSvc)
	body, _ := json.Marshal(CreateLayoutRequest{
		DisplayName: "Layout",
		Layout:      json.RawMessage(`{"structure": "grid"}`),
	})
	req := httptest.NewRequest(http.MethodPost, "/design/layouts", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleLayoutPostRequest(w, req)

	assert.Equal(suite.T(), http.StatusInternalServerError, w.Code)
}

// Test HandleLayoutGetRequest - Success
func (suite *LayoutHandlerTestSuite) TestHandleLayoutGetRequest_Success() {
	layout := &Layout{
		ID:          "layout-123",
		DisplayName: "Test Layout",
		Description: "A test layout",
		Layout:      json.RawMessage(`{"structure": "centered"}`),
		IsReadOnly:  true,
	}

	mockSvc := &mockLayoutService{
		getLayoutFunc: func(id string) (*Layout, *serviceerror.ServiceError) {
			return layout, nil
		},
	}

	handler := newLayoutMgtHandler(mockSvc)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /design/layouts/{id}", handler.HandleLayoutGetRequest)

	req := httptest.NewRequest(http.MethodGet, "/design/layouts/layout-123", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)

	var response Layout
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "layout-123", response.ID)
	assert.True(suite.T(), response.IsReadOnly)
}

// Test HandleLayoutGetRequest - Not found
func (suite *LayoutHandlerTestSuite) TestHandleLayoutGetRequest_NotFound() {
	mockSvc := &mockLayoutService{
		getLayoutFunc: func(id string) (*Layout, *serviceerror.ServiceError) {
			return nil, &ErrorLayoutNotFound
		},
	}

	handler := newLayoutMgtHandler(mockSvc)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /design/layouts/{id}", handler.HandleLayoutGetRequest)

	req := httptest.NewRequest(http.MethodGet, "/design/layouts/non-existent", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusNotFound, w.Code)
}

// Test HandleLayoutPutRequest - Success
func (suite *LayoutHandlerTestSuite) TestHandleLayoutPutRequest_Success() {
	updatedLayout := &Layout{
		ID:          "layout-123",
		DisplayName: "Updated Layout",
		Description: "Updated desc",
		Layout:      json.RawMessage(`{"structure": "flex"}`),
		IsReadOnly:  true,
	}

	mockSvc := &mockLayoutService{
		updateLayoutFunc: func(id string, layout UpdateLayoutRequest) (*Layout, *serviceerror.ServiceError) {
			return updatedLayout, nil
		},
	}

	handler := newLayoutMgtHandler(mockSvc)
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /design/layouts/{id}", handler.HandleLayoutPutRequest)

	body, _ := json.Marshal(UpdateLayoutRequest{
		DisplayName: "Updated Layout",
		Description: "Updated desc",
		Layout:      json.RawMessage(`{"structure": "flex"}`),
	})
	req := httptest.NewRequest(http.MethodPut, "/design/layouts/layout-123", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)

	var response Layout
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), response.IsReadOnly)
}

// Test HandleLayoutPutRequest - Invalid JSON
func (suite *LayoutHandlerTestSuite) TestHandleLayoutPutRequest_InvalidJSON() {
	handler := newLayoutMgtHandler(&mockLayoutService{})
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /design/layouts/{id}", handler.HandleLayoutPutRequest)

	req := httptest.NewRequest(http.MethodPut, "/design/layouts/layout-123", bytes.NewBufferString("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

// Test HandleLayoutDeleteRequest - Success
func (suite *LayoutHandlerTestSuite) TestHandleLayoutDeleteRequest_Success() {
	mockSvc := &mockLayoutService{
		deleteLayoutFunc: func(id string) *serviceerror.ServiceError {
			return nil
		},
	}

	handler := newLayoutMgtHandler(mockSvc)
	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /design/layouts/{id}", handler.HandleLayoutDeleteRequest)

	req := httptest.NewRequest(http.MethodDelete, "/design/layouts/layout-123", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusNoContent, w.Code)
}

// Test HandleLayoutDeleteRequest - Not found (idempotent delete returns 204)
func (suite *LayoutHandlerTestSuite) TestHandleLayoutDeleteRequest_NotFound() {
	mockSvc := &mockLayoutService{
		deleteLayoutFunc: func(id string) *serviceerror.ServiceError {
			return nil
		},
	}

	handler := newLayoutMgtHandler(mockSvc)
	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /design/layouts/{id}", handler.HandleLayoutDeleteRequest)

	req := httptest.NewRequest(http.MethodDelete, "/design/layouts/non-existent", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusNoContent, w.Code)
}

// Test HandleLayoutDeleteRequest - Conflict (layout in use)
func (suite *LayoutHandlerTestSuite) TestHandleLayoutDeleteRequest_Conflict() {
	mockSvc := &mockLayoutService{
		deleteLayoutFunc: func(id string) *serviceerror.ServiceError {
			return &ErrorLayoutAlreadyExists
		},
	}

	handler := newLayoutMgtHandler(mockSvc)
	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /design/layouts/{id}", handler.HandleLayoutDeleteRequest)

	req := httptest.NewRequest(http.MethodDelete, "/design/layouts/layout-123", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusConflict, w.Code)
}

// Test parsePaginationParams
func (suite *LayoutHandlerTestSuite) TestParsePaginationParams_Defaults() {
	query := url.Values{}
	limit, offset, err := parsePaginationParams(query)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), 0, offset)
	assert.Greater(suite.T(), limit, 0) // should use default page size
}

func (suite *LayoutHandlerTestSuite) TestParsePaginationParams_ValidValues() {
	query := url.Values{}
	query.Set("limit", "20")
	query.Set("offset", "5")
	limit, offset, err := parsePaginationParams(query)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), 20, limit)
	assert.Equal(suite.T(), 5, offset)
}

func (suite *LayoutHandlerTestSuite) TestParsePaginationParams_InvalidLimit() {
	query := url.Values{}
	query.Set("limit", "abc")
	_, _, err := parsePaginationParams(query)

	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), "LAY-1011", err.Code)
}

func (suite *LayoutHandlerTestSuite) TestParsePaginationParams_InvalidOffset() {
	query := url.Values{}
	query.Set("offset", "xyz")
	_, _, err := parsePaginationParams(query)

	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), "LAY-1012", err.Code)
}

// Test toHTTPLinks
func (suite *LayoutHandlerTestSuite) TestToHTTPLinks() {
	links := []Link{
		{Href: "/design/layouts?limit=10&offset=0", Rel: "previous"},
		{Href: "/design/layouts?limit=10&offset=20", Rel: "next"},
	}

	result := toHTTPLinks(links)

	assert.Len(suite.T(), result, 2)
	assert.Equal(suite.T(), "/design/layouts?limit=10&offset=0", result[0].Href)
	assert.Equal(suite.T(), "previous", result[0].Rel)
	assert.Equal(suite.T(), "/design/layouts?limit=10&offset=20", result[1].Href)
	assert.Equal(suite.T(), "next", result[1].Rel)
}

func (suite *LayoutHandlerTestSuite) TestToHTTPLinks_Empty() {
	result := toHTTPLinks([]Link{})
	assert.Len(suite.T(), result, 0)
}

// Test handleError - status code mapping
func (suite *LayoutHandlerTestSuite) TestHandleError_StatusCodeMapping() {
	tests := []struct {
		name           string
		svcErr         *serviceerror.ServiceError
		expectedStatus int
	}{
		{
			name:           "LayoutNotFound",
			svcErr:         &ErrorLayoutNotFound,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "LayoutAlreadyExists",
			svcErr:         &ErrorLayoutAlreadyExists,
			expectedStatus: http.StatusConflict,
		},
		{
			name:           "InvalidLayoutID",
			svcErr:         &ErrorInvalidLayoutID,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "InvalidLayoutData",
			svcErr:         &ErrorInvalidLayoutData,
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
