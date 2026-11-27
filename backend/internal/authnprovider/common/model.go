// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import "github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

// SessionValues is what a federated sign-in grants the session without storing it on the account.
type SessionValues struct {
	// Attributes holds the attribute values that win over the stored ones for the session.
	Attributes map[string]interface{}
	// GroupIDs holds the session groups, and GroupIDsWithAncestors the same groups with their
	// ancestors.
	GroupIDs              []string
	GroupIDsWithAncestors []string
	// RoleIDs holds the session roles.
	RoleIDs []string
	// Permissions holds the permissions the authorization mapping grants directly.
	Permissions []providers.AuthorizationTarget
}
