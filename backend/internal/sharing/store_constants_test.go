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
	query, args := buildRelevantPoliciesQuery(testType, nil, "dep-1")

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

func TestBuildRelevantPoliciesQueryBindsEveryChainMember(t *testing.T) {
	query, args := buildRelevantPoliciesQuery(testType, []string{"ou-1", "ou-2"}, "dep-1")

	assert.Contains(t, query.PostgresQuery, "TARGET_OU_ID IN ($3,$4)")
	assert.Contains(t, query.SQLiteQuery, "TARGET_OU_ID IN (?,?)")
	require.Len(t, args, 4)
	assert.Equal(t, []interface{}{"dep-1", string(testType), "ou-1", "ou-2"}, args)
}

// The placeholder styles must not leak into one another's dialect.
func TestBuildRelevantPoliciesQueryKeepsDialectsApart(t *testing.T) {
	query, _ := buildRelevantPoliciesQuery(testType, []string{"ou-1"}, "dep-1")

	assert.NotContains(t, query.SQLiteQuery, "$1")
	assert.False(t, strings.Contains(query.PostgresQuery, "IN (?)"))
}
