// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"bytes"
	"encoding/json"
	"io"
	"maps"
	"slices"
	"strconv"
	"unicode/utf8"
)

// parser builds a decision from its JSON form and collects every diagnostic with its path.
type parser struct {
	limits      limits
	diagnostics []Diagnostic
	leaves      int
	ruleIDs     map[string]bool
}

// newParser creates a parser that enforces the given limits.
func newParser(l limits) *parser {
	return &parser{limits: l, ruleIDs: map[string]bool{}}
}

// parse decodes raw and builds the decision. The decision is complete only when no diagnostic has
// severity ERROR.
func (p *parser) parse(raw []byte) (*Decision, []Diagnostic) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var root any
	if err := decoder.Decode(&root); err != nil || !atEOF(decoder) {
		p.add("", CodeSyntax, map[string]string{paramExpected: typeObject})
		return nil, p.diagnostics
	}
	return p.decision(root), p.diagnostics
}

// atEOF reports whether nothing but whitespace follows the decoded value. More alone misses a stray
// closing delimiter.
func atEOF(decoder *json.Decoder) bool {
	_, err := decoder.Token()
	return err == io.EOF
}

// decision builds the decision block from its decoded JSON value.
func (p *parser) decision(v any) *Decision {
	obj, ok := p.object("", v)
	if !ok {
		return nil
	}
	p.checkFields("", obj, fieldGrammarVersion, fieldHitPolicy, fieldRules, fieldDefault, fieldUnknown,
		fieldTests, fieldSource)

	d := &Decision{GrammarVersion: grammarVersionCurrent, HitPolicy: HitPolicyFirst}
	if version, present := p.integer("", obj, fieldGrammarVersion); present && version != grammarVersionCurrent {
		p.add(fieldGrammarVersion, CodeGrammarVersion, map[string]string{paramValue: strconv.Itoa(version)})
	}
	if policy, present := p.str("", obj, fieldHitPolicy); present && HitPolicy(policy) != HitPolicyFirst {
		p.add(fieldHitPolicy, CodeHitPolicy, map[string]string{paramValue: policy})
	}
	d.Rules = p.rules(obj[fieldRules])
	d.Default = p.arm(fieldDefault, obj[fieldDefault])
	d.Unknown = p.arm(fieldUnknown, obj[fieldUnknown])
	d.Tests = p.tests(obj)
	d.Source = p.source(obj)
	return d
}

// rules builds the rule list.
func (p *parser) rules(v any) []Rule {
	if v == nil {
		p.add(fieldRules, CodeNoRules, nil)
		return nil
	}
	items, ok := p.array(fieldRules, v)
	if !ok {
		return nil
	}
	if len(items) == 0 {
		p.add(fieldRules, CodeNoRules, nil)
		return nil
	}
	if len(items) > p.limits.maxRules {
		p.add(fieldRules, CodeLimit, map[string]string{paramLimit: strconv.Itoa(p.limits.maxRules)})
	}
	rules := make([]Rule, 0, len(items))
	for i, item := range items {
		rules = append(rules, p.rule(index(fieldRules, i), item))
	}
	return rules
}

// rule builds one rule.
func (p *parser) rule(path string, v any) Rule {
	var r Rule
	obj, ok := p.object(path, v)
	if !ok {
		return r
	}
	p.checkFields(path, obj, fieldID, fieldLabel, fieldWhen, fieldNextNode)
	r.ID = p.requiredString(path, obj, fieldID, CodeMissingField)
	p.checkRuleID(join(path, fieldID), r.ID)
	r.Label = p.label(path, obj)
	if when, present := obj[fieldWhen]; present {
		r.When = p.predicate(join(path, fieldWhen), when, 1)
	} else {
		p.add(join(path, fieldWhen), CodeMissingField, map[string]string{paramField: fieldWhen})
	}
	r.NextNode = p.nextNode(path, obj)
	return r
}

// checkRuleID reports a rule ID that has the wrong form or repeats an earlier rule's ID.
func (p *parser) checkRuleID(path, id string) {
	switch {
	case id == "":
		return
	case !isValidRuleID(id):
		p.add(path, CodeInvalidID, map[string]string{paramValue: id})
	case p.ruleIDs[id]:
		p.add(path, CodeDuplicateRuleID, map[string]string{paramValue: id})
	default:
		p.ruleIDs[id] = true
	}
}

// arm builds the default or unknown arm.
func (p *parser) arm(path string, v any) Arm {
	var a Arm
	if v == nil {
		p.add(path, CodeMissingArm, map[string]string{paramName: path})
		return a
	}
	obj, ok := p.object(path, v)
	if !ok {
		return a
	}
	p.checkFields(path, obj, fieldLabel, fieldNextNode)
	a.Label = p.label(path, obj)
	a.NextNode = p.nextNode(path, obj)
	return a
}

// label reads an optional label and checks its length.
func (p *parser) label(path string, obj map[string]any) string {
	label, _ := p.str(path, obj, fieldLabel)
	if utf8.RuneCountInString(label) > maxLabelLength {
		p.add(join(path, fieldLabel), CodeLimit, map[string]string{paramLimit: strconv.Itoa(maxLabelLength)})
	}
	return label
}

// nextNode reads a required nextNode.
func (p *parser) nextNode(path string, obj map[string]any) string {
	return p.requiredString(path, obj, fieldNextNode, CodeUnknownNode)
}

// predicate builds a condition at the given nesting depth.
func (p *parser) predicate(path string, v any, depth int) *Predicate {
	if depth > p.limits.maxDepth {
		p.add(path, CodeLimit, map[string]string{paramLimit: strconv.Itoa(p.limits.maxDepth)})
		return nil
	}
	obj, ok := p.object(path, v)
	if !ok {
		return nil
	}
	op := p.requiredString(path, obj, fieldOp, CodeMissingField)
	if op == "" {
		return nil
	}
	switch op {
	case opAll, opAny:
		return p.group(path, obj, op, depth)
	case opNot:
		return p.negation(path, obj, depth)
	}
	return p.leaf(path, obj, op)
}

// group builds an all or any group.
func (p *parser) group(path string, obj map[string]any, op string, depth int) *Predicate {
	p.checkFields(path, obj, fieldOp, fieldOf)
	ofPath := join(path, fieldOf)
	items, isList := obj[fieldOf].([]any)
	if !isList || len(items) < 2 {
		p.add(ofPath, CodeArity, map[string]string{paramName: op})
		if !isList {
			return nil
		}
	}
	pred := &Predicate{Op: op, Of: make([]*Predicate, 0, len(items))}
	for i, item := range items {
		if child := p.predicate(index(ofPath, i), item, depth+1); child != nil {
			pred.Of = append(pred.Of, child)
		}
	}
	return pred
}

// negation builds a not.
func (p *parser) negation(path string, obj map[string]any, depth int) *Predicate {
	p.checkFields(path, obj, fieldOp, fieldOf)
	ofPath := join(path, fieldOf)
	child, isObject := obj[fieldOf].(map[string]any)
	if !isObject {
		p.add(ofPath, CodeArity, map[string]string{paramName: opNot})
		return nil
	}
	inner := p.predicate(ofPath, child, depth+1)
	if inner == nil {
		return nil
	}
	return &Predicate{Op: opNot, Of: []*Predicate{inner}}
}

// leaf builds a leaf condition. Whether the operator exists, and which operands it accepts, is
// checked when the decision is compiled.
func (p *parser) leaf(path string, obj map[string]any, op string) *Predicate {
	p.checkFields(path, obj, fieldOp, fieldSignal, fieldValue, fieldRef, fieldCI, fieldEvery, fieldOptions)
	p.leaves++
	if p.leaves == p.limits.maxLeaves+1 {
		p.add(path, CodeLimit, map[string]string{paramLimit: strconv.Itoa(p.limits.maxLeaves)})
	}
	pred := &Predicate{Op: op}
	pred.Signal = p.requiredString(path, obj, fieldSignal, CodeMissingField)
	if raw, present := obj[fieldValue]; present {
		pred.Value = p.literal(join(path, fieldValue), raw)
	}
	pred.Ref, _ = p.str(path, obj, fieldRef)
	pred.CI = p.boolean(path, obj, fieldCI)
	pred.Every = p.boolean(path, obj, fieldEvery)
	if raw, present := obj[fieldOptions]; present {
		pred.Options, _ = p.object(join(path, fieldOptions), raw)
	}
	return pred
}

// literal builds a literal: one string, number or boolean, or a non-empty list of them.
func (p *parser) literal(path string, v any) *Literal {
	if items, isList := v.([]any); isList {
		if len(items) == 0 {
			p.add(path, CodeSyntax, map[string]string{paramExpected: typeLiteral})
			return nil
		}
		if len(items) > maxListSizeCap {
			p.add(path, CodeLimit, map[string]string{paramLimit: strconv.Itoa(maxListSizeCap)})
			return nil
		}
		lit := &Literal{list: true, values: make([]literalValue, 0, len(items))}
		for i, item := range items {
			if value, ok := p.literalValue(index(path, i), item); ok {
				lit.values = append(lit.values, value)
			}
		}
		return lit
	}
	value, ok := p.literalValue(path, v)
	if !ok {
		return nil
	}
	return &Literal{values: []literalValue{value}}
}

// literalValue builds one scalar value of a literal.
func (p *parser) literalValue(path string, v any) (literalValue, bool) {
	switch value := v.(type) {
	case string:
		return literalValue{text: value, kind: literalString}, true
	case bool:
		return literalValue{text: strconv.FormatBool(value), kind: literalBool}, true
	case json.Number:
		if n, ok := parseDecimal(value.String()); ok {
			return literalValue{text: n.String(), kind: literalNumber}, true
		}
	}
	p.add(path, CodeSyntax, map[string]string{paramExpected: typeLiteral})
	return literalValue{}, false
}

// tests builds the decision's test scenarios.
func (p *parser) tests(obj map[string]any) []DecisionTest {
	raw, present := obj[fieldTests]
	if !present {
		return nil
	}
	items, ok := p.array(fieldTests, raw)
	if !ok {
		return nil
	}
	if len(items) > p.limits.maxTests {
		p.add(fieldTests, CodeLimit, map[string]string{paramLimit: strconv.Itoa(p.limits.maxTests)})
	}
	tests := make([]DecisionTest, 0, len(items))
	for i, item := range items {
		tests = append(tests, p.test(index(fieldTests, i), item))
	}
	return tests
}

// test builds one test scenario.
func (p *parser) test(path string, v any) DecisionTest {
	var t DecisionTest
	obj, ok := p.object(path, v)
	if !ok {
		return t
	}
	p.checkFields(path, obj, fieldName, fieldContext, fieldFacts, fieldUnresolved, fieldExpect)
	t.Name = p.requiredString(path, obj, fieldName, CodeMissingField)
	t.Expect = p.requiredString(path, obj, fieldExpect, CodeMissingField)
	if raw, present := obj[fieldContext]; present {
		t.Context = p.testContext(join(path, fieldContext), raw)
	}
	if raw, present := obj[fieldFacts]; present {
		t.Facts = p.facts(join(path, fieldFacts), raw)
	}
	if raw, present := obj[fieldUnresolved]; present {
		t.Unresolved = p.stringList(join(path, fieldUnresolved), raw)
	}
	return t
}

// testContext builds the context of a test scenario.
func (p *parser) testContext(path string, v any) *TestContext {
	obj, ok := p.object(path, v)
	if !ok {
		return nil
	}
	p.checkFields(path, obj, fieldFlowType, fieldApplicationID, fieldOUID)
	c := &TestContext{}
	c.FlowType, _ = p.str(path, obj, fieldFlowType)
	c.ApplicationID, _ = p.str(path, obj, fieldApplicationID)
	c.OUID, _ = p.str(path, obj, fieldOUID)
	return c
}

// facts builds the signal values of a test scenario.
func (p *parser) facts(path string, v any) map[string]Literal {
	obj, ok := p.object(path, v)
	if !ok {
		return nil
	}
	facts := make(map[string]Literal, len(obj))
	for _, name := range slices.Sorted(maps.Keys(obj)) {
		if lit := p.literal(join(path, name), obj[name]); lit != nil {
			facts[name] = *lit
		}
	}
	return facts
}

// stringList reads an array of strings.
func (p *parser) stringList(path string, v any) []string {
	items, ok := p.array(path, v)
	if !ok {
		return nil
	}
	list := make([]string, 0, len(items))
	for i, item := range items {
		s, isString := item.(string)
		if !isString {
			p.add(index(path, i), CodeSyntax, map[string]string{paramExpected: typeString})
			continue
		}
		list = append(list, s)
	}
	return list
}

// source builds the optional record of the authoring front end.
func (p *parser) source(obj map[string]any) *DecisionSource {
	raw, present := obj[fieldSource]
	if !present {
		return nil
	}
	src, ok := p.object(fieldSource, raw)
	if !ok {
		return nil
	}
	p.checkFields(fieldSource, src, fieldFrontend, fieldFrontendVersion)
	s := &DecisionSource{}
	s.Frontend, _ = p.str(fieldSource, src, fieldFrontend)
	s.FrontendVersion, _ = p.integer(fieldSource, src, fieldFrontendVersion)
	return s
}

// object checks that v is a JSON object.
func (p *parser) object(path string, v any) (map[string]any, bool) {
	obj, ok := v.(map[string]any)
	if !ok {
		p.add(path, CodeSyntax, map[string]string{paramExpected: typeObject})
	}
	return obj, ok
}

// array checks that v is a JSON array.
func (p *parser) array(path string, v any) ([]any, bool) {
	items, ok := v.([]any)
	if !ok {
		p.add(path, CodeSyntax, map[string]string{paramExpected: typeArray})
	}
	return items, ok
}

// checkFields reports every field of obj that is not in allowed.
func (p *parser) checkFields(path string, obj map[string]any, allowed ...string) {
	for _, name := range slices.Sorted(maps.Keys(obj)) {
		if !slices.Contains(allowed, name) {
			p.add(join(path, name), CodeUnknownField, map[string]string{paramField: name})
		}
	}
}

// str reads an optional string field and reports whether it is present.
func (p *parser) str(path string, obj map[string]any, field string) (string, bool) {
	raw, present := obj[field]
	if !present {
		return "", false
	}
	s, ok := raw.(string)
	if !ok {
		p.add(join(path, field), CodeSyntax, map[string]string{paramExpected: typeString})
	}
	return s, ok
}

// requiredString reads a string field that must be present and non-empty, reporting code when it is
// absent or empty.
func (p *parser) requiredString(path string, obj map[string]any, field string, code DiagnosticCode) string {
	raw, present := obj[field]
	s, isString := raw.(string)
	if present && !isString {
		p.add(join(path, field), CodeSyntax, map[string]string{paramExpected: typeString})
		return ""
	}
	if s == "" {
		p.add(join(path, field), code, map[string]string{paramField: field})
	}
	return s
}

// boolean reads an optional boolean field.
func (p *parser) boolean(path string, obj map[string]any, field string) bool {
	raw, present := obj[field]
	if !present {
		return false
	}
	b, ok := raw.(bool)
	if !ok {
		p.add(join(path, field), CodeSyntax, map[string]string{paramExpected: typeBoolean})
	}
	return b
}

// integer reads an optional integer field and reports whether it is present.
func (p *parser) integer(path string, obj map[string]any, field string) (int, bool) {
	raw, present := obj[field]
	if !present {
		return 0, false
	}
	if n, ok := raw.(json.Number); ok {
		if i, err := strconv.Atoi(n.String()); err == nil {
			return i, true
		}
	}
	p.add(join(path, field), CodeSyntax, map[string]string{paramExpected: typeInteger})
	return 0, false
}

// add records a diagnostic.
func (p *parser) add(path string, code DiagnosticCode, params map[string]string) {
	p.diagnostics = append(p.diagnostics, newDiagnostic(path, code, params))
}

// newDiagnostic builds a diagnostic whose severity follows from its code.
func newDiagnostic(path string, code DiagnosticCode, params map[string]string) Diagnostic {
	severity := SeverityError
	if len(code) > 0 && code[0] == 'W' {
		severity = SeverityWarning
	}
	return Diagnostic{Path: path, Code: code, Severity: severity, Params: params}
}

// join appends a field name to a path.
func join(path, field string) string {
	if path == "" {
		return field
	}
	return path + "." + field
}

// index appends an array index to a path.
func index(path string, i int) string {
	return path + "[" + strconv.Itoa(i) + "]"
}

// isValidRuleID reports whether id is a lowercase letter followed by lowercase letters, digits or
// underscores, at most 64 characters long.
func isValidRuleID(id string) bool {
	if id == "" || len(id) > maxRuleIDLength || id[0] < 'a' || id[0] > 'z' {
		return false
	}
	for i := 1; i < len(id); i++ {
		c := id[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '_' {
			return false
		}
	}
	return true
}

// hasErrors reports whether any diagnostic has severity ERROR.
func hasErrors(diagnostics []Diagnostic) bool {
	return slices.ContainsFunc(diagnostics, func(d Diagnostic) bool { return d.Severity == SeverityError })
}
