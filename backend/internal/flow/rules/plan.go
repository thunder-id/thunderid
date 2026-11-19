// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"sync"
	"time"
)

// PlanInterface is a compiled decision, immutable and safe to share between concurrent evaluations.
type PlanInterface interface {
	Stages() [][]string
	AcquireFrame(now time.Time) FrameInterface
}

// compiledRule is one compiled rule.
type compiledRule struct {
	id       string
	nextNode string
	eval     node
	slots    []int
}

// leafInfo is the static description of a compiled leaf, used to report its result.
type leafInfo struct {
	path   string
	signal string
	op     string
}

// plan is a compiled decision: rules as closures over a frame, the signals they read, and the order in
// which to fetch those signals.
type plan struct {
	rules      []compiledRule
	defaultArm Arm
	unknownArm Arm
	signals    []SignalSpec
	slotIndex  map[string]int
	stages     [][]int
	leaves     []leafInfo
	pool       sync.Pool
}

// Stages returns the signals the plan reads, grouped by cost class, cheapest first.
func (p *plan) Stages() [][]string {
	stages := make([][]string, len(p.stages))
	for i, stage := range p.stages {
		names := make([]string, len(stage))
		for j, s := range stage {
			names[j] = p.signals[s].Name
		}
		stages[i] = names
	}
	return stages
}

// AcquireFrame returns a frame for one evaluation at the given time, with no signal fetched.
func (p *plan) AcquireFrame(now time.Time) FrameInterface {
	f, ok := p.pool.Get().(*frame)
	if !ok {
		f = p.newFrame()
	}
	f.now = now
	f.resetLeaves()
	return f
}

// newFrame creates a frame sized for the plan.
func (p *plan) newFrame() *frame {
	return &frame{
		plan:       p,
		slots:      make([]slot, len(p.signals)),
		leaves:     make([]leafResult, len(p.leaves)),
		ruleTruths: make([]Truth, len(p.rules)),
	}
}

// evaluate applies the FIRST hit policy: the first rule that is not false decides. A true rule
// selects its node; an unknown rule selects the unknown arm and blocks the rules after it; when every
// rule is false, the default arm is selected. The outcome is settled when fetching the signals not
// fetched yet cannot change it.
func (p *plan) evaluate(f *frame) Outcome {
	f.resetLeaves()
	for i := range p.rules {
		r := &p.rules[i]
		t := r.eval(f)
		f.ruleTruths[i] = t
		f.evaluated = i + 1
		switch t {
		case TruthTrue:
			return Outcome{SelectedBy: SelectedByRule, RuleID: r.id, NextNode: r.nextNode, Settled: true}
		case TruthUnknown:
			return Outcome{
				SelectedBy: SelectedByUnknown, RuleID: r.id, NextNode: p.unknownArm.NextNode,
				Settled: !f.pending(r.slots),
			}
		}
	}
	return Outcome{SelectedBy: SelectedByDefault, NextNode: p.defaultArm.NextNode, Settled: true}
}
