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

package flowmgt

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
)

const testFlowIDHandler = "test-flow-id"

type FlowMgtHandlerTestSuite struct {
	suite.Suite
	handler     *flowMgtHandler
	mockService *FlowMgtServiceInterfaceMock
}

func TestFlowMgtHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(FlowMgtHandlerTestSuite))
}

func (s *FlowMgtHandlerTestSuite) SetupTest() {
	s.mockService = NewFlowMgtServiceInterfaceMock(s.T())
	s.handler = newFlowMgtHandler(s.mockService)
}

// Test listFlows

func (s *FlowMgtHandlerTestSuite) TestListFlows_Success() {
	expectedList := &FlowListResponse{
		Flows: []BasicFlowDefinition{
			{ID: "flow1", Handle: "flow1-handle", Name: "Flow 1", FlowType: common.FlowTypeAuthentication},
			{ID: "flow2", Handle: "flow2-handle", Name: "Flow 2", FlowType: common.FlowTypeRegistration},
		},
		Count: 2,
	}

	s.mockService.EXPECT().ListFlows(mock.Anything, 30, 0, common.FlowType("")).Return(expectedList, nil)

	req := httptest.NewRequest(http.MethodGet, "/flows", nil)
	w := httptest.NewRecorder()

	s.handler.listFlows(w, req)

	s.Equal(http.StatusOK, w.Code)
	var response FlowListResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	s.NoError(err)
	s.Equal(2, response.Count)
	s.Len(response.Flows, 2)
}

func (s *FlowMgtHandlerTestSuite) TestListFlows_WithPagination() {
	expectedList := &FlowListResponse{Flows: []BasicFlowDefinition{}, Count: 0}

	s.mockService.EXPECT().ListFlows(mock.Anything, 20, 10, common.FlowType("")).Return(expectedList, nil)

	req := httptest.NewRequest(http.MethodGet, "/flows?limit=20&offset=10", nil)
	w := httptest.NewRecorder()

	s.handler.listFlows(w, req)

	s.Equal(http.StatusOK, w.Code)
}

func (s *FlowMgtHandlerTestSuite) TestListFlows_WithFlowType() {
	expectedList := &FlowListResponse{Flows: []BasicFlowDefinition{}, Count: 0}

	s.mockService.EXPECT().ListFlows(mock.Anything, 30, 0, common.FlowTypeAuthentication).Return(expectedList, nil)

	req := httptest.NewRequest(http.MethodGet, "/flows?flowType=AUTHENTICATION", nil)
	w := httptest.NewRecorder()

	s.handler.listFlows(w, req)

	s.Equal(http.StatusOK, w.Code)
}

func (s *FlowMgtHandlerTestSuite) TestListFlows_InvalidLimit() {
	req := httptest.NewRequest(http.MethodGet, "/flows?limit=invalid", nil)
	w := httptest.NewRecorder()

	s.handler.listFlows(w, req)

	s.Equal(http.StatusBadRequest, w.Code)
}

func (s *FlowMgtHandlerTestSuite) TestListFlows_NegativeLimit() {
	req := httptest.NewRequest(http.MethodGet, "/flows?limit=-1", nil)
	w := httptest.NewRecorder()

	s.handler.listFlows(w, req)

	s.Equal(http.StatusBadRequest, w.Code)
}

func (s *FlowMgtHandlerTestSuite) TestListFlows_InvalidOffset() {
	req := httptest.NewRequest(http.MethodGet, "/flows?offset=invalid", nil)
	w := httptest.NewRecorder()

	s.handler.listFlows(w, req)

	s.Equal(http.StatusBadRequest, w.Code)
}

func (s *FlowMgtHandlerTestSuite) TestListFlows_ServiceError() {
	s.mockService.EXPECT().ListFlows(mock.Anything, 30, 0, common.FlowType("")).
		Return(nil, &serviceerror.InternalServerError)

	req := httptest.NewRequest(http.MethodGet, "/flows", nil)
	w := httptest.NewRecorder()

	s.handler.listFlows(w, req)

	s.Equal(http.StatusInternalServerError, w.Code)
}

// Test createFlow

func (s *FlowMgtHandlerTestSuite) TestCreateFlow_Success() {
	flowDef := &FlowDefinition{
		Handle:   "new-flow-handle",
		Name:     "New Flow",
		FlowType: common.FlowTypeAuthentication,
		Nodes: []NodeDefinition{
			{ID: "start", Type: "START"},
		},
	}
	createdFlow := &CompleteFlowDefinition{
		ID:       testFlowIDHandler,
		Handle:   "new-flow-handle",
		Name:     "New Flow",
		FlowType: common.FlowTypeAuthentication,
		Nodes:    flowDef.Nodes,
	}

	s.mockService.EXPECT().CreateFlow(mock.Anything, flowDef).Return(createdFlow, nil)

	body, _ := json.Marshal(flowDef)
	req := httptest.NewRequest(http.MethodPost, "/flows", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.handler.createFlow(w, req)

	s.Equal(http.StatusCreated, w.Code)
	var response CompleteFlowDefinition
	err := json.Unmarshal(w.Body.Bytes(), &response)
	s.NoError(err)
	s.Equal(testFlowIDHandler, response.ID)
	s.Equal("New Flow", response.Name)
}

func (s *FlowMgtHandlerTestSuite) TestCreateFlow_InvalidJSON() {
	req := httptest.NewRequest(http.MethodPost, "/flows", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.handler.createFlow(w, req)

	s.Equal(http.StatusBadRequest, w.Code)
}

func (s *FlowMgtHandlerTestSuite) TestCreateFlow_ServiceError() {
	flowDef := &FlowDefinition{
		Handle:   "new-flow-handle",
		Name:     "New Flow",
		FlowType: common.FlowTypeAuthentication,
	}

	s.mockService.EXPECT().CreateFlow(mock.Anything, flowDef).Return(nil, &ErrorInvalidFlowData)

	body, _ := json.Marshal(flowDef)
	req := httptest.NewRequest(http.MethodPost, "/flows", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.handler.createFlow(w, req)

	s.Equal(http.StatusBadRequest, w.Code)
}

// Test getFlow

func (s *FlowMgtHandlerTestSuite) TestGetFlow_Success() {
	expectedFlow := &CompleteFlowDefinition{
		ID:       testFlowIDHandler,
		Handle:   "test-handle",
		Name:     "Test Flow",
		FlowType: common.FlowTypeAuthentication,
	}

	s.mockService.EXPECT().GetFlow(mock.Anything, testFlowIDHandler).Return(expectedFlow, nil)

	req := httptest.NewRequest(http.MethodGet, "/flows/"+testFlowIDHandler, nil)
	req.SetPathValue(pathParamFlowID, testFlowIDHandler)
	w := httptest.NewRecorder()

	s.handler.getFlow(w, req)

	s.Equal(http.StatusOK, w.Code)
	var response CompleteFlowDefinition
	err := json.Unmarshal(w.Body.Bytes(), &response)
	s.NoError(err)
	s.Equal(testFlowIDHandler, response.ID)
}

func (s *FlowMgtHandlerTestSuite) TestGetFlow_MissingFlowID() {
	req := httptest.NewRequest(http.MethodGet, "/flows/", nil)
	w := httptest.NewRecorder()

	s.handler.getFlow(w, req)

	s.Equal(http.StatusBadRequest, w.Code)
}

func (s *FlowMgtHandlerTestSuite) TestGetFlow_NotFound() {
	s.mockService.EXPECT().GetFlow(mock.Anything, testFlowIDHandler).Return(nil, &ErrorFlowNotFound)

	req := httptest.NewRequest(http.MethodGet, "/flows/"+testFlowIDHandler, nil)
	req.SetPathValue(pathParamFlowID, testFlowIDHandler)
	w := httptest.NewRecorder()

	s.handler.getFlow(w, req)

	s.Equal(http.StatusNotFound, w.Code)
}

// Test updateFlow

func (s *FlowMgtHandlerTestSuite) TestUpdateFlow_Success() {
	flowDef := &FlowDefinition{
		Handle:   "test-handle",
		Name:     "Updated Flow",
		FlowType: common.FlowTypeAuthentication,
		Nodes: []NodeDefinition{
			{ID: "start", Type: "START"},
		},
	}
	updatedFlow := &CompleteFlowDefinition{
		ID:       testFlowIDHandler,
		Handle:   "test-handle",
		Name:     "Updated Flow",
		FlowType: common.FlowTypeAuthentication,
		Nodes:    flowDef.Nodes,
	}

	s.mockService.EXPECT().UpdateFlow(mock.Anything, testFlowIDHandler, flowDef).Return(updatedFlow, nil)

	body, _ := json.Marshal(flowDef)
	req := httptest.NewRequest(http.MethodPut, "/flows/"+testFlowIDHandler, bytes.NewReader(body))
	req.SetPathValue(pathParamFlowID, testFlowIDHandler)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.handler.updateFlow(w, req)

	s.Equal(http.StatusOK, w.Code)
	var response CompleteFlowDefinition
	err := json.Unmarshal(w.Body.Bytes(), &response)
	s.NoError(err)
	s.Equal(testFlowIDHandler, response.ID)
	s.Equal("Updated Flow", response.Name)
}

func (s *FlowMgtHandlerTestSuite) TestUpdateFlow_MissingFlowID() {
	req := httptest.NewRequest(http.MethodPut, "/flows/", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.handler.updateFlow(w, req)

	s.Equal(http.StatusBadRequest, w.Code)
}

func (s *FlowMgtHandlerTestSuite) TestUpdateFlow_InvalidJSON() {
	req := httptest.NewRequest(http.MethodPut, "/flows/"+testFlowIDHandler, bytes.NewReader([]byte("invalid")))
	req.SetPathValue(pathParamFlowID, testFlowIDHandler)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.handler.updateFlow(w, req)

	s.Equal(http.StatusBadRequest, w.Code)
}

func (s *FlowMgtHandlerTestSuite) TestUpdateFlow_NotFound() {
	flowDef := &FlowDefinition{
		Handle:   "test-handle",
		Name:     "Updated Flow",
		FlowType: common.FlowTypeAuthentication,
	}

	s.mockService.EXPECT().UpdateFlow(mock.Anything, testFlowIDHandler, flowDef).Return(nil, &ErrorFlowNotFound)

	body, _ := json.Marshal(flowDef)
	req := httptest.NewRequest(http.MethodPut, "/flows/"+testFlowIDHandler, bytes.NewReader(body))
	req.SetPathValue(pathParamFlowID, testFlowIDHandler)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.handler.updateFlow(w, req)

	s.Equal(http.StatusNotFound, w.Code)
}

// Test deleteFlow

func (s *FlowMgtHandlerTestSuite) TestDeleteFlow_Success() {
	s.mockService.EXPECT().DeleteFlow(mock.Anything, testFlowIDHandler).Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/flows/"+testFlowIDHandler, nil)
	req.SetPathValue(pathParamFlowID, testFlowIDHandler)
	w := httptest.NewRecorder()

	s.handler.deleteFlow(w, req)

	s.Equal(http.StatusNoContent, w.Code)
}

func (s *FlowMgtHandlerTestSuite) TestDeleteFlow_MissingFlowID() {
	req := httptest.NewRequest(http.MethodDelete, "/flows/", nil)
	w := httptest.NewRecorder()

	s.handler.deleteFlow(w, req)

	s.Equal(http.StatusBadRequest, w.Code)
}

func (s *FlowMgtHandlerTestSuite) TestDeleteFlow_NotFound() {
	s.mockService.EXPECT().DeleteFlow(mock.Anything, testFlowIDHandler).Return(&ErrorFlowNotFound)

	req := httptest.NewRequest(http.MethodDelete, "/flows/"+testFlowIDHandler, nil)
	req.SetPathValue(pathParamFlowID, testFlowIDHandler)
	w := httptest.NewRecorder()

	s.handler.deleteFlow(w, req)

	s.Equal(http.StatusNotFound, w.Code)
}

// Test listFlowVersions

func (s *FlowMgtHandlerTestSuite) TestListFlowVersions_Success() {
	expectedList := &FlowVersionListResponse{
		Versions: []BasicFlowVersion{
			{Version: 1, IsActive: true},
			{Version: 2, IsActive: false},
		},
		TotalVersions: 2,
	}

	s.mockService.EXPECT().ListFlowVersions(mock.Anything, testFlowIDHandler).Return(expectedList, nil)

	req := httptest.NewRequest(http.MethodGet, "/flows/"+testFlowIDHandler+"/versions", nil)
	req.SetPathValue(pathParamFlowID, testFlowIDHandler)
	w := httptest.NewRecorder()

	s.handler.listFlowVersions(w, req)

	s.Equal(http.StatusOK, w.Code)
	var response FlowVersionListResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	s.NoError(err)
	s.Equal(2, response.TotalVersions)
}

func (s *FlowMgtHandlerTestSuite) TestListFlowVersions_MissingFlowID() {
	req := httptest.NewRequest(http.MethodGet, "/flows//versions", nil)
	w := httptest.NewRecorder()

	s.handler.listFlowVersions(w, req)

	s.Equal(http.StatusBadRequest, w.Code)
}

func (s *FlowMgtHandlerTestSuite) TestListFlowVersions_NotFound() {
	s.mockService.EXPECT().ListFlowVersions(mock.Anything, testFlowIDHandler).Return(nil, &ErrorFlowNotFound)

	req := httptest.NewRequest(http.MethodGet, "/flows/"+testFlowIDHandler+"/versions", nil)
	req.SetPathValue(pathParamFlowID, testFlowIDHandler)
	w := httptest.NewRecorder()

	s.handler.listFlowVersions(w, req)

	s.Equal(http.StatusNotFound, w.Code)
}

// Test getFlowVersion

func (s *FlowMgtHandlerTestSuite) TestGetFlowVersion_Success() {
	expectedVersion := &FlowVersion{
		ID:      testFlowIDHandler,
		Handle:  "test-handle",
		Version: 1,
		Name:    "Test Flow",
	}

	s.mockService.EXPECT().GetFlowVersion(mock.Anything, testFlowIDHandler, 1).Return(expectedVersion, nil)

	req := httptest.NewRequest(http.MethodGet, "/flows/"+testFlowIDHandler+"/versions/1", nil)
	req.SetPathValue(pathParamFlowID, testFlowIDHandler)
	req.SetPathValue(pathParamVersion, "1")
	w := httptest.NewRecorder()

	s.handler.getFlowVersion(w, req)

	s.Equal(http.StatusOK, w.Code)
	var response FlowVersion
	err := json.Unmarshal(w.Body.Bytes(), &response)
	s.NoError(err)
	s.Equal(1, response.Version)
}

func (s *FlowMgtHandlerTestSuite) TestGetFlowVersion_MissingFlowID() {
	req := httptest.NewRequest(http.MethodGet, "/flows//versions/1", nil)
	req.SetPathValue(pathParamVersion, "1")
	w := httptest.NewRecorder()

	s.handler.getFlowVersion(w, req)

	s.Equal(http.StatusBadRequest, w.Code)
}

func (s *FlowMgtHandlerTestSuite) TestGetFlowVersion_MissingVersion() {
	req := httptest.NewRequest(http.MethodGet, "/flows/"+testFlowIDHandler+"/versions/", nil)
	req.SetPathValue(pathParamFlowID, testFlowIDHandler)
	w := httptest.NewRecorder()

	s.handler.getFlowVersion(w, req)

	s.Equal(http.StatusBadRequest, w.Code)
}

func (s *FlowMgtHandlerTestSuite) TestGetFlowVersion_InvalidVersion() {
	req := httptest.NewRequest(http.MethodGet, "/flows/"+testFlowIDHandler+"/versions/invalid", nil)
	req.SetPathValue(pathParamFlowID, testFlowIDHandler)
	req.SetPathValue(pathParamVersion, "invalid")
	w := httptest.NewRecorder()

	s.handler.getFlowVersion(w, req)

	s.Equal(http.StatusBadRequest, w.Code)
}

func (s *FlowMgtHandlerTestSuite) TestGetFlowVersion_ZeroVersion() {
	req := httptest.NewRequest(http.MethodGet, "/flows/"+testFlowIDHandler+"/versions/0", nil)
	req.SetPathValue(pathParamFlowID, testFlowIDHandler)
	req.SetPathValue(pathParamVersion, "0")
	w := httptest.NewRecorder()

	s.handler.getFlowVersion(w, req)

	s.Equal(http.StatusBadRequest, w.Code)
}

func (s *FlowMgtHandlerTestSuite) TestGetFlowVersion_NotFound() {
	s.mockService.EXPECT().GetFlowVersion(mock.Anything, testFlowIDHandler, 99).Return(nil, &ErrorVersionNotFound)

	req := httptest.NewRequest(http.MethodGet, "/flows/"+testFlowIDHandler+"/versions/99", nil)
	req.SetPathValue(pathParamFlowID, testFlowIDHandler)
	req.SetPathValue(pathParamVersion, "99")
	w := httptest.NewRecorder()

	s.handler.getFlowVersion(w, req)

	s.Equal(http.StatusNotFound, w.Code)
}

// Test restoreFlowVersion

func (s *FlowMgtHandlerTestSuite) TestRestoreFlowVersion_Success() {
	request := &RestoreVersionRequest{Version: 1}
	restoredFlow := &CompleteFlowDefinition{
		ID:       testFlowIDHandler,
		Handle:   "test-handle",
		Name:     "Restored Flow",
		FlowType: common.FlowTypeAuthentication,
	}

	s.mockService.EXPECT().RestoreFlowVersion(mock.Anything, testFlowIDHandler, 1).Return(restoredFlow, nil)

	body, _ := json.Marshal(request)
	req := httptest.NewRequest(http.MethodPost, "/flows/"+testFlowIDHandler+"/versions/restore",
		bytes.NewReader(body))
	req.SetPathValue(pathParamFlowID, testFlowIDHandler)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.handler.restoreFlowVersion(w, req)

	s.Equal(http.StatusOK, w.Code)
	var response CompleteFlowDefinition
	err := json.Unmarshal(w.Body.Bytes(), &response)
	s.NoError(err)
	s.Equal(testFlowIDHandler, response.ID)
}

func (s *FlowMgtHandlerTestSuite) TestRestoreFlowVersion_MissingFlowID() {
	request := &RestoreVersionRequest{Version: 1}
	body, _ := json.Marshal(request)
	req := httptest.NewRequest(http.MethodPost, "/flows//versions/restore", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.handler.restoreFlowVersion(w, req)

	s.Equal(http.StatusBadRequest, w.Code)
}

func (s *FlowMgtHandlerTestSuite) TestRestoreFlowVersion_InvalidJSON() {
	req := httptest.NewRequest(http.MethodPost, "/flows/"+testFlowIDHandler+"/versions/restore",
		bytes.NewReader([]byte("invalid")))
	req.SetPathValue(pathParamFlowID, testFlowIDHandler)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.handler.restoreFlowVersion(w, req)

	s.Equal(http.StatusBadRequest, w.Code)
}

func (s *FlowMgtHandlerTestSuite) TestRestoreFlowVersion_NotFound() {
	request := &RestoreVersionRequest{Version: 99}

	s.mockService.EXPECT().RestoreFlowVersion(mock.Anything, testFlowIDHandler, 99).Return(nil, &ErrorVersionNotFound)

	body, _ := json.Marshal(request)
	req := httptest.NewRequest(http.MethodPost, "/flows/"+testFlowIDHandler+"/versions/restore",
		bytes.NewReader(body))
	req.SetPathValue(pathParamFlowID, testFlowIDHandler)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.handler.restoreFlowVersion(w, req)

	s.Equal(http.StatusNotFound, w.Code)
}

// Test parsePaginationParams

func (s *FlowMgtHandlerTestSuite) TestParsePaginationParams_DefaultValues() {
	req := httptest.NewRequest(http.MethodGet, "/flows", nil)

	limit, offset, err := parsePaginationParams(req)

	s.Nil(err)
	s.Equal(defaultPageSize, limit)
	s.Equal(0, offset)
}

func (s *FlowMgtHandlerTestSuite) TestParsePaginationParams_CustomValues() {
	req := httptest.NewRequest(http.MethodGet, "/flows?limit=20&offset=10", nil)

	limit, offset, err := parsePaginationParams(req)

	s.Nil(err)
	s.Equal(20, limit)
	s.Equal(10, offset)
}

func (s *FlowMgtHandlerTestSuite) TestParsePaginationParams_InvalidLimit() {
	req := httptest.NewRequest(http.MethodGet, "/flows?limit=invalid", nil)

	_, _, err := parsePaginationParams(req)

	s.NotNil(err)
	s.Equal(ErrorInvalidLimit.Code, err.Code)
}

func (s *FlowMgtHandlerTestSuite) TestParsePaginationParams_NegativeLimit() {
	req := httptest.NewRequest(http.MethodGet, "/flows?limit=-1", nil)

	_, _, err := parsePaginationParams(req)

	s.NotNil(err)
	s.Equal(ErrorInvalidLimit.Code, err.Code)
}

func (s *FlowMgtHandlerTestSuite) TestParsePaginationParams_InvalidOffset() {
	req := httptest.NewRequest(http.MethodGet, "/flows?offset=invalid", nil)

	_, _, err := parsePaginationParams(req)

	s.NotNil(err)
	s.Equal(ErrorInvalidOffset.Code, err.Code)
}

func (s *FlowMgtHandlerTestSuite) TestParsePaginationParams_NegativeOffset() {
	req := httptest.NewRequest(http.MethodGet, "/flows?offset=-1", nil)

	_, _, err := parsePaginationParams(req)

	s.NotNil(err)
	s.Equal(ErrorInvalidOffset.Code, err.Code)
}

// Test sanitizeFlowDefinitionRequest

func (s *FlowMgtHandlerTestSuite) TestSanitizeFlowDefinitionRequest() {
	input := &FlowDefinitionRequest{
		Handle:   "test-handle",
		Name:     "  Test Flow  ",
		FlowType: common.FlowTypeAuthentication,
		Nodes: []NodeDefinition{
			{ID: "start", Type: "START"},
		},
	}

	result := sanitizeFlowDefinitionRequest(input)

	s.Equal("Test Flow", result.Name)
	s.Equal(common.FlowTypeAuthentication, result.FlowType)
	s.Len(result.Nodes, 1)
}
