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

package role

import (
	"context"

	oupkg "github.com/thunder-id/thunderid/internal/ou"
)

// ouRoleResolverAdapter implements oupkg.OURoleResolver using the role store.
// This adapter allows the OU package to query role data without directly
// accessing the ROLE table, breaking the cross-DB access boundary.
type ouRoleResolverAdapter struct {
	store roleStoreInterface
}

// newOURoleResolver creates a new OURoleResolver backed by the given role store.
func newOURoleResolver(store roleStoreInterface) oupkg.OURoleResolver {
	return &ouRoleResolverAdapter{store: store}
}

// GetRoleCountByOUID returns the count of roles belonging to the given organization unit.
func (a *ouRoleResolverAdapter) GetRoleCountByOUID(ctx context.Context, ouID string) (int, error) {
	return a.store.GetRoleListCountByOUID(ctx, ouID)
}

// GetRoleListByOUID returns a paginated list of roles belonging to the given organization unit.
func (a *ouRoleResolverAdapter) GetRoleListByOUID(
	ctx context.Context, ouID string, limit, offset int,
) ([]oupkg.Role, error) {
	roles, err := a.store.GetRoleListByOUID(ctx, ouID, limit, offset)
	if err != nil {
		return nil, err
	}

	result := make([]oupkg.Role, len(roles))
	for i, r := range roles {
		result[i] = oupkg.Role{ID: r.ID, Name: r.Name, Description: r.Description, IsReadOnly: r.IsReadOnly}
	}

	return result, nil
}
