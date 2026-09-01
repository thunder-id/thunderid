// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package registration

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	errCodeOUResolutionFailed    = "FET-1034"
	errCodeOUNotValidForUserType = "FET-1075"
)

// ouStrategyFlowNodes builds a registration flow that resolves the user type, then the OU with the
// given strategy, then provisions the user. The strategy is the only thing that varies between the
// flows this suite creates.
func ouStrategyFlowNodes(resolveFrom string) []map[string]interface{} {
	return []map[string]interface{}{
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
			"onSuccess": "ou_resolver",
		},
		{
			"id":   "ou_resolver",
			"type": "TASK_EXECUTION",
			"executor": map[string]interface{}{
				"name": "OUResolverExecutor",
			},
			"properties": map[string]interface{}{
				"resolveFrom": resolveFrom,
			},
			"onSuccess":    "provisioning",
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
			"onIncomplete": "prompt_details",
		},
		{
			"id":   "prompt_details",
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

func ouStrategyUserType(name, ouID string) testutils.UserType {
	return testutils.UserType{
		Name:                  name,
		OUID:                  ouID,
		AllowSelfRegistration: true,
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

type OUResolverStrategiesTestSuite struct {
	suite.Suite
	config *common.TestSuiteConfig

	parentOUID string
	childOUID  string
	otherOUID  string

	parentTypeID   string
	childTypeID    string
	parentTypeName string
	childTypeName  string

	promptParentAppID string
	promptChildAppID  string
	callerAppID       string
	unsupportedAppID  string

	// An auth flow that references no registration flow, so binding these registration flows to an
	// application does not collide with the default auth flow's own registration reference.
	isolatedAuthFlowID string
}

func TestOUResolverStrategiesTestSuite(t *testing.T) {
	suite.Run(t, new(OUResolverStrategiesTestSuite))
}

func (ts *OUResolverStrategiesTestSuite) SetupSuite() {
	ts.config = &common.TestSuiteConfig{}

	parentOUID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "ou_strategy_parent_test_ou",
		Name:        "OU Strategy Parent Test OU",
		Description: "Parent organization unit for OU resolver strategy testing",
		Parent:      nil,
	})
	ts.Require().NoError(err, "Failed to create parent organization unit")
	ts.parentOUID = parentOUID

	childOUID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "ou_strategy_child_test_ou",
		Name:        "OU Strategy Child Test OU",
		Description: "Child organization unit for OU resolver strategy testing",
		Parent:      &parentOUID,
	})
	ts.Require().NoError(err, "Failed to create child organization unit")
	ts.childOUID = childOUID

	// A second root, outside the parent's subtree, so a selection can be rejected for being
	// somewhere the user type does not reach.
	otherOUID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "ou_strategy_other_test_ou",
		Name:        "OU Strategy Other Test OU",
		Description: "Unrelated organization unit for OU resolver strategy testing",
		Parent:      nil,
	})
	ts.Require().NoError(err, "Failed to create unrelated organization unit")
	ts.otherOUID = otherOUID

	ts.parentTypeName = "ou-strategy-parent-person"
	ts.childTypeName = "ou-strategy-child-person"

	ts.parentTypeID, err = testutils.CreateUserType(ouStrategyUserType(ts.parentTypeName, parentOUID))
	ts.Require().NoError(err, "Failed to create parent user type")

	ts.childTypeID, err = testutils.CreateUserType(ouStrategyUserType(ts.childTypeName, childOUID))
	ts.Require().NoError(err, "Failed to create child user type")

	ts.isolatedAuthFlowID, err = testutils.CreateIsolatedAuthFlow("ou-strategy-isolated-auth")
	ts.Require().NoError(err, "Failed to create isolated auth flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, ts.isolatedAuthFlowID)

	promptFlowID := ts.createFlow("OU Resolver Prompt Flow", "registration_flow_ou_prompt_test", "prompt")
	callerFlowID := ts.createFlow("OU Resolver Caller Flow", "registration_flow_ou_caller_test", "caller")
	unsupportedFlowID := ts.createFlow("OU Resolver Unsupported Flow",
		"registration_flow_ou_unsupported_test", "notAStrategy")

	// The prompt flow is bound twice: once with a user type whose OU has children, and once with a
	// user type whose OU has none, which is what decides whether a selection is asked for.
	ts.promptParentAppID = ts.createApp("OU Strategy Prompt Parent App", "ou_strategy_prompt_parent_client",
		promptFlowID, ts.parentTypeName)
	ts.promptChildAppID = ts.createApp("OU Strategy Prompt Child App", "ou_strategy_prompt_child_client",
		promptFlowID, ts.childTypeName)
	ts.callerAppID = ts.createApp("OU Strategy Caller App", "ou_strategy_caller_client",
		callerFlowID, ts.parentTypeName)
	ts.unsupportedAppID = ts.createApp("OU Strategy Unsupported App", "ou_strategy_unsupported_client",
		unsupportedFlowID, ts.parentTypeName)
}

func (ts *OUResolverStrategiesTestSuite) createFlow(name, handle, resolveFrom string) string {
	ts.T().Helper()

	flowID, err := testutils.CreateFlow(testutils.Flow{
		Name:     name,
		FlowType: "REGISTRATION",
		Handle:   handle,
		Nodes:    ouStrategyFlowNodes(resolveFrom),
	})
	ts.Require().NoError(err, "Failed to create flow %s", handle)
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)
	return flowID
}

func (ts *OUResolverStrategiesTestSuite) createApp(name, clientID, flowID, userType string) string {
	ts.T().Helper()

	appID, err := testutils.CreateApplication(testutils.Application{
		Name:                      name,
		Description:               "Application for OU resolver strategy testing",
		ClientID:                  clientID,
		ClientSecret:              clientID + "_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		OUID:                      ts.parentOUID,
		AllowedUserTypes:          []string{userType},
		IsRegistrationFlowEnabled: true,
		RegistrationFlowID:        flowID,
		AuthFlowID:                ts.isolatedAuthFlowID,
	})
	ts.Require().NoError(err, "Failed to create application %s", name)
	return appID
}

func (ts *OUResolverStrategiesTestSuite) TearDownSuite() {
	for _, appID := range []string{
		ts.promptParentAppID, ts.promptChildAppID, ts.callerAppID, ts.unsupportedAppID,
	} {
		if appID == "" {
			continue
		}
		if err := testutils.DeleteApplication(appID); err != nil {
			ts.T().Logf("Failed to delete test application during teardown: %v", err)
		}
	}
	for _, userID := range ts.config.CreatedUserIDs {
		if err := testutils.DeleteUser(userID); err != nil {
			ts.T().Logf("Failed to delete registered user during teardown: %v", err)
		}
	}
	for _, flowID := range ts.config.CreatedFlowIDs {
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("Failed to delete test flow during teardown: %v", err)
		}
	}
	for _, typeID := range []string{ts.childTypeID, ts.parentTypeID} {
		if typeID == "" {
			continue
		}
		if err := testutils.DeleteUserType(typeID); err != nil {
			ts.T().Logf("Failed to delete test user type during teardown: %v", err)
		}
	}
	for _, ouID := range []string{ts.childOUID, ts.parentOUID, ts.otherOUID} {
		if ouID == "" {
			continue
		}
		if err := testutils.DeleteOrganizationUnit(ouID); err != nil {
			ts.T().Logf("Failed to delete test organization unit during teardown: %v", err)
		}
	}
}

// trackRegisteredUser records a user created by a registration flow so teardown removes it.
func (ts *OUResolverStrategiesTestSuite) trackRegisteredUser(username string) *testutils.User {
	ts.T().Helper()

	user, err := testutils.FindUserByAttribute("username", username)
	ts.Require().NoError(err, "Failed to look up the registered user")
	ts.Require().NotNil(user, "The registration flow should have created a user")
	if user.ID != "" {
		ts.config.CreatedUserIDs = append(ts.config.CreatedUserIDs, user.ID)
	}
	return user
}

// The prompt strategy asks for an OU only when the user type's OU has children, and the selection is
// accepted when it sits inside that subtree. The user is then provisioned into the chosen OU rather
// than the user type's own OU.
func (ts *OUResolverStrategiesTestSuite) TestPrompt_DescendantOUSelected() {
	step, err := common.InitiateRegistrationFlow(ts.promptParentAppID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate registration flow")
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "The flow should pause for the OU selection")
	ts.Require().True(common.HasInput(step.Data.Inputs, "ouId"),
		"A user type whose OU has children must prompt for a selection")

	step, err = common.CompleteFlow(step.ExecutionID,
		map[string]string{"ouId": ts.childOUID}, "action_ou", step.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit the OU selection")
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "The flow should move on to user details")
	ts.Require().True(common.HasInput(step.Data.Inputs, "username"),
		"The flow should collect user details after the OU is chosen")

	username := common.GenerateUniqueUsername("ou_prompt")
	completed, err := common.CompleteFlow(step.ExecutionID, map[string]string{
		"username": username,
		"email":    username + "@ou-strategy.test",
	}, "action_details", step.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit user details")
	ts.Require().Equal("COMPLETE", completed.FlowStatus, "Registration should provision the user")

	user := ts.trackRegisteredUser(username)
	ts.Equal(ts.childOUID, user.OUID, "The user must be provisioned into the selected OU")
}

// An OU outside the user type's subtree is refused and the selection is asked for again, so the
// prompt cannot be used to place a user anywhere in the tree.
func (ts *OUResolverStrategiesTestSuite) TestPrompt_OUOutsideSubtreeRejected() {
	step, err := common.InitiateRegistrationFlow(ts.promptParentAppID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate registration flow")
	ts.Require().True(common.HasInput(step.Data.Inputs, "ouId"), "The flow should prompt for an OU")

	rejected, err := common.CompleteFlow(step.ExecutionID,
		map[string]string{"ouId": ts.otherOUID}, "action_ou", step.ChallengeToken)
	ts.Require().NoError(err, "An out-of-subtree OU should still return a flow step")
	ts.Require().NotNil(rejected.Error, "An out-of-subtree OU must be reported as an error")
	ts.Equal(errCodeOUNotValidForUserType, rejected.Error.Code,
		"An OU outside the user type's subtree must be refused for that user type")
	ts.True(common.HasInput(rejected.Data.Inputs, "ouId"),
		"The OU selection must be requested again after a rejected choice")
}

// When the user type's OU has no children there is nothing to choose, so the strategy resolves
// silently and the flow goes straight to collecting user details.
func (ts *OUResolverStrategiesTestSuite) TestPrompt_NoChildOUsSkipsSelection() {
	step, err := common.InitiateRegistrationFlow(ts.promptChildAppID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate registration flow")
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "The flow should pause for user details")
	ts.False(common.HasInput(step.Data.Inputs, "ouId"),
		"An OU with no children must not be prompted for")
	ts.True(common.HasInput(step.Data.Inputs, "username"),
		"The flow should move straight on to user details")
}

// The caller strategy takes the OU from the caller's security context, so it cannot work on a
// self-service registration flow where there is no authenticated caller. It fails rather than
// falling back to a default OU.
func (ts *OUResolverStrategiesTestSuite) TestCaller_WithoutCallerContextFails() {
	step, err := common.InitiateRegistrationFlow(ts.callerAppID, false, nil, "")
	ts.Require().NoError(err, "The flow should return a step rather than a transport error")
	ts.Require().NotNil(step.Error, "An unresolvable caller OU must be reported")
	ts.Equal(errCodeOUResolutionFailed, step.Error.Code,
		"A missing caller OU must fail OU resolution")
}

// An unrecognized strategy fails the node instead of being ignored, so a typo in a flow definition
// surfaces at execution rather than silently placing users in a default OU.
func (ts *OUResolverStrategiesTestSuite) TestUnsupportedStrategy_Fails() {
	step, err := common.InitiateRegistrationFlow(ts.unsupportedAppID, false, nil, "")
	ts.Require().NoError(err, "The flow should return a step rather than a transport error")
	ts.Require().NotNil(step.Error, "An unsupported strategy must be reported")
	ts.Equal(errCodeOUResolutionFailed, step.Error.Code,
		"An unsupported strategy must fail OU resolution")
}

// The prompt strategy resolves a submitted handle scoped to the parent OU whose children were
// offered, not just a raw ID — the human-readable identifier path #5122 added.
func (ts *OUResolverStrategiesTestSuite) TestPrompt_HandleSubmissionResolved() {
	step, err := common.InitiateRegistrationFlow(ts.promptParentAppID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate registration flow")
	ts.Require().True(common.HasInput(step.Data.Inputs, "ouId"), "The flow should prompt for an OU")

	step, err = common.CompleteFlow(step.ExecutionID,
		map[string]string{"ouHandle": "ou_strategy_child_test_ou"}, "action_ou", step.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit the OU handle")
	ts.Require().Nil(step.Error, "A valid handle among the parent's children must resolve")
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "The flow should move on to user details")

	username := common.GenerateUniqueUsername("ou_prompt_handle")
	completed, err := common.CompleteFlow(step.ExecutionID, map[string]string{
		"username": username,
		"email":    username + "@ou-strategy.test",
	}, "action_details", step.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit user details")
	ts.Require().Equal("COMPLETE", completed.FlowStatus, "Registration should provision the user")

	user := ts.trackRegisteredUser(username)
	ts.Equal(ts.childOUID, user.OUID, "A handle submission must resolve to the same OU as the raw ID would")
}
