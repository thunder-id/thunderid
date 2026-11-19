// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"cmp"
	"maps"
	"slices"
)

// SignalCatalogInterface tells the compiler which signals exist and how to read them.
type SignalCatalogInterface interface {
	Lookup(name string) (SignalSpec, bool)
}

// tree is a condition being compiled: normalised, with the cost of the signals it reads.
type tree struct {
	pred      *Predicate
	path      string
	children  []*tree
	cost      CostClass
	leafCount int
}

// compiler turns a parsed decision into a plan. It lowers names to slots and literals to pre-read
// values, normalises and reorders conditions without changing their meaning, and generates closures.
type compiler struct {
	catalog     SignalCatalogInterface
	diagnostics []Diagnostic
	plan        *plan
	ruleSlots   map[int]bool
}

// newCompiler creates a compiler that resolves signals through catalog.
func newCompiler(catalog SignalCatalogInterface) *compiler {
	return &compiler{catalog: catalog, plan: &plan{slotIndex: map[string]int{}}}
}

// compile builds the plan, or returns nil when any diagnostic has severity ERROR.
func (c *compiler) compile(d *Decision) (*plan, []Diagnostic) {
	if d == nil {
		c.add("", CodeSyntax, map[string]string{paramExpected: typeObject})
		return nil, c.diagnostics
	}
	c.plan.defaultArm, c.plan.unknownArm = d.Default, d.Unknown
	for i := range d.Rules {
		c.rule(index(fieldRules, i), &d.Rules[i])
	}
	if hasErrors(c.diagnostics) {
		return nil, c.diagnostics
	}
	c.plan.stages = c.stages()
	return c.plan, c.diagnostics
}

// rule compiles one rule.
func (c *compiler) rule(path string, r *Rule) {
	whenPath := join(path, fieldWhen)
	if r.When == nil {
		c.add(whenPath, CodeMissingField, map[string]string{paramField: fieldWhen})
		return
	}
	c.ruleSlots = map[int]bool{}
	reported := len(c.diagnostics)
	t := c.normalise(r.When, whenPath)
	if len(c.diagnostics) > reported {
		return
	}
	c.measure(t)
	eval := c.generate(t)
	c.plan.rules = append(c.plan.rules, compiledRule{
		id: r.ID, nextNode: r.NextNode, eval: eval, slots: slices.Sorted(maps.Keys(c.ruleSlots)),
	})
}

// normalise flattens nested groups of the same kind and removes double negation. It reports a
// condition that is missing or has the wrong number of operands.
func (c *compiler) normalise(p *Predicate, path string) *tree {
	switch {
	case p == nil:
		c.add(path, CodeSyntax, map[string]string{paramExpected: typeObject})
		return &tree{pred: &Predicate{}, path: path}
	case p.Op == opNot && len(p.Of) != 1:
		c.add(join(path, fieldOf), CodeArity, map[string]string{paramName: opNot})
		return &tree{pred: p, path: path}
	case p.Op == opNot && len(p.Of) == 1:
		child := c.normalise(p.Of[0], join(path, fieldOf))
		if child.pred.Op == opNot && len(child.children) == 1 {
			return child.children[0]
		}
		return &tree{pred: p, path: path, children: []*tree{child}}
	case p.isGroup() && len(p.Of) < 2:
		c.add(join(path, fieldOf), CodeArity, map[string]string{paramName: p.Op})
		return &tree{pred: p, path: path}
	case p.isGroup():
		t := &tree{pred: p, path: path}
		for i, child := range p.Of {
			ct := c.normalise(child, index(join(path, fieldOf), i))
			if ct.pred.Op == p.Op {
				t.children = append(t.children, ct.children...)
			} else {
				t.children = append(t.children, ct)
			}
		}
		return t
	}
	return &tree{pred: p, path: path}
}

// measure sets the cost class and leaf count of every node, and orders each group's children cheapest
// first. Reordering inside a group never changes its result.
func (c *compiler) measure(t *tree) {
	if len(t.children) == 0 {
		t.leafCount = 1
		t.cost = c.costOf(t.pred.Signal)
		if t.pred.Ref != "" {
			t.cost = max(t.cost, c.costOf(t.pred.Ref))
		}
		return
	}
	for _, child := range t.children {
		c.measure(child)
		t.leafCount += child.leafCount
		t.cost = max(t.cost, child.cost)
	}
	if t.pred.isGroup() {
		slices.SortStableFunc(t.children, func(a, b *tree) int {
			return cmp.Or(cmp.Compare(a.cost, b.cost), cmp.Compare(a.leafCount, b.leafCount))
		})
	}
}

// costOf returns the cost class of a signal; an unknown signal is reported when its leaf is lowered.
func (c *compiler) costOf(name string) CostClass {
	spec, ok := c.catalog.Lookup(name)
	if !ok {
		return CostLocal
	}
	return spec.CostClass
}

// generate builds the closure for a normalised condition.
func (c *compiler) generate(t *tree) node {
	if t.pred.Op == opNot {
		return notNode(c.generate(t.children[0]))
	}
	if t.pred.isGroup() {
		children := make([]node, len(t.children))
		for i, child := range t.children {
			children[i] = c.generate(child)
		}
		if t.pred.Op == opAll {
			return allNode(children)
		}
		return anyNode(children)
	}
	l := c.lowerLeaf(t.path, t.pred)
	if l == nil {
		return nil
	}
	return l.evaluate
}

// lowerLeaf compiles one leaf condition and reports every problem with it.
func (c *compiler) lowerLeaf(path string, p *Predicate) *leaf {
	spec, ok := operatorByName(p.Op)
	if !ok || spec.kind == OperatorKindGroup || spec.kind == OperatorKindNegation {
		c.add(join(path, fieldOp), CodeUnknownOperator, map[string]string{paramName: p.Op})
		return nil
	}
	signal, ok := c.lookup(join(path, fieldSignal), p.Signal)
	if !ok {
		return nil
	}
	if !c.checkLeafShape(path, p, spec, signal) {
		return nil
	}
	l := &leaf{
		id: len(c.plan.leaves), op: p.Op, slot: c.slotOf(signal), refSlot: -1, every: p.Every, test: spec.evaluateAt,
		cmp: comparison{
			pinned: comparatorByName(signal.Comparator), scale: scaleRanks(signal.Scale),
			allowed: inferableWith(spec.need), need: spec.need, ci: p.CI,
		},
	}
	c.checkComparator(path, l, spec)
	if p.Ref != "" {
		ref, found := c.lookup(join(path, fieldRef), p.Ref)
		if !found {
			return nil
		}
		l.refSlot = c.slotOf(ref)
	}
	if p.Value != nil {
		c.lowerLiteral(join(path, fieldValue), l, p.Value)
	}
	c.plan.leaves = append(c.plan.leaves, leafInfo{path: path, signal: p.Signal, op: p.Op})
	return l
}

// lookup resolves a signal name through the catalog, and refuses a signal whose declared comparator
// is not registered.
func (c *compiler) lookup(path, name string) (SignalSpec, bool) {
	spec, ok := c.catalog.Lookup(name)
	spec.Name = name
	if !ok {
		c.add(path, CodeUnknownSignal, map[string]string{paramName: name})
		return spec, false
	}
	if spec.Comparator != "" && comparatorByName(spec.Comparator) == cmpNone {
		c.add(path, CodeUnknownComparator, map[string]string{paramName: string(spec.Comparator)})
		return spec, false
	}
	return spec, true
}

// checkLeafShape reports operands, qualifiers and options the operator does not accept, and ci on a
// signal that declares a scale, whose members are matched exactly.
func (c *compiler) checkLeafShape(path string, p *Predicate, spec *operatorSpec, signal SignalSpec) bool {
	ok := true
	hasValue, hasRef := p.Value != nil, p.Ref != ""
	operandParams := map[string]string{paramName: p.Op}
	switch {
	case spec.kind == OperatorKindPresence && (hasValue || hasRef):
		c.add(path, CodeOperand, operandParams)
		ok = false
	case spec.kind == OperatorKindComparison && hasValue == hasRef:
		c.add(path, CodeOperand, operandParams)
		ok = false
	case hasValue && len(p.Value.values) == 0:
		c.add(join(path, fieldValue), CodeOperand, operandParams)
		ok = false
	case hasValue && p.Value.IsList() && !spec.right.List:
		c.add(join(path, fieldValue), CodeOperand, operandParams)
		ok = false
	case hasRef && !spec.right.Ref:
		c.add(join(path, fieldRef), CodeOperand, operandParams)
		ok = false
	}
	if p.CI && !spec.ci {
		c.add(join(path, fieldCI), CodeQualifier, map[string]string{paramName: fieldCI})
		ok = false
	}
	if p.CI && spec.ci && signal.Comparator == ComparatorScale {
		c.add(join(path, fieldCI), CodeQualifier, map[string]string{paramName: fieldCI})
		ok = false
	}
	if p.Every && !spec.every {
		c.add(join(path, fieldEvery), CodeQualifier, map[string]string{paramName: fieldEvery})
		ok = false
	}
	if len(p.Options) > 0 {
		c.add(join(path, fieldOptions), CodeOptions, map[string]string{paramName: p.Op})
		ok = false
	}
	return ok
}

// checkComparator warns when the signal's declared comparator cannot do what the operator needs.
func (c *compiler) checkComparator(path string, l *leaf, spec *operatorSpec) {
	pinned := l.cmp.pinned
	if pinned == cmpNone {
		return
	}
	mismatch := spec.need != 0 && !pinned.offers(spec.need)
	if l.op == opOlderThan {
		mismatch = pinned != cmpInstant
	}
	if mismatch {
		c.add(path, CodeOperatorComparator, map[string]string{
			paramName: l.op, paramExpected: string(pinned.displayName()),
		})
	}
}

// lowerLiteral pre-reads the literal and warns about values the comparison can never read.
func (c *compiler) lowerLiteral(path string, l *leaf, lit *Literal) {
	l.right = newAtoms(lit.Atoms())
	if l.op == opOlderThan {
		l.period, l.periodOK = parseDuration(l.right[0].text)
		if !l.periodOK {
			c.add(path, CodeIncomparableLiteral, map[string]string{
				paramValue: l.right[0].text, paramExpected: string(ComparatorDuration),
			})
		}
		return
	}
	for i := range l.right {
		if !l.cmp.recognizes(&l.right[i]) {
			c.add(path, CodeIncomparableLiteral, map[string]string{
				paramValue: l.right[i].text, paramExpected: string(l.cmp.pinned.displayName()),
			})
			break
		}
	}
	if l.op == opEq && len(l.right) >= hashSetMinListSize && c.textOnly(l) {
		l.set = make(map[string]struct{}, len(l.right))
		for i := range l.right {
			l.set[l.cmp.textOf(&l.right[i])] = struct{}{}
		}
	}
}

// textOnly reports whether every comparison of the leaf is a text comparison, so the literal list can
// be looked up as a set.
func (c *compiler) textOnly(l *leaf) bool {
	switch l.cmp.pinned {
	case cmpText:
		return true
	case cmpNone:
		return !slices.ContainsFunc(l.right, func(a atom) bool { return a.classes != maskOf(cmpText) })
	}
	return false
}

// slotOf returns the slot of a signal, adding it on first use, and records it against the current rule.
func (c *compiler) slotOf(spec SignalSpec) int {
	i, ok := c.plan.slotIndex[spec.Name]
	if !ok {
		i = len(c.plan.signals)
		c.plan.slotIndex[spec.Name] = i
		c.plan.signals = append(c.plan.signals, spec)
	}
	c.ruleSlots[i] = true
	return i
}

// stages groups the plan's signals by cost class, cheapest first, leaving out empty classes.
func (c *compiler) stages() [][]int {
	byClass := map[CostClass][]int{}
	for i, s := range c.plan.signals {
		byClass[s.CostClass] = append(byClass[s.CostClass], i)
	}
	stages := make([][]int, 0, len(byClass))
	for _, class := range slices.Sorted(maps.Keys(byClass)) {
		stages = append(stages, byClass[class])
	}
	return stages
}

// add records a diagnostic.
func (c *compiler) add(path string, code DiagnosticCode, params map[string]string) {
	c.diagnostics = append(c.diagnostics, newDiagnostic(path, code, params))
}

// scaleRanks returns the position of each member of a declared scale. A repeated member keeps the
// position where it first appears.
func scaleRanks(scale []string) map[string]int {
	if len(scale) == 0 {
		return nil
	}
	ranks := make(map[string]int, len(scale))
	for i, member := range scale {
		if _, seen := ranks[member]; !seen {
			ranks[member] = i
		}
	}
	return ranks
}
