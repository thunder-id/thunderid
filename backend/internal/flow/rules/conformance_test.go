// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"encoding/json"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"github.com/stretchr/testify/suite"
)

const (
	// generatedScenarios is how many random scenarios each property test checks.
	generatedScenarios = 2000
	// maxGeneratedDepth is the deepest condition the generator builds.
	maxGeneratedDepth = 3
)

// conformanceCase is one hand-written case of the conformance corpus.
type conformanceCase struct {
	Name       string              `json:"name"`
	Decision   json.RawMessage     `json:"decision"`
	Values     map[string][]string `json:"values"`
	Absent     []string            `json:"absent"`
	Unresolved []string            `json:"unresolved"`
	Expect     struct {
		SelectedBy SelectedBy `json:"selectedBy"`
		RuleID     string     `json:"ruleId"`
		NextNode   string     `json:"nextNode"`
	} `json:"expect"`
}

// states returns the signal states the case describes.
func (c *conformanceCase) states() map[string]state {
	states := map[string]state{}
	for name, values := range c.Values {
		states[name] = state{values: append([]string{}, values...)}
	}
	for _, name := range c.Absent {
		states[name] = state{absent: true}
	}
	for _, name := range c.Unresolved {
		states[name] = state{unresolved: true}
	}
	return states
}

// scenario is a decision with the signal states to evaluate it against.
type scenario struct {
	rules  []map[string]any
	states map[string]state
}

// raw renders the scenario's decision as JSON.
func (sc *scenario) raw(when func(map[string]any) map[string]any) []byte {
	rules := make([]map[string]any, len(sc.rules))
	for i, r := range sc.rules {
		rules[i] = map[string]any{"id": "r" + strconv.Itoa(i), "when": when(r), "nextNode": "n" + strconv.Itoa(i)}
	}
	raw, _ := json.Marshal(map[string]any{"rules": rules, "default": map[string]any{"nextNode": "d"},
		"unknown": map[string]any{"nextNode": "u"}})
	return raw
}

// generator builds random decisions and signal states from a seed.
type generator struct {
	rnd     *rand.Rand
	signals []string
}

// generatorValues mixes every kind of value, near misses and blanks.
var generatorValues = []string{
	"1", "9", "10", "10.00", "18", "-2", "3.5", "007", "true", "false", "aal1", "aal2", "aal3", "aal9",
	"2026-01-05", "2026-11-18T10:00:00Z", "2026-11-18T11:30:00Z", "2025-01-01T00:00:00+05:30",
	"P1D", "PT25H", "PT1H", "P1Y", "P1M", "treasury", "Treasury", "staff", "a@example.com", "A@EXAMPLE.COM",
	"@example.com", "exa", "", " ", "x",
}

// newGenerator returns a generator for the seed.
func newGenerator(seed uint64) *generator {
	// #nosec G404 -- seeded so a failing case is reproducible; nothing depends on it being unpredictable.
	g := &generator{rnd: rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))}
	for name := range testCatalog() {
		g.signals = append(g.signals, name)
	}
	slices.Sort(g.signals)
	return g
}

// scenario returns a random decision of one to three rules with random signal states.
func (g *generator) scenario() scenario {
	sc := scenario{states: map[string]state{}}
	for range 1 + g.rnd.IntN(3) {
		sc.rules = append(sc.rules, g.predicate(maxGeneratedDepth))
	}
	for _, name := range g.signals {
		switch n := g.rnd.IntN(10); {
		case n < 2:
		case n < 3:
			sc.states[name] = state{absent: true}
		case n < 4:
			sc.states[name] = state{unresolved: true}
		default:
			sc.states[name] = state{values: g.values(g.rnd.IntN(4))}
		}
	}
	return sc
}

// predicate returns a random condition no deeper than depth.
func (g *generator) predicate(depth int) map[string]any {
	if depth > 1 && g.rnd.IntN(3) == 0 {
		switch g.rnd.IntN(3) {
		case 0:
			return map[string]any{"op": opNot, "of": g.predicate(depth - 1)}
		default:
			op := []string{opAll, opAny}[g.rnd.IntN(2)]
			children := make([]map[string]any, 2+g.rnd.IntN(2))
			for i := range children {
				children[i] = g.predicate(depth - 1)
			}
			return map[string]any{"op": op, "of": children}
		}
	}
	return g.leaf()
}

// leaf returns a random well-formed leaf.
func (g *generator) leaf() map[string]any {
	ops := []string{opEq, opGt, opGte, opStartsWith, opEndsWith, opContains, opOlderThan, opExists, opIsEmpty}
	op := ops[g.rnd.IntN(len(ops))]
	leaf := map[string]any{"op": op, "signal": g.signal()}
	if op == opExists || op == opIsEmpty {
		return leaf
	}
	if g.rnd.IntN(4) == 0 {
		leaf["every"] = true
	}
	switch op {
	case opOlderThan:
		leaf["value"] = []string{"PT1H", "P1D", "P1Y", "x"}[g.rnd.IntN(4)]
		return leaf
	case opEq, opStartsWith, opEndsWith, opContains:
		if g.rnd.IntN(3) == 0 && testCatalog()[leaf["signal"].(string)].Comparator != ComparatorScale {
			leaf["ci"] = true
		}
	}
	switch n := g.rnd.IntN(8); {
	case n == 0:
		leaf["ref"] = g.signal()
	case n == 1 && op != opGt && op != opGte:
		leaf["value"] = g.literals(1 + g.rnd.IntN(10))
	default:
		leaf["value"] = g.literal()
	}
	return leaf
}

// signal returns a random catalog signal.
func (g *generator) signal() string {
	return g.signals[g.rnd.IntN(len(g.signals))]
}

// values returns n random values.
func (g *generator) values(n int) []string {
	values := make([]string, n)
	for i := range values {
		values[i] = generatorValues[g.rnd.IntN(len(generatorValues))]
	}
	return values
}

// literal returns a random value as the JSON type an author would write it in.
func (g *generator) literal() any {
	v := generatorValues[g.rnd.IntN(len(generatorValues))]
	if g.rnd.IntN(2) == 0 {
		if b, err := strconv.ParseBool(v); err == nil && (v == "true" || v == "false") {
			return b
		}
		if refNumber.MatchString(v) && (len(v) < 2 || v[0] != '0') {
			return json.Number(v)
		}
	}
	return v
}

// literals returns a list of n random values.
func (g *generator) literals(n int) []any {
	list := make([]any, n)
	for i := range list {
		list[i] = g.literal()
	}
	return list
}

// identity leaves a condition unchanged.
func identity(p map[string]any) map[string]any {
	return p
}

// reverseGroups reverses the children of every group.
func reverseGroups(p map[string]any) map[string]any {
	switch p["op"] {
	case opAll, opAny:
		children := slices.Clone(p["of"].([]map[string]any))
		slices.Reverse(children)
		for i := range children {
			children[i] = reverseGroups(children[i])
		}
		return map[string]any{"op": p["op"], "of": children}
	case opNot:
		return map[string]any{"op": opNot, "of": reverseGroups(p["of"].(map[string]any))}
	}
	return p
}

// deMorgan rewrites every group as the negation of its dual over negated children.
func deMorgan(p map[string]any) map[string]any {
	switch p["op"] {
	case opAll, opAny:
		dual := opAny
		if p["op"] == opAny {
			dual = opAll
		}
		children := p["of"].([]map[string]any)
		negated := make([]map[string]any, len(children))
		for i, child := range children {
			negated[i] = map[string]any{"op": opNot, "of": deMorgan(child)}
		}
		return map[string]any{"op": opNot, "of": map[string]any{"op": dual, "of": negated}}
	case opNot:
		return map[string]any{"op": opNot, "of": deMorgan(p["of"].(map[string]any))}
	}
	return p
}

// doubleNegation wraps every leaf in not not.
func doubleNegation(p map[string]any) map[string]any {
	switch p["op"] {
	case opAll, opAny:
		children := p["of"].([]map[string]any)
		wrapped := make([]map[string]any, len(children))
		for i, child := range children {
			wrapped[i] = doubleNegation(child)
		}
		return map[string]any{"op": p["op"], "of": wrapped}
	case opNot:
		return map[string]any{"op": opNot, "of": doubleNegation(p["of"].(map[string]any))}
	}
	return map[string]any{"op": opNot, "of": map[string]any{"op": opNot, "of": p}}
}

// mustCompile parses and compiles raw, failing the test on any error. The engine runs at its hard
// caps, so rewrites that deepen a condition still parse.
func mustCompile(t testing.TB, raw []byte) (*Decision, PlanInterface) {
	t.Helper()
	e := newRuleEngine(limits{maxRules: hardCapMaxRules, maxDepth: hardCapMaxDepth, maxLeaves: hardCapMaxLeaves})
	d, diagnostics := e.Parse(raw)
	if len(errorCodes(diagnostics)) > 0 {
		t.Fatalf("parse %s: %v", raw, diagnostics)
	}
	p, diagnostics := e.Compile(d, testCatalog())
	if len(errorCodes(diagnostics)) > 0 {
		t.Fatalf("compile %s: %v", raw, diagnostics)
	}
	return d, p
}

// decideBoth evaluates raw with the compiled plan and the reference evaluator and fails the test when
// they disagree on the outcome, a rule's truth or an evaluated leaf's truth.
func decideBoth(t testing.TB, raw []byte, states map[string]state) referenceOutcome {
	t.Helper()
	d, p := mustCompile(t, raw)
	f := p.AcquireFrame(testNow)
	defer f.Release()
	apply(f, states)
	got := f.Evaluate()
	r := &referenceEvaluator{catalog: testCatalog(), states: states, now: testNow}
	want := r.decide(d)

	if (referenceOutcome{got.SelectedBy, got.RuleID, got.NextNode}) != want {
		t.Fatalf("outcome %+v, reference %+v\ndecision %s\nstates %+v", got, want, raw, states)
	}
	reached := r.rules
	if decided := slices.IndexFunc(r.rules, func(t Truth) bool { return t != TruthFalse }); decided >= 0 {
		reached = r.rules[:decided+1]
	}
	if ruleTruths := f.RuleTruths(); !slices.Equal(ruleTruths, reached) {
		t.Fatalf("rule truths %v, reference %v\ndecision %s\nstates %+v", ruleTruths, reached, raw, states)
	}
	reported := map[string]bool{}
	for _, l := range f.Leaves() {
		if _, known := r.leaves[l.Path]; !known || reported[l.Path] {
			t.Fatalf("leaf %s is unexpected or reported twice\ndecision %s", l.Path, raw)
		}
		reported[l.Path] = true
		if l.Reason == ReasonNotEvaluated {
			if l.Truth != TruthUnknown {
				t.Fatalf("leaf %s not evaluated but %v", l.Path, l.Truth)
			}
			continue
		}
		if l.Truth != r.leaves[l.Path] {
			t.Fatalf("leaf %s is %v, reference %v\ndecision %s\nstates %+v", l.Path, l.Truth, r.leaves[l.Path],
				raw, states)
		}
		if l.Reason == ReasonIncomparable && l.Truth != TruthUnknown {
			t.Fatalf("incomparable leaf %s is %v", l.Path, l.Truth)
		}
	}
	if len(reported) != len(r.leaves) {
		t.Fatalf("%d leaves reported, reference %d\ndecision %s", len(reported), len(r.leaves), raw)
	}
	return want
}

// decideStaged supplies the signals stage by stage, as a flow executor would, and fails the test when
// an outcome reported as settled, or the outcome after the last stage, differs from the outcome with
// every signal supplied. With a state for every signal, the last outcome must be settled.
func decideStaged(t testing.TB, raw []byte, states map[string]state, final referenceOutcome) {
	t.Helper()
	_, p := mustCompile(t, raw)
	f := p.AcquireFrame(testNow)
	defer f.Release()
	var got Outcome
	complete := true
	for _, stage := range p.Stages() {
		supplied := map[string]state{}
		for _, name := range stage {
			if st, ok := states[name]; ok {
				supplied[name] = st
			} else {
				complete = false
			}
		}
		apply(f, supplied)
		got = f.Evaluate()
		if got.Settled {
			break
		}
	}
	if (referenceOutcome{got.SelectedBy, got.RuleID, got.NextNode}) != final {
		t.Fatalf("staged outcome %+v, final %+v\ndecision %s\nstates %+v", got, final, raw, states)
	}
	if complete && !got.Settled {
		t.Fatalf("unsettled with every signal supplied: %+v\ndecision %s\nstates %+v", got, raw, states)
	}
}

type ConformanceTestSuite struct {
	suite.Suite
}

func TestConformanceTestSuite(t *testing.T) {
	suite.Run(t, new(ConformanceTestSuite))
}

func (s *ConformanceTestSuite) TestCorpus() {
	raw, err := os.ReadFile(filepath.Join("testdata", "conformance.json"))
	s.Require().NoError(err)
	var cases []conformanceCase
	s.Require().NoError(json.Unmarshal(raw, &cases))
	s.Require().NotEmpty(cases)

	for _, c := range cases {
		s.Run(c.Name, func() {
			got := decideBoth(s.T(), c.Decision, c.states())

			s.Equal(referenceOutcome{c.Expect.SelectedBy, c.Expect.RuleID, c.Expect.NextNode}, got)
			decideStaged(s.T(), c.Decision, c.states(), got)
		})
	}
}

func (s *ConformanceTestSuite) TestPlanMatchesReference() {
	g := newGenerator(1)
	for range generatedScenarios {
		sc := g.scenario()
		raw := sc.raw(identity)
		final := decideBoth(s.T(), raw, sc.states)
		decideStaged(s.T(), raw, sc.states, final)
	}
}

func (s *ConformanceTestSuite) TestRewritesKeepTheOutcome() {
	rewrites := []struct {
		name    string
		rewrite func(map[string]any) map[string]any
	}{
		{"group children in any order", reverseGroups},
		{"De Morgan", deMorgan},
		{"double negation", doubleNegation},
	}
	for _, rw := range rewrites {
		s.Run(rw.name, func() {
			g := newGenerator(2)
			for range generatedScenarios {
				sc := g.scenario()
				want := decideBoth(s.T(), sc.raw(identity), sc.states)

				s.Require().Equal(want, decideBoth(s.T(), sc.raw(rw.rewrite), sc.states))
			}
		})
	}
}

func FuzzPlanMatchesReference(f *testing.F) {
	for _, seed := range []uint64{0, 1, 42, 2026} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, seed uint64) {
		sc := newGenerator(seed).scenario()
		raw := sc.raw(identity)
		final := decideBoth(t, raw, sc.states)
		decideStaged(t, raw, sc.states, final)
		if got := decideBoth(t, sc.raw(deMorgan), sc.states); got != final {
			t.Fatalf("De Morgan changed %+v to %+v\ndecision %s", final, got, raw)
		}
	})
}
