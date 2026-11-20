// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"net/http"
	"net/http/httptest"
)

// A project's status against a gateway that runs nothing yet says nothing waits to be promoted from it,
// rather than refusing to be read: a new pipeline's later gateways have nothing to promote yet.
func (ts *ControlPlaneTestSuite) TestAProjectsStatusFromAGatewayThatRunsNothingHasNothingWaiting() {
	// A control plane registers one gateway at an address, so each is a stand-in of its own.
	stub := func() *httptest.Server {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		ts.T().Cleanup(server.Close)
		return server
	}
	source := ts.registerGateway(stub())
	target := ts.registerGateway(stub())
	code, raw := ts.call(http.MethodPost, "/configuration-versions", map[string]any{"note": "a version to compare"}, nil)
	ts.Require().Contains([]int{http.StatusCreated, http.StatusOK}, code, string(raw))

	var status struct {
		Project struct {
			Pending  int `json:"pending"`
			Required int `json:"required"`
		} `json:"project"`
		Shared struct {
			Pending int `json:"pending"`
		} `json:"shared"`
	}
	code, raw = ts.call(http.MethodGet,
		"/gateways/"+target+"/project-status?project="+defaultProjectID+"&from="+source, nil, &status)

	ts.Require().Equal(http.StatusOK, code, string(raw))
	ts.Zero(status.Project.Pending, "a change was counted as waiting on a gateway that runs nothing")
	ts.Zero(status.Project.Required)
	ts.Zero(status.Shared.Pending)
}
