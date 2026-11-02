// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"

	"github.com/thunder-id/thunderid/internal/system/valuecompare"
)

// CheckModification checks that candidate grants no more than base under the policies for party.
// An empty party allows every modification regardless of who may make it.
func CheckModification(base, candidate Detail, policies FieldPolicies, party string) error {
	baseFields := leafFields(base.Fields)
	candidateFields := leafFields(candidate.Fields)

	paths := make([]string, 0, len(baseFields)+len(candidateFields))
	for path := range baseFields {
		paths = append(paths, path)
	}
	for path := range candidateFields {
		if _, ok := baseFields[path]; !ok {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)

	for _, path := range paths {
		if policies.suppliedByProcessor(path) && party == PartyProcessor {
			continue
		}
		policy := policies[path]
		modification := policy.Modification
		if party != "" && !slices.Contains(policy.ModifiableBy, party) {
			modification = modificationNone
		}
		baseValue, inBase := baseFields[path]
		candidateValue, inCandidate := candidateFields[path]
		if err := checkFieldModification(path, modification, policy.ValueType,
			baseValue, inBase, candidateValue, inCandidate); err != nil {
			return err
		}
	}
	return nil
}

// checkFieldModification checks one field change against the modification the field allows.
func checkFieldModification(path, modification, valueType string, base json.RawMessage, inBase bool,
	candidate json.RawMessage, inCandidate bool) error {
	switch modification {
	case modificationDecrease, modificationIncrease:
		if !inBase || !inCandidate {
			if inBase == inCandidate {
				return nil
			}
			return fmt.Errorf("field %s cannot be added or removed", path)
		}
		order, err := compareOrdered(candidate, base, valueType)
		if err != nil {
			return fmt.Errorf("field %s: %w", path, err)
		}
		if (modification == modificationDecrease && order > 0) ||
			(modification == modificationIncrease && order < 0) {
			return fmt.Errorf("field %s exceeds the granted value", path)
		}
		return nil
	case modificationRemoveItems, modificationAddItems:
		// A list the baseline has is never left out, since a missing list can read as unrestricted; an empty
		// list is the narrowest value. An absent baseline counts as empty.
		if inBase && !inCandidate {
			return fmt.Errorf("field %s cannot be removed; send an empty list instead", path)
		}
		baseItems, err := arrayItems(base, inBase)
		if err != nil {
			return fmt.Errorf("field %s: %w", path, err)
		}
		candidateItems, err := arrayItems(candidate, inCandidate)
		if err != nil {
			return fmt.Errorf("field %s: %w", path, err)
		}
		// Removing items may only drop items of the earlier list; adding items must keep all of them.
		required, allowed := candidateItems, baseItems
		if modification == modificationAddItems {
			required, allowed = baseItems, candidateItems
		}
		// Each allowed item answers for one required item, so a removal cannot repeat a granted item.
		used := make([]bool, len(allowed))
		for _, item := range required {
			match := -1
			for j, o := range allowed {
				if !used[j] && valuesEqual(o, item) {
					match = j
					break
				}
			}
			if match < 0 {
				if modification == modificationAddItems {
					return fmt.Errorf("field %s must keep every granted item", path)
				}
				return fmt.Errorf("field %s exceeds the granted value", path)
			}
			used[match] = true
		}
		return nil
	default:
		if inBase != inCandidate || (inBase && !jsonEqual(base, candidate)) {
			return fmt.Errorf("field %s cannot change", path)
		}
		return nil
	}
}

// leafFields indexes the leaf fields of a detail by their dot-separated path. An empty object holds no
// field, so a processor can fill one in.
func leafFields(fields map[string]json.RawMessage) map[string]json.RawMessage {
	entries := flattenFields(fields, "", nil)
	leaves := make(map[string]json.RawMessage, len(entries))
	for _, entry := range entries {
		if isEmptyObject(entry.Value) {
			continue
		}
		leaves[entry.Path] = entry.Value
	}
	return leaves
}

// isEmptyObject reports whether a raw JSON value is an object with no members.
func isEmptyObject(raw json.RawMessage) bool {
	var members map[string]json.RawMessage
	return len(raw) > 0 && raw[0] == '{' && json.Unmarshal(raw, &members) == nil && len(members) == 0
}

// decodeJSON decodes a value keeping numbers as their literal text, so that no precision is lost.
func decodeJSON(raw json.RawMessage) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var v any
	if err := decoder.Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}

// jsonEqual reports whether two JSON values are equal, comparing numbers by value.
func jsonEqual(a, b json.RawMessage) bool {
	av, errA := decodeJSON(a)
	bv, errB := decodeJSON(b)
	return errA == nil && errB == nil && valuesEqual(av, bv)
}

// SchemaEquivalent reports whether two registered schemas define the same type, ignoring order and defaults.
func SchemaEquivalent(a, b json.RawMessage) bool {
	av, errA := decodeJSON(a)
	bv, errB := decodeJSON(b)
	if errA != nil || errB != nil {
		return false
	}
	return valuesEqual(canonicalSchema(av, true), canonicalSchema(bv, true))
}

// canonicalSchema returns a decoded schema in the form SchemaEquivalent compares.
func canonicalSchema(value any, root bool) any {
	schema, ok := value.(map[string]any)
	if !ok {
		return value
	}
	out := make(map[string]any, len(schema))
	for key, member := range schema {
		switch key {
		case "properties":
			properties, isObject := member.(map[string]any)
			if !isObject {
				out[key] = member
				continue
			}
			if len(properties) == 0 {
				continue
			}
			canonical := make(map[string]any, len(properties))
			for name, property := range properties {
				canonical[name] = canonicalSchema(property, false)
			}
			out[key] = canonical
		case "items":
			out[key] = canonicalSchema(member, false)
		case "required":
			names := sortedNames(member)
			if root {
				names = slices.DeleteFunc(names, func(name any) bool { return name == FieldType })
			}
			if len(names) > 0 {
				out[key] = names
			}
		case policyKeyword:
			if policy := canonicalPolicy(member); len(policy) > 0 {
				out[key] = policy
			}
		default:
			out[key] = member
		}
	}
	return out
}

// canonicalPolicy returns a decoded policy without default settings and with its parties sorted.
func canonicalPolicy(value any) map[string]any {
	policy, ok := value.(map[string]any)
	if !ok {
		return map[string]any{"": value}
	}
	out := make(map[string]any, len(policy))
	for key, setting := range policy {
		switch {
		case key == "suppliedBy" && setting == PartyClient, key == "modification" && setting == modificationNone:
		case key == "modifiableBy":
			out[key] = sortedNames(setting)
		default:
			out[key] = setting
		}
	}
	return out
}

// sortedNames returns a decoded list of strings sorted and without repeats; any other value is kept.
func sortedNames(value any) []any {
	items, ok := value.([]any)
	if !ok {
		return []any{value}
	}
	names := make([]string, 0, len(items))
	for _, item := range items {
		name, isString := item.(string)
		if !isString {
			return items
		}
		names = append(names, name)
	}
	slices.Sort(names)
	names = slices.Compact(names)
	out := make([]any, len(names))
	for i, name := range names {
		out[i] = name
	}
	return out
}

// valuesEqual compares two decoded JSON values, treating numbers by value.
func valuesEqual(a, b any) bool {
	switch av := a.(type) {
	case json.Number:
		bv, ok := b.(json.Number)
		if !ok {
			return false
		}
		if equal, err := valuecompare.Equal(av.String(), bv.String(), valuecompare.TypeNumber); err == nil {
			return equal
		}
		return av == bv
	case []any:
		bv, ok := b.([]any)
		if !ok || len(av) != len(bv) {
			return false
		}
		for i := range av {
			if !valuesEqual(av[i], bv[i]) {
				return false
			}
		}
		return true
	case map[string]any:
		bv, ok := b.(map[string]any)
		if !ok || len(av) != len(bv) {
			return false
		}
		for k, v := range av {
			if w, ok := bv[k]; !ok || !valuesEqual(v, w) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(a, b)
	}
}

// arrayItems decodes an array field. An absent field, which only a baseline can be, is an empty array.
func arrayItems(raw json.RawMessage, present bool) ([]any, error) {
	if !present {
		return nil, nil
	}
	v, err := decodeJSON(raw)
	if err != nil {
		return nil, err
	}
	items, ok := v.([]any)
	if !ok {
		return nil, errors.New("value is not an array")
	}
	return items, nil
}

// compareOrdered compares two scalar values of an ordered value type, returning -1, 0 or 1.
func compareOrdered(a, b json.RawMessage, as string) (int, error) {
	t := valuecompare.Type(as)
	if !t.IsOrdered() {
		return 0, fmt.Errorf("unsupported ordering %q", as)
	}
	aText, err := orderedText(a, t)
	if err != nil {
		return 0, err
	}
	bText, err := orderedText(b, t)
	if err != nil {
		return 0, err
	}
	return valuecompare.Compare(aText, bText, t)
}

// orderedText returns the text of an ordered value, refusing a value of the wrong JSON type.
func orderedText(raw json.RawMessage, t valuecompare.Type) (string, error) {
	v, err := decodeJSON(raw)
	if err != nil {
		return "", err
	}
	switch value := v.(type) {
	case json.Number:
		if t == valuecompare.TypeNumber {
			return value.String(), nil
		}
	case string:
		if t != valuecompare.TypeNumber {
			return value, nil
		}
	}
	return "", fmt.Errorf("value is not a %s", t)
}
