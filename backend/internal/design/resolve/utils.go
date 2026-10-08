// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resolve

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
)

// designPlaceholderRegex matches a design-token placeholder, capturing the dot-path token key:
// "{{design(palette.primary.main)}}" -> "palette.primary.main". The key grammar allows word characters,
// dots, and hyphens to match hyphenated theme keys (e.g. "palette.primary.on-hover").
var designPlaceholderRegex = regexp.MustCompile(`\{\{design\(([\w.-]+)\)}}`)

// resolveApplicableTheme decodes theme JSON and promotes the applicable color scheme (the requested one,
// or defaultColorScheme when that is empty or absent) onto the theme root, so palette.* and root-level
// typography.*/shape.* share one dot-path namespace; the colorSchemes and defaultColorScheme inputs are
// dropped. It returns errEmptyTheme, errNoApplicableColorScheme (no scheme applies), or errMalformedTheme
// (invalid or trailing JSON) so a consumer never silently resolves against no tokens.
func resolveApplicableTheme(themeJSON json.RawMessage, colorScheme string) (map[string]interface{}, error) {
	if len(themeJSON) == 0 {
		return nil, errEmptyTheme
	}
	dec := json.NewDecoder(bytes.NewReader(themeJSON))
	dec.UseNumber()
	var root map[string]interface{}
	if err := dec.Decode(&root); err != nil {
		return nil, fmt.Errorf("%w: %w", errMalformedTheme, err)
	}
	// A well-formed theme is a single JSON object; reject trailing content.
	var trailing json.RawMessage
	if err := dec.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, errMalformedTheme
	}

	schemes, ok := root["colorSchemes"].(map[string]interface{})
	if !ok {
		return nil, errNoApplicableColorScheme
	}
	selected, ok := schemes[colorScheme].(map[string]interface{})
	if !ok {
		def, _ := root["defaultColorScheme"].(string)
		fallback, found := schemes[def].(map[string]interface{})
		if !found {
			return nil, errNoApplicableColorScheme
		}
		selected = fallback
	}

	// Promote the scheme onto the root and drop the scheme-selection inputs.
	delete(root, "colorSchemes")
	delete(root, "defaultColorScheme")
	for key, val := range selected {
		root[key] = val
	}
	return root, nil
}

// flattenTokens flattens a theme object into a dot-path -> value map (e.g. "palette.primary.main" ->
// "#fa7b3f"), recursing into nested objects. Numbers keep their authored form; non-scalar leaves
// (arrays, null) are skipped.
func flattenTokens(prefix string, m map[string]interface{}) map[string]string {
	out := map[string]string{}
	flattenTokensInto(out, prefix, m)
	return out
}

// flattenTokensInto writes m's scalar leaves into out under their dot-path, recursing into nested
// objects, accumulating into a single map to avoid per-level allocations.
func flattenTokensInto(out map[string]string, prefix string, m map[string]interface{}) {
	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		if nested, ok := v.(map[string]interface{}); ok {
			flattenTokensInto(out, key, nested)
			continue
		}
		switch val := v.(type) {
		case string:
			out[key] = val
		case json.Number:
			out[key] = val.String()
		case bool:
			out[key] = strconv.FormatBool(val)
		}
	}
}

// isSafeTokenValue reports whether a resolved token value is free of the crudest injection vector: angle
// brackets and control characters. Legitimate values (colors, CSS functions, numbers, keywords, font
// stacks) never contain these. This is shallow defense in depth only; see ResolveDesignContent for the
// full contract and its limits.
func isSafeTokenValue(val string) bool {
	for _, r := range val {
		if r == '<' || r == '>' || r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

// substituteDesignTokens replaces every {{design(<dot.path>)}} placeholder in content with its token
// value verbatim, leaving a placeholder untouched when its key is absent or its value is unsafe (see
// isSafeTokenValue). It returns the resolved content and the de-duplicated keys rejected as unsafe (for
// logging). Values are NOT encoded; the caller owns output encoding (see ResolveDesignContent).
func substituteDesignTokens(content string, tokens map[string]string) (string, []string) {
	var dropped []string
	seen := map[string]bool{}
	resolved := designPlaceholderRegex.ReplaceAllStringFunc(content, func(match string) string {
		key := designPlaceholderRegex.FindStringSubmatch(match)[1]
		val, ok := tokens[key]
		if !ok {
			return match
		}
		if !isSafeTokenValue(val) {
			if !seen[key] {
				seen[key] = true
				dropped = append(dropped, key)
			}
			return match
		}
		return val
	})
	return resolved, dropped
}
