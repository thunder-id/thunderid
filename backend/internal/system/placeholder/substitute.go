// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package placeholder is the single implementation of ThunderID's {{fn(arg)}} placeholder
// substitution. Templated content (notification subjects and bodies, and any other rendered string)
// may embed three placeholder functions resolved at render time:
//
//	{{ctx(var)}}      flow/request context values
//	{{t(key)}}        localized text (translation keys)
//	{{design(token)}} design tokens, addressed by dot-path (e.g. palette.primary.main)
//
// The package owns the grammar and the replace mechanism only; it holds no reference to the i18n,
// design, or flow services. Each family is resolved through a caller-supplied Resolve func, so the
// data source (a context map, a resolved translation bundle, a flattened theme) stays with the
// feature that owns it and this package stays dependency-light and fully testable.
package placeholder

import (
	"html"
	"regexp"
)

// Resolve returns the replacement value for a placeholder's captured argument. Returning ok=false
// leaves the placeholder literal, so rendering degrades gracefully when a key is unknown.
type Resolve func(arg string) (string, bool)

// Placeholder grammars. All three tolerate optional whitespace around the argument, matching the
// {{ctx(...)}} form already resolved by flow execution. ctx and t take an identifier (letters,
// digits, underscore); design additionally allows the dots of a token path.
var (
	// CtxPattern matches {{ctx(var)}}.
	CtxPattern = regexp.MustCompile(`\{\{\s*ctx\(\s*(\w+)\s*\)\s*\}\}`)
	// TPattern matches {{t(key)}}.
	TPattern = regexp.MustCompile(`\{\{\s*t\(\s*([\w.]+)\s*\)\s*\}\}`)
	// DesignPattern matches {{design(token.path)}}.
	DesignPattern = regexp.MustCompile(`\{\{\s*design\(\s*([\w.]+)\s*\)\s*\}\}`)
)

// Substitute replaces every placeholder matched by pattern in content with the value resolve returns
// for the placeholder's captured argument, HTML-escaping that value when escapeHTML is set. pattern
// must have a single capturing group (the argument). This is the shared mechanism behind every
// {{fn(arg)}} substitution; a new placeholder function supplies its own pattern and Resolve rather
// than re-implementing the replace loop.
func Substitute(content string, pattern *regexp.Regexp, resolve Resolve, escapeHTML bool) string {
	if resolve == nil {
		return content
	}
	return pattern.ReplaceAllStringFunc(content, func(match string) string {
		submatches := pattern.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}
		if val, ok := resolve(submatches[1]); ok {
			if escapeHTML {
				return html.EscapeString(val)
			}
			return val
		}
		return match
	})
}

// MapResolve adapts a lookup map to a Resolve, the common case for context and already-resolved
// translation or token maps.
func MapResolve(m map[string]string) Resolve {
	return func(arg string) (string, bool) {
		val, ok := m[arg]
		return val, ok
	}
}

// Render applies the ctx, t, and design substitutions to content in a single pass each, in that
// order, HTML-escaping substituted values when escapeHTML is set. A nil Resolve skips that family,
// leaving its placeholders untouched.
func Render(content string, escapeHTML bool, ctx, t, design Resolve) string {
	content = Substitute(content, CtxPattern, ctx, escapeHTML)
	content = Substitute(content, TPattern, t, escapeHTML)
	content = Substitute(content, DesignPattern, design, escapeHTML)
	return content
}
