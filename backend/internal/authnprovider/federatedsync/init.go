// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package federatedsync

import (
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/role"
)

// Initialize creates the service that syncs a federated sign-in to the account holding its link.
// idpProvider is required. Without entityProvider no account is read or written, and the mapping goes to
// the session only. groupService and roleAssignmentService are then unused, and may be nil too. Without
// entityTypeService no attribute is synced.
func Initialize(idpProvider providers.IDPProvider, entityProvider entityprovider.EntityProviderInterface,
	entityTypeService entitytype.EntityTypeServiceInterface, groupService group.GroupServiceInterface,
	roleAssignmentService role.RoleAssignmentServiceInterface) FederatedSyncServiceInterface {
	return newFederatedSyncService(idpProvider, entityProvider, entityTypeService, groupService,
		roleAssignmentService)
}
