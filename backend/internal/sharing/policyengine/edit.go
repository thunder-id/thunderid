// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package policyengine

import (
	"errors"
	"slices"
)

// Errors returned by edit validation.
var (
	// ErrBlanketScopeNarrowOnly is returned when an edit tries to do more to a blanket policy than
	// add exclusions.
	ErrBlanketScopeNarrowOnly = errors.New("a blanket policy may only be narrowed by exclusions")
	// ErrScopeFamilyChange is returned when an edit tries to convert between scope families.
	ErrScopeFamilyChange = errors.New("a policy cannot change scope family")
	// ErrDeclaredImmutable is returned when an edit targets a declaratively defined policy.
	ErrDeclaredImmutable = errors.New("a declared policy can only be changed in its own file")
	// ErrVersionMismatch is returned when an edit is based on a stale version of the policy.
	ErrVersionMismatch = errors.New("policy version mismatch")
)

// ScopeFamily groups the target scopes that may be exchanged for one another by an edit.
type ScopeFamily string

const (
	// FamilyBlanket covers the scopes that already reach everything in their family.
	FamilyBlanket ScopeFamily = "blanket"
	// FamilySelective covers the scopes that name organization units individually.
	FamilySelective ScopeFamily = "selective"
)

// FamilyOf returns the family a scope belongs to.
func FamilyOf(s TargetScope) ScopeFamily {
	switch s {
	case TargetScopeAllOUs, TargetScopeAllRoots, TargetScopeAllChildren:
		return FamilyBlanket
	default:
		return FamilySelective
	}
}

// PolicyFamily returns the family of a policy's targets. A policy carries one mode, so its targets
// always agree.
func PolicyFamily(p Policy) ScopeFamily {
	if len(p.Targets) == 0 {
		return FamilySelective
	}
	return FamilyOf(p.Targets[0].Scope)
}

// ValidateEdit reports whether a proposed policy is a legal edit of the current one.
//
// A declared policy is editable: the file states where sharing starts, and an operator narrowing it
// afterwards is the expected flow. What the file does own is the policy's existence, so deletion is
// refused elsewhere. The scope-family rules below still apply, which is what keeps a declared
// blanket policy narrowable by exclusion and nothing more.
//
// The asymmetry between the families is deliberate. A blanket policy already reaches everything in
// its family, so there is nothing to expand toward and converting it would change the meaning of
// every reshare derived from it. A selective policy may grow within the one-hop rule, which creates
// no authority its initiator did not already have when the policy was first checked.
func ValidateEdit(current, proposed Policy, expectedVersion, actualVersion int) error {
	if expectedVersion != actualVersion {
		return ErrVersionMismatch
	}

	currentFamily := PolicyFamily(current)
	if currentFamily != PolicyFamily(proposed) {
		return ErrScopeFamilyChange
	}

	if currentFamily == FamilyBlanket {
		if !sameBlanketTargets(current.Targets, proposed.Targets) {
			return ErrBlanketScopeNarrowOnly
		}
		// Exclusions are the only thing that narrows a blanket policy, so dropping one restores
		// visibility to an organization unit that was deliberately carved out. A selective policy
		// may lose exclusions, because growing within the one-hop rule is a legal edit for it.
		if !containsAll(proposed.ExcludedOUIDs, current.ExcludedOUIDs) {
			return ErrBlanketScopeNarrowOnly
		}
	}
	return nil
}

// containsAll reports whether outer holds every member of inner.
func containsAll(outer, inner []string) bool {
	for _, want := range inner {
		if !slices.Contains(outer, want) {
			return false
		}
	}
	return true
}

// sameBlanketTargets reports whether two blanket target sets select the same thing, ignoring order.
// Only the exclusions around them may change.
func sameBlanketTargets(current, proposed []Target) bool {
	if len(current) != len(proposed) {
		return false
	}
	for _, c := range current {
		found := false
		for _, p := range proposed {
			if c.Scope == p.Scope && c.OUID == p.OUID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
