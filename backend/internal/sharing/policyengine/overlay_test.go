// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package policyengine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// set returns a pointer to a list, for the fields where absent and empty differ.
func set(v ...string) *[]string {
	out := append([]string{}, v...)
	return &out
}

// unconstrained is the open rule an owner starts from for an editable field.
var unconstrained = Rule{Editable: true}

// Validate checks a rule against itself, independent of any parent.
func TestValidateRule(t *testing.T) {
	c := NewContainment(FieldReferenceSet, "")

	tests := []struct {
		name string
		rule Rule
		want error
	}{
		{"open rule", Rule{Editable: true}, nil},
		{"pinned rule", Rule{Editable: false, Value: set("a")}, nil},
		{"menu", Rule{Editable: true, AllowedValues: set("a", "b")}, nil},
		{"seeded and bounded", Rule{Editable: true, Value: set("a"), AllowedValues: set("a", "b")}, nil},
		{
			"a menu nobody may choose from is an authoring mistake",
			Rule{Editable: false, AllowedValues: set("a")},
			ErrPinnedWithAllowed,
		},
		{
			"value outside its own allowed set",
			Rule{Editable: true, Value: set("c"), AllowedValues: set("a", "b")},
			ErrValueOutsideAllowed,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, c.Validate(tt.rule))
		})
	}
}

// Editability only ever narrows: an initiator holding editable false cannot hand on true.
func TestNarrowEditable(t *testing.T) {
	c := NewContainment(FieldReferenceSet, "")

	t.Run("false narrows true", func(t *testing.T) {
		got, err := c.Narrow(Rule{Editable: true}, Rule{Editable: false})
		require.NoError(t, err)
		assert.False(t, got.Editable)
	})

	t.Run("true cannot be handed on by an initiator holding false", func(t *testing.T) {
		_, err := c.Narrow(Rule{Editable: false}, Rule{Editable: true})
		assert.ErrorIs(t, err, ErrWidens)
	})
}

// A child's bound must sit within its parent's, and an omitted bound inherits rather than
// meaning unconstrained.
func TestNarrowAllowedValues(t *testing.T) {
	c := NewContainment(FieldReferenceSet, "")

	t.Run("a subset is accepted", func(t *testing.T) {
		got, err := c.Narrow(
			Rule{Editable: true, AllowedValues: set("a", "b", "c")},
			Rule{Editable: true, AllowedValues: set("a", "b")},
		)
		require.NoError(t, err)
		assert.Equal(t, []string{"a", "b"}, *got.AllowedValues)
	})

	t.Run("a superset widens", func(t *testing.T) {
		_, err := c.Narrow(
			Rule{Editable: true, AllowedValues: set("a")},
			Rule{Editable: true, AllowedValues: set("a", "b")},
		)
		assert.ErrorIs(t, err, ErrWidens)
	})

	// This is the case that makes the pointer types necessary: omitted on the child must inherit
	// the parent's bound, not silently reopen the field.
	t.Run("omitted on the child inherits the parent bound", func(t *testing.T) {
		got, err := c.Narrow(Rule{Editable: true, AllowedValues: set("a")}, unconstrained)
		require.NoError(t, err)
		require.NotNil(t, got.AllowedValues)
		assert.Equal(t, []string{"a"}, *got.AllowedValues)
	})

	// The opposite of the above: explicitly empty permits nothing and must survive the fold.
	t.Run("explicitly empty on the child permits nothing", func(t *testing.T) {
		got, err := c.Narrow(Rule{Editable: true, AllowedValues: set("a")}, Rule{Editable: true, AllowedValues: set()})
		require.NoError(t, err)
		require.NotNil(t, got.AllowedValues)
		assert.Empty(t, *got.AllowedValues)
	})

	t.Run("an unconstrained parent accepts any child bound", func(t *testing.T) {
		got, err := c.Narrow(unconstrained, Rule{Editable: true, AllowedValues: set("a")})
		require.NoError(t, err)
		assert.Equal(t, []string{"a"}, *got.AllowedValues)
	})
}

// Value narrowing is decided by the field's containment kind, so a hierarchy compares by path
// descent rather than by equality.
func TestNarrowValueUsesKindContainment(t *testing.T) {
	c := NewContainment(FieldHierarchy, ":")

	t.Run("a descendant path is within an ancestor path", func(t *testing.T) {
		got, err := c.Narrow(
			Rule{Editable: false, Value: set("billing")},
			Rule{Editable: false, Value: set("billing:invoice")},
		)
		require.NoError(t, err)
		assert.Equal(t, []string{"billing:invoice"}, *got.Value)
	})

	t.Run("a sibling path widens", func(t *testing.T) {
		_, err := c.Narrow(
			Rule{Editable: false, Value: set("billing")},
			Rule{Editable: false, Value: set("bookings")},
		)
		assert.ErrorIs(t, err, ErrWidens)
	})
}

// Exclusions run opposite to the other fields: more carve-outs are always allowed, un-carving
// never is.
func TestNarrowExclusionsAreMonotoneTheOtherWay(t *testing.T) {
	c := NewContainment(FieldHierarchy, ":")

	t.Run("more carve-outs are allowed", func(t *testing.T) {
		got, err := c.Narrow(
			Rule{Editable: false, ExcludedValues: set("billing")},
			Rule{Editable: false, ExcludedValues: set("billing", "bookings")},
		)
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"billing", "bookings"}, *got.ExcludedValues)
	})

	t.Run("un-carving is not", func(t *testing.T) {
		_, err := c.Narrow(
			Rule{Editable: false, ExcludedValues: set("billing", "bookings")},
			Rule{Editable: false, ExcludedValues: set("billing")},
		)
		assert.ErrorIs(t, err, ErrWidens)
	})
}

// Intersection rather than deepest-wins is what keeps rule resolution monotone: adding a covering
// policy can never widen the result.
func TestIntersectIsMonotone(t *testing.T) {
	c := NewContainment(FieldReferenceSet, "")

	t.Run("editable only survives when every covering rule allows it", func(t *testing.T) {
		got := c.Intersect([]Rule{{Editable: true}, {Editable: false}})
		assert.False(t, got.Editable)
	})

	t.Run("allowed sets intersect", func(t *testing.T) {
		got := c.Intersect([]Rule{
			{Editable: true, AllowedValues: set("a", "b")},
			{Editable: true, AllowedValues: set("b", "c")},
		})
		assert.Equal(t, []string{"b"}, *got.AllowedValues)
	})

	// An unconstrained rule must not widen the one it is folded with.
	t.Run("an unconstrained rule contributes no bound", func(t *testing.T) {
		got := c.Intersect([]Rule{{Editable: true}, {Editable: true, AllowedValues: set("a")}})
		require.NotNil(t, got.AllowedValues)
		assert.Equal(t, []string{"a"}, *got.AllowedValues)
	})

	t.Run("exclusions accumulate", func(t *testing.T) {
		got := c.Intersect([]Rule{
			{Editable: true, ExcludedValues: set("a")},
			{Editable: true, ExcludedValues: set("b")},
		})
		assert.ElementsMatch(t, []string{"a", "b"}, *got.ExcludedValues)
	})

	// Effective reads a pinned rule carrying no value of its own as the owner's value, which a
	// surviving bound would never constrain. The fold has to move the bound onto the value, or a
	// policy that allowed only "a" and "b" ends up handing over the owner's "c" as well.
	t.Run("folding a bounded rule with a pinned one keeps the bound", func(t *testing.T) {
		got := c.Intersect([]Rule{
			{Editable: true, AllowedValues: set("a", "b")},
			{Editable: false},
		})

		require.NoError(t, c.Validate(got), "the folded rule must satisfy its own invariant")
		assert.Nil(t, got.AllowedValues, "a pinned rule carries no menu")
		require.NotNil(t, got.Value)
		assert.ElementsMatch(t, []string{"a", "b"}, *got.Value)

		universe := []string{"a", "b", "c"}
		_, value := c.Effective(got, universe, universe)
		assert.NotContains(t, value, "c", "the owner's value must stay within the bound")
		assert.ElementsMatch(t, []string{"a", "b"}, value)
	})

	// The pinned rule names a value of its own that reaches past the bound. Every rule folded here
	// already applies, so the excess is clamped away rather than refused as Narrow would.
	t.Run("a pinned value reaching past the bound is clamped to it", func(t *testing.T) {
		got := c.Intersect([]Rule{
			{Editable: true, AllowedValues: set("a", "b")},
			{Editable: false, Value: set("b", "c")},
		})

		require.NoError(t, c.Validate(got))
		require.NotNil(t, got.Value)
		assert.ElementsMatch(t, []string{"b"}, *got.Value)
	})

	// The property the whole choice of intersection over deepest-wins exists to guarantee.
	t.Run("adding a covering rule never widens the result", func(t *testing.T) {
		narrow := Rule{Editable: false, AllowedValues: nil, ExcludedValues: set("a")}
		broad := Rule{Editable: true}

		withBoth := c.Intersect([]Rule{broad, narrow})

		assert.False(t, withBoth.Editable, "the broader rule must not restore editability")
		assert.ElementsMatch(t, []string{"a"}, *withBoth.ExcludedValues)
	})
}

// Effective resolves a rule into the concrete allowed set and value a target organization unit
// sees, including the owner-value fallback for a pinned rule.
func TestEffective(t *testing.T) {
	c := NewContainment(FieldHierarchy, ":")
	universe := []string{"bookings", "billing", "reports"}
	ownerValue := []string{"bookings", "billing"}

	t.Run("not editable with no value resolves to the owner's own value", func(t *testing.T) {
		allowed, value := c.Effective(Rule{Editable: false}, universe, ownerValue)
		assert.Equal(t, universe, allowed)
		assert.Equal(t, ownerValue, value)
	})

	// "Everything except refunds" needs no wildcard: the owner's value already is everything.
	t.Run("not editable with exclusions is the owner's value minus them", func(t *testing.T) {
		_, value := c.Effective(
			Rule{Editable: false, ExcludedValues: set("billing")}, universe, ownerValue)
		assert.Equal(t, []string{"bookings"}, value)
	})

	t.Run("an editable field with no value starts unset", func(t *testing.T) {
		_, value := c.Effective(unconstrained, universe, ownerValue)
		assert.Empty(t, value)
	})

	t.Run("a pinned value wins over the owner's", func(t *testing.T) {
		_, value := c.Effective(
			Rule{Editable: false, Value: set("reports")}, universe, ownerValue)
		assert.Equal(t, []string{"reports"}, value)
	})

	t.Run("exclusions are subtracted from the allowed set too", func(t *testing.T) {
		allowed, _ := c.Effective(
			Rule{Editable: true, AllowedValues: set("bookings", "billing"), ExcludedValues: set("billing")},
			universe, ownerValue)
		assert.Equal(t, []string{"bookings"}, allowed)
	})

	// Explicitly empty must resolve to nothing permitted, not to the universe.
	t.Run("an explicitly empty allowed set permits nothing", func(t *testing.T) {
		allowed, _ := c.Effective(Rule{Editable: true, AllowedValues: set()}, universe, ownerValue)
		assert.Empty(t, allowed)
	})
}

// Widens compares two already-materialized rules, where an omitted list means unbounded rather
// than inherited. That reading is the whole point of keeping it separate from Narrow.
func TestWidens(t *testing.T) {
	c := NewContainment(FieldReferenceSet, "")
	bounded := Rule{Editable: true, AllowedValues: set("a", "b")}

	t.Run("dropping a bound widens, because nothing named means everything", func(t *testing.T) {
		assert.True(t, c.Widens(bounded, Rule{Editable: true}))
	})

	t.Run("growing a bound widens", func(t *testing.T) {
		assert.True(t, c.Widens(bounded, Rule{Editable: true, AllowedValues: set("a", "b", "c")}))
	})

	t.Run("shrinking a bound does not", func(t *testing.T) {
		assert.False(t, c.Widens(bounded, Rule{Editable: true, AllowedValues: set("a")}))
	})

	t.Run("gaining editability widens", func(t *testing.T) {
		assert.True(t, c.Widens(Rule{Editable: false, Value: set("a")}, Rule{Editable: true}))
	})

	// The case that makes reach necessary: a pinned rule carries no menu, so comparing its absent
	// AllowedValues against the bound would read the narrowest possible edit as a widening.
	t.Run("pinning within the bound does not widen", func(t *testing.T) {
		assert.False(t, c.Widens(bounded, Rule{Editable: false, Value: set("a")}))
	})

	t.Run("pinning outside the bound does widen", func(t *testing.T) {
		assert.True(t, c.Widens(bounded, Rule{Editable: false, Value: set("c")}))
	})

	// A pinned rule with no value of its own resolves to the owner's whole value.
	t.Run("dropping a pinned value widens", func(t *testing.T) {
		assert.True(t, c.Widens(Rule{Editable: false, Value: set("a")}, Rule{Editable: false}))
	})

	t.Run("an unbounded rule cannot be widened", func(t *testing.T) {
		assert.False(t, c.Widens(Rule{Editable: true}, Rule{Editable: true, AllowedValues: set("a")}))
	})

	t.Run("dropping a carve-out widens", func(t *testing.T) {
		assert.True(t, c.Widens(
			Rule{Editable: true, ExcludedValues: set("x")}, Rule{Editable: true}))
	})

	t.Run("adding a carve-out does not", func(t *testing.T) {
		assert.False(t, c.Widens(
			Rule{Editable: true, ExcludedValues: set("x")},
			Rule{Editable: true, ExcludedValues: set("x", "y")}))
	})
}

// Pinning is strictly narrower than an editable menu, so a parent that bounds the field must not
// turn a request carrying no allowedValues into one that is rejected for carrying them.
func TestNarrowAcceptsPinningUnderABoundedParent(t *testing.T) {
	c := NewContainment(FieldReferenceSet, "")

	t.Run("with no value of its own it pins to the parent's bound", func(t *testing.T) {
		got, err := c.Narrow(
			Rule{Editable: true, AllowedValues: set("a", "b")},
			Rule{Editable: false},
		)
		require.NoError(t, err)
		assert.False(t, got.Editable)
		assert.Nil(t, got.AllowedValues, "a pinned rule carries no menu")
		require.NotNil(t, got.Value)
		assert.ElementsMatch(t, []string{"a", "b"}, *got.Value)
	})

	t.Run("a named value within the bound is kept", func(t *testing.T) {
		got, err := c.Narrow(
			Rule{Editable: true, AllowedValues: set("a", "b")},
			Rule{Editable: false, Value: set("a")},
		)
		require.NoError(t, err)
		assert.Nil(t, got.AllowedValues)
		assert.Equal(t, []string{"a"}, *got.Value)
	})

	// The parent bounds the field but names no value, so value narrowing has nothing to check
	// against; the bound has to do it instead.
	t.Run("a named value outside the bound still widens", func(t *testing.T) {
		_, err := c.Narrow(
			Rule{Editable: true, AllowedValues: set("a", "b")},
			Rule{Editable: false, Value: set("c")},
		)
		assert.ErrorIs(t, err, ErrWidens)
	})
}
