// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// maxHandleLength is the maximum length of a handle.
const maxHandleLength = 100

// handleSeparator joins the words of a generated handle.
const handleSeparator = '-'

// handleRegex matches lowercase alphanumeric handles that may contain hyphens and underscores,
// but must start and end with an alphanumeric character.
var handleRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*[a-z0-9]$|^[a-z0-9]$`)

// IsValidHandle reports whether the handle has a valid format. A valid handle contains only
// lowercase letters, numbers, hyphens and underscores, and starts and ends with a letter or a number.
// It must not exceed 100 characters.
func IsValidHandle(handle string) bool {
	return len(handle) <= maxHandleLength && handleRegex.MatchString(handle)
}

// GenerateHandle derives a valid handle from a display name. Accents are dropped, letters are
// lowercased, and every run of other characters becomes a single hyphen. It returns an empty string
// when the name has no letter or number a handle can carry.
func GenerateHandle(name string) string {
	var b strings.Builder
	pendingSeparator := false
	for _, r := range norm.NFKD.String(strings.ToLower(name)) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			if pendingSeparator && b.Len() > 0 {
				b.WriteRune(handleSeparator)
			}
			pendingSeparator = false
			b.WriteRune(r)
		case unicode.Is(unicode.Mn, r):
			// Accent marks split off by the normalization are dropped.
		default:
			pendingSeparator = true
		}
	}

	handle := b.String()
	if len(handle) > maxHandleLength {
		handle = strings.TrimRight(handle[:maxHandleLength], string(handleSeparator))
	}
	return handle
}
