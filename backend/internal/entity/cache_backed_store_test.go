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

package entity

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/cachemock"
)

// CacheBackedEntityStoreTestSuite tests the cacheBackedEntityStore.
type CacheBackedEntityStoreTestSuite struct {
	suite.Suite
	mockStore       *entityStoreInterfaceMock
	entityByIDCache *cachemock.CacheInterfaceMock[*Entity]
	cachedStore     *cacheBackedEntityStore
	entityByIDData  map[string]*Entity
}

func TestCacheBackedEntityStoreTestSuite(t *testing.T) {
	suite.Run(t, new(CacheBackedEntityStoreTestSuite))
}

func (s *CacheBackedEntityStoreTestSuite) SetupTest() {
	s.mockStore = newEntityStoreInterfaceMock(s.T())
	s.entityByIDData = make(map[string]*Entity)

	s.entityByIDCache = cachemock.NewCacheInterfaceMock[*Entity](s.T())

	setupEntityCacheMock(s.entityByIDCache, s.entityByIDData)

	s.entityByIDCache.EXPECT().IsEnabled().Return(true).Maybe()

	s.cachedStore = &cacheBackedEntityStore{
		entityByIDCache: s.entityByIDCache,
		store:           s.mockStore,
		logger: log.GetLogger().With(
			log.String(log.LoggerKeyComponentName, "CacheBackedEntityStore")),
	}
}

func setupEntityCacheMock[T any](
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

func (s *CacheBackedEntityStoreTestSuite) makeEntity(id, clientID string) Entity {
	sysAttrs := map[string]interface{}{"clientId": clientID}
	sysAttrsJSON, _ := json.Marshal(sysAttrs)
	return Entity{
		ID:               id,
		Category:         EntityCategoryApp,
		Type:             "application",
		State:            EntityStateActive,
		SystemAttributes: json.RawMessage(sysAttrsJSON),
	}
}

const testEntityID = "entity-1"

// GetEntity tests

func (s *CacheBackedEntityStoreTestSuite) TestGetEntity_CacheHit() {
	entity := s.makeEntity(testEntityID, "client-1")
	s.entityByIDData[entity.ID] = &entity

	result, err := s.cachedStore.GetEntity(context.Background(), entity.ID)
	s.Nil(err)
	s.Equal(entity.ID, result.ID)
	s.mockStore.AssertNotCalled(s.T(), "GetEntity")
}

func (s *CacheBackedEntityStoreTestSuite) TestGetEntity_CacheMiss() {
	entity := s.makeEntity(testEntityID, "client-1")
	s.mockStore.On("GetEntity", mock.Anything, entity.ID).Return(entity, nil).Once()

	result, err := s.cachedStore.GetEntity(context.Background(), entity.ID)
	s.Nil(err)
	s.Equal(entity.ID, result.ID)
	s.mockStore.AssertExpectations(s.T())

	cached, ok := s.entityByIDCache.Get(context.Background(), cache.CacheKey{Key: entity.ID})
	s.True(ok)
	s.Equal(entity.ID, cached.ID)
}

func (s *CacheBackedEntityStoreTestSuite) TestGetEntity_StoreError() {
	storeErr := errors.New("db error")
	s.mockStore.On("GetEntity", mock.Anything, "bad-id").Return(Entity{}, storeErr).Once()

	_, err := s.cachedStore.GetEntity(context.Background(), "bad-id")
	s.Equal(storeErr, err)

	_, ok := s.entityByIDCache.Get(context.Background(), cache.CacheKey{Key: "bad-id"})
	s.False(ok)
}

// IdentifyEntity tests

func (s *CacheBackedEntityStoreTestSuite) TestIdentifyEntity_CallsStore() {
	entityID := testEntityID
	filters := map[string]interface{}{"clientId": "client-1"}
	s.mockStore.On("IdentifyEntity", mock.Anything, filters).Return(&entityID, nil).Once()

	result, err := s.cachedStore.IdentifyEntity(context.Background(), filters)
	s.Nil(err)
	s.Equal(entityID, *result)
	s.mockStore.AssertExpectations(s.T())
}

func (s *CacheBackedEntityStoreTestSuite) TestIdentifyEntity_StoreError() {
	filters := map[string]interface{}{"clientId": "bad-client"}
	storeErr := errors.New("not found")
	s.mockStore.On("IdentifyEntity", mock.Anything, filters).Return(nil, storeErr).Once()

	result, err := s.cachedStore.IdentifyEntity(context.Background(), filters)
	s.Equal(storeErr, err)
	s.Nil(result)
	s.mockStore.AssertExpectations(s.T())
}

// UpdateSystemAttributes tests

func (s *CacheBackedEntityStoreTestSuite) TestUpdateSystemAttributes_InvalidatesEntityCache() {
	entity := s.makeEntity(testEntityID, "old-client")
	s.entityByIDData[entity.ID] = &entity

	newSysAttrs, _ := json.Marshal(map[string]interface{}{"clientId": "new-client"})
	s.mockStore.On("UpdateSystemAttributes", mock.Anything, entity.ID,
		json.RawMessage(newSysAttrs)).Return(nil).Once()

	err := s.cachedStore.UpdateSystemAttributes(context.Background(), entity.ID, newSysAttrs)
	s.Nil(err)
	s.mockStore.AssertExpectations(s.T())

	_, ok := s.entityByIDCache.Get(context.Background(), cache.CacheKey{Key: entity.ID})
	s.False(ok)
}

// DeleteEntity tests

func (s *CacheBackedEntityStoreTestSuite) TestDeleteEntity_InvalidatesEntityCache() {
	entity := s.makeEntity(testEntityID, "client-1")
	s.entityByIDData[entity.ID] = &entity

	s.mockStore.On("DeleteEntity", mock.Anything, entity.ID).Return(nil).Once()

	err := s.cachedStore.DeleteEntity(context.Background(), entity.ID)
	s.Nil(err)
	s.mockStore.AssertExpectations(s.T())

	_, ok := s.entityByIDCache.Get(context.Background(), cache.CacheKey{Key: entity.ID})
	s.False(ok)
}

func (s *CacheBackedEntityStoreTestSuite) TestDeleteEntity_StoreError() {
	entity := s.makeEntity("entity-2", "client-2")
	s.entityByIDData[entity.ID] = &entity

	storeErr := errors.New("delete error")
	s.mockStore.On("DeleteEntity", mock.Anything, entity.ID).Return(storeErr).Once()

	err := s.cachedStore.DeleteEntity(context.Background(), entity.ID)
	s.Equal(storeErr, err)

	// Cache should NOT be invalidated on error.
	_, ok := s.entityByIDCache.Get(context.Background(), cache.CacheKey{Key: entity.ID})
	s.True(ok)
}

// CreateEntity tests

func (s *CacheBackedEntityStoreTestSuite) TestCreateEntity_CachesEntityByID() {
	entity := s.makeEntity(testEntityID, "client-1")
	s.mockStore.On("CreateEntity", mock.Anything, entity, json.RawMessage(nil),
		json.RawMessage(nil)).Return(nil).Once()

	err := s.cachedStore.CreateEntity(context.Background(), entity, nil, nil)
	s.Nil(err)
	s.mockStore.AssertExpectations(s.T())

	cached, ok := s.entityByIDCache.Get(context.Background(), cache.CacheKey{Key: entity.ID})
	s.True(ok)
	s.Equal(entity.ID, cached.ID)
}

func (s *CacheBackedEntityStoreTestSuite) TestCreateEntity_StoreError_DoesNotCache() {
	entity := s.makeEntity(testEntityID, "client-1")
	storeErr := errors.New("create error")
	s.mockStore.On("CreateEntity", mock.Anything, entity, json.RawMessage(nil),
		json.RawMessage(nil)).Return(storeErr).Once()

	err := s.cachedStore.CreateEntity(context.Background(), entity, nil, nil)
	s.Equal(storeErr, err)

	_, ok := s.entityByIDCache.Get(context.Background(), cache.CacheKey{Key: entity.ID})
	s.False(ok)
}

// UpdateEntity tests

func (s *CacheBackedEntityStoreTestSuite) TestUpdateEntity_InvalidatesAndRecachesEntity() {
	entity := s.makeEntity(testEntityID, "client-1")
	s.entityByIDData[entity.ID] = &entity

	s.mockStore.On("UpdateEntity", mock.Anything, &entity).Return(nil).Once()

	err := s.cachedStore.UpdateEntity(context.Background(), &entity)
	s.Nil(err)
	s.mockStore.AssertExpectations(s.T())

	// Updated entity must be present in the by-ID cache.
	cached, ok := s.entityByIDCache.Get(context.Background(), cache.CacheKey{Key: entity.ID})
	s.True(ok)
	s.Equal(entity.ID, cached.ID)
}
