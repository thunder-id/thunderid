/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package agent

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/tests/mocks/agentmock"
)

type AgentHandlerTestSuite struct {
	suite.Suite
	mockService *agentmock.AgentServiceInterfaceMock
	handler     *agentHandler
}

func TestAgentHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(AgentHandlerTestSuite))
}

func (s *AgentHandlerTestSuite) SetupTest() {
	s.mockService = agentmock.NewAgentServiceInterfaceMock(s.T())
	s.handler = newAgentHandler(s.mockService)
}

func (s *AgentHandlerTestSuite) TestHandleAgentListRequest_InvalidFilter() {
	req := httptest.NewRequest(http.MethodGet, "/agents?filter=invalidfilter", nil)
	rr := httptest.NewRecorder()

	s.handler.HandleAgentListRequest(rr, req)

	s.Equal(http.StatusBadRequest, rr.Code)
	s.Contains(rr.Body.String(), ErrorInvalidFilter.Code)
}

func (s *AgentHandlerTestSuite) TestHandleAgentListRequest_ServiceError() {
	req := httptest.NewRequest(http.MethodGet, "/agents", nil)
	rr := httptest.NewRecorder()

	s.mockService.EXPECT().
		GetAgentList(mock.Anything, mock.Anything, mock.Anything, mock.Anything, false).
		Return(nil, &serviceerror.InternalServerError)

	s.handler.HandleAgentListRequest(rr, req)

	s.Equal(http.StatusInternalServerError, rr.Code)
	s.Contains(rr.Body.String(), serviceerror.InternalServerError.Code)
}

func (s *AgentHandlerTestSuite) TestHandleAgentPostRequest_InvalidJSON() {
	req := httptest.NewRequest(http.MethodPost, "/agents", bytes.NewReader([]byte("invalid json")))
	rr := httptest.NewRecorder()

	s.handler.HandleAgentPostRequest(rr, req)

	s.Equal(http.StatusBadRequest, rr.Code)
	s.Contains(rr.Body.String(), ErrorInvalidRequestFormat.Code)
}

func (s *AgentHandlerTestSuite) TestHandleAgentGetRequest_MissingID() {
	req := httptest.NewRequest(http.MethodGet, "/agents/", nil)
	req.SetPathValue("id", "")
	rr := httptest.NewRecorder()

	s.handler.HandleAgentGetRequest(rr, req)

	s.Equal(http.StatusBadRequest, rr.Code)
	s.Contains(rr.Body.String(), ErrorMissingAgentID.Code)
}

func (s *AgentHandlerTestSuite) TestHandleAgentPutRequest_MissingID() {
	req := httptest.NewRequest(http.MethodPut, "/agents/", nil)
	req.SetPathValue("id", "")
	rr := httptest.NewRecorder()

	s.handler.HandleAgentPutRequest(rr, req)

	s.Equal(http.StatusBadRequest, rr.Code)
	s.Contains(rr.Body.String(), ErrorMissingAgentID.Code)
}

func (s *AgentHandlerTestSuite) TestHandleAgentPutRequest_InvalidJSON() {
	req := httptest.NewRequest(http.MethodPut, "/agents/agent1", bytes.NewReader([]byte("invalid json")))
	req.SetPathValue("id", "agent1")
	rr := httptest.NewRecorder()

	s.handler.HandleAgentPutRequest(rr, req)

	s.Equal(http.StatusBadRequest, rr.Code)
	s.Contains(rr.Body.String(), ErrorInvalidRequestFormat.Code)
}

func (s *AgentHandlerTestSuite) TestHandleAgentDeleteRequest_MissingID() {
	req := httptest.NewRequest(http.MethodDelete, "/agents/", nil)
	req.SetPathValue("id", "")
	rr := httptest.NewRecorder()

	s.handler.HandleAgentDeleteRequest(rr, req)

	s.Equal(http.StatusBadRequest, rr.Code)
	s.Contains(rr.Body.String(), ErrorMissingAgentID.Code)
}

func (s *AgentHandlerTestSuite) TestHandleAgentGroupsRequest_MissingID() {
	req := httptest.NewRequest(http.MethodGet, "/agents//groups", nil)
	req.SetPathValue("id", "")
	rr := httptest.NewRecorder()

	s.handler.HandleAgentGroupsRequest(rr, req)

	s.Equal(http.StatusBadRequest, rr.Code)
	s.Contains(rr.Body.String(), ErrorMissingAgentID.Code)
}

func (s *AgentHandlerTestSuite) TestHandleAgentGroupsRequest_InvalidLimit() {
	req := httptest.NewRequest(http.MethodGet, "/agents/agent1/groups?limit=abc", nil)
	req.SetPathValue("id", "agent1")
	rr := httptest.NewRecorder()

	s.handler.HandleAgentGroupsRequest(rr, req)

	s.Equal(http.StatusBadRequest, rr.Code)
	s.Contains(rr.Body.String(), ErrorInvalidLimit.Code)
}

func (s *AgentHandlerTestSuite) TestHandleAgentGroupsRequest_ServiceError() {
	req := httptest.NewRequest(http.MethodGet, "/agents/agent1/groups", nil)
	req.SetPathValue("id", "agent1")
	rr := httptest.NewRecorder()

	s.mockService.EXPECT().
		GetAgentGroups(mock.Anything, "agent1", mock.Anything, mock.Anything).
		Return(nil, &ErrorAgentNotFound)

	s.handler.HandleAgentGroupsRequest(rr, req)

	s.Equal(http.StatusNotFound, rr.Code)
	s.Contains(rr.Body.String(), ErrorAgentNotFound.Code)
}
