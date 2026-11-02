// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strings"
)

// QueryParamInclude is the query parameter name for the include parameter.
const QueryParamInclude = "include"

// QueryParamFilter is the query parameter name for the filter parameter.
const QueryParamFilter = "filter"

// IncludeValueDisplay is the value for the include query parameter to request display attributes.
const IncludeValueDisplay = "display"

// IncludeDisplayQuery is the query string fragment appended to pagination links
// when the include=display parameter is active.
const IncludeDisplayQuery = "&" + QueryParamInclude + "=" + IncludeValueDisplay

// DisplayQueryParam returns IncludeDisplayQuery if includeDisplay is true, empty string otherwise.
func DisplayQueryParam(includeDisplay bool) string {
	if includeDisplay {
		return IncludeDisplayQuery
	}
	return ""
}

// FilterQueryParam returns the query string fragment that carries an equality filter on pagination
// links, so a link to another page lists the same results. It returns an empty string for no filter.
func FilterQueryParam(filters map[string]interface{}) string {
	if len(filters) == 0 {
		return ""
	}
	attributes := slices.Sorted(maps.Keys(filters))
	expressions := make([]string, 0, len(attributes))
	for _, attribute := range attributes {
		if value, ok := filters[attribute].(string); ok {
			// The filter parser reads a quoted value verbatim, so the value is not escaped.
			expressions = append(expressions, attribute+` eq "`+value+`"`)
			continue
		}
		expressions = append(expressions, fmt.Sprintf("%s eq %v", attribute, filters[attribute]))
	}
	return "&" + QueryParamFilter + "=" + url.QueryEscape(strings.Join(expressions, " and "))
}

// Link represents a pagination link in API responses.
type Link struct {
	Href string `json:"href"`
	Rel  string `json:"rel"`
}

// BuildPaginationLinks builds pagination links for paginated API responses.
// extraQuery is an optional query string fragment (e.g. "&include=display") appended to each link.
func BuildPaginationLinks(base string, limit, offset, totalCount int, extraQuery string) []Link {
	links := make([]Link, 0)

	if limit <= 0 {
		return links
	}

	if offset > 0 {
		links = append(links, Link{
			Href: fmt.Sprintf("%s?offset=0&limit=%d%s", base, limit, extraQuery),
			Rel:  "first",
		})

		prevOffset := offset - limit
		if prevOffset < 0 {
			prevOffset = 0
		}
		links = append(links, Link{
			Href: fmt.Sprintf("%s?offset=%d&limit=%d%s", base, prevOffset, limit, extraQuery),
			Rel:  "prev",
		})
	}

	if offset+limit < totalCount {
		nextOffset := offset + limit
		links = append(links, Link{
			Href: fmt.Sprintf("%s?offset=%d&limit=%d%s", base, nextOffset, limit, extraQuery),
			Rel:  "next",
		})
	}

	lastPageOffset := ((totalCount - 1) / limit) * limit
	if offset < lastPageOffset {
		links = append(links, Link{
			Href: fmt.Sprintf("%s?offset=%d&limit=%d%s", base, lastPageOffset, limit, extraQuery),
			Rel:  "last",
		})
	}

	return links
}

// PaginateOwnedThenShared returns one page of a list that holds the resources an organization unit
// owns, followed by the ones shared into it. Only the page is read: owned resources through
// listOwned, shared ones by loading the page's share of sharedIDs through loadShared, kept in the
// order sharedIDs gives. A shared ID that loads nothing is left out.
func PaginateOwnedThenShared[T any](
	ownedCount int, listOwned func(limit, offset int) ([]T, error),
	sharedIDs []string, loadShared func(ids []string) ([]T, error), idOf func(T) string,
	limit, offset int,
) ([]T, []T, error) {
	owned := []T{}
	ownedOnPage := 0
	if offset < ownedCount {
		ownedOnPage = min(limit, ownedCount-offset)
		page, err := listOwned(ownedOnPage, offset)
		if err != nil {
			return nil, nil, err
		}
		owned = page
	}

	shared := []T{}
	start := max(0, offset-ownedCount)
	if ownedOnPage == limit || start >= len(sharedIDs) {
		return owned, shared, nil
	}
	pageIDs := sharedIDs[start:min(len(sharedIDs), start+limit-ownedOnPage)]
	loaded, err := loadShared(pageIDs)
	if err != nil {
		return nil, nil, err
	}
	byID := make(map[string]T, len(loaded))
	for _, item := range loaded {
		byID[idOf(item)] = item
	}
	for _, id := range pageIDs {
		if item, ok := byID[id]; ok {
			shared = append(shared, item)
		}
	}
	return owned, shared, nil
}
