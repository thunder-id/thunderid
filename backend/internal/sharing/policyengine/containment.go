// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package policyengine holds the sharing framework's decision-making as pure functions over
// values. Nothing here performs I/O, reads a clock or takes a context, which is what makes the
// narrowing algebra, containment and chain evaluation table-testable without a database.
package policyengine

import "strings"

// FieldKind mirrors the resource type's declaration of how a field's members compare.
type FieldKind string

const (
	// FieldScalar is one opaque value, compared by equality.
	FieldScalar FieldKind = "scalar"
	// FieldReferenceSet is a set of opaque ids, compared by set membership.
	FieldReferenceSet FieldKind = "referenceSet"
	// FieldHierarchy is a set of delimiter-joined paths, where naming a path also names everything
	// beneath it.
	FieldHierarchy FieldKind = "hierarchy"
)

// DefaultDelimiter is used for hierarchy fields when a resource type declares none of its own.
const DefaultDelimiter = ":"

// Containment compares members of one field kind.
type Containment struct {
	// Kind selects the comparison.
	Kind FieldKind
	// Delimiter joins hierarchical path segments. Ignored for the other kinds.
	Delimiter string
}

// NewContainment returns the containment for a field kind, defaulting the delimiter.
func NewContainment(kind FieldKind, delimiter string) Containment {
	if delimiter == "" {
		delimiter = DefaultDelimiter
	}
	return Containment{Kind: kind, Delimiter: delimiter}
}

// Covers reports whether outer denotes inner. For hierarchy fields a path covers itself and any
// path beneath it; for the other kinds this is equality.
func (c Containment) Covers(outer, inner string) bool {
	if outer == inner {
		return true
	}
	if c.Kind != FieldHierarchy {
		return false
	}
	return strings.HasPrefix(inner, outer+c.Delimiter)
}

// SetCovers reports whether every member of inner is covered by some member of outer.
func (c Containment) SetCovers(outer, inner []string) bool {
	for _, i := range inner {
		if !c.CoveredBy(outer, i) {
			return false
		}
	}
	return true
}

// CoveredBy reports whether any member of set covers member.
func (c Containment) CoveredBy(set []string, member string) bool {
	for _, s := range set {
		if c.Covers(s, member) {
			return true
		}
	}
	return false
}

// Subtract removes from members everything covered by any entry of excluded.
func (c Containment) Subtract(members, excluded []string) []string {
	if len(excluded) == 0 {
		return members
	}
	out := make([]string, 0, len(members))
	for _, m := range members {
		if !c.CoveredBy(excluded, m) {
			out = append(out, m)
		}
	}
	return out
}

// IntersectMembers returns the members of a that are covered by b, and the members of b that are covered
// by a but not already included. For hierarchy fields this keeps the narrower of two overlapping
// paths, so intersecting "billing" with "billing:invoice" yields "billing:invoice".
func (c Containment) IntersectMembers(a, b []string) []string {
	out := make([]string, 0, min(len(a), len(b)))
	seen := make(map[string]struct{})
	add := func(m string) {
		if _, dup := seen[m]; dup {
			return
		}
		seen[m] = struct{}{}
		out = append(out, m)
	}
	for _, m := range a {
		if c.CoveredBy(b, m) {
			add(m)
		}
	}
	for _, m := range b {
		if c.CoveredBy(a, m) {
			add(m)
		}
	}
	return out
}

// Union returns every member of a and b, dropping those already covered by another member so a
// hierarchy set stays free of redundant descendants.
func (c Containment) Union(a, b []string) []string {
	combined := make([]string, 0, len(a)+len(b))
	combined = append(combined, a...)
	combined = append(combined, b...)

	out := make([]string, 0, len(combined))
	for i, m := range combined {
		redundant := false
		for j, other := range combined {
			if i == j {
				continue
			}
			// Keep the first of two identical members, drop any member a different one covers.
			if other == m && j > i {
				continue
			}
			if c.Covers(other, m) {
				redundant = true
				break
			}
		}
		if !redundant {
			out = append(out, m)
		}
	}
	return out
}
