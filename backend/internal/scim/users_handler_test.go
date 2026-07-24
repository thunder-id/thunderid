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
	mockSvc.On("DeleteUser", mock.Anything, "user-123", "").Return((*tidcommon.ServiceError)(nil))

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
			Version:      `W/"abc12345"`,
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
	require.Equal(t, expectedUser.Meta.Version, rr.Header().Get("ETag"))
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
	mockSvc.On("ListUsers", mock.Anything, 1, 20, mock.Anything,
		testBaseURL).Return(expectedResp, (*tidcommon.ServiceError)(nil))

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
			Version:      `W/"abc12345"`,
		},
	}
	mockSvc.On(
		"ReplaceUser", mock.Anything, "user-123", mock.AnythingOfType("*scim.SCIMUserPayload"), "", testBaseURL,
	).Return(expectedUser, (*tidcommon.ServiceError)(nil))

	h := newSCIMUsersHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodPut, "/scim/v2/Users/user-123", bytes.NewBufferString(payloadBody))
	req.SetPathValue("id", "user-123")
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersReplaceRequest(rr, req)
	require.Equal(t, expectedUser.Meta.Version, rr.Header().Get("ETag"))
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
	mockSvc.On("DeleteUser", mock.Anything, "no-such", "").Return(&ErrorUserNotFound)

	h := newSCIMUsersHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodDelete, "/scim/v2/Users/no-such", nil)
	req.SetPathValue("id", "no-such")
	rr := httptest.NewRecorder()

	h.HandleUsersDeleteRequest(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
}

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

func TestHandleUsersReplaceRequest_NotFound_Returns404(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ReplaceUser", mock.Anything, "no-such",
		mock.AnythingOfType("*scim.SCIMUserPayload"), "", testBaseURL).
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
		mock.AnythingOfType("*scim.SCIMUserPayload"), "", testBaseURL).
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
	// Compound "and" expressions are unsupported; only single "eq" is allowed.
	req := httptest.NewRequest(http.MethodGet,
		`/scim/v2/Users?filter=userName+eq+"alice"+and+active+eq+true`, nil)
	rr := httptest.NewRecorder()

	h.HandleUsersListRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	var errResp SCIMErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, "invalidFilter", errResp.ScimType)
}

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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := NewSCIMUsersServiceInterfaceMock(t)
			mockSvc.On("ListUsers", mock.Anything, 1, 20, tt.expectedFilter, testBaseURL).
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

func TestHandleUsersListRequest_ServiceError_Returns500(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ListUsers", mock.Anything, 1, 20, mock.Anything, testBaseURL).
		Return(SCIMUserListResponse{}, &ErrorInternalServer)

	h := newSCIMUsersHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users", nil)
	rr := httptest.NewRecorder()

	h.HandleUsersListRequest(rr, req)

	require.Equal(t, http.StatusInternalServerError, rr.Code)
}

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

func TestHandleUsersReplaceRequest_PreconditionFailed(t *testing.T) {
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ReplaceUser", mock.Anything, "user-123",
		mock.AnythingOfType("*scim.SCIMUserPayload"), `W/"stale"`, testBaseURL).
		Return((*SCIMUser)(nil), &ErrorPreconditionFailed)

	payloadBody := `{
        "schemas": ["urn:thunderid:params:scim:schemas:person:2.0:User"],
        "urn:thunderid:params:scim:schemas:person:2.0:User": {"given_name": "Test"}
    }`
	h := newSCIMUsersHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodPut, "/scim/v2/Users/user-123", bytes.NewBufferString(payloadBody))
	req.SetPathValue("id", "user-123")
	req.Header.Set("Content-Type", constants.SCIMContentType)
	req.Header.Set("If-Match", `W/"stale"`)
	rr := httptest.NewRecorder()

	h.HandleUsersReplaceRequest(rr, req)

	require.Equal(t, http.StatusPreconditionFailed, rr.Code)
}

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

// --- parseSCIMFilterForEq direct unit tests ---

func TestParseSCIMFilterForEq_EmptyString_ReturnsNil(t *testing.T) {
	filters, err := parseSCIMFilterForEq("")
	require.NoError(t, err)
	require.Nil(t, filters)
}

func TestParseSCIMFilterForEq_UnsupportedOperator_ReturnsError(t *testing.T) {
	tests := []string{
		`userName ne "alice"`,
		`userName co "ali"`,
		`userName sw "al"`,
		`userName ew "ce"`,
		`userName pr`,
		`age gt 5`,
		`age lt 5`,
		`age ge 5`,
		`age le 5`,
	}
	for _, filter := range tests {
		t.Run(filter, func(t *testing.T) {
			_, err := parseSCIMFilterForEq(filter)
			require.Error(t, err)
		})
	}
}

func TestParseSCIMFilterForEq_MalformedExpression_ReturnsError(t *testing.T) {
	_, err := parseSCIMFilterForEq("userName")
	require.Error(t, err)
}

func TestParseSCIMFilterForEq_InvalidCompValue_ReturnsError(t *testing.T) {
	_, err := parseSCIMFilterForEq("userName eq null")
	require.Error(t, err)
}

// --- parseSCIMCompValue direct unit tests ---

func TestParseSCIMCompValue(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		expected  interface{}
		expectErr bool
	}{
		{name: "quoted string", raw: `"alice"`, expected: "alice"},
		{name: "unterminated quoted string", raw: `"alice`, expectErr: true},
		{name: "true", raw: "true", expected: true},
		{name: "false", raw: "false", expected: false},
		{name: "null", raw: "null", expectErr: true},
		{name: "integer", raw: "42", expected: int64(42)},
		{name: "decimal", raw: "3.14", expected: 3.14},
		{name: "unrecognized", raw: "notavalue", expectErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSCIMCompValue(tt.raw)
			if tt.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expected, got)
		})
	}
}
