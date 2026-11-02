// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ou

import (
	"context"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// cacheBackedOUStore wraps an organizationUnitStoreInterface with in-memory caching
// for individual OU lookups by ID and by handle+parent.
type cacheBackedOUStore struct {
	ouByIDCache           cache.CacheInterface[*OrganizationUnit]
	ouByHandleParentCache cache.CacheInterface[*OrganizationUnit]
	store                 organizationUnitStoreInterface
	logger                *log.Logger
}

// newCacheBackedOUStore creates a cache-backed wrapper around the given store.
func newCacheBackedOUStore(store organizationUnitStoreInterface,
	ouByIDCache cache.CacheInterface[*OrganizationUnit],
	ouByHandleParentCache cache.CacheInterface[*OrganizationUnit]) organizationUnitStoreInterface {
	return &cacheBackedOUStore{
		ouByIDCache:           ouByIDCache,
		ouByHandleParentCache: ouByHandleParentCache,
		store:                 store,
		logger: log.GetLogger().With(
			log.String(log.LoggerKeyComponentName, "CacheBackedOUStore")),
	}
}

func (s *cacheBackedOUStore) CreateOrganizationUnit(ctx context.Context, ou OrganizationUnit) error {
	if err := s.store.CreateOrganizationUnit(ctx, ou); err != nil {
		return err
	}
	s.cacheOUByID(ctx, &ou)
	s.cacheOUByHandleParent(ctx, &ou)
	return nil
}

func (s *cacheBackedOUStore) GetOrganizationUnit(ctx context.Context, id string) (OrganizationUnit, error) {
	cacheKey := cache.CacheKey{Key: id}
	if cached, ok := s.ouByIDCache.Get(ctx, cacheKey); ok && cached != nil {
		return *cached, nil
	}

	ou, err := s.store.GetOrganizationUnit(ctx, id)
	if err != nil {
		return ou, err
	}

	s.cacheOUByID(ctx, &ou)
	return ou, nil
}

func (s *cacheBackedOUStore) GetOrganizationUnitByHandle(
	ctx context.Context, handle string, parent *string) (OrganizationUnit, error) {
	cacheKey := cache.CacheKey{Key: handleParentCacheKey(handle, parent)}
	if cached, ok := s.ouByHandleParentCache.Get(ctx, cacheKey); ok && cached != nil {
		return *cached, nil
	}

	ou, err := s.store.GetOrganizationUnitByHandle(ctx, handle, parent)
	if err != nil {
		return ou, err
	}

	s.cacheOUByID(ctx, &ou)
	s.cacheOUByHandleParent(ctx, &ou)
	return ou, nil
}

func (s *cacheBackedOUStore) UpdateOrganizationUnit(ctx context.Context, ou OrganizationUnit) error {
	// Capture old handle+parent key before the store call so we can invalidate it on success.
	oldHandleParentKey := s.getHandleParentKey(ctx, ou.ID)

	if err := s.store.UpdateOrganizationUnit(ctx, ou); err != nil {
		return err
	}

	if oldHandleParentKey != "" {
		s.deleteHandleParentCacheKey(ctx, oldHandleParentKey)
	}
	s.cacheOUByID(ctx, &ou)
	s.cacheOUByHandleParent(ctx, &ou)
	return nil
}

func (s *cacheBackedOUStore) DeleteOrganizationUnit(ctx context.Context, id string) error {
	// Capture handle+parent key before the store call so we can invalidate it on success.
	handleParentKey := s.getHandleParentKey(ctx, id)

	if err := s.store.DeleteOrganizationUnit(ctx, id); err != nil {
		return err
	}

	s.invalidateOUByID(ctx, id)
	if handleParentKey != "" {
		s.deleteHandleParentCacheKey(ctx, handleParentKey)
	}
	return nil
}

// Pass-through methods.

func (s *cacheBackedOUStore) GetOrganizationUnitListCount(
	ctx context.Context, f *tidcommon.FilterGroup) (int, error) {
	return s.store.GetOrganizationUnitListCount(ctx, f)
}

func (s *cacheBackedOUStore) GetOrganizationUnitList(
	ctx context.Context, limit, offset int, f *tidcommon.FilterGroup) ([]OrganizationUnitBasic, error) {
	return s.store.GetOrganizationUnitList(ctx, limit, offset, f)
}

func (s *cacheBackedOUStore) GetOrganizationUnitsByIDs(
	ctx context.Context, ids []string) ([]OrganizationUnitBasic, error) {
	return s.store.GetOrganizationUnitsByIDs(ctx, ids)
}

func (s *cacheBackedOUStore) GetOrganizationUnitByPath(
	ctx context.Context, handles []string) (OrganizationUnit, error) {
	return s.store.GetOrganizationUnitByPath(ctx, handles)
}

func (s *cacheBackedOUStore) IsOrganizationUnitExists(ctx context.Context, id string) (bool, error) {
	if cached, ok := s.ouByIDCache.Get(ctx, cache.CacheKey{Key: id}); ok && cached != nil {
		return true, nil
	}
	return s.store.IsOrganizationUnitExists(ctx, id)
}

func (s *cacheBackedOUStore) IsOrganizationUnitDeclarative(ctx context.Context, id string) bool {
	return s.store.IsOrganizationUnitDeclarative(ctx, id)
}

func (s *cacheBackedOUStore) CheckOrganizationUnitNameConflict(
	ctx context.Context, name string, parent *string) (bool, error) {
	return s.store.CheckOrganizationUnitNameConflict(ctx, name, parent)
}

func (s *cacheBackedOUStore) CheckOrganizationUnitHandleConflict(
	ctx context.Context, handle string, parent *string) (bool, error) {
	return s.store.CheckOrganizationUnitHandleConflict(ctx, handle, parent)
}

func (s *cacheBackedOUStore) GetOrganizationUnitChildrenCount(
	ctx context.Context, id string, f *tidcommon.FilterGroup) (int, error) {
	return s.store.GetOrganizationUnitChildrenCount(ctx, id, f)
}

func (s *cacheBackedOUStore) GetOrganizationUnitChildrenList(
	ctx context.Context,
	id string,
	limit, offset int,
	f *tidcommon.FilterGroup,
) ([]OrganizationUnitBasic, error) {
	return s.store.GetOrganizationUnitChildrenList(ctx, id, limit, offset, f)
}

// --- Cache helpers ---

// handleParentCacheKey builds a composite cache key from handle and parent.
// Root OUs (nil parent) use "handle:" while child OUs use "handle:parentID".
func handleParentCacheKey(handle string, parent *string) string {
	if parent == nil {
		return handle + ":"
	}
	return handle + ":" + *parent
}

func (s *cacheBackedOUStore) cacheOUByID(ctx context.Context, ou *OrganizationUnit) {
	if ou == nil || ou.ID == "" {
		return
	}
	if err := s.ouByIDCache.Set(ctx, cache.CacheKey{Key: ou.ID}, ou); err != nil {
		s.logger.Error(ctx, "Failed to cache OU by ID",
			log.String("ouID", ou.ID), log.Error(err))
	}
}

func (s *cacheBackedOUStore) cacheOUByHandleParent(ctx context.Context, ou *OrganizationUnit) {
	if ou == nil || ou.Handle == "" {
		return
	}
	key := handleParentCacheKey(ou.Handle, ou.Parent)
	if err := s.ouByHandleParentCache.Set(ctx, cache.CacheKey{Key: key}, ou); err != nil {
		s.logger.Error(ctx, "Failed to cache OU by handle+parent",
			log.String("handle", ou.Handle), log.Error(err))
	}
}

func (s *cacheBackedOUStore) invalidateOUByID(ctx context.Context, id string) {
	if id == "" {
		return
	}
	if err := s.ouByIDCache.Delete(ctx, cache.CacheKey{Key: id}); err != nil {
		s.logger.Error(ctx, "Failed to invalidate OU cache by ID",
			log.String("ouID", id), log.Error(err))
	}
}

// getHandleParentKey looks up the OU (from cache or store) and returns its
// handle+parent cache key. Returns "" if the OU cannot be found.
func (s *cacheBackedOUStore) getHandleParentKey(ctx context.Context, id string) string {
	if id == "" {
		return ""
	}
	var ou *OrganizationUnit
	if cached, ok := s.ouByIDCache.Get(ctx, cache.CacheKey{Key: id}); ok && cached != nil {
		ou = cached
	} else {
		fetched, err := s.store.GetOrganizationUnit(ctx, id)
		if err != nil {
			return ""
		}
		ou = &fetched
	}
	return handleParentCacheKey(ou.Handle, ou.Parent)
}

func (s *cacheBackedOUStore) deleteHandleParentCacheKey(ctx context.Context, key string) {
	if err := s.ouByHandleParentCache.Delete(ctx, cache.CacheKey{Key: key}); err != nil {
		s.logger.Error(ctx, "Failed to invalidate OU cache by handle+parent",
			log.String("cacheKey", key), log.Error(err))
	}
}
