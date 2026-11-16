// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package composite

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	pagingParentID = "decl-paging-parent"
	// pagingRuntimeChildren is one more than the composite store's 1000-record limit.
	pagingRuntimeChildren = 1001
	// pagingFileChildren is the number of declarative children of decl-paging-parent.
	pagingFileChildren = 2
)

type pagedOUs struct {
	TotalResults      int       `json:"totalResults"`
	StartIndex        int       `json:"startIndex"`
	Count             int       `json:"count"`
	OrganizationUnits []pagedOU `json:"organizationUnits"`
}

type pagedOU struct {
	ID         string `json:"id"`
	IsReadOnly bool   `json:"isReadOnly"`
}

// TestOrganizationUnitChildrenPagedPastCompositeLimit verifies that a parent with more children than the composite
// store's 1000-record limit lists any page of them, with the runtime children first and the declarative children
// after them, and counts each child once.
func (suite *CompositeModeSuite) TestOrganizationUnitChildrenPagedPastCompositeLimit() {
	runtimeIDs := suite.createPagingChildren(pagingRuntimeChildren)
	defer suite.deletePagingChildren(runtimeIDs)

	total := pagingRuntimeChildren + pagingFileChildren
	declarativeIDs := []string{"decl-paging-file-1", "decl-paging-file-2"}

	// The listing the issue reports: a one-item page of a parent over the limit.
	page := suite.listPagingChildren("limit=1")
	suite.Equal(total, page.TotalResults)
	suite.Equal(1, page.Count)
	suite.False(page.OrganizationUnits[0].IsReadOnly, "the runtime children come first")

	// A page that starts below the limit and reaches past it: the end of the runtime children, then the
	// declarative ones.
	page = suite.listPagingChildren("offset=990&limit=30")
	suite.Equal(total, page.TotalResults)
	runtimeOnPage := pagingRuntimeChildren - 990
	suite.Require().Equal(runtimeOnPage+pagingFileChildren, page.Count)
	for i, ou := range page.OrganizationUnits {
		suite.Equal(i >= runtimeOnPage, ou.IsReadOnly, "item %d of the page", i)
	}
	suite.Equal(declarativeIDs, pagingIDs(page.OrganizationUnits[runtimeOnPage:]))

	// A page that starts on the last runtime child.
	page = suite.listPagingChildren("offset=1000&limit=30")
	suite.Equal(total, page.TotalResults)
	suite.Require().Equal(1+pagingFileChildren, page.Count)
	suite.False(page.OrganizationUnits[0].IsReadOnly)
	suite.Equal(declarativeIDs, pagingIDs(page.OrganizationUnits[1:]))

	// A page past the end.
	page = suite.listPagingChildren(fmt.Sprintf("offset=%d&limit=30", total))
	suite.Equal(total, page.TotalResults)
	suite.Equal(0, page.Count)

	// Paging through the whole list returns every child exactly once.
	seen := make(map[string]bool, total)
	for offset := 0; offset < total; offset += 100 {
		page = suite.listPagingChildren(fmt.Sprintf("offset=%d&limit=100", offset))
		for _, ou := range page.OrganizationUnits {
			suite.False(seen[ou.ID], "child %s listed twice", ou.ID)
			seen[ou.ID] = true
		}
	}
	suite.Len(seen, total)
	for _, id := range append(append([]string{}, runtimeIDs...), declarativeIDs...) {
		suite.True(seen[id], "child %s missing from the pages", id)
	}
}

func (suite *CompositeModeSuite) listPagingChildren(query string) pagedOUs {
	client := testutils.GetHTTPClient()
	resp, err := client.Get(fmt.Sprintf("%s/organization-units/%s/ous?%s",
		testutils.TestServerURL, pagingParentID, query))
	suite.Require().NoError(err)
	defer resp.Body.Close()
	suite.Require().Equal(http.StatusOK, resp.StatusCode, "listing %s should succeed", query)

	var page pagedOUs
	suite.Require().NoError(json.NewDecoder(resp.Body).Decode(&page))
	return page
}

func pagingIDs(ous []pagedOU) []string {
	ids := make([]string, 0, len(ous))
	for _, ou := range ous {
		ids = append(ids, ou.ID)
	}
	return ids
}

// createPagingChildren creates count runtime children of decl-paging-parent and returns their IDs.
func (suite *CompositeModeSuite) createPagingChildren(count int) []string {
	// The server rejects writes that overlap, so the children are created one at a time.
	ids := make([]string, count)
	var failures []string
	for i := range ids {
		id, err := suite.createPagingChild(i)
		if err != nil {
			failures = append(failures, err.Error())
			continue
		}
		ids[i] = id
	}

	if len(failures) > 0 {
		// The caller's cleanup is not registered yet, so remove the children that were created.
		suite.deletePagingChildren(ids)
	}
	suite.Require().Empty(failures, "creating the runtime children failed: %v", failures)
	return ids
}

func (suite *CompositeModeSuite) createPagingChild(i int) (string, error) {
	payload, _ := json.Marshal(map[string]interface{}{
		"handle": fmt.Sprintf("paging-child-%04d", i),
		"name":   fmt.Sprintf("Paging child %04d", i),
		"parent": pagingParentID,
	})
	resp, err := testutils.GetHTTPClient().Post(fmt.Sprintf("%s/organization-units", testutils.TestServerURL),
		"application/json", strings.NewReader(string(payload)))
	if err != nil {
		return "", fmt.Errorf("child %d: %w", i, err)
	}
	defer resp.Body.Close()

	var created struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil || resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("child %d: status %d", i, resp.StatusCode)
	}
	return created.ID, nil
}

func (suite *CompositeModeSuite) deletePagingChildren(ids []string) {
	for _, id := range ids {
		if id != "" {
			suite.deleteResource(fmt.Sprintf("%s/organization-units/%s", testutils.TestServerURL, id))
		}
	}
}
