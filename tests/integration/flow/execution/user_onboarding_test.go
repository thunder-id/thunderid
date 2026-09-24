// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	// A user onboarding flow is app independent: it is resolved by the default handle configured in
	// the server-config flow section rather than from an application binding.
	onboardingFlowHandle = "user_onboarding_flow_test"

	errCodeOUNotFound            = "FET-1030"
	errCodeUserTypeNotAllowed    = "FET-1054"
	errCodeUserTypeNotValidForOU = "FET-1058"
)

type UserOnboardingTestSuite struct {
	suite.Suite
	config *common.TestSuiteConfig

	rootOUID      string
	childOUID     string
	rootTypeID    string
	childTypeID   string
	rootTypeName  string
	childTypeName string
	flowID        string
}

func TestUserOnboardingTestSuite(t *testing.T) {
	suite.Run(t, new(UserOnboardingTestSuite))
}

// onboardingFlowNodes builds the onboarding flow: the OU is chosen first from the full tree, the
// user type is then resolved against that OU, and the user is provisioned into it.
func onboardingFlowNodes(allowedUserTypes []string) []map[string]interface{} {
	return []map[string]interface{}{
		{
			"id":        "start",
			"type":      "START",
			"onSuccess": "ou_resolver",
		},
		{
			"id":   "ou_resolver",
			"type": "TASK_EXECUTION",
			"executor": map[string]interface{}{
				"name": "OUResolverExecutor",
			},
			"properties": map[string]interface{}{
				"resolveFrom": "promptAll",
			},
			"onSuccess":    "user_type_resolver",
			"onIncomplete": "prompt_ou",
		},
		{
			"id":   "prompt_ou",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{
				{
					"inputs": []map[string]interface{}{
						{
							"ref":        "ou_selection_input",
							"identifier": "ouId",
							"type":       "OU_SELECT",
							"required":   true,
						},
					},
					"action": map[string]interface{}{
						"ref":      "action_ou",
						"nextNode": "ou_resolver",
					},
				},
			},
		},
		{
			"id":   "user_type_resolver",
			"type": "TASK_EXECUTION",
			"executor": map[string]interface{}{
				"name": "UserTypeResolver",
			},
			"properties": map[string]interface{}{
				"allowedUserTypes": allowedUserTypes,
			},
			"onSuccess":    "provisioning",
			"onIncomplete": "prompt_usertype",
		},
		{
			"id":   "prompt_usertype",
			"type": "PROMPT",
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
			"id":   "provisioning",
			"type": "TASK_EXECUTION",
			"executor": map[string]interface{}{
				"name": "ProvisioningExecutor",
				"inputs": []map[string]interface{}{
					{
						"ref":        "input_username",
						"identifier": "username",
						"type":       "TEXT_INPUT",
						"required":   true,
					},
					{
						"ref":        "input_email",
						"identifier": "email",
						"type":       "TEXT_INPUT",
						"required":   true,
					},
				},
			},
			"onSuccess":    "end",
			"onIncomplete": "prompt_user_details",
		},
		{
			"id":   "prompt_user_details",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{
				{
					"inputs": []map[string]interface{}{
						{
							"ref":        "input_username",
							"identifier": "username",
							"type":       "TEXT_INPUT",
							"required":   true,
						},
						{
							"ref":        "input_email",
							"identifier": "email",
							"type":       "TEXT_INPUT",
							"required":   true,
						},
					},
					"action": map[string]interface{}{
						"ref":      "action_details",
						"nextNode": "provisioning",
					},
				},
			},
		},
		{
			"id":   "end",
			"type": "END",
		},
	}
}

func onboardingUserTypeSchema(name, ouID string) testutils.UserType {
	return testutils.UserType{
		Name: name,
		OUID: ouID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{
				"type": "string",
			},
			"email": map[string]interface{}{
				"type": "string",
			},
		},
	}
}

func (ts *UserOnboardingTestSuite) SetupSuite() {
	ts.config = &common.TestSuiteConfig{}

	rootOUID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "onboarding_root_test_ou",
		Name:        "Onboarding Root Test OU",
		Description: "Root organization unit for user onboarding testing",
		Parent:      nil,
	})
	ts.Require().NoError(err, "Failed to create root organization unit")
	ts.rootOUID = rootOUID

	childOUID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "onboarding_child_test_ou",
		Name:        "Onboarding Child Test OU",
		Description: "Child organization unit for user onboarding testing",
		Parent:      &rootOUID,
	})
	ts.Require().NoError(err, "Failed to create child organization unit")
	ts.childOUID = childOUID

	// One user type per OU, so which types are offered depends on the OU that was picked.
	ts.rootTypeName = "onboarding-root-person"
	ts.childTypeName = "onboarding-child-person"

	ts.rootTypeID, err = testutils.CreateUserType(onboardingUserTypeSchema(ts.rootTypeName, rootOUID))
	ts.Require().NoError(err, "Failed to create root user type")

	ts.childTypeID, err = testutils.CreateUserType(onboardingUserTypeSchema(ts.childTypeName, childOUID))
	ts.Require().NoError(err, "Failed to create child user type")

	ts.flowID, err = testutils.CreateFlow(testutils.Flow{
		Name:     "User Onboarding Test Flow",
		FlowType: "USER_ONBOARDING",
		Handle:   onboardingFlowHandle,
		Nodes:    onboardingFlowNodes([]string{ts.rootTypeName, ts.childTypeName}),
	})
	ts.Require().NoError(err, "Failed to create user onboarding flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, ts.flowID)

	// The onboarding flow is reached by its default handle, so the flow section has to name it.
	ts.setOnboardingDefaultHandle(onboardingFlowHandle)
}

func (ts *UserOnboardingTestSuite) TearDownSuite() {
	for _, userID := range ts.config.CreatedUserIDs {
		if err := testutils.DeleteUser(userID); err != nil {
			ts.T().Logf("Failed to delete onboarded user during teardown: %v", err)
		}
	}
	for _, flowID := range ts.config.CreatedFlowIDs {
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("Failed to delete test flow during teardown: %v", err)
		}
	}
	for _, typeID := range []string{ts.childTypeID, ts.rootTypeID} {
		if typeID == "" {
			continue
		}
		if err := testutils.DeleteUserType(typeID); err != nil {
			ts.T().Logf("Failed to delete test user type during teardown: %v", err)
		}
	}
	for _, ouID := range []string{ts.childOUID, ts.rootOUID} {
		if ouID == "" {
			continue
		}
		if err := testutils.DeleteOrganizationUnit(ouID); err != nil {
			ts.T().Logf("Failed to delete test organization unit during teardown: %v", err)
		}
	}
}

// setOnboardingDefaultHandle names the onboarding flow in the writable flow section, keeping every
// sibling key the section already carried, and registers the restore.
//
// The restore is registered on the suite's test cleanup rather than in TearDownSuite because
// testify skips teardown entirely when SetupSuite fails, which would leave the handle pointing at a
// flow this suite deletes.
func (ts *UserOnboardingTestSuite) setOnboardingDefaultHandle(handle string) {
	ts.T().Helper()

	original, err := testutils.MergeWritableServerConfig(flowConfigSection, map[string]interface{}{
		"userOnboardingFlow": map[string]interface{}{"defaultHandle": handle},
	})
	ts.Require().NoError(err, "Failed to update flow server config")
	ts.T().Cleanup(func() {
		if err := testutils.PutWritableServerConfig(flowConfigSection, original); err != nil {
			ts.T().Errorf("cleanup: failed to restore flow server config: %v", err)
		}
	})
}

// initiateOnboarding starts a user onboarding flow. It carries no application id, which is what
// distinguishes an app-independent system flow from every other flow type.
func (ts *UserOnboardingTestSuite) initiateOnboarding(inputs map[string]string) *common.FlowStep {
	ts.T().Helper()

	reqBody := map[string]interface{}{"flowType": "USER_ONBOARDING"}
	if len(inputs) > 0 {
		reqBody["inputs"] = inputs
	}

	payload, err := json.Marshal(reqBody)
	ts.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPost, testServerURL+"/flow/execute", bytes.NewReader(payload))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err, "Failed to send onboarding flow request")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, resp.StatusCode,
		"Onboarding initiation should succeed: %s", string(body))

	var step common.FlowStep
	ts.Require().NoError(json.Unmarshal(body, &step))
	return &step
}

// findOnboardedUser locates a provisioned user so the OU and type it landed in can be asserted,
// and tracks it for cleanup.
func (ts *UserOnboardingTestSuite) findOnboardedUser(username string) *testutils.User {
	ts.T().Helper()

	user, err := testutils.FindUserByAttribute("username", username)
	ts.Require().NoError(err, "Failed to look up the onboarded user")
	ts.Require().NotNil(user, "The onboarding flow should have created a user")

	if user.ID != "" {
		ts.config.CreatedUserIDs = append(ts.config.CreatedUserIDs, user.ID)
	}
	return user
}

// An onboarding flow with the promptAll strategy asks for the OU before anything else, and the
// selected OU then decides which user types are on offer. Picking the root OU leaves only the type
// attached to it, so the type is auto-selected and the flow goes straight to the user details.
func (ts *UserOnboardingTestSuite) TestOnboarding_RootOUAutoSelectsItsOnlyUserType() {
	step := ts.initiateOnboarding(nil)
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "Onboarding should pause to select an OU")
	ts.Require().True(common.HasInput(step.Data.Inputs, "ouId"),
		"The promptAll strategy must request an OU selection")

	step, err := common.CompleteFlow(step.ExecutionID,
		map[string]string{"ouId": ts.rootOUID}, "action_ou", step.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit the OU selection")
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "The flow should continue after the OU is chosen")
	ts.False(common.HasInput(step.Data.Inputs, "userType"),
		"A single valid user type should be auto-selected rather than prompted")
	ts.True(common.HasInput(step.Data.Inputs, "username"),
		"The flow should move on to collecting user details")

	username := common.GenerateUniqueUsername("onboard_root")
	completed, err := common.CompleteFlow(step.ExecutionID, map[string]string{
		"username": username,
		"email":    username + "@onboarding.test",
	}, "action_details", step.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit user details")
	ts.Require().Equal("COMPLETE", completed.FlowStatus, "Onboarding should provision the user")

	user := ts.findOnboardedUser(username)
	ts.Equal(ts.rootOUID, user.OUID, "The user must be provisioned into the selected OU")
	ts.Equal(ts.rootTypeName, user.Type, "The user must carry the resolved user type")
}

// Picking the child OU leaves both user types valid — the one attached to the child and the one
// attached to its ancestor — so the flow has to ask which to use.
func (ts *UserOnboardingTestSuite) TestOnboarding_ChildOUPromptsForUserType() {
	step := ts.initiateOnboarding(nil)
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "Onboarding should pause to select an OU")

	step, err := common.CompleteFlow(step.ExecutionID,
		map[string]string{"ouId": ts.childOUID}, "action_ou", step.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit the OU selection")
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "The flow should pause to select a user type")
	ts.Require().True(common.HasInput(step.Data.Inputs, "userType"),
		"Two valid user types must be offered for selection")

	step, err = common.CompleteFlow(step.ExecutionID,
		map[string]string{"userType": ts.childTypeName}, "action_usertype", step.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit the user type selection")
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "The flow should move on to user details")

	username := common.GenerateUniqueUsername("onboard_child")
	completed, err := common.CompleteFlow(step.ExecutionID, map[string]string{
		"username": username,
		"email":    username + "@onboarding.test",
	}, "action_details", step.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit user details")
	ts.Require().Equal("COMPLETE", completed.FlowStatus, "Onboarding should provision the user")

	user := ts.findOnboardedUser(username)
	ts.Equal(ts.childOUID, user.OUID, "The user must be provisioned into the selected OU")
	ts.Equal(ts.childTypeName, user.Type, "The selected user type must be the one applied")
}

// Neither user type in this suite marks an attribute unique, so the same username and email carry no
// notion of a duplicate and must be provisionable twice. Regression test for onboarding refusing the
// second create with "user already exists" while POST /users accepted it.
func (ts *UserOnboardingTestSuite) TestOnboarding_SameUserInTwoOUs_WhenNoAttributeIsUnique() {
	username := common.GenerateUniqueUsername("onboard_dup")
	email := username + "@onboarding.test"

	// First user lands in the root OU, whose single user type is auto-selected.
	step := ts.initiateOnboarding(nil)
	step, err := common.CompleteFlow(step.ExecutionID,
		map[string]string{"ouId": ts.rootOUID}, "action_ou", step.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit the OU selection for the first user")
	completed, err := common.CompleteFlow(step.ExecutionID,
		map[string]string{"username": username, "email": email}, "action_details", step.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit user details for the first user")
	ts.Require().Equal("COMPLETE", completed.FlowStatus, "The first user should be provisioned")

	// The same attribute values are then onboarded into the child OU.
	step = ts.initiateOnboarding(nil)
	step, err = common.CompleteFlow(step.ExecutionID,
		map[string]string{"ouId": ts.childOUID}, "action_ou", step.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit the OU selection for the second user")
	step, err = common.CompleteFlow(step.ExecutionID,
		map[string]string{"userType": ts.childTypeName}, "action_usertype", step.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit the user type selection for the second user")

	completed, err = common.CompleteFlow(step.ExecutionID,
		map[string]string{"username": username, "email": email}, "action_details", step.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit user details for the second user")

	// Track whatever was provisioned before asserting, so a failure does not leak the first user.
	// FindUserByAttribute cannot be used here: the username is deliberately held by both users.
	users, lookupErr := testutils.FindUsersByAttribute("username", username)
	ts.Require().NoError(lookupErr, "Failed to look up the onboarded users")
	for _, user := range users {
		if user.ID != "" {
			ts.config.CreatedUserIDs = append(ts.config.CreatedUserIDs, user.ID)
		}
	}

	ts.Require().Nil(completed.Error, "An attribute-identical user must not be refused as a duplicate")
	ts.Require().Equal("COMPLETE", completed.FlowStatus,
		"The second user should be provisioned when no attribute is unique")
	ts.Require().Len(users, 2, "Both users should exist with the same username")

	ouIDs := []string{users[0].OUID, users[1].OUID}
	ts.ElementsMatch([]string{ts.rootOUID, ts.childOUID}, ouIDs,
		"The two users must sit in the two different OUs")
}

// An OU id that does not exist is refused and the selection is asked for again, so a bad id cannot
// carry the flow into provisioning.
func (ts *UserOnboardingTestSuite) TestOnboarding_UnknownOURejected() {
	step := ts.initiateOnboarding(nil)
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "Onboarding should pause to select an OU")

	rejected, err := common.CompleteFlow(step.ExecutionID,
		map[string]string{"ouId": "019ff000-0000-7000-8000-000000000000"}, "action_ou", step.ChallengeToken)
	ts.Require().NoError(err, "An unknown OU should still return a flow step")
	ts.Require().NotNil(rejected.Error, "An unknown OU must be reported as an error")
	ts.Equal(errCodeOUNotFound, rejected.Error.Code, "An unknown OU must be refused as not found")
	ts.True(common.HasInput(rejected.Data.Inputs, "ouId"),
		"The OU selection must be requested again after a bad id")
}

// A user type outside the node's allowed list is refused even when it exists, because the node
// property is the narrower of the two constraints.
func (ts *UserOnboardingTestSuite) TestOnboarding_UserTypeOutsideAllowedListRejected() {
	step := ts.initiateOnboarding(nil)
	step, err := common.CompleteFlow(step.ExecutionID,
		map[string]string{"ouId": ts.childOUID}, "action_ou", step.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit the OU selection")
	ts.Require().True(common.HasInput(step.Data.Inputs, "userType"),
		"Two valid user types must be offered for selection")

	rejected, err := common.CompleteFlow(step.ExecutionID,
		map[string]string{"userType": "not-an-allowed-type"}, "action_usertype", step.ChallengeToken)
	ts.Require().NoError(err, "A disallowed user type should still return a flow step")
	ts.Require().NotNil(rejected.Error, "A disallowed user type must be reported as an error")
	ts.Equal(errCodeUserTypeNotAllowed, rejected.Error.Code,
		"A user type outside the node's allowed list must be refused")
}

// A user type whose OU is not an ancestor of the selected OU is refused. Supplying both values at
// initiation is what makes this reachable: the interactive path never offers the pairing, because
// the type is filtered out of the prompt before the user can pick it.
func (ts *UserOnboardingTestSuite) TestOnboarding_UserTypeNotValidForSelectedOURejected() {
	step := ts.initiateOnboarding(map[string]string{
		"ouId":     ts.rootOUID,
		"userType": ts.childTypeName,
	})
	ts.Require().NotNil(step.Error, "A user type invalid for the selected OU must be reported")
	ts.Equal(errCodeUserTypeNotValidForOU, step.Error.Code,
		"A user type attached below the selected OU must be refused for it")
}
