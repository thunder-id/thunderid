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

package registration

import (
	"testing"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

const (
	mockHTTPServerPortReg = 9091
)

var (
	httpRequestRegistrationFlow = testutils.Flow{
		Name:     "HTTP Request Registration Test Flow",
		FlowType: "REGISTRATION",
		Handle:   "registration_flow_basic_http_request_test",
		Nodes: []map[string]interface{}{
			{
				"id":        "start",
				"type":      "START",
				"onSuccess": "user_type_resolver",
			},
			{
				"id":   "user_type_resolver",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "UserTypeResolver",
				},
				"onSuccess":    "provision_user",
				"onIncomplete": "prompt_usertype",
			},
			{
				"id":   "prompt_usertype",
				"type": "PROMPT",
				"meta": map[string]interface{}{
					"components": []map[string]interface{}{
						{
							"type":    "TEXT",
							"id":      "heading_usertype",
							"label":   "Sign Up",
							"variant": "HEADING_2",
						},
						{
							"type": "BLOCK",
							"id":   "block_usertype",
							"components": []map[string]interface{}{
								{
									"type":        "SELECT",
									"id":          "usertype_input",
									"ref":         "userType",
									"label":       "User Type",
									"placeholder": "Select your user type",
									"required":    true,
									"options":     []interface{}{},
								},
								{
									"type":      "ACTION",
									"id":        "action_usertype",
									"label":     "Continue",
									"variant":   "PRIMARY",
									"eventType": "SUBMIT",
								},
							},
						},
					},
				},
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{
								"ref":        "usertype_input",
								"identifier": "userType",
								"type":       "SELECT",
								"required":   true,
							},
						},
						"action": map[string]interface{}{
							"ref":      "action_usertype",
							"nextNode": "user_type_resolver",
						},
					},
				},
			},
			{
				"id":   "provision_user",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "ProvisioningExecutor",
					"inputs": []map[string]interface{}{
						{
							"ref":        "input_001",
							"identifier": "username",
							"type":       "string",
							"required":   true,
						},
						{
							"ref":        "input_002",
							"identifier": "password",
							"type":       "string",
							"required":   true,
						},
						{
							"ref":        "input_003",
							"identifier": "email",
							"type":       "string",
							"required":   true,
						},
						{
							"ref":        "input_004",
							"identifier": "given_name",
							"type":       "string",
							"required":   true,
						},
						{
							"ref":        "input_005",
							"identifier": "family_name",
							"type":       "string",
							"required":   true,
						},
					},
				},
				"onSuccess": "create_external_profile",
			},
			{
				"id":   "create_external_profile",
				"type": "TASK_EXECUTION",
				"properties": map[string]interface{}{
					"url":    "http://localhost:9091/api/users",
					"method": "POST",
					"headers": map[string]interface{}{
						"Content-Type":  "application/json",
						"Authorization": "Bearer test-token",
					},
					"body": map[string]interface{}{
						"externalId":   "{{ context.userId }}",
						"username":     "{{ context.username }}",
						"email":        "{{ context.email }}",
						"given_name":   "{{ context.given_name }}",
						"family_name":  "{{ context.family_name }}",
						"unknownField": "{{ context.unknownPlaceholder }}",
					},
					"responseMapping": map[string]interface{}{
						"externalUserId": "data.userId",
						"profileUrl":     "data.profileUrl",
					},
					"timeout": 5,
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

	httpRequestRegTestOU = testutils.OrganizationUnit{
		Name:        "HTTP Request Registration Test OU",
		Handle:      "http-request-reg-test-ou",
		Description: "OU for HTTP request executor registration tests",
	}

	httpRequestRegTestEntityType = testutils.UserType{
		Name: "http_request_reg_test_person",
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

	httpRequestRegTestApp = testutils.Application{
		Name:                      "HTTP Request Executor Registration Test Application",
		Description:               "Application for testing HTTP request executor in registration flows",
		IsRegistrationFlowEnabled: true,
		AllowedUserTypes:          []string{httpRequestRegTestEntityType.Name},
		AssertionConfig: map[string]interface{}{
			"userAttributes": []string{"userType", "ouId", "ouName", "ouHandle"},
		},
	}
)

var (
	httpRequestRegTestAppID    string
	httpRequestRegTestOUID     string
	httpRequestRegEntityTypeID string
)

type HTTPRequestRegistrationFlowTestSuite struct {
	suite.Suite
	config     *common.TestSuiteConfig
	mockServer *testutils.MockHTTPServer
}

func TestHTTPRequestRegistrationFlowTestSuite(t *testing.T) {
	suite.Run(t, new(HTTPRequestRegistrationFlowTestSuite))
}

func (ts *HTTPRequestRegistrationFlowTestSuite) SetupSuite() {
	// Initialize config
	ts.config = &common.TestSuiteConfig{}

	// Create test organization unit
	ouID, err := testutils.CreateOrganizationUnit(httpRequestRegTestOU)
	if err != nil {
		ts.T().Fatalf("Failed to create test organization unit during setup: %v", err)
	}
	httpRequestRegTestOUID = ouID

	// Create test user type within the test OU
	httpRequestRegTestEntityType.OUID = httpRequestRegTestOUID
	httpRequestRegTestEntityType.AllowSelfRegistration = true
	schemaID, err := testutils.CreateUserType(httpRequestRegTestEntityType)
	if err != nil {
		ts.T().Fatalf("Failed to create test user type during setup: %v", err)
	}
	httpRequestRegEntityTypeID = schemaID

	// Create registration flow
	flowID, err := testutils.CreateFlow(httpRequestRegistrationFlow)
	if err != nil {
		ts.T().Fatalf("Failed to create registration flow during setup: %v", err)
	}
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)
	httpRequestRegTestApp.RegistrationFlowID = flowID

	// Create test application
	httpRequestRegTestApp.OUID = httpRequestRegTestOUID
	appID, err := testutils.CreateApplication(httpRequestRegTestApp)
	if err != nil {
		ts.T().Fatalf("Failed to create test application during setup: %v", err)
	}
	httpRequestRegTestAppID = appID

	// Start mock HTTP server
	ts.mockServer = testutils.NewMockHTTPServer(mockHTTPServerPortReg)
	err = ts.mockServer.Start()
	if err != nil {
		ts.T().Fatalf("Failed to start mock HTTP server: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	ts.T().Log("Mock HTTP server started successfully")
}

func (ts *HTTPRequestRegistrationFlowTestSuite) TearDownSuite() {
	// Stop the mock HTTP server
	if ts.mockServer != nil {
		err := ts.mockServer.Stop()
		if err != nil {
			ts.T().Logf("Failed to stop mock HTTP server during teardown: %v", err)
		}
	}

	// Delete all created users
	if err := testutils.CleanupUsers(ts.config.CreatedUserIDs); err != nil {
		ts.T().Logf("Failed to cleanup users during teardown: %v", err)
	}

	// Delete test application
	if httpRequestRegTestAppID != "" {
		if err := testutils.DeleteApplication(httpRequestRegTestAppID); err != nil {
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
	if httpRequestRegTestOUID != "" {
		if err := testutils.DeleteOrganizationUnit(httpRequestRegTestOUID); err != nil {
			ts.T().Logf("Failed to delete test organization unit during teardown: %v", err)
		}
	}

	// Delete test user type
	if httpRequestRegEntityTypeID != "" {
		if err := testutils.DeleteUserType(httpRequestRegEntityTypeID); err != nil {
			ts.T().Logf("Failed to delete test user type during teardown: %v", err)
		}
	}
}

func (ts *HTTPRequestRegistrationFlowTestSuite) SetupTest() {
	// Clear captured requests before each test
	if ts.mockServer != nil {
		ts.mockServer.ClearRequests()
	}
}

func (ts *HTTPRequestRegistrationFlowTestSuite) TestHTTPRequestRegistrationFlow_Success() {
	step1, err := common.InitiateRegistrationFlow(httpRequestRegTestAppID, false, map[string]string{
		"username":    "newuser123",
		"password":    "NewUserPass123!",
		"email":       "newuser@test.com",
		"given_name":  "New",
		"family_name": "User",
	}, "")

	ts.NoError(err, "Registration flow should complete without error")
	ts.NotNil(step1, "Flow response should not be nil")
	ts.Equal("COMPLETE", step1.FlowStatus, "Flow status should be COMPLETE")

	time.Sleep(200 * time.Millisecond)

	requests := ts.mockServer.GetCapturedRequests()
	ts.NotEmpty(requests, "At least one HTTP request should be captured")

	var userCreationRequest *testutils.HTTPRequest
	for _, req := range requests {
		if req.Path == "/api/users" {
			userCreationRequest = &req
			break
		}
	}

	ts.NotNil(userCreationRequest, "User creation request should be sent")
	ts.Equal("POST", userCreationRequest.Method, "Request method should be POST")
	ts.NotNil(userCreationRequest.Body, "Request body should not be nil")
	ts.NotEmpty(userCreationRequest.Headers["Content-Type"], "Content-Type header should be present")
	ts.Contains(userCreationRequest.Headers["Content-Type"], "application/json",
		"Content-Type should be application/json")
	ts.NotEmpty(userCreationRequest.Headers["Authorization"], "Authorization header should be present")
	ts.Contains(userCreationRequest.Headers["Authorization"], "Bearer test-token",
		"Authorization header should contain bearer token")

	ts.Equal("newuser123", userCreationRequest.Body["username"], "Username should match")
	ts.Equal("newuser@test.com", userCreationRequest.Body["email"], "Email should match")
	ts.Equal("New", userCreationRequest.Body["given_name"], "First name should match")
	ts.Equal("User", userCreationRequest.Body["family_name"], "Last name should match")
	ts.NotEmpty(userCreationRequest.Body["externalId"], "External ID should be present in payload")
	ts.Equal("{{ context.unknownPlaceholder }}", userCreationRequest.Body["unknownField"],
		"Unknown field should retain the placeholder value")
}
