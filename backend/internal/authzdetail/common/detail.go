// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package common provides the RFC 9396 authorization details logic shared by management and runtime.
package common

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const (
	// FieldType is the type member every authorization detail carries.
	FieldType = "type"
	// maxDetailsPerRequest bounds the number of details one request may carry.
	maxDetailsPerRequest = 50
)

// Detail is one authorization details object, with fields kept as raw JSON to preserve them exactly.
type Detail struct {
	ID     string
	Type   string
	Fields map[string]json.RawMessage
}

// ParseRequestedDetails parses details in the form clients and tokens carry them.
func ParseRequestedDetails(raw string) ([]Detail, error) {
	return parseDetailArray([]byte(raw))
}

// ParseIdentifiedDetails parses validated details and gives each a new identifier.
func ParseIdentifiedDetails(raw json.RawMessage) ([]Detail, error) {
	objects, err := ParseRequestedDetails(string(raw))
	if err != nil {
		return nil, err
	}
	for i := range objects {
		id, err := sysutils.GenerateUUIDv7()
		if err != nil {
			return nil, fmt.Errorf("failed to generate authorization detail id: %w", err)
		}
		objects[i].ID = id
	}
	return objects, nil
}

// parseDetailArray parses a non-empty JSON array of objects, each carrying a string type member.
func parseDetailArray(raw []byte) ([]Detail, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '[' {
		return nil, errors.New("authorization_details must be a JSON array")
	}
	var items []json.RawMessage
	if err := json.Unmarshal(trimmed, &items); err != nil {
		return nil, fmt.Errorf("authorization_details is not valid JSON: %w", err)
	}
	if err := checkJSONText(trimmed); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, errors.New("authorization_details must contain at least one object")
	}
	if len(items) > maxDetailsPerRequest {
		return nil, fmt.Errorf("authorization_details may carry at most %d entries", maxDetailsPerRequest)
	}

	objects := make([]Detail, 0, len(items))
	for _, item := range items {
		item = bytes.TrimSpace(item)
		if len(item) == 0 || item[0] != '{' {
			return nil, errors.New("each authorization details entry must be a JSON object")
		}
		fields := map[string]json.RawMessage{}
		if err := json.Unmarshal(item, &fields); err != nil {
			return nil, fmt.Errorf("authorization details entry is not valid JSON: %w", err)
		}
		typeRaw, ok := fields[FieldType]
		if !ok {
			return nil, errors.New("each authorization details entry must carry a type")
		}
		var typ string
		if err := json.Unmarshal(typeRaw, &typ); err != nil || typ == "" {
			return nil, errors.New("the type of an authorization details entry must be a non-empty string")
		}
		objects = append(objects, Detail{Type: typ, Fields: fields})
	}
	return objects, nil
}

// DetailFromValue reads a detail held by its identifier and type value from the object it carries.
func DetailFromValue(id, typ string, raw json.RawMessage) (Detail, error) {
	fields := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &fields); err != nil {
		return Detail{}, err
	}
	return Detail{ID: id, Type: typ, Fields: fields}, nil
}

// EncodeValues renders details in the form clients and tokens carry them, without identifiers.
func EncodeValues(objects []Detail) (json.RawMessage, error) {
	items := make([]json.RawMessage, 0, len(objects))
	for _, obj := range objects {
		item, err := obj.Value()
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return json.Marshal(items)
}

// EncodeDetailValues renders the values of held details in the form clients and tokens carry them,
// or nil when there are none.
func EncodeDetailValues(details []providers.AuthorizationDetail) (json.RawMessage, error) {
	if len(details) == 0 {
		return nil, nil
	}
	values := make([]json.RawMessage, 0, len(details))
	for _, detail := range details {
		values = append(values, detail.Value)
	}
	return json.Marshal(values)
}

// Value renders the detail as sent by the client, without its identifier.
func (d Detail) Value() (json.RawMessage, error) {
	return json.Marshal(d.Fields)
}

// instance decodes the detail for schema validation.
func (d Detail) instance() (any, error) {
	raw, err := d.Value()
	if err != nil {
		return nil, err
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	return v, nil
}

// PointerPath converts a JSON Pointer (RFC 6901) to a dot-separated field path.
func PointerPath(pointer string) (string, error) {
	if !strings.HasPrefix(pointer, "/") {
		return "", fmt.Errorf("field pointer %q must start with /", pointer)
	}
	segments := strings.Split(pointer[1:], "/")
	for i, segment := range segments {
		segment = strings.ReplaceAll(strings.ReplaceAll(segment, "~1", "/"), "~0", "~")
		if segment == "" || strings.Contains(segment, ".") {
			return "", fmt.Errorf("field pointer %q does not name a field", pointer)
		}
		segments[i] = segment
	}
	return strings.Join(segments, "."), nil
}

// setPath sets the value at a dot-separated path, creating intermediate objects as needed.
func setPath(fields map[string]json.RawMessage, path string, value json.RawMessage) error {
	head, rest, nested := strings.Cut(path, ".")
	if head == "" {
		return fmt.Errorf("invalid field path %q", path)
	}
	if !nested {
		fields[head] = value
		return nil
	}

	child := map[string]json.RawMessage{}
	if existing, ok := fields[head]; ok && !isJSONNull(existing) {
		trimmed := bytes.TrimSpace(existing)
		if len(trimmed) == 0 || trimmed[0] != '{' {
			return fmt.Errorf("field %q is not an object", head)
		}
		if err := json.Unmarshal(trimmed, &child); err != nil {
			return fmt.Errorf("field %q is not an object: %w", head, err)
		}
	}
	if err := setPath(child, rest, value); err != nil {
		return err
	}
	encoded, err := json.Marshal(child)
	if err != nil {
		return err
	}
	fields[head] = encoded
	return nil
}

// fieldEntry is one leaf field of a detail, addressed by its dot-separated path.
type fieldEntry struct {
	Path  string
	Value json.RawMessage
}

// flattenFields lists the leaf fields of a detail, in the given member order and then by name.
func flattenFields(fields map[string]json.RawMessage, prefix string, order map[string][]string) []fieldEntry {
	keys := make([]string, 0, len(fields))
	listed := map[string]bool{}
	for _, k := range order[prefix] {
		if _, ok := fields[k]; ok {
			keys = append(keys, k)
			listed[k] = true
		}
	}
	rest := make([]string, 0, len(fields)-len(keys))
	for k := range fields {
		if !listed[k] {
			rest = append(rest, k)
		}
	}
	sort.Strings(rest)
	keys = append(keys, rest...)

	entries := make([]fieldEntry, 0, len(keys))
	for _, k := range keys {
		path := k
		if prefix != "" {
			path = prefix + "." + k
		}
		value := bytes.TrimSpace(fields[k])
		if len(value) > 0 && value[0] == '{' {
			child := map[string]json.RawMessage{}
			if err := json.Unmarshal(value, &child); err == nil && len(child) > 0 {
				entries = append(entries, flattenFields(child, path, order)...)
				continue
			}
		}
		entries = append(entries, fieldEntry{Path: path, Value: value})
	}
	return entries
}

// lookupPath returns the value at a dot-separated path, if present.
func lookupPath(fields map[string]json.RawMessage, path string) (json.RawMessage, bool) {
	head, rest, nested := strings.Cut(path, ".")
	value, ok := fields[head]
	if !ok {
		return nil, false
	}
	if !nested {
		return value, true
	}
	child := map[string]json.RawMessage{}
	if err := json.Unmarshal(value, &child); err != nil {
		return nil, false
	}
	return lookupPath(child, rest)
}

// isJSONNull reports whether a raw JSON value is null.
func isJSONNull(raw json.RawMessage) bool {
	return string(bytes.TrimSpace(raw)) == "null"
}

// checkJSONText refuses repeated member names and control or bidirectional formatting characters,
// which could make the value shown on consent differ from the value granted.
func checkJSONText(raw []byte) error {
	type frame struct {
		object    bool
		expectKey bool
		seen      map[string]bool
	}
	var stack []*frame
	decoder := json.NewDecoder(bytes.NewReader(raw))
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("value is not valid JSON: %w", err)
		}
		var top *frame
		if len(stack) > 0 {
			top = stack[len(stack)-1]
		}
		if delim, ok := token.(json.Delim); ok {
			switch delim {
			case '{':
				stack = append(stack, &frame{object: true, expectKey: true, seen: map[string]bool{}})
			case '[':
				stack = append(stack, &frame{})
			default:
				stack = stack[:len(stack)-1]
				if len(stack) > 0 && stack[len(stack)-1].object {
					stack[len(stack)-1].expectKey = true
				}
			}
			continue
		}
		text, isString := token.(string)
		if isString {
			if err := checkText(text); err != nil {
				return err
			}
		}
		if top == nil || !top.object {
			continue
		}
		if top.expectKey {
			if top.seen[text] {
				return fmt.Errorf("value repeats the member %q in one object", text)
			}
			top.seen[text] = true
			top.expectKey = false
			continue
		}
		top.expectKey = true
	}
}

// IsSafeText reports whether text carries no control characters other than whitespace and no bidirectional
// formatting characters, so it can be shown to a user as it is.
func IsSafeText(text string) bool {
	return checkText(text) == nil
}

// MaxReasonLength caps a processor's refusal reason shown to the user or the client.
const MaxReasonLength = 200

// DisplayableReason returns a processor's refusal reason as it may be shown, cut to MaxReasonLength, or empty
// when it is not safe to show.
func DisplayableReason(reason string) string {
	if !IsSafeText(reason) {
		return ""
	}
	if runes := []rune(reason); len(runes) > MaxReasonLength {
		return string(runes[:MaxReasonLength])
	}
	return reason
}

// checkText refuses control characters other than whitespace, and bidirectional formatting characters.
func checkText(text string) error {
	for _, r := range text {
		switch {
		case r == '\t' || r == '\n' || r == '\r':
		case r < 0x20 || (r >= 0x7f && r <= 0x9f):
			return errors.New("value carries a control character")
		case r == 0x061c || r == 0x200e || r == 0x200f || (r >= 0x202a && r <= 0x202e) ||
			(r >= 0x2066 && r <= 0x2069):
			return errors.New("value carries a bidirectional formatting character")
		}
	}
	return nil
}
