// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
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
	other := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Other"}`, ts.handle("agent-other")))

	inShop := ts.createAgent(ts.agentBody("shop", `"projectId":%q`, shop.ID))
	ts.Equal(shop.RootOUID, inShop.OUID)
	inOther := ts.createAgent(ts.agentBody("other", `"ouId":%q`, other.RootOUID))

	list := ts.listAgents("?projectId=" + url.QueryEscape(shop.ID))

	ts.Equal(1, list.TotalResults)
	ts.Require().Len(list.Agents, 1)
	ts.Equal(inShop.ID, list.Agents[0].ID)
	ts.Equal(new(true), list.Agents[0].IsOwnedByOU)

	list = ts.listAgents("?projectId=" + url.QueryEscape(other.ID))
	ts.Require().Len(list.Agents, 1)
	ts.Equal(inOther.ID, list.Agents[0].ID)
}

func (ts *ProjectAPITestSuite) TestAgentInProjectOrganizationUnit() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("agent-ou-shop")))
	other := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Other"}`, ts.handle("agent-ou-other")))
	childOUID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: ts.handle("agent-ou-child"), Name: "Child", Parent: &shop.RootOUID,
	})
	ts.Require().NoError(err)
	ts.childOUs = append(ts.childOUs, childOUID)

	inRoot := ts.createAgent(ts.agentBody("ou-root", `"projectId":%q,"ouId":%q`, shop.ID, shop.RootOUID))
	ts.Equal(shop.RootOUID, inRoot.OUID)
	inChild := ts.createAgent(ts.agentBody("ou-child", `"projectId":%q,"ouId":%q`, shop.ID, childOUID))
	ts.Equal(childOUID, inChild.OUID)

	status, body := ts.do(http.MethodPost, "/agents",
		ts.agentBody("ou-outside", `"projectId":%q,"ouId":%q`, shop.ID, other.RootOUID))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("AGT-1048", codeOf(ts.T(), body))

	list := ts.listAgents("?projectId=" + url.QueryEscape(shop.ID))
	ts.Require().Len(list.Agents, 1)
	ts.Equal(inRoot.ID, list.Agents[0].ID)

	list = ts.listAgents("?projectId=" + url.QueryEscape(shop.ID) + "&filter=" +
		url.QueryEscape(fmt.Sprintf("ouId eq %q", childOUID)))
	ts.Require().Len(list.Agents, 1)
	ts.Equal(inChild.ID, list.Agents[0].ID)

	list = ts.listAgents("?projectId=" + url.QueryEscape(shop.ID) + "&filter=" +
		url.QueryEscape(fmt.Sprintf("ouId eq %q", other.RootOUID)))
	ts.Empty(list.Agents)
}

func (ts *ProjectAPITestSuite) TestUpdateAgentWithProject() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("agent-upd-shop")))
	other := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Other"}`, ts.handle("agent-upd-other")))
	childOUID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: ts.handle("agent-upd-child"), Name: "Child", Parent: &shop.RootOUID,
	})
	ts.Require().NoError(err)
	ts.childOUs = append(ts.childOUs, childOUID)
	agent := ts.createAgent(ts.agentBody("updated", `"projectId":%q`, shop.ID))

	updated := ts.updateAgent(agent.ID, ts.agentBody("updated", `"projectId":%q`, shop.ID))
	ts.Equal(shop.RootOUID, updated.OUID)

	updated = ts.updateAgent(agent.ID, ts.agentBody("updated", `"projectId":%q,"ouId":%q`, shop.ID, childOUID))
	ts.Equal(childOUID, updated.OUID)

	status, body := ts.do(http.MethodPut, "/agents/"+agent.ID,
		ts.agentBody("updated", `"projectId":%q`, other.ID))
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
