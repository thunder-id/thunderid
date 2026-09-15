// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"mime"
	"net/http"
	"strconv"
	"strings"

	scimconfig "github.com/thunder-id/thunderid/internal/scim/config"
	"github.com/thunder-id/thunderid/internal/system/constants"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// ValidateSCIMContentType enforces RFC 7644 §3.1 — write requests must carry
// Content-Type: application/scim+json.
func ValidateSCIMContentType(r *http.Request) *tidcommon.ServiceError {
	mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if !strings.EqualFold(mediaType, constants.SCIMContentType) {
		return &errorInvalidContentType
	}
	return nil
}

// ParseSCIMPaginationQueryParams extracts and clamps startIndex and count query parameters
// per RFC 7644 §3.4.2.4.
func ParseSCIMPaginationQueryParams(r *http.Request) (int, int) {
	startIndex := 1
	if v := strings.TrimSpace(r.URL.Query().Get("startIndex")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			startIndex = n
		}
	}
	var count *int
	if v := strings.TrimSpace(r.URL.Query().Get("count")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			count = &n
		}
	}
	return NormalizeSCIMPagination(startIndex, count)
}

// NormalizeSCIMPagination clamps startIndex and count per RFC 7644 §3.4.2.4. A nil count means
// count was not specified, and falls back to the default page size; a negative count is
// interpreted as 0 (no resources, totalResults only), matching an explicit 0.
func NormalizeSCIMPagination(startIndex int, count *int) (int, int) {
	if startIndex < 1 {
		startIndex = 1
	}
	resolvedCount := constants.DefaultPageSize
	if count != nil {
		resolvedCount = *count
		if resolvedCount < 0 {
			resolvedCount = 0
		}
	}
	if resolvedCount > scimconfig.FilterMaxResults {
		resolvedCount = scimconfig.FilterMaxResults
	}
	return startIndex, resolvedCount
}
