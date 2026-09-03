// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package release_test

import (
	"strings"
	"testing"

	"github.com/thunder-id/thunderid/tools/cli/internal/product"
	"github.com/thunder-id/thunderid/tools/cli/internal/services/release"
)

func TestConfigure_ClassifiesValuesByShape(t *testing.T) {
	tests := []struct {
		name        string
		values      []string
		wantVersion string
		wantURL     string
	}{
		{
			name:        "no values keeps the public manifest",
			values:      nil,
			wantVersion: "",
			wantURL:     product.ReleasesURL,
		},
		{
			name:        "a version pins the release",
			values:      []string{"1.0.1"},
			wantVersion: "1.0.1",
			wantURL:     product.ReleasesURL,
		},
		{
			name:        "a leading v is accepted and dropped",
			values:      []string{"v1.0.1"},
			wantVersion: "1.0.1",
			wantURL:     product.ReleasesURL,
		},
		{
			name:        "a prerelease version is a version",
			values:      []string{"1.1.0-m1"},
			wantVersion: "1.1.0-m1",
			wantURL:     product.ReleasesURL,
		},
		{
			name:        "a URL selects the manifest",
			values:      []string{"https://example.com/releases.json"},
			wantVersion: "",
			wantURL:     "https://example.com/releases.json",
		},
		{
			name:        "both may be combined in either order",
			values:      []string{"https://example.com/releases.json", "1.0.1"},
			wantVersion: "1.0.1",
			wantURL:     "https://example.com/releases.json",
		},
		{
			name:        "order does not matter",
			values:      []string{"1.0.1", "https://example.com/releases.json"},
			wantVersion: "1.0.1",
			wantURL:     "https://example.com/releases.json",
		},
		{
			name:        "http is allowed for loopback, where a local mirror lives",
			values:      []string{"http://127.0.0.1:8080/releases.json"},
			wantVersion: "",
			wantURL:     "http://127.0.0.1:8080/releases.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := release.Configure(tt.values)
			if err != nil {
				t.Fatalf("Configure(%q): %v", tt.values, err)
			}
			if client.PinnedVersion != tt.wantVersion {
				t.Errorf("PinnedVersion = %q, want %q", client.PinnedVersion, tt.wantVersion)
			}
			if client.ManifestURL != tt.wantURL {
				t.Errorf("ManifestURL = %q, want %q", client.ManifestURL, tt.wantURL)
			}
		})
	}
}

// A custom manifest is authoritative. Falling back to the public GitHub API when an operator
// pointed at their own mirror would fetch something they did not ask for.
func TestConfigure_CustomSourceDisablesTheFallback(t *testing.T) {
	client, err := release.Configure([]string{"https://example.com/releases.json"})
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if client.FallbackURL != "" {
		t.Errorf("FallbackURL = %q, want empty for a custom source", client.FallbackURL)
	}
	if !client.IsCustom() {
		t.Error("IsCustom() = false, want true")
	}
}

func TestConfigure_DefaultSourceKeepsTheFallback(t *testing.T) {
	client, err := release.Configure([]string{"1.0.1"})
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if client.FallbackURL != product.GitHubAPI {
		t.Errorf("FallbackURL = %q, want the GitHub API", client.FallbackURL)
	}
	if client.IsCustom() {
		t.Error("IsCustom() = true, want false when only a version was pinned")
	}
}

func TestConfigure_Rejects(t *testing.T) {
	tests := []struct {
		name     string
		values   []string
		wantHint string
	}{
		{
			name:     "a value that is neither a version nor a URL",
			values:   []string{"latest"},
			wantHint: "neither a version nor a URL",
		},
		{
			name:     "an incomplete version",
			values:   []string{"1.0"},
			wantHint: "neither a version nor a URL",
		},
		{
			// Releases are executable, so a cleartext remote source is refused outright rather
			// than warned about.
			name:     "plain http for a remote host",
			values:   []string{"http://releases.example.com/releases.json"},
			wantHint: "plain http",
		},
		{
			name:     "two versions",
			values:   []string{"1.0.1", "1.0.2"},
			wantHint: "two versions",
		},
		{
			name:     "two sources",
			values:   []string{"https://a.example.com/r.json", "https://b.example.com/r.json"},
			wantHint: "two release sources",
		},
		{
			name:     "a URL with no host",
			values:   []string{"https:///releases.json"},
			wantHint: "missing a host",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := release.Configure(tt.values)
			if err == nil {
				t.Fatalf("Configure(%q) succeeded, want an error", tt.values)
			}
			if !strings.Contains(err.Error(), tt.wantHint) {
				t.Errorf("error = %q, want it to mention %q", err, tt.wantHint)
			}
		})
	}
}
