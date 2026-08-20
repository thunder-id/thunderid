// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package declarativeresource

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
)

type testEntity struct {
	ID   string
	Name string
	Data string
}

func TestNewGenericFileBasedStore(t *testing.T) {
	store := NewGenericFileBasedStoreForTest(entity.KeyTypeIDP)

	assert.NotNil(t, store)
	assert.Equal(t, entity.KeyTypeIDP, store.keyType)
	assert.NotNil(t, store.storage)
}

func TestGenericFileBasedStore_Create(t *testing.T) {
	store := NewGenericFileBasedStoreForTest(entity.KeyTypeIDP)
	testData := &testEntity{
		ID:   "test-id",
		Name: "Test Entity",
		Data: "test data",
	}

	err := store.Create("test-id", testData)

	assert.NoError(t, err)
}

func TestGenericFileBasedStore_Get(t *testing.T) {
	store := NewGenericFileBasedStoreForTest(entity.KeyTypeIDP)
	testData := &testEntity{
		ID:   "test-id",
		Name: "Test Entity",
		Data: "test data",
	}

	// Create first
	err := store.Create("test-id", testData)
	assert.NoError(t, err)

	// Get
	result, err := store.Get(context.Background(), "test-id")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	retrieved := result.(*testEntity)
	assert.Equal(t, "test-id", retrieved.ID)
	assert.Equal(t, "Test Entity", retrieved.Name)
}

func TestGenericFileBasedStore_GetByField(t *testing.T) {
	store := NewGenericFileBasedStoreForTest(entity.KeyTypeIDP)
	testData := &testEntity{
		ID:   "test-id",
		Name: "Test Entity",
		Data: "test data",
	}

	// Create first
	err := store.Create("test-id", testData)
	assert.NoError(t, err)

	// Get by name
	result, err := store.GetByField(context.Background(), "Test Entity", func(data interface{}) string {
		return data.(*testEntity).Name
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	retrieved := result.(*testEntity)
	assert.Equal(t, "test-id", retrieved.ID)
	assert.Equal(t, "Test Entity", retrieved.Name)
}

func TestGenericFileBasedStore_GetByField_NotFound(t *testing.T) {
	store := NewGenericFileBasedStoreForTest(entity.KeyTypeIDP)

	// Get by name when nothing exists
	result, err := store.GetByField(context.Background(), "NonExistent", func(data interface{}) string {
		return data.(*testEntity).Name
	})

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "entity not found")
}

func TestGenericFileBasedStore_List(t *testing.T) {
	store := NewGenericFileBasedStoreForTest(entity.KeyTypeIDP)

	// Create multiple entities
	for i := 1; i <= 3; i++ {
		testData := &testEntity{
			ID:   "test-id-" + strconv.Itoa(i),
			Name: "Test Entity " + strconv.Itoa(i),
		}
		err := store.Create(testData.ID, testData)
		assert.NoError(t, err)
	}

	// List
	results, err := store.List(context.Background())

	assert.NoError(t, err)
	assert.NotNil(t, results)
	assert.Equal(t, 3, len(results))
}

func TestGenericFileBasedStore_Count(t *testing.T) {
	store := NewGenericFileBasedStoreForTest(entity.KeyTypeIDP)

	// Create multiple entities
	for i := 1; i <= 3; i++ {
		testData := &testEntity{
			ID:   "test-id-" + strconv.Itoa(i),
			Name: "Test Entity " + strconv.Itoa(i),
		}
		err := store.Create(testData.ID, testData)
		assert.NoError(t, err)
	}

	// Count
	count, err := store.Count(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestGenericFileBasedStore_Update_NotSupported(t *testing.T) {
	store := NewGenericFileBasedStoreForTest(entity.KeyTypeIDP)

	err := store.Update("test-id", &testEntity{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "update operation not supported")
}

func TestGenericFileBasedStore_Delete_NotSupported(t *testing.T) {
	store := NewGenericFileBasedStoreForTest(entity.KeyTypeIDP)

	err := store.Delete("test-id")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "delete operation not supported")
}
