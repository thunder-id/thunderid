// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"net/http"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// The Control Plane writes an audit line for each flow it creates, restores and deletes, and for each
// write to a section of the server configuration, whether the section is written whole or one project's
// auto sync entry in it is changed, with the administrator the trusted issuer's token names as the actor
// and no part of the flow or the value written. It writes its output to the server
// log only when a test started it, so the test restarts it first.
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
	}
}
