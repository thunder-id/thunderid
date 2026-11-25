// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ou

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	childMgmtRootHandle    = "child-mgmt-root"
	childMgmtUnitHandle    = "child-mgmt-unit"
	childMgmtChildHandle   = "child-mgmt-child"
	childMgmtGrandHandle   = "child-mgmt-grandchild"
	childMgmtSiblingHandle = "child-mgmt-sibling"
	childMgmtPassword      = "ChildMgmt@123"
	childMgmtRSIdentifier  = "https://child-mgmt.example.com/system"
)

// OUChildManagementTestSuite covers how far org and org:ou reach into organization units on the Data Plane.
// A token names one unit, and manages the units directly beneath it: it reads its own unit and its direct
// children, lists its own children, creates, updates and deletes its direct children, and reaches nothing
// further down, beside or above. Only the system permission lists the root organization units.
//
//	root (child-mgmt-root)
//	├── unit (child-mgmt-unit)              ← every administrator belongs here
//	│   └── child (child-mgmt-child)
//	│       └── grandchild (child-mgmt-grandchild)
//	└── sibling (child-mgmt-sibling)
type OUChildManagementTestSuite struct {
	suite.Suite

	signIn testutils.OUSignInApps

	rootID, unitID, childID, grandchildID, siblingID string
	userTypeID, rsID                                 string
	userIDs, roleIDs                                 []string

	// admins holds a client per permission an administrator of the unit holds.
	admins map[string]*http.Client
}

func TestOUChildManagementTestSuite(t *testing.T) {
	suite.Run(t, new(OUChildManagementTestSuite))
}

func (ts *OUChildManagementTestSuite) SetupSuite() {
	ts.rootID = ts.createOU(childMgmtRootHandle, nil)
	ts.unitID = ts.createOU(childMgmtUnitHandle, &ts.rootID)
	ts.childID = ts.createOU(childMgmtChildHandle, &ts.unitID)
	ts.grandchildID = ts.createOU(childMgmtGrandHandle, &ts.childID)
	ts.siblingID = ts.createOU(childMgmtSiblingHandle, &ts.rootID)

	var err error
	ts.userTypeID, err = testutils.CreateUserType(testutils.UserType{
		Handle:      "child-mgmt-admin-type",
		DisplayName: "Child Management Admin Type",
		OUID:        ts.unitID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string", "unique": true},
			"password": map[string]interface{}{"type": "string", "credential": true},
		},
	})
	ts.Require().NoError(err, "create the administrators' user type")
	ts.rsID, err = testutils.CreateOrgScopedResourceServer(ts.unitID, "Child Management RS", childMgmtRSIdentifier,
		"ou")
	ts.Require().NoError(err, "create the resource server")

	ts.admins = map[string]*http.Client{
		"org":    ts.adminClient("child-mgmt-org-admin", "org"),
		"org:ou": ts.adminClient("child-mgmt-ou-admin", "org:ou"),
	}
}

func (ts *OUChildManagementTestSuite) TearDownSuite() {
	if err := ts.signIn.DeleteAll(); err != nil {
		ts.T().Logf("teardown: delete sign-in applications: %v", err)
	}
	for _, id := range ts.roleIDs {
		if err := testutils.DeleteRole(id); err != nil {
			ts.T().Logf("teardown: delete role %s: %v", id, err)
		}
	}
	for _, id := range ts.userIDs {
		if err := testutils.DeleteUser(id); err != nil {
			ts.T().Logf("teardown: delete user %s: %v", id, err)
		}
	}
	if ts.rsID != "" {
		if err := testutils.DeleteResourceServerWithChildren(ts.rsID); err != nil {
			ts.T().Logf("teardown: delete resource server: %v", err)
		}
	}
	if ts.userTypeID != "" {
		if err := testutils.DeleteUserType(ts.userTypeID); err != nil {
			ts.T().Logf("teardown: delete user type: %v", err)
		}
	}
	for _, id := range []string{ts.grandchildID, ts.childID, ts.siblingID, ts.unitID, ts.rootID} {
		if id == "" {
			continue
		}
		if err := testutils.DeleteOrganizationUnit(id); err != nil {
			ts.T().Logf("teardown: delete OU %s: %v", id, err)
		}
	}
}

// Only the system permission lists the root organization units.
func (ts *OUChildManagementTestSuite) TestListingTheRootUnitsNeedsSystem() {
	for scope, client := range ts.admins {
		ts.Run(scope, func() {
			status, body := ts.send(client, http.MethodGet, "/organization-units", nil)
			ts.Equal(http.StatusForbidden, status, string(body))
		})
	}
	status, body := ts.send(testutils.GetHTTPClient(), http.MethodGet, "/organization-units", nil)
	ts.Equal(http.StatusOK, status, string(body))
}

// The children of the token's own unit are listed, by ID and by handle path. The children of any other unit,
// below or above, are not.
func (ts *OUChildManagementTestSuite) TestOnlyTheUnitsOwnChildrenAreListed() {
	unitPath := "/organization-units/tree/" + childMgmtRootHandle + "/" + childMgmtUnitHandle
	for scope, client := range ts.admins {
		ts.Run(scope, func() {
			for _, path := range []string{"/organization-units/" + ts.unitID + "/ous", unitPath + "/ous"} {
				listed := ts.childIDs(client, path)
				ts.Contains(listed, ts.childID, path)
				ts.NotContains(listed, ts.grandchildID, path)
			}
			for _, path := range []string{
				"/organization-units/" + ts.childID + "/ous",
				"/organization-units/" + ts.rootID + "/ous",
				unitPath + "/" + childMgmtChildHandle + "/ous",
				"/organization-units/tree/" + childMgmtRootHandle + "/ous",
			} {
				status, body := ts.send(client, http.MethodGet, path, nil)
				ts.Equalf(http.StatusForbidden, status, "%s: %s", path, body)
			}
		})
	}
}

// The token's own unit and its direct children are read, by ID and by handle path. The grandchild, the sibling
// and the parent are not.
func (ts *OUChildManagementTestSuite) TestTheUnitAndItsChildrenAreRead() {
	unitPath := "/organization-units/tree/" + childMgmtRootHandle + "/" + childMgmtUnitHandle
	for scope, client := range ts.admins {
		ts.Run(scope, func() {
			for _, path := range []string{
				"/organization-units/" + ts.unitID, "/organization-units/" + ts.childID,
				unitPath, unitPath + "/" + childMgmtChildHandle,
			} {
				status, body := ts.send(client, http.MethodGet, path, nil)
				ts.Equalf(http.StatusOK, status, "%s: %s", path, body)
			}
			for _, path := range []string{
				"/organization-units/" + ts.grandchildID, "/organization-units/" + ts.siblingID,
				"/organization-units/" + ts.rootID,
				unitPath + "/" + childMgmtChildHandle + "/" + childMgmtGrandHandle,
				"/organization-units/tree/" + childMgmtRootHandle + "/" + childMgmtSiblingHandle,
				"/organization-units/tree/" + childMgmtRootHandle,
			} {
				status, body := ts.send(client, http.MethodGet, path, nil)
				ts.Equalf(http.StatusForbidden, status, "%s: %s", path, body)
			}
		})
	}
}

// A handle path that names no unit is refused as a path out of reach is, so the token learns nothing about the
// units outside its reach. The system permission is told the unit is not found.
func (ts *OUChildManagementTestSuite) TestAPathNamingNoUnitIsRefused() {
	for _, path := range []string{
		"/organization-units/tree/" + childMgmtRootHandle + "/no-such-unit",
		"/organization-units/tree/" + childMgmtRootHandle + "/" + childMgmtUnitHandle + "/no-such-child/ous",
	} {
		for scope, client := range ts.admins {
			status, body := ts.send(client, http.MethodGet, path, nil)
			ts.Equalf(http.StatusForbidden, status, "%s with %s: %s", path, scope, body)
		}
		status, body := ts.send(testutils.GetHTTPClient(), http.MethodGet, path, nil)
		ts.Equalf(http.StatusNotFound, status, "%s with system: %s", path, body)
	}
}

// The token's direct children are updated, by ID and by handle path, but not moved out from under the token's
// unit. The token's own unit, the grandchild and the sibling are not updated.
func (ts *OUChildManagementTestSuite) TestOnlyDirectChildrenAreUpdated() {
	unitPath := "/organization-units/tree/" + childMgmtRootHandle + "/" + childMgmtUnitHandle
	for scope, client := range ts.admins {
		ts.Run(scope, func() {
			status, body := ts.send(client, http.MethodPut, "/organization-units/"+ts.childID,
				ts.update(childMgmtChildHandle, "Child renamed by "+scope, &ts.unitID))
			ts.Equal(http.StatusOK, status, string(body))
			status, body = ts.send(client, http.MethodPut, unitPath+"/"+childMgmtChildHandle,
				ts.update(childMgmtChildHandle, "Child renamed by path by "+scope, &ts.unitID))
			ts.Equal(http.StatusOK, status, string(body))

			for name, parent := range map[string]*string{
				"the sibling": &ts.siblingID, "the parent": &ts.rootID, "the root": nil,
			} {
				status, body := ts.send(client, http.MethodPut, "/organization-units/"+ts.childID,
					ts.update(childMgmtChildHandle, "Child moved", parent))
				ts.Equalf(http.StatusForbidden, status, "move the child under %s: %s", name, body)
			}
			status, body = ts.send(client, http.MethodPut, unitPath+"/"+childMgmtChildHandle,
				ts.update(childMgmtChildHandle, "Child moved by path", &ts.siblingID))
			ts.Equal(http.StatusForbidden, status, "move the child by path: %s", body)

			for id, req := range map[string]UpdateOURequest{
				ts.unitID:       ts.update(childMgmtUnitHandle, "Unit renamed", &ts.rootID),
				ts.grandchildID: ts.update(childMgmtGrandHandle, "Grandchild renamed", &ts.childID),
				ts.siblingID:    ts.update(childMgmtSiblingHandle, "Sibling renamed", &ts.rootID),
			} {
				status, body := ts.send(client, http.MethodPut, "/organization-units/"+id, req)
				ts.Equalf(http.StatusForbidden, status, "update %s: %s", req.Handle, body)
			}
		})
	}
}

// A child is created beneath the token's own unit and deleted, by ID and by handle path. Nothing is created
// beneath a child, and the token's own unit, the grandchild and the sibling are not deleted.
func (ts *OUChildManagementTestSuite) TestOnlyDirectChildrenAreCreatedAndDeleted() {
	unitPath := "/organization-units/tree/" + childMgmtRootHandle + "/" + childMgmtUnitHandle
	for scope, client := range ts.admins {
		ts.Run(scope, func() {
			byID := ts.createAs(client, "child-mgmt-temp-id-"+scope[len(scope)-2:], &ts.unitID)
			status, body := ts.send(client, http.MethodDelete, "/organization-units/"+byID, nil)
			ts.Equal(http.StatusNoContent, status, string(body))

			handle := "child-mgmt-temp-path-" + scope[len(scope)-2:]
			ts.createAs(client, handle, &ts.unitID)
			status, body = ts.send(client, http.MethodDelete, unitPath+"/"+handle, nil)
			ts.Equal(http.StatusNoContent, status, string(body))

			status, body = ts.send(client, http.MethodPost, "/organization-units", CreateOURequest{
				Handle: "child-mgmt-refused", Name: "Refused", Parent: &ts.childID})
			ts.Equal(http.StatusForbidden, status, "nothing is created beneath a child: %s", body)

			for _, id := range []string{ts.unitID, ts.grandchildID, ts.siblingID} {
				status, body := ts.send(client, http.MethodDelete, "/organization-units/"+id, nil)
				ts.Equalf(http.StatusForbidden, status, "delete %s: %s", id, body)
			}
		})
	}
}

func (ts *OUChildManagementTestSuite) update(handle, name string, parent *string) UpdateOURequest {
	return UpdateOURequest{Handle: handle, Name: name, Parent: parent}
}

// createAs creates an organization unit beneath parent with the client, and returns its ID.
func (ts *OUChildManagementTestSuite) createAs(client *http.Client, handle string, parent *string) string {
	ts.T().Helper()
	status, body := ts.send(client, http.MethodPost, "/organization-units",
		CreateOURequest{Handle: handle, Name: handle, Parent: parent})
	ts.Require().Equal(http.StatusCreated, status, string(body))
	var created struct {
		ID string `json:"id"`
	}
	ts.Require().NoError(json.Unmarshal(body, &created))
	return created.ID
}

// childIDs reads a list of organization units with the client and returns the IDs listed.
func (ts *OUChildManagementTestSuite) childIDs(client *http.Client, path string) []string {
	ts.T().Helper()
	status, body := ts.send(client, http.MethodGet, path, nil)
	ts.Require().Equalf(http.StatusOK, status, "%s: %s", path, body)
	var list OrganizationUnitListResponse
	ts.Require().NoError(json.Unmarshal(body, &list))
	ids := make([]string, 0, len(list.OrganizationUnits))
	for _, ou := range list.OrganizationUnits {
		ids = append(ids, ou.ID)
	}
	return ids
}

func (ts *OUChildManagementTestSuite) createOU(handle string, parent *string) string {
	ts.T().Helper()
	id, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{Handle: handle, Name: handle,
		Parent: parent})
	ts.Require().NoError(err, "create OU %s", handle)
	return id
}

// adminClient creates an administrator of the unit holding permission there, and returns a client carrying its
// token.
func (ts *OUChildManagementTestSuite) adminClient(username, permission string) *http.Client {
	ts.T().Helper()
	userID, err := testutils.CreateUser(testutils.User{
		Type: "child-mgmt-admin-type",
		OUID: ts.unitID,
		Attributes: json.RawMessage(fmt.Sprintf(`{"username": %q, "password": %q}`,
			username, childMgmtPassword)),
	})
	ts.Require().NoError(err, "create administrator %s", username)
	ts.userIDs = append(ts.userIDs, userID)
	roleID, err := testutils.CreateRole(testutils.Role{
		Name:        username + "-role",
		OUID:        ts.unitID,
		Permissions: []testutils.ResourcePermissions{{ResourceServerID: ts.rsID, Permissions: []string{permission}}},
		Assignments: []testutils.Assignment{{ID: userID, Type: "user"}},
	})
	ts.Require().NoError(err, "create the role of %s", username)
	ts.roleIDs = append(ts.roleIDs, roleID)

	token, err := ts.signIn.ObtainAccessToken(ts.unitID, permission, username, childMgmtPassword,
		childMgmtRSIdentifier)
	ts.Require().NoError(err, "obtain the token of %s", username)
	ts.Require().Equal(permission, token.Scope)
	return testutils.GetHTTPClientWithToken(token.AccessToken)
}

// send issues a JSON request with the client and returns the status and body.
func (ts *OUChildManagementTestSuite) send(client *http.Client, method, path string, payload interface{}) (
	int, []byte) {
	ts.T().Helper()
	var reader io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		ts.Require().NoError(err)
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, testutils.TestServerURL+path, reader)
	ts.Require().NoError(err)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	return resp.StatusCode, body
}
