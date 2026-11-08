// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authentication

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	agentSubjectAgentUsername = "agent_subject_bot"
	agentSubjectAgentPassword = "AgentSubjectBot1!"
	// agentSubjectAgentType is the bootstrap agent type, which the suite's first agent is of.
	agentSubjectAgentType = "default"
	// agentSubjectOtherAgentType is an agent type the suite creates, in the organization unit above the
	// suite's, for an agent that is not of the `default` type.
	agentSubjectOtherAgentType     = "agent-subject-other"
	agentSubjectOtherAgentUsername = "agent_subject_other_bot"
	agentSubjectOtherAgentPassword = "AgentSubjectOther1!"
	// agentSubjectAuthFailedErrorCode is what the credentials node reports when the subject itself
	// is rejected. A wrong password or an unknown identifier reports a different code, so asserting
	// this one keeps the test honest about why the sign-in failed.
	agentSubjectAuthFailedErrorCode = "FET-1006"
)

// AgentSubjectTypeTestSuite covers the application's allowedAgentTypes setting as a subject
// constraint: an agent may sign in to an application only when its agent type is listed or the list
// is `["*"]`, and an application that lists none accepts no agent at all. The applications share one
// authentication flow, so the flow's SSO cookie also lets the suite drive an agent that holds a live
// session into the application that does not accept it.
type AgentSubjectTypeTestSuite struct {
	suite.Suite
	ouID              string
	agentTypeSnapshot *testutils.AgentTypeSnapshot
	agentID           string
	flowID            string
	allowedAppID      string
	deniedAppID       string
	anyTypeAppID      string
	otherOUID         string
	otherAgentTypeID  string
	otherAgentID      string
}

func TestAgentSubjectTypeTestSuite(t *testing.T) {
	suite.Run(t, new(AgentSubjectTypeTestSuite))
}

func (ts *AgentSubjectTypeTestSuite) SetupSuite() {
	// A sign-in finds only the agents of the application's organization unit, so both agents live in the
	// suite's unit. That unit holds the `default` agent type and an organization unit holds one agent type,
	// so the other agent type sits in the unit above it, which its agent's placement allows.
	otherOUID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "agent-subject-other-type-ou",
		Name:        "Agent Subject Other Type OU",
		Description: "Organization unit holding the suite's non-default agent type",
	})
	ts.Require().NoError(err, "Failed to create the other agent type's organization unit")
	ts.otherOUID = otherOUID

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "agent-subject-type-ou",
		Name:        "Agent Subject Type OU",
		Description: "Organization unit for allowed agent type integration testing",
		Parent:      &ts.otherOUID,
	})
	ts.Require().NoError(err, "Failed to create test organization unit")
	ts.ouID = ouID

	// The `default` agent type is shared with every other suite. Snapshot it before
	// installing this suite's schema, so teardown can put it back before the OU is deleted.
	snapshot, err := testutils.SnapshotAgentType()
	ts.Require().NoError(err, "Failed to snapshot the default agent type")
	ts.agentTypeSnapshot = snapshot

	// agent_username is unique so the credentials node can identify the agent by it, and password
	// is a credential so it is stored hashed and verified at authentication.
	_, err = testutils.CreateAgentType(testutils.UserType{
		Handle:      agentSubjectAgentType,
		DisplayName: "Default",
		OUID:        ts.ouID,
		Schema: map[string]interface{}{
			"name": map[string]interface{}{
				"type":     "string",
				"required": true,
				"unique":   true,
			},
			"agent_username": map[string]interface{}{
				"type":     "string",
				"required": true,
				"unique":   true,
			},
			"password": map[string]interface{}{"type": "string", "credential": true},
		},
	})
	ts.Require().NoError(err, "Failed to install the agent type schema")

	agentID, err := testutils.CreateAgent(testutils.Agent{
		Type:        agentSubjectAgentType,
		Description: "Agent that signs in to the test applications",
		OUID:        ts.ouID,
		Attributes: map[string]interface{}{
			"name":           "agent-subject-type-bot",
			"agent_username": agentSubjectAgentUsername,
			"password":       agentSubjectAgentPassword,
		},
	})
	ts.Require().NoError(err, "Failed to create the test agent")
	ts.agentID = agentID

	ts.flowID = ts.createAuthenticationFlow()

	allowedAppID, err := testutils.CreateApplication(ts.appConfig(
		"Agent Subject Type Allowed App", []string{agentSubjectAgentType}))
	ts.Require().NoError(err, "Failed to create the agent-allowing application")
	ts.allowedAppID = allowedAppID

	deniedAppID, err := testutils.CreateApplication(ts.appConfig(
		"Agent Subject Type Denied App", nil))
	ts.Require().NoError(err, "Failed to create the agent-denying application")
	ts.deniedAppID = deniedAppID

	anyTypeAppID, err := testutils.CreateApplication(ts.appConfig(
		"Agent Subject Type Any Type App", []string{"*"}))
	ts.Require().NoError(err, "Failed to create the application admitting any agent type")
	ts.anyTypeAppID = anyTypeAppID

	ts.setUpOtherAgentType()
}

// setUpOtherAgentType creates an agent type other than `default`, in the organization unit above the
// suite's, and an agent of it in the suite's organization unit.
func (ts *AgentSubjectTypeTestSuite) setUpOtherAgentType() {
	otherAgentTypeID, err := testutils.CreateNewAgentType(testutils.UserType{
		Handle:      agentSubjectOtherAgentType,
		DisplayName: "Agent Subject Other",
		OUID:        ts.otherOUID,
		Schema: map[string]interface{}{
			"name": map[string]interface{}{"type": "string", "required": true, "unique": true},
			"agent_username": map[string]interface{}{
				"type":     "string",
				"required": true,
				"unique":   true,
			},
			"password": map[string]interface{}{"type": "string", "credential": true},
		},
	})
	ts.Require().NoError(err, "Failed to create the other agent type")
	ts.otherAgentTypeID = otherAgentTypeID

	otherAgentID, err := testutils.CreateAgent(testutils.Agent{
		Type:        agentSubjectOtherAgentType,
		Description: "Agent of a non-default agent type",
		OUID:        ts.ouID,
		Attributes: map[string]interface{}{
			"name":           "agent-subject-other-bot",
			"agent_username": agentSubjectOtherAgentUsername,
			"password":       agentSubjectOtherAgentPassword,
		},
	})
	ts.Require().NoError(err, "Failed to create the agent of the other agent type")
	ts.otherAgentID = otherAgentID
}

func (ts *AgentSubjectTypeTestSuite) TearDownSuite() {
	for _, appID := range []string{ts.allowedAppID, ts.deniedAppID, ts.anyTypeAppID} {
		if appID != "" {
			if err := testutils.DeleteApplication(appID); err != nil {
				ts.T().Logf("teardown: failed to delete application %s: %v", appID, err)
			}
		}
	}
	if ts.flowID != "" {
		if err := testutils.DeleteFlow(ts.flowID); err != nil {
			ts.T().Logf("teardown: failed to delete flow: %v", err)
		}
	}
	if ts.agentID != "" {
		if err := testutils.DeleteAgent(ts.agentID); err != nil {
			ts.T().Logf("teardown: failed to delete agent: %v", err)
		}
	}
	if ts.otherAgentID != "" {
		if err := testutils.DeleteAgent(ts.otherAgentID); err != nil {
			ts.T().Logf("teardown: failed to delete the other agent: %v", err)
		}
	}
	if ts.otherAgentTypeID != "" {
		if err := testutils.DeleteAgentType(ts.otherAgentTypeID); err != nil {
			ts.T().Errorf("teardown: failed to delete the other agent type: %v", err)
		}
	}
	// Restore the shared agent type before deleting the OU it points at, or the `default` type is left
	// referencing a deleted OU and a later suite's restore fails. A failed restore keeps the OU:
	// leaking one is cheaper than every later suite inheriting a dangling reference.
	agentTypeRestored := true
	if ts.agentTypeSnapshot != nil {
		if err := testutils.RestoreAgentType(ts.agentTypeSnapshot); err != nil {
			ts.T().Errorf("teardown: failed to restore the default agent type: %v", err)
			agentTypeRestored = false
		}
	}
	if ts.ouID != "" && agentTypeRestored {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("teardown: failed to delete organization unit: %v", err)
		}
	}
	// The other agent type's organization unit is the parent of the suite's, so it goes last.
	if ts.otherOUID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.otherOUID); err != nil {
			ts.T().Logf("teardown: failed to delete the other organization unit: %v", err)
		}
	}
}

// TestAgentSignsInWhenItsTypeIsAllowed is the control: the application lists the agent's type, so
// the same credentials that are rejected below complete the flow.
func (ts *AgentSubjectTypeTestSuite) TestAgentSignsInWhenItsTypeIsAllowed() {
	client := ts.newSessionClient()

	step := ts.submitAgentCredentials(client, ts.allowedAppID)

	ts.Require().Equal("COMPLETE", step.FlowStatus, "the allowed agent should complete the flow")
	ts.Require().NotEmpty(step.Assertion, "the allowed agent should receive an assertion")
	ts.Require().Nil(step.Error, "no error is expected for an allowed agent")
}

// TestAgentRejectedWhenNoAgentTypeIsAllowed verifies that an application listing no agent type
// accepts no agent: the credentials are correct, so only the subject constraint can fail the node.
func (ts *AgentSubjectTypeTestSuite) TestAgentRejectedWhenNoAgentTypeIsAllowed() {
	client := ts.newSessionClient()

	step := ts.submitAgentCredentials(client, ts.deniedAppID)

	ts.Require().NotEqual("COMPLETE", step.FlowStatus, "the barred agent must not complete the flow")
	ts.Require().Empty(step.Assertion, "no assertion should be issued to a barred agent")
	ts.Require().NotNil(step.Error, "the rejected authentication should report an error")
	ts.Equal(agentSubjectAuthFailedErrorCode, step.Error.Code,
		"the sign-in should fail on the subject, not on the credentials")
}

// TestBarredAgentCannotRideItsSessionIntoTheApplication verifies that the constraint is re-checked
// on SSO reuse. The agent signs in to the application that accepts it, then joins the application
// that accepts no agent over the same per-flow session, where the replayed subject is rejected.
func (ts *AgentSubjectTypeTestSuite) TestBarredAgentCannotRideItsSessionIntoTheApplication() {
	client := ts.newSessionClient()

	first := ts.submitAgentCredentials(client, ts.allowedAppID)
	ts.Require().Equal("COMPLETE", first.FlowStatus, "the first sign-in should establish a session")

	status, joined := ts.initiateFlow(client, ts.deniedAppID)

	if status == http.StatusOK {
		ts.Require().NotEqual("COMPLETE", joined.FlowStatus,
			"the barred agent must not complete the joining application's flow")
		ts.Require().Empty(joined.Assertion, "no assertion should be issued over the replayed session")
		return
	}
	// The assertion node resolves the replayed subject itself, so a rejection there surfaces as a
	// failed execution rather than a flow step. The first sign-in completing on the same flow rules
	// out any cause other than the joining application's constraint.
	ts.Require().GreaterOrEqual(status, http.StatusBadRequest,
		"the joining flow should fail rather than return a flow step")
}

// TestAgentOfAnyTypeSignsInWhenAnyAgentTypeIsAllowed verifies that `["*"]` admits an agent whatever
// its agent type: both the `default` agent and the agent of the suite's other type complete the flow.
func (ts *AgentSubjectTypeTestSuite) TestAgentOfAnyTypeSignsInWhenAnyAgentTypeIsAllowed() {
	for name, credentials := range map[string][2]string{
		"default agent type": {agentSubjectAgentUsername, agentSubjectAgentPassword},
		"other agent type":   {agentSubjectOtherAgentUsername, agentSubjectOtherAgentPassword},
	} {
		ts.Run(name, func() {
			step := ts.submitCredentials(ts.newSessionClient(), ts.anyTypeAppID, credentials[0], credentials[1])

			ts.Require().Equal("COMPLETE", step.FlowStatus, "the agent should complete the flow")
			ts.Require().NotEmpty(step.Assertion, "the agent should receive an assertion")
			ts.Require().Nil(step.Error, "no error is expected for an admitted agent")
		})
	}
}

// TestAgentOfUnlistedTypeRejected verifies that an application listing only `default` rejects an agent
// of another agent type whose credentials are correct, as the `["*"]` application above accepts them.
func (ts *AgentSubjectTypeTestSuite) TestAgentOfUnlistedTypeRejected() {
	step := ts.submitCredentials(ts.newSessionClient(), ts.allowedAppID,
		agentSubjectOtherAgentUsername, agentSubjectOtherAgentPassword)

	ts.Require().NotEqual("COMPLETE", step.FlowStatus, "the unlisted agent type must not complete the flow")
	ts.Require().Empty(step.Assertion, "no assertion should be issued to an unlisted agent type")
	ts.Require().NotNil(step.Error, "the rejected authentication should report an error")
	ts.Equal(agentSubjectAuthFailedErrorCode, step.Error.Code,
		"the sign-in should fail on the subject, not on the credentials")
}

// submitAgentCredentials runs the flow for the given application up to the credentials node with the
// `default` agent's credentials and returns the step that node produced.
func (ts *AgentSubjectTypeTestSuite) submitAgentCredentials(
	client *http.Client, appID string,
) *testutils.FlowStep {
	return ts.submitCredentials(client, appID, agentSubjectAgentUsername, agentSubjectAgentPassword)
}

// submitCredentials runs the flow for the given application up to the credentials node with the given
// credentials and returns the step that node produced.
func (ts *AgentSubjectTypeTestSuite) submitCredentials(
	client *http.Client, appID, username, password string,
) *testutils.FlowStep {
	status, initial := ts.initiateFlow(client, appID)
	ts.Require().Equal(http.StatusOK, status, "flow initiation should succeed")
	ts.Require().NotEmpty(initial.ExecutionID, "the flow should return an execution id")
	ts.Require().NotEqual("COMPLETE", initial.FlowStatus, "the flow should prompt for credentials")

	status, step := ts.flowExecute(client, map[string]interface{}{
		"executionId": initial.ExecutionID,
		"inputs": map[string]string{
			"agent_username": username,
			"password":       password,
		},
		"action":         "action_001",
		"challengeToken": initial.ChallengeToken,
	})
	ts.Require().Equal(http.StatusOK, status, "submitting the agent credentials should return a flow step")
	return step
}

func (ts *AgentSubjectTypeTestSuite) initiateFlow(
	client *http.Client, appID string,
) (int, *testutils.FlowStep) {
	return ts.flowExecuteForApp(client, appID, map[string]interface{}{
		"applicationId": appID,
		"flowType":      "AUTHENTICATION",
	})
}

func (ts *AgentSubjectTypeTestSuite) flowExecute(
	client *http.Client, body map[string]interface{},
) (int, *testutils.FlowStep) {
	return ts.flowExecuteForApp(client, "", body)
}

// flowExecuteForApp posts to the flow execution endpoint with the client's cookie jar attached, so
// the per-flow SSO cookie set by one application's flow is replayed to the next. It returns the
// status code alongside the decoded step, since a flow that fails at the engine level answers with
// an error body instead.
func (ts *AgentSubjectTypeTestSuite) flowExecuteForApp(
	client *http.Client, appID string, body map[string]interface{},
) (int, *testutils.FlowStep) {
	data, err := json.Marshal(body)
	ts.Require().NoError(err)

	req, err := http.NewRequest("POST", testutils.TestServerURL+"/flow/execute", bytes.NewReader(data))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if appID != "" {
		if flowSecret := testutils.GetFlowSecret(appID); flowSecret != "" {
			req.Header.Set(testutils.FlowSecretHeaderName, flowSecret)
		}
	}

	resp, err := client.Do(req)
	ts.Require().NoError(err, "flow execute request failed")
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)

	if resp.StatusCode != http.StatusOK {
		ts.T().Logf("flow execute returned %d: %s", resp.StatusCode, string(respBody))
		return resp.StatusCode, nil
	}

	var step testutils.FlowStep
	ts.Require().NoError(json.Unmarshal(respBody, &step), "failed to decode flow step: %s", string(respBody))
	return resp.StatusCode, &step
}

// newSessionClient returns a client with its own cookie jar so each test carries its own session.
func (ts *AgentSubjectTypeTestSuite) newSessionClient() *http.Client {
	jar, err := cookiejar.New(nil)
	ts.Require().NoError(err, "Failed to create cookie jar")
	return &http.Client{
		Jar:       jar,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// createAuthenticationFlow builds an SSO-enabled authentication flow. The session node both
// establishes the session on the first sign-in and is the checkpoint SSO_CHECK looks for when the
// agent joins the second application.
func (ts *AgentSubjectTypeTestSuite) createAuthenticationFlow() string {
	flow := testutils.Flow{
		Name:     "Agent Subject Type Auth Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "agent_subject_type_auth_flow",
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "sso_check"},
			{
				"id":         "sso_check",
				"type":       "TASK_EXECUTION",
				"executor":   map[string]interface{}{"name": "SSOCheckExecutor"},
				"properties": map[string]interface{}{"checkpointRef": "session_main"},
				"onSuccess":  "session_main",
				"onFailure":  "prompt_credentials",
			},
			{
				"id":   "prompt_credentials",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{"ref": "input_001", "identifier": "agent_username", "type": "TEXT_INPUT",
								"required": true},
							{"ref": "input_002", "identifier": "password", "type": "PASSWORD_INPUT",
								"required": true},
						},
						"action": map[string]interface{}{"ref": "action_001", "nextNode": "credentials_auth"},
					},
				},
			},
			{
				"id":   "credentials_auth",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "CredentialsAuthExecutor",
					"inputs": []map[string]interface{}{
						{"ref": "input_001", "identifier": "agent_username", "type": "TEXT_INPUT",
							"required": true},
						{"ref": "input_002", "identifier": "password", "type": "PASSWORD_INPUT",
							"required": true},
					},
				},
				"onSuccess":    "session_main",
				"onIncomplete": "prompt_credentials",
			},
			{
				"id":        "session_main",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "SessionExecutor"},
				"onSuccess": "auth_assert",
			},
			{
				"id":        "auth_assert",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "AuthAssertExecutor"},
				"onSuccess": "end",
			},
			{"id": "end", "type": "END"},
		},
	}

	flowID, err := testutils.CreateFlow(flow)
	ts.Require().NoError(err, "Failed to create the test authentication flow")
	return flowID
}

// appConfig builds an application sharing the suite's authentication flow. allowedAgentTypes is
// left off entirely when nil, which is the configuration that accepts no agent.
func (ts *AgentSubjectTypeTestSuite) appConfig(name string, allowedAgentTypes []string) testutils.Application {
	return testutils.Application{
		Name:              name,
		Description:       "Application for allowed agent type integration testing",
		OUID:              ts.ouID,
		Type:              "fullstack",
		AuthFlowID:        ts.flowID,
		AllowedAgentTypes: allowedAgentTypes,
		Embedded:          true,
	}
}
