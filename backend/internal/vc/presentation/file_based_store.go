// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package presentation

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

// Create stores a presentation definition in the file-based store. Declarative resources
// are loaded through definitionStorer; the management API cannot reach this method because
// the service refuses creates while the store is in declarative-only mode.
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
	// Hand out a copy so a caller cannot mutate the shared declarative entry.
	stored := *dto
	return &stored, nil
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
	stored := *data.(*PresentationDefinitionDTO)
	return &stored, nil
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
