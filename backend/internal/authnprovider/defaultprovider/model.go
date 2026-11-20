// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package defaultprovider

import (
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/role"
)

// FederatedSyncDependencies are the services the provider syncs a federated sign-in to the account
// holding its link with.
type FederatedSyncDependencies struct {
	EntityTypeService     entitytype.EntityTypeServiceInterface
	IDPService            providers.IDPProvider
	GroupService          group.GroupServiceInterface
	RoleAssignmentService role.RoleAssignmentServiceInterface
}

// attributedFailure identifies the subject of a failed proof.
type attributedFailure struct {
	// entityID is the entity the proof was compared against.
	entityID string

	// recipient holds the identifying attribute of the subject when entityID is empty.
	recipient map[string]interface{}
}
