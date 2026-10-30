// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package attributecache provides attribute caching functionality.
package attributecache

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/internal/system/kmprovider"
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
	CreateAttributeCache(ctx context.Context, cache *AttributeCache) (*AttributeCache, *tidcommon.ServiceError)

	// GetAttributeCache retrieves an attribute cache entry by ID.
	GetAttributeCache(ctx context.Context, id string) (*AttributeCache, *tidcommon.ServiceError)

	// ExtendAttributeCacheTTL extends the TTL of an attribute cache entry.
	ExtendAttributeCacheTTL(
		ctx context.Context, id string, ttlSeconds int,
	) *tidcommon.ServiceError

	// DeleteAttributeCache deletes an attribute cache entry by ID.
	DeleteAttributeCache(ctx context.Context, id string) *tidcommon.ServiceError
}

// attributeCacheService is the default implementation of the AttributeCacheServiceInterface.
type attributeCacheService struct {
	store             attributeCacheStoreInterface
	crypto            kmprovider.RuntimeCryptoProvider
	encryptionEnabled bool
}

// newAttributeCacheService creates a new instance of attributeCacheService with injected dependencies.
func newAttributeCacheService(
	store attributeCacheStoreInterface,
	crypto kmprovider.RuntimeCryptoProvider,
	encryptionEnabled bool,
) AttributeCacheServiceInterface {
	return &attributeCacheService{
		store:             store,
		crypto:            crypto,
		encryptionEnabled: encryptionEnabled,
	}
}

// CreateAttributeCache creates a new attribute cache entry.
func (s *attributeCacheService) CreateAttributeCache(
	ctx context.Context, cache *AttributeCache,
) (*AttributeCache, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug(ctx, "Creating attribute cache entry")

	if cache == nil {
		return nil, &ErrorInvalidRequestFormat
	}

	if len(cache.Attributes) == 0 && cache.SessionGrants == nil {
		return nil, &ErrorMissingAttributes
	}

	if cache.TTLSeconds <= 0 || cache.TTLSeconds > MaxTTLSeconds {
		return nil, &ErrorInvalidExpiryTime
	}

	var err error
	cache.ID, err = utils.GenerateUUIDv7()
	if err != nil {
		logger.Error(ctx, "Failed to generate UUID", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	data, err := json.Marshal(withSessionGrants(cache.Attributes, cache.SessionGrants))
	if err != nil {
		logger.Error(ctx, "Failed to marshal attributes", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if s.encryptionEnabled {
		ciphertext, _, encErr := s.crypto.Encrypt(ctx, nil, string(cryptolib.AlgorithmAESGCM),
			nil, data)
		if encErr != nil {
			logger.Error(ctx, "Failed to encrypt attributes", log.Error(encErr))
			return nil, &tidcommon.InternalServerError
		}
		data = ciphertext
	}

	err = s.store.CreateAttributeCache(ctx, cache.ID, data, cache.TTLSeconds)
	if err != nil {
		logger.Error(ctx, "Failed to create attribute cache", log.Error(err), log.String("id", cache.ID))
		return nil, &tidcommon.InternalServerError
	}

	logger.Debug(ctx, "Successfully created attribute cache", log.String("id", cache.ID))
	return cache, nil
}

// GetAttributeCache retrieves an attribute cache entry by ID.
func (s *attributeCacheService) GetAttributeCache(
	ctx context.Context, id string,
) (*AttributeCache, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug(ctx, "Retrieving attribute cache", log.String("id", id))

	if strings.TrimSpace(id) == "" {
		return nil, &ErrorMissingCacheID
	}

	data, err := s.store.GetAttributeCache(ctx, id)
	if err != nil {
		if errors.Is(err, errAttributeCacheNotFound) {
			logger.Debug(ctx, "Attribute cache not found", log.String("id", id))
			return nil, &ErrorAttributeCacheNotFound
		}
		logger.Error(ctx, "Failed to retrieve attribute cache", log.Error(err), log.String("id", id))
		return nil, &tidcommon.InternalServerError
	}

	if s.encryptionEnabled {
		plaintext, decErr := s.crypto.Decrypt(ctx, nil, string(cryptolib.AlgorithmAESGCM),
			nil, data)
		if decErr != nil {
			logger.Error(ctx, "Failed to decrypt attributes", log.Error(decErr), log.String("id", id))
			return nil, &tidcommon.InternalServerError
		}
		data = plaintext
	}
	var attrs map[string]interface{}
	if err := json.Unmarshal(data, &attrs); err != nil {
		logger.Error(ctx, "Failed to unmarshal attributes", log.Error(err), log.String("id", id))
		return nil, &tidcommon.InternalServerError
	}

	grants, err := splitSessionGrants(attrs)
	if err != nil {
		logger.Error(ctx, "Failed to read session grants", log.Error(err), log.String("id", id))
		return nil, &tidcommon.InternalServerError
	}

	logger.Debug(ctx, "Successfully retrieved attribute cache", log.String("id", id))
	return &AttributeCache{
		ID:            id,
		Attributes:    attrs,
		SessionGrants: grants,
	}, nil
}

// ExtendAttributeCacheTTL extends the TTL of an attribute cache entry.
func (s *attributeCacheService) ExtendAttributeCacheTTL(
	ctx context.Context, id string, ttlSeconds int,
) *tidcommon.ServiceError {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug(ctx, "Extending attribute cache TTL", log.String("id", id))

	if strings.TrimSpace(id) == "" {
		return &ErrorMissingCacheID
	}

	if ttlSeconds <= 0 || int64(ttlSeconds) > MaxTTLSeconds {
		return &ErrorInvalidExpiryTime
	}

	err := s.store.ExtendAttributeCacheTTL(ctx, id, ttlSeconds)
	if err != nil {
		if errors.Is(err, providers.ErrRuntimeStoreKeyNotFound) {
			logger.Debug(ctx, "Attribute cache not found", log.String("id", id))
			return &ErrorAttributeCacheNotFound
		}
		logger.Error(ctx, "Failed to extend attribute cache TTL", log.Error(err), log.String("id", id))
		return &tidcommon.InternalServerError
	}

	logger.Debug(ctx, "Successfully extended attribute cache TTL", log.String("id", id))
	return nil
}

// DeleteAttributeCache deletes an attribute cache entry by ID.
func (s *attributeCacheService) DeleteAttributeCache(
	ctx context.Context, id string,
) *tidcommon.ServiceError {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug(ctx, "Deleting attribute cache", log.String("id", id))

	if strings.TrimSpace(id) == "" {
		return &ErrorMissingCacheID
	}

	err := s.store.DeleteAttributeCache(ctx, id)
	if err != nil {
		logger.Error(ctx, "Failed to delete attribute cache", log.Error(err), log.String("id", id))
		return &tidcommon.InternalServerError
	}

	logger.Debug(ctx, "Successfully deleted attribute cache", log.String("id", id))
	return nil
}
