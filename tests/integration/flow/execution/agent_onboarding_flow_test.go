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

// The shipped agent onboarding flow. Executing it by handle keeps the test independent of the
// seeded flow id, which is a bootstrap detail.
const agentOnboardingFlowHandle = "default-agent-onboarding-flow"

// The inputs the shipped flow collects. Name is a column on the agent record; the rest are
// attributes the default agent type declares.
const (
	agentNameInput          = "name"
	agentModelProviderInput = "modelProvider"
	agentModelInput         = "model"
)

// The keys the provisioning node publishes the generated credentials under.
const (
	agentIDData           = "agentId"
	agentClientIDData     = "clientId"
	agentClientSecretData = "clientSecret"
)

// The submit actions of the shipped flow's prompts. A prompt node advances only once an action is
// selected, so a step that supplies inputs alone re-renders the same screen.
const (
	agentOwnerAction   = "action_agent_owner"
	agentNameAction    = "action_agent_name"
	agentDetailsAction = "action_agent_details"
)

// Every agent a test provisions needs a field of its own: the suite deletes them all in teardown,
// and a shared field would leave the agents it was overwritten with behind on the server.
type AgentOnboardingFlowTestSuite struct {
	suite.Suite
	flowID         string
	createdAgent   string
	bareAgent      string
	takenNameAgent string
	secondAgent    string
}

func TestAgentOnboardingFlowTestSuite(t *testing.T) {
	suite.Run(t, new(AgentOnboardingFlowTestSuite))
}

func (ts *AgentOnboardingFlowTestSuite) SetupSuite() {
	flowID, err := testutils.GetFlowIDByHandle(agentOnboardingFlowHandle, administrationFlowType)
	ts.Require().NoError(err, "Failed to resolve the shipped agent onboarding flow")
	ts.Require().NotEmpty(flowID, "The shipped agent onboarding flow must be present")
	ts.flowID = flowID
}

func (ts *AgentOnboardingFlowTestSuite) TearDownSuite() {
	for _, agentID := range []string{ts.createdAgent, ts.bareAgent, ts.takenNameAgent, ts.secondAgent} {
		if agentID == "" {
			continue
		}
		if err := testutils.DeleteAgent(agentID); err != nil {
			ts.T().Logf("Failed to delete the provisioned agent during teardown: %v", err)
		}
	}
}

// agentOnboardingStep posts one interaction to the flow. The first call omits the execution id to
// start the flow; later calls carry it forward.
//
// The bearer token is set on a raw client because the shared test clients treat /flow/execute as a
// public endpoint and skip token injection, which would make the run anonymous and fail the
// flow's permission validator.
func agentOnboardingStep(s *suite.Suite, flowID, executionID, challengeToken, action string,
	inputs map[string]string,
) (int, common.FlowStep, []byte) {
	s.T().Helper()

	token, err := testutils.GetAccessToken()
	s.Require().NoError(err, "Failed to obtain admin access token")

	payload := map[string]interface{}{"flowId": flowID}
	if executionID != "" {
		// The engine issues a challenge token per step and rejects the next request without it.
		payload = map[string]interface{}{"executionId": executionID, "challengeToken": challengeToken}
	}
	if len(inputs) > 0 {
		payload["inputs"] = inputs
	}
	if action != "" {
		payload["action"] = action
	}

	reqBody, err := json.Marshal(payload)
	s.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPost, testServerURL+"/flow/execute", bytes.NewReader(reqBody))
	s.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := testutils.GetRawHTTPClient().Do(req)
	s.Require().NoError(err, "Failed to execute the agent onboarding flow")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)

	var step common.FlowStep
	_ = json.Unmarshal(body, &step)

	return resp.StatusCode, step, body
}

func (ts *AgentOnboardingFlowTestSuite) step(
	executionID string, challengeToken string, action string, inputs map[string]string,
) (int, common.FlowStep, []byte) {
	ts.T().Helper()

	return agentOnboardingStep(&ts.Suite, ts.flowID, executionID, challengeToken, action, inputs)
}

// inputIdentifiers lists what a step asked for, so a test can assert the shape of a prompt without
// depending on the order the executor happened to emit.
func inputIdentifiers(step common.FlowStep) []string {
	identifiers := make([]string, 0, len(step.Data.Inputs))
	for _, input := range step.Data.Inputs {
		identifiers = append(identifiers, input.Identifier)
	}
	return identifiers
}

// Running the shipped flow to completion exercises the whole chain in one execution: the
// permission validator, the owner resolver, the prompts, the provisioning node, and the
// credentials the final screen reads.
func (ts *AgentOnboardingFlowTestSuite) TestAgentOnboardingFlow_CompletesAndReturnsCredentials() {
	status, step, body := ts.step("", "", "", nil)
	ts.Require().Equal(http.StatusOK, status, "Flow initiation failed: %s", string(body))
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "The flow should pause for the owner: %s", string(body))
	ts.Require().NotEmpty(step.ExecutionID)
	executionID := step.ExecutionID

	// The owner is optional, so submitting nothing leaves the agent owned by the caller.
	ts.Contains(inputIdentifiers(step), "owner", "the first prompt asks who owns the agent")

	status, step, body = ts.step(executionID, step.ChallengeToken, agentOwnerAction, map[string]string{})
	ts.Require().Equal(http.StatusOK, status, "Owner step failed: %s", string(body))
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "The flow should pause for the name: %s", string(body))
	ts.Contains(inputIdentifiers(step), agentNameInput, "the second prompt asks for a name")

	agentName := common.GenerateUniqueUsername("integration_agent")
	status, step, body = ts.step(executionID, step.ChallengeToken, agentNameAction, map[string]string{agentNameInput: agentName})
	ts.Require().Equal(http.StatusOK, status, "Name step failed: %s", string(body))
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "The flow should pause for the details: %s", string(body))

	// The detail prompt is driven by the agent type schema, so its inputs prove the executor
	// resolved the agent type without a resolver node ahead of it.
	ts.Contains(inputIdentifiers(step), agentModelInput, "the third prompt offers the schema attributes")

	status, step, body = ts.step(executionID, step.ChallengeToken, agentDetailsAction, map[string]string{
		agentModelProviderInput: "anthropic",
		agentModelInput:         common.GenerateUniqueUsername("model"),
	})
	ts.Require().Equal(http.StatusOK, status, "Details step failed: %s", string(body))
	ts.Require().Equal("COMPLETE", step.FlowStatus, "The flow should complete: %s", string(body))

	// The secret is returned on create and never again, so the completing step is the only place a
	// caller can read it.
	ts.Require().NotEmpty(step.Data.AdditionalData[agentIDData], "the agent id must reach the caller")
	ts.Require().NotEmpty(step.Data.AdditionalData[agentClientIDData], "the client id must reach the caller")
	ts.Require().NotEmpty(step.Data.AdditionalData[agentClientSecretData],
		"the client secret must reach the caller")

	ts.createdAgent = step.Data.AdditionalData[agentIDData]

	// The flow collects no logo, so the provider supplies one; a listing would otherwise draw
	// nothing for an agent created this way.
	agent, err := testutils.GetAgent(ts.createdAgent)
	ts.Require().NoError(err, "Failed to read the provisioned agent")
	ts.NotEmpty(agent.LogoURL, "the agent carries a logo without the flow collecting one")
}

// Every attribute the agent type declares is optional, so the detail step can be submitted with
// nothing filled in. There is then no attribute to identify an existing agent by, and an empty
// filter is rejected by the store rather than matching nothing.
func (ts *AgentOnboardingFlowTestSuite) TestAgentOnboardingFlow_CompletesWithNoDetailsSupplied() {
	_, step, _ := ts.step("", "", "", nil)
	executionID := step.ExecutionID

	_, step, _ = ts.step(executionID, step.ChallengeToken, agentOwnerAction, map[string]string{})

	agentName := common.GenerateUniqueUsername("integration_agent_bare")
	_, step, _ = ts.step(executionID, step.ChallengeToken, agentNameAction,
		map[string]string{agentNameInput: agentName})

	_, step, body := ts.step(executionID, step.ChallengeToken, agentDetailsAction, map[string]string{})

	ts.Require().Equal("COMPLETE", step.FlowStatus,
		"an agent with only a name is provisionable: %s", string(body))
	ts.Require().NotEmpty(step.Data.AdditionalData[agentClientSecretData],
		"the credentials still reach the caller")

	ts.bareAgent = step.Data.AdditionalData[agentIDData]
}

// The agent service rejects a duplicate name, and the name was collected two steps before the
// create attempt. Without routing that failure somewhere the caller can act on, the run ends on a
// screen with nothing to change and no way back.
func (ts *AgentOnboardingFlowTestSuite) TestAgentOnboardingFlow_DuplicateNameReturnsToTheNamePrompt() {
	taken := common.GenerateUniqueUsername("integration_agent_taken")

	// The first run takes the name.
	_, step, _ := ts.step("", "", "", nil)
	first := step.ExecutionID
	_, step, _ = ts.step(first, step.ChallengeToken, agentOwnerAction, map[string]string{})
	_, step, _ = ts.step(first, step.ChallengeToken, agentNameAction, map[string]string{agentNameInput: taken})
	_, step, body := ts.step(first, step.ChallengeToken, agentDetailsAction, map[string]string{})
	ts.Require().Equal("COMPLETE", step.FlowStatus, "%s", string(body))
	ts.takenNameAgent = step.Data.AdditionalData[agentIDData]

	// The second run collides on it.
	_, step, _ = ts.step("", "", "", nil)
	second := step.ExecutionID
	_, step, _ = ts.step(second, step.ChallengeToken, agentOwnerAction, map[string]string{})
	_, step, _ = ts.step(second, step.ChallengeToken, agentNameAction, map[string]string{agentNameInput: taken})
	_, step, body = ts.step(second, step.ChallengeToken, agentDetailsAction, map[string]string{})

	ts.Require().Equal("INCOMPLETE", step.FlowStatus,
		"the run continues so the name can be corrected: %s", string(body))
	ts.Contains(inputIdentifiers(step), agentNameInput,
		"the name is asked again rather than the run ending: %s", string(body))

	// Correcting it completes without repeating the earlier steps.
	_, step, body = ts.step(second, step.ChallengeToken, agentNameAction,
		map[string]string{agentNameInput: common.GenerateUniqueUsername("integration_agent_freed")})
	ts.Require().Equal("COMPLETE", step.FlowStatus, "%s", string(body))
	ts.secondAgent = step.Data.AdditionalData[agentIDData]
}

// With a single agent type available the resolver settles on it without asking, so the flow reaches
// the schema-driven prompt with no type step in between. Reaching the detail prompt proves that.
func (ts *AgentOnboardingFlowTestSuite) TestAgentOnboardingFlow_ResolvesTheAgentTypeWithoutAResolverNode() {
	_, step, _ := ts.step("", "", "", nil)
	executionID := step.ExecutionID

	_, step, _ = ts.step(executionID, step.ChallengeToken, agentOwnerAction, map[string]string{})
	_, step, body := ts.step(executionID, step.ChallengeToken, agentNameAction,
		map[string]string{agentNameInput: common.GenerateUniqueUsername("integration_agent_type")})

	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "%s", string(body))
	ts.Contains(inputIdentifiers(step), agentModelProviderInput,
		"the agent type's attributes are prompted, so its schema was resolved")
}

// The flow runs behind a permission validator, so an anonymous caller must not be able to create an
// agent by driving it directly.
func (ts *AgentOnboardingFlowTestSuite) TestAgentOnboardingFlow_RejectsAnAnonymousCaller() {
	reqBody, err := json.Marshal(map[string]interface{}{"flowId": ts.flowID})
	ts.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPost, testServerURL+"/flow/execute", bytes.NewReader(reqBody))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)

	var step common.FlowStep
	_ = json.Unmarshal(body, &step)

	ts.NotEqual("COMPLETE", step.FlowStatus, "an anonymous run must not provision an agent: %s", string(body))
}
