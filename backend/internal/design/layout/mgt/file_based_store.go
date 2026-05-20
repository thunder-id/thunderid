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
	"errors"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
)

type layoutFileBasedStore struct {
	*declarativeresource.GenericFileBasedStore
}

// Create implements declarativeresource.Storer interface for resource loader
func (f *layoutFileBasedStore) Create(id string, data interface{}) error {
	layout, ok := data.(*Layout)
	if !ok {
		declarativeresource.LogTypeAssertionError("layout", id)
		return errors.New("invalid data type: expected *Layout")
	}
	createReq := CreateLayoutRequest{
		Handle:      layout.Handle,
		DisplayName: layout.DisplayName,
		Description: layout.Description,
		Layout:      layout.Layout,
	}
	return f.CreateLayout(id, createReq)
}

// CreateLayout implements layoutMgtStoreInterface.
func (f *layoutFileBasedStore) CreateLayout(id string, layout CreateLayoutRequest) error {
	layoutData := &Layout{
		ID:          id,
		Handle:      layout.Handle,
		DisplayName: layout.DisplayName,
		Description: layout.Description,
		Layout:      layout.Layout,
		CreatedAt:   "",
		UpdatedAt:   "",
	}
	return f.GenericFileBasedStore.Create(id, layoutData)
}

// DeleteLayout implements layoutMgtStoreInterface.
func (f *layoutFileBasedStore) DeleteLayout(id string) error {
	return errors.New("deleteLayout is not supported in file-based store")
}

// GetLayout implements layoutMgtStoreInterface.
func (f *layoutFileBasedStore) GetLayout(id string) (Layout, error) {
	data, err := f.GenericFileBasedStore.Get(id)
	if err != nil {
		return Layout{}, errLayoutNotFound
	}
	layout, ok := data.(*Layout)
	if !ok {
		declarativeresource.LogTypeAssertionError("layout", id)
		return Layout{}, errors.New("layout data corrupted")
	}
	return *layout, nil
}

// GetLayoutList implements layoutMgtStoreInterface.
func (f *layoutFileBasedStore) GetLayoutList(limit, offset int) ([]Layout, error) {
	// Validate input parameters to prevent panics
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		return []Layout{}, nil
	}

	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return nil, err
	}

	layoutList := make([]Layout, 0)
	for _, item := range list {
		if layout, ok := item.Data.(*Layout); ok {
			layoutList = append(layoutList, *layout)
		}
	}

	// Apply pagination
	start := offset
	if start >= len(layoutList) {
		return []Layout{}, nil
	}

	end := start + limit
	if end > len(layoutList) {
		end = len(layoutList)
	}
	return layoutList[start:end], nil
}

// GetLayoutListCount implements layoutMgtStoreInterface.
func (f *layoutFileBasedStore) GetLayoutListCount() (int, error) {
	count, err := f.GenericFileBasedStore.Count()
	if err != nil {
		return 0, err
	}
	return count, nil
}

// IsLayoutExist implements layoutMgtStoreInterface.
func (f *layoutFileBasedStore) IsLayoutExist(id string) (bool, error) {
	_, err := f.GetLayout(id)
	if err != nil {
		return false, nil
	}
	return true, nil
}

// UpdateLayout implements layoutMgtStoreInterface.
func (f *layoutFileBasedStore) UpdateLayout(id string, layout UpdateLayoutRequest) error {
	return errors.New("updateLayout is not supported in file-based store")
}

// GetApplicationsCountByLayoutID implements layoutMgtStoreInterface.
func (f *layoutFileBasedStore) GetApplicationsCountByLayoutID(id string) (int, error) {
	// In declarative mode, we don't track application references in the file-based store
	return 0, nil
}

// IsLayoutDeclarative checks if a layout is immutable (in file-based store, all layouts are immutable).
func (f *layoutFileBasedStore) IsLayoutDeclarative(id string) bool {
	return true
}

// IsLayoutHandleConflict checks if a layout handle already exists (excluding a specific ID).
func (f *layoutFileBasedStore) IsLayoutHandleConflict(handle string, excludeID string) (bool, error) {
	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return false, err
	}
	for _, item := range list {
		if layout, ok := item.Data.(*Layout); ok {
			if layout.Handle == handle && layout.ID != excludeID {
				return true, nil
			}
		}
	}
	return false, nil
}

// newLayoutFileBasedStore creates a new instance of a file-based store.
func newLayoutFileBasedStore() layoutMgtStoreInterface {
	genericStore := declarativeresource.NewGenericFileBasedStore(entity.KeyTypeLayout)
	return &layoutFileBasedStore{
		GenericFileBasedStore: genericStore,
	}
}
