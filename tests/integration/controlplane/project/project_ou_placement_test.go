// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// A resource in a project is placed in the system organization unit or in an organization unit of the project,
// and in no organization unit of another project.
func (ts *ProjectAPITestSuite) TestProjectResourcesInProjectOrganizationUnits() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("place-shop")))
	root := ts.createOU(ts.ouBody("place-root", nil, shop.ID))
	child := ts.createOU(ts.ouBody("place-child", &root.ID, shop.ID))
	otherOUID := ts.createRootOU("place-other")

	app := ts.createApp(ts.appBody("place", `"projectId":%q,"ouId":%q`, shop.ID, child.ID))
	ts.Equal(child.ID, app.OUID)
	role := ts.createRole(ts.roleBody("place", `"projectId":%q,"ouId":%q`, shop.ID, root.ID))
	ts.Equal(root.ID, role.OUID)
	group := ts.createGroup(ts.groupBody("place", `"projectId":%q,"ouId":%q`, shop.ID, child.ID))
	ts.Equal(child.ID, group.OUID)
	server := ts.createResourceServer(ts.resourceServerBody("place", `"projectId":%q,"ouId":%q`, shop.ID, child.ID))
	ts.Equal(child.ID, server.OUID)

	const outside = `"projectId":%q,"ouId":%q`
	for path, tc := range map[string]struct{ body, code string }{
		"/applications":     {ts.appBody("place-out", outside, shop.ID, otherOUID), "APP-1064"},
		"/roles":            {ts.roleBody("place-out", outside, shop.ID, otherOUID), "ROL-1041"},
		"/groups":           {ts.groupBody("place-out", outside, shop.ID, otherOUID), "GRP-1022"},
		"/resource-servers": {ts.resourceServerBody("place-out", outside, shop.ID, otherOUID), "RES-1026"},
	} {
		status, body := ts.do(http.MethodPost, path, tc.body)
		ts.Equal(http.StatusBadRequest, status, path, body)
		ts.Equal(tc.code, codeOf(ts.T(), body), path)
	}
}

// An organization unit is not deleted while it holds a resource, of any type. The units sit under the system
// organization unit, where the default agent type allows agents too.
func (ts *ProjectAPITestSuite) TestOUHoldingResourcesIsNotDeleted() {
	systemOUID := defaultOUID
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("hold-shop")))
	const placed = `"projectId":%q,"ouId":%q`
	for name, place := range map[string]func(ouID string){
		"application": func(ouID string) {
			ts.createApp(ts.appBody("hold", `"projectId":%q,"ouId":%q`, shop.ID, ouID))
		},
		"role": func(ouID string) {
			ts.createRole(ts.roleBody("hold", `"projectId":%q,"ouId":%q`, shop.ID, ouID))
		},
		"resource server": func(ouID string) {
			ts.createResourceServer(ts.resourceServerBody("hold", `"projectId":%q,"ouId":%q`, shop.ID, ouID))
		},
		"agent": func(ouID string) {
			ts.createAgent(ts.agentBody("hold", placed, shop.ID, ouID))
		},
		"email template": func(ouID string) {
			ts.createTemplate(ts.templateBody("hold", placed, shop.ID, ouID))
		},
		"sms template": func(ouID string) {
			ts.createSMSTemplate("hold", placed, shop.ID, ouID)
		},
		"credential configuration": func(ouID string) {
			ts.createConfiguration(ts.configurationBody("hold", placed, shop.ID, ouID))
		},
		"presentation definition": func(ouID string) {
			ts.createDefinition(ts.definitionBody("hold", placed, shop.ID, ouID))
		},
	} {
		unit := ts.createOU(ts.ouBody("hold-"+name, &systemOUID, shop.ID))
		place(unit.ID)

		status, body := ts.do(http.MethodDelete, "/organization-units/"+unit.ID, "")
		ts.Equal(http.StatusBadRequest, status, name, body)
		ts.Equal("OU-1006", codeOf(ts.T(), body), name)
	}
}

// An organization unit moves to another project only while neither it nor a unit beneath it holds a resource.
func (ts *ProjectAPITestSuite) TestOUMoveWithResourcesIsRefused() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("move-res-shop")))
	other := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Other"}`, ts.handle("move-res-other")))
	root := ts.createOU(ts.ouBody("move-res-root", nil, shop.ID))
	child := ts.createOU(ts.ouBody("move-res-child", &root.ID, shop.ID))
	role := ts.createRole(ts.roleBody("move-res", `"projectId":%q,"ouId":%q`, shop.ID, child.ID))

	status, body := ts.do(http.MethodPut, "/organization-units/"+root.ID, ts.ouBody("move-res-root", nil, other.ID))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("OU-1027", codeOf(ts.T(), body))

	var unmoved projectOU
	ts.Require().Equal(http.StatusOK, ts.getJSON("/organization-units/"+child.ID, &unmoved))
	ts.Equal(shop.ID, unmoved.ProjectID)

	ts.Require().NoError(testutils.DeleteRole(role.ID))
	moved := ts.updateOU(root.ID, ts.ouBody("move-res-root", nil, other.ID))
	ts.Equal(other.ID, moved.ProjectID)
}

// A placement listing names where a project's resources can go: the system organization unit, then the project's
// top organization units, and no other project's.
func (ts *ProjectAPITestSuite) TestPlacementListing() {
	const unknownProjectID = "019a0000-0000-7000-8000-00000000dead"
	systemOUID := defaultOUID
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("placement-shop")))
	other := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Other"}`, ts.handle("placement-other")))
	root := ts.createOU(ts.ouBody("placement-root", nil, shop.ID))
	ts.createOU(ts.ouBody("placement-child", &root.ID, shop.ID))
	underSystem := ts.createOU(ts.ouBody("placement-system-child", &systemOUID, shop.ID))
	otherRoot := ts.createOU(ts.ouBody("placement-other-root", nil, other.ID))
	placement := "?limit=100&placement=true&projectId=" + url.QueryEscape(shop.ID)

	list := ts.listOUs(placement)
	ts.Require().NotEmpty(list.OrganizationUnits)
	ts.Equal(systemOUID, list.OrganizationUnits[0].ID)
	ts.True(list.OrganizationUnits[0].IsSystem)
	ts.ElementsMatch([]string{root.ID, underSystem.ID}, ts.ouIDs(list)[1:])
	ts.Equal(3, list.TotalResults)
	for _, unit := range list.OrganizationUnits[1:] {
		ts.False(unit.IsSystem)
		ts.Equal(shop.ID, unit.ProjectID)
	}

	firstPage := ts.listOUs("?limit=1&placement=true&projectId=" + url.QueryEscape(shop.ID))
	ts.Equal([]string{systemOUID}, ts.ouIDs(firstPage))
	ts.Equal(3, firstPage.TotalResults)

	ts.Empty(ts.listOUs("?placement=true&projectId=" + unknownProjectID).OrganizationUnits)

	// Within the system organization unit: it and the project's units placed under it, not the project's roots.
	withinSystem := ts.listOUs(placement + "&within=" + systemOUID)
	ts.Equal([]string{systemOUID, underSystem.ID}, ts.ouIDs(withinSystem))
	// Each placement names its parent, so the Console nests the units under the system one below it.
	ts.Empty(withinSystem.OrganizationUnits[0].Parent)
	ts.Equal(systemOUID, withinSystem.OrganizationUnits[1].Parent)
	ts.Equal(2, withinSystem.TotalResults)
	// Within a unit of the project: that unit alone.
	ts.Equal([]string{root.ID}, ts.ouIDs(ts.listOUs(placement+"&within="+root.ID)))
	// Within another project's unit: none.
	ts.Empty(ts.listOUs(placement + "&within=" + otherRoot.ID).OrganizationUnits)

	status, body := ts.do(http.MethodGet, "/organization-units"+placement+"&filter="+
		url.QueryEscape(`name eq "x"`), "")
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("OU-1028", codeOf(ts.T(), body))
}
