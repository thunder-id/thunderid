// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package scim

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	scimconfig "github.com/thunder-id/thunderid/internal/scim/config"
	"github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

const scimTestPayloadBody = `{
		"schemas": ["urn:thunderid:params:scim:schemas:person:2.0:User"],
		"urn:thunderid:params:scim:schemas:person:2.0:User": {
			"given_name": "Test"
		}
	}`

// TestHandleUsersGetRequest_Success tests Handle Users Get Request for Success.
func TestHandleUsersGetRequest_Success(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	expectedUser := &SCIMUser{
		Schemas: []string{SCIMCoreUserSchemaURN},
		ID:      "user-123",
		Meta: SCIMMeta{
			ResourceType: "User",
			Location:     testBaseURL + "/scim/v2/Users/user-123",
			Version:      `W/"abc12345"`,
		},
	}
	mockSvc.On("GetUser", mock.Anything, "user-123", testBaseURL).Return(expectedUser, (*tidcommon.ServiceError)(nil))

	h := newSCIMUsersHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users/user-123", nil)
	req.SetPathValue("id", "user-123")
	rr := httptest.NewRecorder()

	h.HandleUsersGetRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, expectedUser.Meta.Version, rr.Header().Get("ETag"))
	require.Equal(t, constants.SCIMContentType, rr.Header().Get("Content-Type"))

	var got SCIMUser
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	require.Equal(t, expectedUser.ID, got.ID)
}

// TestHandleUsersGetRequest_NotFound tests Handle Users Get Request for Not Found.
func TestHandleUsersGetRequest_NotFound(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("GetUser", mock.Anything, "unknown", testBaseURL).Return((*SCIMUser)(nil), &ErrorUserNotFound)

	h := newSCIMUsersHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users/unknown", nil)
	req.SetPathValue("id", "unknown")
	rr := httptest.NewRecorder()

	h.HandleUsersGetRequest(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
}

// TestHandleUsersDeleteRequest_Success tests Handle Users Delete Request for Success.
func TestHandleUsersDeleteRequest_Success(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("DeleteUser", mock.Anything, "user-123", "").Return((*tidcommon.ServiceError)(nil))

	h := newSCIMUsersHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodDelete, "/scim/v2/Users/user-123", nil)
	req.SetPathValue("id", "user-123")
	rr := httptest.NewRecorder()

	h.HandleUsersDeleteRequest(rr, req)

	require.Equal(t, http.StatusNoContent, rr.Code)
}

// TestHandleUsersCreateRequest_Success tests Handle Users Create Request for Success.
func TestHandleUsersCreateRequest_Success(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)

	expectedUser := &SCIMUser{
		Schemas: []string{SCIMCoreUserSchemaURN, "urn:thunderid:params:scim:schemas:person:2.0:User"},
		ID:      "user-123",
		Meta: SCIMMeta{
			ResourceType: "User",
			Location:     testBaseURL + "/scim/v2/Users/user-123",
			Version:      `W/"abc12345"`,
		},
	}
	mockSvc.On(
		"CreateUser", mock.Anything, mock.AnythingOfType("*scim.SCIMUserPayload"), testBaseURL,
	).Return(expectedUser, (*tidcommon.ServiceError)(nil))

	h := newSCIMUsersHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users", bytes.NewBufferString(scimTestPayloadBody))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersCreateRequest(rr, req)

	require.Equal(t, http.StatusCreated, rr.Code)
	require.Equal(t, expectedUser.Meta.Version, rr.Header().Get("ETag"))
	require.Equal(t, expectedUser.Meta.Location, rr.Header().Get("Location"))
}

// TestHandleUsersListRequest_Success tests Handle Users List Request for Success.
func TestHandleUsersListRequest_Success(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	expectedResp := SCIMUserListResponse{
		Schemas:      []string{SCIMListResponseSchemaURN},
		TotalResults: 0,
		StartIndex:   1,
		ItemsPerPage: constants.DefaultPageSize,
		Resources:    []SCIMUser{},
	}
	mockSvc.On("ListUsers", mock.Anything, 1, constants.DefaultPageSize, mock.Anything,
		testBaseURL).Return(expectedResp, (*tidcommon.ServiceError)(nil))

	h := newSCIMUsersHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users", nil)
	rr := httptest.NewRecorder()

	h.HandleUsersListRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
}

// TestHandleUsersReplaceRequest_Success tests Handle Users Replace Request for Success.
func TestHandleUsersReplaceRequest_Success(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)

	expectedUser := &SCIMUser{
		Schemas: []string{SCIMCoreUserSchemaURN, "urn:thunderid:params:scim:schemas:person:2.0:User"},
		ID:      "user-123",
		Meta: SCIMMeta{
			ResourceType: "User",
			Location:     testBaseURL + "/scim/v2/Users/user-123",
			Version:      `W/"abc12345"`,
		},
	}
	mockSvc.On(
		"ReplaceUser", mock.Anything, "user-123", mock.AnythingOfType("*scim.SCIMUserPayload"), "", testBaseURL, false,
	).Return(expectedUser, (*tidcommon.ServiceError)(nil))

	h := newSCIMUsersHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodPut, "/scim/v2/Users/user-123", bytes.NewBufferString(scimTestPayloadBody))
	req.SetPathValue("id", "user-123")
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersReplaceRequest(rr, req)
	require.Equal(t, expectedUser.Meta.Version, rr.Header().Get("ETag"))
	require.Equal(t, http.StatusOK, rr.Code)
}

// --- GET /scim/v2/Me (RFC 7644 §3.11) ---

// TestHandleMeGetRequest_Success tests Handle Me Get Request for Success.
func TestHandleMeGetRequest_Success(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	expectedUser := &SCIMUser{
		Schemas: []string{SCIMCoreUserSchemaURN},
		ID:      "user-123",
		Meta: SCIMMeta{
			ResourceType: "User",
			Location:     testBaseURL + "/scim/v2/Users/user-123",
			Version:      `W/"abc12345"`,
		},
	}
	mockSvc.On("GetUser", mock.Anything, "user-123", testBaseURL).Return(expectedUser, (*tidcommon.ServiceError)(nil))

	h := newSCIMUsersHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Me", nil)
	authCtx := security.NewSecurityContextForTest("user-123", "", "", nil, nil)
	req = req.WithContext(security.WithSecurityContextTest(req.Context(), authCtx))
	rr := httptest.NewRecorder()

	h.HandleMeGetRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, expectedUser.Meta.Location, rr.Header().Get("Location"))
	require.Equal(t, expectedUser.Meta.Version, rr.Header().Get("ETag"))

	var got SCIMUser
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	require.Equal(t, expectedUser.ID, got.ID)
}

// TestHandleMeGetRequest_NoSubject_Returns401 tests Handle Me Get Request for No Subject Returns 401.
func TestHandleMeGetRequest_NoSubject_Returns401(t *testing.T) {
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Me", nil)
	rr := httptest.NewRecorder()

	h.HandleMeGetRequest(rr, req)

	require.Equal(t, http.StatusUnauthorized, rr.Code)
}

// TestHandleMeGetRequest_AppliesAttributeProjection tests Handle Me Get Request for Applies Attribute Projection.
func TestHandleMeGetRequest_AppliesAttributeProjection(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	expectedUser := &SCIMUser{
		ID:      "user-123",
		Schemas: []string{SCIMCoreUserSchemaURN},
		Meta:    SCIMMeta{ResourceType: "User", Version: `W/"abc12345"`},
		CoreAttrs: map[string]json.RawMessage{
			"userName": json.RawMessage(`"alice"`),
			"active":   json.RawMessage(`true`),
		},
	}
	mockSvc.On("GetUser", mock.Anything, "user-123", testBaseURL).Return(expectedUser, (*tidcommon.ServiceError)(nil))

	h := newSCIMUsersHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Me?attributes=userName", nil)
	authCtx := security.NewSecurityContextForTest("user-123", "", "", nil, nil)
	req = req.WithContext(security.WithSecurityContextTest(req.Context(), authCtx))
	rr := httptest.NewRecorder()

	h.HandleMeGetRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var got map[string]interface{}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	require.Equal(t, "alice", got["userName"])
	require.NotContains(t, got, "active")
}

// TestHandleMeGetRequest_ConflictingAttributesParams_Returns400 tests Handle Me Get Request for Conflicting
// Attributes Params Returns 400.
func TestHandleMeGetRequest_ConflictingAttributesParams_Returns400(t *testing.T) {
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Me?attributes=userName&excludedAttributes=active", nil)
	authCtx := security.NewSecurityContextForTest("user-123", "", "", nil, nil)
	req = req.WithContext(security.WithSecurityContextTest(req.Context(), authCtx))
	rr := httptest.NewRecorder()

	h.HandleMeGetRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestHandleMeGetRequest_ServiceError_Returns404 tests Handle Me Get Request for Service Error Returns 404.
func TestHandleMeGetRequest_ServiceError_Returns404(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("GetUser", mock.Anything, "user-123", testBaseURL).
		Return((*SCIMUser)(nil), &ErrorUserNotFound)

	h := newSCIMUsersHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Me", nil)
	authCtx := security.NewSecurityContextForTest("user-123", "", "", nil, nil)
	req = req.WithContext(security.WithSecurityContextTest(req.Context(), authCtx))
	rr := httptest.NewRecorder()

	h.HandleMeGetRequest(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
}

// --- PUT /scim/v2/Me (RFC 7644 §3.11) ---

// TestHandleMeReplaceRequest_Success tests Handle Me Replace Request for Success.
func TestHandleMeReplaceRequest_Success(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	expectedUser := &SCIMUser{
		Schemas: []string{SCIMCoreUserSchemaURN, "urn:thunderid:params:scim:schemas:person:2.0:User"},
		ID:      "user-123",
		Meta: SCIMMeta{
			ResourceType: "User",
			Location:     testBaseURL + "/scim/v2/Users/user-123",
			Version:      `W/"abc12345"`,
		},
	}
	mockSvc.On(
		"ReplaceUser", mock.Anything, "user-123", mock.AnythingOfType("*scim.SCIMUserPayload"), "", testBaseURL, true,
	).Return(expectedUser, (*tidcommon.ServiceError)(nil))

	h := newSCIMUsersHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodPut, "/scim/v2/Me", bytes.NewBufferString(scimTestPayloadBody))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	authCtx := security.NewSecurityContextForTest("user-123", "", "", nil, nil)
	req = req.WithContext(security.WithSecurityContextTest(req.Context(), authCtx))
	rr := httptest.NewRecorder()

	h.HandleMeReplaceRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, expectedUser.Meta.Location, rr.Header().Get("Location"))
	require.Equal(t, expectedUser.Meta.Version, rr.Header().Get("ETag"))
}

// TestHandleMeReplaceRequest_NoSubject_Returns401 tests Handle Me Replace Request for No Subject Returns 401.
func TestHandleMeReplaceRequest_NoSubject_Returns401(t *testing.T) {
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testBaseURL)
	req := httptest.NewRequest(http.MethodPut, "/scim/v2/Me", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleMeReplaceRequest(rr, req)

	require.Equal(t, http.StatusUnauthorized, rr.Code)
}

// TestHandleMeReplaceRequest_WrongContentType_Returns400 tests Handle Me Replace Request for Wrong Content
// Type Returns 400.
func TestHandleMeReplaceRequest_WrongContentType_Returns400(t *testing.T) {
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testBaseURL)
	req := httptest.NewRequest(http.MethodPut, "/scim/v2/Me", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	authCtx := security.NewSecurityContextForTest("user-123", "", "", nil, nil)
	req = req.WithContext(security.WithSecurityContextTest(req.Context(), authCtx))
	rr := httptest.NewRecorder()

	h.HandleMeReplaceRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestHandleMeReplaceRequest_EmptyBody_Returns400 tests Handle Me Replace Request for Empty Body Returns 400.
func TestHandleMeReplaceRequest_EmptyBody_Returns400(t *testing.T) {
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testBaseURL)
	req := httptest.NewRequest(http.MethodPut, "/scim/v2/Me", http.NoBody)
	req.Header.Set("Content-Type", constants.SCIMContentType)
	authCtx := security.NewSecurityContextForTest("user-123", "", "", nil, nil)
	req = req.WithContext(security.WithSecurityContextTest(req.Context(), authCtx))
	rr := httptest.NewRecorder()

	h.HandleMeReplaceRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestHandleMeReplaceRequest_InvalidJSON_Returns400 tests Handle Me Replace Request for Invalid JSON Returns 400.
func TestHandleMeReplaceRequest_InvalidJSON_Returns400(t *testing.T) {
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testBaseURL)
	req := httptest.NewRequest(http.MethodPut, "/scim/v2/Me", bytes.NewBufferString(`not json`))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	authCtx := security.NewSecurityContextForTest("user-123", "", "", nil, nil)
	req = req.WithContext(security.WithSecurityContextTest(req.Context(), authCtx))
	rr := httptest.NewRecorder()

	h.HandleMeReplaceRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestHandleMeReplaceRequest_ServiceError_Returns404 tests Handle Me Replace Request for Service Error Returns 404.
func TestHandleMeReplaceRequest_ServiceError_Returns404(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ReplaceUser", mock.Anything, "user-123",
		mock.AnythingOfType("*scim.SCIMUserPayload"), "", testBaseURL, true).
		Return((*SCIMUser)(nil), &ErrorUserNotFound)

	body := `{"schemas":["urn:thunderid:params:scim:schemas:person:2.0:User"],` +
		`"urn:thunderid:params:scim:schemas:person:2.0:User":{}}`
	h := newSCIMUsersHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodPut, "/scim/v2/Me", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	authCtx := security.NewSecurityContextForTest("user-123", "", "", nil, nil)
	req = req.WithContext(security.WithSecurityContextTest(req.Context(), authCtx))
	rr := httptest.NewRecorder()

	h.HandleMeReplaceRequest(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
}

// --- GET /scim/v2/Users/{id} error paths ---

// TestHandleUsersGetRequest_MissingID_Returns404 tests Handle Users Get Request for Missing ID Returns 404.
func TestHandleUsersGetRequest_MissingID_Returns404(t *testing.T) {
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users/", nil)
	rr := httptest.NewRecorder()

	h.HandleUsersGetRequest(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
}

// TestHandleUsersGetRequest_ServerError_Returns500 tests Handle Users Get Request for Server Error Returns 500.
func TestHandleUsersGetRequest_ServerError_Returns500(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("GetUser", mock.Anything, "user-123", testBaseURL).
		Return((*SCIMUser)(nil), &ErrorInternalServer)

	h := newSCIMUsersHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users/user-123", nil)
	req.SetPathValue("id", "user-123")
	rr := httptest.NewRecorder()

	h.HandleUsersGetRequest(rr, req)

	require.Equal(t, http.StatusInternalServerError, rr.Code)
	var errResp SCIMErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Empty(t, errResp.ScimType)
}

// --- DELETE /scim/v2/Users/{id} error paths ---

// TestHandleUsersDeleteRequest_MissingID_Returns404 tests Handle Users Delete Request for Missing ID Returns 404.
func TestHandleUsersDeleteRequest_MissingID_Returns404(t *testing.T) {
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testBaseURL)
	req := httptest.NewRequest(http.MethodDelete, "/scim/v2/Users/", nil)
	rr := httptest.NewRecorder()

	h.HandleUsersDeleteRequest(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
}

// TestHandleUsersDeleteRequest_NotFound_Returns404 tests Handle Users Delete Request for Not Found Returns 404.
func TestHandleUsersDeleteRequest_NotFound_Returns404(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("DeleteUser", mock.Anything, "no-such", "").Return(&ErrorUserNotFound)

	h := newSCIMUsersHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodDelete, "/scim/v2/Users/no-such", nil)
	req.SetPathValue("id", "no-such")
	rr := httptest.NewRecorder()

	h.HandleUsersDeleteRequest(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
}

// TestHandleUsersDeleteRequest_MutabilityViolation_Returns400 tests Handle Users Delete Request for
// Mutability Violation Returns 400.
func TestHandleUsersDeleteRequest_MutabilityViolation_Returns400(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("DeleteUser", mock.Anything, "readonly", "").Return(&ErrorMutabilityViolation)

	h := newSCIMUsersHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodDelete, "/scim/v2/Users/readonly", nil)
	req.SetPathValue("id", "readonly")
	rr := httptest.NewRecorder()

	h.HandleUsersDeleteRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	var errResp SCIMErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, scimErrorTypeInvalidValue, errResp.ScimType)
}

// --- POST /scim/v2/Users error paths ---

// TestHandleUsersCreateRequest_WrongContentType_Returns400 tests Handle Users Create Request for Wrong
// Content Type Returns 400.
func TestHandleUsersCreateRequest_WrongContentType_Returns400(t *testing.T) {
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testBaseURL)
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users",
		bytes.NewBufferString(`{"schemas":[]}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.HandleUsersCreateRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	var errResp SCIMErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, "invalidSyntax", errResp.ScimType)
}

// TestHandleUsersCreateRequest_EmptyBody_Returns400 tests Handle Users Create Request for Empty Body Returns 400.
func TestHandleUsersCreateRequest_EmptyBody_Returns400(t *testing.T) {
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testBaseURL)
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users", http.NoBody)
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersCreateRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestHandleUsersCreateRequest_InvalidJSON_Returns400 tests Handle Users Create Request for Invalid JSON Returns 400.
func TestHandleUsersCreateRequest_InvalidJSON_Returns400(t *testing.T) {
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testBaseURL)
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users",
		bytes.NewBufferString(`not json`))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersCreateRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestHandleUsersCreateRequest_UniquenessConflict_Returns409 tests Handle Users Create Request for Uniqueness
// Conflict Returns 409.
func TestHandleUsersCreateRequest_UniquenessConflict_Returns409(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("CreateUser", mock.Anything,
		mock.AnythingOfType("*scim.SCIMUserPayload"), testBaseURL).
		Return((*SCIMUser)(nil), &ErrorUniquenessConflict)

	body := `{"schemas":["urn:thunderid:params:scim:schemas:person:2.0:User"],` +
		`"urn:thunderid:params:scim:schemas:person:2.0:User":{"email":"x@x.com"}}`
	h := newSCIMUsersHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users",
		bytes.NewBufferString(body))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersCreateRequest(rr, req)

	require.Equal(t, http.StatusConflict, rr.Code)
	var errResp SCIMErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, "uniqueness", errResp.ScimType)
}

// TestHandleUsersCreateRequest_SchemaValidationFailed_Returns400 tests Handle Users Create Request for Schema
// Validation Failed Returns 400.
func TestHandleUsersCreateRequest_SchemaValidationFailed_Returns400(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("CreateUser", mock.Anything,
		mock.AnythingOfType("*scim.SCIMUserPayload"), testBaseURL).
		Return((*SCIMUser)(nil), &ErrorSchemaValidationFailed)

	body := `{"schemas":["urn:thunderid:params:scim:schemas:person:2.0:User"],` +
		`"urn:thunderid:params:scim:schemas:person:2.0:User":{}}`
	h := newSCIMUsersHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users",
		bytes.NewBufferString(body))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersCreateRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	var errResp SCIMErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, scimErrorTypeInvalidValue, errResp.ScimType)
}

// TestHandleUsersCreateRequest_ServerError_Returns500 tests Handle Users Create Request for Server Error Returns 500.
func TestHandleUsersCreateRequest_ServerError_Returns500(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("CreateUser", mock.Anything,
		mock.AnythingOfType("*scim.SCIMUserPayload"), testBaseURL).
		Return((*SCIMUser)(nil), &ErrorInternalServer)

	body := `{"schemas":["urn:thunderid:params:scim:schemas:person:2.0:User"],` +
		`"urn:thunderid:params:scim:schemas:person:2.0:User":{}}`
	h := newSCIMUsersHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users",
		bytes.NewBufferString(body))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersCreateRequest(rr, req)

	require.Equal(t, http.StatusInternalServerError, rr.Code)
}

// --- PUT /scim/v2/Users/{id} error paths ---

// TestHandleUsersReplaceRequest_MissingID_Returns404 tests Handle Users Replace Request for Missing ID Returns 404.
func TestHandleUsersReplaceRequest_MissingID_Returns404(t *testing.T) {
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testBaseURL)
	req := httptest.NewRequest(http.MethodPut, "/scim/v2/Users/", http.NoBody)
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersReplaceRequest(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
}

// TestHandleUsersReplaceRequest_WrongContentType_Returns400 tests Handle Users Replace Request for Wrong
// Content Type Returns 400.
func TestHandleUsersReplaceRequest_WrongContentType_Returns400(t *testing.T) {
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testBaseURL)
	req := httptest.NewRequest(http.MethodPut, "/scim/v2/Users/user-123",
		bytes.NewBufferString(`{}`))
	req.SetPathValue("id", "user-123")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.HandleUsersReplaceRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestHandleUsersReplaceRequest_EmptyBody_Returns400 tests Handle Users Replace Request for Empty Body Returns 400.
func TestHandleUsersReplaceRequest_EmptyBody_Returns400(t *testing.T) {
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testBaseURL)
	req := httptest.NewRequest(http.MethodPut, "/scim/v2/Users/user-123", http.NoBody)
	req.SetPathValue("id", "user-123")
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersReplaceRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestHandleUsersReplaceRequest_InvalidJSON_Returns400 tests Handle Users Replace Request for Invalid JSON Returns 400.
func TestHandleUsersReplaceRequest_InvalidJSON_Returns400(t *testing.T) {
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testBaseURL)
	req := httptest.NewRequest(http.MethodPut, "/scim/v2/Users/user-123",
		bytes.NewBufferString(`not json`))
	req.SetPathValue("id", "user-123")
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersReplaceRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestHandleUsersReplaceRequest_NotFound_Returns404 tests Handle Users Replace Request for Not Found Returns 404.
func TestHandleUsersReplaceRequest_NotFound_Returns404(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ReplaceUser", mock.Anything, "no-such",
		mock.AnythingOfType("*scim.SCIMUserPayload"), "", testBaseURL, false).
		Return((*SCIMUser)(nil), &ErrorUserNotFound)

	body := `{"schemas":["urn:thunderid:params:scim:schemas:person:2.0:User"],` +
		`"urn:thunderid:params:scim:schemas:person:2.0:User":{}}`
	h := newSCIMUsersHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodPut, "/scim/v2/Users/no-such",
		bytes.NewBufferString(body))
	req.SetPathValue("id", "no-such")
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersReplaceRequest(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
}

// TestHandleUsersReplaceRequest_MutabilityViolation_Returns400 tests Handle Users Replace Request for
// Mutability Violation Returns 400.
func TestHandleUsersReplaceRequest_MutabilityViolation_Returns400(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ReplaceUser", mock.Anything, "readonly",
		mock.AnythingOfType("*scim.SCIMUserPayload"), "", testBaseURL, false).
		Return((*SCIMUser)(nil), &ErrorMutabilityViolation)

	body := `{"schemas":["urn:thunderid:params:scim:schemas:person:2.0:User"],` +
		`"urn:thunderid:params:scim:schemas:person:2.0:User":{}}`
	h := newSCIMUsersHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodPut, "/scim/v2/Users/readonly",
		bytes.NewBufferString(body))
	req.SetPathValue("id", "readonly")
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersReplaceRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	var errResp SCIMErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, scimErrorTypeInvalidValue, errResp.ScimType)
}

// --- GET /scim/v2/Users list error paths ---

// TestHandleUsersListRequest_FilterNotSupported_Returns400 tests Handle Users List Request for Filter Not
// Supported Returns 400.
func TestHandleUsersListRequest_FilterNotSupported_Returns400(t *testing.T) {
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testBaseURL)
	// "or" expressions are unsupported; only "eq" clauses joined by "and" are allowed.
	req := httptest.NewRequest(http.MethodGet,
		`/scim/v2/Users?filter=userName+eq+"alice"+or+active+eq+true`, nil)
	rr := httptest.NewRecorder()

	h.HandleUsersListRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	var errResp SCIMErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, "invalidFilter", errResp.ScimType)
}

// TestHandleUsersListRequest_FilterOnMultiValueUnsupportedSubAttr_Returns400 tests Handle Users List Request
// for Filter On Multi Value Unsupported Sub Attr Returns 400.
func TestHandleUsersListRequest_FilterOnMultiValueUnsupportedSubAttr_Returns400(t *testing.T) {
	// "emails.type"/"emails.primary" have no matching flat ThunderID attribute
	// — ThunderID only stores the value, never a per-entry type or primary
	// flag — so they're rejected explicitly instead of silently matching
	// nothing.
	tests := []string{
		`emails.type+eq+"work"`,
		`emails.primary+eq+true`,
	}
	for _, filter := range tests {
		t.Run(filter, func(t *testing.T) {
			h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testBaseURL)
			req := httptest.NewRequest(http.MethodGet,
				"/scim/v2/Users?filter="+filter, nil)
			rr := httptest.NewRecorder()

			h.HandleUsersListRequest(rr, req)

			require.Equal(t, http.StatusBadRequest, rr.Code)
			var errResp SCIMErrorResponse
			require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
			require.Equal(t, "invalidFilter", errResp.ScimType)
		})
	}
}

// TestHandleUsersListRequest_FilterOnHyphenatedAttr_Returns400 tests Handle Users List Request for Filter On
// Hyphenated Attr Returns 400.
func TestHandleUsersListRequest_FilterOnHyphenatedAttr_Returns400(t *testing.T) {
	// "-" is valid in an attrPath per RFC 7643 but rejected by the store-layer
	// key charset; must be caught here as invalidFilter, not surface as a 500.
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testBaseURL)
	req := httptest.NewRequest(http.MethodGet,
		"/scim/v2/Users?filter="+neturl.QueryEscape(`custom-attr eq "x"`), nil)
	rr := httptest.NewRecorder()

	h.HandleUsersListRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	var errResp SCIMErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, "invalidFilter", errResp.ScimType)
}

// TestHandleUsersListRequest_FilterTranslatesCoreAttributes tests Handle Users List Request for Filter
// Translates Core Attributes.
func TestHandleUsersListRequest_FilterTranslatesCoreAttributes(t *testing.T) {
	tests := []struct {
		name           string
		filter         string
		expectedFilter map[string]interface{}
	}{
		{
			name:           "simple string attribute",
			filter:         `userName eq "alice"`,
			expectedFilter: map[string]interface{}{"username": "alice"},
		},
		{
			name:           "sub-attribute of complex object",
			filter:         `name.givenName eq "Alice"`,
			expectedFilter: map[string]interface{}{"given_name": "Alice"},
		},
		{
			name:           "multi-valued complex attribute value",
			filter:         `emails.value eq "alice@example.com"`,
			expectedFilter: map[string]interface{}{"email": "alice@example.com"},
		},
		{
			name:           "address sub-attribute",
			filter:         `addresses.streetAddress eq "Main St"`,
			expectedFilter: map[string]interface{}{"street_address": "Main St"},
		},
		{
			name:           "unmapped attribute passes through unchanged",
			filter:         `active eq true`,
			expectedFilter: map[string]interface{}{"active": true},
		},
		{
			name:           "URN-prefixed attribute with numeric version segment",
			filter:         `urn:thunderid:params:scim:schemas:employee_hier:2.0:User:active eq true`,
			expectedFilter: map[string]interface{}{"active": true},
		},
		{
			name:           "compound AND with two clauses",
			filter:         `userName eq "alice" and active eq true`,
			expectedFilter: map[string]interface{}{"username": "alice", "active": true},
		},
		{
			name:           "compound AND with case-insensitive keyword",
			filter:         `userName eq "alice" AND active eq true`,
			expectedFilter: map[string]interface{}{"username": "alice", "active": true},
		},
		{
			name:   "compound AND with three clauses",
			filter: `userName eq "alice" and active eq true and name.givenName eq "Alice"`,
			expectedFilter: map[string]interface{}{
				"username": "alice", "active": true, "given_name": "Alice",
			},
		},
		{
			name:           "quoted value containing literal 'and' text is not split",
			filter:         `active eq "call and response"`,
			expectedFilter: map[string]interface{}{"active": "call and response"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := NewSCIMUsersServiceInterfaceMock(t)
			mockSvc.On("ListUsers", mock.Anything, 1, constants.DefaultPageSize, tt.expectedFilter, testBaseURL).
				Return(SCIMUserListResponse{}, (*tidcommon.ServiceError)(nil))

			h := newSCIMUsersHandler(mockSvc, testBaseURL)
			req := httptest.NewRequest(http.MethodGet,
				"/scim/v2/Users?filter="+neturl.QueryEscape(tt.filter), nil)
			rr := httptest.NewRecorder()

			h.HandleUsersListRequest(rr, req)

			require.Equal(t, http.StatusOK, rr.Code)
		})
	}
}

// TestHandleUsersListRequest_ServiceError_Returns500 tests Handle Users List Request for Service Error Returns 500.
func TestHandleUsersListRequest_ServiceError_Returns500(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ListUsers", mock.Anything, 1, constants.DefaultPageSize, mock.Anything, testBaseURL).
		Return(SCIMUserListResponse{}, &ErrorInternalServer)

	h := newSCIMUsersHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users", nil)
	rr := httptest.NewRecorder()

	h.HandleUsersListRequest(rr, req)

	require.Equal(t, http.StatusInternalServerError, rr.Code)
}

// TestHandleUsersListRequest_CustomPagination tests Handle Users List Request for Custom Pagination.
func TestHandleUsersListRequest_CustomPagination(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ListUsers", mock.Anything, 5, 10, mock.Anything, testBaseURL).
		Return(SCIMUserListResponse{
			Schemas:      []string{SCIMListResponseSchemaURN},
			TotalResults: 0,
			StartIndex:   5,
			ItemsPerPage: 10,
			Resources:    []SCIMUser{},
		}, (*tidcommon.ServiceError)(nil))

	h := newSCIMUsersHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users?startIndex=5&count=10", nil)
	rr := httptest.NewRecorder()

	h.HandleUsersListRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
}

// TestHandleUsersListRequest_CapsMaxCount tests Handle Users List Request for Caps Max Count.
func TestHandleUsersListRequest_CapsMaxCount(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ListUsers", mock.Anything, 1, scimconfig.FilterMaxResults, mock.Anything, testBaseURL).
		Return(SCIMUserListResponse{}, (*tidcommon.ServiceError)(nil))

	h := newSCIMUsersHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users?count=100000", nil)
	rr := httptest.NewRecorder()

	h.HandleUsersListRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
}

// TestHandleUsersListRequest_SortNotSupported_Returns400 tests Handle Users List Request for Sort Not
// Supported Returns 400.
func TestHandleUsersListRequest_SortNotSupported_Returns400(t *testing.T) {
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users?sortBy=userName", nil)
	rr := httptest.NewRecorder()

	h.HandleUsersListRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	var errResp SCIMErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, scimErrorTypeInvalidValue, errResp.ScimType)
}

// TestHandleUsersSearchRequest_Success tests Handle Users Search Request for Success.
func TestHandleUsersSearchRequest_Success(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	expectedResp := SCIMUserListResponse{
		Schemas:      []string{SCIMListResponseSchemaURN},
		TotalResults: 1,
		StartIndex:   1,
		ItemsPerPage: constants.DefaultPageSize,
		Resources:    []SCIMUser{{ID: "user-123"}},
	}
	mockSvc.On("ListUsers", mock.Anything, 1, constants.DefaultPageSize,
		map[string]interface{}{"username": "alice"}, testBaseURL).
		Return(expectedResp, (*tidcommon.ServiceError)(nil))

	body := `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:SearchRequest"],
		"filter": "userName eq \"alice\""
	}`
	h := newSCIMUsersHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users/.search", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersSearchRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var got SCIMUserListResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	require.Equal(t, 1, got.TotalResults)
}

// TestHandleUsersSearchRequest_NoFilter_DefaultsPagination tests Handle Users Search Request for No Filter
// Defaults Pagination.
func TestHandleUsersSearchRequest_NoFilter_DefaultsPagination(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ListUsers", mock.Anything, 1, constants.DefaultPageSize, mock.Anything, testBaseURL).
		Return(SCIMUserListResponse{}, (*tidcommon.ServiceError)(nil))

	body := `{"schemas": ["urn:ietf:params:scim:api:messages:2.0:SearchRequest"]}`
	h := newSCIMUsersHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users/.search", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersSearchRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
}

// TestHandleUsersSearchRequest_CustomPagination tests Handle Users Search Request for Custom Pagination.
func TestHandleUsersSearchRequest_CustomPagination(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ListUsers", mock.Anything, 5, 10, mock.Anything, testBaseURL).
		Return(SCIMUserListResponse{
			Schemas: []string{SCIMListResponseSchemaURN}, StartIndex: 5, ItemsPerPage: 10,
			Resources: []SCIMUser{},
		}, (*tidcommon.ServiceError)(nil))

	body := `{"schemas": ["` + SCIMSearchSchemaURN + `"], "startIndex": 5, "count": 10}`
	h := newSCIMUsersHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users/.search", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersSearchRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
}

// TestHandleUsersSearchRequest_ExplicitZeroCount tests Handle Users Search Request for Explicit Zero Count.
func TestHandleUsersSearchRequest_ExplicitZeroCount(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ListUsers", mock.Anything, 1, 0, mock.Anything, testBaseURL).
		Return(SCIMUserListResponse{
			Schemas: []string{SCIMListResponseSchemaURN}, StartIndex: 1, ItemsPerPage: 0, TotalResults: 5,
			Resources: []SCIMUser{},
		}, (*tidcommon.ServiceError)(nil))

	body := `{"schemas": ["` + SCIMSearchSchemaURN + `"], "count": 0}`
	h := newSCIMUsersHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users/.search", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersSearchRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
}

// TestHandleUsersSearchRequest_CapsMaxCount tests Handle Users Search Request for Caps Max Count.
func TestHandleUsersSearchRequest_CapsMaxCount(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ListUsers", mock.Anything, 1, scimconfig.FilterMaxResults, mock.Anything, testBaseURL).
		Return(SCIMUserListResponse{}, (*tidcommon.ServiceError)(nil))

	body := `{"schemas": ["` + SCIMSearchSchemaURN + `"], "count": 100000}`
	h := newSCIMUsersHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users/.search", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersSearchRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
}

// TestHandleUsersSearchRequest_SortNotSupported_Returns400 tests Handle Users Search Request for Sort Not
// Supported Returns 400.
func TestHandleUsersSearchRequest_SortNotSupported_Returns400(t *testing.T) {
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testBaseURL)

	body := `{"schemas": ["` + SCIMSearchSchemaURN + `"], "sortBy": "userName"}`
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users/.search", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersSearchRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	var errResp SCIMErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, scimErrorTypeInvalidValue, errResp.ScimType)
}

// TestHandleUsersSearchRequest_InvalidFilter_Returns400 tests Handle Users Search Request for Invalid Filter
// Returns 400.
func TestHandleUsersSearchRequest_InvalidFilter_Returns400(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	h := newSCIMUsersHandler(mockSvc, testBaseURL)

	body := `{"schemas": ["` + SCIMSearchSchemaURN + `"], "filter": "userName co \"ali\""}`
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users/.search", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersSearchRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	var errResp SCIMErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, "invalidFilter", errResp.ScimType)
}

// TestHandleUsersSearchRequest_MalformedJSON_Returns400 tests Handle Users Search Request for Malformed JSON
// Returns 400.
func TestHandleUsersSearchRequest_MalformedJSON_Returns400(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	h := newSCIMUsersHandler(mockSvc, testBaseURL)

	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users/.search", bytes.NewBufferString(`{not-json`))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersSearchRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestHandleUsersSearchRequest_EmptyBody_Returns400 tests Handle Users Search Request for Empty Body Returns 400.
func TestHandleUsersSearchRequest_EmptyBody_Returns400(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	h := newSCIMUsersHandler(mockSvc, testBaseURL)

	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users/.search", bytes.NewBufferString(``))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersSearchRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestHandleUsersSearchRequest_WrongContentType_Returns400 tests Handle Users Search Request for Wrong
// Content Type Returns 400.
func TestHandleUsersSearchRequest_WrongContentType_Returns400(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	h := newSCIMUsersHandler(mockSvc, testBaseURL)

	body := `{"filter": "userName eq \"alice\""}`
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users/.search", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()

	h.HandleUsersSearchRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestHandleUsersSearchRequest_ServiceError_Returns500 tests Handle Users Search Request for Service Error Returns 500.
func TestHandleUsersSearchRequest_ServiceError_Returns500(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ListUsers", mock.Anything, 1, constants.DefaultPageSize, mock.Anything, testBaseURL).
		Return(SCIMUserListResponse{}, &ErrorInternalServer)

	body := `{"schemas": ["` + SCIMSearchSchemaURN + `"]}`
	h := newSCIMUsersHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users/.search", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersSearchRequest(rr, req)

	require.Equal(t, http.StatusInternalServerError, rr.Code)
}

// TestHandleUsersReplaceRequest_PreconditionFailed tests Handle Users Replace Request for Precondition Failed.
func TestHandleUsersReplaceRequest_PreconditionFailed(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ReplaceUser", mock.Anything, "user-123",
		mock.AnythingOfType("*scim.SCIMUserPayload"), `W/"stale"`, testBaseURL, false).
		Return((*SCIMUser)(nil), &ErrorPreconditionFailed)

	h := newSCIMUsersHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodPut, "/scim/v2/Users/user-123", bytes.NewBufferString(scimTestPayloadBody))
	req.SetPathValue("id", "user-123")
	req.Header.Set("Content-Type", constants.SCIMContentType)
	req.Header.Set("If-Match", `W/"stale"`)
	rr := httptest.NewRecorder()

	h.HandleUsersReplaceRequest(rr, req)

	require.Equal(t, http.StatusPreconditionFailed, rr.Code)
}

// TestHandleUsersDeleteRequest_PreconditionFailed tests Handle Users Delete Request for Precondition Failed.
func TestHandleUsersDeleteRequest_PreconditionFailed(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("DeleteUser", mock.Anything, "user-123", `W/"stale"`).
		Return(&ErrorPreconditionFailed)

	h := newSCIMUsersHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodDelete, "/scim/v2/Users/user-123", nil)
	req.Header.Set("If-Match", `W/"stale"`)
	req.SetPathValue("id", "user-123")
	rr := httptest.NewRecorder()

	h.HandleUsersDeleteRequest(rr, req)

	require.Equal(t, http.StatusPreconditionFailed, rr.Code)
}

// TestParseCSVQueryParam tests Parse CSV Query Param.
func TestParseCSVQueryParam(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"empty string returns nil", "", nil},
		{"single value", "userName", []string{"userName"}},
		{"multiple values", "userName,emails", []string{"userName", "emails"}},
		{"trims whitespace", " userName , emails ", []string{"userName", "emails"}},
		{"drops empty entries", "userName,,emails,", []string{"userName", "emails"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, parseCSVQueryParam(tt.input))
		})
	}
}

// TestHandleUsersListRequest_AttributesProjection tests Handle Users List Request for Attributes Projection.
func TestHandleUsersListRequest_AttributesProjection(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	listResp := SCIMUserListResponse{
		Schemas:      []string{SCIMListResponseSchemaURN},
		TotalResults: 1,
		StartIndex:   1,
		ItemsPerPage: constants.DefaultPageSize,
		Resources: []SCIMUser{
			{
				ID:      "user-123",
				Schemas: []string{SCIMCoreUserSchemaURN},
				Meta:    SCIMMeta{ResourceType: "User"},
				CoreAttrs: map[string]json.RawMessage{
					"userName": json.RawMessage(`"alice"`),
					"active":   json.RawMessage(`true`),
				},
			},
		},
	}
	mockSvc.On("ListUsers", mock.Anything, 1, constants.DefaultPageSize, mock.Anything, testBaseURL).
		Return(listResp, (*tidcommon.ServiceError)(nil))

	h := newSCIMUsersHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users?attributes=userName", nil)
	rr := httptest.NewRecorder()

	h.HandleUsersListRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var got map[string]interface{}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	resources, ok := got["Resources"].([]interface{})
	require.True(t, ok)
	require.Len(t, resources, 1)
	resource, ok := resources[0].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "alice", resource["userName"])
	require.NotContains(t, resource, "active")
}

// TestHandleUsersSearchRequest_ExcludedAttributesProjection tests Handle Users Search Request for Excluded
// Attributes Projection.
func TestHandleUsersSearchRequest_ExcludedAttributesProjection(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	listResp := SCIMUserListResponse{
		Schemas:      []string{SCIMListResponseSchemaURN},
		TotalResults: 1,
		StartIndex:   1,
		ItemsPerPage: constants.DefaultPageSize,
		Resources: []SCIMUser{
			{
				ID:      "user-123",
				Schemas: []string{SCIMCoreUserSchemaURN},
				Meta:    SCIMMeta{ResourceType: "User"},
				CoreAttrs: map[string]json.RawMessage{
					"userName": json.RawMessage(`"alice"`),
					"active":   json.RawMessage(`true`),
				},
			},
		},
	}
	mockSvc.On("ListUsers", mock.Anything, 1, constants.DefaultPageSize, mock.Anything, testBaseURL).
		Return(listResp, (*tidcommon.ServiceError)(nil))

	body := `{"schemas": ["` + SCIMSearchSchemaURN + `"], "excludedAttributes": ["active"]}`
	h := newSCIMUsersHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users/.search", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersSearchRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var got map[string]interface{}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	resources := got["Resources"].([]interface{})
	resource := resources[0].(map[string]interface{})
	require.Equal(t, "alice", resource["userName"])
	require.NotContains(t, resource, "active")
}

// --- RFC 7644 §3.9 attributes/excludedAttributes mutual exclusivity ---

// TestHandleUsersListRequest_ConflictingAttributesParams_Returns400 tests Handle Users List Request for
// Conflicting Attributes Params Returns 400.
func TestHandleUsersListRequest_ConflictingAttributesParams_Returns400(t *testing.T) {
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users?attributes=userName&excludedAttributes=active", nil)
	rr := httptest.NewRecorder()

	h.HandleUsersListRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestHandleUsersSearchRequest_ConflictingAttributesParams_Returns400 tests Handle Users Search Request for
// Conflicting Attributes Params Returns 400.
func TestHandleUsersSearchRequest_ConflictingAttributesParams_Returns400(t *testing.T) {
	body := `{"schemas": ["` + SCIMSearchSchemaURN + `"], "attributes": ["userName"], "excludedAttributes": ["active"]}`
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testBaseURL)
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users/.search", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersSearchRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestHandleUsersGetRequest_ConflictingAttributesParams_Returns400 tests Handle Users Get Request for
// Conflicting Attributes Params Returns 400.
func TestHandleUsersGetRequest_ConflictingAttributesParams_Returns400(t *testing.T) {
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testBaseURL)
	req := httptest.NewRequest(
		http.MethodGet, "/scim/v2/Users/user-123?attributes=userName&excludedAttributes=active", nil)
	req.SetPathValue("id", "user-123")
	rr := httptest.NewRecorder()

	h.HandleUsersGetRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestHandleUsersCreateRequest_ConflictingAttributesParams_Returns400 tests Handle Users Create Request for
// Conflicting Attributes Params Returns 400.
func TestHandleUsersCreateRequest_ConflictingAttributesParams_Returns400(t *testing.T) {
	payloadBody := scimTestPayloadBody
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testBaseURL)
	req := httptest.NewRequest(
		http.MethodPost, "/scim/v2/Users?attributes=userName&excludedAttributes=active",
		bytes.NewBufferString(payloadBody))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersCreateRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestHandleUsersReplaceRequest_ConflictingAttributesParams_Returns400 tests Handle Users Replace Request for
// Conflicting Attributes Params Returns 400.
func TestHandleUsersReplaceRequest_ConflictingAttributesParams_Returns400(t *testing.T) {
	payloadBody := scimTestPayloadBody
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testBaseURL)
	req := httptest.NewRequest(
		http.MethodPut, "/scim/v2/Users/user-123?attributes=userName&excludedAttributes=active",
		bytes.NewBufferString(payloadBody))
	req.SetPathValue("id", "user-123")
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersReplaceRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// --- RFC 7644 §3.9 projection wired into single-resource responses ---

// TestHandleUsersGetRequest_AppliesAttributeProjection tests Handle Users Get Request for Applies Attribute Projection.
func TestHandleUsersGetRequest_AppliesAttributeProjection(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	expectedUser := &SCIMUser{
		ID:      "user-123",
		Schemas: []string{SCIMCoreUserSchemaURN},
		Meta:    SCIMMeta{ResourceType: "User", Version: `W/"abc12345"`},
		CoreAttrs: map[string]json.RawMessage{
			"userName": json.RawMessage(`"alice"`),
			"active":   json.RawMessage(`true`),
		},
	}
	mockSvc.On("GetUser", mock.Anything, "user-123", testBaseURL).Return(expectedUser, (*tidcommon.ServiceError)(nil))

	h := newSCIMUsersHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users/user-123?attributes=userName", nil)
	req.SetPathValue("id", "user-123")
	rr := httptest.NewRecorder()

	h.HandleUsersGetRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var got map[string]interface{}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	require.Equal(t, "alice", got["userName"])
	require.NotContains(t, got, "active")
}

// TestHandleUsersCreateRequest_AppliesAttributeProjection tests Handle Users Create Request for Applies
// Attribute Projection.
func TestHandleUsersCreateRequest_AppliesAttributeProjection(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	payloadBody := scimTestPayloadBody
	expectedUser := &SCIMUser{
		ID:      "user-123",
		Schemas: []string{SCIMCoreUserSchemaURN},
		Meta:    SCIMMeta{ResourceType: "User", Version: `W/"abc12345"`},
		CoreAttrs: map[string]json.RawMessage{
			"userName": json.RawMessage(`"alice"`),
			"active":   json.RawMessage(`true`),
		},
	}
	mockSvc.On(
		"CreateUser", mock.Anything, mock.AnythingOfType("*scim.SCIMUserPayload"), testBaseURL,
	).Return(expectedUser, (*tidcommon.ServiceError)(nil))

	h := newSCIMUsersHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(
		http.MethodPost, "/scim/v2/Users?attributes=userName", bytes.NewBufferString(payloadBody))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersCreateRequest(rr, req)

	require.Equal(t, http.StatusCreated, rr.Code)
	var got map[string]interface{}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	require.Equal(t, "alice", got["userName"])
	require.NotContains(t, got, "active")
}

// TestHandleUsersReplaceRequest_AppliesAttributeProjection tests Handle Users Replace Request for Applies
// Attribute Projection.
func TestHandleUsersReplaceRequest_AppliesAttributeProjection(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	payloadBody := scimTestPayloadBody
	expectedUser := &SCIMUser{
		ID:      "user-123",
		Schemas: []string{SCIMCoreUserSchemaURN},
		Meta:    SCIMMeta{ResourceType: "User", Version: `W/"abc12345"`},
		CoreAttrs: map[string]json.RawMessage{
			"userName": json.RawMessage(`"alice"`),
			"active":   json.RawMessage(`true`),
		},
	}
	mockSvc.On(
		"ReplaceUser", mock.Anything, "user-123", mock.AnythingOfType("*scim.SCIMUserPayload"), "", testBaseURL, false,
	).Return(expectedUser, (*tidcommon.ServiceError)(nil))

	h := newSCIMUsersHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(
		http.MethodPut, "/scim/v2/Users/user-123?excludedAttributes=active", bytes.NewBufferString(payloadBody))
	req.SetPathValue("id", "user-123")
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersReplaceRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var got map[string]interface{}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	require.Equal(t, "alice", got["userName"])
	require.NotContains(t, got, "active")
}

// TestHandleUsers_BodyExceedsLimit tests Handle Users for Body Exceeds Limit.
func TestHandleUsers_BodyExceedsLimit(t *testing.T) {
	h := newSCIMUsersHandler(nil, testBaseURL)

	t.Run("Search", func(t *testing.T) {
		oversizedBody := bytes.NewBuffer(make([]byte, maxRequestBodyBytes+10))
		req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users/.search", oversizedBody)
		req.Header.Set("Content-Type", constants.SCIMContentType)
		rr := httptest.NewRecorder()
		h.HandleUsersSearchRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("Create", func(t *testing.T) {
		oversizedBody := bytes.NewBuffer(make([]byte, maxRequestBodyBytes+10))
		req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users", oversizedBody)
		req.Header.Set("Content-Type", constants.SCIMContentType)
		rr := httptest.NewRecorder()
		h.HandleUsersCreateRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("Replace", func(t *testing.T) {
		oversizedBody := bytes.NewBuffer(make([]byte, maxRequestBodyBytes+10))
		req := httptest.NewRequest(http.MethodPut, "/scim/v2/Users/user-123", oversizedBody)
		req.SetPathValue("id", "user-123")
		req.Header.Set("Content-Type", constants.SCIMContentType)
		rr := httptest.NewRecorder()
		h.HandleUsersReplaceRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("MeReplace", func(t *testing.T) {
		oversizedBody := bytes.NewBuffer(make([]byte, maxRequestBodyBytes+10))
		req := httptest.NewRequest(http.MethodPut, "/scim/v2/Me", oversizedBody)
		req.Header.Set("Content-Type", constants.SCIMContentType)
		authCtx := security.NewSecurityContextForTest("user-123", "", "", nil, nil)
		req = req.WithContext(security.WithSecurityContextTest(req.Context(), authCtx))
		rr := httptest.NewRecorder()
		h.HandleMeReplaceRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})
}
