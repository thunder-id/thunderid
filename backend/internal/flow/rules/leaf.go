// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package rules

import "time"

// leaf is a compiled leaf condition: its signal's slot, its pre-read right-hand values, and how it
// compares them.
type leaf struct {
	id       int
	op       string
	slot     int
	refSlot  int
	right    []atom
	set      map[string]struct{}
	period   isoDuration
	periodOK bool
	every    bool
	cmp      comparison
	test     pairTest
}

// evaluate decides the leaf and records its result in the frame.
func (l *leaf) evaluate(f *frame) Truth {
	t, reason, c := l.decide(f)
	f.leaves[l.id] = leafResult{truth: t, reason: reason, comparator: c}
	return t
}

// decide returns the leaf's truth, the reason for it, and the comparator that decided it.
func (l *leaf) decide(f *frame) (Truth, Reason, comparatorID) {
	s := &f.slots[l.slot]
	switch l.op {
	case opExists:
		return existsTruth(s)
	case opIsEmpty:
		return emptyTruth(s)
	}
	if reason, blocked := gate(s); blocked {
		return TruthUnknown, reason, cmpNone
	}
	right := l.right
	if l.refSlot >= 0 {
		ref := &f.slots[l.refSlot]
		if reason, blocked := gate(ref); blocked {
			return TruthUnknown, reason, cmpNone
		}
		right = ref.atoms
	}
	return l.quantify(s.atoms, right, f.now)
}

// quantify combines the results of every left value: true when some value matches, or, with every,
// when all values match. Every over no values is unknown.
func (l *leaf) quantify(left, right []atom, now time.Time) (Truth, Reason, comparatorID) {
	if l.every && len(left) == 0 {
		return TruthUnknown, ReasonAbsent, cmpNone
	}
	var threshold time.Time
	if l.op == opOlderThan && l.periodOK {
		threshold = l.period.before(now)
	}
	result := TruthFalse
	if l.every {
		result = TruthTrue
	}
	used := cmpNone
	for i := range left {
		t, c := l.atomTruth(&left[i], right, threshold)
		if c != cmpNone {
			used = c
		}
		if l.every {
			result = andTruth(result, t)
			if result == TruthFalse {
				break
			}
		} else {
			result = orTruth(result, t)
			if result == TruthTrue {
				break
			}
		}
	}
	switch result {
	case TruthTrue:
		return result, ReasonMatch, used
	case TruthFalse:
		return result, ReasonNoMatch, used
	}
	return result, ReasonIncomparable, used
}

// atomTruth decides one left value against the right-hand values: true when it matches any of them.
func (l *leaf) atomTruth(a *atom, right []atom, threshold time.Time) (Truth, comparatorID) {
	if l.op == opOlderThan {
		if !l.periodOK || !l.readsInstant(a) {
			return TruthUnknown, cmpNone
		}
		return truthOf(a.instant.Before(threshold)), cmpInstant
	}
	if l.set != nil {
		_, ok := l.set[l.cmp.textOf(a)]
		return truthOf(ok), cmpText
	}
	result := TruthFalse
	used := cmpNone
	for j := range right {
		t, c := l.test(l, a, &right[j])
		if c != cmpNone {
			used = c
		}
		result = orTruth(result, t)
		if result == TruthTrue {
			break
		}
	}
	return result, used
}

// readsInstant reports whether the value can be read as an instant under the leaf's comparison.
func (l *leaf) readsInstant(a *atom) bool {
	if l.cmp.pinned != cmpNone && l.cmp.pinned != cmpInstant {
		return false
	}
	return a.classes&maskOf(cmpInstant) != 0
}

// gate reports whether a slot holds no values to compare, and the reason the leaf is then unknown.
func gate(s *slot) (Reason, bool) {
	switch s.state {
	case slotUnfetched:
		return ReasonNotEvaluated, true
	case slotAbsent:
		return ReasonAbsent, true
	case slotUnresolved:
		return ReasonUnresolved, true
	}
	return "", false
}

// existsTruth decides exists: true when the signal resolved, false when it has no value, and unknown
// when its source failed or it was not fetched yet.
func existsTruth(s *slot) (Truth, Reason, comparatorID) {
	switch s.state {
	case slotResolved:
		return TruthTrue, ReasonMatch, cmpNone
	case slotAbsent:
		return TruthFalse, ReasonNoMatch, cmpNone
	}
	reason, _ := gate(s)
	return TruthUnknown, reason, cmpNone
}

// emptyTruth decides isEmpty: true when the signal resolved with no non-whitespace value.
func emptyTruth(s *slot) (Truth, Reason, comparatorID) {
	if reason, blocked := gate(s); blocked {
		return TruthUnknown, reason, cmpNone
	}
	for i := range s.atoms {
		if !s.atoms[i].isBlank() {
			return TruthFalse, ReasonNoMatch, cmpNone
		}
	}
	return TruthTrue, ReasonMatch, cmpNone
}

// node is a compiled condition.
type node func(f *frame) Truth

// allNode compiles all: false as soon as a child is false, otherwise unknown if any child is unknown.
func allNode(children []node) node {
	return func(f *frame) Truth {
		result := TruthTrue
		for _, child := range children {
			result = andTruth(result, child(f))
			if result == TruthFalse {
				return result
			}
		}
		return result
	}
}

// anyNode compiles any: true as soon as a child is true, otherwise unknown if any child is unknown.
func anyNode(children []node) node {
	return func(f *frame) Truth {
		result := TruthFalse
		for _, child := range children {
			result = orTruth(result, child(f))
			if result == TruthTrue {
				return result
			}
		}
		return result
	}
}

// notNode compiles not.
func notNode(child node) node {
	return func(f *frame) Truth {
		return notTruth(child(f))
	}
}
