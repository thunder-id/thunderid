// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"
)

type PolicyTestSuite struct {
	suite.Suite
}

func TestPolicyTestSuite(t *testing.T) {
	suite.Run(t, new(PolicyTestSuite))
}

func schemaWith(property string) json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"a":` + property +
		`,"actions":{"type":"array","items":{"type":"string"}}}}`)
}

func (s *PolicyTestSuite) TestPolicies_ReadsNestedPolicies() {
	policies := Policies(json.RawMessage(`{"type":"object","properties":{
		"cap":{"type":"object","properties":{"amount":{"type":"number","maximum":100,"policy":{
			"modification":"decrease","modifiableBy":["processor","client"]}}}},
		"account":{"type":"object","policy":{"suppliedBy":"processor"},
			"properties":{"iban":{"type":"string"}}},
		"items":{"type":"array","policy":{"modification":"removeItems","modifiableBy":["user"]}},
		"excluded":{"type":"array","policy":{"modification":"addItems","modifiableBy":["processor"]}}}}`))
	s.Equal(modificationDecrease, policies["cap.amount"].Modification)
	s.Equal([]string{PartyProcessor, PartyClient}, policies["cap.amount"].ModifiableBy)
	s.True(policies.suppliedByProcessor("account.iban"), "a field inside a supplied object is supplied too")
	s.False(policies.suppliedByProcessor("cap.amount"))
	s.Equal(modificationRemoveItems, policies["items"].Modification)
}

func (s *PolicyTestSuite) TestPolicies_RejectedAtRegistration() {
	policy := func(body string) string { return `{"type":"string","policy":{` + body + `}}` }
	cases := map[string]string{
		"unknown setting":            policy(`"rule":"lte"`),
		"setting in other case":      policy(`"SUPPLIEDBY":"processor"`),
		"unknown suppliedBy":         policy(`"suppliedBy":"bank"`),
		"unknown modification":       policy(`"modification":"shrink","modifiableBy":["user"]`),
		"choosing one option":        policy(`"modification":"selectOne","modifiableBy":["user"]`),
		"party without modification": policy(`"modifiableBy":["user"]`),
		"decrease on unordered text": policy(`"modification":"decrease","modifiableBy":["user"]`),
		"removeItems on a string":    policy(`"modification":"removeItems","modifiableBy":["user"]`),
		"addItems on a string":       policy(`"modification":"addItems","modifiableBy":["user"]`),
		"declared valueType":         policy(`"modification":"decrease","modifiableBy":["user"],"valueType":"number"`),
		"limit on unordered text":    policy(`"maximum":"5"`),
		"limit of the wrong type":    `{"type":"string","format":"date","policy":{"maximum":"soon"}}`,
		"decrease on decimal text": `{"type":"string","format":"decimal",` +
			`"policy":{"modification":"decrease","modifiableBy":["user"]}}`,
		"policy limit on a number":   `{"type":"number","policy":{"maximum":10}}`,
		"date limit that is no date": `{"type":"string","format":"date","policy":{"minimum":"2026-02-30"}}`,
		"date minimum after maximum": `{"type":"string","format":"date",` +
			`"policy":{"minimum":"2026-12-31","maximum":"2026-01-01"}}`,
		"number minimum above maximum": `{"type":"number","minimum":10,"maximum":1}`,
	}
	for name, property := range cases {
		_, err := CompileSchema(schemaWith(property))
		s.Error(err, name)
	}
	list := func(policy string) string {
		return `{"type":"array","items":{"type":"string"},"policy":{` + policy + `}}`
	}
	listCases := map[string][2]string{
		"supplied and modifiable": {`"suppliedBy":"processor","modification":"removeItems","modifiableBy":["user"]`,
			"a field supplied by the processor cannot also be modifiable"},
		"modification without party": {`"modification":"removeItems"`, "a modification needs modifiableBy"},
		"unknown party": {`"modification":"removeItems","modifiableBy":["admin"]`,
			`unsupported modifiableBy "admin"`},
		"repeated party": {`"modification":"removeItems","modifiableBy":["user","user"]`,
			`unsupported modifiableBy "user"`},
	}
	for name, c := range listCases {
		_, err := CompileSchema(schemaWith(list(c[0])))
		s.ErrorContains(err, c[1], name)
	}

	_, err := CompileSchema(json.RawMessage(
		`{"type":"object","properties":{"type":{"type":"string","policy":{"suppliedBy":"processor"}}}}`))
	s.Error(err, "the type cannot be supplied by the processor")
	_, err = CompileSchema(json.RawMessage(
		`{"type":"object","properties":{"actions":{"type":"array","items":{"type":"string"},` +
			`"policy":{"suppliedBy":"processor"}}}}`))
	s.NoError(err, "a common field is an ordinary field, so the type decides who supplies it")
	_, err = CompileSchema(json.RawMessage(`{"type":"object","properties":{"account":{"type":"object",` +
		`"policy":{"suppliedBy":"processor"},"properties":{"limit":{"type":"number",` +
		`"policy":{"modification":"decrease","modifiableBy":["user"]}}}}}}`))
	s.ErrorContains(err, "inside one the processor supplies", "a field the processor supplies is fixed as a whole")
	_, err = CompileSchema(json.RawMessage(`{`))
	s.Error(err)
}

func (s *PolicyTestSuite) TestCheckFieldLimits() {
	policies := FieldPolicies{
		"amount": {
			ValueType: valueTypeDate,
			Minimum:   json.RawMessage(`"2026-01-01"`),
			Maximum:   json.RawMessage(`"2026-12-31"`),
		},
		"absent": {ValueType: valueTypeDate, Maximum: json.RawMessage(`"2026-12-31"`)},
	}
	object := func(amount string) Detail {
		return Detail{Fields: map[string]json.RawMessage{"amount": json.RawMessage(amount)}}
	}
	s.NoError(CheckFieldLimits(policies, object(`"2026-12-31"`)))
	s.Error(CheckFieldLimits(policies, object(`"2027-01-01"`)), "above the maximum")
	s.Error(CheckFieldLimits(policies, object(`"2025-12-31"`)), "below the minimum")
	s.Error(CheckFieldLimits(policies, object(`"lots"`)), "not comparable")
}

func (s *PolicyTestSuite) TestCheckRequestedFields() {
	policies := FieldPolicies{"account": {SuppliedBy: PartyProcessor}}
	requested := Detail{Fields: map[string]json.RawMessage{"account": json.RawMessage(`{"iban":"DE12"}`)}}
	s.Error(CheckRequestedFields(policies, requested), "a client cannot send a field the processor supplies")
	_, err := CompileSchema(json.RawMessage(`{"type":"object","properties":{"a":{"type":"object",` +
		`"properties":{"b.c":{"type":"string"}}}}}`))
	s.Error(err, "a property name with a dot would make field paths ambiguous")

	other := Detail{Fields: map[string]json.RawMessage{"a": json.RawMessage(`1`)}}
	s.NoError(CheckRequestedFields(policies, other))
}

func (s *PolicyTestSuite) TestPolicies_UnreadableSchemaFixesEveryField() {
	s.Empty(Policies(json.RawMessage(`{`)))
}

func (s *PolicyTestSuite) TestPolicies_InfersValueTypes() {
	policies := Policies(json.RawMessage(`{"type":"object","properties":{
		"n":{"type":"number","maximum":10,"policy":{"modification":"decrease","modifiableBy":["user"]}},
		"d":{"type":"string","format":"decimal"},
		"day":{"type":"string","format":"date","policy":{"maximum":"2026-12-31"}},
		"at":{"type":"string","format":"date-time","policy":{"maximum":"2026-12-31T23:59:59Z"}},
		"text":{"type":"string","policy":{"suppliedBy":"processor"}}}}`))
	s.Require().NotEmpty(policies)
	s.Equal(valueTypeNumber, policies["n"].ValueType)
	s.Empty(policies["d"].ValueType, "text is never ordered as a number")
	s.Equal(valueTypeDate, policies["day"].ValueType)
	s.Equal(valueTypeDateTime, policies["at"].ValueType)
	s.Empty(policies["text"].ValueType)
}
