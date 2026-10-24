// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const promptDefaultsFlowHandle = "prompt_defaults_admin_flow_test"

type PromptInputDefaultsTestSuite struct {
	suite.Suite
	flowID string
}

func TestPromptInputDefaultsTestSuite(t *testing.T) {
	suite.Run(t, new(PromptInputDefaultsTestSuite))
}

// promptDefaultsFlowNodes builds two prompts. The first asks for the department, which has a default.
// The second repeats the department and adds the notes, so it shows how a later prompt treats a default
// that was already collected.
func promptDefaultsFlowNodes() []map[string]interface{} {
	department := map[string]interface{}{
		"ref":        "input_department",
		"identifier": "department",
		"type":       "TEXT_INPUT",
		"required":   true,
		"default":    "Computer Science",
	}
	notes := map[string]interface{}{
		"ref":        "input_notes",
		"identifier": "notes",
		"type":       "TEXT_INPUT",
		"required":   true,
	}

	return []map[string]interface{}{
		{"id": "start", "type": "START", "onSuccess": "prompt_department"},
		{
			"id":   "prompt_department",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{{
				"inputs": []map[string]interface{}{department},
				"action": map[string]interface{}{"ref": "action_department", "nextNode": "prompt_notes"},
			}},
		},
		{
			"id":   "prompt_notes",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{{
				"inputs": []map[string]interface{}{department, notes},
				"action": map[string]interface{}{"ref": "action_notes", "nextNode": "end"},
			}},
		},
		{"id": "end", "type": "END"},
	}
}

func (ts *PromptInputDefaultsTestSuite) SetupSuite() {
	flowID, err := testutils.CreateFlow(testutils.Flow{
		Name:     "Prompt Input Defaults Test Flow",
		FlowType: "ADMINISTRATION",
		Handle:   promptDefaultsFlowHandle,
		Nodes:    promptDefaultsFlowNodes(),
	})
	ts.Require().NoError(err, "Failed to create the prompt defaults flow")
	ts.flowID = flowID
}

func (ts *PromptInputDefaultsTestSuite) TearDownSuite() {
	if ts.flowID == "" {
		return
	}
	if err := testutils.DeleteFlow(ts.flowID); err != nil {
		ts.T().Logf("Failed to delete the prompt defaults flow during teardown: %v", err)
	}
}

// inputOf returns the prompted input with the given identifier, or nil when the step does not ask for it.
func inputOf(step *common.FlowStep, identifier string) *common.Inputs {
	for i := range step.Data.Inputs {
		if step.Data.Inputs[i].Identifier == identifier {
			return &step.Data.Inputs[i]
		}
	}
	return nil
}

// step posts one interaction to the flow as the admin, since administration flows need an authenticated
// caller at every step. An empty execution id starts the flow.
func (ts *PromptInputDefaultsTestSuite) step(
	step *common.FlowStep, action string, inputs map[string]string,
) *common.FlowStep {
	ts.T().Helper()

	executionID, challengeToken := "", ""
	if step != nil {
		executionID, challengeToken = step.ExecutionID, step.ChallengeToken
	}
	status, next, body := agentOnboardingStep(&ts.Suite, ts.flowID, executionID, challengeToken, action, inputs)
	ts.Require().Equal(200, status, "Flow response: %s", string(body))
	return &next
}

func (ts *PromptInputDefaultsTestSuite) start() *common.FlowStep {
	ts.T().Helper()

	first := ts.step(nil, "", nil)
	ts.Require().Equal("INCOMPLETE", first.FlowStatus, "The first prompt must be shown")
	return first
}

// The first prompt shows the department prefilled with its default.
func (ts *PromptInputDefaultsTestSuite) TestPromptShowsTheDefaultAsTheInputValue() {
	step := ts.start()

	department := inputOf(step, "department")
	ts.Require().NotNil(department, "The prompt must ask for the department")
	ts.Equal("Computer Science", department.Value, "The default must be returned as the input value")
}

// The later prompt repeats the department. It is not asked for again, so only the input without a default
// remains.
func (ts *PromptInputDefaultsTestSuite) TestLaterPromptSkipsTheDefault() {
	step := ts.start()

	next := ts.step(step, "action_department", map[string]string{"department": "Mathematics"})
	ts.Require().Equal("INCOMPLETE", next.FlowStatus, "The second prompt must be shown for the notes")

	ts.Nil(inputOf(next, "department"), "A department that was already collected must not be asked for again")
	notes := inputOf(next, "notes")
	ts.Require().NotNil(notes, "The notes have no value yet and must be asked for")
	ts.Empty(notes.Value, "An input without a default carries no value")

	done := ts.step(next, "action_notes", map[string]string{"notes": "welcome"})
	ts.Equal("COMPLETE", done.FlowStatus, "The flow must complete after the notes are submitted")
}

// A form submitted without touching the department still carries the default, because the prompt
// stored it when it was shown.
func (ts *PromptInputDefaultsTestSuite) TestUntouchedDefaultIsKept() {
	step := ts.start()

	next := ts.step(step, "action_department", nil)

	ts.Equal("INCOMPLETE", next.FlowStatus, "The flow must move on to the second prompt")
	ts.Nil(inputOf(next, "department"), "The stored default must satisfy the department on the second prompt")
	ts.NotNil(inputOf(next, "notes"), "The second prompt must still ask for the notes")
}
