// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// ApplicationOUFilterTestSuite exercises listing the applications of one organization unit: the
// ones it owns, followed by the ones shared into it.
//
// Fixture topology:
//
//	parent (app-ou-filter-parent)              ← owns one application per kind of sharing policy
//	├── child (app-ou-filter-child)            ← owns one application
//	│   └── grandchild (app-ou-filter-grandchild) ← owns one application
//	└── sibling (app-ou-filter-sibling)
//	other root (app-ou-filter-other-root)      ← the root of another tree
type ApplicationOUFilterTestSuite struct {
	suite.Suite

	parentOUID     string
	childOUID      string
	grandchildOUID string
	siblingOUID    string
	otherRootOUID  string

	// apps holds every application the suite created, by name.
	apps map[string]string
	// expected holds, per organization unit, the names of the applications it owns and the ones
	// shared into it.
	expected map[string]appOUExpectation
}

// appOUExpectation is what one organization unit's application list holds.
type appOUExpectation struct {
	owned  []string
	shared []string
}

// ouListedApplication is the part of a listed application these tests read.
type ouListedApplication struct {
	ID          string `json:"id"`
	IsOwnedByOU *bool  `json:"isOwnedByOU"`
	IsReadOnly  bool   `json:"isReadOnly"`
}

// ouApplicationList is the part of an application list response these tests read.
type ouApplicationList struct {
	TotalResults int                   `json:"totalResults"`
	Count        int                   `json:"count"`
	Applications []ouListedApplication `json:"applications"`
}

// declaredAllUnitsAppID is the declarative application shared into every organization unit.
const declaredAllUnitsAppID = "decl-m2m-all-ous"

func TestApplicationOUFilterTestSuite(t *testing.T) {
	suite.Run(t, new(ApplicationOUFilterTestSuite))
}

func (ts *ApplicationOUFilterTestSuite) SetupSuite() {
	ts.apps = map[string]string{}

	ts.parentOUID = ts.createOU("app-ou-filter-parent", nil)
	ts.childOUID = ts.createOU("app-ou-filter-child", &ts.parentOUID)
	ts.grandchildOUID = ts.createOU("app-ou-filter-grandchild", &ts.childOUID)
	ts.siblingOUID = ts.createOU("app-ou-filter-sibling", &ts.parentOUID)
	ts.otherRootOUID = ts.createOU("app-ou-filter-other-root", nil)

	ts.createApp("child-owned", ts.childOUID)
	ts.createApp("grandchild-owned", ts.grandchildOUID)

	// One parent application per kind of policy.
	ts.shareNew("named-unit", selectedChildUnits(childUnit(ts.childOUID, "unit")))
	ts.shareNew("named-subtree", selectedChildUnits(childUnit(ts.childOUID, "subtree")))
	ts.shareNew("all-children", allChildUnits())
	ts.shareNew("all-children-but-child", allChildUnits(ts.childOUID))
	ts.share(ts.createApp("other-root", ts.parentOUID), map[string]interface{}{
		"otherTreeRoots": map[string]interface{}{
			"mode": "selected", "units": []map[string]string{{"ouId": ts.otherRootOUID}},
		},
	})

	// A share the child passes on to the grandchild.
	ts.shareNew("reshared", selectedChildUnits(childUnit(ts.childOUID, "unit")))
	ts.share(ts.apps["reshared"], map[string]interface{}{
		"initiatingOuId": ts.childOUID,
		"childUnits":     selectedChildUnits(childUnit(ts.grandchildOUID, "unit")),
	})

	// A share taken away again reaches nothing.
	revokedID := ts.createApp("revoked", ts.parentOUID)
	policyID := ts.share(revokedID, map[string]interface{}{
		"childUnits": selectedChildUnits(childUnit(ts.childOUID, "unit")),
	})
	status, raw := ts.request(http.MethodDelete, "/applications/"+revokedID+"/sharing-policies/"+policyID, nil)
	ts.Require().Equal(http.StatusNoContent, status, string(raw))

	ts.expected = map[string]appOUExpectation{
		ts.parentOUID: {
			owned: []string{
				"named-unit", "named-subtree", "all-children", "all-children-but-child", "other-root",
				"reshared", "revoked",
			},
			shared: []string{declaredAllUnitsAppID},
		},
		ts.childOUID: {
			owned:  []string{"child-owned"},
			shared: []string{"named-unit", "named-subtree", "all-children", "reshared", declaredAllUnitsAppID},
		},
		ts.grandchildOUID: {
			owned:  []string{"grandchild-owned"},
			shared: []string{"named-subtree", "all-children", "reshared", declaredAllUnitsAppID},
		},
		ts.siblingOUID: {
			shared: []string{"all-children", "all-children-but-child", declaredAllUnitsAppID},
		},
		ts.otherRootOUID: {
			shared: []string{"other-root", declaredAllUnitsAppID},
		},
	}
}

func (ts *ApplicationOUFilterTestSuite) TearDownSuite() {
	for name, id := range ts.apps {
		if err := testutils.DeleteApplication(id); err != nil {
			ts.T().Logf("teardown: delete application %s: %v", name, err)
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
func (ts *ApplicationOUFilterTestSuite) createOU(handle string, parent *string) string {
	ts.T().Helper()
	id, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: handle, Name: handle, Parent: parent,
	})
	ts.Require().NoError(err, "create OU %s", handle)
	return id
}

// createApp creates an application in an organization unit and records it under name.
func (ts *ApplicationOUFilterTestSuite) createApp(name, ouID string) string {
	ts.T().Helper()
	clientID := "app_ou_filter_" + name
	id, err := testutils.CreateApplication(testutils.Application{
		OUID:         ouID,
		Name:         "App OU Filter " + name,
		ClientID:     clientID,
		ClientSecret: clientID + "_secret",
	})
	ts.Require().NoError(err, "create application %s", name)
	ts.apps[name] = id
	return id
}

// shareNew creates a parent application under name and shares it with the given child units.
func (ts *ApplicationOUFilterTestSuite) shareNew(name string, childUnits map[string]interface{}) {
	ts.T().Helper()
	ts.share(ts.createApp(name, ts.parentOUID), map[string]interface{}{"childUnits": childUnits})
}

// share records a policy for an application and returns its ID.
func (ts *ApplicationOUFilterTestSuite) share(appID string, body map[string]interface{}) string {
	ts.T().Helper()
	status, raw := ts.request(http.MethodPost, "/applications/"+appID+"/sharing-policies", body)
	ts.Require().Equal(http.StatusCreated, status, string(raw))
	var policy struct {
		ID string `json:"id"`
	}
	ts.Require().NoError(json.Unmarshal(raw, &policy))
	return policy.ID
}

// request sends a request and returns its status and body.
func (ts *ApplicationOUFilterTestSuite) request(method, path string, body interface{}) (int, []byte) {
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
	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	return resp.StatusCode, raw
}

// list reads an application list.
func (ts *ApplicationOUFilterTestSuite) list(path string) ouApplicationList {
	ts.T().Helper()
	status, raw := ts.request(http.MethodGet, path, nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	var list ouApplicationList
	ts.Require().NoError(json.Unmarshal(raw, &list))
	return list
}

// appOUFilter returns the query string that lists one organization unit's applications.
func appOUFilter(ouID string) string {
	return "filter=" + url.QueryEscape(fmt.Sprintf("ouId eq %q", ouID))
}

// idsOf returns the IDs of the named applications, sorted. A declarative application's name is its ID.
func (ts *ApplicationOUFilterTestSuite) idsOf(names []string) []string {
	ids := make([]string, len(names))
	for i, name := range names {
		ids[i] = name
		if id, ok := ts.apps[name]; ok {
			ids[i] = id
		}
	}
	slices.Sort(ids)
	return ids
}

// nameOf returns the name an application was created under, so a failure names applications
// rather than IDs.
func (ts *ApplicationOUFilterTestSuite) nameOf(id string) string {
	for name, appID := range ts.apps {
		if appID == id {
			return name
		}
	}
	return id
}

// namesOf maps IDs to names, keeping their order.
func (ts *ApplicationOUFilterTestSuite) namesOf(ids []string) []string {
	names := make([]string, len(ids))
	for i, id := range ids {
		names[i] = ts.nameOf(id)
	}
	return names
}

// Every unit lists the applications it owns, then every application a policy reaches it with, each
// group in ID order and marked accordingly.
func (ts *ApplicationOUFilterTestSuite) TestEveryUnitListsOwnedThenSharedApplications() {
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
	for _, unit := range units {
		ts.Run(unit.name, func() {
			want := ts.expected[unit.id]
			ownedIDs := ts.idsOf(want.owned)
			sharedIDs := ts.idsOf(want.shared)

			list := ts.list("/applications?" + appOUFilter(unit.id))

			ts.Equal(len(ownedIDs)+len(sharedIDs), list.TotalResults)
			ts.Equal(len(list.Applications), list.Count)
			got := make([]string, len(list.Applications))
			for i, app := range list.Applications {
				got[i] = app.ID
				ts.Require().NotNil(app.IsOwnedByOU, "application %s carries no isOwnedByOU field", app.ID)
				ts.Equal(i < len(ownedIDs), *app.IsOwnedByOU, "application %s at position %d", ts.nameOf(app.ID), i)
				ts.Equal(i >= len(ownedIDs), app.IsReadOnly, "application %s is read-only only where it is shared",
					ts.nameOf(app.ID))
			}
			ts.Equal(ts.namesOf(append(ownedIDs, sharedIDs...)), ts.namesOf(got))
		})
	}
}

// An unfiltered list holds every application in the unit that owns it.
func (ts *ApplicationOUFilterTestSuite) TestUnfilteredListMarksApplicationsOwned() {
	list := ts.list("/applications")

	ts.Require().NotEmpty(list.Applications)
	for _, app := range list.Applications {
		ts.Require().NotNil(app.IsOwnedByOU, "application %s", app.ID)
		ts.True(*app.IsOwnedByOU, "application %s", app.ID)
	}
}

// Filtering on a unit that does not exist lists no applications rather than failing.
func (ts *ApplicationOUFilterTestSuite) TestFilterOnAnUnknownUnitListsNothing() {
	list := ts.list("/applications?" + appOUFilter("00000000-0000-0000-0000-000000000000"))

	ts.Zero(list.TotalResults)
	ts.Empty(list.Applications)
}

// A second filter parameter is refused rather than ignored.
func (ts *ApplicationOUFilterTestSuite) TestARepeatedFilterParameterIsRefused() {
	status, raw := ts.request(http.MethodGet, "/applications?"+appOUFilter(ts.childOUID)+
		"&filter="+url.QueryEscape(`name eq "x"`), nil)

	ts.Require().Equal(http.StatusBadRequest, status, string(raw))
	var e struct {
		Code string `json:"code"`
	}
	ts.Require().NoError(json.Unmarshal(raw, &e))
	ts.Equal("APP-1062", e.Code)
}

// A filter takes a quoted organization unit ID with eq and nothing else.
func (ts *ApplicationOUFilterTestSuite) TestAMalformedFilterIsRefused() {
	for name, expression := range map[string]string{
		"an empty filter":                   "",
		"an unquoted ID":                    "ouId eq " + ts.childOUID,
		"a number":                          "ouId eq 5",
		"an empty ID":                       `ouId eq ""`,
		"another operator":                  fmt.Sprintf("ouId sw %q", ts.childOUID),
		"another attribute":                 `name eq "App OU Filter child-owned"`,
		"no value":                          "ouId eq",
		"a second expression joined by and": fmt.Sprintf(`ouId eq %q and name eq "x"`, ts.childOUID),
	} {
		ts.Run(name, func() {
			status, raw := ts.request(http.MethodGet, "/applications?filter="+url.QueryEscape(expression), nil)

			ts.Require().Equal(http.StatusBadRequest, status, string(raw))
			var e struct {
				Code string `json:"code"`
			}
			ts.Require().NoError(json.Unmarshal(raw, &e))
			ts.Equal("APP-1062", e.Code)
		})
	}
}
