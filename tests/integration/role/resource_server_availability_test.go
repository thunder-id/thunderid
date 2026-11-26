// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// A resource server is available in an organization unit that owns it or that one of its sharing policies reaches.
// Only there can a role hold its permissions, a token carry them, or an assignment confer them.
const (
	errRoleResourceServerNotAvailable = "ROL-1047"
	errResourceServerUsedInOU         = "RES-1039"
	errSharedResourceServerCannotMove = "RES-1037"
	availabilityPermission            = "orders-read"
	// systemResourceServerID is the bootstrap System resource server, which declares org.
	systemResourceServerID = "01900000-0000-7000-8000-000000000020"
)

// availabilityFixture is a parent organization unit owning a resource server, with two child units the resource
// server is not shared with yet.
type availabilityFixture struct {
	parentOUID       string
	childAOUID       string
	childBOUID       string
	resourceServerID string
	identifier       string
}

// newAvailabilityFixture creates the fixture's organization units and resource server, named after name, and removes
// them when the test ends.
func (suite *RoleAPITestSuite) newAvailabilityFixture(name string) availabilityFixture {
	suite.T().Helper()
	parentID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{Handle: name, Name: name})
	suite.Require().NoError(err)
	suite.T().Cleanup(func() { _ = testutils.DeleteOrganizationUnit(parentID) })
	child := func(suffix string) string {
		id, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
			Handle: name + "-" + suffix, Name: name + " " + suffix, Parent: &parentID})
		suite.Require().NoError(err)
		suite.T().Cleanup(func() { _ = testutils.DeleteOrganizationUnit(id) })
		return id
	}
	childA, childB := child("a"), child("b")

	identifier := "https://api.example.com/" + name
	resourceServerID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name: name + " API", Identifier: identifier, OUID: parentID,
	}, []testutils.Action{{Name: "Read orders", Handle: availabilityPermission}})
	suite.Require().NoError(err)
	suite.T().Cleanup(func() { _ = testutils.DeleteResourceServerWithChildren(resourceServerID) })

	return availabilityFixture{parentOUID: parentID, childAOUID: childA, childBOUID: childB,
		resourceServerID: resourceServerID, identifier: identifier}
}

// shareResourceServer shares the fixture's resource server with one child unit and returns the policy id.
func (suite *RoleAPITestSuite) shareResourceServer(f availabilityFixture, ouID string) string {
	suite.T().Helper()
	resp := suite.doRoleRequest(http.MethodPost, "/resource-servers/"+f.resourceServerID+"/sharing-policies",
		mustMarshal(suite.T(), map[string]interface{}{"childUnits": map[string]interface{}{
			"mode": "selected", "units": []map[string]string{{"ouId": ouID, "reach": "unit"}},
		}}))
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	suite.Require().Equal(http.StatusCreated, resp.StatusCode, "sharing the resource server: %s", string(body))
	var policy struct {
		ID string `json:"id"`
	}
	suite.Require().NoError(json.Unmarshal(body, &policy))
	return policy.ID
}

func (f availabilityFixture) permissions() []ResourcePermissions {
	return []ResourcePermissions{{ResourceServerID: f.resourceServerID, Permissions: []string{availabilityPermission}}}
}

// A role can hold permissions only of resource servers its organization unit owns or has shared with it, on create, on
// update and when it moves.
func (suite *RoleAPITestSuite) TestResourceServerAvailability_RoleWrites() {
	f := suite.newAvailabilityFixture("rs-avail-role-writes")

	owned, err := suite.createRole(CreateRoleRequest{Name: "Owner Role", OUID: f.parentOUID,
		Permissions: f.permissions()})
	suite.Require().NoError(err, "the owner may use its own resource server")
	defer suite.deleteRole(owned.ID)

	_, err = suite.createRole(CreateRoleRequest{Name: "Unshared Role", OUID: f.childAOUID,
		Permissions: f.permissions()})
	suite.Require().Error(err)
	suite.Contains(err.Error(), errRoleResourceServerNotAvailable, "child A has not been shared the resource server")

	suite.shareResourceServer(f, f.childAOUID)

	shared, err := suite.createRole(CreateRoleRequest{Name: "Shared Role", OUID: f.childAOUID,
		Permissions: f.permissions()})
	suite.Require().NoError(err, "child A has been shared the resource server")
	defer suite.deleteRole(shared.ID)

	_, err = suite.createRole(CreateRoleRequest{Name: "Sibling Role", OUID: f.childBOUID,
		Permissions: f.permissions()})
	suite.Require().Error(err)
	suite.Contains(err.Error(), errRoleResourceServerNotAvailable, "sharing with child A does not reach child B")

	_, err = suite.updateRole(shared.ID, UpdateRoleRequest{Name: "Shared Role", OUID: f.childBOUID,
		Permissions: f.permissions()})
	suite.Require().Error(err)
	suite.Contains(err.Error(), errRoleResourceServerNotAvailable,
		"a role cannot move to a unit that may not use its resource servers")

	_, err = suite.updateRole(shared.ID, UpdateRoleRequest{Name: "Shared Role", OUID: f.childAOUID,
		Permissions: f.permissions()})
	suite.NoError(err, "an update that stays where the resource server is shared")
}

// A sharing change cannot take a resource server from an organization unit whose roles still hold its permissions.
func (suite *RoleAPITestSuite) TestResourceServerAvailability_NarrowingRefusedWhileUsed() {
	f := suite.newAvailabilityFixture("rs-avail-narrowing")
	policyID := suite.shareResourceServer(f, f.childAOUID)
	role, err := suite.createRole(CreateRoleRequest{Name: "Child Role", OUID: f.childAOUID,
		Permissions: f.permissions()})
	suite.Require().NoError(err)
	defer suite.deleteRole(role.ID)

	policyPath := "/resource-servers/" + f.resourceServerID + "/sharing-policies/" + policyID
	resp := suite.doRoleRequest(http.MethodDelete, policyPath, nil)
	suite.Equal(http.StatusBadRequest, resp.StatusCode)
	suite.Equal(errResourceServerUsedInOU, decodeErrorCode(suite.T(), resp))
	resp.Body.Close()

	_, err = suite.updateRole(role.ID, UpdateRoleRequest{Name: "Child Role", OUID: f.childAOUID,
		Permissions: []ResourcePermissions{}})
	suite.Require().NoError(err)

	resp = suite.doRoleRequest(http.MethodDelete, policyPath, nil)
	defer resp.Body.Close()
	suite.Equal(http.StatusNoContent, resp.StatusCode, "nothing in child A uses the resource server any more")
}

// Narrowing a policy through an update takes the resource server from the units it stops reaching, and is refused the
// same way as a delete while their roles hold its permissions.
func (suite *RoleAPITestSuite) TestResourceServerAvailability_NarrowingByUpdateRefusedWhileUsed() {
	f := suite.newAvailabilityFixture("rs-avail-narrow-update")
	policyID := suite.shareResourceServer(f, f.childAOUID)
	role, err := suite.createRole(CreateRoleRequest{Name: "Child Role", OUID: f.childAOUID,
		Permissions: f.permissions()})
	suite.Require().NoError(err)
	defer suite.deleteRole(role.ID)

	policyPath := "/resource-servers/" + f.resourceServerID + "/sharing-policies/" + policyID
	update := func() *http.Response {
		suite.T().Helper()
		resp := suite.doRoleRequest(http.MethodGet, policyPath, nil)
		var policy struct {
			Version int `json:"version"`
		}
		suite.Require().NoError(json.NewDecoder(resp.Body).Decode(&policy))
		resp.Body.Close()
		return suite.doRoleRequest(http.MethodPut, policyPath, mustMarshal(suite.T(), map[string]interface{}{
			"version": policy.Version,
			"childUnits": map[string]interface{}{
				"mode": "selected", "units": []map[string]string{{"ouId": f.childBOUID, "reach": "unit"}},
			},
		}))
	}

	resp := update()
	suite.Equal(http.StatusBadRequest, resp.StatusCode)
	suite.Equal(errResourceServerUsedInOU, decodeErrorCode(suite.T(), resp))
	resp.Body.Close()

	_, err = suite.updateRole(role.ID, UpdateRoleRequest{Name: "Child Role", OUID: f.childAOUID,
		Permissions: []ResourcePermissions{}})
	suite.Require().NoError(err)

	resp = update()
	defer resp.Body.Close()
	suite.Equal(http.StatusOK, resp.StatusCode, "nothing in child A uses the resource server any more")
}

// An imported role in an organization unit is checked like a created one: refused while the unit cannot use its
// resource server, and imported once the resource server is shared with the unit.
func (suite *RoleAPITestSuite) TestResourceServerAvailability_ImportedRoleInAChildUnit() {
	f := suite.newAvailabilityFixture("rs-avail-import")
	roleID := newRoleID(suite)
	suite.T().Cleanup(func() { suite.deleteRole(roleID) })
	document := "resource_type: role\nid: " + roleID + "\nname: Imported Child Role\nouId: " + f.childAOUID +
		"\npermissions:\n  - resourceServerId: " + f.resourceServerID + "\n    permissions:\n      - " +
		availabilityPermission + "\n"
	importRole := func() testutils.ImportItem {
		suite.T().Helper()
		resp := suite.doRoleRequest(http.MethodPost, "/import", mustMarshal(suite.T(), testutils.ImportRequest{
			Content: document,
			Options: testutils.ImportOptions{Upsert: true, ContinueOnError: true, Target: "runtime"},
		}))
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		suite.Require().Equal(http.StatusOK, resp.StatusCode, "import: %s", string(body))
		var result testutils.ImportResponse
		suite.Require().NoError(json.Unmarshal(body, &result))
		suite.Require().Len(result.Results, 1, "import: %s", string(body))
		return result.Results[0]
	}

	refused := importRole()
	suite.Equal("failed", refused.Status)
	suite.Equal(errRoleResourceServerNotAvailable, refused.Code, "child A has not been shared the resource server")

	suite.shareResourceServer(f, f.childAOUID)

	imported := importRole()
	suite.Equal("success", imported.Status, "child A has been shared the resource server: %s", imported.Message)
}

// newRoleID returns a random role ID for a role a test imports.
func newRoleID(suite *RoleAPITestSuite) string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	suite.Require().NoError(err)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// A role shared into an organization unit the resource server is not shared with confers none of the resource
// server's permissions there, until the resource server is shared with the unit as well. An application's token counts
// the roles assigned to it in its own organization unit, so it shows what the assignment confers.
func (suite *RoleAPITestSuite) TestResourceServerAvailability_SharedRoleConfersOnlyWhereTheServerIs() {
	f := suite.newAvailabilityFixture("rs-avail-shared-role")
	role, err := suite.createRole(CreateRoleRequest{Name: "Parent Role", OUID: f.parentOUID,
		Permissions: f.permissions()})
	suite.Require().NoError(err)
	defer suite.deleteRole(role.ID)
	resp := suite.doRoleRequest(http.MethodPost, sharingPolicyPath(role.ID), mustMarshal(suite.T(),
		map[string]interface{}{"childUnits": map[string]interface{}{
			"mode": "selected", "units": []map[string]string{{"ouId": f.childAOUID, "reach": "unit"}},
		}}))
	suite.Require().Equal(http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	appID := suite.appIn(f.childAOUID, "rs-avail-shared-role-app")
	clientID, secret := "rs-avail-shared-role-app-client", "rs-avail-shared-role-app-secret"
	request := AssignmentsRequest{OUID: f.childAOUID, Assignments: []Assignment{{ID: appID, Type: AssigneeTypeApp}}}
	status, code := suite.postAssignments(role.ID, "add", request)
	suite.Require().Equal(http.StatusNoContent, status, code)
	defer func() { _, _ = suite.postAssignments(role.ID, "remove", request) }()

	suite.NotContains(suite.grantedScopes("", clientID, secret, f.identifier, availabilityPermission),
		availabilityPermission, "the role is shared with child A, but the resource server is not")

	suite.shareResourceServer(f, f.childAOUID)

	suite.Contains(suite.grantedScopes("", clientID, secret, f.identifier, availabilityPermission),
		availabilityPermission)
}

// A role read as an organization unit it is shared with shows only the permissions of resource servers that unit may
// use. The owner sees every permission.
func (suite *RoleAPITestSuite) TestResourceServerAvailability_SharedRoleShowsOnlyUsablePermissions() {
	f := suite.newAvailabilityFixture("rs-avail-shared-read")
	otherServerID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name: "rs-avail-shared-read other API", Identifier: "https://api.example.com/rs-avail-shared-read-other",
		OUID: f.parentOUID,
	}, []testutils.Action{{Name: "Write invoices", Handle: "invoices-write"}})
	suite.Require().NoError(err)
	defer func() { _ = testutils.DeleteResourceServerWithChildren(otherServerID) }()

	both := []ResourcePermissions{
		{ResourceServerID: f.resourceServerID, Permissions: []string{availabilityPermission}},
		{ResourceServerID: otherServerID, Permissions: []string{"invoices-write"}},
	}
	role, err := suite.createRole(CreateRoleRequest{Name: "Two Server Role", OUID: f.parentOUID, Permissions: both})
	suite.Require().NoError(err)
	defer suite.deleteRole(role.ID)
	resp := suite.doRoleRequest(http.MethodPost, sharingPolicyPath(role.ID), mustMarshal(suite.T(),
		map[string]interface{}{"childUnits": map[string]interface{}{
			"mode": "selected", "units": []map[string]string{{"ouId": f.childAOUID, "reach": "unit"}},
		}}))
	suite.Require().Equal(http.StatusCreated, resp.StatusCode)
	resp.Body.Close()
	suite.shareResourceServer(f, f.childAOUID)

	permissionsIn := func(ouID string) []ResourcePermissions {
		suite.T().Helper()
		path := rolesBasePath + "/" + role.ID
		if ouID != "" {
			path += "?ouId=" + ouID
		}
		resp := suite.doRoleRequest(http.MethodGet, path, nil)
		defer resp.Body.Close()
		suite.Require().Equal(http.StatusOK, resp.StatusCode)
		var read Role
		suite.Require().NoError(json.NewDecoder(resp.Body).Decode(&read))
		return read.Permissions
	}

	suite.Equal([]ResourcePermissions{both[0]}, permissionsIn(f.childAOUID),
		"child A is shared only the first resource server")
	suite.ElementsMatch(both, permissionsIn(f.parentOUID))
	suite.ElementsMatch(both, permissionsIn(""))

	otherPolicy := suite.doRoleRequest(http.MethodPost, "/resource-servers/"+otherServerID+"/sharing-policies",
		mustMarshal(suite.T(), map[string]interface{}{"childUnits": map[string]interface{}{
			"mode": "selected", "units": []map[string]string{{"ouId": f.childAOUID, "reach": "unit"}},
		}}))
	suite.Require().Equal(http.StatusCreated, otherPolicy.StatusCode)
	otherPolicy.Body.Close()

	suite.ElementsMatch(both, permissionsIn(f.childAOUID), "child A is now shared both resource servers")
}

// A token issued for an organization unit carries permissions only of resource servers that unit may use. A
// resource server it may not use answers as one that does not exist, and the bare endpoint is unchanged.
func (suite *RoleAPITestSuite) TestResourceServerAvailability_OUScopedToken() {
	f := suite.newAvailabilityFixture("rs-avail-token")
	appID := suite.appIn(f.parentOUID, "rs-avail-token-app")
	clientID, secret := "rs-avail-token-app-client", "rs-avail-token-app-secret"
	resp := suite.doRoleRequest(http.MethodPost, "/applications/"+appID+"/sharing-policies",
		mustMarshal(suite.T(), map[string]interface{}{"childUnits": map[string]interface{}{"mode": "all"}}))
	suite.Require().Equal(http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	role, err := suite.createRole(CreateRoleRequest{Name: "App Role", OUID: f.parentOUID,
		Permissions: f.permissions(), Assignments: []Assignment{{ID: appID, Type: AssigneeTypeApp}}})
	suite.Require().NoError(err)
	defer suite.deleteRole(role.ID)

	token := func(ouID string) (int, map[string]interface{}) {
		suite.T().Helper()
		form := url.Values{}
		form.Set("grant_type", "client_credentials")
		form.Set("resource", f.identifier)
		form.Set("scope", availabilityPermission)
		endpoint := testServerURL + "/oauth2/token"
		if ouID != "" {
			endpoint = testServerURL + "/ou/" + ouID + "/oauth2/token"
		}
		req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(form.Encode()))
		suite.Require().NoError(err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth(clientID, secret)
		// The request authenticates as the client, so no admin bearer may be added to it.
		resp, err := testutils.GetRawHTTPClient().Do(req)
		suite.Require().NoError(err)
		defer resp.Body.Close()
		body := map[string]interface{}{}
		_ = json.NewDecoder(resp.Body).Decode(&body)
		return resp.StatusCode, body
	}
	grants := func(ouID string) {
		suite.T().Helper()
		status, body := token(ouID)
		suite.Require().Equal(http.StatusOK, status, "token for %q: %v", ouID, body)
		suite.Contains(strings.Fields(body["scope"].(string)), availabilityPermission, "token for %q", ouID)
	}
	refuses := func(ouID string) {
		suite.T().Helper()
		status, body := token(ouID)
		suite.Equal(http.StatusBadRequest, status, "token for %q: %v", ouID, body)
		suite.Equal("invalid_target", body["error"], "token for %q", ouID)
	}

	grants("")
	grants(f.parentOUID)
	refuses(f.childAOUID)

	suite.shareResourceServer(f, f.childAOUID)

	grants(f.childAOUID)
	refuses(f.childBOUID)
}

// What an administrator holds counts only what its roles confer where they are assigned. An administrator of child A
// that holds orders-read only through a role assigned in child A, which the resource server is not shared with, does
// not count it, so it may not assign a role that needs it. Once the resource server is shared with child A, it does.
func (suite *RoleAPITestSuite) TestResourceServerAvailability_OUAdminCountsOnlyWhatItMayUse() {
	f := suite.newAvailabilityFixture("rs-avail-admin-assign")
	unshareSystem, err := testutils.ShareSystemResourceServerWith(f.parentOUID)
	suite.Require().NoError(err, "share the System resource server")
	suite.T().Cleanup(func() { suite.NoError(unshareSystem()) })

	const clientID, clientSecret = "rs-avail-admin-assign-admin", "rs-avail-admin-assign-admin-secret"
	adminID, err := testutils.CreateApplication(testutils.Application{
		Name: "rs-avail-admin-assign-admin", OUID: f.childAOUID,
		InboundAuthConfig: []map[string]interface{}{{"type": "oauth2", "config": map[string]interface{}{
			"clientId": clientID, "clientSecret": clientSecret,
			"grantTypes":              []string{"client_credentials"},
			"tokenEndpointAuthMethod": "client_secret_basic",
			// An org permission acts in the unit the token names.
			"token": map[string]interface{}{"accessToken": map[string]interface{}{
				"clientConfig": map[string]interface{}{"attributes": []string{"ouId"}},
			}},
		}}},
	})
	suite.Require().NoError(err)
	suite.T().Cleanup(func() { _ = testutils.DeleteApplication(adminID) })
	adminRole, err := suite.createRole(CreateRoleRequest{Name: "Child Admin", OUID: f.childAOUID,
		Permissions: []ResourcePermissions{{ResourceServerID: systemResourceServerID, Permissions: []string{"org"}}},
		Assignments: []Assignment{{ID: adminID, Type: AssigneeTypeApp}}})
	suite.Require().NoError(err)
	suite.T().Cleanup(func() { suite.deleteRole(adminRole.ID) })
	token, err := testutils.RequestClientCredentialsToken(clientID, clientSecret, "org",
		testutils.SystemResourceIdentifier)
	suite.Require().NoError(err)
	suite.Require().Equal(http.StatusOK, token.StatusCode, "token: %s", string(token.Body))
	admin := testutils.GetHTTPClientWithToken(token.Token.AccessToken)

	shared, err := suite.createRole(CreateRoleRequest{Name: "Orders Reader", OUID: f.parentOUID,
		Permissions: f.permissions()})
	suite.Require().NoError(err)
	suite.T().Cleanup(func() { suite.deleteRole(shared.ID) })
	resp := suite.doRoleRequest(http.MethodPost, sharingPolicyPath(shared.ID), mustMarshal(suite.T(),
		map[string]interface{}{"childUnits": map[string]interface{}{
			"mode": "selected", "units": []map[string]string{{"ouId": f.childAOUID, "reach": "unit"}},
		}}))
	suite.Require().Equal(http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// The administrator holds orders-read through the shared role, assigned to it in child A.
	adminAssignment := AssignmentsRequest{OUID: f.childAOUID,
		Assignments: []Assignment{{ID: adminID, Type: AssigneeTypeApp}}}
	status, code := suite.postAssignments(shared.ID, "add", adminAssignment)
	suite.Require().Equal(http.StatusNoContent, status, code)
	suite.T().Cleanup(func() { _, _ = suite.postAssignments(shared.ID, "remove", adminAssignment) })

	assign := func(appID string) (int, string) {
		suite.T().Helper()
		body := mustMarshal(suite.T(), AssignmentsRequest{OUID: f.childAOUID,
			Assignments: []Assignment{{ID: appID, Type: AssigneeTypeApp}}})
		req, err := http.NewRequest(http.MethodPost, testServerURL+rolesBasePath+"/"+shared.ID+"/assignments/add",
			bytes.NewReader(body))
		suite.Require().NoError(err)
		req.Header.Set("Content-Type", "application/json")
		resp, err := admin.Do(req)
		suite.Require().NoError(err)
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusNoContent {
			return resp.StatusCode, ""
		}
		return resp.StatusCode, decodeErrorCode(suite.T(), resp)
	}

	status, code = assign(suite.appIn(f.childAOUID, "rs-avail-admin-assign-first"))
	suite.Equal(http.StatusForbidden, status)
	suite.Equal("SAZ-4030", code, "the administrator's orders-read comes from an assignment in child A, which "+
		"cannot use the resource server, so it does not count")

	suite.shareResourceServer(f, f.childAOUID)

	status, code = assign(suite.appIn(f.childAOUID, "rs-avail-admin-assign-second"))
	suite.Equal(http.StatusNoContent, status, "child A can now use the resource server, so the administrator's "+
		"orders-read counts: %s", code)
}

// A resource server cannot move away from an organization unit whose roles still hold its permissions, since the unit
// would no longer own it.
func (suite *RoleAPITestSuite) TestResourceServerAvailability_MoveRefusedWhileUsed() {
	f := suite.newAvailabilityFixture("rs-avail-move")
	role, err := suite.createRole(CreateRoleRequest{Name: "Parent Role", OUID: f.parentOUID,
		Permissions: f.permissions()})
	suite.Require().NoError(err)
	defer suite.deleteRole(role.ID)

	move := func() *http.Response {
		suite.T().Helper()
		return suite.doRoleRequest(http.MethodPut, "/resource-servers/"+f.resourceServerID,
			mustMarshal(suite.T(), map[string]string{
				"name": "rs-avail-move API", "identifier": f.identifier, "ouId": f.childAOUID,
			}))
	}

	// A shared resource server cannot move at all, so that refusal comes before the one about the roles.
	policyID := suite.shareResourceServer(f, f.childBOUID)
	resp := move()
	suite.Equal(http.StatusBadRequest, resp.StatusCode)
	suite.Equal(errSharedResourceServerCannotMove, decodeErrorCode(suite.T(), resp))
	resp.Body.Close()
	resp = suite.doRoleRequest(http.MethodDelete,
		"/resource-servers/"+f.resourceServerID+"/sharing-policies/"+policyID, nil)
	suite.Require().Equal(http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	resp = move()
	suite.Equal(http.StatusBadRequest, resp.StatusCode)
	suite.Equal(errResourceServerUsedInOU, decodeErrorCode(suite.T(), resp))
	resp.Body.Close()

	_, err = suite.updateRole(role.ID, UpdateRoleRequest{Name: "Parent Role", OUID: f.parentOUID,
		Permissions: []ResourcePermissions{}})
	suite.Require().NoError(err)

	resp = move()
	defer resp.Body.Close()
	suite.Equal(http.StatusOK, resp.StatusCode, "nothing in the parent uses the resource server any more")
}
