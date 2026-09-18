// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package policyengine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withTargets returns a policy carrying the given targets.
func withTargets(targets ...Target) Policy {
	return Policy{ID: "p1", OwningOUID: owner, InitiatingOUID: owner, Stage: StageShare, Targets: targets}
}

func TestFamilyOf(t *testing.T) {
	blanket := []TargetScope{TargetScopeAllOUs, TargetScopeAllRoots, TargetScopeAllChildren}
	selective := []TargetScope{TargetScopeRoot, TargetScopeOU, TargetScopeOUSubtree}

	for _, s := range blanket {
		assert.Equal(t, FamilyBlanket, FamilyOf(s), string(s))
	}
	for _, s := range selective {
		assert.Equal(t, FamilySelective, FamilyOf(s), string(s))
	}
}

func TestValidateEditBlanketMayOnlyBeNarrowed(t *testing.T) {
	current := withTargets(Target{ID: "t1", Scope: TargetScopeAllChildren, OUID: owner})

	t.Run("keeping the same scope is allowed, so exclusions may change around it", func(t *testing.T) {
		proposed := withTargets(Target{ID: "t1", Scope: TargetScopeAllChildren, OUID: owner})
		proposed.ExcludedOUIDs = []string{childA}

		assert.NoError(t, ValidateEdit(current, proposed, 1, 1))
	})

	// Converting a blanket policy would change the meaning of every reshare derived from it.
	t.Run("changing the blanket scope itself is rejected", func(t *testing.T) {
		proposed := withTargets(Target{ID: "t1", Scope: TargetScopeAllOUs})

		assert.ErrorIs(t, ValidateEdit(current, proposed, 1, 1), ErrBlanketScopeNarrowOnly)
	})

	t.Run("adding a target to a blanket policy is rejected", func(t *testing.T) {
		proposed := withTargets(
			Target{ID: "t1", Scope: TargetScopeAllChildren, OUID: owner},
			Target{ID: "t2", Scope: TargetScopeAllChildren, OUID: childA},
		)

		assert.ErrorIs(t, ValidateEdit(current, proposed, 1, 1), ErrBlanketScopeNarrowOnly)
	})
}

func TestValidateEditSelectiveMayGrow(t *testing.T) {
	current := withTargets(Target{ID: "t1", Scope: TargetScopeOU, OUID: childA})

	// Expansion within the one-hop rule creates no authority the initiator did not already have:
	// it could have named the same children in the original call.
	t.Run("adding a target is allowed", func(t *testing.T) {
		proposed := withTargets(
			Target{ID: "t1", Scope: TargetScopeOU, OUID: childA},
			Target{ID: "t2", Scope: TargetScopeOU, OUID: "child-b"},
		)

		assert.NoError(t, ValidateEdit(current, proposed, 1, 1))
	})

	t.Run("flipping a target to carry its subtree is allowed", func(t *testing.T) {
		proposed := withTargets(Target{ID: "t1", Scope: TargetScopeOUSubtree, OUID: childA})

		assert.NoError(t, ValidateEdit(current, proposed, 1, 1))
	})

	t.Run("removing a target is allowed", func(t *testing.T) {
		start := withTargets(
			Target{ID: "t1", Scope: TargetScopeOU, OUID: childA},
			Target{ID: "t2", Scope: TargetScopeOU, OUID: "child-b"},
		)
		proposed := withTargets(Target{ID: "t1", Scope: TargetScopeOU, OUID: childA})

		assert.NoError(t, ValidateEdit(start, proposed, 1, 1))
	})
}

func TestValidateEditCannotChangeFamily(t *testing.T) {
	selective := withTargets(Target{ID: "t1", Scope: TargetScopeOU, OUID: childA})
	blanket := withTargets(Target{ID: "t1", Scope: TargetScopeAllChildren, OUID: owner})

	assert.ErrorIs(t, ValidateEdit(selective, blanket, 1, 1), ErrScopeFamilyChange)
	assert.ErrorIs(t, ValidateEdit(blanket, selective, 1, 1), ErrScopeFamilyChange)
}

func TestValidateEditRejectsADeclaredPolicy(t *testing.T) {
	current := withTargets(Target{ID: "t1", Scope: TargetScopeOU, OUID: childA})
	current.Declared = true

	err := ValidateEdit(current, current, 1, 1)

	assert.ErrorIs(t, err, ErrDeclaredImmutable)
}

// Without the version check two administrators editing one policy silently last-write-wins, and
// for a security policy the loser's carve-outs are exactly what must not vanish.
func TestValidateEditRejectsAStaleVersion(t *testing.T) {
	current := withTargets(Target{ID: "t1", Scope: TargetScopeOU, OUID: childA})

	err := ValidateEdit(current, current, 1, 2)

	assert.ErrorIs(t, err, ErrVersionMismatch)
}

func TestValidateEditChecksImmutabilityBeforeAnythingElse(t *testing.T) {
	current := withTargets(Target{ID: "t1", Scope: TargetScopeOU, OUID: childA})
	current.Declared = true
	blanket := withTargets(Target{ID: "t1", Scope: TargetScopeAllChildren, OUID: owner})

	// Both rules are violated; the declared one is the more fundamental answer.
	err := ValidateEdit(current, blanket, 1, 2)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDeclaredImmutable)
}

// Exclusions are the only thing that narrows a blanket policy, so an edit may add them but never
// drop one: doing so restores visibility to an organization unit deliberately carved out.
func TestValidateEditBlanketExclusionsAreAddOnly(t *testing.T) {
	blanket := func(excluded ...string) Policy {
		return Policy{
			Targets:       []Target{{ID: "t1", Scope: TargetScopeAllOUs}},
			ExcludedOUIDs: excluded,
		}
	}

	t.Run("adding one is allowed", func(t *testing.T) {
		err := ValidateEdit(blanket("ou-1"), blanket("ou-1", "ou-2"), 1, 1)
		assert.NoError(t, err)
	})

	t.Run("keeping the same set is allowed", func(t *testing.T) {
		err := ValidateEdit(blanket("ou-1"), blanket("ou-1"), 1, 1)
		assert.NoError(t, err)
	})

	t.Run("dropping one is refused", func(t *testing.T) {
		err := ValidateEdit(blanket("ou-1", "ou-2"), blanket("ou-1"), 1, 1)
		assert.ErrorIs(t, err, ErrBlanketScopeNarrowOnly)
	})

	// A selective policy may grow within the one-hop rule, so it is not held to the same rule.
	t.Run("a selective policy may drop one", func(t *testing.T) {
		selective := func(excluded ...string) Policy {
			return Policy{
				Targets:       []Target{{ID: "t1", Scope: TargetScopeOU, OUID: "ou-9"}},
				ExcludedOUIDs: excluded,
			}
		}
		err := ValidateEdit(selective("ou-1", "ou-2"), selective("ou-1"), 1, 1)
		assert.NoError(t, err)
	})
}
