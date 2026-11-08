// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	// sharedDeclAgentID and sharedDeclClientAgentID live in decl-ou-1, and their resource files share them
	// into decl-m2m-root. The second has a client credentials client.
	sharedDeclAgentID       = "decl-agent-1"
	sharedDeclClientAgentID = "decl-agent-credentials-1"
	sharedDeclClientID      = "decl-agent-cred-client-1"
	sharedDeclClientSecret  = "decl-agent-cred-secret-1"
	// unsharedDeclAgentID lives in decl-ou-1 too, is shared nowhere, and has a client credentials client.
	unsharedDeclAgentID      = "decl-agent-confidential-1"
	unsharedDeclClientID     = "decl-agent-conf-client-1"
	unsharedDeclClientSecret = "decl-agent-conf-secret-1"
	agentSharedIntoOUID      = "decl-m2m-root"
	// agentNotReachedOUID sits beneath decl-m2m-root. The agents' policies reach that root only.
	agentNotReachedOUID = "decl-m2m-child-a"

	// sharedDeclClientAgentName and its password sign decl-agent-credentials-1 in through a flow.
	sharedDeclClientAgentName     = "Declarative Credential Test Agent"
	sharedDeclClientAgentPassword = "Decl-Agent-Pass1!"
	// declResourceServerID is the declared resource server the roles of decl-m2m-root grant permissions on.
	declResourceServerID         = "decl-rs-1"
	declResourceServerIdentifier = "https://localhost:8090/decl-rs-1"
	declResourceRead             = "test-resource:read"
	declResourceWrite            = "test-resource:write"
	// agentOwnerOUID owns the declared agents.
	agentOwnerOUID = "decl-ou-1"
)

// AgentSharingTestSuite covers agents shared by the resource files that declare them.
type AgentSharingTestSuite struct {
	suite.Suite
}

func TestAgentSharingTestSuite(t *testing.T) {
	suite.Run(t, new(AgentSharingTestSuite))
}

// A unit lists the agents shared into it under its own ID, as agents it does not own and cannot change,
// and does not list an agent that is not shared with it.
func (ts *AgentSharingTestSuite) TestAUnitListsTheAgentsSharedIntoIt() {
	resp, err := doGet(testServerURL + agentBasePath + "?limit=100&" + agentOUFilter(agentSharedIntoOUID))
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode, readBody(resp))
	var list ouAgentList
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&list))

	listed := make(map[string]ouListedAgent, len(list.Agents))
	for _, a := range list.Agents {
		listed[a.ID] = a
	}
	for _, id := range []string{sharedDeclAgentID, sharedDeclClientAgentID} {
		a, ok := listed[id]
		ts.Require().True(ok, "%s is listed", id)
		ts.Equal(agentSharedIntoOUID, a.OUID, id)
		ts.Require().NotNil(a.IsOwnedByOU, id)
		ts.False(*a.IsOwnedByOU, id)
		ts.True(a.IsReadOnly, id)
	}
	ts.NotContains(listed, unsharedDeclAgentID)
}

// A unit an agent is shared into can assign it a role there. An agent of the same unit that is not
// shared cannot be.
func (ts *AgentSharingTestSuite) TestAUnitAssignsARoleToAnAgentSharedIntoIt() {
	roleID, err := testutils.CreateRole(testutils.Role{Name: "Agent Sharing Test Role", OUID: agentSharedIntoOUID})
	ts.Require().NoError(err)
	defer func() {
		if err := testutils.DeleteRole(roleID); err != nil {
			ts.T().Logf("cleanup: delete role %s: %v", roleID, err)
		}
	}()
	addURL := testServerURL + "/roles/" + roleID + "/assignments/add"

	resp, err := doPost(addURL, map[string]interface{}{
		"assignments": []map[string]string{{"id": sharedDeclAgentID, "type": "agent"}},
	})
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusNoContent, resp.StatusCode, readBody(resp))
	resp.Body.Close()

	resp, err = doGet(testServerURL + "/roles/" + roleID + "/assignments?limit=100")
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, resp.StatusCode, readBody(resp))
	var assignments struct {
		Assignments []struct {
			ID   string `json:"id"`
			Type string `json:"type"`
			OUID string `json:"ouId"`
		} `json:"assignments"`
	}
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&assignments))
	resp.Body.Close()
	ts.Require().Len(assignments.Assignments, 1)
	ts.Equal(sharedDeclAgentID, assignments.Assignments[0].ID)
	ts.Equal("agent", assignments.Assignments[0].Type)
	ts.Equal(agentSharedIntoOUID, assignments.Assignments[0].OUID)

	resp, err = doPost(addURL, map[string]interface{}{
		"assignments": []map[string]string{{"id": unsharedDeclAgentID, "type": "agent"}},
	})
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusBadRequest, resp.StatusCode, readBody(resp))
	ts.Equal("ROL-1030", errorCode(resp))
}

// The client of an agent shared into a unit gets a token for that unit. The client of an agent that is
// not shared there is refused.
func (ts *AgentSharingTestSuite) TestASharedAgentsClientGetsATokenForTheUnit() {
	status, body := ts.requestTokenFor(agentSharedIntoOUID, sharedDeclClientID, sharedDeclClientSecret)
	ts.Require().Equal(http.StatusOK, status, "%v", body)
	ts.NotEmpty(body["access_token"])

	status, body = ts.requestTokenFor(agentSharedIntoOUID, unsharedDeclClientID, unsharedDeclClientSecret)
	ts.Equal(http.StatusBadRequest, status, "%v", body)
	ts.Equal("unauthorized_client", body["error"])
}

// An agent can also sign in through a flow, as a user does, to the unit that owns it and to a unit it is
// shared with. It holds a different role in each, and each sign-in asks for both permissions: only the role
// and the permission of the unit the sign-in lands in come through. Its client's own token does not count the
// shared unit's role (TestSharedRole_SharedAgentsAssignmentInTheOUIsNotInItsToken). A unit the agent is not
// shared with does not find it.
func (ts *AgentSharingTestSuite) TestASharedAgentSignsInWithOnlyTheRolesOfEachUnit() {
	flowID := ts.createAgentSignInFlow()
	units := []struct {
		name, ouID, permission, role string
	}{
		{name: "Owner", ouID: agentOwnerOUID, permission: declResourceWrite, role: "Agent Sharing Owner Role"},
		{name: "Shared", ouID: agentSharedIntoOUID, permission: declResourceRead, role: "Agent Sharing Shared Role"},
	}
	for _, u := range units {
		roleID, err := testutils.CreateRole(testutils.Role{
			Name: u.role,
			OUID: u.ouID,
			Permissions: []testutils.ResourcePermissions{
				{ResourceServerID: declResourceServerID, Permissions: []string{u.permission}},
			},
			Assignments: []testutils.Assignment{{ID: sharedDeclClientAgentID, Type: "agent"}},
		})
		ts.Require().NoError(err, "create role %s", u.role)
		defer func() {
			if err := testutils.DeleteRole(roleID); err != nil {
				ts.T().Logf("cleanup: delete role %s: %v", roleID, err)
			}
		}()
	}
	inputs := map[string]string{
		"requested_permissions":      declResourceRead + " " + declResourceWrite,
		"resource_server_identifier": declResourceServerIdentifier,
	}

	for _, u := range units {
		ts.Run(u.name, func() {
			step := ts.signInAsSharedAgent(ts.agentSignInApp("Agent Sharing Sign-in "+u.name, u.ouID, flowID), inputs)
			ts.Require().Equal("COMPLETE", step.FlowStatus, "the agent signs in to the unit: %+v", step)
			claims, err := testutils.DecodeJWT(step.Assertion)
			ts.Require().NoError(err)
			ts.Equal(sharedDeclClientAgentID, claims.Sub)
			ts.Equal(u.ouID, claims.OUID)
			ts.Equal(u.permission, claims.Additional["authorized_permissions"],
				"only the permission granted in the sign-in's unit is authorized")
			ts.Equal([]interface{}{u.role}, claims.Additional["roles"],
				"only the role assigned in the sign-in's unit is listed")
		})
	}

	step := ts.signInAsSharedAgent(ts.agentSignInApp("Agent Sharing Sign-in Not Reached", agentNotReachedOUID, flowID),
		inputs)
	ts.NotEqual("COMPLETE", step.FlowStatus, "a unit the agent is not shared with does not find it")
	ts.Empty(step.Assertion)
}

// The same holds for groups: the agent is a member of a group in its owning unit and of one in the unit it is
// shared with, and each sign-in lists only the group of the unit it lands in. A unit cannot make an agent that
// is not shared with it a member of its group.
func (ts *AgentSharingTestSuite) TestASharedAgentSignsInWithOnlyTheGroupsOfEachUnit() {
	flowID := ts.createAgentSignInFlow()
	units := []struct {
		name, ouID, group string
	}{
		{name: "Owner", ouID: agentOwnerOUID, group: "Agent Sharing Owner Group"},
		{name: "Shared", ouID: agentSharedIntoOUID, group: "Agent Sharing Shared Group"},
	}
	for _, u := range units {
		groupID, err := testutils.CreateGroup(testutils.Group{
			Name: u.group, OUID: u.ouID, Members: []testutils.Member{{Id: sharedDeclClientAgentID, Type: "agent"}},
		})
		ts.Require().NoError(err, "create group %s", u.group)
		defer func() {
			if err := testutils.DeleteGroup(groupID); err != nil {
				ts.T().Logf("cleanup: delete group %s: %v", groupID, err)
			}
		}()
	}

	for _, u := range units {
		ts.Run(u.name, func() {
			step := ts.signInAsSharedAgent(ts.agentSignInApp("Agent Sharing Groups "+u.name, u.ouID, flowID), nil)
			ts.Require().Equal("COMPLETE", step.FlowStatus, "the agent signs in to the unit: %+v", step)
			claims, err := testutils.DecodeJWT(step.Assertion)
			ts.Require().NoError(err)
			ts.Equal(u.ouID, claims.OUID)
			ts.Equal([]interface{}{u.group}, claims.Additional["groups"],
				"only the group the sign-in's unit owns is listed")
		})
	}

	resp, err := doPost(testServerURL+"/groups", map[string]interface{}{
		"name":    "Agent Sharing Refused Group",
		"ouId":    agentSharedIntoOUID,
		"members": []map[string]string{{"id": unsharedDeclAgentID, "type": "agent"}},
	})
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusBadRequest, resp.StatusCode, readBody(resp))
	ts.Equal("GRP-1019", errorCode(resp))
}

// createAgentSignInFlow creates the agent sign-in flow and deletes it when the test ends.
func (ts *AgentSharingTestSuite) createAgentSignInFlow() string {
	ts.T().Helper()
	flowID, err := testutils.CreateFlow(agentSignInFlow())
	ts.Require().NoError(err)
	ts.T().Cleanup(func() {
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("cleanup: delete flow %s: %v", flowID, err)
		}
	})
	return flowID
}

// agentSignInApp creates an application in the organization unit whose sign-in admits agents of any type and
// asserts the unit, the roles and the groups, and deletes it when the test ends.
func (ts *AgentSharingTestSuite) agentSignInApp(name, ouID, flowID string) string {
	ts.T().Helper()
	appID, err := testutils.CreateApplication(testutils.Application{
		Name:              name,
		OUID:              ouID,
		Type:              "fullstack",
		AuthFlowID:        flowID,
		AllowedAgentTypes: []string{"*"},
		AssertionConfig:   map[string]interface{}{"userAttributes": []string{"ouId", "roles", "groups"}},
		Embedded:          true,
	})
	ts.Require().NoError(err, "create application %s", name)
	ts.T().Cleanup(func() {
		if err := testutils.DeleteApplication(appID); err != nil {
			ts.T().Logf("cleanup: delete application %s: %v", appID, err)
		}
	})
	return appID
}

// signInAsSharedAgent signs decl-agent-credentials-1 in to the application with its name and password.
func (ts *AgentSharingTestSuite) signInAsSharedAgent(appID string, inputs map[string]string) *common.FlowStep {
	ts.T().Helper()
	step, err := common.InitiateAuthenticationFlow(appID, false, inputs, "")
	ts.Require().NoError(err)
	ts.Require().True(common.HasInput(step.Data.Inputs, "name"), "expected the credentials prompt: %+v", step)
	next, err := common.CompleteFlow(step.ExecutionID,
		map[string]string{"name": sharedDeclClientAgentName, "password": sharedDeclClientAgentPassword},
		"action_001", step.ChallengeToken)
	ts.Require().NoError(err)
	return next
}

// agentSignInFlow returns a sign-in flow that identifies an agent by its name and password, authorizes the
// requested permissions, and asserts the result.
func agentSignInFlow() testutils.Flow {
	inputs := []map[string]interface{}{
		{"ref": "input_001", "identifier": "name", "type": "TEXT_INPUT", "required": true},
		{"ref": "input_002", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
	}
	return testutils.Flow{
		Name:     "Agent Sharing Sign-in",
		FlowType: "AUTHENTICATION",
		Handle:   "agent_sharing_sign_in",
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "prompt_credentials"},
			{
				"id":   "prompt_credentials",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": inputs,
						"action": map[string]interface{}{"ref": "action_001", "nextNode": "credentials_auth"},
					},
				},
			},
			{
				"id":           "credentials_auth",
				"type":         "TASK_EXECUTION",
				"executor":     map[string]interface{}{"name": "CredentialsAuthExecutor", "inputs": inputs},
				"onSuccess":    "authorization_check",
				"onIncomplete": "prompt_credentials",
			},
			{
				"id":        "authorization_check",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "AuthorizationExecutor"},
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
}

// An export of a declared agent carries the sharing its file declares, and an unshared agent's carries
// none.
func (ts *AgentSharingTestSuite) TestADeclaredAgentsExportCarriesItsSharing() {
	exported := func(agentID string) string {
		resp, err := doPost(testServerURL+"/export", map[string]interface{}{"agents": []string{agentID}})
		ts.Require().NoError(err)
		defer resp.Body.Close()
		ts.Require().Equal(http.StatusOK, resp.StatusCode, readBody(resp))
		var body struct {
			Resources string `json:"resources"`
		}
		ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&body))
		return body.Resources
	}

	document := exported(sharedDeclAgentID)
	ts.Contains(document, "sharingPolicies:")
	ts.Contains(document, "id: decl-agent-1-policy-1")
	ts.Contains(document, "ouId: "+agentSharedIntoOUID)

	ts.NotContains(exported(unsharedDeclAgentID), "sharingPolicies:")
}

// requestTokenFor asks for a client credentials token for the organization unit.
func (ts *AgentSharingTestSuite) requestTokenFor(ouID, clientID, clientSecret string) (int, map[string]interface{}) {
	ts.T().Helper()
	form := url.Values{"grant_type": {"client_credentials"}}
	req, err := http.NewRequest(http.MethodPost, testServerURL+"/ou/"+ouID+"/oauth2/token",
		strings.NewReader(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)

	// The request authenticates as the client, so the harness must not add an admin bearer token.
	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	body := map[string]interface{}{}
	ts.Require().NoError(json.Unmarshal(raw, &body), string(raw))
	return resp.StatusCode, body
}
