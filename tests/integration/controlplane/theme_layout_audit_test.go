// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"net/http"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// The Control Plane writes an audit line for each theme and layout it creates, updates and deletes, with
// the administrator the trusted issuer's token names as the actor and nothing of the theme or layout but
// its ID. It writes its output to the server log only when a test started it, so the test restarts it
// first.
func (ts *ControlPlaneTestSuite) TestThemeAndLayoutChangesAreAudited() {
	ts.Require().NoError(testutils.RestartServer(), "the Control Plane must restart so its output is captured")
	reader := testutils.NewAuditLogReader()
	ts.Require().NoError(reader.Reset())

	for _, resource := range []struct{ kind, path string }{
		{"theme", "/design/themes"},
		{"layout", "/design/layouts"},
	} {
		handle := unique("cp-audit-" + resource.kind)
		displayName := unique("CpAuditDesignName")
		content := map[string]any{"colors": map[string]any{"primary": "#c0ffee"}}
		var created struct {
			ID string `json:"id"`
		}
		status, raw := ts.call(http.MethodPost, resource.path, map[string]any{
			"handle": handle, "displayName": displayName, resource.kind: content,
		}, &created)
		ts.Require().Equal(http.StatusCreated, status, string(raw))
		ts.Require().NotEmpty(created.ID)
		ts.T().Cleanup(func() { ts.call(http.MethodDelete, resource.path+"/"+created.ID, nil, nil) })
		status, raw = ts.call(http.MethodPut, resource.path+"/"+created.ID, map[string]any{
			"displayName": displayName, resource.kind: content,
		}, nil)
		ts.Require().Equal(http.StatusOK, status, string(raw))
		status, raw = ts.call(http.MethodDelete, resource.path+"/"+created.ID, nil, nil)
		ts.Require().Equal(http.StatusNoContent, status, string(raw))

		ref := testutils.AuditRef{Type: resource.kind, ID: created.ID}
		for _, action := range []string{"create", "update", "delete"} {
			event := resource.kind + "." + action
			line, err := reader.Await(func(line testutils.AuditLine) bool {
				return line.Event == event && line.Target == ref
			})
			ts.Require().NoError(err, event)
			ts.Equal(ref, line.Subject, event)
			ts.Equal(testutils.AuditRef{Type: "user", ID: testutils.TrustedIssuerAdminSubject}, line.Actor, event)
			ts.Equal(map[string]string{"result": "succeeded"}, line.Outcome, event)
			ts.NotContains(line.Raw, handle, "the handle is not recorded")
			ts.NotContains(line.Raw, displayName, "the display name is not recorded")
			ts.NotContains(line.Raw, "#c0ffee", "no content is recorded")
		}
	}
}
