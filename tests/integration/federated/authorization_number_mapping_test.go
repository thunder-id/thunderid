// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package federated

import (
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// A number rule compares the claim by value rather than as text: a level of 10 is greater than 9, though
// "10" sorts before "9", and a level of 8 is not.
func (s *FederatedMappingSuite) TestAuthzMapping_NumberGreaterThanComparesByValue() {
	config := authzMappingWithRules("level", testutils.AuthorizationValueTypeNumber, []testutils.AuthorizationRule{
		{
			Operator: testutils.AuthorizationOperatorGreaterThan,
			Value:    "9",
			Targets:  []testutils.AuthorizationTarget{roleTarget(s.authzMappedRoleID)},
		},
	})

	senior := s.baseUser(s.nextSubject())
	senior.Custom["level"] = 10
	authorized := s.authorizeFederated(config, senior, "read", "federated-authz-mapping-api")
	s.Contains(authorized, "read", "a level of 10 is greater than 9")

	junior := s.baseUser(s.nextSubject())
	junior.Custom["level"] = 8
	authorized = s.authorizeFederated(config, junior, "read", "federated-authz-mapping-api")
	s.NotContains(authorized, "read", "a level of 8 is not greater than 9")

	unreadable := s.baseUser(s.nextSubject())
	unreadable.Custom["level"] = "senior"
	authorized = s.authorizeFederated(config, unreadable, "read", "federated-authz-mapping-api")
	s.NotContains(authorized, "read", "a claim that is not a number never matches a number rule")
}

// A number rule compares integers beyond the precision of a double exactly: 9007199254740993 equals itself,
// also when written with a fraction, and does not equal 9007199254740992, which a double would round it to.
func (s *FederatedMappingSuite) TestAuthzMapping_NumberEqualsComparesLargeIntegersExactly() {
	config := authzMappingWithRules("account", testutils.AuthorizationValueTypeNumber, []testutils.AuthorizationRule{
		{
			Operator: testutils.AuthorizationOperatorEquals,
			Value:    " 9007199254740993 ",
			Targets:  []testutils.AuthorizationTarget{roleTarget(s.authzMappedRoleID)},
		},
	})

	for _, account := range []string{"9007199254740993", "9007199254740993.0"} {
		user := s.baseUser(s.nextSubject())
		user.Custom["account"] = account
		authorized := s.authorizeFederated(config, user, "read", "federated-authz-mapping-api")
		s.Contains(authorized, "read", "%s equals 9007199254740993", account)
	}

	neighbour := s.baseUser(s.nextSubject())
	neighbour.Custom["account"] = "9007199254740992"
	authorized := s.authorizeFederated(config, neighbour, "read", "federated-authz-mapping-api")
	s.NotContains(authorized, "read", "9007199254740992 does not equal 9007199254740993")
}

// Each ordering operator compares a number rule by value, and not_equals holds only for a different value.
func (s *FederatedMappingSuite) TestAuthzMapping_NumberOperators() {
	cases := []struct {
		operator string
		value    string
		claim    any
		matches  bool
	}{
		{testutils.AuthorizationOperatorLessThan, "10", 9, true},
		{testutils.AuthorizationOperatorLessThan, "10", 10, false},
		{testutils.AuthorizationOperatorGreaterThanOrEqual, "10", 10, true},
		{testutils.AuthorizationOperatorLessThanOrEqual, "10", 11, false},
		{testutils.AuthorizationOperatorNotEquals, "10", "10.0", false},
		{testutils.AuthorizationOperatorNotEquals, "10", 12, true},
		{testutils.AuthorizationOperatorEquals, "1e1", 10, true},
		// Beyond the exact bounds a value is compared as a double, as before: 1e-500 rounds to 0.
		{testutils.AuthorizationOperatorEquals, "1e-500", 0, true},
	}
	for _, tc := range cases {
		config := authzMappingWithRules("level", testutils.AuthorizationValueTypeNumber,
			[]testutils.AuthorizationRule{{
				Operator: tc.operator,
				Value:    tc.value,
				Targets:  []testutils.AuthorizationTarget{roleTarget(s.authzMappedRoleID)},
			}})
		user := s.baseUser(s.nextSubject())
		user.Custom["level"] = tc.claim
		authorized := s.authorizeFederated(config, user, "read", "federated-authz-mapping-api")
		if tc.matches {
			s.Contains(authorized, "read", "%v %s %s", tc.claim, tc.operator, tc.value)
		} else {
			s.NotContains(authorized, "read", "%v %s %s", tc.claim, tc.operator, tc.value)
		}
	}
}

// A boolean rule compares the claim as a boolean, and ordering never applies to it.
func (s *FederatedMappingSuite) TestAuthzMapping_BooleanOperators() {
	cases := []struct {
		operator string
		value    string
		claim    any
		matches  bool
	}{
		{testutils.AuthorizationOperatorEquals, "true", true, true},
		{testutils.AuthorizationOperatorEquals, "true", "TRUE", true},
		{testutils.AuthorizationOperatorNotEquals, "true", false, true},
		{testutils.AuthorizationOperatorEquals, "true", "yes", false},
	}
	for _, tc := range cases {
		config := authzMappingWithRules("verified", testutils.AuthorizationValueTypeBoolean,
			[]testutils.AuthorizationRule{{
				Operator: tc.operator,
				Value:    tc.value,
				Targets:  []testutils.AuthorizationTarget{roleTarget(s.authzMappedRoleID)},
			}})
		user := s.baseUser(s.nextSubject())
		user.Custom["verified"] = tc.claim
		authorized := s.authorizeFederated(config, user, "read", "federated-authz-mapping-api")
		if tc.matches {
			s.Contains(authorized, "read", "%v %s %s", tc.claim, tc.operator, tc.value)
		} else {
			s.NotContains(authorized, "read", "%v %s %s", tc.claim, tc.operator, tc.value)
		}
	}
}
