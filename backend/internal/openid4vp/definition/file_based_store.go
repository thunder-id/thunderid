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

package definition

import (
	"context"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
)

type definitionFileBasedStore struct {
	*declarativeresource.GenericFileBasedStore
}

// newDefinitionFileBasedStore creates a new instance of a file-based store.
func newDefinitionFileBasedStore() *definitionFileBasedStore {
	genericStore := declarativeresource.NewGenericFileBasedStore(entity.KeyTypePresentationDefinition)
	return &definitionFileBasedStore{
		GenericFileBasedStore: genericStore,
	}
}

// Create stores a presentation definition in the file-based store. In declarative
// and composite modes the loader writes resources through this method (resources
// loaded from YAML are immutable; management writes route to the database store).
func (f *definitionFileBasedStore) CreatePresentationDefinition(
	_ context.Context, dto PresentationDefinitionDTO,
) error {
	return f.GenericFileBasedStore.Create(dto.ID, &dto)
}

// definitionStorer adapts the file-based store to declarativeresource.Storer so the
// resource loader can write parsed definitions through the (id, data) entry point.
type definitionStorer struct {
	store *definitionFileBasedStore
}

// Create implements declarativeresource.Storer for the resource loader.
func (s *definitionStorer) Create(id string, data interface{}) error {
	dto, ok := data.(*PresentationDefinitionDTO)
	if !ok {
		return ErrDefinitionDataCorrupted
	}
	if dto.ID == "" {
		dto.ID = id
	}
	return s.store.GenericFileBasedStore.Create(id, dto)
}

// GetByID retrieves a presentation definition by ID from the file-based store.
func (f *definitionFileBasedStore) GetPresentationDefinitionByID(
	_ context.Context, id string,
) (*PresentationDefinitionDTO, error) {
	data, err := f.GenericFileBasedStore.Get(id)
	if err != nil {
		return nil, ErrNotFound
	}
	dto, ok := data.(*PresentationDefinitionDTO)
	if !ok {
		declarativeresource.LogTypeAssertionError("presentation definition", id)
		return nil, ErrDefinitionDataCorrupted
	}
	return dto, nil
}

// GetByHandle retrieves a presentation definition by handle from the file-based store.
func (f *definitionFileBasedStore) GetPresentationDefinitionByHandle(
	_ context.Context, handle string,
) (*PresentationDefinitionDTO, error) {
	data, err := f.GenericFileBasedStore.GetByField(handle, func(d interface{}) string {
		return d.(*PresentationDefinitionDTO).Handle
	})
	if err != nil {
		return nil, ErrNotFound
	}
	return data.(*PresentationDefinitionDTO), nil
}

// ListSummaries retrieves minimal listing data from the file-based store.
func (f *definitionFileBasedStore) ListPresentationDefinitionSummaries(
	_ context.Context,
) ([]PresentationDefinitionList, error) {
	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return nil, err
	}
	summaries := make([]PresentationDefinitionList, 0, len(list))
	for _, item := range list {
		if dto, ok := item.Data.(*PresentationDefinitionDTO); ok {
			summaries = append(summaries, toSummary(*dto))
		}
	}
	return summaries, nil
}

// List retrieves all presentation definitions from the file-based store.
func (f *definitionFileBasedStore) ListPresentationDefinitions(_ context.Context) ([]PresentationDefinitionDTO, error) {
	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return nil, err
	}

	defs := make([]PresentationDefinitionDTO, 0, len(list))
	for _, item := range list {
		if dto, ok := item.Data.(*PresentationDefinitionDTO); ok {
			defs = append(defs, *dto)
		}
	}
	return defs, nil
}

// Update is not supported in the file-based store.
func (f *definitionFileBasedStore) UpdatePresentationDefinition(_ context.Context, _ PresentationDefinitionDTO) error {
	return ErrDefinitionIsImmutable
}

// Delete is not supported in the file-based store.
func (f *definitionFileBasedStore) DeletePresentationDefinition(_ context.Context, _ string) error {
	return ErrDefinitionIsImmutable
}

// IsDeclarative reports whether the given id exists in the file-based store.
func (f *definitionFileBasedStore) IsPresentationDefinitionDeclarative(_ context.Context, id string) (bool, error) {
	_, err := f.GenericFileBasedStore.Get(id)
	return err == nil, nil
}
