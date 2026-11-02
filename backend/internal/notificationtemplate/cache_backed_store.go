// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notificationtemplate

import (
	"context"
	"errors"

	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// cacheBackedStore caches runtime GetTemplateByHandle reads by (channel, handle).
// Writes update or invalidate the cache. Other reads bypass the cache.
type cacheBackedStore struct {
	byHandle cache.CacheInterface[templateDAO]
	inner    notificationTemplateStoreInterface
	logger   *log.Logger
}

// newCacheBackedStore wraps inner with the given by-handle cache.
func newCacheBackedStore(byHandle cache.CacheInterface[templateDAO],
	inner notificationTemplateStoreInterface) notificationTemplateStoreInterface {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "CacheBackedNotificationTemplateStore"))
	return &cacheBackedStore{byHandle: byHandle, inner: inner, logger: logger}
}

// cacheKey builds the by-handle cache key. Templates are addressed at runtime by (channel, handle); the
// inner store scopes by deployment, and the cache is scoped to a single deployment, so the key needs no
// deployment id.
func cacheKey(channel ChannelType, handle string) cache.CacheKey {
	return cache.CacheKey{Key: string(channel) + ":" + handle}
}

// CreateTemplate delegates to the inner store, then populates the cache with the new row.
func (s *cacheBackedStore) CreateTemplate(ctx context.Context, t templateDAO) error {
	if err := s.inner.CreateTemplate(ctx, t); err != nil {
		return err
	}
	s.set(ctx, t)
	return nil
}

// GetTemplate retrieves a template by id. This is the management path; it always delegates and is not
// cached (the runtime hot path is GetTemplateByHandle).
func (s *cacheBackedStore) GetTemplate(ctx context.Context, channel ChannelType, id string) (
	templateDAO, error) {
	return s.inner.GetTemplate(ctx, channel, id)
}

// GetTemplateByHandle serves from cache on a hit, otherwise loads from the inner store and caches it.
func (s *cacheBackedStore) GetTemplateByHandle(ctx context.Context, channel ChannelType, handle string) (
	templateDAO, error) {
	if cached, ok := s.byHandle.Get(ctx, cacheKey(channel, handle)); ok {
		return cached, nil
	}
	dao, err := s.inner.GetTemplateByHandle(ctx, channel, handle)
	if err != nil {
		return dao, err
	}
	s.set(ctx, dao)
	return dao, nil
}

// ListTemplates always delegates; the list is not cached.
func (s *cacheBackedStore) ListTemplates(ctx context.Context, channel ChannelType, limit, offset int) (
	[]templateDAO, error) {
	return s.inner.ListTemplates(ctx, channel, limit, offset)
}

// CountTemplates always delegates; the count is not cached.
func (s *cacheBackedStore) CountTemplates(ctx context.Context, channel ChannelType) (int, error) {
	return s.inner.CountTemplates(ctx, channel)
}

// UpdateTemplate delegates then re-caches the new row. The only cache key, (channel, handle), is
// immutable, so re-caching overwrites the existing entry; there is no secondary key an update could
// leave stale, so no separate invalidation is needed.
func (s *cacheBackedStore) UpdateTemplate(ctx context.Context, t templateDAO) error {
	if err := s.inner.UpdateTemplate(ctx, t); err != nil {
		return err
	}
	s.set(ctx, t)
	return nil
}

// DeleteTemplate resolves the handle, deletes the template, then invalidates the cache.
// Missing rows are tolerated; other read errors abort before deletion.
func (s *cacheBackedStore) DeleteTemplate(ctx context.Context, channel ChannelType, id string) error {
	dao, getErr := s.inner.GetTemplate(ctx, channel, id)
	if getErr != nil && !errors.Is(getErr, errTemplateNotFound) {
		return getErr
	}
	if err := s.inner.DeleteTemplate(ctx, channel, id); err != nil {
		return err
	}
	if getErr == nil {
		s.invalidate(ctx, channel, dao.Handle)
	}
	return nil
}

// IsHandleExists always delegates; uniqueness must be checked against the source of truth.
func (s *cacheBackedStore) IsHandleExists(ctx context.Context, channel ChannelType, handle string) (
	bool, error) {
	return s.inner.IsHandleExists(ctx, channel, handle)
}

// invalidate removes a template's cache entry, logging on failure without failing the operation.
func (s *cacheBackedStore) invalidate(ctx context.Context, channel ChannelType, handle string) {
	if handle == "" {
		return
	}
	if err := s.byHandle.Delete(ctx, cacheKey(channel, handle)); err != nil {
		s.logger.Error(ctx, "Failed to invalidate template cache", log.String("handle", handle), log.Error(err))
	}
}

// set caches a template by its (channel, handle), logging on failure without failing the operation.
func (s *cacheBackedStore) set(ctx context.Context, t templateDAO) {
	if t.Handle == "" {
		return
	}
	if err := s.byHandle.Set(ctx, cacheKey(t.Channel, t.Handle), t); err != nil {
		s.logger.Error(ctx, "Failed to cache template", log.String("handle", t.Handle), log.Error(err))
	}
}
