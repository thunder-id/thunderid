/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

type UserTypeResolverRuntimeTestSuite struct {
	suite.Suite
	testOUID1         string
	testUserTypeID1   string
	testUserTypeID2   string
	testUserTypeName1 string
	testUserTypeName2 string
	testAppID         string
	createdFlowIDs    []string
	createdUserIDs    []string
}

func TestUserTypeResolverRuntimeTestSuite(t *testing.T) {
	suite.Run(t, new(UserTypeResolverRuntimeTestSuite))
}

func (ts *UserTypeResolverRuntimeTestSuite) SetupSuite() {
	// Create OU
	ou1 := testutils.OrganizationUnit{
		Handle:      "runtime-meta-test-ou-1",
		Name:        "Runtime Meta Test OU 1",
		Description: "First OU for runtime meta testing",
	}
	ouID1, err := testutils.CreateOrganizationUnit(ou1)
	if err != nil {
		ts.T().Fatalf("Failed to create first test organization unit: %v", err)
	}
	ts.testOUID1 = ouID1

	// Create first user type with self-registration enabled
	userType1 := testutils.UserType{
		Name:                  "runtime-test-customer",
		OUID:                  ts.testOUID1,
		AllowSelfRegistration: true,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
			"password": map[string]interface{}{"type": "string", "credential": true},
			"email":    map[string]interface{}{"type": "string"},
		},
	}
	userTypeID1, err := testutils.CreateUserType(userType1)
	if err != nil {
		ts.T().Fatalf("Failed to create first test user type: %v", err)
	}
	ts.testUserTypeID1 = userTypeID1
	ts.testUserTypeName1 = userType1.Name

	// Create second user type with self-registration enabled
	userType2 := testutils.UserType{
		Name:                  "runtime-test-employee",
		OUID:                  ts.testOUID1,
		AllowSelfRegistration: true,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
			"password": map[string]interface{}{"type": "string", "credential": true},
			"email":    map[string]interface{}{"type": "string"},
		},
	}
	userTypeID2, err := testutils.CreateUserType(userType2)
	if err != nil {
		ts.T().Fatalf("Failed to create second test user type: %v", err)
	}
	ts.testUserTypeID2 = userTypeID2
	ts.testUserTypeName2 = userType2.Name

	// Look up the default registration flow ID
	regFlowID, err := testutils.GetFlowIDByHandle("default-basic-flow", "REGISTRATION")
	if err != nil {
		ts.T().Fatalf("Failed to get default registration flow ID: %v", err)
	}

	// Create test application with two user types (triggers user type selection)
	testApp := testutils.Application{
		OUID:                      ts.testOUID1,
		Name:                      "Runtime Meta Test Application",
		Description:               "Application for testing runtime meta generation",
		IsRegistrationFlowEnabled: true,
		RegistrationFlowID:        regFlowID,
		ClientID:                  "runtime_meta_test_client",
		ClientSecret:              "runtime_meta_test_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{ts.testUserTypeName1, ts.testUserTypeName2},
		AssertionConfig: map[string]interface{}{
			"userAttributes": []string{"userType", "ouId", "ouName", "ouHandle"},
		},
	}

	appID, err := testutils.CreateApplication(testApp)
	if err != nil {
		ts.T().Fatalf("Failed to create test application: %v", err)
	}
	ts.testAppID = appID
}

func (ts *UserTypeResolverRuntimeTestSuite) TearDownSuite() {
	// Cleanup users
	if err := testutils.CleanupUsers(ts.createdUserIDs); err != nil {
		ts.T().Logf("Failed to cleanup users: %v", err)
	}

	// Delete test flows
	for _, flowID := range ts.createdFlowIDs {
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("Failed to delete test flow %s: %v", flowID, err)
		}
	}

	// Delete test application
	if ts.testAppID != "" {
		if err := testutils.DeleteApplication(ts.testAppID); err != nil {
			ts.T().Logf("Failed to delete test application: %v", err)
		}
	}

	// Delete user types
	if ts.testUserTypeID1 != "" {
		if err := testutils.DeleteUserType(ts.testUserTypeID1); err != nil {
			ts.T().Logf("Failed to delete first test user type: %v", err)
		}
	}
	if ts.testUserTypeID2 != "" {
		if err := testutils.DeleteUserType(ts.testUserTypeID2); err != nil {
			ts.T().Logf("Failed to delete second test user type: %v", err)
		}
	}

	// Delete OU
	if ts.testOUID1 != "" {
		if err := testutils.DeleteOrganizationUnit(ts.testOUID1); err != nil {
			ts.T().Logf("Failed to delete first test OU: %v", err)
		}
	}
}

func (ts *UserTypeResolverRuntimeTestSuite) TestMetaReturnedWithVerbose() {
	// Initiate registration flow with verbose=true
	flowStep, err := common.InitiateRegistrationFlow(ts.testAppID, true, nil, "")
	ts.Require().NoError(err, "Failed to initiate registration flow")

	// Verify flow is waiting for user type selection
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus, "Expected flow status to be INCOMPLETE")
	ts.Require().Equal("VIEW", flowStep.Type, "Expected flow type to be VIEW")

	// Verify meta is returned
	ts.Require().NotNil(flowStep.Data.Meta, "Meta should be returned when verbose=true")

	// Verify meta has expected structure
	metaMap, ok := flowStep.Data.Meta.(map[string]interface{})
	ts.Require().True(ok, "Meta should be a map")
	ts.Require().NotEmpty(metaMap, "Meta should not be empty")

	// Verify components array exists
	components, ok := metaMap["components"].([]interface{})
	ts.Require().True(ok, "Meta should have components array")
	ts.Require().NotEmpty(components, "Components should not be empty")
}

func (ts *UserTypeResolverRuntimeTestSuite) TestSelectInputWithOptions() {
	// Initiate registration flow with verbose=true
	flowStep, err := common.InitiateRegistrationFlow(ts.testAppID, true, nil, "")
	ts.Require().NoError(err, "Failed to initiate registration flow")

	// Verify inputs are returned
	ts.Require().NotEmpty(flowStep.Data.Inputs, "Inputs should be returned")

	// Find the userType input
	var userTypeInput *common.Inputs
	for i := range flowStep.Data.Inputs {
		if flowStep.Data.Inputs[i].Identifier == "userType" {
			userTypeInput = &flowStep.Data.Inputs[i]
			break
		}
	}

	ts.Require().NotNil(userTypeInput, "userType input should be present")
	ts.Equal("SELECT", userTypeInput.Type, "userType input should be of type SELECT")
	ts.True(userTypeInput.Required, "userType input should be required")

	// Verify options contain both user types
	ts.Require().NotEmpty(userTypeInput.Options, "userType input should have options")
	ts.Require().GreaterOrEqual(len(userTypeInput.Options), 2, "Should have at least 2 options")

	// Verify both user types are in options
	foundUserType1 := false
	foundUserType2 := false
	for _, option := range userTypeInput.Options {
		if option == ts.testUserTypeName1 {
			foundUserType1 = true
		}
		if option == ts.testUserTypeName2 {
			foundUserType2 = true
		}
	}
	ts.True(foundUserType1, "Options should contain first user type: %s", ts.testUserTypeName1)
	ts.True(foundUserType2, "Options should contain second user type: %s", ts.testUserTypeName2)
}

func (ts *UserTypeResolverRuntimeTestSuite) TestMetaNotReturnedWithoutVerbose() {
	// Initiate registration flow with verbose=false
	flowStep, err := common.InitiateRegistrationFlow(ts.testAppID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate registration flow")

	// Verify flow is waiting for user type selection
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus, "Expected flow status to be INCOMPLETE")
	ts.Require().Equal("VIEW", flowStep.Type, "Expected flow type to be VIEW")

	// Verify inputs are still returned
	ts.Require().NotEmpty(flowStep.Data.Inputs, "Inputs should be returned even without verbose")

	// Verify meta is NOT returned when verbose=false
	ts.Nil(flowStep.Data.Meta, "Meta should NOT be returned when verbose=false")
}
