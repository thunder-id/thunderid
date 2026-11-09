// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ouresource

import (
	"encoding/json"
	"net/http"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const sharedFlowReadOnlyCode = "FLM-1039"

// sharedFlowView is the part of a flow response that says which organization unit it is read as.
type sharedFlowView struct {
	ID          string `json:"id"`
	OUID        string `json:"ouId"`
	IsOwnedByOU *bool  `json:"isOwnedByOU"`
	IsReadOnly  bool   `json:"isReadOnly"`
	Code        string `json:"code"`
}

func (ts *OUResourceTestSuite) TestFlowSharedFromParent() {
	flow := ouResourceFlow("ou-resource-parent-shared-flow", ts.parentID)
	flowID, err := testutils.CreateFlow(flow)
	ts.Require().NoError(err)
	ts.cleanup("/flows/" + flowID)
	status, created := ts.root(http.MethodPost, "/flows/"+flowID+"/sharing-policies",
		map[string]interface{}{"childUnits": map[string]interface{}{"mode": "all"}})
	ts.Require().Equalf(http.StatusCreated, status, "share the flow: %s", created)
	ts.cleanup("/flows/" + flowID + "/sharing-policies/" + idOf(created))

	body := ts.requireStatus(http.StatusOK, http.MethodGet, "/flows?limit=100", nil)
	var list struct {
		Flows []sharedFlowView `json:"flows"`
	}
	ts.Require().NoError(json.Unmarshal(body, &list))
	var listed *sharedFlowView
	for i := range list.Flows {
		if list.Flows[i].ID == flowID {
			listed = &list.Flows[i]
		}
	}
	ts.Require().NotNil(listed, "the list must include the flow shared into the administrator's OU")
	ts.Require().NotNil(listed.IsOwnedByOU)
	ts.False(*listed.IsOwnedByOU)

	body = ts.requireStatus(http.StatusOK, http.MethodGet, "/flows/"+flowID, nil)
	var read sharedFlowView
	ts.Require().NoError(json.Unmarshal(body, &read))
	ts.Equal(flowID, read.ID)
	ts.Equal(ts.ou1ID, read.OUID, "a shared flow is read under the unit it is shared into")
	ts.Require().NotNil(read.IsOwnedByOU)
	ts.False(*read.IsOwnedByOU)
	ts.True(read.IsReadOnly)

	// The unit it is shared into sees the flow's identity and variables, never its definition, by any route.
	byOU := ts.requireStatus(http.StatusOK, http.MethodGet, "/flows/"+flowID+"?ouId="+ts.ou1ID, nil)
	for _, shared := range [][]byte{body, byOU} {
		var fields map[string]json.RawMessage
		ts.Require().NoError(json.Unmarshal(shared, &fields))
		for _, key := range []string{"nodes", "interceptors", "activeVersion", "createdAt", "updatedAt", "projectId"} {
			ts.NotContainsf(fields, key, "a shared flow is read without its definition: %s", shared)
		}
		for _, key := range []string{"id", "handle", "name", "flowType", "ouId", "isOwnedByOU", "isReadOnly",
			"variables"} {
			ts.Containsf(fields, key, "a shared flow is read with its identity and variables: %s", shared)
		}
	}
	ts.requireStatus(http.StatusForbidden, http.MethodGet, "/flows/"+flowID+"/versions", nil)
	ts.requireStatus(http.StatusForbidden, http.MethodGet, "/flows/"+flowID+"/versions/1", nil)
	ts.requireStatus(http.StatusNotFound, http.MethodGet, "/flows/"+flowID+"/versions?ouId="+ts.ou1ID, nil)
	ts.requireStatus(http.StatusNotFound, http.MethodGet, "/flows/"+flowID+"/versions/1?ouId="+ts.ou1ID, nil)

	var refused sharedFlowView
	body = ts.requireStatus(http.StatusForbidden, http.MethodPut, "/flows/"+flowID, flow)
	ts.Require().NoError(json.Unmarshal(body, &refused))
	ts.Equal(sharedFlowReadOnlyCode, refused.Code)
	body = ts.requireStatus(http.StatusForbidden, http.MethodDelete, "/flows/"+flowID, nil)
	ts.Require().NoError(json.Unmarshal(body, &refused))
	ts.Equal(sharedFlowReadOnlyCode, refused.Code)

	status, body = ts.root(http.MethodGet, "/flows/"+flowID, nil)
	ts.Require().Equalf(http.StatusOK, status, "the flow must survive the refused delete: %s", body)
	ts.Require().NoError(json.Unmarshal(body, &read))
	ts.Equal(ts.parentID, read.OUID)
	ts.Require().NotNil(read.IsOwnedByOU)
	ts.True(*read.IsOwnedByOU)
}

func (ts *OUResourceTestSuite) TestFlowOwnedReadsAsOwned() {
	own, err := testutils.CreateFlow(ouResourceFlow("ou-resource-owned-read-flow", ts.ou1ID))
	ts.Require().NoError(err)
	ts.cleanup("/flows/" + own)

	body := ts.requireStatus(http.StatusOK, http.MethodGet, "/flows/"+own, nil)
	var read sharedFlowView
	ts.Require().NoError(json.Unmarshal(body, &read))
	ts.Equal(ts.ou1ID, read.OUID)
	ts.Require().NotNil(read.IsOwnedByOU)
	ts.True(*read.IsOwnedByOU)
}

// A flow shared into the administrator's OU stays read only, but the OU sets the variables the policy lets it set,
// for itself only.
func (ts *OUResourceTestSuite) TestSharedFlowVariablesAreSetInTheUnit() {
	flow := ouResourceFlow("ou-resource-variables-flow", ts.parentID)
	flow.Nodes = []map[string]interface{}{
		{"id": "start", "type": "START", "onSuccess": "resolve_ou"},
		{"id": "resolve_ou", "type": "TASK_EXECUTION", "onSuccess": "vars",
			"executor":   map[string]interface{}{"name": "OUResolverExecutor"},
			"properties": map[string]interface{}{"resolveFrom": "prompt", "parentOuId": ts.parentID}},
		{"id": "vars", "type": "VARIABLES", "onSuccess": "auth_assert", "inputs": []map[string]interface{}{
			{"identifier": "supportEmail", "type": "EMAIL_INPUT", "label": "Support email",
				"defaultValue": "support@example.com"},
		}},
		{"id": "auth_assert", "type": "TASK_EXECUTION",
			"executor": map[string]interface{}{"name": "AuthAssertExecutor"}, "onSuccess": "end"},
		{"id": "end", "type": "END"},
	}
	flowID, err := testutils.CreateFlow(flow)
	ts.Require().NoError(err)
	ts.cleanup("/flows/" + flowID)
	status, created := ts.root(http.MethodPost, "/flows/"+flowID+"/sharing-policies", map[string]interface{}{
		"childUnits": map[string]interface{}{
			"mode":         "all",
			"overlayRules": map[string]interface{}{"variables.supportEmail": map[string]interface{}{"editable": true}},
		},
	})
	ts.Require().Equalf(http.StatusCreated, status, "share the flow: %s", created)
	ts.cleanup("/flows/" + flowID + "/sharing-policies/" + idOf(created))

	variables := "/flows/" + flowID + "/variables"
	set := func(ouID string) map[string]interface{} {
		return map[string]interface{}{"ouId": ouID, "values": map[string]interface{}{"supportEmail": "help@ou1.example"}}
	}
	ts.requireStatus(http.StatusOK, http.MethodPut, variables, set(ts.ou1ID))
	body := ts.requireStatus(http.StatusOK, http.MethodGet, testutils.WithOU(variables, ts.ou1ID), nil)
	var read struct {
		Values map[string]interface{} `json:"values"`
	}
	ts.Require().NoError(json.Unmarshal(body, &read), string(body))
	ts.Equal(map[string]interface{}{"supportEmail": "help@ou1.example"}, read.Values)

	ts.requireStatus(http.StatusForbidden, http.MethodPut, variables, set(ts.parentID))
}
