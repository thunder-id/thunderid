// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package product contains product-level constants shared across the CLI.
package product

// Product identity constants.
const (
	Name = "ThunderID"
	Slug = "thunderid"
)

// Distribution URLs.
const (
	ReleasesURL      = "https://thunderid.dev/data/releases.json"
	GitHubAPI        = "https://api.github.com/repos/thunder-id/thunderid/releases/latest"
	GitHubArchiveURL = "https://codeload.github.com/thunder-id/thunderid/zip/refs/heads/main"
)

// DocsVersionURL is the root of the current versioned docs tree, indexed at
// https://thunderid.dev/llms.txt. Bump this when the docs site cuts a new version.
const DocsVersionURL = "https://thunderid.dev/docs/v1.1.x"

// DocsBaseURL is the canonical per-platform "connect your application" guide directory.
// Appending "/<slug>.md" fetches the raw markdown guide; appending "/<slug>" (no
// extension) is the human-facing HTML page, for "open in browser" links.
const DocsBaseURL = DocsVersionURL + "/getting-started/connect-your-application"

// Brand colors.
const (
	ColorDeepNavy     = "#05213F" // primary brand — logo text and dark backgrounds
	ColorElectricBlue = "#3688FF" // accent — icon highlight, links, call-to-action
	ColorWhite        = "#FFFFFF" // light backgrounds and inverted text
)
