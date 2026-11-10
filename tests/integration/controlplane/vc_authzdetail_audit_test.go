// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"net/http"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// The Control Plane writes an audit line for each authorization details type, presentation definition and
// credential configuration it creates, updates and deletes, with the administrator as the actor and nothing
// of the resource. A delete of one that does not exist succeeds and writes no line.
func (ts *ControlPlaneTestSuite) TestConfigurationResourceChangesAreAudited() {
	ts.Require().NoError(testutils.RestartServer(), "the Control Plane must restart so its output is captured")
	reader := testutils.NewAuditLogReader()
	ts.Require().NoError(reader.Reset())

	marker := unique("cp-audit-cfg")
	schema := map[string]any{"type": "object", "properties": map[string]any{"amount": map[string]any{"type": "number"}}}
	resources := []struct {
		targetType string
		path       string
		body       func(suffix string) map[string]any
	}{
		{"authorization_detail_type", "/authorization-detail-types", func(suffix string) map[string]any {
			return map[string]any{"type": "https://types.example.com/" + marker, "name": marker + suffix,
				"schema": schema}
		}},
		{"presentation_definition", "/openid4vp/presentation-definitions", func(suffix string) map[string]any {
			return map[string]any{"handle": marker + suffix, "projectId": defaultProjectID,
				"vct": "https://credentials.example.com/" + marker}
		}},
		{"credential_configuration", "/openid4vci/credential-configurations", func(suffix string) map[string]any {
			return map[string]any{"handle": marker + suffix, "projectId": defaultProjectID,
				"vct": "https://credentials.example.com/" + marker}
		}},
	}

	missing := unique("cp-audit-missing")
	ids := make([]string, len(resources))
	for i, resource := range resources {
		var created struct {
			ID string `json:"id"`
		}
		status, raw := ts.call(http.MethodPost, resource.path, resource.body(""), &created)
		ts.Require().Equal(http.StatusCreated, status, "%s: %s", resource.targetType, raw)
		ids[i] = created.ID
		ts.T().Cleanup(func() { _, _ = ts.call(http.MethodDelete, resource.path+"/"+created.ID, nil, nil) })
		status, raw = ts.call(http.MethodPut, resource.path+"/"+created.ID, resource.body("-renamed"), nil)
		ts.Require().Equal(http.StatusOK, status, "%s: %s", resource.targetType, raw)
		status, raw = ts.call(http.MethodDelete, resource.path+"/"+created.ID, nil, nil)
		ts.Require().Equal(http.StatusNoContent, status, "%s: %s", resource.targetType, raw)
		status, raw = ts.call(http.MethodDelete, resource.path+"/"+missing, nil, nil)
		ts.Require().Equal(http.StatusNoContent, status, "%s: %s", resource.targetType, raw)
	}

	admin := testutils.AuditRef{Type: "user", ID: testutils.TrustedIssuerAdminSubject}
	for i, resource := range resources {
		ref := testutils.AuditRef{Type: resource.targetType, ID: ids[i]}
		for _, action := range []string{"create", "update", "delete"} {
			event := resource.targetType + "." + action
			line, err := reader.Await(func(line testutils.AuditLine) bool {
				return line.Event == event && line.Target == ref
			})
			ts.Require().NoError(err, event)
			ts.Equal(ref, line.Subject, event)
			ts.Equal(admin, line.Actor, event)
			ts.Equal(map[string]string{"result": "succeeded"}, line.Outcome, event)
			ts.NotContains(line.Raw, marker, "%s: nothing of the resource is recorded", event)
			ts.NotContains(line.Raw, defaultProjectID, "%s: the project is not recorded", event)
		}
	}
	// Every line the deletes of the missing IDs could have written precedes the last line awaited above.
	lines, err := reader.Read()
	ts.Require().NoError(err)
	for _, line := range lines {
		ts.NotEqual(missing, line.Target.ID, "%s: a delete that removed nothing writes no line", line.Event)
	}
}
