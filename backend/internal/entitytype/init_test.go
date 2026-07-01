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

package entitytype

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/consentmock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

const (
	testCryptoKey = "0579f866ac7c9273580d0ff163fa01a7b2401a7ff3ddc3e3b14ae3136fa6025e"
)

type mockTransactioner struct{}

func (m *mockTransactioner) Transact(ctx context.Context, txFunc func(context.Context) error) error {
	return txFunc(ctx)
}

func testCacheManager() cache.CacheManagerInterface {
	return cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment")
}

func setupEntityTypeStoreRuntime(t *testing.T, entityTypeStore string, declarativeEnabled bool) {
	t.Helper()

	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: declarativeEnabled,
		},
		EntityType: config.EntityTypeConfig{
			Store: entityTypeStore,
		},
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
	}

	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("", testConfig)
	assert.NoError(t, err)
}

func assertMutableEntityTypeStore(t *testing.T, store entityTypeStoreInterface) {
	t.Helper()

	_, isComposite := store.(*compositeEntityTypeStore)
	assert.False(t, isComposite, "Store should not be composite in mutable mode")
	_, isFileBased := store.(*entityTypeFileBasedStore)
	assert.False(t, isFileBased, "Store should not be file-based in mutable mode")
}

// InitTestSuite contains comprehensive tests for the init.go file.
type InitTestSuite struct {
	suite.Suite
	mockOUService      *oumock.OrganizationUnitServiceInterfaceMock
	mockConsentService *consentmock.ConsentServiceInterfaceMock
	mux                *http.ServeMux
}

func TestInitTestSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

func (suite *InitTestSuite) SetupTest() {
	suite.mockOUService = oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())
	suite.mockConsentService = consentmock.NewConsentServiceInterfaceMock(suite.T())
	suite.mockConsentService.EXPECT().IsEnabled().Return(false).Maybe()
	suite.mux = http.NewServeMux()
}

func (suite *InitTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

// TestInitialize tests the Initialize function
func (suite *InitTestSuite) TestInitialize() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
	}
	err := config.InitializeServerRuntime("", testConfig)
	assert.NoError(suite.T(), err)

	service, _, err := Initialize(
		suite.mux, nil, testCacheManager(), suite.mockOUService, nil, suite.mockConsentService)
	assert.NoError(suite.T(), err)

	suite.NotNil(service)
	suite.Implements((*EntityTypeServiceInterface)(nil), service)
}

// TestRegisterRoutes_ListEndpoint tests that the list endpoint is registered
func (suite *InitTestSuite) TestRegisterRoutes_ListEndpoint() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
	}
	err := config.InitializeServerRuntime("", testConfig)
	assert.NoError(suite.T(), err)

	_, _, err = Initialize(
		suite.mux, nil, testCacheManager(), suite.mockOUService, nil, suite.mockConsentService)
	assert.NoError(suite.T(), err)

	req := httptest.NewRequest(http.MethodGet, "/user-types", nil)
	w := httptest.NewRecorder()

	suite.mux.ServeHTTP(w, req)

	suite.NotEqual(http.StatusNotFound, w.Code)
}

// TestRegisterRoutes_CreateEndpoint tests that the create endpoint is registered
func (suite *InitTestSuite) TestRegisterRoutes_CreateEndpoint() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
	}
	err := config.InitializeServerRuntime("", testConfig)
	assert.NoError(suite.T(), err)

	_, _, err = Initialize(
		suite.mux, nil, testCacheManager(), suite.mockOUService, nil, suite.mockConsentService)
	assert.NoError(suite.T(), err)

	req := httptest.NewRequest(http.MethodPost, "/user-types", nil)
	w := httptest.NewRecorder()

	suite.mux.ServeHTTP(w, req)

	suite.NotEqual(http.StatusNotFound, w.Code)
}

// TestInitialize_DBTransactionerError tests Initialize when DBTransactioner fails
func (suite *InitTestSuite) TestInitialize_DBTransactionerError() {
	// Ensure any previously initialized DB clients are closed so it forces re-initialization
	_ = provider.GetDBProviderCloser().Close()
	defer func() {
		_ = provider.GetDBProviderCloser().Close()
	}()

	// Configure with invalid DB driver to force an error during GetConfigDBTransactioner
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "invalid-db-type",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
	}

	err := config.InitializeServerRuntime("", testConfig)
	assert.NoError(suite.T(), err)

	_, _, err = Initialize(
		suite.mux, nil, testCacheManager(), suite.mockOUService, nil, suite.mockConsentService)
	assert.Error(suite.T(), err)
	if err != nil {
		assert.Contains(suite.T(), err.Error(), "failed to get config database client")
	}
}

// TestRegisterRoutes_GetByIDEndpoint tests that the get by ID endpoint is registered
func (suite *InitTestSuite) TestRegisterRoutes_GetByIDEndpoint() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
	}
	err := config.InitializeServerRuntime("", testConfig)
	assert.NoError(suite.T(), err)

	_, _, err = Initialize(
		suite.mux, nil, testCacheManager(), suite.mockOUService, nil, suite.mockConsentService)
	assert.NoError(suite.T(), err)

	req := httptest.NewRequest(http.MethodGet, "/user-types/test-id", nil)
	w := httptest.NewRecorder()

	suite.mux.ServeHTTP(w, req)

	suite.NotEqual(http.StatusNotFound, w.Code)
}

// TestRegisterRoutes_UpdateEndpoint tests that the update endpoint is registered
func (suite *InitTestSuite) TestRegisterRoutes_UpdateEndpoint() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
	}
	err := config.InitializeServerRuntime("", testConfig)
	assert.NoError(suite.T(), err)

	_, _, err = Initialize(
		suite.mux, nil, testCacheManager(), suite.mockOUService, nil, suite.mockConsentService)
	assert.NoError(suite.T(), err)

	req := httptest.NewRequest(http.MethodPut, "/user-types/test-id", nil)
	w := httptest.NewRecorder()

	suite.mux.ServeHTTP(w, req)

	suite.NotEqual(http.StatusNotFound, w.Code)
}

// TestRegisterRoutes_DeleteEndpoint tests that the delete endpoint is registered
func (suite *InitTestSuite) TestRegisterRoutes_DeleteEndpoint() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
	}
	err := config.InitializeServerRuntime("", testConfig)
	assert.NoError(suite.T(), err)

	_, _, err = Initialize(
		suite.mux, nil, testCacheManager(), suite.mockOUService, nil, suite.mockConsentService)
	assert.NoError(suite.T(), err)

	req := httptest.NewRequest(http.MethodDelete, "/user-types/test-id", nil)
	w := httptest.NewRecorder()

	suite.mux.ServeHTTP(w, req)

	suite.NotEqual(http.StatusNotFound, w.Code)
}

// TestRegisterRoutes_CORSPreflight tests that CORS preflight requests are handled
func (suite *InitTestSuite) TestRegisterRoutes_CORSPreflight() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
	}
	err := config.InitializeServerRuntime("", testConfig)
	assert.NoError(suite.T(), err)

	_, _, err = Initialize(
		suite.mux, nil, testCacheManager(), suite.mockOUService, nil, suite.mockConsentService)
	assert.NoError(suite.T(), err)

	req := httptest.NewRequest(http.MethodOptions, "/user-types", nil)
	w := httptest.NewRecorder()

	suite.mux.ServeHTTP(w, req)

	suite.Equal(http.StatusNoContent, w.Code)
}

// TestRegisterRoutes_CORSPreflightByID tests that CORS preflight requests for ID endpoint are handled
func (suite *InitTestSuite) TestRegisterRoutes_CORSPreflightByID() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
	}
	err := config.InitializeServerRuntime("", testConfig)
	assert.NoError(suite.T(), err)

	_, _, err = Initialize(
		suite.mux, nil, testCacheManager(), suite.mockOUService, nil, suite.mockConsentService)
	assert.NoError(suite.T(), err)

	req := httptest.NewRequest(http.MethodOptions, "/user-types/test-id", nil)
	w := httptest.NewRecorder()

	suite.mux.ServeHTTP(w, req)

	suite.Equal(http.StatusNoContent, w.Code)
}

// TestParseToEntityTypeDTO_ValidYAML tests parsing a valid YAML configuration
func (suite *InitTestSuite) TestParseToEntityTypeDTO_ValidYAML() {
	yamlData := `
id: "schema-001"
name: "Employee Schema"
ouId: "550e8400-e29b-41d4-a716-446655440000"
allowSelfRegistration: true
schema: |
  {
    "type": "object",
    "properties": {
      "email": {"type": "string"},
      "username": {"type": "string"}
    },
    "required": ["email", "username"]
  }
`

	schemaDTO, err := parseToEntityTypeDTO([]byte(yamlData))

	suite.NoError(err)
	suite.NotNil(schemaDTO)
	suite.Equal("schema-001", schemaDTO.ID)
	suite.Equal("Employee Schema", schemaDTO.Name)
	suite.Equal("550e8400-e29b-41d4-a716-446655440000", schemaDTO.OUID)
	suite.True(schemaDTO.AllowSelfRegistration)
	suite.NotEmpty(schemaDTO.Schema)
}

// TestParseToEntityTypeDTO_MinimalYAML tests parsing minimal YAML configuration
func (suite *InitTestSuite) TestParseToEntityTypeDTO_MinimalYAML() {
	yamlData := `
id: "minimal-schema"
name: "Minimal Schema"
ouId: "550e8400-e29b-41d4-a716-446655440000"
schema: |
  {
    "type": "object",
    "properties": {
      "email": {"type": "string"}
    }
  }
`

	schemaDTO, err := parseToEntityTypeDTO([]byte(yamlData))

	suite.NoError(err)
	suite.NotNil(schemaDTO)
	suite.Equal("minimal-schema", schemaDTO.ID)
	suite.Equal("Minimal Schema", schemaDTO.Name)
	suite.Equal("550e8400-e29b-41d4-a716-446655440000", schemaDTO.OUID)
	suite.False(schemaDTO.AllowSelfRegistration)
	suite.NotEmpty(schemaDTO.Schema)
}

// TestParseToEntityTypeDTO_InvalidYAML tests parsing invalid YAML
func (suite *InitTestSuite) TestParseToEntityTypeDTO_InvalidYAML() {
	yamlData := `
invalid yaml content
  - this is not valid
`

	schemaDTO, err := parseToEntityTypeDTO([]byte(yamlData))

	suite.Error(err)
	suite.Nil(schemaDTO)
}

// TestParseToEntityTypeDTO_ComplexSchema tests parsing with complex schema
func (suite *InitTestSuite) TestParseToEntityTypeDTO_ComplexSchema() {
	yamlData := `
id: "complex-schema"
name: "Complex Schema"
ouId: "550e8400-e29b-41d4-a716-446655440000"
allowSelfRegistration: true
schema: |
  {
    "type": "object",
    "properties": {
      "email": {
        "type": "string",
        "format": "email"
      },
      "username": {
        "type": "string",
        "minLength": 3,
        "maxLength": 20
      },
      "age": {
        "type": "number",
        "minimum": 18
      },
      "address": {
        "type": "object",
        "properties": {
          "street": {"type": "string"},
          "city": {"type": "string"}
        }
      }
    },
    "required": ["email", "username"]
  }
`

	schemaDTO, err := parseToEntityTypeDTO([]byte(yamlData))

	suite.NoError(err)
	suite.NotNil(schemaDTO)
	suite.Equal("complex-schema", schemaDTO.ID)
	suite.Equal("Complex Schema", schemaDTO.Name)
	suite.True(schemaDTO.AllowSelfRegistration)
	suite.NotEmpty(schemaDTO.Schema)
}

// BenchmarkParseToEntityTypeDTO benchmarks the YAML parsing performance
func BenchmarkParseToEntityTypeDTO(b *testing.B) {
	yamlData := `
id: "benchmark-schema"
name: "Benchmark Schema"
ouId: "550e8400-e29b-41d4-a716-446655440000"
schema: |
  {
    "type": "object",
    "properties": {
      "email": {"type": "string"}
    }
  }
`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := parseToEntityTypeDTO([]byte(yamlData))
		if err != nil {
			b.Fatal(err)
		}
	}
}

// TestParseToEntityTypeDTO_Standalone tests YAML parsing without suite dependencies
func TestParseToEntityTypeDTO_Standalone(t *testing.T) {
	yamlData := `
id: "standalone-schema"
name: "Standalone Schema"
ouId: "550e8400-e29b-41d4-a716-446655440000"
allowSelfRegistration: false
schema: |
  {
    "type": "object",
    "properties": {
      "email": {"type": "string"}
    }
  }
`

	schemaDTO, err := parseToEntityTypeDTO([]byte(yamlData))

	assert.NoError(t, err)
	assert.NotNil(t, schemaDTO)
	assert.Equal(t, "standalone-schema", schemaDTO.ID)
	assert.Equal(t, "Standalone Schema", schemaDTO.Name)
	assert.False(t, schemaDTO.AllowSelfRegistration)
	assert.NotEmpty(t, schemaDTO.Schema)
}

// TestRegisterRoutes_Standalone tests route registration without suite dependencies
func TestRegisterRoutes_Standalone(t *testing.T) {
	mux := http.NewServeMux()
	mockHandler := &entityTypeHandler{category: TypeCategoryUser}

	assert.NotPanics(t, func() {
		registerUserTypeRoutes(mux, mockHandler)
		registerAgentTypeRoutes(mux, mockHandler)
	})
}

// TestInitialize_Standalone tests Initialize function without suite dependencies
func TestInitialize_Standalone(t *testing.T) {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
	}

	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("", testConfig)
	assert.NoError(t, err)

	mux := http.NewServeMux()
	mockOUService := oumock.NewOrganizationUnitServiceInterfaceMock(t)
	mockConsentService := mockConsentServiceWithDisabled(t)

	service, exporter, err := Initialize(mux, nil, testCacheManager(), mockOUService, nil, mockConsentService)

	assert.NoError(t, err)
	assert.NotNil(t, service)
	assert.NotNil(t, exporter)
}

// TestInitializeStore_MutableMode tests initializeStore with mutable mode (database only).
func TestInitializeStore_MutableMode(t *testing.T) {
	setupEntityTypeStoreRuntime(t, "mutable", false)

	store, transactioner, err := initializeStore(getEntityTypeStoreMode(), testCacheManager())

	assert.NoError(t, err)
	assert.NotNil(t, store)
	assert.NotNil(t, transactioner)
	assertMutableEntityTypeStore(t, store)
}

// TestInitializeStore_DeclarativeMode tests initializeStore with declarative mode (file-based only).
func TestInitializeStore_DeclarativeMode(t *testing.T) {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
		EntityType: config.EntityTypeConfig{
			Store: "declarative",
		},
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
	}

	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("", testConfig)
	assert.NoError(t, err)

	store, transactioner, err := initializeStore(getEntityTypeStoreMode(), testCacheManager())

	assert.NoError(t, err)
	assert.NotNil(t, store)
	assert.NotNil(t, transactioner)
	// In declarative mode, should return file-based store
	_, isFileBased := store.(*entityTypeFileBasedStore)
	assert.True(t, isFileBased, "Store should be file-based in declarative mode")
}

// TestInitializeStore_CompositeMode tests initializeStore with composite mode (both stores).
func TestInitializeStore_CompositeMode(t *testing.T) {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
		EntityType: config.EntityTypeConfig{
			Store: "composite",
		},
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
	}

	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("", testConfig)
	assert.NoError(t, err)

	store, transactioner, err := initializeStore(getEntityTypeStoreMode(), testCacheManager())

	assert.NoError(t, err)
	assert.NotNil(t, store)
	assert.NotNil(t, transactioner)
	// In composite mode, should return composite store
	compositeStore, isComposite := store.(*compositeEntityTypeStore)
	assert.True(t, isComposite, "Store should be composite in composite mode")
	assert.NotNil(t, compositeStore.fileStore, "Composite store should have file store")
	assert.NotNil(t, compositeStore.dbStore, "Composite store should have db store")
}

// TestInitializeStore_DefaultFallbackToMutable tests that default config falls back to mutable mode.
func TestInitializeStore_DefaultFallbackToMutable(t *testing.T) {
	setupEntityTypeStoreRuntime(t, "", false)

	store, transactioner, err := initializeStore(getEntityTypeStoreMode(), testCacheManager())

	assert.NoError(t, err)
	assert.NotNil(t, store)
	assert.NotNil(t, transactioner)
	assertMutableEntityTypeStore(t, store)
}

// TestInitializeStore_GlobalDeclarativeEnabled tests fallback to global declarative setting.
func TestInitializeStore_GlobalDeclarativeEnabled(t *testing.T) {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: true, // Global declarative enabled
		},
		EntityType: config.EntityTypeConfig{
			Store: "", // Not specified, should use global setting
		},
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
	}

	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("", testConfig)
	assert.NoError(t, err)

	store, transactioner, err := initializeStore(getEntityTypeStoreMode(), testCacheManager())

	assert.NoError(t, err)
	assert.NotNil(t, store)
	assert.NotNil(t, transactioner)
	// Should use declarative mode when global declarative resources enabled
	_, isFileBased := store.(*entityTypeFileBasedStore)
	assert.True(t, isFileBased, "Store should be file-based when global declarative enabled")
}

// TestInitialize_MutableMode tests Initialize with mutable mode (no transactioner needed).
func TestInitialize_MutableMode(t *testing.T) {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
		EntityType: config.EntityTypeConfig{
			Store: "mutable",
		},
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
	}

	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("", testConfig)
	assert.NoError(t, err)

	mux := http.NewServeMux()
	mockOUService := oumock.NewOrganizationUnitServiceInterfaceMock(t)
	mockConsentService := mockConsentServiceWithDisabled(t)

	service, exporter, err := Initialize(mux, nil, testCacheManager(), mockOUService, nil, mockConsentService)

	assert.NoError(t, err)
	assert.NotNil(t, service)
	assert.NotNil(t, exporter)
	assert.Implements(t, (*EntityTypeServiceInterface)(nil), service)
}

// TestInitialize_StoreModes tests Initialize with various store modes (declarative and composite).
func TestInitialize_StoreModes(t *testing.T) {
	modes := []struct {
		name  string
		store string
	}{
		{"DeclarativeMode", "declarative"},
		{"CompositeMode", "composite"},
	}

	for _, m := range modes {
		t.Run(m.name, func(t *testing.T) {
			testConfig := &config.Config{
				DeclarativeResources: config.DeclarativeResources{Enabled: false},
				EntityType:           config.EntityTypeConfig{Store: m.store},
				Database: config.DatabaseConfig{
					Config: config.DataSource{
						Type:   "sqlite",
						SQLite: config.SQLiteDataSource{Path: ":memory:"},
					},
				},
			}

			config.ResetServerRuntime()
			err := config.InitializeServerRuntime("", testConfig)
			assert.NoError(t, err)

			mux := http.NewServeMux()
			mockOUService := oumock.NewOrganizationUnitServiceInterfaceMock(t)
			// Mock OU service for potential declarative resource loading
			mockOUService.On("GetOrganizationUnit", mock.Anything, mock.Anything).
				Return(providers.OrganizationUnit{ID: "ou-1"}, nil).
				Maybe()
			mockConsentService := mockConsentServiceWithDisabled(t)

			service, exporter, err := Initialize(mux, nil, testCacheManager(), mockOUService, nil, mockConsentService)

			assert.NoError(t, err)
			assert.NotNil(t, service)
			assert.NotNil(t, exporter)
		})
	}
}

// TestRegisterRoutes_AllEndpoints tests that all expected routes are registered.
func TestRegisterRoutes_AllEndpoints(t *testing.T) {
	mux := http.NewServeMux()
	// Create a mock service to avoid nil pointer issues
	mockService := NewEntityTypeServiceInterfaceMock(t)
	mockHandler := newEntityTypeHandler(mockService, TypeCategoryUser)

	registerUserTypeRoutes(mux, mockHandler)

	// Test that OPTIONS endpoints are registered for CORS
	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodOptions, "/user-types"},
		{http.MethodOptions, "/user-types/test-id"},
	}

	for _, ep := range endpoints {
		req := httptest.NewRequest(ep.method, ep.path, nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		// Should not return 404 if route is registered, should return 204 for OPTIONS
		assert.NotEqual(t, http.StatusNotFound, w.Code,
			"Route %s %s should be registered", ep.method, ep.path)
	}
}

// TestParseToEntityTypeDTO_InvalidJSONSchema tests parsing with invalid JSON in schema field
func TestParseToEntityTypeDTO_InvalidJSONSchema(t *testing.T) {
	yamlData := `
id: "invalid-json-schema"
name: "Invalid JSON Schema"
ouId: "550e8400-e29b-41d4-a716-446655440000"
schema: |
  {invalid json here}
`

	schemaDTO, err := parseToEntityTypeDTO([]byte(yamlData))

	assert.Error(t, err)
	assert.Nil(t, schemaDTO)
	assert.Contains(t, err.Error(), "invalid JSON")
}

// TestParseToEntityTypeDTO_EmptySchemaField tests parsing with empty schema field
func TestParseToEntityTypeDTO_EmptySchemaField(t *testing.T) {
	yamlData := `
id: "empty-schema"
name: "Empty Schema"
ouId: "550e8400-e29b-41d4-a716-446655440000"
schema: ""
`

	schemaDTO, err := parseToEntityTypeDTO([]byte(yamlData))

	assert.Error(t, err)
	assert.Nil(t, schemaDTO)
	assert.Contains(t, err.Error(), "invalid JSON")
}

// TestValidateEntityTypeWithOUCheck tests the validation logic that would be used during initialization
// This tests the same validation path that occurs before the OU service call in Initialize()
func TestValidateEntityTypeWithOUCheck(t *testing.T) {
	testCases := []struct {
		name          string
		schema        EntityType
		shouldBeValid bool
		errorContains string
	}{
		{
			name: "Valid schema with valid OU ID",
			schema: EntityType{
				ID:     "valid-schema-001",
				Name:   "Valid Schema",
				OUID:   "550e8400-e29b-41d4-a716-446655440000",
				Schema: []byte(`{"email":{"type":"string","required":true}}`),
			},
			shouldBeValid: true,
		},
		{
			name: "Invalid schema - empty name",
			schema: EntityType{
				ID:     "invalid-001",
				Name:   "",
				OUID:   "550e8400-e29b-41d4-a716-446655440000",
				Schema: []byte(`{"email":{"type":"string"}}`),
			},
			shouldBeValid: false,
			errorContains: "entity type name must not be empty",
		},
		{
			name: "Invalid schema - empty OU ID",
			schema: EntityType{
				ID:     "invalid-002",
				Name:   "Test Schema",
				OUID:   "",
				Schema: []byte(`{"email":{"type":"string"}}`),
			},
			shouldBeValid: false,
			errorContains: "organization unit id must not be empty",
		},
		{
			name: "Valid schema - non-UUID OU ID",
			schema: EntityType{
				ID:     "invalid-003",
				Name:   "Test Schema",
				OUID:   "not-a-valid-uuid",
				Schema: []byte(`{"email":{"type":"string"}}`),
			},
			shouldBeValid: true,
		},
		{
			name: "Invalid schema - empty schema definition",
			schema: EntityType{
				ID:     "invalid-004",
				Name:   "Test Schema",
				OUID:   "550e8400-e29b-41d4-a716-446655440000",
				Schema: []byte{},
			},
			shouldBeValid: false,
			errorContains: "schema definition must not be empty",
		},
		{
			name: "Invalid schema - malformed schema definition",
			schema: EntityType{
				ID:     "invalid-005",
				Name:   "Test Schema",
				OUID:   "550e8400-e29b-41d4-a716-446655440000",
				Schema: []byte(`{"email":"not-an-object"}`),
			},
			shouldBeValid: false,
			errorContains: "property definition must be an object",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateEntityTypeDefinition(context.Background(), TypeCategoryUser, tc.schema)

			if tc.shouldBeValid {
				assert.Nil(t, err, "Expected schema to be valid but got error: %v", err)
			} else {
				assert.NotNil(t, err, "Expected validation to fail")
				if err != nil {
					assert.Contains(t, err.ErrorDescription.DefaultValue, tc.errorContains,
						"Error message should contain expected text")
					assert.Equal(t, ErrorInvalidEntityTypeRequest.Code, err.Code)
				}
			}
		})
	}
}

// TestOUServiceInteractionDuringValidation tests that the OU service would be called correctly
// This validates the logic flow that occurs in Initialize() when checking OU existence
func TestOUServiceInteractionDuringValidation(t *testing.T) {
	testCases := []struct {
		name           string
		ouID           string
		ouExists       bool
		ouServiceError *tidcommon.ServiceError
		expectedResult string
	}{
		{
			name:           "OU exists - should pass",
			ouID:           "550e8400-e29b-41d4-a716-446655440000",
			ouExists:       true,
			ouServiceError: nil,
			expectedResult: "success",
		},
		{
			name:           "OU does not exist - should fail",
			ouID:           "550e8400-e29b-41d4-a716-446655440001",
			ouExists:       false,
			ouServiceError: nil,
			expectedResult: "ou_not_found",
		},
		{
			name:     "OU service returns error - should fail",
			ouID:     "550e8400-e29b-41d4-a716-446655440002",
			ouExists: false,
			ouServiceError: &tidcommon.ServiceError{
				Code: "OUS-5000",
				Type: tidcommon.ServerErrorType,
				Error: tidcommon.I18nMessage{
					Key:          "error.organizationunit.internal_server_error",
					DefaultValue: "Internal server error",
				},
				ErrorDescription: tidcommon.I18nMessage{
					Key:          "error.organizationunit.failed_to_query",
					DefaultValue: "Failed to query organization unit",
				},
			},
			expectedResult: "service_error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockOUService := oumock.NewOrganizationUnitServiceInterfaceMock(t)

			// Mock the GetOrganizationUnit call that happens in Initialize()
			if tc.ouServiceError != nil {
				mockOUService.On("GetOrganizationUnit", mock.Anything, tc.ouID).
					Return(providers.OrganizationUnit{}, tc.ouServiceError).Once()
			} else if tc.ouExists {
				mockOUService.On("GetOrganizationUnit", mock.Anything, tc.ouID).
					Return(providers.OrganizationUnit{ID: tc.ouID}, (*tidcommon.ServiceError)(nil)).Once()
			} else {
				mockOUService.On("GetOrganizationUnit", mock.Anything, tc.ouID).
					Return(providers.OrganizationUnit{}, &tidcommon.ServiceError{
						Code: "OUS-1002",
						Type: tidcommon.ClientErrorType,
						Error: tidcommon.I18nMessage{
							Key:          "error.organizationunit.not_found",
							DefaultValue: "Organization unit not found",
						},
						ErrorDescription: tidcommon.I18nMessage{
							Key:          "error.organizationunit.not_found_description",
							DefaultValue: "The organization unit does not exist",
						},
					}).Once()
			}

			// Simulate the OU validation logic from Initialize()
			_, svcErr := mockOUService.GetOrganizationUnit(context.Background(), tc.ouID)

			switch tc.expectedResult {
			case "success":
				assert.Nil(t, svcErr, "Expected no error when OU exists")
			case "ou_not_found":
				assert.NotNil(t, svcErr, "Expected error when OU does not exist")
				assert.Equal(t, "OUS-1002", svcErr.Code)
			case "service_error":
				assert.NotNil(t, svcErr, "Expected error when OU service fails")
				assert.Equal(t, "OUS-5000", svcErr.Code)
			}

			mockOUService.AssertExpectations(t)
		})
	}
}

// TestParseAndValidateEntityTypeFlow tests the complete flow of parsing and validating
// This simulates what happens in Initialize() before the OU check
func TestParseAndValidateEntityTypeFlow(t *testing.T) {
	testCases := []struct {
		name          string
		yamlData      string
		expectParseOK bool
		expectValidOK bool
		errorContains string
	}{
		{
			name: "Valid YAML and schema",
			yamlData: `
id: "flow-test-001"
name: "Flow Test Schema"
ouId: "550e8400-e29b-41d4-a716-446655440000"
schema: |
  {
    "email": {"type": "string", "required": true}
  }
`,
			expectParseOK: true,
			expectValidOK: true,
		},
		{
			name: "Valid YAML but invalid schema definition",
			yamlData: `
id: "flow-test-002"
name: "Invalid Schema"
ouId: "550e8400-e29b-41d4-a716-446655440000"
schema: |
  {
    "email": {"required": true}
  }
`,
			expectParseOK: true,
			expectValidOK: false,
			errorContains: "missing required 'type' field",
		},
		{
			name: "Valid YAML but empty schema name",
			yamlData: `
id: "flow-test-003"
name: ""
ouId: "550e8400-e29b-41d4-a716-446655440000"
schema: |
  {
    "email": {"type": "string"}
  }
`,
			expectParseOK: true,
			expectValidOK: false,
			errorContains: "entity type name must not be empty",
		},
		{
			name: "Invalid YAML structure",
			yamlData: `
this is not valid yaml:
  - broken structure
`,
			expectParseOK: false,
			expectValidOK: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Step 1: Parse YAML (as done in Initialize)
			schemaDTO, parseErr := parseToEntityTypeDTO([]byte(tc.yamlData))

			if tc.expectParseOK {
				assert.NoError(t, parseErr, "Expected YAML parsing to succeed")
				assert.NotNil(t, schemaDTO)

				// Step 2: Validate schema (as done in Initialize before OU check)
				validationErr := validateEntityTypeDefinition(context.Background(), TypeCategoryUser, *schemaDTO)

				if tc.expectValidOK {
					assert.Nil(t, validationErr, "Expected validation to succeed")
				} else {
					assert.NotNil(t, validationErr, "Expected validation to fail")
					if validationErr != nil && tc.errorContains != "" {
						assert.Contains(t, validationErr.ErrorDescription.DefaultValue, tc.errorContains)
					}
				}
			} else {
				assert.Error(t, parseErr, "Expected YAML parsing to fail")
				assert.Nil(t, schemaDTO)
			}
		})
	}
}

// TestInitialize_WithDeclarativeResourcesEnabled_InvalidYAML tests Initialize with invalid YAML files
//
//nolint:dupl // Similar test setup required for different error scenarios
func TestInitialize_WithDeclarativeResourcesEnabled_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	confDir := tmpDir + "/config/resources"
	schemaDir := confDir + "/user_types"

	err := os.MkdirAll(schemaDir, 0750)
	assert.NoError(t, err)

	// Create an invalid YAML file
	invalidYAML := `invalid yaml content
  - this is not: valid
`
	err = os.WriteFile(schemaDir+"/invalid-schema.yaml", []byte(invalidYAML), 0600)
	assert.NoError(t, err)

	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: true,
		},
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
		Crypto: config.CryptoConfig{
			Encryption: engineconfig.EncryptionConfig{
				Key: testCryptoKey,
			},
		},
	}

	config.ResetServerRuntime()
	err = config.InitializeServerRuntime(tmpDir, testConfig)
	assert.NoError(t, err)
	defer config.ResetServerRuntime()

	mux := http.NewServeMux()
	mockOUService := oumock.NewOrganizationUnitServiceInterfaceMock(t)
	mockConsentService := mockConsentServiceWithDisabled(t)

	// Initialize should return an error due to invalid YAML
	_, _, err = Initialize(mux, nil, testCacheManager(), mockOUService, nil, mockConsentService)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load entity type resources")
}

// TestInitialize_WithDeclarativeResourcesEnabled_ValidationFailure tests Initialize with validation errors
//
//nolint:dupl // Similar test setup required for different error scenarios
func TestInitialize_WithDeclarativeResourcesEnabled_ValidationFailure(t *testing.T) {
	tmpDir := t.TempDir()
	confDir := tmpDir + "/config/resources"
	schemaDir := confDir + "/user_types"

	err := os.MkdirAll(schemaDir, 0750)
	assert.NoError(t, err)

	// Create crypto directory
	cryptoDir := tmpDir + "/config/certs"
	err = os.MkdirAll(cryptoDir, 0750)
	assert.NoError(t, err)

	// Create a YAML file with invalid configuration (empty name)
	invalidSchemaYAML := `id: "invalid-schema"
name: ""
ouId: "550e8400-e29b-41d4-a716-446655440000"
schema: |
  {
    "email": {"type": "string"}
  }
`
	err = os.WriteFile(schemaDir+"/invalid-schema.yaml", []byte(invalidSchemaYAML), 0600)
	assert.NoError(t, err)

	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: true,
		},
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
		Crypto: config.CryptoConfig{
			Encryption: engineconfig.EncryptionConfig{
				Key: testCryptoKey,
			},
		},
	}

	config.ResetServerRuntime()
	err = config.InitializeServerRuntime(tmpDir, testConfig)
	assert.NoError(t, err)
	defer config.ResetServerRuntime()

	mux := http.NewServeMux()
	mockOUService := oumock.NewOrganizationUnitServiceInterfaceMock(t)
	mockConsentService := mockConsentServiceWithDisabled(t)

	// Initialize should return an error due to validation failure
	_, _, err = Initialize(mux, nil, testCacheManager(), mockOUService, nil, mockConsentService)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load entity type resources")
}

// TestInitialize_WithDeclarativeResourcesEnabled_OUHandleNotFound tests Initialize when an
// ou_handle in a declarative resource cannot be resolved.
func TestInitialize_WithDeclarativeResourcesEnabled_OUHandleNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	confDir := tmpDir + "/config/resources"
	schemaDir := confDir + "/user_types"

	err := os.MkdirAll(schemaDir, 0750)
	assert.NoError(t, err)

	// Create crypto directory
	cryptoDir := tmpDir + "/config/certs"
	err = os.MkdirAll(cryptoDir, 0750)
	assert.NoError(t, err)

	// Create a YAML file that uses an ou_handle that cannot be resolved
	validSchemaYAML := `id: "test-schema"
name: "Test Schema"
ouHandle: "nonexistent-handle"
allowSelfRegistration: true
schema: |
  {
    "email": {"type": "string", "required": true}
  }
`
	err = os.WriteFile(schemaDir+"/test-schema.yaml", []byte(validSchemaYAML), 0600)
	assert.NoError(t, err)

	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: true,
		},
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
		Crypto: config.CryptoConfig{
			Encryption: engineconfig.EncryptionConfig{
				Key: testCryptoKey,
			},
		},
	}

	config.ResetServerRuntime()
	err = config.InitializeServerRuntime(tmpDir, testConfig)
	assert.NoError(t, err)
	defer config.ResetServerRuntime()

	mux := http.NewServeMux()
	mockOUService := oumock.NewOrganizationUnitServiceInterfaceMock(t)

	// Mock OU service to return not-found for the handle
	mockOUService.On("GetOrganizationUnitByPath", mock.Anything, "nonexistent-handle").
		Return(providers.OrganizationUnit{}, &tidcommon.ServiceError{
			Code: "OUS-1002",
			Type: tidcommon.ClientErrorType,
			Error: tidcommon.I18nMessage{
				Key:          "error.organizationunit.not_found",
				DefaultValue: "Organization unit not found",
			},
			ErrorDescription: tidcommon.I18nMessage{
				Key:          "error.organizationunit.not_found_description",
				DefaultValue: "The organization unit does not exist",
			},
		}).Once()
	mockConsentService := mockConsentServiceWithDisabled(t)

	_, _, err = Initialize(mux, nil, testCacheManager(), mockOUService, nil, mockConsentService)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load entity type resources")

	mockOUService.AssertExpectations(t)
}

// TestInitialize_WithDeclarativeResourcesEnabled_InvalidJSONSchema tests Initialize with invalid JSON in schema
//
//nolint:dupl // Similar test setup required for different error scenarios
func TestInitialize_WithDeclarativeResourcesEnabled_InvalidJSONSchema(t *testing.T) {
	tmpDir := t.TempDir()
	confDir := tmpDir + "/config/resources"
	schemaDir := confDir + "/user_types"

	err := os.MkdirAll(schemaDir, 0750)
	assert.NoError(t, err)

	// Create crypto directory
	cryptoDir := tmpDir + "/config/certs"
	err = os.MkdirAll(cryptoDir, 0750)
	assert.NoError(t, err)

	// Create a YAML file with invalid JSON in schema field
	invalidJSONYAML := `id: "invalid-json-schema"
name: "Invalid JSON Schema"
ouId: "550e8400-e29b-41d4-a716-446655440000"
schema: |
  {invalid json here}
`
	err = os.WriteFile(schemaDir+"/invalid-json.yaml", []byte(invalidJSONYAML), 0600)
	assert.NoError(t, err)

	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: true,
		},
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
		Crypto: config.CryptoConfig{
			Encryption: engineconfig.EncryptionConfig{
				Key: testCryptoKey,
			},
		},
	}

	config.ResetServerRuntime()
	err = config.InitializeServerRuntime(tmpDir, testConfig)
	assert.NoError(t, err)
	defer config.ResetServerRuntime()

	mux := http.NewServeMux()
	mockOUService := oumock.NewOrganizationUnitServiceInterfaceMock(t)
	mockConsentService := mockConsentServiceWithDisabled(t)

	// Initialize should return an error due to invalid JSON
	_, _, err = Initialize(mux, nil, testCacheManager(), mockOUService, nil, mockConsentService)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load entity type resources")
}

// mockConsentServiceWithDisabled creates a mock ConsentServiceInterface with IsEnabled returning false
func mockConsentServiceWithDisabled(t *testing.T) *consentmock.ConsentServiceInterfaceMock {
	mockConsentService := consentmock.NewConsentServiceInterfaceMock(t)
	mockConsentService.EXPECT().IsEnabled().Return(false).Maybe()
	return mockConsentService
}
