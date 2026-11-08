// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"net/http"
	"strings"
)

// projectStatus is what GET /gateways/{id}/project-status answers, as far as these tests read it.
type projectStatus struct {
	Project struct {
		Versions   []struct{} `json:"versions"`
		DeployedAt string     `json:"deployedAt"`
		LastAction string     `json:"lastAction"`
		Rollback   struct {
			Available bool      `json:"available"`
			Changes   int       `json:"changes"`
			ToVersion *struct{} `json:"toVersion"`
		} `json:"rollback"`
	} `json:"project"`
}

// statusOf reads where a project stands on a gateway, against the latest version.
func (ts *ControlPlaneTestSuite) statusOf(gatewayID, projectID string) projectStatus {
	ts.T().Helper()
	var status projectStatus
	code, raw := ts.call(http.MethodGet, "/gateways/"+gatewayID+"/project-status?project="+projectID, nil, &status)
	ts.Require().Equal(http.StatusOK, code, string(raw))
	return status
}

// defaultProjectID is the project the bootstrap creates, which holds the default flows.
const defaultProjectID = "01900000-0000-7000-8000-000000000002"

// deployDefaultProject captures a version and deploys the default project to the gateway. Its default flows
// are what other projects' applications sign in through, and a project deploy does not send another project's
// resources.
func (ts *ControlPlaneTestSuite) deployDefaultProject(gatewayID string) {
	ts.T().Helper()
	code, raw := ts.call(http.MethodPost, "/configuration-versions", map[string]any{"note": "the default flows"}, nil)
	ts.Require().Contains([]int{http.StatusCreated, http.StatusOK}, code, string(raw))
	code, raw = ts.call(http.MethodPost, "/gateways/"+gatewayID+"/apply", map[string]any{"project": defaultProjectID}, nil)
	ts.Require().Equal(http.StatusOK, code, string(raw))
}

// addProjectApp creates an application in the project, captures and deploys the project.
func (ts *ControlPlaneTestSuite) addProjectApp(gatewayID, projectID string) string {
	ts.T().Helper()
	var app struct{ ID string }
	code, raw := ts.call(http.MethodPost, "/applications", map[string]any{"name": unique("Rollback App"),
		"projectId": projectID, "type": "m2m"}, &app)
	ts.Require().Equal(http.StatusCreated, code, string(raw))
	ts.T().Cleanup(func() { ts.call(http.MethodDelete, "/applications/"+app.ID, nil, nil) })
	code, raw = ts.call(http.MethodPost, "/configuration-versions", map[string]any{"note": "a project app"}, nil)
	ts.Require().Contains([]int{http.StatusCreated, http.StatusOK}, code, string(raw))
	code, raw = ts.call(http.MethodPost, "/gateways/"+gatewayID+"/apply", map[string]any{"project": projectID}, nil)
	ts.Require().Equal(http.StatusOK, code, string(raw))
	return app.ID
}

// A project deployed to a gateway several times, past the versions a deployment keeps, reports where it
// stands, can be rolled back once to its last deploy's earlier state, and reports that rollback as its
// last write; its rollback point outlives the bound on kept masters.
func (ts *ControlPlaneTestSuite) TestAProjectIsDeployedReportedAndRolledBack() {
	gatewayID := ts.registerDataPlane()
	ts.deployDefaultProject(gatewayID)
	var created struct{ ID string }
	handle := unique("rollback")
	code, raw := ts.call(http.MethodPost, "/projects", map[string]any{"handle": handle, "displayName": handle},
		&created)
	ts.Require().Equal(http.StatusCreated, code, string(raw))
	ts.T().Cleanup(func() { ts.call(http.MethodDelete, "/projects/"+created.ID, nil, nil) })

	ts.Empty(ts.statusOf(gatewayID, created.ID).Project.Versions, "a gateway holding none of it names a version")
	first := ts.addProjectApp(gatewayID, created.ID)
	deployed := ts.statusOf(gatewayID, created.ID)
	ts.Equal("deployed", deployed.Project.LastAction)
	ts.NotEmpty(deployed.Project.DeployedAt)
	ts.Require().Len(deployed.Project.Versions, 1)
	ts.True(deployed.Project.Rollback.Available, "the project's first deploy cannot be rolled back")

	var last string
	for i := 0; i < 4; i++ {
		last = ts.addProjectApp(gatewayID, created.ID)
	}
	status := ts.statusOf(gatewayID, created.ID)
	ts.True(status.Project.Rollback.Available)
	ts.Equal(1, status.Project.Rollback.Changes, "the rollback does not undo the last deploy's one application")
	ts.NotNil(status.Project.Rollback.ToVersion)

	for _, dryRun := range []bool{true, false} {
		code, raw = ts.call(http.MethodPost, "/gateways/"+gatewayID+"/revert", map[string]any{"project": created.ID,
			"dryRun": dryRun}, nil)
		ts.Require().Equal(http.StatusOK, code, string(raw))
	}

	held := ts.masterOf(gatewayID)
	ts.NotContains(held.Resources, "id: "+last, "the rollback left the last deploy's application")
	ts.Contains(held.Resources, "id: "+first, "the rollback removed an application an earlier deploy wrote")
	rolledBack := ts.statusOf(gatewayID, created.ID)
	ts.Equal("rolledBack", rolledBack.Project.LastAction)
	ts.False(rolledBack.Project.Rollback.Available)

	code, raw = ts.call(http.MethodPost, "/gateways/"+gatewayID+"/revert", map[string]any{"project": created.ID}, nil)
	ts.Equal(http.StatusBadRequest, code, string(raw))
	ts.True(strings.Contains(string(raw), "GTW-1041"), string(raw))

	code, _ = ts.call(http.MethodPost, "/gateways/"+gatewayID+"/revert", map[string]any{"project": "none"}, nil)
	ts.Equal(http.StatusNotFound, code)
}
