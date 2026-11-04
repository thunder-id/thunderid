// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type VariablesNodeTestSuite struct {
	suite.Suite
}

func TestVariablesNodeTestSuite(t *testing.T) {
	suite.Run(t, new(VariablesNodeTestSuite))
}

func (s *VariablesNodeTestSuite) newNode(defs []providers.VariableDefinition) NodeInterface {
	properties, err := PropertiesFor(defs)
	s.Require().NoError(err)
	node := newVariablesNode("vars", properties, false, false)
	node.(RepresentationNodeInterface).SetOnSuccess("next")
	return node
}

func (s *VariablesNodeTestSuite) nodeContext() *providers.NodeContext {
	return &providers.NodeContext{Context: context.Background(), ExecutionID: "exec-1"}
}

func (s *VariablesNodeTestSuite) definitionsOf(
	properties map[string]interface{},
) ([]providers.VariableDefinition, error) {
	return newVariablesNode("vars", properties, false, false).(*variablesNode).definitions()
}

func (s *VariablesNodeTestSuite) runtimeValuesOf(defs []providers.VariableDefinition) (map[string]string, error) {
	return newVariablesNode("vars", nil, false, false).(*variablesNode).runtimeValues(defs)
}

func (s *VariablesNodeTestSuite) encodeRuntimeOf(dataType, raw string) (string, error) {
	return newVariablesNode("vars", nil, false, false).(*variablesNode).encodeRuntime(dataType, raw)
}

func (s *VariablesNodeTestSuite) TestNewVariablesNode() {
	node := newVariablesNode("vars", nil, true, false)

	s.Equal("vars", node.GetID())
	s.Equal(common.NodeTypeVariables, node.GetType())
	s.True(node.IsStartNode())
	s.False(node.IsFinalNode())
	s.NotNil(node.GetProperties())
}

func (s *VariablesNodeTestSuite) TestResolvesEveryVariableToItsDefault() {
	node := s.newNode([]providers.VariableDefinition{
		{Identifier: "supportEmail", Type: providers.InputTypeEmail, DefaultValue: "support@example.com"},
		{Identifier: "otpValiditySeconds", Type: providers.InputTypeNumber, DefaultValue: 120.0},
		{Identifier: "showRegistrationLink", Type: providers.InputTypeBoolean, DefaultValue: true},
	})

	resp, err := node.Execute(s.nodeContext())

	s.Nil(err)
	s.Equal(common.NodeStatusComplete, resp.Status)
	s.Equal("next", resp.NextNodeID)
	s.Equal(map[string]string{
		"var.supportEmail":         `"support@example.com"`,
		"var.otpValiditySeconds":   "120",
		"var.showRegistrationLink": "true",
	}, resp.RuntimeData)
}

func (s *VariablesNodeTestSuite) TestAFlowWithNoVariablesResolvesNothing() {
	resp, err := s.newNode(nil).Execute(s.nodeContext())

	s.Nil(err)
	s.Equal(common.NodeStatusComplete, resp.Status)
	s.Empty(resp.RuntimeData)
}

func (s *VariablesNodeTestSuite) TestFailsWhenTheDeclarationsAreNotReadable() {
	node := newVariablesNode("vars", map[string]interface{}{propertyVariables: "not a list"}, false, false)

	resp, err := node.Execute(s.nodeContext())

	s.Nil(resp)
	s.NotNil(err)
}

func (s *VariablesNodeTestSuite) TestFailsWhenADefaultIsNotReadable() {
	node := s.newNode([]providers.VariableDefinition{
		{Identifier: "limit", Type: providers.InputTypeNumber, DefaultValue: []string{"x"}},
	})

	resp, err := node.Execute(s.nodeContext())

	s.Nil(resp)
	s.NotNil(err)
}

const testIdentityProviderCategory = "IDENTITY_PROVIDER"

func (s *VariablesNodeTestSuite) TestVariableRuntimeValuesReplaceTheDefaultsTheUnitHoldsValuesFor() {
	node := s.newNode([]providers.VariableDefinition{
		{Identifier: "supportEmail", Type: providers.InputTypeEmail, DefaultValue: "support@example.com"},
		{Identifier: "otp", Type: providers.InputTypeNumber, DefaultValue: 120.0},
		{Identifier: "showLink", Type: providers.InputTypeBoolean, DefaultValue: true},
		{Identifier: "mfa", Type: providers.InputTypeSelect, DefaultValue: "NONE",
			Options: []interface{}{"NONE", "SMS_OTP"}},
	})

	values, err := VariableRuntimeValues(node, map[string]string{
		"supportEmail": "help@emea.example", "otp": "90", "showLink": "false", "mfa": "SMS_OTP",
	})

	s.Require().NoError(err)
	s.Equal(`"help@emea.example"`, values["var.supportEmail"])
	s.Equal("90", values["var.otp"])
	s.Equal("false", values["var.showLink"])
	s.Equal(`"SMS_OTP"`, values["var.mfa"])
}

func (s *VariablesNodeTestSuite) TestVariableRuntimeValuesLeaveTheRestAtTheirDefaults() {
	node := s.newNode([]providers.VariableDefinition{
		{Identifier: "supportEmail", Type: providers.InputTypeEmail, DefaultValue: "support@example.com"},
		{Identifier: "otp", Type: providers.InputTypeNumber, DefaultValue: 120.0},
	})

	values, err := VariableRuntimeValues(node, map[string]string{"otp": "60", "undeclared": "x"})

	s.Require().NoError(err)
	s.Equal(`"support@example.com"`, values["var.supportEmail"])
	s.Equal("60", values["var.otp"])
	s.NotContains(values, "var.undeclared")
}

func (s *VariablesNodeTestSuite) TestVariableRuntimeValuesStartFromTheDefaultsForEveryUnit() {
	node := s.newNode([]providers.VariableDefinition{
		{Identifier: "otp", Type: providers.InputTypeNumber, DefaultValue: 120.0},
	})

	first, err := VariableRuntimeValues(node, map[string]string{"otp": "60"})
	s.Require().NoError(err)
	second, err := VariableRuntimeValues(node, nil)
	s.Require().NoError(err)

	s.Equal("60", first["var.otp"])
	s.Equal("120", second["var.otp"], "what one unit held does not carry over to the next")
}

func (s *VariablesNodeTestSuite) TestVariableRuntimeValuesRefuseAValueOfTheWrongType() {
	node := s.newNode([]providers.VariableDefinition{
		{Identifier: "otp", Type: providers.InputTypeNumber, DefaultValue: 120.0},
	})

	_, err := VariableRuntimeValues(node, map[string]string{"otp": "many"})

	s.Error(err)
}

func (s *VariablesNodeTestSuite) TestVariableRuntimeValuesIgnoreOtherNodes() {
	values, err := VariableRuntimeValues(&node{}, map[string]string{"a": "1"})
	s.NoError(err)
	s.Empty(values)
}

func (s *VariablesNodeTestSuite) TestUnsetConnectionVariablesNamesTheConnectionsStillWithoutOne() {
	node := s.newNode([]providers.VariableDefinition{
		{Identifier: "social", Type: providers.InputTypeConnectionSelect, Category: testIdentityProviderCategory,
			DefaultValue: ""},
		{Identifier: "other", Type: providers.InputTypeConnectionSelect, Category: testIdentityProviderCategory,
			DefaultValue: "c-owner"},
		{Identifier: "brand", Type: providers.InputTypeText, DefaultValue: ""},
	})
	resp, err := node.Execute(s.nodeContext())
	s.Require().Nil(err)

	s.Equal([]string{"social"}, UnsetConnectionVariables(node, resp.RuntimeData))

	held, runErr := VariableRuntimeValues(node, map[string]string{"social": "c-child"})
	s.Require().NoError(runErr)
	s.Empty(UnsetConnectionVariables(node, held), "a unit's own connection sets it")
}

func (s *VariablesNodeTestSuite) TestUnsetConnectionVariablesIgnoresOtherNodes() {
	s.Empty(UnsetConnectionVariables(newVariablesNode("x", nil, false, false), map[string]string{}))
	s.Empty(UnsetConnectionVariables(&node{}, map[string]string{}))
}

func (s *VariablesNodeTestSuite) TestCloneKeepsTheOnSuccess() {
	node := s.newNode(nil)

	clone, err := newFlowFactory().CloneNode(node)

	s.Require().NoError(err)
	s.Equal(common.NodeTypeVariables, clone.GetType())
	s.Equal("next", clone.(RepresentationNodeInterface).GetOnSuccess())
}

func floatPtr(v float64) *float64 { return &v }

func rule(t string, v interface{}) providers.ValidationRuleDefinition {
	return providers.ValidationRuleDefinition{Type: t, Value: v}
}

func (s *VariablesNodeTestSuite) TestParseDefinitions() {
	s.Run("no variables property", func() {
		defs, err := s.definitionsOf(map[string]interface{}{})
		s.Require().NoError(err)
		s.Empty(defs)
	})
	s.Run("reads the declarations", func() {
		defs, err := s.definitionsOf(map[string]interface{}{
			propertyVariables: []interface{}{
				map[string]interface{}{
					"identifier": "allowSignup", "label": "Allow sign-up",
					"type": "BOOLEAN_INPUT", "defaultValue": false,
				},
			},
		})
		s.Require().NoError(err)
		s.Require().Len(defs, 1)
		s.Equal("allowSignup", defs[0].Identifier)
		s.Equal(providers.InputTypeBoolean, defs[0].Type)
		s.Equal(false, defs[0].DefaultValue)
	})
	s.Run("rejects a malformed property", func() {
		_, err := s.definitionsOf(map[string]interface{}{propertyVariables: "nope"})
		s.Require().Error(err)
	})
	s.Run("properties carry the declarations back out", func() {
		in := []providers.VariableDefinition{{
			Identifier: "brand", Type: providers.InputTypeText, DefaultValue: "acme",
			Validation: []providers.ValidationRuleDefinition{rule("maxLength", 8)},
		}}
		props, err := PropertiesFor(in)
		s.Require().NoError(err)
		out, err := s.definitionsOf(props)
		s.Require().NoError(err)
		s.Require().Len(out, 1)
		s.Equal("brand", out[0].Identifier)
		s.Require().Len(out[0].Validation, 1)
	})
}

func (s *VariablesNodeTestSuite) TestValidateDefinitions() {
	valid := func() providers.VariableDefinition {
		return providers.VariableDefinition{Identifier: "brand", Type: providers.InputTypeText, DefaultValue: "acme"}
	}
	cases := []struct {
		name    string
		mutate  func(*providers.VariableDefinition)
		wantErr string
	}{
		{"valid text", func(*providers.VariableDefinition) {}, ""},
		{"bad identifier", func(d *providers.VariableDefinition) { d.Identifier = "1bad" }, "is not valid"},
		{"unknown type", func(d *providers.VariableDefinition) { d.Type = "NOPE" }, "not supported"},
		{"missing default", func(d *providers.VariableDefinition) { d.DefaultValue = nil }, "no default"},
		{"default of the wrong type", func(d *providers.VariableDefinition) { d.DefaultValue = true }, "its default"},
		{"rules on a number", func(d *providers.VariableDefinition) {
			d.Type = providers.InputTypeNumber
			d.DefaultValue = 1.0
			d.Validation = []providers.ValidationRuleDefinition{rule("regex", "x")}
		}, "text, email or phone"},
		{"bad regex", func(d *providers.VariableDefinition) {
			d.Validation = []providers.ValidationRuleDefinition{rule("regex", "(")}
		}, "not a valid expression"},
		{"default off the regex", func(d *providers.VariableDefinition) {
			d.Validation = []providers.ValidationRuleDefinition{rule("regex", "^[0-9]+$")}
		}, "does not match"},
		{"unknown rule", func(d *providers.VariableDefinition) {
			d.Validation = []providers.ValidationRuleDefinition{rule("startsWith", "a")}
		}, "not supported"},
		{"length rules reversed", func(d *providers.VariableDefinition) {
			d.Validation = []providers.ValidationRuleDefinition{rule("minLength", 5), rule("maxLength", 2)}
		}, "maximum length is below"},
		{"number bounds reversed", func(d *providers.VariableDefinition) {
			d.Type = providers.InputTypeNumber
			d.Min, d.Max = floatPtr(5), floatPtr(1)
			d.DefaultValue = 3.0
		}, "maximum is below"},
		{"bound on text", func(d *providers.VariableDefinition) { d.Min = floatPtr(1) }, "number input only"},
		{"select with no options", func(d *providers.VariableDefinition) {
			d.Type = providers.InputTypeSelect
		}, "at least one option"},
		{"select default not an option", func(d *providers.VariableDefinition) {
			d.Type = providers.InputTypeSelect
			d.Options = []interface{}{"a"}
			d.DefaultValue = "b"
		}, "not one of the options"},
		{"duplicate option", func(d *providers.VariableDefinition) {
			d.Type = providers.InputTypeSelect
			d.Options = []interface{}{"a", "a"}
			d.DefaultValue = "a"
		}, "more than once"},
		{"options on text", func(d *providers.VariableDefinition) { d.Options = []interface{}{"a"} }, "select only"},
		{"connection without a category", func(d *providers.VariableDefinition) {
			d.Type = providers.InputTypeConnectionSelect
			d.DefaultValue = "conn-1"
		}, "names the category"},
		{"category on text", func(d *providers.VariableDefinition) {
			d.Category = testIdentityProviderCategory
		}, "connection input only"},
		{"connection that starts with no connection", func(d *providers.VariableDefinition) {
			d.Type = providers.InputTypeConnectionSelect
			d.Category = testIdentityProviderCategory
			d.DefaultValue = ""
		}, ""},
		{"blank text default is still a text default", func(d *providers.VariableDefinition) {
			d.DefaultValue = ""
		}, ""},
		{"valid connection", func(d *providers.VariableDefinition) {
			d.Type = providers.InputTypeConnectionSelect
			d.Category = testIdentityProviderCategory
			d.ConnectionTypes = []string{"google"}
			d.DefaultValue = "conn-1"
		}, ""},
		{"select of booleans", func(d *providers.VariableDefinition) {
			d.Type = providers.InputTypeSelect
			d.Options = []interface{}{true, false}
			d.DefaultValue = true
		}, ""},
		{"select of numbers", func(d *providers.VariableDefinition) {
			d.Type = providers.InputTypeSelect
			d.Options = []interface{}{1.0, 2.5}
			d.DefaultValue = 2.5
		}, ""},
		{"select of mixed values", func(d *providers.VariableDefinition) {
			d.Type = providers.InputTypeSelect
			d.Options = []interface{}{"apple", 1.0, true}
			d.DefaultValue = 1.0
		}, ""},
		{"select option that is not a scalar", func(d *providers.VariableDefinition) {
			d.Type = providers.InputTypeSelect
			d.Options = []interface{}{[]interface{}{"a"}}
			d.DefaultValue = "a"
		}, "not a string, a number or a boolean"},
		{"select options that read the same", func(d *providers.VariableDefinition) {
			d.Type = providers.InputTypeSelect
			d.Options = []interface{}{"1", 1.0}
			d.DefaultValue = "1"
		}, "more than once"},
		{"boolean select default given as text", func(d *providers.VariableDefinition) {
			d.Type = providers.InputTypeSelect
			d.Options = []interface{}{true, false}
			d.DefaultValue = "true"
		}, "not one of the options"},
		{"valid boolean", func(d *providers.VariableDefinition) {
			d.Type = providers.InputTypeBoolean
			d.DefaultValue = true
		}, ""},
	}
	for _, c := range cases {
		s.Run(c.name, func() {
			def := valid()
			c.mutate(&def)
			err := ValidateDefinitions([]providers.VariableDefinition{def})
			if c.wantErr == "" {
				s.Require().NoError(err)
				return
			}
			s.Require().Error(err)
			s.Contains(err.Error(), c.wantErr)
		})
	}

	s.Run("duplicate identifiers", func() {
		err := ValidateDefinitions([]providers.VariableDefinition{valid(), valid()})
		s.Require().Error(err)
		s.Contains(err.Error(), "more than once")
	})
}

func (s *VariablesNodeTestSuite) TestValidateRaw() {
	def := func(t string, mutate func(*providers.VariableDefinition)) providers.VariableDefinition {
		d := providers.VariableDefinition{Identifier: "p", Type: t}
		if mutate != nil {
			mutate(&d)
		}
		return d
	}
	cases := []struct {
		name string
		def  providers.VariableDefinition
		raw  string
		ok   bool
	}{
		{"text ok", def(providers.InputTypeText, nil), "hello", true},
		{"text too long", def(providers.InputTypeText, func(d *providers.VariableDefinition) {
			d.Validation = []providers.ValidationRuleDefinition{rule("maxLength", 3)}
		}), "hello", false},
		{"text too short", def(providers.InputTypeText, func(d *providers.VariableDefinition) {
			d.Validation = []providers.ValidationRuleDefinition{rule("minLength", 10)}
		}), "hello", false},
		{"text regex ok", def(providers.InputTypeText, func(d *providers.VariableDefinition) {
			d.Validation = []providers.ValidationRuleDefinition{rule("regex", "^[a-z]+$")}
		}), "abc", true},
		{"text regex is matched as written", def(providers.InputTypeText, func(d *providers.VariableDefinition) {
			d.Validation = []providers.ValidationRuleDefinition{rule("regex", "[a-z]+")}
		}), "abc1", true},
		{"text regex anchored", def(providers.InputTypeText, func(d *providers.VariableDefinition) {
			d.Validation = []providers.ValidationRuleDefinition{rule("regex", "^[a-z]+$")}
		}), "abc1", false},
		{"email ok", def(providers.InputTypeEmail, nil), "a@b.io", true},
		{"email with a display name", def(providers.InputTypeEmail, nil), "A <a@b.io>", false},
		{"email without domain", def(providers.InputTypeEmail, nil), "nope", false},
		{"phone ok", def(providers.InputTypePhone, nil), "+94 77 123 4567", true},
		{"phone letters", def(providers.InputTypePhone, nil), "call me", false},
		{"number ok", def(providers.InputTypeNumber, func(d *providers.VariableDefinition) {
			d.Min, d.Max = floatPtr(1), floatPtr(10)
		}), "5", true},
		{"number below min", def(providers.InputTypeNumber, func(d *providers.VariableDefinition) {
			d.Min = floatPtr(1)
		}), "0", false},
		{"number above max", def(providers.InputTypeNumber, func(d *providers.VariableDefinition) {
			d.Max = floatPtr(10)
		}), "11", false},
		{"number not numeric", def(providers.InputTypeNumber, nil), "1234x", false},
		{"date ok", def(providers.InputTypeDate, nil), "2026-10-22", true},
		{"date wrong form", def(providers.InputTypeDate, nil), "22/10/2026", false},
		{"select ok", def(providers.InputTypeSelect, func(d *providers.VariableDefinition) {
			d.Options = []interface{}{"a", "b"}
		}), "b", true},
		{"select unknown", def(providers.InputTypeSelect, func(d *providers.VariableDefinition) {
			d.Options = []interface{}{"a"}
		}), "z", false},
		{"boolean true", def(providers.InputTypeBoolean, nil), "true", true},
		{"boolean number", def(providers.InputTypeBoolean, nil), "1234", false},
		{"connection id", def(providers.InputTypeConnectionSelect, nil), "conn-1", true},
		{"connection empty", def(providers.InputTypeConnectionSelect, nil), " ", false},
	}
	for _, c := range cases {
		s.Run(c.name, func() {
			err := validateRaw(c.def, c.raw)
			if c.ok {
				s.Require().NoError(err)
			} else {
				s.Require().Error(err)
			}
		})
	}
}

func (s *VariablesNodeTestSuite) TestRawFromJSONValue() {
	raw, err := rawFromJSONValue(variableDataTypeBoolean, true)
	s.Require().NoError(err)
	s.Equal("true", raw)

	_, err = rawFromJSONValue(variableDataTypeBoolean, "true")
	s.Require().Error(err, "a boolean takes a boolean, not text")

	raw, err = rawFromJSONValue(variableDataTypeNumber, 42.0)
	s.Require().NoError(err)
	s.Equal("42", raw)

	_, err = rawFromJSONValue(variableDataTypeString, 1.0)
	s.Require().Error(err)
}

func (s *VariablesNodeTestSuite) TestEncodeRuntimeAndDefaults() {
	enc, err := s.encodeRuntimeOf(variableDataTypeBoolean, "true")
	s.Require().NoError(err)
	s.Equal("true", enc)

	enc, err = s.encodeRuntimeOf(variableDataTypeNumber, "3.50")
	s.Require().NoError(err)
	s.Equal("3.5", enc)

	enc, err = s.encodeRuntimeOf(variableDataTypeString, `a "quoted" value`)
	s.Require().NoError(err)
	s.Equal(`"a \"quoted\" value"`, enc)

	values, err := s.runtimeValuesOf([]providers.VariableDefinition{
		{Identifier: "allowSignup", Type: providers.InputTypeBoolean, DefaultValue: false},
		{Identifier: "brand", Type: providers.InputTypeText, DefaultValue: "acme"},
	})
	s.Require().NoError(err)
	s.Equal(map[string]string{"var.allowSignup": "false", "var.brand": `"acme"`}, values)
}

func (s *VariablesNodeTestSuite) TestDataTypeOf() {
	sel := func(options ...interface{}) providers.VariableDefinition {
		return providers.VariableDefinition{Type: providers.InputTypeSelect, Options: options}
	}
	cases := []struct {
		name string
		def  providers.VariableDefinition
		want string
	}{
		{"text", providers.VariableDefinition{Type: providers.InputTypeText}, variableDataTypeString},
		{"email", providers.VariableDefinition{Type: providers.InputTypeEmail}, variableDataTypeString},
		{"phone", providers.VariableDefinition{Type: providers.InputTypePhone}, variableDataTypeString},
		{"date", providers.VariableDefinition{Type: providers.InputTypeDate}, variableDataTypeString},
		{"connection", providers.VariableDefinition{Type: providers.InputTypeConnectionSelect},
			variableDataTypeString},
		{"number", providers.VariableDefinition{Type: providers.InputTypeNumber}, variableDataTypeNumber},
		{"boolean", providers.VariableDefinition{Type: providers.InputTypeBoolean}, variableDataTypeBoolean},
		{"select of strings", sel("apple", "orange"), variableDataTypeString},
		{"select of booleans", sel(true, false), variableDataTypeBoolean},
		{"select of numbers", sel(1.0, 2.5), variableDataTypeNumber},
		{"select of mixed values", sel("apple", "orange", 1.0, true), variableDataTypeString},
		{"select of numbers and booleans", sel(1.0, true), variableDataTypeString},
	}
	for _, c := range cases {
		s.Run(c.name, func() { s.Equal(c.want, dataTypeOf(c.def)) })
	}
}

func (s *VariablesNodeTestSuite) TestSelectResolvesToTheTypeOfItsOptions() {
	values, err := s.runtimeValuesOf([]providers.VariableDefinition{
		{Identifier: "flag", Type: providers.InputTypeSelect, Options: []interface{}{true, false}, DefaultValue: false},
		{Identifier: "tier", Type: providers.InputTypeSelect, Options: []interface{}{1.0, 2.0}, DefaultValue: 2.0},
		{Identifier: "fruit", Type: providers.InputTypeSelect,
			Options: []interface{}{"apple", "orange"}, DefaultValue: "apple"},
		{Identifier: "mixed", Type: providers.InputTypeSelect,
			Options: []interface{}{"apple", 1.0, true}, DefaultValue: 1.0},
	})
	s.Require().NoError(err)
	s.Equal("false", values[runtimeKey("flag")])
	s.Equal("2", values[runtimeKey("tier")])
	s.Equal(`"apple"`, values[runtimeKey("fruit")])
	s.Equal(`"1"`, values[runtimeKey("mixed")])

	def := providers.VariableDefinition{Type: providers.InputTypeSelect, Options: []interface{}{true, false}}
	s.Require().NoError(validateRaw(def, "true"))
	s.Require().Error(validateRaw(def, "maybe"))
}

func (s *VariablesNodeTestSuite) TestReferences() {
	props := map[string]interface{}{
		"idpId":  "{{var(idpConnection)}}",
		"nested": map[string]interface{}{"a": []interface{}{"x {{ var( brand ) }} y", 3}},
		"other":  "{{ctx(key)}}",
		"again":  "{{var(idpConnection)}}",
	}
	s.Equal([]string{"brand", "idpConnection"}, References(props))
	s.Empty(References("plain"))
}

func (s *VariablesNodeTestSuite) TestResolve() {
	values := map[string]string{
		"var.allowSignup":   "true",
		"var.limit":         "42",
		"var.idpConnection": `"conn-1"`,
		"var.brand":         `"acme"`,
	}

	s.Run("a whole placeholder keeps the value's type", func() {
		s.Equal(true, resolve("{{var(allowSignup)}}", values))
		s.Equal(float64(42), resolve("{{var(limit)}}", values))
		s.Equal("conn-1", resolve("{{var(idpConnection)}}", values))
	})
	s.Run("a placeholder inside text becomes its text", func() {
		s.Equal("Sign in to acme (42)", resolve("Sign in to {{var(brand)}} ({{var(limit)}})", values))
	})
	s.Run("a placeholder with no value is left as it is", func() {
		s.Equal("{{var(missing)}}", resolve("{{var(missing)}}", values))
		s.Equal("a {{var(missing)}}", resolve("a {{var(missing)}}", values))
	})
	s.Run("walks maps and lists and leaves the input alone", func() {
		in := map[string]interface{}{
			"idpId": "{{var(idpConnection)}}",
			"list":  []interface{}{"{{var(allowSignup)}}", "keep"},
			"n":     7,
		}
		out := resolve(in, values).(map[string]interface{})
		s.Equal("conn-1", out["idpId"])
		s.Equal([]interface{}{true, "keep"}, out["list"])
		s.Equal(7, out["n"])
		s.Equal("{{var(idpConnection)}}", in["idpId"], "the original is not changed")
	})
	s.Run("other placeholders are untouched", func() {
		s.Equal("{{ctx(ouId)}}", resolve("{{ctx(ouId)}}", values))
	})
}
