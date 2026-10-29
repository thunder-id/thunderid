// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// AgentOUFilterTestSuite exercises listing the agents of one organization unit. Agents cannot be
// shared yet, so an organization unit lists only the agents it owns.
//
// Fixture topology:
//
//	parent (agent-ou-filter-parent)    ← owns three agents
//	└── child (agent-ou-filter-child)  ← owns one agent
//	empty (agent-ou-filter-empty)      ← owns no agents
type AgentOUFilterTestSuite struct {
	suite.Suite

	parentOUID string
	childOUID  string
	emptyOUID  string

	// owned holds, per organization unit, the IDs of the agents it owns, sorted.
	owned map[string][]string
}

// ouListedAgent is the part of a listed agent these tests read.
type ouListedAgent struct {
	ID          string `json:"id"`
	OUID        string `json:"ouId"`
	IsOwnedByOU *bool  `json:"isOwnedByOU"`
}

// ouAgentList is the part of an agent list response these tests read.
type ouAgentList struct {
	TotalResults int             `json:"totalResults"`
	Count        int             `json:"count"`
	Agents       []ouListedAgent `json:"agents"`
	Links        []struct {
		Href string `json:"href"`
		Rel  string `json:"rel"`
	} `json:"links"`
}

func TestAgentOUFilterTestSuite(t *testing.T) {
	suite.Run(t, new(AgentOUFilterTestSuite))
}

func (ts *AgentOUFilterTestSuite) SetupSuite() {
	ts.owned = map[string][]string{}

	ts.parentOUID = ts.createOU("agent-ou-filter-parent", nil)
	ts.childOUID = ts.createOU("agent-ou-filter-child", &ts.parentOUID)
	ts.emptyOUID = ts.createOU("agent-ou-filter-empty", nil)

	for _, name := range []string{"parent-1", "parent-2", "parent-3"} {
		ts.createAgent(name, ts.parentOUID)
	}
	ts.createAgent("child-1", ts.childOUID)
}

func (ts *AgentOUFilterTestSuite) TearDownSuite() {
	for _, ids := range ts.owned {
		for _, id := range ids {
			if err := testutils.DeleteAgent(id); err != nil {
				ts.T().Logf("teardown: delete agent %s: %v", id, err)
			}
		}
	}
	for _, id := range []string{ts.childOUID, ts.parentOUID, ts.emptyOUID} {
		if id == "" {
			continue
		}
		if err := testutils.DeleteOrganizationUnit(id); err != nil {
			ts.T().Logf("teardown: delete OU %s: %v", id, err)
		}
	}
}

// createOU creates an organization unit and returns its ID.
func (ts *AgentOUFilterTestSuite) createOU(handle string, parent *string) string {
	ts.T().Helper()
	id, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: handle, Name: handle, Parent: parent,
	})
	ts.Require().NoError(err, "create OU %s", handle)
	return id
}

// createAgent creates an agent in an organization unit and records it as owned there.
func (ts *AgentOUFilterTestSuite) createAgent(name, ouID string) {
	ts.T().Helper()
	id, err := testutils.CreateAgent(testutils.Agent{
		Type:       agentTypeName,
		OUID:       ouID,
		Attributes: map[string]interface{}{"name": "agent-ou-filter-" + name},
	})
	ts.Require().NoError(err, "create agent %s", name)
	ts.owned[ouID] = append(ts.owned[ouID], id)
	slices.Sort(ts.owned[ouID])
}

// request sends a GET request and returns its status and body.
func (ts *AgentOUFilterTestSuite) request(pathAndQuery string) (int, []byte) {
	ts.T().Helper()
	req, err := http.NewRequest(http.MethodGet, testServerURL+pathAndQuery, nil)
	ts.Require().NoError(err)
	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	return resp.StatusCode, raw
}

// list reads an agent list.
func (ts *AgentOUFilterTestSuite) list(pathAndQuery string) ouAgentList {
	ts.T().Helper()
	status, raw := ts.request(pathAndQuery)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	var list ouAgentList
	ts.Require().NoError(json.Unmarshal(raw, &list))
	return list
}

// agentOUFilter returns the query string that lists one organization unit's agents.
func agentOUFilter(ouID string) string {
	return "filter=" + url.QueryEscape(fmt.Sprintf("ouId eq %q", ouID))
}

// Every unit lists exactly the agents it owns, each marked owned and carrying the unit's ID. A
// parent does not list its child's agents.
func (ts *AgentOUFilterTestSuite) TestEveryUnitListsItsOwnedAgents() {
	units := []struct {
		name string
		id   string
	}{
		{"Parent", ts.parentOUID},
		{"Child", ts.childOUID},
		{"Empty", ts.emptyOUID},
	}
	for _, unit := range units {
		ts.Run(unit.name, func() {
			want := ts.owned[unit.id]

			list := ts.list(agentBasePath + "?" + agentOUFilter(unit.id))

			ts.Equal(len(want), list.TotalResults)
			ts.Equal(len(list.Agents), list.Count)
			got := make([]string, len(list.Agents))
			for i, agent := range list.Agents {
				got[i] = agent.ID
				ts.Equal(unit.id, agent.OUID, "agent %s", agent.ID)
				ts.Require().NotNil(agent.IsOwnedByOU, "agent %s carries no isOwnedByOU field", agent.ID)
				ts.True(*agent.IsOwnedByOU, "agent %s", agent.ID)
			}
			ts.ElementsMatch(want, got)
		})
	}
}

// Paging through a unit's agents visits each once, and the links keep the filter.
func (ts *AgentOUFilterTestSuite) TestPagesKeepTheFilter() {
	filter := agentOUFilter(ts.parentOUID)

	first := ts.list(agentBasePath + "?limit=2&offset=0&" + filter)
	ts.Equal(3, first.TotalResults)
	ts.Require().Len(first.Agents, 2)

	var next string
	for _, link := range first.Links {
		if link.Rel == "next" {
			next = link.Href
		}
	}
	ts.Require().NotEmpty(next, "the first page links no next page")
	ts.Contains(next, filter)

	second := ts.list(agentBasePath + "?limit=2&offset=2&" + filter)
	ts.Equal(3, second.TotalResults)
	ts.Require().Len(second.Agents, 1)

	got := []string{first.Agents[0].ID, first.Agents[1].ID, second.Agents[0].ID}
	slices.Sort(got)
	ts.Equal(ts.owned[ts.parentOUID], got)
}

// An unfiltered list marks every agent owned.
func (ts *AgentOUFilterTestSuite) TestUnfilteredListMarksAgentsOwned() {
	list := ts.list(agentBasePath + "?limit=100")

	ts.Require().NotEmpty(list.Agents)
	for _, agent := range list.Agents {
		ts.Require().NotNil(agent.IsOwnedByOU, "agent %s", agent.ID)
		ts.True(*agent.IsOwnedByOU, "agent %s", agent.ID)
	}
}

// Filtering on a unit that does not exist lists no agents rather than failing.
func (ts *AgentOUFilterTestSuite) TestFilterOnAnUnknownUnitListsNothing() {
	list := ts.list(agentBasePath + "?" + agentOUFilter("00000000-0000-0000-0000-000000000000"))

	ts.Zero(list.TotalResults)
	ts.Empty(list.Agents)
}

// A unit filter takes a quoted, non-empty ID and no other condition.
func (ts *AgentOUFilterTestSuite) TestAMalformedUnitFilterIsRefused() {
	for name, expression := range map[string]string{
		"an unquoted ID": "ouId eq " + ts.childOUID,
		"an empty ID":    `ouId eq ""`,
		"no value":       "ouId eq",
	} {
		ts.Run(name, func() {
			status, raw := ts.request(agentBasePath + "?filter=" + url.QueryEscape(expression))

			ts.Require().Equal(http.StatusBadRequest, status, string(raw))
			var e ErrorResponse
			ts.Require().NoError(json.Unmarshal(raw, &e))
			ts.Equal("AGT-1020", e.Code)
		})
	}
}
