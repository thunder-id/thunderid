// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sharing

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

// StoreConstantsTestSuite covers the chain-scoped query builder, whose WHERE clause has to stay
// parameterized however many organization units the chain holds.
type StoreConstantsTestSuite struct {
	suite.Suite
}

func TestStoreConstantsTestSuite(t *testing.T) {
	suite.Run(t, new(StoreConstantsTestSuite))
}

// An empty chain names no organization unit. Emitting the membership test anyway produces IN (),
// which PostgreSQL rejects outright, so the whole lookup fails rather than returning the blanket
// policies that still apply.
func (s *StoreConstantsTestSuite) TestBuildRelevantPoliciesQueryHandlesAnEmptyChain() {
	query, args := buildRelevantPoliciesQuery(testType, "", nil, "dep-1")

	for name, sql := range map[string]string{
		"postgres": query.PostgresQuery,
		"sqlite":   query.SQLiteQuery,
	} {
		s.Run(name, func() {
			s.NotContains(sql, "IN ()", "an empty list is not valid SQL")
			s.Contains(sql, "t.TARGET_SCOPE IN ('allOus', 'allRoots')",
				"blanket policies still have to match, by the values their targets are stored with")
		})
	}
	s.Equal([]interface{}{string(testType), "dep-1"}, args)
}

// Every chain member needs its own placeholder and argument, in matching order.
func (s *StoreConstantsTestSuite) TestBuildRelevantPoliciesQueryBindsEveryChainMember() {
	query, args := buildRelevantPoliciesQuery(testType, "", []string{"ou-1", "ou-2"}, "dep-1")

	s.Contains(query.PostgresQuery, "TARGET_OU_ID IN ($2,$3)")
	s.Contains(query.SQLiteQuery, "TARGET_OU_ID IN (?,?)")
	s.Require().Len(args, 4)
	s.Equal([]interface{}{string(testType), "ou-1", "ou-2", "dep-1"}, args)
}

// The foreign key on RESOURCE_SHARING_POLICY_TARGET.POLICY_ID checks only the policy id, so a
// target row carrying another deployment's id still satisfies it. Without this join condition such
// a row's target scope would decide visibility across the deployment boundary.
func (s *StoreConstantsTestSuite) TestBuildRelevantPoliciesQueryScopesTheTargetJoinByDeployment() {
	query, _ := buildRelevantPoliciesQuery(testType, "", []string{"ou-1"}, "dep-1")

	for name, sql := range map[string]string{
		"postgres": query.PostgresQuery,
		"sqlite":   query.SQLiteQuery,
	} {
		s.Run(name, func() {
			s.Contains(sql, "t.DEPLOYMENT_ID = p.DEPLOYMENT_ID")
		})
	}
}

// Narrowing to one resource inserts an argument ahead of the chain, so the chain's own placeholders
// shift with it. Binding them by a fixed offset is what would put every chain member in the wrong
// slot, which the database would accept as a perfectly valid query against the wrong ids.
func (s *StoreConstantsTestSuite) TestBuildRelevantPoliciesQueryNarrowsToOneResource() {
	query, args := buildRelevantPoliciesQuery(testType, "res-1", []string{"ou-1", "ou-2"}, "dep-1")

	s.Contains(query.PostgresQuery, "p.RESOURCE_ID = $2")
	s.Contains(query.PostgresQuery, "TARGET_OU_ID IN ($3,$4)")
	s.Contains(query.SQLiteQuery, "p.RESOURCE_ID = ?")
	s.Equal([]interface{}{string(testType), "res-1", "ou-1", "ou-2", "dep-1"}, args)
}

// The reverse lookup asks across every resource of the type, so naming none must not emit a
// predicate that matches nothing.
func (s *StoreConstantsTestSuite) TestBuildRelevantPoliciesQueryWithoutAResourceSpansThemAll() {
	query, args := buildRelevantPoliciesQuery(testType, "", []string{"ou-1"}, "dep-1")

	s.NotContains(query.PostgresQuery, "p.RESOURCE_ID = ",
		"the column is selected, but must not be filtered on")
	s.Contains(query.PostgresQuery, "TARGET_OU_ID IN ($2)")
	s.Equal([]interface{}{string(testType), "ou-1", "dep-1"}, args)
}

// The placeholder styles must not leak into one another's dialect.
func (s *StoreConstantsTestSuite) TestBuildRelevantPoliciesQueryKeepsDialectsApart() {
	query, _ := buildRelevantPoliciesQuery(testType, "", []string{"ou-1"}, "dep-1")

	s.NotContains(query.SQLiteQuery, "$1")
	s.False(strings.Contains(query.PostgresQuery, "IN (?)"))
}

// The two dialects share one argument slice, and SQLite binds a bare ? by where it sits in the text.
// So the numbered placeholders have to appear in ascending order: if $3 were written before $2, the
// SQLite form would bind the same arguments to different columns and the database would accept it.
func (s *StoreConstantsTestSuite) TestBuildRelevantPoliciesQueryNumbersPlaceholdersInTextOrder() {
	cases := []struct {
		name       string
		resourceID string
		chain      []string
	}{
		{"no resource, no chain", "", nil},
		{"no resource, one chain member", "", []string{"ou-1"}},
		{"a resource and a chain", "res-1", []string{"ou-1", "ou-2", "ou-3"}},
		{"a resource, no chain", "res-1", nil},
	}

	for _, tt := range cases {
		s.Run(tt.name, func() {
			query, args := buildRelevantPoliciesQuery(testType, tt.resourceID, tt.chain, "dep-1")

			numbers := regexp.MustCompile(`\$(\d+)`).FindAllStringSubmatch(query.PostgresQuery, -1)
			s.Require().Len(numbers, len(args), "one placeholder per argument, and no more")
			for i, m := range numbers {
				n, err := strconv.Atoi(m[1])
				s.Require().NoError(err)
				s.Equal(i+1, n, "placeholders must read 1..n in the order they appear")
			}

			s.Equal(len(args), strings.Count(query.SQLiteQuery, "?"),
				"the sqlite form needs the same count, bound by position")
			s.Equal("dep-1", args[len(args)-1], "the deployment is the last argument")
			s.Regexp(`p\.DEPLOYMENT_ID = \$`+strconv.Itoa(len(args))+`$`, query.PostgresQuery,
				"and the last placeholder in the text")
		})
	}
}
