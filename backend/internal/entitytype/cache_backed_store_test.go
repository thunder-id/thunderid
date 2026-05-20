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

package entitytype

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

// CacheBackedStoreTestSuite tests the cachedBackedEntityTypeStore.
type CacheBackedStoreTestSuite struct {
	suite.Suite
	mockStore         *entityTypeStoreInterfaceMock
	schemaByIDCache   *cachemock.CacheInterfaceMock[*EntityType]
	schemaByNameCache *cachemock.CacheInterfaceMock[*EntityType]
	cachedStore       *cachedBackedEntityTypeStore
	// Helper maps to track cached values for verification.
	schemaByIDData   map[string]*EntityType
	schemaByNameData map[string]*EntityType
}

func TestCacheBackedStoreTestSuite(t *testing.T) {
	suite.Run(t, new(CacheBackedStoreTestSuite))
}

func (s *CacheBackedStoreTestSuite) SetupTest() {
	s.mockStore = newEntityTypeStoreInterfaceMock(s.T())
	s.schemaByIDData = make(map[string]*EntityType)
	s.schemaByNameData = make(map[string]*EntityType)

	s.schemaByIDCache = cachemock.NewCacheInterfaceMock[*EntityType](s.T())
	s.schemaByNameCache = cachemock.NewCacheInterfaceMock[*EntityType](s.T())

	setupCacheMock(s.schemaByIDCache, s.schemaByIDData)
	setupCacheMock(s.schemaByNameCache, s.schemaByNameData)

	s.schemaByIDCache.EXPECT().IsEnabled().Return(true).Maybe()
	s.schemaByNameCache.EXPECT().IsEnabled().Return(true).Maybe()

	s.cachedStore = &cachedBackedEntityTypeStore{
		schemaByIDCache:   s.schemaByIDCache,
		schemaByNameCache: s.schemaByNameCache,
		store:             s.mockStore,
		logger: log.GetLogger().With(
			log.String(log.LoggerKeyComponentName, "CacheBackedEntityTypeStore")),
	}
}

// setupCacheMock configures a cache mock to track Set/Get/Delete operations.
func setupCacheMock[T any](
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

// assertSchemaCachedByIDAndName verifies the schema is cached in both ID and Name caches.
func (s *CacheBackedStoreTestSuite) assertSchemaCachedByIDAndName(schema EntityType) {
	cachedByID, ok := s.schemaByIDCache.Get(context.Background(), cacheKeyForID(TypeCategoryUser, schema.ID))
	s.True(ok)
	s.Equal(schema.ID, cachedByID.ID)

	cachedByName, ok := s.schemaByNameCache.Get(context.Background(), cacheKeyForName(TypeCategoryUser, schema.Name))
	s.True(ok)
	s.Equal(schema.Name, cachedByName.Name)
}

// createTestSchema returns a test entity type.
func (s *CacheBackedStoreTestSuite) createTestSchema() EntityType {
	return EntityType{
		ID:                    "schema-1",
		Name:                  "TestSchema",
		OUID:                  "ou-1",
		Category:              TypeCategoryUser,
		AllowSelfRegistration: true,
		SystemAttributes:      &SystemAttributes{Display: "email"},
		Schema:                json.RawMessage(`{"email":{"type":"string"}}`),
	}
}

// TestNewCachedBackedEntityTypeStore verifies suite setup.
func (s *CacheBackedStoreTestSuite) TestNewCachedBackedEntityTypeStore() {
	s.NotNil(s.cachedStore)
	s.IsType(&cachedBackedEntityTypeStore{}, s.cachedStore)
	s.NotNil(s.cachedStore.schemaByIDCache)
	s.NotNil(s.cachedStore.schemaByNameCache)
	s.NotNil(s.cachedStore.store)
}

// GetEntityTypeByID tests

func (s *CacheBackedStoreTestSuite) TestGetEntityTypeByID_CacheHit() {
	schema := s.createTestSchema()
	s.schemaByIDData[string(TypeCategoryUser)+":"+schema.ID] = &schema

	result, err := s.cachedStore.GetEntityTypeByID(context.Background(), TypeCategoryUser, schema.ID)
	s.Nil(err)
	s.Equal(schema.ID, result.ID)
	s.Equal(schema.Name, result.Name)
	// Store should NOT be called on cache hit.
	s.mockStore.AssertNotCalled(s.T(), "GetEntityTypeByID")
}

func (s *CacheBackedStoreTestSuite) TestGetEntityTypeByID_CacheMiss() {
	schema := s.createTestSchema()
	s.mockStore.On("GetEntityTypeByID", mock.Anything, mock.Anything, schema.ID).Return(schema, nil).Once()

	result, err := s.cachedStore.GetEntityTypeByID(context.Background(), TypeCategoryUser, schema.ID)
	s.Nil(err)
	s.Equal(schema.ID, result.ID)
	s.mockStore.AssertExpectations(s.T())
	s.assertSchemaCachedByIDAndName(schema)
}

func (s *CacheBackedStoreTestSuite) TestGetEntityTypeByID_StoreError() {
	storeErr := errors.New("db error")
	s.mockStore.On("GetEntityTypeByID", mock.Anything, mock.Anything, "bad-id").Return(EntityType{}, storeErr).Once()

	_, err := s.cachedStore.GetEntityTypeByID(context.Background(), TypeCategoryUser, "bad-id")
	s.Equal(storeErr, err)

	// Verify nothing cached.
	_, ok := s.schemaByIDCache.Get(context.Background(), cache.CacheKey{Key: "bad-id"})
	s.False(ok)
}

// GetEntityTypeByName tests

func (s *CacheBackedStoreTestSuite) TestGetEntityTypeByName_CacheHit() {
	schema := s.createTestSchema()
	s.schemaByNameData[string(TypeCategoryUser)+":"+schema.Name] = &schema

	result, err := s.cachedStore.GetEntityTypeByName(context.Background(), TypeCategoryUser, schema.Name)
	s.Nil(err)
	s.Equal(schema.Name, result.Name)
	s.mockStore.AssertNotCalled(s.T(), "GetEntityTypeByName")
}

func (s *CacheBackedStoreTestSuite) TestGetEntityTypeByName_CacheMiss() {
	schema := s.createTestSchema()
	s.mockStore.On("GetEntityTypeByName", mock.Anything, mock.Anything, schema.Name).Return(schema, nil).Once()

	result, err := s.cachedStore.GetEntityTypeByName(context.Background(), TypeCategoryUser, schema.Name)
	s.Nil(err)
	s.Equal(schema.Name, result.Name)
	s.mockStore.AssertExpectations(s.T())
	s.assertSchemaCachedByIDAndName(schema)
}

func (s *CacheBackedStoreTestSuite) TestGetEntityTypeByName_StoreError() {
	storeErr := errors.New("db error")
	s.mockStore.On("GetEntityTypeByName", mock.Anything, mock.Anything, "bad-name").
		Return(EntityType{}, storeErr).Once()

	_, err := s.cachedStore.GetEntityTypeByName(context.Background(), TypeCategoryUser, "bad-name")
	s.Equal(storeErr, err)

	_, ok := s.schemaByNameCache.Get(context.Background(), cache.CacheKey{Key: "bad-name"})
	s.False(ok)
}

// CreateEntityType tests

func (s *CacheBackedStoreTestSuite) TestCreateEntityType_Success() {
	schema := s.createTestSchema()
	s.mockStore.On("CreateEntityType", mock.Anything, schema).Return(nil).Once()

	err := s.cachedStore.CreateEntityType(context.Background(), schema)
	s.Nil(err)
	s.mockStore.AssertExpectations(s.T())
	s.assertSchemaCachedByIDAndName(schema)
}

func (s *CacheBackedStoreTestSuite) TestCreateEntityType_StoreError() {
	schema := s.createTestSchema()
	storeErr := errors.New("store error")
	s.mockStore.On("CreateEntityType", mock.Anything, schema).Return(storeErr).Once()

	err := s.cachedStore.CreateEntityType(context.Background(), schema)
	s.Equal(storeErr, err)

	// Verify nothing cached on error.
	_, ok := s.schemaByIDCache.Get(context.Background(), cacheKeyForID(TypeCategoryUser, schema.ID))
	s.False(ok)
}

// UpdateEntityTypeByID tests

func (s *CacheBackedStoreTestSuite) TestUpdateEntityTypeByID_Success() {
	oldSchema := s.createTestSchema()
	s.schemaByIDData[string(TypeCategoryUser)+":"+oldSchema.ID] = &oldSchema
	s.schemaByNameData[string(TypeCategoryUser)+":"+oldSchema.Name] = &oldSchema

	updatedSchema := oldSchema
	updatedSchema.Name = "UpdatedSchema"
	updatedSchema.SystemAttributes = &SystemAttributes{Display: "given_name"}

	s.mockStore.On("UpdateEntityTypeByID", mock.Anything, mock.Anything, oldSchema.ID, updatedSchema).Return(nil).Once()

	err := s.cachedStore.UpdateEntityTypeByID(context.Background(), TypeCategoryUser, oldSchema.ID, updatedSchema)
	s.Nil(err)
	s.mockStore.AssertExpectations(s.T())

	// Old name key should be invalidated.
	_, ok := s.schemaByNameCache.Get(context.Background(), cacheKeyForName(TypeCategoryUser, "TestSchema"))
	s.False(ok)

	// New name key should be cached.
	cachedByNewName, ok := s.schemaByNameCache.Get(
		context.Background(), cacheKeyForName(TypeCategoryUser, "UpdatedSchema"))
	s.True(ok)
	s.Equal("UpdatedSchema", cachedByNewName.Name)

	// ID cache should now point at the updated schema.
	cachedByID, ok := s.schemaByIDCache.Get(context.Background(), cacheKeyForID(TypeCategoryUser, oldSchema.ID))
	s.True(ok)
	s.Equal("UpdatedSchema", cachedByID.Name)
	s.Equal("given_name", cachedByID.SystemAttributes.Display)
}

func (s *CacheBackedStoreTestSuite) TestUpdateEntityTypeByID_StoreError() {
	oldSchema := s.createTestSchema()
	s.schemaByIDData[string(TypeCategoryUser)+":"+oldSchema.ID] = &oldSchema
	s.schemaByNameData[string(TypeCategoryUser)+":"+oldSchema.Name] = &oldSchema

	updatedSchema := oldSchema
	updatedSchema.Name = "UpdatedSchema"

	storeErr := errors.New("update error")
	s.mockStore.On("UpdateEntityTypeByID", mock.Anything, mock.Anything, oldSchema.ID, updatedSchema).
		Return(storeErr).Once()

	err := s.cachedStore.UpdateEntityTypeByID(context.Background(), TypeCategoryUser, oldSchema.ID, updatedSchema)
	s.Equal(storeErr, err)

	// Original cache entries should still exist (not invalidated on error).
	cachedByID, ok := s.schemaByIDCache.Get(context.Background(), cacheKeyForID(TypeCategoryUser, oldSchema.ID))
	s.True(ok)
	s.Equal("TestSchema", cachedByID.Name)

	cachedByName, ok := s.schemaByNameCache.Get(context.Background(), cacheKeyForName(TypeCategoryUser, oldSchema.Name))
	s.True(ok)
	s.Equal("TestSchema", cachedByName.Name)
}

// DeleteEntityTypeByID tests

func (s *CacheBackedStoreTestSuite) TestDeleteEntityTypeByID_ExistsInCache() {
	schema := s.createTestSchema()
	s.schemaByIDData[string(TypeCategoryUser)+":"+schema.ID] = &schema
	s.schemaByNameData[string(TypeCategoryUser)+":"+schema.Name] = &schema

	s.mockStore.On("DeleteEntityTypeByID", mock.Anything, mock.Anything, schema.ID).Return(nil).Once()

	err := s.cachedStore.DeleteEntityTypeByID(context.Background(), TypeCategoryUser, schema.ID)
	s.Nil(err)
	s.mockStore.AssertExpectations(s.T())

	// Both caches should be invalidated.
	_, ok := s.schemaByIDCache.Get(context.Background(), cacheKeyForID(TypeCategoryUser, schema.ID))
	s.False(ok)

	_, ok = s.schemaByNameCache.Get(context.Background(), cacheKeyForName(TypeCategoryUser, schema.Name))
	s.False(ok)
}

func (s *CacheBackedStoreTestSuite) TestDeleteEntityTypeByID_NotInCache() {
	schema := s.createTestSchema()
	s.mockStore.On("GetEntityTypeByID", mock.Anything, mock.Anything, schema.ID).Return(schema, nil).Once()
	s.mockStore.On("DeleteEntityTypeByID", mock.Anything, mock.Anything, schema.ID).Return(nil).Once()

	err := s.cachedStore.DeleteEntityTypeByID(context.Background(), TypeCategoryUser, schema.ID)
	s.Nil(err)
	s.mockStore.AssertExpectations(s.T())

	// Both caches should be invalidated (even though fetched from store).
	_, ok := s.schemaByIDCache.Get(context.Background(), cacheKeyForID(TypeCategoryUser, schema.ID))
	s.False(ok)

	_, ok = s.schemaByNameCache.Get(context.Background(), cacheKeyForName(TypeCategoryUser, schema.Name))
	s.False(ok)
}

func (s *CacheBackedStoreTestSuite) TestDeleteEntityTypeByID_NotFound() {
	s.mockStore.On("GetEntityTypeByID", mock.Anything, mock.Anything, "nonexistent").
		Return(EntityType{}, ErrEntityTypeNotFound).Once()

	err := s.cachedStore.DeleteEntityTypeByID(context.Background(), TypeCategoryUser, "nonexistent")
	s.Nil(err)
	s.mockStore.AssertNotCalled(s.T(), "DeleteEntityTypeByID")
}

// Pass-through method tests

func (s *CacheBackedStoreTestSuite) TestGetEntityTypeListCount_Delegated() {
	s.mockStore.On("GetEntityTypeListCount", mock.Anything, mock.Anything).Return(5, nil).Once()

	count, err := s.cachedStore.GetEntityTypeListCount(context.Background(), TypeCategoryUser)
	s.Nil(err)
	s.Equal(5, count)
	s.mockStore.AssertExpectations(s.T())
}

func (s *CacheBackedStoreTestSuite) TestGetEntityTypeList_Delegated() {
	expected := []EntityTypeListItem{{ID: "s1", Name: "Schema1"}}
	s.mockStore.On("GetEntityTypeList", mock.Anything, mock.Anything, 10, 0).Return(expected, nil).Once()

	result, err := s.cachedStore.GetEntityTypeList(context.Background(), TypeCategoryUser, 10, 0)
	s.Nil(err)
	s.Equal(expected, result)
	s.mockStore.AssertExpectations(s.T())
}

func (s *CacheBackedStoreTestSuite) TestIsEntityTypeDeclarative_Delegated() {
	s.mockStore.On("IsEntityTypeDeclarative", TypeCategoryUser, "schema-1").Return(false).Once()

	result := s.cachedStore.IsEntityTypeDeclarative(TypeCategoryUser, "schema-1")
	s.False(result)
	s.mockStore.AssertExpectations(s.T())
}

func (s *CacheBackedStoreTestSuite) TestGetEntityTypeListByOUIDs_Delegated() {
	expected := []EntityTypeListItem{{ID: "s1", Name: "Schema1"}}
	s.mockStore.On("GetEntityTypeListByOUIDs", mock.Anything, mock.Anything,
		[]string{"ou-1"}, 10, 0).Return(expected, nil).Once()

	result, err := s.cachedStore.GetEntityTypeListByOUIDs(
		context.Background(), TypeCategoryUser, []string{"ou-1"}, 10, 0)
	s.Nil(err)
	s.Equal(expected, result)
	s.mockStore.AssertExpectations(s.T())
}

func (s *CacheBackedStoreTestSuite) TestGetEntityTypeListCountByOUIDs_Delegated() {
	s.mockStore.On("GetEntityTypeListCountByOUIDs", mock.Anything, mock.Anything,
		[]string{"ou-1", "ou-2"}).Return(3, nil).Once()

	count, err := s.cachedStore.GetEntityTypeListCountByOUIDs(
		context.Background(), TypeCategoryUser, []string{"ou-1", "ou-2"})
	s.Nil(err)
	s.Equal(3, count)
	s.mockStore.AssertExpectations(s.T())
}

func (s *CacheBackedStoreTestSuite) TestGetDisplayAttributesByNames_Delegated() {
	expected := map[string]string{"Schema1": "email", "Schema2": "given_name"}
	s.mockStore.On("GetDisplayAttributesByNames", mock.Anything, mock.Anything,
		[]string{"Schema1", "Schema2"}).Return(expected, nil).Once()

	result, err := s.cachedStore.GetDisplayAttributesByNames(
		context.Background(), TypeCategoryUser, []string{"Schema1", "Schema2"})
	s.Nil(err)
	s.Equal(expected, result)
	s.mockStore.AssertExpectations(s.T())
}
