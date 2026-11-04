// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// projectAgent is the part of an agent create or update response these tests read.
type projectAgent struct {
	ID   string `json:"id"`
	OUID string `json:"ouId"`
}

// projectAgentList is the part of an agent list response these tests read.
type projectAgentList struct {
	TotalResults int `json:"totalResults"`
	Agents       []struct {
		ID          string `json:"id"`
		IsOwnedByOU *bool  `json:"isOwnedByOU"`
	} `json:"agents"`
}

func (ts *ProjectAPITestSuite) TestCreateAgentInProject() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("agent-shop")))
	otherRoot := ts.createRootOU("agent-other")

	inShop := ts.createAgent(ts.agentBody("shop", `"projectId":%q`, shop.ID))
	ts.Equal(defaultOUID, inShop.OUID)
	inOther := ts.createAgent(ts.agentBody("other", `"ouId":%q`, otherRoot))

	ids := ts.agentIDs(ts.listAgents("?limit=100&projectId=" + url.QueryEscape(shop.ID)))
	ts.Contains(ids, inShop.ID)
	ts.NotContains(ids, inOther.ID)
}

func (ts *ProjectAPITestSuite) TestAgentInProjectOrganizationUnit() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("agent-ou-shop")))
	otherRoot := ts.createRootOU("agent-ou-other")
	childOUID := ts.createChildOU("agent-ou-child", defaultOUID)

	inDefault := ts.createAgent(ts.agentBody("ou-default", `"projectId":%q,"ouId":%q`, shop.ID, defaultOUID))
	ts.Equal(defaultOUID, inDefault.OUID)
	inChild := ts.createAgent(ts.agentBody("ou-child", `"projectId":%q,"ouId":%q`, shop.ID, childOUID))
	ts.Equal(childOUID, inChild.OUID)

	status, body := ts.do(http.MethodPost, "/agents",
		ts.agentBody("ou-outside", `"projectId":%q,"ouId":%q`, shop.ID, otherRoot))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("AGT-1048", codeOf(ts.T(), body))

	ids := ts.agentIDs(ts.listAgents("?limit=100&projectId=" + url.QueryEscape(shop.ID)))
	ts.Contains(ids, inDefault.ID)
	ts.NotContains(ids, inChild.ID)

	list := ts.listAgents("?projectId=" + url.QueryEscape(shop.ID) + "&filter=" +
		url.QueryEscape(fmt.Sprintf("ouId eq %q", childOUID)))
	ts.Equal([]string{inChild.ID}, ts.agentIDs(list))

	list = ts.listAgents("?projectId=" + url.QueryEscape(shop.ID) + "&filter=" +
		url.QueryEscape(fmt.Sprintf("ouId eq %q", otherRoot)))
	ts.Empty(list.Agents)
}

func (ts *ProjectAPITestSuite) TestUpdateAgentWithProject() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("agent-upd-shop")))
	otherRoot := ts.createRootOU("agent-upd-other")
	childOUID := ts.createChildOU("agent-upd-child", defaultOUID)
	agent := ts.createAgent(ts.agentBody("updated", `"projectId":%q`, shop.ID))

	updated := ts.updateAgent(agent.ID, ts.agentBody("updated", `"projectId":%q`, shop.ID))
	ts.Equal(defaultOUID, updated.OUID)

	updated = ts.updateAgent(agent.ID, ts.agentBody("updated", `"projectId":%q,"ouId":%q`, shop.ID, childOUID))
	ts.Equal(childOUID, updated.OUID)

	status, body := ts.do(http.MethodPut, "/agents/"+agent.ID,
		ts.agentBody("updated", `"projectId":%q,"ouId":%q`, shop.ID, otherRoot))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("AGT-1048", codeOf(ts.T(), body))
}

func (ts *ProjectAPITestSuite) TestAgentWithUnknownProject() {
	const unknownProjectID = "019a0000-0000-7000-8000-00000000dead"

	status, body := ts.do(http.MethodPost, "/agents",
		ts.agentBody("unknown", `"projectId":%q`, unknownProjectID))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("AGT-1047", codeOf(ts.T(), body))

	list := ts.listAgents("?projectId=" + unknownProjectID)
	ts.Zero(list.TotalResults)
	ts.Empty(list.Agents)
}

// agentBody builds an agent request of the default type, with the given placement fields spliced in.
func (ts *ProjectAPITestSuite) agentBody(name, placement string, args ...interface{}) string {
	return fmt.Sprintf(`{"type":"default","attributes":{"name":%q},%s}`,
		ts.handle("agent-"+name), fmt.Sprintf(placement, args...))
}

// createAgent posts an agent, requires a 201, and records it for cleanup.
func (ts *ProjectAPITestSuite) createAgent(body string) projectAgent {
	status, resp := ts.do(http.MethodPost, "/agents", body)
	ts.Require().Equal(http.StatusCreated, status, resp)
	var created projectAgent
	ts.Require().NoError(json.Unmarshal([]byte(resp), &created))
	ts.agents = append(ts.agents, created.ID)
	return created
}

// updateAgent puts an agent and requires a 200.
func (ts *ProjectAPITestSuite) updateAgent(id, body string) projectAgent {
	status, resp := ts.do(http.MethodPut, "/agents/"+id, body)
	ts.Require().Equal(http.StatusOK, status, resp)
	var updated projectAgent
	ts.Require().NoError(json.Unmarshal([]byte(resp), &updated))
	return updated
}

// listAgents lists agents with the given query and requires a 200.
func (ts *ProjectAPITestSuite) listAgents(query string) projectAgentList {
	var list projectAgentList
	ts.Require().Equal(http.StatusOK, ts.getJSON("/agents"+query, &list))
	return list
}

// agentIDs returns the IDs of the listed agents.
func (ts *ProjectAPITestSuite) agentIDs(list projectAgentList) []string {
	ids := []string{}
	for _, agent := range list.Agents {
		ids = append(ids, agent.ID)
	}
	return ids
}
