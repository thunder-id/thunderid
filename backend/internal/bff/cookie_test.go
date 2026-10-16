// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package bff

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCookieSpec_Prefixes(t *testing.T) {
	assert.Equal(t, "__Host-a", newCookieSpec("a", "/", true).name)
	assert.Equal(t, "__Secure-a", newCookieSpec("a", "/x", true).name)
	assert.Equal(t, "a", newCookieSpec("a", "/", false).name)
}

func TestCookieSpec_ChunksLongValues(t *testing.T) {
	spec := newCookieSpec("s", "/", true)
	value := strings.Repeat("v", maxCookieChunk*2+10)

	rec := httptest.NewRecorder()
	spec.write(rec, httptest.NewRequest(http.MethodGet, "/", nil), value, 0)
	cookies := rec.Result().Cookies()
	require.Len(t, cookies, 3)
	for i, c := range cookies {
		assert.Equal(t, spec.chunkName(i), c.Name)
		assert.LessOrEqual(t, len(c.Value), maxCookieChunk)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	got, ok := spec.read(req)
	assert.True(t, ok)
	assert.Equal(t, value, got)

	// A short rewrite replaces the chunks and expires the ones no longer used.
	rec = httptest.NewRecorder()
	spec.write(rec, req, "short", 0)
	written := map[string]int{}
	for _, c := range rec.Result().Cookies() {
		written[c.Name] = c.MaxAge
	}
	assert.Equal(t, map[string]int{"__Host-s": 0, "__Host-s.0": -1, "__Host-s.1": -1, "__Host-s.2": -1}, written)
}

func TestCookieSpec_ClearExpiresCookieNotSentToThisPath(t *testing.T) {
	spec := newCookieSpec("r", "/auth/refresh", true)

	rec := httptest.NewRecorder()
	spec.clear(rec, httptest.NewRequest(http.MethodPost, "/auth/signout", nil))

	cookies := rec.Result().Cookies()
	require.Len(t, cookies, 1)
	assert.Equal(t, "__Secure-r", cookies[0].Name)
	assert.Equal(t, "/auth/refresh", cookies[0].Path)
	assert.Equal(t, -1, cookies[0].MaxAge)
}

func TestCookieSpec_IsChunk(t *testing.T) {
	spec := newCookieSpec("s", "/", false)
	assert.True(t, spec.isChunk("s.0"))
	assert.False(t, spec.isChunk("s.x"))
	assert.False(t, spec.isChunk("s_other.0"))
}
