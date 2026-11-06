// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// projectFlow is the part of a flow create or update response these tests read.
type projectFlow struct {
	ID        string `json:"id"`
	OUID      string `json:"ouId"`
	ProjectID string `json:"projectId"`
}

// projectFlowList is the part of a flow list response these tests read.
type projectFlowList struct {
	TotalResults int `json:"totalResults"`
	Flows        []struct {
		ID        string `json:"id"`
		ProjectID string `json:"projectId"`
	} `json:"flows"`
}

// Projects share the system organization unit, yet each lists only its own flows.
func (ts *ProjectAPITestSuite) TestCreateFlowInProject() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("flow-shop")))
	other := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Other"}`, ts.handle("flow-other")))

	inShop := ts.createFlow(ts.flowBody("shop", `"projectId":%q`, shop.ID))
	ts.Equal(defaultOUID, inShop.OUID)
	ts.Equal(shop.ID, inShop.ProjectID)
	inOther := ts.createFlow(ts.flowBody("other", `"projectId":%q`, other.ID))
	ts.createFlow(ts.flowBody("unscoped", `"ouId":%q`, defaultOUID))

	shopList := ts.listFlows("?limit=100&projectId=" + url.QueryEscape(shop.ID))
	ts.Equal([]string{inShop.ID}, ts.flowIDs(shopList))
	ts.Equal(shop.ID, shopList.Flows[0].ProjectID)
	ts.Equal([]string{inOther.ID}, ts.flowIDs(ts.listFlows("?limit=100&projectId="+url.QueryEscape(other.ID))))

	ts.Equal([]string{inShop.ID},
		ts.flowIDs(ts.listFlows("?flowType=AUTHENTICATION&projectId="+url.QueryEscape(shop.ID))))
	ts.Empty(ts.flowIDs(ts.listFlows("?flowType=REGISTRATION&projectId=" + url.QueryEscape(shop.ID))))

	var fetched projectFlow
	ts.Require().Equal(http.StatusOK, ts.getJSON("/flows/"+inShop.ID, &fetched))
	ts.Equal(shop.ID, fetched.ProjectID)
}

// A flow in a project is in the system organization unit, and no other.
func (ts *ProjectAPITestSuite) TestFlowInProjectOrganizationUnit() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("flow-ou-shop")))
	otherRoot := ts.createRootOU("flow-ou-other")
	childOUID := ts.createChildOU("flow-ou-child", defaultOUID)

	inDefault := ts.createFlow(ts.flowBody("ou-default", `"projectId":%q,"ouId":%q`, shop.ID, defaultOUID))
	ts.Equal(defaultOUID, inDefault.OUID)

	for name, ouID := range map[string]string{"ou-child": childOUID, "ou-outside": otherRoot} {
		status, body := ts.do(http.MethodPost, "/flows",
			ts.flowBody(name, `"projectId":%q,"ouId":%q`, shop.ID, ouID))
		ts.Equal(http.StatusBadRequest, status, body)
		ts.Equal("FLM-1046", codeOf(ts.T(), body), name)
	}

	ts.Equal([]string{inDefault.ID}, ts.flowIDs(ts.listFlows("?limit=100&projectId="+url.QueryEscape(shop.ID))))

	list := ts.listFlows("?projectId=" + url.QueryEscape(shop.ID) + "&filter=" +
		url.QueryEscape(fmt.Sprintf("ouId eq %q", otherRoot)))
	ts.Empty(list.Flows)
}

// An update keeps a flow in its project unless it names another one, and moves it between projects.
func (ts *ProjectAPITestSuite) TestUpdateFlowWithProject() {
	first := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"First"}`, ts.handle("flow-upd-first")))
	second := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Second"}`, ts.handle("flow-upd-second")))
	flow := ts.createFlow(ts.flowBody("updated", `"projectId":%q`, first.ID))

	updated := ts.updateFlow(flow.ID, ts.flowBody("updated", `"ouId":%q`, defaultOUID))
	ts.Equal(first.ID, updated.ProjectID, "an update without a project keeps it")

	updated = ts.updateFlow(flow.ID, ts.flowBody("updated", `"projectId":%q`, second.ID))
	ts.Equal(second.ID, updated.ProjectID)
	ts.Empty(ts.flowIDs(ts.listFlows("?limit=100&projectId=" + url.QueryEscape(first.ID))))
	ts.Equal([]string{flow.ID}, ts.flowIDs(ts.listFlows("?limit=100&projectId="+url.QueryEscape(second.ID))))
}

func (ts *ProjectAPITestSuite) TestFlowWithUnknownProject() {
	const unknownProjectID = "019a0000-0000-7000-8000-00000000dead"

	status, body := ts.do(http.MethodPost, "/flows", ts.flowBody("unknown", `"projectId":%q`, unknownProjectID))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("FLM-1045", codeOf(ts.T(), body))

	list := ts.listFlows("?projectId=" + unknownProjectID)
	ts.Zero(list.TotalResults)
	ts.Empty(list.Flows)

	flow := ts.createFlow(ts.flowBody("unknown-upd", `"ouId":%q`, defaultOUID))
	status, body = ts.do(http.MethodPut, "/flows/"+flow.ID,
		ts.flowBody("unknown-upd", `"projectId":%q`, unknownProjectID))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("FLM-1045", codeOf(ts.T(), body))
}

// A flow needs an organization unit or a project.
func (ts *ProjectAPITestSuite) TestFlowWithoutPlacement() {
	nodes, err := json.Marshal(flowNodes())
	ts.Require().NoError(err)
	status, body := ts.do(http.MethodPost, "/flows", fmt.Sprintf(
		`{"handle":%q,"name":"Unplaced","flowType":"AUTHENTICATION","nodes":%s}`, ts.handle("flow-unplaced"), nodes))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("FLM-1029", codeOf(ts.T(), body))
}

// The flow tools take a project: create_flow places the flow in it, and list_flows lists only its flows.
func (ts *ProjectAPITestSuite) TestFlowMCPToolsWithProject() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("flow-mcp-shop")))

	created, raw := ts.callMCPTool("create_flow", map[string]any{
		"handle":    ts.handle("flow-mcp"),
		"name":      "MCP Project Flow",
		"flowType":  "AUTHENTICATION",
		"projectId": shop.ID,
		"nodes":     flowNodes(),
	})
	ts.Require().Falsef(created.Result.IsError, "create failed: %s", raw)
	id, _ := created.Result.StructuredContent["id"].(string)
	ts.Require().NotEmpty(id)
	ts.flows = append(ts.flows, id)
	ts.Equal(shop.ID, created.Result.StructuredContent["projectId"])
	ts.Equal(defaultOUID, created.Result.StructuredContent["ouId"])

	listed, raw := ts.callMCPTool("list_flows", map[string]any{"project_id": shop.ID})
	ts.Require().Falsef(listed.Result.IsError, "list failed: %s", raw)
	ts.EqualValues(1, listed.Result.StructuredContent["total_count"])
	ts.Contains(raw, id)

	updated, raw := ts.callMCPTool("update_flow", map[string]any{
		"id":    id,
		"name":  "MCP Project Flow Renamed",
		"nodes": flowNodes(),
	})
	ts.Require().Falsef(updated.Result.IsError, "update failed: %s", raw)
	ts.Equal(shop.ID, updated.Result.StructuredContent["projectId"], "update_flow keeps the project")
}

// flowNodes returns the nodes of the smallest valid login flow.
func flowNodes() []map[string]any {
	return []map[string]any{
		{"id": "start", "type": "START", "onSuccess": "auth_assert"},
		{"id": "auth_assert", "type": "TASK_EXECUTION", "executor": map[string]any{"name": "AuthAssertExecutor"},
			"onSuccess": "end"},
		{"id": "end", "type": "END"},
	}
}

// flowBody builds a login flow request, with the given placement fields spliced in.
func (ts *ProjectAPITestSuite) flowBody(name, placement string, args ...interface{}) string {
	nodes, err := json.Marshal(flowNodes())
	ts.Require().NoError(err)
	return fmt.Sprintf(`{"handle":%q,"name":%q,"flowType":"AUTHENTICATION","nodes":%s,%s}`,
		ts.handle("flow-"+name), "Flow "+name, nodes, fmt.Sprintf(placement, args...))
}

// createFlow posts a flow, requires a 201, and records it for cleanup.
func (ts *ProjectAPITestSuite) createFlow(body string) projectFlow {
	status, resp := ts.do(http.MethodPost, "/flows", body)
	ts.Require().Equal(http.StatusCreated, status, resp)
	var created projectFlow
	ts.Require().NoError(json.Unmarshal([]byte(resp), &created))
	ts.flows = append(ts.flows, created.ID)
	return created
}

// updateFlow puts a flow and requires a 200.
func (ts *ProjectAPITestSuite) updateFlow(id, body string) projectFlow {
	status, resp := ts.do(http.MethodPut, "/flows/"+id, body)
	ts.Require().Equal(http.StatusOK, status, resp)
	var updated projectFlow
	ts.Require().NoError(json.Unmarshal([]byte(resp), &updated))
	return updated
}

// listFlows lists flows with the given query and requires a 200.
func (ts *ProjectAPITestSuite) listFlows(query string) projectFlowList {
	var list projectFlowList
	ts.Require().Equal(http.StatusOK, ts.getJSON("/flows"+query, &list))
	return list
}

// flowIDs returns the IDs of the listed flows.
func (ts *ProjectAPITestSuite) flowIDs(list projectFlowList) []string {
	ids := []string{}
	for _, flow := range list.Flows {
		ids = append(ids, flow.ID)
	}
	return ids
}
