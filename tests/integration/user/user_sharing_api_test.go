// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// UserSharingAPITestSuite exercises the user sharing policy endpoints against a running server.
//
// Fixture topology:
//
//	parent (handle: user-sharing-parent)   ← owns the shared user and the parent admin
//	└── child (handle: user-sharing-child)  ← holds the child admin
//	    └── grandchild (handle: user-sharing-grandchild)
//
// The parent and child admins each hold system:user in their own organization unit only, which is
// what makes "a policy belongs to the unit that issued it" observable.
type UserSharingAPITestSuite struct {
	suite.Suite

	parentOUID     string
	childOUID      string
	grandchildOUID string
	userTypeID     string

	sharedUserID string
	adminUserIDs []string
	roleIDs      []string
	rsIDs        []string

	rootClient   *http.Client
	parentClient *http.Client
	childClient  *http.Client
}

const (
	sharingTestUserType   = "user-sharing-test-type"
	sharingTestPassword   = "Sharing@123"
	sharingTestScopes     = "system system:user system:usertype:view"
	sharingTestConsoleURI = "https://localhost:8095/console"
)

// sharingPolicy is the part of a policy response these tests read.
type sharingPolicy struct {
	ID             string `json:"id"`
	InitiatingOUID string `json:"initiatingOuId"`
	Stage          string `json:"stage"`
	ParentPolicyID string `json:"parentPolicyId"`
	Version        int    `json:"version"`
	IsReadOnly     bool   `json:"isReadOnly"`
}

type sharingPolicyList struct {
	TotalResults int             `json:"totalResults"`
	Count        int             `json:"count"`
	Policies     []sharingPolicy `json:"policies"`
}

type sharingError struct {
	Code string `json:"code"`
}

func TestUserSharingAPITestSuite(t *testing.T) {
	suite.Run(t, new(UserSharingAPITestSuite))
}

func (ts *UserSharingAPITestSuite) SetupSuite() {
	ts.rootClient = testutils.GetHTTPClient()

	var err error
	ts.parentOUID, err = testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "user-sharing-parent", Name: "User Sharing Parent",
	})
	ts.Require().NoError(err, "create parent OU")
	ts.childOUID, err = testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "user-sharing-child", Name: "User Sharing Child", Parent: &ts.parentOUID,
	})
	ts.Require().NoError(err, "create child OU")
	ts.grandchildOUID, err = testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "user-sharing-grandchild", Name: "User Sharing Grandchild", Parent: &ts.childOUID,
	})
	ts.Require().NoError(err, "create grandchild OU")

	ts.userTypeID, err = testutils.CreateUserType(testutils.UserType{
		Handle:      sharingTestUserType,
		DisplayName: "User Sharing Test Type",
		OUID:        ts.parentOUID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
			"password": map[string]interface{}{"type": "string", "credential": true},
		},
	})
	ts.Require().NoError(err, "create user type")

	ts.sharedUserID = ts.createUser(ts.parentOUID, "user-sharing-target")
	ts.parentClient = ts.adminClient(ts.parentOUID, "user-sharing-parent-admin", "parent")
	ts.childClient = ts.adminClient(ts.childOUID, "user-sharing-child-admin", "child")
}

func (ts *UserSharingAPITestSuite) TearDownSuite() {
	for _, id := range ts.roleIDs {
		if err := testutils.DeleteRole(id); err != nil {
			ts.T().Logf("teardown: delete role %s: %v", id, err)
		}
	}
	for _, id := range ts.rsIDs {
		if err := testutils.DeleteResourceServerWithChildren(id); err != nil {
			ts.T().Errorf("teardown: delete resource server %s: %v", id, err)
		}
	}
	for _, id := range append([]string{ts.sharedUserID}, ts.adminUserIDs...) {
		if id == "" {
			continue
		}
		if err := testutils.DeleteUser(id); err != nil {
			ts.T().Logf("teardown: delete user %s: %v", id, err)
		}
	}
	if ts.userTypeID != "" {
		if err := testutils.DeleteUserType(ts.userTypeID); err != nil {
			ts.T().Logf("teardown: delete user type: %v", err)
		}
	}
	for _, id := range []string{ts.grandchildOUID, ts.childOUID, ts.parentOUID} {
		if id == "" {
			continue
		}
		if err := testutils.DeleteOrganizationUnit(id); err != nil {
			ts.T().Logf("teardown: delete OU %s: %v", id, err)
		}
	}
}

// createUser creates a user of the suite's type in the given organization unit.
func (ts *UserSharingAPITestSuite) createUser(ouID, username string) string {
	ts.T().Helper()
	id, err := testutils.CreateUser(testutils.User{
		Type: sharingTestUserType,
		OUID: ouID,
		Attributes: json.RawMessage(fmt.Sprintf(`{"username": %q, "password": %q}`,
			username, sharingTestPassword)),
	})
	ts.Require().NoError(err, "create user %s", username)
	return id
}

// adminClient creates a user in ouID holding system:user there and nowhere else, and returns a
// client carrying that user's token.
func (ts *UserSharingAPITestSuite) adminClient(ouID, username, name string) *http.Client {
	ts.T().Helper()
	userID := ts.createUser(ouID, username)
	ts.adminUserIDs = append(ts.adminUserIDs, userID)

	identifier := "https://user-sharing-test.example.com/" + name
	rsID, err := testutils.CreateSystemScopedResourceServer(
		ouID, "User Sharing Test RS ("+name+")", identifier, "user", "usertype")
	ts.Require().NoError(err, "create resource server for %s", name)
	ts.rsIDs = append(ts.rsIDs, rsID)

	roleID, err := testutils.CreateRole(testutils.Role{
		Name: "User Sharing Admin (" + name + ")",
		OUID: ouID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: rsID, Permissions: []string{"system:user", "system:usertype:view"}},
		},
		Assignments: []testutils.Assignment{{ID: userID, Type: "user"}},
	})
	ts.Require().NoError(err, "create role for %s", name)
	ts.roleIDs = append(ts.roleIDs, roleID)

	token, err := testutils.ObtainAccessTokenWithPassword(
		"CONSOLE", sharingTestConsoleURI, sharingTestScopes, username, sharingTestPassword, true, "", identifier)
	ts.Require().NoError(err, "obtain token for %s", name)
	return testutils.GetHTTPClientWithToken(token.AccessToken)
}

// do sends a request and returns its status and body.
func (ts *UserSharingAPITestSuite) do(client *http.Client, method, path string, body interface{}) (int, []byte) {
	ts.T().Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		ts.Require().NoError(err)
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, testServerURL+path, reader)
	ts.Require().NoError(err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	return resp.StatusCode, raw
}

// expectError asserts a status and an error code.
func (ts *UserSharingAPITestSuite) expectError(status int, raw []byte, wantStatus int, wantCode string) {
	ts.T().Helper()
	ts.Require().Equal(wantStatus, status, string(raw))
	var e sharingError
	ts.Require().NoError(json.Unmarshal(raw, &e))
	ts.Equal(wantCode, e.Code)
}

// share records a policy as the given client and returns it.
func (ts *UserSharingAPITestSuite) share(client *http.Client, userID string, body interface{}) sharingPolicy {
	ts.T().Helper()
	status, raw := ts.do(client, http.MethodPost, "/users/"+userID+"/sharing-policies", body)
	ts.Require().Equal(http.StatusCreated, status, string(raw))
	var p sharingPolicy
	ts.Require().NoError(json.Unmarshal(raw, &p))
	return p
}

func policyPath(userID, policyID string) string {
	return "/users/" + userID + "/sharing-policies/" + policyID
}

func overlayPath(userID, ouID string) string {
	return "/users/" + userID + "/overlay-rules?ouId=" + ouID
}

func namedChild(ouID, reach string) map[string]interface{} {
	return map[string]interface{}{"childUnits": map[string]interface{}{
		"mode": "selected", "units": []map[string]string{{"ouId": ouID, "reach": reach}},
	}}
}

func (ts *UserSharingAPITestSuite) TestSharingPolicyLifecycle() {
	status, raw := ts.do(ts.rootClient, http.MethodGet, overlayPath(ts.sharedUserID, ts.childOUID), nil)
	ts.expectError(status, raw, http.StatusNotFound, "USR-1034")

	created := ts.share(ts.rootClient, ts.sharedUserID, namedChild(ts.childOUID, "unit"))
	ts.Equal(1, created.Version)
	ts.Equal("share", created.Stage)
	ts.Equal(ts.parentOUID, created.InitiatingOUID)
	defer ts.do(ts.rootClient, http.MethodDelete, policyPath(ts.sharedUserID, created.ID), nil)

	status, raw = ts.do(ts.rootClient, http.MethodGet, overlayPath(ts.sharedUserID, ts.childOUID), nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	status, raw = ts.do(ts.rootClient, http.MethodGet, overlayPath(ts.sharedUserID, ts.grandchildOUID), nil)
	ts.expectError(status, raw, http.StatusNotFound, "USR-1034")

	status, raw = ts.do(ts.rootClient, http.MethodGet, "/users/"+ts.sharedUserID+"/sharing-policies", nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	var list sharingPolicyList
	ts.Require().NoError(json.Unmarshal(raw, &list))
	ts.Equal(1, list.TotalResults)

	// Widening the named child to its subtree reaches the grandchild too.
	update := namedChild(ts.childOUID, "subtree")
	update["version"] = 1
	status, raw = ts.do(ts.rootClient, http.MethodPut, policyPath(ts.sharedUserID, created.ID), update)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	var updated sharingPolicy
	ts.Require().NoError(json.Unmarshal(raw, &updated))
	ts.Equal(2, updated.Version)
	status, raw = ts.do(ts.rootClient, http.MethodGet, overlayPath(ts.sharedUserID, ts.grandchildOUID), nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))

	status, _ = ts.do(ts.rootClient, http.MethodDelete, policyPath(ts.sharedUserID, created.ID), nil)
	ts.Equal(http.StatusNoContent, status)
	status, raw = ts.do(ts.rootClient, http.MethodGet, policyPath(ts.sharedUserID, created.ID), nil)
	ts.expectError(status, raw, http.StatusNotFound, "USR-1030")
	status, raw = ts.do(ts.rootClient, http.MethodGet, overlayPath(ts.sharedUserID, ts.childOUID), nil)
	ts.expectError(status, raw, http.StatusNotFound, "USR-1034")
	status, _ = ts.do(ts.rootClient, http.MethodDelete, policyPath(ts.sharedUserID, created.ID), nil)
	ts.Equal(http.StatusNoContent, status, "deleting an absent policy succeeds")
}

func (ts *UserSharingAPITestSuite) TestSharingPolicyRefusals() {
	path := "/users/" + ts.sharedUserID + "/sharing-policies"

	status, raw := ts.do(ts.rootClient, http.MethodPost, path,
		map[string]interface{}{"childUnits": map[string]interface{}{"mode": "all", "excludeOuIds": []string{}}})
	ts.expectError(status, raw, http.StatusBadRequest, "USR-1001")

	status, raw = ts.do(ts.rootClient, http.MethodPost, path, map[string]interface{}{})
	ts.expectError(status, raw, http.StatusBadRequest, "USR-1029")

	// Reaching every unit, or every other tree, is left to a resource file or an import.
	status, raw = ts.do(ts.rootClient, http.MethodPost, path, map[string]interface{}{"allUnits": map[string]interface{}{}})
	ts.expectError(status, raw, http.StatusBadRequest, "USR-1037")
	status, raw = ts.do(ts.rootClient, http.MethodPost, path,
		map[string]interface{}{"otherTreeRoots": map[string]interface{}{"mode": "all"}})
	ts.expectError(status, raw, http.StatusBadRequest, "USR-1037")

	created := ts.share(ts.rootClient, ts.sharedUserID, namedChild(ts.childOUID, "unit"))
	defer ts.do(ts.rootClient, http.MethodDelete, policyPath(ts.sharedUserID, created.ID), nil)

	status, raw = ts.do(ts.rootClient, http.MethodPost, path, namedChild(ts.childOUID, "subtree"))
	ts.expectError(status, raw, http.StatusConflict, "USR-1031")

	stale := namedChild(ts.childOUID, "subtree")
	stale["version"] = 7
	status, raw = ts.do(ts.rootClient, http.MethodPut, policyPath(ts.sharedUserID, created.ID), stale)
	ts.expectError(status, raw, http.StatusConflict, "USR-1032")

	// An edit cannot move the decision to another unit.
	reissued := namedChild(ts.childOUID, "subtree")
	reissued["version"] = created.Version
	reissued["initiatingOuId"] = ts.childOUID
	status, raw = ts.do(ts.rootClient, http.MethodPut, policyPath(ts.sharedUserID, created.ID), reissued)
	ts.expectError(status, raw, http.StatusBadRequest, "USR-1001")

	// The policy is reached only through the user it governs.
	status, raw = ts.do(ts.rootClient, http.MethodGet, policyPath(ts.adminUserIDs[0], created.ID), nil)
	ts.expectError(status, raw, http.StatusNotFound, "USR-1030")
}

func (ts *UserSharingAPITestSuite) TestResharingBelongsToTheIssuingUnit() {
	parentPolicy := ts.share(ts.parentClient, ts.sharedUserID, namedChild(ts.childOUID, "unit"))
	defer ts.do(ts.rootClient, http.MethodDelete, policyPath(ts.sharedUserID, parentPolicy.ID), nil)
	path := "/users/" + ts.sharedUserID + "/sharing-policies"

	// The child holds the user but cannot speak for the owner.
	status, raw := ts.do(ts.childClient, http.MethodPost, path, namedChild(ts.grandchildOUID, "unit"))
	ts.expectError(status, raw, http.StatusForbidden, "SSE-4030")

	reshareBody := namedChild(ts.grandchildOUID, "unit")
	reshareBody["initiatingOuId"] = ts.childOUID
	reshare := ts.share(ts.childClient, ts.sharedUserID, reshareBody)
	ts.Equal("reshare", reshare.Stage)
	ts.Equal(parentPolicy.ID, reshare.ParentPolicyID)

	status, raw = ts.do(ts.rootClient, http.MethodGet, overlayPath(ts.sharedUserID, ts.grandchildOUID), nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))

	// Neither unit can change the other's decision.
	edit := namedChild(ts.childOUID, "subtree")
	edit["version"] = parentPolicy.Version
	status, raw = ts.do(ts.childClient, http.MethodPut, policyPath(ts.sharedUserID, parentPolicy.ID), edit)
	ts.expectError(status, raw, http.StatusForbidden, "SSE-4030")
	status, raw = ts.do(ts.childClient, http.MethodDelete, policyPath(ts.sharedUserID, parentPolicy.ID), nil)
	ts.expectError(status, raw, http.StatusForbidden, "SSE-4030")
	status, raw = ts.do(ts.parentClient, http.MethodDelete, policyPath(ts.sharedUserID, reshare.ID), nil)
	ts.expectError(status, raw, http.StatusForbidden, "SSE-4030")

	// The owner lists every decision about its user; the child lists its own.
	status, raw = ts.do(ts.parentClient, http.MethodGet, path, nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	var parentList sharingPolicyList
	ts.Require().NoError(json.Unmarshal(raw, &parentList))
	ts.Equal(2, parentList.TotalResults)

	status, raw = ts.do(ts.childClient, http.MethodGet, path, nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	var childList sharingPolicyList
	ts.Require().NoError(json.Unmarshal(raw, &childList))
	ts.Require().Equal(1, childList.TotalResults)
	ts.Equal(reshare.ID, childList.Policies[0].ID)

	// Taking the share away takes the reshare beneath it.
	status, _ = ts.do(ts.parentClient, http.MethodDelete, policyPath(ts.sharedUserID, parentPolicy.ID), nil)
	ts.Require().Equal(http.StatusNoContent, status)
	status, raw = ts.do(ts.rootClient, http.MethodGet, policyPath(ts.sharedUserID, reshare.ID), nil)
	ts.expectError(status, raw, http.StatusNotFound, "USR-1030")
	status, raw = ts.do(ts.rootClient, http.MethodGet, overlayPath(ts.sharedUserID, ts.grandchildOUID), nil)
	ts.expectError(status, raw, http.StatusNotFound, "USR-1034")
}

func (ts *UserSharingAPITestSuite) TestDeletingASharedUserSucceeds() {
	userID := ts.createUser(ts.parentOUID, "user-sharing-deletable")
	share := ts.share(ts.rootClient, userID, namedChild(ts.childOUID, "unit"))
	reshareBody := namedChild(ts.grandchildOUID, "unit")
	reshareBody["initiatingOuId"] = ts.childOUID
	ts.share(ts.rootClient, userID, reshareBody)

	status, raw := ts.do(ts.rootClient, http.MethodDelete, "/users/"+userID, nil)
	ts.Require().Equal(http.StatusNoContent, status, string(raw))

	status, raw = ts.do(ts.rootClient, http.MethodGet, policyPath(userID, share.ID), nil)
	ts.expectError(status, raw, http.StatusNotFound, "USR-1003")
}

// A path below a user must not reach the user itself.
func (ts *UserSharingAPITestSuite) TestUnknownSubPathDoesNotDeleteTheUser() {
	status, _ := ts.do(ts.rootClient, http.MethodDelete, "/users/"+ts.sharedUserID+"/anything", nil)
	ts.Equal(http.StatusNotFound, status)

	status, raw := ts.do(ts.rootClient, http.MethodGet, "/users/"+ts.sharedUserID, nil)
	ts.Equal(http.StatusOK, status, string(raw))
}

// accessibleUnits is the part of the accessible-units response these tests read.
type accessibleUnits struct {
	Current           string `json:"current"`
	TotalResults      int    `json:"totalResults"`
	StartIndex        int    `json:"startIndex"`
	OrganizationUnits []struct {
		ID string `json:"id"`
	} `json:"organizationUnits"`
	Links []struct {
		Href string `json:"href"`
		Rel  string `json:"rel"`
	} `json:"links"`
}

// link returns the href of the page link with the given rel, or "" when there is none.
func (l accessibleUnits) link(rel string) string {
	for _, link := range l.Links {
		if link.Rel == rel {
			return link.Href
		}
	}
	return ""
}

// accessible lists the units the client's user can access.
func (ts *UserSharingAPITestSuite) accessible(client *http.Client, query string) (accessibleUnits, []string) {
	ts.T().Helper()
	status, raw := ts.do(client, http.MethodGet, "/users/me/ous"+query, nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	var list accessibleUnits
	ts.Require().NoError(json.Unmarshal(raw, &list))
	ids := make([]string, 0, len(list.OrganizationUnits))
	for _, ou := range list.OrganizationUnits {
		ids = append(ids, ou.ID)
	}
	return list, ids
}

func (ts *UserSharingAPITestSuite) TestASharedUserListsTheUnitsItCanAccess() {
	// The user's own unit is top-level, so another top-level unit is its sibling.
	otherRootID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "user-sharing-other-root", Name: "User Sharing Other Root",
	})
	ts.Require().NoError(err, "create another top-level OU")
	defer func() {
		if err := testutils.DeleteOrganizationUnit(otherRootID); err != nil {
			ts.T().Logf("cleanup: delete OU %s: %v", otherRootID, err)
		}
	}()

	shareBody := namedChild(ts.childOUID, "unit")
	shareBody["otherTreeRoots"] = map[string]interface{}{
		"mode": "selected", "units": []map[string]string{{"ouId": otherRootID}},
	}
	share := ts.share(ts.rootClient, ts.sharedUserID, shareBody)
	defer ts.do(ts.rootClient, http.MethodDelete, policyPath(ts.sharedUserID, share.ID), nil)
	reshareBody := namedChild(ts.grandchildOUID, "unit")
	reshareBody["initiatingOuId"] = ts.childOUID
	ts.share(ts.rootClient, ts.sharedUserID, reshareBody)

	userClient, err := testutils.GetHTTPClientForUser("user-sharing-target", sharingTestPassword)
	ts.Require().NoError(err, "sign in as the shared user")

	list, ids := ts.accessible(userClient, "")
	ts.Equal(ts.parentOUID, list.Current)
	ts.ElementsMatch([]string{ts.parentOUID, ts.childOUID, ts.grandchildOUID, otherRootID}, ids)
	ts.Equal(ts.parentOUID, ids[0], "the user's own unit comes first")

	// The child and grandchild sit beneath the current unit, not beside it.
	_, ids = ts.accessible(userClient, "?scope=siblings")
	ts.ElementsMatch([]string{ts.parentOUID, otherRootID}, ids)

	status, raw := ts.do(userClient, http.MethodGet, "/users/me/ous?scope=children", nil)
	ts.expectError(status, raw, http.StatusBadRequest, "USR-1036")

	// Taking the share away takes the reshare beneath it, and every shared unit drops out of the list.
	status, _ = ts.do(ts.rootClient, http.MethodDelete, policyPath(ts.sharedUserID, share.ID), nil)
	ts.Require().Equal(http.StatusNoContent, status)
	_, ids = ts.accessible(userClient, "")
	ts.Equal([]string{ts.parentOUID}, ids)
}

// The list pages like any other: a page short of the total links the next one, which holds the rest.
func (ts *UserSharingAPITestSuite) TestTheAccessibleUnitsListPages() {
	share := ts.share(ts.rootClient, ts.sharedUserID, namedChild(ts.childOUID, "unit"))
	defer ts.do(ts.rootClient, http.MethodDelete, policyPath(ts.sharedUserID, share.ID), nil)
	reshareBody := namedChild(ts.grandchildOUID, "unit")
	reshareBody["initiatingOuId"] = ts.childOUID
	ts.share(ts.rootClient, ts.sharedUserID, reshareBody)

	userClient, err := testutils.GetHTTPClientForUser("user-sharing-target", sharingTestPassword)
	ts.Require().NoError(err, "sign in as the shared user")

	first, firstIDs := ts.accessible(userClient, "?limit=2")
	ts.Equal(3, first.TotalResults)
	ts.Equal(1, first.StartIndex)
	ts.Len(firstIDs, 2)
	next := first.link("next")
	ts.Require().True(strings.HasPrefix(next, "/users/me/ous?"), "next link: %q", next)

	second, secondIDs := ts.accessible(userClient, strings.TrimPrefix(next, "/users/me/ous"))
	ts.Equal(3, second.StartIndex)
	ts.Empty(second.link("next"), "the last page links no further")
	ts.ElementsMatch([]string{ts.parentOUID, ts.childOUID, ts.grandchildOUID}, append(firstIDs, secondIDs...))
}

// A user nobody has shared lists its own unit alone, whatever the scope.
func (ts *UserSharingAPITestSuite) TestAnUnsharedUserListsOnlyItsOwnUnit() {
	client, err := testutils.GetHTTPClientForUser("user-sharing-child-admin", sharingTestPassword)
	ts.Require().NoError(err, "sign in as the child admin")

	for _, query := range []string{"", "?scope=siblings"} {
		list, ids := ts.accessible(client, query)
		ts.Equal(ts.childOUID, list.Current, "scope %q", query)
		ts.Equal([]string{ts.childOUID}, ids, "scope %q", query)
	}
}

// A user declared in a resource file declares its sharing there too. The declared policy reads like
// any other but cannot be changed or removed here, and the owner may not add another; a unit the
// file shares the user into may still reshare, since there is nowhere else to record that.
func (ts *UserSharingAPITestSuite) TestADeclaredUsersSharingComesFromItsFile() {
	const declUserID, declPolicyID, sharedInto = "decl-user-1", "decl-user-1-policy-1", "decl-m2m-root"
	path := "/users/" + declUserID + "/sharing-policies"

	status, raw := ts.do(ts.rootClient, http.MethodGet, path, nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	var list sharingPolicyList
	ts.Require().NoError(json.Unmarshal(raw, &list))
	ts.Require().Equal(1, list.TotalResults, string(raw))
	declared := list.Policies[0]
	ts.Equal(declPolicyID, declared.ID)
	ts.Equal("share", declared.Stage)
	ts.True(declared.IsReadOnly, "a policy from a resource file is read-only")

	status, raw = ts.do(ts.rootClient, http.MethodGet, overlayPath(declUserID, sharedInto), nil)
	ts.Equal(http.StatusOK, status, "the file shared the user there: %s", raw)

	edit := map[string]interface{}{
		"otherTreeRoots": map[string]interface{}{
			"mode": "selected", "units": []map[string]string{{"ouId": sharedInto}},
		},
		"version": declared.Version,
	}
	status, raw = ts.do(ts.rootClient, http.MethodPut, policyPath(declUserID, declPolicyID), edit)
	ts.expectError(status, raw, http.StatusBadRequest, "USR-1033")
	status, raw = ts.do(ts.rootClient, http.MethodDelete, policyPath(declUserID, declPolicyID), nil)
	ts.expectError(status, raw, http.StatusBadRequest, "USR-1033")

	status, raw = ts.do(ts.rootClient, http.MethodPost, path,
		map[string]interface{}{"childUnits": map[string]interface{}{"mode": "all"}})
	ts.expectError(status, raw, http.StatusBadRequest, "USR-1025")

	reshare := namedChild("decl-m2m-child-a", "unit")
	reshare["initiatingOuId"] = sharedInto
	created := ts.share(ts.rootClient, declUserID, reshare)
	defer ts.do(ts.rootClient, http.MethodDelete, policyPath(declUserID, created.ID), nil)
	ts.Equal("reshare", created.Stage)
	ts.Equal(declPolicyID, created.ParentPolicyID)
}

// A user's sharing travels with it through export and import. Exported, then deleted (which takes
// its policies with it), the user comes back from its document shared as it was, reshare included.
func (ts *UserSharingAPITestSuite) TestAUsersSharingSurvivesExportAndImport() {
	userID, err := testutils.CreateUser(testutils.User{
		Type: sharingTestUserType, OUID: ts.parentOUID,
		Attributes: json.RawMessage(`{"username": "user-sharing-roundtrip"}`),
	})
	ts.Require().NoError(err, "create the user to export")
	defer func() { _ = testutils.DeleteUser(userID) }()
	ts.share(ts.rootClient, userID, namedChild(ts.childOUID, "unit"))
	reshareBody := namedChild(ts.grandchildOUID, "unit")
	reshareBody["initiatingOuId"] = ts.childOUID
	ts.share(ts.rootClient, userID, reshareBody)

	status, raw := ts.do(ts.rootClient, http.MethodPost, "/export",
		map[string]interface{}{"users": []string{userID}})
	ts.Require().Equal(http.StatusOK, status, string(raw))
	var exported struct {
		Resources string `json:"resources"`
	}
	ts.Require().NoError(json.Unmarshal(raw, &exported))
	ts.Require().Contains(exported.Resources, "sharingPolicies", "the export carries the user's sharing")

	status, raw = ts.do(ts.rootClient, http.MethodDelete, "/users/"+userID, nil)
	ts.Require().Equal(http.StatusNoContent, status, string(raw))

	status, raw = ts.do(ts.rootClient, http.MethodPost, "/import", map[string]interface{}{
		"content": exported.Resources,
		"options": map[string]interface{}{"upsert": true, "continueOnError": false},
	})
	ts.Require().Equal(http.StatusOK, status, string(raw))
	var imported struct {
		Summary struct {
			Failed int `json:"failed"`
		} `json:"summary"`
	}
	ts.Require().NoError(json.Unmarshal(raw, &imported))
	ts.Require().Zero(imported.Summary.Failed, string(raw))

	status, raw = ts.do(ts.rootClient, http.MethodGet, "/users/"+userID+"/sharing-policies", nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	var list sharingPolicyList
	ts.Require().NoError(json.Unmarshal(raw, &list))
	ts.Require().Equal(2, list.TotalResults, string(raw))
	stages := map[string]string{}
	for _, p := range list.Policies {
		stages[p.InitiatingOUID] = p.Stage
	}
	ts.Equal(map[string]string{ts.parentOUID: "share", ts.childOUID: "reshare"}, stages)

	status, raw = ts.do(ts.rootClient, http.MethodGet, overlayPath(userID, ts.grandchildOUID), nil)
	ts.Equal(http.StatusOK, status, "the reshare reaches the grandchild again: %s", raw)
}

// exportUser exports one user and returns the document.
func (ts *UserSharingAPITestSuite) exportUser(userID string) string {
	ts.T().Helper()
	status, raw := ts.do(ts.rootClient, http.MethodPost, "/export",
		map[string]interface{}{"users": []string{userID}})
	ts.Require().Equal(http.StatusOK, status, string(raw))
	var exported struct {
		Resources string `json:"resources"`
	}
	ts.Require().NoError(json.Unmarshal(raw, &exported))
	return exported.Resources
}

// importDocument imports a document, updating what already exists, and returns the id of the
// resource it imported.
func (ts *UserSharingAPITestSuite) importDocument(content string) string {
	ts.T().Helper()
	status, raw := ts.do(ts.rootClient, http.MethodPost, "/import", map[string]interface{}{
		"content": content,
		"options": map[string]interface{}{"upsert": true, "continueOnError": false},
	})
	ts.Require().Equal(http.StatusOK, status, string(raw))
	var imported struct {
		Summary struct {
			Failed int `json:"failed"`
		} `json:"summary"`
		Results []struct {
			ResourceID string `json:"resourceId"`
		} `json:"results"`
	}
	ts.Require().NoError(json.Unmarshal(raw, &imported))
	ts.Require().Zero(imported.Summary.Failed, string(raw))
	ts.Require().Len(imported.Results, 1, string(raw))
	return imported.Results[0].ResourceID
}

// policiesOf lists a user's sharing policies.
func (ts *UserSharingAPITestSuite) policiesOf(userID string) []sharingPolicy {
	ts.T().Helper()
	status, raw := ts.do(ts.rootClient, http.MethodGet, "/users/"+userID+"/sharing-policies", nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	var list sharingPolicyList
	ts.Require().NoError(json.Unmarshal(raw, &list))
	return list.Policies
}

// createUnsharedUser creates a user with no credential, so its export needs no variables back.
func (ts *UserSharingAPITestSuite) createUnsharedUser(username string) string {
	ts.T().Helper()
	userID, err := testutils.CreateUser(testutils.User{
		Type: sharingTestUserType, OUID: ts.parentOUID,
		Attributes: json.RawMessage(fmt.Sprintf(`{"username": %q}`, username)),
	})
	ts.Require().NoError(err, "create user %s", username)
	return userID
}

// Importing a user's own document over it is a replay, not an edit: each unit keeps its one policy,
// under the same id and at the same version.
func (ts *UserSharingAPITestSuite) TestReimportingAUserLeavesItsSharingAsItWas() {
	userID := ts.createUnsharedUser("user-sharing-reimport")
	defer func() { _ = testutils.DeleteUser(userID) }()
	ts.share(ts.rootClient, userID, namedChild(ts.childOUID, "unit"))
	reshareBody := namedChild(ts.grandchildOUID, "unit")
	reshareBody["initiatingOuId"] = ts.childOUID
	ts.share(ts.rootClient, userID, reshareBody)
	before := ts.policiesOf(userID)

	ts.Equal(userID, ts.importDocument(ts.exportUser(userID)), "the document updates the user it came from")

	ts.ElementsMatch(before, ts.policiesOf(userID))
}

// Deleting a user takes its sharing with it, so a document brought back under the same id without
// any policy leaves the user shared with nobody.
func (ts *UserSharingAPITestSuite) TestADeletedUserComesBackUnsharedWithoutItsPolicies() {
	userID := ts.createUnsharedUser("user-sharing-deleted")
	defer func() { _ = testutils.DeleteUser(userID) }()
	ts.share(ts.rootClient, userID, namedChild(ts.childOUID, "unit"))

	content := testutils.WithoutTopLevelKey(ts.exportUser(userID), "sharingPolicies")
	ts.Require().NotContains(content, "sharingPolicies", "the document brought back declares no policy")
	status, raw := ts.do(ts.rootClient, http.MethodDelete, "/users/"+userID, nil)
	ts.Require().Equal(http.StatusNoContent, status, string(raw))

	ts.Require().Equal(userID, ts.importDocument(content), "the document brings the user back under its own id")
	ts.Empty(ts.policiesOf(userID), "the deleted user's policies went with it")
}

// A copied document still carries the source's policy ids. The copy's sharing is stored under ids
// of its own, and the source keeps its policy.
func (ts *UserSharingAPITestSuite) TestImportingACopiedUserGetsItsOwnSharing() {
	sourceID := ts.createUnsharedUser("user-sharing-copy-source")
	defer func() { _ = testutils.DeleteUser(sourceID) }()
	sourcePolicy := ts.share(ts.rootClient, sourceID, namedChild(ts.childOUID, "unit"))

	// The copy is a new user: no id of its own yet, and another username.
	content := testutils.WithoutTopLevelKey(ts.exportUser(sourceID), "id")
	content = strings.Replace(content, "username: user-sharing-copy-source", "username: user-sharing-copy", 1)
	copyID := ts.importDocument(content)
	defer func() { _ = testutils.DeleteUser(copyID) }()
	ts.Require().NotEqual(sourceID, copyID, "the copy is a new user")

	copied := ts.policiesOf(copyID)
	ts.Require().Len(copied, 1, "the copy carries the sharing the document declared")
	ts.NotEqual(sourcePolicy.ID, copied[0].ID, "under an id of its own")
	source := ts.policiesOf(sourceID)
	ts.Require().Len(source, 1)
	ts.Equal(sourcePolicy.ID, source[0].ID, "the source keeps its policy")
}

// A user shared nowhere exports a document that declares no sharing.
func (ts *UserSharingAPITestSuite) TestAnUnsharedUserExportsNoSharing() {
	userID := ts.createUnsharedUser("user-sharing-unshared-export")
	defer func() { _ = testutils.DeleteUser(userID) }()

	ts.NotContains(ts.exportUser(userID), "sharingPolicies")
}

// A reach not bounded by the issuer's place in the tree is a deployment decision, so a document may
// carry one while the API refuses to create it. The API may still move its exclusions, but not trade
// the section for another.
func (ts *UserSharingAPITestSuite) TestImportCarriesADeploymentWideSharingPolicy() {
	userID := ts.importDocument(fmt.Sprintf(`resource_type: user
type: %s
ouId: %s
attributes:
  username: user-sharing-every-unit
sharingPolicies:
  - allUnits: {}
`, sharingTestUserType, ts.parentOUID))
	defer func() { _ = testutils.DeleteUser(userID) }()

	type everyUnitPolicy struct {
		ID         string `json:"id"`
		Version    int    `json:"version"`
		IsReadOnly bool   `json:"isReadOnly"`
		AllUnits   *struct {
			ExcludedOUIDs []string `json:"excludedOuIds"`
		} `json:"allUnits"`
	}
	put := func(path string, body map[string]interface{}) (int, everyUnitPolicy, []byte) {
		status, raw := ts.do(ts.rootClient, http.MethodPut, path, body)
		var p everyUnitPolicy
		_ = json.Unmarshal(raw, &p)
		return status, p, raw
	}

	status, raw := ts.do(ts.rootClient, http.MethodGet, "/users/"+userID+"/sharing-policies", nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	var list struct {
		Policies []everyUnitPolicy `json:"policies"`
	}
	ts.Require().NoError(json.Unmarshal(raw, &list))
	ts.Require().Len(list.Policies, 1, "the document's policy was recorded")
	policy := list.Policies[0]
	ts.Require().NotNil(policy.AllUnits, "the policy reaches every organization unit")
	ts.False(policy.IsReadOnly, "an imported policy is stored, not declared")

	status, raw = ts.do(ts.rootClient, http.MethodPost, "/users/"+userID+"/sharing-policies",
		map[string]interface{}{"otherTreeRoots": map[string]interface{}{"mode": "all"}})
	ts.expectError(status, raw, http.StatusBadRequest, "USR-1037")

	path := policyPath(userID, policy.ID)
	status, excluded, raw := put(path, map[string]interface{}{
		"version": policy.Version, "allUnits": map[string]interface{}{"excludedOuIds": []string{ts.childOUID}},
	})
	ts.Require().Equal(http.StatusOK, status, string(raw))
	ts.Require().NotNil(excluded.AllUnits)
	ts.Equal([]string{ts.childOUID}, excluded.AllUnits.ExcludedOUIDs)

	status, handedBack, raw := put(path, map[string]interface{}{
		"version": excluded.Version, "allUnits": map[string]interface{}{},
	})
	ts.Require().Equal(http.StatusOK, status, string(raw))
	ts.Require().NotNil(handedBack.AllUnits)
	ts.Empty(handedBack.AllUnits.ExcludedOUIDs)

	status, _, raw = put(path, map[string]interface{}{
		"version": handedBack.Version, "childUnits": map[string]interface{}{"mode": "all"},
	})
	ts.expectError(status, raw, http.StatusBadRequest, "USR-1029")

	status, raw = ts.do(ts.rootClient, http.MethodDelete, path, nil)
	ts.Require().Equal(http.StatusNoContent, status, string(raw))
	ts.Empty(ts.policiesOf(userID), "an imported policy is deleted like any other")
}

// everyAccessible lists every unit the client's user can access, following the page links.
func (ts *UserSharingAPITestSuite) everyAccessible(client *http.Client, query string) []string {
	ts.T().Helper()
	list, all := ts.accessible(client, query)
	for next := list.link("next"); next != ""; next = list.link("next") {
		ts.Require().True(strings.HasPrefix(next, "/users/me/ous?"), "next link: %q", next)
		var ids []string
		list, ids = ts.accessible(client, strings.TrimPrefix(next, "/users/me/ous"))
		all = append(all, ids...)
	}
	return all
}

// Each kind of grant reaches what it names: a subtree the unit and everything beneath it, every
// child unit everything beneath the owner.
func (ts *UserSharingAPITestSuite) TestEachKindOfShareListsWhatItReaches() {
	cases := []struct {
		name string
		body map[string]interface{}
	}{
		{name: "Subtree", body: namedChild(ts.childOUID, "subtree")},
		{name: "EveryChildUnit", body: map[string]interface{}{"childUnits": map[string]interface{}{"mode": "all"}}},
	}
	userClient, err := testutils.GetHTTPClientForUser("user-sharing-target", sharingTestPassword)
	ts.Require().NoError(err, "sign in as the shared user")

	for _, tc := range cases {
		ts.Run(tc.name, func() {
			share := ts.share(ts.rootClient, ts.sharedUserID, tc.body)
			defer ts.do(ts.rootClient, http.MethodDelete, policyPath(ts.sharedUserID, share.ID), nil)

			ts.ElementsMatch([]string{ts.parentOUID, ts.childOUID, ts.grandchildOUID},
				ts.everyAccessible(userClient, ""))
		})
	}
}

// A user an import shares with every unit can access every unit, including the roots of other trees.
func (ts *UserSharingAPITestSuite) TestAUserSharedWithEveryUnitListsEveryUnit() {
	userID := ts.importDocument(fmt.Sprintf(`resource_type: user
type: %s
ouId: %s
attributes:
  username: user-sharing-everywhere
credentials:
  password: %q
sharingPolicies:
  - allUnits: {}
`, sharingTestUserType, ts.parentOUID, sharingTestPassword))
	defer func() { _ = testutils.DeleteUser(userID) }()
	client, err := testutils.GetHTTPClientForUser("user-sharing-everywhere", sharingTestPassword)
	ts.Require().NoError(err, "sign in as the user shared everywhere")

	every := ts.everyAccessible(client, "")
	ts.Equal(ts.parentOUID, every[0], "the user's own unit comes first")
	ts.Subset(every, []string{ts.childOUID, ts.grandchildOUID, "decl-m2m-root"})
	ts.Contains(ts.everyAccessible(client, "?scope=siblings"), "decl-m2m-root",
		"another tree's root sits beside the user's own top-level unit")
}

// A unit a share names can be deleted while the share still names it. It is no longer anywhere to go.
func (ts *UserSharingAPITestSuite) TestAUnitDeletedAfterItWasSharedIsNotListed() {
	doomedID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "user-sharing-doomed", Name: "User Sharing Doomed", Parent: &ts.parentOUID,
	})
	ts.Require().NoError(err, "create the unit to delete")
	share := ts.share(ts.rootClient, ts.sharedUserID, map[string]interface{}{"childUnits": map[string]interface{}{
		"mode": "selected", "units": []map[string]string{
			{"ouId": ts.childOUID, "reach": "unit"}, {"ouId": doomedID, "reach": "unit"},
		},
	}})
	defer ts.do(ts.rootClient, http.MethodDelete, policyPath(ts.sharedUserID, share.ID), nil)
	userClient, err := testutils.GetHTTPClientForUser("user-sharing-target", sharingTestPassword)
	ts.Require().NoError(err, "sign in as the shared user")
	ts.ElementsMatch([]string{ts.parentOUID, ts.childOUID, doomedID}, ts.everyAccessible(userClient, ""))

	ts.Require().NoError(testutils.DeleteOrganizationUnit(doomedID), "delete a unit the share names")

	ts.ElementsMatch([]string{ts.parentOUID, ts.childOUID}, ts.everyAccessible(userClient, ""))
}

// importResult is one resource's outcome in an import.
type importResult struct {
	ResourceID string `json:"resourceId"`
	Operation  string `json:"operation"`
	Status     string `json:"status"`
	Code       string `json:"code"`
}

// importOneRefused imports a document holding one resource the import is expected to refuse.
func (ts *UserSharingAPITestSuite) importOneRefused(content string) importResult {
	ts.T().Helper()
	status, raw := ts.do(ts.rootClient, http.MethodPost, "/import", map[string]interface{}{
		"content": content,
		"options": map[string]interface{}{"upsert": true, "continueOnError": false},
	})
	ts.Require().Equal(http.StatusOK, status, string(raw))
	var imported struct {
		Results []importResult `json:"results"`
	}
	ts.Require().NoError(json.Unmarshal(raw, &imported))
	ts.Require().Len(imported.Results, 1, string(raw))
	result := imported.Results[0]
	ts.Require().Equal("failed", result.Status, string(raw))
	ts.NotEmpty(result.Code, string(raw))
	return result
}

// grandchildPolicyDoc is a user document whose one policy names the owner's grandchild, which a
// policy may not reach in one step.
func (ts *UserSharingAPITestSuite) grandchildPolicyDoc(idLine, username string) string {
	return fmt.Sprintf(`resource_type: user
%stype: %s
ouId: %s
attributes:
  username: %s
sharingPolicies:
  - childUnits:
      mode: selected
      units:
        - ouId: %s
          reach: unit
`, idLine, sharingTestUserType, ts.parentOUID, username, ts.grandchildOUID)
}

// A new user whose sharing the import refuses is removed again, so a retry does not meet it.
func (ts *UserSharingAPITestSuite) TestARefusedPolicyUndoesTheImportedUser() {
	result := ts.importOneRefused(ts.grandchildPolicyDoc("", "user-sharing-refused-create"))
	ts.Require().NotEmpty(result.ResourceID)

	status, raw := ts.do(ts.rootClient, http.MethodGet, "/users/"+result.ResourceID, nil)
	ts.Equal(http.StatusNotFound, status, "the user the import created is gone: %s", raw)
}

// An existing user whose sharing the import refuses keeps the sharing it had.
func (ts *UserSharingAPITestSuite) TestARefusedPolicyLeavesAnExistingUsersSharingAlone() {
	userID := ts.createUnsharedUser("user-sharing-refused-update")
	defer func() { _ = testutils.DeleteUser(userID) }()

	result := ts.importOneRefused(ts.grandchildPolicyDoc("id: "+userID+"\n", "user-sharing-refused-update"))
	ts.Equal(userID, result.ResourceID)
	ts.Equal("update", result.Operation)

	ts.Empty(ts.policiesOf(userID), "nothing the document stated was recorded")
}

// A shared user stays in its unit, since its policies were issued against it. Once its sharing is
// deleted it can move.
func (ts *UserSharingAPITestSuite) TestASharedUserMovesOnlyOnceItsSharingIsDeleted() {
	userID := ts.createUnsharedUser("user-sharing-mover")
	defer func() { _ = testutils.DeleteUser(userID) }()
	share := ts.share(ts.rootClient, userID, namedChild(ts.childOUID, "unit"))
	move := map[string]interface{}{
		"type": sharingTestUserType, "ouId": ts.childOUID,
		"attributes": map[string]interface{}{"username": "user-sharing-mover"},
	}

	status, raw := ts.do(ts.rootClient, http.MethodPut, "/users/"+userID, move)
	ts.expectError(status, raw, http.StatusConflict, "USR-1038")

	status, raw = ts.do(ts.rootClient, http.MethodDelete, policyPath(userID, share.ID), nil)
	ts.Require().Equal(http.StatusNoContent, status, string(raw))
	status, raw = ts.do(ts.rootClient, http.MethodPut, "/users/"+userID, move)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	var moved struct {
		OUID string `json:"ouId"`
	}
	ts.Require().NoError(json.Unmarshal(raw, &moved))
	ts.Equal(ts.childOUID, moved.OUID)
}

// A share into every other tree's root reaches those roots only: what lies beneath them, and the
// user's own tree, are reached only through shares of their own.
func (ts *UserSharingAPITestSuite) TestAUserSharedWithEveryOtherRootListsOnlyTheRoots() {
	userID := ts.importDocument(fmt.Sprintf(`resource_type: user
type: %s
ouId: %s
attributes:
  username: user-sharing-every-root
credentials:
  password: %q
sharingPolicies:
  - otherTreeRoots:
      mode: all
`, sharingTestUserType, ts.parentOUID, sharingTestPassword))
	defer func() { _ = testutils.DeleteUser(userID) }()
	client, err := testutils.GetHTTPClientForUser("user-sharing-every-root", sharingTestPassword)
	ts.Require().NoError(err, "sign in as the user shared with every other root")

	every := ts.everyAccessible(client, "")
	ts.Equal(ts.parentOUID, every[0], "the user's own unit comes first")
	ts.Contains(every, "decl-m2m-root", "another tree's root")
	ts.NotContains(every, "decl-m2m-child-a", "a unit beneath another tree's root")
	ts.NotContains(every, ts.childOUID, "a unit beneath the user's own unit")
}
