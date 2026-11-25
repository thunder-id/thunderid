// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"context"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/security"
)

// resolveActor derives the acting principal from the verified security context of ctx. The second
// result is the principal the actor acted for, set when the token carries an act claim (RFC 8693).
func resolveActor(ctx context.Context) (Actor, Ref) {
	subject := security.GetSubject(ctx)
	if subject == "" {
		return Actor{Type: ActorTypeAnonymous}, Ref{}
	}
	if security.IsManagementAPIKey(ctx) {
		return Actor{Type: ActorTypeSystem, ID: subject}, Ref{}
	}
	if act, ok := security.GetAttribute(ctx, serverconst.ActClaimName).(map[string]interface{}); ok {
		if actorID, _ := act[serverconst.SubClaimName].(string); actorID != "" {
			return Actor{Type: ActorTypeAgent, ID: actorID}, Ref{Type: string(subjectType(ctx)), ID: subject}
		}
	}
	return Actor{Type: subjectType(ctx), ID: subject}, Ref{}
}

// subjectType maps the sub_type claim of the token to an actor type. Only client credentials tokens
// carry the claim today, so a token without it is taken for a user; a later change may have every
// token state its subject type, including a user value.
func subjectType(ctx context.Context) ActorType {
	subType, _ := security.GetAttribute(ctx, serverconst.SubTypeClaimName).(string)
	switch ActorType(subType) {
	case ActorTypeApplication, ActorTypeAgent:
		return ActorType(subType)
	default:
		return ActorTypeUser
	}
}
