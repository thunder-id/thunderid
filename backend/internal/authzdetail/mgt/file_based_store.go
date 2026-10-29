// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzdetailmgt

import (
	"context"
	"slices"
	"strings"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// typeFileBasedStore holds the read-only authorization details types loaded from declarative files.
type typeFileBasedStore struct {
	*declarativeresource.GenericFileBasedStore
}

// newTypeFileBasedStore creates a new file-based store for authorization details types.
func newTypeFileBasedStore() *typeFileBasedStore {
	return &typeFileBasedStore{
		GenericFileBasedStore: declarativeresource.NewGenericFileBasedStore(entity.KeyTypeAuthorizationDetailType),
	}
}

// Create implements declarativeresource.Storer for the resource loader.
func (f *typeFileBasedStore) Create(id string, data interface{}) error {
	t, ok := data.(*providers.AuthorizationDetailType)
	if !ok {
		return errTypeDataCorrupted
	}
	return f.GenericFileBasedStore.Create(id, t)
}

// CreateType is not supported in the file-based store.
func (f *typeFileBasedStore) CreateType(_ context.Context, _ providers.AuthorizationDetailType) error {
	return errTypeImmutable
}

// GetTypeByID retrieves a type by id.
func (f *typeFileBasedStore) GetTypeByID(_ context.Context, id string) (providers.AuthorizationDetailType, error) {
	data, err := f.GenericFileBasedStore.Get(id)
	if err != nil {
		return providers.AuthorizationDetailType{}, errTypeNotFound
	}
	return typeFromDeclarativeData(id, data)
}

// GetTypeByValue retrieves a type by its type value.
func (f *typeFileBasedStore) GetTypeByValue(ctx context.Context, typ string) (providers.AuthorizationDetailType,
	error) {
	types, err := f.ListTypes(ctx)
	if err != nil {
		return providers.AuthorizationDetailType{}, err
	}
	for _, t := range types {
		if t.Type == typ {
			return t, nil
		}
	}
	return providers.AuthorizationDetailType{}, errTypeNotFound
}

// ListTypes lists every type, ordered by type value.
func (f *typeFileBasedStore) ListTypes(ctx context.Context) ([]providers.AuthorizationDetailType, error) {
	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return nil, err
	}
	types := make([]providers.AuthorizationDetailType, 0, len(list))
	for _, item := range list {
		t, err := typeFromDeclarativeData(item.ID.ID, item.Data)
		if err != nil {
			log.GetLogger().Warn(ctx, "Skipping malformed declarative authorization details type",
				log.String("id", item.ID.ID), log.Error(err))
			continue
		}
		types = append(types, t)
	}
	sortTypes(types)
	return types, nil
}

// CountTypes counts the types.
func (f *typeFileBasedStore) CountTypes(ctx context.Context) (int, error) {
	types, err := f.ListTypes(ctx)
	return len(types), err
}

// ListTypePage lists one page of the types, ordered by type value.
func (f *typeFileBasedStore) ListTypePage(ctx context.Context, limit, offset int) (
	[]providers.AuthorizationDetailType, error) {
	types, err := f.ListTypes(ctx)
	if err != nil {
		return nil, err
	}
	return pageOf(types, limit, offset), nil
}

// UpdateType is not supported in the file-based store.
func (f *typeFileBasedStore) UpdateType(_ context.Context, _ providers.AuthorizationDetailType, _ bool) error {
	return errTypeImmutable
}

// DeleteType is not supported in the file-based store.
func (f *typeFileBasedStore) DeleteType(_ context.Context, _ string) error {
	return errTypeImmutable
}

// IsTypeDeclarative reports whether the type exists in the file-based store.
func (f *typeFileBasedStore) IsTypeDeclarative(_ context.Context, id string) (bool, error) {
	_, err := f.GenericFileBasedStore.Get(id)
	return err == nil, nil
}

// typeFromDeclarativeData returns a copy of a declarative type so callers cannot mutate the shared entry.
func typeFromDeclarativeData(id string, data interface{}) (providers.AuthorizationDetailType, error) {
	t, ok := data.(*providers.AuthorizationDetailType)
	if !ok || t == nil {
		declarativeresource.LogTypeAssertionError("authorization details type", id)
		return providers.AuthorizationDetailType{}, errTypeDataCorrupted
	}
	stored := *t
	stored.IsReadOnly = true
	return stored, nil
}

// sortTypes orders types by type value, as the database store lists them.
func sortTypes(types []providers.AuthorizationDetailType) {
	slices.SortFunc(types, func(a, b providers.AuthorizationDetailType) int {
		return strings.Compare(a.Type, b.Type)
	})
}
