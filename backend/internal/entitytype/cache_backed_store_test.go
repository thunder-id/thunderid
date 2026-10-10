// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
	mockStore           *entityTypeStoreInterfaceMock
	schemaByIDCache     *cachemock.CacheInterfaceMock[*EntityType]
	schemaByHandleCache *cachemock.CacheInterfaceMock[*EntityType]
	cachedStore         *cachedBackedEntityTypeStore
	// Helper maps to track cached values for verification.
	schemaByIDData     map[string]*EntityType
	schemaByHandleData map[string]*EntityType
}

func TestCacheBackedStoreTestSuite(t *testing.T) {
	suite.Run(t, new(CacheBackedStoreTestSuite))
}

func (s *CacheBackedStoreTestSuite) SetupTest() {
	s.mockStore = newEntityTypeStoreInterfaceMock(s.T())
	s.schemaByIDData = make(map[string]*EntityType)
	s.schemaByHandleData = make(map[string]*EntityType)

	s.schemaByIDCache = cachemock.NewCacheInterfaceMock[*EntityType](s.T())
	s.schemaByHandleCache = cachemock.NewCacheInterfaceMock[*EntityType](s.T())

	setupCacheMock(s.schemaByIDCache, s.schemaByIDData)
	setupCacheMock(s.schemaByHandleCache, s.schemaByHandleData)

	s.schemaByIDCache.EXPECT().IsEnabled().Return(true).Maybe()
	s.schemaByHandleCache.EXPECT().IsEnabled().Return(true).Maybe()

	s.cachedStore = &cachedBackedEntityTypeStore{
		schemaByIDCache:     s.schemaByIDCache,
		schemaByHandleCache: s.schemaByHandleCache,
		store:               s.mockStore,
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

// assertSchemaCachedByIDAndHandle verifies the schema is cached in both ID and handle caches.
func (s *CacheBackedStoreTestSuite) assertSchemaCachedByIDAndHandle(schema EntityType) {
	cachedByID, ok := s.schemaByIDCache.Get(context.Background(), cacheKeyForID(TypeCategoryUser, schema.ID))
	s.True(ok)
	s.Equal(schema.ID, cachedByID.ID)

	cachedByName, ok := s.schemaByHandleCache.Get(
		context.Background(), cacheKeyForHandle(TypeCategoryUser, schema.Handle))
	s.True(ok)
	s.Equal(schema.Handle, cachedByName.Handle)
}

// createTestSchema returns a test entity type.
func (s *CacheBackedStoreTestSuite) createTestSchema() EntityType {
	return EntityType{
		ID:                    "schema-1",
		Handle:                "test-schema",
		DisplayName:           "Test Schema",
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
	s.NotNil(s.cachedStore.schemaByHandleCache)
	s.NotNil(s.cachedStore.store)
}

// GetEntityTypeByID tests

func (s *CacheBackedStoreTestSuite) TestGetEntityTypeByID_CacheHit() {
	schema := s.createTestSchema()
	s.schemaByIDData[string(TypeCategoryUser)+":"+schema.ID] = &schema

	result, err := s.cachedStore.GetEntityTypeByID(context.Background(), TypeCategoryUser, schema.ID)
	s.Nil(err)
	s.Equal(schema.ID, result.ID)
	s.Equal(schema.Handle, result.Handle)
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
	s.assertSchemaCachedByIDAndHandle(schema)
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

// GetEntityTypeByHandle tests

func (s *CacheBackedStoreTestSuite) TestGetEntityTypeByHandle_CacheHit() {
	schema := s.createTestSchema()
	s.schemaByHandleData[string(TypeCategoryUser)+":"+schema.Handle] = &schema

	result, err := s.cachedStore.GetEntityTypeByHandle(context.Background(), TypeCategoryUser, schema.Handle)
	s.Nil(err)
	s.Equal(schema.Handle, result.Handle)
	s.mockStore.AssertNotCalled(s.T(), "GetEntityTypeByHandle")
}

func (s *CacheBackedStoreTestSuite) TestGetEntityTypeByHandle_CacheMiss() {
	schema := s.createTestSchema()
	s.mockStore.On("GetEntityTypeByHandle", mock.Anything, mock.Anything, schema.Handle).Return(schema, nil).Once()

	result, err := s.cachedStore.GetEntityTypeByHandle(context.Background(), TypeCategoryUser, schema.Handle)
	s.Nil(err)
	s.Equal(schema.Handle, result.Handle)
	s.mockStore.AssertExpectations(s.T())
	s.assertSchemaCachedByIDAndHandle(schema)
}

func (s *CacheBackedStoreTestSuite) TestGetEntityTypeByHandle_CacheKeyIncludesCategoryAndHandle() {
	s.Equal("user:test-schema", cacheKeyForHandle(TypeCategoryUser, "test-schema").Key)
	s.Equal("agent:test-schema", cacheKeyForHandle(TypeCategoryAgent, "test-schema").Key)

	// A user type cached under a handle must not satisfy a lookup in the agent category.
	schema := s.createTestSchema()
	s.schemaByHandleData[string(TypeCategoryUser)+":"+schema.Handle] = &schema

	agentSchema := schema
	agentSchema.ID = "agent-schema-1"
	agentSchema.Category = TypeCategoryAgent
	s.mockStore.On("GetEntityTypeByHandle", mock.Anything, TypeCategoryAgent, schema.Handle).
		Return(agentSchema, nil).Once()

	result, err := s.cachedStore.GetEntityTypeByHandle(context.Background(), TypeCategoryAgent, schema.Handle)
	s.Nil(err)
	s.Equal("agent-schema-1", result.ID)
	s.mockStore.AssertExpectations(s.T())
}

func (s *CacheBackedStoreTestSuite) TestGetEntityTypeByHandle_StoreError() {
	storeErr := errors.New("db error")
	s.mockStore.On("GetEntityTypeByHandle", mock.Anything, mock.Anything, "bad-name").
		Return(EntityType{}, storeErr).Once()

	_, err := s.cachedStore.GetEntityTypeByHandle(context.Background(), TypeCategoryUser, "bad-name")
	s.Equal(storeErr, err)

	_, ok := s.schemaByHandleCache.Get(context.Background(), cache.CacheKey{Key: "bad-name"})
	s.False(ok)
}

// CreateEntityType tests

func (s *CacheBackedStoreTestSuite) TestCreateEntityType_Success() {
	schema := s.createTestSchema()
	s.mockStore.On("CreateEntityType", mock.Anything, schema).Return(nil).Once()

	err := s.cachedStore.CreateEntityType(context.Background(), schema)
	s.Nil(err)
	s.mockStore.AssertExpectations(s.T())
	s.assertSchemaCachedByIDAndHandle(schema)
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
	s.schemaByHandleData[string(TypeCategoryUser)+":"+oldSchema.Handle] = &oldSchema

	updatedSchema := oldSchema
	updatedSchema.DisplayName = "Updated Schema"
	updatedSchema.SystemAttributes = &SystemAttributes{Display: "given_name"}

	s.mockStore.On("UpdateEntityTypeByID", mock.Anything, mock.Anything, oldSchema.ID, updatedSchema).Return(nil).Once()

	err := s.cachedStore.UpdateEntityTypeByID(context.Background(), TypeCategoryUser, oldSchema.ID, updatedSchema)
	s.Nil(err)
	s.mockStore.AssertExpectations(s.T())

	// The handle key is immutable and must now hold the updated schema.
	cachedByHandle, ok := s.schemaByHandleCache.Get(
		context.Background(), cacheKeyForHandle(TypeCategoryUser, "test-schema"))
	s.True(ok)
	s.Equal("test-schema", cachedByHandle.Handle)
	s.Equal("Updated Schema", cachedByHandle.DisplayName)

	// ID cache should now point at the updated schema.
	cachedByID, ok := s.schemaByIDCache.Get(context.Background(), cacheKeyForID(TypeCategoryUser, oldSchema.ID))
	s.True(ok)
	s.Equal("Updated Schema", cachedByID.DisplayName)
	s.Equal("given_name", cachedByID.SystemAttributes.Display)
}

func (s *CacheBackedStoreTestSuite) TestUpdateEntityTypeByID_StoreError() {
	oldSchema := s.createTestSchema()
	s.schemaByIDData[string(TypeCategoryUser)+":"+oldSchema.ID] = &oldSchema
	s.schemaByHandleData[string(TypeCategoryUser)+":"+oldSchema.Handle] = &oldSchema

	updatedSchema := oldSchema
	updatedSchema.DisplayName = "Updated Schema"

	storeErr := errors.New("update error")
	s.mockStore.On("UpdateEntityTypeByID", mock.Anything, mock.Anything, oldSchema.ID, updatedSchema).
		Return(storeErr).Once()

	err := s.cachedStore.UpdateEntityTypeByID(context.Background(), TypeCategoryUser, oldSchema.ID, updatedSchema)
	s.Equal(storeErr, err)

	// Original cache entries should still exist (not invalidated on error).
	cachedByID, ok := s.schemaByIDCache.Get(context.Background(), cacheKeyForID(TypeCategoryUser, oldSchema.ID))
	s.True(ok)
	s.Equal("Test Schema", cachedByID.DisplayName)

	cachedByHandle, ok := s.schemaByHandleCache.Get(
		context.Background(), cacheKeyForHandle(TypeCategoryUser, oldSchema.Handle))
	s.True(ok)
	s.Equal("Test Schema", cachedByHandle.DisplayName)
}

// DeleteEntityTypeByID tests

func (s *CacheBackedStoreTestSuite) TestDeleteEntityTypeByID_ExistsInCache() {
	schema := s.createTestSchema()
	s.schemaByIDData[string(TypeCategoryUser)+":"+schema.ID] = &schema
	s.schemaByHandleData[string(TypeCategoryUser)+":"+schema.Handle] = &schema

	s.mockStore.On("DeleteEntityTypeByID", mock.Anything, mock.Anything, schema.ID).Return(nil).Once()

	err := s.cachedStore.DeleteEntityTypeByID(context.Background(), TypeCategoryUser, schema.ID)
	s.Nil(err)
	s.mockStore.AssertExpectations(s.T())

	// Both caches should be invalidated.
	_, ok := s.schemaByIDCache.Get(context.Background(), cacheKeyForID(TypeCategoryUser, schema.ID))
	s.False(ok)

	_, ok = s.schemaByHandleCache.Get(context.Background(), cacheKeyForHandle(TypeCategoryUser, schema.Handle))
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

	_, ok = s.schemaByHandleCache.Get(context.Background(), cacheKeyForHandle(TypeCategoryUser, schema.Handle))
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
	expected := []EntityTypeListItem{{ID: "s1", Handle: "schema1", DisplayName: "Schema1"}}
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
	expected := []EntityTypeListItem{{ID: "s1", Handle: "schema1", DisplayName: "Schema1"}}
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

func (s *CacheBackedStoreTestSuite) TestGetDisplayAttributesByHandles_Delegated() {
	expected := map[string]string{"Schema1": "email", "Schema2": "given_name"}
	s.mockStore.On("GetDisplayAttributesByHandles", mock.Anything, mock.Anything,
		[]string{"Schema1", "Schema2"}).Return(expected, nil).Once()

	result, err := s.cachedStore.GetDisplayAttributesByHandles(
		context.Background(), TypeCategoryUser, []string{"Schema1", "Schema2"})
	s.Nil(err)
	s.Equal(expected, result)
	s.mockStore.AssertExpectations(s.T())
}
