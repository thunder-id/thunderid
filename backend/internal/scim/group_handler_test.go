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

func TestHandleGroupsListRequest_Success(t *testing.T) {
	mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
	expectedResp := SCIMGroupListResponse{
		Schemas: []string{SCIMListResponseSchemaURN}, StartIndex: 1, ItemsPerPage: 20, Resources: []SCIMGroup{},
	}
	mockSvc.On("ListGroups", mock.Anything, 1, 20, testBaseURL).
		Return(expectedResp, (*tidcommon.ServiceError)(nil))

	h := newSCIMGroupsHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Groups", nil)
	rr := httptest.NewRecorder()

	h.HandleGroupsListRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
}

func TestHandleGroupsListRequest_FilterNotSupported(t *testing.T) {
	mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
	h := newSCIMGroupsHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Groups?filter=displayName+eq+%22x%22", nil)
	rr := httptest.NewRecorder()

	h.HandleGroupsListRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleGroupsGetRequest_Success(t *testing.T) {
	mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
	expected := &SCIMGroup{ID: "group-1", DisplayName: "Admins",
		Meta: SCIMMeta{ResourceType: "Group", Location: testBaseURL + "/scim/v2/Groups/group-1",
			Version: `W/"abc12345"`}}
	mockSvc.On("GetGroup", mock.Anything, "group-1", testBaseURL).
		Return(expected, (*tidcommon.ServiceError)(nil))

	h := newSCIMGroupsHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Groups/group-1", nil)
	req.SetPathValue("id", "group-1")
	rr := httptest.NewRecorder()

	h.HandleGroupsGetRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, expected.Meta.Version, rr.Header().Get("ETag"))
	var got SCIMGroup
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	require.Equal(t, "Admins", got.DisplayName)
}

// Verifies that ErrorResourceNotFound maps to a 404 Not Found response (RFC 7644 §3.12).
func TestHandleGroupsGetRequest_NotFound(t *testing.T) {
	mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
	mockSvc.On("GetGroup", mock.Anything, "missing", testBaseURL).
		Return((*SCIMGroup)(nil), &ErrorResourceNotFound)

	h := newSCIMGroupsHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Groups/missing", nil)
	req.SetPathValue("id", "missing")
	rr := httptest.NewRecorder()

	h.HandleGroupsGetRequest(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleGroupsGetRequest_MissingID(t *testing.T) {
	mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
	h := newSCIMGroupsHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Groups/", nil)
	rr := httptest.NewRecorder()

	h.HandleGroupsGetRequest(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code) // documents the same gap as above
}

func TestHandleGroupsCreateRequest_Success(t *testing.T) {
	mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
	expected := &SCIMGroup{ID: "group-1", DisplayName: "Engineering",
		Meta: SCIMMeta{Location: testBaseURL + "/scim/v2/Groups/group-1",
			Version: `W/"abc12345"`}}
	mockSvc.On("CreateGroup", mock.Anything, "Engineering", mock.Anything, testBaseURL).
		Return(expected, (*tidcommon.ServiceError)(nil))

	h := newSCIMGroupsHandler(mockSvc, testBaseURL)
	body := `{"schemas":["` + SCIMCoreGroupSchemaURN + `"],"displayName":"Engineering","members":[]}`
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Groups", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleGroupsCreateRequest(rr, req)

	require.Equal(t, http.StatusCreated, rr.Code)
	require.Equal(t, expected.Meta.Version, rr.Header().Get("ETag"))
	require.Equal(t, expected.Meta.Location, rr.Header().Get("Location"))
}

func TestHandleGroupsCreateRequest_MissingDisplayName(t *testing.T) {
	mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
	h := newSCIMGroupsHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Groups", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleGroupsCreateRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleGroupsCreateRequest_WrongContentType(t *testing.T) {
	mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
	h := newSCIMGroupsHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Groups",
		bytes.NewBufferString(`{"displayName":"X"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.HandleGroupsCreateRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleGroupsCreateRequest_EmptyBody(t *testing.T) {
	mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
	h := newSCIMGroupsHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Groups", bytes.NewBufferString(""))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleGroupsCreateRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleGroupsReplaceRequest_Success(t *testing.T) {
	mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
	expected := &SCIMGroup{ID: "group-1", DisplayName: "Renamed", Meta: SCIMMeta{Version: `W/"abc12345"`}}
	mockSvc.On("ReplaceGroup", mock.Anything, "group-1", "Renamed", mock.Anything, "", testBaseURL).
		Return(expected, (*tidcommon.ServiceError)(nil))

	h := newSCIMGroupsHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodPut, "/scim/v2/Groups/group-1",
		bytes.NewBufferString(`{"displayName":"Renamed","members":[]}`))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	req.SetPathValue("id", "group-1")
	rr := httptest.NewRecorder()

	h.HandleGroupsReplaceRequest(rr, req)
	require.Equal(t, expected.Meta.Version, rr.Header().Get("ETag"))
	require.Equal(t, http.StatusOK, rr.Code)
}

func TestHandleGroupsPatchRequest_Success(t *testing.T) {
	mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
	expected := &SCIMGroup{ID: "group-1", DisplayName: "Patched", Meta: SCIMMeta{Version: `W/"abc12345"`}}
	mockSvc.On("PatchGroup", mock.Anything, "group-1", mock.Anything, "", testBaseURL).
		Return(expected, (*tidcommon.ServiceError)(nil))

	h := newSCIMGroupsHandler(mockSvc, testBaseURL)
	body := `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
		"Operations": [{"op": "replace", "path": "displayName", "value": "Patched"}]
	}`
	req := httptest.NewRequest(http.MethodPatch, "/scim/v2/Groups/group-1", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	req.SetPathValue("id", "group-1")
	rr := httptest.NewRecorder()

	h.HandleGroupsPatchRequest(rr, req)

	require.Equal(t, expected.Meta.Version, rr.Header().Get("ETag"))
	require.Equal(t, http.StatusOK, rr.Code)
}

func TestHandleGroupsPatchRequest_InvalidBody(t *testing.T) {
	mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
	h := newSCIMGroupsHandler(mockSvc, testBaseURL)
	body := `{"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"], "Operations": [{"op": "bogus"}]}`
	req := httptest.NewRequest(http.MethodPatch, "/scim/v2/Groups/group-1", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	req.SetPathValue("id", "group-1")
	rr := httptest.NewRecorder()

	h.HandleGroupsPatchRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleGroupsDeleteRequest_Success(t *testing.T) {
	mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
	mockSvc.On("DeleteGroup", mock.Anything, "group-1", "").Return((*tidcommon.ServiceError)(nil))

	h := newSCIMGroupsHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodDelete, "/scim/v2/Groups/group-1", nil)
	req.SetPathValue("id", "group-1")
	rr := httptest.NewRecorder()

	h.HandleGroupsDeleteRequest(rr, req)

	require.Equal(t, http.StatusNoContent, rr.Code)
}

func TestHandleGroupsDeleteRequest_MutabilityViolation(t *testing.T) {
	mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
	mockSvc.On("DeleteGroup", mock.Anything, "group-1", "").Return(&ErrorMutabilityViolation)

	h := newSCIMGroupsHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodDelete, "/scim/v2/Groups/group-1", nil)
	req.SetPathValue("id", "group-1")
	rr := httptest.NewRecorder()

	h.HandleGroupsDeleteRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleGroupsReplaceRequest_ForwardsIfMatchHeader(t *testing.T) {
	mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
	expected := &SCIMGroup{ID: "group-1", DisplayName: "Renamed"}
	mockSvc.On("ReplaceGroup", mock.Anything, "group-1", "Renamed", mock.Anything, `W/"v1"`, testBaseURL).
		Return(expected, (*tidcommon.ServiceError)(nil))

	h := newSCIMGroupsHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodPut, "/scim/v2/Groups/group-1",
		bytes.NewBufferString(`{"displayName":"Renamed","members":[]}`))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	req.Header.Set("If-Match", `W/"v1"`)
	req.SetPathValue("id", "group-1")
	rr := httptest.NewRecorder()

	h.HandleGroupsReplaceRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
}

func TestHandleGroupsReplaceRequest_PreconditionFailed(t *testing.T) {
	mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
	mockSvc.On("ReplaceGroup", mock.Anything, "group-1", "Renamed", mock.Anything, `W/"stale"`, testBaseURL).
		Return((*SCIMGroup)(nil), &ErrorPreconditionFailed)

	h := newSCIMGroupsHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodPut, "/scim/v2/Groups/group-1",
		bytes.NewBufferString(`{"displayName":"Renamed","members":[]}`))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	req.Header.Set("If-Match", `W/"stale"`)
	req.SetPathValue("id", "group-1")
	rr := httptest.NewRecorder()

	h.HandleGroupsReplaceRequest(rr, req)

	require.Equal(t, http.StatusPreconditionFailed, rr.Code)
}

func TestHandleGroupsPatchRequest_PreconditionFailed(t *testing.T) {
	mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
	mockSvc.On("PatchGroup", mock.Anything, "group-1", mock.Anything, `W/"stale"`, testBaseURL).
		Return((*SCIMGroup)(nil), &ErrorPreconditionFailed)

	h := newSCIMGroupsHandler(mockSvc, testBaseURL)
	body := `{
        "schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
        "Operations": [{"op": "replace", "path": "displayName", "value": "Patched"}]
    }`
	req := httptest.NewRequest(http.MethodPatch, "/scim/v2/Groups/group-1", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	req.Header.Set("If-Match", `W/"stale"`)
	req.SetPathValue("id", "group-1")
	rr := httptest.NewRecorder()

	h.HandleGroupsPatchRequest(rr, req)

	require.Equal(t, http.StatusPreconditionFailed, rr.Code)
}

func TestHandleGroupsDeleteRequest_PreconditionFailed(t *testing.T) {
	mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
	mockSvc.On("DeleteGroup", mock.Anything, "group-1", `W/"stale"`).
		Return(&ErrorPreconditionFailed)

	h := newSCIMGroupsHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodDelete, "/scim/v2/Groups/group-1", nil)
	req.Header.Set("If-Match", `W/"stale"`)
	req.SetPathValue("id", "group-1")
	rr := httptest.NewRecorder()

	h.HandleGroupsDeleteRequest(rr, req)

	require.Equal(t, http.StatusPreconditionFailed, rr.Code)
}

func TestHandleGroupsListRequest_CustomParamsAndError(t *testing.T) {
	t.Run("ValidParams", func(t *testing.T) {
		mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
		expectedResp := SCIMGroupListResponse{
			Schemas: []string{SCIMListResponseSchemaURN}, StartIndex: 5, ItemsPerPage: 10, Resources: []SCIMGroup{},
		}
		mockSvc.On("ListGroups", mock.Anything, 5, 10, testBaseURL).
			Return(expectedResp, (*tidcommon.ServiceError)(nil))

		h := newSCIMGroupsHandler(mockSvc, testBaseURL)
		req := httptest.NewRequest(http.MethodGet, "/scim/v2/Groups?startIndex=5&count=10", nil)
		rr := httptest.NewRecorder()

		h.HandleGroupsListRequest(rr, req)
		require.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("InvalidParamsUseDefaults", func(t *testing.T) {
		mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
		expectedResp := SCIMGroupListResponse{
			Schemas: []string{SCIMListResponseSchemaURN}, StartIndex: 1, ItemsPerPage: 20, Resources: []SCIMGroup{},
		}
		mockSvc.On("ListGroups", mock.Anything, 1, 20, testBaseURL).
			Return(expectedResp, (*tidcommon.ServiceError)(nil))

		h := newSCIMGroupsHandler(mockSvc, testBaseURL)
		req := httptest.NewRequest(http.MethodGet, "/scim/v2/Groups?startIndex=abc&count=-5", nil)
		rr := httptest.NewRecorder()

		h.HandleGroupsListRequest(rr, req)
		require.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("ServiceError", func(t *testing.T) {
		mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
		mockSvc.On("ListGroups", mock.Anything, 1, 20, testBaseURL).
			Return(SCIMGroupListResponse{}, &ErrorInternalServer)

		h := newSCIMGroupsHandler(mockSvc, testBaseURL)
		req := httptest.NewRequest(http.MethodGet, "/scim/v2/Groups", nil)
		rr := httptest.NewRecorder()

		h.HandleGroupsListRequest(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestHandleGroupsCreateRequest_ServiceError(t *testing.T) {
	mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
	mockSvc.On("CreateGroup", mock.Anything, "Engineering", mock.Anything, testBaseURL).
		Return((*SCIMGroup)(nil), &ErrorUniquenessConflict)

	h := newSCIMGroupsHandler(mockSvc, testBaseURL)
	body := `{"schemas":["` + SCIMCoreGroupSchemaURN + `"],"displayName":"Engineering","members":[]}`
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Groups", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleGroupsCreateRequest(rr, req)
	require.Equal(t, http.StatusConflict, rr.Code)
}

func TestHandleGroupsReplaceRequest_ErrorScenarios(t *testing.T) {
	t.Run("MissingID", func(t *testing.T) {
		h := newSCIMGroupsHandler(nil, testBaseURL)
		req := httptest.NewRequest(http.MethodPut, "/scim/v2/Groups/", nil)
		rr := httptest.NewRecorder()

		h.HandleGroupsReplaceRequest(rr, req)
		require.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("WrongContentType", func(t *testing.T) {
		h := newSCIMGroupsHandler(nil, testBaseURL)
		req := httptest.NewRequest(http.MethodPut, "/scim/v2/Groups/group-1", bytes.NewBufferString(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.SetPathValue("id", "group-1")
		rr := httptest.NewRecorder()

		h.HandleGroupsReplaceRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("EmptyBody", func(t *testing.T) {
		h := newSCIMGroupsHandler(nil, testBaseURL)
		req := httptest.NewRequest(http.MethodPut, "/scim/v2/Groups/group-1", bytes.NewBufferString(""))
		req.Header.Set("Content-Type", constants.SCIMContentType)
		req.SetPathValue("id", "group-1")
		rr := httptest.NewRecorder()

		h.HandleGroupsReplaceRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		h := newSCIMGroupsHandler(nil, testBaseURL)
		req := httptest.NewRequest(http.MethodPut, "/scim/v2/Groups/group-1", bytes.NewBufferString(`{invalid`))
		req.Header.Set("Content-Type", constants.SCIMContentType)
		req.SetPathValue("id", "group-1")
		rr := httptest.NewRecorder()

		h.HandleGroupsReplaceRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("ServiceError", func(t *testing.T) {
		mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
		mockSvc.On("ReplaceGroup", mock.Anything, "group-1", "Renamed", mock.Anything, "", testBaseURL).
			Return((*SCIMGroup)(nil), &ErrorMutabilityViolation)

		h := newSCIMGroupsHandler(mockSvc, testBaseURL)
		req := httptest.NewRequest(http.MethodPut, "/scim/v2/Groups/group-1",
			bytes.NewBufferString(`{"displayName":"Renamed","members":[]}`))
		req.Header.Set("Content-Type", constants.SCIMContentType)
		req.SetPathValue("id", "group-1")
		rr := httptest.NewRecorder()

		h.HandleGroupsReplaceRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})
}

func TestHandleGroupsPatchRequest_ErrorScenarios(t *testing.T) {
	t.Run("MissingID", func(t *testing.T) {
		h := newSCIMGroupsHandler(nil, testBaseURL)
		req := httptest.NewRequest(http.MethodPatch, "/scim/v2/Groups/", nil)
		rr := httptest.NewRecorder()

		h.HandleGroupsPatchRequest(rr, req)
		require.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("WrongContentType", func(t *testing.T) {
		h := newSCIMGroupsHandler(nil, testBaseURL)
		req := httptest.NewRequest(http.MethodPatch, "/scim/v2/Groups/group-1", bytes.NewBufferString(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.SetPathValue("id", "group-1")
		rr := httptest.NewRecorder()

		h.HandleGroupsPatchRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("EmptyBody", func(t *testing.T) {
		h := newSCIMGroupsHandler(nil, testBaseURL)
		req := httptest.NewRequest(http.MethodPatch, "/scim/v2/Groups/group-1", bytes.NewBufferString(""))
		req.Header.Set("Content-Type", constants.SCIMContentType)
		req.SetPathValue("id", "group-1")
		rr := httptest.NewRecorder()

		h.HandleGroupsPatchRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("ServiceError", func(t *testing.T) {
		mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
		mockSvc.On("PatchGroup", mock.Anything, "group-1", mock.Anything, "", testBaseURL).
			Return((*SCIMGroup)(nil), &ErrorMutabilityViolation)

		h := newSCIMGroupsHandler(mockSvc, testBaseURL)
		body := `{
			"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
			"Operations": [{"op": "replace", "path": "displayName", "value": "Patched"}]
		}`
		req := httptest.NewRequest(http.MethodPatch, "/scim/v2/Groups/group-1", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", constants.SCIMContentType)
		req.SetPathValue("id", "group-1")
		rr := httptest.NewRecorder()

		h.HandleGroupsPatchRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})
}

func TestHandleGroupsDeleteRequest_ErrorScenarios(t *testing.T) {
	t.Run("MissingID", func(t *testing.T) {
		h := newSCIMGroupsHandler(nil, testBaseURL)
		req := httptest.NewRequest(http.MethodDelete, "/scim/v2/Groups/", nil)
		rr := httptest.NewRecorder()

		h.HandleGroupsDeleteRequest(rr, req)
		require.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("ServiceError", func(t *testing.T) {
		mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
		mockSvc.On("DeleteGroup", mock.Anything, "group-1", "").
			Return(&ErrorInternalServer)

		h := newSCIMGroupsHandler(mockSvc, testBaseURL)
		req := httptest.NewRequest(http.MethodDelete, "/scim/v2/Groups/group-1", nil)
		req.SetPathValue("id", "group-1")
		rr := httptest.NewRecorder()

		h.HandleGroupsDeleteRequest(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestGroupsHandler_HandleSCIMError_ServerError(t *testing.T) {
	h := newSCIMGroupsHandler(nil, testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Groups/group-1", nil)
	req.SetPathValue("id", "group-1")
	rr := httptest.NewRecorder()

	svcErr := &tidcommon.ServiceError{
		Type: tidcommon.ServerErrorType,
		ErrorDescription: tidcommon.I18nMessage{
			DefaultValue: "internal server error happened",
		},
	}
	h.handleSCIMError(rr, req, svcErr)

	require.Equal(t, http.StatusInternalServerError, rr.Code)
	var errResp SCIMErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, "500", errResp.Status)
	require.Equal(t, "internal server error happened", errResp.Detail)
}
