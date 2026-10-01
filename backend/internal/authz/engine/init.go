// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"github.com/thunder-id/thunderid/internal/role"
	httpservice "github.com/thunder-id/thunderid/internal/system/http"
)

// Initialize creates the authorization engine implementations.
func Initialize(
	roleService role.RoleServiceInterface,
	connectionService authZENPDPConnectionService,
	client httpservice.HTTPClientInterface,
) (AuthorizationEngine, AuthorizationEngine) {
	return newRBACEngine(roleService), newAuthZENPDPEngine(connectionService, client)
}
