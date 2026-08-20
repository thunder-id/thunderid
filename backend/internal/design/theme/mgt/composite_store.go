// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package thememgt

import (
	"context"

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
func (c *compositeThemeStore) GetThemeListCount(ctx context.Context) (int, error) {
	return declarativeresource.CompositeMergeCountHelper(
		func() (int, error) { return c.dbStore.GetThemeListCount(ctx) },
		func() (int, error) { return c.fileStore.GetThemeListCount(ctx) },
	)
}

// GetThemeList retrieves themes from both stores with pagination.
// Applies the 1000-record limit in composite mode to prevent memory exhaustion.
// Returns errResultLimitExceededInCompositeMode if the limit is exceeded.
func (c *compositeThemeStore) GetThemeList(ctx context.Context, limit, offset int) ([]Theme, error) {
	items, limitExceeded, err := declarativeresource.CompositeMergeListHelperWithLimit(
		func() (int, error) { return c.dbStore.GetThemeListCount(ctx) },
		func() (int, error) { return c.fileStore.GetThemeListCount(ctx) },
		func(count int) ([]Theme, error) { return c.dbStore.GetThemeList(ctx, count, 0) },
		func(count int) ([]Theme, error) { return c.fileStore.GetThemeList(ctx, count, 0) },
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
func (c *compositeThemeStore) CreateTheme(ctx context.Context, id string, theme CreateThemeRequest) error {
	return c.dbStore.CreateTheme(ctx, id, theme)
}

// GetTheme retrieves a theme by ID from either store.
// Checks database store first, then falls back to file store (declarative).
func (c *compositeThemeStore) GetTheme(ctx context.Context, id string) (Theme, error) {
	theme, err := declarativeresource.CompositeGetHelper(
		func() (Theme, error) {
			theme, err := c.dbStore.GetTheme(ctx, id)
			if err != nil {
				return Theme{}, err
			}
			theme.IsReadOnly = false
			return theme, nil
		},
		func() (Theme, error) {
			theme, err := c.fileStore.GetTheme(ctx, id)
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
func (c *compositeThemeStore) IsThemeExist(ctx context.Context, id string) (bool, error) {
	// Check database store first
	exists, err := c.dbStore.IsThemeExist(ctx, id)
	if err != nil {
		return false, err
	}
	if exists {
		return true, nil
	}

	// Check file store
	return c.fileStore.IsThemeExist(ctx, id)
}

// UpdateTheme updates a theme in the database store only.
// Returns an error if the theme is declarative (immutable).
func (c *compositeThemeStore) UpdateTheme(ctx context.Context, id string, theme UpdateThemeRequest) error {
	return declarativeresource.CompositeUpdateHelper(
		theme,
		func(UpdateThemeRequest) string { return id },
		func(id string) (bool, error) { return c.fileStore.IsThemeExist(ctx, id) },
		func(UpdateThemeRequest) error { return c.dbStore.UpdateTheme(ctx, id, theme) },
		errCannotUpdateDeclarativeTheme,
	)
}

// DeleteTheme deletes a theme from the database store only.
// Returns an error if the theme is declarative (immutable).
func (c *compositeThemeStore) DeleteTheme(ctx context.Context, id string) error {
	return declarativeresource.CompositeDeleteHelper(
		id,
		func(id string) (bool, error) { return c.fileStore.IsThemeExist(ctx, id) },
		func(id string) error { return c.dbStore.DeleteTheme(ctx, id) },
		errCannotDeleteDeclarativeTheme,
	)
}

// IsThemeDeclarative checks if a theme is immutable (exists in file store).
func (c *compositeThemeStore) IsThemeDeclarative(id string) bool {
	exists, err := c.fileStore.IsThemeExist(context.Background(), id)
	return err == nil && exists
}

// IsThemeHandleConflict checks if a theme handle conflicts in either store.
func (c *compositeThemeStore) IsThemeHandleConflict(ctx context.Context, handle string, excludeID string) (bool,
	error) {
	// Check file store first
	conflict, err := c.fileStore.IsThemeHandleConflict(ctx, handle, excludeID)
	if err != nil {
		return false, err
	}
	if conflict {
		return true, nil
	}
	// Then check db store
	return c.dbStore.IsThemeHandleConflict(ctx, handle, excludeID)
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
