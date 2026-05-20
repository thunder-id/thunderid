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
	return f.CreateTheme(id, createReq)
}

// CreateTheme implements themeMgtStoreInterface.
func (f *themeFileBasedStore) CreateTheme(id string, theme CreateThemeRequest) error {
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
func (f *themeFileBasedStore) DeleteTheme(id string) error {
	return errors.New("deleteTheme is not supported in file-based store")
}

// GetTheme implements themeMgtStoreInterface.
func (f *themeFileBasedStore) GetTheme(id string) (Theme, error) {
	data, err := f.GenericFileBasedStore.Get(id)
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
func (f *themeFileBasedStore) GetThemeList(limit, offset int) ([]Theme, error) {
	// Validate input parameters to prevent panics
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		return []Theme{}, nil
	}

	list, err := f.GenericFileBasedStore.List()
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
func (f *themeFileBasedStore) GetThemeListCount() (int, error) {
	count, err := f.GenericFileBasedStore.Count()
	if err != nil {
		return 0, err
	}
	return count, nil
}

// IsThemeExist implements themeMgtStoreInterface.
func (f *themeFileBasedStore) IsThemeExist(id string) (bool, error) {
	_, err := f.GetTheme(id)
	if err != nil {
		return false, nil
	}
	return true, nil
}

// UpdateTheme implements themeMgtStoreInterface.
func (f *themeFileBasedStore) UpdateTheme(id string, theme UpdateThemeRequest) error {
	return errors.New("updateTheme is not supported in file-based store")
}

// GetApplicationsCountByThemeID implements themeMgtStoreInterface.
func (f *themeFileBasedStore) GetApplicationsCountByThemeID(id string) (int, error) {
	// In declarative mode, we don't track application references in the file-based store
	return 0, nil
}

// IsThemeDeclarative checks if a theme is immutable (in file-based store, all themes are immutable).
func (f *themeFileBasedStore) IsThemeDeclarative(id string) bool {
	return true
}

// IsThemeHandleConflict checks if a theme handle already exists (excluding a specific ID).
func (f *themeFileBasedStore) IsThemeHandleConflict(handle string, excludeID string) (bool, error) {
	list, err := f.GenericFileBasedStore.List()
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
