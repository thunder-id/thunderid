// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

// namelessCatalog is a signal catalog whose specs leave Name empty.
type namelessCatalog map[string]SignalSpec

// Lookup returns the signal with the given name, without setting its Name.
func (m namelessCatalog) Lookup(name string) (SignalSpec, bool) {
	spec, ok := m[name]
	return spec, ok
}

type CompilerTestSuite struct {
	suite.Suite
}

func TestCompilerTestSuite(t *testing.T) {
	suite.Run(t, new(CompilerTestSuite))
}

// compileWhen parses and compiles a single-rule decision with the given condition.
func (s *CompilerTestSuite) compileWhen(when string) (PlanInterface, []Diagnostic) {
	e := Initialize()
	d, diagnostics := e.Parse([]byte(minimal(when)))
	s.Require().Empty(diagnostics)
	return e.Compile(d, testCatalog())
}

func (s *CompilerTestSuite) TestReportsErrors() {
	cases := []struct {
		name string
		when string
		path string
		code DiagnosticCode
	}{
		{"unknown operator", `{"op":"matches","signal":"input.email","value":"x"}`, "rules[0].when.op",
			CodeUnknownOperator},
		{"unknown signal", `{"op":"eq","signal":"user.group","value":"x"}`, "rules[0].when.signal",
			CodeUnknownSignal},
		{"unknown ref", `{"op":"eq","signal":"input.email","ref":"user.mail"}`, "rules[0].when.ref",
			CodeUnknownSignal},
		{"comparison without operand", `{"op":"eq","signal":"input.email"}`, "rules[0].when", CodeOperand},
		{"comparison with both operands", `{"op":"eq","signal":"input.email","value":"x","ref":"input.note"}`,
			"rules[0].when", CodeOperand},
		{"presence with a value", `{"op":"exists","signal":"input.email","value":"x"}`, "rules[0].when",
			CodeOperand},
		{"list on gt", `{"op":"gt","signal":"external.age","value":[1,2]}`, "rules[0].when.value", CodeOperand},
		{"ref on olderThan", `{"op":"olderThan","signal":"session.authTime","ref":"input.period"}`,
			"rules[0].when.ref", CodeOperand},
		{"list on olderThan", `{"op":"olderThan","signal":"session.authTime","value":["P1D"]}`,
			"rules[0].when.value", CodeOperand},
		{"ci on gt", `{"op":"gt","signal":"external.age","value":1,"ci":true}`, "rules[0].when.ci", CodeQualifier},
		{"every on exists", `{"op":"exists","signal":"input.email","every":true}`, "rules[0].when.every",
			CodeQualifier},
		{"options", `{"op":"eq","signal":"input.email","value":"x","options":{"fuzzy":true}}`,
			"rules[0].when.options", CodeOptions},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			p, diagnostics := s.compileWhen(tc.when)

			s.Nil(p)
			s.Require().NotEmpty(diagnostics)
			s.Equal(tc.path, diagnostics[0].Path)
			s.Equal(tc.code, diagnostics[0].Code)
		})
	}
}

func (s *CompilerTestSuite) TestWarningsStillCompile() {
	cases := []struct {
		name string
		when string
		code DiagnosticCode
	}{
		{"literal the declared comparator cannot read", `{"op":"eq","signal":"user.grade","value":"high"}`,
			CodeIncomparableLiteral},
		{"value outside the declared scale", `{"op":"gte","signal":"request.acr","value":"aal9"}`,
			CodeIncomparableLiteral},
		{"text operator on a number", `{"op":"startsWith","signal":"user.grade","value":"1"}`,
			CodeOperatorComparator},
		{"ordering on declared text", `{"op":"gt","signal":"user.postalCode","value":"9"}`,
			CodeOperatorComparator},
		{"olderThan on declared text", `{"op":"olderThan","signal":"user.postalCode","value":"P1D"}`,
			CodeOperatorComparator},
		{"olderThan with a non-duration", `{"op":"olderThan","signal":"session.authTime","value":"30 days"}`,
			CodeIncomparableLiteral},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			p, diagnostics := s.compileWhen(tc.when)

			s.NotNil(p)
			s.Equal([]DiagnosticCode{tc.code}, codes(diagnostics))
			s.Equal(SeverityWarning, diagnostics[0].Severity)
		})
	}
}

func (s *CompilerTestSuite) TestRuleWithoutConditionIsRefused() {
	d := &Decision{Rules: []Rule{{ID: "r", NextNode: "a"}}, Default: Arm{NextNode: "b"}, Unknown: Arm{NextNode: "c"}}

	p, diagnostics := Initialize().Compile(d, testCatalog())

	s.Nil(p)
	s.Equal([]Diagnostic{newDiagnostic("rules[0].when", CodeMissingField, map[string]string{paramField: "when"})},
		diagnostics)
}

func (s *CompilerTestSuite) TestNegationWithoutOneOperandIsRefused() {
	two := []*Predicate{{Op: opExists, Signal: "input.email"}, {Op: opExists, Signal: "input.note"}}
	for _, of := range [][]*Predicate{nil, two} {
		d := &Decision{Rules: []Rule{{ID: "r", When: &Predicate{Op: opNot, Of: of}, NextNode: "a"}},
			Default: Arm{NextNode: "b"}, Unknown: Arm{NextNode: "c"}}

		p, diagnostics := Initialize().Compile(d, testCatalog())

		s.Nil(p)
		s.Equal([]Diagnostic{newDiagnostic("rules[0].when.of", CodeArity, map[string]string{paramName: opNot})},
			diagnostics)
	}
}

func (s *CompilerTestSuite) TestSignalNameComesFromTheRequestedKey() {
	catalog := namelessCatalog{"input.email": {}, "input.note": {}}
	d, _ := newParser(defaultLimits()).parse([]byte(minimal(`{"op":"all","of":[` +
		`{"op":"exists","signal":"input.email"},{"op":"exists","signal":"input.note"}]}`)))

	p, diagnostics := Initialize().Compile(d, catalog)
	s.Require().Empty(diagnostics)
	f := p.AcquireFrame(time.Now())
	defer f.Release()
	f.SetValues("input.email", []string{"a@example.com"})
	f.SetAbsent("input.note")

	s.Equal("b", f.Evaluate().NextNode)
}

func (s *CompilerTestSuite) TestNormalisationFlattensAndRemovesDoubleNegation() {
	c := newCompiler(testCatalog())
	d, _ := newParser(defaultLimits()).parse([]byte(minimal(`{"op":"all","of":[` +
		`{"op":"all","of":[{"op":"exists","signal":"input.email"},{"op":"exists","signal":"input.note"}]},` +
		`{"op":"not","of":{"op":"not","of":{"op":"exists","signal":"input.username"}}}]}`)))

	t := c.normalise(d.Rules[0].When, "rules[0].when")

	s.Equal(opAll, t.pred.Op)
	s.Len(t.children, 3)
	s.Equal("rules[0].when.of[0].of[0]", t.children[0].path)
	s.Equal("rules[0].when.of[0].of[1]", t.children[1].path)
	s.Equal("rules[0].when.of[1].of.of", t.children[2].path)
	s.Equal(opExists, t.children[2].pred.Op)
}

func (s *CompilerTestSuite) TestCheapestChildrenFirst() {
	p, diagnostics := s.compileWhen(`{"op":"all","of":[` +
		`{"op":"eq","signal":"user.manager","value":"x"},` +
		`{"op":"eq","signal":"user.groups","value":"x"},` +
		`{"op":"eq","signal":"request.network","value":"corporate"}]}`)
	s.Require().Empty(diagnostics)
	f := p.AcquireFrame(testNow)
	f.SetValues("request.network", []string{"public"})
	f.SetValues("user.groups", []string{"x"})
	f.SetValues("user.manager", []string{"x"})

	f.Evaluate()

	reasons := map[string]Reason{}
	for _, l := range f.Leaves() {
		reasons[l.Signal] = l.Reason
	}
	s.Equal(map[string]Reason{"request.network": ReasonNoMatch, "user.groups": ReasonNotEvaluated,
		"user.manager": ReasonNotEvaluated}, reasons)
	s.Equal("request.network", f.Leaves()[0].Signal)
}

func (s *CompilerTestSuite) TestRuleOrderNeverChanges() {
	p := compileDecision(&s.Suite, `{"rules":[`+
		`{"id":"store_first","when":{"op":"exists","signal":"user.groups"},"nextNode":"a"},`+
		`{"id":"local_second","when":{"op":"exists","signal":"input.email"},"nextNode":"b"}],`+
		`"default":{"nextNode":"c"},"unknown":{"nextNode":"d"}}`)
	f := p.AcquireFrame(testNow)
	f.SetValues("user.groups", []string{"x"})
	f.SetValues("input.email", []string{"x"})

	s.Equal("store_first", f.Evaluate().RuleID)
}

func (s *CompilerTestSuite) TestStagesGroupSignalsByCost() {
	p, _ := s.compileWhen(`{"op":"any","of":[` +
		`{"op":"eq","signal":"user.manager","ref":"user.otherManager"},` +
		`{"op":"eq","signal":"user.groups","value":"x"},` +
		`{"op":"exists","signal":"input.email"},` +
		`{"op":"eq","signal":"user.grade","value":3}]}`)

	s.Equal([][]string{{"input.email"}, {"user.groups", "user.grade"}, {"user.manager", "user.otherManager"}},
		p.Stages())
}

func (s *CompilerTestSuite) TestLongTextListBecomesSet() {
	c := newCompiler(testCatalog())
	c.ruleSlots = map[int]bool{}
	list := &Literal{list: true}
	for _, v := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		list.values = append(list.values, literalValue{text: v})
	}

	text := c.lowerLeaf("p", &Predicate{Op: opEq, Signal: "input.email", Value: list})
	s.Len(text.set, hashSetMinListSize)

	numbers := &Literal{list: true}
	for _, v := range []string{"1", "2", "3", "4", "5", "6", "7", "8"} {
		numbers.values = append(numbers.values, literalValue{text: v, kind: literalNumber})
	}
	number := c.lowerLeaf("p", &Predicate{Op: opEq, Signal: "input.email", Value: numbers})
	s.Nil(number.set)

	declared := c.lowerLeaf("p", &Predicate{Op: opEq, Signal: "user.postalCode", Value: numbers})
	s.Len(declared.set, hashSetMinListSize)

	short := c.lowerLeaf("p", &Predicate{Op: opEq, Signal: "input.email", Value: &Literal{list: true,
		values: list.values[:2]}})
	s.Nil(short.set)
}
