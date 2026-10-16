// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize initializes the entity service.
// The entity store is always composite: a DB store backed by an in-memory file store.
// Declarative resources are loaded on demand by consumer packages (e.g. user, application)
// based on their own store mode configuration.
func Initialize(
	cacheManager cache.CacheManagerInterface,
	hashService cryptolib.HashServiceInterface,
	entityTypeService entitytype.EntityTypeServiceInterface,
	ouService ou.OrganizationUnitServiceInterface,
) (EntityServiceInterface, error) {
	store, transactioner, err := initializeStore(cacheManager)
	if err != nil {
		return nil, err
	}

	svc := newEntityService(store, newEntityRuntimeDBStore(), hashService, entityTypeService, ouService,
		transactioner)
	return svc, nil
}

// initializeStore always creates a composite store (DB + in-memory file store).
func initializeStore(cacheManager cache.CacheManagerInterface) (
	entityStoreInterface, providers.Transactioner, error) {
	fileStore := newEntityFileBasedStore()
	dbStore, transactioner, err := newEntityDBStore()
	if err != nil {
		return nil, nil, err
	}
	entityByIDCache := cache.GetCache[*providers.Entity](cacheManager, "EntityByIDCache")
	entityWithCredsByIDCache := cache.GetCache[*entityWithCredentials](cacheManager,
		"EntityWithCredentialsByIDCache")
	entityIDByIdentifierCache := cache.GetCache[*string](cacheManager,
		"EntityIDByIdentifierCache")
	cacheBackedEntityStore := newCacheBackedEntityStore(dbStore, entityByIDCache,
		entityWithCredsByIDCache, entityIDByIdentifierCache)
	return newEntityCompositeStore(fileStore, cacheBackedEntityStore), transactioner, nil
}
