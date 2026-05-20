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
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/cachemock"
)

const testAuthenticationHandleCacheKey = "test-handle:AUTHENTICATION"

type CacheBackedFlowStoreTestSuite struct {
	suite.Suite
	mockStore         *flowStoreInterfaceMock
	flowByIDCache     *cachemock.CacheInterfaceMock[*CompleteFlowDefinition]
	flowByHandleCache *cachemock.CacheInterfaceMock[*CompleteFlowDefinition]
	cachedStore       *cacheBackedFlowStore
	cacheData         map[string]*CompleteFlowDefinition
	handleCacheData   map[string]*CompleteFlowDefinition
}

func TestCacheBackedFlowStoreTestSuite(t *testing.T) {
	suite.Run(t, new(CacheBackedFlowStoreTestSuite))
}

func (s *CacheBackedFlowStoreTestSuite) SetupTest() {
	s.mockStore = newFlowStoreInterfaceMock(s.T())
	s.cacheData = make(map[string]*CompleteFlowDefinition)
	s.handleCacheData = make(map[string]*CompleteFlowDefinition)

	s.flowByIDCache = cachemock.NewCacheInterfaceMock[*CompleteFlowDefinition](s.T())
	s.flowByHandleCache = cachemock.NewCacheInterfaceMock[*CompleteFlowDefinition](s.T())

	s.setupCacheMock()

	s.cachedStore = &cacheBackedFlowStore{
		flowByIDCache:     s.flowByIDCache,
		flowByHandleCache: s.flowByHandleCache,
		store:             s.mockStore,
		logger:            log.GetLogger().With(log.String(log.LoggerKeyComponentName, "CacheBackedFlowStore")),
	}
}

func (s *CacheBackedFlowStoreTestSuite) setupCacheMock() {
	s.flowByIDCache.EXPECT().Set(mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, key cache.CacheKey, value *CompleteFlowDefinition) error {
			s.cacheData[key.Key] = value
			return nil
		}).Maybe()

	s.flowByIDCache.EXPECT().Get(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, key cache.CacheKey) (*CompleteFlowDefinition, bool) {
			if val, ok := s.cacheData[key.Key]; ok {
				return val, true
			}
			return nil, false
		}).Maybe()

	s.flowByIDCache.EXPECT().Delete(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, key cache.CacheKey) error {
			delete(s.cacheData, key.Key)
			return nil
		}).Maybe()

	s.flowByIDCache.EXPECT().Clear(mock.Anything).
		RunAndReturn(func(ctx context.Context) error {
			for k := range s.cacheData {
				delete(s.cacheData, k)
			}
			return nil
		}).Maybe()

	s.flowByIDCache.EXPECT().GetName().Return("FlowByIDCache").Maybe()
	s.flowByIDCache.EXPECT().CleanupExpired().Maybe()
	s.flowByIDCache.EXPECT().IsEnabled().Return(true).Maybe()

	// Setup mock for flowByHandleCache
	s.flowByHandleCache.EXPECT().Set(mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, key cache.CacheKey, value *CompleteFlowDefinition) error {
			s.handleCacheData[key.Key] = value
			return nil
		}).Maybe()

	s.flowByHandleCache.EXPECT().Get(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, key cache.CacheKey) (*CompleteFlowDefinition, bool) {
			if val, ok := s.handleCacheData[key.Key]; ok {
				return val, true
			}
			return nil, false
		}).Maybe()

	s.flowByHandleCache.EXPECT().Delete(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, key cache.CacheKey) error {
			delete(s.handleCacheData, key.Key)
			return nil
		}).Maybe()

	s.flowByHandleCache.EXPECT().GetName().Return("FlowByHandleCache").Maybe()
	s.flowByHandleCache.EXPECT().CleanupExpired().Maybe()
	s.flowByHandleCache.EXPECT().IsEnabled().Return(true).Maybe()
}

func (s *CacheBackedFlowStoreTestSuite) createTestFlow() *CompleteFlowDefinition {
	return &CompleteFlowDefinition{
		ID:            "flow-1",
		Handle:        "test-handle",
		Name:          "Test Flow",
		FlowType:      common.FlowTypeAuthentication,
		ActiveVersion: 1,
		Nodes: []NodeDefinition{
			{
				ID:   "node-1",
				Type: "basic-auth",
			},
		},
		CreatedAt: "2025-01-01T00:00:00Z",
		UpdatedAt: "2025-01-01T00:00:00Z",
	}
}

func (s *CacheBackedFlowStoreTestSuite) TestListFlows() {
	flows := []BasicFlowDefinition{
		{
			ID:            "flow-1",
			Handle:        "flow-1-handle",
			Name:          "Flow 1",
			FlowType:      common.FlowTypeAuthentication,
			ActiveVersion: 1,
		},
		{
			ID:            "flow-2",
			Handle:        "flow-2-handle",
			Name:          "Flow 2",
			FlowType:      common.FlowTypeRegistration,
			ActiveVersion: 1,
		},
	}

	s.mockStore.EXPECT().ListFlows(mock.Anything, 10, 0, "").Return(flows, 2, nil)

	result, count, err := s.cachedStore.ListFlows(context.Background(), 10, 0, "")

	s.NoError(err)
	s.Len(result, 2)
	s.Equal(2, count)
	s.Equal("flow-1", result[0].ID)
}

func (s *CacheBackedFlowStoreTestSuite) TestListFlowsError() {
	s.mockStore.EXPECT().ListFlows(mock.Anything, 10, 0, "").Return(nil, 0, errors.New("list error"))

	result, count, err := s.cachedStore.ListFlows(context.Background(), 10, 0, "")

	s.Error(err)
	s.Nil(result)
	s.Equal(0, count)
}

func (s *CacheBackedFlowStoreTestSuite) TestCreateFlowSuccess() {
	flowDef := &FlowDefinition{
		Handle:   "test-handle",
		Name:     "Test Flow",
		FlowType: common.FlowTypeAuthentication,
		Nodes: []NodeDefinition{
			{ID: "node-1", Type: "basic-auth"},
		},
	}

	expected := s.createTestFlow()
	s.mockStore.EXPECT().CreateFlow(mock.Anything, "flow-1", flowDef).Return(expected, nil)

	result, err := s.cachedStore.CreateFlow(context.Background(), "flow-1", flowDef)

	s.NoError(err)
	s.NotNil(result)
	s.Equal("flow-1", result.ID)

	cached, ok := s.cacheData["flow-1"]
	s.True(ok)
	s.Equal("flow-1", cached.ID)
}

func (s *CacheBackedFlowStoreTestSuite) TestCreateFlowError() {
	flowDef := &FlowDefinition{
		Handle:   "test-handle",
		Name:     "Test Flow",
		FlowType: common.FlowTypeAuthentication,
		Nodes:    []NodeDefinition{{ID: "node-1", Type: "basic-auth"}},
	}

	s.mockStore.EXPECT().CreateFlow(mock.Anything, "flow-1", flowDef).Return(nil, errors.New("create error"))

	result, err := s.cachedStore.CreateFlow(context.Background(), "flow-1", flowDef)

	s.Error(err)
	s.Nil(result)

	_, ok := s.cacheData["flow-1"]
	s.False(ok)
}

func (s *CacheBackedFlowStoreTestSuite) TestGetFlowByIDFromCache() {
	expected := s.createTestFlow()
	s.cacheData["flow-1"] = expected

	result, err := s.cachedStore.GetFlowByID(context.Background(), "flow-1")

	s.NoError(err)
	s.NotNil(result)
	s.Equal("flow-1", result.ID)
}

func (s *CacheBackedFlowStoreTestSuite) TestGetFlowByIDFromStoreAndCache() {
	expected := s.createTestFlow()
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, "flow-1").Return(expected, nil)

	result, err := s.cachedStore.GetFlowByID(context.Background(), "flow-1")

	s.NoError(err)
	s.NotNil(result)
	s.Equal("flow-1", result.ID)

	cached, ok := s.cacheData["flow-1"]
	s.True(ok)
	s.Equal("flow-1", cached.ID)
}

func (s *CacheBackedFlowStoreTestSuite) TestGetFlowByIDNotFound() {
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, "nonexistent").Return(nil, errFlowNotFound)

	result, err := s.cachedStore.GetFlowByID(context.Background(), "nonexistent")

	s.Error(err)
	s.Nil(result)

	// Verify nothing was cached for not-found
	_, ok := s.cacheData["nonexistent"]
	s.False(ok)
}

func (s *CacheBackedFlowStoreTestSuite) TestGetFlowByIDNilFlow() {
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, "flow-1").Return(nil, nil)

	result, err := s.cachedStore.GetFlowByID(context.Background(), "flow-1")

	s.NoError(err)
	s.Nil(result)

	_, ok := s.cacheData["flow-1"]
	s.False(ok)
}

// GetFlowByHandle Tests

func (s *CacheBackedFlowStoreTestSuite) TestGetFlowByHandleFromCache() {
	flow := s.createTestFlow()
	s.handleCacheData[testAuthenticationHandleCacheKey] = flow

	result, err := s.cachedStore.GetFlowByHandle(context.Background(), "test-handle", common.FlowTypeAuthentication)

	s.NoError(err)
	s.NotNil(result)
	s.Equal(flow.ID, result.ID)
	s.Equal(flow.Handle, result.Handle)
}

func (s *CacheBackedFlowStoreTestSuite) TestGetFlowByHandleFromStoreAndCache() {
	flow := s.createTestFlow()
	s.mockStore.EXPECT().GetFlowByHandle(mock.Anything, "test-handle", common.FlowTypeAuthentication).Return(flow, nil)

	result, err := s.cachedStore.GetFlowByHandle(context.Background(), "test-handle", common.FlowTypeAuthentication)

	s.NoError(err)
	s.NotNil(result)
	s.Equal(flow.ID, result.ID)
	s.Equal(flow.Handle, result.Handle)

	cached, ok := s.handleCacheData[testAuthenticationHandleCacheKey]
	s.True(ok)
	s.Equal(flow.ID, cached.ID)
}

func (s *CacheBackedFlowStoreTestSuite) TestGetFlowByHandleNotFound() {
	s.mockStore.EXPECT().GetFlowByHandle(mock.Anything, "non-existent", common.FlowTypeAuthentication).
		Return(nil, errFlowNotFound)

	result, err := s.cachedStore.GetFlowByHandle(context.Background(), "non-existent", common.FlowTypeAuthentication)

	s.Error(err)
	s.ErrorIs(err, errFlowNotFound)
	s.Nil(result)

	cacheKey := "non-existent:AUTHENTICATION"
	_, ok := s.handleCacheData[cacheKey]
	s.False(ok)
}

func (s *CacheBackedFlowStoreTestSuite) TestGetFlowByHandleNilFlow() {
	s.mockStore.EXPECT().GetFlowByHandle(mock.Anything, "test-handle", common.FlowTypeAuthentication).Return(nil, nil)

	result, err := s.cachedStore.GetFlowByHandle(context.Background(), "test-handle", common.FlowTypeAuthentication)

	s.NoError(err)
	s.Nil(result)

	_, ok := s.handleCacheData[testAuthenticationHandleCacheKey]
	s.False(ok)
}

func (s *CacheBackedFlowStoreTestSuite) TestUpdateFlowSuccess() {
	flowDef := &FlowDefinition{
		Handle:   "test-handle",
		Name:     "Updated Flow",
		FlowType: common.FlowTypeAuthentication,
		Nodes:    []NodeDefinition{{ID: "node-1", Type: "basic-auth"}},
	}

	updated := s.createTestFlow()
	updated.Name = "Updated Flow"
	updated.ActiveVersion = 2

	s.mockStore.EXPECT().UpdateFlow(mock.Anything, "flow-1", flowDef).Return(updated, nil)

	result, err := s.cachedStore.UpdateFlow(context.Background(), "flow-1", flowDef)

	s.NoError(err)
	s.NotNil(result)
	s.Equal("Updated Flow", result.Name)
	s.Equal(2, result.ActiveVersion)

	cached, ok := s.cacheData["flow-1"]
	s.True(ok)
	s.Equal("Updated Flow", cached.Name)
}

func (s *CacheBackedFlowStoreTestSuite) TestUpdateFlowError() {
	flowDef := &FlowDefinition{
		Handle:   "test-handle",
		Name:     "Updated Flow",
		FlowType: common.FlowTypeAuthentication,
		Nodes:    []NodeDefinition{{ID: "node-1", Type: "basic-auth"}},
	}

	s.mockStore.EXPECT().UpdateFlow(mock.Anything, "flow-1", flowDef).Return(nil, errors.New("update error"))

	result, err := s.cachedStore.UpdateFlow(context.Background(), "flow-1", flowDef)

	s.Error(err)
	s.Nil(result)
}

func (s *CacheBackedFlowStoreTestSuite) TestDeleteFlowFromCache() {
	flow := s.createTestFlow()
	s.cacheData["flow-1"] = flow

	s.mockStore.EXPECT().DeleteFlow(mock.Anything, "flow-1").Return(nil)

	err := s.cachedStore.DeleteFlow(context.Background(), "flow-1")

	s.NoError(err)

	_, ok := s.cacheData["flow-1"]
	s.False(ok)
}

func (s *CacheBackedFlowStoreTestSuite) TestDeleteFlowFromStore() {
	flow := s.createTestFlow()

	s.mockStore.EXPECT().GetFlowByID(mock.Anything, "flow-1").Return(flow, nil)
	s.mockStore.EXPECT().DeleteFlow(mock.Anything, "flow-1").Return(nil)

	err := s.cachedStore.DeleteFlow(context.Background(), "flow-1")

	s.NoError(err)

	_, ok := s.cacheData["flow-1"]
	s.False(ok)
}

func (s *CacheBackedFlowStoreTestSuite) TestDeleteFlowNotFound() {
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, "nonexistent").Return(nil, errFlowNotFound)

	err := s.cachedStore.DeleteFlow(context.Background(), "nonexistent")

	s.NoError(err)
}

func (s *CacheBackedFlowStoreTestSuite) TestDeleteFlowGetError() {
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, "flow-1").Return(nil, errors.New("get error"))

	err := s.cachedStore.DeleteFlow(context.Background(), "flow-1")

	s.Error(err)
	s.Contains(err.Error(), "get error")
}

func (s *CacheBackedFlowStoreTestSuite) TestDeleteFlowDeleteError() {
	flow := s.createTestFlow()
	s.cacheData["flow-1"] = flow

	s.mockStore.EXPECT().DeleteFlow(mock.Anything, "flow-1").Return(errors.New("delete error"))

	err := s.cachedStore.DeleteFlow(context.Background(), "flow-1")

	s.Error(err)
	s.Contains(err.Error(), "delete error")
}

func (s *CacheBackedFlowStoreTestSuite) TestDeleteFlowNilFromStore() {
	s.mockStore.EXPECT().GetFlowByID(mock.Anything, "flow-1").Return(nil, nil)

	err := s.cachedStore.DeleteFlow(context.Background(), "flow-1")

	s.NoError(err)
}

// IsFlowExistsByHandle Tests

func (s *CacheBackedFlowStoreTestSuite) TestIsFlowExistsByHandleFromCache() {
	// Add flow object to cache
	s.handleCacheData[testAuthenticationHandleCacheKey] = &CompleteFlowDefinition{
		ID:       "flow-id-1",
		Handle:   "test-handle",
		FlowType: common.FlowTypeAuthentication,
	}

	exists, err := s.cachedStore.IsFlowExistsByHandle(context.Background(),
		"test-handle", common.FlowTypeAuthentication)

	s.NoError(err)
	s.True(exists)
	// Verify store was not called since it's in cache
	s.mockStore.AssertNotCalled(s.T(), "IsFlowExistsByHandle", "test-handle", common.FlowTypeAuthentication)
}

func (s *CacheBackedFlowStoreTestSuite) TestIsFlowExistsByHandleFromStore() {
	// Not in cache, should query store
	s.mockStore.EXPECT().IsFlowExistsByHandle(mock.Anything, "new-handle",
		common.FlowTypeAuthentication).Return(true, nil)

	exists, err := s.cachedStore.IsFlowExistsByHandle(context.Background(), "new-handle", common.FlowTypeAuthentication)

	s.NoError(err)
	s.True(exists)
}

func (s *CacheBackedFlowStoreTestSuite) TestIsFlowExistsByHandleNotFound() {
	// Not in cache, store returns false
	s.mockStore.EXPECT().IsFlowExistsByHandle(mock.Anything, "non-existent",
		common.FlowTypeRegistration).Return(false, nil)

	exists, err := s.cachedStore.IsFlowExistsByHandle(context.Background(), "non-existent", common.FlowTypeRegistration)

	s.NoError(err)
	s.False(exists)
}

func (s *CacheBackedFlowStoreTestSuite) TestIsFlowExistsByHandleStoreError() {
	// Not in cache, store returns error
	s.mockStore.EXPECT().IsFlowExistsByHandle(mock.Anything, "error-handle", common.FlowTypeAuthentication).
		Return(false, errors.New("db connection error"))

	exists, err := s.cachedStore.IsFlowExistsByHandle(context.Background(),
		"error-handle", common.FlowTypeAuthentication)

	s.Error(err)
	s.Contains(err.Error(), "db connection error")
	s.False(exists)
}

func (s *CacheBackedFlowStoreTestSuite) TestIsFlowExistsByHandleCompositeKey() {
	// Test that different flow types with same handle are cached separately
	authFlow := &CompleteFlowDefinition{
		ID:       "flow-id-auth",
		Handle:   "common-handle",
		FlowType: common.FlowTypeAuthentication,
	}
	// Cache the auth flow
	s.handleCacheData["common-handle:AUTHENTICATION"] = authFlow
	// Registration not in cache, should query store
	s.mockStore.EXPECT().IsFlowExistsByHandle(mock.Anything, "common-handle",
		common.FlowTypeRegistration).Return(false, nil)

	// First call - authentication exists (from cache)
	exists1, err1 := s.cachedStore.IsFlowExistsByHandle(context.Background(),
		"common-handle", common.FlowTypeAuthentication)
	s.NoError(err1)
	s.True(exists1)

	// Second call - registration doesn't exist
	exists2, err2 := s.cachedStore.IsFlowExistsByHandle(context.Background(),
		"common-handle", common.FlowTypeRegistration)
	s.NoError(err2)
	s.False(exists2)

	// Verify both are cached with different keys
	cachedAuthFlow := s.handleCacheData["common-handle:AUTHENTICATION"]
	s.NotNil(cachedAuthFlow)
	s.Equal("flow-id-auth", cachedAuthFlow.ID)

	cachedRegFlow := s.handleCacheData["common-handle:REGISTRATION"]
	s.Nil(cachedRegFlow) // nil marks non-existence
}

func (s *CacheBackedFlowStoreTestSuite) TestListFlowVersions() {
	versions := []BasicFlowVersion{
		{Version: 3, CreatedAt: "2025-01-03T00:00:00Z", IsActive: true},
		{Version: 2, CreatedAt: "2025-01-02T00:00:00Z", IsActive: false},
		{Version: 1, CreatedAt: "2025-01-01T00:00:00Z", IsActive: false},
	}

	s.mockStore.EXPECT().ListFlowVersions(mock.Anything, "flow-1").Return(versions, nil)

	result, err := s.cachedStore.ListFlowVersions(context.Background(), "flow-1")

	s.NoError(err)
	s.Len(result, 3)
	s.Equal(3, result[0].Version)
	s.True(result[0].IsActive)
}

func (s *CacheBackedFlowStoreTestSuite) TestListFlowVersionsError() {
	s.mockStore.EXPECT().ListFlowVersions(mock.Anything, "flow-1").Return(nil, errors.New("list versions error"))

	result, err := s.cachedStore.ListFlowVersions(context.Background(), "flow-1")

	s.Error(err)
	s.Nil(result)
}

func (s *CacheBackedFlowStoreTestSuite) TestGetFlowVersion() {
	version := &FlowVersion{
		ID:        "flow-1",
		Handle:    "test-handle",
		Name:      "Test Flow",
		FlowType:  string(common.FlowTypeAuthentication),
		Version:   2,
		IsActive:  false,
		Nodes:     []NodeDefinition{{ID: "node-1", Type: "basic-auth"}},
		CreatedAt: "2025-01-02T00:00:00Z",
	}

	s.mockStore.EXPECT().GetFlowVersion(mock.Anything, "flow-1", 2).Return(version, nil)

	result, err := s.cachedStore.GetFlowVersion(context.Background(), "flow-1", 2)

	s.NoError(err)
	s.NotNil(result)
	s.Equal(2, result.Version)
	s.False(result.IsActive)
}

func (s *CacheBackedFlowStoreTestSuite) TestGetFlowVersionError() {
	s.mockStore.EXPECT().GetFlowVersion(mock.Anything, "flow-1", 999).Return(nil, errVersionNotFound)

	result, err := s.cachedStore.GetFlowVersion(context.Background(), "flow-1", 999)

	s.Error(err)
	s.Nil(result)
}

func (s *CacheBackedFlowStoreTestSuite) TestRestoreFlowVersionSuccess() {
	restored := s.createTestFlow()
	restored.ActiveVersion = 4

	s.mockStore.EXPECT().RestoreFlowVersion(mock.Anything, "flow-1", 1).Return(restored, nil)

	result, err := s.cachedStore.RestoreFlowVersion(context.Background(), "flow-1", 1)

	s.NoError(err)
	s.NotNil(result)
	s.Equal(4, result.ActiveVersion)

	cached, ok := s.cacheData["flow-1"]
	s.True(ok)
	s.Equal(4, cached.ActiveVersion)
}

func (s *CacheBackedFlowStoreTestSuite) TestRestoreFlowVersionError() {
	s.mockStore.EXPECT().RestoreFlowVersion(mock.Anything, "flow-1", 1).Return(nil, errors.New("restore error"))

	result, err := s.cachedStore.RestoreFlowVersion(context.Background(), "flow-1", 1)

	s.Error(err)
	s.Nil(result)

	_, ok := s.cacheData["flow-1"]
	s.False(ok)
}

func (s *CacheBackedFlowStoreTestSuite) TestCacheFlowNil() {
	s.cachedStore.cacheFlow(context.Background(), nil)

	s.Empty(s.cacheData)
}

func (s *CacheBackedFlowStoreTestSuite) TestCacheFlowEmptyID() {
	flow := s.createTestFlow()
	flow.ID = ""

	s.cachedStore.cacheFlow(context.Background(), flow)

	s.Empty(s.cacheData)
}

func (s *CacheBackedFlowStoreTestSuite) TestCacheFlowCacheError() {
	// Create a new cache mock just for this test to override the setupCacheMock expectations
	errorCache := cachemock.NewCacheInterfaceMock[*CompleteFlowDefinition](s.T())
	errorCache.EXPECT().Set(mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("cache error")).Once()
	errorCache.EXPECT().GetName().Return("FlowByIDCache").Maybe()

	// Temporarily replace the cache
	originalCache := s.cachedStore.flowByIDCache
	s.cachedStore.flowByIDCache = errorCache

	flow := s.createTestFlow()
	s.cachedStore.cacheFlow(context.Background(), flow)

	// Restore original cache
	s.cachedStore.flowByIDCache = originalCache

	// Verify the flow was not cached in the original cache data
	_, found := s.cacheData[flow.ID]
	s.False(found)
}

func (s *CacheBackedFlowStoreTestSuite) TestInvalidateFlowCacheEmptyID() {
	s.cachedStore.invalidateFlowCache(context.Background(), "")

	s.Empty(s.cacheData)
}

func (s *CacheBackedFlowStoreTestSuite) TestInvalidateFlowCacheError() {
	flow := s.createTestFlow()
	s.cacheData["flow-1"] = flow

	// Create a new cache mock just for this test to override the setupCacheMock expectations
	errorCache := cachemock.NewCacheInterfaceMock[*CompleteFlowDefinition](s.T())
	errorCache.EXPECT().Delete(mock.Anything, mock.Anything).
		Return(errors.New("cache error")).Once()
	errorCache.EXPECT().GetName().Return("FlowByIDCache").Maybe()

	// Temporarily replace the cache
	originalCache := s.cachedStore.flowByIDCache
	s.cachedStore.flowByIDCache = errorCache

	s.cachedStore.invalidateFlowCache(context.Background(), "flow-1")

	// Restore original cache
	s.cachedStore.flowByIDCache = originalCache

	// The flow should still be in the original cache data since we used error cache
	val, found := s.cacheData[flow.ID]
	s.True(found)
	s.Equal(flow, val)
}
