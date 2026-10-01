// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authz

import (
	"github.com/thunder-id/thunderid/internal/authz/engine"
	"github.com/thunder-id/thunderid/internal/connection/authzenpdp"
	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/role"
	httpservice "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize creates and initializes the authorization service.
func Initialize(
	roleService role.RoleServiceInterface,
	resourceService resource.ResourceServiceInterface,
	entityService entity.EntityServiceInterface,
	authZENPDPService authzenpdp.AuthZENPDPServiceInterface,
) providers.AuthorizationProvider {
	rbacEngine, authZENPDPEngine := engine.Initialize(
		roleService,
		authZENPDPService,
		httpservice.NewHTTPClientWithTimeout(0),
	)
	return newAuthorizationService(
		rbacEngine,
		resourceService,
		entityService,
		authZENPDPEngine,
	)
}
