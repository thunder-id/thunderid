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

package user

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/security"
)

const (
	testUserID789 = "user-789"
	testUserID123 = "user-123"
)

func TestHandleSelfUserGetRequest_Success(t *testing.T) {
	userID := testUserID123
	authCtx := security.NewSecurityContextForTest(userID, "", "", nil, nil)

	mockSvc := NewUserServiceInterfaceMock(t)
	expectedUser := &User{
		ID:         userID,
		Attributes: json.RawMessage(`{"username":"alice"}`),
	}
	mockSvc.On("GetUser", mock.Anything, userID, false).Return(expectedUser, nil)

	handler := newUserHandler(mockSvc)
	req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
	req = req.WithContext(security.WithSecurityContextTest(req.Context(), authCtx))
	rr := httptest.NewRecorder()

	handler.HandleSelfUserGetRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Contains(t, rr.Header().Get("Content-Type"), "application/json")

	var respUser User
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&respUser))
	require.Equal(t, expectedUser.ID, respUser.ID)
	require.JSONEq(t, string(expectedUser.Attributes), string(respUser.Attributes))
}

func TestHandleSelfUserGetRequest_IncludeDisplay(t *testing.T) {
	userID := testUserID123
	authCtx := security.NewSecurityContextForTest(userID, "", "", nil, nil)

	mockSvc := NewUserServiceInterfaceMock(t)
	expectedUser := &User{ID: userID}
	mockSvc.On("GetUser", mock.Anything, userID, true).Return(expectedUser, nil)

	handler := newUserHandler(mockSvc)
	req := httptest.NewRequest(http.MethodGet, "/users/me?include=display", nil)
	req = req.WithContext(security.WithSecurityContextTest(req.Context(), authCtx))
	rr := httptest.NewRecorder()

	handler.HandleSelfUserGetRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	mockSvc.AssertExpectations(t)
}

func TestHandleSelfUserGetRequest_Unauthorized(t *testing.T) {
	mockSvc := NewUserServiceInterfaceMock(t)
	handler := newUserHandler(mockSvc)
	req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
	rr := httptest.NewRecorder()

	handler.HandleSelfUserGetRequest(rr, req)

	require.Equal(t, http.StatusUnauthorized, rr.Code)

	var errResp apierror.ErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, ErrorAuthenticationFailed.Code, errResp.Code)
}

func TestHandleSelfUserPutRequest_Success(t *testing.T) {
	userID := "user-456"
	authCtx := security.NewSecurityContextForTest(userID, "", "", nil, nil)
	attributes := json.RawMessage(`{"email":"alice@example.com"}`)

	mockSvc := NewUserServiceInterfaceMock(t)
	updatedUser := &User{
		ID:         userID,
		Type:       "employee",
		Attributes: attributes,
	}
	mockSvc.On("UpdateUserAttributes", mock.Anything, userID, attributes).Return(updatedUser, nil)

	handler := newUserHandler(mockSvc)
	body := bytes.NewBufferString(`{"attributes":{"email":"alice@example.com"}}`)
	req := httptest.NewRequest(http.MethodPut, "/users/me", body)
	req = req.WithContext(security.WithSecurityContextTest(req.Context(), authCtx))
	rr := httptest.NewRecorder()

	handler.HandleSelfUserPutRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var respUser User
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&respUser))
	require.Equal(t, updatedUser.ID, respUser.ID)
	require.JSONEq(t, string(updatedUser.Attributes), string(respUser.Attributes))
}

func TestHandleSelfUserPutRequest_InvalidBody(t *testing.T) {
	userID := "user-456"
	authCtx := security.NewSecurityContextForTest(userID, "", "", nil, nil)

	mockSvc := NewUserServiceInterfaceMock(t)
	handler := newUserHandler(mockSvc)

	req := httptest.NewRequest(http.MethodPut, "/users/me", bytes.NewBufferString(`{"attributes":`))
	req = req.WithContext(security.WithSecurityContextTest(req.Context(), authCtx))
	rr := httptest.NewRecorder()

	handler.HandleSelfUserPutRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)

	var errResp apierror.ErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, ErrorInvalidRequestFormat.Code, errResp.Code)
}

func TestHandleSelfUserCredentialUpdateRequest_Success(t *testing.T) {
	userID := testUserID789
	authCtx := security.NewSecurityContextForTest(userID, "", "", nil, nil)

	mockSvc := NewUserServiceInterfaceMock(t)
	credentialsJSON := json.RawMessage(`{"password":[{"value":"Secret123!"}]}`)
	mockSvc.On("UpdateUserCredentials", mock.Anything, userID, credentialsJSON).Return(nil)

	handler := newUserHandler(mockSvc)
	req := httptest.NewRequest(http.MethodPost, "/users/me/update-credentials",
		bytes.NewBufferString(`{"attributes":{"password":[{"value":"Secret123!"}]}}`))
	req = req.WithContext(security.WithSecurityContextTest(req.Context(), authCtx))
	rr := httptest.NewRecorder()

	handler.HandleSelfUserCredentialUpdateRequest(rr, req)

	require.Equal(t, http.StatusNoContent, rr.Code)
	require.Equal(t, 0, rr.Body.Len())
}

func TestHandleSelfUserCredentialUpdateRequest_StringValue(t *testing.T) {
	userID := testUserID789
	authCtx := security.NewSecurityContextForTest(userID, "", "", nil, nil)

	mockSvc := NewUserServiceInterfaceMock(t)
	credentialsJSON := json.RawMessage(`{"password":"plaintext-password"}`)
	mockSvc.On("UpdateUserCredentials", mock.Anything, userID, credentialsJSON).Return(nil)

	handler := newUserHandler(mockSvc)
	req := httptest.NewRequest(http.MethodPost, "/users/me/update-credentials",
		bytes.NewBufferString(`{"attributes":{"password":"plaintext-password"}}`))
	req = req.WithContext(security.WithSecurityContextTest(req.Context(), authCtx))
	rr := httptest.NewRecorder()

	handler.HandleSelfUserCredentialUpdateRequest(rr, req)

	require.Equal(t, http.StatusNoContent, rr.Code)
	require.Equal(t, 0, rr.Body.Len())
}

func TestHandleSelfUserCredentialUpdateRequest_MissingCredentials(t *testing.T) {
	userID := testUserID789
	authCtx := security.NewSecurityContextForTest(userID, "", "", nil, nil)

	mockSvc := NewUserServiceInterfaceMock(t)
	handler := newUserHandler(mockSvc)

	req := httptest.NewRequest(http.MethodPost, "/users/me/update-credentials",
		bytes.NewBufferString(`{"attributes":{}}`))
	req = req.WithContext(security.WithSecurityContextTest(req.Context(), authCtx))
	rr := httptest.NewRecorder()

	handler.HandleSelfUserCredentialUpdateRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)

	var errResp apierror.ErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, ErrorMissingCredentials.Code, errResp.Code)
}

func TestHandleSelfUserCredentialUpdateRequest_ErrorCases(t *testing.T) {
	userID := testUserID789
	authCtx := security.NewSecurityContextForTest(userID, "", "", nil, nil)

	testCases := []struct {
		name             string
		requestBody      string
		mockJSON         json.RawMessage
		mockError        *serviceerror.ServiceError
		expectedHTTPCode int
		expectedErrCode  string
	}{
		{
			name:             "Invalid JSON in attributes",
			requestBody:      `{"attributes":["invalid","array"]}`,
			mockJSON:         json.RawMessage(`["invalid","array"]`),
			mockError:        &ErrorInvalidRequestFormat,
			expectedHTTPCode: http.StatusBadRequest,
			expectedErrCode:  ErrorInvalidRequestFormat.Code,
		},
		{
			name:             "Invalid credential type",
			requestBody:      `{"attributes":{"unsupported_type":"some_value"}}`,
			mockJSON:         json.RawMessage(`{"unsupported_type":"some_value"}`),
			mockError:        &ErrorInvalidCredential,
			expectedHTTPCode: http.StatusBadRequest,
			expectedErrCode:  ErrorInvalidCredential.Code,
		},
		{
			name:             "Service error",
			requestBody:      `{"attributes":{"password":"test_password"}}`,
			mockJSON:         json.RawMessage(`{"password":"test_password"}`),
			mockError:        &ErrorInvalidCredential,
			expectedHTTPCode: http.StatusBadRequest,
			expectedErrCode:  ErrorInvalidCredential.Code,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockSvc := NewUserServiceInterfaceMock(t)
			mockSvc.On("UpdateUserCredentials", mock.Anything, userID, tc.mockJSON).Return(tc.mockError)

			handler := newUserHandler(mockSvc)
			req := httptest.NewRequest(http.MethodPost, "/users/me/update-credentials",
				bytes.NewBufferString(tc.requestBody))
			req = req.WithContext(security.WithSecurityContextTest(req.Context(), authCtx))
			rr := httptest.NewRecorder()

			handler.HandleSelfUserCredentialUpdateRequest(rr, req)

			require.Equal(t, tc.expectedHTTPCode, rr.Code)

			var errResp apierror.ErrorResponse
			require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
			require.Equal(t, tc.expectedErrCode, errResp.Code)
		})
	}
}

func TestHandleSelfUserCredentialUpdateRequest_MultipleCredentialTypes(t *testing.T) {
	userID := testUserID789
	authCtx := security.NewSecurityContextForTest(userID, "", "", nil, nil)

	mockSvc := NewUserServiceInterfaceMock(t)
	// Test that multiple credential types are updated in a single atomic call
	credentialsJSON := json.RawMessage(`{"password":"new-password","pin":"1234"}`)
	mockSvc.On("UpdateUserCredentials", mock.Anything, userID, credentialsJSON).Return(nil)

	handler := newUserHandler(mockSvc)
	req := httptest.NewRequest(http.MethodPost, "/users/me/update-credentials",
		bytes.NewBufferString(`{"attributes":{"password":"new-password","pin":"1234"}}`))
	req = req.WithContext(security.WithSecurityContextTest(req.Context(), authCtx))
	rr := httptest.NewRecorder()

	handler.HandleSelfUserCredentialUpdateRequest(rr, req)

	require.Equal(t, http.StatusNoContent, rr.Code)
	require.Equal(t, 0, rr.Body.Len())
	// Verify that UpdateUserCredentials was called exactly once with all credentials
	mockSvc.AssertNumberOfCalls(t, "UpdateUserCredentials", 1)
}

func TestHandleUserListRequest_Success(t *testing.T) {
	mockSvc := NewUserServiceInterfaceMock(t)
	expectedResp := &UserListResponse{
		TotalResults: 10,
		Users:        []User{{ID: "user-1"}},
	}
	mockSvc.On("GetUserList", mock.Anything, 10, 0, mock.Anything, false).Return(expectedResp, nil)

	handler := newUserHandler(mockSvc)
	req := httptest.NewRequest(http.MethodGet, "/users?limit=10&offset=0", nil)
	rr := httptest.NewRecorder()

	handler.HandleUserListRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp UserListResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	require.Equal(t, expectedResp.TotalResults, resp.TotalResults)
}

func TestHandleUserListRequest_WithIncludeDisplay(t *testing.T) {
	mockSvc := NewUserServiceInterfaceMock(t)
	expectedResp := &UserListResponse{
		TotalResults: 1,
		Users:        []User{{ID: "user-1", Display: "Alice"}},
	}
	mockSvc.On("GetUserList", mock.Anything, 10, 0, mock.Anything, true).Return(expectedResp, nil)

	handler := newUserHandler(mockSvc)
	req := httptest.NewRequest(http.MethodGet, "/users?limit=10&offset=0&include=display", nil)
	rr := httptest.NewRecorder()

	handler.HandleUserListRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp UserListResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	require.Equal(t, "Alice", resp.Users[0].Display)
}

func TestHandleUserListRequest_WithInvalidIncludeParam(t *testing.T) {
	mockSvc := NewUserServiceInterfaceMock(t)
	expectedResp := &UserListResponse{
		TotalResults: 1,
		Users:        []User{{ID: "user-1"}},
	}
	// Invalid include value should be treated as no include (includeDisplay=false).
	mockSvc.On("GetUserList", mock.Anything, 10, 0, mock.Anything, false).Return(expectedResp, nil)

	handler := newUserHandler(mockSvc)
	req := httptest.NewRequest(http.MethodGet, "/users?limit=10&offset=0&include=invalid", nil)
	rr := httptest.NewRecorder()

	handler.HandleUserListRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp UserListResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	require.Empty(t, resp.Users[0].Display)
}

func TestHandleUserPostRequest_Success(t *testing.T) {
	mockSvc := NewUserServiceInterfaceMock(t)
	userReq := &User{Type: "employee", Attributes: json.RawMessage(`{"username":"bob"}`)}
	createdUser := &User{ID: "user-bob", Type: "employee", Attributes: json.RawMessage(`{"username":"bob"}`)}
	mockSvc.On("CreateUser", mock.Anything, mock.Anything).Return(createdUser, nil)

	handler := newUserHandler(mockSvc)
	body, _ := json.Marshal(userReq)
	req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	handler.HandleUserPostRequest(rr, req)

	require.Equal(t, http.StatusCreated, rr.Code)
	var resp User
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	require.Equal(t, createdUser.ID, resp.ID)
}

func TestHandleUserGetRequest_Success(t *testing.T) {
	mockSvc := NewUserServiceInterfaceMock(t)
	userID := testUserID123
	expectedUser := &User{ID: userID}
	mockSvc.On("GetUser", mock.Anything, userID, false).Return(expectedUser, nil)

	handler := newUserHandler(mockSvc)
	req := httptest.NewRequest(http.MethodGet, "/users/"+userID, nil)
	// Set path value for Go 1.22+ standard router
	req.SetPathValue("id", userID)
	rr := httptest.NewRecorder()

	handler.HandleUserGetRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp User
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	require.Equal(t, userID, resp.ID)
}

func TestHandleUserGetRequest_IncludeDisplay(t *testing.T) {
	mockSvc := NewUserServiceInterfaceMock(t)
	userID := testUserID123
	expectedUser := &User{ID: userID}
	mockSvc.On("GetUser", mock.Anything, userID, true).Return(expectedUser, nil)

	handler := newUserHandler(mockSvc)
	req := httptest.NewRequest(http.MethodGet, "/users/"+userID+"?include=display", nil)
	req.SetPathValue("id", userID)
	rr := httptest.NewRecorder()

	handler.HandleUserGetRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	mockSvc.AssertExpectations(t)
}

func TestHandleUserPutRequest_Success(t *testing.T) {
	mockSvc := NewUserServiceInterfaceMock(t)
	userID := testUserID123
	userReq := &User{Attributes: json.RawMessage(`{"name":"Updated"}`)}
	updatedUser := &User{ID: userID, Attributes: json.RawMessage(`{"name":"Updated"}`)}
	mockSvc.On("UpdateUser", mock.Anything, userID, mock.Anything).Return(updatedUser, nil)

	handler := newUserHandler(mockSvc)
	body, _ := json.Marshal(userReq)
	req := httptest.NewRequest(http.MethodPut, "/users/"+userID, bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	handler.HandleUserPutRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp User
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	require.Equal(t, userID, resp.ID)
}

func TestHandleUserDeleteRequest_Success(t *testing.T) {
	mockSvc := NewUserServiceInterfaceMock(t)
	userID := testUserID123
	mockSvc.On("DeleteUser", mock.Anything, userID).Return(nil)

	handler := newUserHandler(mockSvc)
	req := httptest.NewRequest(http.MethodDelete, "/users/"+userID, nil)
	rr := httptest.NewRecorder()

	handler.HandleUserDeleteRequest(rr, req)

	require.Equal(t, http.StatusNoContent, rr.Code)
}

func TestHandleUserListByPathRequest_Success(t *testing.T) {
	mockSvc := NewUserServiceInterfaceMock(t)
	expectedResp := &UserListResponse{
		TotalResults: 5,
		Users:        []User{{ID: "user-path-1"}},
	}
	mockSvc.On("GetUsersByPath", mock.Anything, "root/engineering", 10, 0,
		mock.Anything, false).Return(expectedResp, nil)

	handler := newUserHandler(mockSvc)
	req := httptest.NewRequest(http.MethodGet, "/users/path/root/engineering?limit=10", nil)
	req.SetPathValue("path", "root/engineering")
	rr := httptest.NewRecorder()

	handler.HandleUserListByPathRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
}

func TestHandleUserListByPathRequest_WithIncludeDisplay(t *testing.T) {
	mockSvc := NewUserServiceInterfaceMock(t)
	expectedResp := &UserListResponse{
		TotalResults: 1,
		Users:        []User{{ID: "user-1", Display: "Bob"}},
	}
	mockSvc.On("GetUsersByPath", mock.Anything, "root/engineering", 10, 0,
		mock.Anything, true).Return(expectedResp, nil)

	handler := newUserHandler(mockSvc)
	req := httptest.NewRequest(
		http.MethodGet, "/users/path/root/engineering?limit=10&include=display", nil)
	req.SetPathValue("path", "root/engineering")
	rr := httptest.NewRecorder()

	handler.HandleUserListByPathRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp UserListResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	require.Equal(t, "Bob", resp.Users[0].Display)
}

func TestHandleUserPostByPathRequest_Success(t *testing.T) {
	mockSvc := NewUserServiceInterfaceMock(t)
	createdUser := &User{ID: "user-new", Type: "customer"}
	mockSvc.On("CreateUserByPath", mock.Anything, "root/sales", mock.Anything).Return(createdUser, nil)

	handler := newUserHandler(mockSvc)
	body := bytes.NewBufferString(`{"type":"customer"}`)
	req := httptest.NewRequest(http.MethodPost, "/users/path/root/sales", body)
	req.SetPathValue("path", "root/sales")
	rr := httptest.NewRecorder()

	handler.HandleUserPostByPathRequest(rr, req)

	require.Equal(t, http.StatusCreated, rr.Code)
}

func TestHandleUserGroupsGetRequest_Success(t *testing.T) {
	mockSvc := NewUserServiceInterfaceMock(t)
	userID := testUserID123
	expectedResp := &UserGroupListResponse{
		TotalResults: 2,
		Groups:       []entity.EntityGroup{{ID: "group-1", Name: "Admin"}},
	}
	mockSvc.On("GetUserGroups", mock.Anything, userID, 10, 0).Return(expectedResp, nil)

	handler := newUserHandler(mockSvc)
	req := httptest.NewRequest(http.MethodGet, "/users/"+userID+"/groups?limit=10", nil)
	req.SetPathValue("id", userID)
	rr := httptest.NewRecorder()

	handler.HandleUserGroupsGetRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp UserGroupListResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	require.Equal(t, 2, resp.TotalResults)
}

func TestHandleUserListRequest_InvalidParams(t *testing.T) {
	mockSvc := NewUserServiceInterfaceMock(t)
	handler := newUserHandler(mockSvc)
	req := httptest.NewRequest(http.MethodGet, "/users?limit=abc", nil)
	rr := httptest.NewRecorder()

	handler.HandleUserListRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleUserListRequest_WithFilter(t *testing.T) {
	mockSvc := NewUserServiceInterfaceMock(t)
	expectedResp := &UserListResponse{TotalResults: 1}
	mockSvc.On("GetUserList", mock.Anything, mock.Anything, mock.Anything,
		mock.MatchedBy(func(m map[string]interface{}) bool {
			return m["username"] == "alice"
		}), false).Return(expectedResp, nil)

	handler := newUserHandler(mockSvc)
	req := httptest.NewRequest(http.MethodGet, "/users?filter=username%20eq%20%22alice%22", nil)
	rr := httptest.NewRecorder()

	handler.HandleUserListRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
}

func TestHandleUserListRequest_WithFilter_Unquoted(t *testing.T) {
	mockSvc := NewUserServiceInterfaceMock(t)
	expectedResp := &UserListResponse{TotalResults: 1}
	mockSvc.On("GetUserList", mock.Anything, mock.Anything, mock.Anything,
		mock.MatchedBy(func(m map[string]interface{}) bool {
			return m["age"] == int64(30)
		}), false).Return(expectedResp, nil)

	handler := newUserHandler(mockSvc)
	req := httptest.NewRequest(http.MethodGet, "/users?filter=age%20eq%2030", nil)
	rr := httptest.NewRecorder()

	handler.HandleUserListRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
}

func TestHandleUserListRequest_InvalidFilter(t *testing.T) {
	mockSvc := NewUserServiceInterfaceMock(t)
	handler := newUserHandler(mockSvc)
	req := httptest.NewRequest(http.MethodGet, "/users?filter=username%20invalid%20%22alice%22", nil)
	rr := httptest.NewRecorder()

	handler.HandleUserListRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleUserPostRequest_ErrorCases(t *testing.T) {
	mockSvc := NewUserServiceInterfaceMock(t)
	handler := newUserHandler(mockSvc)

	t.Run("InvalidBody", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader("invalid"))
		rr := httptest.NewRecorder()
		handler.HandleUserPostRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("ServiceError", func(t *testing.T) {
		mockSvc.On("CreateUser", mock.Anything, mock.Anything).Return(nil, &serviceerror.InternalServerError).Once()
		req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"type":"customer"}`))
		rr := httptest.NewRecorder()
		handler.HandleUserPostRequest(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestHandleUserGetRequest_ErrorCases(t *testing.T) {
	mockSvc := NewUserServiceInterfaceMock(t)
	handler := newUserHandler(mockSvc)
	userID := "u1"

	t.Run("MissingID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/users/", nil)
		rr := httptest.NewRecorder()
		handler.HandleUserGetRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("ServiceError", func(t *testing.T) {
		mockSvc.On("GetUser", mock.Anything, userID, false).Return(nil, &ErrorUserNotFound).Once()
		req := httptest.NewRequest(http.MethodGet, "/users/"+userID, nil)
		req.SetPathValue("id", userID)
		rr := httptest.NewRecorder()
		handler.HandleUserGetRequest(rr, req)
		require.Equal(t, http.StatusNotFound, rr.Code)
	})
}

func TestHandleUserPutRequest_ErrorCases(t *testing.T) {
	mockSvc := NewUserServiceInterfaceMock(t)
	handler := newUserHandler(mockSvc)
	userID := "u1"

	t.Run("InvalidBody", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/users/"+userID, strings.NewReader("invalid"))
		req.SetPathValue("id", userID)
		rr := httptest.NewRecorder()
		handler.HandleUserPutRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("ServiceError", func(t *testing.T) {
		svcErr := &serviceerror.InternalServerError
		mockSvc.On("UpdateUser", mock.Anything, userID, mock.Anything).Return(nil, svcErr).Once()
		req := httptest.NewRequest(http.MethodPut, "/users/"+userID, strings.NewReader(`{"attributes":{}}`))
		req.SetPathValue("id", userID)
		rr := httptest.NewRecorder()
		handler.HandleUserPutRequest(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestHandleUserDeleteRequest_ErrorCases(t *testing.T) {
	mockSvc := NewUserServiceInterfaceMock(t)
	handler := newUserHandler(mockSvc)
	userID := "u1"

	t.Run("MissingID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/users/", nil)
		rr := httptest.NewRecorder()
		handler.HandleUserDeleteRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("ServiceError", func(t *testing.T) {
		mockSvc.On("DeleteUser", mock.Anything, userID).Return(&serviceerror.InternalServerError).Once()
		req := httptest.NewRequest(http.MethodDelete, "/users/"+userID, nil)
		req.SetPathValue("id", userID)
		rr := httptest.NewRecorder()
		handler.HandleUserDeleteRequest(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestHandleError_ErrorUnauthorized_Returns403(t *testing.T) {
	tests := []struct {
		name     string
		svcErr   *serviceerror.ServiceError
		wantCode int
	}{
		{
			name:     "UnauthorizedError_ReturnsForbidden",
			svcErr:   &serviceerror.ErrorUnauthorized,
			wantCode: http.StatusForbidden,
		},
		{
			name:     "AuthenticationFailedError_ReturnsUnauthorized",
			svcErr:   &ErrorAuthenticationFailed,
			wantCode: http.StatusUnauthorized,
		},
		{
			name:     "InternalServerError_Returns500",
			svcErr:   &serviceerror.InternalServerError,
			wantCode: http.StatusInternalServerError,
		},
		{
			name:     "UserNotFoundError_Returns404",
			svcErr:   &ErrorUserNotFound,
			wantCode: http.StatusNotFound,
		},
	}

	mockSvc := NewUserServiceInterfaceMock(t)
	handler := newUserHandler(mockSvc)
	userID := "u1"

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockSvc.On("GetUser", mock.Anything, userID, false).Return(nil, tc.svcErr).Once()
			req := httptest.NewRequest(http.MethodGet, "/users/"+userID, nil)
			req.SetPathValue("id", userID)
			rr := httptest.NewRecorder()
			handler.HandleUserGetRequest(rr, req)
			require.Equal(t, tc.wantCode, rr.Code)
		})
	}
}
