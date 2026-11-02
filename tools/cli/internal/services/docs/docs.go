// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package docs fetches per-platform integration guides from the ThunderID docs site.
package docs

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/thunder-id/thunderid/tools/cli/internal/product"
)

var client = &http.Client{Timeout: 10 * time.Second}

// baseURL is a var (not product.DocsBaseURL directly) so tests can point it at
// an httptest.Server instead of the live docs site.
var baseURL = product.DocsBaseURL

// FetchGuide downloads the raw markdown "connect your application" guide for slug
// (e.g. "react", "node", "browser") from product.DocsBaseURL.
func FetchGuide(slug string) (string, error) {
	url := baseURL + "/" + slug + ".md"

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", product.Slug+"-cli")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// SiteURL returns the human-facing HTML page for slug, for "open in browser" links.
func SiteURL(slug string) string {
	return product.DocsBaseURL + "/" + slug
}
