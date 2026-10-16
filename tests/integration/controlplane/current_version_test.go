// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"net/http"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// A change to the configuration is captured as the current version without anyone asking for a
// capture, and the next change updates that version rather than adding one.
func (ts *ControlPlaneTestSuite) TestAChangeIsCapturedAsTheCurrentVersion() {
	type listed struct {
		Version string `json:"version"`
		Current bool   `json:"current"`
	}
	type read struct {
		Resources string `json:"resources"`
	}
	// waitForCurrent waits for the newest version to be the current one and to hold what is asked.
	waitForCurrent := func(holds string) listed {
		ts.T().Helper()
		deadline := time.Now().Add(20 * time.Second)
		for {
			var versions []listed
			status, raw := ts.call(http.MethodGet, "/configuration-versions", nil, &versions)
			ts.Require().Equal(http.StatusOK, status, string(raw))
			if len(versions) > 0 && versions[0].Current {
				var newest read
				status, raw = ts.call(http.MethodGet, "/configuration-versions/"+versions[0].Version, nil, &newest)
				ts.Require().Equal(http.StatusOK, status, string(raw))
				if strings.Contains(newest.Resources, holds) {
					return versions[0]
				}
			}
			if time.Now().After(deadline) {
				ts.FailNow("the change was not captured as the current version", holds)
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
	count := func() int {
		ts.T().Helper()
		var versions []listed
		status, raw := ts.call(http.MethodGet, "/configuration-versions", nil, &versions)
		ts.Require().Equal(http.StatusOK, status, string(raw))
		return len(versions)
	}

	first, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: unique("current-first"), Name: "Current first", Parent: &ts.ouID,
	})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteOrganizationUnit(first) }()
	captured := waitForCurrent(first)
	kept := count()

	second, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: unique("current-second"), Name: "Current second", Parent: &ts.ouID,
	})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteOrganizationUnit(second) }()
	updated := waitForCurrent(second)

	ts.NotEqual(captured.Version, updated.Version)
	ts.Equal(kept, count(), "a change added a version rather than updating the current one")
}
