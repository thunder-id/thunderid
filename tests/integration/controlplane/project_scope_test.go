// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"net/http"
	"strings"
)

type scopedChange struct {
	Key      string `json:"key"`
	Section  string `json:"section"`
	Required bool   `json:"required"`
}

// A diff scoped to a project places the project's application and its root unit in the project's section,
// and the flows the application names, which the Data Plane lacks, as needed changes it requires. An
// apply that leaves them out is refused before anything is sent, and an unknown project is not found.
func (ts *ControlPlaneTestSuite) TestADeployScopedToAProjectSendsWhatItNeeds() {
	gatewayID := ts.registerDataPlane()
	var created struct {
		ID string `json:"id"`
	}
	handle := unique("scoped")
	status, raw := ts.call(http.MethodPost, "/projects", map[string]any{"handle": handle, "displayName": handle},
		&created)
	ts.Require().Equal(http.StatusCreated, status, string(raw))
	var app struct {
		ID string `json:"id"`
	}
	status, raw = ts.call(http.MethodPost, "/applications", map[string]any{"name": unique("Scoped App"),
		"projectId": created.ID, "type": "m2m"}, &app)
	ts.Require().Equal(http.StatusCreated, status, string(raw))
	defer func() {
		ts.call(http.MethodDelete, "/applications/"+app.ID, nil, nil)
		ts.call(http.MethodDelete, "/projects/"+created.ID, nil, nil)
	}()

	var captured struct {
		Version string `json:"version"`
	}
	status, raw = ts.call(http.MethodPost, "/configuration-versions", map[string]any{"note": "a project's app"},
		&captured)
	ts.Require().Contains([]int{http.StatusCreated, http.StatusOK}, status, string(raw))

	var diff struct {
		Changes []scopedChange `json:"changes"`
	}
	status, raw = ts.call(http.MethodGet, "/gateways/"+gatewayID+"/diff?version="+captured.Version+
		"&project="+created.ID, nil, &diff)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	appKey := "application/" + app.ID
	sections := map[string]scopedChange{}
	for _, change := range diff.Changes {
		sections[change.Key] = change
	}
	ts.Equal("project", sections[appKey].Section, "the project's application is not its own")
	required := 0
	for _, change := range diff.Changes {
		if change.Required {
			required++
			ts.Equal("needed", change.Section, "%s is required but not needed", change.Key)
		}
	}
	ts.Positive(required, "nothing the application needs is required")

	status, raw = ts.call(http.MethodPost, "/gateways/"+gatewayID+"/apply", map[string]any{
		"version": captured.Version, "project": created.ID, "selection": []string{appKey}}, nil)
	ts.Equal(http.StatusBadRequest, status, string(raw))
	ts.True(strings.Contains(string(raw), "GTW-1040"), string(raw))

	status, raw = ts.call(http.MethodGet, "/gateways/"+gatewayID+"/diff?project=no-such-project", nil, nil)
	ts.Equal(http.StatusNotFound, status, string(raw))
	ts.True(strings.Contains(string(raw), "GTW-1038"), string(raw))
}
