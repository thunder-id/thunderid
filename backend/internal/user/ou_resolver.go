/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package user

import (
	"context"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/entitytype"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// ouUserResolverAdapter implements oupkg.OUUserResolver using the entity service.
// This adapter allows the OU package to query user data without directly
// accessing the entity layer, maintaining proper package boundaries.
type ouUserResolverAdapter struct {
	entityService     entity.EntityServiceInterface
	entityTypeService entitytype.EntityTypeServiceInterface
}

// newOUUserResolver creates a new OUUserResolver backed by the given entity service.
func newOUUserResolver(
	entityService entity.EntityServiceInterface, entityTypeService entitytype.EntityTypeServiceInterface,
) oupkg.OUUserResolver {
	return &ouUserResolverAdapter{entityService: entityService, entityTypeService: entityTypeService}
}

// GetUserCountByOUID returns the count of users belonging to the given organization unit.
func (a *ouUserResolverAdapter) GetUserCountByOUID(ctx context.Context, ouID string) (int, error) {
	return a.entityService.GetEntityListCountByOUIDs(ctx, entity.EntityCategoryUser, []string{ouID}, nil)
}

// GetUserListByOUID returns a paginated list of users belonging to the given organization unit.
// When includeDisplay is true, display names are resolved from user attributes using the schema service.
func (a *ouUserResolverAdapter) GetUserListByOUID(
	ctx context.Context, ouID string, limit, offset int, includeDisplay bool,
) ([]oupkg.User, error) {
	entities, err := a.entityService.GetEntityListByOUIDs(
		ctx, entity.EntityCategoryUser, []string{ouID}, limit, offset, nil)
	if err != nil {
		return nil, err
	}
	users := entitiesToUsers(entities)

	var displayAttrPaths map[string]string
	if includeDisplay {
		displayAttrPaths = resolveOUUserDisplayPaths(ctx, users, a.entityTypeService)
	}

	result := make([]oupkg.User, len(users))
	for i, u := range users {
		result[i] = oupkg.User{ID: u.ID, Type: u.Type}
		if includeDisplay {
			result[i].Display = utils.ResolveDisplay(u.ID, u.Type, u.Attributes, displayAttrPaths)
		}
	}

	return result, nil
}

// resolveOUUserDisplayPaths collects user types and resolves their display attribute paths.
func resolveOUUserDisplayPaths(
	ctx context.Context, users []User, schemaService entitytype.EntityTypeServiceInterface,
) map[string]string {
	userTypes := make([]string, 0, len(users))
	for _, u := range users {
		userTypes = append(userTypes, u.Type)
	}

	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "OUUserResolver"))
	return ResolveDisplayAttributePaths(ctx, userTypes, schemaService, logger)
}
