/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package flowmgt

import (
	"context"
	"errors"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/transaction"
)

const cacheBackedStoreLoggerComponentName = "CacheBackedFlowStore"

// cacheBackedFlowStore is the implementation of flowStoreInterface that uses caching.
type cacheBackedFlowStore struct {
	flowByIDCache     cache.CacheInterface[*CompleteFlowDefinition]
	flowByHandleCache cache.CacheInterface[*CompleteFlowDefinition]
	store             flowStoreInterface
	logger            *log.Logger
}

// newCacheBackedFlowStore creates a new instance of cacheBackedFlowStore.
func newCacheBackedFlowStore(
	flowByIDCache cache.CacheInterface[*CompleteFlowDefinition],
	flowByHandleCache cache.CacheInterface[*CompleteFlowDefinition],
) (flowStoreInterface, transaction.Transactioner, error) {
	store, transactioner, err := newFlowStore()
	if err != nil {
		return nil, nil, err
	}
	return &cacheBackedFlowStore{
		flowByIDCache:     flowByIDCache,
		flowByHandleCache: flowByHandleCache,
		store:             store,
		logger: log.GetLogger().With(
			log.String(log.LoggerKeyComponentName, cacheBackedStoreLoggerComponentName)),
	}, transactioner, nil
}

// ListFlows retrieves a paginated list of flow definitions.
// Note: List operations are not cached as they can vary by parameters and change frequently.
func (s *cacheBackedFlowStore) ListFlows(ctx context.Context, limit, offset int, flowType string) (
	[]BasicFlowDefinition, int, error) {
	return s.store.ListFlows(ctx, limit, offset, flowType)
}

// CreateFlow creates a new flow definition and caches it.
func (s *cacheBackedFlowStore) CreateFlow(ctx context.Context, flowID string, flow *FlowDefinition) (
	*CompleteFlowDefinition, error) {
	createdFlow, err := s.store.CreateFlow(ctx, flowID, flow)
	if err != nil {
		return nil, err
	}
	s.cacheFlow(ctx, createdFlow)

	return createdFlow, nil
}

// GetFlowByID retrieves a flow definition by its ID, using cache if available.
func (s *cacheBackedFlowStore) GetFlowByID(ctx context.Context, flowID string) (*CompleteFlowDefinition, error) {
	cacheKey := cache.CacheKey{
		Key: flowID,
	}
	cachedFlow, ok := s.flowByIDCache.Get(ctx, cacheKey)
	if ok {
		return cachedFlow, nil
	}

	flow, err := s.store.GetFlowByID(ctx, flowID)
	if err != nil || flow == nil {
		return flow, err
	}
	s.cacheFlow(ctx, flow)

	return flow, nil
}

// GetFlowByHandle retrieves a flow definition by handle and flow type, using cache if available.
func (s *cacheBackedFlowStore) GetFlowByHandle(ctx context.Context, handle string, flowType common.FlowType) (
	*CompleteFlowDefinition, error) {
	cacheKey := getFlowByHandleCacheKey(handle, flowType)
	cachedFlow, ok := s.flowByHandleCache.Get(ctx, cacheKey)
	if ok {
		return cachedFlow, nil
	}

	flow, err := s.store.GetFlowByHandle(ctx, handle, flowType)
	if err != nil || flow == nil {
		return flow, err
	}

	s.cacheFlow(ctx, flow)

	return flow, nil
}

// UpdateFlow updates an existing flow definition and refreshes the cache.
func (s *cacheBackedFlowStore) UpdateFlow(ctx context.Context, flowID string, flow *FlowDefinition) (
	*CompleteFlowDefinition, error) {
	updatedFlow, err := s.store.UpdateFlow(ctx, flowID, flow)
	if err != nil {
		return nil, err
	}
	s.cacheFlow(ctx, updatedFlow)

	return updatedFlow, nil
}

// DeleteFlow deletes a flow definition by its ID and invalidates the cache.
func (s *cacheBackedFlowStore) DeleteFlow(ctx context.Context, flowID string) error {
	cacheKey := cache.CacheKey{
		Key: flowID,
	}
	existingFlow, ok := s.flowByIDCache.Get(ctx, cacheKey)
	if !ok {
		var err error
		existingFlow, err = s.store.GetFlowByID(ctx, flowID)
		if err != nil {
			if errors.Is(err, errFlowNotFound) {
				return nil
			}
			return err
		}
	}
	if existingFlow == nil {
		return nil
	}

	if err := s.store.DeleteFlow(ctx, flowID); err != nil {
		return err
	}
	s.invalidateFlowCache(ctx, flowID)
	s.invalidateFlowCacheByHandle(ctx, existingFlow.Handle, existingFlow.FlowType)

	return nil
}

// IsFlowExistsByHandle checks if a flow exists with a given handle and flow type, using cache if available.
func (s *cacheBackedFlowStore) IsFlowExistsByHandle(ctx context.Context, handle string,
	flowType common.FlowType) (bool, error) {
	cacheKey := getFlowByHandleCacheKey(handle, flowType)
	cachedFlow, ok := s.flowByHandleCache.Get(ctx, cacheKey)
	if ok && cachedFlow != nil {
		return true, nil
	}

	return s.store.IsFlowExistsByHandle(ctx, handle, flowType)
}

// ListFlowVersions retrieves all versions of a flow.
// Note: Version operations are not cached as they are less frequently accessed.
func (s *cacheBackedFlowStore) ListFlowVersions(ctx context.Context, flowID string) ([]BasicFlowVersion, error) {
	return s.store.ListFlowVersions(ctx, flowID)
}

// GetFlowVersion retrieves a specific version of a flow.
// Note: Version operations are not cached as they are less frequently accessed.
func (s *cacheBackedFlowStore) GetFlowVersion(ctx context.Context, flowID string, version int) (*FlowVersion, error) {
	return s.store.GetFlowVersion(ctx, flowID, version)
}

// RestoreFlowVersion restores a flow to a specific version and invalidates the cache.
func (s *cacheBackedFlowStore) RestoreFlowVersion(ctx context.Context, flowID string, version int) (
	*CompleteFlowDefinition, error) {
	restoredFlow, err := s.store.RestoreFlowVersion(ctx, flowID, version)
	if err != nil {
		return nil, err
	}

	s.cacheFlow(ctx, restoredFlow)

	return restoredFlow, nil
}

// cacheFlow caches the flow definition by ID and by handle.
func (s *cacheBackedFlowStore) cacheFlow(ctx context.Context, flow *CompleteFlowDefinition) {
	if flow == nil {
		return
	}

	logger := s.logger.With(log.String("flowID", flow.ID))

	// Cache by ID
	if flow.ID != "" {
		cacheKey := cache.CacheKey{
			Key: flow.ID,
		}
		if err := s.flowByIDCache.Set(ctx, cacheKey, flow); err != nil {
			logger.Error("Failed to cache flow by ID", log.Error(err))
		} else {
			logger.Debug("Flow cached by ID")
		}
	}

	// Cache by handle and flowType
	if flow.Handle != "" && flow.FlowType != "" {
		handleCacheKey := getFlowByHandleCacheKey(flow.Handle, flow.FlowType)
		if err := s.flowByHandleCache.Set(ctx, handleCacheKey, flow); err != nil {
			logger.Error("Failed to cache flow by handle", log.String("handle", flow.Handle),
				log.String("flowType", string(flow.FlowType)), log.Error(err))
		} else {
			logger.Debug("Flow cached by handle",
				log.String("handle", flow.Handle), log.String("flowType", string(flow.FlowType)))
		}
	}
}

// invalidateFlowCache invalidates the flow cache for the given ID.
func (s *cacheBackedFlowStore) invalidateFlowCache(ctx context.Context, flowID string) {
	logger := s.logger.With(log.String("flowID", flowID))

	if flowID != "" {
		cacheKey := cache.CacheKey{
			Key: flowID,
		}
		if err := s.flowByIDCache.Delete(ctx, cacheKey); err != nil {
			logger.Error("Failed to invalidate flow cache by ID", log.Error(err))
		} else {
			logger.Debug("Flow cache invalidated by ID")
		}
	}
}

// invalidateFlowCacheByHandle invalidates the flow cache for the given handle and type.
func (s *cacheBackedFlowStore) invalidateFlowCacheByHandle(
	ctx context.Context, handle string, flowType common.FlowType) {
	if handle == "" || flowType == "" {
		return
	}

	cacheKey := getFlowByHandleCacheKey(handle, flowType)
	if err := s.flowByHandleCache.Delete(ctx, cacheKey); err != nil {
		s.logger.Error("Failed to invalidate flow cache by handle",
			log.String("handle", handle), log.String("flowType", string(flowType)), log.Error(err))
	}
}

// getFlowByHandleCacheKey generates a cache key for flow lookup by handle and type.
func getFlowByHandleCacheKey(handle string, flowType common.FlowType) cache.CacheKey {
	return cache.CacheKey{
		Key: handle + ":" + string(flowType),
	}
}
