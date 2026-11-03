// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Data types of the value a variable resolves to.
const (
	variableDataTypeString  = "STRING"
	variableDataTypeNumber  = "NUMBER"
	variableDataTypeBoolean = "BOOLEAN"
)

// maxTextLength bounds a text value that declares no length of its own.
const maxTextLength = 1024

var (
	idPattern    = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,63}$`)
	phonePattern = regexp.MustCompile(`^\+?[0-9][0-9 ()-]{4,18}[0-9]$`)
)

// PropertiesFor is the properties a VARIABLES node carries in the graph: the declarations, in the form properties are
// held in.
func PropertiesFor(defs []providers.VariableDefinition) (map[string]interface{}, error) {
	data, err := json.Marshal(defs)
	if err != nil {
		return nil, err
	}
	var generic []interface{}
	if err := json.Unmarshal(data, &generic); err != nil {
		return nil, err
	}
	return map[string]interface{}{propertyVariables: generic}, nil
}

// isTextual reports whether a type holds free text, which the validation rules of a prompt input apply to.
func isTextual(t string) bool {
	return t == providers.InputTypeText || t == providers.InputTypeEmail || t == providers.InputTypePhone
}

// ValidateDefinitions checks a set of declarations: identifiers are well formed and unique, every type is known and
// consistently configured, and every variable has a default that is a valid value of it.
func ValidateDefinitions(defs []providers.VariableDefinition) error {
	seen := make(map[string]bool, len(defs))
	for _, def := range defs {
		if !idPattern.MatchString(def.Identifier) {
			return fmt.Errorf("variable %q is not valid: the identifier starts with a letter and holds letters, "+
				"digits and underscores", def.Identifier)
		}
		if seen[def.Identifier] {
			return fmt.Errorf("variable %q is declared more than once", def.Identifier)
		}
		seen[def.Identifier] = true

		if err := validateShape(def); err != nil {
			return fmt.Errorf("variable %q: %w", def.Identifier, err)
		}
		if def.DefaultValue == nil {
			return fmt.Errorf("variable %q has no default", def.Identifier)
		}
		raw, err := DefaultRaw(def)
		if err != nil {
			return fmt.Errorf("variable %q: its default %w", def.Identifier, err)
		}
		if def.Type == providers.InputTypeConnectionSelect && raw == "" {
			// A connection variable may start with no connection, which stops the flow for an organization unit that
			// has not set one.
			continue
		}
		if err := validateRaw(def, raw); err != nil {
			return fmt.Errorf("variable %q: its default %w", def.Identifier, err)
		}
	}
	return nil
}

func validateShape(def providers.VariableDefinition) error {
	switch def.Type {
	case providers.InputTypeText, providers.InputTypeEmail, providers.InputTypePhone, providers.InputTypeNumber,
		providers.InputTypeDate, providers.InputTypeBoolean, providers.InputTypeSelect,
		providers.InputTypeConnectionSelect:
	default:
		return fmt.Errorf("input type %q is not supported", def.Type)
	}
	if err := validateRules(def); err != nil {
		return err
	}
	if (def.Min != nil || def.Max != nil) && def.Type != providers.InputTypeNumber {
		return errors.New("a bound applies to a number input only")
	}
	if def.Min != nil && def.Max != nil && *def.Max < *def.Min {
		return errors.New("the maximum is below the minimum")
	}
	if def.Type == providers.InputTypeSelect {
		if len(def.Options) == 0 {
			return errors.New("a select needs at least one option")
		}
		values := make(map[string]bool, len(def.Options))
		for _, o := range def.Options {
			raw, err := rawFromScalar(o)
			if err != nil {
				return fmt.Errorf("the option %v %w", o, err)
			}
			if raw == "" {
				return errors.New("a select option needs a value")
			}
			if values[raw] {
				return fmt.Errorf("the option %v is listed more than once", o)
			}
			values[raw] = true
		}
	} else if len(def.Options) > 0 {
		return errors.New("options apply to a select only")
	}
	if def.Type == providers.InputTypeConnectionSelect {
		if def.Category == "" {
			return errors.New("a connection input names the category of connection it takes")
		}
	} else if def.Category != "" || len(def.ConnectionTypes) > 0 {
		return errors.New("a category and connection types apply to a connection input only")
	}
	return nil
}

// rawFromScalar is the raw string form of a select option or of the default of a select: a boolean is "true" or
// "false", a number its decimal form, a string itself. Nothing else can be one.
func rawFromScalar(v interface{}) (string, error) {
	switch n := v.(type) {
	case string:
		return n, nil
	case bool:
		return strconv.FormatBool(n), nil
	case float64:
		return strconv.FormatFloat(n, 'f', -1, 64), nil
	case int:
		return strconv.Itoa(n), nil
	default:
		return "", errors.New("is not a string, a number or a boolean")
	}
}

// selectDefaultRaw is the raw form of a select's default, which has to be one of the options as the option is written:
// a boolean option is matched by a boolean, never by the text that reads as one.
func selectDefaultRaw(def providers.VariableDefinition) (string, error) {
	raw, err := rawFromScalar(def.DefaultValue)
	if err != nil {
		return "", err
	}
	for _, o := range def.Options {
		if optionRaw, err := rawFromScalar(o); err == nil && optionRaw == raw &&
			fmt.Sprintf("%T", o) == fmt.Sprintf("%T", def.DefaultValue) {
			return raw, nil
		}
	}
	return "", errors.New("is not one of the options")
}

// validateRules checks the validation rules of a prompt input a variable carries. It is stricter than a prompt: a
// rule that no value could meet, or that would be ignored, is rejected when the flow is saved.
func validateRules(def providers.VariableDefinition) error {
	if len(def.Validation) > 0 && !isTextual(def.Type) {
		return errors.New("validation rules apply to a text, email or phone input only")
	}
	var minLen, maxLen *float64
	for _, rule := range def.Validation {
		switch providers.ValidationType(rule.Type) {
		case providers.ValidationTypeRegex:
			pattern, ok := rule.Value.(string)
			if !ok || pattern == "" {
				return errors.New("a regex rule needs a pattern")
			}
			if _, err := regexp.Compile(pattern); err != nil {
				return fmt.Errorf("the pattern is not a valid expression: %w", err)
			}
		case providers.ValidationTypeMinLength, providers.ValidationTypeMaxLength:
			n, ok := utils.ToFloat64(rule.Value)
			if !ok || n < 0 {
				return fmt.Errorf("a %s rule needs a length of zero or more", rule.Type)
			}
			if providers.ValidationType(rule.Type) == providers.ValidationTypeMinLength {
				minLen = &n
			} else {
				maxLen = &n
			}
		default:
			return fmt.Errorf("the validation rule %q is not supported", rule.Type)
		}
	}
	if minLen != nil && maxLen != nil && *maxLen < *minLen {
		return errors.New("the maximum length is below the minimum")
	}
	return nil
}

// rawFromJSONValue turns a value read from JSON (a bool, a number or a string) into the raw string form values are
// stored and compared in, for a value of the given data type. A boolean is "true" or "false", a number is its decimal
// form, a string is itself.
func rawFromJSONValue(dataType string, v interface{}) (string, error) {
	switch dataType {
	case variableDataTypeBoolean:
		b, ok := v.(bool)
		if !ok {
			return "", errors.New("is not a true or false value")
		}
		return strconv.FormatBool(b), nil
	case variableDataTypeNumber:
		switch n := v.(type) {
		case float64:
			return strconv.FormatFloat(n, 'f', -1, 64), nil
		case int:
			return strconv.Itoa(n), nil
		case string:
			return n, nil
		default:
			return "", errors.New("is not a number")
		}
	default:
		s, ok := v.(string)
		if !ok {
			return "", errors.New("is not a string")
		}
		return s, nil
	}
}

// validateRaw checks a raw value against what a variable allows. A connection value is only checked to be an
// identifier here: that the connection exists, is the right kind and belongs to the organization unit setting it is
// checked where the connections can be read.
func validateRaw(def providers.VariableDefinition, raw string) error {
	switch def.Type {
	case providers.InputTypeText:
		return validateText(def, raw)
	case providers.InputTypeEmail:
		addr, err := mail.ParseAddress(raw)
		if err != nil || addr.Address != raw || !strings.Contains(raw, "@") {
			return errors.New("is not an email address")
		}
		return validateText(def, raw)
	case providers.InputTypePhone:
		if !phonePattern.MatchString(raw) {
			return errors.New("is not a phone number")
		}
		return validateText(def, raw)
	case providers.InputTypeNumber:
		n, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return errors.New("is not a number")
		}
		if def.Min != nil && n < *def.Min {
			return fmt.Errorf("is below the minimum of %s", strconv.FormatFloat(*def.Min, 'f', -1, 64))
		}
		if def.Max != nil && n > *def.Max {
			return fmt.Errorf("is above the maximum of %s", strconv.FormatFloat(*def.Max, 'f', -1, 64))
		}
	case providers.InputTypeDate:
		if _, err := time.Parse("2006-01-02", raw); err != nil {
			return errors.New("is not a date in the form YYYY-MM-DD")
		}
	case providers.InputTypeSelect:
		for _, o := range def.Options {
			if optionRaw, err := rawFromScalar(o); err == nil && optionRaw == raw {
				return nil
			}
		}
		return errors.New("is not one of the options")
	case providers.InputTypeBoolean:
		if raw != "true" && raw != "false" {
			return errors.New("is not true or false")
		}
	case providers.InputTypeConnectionSelect:
		if strings.TrimSpace(raw) == "" {
			return errors.New("is not a connection id")
		}
	default:
		return fmt.Errorf("has an unsupported input type %q", def.Type)
	}
	return nil
}

// validateText applies the validation rules of a prompt input to a text value, through the same check a prompt uses.
// A text value that carries no length rule is bounded by maxTextLength.
func validateText(def providers.VariableDefinition, raw string) error {
	rules := make([]providers.ValidationRule, len(def.Validation))
	for i, r := range def.Validation {
		rules[i] = providers.ValidationRule{Type: providers.ValidationType(r.Type), Value: r.Value}
	}
	if err := common.PrepareValidationRules(rules); err != nil {
		return errors.New("does not match the pattern")
	}

	hasMaxLength := false
	for _, rule := range rules {
		if validateInput(rule, raw) {
			hasMaxLength = hasMaxLength || rule.Type == providers.ValidationTypeMaxLength
			continue
		}
		n, _ := utils.ToFloat64(rule.Value)
		switch rule.Type {
		case providers.ValidationTypeMinLength:
			return fmt.Errorf("is shorter than %d characters", int(n))
		case providers.ValidationTypeMaxLength:
			return fmt.Errorf("is longer than %d characters", int(n))
		default:
			return errors.New("does not match the pattern")
		}
	}
	if !hasMaxLength && utf8.RuneCountInString(raw) > maxTextLength {
		return fmt.Errorf("is longer than %d characters", maxTextLength)
	}
	return nil
}

// DefaultRaw is the raw form of a variable's default.
func DefaultRaw(def providers.VariableDefinition) (string, error) {
	if def.Type == providers.InputTypeSelect {
		return selectDefaultRaw(def)
	}
	return rawFromJSONValue(dataTypeOf(def), def.DefaultValue)
}

// dataTypeOf is the data type of the value a variable resolves to. Every input type but a select fixes it: text,
// email, phone, date and connection inputs hold a string, a number input a number and a boolean input a boolean. A
// select takes it from its options: all booleans make it a boolean, all numbers a number, and anything else, a mix
// included, a string.
func dataTypeOf(def providers.VariableDefinition) string {
	switch def.Type {
	case providers.InputTypeNumber:
		return variableDataTypeNumber
	case providers.InputTypeBoolean:
		return variableDataTypeBoolean
	case providers.InputTypeSelect:
		return optionsDataType(def.Options)
	}
	return variableDataTypeString
}

func optionsDataType(options []interface{}) string {
	if len(options) == 0 {
		return variableDataTypeString
	}
	dataType := ""
	for _, o := range options {
		var t string
		switch o.(type) {
		case bool:
			t = variableDataTypeBoolean
		case float64, int:
			t = variableDataTypeNumber
		default:
			return variableDataTypeString
		}
		if dataType != "" && dataType != t {
			return variableDataTypeString
		}
		dataType = t
	}
	return dataType
}

// runtimeKey is the runtime data key a variable's value is kept under.
func runtimeKey(id string) string {
	return runtimeKeyPrefix + id
}

// References lists the variable ids a value names, anywhere in it, once each and in order.
func References(v interface{}) []string {
	seen := map[string]bool{}
	collect(v, seen)
	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func collect(v interface{}, seen map[string]bool) {
	switch t := v.(type) {
	case string:
		for _, m := range variablePlaceholderPattern.FindAllStringSubmatch(t, -1) {
			seen[m[1]] = true
		}
	case map[string]interface{}:
		for _, e := range t {
			collect(e, seen)
		}
	case []interface{}:
		for _, e := range t {
			collect(e, seen)
		}
	}
}

// resolve returns a copy of a value with every {{var(id)}} replaced by the variable's value, which values holds
// JSON encoded by the variable's runtime key. A string that is nothing but a placeholder becomes the value itself,
// of its own type: a switch resolves to a boolean, a number to a number. A placeholder inside a longer string is
// replaced by the value's text. A placeholder no value exists for is left as it is, so a missing value shows up as
// itself and not as an empty one.
func resolve(v interface{}, values map[string]string) interface{} {
	switch t := v.(type) {
	case string:
		return resolveString(t, values)
	case map[string]interface{}:
		out := make(map[string]interface{}, len(t))
		for k, e := range t {
			out[k] = resolve(e, values)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(t))
		for i, e := range t {
			out[i] = resolve(e, values)
		}
		return out
	default:
		return v
	}
}

func resolveString(s string, values map[string]string) interface{} {
	if !strings.Contains(s, "{{") {
		return s
	}
	if loc := variablePlaceholderPattern.FindStringSubmatchIndex(s); len(loc) == 4 && loc[0] == 0 && loc[1] == len(s) {
		if encoded, ok := values[runtimeKey(s[loc[2]:loc[3]])]; ok {
			var decoded interface{}
			if json.Unmarshal([]byte(encoded), &decoded) == nil {
				return decoded
			}
		}
		return s
	}
	return variablePlaceholderPattern.ReplaceAllStringFunc(s, func(match string) string {
		id := variablePlaceholderPattern.FindStringSubmatch(match)[1]
		encoded, ok := values[runtimeKey(id)]
		if !ok {
			return match
		}
		var decoded interface{}
		if json.Unmarshal([]byte(encoded), &decoded) != nil {
			return match
		}
		if text, isString := decoded.(string); isString {
			return text
		}
		return encoded
	})
}

// RawFromJSONValue turns a value read from JSON into the raw string form values are stored and compared in, for the
// data type of the variable.
func RawFromJSONValue(def providers.VariableDefinition, v interface{}) (string, error) {
	if def.Type == providers.InputTypeSelect {
		return rawFromScalar(v)
	}
	return rawFromJSONValue(dataTypeOf(def), v)
}

// ValidateRaw checks a raw value against what a variable allows.
func ValidateRaw(def providers.VariableDefinition, raw string) error {
	return validateRaw(def, raw)
}

// EncodeRuntime encodes a raw value as the JSON a runtime data entry holds, of the data type of the variable.
func EncodeRuntime(def providers.VariableDefinition, raw string) (string, error) {
	return (&variablesNode{}).encodeRuntime(dataTypeOf(def), raw)
}
