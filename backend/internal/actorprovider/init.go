// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package actorprovider resolves inbound actors (applications, agents, and similar entities)
// for runtime layers that need a unified view of inbound-client and backing records.
package actorprovider

import (
	"github.com/thunder-id/thunderid/internal/application"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize creates the default ActorProvider backed by inbound-client, entity-provider,
// authentication provider, and role services.
func Initialize(
	inboundClient inboundclient.InboundClientServiceInterface,
	entityProvider entityprovider.EntityProviderInterface,
	authnProvider providers.AuthnProviderManager,
	roleService role.RoleServiceInterface,
	appService application.ApplicationServiceInterface,
) providers.ActorProvider {
	return newActorProvider(inboundClient, entityProvider, authnProvider, roleService, appService)
}
