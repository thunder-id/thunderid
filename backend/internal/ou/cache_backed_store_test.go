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

package ou

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/filter"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/cachemock"
)

type CacheBackedOUStoreTestSuite struct {
	suite.Suite
	mockStore             *organizationUnitStoreInterfaceMock
	ouByIDCache           *cachemock.CacheInterfaceMock[*OrganizationUnit]
	ouByHandleParentCache *cachemock.CacheInterfaceMock[*OrganizationUnit]
	cachedStore           *cacheBackedOUStore
	ouByIDData            map[string]*OrganizationUnit
	ouByHandleParentData  map[string]*OrganizationUnit
}

func TestCacheBackedOUStoreTestSuite(t *testing.T) {
	suite.Run(t, new(CacheBackedOUStoreTestSuite))
}

func (s *CacheBackedOUStoreTestSuite) SetupTest() {
	s.mockStore = newOrganizationUnitStoreInterfaceMock(s.T())
	s.ouByIDData = make(map[string]*OrganizationUnit)
	s.ouByHandleParentData = make(map[string]*OrganizationUnit)

	s.ouByIDCache = cachemock.NewCacheInterfaceMock[*OrganizationUnit](s.T())
	s.ouByHandleParentCache = cachemock.NewCacheInterfaceMock[*OrganizationUnit](s.T())

	setupOUCacheMock(s.ouByIDCache, s.ouByIDData)
	setupOUCacheMock(s.ouByHandleParentCache, s.ouByHandleParentData)

	s.ouByIDCache.EXPECT().IsEnabled().Return(true).Maybe()
	s.ouByHandleParentCache.EXPECT().IsEnabled().Return(true).Maybe()

	s.cachedStore = &cacheBackedOUStore{
		ouByIDCache:           s.ouByIDCache,
		ouByHandleParentCache: s.ouByHandleParentCache,
		store:                 s.mockStore,
		logger: log.GetLogger().With(
			log.String(log.LoggerKeyComponentName, "CacheBackedOUStore")),
	}
}

func setupOUCacheMock[T any](
	mockCache *cachemock.CacheInterfaceMock[T],
	data map[string]T,
) {
	mockCache.EXPECT().Set(mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, key cache.CacheKey, value T) error {
			data[key.Key] = value
			return nil
		}).Maybe()

	mockCache.EXPECT().Get(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, key cache.CacheKey) (T, bool) {
			if val, ok := data[key.Key]; ok {
				return val, true
			}
			var zero T
			return zero, false
		}).Maybe()

	mockCache.EXPECT().Delete(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, key cache.CacheKey) error {
			delete(data, key.Key)
			return nil
		}).Maybe()

	mockCache.EXPECT().Clear(mock.Anything).
		RunAndReturn(func(ctx context.Context) error {
			for k := range data {
				delete(data, k)
			}
			return nil
		}).Maybe()

	mockCache.EXPECT().GetName().Return("mockCache").Maybe()
	mockCache.EXPECT().CleanupExpired().Maybe()
}

func (s *CacheBackedOUStoreTestSuite) makeOU(handle string, parent *string) OrganizationUnit {
	return OrganizationUnit{
		ID:     cacheTestOUID,
		Handle: handle,
		Name:   "Test OU " + handle,
		Parent: parent,
	}
}

const cacheTestOUID = "ou-1"

// --- GetOrganizationUnit tests ---

func (s *CacheBackedOUStoreTestSuite) TestGetOrganizationUnit_CacheHit() {
	ou := s.makeOU("marketing", nil)
	s.ouByIDData[ou.ID] = &ou

	result, err := s.cachedStore.GetOrganizationUnit(context.Background(), ou.ID)
	s.Nil(err)
	s.Equal(ou.ID, result.ID)
	s.mockStore.AssertNotCalled(s.T(), "GetOrganizationUnit")
}

func (s *CacheBackedOUStoreTestSuite) TestGetOrganizationUnit_CacheMiss() {
	ou := s.makeOU("marketing", nil)
	s.mockStore.On("GetOrganizationUnit", mock.Anything, ou.ID).Return(ou, nil).Once()

	result, err := s.cachedStore.GetOrganizationUnit(context.Background(), ou.ID)
	s.Nil(err)
	s.Equal(ou.ID, result.ID)
	s.mockStore.AssertExpectations(s.T())

	cached, ok := s.ouByIDCache.Get(context.Background(), cache.CacheKey{Key: ou.ID})
	s.True(ok)
	s.Equal(ou.ID, cached.ID)
}

func (s *CacheBackedOUStoreTestSuite) TestGetOrganizationUnit_StoreError() {
	storeErr := errors.New("db error")
	s.mockStore.On("GetOrganizationUnit", mock.Anything, "bad-id").
		Return(OrganizationUnit{}, storeErr).Once()

	_, err := s.cachedStore.GetOrganizationUnit(context.Background(), "bad-id")
	s.Equal(storeErr, err)

	_, ok := s.ouByIDCache.Get(context.Background(), cache.CacheKey{Key: "bad-id"})
	s.False(ok)
}

// --- GetOrganizationUnitByHandle tests ---

func (s *CacheBackedOUStoreTestSuite) TestGetOrganizationUnitByHandle_CacheHit_RootOU() {
	ou := s.makeOU("marketing", nil)
	key := handleParentCacheKey("marketing", nil)
	s.ouByHandleParentData[key] = &ou

	result, err := s.cachedStore.GetOrganizationUnitByHandle(
		context.Background(), "marketing", nil)
	s.Nil(err)
	s.Equal(ou.ID, result.ID)
	s.mockStore.AssertNotCalled(s.T(), "GetOrganizationUnitByHandle")
}

func (s *CacheBackedOUStoreTestSuite) TestGetOrganizationUnitByHandle_CacheHit_ChildOU() {
	parentID := testParentOUID
	ou := s.makeOU("sales", &parentID)
	key := handleParentCacheKey("sales", &parentID)
	s.ouByHandleParentData[key] = &ou

	result, err := s.cachedStore.GetOrganizationUnitByHandle(
		context.Background(), "sales", &parentID)
	s.Nil(err)
	s.Equal(ou.ID, result.ID)
	s.mockStore.AssertNotCalled(s.T(), "GetOrganizationUnitByHandle")
}

func (s *CacheBackedOUStoreTestSuite) TestGetOrganizationUnitByHandle_CacheMiss() {
	ou := s.makeOU("marketing", nil)
	s.mockStore.On("GetOrganizationUnitByHandle", mock.Anything, "marketing",
		(*string)(nil)).Return(ou, nil).Once()

	result, err := s.cachedStore.GetOrganizationUnitByHandle(
		context.Background(), "marketing", nil)
	s.Nil(err)
	s.Equal(ou.ID, result.ID)
	s.mockStore.AssertExpectations(s.T())

	// Should be cached in both caches.
	cachedByID, ok := s.ouByIDCache.Get(context.Background(), cache.CacheKey{Key: ou.ID})
	s.True(ok)
	s.Equal(ou.ID, cachedByID.ID)

	key := handleParentCacheKey("marketing", nil)
	cachedByHandle, ok := s.ouByHandleParentCache.Get(context.Background(),
		cache.CacheKey{Key: key})
	s.True(ok)
	s.Equal(ou.ID, cachedByHandle.ID)
}

func (s *CacheBackedOUStoreTestSuite) TestGetOrganizationUnitByHandle_StoreError() {
	storeErr := errors.New("db error")
	s.mockStore.On("GetOrganizationUnitByHandle", mock.Anything, "bad-handle",
		(*string)(nil)).Return(OrganizationUnit{}, storeErr).Once()

	_, err := s.cachedStore.GetOrganizationUnitByHandle(
		context.Background(), "bad-handle", nil)
	s.Equal(storeErr, err)

	key := handleParentCacheKey("bad-handle", nil)
	_, ok := s.ouByHandleParentCache.Get(context.Background(), cache.CacheKey{Key: key})
	s.False(ok)
}

// --- CreateOrganizationUnit tests ---

func (s *CacheBackedOUStoreTestSuite) TestCreateOrganizationUnit_CachesBothKeys() {
	ou := s.makeOU("marketing", nil)
	s.mockStore.On("CreateOrganizationUnit", mock.Anything, ou).Return(nil).Once()

	err := s.cachedStore.CreateOrganizationUnit(context.Background(), ou)
	s.Nil(err)
	s.mockStore.AssertExpectations(s.T())

	cachedByID, ok := s.ouByIDCache.Get(context.Background(), cache.CacheKey{Key: ou.ID})
	s.True(ok)
	s.Equal(ou.ID, cachedByID.ID)

	key := handleParentCacheKey("marketing", nil)
	cachedByHandle, ok := s.ouByHandleParentCache.Get(context.Background(),
		cache.CacheKey{Key: key})
	s.True(ok)
	s.Equal(ou.ID, cachedByHandle.ID)
}

func (s *CacheBackedOUStoreTestSuite) TestCreateOrganizationUnit_StoreError_DoesNotCache() {
	ou := s.makeOU("marketing", nil)
	storeErr := errors.New("create error")
	s.mockStore.On("CreateOrganizationUnit", mock.Anything, ou).Return(storeErr).Once()

	err := s.cachedStore.CreateOrganizationUnit(context.Background(), ou)
	s.Equal(storeErr, err)

	_, ok := s.ouByIDCache.Get(context.Background(), cache.CacheKey{Key: ou.ID})
	s.False(ok)
}

// --- UpdateOrganizationUnit tests ---

func (s *CacheBackedOUStoreTestSuite) TestUpdateOrganizationUnit_InvalidatesAndRecaches() {
	ou := s.makeOU("marketing", nil)
	s.ouByIDData[ou.ID] = &ou
	key := handleParentCacheKey("marketing", nil)
	s.ouByHandleParentData[key] = &ou

	s.mockStore.On("UpdateOrganizationUnit", mock.Anything, ou).Return(nil).Once()

	err := s.cachedStore.UpdateOrganizationUnit(context.Background(), ou)
	s.Nil(err)
	s.mockStore.AssertExpectations(s.T())

	// Updated OU must be present in the by-ID cache.
	cached, ok := s.ouByIDCache.Get(context.Background(), cache.CacheKey{Key: ou.ID})
	s.True(ok)
	s.Equal(ou.ID, cached.ID)

	// Updated OU must be present in the handle+parent cache.
	cachedByHandle, ok := s.ouByHandleParentCache.Get(context.Background(),
		cache.CacheKey{Key: key})
	s.True(ok)
	s.Equal(ou.ID, cachedByHandle.ID)
}

func (s *CacheBackedOUStoreTestSuite) TestUpdateOrganizationUnit_HandleChanged_InvalidatesOldKey() {
	oldOU := s.makeOU("old-handle", nil)
	s.ouByIDData[oldOU.ID] = &oldOU
	oldKey := handleParentCacheKey("old-handle", nil)
	s.ouByHandleParentData[oldKey] = &oldOU

	newOU := s.makeOU("new-handle", nil)
	s.mockStore.On("UpdateOrganizationUnit", mock.Anything, newOU).Return(nil).Once()

	err := s.cachedStore.UpdateOrganizationUnit(context.Background(), newOU)
	s.Nil(err)

	// Old handle key should be invalidated.
	_, ok := s.ouByHandleParentCache.Get(context.Background(),
		cache.CacheKey{Key: oldKey})
	s.False(ok)

	// New handle key should be cached.
	newKey := handleParentCacheKey("new-handle", nil)
	cached, ok := s.ouByHandleParentCache.Get(context.Background(),
		cache.CacheKey{Key: newKey})
	s.True(ok)
	s.Equal(cacheTestOUID, cached.ID)
}

func (s *CacheBackedOUStoreTestSuite) TestUpdateOrganizationUnit_StoreError_DoesNotInvalidate() {
	ou := s.makeOU("marketing", nil)
	s.ouByIDData[ou.ID] = &ou
	key := handleParentCacheKey("marketing", nil)
	s.ouByHandleParentData[key] = &ou

	storeErr := errors.New("update error")
	s.mockStore.On("UpdateOrganizationUnit", mock.Anything, ou).Return(storeErr).Once()

	err := s.cachedStore.UpdateOrganizationUnit(context.Background(), ou)
	s.Equal(storeErr, err)

	// Both caches should remain intact on store error.
	_, ok := s.ouByIDCache.Get(context.Background(), cache.CacheKey{Key: ou.ID})
	s.True(ok)

	_, ok = s.ouByHandleParentCache.Get(context.Background(), cache.CacheKey{Key: key})
	s.True(ok)
}

// --- DeleteOrganizationUnit tests ---

func (s *CacheBackedOUStoreTestSuite) TestDeleteOrganizationUnit_InvalidatesBothCaches() {
	ou := s.makeOU("marketing", nil)
	s.ouByIDData[ou.ID] = &ou
	key := handleParentCacheKey("marketing", nil)
	s.ouByHandleParentData[key] = &ou

	s.mockStore.On("DeleteOrganizationUnit", mock.Anything, ou.ID).Return(nil).Once()

	err := s.cachedStore.DeleteOrganizationUnit(context.Background(), ou.ID)
	s.Nil(err)
	s.mockStore.AssertExpectations(s.T())

	_, ok := s.ouByIDCache.Get(context.Background(), cache.CacheKey{Key: ou.ID})
	s.False(ok)

	_, ok = s.ouByHandleParentCache.Get(context.Background(), cache.CacheKey{Key: key})
	s.False(ok)
}

func (s *CacheBackedOUStoreTestSuite) TestDeleteOrganizationUnit_CacheMiss_FallsBackToStore() {
	ou := s.makeOU("marketing", nil)
	// OU is NOT in the by-ID cache — invalidateHandleParentCache must fall back to the store.
	key := handleParentCacheKey("marketing", nil)
	s.ouByHandleParentData[key] = &ou

	s.mockStore.On("GetOrganizationUnit", mock.Anything, ou.ID).Return(ou, nil).Once()
	s.mockStore.On("DeleteOrganizationUnit", mock.Anything, ou.ID).Return(nil).Once()

	err := s.cachedStore.DeleteOrganizationUnit(context.Background(), ou.ID)
	s.Nil(err)
	s.mockStore.AssertExpectations(s.T())

	_, ok := s.ouByHandleParentCache.Get(context.Background(), cache.CacheKey{Key: key})
	s.False(ok)
}

func (s *CacheBackedOUStoreTestSuite) TestDeleteOrganizationUnit_StoreError() {
	ou := s.makeOU("marketing", nil)
	s.ouByIDData[ou.ID] = &ou
	key := handleParentCacheKey("marketing", nil)
	s.ouByHandleParentData[key] = &ou

	storeErr := errors.New("delete error")
	s.mockStore.On("DeleteOrganizationUnit", mock.Anything, ou.ID).Return(storeErr).Once()

	err := s.cachedStore.DeleteOrganizationUnit(context.Background(), ou.ID)
	s.Equal(storeErr, err)

	// Both caches should remain intact on store error.
	_, ok := s.ouByIDCache.Get(context.Background(), cache.CacheKey{Key: ou.ID})
	s.True(ok)

	_, ok = s.ouByHandleParentCache.Get(context.Background(), cache.CacheKey{Key: key})
	s.True(ok)
}

// --- handleParentCacheKey tests ---

func (s *CacheBackedOUStoreTestSuite) TestHandleParentCacheKey_NilParent() {
	key := handleParentCacheKey("marketing", nil)
	s.Equal("marketing:", key)
}

func (s *CacheBackedOUStoreTestSuite) TestHandleParentCacheKey_WithParent() {
	parentID := testParentOUID
	key := handleParentCacheKey("sales", &parentID)
	s.Equal("sales:parent-1", key)
}

func (s *CacheBackedOUStoreTestSuite) TestHandleParentCacheKey_DifferentParents_DifferentKeys() {
	parent1 := testParentOUID
	parent2 := "parent-2"
	key1 := handleParentCacheKey("sales", &parent1)
	key2 := handleParentCacheKey("sales", &parent2)
	s.NotEqual(key1, key2)
}

// --- Pass-through method tests ---

func (s *CacheBackedOUStoreTestSuite) TestPassThroughMethods() {
	ctx := context.Background()

	s.mockStore.On("GetOrganizationUnitListCount", mock.Anything,
		(*filter.FilterGroup)(nil)).Return(5, nil).Once()
	count, err := s.cachedStore.GetOrganizationUnitListCount(ctx, nil)
	s.Nil(err)
	s.Equal(5, count)

	s.mockStore.On("GetOrganizationUnitList", mock.Anything, 10, 0,
		(*filter.FilterGroup)(nil)).Return([]OrganizationUnitBasic{}, nil).Once()
	list, err := s.cachedStore.GetOrganizationUnitList(ctx, 10, 0, nil)
	s.Nil(err)
	s.Empty(list)

	s.mockStore.On("GetOrganizationUnitsByIDs", mock.Anything,
		[]string{"id-1"}).Return([]OrganizationUnitBasic{}, nil).Once()
	byIDs, err := s.cachedStore.GetOrganizationUnitsByIDs(ctx, []string{"id-1"})
	s.Nil(err)
	s.Empty(byIDs)

	s.mockStore.On("GetOrganizationUnitByPath", mock.Anything,
		[]string{"root", "child"}).Return(OrganizationUnit{ID: "path-ou"}, nil).Once()
	byPath, err := s.cachedStore.GetOrganizationUnitByPath(ctx, []string{"root", "child"})
	s.Nil(err)
	s.Equal("path-ou", byPath.ID)

	s.mockStore.On("IsOrganizationUnitDeclarative", mock.Anything, "ou-1").Return(true).Once()
	declarative := s.cachedStore.IsOrganizationUnitDeclarative(ctx, "ou-1")
	s.True(declarative)

	s.mockStore.On("CheckOrganizationUnitNameConflict", mock.Anything, "name",
		(*string)(nil)).Return(false, nil).Once()
	conflict, err := s.cachedStore.CheckOrganizationUnitNameConflict(ctx, "name", nil)
	s.Nil(err)
	s.False(conflict)

	s.mockStore.On("CheckOrganizationUnitHandleConflict", mock.Anything, "handle",
		(*string)(nil)).Return(false, nil).Once()
	conflict, err = s.cachedStore.CheckOrganizationUnitHandleConflict(ctx, "handle", nil)
	s.Nil(err)
	s.False(conflict)

	s.mockStore.On("GetOrganizationUnitChildrenCount", mock.Anything, "ou-1",
		(*filter.FilterGroup)(nil)).Return(3, nil).Once()
	childCount, err := s.cachedStore.GetOrganizationUnitChildrenCount(ctx, "ou-1", nil)
	s.Nil(err)
	s.Equal(3, childCount)

	s.mockStore.On("GetOrganizationUnitChildrenList", mock.Anything, "ou-1", 10, 0,
		(*filter.FilterGroup)(nil)).Return([]OrganizationUnitBasic{}, nil).Once()
	children, err := s.cachedStore.GetOrganizationUnitChildrenList(ctx, "ou-1", 10, 0, nil)
	s.Nil(err)
	s.Empty(children)

	s.mockStore.AssertExpectations(s.T())
}

// --- IsOrganizationUnitExists cache optimization tests ---

func (s *CacheBackedOUStoreTestSuite) TestIsOrganizationUnitExists_CacheHit() {
	ou := s.makeOU("marketing", nil)
	s.ouByIDData[ou.ID] = &ou

	exists, err := s.cachedStore.IsOrganizationUnitExists(context.Background(), ou.ID)
	s.Nil(err)
	s.True(exists)
	s.mockStore.AssertNotCalled(s.T(), "IsOrganizationUnitExists")
}

func (s *CacheBackedOUStoreTestSuite) TestIsOrganizationUnitExists_CacheMiss_DelegatesToStore() {
	s.mockStore.On("IsOrganizationUnitExists", mock.Anything, "ou-2").
		Return(true, nil).Once()

	exists, err := s.cachedStore.IsOrganizationUnitExists(context.Background(), "ou-2")
	s.Nil(err)
	s.True(exists)
	s.mockStore.AssertExpectations(s.T())
}

// --- Cache error handling tests ---

func (s *CacheBackedOUStoreTestSuite) TestCacheSetError_DoesNotPropagateOnGetEntity() {
	failingIDCache := cachemock.NewCacheInterfaceMock[*OrganizationUnit](s.T())
	failingIDCache.EXPECT().Get(mock.Anything, mock.Anything).
		Return(nil, false).Once()
	failingIDCache.EXPECT().Set(mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("cache set error")).Once()
	failingIDCache.EXPECT().Delete(mock.Anything, mock.Anything).Return(nil).Maybe()
	failingIDCache.EXPECT().GetName().Return("failingIDCache").Maybe()
	failingIDCache.EXPECT().CleanupExpired().Maybe()
	failingIDCache.EXPECT().IsEnabled().Return(true).Maybe()

	store := &cacheBackedOUStore{
		ouByIDCache:           failingIDCache,
		ouByHandleParentCache: s.ouByHandleParentCache,
		store:                 s.mockStore,
		logger: log.GetLogger().With(
			log.String(log.LoggerKeyComponentName, "CacheBackedOUStore")),
	}

	ou := s.makeOU("marketing", nil)
	s.mockStore.On("GetOrganizationUnit", mock.Anything, ou.ID).Return(ou, nil).Once()

	result, err := store.GetOrganizationUnit(context.Background(), ou.ID)
	s.Nil(err)
	s.Equal(ou.ID, result.ID)
}

func (s *CacheBackedOUStoreTestSuite) TestCacheSetError_DoesNotPropagateOnGetByHandle() {
	failingHandleCache := cachemock.NewCacheInterfaceMock[*OrganizationUnit](s.T())
	failingHandleCache.EXPECT().Get(mock.Anything, mock.Anything).
		Return(nil, false).Once()
	failingHandleCache.EXPECT().Set(mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("cache set error")).Once()
	failingHandleCache.EXPECT().Delete(mock.Anything, mock.Anything).Return(nil).Maybe()
	failingHandleCache.EXPECT().GetName().Return("failingHandleCache").Maybe()
	failingHandleCache.EXPECT().CleanupExpired().Maybe()
	failingHandleCache.EXPECT().IsEnabled().Return(true).Maybe()

	store := &cacheBackedOUStore{
		ouByIDCache:           s.ouByIDCache,
		ouByHandleParentCache: failingHandleCache,
		store:                 s.mockStore,
		logger: log.GetLogger().With(
			log.String(log.LoggerKeyComponentName, "CacheBackedOUStore")),
	}

	ou := s.makeOU("marketing", nil)
	s.mockStore.On("GetOrganizationUnitByHandle", mock.Anything, "marketing",
		(*string)(nil)).Return(ou, nil).Once()

	result, err := store.GetOrganizationUnitByHandle(context.Background(), "marketing", nil)
	s.Nil(err)
	s.Equal(ou.ID, result.ID)
}

func (s *CacheBackedOUStoreTestSuite) TestCacheDeleteError_DoesNotPropagateOnDelete() {
	failingIDCache := cachemock.NewCacheInterfaceMock[*OrganizationUnit](s.T())
	failingIDCache.EXPECT().Get(mock.Anything, mock.Anything).Return(nil, false).Maybe()
	failingIDCache.EXPECT().Set(mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	failingIDCache.EXPECT().Delete(mock.Anything, mock.Anything).
		Return(errors.New("cache delete error")).Once()
	failingIDCache.EXPECT().GetName().Return("failingIDCache").Maybe()
	failingIDCache.EXPECT().CleanupExpired().Maybe()
	failingIDCache.EXPECT().IsEnabled().Return(true).Maybe()

	store := &cacheBackedOUStore{
		ouByIDCache:           failingIDCache,
		ouByHandleParentCache: s.ouByHandleParentCache,
		store:                 s.mockStore,
		logger: log.GetLogger().With(
			log.String(log.LoggerKeyComponentName, "CacheBackedOUStore")),
	}

	ou := s.makeOU("marketing", nil)
	s.mockStore.On("GetOrganizationUnit", mock.Anything, ou.ID).Return(ou, nil).Once()
	s.mockStore.On("DeleteOrganizationUnit", mock.Anything, ou.ID).Return(nil).Once()

	// Should not propagate the cache delete error.
	err := store.DeleteOrganizationUnit(context.Background(), ou.ID)
	s.Nil(err)
}

// --- invalidateHandleParentCache store error test ---

func (s *CacheBackedOUStoreTestSuite) TestDeleteOrganizationUnit_HandleParentInvalidation_StoreError() {
	// OU is NOT in cache, and store.GetOrganizationUnit fails — handle+parent cache should
	// remain (graceful degradation), but DeleteOrganizationUnit should still succeed.
	s.mockStore.On("GetOrganizationUnit", mock.Anything, cacheTestOUID).
		Return(OrganizationUnit{}, errors.New("fetch error")).Once()
	s.mockStore.On("DeleteOrganizationUnit", mock.Anything, cacheTestOUID).
		Return(nil).Once()

	err := s.cachedStore.DeleteOrganizationUnit(context.Background(), cacheTestOUID)
	s.Nil(err)
	s.mockStore.AssertExpectations(s.T())
}

// --- newCacheBackedOUStore constructor test ---

func (s *CacheBackedOUStoreTestSuite) TestNewCacheBackedOUStore_ReturnsInterface() {
	store := newCacheBackedOUStore(s.mockStore, s.ouByIDCache, s.ouByHandleParentCache)
	s.NotNil(store)

	// Verify it implements the interface.
	var _ organizationUnitStoreInterface = store
}

// --- wrapWithCache tests ---

func (s *CacheBackedOUStoreTestSuite) TestWrapWithCache_NilCacheManager() {
	result := wrapWithCache(s.mockStore, nil)
	// Should return the original store unwrapped.
	s.Equal(s.mockStore, result)
}

func (s *CacheBackedOUStoreTestSuite) TestWrapWithCache_WithCacheManager() {
	config.ResetServerRuntime()
	defer config.ResetServerRuntime()
	tmpDir := s.T().TempDir()
	err := config.InitializeServerRuntime(tmpDir, &config.Config{})
	s.Require().NoError(err)

	cm := cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment")
	defer cm.Close()

	result := wrapWithCache(s.mockStore, cm)
	// Should return a wrapped store (different from original).
	s.NotEqual(s.mockStore, result)

	_, ok := result.(*cacheBackedOUStore)
	s.True(ok)
}
