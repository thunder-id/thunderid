// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// The shipped agent type declares name as a required, unique attribute, so a duplicate is
// reachable against it with no schema changes.
type AgentOnboardingUniquenessTestSuite struct {
	suite.Suite
	flowID       string
	createdAgent string
}

func TestAgentOnboardingUniquenessTestSuite(t *testing.T) {
	suite.Run(t, new(AgentOnboardingUniquenessTestSuite))
}

func (ts *AgentOnboardingUniquenessTestSuite) SetupSuite() {
	flowID, err := testutils.GetFlowIDByHandle(agentOnboardingFlowHandle, administrationFlowType)
	ts.Require().NoError(err, "Failed to resolve the shipped agent onboarding flow")
	ts.flowID = flowID
}

func (ts *AgentOnboardingUniquenessTestSuite) TearDownSuite() {
	if ts.createdAgent != "" {
		if err := testutils.DeleteAgent(ts.createdAgent); err != nil {
			ts.T().Logf("Failed to delete the provisioned agent during teardown: %v", err)
		}
	}
}

func (ts *AgentOnboardingUniquenessTestSuite) step(
	executionID string, challengeToken string, action string, inputs map[string]string,
) (int, common.FlowStep, []byte) {
	ts.T().Helper()

	return agentOnboardingStep(&ts.Suite, ts.flowID, executionID, challengeToken, action, inputs)
}

// runWithName drives a run through the owner prompt and submits the given name on the details prompt.
func (ts *AgentOnboardingUniquenessTestSuite) runWithName(name string) common.FlowStep {
	ts.T().Helper()

	_, step, _ := ts.step("", "", "", nil)
	executionID := step.ExecutionID
	_, step, _ = ts.step(executionID, step.ChallengeToken, agentOwnerAction, map[string]string{})
	_, step, _ = ts.step(executionID, step.ChallengeToken, agentDetailsAction,
		map[string]string{agentNameInput: name})

	return step
}

// A name already held by another agent is corrected where it was collected: the run returns to the
// details prompt, which asks for the name again, and the error identifies the name field.
func (ts *AgentOnboardingUniquenessTestSuite) TestDuplicateNameReturnsToTheDetailsPrompt() {
	name := common.GenerateUniqueUsername("integration_agent_unique")

	step := ts.runWithName(name)
	ts.Require().Equal("COMPLETE", step.FlowStatus, "the first run takes the name")
	ts.createdAgent = step.Data.AdditionalData[agentIDData]

	step = ts.runWithName(name)

	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "the run continues so the name can be corrected")
	ts.Contains(inputIdentifiers(step), agentNameInput,
		"the name is asked again, on the details prompt that collects it")
	ts.Contains(inputIdentifiers(step), agentModelInput, "the run is back on the details prompt")
	for _, input := range step.Data.Inputs {
		if input.Identifier == agentModelProviderInput {
			ts.NotEmpty(input.Options, "the re-shown details prompt still offers the model provider choices")
		}
	}
	ts.Require().NotNil(step.Error)
	ts.Equal("FET-1061", step.Error.Code, "the conflict names the attribute rather than the entity")
}
