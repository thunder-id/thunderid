// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/thunder-id/thunderid/internal/system/valuecompare"
)

// policyKeyword is the schema keyword that carries a property's field policy.
const policyKeyword = "policy"

// Parties that supply or modify a field of a detail.
const (
	PartyClient = "client"
	// PartyProcessor is the resource owner's system, reached through the type's processor endpoint.
	PartyProcessor = "processor"
	PartyUser      = "user"
)

// Modifications a field policy may allow after the request.
const (
	modificationNone        = "none"
	modificationDecrease    = "decrease"
	modificationIncrease    = "increase"
	modificationRemoveItems = "removeItems"
	// modificationAddItems suits lists whose items take permission away, such as excluded merchants.
	modificationAddItems = "addItems"
)

// Value types a field policy orders values by, from the shared value types.
const (
	valueTypeNumber   = string(valuecompare.TypeNumber)
	valueTypeDate     = string(valuecompare.TypeDate)
	valueTypeDateTime = string(valuecompare.TypeDateTime)
)

// inferValueType returns the value type a property is ordered by, or empty if it has no order.
func inferValueType(property *jsonschema.Schema) string {
	switch property.Type {
	case "number":
		return valueTypeNumber
	case "string":
		switch property.Format {
		case "date":
			return valueTypeDate
		case "date-time":
			return valueTypeDateTime
		}
	}
	return ""
}

// fieldPolicy says who supplies a field and who may change it and how. ValueType is inferred.
// policySettings are the settings a policy may declare.
var policySettings = []string{"suppliedBy", "modification", "modifiableBy", "minimum", "maximum"}

// fieldPolicy is the policy a schema property declares.
type fieldPolicy struct {
	SuppliedBy   string          `json:"suppliedBy,omitempty"`
	Modification string          `json:"modification,omitempty"`
	ModifiableBy []string        `json:"modifiableBy,omitempty"`
	ValueType    string          `json:"-"`
	Minimum      json.RawMessage `json:"minimum,omitempty"`
	Maximum      json.RawMessage `json:"maximum,omitempty"`
}

// modifiable reports whether the policy lets the field change after the request.
func (p fieldPolicy) modifiable() bool {
	return p.Modification != "" && p.Modification != modificationNone
}

// FieldPolicies holds the policies of a type, keyed by the dot-separated path of the property.
type FieldPolicies map[string]fieldPolicy

// suppliedByProcessor reports whether the field at path, or an enclosing object, is processor-supplied.
func (p FieldPolicies) suppliedByProcessor(path string) bool {
	for {
		if p[path].SuppliedBy == PartyProcessor {
			return true
		}
		i := strings.LastIndex(path, ".")
		if i < 0 {
			return false
		}
		path = path[:i]
	}
}

// UsesProcessor reports whether any field is supplied or may be changed by the processor.
func (p FieldPolicies) UsesProcessor() bool {
	for _, policy := range p {
		if policy.SuppliedBy == PartyProcessor || slices.Contains(policy.ModifiableBy, PartyProcessor) {
			return true
		}
	}
	return false
}

// Policies returns the field policies of a registered schema, or none if it cannot be read.
func Policies(schema json.RawMessage) FieldPolicies {
	root, err := parseSchema(schema)
	if err != nil {
		return FieldPolicies{}
	}
	policies, err := policiesOf(root)
	if err != nil {
		return FieldPolicies{}
	}
	return policies
}

// policiesOf reads and validates the field policies a parsed schema declares.
func policiesOf(root *jsonschema.Schema) (FieldPolicies, error) {
	policies := FieldPolicies{}
	err := walkProperties(root, "", func(path, name string, property *jsonschema.Schema) error {
		if name == "" || strings.Contains(name, ".") {
			return fmt.Errorf("property name %q must be non-empty and contain no dot", name)
		}
		declared, ok := property.Extra[policyKeyword]
		if !ok {
			return nil
		}
		policy, err := decodePolicy(declared)
		if err != nil {
			return fieldError(path, "policy of %s: %w", path, err)
		}
		policy.ValueType = inferValueType(property)
		if err := validatePolicy(path, property, policy); err != nil {
			return fieldError(path, "policy of %s: %w", path, err)
		}
		// Parents are visited first, so a processor-supplied ancestor is already known.
		if parent, nested := parentPath(path); nested && policy.modifiable() && policies.suppliedByProcessor(parent) {
			return fieldError(path, "policy of %s: a field inside one the processor supplies cannot be modifiable",
				path)
		}
		policies[path] = policy
		return nil
	})
	if err != nil {
		return nil, err
	}
	return policies, nil
}

// decodePolicy decodes a policy keyword value, refusing settings it does not know.
func decodePolicy(declared any) (fieldPolicy, error) {
	// Settings are matched by their exact names, since every reader of a policy compares them exactly.
	if settings, ok := declared.(map[string]any); ok {
		for name := range settings {
			if !slices.Contains(policySettings, name) {
				return fieldPolicy{}, fmt.Errorf("unknown policy setting %q", name)
			}
		}
	}
	encoded, err := json.Marshal(declared)
	if err != nil {
		return fieldPolicy{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var policy fieldPolicy
	if err := decoder.Decode(&policy); err != nil {
		return fieldPolicy{}, err
	}
	return policy, nil
}

// validatePolicy checks one field policy against the property that declares it.
func validatePolicy(path string, property *jsonschema.Schema, p fieldPolicy) error {
	switch p.SuppliedBy {
	case "", PartyClient:
	case PartyProcessor:
		// The type decides which schema applies, so nothing but the client may set it.
		if head, _, _ := strings.Cut(path, "."); head == FieldType {
			return errors.New("the type cannot be supplied by the processor")
		}
		if p.modifiable() {
			return errors.New("a field supplied by the processor cannot also be modifiable")
		}
	default:
		return fmt.Errorf("unsupported suppliedBy %q", p.SuppliedBy)
	}

	if p.ModifiableBy != nil && len(p.ModifiableBy) == 0 {
		return errors.New("modifiableBy must name at least one party; leave it out otherwise")
	}
	switch p.Modification {
	case "", modificationNone:
		if len(p.ModifiableBy) > 0 {
			return errors.New("modifiableBy needs a modification")
		}
	case modificationDecrease, modificationIncrease:
		if p.ValueType == "" {
			return errors.New("decrease and increase need a number, or a string in the date or date-time format")
		}
	case modificationRemoveItems, modificationAddItems:
		if property.Type != "array" {
			return fmt.Errorf("%s applies to an array", p.Modification)
		}
	default:
		return fmt.Errorf("unsupported modification %q", p.Modification)
	}
	if p.modifiable() {
		if len(p.ModifiableBy) == 0 {
			return errors.New("a modification needs modifiableBy")
		}
		seen := map[string]bool{}
		for _, party := range p.ModifiableBy {
			if !slices.Contains([]string{PartyProcessor, PartyUser, PartyClient}, party) || seen[party] {
				return fmt.Errorf("unsupported modifiableBy %q", party)
			}
			// The consent screen offers no way to change a date and time.
			if party == PartyUser && p.ValueType == valueTypeDateTime {
				return errors.New("the user cannot change a date and time, so modifiableBy cannot name the user")
			}
			seen[party] = true
		}
	}

	return validatePolicyLimits(property, p)
}

// validatePolicyLimits checks a policy's date limits; numbers use the schema's own minimum and maximum.
func validatePolicyLimits(property *jsonschema.Schema, p fieldPolicy) error {
	hasLimit := len(p.Minimum) > 0 || len(p.Maximum) > 0
	if hasLimit && property.Type == "number" {
		return errors.New("limit a number with the schema's own minimum and maximum keywords")
	}
	if p.ValueType == "" {
		if hasLimit {
			return errors.New("minimum and maximum need a string in the date or date-time format")
		}
		return nil
	}
	for _, limit := range []json.RawMessage{p.Minimum, p.Maximum} {
		if len(limit) == 0 {
			continue
		}
		if _, err := compareOrdered(limit, limit, p.ValueType); err != nil {
			return errLimitFormat(p.ValueType)
		}
	}
	if len(p.Minimum) > 0 && len(p.Maximum) > 0 {
		if order, _ := compareOrdered(p.Minimum, p.Maximum, p.ValueType); order > 0 {
			return errLimitOrder
		}
	}
	return nil
}

// errLimitFormat is returned when a limit is not written as its value type is read.
func errLimitFormat(valueType string) error {
	if valueType == valueTypeDate {
		return errors.New("a limit must be a date written as YYYY-MM-DD")
	}
	return errors.New("a limit must be an RFC 3339 date and time with an offset, such as 2026-01-31T09:00:00Z")
}

// CheckRequestedFields checks that a requested detail has no processor-supplied field and is within limits.
func CheckRequestedFields(policies FieldPolicies, obj Detail) error {
	for path := range leafFields(obj.Fields) {
		if policies.suppliedByProcessor(path) {
			return fmt.Errorf("field %s is supplied by the processor", path)
		}
	}
	return CheckFieldLimits(policies, obj)
}

// CheckFieldLimits checks that every field with a minimum or maximum is within it.
func CheckFieldLimits(policies FieldPolicies, obj Detail) error {
	for path, policy := range policies {
		if len(policy.Minimum) == 0 && len(policy.Maximum) == 0 {
			continue
		}
		value, ok := lookupPath(obj.Fields, path)
		if !ok {
			continue
		}
		if len(policy.Minimum) > 0 {
			if order, err := compareOrdered(value, policy.Minimum, policy.ValueType); err != nil || order < 0 {
				return fmt.Errorf("field %s is below its minimum", path)
			}
		}
		if len(policy.Maximum) > 0 {
			if order, err := compareOrdered(value, policy.Maximum, policy.ValueType); err != nil || order > 0 {
				return fmt.Errorf("field %s is above its maximum", path)
			}
		}
	}
	return nil
}

// parentPath returns the path of the field that holds the one at path, reporting whether there is one.
func parentPath(path string) (string, bool) {
	i := strings.LastIndex(path, ".")
	if i < 0 {
		return "", false
	}
	return path[:i], true
}
