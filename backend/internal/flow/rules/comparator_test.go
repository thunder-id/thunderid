// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type ComparatorTestSuite struct {
	suite.Suite
}

func TestComparatorTestSuite(t *testing.T) {
	suite.Run(t, new(ComparatorTestSuite))
}

// inferred returns a comparison with no declared comparator for an operator with the given needs.
func inferred(need ability) *comparison {
	return &comparison{pinned: cmpNone, allowed: inferableWith(need), need: need}
}

func (s *ComparatorTestSuite) TestInferenceTakesMostSpecificComparator() {
	cases := []struct {
		name string
		a, b string
		need ability
		want comparatorID
	}{
		{"numbers for equality", "30", "30.0", abilityEquality, cmpNumber},
		{"numbers for order", "30", "18", abilityOrdering, cmpNumber},
		{"dates", "2026-01-05", "2025-12-01", abilityOrdering, cmpInstant},
		{"durations", "PT15M", "PT1H", abilityOrdering, cmpDuration},
		{"booleans", "true", "false", abilityEquality, cmpBoolean},
		{"words", "payments-approver", "treasury", abilityEquality, cmpText},
		{"number and word", "30", "thirty", abilityEquality, cmpText},
		{"words cannot be ordered", "a", "b", abilityOrdering, cmpNone},
		{"booleans cannot be ordered", "true", "false", abilityOrdering, cmpNone},
		{"number and word cannot be ordered", "30", "thirty", abilityOrdering, cmpNone},
		{"text match on numbers reads text", "30", "3", abilityText, cmpText},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			a, b := newAtom(tc.a), newAtom(tc.b)

			s.Equal(tc.want, inferred(tc.need).pick(&a, &b))
		})
	}
}

func (s *ComparatorTestSuite) TestDeclaredComparator() {
	text := &comparison{pinned: cmpText, allowed: inferableWith(abilityEquality), need: abilityEquality}
	a, b := newAtom("0123"), newAtom("123")
	s.Equal(cmpText, text.pick(&a, &b))
	s.False(text.equal(cmpText, &a, &b))

	number := &comparison{pinned: cmpNumber, allowed: inferableWith(abilityOrdering), need: abilityOrdering}
	word := newAtom("thirty")
	s.Equal(cmpNone, number.pick(&a, &word))
	s.False(number.recognizes(&word))
	s.True(number.recognizes(&a))

	ordering := &comparison{pinned: cmpText, allowed: inferableWith(abilityOrdering), need: abilityOrdering}
	s.Equal(cmpNone, ordering.pick(&a, &b))

	instant := &comparison{pinned: cmpInstant, need: abilityEquality}
	unix := newAtom("1730000000")
	s.False(instant.recognizes(&unix))
	s.True((&comparison{pinned: cmpNone}).recognizes(&unix))
}

func (s *ComparatorTestSuite) TestScale() {
	scale := &comparison{pinned: cmpScale, scale: scaleRanks([]string{"aal1", "aal2", "aal3"}),
		need: abilityOrdering}
	aal1, aal2, other := newAtom("aal1"), newAtom("aal2"), newAtom("aal9")

	s.Equal(cmpScale, scale.pick(&aal2, &aal1))
	s.Equal(1, scale.order(cmpScale, &aal2, &aal1))
	s.True(scale.equal(cmpScale, &aal2, &aal2))
	s.Equal(cmpNone, scale.pick(&aal2, &other))
	s.False(scale.recognizes(&other))
	s.Nil(scaleRanks(nil))
}

func (s *ComparatorTestSuite) TestEqualAndOrder() {
	c := inferred(abilityEquality)
	three, threePointZero := newAtom("3"), newAtom("3.0")
	s.True(c.equal(cmpNumber, &three, &threePointZero))
	s.False(c.equal(cmpText, &three, &threePointZero))

	upper, lower := newAtom("Finance"), newAtom("finance")
	s.False(c.equal(cmpText, &upper, &lower))
	c.ci = true
	s.True(c.equal(cmpText, &upper, &lower))
	s.Equal("finance", c.textOf(&upper))

	t1, t2 := newAtom("2026-01-05"), newAtom("2026-01-05T00:00:00Z")
	s.True(c.equal(cmpInstant, &t1, &t2))
	d1, d2 := newAtom("PT60M"), newAtom("PT1H")
	s.True(c.equal(cmpDuration, &d1, &d2))
	yes := newAtom("true")
	s.True(c.equal(cmpBoolean, &yes, &yes))
	s.Equal(-1, c.order(cmpDuration, &newAtoms([]string{"P1D"})[0], &newAtoms([]string{"P2D"})[0]))
	s.Equal(1, c.order(cmpDuration, &newAtoms([]string{"P100Y"})[0], &newAtoms([]string{"P1D"})[0]))
	s.Equal(0, c.order(cmpText, &upper, &lower))
}

func (s *ComparatorTestSuite) TestRegistryHelpers() {
	s.Equal(cmpNumber, comparatorByName(ComparatorNumber))
	s.Equal(cmpScale, comparatorByName(ComparatorScale))
	s.Equal(cmpNone, comparatorByName(""))
	s.Equal(cmpNone, comparatorByName("currency"))
	s.Equal(ComparatorText, cmpText.displayName())
	s.Equal(ComparatorName(""), cmpNone.displayName())
	s.False(cmpNone.offers(abilityEquality))
	s.Equal(maskOf(cmpText), inferableWith(abilityText))
	s.Equal(maskOf(cmpInstant)|maskOf(cmpDuration)|maskOf(cmpNumber), inferableWith(abilityOrdering))
}

func (s *ComparatorTestSuite) TestMatchText() {
	s.True(matchText(opStartsWith, "alice@example.com", "alice"))
	s.True(matchText(opEndsWith, "alice@example.com", "@example.com"))
	s.True(matchText(opContains, "alice@example.com", "@"))
	s.False(matchText(opEndsWith, "alice@example.com.evil.test", "@example.com"))
}
