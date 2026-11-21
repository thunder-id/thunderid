// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// ouTypeResponse is the part of an organization unit type the tests read.
type ouTypeResponse struct {
	ID     string `json:"id"`
	Handle string `json:"handle"`
	OUID   string `json:"ouId"`
}

// ouTypeListResponse is the part of an organization unit type list the tests read.
type ouTypeListResponse struct {
	Types []ouTypeResponse `json:"types"`
}

func (ts *ControlPlaneTestSuite) TestOUTypesAreManagedOnTheControlPlane() {
	handle := unique("cp-ou-type")
	request := map[string]any{
		"handle":      handle,
		"displayName": "Partner",
		"ouId":        ts.ouID,
		"schema": map[string]any{
			"registrationNo": map[string]any{"type": "string", "required": true, "unique": true},
		},
	}
	var created ouTypeResponse
	status, raw := ts.call(http.MethodPost, "/organization-unit-types", request, &created)
	ts.Require().Equal(http.StatusCreated, status, string(raw))
	ts.Require().NotEmpty(created.ID)
	deleted := false
	ts.T().Cleanup(func() {
		if !deleted {
			ts.call(http.MethodDelete, "/organization-unit-types/"+created.ID, nil, nil)
		}
	})
	ts.Equal(handle, created.Handle)
	ts.Equal(ts.ouID, created.OUID)

	var fetched ouTypeResponse
	status, raw = ts.call(http.MethodGet, "/organization-unit-types/"+created.ID, nil, &fetched)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	ts.Equal(created.ID, fetched.ID)

	var list ouTypeListResponse
	status, raw = ts.call(http.MethodGet, "/organization-unit-types?limit=100", nil, &list)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	ids := make([]string, 0, len(list.Types))
	for _, item := range list.Types {
		ids = append(ids, item.ID)
	}
	ts.Contains(ids, created.ID)

	status, raw = ts.call(http.MethodDelete, "/organization-unit-types/"+created.ID, nil, nil)
	ts.Require().Equal(http.StatusNoContent, status, string(raw))
	deleted = true

	status, _ = ts.call(http.MethodGet, "/organization-unit-types/"+created.ID, nil, nil)
	ts.Equal(http.StatusNotFound, status)
}

// An organization unit type the Control Plane deploys reaches the Data Plane with its organization unit,
// and is read-only there. A later version without them removes both.
//
// The Data Plane is shared with the suite's other tests, so each apply sends only the test's own unit and
// type, leaving the rest of the Data Plane alone.
func (ts *ControlPlaneTestSuite) TestOUTypesAreDeployedToTheDataPlane() {
	gatewayID := ts.registerDataPlane()
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: unique("deployed-ou-type"), Name: "Deployed organization unit type", ProjectID: defaultProjectID,
	})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteOrganizationUnit(ouID) }()
	handle := unique("deployed-ou-type")
	var created ouTypeResponse
	status, raw := ts.call(http.MethodPost, "/organization-unit-types", map[string]any{
		"handle": handle, "displayName": "Deployed", "ouId": ouID,
		"schema": map[string]any{"region": map[string]any{"type": "string"}},
	}, &created)
	ts.Require().Equal(http.StatusCreated, status, string(raw))
	typeDeleted := false
	defer func() {
		if !typeDeleted {
			ts.call(http.MethodDelete, "/organization-unit-types/"+created.ID, nil, nil)
		}
	}()
	selection := []string{"organization_unit/" + ouID, "ou_type/" + created.ID}

	var withType, withoutType struct {
		Version string `json:"version"`
	}
	status, raw = ts.call(http.MethodPost, "/configuration-versions", map[string]any{"note": "with the type"},
		&withType)
	ts.Require().Contains([]int{http.StatusCreated, http.StatusOK}, status, string(raw))
	status, raw = ts.call(http.MethodDelete, "/organization-unit-types/"+created.ID, nil, nil)
	ts.Require().Equal(http.StatusNoContent, status, string(raw))
	typeDeleted = true
	ts.Require().NoError(testutils.DeleteOrganizationUnit(ouID))
	status, raw = ts.call(http.MethodPost, "/configuration-versions", map[string]any{"note": "without the type"},
		&withoutType)
	ts.Require().Contains([]int{http.StatusCreated, http.StatusOK}, status, string(raw))

	var applied struct {
		Recorded bool `json:"recorded"`
		Import   any  `json:"import"`
	}
	status, raw = ts.call(http.MethodPost, "/gateways/"+gatewayID+"/apply",
		map[string]any{"version": withType.Version, "selection": selection}, &applied)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	ts.Require().True(applied.Recorded, "the apply was not recorded: %s", raw)
	held := ts.masterOf(gatewayID)
	ts.Zero(held.Summary.Failed, "the data plane refused something: %v", applied.Import)
	ts.Equal(2, held.Summary.Deployed)

	// The management key reaches only the import, so the Data Plane is read through it: a plain import
	// that changes the deployed type is refused, which shows the type is there and read-only.
	document := fmt.Sprintf("resource_type: ou_type\nid: %s\nhandle: %s\ndisplayName: Changed\nouId: %s\n"+
		"schema:\n  region:\n    type: string\n", created.ID, handle, ouID)
	outcome := ts.importToDataPlane(document)
	ts.Require().Len(outcome.Results, 1, "%+v", outcome.Results)
	ts.Equal("failed", outcome.Results[0].Status)
	ts.Equal("OUT-1011", outcome.Results[0].Code, "%+v", outcome.Results)

	status, raw = ts.call(http.MethodPost, "/gateways/"+gatewayID+"/apply",
		map[string]any{"version": withoutType.Version, "selection": selection}, &applied)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	held = ts.masterOf(gatewayID)
	ts.Zero(held.Summary.Failed, "the data plane refused something: %v", applied.Import)
	ts.Equal(2, held.Summary.Removed)
	status, raw = ts.toDataPlane(http.MethodPost, "/import", map[string]any{
		"content": document, "dryRun": true, "options": map[string]any{"target": "runtime"},
	})
	ts.Require().Equal(http.StatusOK, status, string(raw))
	var dryRun struct {
		Results []struct {
			Operation string `json:"operation"`
		} `json:"results"`
	}
	ts.Require().NoError(json.Unmarshal(raw, &dryRun), string(raw))
	ts.Require().Len(dryRun.Results, 1, string(raw))
	ts.Equal("create", dryRun.Results[0].Operation, "the data plane still holds the removed type: %s", raw)
}
