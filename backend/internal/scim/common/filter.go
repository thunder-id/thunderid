// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/database/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

var (
	scimFilterQuotedStringRe = regexp.MustCompile(`"([^"\\]|\\.)*"`)
	scimFilterEqRe           = regexp.MustCompile(`(?i)^((?:[A-Za-z0-9][A-Za-z0-9.\-_]*:)*)` +
		`([A-Za-z][A-Za-z0-9.\-_]*)\s+eq\s+(.+)$`)
	scimFilterAndRe           = regexp.MustCompile(`(?i)\s+and\s+`)
	scimFilterUnsupportedOpRe = regexp.MustCompile(`(?i)(?:^|\s)(?:ne|co|sw|ew|pr|gt|lt|ge|le)(?:\s|$)`)
)

// FilterAttrRules bundles the resource-specific attribute checks used when parsing
// SCIM filter expressions, so the grammar in this file stays resource-agnostic and
// reusable by any resource (users today, groups or others later).
type FilterAttrRules struct {
	IsUnsupported func(attr string) bool
	IsCore        func(attr string) bool
	Translate     func(attr string) string
}

// ParseSCIMFilterForEq parses a SCIM filter string containing one or more
// "eq" comparisons joined by "and", with no "or", "not", grouping, or square
// brackets. Returns a native filter map suitable for a resource's list query,
// or a ServiceError with a specific detail if the expression uses any
// unsupported syntax.
func ParseSCIMFilterForEq(filterStr string, rules FilterAttrRules) (map[string]interface{}, *tidcommon.ServiceError) {
	filterStr = strings.TrimSpace(filterStr)
	if filterStr == "" {
		return nil, nil
	}
	sanitized := scimFilterQuotedStringRe.ReplaceAllString(filterStr, `""`)
	lower := strings.ToLower(sanitized)
	// Reject all unsupported compound/complex expressions up front (outside of quoted strings).
	if strings.Contains(lower, " or ") ||
		strings.HasPrefix(lower, "not ") ||
		strings.ContainsAny(sanitized, "()[]") {
		return nil, newInvalidFilterSyntaxError(
			"compound filter expressions are only supported using 'and'; " +
				"'or', 'not', and grouping are not supported",
		)
	}
	// Reject any operator that is not "eq". Matched as whole word tokens so
	// attribute names that merely contain these letters (e.g. "preferredLanguage")
	// are not mistaken for the "pr" operator.
	if scimFilterUnsupportedOpRe.MatchString(lower) {
		return nil, newInvalidFilterSyntaxError(
			"the specified filter operator is not supported; only 'eq' is supported",
		)
	}
	filters := make(map[string]interface{})
	for _, clause := range splitSCIMAndClauses(filterStr) {
		attribute, value, err := parseSCIMEqClause(clause, rules)
		if err != nil {
			return nil, newInvalidFilterSyntaxError(err.Error())
		}
		if _, exists := filters[attribute]; exists {
			return nil, newInvalidFilterSyntaxError(
				fmt.Sprintf("attribute %q is repeated in the filter expression", attribute))
		}
		filters[attribute] = value
	}
	return filters, nil
}

// splitSCIMAndClauses splits a SCIM filter string on "and" (case-insensitive),
// ignoring any occurrence of "and" that falls inside a quoted string value.
func splitSCIMAndClauses(filterStr string) []string {
	quotedSpans := scimFilterQuotedStringRe.FindAllStringIndex(filterStr, -1)
	insideQuotes := func(pos int) bool {
		for _, span := range quotedSpans {
			if pos >= span[0] && pos < span[1] {
				return true
			}
		}
		return false
	}
	clauses := make([]string, 0, 1)
	last := 0
	for _, m := range scimFilterAndRe.FindAllStringIndex(filterStr, -1) {
		if insideQuotes(m[0]) {
			continue
		}
		clauses = append(clauses, filterStr[last:m[0]])
		last = m[1]
	}
	clauses = append(clauses, filterStr[last:])
	return clauses
}

// parseSCIMEqClause parses a single "[optional-URN-prefix:]attrPath eq compValue"
// clause into its native attribute name and typed comparison value.
func parseSCIMEqClause(clause string, rules FilterAttrRules) (string, interface{}, error) {
	// Match: [optional-URN-prefix:]attrPath eq compValue
	// attrPath allows alphanumeric, underscore, hyphen, and dot (for sub-attributes).
	// compValue is a quoted string, a boolean literal, or a number.
	matches := scimFilterEqRe.FindStringSubmatch(strings.TrimSpace(clause))
	if len(matches) == 0 {
		return "", nil, fmt.Errorf(
			"invalid filter expression; expected format: 'attrPath eq value'",
		)
	}
	// matches[1] = optional URN prefix (e.g. "urn:thunderid:params:scim:schemas:employee:2.0:User:")
	// matches[2] = attribute path (e.g. "profile.manager.id")
	// matches[3] = raw comparison value
	if rules.IsUnsupported(matches[2]) {
		return "", nil, fmt.Errorf("filtering on %q is not supported", matches[2])
	}
	// Custom/extension attributes must be qualified with their schema URN per RFC 7643 §3.10;
	// core schema attributes may be referenced with or without a URN prefix.
	if strings.TrimSuffix(matches[1], ":") == "" && !rules.IsCore(matches[2]) {
		return "", nil, fmt.Errorf(
			"filtering on custom attribute %q requires a schema URN prefix", matches[2])
	}
	attribute := rules.Translate(matches[2])
	if err := utils.ValidateKey(attribute); err != nil {
		return "", nil, fmt.Errorf("filtering on %q is not supported", matches[2])
	}
	rawValue := strings.TrimSpace(matches[3])
	value, err := parseSCIMCompValue(rawValue)
	if err != nil {
		return "", nil, err
	}
	return attribute, value, nil
}

// parseSCIMCompValue converts a raw SCIM compValue token into a typed Go value.
// compValue = false / null / true / number / string  (RFC 7159 JSON rules)
func parseSCIMCompValue(raw string) (interface{}, error) {
	// Quoted string — parse as a JSON string literal so escapes are handled correctly.
	if len(raw) > 0 && raw[0] == '"' {
		s, err := strconv.Unquote(raw)
		if err == nil {
			return s, nil
		}
		return nil, fmt.Errorf("invalid quoted string comparison value: %q", raw)
	}
	lower := strings.ToLower(raw)
	switch lower {
	case "true":
		return true, nil
	case "false":
		return false, nil
	case "null":
		// null comparisons are not meaningful for our store.
		return nil, fmt.Errorf("null comparison values are not supported")
	}
	// Integer
	if intVal, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return intVal, nil
	}
	// Decimal
	if floatVal, err := strconv.ParseFloat(raw, 64); err == nil {
		return floatVal, nil
	}
	return nil, fmt.Errorf("unrecognized comparison value: %q", raw)
}
