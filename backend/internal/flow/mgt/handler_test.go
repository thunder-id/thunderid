// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowmgt

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/sharing"
)

const (
	testFlowIDHandler = "test-flow-id"
	testOUIDHandler   = "test-ou-id"
)

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
			{ID: "flow1", Handle: "flow1-handle", Name: "Flow 1", FlowType: providers.FlowTypeAuthentication},
			{ID: "flow2", Handle: "flow2-handle", Name: "Flow 2", FlowType: providers.FlowTypeRegistration},
		},
		Count: 2,
	}

	s.mockService.EXPECT().ListFlows(mock.Anything, 30, 0, providers.FlowType("")).Return(expectedList, nil)

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

	s.mockService.EXPECT().ListFlows(mock.Anything, 20, 10, providers.FlowType("")).Return(expectedList, nil)

	req := httptest.NewRequest(http.MethodGet, "/flows?limit=20&offset=10", nil)
	w := httptest.NewRecorder()

	s.handler.listFlows(w, req)

	s.Equal(http.StatusOK, w.Code)
}

func (s *FlowMgtHandlerTestSuite) TestListFlows_WithFlowType() {
	expectedList := &FlowListResponse{Flows: []BasicFlowDefinition{}, Count: 0}

	s.mockService.EXPECT().ListFlows(mock.Anything, 30, 0, providers.FlowTypeAuthentication).Return(expectedList, nil)

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
	s.mockService.EXPECT().ListFlows(mock.Anything, 30, 0, providers.FlowType("")).
		Return(nil, &tidcommon.InternalServerError)

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
		FlowType: providers.FlowTypeAuthentication,
		Nodes: []providers.NodeDefinition{
			{ID: "start", Type: "START"},
		},
	}
	createdFlow := &providers.CompleteFlowDefinition{
		ID:       testFlowIDHandler,
		Handle:   "new-flow-handle",
		Name:     "New Flow",
		FlowType: providers.FlowTypeAuthentication,
		Nodes:    flowDef.Nodes,
	}

	s.mockService.EXPECT().CreateFlow(mock.Anything, flowDef).Return(createdFlow, nil)

	body, _ := json.Marshal(flowDef)
	req := httptest.NewRequest(http.MethodPost, "/flows", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.handler.createFlow(w, req)

	s.Equal(http.StatusCreated, w.Code)
	var response providers.CompleteFlowDefinition
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
		FlowType: providers.FlowTypeAuthentication,
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
	expectedFlow := &providers.CompleteFlowDefinition{
		ID:       testFlowIDHandler,
		Handle:   "test-handle",
		Name:     "Test Flow",
		FlowType: providers.FlowTypeAuthentication,
	}

	s.mockService.EXPECT().GetFlow(mock.Anything, testFlowIDHandler).Return(expectedFlow, nil)

	req := httptest.NewRequest(http.MethodGet, "/flows/"+testFlowIDHandler, nil)
	req.SetPathValue(pathParamFlowID, testFlowIDHandler)
	w := httptest.NewRecorder()

	s.handler.getFlow(w, req)

	s.Equal(http.StatusOK, w.Code)
	var response providers.CompleteFlowDefinition
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

// Test getFlowUsages

func (s *FlowMgtHandlerTestSuite) TestGetFlowUsages_Success() {
	total := 1
	s.mockService.EXPECT().GetFlowUsages(mock.Anything, testFlowIDHandler).Return(
		&resourcedependency.DependenciesResponse{
			TotalResults: &total,
			Count:        1,
			Summary:      map[string]int{resourcedependency.ResourceTypeApplication: 1},
			Usages: []resourcedependency.ResourceDependency{
				{ResourceType: resourcedependency.ResourceTypeApplication, ID: "app-1",
					DisplayName: "App One", BehaviorOnDelete: resourcedependency.BehaviorFallback},
			},
		}, nil)

	req := httptest.NewRequest(http.MethodGet, "/flows/"+testFlowIDHandler+"/usages", nil)
	req.SetPathValue(pathParamFlowID, testFlowIDHandler)
	w := httptest.NewRecorder()

	s.handler.getFlowUsages(w, req)

	s.Equal(http.StatusOK, w.Code)
	var response resourcedependency.DependenciesResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	s.NoError(err)
	s.Equal(1, *response.TotalResults)
	s.Len(response.Usages, 1)
	s.Equal(resourcedependency.ResourceTypeApplication, response.Usages[0].ResourceType)
}

func (s *FlowMgtHandlerTestSuite) TestGetFlowUsages_MissingFlowID() {
	req := httptest.NewRequest(http.MethodGet, "/flows//usages", nil)
	w := httptest.NewRecorder()

	s.handler.getFlowUsages(w, req)

	s.Equal(http.StatusBadRequest, w.Code)
}

func (s *FlowMgtHandlerTestSuite) TestGetFlowUsages_NotFound() {
	s.mockService.EXPECT().GetFlowUsages(mock.Anything, testFlowIDHandler).Return(nil, &ErrorFlowNotFound)

	req := httptest.NewRequest(http.MethodGet, "/flows/"+testFlowIDHandler+"/usages", nil)
	req.SetPathValue(pathParamFlowID, testFlowIDHandler)
	w := httptest.NewRecorder()

	s.handler.getFlowUsages(w, req)

	s.Equal(http.StatusNotFound, w.Code)
}

// Test updateFlow

func (s *FlowMgtHandlerTestSuite) TestUpdateFlow_Success() {
	flowDef := &FlowDefinition{
		Handle:   "test-handle",
		Name:     "Updated Flow",
		FlowType: providers.FlowTypeAuthentication,
		Nodes: []providers.NodeDefinition{
			{ID: "start", Type: "START"},
		},
	}
	updatedFlow := &providers.CompleteFlowDefinition{
		ID:       testFlowIDHandler,
		Handle:   "test-handle",
		Name:     "Updated Flow",
		FlowType: providers.FlowTypeAuthentication,
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
	var response providers.CompleteFlowDefinition
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
		FlowType: providers.FlowTypeAuthentication,
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
	restoredFlow := &providers.CompleteFlowDefinition{
		ID:       testFlowIDHandler,
		Handle:   "test-handle",
		Name:     "Restored Flow",
		FlowType: providers.FlowTypeAuthentication,
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
	var response providers.CompleteFlowDefinition
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
		FlowType: providers.FlowTypeAuthentication,
		Nodes: []providers.NodeDefinition{
			{ID: "start", Type: "START"},
		},
	}

	result := sanitizeFlowDefinitionRequest(input)

	s.Empty(result.OUID)
	s.Equal("Test Flow", result.Name)
	s.Equal(providers.FlowTypeAuthentication, result.FlowType)
	s.Len(result.Nodes, 1)
}

func (s *FlowMgtHandlerTestSuite) TestSanitizeFlowDefinitionRequest_CarriesTheOrganizationUnit() {
	result := sanitizeFlowDefinitionRequest(&FlowDefinitionRequest{
		Handle: "test-handle", Name: "Test Flow", FlowType: providers.FlowTypeAuthentication, OUID: "  ou-1 ",
		Nodes: []providers.NodeDefinition{{ID: "start", Type: "START"}},
	})

	s.Equal("ou-1", result.OUID)
}

func (s *FlowMgtHandlerTestSuite) TestCreateFlow_OwnedByTheOUInTheBody() {
	body := &FlowDefinitionRequest{
		Handle: "new-flow-handle", Name: "New Flow", FlowType: providers.FlowTypeAuthentication, OUID: "ou-child",
		Nodes: []providers.NodeDefinition{{ID: "start", Type: "START"}},
	}
	s.mockService.EXPECT().CreateFlow(mock.Anything, mock.MatchedBy(func(def *FlowDefinition) bool {
		return def.OUID == "ou-child" && def.Handle == "new-flow-handle"
	})).Return(&providers.CompleteFlowDefinition{ID: testFlowIDHandler, OUID: "ou-child"}, nil)
	payload, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/flows", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.handler.createFlow(w, req)

	s.Equal(http.StatusCreated, w.Code)
	var response providers.CompleteFlowDefinition
	s.NoError(json.Unmarshal(w.Body.Bytes(), &response))
	s.Equal("ou-child", response.OUID)
}

func boolPtr(v bool) *bool { return &v }

// With an organization unit in the request, the list and the flow are as that unit sees them.

func (s *FlowMgtHandlerTestSuite) TestListFlows_ForAnOrganizationUnit() {
	expectedList := &FlowListResponse{Flows: []BasicFlowDefinition{}, Count: 0}
	s.mockService.EXPECT().ListFlowsForOU(
		mock.Anything, testOUIDHandler, providers.FlowTypeAuthentication, 30, 0).
		Return(expectedList, nil)

	req := httptest.NewRequest(http.MethodGet,
		"/flows?filter="+url.QueryEscape(`ouId eq "`+testOUIDHandler+`"`)+"&flowType=AUTHENTICATION", nil)
	w := httptest.NewRecorder()

	s.handler.listFlows(w, req)

	s.Equal(http.StatusOK, w.Code)
}

func (s *FlowMgtHandlerTestSuite) TestListFlows_AFlowSharedWithTheUnitIsListedAsAnOwnedOneIs() {
	list := &FlowListResponse{Count: 2, Flows: []BasicFlowDefinition{
		{ID: "owned", Handle: "o", Name: "Owned", ActiveVersion: 2, CreatedAt: "t", UpdatedAt: "t", OUID: "ou-a",
			IsOwnedByOU: boolPtr(true)},
		{ID: "shared", Handle: "s", Name: "Shared", ActiveVersion: 3, CreatedAt: "t", UpdatedAt: "t", OUID: "ou-b",
			IsOwnedByOU: boolPtr(false), IsReadOnly: true},
	}}
	s.mockService.EXPECT().ListFlowsForOU(mock.Anything, testOUIDHandler, providers.FlowType(""), 30, 0).
		Return(list, nil)

	req := httptest.NewRequest(
		http.MethodGet, "/flows?filter="+url.QueryEscape(`ouId eq "`+testOUIDHandler+`"`), nil)
	w := httptest.NewRecorder()

	s.handler.listFlows(w, req)

	var response struct {
		Flows []map[string]interface{} `json:"flows"`
	}
	s.NoError(json.Unmarshal(w.Body.Bytes(), &response))
	s.Equal(float64(2), response.Flows[0]["activeVersion"])
	s.Equal(float64(3), response.Flows[1]["activeVersion"])
	s.Contains(response.Flows[1], "createdAt")
	s.Equal(true, response.Flows[0]["isOwnedByOU"])
	s.Equal(false, response.Flows[1]["isOwnedByOU"])
	s.Equal(true, response.Flows[1]["isReadOnly"])
}

func (s *FlowMgtHandlerTestSuite) TestListFlows_WithoutAnOrganizationUnitEntriesAreAsTheyAlwaysWere() {
	list := &FlowListResponse{Count: 1, Flows: []BasicFlowDefinition{{ID: "f", Handle: "h", Name: "F"}}}
	s.mockService.EXPECT().ListFlows(mock.Anything, 30, 0, providers.FlowType("")).Return(list, nil)

	w := httptest.NewRecorder()
	s.handler.listFlows(w, httptest.NewRequest(http.MethodGet, "/flows", nil))

	var response struct {
		Flows []map[string]interface{} `json:"flows"`
	}
	s.NoError(json.Unmarshal(w.Body.Bytes(), &response))
	s.Contains(response.Flows[0], "activeVersion")
	s.Contains(response.Flows[0], "createdAt")
	s.NotContains(response.Flows[0], "isOwnedByOU")
}

func (s *FlowMgtHandlerTestSuite) TestGetFlow_AsTheOwnerSeesIt() {
	flow := &providers.CompleteFlowDefinition{ID: testFlowIDHandler, Handle: "h", Name: "F"}
	s.mockService.EXPECT().GetFlowForOU(mock.Anything, testFlowIDHandler, testOUIDHandler).
		Return(&FlowView{Owned: &OwnedFlow{CompleteFlowDefinition: flow, IsOwnedByOU: true}}, nil)

	req := httptest.NewRequest(http.MethodGet, "/flows/"+testFlowIDHandler+"?ouId="+testOUIDHandler, nil)
	req.SetPathValue(pathParamFlowID, testFlowIDHandler)
	w := httptest.NewRecorder()

	s.handler.getFlow(w, req)

	s.Equal(http.StatusOK, w.Code)
	var response map[string]interface{}
	s.NoError(json.Unmarshal(w.Body.Bytes(), &response))
	s.Equal(testFlowIDHandler, response["id"])
	s.Equal(true, response["isOwnedByOU"])
}

func (s *FlowMgtHandlerTestSuite) TestGetFlow_SharedFlowShowsItsVariablesOnly() {
	shared := &SharedFlow{
		ID: testFlowIDHandler, Handle: "test-handle", Name: "Test Flow", FlowType: providers.FlowTypeAuthentication,
		OUID: "owner-ou", IsOwnedByOU: false, IsReadOnly: true,
		Variables: []providers.VariableDefinition{
			{Identifier: "supportEmail", Type: providers.InputTypeEmail, DefaultValue: "a@b.co"},
		},
	}
	s.mockService.EXPECT().GetFlowForOU(mock.Anything, testFlowIDHandler, testOUIDHandler).
		Return(&FlowView{Shared: shared}, nil)

	req := httptest.NewRequest(http.MethodGet, "/flows/"+testFlowIDHandler+"?ouId="+testOUIDHandler, nil)
	req.SetPathValue(pathParamFlowID, testFlowIDHandler)
	w := httptest.NewRecorder()

	s.handler.getFlow(w, req)

	s.Equal(http.StatusOK, w.Code)
	var response map[string]interface{}
	s.NoError(json.Unmarshal(w.Body.Bytes(), &response))
	s.Equal(false, response["isOwnedByOU"])
	s.Equal(true, response["isReadOnly"])
	s.NotContains(response, "nodes")
	s.Len(response["variables"], 1)
}

func (s *FlowMgtHandlerTestSuite) TestGetFlow_NotSharedWithTheUnitIsNotFound() {
	s.mockService.EXPECT().GetFlowForOU(mock.Anything, testFlowIDHandler, testOUIDHandler).
		Return(nil, &ErrorFlowNotFound)

	req := httptest.NewRequest(http.MethodGet, "/flows/"+testFlowIDHandler+"?ouId="+testOUIDHandler, nil)
	req.SetPathValue(pathParamFlowID, testFlowIDHandler)
	w := httptest.NewRecorder()

	s.handler.getFlow(w, req)

	s.Equal(http.StatusNotFound, w.Code)
}

// Without an organization unit the whole definition comes back. The filter is a list parameter, and a single flow
// does not read it.
func (s *FlowMgtHandlerTestSuite) TestGetFlow_WithoutAnOrganizationUnitIsTheWholeDefinition() {
	s.mockService.EXPECT().GetFlow(mock.Anything, testFlowIDHandler).
		Return(&providers.CompleteFlowDefinition{ID: testFlowIDHandler}, nil).Twice()

	for _, target := range []string{
		"/flows/" + testFlowIDHandler,
		"/flows/" + testFlowIDHandler + "?filter=" + url.QueryEscape(`ouId eq "`+testOUIDHandler+`"`),
	} {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		req.SetPathValue(pathParamFlowID, testFlowIDHandler)
		w := httptest.NewRecorder()

		s.handler.getFlow(w, req)

		s.Equal(http.StatusOK, w.Code, target)
	}
}

func (s *FlowMgtHandlerTestSuite) TestListFlows_RefusesAFilterItCannotApply() {
	for _, filterExpr := range []string{
		"ouId eq " + testOUIDHandler,
		`ouId eq ""`,
		`ouId eq 5`,
		`ouId sw "ou"`,
		`handle eq "h"`,
		`ouId eq "` + testOUIDHandler + `" and handle eq "h"`,
	} {
		w := httptest.NewRecorder()

		s.handler.listFlows(w, httptest.NewRequest(http.MethodGet, "/flows?filter="+url.QueryEscape(filterExpr), nil))

		s.Equal(http.StatusBadRequest, w.Code, filterExpr)
		s.Contains(w.Body.String(), ErrorInvalidFilter.Code, filterExpr)
	}
}

// ---------------------------------------------------------------------------
// Sharing, overlay rules and variables
// ---------------------------------------------------------------------------

func (s *FlowMgtHandlerTestSuite) request(method, target, body string) (*httptest.ResponseRecorder, *http.Request) {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.SetPathValue(pathParamFlowID, testFlowIDHandler)
	req.SetPathValue(pathParamPolicyID, "p1")
	return httptest.NewRecorder(), req
}

const allChildrenBody = `{"childUnits":{"mode":"all","overlayRules":{"variables.supportEmail":{"editable":true}}}}`

func (s *FlowMgtHandlerTestSuite) TestCreateSharingPolicy() {
	s.mockService.EXPECT().CreateSharingPolicy(mock.Anything, testFlowIDHandler, mock.MatchedBy(
		func(req sharing.PolicyRequest) bool {
			return req.ChildUnits != nil && req.ChildUnits.OverlayRules["variables.supportEmail"].Editable
		})).Return(sharing.Policy{ID: "p1"}, nil)
	w, r := s.request(http.MethodPost, "/flows/f/sharing-policies", allChildrenBody)

	s.handler.createSharingPolicy(w, r)

	s.Equal(http.StatusCreated, w.Code)
	var resp map[string]interface{}
	s.NoError(json.Unmarshal(w.Body.Bytes(), &resp))
	s.Equal("p1", resp["id"])
}

func (s *FlowMgtHandlerTestSuite) TestCreateSharingPolicyRefusesWhatItCannotRead() {
	for name, body := range map[string]string{
		"not json":                 `{`,
		"a field it does not know": `{"childUnits":{"mode":"all","excludedOuId":["x"]}}`,
	} {
		s.Run(name, func() {
			w, r := s.request(http.MethodPost, "/flows/f/sharing-policies", body)

			s.handler.createSharingPolicy(w, r)

			s.Equal(http.StatusBadRequest, w.Code)
		})
	}
}

func (s *FlowMgtHandlerTestSuite) TestCreateSharingPolicyLeavesDeploymentWideReachToResourceFiles() {
	for name, body := range map[string]string{
		"every unit":       `{"allUnits":{"overlayRules":{"variables.supportEmail":{"editable":true}}}}`,
		"every other tree": `{"otherTreeRoots":{"mode":"all"}}`,
	} {
		s.Run(name, func() {
			w, r := s.request(http.MethodPost, "/flows/f/sharing-policies", body)

			s.handler.createSharingPolicy(w, r)

			s.Equal(http.StatusBadRequest, w.Code)
			s.Contains(w.Body.String(), ErrorDeploymentWideSharingNotAllowed.Code)
		})
	}
}

func (s *FlowMgtHandlerTestSuite) TestSharingErrorsKeepTheirStatus() {
	for svcErr, status := range map[*tidcommon.ServiceError]int{
		&ErrorFlowNotFound:                   http.StatusNotFound,
		&ErrorSharingPolicyExists:            http.StatusConflict,
		&ErrorSharingPolicyVersionMismatch:   http.StatusConflict,
		&ErrorInvalidSharingPolicy:           http.StatusBadRequest,
		&ErrorFlowConflictsWithSharingPolicy: http.StatusConflict,
		&ErrorFlowConflictsWithSavedValues:   http.StatusConflict,
		&tidcommon.InternalServerError:       http.StatusInternalServerError,
	} {
		s.mockService.EXPECT().CreateSharingPolicy(mock.Anything, testFlowIDHandler, mock.Anything).
			Return(sharing.Policy{}, svcErr).Once()
		w, r := s.request(http.MethodPost, "/flows/f/sharing-policies", allChildrenBody)

		s.handler.createSharingPolicy(w, r)

		s.Equal(status, w.Code, svcErr.Code)
	}
}

func (s *FlowMgtHandlerTestSuite) TestListSharingPolicies() {
	s.mockService.EXPECT().ListSharingPolicies(mock.Anything, testFlowIDHandler, 30, 0).
		Return(sharing.PolicyList{
			TotalResults: 1, StartIndex: 1, Count: 1, Policies: []sharing.Policy{{ID: "p1"}},
		}, nil).Once()
	w, r := s.request(http.MethodGet, "/flows/f/sharing-policies", "")

	s.handler.listSharingPolicies(w, r)

	s.Equal(http.StatusOK, w.Code)
	s.Contains(w.Body.String(), `"policies"`)

	w, r = s.request(http.MethodGet, "/flows/f/sharing-policies?limit=zero", "")
	s.handler.listSharingPolicies(w, r)
	s.Equal(http.StatusBadRequest, w.Code)

	s.mockService.EXPECT().ListSharingPolicies(mock.Anything, testFlowIDHandler, 30, 0).
		Return(sharing.PolicyList{}, &ErrorFlowNotFound)
	w, r = s.request(http.MethodGet, "/flows/f/sharing-policies", "")
	s.handler.listSharingPolicies(w, r)
	s.Equal(http.StatusNotFound, w.Code)
}

func (s *FlowMgtHandlerTestSuite) TestGetUpdateAndDeleteASharingPolicy() {
	s.mockService.EXPECT().GetSharingPolicy(mock.Anything, testFlowIDHandler, "p1").
		Return(sharing.Policy{ID: "p1"}, nil).Once()
	w, r := s.request(http.MethodGet, "/flows/f/sharing-policies/p1", "")
	s.handler.getSharingPolicy(w, r)
	s.Equal(http.StatusOK, w.Code)

	s.mockService.EXPECT().GetSharingPolicy(mock.Anything, testFlowIDHandler, "p1").
		Return(sharing.Policy{}, &ErrorSharingPolicyNotFound).Once()
	w, r = s.request(http.MethodGet, "/flows/f/sharing-policies/p1", "")
	s.handler.getSharingPolicy(w, r)
	s.Equal(http.StatusNotFound, w.Code)

	s.mockService.EXPECT().UpdateSharingPolicy(mock.Anything, testFlowIDHandler, "p1", mock.MatchedBy(
		func(req sharing.PolicyRequest) bool { return req.Version == 3 })).
		Return(sharing.Policy{ID: "p1", Version: 4}, nil)
	w, r = s.request(http.MethodPut, "/flows/f/sharing-policies/p1", `{"version":3,"childUnits":{"mode":"all"}}`)
	s.handler.updateSharingPolicy(w, r)
	s.Equal(http.StatusOK, w.Code)

	w, r = s.request(http.MethodPut, "/flows/f/sharing-policies/p1", `{"version":`)
	s.handler.updateSharingPolicy(w, r)
	s.Equal(http.StatusBadRequest, w.Code)

	s.mockService.EXPECT().DeleteSharingPolicy(mock.Anything, testFlowIDHandler, "p1").Return(nil).Once()
	w, r = s.request(http.MethodDelete, "/flows/f/sharing-policies/p1", "")
	s.handler.deleteSharingPolicy(w, r)
	s.Equal(http.StatusNoContent, w.Code)

	s.mockService.EXPECT().DeleteSharingPolicy(mock.Anything, testFlowIDHandler, "p1").
		Return(&ErrorSharingPolicyDeclared).Once()
	w, r = s.request(http.MethodDelete, "/flows/f/sharing-policies/p1", "")
	s.handler.deleteSharingPolicy(w, r)
	s.Equal(http.StatusBadRequest, w.Code)
}

func (s *FlowMgtHandlerTestSuite) TestOverlayRules() {
	s.mockService.EXPECT().GetFlowOverlayRules(mock.Anything, testFlowIDHandler, testOUIDHandler).
		Return(sharing.ResolvedOverlay{OUID: testOUIDHandler, Visible: true}, nil).Once()
	w, r := s.request(http.MethodGet, "/flows/f/overlay-rules?ouId="+testOUIDHandler, "")
	s.handler.getOverlayRules(w, r)
	s.Equal(http.StatusOK, w.Code)
	var resp map[string]interface{}
	s.NoError(json.Unmarshal(w.Body.Bytes(), &resp))
	s.Equal(true, resp["visible"])
	s.Equal(map[string]interface{}{}, resp["rules"], "no entry for what no policy names, and never null")

	s.mockService.EXPECT().GetFlowOverlayRules(mock.Anything, testFlowIDHandler, "").
		Return(sharing.ResolvedOverlay{}, &ErrorMissingOUID)
	w, r = s.request(http.MethodGet, "/flows/f/overlay-rules", "")
	s.handler.getOverlayRules(w, r)
	s.Equal(http.StatusBadRequest, w.Code)

	s.mockService.EXPECT().GetFlowOverlayRules(mock.Anything, testFlowIDHandler, "x").
		Return(sharing.ResolvedOverlay{}, &ErrorFlowNotFound)
	w, r = s.request(http.MethodGet, "/flows/f/overlay-rules?ouId=x", "")
	s.handler.getOverlayRules(w, r)
	s.Equal(http.StatusNotFound, w.Code)
}

func (s *FlowMgtHandlerTestSuite) TestGetAndSetVariables() {
	s.mockService.EXPECT().GetVariables(mock.Anything, testFlowIDHandler, testOUIDHandler).
		Return(&VariablesResponse{Values: map[string]interface{}{"otp": 90.0}}, nil)
	w, r := s.request(http.MethodGet, "/flows/f/variables?ouId="+testOUIDHandler, "")
	s.handler.getVariables(w, r)
	s.Equal(http.StatusOK, w.Code)
	s.JSONEq(`{"values":{"otp":90}}`, w.Body.String())

	s.mockService.EXPECT().SetVariables(mock.Anything, testFlowIDHandler, testOUIDHandler, mock.MatchedBy(
		func(req SetVariablesRequest) bool { return req.Values["supportEmail"] == "help@emea.example" })).
		Return(&VariablesResponse{Values: map[string]interface{}{"supportEmail": "help@emea.example"}}, nil)
	w, r = s.request(http.MethodPut, "/flows/f/variables",
		`{"ouId":"`+testOUIDHandler+`","values":{"supportEmail":"help@emea.example"}}`)
	s.handler.setVariables(w, r)
	s.Equal(http.StatusOK, w.Code)

	for name, body := range map[string]string{
		"not json": `{`, "no values": `{}`, "an unknown field": `{"values":{},"x":1}`,
	} {
		w, r = s.request(http.MethodPut, "/flows/f/variables", body)
		s.handler.setVariables(w, r)
		s.Equal(http.StatusBadRequest, w.Code, name)
	}

	s.mockService.EXPECT().SetVariables(mock.Anything, testFlowIDHandler, "x", mock.Anything).
		Return(nil, variableNotEditable("mfa"))
	w, r = s.request(http.MethodPut, "/flows/f/variables", `{"ouId":"x","values":{"mfa":"SMS_OTP"}}`)
	s.handler.setVariables(w, r)
	s.Equal(http.StatusForbidden, w.Code)

	// The unit is named in the body, so one in the query is not read.
	s.mockService.EXPECT().SetVariables(mock.Anything, testFlowIDHandler, "", mock.Anything).
		Return(nil, &ErrorMissingOUID)
	w, r = s.request(http.MethodPut, "/flows/f/variables?ouId="+testOUIDHandler, `{"values":{}}`)
	s.handler.setVariables(w, r)
	s.Equal(http.StatusBadRequest, w.Code)
}

func (s *FlowMgtHandlerTestSuite) TestAFlowSharedWithTheUnitCannotBeChangedThroughIt() {
	s.mockService.EXPECT().CheckFlowAccess(mock.Anything, testFlowIDHandler, testOUIDHandler, true).
		Return(&ErrorSharedFlowReadOnly)
	w, r := s.request(http.MethodDelete, "/flows/f?ouId="+testOUIDHandler, "")

	s.handler.deleteFlow(w, r)

	s.Equal(http.StatusForbidden, w.Code)
	s.Contains(w.Body.String(), ErrorSharedFlowReadOnly.Code)
}

func (s *FlowMgtHandlerTestSuite) TestADefinitionIsNotShownToAUnitItWasOnlySharedWith() {
	s.mockService.EXPECT().CheckFlowAccess(mock.Anything, testFlowIDHandler, testOUIDHandler, false).
		Return(&ErrorFlowNotFound)
	w, r := s.request(http.MethodGet, "/flows/f/versions?ouId="+testOUIDHandler, "")

	s.handler.listFlowVersions(w, r)

	s.Equal(http.StatusNotFound, w.Code)
}

func (s *FlowMgtHandlerTestSuite) TestACallerOutsideTheUnitIsForbidden() {
	s.mockService.EXPECT().GetFlowOverlayRules(mock.Anything, testFlowIDHandler, "x").
		Return(sharing.ResolvedOverlay{}, &tidcommon.ErrorUnauthorized)
	w, r := s.request(http.MethodGet, "/flows/f/overlay-rules?ouId=x", "")

	s.handler.getOverlayRules(w, r)

	s.Equal(http.StatusForbidden, w.Code)
}
