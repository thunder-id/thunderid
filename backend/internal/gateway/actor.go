// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/security"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

// Actor is the user a version was captured for or applied by. Only their id is recorded, read from
// the claim server.security.trusted_issuer.user_id_claim names, sub by default; their name is put to
// it when the record is read, from the trusted issuer's user API when
// server.security.trusted_issuer.user_lookup is configured, and is never stored. Only a control plane
// records actors, since it is where versions are captured and applied.
type Actor struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// shown is an actor as a response gives them out: with their name, or not at all. The id alone is an
// identifier of the issuer's that says nothing to a viewer, so a user whose name is not known, because
// no lookup is configured, the issuer does not know them or the lookup failed, is left out. The id
// stays recorded, so a lookup configured later names the history already captured.
func shown(actor *Actor) *Actor {
	if actor == nil || actor.Name == "" {
		return nil
	}
	return actor
}

type actorContextKey struct{}

// withActor sets who work is done for when the caller is not them, such as a capture the system runs
// after a user's change.
func withActor(ctx context.Context, actor *Actor) context.Context {
	if actor == nil {
		return ctx
	}
	return context.WithValue(ctx, actorContextKey{}, actor)
}

// actorOf is who ctx acts for: the actor set with withActor, otherwise the authenticated caller, or
// nil when there is neither.
func actorOf(ctx context.Context) *Actor {
	if actor, ok := ctx.Value(actorContextKey{}).(*Actor); ok {
		return actor
	}
	idClaim := userIDClaim()
	// The subject is the caller's id however they were authenticated, a management API key included,
	// so it is read as the security context holds it rather than from the token.
	id := security.GetSubject(ctx)
	if idClaim != "sub" {
		id = claimOf(ctx, idClaim)
	}
	// An id longer than its column cannot be recorded, so such a caller is recorded as no one rather
	// than failing the capture or the apply.
	if id == "" || len(id) > maxActorIDLength {
		return nil
	}
	return &Actor{ID: id}
}

// maxActorIDLength is the width of the CREATED_BY and APPLIED_BY columns.
const maxActorIDLength = 255

// userIDClaim names the claim a user's id is read from.
func userIDClaim() string {
	if !config.IsServerRuntimeInitialized() {
		return engineconfig.TrustedIssuerConfig{}.UserIDClaimOrDefault()
	}
	return config.GetServerRuntime().Config.Server.SecurityConfig.TrustedIssuer.UserIDClaimOrDefault()
}

// claimOf reads a string claim of the caller's token, trimmed, or "" when it carries none.
func claimOf(ctx context.Context, claim string) string {
	value, _ := security.GetAttribute(ctx, claim).(string)
	return strings.TrimSpace(value)
}

// actorColumn is how an actor is stored: its id, NULL when absent.
func actorColumn(actor *Actor) interface{} {
	if actor == nil {
		return nil
	}
	return nullable(actor.ID)
}

// actorFrom reads an actor from its stored id, or nil when none was stored.
func actorFrom(id interface{}) *Actor {
	actorID := asString(id)
	if actorID == "" {
		return nil
	}
	return &Actor{ID: actorID}
}
