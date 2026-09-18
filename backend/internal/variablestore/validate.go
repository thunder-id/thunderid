// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package variablestore

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

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

// applyFilter reads the one filter expression this store understands: name eq "x" or name sw "x".
//
// Deliberately not a general expression language. There is one filterable attribute, and a parser
// for a grammar nobody asked for is a parser with bugs nobody finds.
func applyFilter(parsed *listQuery, expression string) *common.ServiceError {
	fields := strings.Fields(strings.TrimSpace(expression))
	if len(fields) != 3 || !strings.EqualFold(fields[0], "name") {
		return &ErrorInvalidFilter
	}

	operand, ok := unquote(fields[2])
	if !ok {
		return &ErrorInvalidFilter
	}

	switch strings.ToLower(fields[1]) {
	case "eq":
		parsed.nameEquals = operand
	case "sw":
		parsed.namePrefix = operand
	default:
		return &ErrorInvalidFilter
	}
	return nil
}

// unquote reads the operand of a filter expression, which the grammar requires to be quoted.
//
// The pair has to match. Trimming quote characters off both ends would accept an unquoted operand
// and a mismatched pair alike, and answer them as though they had been understood, which is the
// failure mode a filter can least afford: the caller reads a successful response and cannot tell
// their expression was not the one applied.
func unquote(operand string) (string, bool) {
	if len(operand) < 3 {
		return "", false
	}
	quote := operand[0]
	if quote != '"' && quote != '\'' {
		return "", false
	}
	if operand[len(operand)-1] != quote {
		return "", false
	}

	inner := operand[1 : len(operand)-1]
	if inner == "" || strings.ContainsRune(inner, rune(quote)) {
		return "", false
	}
	return inner, true
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
