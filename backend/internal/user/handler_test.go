// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/internal/system/security"
)

const (
	testUserID789 = "user-789"
	testUserID123 = "user-123"
	testUserID456 = "user-456"
)

func TestHandleSelfUserGetRequest_Success(t *testing.T) {
	userID := testUserID123
	authCtx := security.NewSecurityContextForTest(userID, "", "", nil, nil)

	mockSvc := NewUserServiceInterfaceMock(t)
	expectedUser := &providers.User{
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

	var respUser providers.User
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&respUser))
	require.Equal(t, expectedUser.ID, respUser.ID)
	require.JSONEq(t, string(expectedUser.Attributes), string(respUser.Attributes))
}

func TestHandleSelfUserGetRequest_OmitsOperatorNote(t *testing.T) {
	userID := testUserID123
	authCtx := security.NewSecurityContextForTest(userID, "", "", nil, nil)

	mockSvc := NewUserServiceInterfaceMock(t)
	suspended := &providers.User{ID: userID, Status: &providers.AccessStatusReport{
		Value: providers.AccessStatusSuspended,
		Details: &providers.AccessStatusDetails{
			Reason:       providers.AccessHoldReasonAdministrativeSuspension,
			Since:        "2026-10-09T10:00:00Z",
			OperatorNote: "Under investigation",
		},
	}}
	mockSvc.On("GetUser", mock.Anything, userID, false).Return(suspended, nil)

	handler := newUserHandler(mockSvc)
	req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
	req = req.WithContext(security.WithSecurityContextTest(req.Context(), authCtx))
	rr := httptest.NewRecorder()

	handler.HandleSelfUserGetRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.NotContains(t, rr.Body.String(), "operatorNote")
	var respUser providers.User
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&respUser))
	require.Equal(t, providers.AccessStatusSuspended, respUser.Status.Value)
	require.Equal(t, "2026-10-09T10:00:00Z", respUser.Status.Details.Since)
	require.Equal(t, "Under investigation", suspended.Status.Details.OperatorNote,
		"the service's value is not modified")
}

func TestHandleSelfUserGetRequest_IncludeDisplay(t *testing.T) {
	userID := testUserID123
	authCtx := security.NewSecurityContextForTest(userID, "", "", nil, nil)

	mockSvc := NewUserServiceInterfaceMock(t)
	expectedUser := &providers.User{ID: userID}
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
	userID := testUserID456
	authCtx := security.NewSecurityContextForTest(userID, "", "", nil, nil)
	attributes := json.RawMessage(`{"email":"alice@example.com"}`)

	mockSvc := NewUserServiceInterfaceMock(t)
	updatedUser := &providers.User{
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

	var respUser providers.User
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&respUser))
	require.Equal(t, updatedUser.ID, respUser.ID)
	require.JSONEq(t, string(updatedUser.Attributes), string(respUser.Attributes))
}

func TestHandleSelfUserPutRequest_OmitsOperatorNote(t *testing.T) {
	userID := testUserID456
	authCtx := security.NewSecurityContextForTest(userID, "", "", nil, nil)
	attributes := json.RawMessage(`{"email":"alice@example.com"}`)

	mockSvc := NewUserServiceInterfaceMock(t)
	updatedUser := &providers.User{ID: userID, Attributes: attributes, Status: &providers.AccessStatusReport{
		Value: providers.AccessStatusSuspended,
		Details: &providers.AccessStatusDetails{Reason: providers.AccessHoldReasonAdministrativeSuspension,
			Since: "2026-10-21T08:00:00Z", OperatorNote: "Under investigation"},
	}}
	mockSvc.On("UpdateUserAttributes", mock.Anything, userID, attributes).Return(updatedUser, nil)

	handler := newUserHandler(mockSvc)
	body := bytes.NewBufferString(`{"attributes":{"email":"alice@example.com"}}`)
	req := httptest.NewRequest(http.MethodPut, "/users/me", body)
	req = req.WithContext(security.WithSecurityContextTest(req.Context(), authCtx))
	rr := httptest.NewRecorder()

	handler.HandleSelfUserPutRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.NotContains(t, rr.Body.String(), "operatorNote")
	var response providers.User
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))
	require.Equal(t, &providers.AccessStatusReport{Value: providers.AccessStatusSuspended,
		Details: &providers.AccessStatusDetails{Reason: providers.AccessHoldReasonAdministrativeSuspension,
			Since: "2026-10-21T08:00:00Z"}}, response.Status, "only the note is withheld from the holder")
}

func TestHandleSelfUserPutRequest_InvalidBody(t *testing.T) {
	userID := testUserID456
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
		mockError        *tidcommon.ServiceError
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

func TestHandleUserCredentialUpdateRequest_Success(t *testing.T) {
	userID := testUserID789

	mockSvc := NewUserServiceInterfaceMock(t)
	credentialsJSON := json.RawMessage(`{"password":"new-password"}`)
	mockSvc.On("UpdateUserCredentials", mock.Anything, userID, credentialsJSON).Return(nil)

	handler := newUserHandler(mockSvc)
	req := httptest.NewRequest(http.MethodPost, "/users/"+userID+"/update-credentials",
		bytes.NewBufferString(`{"credentials":{"password":"new-password"}}`))
	req.SetPathValue("id", userID)
	rr := httptest.NewRecorder()

	handler.HandleUserCredentialUpdateRequest(rr, req)

	require.Equal(t, http.StatusNoContent, rr.Code)
	require.Equal(t, 0, rr.Body.Len())
}

func TestHandleUserCredentialUpdateRequest_MissingUserID(t *testing.T) {
	mockSvc := NewUserServiceInterfaceMock(t)
	handler := newUserHandler(mockSvc)

	req := httptest.NewRequest(http.MethodPost, "/users/update-credentials",
		bytes.NewBufferString(`{"credentials":{"password":"new-password"}}`))
	req.SetPathValue("id", "")
	rr := httptest.NewRecorder()

	handler.HandleUserCredentialUpdateRequest(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)

	var errResp apierror.ErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, ErrorMissingUserID.Code, errResp.Code)
}

func TestHandleUserCredentialUpdateRequest_MissingCredentials(t *testing.T) {
	userID := testUserID789

	mockSvc := NewUserServiceInterfaceMock(t)
	handler := newUserHandler(mockSvc)

	req := httptest.NewRequest(http.MethodPost, "/users/"+userID+"/update-credentials",
		bytes.NewBufferString(`{"credentials":{}}`))
	req.SetPathValue("id", userID)
	rr := httptest.NewRecorder()

	handler.HandleUserCredentialUpdateRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)

	var errResp apierror.ErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, ErrorMissingCredentials.Code, errResp.Code)
}

func TestHandleUserCredentialUpdateRequest_InvalidBody(t *testing.T) {
	userID := testUserID789

	mockSvc := NewUserServiceInterfaceMock(t)
	handler := newUserHandler(mockSvc)

	req := httptest.NewRequest(http.MethodPost, "/users/"+userID+"/update-credentials",
		bytes.NewBufferString(`{"credentials":`))
	req.SetPathValue("id", userID)
	rr := httptest.NewRecorder()

	handler.HandleUserCredentialUpdateRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)

	var errResp apierror.ErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, ErrorInvalidRequestFormat.Code, errResp.Code)
}

func TestHandleUserCredentialUpdateRequest_ErrorCases(t *testing.T) {
	userID := testUserID789

	testCases := []struct {
		name             string
		requestBody      string
		mockJSON         json.RawMessage
		mockError        *tidcommon.ServiceError
		expectedHTTPCode int
		expectedErrCode  string
	}{
		{
			name:             "User not found",
			requestBody:      `{"credentials":{"password":"test_password"}}`,
			mockJSON:         json.RawMessage(`{"password":"test_password"}`),
			mockError:        &ErrorUserNotFound,
			expectedHTTPCode: http.StatusNotFound,
			expectedErrCode:  ErrorUserNotFound.Code,
		},
		{
			name:             "Invalid credential type",
			requestBody:      `{"credentials":{"unsupported_type":"some_value"}}`,
			mockJSON:         json.RawMessage(`{"unsupported_type":"some_value"}`),
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
			req := httptest.NewRequest(http.MethodPost, "/users/"+userID+"/update-credentials",
				bytes.NewBufferString(tc.requestBody))
			req.SetPathValue("id", userID)
			rr := httptest.NewRecorder()

			handler.HandleUserCredentialUpdateRequest(rr, req)

			require.Equal(t, tc.expectedHTTPCode, rr.Code)

			var errResp apierror.ErrorResponse
			require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
			require.Equal(t, tc.expectedErrCode, errResp.Code)
		})
	}
}

func TestHandleUserCredentialUpdateRequest_MultipleCredentialTypes(t *testing.T) {
	userID := testUserID789

	mockSvc := NewUserServiceInterfaceMock(t)
	credentialsJSON := json.RawMessage(`{"password":"new-password","pin":"1234"}`)
	mockSvc.On("UpdateUserCredentials", mock.Anything, userID, credentialsJSON).Return(nil)

	handler := newUserHandler(mockSvc)
	req := httptest.NewRequest(http.MethodPost, "/users/"+userID+"/update-credentials",
		bytes.NewBufferString(`{"credentials":{"password":"new-password","pin":"1234"}}`))
	req.SetPathValue("id", userID)
	rr := httptest.NewRecorder()

	handler.HandleUserCredentialUpdateRequest(rr, req)

	require.Equal(t, http.StatusNoContent, rr.Code)
	require.Equal(t, 0, rr.Body.Len())
	mockSvc.AssertNumberOfCalls(t, "UpdateUserCredentials", 1)
}

func TestHandleUserListRequest_Success(t *testing.T) {
	mockSvc := NewUserServiceInterfaceMock(t)
	expectedResp := &UserListResponse{
		TotalResults: 10,
		Users:        []providers.User{{ID: "user-1"}},
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
		Users:        []providers.User{{ID: "user-1", Display: "Alice"}},
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
		Users:        []providers.User{{ID: "user-1"}},
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
	userReq := &CreateUserRequest{
		OUID:       "ou-corporate-root",
		Type:       "employee",
		Attributes: json.RawMessage(`{"username":"bob"}`),
	}
	createdUser := &providers.User{ID: "user-bob", Type: "employee", Attributes: json.RawMessage(`{"username":"bob"}`)}
	mockSvc.On("CreateUser", mock.Anything, mock.Anything).Return(createdUser, nil)

	handler := newUserHandler(mockSvc)
	body, _ := json.Marshal(userReq)
	req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	handler.HandleUserPostRequest(rr, req)

	require.Equal(t, http.StatusCreated, rr.Code)
	var resp providers.User
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	require.Equal(t, createdUser.ID, resp.ID)
}

func TestHandleUserGetRequest_Success(t *testing.T) {
	mockSvc := NewUserServiceInterfaceMock(t)
	userID := testUserID123
	expectedUser := &providers.User{ID: userID}
	mockSvc.On("GetUser", mock.Anything, userID, false).Return(expectedUser, nil)

	handler := newUserHandler(mockSvc)
	req := httptest.NewRequest(http.MethodGet, "/users/"+userID, nil)
	// Set path value for Go 1.22+ standard router
	req.SetPathValue("id", userID)
	rr := httptest.NewRecorder()

	handler.HandleUserGetRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp providers.User
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	require.Equal(t, userID, resp.ID)
}

func TestHandleUserGetRequest_IncludeDisplay(t *testing.T) {
	mockSvc := NewUserServiceInterfaceMock(t)
	userID := testUserID123
	expectedUser := &providers.User{ID: userID}
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
	userReq := &providers.User{Attributes: json.RawMessage(`{"name":"Updated"}`)}
	updatedUser := &providers.User{ID: userID, Attributes: json.RawMessage(`{"name":"Updated"}`)}
	mockSvc.On("UpdateUser", mock.Anything, userID, mock.Anything).Return(updatedUser, nil)

	handler := newUserHandler(mockSvc)
	body, _ := json.Marshal(userReq)
	req := httptest.NewRequest(http.MethodPut, "/users/"+userID, bytes.NewBuffer(body))
	req.SetPathValue("id", userID)
	rr := httptest.NewRecorder()

	handler.HandleUserPutRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp providers.User
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	require.Equal(t, userID, resp.ID)
}

func TestHandleUserDeleteRequest_Success(t *testing.T) {
	mockSvc := NewUserServiceInterfaceMock(t)
	userID := testUserID123
	mockSvc.On("DeleteUser", mock.Anything, userID).Return(nil)

	handler := newUserHandler(mockSvc)
	req := httptest.NewRequest(http.MethodDelete, "/users/"+userID, nil)
	req.SetPathValue("id", userID)
	rr := httptest.NewRecorder()

	handler.HandleUserDeleteRequest(rr, req)

	require.Equal(t, http.StatusNoContent, rr.Code)
}

func TestHandleUserListByPathRequest_Success(t *testing.T) {
	mockSvc := NewUserServiceInterfaceMock(t)
	expectedResp := &UserListResponse{
		TotalResults: 5,
		Users:        []providers.User{{ID: "user-path-1"}},
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
		Users:        []providers.User{{ID: "user-1", Display: "Bob"}},
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
	createdUser := &providers.User{ID: "user-new", Type: "customer"}
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
		Groups:       []providers.EntityGroup{{ID: "group-1", Name: "Admin"}},
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
		mockSvc.On("CreateUser", mock.Anything, mock.Anything).Return(nil, &tidcommon.InternalServerError).Once()
		req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"ouId":"ou-123","type":"customer"}`))
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
		svcErr := &tidcommon.InternalServerError
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
		mockSvc.On("DeleteUser", mock.Anything, userID).Return(&tidcommon.InternalServerError).Once()
		req := httptest.NewRequest(http.MethodDelete, "/users/"+userID, nil)
		req.SetPathValue("id", userID)
		rr := httptest.NewRecorder()
		handler.HandleUserDeleteRequest(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestHandleUserListRequest_ServiceError(t *testing.T) {
	mockSvc := NewUserServiceInterfaceMock(t)
	mockSvc.On("GetUserList", mock.Anything, 10, 0, mock.Anything, false).
		Return(nil, &tidcommon.InternalServerError).Once()

	handler := newUserHandler(mockSvc)
	req := httptest.NewRequest(http.MethodGet, "/users?limit=10&offset=0", nil)
	rr := httptest.NewRecorder()

	handler.HandleUserListRequest(rr, req)

	require.Equal(t, http.StatusInternalServerError, rr.Code)

	var errResp apierror.ErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, tidcommon.InternalServerError.Code, errResp.Code)
}

func TestHandleUserGroupsGetRequest_ErrorCases(t *testing.T) {
	mockSvc := NewUserServiceInterfaceMock(t)
	handler := newUserHandler(mockSvc)
	userID := testUserID123

	t.Run("MissingID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/users//groups", nil)
		rr := httptest.NewRecorder()
		handler.HandleUserGroupsGetRequest(rr, req)
		require.Equal(t, http.StatusNotFound, rr.Code)

		var errResp apierror.ErrorResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
		require.Equal(t, ErrorMissingUserID.Code, errResp.Code)
	})

	t.Run("InvalidPaginationParams", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/users/"+userID+"/groups?limit=abc", nil)
		req.SetPathValue("id", userID)
		rr := httptest.NewRecorder()
		handler.HandleUserGroupsGetRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)

		var errResp apierror.ErrorResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
		require.Equal(t, ErrorInvalidLimit.Code, errResp.Code)
	})
}

func TestHandleUserPutRequest_MissingID(t *testing.T) {
	mockSvc := NewUserServiceInterfaceMock(t)
	handler := newUserHandler(mockSvc)
	req := httptest.NewRequest(http.MethodPut, "/users/", strings.NewReader(`{"attributes":{}}`))
	rr := httptest.NewRecorder()

	handler.HandleUserPutRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)

	var errResp apierror.ErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, ErrorMissingUserID.Code, errResp.Code)
}

func TestHandleUserListByPathRequest_ErrorCases(t *testing.T) {
	mockSvc := NewUserServiceInterfaceMock(t)
	handler := newUserHandler(mockSvc)

	t.Run("InvalidPaginationParams", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/users/path/root/engineering?limit=abc", nil)
		req.SetPathValue("path", "root/engineering")
		rr := httptest.NewRecorder()
		handler.HandleUserListByPathRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)

		var errResp apierror.ErrorResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
		require.Equal(t, ErrorInvalidLimit.Code, errResp.Code)
	})

	t.Run("InvalidFilter", func(t *testing.T) {
		req := httptest.NewRequest(
			http.MethodGet, "/users/path/root/engineering?filter=username%20invalid%20%22alice%22", nil)
		req.SetPathValue("path", "root/engineering")
		rr := httptest.NewRecorder()
		handler.HandleUserListByPathRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)

		var errResp apierror.ErrorResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
		require.Equal(t, ErrorInvalidFilter.Code, errResp.Code)
	})

	t.Run("MissingPath", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/users/path/", nil)
		rr := httptest.NewRecorder()
		handler.HandleUserListByPathRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)

		var errResp apierror.ErrorResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
		require.Equal(t, ErrorHandlePathRequired.Code, errResp.Code)
	})
}

func TestHandleUserPostByPathRequest_InvalidBody(t *testing.T) {
	mockSvc := NewUserServiceInterfaceMock(t)
	handler := newUserHandler(mockSvc)
	req := httptest.NewRequest(http.MethodPost, "/users/path/root/sales", strings.NewReader("invalid"))
	req.SetPathValue("path", "root/sales")
	rr := httptest.NewRecorder()

	handler.HandleUserPostByPathRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)

	var errResp apierror.ErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, ErrorInvalidRequestFormat.Code, errResp.Code)
}

func TestHandleSelfUserGetRequest_ServiceError(t *testing.T) {
	userID := testUserID123
	authCtx := security.NewSecurityContextForTest(userID, "", "", nil, nil)

	mockSvc := NewUserServiceInterfaceMock(t)
	mockSvc.On("GetUser", mock.Anything, userID, false).Return(nil, &ErrorUserNotFound).Once()

	handler := newUserHandler(mockSvc)
	req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
	req = req.WithContext(security.WithSecurityContextTest(req.Context(), authCtx))
	rr := httptest.NewRecorder()

	handler.HandleSelfUserGetRequest(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)

	var errResp apierror.ErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, ErrorUserNotFound.Code, errResp.Code)
}

func TestHandleSelfUserPutRequest_Unauthorized(t *testing.T) {
	mockSvc := NewUserServiceInterfaceMock(t)
	handler := newUserHandler(mockSvc)
	req := httptest.NewRequest(http.MethodPut, "/users/me",
		bytes.NewBufferString(`{"attributes":{"email":"alice@example.com"}}`))
	rr := httptest.NewRecorder()

	handler.HandleSelfUserPutRequest(rr, req)

	require.Equal(t, http.StatusUnauthorized, rr.Code)

	var errResp apierror.ErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, ErrorAuthenticationFailed.Code, errResp.Code)
}

func TestHandleSelfUserPutRequest_EmptyAttributes(t *testing.T) {
	userID := testUserID456
	authCtx := security.NewSecurityContextForTest(userID, "", "", nil, nil)

	mockSvc := NewUserServiceInterfaceMock(t)
	handler := newUserHandler(mockSvc)
	req := httptest.NewRequest(http.MethodPut, "/users/me", bytes.NewBufferString(`{}`))
	req = req.WithContext(security.WithSecurityContextTest(req.Context(), authCtx))
	rr := httptest.NewRecorder()

	handler.HandleSelfUserPutRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)

	var errResp apierror.ErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, ErrorInvalidRequestFormat.Code, errResp.Code)
}

func TestHandleSelfUserPutRequest_ServiceError(t *testing.T) {
	userID := testUserID456
	authCtx := security.NewSecurityContextForTest(userID, "", "", nil, nil)
	attributes := json.RawMessage(`{"email":"alice@example.com"}`)

	mockSvc := NewUserServiceInterfaceMock(t)
	mockSvc.On("UpdateUserAttributes", mock.Anything, userID, attributes).
		Return(nil, &tidcommon.InternalServerError).Once()

	handler := newUserHandler(mockSvc)
	body := bytes.NewBufferString(`{"attributes":{"email":"alice@example.com"}}`)
	req := httptest.NewRequest(http.MethodPut, "/users/me", body)
	req = req.WithContext(security.WithSecurityContextTest(req.Context(), authCtx))
	rr := httptest.NewRecorder()

	handler.HandleSelfUserPutRequest(rr, req)

	require.Equal(t, http.StatusInternalServerError, rr.Code)

	var errResp apierror.ErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, tidcommon.InternalServerError.Code, errResp.Code)
}

func TestHandleSelfUserCredentialUpdateRequest_Unauthorized(t *testing.T) {
	mockSvc := NewUserServiceInterfaceMock(t)
	handler := newUserHandler(mockSvc)
	req := httptest.NewRequest(http.MethodPost, "/users/me/update-credentials",
		bytes.NewBufferString(`{"attributes":{"password":"Secret123!"}}`))
	rr := httptest.NewRecorder()

	handler.HandleSelfUserCredentialUpdateRequest(rr, req)

	require.Equal(t, http.StatusUnauthorized, rr.Code)

	var errResp apierror.ErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, ErrorAuthenticationFailed.Code, errResp.Code)
}

func TestHandleSelfUserCredentialUpdateRequest_InvalidBody(t *testing.T) {
	userID := testUserID789
	authCtx := security.NewSecurityContextForTest(userID, "", "", nil, nil)

	mockSvc := NewUserServiceInterfaceMock(t)
	handler := newUserHandler(mockSvc)
	req := httptest.NewRequest(http.MethodPost, "/users/me/update-credentials",
		bytes.NewBufferString(`{"attributes":`))
	req = req.WithContext(security.WithSecurityContextTest(req.Context(), authCtx))
	rr := httptest.NewRecorder()

	handler.HandleSelfUserCredentialUpdateRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)

	var errResp apierror.ErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, ErrorInvalidRequestFormat.Code, errResp.Code)
}

func TestHandleError_ErrorUnauthorized_Returns403(t *testing.T) {
	tests := []struct {
		name     string
		svcErr   *tidcommon.ServiceError
		wantCode int
	}{
		{
			name:     "UnauthorizedError_ReturnsForbidden",
			svcErr:   &tidcommon.ErrorUnauthorized,
			wantCode: http.StatusForbidden,
		},
		{
			name:     "AuthenticationFailedError_ReturnsUnauthorized",
			svcErr:   &ErrorAuthenticationFailed,
			wantCode: http.StatusUnauthorized,
		},
		{
			name:     "InternalServerError_Returns500",
			svcErr:   &tidcommon.InternalServerError,
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

func TestHandleUserPostRequest_ValidationError_MissingFields(t *testing.T) {
	mockSvc := NewUserServiceInterfaceMock(t)
	handler := newUserHandler(mockSvc)

	body := `{"groups":["engineering"]}`
	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(body))
	rr := httptest.NewRecorder()

	handler.HandleUserPostRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	require.Equal(t, "INVALID_INPUT_METADATA", resp["code"])
	mockSvc.AssertNotCalled(t, "CreateUser", mock.Anything, mock.Anything)
}

func TestHandleUserListRequest_PaginationEdgeCases(t *testing.T) {
	mockSvc := NewUserServiceInterfaceMock(t)
	handler := newUserHandler(mockSvc)

	t.Run("Malformed Limit String Parameter", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/users?limit=xyz", nil)
		rr := httptest.NewRecorder()
		handler.HandleUserListRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("Negative Limit Value Guard", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/users?limit=-100", nil)
		rr := httptest.NewRecorder()
		handler.HandleUserListRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("Malformed Offset Value Parameter", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/users?offset=abc", nil)
		rr := httptest.NewRecorder()
		handler.HandleUserListRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("Negative Offset Value Guard", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/users?offset=-5", nil)
		rr := httptest.NewRecorder()
		handler.HandleUserListRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})
}

func TestHandleUserListRequest_FilterExpressionEdgeCases(t *testing.T) {
	mockSvc := NewUserServiceInterfaceMock(t)
	handler := newUserHandler(mockSvc)

	t.Run("Empty Filter Value Field Space", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/users?filter=%20%20", nil)
		rr := httptest.NewRecorder()
		handler.HandleUserListRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("Unsupported Expression Operator", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/users?filter=age%20gt%2018", nil)
		rr := httptest.NewRecorder()
		handler.HandleUserListRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("Malformed Value Token Payload", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/users?filter=username%20eq", nil)
		rr := httptest.NewRecorder()
		handler.HandleUserListRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})
}

func TestHandleUserMissingIDs_DirectTrimming(t *testing.T) {
	mockSvc := NewUserServiceInterfaceMock(t)
	handler := newUserHandler(mockSvc)

	t.Run("Empty String Strip on PUT Route", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/users/", bytes.NewBufferString(`{}`))
		rr := httptest.NewRecorder()
		handler.HandleUserPutRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("Empty String Strip on DELETE Route", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/users/", nil)
		rr := httptest.NewRecorder()
		handler.HandleUserDeleteRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("Missing Segment on Path Fetch Route", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/users/path/", nil)
		req.SetPathValue("path", "") // Force early validation block error
		rr := httptest.NewRecorder()
		handler.HandleUserListByPathRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("Missing Segment on Path Create Route", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/users/path/", nil)
		req.SetPathValue("path", "")
		rr := httptest.NewRecorder()
		handler.HandleUserPostByPathRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})
}

func TestHandleSelfUserExits_MissingContextTokens(t *testing.T) {
	mockSvc := NewUserServiceInterfaceMock(t)
	handler := newUserHandler(mockSvc)
	emptyAuthCtx := security.NewSecurityContextForTest("   ", "", "", nil, nil) // Spaces force token rejection

	t.Run("Blank Context Verification on Self GET", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
		req = req.WithContext(security.WithSecurityContextTest(req.Context(), emptyAuthCtx))
		rr := httptest.NewRecorder()
		handler.HandleSelfUserGetRequest(rr, req)
		require.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("Blank Context Verification on Self PUT", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/users/me", bytes.NewBufferString(`{}`))
		req = req.WithContext(security.WithSecurityContextTest(req.Context(), emptyAuthCtx))
		rr := httptest.NewRecorder()
		handler.HandleSelfUserPutRequest(rr, req)
		require.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("Blank Context Verification on Credential POST", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/users/me/update-credentials", bytes.NewBufferString(`{}`))
		req = req.WithContext(security.WithSecurityContextTest(req.Context(), emptyAuthCtx))
		rr := httptest.NewRecorder()
		handler.HandleSelfUserCredentialUpdateRequest(rr, req)
		require.Equal(t, http.StatusUnauthorized, rr.Code)
	})
}

func TestHandleUserUsagesGetRequest_Success(t *testing.T) {
	mockSvc := NewUserServiceInterfaceMock(t)
	total := 1
	mockSvc.On("GetUserUsages", mock.Anything, testUserID123).
		Return(&resourcedependency.DependenciesResponse{
			TotalResults: &total,
			Count:        1,
			Summary:      map[string]int{resourcedependency.ResourceTypeAgent: 1},
			Usages: []resourcedependency.ResourceDependency{
				{ResourceType: resourcedependency.ResourceTypeAgent, ID: "agent-1",
					DisplayName: "Support Agent", BehaviorOnDelete: resourcedependency.BehaviorFallback},
			},
		}, nil)

	handler := newUserHandler(mockSvc)
	req := httptest.NewRequest(http.MethodGet, "/users/"+testUserID123+"/usages", nil)
	req.SetPathValue("id", testUserID123)
	rr := httptest.NewRecorder()

	handler.HandleUserUsagesGetRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var response resourcedependency.DependenciesResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&response))
	require.NotNil(t, response.TotalResults)
	require.Equal(t, 1, *response.TotalResults)
	require.Len(t, response.Usages, 1)
	require.Equal(t, resourcedependency.ResourceTypeAgent, response.Usages[0].ResourceType)
}

func TestHandleUserUsagesGetRequest_MissingID(t *testing.T) {
	mockSvc := NewUserServiceInterfaceMock(t)
	handler := newUserHandler(mockSvc)
	req := httptest.NewRequest(http.MethodGet, "/users//usages", nil)
	rr := httptest.NewRecorder()

	handler.HandleUserUsagesGetRequest(rr, req)

	// ErrorMissingUserID maps to 404 in the user handler's error mapping.
	require.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleUserUsagesGetRequest_NotFound(t *testing.T) {
	mockSvc := NewUserServiceInterfaceMock(t)
	mockSvc.On("GetUserUsages", mock.Anything, testUserID123).
		Return(nil, &ErrorUserNotFound)

	handler := newUserHandler(mockSvc)
	req := httptest.NewRequest(http.MethodGet, "/users/"+testUserID123+"/usages", nil)
	req.SetPathValue("id", testUserID123)
	rr := httptest.NewRecorder()

	handler.HandleUserUsagesGetRequest(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleSelfUserMetadataGetRequest_Success(t *testing.T) {
	userID := testUserID123
	authCtx := security.NewSecurityContextForTest(userID, "", "", nil, nil)

	mockSvc := NewUserServiceInterfaceMock(t)
	expectedSchema := &entitytype.EntityType{
		Handle: "employee",
		Schema: json.RawMessage(`{"email":{"type":"string"}}`),
	}
	mockSvc.On("GetUserMetadata", mock.Anything, userID).Return(expectedSchema, nil)

	handler := newUserHandler(mockSvc)
	req := httptest.NewRequest(http.MethodGet, "/users/me/meta", nil)
	req = req.WithContext(security.WithSecurityContextTest(req.Context(), authCtx))
	rr := httptest.NewRecorder()

	handler.HandleSelfUserMetadataGetRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Contains(t, rr.Header().Get("Content-Type"), "application/json")

	var res map[string]interface{}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&res))
	require.NotNil(t, res["schema"])
}

func TestHandleSelfUserMetadataGetRequest_Unauthorized(t *testing.T) {
	mockSvc := NewUserServiceInterfaceMock(t)
	handler := newUserHandler(mockSvc)
	req := httptest.NewRequest(http.MethodGet, "/users/me/meta", nil)
	rr := httptest.NewRecorder()

	handler.HandleSelfUserMetadataGetRequest(rr, req)

	require.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestHandleSelfUserMetadataGetRequest_ServiceError(t *testing.T) {
	userID := testUserID123
	authCtx := security.NewSecurityContextForTest(userID, "", "", nil, nil)

	mockSvc := NewUserServiceInterfaceMock(t)
	mockSvc.On("GetUserMetadata", mock.Anything, userID).Return(nil, &ErrorUserNotFound)

	handler := newUserHandler(mockSvc)
	req := httptest.NewRequest(http.MethodGet, "/users/me/meta", nil)
	req = req.WithContext(security.WithSecurityContextTest(req.Context(), authCtx))
	rr := httptest.NewRecorder()

	handler.HandleSelfUserMetadataGetRequest(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleSharingPolicyPostRequest_RefusesFieldsTheSchemaDoesNotList(t *testing.T) {
	bodies := map[string]string{
		"MisspelledExclusion": `{"childUnits":{"mode":"all","excludeOuIds":["ou-1"]}}`,
		"UnknownTopLevel":     `{"childUnits":{"mode":"all"},"extra":true}`,
		"FlatTargets":         `{"targets":[{"scope":"child","ouId":"ou-1"}]}`,
		"PolicyID":            `{"id":"policy-1","childUnits":{"mode":"all"}}`,
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			handler := newUserHandler(NewUserServiceInterfaceMock(t))
			req := httptest.NewRequest(http.MethodPost, "/users/user-1/sharing-policies", strings.NewReader(body))
			req.SetPathValue("id", "user-1")
			rr := httptest.NewRecorder()

			handler.HandleSharingPolicyPostRequest(rr, req)

			require.Equal(t, http.StatusBadRequest, rr.Code)
			var errResp apierror.ErrorResponse
			require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
			require.Equal(t, ErrorInvalidRequestFormat.Code, errResp.Code)
		})
	}
}

func TestHandleSharingPolicyPostRequest_RefusesADeploymentWideReach(t *testing.T) {
	bodies := map[string]string{
		"AllUnits":               `{"allUnits":{}}`,
		"AllUnitsWithExclusions": `{"allUnits":{"excludedOuIds":["ou-1"]}}`,
		"AllOtherTreeRoots":      `{"otherTreeRoots":{"mode":"all"}}`,
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			handler := newUserHandler(NewUserServiceInterfaceMock(t))
			req := httptest.NewRequest(http.MethodPost, "/users/user-1/sharing-policies", strings.NewReader(body))
			req.SetPathValue("id", "user-1")
			rr := httptest.NewRecorder()

			handler.HandleSharingPolicyPostRequest(rr, req)

			require.Equal(t, http.StatusBadRequest, rr.Code)
			var errResp apierror.ErrorResponse
			require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
			require.Equal(t, ErrorDeploymentWideSharingNotAllowed.Code, errResp.Code)
		})
	}
}

func TestHandleSharingPolicyPostRequest_PassesTheWireShapeToTheService(t *testing.T) {
	expected := sharing.PolicyRequest{
		InitiatingOUID: "ou-child",
		ChildUnits: &sharing.ChildUnitsRequest{
			Mode: sharing.ChildUnitsSelected,
			Units: []sharing.ChildUnitRequest{
				{OUID: "ou-a", Reach: sharing.ReachUnit},
				{OUID: "ou-b", Reach: sharing.ReachSubtree},
			},
		},
	}
	mockSvc := NewUserServiceInterfaceMock(t)
	mockSvc.On("CreateSharingPolicy", mock.Anything, "user-1", expected).
		Return(sharing.Policy{ID: "policy-1", Version: 1}, (*tidcommon.ServiceError)(nil)).Once()
	handler := newUserHandler(mockSvc)
	body := `{"initiatingOuId":"ou-child","childUnits":{"mode":"selected","units":[` +
		`{"ouId":"ou-a","reach":"unit"},{"ouId":"ou-b","reach":"subtree"}]}}`
	req := httptest.NewRequest(http.MethodPost, "/users/user-1/sharing-policies", strings.NewReader(body))
	req.SetPathValue("id", "user-1")
	rr := httptest.NewRecorder()

	handler.HandleSharingPolicyPostRequest(rr, req)

	require.Equal(t, http.StatusCreated, rr.Code)
}

// Overlay rules decode and reach the service, on a section and on a named unit. The refusal is the
// framework's, since a user declares no field for them to govern.
func TestHandleSharingPolicyPostRequest_PassesOverlayRulesToTheService(t *testing.T) {
	var recorded sharing.PolicyRequest
	mockSvc := NewUserServiceInterfaceMock(t)
	mockSvc.On("CreateSharingPolicy", mock.Anything, "user-1", mock.Anything).
		Run(func(args mock.Arguments) { recorded = args.Get(2).(sharing.PolicyRequest) }).
		Return(sharing.Policy{}, &ErrorInvalidSharingPolicy).Once()
	handler := newUserHandler(mockSvc)
	body := `{"childUnits":{"mode":"selected","overlayRules":{"roles":{"editable":false}},"units":[` +
		`{"ouId":"ou-1","reach":"unit","overlayRules":{"roles":{"editable":true}}}]}}`
	req := httptest.NewRequest(http.MethodPost, "/users/user-1/sharing-policies", strings.NewReader(body))
	req.SetPathValue("id", "user-1")
	rr := httptest.NewRecorder()

	handler.HandleSharingPolicyPostRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	var errResp apierror.ErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, ErrorInvalidSharingPolicy.Code, errResp.Code)
	require.NotNil(t, recorded.ChildUnits)
	require.Contains(t, recorded.ChildUnits.OverlayRules, "roles")
	require.Len(t, recorded.ChildUnits.Units, 1)
	require.Contains(t, recorded.ChildUnits.Units[0].OverlayRules, "roles")
}

func TestHandleSharingPolicyPutRequest_RefusesAChangeOfIssuer(t *testing.T) {
	handler := newUserHandler(NewUserServiceInterfaceMock(t))
	body := `{"initiatingOuId":"ou-child","childUnits":{"mode":"all"},"version":1}`
	req := httptest.NewRequest(http.MethodPut, "/users/user-1/sharing-policies/policy-1", strings.NewReader(body))
	req.SetPathValue("id", "user-1")
	req.SetPathValue("policyId", "policy-1")
	rr := httptest.NewRecorder()

	handler.HandleSharingPolicyPutRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	var errResp apierror.ErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, ErrorInvalidRequestFormat.Code, errResp.Code)
}

func TestHandleSharingPolicyPutRequest_PassesTheSectionsToTheService(t *testing.T) {
	expected := sharing.PolicyRequest{
		ChildUnits: &sharing.ChildUnitsRequest{Mode: sharing.ChildUnitsAll, ExcludedOUIDs: []string{"ou-a"}},
		Version:    3,
	}
	mockSvc := NewUserServiceInterfaceMock(t)
	mockSvc.On("UpdateSharingPolicy", mock.Anything, "user-1", "policy-1", expected).
		Return(sharing.Policy{ID: "policy-1", Version: 4}, (*tidcommon.ServiceError)(nil)).Once()
	handler := newUserHandler(mockSvc)
	body := `{"childUnits":{"mode":"all","excludedOuIds":["ou-a"]},"version":3}`
	req := httptest.NewRequest(http.MethodPut, "/users/user-1/sharing-policies/policy-1", strings.NewReader(body))
	req.SetPathValue("id", "user-1")
	req.SetPathValue("policyId", "policy-1")
	rr := httptest.NewRecorder()

	handler.HandleSharingPolicyPutRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
}

func TestHandleSharingPolicy_StatusPerError(t *testing.T) {
	cases := []struct {
		svcErr tidcommon.ServiceError
		status int
	}{
		{ErrorSharingPolicyNotFound, http.StatusNotFound},
		{ErrorUserNotFound, http.StatusNotFound},
		{ErrorUserNotSharedToOU, http.StatusNotFound},
		{ErrorSharingPolicyExists, http.StatusConflict},
		{ErrorSharingPolicyVersionMismatch, http.StatusConflict},
		{ErrorInvalidSharingPolicy, http.StatusBadRequest},
		{ErrorSharingPolicyDeclared, http.StatusBadRequest},
		{ErrorMissingOUIDParameter, http.StatusBadRequest},
		{tidcommon.ErrorUnauthorized, http.StatusForbidden},
		{tidcommon.InternalServerError, http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.svcErr.Code, func(t *testing.T) {
			svcErr := tc.svcErr
			mockSvc := NewUserServiceInterfaceMock(t)
			mockSvc.On("GetSharingPolicy", mock.Anything, "user-1", "policy-1").
				Return(sharing.Policy{}, &svcErr).Once()
			handler := newUserHandler(mockSvc)
			req := httptest.NewRequest(http.MethodGet, "/users/user-1/sharing-policies/policy-1", nil)
			req.SetPathValue("id", "user-1")
			req.SetPathValue("policyId", "policy-1")
			rr := httptest.NewRecorder()

			handler.HandleSharingPolicyGetRequest(rr, req)

			require.Equal(t, tc.status, rr.Code)
		})
	}
}

func TestHandleSharingPolicyListRequest_Pagination(t *testing.T) {
	t.Run("DefaultsThePageSize", func(t *testing.T) {
		mockSvc := NewUserServiceInterfaceMock(t)
		mockSvc.On("ListSharingPolicies", mock.Anything, "user-1", 30, 0).
			Return(sharing.PolicyList{}, (*tidcommon.ServiceError)(nil)).Once()
		req := httptest.NewRequest(http.MethodGet, "/users/user-1/sharing-policies", nil)
		req.SetPathValue("id", "user-1")
		rr := httptest.NewRecorder()

		newUserHandler(mockSvc).HandleSharingPolicyListRequest(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)
	})
	t.Run("RefusesAMalformedLimit", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/users/user-1/sharing-policies?limit=abc", nil)
		req.SetPathValue("id", "user-1")
		rr := httptest.NewRecorder()

		newUserHandler(NewUserServiceInterfaceMock(t)).HandleSharingPolicyListRequest(rr, req)

		require.Equal(t, http.StatusBadRequest, rr.Code)
	})
}

// A user with no operator note is returned as is, without a copy.
func TestWithoutOperatorNote_ReturnsAUserWithoutANoteUnchanged(t *testing.T) {
	cases := []struct {
		name string
		user *providers.User
	}{
		{name: "no user"},
		{name: "no status", user: &providers.User{ID: testUserID123}},
		{name: "no details", user: &providers.User{ID: testUserID123,
			Status: &providers.AccessStatusReport{Value: providers.AccessStatusActive}}},
		{name: "an empty note", user: &providers.User{ID: testUserID123, Status: &providers.AccessStatusReport{
			Value:   providers.AccessStatusSuspended,
			Details: &providers.AccessStatusDetails{Reason: providers.AccessHoldReasonAdministrativeSuspension},
		}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.True(t, withoutOperatorNote(tc.user) == tc.user)
		})
	}
}
