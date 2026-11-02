// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ModifyTestSuite struct {
	suite.Suite
}

func TestModifyTestSuite(t *testing.T) {
	suite.Run(t, new(ModifyTestSuite))
}

// modifyPolicies declares every kind of modification and a processor-supplied field.
func modifyPolicies() FieldPolicies {
	everyone := []string{PartyProcessor, PartyUser, PartyClient}
	return FieldPolicies{
		"cap.amount": {Modification: modificationDecrease, ModifiableBy: everyone, ValueType: valueTypeNumber},
		"minStay":    {Modification: modificationIncrease, ModifiableBy: everyone, ValueType: valueTypeNumber},
		"until":      {Modification: modificationDecrease, ModifiableBy: everyone, ValueType: valueTypeDate},
		"at":         {Modification: modificationDecrease, ModifiableBy: everyone, ValueType: valueTypeDateTime},
		"actions":    {Modification: modificationRemoveItems, ModifiableBy: []string{PartyUser, PartyClient}},
		"account":    {SuppliedBy: PartyProcessor},
	}
}

const modifyBase = `{"type":"trip","route":"CMB-SIN","cap":{"amount":450.00,"currency":"USD"},"minStay":2,
	"until":"2026-10-01","at":"2026-10-01T10:00:00Z","actions":["book","cancel"],"requiredChecks":["id"],
	"account":"A1"}`

func (s *ModifyTestSuite) object() Detail {
	fields := map[string]json.RawMessage{}
	s.Require().NoError(json.Unmarshal([]byte(modifyBase), &fields))
	return Detail{ID: "d-1", Type: "trip", Fields: fields}
}

// with returns the base detail with one field replaced, or removed when value is empty.
func (s *ModifyTestSuite) with(field, value string) Detail {
	obj := s.object()
	if value == "" {
		delete(obj.Fields, field)
	} else {
		obj.Fields[field] = json.RawMessage(value)
	}
	return obj
}

func (s *ModifyTestSuite) TestCheckModification_Accepts() {
	cases := map[string]Detail{
		"unchanged":              s.object(),
		"lower number":           s.with("cap", `{"amount":30,"currency":"USD"}`),
		"negative is lower":      s.with("cap", `{"amount":-1.5,"currency":"USD"}`),
		"equal number, new text": s.with("cap", `{"amount":450,"currency":"USD"}`),
		"higher integer (gte)":   s.with("minStay", `5`),
		"earlier date":           s.with("until", `"2026-09-30"`),
		"earlier datetime":       s.with("at", `"2026-10-01T09:59:59Z"`),
		"subset of actions":      s.with("actions", `["book"]`),
		"emptied actions":        s.with("actions", `[]`),
	}
	for name, candidate := range cases {
		s.NoError(CheckModification(s.object(), candidate, modifyPolicies(), PartyClient), name)
	}
}

func (s *ModifyTestSuite) TestCheckModification_Rejects() {
	cases := map[string]Detail{
		"higher number":          s.with("cap", `{"amount":450.01,"currency":"USD"}`),
		"number sent as text":    s.with("cap", `{"amount":"30","currency":"USD"}`),
		"changed currency":       s.with("cap", `{"amount":30,"currency":"EUR"}`),
		"lower integer (gte)":    s.with("minStay", `1`),
		"later date":             s.with("until", `"2026-10-02"`),
		"removed lte field":      s.with("minStay", ""),
		"non numeric amount":     s.with("cap", `{"amount":"lots","currency":"USD"}`),
		"date sent as a number":  s.with("until", `20261001`),
		"new action":             s.with("actions", `["book","refund"]`),
		"repeated action":        s.with("actions", `["book","book"]`),
		"dropped required check": s.with("requiredChecks", `[]`),
		"field without a rule":   s.with("route", `"CMB-DXB"`),
		"added field":            s.with("note", `"x"`),
		"changed type":           s.with("type", `"other"`),
		"changed server field":   s.with("account", `"A2"`),
		"actions not an array":   s.with("actions", `"book"`),
		"left out actions":       s.with("actions", ""),
		"invalid date":           s.with("until", `"soon"`),
	}
	for name, candidate := range cases {
		s.Error(CheckModification(s.object(), candidate, modifyPolicies(), PartyClient), name)
	}
}

func (s *ModifyTestSuite) TestCheckModification_AddItemsKeepsEveryGrantedItem() {
	policies := FieldPolicies{
		"excluded": {Modification: modificationAddItems, ModifiableBy: []string{PartyProcessor, PartyUser}},
	}
	base := s.with("excluded", `["gambling"]`)

	s.NoError(CheckModification(base, s.with("excluded", `["gambling","crypto"]`), policies, PartyUser),
		"adding an exclusion grants less")
	s.NoError(CheckModification(base, base, policies, PartyUser))
	s.Error(CheckModification(base, s.with("excluded", `["crypto"]`), policies, PartyUser),
		"dropping an exclusion widens the grant")
	s.Error(CheckModification(base, s.with("excluded", ""), policies, PartyUser),
		"removing the list drops every exclusion")
	s.Error(CheckModification(s.with("excluded", `["gambling","gambling"]`), base, policies, PartyUser),
		"each granted item must still be there as often as it was granted")
	s.Error(CheckModification(base, s.with("excluded", `["gambling","crypto"]`), policies, PartyClient),
		"only the parties the policy names may add items")
	s.NoError(CheckModification(s.with("excluded", ""), s.with("excluded", `["crypto"]`), policies, ""),
		"a request with more exclusions than an earlier approval stays within it")
}

func (s *ModifyTestSuite) TestCheckModification_ServerOwnedFieldsAreFree() {
	candidate := s.with("account", `"A2"`)
	s.NoError(CheckModification(s.object(), candidate, modifyPolicies(), PartyProcessor),
		"the processor that owns a server field may set it")
	s.Error(CheckModification(s.object(), s.with("route", `"X"`), modifyPolicies(), PartyProcessor),
		"the processor still may not change a field without a rule")
}

func (s *ModifyTestSuite) TestCompareOrderedUnsupported() {
	_, err := compareOrdered(json.RawMessage(`"1"`), json.RawMessage(`"2"`), "weight")
	s.Error(err)
	_, err = compareOrdered(json.RawMessage(`true`), json.RawMessage(`"2"`), valueTypeNumber)
	s.Error(err)
	_, err = compareOrdered(json.RawMessage(`"soon"`), json.RawMessage(`"2026-01-31"`), valueTypeDate)
	s.Error(err)
	_, err = compareOrdered(json.RawMessage(`"x"`), json.RawMessage(`"y"`), "integer")
	s.Error(err, "whole numbers are ordered as numbers, not as a type of their own")
	_, err = compareOrdered(json.RawMessage(`"1"`), json.RawMessage(`2`), valueTypeNumber)
	s.Error(err, "a number written as text is not a number")
}

func (s *ModifyTestSuite) TestCheckModification_OnlyPermittedPartiesMayModify() {
	fewer := s.with("actions", `["book"]`)
	s.NoError(CheckModification(s.object(), fewer, modifyPolicies(), PartyUser))
	s.Error(CheckModification(s.object(), fewer, modifyPolicies(), PartyProcessor),
		"the resource server is not among the parties that may remove actions")
	s.NoError(CheckModification(s.object(), fewer, modifyPolicies(), ""),
		"coverage checks the direction of a modification whoever may make it")
}

func (s *ModifyTestSuite) TestJSONEqual_ComparesNumbersByValue() {
	s.True(jsonEqual(json.RawMessage(`45`), json.RawMessage(`45.00`)))
	s.True(jsonEqual(json.RawMessage(`{"a":[1.5]}`), json.RawMessage(`{"a":[1.50]}`)))
	s.False(jsonEqual(json.RawMessage(`45`), json.RawMessage(`"45"`)), "a number is not a string")
	s.False(jsonEqual(json.RawMessage(`45`), json.RawMessage(`45.01`)))
	s.True(jsonEqual(json.RawMessage(`1e2`), json.RawMessage(`1e2`)), "exponent forms fall back to text")
}

func (s *ModifyTestSuite) TestCheckModification_NumericAmounts() {
	policies := FieldPolicies{"amount": {Modification: modificationDecrease,
		ModifiableBy: []string{PartyClient}, ValueType: valueTypeNumber}}
	obj := func(amount string) Detail {
		return Detail{Fields: map[string]json.RawMessage{"amount": json.RawMessage(amount)}}
	}
	s.NoError(CheckModification(obj(`45.00`), obj(`30`), policies, PartyClient))
	s.Error(CheckModification(obj(`45.00`), obj(`45.5`), policies, PartyClient))
}

func (s *ModifyTestSuite) TestSchemaEquivalent() {
	base := `{"type":"object","required":["type","b","a"],"properties":{"a":{"type":"string"},` +
		`"b":{"type":"number","policy":{"suppliedBy":"client","modification":"decrease",` +
		`"modifiableBy":["user","client"]}},"c":{"type":"object","properties":{}}}}`
	same := `{"type":"object","required":["a","b"],"properties":{"c":{"type":"object"},"b":{"type":"number",` +
		`"policy":{"modification":"decrease","modifiableBy":["client","user","user"]}},"a":{"type":"string"}}}`
	s.True(SchemaEquivalent(json.RawMessage(base), json.RawMessage(same)))

	for name, other := range map[string]string{
		"a field required no more": `{"type":"object","required":["a"],"properties":{"a":{"type":"string"},` +
			`"b":{"type":"number","policy":{"modification":"decrease","modifiableBy":["user","client"]}},` +
			`"c":{"type":"object"}}}`,
		"a party removed": `{"type":"object","required":["a","b"],"properties":{"a":{"type":"string"},` +
			`"b":{"type":"number","policy":{"modification":"decrease","modifiableBy":["user"]}},` +
			`"c":{"type":"object"}}}`,
		"a limit added": `{"type":"object","required":["a","b"],"properties":{"a":{"type":"string","maxLength":5},` +
			`"b":{"type":"number","policy":{"modification":"decrease","modifiableBy":["user","client"]}},` +
			`"c":{"type":"object"}}}`,
	} {
		s.False(SchemaEquivalent(json.RawMessage(base), json.RawMessage(other)), name)
	}
	s.False(SchemaEquivalent(json.RawMessage(`{`), json.RawMessage(`{}`)))
}
