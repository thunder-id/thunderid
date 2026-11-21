// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// reasonFromCode derives a stable outcome reason from a service error code, as usr_1003 from USR-1003.
// A code that cannot make a valid reason gives server_error.
func reasonFromCode(code string) string {
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

// PrincipalRef names the principal a role assignment or a group membership names by its type, user, app,
// agent or group, and its ID. A type with no target type is kept as it is, so the record stays truthful,
// and logged, since a principal type added without one is a gap in what audit maps.
func PrincipalRef(ctx context.Context, principalType, id string) Ref {
	targetType, ok := principalTargetTypes[principalType]
	if !ok {
		log.GetLogger().With(log.String(log.LoggerKeyComponentName, "Audit")).Error(ctx,
			"Principal type has no audit target type", log.String("principalType", principalType))
		targetType = principalType
	}
	return Ref{Type: targetType, ID: id}
}

// capID cuts an ID to its first maxIDLength characters.
func capID(id string) string {
	return utils.TruncateRunes(id, maxIDLength)
}

// capDetail cuts a detail value to its first maxDetailLength characters.
func capDetail(value string) string {
	return utils.TruncateRunes(value, maxDetailLength)
}

// listEscaper escapes the characters a list detail uses as separators, and the escape character itself.
var listEscaper = strings.NewReplacer(`\`, `\\`, ",", `\,`, ":", `\:`)

// ListEntry formats one entry of a list detail from its parts, such as a resource type and its ID, joined
// with ":". Each part is cut to the length of an ID and has its separators escaped with a backslash, so a
// value taken from a request can neither add an entry nor split one.
func ListEntry(parts ...string) string {
	escaped := make([]string, len(parts))
	for i, part := range parts {
		escaped[i] = listEscaper.Replace(capID(part))
	}
	return strings.Join(escaped, ":")
}

// JoinList joins the entries of a list detail with ",", cut after the last whole entry that fits a detail
// value, so no entry is split. An entry too long to fit on its own is cut, so a list that has an entry is
// never empty.
func JoinList(entries []string) string {
	var list strings.Builder
	length := 0
	for i, entry := range entries {
		size := utf8.RuneCountInString(entry)
		if i > 0 {
			size++
		}
		if length+size > maxDetailLength {
			if i == 0 {
				return capDetail(entry)
			}
			break
		}
		if i > 0 {
			list.WriteString(",")
		}
		list.WriteString(entry)
		length += size
	}
	return list.String()
}
