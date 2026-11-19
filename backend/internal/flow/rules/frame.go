// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package rules

import "time"

// slotState is what is known about a signal's value in an evaluation.
type slotState uint8

// Slot states.
const (
	// slotUnfetched is a signal whose value has not been fetched yet.
	slotUnfetched slotState = iota
	// slotResolved is a signal with values.
	slotResolved
	// slotAbsent is a signal whose source answered with no value.
	slotAbsent
	// slotUnresolved is a signal whose source failed.
	slotUnresolved
)

// slot holds one signal's values for an evaluation.
type slot struct {
	state slotState
	atoms []atom
}

// leafResult is the outcome of one leaf in an evaluation.
type leafResult struct {
	truth      Truth
	reason     Reason
	comparator comparatorID
}

// FrameInterface holds the signal values of one evaluation of a plan.
type FrameInterface interface {
	SetValues(signal string, values []string)
	SetAbsent(signal string)
	SetUnresolved(signal string)
	Evaluate() Outcome
	Leaves() []LeafRecord
	RuleTruths() []Truth
	Release()
}

// frame is the working state of one evaluation. It is sized from its plan and reused through the
// plan's pool, so evaluating it allocates nothing.
type frame struct {
	plan       *plan
	now        time.Time
	slots      []slot
	leaves     []leafResult
	ruleTruths []Truth
	evaluated  int
}

// SetValues records the values of a signal. A signal the plan does not use is ignored.
func (f *frame) SetValues(signal string, values []string) {
	s := f.slotOf(signal)
	if s == nil {
		return
	}
	s.state = slotResolved
	s.atoms = s.atoms[:0]
	for _, v := range values {
		s.atoms = append(s.atoms, newAtom(v))
	}
}

// SetAbsent records that a signal's source answered with no value.
func (f *frame) SetAbsent(signal string) {
	f.setState(signal, slotAbsent)
}

// SetUnresolved records that a signal's source failed.
func (f *frame) SetUnresolved(signal string) {
	f.setState(signal, slotUnresolved)
}

// Evaluate evaluates the plan with the values recorded so far. Signals not recorded yet are unknown.
func (f *frame) Evaluate() Outcome {
	return f.plan.evaluate(f)
}

// Leaves returns the result of every leaf in the last evaluation.
func (f *frame) Leaves() []LeafRecord {
	records := make([]LeafRecord, len(f.leaves))
	for i, r := range f.leaves {
		info := f.plan.leaves[i]
		records[i] = LeafRecord{
			Path: info.path, Signal: info.signal, Operator: info.op,
			Truth: r.truth, Reason: r.reason, Comparator: r.comparator.displayName(),
		}
	}
	return records
}

// RuleTruths returns the truth of each rule the last evaluation reached, in author order.
func (f *frame) RuleTruths() []Truth {
	return append([]Truth(nil), f.ruleTruths[:f.evaluated]...)
}

// Release returns the frame to its plan's pool. The frame must not be used afterwards.
func (f *frame) Release() {
	for i := range f.slots {
		f.slots[i].state = slotUnfetched
		f.slots[i].atoms = f.slots[i].atoms[:0]
	}
	f.evaluated = 0
	f.plan.pool.Put(f)
}

// slotOf returns the slot of a signal, or nil when the plan does not use it.
func (f *frame) slotOf(signal string) *slot {
	i, ok := f.plan.slotIndex[signal]
	if !ok {
		return nil
	}
	return &f.slots[i]
}

// setState records a state with no values for a signal.
func (f *frame) setState(signal string, state slotState) {
	if s := f.slotOf(signal); s != nil {
		s.state = state
		s.atoms = s.atoms[:0]
	}
}

// pending reports whether any of the slots has not been fetched yet.
func (f *frame) pending(slots []int) bool {
	for _, i := range slots {
		if f.slots[i].state == slotUnfetched {
			return true
		}
	}
	return false
}

// resetLeaves marks every leaf as not evaluated.
func (f *frame) resetLeaves() {
	for i := range f.leaves {
		f.leaves[i] = leafResult{truth: TruthUnknown, reason: ReasonNotEvaluated, comparator: cmpNone}
	}
	f.evaluated = 0
}
