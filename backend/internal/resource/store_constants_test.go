// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"regexp"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// placeholders returns every $n in a statement, in order of appearance.
func placeholders(query string) []int {
	out := []int{}
	for _, m := range regexp.MustCompile(`\$(\d+)`).FindAllStringSubmatch(query, -1) {
		n, _ := strconv.Atoi(m[1])
		out = append(out, n)
	}
	return out
}

// Every placeholder the statement uses must have an argument, and no argument may be left over: a
// mismatch binds the deployment or the page bounds to the wrong value and the query silently
// matches nothing.
func TestResourceServerListForOUsBindsEveryPlaceholder(t *testing.T) {
	tests := []struct {
		name      string
		ouIDs     []string
		sharedIDs []string
		paginated bool
	}{
		{"owned only", []string{"ou-1"}, nil, true},
		{"shared only", nil, []string{"rs-1"}, true},
		{"owned and shared", []string{"ou-1", "ou-2"}, []string{"rs-1"}, true},
		{"neither", nil, nil, true},
		{"count variant", []string{"ou-1"}, []string{"rs-1"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, args := buildResourceServerListForOUsQuery(
				tt.ouIDs, tt.sharedIDs, "dep-1", 30, 60, tt.paginated)

			used := placeholders(query.Query)
			require.NotEmpty(t, used, "the statement binds nothing")

			highest := 0
			for _, n := range used {
				if n > highest {
					highest = n
				}
			}
			assert.Equal(t, highest, len(args),
				"argument count must match the highest placeholder, or every later one is misbound")

			for _, n := range used {
				assert.NotNil(t, args[n-1], "$%d has no value", n)
			}
		})
	}
}

// The deployment and the page bounds must land on their own placeholders, not on ones the reach
// predicate already claimed.
func TestResourceServerListForOUsBindsTheDeploymentAndPage(t *testing.T) {
	query, args := buildResourceServerListForOUsQuery(
		[]string{"ou-1"}, []string{"rs-1"}, "dep-1", 30, 60, true)

	assert.Equal(t, []interface{}{"ou-1", "rs-1", "dep-1", 30, 60}, args)
	assert.Contains(t, query.Query, "OU_ID IN ($1)")
	assert.Contains(t, query.Query, "ID IN ($2)")
	assert.Contains(t, query.Query, "DEPLOYMENT_ID = $3")
	assert.Contains(t, query.Query, "LIMIT $4 OFFSET $5")
}

// A count has no page bounds, so nothing may be reserved for them.
func TestResourceServerListCountForOUsHasNoPageBounds(t *testing.T) {
	query, args := buildResourceServerListForOUsQuery(
		[]string{"ou-1"}, nil, "dep-1", 0, 0, false)

	assert.Equal(t, []interface{}{"ou-1", "dep-1"}, args)
	assert.NotContains(t, query.Query, "LIMIT")
	assert.Contains(t, query.Query, "COUNT(*)")
}

// Reaching nothing must still be a valid statement rather than an unbounded one.
func TestResourceServerListForOUsReachingNothingMatchesNothing(t *testing.T) {
	query, args := buildResourceServerListForOUsQuery(nil, nil, "dep-1", 30, 0, true)

	assert.Contains(t, query.Query, "1 = 0", "an empty reach must not return the whole table")
	assert.Equal(t, []interface{}{"dep-1", 30, 0}, args)
}
