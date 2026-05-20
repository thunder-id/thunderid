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

package group

import (
	"context"

	oupkg "github.com/thunder-id/thunderid/internal/ou"
)

// ouGroupResolverAdapter implements oupkg.OUGroupResolver using the group store.
// This adapter allows the OU package to query group data without directly
// accessing the GROUP table, breaking the cross-DB access boundary.
type ouGroupResolverAdapter struct {
	store groupStoreInterface
}

// newOUGroupResolver creates a new OUGroupResolver backed by the given group store.
func newOUGroupResolver(store groupStoreInterface) oupkg.OUGroupResolver {
	return &ouGroupResolverAdapter{store: store}
}

// GetGroupCountByOUID returns the count of groups belonging to the given organization unit.
func (a *ouGroupResolverAdapter) GetGroupCountByOUID(ctx context.Context, ouID string) (int, error) {
	return a.store.GetGroupsByOrganizationUnitCount(ctx, ouID)
}

// GetGroupListByOUID returns a paginated list of groups belonging to the given organization unit.
func (a *ouGroupResolverAdapter) GetGroupListByOUID(
	ctx context.Context, ouID string, limit, offset int,
) ([]oupkg.Group, error) {
	groups, err := a.store.GetGroupsByOrganizationUnit(ctx, ouID, limit, offset)
	if err != nil {
		return nil, err
	}

	result := make([]oupkg.Group, len(groups))
	for i, g := range groups {
		result[i] = oupkg.Group{ID: g.ID, Name: g.Name}
	}

	return result, nil
}
