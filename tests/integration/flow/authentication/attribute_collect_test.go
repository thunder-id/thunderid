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

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

var (
	attrCollectFlow = testutils.Flow{
		Name:     "Attribute Collect Flow Test",
		FlowType: "AUTHENTICATION",
		Handle:   "attr_collect_test_1",
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
				"onSuccess": "attribute_collect",
			},
			{
				"id":   "attribute_collect",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "AttributeCollector",
					"inputs": []map[string]interface{}{
						{
							"ref":        "input_003",
							"identifier": "given_name",
							"type":       "TEXT_INPUT",
							"required":   true,
						},
						{
							"ref":        "input_004",
							"identifier": "family_name",
							"type":       "TEXT_INPUT",
							"required":   true,
						},
						{
							"ref":        "input_005",
							"identifier": "email",
							"type":       "TEXT_INPUT",
							"required":   true,
						},
						{
							"ref":        "input_006",
							"identifier": "mobileNumber",
							"type":       "TEXT_INPUT",
							"required":   false,
						},
					},
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

	attrCollectTestApp = testutils.Application{
		Name:                      "Attribute Collect Flow Test Application",
		Description:               "Application for testing attribute collection flows",
		IsRegistrationFlowEnabled: false,
		ClientID:                  "attr_collect_flow_test_client",
		ClientSecret:              "attr_collect_flow_test_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{"attr_collect_flow_user"},
		AssertionConfig: map[string]interface{}{
			"userAttributes": []string{"userType", "ouId", "ouName", "ouHandle"},
		},
	}

	attrCollectTestOU = testutils.OrganizationUnit{
		Handle:      "attr-collect-flow-test-ou",
		Name:        "Attribute Collect Flow Test Organization Unit",
		Description: "Organization unit for attribute collection flow testing",
		Parent:      nil,
	}

	attrCollectEntityType = testutils.UserType{
		Name: "attr_collect_flow_user",
		Schema: map[string]interface{}{
			"username": map[string]interface{}{
				"type": "string",
			},
			"password": map[string]interface{}{
				"type":       "string",
				"credential": true,
			},
			"given_name": map[string]interface{}{
				"type": "string",
			},
			"family_name": map[string]interface{}{
				"type": "string",
			},
			"email": map[string]interface{}{
				"type": "string",
			},
			"mobileNumber": map[string]interface{}{
				"type": "string",
			},
		},
	}

	// User templates with different attribute configurations
	testUserNoAttributes = testutils.User{
		Type: attrCollectEntityType.Name,
		Attributes: json.RawMessage(`{
			"username": "noattrsuser",
			"password": "testpassword"
		}`),
	}

	testUserPartialAttributes = testutils.User{
		Type: attrCollectEntityType.Name,
		Attributes: json.RawMessage(`{
			"username": "partialuser",
			"password": "testpassword",
			"given_name": "Partial",
			"family_name": "User"
		}`),
	}

	testUserFullAttributes = testutils.User{
		Type: attrCollectEntityType.Name,
		Attributes: json.RawMessage(`{
			"username": "fulluser",
			"password": "testpassword",
			"given_name": "Full",
			"family_name": "User",
			"email": "fulluser@example.com",
			"mobileNumber": "+1234567890"
		}`),
	}

	testUserNoAttributes2 = testutils.User{
		Type: attrCollectEntityType.Name,
		Attributes: json.RawMessage(`{
			"username": "noattrsuser2",
			"password": "testpassword"
		}`),
	}
)

var (
	attrCollectTestAppID    string
	attrCollectTestOUID     string
	attrCollectEntityTypeID string
)

type AttributeCollectTestData struct {
	name                 string
	user                 testutils.User
	expectedMissingAttrs []string
	credentials          map[string]string
	providedAttrs        map[string]string
}

type AttributeCollectFlowTestSuite struct {
	suite.Suite
	config   *common.TestSuiteConfig
	testData []AttributeCollectTestData
}

func TestAttributeCollectFlowTestSuite(t *testing.T) {
	suite.Run(t, new(AttributeCollectFlowTestSuite))
}

func (ts *AttributeCollectFlowTestSuite) SetupSuite() {
	// Initialize config
	ts.config = &common.TestSuiteConfig{}

	// Create test organization unit for attribute collect tests
	ouID, err := testutils.CreateOrganizationUnit(attrCollectTestOU)
	if err != nil {
		ts.T().Fatalf("Failed to create test organization unit during setup: %v", err)
	}
	attrCollectTestOUID = ouID

	// Create test user type within the OU
	attrCollectEntityType.OUID = attrCollectTestOUID
	schemaID, err := testutils.CreateUserType(attrCollectEntityType)
	if err != nil {
		ts.T().Fatalf("Failed to create test user type during setup: %v", err)
	}
	attrCollectEntityTypeID = schemaID

	// Create attribute collect flow
	attrFlowID, err := testutils.CreateFlow(attrCollectFlow)
	ts.Require().NoError(err, "Failed to create attribute collect flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, attrFlowID)
	attrCollectTestApp.AuthFlowID = attrFlowID

	// Create test application for attribute collect tests
	attrCollectTestApp.OUID = attrCollectTestOUID
	appID, err := testutils.CreateApplication(attrCollectTestApp)
	if err != nil {
		ts.T().Fatalf("Failed to create test application during setup: %v", err)
	}
	attrCollectTestAppID = appID

	// Create users with the created OU ID
	testUserNoAttributes := testUserNoAttributes
	testUserNoAttributes.OUID = attrCollectTestOUID
	testUserPartialAttributes := testUserPartialAttributes
	testUserPartialAttributes.OUID = attrCollectTestOUID
	testUserFullAttributes := testUserFullAttributes
	testUserFullAttributes.OUID = attrCollectTestOUID
	testUserNoAttributes2 := testUserNoAttributes2
	testUserNoAttributes2.OUID = attrCollectTestOUID

	// Setup test data
	ts.testData = []AttributeCollectTestData{
		{
			name:                 "UserWithNoAttributes",
			user:                 testUserNoAttributes,
			expectedMissingAttrs: []string{"given_name", "family_name", "email", "mobileNumber"},
			credentials: map[string]string{
				"username": "noattrsuser",
				"password": "testpassword",
			},
			providedAttrs: map[string]string{
				"given_name":   "John",
				"family_name":  "Doe",
				"email":        "john.doe@example.com",
				"mobileNumber": "+1987654321",
			},
		},
		{
			name:                 "UserWithPartialAttributes",
			user:                 testUserPartialAttributes,
			expectedMissingAttrs: []string{"email", "mobileNumber"},
			credentials: map[string]string{
				"username": "partialuser",
				"password": "testpassword",
			},
			providedAttrs: map[string]string{
				"email":        "partial@example.com",
				"mobileNumber": "+1555666777",
			},
		},
		{
			name:                 "UserWithFullAttributes",
			user:                 testUserFullAttributes,
			expectedMissingAttrs: []string{},
			credentials: map[string]string{
				"username": "fulluser",
				"password": "testpassword",
			},
			providedAttrs: map[string]string{},
		},
	}

	// Create all test users
	var users []testutils.User
	for _, testCase := range ts.testData {
		users = append(users, testCase.user)
	}
	users = append(users, testUserNoAttributes2) // Additional user for second login tests

	userIDs, err := testutils.CreateMultipleUsers(users...)
	if err != nil {
		ts.T().Fatalf("Failed to create test users during setup: %v", err)
	}
	ts.config.CreatedUserIDs = userIDs
}

func (ts *AttributeCollectFlowTestSuite) TearDownSuite() {
	// Delete all created users
	if err := testutils.CleanupUsers(ts.config.CreatedUserIDs); err != nil {
		ts.T().Logf("Failed to cleanup users during teardown: %v", err)
	}

	// Delete test application
	if attrCollectTestAppID != "" {
		if err := testutils.DeleteApplication(attrCollectTestAppID); err != nil {
			ts.T().Logf("Failed to delete test application during teardown: %v", err)
		}
	}

	// Delete test organization unit
	if attrCollectTestOUID != "" {
		if err := testutils.DeleteOrganizationUnit(attrCollectTestOUID); err != nil {
			ts.T().Logf("Failed to delete test organization unit during teardown: %v", err)
		}
	}

	if attrCollectEntityTypeID != "" {
		if err := testutils.DeleteUserType(attrCollectEntityTypeID); err != nil {
			ts.T().Logf("Failed to delete test user type during teardown: %v", err)
		}
	}

	for _, flowID := range ts.config.CreatedFlowIDs {
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("Failed to delete test flow during teardown: %v", err)
		}
	}
}

// TestAttributeCollectionFlow tests the complete attribute collection flow including first and second login
func (ts *AttributeCollectFlowTestSuite) TestAttributeCollectionFlow() {
	for _, testCase := range ts.testData {
		ts.Run(testCase.name, func() {
			// Test First Login - should prompt for missing attributes
			ts.Run("FirstLogin", func() {
				// Step 1: Initialize the flow - should prompt for username/password
				flowStep, err := common.InitiateAuthenticationFlow(attrCollectTestAppID, false, nil, "")
				ts.Require().NoError(err, "Failed to initiate authentication flow")
				ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus, "Expected flow status to be INCOMPLETE")
				ts.Require().Equal("VIEW", flowStep.Type, "Expected flow type to be VIEW")

				// Validate that username and password are required
				ts.validateRequiredInputs(flowStep.Data.Inputs, []string{"username", "password"})

				// Step 2: Provide credentials - should authenticate and proceed to attribute collection
				credentialStep, err := common.CompleteFlow(flowStep.ExecutionID, testCase.credentials, "",
					flowStep.ChallengeToken)
				ts.Require().NoError(err, "Failed to complete basic authentication")

				if len(testCase.expectedMissingAttrs) == 0 {
					// User has all attributes - should complete authentication
					ts.Require().Equal("COMPLETE", credentialStep.FlowStatus,
						"Expected flow to complete for user with all attributes")
					ts.Require().NotEmpty(credentialStep.Assertion, "Expected assertion for completed flow")
				} else {
					// User missing attributes - should prompt for them
					ts.Require().Equal("INCOMPLETE", credentialStep.FlowStatus,
						"Expected flow status to be INCOMPLETE")
					ts.Require().Equal("VIEW", credentialStep.Type, "Expected flow type to be VIEW")

					// Validate that the missing attributes are prompted
					ts.validateRequiredInputs(credentialStep.Data.Inputs, testCase.expectedMissingAttrs)

					// Step 3: Provide missing attributes
					if len(testCase.providedAttrs) > 0 {
						finalStep, err := common.CompleteFlow(credentialStep.ExecutionID, testCase.providedAttrs, "",
							credentialStep.ChallengeToken)
						ts.Require().NoError(err, "Failed to complete attribute collection")
						ts.Require().Equal("COMPLETE", finalStep.FlowStatus, "Expected flow status to be COMPLETE")
						ts.Require().NotEmpty(finalStep.Assertion, "Expected assertion after attribute collection")
					}
				}
			})

			// Test Second Login - should not prompt for attributes again
			if len(testCase.expectedMissingAttrs) > 0 {
				ts.Run("SecondLogin", func() {
					// Now perform second login - should not prompt for attributes
					flowStep, err := common.InitiateAuthenticationFlow(attrCollectTestAppID, false, nil, "")
					ts.Require().NoError(err, "Failed to initiate second authentication flow")
					ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus, "Expected flow status to be INCOMPLETE")
					ts.Require().Equal("VIEW", flowStep.Type, "Expected flow type to be VIEW")

					// Provide credentials
					credentialStep, err := common.CompleteFlow(flowStep.ExecutionID, testCase.credentials, "",
						flowStep.ChallengeToken)
					ts.Require().NoError(err, "Failed to complete second authentication")
					ts.Require().Equal("COMPLETE", credentialStep.FlowStatus,
						"Expected flow to complete on second login")
					ts.Require().NotEmpty(credentialStep.Assertion, "Expected assertion on second login")
				})
			}
		})
	}
}

func (ts *AttributeCollectFlowTestSuite) TestSingleRequestLogin_WithAllInputs() {
	flowStep, err := common.InitiateAuthenticationFlow(attrCollectTestAppID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus, "Expected flow status to be INCOMPLETE")
	ts.Require().Equal("VIEW", flowStep.Type, "Expected flow type to be VIEW")
	ts.validateRequiredInputs(flowStep.Data.Inputs, []string{"username", "password"})

	// Provide all required inputs in a single request
	allInputs := map[string]string{
		"username":     "fulluser",
		"password":     "testpassword",
		"given_name":   "Full",
		"family_name":  "User",
		"email":        "john.doe2@example.com",
		"mobileNumber": "+1987654345",
	}
	finalStep, err := common.CompleteFlow(flowStep.ExecutionID, allInputs, "", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to complete authentication with all inputs")
	ts.Require().Equal("COMPLETE", finalStep.FlowStatus, "Expected flow status to be COMPLETE")
	ts.Require().NotEmpty(finalStep.Assertion, "Expected assertion after completing flow with all inputs")
}

func (ts *AttributeCollectFlowTestSuite) TestInvalidCredentials() {
	invalidCredentials := map[string]string{
		"username": "invaliduser",
		"password": "wrongpassword",
	}

	flowStep, err := common.InitiateAuthenticationFlow(attrCollectTestAppID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")

	errorResp, err := common.CompleteFlow(flowStep.ExecutionID, invalidCredentials, "", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Expected error response for invalid credentials")
	ts.Require().NotEmpty(errorResp.FailureReason, "Expected failure reason for invalid credentials")
	ts.Require().Contains(errorResp.FailureReason, "User not found",
		"Expected failure reason to indicate user not found")
}

func (ts *AttributeCollectFlowTestSuite) validateRequiredInputs(actualInputs []common.Inputs,
	expectedInputNames []string) {
	// Use utility function for basic validation
	ts.Require().True(common.ValidateRequiredInputs(actualInputs, expectedInputNames),
		"Expected inputs should be present")

	// Additional validation specific to attribute collection
	ts.Require().Len(actualInputs, len(expectedInputNames),
		"Expected %d inputs, got %d", len(expectedInputNames), len(actualInputs))

	actualInputMap := make(map[string]common.Inputs)
	for _, input := range actualInputs {
		actualInputMap[input.Identifier] = input
	}

	for _, expectedName := range expectedInputNames {
		input, exists := actualInputMap[expectedName]
		ts.Require().True(exists, "Expected input '%s' not found", expectedName)

		if expectedName == "password" {
			ts.Require().Equal("PASSWORD_INPUT", input.Type, "Expected input password to be of type PASSWORD_INPUT")
		} else {
			ts.Require().Equal("TEXT_INPUT", input.Type, "Expected input '%s' to be of type string", expectedName)
		}

		// Check if required field is set correctly based on the flow definition
		if expectedName == "mobileNumber" {
			ts.Require().False(input.Required, "Expected input '%s' to be optional", expectedName)
		} else {
			ts.Require().True(input.Required, "Expected input '%s' to be required", expectedName)
		}
	}
}
