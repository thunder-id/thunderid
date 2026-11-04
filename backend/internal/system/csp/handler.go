// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package csp

import (
	"context"
	"encoding/json"
)

// PolicyHandler decodes, validates, and merges the "csp" server-config section. It implements the
// section-handler contract structurally, without importing the serverconfig package.
type PolicyHandler struct{}

// Decode parses a raw JSON csp value into PolicyConfig. Empty input yields the zero value, the
// deny-first baseline in report-only mode.
func (PolicyHandler) Decode(raw json.RawMessage) (any, error) {
	if len(raw) == 0 {
		return PolicyConfig{}, nil
	}
	var cfg PolicyConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate checks that the incoming value is an acceptable csp policy.
func (PolicyHandler) Validate(_ context.Context, incoming, _, _ any) error {
	cfg, _ := incoming.(PolicyConfig)
	return cfg.Validate()
}

// Merge overlays the writable (db) layer onto the read-only (declarative) layer: an explicit writable
// value wins per field, directive, and path.
func (PolicyHandler) Merge(readOnly, writable any) any {
	ro, _ := readOnly.(PolicyConfig)
	wr, _ := writable.(PolicyConfig)

	merged := PolicyConfig{ReportOnly: ro.ReportOnly, ReportURI: ro.ReportURI}
	if wr.ReportOnly != nil {
		merged.ReportOnly = wr.ReportOnly
	}
	if wr.ReportURI != "" {
		merged.ReportURI = wr.ReportURI
	}
	merged.Directives = mergeDirectives(ro.Directives, wr.Directives)
	merged.Paths = mergePaths(ro.Paths, wr.Paths)
	return merged
}

// mergePaths overlays writable path policies onto read-only ones by prefix: a match replaces the
// read-only entry; entries present in only one layer are kept. Returns nil when both are empty.
func mergePaths(readOnly, writable []PathPolicy) []PathPolicy {
	if len(readOnly) == 0 && len(writable) == 0 {
		return nil
	}
	byPrefix := make(map[string]map[string][]string, len(readOnly)+len(writable))
	order := make([]string, 0, len(readOnly)+len(writable))
	for _, layer := range [][]PathPolicy{readOnly, writable} {
		for _, p := range layer {
			if _, seen := byPrefix[p.Location]; !seen {
				order = append(order, p.Location)
			}
			byPrefix[p.Location] = p.Directives
		}
	}
	out := make([]PathPolicy, 0, len(order))
	for _, prefix := range order {
		out = append(out, PathPolicy{Location: prefix, Directives: byPrefix[prefix]})
	}
	return out
}

// mergeDirectives overlays writable directives onto read-only ones: a directive in the writable layer
// replaces the read-only entry for that directive. Returns nil when both layers are empty.
func mergeDirectives(readOnly, writable map[string][]string) map[string][]string {
	if len(readOnly) == 0 && len(writable) == 0 {
		return nil
	}
	out := make(map[string][]string, len(readOnly)+len(writable))
	for name, sources := range readOnly {
		out[name] = sources
	}
	for name, sources := range writable {
		out[name] = sources
	}
	return out
}
