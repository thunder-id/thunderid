// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package export

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/mock"

	"github.com/thunder-id/thunderid/internal/application"
	"github.com/thunder-id/thunderid/internal/connection"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/system/config"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/tests/mocks/applicationmock"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
	"github.com/thunder-id/thunderid/tests/mocks/idp/idpmock"
	"github.com/thunder-id/thunderid/tests/mocks/notification/notificationmock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// HandlerTestSuite contains comprehensive tests for the export handler functions.
type HandlerTestSuite struct {
	suite.Suite
	mockAppService          *applicationmock.ApplicationServiceInterfaceMock
	mockIDPService          *idpmock.IDPServiceInterfaceMock
	mockNotificationService *notificationmock.NotificationSenderMgtSvcInterfaceMock
	mockEntityTypeService   *entitytypemock.EntityTypeServiceInterfaceMock
	exportService           ExportServiceInterface
	handler                 *exportHandler
}

func (suite *HandlerTestSuite) SetupTest() {
	// Initialize config for tests
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", &config.Config{})
	suite.Require().NoError(err)

	// Setup services and handler
	suite.mockAppService = applicationmock.NewApplicationServiceInterfaceMock(suite.T())
	suite.mockIDPService = idpmock.NewIDPServiceInterfaceMock(suite.T())
	suite.mockNotificationService = notificationmock.NewNotificationSenderMgtSvcInterfaceMock(suite.T())
	suite.mockEntityTypeService = entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T())
	exporters := []declarativeresource.ResourceExporter{
		application.NewApplicationExporterForTest(suite.mockAppService),
		connection.NewConnectionExporterForTest(suite.mockIDPService, suite.mockNotificationService),
		entitytype.NewEntityTypeExporterForTest(suite.mockEntityTypeService, entitytype.TypeCategoryUser),
	}
	parameterizer := newParameterizer(templatingRules{})
	suite.exportService = newExportService(exporters, parameterizer)
	suite.handler = newExportHandler(suite.exportService)
}

func (suite *HandlerTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func TestHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(HandlerTestSuite))
}

// TestNewExportHandler tests the handler constructor.
func TestNewExportHandler(t *testing.T) {
	mockAppService := applicationmock.NewApplicationServiceInterfaceMock(t)
	mockIDPService := idpmock.NewIDPServiceInterfaceMock(t)
	mockNotificationService := notificationmock.NewNotificationSenderMgtSvcInterfaceMock(t)
	mockEntityTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	exporters := []declarativeresource.ResourceExporter{
		application.NewApplicationExporterForTest(mockAppService),
		connection.NewConnectionExporterForTest(mockIDPService, mockNotificationService),
		entitytype.NewEntityTypeExporterForTest(mockEntityTypeService, entitytype.TypeCategoryUser),
	}
	parameterizer := newParameterizer(templatingRules{})
	exportService := newExportService(exporters, parameterizer)

	handler := newExportHandler(exportService)

	assert.NotNil(t, handler)
	assert.Equal(t, exportService, handler.service)
}

// Handler Function Tests

// TestHandleExportRequest_Success tests successful JSON export on the /export endpoint.
func (suite *HandlerTestSuite) TestHandleExportRequest_Success() {
	// Setup mock expectations
	suite.mockAppService.EXPECT().GetApplication(mock.Anything, "app1").Return(&providers.Application{
		ID:          "app1",
		Name:        "Test App 1",
		Description: "Test Application 1",
		URL:         "https://example.com",
	}, nil).Once()

	// Create request body
	requestBody := &ExportRequest{
		Applications: []string{"app1"},
		Options: &ExportOptions{
			Format:          "yaml",
			IncludeMetadata: true,
		},
	}
	requestJSON, _ := json.Marshal(requestBody)

	// Create HTTP request
	req := httptest.NewRequest("POST", "/export", bytes.NewReader(requestJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	suite.handler.HandleExportRequest(w, req)

	// Assert response
	assert.Equal(suite.T(), http.StatusOK, w.Code)
	assert.Equal(suite.T(), "application/json", w.Header().Get("Content-Type"))

	var response JSONExportResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), response.Resources, "# File: Test_App_1.yaml")
	assert.Contains(suite.T(), response.Resources, "resource_type: application")
	assert.Contains(suite.T(), response.Resources, "name: Test App 1")
	// The home URL is environment specific, so it is exported as a placeholder with its current value
	// in the sidecar for the operator to override per deployment.
	assert.Equal(suite.T(), "APPLICATION_TEST_APP_1_URL=https://example.com\n", response.EnvironmentVariables)
}

// TestHandleExportRequest_InvalidJSON tests invalid JSON request handling.
func (suite *HandlerTestSuite) TestHandleExportRequest_InvalidJSON() {
	// Create malformed JSON request
	req := httptest.NewRequest("POST", "/export", strings.NewReader("{invalid json}"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	suite.handler.HandleExportRequest(w, req)

	// Assert error response
	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	assert.Equal(suite.T(), "application/json", w.Header().Get("Content-Type"))

	var errResp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &errResp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "EXP-1001", errResp["code"])
	assert.Equal(suite.T(), "Invalid export request", errResp["message"].(map[string]interface{})["defaultValue"])
}

// Helper function to test service error responses
func (suite *HandlerTestSuite) testServiceErrorResponse(
	method, endpoint, appID string, serviceError *tidcommon.ServiceError, expectedErrorCode string) {
	// Setup mock to return service error
	suite.mockAppService.EXPECT().GetApplication(mock.Anything, appID).Return(nil, serviceError).Once()

	// Create request body
	requestBody := &ExportRequest{
		Applications: []string{appID},
	}
	requestJSON, _ := json.Marshal(requestBody)

	// Create HTTP request
	req := httptest.NewRequest(method, endpoint, bytes.NewReader(requestJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute based on endpoint
	if endpoint == "/export" {
		suite.handler.HandleExportRequest(w, req)
	}

	// Assert error response
	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	assert.Equal(suite.T(), "application/json", w.Header().Get("Content-Type"))

	var errResp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &errResp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), expectedErrorCode, errResp["code"])
}

// TestHandleExportRequest_ServiceError tests service error handling.
func (suite *HandlerTestSuite) TestHandleExportRequest_ServiceError() {
	suite.testServiceErrorResponse("POST", "/export", "app1", &ErrorNoResourcesFound, "EXP-1002")
}

// TestHandleExportRequest_MultipleFiles tests JSON export with multiple files.
func (suite *HandlerTestSuite) TestHandleExportRequest_MultipleFiles() {
	// Setup mock expectations for multiple applications
	suite.mockAppService.EXPECT().GetApplication(mock.Anything, "app1").Return(&providers.Application{
		ID:   "app1",
		Name: "App One",
	}, nil).Once()
	suite.mockAppService.EXPECT().GetApplication(mock.Anything, "app2").Return(&providers.Application{
		ID:   "app2",
		Name: "App Two",
	}, nil).Once()

	// Create request body
	requestBody := &ExportRequest{
		Applications: []string{"app1", "app2"},
	}
	requestJSON, _ := json.Marshal(requestBody)

	// Create HTTP request
	req := httptest.NewRequest("POST", "/export", bytes.NewReader(requestJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	suite.handler.HandleExportRequest(w, req)

	// Assert response
	assert.Equal(suite.T(), http.StatusOK, w.Code)
	assert.Equal(suite.T(), "application/json", w.Header().Get("Content-Type"))

	var response JSONExportResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), response.Resources, "# File: App_One.yaml")
	assert.Contains(suite.T(), response.Resources, "# File: App_Two.yaml")
	assert.Contains(suite.T(), response.Resources, "name: App One")
	assert.Contains(suite.T(), response.Resources, "name: App Two")
	assert.Contains(suite.T(), response.Resources, "---")
	assert.Equal(suite.T(), "", response.EnvironmentVariables)

	resourceTypeHeaders := strings.Count(response.Resources, "resource_type: application")
	assert.Equal(suite.T(), 2, resourceTypeHeaders)
}

// TestHandleExportJSONRequest_Success tests successful JSON export.
func (suite *HandlerTestSuite) TestHandleExportJSONRequest_Success() {
	// Setup mock expectations
	suite.mockAppService.EXPECT().GetApplication(mock.Anything, "app1").Return(&providers.Application{
		ID:          "app1",
		Name:        "Test App JSON",
		Description: "JSON Test Application",
	}, nil).Once()

	// Create request body
	requestBody := &ExportRequest{
		Applications: []string{"app1"},
		Options: &ExportOptions{
			Format: "json", // Note: JSON format currently falls back to YAML
		},
	}
	requestJSON, _ := json.Marshal(requestBody)

	// Create HTTP request
	req := httptest.NewRequest("POST", "/export", bytes.NewReader(requestJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	suite.handler.HandleExportRequest(w, req)

	// Assert response
	assert.Equal(suite.T(), http.StatusOK, w.Code)
	assert.Equal(suite.T(), "application/json", w.Header().Get("Content-Type"))

	var response JSONExportResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), response.Resources, "# File: Test_App_JSON.yaml")
	assert.Contains(suite.T(), response.Resources, "name: Test App JSON")
	assert.Equal(suite.T(), "", response.EnvironmentVariables)
}

// TestHandleExportJSONRequest_InvalidJSON tests invalid JSON handling for JSON export.
func (suite *HandlerTestSuite) TestHandleExportJSONRequest_InvalidJSON() {
	// Create malformed JSON request
	req := httptest.NewRequest("POST", "/export", strings.NewReader("invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	suite.handler.HandleExportRequest(w, req)

	// Assert error response
	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	assert.Equal(suite.T(), "application/json", w.Header().Get("Content-Type"))

	var errResp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &errResp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "EXP-1001", errResp["code"])
}

// TestHandleExportJSONRequest_ServiceError tests service error handling for JSON export.
func (suite *HandlerTestSuite) TestHandleExportJSONRequest_ServiceError() {
	// Setup mock to return service error
	suite.testServiceErrorResponse("POST", "/export", "app1", &tidcommon.InternalServerError, "EXP-1002")
}

// TestHandleError_ClientError tests error handling for client errors.
func (suite *HandlerTestSuite) TestHandleError_ClientError() {
	w := httptest.NewRecorder()

	// Create client error
	clientErr := &ErrorNoResourcesFound

	// Execute
	suite.handler.handleError(context.Background(), w, clientErr)

	// Assert response
	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	assert.Equal(suite.T(), "application/json", w.Header().Get("Content-Type"))

	var errResp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &errResp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "EXP-1002", errResp["code"])
	assert.Equal(suite.T(), "No resources found", errResp["message"].(map[string]interface{})["defaultValue"])
	assert.Equal(suite.T(), "No valid resources found for the provided identifiers",
		errResp["description"].(map[string]interface{})["defaultValue"])
}

// TestHandleError_ServerError tests error handling for server errors.
func (suite *HandlerTestSuite) TestHandleError_ServerError() {
	w := httptest.NewRecorder()

	// Create server error
	serverErr := &tidcommon.InternalServerError

	// Execute
	suite.handler.handleError(context.Background(), w, serverErr)

	// Assert response
	assert.Equal(suite.T(), http.StatusInternalServerError, w.Code)
	assert.Equal(suite.T(), "application/json", w.Header().Get("Content-Type"))

	var errResp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &errResp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, errResp["code"])
	assert.Equal(suite.T(), "Internal server error", errResp["message"].(map[string]interface{})["defaultValue"])
	assert.Equal(suite.T(), "An unexpected error occurred while processing the request",
		errResp["description"].(map[string]interface{})["defaultValue"])
}

// Edge case tests

// TestHandleExportRequest_EmptyBody tests empty request body.
func (suite *HandlerTestSuite) TestHandleExportRequest_EmptyBody() {
	req := httptest.NewRequest("POST", "/export", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	suite.handler.HandleExportRequest(w, req)

	// Assert error response
	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	assert.Equal(suite.T(), "application/json", w.Header().Get("Content-Type"))
}

// TestHandleExportRequest_NilOptions tests request with nil options.
func (suite *HandlerTestSuite) TestHandleExportRequest_NilOptions() {
	// Setup mock expectations
	suite.mockAppService.EXPECT().GetApplication(mock.Anything, "app1").Return(&providers.Application{
		ID:   "app1",
		Name: "Test App",
	}, nil).Once()

	// Create request body with nil options
	requestBody := &ExportRequest{
		Applications: []string{"app1"},
		Options:      nil, // Test nil options
	}
	requestJSON, _ := json.Marshal(requestBody)

	// Create HTTP request
	req := httptest.NewRequest("POST", "/export", bytes.NewReader(requestJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	suite.handler.HandleExportRequest(w, req)

	// Assert successful response with default behavior
	assert.Equal(suite.T(), http.StatusOK, w.Code)
	assert.Equal(suite.T(), "application/json", w.Header().Get("Content-Type"))
}

// TestHandleExportJSONRequest_EmptyFiles tests JSON export with no files.
func (suite *HandlerTestSuite) TestHandleExportJSONRequest_EmptyFiles() {
	// Create request body with empty applications
	requestBody := &ExportRequest{
		Applications: []string{}, // No applications
	}
	requestJSON, _ := json.Marshal(requestBody)

	// Create HTTP request
	req := httptest.NewRequest("POST", "/export", bytes.NewReader(requestJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	suite.handler.HandleExportRequest(w, req)

	// Assert error response (empty applications list returns NoResourcesFound error)
	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	assert.Equal(suite.T(), "application/json", w.Header().Get("Content-Type"))

	var errResp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &errResp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "EXP-1002", errResp["code"]) // NoResourcesFound
	assert.Equal(suite.T(), "No resources found", errResp["message"].(map[string]interface{})["defaultValue"])
}

// Benchmark tests

// Helper function for benchmark tests
func setupBenchmarkTest(b *testing.B) (*exportHandler, []byte) {
	// Setup
	config.ResetServerRuntime()
	testConfig := &config.Config{}
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)
	b.Cleanup(func() { config.ResetServerRuntime() })

	mockAppService := applicationmock.NewApplicationServiceInterfaceMock(b)
	mockIDPService := idpmock.NewIDPServiceInterfaceMock(b)
	mockNotificationService := notificationmock.NewNotificationSenderMgtSvcInterfaceMock(b)
	mockEntityTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(b)
	exporters := []declarativeresource.ResourceExporter{
		application.NewApplicationExporterForTest(mockAppService),
		connection.NewConnectionExporterForTest(mockIDPService, mockNotificationService),
		entitytype.NewEntityTypeExporterForTest(mockEntityTypeService, entitytype.TypeCategoryUser),
	}
	parameterizer := newParameterizer(templatingRules{})
	exportService := newExportService(exporters, parameterizer)
	handler := newExportHandler(exportService)

	// Setup mock expectation
	mockAppService.EXPECT().GetApplication(mock.Anything, "benchmark-app").Return(&providers.Application{
		ID:   "benchmark-app",
		Name: "Benchmark Application",
	}, nil).Times(b.N)

	// Create request body
	requestBody := &ExportRequest{
		Applications: []string{"benchmark-app"},
	}
	requestJSON, _ := json.Marshal(requestBody)

	return handler, requestJSON
}

// BenchmarkHandleExportRequest benchmarks YAML export performance.
func BenchmarkHandleExportRequest(b *testing.B) {
	handler, requestJSON := setupBenchmarkTest(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/export", bytes.NewReader(requestJSON))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.HandleExportRequest(w, req)
	}
}

// BenchmarkHandleExportJSONRequest benchmarks JSON export performance.
func BenchmarkHandleExportJSONRequest(b *testing.B) {
	handler, requestJSON := setupBenchmarkTest(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/export", bytes.NewReader(requestJSON))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.HandleExportRequest(w, req)
	}
}
