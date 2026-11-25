// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package release

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"

	"github.com/thunder-id/thunderid/tools/cli/internal/plane"
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
	// Plane is the distribution to install. Auto takes the quickstart when the release has one.
	Plane plane.Plane
	// GitHubRelease reports that ManifestURL is the GitHub release API for one release, which a
	// GitHub release page given as the source resolves to, rather than a releases manifest.
	GitHubRelease bool
	// Prereleases makes the newest release the latest even when it is a prerelease. A CLI
	// published on the next channel sets it, so `npx thunderid@next` installs milestones.
	Prereleases bool
}

// gitHubAPIBase is where GitHub release pages are resolved.
const gitHubAPIBase = "https://api.github.com"

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

// SelectedPlane returns the plane --plane asked for, or plane.Auto when the release decides.
func SelectedPlane() plane.Plane { return Default.Plane }

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
//
// A GitHub release page (github.com/<owner>/<repo>/releases/tag/<tag>, or .../releases/latest) is
// also a source: the release is read from the GitHub API, so a build published on any repository,
// such as a fork, can be installed before it reaches the public manifest. A tag page pins its tag.
func Configure(values []string) (*Client, error) {
	client := &Client{ManifestURL: product.ReleasesURL, FallbackURL: product.GitHubAPI}
	var sawURL, sawVersion bool
	var releaseTag string

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
			apiURL, tag, isRelease, err := gitHubReleaseAPI(value)
			if err != nil {
				return nil, err
			}
			if isRelease {
				client.ManifestURL = apiURL
				client.GitHubRelease = true
				releaseTag = tag
			}

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
	if releaseTag != "" {
		version := strings.TrimPrefix(releaseTag, "v")
		if sawVersion && client.PinnedVersion != version {
			return nil, fmt.Errorf("--product-version was given the release page for %s and the version %s; "+
				"pass one of them", releaseTag, client.PinnedVersion)
		}
		client.PinnedVersion = version
	}
	return client, nil
}

// gitHubReleaseAPI resolves a GitHub release page to the API URL that describes it. isRelease is
// false for any other URL, which is read as a manifest. For a tag page, tag is the release tag; it
// has to be a "v" and a version, which is how releases are tagged and matched.
func gitHubReleaseAPI(raw string) (apiURL, tag string, isRelease bool, err error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", "", false, nil
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "github.com" && host != "www.github.com" {
		return "", "", false, nil
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 3 || parts[2] != "releases" {
		return "", "", false, nil
	}
	repo := gitHubAPIBase + "/repos/" + url.PathEscape(parts[0]) + "/" + url.PathEscape(parts[1]) + "/releases"
	switch {
	case len(parts) == 4 && parts[3] == "latest":
		return repo + "/latest", "", true, nil
	// A release asset (.../releases/download/<tag>/<file>, or .../releases/latest/download/<file>)
	// is a file someone published, such as a manifest kept on a release, so it is read as one.
	case len(parts) == 6 && parts[3] == "download" && parts[4] != "" && parts[5] != "",
		len(parts) == 6 && parts[3] == "latest" && parts[4] == "download" && parts[5] != "":
		return "", "", false, nil
	case len(parts) == 5 && parts[3] == "tag":
		tag, err := url.PathUnescape(parts[4])
		if err != nil || !strings.HasPrefix(tag, "v") || !semverPattern.MatchString(tag) {
			return "", "", false, fmt.Errorf("--product-version %q is a release page whose tag is not a "+
				"version like v2.0.0-m1", raw)
		}
		return repo + "/tags/" + url.PathEscape(tag), tag, true, nil
	default:
		return "", "", false, fmt.Errorf("--product-version %q is a GitHub page but not a release; pass a "+
			"release page like https://github.com/<owner>/<repo>/releases/tag/v2.0.0", raw)
	}
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
