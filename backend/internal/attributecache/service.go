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

// Package attributecache provides attribute caching functionality.
package attributecache

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

const (
	loggerComponentName = "AttributeCacheService"
	// MaxTTLSeconds is the maximum allowed TTL in seconds to prevent time.Duration overflow.
	// Calculated as math.MaxInt64 / int64(time.Second) to ensure ttlSeconds * time.Second doesn't overflow.
	MaxTTLSeconds = math.MaxInt64 / int64(time.Second)
)

// AttributeCacheServiceInterface defines the interface for the attribute cache service.
type AttributeCacheServiceInterface interface {
	// CreateAttributeCache creates a new attribute cache entry.
	CreateAttributeCache(ctx context.Context, cache *AttributeCache) (*AttributeCache, *serviceerror.ServiceError)

	// GetAttributeCache retrieves an attribute cache entry by ID.
	GetAttributeCache(ctx context.Context, id string) (*AttributeCache, *serviceerror.ServiceError)

	// ExtendAttributeCacheTTL extends the TTL of an attribute cache entry.
	ExtendAttributeCacheTTL(
		ctx context.Context, id string, ttlSeconds int,
	) *serviceerror.ServiceError

	// DeleteAttributeCache deletes an attribute cache entry by ID.
	DeleteAttributeCache(ctx context.Context, id string) *serviceerror.ServiceError
}

// attributeCacheService is the default implementation of the AttributeCacheServiceInterface.
type attributeCacheService struct {
	store attributeCacheStoreInterface
}

// newAttributeCacheService creates a new instance of attributeCacheService with injected dependencies.
func newAttributeCacheService(store attributeCacheStoreInterface) AttributeCacheServiceInterface {
	return &attributeCacheService{
		store: store,
	}
}

// CreateAttributeCache creates a new attribute cache entry.
func (s *attributeCacheService) CreateAttributeCache(
	ctx context.Context, cache *AttributeCache,
) (*AttributeCache, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug("Creating attribute cache entry")

	if cache == nil {
		return nil, &ErrorInvalidRequestFormat
	}

	if len(cache.Attributes) == 0 {
		return nil, &ErrorMissingAttributes
	}

	if cache.TTLSeconds <= 0 || int64(cache.TTLSeconds) > MaxTTLSeconds {
		return nil, &ErrorInvalidExpiryTime
	}

	var err error
	cache.ID, err = utils.GenerateUUIDv7()
	if err != nil {
		logger.Error("Failed to generate UUID", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	err = s.store.CreateAttributeCache(ctx, *cache)
	if err != nil {
		logger.Error("Failed to create attribute cache", log.Error(err), log.String("id", cache.ID))
		return nil, &serviceerror.InternalServerError
	}

	logger.Debug("Successfully created attribute cache", log.String("id", cache.ID))
	return cache, nil
}

// GetAttributeCache retrieves an attribute cache entry by ID.
func (s *attributeCacheService) GetAttributeCache(
	ctx context.Context, id string,
) (*AttributeCache, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug("Retrieving attribute cache", log.String("id", id))

	if strings.TrimSpace(id) == "" {
		return nil, &ErrorMissingCacheID
	}

	cache, err := s.store.GetAttributeCache(ctx, id)
	if err != nil {
		if errors.Is(err, errAttributeCacheNotFound) {
			logger.Debug("Attribute cache not found", log.String("id", id))
			return nil, &ErrorAttributeCacheNotFound
		}
		logger.Error("Failed to retrieve attribute cache", log.Error(err), log.String("id", id))
		return nil, &serviceerror.InternalServerError
	}

	logger.Debug("Successfully retrieved attribute cache", log.String("id", id))
	return &cache, nil
}

// ExtendAttributeCacheTTL extends the TTL of an attribute cache entry.
func (s *attributeCacheService) ExtendAttributeCacheTTL(
	ctx context.Context, id string, ttlSeconds int,
) *serviceerror.ServiceError {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug("Extending attribute cache TTL", log.String("id", id))

	if strings.TrimSpace(id) == "" {
		return &ErrorMissingCacheID
	}

	if ttlSeconds <= 0 || int64(ttlSeconds) > MaxTTLSeconds {
		return &ErrorInvalidExpiryTime
	}

	err := s.store.ExtendAttributeCacheTTL(ctx, id, ttlSeconds)
	if err != nil {
		if errors.Is(err, errAttributeCacheNotFound) {
			logger.Debug("Attribute cache not found", log.String("id", id))
			return &ErrorAttributeCacheNotFound
		}
		logger.Error("Failed to extend attribute cache TTL", log.Error(err), log.String("id", id))
		return &serviceerror.InternalServerError
	}

	logger.Debug("Successfully extended attribute cache TTL", log.String("id", id))
	return nil
}

// DeleteAttributeCache deletes an attribute cache entry by ID.
func (s *attributeCacheService) DeleteAttributeCache(
	ctx context.Context, id string,
) *serviceerror.ServiceError {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug("Deleting attribute cache", log.String("id", id))

	if strings.TrimSpace(id) == "" {
		return &ErrorMissingCacheID
	}

	err := s.store.DeleteAttributeCache(ctx, id)
	if err != nil {
		if errors.Is(err, errAttributeCacheNotFound) {
			logger.Debug("Attribute cache not found", log.String("id", id))
			return &ErrorAttributeCacheNotFound
		}
		logger.Error("Failed to delete attribute cache", log.Error(err), log.String("id", id))
		return &serviceerror.InternalServerError
	}

	logger.Debug("Successfully deleted attribute cache", log.String("id", id))
	return nil
}
