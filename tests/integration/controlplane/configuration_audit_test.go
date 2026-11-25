// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"net/http"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// The Control Plane writes one line for each import and each export request, naming the configuration
// as a whole and nothing the request carried. The capture of the current version that a successful
// import schedules reads the configuration as an export does, but is not an export request, so it
// writes no export line.
func (ts *ControlPlaneTestSuite) TestConfigurationImportAndExportAreAudited() {
	ts.Require().NoError(testutils.RestartServer(), "the Control Plane must restart so its output is captured")
	reader := testutils.NewAuditLogReader()
	ts.Require().NoError(reader.Reset())
	adminRef := testutils.AuditRef{Type: "user", ID: testutils.TrustedIssuerAdminSubject}
	configurationRef := testutils.AuditRef{Type: "configuration"}
	handle := unique("cp-audit-dry-ou")

	status, raw := ts.call(http.MethodPost, "/import", map[string]any{
		"dryRun":  true,
		"content": "resource_type: organization_unit\nhandle: " + handle + "\nname: " + handle + "\n",
		"options": map[string]any{"target": "runtime", "upsert": true},
	}, nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	imported, err := reader.Await(func(line testutils.AuditLine) bool {
		return line.Event == "configuration.import"
	})
	ts.Require().NoError(err)
	ts.Equal(adminRef, imported.Actor)
	ts.Equal(configurationRef, imported.Target)
	ts.Equal(map[string]string{"result": "succeeded"}, imported.Outcome)
	ts.Contains(imported.Raw, `"details":{"dry_run":"true"}`)
	ts.NotContains(imported.Raw, handle, "nothing the import carried is recorded")

	// A capture runs two seconds after the change it follows.
	ts.Never(func() bool {
		lines, err := reader.Read()
		if err != nil {
			return true
		}
		for _, line := range lines {
			if line.Event == "configuration.export" {
				return true
			}
		}
		return false
	}, 4*time.Second, 200*time.Millisecond, "a capture of the current version is not an export request")

	status, raw = ts.call(http.MethodPost, "/export", map[string]any{"organizationUnits": []string{ts.ouID}}, nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	exported, err := reader.Await(func(line testutils.AuditLine) bool {
		return line.Event == "configuration.export"
	})
	ts.Require().NoError(err)
	ts.Equal(adminRef, exported.Actor)
	ts.Equal(configurationRef, exported.Target)
	ts.Equal(testutils.AuditRef{}, exported.Subject)
	ts.Equal(map[string]string{"result": "succeeded"}, exported.Outcome)
	ts.NotContains(exported.Raw, ts.ouID, "nothing the export named is recorded")
}
