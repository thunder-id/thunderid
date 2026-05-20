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

package resolve

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/design/common"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
)

// Mock for DesignResolveServiceInterface
type mockDesignResolveService struct {
	resolveDesignFn func(
		ctx context.Context,
		resolveType common.DesignResolveType,
		id string,
	) (*common.DesignResponse, *serviceerror.ServiceError)
}

func (m *mockDesignResolveService) ResolveDesign(
	ctx context.Context, resolveType common.DesignResolveType, id string,
) (*common.DesignResponse, *serviceerror.ServiceError) {
	return m.resolveDesignFn(ctx, resolveType, id)
}

// Test Suite
type ResolveHandlerTestSuite struct {
	suite.Suite
}

func TestResolveHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(ResolveHandlerTestSuite))
}

// Test HandleResolveRequest - Success
func (suite *ResolveHandlerTestSuite) TestHandleResolveRequest_Success() {
	designResponse := &common.DesignResponse{
		Theme:  json.RawMessage(`{"colors": {"primary": "#007bff"}}`),
		Layout: json.RawMessage(`{"structure": "centered"}`),
	}

	mockService := &mockDesignResolveService{
		resolveDesignFn: func(
			ctx context.Context,
			resolveType common.DesignResolveType,
			id string,
		) (*common.DesignResponse, *serviceerror.ServiceError) {
			assert.Equal(suite.T(), common.DesignResolveTypeAPP, resolveType)
			assert.Equal(suite.T(), "app-123", id)
			return designResponse, nil
		},
	}

	handler := newDesignResolveHandler(mockService)
	req := httptest.NewRequest(http.MethodGet, "/design/resolve?type=APP&id=app-123", nil)
	w := httptest.NewRecorder()

	handler.HandleResolveRequest(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)
}

// Test HandleResolveRequest - Type is case-insensitive (lowercased input)
func (suite *ResolveHandlerTestSuite) TestHandleResolveRequest_CaseInsensitiveType() {
	designResponse := &common.DesignResponse{
		Theme: json.RawMessage(`{"colors": {}}`),
	}

	mockService := &mockDesignResolveService{
		resolveDesignFn: func(
			ctx context.Context,
			resolveType common.DesignResolveType,
			id string,
		) (*common.DesignResponse, *serviceerror.ServiceError) {
			assert.Equal(suite.T(), common.DesignResolveTypeAPP, resolveType)
			return designResponse, nil
		},
	}

	handler := newDesignResolveHandler(mockService)
	req := httptest.NewRequest(http.MethodGet, "/design/resolve?type=app&id=app-123", nil)
	w := httptest.NewRecorder()

	handler.HandleResolveRequest(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)
}

// Test HandleResolveRequest - Client error (bad request)
func (suite *ResolveHandlerTestSuite) TestHandleResolveRequest_InvalidResolveType() {
	mockService := &mockDesignResolveService{
		resolveDesignFn: func(
			ctx context.Context,
			resolveType common.DesignResolveType,
			id string,
		) (*common.DesignResponse, *serviceerror.ServiceError) {
			return nil, &common.ErrorInvalidResolveType
		},
	}

	handler := newDesignResolveHandler(mockService)
	req := httptest.NewRequest(http.MethodGet, "/design/resolve?id=app-123", nil)
	w := httptest.NewRecorder()

	handler.HandleResolveRequest(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

// Test HandleResolveRequest - Missing ID
func (suite *ResolveHandlerTestSuite) TestHandleResolveRequest_MissingID() {
	mockService := &mockDesignResolveService{
		resolveDesignFn: func(
			ctx context.Context,
			resolveType common.DesignResolveType,
			id string,
		) (*common.DesignResponse, *serviceerror.ServiceError) {
			return nil, &common.ErrorMissingResolveID
		},
	}

	handler := newDesignResolveHandler(mockService)
	req := httptest.NewRequest(http.MethodGet, "/design/resolve?type=APP", nil)
	w := httptest.NewRecorder()

	handler.HandleResolveRequest(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

// Test HandleResolveRequest - Unsupported type
func (suite *ResolveHandlerTestSuite) TestHandleResolveRequest_UnsupportedType() {
	mockService := &mockDesignResolveService{
		resolveDesignFn: func(
			ctx context.Context,
			resolveType common.DesignResolveType,
			id string,
		) (*common.DesignResponse, *serviceerror.ServiceError) {
			return nil, &common.ErrorUnsupportedResolveType
		},
	}

	handler := newDesignResolveHandler(mockService)
	req := httptest.NewRequest(http.MethodGet, "/design/resolve?type=OU&id=ou-123", nil)
	w := httptest.NewRecorder()

	handler.HandleResolveRequest(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

// Test HandleResolveRequest - Application not found
func (suite *ResolveHandlerTestSuite) TestHandleResolveRequest_ApplicationNotFound() {
	mockService := &mockDesignResolveService{
		resolveDesignFn: func(
			ctx context.Context,
			resolveType common.DesignResolveType,
			id string,
		) (*common.DesignResponse, *serviceerror.ServiceError) {
			return nil, &common.ErrorApplicationNotFound
		},
	}

	handler := newDesignResolveHandler(mockService)
	req := httptest.NewRequest(http.MethodGet, "/design/resolve?type=APP&id=non-existent", nil)
	w := httptest.NewRecorder()

	handler.HandleResolveRequest(w, req)

	assert.Equal(suite.T(), http.StatusNotFound, w.Code)
}

// Test HandleResolveRequest - Application has no design
func (suite *ResolveHandlerTestSuite) TestHandleResolveRequest_ApplicationHasNoDesign() {
	mockService := &mockDesignResolveService{
		resolveDesignFn: func(
			ctx context.Context,
			resolveType common.DesignResolveType,
			id string,
		) (*common.DesignResponse, *serviceerror.ServiceError) {
			return nil, &common.ErrorApplicationHasNoDesign
		},
	}

	handler := newDesignResolveHandler(mockService)
	req := httptest.NewRequest(http.MethodGet, "/design/resolve?type=APP&id=app-no-design", nil)
	w := httptest.NewRecorder()

	handler.HandleResolveRequest(w, req)

	assert.Equal(suite.T(), http.StatusNotFound, w.Code)
}

// Test HandleResolveRequest - Internal server error
func (suite *ResolveHandlerTestSuite) TestHandleResolveRequest_InternalServerError() {
	mockService := &mockDesignResolveService{
		resolveDesignFn: func(
			ctx context.Context,
			resolveType common.DesignResolveType,
			id string,
		) (*common.DesignResponse, *serviceerror.ServiceError) {
			return nil, &serviceerror.InternalServerError
		},
	}

	handler := newDesignResolveHandler(mockService)
	req := httptest.NewRequest(http.MethodGet, "/design/resolve?type=APP&id=app-123", nil)
	w := httptest.NewRecorder()

	handler.HandleResolveRequest(w, req)

	assert.Equal(suite.T(), http.StatusInternalServerError, w.Code)
}

// Test handleError - status code mapping
func (suite *ResolveHandlerTestSuite) TestHandleError_StatusCodeMapping() {
	handler := newDesignResolveHandler(nil)

	tests := []struct {
		name           string
		svcErr         *serviceerror.ServiceError
		expectedStatus int
	}{
		{
			name:           "InvalidResolveType",
			svcErr:         &common.ErrorInvalidResolveType,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "MissingResolveID",
			svcErr:         &common.ErrorMissingResolveID,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "UnsupportedResolveType",
			svcErr:         &common.ErrorUnsupportedResolveType,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "ApplicationHasNoDesign",
			svcErr:         &common.ErrorApplicationHasNoDesign,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "ApplicationNotFound",
			svcErr:         &common.ErrorApplicationNotFound,
			expectedStatus: http.StatusNotFound,
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
			handler.handleError(w, tc.svcErr)
			assert.Equal(suite.T(), tc.expectedStatus, w.Code)
		})
	}
}
