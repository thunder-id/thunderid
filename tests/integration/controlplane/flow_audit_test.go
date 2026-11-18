// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// The Control Plane writes an audit line for each flow it creates, restores and deletes, for each set of
// a unit's values for the variables of a flow, and for each write to a section of the server configuration,
// whether the section is written whole or one project's auto sync entry in it is changed, with the
// administrator the trusted issuer's token names as the actor and no part of the flow or the value written.
// It writes its output to the server log only when a test started it, so the test restarts it first.
func (ts *ControlPlaneTestSuite) TestFlowAndServerConfigChangesAreAudited() {
	ts.Require().NoError(testutils.RestartServer(), "the Control Plane must restart so its output is captured")
	reader := testutils.NewAuditLogReader()
	ts.Require().NoError(reader.Reset())

	handle := unique("cp-audit-flow")
	flowID, err := testutils.CreateIsolatedAuthFlow(handle)
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteFlow(flowID) })
	status, raw := ts.call(http.MethodPost, "/flows/"+flowID+"/restore", map[string]any{"version": 1}, nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	ts.Require().NoError(testutils.DeleteFlow(flowID))

	variablesFlowID, written := ts.setVariablesWithUpdateOnly()

	previous, err := testutils.GetWritableServerConfig("cors")
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.PutWritableServerConfig("cors", previous) })
	origin := "https://" + unique("cp-audit-origin") + ".example.com"
	status, raw = ts.call(http.MethodPut, "/server-config/cors",
		map[string]any{"allowedOrigins": []string{origin}}, nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	// Removing the auto sync entry of a project that has none still writes the gateway section.
	status, raw = ts.call(http.MethodDelete, "/projects/"+unique("cp-audit-project")+"/auto-sync", nil, nil)
	ts.Require().Equal(http.StatusNoContent, status, string(raw))

	admin := testutils.AuditRef{Type: "user", ID: testutils.TrustedIssuerAdminSubject}
	flowRef := testutils.AuditRef{Type: "flow", ID: flowID}
	section := testutils.AuditRef{Type: "server_config", ID: "cors"}
	gatewaySection := testutils.AuditRef{Type: "server_config", ID: "gateway"}
	for _, tc := range []struct {
		event  string
		target testutils.AuditRef
	}{
		{"flow.create", flowRef},
		{"flow.restore", flowRef},
		{"flow.delete", flowRef},
		{"flow.variables.update", testutils.AuditRef{Type: "flow", ID: variablesFlowID}},
		{"server_config.update", section},
		{"server_config.update", gatewaySection},
	} {
		line, err := reader.Await(func(line testutils.AuditLine) bool {
			return line.Event == tc.event && line.Target == tc.target
		})
		ts.Require().NoError(err, tc.event)
		ts.Equal(tc.target, line.Subject, tc.event)
		ts.Equal(admin, line.Actor, tc.event)
		ts.Equal(map[string]string{"result": "succeeded"}, line.Outcome, tc.event)
		ts.NotContains(line.Raw, handle, "no part of the flow is recorded")
		ts.NotContains(line.Raw, origin, "the value of a section is never recorded")
		for name, value := range written {
			ts.NotContains(line.Raw, name, "%s: the name of a variable is not recorded", tc.event)
			ts.NotContains(line.Raw, value, "%s: the value of a variable is never recorded", tc.event)
		}
	}
}

// setVariablesWithUpdateOnly sets a child unit's value for a variable of a flow shared with it, as a caller
// who may update flows but not view them, and returns the flow's ID and the values written. The response
// reads the values back, and that read must not ask for the view permission the caller lacks, since the
// write is already stored.
func (ts *ControlPlaneTestSuite) setVariablesWithUpdateOnly() (string, map[string]string) {
	updateOnly := os.Getenv(testutils.ControlPlaneScopedTokenEnv("system:flow:update"))
	ts.Require().NotEmpty(updateOnly, "the harness exports a system:flow:update token")
	childID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: unique("cp-audit-vars-child"), Name: "Control Plane variables child", Parent: &ts.ouID,
		ProjectID: defaultProjectID,
	})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteOrganizationUnit(childID) })

	const variable = "cpAuditSupportEmail"
	var flow struct {
		ID string `json:"id"`
	}
	status, raw := ts.call(http.MethodPost, "/flows", map[string]any{
		"handle": unique("cp-audit-vars-flow"), "name": "CP Audit Variables Flow", "flowType": "AUTHENTICATION",
		"ouId": ts.ouID,
		"nodes": []map[string]any{
			{"id": "start", "type": "START", "onSuccess": "resolve_ou"},
			{"id": "resolve_ou", "type": "TASK_EXECUTION", "onSuccess": "vars",
				"executor":   map[string]any{"name": "OUResolverExecutor"},
				"properties": map[string]any{"resolveFrom": "prompt", "parentOuId": ts.ouID}},
			{"id": "vars", "type": "VARIABLES", "onSuccess": "auth_assert", "inputs": []map[string]any{
				{"identifier": variable, "type": "EMAIL_INPUT", "label": "Support email",
					"defaultValue": "support@example.com"},
			}},
			{"id": "auth_assert", "type": "TASK_EXECUTION",
				"executor": map[string]any{"name": "AuthAssertExecutor"}, "onSuccess": "end"},
			{"id": "end", "type": "END"},
		},
	}, &flow)
	ts.Require().Equal(http.StatusCreated, status, string(raw))
	ts.T().Cleanup(func() { _ = testutils.DeleteFlow(flow.ID) })
	status, raw = ts.call(http.MethodPost, "/flows/"+flow.ID+"/sharing-policies", map[string]any{
		"childUnits": map[string]any{
			"mode":         "all",
			"overlayRules": map[string]any{"variables." + variable: map[string]any{"editable": true}},
		},
	}, nil)
	ts.Require().Equal(http.StatusCreated, status, string(raw))

	written := map[string]string{variable: unique("cp-audit-help") + "@unit.example"}
	request, err := json.Marshal(map[string]any{"ouId": childID, "values": written})
	ts.Require().NoError(err)
	status, raw = ts.send(testutils.GetHTTPClientWithToken(updateOnly), http.MethodPut,
		"/flows/"+flow.ID+"/variables", "application/json", request)
	ts.Require().Equal(http.StatusOK, status, "a caller who may update the flow is answered: %s", raw)
	var answered struct {
		Values map[string]string `json:"values"`
	}
	ts.Require().NoError(json.Unmarshal(raw, &answered), string(raw))
	ts.Equal(written, answered.Values, "the response carries the values written")
	status, raw = ts.send(testutils.GetHTTPClientWithToken(updateOnly), http.MethodGet,
		testutils.WithOU("/flows/"+flow.ID+"/variables", childID), "", nil)
	ts.Require().Equal(http.StatusForbidden, status, "the caller must not hold the view permission: %s", raw)
	return flow.ID, written
}
