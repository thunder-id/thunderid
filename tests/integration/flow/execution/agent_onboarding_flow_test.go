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

// The inputs the shipped flow collects. All of them are attributes the default agent type
// declares, including the required, unique name.
const (
	agentNameInput          = "name"
	agentModelProviderInput = "modelProvider"
	agentModelInput         = "model"
	agentOwnerInput         = "owner"
	agentDelegatedInput     = "delegated"
	agentRedirectURIsInput  = "redirectUris"
)

// The user type the bootstrap seeds, whose schema requires a unique username and email.
const seededUserTypeName = "person"

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
	agentDetailsAction = "action_agent_details"
)

// Every agent a test provisions needs a field of its own: the suite deletes them all in teardown,
// and a shared field would leave the agents it was overwritten with behind on the server.
type AgentOnboardingFlowTestSuite struct {
	suite.Suite
	flowID          string
	createdAgent    string
	bareAgent       string
	takenNameAgent  string
	secondAgent     string
	ownedAgent      string
	delegatedAgent  string
	ownerCheckAgent string
	ownerUserID     string
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
	for _, agentID := range []string{ts.createdAgent, ts.bareAgent, ts.takenNameAgent, ts.secondAgent,
		ts.ownedAgent, ts.delegatedAgent, ts.ownerCheckAgent} {
		if agentID == "" {
			continue
		}
		if err := testutils.DeleteAgent(agentID); err != nil {
			ts.T().Logf("Failed to delete the provisioned agent during teardown: %v", err)
		}
	}
	if ts.ownerUserID != "" {
		if err := testutils.DeleteUser(ts.ownerUserID); err != nil {
			ts.T().Logf("Failed to delete the owner user during teardown: %v", err)
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

// A value the agent type rejects at creation sends the run back to the details prompt, which must
// still offer the choices of its enumerated attributes so the administrator can pick again.
func (ts *AgentOnboardingFlowTestSuite) TestAgentOnboardingFlow_RejectedValueReturnsToDetailsWithChoices() {
	_, step, _ := ts.step("", "", "", nil)
	executionID := step.ExecutionID
	_, step, _ = ts.step(executionID, step.ChallengeToken, agentOwnerAction, map[string]string{})

	_, step, body := ts.step(executionID, step.ChallengeToken, agentDetailsAction, map[string]string{
		agentNameInput:          common.GenerateUniqueUsername("integration_agent_rejected"),
		agentModelProviderInput: "not-in-the-enum",
	})

	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "the run returns to the details prompt: %s", string(body))
	ts.Require().NotNil(step.Error, "the rejection is reported")
	found := false
	for _, input := range step.Data.Inputs {
		if input.Identifier == agentModelProviderInput {
			found = true
			ts.NotEmpty(input.Options, "the model provider choices are offered again")
		}
	}
	ts.True(found, "the details prompt asks for the model provider again")
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
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "The flow should pause for the details: %s", string(body))

	// The detail prompt is driven by the agent type schema, so its inputs prove the executor
	// resolved the agent type without a resolver node ahead of it. The name is one of them.
	ts.Contains(inputIdentifiers(step), agentNameInput, "the details prompt asks for the name")
	ts.Contains(inputIdentifiers(step), agentModelInput, "the details prompt offers the schema attributes")

	// A select the schema restricts to a fixed set must carry its choices, or the Console has nothing
	// to render. They reach the prompt only through the provisioning node, so a flow that skipped it
	// would still list the input but with no options.
	for _, input := range step.Data.Inputs {
		if input.Identifier == agentModelProviderInput {
			ts.NotEmpty(input.Options, "the model provider select carries the schema's enum values")
		}
		if input.Identifier == "function" {
			ts.NotEmpty(input.Options, "the function select carries the schema's enum values")
		}
	}

	agentName := common.GenerateUniqueUsername("integration_agent")
	status, step, body = ts.step(executionID, step.ChallengeToken, agentDetailsAction, map[string]string{
		agentNameInput:          agentName,
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
	ts.Equal(agentName, attributeString(agent.Attributes, agentNameInput),
		"the name collected by the details prompt is stored as the name attribute")
}

// Every attribute except the name is optional, so the detail step can be submitted with only a
// name filled in.
func (ts *AgentOnboardingFlowTestSuite) TestAgentOnboardingFlow_CompletesWithNoDetailsSupplied() {
	_, step, _ := ts.step("", "", "", nil)
	executionID := step.ExecutionID

	_, step, _ = ts.step(executionID, step.ChallengeToken, agentOwnerAction, map[string]string{})

	agentName := common.GenerateUniqueUsername("integration_agent_bare")
	_, step, body := ts.step(executionID, step.ChallengeToken, agentDetailsAction,
		map[string]string{agentNameInput: agentName})

	ts.Require().Equal("COMPLETE", step.FlowStatus,
		"an agent with only a name is provisionable: %s", string(body))
	ts.Require().NotEmpty(step.Data.AdditionalData[agentClientSecretData],
		"the credentials still reach the caller")

	ts.bareAgent = step.Data.AdditionalData[agentIDData]
}

// The name is a unique schema attribute collected on the details prompt. A duplicate is reported
// against the name field on that same prompt, so the caller can correct it where it was entered.
func (ts *AgentOnboardingFlowTestSuite) TestAgentOnboardingFlow_DuplicateNameReturnsToTheDetailsPrompt() {
	taken := common.GenerateUniqueUsername("integration_agent_taken")

	// The first run takes the name.
	_, step, _ := ts.step("", "", "", nil)
	first := step.ExecutionID
	_, step, _ = ts.step(first, step.ChallengeToken, agentOwnerAction, map[string]string{})
	_, step, body := ts.step(first, step.ChallengeToken, agentDetailsAction,
		map[string]string{agentNameInput: taken})
	ts.Require().Equal("COMPLETE", step.FlowStatus, "%s", string(body))
	ts.takenNameAgent = step.Data.AdditionalData[agentIDData]

	// The second run collides on it.
	_, step, _ = ts.step("", "", "", nil)
	second := step.ExecutionID
	_, step, _ = ts.step(second, step.ChallengeToken, agentOwnerAction, map[string]string{})
	_, step, body = ts.step(second, step.ChallengeToken, agentDetailsAction,
		map[string]string{agentNameInput: taken})

	ts.Require().Equal("INCOMPLETE", step.FlowStatus,
		"the run continues so the name can be corrected: %s", string(body))
	ts.Contains(inputIdentifiers(step), agentNameInput,
		"the name is asked again on the details prompt rather than the run ending: %s", string(body))
	ts.Contains(inputIdentifiers(step), agentModelInput, "the run is back on the details prompt: %s", string(body))

	// Correcting it completes without repeating the earlier steps.
	_, step, body = ts.step(second, step.ChallengeToken, agentDetailsAction,
		map[string]string{agentNameInput: common.GenerateUniqueUsername("integration_agent_freed")})
	ts.Require().Equal("COMPLETE", step.FlowStatus, "%s", string(body))
	ts.secondAgent = step.Data.AdditionalData[agentIDData]
}

// With a single agent type available the resolver settles on it without asking, so the flow reaches
// the schema-driven prompt with no type step in between. Reaching the detail prompt proves that.
func (ts *AgentOnboardingFlowTestSuite) TestAgentOnboardingFlow_ResolvesTheAgentTypeWithoutAResolverNode() {
	_, step, _ := ts.step("", "", "", nil)
	executionID := step.ExecutionID

	_, step, body := ts.step(executionID, step.ChallengeToken, agentOwnerAction, map[string]string{})
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "%s", string(body))
	ts.Contains(inputIdentifiers(step), agentNameInput, "the details prompt asks for the name")
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

	// Asserting the rejection itself, not merely that the run fell short of completing: an
	// unparseable body or a flow that started and paused would both satisfy that weaker check.
	ts.Require().Equal(http.StatusUnauthorized, resp.StatusCode,
		"an anonymous caller must be rejected outright: %s", string(body))

	var errorResponse common.ErrorResponse
	ts.Require().NoError(json.Unmarshal(body, &errorResponse),
		"the rejection must carry a structured error: %s", string(body))
	ts.Equal(errCodeAdminAuthRequired, errorResponse.Code,
		"the rejection names the administration authentication requirement: %s", string(body))
}

// createOwnerUser provisions a user for the owner test to point at, of the seeded person type and
// in that type's own organization unit so the owner resolves from the flow's scope.
func (ts *AgentOnboardingFlowTestSuite) createOwnerUser() string {
	ts.T().Helper()

	userTypes, err := testutils.ListUserTypes()
	ts.Require().NoError(err, "Failed to list user types")

	var personType *testutils.UserType
	for i := range userTypes {
		if userTypes[i].Handle == seededUserTypeName {
			personType = &userTypes[i]
			break
		}
	}
	ts.Require().NotNil(personType, "the seeded %q user type is needed to create the owner", seededUserTypeName)

	// Username and email are both required and unique on that type, so both are generated.
	username := common.GenerateUniqueUsername("agent_owner")
	attributes := `{"username":"` + username + `","email":"` + username + `@example.com"}`

	userID, err := testutils.CreateUser(testutils.User{
		OUID:       personType.OUID,
		Type:       personType.Handle,
		Attributes: []byte(attributes),
	})
	ts.Require().NoError(err, "Failed to create the owner user")
	ts.Require().NotEmpty(userID)

	return userID
}

// runToCompletion drives the flow from the owner prompt to the credentials screen, submitting the
// given owner and detail inputs. It returns the provisioned agent id.
func (ts *AgentOnboardingFlowTestSuite) runToCompletion(name, owner string, details map[string]string) string {
	ts.T().Helper()

	_, step, _ := ts.step("", "", "", nil)
	executionID := step.ExecutionID

	ownerInputs := map[string]string{}
	if owner != "" {
		ownerInputs[agentOwnerInput] = owner
	}
	_, step, _ = ts.step(executionID, step.ChallengeToken, agentOwnerAction, ownerInputs)
	inputs := map[string]string{agentNameInput: name}
	for k, v := range details {
		inputs[k] = v
	}
	_, step, body := ts.step(executionID, step.ChallengeToken, agentDetailsAction, inputs)

	ts.Require().Equal("COMPLETE", step.FlowStatus, "the flow should complete: %s", string(body))

	return step.Data.AdditionalData[agentIDData]
}

// An owner the caller picks is verified and recorded against the agent, rather than the flow
// falling back to the administrator running it.
func (ts *AgentOnboardingFlowTestSuite) TestAgentOnboardingFlow_SelectedOwnerIsRecorded() {
	ts.ownerUserID = ts.createOwnerUser()

	ts.ownedAgent = ts.runToCompletion(
		common.GenerateUniqueUsername("integration_agent_owned"), ts.ownerUserID, map[string]string{})

	agent, err := testutils.GetAgent(ts.ownedAgent)
	ts.Require().NoError(err, "Failed to read the provisioned agent")
	ts.Equal(ts.ownerUserID, agent.Owner, "the agent is owned by the user the caller chose")
}

// An owner has to be a user. Nothing downstream repeats that check, so an identifier of another
// category is refused here and the prompt is shown again rather than the run failing outright.
func (ts *AgentOnboardingFlowTestSuite) TestAgentOnboardingFlow_RejectsAnOwnerThatIsNotAUser() {
	// An agent's own identifier is the nearest thing to a user id that is not one.
	ts.ownerCheckAgent = ts.runToCompletion(
		common.GenerateUniqueUsername("integration_agent_owner_probe"), "", map[string]string{})

	_, step, _ := ts.step("", "", "", nil)
	executionID := step.ExecutionID

	_, step, body := ts.step(executionID, step.ChallengeToken, agentOwnerAction,
		map[string]string{agentOwnerInput: ts.ownerCheckAgent})

	ts.Require().Equal("INCOMPLETE", step.FlowStatus,
		"the run continues so the owner can be corrected: %s", string(body))
	ts.Contains(inputIdentifiers(step), agentOwnerInput,
		"the owner is asked again rather than the agent being provisioned: %s", string(body))
}

// A delegated agent signs a user in, so it needs a callback to return them to. The flow asks for
// one rather than provisioning an agent that cannot complete the exchange.
//
// The shipped flow collects no delegation inputs, so they are submitted directly here, which is
// what a flow carrying the delegation widget would do.
func (ts *AgentOnboardingFlowTestSuite) TestAgentOnboardingFlow_DelegationRequiresARedirectURI() {
	_, step, _ := ts.step("", "", "", nil)
	executionID := step.ExecutionID

	_, step, _ = ts.step(executionID, step.ChallengeToken, agentOwnerAction, map[string]string{})

	_, step, body := ts.step(executionID, step.ChallengeToken, agentDetailsAction, map[string]string{
		agentNameInput:      common.GenerateUniqueUsername("integration_agent_delegated_bare"),
		agentDelegatedInput: "true",
	})

	ts.Require().Equal("INCOMPLETE", step.FlowStatus,
		"delegation without a callback pauses rather than completing: %s", string(body))
	ts.Contains(inputIdentifiers(step), agentRedirectURIsInput,
		"the callback is asked for: %s", string(body))
}

// Given the callback, the agent is provisioned as a delegated client: it keeps the URI it was
// given and gains the authorization code grant it needs to sign a user in.
func (ts *AgentOnboardingFlowTestSuite) TestAgentOnboardingFlow_DelegatedAgentKeepsItsRedirectURI() {
	const callback = "https://agent.example.com/callback"

	ts.delegatedAgent = ts.runToCompletion(
		common.GenerateUniqueUsername("integration_agent_delegated"), "",
		map[string]string{agentDelegatedInput: "true", agentRedirectURIsInput: callback})

	agent, err := testutils.GetAgent(ts.delegatedAgent)
	ts.Require().NoError(err, "Failed to read the provisioned agent")
	ts.Require().NotEmpty(agent.InboundAuthConfig, "a delegated agent carries an OAuth configuration")

	oauth := agent.InboundAuthConfig[0].Config
	ts.Require().NotNil(oauth)
	ts.Contains(oauth.RedirectURIs, callback, "the callback the caller gave is the one stored")
	ts.Contains(oauth.GrantTypes, "authorization_code", "delegation adds the authorization code grant")
	ts.True(oauth.PKCERequired, "a delegated agent requires PKCE")
}

// attributeString reads a string attribute from an agent's attributes payload.
func attributeString(attributes interface{}, key string) string {
	m, ok := attributes.(map[string]interface{})
	if !ok {
		return ""
	}
	value, _ := m[key].(string)
	return value
}
