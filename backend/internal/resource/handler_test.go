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

package resource

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
)

const (
	testResourceID = "res-123"
)

// HandlerTestSuite is the test suite for resource handler tests
type HandlerTestSuite struct {
	suite.Suite
	mockService *ResourceServiceInterfaceMock
	handler     *resourceHandler
}

// SetupTest runs before each test
func (suite *HandlerTestSuite) SetupTest() {
	suite.mockService = new(ResourceServiceInterfaceMock)
	suite.handler = newResourceHandler(suite.mockService)
}

// TestHandlerTestSuite runs the test suite
func TestHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(HandlerTestSuite))
}

// Resource Server Handler Tests

func (suite *HandlerTestSuite) TestHandleResourceServerListRequest_Success() {
	resourceServers := []ResourceServer{
		{ID: "rs-1", Name: "RS 1"},
		{ID: "rs-2", Name: "RS 2"},
	}
	links := []Link{
		{Href: "/resource-servers?limit=30&offset=0", Rel: "self"},
	}
	suite.mockService.On("GetResourceServerList", mock.Anything,
		30, 0).Return(&ResourceServerList{
		TotalResults:    2,
		StartIndex:      1,
		Count:           2,
		ResourceServers: resourceServers,
		Links:           links,
	}, nil)

	req := httptest.NewRequest("GET", "/resource-servers", nil)
	w := httptest.NewRecorder()

	suite.handler.HandleResourceServerListRequest(w, req)

	suite.Equal(http.StatusOK, w.Code)
	var resp ResourceServerListResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	suite.NoError(err)
	suite.Equal(2, resp.TotalResults)
	suite.Equal(2, len(resp.ResourceServers))
}

func (suite *HandlerTestSuite) TestHandleResourceServerListRequest_InvalidLimit() {
	req := httptest.NewRequest("GET", "/resource-servers?limit=invalid", nil)
	w := httptest.NewRecorder()

	suite.handler.HandleResourceServerListRequest(w, req)

	suite.Equal(http.StatusBadRequest, w.Code)
}

func (suite *HandlerTestSuite) TestHandleResourceServerListRequest_Error() {
	suite.mockService.On("GetResourceServerList", mock.Anything,
		30, 0).Return(nil, &serviceerror.InternalServerError)

	req := httptest.NewRequest("GET", "/resource-servers", nil)
	w := httptest.NewRecorder()

	suite.handler.HandleResourceServerListRequest(w, req)

	suite.Equal(http.StatusInternalServerError, w.Code)
}

func (suite *HandlerTestSuite) TestHandleResourceServerPostRequest_Success() {
	reqBody := CreateResourceServerRequest{
		Name:        "test-rs",
		Description: "Test",
		OUID:        "ou-123",
	}

	suite.mockService.On("CreateResourceServer", mock.Anything,
		mock.MatchedBy(func(rs ResourceServer) bool {
			return rs.Name == "test-rs"
		})).Return(&ResourceServer{
		ID:          "rs-123",
		Name:        "test-rs",
		Description: "Test",
		OUID:        "ou-123",
	}, nil)

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/resource-servers", bytes.NewReader(body))
	w := httptest.NewRecorder()

	suite.handler.HandleResourceServerPostRequest(w, req)

	suite.Equal(http.StatusCreated, w.Code)
	var resp ResourceServerResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	suite.NoError(err)
	suite.Equal("rs-123", resp.ID)
	suite.Equal("test-rs", resp.Name)
}

func (suite *HandlerTestSuite) TestHandleResourceServerPostRequest_InvalidJSON() {
	req := httptest.NewRequest("POST", "/resource-servers", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	suite.handler.HandleResourceServerPostRequest(w, req)

	suite.Equal(http.StatusBadRequest, w.Code)
}

func (suite *HandlerTestSuite) TestHandleResourceServerGetRequest_Success() {
	suite.mockService.On("GetResourceServer", mock.Anything,
		"rs-123").Return(&ResourceServer{
		ID:   "rs-123",
		Name: "test-rs",
	}, nil)

	req := httptest.NewRequest("GET", "/resource-servers/rs-123", nil)
	req.SetPathValue("id", "rs-123")
	w := httptest.NewRecorder()

	suite.handler.HandleResourceServerGetRequest(w, req)

	suite.Equal(http.StatusOK, w.Code)
	var resp ResourceServerResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	suite.NoError(err)
	suite.Equal("rs-123", resp.ID)
}

func (suite *HandlerTestSuite) TestHandleResourceServerGetRequest_NotFound() {
	suite.mockService.On("GetResourceServer", mock.Anything,
		"rs-123").Return(nil, &ErrorResourceServerNotFound)

	req := httptest.NewRequest("GET", "/resource-servers/rs-123", nil)
	req.SetPathValue("id", "rs-123")
	w := httptest.NewRecorder()

	suite.handler.HandleResourceServerGetRequest(w, req)

	suite.Equal(http.StatusNotFound, w.Code)
}

func (suite *HandlerTestSuite) TestHandleResourceServerPutRequest_Success() {
	reqBody := UpdateResourceServerRequest{
		Name: "updated-rs",
		OUID: "ou-123",
	}

	suite.mockService.On("UpdateResourceServer", mock.Anything,
		"rs-123", mock.Anything).Return(&ResourceServer{
		ID:   "rs-123",
		Name: "updated-rs",
	}, nil)

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("PUT", "/resource-servers/rs-123", bytes.NewReader(body))
	req.SetPathValue("id", "rs-123")
	w := httptest.NewRecorder()

	suite.handler.HandleResourceServerPutRequest(w, req)

	suite.Equal(http.StatusOK, w.Code)
}

func (suite *HandlerTestSuite) TestHandleResourceServerDeleteRequest_Success() {
	suite.mockService.On("DeleteResourceServer", mock.Anything,
		"rs-123").Return(nil)

	req := httptest.NewRequest("DELETE", "/resource-servers/rs-123", nil)
	req.SetPathValue("id", "rs-123")
	w := httptest.NewRecorder()

	suite.handler.HandleResourceServerDeleteRequest(w, req)

	suite.Equal(http.StatusNoContent, w.Code)
}

// Resource Handler Tests

func (suite *HandlerTestSuite) TestHandleResourceListRequest_Success() {
	resources := []Resource{
		{ID: "res-1", Name: "Resource 1"},
		{ID: "res-2", Name: "Resource 2"},
	}
	links := []Link{
		{Href: "/resource-servers?limit=30&offset=0", Rel: "self"},
	}
	suite.mockService.On("GetResourceList", mock.Anything,
		"rs-123", (*string)(nil), 30, 0).Return(&ResourceList{
		TotalResults: 2,
		StartIndex:   1,
		Count:        2,
		Resources:    resources,
		Links:        links,
	}, nil)

	req := httptest.NewRequest("GET", "/resource-servers/rs-123/resources", nil)
	req.SetPathValue("rsId", "rs-123")
	w := httptest.NewRecorder()

	suite.handler.HandleResourceListRequest(w, req)

	suite.Equal(http.StatusOK, w.Code)
	var resp ResourceListResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	suite.NoError(err)
	suite.Equal(2, resp.TotalResults)
}

func (suite *HandlerTestSuite) TestHandleResourceListRequest_WithParentFilter() {
	emptyStr := ""
	suite.mockService.On("GetResourceList", mock.Anything,
		"rs-123", &emptyStr, 30, 0).Return(&ResourceList{
		TotalResults: 1,
		Resources:    []Resource{{ID: "res-1"}},
	}, nil)

	req := httptest.NewRequest("GET", "/resource-servers/rs-123/resources?parentId=", nil)
	req.SetPathValue("rsId", "rs-123")
	w := httptest.NewRecorder()

	suite.handler.HandleResourceListRequest(w, req)

	suite.Equal(http.StatusOK, w.Code)
}

func (suite *HandlerTestSuite) TestHandleResourceListRequest_WithParentUUID() {
	parentUUID := "parent-uuid-123"
	suite.mockService.On("GetResourceList", mock.Anything,
		"rs-123", &parentUUID, 30, 0).Return(&ResourceList{
		TotalResults: 2,
		Resources:    []Resource{{ID: "res-1"}, {ID: "res-2"}},
	}, nil)

	req := httptest.NewRequest("GET", "/resource-servers/rs-123/resources?parentId=parent-uuid-123", nil)
	req.SetPathValue("rsId", "rs-123")
	w := httptest.NewRecorder()

	suite.handler.HandleResourceListRequest(w, req)

	suite.Equal(http.StatusOK, w.Code)
}

func (suite *HandlerTestSuite) TestHandleResourcePostRequest_Success() {
	reqBody := CreateResourceRequest{
		Name:   "test-resource",
		Handle: "test-handle",
	}

	suite.mockService.On("CreateResource", mock.Anything,
		"rs-123", mock.Anything).Return(&Resource{
		ID:     "res-123",
		Name:   "test-resource",
		Handle: "test-handle",
	}, nil)

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/resource-servers/rs-123/resources", bytes.NewReader(body))
	req.SetPathValue("rsId", "rs-123")
	w := httptest.NewRecorder()

	suite.handler.HandleResourcePostRequest(w, req)

	suite.Equal(http.StatusCreated, w.Code)
	var resp ResourceResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	suite.NoError(err)
	suite.Equal("res-123", resp.ID)
	suite.Equal("test-handle", resp.Handle)
}

func (suite *HandlerTestSuite) TestHandleResourceGetRequest_Success() {
	suite.mockService.On("GetResource", mock.Anything,
		"rs-123", "res-123").Return(&Resource{
		ID:   "res-123",
		Name: "test-resource",
	}, nil)

	req := httptest.NewRequest("GET", "/resource-servers/rs-123/resources/res-123", nil)
	req.SetPathValue("rsId", "rs-123")
	req.SetPathValue("id", "res-123")
	w := httptest.NewRecorder()

	suite.handler.HandleResourceGetRequest(w, req)

	suite.Equal(http.StatusOK, w.Code)
}

func (suite *HandlerTestSuite) TestHandleResourcePutRequest_Success() {
	reqBody := UpdateResourceRequest{
		Description: "updated description",
	}

	suite.mockService.On("UpdateResource", mock.Anything,
		"rs-123", "res-123", mock.Anything).Return(&Resource{
		ID:          "res-123",
		Description: "updated description",
	}, nil)

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("PUT", "/resource-servers/rs-123/resources/res-123", bytes.NewReader(body))
	req.SetPathValue("rsId", "rs-123")
	req.SetPathValue("id", "res-123")
	w := httptest.NewRecorder()

	suite.handler.HandleResourcePutRequest(w, req)

	suite.Equal(http.StatusOK, w.Code)
}

func (suite *HandlerTestSuite) TestHandleResourceDeleteRequest_Success() {
	suite.mockService.On("DeleteResource", mock.Anything,
		"rs-123", "res-123").Return(nil)

	req := httptest.NewRequest("DELETE", "/resource-servers/rs-123/resources/res-123", nil)
	req.SetPathValue("rsId", "rs-123")
	req.SetPathValue("id", "res-123")
	w := httptest.NewRecorder()

	suite.handler.HandleResourceDeleteRequest(w, req)

	suite.Equal(http.StatusNoContent, w.Code)
}

// Action Handler Tests (Resource Server Level)

func (suite *HandlerTestSuite) TestHandleActionListAtResourceServerRequest_Success() {
	actions := []Action{
		{ID: "action-1", Name: "Action 1"},
		{ID: "action-2", Name: "Action 2"},
	}
	links := []Link{
		{Href: "/resource-servers?limit=30&offset=0", Rel: "self"},
	}
	var nilResourceID *string
	suite.mockService.On("GetActionList", mock.Anything,
		"rs-123", nilResourceID, 30, 0).Return(&ActionList{
		TotalResults: 2,
		StartIndex:   1,
		Count:        2,
		Actions:      actions,
		Links:        links,
	}, nil)

	req := httptest.NewRequest("GET", "/resource-servers/rs-123/actions", nil)
	req.SetPathValue("rsId", "rs-123")
	w := httptest.NewRecorder()

	suite.handler.HandleActionListAtResourceServerRequest(w, req)

	suite.Equal(http.StatusOK, w.Code)
	var resp ActionListResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	suite.NoError(err)
	suite.Equal(2, resp.TotalResults)
}

func (suite *HandlerTestSuite) TestHandleActionPostAtResourceServerRequest_Success() {
	reqBody := CreateActionRequest{
		Name:   "test-action",
		Handle: "test-handle",
	}

	var nilResourceID *string
	suite.mockService.On("CreateAction", mock.Anything,
		"rs-123", nilResourceID, mock.Anything).Return(&Action{
		ID:     "action-123",
		Name:   "test-action",
		Handle: "test-handle",
	}, nil)

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/resource-servers/rs-123/actions", bytes.NewReader(body))
	req.SetPathValue("rsId", "rs-123")
	w := httptest.NewRecorder()

	suite.handler.HandleActionPostAtResourceServerRequest(w, req)

	suite.Equal(http.StatusCreated, w.Code)
	var resp ActionResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	suite.NoError(err)
	suite.Equal("action-123", resp.ID)
	suite.Equal("test-handle", resp.Handle)
}

func (suite *HandlerTestSuite) TestHandleActionGetAtResourceServerRequest_Success() {
	var nilResourceID *string
	suite.mockService.On("GetAction", mock.Anything,
		"rs-123", nilResourceID, "action-123").Return(&Action{
		ID:   "action-123",
		Name: "test-action",
	}, nil)

	req := httptest.NewRequest("GET", "/resource-servers/rs-123/actions/action-123", nil)
	req.SetPathValue("rsId", "rs-123")
	req.SetPathValue("id", "action-123")
	w := httptest.NewRecorder()

	suite.handler.HandleActionGetAtResourceServerRequest(w, req)

	suite.Equal(http.StatusOK, w.Code)
}

func (suite *HandlerTestSuite) TestHandleActionPutAtResourceServerRequest_Success() {
	reqBody := UpdateActionRequest{
		Description: "updated description",
	}

	var nilResourceID *string
	suite.mockService.On("UpdateAction", mock.Anything,
		"rs-123", nilResourceID, "action-123", mock.Anything).Return(&Action{
		ID:          "action-123",
		Description: "updated description",
	}, nil)

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("PUT", "/resource-servers/rs-123/actions/action-123", bytes.NewReader(body))
	req.SetPathValue("rsId", "rs-123")
	req.SetPathValue("id", "action-123")
	w := httptest.NewRecorder()

	suite.handler.HandleActionPutAtResourceServerRequest(w, req)

	suite.Equal(http.StatusOK, w.Code)
}

func (suite *HandlerTestSuite) TestHandleActionDeleteAtResourceServerRequest_Success() {
	var nilResourceID *string
	suite.mockService.On("DeleteAction", mock.Anything,
		"rs-123", nilResourceID, "action-123").Return(nil)

	req := httptest.NewRequest("DELETE", "/resource-servers/rs-123/actions/action-123", nil)
	req.SetPathValue("rsId", "rs-123")
	req.SetPathValue("id", "action-123")
	w := httptest.NewRecorder()

	suite.handler.HandleActionDeleteAtResourceServerRequest(w, req)

	suite.Equal(http.StatusNoContent, w.Code)
}

// Action Handler Tests (Resource Level)

func (suite *HandlerTestSuite) TestHandleActionListAtResourceRequest_Success() {
	actions := []Action{
		{ID: "action-1", Name: "Action 1"},
	}

	resourceID := testResourceID
	suite.mockService.On("GetActionList", mock.Anything,
		"rs-123", &resourceID, 30, 0).Return(&ActionList{
		TotalResults: 1,
		Actions:      actions,
	}, nil)

	req := httptest.NewRequest("GET", "/resource-servers/rs-123/resources/res-123/actions", nil)
	req.SetPathValue("rsId", "rs-123")
	req.SetPathValue("resourceId", "res-123")
	w := httptest.NewRecorder()

	suite.handler.HandleActionListAtResourceRequest(w, req)

	suite.Equal(http.StatusOK, w.Code)
}

func (suite *HandlerTestSuite) TestHandleActionPostAtResourceRequest_Success() {
	reqBody := CreateActionRequest{
		Name:   "test-action",
		Handle: "test-handle",
	}

	resourceID := testResourceID
	suite.mockService.On("CreateAction", mock.Anything,
		"rs-123", &resourceID, mock.Anything).Return(&Action{
		ID:     "action-123",
		Name:   "test-action",
		Handle: "test-handle",
	}, nil)

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/resource-servers/rs-123/resources/res-123/actions", bytes.NewReader(body))
	req.SetPathValue("rsId", "rs-123")
	req.SetPathValue("resourceId", "res-123")
	w := httptest.NewRecorder()

	suite.handler.HandleActionPostAtResourceRequest(w, req)

	suite.Equal(http.StatusCreated, w.Code)
	var resp ActionResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	suite.NoError(err)
	suite.Equal("action-123", resp.ID)
	suite.Equal("test-handle", resp.Handle)
}

func (suite *HandlerTestSuite) TestHandleActionGetAtResourceRequest_Success() {
	resourceID := testResourceID
	suite.mockService.On("GetAction", mock.Anything,
		"rs-123", &resourceID, "action-123").Return(&Action{
		ID:   "action-123",
		Name: "test-action",
	}, nil)

	req := httptest.NewRequest("GET", "/resource-servers/rs-123/resources/res-123/actions/action-123", nil)
	req.SetPathValue("rsId", "rs-123")
	req.SetPathValue("resourceId", "res-123")
	req.SetPathValue("id", "action-123")
	w := httptest.NewRecorder()

	suite.handler.HandleActionGetAtResourceRequest(w, req)

	suite.Equal(http.StatusOK, w.Code)
}

// Helper Function Tests

func (suite *HandlerTestSuite) TestParsePaginationParams_Success() {
	req := httptest.NewRequest("GET", "/test?limit=50&offset=10", nil)

	limit, offset, err := parsePaginationParams(req.URL.Query())

	suite.Nil(err)
	suite.Equal(50, limit)
	suite.Equal(10, offset)
}

func (suite *HandlerTestSuite) TestParsePaginationParams_Defaults() {
	req := httptest.NewRequest("GET", "/test", nil)

	limit, offset, err := parsePaginationParams(req.URL.Query())

	suite.Nil(err)
	suite.Equal(30, limit) // Default page size
	suite.Equal(0, offset)
}

func (suite *HandlerTestSuite) TestParsePaginationParams_InvalidLimit() {
	req := httptest.NewRequest("GET", "/test?limit=invalid", nil)

	_, _, err := parsePaginationParams(req.URL.Query())

	suite.NotNil(err)
	suite.Equal(ErrorInvalidLimit.Code, err.Code)
}

func (suite *HandlerTestSuite) TestParsePaginationParams_InvalidOffset() {
	req := httptest.NewRequest("GET", "/test?offset=invalid", nil)

	_, _, err := parsePaginationParams(req.URL.Query())

	suite.NotNil(err)
	suite.Equal(ErrorInvalidOffset.Code, err.Code)
}

func (suite *HandlerTestSuite) TestParsePaginationParams_NegativeOffset() {
	req := httptest.NewRequest("GET", "/test?offset=-1", nil)

	_, _, err := parsePaginationParams(req.URL.Query())

	suite.NotNil(err)
	suite.Equal(ErrorInvalidOffset.Code, err.Code)
}

// Sanitization Tests

func (suite *HandlerTestSuite) TestSanitizeCreateResourceServerRequest() {
	input := &CreateResourceServerRequest{
		Name:        "  test-rs  ",
		Description: "  Test Description  ",
		OUID:        "  ou-123  ",
	}

	result := sanitizeCreateResourceServerRequest(input)

	// Verify exact trimmed values
	suite.Equal("test-rs", result.Name)
	suite.Equal("Test Description", result.Description)
	suite.Equal("ou-123", result.OUID)
}

func (suite *HandlerTestSuite) TestSanitizeCreateResourceRequest_WithParent() {
	parentID := "  parent-123  "
	input := &CreateResourceRequest{
		Name:   "  test-resource  ",
		Handle: "  test-handle  ",
		Parent: &parentID,
	}

	result := sanitizeCreateResourceRequest(input)

	suite.NotNil(result.Parent)
	suite.Equal("test-resource", result.Name)
	suite.Equal("test-handle", result.Handle)
	suite.Equal("parent-123", *result.Parent)
}

func (suite *HandlerTestSuite) TestSanitizeCreateResourceRequest_NullParent() {
	input := &CreateResourceRequest{
		Name:   "test-resource",
		Handle: "test-handle",
		Parent: nil,
	}

	result := sanitizeCreateResourceRequest(input)

	suite.Nil(result.Parent)
	suite.Equal("test-handle", result.Handle)
}

// Error Handling Tests

func (suite *HandlerTestSuite) TestHandleError_NotFoundStatus() {
	suite.mockService.On("GetResourceServer", mock.Anything,
		"rs-123").Return(nil, &ErrorResourceServerNotFound)

	req := httptest.NewRequest("GET", "/resource-servers/rs-123", nil)
	req.SetPathValue("id", "rs-123")
	w := httptest.NewRecorder()

	suite.handler.HandleResourceServerGetRequest(w, req)

	suite.Equal(http.StatusNotFound, w.Code)
}

func (suite *HandlerTestSuite) TestHandleError_ConflictStatus() {
	reqBody := CreateResourceServerRequest{
		Name: "test-rs",
		OUID: "ou-123",
	}

	suite.mockService.On("CreateResourceServer", mock.Anything,
		mock.Anything).Return(nil, &ErrorHandleConflict)

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/resource-servers", bytes.NewReader(body))
	w := httptest.NewRecorder()

	suite.handler.HandleResourceServerPostRequest(w, req)

	suite.Equal(http.StatusConflict, w.Code)
}

func (suite *HandlerTestSuite) TestHandleError_BadRequestStatus() {
	reqBody := CreateResourceServerRequest{
		Name: "",
		OUID: "ou-123",
	}

	suite.mockService.On("CreateResourceServer", mock.Anything,
		mock.Anything).Return(nil, &ErrorInvalidRequestFormat)

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/resource-servers", bytes.NewReader(body))
	w := httptest.NewRecorder()

	suite.handler.HandleResourceServerPostRequest(w, req)

	suite.Equal(http.StatusBadRequest, w.Code)
}

// HandleActionPutAtResourceRequest Tests

func (suite *HandlerTestSuite) TestHandleActionPutAtResourceRequest_Success() {
	reqBody := UpdateActionRequest{
		Description: "Updated description",
	}

	resourceID := testResourceID
	suite.mockService.On("UpdateAction", mock.Anything,
		"rs-123", &resourceID, "action-123", mock.Anything).Return(&Action{
		ID:          "action-123",
		Name:        "test-action",
		Description: "Updated description",
	}, nil)

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(
		"PUT", "/resource-servers/rs-123/resources/res-123/actions/action-123", bytes.NewReader(body),
	)
	req.SetPathValue("rsId", "rs-123")
	req.SetPathValue("resourceId", testResourceID)
	req.SetPathValue("id", "action-123")
	w := httptest.NewRecorder()

	suite.handler.HandleActionPutAtResourceRequest(w, req)

	suite.Equal(http.StatusOK, w.Code)
	var resp ActionResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	suite.NoError(err)
	suite.Equal("action-123", resp.ID)
	suite.Equal("Updated description", resp.Description)
}

func (suite *HandlerTestSuite) TestHandleActionPutAtResourceRequest_InvalidJSON() {
	req := httptest.NewRequest(
		"PUT", "/resource-servers/rs-123/resources/res-123/actions/action-123",
		bytes.NewReader([]byte("invalid json")),
	)
	req.SetPathValue("rsId", "rs-123")
	req.SetPathValue("resourceId", testResourceID)
	req.SetPathValue("id", "action-123")
	w := httptest.NewRecorder()

	suite.handler.HandleActionPutAtResourceRequest(w, req)

	suite.Equal(http.StatusBadRequest, w.Code)
}

func (suite *HandlerTestSuite) TestHandleActionPutAtResourceRequest_NotFound() {
	reqBody := UpdateActionRequest{
		Description: "Updated description",
	}

	resourceID := testResourceID
	suite.mockService.On("UpdateAction", mock.Anything,
		"rs-123", &resourceID, "action-123",
		mock.Anything).Return(nil, &ErrorActionNotFound)

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(
		"PUT", "/resource-servers/rs-123/resources/res-123/actions/action-123", bytes.NewReader(body),
	)
	req.SetPathValue("rsId", "rs-123")
	req.SetPathValue("resourceId", testResourceID)
	req.SetPathValue("id", "action-123")
	w := httptest.NewRecorder()

	suite.handler.HandleActionPutAtResourceRequest(w, req)

	suite.Equal(http.StatusNotFound, w.Code)
}

func (suite *HandlerTestSuite) TestHandleActionPutAtResourceRequest_ServiceError() {
	reqBody := UpdateActionRequest{
		Description: "Updated description",
	}

	resourceID := testResourceID
	suite.mockService.On("UpdateAction", mock.Anything,
		"rs-123", &resourceID, "action-123",
		mock.Anything).Return(nil, &serviceerror.InternalServerError)

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(
		"PUT", "/resource-servers/rs-123/resources/res-123/actions/action-123", bytes.NewReader(body),
	)
	req.SetPathValue("rsId", "rs-123")
	req.SetPathValue("resourceId", testResourceID)
	req.SetPathValue("id", "action-123")
	w := httptest.NewRecorder()

	suite.handler.HandleActionPutAtResourceRequest(w, req)

	suite.Equal(http.StatusInternalServerError, w.Code)
}

// HandleActionDeleteAtResourceRequest Tests

func (suite *HandlerTestSuite) TestHandleActionDeleteAtResourceRequest_Success() {
	resourceID := testResourceID
	suite.mockService.On("DeleteAction", mock.Anything,
		"rs-123", &resourceID, "action-123").Return(nil)

	req := httptest.NewRequest("DELETE", "/resource-servers/rs-123/resources/res-123/actions/action-123", nil)
	req.SetPathValue("rsId", "rs-123")
	req.SetPathValue("resourceId", testResourceID)
	req.SetPathValue("id", "action-123")
	w := httptest.NewRecorder()

	suite.handler.HandleActionDeleteAtResourceRequest(w, req)

	suite.Equal(http.StatusNoContent, w.Code)
}

func (suite *HandlerTestSuite) TestHandleActionDeleteAtResourceRequest_NotFound() {
	resourceID := testResourceID
	suite.mockService.On("DeleteAction", mock.Anything,
		"rs-123", &resourceID, "action-123").Return(&ErrorActionNotFound)

	req := httptest.NewRequest("DELETE", "/resource-servers/rs-123/resources/res-123/actions/action-123", nil)
	req.SetPathValue("rsId", "rs-123")
	req.SetPathValue("resourceId", testResourceID)
	req.SetPathValue("id", "action-123")
	w := httptest.NewRecorder()

	suite.handler.HandleActionDeleteAtResourceRequest(w, req)

	suite.Equal(http.StatusNotFound, w.Code)
}

func (suite *HandlerTestSuite) TestHandleActionDeleteAtResourceRequest_ServiceError() {
	resourceID := testResourceID
	suite.mockService.On("DeleteAction", mock.Anything,
		"rs-123", &resourceID, "action-123").
		Return(&serviceerror.InternalServerError)

	req := httptest.NewRequest("DELETE", "/resource-servers/rs-123/resources/res-123/actions/action-123", nil)
	req.SetPathValue("rsId", "rs-123")
	req.SetPathValue("resourceId", testResourceID)
	req.SetPathValue("id", "action-123")
	w := httptest.NewRecorder()

	suite.handler.HandleActionDeleteAtResourceRequest(w, req)

	suite.Equal(http.StatusInternalServerError, w.Code)
}

// Additional edge case tests for better coverage

func (suite *HandlerTestSuite) TestHandleResourceServerPutRequest_InvalidJSON() {
	req := httptest.NewRequest("PUT", "/resource-servers/rs-123", bytes.NewReader([]byte("invalid json")))
	req.SetPathValue("id", "rs-123")
	w := httptest.NewRecorder()

	suite.handler.HandleResourceServerPutRequest(w, req)

	suite.Equal(http.StatusBadRequest, w.Code)
}

func (suite *HandlerTestSuite) TestHandleResourceServerPutRequest_ServiceError() {
	reqBody := UpdateResourceServerRequest{
		Name: "updated-rs",
		OUID: "ou-123",
	}

	suite.mockService.On("UpdateResourceServer", mock.Anything,
		"rs-123", mock.Anything).Return(nil, &serviceerror.InternalServerError)

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("PUT", "/resource-servers/rs-123", bytes.NewReader(body))
	req.SetPathValue("id", "rs-123")
	w := httptest.NewRecorder()

	suite.handler.HandleResourceServerPutRequest(w, req)

	suite.Equal(http.StatusInternalServerError, w.Code)
}

func (suite *HandlerTestSuite) TestHandleResourceServerDeleteRequest_ServiceError() {
	suite.mockService.On("DeleteResourceServer", mock.Anything,
		"rs-123").Return(&serviceerror.InternalServerError)

	req := httptest.NewRequest("DELETE", "/resource-servers/rs-123", nil)
	req.SetPathValue("id", "rs-123")
	w := httptest.NewRecorder()

	suite.handler.HandleResourceServerDeleteRequest(w, req)

	suite.Equal(http.StatusInternalServerError, w.Code)
}

func (suite *HandlerTestSuite) TestHandleResourceListRequest_InvalidLimit() {
	req := httptest.NewRequest("GET", "/resource-servers/rs-123/resources?limit=invalid", nil)
	req.SetPathValue("rsId", "rs-123")
	w := httptest.NewRecorder()

	suite.handler.HandleResourceListRequest(w, req)

	suite.Equal(http.StatusBadRequest, w.Code)
}

func (suite *HandlerTestSuite) TestHandleResourceListRequest_ServiceError() {
	suite.mockService.On("GetResourceList", mock.Anything,
		"rs-123", (*string)(nil), 30, 0).
		Return(nil, &serviceerror.InternalServerError)

	req := httptest.NewRequest("GET", "/resource-servers/rs-123/resources", nil)
	req.SetPathValue("rsId", "rs-123")
	w := httptest.NewRecorder()

	suite.handler.HandleResourceListRequest(w, req)

	suite.Equal(http.StatusInternalServerError, w.Code)
}

func (suite *HandlerTestSuite) TestHandleResourcePostRequest_InvalidJSON() {
	req := httptest.NewRequest("POST", "/resource-servers/rs-123/resources", bytes.NewReader([]byte("invalid json")))
	req.SetPathValue("rsId", "rs-123")
	w := httptest.NewRecorder()

	suite.handler.HandleResourcePostRequest(w, req)

	suite.Equal(http.StatusBadRequest, w.Code)
}

func (suite *HandlerTestSuite) TestHandleResourcePostRequest_ServiceError() {
	reqBody := CreateResourceRequest{
		Name:   "test-resource",
		Handle: "test-handle",
	}

	suite.mockService.On("CreateResource", mock.Anything,
		"rs-123", mock.Anything).Return(nil, &serviceerror.InternalServerError)

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/resource-servers/rs-123/resources", bytes.NewReader(body))
	req.SetPathValue("rsId", "rs-123")
	w := httptest.NewRecorder()

	suite.handler.HandleResourcePostRequest(w, req)

	suite.Equal(http.StatusInternalServerError, w.Code)
}

func (suite *HandlerTestSuite) TestHandleResourceGetRequest_NotFound() {
	suite.mockService.On("GetResource", mock.Anything,
		"rs-123", "res-123").Return(nil, &ErrorResourceNotFound)

	req := httptest.NewRequest("GET", "/resource-servers/rs-123/resources/res-123", nil)
	req.SetPathValue("rsId", "rs-123")
	req.SetPathValue("id", "res-123")
	w := httptest.NewRecorder()

	suite.handler.HandleResourceGetRequest(w, req)

	suite.Equal(http.StatusNotFound, w.Code)
}

func (suite *HandlerTestSuite) TestHandleResourceGetRequest_ServiceError() {
	suite.mockService.On("GetResource", mock.Anything,
		"rs-123", "res-123").Return(nil, &serviceerror.InternalServerError)

	req := httptest.NewRequest("GET", "/resource-servers/rs-123/resources/res-123", nil)
	req.SetPathValue("rsId", "rs-123")
	req.SetPathValue("id", "res-123")
	w := httptest.NewRecorder()

	suite.handler.HandleResourceGetRequest(w, req)

	suite.Equal(http.StatusInternalServerError, w.Code)
}

func (suite *HandlerTestSuite) TestHandleResourcePutRequest_InvalidJSON() {
	req := httptest.NewRequest(
		"PUT", "/resource-servers/rs-123/resources/res-123", bytes.NewReader([]byte("invalid json")),
	)
	req.SetPathValue("rsId", "rs-123")
	req.SetPathValue("id", "res-123")
	w := httptest.NewRecorder()

	suite.handler.HandleResourcePutRequest(w, req)

	suite.Equal(http.StatusBadRequest, w.Code)
}

func (suite *HandlerTestSuite) TestHandleResourcePutRequest_ServiceError() {
	reqBody := UpdateResourceRequest{
		Description: "updated",
	}

	suite.mockService.On("UpdateResource", mock.Anything,
		"rs-123", "res-123", mock.Anything).
		Return(nil, &serviceerror.InternalServerError)

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("PUT", "/resource-servers/rs-123/resources/res-123", bytes.NewReader(body))
	req.SetPathValue("rsId", "rs-123")
	req.SetPathValue("id", "res-123")
	w := httptest.NewRecorder()

	suite.handler.HandleResourcePutRequest(w, req)

	suite.Equal(http.StatusInternalServerError, w.Code)
}

func (suite *HandlerTestSuite) TestHandleResourceDeleteRequest_ServiceError() {
	suite.mockService.On("DeleteResource", mock.Anything,
		"rs-123", "res-123").Return(&serviceerror.InternalServerError)

	req := httptest.NewRequest("DELETE", "/resource-servers/rs-123/resources/res-123", nil)
	req.SetPathValue("rsId", "rs-123")
	req.SetPathValue("id", "res-123")
	w := httptest.NewRecorder()

	suite.handler.HandleResourceDeleteRequest(w, req)

	suite.Equal(http.StatusInternalServerError, w.Code)
}

func (suite *HandlerTestSuite) TestHandleActionListAtResourceServerRequest_InvalidLimit() {
	req := httptest.NewRequest("GET", "/resource-servers/rs-123/actions?limit=invalid", nil)
	req.SetPathValue("rsId", "rs-123")
	w := httptest.NewRecorder()

	suite.handler.HandleActionListAtResourceServerRequest(w, req)

	suite.Equal(http.StatusBadRequest, w.Code)
}

func (suite *HandlerTestSuite) TestHandleActionListAtResourceServerRequest_ServiceError() {
	var nilResourceID *string
	suite.mockService.On("GetActionList", mock.Anything,
		"rs-123", nilResourceID, 30, 0).Return(nil, &serviceerror.InternalServerError)

	req := httptest.NewRequest("GET", "/resource-servers/rs-123/actions", nil)
	req.SetPathValue("rsId", "rs-123")
	w := httptest.NewRecorder()

	suite.handler.HandleActionListAtResourceServerRequest(w, req)

	suite.Equal(http.StatusInternalServerError, w.Code)
}

func (suite *HandlerTestSuite) TestHandleActionPostAtResourceServerRequest_InvalidJSON() {
	req := httptest.NewRequest("POST", "/resource-servers/rs-123/actions", bytes.NewReader([]byte("invalid json")))
	req.SetPathValue("rsId", "rs-123")
	w := httptest.NewRecorder()

	suite.handler.HandleActionPostAtResourceServerRequest(w, req)

	suite.Equal(http.StatusBadRequest, w.Code)
}

func (suite *HandlerTestSuite) TestHandleActionPostAtResourceServerRequest_ServiceError() {
	reqBody := CreateActionRequest{
		Name:   "test-action",
		Handle: "test-handle",
	}

	var nilResourceID *string
	suite.mockService.On("CreateAction", mock.Anything,
		"rs-123", nilResourceID, mock.Anything).
		Return(nil, &serviceerror.InternalServerError)

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/resource-servers/rs-123/actions", bytes.NewReader(body))
	req.SetPathValue("rsId", "rs-123")
	w := httptest.NewRecorder()

	suite.handler.HandleActionPostAtResourceServerRequest(w, req)

	suite.Equal(http.StatusInternalServerError, w.Code)
}

func (suite *HandlerTestSuite) TestHandleActionGetAtResourceServerRequest_NotFound() {
	var nilResourceID *string
	suite.mockService.On("GetAction", mock.Anything,
		"rs-123", nilResourceID, "action-123").Return(nil, &ErrorActionNotFound)

	req := httptest.NewRequest("GET", "/resource-servers/rs-123/actions/action-123", nil)
	req.SetPathValue("rsId", "rs-123")
	req.SetPathValue("id", "action-123")
	w := httptest.NewRecorder()

	suite.handler.HandleActionGetAtResourceServerRequest(w, req)

	suite.Equal(http.StatusNotFound, w.Code)
}

func (suite *HandlerTestSuite) TestHandleActionGetAtResourceServerRequest_ServiceError() {
	var nilResourceID *string
	suite.mockService.On("GetAction", mock.Anything,
		"rs-123", nilResourceID, "action-123").
		Return(nil, &serviceerror.InternalServerError)

	req := httptest.NewRequest("GET", "/resource-servers/rs-123/actions/action-123", nil)
	req.SetPathValue("rsId", "rs-123")
	req.SetPathValue("id", "action-123")
	w := httptest.NewRecorder()

	suite.handler.HandleActionGetAtResourceServerRequest(w, req)

	suite.Equal(http.StatusInternalServerError, w.Code)
}

func (suite *HandlerTestSuite) TestHandleActionPutAtResourceServerRequest_InvalidJSON() {
	req := httptest.NewRequest(
		"PUT", "/resource-servers/rs-123/actions/action-123", bytes.NewReader([]byte("invalid json")),
	)
	req.SetPathValue("rsId", "rs-123")
	req.SetPathValue("id", "action-123")
	w := httptest.NewRecorder()

	suite.handler.HandleActionPutAtResourceServerRequest(w, req)

	suite.Equal(http.StatusBadRequest, w.Code)
}

func (suite *HandlerTestSuite) TestHandleActionPutAtResourceServerRequest_ServiceError() {
	reqBody := UpdateActionRequest{
		Description: "updated",
	}

	var nilResourceID *string
	suite.mockService.On("UpdateAction", mock.Anything,
		"rs-123", nilResourceID, "action-123",
		mock.Anything).Return(nil, &serviceerror.InternalServerError)

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("PUT", "/resource-servers/rs-123/actions/action-123", bytes.NewReader(body))
	req.SetPathValue("rsId", "rs-123")
	req.SetPathValue("id", "action-123")
	w := httptest.NewRecorder()

	suite.handler.HandleActionPutAtResourceServerRequest(w, req)

	suite.Equal(http.StatusInternalServerError, w.Code)
}

func (suite *HandlerTestSuite) TestHandleActionDeleteAtResourceServerRequest_ServiceError() {
	var nilResourceID *string
	suite.mockService.On("DeleteAction", mock.Anything,
		"rs-123", nilResourceID, "action-123").
		Return(&serviceerror.InternalServerError)

	req := httptest.NewRequest("DELETE", "/resource-servers/rs-123/actions/action-123", nil)
	req.SetPathValue("rsId", "rs-123")
	req.SetPathValue("id", "action-123")
	w := httptest.NewRecorder()

	suite.handler.HandleActionDeleteAtResourceServerRequest(w, req)

	suite.Equal(http.StatusInternalServerError, w.Code)
}

func (suite *HandlerTestSuite) TestHandleActionListAtResourceRequest_InvalidLimit() {
	req := httptest.NewRequest("GET", "/resource-servers/rs-123/resources/res-123/actions?limit=invalid", nil)
	req.SetPathValue("rsId", "rs-123")
	req.SetPathValue("resourceId", testResourceID)
	w := httptest.NewRecorder()

	suite.handler.HandleActionListAtResourceRequest(w, req)

	suite.Equal(http.StatusBadRequest, w.Code)
}

func (suite *HandlerTestSuite) TestHandleActionListAtResourceRequest_ServiceError() {
	resourceID := testResourceID
	suite.mockService.On("GetActionList", mock.Anything,
		"rs-123", &resourceID, 30, 0).
		Return(nil, &serviceerror.InternalServerError)

	req := httptest.NewRequest("GET", "/resource-servers/rs-123/resources/res-123/actions", nil)
	req.SetPathValue("rsId", "rs-123")
	req.SetPathValue("resourceId", testResourceID)
	w := httptest.NewRecorder()

	suite.handler.HandleActionListAtResourceRequest(w, req)

	suite.Equal(http.StatusInternalServerError, w.Code)
}

func (suite *HandlerTestSuite) TestHandleActionPostAtResourceRequest_InvalidJSON() {
	req := httptest.NewRequest(
		"POST", "/resource-servers/rs-123/resources/res-123/actions", bytes.NewReader([]byte("invalid json")),
	)
	req.SetPathValue("rsId", "rs-123")
	req.SetPathValue("resourceId", testResourceID)
	w := httptest.NewRecorder()

	suite.handler.HandleActionPostAtResourceRequest(w, req)

	suite.Equal(http.StatusBadRequest, w.Code)
}

func (suite *HandlerTestSuite) TestHandleActionPostAtResourceRequest_ServiceError() {
	reqBody := CreateActionRequest{
		Name:   "test-action",
		Handle: "test-handle",
	}

	resourceID := testResourceID
	suite.mockService.On("CreateAction", mock.Anything,
		"rs-123", &resourceID, mock.Anything).
		Return(nil, &serviceerror.InternalServerError)

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/resource-servers/rs-123/resources/res-123/actions", bytes.NewReader(body))
	req.SetPathValue("rsId", "rs-123")
	req.SetPathValue("resourceId", testResourceID)
	w := httptest.NewRecorder()

	suite.handler.HandleActionPostAtResourceRequest(w, req)

	suite.Equal(http.StatusInternalServerError, w.Code)
}

func (suite *HandlerTestSuite) TestHandleActionGetAtResourceRequest_NotFound() {
	resourceID := testResourceID
	suite.mockService.On("GetAction", mock.Anything,
		"rs-123", &resourceID, "action-123").Return(nil, &ErrorActionNotFound)

	req := httptest.NewRequest("GET", "/resource-servers/rs-123/resources/res-123/actions/action-123", nil)
	req.SetPathValue("rsId", "rs-123")
	req.SetPathValue("resourceId", testResourceID)
	req.SetPathValue("id", "action-123")
	w := httptest.NewRecorder()

	suite.handler.HandleActionGetAtResourceRequest(w, req)

	suite.Equal(http.StatusNotFound, w.Code)
}

func (suite *HandlerTestSuite) TestHandleActionGetAtResourceRequest_ServiceError() {
	resourceID := testResourceID
	suite.mockService.On("GetAction", mock.Anything,
		"rs-123", &resourceID, "action-123").
		Return(nil, &serviceerror.InternalServerError)

	req := httptest.NewRequest("GET", "/resource-servers/rs-123/resources/res-123/actions/action-123", nil)
	req.SetPathValue("rsId", "rs-123")
	req.SetPathValue("resourceId", testResourceID)
	req.SetPathValue("id", "action-123")
	w := httptest.NewRecorder()

	suite.handler.HandleActionGetAtResourceRequest(w, req)

	suite.Equal(http.StatusInternalServerError, w.Code)
}
