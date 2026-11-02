// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package release

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"

	"github.com/thunder-id/thunderid/tools/cli/internal/product"
)

// Client resolves releases from a manifest and downloads their assets.
//
// The zero value is not useful; use Default, or build one with Configure. Default exists so the
// package-level helpers keep working for the callers that only ever want the public releases:
// the release source is process-wide configuration derived from the command line, in the same way
// GOPROXY or npm_config_registry are.
type Client struct {
	// ManifestURL is the releases manifest to read.
	ManifestURL string
	// FallbackURL is the GitHub release API to fall back to. Empty disables the fallback, which
	// is what a custom ManifestURL implies: quietly reaching for the public API when the operator
	// pointed at their own mirror would defeat the point of pointing at it.
	FallbackURL string
	// PinnedVersion is the release to install, without a leading "v". Empty means the latest.
	PinnedVersion string
}

// Default is the client the package-level helpers use. main replaces it once the command line
// has been parsed.
var Default = &Client{
	ManifestURL: product.ReleasesURL,
	FallbackURL: product.GitHubAPI,
}

// IsCustom reports whether releases come from somewhere other than the public manifest.
func (c *Client) IsCustom() bool { return c.ManifestURL != product.ReleasesURL }

// Pinned returns the pinned version, or "" when the latest release is wanted.
func Pinned() string { return Default.PinnedVersion }

// SourceURL returns the manifest the CLI is reading releases from.
func SourceURL() string { return Default.ManifestURL }

// IsCustomSource reports whether releases come from somewhere other than the public manifest.
func IsCustomSource() bool { return Default.IsCustom() }

// semverPattern matches the release versions the manifest carries: three numeric components with
// an optional pre-release suffix, and an optional leading "v".
var semverPattern = regexp.MustCompile(`^v?\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$`)

// Configure builds a Client from --product-version values.
//
// Each value is classified by its shape: a URL selects the manifest to read, a semantic version
// pins the release to install. Both may be given, in either order, so "this version from that
// mirror" is expressible. Two of the same kind is a mistake rather than a last-one-wins.
func Configure(values []string) (*Client, error) {
	client := &Client{ManifestURL: product.ReleasesURL, FallbackURL: product.GitHubAPI}
	var sawURL, sawVersion bool

	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}

		switch {
		case looksLikeURL(value):
			if sawURL {
				return nil, fmt.Errorf("--product-version was given two release sources; pass one URL at most")
			}
			if err := validateSourceURL(value); err != nil {
				return nil, err
			}
			// A custom manifest is authoritative: no silent fallback to the public GitHub API.
			client.ManifestURL = value
			client.FallbackURL = ""
			sawURL = true

		case semverPattern.MatchString(value):
			if sawVersion {
				return nil, fmt.Errorf("--product-version was given two versions; pass one at most")
			}
			client.PinnedVersion = strings.TrimPrefix(value, "v")
			sawVersion = true

		default:
			return nil, fmt.Errorf(
				"--product-version %q is neither a version nor a URL; "+
					"pass a version like 1.0.1 or a manifest URL like https://example.com/releases.json", value)
		}
	}
	return client, nil
}

func looksLikeURL(value string) bool {
	lower := strings.ToLower(value)
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

// validateSourceURL rejects sources that would fetch an executable distribution over cleartext.
// Plain http is allowed only for a loopback host, which is what a local mirror or a test server
// looks like; anything else has to be https.
func validateSourceURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("--product-version %q is not a valid URL: %w", value, err)
	}
	if parsed.Host == "" {
		return fmt.Errorf("--product-version %q is missing a host", value)
	}
	if parsed.Scheme == "https" {
		return nil
	}
	if isLoopback(parsed.Hostname()) {
		return nil
	}
	return fmt.Errorf(
		"--product-version %q uses plain http; releases are executable, so http is allowed only "+
			"for localhost. Use https for a remote source", value)
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
