// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package groups

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	scim "github.com/thunder-id/thunderid/internal/scim/common"
	scimconfig "github.com/thunder-id/thunderid/internal/scim/config"
	"github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// HandlerTestSuite groups the tests in handler_test.go.
type HandlerTestSuite struct {
	suite.Suite
}

// TestHandlerTestSuite runs HandlerTestSuite.
func TestHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(HandlerTestSuite))
}

const (
	testBaseURL    = "https://thunderid.example.com"
	testAPIBaseURL = "https://api.example.com"
)

// testHandlerConfig is the handler configuration used by the tests.
var testHandlerConfig = scimconfig.SCIMConfig{PublicURL: testBaseURL}

// TestHandleGroupsListRequest_Success tests Handle Groups List Request for Success.
func (suite *HandlerTestSuite) TestHandleGroupsListRequest_Success() {
	t := suite.T()
	mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
	expectedResp := SCIMGroupListResponse{
		Schemas: []string{scim.SCIMListResponseSchemaURN}, StartIndex: 1, ItemsPerPage: constants.DefaultPageSize,
		Resources: []SCIMGroup{},
	}
	mockSvc.On("ListGroups", mock.Anything, 1, constants.DefaultPageSize, testBaseURL).
		Return(expectedResp, (*tidcommon.ServiceError)(nil))

	h := newSCIMGroupsHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Groups", nil)
	rr := httptest.NewRecorder()

	h.HandleGroupsListRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
}

// TestHandleGroupsListRequest_FilterNotSupported tests Handle Groups List Request for Filter Not Supported.
func (suite *HandlerTestSuite) TestHandleGroupsListRequest_FilterNotSupported() {
	t := suite.T()
	mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
	h := newSCIMGroupsHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Groups?filter=displayName+eq+%22x%22", nil)
	rr := httptest.NewRecorder()

	h.HandleGroupsListRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestHandleGroupsListRequest_CapsMaxCount tests Handle Groups List Request for Caps Max Count.
func (suite *HandlerTestSuite) TestHandleGroupsListRequest_CapsMaxCount() {
	t := suite.T()
	mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
	mockSvc.On("ListGroups", mock.Anything, 1, constants.MaxPageSize, testBaseURL).
		Return(SCIMGroupListResponse{}, (*tidcommon.ServiceError)(nil))

	h := newSCIMGroupsHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Groups?count=100000", nil)
	rr := httptest.NewRecorder()

	h.HandleGroupsListRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
}

// TestHandleGroupsListRequest_SortNotSupported tests Handle Groups List Request for Sort Not Supported.
func (suite *HandlerTestSuite) TestHandleGroupsListRequest_SortNotSupported() {
	t := suite.T()
	h := newSCIMGroupsHandler(NewSCIMGroupsServiceInterfaceMock(t), testHandlerConfig)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Groups?sortBy=displayName", nil)
	rr := httptest.NewRecorder()

	h.HandleGroupsListRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	var errResp scim.SCIMErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, scim.ScimErrorTypeInvalidValue, errResp.ScimType)
}

// TestHandleGroupsGetRequest_Success tests Handle Groups Get Request for Success.
func (suite *HandlerTestSuite) TestHandleGroupsGetRequest_Success() {
	t := suite.T()
	mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
	expected := &SCIMGroup{ID: "group-1", DisplayName: "Admins",
		Meta: scim.SCIMMeta{ResourceType: "Group", Location: testBaseURL + "/scim/v2/Groups/group-1"}}
	mockSvc.On("GetGroup", mock.Anything, "group-1", testBaseURL).
		Return(expected, (*tidcommon.ServiceError)(nil))

	h := newSCIMGroupsHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Groups/group-1", nil)
	req.SetPathValue("id", "group-1")
	rr := httptest.NewRecorder()

	h.HandleGroupsGetRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var got SCIMGroup
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	require.Equal(t, "Admins", got.DisplayName)
}

// Verifies that ErrorResourceNotFound maps to a 404 Not Found response (RFC 7644 §3.12).
// TestHandleGroupsGetRequest_NotFound tests Handle Groups Get Request for Not Found.
func (suite *HandlerTestSuite) TestHandleGroupsGetRequest_NotFound() {
	t := suite.T()
	mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
	mockSvc.On("GetGroup", mock.Anything, "missing", testBaseURL).
		Return((*SCIMGroup)(nil), &scim.ErrorResourceNotFound)

	h := newSCIMGroupsHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Groups/missing", nil)
	req.SetPathValue("id", "missing")
	rr := httptest.NewRecorder()

	h.HandleGroupsGetRequest(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
}

// TestHandleGroupsGetRequest_MissingID tests Handle Groups Get Request for Missing ID.
func (suite *HandlerTestSuite) TestHandleGroupsGetRequest_MissingID() {
	t := suite.T()
	mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
	h := newSCIMGroupsHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Groups/", nil)
	rr := httptest.NewRecorder()

	h.HandleGroupsGetRequest(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code) // documents the same gap as above
}

// TestHandleGroupsCreateRequest_Success tests Handle Groups Create Request for Success.
func (suite *HandlerTestSuite) TestHandleGroupsCreateRequest_Success() {
	t := suite.T()
	mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
	expected := &SCIMGroup{ID: "group-1", DisplayName: "Engineering",
		Meta: scim.SCIMMeta{Location: testBaseURL + "/scim/v2/Groups/group-1"}}
	mockSvc.On("CreateGroup", mock.Anything, "Engineering", mock.Anything, testBaseURL).
		Return(expected, (*tidcommon.ServiceError)(nil))

	h := newSCIMGroupsHandler(mockSvc, testHandlerConfig)
	body := `{"schemas":["` + scim.SCIMCoreGroupSchemaURN + `"],"displayName":"Engineering","members":[]}`
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Groups", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleGroupsCreateRequest(rr, req)

	require.Equal(t, http.StatusCreated, rr.Code)
	require.Equal(t, expected.Meta.Location, rr.Header().Get("Location"))
}

// TestHandleGroupsCreateRequest_MissingDisplayName tests Handle Groups Create Request for Missing Display Name.
func (suite *HandlerTestSuite) TestHandleGroupsCreateRequest_MissingDisplayName() {
	t := suite.T()
	mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
	h := newSCIMGroupsHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Groups", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleGroupsCreateRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestHandleGroupsCreateRequest_WrongContentType tests Handle Groups Create Request for Wrong Content Type.
func (suite *HandlerTestSuite) TestHandleGroupsCreateRequest_WrongContentType() {
	t := suite.T()
	mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
	h := newSCIMGroupsHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Groups",
		bytes.NewBufferString(`{"displayName":"X"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.HandleGroupsCreateRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestHandleGroupsCreateRequest_EmptyBody tests Handle Groups Create Request for Empty Body.
func (suite *HandlerTestSuite) TestHandleGroupsCreateRequest_EmptyBody() {
	t := suite.T()
	mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
	h := newSCIMGroupsHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Groups", bytes.NewBufferString(""))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleGroupsCreateRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestHandleGroupsReplaceRequest_Success tests Handle Groups Replace Request for Success.
func (suite *HandlerTestSuite) TestHandleGroupsReplaceRequest_Success() {
	t := suite.T()
	mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
	expected := &SCIMGroup{ID: "group-1", DisplayName: "Renamed"}
	mockSvc.On("ReplaceGroup", mock.Anything, "group-1", "Renamed", mock.Anything, testBaseURL).
		Return(expected, (*tidcommon.ServiceError)(nil))

	h := newSCIMGroupsHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(http.MethodPut, "/scim/v2/Groups/group-1",
		bytes.NewBufferString(`{"schemas":["`+scim.SCIMCoreGroupSchemaURN+`"],"displayName":"Renamed","members":[]}`))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	req.SetPathValue("id", "group-1")
	rr := httptest.NewRecorder()

	h.HandleGroupsReplaceRequest(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)
}

// TestHandleGroupsPatchRequest_Success tests Handle Groups Patch Request for Success.
func (suite *HandlerTestSuite) TestHandleGroupsPatchRequest_Success() {
	t := suite.T()
	mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
	expected := &SCIMGroup{ID: "group-1", DisplayName: "Patched"}
	mockSvc.On("PatchGroup", mock.Anything, "group-1", mock.Anything, testBaseURL).
		Return(expected, (*tidcommon.ServiceError)(nil))

	h := newSCIMGroupsHandler(mockSvc, testHandlerConfig)
	body := `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
		"Operations": [{"op": "replace", "path": "displayName", "value": "Patched"}]
	}`
	req := httptest.NewRequest(http.MethodPatch, "/scim/v2/Groups/group-1", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	req.SetPathValue("id", "group-1")
	rr := httptest.NewRecorder()

	h.HandleGroupsPatchRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
}

// TestHandleGroupsPatchRequest_InvalidBody tests Handle Groups Patch Request for Invalid Body.
func (suite *HandlerTestSuite) TestHandleGroupsPatchRequest_InvalidBody() {
	t := suite.T()
	mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
	h := newSCIMGroupsHandler(mockSvc, testHandlerConfig)
	body := `{"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"], "Operations": [{"op": "bogus"}]}`
	req := httptest.NewRequest(http.MethodPatch, "/scim/v2/Groups/group-1", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	req.SetPathValue("id", "group-1")
	rr := httptest.NewRecorder()

	h.HandleGroupsPatchRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestHandleGroupsDeleteRequest_Success tests Handle Groups Delete Request for Success.
func (suite *HandlerTestSuite) TestHandleGroupsDeleteRequest_Success() {
	t := suite.T()
	mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
	mockSvc.On("DeleteGroup", mock.Anything, "group-1").Return((*tidcommon.ServiceError)(nil))

	h := newSCIMGroupsHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(http.MethodDelete, "/scim/v2/Groups/group-1", nil)
	req.SetPathValue("id", "group-1")
	rr := httptest.NewRecorder()

	h.HandleGroupsDeleteRequest(rr, req)

	require.Equal(t, http.StatusNoContent, rr.Code)
}

// TestHandleGroupsDeleteRequest_MutabilityViolation tests Handle Groups Delete Request for Mutability Violation.
func (suite *HandlerTestSuite) TestHandleGroupsDeleteRequest_MutabilityViolation() {
	t := suite.T()
	mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
	mockSvc.On("DeleteGroup", mock.Anything, "group-1").Return(&scim.ErrorMutabilityViolation)

	h := newSCIMGroupsHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(http.MethodDelete, "/scim/v2/Groups/group-1", nil)
	req.SetPathValue("id", "group-1")
	rr := httptest.NewRecorder()

	h.HandleGroupsDeleteRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestHandleGroupsListRequest_CustomParamsAndError tests Handle Groups List Request for Custom Params And Error.
func (suite *HandlerTestSuite) TestHandleGroupsListRequest_CustomParamsAndError() {
	t := suite.T()
	t.Run("ValidParams", func(t *testing.T) {
		mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
		expectedResp := SCIMGroupListResponse{
			Schemas:      []string{scim.SCIMListResponseSchemaURN},
			StartIndex:   5,
			ItemsPerPage: 10,
			Resources:    []SCIMGroup{},
		}
		mockSvc.On("ListGroups", mock.Anything, 5, 10, testBaseURL).
			Return(expectedResp, (*tidcommon.ServiceError)(nil))

		h := newSCIMGroupsHandler(mockSvc, testHandlerConfig)
		req := httptest.NewRequest(http.MethodGet, "/scim/v2/Groups?startIndex=5&count=10", nil)
		rr := httptest.NewRecorder()

		h.HandleGroupsListRequest(rr, req)
		require.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("InvalidStartIndexUsesDefault", func(t *testing.T) {
		mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
		expectedResp := SCIMGroupListResponse{
			Schemas: []string{scim.SCIMListResponseSchemaURN}, StartIndex: 1, ItemsPerPage: constants.DefaultPageSize,
			Resources: []SCIMGroup{},
		}
		mockSvc.On("ListGroups", mock.Anything, 1, constants.DefaultPageSize, testBaseURL).
			Return(expectedResp, (*tidcommon.ServiceError)(nil))

		h := newSCIMGroupsHandler(mockSvc, testHandlerConfig)
		req := httptest.NewRequest(http.MethodGet, "/scim/v2/Groups?startIndex=abc", nil)
		rr := httptest.NewRecorder()

		h.HandleGroupsListRequest(rr, req)
		require.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("NegativeCountInterpretedAsZero", func(t *testing.T) {
		mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
		expectedResp := SCIMGroupListResponse{
			Schemas: []string{scim.SCIMListResponseSchemaURN}, StartIndex: 1, ItemsPerPage: 0,
			Resources: []SCIMGroup{},
		}
		mockSvc.On("ListGroups", mock.Anything, 1, 0, testBaseURL).
			Return(expectedResp, (*tidcommon.ServiceError)(nil))

		h := newSCIMGroupsHandler(mockSvc, testHandlerConfig)
		req := httptest.NewRequest(http.MethodGet, "/scim/v2/Groups?count=-5", nil)
		rr := httptest.NewRecorder()

		h.HandleGroupsListRequest(rr, req)
		require.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("ServiceError", func(t *testing.T) {
		mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
		mockSvc.On("ListGroups", mock.Anything, 1, constants.DefaultPageSize, testBaseURL).
			Return(SCIMGroupListResponse{}, &tidcommon.InternalServerError)

		h := newSCIMGroupsHandler(mockSvc, testHandlerConfig)
		req := httptest.NewRequest(http.MethodGet, "/scim/v2/Groups", nil)
		rr := httptest.NewRecorder()

		h.HandleGroupsListRequest(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

// TestHandleGroupsCreateRequest_ServiceError tests Handle Groups Create Request for Service Error.
func (suite *HandlerTestSuite) TestHandleGroupsCreateRequest_ServiceError() {
	t := suite.T()
	mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
	mockSvc.On("CreateGroup", mock.Anything, "Engineering", mock.Anything, testBaseURL).
		Return((*SCIMGroup)(nil), &scim.ErrorUniquenessConflict)

	h := newSCIMGroupsHandler(mockSvc, testHandlerConfig)
	body := `{"schemas":["` + scim.SCIMCoreGroupSchemaURN + `"],"displayName":"Engineering","members":[]}`
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Groups", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleGroupsCreateRequest(rr, req)
	require.Equal(t, http.StatusConflict, rr.Code)
}

// TestHandleGroupsReplaceRequest_ErrorScenarios tests Handle Groups Replace Request for Error Scenarios.
func (suite *HandlerTestSuite) TestHandleGroupsReplaceRequest_ErrorScenarios() {
	t := suite.T()
	t.Run("MissingID", func(t *testing.T) {
		h := newSCIMGroupsHandler(nil, testHandlerConfig)
		req := httptest.NewRequest(http.MethodPut, "/scim/v2/Groups/", nil)
		rr := httptest.NewRecorder()

		h.HandleGroupsReplaceRequest(rr, req)
		require.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("WrongContentType", func(t *testing.T) {
		h := newSCIMGroupsHandler(nil, testHandlerConfig)
		req := httptest.NewRequest(http.MethodPut, "/scim/v2/Groups/group-1", bytes.NewBufferString(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.SetPathValue("id", "group-1")
		rr := httptest.NewRecorder()

		h.HandleGroupsReplaceRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("EmptyBody", func(t *testing.T) {
		h := newSCIMGroupsHandler(nil, testHandlerConfig)
		req := httptest.NewRequest(http.MethodPut, "/scim/v2/Groups/group-1", bytes.NewBufferString(""))
		req.Header.Set("Content-Type", constants.SCIMContentType)
		req.SetPathValue("id", "group-1")
		rr := httptest.NewRecorder()

		h.HandleGroupsReplaceRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		h := newSCIMGroupsHandler(nil, testHandlerConfig)
		req := httptest.NewRequest(http.MethodPut, "/scim/v2/Groups/group-1", bytes.NewBufferString(`{invalid`))
		req.Header.Set("Content-Type", constants.SCIMContentType)
		req.SetPathValue("id", "group-1")
		rr := httptest.NewRecorder()

		h.HandleGroupsReplaceRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("MissingCoreSchema", func(t *testing.T) {
		h := newSCIMGroupsHandler(nil, testHandlerConfig)
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
		mockSvc.On("ReplaceGroup", mock.Anything, "group-1", "Renamed", mock.Anything, testBaseURL).
			Return((*SCIMGroup)(nil), &scim.ErrorMutabilityViolation)

		h := newSCIMGroupsHandler(mockSvc, testHandlerConfig)
		req := httptest.NewRequest(http.MethodPut, "/scim/v2/Groups/group-1",
			bytes.NewBufferString(
				`{"schemas":["`+scim.SCIMCoreGroupSchemaURN+`"],"displayName":"Renamed","members":[]}`))
		req.Header.Set("Content-Type", constants.SCIMContentType)
		req.SetPathValue("id", "group-1")
		rr := httptest.NewRecorder()

		h.HandleGroupsReplaceRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})
}

// TestHandleGroupsPatchRequest_ErrorScenarios tests Handle Groups Patch Request for Error Scenarios.
func (suite *HandlerTestSuite) TestHandleGroupsPatchRequest_ErrorScenarios() {
	t := suite.T()
	t.Run("MissingID", func(t *testing.T) {
		h := newSCIMGroupsHandler(nil, testHandlerConfig)
		req := httptest.NewRequest(http.MethodPatch, "/scim/v2/Groups/", nil)
		rr := httptest.NewRecorder()

		h.HandleGroupsPatchRequest(rr, req)
		require.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("WrongContentType", func(t *testing.T) {
		h := newSCIMGroupsHandler(nil, testHandlerConfig)
		req := httptest.NewRequest(http.MethodPatch, "/scim/v2/Groups/group-1", bytes.NewBufferString(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.SetPathValue("id", "group-1")
		rr := httptest.NewRecorder()

		h.HandleGroupsPatchRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("EmptyBody", func(t *testing.T) {
		h := newSCIMGroupsHandler(nil, testHandlerConfig)
		req := httptest.NewRequest(http.MethodPatch, "/scim/v2/Groups/group-1", bytes.NewBufferString(""))
		req.Header.Set("Content-Type", constants.SCIMContentType)
		req.SetPathValue("id", "group-1")
		rr := httptest.NewRecorder()

		h.HandleGroupsPatchRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("ServiceError", func(t *testing.T) {
		mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
		mockSvc.On("PatchGroup", mock.Anything, "group-1", mock.Anything, testBaseURL).
			Return((*SCIMGroup)(nil), &scim.ErrorMutabilityViolation)

		h := newSCIMGroupsHandler(mockSvc, testHandlerConfig)
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
func (suite *HandlerTestSuite) TestHandleGroupsDeleteRequest_ErrorScenarios() {
	t := suite.T()
	t.Run("MissingID", func(t *testing.T) {
		h := newSCIMGroupsHandler(nil, testHandlerConfig)
		req := httptest.NewRequest(http.MethodDelete, "/scim/v2/Groups/", nil)
		rr := httptest.NewRecorder()

		h.HandleGroupsDeleteRequest(rr, req)
		require.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("ServiceError", func(t *testing.T) {
		mockSvc := NewSCIMGroupsServiceInterfaceMock(t)
		mockSvc.On("DeleteGroup", mock.Anything, "group-1").
			Return(&tidcommon.InternalServerError)

		h := newSCIMGroupsHandler(mockSvc, testHandlerConfig)
		req := httptest.NewRequest(http.MethodDelete, "/scim/v2/Groups/group-1", nil)
		req.SetPathValue("id", "group-1")
		rr := httptest.NewRecorder()

		h.HandleGroupsDeleteRequest(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

// TestGroupsHandler_HandleSCIMError_ServerError tests Groups Handler for Handle SCIM Error Server Error.
func (suite *HandlerTestSuite) TestGroupsHandler_HandleSCIMError_ServerError() {
	t := suite.T()
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Groups/group-1", nil)
	req.SetPathValue("id", "group-1")
	rr := httptest.NewRecorder()

	svcErr := &tidcommon.ServiceError{
		Type: tidcommon.ServerErrorType,
		ErrorDescription: tidcommon.I18nMessage{
			DefaultValue: "internal server error happened",
		},
	}
	scim.HandleSCIMError(rr, req, svcErr, *log.GetLogger())

	require.Equal(t, http.StatusInternalServerError, rr.Code)
	var errResp scim.SCIMErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, "500", errResp.Status)
	require.Equal(t, "internal server error happened", errResp.Detail)
}

// TestHandleGroups_BodyExceedsLimit tests Handle Groups for Body Exceeds Limit.
func (suite *HandlerTestSuite) TestHandleGroups_BodyExceedsLimit() {
	t := suite.T()
	h := newSCIMGroupsHandler(nil, testHandlerConfig)

	t.Run("Create", func(t *testing.T) {
		oversizedBody := bytes.NewBuffer(make([]byte, scim.MaxRequestBodyBytes+10))
		req := httptest.NewRequest(http.MethodPost, "/scim/v2/Groups", oversizedBody)
		req.Header.Set("Content-Type", constants.SCIMContentType)
		rr := httptest.NewRecorder()
		h.HandleGroupsCreateRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("Replace", func(t *testing.T) {
		oversizedBody := bytes.NewBuffer(make([]byte, scim.MaxRequestBodyBytes+10))
		req := httptest.NewRequest(http.MethodPut, "/scim/v2/Groups/group-1", oversizedBody)
		req.SetPathValue("id", "group-1")
		req.Header.Set("Content-Type", constants.SCIMContentType)
		rr := httptest.NewRecorder()
		h.HandleGroupsReplaceRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("Patch", func(t *testing.T) {
		oversizedBody := bytes.NewBuffer(make([]byte, scim.MaxRequestBodyBytes+10))
		req := httptest.NewRequest(http.MethodPatch, "/scim/v2/Groups/group-1", oversizedBody)
		req.SetPathValue("id", "group-1")
		req.Header.Set("Content-Type", constants.SCIMContentType)
		rr := httptest.NewRecorder()
		h.HandleGroupsPatchRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})
}
