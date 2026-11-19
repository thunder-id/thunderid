// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package rules

import "encoding/json"

// EngineInterface parses, compiles and describes the rules of DECISION nodes.
type EngineInterface interface {
	Parse(raw []byte) (*Decision, []Diagnostic)
	Canonical(d *Decision) ([]byte, error)
	Compile(d *Decision, catalog SignalCatalogInterface) (PlanInterface, []Diagnostic)
	Operators() []OperatorDescriptor
	Comparators() []ComparatorDescriptor
}

// limits bounds the size of a decision block.
type limits struct {
	maxRules  int
	maxDepth  int
	maxLeaves int
	maxTests  int
}

// defaultLimits returns the default limits.
func defaultLimits() limits {
	return limits{maxRules: defaultMaxRules, maxDepth: defaultMaxDepth, maxLeaves: defaultMaxLeaves,
		maxTests: defaultMaxTests}
}

// clamp keeps each limit between one and its hard cap.
func (l limits) clamp() limits {
	return limits{
		maxRules:  min(max(l.maxRules, 1), hardCapMaxRules),
		maxDepth:  min(max(l.maxDepth, 1), hardCapMaxDepth),
		maxLeaves: min(max(l.maxLeaves, 1), hardCapMaxLeaves),
		maxTests:  min(max(l.maxTests, 0), hardCapMaxTests),
	}
}

// ruleEngine implements EngineInterface.
type ruleEngine struct {
	limits limits
}

// newRuleEngine creates a rule engine that enforces the given limits, kept within their hard caps.
func newRuleEngine(l limits) *ruleEngine {
	return &ruleEngine{limits: l.clamp()}
}

// Parse reads a decision block from its JSON form and reports every problem with its path. It returns
// no decision when any diagnostic has severity ERROR.
func (e *ruleEngine) Parse(raw []byte) (*Decision, []Diagnostic) {
	d, diagnostics := newParser(e.limits).parse(raw)
	if hasErrors(diagnostics) {
		return nil, diagnostics
	}
	return d, diagnostics
}

// Canonical encodes a decision in its canonical JSON form, the form that is stored.
func (e *ruleEngine) Canonical(d *Decision) ([]byte, error) {
	return json.Marshal(d)
}

// Compile turns a parsed decision into a plan. It returns no plan when any diagnostic has severity
// ERROR.
func (e *ruleEngine) Compile(d *Decision, catalog SignalCatalogInterface) (PlanInterface, []Diagnostic) {
	p, diagnostics := newCompiler(catalog).compile(d)
	if p == nil {
		return nil, diagnostics
	}
	return p, diagnostics
}

// Operators describes every registered operator, in a stable order.
func (e *ruleEngine) Operators() []OperatorDescriptor {
	descriptors := make([]OperatorDescriptor, len(operators))
	for i := range operators {
		descriptors[i] = operators[i].descriptor()
	}
	return descriptors
}

// Comparators describes every comparator, with the inferred ones numbered in the order they are tried.
func (e *ruleEngine) Comparators() []ComparatorDescriptor {
	descriptors := make([]ComparatorDescriptor, 0, len(comparators))
	order := 0
	for c, info := range comparators {
		d := ComparatorDescriptor{
			Name: info.name, Inferred: info.inferred,
			Ordering:  comparatorID(c).offers(abilityOrdering),
			TextMatch: comparatorID(c).offers(abilityText),
		}
		if info.inferred {
			order++
			d.Order = order
		}
		descriptors = append(descriptors, d)
	}
	return descriptors
}
