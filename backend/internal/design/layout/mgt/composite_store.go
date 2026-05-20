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

package layoutmgt

import (
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
)

// compositeLayoutStore implements a composite store that combines file-based (immutable) and
// database (mutable) stores.
// - Read operations query both stores and merge results
// - Write operations (Create/Update/Delete) only affect the database store
// - Declarative layouts (from YAML files) cannot be modified or deleted
type compositeLayoutStore struct {
	fileStore layoutMgtStoreInterface
	dbStore   layoutMgtStoreInterface
}

// newCompositeLayoutStore creates a new composite store with both file-based and database stores.
func newCompositeLayoutStore(fileStore, dbStore layoutMgtStoreInterface) *compositeLayoutStore {
	return &compositeLayoutStore{
		fileStore: fileStore,
		dbStore:   dbStore,
	}
}

// GetLayoutListCount retrieves the total count of layouts from both stores.
func (c *compositeLayoutStore) GetLayoutListCount() (int, error) {
	return declarativeresource.CompositeMergeCountHelper(
		func() (int, error) { return c.dbStore.GetLayoutListCount() },
		func() (int, error) { return c.fileStore.GetLayoutListCount() },
	)
}

// GetLayoutList retrieves layouts from both stores with pagination.
// Applies the 1000-record limit in composite mode to prevent memory exhaustion.
// Returns errResultLimitExceededInCompositeMode if the limit is exceeded.
func (c *compositeLayoutStore) GetLayoutList(limit, offset int) ([]Layout, error) {
	items, limitExceeded, err := declarativeresource.CompositeMergeListHelperWithLimit(
		func() (int, error) { return c.dbStore.GetLayoutListCount() },
		func() (int, error) { return c.fileStore.GetLayoutListCount() },
		func(count int) ([]Layout, error) { return c.dbStore.GetLayoutList(count, 0) },
		func(count int) ([]Layout, error) { return c.fileStore.GetLayoutList(count, 0) },
		mergeAndDeduplicateLayouts,
		limit,
		offset,
		serverconst.MaxCompositeStoreRecords, // Apply 1000-record limit
	)
	if err != nil {
		return nil, err
	}
	// Return limit exceeded as an error
	if limitExceeded {
		return nil, errResultLimitExceededInCompositeMode
	}
	return items, nil
}

// CreateLayout creates a new layout in the database store only.
// Conflict checking is handled at the service layer.
func (c *compositeLayoutStore) CreateLayout(id string, layout CreateLayoutRequest) error {
	return c.dbStore.CreateLayout(id, layout)
}

// GetLayout retrieves a layout by ID from either store.
// Checks database store first, then falls back to file store (declarative).
func (c *compositeLayoutStore) GetLayout(id string) (Layout, error) {
	layout, err := declarativeresource.CompositeGetHelper(
		func() (Layout, error) {
			layout, err := c.dbStore.GetLayout(id)
			if err != nil {
				return Layout{}, err
			}
			layout.IsReadOnly = false
			return layout, nil
		},
		func() (Layout, error) {
			layout, err := c.fileStore.GetLayout(id)
			if err != nil {
				return Layout{}, err
			}
			layout.IsReadOnly = true
			return layout, nil
		},
		errLayoutNotFound,
	)
	return layout, err
}

// IsLayoutExist checks if a layout exists in either store.
func (c *compositeLayoutStore) IsLayoutExist(id string) (bool, error) {
	// Check database store first
	exists, err := c.dbStore.IsLayoutExist(id)
	if err != nil {
		return false, err
	}
	if exists {
		return true, nil
	}

	// Check file store
	return c.fileStore.IsLayoutExist(id)
}

// UpdateLayout updates a layout in the database store only.
// Returns an error if the layout is declarative (immutable).
func (c *compositeLayoutStore) UpdateLayout(id string, layout UpdateLayoutRequest) error {
	return declarativeresource.CompositeUpdateHelper(
		layout,
		func(UpdateLayoutRequest) string { return id },
		func(id string) (bool, error) { return c.fileStore.IsLayoutExist(id) },
		func(UpdateLayoutRequest) error { return c.dbStore.UpdateLayout(id, layout) },
		errCannotUpdateDeclarativeLayout,
	)
}

// DeleteLayout deletes a layout from the database store only.
// Returns an error if the layout is declarative (immutable).
func (c *compositeLayoutStore) DeleteLayout(id string) error {
	return declarativeresource.CompositeDeleteHelper(
		id,
		func(id string) (bool, error) { return c.fileStore.IsLayoutExist(id) },
		func(id string) error { return c.dbStore.DeleteLayout(id) },
		errCannotDeleteDeclarativeLayout,
	)
}

// GetApplicationsCountByLayoutID retrieves the count of applications using a layout.
// Only queries database store since declarative layouts don't track application references.
func (c *compositeLayoutStore) GetApplicationsCountByLayoutID(id string) (int, error) {
	return c.dbStore.GetApplicationsCountByLayoutID(id)
}

// IsLayoutDeclarative checks if a layout is immutable (exists in file store).
func (c *compositeLayoutStore) IsLayoutDeclarative(id string) bool {
	exists, err := c.fileStore.IsLayoutExist(id)
	return err == nil && exists
}

// IsLayoutHandleConflict checks if a layout handle conflicts in either store.
func (c *compositeLayoutStore) IsLayoutHandleConflict(handle string, excludeID string) (bool, error) {
	// Check file store first
	conflict, err := c.fileStore.IsLayoutHandleConflict(handle, excludeID)
	if err != nil {
		return false, err
	}
	if conflict {
		return true, nil
	}
	// Then check db store
	return c.dbStore.IsLayoutHandleConflict(handle, excludeID)
}

// mergeAndDeduplicateLayouts merges layouts from DB and file stores, removing duplicates.
// File store (declarative) layouts take precedence over DB layouts with the same ID.
func mergeAndDeduplicateLayouts(dbLayouts, fileLayouts []Layout) []Layout {
	// Create a map to track IDs we've seen
	seen := make(map[string]bool)
	merged := make([]Layout, 0, len(dbLayouts)+len(fileLayouts))

	// Add file-based (declarative) layouts first (they take precedence)
	for i := range fileLayouts {
		if !seen[fileLayouts[i].ID] {
			fileLayouts[i].IsReadOnly = true
			merged = append(merged, fileLayouts[i])
			seen[fileLayouts[i].ID] = true
		}
	}

	// Add database layouts (skip if already added from file store)
	for i := range dbLayouts {
		if !seen[dbLayouts[i].ID] {
			dbLayouts[i].IsReadOnly = false
			merged = append(merged, dbLayouts[i])
			seen[dbLayouts[i].ID] = true
		}
	}

	return merged
}
