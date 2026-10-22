// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package attributecache

import (
	"encoding/json"

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
}

// SessionGrantsKey is the reserved attribute under which a sign-in caches the authorization it granted
// for the session without storing it, so a refresh can re-check scopes against it. Token and userinfo
// claims are built only from allow-listed attribute names, so it is never emitted as a claim.
const SessionGrantsKey = "__session_grants"

// SessionGrants is the authorization a federated sign-in granted for the session without storing it on
// the account: session-only groups (with their ancestors) and roles, and mapped permissions.
type SessionGrants struct {
	GroupIDs    []string                        `json:"groupIds,omitempty"`
	RoleIDs     []string                        `json:"roleIds,omitempty"`
	Permissions []providers.AuthorizationTarget `json:"permissions,omitempty"`
}

// GetSessionGrants returns the session grants cached in attributes, or nil when there are none.
func GetSessionGrants(attributes map[string]interface{}) (*SessionGrants, error) {
	value, ok := attributes[SessionGrantsKey]
	if !ok || value == nil {
		return nil, nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var grants SessionGrants
	if err := json.Unmarshal(encoded, &grants); err != nil {
		return nil, err
	}
	return &grants, nil
}
