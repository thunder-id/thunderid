// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"encoding/pem"
	"net/http"
	"net/http/httptest"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// gatewayConfigWith returns the control plane's gateway section with auto_apply set, keeping every
// other key as it is.
func gatewayConfigWith(current map[string]interface{}, autoApply bool) map[string]interface{} {
	gateway := map[string]interface{}{}
	for key, value := range current {
		gateway[key] = value
	}
	gateway["auto_apply"] = autoApply
	return map[string]interface{}{"gateway": gateway}
}

// registerDefaultGateway registers a stand-in as this control plane's default gateway for one test.
// A gateway registered earlier stops being the default.
func (ts *ControlPlaneTestSuite) registerDefaultGateway(server *httptest.Server) string {
	ts.T().Helper()
	certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	var registered struct {
		ID        string `json:"id"`
		IsDefault bool   `json:"isDefault"`
	}
	status, raw := ts.call(http.MethodPost, "/gateways", map[string]any{
		"name":          unique("auto-apply-gateway"),
		"baseUrl":       server.URL,
		"key":           captureGatewayKey,
		"caCertificate": string(certificate),
		"isDefault":     true,
	}, &registered)
	ts.Require().Equal(http.StatusCreated, status, string(raw))
	ts.Require().True(registered.IsDefault, "the gateway is not the default")
	ts.T().Cleanup(func() { ts.call(http.MethodDelete, "/gateways/"+registered.ID, nil, nil) })
	return registered.ID
}

// With gateway.auto_apply on, a change written on the control plane reaches its default gateway
// without anyone applying a version: the control plane applies the latest one once it has captured
// it. The option is off by default, so the test turns it on, restarts the control plane, and puts the
// default configuration back afterwards so the other tests run as they did.
func (ts *ControlPlaneTestSuite) TestAChangeIsAppliedToTheDefaultGatewayWhenAutoApplyIsOn() {
	current, err := testutils.ReadDeploymentConfigKey("gateway")
	ts.Require().NoError(err)
	original, ok := current.(map[string]interface{})
	ts.Require().True(ok, "the control plane's deployment.yaml has no gateway section")

	ts.Require().NoError(testutils.PatchDeploymentConfig(gatewayConfigWith(original, true)),
		"Failed to turn gateway.auto_apply on")
	// Registered before the restart, so a restart that fails still puts the shared configuration back.
	// Restore steps use non-fatal assertions so a failure in one still lets the rest run, leaving the
	// shared control plane in the default state for the other tests.
	defer func() {
		ts.Assert().NoError(testutils.PatchDeploymentConfig(gatewayConfigWith(original, false)),
			"Failed to restore gateway.auto_apply")
		ts.Assert().NoError(testutils.RestartServer(), "Failed to restart the control plane after the restore")
	}()
	ts.Require().NoError(testutils.RestartServer(), "Failed to restart the control plane with auto apply on")

	// The first gateway registered becomes the default, so this one is registered first and then
	// loses that to the next: only the default gateway is applied to.
	other := newApplyGateway(0)
	ts.T().Cleanup(other.server.Close)
	ts.registerGateway(other.server)
	g := newApplyGateway(0)
	ts.T().Cleanup(g.server.Close)
	ts.registerDefaultGateway(g.server)

	ts.createResourceServer(unique("CP Auto Apply Orders"), "https://"+unique("cp-auto-apply")+".example.com")

	ts.True(g.importsSoon(1), "the control plane did not apply the change to its default gateway")
	other.mu.Lock()
	ts.Zero(other.imports, "the control plane applied the change to a gateway that is not the default")
	other.mu.Unlock()

	// A gateway that refuses the version does not fail the write, which is already done by the time the
	// apply runs. The control plane still tried it, and the resource server it was asked to write exists.
	refusing := newRefusingApplyGateway()
	ts.T().Cleanup(refusing.server.Close)
	ts.registerDefaultGateway(refusing.server)

	refusedID := ts.createResourceServer(unique("CP Auto Apply Refused"),
		"https://"+unique("cp-auto-apply-refused")+".example.com")

	ts.True(refusing.importsSoon(1), "the control plane did not try to apply the change to its default gateway")
	status, raw := ts.call(http.MethodGet, "/resource-servers/"+refusedID, nil, nil)
	ts.Equal(http.StatusOK, status, string(raw))
}
