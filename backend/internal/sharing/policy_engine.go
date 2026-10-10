// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sharing

import (
	"errors"
	"slices"
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// The framework's decision-making, as pure functions over values: the narrowing algebra, member
// containment, chain evaluation and edit validation. Nothing here performs I/O, reads a clock or
// takes a context, which is what keeps it testable from a table of inputs. The service above
// fetches and stores; every judgement it makes is a call into this file.

// defaultDelimiter is the separator a containment is built with when none is given.
const defaultDelimiter = ":"

// containment compares members of one field kind.
type containment struct {
	// kind selects the comparison.
	kind FieldKind
	// delimiter joins hierarchical path segments. Ignored for the other kinds.
	delimiter string
}

// newContainment returns the containment for a field kind, defaulting the delimiter.
func newContainment(kind FieldKind, delimiter string) containment {
	if delimiter == "" {
		delimiter = defaultDelimiter
	}
	return containment{kind: kind, delimiter: delimiter}
}

// covers reports whether outer denotes inner. For hierarchy fields a path covers itself and any
// path beneath it; for the other kinds this is equality.
func (c containment) covers(outer, inner string) bool {
	if outer == inner {
		return true
	}
	if c.kind != FieldHierarchy {
		return false
	}
	return strings.HasPrefix(inner, outer+c.delimiter)
}

// setCovers reports whether every member of inner is covered by some member of outer.
func (c containment) setCovers(outer, inner []string) bool {
	for _, i := range inner {
		if !c.coveredBy(outer, i) {
			return false
		}
	}
	return true
}

// coveredBy reports whether any member of set covers member.
func (c containment) coveredBy(set []string, member string) bool {
	for _, s := range set {
		if c.covers(s, member) {
			return true
		}
	}
	return false
}

// intersectMembers returns the members of a that are covered by b, and the members of b that are covered
// by a but not already included. For hierarchy fields this keeps the narrower of two overlapping
// paths, so intersecting "billing" with "billing:invoice" yields "billing:invoice".
func (c containment) intersectMembers(a, b []string) []string {
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
		if c.coveredBy(b, m) {
			add(m)
		}
	}
	for _, m := range b {
		if c.coveredBy(a, m) {
			add(m)
		}
	}
	return out
}

// union returns every member of a and b, dropping those already covered by another member so a
// hierarchy set stays free of redundant descendants.
func (c containment) union(a, b []string) []string {
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
			if c.covers(other, m) {
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

// validate checks a rule against itself, independent of any parent.
func (c containment) validate(r OverlayRule) error {
	// A menu no one may choose from means the author misunderstood the shape, so this is an error
	// rather than a silent no-op.
	if !r.Editable && r.AllowedValues != nil {
		return errPinnedWithAllowed
	}
	if r.Value != nil && r.AllowedValues != nil {
		if !c.setCovers(*r.AllowedValues, *r.Value) {
			return errValueOutsideAllowed
		}
	}
	return nil
}

// narrow folds a requested rule against the initiating organization unit's own effective rule and
// returns what may actually be stored. A request that asks for anything the initiator does not
// itself hold is rejected rather than silently clamped, so the caller learns its policy was wrong.
func (c containment) narrow(parent, requested OverlayRule) (OverlayRule, error) {
	if err := c.validate(requested); err != nil {
		return OverlayRule{}, err
	}

	// An initiator holding editable false can never hand on editable true.
	if requested.Editable && !parent.Editable {
		return OverlayRule{}, errWidens
	}

	out := OverlayRule{Editable: requested.Editable}

	// Omitted on the child inherits the parent's bound; it does not mean unconstrained.
	switch {
	case requested.AllowedValues == nil:
		out.AllowedValues = parent.AllowedValues
	case parent.AllowedValues == nil:
		out.AllowedValues = requested.AllowedValues
	default:
		if !c.setCovers(*parent.AllowedValues, *requested.AllowedValues) {
			return OverlayRule{}, errWidens
		}
		out.AllowedValues = requested.AllowedValues
	}

	switch {
	case requested.Value == nil:
		out.Value = parent.Value
	case parent.Value == nil:
		// The parent names no value, so it grants everything the resource's owner holds and there
		// is no set here to check the request against. Naming members is therefore a narrowing of
		// something unbounded, which this cannot refuse: whether those members exist at all, and
		// whether the initiator may name them, is the resource type's own question, asked through
		// MemberValidator before this is reached. The framework keeps no rows of its own for the
		// resource and so cannot answer it here.
		out.Value = requested.Value
	default:
		if !c.setCovers(*parent.Value, *requested.Value) {
			return OverlayRule{}, errWidens
		}
		out.Value = requested.Value
	}

	// Exclusions compose rather than replace: a request's carve-outs are added to the parent's, so
	// every value the parent withheld stays withheld whatever the request says. That is what makes
	// un-carving impossible here, and why there is nothing to refuse. A request naming one carve-out
	// of its own is narrowing, not widening, even though it does not restate the parent's.
	//
	// Widens is the other case and keeps its own check: there a proposed rule replaces the stored
	// one on the same policy, so dropping a carve-out really would give something back.
	out.ExcludedValues = c.unionExclusions(parent.ExcludedValues, requested.ExcludedValues)

	// A pinned rule has no use for a menu, because its value is fixed. Carrying the parent's
	// inherited bound onto one would fail this rule's own validation and report a request as
	// carrying allowedValues when it carried none, so the bound is applied to the value instead:
	// pinning with no value of its own pins to everything the parent could have handed over.
	if !out.Editable && out.AllowedValues != nil {
		switch {
		case out.Value == nil:
			pinned := append([]string{}, *out.AllowedValues...)
			out.Value = &pinned
		case !c.setCovers(*out.AllowedValues, *out.Value):
			// Reachable when the parent bounds the field but names no value of its own, so the
			// value narrowing above had nothing to check the request against.
			return OverlayRule{}, errWidens
		}
		out.AllowedValues = nil
	}

	if err := c.validate(out); err != nil {
		return OverlayRule{}, err
	}
	return out, nil
}

// reach returns the members a rule can put in a target's hands.
//
// Which field carries that depends on editability: an editable rule grants its menu, and a pinned
// one grants its value alone. A nil result means unbounded, because an editable rule with no menu
// may choose anything and a pinned rule with no value of its own takes the owner's whole value.
func reach(r OverlayRule) *[]string {
	if r.Editable {
		return r.AllowedValues
	}
	return r.Value
}

// widens reports whether proposed grants more than current for the same field.
//
// Both sides are already-materialized rules, which is what separates this from Narrow. Narrow
// resolves a request against a ceiling and reads an omitted list as "inherit from the parent".
// Here an omitted list is the resolved state itself, and it means unbounded. Dropping a restriction
// is exactly the widening this exists to catch, so the two readings cannot share an implementation.
//
// The comparison goes through reach rather than field by field. A pinned rule carries no menu, by
// the invariant Narrow maintains, so comparing its absent AllowedValues against the bound of an
// editable rule would read pinning as a widening when it is the narrowest thing a rule can do.
func (c containment) widens(current, proposed OverlayRule) bool {
	if proposed.Editable && !current.Editable {
		return true
	}

	// Exclusions are subtracted from whatever the rule resolves to, so every carve-out current made
	// has to survive, whichever side is editable.
	if current.ExcludedValues != nil {
		if proposed.ExcludedValues == nil ||
			!c.setCovers(*proposed.ExcludedValues, *current.ExcludedValues) {
			return true
		}
	}

	currentReach := reach(current)
	if currentReach == nil {
		return false
	}
	proposedReach := reach(proposed)
	return proposedReach == nil || !c.setCovers(*currentReach, *proposedReach)
}

// unionExclusions merges two exclusion lists, preserving absent-vs-empty.
func (c containment) unionExclusions(parent, child *[]string) *[]string {
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
	u := c.union(merged, nil)
	return &u
}

// intersect folds every rule covering one organization unit into the single rule that applies.
//
// With one policy covering a unit this receives one rule and returns it. It folds rather than takes
// the nearest because peer coverage will hand it two, and a unit reached twice must hold only what
// both permit. Write-time narrowing already keeps a nearer rule inside a farther one, so on a
// parent chain the two agree anyway.
func (c containment) intersect(rules []OverlayRule) OverlayRule {
	if len(rules) == 0 {
		return OverlayRule{}
	}
	out := rules[0]
	for _, r := range rules[1:] {
		out = c.intersectPair(out, r)
	}
	return out
}

// permits reports whether a target organization unit may choose these members for the field.
//
// The bound an editable rule offers is its menu, not its value: the value is where the target
// starts, and choosing something else from the menu is what editable means. A pinned rule permits
// nothing, because its value is the owner's decision rather than an opening position. Exclusions
// are subtracted last, as everywhere else, so a carved-out member is refused even when the menu
// covers it.
func (c containment) permits(r OverlayRule, members []string) bool {
	if !r.Editable {
		return false
	}
	for _, m := range members {
		if r.ExcludedValues != nil && c.coveredBy(*r.ExcludedValues, m) {
			return false
		}
	}
	if r.AllowedValues == nil {
		return true
	}
	return c.setCovers(*r.AllowedValues, members)
}

// effective returns what a target organization unit actually holds for a field: the members it
// chose where the rule still permits them, and the rule's own value where it did not choose.
//
// This is where ExcludedValues is finally subtracted. A rule carries its carve-outs rather than
// baking them into Value and AllowedValues, because composing two policies has to union the
// carve-outs before either list is read; the subtraction therefore belongs at the point of use,
// which is here.
//
// A choice is clamped rather than refused, for the same reason Clamp exists: a value accepted when
// it was written must not stop an ancestor from narrowing the field afterwards. A pinned rule
// ignores the choice outright, since a choice made before the field was pinned is not a choice the
// target still holds.
func (c containment) effective(r OverlayRule, chosen []string) []string {
	base := chosen
	if !r.Editable || chosen == nil {
		base = nil
		if r.Value != nil {
			base = *r.Value
		}
	}

	out := make([]string, 0, len(base))
	for _, m := range base {
		if r.ExcludedValues != nil && c.coveredBy(*r.ExcludedValues, m) {
			continue
		}
		if r.AllowedValues != nil && !c.coveredBy(*r.AllowedValues, m) {
			continue
		}
		out = append(out, m)
	}
	return out
}

// clamp cuts requested back to what parent permits, without refusing it.
//
// narrow is the write-time operation: a unit asking for more than it holds has made a mistake worth
// reporting. Clamp is the replay-time one: the request was accepted once already, and an ancestor
// has since tightened. Refusing there would let a sharee's old request stop an owner from narrowing
// its own resource, so the request is cut back to the new ceiling instead. The original request is
// what is kept on the policy, so widening the ancestor again restores what it had cut back.
func (c containment) clamp(parent, requested OverlayRule) OverlayRule {
	return c.intersectPair(parent, requested)
}

// intersectPair folds two rules into the single rule that satisfies both.
func (c containment) intersectPair(a, b OverlayRule) OverlayRule {
	out := OverlayRule{Editable: a.Editable && b.Editable}

	switch {
	case a.AllowedValues == nil:
		out.AllowedValues = b.AllowedValues
	case b.AllowedValues == nil:
		out.AllowedValues = a.AllowedValues
	default:
		v := c.intersectMembers(*a.AllowedValues, *b.AllowedValues)
		out.AllowedValues = &v
	}

	switch {
	case a.Value == nil:
		out.Value = b.Value
	case b.Value == nil:
		out.Value = a.Value
	default:
		v := c.intersectMembers(*a.Value, *b.Value)
		out.Value = &v
	}

	out.ExcludedValues = c.unionExclusions(a.ExcludedValues, b.ExcludedValues)

	if out.Value != nil && out.AllowedValues != nil {
		v := c.intersectMembers(*out.Value, *out.AllowedValues)
		out.Value = &v
	}

	// A pinned rule has no use for a menu, and one carrying no value of its own stands for whatever
	// the owner holds, which the bound would then never constrain. Folding a bounded editable rule
	// with a pinned one therefore has to move the bound onto the value, the same normalization Narrow
	// applies when it resolves a rule. The clamp above has already done the intersecting, so all that
	// is left here is the case where the pinned rule brought no value of its own.
	if !out.Editable && out.AllowedValues != nil {
		if out.Value == nil {
			pinned := append([]string{}, *out.AllowedValues...)
			out.Value = &pinned
		}
		out.AllowedValues = nil
	}

	return out
}

// evaluateChain walks an organization unit chain, root first and the organization unit in question
// last, and reports whether the last position is visible along with every policy covering it.
//
// Visibility chains rather than pattern-matches: a position is visible only if every hop above it
// is, so a missing or excluded link cuts off everything below it.
//
// Only a unit a policy named and stopped at may issue one of its own, which the service enforces on
// write, so today exactly one policy covers a position and the slice holds one entry. The slice is
// the return type all the same: peer targets reach a unit sideways, without needing the position
// above it covered, and will put a second policy on units a parent-chain policy already reaches.
func evaluateChain(chain []string, policies []Policy) (visible bool, covering []Coverage) {
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
			if c, ok := allOUsCoverage(chain, i, policies); ok {
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
		if p.Stage != stageShare {
			continue
		}
		for _, t := range p.Targets {
			if slices.Contains(t.ExcludedOUIDs, ouID) {
				continue
			}
			switch {
			case t.Scope == ScopeAllRoots:
				out = append(out, Coverage{Policy: p, TargetID: t.ID})
			case t.Scope == ScopeRoot && t.OUID == ouID:
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
				anchored := (t.Scope == ScopeAllChildren || t.Scope == ScopeChildSubtree) &&
					t.OUID == chain[j]
				if anchored && !excludedBetween(chain, j, i, t.ExcludedOUIDs) {
					out = append(out, Coverage{Policy: p, TargetID: t.ID})
					continue
				}
				// An explicit target never authorizes skipping a hop, so it only covers the
				// position immediately below the organization unit that named it.
				if j == i-1 && (t.Scope == ScopeChild || t.Scope == ScopeChildSubtree) &&
					t.OUID == ouID && !slices.Contains(t.ExcludedOUIDs, ouID) {
					out = append(out, Coverage{Policy: p, TargetID: t.ID})
				}
			}
		}
	}
	return dedupe(out)
}

// allOUsCoverage returns a deployment-wide policy reaching chain[i], if one exists.
func allOUsCoverage(chain []string, i int, policies []Policy) (Coverage, bool) {
	for _, p := range policies {
		for _, t := range p.Targets {
			if t.Scope == ScopeAllOUs && !excludedAnywhere(chain[:i+1], t.ExcludedOUIDs) {
				return Coverage{Policy: p, TargetID: t.ID, ByAllOUs: true}, true
			}
		}
	}
	return Coverage{}, false
}

// excludedBetween reports whether any organization unit from the anchor down to the target is
// carved out, which cuts off everything below it.
func excludedBetween(chain []string, from, to int, excluded []string) bool {
	return excludedAnywhere(chain[from+1:to+1], excluded)
}

// excludedAnywhere reports whether any of the organization units is carved out.
func excludedAnywhere(ouIDs, excluded []string) bool {
	for _, ouID := range ouIDs {
		if slices.Contains(excluded, ouID) {
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

// familyOf returns the family a scope belongs to.
func familyOf(s TargetScope) scopeFamily {
	switch s {
	case ScopeAllOUs, ScopeAllRoots, ScopeAllChildren:
		return familyBlanket
	default:
		return familySelective
	}
}

// policyFamily returns the family a policy belongs to. A policy may now hold targets of both
// families at once, which is how one organization unit is given terms of its own while a broad
// target covers everyone else. One blanket target is enough to make the policy blanket: the edit
// rules exist to stop a broad reach changing shape, and a policy that holds one has a broad reach
// whatever else it carries.
func policyFamily(p Policy) scopeFamily {
	for _, t := range p.Targets {
		if familyOf(t.Scope) == familyBlanket {
			return familyBlanket
		}
	}
	return familySelective
}

// validateEdit reports whether a proposed policy is a legal edit of the current one.
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
func validateEdit(current, proposed Policy, expectedVersion, actualVersion int) error {
	if expectedVersion != actualVersion {
		return errVersionMismatchEngine
	}

	currentFamily := policyFamily(current)
	if currentFamily != policyFamily(proposed) {
		return errScopeFamilyChange
	}

	// A blanket policy cannot change what family of organization units it reaches, because there is
	// nothing wider to grow into and a different family is a different policy.
	//
	// Its exclusions may be edited in both directions. Adding one carves a unit out; dropping one
	// hands the resource back to a unit the issuer had carved out itself, and that is the issuer's
	// own decision to reverse. What the scope family protects is other people's reach, not the
	// issuer's earlier opinion of its own.
	if currentFamily == familyBlanket && !sameBlanketTargets(current.Targets, proposed.Targets) {
		return errBlanketScopeNarrowOnly
	}
	return nil
}

// sameBlanketTargets reports whether two policies select the same thing with their blanket
// targets, ignoring order. Only the exclusions around them may change.
//
// Selective targets are left out of the comparison on purpose. They are bounded by the one-hop
// rule wherever they appear, so adding or dropping one creates no reach its initiator did not
// already hold, and a carve-out is edited by doing exactly that.
func sameBlanketTargets(current, proposed []Target) bool {
	blanket := func(targets []Target) []Target {
		out := make([]Target, 0, len(targets))
		for _, t := range targets {
			if familyOf(t.Scope) == familyBlanket {
				out = append(out, t)
			}
		}
		return out
	}
	c, p := blanket(current), blanket(proposed)
	if len(c) != len(p) {
		return false
	}
	for _, want := range c {
		found := false
		for _, got := range p {
			if want.Scope == got.Scope && want.OUID == got.OUID {
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

// orderByDependency sorts policies so a parent always precedes any policy derived from it.
//
// Replaying an export in this order keeps every step valid, because the policy that makes an
// initiator visible is always applied before the policy that initiator issued. Policies whose
// parent is absent from the set are treated as roots, which is what makes an export of one
// resource replayable on its own.
func orderByDependency(lineages []Lineage) []Lineage {
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

// requestFromPolicy rebuilds the request that recreates a policy, for export and replay. The
// resolved rules are exported rather than the requested ones, so replaying reaches the same
// result and the export is a fixed point. The id travels with it, because a declared policy owns
// its own and a replay without it is refused; an API create mints a fresh one and ignores it.
func requestFromPolicy(p Policy) PolicyRequest {
	targets := make([]TargetRequest, 0, len(p.Targets))
	for _, t := range p.Targets {
		// The scopes that name no organization unit carry one in storage anyway: allChildren
		// anchors on the initiator, which the request already states. Replaying it as a named
		// unit would be refused, so it is dropped here.
		ouID := t.OUID
		if t.Scope == ScopeAllOUs || t.Scope == ScopeAllRoots || t.Scope == ScopeAllChildren {
			ouID = ""
		}
		rules := p.TargetRules(t.ID)
		if len(rules) == 0 {
			// Absent rather than empty: a replay of the export has to produce the same policy, and
			// an empty map is not what a target carrying no terms was created from.
			rules = nil
		}
		targets = append(targets, TargetRequest{
			Scope:         t.Scope,
			OUID:          ouID,
			ExcludedOUIDs: slices.Clone(t.ExcludedOUIDs),
			OverlayRules:  rules,
		})
	}

	return PolicyRequest{
		ID:             p.ID,
		InitiatingOUID: p.InitiatingOUID,
		Targets:        targets,
		Version:        p.Version,
	}
}

// mapEngineError turns a narrowing failure into the service error naming the offending field.
func mapEngineError(err error, fieldKey string) *tidcommon.ServiceError {
	switch {
	case errors.Is(err, errWidens):
		return withDetail(ErrorRuleWidens, fieldKey)
	case errors.Is(err, errValueOutsideAllowed):
		return withDetail(ErrorValueOutsideAllowed, fieldKey)
	case errors.Is(err, errPinnedWithAllowed):
		return withDetail(ErrorPinnedWithAllowed, fieldKey)
	default:
		return &tidcommon.InternalServerError
	}
}

// mapEditError turns an edit-validation failure into its service error.
func mapEditError(err error) *tidcommon.ServiceError {
	switch {
	case errors.Is(err, errBlanketScopeNarrowOnly):
		return &ErrorBlanketNarrowOnly
	case errors.Is(err, errScopeFamilyChange):
		return &ErrorScopeFamilyChange
	case errors.Is(err, errVersionMismatchEngine):
		return &ErrorVersionMismatch
	default:
		return &tidcommon.InternalServerError
	}
}
