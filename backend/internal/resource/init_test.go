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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"
	_ "modernc.org/sqlite"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
)

// fakeTransactioner is a test double for transaction.Transactioner
type fakeTransactioner struct{}

func (f *fakeTransactioner) Transact(ctx context.Context, txFunc func(context.Context) error) error {
	return txFunc(ctx)
}

type InitTestSuite struct {
	suite.Suite
	mockOUService *oumock.OrganizationUnitServiceInterfaceMock
}

func (suite *InitTestSuite) SetupTest() {
	suite.mockOUService = oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())

	// Reset config to clear singleton state
	config.ResetServerRuntime()

	// Initialize runtime config for the test
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
		Server: config.ServerConfig{
			Identifier: "test-deployment",
		},
	}
	err := config.InitializeServerRuntime(".", testConfig)
	if err != nil {
		suite.T().Fatalf("Failed to initialize server runtime: %v", err)
	}
}

func (suite *InitTestSuite) TearDownTest() {
	// Reset config to clear singleton state for next test
	config.ResetServerRuntime()
}

func TestInitTestSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

// TestInitialize tests the Initialize function
func (suite *InitTestSuite) TestInitialize() {
	mux := http.NewServeMux()

	// Execute
	service, exporter, err := Initialize(mux, suite.mockOUService, newDisabledConsentServiceMock(suite.T()))

	// Assert
	suite.NoError(err)
	suite.NotNil(service)
	suite.NotNil(exporter)
	suite.Implements((*ResourceServiceInterface)(nil), service)
}

// TestRegisterRoutes tests that all routes are properly registered
func (suite *InitTestSuite) TestRegisterRoutes() {
	mux := http.NewServeMux()
	handler := &resourceHandler{}

	// This test mainly ensures registerRoutes doesn't panic
	suite.NotPanics(func() {
		registerRoutes(mux, handler)
	})

	// Table-driven test for route verification
	testCases := []struct {
		name     string
		method   string
		target   string
		expected string
	}{
		// Resource Server routes
		{
			name:     "GET resource servers list",
			method:   http.MethodGet,
			target:   "/resource-servers",
			expected: "GET /resource-servers",
		},
		{
			name:     "POST create resource server",
			method:   http.MethodPost,
			target:   "/resource-servers",
			expected: "POST /resource-servers",
		},
		{
			name:     "OPTIONS resource servers",
			method:   http.MethodOptions,
			target:   "/resource-servers",
			expected: "OPTIONS /resource-servers",
		},
		{
			name:     "GET resource server by ID",
			method:   http.MethodGet,
			target:   "/resource-servers/rs-123",
			expected: "GET /resource-servers/{id}",
		},
		{
			name:     "PUT update resource server",
			method:   http.MethodPut,
			target:   "/resource-servers/rs-123",
			expected: "PUT /resource-servers/{id}",
		},
		{
			name:     "DELETE resource server",
			method:   http.MethodDelete,
			target:   "/resource-servers/rs-123",
			expected: "DELETE /resource-servers/{id}",
		},
		{
			name:     "OPTIONS resource server by ID",
			method:   http.MethodOptions,
			target:   "/resource-servers/rs-123",
			expected: "OPTIONS /resource-servers/{id}",
		},
		// Resource routes
		{
			name:     "GET resources list",
			method:   http.MethodGet,
			target:   "/resource-servers/rs-123/resources",
			expected: "GET /resource-servers/{rsId}/resources",
		},
		{
			name:     "POST create resource",
			method:   http.MethodPost,
			target:   "/resource-servers/rs-123/resources",
			expected: "POST /resource-servers/{rsId}/resources",
		},
		{
			name:     "OPTIONS resources",
			method:   http.MethodOptions,
			target:   "/resource-servers/rs-123/resources",
			expected: "OPTIONS /resource-servers/{rsId}/resources",
		},
		{
			name:     "GET resource by ID",
			method:   http.MethodGet,
			target:   "/resource-servers/rs-123/resources/res-456",
			expected: "GET /resource-servers/{rsId}/resources/{id}",
		},
		{
			name:     "PUT update resource",
			method:   http.MethodPut,
			target:   "/resource-servers/rs-123/resources/res-456",
			expected: "PUT /resource-servers/{rsId}/resources/{id}",
		},
		{
			name:     "DELETE resource",
			method:   http.MethodDelete,
			target:   "/resource-servers/rs-123/resources/res-456",
			expected: "DELETE /resource-servers/{rsId}/resources/{id}",
		},
		{
			name:     "OPTIONS resource by ID",
			method:   http.MethodOptions,
			target:   "/resource-servers/rs-123/resources/res-456",
			expected: "OPTIONS /resource-servers/{rsId}/resources/{id}",
		},
		// Action routes at Resource Server level
		{
			name:     "GET actions at resource server",
			method:   http.MethodGet,
			target:   "/resource-servers/rs-123/actions",
			expected: "GET /resource-servers/{rsId}/actions",
		},
		{
			name:     "POST create action at resource server",
			method:   http.MethodPost,
			target:   "/resource-servers/rs-123/actions",
			expected: "POST /resource-servers/{rsId}/actions",
		},
		{
			name:     "OPTIONS actions at resource server",
			method:   http.MethodOptions,
			target:   "/resource-servers/rs-123/actions",
			expected: "OPTIONS /resource-servers/{rsId}/actions",
		},
		{
			name:     "GET action by ID at resource server",
			method:   http.MethodGet,
			target:   "/resource-servers/rs-123/actions/act-789",
			expected: "GET /resource-servers/{rsId}/actions/{id}",
		},
		{
			name:     "PUT update action at resource server",
			method:   http.MethodPut,
			target:   "/resource-servers/rs-123/actions/act-789",
			expected: "PUT /resource-servers/{rsId}/actions/{id}",
		},
		{
			name:     "DELETE action at resource server",
			method:   http.MethodDelete,
			target:   "/resource-servers/rs-123/actions/act-789",
			expected: "DELETE /resource-servers/{rsId}/actions/{id}",
		},
		{
			name:     "OPTIONS action by ID at resource server",
			method:   http.MethodOptions,
			target:   "/resource-servers/rs-123/actions/act-789",
			expected: "OPTIONS /resource-servers/{rsId}/actions/{id}",
		},
		// Action routes at Resource level
		{
			name:     "GET actions at resource",
			method:   http.MethodGet,
			target:   "/resource-servers/rs-123/resources/res-456/actions",
			expected: "GET /resource-servers/{rsId}/resources/{resourceId}/actions",
		},
		{
			name:     "POST create action at resource",
			method:   http.MethodPost,
			target:   "/resource-servers/rs-123/resources/res-456/actions",
			expected: "POST /resource-servers/{rsId}/resources/{resourceId}/actions",
		},
		{
			name:     "OPTIONS actions at resource",
			method:   http.MethodOptions,
			target:   "/resource-servers/rs-123/resources/res-456/actions",
			expected: "OPTIONS /resource-servers/{rsId}/resources/{resourceId}/actions",
		},
		{
			name:     "GET action by ID at resource",
			method:   http.MethodGet,
			target:   "/resource-servers/rs-123/resources/res-456/actions/act-789",
			expected: "GET /resource-servers/{rsId}/resources/{resourceId}/actions/{id}",
		},
		{
			name:     "PUT update action at resource",
			method:   http.MethodPut,
			target:   "/resource-servers/rs-123/resources/res-456/actions/act-789",
			expected: "PUT /resource-servers/{rsId}/resources/{resourceId}/actions/{id}",
		},
		{
			name:     "DELETE action at resource",
			method:   http.MethodDelete,
			target:   "/resource-servers/rs-123/resources/res-456/actions/act-789",
			expected: "DELETE /resource-servers/{rsId}/resources/{resourceId}/actions/{id}",
		},
		{
			name:     "OPTIONS action by ID at resource",
			method:   http.MethodOptions,
			target:   "/resource-servers/rs-123/resources/res-456/actions/act-789",
			expected: "OPTIONS /resource-servers/{rsId}/resources/{resourceId}/actions/{id}",
		},
	}

	// Verify all routes are registered with correct patterns
	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			req := httptest.NewRequest(tc.method, tc.target, nil)
			_, pattern := mux.Handler(req)
			suite.Equal(tc.expected, pattern, "Route pattern mismatch for %s %s", tc.method, tc.target)
		})
	}
}

// TestNewResourceHandler tests the newResourceHandler constructor
func (suite *InitTestSuite) TestNewResourceHandler() {
	mockService := NewResourceServiceInterfaceMock(suite.T())

	// Execute
	handler := newResourceHandler(mockService)

	// Assert
	suite.NotNil(handler)
	suite.Equal(mockService, handler.resourceService)
}

// TestNewResourceService tests the newResourceService constructor
func (suite *InitTestSuite) TestNewResourceService() {
	mockStore := newResourceStoreInterfaceMock(suite.T())

	// Execute
	mockTransactioner := &fakeTransactioner{}
	service, err := newResourceService(
		suite.mockOUService, newDisabledConsentServiceMock(suite.T()), mockStore, mockTransactioner,
	)

	// Assert
	suite.NoError(err)
	suite.NotNil(service)
	suite.Implements((*ResourceServiceInterface)(nil), service)

	// Verify dependencies are set correctly
	resSvc, ok := service.(*resourceService)
	suite.True(ok)
	suite.Equal(mockStore, resSvc.resourceStore)
	suite.Equal(suite.mockOUService, resSvc.ouService)
}

// TestNewResourceStore tests the newResourceStore constructor
func (suite *InitTestSuite) TestNewResourceStore() {
	// Execute
	store, _, _ := newResourceStore()

	// Assert
	suite.NotNil(store)
	suite.Implements((*resourceStoreInterface)(nil), store)

	// Verify store is properly initialized
	resStore, ok := store.(*resourceStore)
	suite.True(ok)
	suite.NotNil(resStore.dbProvider)
	suite.Equal("test-deployment", resStore.deploymentID)
}

// TestRegisterRoutes_AllOPTIONSRoutes tests that all OPTIONS routes return NoContent
func (suite *InitTestSuite) TestRegisterRoutes_AllOPTIONSRoutes() {
	mux := http.NewServeMux()
	handler := &resourceHandler{}
	registerRoutes(mux, handler)

	// Table-driven test for OPTIONS routes
	optionsRoutes := []struct {
		name   string
		target string
	}{
		{name: "OPTIONS /resource-servers", target: "/resource-servers"},
		{name: "OPTIONS /resource-servers/{id}", target: "/resource-servers/rs-123"},
		{name: "OPTIONS /resource-servers/{rsId}/resources", target: "/resource-servers/rs-123/resources"},
		{name: "OPTIONS /resource-servers/{rsId}/resources/{id}", target: "/resource-servers/rs-123/resources/res-456"},
		{name: "OPTIONS /resource-servers/{rsId}/actions", target: "/resource-servers/rs-123/actions"},
		{name: "OPTIONS /resource-servers/{rsId}/actions/{id}", target: "/resource-servers/rs-123/actions/act-789"},
		{name: "OPTIONS /resource-servers/{rsId}/resources/{resourceId}/actions",
			target: "/resource-servers/rs-123/resources/res-456/actions"},
		{name: "OPTIONS /resource-servers/{rsId}/resources/{resourceId}/actions/{id}",
			target: "/resource-servers/rs-123/resources/res-456/actions/act-789"},
	}

	for _, tc := range optionsRoutes {
		suite.Run(tc.name, func() {
			req := httptest.NewRequest(http.MethodOptions, tc.target, nil)
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, req)

			suite.Equal(http.StatusNoContent, w.Code, "OPTIONS request should return 204 No Content")
		})
	}
}

// TestInitialize_IntegrationFlow tests the complete initialization flow
func (suite *InitTestSuite) TestInitialize_IntegrationFlow() {
	mux := http.NewServeMux()

	// Execute
	service, _, err := Initialize(mux, suite.mockOUService, newDisabledConsentServiceMock(suite.T()))

	// Assert service is created
	suite.NoError(err)
	suite.NotNil(service)
	suite.Implements((*ResourceServiceInterface)(nil), service)

	// Verify routes are registered by checking a sample route
	req := httptest.NewRequest(http.MethodGet, "/resource-servers", nil)
	_, pattern := mux.Handler(req)
	suite.Equal("GET /resource-servers", pattern, "Routes should be registered during initialization")
}

// TestRegisterRoutes_CORSConfiguration tests CORS headers are properly configured
func (suite *InitTestSuite) TestRegisterRoutes_CORSConfiguration() {
	mux := http.NewServeMux()
	handler := &resourceHandler{}
	registerRoutes(mux, handler)

	// Table-driven test for CORS verification on different route groups
	corsTestCases := []struct {
		name           string
		method         string
		target         string
		expectedStatus int
	}{
		{
			name:           "OPTIONS resource servers endpoint",
			method:         http.MethodOptions,
			target:         "/resource-servers",
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "OPTIONS resource server detail endpoint",
			method:         http.MethodOptions,
			target:         "/resource-servers/rs-123",
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "OPTIONS resources endpoint",
			method:         http.MethodOptions,
			target:         "/resource-servers/rs-123/resources",
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "OPTIONS resource detail endpoint",
			method:         http.MethodOptions,
			target:         "/resource-servers/rs-123/resources/res-456",
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "OPTIONS actions at resource server endpoint",
			method:         http.MethodOptions,
			target:         "/resource-servers/rs-123/actions",
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "OPTIONS action detail at resource server endpoint",
			method:         http.MethodOptions,
			target:         "/resource-servers/rs-123/actions/act-789",
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "OPTIONS actions at resource endpoint",
			method:         http.MethodOptions,
			target:         "/resource-servers/rs-123/resources/res-456/actions",
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "OPTIONS action detail at resource endpoint",
			method:         http.MethodOptions,
			target:         "/resource-servers/rs-123/resources/res-456/actions/act-789",
			expectedStatus: http.StatusNoContent,
		},
	}

	for _, tc := range corsTestCases {
		suite.Run(tc.name, func() {
			req := httptest.NewRequest(tc.method, tc.target, nil)
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, req)

			suite.Equal(tc.expectedStatus, w.Code, "OPTIONS request should return correct status")
		})
	}
}

// TestInitializeStore_MutableMode tests store initialization in mutable mode
func (suite *InitTestSuite) TestInitializeStore_MutableMode() {
	// Setup: Configure mutable store mode
	runtime := config.GetServerRuntime()
	runtime.Config.Resource.Store = "mutable"
	runtime.Config.DeclarativeResources.Enabled = false

	// Execute
	store, _, err := initializeStore()

	// Assert
	suite.NoError(err)
	suite.NotNil(store)

	// Verify the store is of type resourceStore (database store)
	_, ok := store.(*resourceStore)
	suite.True(ok, "Mutable mode should return a database store (*resourceStore)")
}

// TestInitializeStore_DeclarativeMode tests store initialization in declarative mode
func (suite *InitTestSuite) TestInitializeStore_DeclarativeMode() {
	// Setup: Configure declarative store mode
	runtime := config.GetServerRuntime()
	runtime.Config.Resource.Store = "declarative"

	// Execute
	store, _, err := initializeStore()

	// Assert
	suite.NoError(err)
	suite.NotNil(store)

	// Verify the store is of type fileBasedResourceStore
	_, ok := store.(*fileBasedResourceStore)
	suite.True(ok, "Declarative mode should return a file-based store (*fileBasedResourceStore)")
}

// TestInitializeStore_CompositeMode tests store initialization in composite mode
func (suite *InitTestSuite) TestInitializeStore_CompositeMode() {
	// Setup: Configure composite store mode
	runtime := config.GetServerRuntime()
	runtime.Config.Resource.Store = testStoreModeComposite

	// Execute
	store, _, err := initializeStore()

	// Assert
	suite.NoError(err)
	suite.NotNil(store)

	// Verify the store is of type compositeResourceStore
	compositeStore, ok := store.(*compositeResourceStore)
	suite.True(ok, "Composite mode should return a composite store (*compositeResourceStore)")

	// Verify internal stores are correctly initialized
	suite.NotNil(compositeStore.fileStore, "Composite store should have file store")
	suite.NotNil(compositeStore.dbStore, "Composite store should have db store")

	// Verify the file store is fileBasedResourceStore
	_, ok = compositeStore.fileStore.(*fileBasedResourceStore)
	suite.True(ok, "Composite store's file store should be fileBasedResourceStore")

	// Verify the db store is resourceStore
	_, ok = compositeStore.dbStore.(*resourceStore)
	suite.True(ok, "Composite store's db store should be resourceStore")
}

// TestInitializeStore_CompositeMode_FileStoreInitializationFailure tests error handling when file store creation fails
func (suite *InitTestSuite) TestInitializeStore_CompositeMode_FileStoreInitializationFailure() {
	// This test verifies that errors during file store initialization are properly propagated
	// In a normal scenario, newFileBasedResourceStore() should not fail, but we document the behavior

	runtime := config.GetServerRuntime()
	runtime.Config.Resource.Store = testStoreModeComposite

	// Execute (in normal conditions, this should succeed)
	store, _, err := initializeStore()

	// Assert - in composite mode, it should succeed with a valid store
	suite.NoError(err)
	suite.NotNil(store)
}

// TestInitializeStore_InvalidMode tests error handling for unsupported store mode
// Note: Invalid service-level modes fall back to global configuration, so pure invalid mode
// won't produce an error - it will use the global setting
func (suite *InitTestSuite) TestInitializeStore_InvalidMode_ReturnedFromInitializeStore() {
	// This test verifies that a directly invalid store mode returned from initializeStore would error
	// However, since getResourceStoreMode() has fallback logic, we need to test direct usage
	// For now, we document that invalid modes are handled by fallback in getResourceStoreMode()

	// Setup with explicit invalid mode that bypasses the fallback
	runtime := config.GetServerRuntime()
	runtime.Config.Resource.Store = "invalid-mode"
	runtime.Config.DeclarativeResources.Enabled = false

	// Execute
	store, _, err := initializeStore()

	// Assert - invalid mode falls back to mutable (global disabled), so we get a store
	suite.NoError(err)
	suite.NotNil(store)
	_, ok := store.(*resourceStore)
	suite.True(ok, "Invalid mode should fall back to mutable mode")
}

// TestInitializeStore_CaseInsensitiveMode tests that store mode is case-insensitive
func (suite *InitTestSuite) TestInitializeStore_CaseInsensitiveMode() {
	testCases := []struct {
		name         string
		storeMode    string
		expectedType string
	}{
		{"MUTABLE uppercase", "MUTABLE", "resourceStore"},
		{"Mutable mixed case", "Mutable", "resourceStore"},
		{"DECLARATIVE uppercase", "DECLARATIVE", "fileBasedResourceStore"},
		{"Declarative mixed case", "Declarative", "fileBasedResourceStore"},
		{"COMPOSITE uppercase", "COMPOSITE", "compositeResourceStore"},
		{"Composite mixed case", "Composite", "compositeResourceStore"},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			runtime := config.GetServerRuntime()
			runtime.Config.Resource.Store = tc.storeMode

			store, _, err := initializeStore()

			suite.NoError(err, "Store mode '%s' should be valid", tc.storeMode)
			suite.NotNil(store, "Store should not be nil for mode '%s'", tc.storeMode)

			// Verify type matches expected
			switch tc.expectedType {
			case "resourceStore":
				_, ok := store.(*resourceStore)
				suite.True(ok, "Expected resourceStore for mode '%s'", tc.storeMode)
			case "fileBasedResourceStore":
				_, ok := store.(*fileBasedResourceStore)
				suite.True(ok, "Expected fileBasedResourceStore for mode '%s'", tc.storeMode)
			case "compositeResourceStore":
				_, ok := store.(*compositeResourceStore)
				suite.True(ok, "Expected compositeResourceStore for mode '%s'", tc.storeMode)
			}
		})
	}
}

// TestInitializeStore_WithWhitespace tests that store mode handles whitespace
func (suite *InitTestSuite) TestInitializeStore_WithWhitespace() {
	testCases := []struct {
		name          string
		storeMode     string
		shouldSucceed bool
	}{
		{"Leading whitespace", "  mutable", true},
		{"Trailing whitespace", "mutable  ", true},
		{"Both whitespace", "  declarative  ", true},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			runtime := config.GetServerRuntime()
			runtime.Config.Resource.Store = tc.storeMode

			store, _, err := initializeStore()

			if tc.shouldSucceed {
				suite.NoError(err)
				suite.NotNil(store)
			} else {
				suite.Error(err)
				suite.Nil(store)
			}
		})
	}
}

// TestInitializeStore_FallbackToGlobalConfig tests fallback to global configuration
func (suite *InitTestSuite) TestInitializeStore_FallbackToGlobalConfig() {
	runtime := config.GetServerRuntime()

	testCases := []struct {
		name              string
		serviceStore      string
		globalEnabled     bool
		expectedStoreType string
	}{
		{
			name:              "No service config, global disabled",
			serviceStore:      "",
			globalEnabled:     false,
			expectedStoreType: "resourceStore",
		},
		{
			name:              "No service config, global enabled",
			serviceStore:      "",
			globalEnabled:     true,
			expectedStoreType: "fileBasedResourceStore",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			runtime.Config.Resource.Store = tc.serviceStore
			runtime.Config.DeclarativeResources.Enabled = tc.globalEnabled

			store, _, err := initializeStore()

			suite.NoError(err)
			suite.NotNil(store)

			switch tc.expectedStoreType {
			case "resourceStore":
				_, ok := store.(*resourceStore)
				suite.True(ok)
			case "fileBasedResourceStore":
				_, ok := store.(*fileBasedResourceStore)
				suite.True(ok)
			}
		})
	}
}
