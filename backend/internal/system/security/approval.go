// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package security

import (
	"context"
	"slices"
	"strings"
	"sync"
)

// ApprovedAction is a write that the caller who started a flow approved for a later step they do not send: the
// action, the organization unit it acts in, and, when set, the only resources it may act on. An action that names
// its resources and no unit acts on them in whatever unit they are in.
type ApprovedAction struct {
	Action      Action   `json:"action"`
	OUID        string   `json:"ouId"`
	ResourceIDs []string `json:"resourceIds,omitempty"`
}

// CreatedEntity is an entity that a caller limited to its approved writes created earlier in the same flow.
type CreatedEntity struct {
	ResourceType ResourceType `json:"resourceType"`
	ID           string       `json:"id"`
}

// createdEntities collects the entities a caller limited to its approved writes creates during a request.
type createdEntities struct {
	mu       sync.Mutex
	entities []CreatedEntity
}

// WithApprovedActions rebuilds a caller recorded earlier in the same flow, for a step that caller started but
// does not send, limited to the writes they approved and to updating the entities created earlier in the flow
// under those writes. Reads stay with the permissions given. The result is never a runtime context.
func WithApprovedActions(ctx context.Context, subject, ouID string, permissions []string,
	approved []ApprovedAction, created []CreatedEntity) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = context.WithValue(ctx, runtimeContextKey, false)
	sc := newSecurityContext(subject, ouID, "", permissions, nil)
	sc.recorded = true
	sc.approved = slices.Clone(approved)
	if sc.approved == nil {
		sc.approved = []ApprovedAction{}
	}
	sc.created = &createdEntities{entities: slices.Clone(created)}
	return withSecurityContext(ctx, sc)
}

// RecordCreated records an entity a caller limited to its approved writes has just created, so later steps of
// the flow may update it. The create path calls it once the entity is stored. Any other caller records nothing.
func RecordCreated(ctx context.Context, resourceType ResourceType, id string) {
	authCtx := getSecurityContext(ctx)
	if authCtx == nil || authCtx.created == nil || id == "" {
		return
	}
	authCtx.created.mu.Lock()
	defer authCtx.created.mu.Unlock()
	authCtx.created.entities = append(authCtx.created.entities, CreatedEntity{ResourceType: resourceType, ID: id})
}

// CreatedEntities returns the entities a caller limited to its approved writes created in the flow so far.
func CreatedEntities(ctx context.Context) []CreatedEntity {
	authCtx := getSecurityContext(ctx)
	if authCtx == nil || authCtx.created == nil {
		return nil
	}
	authCtx.created.mu.Lock()
	defer authCtx.created.mu.Unlock()
	return slices.Clone(authCtx.created.entities)
}

// IsUpdateOfCreated reports whether the action updates an entity the caller created earlier in the flow.
func IsUpdateOfCreated(ctx context.Context, action Action, resourceType ResourceType, resourceID string) bool {
	if resourceID == "" || !strings.HasSuffix(string(action), ":update") {
		return false
	}
	return slices.Contains(CreatedEntities(ctx), CreatedEntity{ResourceType: resourceType, ID: resourceID})
}

// IsLimitedToApprovedActions reports whether the caller may write only what it approved.
func IsLimitedToApprovedActions(ctx context.Context) bool {
	authCtx := getSecurityContext(ctx)
	return authCtx != nil && authCtx.approved != nil
}

// IsApproved reports whether the caller approved the action in the organization unit, on the resource when one is
// named.
func IsApproved(ctx context.Context, action Action, ouID, resourceID string) bool {
	authCtx := getSecurityContext(ctx)
	if authCtx == nil {
		return false
	}
	return slices.ContainsFunc(authCtx.approved, func(a ApprovedAction) bool {
		if a.Action != action {
			return false
		}
		if len(a.ResourceIDs) == 0 {
			return a.OUID == ouID
		}
		return (a.OUID == "" || a.OUID == ouID) && slices.Contains(a.ResourceIDs, resourceID)
	})
}

// IsApprovedOn reports whether the caller approved the action on the resource, which it must have named. It ignores
// the unit the approval names, so a caller must also check the action in the unit with IsActionAllowed.
func IsApprovedOn(ctx context.Context, action Action, resourceID string) bool {
	authCtx := getSecurityContext(ctx)
	if authCtx == nil || resourceID == "" {
		return false
	}
	return slices.ContainsFunc(authCtx.approved, func(a ApprovedAction) bool {
		return a.Action == action && slices.Contains(a.ResourceIDs, resourceID)
	})
}

// IsReadAction reports whether the action only reads.
func IsReadAction(action Action) bool {
	verb := string(action)
	if i := strings.LastIndex(verb, ":"); i >= 0 {
		verb = verb[i+1:]
	}
	return verb == "read" || verb == "list" || verb == "list-children"
}
