// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package group

import (
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

// GroupOUFilterTestSuite exercises listing the groups of one organization unit. Groups are never
// shared, so an organization unit lists only the groups it owns, from both the database and the
// declarative files.
//
// Fixture topology:
//
//	parent (group-ou-filter-parent)    ← owns three groups
//	└── child (group-ou-filter-child)  ← owns one group
//	empty (group-ou-filter-empty)      ← owns no groups
//	decl-ou-1                          ← owns the two declarative groups and one runtime group
type GroupOUFilterTestSuite struct {
	suite.Suite

	parentOUID string
	childOUID  string
	emptyOUID  string

	// owned holds, per organization unit, the IDs of the groups it owns, sorted.
	owned map[string][]string
	// created holds the IDs of the groups the suite created, to delete on teardown.
	created []string
}

func TestGroupOUFilterTestSuite(t *testing.T) {
	suite.Run(t, new(GroupOUFilterTestSuite))
}

func (ts *GroupOUFilterTestSuite) SetupSuite() {
	ts.owned = map[string][]string{declGroupOUID: {declGroupID, declNestedGroupID}}

	ts.parentOUID = ts.createOU("group-ou-filter-parent", nil)
	ts.childOUID = ts.createOU("group-ou-filter-child", &ts.parentOUID)
	ts.emptyOUID = ts.createOU("group-ou-filter-empty", nil)

	for _, name := range []string{"parent-1", "parent-2", "parent-3"} {
		ts.createGroup(name, ts.parentOUID)
	}
	ts.createGroup("child-1", ts.childOUID)
	ts.createGroup("decl-runtime-1", declGroupOUID)
}

func (ts *GroupOUFilterTestSuite) TearDownSuite() {
	for _, id := range ts.created {
		if err := testutils.DeleteGroup(id); err != nil {
			ts.T().Logf("teardown: delete group %s: %v", id, err)
		}
	}
	for _, id := range []string{ts.childOUID, ts.parentOUID, ts.emptyOUID} {
		if id == "" {
			continue
		}
		if err := testutils.DeleteOrganizationUnit(id); err != nil {
			ts.T().Logf("teardown: delete OU %s: %v", id, err)
		}
	}
}

// createOU creates an organization unit and returns its ID.
func (ts *GroupOUFilterTestSuite) createOU(handle string, parent *string) string {
	ts.T().Helper()
	id, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: handle, Name: handle, Parent: parent,
	})
	ts.Require().NoError(err, "create OU %s", handle)
	return id
}

// createGroup creates a group in an organization unit and records it as owned there.
func (ts *GroupOUFilterTestSuite) createGroup(name, ouID string) {
	ts.T().Helper()
	id, err := testutils.CreateGroup(testutils.Group{Name: "group-ou-filter-" + name, OUID: ouID})
	ts.Require().NoError(err, "create group %s", name)
	ts.created = append(ts.created, id)
	ts.owned[ouID] = append(ts.owned[ouID], id)
	slices.Sort(ts.owned[ouID])
}

// request sends a GET request and returns its status and body.
func (ts *GroupOUFilterTestSuite) request(pathAndQuery string) (int, []byte) {
	ts.T().Helper()
	req, err := http.NewRequest(http.MethodGet, testServerURL+pathAndQuery, nil)
	ts.Require().NoError(err)
	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	return resp.StatusCode, raw
}

// list reads a group list.
func (ts *GroupOUFilterTestSuite) list(pathAndQuery string) GroupListResponse {
	ts.T().Helper()
	status, raw := ts.request(pathAndQuery)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	var list GroupListResponse
	ts.Require().NoError(json.Unmarshal(raw, &list))
	return list
}

// groupOUFilter returns the query string that lists one organization unit's groups.
func groupOUFilter(ouID string) string {
	return "filter=" + url.QueryEscape(fmt.Sprintf("ouId eq %q", ouID))
}

// Every unit lists exactly the groups it owns, each carrying the unit's ID. A parent does not list
// its child's groups, and the declarative unit lists its declared groups next to its runtime one.
func (ts *GroupOUFilterTestSuite) TestEveryUnitListsItsOwnedGroups() {
	units := []struct {
		name string
		id   string
	}{
		{"Parent", ts.parentOUID},
		{"Child", ts.childOUID},
		{"Empty", ts.emptyOUID},
		{"Declarative", declGroupOUID},
	}
	for _, unit := range units {
		ts.Run(unit.name, func() {
			want := ts.owned[unit.id]

			list := ts.list("/groups?limit=100&" + groupOUFilter(unit.id))

			ts.Equal(len(want), list.TotalResults)
			ts.Equal(len(list.Groups), list.Count)
			got := make([]string, len(list.Groups))
			for i, group := range list.Groups {
				got[i] = group.Id
				ts.Equal(unit.id, group.OUID, "group %s", group.Id)
			}
			ts.ElementsMatch(want, got)
		})
	}
}

// Paging through a unit's groups visits each once across both stores, and the links keep the filter.
func (ts *GroupOUFilterTestSuite) TestPagesKeepTheFilter() {
	filter := groupOUFilter(declGroupOUID)

	first := ts.list("/groups?limit=2&offset=0&" + filter)
	ts.Equal(3, first.TotalResults)
	ts.Require().Len(first.Groups, 2)

	var next string
	for _, link := range first.Links {
		if link.Rel == "next" {
			next = link.Href
		}
	}
	ts.Require().NotEmpty(next, "the first page links no next page")
	ts.Contains(next, filter)

	second := ts.list("/groups?limit=2&offset=2&" + filter)
	ts.Equal(3, second.TotalResults)
	ts.Require().Len(second.Groups, 1)

	got := []string{first.Groups[0].Id, first.Groups[1].Id, second.Groups[0].Id}
	slices.Sort(got)
	ts.Equal(ts.owned[declGroupOUID], got)
}

func (ts *GroupOUFilterTestSuite) TestFilterOnAnUnknownUnitListsNothing() {
	list := ts.list("/groups?" + groupOUFilter("00000000-0000-0000-0000-000000000000"))

	ts.Zero(list.TotalResults)
	ts.Empty(list.Groups)
}

func (ts *GroupOUFilterTestSuite) TestAMalformedUnitFilterIsRefused() {
	for name, expression := range map[string]string{
		"an unquoted ID":       "ouId eq " + ts.childOUID,
		"an empty ID":          `ouId eq ""`,
		"no value":             "ouId eq",
		"another attribute":    `name eq "group-ou-filter-child-1"`,
		"a second expression":  fmt.Sprintf(`ouId eq %q and name eq "group-ou-filter-child-1"`, ts.childOUID),
		"another operator":     fmt.Sprintf("ouId sw %q", ts.childOUID),
		"an empty filter text": "",
	} {
		ts.Run(name, func() {
			status, raw := ts.request("/groups?filter=" + url.QueryEscape(expression))

			ts.Require().Equal(http.StatusBadRequest, status, string(raw))
			var e ErrorResponse
			ts.Require().NoError(json.Unmarshal(raw, &e))
			ts.Equal("GRP-1018", e.Code)
		})
	}
}
