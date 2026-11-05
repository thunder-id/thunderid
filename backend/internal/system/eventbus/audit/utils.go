// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"strings"

	"github.com/thunder-id/thunderid/internal/system/utils"
)

// ReasonFromCode derives a stable outcome reason from a service error code, as usr_1003 from USR-1003.
// A code that cannot make a valid reason gives server_error.
func ReasonFromCode(code string) string {
	reason := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, strings.ToLower(code))
	if !reasonPattern.MatchString(reason) {
		return reasonServerError
	}
	return reason
}

// capID cuts an ID to its first maxIDLength characters.
func capID(id string) string {
	return utils.TruncateRunes(id, maxIDLength)
}
