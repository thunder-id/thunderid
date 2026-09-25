// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sharing

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An empty chain names no organization unit. Emitting the membership test anyway produces IN (),
// which PostgreSQL rejects outright, so the whole lookup fails rather than returning the blanket
// policies that still apply.
func TestBuildRelevantPoliciesQueryHandlesAnEmptyChain(t *testing.T) {
	query, args := buildRelevantPoliciesQuery(testType, "", nil, "dep-1")

	for name, sql := range map[string]string{
		"postgres": query.PostgresQuery,
		"sqlite":   query.SQLiteQuery,
	} {
		t.Run(name, func(t *testing.T) {
			assert.NotContains(t, sql, "IN ()", "an empty list is not valid SQL")
			assert.Contains(t, sql, "all_ous", "blanket policies still have to match")
		})
	}
	assert.Equal(t, []interface{}{"dep-1", string(testType)}, args)
}

// Every chain member needs its own placeholder and argument, in matching order.
func TestBuildRelevantPoliciesQueryBindsEveryChainMember(t *testing.T) {
	query, args := buildRelevantPoliciesQuery(testType, "", []string{"ou-1", "ou-2"}, "dep-1")

	assert.Contains(t, query.PostgresQuery, "TARGET_OU_ID IN ($3,$4)")
	assert.Contains(t, query.SQLiteQuery, "TARGET_OU_ID IN (?,?)")
	require.Len(t, args, 4)
	assert.Equal(t, []interface{}{"dep-1", string(testType), "ou-1", "ou-2"}, args)
}

// The foreign key on RESOURCE_SHARING_POLICY_TARGET.POLICY_ID checks only the policy id, so a
// target row carrying another deployment's id still satisfies it. Without this join condition such
// a row's target scope would decide visibility across the deployment boundary.
func TestBuildRelevantPoliciesQueryScopesTheTargetJoinByDeployment(t *testing.T) {
	query, _ := buildRelevantPoliciesQuery(testType, "", []string{"ou-1"}, "dep-1")

	for name, sql := range map[string]string{
		"postgres": query.PostgresQuery,
		"sqlite":   query.SQLiteQuery,
	} {
		t.Run(name, func(t *testing.T) {
			assert.Contains(t, sql, "t.DEPLOYMENT_ID = p.DEPLOYMENT_ID")
		})
	}
}

// Narrowing to one resource inserts an argument ahead of the chain, so the chain's own placeholders
// shift with it. Binding them by a fixed offset is what would put every chain member in the wrong
// slot, which the database would accept as a perfectly valid query against the wrong ids.
func TestBuildRelevantPoliciesQueryNarrowsToOneResource(t *testing.T) {
	query, args := buildRelevantPoliciesQuery(testType, "res-1", []string{"ou-1", "ou-2"}, "dep-1")

	assert.Contains(t, query.PostgresQuery, "p.RESOURCE_ID = $3")
	assert.Contains(t, query.PostgresQuery, "TARGET_OU_ID IN ($4,$5)")
	assert.Contains(t, query.SQLiteQuery, "p.RESOURCE_ID = ?")
	assert.Equal(t, []interface{}{"dep-1", string(testType), "res-1", "ou-1", "ou-2"}, args)
}

// The reverse lookup asks across every resource of the type, so naming none must not emit a
// predicate that matches nothing.
func TestBuildRelevantPoliciesQueryWithoutAResourceSpansThemAll(t *testing.T) {
	query, args := buildRelevantPoliciesQuery(testType, "", []string{"ou-1"}, "dep-1")

	assert.NotContains(t, query.PostgresQuery, "p.RESOURCE_ID = ",
		"the column is selected, but must not be filtered on")
	assert.Contains(t, query.PostgresQuery, "TARGET_OU_ID IN ($3)")
	assert.Equal(t, []interface{}{"dep-1", string(testType), "ou-1"}, args)
}

// The placeholder styles must not leak into one another's dialect.
func TestBuildRelevantPoliciesQueryKeepsDialectsApart(t *testing.T) {
	query, _ := buildRelevantPoliciesQuery(testType, "", []string{"ou-1"}, "dep-1")

	assert.NotContains(t, query.SQLiteQuery, "$1")
	assert.False(t, strings.Contains(query.PostgresQuery, "IN (?)"))
}
