// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"crypto/rand"
	"fmt"
	"net/http"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// The suite's deployment.yaml sets gateway.unused_value_grace_seconds to 1 and
// gateway.unused_value_sweep_interval_seconds to 0, so every apply sweeps the gateway's store and a
// value set more than a second ago can go.

// pastTheGrace is how long a value is left before an apply that may remove it: the data plane keeps
// UPDATED_AT in whole seconds and removes only a value it finds older than the one-second grace, so
// two seconds must have passed on its clock, with room for a slow machine.
const pastTheGrace = 3 * time.Second

// sweepWait bounds how long a test waits for the sweep an apply starts in the background, which
// exports the whole configuration before it removes anything.
const sweepWait = 90 * time.Second

// randomUUID is a random lower-case UUID, the form the server gives a resource's ID.
func randomUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// storedValue is a value the test sets on the data plane's store, by collection and name.
type storedValue struct {
	collection, name string
}

func (v storedValue) String() string { return v.collection + "/" + v.name }

// setOnDataPlane sets a value in the data plane's store and removes it again when the test ends.
func (ts *ControlPlaneTestSuite) setOnDataPlane(v storedValue, value string) {
	ts.T().Helper()
	status, raw := ts.toDataPlane(http.MethodPut, "/"+v.collection+"/"+v.name, map[string]any{"value": value})
	ts.Require().Less(status, http.StatusBadRequest, "%s: %s", v, raw)
	ts.T().Cleanup(func() { ts.toDataPlane(http.MethodDelete, "/"+v.collection+"/"+v.name, nil) })
}

// heldOnDataPlane reports whether the data plane's store holds the value.
func (ts *ControlPlaneTestSuite) heldOnDataPlane(v storedValue) bool {
	ts.T().Helper()
	status, raw := ts.toDataPlane(http.MethodGet, "/"+v.collection+"/"+v.name, nil)
	ts.Require().Contains([]int{http.StatusOK, http.StatusNotFound}, status, "%s: %s", v, raw)
	return status == http.StatusOK
}

// goneSoon waits for the data plane's store to stop holding every value, and reports whether it did.
func (ts *ControlPlaneTestSuite) goneSoon(values ...storedValue) bool {
	ts.T().Helper()
	deadline := time.Now().Add(sweepWait)
	for {
		held := false
		for _, v := range values {
			if ts.heldOnDataPlane(v) {
				held = true
				break
			}
		}
		if !held {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(250 * time.Millisecond)
	}
}

// applySelection captures a version and applies the selected resources of it to the gateway, answering
// whether the apply was recorded. With withDependencies, what the selection needs, such as the
// application's flows, is sent with it; without, only the selection is, as a removal wants.
func (ts *ControlPlaneTestSuite) applySelection(gatewayID string, dryRun, withDependencies bool,
	keys ...string) bool {
	ts.T().Helper()
	status, raw := ts.call(http.MethodPost, "/configuration-versions", map[string]any{"note": "unused values"}, nil)
	ts.Require().Contains([]int{http.StatusCreated, http.StatusOK}, status, string(raw))
	var applied struct {
		Recorded bool `json:"recorded"`
	}
	status, raw = ts.call(http.MethodPost, "/gateways/"+gatewayID+"/apply", map[string]any{
		"selection": keys, "withDependencies": withDependencies, "dryRun": dryRun,
	}, &applied)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	return applied.Recorded
}

// An apply to the Data Plane removes from its store the values the Control Plane named for a resource
// that no longer exists, variable and secret alike, once they are older than the grace. It keeps a
// value an administrator named, the values of the application it deployed, and those of an
// application that exists but was never deployed. A dry run removes nothing.
func (ts *ControlPlaneTestSuite) TestAnApplyRemovesTheUnusedValuesFromTheDataPlanesStore() {
	gatewayID := ts.registerDataPlane()

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: unique("swept"), Name: "Swept on the data plane", ProjectID: defaultProjectID,
	})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteOrganizationUnit(ouID) })
	deployedID, err := testutils.CreateApplication(testutils.Application{
		OUID:         ouID,
		Name:         unique("Swept Deployed App"),
		ClientID:     unique("swept-deployed-client"),
		ClientSecret: unique("swept-deployed-secret"),
		RedirectURIs: []string{"https://swept-deployed.example.com/callback"},
	})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteApplication(deployedID) })
	undeployedID, err := testutils.CreateApplication(testutils.Application{
		OUID:         ts.ouID,
		Name:         unique("Swept Undeployed App"),
		ClientID:     unique("swept-undeployed-client"),
		ClientSecret: unique("swept-undeployed-secret"),
		RedirectURIs: []string{"https://swept-undeployed.example.com/callback"},
	})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteApplication(undeployedID) })

	// A resource that never existed here, as one deleted long ago leaves its values behind.
	goneID := randomUUID()
	unused := []storedValue{
		{"variables", variableName("APPLICATION", goneID, "CLIENT_ID")},
		{"secrets", variableName("APPLICATION", goneID, "CLIENT_SECRET")},
	}
	handNamed := storedValue{"variables", fmt.Sprintf("SWEEP_HAND_NAMED_%d", time.Now().UnixNano())}
	deployed := []storedValue{
		{"variables", variableName("APPLICATION", deployedID, "CLIENT_ID")},
		{"secrets", variableName("APPLICATION", deployedID, "CLIENT_SECRET")},
		{"variables", variableName("APPLICATION", deployedID, "REDIRECT_URIS")},
	}
	undeployed := []storedValue{
		{"variables", variableName("APPLICATION", undeployedID, "CLIENT_ID")},
		{"secrets", variableName("APPLICATION", undeployedID, "CLIENT_SECRET")},
	}
	for _, v := range append(append([]storedValue{handNamed}, unused...), undeployed...) {
		ts.setOnDataPlane(v, unique("value"))
	}
	ts.setOnDataPlane(deployed[0], unique("swept-dp-client"))
	ts.setOnDataPlane(deployed[1], unique("swept-dp-secret"))
	ts.setOnDataPlane(deployed[2], `["https://swept-dp.example.com/callback"]`)
	time.Sleep(pastTheGrace)

	keys := []string{"organization_unit/" + ouID, "application/" + deployedID}
	// A dry run records nothing, so it starts no sweep.
	// The application needs its flows, which both planes hold from their bootstrap, so they go with it.
	ts.False(ts.applySelection(gatewayID, true, true, keys...), "a dry run was recorded")
	time.Sleep(pastTheGrace)
	for _, v := range unused {
		ts.True(ts.heldOnDataPlane(v), "a dry run removed %s", v)
	}

	ts.T().Cleanup(func() {
		// Take the deployed resources off the shared data plane again.
		_ = testutils.DeleteApplication(deployedID)
		_ = testutils.DeleteOrganizationUnit(ouID)
		// Only the two removals, so the flows sent with the application stay as every other test needs them.
		// Best effort: a deploy that failed left nothing to remove.
		ts.call(http.MethodPost, "/configuration-versions", map[string]any{"note": "unused values removed"}, nil)
		ts.call(http.MethodPost, "/gateways/"+gatewayID+"/apply",
			map[string]any{"selection": keys, "withDependencies": false}, nil)
	})
	ts.Require().True(ts.applySelection(gatewayID, false, true, keys...), "the apply was not recorded")
	held := ts.masterOf(gatewayID)
	ts.Require().Zero(held.Summary.Failed, "the data plane refused the deployed application")

	ts.Require().True(ts.goneSoon(unused...), "the unused values were not removed from the data plane")
	ts.True(ts.heldOnDataPlane(handNamed), "a value an administrator named was removed")
	for _, v := range deployed {
		ts.True(ts.heldOnDataPlane(v), "a value of the deployed application was removed: %s", v)
	}
	for _, v := range undeployed {
		ts.True(ts.heldOnDataPlane(v), "a value of an application that exists was removed: %s", v)
	}
}
