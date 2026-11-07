// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"context"
	"strings"

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
