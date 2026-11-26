// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package manager

import (
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/role"
)

// FederatedSyncDependencies are what the manager syncs a federated sign-in to the account holding its link
// with. Without an entity provider no sign-in is synced.
type FederatedSyncDependencies struct {
	EntityProvider        entityprovider.EntityProviderInterface
	EntityTypeService     entitytype.EntityTypeServiceInterface
	GroupService          group.GroupServiceInterface
	RoleAssignmentService role.RoleAssignmentServiceInterface
	IDPProvider           providers.IDPProvider
}
