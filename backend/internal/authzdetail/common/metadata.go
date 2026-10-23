// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"bytes"
	"encoding/json"
	"slices"
	"sort"
)

// ClientModification is a change a client may make to a field of a granted detail at the token endpoint.
type ClientModification struct {
	Field        string `json:"field"`
	Modification string `json:"modification"`
}

// RequestSchema returns the client view of a schema, without processor-supplied fields or policies.
func RequestSchema(raw json.RawMessage) (json.RawMessage, error) {
	root, err := parseSchema(raw)
	if err != nil {
		return nil, err
	}
	policies, err := policiesOf(root)
	if err != nil {
		return nil, err
	}
	return requestSchema(raw, policies, "")
}

// requestSchema returns the schema at prefix without the fields the processor supplies.
func requestSchema(schema json.RawMessage, policies FieldPolicies, prefix string) (json.RawMessage, error) {
	names, members, ok := objectMembers(schema)
	if !ok {
		return schema, nil
	}
	supplied := map[string]bool{}
	if propertyNames, properties, ok := objectMembers(members["properties"]); ok {
		kept := make([]string, 0, len(propertyNames))
		for _, name := range propertyNames {
			path := name
			if prefix != "" {
				path = prefix + "." + name
			}
			if policies[path].SuppliedBy == PartyProcessor {
				supplied[name] = true
				continue
			}
			view, err := requestSchema(properties[name], policies, path)
			if err != nil {
				return nil, err
			}
			properties[name] = view
			kept = append(kept, name)
		}
		encoded, err := encodeObject(kept, properties)
		if err != nil {
			return nil, err
		}
		members["properties"] = encoded
	}
	if len(supplied) > 0 {
		var required []string
		if err := json.Unmarshal(members["required"], &required); err == nil {
			encoded, err := json.Marshal(slices.DeleteFunc(required, func(name string) bool { return supplied[name] }))
			if err != nil {
				return nil, err
			}
			members["required"] = encoded
		}
	}
	return encodeObject(slices.DeleteFunc(names, func(name string) bool { return name == policyKeyword }),
		members)
}

// encodeObject encodes the named members as a JSON object in the given order.
func encodeObject(names []string, members map[string]json.RawMessage) (json.RawMessage, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, name := range names {
		if i > 0 {
			buf.WriteByte(',')
		}
		key, err := json.Marshal(name)
		if err != nil {
			return nil, err
		}
		buf.Write(key)
		buf.WriteByte(':')
		buf.Write(members[name])
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// ClientModifications returns the changes a client may make to a granted detail, ordered by field path.
func ClientModifications(raw json.RawMessage) []ClientModification {
	modifications := []ClientModification{}
	for path, policy := range Policies(raw) {
		if policy.modifiable() && slices.Contains(policy.ModifiableBy, PartyClient) {
			modifications = append(modifications, ClientModification{Field: path, Modification: policy.Modification})
		}
	}
	sort.Slice(modifications, func(i, j int) bool { return modifications[i].Field < modifications[j].Field })
	return modifications
}
