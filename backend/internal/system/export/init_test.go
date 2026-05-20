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

package export

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/thunder-id/thunderid/internal/application"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/notification"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/cors"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/tests/mocks/applicationmock"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
	"github.com/thunder-id/thunderid/tests/mocks/idp/idpmock"
	"github.com/thunder-id/thunderid/tests/mocks/notification/notificationmock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	yaml "gopkg.in/yaml.v3"
)

// InitTestSuite contains comprehensive tests for the init.go file.
// The test suite covers:
// - Initialize function with proper service creation and route registration
// - registerRoutes function with CORS setup and endpoint registration
// - Route handling validation for all export endpoints
// - HTTP method validation and OPTIONS request handling
type InitTestSuite struct {
	suite.Suite
	mockAppService          *applicationmock.ApplicationServiceInterfaceMock
	mockIDPService          *idpmock.IDPServiceInterfaceMock
	mockNotificationService *notificationmock.NotificationSenderMgtSvcInterfaceMock
	mockEntityTypeService   *entitytypemock.EntityTypeServiceInterfaceMock
}

func (suite *InitTestSuite) SetupTest() {
	suite.mockAppService = applicationmock.NewApplicationServiceInterfaceMock(suite.T())
	suite.mockIDPService = idpmock.NewIDPServiceInterfaceMock(suite.T())
	suite.mockNotificationService = notificationmock.NewNotificationSenderMgtSvcInterfaceMock(suite.T())
	suite.mockEntityTypeService = entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T())
	// Initialize config for CORS middleware
	config.ResetServerRuntime()
	var allowedOrigins cors.OriginEntries
	suite.Require().NoError(yaml.Unmarshal([]byte(`
- https://example.com
- https://localhost:3000
`), &allowedOrigins))
	testConfig := &config.Config{
		CORS: config.CORSConfig{AllowedOrigins: allowedOrigins},
	}
	if err := cors.InitializeMatcher(testConfig.CORS.AllowedOrigins); err != nil {
		suite.T().Fatalf("Failed to initialize CORS matcher: %v", err)
	}
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	if err != nil {
		suite.T().Fatalf("Failed to initialize config: %v", err)
	}
}

func (suite *InitTestSuite) TearDownTest() {
	// Reset config to clear singleton state for next test
	config.ResetServerRuntime()
}

func TestInitTestSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

// createTestExporters is a helper function to create exporters from mock services for testing
func createTestExporters(
	appService *applicationmock.ApplicationServiceInterfaceMock,
	idpService *idpmock.IDPServiceInterfaceMock,
	notificationService *notificationmock.NotificationSenderMgtSvcInterfaceMock,
	entityTypeService *entitytypemock.EntityTypeServiceInterfaceMock,
) []declarativeresource.ResourceExporter {
	return []declarativeresource.ResourceExporter{
		application.NewApplicationExporterForTest(appService),
		idp.NewIDPExporterForTest(idpService),
		notification.NewNotificationSenderExporterForTest(notificationService),
		entitytype.NewEntityTypeExporterForTest(entityTypeService),
	}
}

// TestInitialize tests the Initialize function
func (suite *InitTestSuite) TestInitialize() {
	mux := http.NewServeMux()
	exporters := createTestExporters(suite.mockAppService, suite.mockIDPService,
		suite.mockNotificationService, suite.mockEntityTypeService)

	// Execute
	service := Initialize(mux, exporters)

	// Assert
	assert.NotNil(suite.T(), service)
	assert.Implements(suite.T(), (*ExportServiceInterface)(nil), service)
}

// TestInitialize_ServiceCreation tests that Initialize creates the service with proper dependencies
func (suite *InitTestSuite) TestInitialize_ServiceCreation() {
	mux := http.NewServeMux()
	exporters := createTestExporters(suite.mockAppService, suite.mockIDPService,
		suite.mockNotificationService, suite.mockEntityTypeService)

	// Execute
	service := Initialize(mux, exporters)

	// Assert
	assert.NotNil(suite.T(), service)
	// Verify that the service is properly configured with dependencies
	// Since we can't directly access the internal fields, we test through interface
	assert.Implements(suite.T(), (*ExportServiceInterface)(nil), service)
}

// TestRegisterRoutes tests the route registration function
func (suite *InitTestSuite) TestRegisterRoutes() {
	mux := http.NewServeMux()
	exporters := createTestExporters(suite.mockAppService, suite.mockIDPService,
		suite.mockNotificationService, suite.mockEntityTypeService)
	mockService := newExportService(exporters, newParameterizer(templatingRules{}))
	exportHandler := newExportHandler(mockService)

	// Execute
	assert.NotPanics(suite.T(), func() {
		registerRoutes(mux, exportHandler)
	})
}

// TestRegisterRoutes_JSONEndpoint tests the JSON export endpoint registration.
func (suite *InitTestSuite) TestRegisterRoutes_JSONEndpoint() {
	mux := http.NewServeMux()
	exporters := createTestExporters(suite.mockAppService, suite.mockIDPService,
		suite.mockNotificationService, suite.mockEntityTypeService)
	mockService := newExportService(exporters, newParameterizer(templatingRules{}))
	exportHandler := newExportHandler(mockService)

	registerRoutes(mux, exportHandler)

	// Test POST /export endpoint
	req := httptest.NewRequest("POST", "/export", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// The mux should handle the request (even if it fails due to invalid request body)
	mux.ServeHTTP(w, req)

	// Should not be 404 (route exists)
	assert.NotEqual(suite.T(), http.StatusNotFound, w.Code)
}

// TestRegisterRoutes_ZIPEndpoint tests the ZIP export endpoint registration
func (suite *InitTestSuite) TestRegisterRoutes_ZIPEndpoint() {
	mux := http.NewServeMux()
	exporters := createTestExporters(suite.mockAppService, suite.mockIDPService,
		suite.mockNotificationService, suite.mockEntityTypeService)
	mockService := newExportService(exporters, newParameterizer(templatingRules{}))
	exportHandler := newExportHandler(mockService)

	registerRoutes(mux, exportHandler)

	// Test POST /export/zip endpoint
	req := httptest.NewRequest("POST", "/export/zip", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	// Should not be 404 (route exists)
	assert.NotEqual(suite.T(), http.StatusNotFound, w.Code)
}

// TestRegisterRoutes_OptionsEndpoint tests the OPTIONS endpoint registration
func (suite *InitTestSuite) TestRegisterRoutes_OptionsEndpoint() {
	mux := http.NewServeMux()
	exporters := createTestExporters(suite.mockAppService, suite.mockIDPService,
		suite.mockNotificationService, suite.mockEntityTypeService)
	mockService := newExportService(exporters, newParameterizer(templatingRules{}))
	exportHandler := newExportHandler(mockService)

	registerRoutes(mux, exportHandler)

	// Test OPTIONS /export endpoint
	req := httptest.NewRequest("OPTIONS", "/export", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	// Should return 204 No Content for OPTIONS request
	assert.Equal(suite.T(), http.StatusNoContent, w.Code)
}

// TestRegisterRoutes_CORSHeaders tests that CORS headers are properly set on
// a CORS preflight request (OPTIONS + Access-Control-Request-Method). Under
// the Fetch spec the Allow-Methods/Allow-Headers/Max-Age response headers are
// preflight-only, so the request must carry ACRM to elicit them.
func (suite *InitTestSuite) TestRegisterRoutes_CORSHeaders() {
	mux := http.NewServeMux()
	exporters := createTestExporters(suite.mockAppService, suite.mockIDPService,
		suite.mockNotificationService, suite.mockEntityTypeService)
	mockService := newExportService(exporters, newParameterizer(templatingRules{}))
	exportHandler := newExportHandler(mockService)

	registerRoutes(mux, exportHandler)

	// Issue a real preflight: OPTIONS with Origin and Access-Control-Request-Method.
	req := httptest.NewRequest("OPTIONS", "/export", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	headers := w.Header()
	assert.Contains(suite.T(), headers, "Access-Control-Allow-Origin")
	assert.Contains(suite.T(), headers, "Access-Control-Allow-Methods")
	assert.Contains(suite.T(), headers, "Access-Control-Allow-Headers")
	assert.Contains(suite.T(), headers, "Access-Control-Allow-Credentials")
	assert.Contains(suite.T(), headers, "Vary")
}

// TestRegisterRoutes_InvalidMethod tests that invalid HTTP methods return appropriate responses
func (suite *InitTestSuite) TestRegisterRoutes_InvalidMethod() {
	mux := http.NewServeMux()
	exporters := createTestExporters(suite.mockAppService, suite.mockIDPService,
		suite.mockNotificationService, suite.mockEntityTypeService)
	mockService := newExportService(exporters, newParameterizer(templatingRules{}))
	exportHandler := newExportHandler(mockService)

	registerRoutes(mux, exportHandler)

	// Test GET method on POST-only endpoint
	req := httptest.NewRequest("GET", "/export", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	// Should return method not allowed
	assert.Equal(suite.T(), http.StatusMethodNotAllowed, w.Code)
}

// TestRegisterRoutes_UnregisteredPath tests that unregistered paths return 404
func (suite *InitTestSuite) TestRegisterRoutes_UnregisteredPath() {
	mux := http.NewServeMux()
	exporters := createTestExporters(suite.mockAppService, suite.mockIDPService,
		suite.mockNotificationService, suite.mockEntityTypeService)
	mockService := newExportService(exporters, newParameterizer(templatingRules{}))
	exportHandler := newExportHandler(mockService)

	registerRoutes(mux, exportHandler)

	// Test unregistered path
	req := httptest.NewRequest("POST", "/export/invalid", strings.NewReader(`{}`))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	// Should return 404 for unregistered path
	assert.Equal(suite.T(), http.StatusNotFound, w.Code)
}

// TestRegisterRoutes_WithNilHandler tests that registerRoutes handles nil handler gracefully
func (suite *InitTestSuite) TestRegisterRoutes_WithNilHandler() {
	mux := http.NewServeMux()

	// This should not panic even with nil handler
	assert.NotPanics(suite.T(), func() {
		registerRoutes(mux, nil)
	})
}

// TestRegisterRoutes_PreflightRequest tests CORS preflight request handling
func (suite *InitTestSuite) TestRegisterRoutes_PreflightRequest() {
	mux := http.NewServeMux()
	exporters := createTestExporters(suite.mockAppService, suite.mockIDPService,
		suite.mockNotificationService, suite.mockEntityTypeService)
	mockService := newExportService(exporters, newParameterizer(templatingRules{}))
	exportHandler := newExportHandler(mockService)

	registerRoutes(mux, exportHandler)

	// Test preflight request with proper Origin header
	req := httptest.NewRequest("OPTIONS", "/export", nil)
	req.Header.Set("Origin", "https://example.com") // This is required for CORS headers
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type, Authorization")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	// Should return 204 No Content for preflight
	assert.Equal(suite.T(), http.StatusNoContent, w.Code)

	// Verify CORS headers when Origin is provided
	headers := w.Header()
	assert.NotEmpty(suite.T(), headers.Get("Access-Control-Allow-Methods"))
	assert.NotEmpty(suite.T(), headers.Get("Access-Control-Allow-Headers"))
}

// Benchmark tests for performance evaluation

// BenchmarkInitialize benchmarks the Initialize function
func BenchmarkInitialize(b *testing.B) {
	mockAppService := applicationmock.NewApplicationServiceInterfaceMock(b)
	mockIDPService := idpmock.NewIDPServiceInterfaceMock(b)
	mockNotificationService := notificationmock.NewNotificationSenderMgtSvcInterfaceMock(b)
	mockEntityTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		exporters := createTestExporters(mockAppService, mockIDPService, mockNotificationService, mockEntityTypeService)
		mux := http.NewServeMux()
		Initialize(mux, exporters)
	}
}

// BenchmarkRegisterRoutes benchmarks the route registration
func BenchmarkRegisterRoutes(b *testing.B) {
	mockAppService := applicationmock.NewApplicationServiceInterfaceMock(b)
	mockIDPService := idpmock.NewIDPServiceInterfaceMock(b)
	mockNotificationService := notificationmock.NewNotificationSenderMgtSvcInterfaceMock(b)
	mockEntityTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(b)
	exporters := createTestExporters(mockAppService, mockIDPService, mockNotificationService, mockEntityTypeService)
	mockService := newExportService(exporters, newParameterizer(templatingRules{}))
	exportHandler := newExportHandler(mockService)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mux := http.NewServeMux()
		registerRoutes(mux, exportHandler)
	}
}

// Individual test functions that don't rely on suite setup

// TestInitialize_Standalone tests Initialize function without suite dependencies
func TestInitialize_Standalone(t *testing.T) {
	// Setup config for CORS middleware
	config.ResetServerRuntime()
	var allowedOrigins cors.OriginEntries
	assert.NoError(t, yaml.Unmarshal([]byte(`
- https://example.com
- https://localhost:3000
`), &allowedOrigins))
	testConfig := &config.Config{
		CORS: config.CORSConfig{AllowedOrigins: allowedOrigins},
	}
	assert.NoError(t, cors.InitializeMatcher(testConfig.CORS.AllowedOrigins))
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	assert.NoError(t, err)
	defer config.ResetServerRuntime()

	mockAppService := applicationmock.NewApplicationServiceInterfaceMock(t)
	mockIDPService := idpmock.NewIDPServiceInterfaceMock(t)
	mockNotificationService := notificationmock.NewNotificationSenderMgtSvcInterfaceMock(t)
	mockEntityTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	exporters := createTestExporters(mockAppService, mockIDPService, mockNotificationService, mockEntityTypeService)
	mux := http.NewServeMux()

	// Execute
	service := Initialize(mux, exporters)

	// Assert
	assert.NotNil(t, service)
	assert.Implements(t, (*ExportServiceInterface)(nil), service)
}

// TestRegisterRoutes_Standalone tests route registration without suite dependencies
func TestRegisterRoutes_Standalone(t *testing.T) {
	// Setup config for CORS middleware
	config.ResetServerRuntime()
	var allowedOrigins cors.OriginEntries
	assert.NoError(t, yaml.Unmarshal([]byte(`
- https://example.com
- https://localhost:3000
`), &allowedOrigins))
	testConfig := &config.Config{
		CORS: config.CORSConfig{AllowedOrigins: allowedOrigins},
	}
	assert.NoError(t, cors.InitializeMatcher(testConfig.CORS.AllowedOrigins))
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	assert.NoError(t, err)
	defer config.ResetServerRuntime()

	mockAppService := applicationmock.NewApplicationServiceInterfaceMock(t)
	mockIDPService := idpmock.NewIDPServiceInterfaceMock(t)
	mockNotificationService := notificationmock.NewNotificationSenderMgtSvcInterfaceMock(t)
	mockEntityTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	exporters := createTestExporters(mockAppService, mockIDPService, mockNotificationService, mockEntityTypeService)
	mockService := newExportService(exporters, newParameterizer(templatingRules{}))
	exportHandler := newExportHandler(mockService)
	mux := http.NewServeMux()

	// Execute - should not panic
	assert.NotPanics(t, func() {
		registerRoutes(mux, exportHandler)
	})
}

// TestRouteHandling_Standalone tests that routes are properly handled
func TestRouteHandling_Standalone(t *testing.T) {
	// Setup config for CORS middleware
	config.ResetServerRuntime()
	var allowedOrigins cors.OriginEntries
	assert.NoError(t, yaml.Unmarshal([]byte(`
- https://example.com
- https://localhost:3000
`), &allowedOrigins))
	testConfig := &config.Config{
		CORS: config.CORSConfig{AllowedOrigins: allowedOrigins},
	}
	assert.NoError(t, cors.InitializeMatcher(testConfig.CORS.AllowedOrigins))
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	assert.NoError(t, err)
	defer config.ResetServerRuntime()

	mockAppService := applicationmock.NewApplicationServiceInterfaceMock(t)
	mockIDPService := idpmock.NewIDPServiceInterfaceMock(t)
	mockNotificationService := notificationmock.NewNotificationSenderMgtSvcInterfaceMock(t)
	mockEntityTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	exporters := createTestExporters(mockAppService, mockIDPService, mockNotificationService, mockEntityTypeService)
	mux := http.NewServeMux()
	Initialize(mux, exporters)

	// Test that all routes are registered
	testCases := []struct {
		method         string
		path           string
		expectNotFound bool
	}{
		{"POST", "/export", false},
		{"POST", "/export/zip", false},
		{"OPTIONS", "/export", false},
		{"GET", "/export", true},   // Should be method not allowed, not not found
		{"POST", "/invalid", true}, // Should be not found
	}

	for _, tc := range testCases {
		t.Run(tc.method+"_"+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{}`))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, req)

			if tc.expectNotFound {
				if tc.method == "GET" && tc.path == "/export" {
					// This should be method not allowed, not not found
					assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
				} else {
					assert.Equal(t, http.StatusNotFound, w.Code)
				}
			} else {
				assert.NotEqual(t, http.StatusNotFound, w.Code)
			}
		})
	}
}

// TestCORSConfiguration_Standalone tests CORS configuration without suite
func TestCORSConfiguration_Standalone(t *testing.T) {
	// Setup config for CORS middleware
	config.ResetServerRuntime()
	var allowedOrigins cors.OriginEntries
	assert.NoError(t, yaml.Unmarshal([]byte(`
- https://example.com
- https://localhost:3000
`), &allowedOrigins))
	testConfig := &config.Config{
		CORS: config.CORSConfig{AllowedOrigins: allowedOrigins},
	}
	assert.NoError(t, cors.InitializeMatcher(testConfig.CORS.AllowedOrigins))
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	assert.NoError(t, err)
	defer config.ResetServerRuntime()

	mockAppService := applicationmock.NewApplicationServiceInterfaceMock(t)
	mockIDPService := idpmock.NewIDPServiceInterfaceMock(t)
	mockNotificationService := notificationmock.NewNotificationSenderMgtSvcInterfaceMock(t)
	mockEntityTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	exporters := createTestExporters(mockAppService, mockIDPService, mockNotificationService, mockEntityTypeService)
	mux := http.NewServeMux()
	Initialize(mux, exporters)

	// Test CORS on actual request with Origin header
	req := httptest.NewRequest("OPTIONS", "/export", nil)
	req.Header.Set("Origin", "https://localhost:3000") // This is required for CORS headers
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	// Verify response code
	assert.Equal(t, http.StatusNoContent, w.Code)

	// Verify CORS headers are present when Origin is provided
	headers := w.Header()
	assert.True(t, len(headers) > 0, "CORS headers should be present")
}
