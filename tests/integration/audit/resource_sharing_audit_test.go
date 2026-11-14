// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	// auditVariableValue is the value a unit sets for a flow variable. No line may carry it.
	auditVariableValue = "audit-variable-value@audit-variables.test"
	// auditVariableName is a variable the flows of these tests declare. No line may carry it.
	auditVariableName = "auditSupportEmail"
)

// Creating, updating and deleting a sharing policy of a role, a flow and a connection writes one line
// each, with the policy as target and the role, flow or connection as subject. A notification sender is
// shared through the connections API, so its lines are connection events with the sender as subject. A
// delete of a policy the resource does not hold still succeeds and writes no line. No organization unit
// the policy names appears in any line.
func (ts *AuditLogTestSuite) TestARoleFlowAndConnectionSharingPolicyLifecycleWritesOneLinePerChange() {
	cases := []struct {
		name         string
		resourceType string
		subjectType  string
		create       func(ouID string) string
		base         func(id string) string
	}{
		{
			name: "role", resourceType: "role", subjectType: "role",
			create: func(ouID string) string {
				return ts.createRoleIn(ouID)
			},
			base: func(id string) string { return "/roles/" + id + "/sharing-policies" },
		},
		{
			name: "flow", resourceType: "flow", subjectType: "flow",
			create: func(ouID string) string {
				return ts.createFlowIn(ouID, "share-flow")
			},
			base: func(id string) string { return "/flows/" + id + "/sharing-policies" },
		},
		{
			name: "connection", resourceType: "connection", subjectType: "connection",
			create: func(ouID string) string {
				return ts.createConnectionIn(ouID, "share-conn")
			},
			base: func(id string) string { return "/connections/google/" + id + "/sharing-policies" },
		},
		{
			name: "sender", resourceType: "connection", subjectType: "notification_sender",
			create: func(ouID string) string {
				return ts.createSender(ts.prefix+"share-sender", ouID)
			},
			base: func(id string) string { return "/connections/twilio/" + id + "/sharing-policies" },
		},
	}
	for _, tc := range cases {
		ts.Run(tc.name, func() {
			ouID := ts.createOrganizationUnit("share-" + tc.name)
			ts.createChildOrganizationUnit("share-"+tc.name+"-child", ouID)
			resourceID := tc.create(ouID)
			base := tc.base(resourceID)

			created, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPost, base, sharingPolicyBody(0))
			ts.Require().Equal(http.StatusCreated, created.StatusCode, string(body))
			policyID := ts.idOf(body)
			ts.T().Cleanup(func() { ts.send(http.MethodDelete, base+"/"+policyID, "") })
			updated, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPut, base+"/"+policyID,
				sharingPolicyBody(1))
			ts.Require().Equal(http.StatusOK, updated.StatusCode, string(body))
			deleted := ts.send(http.MethodDelete, base+"/"+policyID, "")
			ts.Require().Equal(http.StatusNoContent, deleted.StatusCode)
			deletedAgain := ts.send(http.MethodDelete, base+"/"+policyID, "")
			ts.Require().Equal(http.StatusNoContent, deletedAgain.StatusCode)

			ts.requireSharingPolicyLifecycle(ts.linesPerCall(created, updated, deleted, deletedAgain),
				tc.resourceType, testutils.AuditRef{Type: tc.subjectType, ID: resourceID}, policyID, ouID)
		})
	}
}

// A sharing policy change that fails records the policy it named, or none on a create, and no subject.
// A manager whose permissions reach only its own unit is refused a policy for a role, flow or connection
// of another unit, which is recorded as denied with the resource, which exists, as subject. A delete of a
// policy the resource does not hold succeeds and writes no line.
func (ts *AuditLogTestSuite) TestEveryRoleFlowAndConnectionSharingPolicyPathRecordsItsOutcome() {
	siblingOU := ts.createOrganizationUnit("share-paths")
	roleID := ts.createRoleIn(siblingOU)
	flowID := ts.createFlowIn(siblingOU, "share-paths-flow")
	connectionID := ts.createConnectionIn(siblingOU, "share-paths-conn")
	manager := ts.createScopedManager("share-paths-mgr", nil, []string{"org"})
	missing := unusedID()

	type sharingCase struct {
		name    string
		client  *http.Client
		method  string
		path    string
		body    string
		status  int
		event   string
		outcome map[string]string
		target  testutils.AuditRef
		subject testutils.AuditRef
	}
	var cases []sharingCase
	for _, resource := range []struct {
		kind, collection, id, notFound, policyNotFound string
	}{
		{"role", "/roles/", roleID, "rol_1003", "rol_1022"},
		{"flow", "/flows/", flowID, "flm_1003", "flm_1030"},
		{"connection", "/connections/google/", connectionID, "idp_1001", "con_1032"},
	} {
		policyType := resource.kind + "_sharing_policy"
		policies := resource.collection + resource.id + "/sharing-policies"
		cases = append(cases,
			sharingCase{
				name: "a policy for a " + resource.kind + " that does not exist", client: testutils.GetHTTPClient(),
				method: http.MethodPost, path: resource.collection + missing + "/sharing-policies",
				body: sharingPolicyBody(0), status: http.StatusNotFound,
				event:   resource.kind + ".sharing_policy.create",
				outcome: map[string]string{"result": "failed", "reason": resource.notFound},
				target:  testutils.AuditRef{Type: policyType},
			},
			sharingCase{
				name: "an update of a policy the " + resource.kind + " does not hold", client: testutils.GetHTTPClient(),
				method: http.MethodPut, path: policies + "/" + missing, body: sharingPolicyBody(1),
				status: http.StatusNotFound, event: resource.kind + ".sharing_policy.update",
				outcome: map[string]string{"result": "failed", "reason": resource.policyNotFound},
				target:  testutils.AuditRef{Type: policyType, ID: missing},
			},
			sharingCase{
				name:   "a policy for a " + resource.kind + " of a unit the caller does not manage",
				client: manager.client, method: http.MethodPost, path: policies, body: sharingPolicyBody(0),
				status: http.StatusForbidden, event: resource.kind + ".sharing_policy.create",
				outcome: map[string]string{"result": "denied", "reason": "forbidden"},
				target:  testutils.AuditRef{Type: policyType},
				subject: testutils.AuditRef{Type: resource.kind, ID: resource.id},
			},
			sharingCase{
				name: "a delete of a policy the " + resource.kind + " does not hold", client: testutils.GetHTTPClient(),
				method: http.MethodDelete, path: policies + "/" + missing, status: http.StatusNoContent,
			},
		)
	}

	responses := make([]*http.Response, len(cases))
	for i, tc := range cases {
		response, body := ts.exchange(tc.client, tc.method, tc.path, tc.body)
		ts.Require().Equal(tc.status, response.StatusCode, "%s: %s", tc.name, string(body))
		responses[i] = response
	}
	written := ts.linesPerCall(responses...)
	for i, tc := range cases {
		if tc.event == "" {
			ts.Empty(written[i], "%s: a delete that removed nothing writes no line", tc.name)
			continue
		}
		ts.Require().Len(written[i], 1, tc.name)
		line := written[i][0]
		ts.Equal(tc.event, line.Event, tc.name)
		ts.Equal(tc.outcome, line.Outcome, tc.name)
		ts.Equal(tc.target, line.Target, tc.name)
		ts.Equal(tc.subject, line.Subject, tc.name)
		if tc.client == manager.client {
			ts.Equal(testutils.AuditRef{Type: "user", ID: manager.id}, line.Actor, tc.name)
		}
		ts.NotContains(line.Raw, siblingOU, "%s: no organization unit is recorded", tc.name)
	}
}

// Setting the values a unit holds for the variables of a flow writes one line with the flow as target and
// subject, and neither the name nor the value of any variable, nor the unit. A failed attempt records the
// flow ID the caller sent and no subject; a manager of another unit is refused, which is recorded as
// denied with the flow, which exists, as subject.
func (ts *AuditLogTestSuite) TestSettingFlowVariablesRecordsItsOutcome() {
	ownerOU := ts.createOrganizationUnit("vars-owner")
	childOU := ts.createChildOrganizationUnit("vars-child", ownerOU)
	flowID := ts.createVariablesFlowIn(ownerOU, "vars-flow")
	shared, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPost, "/flows/"+flowID+"/sharing-policies",
		ts.jsonBody(variablesSharingPolicy()))
	ts.Require().Equal(http.StatusCreated, shared.StatusCode, string(body))
	policyID := ts.idOf(body)
	ts.T().Cleanup(func() { ts.send(http.MethodDelete, "/flows/"+flowID+"/sharing-policies/"+policyID, "") })
	manager := ts.createScopedManager("vars-mgr", nil, []string{"org"})
	missing := unusedID()
	variablesPath := "/flows/" + flowID + "/variables"
	values := map[string]any{auditVariableName: auditVariableValue}

	flowRef := testutils.AuditRef{Type: "flow", ID: flowID}
	cases := []struct {
		name    string
		client  *http.Client
		path    string
		body    map[string]any
		status  int
		target  testutils.AuditRef
		subject testutils.AuditRef
		outcome map[string]string
	}{
		{"a unit's values set", testutils.GetHTTPClient(), variablesPath,
			map[string]any{"ouId": childOU, "values": values}, http.StatusOK, flowRef, flowRef,
			map[string]string{"result": "succeeded"}},
		{"values for a flow that does not exist", testutils.GetHTTPClient(), "/flows/" + missing + "/variables",
			map[string]any{"ouId": childOU, "values": values}, http.StatusNotFound,
			testutils.AuditRef{Type: "flow", ID: missing}, testutils.AuditRef{},
			map[string]string{"result": "failed", "reason": "flm_1003"}},
		{"values that name no unit", testutils.GetHTTPClient(), variablesPath,
			map[string]any{"values": values}, http.StatusBadRequest, flowRef, testutils.AuditRef{},
			map[string]string{"result": "failed", "reason": "flm_1038"}},
		{"a value for a variable the flow does not declare", testutils.GetHTTPClient(), variablesPath,
			map[string]any{"ouId": childOU, "values": map[string]any{"auditUndeclared": auditVariableValue}},
			http.StatusBadRequest, flowRef, testutils.AuditRef{},
			map[string]string{"result": "failed", "reason": "flm_1035"}},
		{"values for a unit the caller does not manage", manager.client, variablesPath,
			map[string]any{"ouId": ownerOU, "values": map[string]any{}}, http.StatusForbidden, flowRef, flowRef,
			map[string]string{"result": "denied", "reason": "forbidden"}},
	}

	responses := make([]*http.Response, len(cases))
	for i, tc := range cases {
		response, body := ts.exchange(tc.client, http.MethodPut, tc.path, ts.jsonBody(tc.body))
		ts.Require().Equal(tc.status, response.StatusCode, "%s: %s", tc.name, string(body))
		responses[i] = response
	}
	lines := ts.linesOf(responses...)
	for i, tc := range cases {
		line := lines[i]
		if tc.subject != (testutils.AuditRef{}) {
			ts.Equal(lineKeys, ts.keys(line.Raw), tc.name)
		}
		ts.Equal("flow.variables.update", line.Event, tc.name)
		ts.Equal("update", line.Action, tc.name)
		ts.Equal(tc.target, line.Target, tc.name)
		ts.Equal(tc.subject, line.Subject, tc.name)
		ts.Equal(tc.outcome, line.Outcome, tc.name)
		ts.requireNoVariable(line, ownerOU, childOU)
	}
}

// An import of a flow that carries the values a unit holds for its variables writes the flow's line, then
// one flow.variables.update line, then the import's own. The policy the document shares the flow with
// writes none: importing a sharing policy is not audited.
func (ts *AuditLogTestSuite) TestAnImportOfAFlowWithVariableValuesWritesTheirLine() {
	ownerOU := ts.createOrganizationUnit("vars-import")
	childOU := ts.createChildOrganizationUnit("vars-import-child", ownerOU)
	flowID := unusedID()
	handle := ts.prefix + "vars-import"
	ts.T().Cleanup(func() { _ = testutils.DeleteFlow(flowID) })

	document := variablesFlowDefinition(handle, ownerOU)
	document["resource_type"] = "flow"
	document["id"] = flowID
	document["sharingPolicies"] = []map[string]any{variablesSharingPolicy()}
	document["variableValues"] = []map[string]any{
		{"ouId": childOU, "values": map[string]any{auditVariableName: auditVariableValue}},
	}
	content, err := json.Marshal(document)
	ts.Require().NoError(err)

	succeeded := map[string]string{"result": "succeeded"}
	flowRef := testutils.AuditRef{Type: "flow", ID: flowID}
	for _, operation := range []string{"create", "update"} {
		imported, response := ts.importUpsert(string(content), 1)
		result := response.Results[0]
		ts.Require().Equal("success", result.Status, result.Message)
		ts.Require().Equal(operation, result.Operation, result.Message)

		written := ts.linesPerCall(imported)[0]
		ts.Require().Len(written, 3, "%s: the flow's line, its values' line and the import's", operation)
		for i, event := range []string{"flow." + operation, "flow.variables.update", importEvent} {
			ts.Equal(event, written[i].Event, operation)
			ts.Equal(succeeded, written[i].Outcome, "%s: %s", operation, event)
			ts.requireNoVariable(written[i], ownerOU, childOU)
			ts.NotContains(written[i].Raw, handle, "%s: the flow's handle is not recorded", event)
		}
		ts.Equal(flowRef, written[0].Target, operation)
		ts.Equal(flowRef, written[1].Target, operation)
		ts.Equal(flowRef, written[1].Subject, operation)
	}
}

// requireNoVariable checks that line carries neither the name nor the value of a variable, nor the units
// that own or set them.
func (ts *AuditLogTestSuite) requireNoVariable(line testutils.AuditLine, ouIDs ...string) {
	ts.T().Helper()
	ts.NotContains(line.Raw, auditVariableValue, "%s: the value of a variable is never recorded", line.Event)
	ts.NotContains(line.Raw, "audit-variables", "%s: no part of a variable's value is recorded", line.Event)
	ts.NotContains(line.Raw, auditVariableName, "%s: the name of a variable is not recorded", line.Event)
	for _, ouID := range ouIDs {
		ts.NotContains(line.Raw, ouID, "%s: no organization unit is recorded", line.Event)
	}
}

// createRoleIn creates a role in ouID, removed when the test ends, and returns its ID.
func (ts *AuditLogTestSuite) createRoleIn(ouID string) string {
	ts.T().Helper()
	roleID, err := testutils.CreateRole(testutils.Role{Name: auditRoleName, OUID: ouID})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteRole(roleID) })
	return roleID
}

// createFlowIn creates the smallest authentication flow in ouID, removed when the test ends, and returns
// its ID.
func (ts *AuditLogTestSuite) createFlowIn(ouID, name string) string {
	ts.T().Helper()
	created, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPost, "/flows",
		ts.flowBody(ts.prefix+name, "Audit Flow "+name, ouID))
	ts.Require().Equal(http.StatusCreated, created.StatusCode, string(body))
	flowID := ts.idOf(body)
	ts.T().Cleanup(func() { _ = testutils.DeleteFlow(flowID) })
	return flowID
}

// createVariablesFlowIn creates an authentication flow in ouID that declares auditVariableName, removed
// when the test ends, and returns its ID.
func (ts *AuditLogTestSuite) createVariablesFlowIn(ouID, name string) string {
	ts.T().Helper()
	created, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPost, "/flows",
		ts.jsonBody(variablesFlowDefinition(ts.prefix+name, ouID)))
	ts.Require().Equal(http.StatusCreated, created.StatusCode, string(body))
	flowID := ts.idOf(body)
	ts.T().Cleanup(func() { _ = testutils.DeleteFlow(flowID) })
	return flowID
}

// createConnectionIn creates a Google connection in ouID, removed when the test ends, and returns its ID.
func (ts *AuditLogTestSuite) createConnectionIn(ouID, name string) string {
	ts.T().Helper()
	_, id := ts.create("/connections/google", map[string]interface{}{
		"name": ts.prefix + name, "ouId": ouID, "clientId": auditIDPClientID,
		"clientSecret": auditIDPClientSecret, "redirectUri": "https://localhost:3000/google/callback",
	})
	return id
}

// variablesSharingPolicy shares a flow with every unit beneath its owner and lets each set its own value
// of auditVariableName.
func variablesSharingPolicy() map[string]any {
	return map[string]any{"childUnits": map[string]any{
		"mode": "all",
		"overlayRules": map[string]any{
			"variables." + auditVariableName: map[string]any{"editable": true},
		},
	}}
}

// variablesFlowDefinition is an authentication flow with handle, owned by ouID, that picks an organization
// unit, declares auditVariableName in a VARIABLES node and shows it.
func variablesFlowDefinition(handle, ouID string) map[string]any {
	return map[string]any{
		"handle":   handle,
		"name":     "Audit Variables Flow",
		"flowType": "AUTHENTICATION",
		"ouId":     ouID,
		"nodes": []map[string]any{
			{"id": "start", "type": "START", "onSuccess": "resolve_ou"},
			{
				"id": "resolve_ou", "type": "TASK_EXECUTION", "onSuccess": "vars",
				"executor":   map[string]any{"name": "OUResolverExecutor"},
				"properties": map[string]any{"resolveFrom": "prompt", "parentOuId": ouID},
			},
			{
				"id": "vars", "type": "VARIABLES", "onSuccess": "prompt_credentials",
				"inputs": []map[string]any{
					{"identifier": auditVariableName, "type": "EMAIL_INPUT", "label": "Support email",
						"defaultValue": "support@example.org"},
				},
			},
			{
				"id": "prompt_credentials", "type": "PROMPT",
				"properties": map[string]any{
					"helpText": fmt.Sprintf("Write to {{var(%s)}}", auditVariableName),
				},
				"meta": map[string]any{"components": []map[string]any{
					{"type": "TEXT", "id": "help", "label": "Sign in", "variant": "BODY_1"},
				}},
				"prompts": []map[string]any{{
					"inputs": []map[string]any{
						{"ref": "input_username", "identifier": "username", "type": "TEXT_INPUT", "required": true},
						{"ref": "input_password", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
					},
					"action": map[string]any{"ref": "action_signin", "nextNode": "credentials_auth"},
				}},
			},
			{
				"id": "credentials_auth", "type": "TASK_EXECUTION", "onSuccess": "auth_assert",
				"executor": map[string]any{"name": "CredentialsAuthExecutor"},
			},
			{
				"id": "auth_assert", "type": "TASK_EXECUTION", "onSuccess": "end",
				"executor": map[string]any{"name": "AuthAssertExecutor"},
			},
			{"id": "end", "type": "END"},
		},
	}
}
