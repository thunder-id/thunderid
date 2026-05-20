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

package executor

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	i18ncore "github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
)

type HTTPRequestExecutorTestSuite struct {
	suite.Suite
	executor   *httpRequestExecutor
	mockServer *httptest.Server
}

func TestHTTPRequestExecutorTestSuite(t *testing.T) {
	suite.Run(t, new(HTTPRequestExecutorTestSuite))
}

func (suite *HTTPRequestExecutorTestSuite) SetupSuite() {
	_ = config.InitializeServerRuntime("test", &config.Config{})
}

func (suite *HTTPRequestExecutorTestSuite) TearDownSuite() {
	config.ResetServerRuntime()
}

func (suite *HTTPRequestExecutorTestSuite) SetupTest() {
	mockFlowFactory := coremock.NewFlowFactoryInterfaceMock(suite.T())
	mockFlowFactory.On("CreateExecutor", ExecutorNameHTTPRequest, common.ExecutorTypeUtility,
		[]common.Input{}, []common.Input{}).
		Return(newMockExecutor(ExecutorNameHTTPRequest, common.ExecutorTypeUtility, []common.Input{}, []common.Input{}))
	suite.executor = newHTTPRequestExecutor(mockFlowFactory, nil)
}

func (suite *HTTPRequestExecutorTestSuite) TearDownTest() {
	if suite.mockServer != nil {
		suite.mockServer.Close()
		suite.mockServer = nil
	}
}

func (suite *HTTPRequestExecutorTestSuite) TestResolvePlaceholdersInConfig() {
	var receivedURL string
	var receivedHeaders http.Header
	var receivedBody map[string]interface{}

	suite.mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedURL = r.URL.Path
		receivedHeaders = r.Header
		err := json.NewDecoder(r.Body).Decode(&receivedBody)
		if err != nil {
			receivedBody = nil
		}
		w.WriteHeader(http.StatusOK)
	}))

	ctx := &core.NodeContext{
		ExecutionID: "test-flow",
		UserInputs: map[string]string{
			"username": "testuser",
			"email":    "test@example.com",
		},
		RuntimeData: map[string]string{
			"sessionId": "session-123",
			"orgId":     "org-456",
		},
		NodeProperties: map[string]interface{}{
			"url":    suite.mockServer.URL + "/api/users/{{ context.username }}",
			"method": "POST",
			"headers": map[string]interface{}{
				"X-Session-Id": "{{ context.sessionId }}",
				"X-Org-Id":     "{{ context.orgId }}",
			},
			"body": map[string]interface{}{
				"user":  "{{ context.username }}",
				"email": "{{ context.email }}",
			},
		},
	}

	execResp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, execResp.Status)

	// Verify URL placeholder was resolved
	assert.Equal(suite.T(), "/api/users/testuser", receivedURL)

	// Verify header placeholders were resolved
	assert.Equal(suite.T(), "session-123", receivedHeaders.Get("X-Session-Id"))
	assert.Equal(suite.T(), "org-456", receivedHeaders.Get("X-Org-Id"))

	// Verify body placeholders were resolved
	assert.Equal(suite.T(), "testuser", receivedBody["user"])
	assert.Equal(suite.T(), "test@example.com", receivedBody["email"])
}

func (suite *HTTPRequestExecutorTestSuite) TestResolvePlaceholderUserIDSpecialHandling() {
	// Test that userID is resolved correctly from AuthenticatedUser
	var receivedBody map[string]interface{}

	suite.mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := json.NewDecoder(r.Body).Decode(&receivedBody)
		if err != nil {
			receivedBody = nil
		}
		w.WriteHeader(http.StatusOK)
	}))

	ctx := &core.NodeContext{
		ExecutionID: "test-flow",
		UserInputs: map[string]string{
			"userId": "input-user-id", // This should NOT be used for userId
		},
		RuntimeData: map[string]string{},
		AuthenticatedUser: authncm.AuthenticatedUser{
			UserID: "auth-user-456",
		},
		NodeProperties: map[string]interface{}{
			"url":    suite.mockServer.URL + "/api/user",
			"method": "POST",
			"body": map[string]interface{}{
				"userId": "{{ context.userId }}",
			},
		},
	}

	execResp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, execResp.Status)

	// userId should be resolved from AuthenticatedUser, not from UserInputs
	assert.Equal(suite.T(), "auth-user-456", receivedBody["userId"])
}

func (suite *HTTPRequestExecutorTestSuite) TestResolvePlaceholderRuntimeDataPrecedence() {
	// Test that RuntimeData takes precedence over UserInputs
	var receivedBody map[string]interface{}

	suite.mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := json.NewDecoder(r.Body).Decode(&receivedBody)
		if err != nil {
			receivedBody = nil
		}
		w.WriteHeader(http.StatusOK)
	}))

	ctx := &core.NodeContext{
		ExecutionID: "test-flow",
		UserInputs: map[string]string{
			"key": "user-input-value",
		},
		RuntimeData: map[string]string{
			"key": "runtime-value",
		},
		NodeProperties: map[string]interface{}{
			"url":    suite.mockServer.URL + "/api/test",
			"method": "POST",
			"body": map[string]interface{}{
				"value": "{{ context.key }}",
			},
		},
	}

	execResp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, execResp.Status)

	// RuntimeData should take precedence
	assert.Equal(suite.T(), "runtime-value", receivedBody["value"])
}

func (suite *HTTPRequestExecutorTestSuite) TestResolvePlaceholderNonExistentKey() {
	// Test that non-existent keys keep the placeholder as-is
	var receivedBody map[string]interface{}

	suite.mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := json.NewDecoder(r.Body).Decode(&receivedBody)
		if err != nil {
			receivedBody = nil
		}
		w.WriteHeader(http.StatusOK)
	}))

	ctx := &core.NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{},
		NodeProperties: map[string]interface{}{
			"url":    suite.mockServer.URL + "/api/test",
			"method": "POST",
			"body": map[string]interface{}{
				"value": "{{ context.nonexistent }}",
			},
		},
	}

	execResp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, execResp.Status)

	// Non-existent key should keep placeholder
	assert.Equal(suite.T(), "{{ context.nonexistent }}", receivedBody["value"])
}

func (suite *HTTPRequestExecutorTestSuite) TestResolveMapPlaceholders() {
	ctx := &core.NodeContext{
		ExecutionID: "test-flow",
		UserInputs: map[string]string{
			"username": "testuser",
			"email":    "test@example.com",
		},
		RuntimeData: map[string]string{
			"orgId": "org-123",
		},
	}

	input := map[string]interface{}{
		"user": map[string]interface{}{
			"name":  "{{ context.username }}",
			"email": "{{ context.email }}",
			"metadata": map[string]interface{}{
				"orgId":  "{{ context.orgId }}",
				"static": "value",
			},
		},
		"items": []interface{}{
			"{{ context.username }}",
			"static",
			map[string]interface{}{
				"nested": "{{ context.email }}",
			},
		},
	}

	result := suite.executor.resolveMapPlaceholders(ctx, input)

	expected := map[string]interface{}{
		"user": map[string]interface{}{
			"name":  "testuser",
			"email": "test@example.com",
			"metadata": map[string]interface{}{
				"orgId":  "org-123",
				"static": "value",
			},
		},
		"items": []interface{}{
			"testuser",
			"static",
			map[string]interface{}{
				"nested": "test@example.com",
			},
		},
	}

	assert.Equal(suite.T(), expected, result)
}

func (suite *HTTPRequestExecutorTestSuite) TestExecute_SuccessfulGETRequest() {
	// Setup mock server
	suite.mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(suite.T(), "GET", r.Method)
		assert.Equal(suite.T(), "/api/users/123", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		err := json.NewEncoder(w).Encode(map[string]interface{}{
			"id":   "123",
			"name": "Test User",
		})
		assert.NoError(suite.T(), err, "Failed to encode mock response")
	}))

	responseMappingJSON := `{"id": "response.data.id", "name": "response.data.name"}`

	ctx := &core.NodeContext{
		ExecutionID: "test-flow",
		NodeProperties: map[string]interface{}{
			"url":             suite.mockServer.URL + "/api/users/123",
			"method":          "GET",
			"responseMapping": responseMappingJSON,
		},
		UserInputs:  make(map[string]string),
		RuntimeData: make(map[string]string),
	}

	execResp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, execResp.Status)
	assert.Equal(suite.T(), "123", execResp.RuntimeData["id"])
	assert.Equal(suite.T(), "Test User", execResp.RuntimeData["name"])
}

func (suite *HTTPRequestExecutorTestSuite) TestExecute_SuccessfulPOSTRequest() {
	var receivedBody map[string]interface{}
	var receivedHeaders http.Header

	suite.mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(suite.T(), "POST", r.Method)

		receivedHeaders = r.Header
		err := json.NewDecoder(r.Body).Decode(&receivedBody)
		assert.NoError(suite.T(), err, "Failed to decode request body")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		err = json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "created",
			"userId": "new-user-123",
		})
		assert.NoError(suite.T(), err, "Failed to encode mock response")
	}))

	bodyJSON := `{"username": "{{ context.username }}", "email": "{{ context.email }}"}`
	headersJSON := `{"Authorization": "Bearer token123", "X-Custom-Header": "{{ context.customValue }}"}`
	responseMappingJSON := `{"status": "response.data.status", "userId": "response.data.userId"}`

	ctx := &core.NodeContext{
		ExecutionID: "test-flow",
		NodeProperties: map[string]interface{}{
			"url":             suite.mockServer.URL + "/api/users",
			"method":          "POST",
			"body":            bodyJSON,
			"headers":         headersJSON,
			"responseMapping": responseMappingJSON,
		},
		UserInputs: map[string]string{
			"username":    "newuser",
			"email":       "newuser@example.com",
			"customValue": "custom123",
		},
		RuntimeData: make(map[string]string),
	}

	execResp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, execResp.Status)
	assert.Equal(suite.T(), "created", execResp.RuntimeData["status"])
	assert.Equal(suite.T(), "new-user-123", execResp.RuntimeData["userId"])

	// Verify received body
	assert.Equal(suite.T(), "newuser", receivedBody["username"])
	assert.Equal(suite.T(), "newuser@example.com", receivedBody["email"])

	// Verify headers
	assert.Equal(suite.T(), "Bearer token123", receivedHeaders.Get("Authorization"))
	assert.Equal(suite.T(), "custom123", receivedHeaders.Get("X-Custom-Header"))
}

func (suite *HTTPRequestExecutorTestSuite) TestExecute_ResponseMapping() {
	suite.mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		err := json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"userId":     "user-789",
				"profileUrl": "https://example.com/profile",
			},
			"metadata": map[string]interface{}{
				"timestamp": "2025-11-12T10:00:00Z",
			},
		})
		assert.NoError(suite.T(), err, "Failed to encode mock response")
	}))

	responseMappingJSON := `{"externalUserId": "response.data.data.userId", 
	"profileUrl": "response.data.data.profileUrl", "timestamp": "response.data.metadata.timestamp"}`

	ctx := &core.NodeContext{
		ExecutionID: "test-flow",
		NodeProperties: map[string]interface{}{
			"url":             suite.mockServer.URL + "/api/data",
			"responseMapping": responseMappingJSON,
		},
		UserInputs:  make(map[string]string),
		RuntimeData: make(map[string]string),
	}

	execResp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, execResp.Status)
	assert.Equal(suite.T(), "user-789", execResp.RuntimeData["externalUserId"])
	assert.Equal(suite.T(), "https://example.com/profile", execResp.RuntimeData["profileUrl"])
	assert.Equal(suite.T(), "2025-11-12T10:00:00Z", execResp.RuntimeData["timestamp"])
	// Original keys should not be present when mapping is specified
	assert.Empty(suite.T(), execResp.RuntimeData["data"])
}

func (suite *HTTPRequestExecutorTestSuite) TestExecute_DefaultMethod() {
	suite.mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(suite.T(), "GET", r.Method)
		w.WriteHeader(http.StatusOK)
	}))

	ctx := &core.NodeContext{
		ExecutionID: "test-flow",
		NodeProperties: map[string]interface{}{
			"url": suite.mockServer.URL + "/api/test",
			// method not specified, should default to GET
		},
		UserInputs:  make(map[string]string),
		RuntimeData: make(map[string]string),
	}

	execResp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, execResp.Status)
}

func (suite *HTTPRequestExecutorTestSuite) TestExecute_ErrorHandling_FailOnErrorFalse() {
	suite.mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte("Internal Server Error"))
		assert.NoError(suite.T(), err, "Failed to write mock error response")
	}))

	ctx := &core.NodeContext{
		ExecutionID: "test-flow",
		NodeProperties: map[string]interface{}{
			"url": suite.mockServer.URL + "/api/error",
		},
		UserInputs:  make(map[string]string),
		RuntimeData: make(map[string]string),
	}

	execResp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	// Should complete without failure when failOnError defaults to false
	assert.Equal(suite.T(), common.ExecComplete, execResp.Status)
}

func (suite *HTTPRequestExecutorTestSuite) TestExecute_ErrorHandling_FailOnErrorTrue() {
	suite.mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte("Bad Request"))
		assert.NoError(suite.T(), err, "Failed to write mock error response")
	}))

	errorHandlingJSON := `{"failOnError": true}`

	ctx := &core.NodeContext{
		ExecutionID: "test-flow",
		NodeProperties: map[string]interface{}{
			"url":           suite.mockServer.URL + "/api/error",
			"errorHandling": errorHandlingJSON,
		},
		UserInputs:  make(map[string]string),
		RuntimeData: make(map[string]string),
	}

	execResp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecFailure, execResp.Status)
	assert.Contains(suite.T(), execResp.FailureReason, "HTTP request failed with status 400")
}

func (suite *HTTPRequestExecutorTestSuite) TestExecute_MissingURL() {
	ctx := &core.NodeContext{
		ExecutionID: "test-flow",
		NodeProperties: map[string]interface{}{
			// URL is missing
			"method": "GET",
		},
		UserInputs:  make(map[string]string),
		RuntimeData: make(map[string]string),
	}

	execResp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	// Configuration errors always fail the flow regardless of failOnError setting
	assert.Equal(suite.T(), common.ExecFailure, execResp.Status)
	assert.Contains(suite.T(), execResp.FailureReason, "url is required")
}

func (suite *HTTPRequestExecutorTestSuite) TestExecute_InvalidHTTPMethod() {
	ctx := &core.NodeContext{
		ExecutionID: "test-flow",
		NodeProperties: map[string]interface{}{
			"url":    "https://example.com/api/test",
			"method": "INVALID",
		},
		UserInputs:  make(map[string]string),
		RuntimeData: make(map[string]string),
	}

	execResp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	// Configuration errors always fail the flow regardless of failOnError setting
	assert.Equal(suite.T(), common.ExecFailure, execResp.Status)
	assert.Contains(suite.T(), execResp.FailureReason, "invalid HTTP method")
}

func (suite *HTTPRequestExecutorTestSuite) TestParseAndValidateConfig_TimeoutLimits() {
	// Test timeout exceeding maximum
	properties := map[string]interface{}{
		"url":     "https://example.com/api/test",
		"timeout": "60", // Exceeds max of 30
	}

	config, err := suite.executor.parseAndValidateConfig(properties)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), maxHTTPRequestTimeout, config.Timeout, "Timeout should be capped at maximum")

	// Test default timeout
	properties2 := map[string]interface{}{
		"url": "https://example.com/api/test",
	}

	config2, err := suite.executor.parseAndValidateConfig(properties2)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), defaultHTTPTimeout, config2.Timeout)
}

func (suite *HTTPRequestExecutorTestSuite) TestParseAndValidateConfig_RetryLimits() {
	errorHandlingJSON := `{"retryCount": 10, "retryDelay": 10000}`

	properties := map[string]interface{}{
		"url":           "https://example.com/api/test",
		"errorHandling": errorHandlingJSON,
	}

	config, err := suite.executor.parseAndValidateConfig(properties)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), maxHTTPRequestRetryCount, config.ErrorHandling.RetryCount)
	assert.Equal(suite.T(), maxHTTPRequestRetryDelay, config.ErrorHandling.RetryDelay)
}

func (suite *HTTPRequestExecutorTestSuite) TestExecute_AllHTTPMethods() {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}

	for _, method := range methods {
		suite.Run("Method_"+method, func() {
			suite.mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(suite.T(), method, r.Method)
				w.WriteHeader(http.StatusOK)
			}))
			defer suite.mockServer.Close()

			ctx := &core.NodeContext{
				ExecutionID: "test-flow",
				NodeProperties: map[string]interface{}{
					"url":    suite.mockServer.URL + "/api/test",
					"method": method,
				},
				UserInputs:  make(map[string]string),
				RuntimeData: make(map[string]string),
			}

			execResp, err := suite.executor.Execute(ctx)

			assert.NoError(suite.T(), err)
			assert.Equal(suite.T(), common.ExecComplete, execResp.Status)
		})
	}
}

func (suite *HTTPRequestExecutorTestSuite) TestExtractValueFromPath() {
	data := map[string]interface{}{
		"user": map[string]interface{}{
			"id":   "123",
			"name": "Test User",
			"profile": map[string]interface{}{
				"email": "test@example.com",
			},
		},
		"count": 42,
	}

	tests := []struct {
		name     string
		path     string
		expected interface{}
	}{
		{
			name:     "Top level string",
			path:     "count",
			expected: 42,
		},
		{
			name:     "Nested string",
			path:     "user.id",
			expected: "123",
		},
		{
			name:     "Deeply nested string",
			path:     "user.profile.email",
			expected: "test@example.com",
		},
		{
			name:     "Non-existent path",
			path:     "user.nonexistent",
			expected: nil,
		},
		{
			name:     "Empty path",
			path:     "",
			expected: data,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			result := suite.executor.extractValueFromPath(data, tt.path)
			assert.Equal(suite.T(), tt.expected, result)
		})
	}
}

func (suite *HTTPRequestExecutorTestSuite) TestExecute_NonJSONResponse() {
	suite.mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("Plain text response"))
		assert.NoError(suite.T(), err, "Failed to write mock plain text response")
	}))

	responseMappingJSON := `{"raw": "response.data.raw"}`

	ctx := &core.NodeContext{
		ExecutionID: "test-flow",
		NodeProperties: map[string]interface{}{
			"url":             suite.mockServer.URL + "/api/text",
			"responseMapping": responseMappingJSON,
		},
		UserInputs:  make(map[string]string),
		RuntimeData: make(map[string]string),
	}

	execResp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, execResp.Status)
	assert.Equal(suite.T(), "Plain text response", execResp.RuntimeData["raw"])
}

func (suite *HTTPRequestExecutorTestSuite) TestExecute_ResponseStatusExtraction() {
	suite.mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		err := json.NewEncoder(w).Encode(map[string]interface{}{
			"id":      "123",
			"message": "Resource created",
		})
		assert.NoError(suite.T(), err, "Failed to encode mock response")
	}))

	responseMappingJSON := `{"resourceId": "response.data.id", "statusCode": "response.status"}`

	ctx := &core.NodeContext{
		ExecutionID: "test-flow",
		NodeProperties: map[string]interface{}{
			"url":             suite.mockServer.URL + "/api/resource",
			"method":          "POST",
			"responseMapping": responseMappingJSON,
		},
		UserInputs:  make(map[string]string),
		RuntimeData: make(map[string]string),
	}

	execResp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, execResp.Status)
	assert.Equal(suite.T(), "123", execResp.RuntimeData["resourceId"])
	assert.Equal(suite.T(), "201", execResp.RuntimeData["statusCode"])
}

func (suite *HTTPRequestExecutorTestSuite) TestEnrichOURuntimeData_OUIDFromAuthenticatedUser() {
	// When AuthenticatedUser.OUID is set, it should be used to fetch OU details
	// and ouHandle/ouName should be enriched into RuntimeData before placeholder resolution.
	var receivedBody map[string]interface{}

	suite.mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := json.NewDecoder(r.Body).Decode(&receivedBody)
		if err != nil {
			receivedBody = nil
		}
		w.WriteHeader(http.StatusOK)
	}))

	mockOUService := oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())
	mockOUService.On("GetOrganizationUnit", mock.Anything, "ou-auth-123").
		Return(ou.OrganizationUnit{
			ID:          "ou-auth-123",
			Handle:      "acme-corp",
			Name:        "Acme Corporation",
			Description: "Acme Corporation description",
		}, nil)

	mockFlowFactory := coremock.NewFlowFactoryInterfaceMock(suite.T())
	mockFlowFactory.On("CreateExecutor", ExecutorNameHTTPRequest, common.ExecutorTypeUtility,
		[]common.Input{}, []common.Input{}).
		Return(newMockExecutor(ExecutorNameHTTPRequest, common.ExecutorTypeUtility, []common.Input{}, []common.Input{}))
	executor := newHTTPRequestExecutor(mockFlowFactory, mockOUService)

	ctx := &core.NodeContext{
		ExecutionID: "test-flow",
		AuthenticatedUser: authncm.AuthenticatedUser{
			OUID: "ou-auth-123",
		},
		RuntimeData: map[string]string{},
		UserInputs:  map[string]string{},
		NodeProperties: map[string]interface{}{
			"url":    suite.mockServer.URL + "/api/enrich",
			"method": "POST",
			"body": map[string]interface{}{
				"orgHandle":      "{{ context.ouHandle }}",
				"orgName":        "{{ context.ouName }}",
				"orgDescription": "{{ context.ouDescription }}",
			},
		},
	}

	execResp, err := executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, execResp.Status)
	assert.Equal(suite.T(), "acme-corp", receivedBody["orgHandle"])
	assert.Equal(suite.T(), "Acme Corporation", receivedBody["orgName"])
	assert.Equal(suite.T(), "Acme Corporation description", receivedBody["orgDescription"])
}

func (suite *HTTPRequestExecutorTestSuite) TestEnrichOURuntimeData_FallbackToRuntimeData() {
	// When AuthenticatedUser.OUID is absent, RuntimeData["ouId"] should be used to fetch OU details.
	var receivedBody map[string]interface{}

	suite.mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := json.NewDecoder(r.Body).Decode(&receivedBody)
		if err != nil {
			receivedBody = nil
		}
		w.WriteHeader(http.StatusOK)
	}))

	mockOUService := oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())
	mockOUService.On("GetOrganizationUnit", mock.Anything, "ou-runtime-456").
		Return(ou.OrganizationUnit{
			ID:          "ou-runtime-456",
			Handle:      "beta-org",
			Name:        "Beta Organization",
			Description: "Beta Organization description",
		}, nil)

	mockFlowFactory := coremock.NewFlowFactoryInterfaceMock(suite.T())
	mockFlowFactory.On("CreateExecutor", ExecutorNameHTTPRequest, common.ExecutorTypeUtility,
		[]common.Input{}, []common.Input{}).
		Return(newMockExecutor(ExecutorNameHTTPRequest, common.ExecutorTypeUtility, []common.Input{}, []common.Input{}))
	executor := newHTTPRequestExecutor(mockFlowFactory, mockOUService)

	ctx := &core.NodeContext{
		ExecutionID:       "test-flow",
		AuthenticatedUser: authncm.AuthenticatedUser{
			// OUID is empty — falls back to RuntimeData["ouId"]
		},
		RuntimeData: map[string]string{
			"ouId": "ou-runtime-456",
		},
		UserInputs: map[string]string{},
		NodeProperties: map[string]interface{}{
			"url":    suite.mockServer.URL + "/api/enrich",
			"method": "POST",
			"body": map[string]interface{}{
				"handle":      "{{ context.ouHandle }}",
				"name":        "{{ context.ouName }}",
				"description": "{{ context.ouDescription }}",
			},
		},
	}

	execResp, err := executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, execResp.Status)
	assert.Equal(suite.T(), "beta-org", receivedBody["handle"])
	assert.Equal(suite.T(), "Beta Organization", receivedBody["name"])
	assert.Equal(suite.T(), "Beta Organization description", receivedBody["description"])
}

func (suite *HTTPRequestExecutorTestSuite) TestEnrichOURuntimeData_AuthenticatedUserOUIDPreferredOverRuntimeData() {
	// When both AuthenticatedUser.OUID and RuntimeData["ouId"] are set,
	// AuthenticatedUser.OUID should be preferred.
	var receivedBody map[string]interface{}

	suite.mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := json.NewDecoder(r.Body).Decode(&receivedBody)
		if err != nil {
			receivedBody = nil
		}
		w.WriteHeader(http.StatusOK)
	}))

	mockOUService := oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())
	// Only the AuthenticatedUser.OUID should trigger a lookup — not the RuntimeData fallback.
	mockOUService.On("GetOrganizationUnit", mock.Anything, "ou-auth-primary").
		Return(ou.OrganizationUnit{
			ID:     "ou-auth-primary",
			Handle: "primary-handle",
			Name:   "Primary Org",
		}, nil)

	mockFlowFactory := coremock.NewFlowFactoryInterfaceMock(suite.T())
	mockFlowFactory.On("CreateExecutor", ExecutorNameHTTPRequest, common.ExecutorTypeUtility,
		[]common.Input{}, []common.Input{}).
		Return(newMockExecutor(ExecutorNameHTTPRequest, common.ExecutorTypeUtility, []common.Input{}, []common.Input{}))
	executor := newHTTPRequestExecutor(mockFlowFactory, mockOUService)

	ctx := &core.NodeContext{
		ExecutionID: "test-flow",
		AuthenticatedUser: authncm.AuthenticatedUser{
			OUID: "ou-auth-primary",
		},
		RuntimeData: map[string]string{
			"ouId": "ou-runtime-fallback", // should NOT be used
		},
		UserInputs: map[string]string{},
		NodeProperties: map[string]interface{}{
			"url":    suite.mockServer.URL + "/api/enrich",
			"method": "POST",
			"body": map[string]interface{}{
				"handle": "{{ context.ouHandle }}",
			},
		},
	}

	execResp, err := executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, execResp.Status)
	assert.Equal(suite.T(), "primary-handle", receivedBody["handle"])
	// The mock expectation for "ou-auth-primary" (not "ou-runtime-fallback") asserts the correct OU was fetched.
	mockOUService.AssertNotCalled(suite.T(), "GetOrganizationUnit", mock.Anything, "ou-runtime-fallback")
}

func (suite *HTTPRequestExecutorTestSuite) TestEnrichOURuntimeData_OverwritesExistingRuntimeData() {
	// When ctx.RuntimeData["ouHandle"] is pre-populated, enrichOURuntimeData must overwrite it
	// with the value fetched from the OU service.
	var receivedBody map[string]interface{}

	suite.mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := json.NewDecoder(r.Body).Decode(&receivedBody)
		if err != nil {
			receivedBody = nil
		}
		w.WriteHeader(http.StatusOK)
	}))

	mockOUService := oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())
	mockOUService.On("GetOrganizationUnit", mock.Anything, "ou-overwrite-test").
		Return(ou.OrganizationUnit{
			ID:     "ou-overwrite-test",
			Handle: "fetched-handle",
			Name:   "Fetched Org",
		}, nil)

	mockFlowFactory := coremock.NewFlowFactoryInterfaceMock(suite.T())
	mockFlowFactory.On("CreateExecutor", ExecutorNameHTTPRequest, common.ExecutorTypeUtility,
		[]common.Input{}, []common.Input{}).
		Return(newMockExecutor(ExecutorNameHTTPRequest, common.ExecutorTypeUtility, []common.Input{}, []common.Input{}))
	executor := newHTTPRequestExecutor(mockFlowFactory, mockOUService)

	ctx := &core.NodeContext{
		ExecutionID: "test-flow",
		AuthenticatedUser: authncm.AuthenticatedUser{
			OUID: "ou-overwrite-test",
		},
		RuntimeData: map[string]string{
			"ouHandle": "stale-handle", // pre-populated — must be overwritten
		},
		UserInputs: map[string]string{},
		NodeProperties: map[string]interface{}{
			"url":    suite.mockServer.URL + "/api/enrich",
			"method": "POST",
			"body": map[string]interface{}{
				"handle": "{{ context.ouHandle }}",
			},
		},
	}

	execResp, err := executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, execResp.Status)
	// The fetched value must overwrite the stale preset value.
	assert.Equal(suite.T(), "fetched-handle", receivedBody["handle"])
}

func (suite *HTTPRequestExecutorTestSuite) TestEnrichOURuntimeData_OULookupFailure_GracefulDegradation() {
	// When OU lookup fails, execution should continue (graceful degradation):
	// a warning is logged but the executor completes, leaving placeholders unresolved.
	var receivedBody map[string]interface{}

	suite.mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := json.NewDecoder(r.Body).Decode(&receivedBody)
		if err != nil {
			receivedBody = nil
		}
		w.WriteHeader(http.StatusOK)
	}))

	mockOUService := oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())
	mockOUService.On("GetOrganizationUnit", mock.Anything, "ou-not-found").
		Return(ou.OrganizationUnit{}, &serviceerror.ServiceError{
			Error:            i18ncore.I18nMessage{DefaultValue: "ou_not_found"},
			ErrorDescription: i18ncore.I18nMessage{DefaultValue: "organization unit not found"},
		})

	mockFlowFactory := coremock.NewFlowFactoryInterfaceMock(suite.T())
	mockFlowFactory.On("CreateExecutor", ExecutorNameHTTPRequest, common.ExecutorTypeUtility,
		[]common.Input{}, []common.Input{}).
		Return(newMockExecutor(ExecutorNameHTTPRequest, common.ExecutorTypeUtility, []common.Input{}, []common.Input{}))
	executor := newHTTPRequestExecutor(mockFlowFactory, mockOUService)

	ctx := &core.NodeContext{
		ExecutionID: "test-flow",
		AuthenticatedUser: authncm.AuthenticatedUser{
			OUID: "ou-not-found",
		},
		RuntimeData: map[string]string{},
		UserInputs:  map[string]string{},
		NodeProperties: map[string]interface{}{
			"url":    suite.mockServer.URL + "/api/enrich",
			"method": "POST",
			"body": map[string]interface{}{
				"orgHandle": "{{ context.ouHandle }}",
			},
		},
	}

	execResp, err := executor.Execute(ctx)

	// Execution must complete despite the OU lookup failure.
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, execResp.Status)
	// ouHandle placeholder remains unresolved because enrichment failed.
	assert.Equal(suite.T(), "{{ context.ouHandle }}", receivedBody["orgHandle"])
}
