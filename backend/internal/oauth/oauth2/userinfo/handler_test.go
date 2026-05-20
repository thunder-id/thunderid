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

package userinfo

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
)

type UserInfoHandlerTestSuite struct {
	suite.Suite
	mockService *userInfoServiceInterfaceMock
	handler     *userInfoHandler
}

func TestUserInfoHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(UserInfoHandlerTestSuite))
}

func (s *UserInfoHandlerTestSuite) SetupTest() {
	s.mockService = new(userInfoServiceInterfaceMock)
	s.handler = newUserInfoHandler(s.mockService)
}

// TestHandleUserInfo_MissingAuthorizationHeader tests missing Authorization header.
func (s *UserInfoHandlerTestSuite) TestHandleUserInfo_MissingAuthorizationHeader() {
	req := httptest.NewRequest(http.MethodGet, "/oauth2/userinfo", nil)
	rr := httptest.NewRecorder()

	s.handler.HandleUserInfo(rr, req)

	assert.Equal(s.T(), http.StatusUnauthorized, rr.Code)
	assert.Equal(s.T(), "Bearer", rr.Header().Get("WWW-Authenticate"))
	assert.Empty(s.T(), rr.Body.String())
}

// TestHandleUserInfo_InvalidAuthorizationHeaderFormat tests invalid Authorization header format.
func (s *UserInfoHandlerTestSuite) TestHandleUserInfo_InvalidAuthorizationHeaderFormat() {
	req := httptest.NewRequest(http.MethodGet, "/oauth2/userinfo", nil)
	req.Header.Set("Authorization", "InvalidFormat token123")
	rr := httptest.NewRecorder()

	s.handler.HandleUserInfo(rr, req)

	assert.Equal(s.T(), http.StatusUnauthorized, rr.Code)
	assert.Equal(s.T(), "Bearer", rr.Header().Get("WWW-Authenticate"))
	assert.Empty(s.T(), rr.Body.String())
}

// TestHandleUserInfo_MissingBearerToken tests missing Bearer token.
// RFC 6750 §3.1: Malformed Bearer request should return 400 with invalid_request.
func (s *UserInfoHandlerTestSuite) TestHandleUserInfo_MissingBearerToken() {
	req := httptest.NewRequest(http.MethodGet, "/oauth2/userinfo", nil)
	req.Header.Set("Authorization", "Bearer ")
	rr := httptest.NewRecorder()

	s.handler.HandleUserInfo(rr, req)

	assert.Equal(s.T(), http.StatusBadRequest, rr.Code)
	assert.Contains(s.T(), rr.Body.String(), constants.ErrorInvalidRequest)
	assert.Contains(s.T(), rr.Body.String(), "Invalid or malformed Bearer token")
	wwwAuth := rr.Header().Get("WWW-Authenticate")
	assert.Contains(s.T(), wwwAuth, "Bearer")
	assert.Contains(s.T(), wwwAuth, constants.ErrorInvalidRequest)
}

// TestHandleUserInfo_InvalidToken tests invalid token error
func (s *UserInfoHandlerTestSuite) TestHandleUserInfo_InvalidToken() {
	s.assertServiceErrorResponse("invalid-token", &errorInvalidAccessToken,
		http.StatusUnauthorized, "invalid_token")
}

// TestHandleUserInfo_MissingSubClaim tests missing sub claim error
func (s *UserInfoHandlerTestSuite) TestHandleUserInfo_MissingSubClaim() {
	s.assertServiceErrorResponse("token123", &errorMissingSubClaim,
		http.StatusUnauthorized, "invalid_token")
}

// TestHandleUserInfo_ServerError tests server error
func (s *UserInfoHandlerTestSuite) TestHandleUserInfo_ServerError() {
	req := httptest.NewRequest(http.MethodGet, "/oauth2/userinfo", nil)
	req.Header.Set("Authorization", "Bearer token123")
	rr := httptest.NewRecorder()

	expectedError := serviceerror.CustomServiceError(serviceerror.InternalServerError,
		core.I18nMessage{
			Key:          "error.test.fetch_userinfo_attributes_or_groups",
			DefaultValue: "An error occurred while fetching user attributes or groups",
		})
	s.mockService.On("GetUserInfo", mock.Anything, "token123").Return(nil, expectedError)

	s.handler.HandleUserInfo(rr, req)

	assert.Equal(s.T(), http.StatusInternalServerError, rr.Code)
	assert.Contains(s.T(), rr.Body.String(), "server_error")
	assert.Contains(s.T(), rr.Body.String(), serviceerror.InternalServerError.Error.DefaultValue)
	assert.NotContains(s.T(), rr.Body.String(), expectedError.ErrorDescription.DefaultValue)
	// Server errors should not include WWW-Authenticate
	assert.Empty(s.T(), rr.Header().Get("WWW-Authenticate"))
	s.mockService.AssertExpectations(s.T())
}

// TestHandleUserInfo_InsufficientScope tests insufficient scope error returns 403 with WWW-Authenticate
func (s *UserInfoHandlerTestSuite) TestHandleUserInfo_InsufficientScope() {
	s.assertServiceErrorResponse("token123", &errorInsufficientScope,
		http.StatusForbidden, "insufficient_scope")
}

// TestHandleUserInfo_Success tests successful response
func (s *UserInfoHandlerTestSuite) TestHandleUserInfo_Success() {
	req := httptest.NewRequest(http.MethodGet, "/oauth2/userinfo", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rr := httptest.NewRecorder()

	userInfo := map[string]interface{}{
		"sub":   "user123",
		"name":  "John Doe",
		"email": "john@example.com",
	}

	s.mockService.On("GetUserInfo", mock.Anything, "valid-token").Return(jsonResponse(userInfo), nil)

	s.handler.HandleUserInfo(rr, req)

	assert.Equal(s.T(), http.StatusOK, rr.Code)
	assert.Equal(s.T(), "application/json", rr.Header().Get("Content-Type"))
	assert.Equal(s.T(), "no-store", rr.Header().Get("Cache-Control"))
	assert.Equal(s.T(), "no-cache", rr.Header().Get("Pragma"))
	assert.Contains(s.T(), rr.Body.String(), `"sub":"user123"`)
	assert.Contains(s.T(), rr.Body.String(), `"name":"John Doe"`)
	assert.Contains(s.T(), rr.Body.String(), `"email":"john@example.com"`)
	s.mockService.AssertExpectations(s.T())
}

// TestHandleUserInfo_Success_POST tests successful POST request
func (s *UserInfoHandlerTestSuite) TestHandleUserInfo_Success_POST() {
	req := httptest.NewRequest(http.MethodPost, "/oauth2/userinfo", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rr := httptest.NewRecorder()

	userInfo := map[string]interface{}{
		"sub": "user123",
	}

	s.mockService.On("GetUserInfo", mock.Anything, "valid-token").Return(jsonResponse(userInfo), nil)

	s.handler.HandleUserInfo(rr, req)

	assert.Equal(s.T(), http.StatusOK, rr.Code)
	assert.Contains(s.T(), rr.Body.String(), `"sub":"user123"`)
	s.mockService.AssertExpectations(s.T())
}

// TestHandleUserInfo_Success_WithGroups tests successful response with groups
func (s *UserInfoHandlerTestSuite) TestHandleUserInfo_Success_WithGroups() {
	req := httptest.NewRequest(http.MethodGet, "/oauth2/userinfo", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rr := httptest.NewRecorder()

	userInfo := map[string]interface{}{
		"sub":    "user123",
		"name":   "John Doe",
		"groups": []interface{}{"admin", "users"},
	}

	s.mockService.On("GetUserInfo", mock.Anything, "valid-token").Return(jsonResponse(userInfo), nil)

	s.handler.HandleUserInfo(rr, req)

	assert.Equal(s.T(), http.StatusOK, rr.Code)
	assert.Contains(s.T(), rr.Body.String(), `"sub":"user123"`)
	assert.Contains(s.T(), rr.Body.String(), `"name":"John Doe"`)
	assert.Contains(s.T(), rr.Body.String(), `"groups"`)
	s.mockService.AssertExpectations(s.T())
}

// TestHandleUserInfo_CaseInsensitiveBearer tests case-insensitive Bearer token
func (s *UserInfoHandlerTestSuite) TestHandleUserInfo_CaseInsensitiveBearer() {
	req := httptest.NewRequest(http.MethodGet, "/oauth2/userinfo", nil)
	req.Header.Set("Authorization", "bearer valid-token")
	rr := httptest.NewRecorder()

	userInfo := map[string]interface{}{
		"sub": "user123",
	}

	s.mockService.On("GetUserInfo", mock.Anything, "valid-token").Return(jsonResponse(userInfo), nil)

	s.handler.HandleUserInfo(rr, req)

	assert.Equal(s.T(), http.StatusOK, rr.Code)
	s.mockService.AssertExpectations(s.T())
}

// TestHandleUserInfo_BEARERUpperCase tests BEARER in uppercase
func (s *UserInfoHandlerTestSuite) TestHandleUserInfo_BEARERUpperCase() {
	req := httptest.NewRequest(http.MethodGet, "/oauth2/userinfo", nil)
	req.Header.Set("Authorization", "BEARER valid-token")
	rr := httptest.NewRecorder()

	userInfo := map[string]interface{}{
		"sub": "user123",
	}

	s.mockService.On("GetUserInfo", mock.Anything, "valid-token").Return(jsonResponse(userInfo), nil)

	s.handler.HandleUserInfo(rr, req)

	assert.Equal(s.T(), http.StatusOK, rr.Code)
	s.mockService.AssertExpectations(s.T())
}

// TestHandleUserInfo_EmptyResponse tests empty response
func (s *UserInfoHandlerTestSuite) TestHandleUserInfo_EmptyResponse() {
	req := httptest.NewRequest(http.MethodGet, "/oauth2/userinfo", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rr := httptest.NewRecorder()

	userInfo := map[string]interface{}{
		"sub": "user123",
	}

	s.mockService.On("GetUserInfo", mock.Anything, "valid-token").Return(jsonResponse(userInfo), nil)

	s.handler.HandleUserInfo(rr, req)

	assert.Equal(s.T(), http.StatusOK, rr.Code)
	assert.Contains(s.T(), rr.Body.String(), `"sub":"user123"`)
	s.mockService.AssertExpectations(s.T())
}

// TestHandleUserInfo_InvalidAuthorizationHeaderSinglePart tests "Bearer" without a token.
// RFC 6750 §3.1: Malformed Bearer request should return 400 with invalid_request.
func (s *UserInfoHandlerTestSuite) TestHandleUserInfo_InvalidAuthorizationHeaderSinglePart() {
	req := httptest.NewRequest(http.MethodGet, "/oauth2/userinfo", nil)
	req.Header.Set("Authorization", "Bearer")
	rr := httptest.NewRecorder()

	s.handler.HandleUserInfo(rr, req)

	assert.Equal(s.T(), http.StatusBadRequest, rr.Code)
	assert.Contains(s.T(), rr.Body.String(), constants.ErrorInvalidRequest)
	assert.Contains(s.T(), rr.Body.String(), "Invalid or malformed Bearer token")
}

// TestHandleUserInfo_EncodingError tests encoding error handling
func (s *UserInfoHandlerTestSuite) TestHandleUserInfo_EncodingError() {
	req := httptest.NewRequest(http.MethodGet, "/oauth2/userinfo", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rr := httptest.NewRecorder()

	// Use a function which cannot be JSON encoded - this will cause Encode to return an error
	userInfo := map[string]interface{}{
		"sub":  "user123",
		"name": "John Doe",
		"func": func() {}, // Function cannot be JSON encoded and will cause an error
	}

	s.mockService.On("GetUserInfo", mock.Anything, "valid-token").Return(jsonResponse(userInfo), nil)

	s.handler.HandleUserInfo(rr, req)

	// With buffer approach, encoding fails BEFORE headers are sent, so we get HTTP 500
	assert.Equal(s.T(), http.StatusInternalServerError, rr.Code)
	// Verify that encoding error message is returned
	assert.Contains(s.T(), rr.Body.String(), serviceerror.ErrorEncodingError.Code)
	s.mockService.AssertExpectations(s.T())
}

// TestWriteServiceErrorResponse_DefaultCase tests the default case in writeServiceErrorResponse
func (s *UserInfoHandlerTestSuite) TestWriteServiceErrorResponse_DefaultCase() {
	req := httptest.NewRequest(http.MethodGet, "/oauth2/userinfo", nil)
	req.Header.Set("Authorization", "Bearer token123")
	rr := httptest.NewRecorder()

	// Create a service error with an unknown type (not ClientErrorType or ServerErrorType)
	unknownError := &serviceerror.ServiceError{
		Type: "UnknownErrorType", // Unknown type
		Code: "unknown_error",
		ErrorDescription: core.I18nMessage{
			Key: "error.test.an_unknown_error_occurred", DefaultValue: "An unknown error occurred",
		},
	}
	s.mockService.On("GetUserInfo", mock.Anything, "token123").Return(nil, unknownError)

	s.handler.HandleUserInfo(rr, req)

	// Default case should return StatusUnauthorized
	assert.Equal(s.T(), http.StatusUnauthorized, rr.Code)
	assert.Contains(s.T(), rr.Body.String(), "unknown_error")
	assert.Contains(s.T(), rr.Body.String(), "An unknown error occurred")
	// RFC 6750 §3: WWW-Authenticate header must be present on 401 responses
	wwwAuth := rr.Header().Get("WWW-Authenticate")
	assert.Contains(s.T(), wwwAuth, "Bearer")
	assert.Contains(s.T(), wwwAuth, "unknown_error")
	s.mockService.AssertExpectations(s.T())
}

// assertServiceErrorResponse is a helper to test service error responses with WWW-Authenticate headers.
func (s *UserInfoHandlerTestSuite) assertServiceErrorResponse(
	token string, svcErr *serviceerror.ServiceError, expectedStatus int, expectedWWWAuthError string,
) {
	req := httptest.NewRequest(http.MethodGet, "/oauth2/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	s.mockService.On("GetUserInfo", mock.Anything, token).Return(nil, svcErr)

	s.handler.HandleUserInfo(rr, req)

	assert.Equal(s.T(), expectedStatus, rr.Code)
	assert.Contains(s.T(), rr.Body.String(), svcErr.Code)
	assert.Contains(s.T(), rr.Body.String(), svcErr.ErrorDescription.DefaultValue)
	// RFC 6750 §3: WWW-Authenticate header must be present on error responses
	wwwAuth := rr.Header().Get("WWW-Authenticate")
	assert.Contains(s.T(), wwwAuth, "Bearer")
	assert.Contains(s.T(), wwwAuth, expectedWWWAuthError)
	s.mockService.AssertExpectations(s.T())
}

func jsonResponse(body map[string]interface{}) *UserInfoResponse {
	return &UserInfoResponse{
		JSONBody: body,
	}
}
