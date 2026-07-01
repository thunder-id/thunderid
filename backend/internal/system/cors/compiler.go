/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package cors

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	yaml "gopkg.in/yaml.v3"
)

// UnmarshalYAML decodes a YAML sequence whose elements are either scalar
// strings (literal entries) or mappings of the shape { regex: "..." } (regex
// entries). Anything else is rejected at decode time.
func (e *OriginEntries) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.SequenceNode {
		return fmt.Errorf("cors: allowedOrigins must be a list, got %v", nodeKindString(node.Kind))
	}
	out := make(OriginEntries, 0, len(node.Content))
	for i, child := range node.Content {
		switch child.Kind {
		case yaml.ScalarNode:
			out = append(out, literalEntry{Value: child.Value})
		case yaml.MappingNode:
			var obj struct {
				Regex string `yaml:"regex"`
			}
			if err := child.Decode(&obj); err != nil {
				return fmt.Errorf("cors: allowedOrigins[%d]: %w", i, err)
			}
			if obj.Regex == "" {
				return fmt.Errorf("cors: allowedOrigins[%d]: regex object missing 'regex' field", i)
			}
			out = append(out, regexEntry{Pattern: obj.Regex})
		default:
			return fmt.Errorf("cors: allowedOrigins[%d]: entry must be a string or { regex: ... } object", i)
		}
	}
	*e = out
	return nil
}

// UnmarshalJSON decodes a JSON array whose elements are either strings (literal entries) or objects
// of the shape { "regex": "..." } (regex entries). Anything else is rejected at decode time. The
// stored server-config value is JSON, so this mirrors UnmarshalYAML for the runtime config path.
func (e *OriginEntries) UnmarshalJSON(data []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("cors: allowedOrigins must be a list: %w", err)
	}
	if raw == nil {
		return fmt.Errorf("cors: allowedOrigins must be a list, not null")
	}
	out := make(OriginEntries, 0, len(raw))
	for i, child := range raw {
		var literal string
		if err := json.Unmarshal(child, &literal); err == nil {
			out = append(out, literalEntry{Value: literal})
			continue
		}
		var obj struct {
			Regex string `json:"regex"`
		}
		if err := json.Unmarshal(child, &obj); err != nil {
			return fmt.Errorf("cors: allowedOrigins[%d]: entry must be a string or { regex: ... } object", i)
		}
		if obj.Regex == "" {
			return fmt.Errorf("cors: allowedOrigins[%d]: regex object missing 'regex' field", i)
		}
		out = append(out, regexEntry{Pattern: obj.Regex})
	}
	*e = out
	return nil
}

// nodeKindString renders a yaml.Node kind for diagnostics.
func nodeKindString(k yaml.Kind) string {
	switch k {
	case yaml.DocumentNode:
		return "document"
	case yaml.SequenceNode:
		return "sequence"
	case yaml.MappingNode:
		return "mapping"
	case yaml.ScalarNode:
		return "scalar"
	case yaml.AliasNode:
		return "alias"
	default:
		return "unknown"
	}
}

// Validate checks every entry without installing a matcher.
func Validate(entries OriginEntries) error {
	_, err := compileAll(entries)
	return err
}

// CompileMatcher compiles allowed-origin entries into a Matcher.
func CompileMatcher(entries OriginEntries) (*Matcher, error) {
	rules, err := compileAll(entries)
	if err != nil {
		return nil, err
	}
	return newMatcher(rules), nil
}

// entryKey returns a stable de-duplication key that distinguishes literal and regex entries.
func entryKey(e entry) string {
	switch v := e.(type) {
	case literalEntry:
		return "literal:" + v.Value
	case regexEntry:
		return "regex:" + v.Pattern
	default:
		return ""
	}
}

// toGeneric renders entries as plain values (string or { "regex": ... }) for JSON and YAML encoding.
func (e OriginEntries) toGeneric() ([]any, error) {
	out := make([]any, 0, len(e))
	for _, item := range e {
		switch v := item.(type) {
		case literalEntry:
			out = append(out, v.Value)
		case regexEntry:
			out = append(out, map[string]string{"regex": v.Pattern})
		default:
			return nil, fmt.Errorf("cors: unknown origin entry type %T", item)
		}
	}
	return out, nil
}

// MarshalJSON encodes entries back to the allowedOrigins JSON array; an empty list encodes as [], not null.
func (e OriginEntries) MarshalJSON() ([]byte, error) {
	out, err := e.toGeneric()
	if err != nil {
		return nil, err
	}
	return json.Marshal(out)
}

// MarshalYAML encodes entries to the allowedOrigins YAML sequence, mirroring MarshalJSON.
func (e OriginEntries) MarshalYAML() (any, error) {
	return e.toGeneric()
}

// compile turns one entry into a compiled originRule. Literal entries are
// gated through ParseOrigin and (for non-null entries) canonicalized; regex
// entries are compiled via Go's RE2 engine without any additional validation.
// Operator-supplied regex patterns are taken as-is.
func compile(e entry) (originRule, error) {
	switch v := e.(type) {
	case literalEntry:
		return compileLiteral(v.Value)
	case regexEntry:
		return compileRegex(v.Pattern)
	default:
		return nil, fmt.Errorf("cors: unknown entry type %T", e)
	}
}

// compileAll compiles a slice of entries in declaration order. It fails fast
// on the first invalid entry, reporting the index and underlying cause so the
// operator can locate the bad entry in deployment.yaml. Empty input yields a
// nil slice with no error.
func compileAll(entries []entry) ([]originRule, error) {
	if len(entries) == 0 {
		return nil, nil
	}
	out := make([]originRule, 0, len(entries))
	for i, e := range entries {
		rule, err := compile(e)
		if err != nil {
			return nil, fmt.Errorf("cors: allowedOrigins[%d]: %w", i, err)
		}
		out = append(out, rule)
	}
	return out, nil
}

// compileLiteral builds a literalRule from a YAML bare-string value.
func compileLiteral(value string) (originRule, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, fmt.Errorf("%w: literal value is empty", ErrEmptyEntry)
	}
	if trimmed == "*" {
		return nil, fmt.Errorf("%w: list explicit origins or use a regex entry", ErrWildcardLiteral)
	}
	if trimmed == "null" {
		return literalRule{isNull: true}, nil
	}
	if _, err := ParseOrigin(trimmed); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidLiteral, err)
	}
	canonical, err := canonicalize(trimmed)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidLiteral, err)
	}
	return literalRule{canonical: canonical}, nil
}

// compileRegex builds a regexRule from a YAML regex object's pattern. The
// pattern is compiled by Go's RE2 engine; no further "safety" checks are
// applied — operator owns the pattern.
func compileRegex(pattern string) (originRule, error) {
	if pattern == "" {
		return nil, fmt.Errorf("%w: regex pattern is empty", ErrEmptyEntry)
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidRegex, err)
	}
	return regexRule{re: re}, nil
}

// isRegexAnchored reports whether the given pattern starts with a
// start-of-input anchor (^ or \A) and ends with an end-of-input anchor ($ or
// \z). A pattern lacking either anchor permits substring matches and almost
// always allows far more origins than the operator intended; callers should
// log a warning at boot for unanchored patterns. The check is intentionally
// syntactic — alternation patterns like "(^a|^b)$" are flagged as a false
// positive, which is acceptable given the diagnostic-only intent.
func isRegexAnchored(pattern string) bool {
	starts := strings.HasPrefix(pattern, "^") || strings.HasPrefix(pattern, `\A`)
	ends := strings.HasSuffix(pattern, "$") || strings.HasSuffix(pattern, `\z`)
	return starts && ends
}
