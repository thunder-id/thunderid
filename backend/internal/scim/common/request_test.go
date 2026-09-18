// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/constants"
)

// TestParseSCIMPagination tests Parse SCIM Pagination.
func TestParseSCIMPagination(t *testing.T) {
	t.Run("ValidParams", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users?startIndex=5&count=20", nil)
		start, count := ParseSCIMPaginationQueryParams(req)
		require.Equal(t, 5, start)
		require.Equal(t, 20, count)
	})

	t.Run("MissingParamsUseDefaults", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users?startIndex=invalid", nil)
		start, count := ParseSCIMPaginationQueryParams(req)
		require.Equal(t, 1, start)
		require.Equal(t, constants.DefaultPageSize, count)
	})

	t.Run("NegativeCountInterpretedAsZero", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users?count=-10", nil)
		_, count := ParseSCIMPaginationQueryParams(req)
		require.Equal(t, 0, count)
	})

	t.Run("ExplicitZeroCountPreserved", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users?count=0", nil)
		_, count := ParseSCIMPaginationQueryParams(req)
		require.Equal(t, 0, count)
	})

	t.Run("ClampedCount", func(t *testing.T) {
		count := 500
		start, resolved := NormalizeSCIMPagination(0, &count)
		require.Equal(t, 1, start)
		require.Equal(t, 100, resolved)
	})

	t.Run("NilCountUsesDefault", func(t *testing.T) {
		start, count := NormalizeSCIMPagination(0, nil)
		require.Equal(t, 1, start)
		require.Equal(t, constants.DefaultPageSize, count)
	})
}
