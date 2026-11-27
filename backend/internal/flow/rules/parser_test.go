// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

// fullDecision is a decision block that uses every element of the grammar.
const fullDecision = `{
  "grammarVersion": 1,
  "hitPolicy": "FIRST",
  "rules": [
    {
      "id": "privileged",
      "label": "Payment approvers need MFA",
      "when": {
        "op": "all",
        "of": [
          {"op": "eq", "signal": "user.groups", "value": ["payments-approver", "treasury"]},
          {"op": "not", "of": {"op": "eq", "signal": "request.network", "value": "corporate", "ci": true}}
        ]
      },
      "nextNode": "mfa_prompt"
    },
    {
      "id": "under_age",
      "when": {"op": "not", "of": {"op": "gte", "signal": "external.age", "value": 18.50}},
      "nextNode": "deny_access"
    }
  ],
  "default": {"label": "Continue", "nextNode": "issue_token"},
  "unknown": {"nextNode": "deny_access"},
  "tests": [
    {
      "name": "approver on an unresolved network is denied",
      "context": {"flowType": "AUTHENTICATION", "applicationId": "app-1", "ouId": "ou-1"},
      "facts": {"user.groups": ["payments-approver"], "external.age": 30, "user.active": true},
      "unresolved": ["request.network"],
      "expect": "deny_access"
    }
  ],
  "source": {"frontend": "json", "frontendVersion": 1}
}`

type ParserTestSuite struct {
	suite.Suite
}

func TestParserTestSuite(t *testing.T) {
	suite.Run(t, new(ParserTestSuite))
}

// parse parses raw with the default limits.
func (s *ParserTestSuite) parse(raw string) (*Decision, []Diagnostic) {
	return newParser(defaultLimits()).parse([]byte(raw))
}

// minimal returns a valid decision whose single rule has the given condition.
func minimal(when string) string {
	return `{"rules":[{"id":"r1","when":` + when + `,"nextNode":"a"}],` +
		`"default":{"nextNode":"b"},"unknown":{"nextNode":"c"}}`
}

func (s *ParserTestSuite) TestParsesFullDecision() {
	d, diagnostics := s.parse(fullDecision)

	s.Empty(diagnostics)
	s.Equal(1, d.GrammarVersion)
	s.Equal(HitPolicyFirst, d.HitPolicy)
	s.Len(d.Rules, 2)
	s.Equal("privileged", d.Rules[0].ID)
	s.Equal("Payment approvers need MFA", d.Rules[0].Label)
	s.Equal(opAll, d.Rules[0].When.Op)
	s.Len(d.Rules[0].When.Of, 2)
	s.Equal([]string{"payments-approver", "treasury"}, d.Rules[0].When.Of[0].Value.Atoms())
	s.True(d.Rules[0].When.Of[0].Value.IsList())
	negated := d.Rules[0].When.Of[1].Of[0]
	s.True(negated.CI)
	s.Equal([]string{"18.5"}, d.Rules[1].When.Of[0].Value.Atoms())
	s.Equal(Arm{Label: "Continue", NextNode: "issue_token"}, d.Default)
	s.Equal(Arm{NextNode: "deny_access"}, d.Unknown)
	s.Equal(&TestContext{FlowType: "AUTHENTICATION", ApplicationID: "app-1", OUID: "ou-1"}, d.Tests[0].Context)
	s.Equal([]string{"30"}, d.Tests[0].Facts["external.age"].Atoms())
	s.Equal([]string{"true"}, d.Tests[0].Facts["user.active"].Atoms())
	s.Equal([]string{"request.network"}, d.Tests[0].Unresolved)
	s.Equal(&DecisionSource{Frontend: "json", FrontendVersion: 1}, d.Source)
}

func (s *ParserTestSuite) TestDefaultsGrammarVersionAndHitPolicy() {
	d, diagnostics := s.parse(minimal(`{"op":"exists","signal":"input.email"}`))

	s.Empty(diagnostics)
	s.Equal(grammarVersionCurrent, d.GrammarVersion)
	s.Equal(HitPolicyFirst, d.HitPolicy)
}

func (s *ParserTestSuite) TestCanonicalFormRoundTrips() {
	e := Initialize()
	d, diagnostics := e.Parse([]byte(fullDecision))
	s.Require().Empty(diagnostics)

	first, err := e.Canonical(d)
	s.Require().NoError(err)
	again, diagnostics := e.Parse(first)
	s.Require().Empty(diagnostics)
	second, err := e.Canonical(again)
	s.Require().NoError(err)

	s.Equal(string(first), string(second))
	s.Contains(string(first), `"value":18.5`)
	s.Contains(string(first), `{"op":"not","of":{"op":"eq","signal":"request.network","value":"corporate","ci":true}}`)
	s.Contains(string(first), `"value":["payments-approver","treasury"]`)
	s.Contains(string(first), `"user.active":true`)
}

func (s *ParserTestSuite) TestCanonicalNormalisesNumbers() {
	e := Initialize()
	d, diagnostics := e.Parse([]byte(minimal(`{"op":"eq","signal":"external.age","value":[7.000,-0.0,3.10]}`)))
	s.Require().Empty(diagnostics)

	out, err := e.Canonical(d)

	s.Require().NoError(err)
	s.Contains(string(out), `"value":[7,0,3.1]`)
}

func (s *ParserTestSuite) TestReportsUnknownFieldsWithPaths() {
	raw := `{"rules":[{"id":"r1","when":{"op":"eq","signal":"x","value":1,"color":"red"},"nextNode":"a",` +
		`"extra":1}],"default":{"nextNode":"b","x":1},"unknown":{"nextNode":"c"},"other":true}`

	_, diagnostics := s.parse(raw)

	s.Equal([]Diagnostic{
		newDiagnostic("other", CodeUnknownField, map[string]string{paramField: "other"}),
		newDiagnostic("rules[0].extra", CodeUnknownField, map[string]string{paramField: "extra"}),
		newDiagnostic("rules[0].when.color", CodeUnknownField, map[string]string{paramField: "color"}),
		newDiagnostic("default.x", CodeUnknownField, map[string]string{paramField: "x"}),
	}, diagnostics)
}

func (s *ParserTestSuite) TestReportsStructuralProblems() {
	cases := []struct {
		name string
		raw  string
		path string
		code DiagnosticCode
	}{
		{"not JSON", `{"rules":`, "", CodeSyntax},
		{"trailing data", `{} {}`, "", CodeSyntax},
		{"stray closing bracket", `{}]`, "", CodeSyntax},
		{"stray closing brace", "{}}", "", CodeSyntax},
		{"not an object", `[]`, "", CodeSyntax},
		{"no rules field", `{"default":{"nextNode":"b"},"unknown":{"nextNode":"c"}}`, "rules", CodeNoRules},
		{"empty rules", `{"rules":[],"default":{"nextNode":"b"},"unknown":{"nextNode":"c"}}`, "rules", CodeNoRules},
		{"rules not a list", `{"rules":{},"default":{"nextNode":"b"},"unknown":{"nextNode":"c"}}`, "rules",
			CodeSyntax},
		{"missing default", `{"rules":[{"id":"r","when":{"op":"exists","signal":"x"},"nextNode":"a"}],` +
			`"unknown":{"nextNode":"c"}}`, "default", CodeMissingArm},
		{"missing unknown", `{"rules":[{"id":"r","when":{"op":"exists","signal":"x"},"nextNode":"a"}],` +
			`"default":{"nextNode":"c"}}`, "unknown", CodeMissingArm},
		{"arm without nextNode", `{"rules":[{"id":"r","when":{"op":"exists","signal":"x"},"nextNode":"a"}],` +
			`"default":{},"unknown":{"nextNode":"c"}}`, "default.nextNode", CodeUnknownNode},
		{"rule without nextNode", `{"rules":[{"id":"r","when":{"op":"exists","signal":"x"}}],` +
			`"default":{"nextNode":"b"},"unknown":{"nextNode":"c"}}`, "rules[0].nextNode", CodeUnknownNode},
		{"rule without when", `{"rules":[{"id":"r","nextNode":"a"}],"default":{"nextNode":"b"},` +
			`"unknown":{"nextNode":"c"}}`, "rules[0].when", CodeMissingField},
		{"rule without id", `{"rules":[{"when":{"op":"exists","signal":"x"},"nextNode":"a"}],` +
			`"default":{"nextNode":"b"},"unknown":{"nextNode":"c"}}`, "rules[0].id", CodeMissingField},
		{"grammar version 2", `{"grammarVersion":2,"rules":[{"id":"r","when":{"op":"exists","signal":"x"},` +
			`"nextNode":"a"}],"default":{"nextNode":"b"},"unknown":{"nextNode":"c"}}`, "grammarVersion",
			CodeGrammarVersion},
		{"grammar version not an integer", `{"grammarVersion":1.5,"rules":[{"id":"r","when":{"op":"exists",` +
			`"signal":"x"},"nextNode":"a"}],"default":{"nextNode":"b"},"unknown":{"nextNode":"c"}}`,
			"grammarVersion", CodeSyntax},
		{"hit policy PRIORITY", `{"hitPolicy":"PRIORITY","rules":[{"id":"r","when":{"op":"exists","signal":"x"},` +
			`"nextNode":"a"}],"default":{"nextNode":"b"},"unknown":{"nextNode":"c"}}`, "hitPolicy", CodeHitPolicy},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			_, diagnostics := s.parse(tc.raw)

			s.Require().NotEmpty(diagnostics)
			s.Equal(tc.path, diagnostics[0].Path)
			s.Equal(tc.code, diagnostics[0].Code)
			s.Equal(SeverityError, diagnostics[0].Severity)
		})
	}
}

func (s *ParserTestSuite) TestRuleIDs() {
	cases := []struct {
		id    string
		valid bool
	}{
		{"privileged", true},
		{"under_age_2", true},
		{"a", true},
		{strings.Repeat("a", maxRuleIDLength), true},
		{strings.Repeat("a", maxRuleIDLength+1), false},
		{"Upper", false},
		{"1st", false},
		{"with-hyphen", false},
		{"_leading", false},
	}
	for _, tc := range cases {
		s.Run(tc.id, func() {
			raw := `{"rules":[{"id":"` + tc.id + `","when":{"op":"exists","signal":"x"},"nextNode":"a"}],` +
				`"default":{"nextNode":"b"},"unknown":{"nextNode":"c"}}`

			_, diagnostics := s.parse(raw)

			if tc.valid {
				s.Empty(diagnostics)
			} else {
				s.Equal([]DiagnosticCode{CodeInvalidID}, codes(diagnostics))
			}
		})
	}
}

func (s *ParserTestSuite) TestDuplicateRuleID() {
	raw := `{"rules":[{"id":"r","when":{"op":"exists","signal":"x"},"nextNode":"a"},` +
		`{"id":"r","when":{"op":"exists","signal":"y"},"nextNode":"a"}],"default":{"nextNode":"b"},` +
		`"unknown":{"nextNode":"c"}}`

	_, diagnostics := s.parse(raw)

	s.Equal([]Diagnostic{newDiagnostic("rules[1].id", CodeDuplicateRuleID, map[string]string{paramValue: "r"})},
		diagnostics)
}

func (s *ParserTestSuite) TestGroupAndNegationArity() {
	cases := []struct {
		name string
		when string
		path string
	}{
		{"all with one child", `{"op":"all","of":[{"op":"exists","signal":"x"}]}`, "rules[0].when.of"},
		{"any with no children", `{"op":"any","of":[]}`, "rules[0].when.of"},
		{"any without of", `{"op":"any"}`, "rules[0].when.of"},
		{"not with a list", `{"op":"not","of":[{"op":"exists","signal":"x"}]}`, "rules[0].when.of"},
		{"not without of", `{"op":"not"}`, "rules[0].when.of"},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			_, diagnostics := s.parse(minimal(tc.when))

			s.Require().NotEmpty(diagnostics)
			s.Equal(CodeArity, diagnostics[0].Code)
			s.Equal(tc.path, diagnostics[0].Path)
		})
	}
}

func (s *ParserTestSuite) TestInvalidUTF8IsRefused() {
	_, diagnostics := s.parse(minimal("{\"op\":\"eq\",\"signal\":\"x\",\"value\":\"a\xffb\"}"))
	s.Equal([]DiagnosticCode{CodeSyntax}, codes(diagnostics))

	_, diagnostics = s.parse(minimal(`{"op":"eq","signal":"x","value":"a\ufffdb"}`))
	s.Empty(diagnostics)
}

func (s *ParserTestSuite) TestCanonicalRefusesNegationWithoutOneCondition() {
	two := []*Predicate{{Op: opExists, Signal: "x"}, {Op: opExists, Signal: "y"}}
	for _, of := range [][]*Predicate{nil, two} {
		d := &Decision{Rules: []Rule{{ID: "r", When: &Predicate{Op: opNot, Of: of}, NextNode: "a"}}}

		_, err := Initialize().Canonical(d)

		s.ErrorIs(err, errNegationArity)
	}
}

func (s *ParserTestSuite) TestUnknownOperator() {
	cases := []struct {
		name string
		when string
	}{
		{"unregistered leaf operator", `{"op":"matches","signal":"x","value":"a"}`},
		{"misspelled group", `{"op":"and","of":[{"op":"exists","signal":"x"},{"op":"exists","signal":"y"}]}`},
		{"wrong case", `{"op":"ALL","of":[{"op":"exists","signal":"x"},{"op":"exists","signal":"y"}]}`},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			_, diagnostics := s.parse(minimal(tc.when))

			s.Equal([]DiagnosticCode{CodeUnknownOperator}, codes(diagnostics))
			s.Equal("rules[0].when.op", diagnostics[0].Path)
		})
	}
}

func (s *ParserTestSuite) TestLeafFields() {
	cases := []struct {
		name string
		when string
		path string
		code DiagnosticCode
	}{
		{"op missing", `{"signal":"x"}`, "rules[0].when.op", CodeMissingField},
		{"op not a string", `{"op":1,"signal":"x"}`, "rules[0].when.op", CodeSyntax},
		{"signal missing", `{"op":"exists"}`, "rules[0].when.signal", CodeMissingField},
		{"ci not a boolean", `{"op":"eq","signal":"x","value":"a","ci":"yes"}`, "rules[0].when.ci", CodeSyntax},
		{"ref not a string", `{"op":"eq","signal":"x","ref":1}`, "rules[0].when.ref", CodeSyntax},
		{"ref empty", `{"op":"eq","signal":"x","value":"a","ref":""}`, "rules[0].when.ref", CodeMissingField},
		{"options not an object", `{"op":"eq","signal":"x","value":1,"options":[]}`, "rules[0].when.options",
			CodeSyntax},
		{"value empty list", `{"op":"eq","signal":"x","value":[]}`, "rules[0].when.value", CodeSyntax},
		{"value object", `{"op":"eq","signal":"x","value":{}}`, "rules[0].when.value", CodeSyntax},
		{"value null", `{"op":"eq","signal":"x","value":null}`, "rules[0].when.value", CodeSyntax},
		{"value nested list", `{"op":"eq","signal":"x","value":[["a"]]}`, "rules[0].when.value[0]", CodeSyntax},
		{"value exponent", `{"op":"eq","signal":"x","value":1e3}`, "rules[0].when.value", CodeSyntax},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			_, diagnostics := s.parse(minimal(tc.when))

			s.Require().NotEmpty(diagnostics)
			s.Equal(tc.path, diagnostics[0].Path)
			s.Equal(tc.code, diagnostics[0].Code)
		})
	}
}

func (s *ParserTestSuite) TestDepthLimit() {
	when := `{"op":"exists","signal":"x"}`
	for range defaultMaxDepth - 1 {
		when = `{"op":"not","of":` + when + `}`
	}
	_, diagnostics := s.parse(minimal(when))
	s.Empty(diagnostics)

	_, diagnostics = s.parse(minimal(`{"op":"not","of":` + when + `}`))
	s.Equal([]DiagnosticCode{CodeLimit}, codes(diagnostics))
	s.Equal(map[string]string{paramLimit: "6"}, diagnostics[0].Params)
}

func (s *ParserTestSuite) TestLeafLimit() {
	leaves := make([]string, defaultMaxLeaves+1)
	for i := range leaves {
		leaves[i] = `{"op":"exists","signal":"x"}`
	}

	_, diagnostics := s.parse(minimal(`{"op":"any","of":[` + strings.Join(leaves, ",") + `]}`))

	s.Equal([]DiagnosticCode{CodeLimit}, codes(diagnostics))
	s.Equal("rules[0].when.of[128]", diagnostics[0].Path)
}

func (s *ParserTestSuite) TestRuleAndTestLimits() {
	rules := make([]string, defaultMaxRules+1)
	for i := range rules {
		rules[i] = `{"id":"r` + strings.Repeat("a", i) + `","when":{"op":"exists","signal":"x"},"nextNode":"a"}`
	}
	tests := make([]string, defaultMaxTests+1)
	for i := range tests {
		tests[i] = `{"name":"t","expect":"a"}`
	}
	raw := `{"rules":[` + strings.Join(rules, ",") + `],"default":{"nextNode":"b"},"unknown":{"nextNode":"c"},` +
		`"tests":[` + strings.Join(tests, ",") + `]}`

	_, diagnostics := s.parse(raw)

	s.Equal([]Diagnostic{
		newDiagnostic("rules", CodeLimit, map[string]string{paramLimit: "32"}),
		newDiagnostic("tests", CodeLimit, map[string]string{paramLimit: "32"}),
	}, diagnostics)
}

func (s *ParserTestSuite) TestListAndLabelLimits() {
	values := make([]string, maxListSizeCap+1)
	for i := range values {
		values[i] = `"v"`
	}
	_, diagnostics := s.parse(minimal(`{"op":"eq","signal":"x","value":[` + strings.Join(values, ",") + `]}`))
	s.Equal([]DiagnosticCode{CodeLimit}, codes(diagnostics))

	raw := `{"rules":[{"id":"r","label":"` + strings.Repeat("é", maxLabelLength+1) + `",` +
		`"when":{"op":"exists","signal":"x"},"nextNode":"a"}],"default":{"nextNode":"b"},"unknown":{"nextNode":"c"}}`
	_, diagnostics = s.parse(raw)
	s.Equal([]Diagnostic{newDiagnostic("rules[0].label", CodeLimit, map[string]string{paramLimit: "200"})},
		diagnostics)
}

func (s *ParserTestSuite) TestTestScenarios() {
	raw := `{"rules":[{"id":"r","when":{"op":"exists","signal":"x"},"nextNode":"a"}],` +
		`"default":{"nextNode":"b"},"unknown":{"nextNode":"c"},"tests":[{"facts":{"x":{}},"unresolved":["y",1],` +
		`"context":{"other":1}},"not an object"]}`

	_, diagnostics := s.parse(raw)

	s.Equal([]Diagnostic{
		newDiagnostic("tests[0].name", CodeMissingField, map[string]string{paramField: "name"}),
		newDiagnostic("tests[0].expect", CodeMissingField, map[string]string{paramField: "expect"}),
		newDiagnostic("tests[0].context.other", CodeUnknownField, map[string]string{paramField: "other"}),
		newDiagnostic("tests[0].facts.x", CodeSyntax, map[string]string{paramExpected: typeLiteral}),
		newDiagnostic("tests[0].unresolved[1]", CodeSyntax, map[string]string{paramExpected: typeString}),
		newDiagnostic("tests[1]", CodeSyntax, map[string]string{paramExpected: typeObject}),
	}, diagnostics)
}

func (s *ParserTestSuite) TestCollectsEveryDiagnostic() {
	raw := `{"rules":[{"id":"Bad","when":{"op":"all","of":[{"op":"exists"}]}}],"default":{},"extra":1}`

	_, diagnostics := s.parse(raw)

	s.Equal([]DiagnosticCode{CodeUnknownField, CodeInvalidID, CodeArity, CodeMissingField, CodeUnknownNode,
		CodeUnknownNode, CodeMissingArm}, codes(diagnostics))
}

func (s *ParserTestSuite) TestSourceAndWrongTypes() {
	raw := `{"rules":[{"id":"r","label":5,"when":{"op":"exists","signal":"x"},"nextNode":"a"}],` +
		`"default":{"nextNode":"b"},"unknown":{"nextNode":"c"},"tests":{},"source":{"frontend":"dsl",` +
		`"frontendVersion":"one","x":1}}`

	_, diagnostics := s.parse(raw)

	s.Equal([]Diagnostic{
		newDiagnostic("rules[0].label", CodeSyntax, map[string]string{paramExpected: typeString}),
		newDiagnostic("tests", CodeSyntax, map[string]string{paramExpected: typeArray}),
		newDiagnostic("source.x", CodeUnknownField, map[string]string{paramField: "x"}),
		newDiagnostic("source.frontendVersion", CodeSyntax, map[string]string{paramExpected: typeInteger}),
	}, diagnostics)
}

func (s *ParserTestSuite) TestSeverityFollowsCode() {
	s.Equal(SeverityWarning, newDiagnostic("", CodeIncomparableLiteral, nil).Severity)
	s.Equal(SeverityError, newDiagnostic("", CodeUnknownSignal, nil).Severity)
}
