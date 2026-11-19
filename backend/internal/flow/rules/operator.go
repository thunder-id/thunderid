// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package rules

// operatorSpec is one registered operator: what it accepts and what it needs to compare values.
type operatorSpec struct {
	name       string
	since      int
	kind       OperatorKind
	right      RightSpec
	ci         bool
	every      bool
	need       ability
	phrases    Phrases
	evaluateAt pairTest
}

// pairTest decides one left value against one right value.
type pairTest func(l *leaf, a, b *atom) (Truth, comparatorID)

// operators lists the registered operators in the order they are described to clients.
var operators = []operatorSpec{
	{
		name: opEq, since: 1, kind: OperatorKindComparison, right: RightSpec{Value: true, List: true, Ref: true},
		ci: true, every: true, need: abilityEquality, phrases: Phrases{Positive: "is", Negative: "is not"},
		evaluateAt: testEqual,
	},
	{
		name: opGt, since: 1, kind: OperatorKindComparison, right: RightSpec{Value: true, Ref: true},
		every: true, need: abilityOrdering,
		phrases:    Phrases{Positive: "is greater than", Negative: "is not greater than"},
		evaluateAt: testGreater,
	},
	{
		name: opGte, since: 1, kind: OperatorKindComparison, right: RightSpec{Value: true, Ref: true},
		every: true, need: abilityOrdering, phrases: Phrases{Positive: "is at least", Negative: "is less than"},
		evaluateAt: testGreaterOrEqual,
	},
	{
		name: opStartsWith, since: 1, kind: OperatorKindComparison, ci: true, every: true, need: abilityText,
		right:      RightSpec{Value: true, List: true, Ref: true},
		phrases:    Phrases{Positive: "starts with", Negative: "does not start with"},
		evaluateAt: testText,
	},
	{
		name: opEndsWith, since: 1, kind: OperatorKindComparison, right: RightSpec{Value: true, List: true, Ref: true},
		ci: true, every: true, need: abilityText,
		phrases:    Phrases{Positive: "ends with", Negative: "does not end with"},
		evaluateAt: testText,
	},
	{
		name: opContains, since: 1, kind: OperatorKindComparison, right: RightSpec{Value: true, List: true, Ref: true},
		ci: true, every: true, need: abilityText,
		phrases:    Phrases{Positive: "contains", Negative: "does not contain"},
		evaluateAt: testText,
	},
	{
		name: opOlderThan, since: 1, kind: OperatorKindComparison, right: RightSpec{Value: true},
		every: true, phrases: Phrases{Positive: "is older than", Negative: "is not older than"},
	},
	{
		name: opExists, since: 1, kind: OperatorKindPresence,
		phrases: Phrases{Positive: "is present", Negative: "is not present"},
	},
	{
		name: opIsEmpty, since: 1, kind: OperatorKindPresence,
		phrases: Phrases{Positive: "is empty", Negative: "is not empty"},
	},
	{name: opAll, since: 1, kind: OperatorKindGroup, phrases: Phrases{Positive: "all of", Negative: "not all of"}},
	{name: opAny, since: 1, kind: OperatorKindGroup, phrases: Phrases{Positive: "any of", Negative: "none of"}},
	{name: opNot, since: 1, kind: OperatorKindNegation, phrases: Phrases{Positive: "not", Negative: "not"}},
}

// operatorByName returns the registered operator with the given name.
func operatorByName(name string) (*operatorSpec, bool) {
	for i := range operators {
		if operators[i].name == name {
			return &operators[i], true
		}
	}
	return nil, false
}

// descriptor describes the operator to clients.
func (o *operatorSpec) descriptor() OperatorDescriptor {
	d := OperatorDescriptor{Name: o.name, Since: o.since, Kind: o.kind, Qualifiers: []string{}, Phrases: o.phrases}
	if o.kind == OperatorKindComparison {
		right := o.right
		d.Right = &right
	}
	if o.ci {
		d.Qualifiers = append(d.Qualifiers, fieldCI)
	}
	if o.every {
		d.Qualifiers = append(d.Qualifiers, fieldEvery)
	}
	return d
}

// testEqual decides eq for one pair.
func testEqual(l *leaf, a, b *atom) (Truth, comparatorID) {
	c := l.cmp.pick(a, b)
	if c == cmpNone {
		return TruthUnknown, cmpNone
	}
	return truthOf(l.cmp.equal(c, a, b)), c
}

// testGreater decides gt for one pair.
func testGreater(l *leaf, a, b *atom) (Truth, comparatorID) {
	c := l.cmp.pick(a, b)
	if c == cmpNone {
		return TruthUnknown, cmpNone
	}
	return truthOf(l.cmp.order(c, a, b) > 0), c
}

// testGreaterOrEqual decides gte for one pair.
func testGreaterOrEqual(l *leaf, a, b *atom) (Truth, comparatorID) {
	c := l.cmp.pick(a, b)
	if c == cmpNone {
		return TruthUnknown, cmpNone
	}
	return truthOf(l.cmp.order(c, a, b) >= 0), c
}

// testText decides startsWith, endsWith and contains for one pair. Only text can be matched this way.
func testText(l *leaf, a, b *atom) (Truth, comparatorID) {
	if l.cmp.pick(a, b) != cmpText {
		return TruthUnknown, cmpNone
	}
	return truthOf(matchText(l.op, l.cmp.textOf(a), l.cmp.textOf(b))), cmpText
}

// truthOf converts a boolean to a truth value.
func truthOf(b bool) Truth {
	if b {
		return TruthTrue
	}
	return TruthFalse
}
