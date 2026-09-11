// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package docs

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	orig := baseURL
	baseURL = srv.URL
	t.Cleanup(func() { baseURL = orig })
	return srv
}

func TestFetchGuide_ReturnsBody(t *testing.T) {
	withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/react.md", r.URL.Path)
		assert.NotEmpty(t, r.Header.Get("User-Agent"))
		_, _ = w.Write([]byte("# React Quickstart"))
	})

	got, err := FetchGuide("react")
	require.NoError(t, err)
	assert.Equal(t, "# React Quickstart", got)
}

func TestFetchGuide_ErrorsOnNon200(t *testing.T) {
	withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := FetchGuide("does-not-exist")
	assert.Error(t, err)
}

func TestSiteURL_NoMarkdownExtension(t *testing.T) {
	assert.Equal(t,
		"https://thunderid.dev/docs/getting-started/connect-your-application/react",
		SiteURL("react"),
	)
}
