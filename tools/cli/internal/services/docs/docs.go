// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package docs fetches per-platform integration guides from the ThunderID docs site.
package docs

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"

	"github.com/thunder-id/thunderid/tools/cli/internal/product"
)

var client = &http.Client{Timeout: 10 * time.Second}

// siteURL is a var (not product.DocsURL directly) so tests can point it at
// an httptest.Server instead of the live docs site.
var siteURL = product.DocsURL

// releaseVersion matches a stable release version and captures its major and minor.
var releaseVersion = regexp.MustCompile(`^v?(\d+)\.(\d+)\.\d+$`)

// Tree returns the docs tree that documents version: the release line's for a stable release
// (1.1.0 reads /docs/v1.1.x), and "next" for a prerelease or a version that cannot be read, since
// a prerelease is cut from the development line those docs follow.
func Tree(version string) string {
	if m := releaseVersion.FindStringSubmatch(version); m != nil {
		return siteURL + "/v" + m[1] + "." + m[2] + ".x"
	}
	return siteURL + "/next"
}

// guideURL returns the guide directory in the docs tree for version.
func guideURL(version string) string {
	return Tree(version) + product.DocsGuidePath
}

// FetchGuide downloads the raw markdown "connect your application" guide for slug
// (e.g. "react", "node", "browser") from the docs of the given product version.
func FetchGuide(version, slug string) (string, error) {
	url := guideURL(version) + "/" + slug + ".md"

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

// SiteURL returns the human-facing HTML page for slug in the docs of the given product version,
// for "open in browser" links.
func SiteURL(version, slug string) string {
	return guideURL(version) + "/" + slug
}
