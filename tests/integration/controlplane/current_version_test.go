// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// listed is a version as the versions listing reports it.
type listed struct {
	Version string `json:"version"`
	Current bool   `json:"current"`
}

// waitForCurrent waits for the newest version to be the current one and to hold what is asked.
func (ts *ControlPlaneTestSuite) waitForCurrent(holds string) listed {
	ts.T().Helper()
	type read struct {
		Resources string `json:"resources"`
	}
	deadline := time.Now().Add(20 * time.Second)
	for {
		var versions []listed
		status, raw := ts.call(http.MethodGet, "/configuration-versions", nil, &versions)
		ts.Require().Equal(http.StatusOK, status, string(raw))
		if len(versions) > 0 && versions[0].Current {
			// The current version is replaced in place by each later change, which gives it a new hash,
			// so the one just listed can be gone by the time it is read; it is then listed again.
			var newest read
			status, raw = ts.call(http.MethodGet, "/configuration-versions/"+versions[0].Version, nil, &newest)
			if status != http.StatusNotFound {
				ts.Require().Equal(http.StatusOK, status, string(raw))
				if strings.Contains(newest.Resources, holds) {
					return versions[0]
				}
			}
		}
		if time.Now().After(deadline) {
			ts.FailNow("the change was not captured as the current version", holds)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// A change to the configuration is captured as the current version without anyone asking for a
// capture, and the next change updates that version rather than adding one.
func (ts *ControlPlaneTestSuite) TestAChangeIsCapturedAsTheCurrentVersion() {
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
	captured := ts.waitForCurrent(first)
	kept := count()

	second, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: unique("current-second"), Name: "Current second", Parent: &ts.ouID,
	})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteOrganizationUnit(second) }()
	updated := ts.waitForCurrent(second)

	ts.NotEqual(captured.Version, updated.Version)
	ts.Equal(kept, count(), "a change added a version rather than updating the current one")
}

// The current version names the user whose change it captured: by the id the trusted issuer's token
// carries, and by the name the issuer's user endpoint reports for that id, which the control plane
// looks up rather than stores.
func (ts *ControlPlaneTestSuite) TestTheCurrentVersionNamesWhoseChangeItCaptured() {
	type actor struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	var got struct {
		CreatedBy *actor `json:"createdBy"`
	}

	named, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: unique("actor-named"), Name: "Actor named", Parent: &ts.ouID,
	})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteOrganizationUnit(named) }()
	version := ts.waitForCurrent(named).Version

	status, raw := ts.call(http.MethodGet, "/configuration-versions/"+version, nil, &got)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	ts.Require().NotNil(got.CreatedBy, "the version does not name whose change it captured")
	ts.Equal(testutils.TrustedIssuerAdminSubject, got.CreatedBy.ID)
	ts.Equal(testutils.TrustedIssuerAdminName, got.CreatedBy.Name, "the name was not looked up at the issuer")
}

// A user the issuer signs tokens for but does not know is left out of the version and of the
// listing: their id alone is not given out.
func (ts *ControlPlaneTestSuite) TestAUserTheIssuerDoesNotKnowIsLeftOut() {
	type named struct {
		Version   string          `json:"version"`
		CreatedBy json.RawMessage `json:"createdBy"`
	}
	stranger, ok := testutils.PersonaClient(testutils.PersonaStranger)
	ts.Require().True(ok, "the harness provides a token for a user the issuer does not know")

	var unit testutils.OrganizationUnit
	status, raw := ts.send(stranger, http.MethodPost, "/organization-units",
		"application/json", []byte(`{"handle":"`+unique("actor-stranger")+`","name":"Actor stranger","parent":"`+
			ts.ouID+`"}`))
	ts.Require().Equal(http.StatusCreated, status, string(raw))
	ts.Require().NoError(json.Unmarshal(raw, &unit))
	defer func() { _ = testutils.DeleteOrganizationUnit(unit.ID) }()
	version := ts.waitForCurrent(unit.ID).Version

	var got named
	status, raw = ts.call(http.MethodGet, "/configuration-versions/"+version, nil, &got)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	ts.Nil(got.CreatedBy, "a user the issuer does not know was given out: %s", string(got.CreatedBy))

	var versions []named
	status, raw = ts.call(http.MethodGet, "/configuration-versions", nil, &versions)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	for _, listed := range versions {
		if listed.Version == version {
			ts.Nil(listed.CreatedBy, "the listing gave out a user the issuer does not know: %s", string(listed.CreatedBy))
			return
		}
	}
	ts.Fail("the version left the listing", "%s", string(raw))
}
