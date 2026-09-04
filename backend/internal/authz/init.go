// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authz

import (
	"github.com/thunder-id/thunderid/internal/authz/engine"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/role"
	httpservice "github.com/thunder-id/thunderid/internal/system/http"
	userpkg "github.com/thunder-id/thunderid/internal/user"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize creates and initializes the authorization service.
func Initialize(
	roleService role.RoleServiceInterface,
	resourceService resource.ResourceServiceInterface,
	userService userpkg.UserServiceInterface,
) providers.AuthorizationProvider {
	return newAuthorizationService(
		engine.NewRBACEngine(roleService), resourceService, userService, httpservice.NewHTTPClient())
}
