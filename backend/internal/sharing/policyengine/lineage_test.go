// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package policyengine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// indexOf returns the position of id in the ordered lineages.
func indexOf(t *testing.T, ordered []Lineage, id string) int {
	t.Helper()
	for i, l := range ordered {
		if l.ID == id {
			return i
		}
	}
	require.Failf(t, "missing lineage", "id %s not in result", id)
	return -1
}

// Replaying an export in this order keeps every step valid, because the policy that makes an
// initiator visible is applied before the policy that initiator issued.
func TestOrderByDependencyPutsParentsFirst(t *testing.T) {
	// Deliberately supplied children-first, so a stable sort alone would not produce the answer.
	ordered := OrderByDependency([]Lineage{
		{ID: "grandchild", ParentID: "child"},
		{ID: "child", ParentID: "root"},
		{ID: "root"},
	})

	require.Len(t, ordered, 3)
	assert.Less(t, indexOf(t, ordered, "root"), indexOf(t, ordered, "child"))
	assert.Less(t, indexOf(t, ordered, "child"), indexOf(t, ordered, "grandchild"))
}

// An export of one resource has to replay on its own, so a parent outside the set is a root here
// rather than an unsatisfiable dependency.
func TestOrderByDependencyTreatsAnAbsentParentAsARoot(t *testing.T) {
	ordered := OrderByDependency([]Lineage{
		{ID: "derived", ParentID: "not-in-this-export"},
	})

	require.Len(t, ordered, 1)
	assert.Equal(t, "derived", ordered[0].ID)
}

// A cycle cannot arise from a well-formed graph, but the remainder is still emitted rather than
// silently dropped.
func TestOrderByDependencyKeepsEveryPolicyOnACycle(t *testing.T) {
	ordered := OrderByDependency([]Lineage{
		{ID: "a", ParentID: "b"},
		{ID: "b", ParentID: "a"},
	})

	assert.Len(t, ordered, 2, "a malformed graph must not silently lose policies")
}

// Two entries sharing an id must both be emitted. Tracking placement by id instead of by position
// leaves the second one permanently unplaceable, and the loop never finishes; the test binary's own
// timeout is what surfaces that.
func TestOrderByDependencyKeepsBothEntriesWhenIDsCollide(t *testing.T) {
	ordered := OrderByDependency([]Lineage{{ID: "a"}, {ID: "a"}})

	assert.Len(t, ordered, 2, "a duplicate id must not be dropped")
}

// A duplicate id must not disturb the ordering of the entries around it.
func TestOrderByDependencyStillOrdersAroundACollidingID(t *testing.T) {
	ordered := OrderByDependency([]Lineage{
		{ID: "child", ParentID: "dup"},
		{ID: "dup"},
		{ID: "dup"},
	})

	require.Len(t, ordered, 3)
	assert.Less(t, indexOf(t, ordered, "dup"), indexOf(t, ordered, "child"),
		"the parent still precedes what derives from it")
}
