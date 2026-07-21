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

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/healthcheck/model"

	dbprovidermock "github.com/thunder-id/thunderid/tests/mocks/database/providermock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type HealthCheckServiceTestSuite struct {
	suite.Suite
	service                HealthCheckServiceInterface
	mockDBProvider         *dbprovidermock.DBProviderInterfaceMock
	mockConfigDB           *dbprovidermock.DBClientInterfaceMock
	mockRuntimeTransientDB *dbprovidermock.DBClientInterfaceMock
	mockEntityDB           *dbprovidermock.DBClientInterfaceMock
}

func TestHealthCheckServiceSuite(t *testing.T) {
	suite.Run(t, new(HealthCheckServiceTestSuite))
}

func (suite *HealthCheckServiceTestSuite) SetupTest() {
	testConfig := &config.Config{
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
			RuntimeTransient: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
			Entity: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
	}
	_ = config.InitializeServerRuntime("test", testConfig)

	suite.service = Initialize(nil, nil)
}

func (suite *HealthCheckServiceTestSuite) BeforeTest(suiteName, testName string) {
	dbClientConfig := &dbprovidermock.DBClientInterfaceMock{}
	suite.mockConfigDB = dbClientConfig

	dbClientRuntime := &dbprovidermock.DBClientInterfaceMock{}
	suite.mockRuntimeTransientDB = dbClientRuntime

	dbClientEntity := &dbprovidermock.DBClientInterfaceMock{}
	suite.mockEntityDB = dbClientEntity

	dbProvider := &dbprovidermock.DBProviderInterfaceMock{}
	dbProvider.On("GetConfigDBClient").Return(dbClientConfig, nil)
	dbProvider.On("GetRuntimeTransientDBClient").Return(dbClientRuntime, nil)
	dbProvider.On("GetEntityDBClient").Return(dbClientEntity, nil)
	suite.mockDBProvider = dbProvider
	suite.service.(*HealthCheckService).DBProvider = dbProvider
}

func (suite *HealthCheckServiceTestSuite) TestCheckReadiness() {
	const (
		tcAllDBUp                = "AllDatabasesUp"
		tcConfigDBDown           = "ConfigDBDown"
		tcRuntimeTransientDBDown = "RuntimeTransientDBDown"
		tcEntityDBDown           = "EntityDBDown"
		tcAllThreeDBDown         = "AllThreeDBsDown"
	)
	testCases := []struct {
		name                    string
		setupConfigDB           func()
		setupRuntimeTransientDB func()
		setupEntityDB           func()
		expectedStatus          model.Status
		expectedServiceCount    int
	}{
		{
			name: tcAllDBUp,
			setupConfigDB: func() {
				suite.mockConfigDB.On("Query", queryConfigDBTable).Return([]map[string]interface{}{
					{"1": 1}}, nil)
			},
			setupRuntimeTransientDB: func() {
				suite.mockRuntimeTransientDB.On("Query", queryRuntimeTransientDBTable).Return([]map[string]interface{}{
					{"1": 1}}, nil)
			},
			setupEntityDB: func() {
				suite.mockEntityDB.On("Query", queryEntityDBTable).Return([]map[string]interface{}{
					{"1": 1}}, nil)
			},
			expectedStatus:       model.StatusUp,
			expectedServiceCount: 3,
		},
		{
			name: tcConfigDBDown,
			setupConfigDB: func() {
				suite.mockConfigDB.On("Query", queryConfigDBTable).Return(nil, errors.New("database error"))
			},
			setupRuntimeTransientDB: func() {
				suite.mockRuntimeTransientDB.On("Query", queryRuntimeTransientDBTable).Return([]map[string]interface{}{
					{"1": 1}}, nil)
			},
			setupEntityDB: func() {
				suite.mockEntityDB.On("Query", queryEntityDBTable).Return([]map[string]interface{}{
					{"1": 1}}, nil)
			},
			expectedStatus:       model.StatusDown,
			expectedServiceCount: 3,
		},
		{
			name: tcRuntimeTransientDBDown,
			setupConfigDB: func() {
				suite.mockConfigDB.On("Query", queryConfigDBTable).Return([]map[string]interface{}{
					{"1": 1}}, nil)
			},
			setupRuntimeTransientDB: func() {
				suite.mockRuntimeTransientDB.On("Query", queryRuntimeTransientDBTable).
					Return(nil, errors.New("database error"))
			},
			setupEntityDB: func() {
				suite.mockEntityDB.On("Query", queryEntityDBTable).Return([]map[string]interface{}{
					{"1": 1}}, nil)
			},
			expectedStatus:       model.StatusDown,
			expectedServiceCount: 3,
		},
		{
			name: tcEntityDBDown,
			setupConfigDB: func() {
				suite.mockConfigDB.On("Query", queryConfigDBTable).Return([]map[string]interface{}{
					{"1": 1}}, nil)
			},
			setupRuntimeTransientDB: func() {
				suite.mockRuntimeTransientDB.On("Query", queryRuntimeTransientDBTable).
					Return(nil, errors.New("database error"))
			},
			setupEntityDB: func() {
				suite.mockEntityDB.On("Query", queryEntityDBTable).Return(nil, errors.New("database error"))
			},
			expectedStatus:       model.StatusDown,
			expectedServiceCount: 3,
		},
		{
			name: tcAllThreeDBDown,
			setupConfigDB: func() {
				suite.mockConfigDB.On("Query", queryConfigDBTable).Return(nil, errors.New("database error"))
			},
			setupRuntimeTransientDB: func() {
				suite.mockRuntimeTransientDB.On("Query", queryRuntimeTransientDBTable).
					Return(nil, errors.New("database error"))
			},
			setupEntityDB: func() {
				suite.mockEntityDB.On("Query", queryEntityDBTable).Return(nil, errors.New("database error"))
			},
			expectedStatus:       model.StatusDown,
			expectedServiceCount: 3,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			// Reset mock expectations
			suite.mockConfigDB.ExpectedCalls = nil
			suite.mockRuntimeTransientDB.ExpectedCalls = nil
			suite.mockEntityDB.ExpectedCalls = nil

			// Setup database mocks
			if tc.setupConfigDB != nil {
				tc.setupConfigDB()
			}
			if tc.setupRuntimeTransientDB != nil {
				tc.setupRuntimeTransientDB()
			}
			if tc.setupEntityDB != nil {
				tc.setupEntityDB()
			}

			// Execute the method being tested
			serverStatus := suite.service.CheckReadiness(context.Background())

			// Assertions
			assert.Equal(t, tc.expectedStatus, serverStatus.Status, "Server status should match expected")
			assert.Equal(t, tc.expectedServiceCount, len(serverStatus.ServiceStatus),
				"Service status count should match expected")

			serviceNames := make(map[string]bool)
			for _, status := range serverStatus.ServiceStatus {
				serviceNames[status.ServiceName] = true
			}
			assert.True(t, serviceNames["ConfigDB"], "ConfigDB service status should be present")
			assert.True(t, serviceNames["RuntimeTransientDB"], "RuntimeTransientDB service status should be present")
			assert.True(t, serviceNames["EntityDB"], "EntityDB service status should be present")

			// If config DB is expected down, verify it's reported as down
			if tc.name == tcConfigDBDown || tc.name == "ConfigDBClientError" || tc.name == tcAllThreeDBDown {
				for _, status := range serverStatus.ServiceStatus {
					if status.ServiceName == "ConfigDB" {
						assert.Equal(t, model.StatusDown, status.Status, "ConfigDB should be DOWN")
					}
				}
			}

			// If runtime transient DB is expected down, verify it's reported as down
			if tc.name == tcRuntimeTransientDBDown || tc.name == "RuntimeTransientDBClientError" ||
				tc.name == tcAllThreeDBDown {
				for _, status := range serverStatus.ServiceStatus {
					if status.ServiceName == "RuntimeTransientDB" {
						assert.Equal(t, model.StatusDown, status.Status, "RuntimeTransientDB should be DOWN")
					}
				}
			}

			// If entity DB is expected down, verify it's reported as down
			if tc.name == tcEntityDBDown || tc.name == "EntityDBClientError" || tc.name == tcAllThreeDBDown {
				for _, status := range serverStatus.ServiceStatus {
					if status.ServiceName == "EntityDB" {
						assert.Equal(t, model.StatusDown, status.Status, "EntityDB should be DOWN")
					}
				}
			}

			// Verify that the mock expectations were met
			suite.mockDBProvider.AssertExpectations(t)
			suite.mockConfigDB.AssertExpectations(t)
			suite.mockRuntimeTransientDB.AssertExpectations(t)
			suite.mockEntityDB.AssertExpectations(t)
		})
	}
}

func (suite *HealthCheckServiceTestSuite) TestCheckReadiness_DBRetrievalError() {
	suite.mockDBProvider.ExpectedCalls = nil
	suite.mockDBProvider.On("GetConfigDBClient").Return(nil, errors.New("failed to get config DB client"))
	suite.mockDBProvider.On("GetRuntimeTransientDBClient").
		Return(nil, errors.New("failed to get runtime transient DB client"))
	suite.mockDBProvider.On("GetEntityDBClient").Return(nil, errors.New("failed to get entity DB client"))

	// Execute the method being tested
	serverStatus := suite.service.CheckReadiness(context.Background())

	// Assertions
	assert.Equal(suite.T(), model.StatusDown, serverStatus.Status, "Server status should be DOWN")
	assert.Len(suite.T(), serverStatus.ServiceStatus, 3, "There should be three service statuses reported")

	for _, status := range serverStatus.ServiceStatus {
		if status.ServiceName == "ConfigDB" {
			assert.Equal(suite.T(), model.StatusDown, status.Status, "ConfigDB should be DOWN")
		} else if status.ServiceName == "RuntimeTransientDB" {
			assert.Equal(suite.T(), model.StatusDown, status.Status, "RuntimeTransientDB should be DOWN")
		} else if status.ServiceName == "EntityDB" {
			assert.Equal(suite.T(), model.StatusDown, status.Status, "EntityDB should be DOWN")
		}
	}

	suite.mockDBProvider.AssertExpectations(suite.T())
}
