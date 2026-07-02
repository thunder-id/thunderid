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

package flowexec

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

const testFlowExecRequestBody = `{"applicationId":"app-1","flowType":"AUTHENTICATION","action":"submit"}`

type HandlerTestSuite struct {
	suite.Suite
}

func TestHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(HandlerTestSuite))
}

func (s *HandlerTestSuite) TestNewFlowExecutionHandler() {
	t := s.T()
	mockSvc := NewFlowExecServiceInterfaceMock(t)
	h := newFlowExecutionHandler(mockSvc)
	s.NotNil(h)
	s.Equal(mockSvc, h.flowExecService)
}

func (s *HandlerTestSuite) TestHandleFlowError_ClientError_Returns400() {
	w := httptest.NewRecorder()
	svcErr := &tidcommon.ServiceError{
		Code: "FES-4001",
		Type: tidcommon.ClientErrorType,
		Error: tidcommon.I18nMessage{
			Key:          "client.error",
			DefaultValue: "bad request",
		},
	}
	handleFlowError(context.Background(), w, svcErr)
	s.Equal(http.StatusBadRequest, w.Code)
}

func (s *HandlerTestSuite) TestHandleFlowError_ForbiddenError_Returns403() {
	w := httptest.NewRecorder()
	handleFlowError(context.Background(), w, &ErrorDirectFlowInitiationNotPermitted)
	s.Equal(http.StatusForbidden, w.Code)
}

func (s *HandlerTestSuite) TestHandleFlowError_ServerError_Returns500() {
	w := httptest.NewRecorder()
	svcErr := &tidcommon.ServiceError{
		Code: "FES-5001",
		Type: tidcommon.ServerErrorType,
		Error: tidcommon.I18nMessage{
			Key:          "server.error",
			DefaultValue: "internal error",
		},
	}
	handleFlowError(context.Background(), w, svcErr)
	s.Equal(http.StatusInternalServerError, w.Code)
}

func (s *HandlerTestSuite) TestConvertToAPIError() {
	svcErr := &tidcommon.ServiceError{
		Code: "FES-1234",
		Error: tidcommon.I18nMessage{
			Key:          "test.error",
			DefaultValue: "test message",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "test.error.desc",
			DefaultValue: "test description",
		},
	}
	resp := convertToAPIError(svcErr)
	s.Equal("FES-1234", resp.Code)
}

func (s *HandlerTestSuite) TestHandleFlowExecutionRequest_InvalidJSON() {
	t := s.T()
	mockSvc := NewFlowExecServiceInterfaceMock(t)
	h := newFlowExecutionHandler(mockSvc)

	req := httptest.NewRequest(http.MethodPost, "/flow/execute", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleFlowExecutionRequest(w, req)
	s.Equal(http.StatusBadRequest, w.Code)
}

func (s *HandlerTestSuite) TestHandleFlowExecutionRequest_ServiceError() {
	t := s.T()
	mockSvc := NewFlowExecServiceInterfaceMock(t)
	mockSvc.EXPECT().Execute(mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, &ErrorDirectFlowInitiationNotPermitted)

	h := newFlowExecutionHandler(mockSvc)
	req := httptest.NewRequest(http.MethodPost, "/flow/execute", bytes.NewBufferString(testFlowExecRequestBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleFlowExecutionRequest(w, req)
	s.Equal(http.StatusForbidden, w.Code)
}

func (s *HandlerTestSuite) TestHandleFlowExecutionRequest_Success() {
	t := s.T()
	mockSvc := NewFlowExecServiceInterfaceMock(t)
	flowStep := &FlowStep{
		ExecutionID: "exec-1",
		Status:      providers.FlowStatusIncomplete,
	}
	mockSvc.EXPECT().Execute(mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(flowStep, (*tidcommon.ServiceError)(nil))

	h := newFlowExecutionHandler(mockSvc)
	req := httptest.NewRequest(http.MethodPost, "/flow/execute", bytes.NewBufferString(testFlowExecRequestBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleFlowExecutionRequest(w, req)
	s.Equal(http.StatusOK, w.Code)
}

func (s *HandlerTestSuite) TestHandleFlowExecutionRequest_StepWithError() {
	t := s.T()
	mockSvc := NewFlowExecServiceInterfaceMock(t)
	stepErr := &tidcommon.ServiceError{
		Code: "FES-9999",
		Error: tidcommon.I18nMessage{
			Key:          "step.error",
			DefaultValue: "step failed",
		},
	}
	flowStep := &FlowStep{
		ExecutionID: "exec-1",
		Status:      providers.FlowStatusError,
		Error:       stepErr,
	}
	mockSvc.EXPECT().Execute(mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(flowStep, (*tidcommon.ServiceError)(nil))

	h := newFlowExecutionHandler(mockSvc)
	req := httptest.NewRequest(http.MethodPost, "/flow/execute", bytes.NewBufferString(testFlowExecRequestBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleFlowExecutionRequest(w, req)
	s.Equal(http.StatusOK, w.Code)
}
