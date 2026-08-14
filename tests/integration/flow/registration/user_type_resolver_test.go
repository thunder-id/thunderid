// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package registration

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

type UserTypeResolverRuntimeTestSuite struct {
	suite.Suite
	testOUID1         string
	testUserTypeID1   string
	testUserTypeID2   string
	testUserTypeID3   string
	testUserTypeName1 string
	testUserTypeName2 string
	testUserTypeName3 string
	narrowAuthFlowID  string
	createdAppIDs     []string
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

	// A third type so the allowedUserTypes node property can narrow 3 down to 2. Narrowing to a
	// single type would auto-select instead of prompting (user_type_resolver.go:162).
	userType3 := testutils.UserType{
		Name:                  "runtime-test-contractor",
		OUID:                  ts.testOUID1,
		AllowSelfRegistration: true,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
			"password": map[string]interface{}{"type": "string", "credential": true},
			"email":    map[string]interface{}{"type": "string"},
		},
	}
	userTypeID3, err := testutils.CreateUserType(userType3)
	if err != nil {
		ts.T().Fatalf("Failed to create third test user type: %v", err)
	}
	ts.testUserTypeID3 = userTypeID3
	ts.testUserTypeName3 = userType3.Name

	// Custom registration flows need an isolated auth flow, else the app's default auth flow CALLs
	// the default registration flow and the server rejects the mismatch with APP-1039.
	narrowAuthFlowID, err := testutils.CreateIsolatedAuthFlow("resolver-narrow-isolated-auth")
	if err != nil {
		ts.T().Fatalf("Failed to create isolated auth flow: %v", err)
	}
	ts.narrowAuthFlowID = narrowAuthFlowID
	ts.createdFlowIDs = append(ts.createdFlowIDs, narrowAuthFlowID)

	// Look up the default registration flow ID
	regFlowID, err := testutils.GetFlowIDByHandle("default-flow", "REGISTRATION")
	if err != nil {
		ts.T().Fatalf("Failed to get default registration flow ID: %v", err)
	}

	// Create test application with two user types (triggers user type selection).
	// This test uses the default registration flow, which CALLs the default auth flow. Let the
	// server default AuthFlowID to the default auth flow so the two remain consistent.
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

	// Delete applications before flows: an application still referencing a flow can make the flow
	// deletion fail, leaving a stale handle that breaks the next run's SetupSuite.
	if ts.testAppID != "" {
		if err := testutils.DeleteApplication(ts.testAppID); err != nil {
			ts.T().Logf("Failed to delete test application: %v", err)
		}
	}

	// Delete applications created by the allowedUserTypes narrowing tests
	for _, appID := range ts.createdAppIDs {
		if err := testutils.DeleteApplication(appID); err != nil {
			ts.T().Logf("Failed to delete test application %s: %v", appID, err)
		}
	}

	// Delete test flows
	for _, flowID := range ts.createdFlowIDs {
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("Failed to delete test flow %s: %v", flowID, err)
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
	if ts.testUserTypeID3 != "" {
		if err := testutils.DeleteUserType(ts.testUserTypeID3); err != nil {
			ts.T().Logf("Failed to delete third test user type: %v", err)
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

// buildResolverFlowWithAllowedTypes returns a registration flow whose UserTypeResolver node carries
// an allowedUserTypes property. allowedTypes is passed through verbatim so a non-array value can be
// tested. REGISTRATION requires both UserTypeResolver and ProvisioningExecutor.
func buildResolverFlowWithAllowedTypes(handle string, allowedTypes interface{}) testutils.Flow {
	resolverNode := map[string]interface{}{
		"id":        "resolve_user_type",
		"type":      "TASK_EXECUTION",
		"executor":  map[string]interface{}{"name": "UserTypeResolver"},
		"onSuccess": "provision_user",
	}
	if allowedTypes != nil {
		resolverNode["properties"] = map[string]interface{}{"allowedUserTypes": allowedTypes}
	}

	return testutils.Flow{
		Name:     "User Type Resolver Allowed Types Flow",
		FlowType: "REGISTRATION",
		Handle:   handle,
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "resolve_user_type"},
			resolverNode,
			{
				"id":        "provision_user",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "ProvisioningExecutor"},
				"onSuccess": "end",
			},
			{"id": "end", "type": "END"},
		},
	}
}

// appForResolverFlow creates the flow plus a bound app allowing all three suite user types.
func (ts *UserTypeResolverRuntimeTestSuite) appForResolverFlow(
	flow testutils.Flow, name, clientID string) string {
	flowID, err := testutils.CreateFlow(flow)
	ts.Require().NoError(err, "Failed to create flow %s", flow.Handle)
	ts.createdFlowIDs = append(ts.createdFlowIDs, flowID)

	appID, err := testutils.CreateApplication(testutils.Application{
		OUID:                      ts.testOUID1,
		Name:                      name,
		IsRegistrationFlowEnabled: true,
		RegistrationFlowID:        flowID,
		ClientID:                  clientID,
		ClientSecret:              clientID + "_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes: []string{
			ts.testUserTypeName1, ts.testUserTypeName2, ts.testUserTypeName3,
		},
		AuthFlowID: ts.narrowAuthFlowID,
	})
	ts.Require().NoError(err, "Failed to create app %s", name)
	ts.createdAppIDs = append(ts.createdAppIDs, appID)
	return appID
}

// findResolverInput returns the input with the given identifier, or nil.
func findResolverInput(inputs []common.Inputs, identifier string) *common.Inputs {
	for i := range inputs {
		if inputs[i].Identifier == identifier {
			return &inputs[i]
		}
	}
	return nil
}

// Scenario 23: the allowedUserTypes node property narrows the offered user types even though the
// app allows all three (user_type_resolver.go:325-334).
func (ts *UserTypeResolverRuntimeTestSuite) TestNodePropertyNarrowsAllowedUserTypes() {
	appID := ts.appForResolverFlow(
		buildResolverFlowWithAllowedTypes("reg_flow_resolver_narrowed",
			[]interface{}{ts.testUserTypeName1, ts.testUserTypeName2}),
		"Resolver Narrowed App", "resolver_narrowed_client")

	flowStep, err := common.InitiateRegistrationFlow(appID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate the narrowed resolver flow")

	userTypeInput := findResolverInput(flowStep.Data.Inputs, "userType")
	ts.Require().NotNil(userTypeInput, "Expected a userType selection input")
	ts.ElementsMatch([]string{ts.testUserTypeName1, ts.testUserTypeName2}, userTypeInput.Options,
		"The node property must narrow the offered user types to the two it lists")
	ts.NotContains(userTypeInput.Options, ts.testUserTypeName3,
		"A user type excluded by the node property must not be offered")
}

// Scenario 24: a malformed allowedUserTypes property must not lock every user type out and break
// registration. The resolver ignores a non-array value (user_type_resolver.go:302-306) and the
// empty result means "no restriction" (:328-330), so the flow stays usable.
func (ts *UserTypeResolverRuntimeTestSuite) TestMalformedAllowedUserTypesDoesNotBreakRegistration() {
	appID := ts.appForResolverFlow(
		buildResolverFlowWithAllowedTypes("reg_flow_resolver_bad_prop", ts.testUserTypeName1),
		"Resolver Bad Property App", "resolver_badprop_client")

	flowStep, err := common.InitiateRegistrationFlow(appID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate the bad-property resolver flow")

	userTypeInput := findResolverInput(flowStep.Data.Inputs, "userType")
	ts.Require().NotNil(userTypeInput, "Expected a userType selection input")
	ts.NotEmpty(userTypeInput.Options,
		"A malformed property must not leave registration with zero selectable types")
	ts.ElementsMatch(
		[]string{ts.testUserTypeName1, ts.testUserTypeName2, ts.testUserTypeName3},
		userTypeInput.Options,
		"A malformed property must fall back to every user type the app allows")
}
