// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package utils

import "regexp"

// maxHandleLength is the maximum length of a handle.
const maxHandleLength = 100

// handleRegex matches lowercase alphanumeric handles that may contain hyphens and underscores,
// but must start and end with an alphanumeric character.
var handleRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*[a-z0-9]$|^[a-z0-9]$`)

// IsValidHandle reports whether the handle has a valid format. A valid handle contains only
// lowercase letters, numbers, hyphens and underscores, and starts and ends with a letter or a number.
// It must not exceed 100 characters.
func IsValidHandle(handle string) bool {
	return len(handle) <= maxHandleLength && handleRegex.MatchString(handle)
}
