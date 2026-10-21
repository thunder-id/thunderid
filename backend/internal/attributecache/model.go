// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package attributecache

import (
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// AttributeCache represents a cached attribute entry.
type AttributeCache struct {
	// ID is the unique identifier for the cache entry.
	ID string `json:"id"`

	// Attributes contains the cached attributes.
	Attributes map[string]interface{} `json:"attributes"`

	// TTLSeconds is the time-to-live in seconds for this cache entry.
	TTLSeconds int64 `json:"ttlSeconds"`

<<<<<<< HEAD
	// SubjectID is the resource ID of the entity the cached attributes belong to, and
	// SubjectCategory its entity category. They are recorded when the entry is created, where the
	// authentication has just resolved the entity, so a later grant that holds only this entry's ID
	// can report the subject without resolving it again. This matters for a token whose sub claim is
	// mapped to an attribute such as an email address: the resource ID is not recoverable from the
	// token, and this entry is the only server-side record of it. Observability-only: no
	// authorization decision reads either field, and both may be empty.
	SubjectID       string `json:"subjectId,omitempty"`
	SubjectCategory string `json:"subjectCategory,omitempty"`
=======
	// SessionGrants holds the authorization the sign-in granted for the session without storing it, so
	// a refresh can re-check scopes against it.
	SessionGrants *SessionGrants `json:"-"`
}

// sessionGrantsKey is the reserved attribute the cache service stores SessionGrants under. The service
// drops an attribute of that name on write and moves the grants out of Attributes on read, so it never
// reaches a caller's attributes.
const sessionGrantsKey = "__session_grants"

// SessionGrants is the authorization a federated sign-in granted for the session without storing it on
// the account: session-only groups (with their ancestors) and roles, and mapped permissions.
type SessionGrants struct {
	GroupIDs    []string                        `json:"groupIds,omitempty"`
	RoleIDs     []string                        `json:"roleIds,omitempty"`
	Permissions []providers.AuthorizationTarget `json:"permissions,omitempty"`
>>>>>>> 2b18ce8b0 (Sync federated users with per-connection sync modes)
}
