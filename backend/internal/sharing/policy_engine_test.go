// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sharing

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

// ContainmentTestSuite covers member comparison for the three field kinds, including the prefix
// containment a hierarchy field needs its own delimiter for.
type ContainmentTestSuite struct {
	suite.Suite
}

func TestContainmentTestSuite(t *testing.T) {
	suite.Run(t, new(ContainmentTestSuite))
}

// Containment means equality for a reference set and path descent for a hierarchy, so the same
// pair of members is covered under one kind and not the other.
func (s *ContainmentTestSuite) TestCoversByKind() {
	tests := []struct {
		name  string
		kind  FieldKind
		outer string
		inner string
		want  bool
	}{
		{"scalar equal", FieldScalar, "totp", "totp", true},
		{"scalar different", FieldScalar, "totp", "sms-otp", false},
		{"reference equal", FieldReferenceSet, "group-1", "group-1", true},
		{"reference different", FieldReferenceSet, "group-1", "group-2", false},
		// A reference id that happens to share a prefix must not be treated as hierarchical.
		{"reference prefix is not containment", FieldReferenceSet, "group", "group:1", false},
		{"hierarchy self", FieldHierarchy, "bookings", "bookings", true},
		{"hierarchy descendant", FieldHierarchy, "bookings", "bookings:create", true},
		{"hierarchy deep descendant", FieldHierarchy, "billing", "billing:invoice:view", true},
		{"hierarchy ancestor does not cover upward", FieldHierarchy, "bookings:create", "bookings", false},
		{"hierarchy sibling", FieldHierarchy, "bookings", "billing", false},
		// "bookings" must not cover "bookingsx": containment is by segment, not by string prefix.
		{"hierarchy partial segment is not containment", FieldHierarchy, "bookings", "bookingsx", false},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			c := newContainment(tt.kind, ":")
			s.Equal(tt.want, c.covers(tt.outer, tt.inner))
		})
	}
}

// A containment built with no separator gets the default one. This is the algebra's own floor, not
// something a resource type may rely on: one that declares a hierarchical field and resolves no
// separator is refused by the service before a containment is built.
//
// The floor still matters, because an empty separator turns prefix containment into raw string
// prefixing, under which "billing" covers "billingx". The constructor is the only place it can be
// enforced, since a containment's fields are private and cannot be assembled by a literal.
func (s *ContainmentTestSuite) TestAContainmentBuiltWithNoDelimiterGetsTheDefault() {
	c := newContainment(FieldHierarchy, "")

	s.Equal(defaultDelimiter, c.delimiter)
	s.True(c.covers("billing", "billing"+defaultDelimiter+"invoice"), "a path beneath it is covered")
	s.False(c.covers("billing", "billingx"), "a sibling sharing a character prefix is not")
}

// A set is covered only when every one of its members is covered by some member of the outer set.
func (s *ContainmentTestSuite) TestSetCovers() {
	c := newContainment(FieldHierarchy, ":")

	s.True(c.setCovers([]string{"bookings", "billing"}, []string{"bookings:create", "billing"}))
	s.False(c.setCovers([]string{"bookings"}, []string{"bookings:create", "billing"}))
	// An empty inner set is covered by anything, including an empty outer set.
	s.True(c.setCovers(nil, nil))
	s.False(c.setCovers(nil, []string{"bookings"}))
}

// Intersecting a path with one beneath it yields the deeper path, which is the narrower grant.
func (s *ContainmentTestSuite) TestIntersectMembersKeepsTheNarrowerPath() {
	c := newContainment(FieldHierarchy, ":")

	got := c.intersectMembers([]string{"billing"}, []string{"billing:invoice"})

	s.Equal([]string{"billing:invoice"}, got)
}

// Only members present on both sides survive an intersection.
func (s *ContainmentTestSuite) TestIntersectMembersDropsDisjointEntries() {
	c := newContainment(FieldReferenceSet, "")

	got := c.intersectMembers([]string{"a", "b"}, []string{"b", "c"})

	s.Equal([]string{"b"}, got)
}

// A path already covered by an ancestor in the same union adds nothing, so it is folded away.
func (s *ContainmentTestSuite) TestUnionDropsRedundantDescendants() {
	c := newContainment(FieldHierarchy, ":")

	got := c.union([]string{"billing", "billing:invoice"}, []string{"bookings"})

	s.ElementsMatch([]string{"billing", "bookings"}, got)
}

// A union is a set: the same member twice collapses to one entry.
func (s *ContainmentTestSuite) TestUnionKeepsOneOfTwoIdenticalMembers() {
	c := newContainment(FieldReferenceSet, "")

	got := c.union([]string{"a"}, []string{"a"})

	s.Equal([]string{"a"}, got)
}

// OverlayTestSuite covers the narrowing algebra: what a rule may ask for, how two rules fold, and
// the absent-versus-empty distinction the pointer fields carry.
type OverlayTestSuite struct {
	suite.Suite
}

func TestOverlayTestSuite(t *testing.T) {
	suite.Run(t, new(OverlayTestSuite))
}

// set returns a pointer to a list, for the fields where absent and empty differ.
func set(v ...string) *[]string {
	out := append([]string{}, v...)
	return &out
}

// unconstrained is the open rule an owner starts from for an editable field.
var unconstrained = OverlayRule{Editable: true}

// validate checks a rule against itself, independent of any parent.
func (s *OverlayTestSuite) TestValidateRule() {
	c := newContainment(FieldReferenceSet, "")

	tests := []struct {
		name string
		rule OverlayRule
		want error
	}{
		{"open rule", OverlayRule{Editable: true}, nil},
		{"pinned rule", OverlayRule{Editable: false, Value: set("a")}, nil},
		{"menu", OverlayRule{Editable: true, AllowedValues: set("a", "b")}, nil},
		{"seeded and bounded", OverlayRule{Editable: true, Value: set("a"), AllowedValues: set("a", "b")}, nil},
		{
			"a menu nobody may choose from is an authoring mistake",
			OverlayRule{Editable: false, AllowedValues: set("a")},
			errPinnedWithAllowed,
		},
		{
			"value outside its own allowed set",
			OverlayRule{Editable: true, Value: set("c"), AllowedValues: set("a", "b")},
			errValueOutsideAllowed,
		},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.Equal(tt.want, c.validate(tt.rule))
		})
	}
}

// Editability only ever narrows: an initiator holding editable false cannot hand on true.
func (s *OverlayTestSuite) TestNarrowEditable() {
	c := newContainment(FieldReferenceSet, "")

	s.Run("false narrows true", func() {
		got, err := c.narrow(OverlayRule{Editable: true}, OverlayRule{Editable: false})
		s.Require().NoError(err)
		s.False(got.Editable)
	})

	s.Run("true cannot be handed on by an initiator holding false", func() {
		_, err := c.narrow(OverlayRule{Editable: false}, OverlayRule{Editable: true})
		s.ErrorIs(err, errWidens)
	})
}

// A child's bound must sit within its parent's, and an omitted bound inherits rather than
// meaning unconstrained.
func (s *OverlayTestSuite) TestNarrowAllowedValues() {
	c := newContainment(FieldReferenceSet, "")

	s.Run("a subset is accepted", func() {
		got, err := c.narrow(
			OverlayRule{Editable: true, AllowedValues: set("a", "b", "c")},
			OverlayRule{Editable: true, AllowedValues: set("a", "b")},
		)
		s.Require().NoError(err)
		s.Equal([]string{"a", "b"}, *got.AllowedValues)
	})

	s.Run("a superset widens", func() {
		_, err := c.narrow(
			OverlayRule{Editable: true, AllowedValues: set("a")},
			OverlayRule{Editable: true, AllowedValues: set("a", "b")},
		)
		s.ErrorIs(err, errWidens)
	})

	// This is the case that makes the pointer types necessary: omitted on the child must inherit
	// the parent's bound, not silently reopen the field.
	s.Run("omitted on the child inherits the parent bound", func() {
		got, err := c.narrow(OverlayRule{Editable: true, AllowedValues: set("a")}, unconstrained)
		s.Require().NoError(err)
		s.Require().NotNil(got.AllowedValues)
		s.Equal([]string{"a"}, *got.AllowedValues)
	})

	// The opposite of the above: explicitly empty permits nothing and must survive the fold.
	s.Run("explicitly empty on the child permits nothing", func() {
		got, err := c.narrow(
			OverlayRule{Editable: true, AllowedValues: set("a")},
			OverlayRule{Editable: true, AllowedValues: set()})
		s.Require().NoError(err)
		s.Require().NotNil(got.AllowedValues)
		s.Empty(*got.AllowedValues)
	})

	s.Run("an unconstrained parent accepts any child bound", func() {
		got, err := c.narrow(unconstrained, OverlayRule{Editable: true, AllowedValues: set("a")})
		s.Require().NoError(err)
		s.Equal([]string{"a"}, *got.AllowedValues)
	})
}

// Value narrowing is decided by the field's containment kind, so a hierarchy compares by path
// descent rather than by equality.
func (s *OverlayTestSuite) TestNarrowValueUsesKindContainment() {
	c := newContainment(FieldHierarchy, ":")

	s.Run("a descendant path is within an ancestor path", func() {
		got, err := c.narrow(
			OverlayRule{Editable: false, Value: set("billing")},
			OverlayRule{Editable: false, Value: set("billing:invoice")},
		)
		s.Require().NoError(err)
		s.Equal([]string{"billing:invoice"}, *got.Value)
	})

	s.Run("a sibling path widens", func() {
		_, err := c.narrow(
			OverlayRule{Editable: false, Value: set("billing")},
			OverlayRule{Editable: false, Value: set("bookings")},
		)
		s.ErrorIs(err, errWidens)
	})
}

// Exclusions run opposite to the other fields: more carve-outs are always allowed, un-carving
// never is.
func (s *OverlayTestSuite) TestNarrowExclusionsAreMonotoneTheOtherWay() {
	c := newContainment(FieldHierarchy, ":")

	s.Run("more carve-outs are allowed", func() {
		got, err := c.narrow(
			OverlayRule{Editable: false, ExcludedValues: set("billing")},
			OverlayRule{Editable: false, ExcludedValues: set("billing", "bookings")},
		)
		s.Require().NoError(err)
		s.ElementsMatch([]string{"billing", "bookings"}, *got.ExcludedValues)
	})

	// A request's carve-outs are added to the parent's rather than replacing them, so a request
	// naming fewer of them takes nothing back: the parent's survive on their own.
	s.Run("naming fewer of them takes none of them back", func() {
		got, err := c.narrow(
			OverlayRule{Editable: false, ExcludedValues: set("billing", "bookings")},
			OverlayRule{Editable: false, ExcludedValues: set("billing")},
		)
		s.Require().NoError(err)
		s.ElementsMatch([]string{"billing", "bookings"}, *got.ExcludedValues)
	})

	// The case that matters in practice: a request carves out something of its own without repeating
	// the parent's. The result carves out both, which is strictly narrower than the parent.
	s.Run("a carve-out of its own is a narrowing", func() {
		got, err := c.narrow(
			OverlayRule{Editable: true, ExcludedValues: set("billing")},
			OverlayRule{Editable: true, ExcludedValues: set("bookings")},
		)
		s.Require().NoError(err)
		s.ElementsMatch([]string{"billing", "bookings"}, *got.ExcludedValues,
			"requiring the request to restate every ancestor carve-out would refuse a narrowing")
	})

	s.Run("carving out nothing keeps the parent's", func() {
		got, err := c.narrow(
			OverlayRule{Editable: true, ExcludedValues: set("billing")},
			OverlayRule{Editable: true, ExcludedValues: set()},
		)
		s.Require().NoError(err)
		s.ElementsMatch([]string{"billing"}, *got.ExcludedValues)
	})

	s.Run("omitting the field keeps the parent's", func() {
		got, err := c.narrow(
			OverlayRule{Editable: true, ExcludedValues: set("billing")},
			OverlayRule{Editable: true},
		)
		s.Require().NoError(err)
		s.ElementsMatch([]string{"billing"}, *got.ExcludedValues)
	})
}

// Intersection rather than deepest-wins is what keeps rule resolution monotone: adding a covering
// policy can never widen the result.
func (s *OverlayTestSuite) TestIntersectIsMonotone() {
	c := newContainment(FieldReferenceSet, "")

	s.Run("editable only survives when every covering rule allows it", func() {
		got := c.intersect([]OverlayRule{{Editable: true}, {Editable: false}})
		s.False(got.Editable)
	})

	s.Run("allowed sets intersect", func() {
		got := c.intersect([]OverlayRule{
			{Editable: true, AllowedValues: set("a", "b")},
			{Editable: true, AllowedValues: set("b", "c")},
		})
		s.Equal([]string{"b"}, *got.AllowedValues)
	})

	// An unconstrained rule must not widen the one it is folded with.
	s.Run("an unconstrained rule contributes no bound", func() {
		got := c.intersect([]OverlayRule{{Editable: true}, {Editable: true, AllowedValues: set("a")}})
		s.Require().NotNil(got.AllowedValues)
		s.Equal([]string{"a"}, *got.AllowedValues)
	})

	// The value and the bound are folded independently, so one rule's value can meet another's
	// bound and carry members that bound never allowed. The fold has to stay closed over rules
	// Validate accepts.
	s.Run("an editable value is clamped to the bound it is folded with", func() {
		got := c.intersect([]OverlayRule{
			{Editable: true, Value: set("a", "b")},
			{Editable: true, AllowedValues: set("b", "c")},
		})

		s.Require().NotNil(got.Value)
		s.Equal([]string{"b"}, *got.Value, "the value keeps only what the bound allows")
		s.Require().NotNil(got.AllowedValues)
		s.Equal([]string{"b", "c"}, *got.AllowedValues)
		s.NoError(c.validate(got), "the fold must not produce a rule it would itself reject")
	})

	// Nothing survives when the two are disjoint, which is the honest answer rather than a value
	// standing outside its own menu.
	s.Run("a disjoint bound empties the value", func() {
		got := c.intersect([]OverlayRule{
			{Editable: true, Value: set("a")},
			{Editable: true, AllowedValues: set("b")},
		})

		s.Require().NotNil(got.Value)
		s.Empty(*got.Value)
		s.NoError(c.validate(got))
	})

	s.Run("exclusions accumulate", func() {
		got := c.intersect([]OverlayRule{
			{Editable: true, ExcludedValues: set("a")},
			{Editable: true, ExcludedValues: set("b")},
		})
		s.ElementsMatch([]string{"a", "b"}, *got.ExcludedValues)
	})

	// A pinned rule carrying no value of its own stands for whatever the owner holds, which a
	// surviving bound would never constrain. The fold has to move the bound onto the value, or a
	// policy that allowed only "a" and "b" ends up handing over the owner's "c" as well.
	s.Run("folding a bounded rule with a pinned one keeps the bound", func() {
		got := c.intersect([]OverlayRule{
			{Editable: true, AllowedValues: set("a", "b")},
			{Editable: false},
		})

		s.Require().NoError(c.validate(got), "the folded rule must satisfy its own invariant")
		s.Nil(got.AllowedValues, "a pinned rule carries no menu")
		s.Require().NotNil(got.Value, "the bound had to land on the value, since nothing else carries it")
		s.ElementsMatch([]string{"a", "b"}, *got.Value)
	})

	// The pinned rule names a value of its own that reaches past the bound. Every rule folded here
	// already applies, so the excess is clamped away rather than refused as Narrow would.
	s.Run("a pinned value reaching past the bound is clamped to it", func() {
		got := c.intersect([]OverlayRule{
			{Editable: true, AllowedValues: set("a", "b")},
			{Editable: false, Value: set("b", "c")},
		})

		s.Require().NoError(c.validate(got))
		s.Require().NotNil(got.Value)
		s.ElementsMatch([]string{"b"}, *got.Value)
	})

	// The property the whole choice of intersection over deepest-wins exists to guarantee.
	s.Run("adding a covering rule never widens the result", func() {
		narrow := OverlayRule{Editable: false, AllowedValues: nil, ExcludedValues: set("a")}
		broad := OverlayRule{Editable: true}

		withBoth := c.intersect([]OverlayRule{broad, narrow})

		s.False(withBoth.Editable, "the broader rule must not restore editability")
		s.ElementsMatch([]string{"a"}, *withBoth.ExcludedValues)
	})
}

// widens compares two already-materialized rules, where an omitted list means unbounded rather
// than inherited. That reading is the whole point of keeping it separate from Narrow.
func (s *OverlayTestSuite) TestWidens() {
	c := newContainment(FieldReferenceSet, "")
	bounded := OverlayRule{Editable: true, AllowedValues: set("a", "b")}

	s.Run("dropping a bound widens, because nothing named means everything", func() {
		s.True(c.widens(bounded, OverlayRule{Editable: true}))
	})

	s.Run("growing a bound widens", func() {
		s.True(c.widens(bounded, OverlayRule{Editable: true, AllowedValues: set("a", "b", "c")}))
	})

	s.Run("shrinking a bound does not", func() {
		s.False(c.widens(bounded, OverlayRule{Editable: true, AllowedValues: set("a")}))
	})

	s.Run("gaining editability widens", func() {
		s.True(c.widens(OverlayRule{Editable: false, Value: set("a")}, OverlayRule{Editable: true}))
	})

	// The case that makes reach necessary: a pinned rule carries no menu, so comparing its absent
	// AllowedValues against the bound would read the narrowest possible edit as a widening.
	s.Run("pinning within the bound does not widen", func() {
		s.False(c.widens(bounded, OverlayRule{Editable: false, Value: set("a")}))
	})

	s.Run("pinning outside the bound does widen", func() {
		s.True(c.widens(bounded, OverlayRule{Editable: false, Value: set("c")}))
	})

	// A pinned rule with no value of its own resolves to the owner's whole value.
	s.Run("dropping a pinned value widens", func() {
		s.True(c.widens(OverlayRule{Editable: false, Value: set("a")}, OverlayRule{Editable: false}))
	})

	s.Run("an unbounded rule cannot be widened", func() {
		s.False(c.widens(OverlayRule{Editable: true}, OverlayRule{Editable: true, AllowedValues: set("a")}))
	})

	s.Run("dropping a carve-out widens", func() {
		s.True(c.widens(
			OverlayRule{Editable: true, ExcludedValues: set("x")}, OverlayRule{Editable: true}))
	})

	s.Run("adding a carve-out does not", func() {
		s.False(c.widens(
			OverlayRule{Editable: true, ExcludedValues: set("x")},
			OverlayRule{Editable: true, ExcludedValues: set("x", "y")}))
	})
}

// A parent that is pinned and names no value grants whatever the owner holds, which is unbounded as
// far as the algebra can see. A request naming members is then a narrowing, and is accepted: there
// is no set to check it against, and whether those members exist is the resource type's question,
// asked through MemberValidator on the way in. A parent that does name a value is a different case,
// and bounds the request as usual.
func (s *OverlayTestSuite) TestNarrowUnderAParentThatNamesNoValue() {
	c := newContainment(FieldReferenceSet, "")
	unbounded := OverlayRule{Editable: false}

	s.Run("naming members under it is a narrowing", func() {
		got, err := c.narrow(unbounded, OverlayRule{Editable: false, Value: set("x")})

		s.Require().NoError(err)
		s.Equal([]string{"x"}, *got.Value)
		s.Nil(reach(unbounded), "the parent's reach is unbounded, which is why there is nothing to refuse")
		s.False(c.widens(unbounded, got), "and the result grants less than the parent, not more")
	})

	s.Run("a parent naming a value still bounds the request", func() {
		bounded := OverlayRule{Editable: false, Value: set("a", "b")}

		_, err := c.narrow(bounded, OverlayRule{Editable: false, Value: set("x")})
		s.ErrorIs(err, errWidens, "a member the parent does not hold is refused where the parent holds any")

		got, err := c.narrow(bounded, OverlayRule{Editable: false, Value: set("a")})
		s.Require().NoError(err)
		s.Equal([]string{"a"}, *got.Value)
	})
}

// Pinning is strictly narrower than an editable menu, so a parent that bounds the field must not
// turn a request carrying no allowedValues into one that is rejected for carrying them.
func (s *OverlayTestSuite) TestNarrowAcceptsPinningUnderABoundedParent() {
	c := newContainment(FieldReferenceSet, "")

	s.Run("with no value of its own it pins to the parent's bound", func() {
		got, err := c.narrow(
			OverlayRule{Editable: true, AllowedValues: set("a", "b")},
			OverlayRule{Editable: false},
		)
		s.Require().NoError(err)
		s.False(got.Editable)
		s.Nil(got.AllowedValues, "a pinned rule carries no menu")
		s.Require().NotNil(got.Value)
		s.ElementsMatch([]string{"a", "b"}, *got.Value)
	})

	s.Run("a named value within the bound is kept", func() {
		got, err := c.narrow(
			OverlayRule{Editable: true, AllowedValues: set("a", "b")},
			OverlayRule{Editable: false, Value: set("a")},
		)
		s.Require().NoError(err)
		s.Nil(got.AllowedValues)
		s.Equal([]string{"a"}, *got.Value)
	})

	// The parent bounds the field but names no value, so value narrowing has nothing to check
	// against; the bound has to do it instead.
	s.Run("a named value outside the bound still widens", func() {
		_, err := c.narrow(
			OverlayRule{Editable: true, AllowedValues: set("a", "b")},
			OverlayRule{Editable: false, Value: set("c")},
		)
		s.ErrorIs(err, errWidens)
	})
}

// What an editable rule offers is its menu. Its value is where the target starts, so a choice that
// leaves the value but stays on the menu is exactly what editable is for.
func (s *OverlayTestSuite) TestPermitsChecksTheMenuNotTheValue() {
	c := newContainment(FieldReferenceSet, "")
	r := OverlayRule{Editable: true, Value: set("a"), AllowedValues: set("a", "b")}

	s.True(c.permits(r, []string{"b"}))
	s.False(c.permits(r, []string{"c"}), "a member off the menu was accepted")
}

// A rule with no menu bounds nothing, which is the difference between an omitted list and an empty
// one: the empty list is the rule that permits nothing at all.
func (s *OverlayTestSuite) TestPermitsSeparatesAbsentFromEmpty() {
	c := newContainment(FieldReferenceSet, "")

	s.True(c.permits(OverlayRule{Editable: true}, []string{"anything"}))
	s.False(c.permits(OverlayRule{Editable: true, AllowedValues: set()}, []string{"anything"}))
}

// A pinned field is the owner's decision, so there is nothing for the target to choose.
func (s *OverlayTestSuite) TestPermitsRefusesAPinnedRule() {
	c := newContainment(FieldReferenceSet, "")

	s.False(c.permits(OverlayRule{Editable: false, Value: set("a")}, []string{"a"}))
}

// Exclusions are subtracted last, after the menu has been consulted, or a carve-out inside the menu
// would never take effect.
func (s *OverlayTestSuite) TestPermitsSubtractsExclusionsLast() {
	c := newContainment(FieldReferenceSet, "")
	r := OverlayRule{Editable: true, AllowedValues: set("a", "b"), ExcludedValues: set("b")}

	s.True(c.permits(r, []string{"a"}))
	s.False(c.permits(r, []string{"b"}))
}

// An exclusion on a hierarchy field carries its whole subtree, the same way containment works
// everywhere else in the engine.
func (s *OverlayTestSuite) TestPermitsExcludesAWholeSubtree() {
	c := newContainment(FieldHierarchy, ":")
	r := OverlayRule{Editable: true, ExcludedValues: set("billing")}

	s.False(c.permits(r, []string{"billing:invoice"}))
	s.True(c.permits(r, []string{"billingx"}), "a sibling sharing a prefix is not beneath it")
}

// A pinned rule grants what its value covers, or everything when it names no value, less what it
// carves out. Coverage is the hierarchy's own, so a path grants what lies beneath it and nothing
// that merely shares its prefix.
func (s *OverlayTestSuite) TestRuleGrantsAPinnedRule() {
	cases := []struct {
		name   string
		rule   OverlayRule
		member string
		want   bool
	}{
		{"no value grants everything", OverlayRule{}, "orders:read", true},
		{"a value grants itself", OverlayRule{Value: set("orders")}, "orders", true},
		{"a value grants what lies beneath it", OverlayRule{Value: set("orders")}, "orders:items:read", true},
		{"a value grants no sibling", OverlayRule{Value: set("orders")}, "reports:view", false},
		{"a value grants no prefix-sharing sibling", OverlayRule{Value: set("orders")}, "ordersx", false},
		{"an empty value grants nothing", OverlayRule{Value: set()}, "orders", false},
		{"an exclusion withholds its subtree", OverlayRule{ExcludedValues: set("orders")}, "orders:read", false},
		{"an exclusion leaves the rest", OverlayRule{ExcludedValues: set("orders:delete")}, "orders:read", true},
		{"an exclusion leaves the parent path", OverlayRule{ExcludedValues: set("orders:delete")}, "orders", true},
		{"an exclusion applies inside a value",
			OverlayRule{Value: set("orders"), ExcludedValues: set("orders:delete")}, "orders:delete", false},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.Equal(tc.want, RuleGrants(tc.rule, FieldHierarchy, ":", tc.member))
		})
	}
}

// An editable rule grants its menu rather than its value, which is where the target only starts.
func (s *OverlayTestSuite) TestRuleGrantsAnEditableRuleByItsMenu() {
	r := OverlayRule{Editable: true, Value: set("a"), AllowedValues: set("a", "b")}

	s.True(RuleGrants(r, FieldReferenceSet, "", "b"))
	s.False(RuleGrants(r, FieldReferenceSet, "", "c"))
}

// The delimiter is the resource's own, so a path is only beneath another across that separator.
func (s *OverlayTestSuite) TestRuleGrantsUsesTheGivenDelimiter() {
	r := OverlayRule{Value: set("orders")}

	s.True(RuleGrants(r, FieldHierarchy, "/", "orders/read"))
	s.False(RuleGrants(r, FieldHierarchy, "/", "orders:read"))
}

// With no choice made the rule's own value is what the target holds.
func (s *OverlayTestSuite) TestEffectiveFallsBackToTheRuleValue() {
	c := newContainment(FieldReferenceSet, "")

	got := c.effective(OverlayRule{Editable: true, Value: set("a", "b")}, nil)

	s.ElementsMatch([]string{"a", "b"}, got)
}

// A choice is cut back rather than refused, so an owner narrowing the field afterwards is not held
// up by a value that was accepted when it was written.
func (s *OverlayTestSuite) TestEffectiveClampsAChoiceToTheMenu() {
	c := newContainment(FieldReferenceSet, "")

	got := c.effective(OverlayRule{Editable: true, AllowedValues: set("a")}, []string{"a", "b"})

	s.Equal([]string{"a"}, got)
}

// A choice made before the field was pinned is not one the target still holds.
func (s *OverlayTestSuite) TestEffectiveIgnoresAChoiceOnAPinnedRule() {
	c := newContainment(FieldReferenceSet, "")

	got := c.effective(OverlayRule{Editable: false, Value: set("pinned")}, []string{"chosen"})

	s.Equal([]string{"pinned"}, got)
}

// This is the point where a rule's carve-outs are finally applied; nothing before it removes them.
func (s *OverlayTestSuite) TestEffectiveSubtractsExclusions() {
	c := newContainment(FieldHierarchy, ":")

	got := c.effective(
		OverlayRule{Editable: true, Value: set("billing", "support"), ExcludedValues: set("billing")},
		nil,
	)

	s.Equal([]string{"support"}, got)
}

// An empty choice is a choice: the target cleared the field, which is not the same as never having
// touched it and inheriting the rule's value.
func (s *OverlayTestSuite) TestEffectiveDistinguishesAnEmptyChoiceFromNone() {
	c := newContainment(FieldReferenceSet, "")
	r := OverlayRule{Editable: true, Value: set("a")}

	s.Empty(c.effective(r, []string{}))
	s.Equal([]string{"a"}, c.effective(r, nil))
}

// VisibilityTestSuite covers the chain walk that decides whether an organization unit can see a
// resource, and which policy covered it.
type VisibilityTestSuite struct {
	suite.Suite
}

func TestVisibilityTestSuite(t *testing.T) {
	suite.Run(t, new(VisibilityTestSuite))
}

const (
	owner  = "owner-ou"
	rootA  = "root-a"
	childA = "child-a"
	grandA = "grand-a"
)

// policy builds a share-stage policy owned by owner with one target.
func policy(id string, scope TargetScope, targetOU string, excluded ...string) Policy {
	return Policy{
		ID: id, OwningOUID: owner, InitiatingOUID: owner, Stage: stageShare,
		Targets: []Target{{ID: id + "-t", Scope: scope, OUID: targetOU, ExcludedOUIDs: excluded}},
	}
}

// reshare builds a reshare-stage policy issued by initiator with one target.
func reshare(id, initiator string, scope TargetScope, targetOU string, excluded ...string) Policy {
	p := policy(id, scope, targetOU, excluded...)
	p.Stage = stageReshare
	p.InitiatingOUID = initiator
	return p
}

// The owner holds its own resource, so it is visible whatever the policies say.
func (s *VisibilityTestSuite) TestEvaluateChainOwnerIsAlwaysVisible() {
	visible, covering := evaluateChain([]string{owner}, []Policy{policy("p1", ScopeRoot, rootA)})

	s.True(visible)
	// The owner needs no policy of its own, so nothing is recorded as covering it.
	s.Empty(covering)
}

// The three root-facing scopes each decide whether a tree's root is reached.
func (s *VisibilityTestSuite) TestEvaluateChainRootTargeting() {
	tests := []struct {
		name     string
		policies []Policy
		want     bool
	}{
		{"named root", []Policy{policy("p1", ScopeRoot, rootA)}, true},
		{"a different root", []Policy{policy("p1", ScopeRoot, "root-b")}, false},
		{"all roots", []Policy{policy("p1", ScopeAllRoots, "")}, true},
		{"all roots minus this one", []Policy{policy("p1", ScopeAllRoots, "", rootA)}, false},
		// Root targeting is owner-only, so a reshare must never reach a root.
		{"a reshare cannot reach a root", []Policy{reshare("p1", childA, ScopeRoot, rootA)}, false},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			visible, _ := evaluateChain([]string{rootA}, tt.policies)
			s.Equal(tt.want, visible)
		})
	}
}

// Visibility is carried hop by hop: reaching a root does not by itself reach anything below it.
func (s *VisibilityTestSuite) TestEvaluateChainRequiresEveryHop() {
	// The root is reached, but nothing carries the resource further down.
	visible, _ := evaluateChain(
		[]string{rootA, childA},
		[]Policy{policy("p1", ScopeRoot, rootA)},
	)
	s.False(visible, "a child is not visible merely because its root is")
}

// An all-children scope reaches every depth beneath its anchor, not only the first level.
func (s *VisibilityTestSuite) TestEvaluateChainSubtreeReachesAnyDepth() {
	policies := []Policy{
		policy("p1", ScopeRoot, rootA),
		reshare("p2", rootA, ScopeAllChildren, rootA),
	}

	visible, covering := evaluateChain([]string{rootA, childA, grandA}, policies)

	s.True(visible)
	s.Require().Len(covering, 1)
	s.Equal("p2", covering[0].Policy.ID)
}

// An excluded organization unit takes its whole subtree with it.
func (s *VisibilityTestSuite) TestEvaluateChainExclusionCutsOffEverythingBelow() {
	policies := []Policy{
		policy("p1", ScopeRoot, rootA),
		reshare("p2", rootA, ScopeAllChildren, rootA, childA),
	}

	childVisible, _ := evaluateChain([]string{rootA, childA}, policies)
	grandVisible, _ := evaluateChain([]string{rootA, childA, grandA}, policies)

	s.False(childVisible)
	s.False(grandVisible, "an excluded organization unit takes its subtree with it")
}

// Naming a grandchild directly does not reach it: depth comes from a subtree scope, never from
// naming deeper.
func (s *VisibilityTestSuite) TestEvaluateChainExplicitTargetNeverSkipsAHop() {
	// The root names a grandchild directly, which must not reach it.
	policies := []Policy{
		policy("p1", ScopeRoot, rootA),
		reshare("p2", rootA, ScopeChild, grandA),
	}

	visible, _ := evaluateChain([]string{rootA, childA, grandA}, policies)

	s.False(visible)
}

// A subtree target reaches the organization unit it names as well as everything beneath it.
func (s *VisibilityTestSuite) TestEvaluateChainSubtreeTargetCoversItsAnchorAndBelow() {
	policies := []Policy{
		policy("p1", ScopeRoot, rootA),
		reshare("p2", rootA, ScopeChildSubtree, childA),
	}

	anchorVisible, _ := evaluateChain([]string{rootA, childA}, policies)
	belowVisible, _ := evaluateChain([]string{rootA, childA, grandA}, policies)

	s.True(anchorVisible)
	s.True(belowVisible)
}

// The walk itself puts no limit on how many policies can cover a position, which is what peer
// targets will need. The service refuses to create this shape today, by allowing only a unit a
// policy named and stopped at to issue one of its own, so the engine is fed it directly here.
func (s *VisibilityTestSuite) TestEvaluateChainReturnsEveryCoveringPolicy() {
	// A diamond: a subtree reshare and a narrower one below it both reach the same organization
	// unit, and rule resolution needs both to intersect them.
	policies := []Policy{
		policy("p1", ScopeRoot, rootA),
		reshare("p2", rootA, ScopeAllChildren, rootA),
		reshare("p3", childA, ScopeChild, grandA),
	}

	visible, covering := evaluateChain([]string{rootA, childA, grandA}, policies)

	s.True(visible)
	ids := make([]string, 0, len(covering))
	for _, c := range covering {
		ids = append(ids, c.Policy.ID)
	}
	s.ElementsMatch([]string{"p2", "p3"}, ids)
}

// A blanket policy stands behind the specific ones rather than replacing them.
func (s *VisibilityTestSuite) TestEvaluateChainAllOUsIsAFallbackNotAShadow() {
	allOUs := policy("p-blanket", ScopeAllOUs, "")
	specific := reshare("p-specific", rootA, ScopeAllChildren, rootA)

	s.Run("it covers where nothing specific reached", func() {
		visible, covering := evaluateChain([]string{"other-root"}, []Policy{allOUs})
		s.True(visible)
		s.Require().Len(covering, 1)
		s.True(covering[0].ByAllOUs)
	})

	// The point of the carve-out: a deployment-wide policy must not join the intersection and
	// clamp a policy the owner deliberately made narrower.
	s.Run("it stands aside where a specific target reached", func() {
		_, covering := evaluateChain(
			[]string{rootA, childA},
			[]Policy{policy("p1", ScopeRoot, rootA), specific, allOUs},
		)
		s.Require().Len(covering, 1)
		s.Equal("p-specific", covering[0].Policy.ID)
		s.False(covering[0].ByAllOUs)
	})

	s.Run("an exclusion still applies to it", func() {
		visible, _ := evaluateChain([]string{"other-root"},
			[]Policy{policy("p-blanket", ScopeAllOUs, "", "other-root")})
		s.False(visible)
	})

	// A carve-out takes the unit's subtree with it, and a second target naming the carved-out unit
	// does not hand that subtree back. Without checking the whole chain the walk would reach the
	// grandchild: its parent is covered, by the other target, and the grandchild is not itself
	// named in the exclusion list.
	s.Run("an exclusion takes its subtree even when another target covers the unit", func() {
		carvedOut := Policy{
			ID: "p-mixed", OwningOUID: rootA, InitiatingOUID: rootA, Stage: stageShare,
			Targets: []Target{
				{ID: "t-blanket", Scope: ScopeAllOUs, ExcludedOUIDs: []string{childA}},
				{ID: "t-child", Scope: ScopeChild, OUID: childA},
			},
		}

		visible, covering := evaluateChain([]string{rootA, childA}, []Policy{carvedOut})
		s.True(visible, "the named target reaches the carved-out unit itself")
		s.Require().Len(covering, 1)
		s.Equal("t-child", covering[0].TargetID)

		visible, _ = evaluateChain([]string{rootA, childA, grandA}, []Policy{carvedOut})
		s.False(visible, "the blanket target was carved out above it and the named one stops short")
	})
}

// An empty chain names no organization unit, so nothing is visible and nothing covers it.
func (s *VisibilityTestSuite) TestEvaluateChainEmptyInputs() {
	visible, covering := evaluateChain(nil, []Policy{policy("p1", ScopeAllOUs, "")})
	s.False(visible)
	s.Nil(covering)

	visible, covering = evaluateChain([]string{rootA}, nil)
	s.False(visible)
	s.Nil(covering)
}

// EditTestSuite covers which edits a policy admits: the scope families, the blanket narrow-only
// rule, and the version check.
type EditTestSuite struct {
	suite.Suite
}

func TestEditTestSuite(t *testing.T) {
	suite.Run(t, new(EditTestSuite))
}

// withTargets returns a policy carrying the given targets.
func withTargets(targets ...Target) Policy {
	return Policy{ID: "p1", OwningOUID: owner, InitiatingOUID: owner, Stage: stageShare, Targets: targets}
}

// Every target scope belongs to exactly one family, which is what the edit rules key off.
func (s *EditTestSuite) TestFamilyOf() {
	blanket := []TargetScope{ScopeAllOUs, ScopeAllRoots, ScopeAllChildren}
	selective := []TargetScope{ScopeRoot, ScopeChild, ScopeChildSubtree}

	for _, scope := range blanket {
		s.Equal(familyBlanket, familyOf(scope), string(scope))
	}
	for _, scope := range selective {
		s.Equal(familySelective, familyOf(scope), string(scope))
	}
}

// A blanket policy already reaches everything in its family, so an edit may only carve out of it.
func (s *EditTestSuite) TestValidateEditBlanketMayOnlyBeNarrowed() {
	current := withTargets(Target{ID: "t1", Scope: ScopeAllChildren, OUID: owner})

	s.Run("keeping the same scope is allowed, so exclusions may change around it", func() {
		proposed := withTargets(Target{
			ID: "t1", Scope: ScopeAllChildren, OUID: owner, ExcludedOUIDs: []string{childA},
		})

		s.NoError(validateEdit(current, proposed, 1, 1))
	})

	// Converting a blanket policy would change the meaning of every reshare derived from it.
	s.Run("changing the blanket scope itself is rejected", func() {
		proposed := withTargets(Target{ID: "t1", Scope: ScopeAllOUs})

		s.ErrorIs(validateEdit(current, proposed, 1, 1), errBlanketScopeNarrowOnly)
	})

	s.Run("adding a target to a blanket policy is rejected", func() {
		proposed := withTargets(
			Target{ID: "t1", Scope: ScopeAllChildren, OUID: owner},
			Target{ID: "t2", Scope: ScopeAllChildren, OUID: childA},
		)

		s.ErrorIs(validateEdit(current, proposed, 1, 1), errBlanketScopeNarrowOnly)
	})
}

// A selective policy may grow within the one-hop rule, which creates no authority its initiator
// did not already hold when the policy was first checked.
func (s *EditTestSuite) TestValidateEditSelectiveMayGrow() {
	current := withTargets(Target{ID: "t1", Scope: ScopeChild, OUID: childA})

	// Expansion within the one-hop rule creates no authority the initiator did not already have:
	// it could have named the same children in the original call.
	s.Run("adding a target is allowed", func() {
		proposed := withTargets(
			Target{ID: "t1", Scope: ScopeChild, OUID: childA},
			Target{ID: "t2", Scope: ScopeChild, OUID: "child-b"},
		)

		s.NoError(validateEdit(current, proposed, 1, 1))
	})

	s.Run("flipping a target to carry its subtree is allowed", func() {
		proposed := withTargets(Target{ID: "t1", Scope: ScopeChildSubtree, OUID: childA})

		s.NoError(validateEdit(current, proposed, 1, 1))
	})

	s.Run("removing a target is allowed", func() {
		start := withTargets(
			Target{ID: "t1", Scope: ScopeChild, OUID: childA},
			Target{ID: "t2", Scope: ScopeChild, OUID: "child-b"},
		)
		proposed := withTargets(Target{ID: "t1", Scope: ScopeChild, OUID: childA})

		s.NoError(validateEdit(start, proposed, 1, 1))
	})
}

// Converting between the families would change the meaning of every reshare derived from the
// policy, so it is refused in both directions.
func (s *EditTestSuite) TestValidateEditCannotChangeFamily() {
	selective := withTargets(Target{ID: "t1", Scope: ScopeChild, OUID: childA})
	blanket := withTargets(Target{ID: "t1", Scope: ScopeAllChildren, OUID: owner})

	s.ErrorIs(validateEdit(selective, blanket, 1, 1), errScopeFamilyChange)
	s.ErrorIs(validateEdit(blanket, selective, 1, 1), errScopeFamilyChange)
}

// Exclusions are the only thing that narrows a blanket policy, so an edit may add them but never
// drop one: doing so restores visibility to an organization unit deliberately carved out.
func (s *EditTestSuite) TestValidateEditBlanketExclusionsMayBeEditedBothWays() {
	blanket := func(excluded ...string) Policy {
		return Policy{
			Targets: []Target{{ID: "t1", Scope: ScopeAllOUs, ExcludedOUIDs: excluded}},
		}
	}

	s.Run("adding one is allowed", func() {
		err := validateEdit(blanket("ou-1"), blanket("ou-1", "ou-2"), 1, 1)
		s.NoError(err)
	})

	s.Run("keeping the same set is allowed", func() {
		err := validateEdit(blanket("ou-1"), blanket("ou-1"), 1, 1)
		s.NoError(err)
	})

	// Dropping one hands the resource back to an organization unit the issuer carved out itself,
	// which is the issuer's own decision to reverse.
	s.Run("dropping one is allowed", func() {
		err := validateEdit(blanket("ou-1", "ou-2"), blanket("ou-1"), 1, 1)
		s.NoError(err)
	})

	s.Run("dropping all of them is allowed", func() {
		err := validateEdit(blanket("ou-1", "ou-2"), blanket(), 1, 1)
		s.NoError(err)
	})

	// What stays fixed is the family the policy reaches.
	s.Run("changing the blanket target is still refused", func() {
		current := Policy{Targets: []Target{{ID: "t1", Scope: ScopeAllOUs}}}
		proposed := Policy{Targets: []Target{{ID: "t1", Scope: ScopeAllRoots}}}
		err := validateEdit(current, proposed, 1, 1)
		s.ErrorIs(err, errBlanketScopeNarrowOnly)
	})

	// A selective policy may grow within the one-hop rule, so it is not held to the same rule.
	s.Run("a selective policy may drop one", func() {
		selective := func(excluded ...string) Policy {
			return Policy{
				Targets: []Target{{ID: "t1", Scope: ScopeChild, OUID: "ou-9", ExcludedOUIDs: excluded}},
			}
		}
		err := validateEdit(selective("ou-1", "ou-2"), selective("ou-1"), 1, 1)
		s.NoError(err)
	})
}

// LineageTestSuite covers the dependency ordering an export replays in, where a parent has to
// precede everything it covers.
type LineageTestSuite struct {
	suite.Suite
}

func TestLineageTestSuite(t *testing.T) {
	suite.Run(t, new(LineageTestSuite))
}

// indexOf returns the position of id in the ordered lineages.
func (s *LineageTestSuite) indexOf(ordered []Lineage, id string) int {
	s.T().Helper()
	for i, l := range ordered {
		if l.ID == id {
			return i
		}
	}
	s.Require().Failf("missing lineage", "id %s not in result", id)
	return -1
}

// Replaying an export in this order keeps every step valid, because the policy that makes an
// initiator visible is applied before the policy that initiator issued.
func (s *LineageTestSuite) TestOrderByDependencyPutsParentsFirst() {
	// Deliberately supplied children-first, so a stable sort alone would not produce the answer.
	ordered := orderByDependency([]Lineage{
		{ID: "grandchild", ParentID: "child"},
		{ID: "child", ParentID: "root"},
		{ID: "root"},
	})

	s.Require().Len(ordered, 3)
	s.Less(s.indexOf(ordered, "root"), s.indexOf(ordered, "child"))
	s.Less(s.indexOf(ordered, "child"), s.indexOf(ordered, "grandchild"))
}

// An export of one resource has to replay on its own, so a parent outside the set is a root here
// rather than an unsatisfiable dependency.
func (s *LineageTestSuite) TestOrderByDependencyTreatsAnAbsentParentAsARoot() {
	ordered := orderByDependency([]Lineage{
		{ID: "derived", ParentID: "not-in-this-export"},
	})

	s.Require().Len(ordered, 1)
	s.Equal("derived", ordered[0].ID)
}

// A cycle cannot arise from a well-formed graph, but the remainder is still emitted rather than
// silently dropped.
func (s *LineageTestSuite) TestOrderByDependencyKeepsEveryPolicyOnACycle() {
	ordered := orderByDependency([]Lineage{
		{ID: "a", ParentID: "b"},
		{ID: "b", ParentID: "a"},
	})

	s.Len(ordered, 2, "a malformed graph must not silently lose policies")
}

// Two entries sharing an id must both be emitted. Tracking placement by id instead of by position
// leaves the second one permanently unplaceable, and the loop never finishes; the test binary's own
// timeout is what surfaces that.
func (s *LineageTestSuite) TestOrderByDependencyKeepsBothEntriesWhenIDsCollide() {
	ordered := orderByDependency([]Lineage{{ID: "a"}, {ID: "a"}})

	s.Len(ordered, 2, "a duplicate id must not be dropped")
}

// A duplicate id must not disturb the ordering of the entries around it.
func (s *LineageTestSuite) TestOrderByDependencyStillOrdersAroundACollidingID() {
	ordered := orderByDependency([]Lineage{
		{ID: "child", ParentID: "dup"},
		{ID: "dup"},
		{ID: "dup"},
	})

	s.Require().Len(ordered, 3)
	s.Less(s.indexOf(ordered, "dup"), s.indexOf(ordered, "child"),
		"the parent still precedes what derives from it")
}
