// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resolve

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/design/common"
)

// Mock for DesignResolveServiceInterface
type mockDesignResolveService struct {
	resolveDesignFn func(
		ctx context.Context,
		resolveType providers.DesignResolveType,
		id string,
	) (*providers.DesignResponse, *tidcommon.ServiceError)
}

func (m *mockDesignResolveService) ResolveDesign(
	ctx context.Context, resolveType providers.DesignResolveType, id string,
) (*providers.DesignResponse, *tidcommon.ServiceError) {
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
	designResponse := &providers.DesignResponse{
		Theme:  json.RawMessage(`{"colors": {"primary": "#007bff"}}`),
		Layout: json.RawMessage(`{"structure": "centered"}`),
	}

	mockService := &mockDesignResolveService{
		resolveDesignFn: func(
			ctx context.Context,
			resolveType providers.DesignResolveType,
			id string,
		) (*providers.DesignResponse, *tidcommon.ServiceError) {
			assert.Equal(suite.T(), providers.DesignResolveTypeAPP, resolveType)
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
	designResponse := &providers.DesignResponse{
		Theme: json.RawMessage(`{"colors": {}}`),
	}

	mockService := &mockDesignResolveService{
		resolveDesignFn: func(
			ctx context.Context,
			resolveType providers.DesignResolveType,
			id string,
		) (*providers.DesignResponse, *tidcommon.ServiceError) {
			assert.Equal(suite.T(), providers.DesignResolveTypeAPP, resolveType)
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
			resolveType providers.DesignResolveType,
			id string,
		) (*providers.DesignResponse, *tidcommon.ServiceError) {
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
			resolveType providers.DesignResolveType,
			id string,
		) (*providers.DesignResponse, *tidcommon.ServiceError) {
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
			resolveType providers.DesignResolveType,
			id string,
		) (*providers.DesignResponse, *tidcommon.ServiceError) {
			return nil, &common.ErrorUnsupportedResolveType
		},
	}

	handler := newDesignResolveHandler(mockService)
	req := httptest.NewRequest(http.MethodGet, "/design/resolve?type=OU&id=ou-123", nil)
	w := httptest.NewRecorder()

	handler.HandleResolveRequest(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

// Test HandleResolveRequest - Application has no design
func (suite *ResolveHandlerTestSuite) TestHandleResolveRequest_ApplicationHasNoDesign() {
	mockService := &mockDesignResolveService{
		resolveDesignFn: func(
			ctx context.Context,
			resolveType providers.DesignResolveType,
			id string,
		) (*providers.DesignResponse, *tidcommon.ServiceError) {
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
			resolveType providers.DesignResolveType,
			id string,
		) (*providers.DesignResponse, *tidcommon.ServiceError) {
			return nil, &tidcommon.InternalServerError
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
		svcErr         *tidcommon.ServiceError
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
			name:           "InternalServerError",
			svcErr:         &tidcommon.InternalServerError,
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name: "UnknownClientError",
			svcErr: &tidcommon.ServiceError{
				Type: tidcommon.ClientErrorType,
				Code: "UNKNOWN",
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		suite.Run(tc.name, func() {
			w := httptest.NewRecorder()
			handler.handleError(context.Background(), w, tc.svcErr)
			assert.Equal(suite.T(), tc.expectedStatus, w.Code)
		})
	}
}
