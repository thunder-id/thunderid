// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package serverconfig

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
)

// effectiveValue lays a section's configured value, composed from its readOnly and writable layers, over
// the section's default.
func effectiveValue(handler ServerConfigHandlerInterface, readOnly, writable any) (any, error) {
	configured := handler.Merge(readOnly, writable)
	def := handler.Default()
	if def == nil {
		return configured, nil
	}
	defaultFields, err := jsonObject(def)
	if err != nil {
		return nil, fmt.Errorf("failed to encode the default value: %w", err)
	}
	configuredFields, err := jsonObject(configured)
	if err != nil {
		return nil, fmt.Errorf("failed to encode the configured value: %w", err)
	}
	raw, err := json.Marshal(overlayFields(defaultFields, configuredFields))
	if err != nil {
		return nil, fmt.Errorf("failed to encode the effective value: %w", err)
	}
	return handler.Decode(raw)
}

// jsonObject encodes a section value as the JSON object it marshals to, keeping numbers exact.
func jsonObject(value any) (map[string]any, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var fields map[string]any
	err = decoder.Decode(&fields)
	return fields, err
}

// overlayFields lays top over base field by field: a field top sets wins, an object set in both is laid
// over the other the same way, and any other value, a list included, replaces base's whole. A field top
// sets to null stays unset.
func overlayFields(base, top map[string]any) map[string]any {
	merged := make(map[string]any, len(base)+len(top))
	maps.Copy(merged, base)
	for key, value := range top {
		if value == nil {
			continue
		}
		topObject, topIsObject := value.(map[string]any)
		baseObject, baseIsObject := merged[key].(map[string]any)
		if topIsObject && baseIsObject {
			merged[key] = overlayFields(baseObject, topObject)
			continue
		}
		merged[key] = value
	}
	return merged
}
