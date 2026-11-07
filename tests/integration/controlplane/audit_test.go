// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"net/http"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// The Control Plane writes an audit line for each application it creates, updates and deletes, with
// the administrator the trusted issuer's token names as the actor and no client secret. It writes its
// output to the server log only when a test started it, so the test restarts it first.
func (ts *ControlPlaneTestSuite) TestApplicationChangesAreAudited() {
	ts.Require().NoError(testutils.RestartServer(), "the Control Plane must restart so its output is captured")
	reader := testutils.NewAuditLogReader()
	ts.Require().NoError(reader.Reset())

	name := unique("cp-audit-app")
	clientSecret := unique("CpAuditSecret")
	appID, err := testutils.CreateApplication(testutils.Application{
		OUID: ts.ouID, Name: name, ClientID: unique("cp-audit-client"), ClientSecret: clientSecret,
	})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteApplication(appID) })
	status, raw := ts.call(http.MethodPut, "/applications/"+appID, map[string]any{
		"ouId": ts.ouID, "name": name, "description": "Updated on the Control Plane",
	}, nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	ts.Require().NoError(testutils.DeleteApplication(appID))

	appRef := testutils.AuditRef{Type: "application", ID: appID}
	for _, event := range []string{"application.create", "application.update", "application.delete"} {
		line, err := reader.Await(func(line testutils.AuditLine) bool {
			return line.Event == event && line.Target == appRef
		})
		ts.Require().NoError(err, event)
		ts.Equal(appRef, line.Subject, event)
		ts.Equal(testutils.AuditRef{Type: "user", ID: testutils.TrustedIssuerAdminSubject}, line.Actor, event)
		ts.Equal(map[string]string{"result": "succeeded"}, line.Outcome, event)
		ts.NotContains(line.Raw, clientSecret, "a client secret is never recorded")
		ts.NotContains(line.Raw, name, "no attribute of the application is recorded")
	}
}
