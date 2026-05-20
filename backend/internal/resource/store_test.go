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
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/database/provider"

	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

var (
	testParentID1   = "parent1"
	testResourceID1 = "res1"
)

const (
	// Test constants for IDs and pagination
	testLimit  = 10
	testOffset = 0
)

// ResourceStoreTestSuite is the test suite for resourceStore.
type ResourceStoreTestSuite struct {
	suite.Suite
	mockDBProvider *providermock.DBProviderInterfaceMock
	mockDBClient   *providermock.DBClientInterfaceMock
	store          *resourceStore
}

// TestResourceStoreTestSuite runs the test suite.
func TestResourceStoreTestSuite(t *testing.T) {
	suite.Run(t, new(ResourceStoreTestSuite))
}

// SetupTest sets up the test suite.
func (suite *ResourceStoreTestSuite) SetupTest() {
	suite.mockDBProvider = providermock.NewDBProviderInterfaceMock(suite.T())
	suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())
	suite.store = &resourceStore{
		dbProvider:   suite.mockDBProvider,
		deploymentID: "test-deployment",
	}
}

// Resource Server Tests

func (suite *ResourceStoreTestSuite) TestCreateResourceServer() {
	testCases := []struct {
		name           string
		resourceID     string
		resourceServer ResourceServer
		setupMocks     func()
		shouldErr      bool
		checkError     func(error) bool
	}{
		{
			name:       "Success",
			resourceID: "rs1",
			resourceServer: ResourceServer{
				OUID:        "ou1",
				Name:        "Test Server",
				Description: "Test Description",
				Identifier:  "test-identifier",
				Delimiter:   ":",
			},
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("ExecuteContext", context.Background(),
					queryCreateResourceServer, "rs1", "ou1", "Test Server",
					"Test Description", nil, "test-identifier", []byte(`{"delimiter":":"}`), "test-deployment").
					Return(int64(1), nil)
			},
			shouldErr: false,
		},
		{
			name:       "ExecuteError",
			resourceID: "rs1",
			resourceServer: ResourceServer{
				OUID:        "ou1",
				Name:        "Test Server",
				Description: "Test Description",
				Identifier:  "test-identifier",
				Delimiter:   ":",
			},
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("ExecuteContext", context.Background(),
					queryCreateResourceServer, "rs1", "ou1", "Test Server",
					"Test Description", nil, "test-identifier", []byte(`{"delimiter":":"}`), "test-deployment").
					Return(int64(0), errors.New("insert failed"))
			},
			shouldErr: true,
			checkError: func(err error) bool {
				suite.Contains(err.Error(), "failed to create resource server")
				return true
			},
		},
		{
			name:       "DBClientError",
			resourceID: "rs1",
			resourceServer: ResourceServer{
				OUID:        "ou1",
				Name:        "Test Server",
				Description: "Test Description",
				Identifier:  "test-identifier",
			},
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(nil, errors.New("database connection error"))
			},
			shouldErr: true,
			checkError: func(err error) bool {
				suite.Contains(err.Error(), "failed to get config DB client")
				return true
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create fresh mocks for each test case
			suite.mockDBProvider = providermock.NewDBProviderInterfaceMock(suite.T())
			suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())
			suite.store = &resourceStore{
				dbProvider:   suite.mockDBProvider,
				deploymentID: "test-deployment",
			}

			tc.setupMocks()

			err := suite.store.CreateResourceServer(context.Background(),
				tc.resourceID, tc.resourceServer)

			if tc.shouldErr {
				suite.Error(err)
				if tc.checkError != nil {
					tc.checkError(err)
				}
			} else {
				suite.NoError(err)
			}
		})
	}
}

func (suite *ResourceStoreTestSuite) TestGetResourceServer() {
	testCases := []struct {
		name               string
		resourceID         string
		setupMocks         func()
		expectedInternalID int
		expectedRS         ResourceServer
		expectedError      error
		shouldErr          bool
		checkError         func(error) bool
	}{
		{
			name:       "Success",
			resourceID: "rs1",
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryGetResourceServerByID, "rs1", "test-deployment").
					Return([]map[string]interface{}{
						{
							"internal_id": 7,
							"id":          "rs1",
							"ou_id":       "ou1",
							"name":        "Test Server",
							"description": "Test Description",
							"identifier":  "test-identifier",
							"properties":  []byte(`{"delimiter":"/"}`),
						},
					}, nil)
			},
			expectedInternalID: 7,
			expectedRS: ResourceServer{
				ID:          "rs1",
				OUID:        "ou1",
				Name:        "Test Server",
				Description: "Test Description",
				Identifier:  "test-identifier",
				Delimiter:   "/",
			},
			shouldErr: false,
		},
		{
			name:       "NotFound",
			resourceID: "nonexistent",
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryGetResourceServerByID, "nonexistent",
					"test-deployment").Return([]map[string]interface{}{}, nil)
			},
			expectedInternalID: 0,
			expectedError:      errResourceServerNotFound,
			shouldErr:          true,
		},
		{
			name:       "QueryError",
			resourceID: "rs1",
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryGetResourceServerByID, "rs1", "test-deployment").
					Return(nil, errors.New("query error"))
			},
			expectedInternalID: 0,
			shouldErr:          true,
			checkError: func(err error) bool {
				suite.Contains(err.Error(), "failed to get resource server")
				return true
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create fresh mocks for each test case
			suite.mockDBProvider = providermock.NewDBProviderInterfaceMock(suite.T())
			suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())
			suite.store = &resourceStore{
				dbProvider:   suite.mockDBProvider,
				deploymentID: "test-deployment",
			}

			tc.setupMocks()

			rs, err := suite.store.GetResourceServer(context.Background(),
				tc.resourceID)

			if tc.shouldErr {
				suite.Error(err)
				if tc.expectedError != nil {
					suite.Equal(tc.expectedError, err)
				}
				if tc.checkError != nil {
					tc.checkError(err)
				}
				suite.Empty(rs.ID)
			} else {
				suite.NoError(err)
				suite.Equal(tc.expectedRS.ID, rs.ID)
				suite.Equal(tc.expectedRS.OUID, rs.OUID)
				suite.Equal(tc.expectedRS.Name, rs.Name)
				suite.Equal(tc.expectedRS.Description, rs.Description)
				suite.Equal(tc.expectedRS.Identifier, rs.Identifier)
				suite.Equal(tc.expectedRS.Delimiter, rs.Delimiter)
			}
		})
	}
}

func (suite *ResourceStoreTestSuite) TestGetResourceServerList() {
	testCases := []struct {
		name            string
		limit           int
		offset          int
		setupMocks      func()
		expectedServers []ResourceServer
		shouldErr       bool
		checkError      func(error) bool
	}{
		{
			name:   "Success",
			limit:  10,
			offset: 0,
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryGetResourceServerList, 10, 0, "test-deployment").
					Return([]map[string]interface{}{
						{
							"internal_id": 1,
							"id":          "rs1",
							"ou_id":       "ou1",
							"name":        "Server 1",
							"description": "Description 1",
							"identifier":  "identifier-1",
						},
						{
							"internal_id": 2,
							"id":          "rs2",
							"ou_id":       "ou1",
							"name":        "Server 2",
							"description": "Description 2",
							"identifier":  "identifier-2",
						},
					}, nil)
			},
			expectedServers: []ResourceServer{
				{ID: "rs1", Name: "Server 1"},
				{ID: "rs2", Name: "Server 2"},
			},
			shouldErr: false,
		},
		{
			name:   "QueryError",
			limit:  10,
			offset: 0,
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryGetResourceServerList, 10, 0, "test-deployment").
					Return(nil, errors.New("query error"))
			},
			shouldErr: true,
		},
		{
			name:   "InvalidRowData",
			limit:  10,
			offset: 0,
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryGetResourceServerList, 10, 0, "test-deployment").
					Return([]map[string]interface{}{
						{
							"internal_id": 3,
							"id":          123, // Invalid type
							"ou_id":       "ou1",
							"name":        "Server 1",
						},
					}, nil)
			},
			shouldErr: true,
			checkError: func(err error) bool {
				suite.Contains(err.Error(), "failed to build resource server")
				return true
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create fresh mocks for each test case
			suite.mockDBProvider = providermock.NewDBProviderInterfaceMock(suite.T())
			suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())
			suite.store = &resourceStore{
				dbProvider:   suite.mockDBProvider,
				deploymentID: "test-deployment",
			}

			tc.setupMocks()

			servers, err := suite.store.GetResourceServerList(context.Background(),
				tc.limit, tc.offset)

			if tc.shouldErr {
				suite.Error(err)
				suite.Nil(servers)
				if tc.checkError != nil {
					tc.checkError(err)
				}
			} else {
				suite.NoError(err)
				suite.Len(servers, len(tc.expectedServers))
				for i, expected := range tc.expectedServers {
					suite.Equal(expected.ID, servers[i].ID)
					suite.Equal(expected.Name, servers[i].Name)
				}
			}
		})
	}
}

func (suite *ResourceStoreTestSuite) TestGetResourceServerListCount() {
	testCases := []struct {
		name          string
		setupMocks    func()
		expectedCount int
		shouldErr     bool
	}{
		{
			name: "Success",
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryGetResourceServerListCount, "test-deployment").
					Return([]map[string]interface{}{
						{"total": int64(5)},
					}, nil)
			},
			expectedCount: 5,
			shouldErr:     false,
		},
		{
			name: "QueryError",
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryGetResourceServerListCount, "test-deployment").
					Return(nil, errors.New("query error"))
			},
			expectedCount: 0,
			shouldErr:     true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create fresh mocks for each test case
			suite.mockDBProvider = providermock.NewDBProviderInterfaceMock(suite.T())
			suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())
			suite.store = &resourceStore{
				dbProvider:   suite.mockDBProvider,
				deploymentID: "test-deployment",
			}

			tc.setupMocks()

			count, err := suite.store.GetResourceServerListCount(context.Background())

			if tc.shouldErr {
				suite.Error(err)
			} else {
				suite.NoError(err)
			}
			suite.Equal(tc.expectedCount, count)
		})
	}
}

func (suite *ResourceStoreTestSuite) TestUpdateResourceServer() {
	testCases := []struct {
		name           string
		resourceID     string
		resourceServer ResourceServer
		setupMocks     func()
		shouldErr      bool
		checkError     func(error) bool
	}{
		{
			name:       "Success",
			resourceID: "rs1",
			resourceServer: ResourceServer{
				OUID:        "ou1",
				Name:        "Updated Server",
				Description: "Updated Description",
				Identifier:  "updated-identifier",
				Delimiter:   "-",
			},
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("ExecuteContext", context.Background(),
					queryUpdateResourceServer, "ou1", "Updated Server",
					"Updated Description", nil, "updated-identifier",
					[]byte(`{"delimiter":"-"}`), "rs1", "test-deployment").
					Return(int64(1), nil)
			},
			shouldErr: false,
		},
		{
			name:       "ExecuteError",
			resourceID: "rs1",
			resourceServer: ResourceServer{
				OUID:        "ou1",
				Name:        "Updated Server",
				Description: "Updated Description",
				Identifier:  "updated-identifier",
				Delimiter:   "-",
			},
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("ExecuteContext", context.Background(),
					queryUpdateResourceServer, "ou1", "Updated Server",
					"Updated Description", nil, "updated-identifier",
					[]byte(`{"delimiter":"-"}`), "rs1", "test-deployment").
					Return(int64(0), errors.New("update failed"))
			},
			shouldErr: true,
			checkError: func(err error) bool {
				suite.Contains(err.Error(), "failed to update resource server")
				return true
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create fresh mocks for each test case
			suite.mockDBProvider = providermock.NewDBProviderInterfaceMock(suite.T())
			suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())
			suite.store = &resourceStore{
				dbProvider:   suite.mockDBProvider,
				deploymentID: "test-deployment",
			}

			tc.setupMocks()

			err := suite.store.UpdateResourceServer(context.Background(),
				tc.resourceID, tc.resourceServer)

			if tc.shouldErr {
				suite.Error(err)
				if tc.checkError != nil {
					tc.checkError(err)
				}
			} else {
				suite.NoError(err)
			}
		})
	}
}

func (suite *ResourceStoreTestSuite) TestDeleteResourceServer() {
	testCases := []struct {
		name       string
		resourceID string
		setupMocks func()
		shouldErr  bool
		checkError func(error) bool
	}{
		{
			name:       "Success",
			resourceID: "rs1",
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("ExecuteContext", context.Background(),
					queryDeleteResourceServer, "rs1", "test-deployment").
					Return(int64(1), nil)
			},
			shouldErr: false,
		},
		{
			name:       "ExecuteError",
			resourceID: "rs1",
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("ExecuteContext", context.Background(),
					queryDeleteResourceServer, "rs1", "test-deployment").
					Return(int64(0), errors.New("delete failed"))
			},
			shouldErr: true,
			checkError: func(err error) bool {
				suite.Contains(err.Error(), "failed to delete resource server")
				return true
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create fresh mocks for each test case
			suite.mockDBProvider = providermock.NewDBProviderInterfaceMock(suite.T())
			suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())
			suite.store = &resourceStore{
				dbProvider:   suite.mockDBProvider,
				deploymentID: "test-deployment",
			}

			tc.setupMocks()

			err := suite.store.DeleteResourceServer(context.Background(),
				tc.resourceID)

			if tc.shouldErr {
				suite.Error(err)
				if tc.checkError != nil {
					tc.checkError(err)
				}
			} else {
				suite.NoError(err)
			}
		})
	}
}

func (suite *ResourceStoreTestSuite) TestCheckResourceServerNameExists() {
	testCases := []struct {
		name           string
		serverName     string
		setupMocks     func()
		expectedExists bool
		shouldErr      bool
		checkError     func(error) bool
	}{
		{
			name:       "Exists",
			serverName: "Test Server",
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryCheckResourceServerNameExists, "Test Server",
					"test-deployment").Return([]map[string]interface{}{
					{"count": int64(1)},
				}, nil)
			},
			expectedExists: true,
			shouldErr:      false,
		},
		{
			name:       "NotExists",
			serverName: "Nonexistent Server",
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryCheckResourceServerNameExists, "Nonexistent Server",
					"test-deployment").Return([]map[string]interface{}{
					{"count": int64(0)},
				}, nil)
			},
			expectedExists: false,
			shouldErr:      false,
		},
		{
			name:       "QueryError",
			serverName: "Test Server",
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryCheckResourceServerNameExists, "Test Server",
					"test-deployment").Return(nil, errors.New("query error"))
			},
			expectedExists: false,
			shouldErr:      true,
			checkError: func(err error) bool {
				suite.Contains(err.Error(), "failed to check resource server name")
				return true
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create fresh mocks for each test case
			suite.mockDBProvider = providermock.NewDBProviderInterfaceMock(suite.T())
			suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())
			suite.store = &resourceStore{
				dbProvider:   suite.mockDBProvider,
				deploymentID: "test-deployment",
			}

			tc.setupMocks()

			exists, err := suite.store.CheckResourceServerNameExists(context.Background(),
				tc.serverName)

			if tc.shouldErr {
				suite.Error(err)
				if tc.checkError != nil {
					tc.checkError(err)
				}
			} else {
				suite.NoError(err)
			}
			suite.Equal(tc.expectedExists, exists)
		})
	}
}

func (suite *ResourceStoreTestSuite) runBoolCheckTest(
	testName string,
	setupMocks func(),
	checkFunc func() (bool, error),
	expectedVal bool,
	shouldErr bool,
) {
	suite.Run(testName, func() {
		// Create fresh mocks for each test case
		suite.mockDBProvider = providermock.NewDBProviderInterfaceMock(suite.T())
		suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())
		suite.store = &resourceStore{
			dbProvider:   suite.mockDBProvider,
			deploymentID: "test-deployment",
		}

		setupMocks()

		result, err := checkFunc()

		if shouldErr {
			suite.Error(err)
		} else {
			suite.NoError(err)
		}
		suite.Equal(expectedVal, result)
	})
}

// nolint:dupl
func (suite *ResourceStoreTestSuite) TestCheckResourceServerIdentifierExists() {
	testCases := []struct {
		name           string
		identifier     string
		setupMocks     func()
		expectedExists bool
		shouldErr      bool
	}{
		{
			name:       "Exists",
			identifier: "test-identifier",
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryCheckResourceServerIdentifierExists,
					"test-identifier", "test-deployment").Return([]map[string]interface{}{
					{"count": int64(1)},
				}, nil)
			},
			expectedExists: true,
			shouldErr:      false,
		},
		{
			name:       "NotExists",
			identifier: "nonexistent-identifier",
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryCheckResourceServerIdentifierExists,
					"nonexistent-identifier", "test-deployment").Return([]map[string]interface{}{
					{"count": int64(0)},
				}, nil)
			},
			expectedExists: false,
			shouldErr:      false,
		},
		{
			name:       "QueryError",
			identifier: "test-identifier",
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryCheckResourceServerIdentifierExists,
					"test-identifier", "test-deployment").Return(nil, errors.New("query error"))
			},
			expectedExists: false,
			shouldErr:      true,
		},
	}

	for _, tc := range testCases {
		suite.runBoolCheckTest(tc.name, tc.setupMocks,
			func() (bool, error) {
				return suite.store.CheckResourceServerIdentifierExists(context.Background(),
					tc.identifier)
			},
			tc.expectedExists, tc.shouldErr)
	}
}

// nolint:dupl
func (suite *ResourceStoreTestSuite) TestCheckResourceServerHasDependencies() {
	testCases := []struct {
		name            string
		id              string
		setupMocks      func()
		expectedHasDeps bool
		shouldErr       bool
	}{
		{
			name: "HasDependencies",
			id:   "rs1",
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryCheckResourceServerHasDependencies, "rs1",
					"test-deployment").Return([]map[string]interface{}{
					{"count": int64(3)},
				}, nil)
			},
			expectedHasDeps: true,
			shouldErr:       false,
		},
		{
			name: "NoDependencies",
			id:   "rs1",
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryCheckResourceServerHasDependencies, "rs1",
					"test-deployment").Return([]map[string]interface{}{
					{"count": int64(0)},
				}, nil)
			},
			expectedHasDeps: false,
			shouldErr:       false,
		},
		{
			name: "QueryError",
			id:   "rs1",
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryCheckResourceServerHasDependencies, "rs1",
					"test-deployment").Return(nil, errors.New("query error"))
			},
			expectedHasDeps: false,
			shouldErr:       true,
		},
	}

	for _, tc := range testCases {
		suite.runBoolCheckTest(tc.name, tc.setupMocks,
			func() (bool, error) {
				return suite.store.CheckResourceServerHasDependencies(context.Background(),
					tc.id)
			},
			tc.expectedHasDeps, tc.shouldErr)
	}
}

// Resource Tests

func (suite *ResourceStoreTestSuite) TestCreateResource() {
	testCases := []struct {
		name             string
		resourceID       string
		resourceServerID string
		parentID         *string
		resource         Resource
		setupMocks       func(*string)
		shouldErr        bool
		checkError       func(error) bool
	}{
		{
			name:             "Success_WithParent",
			resourceID:       "res1",
			resourceServerID: "rs1",
			parentID:         &testParentID1,
			resource: Resource{
				Name:        "Test Resource",
				Handle:      "test-handle",
				Description: "Test Description",
				Permission:  "perm:create",
			},
			setupMocks: func(parentID *string) {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("ExecuteContext", context.Background(),
					queryCreateResource, "res1", "rs1", "Test Resource",
					"test-handle", "Test Description", "perm:create", "{}", parentID, "test-deployment").
					Return(int64(1), nil)
			},
			shouldErr: false,
		},
		{
			name:             "Success_NullParent",
			resourceID:       "res1",
			resourceServerID: "rs1",
			parentID:         nil,
			resource: Resource{
				Name:        "Test Resource",
				Handle:      "test-handle",
				Description: "Test Description",
				Permission:  "perm:create",
			},
			setupMocks: func(parentID *string) {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("ExecuteContext", context.Background(),
					queryCreateResource, "res1", "rs1", "Test Resource",
					"test-handle", "Test Description", "perm:create", "{}", (*string)(nil), "test-deployment").
					Return(int64(1), nil)
			},
			shouldErr: false,
		},
		{
			name:             "ExecuteError",
			resourceID:       "res1",
			resourceServerID: "rs1",
			parentID:         nil,
			resource: Resource{
				Name:        "Test Resource",
				Handle:      "test-handle",
				Description: "Test Description",
				Permission:  "perm:create",
			},
			setupMocks: func(parentID *string) {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("ExecuteContext", context.Background(),
					queryCreateResource, "res1", "rs1", "Test Resource",
					"test-handle", "Test Description", "perm:create", "{}", (*string)(nil), "test-deployment").
					Return(int64(0), errors.New("insert failed"))
			},
			shouldErr: true,
			checkError: func(err error) bool {
				suite.Contains(err.Error(), "failed to create resource")
				return true
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create fresh mocks for each test case
			suite.mockDBProvider = providermock.NewDBProviderInterfaceMock(suite.T())
			suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())
			suite.store = &resourceStore{
				dbProvider:   suite.mockDBProvider,
				deploymentID: "test-deployment",
			}

			tc.setupMocks(tc.parentID)

			err := suite.store.CreateResource(context.Background(),
				tc.resourceID, tc.resourceServerID, tc.parentID, tc.resource)

			if tc.shouldErr {
				suite.Error(err)
				if tc.checkError != nil {
					tc.checkError(err)
				}
			} else {
				suite.NoError(err)
			}
		})
	}
}

func (suite *ResourceStoreTestSuite) TestGetResource() {
	testCases := []struct {
		name               string
		resourceID         string
		resourceServerID   string
		setupMocks         func()
		expectedInternalID int
		expectedResource   Resource
		expectedError      error
		shouldErr          bool
	}{
		{
			name:             "Success_WithParent",
			resourceID:       "res1",
			resourceServerID: "rs1",
			setupMocks: func() {
				parentID := testParentID1
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryGetResourceByID, "res1", "rs1",
					"test-deployment").Return([]map[string]interface{}{
					{
						"internal_id":        11,
						"id":                 "res1",
						"resource_server_id": "rs1",
						"name":               "Test Resource",
						"handle":             "test-handle",
						"description":        "Test Description",
						"parent_resource_id": parentID,
						"permission":         "perm:read",
					},
				}, nil)
			},
			expectedInternalID: 11,
			expectedResource: Resource{
				ID:          "res1",
				Name:        "Test Resource",
				Handle:      "test-handle",
				Description: "Test Description",
				Permission:  "perm:read",
				Parent:      &testParentID1,
			},
			shouldErr: false,
		},
		{
			name:             "NotFound",
			resourceID:       "nonexistent",
			resourceServerID: "rs1",
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryGetResourceByID, "nonexistent", "rs1",
					"test-deployment").Return([]map[string]interface{}{}, nil)
			},
			expectedInternalID: 0,
			expectedError:      errResourceNotFound,
			shouldErr:          true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create fresh mocks for each test case
			suite.mockDBProvider = providermock.NewDBProviderInterfaceMock(suite.T())
			suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())
			suite.store = &resourceStore{
				dbProvider:   suite.mockDBProvider,
				deploymentID: "test-deployment",
			}

			tc.setupMocks()

			res, err := suite.store.GetResource(context.Background(),
				tc.resourceID, tc.resourceServerID)

			if tc.shouldErr {
				suite.Error(err)
				if tc.expectedError != nil {
					suite.Equal(tc.expectedError, err)
				}
				suite.Empty(res.ID)
			} else {
				suite.NoError(err)
				suite.Equal(tc.expectedResource.ID, res.ID)
				suite.Equal(tc.expectedResource.Name, res.Name)
				suite.Equal(tc.expectedResource.Handle, res.Handle)
				suite.Equal(tc.expectedResource.Description, res.Description)
				suite.Equal(tc.expectedResource.Permission, res.Permission)
				if tc.expectedResource.Parent != nil {
					suite.NotNil(res.Parent)
					suite.Equal(*tc.expectedResource.Parent, *res.Parent)
				} else {
					suite.Nil(res.Parent)
				}
			}
		})
	}
}

func (suite *ResourceStoreTestSuite) TestGetResourceList() {
	testCases := []struct {
		name              string
		resourceServerID  string
		limit             int
		offset            int
		setupMocks        func()
		expectedResources []Resource
		shouldErr         bool
		checkError        func(error) bool
	}{
		{
			name:             "Success",
			resourceServerID: "rs1",
			limit:            10,
			offset:           0,
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryGetResourceList, "rs1", 10, 0, "test-deployment").
					Return([]map[string]interface{}{
						{
							"internal_id":        21,
							"id":                 "res1",
							"resource_server_id": "rs1",
							"name":               "Resource 1",
							"handle":             "resource-1",
							"description":        "Description 1",
							"permission":         "perm:r1",
						},
						{
							"internal_id":        22,
							"id":                 "res2",
							"resource_server_id": "rs1",
							"name":               "Resource 2",
							"handle":             "resource-2",
							"description":        "Description 2",
							"permission":         "perm:r2",
						},
					}, nil)
			},
			expectedResources: []Resource{
				{ID: "res1", Name: "Resource 1"},
				{ID: "res2", Name: "Resource 2"},
			},
			shouldErr: false,
		},
		{
			name:             "QueryError",
			resourceServerID: "rs1",
			limit:            10,
			offset:           0,
			setupMocks: func() {
				queryError := errors.New("query error")
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryGetResourceList, "rs1", 10, 0, "test-deployment").
					Return(nil, queryError)
			},
			shouldErr: true,
			checkError: func(err error) bool {
				suite.Contains(err.Error(), "failed to get resource list")
				return true
			},
		},
		{
			name:             "InvalidRowData",
			resourceServerID: "rs1",
			limit:            10,
			offset:           0,
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryGetResourceList, "rs1", 10, 0, "test-deployment").
					Return([]map[string]interface{}{
						{
							"internal_id":        23,
							"id":                 123, // Invalid type
							"resource_server_id": "rs1",
							"name":               "Resource 1",
						},
					}, nil)
			},
			shouldErr: true,
			checkError: func(err error) bool {
				suite.Contains(err.Error(), "failed to build resource")
				return true
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create fresh mocks for each test case
			suite.mockDBProvider = providermock.NewDBProviderInterfaceMock(suite.T())
			suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())
			suite.store = &resourceStore{
				dbProvider:   suite.mockDBProvider,
				deploymentID: "test-deployment",
			}

			tc.setupMocks()
			resources, err := suite.store.GetResourceList(context.Background(),
				tc.resourceServerID, tc.limit, tc.offset)

			if tc.shouldErr {
				suite.Error(err)
				suite.Nil(resources)
				if tc.checkError != nil {
					tc.checkError(err)
				}
			} else {
				suite.NoError(err)
				suite.Len(resources, len(tc.expectedResources))
				if len(tc.expectedResources) > 0 {
					suite.Equal(tc.expectedResources[0].ID, resources[0].ID)
					suite.Equal(tc.expectedResources[0].Name, resources[0].Name)
				}
			}
		})
	}
}

func (suite *ResourceStoreTestSuite) TestGetResourceListByParent() {
	testCases := []struct {
		name             string
		resourceServerID string
		parentID         *string
		limit            int
		offset           int
		setupMocks       func(*string)
		expectedCount    int
		shouldErr        bool
		checkError       func(error) bool
	}{
		{
			name:             "Success_NullParent",
			resourceServerID: "rs1",
			parentID:         nil,
			limit:            10,
			offset:           0,
			setupMocks: func(parentID *string) {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryGetResourceListByNullParent, "rs1", 10, 0,
					"test-deployment").Return([]map[string]interface{}{
					{
						"internal_id":        31,
						"id":                 "res1",
						"resource_server_id": "rs1",
						"name":               "Resource 1",
						"handle":             "resource-1",
						"description":        "Description 1",
					},
				}, nil)
			},
			expectedCount: 1,
			shouldErr:     false,
		},
		{
			name:             "Success_WithParent",
			resourceServerID: "rs1",
			parentID:         &testParentID1,
			limit:            10,
			offset:           0,
			setupMocks: func(parentID *string) {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryGetResourceListByParent, "rs1", *parentID, 10, 0,
					"test-deployment").Return([]map[string]interface{}{
					{
						"internal_id":        32,
						"id":                 "res1",
						"resource_server_id": "rs1",
						"name":               "Resource 1",
						"handle":             "resource-1",
						"description":        "Description 1",
						"parent_resource_id": testParentID1,
					},
				}, nil)
			},
			expectedCount: 1,
			shouldErr:     false,
		},
		{
			name:             "QueryError_NullParent",
			resourceServerID: "rs1",
			parentID:         nil,
			limit:            10,
			offset:           0,
			setupMocks: func(parentID *string) {
				queryError := errors.New("query error")
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryGetResourceListByNullParent, "rs1", 10, 0,
					"test-deployment").Return(nil, queryError)
			},
			shouldErr: true,
			checkError: func(err error) bool {
				suite.Contains(err.Error(), "failed to get resource list by parent")
				return true
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create fresh mocks for each test case
			suite.mockDBProvider = providermock.NewDBProviderInterfaceMock(suite.T())
			suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())
			suite.store = &resourceStore{
				dbProvider:   suite.mockDBProvider,
				deploymentID: "test-deployment",
			}

			tc.setupMocks(tc.parentID)

			resources, err := suite.store.GetResourceListByParent(context.Background(),

				tc.resourceServerID, tc.parentID, tc.limit, tc.offset)

			if tc.shouldErr {
				suite.Error(err)
				suite.Nil(resources)
				if tc.checkError != nil {
					tc.checkError(err)
				}
			} else {
				suite.NoError(err)
				suite.Len(resources, tc.expectedCount)
				if tc.expectedCount > 0 {
					suite.Equal("res1", resources[0].ID)
				}
			}
		})
	}
}

func (suite *ResourceStoreTestSuite) TestGetResourceListCount() {
	testCases := []struct {
		name             string
		resourceServerID string
		setupMocks       func()
		expectedCount    int
		shouldErr        bool
	}{
		{
			name:             "Success",
			resourceServerID: "rs1",
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryGetResourceListCount, "rs1", "test-deployment").
					Return([]map[string]interface{}{
						{"total": int64(10)},
					}, nil)
			},
			expectedCount: 10,
			shouldErr:     false,
		},
		{
			name:             "QueryError",
			resourceServerID: "rs1",
			setupMocks: func() {
				queryError := errors.New("query error")
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryGetResourceListCount, "rs1", "test-deployment").Return(nil, queryError)
			},
			expectedCount: 0,
			shouldErr:     true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create fresh mocks for each test case
			suite.mockDBProvider = providermock.NewDBProviderInterfaceMock(suite.T())
			suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())
			suite.store = &resourceStore{
				dbProvider:   suite.mockDBProvider,
				deploymentID: "test-deployment",
			}

			tc.setupMocks()
			count, err := suite.store.GetResourceListCount(context.Background(),
				tc.resourceServerID)

			if tc.shouldErr {
				suite.Error(err)
			} else {
				suite.NoError(err)
			}
			suite.Equal(tc.expectedCount, count)
		})
	}
}

func (suite *ResourceStoreTestSuite) TestGetResourceListCountByParent() {
	testCases := []struct {
		name             string
		resourceServerID string
		parentID         *string
		setupMocks       func(*string)
		expectedCount    int
		shouldErr        bool
	}{
		{
			name:             "Success_NullParent",
			resourceServerID: "rs1",
			parentID:         nil,
			setupMocks: func(parentID *string) {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryGetResourceListCountByNullParent, "rs1",
					"test-deployment").Return([]map[string]interface{}{
					{"total": int64(5)},
				}, nil)
			},
			expectedCount: 5,
			shouldErr:     false,
		},
		{
			name:             "Success_WithParent",
			resourceServerID: "rs1",
			parentID:         func() *string { id := "parent2"; return &id }(),
			setupMocks: func(parentID *string) {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryGetResourceListCountByParent, "rs1", *parentID,
					"test-deployment").Return([]map[string]interface{}{
					{"total": int64(3)},
				}, nil)
			},
			expectedCount: 3,
			shouldErr:     false,
		},
		{
			name:             "QueryError_NullParent",
			resourceServerID: "rs1",
			parentID:         nil,
			setupMocks: func(parentID *string) {
				queryError := errors.New("query error")
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryGetResourceListCountByNullParent, "rs1", "test-deployment").
					Return(nil, queryError)
			},
			expectedCount: 0,
			shouldErr:     true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create fresh mocks for each test case
			suite.mockDBProvider = providermock.NewDBProviderInterfaceMock(suite.T())
			suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())
			suite.store = &resourceStore{
				dbProvider:   suite.mockDBProvider,
				deploymentID: "test-deployment",
			}

			tc.setupMocks(tc.parentID)
			count, err := suite.store.GetResourceListCountByParent(context.Background(),
				tc.resourceServerID, tc.parentID)

			if tc.shouldErr {
				suite.Error(err)
			} else {
				suite.NoError(err)
			}
			suite.Equal(tc.expectedCount, count)
		})
	}
}

func (suite *ResourceStoreTestSuite) TestUpdateResource() {
	testCases := []struct {
		name             string
		resourceID       string
		resourceServerID string
		resource         Resource
		setupMocks       func()
		shouldErr        bool
		checkError       func(error) bool
	}{
		{
			name:             "Success",
			resourceID:       "res1",
			resourceServerID: "rs1",
			resource: Resource{
				Name:        "Updated Resource",
				Description: "Updated Description",
			},
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("ExecuteContext", context.Background(),
					queryUpdateResource, "Updated Resource", "Updated Description", "{}",
					"res1", "rs1", "test-deployment").Return(int64(1), nil)
			},
			shouldErr: false,
		},
		{
			name:             "ParentNotFound",
			resourceID:       "nonexistent",
			resourceServerID: "rs1",
			resource: Resource{
				Name:        "Updated Name",
				Description: "Updated Description",
			},
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("ExecuteContext", context.Background(),
					queryUpdateResource, "Updated Name", "Updated Description", "{}",
					"nonexistent", "rs1", "test-deployment").Return(int64(0), errResourceNotFound)
			},
			shouldErr: true,
		},
		{
			name:             "ExecuteError",
			resourceID:       "res1",
			resourceServerID: "rs1",
			resource: Resource{
				Name:        "Updated Name",
				Description: "Updated Description",
			},
			setupMocks: func() {
				execError := errors.New("update failed")
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("ExecuteContext", context.Background(),
					queryUpdateResource, "Updated Name", "Updated Description", "{}",
					"res1", "rs1", "test-deployment").Return(int64(0), execError)
			},
			shouldErr: true,
			checkError: func(err error) bool {
				suite.Contains(err.Error(), "failed to update resource")
				return true
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create fresh mocks for each test case
			suite.mockDBProvider = providermock.NewDBProviderInterfaceMock(suite.T())
			suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())
			suite.store = &resourceStore{
				dbProvider:   suite.mockDBProvider,
				deploymentID: "test-deployment",
			}

			tc.setupMocks()
			err := suite.store.UpdateResource(context.Background(),
				tc.resourceID, tc.resourceServerID, tc.resource)

			if tc.shouldErr {
				suite.Error(err)
				if tc.checkError != nil {
					tc.checkError(err)
				}
			} else {
				suite.NoError(err)
			}
		})
	}
}

func (suite *ResourceStoreTestSuite) TestDeleteResource() {
	testCases := []struct {
		name             string
		resourceID       string
		resourceServerID string
		setupMocks       func()
		shouldErr        bool
		checkError       func(error) bool
	}{
		{
			name:             "Success",
			resourceID:       "res1",
			resourceServerID: "rs1",
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("ExecuteContext", context.Background(),
					queryDeleteResource, "res1", "rs1", "test-deployment").
					Return(int64(1), nil)
			},
			shouldErr: false,
		},
		{
			name:             "ExecuteError",
			resourceID:       "res1",
			resourceServerID: "rs1",
			setupMocks: func() {
				execError := errors.New("delete error")
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("ExecuteContext", context.Background(),
					queryDeleteResource, "res1", "rs1", "test-deployment").
					Return(int64(0), execError)
			},
			shouldErr: true,
			checkError: func(err error) bool {
				suite.Contains(err.Error(), "failed to delete resource")
				return true
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create fresh mocks for each test case
			suite.mockDBProvider = providermock.NewDBProviderInterfaceMock(suite.T())
			suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())
			suite.store = &resourceStore{
				dbProvider:   suite.mockDBProvider,
				deploymentID: "test-deployment",
			}

			tc.setupMocks()
			err := suite.store.DeleteResource(context.Background(),
				tc.resourceID, tc.resourceServerID)

			if tc.shouldErr {
				suite.Error(err)
				if tc.checkError != nil {
					tc.checkError(err)
				}
			} else {
				suite.NoError(err)
			}
		})
	}
}

func (suite *ResourceStoreTestSuite) TestCheckResourceHandleExists() {
	testCases := []struct {
		name             string
		resourceServerID string
		resourceHandle   string
		parentID         *string
		setupMocks       func(*string)
		expectedExists   bool
		shouldErr        bool
	}{
		{
			name:             "Exists_NullParent",
			resourceServerID: "rs1",
			resourceHandle:   "Test Resource",
			parentID:         nil,
			setupMocks: func(parentID *string) {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryCheckResourceHandleExistsUnderNullParent, "rs1",
					"Test Resource", "test-deployment").Return([]map[string]interface{}{
					{"count": int64(1)},
				}, nil)
			},
			expectedExists: true,
			shouldErr:      false,
		},
		{
			name:             "NotExists_NullParent",
			resourceServerID: "rs1",
			resourceHandle:   "Nonexistent",
			parentID:         nil,
			setupMocks: func(parentID *string) {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryCheckResourceHandleExistsUnderNullParent, "rs1",
					"Nonexistent", "test-deployment").Return([]map[string]interface{}{
					{"count": int64(0)},
				}, nil)
			},
			expectedExists: false,
			shouldErr:      false,
		},
		{
			name:             "NotExists_WithParent",
			resourceServerID: "rs1",
			resourceHandle:   "Test Resource",
			parentID:         &testParentID1,
			setupMocks: func(parentID *string) {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryCheckResourceHandleExistsUnderParent, "rs1",
					"Test Resource", *parentID, "test-deployment").Return([]map[string]interface{}{
					{"count": int64(0)},
				}, nil)
			},
			expectedExists: false,
			shouldErr:      false,
		},
		{
			name:             "QueryError",
			resourceServerID: "rs1",
			resourceHandle:   "Test Resource",
			parentID:         nil,
			setupMocks: func(parentID *string) {
				queryError := errors.New("query error")
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryCheckResourceHandleExistsUnderNullParent, "rs1",
					"Test Resource", "test-deployment").Return(nil, queryError)
			},
			expectedExists: false,
			shouldErr:      true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create fresh mocks for each test case
			suite.mockDBProvider = providermock.NewDBProviderInterfaceMock(suite.T())
			suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())
			suite.store = &resourceStore{
				dbProvider:   suite.mockDBProvider,
				deploymentID: "test-deployment",
			}

			tc.setupMocks(tc.parentID)

			exists, err := suite.store.CheckResourceHandleExists(context.Background(),

				tc.resourceServerID, tc.resourceHandle, tc.parentID)

			if tc.shouldErr {
				suite.Error(err)
			} else {
				suite.NoError(err)
			}
			suite.Equal(tc.expectedExists, exists)
		})
	}
}

func (suite *ResourceStoreTestSuite) TestCheckResourceHasDependencies() {
	testCases := []struct {
		name            string
		resourceID      string
		setupMocks      func()
		expectedHasDeps bool
		shouldErr       bool
	}{
		{
			name:       "HasDependencies",
			resourceID: "res1",
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryCheckResourceHasDependencies, "res1",
					"test-deployment").Return([]map[string]interface{}{
					{"count": int64(2)},
				}, nil)
			},
			expectedHasDeps: true,
			shouldErr:       false,
		},
		{
			name:       "NoDependencies",
			resourceID: "res1",
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryCheckResourceHasDependencies, "res1",
					"test-deployment").Return([]map[string]interface{}{
					{"count": int64(0)},
				}, nil)
			},
			expectedHasDeps: false,
			shouldErr:       false,
		},
		{
			name:       "QueryError",
			resourceID: "res1",
			setupMocks: func() {
				queryError := errors.New("query error")
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryCheckResourceHasDependencies, "res1", "test-deployment").
					Return(nil, queryError)
			},
			expectedHasDeps: false,
			shouldErr:       true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create fresh mocks for each test case
			suite.mockDBProvider = providermock.NewDBProviderInterfaceMock(suite.T())
			suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())
			suite.store = &resourceStore{
				dbProvider:   suite.mockDBProvider,
				deploymentID: "test-deployment",
			}

			tc.setupMocks()
			hasDeps, err := suite.store.CheckResourceHasDependencies(context.Background(),
				tc.resourceID)

			if tc.shouldErr {
				suite.Error(err)
			} else {
				suite.NoError(err)
			}
			suite.Equal(tc.expectedHasDeps, hasDeps)
		})
	}
}

func (suite *ResourceStoreTestSuite) TestCheckCircularDependency() {
	testCases := []struct {
		name                string
		resourceID          string
		parentResourceID    string
		setupMocks          func()
		expectedHasCircular bool
		shouldErr           bool
	}{
		{
			name:             "HasCircular",
			resourceID:       "res1",
			parentResourceID: "parent1",
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryCheckCircularDependency, "parent1", "res1",
					"test-deployment").Return([]map[string]interface{}{
					{"count": int64(1)},
				}, nil)
			},
			expectedHasCircular: true,
			shouldErr:           false,
		},
		{
			name:             "NoCircular",
			resourceID:       "res1",
			parentResourceID: "parent1",
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryCheckCircularDependency, "parent1", "res1",
					"test-deployment").Return([]map[string]interface{}{
					{"count": int64(0)},
				}, nil)
			},
			expectedHasCircular: false,
			shouldErr:           false,
		},
		{
			name:             "QueryError",
			resourceID:       "res1",
			parentResourceID: "parent1",
			setupMocks: func() {
				queryError := errors.New("query error")
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryCheckCircularDependency, "parent1", "res1",
					"test-deployment").Return(nil, queryError)
			},
			expectedHasCircular: false,
			shouldErr:           true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create fresh mocks for each test case
			suite.mockDBProvider = providermock.NewDBProviderInterfaceMock(suite.T())
			suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())
			suite.store = &resourceStore{
				dbProvider:   suite.mockDBProvider,
				deploymentID: "test-deployment",
			}

			tc.setupMocks()
			hasCircular, err := suite.store.CheckCircularDependency(context.Background(),
				tc.resourceID, tc.parentResourceID)

			if tc.shouldErr {
				suite.Error(err)
			} else {
				suite.NoError(err)
			}
			suite.Equal(tc.expectedHasCircular, hasCircular)
		})
	}
}

// Action Tests

func (suite *ResourceStoreTestSuite) TestCreateAction() {
	testCases := []struct {
		name             string
		actionID         string
		resourceServerID string
		resourceID       *string
		action           Action
		setupMocks       func(*string)
		shouldErr        bool
		checkError       func(error) bool
	}{
		{
			name:             "Success_WithResource",
			actionID:         "action1",
			resourceServerID: "rs1",
			resourceID:       &testResourceID1,
			action: Action{
				Name:        "Test Action",
				Handle:      "test-handle",
				Description: "Test Description",
				Permission:  "perm:act",
			},
			setupMocks: func(resourceID *string) {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("ExecuteContext", context.Background(),
					queryCreateAction, "action1", "rs1", resourceID,
					"Test Action", "test-handle", "Test Description", "perm:act", "{}", "test-deployment").
					Return(int64(1), nil)
			},
			shouldErr: false,
		},
		{
			name:             "Success_NullResource",
			actionID:         "action1",
			resourceServerID: "rs1",
			resourceID:       nil,
			action: Action{
				Name:        "Test Action",
				Handle:      "test-handle",
				Description: "Test Description",
				Permission:  "perm:act",
			},
			setupMocks: func(resourceID *string) {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("ExecuteContext", context.Background(),
					queryCreateAction, "action1", "rs1", (*string)(nil),
					"Test Action", "test-handle", "Test Description", "perm:act", "{}", "test-deployment").
					Return(int64(1), nil)
			},
			shouldErr: false,
		},
		{
			name:             "ExecuteError",
			actionID:         "action1",
			resourceServerID: "rs1",
			resourceID:       nil,
			action: Action{
				Name:        "Test Action",
				Handle:      "test-handle",
				Description: "Test Description",
				Permission:  "perm:act",
			},
			setupMocks: func(resourceID *string) {
				execError := errors.New("insert error")
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("ExecuteContext", context.Background(),
					queryCreateAction, "action1", "rs1", (*string)(nil),
					"Test Action", "test-handle", "Test Description", "perm:act", "{}", "test-deployment").
					Return(int64(0), execError)
			},
			shouldErr: true,
			checkError: func(err error) bool {
				suite.Contains(err.Error(), "failed to create action")
				return true
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create fresh mocks for each test case
			suite.mockDBProvider = providermock.NewDBProviderInterfaceMock(suite.T())
			suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())
			suite.store = &resourceStore{
				dbProvider:   suite.mockDBProvider,
				deploymentID: "test-deployment",
			}

			tc.setupMocks(tc.resourceID)
			err := suite.store.CreateAction(context.Background(),
				tc.actionID, tc.resourceServerID, tc.resourceID, tc.action)

			if tc.shouldErr {
				suite.Error(err)
				if tc.checkError != nil {
					tc.checkError(err)
				}
			} else {
				suite.NoError(err)
			}
		})
	}
}

func (suite *ResourceStoreTestSuite) TestGetAction() {
	testCases := []struct {
		name             string
		actionID         string
		resourceServerID string
		resourceID       *string
		setupMocks       func(*string)
		expectedActionID string
		expectedError    error
		shouldErr        bool
		checkError       func(error) bool
	}{
		{
			name:             "Success_AtResourceServer",
			actionID:         "action1",
			resourceServerID: "rs1",
			resourceID:       nil,
			setupMocks: func(resourceID *string) {
				var nilResID *string
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryGetActionByID, "action1", "rs1",
					nilResID, "test-deployment").Return([]map[string]interface{}{
					{
						"id":                 "action1",
						"resource_server_id": "rs1",
						"name":               "Test Action",
						"handle":             "test-handle",
						"description":        "Test Description",
						"permission":         "perm:a",
					},
				}, nil)
			},
			expectedActionID: "action1",
			shouldErr:        false,
		},
		{
			name:             "Success_AtResource",
			actionID:         "action1",
			resourceServerID: "rs1",
			resourceID:       &testResourceID1,
			setupMocks: func(resourceID *string) {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryGetActionByID, "action1", "rs1",
					resourceID, "test-deployment").Return([]map[string]interface{}{
					{
						"id":                 "action1",
						"resource_server_id": "rs1",
						"resource_id":        testResourceID1,
						"name":               "Test Action",
						"handle":             "test-handle",
						"description":        "Test Description",
						"permission":         "perm:a",
					},
				}, nil)
			},
			expectedActionID: "action1",
			shouldErr:        false,
		},
		{
			name:             "NotFound_AtResourceServer",
			actionID:         "nonexistent",
			resourceServerID: "rs1",
			resourceID:       nil,
			setupMocks: func(resourceID *string) {
				var nilResID *string
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryGetActionByID, "nonexistent", "rs1",
					nilResID, "test-deployment").Return([]map[string]interface{}{}, nil)
			},
			expectedError: errActionNotFound,
			shouldErr:     true,
		},
		{
			name:             "QueryError",
			actionID:         "action1",
			resourceServerID: "rs1",
			resourceID:       nil,
			setupMocks: func(resourceID *string) {
				var nilResID *string
				queryError := errors.New("query error")
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryGetActionByID, "action1", "rs1",
					nilResID, "test-deployment").Return(nil, queryError)
			},
			shouldErr: true,
			checkError: func(err error) bool {
				suite.Contains(err.Error(), "failed to get action")
				return true
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create fresh mocks for each test case
			suite.mockDBProvider = providermock.NewDBProviderInterfaceMock(suite.T())
			suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())
			suite.store = &resourceStore{
				dbProvider:   suite.mockDBProvider,
				deploymentID: "test-deployment",
			}

			tc.setupMocks(tc.resourceID)
			action, err := suite.store.GetAction(context.Background(),
				tc.actionID, tc.resourceServerID, tc.resourceID)

			if tc.shouldErr {
				suite.Error(err)
				if tc.expectedError != nil {
					suite.Equal(tc.expectedError, err)
				}
				if tc.checkError != nil {
					tc.checkError(err)
				}
				suite.Empty(action.ID)
			} else {
				suite.NoError(err)
				suite.Equal(tc.expectedActionID, action.ID)
			}
		})
	}
}

func (suite *ResourceStoreTestSuite) TestGetActionList() {
	testCases := []struct {
		name             string
		resourceServerID string
		resourceID       *string
		limit            int
		offset           int
		setupMocks       func(*string, int, int)
		expectedCount    int
		shouldErr        bool
		checkError       func(error) bool
	}{
		{
			name:             "Success_AtResourceServer",
			resourceServerID: "rs1",
			resourceID:       nil,
			limit:            testLimit,
			offset:           testOffset,
			setupMocks: func(resourceID *string, limit, offset int) {
				var nilResID *string
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryGetActionList, "rs1", nilResID,
					limit, offset, "test-deployment").Return([]map[string]interface{}{
					{
						"id":                 "action1",
						"resource_server_id": "rs1",
						"name":               "Action 1",
						"handle":             "action-1",
						"description":        "Description 1",
						"permission":         "perm:1",
					},
					{
						"id":                 "action2",
						"resource_server_id": "rs1",
						"name":               "Action 2",
						"handle":             "action-2",
						"description":        "Description 2",
						"permission":         "perm:2",
					},
				}, nil)
			},
			expectedCount: 2,
			shouldErr:     false,
		},
		{
			name:             "QueryError_AtResourceServer",
			resourceServerID: "rs1",
			resourceID:       nil,
			limit:            testLimit,
			offset:           testOffset,
			setupMocks: func(resourceID *string, limit, offset int) {
				var nilResID *string
				queryError := errors.New("query error")
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryGetActionList, "rs1", nilResID,
					limit, offset, "test-deployment").Return(nil, queryError)
			},
			shouldErr: true,
		},
		{
			name:             "InvalidRowData",
			resourceServerID: "rs1",
			resourceID:       nil,
			limit:            testLimit,
			offset:           testOffset,
			setupMocks: func(resourceID *string, limit, offset int) {
				var nilResID *string
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryGetActionList, "rs1", nilResID,
					limit, offset, "test-deployment").Return([]map[string]interface{}{
					{
						"internal_id": 123, // Invalid type
						"name":        "Action 1",
					},
				}, nil)
			},
			shouldErr: true,
		},
		{
			name:             "Success_AtResource",
			resourceServerID: "rs1",
			resourceID:       &testResourceID1,
			limit:            10,
			offset:           0,
			setupMocks: func(resourceID *string, limit, offset int) {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryGetActionList, "rs1", resourceID, 10, 0,
					"test-deployment").Return([]map[string]interface{}{
					{
						"id":                 "action1",
						"resource_server_id": "rs1",
						"resource_id":        "res1",
						"name":               "Action 1",
						"handle":             "action-1",
						"description":        "Description 1",
						"permission":         "perm:r",
					},
				}, nil)
			},
			expectedCount: 1,
			shouldErr:     false,
		},
		{
			name:             "QueryError_AtResource",
			resourceServerID: "rs1",
			resourceID:       &testResourceID1,
			limit:            10,
			offset:           0,
			setupMocks: func(resourceID *string, limit, offset int) {
				queryError := errors.New("query error")
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryGetActionList, "rs1", resourceID, 10, 0,
					"test-deployment").Return(nil, queryError)
			},
			shouldErr: true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create fresh mocks for each test case
			suite.mockDBProvider = providermock.NewDBProviderInterfaceMock(suite.T())
			suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())
			suite.store = &resourceStore{
				dbProvider:   suite.mockDBProvider,
				deploymentID: "test-deployment",
			}

			tc.setupMocks(tc.resourceID, tc.limit, tc.offset)

			actions, err := suite.store.GetActionList(context.Background(),

				tc.resourceServerID, tc.resourceID, tc.limit, tc.offset)

			if tc.shouldErr {
				suite.Error(err)
				suite.Nil(actions)
				if tc.checkError != nil {
					tc.checkError(err)
				}
			} else {
				suite.NoError(err)
				suite.Len(actions, tc.expectedCount)
				if tc.expectedCount > 0 {
					suite.NotEmpty(actions[0].ID)
				}
			}
		})
	}
}

func (suite *ResourceStoreTestSuite) TestGetActionListCount() {
	testCases := []struct {
		name             string
		resourceServerID string
		resourceID       *string
		setupMocks       func(*string)
		expectedCount    int
		shouldErr        bool
	}{
		{
			name:             "Success_AtResourceServer",
			resourceServerID: "rs1",
			resourceID:       nil,
			setupMocks: func(resourceID *string) {
				var nilResID *string
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryGetActionListCount, "rs1",
					nilResID, "test-deployment").Return([]map[string]interface{}{
					{"total": int64(15)},
				}, nil)
			},
			expectedCount: 15,
			shouldErr:     false,
		},
		{
			name:             "QueryError_AtResourceServer",
			resourceServerID: "rs1",
			resourceID:       nil,
			setupMocks: func(resourceID *string) {
				var nilResID *string
				queryError := errors.New("query error")
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryGetActionListCount, "rs1",
					nilResID, "test-deployment").Return(nil, queryError)
			},
			expectedCount: 0,
			shouldErr:     true,
		},
		{
			name:             "Success_AtResource",
			resourceServerID: "rs1",
			resourceID:       &testResourceID1,
			setupMocks: func(resourceID *string) {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryGetActionListCount, "rs1", resourceID,
					"test-deployment").Return([]map[string]interface{}{
					{"total": int64(5)},
				}, nil)
			},
			expectedCount: 5,
			shouldErr:     false,
		},
		{
			name:             "QueryError_AtResource",
			resourceServerID: "rs1",
			resourceID:       &testResourceID1,
			setupMocks: func(resourceID *string) {
				queryError := errors.New("query error")
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryGetActionListCount, "rs1", resourceID,
					"test-deployment").Return(nil, queryError)
			},
			expectedCount: 0,
			shouldErr:     true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create fresh mocks for each test case
			suite.mockDBProvider = providermock.NewDBProviderInterfaceMock(suite.T())
			suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())
			suite.store = &resourceStore{
				dbProvider:   suite.mockDBProvider,
				deploymentID: "test-deployment",
			}

			tc.setupMocks(tc.resourceID)
			count, err := suite.store.GetActionListCount(context.Background(),
				tc.resourceServerID, tc.resourceID)

			if tc.shouldErr {
				suite.Error(err)
			} else {
				suite.NoError(err)
			}
			suite.Equal(tc.expectedCount, count)
		})
	}
}

func (suite *ResourceStoreTestSuite) TestUpdateAction() {
	testCases := []struct {
		name             string
		actionID         string
		resourceServerID string
		resourceID       *string
		action           Action
		setupMocks       func(*string)
		shouldErr        bool
		checkError       func(error) bool
	}{
		{
			name:             "Success_AtResourceServer",
			actionID:         "action1",
			resourceServerID: "rs1",
			resourceID:       nil,
			action: Action{
				Name:        "Updated Action",
				Description: "Updated Description",
			},
			setupMocks: func(resourceID *string) {
				var nilResID *string
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("ExecuteContext", context.Background(),
					queryUpdateAction, "Updated Action",
					"Updated Description", "{}", "action1", "rs1", nilResID, "test-deployment").
					Return(int64(1), nil)
			},
			shouldErr: false,
		},
		{
			name:             "Success_WithResourceID",
			actionID:         "action1",
			resourceServerID: "rs1",
			resourceID:       &testResourceID1,
			action: Action{
				Name:        "Updated Action",
				Description: "Updated Description",
			},
			setupMocks: func(resourceID *string) {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("ExecuteContext", context.Background(),
					queryUpdateAction, "Updated Action",
					"Updated Description", "{}", "action1", "rs1", resourceID, "test-deployment").
					Return(int64(1), nil)
			},
			shouldErr: false,
		},
		{
			name:             "ExecuteError",
			actionID:         "action1",
			resourceServerID: "rs1",
			resourceID:       nil,
			action: Action{
				Name:        "Updated Action",
				Description: "Updated Description",
			},
			setupMocks: func(resourceID *string) {
				var nilResID *string
				execError := errors.New("update error")
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("ExecuteContext", context.Background(),
					queryUpdateAction, "Updated Action",
					"Updated Description", "{}", "action1", "rs1", nilResID, "test-deployment").
					Return(int64(0), execError)
			},
			shouldErr: true,
			checkError: func(err error) bool {
				suite.Contains(err.Error(), "failed to update action")
				return true
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create fresh mocks for each test case
			suite.mockDBProvider = providermock.NewDBProviderInterfaceMock(suite.T())
			suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())
			suite.store = &resourceStore{
				dbProvider:   suite.mockDBProvider,
				deploymentID: "test-deployment",
			}

			tc.setupMocks(tc.resourceID)
			err := suite.store.UpdateAction(context.Background(),
				tc.actionID, tc.resourceServerID, tc.resourceID, tc.action)

			if tc.shouldErr {
				suite.Error(err)
				if tc.checkError != nil {
					tc.checkError(err)
				}
			} else {
				suite.NoError(err)
			}
		})
	}
}

func (suite *ResourceStoreTestSuite) TestDeleteAction() {
	testCases := []struct {
		name             string
		actionID         string
		resourceServerID string
		resourceID       *string
		setupMocks       func(*string)
		shouldErr        bool
		checkError       func(error) bool
	}{
		{
			name:             "Success_AtResourceServer",
			actionID:         "action1",
			resourceServerID: "rs1",
			resourceID:       nil,
			setupMocks: func(resourceID *string) {
				var nilResID *string
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On(
					"ExecuteContext", context.Background(),
					queryDeleteAction, "action1", "rs1", nilResID, "test-deployment",
				).Return(int64(1), nil)
			},
			shouldErr: false,
		},
		{
			name:             "Success_WithResourceID",
			actionID:         "action1",
			resourceServerID: "rs1",
			resourceID:       &testResourceID1,
			setupMocks: func(resourceID *string) {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("ExecuteContext", context.Background(),
					queryDeleteAction, "action1", "rs1",
					resourceID, "test-deployment").Return(int64(1), nil)
			},
			shouldErr: false,
		},
		{
			name:             "ExecuteError",
			actionID:         "action1",
			resourceServerID: "rs1",
			resourceID:       nil,
			setupMocks: func(resourceID *string) {
				var nilResID *string
				execError := errors.New("delete error")
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On(
					"ExecuteContext", context.Background(),
					queryDeleteAction, "action1", "rs1", nilResID, "test-deployment",
				).Return(int64(0), execError)
			},
			shouldErr: true,
			checkError: func(err error) bool {
				suite.Contains(err.Error(), "failed to delete action")
				return true
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create fresh mocks for each test case
			suite.mockDBProvider = providermock.NewDBProviderInterfaceMock(suite.T())
			suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())
			suite.store = &resourceStore{
				dbProvider:   suite.mockDBProvider,
				deploymentID: "test-deployment",
			}

			tc.setupMocks(tc.resourceID)
			err := suite.store.DeleteAction(context.Background(),
				tc.actionID, tc.resourceServerID, tc.resourceID)

			if tc.shouldErr {
				suite.Error(err)
				if tc.checkError != nil {
					tc.checkError(err)
				}
			} else {
				suite.NoError(err)
			}
		})
	}
}

func (suite *ResourceStoreTestSuite) TestIsActionExist() {
	testCases := []struct {
		name             string
		actionID         string
		resourceServerID string
		resourceID       *string
		setupMocks       func(*string)
		expectedExists   bool
		shouldErr        bool
		checkError       func(error) bool
	}{
		{
			name:             "Exists_AtResourceServer",
			actionID:         "action1",
			resourceServerID: "rs1",
			resourceID:       nil,
			setupMocks: func(resourceID *string) {
				var nilResID *string
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryCheckActionExists, "action1", "rs1",
					nilResID, "test-deployment").Return([]map[string]interface{}{
					{"count": int64(1)},
				}, nil)
			},
			expectedExists: true,
			shouldErr:      false,
		},
		{
			name:             "NotExists_AtResourceServer",
			actionID:         "nonexistent",
			resourceServerID: "rs1",
			resourceID:       nil,
			setupMocks: func(resourceID *string) {
				var nilResID *string
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryCheckActionExists, "nonexistent", "rs1",
					nilResID, "test-deployment").Return([]map[string]interface{}{
					{"count": int64(0)},
				}, nil)
			},
			expectedExists: false,
			shouldErr:      false,
		},
		{
			name:             "Exists_AtResource",
			actionID:         "action1",
			resourceServerID: "rs1",
			resourceID:       &testResourceID1,
			setupMocks: func(resourceID *string) {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryCheckActionExists, "action1", "rs1",
					resourceID, "test-deployment").Return([]map[string]interface{}{
					{"count": int64(1)},
				}, nil)
			},
			expectedExists: true,
			shouldErr:      false,
		},
		{
			name:             "QueryError",
			actionID:         "action1",
			resourceServerID: "rs1",
			resourceID:       nil,
			setupMocks: func(resourceID *string) {
				var nilResID *string
				queryError := errors.New("query error")
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryCheckActionExists, "action1", "rs1",
					nilResID, "test-deployment").Return(nil, queryError)
			},
			expectedExists: false,
			shouldErr:      true,
			checkError: func(err error) bool {
				suite.Contains(err.Error(), "failed to check action existence")
				return true
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create fresh mocks for each test case
			suite.mockDBProvider = providermock.NewDBProviderInterfaceMock(suite.T())
			suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())
			suite.store = &resourceStore{
				dbProvider:   suite.mockDBProvider,
				deploymentID: "test-deployment",
			}

			tc.setupMocks(tc.resourceID)
			exists, err := suite.store.IsActionExist(context.Background(),
				tc.actionID, tc.resourceServerID, tc.resourceID)

			if tc.shouldErr {
				suite.Error(err)
				if tc.checkError != nil {
					tc.checkError(err)
				}
			} else {
				suite.NoError(err)
			}
			suite.Equal(tc.expectedExists, exists)
		})
	}
}

func (suite *ResourceStoreTestSuite) TestCheckActionHandleExists() {
	testCases := []struct {
		name             string
		resourceServerID string
		resourceID       *string
		actionHandle     string
		setupMocks       func(*string)
		expectedExists   bool
		shouldErr        bool
		checkError       func(error) bool
	}{
		{
			name:             "Exists_AtResourceServer",
			resourceServerID: "rs1",
			resourceID:       nil,
			actionHandle:     "Test Action",
			setupMocks: func(resourceID *string) {
				var nilResID *string
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryCheckActionHandleExists, "rs1",
					nilResID, "Test Action", "test-deployment").Return([]map[string]interface{}{
					{"count": int64(1)},
				}, nil)
			},
			expectedExists: true,
			shouldErr:      false,
		},
		{
			name:             "NotExists_AtResourceServer",
			resourceServerID: "rs1",
			resourceID:       nil,
			actionHandle:     "Nonexistent",
			setupMocks: func(resourceID *string) {
				var nilResID *string
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryCheckActionHandleExists, "rs1",
					nilResID, "Nonexistent", "test-deployment").Return([]map[string]interface{}{
					{"count": int64(0)},
				}, nil)
			},
			expectedExists: false,
			shouldErr:      false,
		},
		{
			name:             "NotExists_AtResource",
			resourceServerID: "rs1",
			resourceID:       &testResourceID1,
			actionHandle:     "Test Action",
			setupMocks: func(resourceID *string) {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryCheckActionHandleExists, "rs1",
					resourceID, "Test Action", "test-deployment").Return([]map[string]interface{}{
					{"count": int64(0)},
				}, nil)
			},
			expectedExists: false,
			shouldErr:      false,
		},
		{
			name:             "QueryError",
			resourceServerID: "rs1",
			resourceID:       nil,
			actionHandle:     "Test Action",
			setupMocks: func(resourceID *string) {
				var nilResID *string
				queryError := errors.New("query error")
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryCheckActionHandleExists, "rs1",
					nilResID, "Test Action", "test-deployment").Return(nil, queryError)
			},
			expectedExists: false,
			shouldErr:      true,
			checkError: func(err error) bool {
				suite.Contains(err.Error(), "failed to check action handle")
				return true
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create fresh mocks for each test case
			suite.mockDBProvider = providermock.NewDBProviderInterfaceMock(suite.T())
			suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())
			suite.store = &resourceStore{
				dbProvider:   suite.mockDBProvider,
				deploymentID: "test-deployment",
			}

			tc.setupMocks(tc.resourceID)
			exists, err := suite.store.CheckActionHandleExists(context.Background(),
				tc.resourceServerID,
				tc.resourceID, tc.actionHandle)

			if tc.shouldErr {
				suite.Error(err)
				if tc.checkError != nil {
					tc.checkError(err)
				}
			} else {
				suite.NoError(err)
			}
			suite.Equal(tc.expectedExists, exists)
		})
	}
}

// Helper Function Tests

func (suite *ResourceStoreTestSuite) TestGetConfigDBClient_Success() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)

	client, err := suite.store.getConfigDBClient()

	suite.NoError(err)
	suite.NotNil(client)
	suite.Equal(suite.mockDBClient, client)
}

func (suite *ResourceStoreTestSuite) TestGetConfigDBClient_Error() {
	dbError := errors.New("database connection error")
	suite.mockDBProvider.On("GetConfigDBClient").Return(nil, dbError)

	client, err := suite.store.getConfigDBClient()

	suite.Error(err)
	suite.Nil(client)
	suite.Contains(err.Error(), "failed to get config DB client")
}

func (suite *ResourceStoreTestSuite) TestWithDBClient_Success() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)

	called := false
	err := suite.store.withDBClient(func(dbClient provider.DBClientInterface) error {
		called = true
		suite.Equal(suite.mockDBClient, dbClient)
		return nil
	})

	suite.NoError(err)
	suite.True(called)
}

func (suite *ResourceStoreTestSuite) TestWithDBClient_DBClientError() {
	dbError := errors.New("database connection error")
	suite.mockDBProvider.On("GetConfigDBClient").Return(nil, dbError)

	called := false
	err := suite.store.withDBClient(func(dbClient provider.DBClientInterface) error {
		called = true
		return nil
	})

	suite.Error(err)
	suite.False(called, "Function should not be called when DB client retrieval fails")
	suite.Contains(err.Error(), "failed to get config DB client")
}

func (suite *ResourceStoreTestSuite) TestWithDBClient_InnerFunctionError() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)

	expectedErr := errors.New("inner function error")
	err := suite.store.withDBClient(func(dbClient provider.DBClientInterface) error {
		return expectedErr
	})

	suite.Error(err)
	suite.Equal(expectedErr, err, "Error from inner function should be propagated")
}

func (suite *ResourceStoreTestSuite) TestParseCountResult() {
	testCases := []struct {
		name          string
		results       []map[string]interface{}
		expectedCount int
		shouldErr     bool
		errContains   string
	}{
		{
			name: "Success_TotalField_Int64",
			results: []map[string]interface{}{
				{"total": int64(42)},
			},
			expectedCount: 42,
			shouldErr:     false,
		},
		{
			name: "Success_CountField_Int64",
			results: []map[string]interface{}{
				{"count": int64(42)},
			},
			expectedCount: 42,
			shouldErr:     false,
		},
		{
			name: "Success_TotalField_Int",
			results: []map[string]interface{}{
				{"total": int(42)},
			},
			expectedCount: 42,
			shouldErr:     false,
		},
		{
			name: "Success_TotalField_Float64",
			results: []map[string]interface{}{
				{"total": float64(42)},
			},
			expectedCount: 42,
			shouldErr:     false,
		},
		{
			name:          "Error_EmptyResults",
			results:       []map[string]interface{}{},
			expectedCount: 0,
			shouldErr:     true,
			errContains:   "no count result returned",
		},
		{
			name: "Error_MissingField",
			results: []map[string]interface{}{
				{"other": int64(42)},
			},
			expectedCount: 0,
			shouldErr:     true,
			errContains:   "count field not found",
		},
		{
			name: "Error_InvalidType",
			results: []map[string]interface{}{
				{"total": "not_a_number"},
			},
			expectedCount: 0,
			shouldErr:     true,
			errContains:   "unexpected count type",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			count, err := parseCountResult(tc.results)

			if tc.shouldErr {
				suite.Error(err)
				suite.Contains(err.Error(), tc.errContains)
				suite.Equal(0, count)
			} else {
				suite.NoError(err)
				suite.Equal(tc.expectedCount, count)
			}
		})
	}
}

func (suite *ResourceStoreTestSuite) TestParseBoolFromCount() {
	testCases := []struct {
		name         string
		results      []map[string]interface{}
		expectedBool bool
		shouldErr    bool
	}{
		{
			name: "Success_True",
			results: []map[string]interface{}{
				{"count": int64(5)},
			},
			expectedBool: true,
			shouldErr:    false,
		},
		{
			name: "Success_False",
			results: []map[string]interface{}{
				{"count": int64(0)},
			},
			expectedBool: false,
			shouldErr:    false,
		},
		{
			name:         "Error_EmptyResults",
			results:      []map[string]interface{}{},
			expectedBool: false,
			shouldErr:    true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			exists, err := parseBoolFromCount(tc.results)

			if tc.shouldErr {
				suite.Error(err)
				suite.False(exists)
			} else {
				suite.NoError(err)
				suite.Equal(tc.expectedBool, exists)
			}
		})
	}
}

func (suite *ResourceStoreTestSuite) TestBuildResourceServerFromResultRow() {
	testCases := []struct {
		name                   string
		row                    map[string]interface{}
		expectedResourceServer ResourceServer
		shouldErr              bool
		errContains            string
	}{
		{
			name: "Success_AllFields",
			row: map[string]interface{}{
				"internal_id": 50,
				"id":          "rs1",
				"ou_id":       "ou1",
				"name":        "Test Server",
				"description": "Test Description",
				"identifier":  "test-identifier",
				"properties":  []byte(`{"delimiter":"|"}`),
			},
			expectedResourceServer: ResourceServer{
				ID:          "rs1",
				OUID:        "ou1",
				Name:        "Test Server",
				Description: "Test Description",
				Identifier:  "test-identifier",
				Delimiter:   "|",
			},
			shouldErr: false,
		},
		{
			name: "Success_OptionalFields",
			row: map[string]interface{}{
				"internal_id": 51,
				"id":          "rs1",
				"ou_id":       "ou1",
				"name":        "Test Server",
			},
			expectedResourceServer: ResourceServer{
				ID:          "rs1",
				OUID:        "ou1",
				Name:        "Test Server",
				Description: "",
				Identifier:  "",
			},
			shouldErr: false,
		},
		{
			name: "Success_PropertiesString",
			row: map[string]interface{}{
				"internal_id": 52,
				"id":          "rs1",
				"ou_id":       "ou1",
				"name":        "Test Server",
				"properties":  `{"delimiter":"."}`,
			},
			expectedResourceServer: ResourceServer{
				ID:        "rs1",
				OUID:      "ou1",
				Name:      "Test Server",
				Delimiter: ".",
			},
			shouldErr: false,
		},
		{
			name: "Error_MissingResourceServerID",
			row: map[string]interface{}{
				"internal_id": 60,
				"ou_id":       "ou1",
				"name":        "Test Server",
			},
			shouldErr:   true,
			errContains: "id",
		},
		{
			name: "Error_InvalidResourceServerID",
			row: map[string]interface{}{
				"internal_id": 61,
				"id":          123,
				"ou_id":       "ou1",
				"name":        "Test Server",
			},
			shouldErr:   true,
			errContains: "id",
		},
		{
			name: "Error_MissingOUID",
			row: map[string]interface{}{
				"internal_id": 62,
				"id":          "rs1",
				"name":        "Test Server",
			},
			shouldErr:   true,
			errContains: "ou_id",
		},
		{
			name: "Error_InvalidOUID",
			row: map[string]interface{}{
				"internal_id": 63,
				"id":          "rs1",
				"ou_id":       123,
				"name":        "Test Server",
			},
			shouldErr:   true,
			errContains: "ou_id",
		},
		{
			name: "Error_MissingName",
			row: map[string]interface{}{
				"internal_id": 64,
				"id":          "rs1",
				"ou_id":       "ou1",
			},
			shouldErr:   true,
			errContains: "name",
		},
		{
			name: "Error_InvalidName",
			row: map[string]interface{}{
				"internal_id": 65,
				"id":          "rs1",
				"ou_id":       "ou1",
				"name":        123,
			},
			shouldErr:   true,
			errContains: "name",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			rs, err := buildResourceServerFromResultRow(tc.row)

			if tc.shouldErr {
				suite.Error(err)
				suite.Contains(err.Error(), tc.errContains)
			} else {
				suite.NoError(err)
				suite.Equal(tc.expectedResourceServer.ID, rs.ID)
				suite.Equal(tc.expectedResourceServer.OUID, rs.OUID)
				suite.Equal(tc.expectedResourceServer.Name, rs.Name)
				suite.Equal(tc.expectedResourceServer.Description, rs.Description)
				suite.Equal(tc.expectedResourceServer.Identifier, rs.Identifier)
				suite.Equal(tc.expectedResourceServer.Delimiter, rs.Delimiter)
			}
		})
	}
}

func (suite *ResourceStoreTestSuite) TestBuildResourceFromResultRow() {
	parentID := testParentID1
	testCases := []struct {
		name             string
		row              map[string]interface{}
		expectedResource Resource
		shouldErr        bool
		errContains      string
	}{
		{
			name: "Success_WithParent",
			row: map[string]interface{}{
				"internal_id":        70,
				"id":                 "res1",
				"resource_server_id": "rs1",
				"name":               "Test Resource",
				"handle":             "test-handle",
				"description":        "Test Description",
				"parent_resource_id": parentID,
				"permission":         "perm:r",
			},
			expectedResource: Resource{
				ID:          "res1",
				Name:        "Test Resource",
				Handle:      "test-handle",
				Description: "Test Description",
				Permission:  "perm:r",
				Parent:      &parentID,
			},
			shouldErr: false,
		},
		{
			name: "Success_NullParent",
			row: map[string]interface{}{
				"internal_id":        71,
				"id":                 "res1",
				"resource_server_id": "rs1",
				"name":               "Test Resource",
				"handle":             "test-handle",
				"description":        "Test Description",
				"parent_resource_id": "",
				"permission":         "perm:r",
			},
			expectedResource: Resource{
				ID:          "res1",
				Name:        "Test Resource",
				Handle:      "test-handle",
				Description: "Test Description",
				Permission:  "perm:r",
				Parent:      nil,
			},
			shouldErr: false,
		},
		{
			name: "Success_EmptyDescription",
			row: map[string]interface{}{
				"internal_id": 72,
				"id":          "res1",
				"name":        "Test Resource",
				"handle":      "test-handle",
				"description": "",
				"permission":  "perm:r",
			},
			expectedResource: Resource{
				ID:          "res1",
				Name:        "Test Resource",
				Handle:      "test-handle",
				Description: "",
				Permission:  "perm:r",
				Parent:      nil,
			},
			shouldErr: false,
		},
		{
			name: "Success_MissingDescription",
			row: map[string]interface{}{
				"internal_id": 73,
				"id":          "res1",
				"name":        "Test Resource",
				"handle":      "test-handle",
				"permission":  "perm:r",
			},
			expectedResource: Resource{
				ID:          "res1",
				Name:        "Test Resource",
				Handle:      "test-handle",
				Description: "",
				Permission:  "perm:r",
				Parent:      nil,
			},
			shouldErr: false,
		},
		{
			name: "Error_MissingResourceID",
			row: map[string]interface{}{
				"internal_id": 80,
				"name":        "Test Resource",
				"handle":      "test-handle",
			},
			shouldErr:   true,
			errContains: "id",
		},
		{
			name: "Error_InvalidResourceID",
			row: map[string]interface{}{
				"internal_id": 81,
				"id":          123,
				"name":        "Test Resource",
				"handle":      "test-handle",
			},
			shouldErr:   true,
			errContains: "id",
		},
		{
			name: "Error_MissingName",
			row: map[string]interface{}{
				"internal_id": 82,
				"id":          "res1",
				"handle":      "test-handle",
			},
			shouldErr:   true,
			errContains: "name",
		},
		{
			name: "Error_InvalidName",
			row: map[string]interface{}{
				"internal_id": 83,
				"id":          "res1",
				"name":        123,
				"handle":      "test-handle",
			},
			shouldErr:   true,
			errContains: "name",
		},
		{
			name: "Error_MissingHandle",
			row: map[string]interface{}{
				"internal_id": 84,
				"id":          "res1",
				"name":        "Test Resource",
			},
			shouldErr:   true,
			errContains: "handle",
		},
		{
			name: "Error_InvalidHandle",
			row: map[string]interface{}{
				"internal_id": 85,
				"id":          "res1",
				"name":        "Test Resource",
				"handle":      123,
			},
			shouldErr:   true,
			errContains: "handle",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			res, err := buildResourceFromResultRow(tc.row)

			if tc.shouldErr {
				suite.Error(err)
				suite.Contains(err.Error(), tc.errContains)
			} else {
				suite.NoError(err)
				suite.Equal(tc.expectedResource.ID, res.ID)
				suite.Equal(tc.expectedResource.Name, res.Name)
				suite.Equal(tc.expectedResource.Handle, res.Handle)
				suite.Equal(tc.expectedResource.Description, res.Description)
				suite.Equal(tc.expectedResource.Permission, res.Permission)
				if tc.expectedResource.Parent != nil {
					suite.NotNil(res.Parent)
					suite.Equal(*tc.expectedResource.Parent, *res.Parent)
				} else {
					suite.Nil(res.Parent)
				}
			}
		})
	}
}

func (suite *ResourceStoreTestSuite) TestBuildActionFromResultRow() {
	testCases := []struct {
		name           string
		row            map[string]interface{}
		expectedAction Action
		shouldErr      bool
		errContains    string
	}{
		{
			name: "Success_WithResourceID",
			row: map[string]interface{}{
				"id":                 "action1",
				"resource_server_id": "rs1",
				"resource_id":        testResourceID1,
				"name":               "Test Action",
				"handle":             "test-handle",
				"description":        "Test Description",
			},
			expectedAction: Action{
				ID:          "action1",
				Name:        "Test Action",
				Handle:      "test-handle",
				Description: "Test Description",
			},
			shouldErr: false,
		},
		{
			name: "Success_NullResource",
			row: map[string]interface{}{
				"id":                 "action1",
				"resource_server_id": "rs1",
				"resource_id":        "",
				"name":               "Test Action",
				"handle":             "test-handle",
				"description":        "Test Description",
			},
			expectedAction: Action{
				ID:          "action1",
				Name:        "Test Action",
				Handle:      "test-handle",
				Description: "Test Description",
			},
			shouldErr: false,
		},
		{
			name: "Success_EmptyDescription",
			row: map[string]interface{}{
				"id":          "action1",
				"name":        "Test Action",
				"handle":      "test-handle",
				"description": "",
			},
			expectedAction: Action{
				ID:          "action1",
				Name:        "Test Action",
				Handle:      "test-handle",
				Description: "",
			},
			shouldErr: false,
		},
		{
			name: "Success_MissingDescription",
			row: map[string]interface{}{
				"id":     "action1",
				"name":   "Test Action",
				"handle": "test-handle",
			},
			expectedAction: Action{
				ID:          "action1",
				Name:        "Test Action",
				Handle:      "test-handle",
				Description: "",
			},
			shouldErr: false,
		},
		{
			name: "Error_MissingActionID",
			row: map[string]interface{}{
				"name":   "Test Action",
				"handle": "test-handle",
			},
			shouldErr:   true,
			errContains: "id",
		},
		{
			name: "Error_InvalidActionID",
			row: map[string]interface{}{
				"id":     123,
				"name":   "Test Action",
				"handle": "test-handle",
			},
			shouldErr:   true,
			errContains: "id",
		},
		{
			name: "Error_MissingName",
			row: map[string]interface{}{
				"id":     "action1",
				"handle": "test-handle",
			},
			shouldErr:   true,
			errContains: "name",
		},
		{
			name: "Error_InvalidName",
			row: map[string]interface{}{
				"id":     "action1",
				"name":   123,
				"handle": "test-handle",
			},
			shouldErr:   true,
			errContains: "name",
		},
		{
			name: "Error_MissingHandle",
			row: map[string]interface{}{
				"id":   "action1",
				"name": "Test Action",
			},
			shouldErr:   true,
			errContains: "handle",
		},
		{
			name: "Error_InvalidHandle",
			row: map[string]interface{}{
				"id":     "action1",
				"name":   "Test Action",
				"handle": 123,
			},
			shouldErr:   true,
			errContains: "handle",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			action, err := buildActionFromResultRow(tc.row)

			if tc.shouldErr {
				suite.Error(err)
				suite.Contains(err.Error(), tc.errContains)
			} else {
				suite.NoError(err)
				suite.Equal(tc.expectedAction.ID, action.ID)
				suite.Equal(tc.expectedAction.Name, action.Name)
				suite.Equal(tc.expectedAction.Handle, action.Handle)
				suite.Equal(tc.expectedAction.Description, action.Description)
			}
		})
	}
}

// resolveNullableString Tests

func (suite *ResourceStoreTestSuite) TestResolveNullableString() {
	testCases := []struct {
		name     string
		value    string
		expected interface{}
	}{
		{
			name:     "Success_NonEmptyValue",
			value:    "https://api.example.com",
			expected: "https://api.example.com",
		},
		{
			name:     "Success_AnotherNonEmptyValue",
			value:    "urn:example:resource:server",
			expected: "urn:example:resource:server",
		},
		{
			name:     "Success_EmptyValue_ReturnsNil",
			value:    "",
			expected: nil,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			result := resolveNullableString(tc.value)
			suite.Equal(tc.expected, result)
		})
	}
}

// TestValidatePermissions tests the ValidatePermissions function with various scenarios.
func (suite *ResourceStoreTestSuite) TestValidatePermissions() {
	testCases := []struct {
		name            string
		resServerID     string
		permissions     []string
		setupMocks      func()
		expectedInvalid []string
		shouldErr       bool
		checkError      func(error) bool
	}{
		{
			name:            "Success_EmptyPermissions",
			resServerID:     "rs1",
			permissions:     []string{},
			setupMocks:      func() {},
			expectedInvalid: []string{},
			shouldErr:       false,
		},
		{
			name:        "Success_AllPermissionsValid",
			resServerID: "rs1",
			permissions: []string{"read", "write", "delete"},
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryValidatePermissions, "rs1",
					"test-deployment", `["read","write","delete"]`).
					Return([]map[string]interface{}{}, nil)
			},
			expectedInvalid: nil,
			shouldErr:       false,
		},
		{
			name:        "Success_SomePermissionsInvalid",
			resServerID: "rs1",
			permissions: []string{"read", "write", "invalid1", "delete", "invalid2"},
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryValidatePermissions, "rs1", "test-deployment",
					`["read","write","invalid1","delete","invalid2"]`).
					Return([]map[string]interface{}{
						{"permission": "invalid1"},
						{"permission": "invalid2"},
					}, nil)
			},
			expectedInvalid: []string{"invalid1", "invalid2"},
			shouldErr:       false,
		},
		{
			name:        "Success_AllPermissionsInvalid",
			resServerID: "rs2",
			permissions: []string{"badperm1", "badperm2", "badperm3"},
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryValidatePermissions, "rs2", "test-deployment",
					`["badperm1","badperm2","badperm3"]`).
					Return([]map[string]interface{}{
						{"permission": "badperm1"},
						{"permission": "badperm2"},
						{"permission": "badperm3"},
					}, nil)
			},
			expectedInvalid: []string{"badperm1", "badperm2", "badperm3"},
			shouldErr:       false,
		},
		{
			name:        "Error_InvalidRowDataType_Int",
			resServerID: "rs1",
			permissions: []string{"read", "write"},
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryValidatePermissions, "rs1", "test-deployment", `["read","write"]`).
					Return([]map[string]interface{}{
						{"permission": "read"},
						{"permission": 123}, // invalid type (int) - should cause error
					}, nil)
			},
			expectedInvalid: nil,
			shouldErr:       true,
			checkError: func(err error) bool {
				suite.Contains(err.Error(), "permission field is missing or invalid")
				return true
			},
		},
		{
			name:        "Error_InvalidRowDataType_Nil",
			resServerID: "rs1",
			permissions: []string{"admin"},
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryValidatePermissions, "rs1", "test-deployment", `["admin"]`).
					Return([]map[string]interface{}{
						{"permission": nil}, // invalid type (nil) - should cause error
					}, nil)
			},
			expectedInvalid: nil,
			shouldErr:       true,
			checkError: func(err error) bool {
				suite.Contains(err.Error(), "permission field is missing or invalid")
				return true
			},
		},
		{
			name:        "Error_MissingPermissionField",
			resServerID: "rs1",
			permissions: []string{"read"},
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryValidatePermissions, "rs1", "test-deployment", `["read"]`).
					Return([]map[string]interface{}{
						{"some_other_field": "value"}, // missing "permission" field - should cause error
					}, nil)
			},
			expectedInvalid: nil,
			shouldErr:       true,
			checkError: func(err error) bool {
				suite.Contains(err.Error(), "permission field is missing or invalid")
				return true
			},
		},
		{
			name:        "Error_QueryError",
			resServerID: "rs1",
			permissions: []string{"read", "write"},
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryValidatePermissions, "rs1", "test-deployment", `["read","write"]`).
					Return(nil, errors.New("database connection lost"))
			},
			expectedInvalid: nil,
			shouldErr:       true,
			checkError: func(err error) bool {
				suite.Contains(err.Error(), "failed to validate permissions")
				suite.Contains(err.Error(), "database connection lost")
				return true
			},
		},
		{
			name:        "Error_GetDBClientError",
			resServerID: "rs1",
			permissions: []string{"read", "write"},
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(nil, errors.New("db client error"))
			},
			expectedInvalid: nil,
			shouldErr:       true,
			checkError: func(err error) bool {
				suite.Contains(err.Error(), "db client error")
				return true
			},
		},
		{
			name:        "Success_SinglePermission_Valid",
			resServerID: "rs5",
			permissions: []string{"admin"},
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryValidatePermissions, "rs5", "test-deployment", `["admin"]`).
					Return([]map[string]interface{}{}, nil)
			},
			expectedInvalid: nil,
			shouldErr:       false,
		},
		{
			name:        "Success_SinglePermission_Invalid",
			resServerID: "rs5",
			permissions: []string{"nonexistent"},
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", context.Background(),
					queryValidatePermissions, "rs5", "test-deployment", `["nonexistent"]`).
					Return([]map[string]interface{}{
						{"permission": "nonexistent"},
					}, nil)
			},
			expectedInvalid: []string{"nonexistent"},
			shouldErr:       false,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create fresh mocks for each test case
			suite.mockDBProvider = providermock.NewDBProviderInterfaceMock(suite.T())
			suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())
			suite.store = &resourceStore{
				dbProvider:   suite.mockDBProvider,
				deploymentID: "test-deployment",
			}

			tc.setupMocks()

			invalidPerms, err := suite.store.ValidatePermissions(context.Background(),
				tc.resServerID, tc.permissions)

			if tc.shouldErr {
				suite.Error(err)
				if tc.checkError != nil {
					tc.checkError(err)
				}
				suite.Nil(invalidPerms)
			} else {
				suite.NoError(err)
				suite.Equal(tc.expectedInvalid, invalidPerms)
			}
		})
	}
}

// TestIsResourceServerDeclarative tests that database store always returns false
func (suite *ResourceStoreTestSuite) TestIsResourceServerDeclarative() {
	testCases := []struct {
		name       string
		resourceID string
		expected   bool
	}{
		{
			name:       "DatabaseStoreIsNotDeclarative_RS1",
			resourceID: "rs1",
			expected:   false,
		},
		{
			name:       "DatabaseStoreIsNotDeclarative_RS2",
			resourceID: "rs2",
			expected:   false,
		},
		{
			name:       "DatabaseStoreIsNotDeclarative_AnyID",
			resourceID: "any-resource-server-id",
			expected:   false,
		},
		{
			name:       "DatabaseStoreIsNotDeclarative_EmptyID",
			resourceID: "",
			expected:   false,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			result := suite.store.IsResourceServerDeclarative(tc.resourceID)
			suite.Equal(tc.expected, result)
		})
	}
}

// TestBuildPropertiesJSONFunction tests the buildPropertiesJSON function
// This is a helper function test that constructs JSON properties from a ResourceServer
func (suite *ResourceStoreTestSuite) TestBuildPropertiesJSONFunction() {
	testCases := []struct {
		name           string
		resourceServer ResourceServer
	}{
		{
			name: "Success_SlashDelimiter",
			resourceServer: ResourceServer{
				Delimiter: "/",
			},
		},
		{
			name: "Success_ColonDelimiter",
			resourceServer: ResourceServer{
				Delimiter: ":",
			},
		},
		{
			name: "Success_DotDelimiter",
			resourceServer: ResourceServer{
				Delimiter: ".",
			},
		},
		{
			name: "Success_EmptyDelimiter",
			resourceServer: ResourceServer{
				Delimiter: "",
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			result := buildPropertiesJSON(tc.resourceServer)
			suite.NotNil(result)
		})
	}
}
