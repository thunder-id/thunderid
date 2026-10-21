// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package idp

import (
	"context"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// BasicIDPDTO represents a basic data transfer object for an identity provider.
type BasicIDPDTO struct {
	ID           string
	Name         string
	Description  string
	Type         providers.IDPType
	OUID         string
	IsReadOnly   bool
	IDJagEnabled *bool
}

// ResolvedAuthorizationMapping is a connection's sync modes and the authorization targets its mapping
// grants for a federated identity's claims.
type ResolvedAuthorizationMapping struct {
	AttributesSyncMode     providers.SyncMode
	GroupsAndRolesSyncMode providers.SyncMode
	// Targets holds the rule and direct targets, or only the rule targets when Complete is false.
	Targets []providers.AuthorizationTarget
	// Ambiguous holds the groups and roles a direct mapping name matched more than once. They are
	// neither granted nor removed.
	Ambiguous []providers.AuthorizationTarget
	// Configured reports whether the connection has any rule or direct mapping.
	Configured bool
	// MapsGroups and MapsRoles report whether the mapping can resolve groups and roles.
	MapsGroups bool
	MapsRoles  bool
	// Complete is false when the direct lookups failed.
	Complete bool
}

// DirectTargetResolver resolves a connection's direct authorization mapping for claims.
type DirectTargetResolver func(ctx context.Context, idp *providers.IDPDTO,
	claims map[string]interface{}) ([]providers.AuthorizationTarget, []providers.AuthorizationTarget,
	*tidcommon.ServiceError)
