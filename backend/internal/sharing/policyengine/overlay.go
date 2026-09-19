// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package policyengine

import "errors"

// Rule is what one policy says a target organization unit may do with one field.
//
// The list fields are pointers because absent and empty are opposite: omitted leaves the field
// unconstrained, an explicitly empty list permits nothing.
type Rule struct {
	// Editable reports whether the target organization unit may write the field.
	Editable bool
	// Value is what the target starts with, or is pinned to when the field is not editable.
	Value *[]string
	// AllowedValues bounds what the target may choose from. Nil means the field's whole universe.
	AllowedValues *[]string
	// ExcludedValues is subtracted from both Value and AllowedValues, last.
	ExcludedValues *[]string
}

// Errors returned by the narrowing algebra. The service maps each to its own SHR code.
var (
	// ErrWidens is returned when a rule asks for more than the initiating OU itself holds.
	ErrWidens = errors.New("rule widens what the initiating organization unit holds")
	// ErrValueOutsideAllowed is returned when a rule's value is not within its own allowed set.
	ErrValueOutsideAllowed = errors.New("value is not within allowedValues")
	// ErrPinnedWithAllowed is returned when a non-editable rule also bounds a choice nobody can make.
	ErrPinnedWithAllowed = errors.New("editable false cannot be combined with allowedValues")
)

// Validate checks a rule against itself, independent of any parent.
func (c Containment) Validate(r Rule) error {
	// A menu no one may choose from means the author misunderstood the shape, so this is an error
	// rather than a silent no-op.
	if !r.Editable && r.AllowedValues != nil {
		return ErrPinnedWithAllowed
	}
	if r.Value != nil && r.AllowedValues != nil {
		if !c.SetCovers(*r.AllowedValues, *r.Value) {
			return ErrValueOutsideAllowed
		}
	}
	return nil
}

// Narrow folds a requested rule against the initiating organization unit's own effective rule and
// returns what may actually be stored. A request that asks for anything the initiator does not
// itself hold is rejected rather than silently clamped, so the caller learns its policy was wrong.
func (c Containment) Narrow(parent, requested Rule) (Rule, error) {
	if err := c.Validate(requested); err != nil {
		return Rule{}, err
	}

	// An initiator holding editable false can never hand on editable true.
	if requested.Editable && !parent.Editable {
		return Rule{}, ErrWidens
	}

	out := Rule{Editable: requested.Editable}

	// Omitted on the child inherits the parent's bound; it does not mean unconstrained.
	switch {
	case requested.AllowedValues == nil:
		out.AllowedValues = parent.AllowedValues
	case parent.AllowedValues == nil:
		out.AllowedValues = requested.AllowedValues
	default:
		if !c.SetCovers(*parent.AllowedValues, *requested.AllowedValues) {
			return Rule{}, ErrWidens
		}
		out.AllowedValues = requested.AllowedValues
	}

	switch {
	case requested.Value == nil:
		out.Value = parent.Value
	case parent.Value == nil:
		out.Value = requested.Value
	default:
		if !c.SetCovers(*parent.Value, *requested.Value) {
			return Rule{}, ErrWidens
		}
		out.Value = requested.Value
	}

	// Exclusions are monotone the other way: more carve-outs are always allowed, un-carving is not.
	out.ExcludedValues = c.unionExclusions(parent.ExcludedValues, requested.ExcludedValues)
	if parent.ExcludedValues != nil && requested.ExcludedValues != nil {
		if !c.SetCovers(*requested.ExcludedValues, *parent.ExcludedValues) {
			return Rule{}, ErrWidens
		}
	}

	// A pinned rule has no use for a menu, because its value is fixed. Carrying the parent's
	// inherited bound onto one would fail this rule's own validation and report a request as
	// carrying allowedValues when it carried none, so the bound is applied to the value instead:
	// pinning with no value of its own pins to everything the parent could have handed over.
	if !out.Editable && out.AllowedValues != nil {
		switch {
		case out.Value == nil:
			pinned := append([]string{}, *out.AllowedValues...)
			out.Value = &pinned
		case !c.SetCovers(*out.AllowedValues, *out.Value):
			// Reachable when the parent bounds the field but names no value of its own, so the
			// value narrowing above had nothing to check the request against.
			return Rule{}, ErrWidens
		}
		out.AllowedValues = nil
	}

	if err := c.Validate(out); err != nil {
		return Rule{}, err
	}
	return out, nil
}

// unionExclusions merges two exclusion lists, preserving absent-vs-empty.
func (c Containment) unionExclusions(parent, child *[]string) *[]string {
	if parent == nil && child == nil {
		return nil
	}
	var merged []string
	if parent != nil {
		merged = append(merged, *parent...)
	}
	if child != nil {
		merged = append(merged, *child...)
	}
	u := c.Union(merged, nil)
	return &u
}

// Intersect folds every rule covering one organization unit into the single rule that applies.
//
// Intersection rather than deepest-wins is what keeps rule resolution monotone: revoking a narrow
// policy can then never widen a descendant that falls back to a broader ancestor.
func (c Containment) Intersect(rules []Rule) Rule {
	if len(rules) == 0 {
		return Rule{}
	}
	out := rules[0]
	for _, r := range rules[1:] {
		out = c.intersectPair(out, r)
	}
	return out
}

func (c Containment) intersectPair(a, b Rule) Rule {
	out := Rule{Editable: a.Editable && b.Editable}

	switch {
	case a.AllowedValues == nil:
		out.AllowedValues = b.AllowedValues
	case b.AllowedValues == nil:
		out.AllowedValues = a.AllowedValues
	default:
		v := c.IntersectMembers(*a.AllowedValues, *b.AllowedValues)
		out.AllowedValues = &v
	}

	switch {
	case a.Value == nil:
		out.Value = b.Value
	case b.Value == nil:
		out.Value = a.Value
	default:
		v := c.IntersectMembers(*a.Value, *b.Value)
		out.Value = &v
	}

	out.ExcludedValues = c.unionExclusions(a.ExcludedValues, b.ExcludedValues)
	return out
}

// Effective resolves a rule into the concrete sets a target organization unit sees.
//
// universe is the field's full value space and ownerValue the owning organization unit's own value;
// a non-editable rule with no value of its own resolves to the owner's, which is what makes
// "share everything except these" expressible without a wildcard.
func (c Containment) Effective(r Rule, universe, ownerValue []string) (allowed, value []string) {
	excluded := []string{}
	if r.ExcludedValues != nil {
		excluded = *r.ExcludedValues
	}

	if r.AllowedValues != nil {
		allowed = *r.AllowedValues
	} else {
		allowed = universe
	}
	allowed = c.Subtract(allowed, excluded)

	switch {
	case r.Value != nil:
		value = *r.Value
	case !r.Editable:
		value = ownerValue
	default:
		value = nil
	}
	value = c.Subtract(value, excluded)

	return allowed, value
}
