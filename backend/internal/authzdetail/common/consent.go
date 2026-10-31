// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Approval is an authorization detail the user approved earlier, as held in their consent record.
type Approval struct {
	// TypeID tells apart a type deleted and created again under the same value.
	TypeID      string
	TypeVersion int
	Granted     json.RawMessage
}

// IsCovered reports whether an earlier approval on the same type version covers the detail.
func IsCovered(obj Detail, t providers.AuthorizationDetailType, approvals []Approval) bool {
	if !t.Reuse {
		return false
	}
	policies := Policies(t.Schema)
	for _, approval := range approvals {
		if approval.TypeID != t.ID || approval.TypeVersion != t.Version {
			continue
		}
		granted, err := DetailFromValue("", t.Type, approval.Granted)
		if err != nil {
			continue
		}
		if CheckModification(granted, obj, policies, "") == nil {
			return true
		}
	}
	return false
}

// DetailElement builds the consent prompt element of a detail, named after the detail identifier.
func DetailElement(obj Detail, t providers.AuthorizationDetailType) providers.PromptElement {
	rows := []providers.ConsentDetailRow{}
	// A registered schema has been checked, so an unreadable one just shows every field as fixed.
	root, err := parseSchema(t.Schema)
	if err != nil {
		root = &jsonschema.Schema{}
	}
	titles := titlesOf(root)
	itemOptions := itemOptionsOf(root)
	policies, _ := policiesOf(root)
	for _, entry := range flattenFields(obj.Fields, "", propertyOrderOf(t.Schema)) {
		if entry.Path == FieldType {
			continue
		}
		row := providers.ConsentDetailRow{Path: entry.Path, Label: entry.Path, Value: entry.Value}
		if title, ok := titles[entry.Path]; ok {
			row.Label = title
		}
		if policy := policies[entry.Path]; policy.modifiable() &&
			slices.Contains(policy.ModifiableBy, PartyUser) {
			row.Modification = policy.Modification
			row.As = policy.ValueType
			if policy.Modification == modificationAddItems {
				if values, ok := itemOptions[entry.Path]; ok {
					if encoded, err := json.Marshal(values); err == nil {
						row.Options = encoded
					}
				}
			}
		}
		if policies.suppliedByProcessor(entry.Path) {
			row.Origin = providers.ConsentDetailRowOriginServer
		}
		rows = append(rows, row)
	}

	return providers.PromptElement{
		Name:       obj.ID,
		DetailType: t.Type,
		Summary:    renderConsentTemplate(t.ConsentTemplate, obj.Fields),
		Rows:       rows,
	}
}

// renderConsentTemplate fills the template's placeholders, returning empty if any placeholder has no value.
func renderConsentTemplate(template string, fields map[string]json.RawMessage) string {
	if template == "" {
		return ""
	}
	complete := true
	summary := TemplatePlaceholder.ReplaceAllStringFunc(template, func(match string) string {
		path := TemplatePlaceholder.FindStringSubmatch(match)[1]
		value, ok := lookupPath(fields, path)
		text := ""
		if ok {
			text = formatValue(value)
		}
		if text == "" {
			complete = false
		}
		return text
	})
	if !complete {
		return ""
	}
	return summary
}

// formatValue renders a field value as display text.
func formatValue(raw json.RawMessage) string {
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return ""
	}
	switch v := decoded.(type) {
	case nil:
		return ""
	case string:
		return v
	case []any:
		// Items are rendered from their own JSON text, so numbers are shown exactly as granted.
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			return ""
		}
		parts := make([]string, 0, len(v))
		for _, item := range items {
			var text string
			if err := json.Unmarshal(item, &text); err == nil {
				parts = append(parts, text)
				continue
			}
			var compact bytes.Buffer
			if err := json.Compact(&compact, item); err != nil {
				return ""
			}
			parts = append(parts, compact.String())
		}
		return strings.Join(parts, ", ")
	case map[string]any:
		return string(raw)
	default:
		return strings.TrimSpace(string(raw))
	}
}
