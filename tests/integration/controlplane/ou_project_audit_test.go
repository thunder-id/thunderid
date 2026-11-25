// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"fmt"
	"net/http"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// The Control Plane writes an audit line for each organization unit and each project it creates,
// updates and deletes, with the administrator the trusted issuer's token names as the actor and none of
// their names, handles or descriptions. A delete of a project that does not exist succeeds and writes no
// line. Projects are served only here.
func (ts *ControlPlaneTestSuite) TestOrganizationUnitAndProjectChangesAreAudited() {
	ts.Require().NoError(testutils.RestartServer(), "the Control Plane must restart so its output is captured")
	reader := testutils.NewAuditLogReader()
	ts.Require().NoError(reader.Reset())

	projectHandle := unique("cp-audit-prj")
	var project struct {
		ID string `json:"id"`
	}
	status, raw := ts.call(http.MethodPost, "/projects",
		map[string]any{"handle": projectHandle, "displayName": "Audited CP Project"}, &project)
	ts.Require().Equal(http.StatusCreated, status, string(raw))
	ts.T().Cleanup(func() { _, _ = ts.call(http.MethodDelete, "/projects/"+project.ID, nil, nil) })

	// The Control Plane serves projects, so the organization unit belongs to the one above.
	ouHandle := unique("cp-audit-ou")
	var ou struct {
		ID string `json:"id"`
	}
	status, raw = ts.call(http.MethodPost, "/organization-units",
		map[string]any{"handle": ouHandle, "name": "Audited CP Unit", "projectId": project.ID}, &ou)
	ts.Require().Equal(http.StatusCreated, status, string(raw))
	ts.T().Cleanup(func() { _ = testutils.DeleteOrganizationUnit(ou.ID) })
	status, raw = ts.call(http.MethodPut, "/organization-units/"+ou.ID,
		map[string]any{"handle": ouHandle, "name": "Audited CP Unit renamed", "projectId": project.ID}, nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	ts.Require().NoError(testutils.DeleteOrganizationUnit(ou.ID))

	status, raw = ts.call(http.MethodPut, "/projects/"+project.ID,
		map[string]any{"displayName": "Audited CP Project renamed", "description": "Audited description"}, nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	status, raw = ts.call(http.MethodDelete, "/projects/"+project.ID, nil, nil)
	ts.Require().Equal(http.StatusNoContent, status, string(raw))

	// An import creates a project that does not exist and updates one that does.
	importedID := fmt.Sprintf("019a0000-0000-7000-8000-%012x", time.Now().UnixNano()&0xffffffffffff)
	importedHandle := unique("cp-audit-imp")
	ts.T().Cleanup(func() { _, _ = ts.call(http.MethodDelete, "/projects/"+importedID, nil, nil) })
	for range 2 {
		status, raw = ts.call(http.MethodPost, "/import", map[string]any{
			"content": fmt.Sprintf("resource_type: project\nid: %s\nhandle: %s\ndisplayName: Audited Import\n",
				importedID, importedHandle),
			"options": map[string]any{"target": "runtime", "upsert": true},
		}, nil)
		ts.Require().Equal(http.StatusOK, status, string(raw))
	}

	missing := fmt.Sprintf("019a0000-0000-7000-8000-%012x", time.Now().UnixNano()&0xffffffffffff)
	status, raw = ts.call(http.MethodDelete, "/projects/"+missing, nil, nil)
	ts.Require().Equal(http.StatusNoContent, status, string(raw))
	status, raw = ts.call(http.MethodPut, "/projects/"+missing, map[string]any{"displayName": "Audited"}, nil)
	ts.Require().Equal(http.StatusNotFound, status, string(raw))

	admin := testutils.AuditRef{Type: "user", ID: testutils.TrustedIssuerAdminSubject}
	for _, tc := range []struct {
		event string
		ref   testutils.AuditRef
	}{
		{"ou.create", testutils.AuditRef{Type: "ou", ID: ou.ID}},
		{"ou.update", testutils.AuditRef{Type: "ou", ID: ou.ID}},
		{"ou.delete", testutils.AuditRef{Type: "ou", ID: ou.ID}},
		{"project.create", testutils.AuditRef{Type: "project", ID: project.ID}},
		{"project.update", testutils.AuditRef{Type: "project", ID: project.ID}},
		{"project.delete", testutils.AuditRef{Type: "project", ID: project.ID}},
		{"project.create", testutils.AuditRef{Type: "project", ID: importedID}},
		{"project.update", testutils.AuditRef{Type: "project", ID: importedID}},
	} {
		line, err := reader.Await(func(line testutils.AuditLine) bool {
			return line.Event == tc.event && line.Target == tc.ref
		})
		ts.Require().NoError(err, tc.event)
		ts.Equal(tc.ref, line.Subject, tc.event)
		ts.Equal(admin, line.Actor, tc.event)
		ts.Equal(map[string]string{"result": "succeeded"}, line.Outcome, tc.event)
		ts.NotContains(line.Raw, "Audited", "%s: no name or description is recorded", tc.event)
		ts.NotContains(line.Raw, ouHandle, "%s: no handle is recorded", tc.event)
		ts.NotContains(line.Raw, projectHandle, "%s: no handle is recorded", tc.event)
		ts.NotContains(line.Raw, importedHandle, "%s: no handle is recorded", tc.event)
	}

	// The failed update of the missing project is written after its delete would have been, so once it is
	// read, every line the delete could have written is read too.
	missingRef := testutils.AuditRef{Type: "project", ID: missing}
	failed, err := reader.Await(func(line testutils.AuditLine) bool {
		return line.Event == "project.update" && line.Target == missingRef
	})
	ts.Require().NoError(err)
	ts.Equal(map[string]string{"result": "failed", "reason": "prj_1002"}, failed.Outcome)
	ts.Equal(testutils.AuditRef{}, failed.Subject, "a failed update has no subject")
	lines, err := reader.Read()
	ts.Require().NoError(err)
	for _, line := range lines {
		ts.False(line.Event == "project.delete" && line.Target == missingRef,
			"a delete that removed nothing writes no line")
	}
}
