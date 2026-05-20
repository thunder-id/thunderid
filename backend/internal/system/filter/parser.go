/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package filter

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// filterPattern matches a complete single expression (with end anchor) for validation.
var filterPattern = regexp.MustCompile(`^(\w+(?:\.\w+)*)\s+(eq|gt|lt)\s+(?:"([^"]*)"|(\S+))$`)

// singleExprPrefix matches one expression from the start of the string without an end anchor,
// used during iterative multi-expression parsing.
var singleExprPrefix = regexp.MustCompile(`^(\w+(?:\.\w+)*)\s+(eq|gt|lt)\s+(?:"([^"]*)"|(\S+))`)

// connectorPrefix matches a leading AND or OR connector (case-insensitive) surrounded by whitespace.
var connectorPrefix = regexp.MustCompile(`(?i)^\s+(AND|OR)\s+`)

// ParseFilterParam reads the "filter" query parameter and parses it into a FilterGroup.
// Returns nil when no filter parameter is present.
func ParseFilterParam(query url.Values) (*FilterGroup, error) {
	if !query.Has("filter") {
		return nil, nil
	}

	filterStr := strings.TrimSpace(query.Get("filter"))
	if filterStr == "" {
		return nil, fmt.Errorf("filter parameter is empty")
	}

	return ParseFilterGroup(filterStr)
}

// ParseFilterGroup parses a filter string that may contain multiple expressions joined by AND or OR.
// AND has higher precedence than OR, matching standard SQL behavior.
// Examples:
//
//	name eq "Engineering"
//	name eq "Engineering" AND createdAt gt "2024-01-01T00:00:00Z"
//	name eq "A" OR name eq "B"
func ParseFilterGroup(filterStr string) (*FilterGroup, error) {
	remaining := filterStr
	connector := LogicalOperator("")
	var clauses []FilterClause

	for remaining != "" {
		loc := singleExprPrefix.FindStringIndex(remaining)
		if len(loc) == 0 || loc[0] != 0 {
			return nil, fmt.Errorf("invalid filter format near: %q", remaining)
		}

		expr, err := parseExpressionFromMatches(singleExprPrefix.FindStringSubmatch(remaining))
		if err != nil {
			return nil, err
		}
		clauses = append(clauses, FilterClause{Connector: connector, Expr: *expr})
		remaining = remaining[loc[1]:]

		if remaining == "" {
			break
		}

		m := connectorPrefix.FindStringSubmatch(remaining)
		if m == nil {
			return nil, fmt.Errorf("expected AND or OR after expression, got: %q", remaining)
		}
		connector = LogicalOperator(strings.ToUpper(m[1]))
		remaining = remaining[len(m[0]):]
	}

	if len(clauses) == 0 {
		return nil, fmt.Errorf("no filter expressions found")
	}

	return &FilterGroup{Clauses: clauses}, nil
}

// ParseFilterExpression parses a single filter expression string of the form:
//
//	attribute (eq|gt|lt) "value"
//	attribute (eq|gt|lt) value
func ParseFilterExpression(filterStr string) (*FilterExpression, error) {
	matches := filterPattern.FindStringSubmatch(filterStr)
	if len(matches) == 0 {
		return nil, fmt.Errorf("invalid filter format: %q", filterStr)
	}
	return parseExpressionFromMatches(matches)
}

// parseExpressionFromMatches extracts a FilterExpression from a regex match slice.
// Slot layout: [full, attribute, operator, quotedValue, unquotedValue].
func parseExpressionFromMatches(matches []string) (*FilterExpression, error) {
	if len(matches) < 5 {
		return nil, fmt.Errorf("invalid filter format")
	}

	attribute := matches[1]
	op := Operator(matches[2])

	var value interface{}
	if matches[3] != "" {
		value = matches[3]
	} else {
		raw := matches[4]
		if intVal, err := strconv.ParseInt(raw, 10, 64); err == nil {
			value = intVal
		} else if floatVal, err := strconv.ParseFloat(raw, 64); err == nil {
			value = floatVal
		} else if boolVal, err := strconv.ParseBool(raw); err == nil {
			value = boolVal
		} else {
			return nil, fmt.Errorf("invalid filter value: %q", raw)
		}
	}

	return &FilterExpression{
		Attribute: attribute,
		Operator:  op,
		Value:     value,
	}, nil
}
