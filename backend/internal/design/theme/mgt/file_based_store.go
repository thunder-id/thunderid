// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package thememgt

import (
	"context"
	"errors"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
)

type themeFileBasedStore struct {
	*declarativeresource.GenericFileBasedStore
}

// Create implements declarativeresource.Storer interface for resource loader
func (f *themeFileBasedStore) Create(id string, data interface{}) error {
	theme, ok := data.(*Theme)
	if !ok {
		declarativeresource.LogTypeAssertionError("theme", id)
		return errors.New("invalid data type: expected *Theme")
	}
	createReq := CreateThemeRequest{
		Handle:      theme.Handle,
		DisplayName: theme.DisplayName,
		Description: theme.Description,
		Theme:       theme.Theme,
	}
	return f.CreateTheme(context.Background(), id, createReq)
}

// CreateTheme implements themeMgtStoreInterface.
func (f *themeFileBasedStore) CreateTheme(ctx context.Context, id string, theme CreateThemeRequest) error {
	themeData := &Theme{
		ID:          id,
		Handle:      theme.Handle,
		DisplayName: theme.DisplayName,
		Description: theme.Description,
		Theme:       theme.Theme,
		CreatedAt:   "",
		UpdatedAt:   "",
	}
	return f.GenericFileBasedStore.Create(id, themeData)
}

// DeleteTheme implements themeMgtStoreInterface.
func (f *themeFileBasedStore) DeleteTheme(ctx context.Context, id string) error {
	return errors.New("deleteTheme is not supported in file-based store")
}

// GetTheme implements themeMgtStoreInterface.
func (f *themeFileBasedStore) GetTheme(ctx context.Context, id string) (Theme, error) {
	data, err := f.GenericFileBasedStore.Get(ctx, id)
	if err != nil {
		return Theme{}, errThemeNotFound
	}
	theme, ok := data.(*Theme)
	if !ok {
		declarativeresource.LogTypeAssertionError("theme", id)
		return Theme{}, errors.New("theme data corrupted")
	}
	return *theme, nil
}

// GetThemeList implements themeMgtStoreInterface.
func (f *themeFileBasedStore) GetThemeList(ctx context.Context, limit, offset int) ([]Theme, error) {
	// Validate input parameters to prevent panics
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		return []Theme{}, nil
	}

	list, err := f.GenericFileBasedStore.List(ctx)
	if err != nil {
		return nil, err
	}

	themeList := make([]Theme, 0)
	for _, item := range list {
		if theme, ok := item.Data.(*Theme); ok {
			themeList = append(themeList, *theme)
		}
	}

	// Apply pagination
	start := offset
	if start >= len(themeList) {
		return []Theme{}, nil
	}

	end := start + limit
	if end > len(themeList) {
		end = len(themeList)
	}
	return themeList[start:end], nil
}

// GetThemeListCount implements themeMgtStoreInterface.
func (f *themeFileBasedStore) GetThemeListCount(ctx context.Context) (int, error) {
	count, err := f.GenericFileBasedStore.Count(ctx)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// IsThemeExist implements themeMgtStoreInterface.
func (f *themeFileBasedStore) IsThemeExist(ctx context.Context, id string) (bool, error) {
	_, err := f.GetTheme(ctx, id)
	if err != nil {
		return false, nil
	}
	return true, nil
}

// UpdateTheme implements themeMgtStoreInterface.
func (f *themeFileBasedStore) UpdateTheme(ctx context.Context, id string, theme UpdateThemeRequest) error {
	return errors.New("updateTheme is not supported in file-based store")
}

// IsThemeDeclarative checks if a theme is immutable (in file-based store, all themes are immutable).
func (f *themeFileBasedStore) IsThemeDeclarative(id string) bool {
	return true
}

// IsThemeHandleConflict checks if a theme handle already exists (excluding a specific ID).
func (f *themeFileBasedStore) IsThemeHandleConflict(ctx context.Context, handle string, excludeID string) (bool,
	error) {
	list, err := f.GenericFileBasedStore.List(ctx)
	if err != nil {
		return false, err
	}
	for _, item := range list {
		if theme, ok := item.Data.(*Theme); ok {
			if theme.Handle == handle && theme.ID != excludeID {
				return true, nil
			}
		}
	}
	return false, nil
}

// newThemeFileBasedStore creates a new instance of a file-based store.
func newThemeFileBasedStore() themeMgtStoreInterface {
	genericStore := declarativeresource.NewGenericFileBasedStore(entity.KeyTypeTheme)
	return &themeFileBasedStore{
		GenericFileBasedStore: genericStore,
	}
}
