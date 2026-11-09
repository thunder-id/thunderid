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
	ID        string `json:"id"`
	OUID      string `json:"ouId"`
	ProjectID string `json:"projectId"`
}

// projectAgentList is the part of an agent list response these tests read.
type projectAgentList struct {
	TotalResults int `json:"totalResults"`
	Agents       []struct {
		ID          string `json:"id"`
		ProjectID   string `json:"projectId"`
		IsOwnedByOU *bool  `json:"isOwnedByOU"`
	} `json:"agents"`
}

// Projects share the system organization unit, yet each lists only its own agents.
func (ts *ProjectAPITestSuite) TestCreateAgentInProject() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("agent-shop")))
	other := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Other"}`, ts.handle("agent-other")))

	inShop := ts.createAgent(ts.agentBody("shop", `"projectId":%q`, shop.ID))
	ts.Equal(defaultOUID, inShop.OUID)
	ts.Equal(shop.ID, inShop.ProjectID)
	inOther := ts.createAgent(ts.agentBody("other", `"projectId":%q`, other.ID))

	shopList := ts.listAgents("?limit=100&projectId=" + url.QueryEscape(shop.ID))
	ts.Equal([]string{inShop.ID}, ts.agentIDs(shopList))
	ts.Equal(shop.ID, shopList.Agents[0].ProjectID)
	ts.Equal([]string{inOther.ID}, ts.agentIDs(ts.listAgents("?limit=100&projectId="+url.QueryEscape(other.ID))))

	var fetched projectAgent
	ts.Require().Equal(http.StatusOK, ts.getJSON("/agents/"+inShop.ID, &fetched))
	ts.Equal(shop.ID, fetched.ProjectID)
}

// The Control Plane serves projects, so an agent must name one, even with an organization unit, whether it
// is created through the API, the onboard_agent tool or an import.
func (ts *ProjectAPITestSuite) TestCreateAgentRequiresProject() {
	status, body := ts.do(http.MethodPost, "/agents", ts.agentBody("unscoped", `"ouId":%q`, defaultOUID))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("AGT-1050", codeOf(ts.T(), body))

	onboarded, raw := ts.callMCPTool("onboard_agent", map[string]any{
		"name": ts.handle("agent-mcp-unscoped"), "ouId": defaultOUID,
	})
	ts.Truef(onboarded.Result.IsError, "an agent without a project was onboarded: %s", raw)
	ts.Contains(raw, "Set projectId")

	ts.Equal("AGT-1050", ts.importFailure(fmt.Sprintf("resource_type: agent\ntype: default\nouId: %s\n"+
		"attributes:\n  name: %s\n", defaultOUID, ts.handle("agent-import-unscoped")), false))
}

// An agent in a project is in the system organization unit, and no other.
func (ts *ProjectAPITestSuite) TestAgentInProjectOrganizationUnit() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("agent-ou-shop")))
	otherRoot := ts.createRootOU("agent-ou-other")
	childOUID := ts.createChildOU("agent-ou-child", defaultOUID)

	inDefault := ts.createAgent(ts.agentBody("ou-default", `"projectId":%q,"ouId":%q`, shop.ID, defaultOUID))
	ts.Equal(defaultOUID, inDefault.OUID)

	for name, ouID := range map[string]string{"ou-child": childOUID, "ou-outside": otherRoot} {
		status, body := ts.do(http.MethodPost, "/agents",
			ts.agentBody(name, `"projectId":%q,"ouId":%q`, shop.ID, ouID))
		ts.Equal(http.StatusBadRequest, status, body)
		ts.Equal("AGT-1048", codeOf(ts.T(), body), name)
	}

	ts.Equal([]string{inDefault.ID}, ts.agentIDs(ts.listAgents("?limit=100&projectId="+url.QueryEscape(shop.ID))))

	list := ts.listAgents("?projectId=" + url.QueryEscape(shop.ID) + "&filter=" +
		url.QueryEscape(fmt.Sprintf("ouId eq %q", otherRoot)))
	ts.Empty(list.Agents)
}

// An update keeps an agent in its project unless it names another, and moves it between projects.
func (ts *ProjectAPITestSuite) TestUpdateAgentWithProject() {
	first := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"First"}`, ts.handle("agent-upd-first")))
	second := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Second"}`, ts.handle("agent-upd-second")))
	otherRoot := ts.createRootOU("agent-upd-other")
	childOUID := ts.createChildOU("agent-upd-child", defaultOUID)
	agent := ts.createAgent(ts.agentBody("updated", `"projectId":%q`, first.ID))

	updated := ts.updateAgent(agent.ID, ts.agentBody("updated", `"description":%q`, "updated"))
	ts.Equal(defaultOUID, updated.OUID)
	ts.Equal(first.ID, updated.ProjectID, "an update without a project keeps it")

	updated = ts.updateAgent(agent.ID, ts.agentBody("updated", `"projectId":%q`, second.ID))
	ts.Equal(second.ID, updated.ProjectID)
	ts.Empty(ts.agentIDs(ts.listAgents("?limit=100&projectId=" + url.QueryEscape(first.ID))))
	ts.Equal([]string{agent.ID}, ts.agentIDs(ts.listAgents("?limit=100&projectId="+url.QueryEscape(second.ID))))

	for _, ouID := range []string{childOUID, otherRoot} {
		status, body := ts.do(http.MethodPut, "/agents/"+agent.ID, ts.agentBody("updated", `"ouId":%q`, ouID))
		ts.Equal(http.StatusBadRequest, status, body)
		ts.Equal("AGT-1048", codeOf(ts.T(), body))
	}
}

// A project is not deleted while an agent is still in it.
func (ts *ProjectAPITestSuite) TestDeleteProjectWithAgentIsRefused() {
	created := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Busy"}`, ts.handle("agent-busy")))
	agent := ts.createAgent(ts.agentBody("busy", `"projectId":%q`, created.ID))

	status, body := ts.do(http.MethodDelete, "/projects/"+created.ID, "")
	ts.Equal(http.StatusConflict, status, body)
	ts.Equal("PRJ-1013", codeOf(ts.T(), body))

	ts.Require().NoError(testutils.DeleteAgent(agent.ID))
	status, body = ts.do(http.MethodDelete, "/projects/"+created.ID, "")
	ts.Equal(http.StatusNoContent, status, body)
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

// An agent of a project is placed within its agent type's organization unit tree too. The agent takes the
// system organization unit, so an agent type of another tree is refused.
func (ts *ProjectAPITestSuite) TestAgentOfAgentTypeOutsideItsTreeRejected() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("agent-type-shop")))
	otherRoot := ts.createRootOU("agent-type-other")
	otherType := ts.handle("agent-type-other-agents")
	agentTypeID, err := testutils.CreateNewAgentType(testutils.UserType{
		Handle: otherType, DisplayName: "Other agents", OUID: otherRoot,
		Schema: map[string]interface{}{"name": map[string]interface{}{"type": "string", "required": true}},
	})
	ts.Require().NoError(err)
	// Removed before TearDownTest removes its organization unit.
	defer func() { _ = testutils.DeleteAgentType(agentTypeID) }()

	status, body := ts.do(http.MethodPost, "/agents", fmt.Sprintf(
		`{"type":%q,"attributes":{"name":%q},"projectId":%q}`, otherType, ts.handle("agent-foreign-type"), shop.ID))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("AGT-1053", codeOf(ts.T(), body))
}

// An agent of a project whose agent type does not exist is refused with the not-found code, as on /agents.
func (ts *ProjectAPITestSuite) TestAgentOfUnknownAgentTypeRejected() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("agent-type-unknown-shop")))

	status, body := ts.do(http.MethodPost, "/agents", fmt.Sprintf(
		`{"type":"no-such-agents","attributes":{"name":%q},"projectId":%q}`, ts.handle("agent-unknown-type"), shop.ID))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("AGT-1054", codeOf(ts.T(), body))
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
