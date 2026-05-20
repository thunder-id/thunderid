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

package thememgt

import (
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
)

// compositeThemeStore implements a composite store that combines file-based (immutable) and
// database (mutable) stores.
// - Read operations query both stores and merge results
// - Write operations (Create/Update/Delete) only affect the database store
// - Declarative themes (from YAML files) cannot be modified or deleted
type compositeThemeStore struct {
	fileStore themeMgtStoreInterface
	dbStore   themeMgtStoreInterface
}

// newCompositeThemeStore creates a new composite store with both file-based and database stores.
func newCompositeThemeStore(fileStore, dbStore themeMgtStoreInterface) *compositeThemeStore {
	return &compositeThemeStore{
		fileStore: fileStore,
		dbStore:   dbStore,
	}
}

// GetThemeListCount retrieves the total count of themes from both stores.
func (c *compositeThemeStore) GetThemeListCount() (int, error) {
	return declarativeresource.CompositeMergeCountHelper(
		func() (int, error) { return c.dbStore.GetThemeListCount() },
		func() (int, error) { return c.fileStore.GetThemeListCount() },
	)
}

// GetThemeList retrieves themes from both stores with pagination.
// Applies the 1000-record limit in composite mode to prevent memory exhaustion.
// Returns errResultLimitExceededInCompositeMode if the limit is exceeded.
func (c *compositeThemeStore) GetThemeList(limit, offset int) ([]Theme, error) {
	items, limitExceeded, err := declarativeresource.CompositeMergeListHelperWithLimit(
		func() (int, error) { return c.dbStore.GetThemeListCount() },
		func() (int, error) { return c.fileStore.GetThemeListCount() },
		func(count int) ([]Theme, error) { return c.dbStore.GetThemeList(count, 0) },
		func(count int) ([]Theme, error) { return c.fileStore.GetThemeList(count, 0) },
		mergeAndDeduplicateThemes,
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

// CreateTheme creates a new theme in the database store only.
// Conflict checking is handled at the service layer.
func (c *compositeThemeStore) CreateTheme(id string, theme CreateThemeRequest) error {
	return c.dbStore.CreateTheme(id, theme)
}

// GetTheme retrieves a theme by ID from either store.
// Checks database store first, then falls back to file store (declarative).
func (c *compositeThemeStore) GetTheme(id string) (Theme, error) {
	theme, err := declarativeresource.CompositeGetHelper(
		func() (Theme, error) {
			theme, err := c.dbStore.GetTheme(id)
			if err != nil {
				return Theme{}, err
			}
			theme.IsReadOnly = false
			return theme, nil
		},
		func() (Theme, error) {
			theme, err := c.fileStore.GetTheme(id)
			if err != nil {
				return Theme{}, err
			}
			theme.IsReadOnly = true
			return theme, nil
		},
		errThemeNotFound,
	)
	return theme, err
}

// IsThemeExist checks if a theme exists in either store.
func (c *compositeThemeStore) IsThemeExist(id string) (bool, error) {
	// Check database store first
	exists, err := c.dbStore.IsThemeExist(id)
	if err != nil {
		return false, err
	}
	if exists {
		return true, nil
	}

	// Check file store
	return c.fileStore.IsThemeExist(id)
}

// UpdateTheme updates a theme in the database store only.
// Returns an error if the theme is declarative (immutable).
func (c *compositeThemeStore) UpdateTheme(id string, theme UpdateThemeRequest) error {
	return declarativeresource.CompositeUpdateHelper(
		theme,
		func(UpdateThemeRequest) string { return id },
		func(id string) (bool, error) { return c.fileStore.IsThemeExist(id) },
		func(UpdateThemeRequest) error { return c.dbStore.UpdateTheme(id, theme) },
		errCannotUpdateDeclarativeTheme,
	)
}

// DeleteTheme deletes a theme from the database store only.
// Returns an error if the theme is declarative (immutable).
func (c *compositeThemeStore) DeleteTheme(id string) error {
	return declarativeresource.CompositeDeleteHelper(
		id,
		func(id string) (bool, error) { return c.fileStore.IsThemeExist(id) },
		func(id string) error { return c.dbStore.DeleteTheme(id) },
		errCannotDeleteDeclarativeTheme,
	)
}

// GetApplicationsCountByThemeID retrieves the count of applications using a theme.
// Only queries database store since declarative themes don't track application references.
func (c *compositeThemeStore) GetApplicationsCountByThemeID(id string) (int, error) {
	return c.dbStore.GetApplicationsCountByThemeID(id)
}

// IsThemeDeclarative checks if a theme is immutable (exists in file store).
func (c *compositeThemeStore) IsThemeDeclarative(id string) bool {
	exists, err := c.fileStore.IsThemeExist(id)
	return err == nil && exists
}

// IsThemeHandleConflict checks if a theme handle conflicts in either store.
func (c *compositeThemeStore) IsThemeHandleConflict(handle string, excludeID string) (bool, error) {
	// Check file store first
	conflict, err := c.fileStore.IsThemeHandleConflict(handle, excludeID)
	if err != nil {
		return false, err
	}
	if conflict {
		return true, nil
	}
	// Then check db store
	return c.dbStore.IsThemeHandleConflict(handle, excludeID)
}

// mergeAndDeduplicateThemes merges themes from DB and file stores, removing duplicates.
// File store (declarative) themes take precedence over DB themes with the same ID.
func mergeAndDeduplicateThemes(dbThemes, fileThemes []Theme) []Theme {
	// Create a map to track IDs we've seen
	seen := make(map[string]bool)
	merged := make([]Theme, 0, len(dbThemes)+len(fileThemes))

	// Add file-based (declarative) themes first (they take precedence)
	for i := range fileThemes {
		if !seen[fileThemes[i].ID] {
			fileThemes[i].IsReadOnly = true
			merged = append(merged, fileThemes[i])
			seen[fileThemes[i].ID] = true
		}
	}

	// Add database themes (skip if already added from file store)
	for i := range dbThemes {
		if !seen[dbThemes[i].ID] {
			dbThemes[i].IsReadOnly = false
			merged = append(merged, dbThemes[i])
			seen[dbThemes[i].ID] = true
		}
	}

	return merged
}
