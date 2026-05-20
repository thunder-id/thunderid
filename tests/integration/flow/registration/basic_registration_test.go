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
	"encoding/json"
	"testing"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

var (
	testOU = testutils.OrganizationUnit{
		Handle:      "reg-flow-test-ou",
		Name:        "Registration Flow Test Organization Unit",
		Description: "Organization unit for registration flow testing",
		Parent:      nil,
	}

	testUserType = testutils.UserType{
		Name: "test-user-type",
		Schema: map[string]interface{}{
			"username": map[string]interface{}{
				"type": "string",
			},
			"password": map[string]interface{}{
				"type":       "string",
				"credential": true,
			},
			"email": map[string]interface{}{
				"type":     "string",
				"required": true,
			},
			"given_name": map[string]interface{}{
				"type":     "string",
				"required": true,
			},
			"family_name": map[string]interface{}{
				"type":     "string",
				"required": true,
			},
			"mobileNumber": map[string]interface{}{
				"type": "string",
			},
		},
		AllowSelfRegistration: true,
	}
)

type BasicRegistrationFlowTestSuite struct {
	suite.Suite
	config           *common.TestSuiteConfig
	entityTypeID     string
	testAppID        string
	testOUID         string
	testUserTypeName string
}

func TestBasicRegistrationFlowTestSuite(t *testing.T) {
	suite.Run(t, new(BasicRegistrationFlowTestSuite))
}

func (ts *BasicRegistrationFlowTestSuite) SetupSuite() {
	// Initialize config
	ts.config = &common.TestSuiteConfig{}

	// Create test organization unit
	ouID, err := testutils.CreateOrganizationUnit(testOU)
	if err != nil {
		ts.T().Fatalf("Failed to create test organization unit during setup: %v", err)
	}
	ts.testOUID = ouID

	// Create test user type
	testUserType.OUID = ts.testOUID
	schemaID, err := testutils.CreateUserType(testUserType)
	if err != nil {
		ts.T().Fatalf("Failed to create test user type during setup: %v", err)
	}
	ts.entityTypeID = schemaID
	ts.testUserTypeName = testUserType.Name

	// Look up the default registration flow ID
	regFlowID, err := testutils.GetFlowIDByHandle("default-basic-flow", "REGISTRATION")
	if err != nil {
		ts.T().Fatalf("Failed to get default registration flow ID: %v", err)
	}

	// Create test application with allowed user types
	testApp := testutils.Application{
		OUID:                      ts.testOUID,
		Name:                      "Registration Flow Test Application",
		Description:               "Application for testing registration flows",
		IsRegistrationFlowEnabled: true,
		RegistrationFlowID:        regFlowID,
		ClientID:                  "reg_flow_test_client",
		ClientSecret:              "reg_flow_test_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{testUserType.Name},
		AssertionConfig: map[string]interface{}{
			"userAttributes": []string{"userType", "ouId", "ouName", "ouHandle"},
		},
	}

	appID, err := testutils.CreateApplication(testApp)
	if err != nil {
		ts.T().Fatalf("Failed to create test application during setup: %v", err)
	}
	ts.testAppID = appID
}

func (ts *BasicRegistrationFlowTestSuite) TearDownSuite() {
	// Clean up users created during registration tests
	if err := testutils.CleanupUsers(ts.config.CreatedUserIDs); err != nil {
		ts.T().Logf("Failed to cleanup users during teardown: %v", err)
	}

	// Delete test application
	if ts.testAppID != "" {
		if err := testutils.DeleteApplication(ts.testAppID); err != nil {
			ts.T().Logf("Failed to delete test application during teardown: %v", err)
		}
	}

	// Delete test organization unit
	if ts.testOUID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.testOUID); err != nil {
			ts.T().Logf("Failed to delete test organization unit during teardown: %v", err)
		}
	}

	if ts.entityTypeID != "" {
		if err := testutils.DeleteUserType(ts.entityTypeID); err != nil {
			ts.T().Logf("Failed to delete test user type during teardown: %v", err)
		}
	}
}

func (ts *BasicRegistrationFlowTestSuite) TestBasicRegistrationFlowSuccess() {
	// Generate unique username for this test
	username := common.GenerateUniqueUsername("reguser")

	// Step 1: Initialize the registration flow
	flowStep, err := common.InitiateRegistrationFlow(ts.testAppID, false, nil, "")
	if err != nil {
		ts.T().Fatalf("Failed to initiate registration flow: %v", err)
	}

	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus, "Expected flow status to be INCOMPLETE")
	ts.Require().Equal("VIEW", flowStep.Type, "Expected flow type to be VIEW")
	ts.Require().NotEmpty(flowStep.ExecutionID, "Execution ID should not be empty")
	ts.Require().NotEmpty(flowStep.Data, "Flow data should not be empty")
	ts.Require().NotEmpty(flowStep.Data.Inputs, "Flow should require inputs")

	// Verify username and password are required inputs
	ts.Require().True(common.ValidateRequiredInputs(flowStep.Data.Inputs, []string{"username", "password"}),
		"Username and password inputs should be required")
	ts.Require().True(common.HasInput(flowStep.Data.Inputs, "username"), "Username input should be present")
	ts.Require().True(common.HasInput(flowStep.Data.Inputs, "password"), "Password input should be present")

	// Step 2: Continue the flow with registration credentials
	inputs := map[string]string{
		"username": username,
		"password": "testpassword123",
	}

	completeFlowStep, err := common.CompleteFlow(flowStep.ExecutionID, inputs, "action_credentials",
		flowStep.ChallengeToken)
	if err != nil {
		ts.T().Fatalf("Failed to complete registration flow: %v", err)
	}

	// Step 3: Continue the flow with additional attributes
	ts.Require().Equal("INCOMPLETE", completeFlowStep.FlowStatus,
		"Expected flow status to be INCOMPLETE after first step")
	ts.Require().NotEmpty(completeFlowStep.Data, "Flow data should not be empty after first step")
	ts.Require().NotEmpty(completeFlowStep.Data.Inputs, "Flow should require additional inputs after first step")
	ts.Require().True(common.ValidateRequiredInputs(completeFlowStep.Data.Inputs,
		[]string{"email", "given_name", "family_name"}),
		"Email, first name, and last name should be required inputs after first step")

	inputs = map[string]string{
		"email":       username + "@example.com",
		"given_name":  "Test",
		"family_name": "User",
	}
	completeFlowStep, err = common.CompleteFlow(completeFlowStep.ExecutionID, inputs, "action_schema_attrs",
		completeFlowStep.ChallengeToken)
	if err != nil {
		ts.T().Fatalf("Failed to complete registration flow with additional attributes: %v", err)
	}

	// Step 4: Verify successful registration
	ts.Require().Equal("COMPLETE", completeFlowStep.FlowStatus, "Expected flow status to be COMPLETE")
	ts.Require().NotEmpty(completeFlowStep.Assertion,
		"JWT assertion should be returned after successful registration")
	ts.Require().Empty(completeFlowStep.FailureReason, "Failure reason should be empty for successful registration")

	// Decode and validate JWT claims
	jwtClaims, err := testutils.DecodeJWT(completeFlowStep.Assertion)
	ts.Require().NoError(err, "Failed to decode JWT assertion")
	ts.Require().NotNil(jwtClaims, "JWT claims should not be nil")

	// Validate JWT contains expected user type and OU ID
	ts.Require().Equal(testUserType.Name, jwtClaims.UserType, "Expected userType to match created schema")
	ts.Require().Equal(ts.testOUID, jwtClaims.OUID, "Expected ouId to match the created organization unit")
	ts.Require().Equal(ts.testAppID, jwtClaims.Aud, "Expected aud to match the application ID")
	ts.Require().NotEmpty(jwtClaims.Sub, "JWT subject should not be empty")

	// Step 5: Verify the user was created by searching via the user API
	user, err := testutils.FindUserByAttribute("username", username)
	if err != nil {
		ts.T().Fatalf("Failed to retrieve user by username: %v", err)
	}
	ts.Require().NotNil(user, "User should be found in user list after registration")

	// Store the created user for cleanup
	if user != nil {
		ts.config.CreatedUserIDs = append(ts.config.CreatedUserIDs, user.ID)
	}
}

func (ts *BasicRegistrationFlowTestSuite) TestBasicRegistrationFlowDuplicateUser() {
	// Create a test user first
	testUser := testutils.User{
		OUID: ts.testOUID,
		Type: testUserType.Name,
		Attributes: json.RawMessage(`{
			"username": "duplicateuser",
			"password": "testpassword",
			"email": "duplicate@example.com",
			"given_name": "Duplicate",
			"family_name": "User"
		}`),
	}

	userIDs, err := testutils.CreateMultipleUsers(testUser)
	if err != nil {
		ts.T().Fatalf("Failed to create test user for duplicate test: %v", err)
	}
	ts.config.CreatedUserIDs = append(ts.config.CreatedUserIDs, userIDs...)

	// Step 1: Initialize the registration flow
	flowStep, err := common.InitiateRegistrationFlow(ts.testAppID, false, nil, "")
	if err != nil {
		ts.T().Fatalf("Failed to initiate registration flow: %v", err)
	}

	// Step 2: Try to register with existing username
	inputs := map[string]string{
		"username": "duplicateuser",
		"password": "newpassword123",
	}

	completeFlowStep, err := common.CompleteFlow(flowStep.ExecutionID, inputs, "action_credentials",
		flowStep.ChallengeToken)
	if err != nil {
		ts.T().Fatalf("Failed to complete registration flow: %v", err)
	}

	// Step 3: Verify registration failure due to duplicate username
	ts.Require().Equal("ERROR", completeFlowStep.FlowStatus, "Expected flow status to be ERROR")
	ts.Require().Empty(completeFlowStep.Assertion, "No JWT assertion should be returned for failed registration")
	ts.Require().NotEmpty(completeFlowStep.FailureReason, "Failure reason should be provided for duplicate user")
	ts.Equal("User already exists with the provided attributes.", completeFlowStep.FailureReason,
		"Failure reason should indicate duplicate username")
}

func (ts *BasicRegistrationFlowTestSuite) TestBasicRegistrationFlowInitialInvalidInput() {
	// Step 1: Initialize the registration flow
	flowStep, err := common.InitiateRegistrationFlow(ts.testAppID, false, nil, "")
	if err != nil {
		ts.T().Fatalf("Failed to initiate registration flow: %v", err)
	}

	// Step 2: Try to register with only the username
	username := common.GenerateUniqueUsername("newuser")
	inputs := map[string]string{
		"username": username,
	}
	completeFlowStep, err := common.CompleteFlow(flowStep.ExecutionID, inputs, "", flowStep.ChallengeToken)
	if err != nil {
		ts.T().Fatalf("Failed to complete registration flow: %v", err)
	}

	// Step 3: Verify flow prompt for username again
	ts.Require().Equal("INCOMPLETE", completeFlowStep.FlowStatus, "Expected flow status to be INCOMPLETE")
	ts.Require().Empty(completeFlowStep.Assertion, "No JWT assertion should be returned for incomplete registration")
	ts.Require().Empty(completeFlowStep.FailureReason, "Failure reason should be empty for incomplete registration")
	ts.Require().True(common.HasInput(completeFlowStep.Data.Inputs, "password"),
		"Flow should prompt for password after invalid input")

	// Step 4: Continue with the password input
	inputs = map[string]string{
		"password": "testpassword123",
	}
	completeFlowStep, err = common.CompleteFlow(completeFlowStep.ExecutionID, inputs, "action_credentials",
		completeFlowStep.ChallengeToken)
	if err != nil {
		ts.T().Fatalf("Failed to complete registration flow with username input: %v", err)
	}

	// Step 5: Continue the flow with additional attributes
	ts.Require().Equal("INCOMPLETE", completeFlowStep.FlowStatus,
		"Expected flow status to be INCOMPLETE after first step")
	ts.Require().NotEmpty(completeFlowStep.Data, "Flow data should not be empty after first step")
	ts.Require().NotEmpty(completeFlowStep.Data.Inputs, "Flow should require additional inputs after first step")
	ts.Require().True(common.ValidateRequiredInputs(completeFlowStep.Data.Inputs,
		[]string{"email", "given_name", "family_name"}),
		"Email, first name, and last name should be required inputs after first step")

	inputs = map[string]string{
		"email":       username + "@example.com",
		"given_name":  "Test",
		"family_name": "User",
	}
	completeFlowStep, err = common.CompleteFlow(completeFlowStep.ExecutionID, inputs, "action_schema_attrs",
		completeFlowStep.ChallengeToken)
	if err != nil {
		ts.T().Fatalf("Failed to complete registration flow with additional attributes: %v", err)
	}

	// Step 6: Verify successful registration
	ts.Require().Equal("COMPLETE", completeFlowStep.FlowStatus, "Expected flow status to be COMPLETE")
	ts.Require().NotEmpty(completeFlowStep.Assertion,
		"JWT assertion should be returned after successful registration")
	ts.Require().Empty(completeFlowStep.FailureReason, "Failure reason should be empty for successful registration")

	// Decode and validate JWT claims
	jwtClaims, err := testutils.DecodeJWT(completeFlowStep.Assertion)
	ts.Require().NoError(err, "Failed to decode JWT assertion")
	ts.Require().NotNil(jwtClaims, "JWT claims should not be nil")

	// Validate JWT contains expected user type and OU ID
	ts.Require().Equal(testUserType.Name, jwtClaims.UserType, "Expected userType to match created schema")
	ts.Require().Equal(ts.testOUID, jwtClaims.OUID, "Expected ouId to match the created organization unit")
	ts.Require().Equal(ts.testAppID, jwtClaims.Aud, "Expected aud to match the application ID")
	ts.Require().NotEmpty(jwtClaims.Sub, "JWT subject should not be empty")

	// Step 7: Verify the user was created by searching via the user API
	user, err := testutils.FindUserByAttribute("username", username)
	if err != nil {
		ts.T().Fatalf("Failed to retrieve user by username: %v", err)
	}
	ts.Require().NotNil(user, "User should be found in user list after registration")

	// Store the created user for cleanup
	if user != nil {
		ts.config.CreatedUserIDs = append(ts.config.CreatedUserIDs, user.ID)
	}
}

func (ts *BasicRegistrationFlowTestSuite) TestBasicRegistrationFlowSingleRequest() {
	// Generate unique username for this test
	username := common.GenerateUniqueUsername("singlereguser")

	// Step 1: Initialize the registration flow with credentials in one request
	inputs := map[string]string{
		"username":    username,
		"password":    "testpassword123",
		"email":       username + "@example.com",
		"given_name":  "Single",
		"family_name": "Request",
	}

	flowStep, err := common.InitiateRegistrationFlow(ts.testAppID, false, inputs, "")
	if err != nil {
		ts.T().Fatalf("Failed to initiate registration flow with inputs: %v", err)
	}

	// Step 2: Verify successful registration in a single request
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus, "Expected flow status to be COMPLETE")
	ts.Require().NotEmpty(flowStep.Assertion,
		"JWT assertion should be returned after successful registration")
	ts.Require().Empty(flowStep.FailureReason, "Failure reason should be empty for successful registration")

	// Decode and validate JWT claims
	jwtClaims, err := testutils.DecodeJWT(flowStep.Assertion)
	ts.Require().NoError(err, "Failed to decode JWT assertion")
	ts.Require().NotNil(jwtClaims, "JWT claims should not be nil")

	// Validate JWT contains expected user type and OU ID
	ts.Require().Equal(testUserType.Name, jwtClaims.UserType, "Expected userType to match created schema")
	ts.Require().Equal(ts.testOUID, jwtClaims.OUID, "Expected ouId to match the created organization unit")
	ts.Require().Equal(ts.testAppID, jwtClaims.Aud, "Expected aud to match the application ID")
	ts.Require().NotEmpty(jwtClaims.Sub, "JWT subject should not be empty")

	// Step 3: Verify the user was created by searching via the user API
	user, err := testutils.FindUserByAttribute("username", username)
	if err != nil {
		ts.T().Fatalf("Failed to retrieve user by username: %v", err)
	}
	ts.Require().NotNil(user, "User should be found in user list after registration")

	// Store the created user for cleanup
	if user != nil {
		ts.config.CreatedUserIDs = append(ts.config.CreatedUserIDs, user.ID)
	}
}

// TestBasicRegistrationFlow_WithoutTokenConfig tests that userType and OU attributes are NOT included
// in JWT assertion when TokenConfig is not specified.
func (ts *BasicRegistrationFlowTestSuite) TestBasicRegistrationFlow_WithoutTokenConfig() {
	// Look up the default registration flow
	regFlowID, err := testutils.GetFlowIDByHandle("default-basic-flow", "REGISTRATION")
	ts.Require().NoError(err, "Failed to get default registration flow ID")

	// Create a new application without TokenConfig
	appWithoutTokenConfig := testutils.Application{
		Name:                      "Registration Flow Test Application Without Token Config",
		OUID:                      ts.testOUID,
		Description:               "Application for testing default behavior without token config",
		IsRegistrationFlowEnabled: true,
		RegistrationFlowID:        regFlowID,
		ClientID:                  "reg_flow_test_client_no_token_config",
		ClientSecret:              "reg_flow_test_secret_no_token_config",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{testUserType.Name},
		// TokenConfig is nil - not specified
	}

	appID, err := testutils.CreateApplication(appWithoutTokenConfig)
	ts.Require().NoError(err, "Failed to create application without token config")
	defer func() {
		if err := testutils.DeleteApplication(appID); err != nil {
			ts.T().Logf("Failed to delete test application: %v", err)
		}
	}()

	// Generate unique username for this test
	username := common.GenerateUniqueUsername("reguser")

	// Execute registration flow
	flowStep, err := common.InitiateRegistrationFlow(appID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate registration flow")

	inputs := map[string]string{
		"username": username,
		"password": "testpassword123",
	}
	completeFlowStep, err := common.CompleteFlow(flowStep.ExecutionID, inputs, "action_credentials",
		flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to complete registration flow")

	inputs = map[string]string{
		"email":       username + "@example.com",
		"given_name":  "Test",
		"family_name": "User",
	}
	completeFlowStep, err = common.CompleteFlow(completeFlowStep.ExecutionID, inputs, "action_schema_attrs",
		completeFlowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to complete registration flow with additional attributes")

	ts.Require().Equal("COMPLETE", completeFlowStep.FlowStatus, "Expected flow status to be COMPLETE")
	ts.Require().NotEmpty(completeFlowStep.Assertion, "JWT assertion should be returned")

	// Decode and validate JWT claims
	jwtClaims, err := testutils.DecodeJWT(completeFlowStep.Assertion)
	ts.Require().NoError(err, "Failed to decode JWT assertion")
	ts.Require().NotNil(jwtClaims, "JWT claims should not be nil")

	// Verify standard claims are present
	ts.Require().Equal(appID, jwtClaims.Aud, "JWT aud should match app ID")
	ts.Require().NotEmpty(jwtClaims.Sub, "JWT subject should not be empty")

	// Verify userType and OU attributes are NOT present (since TokenConfig is not specified)
	ts.Require().Empty(jwtClaims.UserType, "userType should NOT be present when TokenConfig is not specified")
	ts.Require().Empty(jwtClaims.OUID, "ouId should NOT be present when TokenConfig is not specified")
	ts.Require().Empty(jwtClaims.OuName, "ouName should NOT be present when TokenConfig is not specified")
	ts.Require().Empty(jwtClaims.OuHandle, "ouHandle should NOT be present when TokenConfig is not specified")
}

// TestBasicRegistrationFlow_WithEmptyUserAttributes tests that userType and OU attributes are NOT included
// in JWT assertion when user_attributes is an empty array.
func (ts *BasicRegistrationFlowTestSuite) TestBasicRegistrationFlow_WithEmptyUserAttributes() {
	// Look up the default registration flow
	regFlowID, err := testutils.GetFlowIDByHandle("default-basic-flow", "REGISTRATION")
	ts.Require().NoError(err, "Failed to get default registration flow ID")

	// Create a new application with empty user_attributes
	appWithEmptyAttrs := testutils.Application{
		Name:                      "Registration Flow Test Application With Empty User Attributes",
		OUID:                      ts.testOUID,
		Description:               "Application for testing behavior with empty user_attributes",
		IsRegistrationFlowEnabled: true,
		RegistrationFlowID:        regFlowID,
		ClientID:                  "reg_flow_test_client_empty_attrs",
		ClientSecret:              "reg_flow_test_secret_empty_attrs",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{testUserType.Name},
		AssertionConfig: map[string]interface{}{
			"userAttributes": []string{}, // Empty array
		},
	}

	appID, err := testutils.CreateApplication(appWithEmptyAttrs)
	ts.Require().NoError(err, "Failed to create application with empty user attributes")
	defer func() {
		if err := testutils.DeleteApplication(appID); err != nil {
			ts.T().Logf("Failed to delete test application: %v", err)
		}
	}()

	// Generate unique username for this test
	username := common.GenerateUniqueUsername("reguser")

	// Execute registration flow
	flowStep, err := common.InitiateRegistrationFlow(appID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate registration flow")

	inputs := map[string]string{
		"username": username,
		"password": "testpassword123",
	}
	completeFlowStep, err := common.CompleteFlow(flowStep.ExecutionID, inputs, "action_credentials",
		flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to complete registration flow")

	inputs = map[string]string{
		"email":       username + "@example.com",
		"given_name":  "Test",
		"family_name": "User",
	}
	completeFlowStep, err = common.CompleteFlow(completeFlowStep.ExecutionID, inputs, "action_schema_attrs",
		completeFlowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to complete registration flow with additional attributes")

	ts.Require().Equal("COMPLETE", completeFlowStep.FlowStatus, "Expected flow status to be COMPLETE")
	ts.Require().NotEmpty(completeFlowStep.Assertion, "JWT assertion should be returned")

	// Decode and validate JWT claims
	jwtClaims, err := testutils.DecodeJWT(completeFlowStep.Assertion)
	ts.Require().NoError(err, "Failed to decode JWT assertion")
	ts.Require().NotNil(jwtClaims, "JWT claims should not be nil")

	// Verify standard claims are present
	ts.Require().Equal(appID, jwtClaims.Aud, "JWT aud should match app ID")
	ts.Require().NotEmpty(jwtClaims.Sub, "JWT subject should not be empty")

	// Verify userType and OU attributes are NOT present (since user_attributes is empty)
	ts.Require().Empty(jwtClaims.UserType, "userType should NOT be present when user_attributes is empty")
	ts.Require().Empty(jwtClaims.OUID, "ouId should NOT be present when user_attributes is empty")
	ts.Require().Empty(jwtClaims.OuName, "ouName should NOT be present when user_attributes is empty")
	ts.Require().Empty(jwtClaims.OuHandle, "ouHandle should NOT be present when user_attributes is empty")
}

// TestSchemaDriverInputs_DynamicPromptForMissingRequiredAttrs verifies that when only credentials
// are submitted, the provisioning executor detects schema-required attributes and triggers an
// additional prompt step carrying those inputs dynamically.
func (ts *BasicRegistrationFlowTestSuite) TestSchemaDriverInputs_DynamicPromptForMissingRequiredAttrs() {
	username := common.GenerateUniqueUsername("schemauser")

	// Step 1: Initiate flow — credentials only, no schema attrs.
	flowStep, err := common.InitiateRegistrationFlow(ts.testAppID, false, nil, "")
	ts.Require().NoError(err)

	inputs := map[string]string{
		"username": username,
		"password": "testpassword123",
	}
	flowStep, err = common.CompleteFlow(flowStep.ExecutionID, inputs, "action_credentials", flowStep.ChallengeToken)
	ts.Require().NoError(err)

	// Step 2: Expect an incomplete step prompting for schema-required attrs.
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)
	ts.Require().NotEmpty(flowStep.Data.Inputs, "schema-required inputs should be present in the prompt")
	ts.Require().True(common.ValidateRequiredInputs(flowStep.Data.Inputs, []string{"email", "given_name", "family_name"}),
		"email, given_name, family_name must be dynamically prompted")
	ts.Require().False(common.HasInput(flowStep.Data.Inputs, "username"),
		"username should not appear again — it was already provided")
	ts.Require().False(common.HasInput(flowStep.Data.Inputs, "mobileNumber"),
		"optional mobileNumber should not be prompted when not in node inputs")

	// Step 3: Submit schema attrs and complete.
	inputs = map[string]string{
		"email":       username + "@example.com",
		"given_name":  "Schema",
		"family_name": "User",
	}
	flowStep, err = common.CompleteFlow(flowStep.ExecutionID, inputs, "action_schema_attrs", flowStep.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus)

	user, err := testutils.FindUserByAttribute("username", username)
	ts.Require().NoError(err)
	ts.Require().NotNil(user)
	ts.config.CreatedUserIDs = append(ts.config.CreatedUserIDs, user.ID)
}

// TestSchemaDriverInputs_NoPromptWhenAllInputsPresent verifies that when all schema-required
// attributes are provided upfront in the initial request, the flow completes without an
// additional schema-attrs prompt step.
func (ts *BasicRegistrationFlowTestSuite) TestSchemaDriverInputs_NoPromptWhenAllInputsPresent() {
	username := common.GenerateUniqueUsername("nopromptuser")

	inputs := map[string]string{
		"username":    username,
		"password":    "testpassword123",
		"email":       username + "@example.com",
		"given_name":  "No",
		"family_name": "Prompt",
	}

	flowStep, err := common.InitiateRegistrationFlow(ts.testAppID, false, inputs, "")
	ts.Require().NoError(err)
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus,
		"flow should complete in one step when all required attrs are provided")
	ts.Require().NotEmpty(flowStep.Assertion)

	user, err := testutils.FindUserByAttribute("username", username)
	ts.Require().NoError(err)
	ts.Require().NotNil(user)
	ts.config.CreatedUserIDs = append(ts.config.CreatedUserIDs, user.ID)
}

// TestSchemaDriverInputs_OptionalAttrProvisionedWhenInNodeInputs verifies that an optional
// schema attribute (mobileNumber) is collected and stored when it is explicitly listed as a
// node input in the provisioning executor configuration.
func (ts *BasicRegistrationFlowTestSuite) TestSchemaDriverInputs_OptionalAttrProvisionedWhenInNodeInputs() {
	// Create a custom flow that adds mobileNumber as a node input on the provisioning executor.
	optionalAttrFlow := testutils.Flow{
		Name:     "Optional Attr Registration Flow",
		Handle:   "optional-attr-reg-flow",
		FlowType: "REGISTRATION",
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "user_type_resolver"},
			{
				"id":   "user_type_resolver",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "UserTypeResolver",
				},
				"onSuccess":    "prompt_credentials",
				"onIncomplete": "prompt_usertype",
			},
			{
				"id":   "prompt_usertype",
				"type": "PROMPT",
				"meta": map[string]interface{}{
					"components": []map[string]interface{}{
						{"type": "BLOCK", "id": "block_usertype", "components": []map[string]interface{}{
							{"type": "SELECT", "id": "usertype_input", "ref": "userType", "label": "User Type", "required": true, "options": []string{}},
							{"type": "ACTION", "id": "action_usertype", "label": "Continue", "variant": "PRIMARY", "eventType": "SUBMIT"},
						}},
					},
				},
				"prompts": []map[string]interface{}{
					{"inputs": []map[string]interface{}{{"ref": "usertype_input", "identifier": "userType", "type": "SELECT", "required": true}},
						"action": map[string]interface{}{"ref": "action_usertype", "nextNode": "user_type_resolver"}},
				},
			},
			{
				"id":   "prompt_credentials",
				"type": "PROMPT",
				"meta": map[string]interface{}{
					"components": []map[string]interface{}{
						{"type": "BLOCK", "id": "block_creds", "components": []map[string]interface{}{
							{"type": "TEXT_INPUT", "id": "input_username", "ref": "username", "label": "Username", "required": true},
							{"type": "PASSWORD_INPUT", "id": "input_password", "ref": "password", "label": "Password", "required": true},
							{"type": "ACTION", "id": "action_credentials", "label": "Continue", "variant": "PRIMARY", "eventType": "SUBMIT"},
						}},
					},
				},
				"prompts": []map[string]interface{}{
					{"inputs": []map[string]interface{}{
						{"ref": "input_username", "identifier": "username", "type": "TEXT_INPUT", "required": true},
						{"ref": "input_password", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
					}, "action": map[string]interface{}{"ref": "action_credentials", "nextNode": "basic_auth"}},
				},
			},
			{
				"id":   "basic_auth",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "BasicAuthExecutor",
				},
				"onSuccess": "provisioning",
			},
			{
				"id":   "provisioning",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "ProvisioningExecutor",
					"inputs": []map[string]interface{}{
						{"ref": "input_001", "identifier": "username", "type": "TEXT_INPUT", "required": true},
						{"ref": "input_002", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
						{"ref": "input_003", "identifier": "mobileNumber", "type": "TEXT_INPUT", "required": false},
					},
				},
				"onSuccess":    "auth_assert",
				"onIncomplete": "prompt_schema_attrs",
			},
			{
				"id":   "prompt_schema_attrs",
				"type": "PROMPT",
				"meta": map[string]interface{}{
					"components": []map[string]interface{}{
						{"align": "center", "type": "TEXT", "id": "heading_schema_attrs", "label": "Complete Your Profile", "variant": "HEADING_1"},
						{"type": "BLOCK", "id": "block_dynamic_user_inputs", "components": []map[string]interface{}{
							{"type": "ACTION", "id": "action_schema_attrs", "label": "Continue", "variant": "PRIMARY", "eventType": "SUBMIT"},
						}},
					},
				},
				"prompts": []map[string]interface{}{
					{"inputs": []map[string]interface{}{}, "action": map[string]interface{}{"ref": "action_schema_attrs", "nextNode": "provisioning"}},
				},
			},
			{
				"id":   "auth_assert",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "AuthAssertExecutor",
				},
				"onSuccess": "end",
			},
			{"id": "end", "type": "END"},
		},
	}

	flowID, err := testutils.CreateFlow(optionalAttrFlow)
	ts.Require().NoError(err, "Failed to create optional attr flow")
	defer func() {
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("Failed to delete optional attr flow: %v", err)
		}
	}()

	appID, err := testutils.CreateApplication(testutils.Application{
		OUID:                      ts.testOUID,
		Name:                      "Optional Attr Test App",
		IsRegistrationFlowEnabled: true,
		ClientID:                  "optional_attr_test_client",
		ClientSecret:              "optional_attr_test_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{testUserType.Name},
		RegistrationFlowID:        flowID,
	})
	ts.Require().NoError(err, "Failed to create test app")
	defer func() {
		if err := testutils.DeleteApplication(appID); err != nil {
			ts.T().Logf("Failed to delete test app: %v", err)
		}
	}()

	username := common.GenerateUniqueUsername("optionaluser")
	mobile := "+94771234567"

	// Initiate with credentials.
	flowStep, err := common.InitiateRegistrationFlow(appID, false, nil, "")
	ts.Require().NoError(err)

	inputs := map[string]string{"username": username, "password": "testpassword123"}
	flowStep, err = common.CompleteFlow(flowStep.ExecutionID, inputs, "action_credentials", flowStep.ChallengeToken)
	ts.Require().NoError(err)

	// Submit schema-required attrs and the optional mobileNumber.
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)
	inputs = map[string]string{
		"email":        username + "@example.com",
		"given_name":   "Optional",
		"family_name":  "User",
		"mobileNumber": mobile,
	}
	flowStep, err = common.CompleteFlow(flowStep.ExecutionID, inputs, "action_schema_attrs", flowStep.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus)

	// Verify mobileNumber was stored.
	user, err := testutils.FindUserByAttribute("username", username)
	ts.Require().NoError(err)
	ts.Require().NotNil(user)
	userAttrs, err := testutils.GetUserAttributes(*user)
	ts.Require().NoError(err)
	ts.Require().Equal(mobile, userAttrs["mobileNumber"],
		"optional mobileNumber should be provisioned when listed in node inputs")
	ts.config.CreatedUserIDs = append(ts.config.CreatedUserIDs, user.ID)
}

// TestSchemaDriverInputs_VerboseModeReturnsDynamicMeta verifies that in verbose mode the
// prompt_schema_attrs step includes synthetic meta components for each schema-derived input
// that has no pre-configured meta component in the flow graph node.
func (ts *BasicRegistrationFlowTestSuite) TestSchemaDriverInputs_VerboseModeReturnsDynamicMeta() {
	username := common.GenerateUniqueUsername("verboseuser")

	// Initiate with verbose=true and credentials only.
	flowStep, err := common.InitiateRegistrationFlow(ts.testAppID, true, nil, "")
	ts.Require().NoError(err)

	inputs := map[string]string{"username": username, "password": "testpassword123"}
	flowStep, err = common.CompleteFlow(flowStep.ExecutionID, inputs, "action_credentials", flowStep.ChallengeToken)
	ts.Require().NoError(err)

	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)

	// Verbose mode must return meta.
	ts.Require().NotNil(flowStep.Data.Meta, "meta should be present in verbose mode")

	metaMap, ok := flowStep.Data.Meta.(map[string]interface{})
	ts.Require().True(ok, "meta should be a JSON object")

	components, ok := metaMap["components"].([]interface{})
	ts.Require().True(ok, "meta.components should be an array")
	ts.Require().NotEmpty(components, "meta.components should not be empty")

	// Collect all component refs/ids from the meta tree.
	componentRefs := collectMetaComponentRefs(components)
	for _, attr := range []string{"email", "given_name", "family_name"} {
		ts.Require().True(componentRefs[attr],
			"synthetic meta component must exist for schema-derived input %q", attr)
	}

	// Complete registration.
	inputs = map[string]string{
		"email":       username + "@example.com",
		"given_name":  "Verbose",
		"family_name": "User",
	}
	flowStep, err = common.CompleteFlow(flowStep.ExecutionID, inputs, "action_schema_attrs", flowStep.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus)

	user, err := testutils.FindUserByAttribute("username", username)
	ts.Require().NoError(err)
	ts.Require().NotNil(user)
	ts.config.CreatedUserIDs = append(ts.config.CreatedUserIDs, user.ID)
}

// TestSchemaDriverInputs_PromptedInputsHaveRequiredTrue verifies that all schema-required inputs
// surfaced in the INCOMPLETE prompt step carry required=true.
func (ts *BasicRegistrationFlowTestSuite) TestSchemaDriverInputs_PromptedInputsHaveRequiredTrue() {
	username := common.GenerateUniqueUsername("reqflaguser")

	flowStep, err := common.InitiateRegistrationFlow(ts.testAppID, false, nil, "")
	ts.Require().NoError(err)

	inputs := map[string]string{"username": username, "password": "testpassword123"}
	flowStep, err = common.CompleteFlow(flowStep.ExecutionID, inputs, "action_credentials", flowStep.ChallengeToken)
	ts.Require().NoError(err)

	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)
	ts.Require().NotEmpty(flowStep.Data.Inputs)
	for _, inp := range flowStep.Data.Inputs {
		ts.Require().True(inp.Required,
			"schema-required attr %q must have required=true in the prompt response", inp.Identifier)
	}
}

// TestSchemaDriverInputs_VerboseMeta_NoDynamicPlaceholder verifies that the
// DYNAMIC_INPUT_PLACEHOLDER sentinel component is replaced and never appears in
// the meta returned to the client.
func (ts *BasicRegistrationFlowTestSuite) TestSchemaDriverInputs_VerboseMeta_NoDynamicPlaceholder() {
	username := common.GenerateUniqueUsername("noplaceholder")

	flowStep, err := common.InitiateRegistrationFlow(ts.testAppID, true, nil, "")
	ts.Require().NoError(err)

	inputs := map[string]string{"username": username, "password": "testpassword123"}
	flowStep, err = common.CompleteFlow(flowStep.ExecutionID, inputs, "action_credentials", flowStep.ChallengeToken)
	ts.Require().NoError(err)

	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)
	ts.Require().NotNil(flowStep.Data.Meta)

	metaMap, ok := flowStep.Data.Meta.(map[string]interface{})
	ts.Require().True(ok)
	comps, ok := metaMap["components"].([]interface{})
	ts.Require().True(ok)

	types := collectMetaComponentTypes(comps)
	ts.Require().False(types["DYNAMIC_INPUT_PLACEHOLDER"],
		"DYNAMIC_INPUT_PLACEHOLDER must be replaced, not present in the meta output")
}

// TestSchemaDriverInputs_DisplayNameUsedAsMetaLabel verifies that when a schema attribute
// has a displayName, the synthetic meta component generated for it in verbose mode uses
// the displayName as the label, not the attribute identifier.
func (ts *BasicRegistrationFlowTestSuite) TestSchemaDriverInputs_DisplayNameUsedAsMetaLabel() {
	schemaWithDisplayName := testutils.UserType{
		Name: "dn-label-test-type",
		OUID: ts.testOUID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
			"password": map[string]interface{}{"type": "string", "credential": true},
			"email": map[string]interface{}{
				"type":        "string",
				"required":    true,
				"displayName": "Email Address",
			},
			"given_name": map[string]interface{}{
				"type":     "string",
				"required": true,
			},
		},
		AllowSelfRegistration: true,
	}
	schemaID, err := testutils.CreateUserType(schemaWithDisplayName)
	ts.Require().NoError(err, "Failed to create displayName test schema")
	defer func() {
		if err := testutils.DeleteUserType(schemaID); err != nil {
			ts.T().Logf("Failed to delete displayName test schema: %v", err)
		}
	}()

	regFlowID, err := testutils.GetFlowIDByHandle("default-basic-flow", "REGISTRATION")
	ts.Require().NoError(err, "Failed to get default registration flow ID")

	appID, err := testutils.CreateApplication(testutils.Application{
		OUID:                      ts.testOUID,
		Name:                      "DisplayName Label Test App",
		IsRegistrationFlowEnabled: true,
		RegistrationFlowID:        regFlowID,
		ClientID:                  "dn_label_test_client",
		ClientSecret:              "dn_label_test_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{schemaWithDisplayName.Name},
	})
	ts.Require().NoError(err, "Failed to create displayName test app")
	defer func() {
		if err := testutils.DeleteApplication(appID); err != nil {
			ts.T().Logf("Failed to delete displayName test app: %v", err)
		}
	}()

	username := common.GenerateUniqueUsername("dnlabeluser")

	flowStep, err := common.InitiateRegistrationFlow(appID, true, nil, "")
	ts.Require().NoError(err)

	inputs := map[string]string{"username": username, "password": "testpassword123"}
	flowStep, err = common.CompleteFlow(flowStep.ExecutionID, inputs, "action_credentials", flowStep.ChallengeToken)
	ts.Require().NoError(err)

	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)
	ts.Require().NotNil(flowStep.Data.Meta, "verbose meta must be present in schema-prompting step")

	metaMap, ok := flowStep.Data.Meta.(map[string]interface{})
	ts.Require().True(ok)
	comps, ok := metaMap["components"].([]interface{})
	ts.Require().True(ok)

	emailLabel, found := findMetaComponentLabel(comps, "email")
	ts.Require().True(found, "synthetic meta component for 'email' must exist")
	ts.Require().Equal("Email Address", emailLabel,
		"displayName must be used as the label for schema-derived inputs with displayName set")

	givenNameLabel, found := findMetaComponentLabel(comps, "given_name")
	ts.Require().True(found, "synthetic meta component for 'given_name' must exist")
	ts.Require().Equal("given_name", givenNameLabel,
		"identifier must be used as fallback label when displayName is absent")
}

// collectMetaComponentRefs recursively collects "ref" and "id" values from a meta components tree.
func collectMetaComponentRefs(components []interface{}) map[string]bool {
	refs := make(map[string]bool)
	for _, c := range components {
		comp, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		if ref, ok := comp["ref"].(string); ok && ref != "" {
			refs[ref] = true
		}
		if id, ok := comp["id"].(string); ok && id != "" {
			refs[id] = true
		}
		if children, ok := comp["components"].([]interface{}); ok {
			for k, v := range collectMetaComponentRefs(children) {
				refs[k] = v
			}
		}
	}
	return refs
}

// collectMetaComponentTypes recursively collects all "type" values from a meta components tree.
func collectMetaComponentTypes(components []interface{}) map[string]bool {
	types := make(map[string]bool)
	for _, c := range components {
		comp, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		if t, ok := comp["type"].(string); ok && t != "" {
			types[t] = true
		}
		if children, ok := comp["components"].([]interface{}); ok {
			for k, v := range collectMetaComponentTypes(children) {
				types[k] = v
			}
		}
	}
	return types
}

// findMetaComponentLabel recursively searches for a component by ref or id and returns its label.
func findMetaComponentLabel(components []interface{}, ref string) (string, bool) {
	for _, c := range components {
		comp, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		compRef, _ := comp["ref"].(string)
		compID, _ := comp["id"].(string)
		if compRef == ref || compID == ref {
			if label, ok := comp["label"].(string); ok {
				return label, true
			}
		}
		if children, ok := comp["components"].([]interface{}); ok {
			if label, found := findMetaComponentLabel(children, ref); found {
				return label, found
			}
		}
	}
	return "", false
}
