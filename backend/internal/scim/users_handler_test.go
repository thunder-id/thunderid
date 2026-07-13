package scim

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/constants"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

func TestHandleUsersGetRequest_Success(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	expectedUser := &SCIMUser{
		Schemas: []string{SCIMCoreUserSchemaURN},
		ID:      "user-123",
		Meta: SCIMMeta{
			ResourceType: "User",
			Location:     testBaseURL + "/scim/v2/Users/user-123",
		},
	}
	mockSvc.On("GetUser", mock.Anything, "user-123", testBaseURL).Return(expectedUser, (*tidcommon.ServiceError)(nil))

	h := newSCIMUsersHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users/user-123", nil)
	req.SetPathValue("id", "user-123")
	rr := httptest.NewRecorder()

	h.HandleUsersGetRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, constants.SCIMContentType, rr.Header().Get("Content-Type"))

	var got SCIMUser
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	require.Equal(t, expectedUser.ID, got.ID)
}

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

func TestHandleUsersDeleteRequest_Success(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("DeleteUser", mock.Anything, "user-123").Return((*tidcommon.ServiceError)(nil))

	h := newSCIMUsersHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodDelete, "/scim/v2/Users/user-123", nil)
	req.SetPathValue("id", "user-123")
	rr := httptest.NewRecorder()

	h.HandleUsersDeleteRequest(rr, req)

	require.Equal(t, http.StatusNoContent, rr.Code)
}

func TestHandleUsersCreateRequest_Success(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	payloadBody := `{
		"schemas": ["urn:thunderid:params:scim:schemas:person:2.0:User"],
		"urn:thunderid:params:scim:schemas:person:2.0:User": {
			"given_name": "Test"
		}
	}`

	expectedUser := &SCIMUser{
		Schemas: []string{SCIMCoreUserSchemaURN, "urn:thunderid:params:scim:schemas:person:2.0:User"},
		ID:      "user-123",
		Meta: SCIMMeta{
			ResourceType: "User",
			Location:     testBaseURL + "/scim/v2/Users/user-123",
		},
	}
	mockSvc.On(
		"CreateUser", mock.Anything, mock.AnythingOfType("*scim.SCIMUserPayload"), testBaseURL,
	).Return(expectedUser, (*tidcommon.ServiceError)(nil))

	h := newSCIMUsersHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users", bytes.NewBufferString(payloadBody))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersCreateRequest(rr, req)

	require.Equal(t, http.StatusCreated, rr.Code)
	require.Equal(t, expectedUser.Meta.Location, rr.Header().Get("Location"))
}

func TestHandleUsersListRequest_Success(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	expectedResp := SCIMUserListResponse{
		Schemas:      []string{SCIMListResponseSchemaURN},
		TotalResults: 0,
		StartIndex:   1,
		ItemsPerPage: 20,
		Resources:    []SCIMUser{},
	}
	mockSvc.On("ListUsers", mock.Anything, 1, 20, testBaseURL).Return(expectedResp, (*tidcommon.ServiceError)(nil))

	h := newSCIMUsersHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users", nil)
	rr := httptest.NewRecorder()

	h.HandleUsersListRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
}

func TestHandleUsersReplaceRequest_Success(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	payloadBody := `{
		"schemas": ["urn:thunderid:params:scim:schemas:person:2.0:User"],
		"urn:thunderid:params:scim:schemas:person:2.0:User": {
			"given_name": "Test"
		}
	}`

	expectedUser := &SCIMUser{
		Schemas: []string{SCIMCoreUserSchemaURN, "urn:thunderid:params:scim:schemas:person:2.0:User"},
		ID:      "user-123",
		Meta: SCIMMeta{
			ResourceType: "User",
			Location:     testBaseURL + "/scim/v2/Users/user-123",
		},
	}
	mockSvc.On(
		"ReplaceUser", mock.Anything, "user-123", mock.AnythingOfType("*scim.SCIMUserPayload"), testBaseURL,
	).Return(expectedUser, (*tidcommon.ServiceError)(nil))

	h := newSCIMUsersHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodPut, "/scim/v2/Users/user-123", bytes.NewBufferString(payloadBody))
	req.SetPathValue("id", "user-123")
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersReplaceRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
}

// --- GET /scim/v2/Users/{id} error paths ---

func TestHandleUsersGetRequest_MissingID_Returns404(t *testing.T) {
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users/", nil)
	rr := httptest.NewRecorder()

	h.HandleUsersGetRequest(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
}

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

func TestHandleUsersDeleteRequest_MissingID_Returns404(t *testing.T) {
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testBaseURL)
	req := httptest.NewRequest(http.MethodDelete, "/scim/v2/Users/", nil)
	rr := httptest.NewRecorder()

	h.HandleUsersDeleteRequest(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleUsersDeleteRequest_NotFound_Returns404(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("DeleteUser", mock.Anything, "no-such").Return(&ErrorUserNotFound)

	h := newSCIMUsersHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodDelete, "/scim/v2/Users/no-such", nil)
	req.SetPathValue("id", "no-such")
	rr := httptest.NewRecorder()

	h.HandleUsersDeleteRequest(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleUsersDeleteRequest_MutabilityViolation_Returns400(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("DeleteUser", mock.Anything, "readonly").Return(&ErrorMutabilityViolation)

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

func TestHandleUsersCreateRequest_EmptyBody_Returns400(t *testing.T) {
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testBaseURL)
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users", http.NoBody)
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersCreateRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleUsersCreateRequest_InvalidJSON_Returns400(t *testing.T) {
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testBaseURL)
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users",
		bytes.NewBufferString(`not json`))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersCreateRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

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

func TestHandleUsersReplaceRequest_MissingID_Returns404(t *testing.T) {
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testBaseURL)
	req := httptest.NewRequest(http.MethodPut, "/scim/v2/Users/", http.NoBody)
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersReplaceRequest(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
}

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

func TestHandleUsersReplaceRequest_EmptyBody_Returns400(t *testing.T) {
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testBaseURL)
	req := httptest.NewRequest(http.MethodPut, "/scim/v2/Users/user-123", http.NoBody)
	req.SetPathValue("id", "user-123")
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersReplaceRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleUsersReplaceRequest_NotFound_Returns404(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ReplaceUser", mock.Anything, "no-such",
		mock.AnythingOfType("*scim.SCIMUserPayload"), testBaseURL).
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

func TestHandleUsersReplaceRequest_MutabilityViolation_Returns400(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ReplaceUser", mock.Anything, "readonly",
		mock.AnythingOfType("*scim.SCIMUserPayload"), testBaseURL).
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

func TestHandleUsersListRequest_FilterNotSupported_Returns400(t *testing.T) {
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testBaseURL)
	req := httptest.NewRequest(http.MethodGet, `/scim/v2/Users?filter=userName+eq+"alice"`, nil)
	rr := httptest.NewRecorder()

	h.HandleUsersListRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	var errResp SCIMErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, "invalidFilter", errResp.ScimType)
}

func TestHandleUsersListRequest_ServiceError_Returns500(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ListUsers", mock.Anything, 1, 20, testBaseURL).
		Return(SCIMUserListResponse{}, &ErrorInternalServer)

	h := newSCIMUsersHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users", nil)
	rr := httptest.NewRecorder()

	h.HandleUsersListRequest(rr, req)

	require.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestHandleUsersListRequest_CustomPagination(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ListUsers", mock.Anything, 5, 10, testBaseURL).
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
