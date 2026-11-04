// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// startingGateway is a stand-in data plane whose readiness check fails until it is told it is up.
type startingGateway struct {
	server *httptest.Server
	up     atomic.Bool
}

func newStartingGateway() *startingGateway {
	g := &startingGateway{}
	g.server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health/readiness" && g.up.Load() {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	return g
}

type dataPlaneStatus struct {
	State         string  `json:"state"`
	ProvisionedAt *string `json:"provisionedAt"`
	CheckedAt     *string `json:"checkedAt"`
}

func (ts *ControlPlaneTestSuite) dataPlaneStatus() dataPlaneStatus {
	ts.T().Helper()
	var status dataPlaneStatus
	code, raw := ts.call(http.MethodGet, "/gateways/default/status", nil, &status)
	ts.Require().Equal(http.StatusOK, code, string(raw))
	return status
}

// createApplication asks for an application and answers with the status and body, without failing the
// test on a refusal.
func (ts *ControlPlaneTestSuite) createApplication() (int, string) {
	ts.T().Helper()
	code, raw := ts.call(http.MethodPost, "/applications", map[string]any{
		"name": unique("Readiness App"), "ouId": ts.ouID,
	}, nil)
	return code, string(raw)
}

// Until a data plane is registered and has answered its health check, a create whose values go to it
// is refused, as is a deploy to it; once it answers, both go ahead.
func (ts *ControlPlaneTestSuite) TestCreatesWaitForTheDataPlane() {
	// With no default gateway, there is no data plane to wait for.
	ts.Require().NoError(testutils.DeleteGateway(ts.dataPlaneID))
	ts.dataPlaneID = ""
	ts.Equal("pending", ts.dataPlaneStatus().State)
	code, raw := ts.createApplication()
	ts.Equal(http.StatusConflict, code, raw)
	ts.Contains(raw, "GTW-1038")

	// A default gateway that does not answer yet is being provisioned.
	g := newStartingGateway()
	defer g.server.Close()
	certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: g.server.Certificate().Raw})
	var registered struct {
		ID string `json:"id"`
	}
	code, rawBytes := ts.call(http.MethodPost, "/gateways", map[string]any{
		"name": unique("starting-gateway"), "baseUrl": g.server.URL, "key": captureGatewayKey,
		"caCertificate": string(certificate), "isDefault": true,
	}, &registered)
	ts.Require().Equal(http.StatusCreated, code, string(rawBytes))
	defer ts.call(http.MethodDelete, "/gateways/"+registered.ID, nil, nil)

	ts.Equal("provisioning", ts.dataPlaneStatus().State)
	code, raw = ts.createApplication()
	ts.Equal(http.StatusConflict, code, raw)
	ts.Contains(raw, "GTW-1034")
	code, rawBytes = ts.call(http.MethodPost, "/gateways/"+registered.ID+"/apply",
		map[string]any{"dryRun": true}, nil)
	ts.Equal(http.StatusConflict, code, string(rawBytes))
	ts.Contains(string(rawBytes), "GTW-1034")

	// Once it answers, a change to its registration checks it at once, and creates go ahead.
	g.up.Store(true)
	code, rawBytes = ts.call(http.MethodPut, "/gateways/"+registered.ID,
		map[string]any{"name": unique("started-gateway")}, nil)
	ts.Require().Equal(http.StatusOK, code, string(rawBytes))
	ts.Eventually(func() bool { return ts.dataPlaneStatus().State == "ready" }, 10*time.Second,
		200*time.Millisecond, "the data plane was not seen to be ready")
	status := ts.dataPlaneStatus()
	ts.NotNil(status.ProvisionedAt)
	ts.NotNil(status.CheckedAt)

	code, raw = ts.createApplication()
	ts.Equal(http.StatusCreated, code, raw)
	if code == http.StatusCreated {
		var created struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal([]byte(raw), &created)
		if created.ID != "" {
			_ = testutils.DeleteApplication(created.ID)
		}
	}
}

// The status names no gateway, so any signed-in user can read it.
func (ts *ControlPlaneTestSuite) TestTheDataPlaneStatusNamesNoGateway() {
	code, raw := ts.call(http.MethodGet, "/gateways/default/status", nil, nil)

	ts.Require().Equal(http.StatusOK, code, string(raw))
	ts.False(strings.Contains(string(raw), testutils.DataPlaneURL), "the status disclosed the gateway's address")
	ts.False(strings.Contains(string(raw), ts.dataPlaneID), "the status disclosed the gateway's id")
}
