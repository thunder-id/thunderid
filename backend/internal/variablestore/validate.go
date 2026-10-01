// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package variablestore

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/filter"
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
	return nil
}

// validateSecretValue checks a secret's value. Unlike a variable, an empty secret is refused: it is
// always a mistake, and one that would otherwise be undetectable later because nothing reads it back.
func validateSecretValue(value string) *common.ServiceError {
	if value == "" || len(value) > maxValueLength {
		return &ErrorInvalidValue
	}
	return nil
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
