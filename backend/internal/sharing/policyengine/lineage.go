// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package policyengine

// Lineage is the parent link of one policy, which is all the ordering and cascade walks need.
type Lineage struct {
	// ID identifies the policy.
	ID string
	// ParentID is the policy that made this one's initiator visible. Empty for owner-issued
	// policies and for anything derived from a declared one.
	ParentID string
}

// OrderByDependency sorts policies so a parent always precedes any policy derived from it.
//
// Replaying an export in this order keeps every step valid, because the policy that makes an
// initiator visible is always applied before the policy that initiator issued. Policies whose
// parent is absent from the set are treated as roots, which is what makes an export of one
// resource replayable on its own.
func OrderByDependency(lineages []Lineage) []Lineage {
	present := make(map[string]struct{}, len(lineages))
	for _, l := range lineages {
		present[l.ID] = struct{}{}
	}

	// Placement is tracked per entry rather than per id. Two entries sharing an id would otherwise
	// leave the second permanently unplaceable, and the loop below would spin rather than finish.
	placed := make([]bool, len(lineages))
	satisfied := make(map[string]bool, len(lineages))
	out := make([]Lineage, 0, len(lineages))

	for len(out) < len(lineages) {
		progressed := false
		for i, l := range lineages {
			if placed[i] {
				continue
			}
			_, parentInSet := present[l.ParentID]
			if l.ParentID == "" || !parentInSet || satisfied[l.ParentID] {
				out = append(out, l)
				placed[i] = true
				satisfied[l.ID] = true
				progressed = true
			}
		}
		// A cycle cannot arise from a well-formed graph, but emitting the remainder keeps the
		// caller's set complete rather than silently dropping policies.
		if !progressed {
			for i, l := range lineages {
				if !placed[i] {
					out = append(out, l)
					placed[i] = true
					satisfied[l.ID] = true
				}
			}
		}
	}
	return out
}
