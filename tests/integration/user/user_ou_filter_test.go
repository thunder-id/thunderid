// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// UserOUFilterTestSuite exercises listing the users of one organization unit: the ones it owns,
// followed by the ones shared into it, through every endpoint that lists an organization unit's users.
//
// Fixture topology:
//
//	parent (ou-filter-parent)              ← owns one user per kind of sharing policy
//	├── child (ou-filter-child)            ← owns one user and the child admin
//	│   └── grandchild (ou-filter-grandchild) ← owns one user
//	└── sibling (ou-filter-sibling)
//	other root (ou-filter-other-root)      ← the root of another tree
//
// Each parent user carries one kind of policy, so every organization unit's expected list follows
// from what that kind reaches.
type UserOUFilterTestSuite struct {
	suite.Suite

	parentOUID     string
	childOUID      string
	grandchildOUID string
	siblingOUID    string
	otherRootOUID  string
	userTypeID     string

	// users holds every user the suite created, by name.
	users map[string]string
	// expected holds, per organization unit, the names of the users it owns and the ones shared into it.
	expected map[string]ouExpectation

	adminRSID   string
	adminRoleID string
	childAdmin  *http.Client
	rootClient  *http.Client
	ouHandleFor map[string]string
}

// ouExpectation is what one organization unit's user list holds.
type ouExpectation struct {
	owned  []string
	shared []string
}

const (
	ouFilterTestUserType = "ou-filter-test-type"
	ouFilterPageSize     = 4
)

func TestUserOUFilterTestSuite(t *testing.T) {
	suite.Run(t, new(UserOUFilterTestSuite))
}

func (ts *UserOUFilterTestSuite) SetupSuite() {
	ts.rootClient = testutils.GetHTTPClient()
	ts.users = map[string]string{}

	ts.parentOUID = ts.createOU("ou-filter-parent", nil)
	ts.childOUID = ts.createOU("ou-filter-child", &ts.parentOUID)
	ts.grandchildOUID = ts.createOU("ou-filter-grandchild", &ts.childOUID)
	ts.siblingOUID = ts.createOU("ou-filter-sibling", &ts.parentOUID)
	ts.otherRootOUID = ts.createOU("ou-filter-other-root", nil)
	ts.ouHandleFor = map[string]string{
		ts.parentOUID:     "ou-filter-parent",
		ts.childOUID:      "ou-filter-parent/ou-filter-child",
		ts.grandchildOUID: "ou-filter-parent/ou-filter-child/ou-filter-grandchild",
		ts.siblingOUID:    "ou-filter-parent/ou-filter-sibling",
		ts.otherRootOUID:  "ou-filter-other-root",
	}

	var err error
	ts.userTypeID, err = testutils.CreateUserType(testutils.UserType{
		Handle:      ouFilterTestUserType,
		DisplayName: "OU Filter Test Type",
		OUID:        ts.parentOUID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
			"password": map[string]interface{}{"type": "string", "credential": true},
		},
	})
	ts.Require().NoError(err, "create user type")

	// Users the child and grandchild own.
	ts.createUser("child-owned", ts.childOUID)
	ts.createUser("grandchild-owned", ts.grandchildOUID)
	ts.childAdmin = ts.childAdminClient()

	// One parent user per kind of policy the API accepts.
	ts.shareNew("named-unit", namedChild(ts.childOUID, "unit"))
	ts.shareNew("named-subtree", namedChild(ts.childOUID, "subtree"))
	ts.shareNew("all-children", map[string]interface{}{"childUnits": map[string]interface{}{"mode": "all"}})
	ts.shareNew("all-children-but-child", map[string]interface{}{"childUnits": map[string]interface{}{
		"mode": "all", "excludedOuIds": []string{ts.childOUID},
	}})
	ts.shareNew("other-root", ts.otherRoots())
	combined := namedChild(ts.childOUID, "unit")
	combined["otherTreeRoots"] = ts.otherRoots()["otherTreeRoots"]
	ts.shareNew("child-and-other-root", combined)

	// A share the child passes on to the grandchild.
	ts.shareNew("reshared", namedChild(ts.childOUID, "unit"))
	reshare := namedChild(ts.grandchildOUID, "unit")
	reshare["initiatingOuId"] = ts.childOUID
	ts.share(ts.users["reshared"], reshare)

	// A share taken away again reaches nothing.
	revoked := ts.shareNew("revoked", namedChild(ts.childOUID, "unit"))
	status, raw := sendRequest(ts.T(), ts.rootClient, http.MethodDelete,
		"/users/"+ts.users["revoked"]+"/sharing-policies/"+revoked, nil)
	ts.Require().Equal(http.StatusNoContent, status, string(raw))

	// Deployment-wide reach is refused on create through the API, so these come in through an import.
	ts.importUser("all-units", "  - allUnits: {}\n")
	ts.importUser("all-units-but-sibling",
		fmt.Sprintf("  - allUnits:\n      excludedOuIds:\n        - %s\n", ts.siblingOUID))
	ts.importUser("every-other-root", "  - otherTreeRoots:\n      mode: all\n")

	ts.expected = map[string]ouExpectation{
		ts.parentOUID: {owned: []string{
			"named-unit", "named-subtree", "all-children", "all-children-but-child", "other-root",
			"child-and-other-root", "reshared", "revoked", "all-units", "all-units-but-sibling",
			"every-other-root",
		}},
		ts.childOUID: {
			owned: []string{"child-owned", "child-admin"},
			shared: []string{"named-unit", "named-subtree", "all-children", "child-and-other-root", "reshared",
				"all-units", "all-units-but-sibling"},
		},
		ts.grandchildOUID: {
			owned:  []string{"grandchild-owned"},
			shared: []string{"named-subtree", "all-children", "reshared", "all-units", "all-units-but-sibling"},
		},
		ts.siblingOUID: {
			shared: []string{"all-children", "all-children-but-child", "all-units"},
		},
		ts.otherRootOUID: {
			shared: []string{"other-root", "child-and-other-root", "all-units", "all-units-but-sibling",
				"every-other-root"},
		},
	}
}

func (ts *UserOUFilterTestSuite) TearDownSuite() {
	if ts.adminRoleID != "" {
		if err := testutils.DeleteRole(ts.adminRoleID); err != nil {
			ts.T().Logf("teardown: delete role: %v", err)
		}
	}
	if ts.adminRSID != "" {
		if err := testutils.DeleteResourceServerWithChildren(ts.adminRSID); err != nil {
			ts.T().Logf("teardown: delete resource server: %v", err)
		}
	}
	for name, id := range ts.users {
		if err := testutils.DeleteUser(id); err != nil {
			ts.T().Logf("teardown: delete user %s: %v", name, err)
		}
	}
	if ts.userTypeID != "" {
		if err := testutils.DeleteUserType(ts.userTypeID); err != nil {
			ts.T().Logf("teardown: delete user type: %v", err)
		}
	}
	for _, id := range []string{ts.grandchildOUID, ts.childOUID, ts.siblingOUID, ts.parentOUID, ts.otherRootOUID} {
		if id == "" {
			continue
		}
		if err := testutils.DeleteOrganizationUnit(id); err != nil {
			ts.T().Logf("teardown: delete OU %s: %v", id, err)
		}
	}
}

// createOU creates an organization unit and returns its ID.
func (ts *UserOUFilterTestSuite) createOU(handle string, parent *string) string {
	ts.T().Helper()
	id, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: handle, Name: handle, Parent: parent,
	})
	ts.Require().NoError(err, "create OU %s", handle)
	return id
}

// createUser creates a user of the suite's type and records it under name.
func (ts *UserOUFilterTestSuite) createUser(name, ouID string) string {
	ts.T().Helper()
	id, err := testutils.CreateUser(testutils.User{
		Type: ouFilterTestUserType,
		OUID: ouID,
		Attributes: json.RawMessage(fmt.Sprintf(`{"username": %q, "password": %q}`,
			"ou-filter-"+name, sharingTestPassword)),
	})
	ts.Require().NoError(err, "create user %s", name)
	ts.users[name] = id
	return id
}

// shareNew creates a parent user under name, shares it with body, and returns the policy ID.
func (ts *UserOUFilterTestSuite) shareNew(name string, body map[string]interface{}) string {
	ts.T().Helper()
	return ts.share(ts.createUser(name, ts.parentOUID), body)
}

// share records a policy for a user and returns its ID.
func (ts *UserOUFilterTestSuite) share(userID string, body map[string]interface{}) string {
	ts.T().Helper()
	status, raw := sendRequest(ts.T(), ts.rootClient, http.MethodPost, "/users/"+userID+"/sharing-policies", body)
	ts.Require().Equal(http.StatusCreated, status, string(raw))
	var policy struct {
		ID string `json:"id"`
	}
	ts.Require().NoError(json.Unmarshal(raw, &policy))
	return policy.ID
}

// otherRoots returns a policy body naming the other tree's root.
func (ts *UserOUFilterTestSuite) otherRoots() map[string]interface{} {
	return map[string]interface{}{"otherTreeRoots": map[string]interface{}{
		"mode": "selected", "units": []map[string]string{{"ouId": ts.otherRootOUID}},
	}}
}

// importUser imports a parent user under name carrying the given sharing policies.
func (ts *UserOUFilterTestSuite) importUser(name, policies string) {
	ts.T().Helper()
	content := fmt.Sprintf("resource_type: user\ntype: %s\nouId: %s\nattributes:\n  username: %s\n"+
		"sharingPolicies:\n%s", ouFilterTestUserType, ts.parentOUID, "ou-filter-"+name, policies)
	status, raw := sendRequest(ts.T(), ts.rootClient, http.MethodPost, "/import", map[string]interface{}{
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
	ts.users[name] = imported.Results[0].ResourceID
}

// childAdminClient creates a child user holding system:user in the child only, and returns a client
// carrying its token.
func (ts *UserOUFilterTestSuite) childAdminClient() *http.Client {
	ts.T().Helper()
	userID := ts.createUser("child-admin", ts.childOUID)
	client, rsID, roleID := scopedAdminClient(ts.T(), ts.childOUID, userID, "ou-filter-child-admin",
		"OU Filter Child", "https://ou-filter-test.example.com/child")
	ts.adminRSID, ts.adminRoleID = rsID, roleID
	return client
}

// list reads one page of a user list.
func (ts *UserOUFilterTestSuite) list(client *http.Client, path string) testutils.UserListResponse {
	ts.T().Helper()
	status, raw := sendRequest(ts.T(), client, http.MethodGet, path, nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	var list testutils.UserListResponse
	ts.Require().NoError(json.Unmarshal(raw, &list))
	return list
}

// everyPage reads a user list page by page through its next links, checking that every page reports
// the same total and that each link lists the same users as the first page did.
func (ts *UserOUFilterTestSuite) everyPage(client *http.Client, path string) []testutils.User {
	ts.T().Helper()
	list := ts.list(client, path)
	total := list.TotalResults
	users := list.Users
	for next := linkOf(list, "next"); next != ""; next = linkOf(list, "next") {
		ts.Require().Less(len(users), total, "a next link past the last user: %s", next)
		list = ts.list(client, next)
		ts.Equal(total, list.TotalResults, "every page reports the same total")
		users = append(users, list.Users...)
	}
	ts.Equal(total, len(users), "the pages together hold every user the total counts")
	return users
}

// linkOf returns the link of a relation, or an empty string.
func linkOf(list testutils.UserListResponse, rel string) string {
	for _, link := range list.Links {
		if link.Rel == rel {
			return link.Href
		}
	}
	return ""
}

// ouFilter returns the query string that lists one organization unit's users.
func ouFilter(ouID string) string {
	return "filter=" + url.QueryEscape(fmt.Sprintf("ouId eq %q", ouID))
}

// assertListed checks a full listing against what an organization unit owns and is shared: its own
// users first, then the shared ones, each group in ID order and marked accordingly. withOUID says the
// endpoint lists each user's organization unit, which for a shared user is the one it is shared into.
func (ts *UserOUFilterTestSuite) assertListed(ouID string, users []testutils.User, withOUID bool) {
	ts.T().Helper()
	want := ts.expected[ouID]
	ownedIDs := ts.idsOf(want.owned)
	sharedIDs := ts.idsOf(want.shared)

	got := make([]string, len(users))
	for i, u := range users {
		got[i] = u.ID
		ts.Require().NotNil(u.IsOwnedByOU, "user %s carries no isOwnedByOU field", u.ID)
		ts.Equal(i < len(ownedIDs), *u.IsOwnedByOU, "user %s at position %d", ts.nameOf(u.ID), i)
		if withOUID {
			ts.Equal(ouID, u.OUID, "user %s is listed under the filtered unit", ts.nameOf(u.ID))
		}
	}
	ts.Equal(ts.namesOf(append(ownedIDs, sharedIDs...)), ts.namesOf(got))
}

// idsOf returns the IDs of the named users, sorted.
func (ts *UserOUFilterTestSuite) idsOf(names []string) []string {
	ids := make([]string, len(names))
	for i, name := range names {
		ids[i] = ts.users[name]
	}
	slices.Sort(ids)
	return ids
}

// nameOf returns the name a user was created under, so a failure names users rather than IDs.
func (ts *UserOUFilterTestSuite) nameOf(id string) string {
	for name, userID := range ts.users {
		if userID == id {
			return name
		}
	}
	return id
}

// namesOf maps IDs to names, keeping their order.
func (ts *UserOUFilterTestSuite) namesOf(ids []string) []string {
	names := make([]string, len(ids))
	for i, id := range ids {
		names[i] = ts.nameOf(id)
	}
	return names
}

// Every endpoint that lists an organization unit's users lists the same users for every unit: the
// ones it owns, then every user a policy reaches it with. Pages are small, so most lists span pages
// and cross from owned to shared users in the middle of one.
func (ts *UserOUFilterTestSuite) TestEveryListingEndpointListsOwnedThenSharedUsers() {
	endpoints := []struct {
		name     string
		withOUID bool
		path     func(ouID string) string
	}{
		{"UsersFilter", true, func(ouID string) string { return "/users?" + ouFilter(ouID) }},
		{"OrganizationUnitUsers", false, func(ouID string) string {
			return "/organization-units/" + ouID + "/users"
		}},
		{"OrganizationUnitUsersByPath", false, func(ouID string) string {
			return "/organization-units/tree/" + ts.ouHandleFor[ouID] + "/users"
		}},
		{"UsersByPath", true, func(ouID string) string { return "/users/tree/" + ts.ouHandleFor[ouID] }},
	}
	units := []struct {
		name string
		id   string
	}{
		{"Parent", ts.parentOUID},
		{"Child", ts.childOUID},
		{"Grandchild", ts.grandchildOUID},
		{"Sibling", ts.siblingOUID},
		{"OtherRoot", ts.otherRootOUID},
	}
	for _, endpoint := range endpoints {
		for _, unit := range units {
			ts.Run(endpoint.name+"/"+unit.name, func() {
				path := endpoint.path(unit.id)
				separator := "?"
				if strings.Contains(path, "?") {
					separator = "&"
				}
				ts.assertListed(unit.id, ts.everyPage(ts.rootClient, fmt.Sprintf("%s%slimit=%d",
					path, separator, ouFilterPageSize)), endpoint.withOUID)
			})
		}
	}
}

// A shared user is listed with the handle of the unit it is shared into, not the one owning it.
func (ts *UserOUFilterTestSuite) TestSharedUsersCarryTheFilteredUnitHandle() {
	users := ts.everyPage(ts.rootClient, fmt.Sprintf("/users?%s&include=display&limit=%d",
		ouFilter(ts.childOUID), ouFilterPageSize))
	ts.Require().NotEmpty(users)
	for _, u := range users {
		ts.Equal(ts.childOUID, u.OUID, "user %s", ts.nameOf(u.ID))
		ts.Equal("ou-filter-child", u.OUHandle, "user %s", ts.nameOf(u.ID))
	}
}

// The next link of a filtered list carries the filter, or the second page would list every user.
func (ts *UserOUFilterTestSuite) TestPageLinksKeepTheFilter() {
	list := ts.list(ts.rootClient, fmt.Sprintf("/users?limit=%d&%s", ouFilterPageSize, ouFilter(ts.childOUID)))

	for _, rel := range []string{"next", "last"} {
		link := linkOf(list, rel)
		ts.Require().NotEmpty(link, "the first page has a %s link", rel)
		ts.Contains(link, ouFilter(ts.childOUID))
	}
}

// A page past the last user is empty but still reports the total.
func (ts *UserOUFilterTestSuite) TestAnOffsetPastTheEndListsNoUsers() {
	list := ts.list(ts.rootClient, "/users?offset=100&"+ouFilter(ts.childOUID))

	ts.Empty(list.Users)
	ts.Equal(len(ts.expected[ts.childOUID].owned)+len(ts.expected[ts.childOUID].shared), list.TotalResults)
}

// An unfiltered list holds every user in the unit that owns it.
func (ts *UserOUFilterTestSuite) TestUnfilteredListMarksUsersOwned() {
	list := ts.list(ts.rootClient, "/users?limit=20")

	ts.Require().NotEmpty(list.Users)
	for _, u := range list.Users {
		ts.Require().NotNil(u.IsOwnedByOU, "user %s", u.ID)
		ts.True(*u.IsOwnedByOU)
	}
}

// A caller allowed to list users in the child sees what the child owns and is shared.
func (ts *UserOUFilterTestSuite) TestAScopedCallerListsAUnitItMayList() {
	ts.assertListed(ts.childOUID, ts.everyPage(ts.childAdmin,
		fmt.Sprintf("/users?limit=%d&%s", ouFilterPageSize, ouFilter(ts.childOUID))), true)
}

// A caller not allowed to list users in a unit sees none there, rather than an error. A role in the
// child reaches the child only, so the grandchild below it is out of reach too.
func (ts *UserOUFilterTestSuite) TestAScopedCallerListsNoUsersInAUnitItMayNotList() {
	for _, ouID := range []string{ts.parentOUID, ts.grandchildOUID, ts.otherRootOUID} {
		list := ts.list(ts.childAdmin, "/users?"+ouFilter(ouID))

		ts.Zero(list.TotalResults)
		ts.Empty(list.Users)
	}
}

// Filtering on a unit that does not exist lists no users rather than failing.
func (ts *UserOUFilterTestSuite) TestFilterOnAnUnknownUnitListsNothing() {
	list := ts.list(ts.rootClient, "/users?"+ouFilter("00000000-0000-0000-0000-000000000000"))

	ts.Zero(list.TotalResults)
	ts.Empty(list.Users)
}

// The organization unit endpoints name the unit in the path, so an unknown one is not found.
func (ts *UserOUFilterTestSuite) TestAnUnknownUnitInThePathIsNotFound() {
	for _, path := range []string{
		"/organization-units/00000000-0000-0000-0000-000000000000/users",
		"/organization-units/tree/ou-filter-missing/users",
		"/users/tree/ou-filter-missing",
		"/users/tree/ou-filter-parent/ou-filter-missing",
	} {
		status, raw := sendRequest(ts.T(), ts.rootClient, http.MethodGet, path, nil)
		ts.Equal(http.StatusNotFound, status, "%s: %s", path, raw)
	}
}

// A filter on the organization unit takes a quoted ID with eq and nothing else.
func (ts *UserOUFilterTestSuite) TestAMalformedUnitFilterIsRefused() {
	for name, expression := range map[string]string{
		"an unquoted ID":                    "ouId eq " + ts.childOUID,
		"a number":                          "ouId eq 5",
		"an empty ID":                       `ouId eq ""`,
		"another operator":                  fmt.Sprintf("ouId sw %q", ts.childOUID),
		"no value":                          "ouId eq",
		"a second expression joined by and": fmt.Sprintf(`ouId eq %q and username eq "ou-filter-child-owned"`, ts.childOUID),
	} {
		ts.Run(name, func() {
			status, raw := sendRequest(ts.T(), ts.rootClient, http.MethodGet,
				"/users?filter="+url.QueryEscape(expression), nil)

			ts.Require().Equal(http.StatusBadRequest, status, string(raw))
			var e testutils.ErrorResponse
			ts.Require().NoError(json.Unmarshal(raw, &e))
			ts.Equal("USR-1020", e.Code)
		})
	}
}
