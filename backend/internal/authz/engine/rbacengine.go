/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Package engine provides authorization engine implementations.
// It includes various authorization engines such as RBAC (Role-Based Access Control)
// that delegate authorization decisions to the appropriate services.
package engine

import (
	"context"
	"fmt"

	"github.com/thunder-id/thunderid/internal/role"
)

// rbacEngine implements Role-Based Access Control (RBAC) authorization.
// It delegates authorization decisions to the role service.
type rbacEngine struct {
	roleService role.RoleServiceInterface
}

// NewRBACEngine creates a new RBAC authorization engine.
func NewRBACEngine(roleService role.RoleServiceInterface) AuthorizationEngine {
	return &rbacEngine{
		roleService: roleService,
	}
}

// GetAuthorizedPermissions returns the subset of requested permissions
// that the entity is authorized for based on their role assignments.
func (e *rbacEngine) GetAuthorizedPermissions(
	ctx context.Context,
	entityID string,
	groupIDs []string,
	requestedPermissions []string,
) ([]string, error) {
	// Delegate to role service
	authorizedPerms, svcErr := e.roleService.GetAuthorizedPermissions(
		ctx, entityID, groupIDs, requestedPermissions)
	if svcErr != nil {
		return nil, fmt.Errorf("role service error: %s", svcErr.Error)
	}

	return authorizedPerms, nil
}
