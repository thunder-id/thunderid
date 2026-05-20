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

package ou

import (
	"context"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/filter"
)

// compositeOUStore implements a composite store that combines file-based (immutable) and database (mutable) stores.
// - Read operations query both stores and merge results
// - Write operations (Create/Update/Delete) only affect the database store
// - Declarative OUs (from YAML files) cannot be modified or deleted
type compositeOUStore struct {
	fileStore organizationUnitStoreInterface
	dbStore   organizationUnitStoreInterface
}

// newCompositeOUStore creates a new composite store with both file-based and database stores.
func newCompositeOUStore(fileStore, dbStore organizationUnitStoreInterface) *compositeOUStore {
	return &compositeOUStore{
		fileStore: fileStore,
		dbStore:   dbStore,
	}
}

// GetOrganizationUnitListCount retrieves the total count of organization units from both stores.
func (c *compositeOUStore) GetOrganizationUnitListCount(ctx context.Context, f *filter.FilterGroup) (int, error) {
	return declarativeresource.CompositeMergeCountHelper(
		func() (int, error) { return c.dbStore.GetOrganizationUnitListCount(ctx, f) },
		func() (int, error) { return c.fileStore.GetOrganizationUnitListCount(ctx, f) },
	)
}

// GetOrganizationUnitList retrieves organization units from both stores with pagination.
// Applies the 1000-record limit in composite mode to prevent memory exhaustion.
// Returns ErrResultLimitExceededInCompositeMode if the limit is exceeded.
func (c *compositeOUStore) GetOrganizationUnitList(
	ctx context.Context, limit, offset int, f *filter.FilterGroup,
) ([]OrganizationUnitBasic, error) {
	items, limitExceeded, err := declarativeresource.CompositeMergeListHelperWithLimit(
		func() (int, error) { return c.dbStore.GetOrganizationUnitListCount(ctx, f) },
		func() (int, error) { return c.fileStore.GetOrganizationUnitListCount(ctx, f) },
		func(count int) ([]OrganizationUnitBasic, error) {
			return c.dbStore.GetOrganizationUnitList(ctx, count, 0, f)
		},
		func(count int) ([]OrganizationUnitBasic, error) {
			return c.fileStore.GetOrganizationUnitList(ctx, count, 0, f)
		},
		mergeAndDeduplicateOUs,
		limit,
		offset,
		serverconst.MaxCompositeStoreRecords,
	)
	if err != nil {
		return nil, err
	}
	if limitExceeded {
		return nil, ErrResultLimitExceededInCompositeMode
	}
	return items, nil
}

// GetOrganizationUnitsByIDs retrieves organization units matching the given IDs from both stores.
func (c *compositeOUStore) GetOrganizationUnitsByIDs(
	ctx context.Context, ids []string,
) ([]OrganizationUnitBasic, error) {
	if len(ids) == 0 {
		return []OrganizationUnitBasic{}, nil
	}

	dbOUs, err := c.dbStore.GetOrganizationUnitsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	fileOUs, err := c.fileStore.GetOrganizationUnitsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	return mergeAndDeduplicateOUs(dbOUs, fileOUs), nil
}

// CreateOrganizationUnit creates a new organization unit in the database store only.
// Conflict checking and parent validation are handled at the service layer.
func (c *compositeOUStore) CreateOrganizationUnit(ctx context.Context, ou OrganizationUnit) error {
	return c.dbStore.CreateOrganizationUnit(ctx, ou)
}

// GetOrganizationUnit retrieves an organization unit by ID from either store.
// Checks database store first, then falls back to file store.
func (c *compositeOUStore) GetOrganizationUnit(ctx context.Context, id string) (OrganizationUnit, error) {
	return declarativeresource.CompositeGetHelper(
		func() (OrganizationUnit, error) { return c.dbStore.GetOrganizationUnit(ctx, id) },
		func() (OrganizationUnit, error) { return c.fileStore.GetOrganizationUnit(ctx, id) },
		ErrOrganizationUnitNotFound,
	)
}

// GetOrganizationUnitByHandle retrieves an organization unit by handle and parent from either store.
// Checks database store first, then falls back to file store.
func (c *compositeOUStore) GetOrganizationUnitByHandle(
	ctx context.Context, handle string, parent *string,
) (OrganizationUnit, error) {
	return declarativeresource.CompositeGetHelper(
		func() (OrganizationUnit, error) { return c.dbStore.GetOrganizationUnitByHandle(ctx, handle, parent) },
		func() (OrganizationUnit, error) { return c.fileStore.GetOrganizationUnitByHandle(ctx, handle, parent) },
		ErrOrganizationUnitNotFound,
	)
}

// GetOrganizationUnitByPath retrieves an organization unit by hierarchical path from either store.
func (c *compositeOUStore) GetOrganizationUnitByPath(ctx context.Context, handles []string) (OrganizationUnit, error) {
	if len(handles) == 0 {
		return OrganizationUnit{}, ErrOrganizationUnitNotFound
	}

	current, err := c.findRootByHandle(ctx, handles[0])
	if err != nil {
		return OrganizationUnit{}, err
	}

	for i := 1; i < len(handles); i++ {
		current, err = c.findChildByHandle(ctx, current.ID, handles[i])
		if err != nil {
			return OrganizationUnit{}, err
		}
	}

	return current, nil
}

func (c *compositeOUStore) findRootByHandle(ctx context.Context, handle string) (OrganizationUnit, error) {
	return c.GetOrganizationUnitByHandle(ctx, handle, nil)
}

func (c *compositeOUStore) findChildByHandle(
	ctx context.Context, parentID, handle string,
) (OrganizationUnit, error) {
	return c.GetOrganizationUnitByHandle(ctx, handle, &parentID)
}

// IsOrganizationUnitExists checks if an organization unit exists in either store.
func (c *compositeOUStore) IsOrganizationUnitExists(ctx context.Context, id string) (bool, error) {
	return declarativeresource.CompositeBooleanCheckHelper(
		func() (bool, error) { return c.fileStore.IsOrganizationUnitExists(ctx, id) },
		func() (bool, error) { return c.dbStore.IsOrganizationUnitExists(ctx, id) },
	)
}

// IsOrganizationUnitDeclarative checks if an organization unit is immutable (exists in file store).
func (c *compositeOUStore) IsOrganizationUnitDeclarative(ctx context.Context, id string) bool {
	return declarativeresource.CompositeIsDeclarativeHelper(
		id,
		func(id string) (bool, error) { return c.fileStore.IsOrganizationUnitExists(ctx, id) },
	)
}

// CheckOrganizationUnitNameConflict checks for name conflicts in both stores.
func (c *compositeOUStore) CheckOrganizationUnitNameConflict(
	ctx context.Context, name string, parent *string,
) (bool, error) {
	return declarativeresource.CompositeBooleanCheckHelper(
		func() (bool, error) { return c.fileStore.CheckOrganizationUnitNameConflict(ctx, name, parent) },
		func() (bool, error) { return c.dbStore.CheckOrganizationUnitNameConflict(ctx, name, parent) },
	)
}

// CheckOrganizationUnitHandleConflict checks for handle conflicts in both stores.
func (c *compositeOUStore) CheckOrganizationUnitHandleConflict(
	ctx context.Context, handle string, parent *string,
) (bool, error) {
	return declarativeresource.CompositeBooleanCheckHelper(
		func() (bool, error) { return c.fileStore.CheckOrganizationUnitHandleConflict(ctx, handle, parent) },
		func() (bool, error) { return c.dbStore.CheckOrganizationUnitHandleConflict(ctx, handle, parent) },
	)
}

// UpdateOrganizationUnit updates an organization unit in the database store only.
// Immutability checks and parent validation are handled at the service layer.
func (c *compositeOUStore) UpdateOrganizationUnit(ctx context.Context, ou OrganizationUnit) error {
	return c.dbStore.UpdateOrganizationUnit(ctx, ou)
}

// DeleteOrganizationUnit deletes an organization unit from the database store only.
// Immutability and children validation are handled at the service layer.
func (c *compositeOUStore) DeleteOrganizationUnit(ctx context.Context, id string) error {
	return c.dbStore.DeleteOrganizationUnit(ctx, id)
}

// GetOrganizationUnitChildrenCount retrieves the count of child OUs from both stores.
func (c *compositeOUStore) GetOrganizationUnitChildrenCount(
	ctx context.Context, id string, f *filter.FilterGroup,
) (int, error) {
	return declarativeresource.CompositeMergeCountHelper(
		func() (int, error) { return c.dbStore.GetOrganizationUnitChildrenCount(ctx, id, f) },
		func() (int, error) { return c.fileStore.GetOrganizationUnitChildrenCount(ctx, id, f) },
	)
}

// GetOrganizationUnitChildrenList retrieves child OUs from both stores with pagination.
// Applies the 1000-record limit in composite mode to prevent memory exhaustion.
// Returns ErrResultLimitExceededInCompositeMode if the limit is exceeded.
func (c *compositeOUStore) GetOrganizationUnitChildrenList(ctx context.Context,
	id string, limit, offset int, f *filter.FilterGroup,
) ([]OrganizationUnitBasic, error) {
	items, limitExceeded, err := declarativeresource.CompositeMergeListHelperWithLimit(
		func() (int, error) { return c.dbStore.GetOrganizationUnitChildrenCount(ctx, id, f) },
		func() (int, error) { return c.fileStore.GetOrganizationUnitChildrenCount(ctx, id, f) },
		func(count int) ([]OrganizationUnitBasic, error) {
			return c.dbStore.GetOrganizationUnitChildrenList(ctx, id, count, 0, f)
		},
		func(count int) ([]OrganizationUnitBasic, error) {
			return c.fileStore.GetOrganizationUnitChildrenList(ctx, id, count, 0, f)
		},
		mergeAndDeduplicateChildren,
		limit,
		offset,
		serverconst.MaxCompositeStoreRecords,
	)
	if err != nil {
		return nil, err
	}
	if limitExceeded {
		return nil, ErrResultLimitExceededInCompositeMode
	}
	return items, nil
}

// mergeAndDeduplicateOUs merges root-level OUs from both stores and removes duplicates by ID.
// While duplicates shouldn't exist by design (an OU exists in only one store), this provides
// defensive programming against misconfigurations or bugs.
func mergeAndDeduplicateOUs(dbOUs, fileOUs []OrganizationUnitBasic) []OrganizationUnitBasic {
	seen := make(map[string]bool)
	result := make([]OrganizationUnitBasic, 0, len(dbOUs)+len(fileOUs))

	// Add DB OUs first (they take precedence) - mark as mutable (isReadOnly=false)
	for i := range dbOUs {
		if !seen[dbOUs[i].ID] {
			seen[dbOUs[i].ID] = true
			dbOUs[i].IsReadOnly = false
			result = append(result, dbOUs[i])
		}
	}

	// Add file OUs if not already present - mark as immutable (isReadOnly=true)
	for i := range fileOUs {
		if !seen[fileOUs[i].ID] {
			seen[fileOUs[i].ID] = true
			fileOUs[i].IsReadOnly = true
			result = append(result, fileOUs[i])
		}
	}

	return result
}

// mergeAndDeduplicateChildren merges children from both stores and removes duplicates by ID.
// While duplicates shouldn't exist by design (an OU exists in only one store), this provides
// defensive programming against misconfigurations or bugs.
func mergeAndDeduplicateChildren(dbChildren, fileChildren []OrganizationUnitBasic) []OrganizationUnitBasic {
	seen := make(map[string]bool)
	result := make([]OrganizationUnitBasic, 0, len(dbChildren)+len(fileChildren))

	// Add DB children first (they take precedence) - mark as mutable (isReadOnly=false)
	for i := range dbChildren {
		if !seen[dbChildren[i].ID] {
			seen[dbChildren[i].ID] = true
			dbChildren[i].IsReadOnly = false
			result = append(result, dbChildren[i])
		}
	}

	// Add file children if not already present - mark as immutable (isReadOnly=true)
	for i := range fileChildren {
		if !seen[fileChildren[i].ID] {
			seen[fileChildren[i].ID] = true
			fileChildren[i].IsReadOnly = true
			result = append(result, fileChildren[i])
		}
	}

	return result
}
