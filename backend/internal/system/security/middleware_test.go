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

package security

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thunder-id/thunderid/internal/system/error/apierror"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// MiddlewareTestSuite defines the test suite for middleware functionality
type MiddlewareTestSuite struct {
	suite.Suite
	mockService *SecurityServiceInterfaceMock
	middleware  func(http.Handler) http.Handler
	testHandler http.Handler
	testCtx     context.Context
}

func (suite *MiddlewareTestSuite) SetupTest() {
	suite.mockService = NewSecurityServiceInterfaceMock(suite.T())
	suite.middleware, _ = middleware(suite.mockService)

	// Create a test handler that captures the received context and request
	suite.testHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Store the context for verification
		suite.testCtx = r.Context()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	})

	// Reset test context
	suite.testCtx = nil
}

// Test successful authentication flow
func (suite *MiddlewareTestSuite) TestMiddleware_SuccessfulAuthentication() {
	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	w := httptest.NewRecorder()

	// Create enriched context with authentication information
	ctx := newSecurityContext(
		"user123",
		"ou456",
		"test_token",
		nil,
		map[string]interface{}{
			"scope": []string{"read", "write"},
			"role":  "admin",
		},
	)
	enrichedCtx := withSecurityContext(context.Background(), ctx)

	// Mock successful authentication
	suite.mockService.EXPECT().Process(req).Return(enrichedCtx, nil)

	handler := suite.middleware(suite.testHandler)
	handler.ServeHTTP(w, req)

	// Verify response
	assert.Equal(suite.T(), http.StatusOK, w.Code)
	assert.Equal(suite.T(), "success", w.Body.String())

	// Verify enriched context was passed to next handler
	assert.NotNil(suite.T(), suite.testCtx)
	assert.Equal(suite.T(), "user123", GetSubject(suite.testCtx))
	assert.Equal(suite.T(), "ou456", GetOUID(suite.testCtx))
}

// Test authentication failure with unauthorized error
func (suite *MiddlewareTestSuite) TestMiddleware_AuthenticationFailure_Unauthorized() {
	req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)
	w := httptest.NewRecorder()

	// Mock authentication failure
	suite.mockService.EXPECT().Process(req).Return(context.Background(), errUnauthorized)

	handler := suite.middleware(suite.testHandler)
	handler.ServeHTTP(w, req)

	// Verify error response
	suite.assertUnauthorizedResponse(w)

	// Verify next handler was not called
	assert.Nil(suite.T(), suite.testCtx)
}

// Test authentication failure with invalid token error
func (suite *MiddlewareTestSuite) TestMiddleware_AuthenticationFailure_InvalidToken() {
	req := httptest.NewRequest(http.MethodPost, "/api/groups", nil)
	w := httptest.NewRecorder()

	suite.mockService.EXPECT().Process(req).Return(context.Background(), errInvalidToken)

	handler := suite.middleware(suite.testHandler)
	handler.ServeHTTP(w, req)

	suite.assertUnauthorizedResponse(w)
	assert.Nil(suite.T(), suite.testCtx)
}

// Test authentication failure with missing auth header error
func (suite *MiddlewareTestSuite) TestMiddleware_AuthenticationFailure_MissingAuthHeader() {
	req := httptest.NewRequest(http.MethodPut, "/api/roles", nil)
	w := httptest.NewRecorder()

	suite.mockService.EXPECT().Process(req).Return(context.Background(), errMissingAuthHeader)

	handler := suite.middleware(suite.testHandler)
	handler.ServeHTTP(w, req)

	suite.assertUnauthorizedResponse(w)
	assert.Nil(suite.T(), suite.testCtx)
}

// Test authentication failure with no handler found error
func (suite *MiddlewareTestSuite) TestMiddleware_AuthenticationFailure_NoHandlerFound() {
	req := httptest.NewRequest(http.MethodDelete, "/api/applications", nil)
	w := httptest.NewRecorder()

	suite.mockService.EXPECT().Process(req).Return(context.Background(), errNoHandlerFound)

	handler := suite.middleware(suite.testHandler)
	handler.ServeHTTP(w, req)

	suite.assertUnauthorizedResponse(w)
	assert.Nil(suite.T(), suite.testCtx)
}

// Test authorization failure with forbidden error
func (suite *MiddlewareTestSuite) TestMiddleware_AuthorizationFailure_Forbidden() {
	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	w := httptest.NewRecorder()

	suite.mockService.EXPECT().Process(req).Return(context.Background(), errForbidden)

	handler := suite.middleware(suite.testHandler)
	handler.ServeHTTP(w, req)

	suite.assertForbiddenResponse(w)
	assert.Nil(suite.T(), suite.testCtx)
}

// Test authorization failure with insufficient permissions error
func (suite *MiddlewareTestSuite) TestMiddleware_AuthorizationFailure_InsufficientPermissions() {
	req := httptest.NewRequest(http.MethodPost, "/admin/settings", nil)
	w := httptest.NewRecorder()

	suite.mockService.EXPECT().Process(req).Return(context.Background(), errInsufficientPermissions)

	handler := suite.middleware(suite.testHandler)
	handler.ServeHTTP(w, req)

	suite.assertForbiddenResponse(w)
	assert.Nil(suite.T(), suite.testCtx)
}

// Test unknown error (default case)
func (suite *MiddlewareTestSuite) TestMiddleware_UnknownError() {
	req := httptest.NewRequest(http.MethodGet, "/api/unknown", nil)
	w := httptest.NewRecorder()

	unknownErr := errors.New("some unexpected error")
	suite.mockService.EXPECT().Process(req).Return(context.Background(), unknownErr)

	handler := suite.middleware(suite.testHandler)
	handler.ServeHTTP(w, req)

	// Unknown errors should be treated as unauthorized with specific message
	assert.Equal(suite.T(), http.StatusUnauthorized, w.Code)
	assert.Equal(suite.T(), "application/json", w.Header().Get("Content-Type"))
	assert.Equal(suite.T(), "Bearer", w.Header().Get("WWW-Authenticate"))

	var response apierror.ErrorResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), apierror.ErrUnauthorized.Code, response.Code)
	assert.Equal(suite.T(), apierror.ErrUnauthorized.Description.DefaultValue, response.Description.DefaultValue)

	assert.Nil(suite.T(), suite.testCtx)
}

// Test context propagation with nil context from service
func (suite *MiddlewareTestSuite) TestMiddleware_NilContextFromService() {
	req := httptest.NewRequest(http.MethodGet, "/public/health", nil)
	w := httptest.NewRecorder()

	// Service returns request context (e.g., for public paths)
	suite.mockService.EXPECT().Process(req).Return(req.Context(), nil)

	handler := suite.middleware(suite.testHandler)
	handler.ServeHTTP(w, req)

	// Should succeed even with nil context
	assert.Equal(suite.T(), http.StatusOK, w.Code)
	assert.Equal(suite.T(), "success", w.Body.String())

	// Should receive the original request context
	assert.NotNil(suite.T(), suite.testCtx)
}

// Test with different HTTP methods
func (suite *MiddlewareTestSuite) TestMiddleware_DifferentHTTPMethods() {
	methods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
		http.MethodOptions,
		http.MethodHead,
	}

	for _, method := range methods {
		suite.Run("Method_"+method, func() {
			req := httptest.NewRequest(method, "/api/test", nil)
			w := httptest.NewRecorder()

			// Reset test context for each iteration
			suite.testCtx = nil

			ctx := newSecurityContext("user", "ou", "token", nil, nil)
			enrichedCtx := withSecurityContext(context.Background(), ctx)

			suite.mockService.EXPECT().Process(req).Return(enrichedCtx, nil)

			handler := suite.middleware(suite.testHandler)
			handler.ServeHTTP(w, req)

			assert.Equal(suite.T(), http.StatusOK, w.Code)
			assert.NotNil(suite.T(), suite.testCtx)
		})
	}
}

// Test middleware chaining
func (suite *MiddlewareTestSuite) TestMiddleware_Chaining() {
	req := httptest.NewRequest(http.MethodGet, "/api/chained", nil)
	w := httptest.NewRecorder()

	// Create a chain of middleware
	var executionOrder []string

	firstMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			executionOrder = append(executionOrder, "first")
			next.ServeHTTP(w, r)
		})
	}

	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		executionOrder = append(executionOrder, "final")
		w.WriteHeader(http.StatusOK)
	})

	ctx := newSecurityContext("user", "ou", "token", nil, nil)
	enrichedCtx := withSecurityContext(context.Background(), ctx)
	suite.mockService.EXPECT().Process(req).Return(enrichedCtx, nil)

	// Chain middleware: first -> security -> final
	chain := firstMiddleware(suite.middleware(finalHandler))
	chain.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)
	assert.Equal(suite.T(), []string{"first", "final"}, executionOrder)
}

// Test error handling doesn't panic with nil response writer (edge case)
func (suite *MiddlewareTestSuite) TestMiddleware_ErrorHandling_EdgeCases() {
	req := httptest.NewRequest(http.MethodGet, "/api/edge", nil)
	w := httptest.NewRecorder()

	// Test with service returning context but also error (edge case)
	ctx := newSecurityContext("user", "ou", "token", nil, nil)
	enrichedCtx := withSecurityContext(context.Background(), ctx)
	suite.mockService.EXPECT().Process(req).Return(enrichedCtx, errUnauthorized)

	handler := suite.middleware(suite.testHandler)
	handler.ServeHTTP(w, req)

	// Error should take precedence over context
	suite.assertUnauthorizedResponse(w)
	assert.Nil(suite.T(), suite.testCtx)
}

// Helper method to assert unauthorized response
func (suite *MiddlewareTestSuite) assertUnauthorizedResponse(w *httptest.ResponseRecorder) {
	assert.Equal(suite.T(), http.StatusUnauthorized, w.Code)
	assert.Equal(suite.T(), "application/json", w.Header().Get("Content-Type"))
	assert.Equal(suite.T(), "Bearer", w.Header().Get("WWW-Authenticate"))

	var response apierror.ErrorResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), apierror.ErrUnauthorized.Code, response.Code)
	assert.Equal(suite.T(), apierror.ErrUnauthorized.Description.DefaultValue, response.Description.DefaultValue)
}

// Helper method to assert forbidden response
func (suite *MiddlewareTestSuite) assertForbiddenResponse(w *httptest.ResponseRecorder) {
	assert.Equal(suite.T(), http.StatusForbidden, w.Code)
	assert.Equal(suite.T(), "application/json", w.Header().Get("Content-Type"))
	assert.Equal(suite.T(), "Bearer", w.Header().Get("WWW-Authenticate"))

	var response apierror.ErrorResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), apierror.ErrForbidden.Code, response.Code)
	assert.Equal(suite.T(), apierror.ErrForbidden.Description.DefaultValue, response.Description.DefaultValue)
}

// Test writeSecurityError function directly
func TestWriteSecurityError(t *testing.T) {
	testCases := []struct {
		name               string
		err                error
		expectedStatus     int
		expectedErrResp    apierror.ErrorResponse
		expectedAuthHeader bool
	}{
		{
			name:               "Unauthorized error",
			err:                errUnauthorized,
			expectedStatus:     http.StatusUnauthorized,
			expectedErrResp:    apierror.ErrUnauthorized,
			expectedAuthHeader: true,
		},
		{
			name:               "Invalid token error",
			err:                errInvalidToken,
			expectedStatus:     http.StatusUnauthorized,
			expectedErrResp:    apierror.ErrUnauthorized,
			expectedAuthHeader: true,
		},
		{
			name:               "Missing auth header error",
			err:                errMissingAuthHeader,
			expectedStatus:     http.StatusUnauthorized,
			expectedErrResp:    apierror.ErrUnauthorized,
			expectedAuthHeader: true,
		},
		{
			name:               "No handler found error",
			err:                errNoHandlerFound,
			expectedStatus:     http.StatusUnauthorized,
			expectedErrResp:    apierror.ErrUnauthorized,
			expectedAuthHeader: true,
		},
		{
			name:               "Forbidden error",
			err:                errForbidden,
			expectedStatus:     http.StatusForbidden,
			expectedErrResp:    apierror.ErrForbidden,
			expectedAuthHeader: true,
		},
		{
			name:               "Insufficient permissions error",
			err:                errInsufficientPermissions,
			expectedStatus:     http.StatusForbidden,
			expectedErrResp:    apierror.ErrForbidden,
			expectedAuthHeader: true,
		},
		{
			name:               "Unknown error (default case)",
			err:                errors.New("unexpected error"),
			expectedStatus:     http.StatusUnauthorized,
			expectedErrResp:    apierror.ErrUnauthorized,
			expectedAuthHeader: true,
		},
		{
			name:               "Nil error (edge case)",
			err:                nil,
			expectedStatus:     http.StatusUnauthorized,
			expectedErrResp:    apierror.ErrUnauthorized,
			expectedAuthHeader: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			writeSecurityError(w, tc.err)

			assert.Equal(t, tc.expectedStatus, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			if tc.expectedAuthHeader {
				assert.Equal(t, "Bearer", w.Header().Get("WWW-Authenticate"))
			} else {
				assert.Empty(t, w.Header().Get("WWW-Authenticate"))
			}

			var response apierror.ErrorResponse
			err := json.NewDecoder(w.Body).Decode(&response)
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedErrResp.Code, response.Code)
			assert.Equal(t, tc.expectedErrResp.Message.DefaultValue, response.Message.DefaultValue)
			assert.Equal(t, tc.expectedErrResp.Description.DefaultValue, response.Description.DefaultValue)
		})
	}
}

// Test middleware creation with nil service (edge case)
func TestMiddleware_NilService(t *testing.T) {
	// This should return an error
	handler, err := middleware(nil)
	assert.Error(t, err)
	assert.Nil(t, handler)
}

// Run the test suite
func TestMiddlewareTestSuite(t *testing.T) {
	suite.Run(t, new(MiddlewareTestSuite))
}
