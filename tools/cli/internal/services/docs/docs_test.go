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
	orig := siteURL
	siteURL = srv.URL
	t.Cleanup(func() { siteURL = orig })
	return srv
}

func TestFetchGuide_ReturnsBody(t *testing.T) {
	withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1.1.x/getting-started/connect-your-application/react.md", r.URL.Path)
		assert.NotEmpty(t, r.Header.Get("User-Agent"))
		_, _ = w.Write([]byte("# React Quickstart"))
	})

	got, err := FetchGuide("1.1.0", "react")
	require.NoError(t, err)
	assert.Equal(t, "# React Quickstart", got)
}

func TestFetchGuide_ErrorsOnNon200(t *testing.T) {
	withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := FetchGuide("1.1.0", "does-not-exist")
	assert.Error(t, err)
}

func TestSiteURL_NoMarkdownExtension(t *testing.T) {
	assert.Equal(t,
		"https://thunderid.dev/docs/v1.1.x/getting-started/connect-your-application/react",
		SiteURL("1.1.0", "react"),
	)
}

// The docs follow the installed release, so a new release needs no change to the CLI.
func TestTree_FollowsTheReleaseLine(t *testing.T) {
	tests := map[string]string{
		"1.0.2":     "https://thunderid.dev/docs/v1.0.x",
		"1.1.0":     "https://thunderid.dev/docs/v1.1.x",
		"v2.0.0":    "https://thunderid.dev/docs/v2.0.x",
		"2.0.0-m1":  "https://thunderid.dev/docs/next",
		"2.0.0-rc1": "https://thunderid.dev/docs/next",
		"":          "https://thunderid.dev/docs/next",
	}
	for version, want := range tests {
		assert.Equal(t, want, Tree(version), version)
	}
}
