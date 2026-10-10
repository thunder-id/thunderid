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
		// PDP endpoints are admin-configured and may be internal hosts, so no
		// SSRF dial guard. The engine stamps its own per-request context
		// deadline, so the client itself stays unbounded, matching the
		// previous NewHTTPClientWithTimeout(0) behavior.
		httpservice.NewHTTPClient(httpservice.HTTPClientConfig{DisableTimeout: true}),
	)
	return newAuthorizationService(
		rbacEngine,
		resourceService,
		entityService,
		authZENPDPEngine,
	)
}
