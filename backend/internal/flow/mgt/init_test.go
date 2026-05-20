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
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"
	yaml "gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/cors"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

// mockTransactioner is a simple no-op transactioner for tests.
type mockTransactioner struct{}

func (m *mockTransactioner) Transact(ctx context.Context, operation func(txCtx context.Context) error) error {
	return operation(ctx)
}

// setupMockDBProvider sets up a mock DB provider that returns a no-op transactioner.
func setupMockDBProvider() func() {
	mockProvider := &providermock.DBProviderInterfaceMock{}
	mockProvider.On("GetConfigDBTransactioner").Return(&mockTransactioner{}, nil)
	originalGetDBProvider := getDBProvider
	getDBProvider = func() provider.DBProviderInterface { return mockProvider }
	return func() { getDBProvider = originalGetDBProvider }
}

const (
	testFlowIDInit = "test-flow-id"
)

type InitTestSuite struct {
	suite.Suite
	mockService *FlowMgtServiceInterfaceMock
}

func (s *InitTestSuite) SetupTest() {
	s.mockService = NewFlowMgtServiceInterfaceMock(s.T())

	var allowedOrigins cors.OriginEntries
	s.Require().NoError(yaml.Unmarshal([]byte(`
- https://example.com
- https://localhost:3000
`), &allowedOrigins))
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
		},
		Server: config.ServerConfig{
			Identifier: "test-deployment",
		},
		CORS: config.CORSConfig{AllowedOrigins: allowedOrigins},
	}
	s.Require().NoError(cors.InitializeMatcher(testConfig.CORS.AllowedOrigins))
	_ = config.InitializeServerRuntime("test", testConfig)
}

func (s *InitTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func TestInitTestSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

func (s *InitTestSuite) TestRegisterRoutes_AllRoutesRegistered() {
	mux := http.NewServeMux()
	handler := newFlowMgtHandler(s.mockService)
	registerRoutes(mux, handler)

	// Test OPTIONS endpoints which don't require service calls
	testCases := []struct {
		name string
		path string
	}{
		{"OPTIONS /flows", "/flows"},
		{"OPTIONS /flows/{flowId}", "/flows/test-id"},
		{"OPTIONS /flows/{flowId}/versions", "/flows/test-id/versions"},
		{"OPTIONS /flows/{flowId}/versions/{version}", "/flows/test-id/versions/1"},
		{"OPTIONS /flows/{flowId}/restore", "/flows/test-id/restore"},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			req := httptest.NewRequest(http.MethodOptions, tc.path, nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			s.Equal(http.StatusNoContent, w.Code, "Route %s should be registered", tc.path)
		})
	}
}

func (s *InitTestSuite) TestRegisterRoutes_CORSHeadersConfigured() {
	mux := http.NewServeMux()
	handler := newFlowMgtHandler(s.mockService)

	registerRoutes(mux, handler)

	testCases := []struct {
		name                   string
		method                 string
		path                   string
		expectedAllowedMethods string
	}{
		{
			name:                   "CORS for /flows",
			method:                 http.MethodOptions,
			path:                   "/flows",
			expectedAllowedMethods: "GET, POST",
		},
		{
			name:                   "CORS for /flows/{flowId}",
			method:                 http.MethodOptions,
			path:                   "/flows/" + testFlowIDInit,
			expectedAllowedMethods: "GET, PUT, DELETE",
		},
		{
			name:                   "CORS for /flows/{flowId}/versions",
			method:                 http.MethodOptions,
			path:                   "/flows/" + testFlowIDInit + "/versions",
			expectedAllowedMethods: "GET",
		},
		{
			name:                   "CORS for /flows/{flowId}/versions/{version}",
			method:                 http.MethodOptions,
			path:                   "/flows/" + testFlowIDInit + "/versions/1",
			expectedAllowedMethods: "GET",
		},
		{
			name:                   "CORS for /flows/{flowId}/restore",
			method:                 http.MethodOptions,
			path:                   "/flows/" + testFlowIDInit + "/restore",
			expectedAllowedMethods: "POST",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// Allow-Methods/Allow-Headers are preflight-only response headers
			// per the Fetch spec; the request must carry
			// Access-Control-Request-Method to elicit them.
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("Origin", "https://example.com")
			req.Header.Set("Access-Control-Request-Method", "GET")
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, req)

			s.Equal(http.StatusNoContent, w.Code)
			s.Contains(w.Header().Get("Access-Control-Allow-Methods"), tc.expectedAllowedMethods)
			s.Contains(w.Header().Get("Access-Control-Allow-Headers"), "Content-Type")
			s.Contains(w.Header().Get("Access-Control-Allow-Headers"), "Authorization")
			s.Equal("true", w.Header().Get("Access-Control-Allow-Credentials"))
		})
	}
}

func (s *InitTestSuite) TestRegisterRoutes_OPTIONSHandlers() {
	mux := http.NewServeMux()
	handler := newFlowMgtHandler(s.mockService)

	registerRoutes(mux, handler)

	optionsPaths := []string{
		"/flows",
		"/flows/" + testFlowIDInit,
		"/flows/" + testFlowIDInit + "/versions",
		"/flows/" + testFlowIDInit + "/versions/1",
		"/flows/" + testFlowIDInit + "/restore",
	}

	for _, path := range optionsPaths {
		s.Run("OPTIONS "+path, func() {
			req := httptest.NewRequest(http.MethodOptions, path, nil)
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, req)

			s.Equal(http.StatusNoContent, w.Code, "OPTIONS request should return 204")
			s.Empty(w.Body.String(), "OPTIONS response should have empty body")
		})
	}
}

func (s *InitTestSuite) TestRegisterRoutes_WithNilHandler() {
	mux := http.NewServeMux()

	// Routes can be registered with nil handler, but calling them would fail
	s.NotPanics(func() {
		registerRoutes(mux, nil)
	}, "Should not panic when handler is nil during registration")
}

func (s *InitTestSuite) TestRegisterRoutes_PreflightRequests() {
	mux := http.NewServeMux()
	handler := newFlowMgtHandler(s.mockService)

	registerRoutes(mux, handler)

	testCases := []struct {
		name           string
		path           string
		origin         string
		requestMethod  string
		requestHeaders string
	}{
		{
			name:           "Preflight for POST /flows",
			path:           "/flows",
			origin:         "https://example.com",
			requestMethod:  "POST",
			requestHeaders: "Content-Type",
		},
		{
			name:           "Preflight for PUT /flows/{flowId}",
			path:           "/flows/" + testFlowIDInit,
			origin:         "https://example.com",
			requestMethod:  "PUT",
			requestHeaders: "Authorization",
		},
		{
			name:           "Preflight for DELETE /flows/{flowId}",
			path:           "/flows/" + testFlowIDInit,
			origin:         "https://example.com",
			requestMethod:  "DELETE",
			requestHeaders: "Content-Type, Authorization",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			req := httptest.NewRequest(http.MethodOptions, tc.path, nil)
			req.Header.Set("Origin", tc.origin)
			req.Header.Set("Access-Control-Request-Method", tc.requestMethod)
			req.Header.Set("Access-Control-Request-Headers", tc.requestHeaders)
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, req)

			s.Equal(http.StatusNoContent, w.Code)
			s.NotEmpty(w.Header().Get("Access-Control-Allow-Origin"))
			s.NotEmpty(w.Header().Get("Access-Control-Allow-Methods"))
			s.NotEmpty(w.Header().Get("Access-Control-Allow-Headers"))
		})
	}
}

// Store Mode Detection tests
func (s *InitTestSuite) TestGetFlowStoreMode_Mutable() {
	testConfig := &config.Config{
		Flow: config.FlowConfig{
			Store: string(serverconst.StoreModeMutable),
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("test", testConfig)
	defer config.ResetServerRuntime()

	mode := getFlowStoreMode()

	s.Equal(serverconst.StoreModeMutable, mode)
}

func (s *InitTestSuite) TestGetFlowStoreMode_Declarative() {
	testConfig := &config.Config{
		Flow: config.FlowConfig{
			Store: string(serverconst.StoreModeDeclarative),
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("test", testConfig)
	defer config.ResetServerRuntime()

	mode := getFlowStoreMode()

	s.Equal(serverconst.StoreModeDeclarative, mode)
}

func (s *InitTestSuite) TestGetFlowStoreMode_Composite() {
	testConfig := &config.Config{
		Flow: config.FlowConfig{
			Store: string(serverconst.StoreModeComposite),
		},
		DeclarativeResources: config.DeclarativeResources{
			Enabled: true,
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("test", testConfig)
	defer config.ResetServerRuntime()

	mode := getFlowStoreMode()

	s.Equal(serverconst.StoreModeComposite, mode)
}

func (s *InitTestSuite) TestGetFlowStoreMode_DefaultMutable() {
	testConfig := &config.Config{
		Flow: config.FlowConfig{
			Store: "",
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("test", testConfig)
	defer config.ResetServerRuntime()

	mode := getFlowStoreMode()

	s.Equal(serverconst.StoreModeMutable, mode)
}

func (s *InitTestSuite) TestIsCompositeModeEnabled_Enabled() {
	testConfig := &config.Config{
		Flow: config.FlowConfig{
			Store: string(serverconst.StoreModeComposite),
		},
		DeclarativeResources: config.DeclarativeResources{
			Enabled: true,
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("test", testConfig)
	defer config.ResetServerRuntime()

	enabled := isCompositeModeEnabled()

	s.True(enabled)
}

func (s *InitTestSuite) TestIsCompositeModeEnabled_Disabled() {
	testConfig := &config.Config{
		Flow: config.FlowConfig{
			Store: string(serverconst.StoreModeMutable),
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("test", testConfig)
	defer config.ResetServerRuntime()

	enabled := isCompositeModeEnabled()

	s.False(enabled)
}

// Additional tests for init.go - store mode detection and initialization

// Test getFlowStoreMode with invalid store mode (should fall back to mutable)
func (s *InitTestSuite) TestGetFlowStoreMode_InvalidMode() {
	testConfig := &config.Config{
		Flow: config.FlowConfig{
			Store: "invalid-mode",
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("test", testConfig)
	defer config.ResetServerRuntime()

	mode := getFlowStoreMode()

	// Should default to mutable when invalid mode is provided
	s.Equal(serverconst.StoreModeMutable, mode)
}

// Test getFlowStoreMode with whitespace in mode
func (s *InitTestSuite) TestGetFlowStoreMode_WithWhitespace() {
	testConfig := &config.Config{
		Flow: config.FlowConfig{
			Store: "  composite  ",
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("test", testConfig)
	defer config.ResetServerRuntime()

	mode := getFlowStoreMode()

	// Should trim whitespace and recognize composite
	s.Equal(serverconst.StoreModeComposite, mode)
}

// Test getFlowStoreMode with mixed case
func (s *InitTestSuite) TestGetFlowStoreMode_MixedCase() {
	testConfig := &config.Config{
		Flow: config.FlowConfig{
			Store: "Declarative",
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("test", testConfig)
	defer config.ResetServerRuntime()

	mode := getFlowStoreMode()

	// Should convert to lowercase and recognize declarative
	s.Equal(serverconst.StoreModeDeclarative, mode)
}

// Test getFlowStoreMode fallback to global DeclarativeResources.Enabled=true
func (s *InitTestSuite) TestGetFlowStoreMode_FallbackToGlobalDeclarative() {
	testConfig := &config.Config{
		Flow: config.FlowConfig{
			Store: "", // Not explicitly set
		},
		DeclarativeResources: config.DeclarativeResources{
			Enabled: true,
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("test", testConfig)
	defer config.ResetServerRuntime()

	mode := getFlowStoreMode()

	// Should fall back to declarative when global setting is enabled
	s.Equal(serverconst.StoreModeDeclarative, mode)
}

// Test getFlowStoreMode fallback to global DeclarativeResources.Enabled=false
func (s *InitTestSuite) TestGetFlowStoreMode_FallbackToGlobalMutable() {
	testConfig := &config.Config{
		Flow: config.FlowConfig{
			Store: "", // Not explicitly set
		},
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("test", testConfig)
	defer config.ResetServerRuntime()

	mode := getFlowStoreMode()

	// Should default to mutable when global setting is disabled
	s.Equal(serverconst.StoreModeMutable, mode)
}

// Test getFlowStoreMode - explicit setting overrides global
func (s *InitTestSuite) TestGetFlowStoreMode_ExplicitOverridesGlobal() {
	testConfig := &config.Config{
		Flow: config.FlowConfig{
			Store: string(serverconst.StoreModeMutable), // Explicitly set to mutable
		},
		DeclarativeResources: config.DeclarativeResources{
			Enabled: true, // Global says declarative
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("test", testConfig)
	defer config.ResetServerRuntime()

	mode := getFlowStoreMode()

	// Explicit setting should override global
	s.Equal(serverconst.StoreModeMutable, mode)
}

// Test isCompositeModeEnabled - true case
func (s *InitTestSuite) TestIsCompositeModeEnabled_True() {
	testConfig := &config.Config{
		Flow: config.FlowConfig{
			Store: string(serverconst.StoreModeComposite),
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("test", testConfig)
	defer config.ResetServerRuntime()

	enabled := isCompositeModeEnabled()

	s.True(enabled)
}

// Test isCompositeModeEnabled - false for mutable
func (s *InitTestSuite) TestIsCompositeModeEnabled_FalseForMutable() {
	testConfig := &config.Config{
		Flow: config.FlowConfig{
			Store: string(serverconst.StoreModeMutable),
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("test", testConfig)
	defer config.ResetServerRuntime()

	enabled := isCompositeModeEnabled()

	s.False(enabled)
}

// Test isCompositeModeEnabled - false for declarative
func (s *InitTestSuite) TestIsCompositeModeEnabled_FalseForDeclarative() {
	testConfig := &config.Config{
		Flow: config.FlowConfig{
			Store: string(serverconst.StoreModeDeclarative),
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("test", testConfig)
	defer config.ResetServerRuntime()

	enabled := isCompositeModeEnabled()

	s.False(enabled)
}

// Test getFlowStoreMode with uppercase
func (s *InitTestSuite) TestGetFlowStoreMode_Uppercase() {
	testConfig := &config.Config{
		Flow: config.FlowConfig{
			Store: "COMPOSITE",
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("test", testConfig)
	defer config.ResetServerRuntime()

	mode := getFlowStoreMode()

	// Should handle uppercase correctly
	s.Equal(serverconst.StoreModeComposite, mode)
}

// Test getFlowStoreMode with special characters (should fall back to mutable)
func (s *InitTestSuite) TestGetFlowStoreMode_SpecialCharacters() {
	testConfig := &config.Config{
		Flow: config.FlowConfig{
			Store: "mutable@#$",
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("test", testConfig)
	defer config.ResetServerRuntime()

	mode := getFlowStoreMode()

	// Should default to mutable when invalid characters are present
	s.Equal(serverconst.StoreModeMutable, mode)
}

// Test getFlowStoreMode fallback when Flow config is nil
func (s *InitTestSuite) TestGetFlowStoreMode_NilFlowConfig() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("test", testConfig)
	defer config.ResetServerRuntime()

	mode := getFlowStoreMode()

	// Should default to mutable
	s.Equal(serverconst.StoreModeMutable, mode)
}

// Test mode resolution priority
func (s *InitTestSuite) TestGetFlowStoreMode_PriorityOrder() {
	testCases := []struct {
		name               string
		flowStore          string
		declarativeEnabled bool
		expectedMode       serverconst.StoreMode
	}{
		{
			name:               "Explicit composite overrides global",
			flowStore:          string(serverconst.StoreModeComposite),
			declarativeEnabled: false,
			expectedMode:       serverconst.StoreModeComposite,
		},
		{
			name:               "Explicit declarative overrides global",
			flowStore:          string(serverconst.StoreModeDeclarative),
			declarativeEnabled: false,
			expectedMode:       serverconst.StoreModeDeclarative,
		},
		{
			name:               "Explicit mutable overrides global declarative",
			flowStore:          string(serverconst.StoreModeMutable),
			declarativeEnabled: true,
			expectedMode:       serverconst.StoreModeMutable,
		},
		{
			name:               "Empty flow store, global declarative enabled",
			flowStore:          "",
			declarativeEnabled: true,
			expectedMode:       serverconst.StoreModeDeclarative,
		},
		{
			name:               "Empty flow store, global declarative disabled",
			flowStore:          "",
			declarativeEnabled: false,
			expectedMode:       serverconst.StoreModeMutable,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			testConfig := &config.Config{
				Flow: config.FlowConfig{
					Store: tc.flowStore,
				},
				DeclarativeResources: config.DeclarativeResources{
					Enabled: tc.declarativeEnabled,
				},
			}
			config.ResetServerRuntime()
			_ = config.InitializeServerRuntime("test", testConfig)
			defer config.ResetServerRuntime()

			mode := getFlowStoreMode()

			s.Equal(tc.expectedMode, mode)
		})
	}
}

// Test all valid store mode values
func (s *InitTestSuite) TestGetFlowStoreMode_AllValidModes() {
	validModes := []serverconst.StoreMode{
		serverconst.StoreModeMutable,
		serverconst.StoreModeDeclarative,
		serverconst.StoreModeComposite,
	}

	for _, validMode := range validModes {
		s.Run("Mode_"+string(validMode), func() {
			testConfig := &config.Config{
				Flow: config.FlowConfig{
					Store: string(validMode),
				},
			}
			config.ResetServerRuntime()
			_ = config.InitializeServerRuntime("test", testConfig)
			defer config.ResetServerRuntime()

			mode := getFlowStoreMode()

			s.Equal(validMode, mode)
		})
	}
}

// Test edge case: empty string vs nil
func (s *InitTestSuite) TestGetFlowStoreMode_EmptyStringVsNil() {
	s.Run("Empty string falls back to global", func() {
		testConfig := &config.Config{
			Flow: config.FlowConfig{
				Store: "",
			},
			DeclarativeResources: config.DeclarativeResources{
				Enabled: true,
			},
		}
		config.ResetServerRuntime()
		_ = config.InitializeServerRuntime("test", testConfig)
		defer config.ResetServerRuntime()

		mode := getFlowStoreMode()

		s.Equal(serverconst.StoreModeDeclarative, mode)
	})
}

// Test mode normalization edge cases
func (s *InitTestSuite) TestGetFlowStoreMode_NormalizationCases() {
	testCases := []struct {
		name         string
		input        string
		expectedMode serverconst.StoreMode
	}{
		{"Leading spaces", "  mutable", serverconst.StoreModeMutable},
		{"Trailing spaces", "mutable  ", serverconst.StoreModeMutable},
		{"Mixed case 1", "MuTaBlE", serverconst.StoreModeMutable},
		{"Mixed case 2", "CoMpOsItE", serverconst.StoreModeComposite},
		{"Tabs and spaces", "\t composite \t", serverconst.StoreModeComposite},
		{"All uppercase", "DECLARATIVE", serverconst.StoreModeDeclarative},
		{"Invalid normalized", "  invalid  ", serverconst.StoreModeMutable},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			testConfig := &config.Config{
				Flow: config.FlowConfig{
					Store: tc.input,
				},
			}
			config.ResetServerRuntime()
			_ = config.InitializeServerRuntime("test", testConfig)
			defer config.ResetServerRuntime()

			mode := getFlowStoreMode()

			s.Equal(tc.expectedMode, mode)
		})
	}
}

// Test initializeStore with mutable mode
func (s *InitTestSuite) TestInitializeStore_MutableMode() {
	testConfig := &config.Config{
		Flow: config.FlowConfig{
			Store: string(serverconst.StoreModeMutable),
		},
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
			Runtime: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
		Server: config.ServerConfig{
			Identifier: "test-deployment",
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("test", testConfig)
	defer config.ResetServerRuntime()

	cleanup := setupMockDBProvider()
	defer cleanup()

	store, compositeStore, _, err := initializeStore(cache.Initialize())

	s.NoError(err)
	s.NotNil(store)
	s.Nil(compositeStore, "compositeStore should be nil in mutable mode")
	// Verify it's a cacheBackedFlowStore
	_, ok := store.(*cacheBackedFlowStore)
	s.True(ok, "store should be of type *cacheBackedFlowStore")
}

// Test initializeStore with declarative mode
func (s *InitTestSuite) TestInitializeStore_DeclarativeMode() {
	testConfig := &config.Config{
		Flow: config.FlowConfig{
			Store: string(serverconst.StoreModeDeclarative),
		},
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
			Runtime: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
		Server: config.ServerConfig{
			Identifier: "test-deployment",
		},
		DeclarativeResources: config.DeclarativeResources{
			Enabled: true,
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("test", testConfig)
	defer config.ResetServerRuntime()

	store, compositeStore, _, err := initializeStore(cache.Initialize())

	// Note: err might occur if declarative resources path doesn't exist, but that's expected
	// We're testing store type initialization, not resource loading
	_ = err // Ignore error for this test
	s.NotNil(store)
	s.Nil(compositeStore, "compositeStore should be nil in declarative mode")
	// Verify it's a fileBasedStore
	_, ok := store.(*fileBasedStore)
	s.True(ok, "store should be of type *fileBasedStore")
}

// Test initializeStore with composite mode
func (s *InitTestSuite) TestInitializeStore_CompositeMode() {
	testConfig := &config.Config{
		Flow: config.FlowConfig{
			Store: string(serverconst.StoreModeComposite),
		},
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
			Runtime: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
		Server: config.ServerConfig{
			Identifier: "test-deployment",
		},
		DeclarativeResources: config.DeclarativeResources{
			Enabled: true,
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("test", testConfig)
	defer config.ResetServerRuntime()

	cleanup := setupMockDBProvider()
	defer cleanup()

	store, compositeStore, _, err := initializeStore(cache.Initialize())

	// Note: err might occur if declarative resources path doesn't exist, but that's expected
	// We're testing store type initialization, not resource loading
	_ = err // Ignore error for this test
	s.NotNil(store)
	s.NotNil(compositeStore, "compositeStore should not be nil in composite mode")
	// Verify it's a compositeFlowStore
	_, ok := store.(*compositeFlowStore)
	s.True(ok, "store should be of type *compositeFlowStore")
	// Verify compositeStore is the same instance as store
	s.Equal(compositeStore, store, "compositeStore should be the same instance as store")
}

// Test initializeStore with declarative mode handles resource loading errors
func (s *InitTestSuite) TestInitializeStore_DeclarativeMode_ResourceLoadingError() {
	testConfig := &config.Config{
		Flow: config.FlowConfig{
			Store: string(serverconst.StoreModeDeclarative),
		},
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
			Runtime: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
		Server: config.ServerConfig{
			Identifier: "test-deployment",
		},
		DeclarativeResources: config.DeclarativeResources{
			Enabled: true,
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("test", testConfig)
	defer config.ResetServerRuntime()

	store, compositeStore, _, err := initializeStore(cache.Initialize())

	// When declarative resources path doesn't exist or has issues, error is returned
	// The store and compositeStore should be nil when error occurs
	if err != nil {
		s.Nil(store)
		s.Nil(compositeStore)
	} else {
		// If no error (e.g., empty directory), store should be created
		s.NotNil(store)
		s.Nil(compositeStore)
	}
}

// Test initializeStore with composite mode handles resource loading errors
func (s *InitTestSuite) TestInitializeStore_CompositeMode_ResourceLoadingError() {
	testConfig := &config.Config{
		Flow: config.FlowConfig{
			Store: string(serverconst.StoreModeComposite),
		},
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
			Runtime: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
		Server: config.ServerConfig{
			Identifier: "test-deployment",
		},
		DeclarativeResources: config.DeclarativeResources{
			Enabled: true,
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("test", testConfig)
	defer config.ResetServerRuntime()

	cleanup := setupMockDBProvider()
	defer cleanup()

	store, compositeStore, _, err := initializeStore(cache.Initialize())

	// When declarative resources path doesn't exist or has issues, error is returned
	// The store and compositeStore should be nil when error occurs
	if err != nil {
		s.Nil(store)
		s.Nil(compositeStore)
	} else {
		// If no error (e.g., empty directory), stores should be created
		s.NotNil(store)
		s.NotNil(compositeStore)
	}
}

// Test initializeStore with default mode (fallback to mutable)
func (s *InitTestSuite) TestInitializeStore_DefaultMode() {
	testConfig := &config.Config{
		Flow: config.FlowConfig{
			Store: "", // Empty should fallback to mutable
		},
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
			Runtime: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
		Server: config.ServerConfig{
			Identifier: "test-deployment",
		},
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("test", testConfig)
	defer config.ResetServerRuntime()

	cleanup := setupMockDBProvider()
	defer cleanup()

	store, compositeStore, _, err := initializeStore(cache.Initialize())

	s.NoError(err)
	s.NotNil(store)
	s.Nil(compositeStore, "compositeStore should be nil in default (mutable) mode")
	// Verify it's a cacheBackedFlowStore
	_, ok := store.(*cacheBackedFlowStore)
	s.True(ok, "store should be of type *cacheBackedFlowStore")
}

// Test initializeStore validates store mode normalization
func (s *InitTestSuite) TestInitializeStore_ModeNormalization() {
	testConfig := &config.Config{
		Flow: config.FlowConfig{
			Store: "  COMPOSITE  ", // Should normalize to composite
		},
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
			Runtime: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
		Server: config.ServerConfig{
			Identifier: "test-deployment",
		},
		DeclarativeResources: config.DeclarativeResources{
			Enabled: true,
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("test", testConfig)
	defer config.ResetServerRuntime()

	cleanup := setupMockDBProvider()
	defer cleanup()

	store, compositeStore, _, err := initializeStore(cache.Initialize())

	// Note: err might occur if declarative resources path doesn't exist, but that's expected
	// We're testing store type initialization and mode normalization
	_ = err // Ignore error for this test
	s.NotNil(store)
	s.NotNil(compositeStore, "compositeStore should not be nil")
	// Verify it's a compositeFlowStore
	_, ok := store.(*compositeFlowStore)
	s.True(ok, "store should be of type *compositeFlowStore")
}
