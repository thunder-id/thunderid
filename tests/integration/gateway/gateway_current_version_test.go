// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"fmt"
	"net/http"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

type listedVersion struct {
	Version string `json:"version"`
	Current bool   `json:"current"`
}

func (ts *GatewayVersionsTestSuite) captureCurrent() version {
	ts.T().Helper()
	var captured version
	status := ts.call(http.MethodPost, "/configuration-versions", map[string]any{"current": true}, &captured)
	ts.Require().Contains([]int{http.StatusCreated, http.StatusOK}, status)
	ts.Require().Len(captured.Version, 64, "a version is named by its SHA-256")
	return captured
}

func (ts *GatewayVersionsTestSuite) listVersions() []listedVersion {
	ts.T().Helper()
	var listed []listedVersion
	ts.Require().Equal(http.StatusOK, ts.call(http.MethodGet, "/configuration-versions", nil, &listed))
	return listed
}

func (ts *GatewayVersionsTestSuite) unit(name string) string {
	ts.T().Helper()
	id, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: fmt.Sprintf("current-%s-%d", name, time.Now().UnixNano()), Name: name,
	})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteOrganizationUnit(id) })
	return id
}

// A capture that keeps the configuration current updates one version in place until a gateway holds
// it; once it does, it stays as it was and the next change becomes a new current version.
func (ts *GatewayVersionsTestSuite) TestTheCurrentVersionIsKeptUpToDateUntilDeployed() {
	ts.unit("First")
	first := ts.captureCurrent()
	before := ts.listVersions()
	ts.Require().NotEmpty(before)
	ts.Equal(listedVersion{Version: first.Version, Current: true}, before[0])

	secondUnit := ts.unit("Second")
	second := ts.captureCurrent()
	ts.NotEqual(first.Version, second.Version)
	ts.Contains(second.Resources, secondUnit)
	after := ts.listVersions()
	ts.Len(after, len(before), "a change added a version rather than updating the current one")
	ts.Equal(listedVersion{Version: second.Version, Current: true}, after[0])
	for _, listed := range after {
		ts.NotEqual(first.Version, listed.Version, "the replaced version is still listed")
	}

	applied := ts.apply(second.Version, false)
	ts.True(applied.Recorded, "the apply was not recorded: %s", applied.Import)

	ts.unit("Third")
	third := ts.captureCurrent()
	ts.NotEqual(second.Version, third.Version)
	// The bound on kept versions may remove the oldest here, but never the one the gateway holds.
	deployed := ts.listVersions()
	ts.Require().GreaterOrEqual(len(deployed), 2)
	ts.Equal(listedVersion{Version: third.Version, Current: true}, deployed[0])
	ts.Equal(listedVersion{Version: second.Version}, deployed[1],
		"the version a gateway holds was replaced, or is still listed as current")
}
