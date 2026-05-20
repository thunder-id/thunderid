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

package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/healthcheck/model"
	"github.com/thunder-id/thunderid/tests/mocks/healthcheck/servicemock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type HealthCheckHandlerTestSuite struct {
	suite.Suite
	handler     *HealthCheckHandler
	mockService *servicemock.HealthCheckServiceInterfaceMock
}

func TestHealthCheckHandlerSuite(t *testing.T) {
	suite.Run(t, new(HealthCheckHandlerTestSuite))
}

func (suite *HealthCheckHandlerTestSuite) BeforeTest(suiteName, testName string) {
	suite.mockService = &servicemock.HealthCheckServiceInterfaceMock{}
	suite.handler = NewHealthCheckHandler(suite.mockService)
}

func (suite *HealthCheckHandlerTestSuite) TestHandleLivenessRequest() {
	// Create request and recorder
	req := httptest.NewRequest("GET", "/health/liveness", nil)
	rec := httptest.NewRecorder()

	// Call handler method
	suite.handler.HandleLivenessRequest(rec, req)

	// Assert response
	assert.Equal(suite.T(), http.StatusOK, rec.Code)
}

func (suite *HealthCheckHandlerTestSuite) TestHandleReadinessRequest_AllUp() {
	// Create request and recorder
	req := httptest.NewRequest("GET", "/health/readiness", nil)
	rec := httptest.NewRecorder()

	// Setup mock to return status UP
	serviceStatus := []model.ServiceStatus{
		{
			ServiceName: "ConfigDB",
			Status:      model.StatusUp,
		},
		{
			ServiceName: "RuntimeDB",
			Status:      model.StatusUp,
		},
		{
			ServiceName: "UserDB",
			Status:      model.StatusUp,
		},
	}
	serverStatus := model.ServerStatus{
		Status:        model.StatusUp,
		ServiceStatus: serviceStatus,
	}
	suite.mockService.On("CheckReadiness").Return(serverStatus)

	// Call handler method
	suite.handler.HandleReadinessRequest(rec, req)

	// Assert response
	assert.Equal(suite.T(), http.StatusOK, rec.Code)
	assert.Equal(suite.T(), constants.ContentTypeJSON, rec.Header().Get(constants.ContentTypeHeaderName))

	var response model.ServerStatus
	err := json.NewDecoder(rec.Body).Decode(&response)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), model.StatusUp, response.Status)
	assert.Len(suite.T(), response.ServiceStatus, 3)

	suite.mockService.AssertExpectations(suite.T())
}

func (suite *HealthCheckHandlerTestSuite) TestHandleReadinessRequest_Down() {
	// Create request and recorder
	req := httptest.NewRequest("GET", "/health/readiness", nil)
	rec := httptest.NewRecorder()

	// Setup mock to return status DOWN
	serviceStatus := []model.ServiceStatus{
		{
			ServiceName: "ConfigDB",
			Status:      model.StatusUp,
		},
		{
			ServiceName: "RuntimeDB",
			Status:      model.StatusDown,
		},
		{
			ServiceName: "UserDB",
			Status:      model.StatusDown,
		},
	}
	serverStatus := model.ServerStatus{
		Status:        model.StatusDown,
		ServiceStatus: serviceStatus,
	}
	suite.mockService.On("CheckReadiness").Return(serverStatus)

	// Call handler method
	suite.handler.HandleReadinessRequest(rec, req)

	// Assert response
	assert.Equal(suite.T(), http.StatusServiceUnavailable, rec.Code)
	assert.Equal(suite.T(), constants.ContentTypeJSON, rec.Header().Get(constants.ContentTypeHeaderName))

	var response model.ServerStatus
	err := json.NewDecoder(rec.Body).Decode(&response)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), model.StatusDown, response.Status)

	suite.mockService.AssertExpectations(suite.T())
}
