// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package scim

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	scimconfig "github.com/thunder-id/thunderid/internal/scim/config"
	"github.com/thunder-id/thunderid/internal/system/constants"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// TestHandleGroupsListRequest_Success tests Handle Groups List Request for Success.
func TestHandleGroupsListRequest_Success(t *testing.T) {
	mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
	expectedResp := SCIMGroupListResponse{
		Schemas: []string{SCIMListResponseSchemaURN}, StartIndex: 1, ItemsPerPage: constants.DefaultPageSize,
		Resources: []SCIMGroup{},
	}
	mockSvc.On("ListGroups", mock.Anything, 1, constants.DefaultPageSize, testBaseURL).
		Return(expectedResp, (*tidcommon.ServiceError)(nil))

	h := newSCIMGroupsHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Groups", nil)
	rr := httptest.NewRecorder()

	h.HandleGroupsListRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
}

// TestHandleGroupsListRequest_FilterNotSupported tests Handle Groups List Request for Filter Not Supported.
func TestHandleGroupsListRequest_FilterNotSupported(t *testing.T) {
	mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
	h := newSCIMGroupsHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Groups?filter=displayName+eq+%22x%22", nil)
	rr := httptest.NewRecorder()

	h.HandleGroupsListRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestHandleGroupsListRequest_CapsMaxCount tests Handle Groups List Request for Caps Max Count.
func TestHandleGroupsListRequest_CapsMaxCount(t *testing.T) {
	mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
	mockSvc.On("ListGroups", mock.Anything, 1, scimconfig.FilterMaxResults, testBaseURL).
		Return(SCIMGroupListResponse{}, (*tidcommon.ServiceError)(nil))

	h := newSCIMGroupsHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Groups?count=100000", nil)
	rr := httptest.NewRecorder()

	h.HandleGroupsListRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
}

// TestHandleGroupsListRequest_SortNotSupported tests Handle Groups List Request for Sort Not Supported.
func TestHandleGroupsListRequest_SortNotSupported(t *testing.T) {
	h := newSCIMGroupsHandler(NewSCIMGroupsServiceInterfaceMock(t), testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Groups?sortBy=displayName", nil)
	rr := httptest.NewRecorder()

	h.HandleGroupsListRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	var errResp SCIMErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, scimErrorTypeInvalidValue, errResp.ScimType)
}

// TestHandleGroupsGetRequest_Success tests Handle Groups Get Request for Success.
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
// TestHandleGroupsGetRequest_NotFound tests Handle Groups Get Request for Not Found.
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

// TestHandleGroupsGetRequest_MissingID tests Handle Groups Get Request for Missing ID.
func TestHandleGroupsGetRequest_MissingID(t *testing.T) {
	mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
	h := newSCIMGroupsHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Groups/", nil)
	rr := httptest.NewRecorder()

	h.HandleGroupsGetRequest(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code) // documents the same gap as above
}

// TestHandleGroupsCreateRequest_Success tests Handle Groups Create Request for Success.
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

// TestHandleGroupsCreateRequest_MissingDisplayName tests Handle Groups Create Request for Missing Display Name.
func TestHandleGroupsCreateRequest_MissingDisplayName(t *testing.T) {
	mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
	h := newSCIMGroupsHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Groups", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleGroupsCreateRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestHandleGroupsCreateRequest_WrongContentType tests Handle Groups Create Request for Wrong Content Type.
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

// TestHandleGroupsCreateRequest_EmptyBody tests Handle Groups Create Request for Empty Body.
func TestHandleGroupsCreateRequest_EmptyBody(t *testing.T) {
	mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
	h := newSCIMGroupsHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Groups", bytes.NewBufferString(""))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleGroupsCreateRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestHandleGroupsReplaceRequest_Success tests Handle Groups Replace Request for Success.
func TestHandleGroupsReplaceRequest_Success(t *testing.T) {
	mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
	expected := &SCIMGroup{ID: "group-1", DisplayName: "Renamed", Meta: SCIMMeta{Version: `W/"abc12345"`}}
	mockSvc.On("ReplaceGroup", mock.Anything, "group-1", "Renamed", mock.Anything, "", testBaseURL).
		Return(expected, (*tidcommon.ServiceError)(nil))

	h := newSCIMGroupsHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodPut, "/scim/v2/Groups/group-1",
		bytes.NewBufferString(`{"schemas":["`+SCIMCoreGroupSchemaURN+`"],"displayName":"Renamed","members":[]}`))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	req.SetPathValue("id", "group-1")
	rr := httptest.NewRecorder()

	h.HandleGroupsReplaceRequest(rr, req)
	require.Equal(t, expected.Meta.Version, rr.Header().Get("ETag"))
	require.Equal(t, http.StatusOK, rr.Code)
}

// TestHandleGroupsPatchRequest_Success tests Handle Groups Patch Request for Success.
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

// TestHandleGroupsPatchRequest_InvalidBody tests Handle Groups Patch Request for Invalid Body.
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

// TestHandleGroupsDeleteRequest_Success tests Handle Groups Delete Request for Success.
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

// TestHandleGroupsDeleteRequest_MutabilityViolation tests Handle Groups Delete Request for Mutability Violation.
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

// TestHandleGroupsReplaceRequest_ForwardsIfMatchHeader tests Handle Groups Replace Request for Forwards If
// Match Header.
func TestHandleGroupsReplaceRequest_ForwardsIfMatchHeader(t *testing.T) {
	mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
	expected := &SCIMGroup{ID: "group-1", DisplayName: "Renamed"}
	mockSvc.On("ReplaceGroup", mock.Anything, "group-1", "Renamed", mock.Anything, `W/"v1"`, testBaseURL).
		Return(expected, (*tidcommon.ServiceError)(nil))

	h := newSCIMGroupsHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodPut, "/scim/v2/Groups/group-1",
		bytes.NewBufferString(`{"schemas":["`+SCIMCoreGroupSchemaURN+`"],"displayName":"Renamed","members":[]}`))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	req.Header.Set("If-Match", `W/"v1"`)
	req.SetPathValue("id", "group-1")
	rr := httptest.NewRecorder()

	h.HandleGroupsReplaceRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
}

// TestHandleGroupsReplaceRequest_PreconditionFailed tests Handle Groups Replace Request for Precondition Failed.
func TestHandleGroupsReplaceRequest_PreconditionFailed(t *testing.T) {
	mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
	mockSvc.On("ReplaceGroup", mock.Anything, "group-1", "Renamed", mock.Anything, `W/"stale"`, testBaseURL).
		Return((*SCIMGroup)(nil), &ErrorPreconditionFailed)

	h := newSCIMGroupsHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodPut, "/scim/v2/Groups/group-1",
		bytes.NewBufferString(`{"schemas":["`+SCIMCoreGroupSchemaURN+`"],"displayName":"Renamed","members":[]}`))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	req.Header.Set("If-Match", `W/"stale"`)
	req.SetPathValue("id", "group-1")
	rr := httptest.NewRecorder()

	h.HandleGroupsReplaceRequest(rr, req)

	require.Equal(t, http.StatusPreconditionFailed, rr.Code)
}

// TestHandleGroupsPatchRequest_PreconditionFailed tests Handle Groups Patch Request for Precondition Failed.
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

// TestHandleGroupsDeleteRequest_PreconditionFailed tests Handle Groups Delete Request for Precondition Failed.
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

// TestHandleGroupsListRequest_CustomParamsAndError tests Handle Groups List Request for Custom Params And Error.
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

	t.Run("InvalidStartIndexUsesDefault", func(t *testing.T) {
		mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
		expectedResp := SCIMGroupListResponse{
			Schemas: []string{SCIMListResponseSchemaURN}, StartIndex: 1, ItemsPerPage: constants.DefaultPageSize,
			Resources: []SCIMGroup{},
		}
		mockSvc.On("ListGroups", mock.Anything, 1, constants.DefaultPageSize, testBaseURL).
			Return(expectedResp, (*tidcommon.ServiceError)(nil))

		h := newSCIMGroupsHandler(mockSvc, testBaseURL)
		req := httptest.NewRequest(http.MethodGet, "/scim/v2/Groups?startIndex=abc", nil)
		rr := httptest.NewRecorder()

		h.HandleGroupsListRequest(rr, req)
		require.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("NegativeCountInterpretedAsZero", func(t *testing.T) {
		mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
		expectedResp := SCIMGroupListResponse{
			Schemas: []string{SCIMListResponseSchemaURN}, StartIndex: 1, ItemsPerPage: 0,
			Resources: []SCIMGroup{},
		}
		mockSvc.On("ListGroups", mock.Anything, 1, 0, testBaseURL).
			Return(expectedResp, (*tidcommon.ServiceError)(nil))

		h := newSCIMGroupsHandler(mockSvc, testBaseURL)
		req := httptest.NewRequest(http.MethodGet, "/scim/v2/Groups?count=-5", nil)
		rr := httptest.NewRecorder()

		h.HandleGroupsListRequest(rr, req)
		require.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("ServiceError", func(t *testing.T) {
		mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
		mockSvc.On("ListGroups", mock.Anything, 1, constants.DefaultPageSize, testBaseURL).
			Return(SCIMGroupListResponse{}, &ErrorInternalServer)

		h := newSCIMGroupsHandler(mockSvc, testBaseURL)
		req := httptest.NewRequest(http.MethodGet, "/scim/v2/Groups", nil)
		rr := httptest.NewRecorder()

		h.HandleGroupsListRequest(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

// TestHandleGroupsCreateRequest_ServiceError tests Handle Groups Create Request for Service Error.
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

// TestHandleGroupsReplaceRequest_ErrorScenarios tests Handle Groups Replace Request for Error Scenarios.
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

	t.Run("MissingCoreSchema", func(t *testing.T) {
		h := newSCIMGroupsHandler(nil, testBaseURL)
		req := httptest.NewRequest(http.MethodPut, "/scim/v2/Groups/group-1",
			bytes.NewBufferString(`{"displayName":"Renamed","members":[]}`))
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
			bytes.NewBufferString(`{"schemas":["`+SCIMCoreGroupSchemaURN+`"],"displayName":"Renamed","members":[]}`))
		req.Header.Set("Content-Type", constants.SCIMContentType)
		req.SetPathValue("id", "group-1")
		rr := httptest.NewRecorder()

		h.HandleGroupsReplaceRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})
}

// TestHandleGroupsPatchRequest_ErrorScenarios tests Handle Groups Patch Request for Error Scenarios.
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

// TestHandleGroupsDeleteRequest_ErrorScenarios tests Handle Groups Delete Request for Error Scenarios.
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

// TestGroupsHandler_HandleSCIMError_ServerError tests Groups Handler for Handle SCIM Error Server Error.
func TestGroupsHandler_HandleSCIMError_ServerError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Groups/group-1", nil)
	req.SetPathValue("id", "group-1")
	rr := httptest.NewRecorder()

	svcErr := &tidcommon.ServiceError{
		Type: tidcommon.ServerErrorType,
		ErrorDescription: tidcommon.I18nMessage{
			DefaultValue: "internal server error happened",
		},
	}
	handleSCIMError(rr, req, svcErr)

	require.Equal(t, http.StatusInternalServerError, rr.Code)
	var errResp SCIMErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, "500", errResp.Status)
	require.Equal(t, "internal server error happened", errResp.Detail)
}

// TestHandleGroups_BodyExceedsLimit tests Handle Groups for Body Exceeds Limit.
func TestHandleGroups_BodyExceedsLimit(t *testing.T) {
	h := newSCIMGroupsHandler(nil, testBaseURL)

	t.Run("Create", func(t *testing.T) {
		oversizedBody := bytes.NewBuffer(make([]byte, maxRequestBodyBytes+10))
		req := httptest.NewRequest(http.MethodPost, "/scim/v2/Groups", oversizedBody)
		req.Header.Set("Content-Type", constants.SCIMContentType)
		rr := httptest.NewRecorder()
		h.HandleGroupsCreateRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("Replace", func(t *testing.T) {
		oversizedBody := bytes.NewBuffer(make([]byte, maxRequestBodyBytes+10))
		req := httptest.NewRequest(http.MethodPut, "/scim/v2/Groups/group-1", oversizedBody)
		req.SetPathValue("id", "group-1")
		req.Header.Set("Content-Type", constants.SCIMContentType)
		rr := httptest.NewRecorder()
		h.HandleGroupsReplaceRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("Patch", func(t *testing.T) {
		oversizedBody := bytes.NewBuffer(make([]byte, maxRequestBodyBytes+10))
		req := httptest.NewRequest(http.MethodPatch, "/scim/v2/Groups/group-1", oversizedBody)
		req.SetPathValue("id", "group-1")
		req.Header.Set("Content-Type", constants.SCIMContentType)
		rr := httptest.NewRecorder()
		h.HandleGroupsPatchRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})
}
