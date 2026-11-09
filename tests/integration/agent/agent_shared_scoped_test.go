// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	sharedScopedUserType     = "agent-shared-scoped-admin-type"
	sharedScopedPassword     = "AgentShared@123"
	sharedScopedRSIdentifier = "https://agent-shared-scoped.example.com/system"
	// sharedAgentReadOnlyCode refuses a change to an agent its caller's unit does not own, as the agents
	// shared into a unit are declared.
	sharedAgentReadOnlyCode = "AGT-1027"
)

// AgentSharedScopedTestSuite covers the agents shared into the organization unit an administrator is
// confined to: the unfiltered list shows them after the agents the unit owns, a read returns them under
// that unit, and changing or deleting them is refused. Each administrator lives in agentSharedIntoOUID,
// where the declared agents sharedDeclAgentID and sharedDeclClientAgentID are shared, and holds one
// permission set there.
type AgentSharedScopedTestSuite struct {
	suite.Suite

	signIn     testutils.OUSignInApps
	userTypeID string
	rsID       string
	userIDs    []string
	roleIDs    []string

	// admins holds a client per permission set the confined administrators hold.
	admins map[string]*http.Client
	// systemAdmin holds system, which reaches every unit, in agentSharedIntoOUID.
	systemAdmin *http.Client
}

// sharedScopedAgentView is the part of an agent response these tests read.
type sharedScopedAgentView struct {
	ID          string `json:"id"`
	OUID        string `json:"ouId"`
	IsOwnedByOU *bool  `json:"isOwnedByOU"`
	IsReadOnly  bool   `json:"isReadOnly"`
}

func TestAgentSharedScopedTestSuite(t *testing.T) {
	suite.Run(t, new(AgentSharedScopedTestSuite))
}

func (ts *AgentSharedScopedTestSuite) SetupSuite() {
	var err error
	ts.userTypeID, err = testutils.CreateUserType(testutils.UserType{
		Handle:      sharedScopedUserType,
		DisplayName: "Agent Shared Scoped Admin Type",
		OUID:        agentSharedIntoOUID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string", "unique": true},
			"password": map[string]interface{}{"type": "string", "credential": true},
		},
	})
	ts.Require().NoError(err, "create the administrators' user type")

	ts.rsID, err = testutils.CreateOrgScopedResourceServer(agentSharedIntoOUID, "Agent Shared Scoped RS",
		sharedScopedRSIdentifier, "agent")
	ts.Require().NoError(err, "create the resource server")

	ts.admins = map[string]*http.Client{
		"org":            ts.adminClient("agent-shared-scoped-org", []string{"org"}),
		"org:agent":      ts.adminClient("agent-shared-scoped-agent-admin", []string{"org:agent"}),
		"org:agent:view": ts.adminClient("agent-shared-scoped-viewer", []string{"org:agent:view"}),
	}
	ts.systemAdmin = ts.adminClient("agent-shared-scoped-system", []string{"system"})
}

func (ts *AgentSharedScopedTestSuite) TearDownSuite() {
	if err := ts.signIn.DeleteAll(); err != nil {
		ts.T().Logf("teardown: delete sign-in applications: %v", err)
	}
	for _, id := range ts.roleIDs {
		if err := testutils.DeleteRole(id); err != nil {
			ts.T().Logf("teardown: delete role %s: %v", id, err)
		}
	}
	for _, id := range ts.userIDs {
		if err := testutils.DeleteUser(id); err != nil {
			ts.T().Logf("teardown: delete user %s: %v", id, err)
		}
	}
	if ts.rsID != "" {
		if err := testutils.DeleteResourceServerWithChildren(ts.rsID); err != nil {
			ts.T().Errorf("teardown: delete resource server: %v", err)
		}
	}
	if ts.userTypeID != "" {
		if err := testutils.DeleteUserType(ts.userTypeID); err != nil {
			ts.T().Logf("teardown: delete user type: %v", err)
		}
	}
}

// adminClient creates an administrator in agentSharedIntoOUID holding the permissions there and nowhere
// else, and returns a client carrying its token.
func (ts *AgentSharedScopedTestSuite) adminClient(username string, permissions []string) *http.Client {
	ts.T().Helper()
	userID, err := testutils.CreateUser(testutils.User{
		Type: sharedScopedUserType,
		OUID: agentSharedIntoOUID,
		Attributes: json.RawMessage(fmt.Sprintf(`{"username": %q, "password": %q}`,
			username, sharedScopedPassword)),
	})
	ts.Require().NoError(err, "create administrator %s", username)
	ts.userIDs = append(ts.userIDs, userID)

	roleID, err := testutils.CreateRole(testutils.Role{
		Name: username + " role",
		OUID: agentSharedIntoOUID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: ts.rsID, Permissions: permissions},
		},
		Assignments: []testutils.Assignment{{ID: userID, Type: "user"}},
	})
	ts.Require().NoError(err, "create the role of %s", username)
	ts.roleIDs = append(ts.roleIDs, roleID)

	scope := "system"
	for _, p := range permissions {
		scope += " " + p
	}
	token, err := ts.signIn.ObtainAccessToken(agentSharedIntoOUID, scope, username, sharedScopedPassword,
		sharedScopedRSIdentifier)
	ts.Require().NoError(err, "obtain the token of %s", username)
	ts.Require().NotEmpty(token.AccessToken)
	return testutils.GetHTTPClientWithToken(token.AccessToken)
}

// send issues a request with the client and returns the status and body.
func (ts *AgentSharedScopedTestSuite) send(client *http.Client, method, path string, body interface{}) (
	int, []byte) {
	ts.T().Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		ts.Require().NoError(err)
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, testServerURL+path, reader)
	ts.Require().NoError(err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	return resp.StatusCode, raw
}

// The unfiltered list of a confined administrator shows the agents shared into its unit, under its unit, as
// agents the unit does not own and cannot change. It leaves out an agent not shared there.
func (ts *AgentSharedScopedTestSuite) TestUnfilteredListShowsSharedAgents() {
	for name, client := range ts.admins {
		ts.Run(name, func() {
			status, body := ts.send(client, http.MethodGet, agentBasePath+"?limit=100", nil)
			ts.Require().Equal(http.StatusOK, status, string(body))
			var list struct {
				Agents []sharedScopedAgentView `json:"agents"`
			}
			ts.Require().NoError(json.Unmarshal(body, &list))

			listed := make(map[string]sharedScopedAgentView, len(list.Agents))
			sharedSeen := false
			for _, a := range list.Agents {
				listed[a.ID] = a
				ts.Require().NotNil(a.IsOwnedByOU, a.ID)
				if !*a.IsOwnedByOU {
					sharedSeen = true
				} else {
					ts.False(sharedSeen, "the owned agents are listed before the shared ones")
				}
			}
			for _, id := range []string{sharedDeclAgentID, sharedDeclClientAgentID} {
				a, ok := listed[id]
				ts.Require().True(ok, "%s is listed", id)
				ts.Equal(agentSharedIntoOUID, a.OUID, id)
				ts.False(*a.IsOwnedByOU, id)
				ts.True(a.IsReadOnly, id)
			}
			ts.NotContains(listed, unsharedDeclAgentID)
		})
	}
}

// A confined administrator reads an agent shared into its unit under its unit, as one it does not own and
// cannot change, and is refused an agent of the same owner that is not shared there.
func (ts *AgentSharedScopedTestSuite) TestGetSharedAgent() {
	for name, client := range ts.admins {
		ts.Run(name, func() {
			status, body := ts.send(client, http.MethodGet, agentBasePath+"/"+sharedDeclAgentID, nil)
			ts.Require().Equal(http.StatusOK, status, string(body))
			var a sharedScopedAgentView
			ts.Require().NoError(json.Unmarshal(body, &a))
			ts.Equal(sharedDeclAgentID, a.ID)
			ts.Equal(agentSharedIntoOUID, a.OUID, "a shared agent is read under the unit it is shared into")
			ts.Require().NotNil(a.IsOwnedByOU)
			ts.False(*a.IsOwnedByOU)
			ts.True(a.IsReadOnly)

			status, body = ts.send(client, http.MethodGet, agentBasePath+"/"+unsharedDeclAgentID, nil)
			ts.Equal(http.StatusForbidden, status, string(body))
		})
	}
}

// The root administrator reads the shared agent as owned, under the unit that owns it.
func (ts *AgentSharedScopedTestSuite) TestRootReadsSharedAgentAsOwned() {
	status, body := ts.send(testutils.GetHTTPClient(), http.MethodGet, agentBasePath+"/"+sharedDeclAgentID, nil)
	ts.Require().Equal(http.StatusOK, status, string(body))
	var a sharedScopedAgentView
	ts.Require().NoError(json.Unmarshal(body, &a))
	ts.Equal(agentOwnerOUID, a.OUID)
	ts.Require().NotNil(a.IsOwnedByOU)
	ts.True(*a.IsOwnedByOU)
}

// A confined administrator cannot change or delete an agent shared into its unit. One that may change agents is
// told the agent is read only; one that may only view them is refused before the agent is looked at.
func (ts *AgentSharedScopedTestSuite) TestSharedAgentIsReadOnly() {
	path := agentBasePath + "/" + sharedDeclAgentID
	update := map[string]interface{}{
		"type":       "declarative-agents",
		"ouId":       agentOwnerOUID,
		"attributes": map[string]interface{}{"name": "Declarative Test Agent Renamed"},
	}
	refusals := map[string]string{
		"org": sharedAgentReadOnlyCode, "org:agent": sharedAgentReadOnlyCode, "org:agent:view": "AUTH-4030",
	}
	for name, client := range ts.admins {
		ts.Run(name, func() {
			status, body := ts.send(client, http.MethodPut, path, update)
			ts.Require().Equal(http.StatusForbidden, status, string(body))
			ts.Equal(refusals[name], errorCodeOf(body))

			status, body = ts.send(client, http.MethodDelete, path, nil)
			ts.Require().Equal(http.StatusForbidden, status, string(body))
			ts.Equal(refusals[name], errorCodeOf(body))
		})
	}

	status, body := ts.send(testutils.GetHTTPClient(), http.MethodGet, path, nil)
	ts.Equal(http.StatusOK, status, "the agent survives the refused delete: %s", body)
}

// A confined administrator filtered to its own unit lists the agents shared into it, and filtered to any other
// unit, the owner's or one beneath its own, lists none.
func (ts *AgentSharedScopedTestSuite) TestFilteredListFollowsTheTokensUnit() {
	for name, client := range ts.admins {
		ts.Run(name, func() {
			listed := ts.listAgents(client, agentOUFilter(agentSharedIntoOUID))
			for _, id := range []string{sharedDeclAgentID, sharedDeclClientAgentID} {
				a, ok := listed[id]
				ts.Require().True(ok, "%s is listed", id)
				ts.Require().NotNil(a.IsOwnedByOU, id)
				ts.False(*a.IsOwnedByOU, id)
			}
			ts.NotContains(listed, unsharedDeclAgentID)

			for _, ouID := range []string{agentOwnerOUID, agentNotReachedOUID} {
				ts.Empty(ts.listAgents(client, agentOUFilter(ouID)), "filtered to %s", ouID)
			}
		})
	}
}

// A system token taken in a unit reaches every unit: without a filter it lists the agents of other units as their
// owners hold them, and a filter lists what the unit owns, then what is shared into it.
func (ts *AgentSharedScopedTestSuite) TestSystemTokenInAUnitListsEveryUnit() {
	every := ts.listAgents(ts.systemAdmin, "")
	for _, id := range []string{sharedDeclAgentID, sharedDeclClientAgentID, unsharedDeclAgentID} {
		a, ok := every[id]
		ts.Require().True(ok, "%s is listed", id)
		ts.Equal(agentOwnerOUID, a.OUID, id)
	}

	into := ts.listAgents(ts.systemAdmin, agentOUFilter(agentSharedIntoOUID))
	for _, id := range []string{sharedDeclAgentID, sharedDeclClientAgentID} {
		a, ok := into[id]
		ts.Require().True(ok, "%s is listed in the unit it is shared into", id)
		ts.Require().NotNil(a.IsOwnedByOU, id)
		ts.False(*a.IsOwnedByOU, id)
	}
	ts.NotContains(into, unsharedDeclAgentID)

	owner := ts.listAgents(ts.systemAdmin, agentOUFilter(agentOwnerOUID))
	for _, id := range []string{sharedDeclAgentID, sharedDeclClientAgentID, unsharedDeclAgentID} {
		a, ok := owner[id]
		ts.Require().True(ok, "%s is listed in its owner", id)
		ts.Require().NotNil(a.IsOwnedByOU, id)
		ts.True(*a.IsOwnedByOU, id)
	}
}

// listAgents reads every page of the agent list with the client, narrowed by query when it is set, and returns each
// listed agent by ID.
func (ts *AgentSharedScopedTestSuite) listAgents(client *http.Client, query string) map[string]sharedScopedAgentView {
	ts.T().Helper()
	listed := map[string]sharedScopedAgentView{}
	for offset := 0; ; offset += 100 {
		path := fmt.Sprintf("%s?limit=100&offset=%d", agentBasePath, offset)
		if query != "" {
			path += "&" + query
		}
		status, body := ts.send(client, http.MethodGet, path, nil)
		ts.Require().Equal(http.StatusOK, status, string(body))
		var page struct {
			TotalResults int                     `json:"totalResults"`
			Agents       []sharedScopedAgentView `json:"agents"`
		}
		ts.Require().NoError(json.Unmarshal(body, &page))
		for _, a := range page.Agents {
			listed[a.ID] = a
		}
		if len(page.Agents) < 100 || offset+100 >= page.TotalResults {
			return listed
		}
	}
}

// errorCodeOf returns the code of an error response body.
func errorCodeOf(body []byte) string {
	var e struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(body, &e)
	return e.Code
}
