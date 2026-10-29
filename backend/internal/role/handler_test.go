// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

type RoleHandlerTestSuite struct {
	suite.Suite
	mockService           *RoleServiceInterfaceMock
	mockAssignmentService *RoleAssignmentServiceInterfaceMock
	handler               *roleHandler
}

func TestRoleHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(RoleHandlerTestSuite))
}

func (suite *RoleHandlerTestSuite) SetupTest() {
	suite.mockService = NewRoleServiceInterfaceMock(suite.T())
	suite.mockAssignmentService = NewRoleAssignmentServiceInterfaceMock(suite.T())
	suite.handler = newRoleHandler(suite.mockService, suite.mockAssignmentService)
}

// HandleRoleListRequest Tests
func (suite *RoleHandlerTestSuite) TestHandleRoleListRequest_Success() {
	expectedResponse := &RoleList{
		TotalResults: 2,
		StartIndex:   1,
		Count:        2,
		Roles: []Role{
			{ID: "role1", Name: "Admin"},
			{ID: "role2", Name: "User"},
		},
		Links: []utils.Link{},
	}

	suite.mockService.On("GetRoleList", mock.Anything, 10, 0).Return(expectedResponse, nil)

	req := httptest.NewRequest(http.MethodGet, "/roles?limit=10&offset=0", nil)
	w := httptest.NewRecorder()

	suite.handler.HandleRoleListRequest(w, req)

	suite.Equal(http.StatusOK, w.Code)

	var response RoleListResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	suite.NoError(err)
	suite.Equal(2, response.TotalResults)
	suite.Equal(2, len(response.Roles))
}

func (suite *RoleHandlerTestSuite) TestHandleRoleListRequest_DefaultPagination() {
	expectedResponse := &RoleList{
		TotalResults: 1,
		StartIndex:   1,
		Count:        1,
		Roles:        []Role{{ID: "role1", Name: "Admin"}},
		Links:        []utils.Link{},
	}

	suite.mockService.On("GetRoleList", mock.Anything, 30, 0).Return(expectedResponse, nil)

	req := httptest.NewRequest(http.MethodGet, "/roles", nil)
	w := httptest.NewRecorder()

	suite.handler.HandleRoleListRequest(w, req)

	suite.Equal(http.StatusOK, w.Code)
}

func (suite *RoleHandlerTestSuite) TestHandleRoleListRequest_ServiceError() {
	suite.mockService.On("GetRoleList", mock.Anything, 10, 0).Return(nil, &ErrorInvalidLimit)

	req := httptest.NewRequest(http.MethodGet, "/roles?limit=10&offset=0", nil)
	w := httptest.NewRecorder()

	suite.handler.HandleRoleListRequest(w, req)

	suite.Equal(http.StatusBadRequest, w.Code)
}

// HandleRolePostRequest Tests
func (suite *RoleHandlerTestSuite) TestHandleRolePostRequest_Success() {
	request := CreateRoleRequest{
		Name:        "Test Role",
		Description: "Description",
		OUID:        "ou1",
		Permissions: []ResourcePermissions{{ResourceServerID: "rs1", Permissions: []string{"perm1"}}},
	}

	expectedRole := &RoleWithPermissionsAndAssignments{
		ID:          "role1",
		Name:        "Test Role",
		Description: "Description",
		OUID:        "ou1",
		Permissions: []ResourcePermissions{{ResourceServerID: "rs1", Permissions: []string{"perm1"}}},
	}

	suite.mockService.On("CreateRole", mock.Anything, mock.AnythingOfType("RoleCreationDetail")).
		Return(expectedRole, nil)

	body, _ := json.Marshal(request)
	req := httptest.NewRequest(http.MethodPost, "/roles", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	suite.handler.HandleRolePostRequest(w, req)

	suite.Equal(http.StatusCreated, w.Code)

	var response CreateRoleResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	suite.NoError(err)
	suite.Equal("role1", response.ID)
	suite.Equal("Test Role", response.Name)
}

func (suite *RoleHandlerTestSuite) TestHandleRolePostRequest_WithoutPermissions_ReturnsEmptyArray() {
	request := CreateRoleRequest{
		Name: "Test Role",
		OUID: "ou1",
	}

	expectedRole := &RoleWithPermissionsAndAssignments{
		ID:   "role1",
		Name: "Test Role",
		OUID: "ou1",
	}

	suite.mockService.On("CreateRole", mock.Anything, mock.AnythingOfType("RoleCreationDetail")).
		Return(expectedRole, nil)

	body, _ := json.Marshal(request)
	req := httptest.NewRequest(http.MethodPost, "/roles", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	suite.handler.HandleRolePostRequest(w, req)

	suite.Equal(http.StatusCreated, w.Code)

	var response CreateRoleResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	suite.NoError(err)
	suite.NotNil(response.Permissions)
	suite.Empty(response.Permissions)
}

func (suite *RoleHandlerTestSuite) TestHandleRolePostRequest_InvalidJSON() {
	req := httptest.NewRequest(http.MethodPost, "/roles", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	suite.handler.HandleRolePostRequest(w, req)

	suite.Equal(http.StatusBadRequest, w.Code)
}

func (suite *RoleHandlerTestSuite) TestHandleRolePostRequest_ServiceError() {
	request := CreateRoleRequest{
		Name:        "Test Role",
		OUID:        "ou1",
		Permissions: []ResourcePermissions{{ResourceServerID: "rs1", Permissions: []string{"perm1"}}},
	}

	suite.mockService.On("CreateRole", mock.Anything, mock.AnythingOfType("RoleCreationDetail")).
		Return(nil, &ErrorOrganizationUnitNotFound)

	body, _ := json.Marshal(request)
	req := httptest.NewRequest(http.MethodPost, "/roles", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	suite.handler.HandleRolePostRequest(w, req)

	suite.Equal(http.StatusBadRequest, w.Code)
}

func (suite *RoleHandlerTestSuite) TestHandleRolePostRequest_ValidationError_NameTooLong() {
	request := CreateRoleRequest{
		Name: strings.Repeat("a", 101),
		OUID: "ou-tenancy-1",
	}

	body, _ := json.Marshal(request)
	req := httptest.NewRequest(http.MethodPost, "/roles", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	suite.handler.HandleRolePostRequest(w, req)

	suite.Equal(http.StatusBadRequest, w.Code)
	suite.Contains(w.Body.String(), "INVALID_INPUT_METADATA")

	suite.mockService.AssertNotCalled(suite.T(), "CreateRole", mock.Anything, mock.Anything)
}

func (suite *RoleHandlerTestSuite) TestHandleRoleAddAssignmentsRequest_ValidationError_TypeSpoof() {
	body := `{"assignments":[]}`
	req := httptest.NewRequest(http.MethodPost, "/roles/role1/add-assignments", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "role1")
	w := httptest.NewRecorder()

	suite.handler.HandleRoleAddAssignmentsRequest(w, req)

	suite.Equal(http.StatusBadRequest, w.Code)
	suite.Contains(w.Body.String(), "Invalid request format")

	suite.mockAssignmentService.AssertNotCalled(
		suite.T(), "AddAssignments", mock.Anything, mock.Anything, mock.Anything)
}

// HandleRoleGetRequest Tests
func (suite *RoleHandlerTestSuite) TestHandleRoleGetRequest_Success() {
	expectedRole := &RoleWithPermissions{
		ID:          "role1",
		Name:        "Admin",
		Description: "Admin role",
		OUID:        "ou1",
		Permissions: []ResourcePermissions{{ResourceServerID: "rs1", Permissions: []string{"perm1"}}},
	}

	suite.mockService.On("GetRoleWithPermissions", mock.Anything, "role1").Return(expectedRole, nil)

	req := httptest.NewRequest(http.MethodGet, "/roles/role1", nil)
	req.SetPathValue("id", "role1")
	w := httptest.NewRecorder()

	suite.handler.HandleRoleGetRequest(w, req)

	suite.Equal(http.StatusOK, w.Code)

	var response RoleResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	suite.NoError(err)
	suite.Equal("role1", response.ID)
	suite.Equal("Admin", response.Name)
}

func (suite *RoleHandlerTestSuite) TestHandleRoleGetRequest_MissingID() {
	suite.mockService.On("GetRoleWithPermissions", mock.Anything, "").Return(nil, &ErrorMissingRoleID)

	req := httptest.NewRequest(http.MethodGet, "/roles/", nil)
	w := httptest.NewRecorder()

	suite.handler.HandleRoleGetRequest(w, req)

	suite.Equal(http.StatusBadRequest, w.Code)
}

func (suite *RoleHandlerTestSuite) TestHandleRoleGetRequest_NotFound() {
	suite.mockService.On("GetRoleWithPermissions", mock.Anything, "nonexistent").Return(nil, &ErrorRoleNotFound)

	req := httptest.NewRequest(http.MethodGet, "/roles/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	suite.handler.HandleRoleGetRequest(w, req)

	suite.Equal(http.StatusNotFound, w.Code)
}

// HandleRolePutRequest Tests
func (suite *RoleHandlerTestSuite) TestHandleRolePutRequest_Success() {
	request := UpdateRoleRequest{
		Name:        "Updated Role",
		OUID:        "ou1",
		Permissions: []ResourcePermissions{{ResourceServerID: "rs1", Permissions: []string{"perm1", "perm2"}}},
	}

	updatedRole := &RoleWithPermissions{
		ID:          "role1",
		Name:        "Updated Role",
		OUID:        "ou1",
		Permissions: []ResourcePermissions{{ResourceServerID: "rs1", Permissions: []string{"perm1", "perm2"}}},
	}

	suite.mockService.On("UpdateRoleWithPermissions", mock.Anything, "role1",
		mock.AnythingOfType("RoleUpdateDetail")).Return(updatedRole, nil)

	body, _ := json.Marshal(request)
	req := httptest.NewRequest(http.MethodPut, "/roles/role1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "role1")
	w := httptest.NewRecorder()

	suite.handler.HandleRolePutRequest(w, req)

	suite.Equal(http.StatusOK, w.Code)

	var response RoleResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	suite.NoError(err)
	suite.Equal("Updated Role", response.Name)
}

func (suite *RoleHandlerTestSuite) TestHandleRolePutRequest_InvalidJSON() {
	req := httptest.NewRequest(http.MethodPut, "/roles/role1", bytes.NewBufferString("invalid"))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "role1")
	w := httptest.NewRecorder()

	suite.handler.HandleRolePutRequest(w, req)

	suite.Equal(http.StatusBadRequest, w.Code)
}

// HandleRoleDeleteRequest Tests
func (suite *RoleHandlerTestSuite) TestHandleRoleDeleteRequest_Success() {
	suite.mockService.On("DeleteRole", mock.Anything, "role1").Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/roles/role1", nil)
	req.SetPathValue("id", "role1")
	w := httptest.NewRecorder()

	suite.handler.HandleRoleDeleteRequest(w, req)

	suite.Equal(http.StatusNoContent, w.Code)
}

// HandleRoleAssignmentsGetRequest Tests
func (suite *RoleHandlerTestSuite) TestHandleRoleAssignmentsGetRequest_Success() {
	expectedResponse := &AssignmentList{
		TotalResults: 2,
		StartIndex:   1,
		Count:        2,
		Assignments: []RoleAssignmentWithDisplay{
			{ID: "user1", Type: AssigneeTypeUser},
			{ID: "group1", Type: AssigneeTypeGroup},
		},
		Links: []utils.Link{},
	}

	suite.mockAssignmentService.On("GetRoleAssignments", mock.Anything, "role1", 10, 0, false).
		Return(expectedResponse, nil)

	req := httptest.NewRequest(http.MethodGet, "/roles/role1/assignments?limit=10&offset=0", nil)
	req.SetPathValue("id", "role1")
	w := httptest.NewRecorder()

	suite.handler.HandleRoleAssignmentsGetRequest(w, req)

	suite.Equal(http.StatusOK, w.Code)

	var response AssignmentListResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	suite.NoError(err)
	suite.Equal(2, response.TotalResults)
	suite.Equal(2, len(response.Assignments))
}

func (suite *RoleHandlerTestSuite) TestHandleRoleAssignmentsGetRequest_RoleNotFound() {
	suite.mockAssignmentService.On("GetRoleAssignments", mock.Anything, "nonexistent", 30, 0, false).
		Return(nil, &ErrorRoleNotFound)

	req := httptest.NewRequest(http.MethodGet, "/roles/nonexistent/assignments", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	suite.handler.HandleRoleAssignmentsGetRequest(w, req)

	suite.Equal(http.StatusNotFound, w.Code)
}

// HandleRoleAddAssignmentsRequest Tests
func (suite *RoleHandlerTestSuite) TestHandleRoleAddAssignmentsRequest_Success() {
	request := AssignmentsRequest{
		Assignments: []AssignmentRequest{
			{ID: "user1", Type: AssigneeTypeUser},
		},
	}

	suite.mockAssignmentService.On(
		"AddAssignments", mock.Anything, "role1", mock.AnythingOfType("[]role.RoleAssignment"),
	).Return(nil)

	body, _ := json.Marshal(request)
	req := httptest.NewRequest(http.MethodPost, "/roles/role1/add-assignments", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "role1")
	w := httptest.NewRecorder()

	suite.handler.HandleRoleAddAssignmentsRequest(w, req)

	suite.Equal(http.StatusNoContent, w.Code)
}

func (suite *RoleHandlerTestSuite) TestHandleRoleAddAssignmentsRequest_InvalidJSON() {
	req := httptest.NewRequest(http.MethodPost, "/roles/role1/add-assignments", bytes.NewBufferString("invalid"))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "role1")
	w := httptest.NewRecorder()

	suite.handler.HandleRoleAddAssignmentsRequest(w, req)

	suite.Equal(http.StatusBadRequest, w.Code)
}

func (suite *RoleHandlerTestSuite) TestHandleRoleAddAssignmentsRequest_ServiceError() {
	request := AssignmentsRequest{
		Assignments: []AssignmentRequest{
			{ID: "invalid_user", Type: AssigneeTypeUser},
		},
	}

	suite.mockAssignmentService.On(
		"AddAssignments", mock.Anything, "role1", mock.AnythingOfType("[]role.RoleAssignment"),
	).Return(&ErrorInvalidAssignmentID)

	body, _ := json.Marshal(request)
	req := httptest.NewRequest(http.MethodPost, "/roles/role1/add-assignments", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "role1")
	w := httptest.NewRecorder()

	suite.handler.HandleRoleAddAssignmentsRequest(w, req)

	suite.Equal(http.StatusBadRequest, w.Code)
}

// HandleRoleRemoveAssignmentsRequest Tests
func (suite *RoleHandlerTestSuite) TestHandleRoleRemoveAssignmentsRequest_Success() {
	request := AssignmentsRequest{
		Assignments: []AssignmentRequest{
			{ID: "user1", Type: AssigneeTypeUser},
		},
	}

	suite.mockAssignmentService.On(
		"RemoveAssignments", mock.Anything, "role1", mock.AnythingOfType("[]role.RoleAssignment"),
	).Return(nil)

	body, _ := json.Marshal(request)
	req := httptest.NewRequest(http.MethodPost, "/roles/role1/remove-assignments", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "role1")
	w := httptest.NewRecorder()

	suite.handler.HandleRoleRemoveAssignmentsRequest(w, req)

	suite.Equal(http.StatusNoContent, w.Code)
}

// ParsePaginationParams Tests
func (suite *RoleHandlerTestSuite) TestParsePaginationParams() {
	testCases := []struct {
		name           string
		queryString    string
		expectedLimit  int
		expectedOffset int
		expectError    bool
	}{
		{
			name:           "ValidParams",
			queryString:    "limit=20&offset=10",
			expectedLimit:  20,
			expectedOffset: 10,
			expectError:    false,
		},
		{
			name:           "DefaultLimit",
			queryString:    "offset=5",
			expectedLimit:  30,
			expectedOffset: 5,
			expectError:    false,
		},
		{
			name:           "NoParams",
			queryString:    "",
			expectedLimit:  30,
			expectedOffset: 0,
			expectError:    false,
		},
		{
			name:           "InvalidLimit",
			queryString:    "limit=abc",
			expectedLimit:  0,
			expectedOffset: 0,
			expectError:    true,
		},
		{
			name:           "InvalidOffset",
			queryString:    "offset=xyz",
			expectedLimit:  0,
			expectedOffset: 0,
			expectError:    true,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			query, _ := url.ParseQuery(tc.queryString)
			limit, offset, err := parsePaginationParams(query)

			if tc.expectError {
				suite.NotNil(err)
			} else {
				suite.Nil(err)
				suite.Equal(tc.expectedLimit, limit)
				suite.Equal(tc.expectedOffset, offset)
			}
		})
	}
}

// HandleRolePutRequest additional tests
func (suite *RoleHandlerTestSuite) TestHandleRolePutRequest_MissingID() {
	request := UpdateRoleRequest{
		Name:        "Updated Role",
		OUID:        "ou1",
		Permissions: []ResourcePermissions{{ResourceServerID: "rs1", Permissions: []string{"perm1"}}},
	}

	suite.mockService.On("UpdateRoleWithPermissions", mock.Anything, "", mock.AnythingOfType("RoleUpdateDetail")).
		Return(nil, &ErrorMissingRoleID)

	body, _ := json.Marshal(request)
	req := httptest.NewRequest(http.MethodPut, "/roles/", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	suite.handler.HandleRolePutRequest(w, req)

	suite.Equal(http.StatusBadRequest, w.Code)
}

func (suite *RoleHandlerTestSuite) TestHandleRolePutRequest_RoleNotFound() {
	request := UpdateRoleRequest{
		Name:        "Updated Role",
		OUID:        "ou1",
		Permissions: []ResourcePermissions{{ResourceServerID: "rs1", Permissions: []string{"perm1"}}},
	}

	suite.mockService.On("UpdateRoleWithPermissions", mock.Anything, "nonexistent",
		mock.AnythingOfType("RoleUpdateDetail")).
		Return(nil, &ErrorRoleNotFound)

	body, _ := json.Marshal(request)
	req := httptest.NewRequest(http.MethodPut, "/roles/nonexistent", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	suite.handler.HandleRolePutRequest(w, req)

	suite.Equal(http.StatusNotFound, w.Code)
}

// HandleRoleDeleteRequest additional tests
func (suite *RoleHandlerTestSuite) TestHandleRoleDeleteRequest_MissingID() {
	suite.mockService.On("DeleteRole", mock.Anything, "").Return(&ErrorMissingRoleID)

	req := httptest.NewRequest(http.MethodDelete, "/roles/", nil)
	w := httptest.NewRecorder()

	suite.handler.HandleRoleDeleteRequest(w, req)

	suite.Equal(http.StatusBadRequest, w.Code)
}

func (suite *RoleHandlerTestSuite) TestHandleRoleDeleteRequest_RoleNotFound() {
	suite.mockService.On("DeleteRole", mock.Anything, "nonexistent").Return(&ErrorRoleNotFound)

	req := httptest.NewRequest(http.MethodDelete, "/roles/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	suite.handler.HandleRoleDeleteRequest(w, req)

	suite.Equal(http.StatusNotFound, w.Code)
}

// HandleRoleAssignmentsGetRequest additional tests
func (suite *RoleHandlerTestSuite) TestHandleRoleAssignmentsGetRequest_MissingID() {
	suite.mockAssignmentService.On("GetRoleAssignments", mock.Anything, "", 30, 0, false).
		Return(nil, &ErrorMissingRoleID)

	req := httptest.NewRequest(http.MethodGet, "/roles//assignments", nil)
	w := httptest.NewRecorder()

	suite.handler.HandleRoleAssignmentsGetRequest(w, req)

	suite.Equal(http.StatusBadRequest, w.Code)
}

func (suite *RoleHandlerTestSuite) TestHandleRoleAssignmentsGetRequest_InvalidPagination() {
	req := httptest.NewRequest(http.MethodGet, "/roles/role1/assignments?limit=invalid", nil)
	req.SetPathValue("id", "role1")
	w := httptest.NewRecorder()

	suite.handler.HandleRoleAssignmentsGetRequest(w, req)

	suite.Equal(http.StatusBadRequest, w.Code)
}

// HandleRoleAddAssignmentsRequest additional tests
func (suite *RoleHandlerTestSuite) TestHandleRoleAddAssignmentsRequest_MissingID() {
	request := AssignmentsRequest{
		Assignments: []AssignmentRequest{
			{ID: "user1", Type: AssigneeTypeUser},
		},
	}

	suite.mockAssignmentService.On("AddAssignments", mock.Anything, "", mock.AnythingOfType("[]role.RoleAssignment")).
		Return(&ErrorMissingRoleID)

	body, _ := json.Marshal(request)
	req := httptest.NewRequest(http.MethodPost, "/roles//add-assignments", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	suite.handler.HandleRoleAddAssignmentsRequest(w, req)

	suite.Equal(http.StatusBadRequest, w.Code)
}

func (suite *RoleHandlerTestSuite) TestHandleRoleAddAssignmentsRequest_RoleNotFound() {
	request := AssignmentsRequest{
		Assignments: []AssignmentRequest{
			{ID: "user1", Type: AssigneeTypeUser},
		},
	}

	suite.mockAssignmentService.On(
		"AddAssignments", mock.Anything, "nonexistent", mock.AnythingOfType("[]role.RoleAssignment"),
	).Return(&ErrorRoleNotFound)

	body, _ := json.Marshal(request)
	req := httptest.NewRequest(http.MethodPost, "/roles/nonexistent/add-assignments", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	suite.handler.HandleRoleAddAssignmentsRequest(w, req)

	suite.Equal(http.StatusNotFound, w.Code)
}

// HandleRoleRemoveAssignmentsRequest additional tests
func (suite *RoleHandlerTestSuite) TestHandleRoleRemoveAssignmentsRequest_MissingID() {
	request := AssignmentsRequest{
		Assignments: []AssignmentRequest{
			{ID: "user1", Type: AssigneeTypeUser},
		},
	}

	suite.mockAssignmentService.On(
		"RemoveAssignments", mock.Anything, "", mock.AnythingOfType("[]role.RoleAssignment"),
	).Return(&ErrorMissingRoleID)

	body, _ := json.Marshal(request)
	req := httptest.NewRequest(http.MethodPost, "/roles//remove-assignments", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	suite.handler.HandleRoleRemoveAssignmentsRequest(w, req)

	suite.Equal(http.StatusBadRequest, w.Code)
}

func (suite *RoleHandlerTestSuite) TestHandleRoleRemoveAssignmentsRequest_InvalidJSON() {
	req := httptest.NewRequest(http.MethodPost, "/roles/role1/remove-assignments", bytes.NewBufferString("invalid"))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "role1")
	w := httptest.NewRecorder()

	suite.handler.HandleRoleRemoveAssignmentsRequest(w, req)

	suite.Equal(http.StatusBadRequest, w.Code)
}

func (suite *RoleHandlerTestSuite) TestHandleRoleRemoveAssignmentsRequest_RoleNotFound() {
	request := AssignmentsRequest{
		Assignments: []AssignmentRequest{
			{ID: "user1", Type: AssigneeTypeUser},
		},
	}

	suite.mockAssignmentService.On("RemoveAssignments", mock.Anything, "nonexistent",
		mock.AnythingOfType("[]role.RoleAssignment")).
		Return(&ErrorRoleNotFound)

	body, _ := json.Marshal(request)
	req := httptest.NewRequest(http.MethodPost, "/roles/nonexistent/remove-assignments", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	suite.handler.HandleRoleRemoveAssignmentsRequest(w, req)

	suite.Equal(http.StatusNotFound, w.Code)
}

func (suite *RoleHandlerTestSuite) TestHandleRoleRemoveAssignmentsRequest_ServiceError() {
	request := AssignmentsRequest{
		Assignments: []AssignmentRequest{
			{ID: "user1", Type: AssigneeTypeUser},
		},
	}

	suite.mockAssignmentService.On(
		"RemoveAssignments", mock.Anything, "role1", mock.AnythingOfType("[]role.RoleAssignment"),
	).Return(&ErrorInvalidAssignmentID)

	body, _ := json.Marshal(request)
	req := httptest.NewRequest(http.MethodPost, "/roles/role1/remove-assignments", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "role1")
	w := httptest.NewRecorder()

	suite.handler.HandleRoleRemoveAssignmentsRequest(w, req)

	suite.Equal(http.StatusBadRequest, w.Code)
}

// Sanitization Tests
func (suite *RoleHandlerTestSuite) TestSanitizeCreateRoleRequest() {
	request := &CreateRoleRequest{
		Name:        "  Test Role  ",
		Description: "  Description  ",
		OUID:        "  ou1  ",
		Permissions: []ResourcePermissions{
			{
				ResourceServerID: "  rs1  ",
				Permissions:      []string{"  perm1  ", "  perm2  "},
			},
		},
		Assignments: []AssignmentRequest{
			{ID: "  user1  ", Type: AssigneeTypeUser},
		},
	}

	sanitized := suite.handler.sanitizeCreateRoleRequest(request)

	suite.Equal("Test Role", sanitized.Name)
	suite.Equal("Description", sanitized.Description)
	suite.Equal("ou1", sanitized.OUID)
	suite.Equal("rs1", sanitized.Permissions[0].ResourceServerID)
	suite.Equal("perm1", sanitized.Permissions[0].Permissions[0])
	suite.Equal("user1", sanitized.Assignments[0].ID)
}

func (suite *RoleHandlerTestSuite) TestSanitizeUpdateRoleRequest() {
	request := &UpdateRoleRequest{
		Name:        "  Updated Name  ",
		OUID:        "  ou2  ",
		Permissions: []ResourcePermissions{{ResourceServerID: "  rs2  ", Permissions: []string{"  perm3  "}}},
	}

	sanitized := suite.handler.sanitizeUpdateRoleRequest(request)

	suite.Equal("Updated Name", sanitized.Name)
	suite.Equal("ou2", sanitized.OUID)
	suite.Equal("rs2", sanitized.Permissions[0].ResourceServerID)
	suite.Equal("perm3", sanitized.Permissions[0].Permissions[0])
}

func (suite *RoleHandlerTestSuite) TestSanitizeCreateRoleRequest_PreservesSpecialCharacters() {
	request := &CreateRoleRequest{
		Name:        "  Dean's Sub team  ",
		Description: `  R&D <team> "core"  `,
	}

	sanitized := suite.handler.sanitizeCreateRoleRequest(request)

	suite.Equal("Dean's Sub team", sanitized.Name)
	suite.Equal(`R&D <team> "core"`, sanitized.Description)
}

func (suite *RoleHandlerTestSuite) TestSanitizeUpdateRoleRequest_PreservesSpecialCharacters() {
	request := &UpdateRoleRequest{
		Name:        "  Dean's Sub team  ",
		Description: `  R&D <team> "core"  `,
	}

	sanitized := suite.handler.sanitizeUpdateRoleRequest(request)

	suite.Equal("Dean's Sub team", sanitized.Name)
	suite.Equal(`R&D <team> "core"`, sanitized.Description)
}

// The Console echoes the stored name back on every save. Replaying that loop must not drift the name.
func (suite *RoleHandlerTestSuite) TestSanitizeUpdateRoleRequestIsIdempotentAcrossSaves() {
	name := suite.handler.sanitizeCreateRoleRequest(&CreateRoleRequest{
		Name: "Dean's Sub team",
	}).Name

	for i := 0; i < 5; i++ {
		sanitized := suite.handler.sanitizeUpdateRoleRequest(&UpdateRoleRequest{
			Name:        name,
			Description: "save " + strconv.Itoa(i),
		})
		suite.Equal("Dean's Sub team", sanitized.Name)
		name = sanitized.Name
	}
}

func (suite *RoleHandlerTestSuite) TestSanitizeAssignmentsRequest() {
	request := &AssignmentsRequest{
		Assignments: []AssignmentRequest{
			{ID: "  group1  ", Type: AssigneeTypeGroup},
		},
	}

	sanitized := suite.handler.sanitizeAssignmentsRequest(request)

	suite.Equal("group1", sanitized.Assignments[0].ID)
	suite.Equal(AssigneeTypeGroup, sanitized.Assignments[0].Type)
}

// handleError coverage
func (suite *RoleHandlerTestSuite) TestHandleError_ClientAndServerErrors() {
	suite.T().Run("Client_NotFound", func(t *testing.T) {
		w := httptest.NewRecorder()

		handleError(context.Background(), w, &ErrorRoleNotFound)

		suite.Equal(http.StatusNotFound, w.Code)
		var resp apierror.ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		suite.NoError(err)
		suite.Equal(ErrorRoleNotFound.Code, resp.Code)
	})

	suite.T().Run("ServerError", func(t *testing.T) {
		w := httptest.NewRecorder()

		handleError(context.Background(), w, &tidcommon.InternalServerError)

		suite.Equal(http.StatusInternalServerError, w.Code)
		var resp apierror.ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		suite.NoError(err)
		suite.Equal(tidcommon.InternalServerError.Code, resp.Code)
	})
}

func sharingRequest(method, target string, body interface{}, pathValues map[string]string) *http.Request {
	var raw []byte
	if body != nil {
		raw, _ = json.Marshal(body)
	}
	req := httptest.NewRequest(method, target, bytes.NewReader(raw))
	for k, v := range pathValues {
		req.SetPathValue(k, v)
	}
	return req
}

// The status separates the kinds of refusal a sharing call produces: a malformed request, a policy
// or an organization unit that is not there, and a conflict with state the caller has not seen.
func (suite *RoleHandlerTestSuite) TestSharingErrorsCarryTheirOwnStatus() {
	for _, tc := range []struct {
		name       string
		svcErr     *tidcommon.ServiceError
		wantStatus int
	}{
		{"a policy the unit may not write", &ErrorInvalidSharingPolicy, http.StatusBadRequest},
		{"no such policy", &ErrorSharingPolicyNotFound, http.StatusNotFound},
		{"a second policy for one unit", &ErrorSharingPolicyExists, http.StatusConflict},
		{"a stale version", &ErrorSharingPolicyVersionMismatch, http.StatusConflict},
		{"a shared role moved", &ErrorSharedRoleCannotMove, http.StatusBadRequest},
		{"a declared policy", &ErrorSharingPolicyDeclared, http.StatusBadRequest},
		{"an unshared organization unit", &ErrorRoleNotSharedToOU, http.StatusNotFound},
		{"a framework failure", &tidcommon.InternalServerError, http.StatusInternalServerError},
	} {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			suite.mockService.EXPECT().GetSharingPolicy(mock.Anything, "role1", "p1").
				Return(sharing.Policy{}, tc.svcErr)

			w := httptest.NewRecorder()
			suite.handler.HandleSharingPolicyGetRequest(w, sharingRequest(http.MethodGet,
				"/roles/role1/sharing-policies/p1", nil, map[string]string{"id": "role1", "policyId": "p1"}))

			suite.Equal(tc.wantStatus, w.Code)
			var body map[string]interface{}
			suite.Require().NoError(json.Unmarshal(w.Body.Bytes(), &body))
			suite.Equal(tc.svcErr.Code, body["code"])
		})
	}
}

// A create answers 201 with the recorded policy, and names the issuer the body gives.
func (suite *RoleHandlerTestSuite) TestSharingPolicyPostAnswersCreated() {
	var recorded sharing.PolicyRequest
	suite.mockService.EXPECT().CreateSharingPolicy(mock.Anything, "role1", mock.Anything).
		Run(func(_ context.Context, _ string, req sharing.PolicyRequest) { recorded = req }).
		Return(sharing.Policy{ID: "p1", Version: 1,
			ChildUnits: &sharing.ChildUnits{Mode: sharing.ChildUnitsAll}}, nil)

	w := httptest.NewRecorder()
	suite.handler.HandleSharingPolicyPostRequest(w, sharingRequest(http.MethodPost, "/roles/role1/sharing-policies",
		map[string]interface{}{
			"initiatingOuId": "ou-owner",
			"childUnits": map[string]interface{}{
				"mode": "selected",
				"units": []map[string]interface{}{{"ouId": "ou-child", "reach": "subtree",
					"overlayRules": map[string]interface{}{"assignments": map[string]bool{"editable": false}}}}},
		},
		map[string]string{"id": "role1"}))

	suite.Require().Equal(http.StatusCreated, w.Code, "body: %s", w.Body.String())
	var body map[string]interface{}
	suite.Require().NoError(json.Unmarshal(w.Body.Bytes(), &body))
	suite.Equal("p1", body["id"])

	suite.Equal("ou-owner", recorded.InitiatingOUID)
	suite.Require().NotNil(recorded.ChildUnits)
	suite.Equal(sharing.ChildUnitsSelected, recorded.ChildUnits.Mode)
	suite.Require().Len(recorded.ChildUnits.Units, 1)
	suite.Equal(sharing.ReachSubtree, recorded.ChildUnits.Units[0].Reach)
	suite.Equal(map[string]sharing.OverlayRule{"assignments": {Editable: false}},
		recorded.ChildUnits.Units[0].OverlayRules, "a policy may turn the role's assignments off")
}

// A field name the body gets wrong is refused rather than dropped, so a misspelled exclusion does
// not share the role with the organization unit the caller named to keep it from.
func (suite *RoleHandlerTestSuite) TestSharingPolicyPostRefusesAnUnknownField() {
	for _, tc := range []struct {
		name string
		body map[string]interface{}
	}{
		{"a misspelled exclusion", map[string]interface{}{"childUnits": map[string]interface{}{
			"mode": "all", "excludeOuIds": []string{"ou-trial"}}}},
		{"a policy id", map[string]interface{}{"id": "caller-chosen",
			"childUnits": map[string]string{"mode": "all"}}},
	} {
		suite.Run(tc.name, func() {
			suite.SetupTest()

			w := httptest.NewRecorder()
			suite.handler.HandleSharingPolicyPostRequest(w, sharingRequest(http.MethodPost,
				"/roles/role1/sharing-policies", tc.body, map[string]string{"id": "role1"}))

			// The service mock records no call: the body never gets that far.
			suite.Equal(http.StatusBadRequest, w.Code, "body: %s", w.Body.String())
		})
	}
}

// This API leaves a deployment-wide reach to resource files and imports.
func (suite *RoleHandlerTestSuite) TestSharingPolicyPostRefusesADeploymentWideReach() {
	for _, tc := range []struct {
		name string
		body map[string]interface{}
	}{
		{"all units", map[string]interface{}{"allUnits": map[string]interface{}{}}},
		{"every other tree's root", map[string]interface{}{"otherTreeRoots": map[string]string{"mode": "all"}}},
	} {
		suite.Run(tc.name, func() {
			suite.SetupTest()

			w := httptest.NewRecorder()
			suite.handler.HandleSharingPolicyPostRequest(w, sharingRequest(http.MethodPost,
				"/roles/role1/sharing-policies", tc.body, map[string]string{"id": "role1"}))

			suite.Require().Equal(http.StatusBadRequest, w.Code, "body: %s", w.Body.String())
			var body map[string]interface{}
			suite.Require().NoError(json.Unmarshal(w.Body.Bytes(), &body))
			suite.Equal(ErrorDeploymentWideSharingNotAllowed.Code, body["code"])
		})
	}
}

// An edit cannot change whose decision a policy is, so a PUT naming an issuer is refused.
func (suite *RoleHandlerTestSuite) TestSharingPolicyPutRefusesAnIssuer() {
	w := httptest.NewRecorder()
	suite.handler.HandleSharingPolicyPutRequest(w, sharingRequest(http.MethodPut, "/roles/role1/sharing-policies/p1",
		map[string]interface{}{"version": 1, "initiatingOuId": "another-ou",
			"childUnits": map[string]string{"mode": "all"}},
		map[string]string{"id": "role1", "policyId": "p1"}))

	suite.Require().Equal(http.StatusBadRequest, w.Code, "body: %s", w.Body.String())
	var body map[string]interface{}
	suite.Require().NoError(json.Unmarshal(w.Body.Bytes(), &body))
	suite.Equal(ErrorInvalidRequestFormat.Code, body["code"])
}

// An edit may restate a deployment-wide section to move its exclusions.
func (suite *RoleHandlerTestSuite) TestSharingPolicyPutPassesADeploymentWideSection() {
	var recorded sharing.PolicyRequest
	suite.mockService.EXPECT().UpdateSharingPolicy(mock.Anything, "role1", "p1", mock.Anything).
		Run(func(_ context.Context, _, _ string, req sharing.PolicyRequest) { recorded = req }).
		Return(sharing.Policy{ID: "p1", Version: 3}, nil)

	w := httptest.NewRecorder()
	suite.handler.HandleSharingPolicyPutRequest(w, sharingRequest(http.MethodPut, "/roles/role1/sharing-policies/p1",
		map[string]interface{}{"version": 2,
			"allUnits": map[string]interface{}{"excludedOuIds": []string{"ou-trial"}}},
		map[string]string{"id": "role1", "policyId": "p1"}))

	suite.Require().Equal(http.StatusOK, w.Code, "body: %s", w.Body.String())
	suite.Equal(2, recorded.Version)
	suite.Require().NotNil(recorded.AllUnits)
	suite.Equal([]string{"ou-trial"}, recorded.AllUnits.ExcludedOUIDs)
}

func (suite *RoleHandlerTestSuite) TestSharingPolicyDeleteAnswersNoContent() {
	suite.mockService.EXPECT().DeleteSharingPolicy(mock.Anything, "role1", "p1").Return(nil)

	w := httptest.NewRecorder()
	suite.handler.HandleSharingPolicyDeleteRequest(w, sharingRequest(http.MethodDelete,
		"/roles/role1/sharing-policies/p1", nil, map[string]string{"id": "role1", "policyId": "p1"}))

	suite.Equal(http.StatusNoContent, w.Code)
	suite.Empty(w.Body.String())
}

// A listing pages the role's own policies and links within the role's own path.
func (suite *RoleHandlerTestSuite) TestSharingPolicyListShapesThePage() {
	suite.mockService.EXPECT().ListSharingPolicies(mock.Anything, "role1", 1, 0).
		Return(sharing.PolicyList{TotalResults: 2, StartIndex: 1, Count: 1,
			Policies: []sharing.Policy{{ID: "p1"}}}, nil)

	w := httptest.NewRecorder()
	suite.handler.HandleSharingPolicyListRequest(w, sharingRequest(http.MethodGet,
		"/roles/role1/sharing-policies?limit=1", nil, map[string]string{"id": "role1"}))

	suite.Require().Equal(http.StatusOK, w.Code)
	var body sharing.PolicyListResponse
	suite.Require().NoError(json.Unmarshal(w.Body.Bytes(), &body))
	suite.Equal(2, body.TotalResults)
	suite.Require().Len(body.Policies, 1)
	suite.Require().NotEmpty(body.Links)
	suite.Contains(body.Links[0].Href, "/roles/role1/sharing-policies")
}

func (suite *RoleHandlerTestSuite) TestSharingPolicyListRefusesABadLimit() {
	w := httptest.NewRecorder()
	suite.handler.HandleSharingPolicyListRequest(w, sharingRequest(http.MethodGet,
		"/roles/role1/sharing-policies?limit=many", nil, map[string]string{"id": "role1"}))

	suite.Equal(http.StatusBadRequest, w.Code)
}

// The organization unit to resolve for travels as a query parameter.
func (suite *RoleHandlerTestSuite) TestOverlayRulesReadsTheUnitFromTheQuery() {
	suite.mockService.EXPECT().ResolveSharingOverlay(mock.Anything, "role1", "ou-child").
		Return(sharing.ResolvedOverlay{OUID: "ou-child", Visible: true}, nil)

	w := httptest.NewRecorder()
	suite.handler.HandleOverlayRuleGetRequest(w, sharingRequest(http.MethodGet,
		"/roles/role1/overlay-rules?ouId=ou-child", nil, map[string]string{"id": "role1"}))

	suite.Require().Equal(http.StatusOK, w.Code)
	var body map[string]interface{}
	suite.Require().NoError(json.Unmarshal(w.Body.Bytes(), &body))
	suite.Equal("ou-child", body["ouId"])
	suite.Equal(true, body["visible"])
	suite.NotNil(body["rules"], "an absent rule set is an empty object, not null")
}
