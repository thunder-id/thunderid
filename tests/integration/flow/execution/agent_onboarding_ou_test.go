// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	agentOnboardingChildOUHandle   = "agent-onboarding-child-ou"
	agentOnboardingOutsideOUHandle = "agent-onboarding-outside-ou"
	agentOUSelectionInput          = "ouId"
	agentOUSelectionAction         = "action_ou_selection"
	// An identifier of the right shape that names nothing, for the case where a caller submits
	// an organization unit that does not exist.
	agentOnboardingUnknownOUID = "01900000-0000-7000-8000-0000000009ff"
)

// The organization unit step only appears when there is a choice to make, so it is invisible in a
// deployment with a flat tree. This suite creates a child of the agent type's organization unit so
// the step is reached, and asserts the agent lands where it was placed rather than alongside the
// type whose schema describes it.
//
// The child is torn down: leaving it behind would make the step appear for every later agent
// onboarding run, which expects no choice.
type AgentOnboardingOUTestSuite struct {
	suite.Suite
	flowID       string
	parentOUID   string
	childOUID    string
	outsideOUID  string
	createdAgent string
}

func TestAgentOnboardingOUTestSuite(t *testing.T) {
	suite.Run(t, new(AgentOnboardingOUTestSuite))
}

func (ts *AgentOnboardingOUTestSuite) SetupSuite() {
	flowID, err := testutils.GetFlowIDByHandle(agentOnboardingFlowHandle, administrationFlowType)
	ts.Require().NoError(err, "Failed to resolve the shipped agent onboarding flow")
	ts.flowID = flowID

	// The organization unit resolver roots its tree at the agent type's own unit, so the child has
	// to hang off that one to be offered.
	snapshot, err := testutils.SnapshotAgentType()
	ts.Require().NoError(err, "Failed to read the agent type")
	ts.Require().NotEmpty(snapshot.OUID, "The agent type must carry an organization unit")
	ts.parentOUID = snapshot.OUID

	childOUID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      agentOnboardingChildOUHandle,
		Name:        "Agent Onboarding Child OU",
		Description: "Child of the agent type's organization unit, so the OU step has a choice",
		Parent:      &ts.parentOUID,
	})
	ts.Require().NoError(err, "Failed to create the child organization unit")
	ts.childOUID = childOUID

	// A second root, so it is a real organization unit that the agent type's subtree does not
	// reach. Without one the only rejection that can be exercised is an identifier naming nothing.
	outsideOUID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      agentOnboardingOutsideOUHandle,
		Name:        "Agent Onboarding Outside OU",
		Description: "Root level, so it sits outside the agent type's subtree",
	})
	ts.Require().NoError(err, "Failed to create the outside organization unit")
	ts.outsideOUID = outsideOUID
}

func (ts *AgentOnboardingOUTestSuite) TearDownSuite() {
	if ts.createdAgent != "" {
		if err := testutils.DeleteAgent(ts.createdAgent); err != nil {
			ts.T().Logf("Failed to delete the provisioned agent during teardown: %v", err)
		}
	}
	for _, ouID := range []string{ts.childOUID, ts.outsideOUID} {
		if ouID == "" {
			continue
		}
		if err := testutils.DeleteOrganizationUnit(ouID); err != nil {
			ts.T().Errorf("teardown: failed to delete organization unit %s: %v", ouID, err)
		}
	}
}

// step posts one interaction, reusing the helper shape the sibling suite uses.
func (ts *AgentOnboardingOUTestSuite) step(
	executionID string, challengeToken string, action string, inputs map[string]string,
) (int, common.FlowStep, []byte) {
	ts.T().Helper()

	return agentOnboardingStep(&ts.Suite, ts.flowID, executionID, challengeToken, action, inputs)
}

// A child organization unit gives the step something to ask, and the answer decides where the
// agent is created. Without it the agent would follow its type's unit.
func (ts *AgentOnboardingOUTestSuite) TestAgentIsProvisionedIntoTheSelectedOU() {
	status, step, body := ts.step("", "", "", nil)
	ts.Require().Equal(http.StatusOK, status, "Flow initiation failed: %s", string(body))
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "The flow should pause for the OU: %s", string(body))
	executionID := step.ExecutionID

	ts.Require().Contains(inputIdentifiers(step), agentOUSelectionInput,
		"a child organization unit makes the step a real choice: %s", string(body))

	status, step, body = ts.step(executionID, step.ChallengeToken, agentOUSelectionAction,
		map[string]string{agentOUSelectionInput: ts.childOUID})
	ts.Require().Equal(http.StatusOK, status, "OU step failed: %s", string(body))
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "The flow should continue past the OU: %s", string(body))

	status, step, body = ts.step(executionID, step.ChallengeToken, agentOwnerAction, map[string]string{})
	ts.Require().Equal(http.StatusOK, status, "Owner step failed: %s", string(body))

	agentName := common.GenerateUniqueUsername("integration_agent_ou")
	status, step, body = ts.step(executionID, step.ChallengeToken, agentNameAction,
		map[string]string{agentNameInput: agentName})
	ts.Require().Equal(http.StatusOK, status, "Name step failed: %s", string(body))

	status, step, body = ts.step(executionID, step.ChallengeToken, agentDetailsAction, map[string]string{
		agentModelProviderInput: "anthropic",
		agentModelInput:         common.GenerateUniqueUsername("model"),
	})
	ts.Require().Equal(http.StatusOK, status, "Details step failed: %s", string(body))
	ts.Require().Equal("COMPLETE", step.FlowStatus, "The flow should complete: %s", string(body))

	ts.createdAgent = step.Data.AdditionalData[agentIDData]
	ts.Require().NotEmpty(ts.createdAgent, "the agent id must reach the caller")

	agent, err := testutils.GetAgent(ts.createdAgent)
	ts.Require().NoError(err, "Failed to read the provisioned agent")
	ts.Equal(ts.childOUID, agent.OUID,
		"the agent belongs to the chosen organization unit, not the one its type lives in")
	ts.NotEqual(ts.parentOUID, agent.OUID)
}

// The step confines the choice to the agent type's subtree, so an organization unit the type does
// not reach must not place an agent. This is the case a caller reaches by submitting an
// identifier the picker never offered.
func (ts *AgentOnboardingOUTestSuite) TestOUOutsideTheTypeSubtreeIsRejected() {
	_, step, _ := ts.step("", "", "", nil)
	executionID := step.ExecutionID

	_, step, body := ts.step(executionID, step.ChallengeToken, agentOUSelectionAction,
		map[string]string{agentOUSelectionInput: ts.outsideOUID})

	// Still on the same step rather than advanced: a rejection and an acceptance both leave the
	// flow incomplete, so the step it is sitting on is what tells them apart.
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "%s", string(body))
	ts.Contains(inputIdentifiers(step), agentOUSelectionInput,
		"the step is asked again rather than advancing: %s", string(body))
	ts.Empty(step.Data.AdditionalData[agentIDData], "no agent is created")
}

// An identifier that names no organization unit is the other way the step can be given something
// it cannot place an agent in.
func (ts *AgentOnboardingOUTestSuite) TestUnknownOUIsRejected() {
	_, step, _ := ts.step("", "", "", nil)
	executionID := step.ExecutionID

	_, step, body := ts.step(executionID, step.ChallengeToken, agentOUSelectionAction,
		map[string]string{agentOUSelectionInput: agentOnboardingUnknownOUID})

	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "%s", string(body))
	ts.Contains(inputIdentifiers(step), agentOUSelectionInput,
		"the step is asked again rather than advancing: %s", string(body))
	ts.Empty(step.Data.AdditionalData[agentIDData], "no agent is created")
}
