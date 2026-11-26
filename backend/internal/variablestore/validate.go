// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package variablestore

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/system/filter"
	"github.com/thunder-id/thunderid/internal/system/valueref"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// Where each collection is served. These appear in pagination links, so they live next to the code
// that builds them rather than in the handler.
const (
	pathVariables = "/variables"
	pathSecrets   = "/secrets"
)

const (
	defaultLimit = 30
	maxLimit     = 100
)

// validateName checks a name against what the store accepts.
func validateName(name string) *common.ServiceError {
	if name == "" || len(name) > maxNameLength || !nameFormat.MatchString(name) {
		return &ErrorInvalidName
	}
	return nil
}

// validateValue checks a variable's value. An empty value is allowed: a variable that is set to
// nothing is different from one that is not set, and configuration relies on that difference.
func validateValue(value string) *common.ServiceError {
	if len(value) > maxValueLength {
		return &ErrorInvalidValue
	}
	if isReference(value) {
		return &ErrorReferenceValue
	}
	return nil
}

// validateSecretValue checks a secret's value. Unlike a variable, an empty secret is refused: it is
// always a mistake, and one that would otherwise be undetectable later because nothing reads it back.
func validateSecretValue(value string) *common.ServiceError {
	if value == "" || len(value) > maxValueLength {
		return &ErrorInvalidValue
	}
	if isReference(value) {
		return &ErrorReferenceValue
	}
	return nil
}

// isReference reports whether a value is a reference rather than a value: `var:NAME` or `sec:NAME`
// itself, or a list of strings, as a list's variable holds one, with such an item in it. A list
// holding anything but strings is not a list's value and is left as any other text is.
//
// The store holds what a reference stands for, so a reference stored as a value can only come from a
// caller that copied a placeholder where the value should have been. Resolving it would hand the
// placeholder to a resource, or, when it names the variable it is stored under, resolve without end.
func isReference(value string) bool {
	if valueref.IsWellFormedReference(value) {
		return true
	}
	if !strings.HasPrefix(strings.TrimSpace(value), "[") {
		return false
	}
	var items []interface{}
	if err := json.Unmarshal([]byte(value), &items); err != nil {
		return false
	}
	found := false
	for _, item := range items {
		text, isText := item.(string)
		if !isText {
			return false
		}
		found = found || valueref.IsWellFormedReference(text)
	}
	return found
}

func validateDescription(description string) *common.ServiceError {
	if len(description) > maxDescriptionLength {
		return &ErrorInvalidDescription
	}
	return nil
}

// listQueryParams are the query parameters a listing understands. Anything else is refused rather
// than ignored.
var listQueryParams = map[string]bool{
	"limit":  true,
	"offset": true,
	"names":  true,
	"filter": true,
}

// parseListQuery reads paging and narrowing from a query string.
//
// Anything it cannot understand is an error rather than an ignored parameter. A filter that is
// silently dropped returns more than the caller asked for, and for a collection of names that is the
// wrong way to fail: the caller reads a successful response and cannot tell their narrowing was
// never applied. That applies equally to a misspelled parameter, a parameter given twice with
// different values, and one supplied with nothing after the equals sign.
func parseListQuery(query url.Values) (listQuery, *common.ServiceError) {
	for name, values := range query {
		if !listQueryParams[name] {
			return listQuery{}, &ErrorUnknownParameter
		}
		if len(values) != 1 {
			return listQuery{}, &ErrorUnknownParameter
		}
		if strings.TrimSpace(values[0]) == "" {
			return listQuery{}, &ErrorUnknownParameter
		}
	}

	parsed := listQuery{limit: defaultLimit}

	if raw := query.Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > maxLimit {
			return listQuery{}, &ErrorInvalidPagination
		}
		parsed.limit = limit
	}
	if raw := query.Get("offset"); raw != "" {
		offset, err := strconv.Atoi(raw)
		if err != nil || offset < 0 {
			return listQuery{}, &ErrorInvalidPagination
		}
		parsed.offset = offset
	}

	if raw := query.Get("names"); raw != "" {
		names := strings.Split(raw, ",")
		if len(names) > maxNamesPerQuery {
			return listQuery{}, &ErrorTooManyNames
		}
		for _, name := range names {
			trimmed := strings.TrimSpace(name)
			if svcErr := validateName(trimmed); svcErr != nil {
				return listQuery{}, svcErr
			}
			parsed.names = append(parsed.names, trimmed)
		}
	}

	if raw := query.Get("filter"); raw != "" {
		if svcErr := applyFilter(&parsed, raw); svcErr != nil {
			return listQuery{}, svcErr
		}
	}

	return parsed, nil
}

// maxUnchangedForSeconds is the largest age a delete of many names takes, a hundred years, which
// keeps the boundary inside the range of a time both databases hold.
const maxUnchangedForSeconds = 100 * 365 * 24 * 60 * 60

// deleteQuery is a resolved delete of many names: the names, and how long ago a value must have been
// last set to be removed, zero for any time.
type deleteQuery struct {
	names        []string
	unchangedFor time.Duration
}

// parseDeleteQuery reads a delete of many names from the request's raw query: names, required and
// given once, a comma-separated list of 1 to maxNamesPerQuery names, and unchangedFor, optional and
// given at most once, a whole number of seconds from 1 to maxUnchangedForSeconds. Anything else is
// refused, as a listing refuses what it does not understand. The raw query is decoded here rather than
// through URL.Query, which drops a pair it cannot decode: a delete must not go ahead on what is left of
// a query it could not read.
func parseDeleteQuery(rawQuery string) (deleteQuery, *common.ServiceError) {
	query, err := url.ParseQuery(rawQuery)
	if err != nil {
		return deleteQuery{}, &ErrorInvalidDeleteQuery
	}
	for name := range query {
		if name != "names" && name != "unchangedFor" {
			return deleteQuery{}, &ErrorUnknownDeleteParameter
		}
	}
	values := query["names"]
	if len(values) != 1 || strings.TrimSpace(values[0]) == "" {
		return deleteQuery{}, &ErrorNamesRequired
	}
	raw := strings.Split(values[0], ",")
	parsed := deleteQuery{names: make([]string, 0, len(raw))}
	for _, name := range raw {
		parsed.names = append(parsed.names, strings.TrimSpace(name))
	}
	if svcErr := validateNames(parsed.names); svcErr != nil {
		return deleteQuery{}, svcErr
	}
	if ages, given := query["unchangedFor"]; given {
		if len(ages) != 1 {
			return deleteQuery{}, &ErrorInvalidUnchangedFor
		}
		seconds, err := strconv.ParseInt(ages[0], 10, 64)
		if err != nil || seconds < 1 || seconds > maxUnchangedForSeconds {
			return deleteQuery{}, &ErrorInvalidUnchangedFor
		}
		parsed.unchangedFor = time.Duration(seconds) * time.Second
	}
	return parsed, nil
}

// validateNames checks the names a delete of many names removes: 1 to maxNamesPerQuery of them, each
// a valid name.
func validateNames(names []string) *common.ServiceError {
	if len(names) == 0 {
		return &ErrorNamesRequired
	}
	if len(names) > maxNamesPerQuery {
		return &ErrorTooManyNames
	}
	for _, name := range names {
		if svcErr := validateName(name); svcErr != nil {
			return svcErr
		}
	}
	return nil
}

// applyFilter reads the filter expression this store understands: name eq "x" or name sw "x".
//
// The grammar itself is the shared one in internal/system/filter, so an expression means here what
// it means on every other collection. What this narrows is which of it applies: one clause, on the
// one attribute a name collection has, with the two operators that are meaningful for names.
// Anything else is refused rather than partly honored, because a filter that is silently widened
// returns more than the caller asked for and the caller cannot tell from a successful response.
func applyFilter(parsed *listQuery, expression string) *common.ServiceError {
	group, err := filter.ParseFilterGroup(strings.TrimSpace(expression))
	if err != nil || group == nil || len(group.Clauses) != 1 {
		return &ErrorInvalidFilter
	}

	expr := group.Clauses[0].Expr
	if !strings.EqualFold(expr.Attribute, "name") {
		return &ErrorInvalidFilter
	}
	operand, ok := expr.Value.(string)
	if !ok || operand == "" {
		return &ErrorInvalidFilter
	}
	// The shared grammar also accepts an unquoted operand, so an operand has to be checked against
	// what a name may be rather than taken as given. Without this a malformed one is a legal
	// expression naming something that cannot exist, and the caller gets an empty result instead of
	// being told their filter was wrong. A prefix of a valid name is itself a valid name, so the
	// same check serves both operators.
	if svcErr := validateName(operand); svcErr != nil {
		return &ErrorInvalidFilter
	}

	switch expr.Operator {
	case common.OperatorEq:
		parsed.nameEquals = operand
	case common.OperatorSw:
		parsed.namePrefix = operand
	default:
		return &ErrorInvalidFilter
	}
	return nil
}

// buildLinks describes the neighboring pages, and omits a link that would lead nowhere.
func buildLinks(path string, q listQuery, total int) []Link {
	links := make([]Link, 0, 2)
	if q.offset+q.limit < total {
		links = append(links, Link{Href: pageHref(path, q, q.offset+q.limit), Rel: "next"})
	}
	if q.offset > 0 {
		previous := q.offset - q.limit
		if previous < 0 {
			previous = 0
		}
		links = append(links, Link{Href: pageHref(path, q, previous), Rel: "previous"})
	}
	return links
}

// pageHref keeps the query's narrowing on the link, so following a page does not widen the result.
func pageHref(path string, q listQuery, offset int) string {
	values := url.Values{}
	values.Set("offset", strconv.Itoa(offset))
	values.Set("limit", strconv.Itoa(q.limit))
	if len(q.names) > 0 {
		values.Set("names", strings.Join(q.names, ","))
	}
	switch {
	case q.nameEquals != "":
		values.Set("filter", fmt.Sprintf("name eq %q", q.nameEquals))
	case q.namePrefix != "":
		values.Set("filter", fmt.Sprintf("name sw %q", q.namePrefix))
	}
	return path + "?" + values.Encode()
}
