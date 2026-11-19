// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

// referenceEvaluator interprets a parsed decision directly, written from the specification alone. It
// shares no comparison or evaluation code with the compiled evaluator, does not short-circuit, and
// does not reorder anything, so the two can be checked against each other.
type referenceEvaluator struct {
	catalog mapCatalog
	states  map[string]state
	now     time.Time
	leaves  map[string]Truth
	rules   []Truth
}

// referenceOutcome is the decision the reference evaluator reaches.
type referenceOutcome struct {
	selectedBy SelectedBy
	ruleID     string
	nextNode   string
}

var (
	refNumber   = regexp.MustCompile(`^-?[0-9]+(\.[0-9]+)?$`)
	refDate     = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)
	refDuration = regexp.MustCompile(`^P(?:([0-9]{1,9})Y)?(?:([0-9]{1,9})M)?(?:([0-9]{1,9})W)?(?:([0-9]{1,9})D)?` +
		`(?:T(?:([0-9]{1,9})H)?(?:([0-9]{1,9})M)?(?:([0-9]{1,9})S)?)?$`)
)

// refKinds are the inferable ways of reading a value, in the order the specification tries them.
var refKinds = []ComparatorName{ComparatorInstant, ComparatorDuration, ComparatorNumber, ComparatorBoolean,
	ComparatorText}

// decide applies the FIRST hit policy to the decision.
func (r *referenceEvaluator) decide(d *Decision) referenceOutcome {
	r.leaves = map[string]Truth{}
	truths := make([]Truth, len(d.Rules))
	for i := range d.Rules {
		truths[i] = r.truth(d.Rules[i].When, "rules["+strconv.Itoa(i)+"].when")
	}
	r.rules = truths
	for i, t := range truths {
		switch t {
		case TruthTrue:
			return referenceOutcome{SelectedByRule, d.Rules[i].ID, d.Rules[i].NextNode}
		case TruthUnknown:
			return referenceOutcome{SelectedByUnknown, d.Rules[i].ID, d.Unknown.NextNode}
		}
	}
	return referenceOutcome{SelectedByDefault, "", d.Default.NextNode}
}

// truth evaluates a condition with Kleene's three-valued logic, recording every leaf by path.
func (r *referenceEvaluator) truth(p *Predicate, path string) Truth {
	switch p.Op {
	case opAll, opAny:
		results := make([]Truth, len(p.Of))
		for i, child := range p.Of {
			results[i] = r.truth(child, path+".of["+strconv.Itoa(i)+"]")
		}
		return refCombine(p.Op, results)
	case opNot:
		switch r.truth(p.Of[0], path+".of") {
		case TruthTrue:
			return TruthFalse
		case TruthFalse:
			return TruthTrue
		}
		return TruthUnknown
	}
	t := r.leafTruth(p)
	r.leaves[path] = t
	return t
}

// refCombine combines group results: for all, any false is false, else any unknown is unknown; any
// is the dual.
func refCombine(op string, results []Truth) Truth {
	decisive, opposite := TruthFalse, TruthTrue
	if op == opAny {
		decisive, opposite = TruthTrue, TruthFalse
	}
	sawUnknown := false
	for _, t := range results {
		if t == decisive {
			return decisive
		}
		if t == TruthUnknown {
			sawUnknown = true
		}
	}
	if sawUnknown {
		return TruthUnknown
	}
	return opposite
}

// leafTruth evaluates one leaf.
func (r *referenceEvaluator) leafTruth(p *Predicate) Truth {
	st, fetched := r.states[p.Signal]
	switch p.Op {
	case opExists:
		switch {
		case !fetched || st.unresolved:
			return TruthUnknown
		case st.absent:
			return TruthFalse
		}
		return TruthTrue
	case opIsEmpty:
		if !fetched || st.unresolved || st.absent {
			return TruthUnknown
		}
		for _, v := range st.values {
			if strings.TrimSpace(v) != "" {
				return TruthFalse
			}
		}
		return TruthTrue
	}
	if !fetched || st.unresolved || st.absent {
		return TruthUnknown
	}
	right := []string{}
	if p.Value != nil {
		right = p.Value.Atoms()
	}
	if p.Ref != "" {
		ref, refFetched := r.states[p.Ref]
		if !refFetched || ref.unresolved || ref.absent {
			return TruthUnknown
		}
		right = ref.values
	}
	return r.quantify(p, st.values, right)
}

// quantify applies the leaf to every left value: some by default, every when asked.
func (r *referenceEvaluator) quantify(p *Predicate, left, right []string) Truth {
	if p.Every && len(left) == 0 {
		return TruthUnknown
	}
	results := make([]Truth, len(left))
	for i, a := range left {
		pairs := make([]Truth, len(right), len(right)+1)
		for j, b := range right {
			pairs[j] = r.pair(p, a, b)
		}
		if p.Op == opOlderThan {
			pairs = []Truth{r.older(p, a, right[0])}
		}
		results[i] = refCombine(opAny, append(pairs, TruthFalse))
	}
	if p.Every {
		return refCombine(opAll, append(results, TruthTrue))
	}
	return refCombine(opAny, append(results, TruthFalse))
}

// pair compares one left value with one right value.
func (r *referenceEvaluator) pair(p *Predicate, a, b string) Truth {
	kind, ok := r.kindFor(p, a, b)
	if !ok {
		return TruthUnknown
	}
	switch p.Op {
	case opEq:
		return refTruth(r.compare(p, kind, a, b) == 0)
	case opGt:
		return refTruth(r.compare(p, kind, a, b) > 0)
	case opGte:
		return refTruth(r.compare(p, kind, a, b) >= 0)
	}
	if p.CI {
		a, b = strings.ToLower(a), strings.ToLower(b)
	}
	switch p.Op {
	case opStartsWith:
		return refTruth(strings.HasPrefix(a, b))
	case opEndsWith:
		return refTruth(strings.HasSuffix(a, b))
	}
	return refTruth(strings.Contains(a, b))
}

// older decides olderThan for one left value.
func (r *referenceEvaluator) older(p *Predicate, a, period string) Truth {
	declared := r.catalog[p.Signal].Comparator
	if declared != "" && declared != ComparatorInstant {
		return TruthUnknown
	}
	t, isInstant := refInstant(a)
	m := refDuration.FindStringSubmatch(period)
	if !isInstant || !refIsDuration(period) {
		return TruthUnknown
	}
	n := func(i int) int { v, _ := strconv.Atoi(m[i]); return v }
	threshold := refMonthsBefore(r.now, n(1)*12+n(2)).AddDate(0, 0, -(n(3)*7 + n(4)))
	seconds := n(5)*3600 + n(6)*60 + n(7)
	threshold = threshold.AddDate(0, 0, -(seconds / 86400)).Add(-time.Duration(seconds%86400) * time.Second)
	return refTruth(t.Before(threshold))
}

// refMonthsBefore steps back whole months, to the same day of the month or the last day of a shorter
// month.
func refMonthsBefore(t time.Time, months int) time.Time {
	first := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location()).AddDate(0, -months, 0)
	lastDay := time.Date(first.Year(), first.Month()+1, 0, 0, 0, 0, 0, t.Location()).Day()
	return time.Date(first.Year(), first.Month(), min(t.Day(), lastDay), t.Hour(), t.Minute(), t.Second(),
		t.Nanosecond(), t.Location())
}

// kindFor chooses how to read a pair: the signal's declared comparator, or the first inferable one
// that reads both values and can answer the operator.
func (r *referenceEvaluator) kindFor(p *Predicate, a, b string) (ComparatorName, bool) {
	declared := r.catalog[p.Signal].Comparator
	if declared != "" {
		if !refCanAnswer(p.Op, declared) || !r.reads(p.Signal, declared, a) || !r.reads(p.Signal, declared, b) {
			return "", false
		}
		return declared, true
	}
	for _, kind := range refKinds {
		if refCanAnswer(p.Op, kind) && r.reads(p.Signal, kind, a) && r.reads(p.Signal, kind, b) {
			return kind, true
		}
	}
	return "", false
}

// refCanAnswer reports whether a way of reading values can answer an operator.
func refCanAnswer(op string, kind ComparatorName) bool {
	switch op {
	case opEq:
		return true
	case opGt, opGte:
		return kind != ComparatorText && kind != ComparatorBoolean
	}
	return kind == ComparatorText
}

// reads reports whether a value can be read in the given way.
func (r *referenceEvaluator) reads(signal string, kind ComparatorName, v string) bool {
	switch kind {
	case ComparatorNumber:
		return refNumber.MatchString(v)
	case ComparatorInstant:
		_, ok := refInstant(v)
		return ok
	case ComparatorDuration:
		return refIsDuration(v)
	case ComparatorBoolean:
		return v == "true" || v == "false"
	case ComparatorScale:
		for _, member := range r.catalog[signal].Scale {
			if member == v {
				return true
			}
		}
		return false
	}
	return true
}

// compare orders two values read in the given way.
func (r *referenceEvaluator) compare(p *Predicate, kind ComparatorName, a, b string) int {
	switch kind {
	case ComparatorNumber:
		x, _ := new(big.Rat).SetString(a)
		y, _ := new(big.Rat).SetString(b)
		return x.Cmp(y)
	case ComparatorInstant:
		x, _ := refInstant(a)
		y, _ := refInstant(b)
		return x.Compare(y)
	case ComparatorDuration:
		return refCompareInt(refNominal(a), refNominal(b))
	case ComparatorScale:
		scale := r.catalog[p.Signal].Scale
		return refCompareInt(int64(refIndex(scale, a)), int64(refIndex(scale, b)))
	case ComparatorText:
		if p.CI {
			a, b = strings.ToLower(a), strings.ToLower(b)
		}
	}
	if a == b {
		return 0
	}
	return 1
}

// refInstant reads a date or an RFC 3339 timestamp with a zone.
func refInstant(v string) (time.Time, bool) {
	if refDate.MatchString(v) {
		t, err := time.Parse("2006-01-02", v)
		return t, err == nil
	}
	if !strings.Contains(v, "T") {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339Nano, v)
	return t, err == nil
}

// refIsDuration reports whether v is an ISO 8601 duration with at least one component.
func refIsDuration(v string) bool {
	m := refDuration.FindStringSubmatch(v)
	if m == nil || strings.HasSuffix(v, "T") {
		return false
	}
	for _, part := range m[1:] {
		if part != "" {
			return true
		}
	}
	return false
}

// refNominal is the length of a duration with 365-day years and 30-day months.
func refNominal(v string) int64 {
	m := refDuration.FindStringSubmatch(v)
	n := func(i int) int64 { x, _ := strconv.ParseInt(m[i], 10, 64); return x }
	days := n(1)*365 + n(2)*30 + n(3)*7 + n(4)
	return days*86400 + n(5)*3600 + n(6)*60 + n(7)
}

// refIndex returns the position of v in the scale.
func refIndex(scale []string, v string) int {
	for i, member := range scale {
		if member == v {
			return i
		}
	}
	return -1
}

// refCompareInt compares two integers.
func refCompareInt(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// refTruth converts a boolean to a truth value.
func refTruth(b bool) Truth {
	if b {
		return TruthTrue
	}
	return TruthFalse
}

type ReferenceEvaluatorTestSuite struct {
	suite.Suite
}

func TestReferenceEvaluatorTestSuite(t *testing.T) {
	suite.Run(t, new(ReferenceEvaluatorTestSuite))
}

func (s *ReferenceEvaluatorTestSuite) TestDecidesTheRunningExample() {
	d, diagnostics := Initialize().Parse([]byte(accessCheck))
	s.Require().Empty(diagnostics)
	r := &referenceEvaluator{catalog: testCatalog(), now: testNow, states: map[string]state{
		"user.groups": vals("treasury"), "request.network": vals("public"), "external.age": vals("30"),
	}}

	s.Equal(referenceOutcome{SelectedByRule, "privileged", "mfa_prompt"}, r.decide(d))
	s.Equal(TruthTrue, r.leaves["rules[0].when.of[0]"])
	s.Equal(TruthTrue, r.leaves["rules[1].when.of"])
}

func (s *ReferenceEvaluatorTestSuite) TestReadsValuesAsTheSpecificationSays() {
	r := &referenceEvaluator{catalog: testCatalog()}

	s.True(r.reads("", ComparatorNumber, "3.50"))
	s.False(r.reads("", ComparatorNumber, "1e3"))
	s.True(r.reads("", ComparatorInstant, "2026-01-05T10:00:00+05:30"))
	s.False(r.reads("", ComparatorInstant, "2026-01-05T10:00:00"))
	s.True(r.reads("", ComparatorDuration, "P1Y2M3DT4H"))
	s.False(r.reads("", ComparatorDuration, "P"))
	s.False(r.reads("", ComparatorDuration, "P1DT"))
	s.True(r.reads("request.acr", ComparatorScale, "aal2"))
	s.False(r.reads("request.acr", ComparatorScale, "aal9"))
	s.Equal(int64(90061), refNominal("P1DT1H1M1S"))
}
