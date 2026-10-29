// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzdetailmgt

import (
	"context"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// compositeTypeStore implements a composite store that combines file-based (immutable) and database
// (mutable) stores.
// - Read operations query both stores and merge results
// - Write operations (Create/Update/Delete) only affect the database store
// - Declarative types (from YAML files) cannot be modified or deleted
type compositeTypeStore struct {
	fileStore typeStoreInterface
	dbStore   typeStoreInterface
}

// newCompositeTypeStore creates a new composite store with both file-based and database stores.
func newCompositeTypeStore(fileStore, dbStore typeStoreInterface) *compositeTypeStore {
	return &compositeTypeStore{
		fileStore: fileStore,
		dbStore:   dbStore,
	}
}

// CreateType creates a type in the database store only.
func (c *compositeTypeStore) CreateType(ctx context.Context, t providers.AuthorizationDetailType) error {
	return c.dbStore.CreateType(ctx, t)
}

// GetTypeByID retrieves a type by id from either store, checking the database first.
func (c *compositeTypeStore) GetTypeByID(ctx context.Context, id string) (providers.AuthorizationDetailType, error) {
	return declarativeresource.CompositeGetHelper(
		func() (providers.AuthorizationDetailType, error) { return c.dbStore.GetTypeByID(ctx, id) },
		func() (providers.AuthorizationDetailType, error) { return c.fileStore.GetTypeByID(ctx, id) },
		errTypeNotFound,
	)
}

// GetTypeByValue retrieves a type by its type value from either store, checking the database first.
func (c *compositeTypeStore) GetTypeByValue(ctx context.Context, typ string) (providers.AuthorizationDetailType,
	error) {
	return declarativeresource.CompositeGetHelper(
		func() (providers.AuthorizationDetailType, error) { return c.dbStore.GetTypeByValue(ctx, typ) },
		func() (providers.AuthorizationDetailType, error) { return c.fileStore.GetTypeByValue(ctx, typ) },
		errTypeNotFound,
	)
}

// ListTypes lists the types of both stores.
func (c *compositeTypeStore) ListTypes(ctx context.Context) ([]providers.AuthorizationDetailType, error) {
	dbTypes, err := c.dbStore.ListTypes(ctx)
	if err != nil {
		return nil, err
	}
	fileTypes, err := c.fileStore.ListTypes(ctx)
	if err != nil {
		return nil, err
	}
	return mergeTypes(dbTypes, fileTypes), nil
}

// CountTypes counts the types of both stores, counting a type both hold once.
func (c *compositeTypeStore) CountTypes(ctx context.Context) (int, error) {
	types, err := c.listCappedTypes(ctx)
	return len(types), err
}

// ListTypePage lists one page of the types of both stores. The page is cut from the merged list, since a type
// both stores hold is counted once.
func (c *compositeTypeStore) ListTypePage(ctx context.Context, limit, offset int) (
	[]providers.AuthorizationDetailType, error) {
	types, err := c.listCappedTypes(ctx)
	if err != nil {
		return nil, err
	}
	return pageOf(types, limit, offset), nil
}

// listCappedTypes lists the types of both stores, refusing a merged list over the composite mode limit.
func (c *compositeTypeStore) listCappedTypes(ctx context.Context) ([]providers.AuthorizationDetailType, error) {
	types, err := c.ListTypes(ctx)
	if err != nil {
		return nil, err
	}
	if len(types) > serverconst.MaxCompositeStoreRecords {
		return nil, errResultLimitExceededInCompositeMode
	}
	return types, nil
}

// UpdateType updates a type in the database store only.
func (c *compositeTypeStore) UpdateType(ctx context.Context, t providers.AuthorizationDetailType,
	bumpVersion bool) error {
	return declarativeresource.CompositeUpdateHelper(
		t,
		func(t providers.AuthorizationDetailType) string { return t.ID },
		func(id string) (bool, error) { return c.fileStore.IsTypeDeclarative(ctx, id) },
		func(t providers.AuthorizationDetailType) error { return c.dbStore.UpdateType(ctx, t, bumpVersion) },
		errTypeImmutable,
	)
}

// DeleteType deletes a type from the database store only.
func (c *compositeTypeStore) DeleteType(ctx context.Context, id string) error {
	return declarativeresource.CompositeDeleteHelper(
		id,
		func(id string) (bool, error) { return c.fileStore.IsTypeDeclarative(ctx, id) },
		func(id string) error { return c.dbStore.DeleteType(ctx, id) },
		errTypeImmutable,
	)
}

// IsTypeDeclarative reports whether the type is defined in the file-based store.
func (c *compositeTypeStore) IsTypeDeclarative(ctx context.Context, id string) (bool, error) {
	return c.fileStore.IsTypeDeclarative(ctx, id)
}

// pageOf returns the page of types starting at offset, with at most limit types.
func pageOf(types []providers.AuthorizationDetailType, limit, offset int) []providers.AuthorizationDetailType {
	start := min(offset, len(types))
	end := min(start+limit, len(types))
	return types[start:end]
}

// mergeTypes merges the types of both stores, ordered by type value; database types win on id clashes.
func mergeTypes(dbTypes, fileTypes []providers.AuthorizationDetailType) []providers.AuthorizationDetailType {
	seen := make(map[string]bool, len(dbTypes))
	result := make([]providers.AuthorizationDetailType, 0, len(dbTypes)+len(fileTypes))
	for _, t := range dbTypes {
		seen[t.ID] = true
		result = append(result, t)
	}
	for _, t := range fileTypes {
		if !seen[t.ID] {
			seen[t.ID] = true
			result = append(result, t)
		}
	}
	sortTypes(result)
	return result
}
