// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resolve

import "errors"

// Internal sentinels describing why a theme could not be resolved into design tokens. They are a
// server-side data problem, not a client error, so the caller (resolveDesignTokens) logs them with the
// theme ID and returns a generic InternalServerError; this package exposes only client errors (via
// design/common).
var (
	errEmptyTheme              = errors.New("theme has no content to resolve design tokens from")
	errNoApplicableColorScheme = errors.New("theme defines no color scheme that applies")
	errMalformedTheme          = errors.New("theme JSON is malformed or has trailing content")
)
