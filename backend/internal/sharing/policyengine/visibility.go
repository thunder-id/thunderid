// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package policyengine

import "slices"

// PolicyStage records who issued a policy.
type PolicyStage string

const (
	// StageShare is a policy issued by the resource's own owning organization unit.
	StageShare PolicyStage = "share"
	// StageReshare is a policy issued by an organization unit the resource was shared to.
	StageReshare PolicyStage = "reshare"
)

// TargetScope identifies the breadth of one target entry.
type TargetScope string

const (
	// TargetScopeAllOUs reaches every organization unit at every depth.
	TargetScopeAllOUs TargetScope = "all_ous"
	// TargetScopeAllRoots reaches every tree's root organization unit.
	TargetScopeAllRoots TargetScope = "all_roots"
	// TargetScopeRoot reaches one named root organization unit.
	TargetScopeRoot TargetScope = "root"
	// TargetScopeAllChildren reaches every organization unit beneath the initiator.
	TargetScopeAllChildren TargetScope = "all_children"
	// TargetScopeOU reaches one named organization unit alone.
	TargetScopeOU TargetScope = "ou"
	// TargetScopeOUSubtree reaches one named organization unit and everything beneath it.
	TargetScopeOUSubtree TargetScope = "ou_subtree"
)

// Target is one organization unit selection within a policy.
type Target struct {
	// ID identifies the target, so per-target rules can point at it.
	ID string
	// Scope is the breadth this entry reaches.
	Scope TargetScope
	// OUID is the named organization unit. Empty only for all_ous and all_roots: every other
	// scope anchors on a unit, all_children included, where it holds the issuing unit itself.
	OUID string
}

// Policy is the subset of a stored policy the chain evaluation needs.
type Policy struct {
	// ID identifies the policy.
	ID string
	// OwningOUID is the organization unit that owns the resource.
	OwningOUID string
	// InitiatingOUID is the organization unit whose decision this policy is.
	InitiatingOUID string
	// Stage records whether the owner or a sharee issued it.
	Stage PolicyStage
	// Targets is the set of organization unit selections.
	Targets []Target
	// ExcludedOUIDs carves organization units, and their subtrees, out of every target.
	ExcludedOUIDs []string
	// Declared marks a policy defined in a resource file, which cannot be edited.
	Declared bool
}

// Coverage records which policy and target made one chain position visible.
type Coverage struct {
	// Policy is the covering policy.
	Policy Policy
	// TargetID is the target entry that covered the position; empty when the owner covered itself.
	TargetID string
	// ByAllOUs marks coverage that only a deployment-wide policy provided.
	ByAllOUs bool
}

// EvaluateChain walks an organization unit chain, root first and the organization unit in question
// last, and reports whether the last position is visible along with every policy covering it.
//
// Visibility chains rather than pattern-matches: a position is visible only if every hop above it
// is, so a missing or excluded link cuts off everything below it.
//
// Only a unit a policy named and stopped at may issue one of its own, which the service enforces on
// write, so today exactly one policy covers a position and the slice holds one entry. The slice is
// the return type all the same: peer targets reach a unit sideways, without needing the position
// above it covered, and will put a second policy on units a parent-chain policy already reaches.
func EvaluateChain(chain []string, policies []Policy) (visible bool, covering []Coverage) {
	if len(chain) == 0 || len(policies) == 0 {
		return false, nil
	}
	owningOUID := policies[0].OwningOUID

	covered := make([]bool, len(chain))
	coveredBy := make([][]Coverage, len(chain))

	for i, ouID := range chain {
		// The owner is unconditionally visible to its own resource, wherever it sits in the chain.
		if ouID == owningOUID {
			covered[i] = true
			continue
		}

		if i == 0 {
			coveredBy[0] = rootCoverage(ouID, policies)
			covered[0] = len(coveredBy[0]) > 0
		} else {
			coveredBy[i] = descendantCoverage(chain, i, covered, policies)
			covered[i] = len(coveredBy[i]) > 0
		}

		// all_ous is a fallback, never a shadow: it applies only where no specific target reached,
		// so a narrower policy stays authoritative for the positions it does cover.
		if !covered[i] && (i == 0 || covered[i-1]) {
			if c, ok := allOUsCoverage(ouID, policies); ok {
				coveredBy[i], covered[i] = []Coverage{c}, true
			}
		}
	}

	last := len(chain) - 1
	return covered[last], coveredBy[last]
}

// rootCoverage returns the share-stage policies reaching a tree's root directly. Root targeting is
// owner-only, so only share-stage policies are considered.
func rootCoverage(ouID string, policies []Policy) []Coverage {
	var out []Coverage
	for _, p := range policies {
		if p.Stage != StageShare || slices.Contains(p.ExcludedOUIDs, ouID) {
			continue
		}
		for _, t := range p.Targets {
			switch {
			case t.Scope == TargetScopeAllRoots:
				out = append(out, Coverage{Policy: p, TargetID: t.ID})
			case t.Scope == TargetScopeRoot && t.OUID == ouID:
				out = append(out, Coverage{Policy: p, TargetID: t.ID})
			}
		}
	}
	return out
}

// descendantCoverage returns every policy reaching chain[i] from a covered position above it.
func descendantCoverage(chain []string, i int, covered []bool, policies []Policy) []Coverage {
	var out []Coverage
	ouID := chain[i]

	for j := i - 1; j >= 0; j-- {
		if !covered[j] {
			continue
		}
		for _, p := range policies {
			for _, t := range p.Targets {
				// A subtree target behaves from its anchor downwards exactly as an all_children
				// target issued by that organization unit would.
				anchored := (t.Scope == TargetScopeAllChildren || t.Scope == TargetScopeOUSubtree) &&
					t.OUID == chain[j]
				if anchored && !excludedBetween(chain, j, i, p.ExcludedOUIDs) {
					out = append(out, Coverage{Policy: p, TargetID: t.ID})
					continue
				}
				// An explicit target never authorizes skipping a hop, so it only covers the
				// position immediately below the organization unit that named it.
				if j == i-1 && (t.Scope == TargetScopeOU || t.Scope == TargetScopeOUSubtree) &&
					t.OUID == ouID && !slices.Contains(p.ExcludedOUIDs, ouID) {
					out = append(out, Coverage{Policy: p, TargetID: t.ID})
				}
			}
		}
	}
	return dedupe(out)
}

// allOUsCoverage returns a deployment-wide policy reaching ouID, if one exists.
func allOUsCoverage(ouID string, policies []Policy) (Coverage, bool) {
	for _, p := range policies {
		if slices.Contains(p.ExcludedOUIDs, ouID) {
			continue
		}
		for _, t := range p.Targets {
			if t.Scope == TargetScopeAllOUs {
				return Coverage{Policy: p, TargetID: t.ID, ByAllOUs: true}, true
			}
		}
	}
	return Coverage{}, false
}

// excludedBetween reports whether any organization unit from the anchor down to the target is
// carved out, which cuts off everything below it.
func excludedBetween(chain []string, from, to int, excluded []string) bool {
	for _, mid := range chain[from+1 : to+1] {
		if slices.Contains(excluded, mid) {
			return true
		}
	}
	return false
}

// dedupe drops repeated (policy, target) pairs. No single target can be appended twice today, since
// an anchored target matches one ancestor and an explicit one matches one position; it guards the
// walk against the case rather than a case the callers currently reach.
func dedupe(in []Coverage) []Coverage {
	if len(in) < 2 {
		return in
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]Coverage, 0, len(in))
	for _, c := range in {
		key := c.Policy.ID + "\x00" + c.TargetID
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, c)
	}
	return out
}
