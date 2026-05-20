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
	service        HealthCheckServiceInterface
	mockDBProvider *dbprovidermock.DBProviderInterfaceMock
	mockConfigDB   *dbprovidermock.DBClientInterfaceMock
	mockRuntimeDB  *dbprovidermock.DBClientInterfaceMock
	mockUserDB     *dbprovidermock.DBClientInterfaceMock
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
			Runtime: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
			User: config.DataSource{
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
	suite.mockRuntimeDB = dbClientRuntime

	dbClientUser := &dbprovidermock.DBClientInterfaceMock{}
	suite.mockUserDB = dbClientUser

	dbProvider := &dbprovidermock.DBProviderInterfaceMock{}
	dbProvider.On("GetConfigDBClient").Return(dbClientConfig, nil)
	dbProvider.On("GetRuntimeDBClient").Return(dbClientRuntime, nil)
	dbProvider.On("GetUserDBClient").Return(dbClientUser, nil)
	suite.mockDBProvider = dbProvider
	suite.service.(*HealthCheckService).DBProvider = dbProvider
}

func (suite *HealthCheckServiceTestSuite) TestCheckReadiness() {
	const (
		tcAllDBUp        = "AllDatabasesUp"
		tcConfigDBDown   = "ConfigDBDown"
		tcRuntimeDBDown  = "RuntimeDBDown"
		tcUserDBDown     = "UserDBDown"
		tcAllThreeDBDown = "AllThreeDBsDown"
	)
	testCases := []struct {
		name                 string
		setupConfigDB        func()
		setupRuntimeDB       func()
		setupUserDB          func()
		expectedStatus       model.Status
		expectedServiceCount int
	}{
		{
			name: tcAllDBUp,
			setupConfigDB: func() {
				suite.mockConfigDB.On("Query", queryConfigDBTable).Return([]map[string]interface{}{
					{"1": 1}}, nil)
			},
			setupRuntimeDB: func() {
				suite.mockRuntimeDB.On("Query", queryRuntimeDBTable).Return([]map[string]interface{}{
					{"1": 1}}, nil)
			},
			setupUserDB: func() {
				suite.mockUserDB.On("Query", queryUserDBTable).Return([]map[string]interface{}{
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
			setupRuntimeDB: func() {
				suite.mockRuntimeDB.On("Query", queryRuntimeDBTable).Return([]map[string]interface{}{
					{"1": 1}}, nil)
			},
			setupUserDB: func() {
				suite.mockUserDB.On("Query", queryUserDBTable).Return([]map[string]interface{}{
					{"1": 1}}, nil)
			},
			expectedStatus:       model.StatusDown,
			expectedServiceCount: 3,
		},
		{
			name: tcRuntimeDBDown,
			setupConfigDB: func() {
				suite.mockConfigDB.On("Query", queryConfigDBTable).Return([]map[string]interface{}{
					{"1": 1}}, nil)
			},
			setupRuntimeDB: func() {
				suite.mockRuntimeDB.On("Query", queryRuntimeDBTable).Return(nil, errors.New("database error"))
			},
			setupUserDB: func() {
				suite.mockUserDB.On("Query", queryUserDBTable).Return([]map[string]interface{}{
					{"1": 1}}, nil)
			},
			expectedStatus:       model.StatusDown,
			expectedServiceCount: 3,
		},
		{
			name: tcUserDBDown,
			setupConfigDB: func() {
				suite.mockConfigDB.On("Query", queryConfigDBTable).Return([]map[string]interface{}{
					{"1": 1}}, nil)
			},
			setupRuntimeDB: func() {
				suite.mockRuntimeDB.On("Query", queryRuntimeDBTable).Return(nil, errors.New("database error"))
			},
			setupUserDB: func() {
				suite.mockUserDB.On("Query", queryUserDBTable).Return(nil, errors.New("database error"))
			},
			expectedStatus:       model.StatusDown,
			expectedServiceCount: 3,
		},
		{
			name: tcAllThreeDBDown,
			setupConfigDB: func() {
				suite.mockConfigDB.On("Query", queryConfigDBTable).Return(nil, errors.New("database error"))
			},
			setupRuntimeDB: func() {
				suite.mockRuntimeDB.On("Query", queryRuntimeDBTable).Return(nil, errors.New("database error"))
			},
			setupUserDB: func() {
				suite.mockUserDB.On("Query", queryUserDBTable).Return(nil, errors.New("database error"))
			},
			expectedStatus:       model.StatusDown,
			expectedServiceCount: 3,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			// Reset mock expectations
			suite.mockConfigDB.ExpectedCalls = nil
			suite.mockRuntimeDB.ExpectedCalls = nil
			suite.mockUserDB.ExpectedCalls = nil

			// Setup database mocks
			if tc.setupConfigDB != nil {
				tc.setupConfigDB()
			}
			if tc.setupRuntimeDB != nil {
				tc.setupRuntimeDB()
			}
			if tc.setupUserDB != nil {
				tc.setupUserDB()
			}

			// Execute the method being tested
			serverStatus := suite.service.CheckReadiness()

			// Assertions
			assert.Equal(t, tc.expectedStatus, serverStatus.Status, "Server status should match expected")
			assert.Equal(t, tc.expectedServiceCount, len(serverStatus.ServiceStatus),
				"Service status count should match expected")

			serviceNames := make(map[string]bool)
			for _, status := range serverStatus.ServiceStatus {
				serviceNames[status.ServiceName] = true
			}
			assert.True(t, serviceNames["ConfigDB"], "ConfigDB service status should be present")
			assert.True(t, serviceNames["RuntimeDB"], "RuntimeDB service status should be present")
			assert.True(t, serviceNames["UserDB"], "UserDB service status should be present")

			// If config DB is expected down, verify it's reported as down
			if tc.name == tcConfigDBDown || tc.name == "ConfigDBClientError" || tc.name == tcAllThreeDBDown {
				for _, status := range serverStatus.ServiceStatus {
					if status.ServiceName == "ConfigDB" {
						assert.Equal(t, model.StatusDown, status.Status, "ConfigDB should be DOWN")
					}
				}
			}

			// If runtime DB is expected down, verify it's reported as down
			if tc.name == tcRuntimeDBDown || tc.name == "RuntimeDBClientError" || tc.name == tcAllThreeDBDown {
				for _, status := range serverStatus.ServiceStatus {
					if status.ServiceName == "RuntimeDB" {
						assert.Equal(t, model.StatusDown, status.Status, "RuntimeDB should be DOWN")
					}
				}
			}

			// If user DB is expected down, verify it's reported as down
			if tc.name == tcUserDBDown || tc.name == "UserDBClientError" || tc.name == tcAllThreeDBDown {
				for _, status := range serverStatus.ServiceStatus {
					if status.ServiceName == "UserDB" {
						assert.Equal(t, model.StatusDown, status.Status, "UserDB should be DOWN")
					}
				}
			}

			// Verify that the mock expectations were met
			suite.mockDBProvider.AssertExpectations(t)
			suite.mockConfigDB.AssertExpectations(t)
			suite.mockRuntimeDB.AssertExpectations(t)
			suite.mockUserDB.AssertExpectations(t)
		})
	}
}

func (suite *HealthCheckServiceTestSuite) TestCheckReadiness_DBRetrievalError() {
	suite.mockDBProvider.ExpectedCalls = nil
	suite.mockDBProvider.On("GetConfigDBClient").Return(nil, errors.New("failed to get config DB client"))
	suite.mockDBProvider.On("GetRuntimeDBClient").Return(nil, errors.New("failed to get runtime DB client"))
	suite.mockDBProvider.On("GetUserDBClient").Return(nil, errors.New("failed to get user DB client"))

	// Execute the method being tested
	serverStatus := suite.service.CheckReadiness()

	// Assertions
	assert.Equal(suite.T(), model.StatusDown, serverStatus.Status, "Server status should be DOWN")
	assert.Len(suite.T(), serverStatus.ServiceStatus, 3, "There should be three service statuses reported")

	for _, status := range serverStatus.ServiceStatus {
		if status.ServiceName == "ConfigDB" {
			assert.Equal(suite.T(), model.StatusDown, status.Status, "ConfigDB should be DOWN")
		} else if status.ServiceName == "RuntimeDB" {
			assert.Equal(suite.T(), model.StatusDown, status.Status, "RuntimeDB should be DOWN")
		} else if status.ServiceName == "UserDB" {
			assert.Equal(suite.T(), model.StatusDown, status.Status, "UserDB should be DOWN")
		}
	}

	suite.mockDBProvider.AssertExpectations(suite.T())
}
