// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// The shipped agent type declares no unique attribute, so a conflict is unreachable against it.
// This suite installs one for the duration, and restores the type afterwards: the type is shared
// with every other run, and a schema left behind would change what they collect.
type AgentOnboardingUniquenessTestSuite struct {
	suite.Suite
	flowID       string
	snapshot     *testutils.AgentTypeSnapshot
	createdAgent string
}

func TestAgentOnboardingUniquenessTestSuite(t *testing.T) {
	suite.Run(t, new(AgentOnboardingUniquenessTestSuite))
}

func (ts *AgentOnboardingUniquenessTestSuite) SetupSuite() {
	flowID, err := testutils.GetFlowIDByHandle(agentOnboardingFlowHandle, administrationFlowType)
	ts.Require().NoError(err, "Failed to resolve the shipped agent onboarding flow")
	ts.flowID = flowID

	snapshot, err := testutils.SnapshotAgentType()
	ts.Require().NoError(err, "Failed to snapshot the agent type")
	ts.snapshot = snapshot

	_, err = testutils.CreateAgentType(testutils.UserType{
		Name: "default",
		OUID: snapshot.OUID,
		Schema: map[string]interface{}{
			"model": map[string]interface{}{
				"type":     "string",
				"required": true,
				"unique":   true,
			},
		},
	})
	ts.Require().NoError(err, "Failed to install an agent type with a unique attribute")
}

func (ts *AgentOnboardingUniquenessTestSuite) TearDownSuite() {
	if ts.createdAgent != "" {
		if err := testutils.DeleteAgent(ts.createdAgent); err != nil {
			ts.T().Logf("Failed to delete the provisioned agent during teardown: %v", err)
		}
	}
	if ts.snapshot != nil {
		if err := testutils.RestoreAgentType(ts.snapshot); err != nil {
			ts.T().Errorf("teardown: failed to restore the default agent type: %v", err)
		}
	}
}

func (ts *AgentOnboardingUniquenessTestSuite) step(
	executionID string, challengeToken string, action string, inputs map[string]string,
) (int, common.FlowStep, []byte) {
	ts.T().Helper()

	return agentOnboardingStep(&ts.Suite, ts.flowID, executionID, challengeToken, action, inputs)
}

// runTo drives a run as far as the detail step and submits the given model value.
func (ts *AgentOnboardingUniquenessTestSuite) runWithModel(model string) common.FlowStep {
	ts.T().Helper()

	_, step, _ := ts.step("", "", "", nil)
	executionID := step.ExecutionID
	_, step, _ = ts.step(executionID, step.ChallengeToken, agentOwnerAction, map[string]string{})
	_, step, _ = ts.step(executionID, step.ChallengeToken, agentNameAction,
		map[string]string{agentNameInput: common.GenerateUniqueUsername("integration_agent_unique")})
	_, step, _ = ts.step(executionID, step.ChallengeToken, agentDetailsAction,
		map[string]string{agentModelInput: model})

	return step
}

// A value already held by another agent is corrected where it was collected. Reporting it against
// the name step instead would send the caller to a screen that cannot change it.
func (ts *AgentOnboardingUniquenessTestSuite) TestDuplicateUniqueAttributeReturnsToTheDetailPrompt() {
	model := common.GenerateUniqueUsername("model")

	step := ts.runWithModel(model)
	ts.Require().Equal("COMPLETE", step.FlowStatus, "the first run takes the value")
	ts.createdAgent = step.Data.AdditionalData[agentIDData]

	step = ts.runWithModel(model)

	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "the run continues so the value can be corrected")
	ts.Contains(inputIdentifiers(step), agentModelInput,
		"the attribute is asked again, on the step that collects it")
	ts.NotContains(inputIdentifiers(step), agentNameInput,
		"the name step cannot correct an attribute conflict")
	ts.Require().NotNil(step.Error)
	ts.Equal("FET-1061", step.Error.Code, "the conflict names the attribute rather than the entity")
}
