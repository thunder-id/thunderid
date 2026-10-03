// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package thememgt

import (
	"bytes"
	"context"
	"encoding/json"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// ResolveDesignTokens loads the theme and flattens the selected color scheme into a dot-path token
// map that consumers resolve their own placeholders against.
func (ts *themeMgtService) ResolveDesignTokens(ctx context.Context, themeID, colorScheme string) (
	map[string]string, *tidcommon.ServiceError) {
	theme, svcErr := ts.GetTheme(ctx, themeID)
	if svcErr != nil {
		return nil, svcErr
	}
	return flattenDesignTokens(theme.Theme, colorScheme), nil
}

// flattenDesignTokens selects a color scheme from theme JSON and flattens the whole scheme (every key
// under colorSchemes.<scheme>, not just palette) into a dot-path -> value map (e.g.
// "palette.primary.main" -> "#fa7b3f"). An empty colorScheme falls back to the theme's
// defaultColorScheme. Returns an empty map on any parse or navigation failure; numbers keep their
// authored form via json.Number.
func flattenDesignTokens(themeJSON json.RawMessage, colorScheme string) map[string]string {
	tokens := map[string]string{}
	if len(themeJSON) == 0 {
		return tokens
	}
	dec := json.NewDecoder(bytes.NewReader(themeJSON))
	dec.UseNumber()
	var root map[string]interface{}
	if err := dec.Decode(&root); err != nil {
		return tokens
	}
	scheme := colorScheme
	if scheme == "" {
		if def, ok := root["defaultColorScheme"].(string); ok {
			scheme = def
		}
	}
	schemes, ok := root["colorSchemes"].(map[string]interface{})
	if !ok {
		return tokens
	}
	selected, ok := schemes[scheme].(map[string]interface{})
	if !ok {
		return tokens
	}
	flattenTokensInto("", selected, tokens)
	return tokens
}

// flattenTokensInto walks a nested JSON object, writing scalar leaves into out keyed by their
// dot-path. Non-scalar leaves (arrays, null) are skipped.
func flattenTokensInto(prefix string, m map[string]interface{}, out map[string]string) {
	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		if nested, ok := v.(map[string]interface{}); ok {
			flattenTokensInto(key, nested, out)
			continue
		}
		switch val := v.(type) {
		case string:
			out[key] = val
		case json.Number:
			out[key] = val.String()
		case bool:
			if val {
				out[key] = "true"
			} else {
				out[key] = "false"
			}
		}
	}
}
