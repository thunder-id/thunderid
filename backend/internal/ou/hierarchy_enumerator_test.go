// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ou

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type HierarchyEnumeratorTestSuite struct {
	suite.Suite
}

// TestHierarchyEnumeratorTestSuite runs the organization unit hierarchy enumerator suite.
func TestHierarchyEnumeratorTestSuite(t *testing.T) {
	suite.Run(t, new(HierarchyEnumeratorTestSuite))
}

// basics returns a one-page listing of organization units with the given ids.
func basics(ids ...string) []providers.OrganizationUnitBasic {
	out := make([]providers.OrganizationUnitBasic, 0, len(ids))
	for _, id := range ids {
		out = append(out, providers.OrganizationUnitBasic{ID: id})
	}
	return out
}

// expectChildren stubs one full page-read for a parent: the ids, then the empty page that ends it.
func expectChildren(m *organizationUnitStoreInterfaceMock, parent string, ids ...string) {
	m.On("GetOrganizationUnitChildrenList", mock.Anything, parent, hierarchyPageSize, 0, mock.Anything).
		Return(basics(ids...), nil).Once()
}

// The walk is recursive, not one level deep: a grandchild has to come back alongside the children.
func (suite *HierarchyEnumeratorTestSuite) TestDescendantOUIDsWalksEveryLevel() {
	mockStore := newOrganizationUnitStoreInterfaceMock(suite.T())
	expectChildren(mockStore, "root", "child-a", "child-b")
	expectChildren(mockStore, "child-a", "grandchild")
	expectChildren(mockStore, "child-b")
	expectChildren(mockStore, "grandchild")

	got, svcErr := newOUHierarchyEnumerator(mockStore).DescendantOUIDs(context.Background(), "root")

	require.Nil(suite.T(), svcErr)
	assert.ElementsMatch(suite.T(), []string{"child-a", "child-b", "grandchild"}, got,
		"the walk must reach past the first level")
}

// The anchor keeps its own visibility, so only what lies beneath it is enumerated.
func (suite *HierarchyEnumeratorTestSuite) TestDescendantOUIDsExcludesTheAnchor() {
	mockStore := newOrganizationUnitStoreInterfaceMock(suite.T())
	expectChildren(mockStore, "leaf")

	got, svcErr := newOUHierarchyEnumerator(mockStore).DescendantOUIDs(context.Background(), "leaf")

	require.Nil(suite.T(), svcErr)
	assert.Empty(suite.T(), got)
}

// An empty anchor names no organization unit, so it is answered without touching the store.
func (suite *HierarchyEnumeratorTestSuite) TestDescendantOUIDsIgnoresAnEmptyID() {
	// No store call is stubbed, so reaching the store at all fails the mock's expectations.
	got, svcErr := newOUHierarchyEnumerator(
		newOrganizationUnitStoreInterfaceMock(suite.T()),
	).DescendantOUIDs(context.Background(), "")

	require.Nil(suite.T(), svcErr)
	assert.Empty(suite.T(), got)
}

// A page that comes back full means there may be more, so the walk has to ask again rather than
// stopping at the first hundred children.
func (suite *HierarchyEnumeratorTestSuite) TestDescendantOUIDsReadsEveryPage() {
	first := make([]string, hierarchyPageSize)
	for i := range first {
		first[i] = string(rune('a'+i%26)) + "-" + string(rune('0'+i/26))
	}
	mockStore := newOrganizationUnitStoreInterfaceMock(suite.T())
	mockStore.On("GetOrganizationUnitChildrenList", mock.Anything, "root",
		hierarchyPageSize, 0, mock.Anything).
		Return(basics(first...), nil).Once()
	mockStore.On("GetOrganizationUnitChildrenList", mock.Anything, "root",
		hierarchyPageSize, hierarchyPageSize, mock.Anything).
		Return(basics("last"), nil).Once()
	for _, id := range append(first, "last") {
		expectChildren(mockStore, id)
	}

	got, svcErr := newOUHierarchyEnumerator(mockStore).DescendantOUIDs(context.Background(), "root")

	require.Nil(suite.T(), svcErr)
	assert.Len(suite.T(), got, hierarchyPageSize+1)
	assert.Contains(suite.T(), got, "last", "the second page was never read")
}

// A parent chain that loops would otherwise walk forever.
func (suite *HierarchyEnumeratorTestSuite) TestDescendantOUIDsRefusesACycle() {
	mockStore := newOrganizationUnitStoreInterfaceMock(suite.T())
	expectChildren(mockStore, "root", "child")
	expectChildren(mockStore, "child", "root")

	_, svcErr := newOUHierarchyEnumerator(mockStore).DescendantOUIDs(context.Background(), "root")

	require.NotNil(suite.T(), svcErr)
}

// A failed page must surface as an error rather than a short, silently incomplete walk, which
// callers would read as "nothing lies beneath here".
func (suite *HierarchyEnumeratorTestSuite) TestDescendantOUIDsSurfacesAStoreFailure() {
	mockStore := newOrganizationUnitStoreInterfaceMock(suite.T())
	mockStore.On("GetOrganizationUnitChildrenList", mock.Anything, "root", hierarchyPageSize, 0, mock.Anything).
		Return(nil, errors.New("database down")).Once()

	_, svcErr := newOUHierarchyEnumerator(mockStore).DescendantOUIDs(context.Background(), "root")

	require.NotNil(suite.T(), svcErr)
}

// AllOUIDs spans every root and everything beneath each of them, not just the first tree.
func (suite *HierarchyEnumeratorTestSuite) TestAllOUIDsCoversEveryTree() {
	mockStore := newOrganizationUnitStoreInterfaceMock(suite.T())
	mockStore.On("GetOrganizationUnitList", mock.Anything, hierarchyPageSize, 0, mock.Anything).
		Return(basics("root-a", "root-b"), nil).Once()
	expectChildren(mockStore, "root-a", "child")
	expectChildren(mockStore, "child")
	expectChildren(mockStore, "root-b")

	got, svcErr := newOUHierarchyEnumerator(mockStore).AllOUIDs(context.Background())

	require.Nil(suite.T(), svcErr)
	assert.ElementsMatch(suite.T(), []string{"root-a", "root-b", "child"}, got,
		"every root and everything beneath it belongs to the deployment")
}
