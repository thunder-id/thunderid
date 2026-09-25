// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package policyengine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	owner  = "owner-ou"
	rootA  = "root-a"
	childA = "child-a"
	grandA = "grand-a"
)

// policy builds a share-stage policy owned by owner with one target.
func policy(id string, scope TargetScope, targetOU string, excluded ...string) Policy {
	return Policy{
		ID: id, OwningOUID: owner, InitiatingOUID: owner, Stage: StageShare,
		Targets:       []Target{{ID: id + "-t", Scope: scope, OUID: targetOU}},
		ExcludedOUIDs: excluded,
	}
}

// reshare builds a reshare-stage policy issued by initiator with one target.
func reshare(id, initiator string, scope TargetScope, targetOU string, excluded ...string) Policy {
	p := policy(id, scope, targetOU, excluded...)
	p.Stage = StageReshare
	p.InitiatingOUID = initiator
	return p
}

// The owner holds its own resource, so it is visible whatever the policies say.
func TestEvaluateChainOwnerIsAlwaysVisible(t *testing.T) {
	visible, covering := EvaluateChain([]string{owner}, []Policy{policy("p1", TargetScopeRoot, rootA)})

	assert.True(t, visible)
	// The owner needs no policy of its own, so nothing is recorded as covering it.
	assert.Empty(t, covering)
}

// The three root-facing scopes each decide whether a tree's root is reached.
func TestEvaluateChainRootTargeting(t *testing.T) {
	tests := []struct {
		name     string
		policies []Policy
		want     bool
	}{
		{"named root", []Policy{policy("p1", TargetScopeRoot, rootA)}, true},
		{"a different root", []Policy{policy("p1", TargetScopeRoot, "root-b")}, false},
		{"all roots", []Policy{policy("p1", TargetScopeAllRoots, "")}, true},
		{"all roots minus this one", []Policy{policy("p1", TargetScopeAllRoots, "", rootA)}, false},
		// Root targeting is owner-only, so a reshare must never reach a root.
		{"a reshare cannot reach a root", []Policy{reshare("p1", childA, TargetScopeRoot, rootA)}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			visible, _ := EvaluateChain([]string{rootA}, tt.policies)
			assert.Equal(t, tt.want, visible)
		})
	}
}

// Visibility is carried hop by hop: reaching a root does not by itself reach anything below it.
func TestEvaluateChainRequiresEveryHop(t *testing.T) {
	// The root is reached, but nothing carries the resource further down.
	visible, _ := EvaluateChain(
		[]string{rootA, childA},
		[]Policy{policy("p1", TargetScopeRoot, rootA)},
	)
	assert.False(t, visible, "a child is not visible merely because its root is")
}

// An all-children scope reaches every depth beneath its anchor, not only the first level.
func TestEvaluateChainSubtreeReachesAnyDepth(t *testing.T) {
	policies := []Policy{
		policy("p1", TargetScopeRoot, rootA),
		reshare("p2", rootA, TargetScopeAllChildren, rootA),
	}

	visible, covering := EvaluateChain([]string{rootA, childA, grandA}, policies)

	assert.True(t, visible)
	require.Len(t, covering, 1)
	assert.Equal(t, "p2", covering[0].Policy.ID)
}

// An excluded organization unit takes its whole subtree with it.
func TestEvaluateChainExclusionCutsOffEverythingBelow(t *testing.T) {
	policies := []Policy{
		policy("p1", TargetScopeRoot, rootA),
		reshare("p2", rootA, TargetScopeAllChildren, rootA, childA),
	}

	childVisible, _ := EvaluateChain([]string{rootA, childA}, policies)
	grandVisible, _ := EvaluateChain([]string{rootA, childA, grandA}, policies)

	assert.False(t, childVisible)
	assert.False(t, grandVisible, "an excluded organization unit takes its subtree with it")
}

// Naming a grandchild directly does not reach it: depth comes from a subtree scope, never from
// naming deeper.
func TestEvaluateChainExplicitTargetNeverSkipsAHop(t *testing.T) {
	// The root names a grandchild directly, which must not reach it.
	policies := []Policy{
		policy("p1", TargetScopeRoot, rootA),
		reshare("p2", rootA, TargetScopeOU, grandA),
	}

	visible, _ := EvaluateChain([]string{rootA, childA, grandA}, policies)

	assert.False(t, visible)
}

// A subtree target reaches the organization unit it names as well as everything beneath it.
func TestEvaluateChainSubtreeTargetCoversItsAnchorAndBelow(t *testing.T) {
	policies := []Policy{
		policy("p1", TargetScopeRoot, rootA),
		reshare("p2", rootA, TargetScopeOUSubtree, childA),
	}

	anchorVisible, _ := EvaluateChain([]string{rootA, childA}, policies)
	belowVisible, _ := EvaluateChain([]string{rootA, childA, grandA}, policies)

	assert.True(t, anchorVisible)
	assert.True(t, belowVisible)
}

// Coverage is not a single edge: rule resolution intersects every policy that reaches the unit,
// so all of them have to come back.
func TestEvaluateChainReturnsEveryCoveringPolicy(t *testing.T) {
	// A diamond: the owner's blanket subtree policy and a narrower reshare both reach the same
	// organization unit, and rule resolution needs both to intersect them.
	policies := []Policy{
		policy("p1", TargetScopeRoot, rootA),
		reshare("p2", rootA, TargetScopeAllChildren, rootA),
		reshare("p3", childA, TargetScopeOU, grandA),
	}

	visible, covering := EvaluateChain([]string{rootA, childA, grandA}, policies)

	assert.True(t, visible)
	ids := make([]string, 0, len(covering))
	for _, c := range covering {
		ids = append(ids, c.Policy.ID)
	}
	assert.ElementsMatch(t, []string{"p2", "p3"}, ids)
}

// A blanket policy stands behind the specific ones rather than replacing them.
func TestEvaluateChainAllOUsIsAFallbackNotAShadow(t *testing.T) {
	allOUs := policy("p-blanket", TargetScopeAllOUs, "")
	specific := reshare("p-specific", rootA, TargetScopeAllChildren, rootA)

	t.Run("it covers where nothing specific reached", func(t *testing.T) {
		visible, covering := EvaluateChain([]string{"other-root"}, []Policy{allOUs})
		assert.True(t, visible)
		require.Len(t, covering, 1)
		assert.True(t, covering[0].ByAllOUs)
	})

	// The point of the carve-out: a deployment-wide policy must not join the intersection and
	// clamp a policy the owner deliberately made narrower.
	t.Run("it stands aside where a specific target reached", func(t *testing.T) {
		_, covering := EvaluateChain(
			[]string{rootA, childA},
			[]Policy{policy("p1", TargetScopeRoot, rootA), specific, allOUs},
		)
		require.Len(t, covering, 1)
		assert.Equal(t, "p-specific", covering[0].Policy.ID)
		assert.False(t, covering[0].ByAllOUs)
	})

	t.Run("an exclusion still applies to it", func(t *testing.T) {
		visible, _ := EvaluateChain([]string{"other-root"},
			[]Policy{policy("p-blanket", TargetScopeAllOUs, "", "other-root")})
		assert.False(t, visible)
	})
}

// An empty chain names no organization unit, so nothing is visible and nothing covers it.
func TestEvaluateChainEmptyInputs(t *testing.T) {
	visible, covering := EvaluateChain(nil, []Policy{policy("p1", TargetScopeAllOUs, "")})
	assert.False(t, visible)
	assert.Nil(t, covering)

	visible, covering = EvaluateChain([]string{rootA}, nil)
	assert.False(t, visible)
	assert.Nil(t, covering)
}
