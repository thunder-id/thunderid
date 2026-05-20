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

package authentication

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

const (
	mockHTTPServerPort = 9091
)

var (
	httpRequestExecutorFlow = testutils.Flow{
		Name:     "HTTP Request Executor Auth Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_http_request",
		Nodes: []map[string]interface{}{
			{
				"id":        "start",
				"type":      "START",
				"onSuccess": "basic_auth",
			},
			{
				"id":   "basic_auth",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "BasicAuthExecutor",
				},
				"onSuccess": "http_request_notification",
			},
			{
				"id":   "http_request_notification",
				"type": "TASK_EXECUTION",
				"properties": map[string]interface{}{
					"url":    "http://localhost:9091/api/notifications",
					"method": "POST",
					"headers": "{\"Content-Type\": \"application/json\", " +
						"\"X-Flow-Id\": \"{{ context.flowID }}\"}",
					"body": "{\"userId\": \"{{ context.userId }}\", " +
						"\"username\": \"{{ context.username }}\", \"event\": " +
						"\"user_authenticated\", \"unknownField\": \"{{ context.unknownPlaceholder }}\"}",
					"responseMapping": "{\"notificationId\": \"id\", \"status\": \"status\"}",
					"timeout":         "5",
				},
				"executor": map[string]interface{}{
					"name": "HTTPRequestExecutor",
				},
				"onSuccess": "auth_assert",
			},
			{
				"id":   "auth_assert",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "AuthAssertExecutor",
				},
				"onSuccess": "end",
			},
			{
				"id":   "end",
				"type": "END",
			},
		},
	}

	httpRequestExecutorContinueWithErrorFlow = testutils.Flow{
		Name:     "HTTP Request Auth Flow - Continue on Error",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_http_request_error_continue",
		Nodes: []map[string]interface{}{
			{
				"id":        "start",
				"type":      "START",
				"onSuccess": "basic_auth",
			},
			{
				"id":   "basic_auth",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "BasicAuthExecutor",
				},
				"onSuccess": "http_request_notification",
			},
			{
				"id":   "http_request_notification",
				"type": "TASK_EXECUTION",
				"properties": map[string]interface{}{
					"url":           "http://localhost:9091/api/error",
					"method":        "POST",
					"headers":       "{\"Content-Type\": \"application/json\"}",
					"body":          "{\"userId\": \"{{ context.userId }}\"}",
					"errorHandling": "{\"failOnError\": false}",
					"timeout":       "5",
				},
				"executor": map[string]interface{}{
					"name": "HTTPRequestExecutor",
				},
				"onSuccess": "auth_assert",
			},
			{
				"id":   "auth_assert",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "AuthAssertExecutor",
				},
				"onSuccess": "end",
			},
			{
				"id":   "end",
				"type": "END",
			},
		},
	}

	httpRequestExecutorFailOnErrorFlow = testutils.Flow{
		Name:     "HTTP Request Executor Auth Flow - Fail on Error",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_http_request_error_fail",
		Nodes: []map[string]interface{}{
			{
				"id":        "start",
				"type":      "START",
				"onSuccess": "basic_auth",
			},
			{
				"id":   "basic_auth",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "BasicAuthExecutor",
				},
				"onSuccess": "http_request_notification",
			},
			{
				"id":   "http_request_notification",
				"type": "TASK_EXECUTION",
				"properties": map[string]interface{}{
					"url":           "http://localhost:9091/api/error",
					"method":        "POST",
					"headers":       "{\"Content-Type\": \"application/json\"}",
					"body":          "{\"userId\": \"{{ context.userId }}\"}",
					"errorHandling": "{\"failOnError\": true}",
					"timeout":       "5",
				},
				"executor": map[string]interface{}{
					"name": "HTTPRequestExecutor",
				},
				"onSuccess": "auth_assert",
			},
			{
				"id":   "auth_assert",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "AuthAssertExecutor",
				},
				"onSuccess": "end",
			},
			{
				"id":   "end",
				"type": "END",
			},
		},
	}

	httpRequestExecutorTestApp = testutils.Application{
		Name:                      "HTTP Request Executor Test Application",
		Description:               "Application for testing HTTP request executor in authentication flows",
		IsRegistrationFlowEnabled: false,
		ClientID:                  "http_request_executor_test_client",
		ClientSecret:              "http_request_executor_test_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{"http_request_test_person"},
		AssertionConfig: map[string]interface{}{
			"userAttributes": []string{"userType", "ouId", "ouName", "ouHandle"},
		},
	}

	httpRequestTestOU = testutils.OrganizationUnit{
		Name:        "HTTP Request Test OU",
		Handle:      "http-request-test-ou",
		Description: "OU for HTTP request executor authentication tests",
	}

	httpRequestTestEntityType = testutils.UserType{
		Name: "http_request_test_person",
		Schema: map[string]interface{}{
			"username": map[string]interface{}{
				"type": "string",
			},
			"password": map[string]interface{}{
				"type":       "string",
				"credential": true,
			},
			"email": map[string]interface{}{
				"type": "string",
			},
			"given_name": map[string]interface{}{
				"type": "string",
			},
			"family_name": map[string]interface{}{
				"type": "string",
			},
		},
	}

	httpRequestTestUser = testutils.User{
		Type: "http_request_test_person",
		Attributes: json.RawMessage(`{
			"username": "httprequestuser",
			"password": "SecurePass123!",
			"email": "httprequest@test.com",
			"given_name": "HTTP",
			"family_name": "User"
		}`),
	}
)

type HTTPRequestExecutorTestSuite struct {
	suite.Suite
	config                 *common.TestSuiteConfig
	mockNotificationServer *testutils.MockHTTPServer
	ouID                   string
	entityTypeID           string
	testAppID              string
	testFlowID             string
}

func TestHTTPRequestAuthFlowTestSuite(t *testing.T) {
	suite.Run(t, new(HTTPRequestExecutorTestSuite))
}

func (ts *HTTPRequestExecutorTestSuite) SetupSuite() {
	// Initialize config
	ts.config = &common.TestSuiteConfig{}

	// Create test organization unit
	ouID, err := testutils.CreateOrganizationUnit(httpRequestTestOU)
	if err != nil {
		ts.T().Fatalf("Failed to create test organization unit during setup: %v", err)
	}
	ts.ouID = ouID

	// Create test user type within the OU
	httpRequestTestEntityType.OUID = ts.ouID
	schemaID, err := testutils.CreateUserType(httpRequestTestEntityType)
	if err != nil {
		ts.T().Fatalf("Failed to create test user type during setup: %v", err)
	}
	ts.entityTypeID = schemaID

	// Start mock HTTP server
	ts.mockNotificationServer = testutils.NewMockHTTPServer(mockHTTPServerPort)
	err = ts.mockNotificationServer.Start()
	if err != nil {
		ts.T().Fatalf("Failed to start mock HTTP server: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	ts.T().Log("Mock HTTP server started successfully")

	// Create test user for authentication
	testUser := httpRequestTestUser
	testUser.OUID = ts.ouID
	userIDs, err := testutils.CreateMultipleUsers(testUser)
	if err != nil {
		ts.T().Fatalf("Failed to create test user during setup: %v", err)
	}
	ts.config.CreatedUserIDs = userIDs
	ts.T().Logf("Test user created with ID: %s", ts.config.CreatedUserIDs[0])

	// Create flow
	flowID, err := testutils.CreateFlow(httpRequestExecutorFlow)
	ts.Require().NoError(err, "Failed to create HTTP request executor flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)
	ts.testFlowID = flowID
	httpRequestExecutorTestApp.AuthFlowID = flowID

	// Create test application
	httpRequestExecutorTestApp.OUID = ts.ouID
	appID, err := testutils.CreateApplication(httpRequestExecutorTestApp)
	if err != nil {
		ts.T().Fatalf("Failed to create test application during setup: %v", err)
	}
	ts.testAppID = appID
}

func (ts *HTTPRequestExecutorTestSuite) TearDownSuite() {
	// Stop the mock HTTP server
	if ts.mockNotificationServer != nil {
		err := ts.mockNotificationServer.Stop()
		if err != nil {
			ts.T().Logf("Failed to stop mock HTTP server during teardown: %v", err)
		}
	}

	// Delete all created users
	if err := testutils.CleanupUsers(ts.config.CreatedUserIDs); err != nil {
		ts.T().Logf("Failed to cleanup users during teardown: %v", err)
	}

	// Delete test application
	if ts.testAppID != "" {
		if err := testutils.DeleteApplication(ts.testAppID); err != nil {
			ts.T().Logf("Failed to delete test application during teardown: %v", err)
		}
	}

	// Delete test flows
	for _, flowID := range ts.config.CreatedFlowIDs {
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("Failed to delete test flow during teardown: %v", err)
		}
	}

	// Delete test organization unit
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete test organization unit during teardown: %v", err)
		}
	}

	// Delete test user type
	if ts.entityTypeID != "" {
		if err := testutils.DeleteUserType(ts.entityTypeID); err != nil {
			ts.T().Logf("Failed to delete test user type during teardown: %v", err)
		}
	}
}

func (ts *HTTPRequestExecutorTestSuite) SetupTest() {
	// Clear captured requests before each test
	if ts.mockNotificationServer != nil {
		ts.mockNotificationServer.ClearRequests()
	}
}

func (ts *HTTPRequestExecutorTestSuite) TestHTTPRequestAuthFlow_Success() {
	step1, err := common.InitiateAuthenticationFlow(ts.testAppID, false, map[string]string{
		"username": "httprequestuser",
		"password": "SecurePass123!",
	}, "")

	ts.NoError(err, "Authentication flow should complete without error")
	ts.NotNil(step1, "Flow response should not be nil")
	ts.Equal("COMPLETE", step1.FlowStatus, "Flow status should be COMPLETE")
	ts.Require().NotEmpty(step1.Assertion, "JWT assertion should be returned")
	ts.Require().Empty(step1.FailureReason, "Failure reason should be empty")

	time.Sleep(200 * time.Millisecond)

	requests := ts.mockNotificationServer.GetCapturedRequests()
	ts.NotEmpty(requests, "At least one HTTP request should be captured")

	var notificationRequest *testutils.HTTPRequest
	for _, req := range requests {
		if req.Path == "/api/notifications" {
			notificationRequest = &req
			break
		}
	}

	ts.NotNil(notificationRequest, "Notification request should be sent")
	ts.Equal("POST", notificationRequest.Method, "Request method should be POST")
	ts.NotNil(notificationRequest.Body, "Request body should not be nil")
	ts.NotEmpty(notificationRequest.Headers["Content-Type"], "Content-Type header should be present")
	ts.Contains(notificationRequest.Headers["Content-Type"], "application/json",
		"Content-Type should be application/json")

	ts.Equal("httprequestuser", notificationRequest.Body["username"], "Username should match")
	ts.Equal("user_authenticated", notificationRequest.Body["event"], "Event should be user_authenticated")
	ts.NotEmpty(notificationRequest.Body["userId"], "User ID should be present in payload")
	ts.Equal(ts.config.CreatedUserIDs[0], notificationRequest.Body["userId"],
		"User ID should match the authenticated user")
	ts.Equal("{{ context.unknownPlaceholder }}", notificationRequest.Body["unknownField"],
		"Unknown field should retain the placeholder value")
}

func (ts *HTTPRequestExecutorTestSuite) TestHTTPRequestAuthFlow_WithFailOnErrorFalse() {
	// Create flow that continues on error
	flowID, err := testutils.CreateFlow(httpRequestExecutorContinueWithErrorFlow)
	ts.Require().NoError(err, "Failed to create continue-on-error flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)

	// Update application to use the new flow
	err = common.UpdateAppConfig(ts.testAppID, flowID, "")
	ts.NoError(err, "App config update should succeed")

	defer func() {
		common.UpdateAppConfig(ts.testAppID, ts.testFlowID, "")
	}()

	step1, err := common.InitiateAuthenticationFlow(ts.testAppID, false, map[string]string{
		"username": "httprequestuser",
		"password": "SecurePass123!",
	}, "")

	ts.NoError(err, "Authentication flow should complete without error")
	ts.NotNil(step1, "Flow response should not be nil")
	ts.Equal("COMPLETE", step1.FlowStatus, "Flow status should be COMPLETE")
	ts.NotEmpty(step1.Assertion, "JWT assertion should be returned")
	ts.Empty(step1.FailureReason, "Failure reason should be empty")
}

func (ts *HTTPRequestExecutorTestSuite) TestHTTPRequestAuthFlow_WithFailOnErrorTrue() {
	// Create flow that fails on error
	flowID, err := testutils.CreateFlow(httpRequestExecutorFailOnErrorFlow)
	ts.Require().NoError(err, "Failed to create fail-on-error flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)

	// Update application to use the new flow
	err = common.UpdateAppConfig(ts.testAppID, flowID, "")
	ts.NoError(err, "App config update should succeed")

	defer func() {
		common.UpdateAppConfig(ts.testAppID, ts.testFlowID, "")
	}()

	_, err = common.InitiateAuthFlowWithError(ts.testAppID, map[string]string{
		"username": "httprequestuser",
		"password": "SecurePass123!",
	})

	ts.Require().Error(err, "HTTP request failure should cause authentication flow to fail")
}
