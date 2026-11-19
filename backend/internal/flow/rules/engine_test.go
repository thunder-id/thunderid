// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

// testNow is the fixed time evaluations run at in tests.
var testNow = time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)

// mapCatalog is a signal catalog backed by a map, for tests.
type mapCatalog map[string]SignalSpec

// Lookup returns the signal with the given name.
func (m mapCatalog) Lookup(name string) (SignalSpec, bool) {
	spec, ok := m[name]
	spec.Name = name
	return spec, ok
}

// testCatalog returns the signals used across the tests.
func testCatalog() mapCatalog {
	return mapCatalog{
		"user.groups":       {CostClass: CostStore},
		"user.grade":        {Comparator: ComparatorNumber, CostClass: CostStore},
		"user.postalCode":   {Comparator: ComparatorText, CostClass: CostStore},
		"user.active":       {Comparator: ComparatorBoolean, CostClass: CostStore},
		"request.network":   {CostClass: CostLocal},
		"request.acr":       {Comparator: ComparatorScale, Scale: []string{"aal1", "aal2", "aal3"}},
		"session.authTime":  {Comparator: ComparatorInstant},
		"external.age":      {},
		"external.limit":    {},
		"input.email":       {},
		"input.username":    {},
		"input.note":        {},
		"input.colours":     {},
		"input.when":        {},
		"input.period":      {},
		"user.manager":      {CostClass: CostRemote},
		"user.otherManager": {CostClass: CostRemote},
	}
}

// compileDecision parses and compiles raw against the test catalog and requires no error.
func compileDecision(s *suite.Suite, raw string) PlanInterface {
	e := Initialize()
	d, diagnostics := e.Parse([]byte(raw))
	s.Require().Empty(errorCodes(diagnostics))
	p, diagnostics := e.Compile(d, testCatalog())
	s.Require().Empty(errorCodes(diagnostics))
	s.Require().NotNil(p)
	return p
}

// errorCodes returns the codes of the diagnostics with severity ERROR.
func errorCodes(diagnostics []Diagnostic) []DiagnosticCode {
	var list []DiagnosticCode
	for _, d := range diagnostics {
		if d.Severity == SeverityError {
			list = append(list, d.Code)
		}
	}
	return list
}

// codes returns the code of each diagnostic, in order.
func codes(diagnostics []Diagnostic) []DiagnosticCode {
	list := make([]DiagnosticCode, len(diagnostics))
	for i, d := range diagnostics {
		list[i] = d.Code
	}
	return list
}

type EngineTestSuite struct {
	suite.Suite
}

func TestEngineTestSuite(t *testing.T) {
	suite.Run(t, new(EngineTestSuite))
}

func (s *EngineTestSuite) TestOperatorsDescribesAllTwelveInOrder() {
	descriptors := Initialize().Operators()

	names := make([]string, len(descriptors))
	for i, d := range descriptors {
		names[i] = d.Name
	}
	s.Equal([]string{"eq", "gt", "gte", "startsWith", "endsWith", "contains", "olderThan", "exists", "isEmpty",
		"all", "any", "not"}, names)
}

func (s *EngineTestSuite) TestOperatorDescriptorShape() {
	descriptors := Initialize().Operators()

	eq := descriptors[0]
	s.Equal(OperatorKindComparison, eq.Kind)
	s.Equal(&RightSpec{Value: true, List: true, Ref: true}, eq.Right)
	s.Equal([]string{"ci", "every"}, eq.Qualifiers)
	s.Equal(Phrases{Positive: "is", Negative: "is not"}, eq.Phrases)

	olderThan := descriptors[6]
	s.Equal(&RightSpec{Value: true}, olderThan.Right)
	s.Equal([]string{"every"}, olderThan.Qualifiers)

	exists := descriptors[7]
	s.Equal(OperatorKindPresence, exists.Kind)
	s.Nil(exists.Right)
	s.Empty(exists.Qualifiers)

	s.Equal(OperatorKindGroup, descriptors[9].Kind)
	s.Equal(OperatorKindNegation, descriptors[11].Kind)
}

func (s *EngineTestSuite) TestComparatorsListsInferenceOrder() {
	descriptors := Initialize().Comparators()

	s.Len(descriptors, 6)
	inferred := []ComparatorName{}
	for _, d := range descriptors {
		if d.Inferred {
			inferred = append(inferred, d.Name)
			s.Equal(len(inferred), d.Order)
		}
	}
	s.Equal([]ComparatorName{ComparatorInstant, ComparatorDuration, ComparatorNumber, ComparatorBoolean,
		ComparatorText}, inferred)
	scale := descriptors[len(descriptors)-1]
	s.Equal(ComparatorScale, scale.Name)
	s.False(scale.Inferred)
	s.True(scale.Ordering)
	s.False(descriptors[4].Ordering)
	s.True(descriptors[4].TextMatch)
}

func (s *EngineTestSuite) TestLimitsAreClampedToHardCaps() {
	e := newRuleEngine(limits{maxRules: 1000, maxDepth: 0, maxLeaves: 1000, maxTests: -1})

	s.Equal(limits{maxRules: hardCapMaxRules, maxDepth: 1, maxLeaves: hardCapMaxLeaves, maxTests: 0}, e.limits)
	s.Equal(defaultLimits(), Initialize().(*ruleEngine).limits)
}

func (s *EngineTestSuite) TestCompileWithErrorsReturnsNoPlan() {
	e := Initialize()

	p, diagnostics := e.Compile(nil, testCatalog())

	s.Nil(p)
	s.Equal([]DiagnosticCode{CodeSyntax}, codes(diagnostics))
}
