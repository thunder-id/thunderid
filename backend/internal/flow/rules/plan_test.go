// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

// threeRules is a decision whose rules each test one input signal for the value "yes".
const threeRules = `{"rules":[` +
	`{"id":"first","when":{"op":"eq","signal":"input.email","value":"yes"},"nextNode":"n1"},` +
	`{"id":"second","when":{"op":"eq","signal":"input.note","value":"yes"},"nextNode":"n2"},` +
	`{"id":"third","when":{"op":"eq","signal":"input.username","value":"yes"},"nextNode":"n3"}],` +
	`"default":{"nextNode":"default_node"},"unknown":{"nextNode":"unknown_node"}}`

// accessCheck is the running example of the specification.
const accessCheck = `{"rules":[` +
	`{"id":"privileged","when":{"op":"all","of":[` +
	`{"op":"eq","signal":"user.groups","value":["payments-approver","treasury"]},` +
	`{"op":"not","of":{"op":"eq","signal":"request.network","value":"corporate","ci":true}}]},` +
	`"nextNode":"mfa_prompt"},` +
	`{"id":"under_age","when":{"op":"not","of":{"op":"gte","signal":"external.age","value":18}},` +
	`"nextNode":"deny_access"}],` +
	`"default":{"nextNode":"issue_token"},"unknown":{"nextNode":"deny_access"}}`

type PlanTestSuite struct {
	suite.Suite
}

func TestPlanTestSuite(t *testing.T) {
	suite.Run(t, new(PlanTestSuite))
}

// state sets a signal in a frame: values, "absent" or "unresolved".
type state struct {
	values     []string
	absent     bool
	unresolved bool
}

// apply records the state of each signal in the frame.
func apply(f FrameInterface, states map[string]state) {
	for name, st := range states {
		switch {
		case st.absent:
			f.SetAbsent(name)
		case st.unresolved:
			f.SetUnresolved(name)
		default:
			f.SetValues(name, st.values)
		}
	}
}

// vals is a resolved state with the given values.
func vals(values ...string) state {
	return state{values: values}
}

// run evaluates the decision with the signal states.
func (s *PlanTestSuite) run(raw string, states map[string]state) (Outcome, FrameInterface) {
	f := compileDecision(&s.Suite, raw).AcquireFrame(testNow)
	apply(f, states)
	return f.Evaluate(), f
}

// truthOfWhen evaluates a single-rule decision and returns its rule's truth.
func (s *PlanTestSuite) truthOfWhen(when string, states map[string]state) Truth {
	_, f := s.run(minimal(when), states)
	return f.RuleTruths()[0]
}

func (s *PlanTestSuite) TestFirstTrueRuleDecides() {
	out, f := s.run(threeRules, map[string]state{
		"input.email": vals("yes"), "input.note": vals("yes"), "input.username": vals("yes"),
	})

	s.Equal(Outcome{SelectedBy: SelectedByRule, RuleID: "first", NextNode: "n1", Settled: true}, out)
	s.Equal([]Truth{TruthTrue}, f.RuleTruths())
	s.Equal(ReasonNotEvaluated, f.Leaves()[1].Reason)
}

func (s *PlanTestSuite) TestEveryRuleFalseTakesDefault() {
	out, f := s.run(threeRules, map[string]state{
		"input.email": vals("no"), "input.note": vals("no"), "input.username": vals("no"),
	})

	s.Equal(Outcome{SelectedBy: SelectedByDefault, NextNode: "default_node", Settled: true}, out)
	s.Equal([]Truth{TruthFalse, TruthFalse, TruthFalse}, f.RuleTruths())
}

func (s *PlanTestSuite) TestUnknownRuleBlocksLaterRules() {
	out, f := s.run(threeRules, map[string]state{
		"input.email": vals("no"), "input.note": {unresolved: true}, "input.username": vals("yes"),
	})

	s.Equal(Outcome{SelectedBy: SelectedByUnknown, RuleID: "second", NextNode: "unknown_node", Settled: true}, out)
	s.Equal([]Truth{TruthFalse, TruthUnknown}, f.RuleTruths())
	s.Equal(ReasonUnresolved, f.Leaves()[1].Reason)
}

func (s *PlanTestSuite) TestThreeValuedGroups() {
	yes, no, unknown := vals("yes"), vals("no"), state{unresolved: true}
	leafA := `{"op":"eq","signal":"input.email","value":"yes"}`
	leafB := `{"op":"eq","signal":"input.note","value":"yes"}`
	cases := []struct {
		name string
		when string
		a, b state
		want Truth
	}{
		{"all true and unknown", `{"op":"all","of":[` + leafA + `,` + leafB + `]}`, yes, unknown, TruthUnknown},
		{"all false and unknown", `{"op":"all","of":[` + leafA + `,` + leafB + `]}`, no, unknown, TruthFalse},
		{"all true and true", `{"op":"all","of":[` + leafA + `,` + leafB + `]}`, yes, yes, TruthTrue},
		{"any true and unknown", `{"op":"any","of":[` + leafA + `,` + leafB + `]}`, yes, unknown, TruthTrue},
		{"any false and unknown", `{"op":"any","of":[` + leafA + `,` + leafB + `]}`, no, unknown, TruthUnknown},
		{"any false and false", `{"op":"any","of":[` + leafA + `,` + leafB + `]}`, no, no, TruthFalse},
		{"not unknown", `{"op":"not","of":` + leafA + `}`, unknown, yes, TruthUnknown},
		{"not true", `{"op":"not","of":` + leafA + `}`, yes, yes, TruthFalse},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.Equal(tc.want, s.truthOfWhen(tc.when, map[string]state{"input.email": tc.a, "input.note": tc.b}))
		})
	}
}

func (s *PlanTestSuite) TestMultiValuedSignals() {
	groups := map[string]state{"input.colours": vals("employees", "payments-approver")}

	s.Equal(TruthTrue, s.truthOfWhen(`{"op":"eq","signal":"input.colours","value":"payments-approver"}`, groups))
	s.Equal(TruthFalse, s.truthOfWhen(
		`{"op":"eq","signal":"input.colours","value":"payments-approver","every":true}`, groups))
	s.Equal(TruthTrue, s.truthOfWhen(
		`{"op":"eq","signal":"input.colours","value":["employees","payments-approver"],"every":true}`, groups))
	s.Equal(TruthFalse, s.truthOfWhen(
		`{"op":"not","of":{"op":"eq","signal":"input.colours","value":"employees"}}`, groups))

	empty := map[string]state{"input.colours": vals()}
	s.Equal(TruthFalse, s.truthOfWhen(`{"op":"eq","signal":"input.colours","value":"x"}`, empty))
	s.Equal(TruthUnknown, s.truthOfWhen(`{"op":"eq","signal":"input.colours","value":"x","every":true}`, empty))
}

func (s *PlanTestSuite) TestTextOperators() {
	email := map[string]state{"input.username": vals("Alice@Example.com")}

	s.Equal(TruthTrue, s.truthOfWhen(`{"op":"contains","signal":"input.username","value":"@"}`, email))
	s.Equal(TruthFalse, s.truthOfWhen(`{"op":"endsWith","signal":"input.username","value":"@example.com"}`, email))
	s.Equal(TruthTrue, s.truthOfWhen(
		`{"op":"endsWith","signal":"input.username","value":"@example.com","ci":true}`, email))
	s.Equal(TruthTrue, s.truthOfWhen(
		`{"op":"startsWith","signal":"input.username","value":["bob","alice"],"ci":true}`, email))
	s.Equal(TruthTrue, s.truthOfWhen(`{"op":"eq","signal":"input.username","value":"alice@example.com","ci":true}`,
		email))
}

func (s *PlanTestSuite) TestPresence() {
	cases := []struct {
		name string
		op   string
		st   state
		want Truth
	}{
		{"exists resolved", opExists, vals("x"), TruthTrue},
		{"exists resolved empty", opExists, vals(), TruthTrue},
		{"exists absent", opExists, state{absent: true}, TruthFalse},
		{"exists unresolved", opExists, state{unresolved: true}, TruthUnknown},
		{"isEmpty blank", opIsEmpty, vals(" ", ""), TruthTrue},
		{"isEmpty no values", opIsEmpty, vals(), TruthTrue},
		{"isEmpty with a value", opIsEmpty, vals(" ", "x"), TruthFalse},
		{"isEmpty absent", opIsEmpty, state{absent: true}, TruthUnknown},
		{"isEmpty unresolved", opIsEmpty, state{unresolved: true}, TruthUnknown},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.Equal(tc.want, s.truthOfWhen(`{"op":"`+tc.op+`","signal":"input.note"}`,
				map[string]state{"input.note": tc.st}))
		})
	}
	s.Equal(TruthUnknown, s.truthOfWhen(`{"op":"eq","signal":"input.note","value":"x"}`,
		map[string]state{"input.note": {absent: true}}))
}

func (s *PlanTestSuite) TestComparisonDoesNotDependOnDeclaredTypes() {
	s.Equal(TruthTrue, s.truthOfWhen(`{"op":"gte","signal":"external.age","value":18}`,
		map[string]state{"external.age": vals("30")}))
	s.Equal(TruthFalse, s.truthOfWhen(`{"op":"eq","signal":"user.postalCode","value":"123"}`,
		map[string]state{"user.postalCode": vals("0123")}))
	s.Equal(TruthTrue, s.truthOfWhen(`{"op":"olderThan","signal":"session.authTime","value":"P30D"}`,
		map[string]state{"session.authTime": vals(testNow.AddDate(0, 0, -40).Format(time.RFC3339))}))
	s.Equal(TruthFalse, s.truthOfWhen(`{"op":"olderThan","signal":"session.authTime","value":"P30D"}`,
		map[string]state{"session.authTime": vals(testNow.AddDate(0, 0, -20).Format(time.RFC3339))}))
	s.Equal(TruthTrue, s.truthOfWhen(`{"op":"gt","signal":"input.note","value":9}`,
		map[string]state{"input.note": vals("10")}))
	s.Equal(TruthTrue, s.truthOfWhen(`{"op":"gte","signal":"request.acr","value":"aal2"}`,
		map[string]state{"request.acr": vals("aal3")}))
}

func (s *PlanTestSuite) TestIncomparableIsUnknown() {
	out, f := s.run(minimal(`{"op":"gte","signal":"external.age","value":18}`),
		map[string]state{"external.age": vals("thirty")})
	s.Equal(SelectedByUnknown, out.SelectedBy)
	s.Equal(ReasonIncomparable, f.Leaves()[0].Reason)

	s.Equal(TruthUnknown, s.truthOfWhen(`{"op":"gt","signal":"user.postalCode","value":"9"}`,
		map[string]state{"user.postalCode": vals("10")}))
	s.Equal(TruthUnknown, s.truthOfWhen(`{"op":"olderThan","signal":"session.authTime","value":"30 days"}`,
		map[string]state{"session.authTime": vals("2020-01-01")}))
	s.Equal(TruthUnknown, s.truthOfWhen(`{"op":"olderThan","signal":"input.when","value":"P1D"}`,
		map[string]state{"input.when": vals("yesterday")}))
	s.Equal(TruthTrue, s.truthOfWhen(`{"op":"olderThan","signal":"input.when","value":"P1D"}`,
		map[string]state{"input.when": vals("2020-01-01")}))
}

func (s *PlanTestSuite) TestMissingDataNeverGrantsAccess() {
	out, f := s.run(accessCheck, map[string]state{
		"user.groups": {unresolved: true}, "request.network": vals("public"), "external.age": vals("30"),
	})
	s.Equal(Outcome{SelectedBy: SelectedByUnknown, RuleID: "privileged", NextNode: "deny_access", Settled: true}, out)
	s.Equal([]Truth{TruthUnknown}, f.RuleTruths())

	s.Equal(TruthUnknown, s.truthOfWhen(`{"op":"not","of":{"op":"eq","signal":"user.groups","value":"admin"}}`,
		map[string]state{"user.groups": {unresolved: true}}))
}

func (s *PlanTestSuite) TestAccessCheckExample() {
	cases := []struct {
		name   string
		states map[string]state
		want   string
	}{
		{"approver off the corporate network", map[string]state{"user.groups": vals("payments-approver"),
			"request.network": vals("public"), "external.age": vals("30")}, "mfa_prompt"},
		{"approver on the corporate network", map[string]state{"user.groups": vals("payments-approver"),
			"request.network": vals("CORPORATE"), "external.age": vals("30")}, "issue_token"},
		{"under eighteen", map[string]state{"user.groups": vals("employees"),
			"request.network": vals("public"), "external.age": vals("16")}, "deny_access"},
		{"age missing", map[string]state{"user.groups": vals("employees"),
			"request.network": vals("public"), "external.age": {absent: true}}, "deny_access"},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			out, _ := s.run(accessCheck, tc.states)

			s.Equal(tc.want, out.NextNode)
			s.True(out.Settled)
		})
	}
}

func (s *PlanTestSuite) TestStagedEvaluationStopsWhenSettled() {
	p := compileDecision(&s.Suite, accessCheck)
	s.Equal([][]string{{"request.network", "external.age"}, {"user.groups"}}, p.Stages())

	f := p.AcquireFrame(testNow)
	f.SetValues("request.network", []string{"corporate"})
	f.SetValues("external.age", []string{"30"})
	out := f.Evaluate()
	s.Equal(Outcome{SelectedBy: SelectedByDefault, NextNode: "issue_token", Settled: true}, out)

	g := p.AcquireFrame(testNow)
	g.SetValues("request.network", []string{"public"})
	g.SetValues("external.age", []string{"30"})
	out = g.Evaluate()
	s.False(out.Settled)
	s.Equal(SelectedByUnknown, out.SelectedBy)

	g.SetValues("user.groups", []string{"treasury"})
	s.Equal(Outcome{SelectedBy: SelectedByRule, RuleID: "privileged", NextNode: "mfa_prompt", Settled: true},
		g.Evaluate())
}

func (s *PlanTestSuite) TestRefComparison() {
	when := `{"op":"gt","signal":"external.age","ref":"external.limit"}`

	s.Equal(TruthTrue, s.truthOfWhen(when, map[string]state{"external.age": vals("30"),
		"external.limit": vals("18")}))
	s.Equal(TruthFalse, s.truthOfWhen(when, map[string]state{"external.age": vals("10"),
		"external.limit": vals("18", "21")}))
	_, f := s.run(minimal(when), map[string]state{"external.age": vals("30"), "external.limit": {absent: true}})
	s.Equal(ReasonAbsent, f.Leaves()[0].Reason)
	s.Equal(TruthFalse, s.truthOfWhen(when, map[string]state{"external.age": vals("30"), "external.limit": vals()}))
}

func (s *PlanTestSuite) TestLongListLookup() {
	when := `{"op":"eq","signal":"input.note","value":["a","b","c","d","e","f","g","Target"],"ci":true}`

	s.Equal(TruthTrue, s.truthOfWhen(when, map[string]state{"input.note": vals("x", "target")}))
	s.Equal(TruthFalse, s.truthOfWhen(when, map[string]state{"input.note": vals("x")}))
}

func (s *PlanTestSuite) TestLeafRecords() {
	_, f := s.run(accessCheck, map[string]state{
		"user.groups": vals("treasury"), "request.network": vals("public"), "external.age": vals("30"),
	})

	records := map[string]LeafRecord{}
	for _, r := range f.Leaves() {
		records[r.Path] = r
	}
	s.Equal(LeafRecord{Path: "rules[0].when.of[0]", Signal: "user.groups", Operator: opEq, Truth: TruthTrue,
		Reason: ReasonMatch, Comparator: ComparatorText}, records["rules[0].when.of[0]"])
	s.Equal(LeafRecord{Path: "rules[0].when.of[1].of", Signal: "request.network", Operator: opEq,
		Truth: TruthFalse, Reason: ReasonNoMatch, Comparator: ComparatorText}, records["rules[0].when.of[1].of"])
	s.Equal(LeafRecord{Path: "rules[1].when.of", Signal: "external.age", Operator: opGte, Truth: TruthUnknown,
		Reason: ReasonNotEvaluated}, records["rules[1].when.of"])
}

func (s *PlanTestSuite) TestFrameIsReusedClean() {
	p := compileDecision(&s.Suite, accessCheck)
	f := p.AcquireFrame(testNow)
	f.SetValues("user.groups", []string{"treasury"})
	f.SetValues("request.network", []string{"public"})
	s.Equal("mfa_prompt", f.Evaluate().NextNode)
	f.Release()

	g := p.AcquireFrame(testNow)
	out := g.Evaluate()

	s.False(out.Settled)
	for _, r := range g.Leaves() {
		s.Equal(ReasonNotEvaluated, r.Reason)
	}
}

func (s *PlanTestSuite) TestReleasingTwiceDoesNotShareTheFrame() {
	p := compileDecision(&s.Suite, accessCheck)
	f := p.AcquireFrame(testNow)
	f.Release()
	f.Release()

	g, h := p.AcquireFrame(testNow), p.AcquireFrame(testNow)

	s.NotSame(g, h)
}

func (s *PlanTestSuite) TestReleasedFrameHoldsNoValues() {
	f := compileDecision(&s.Suite, accessCheck).AcquireFrame(testNow)
	f.SetValues("user.groups", []string{"employees", "treasury"})
	f.SetValues("user.groups", []string{"treasury"})

	f.Release()

	for _, slot := range f.(*frame).slots {
		for _, a := range slot.atoms[:cap(slot.atoms)] {
			s.Equal(atom{}, a)
		}
	}
}

func (s *PlanTestSuite) TestUnknownSignalsAreIgnored() {
	_, f := s.run(minimal(`{"op":"exists","signal":"input.email"}`), map[string]state{
		"input.email": vals("x"), "user.groups": vals("x"), "external.age": {absent: true},
		"request.network": {unresolved: true},
	})

	s.Equal([]Truth{TruthTrue}, f.RuleTruths())
}

func (s *PlanTestSuite) TestEvaluationAllocatesNothing() {
	f := compileDecision(&s.Suite, accessCheck).AcquireFrame(testNow)
	f.SetValues("user.groups", []string{"employees", "treasury"})
	f.SetValues("request.network", []string{"public"})
	f.SetValues("external.age", []string{"30"})

	allocs := testing.AllocsPerRun(100, func() { f.Evaluate() })

	s.Equal(float64(0), allocs)
}

func BenchmarkEvaluate(b *testing.B) {
	e := Initialize()
	d, _ := e.Parse([]byte(accessCheck))
	p, _ := e.Compile(d, testCatalog())
	f := p.AcquireFrame(testNow)
	f.SetValues("user.groups", []string{"employees", "treasury"})
	f.SetValues("request.network", []string{"public"})
	f.SetValues("external.age", []string{"30"})
	b.ReportAllocs()
	for b.Loop() {
		f.Evaluate()
	}
}
