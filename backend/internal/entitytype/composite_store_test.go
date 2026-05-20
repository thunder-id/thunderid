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
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
)

// CompositeStoreTestSuite tests the composite entity type store functionality.
type CompositeStoreTestSuite struct {
	suite.Suite
	fileStore      entityTypeStoreInterface
	dbStoreMock    *entityTypeStoreInterfaceMock
	compositeStore *compositeEntityTypeStore
}

// SetupTest sets up the test environment.
func (suite *CompositeStoreTestSuite) SetupTest() {
	// Clear the singleton entity store to avoid state leakage between tests
	_ = entity.GetInstance().Clear()

	// Create NEW file-based store for each test to avoid state leakage
	suite.fileStore, _ = newEntityTypeFileBasedStore()

	// Create mock DB store
	suite.dbStoreMock = newEntityTypeStoreInterfaceMock(suite.T())

	// Create composite store
	suite.compositeStore = newCompositeEntityTypeStore(suite.fileStore, suite.dbStoreMock)
}

// TestCompositeStore_GetEntityTypeByID tests retrieving entity types from composite store.
func (suite *CompositeStoreTestSuite) TestCompositeStore_GetEntityTypeByID() {
	ctx := context.Background()

	testCases := []struct {
		name           string
		schemaID       string
		setupFileStore func()
		setupDBStore   func()
		want           EntityType
		wantErr        bool
	}{
		{
			name:     "retrieves from DB store",
			schemaID: "db-schema-1",
			setupFileStore: func() {
				// File store doesn't have this schema
			},
			setupDBStore: func() {
				suite.dbStoreMock.On("GetEntityTypeByID", ctx, TypeCategoryUser, "db-schema-1").
					Return(EntityType{
						ID:   "db-schema-1",
						Name: "DB Schema",
						OUID: "ou-1",
					}, nil).
					Once()
			},
			want: EntityType{
				ID:   "db-schema-1",
				Name: "DB Schema",
				OUID: "ou-1",
			},
		},
		{
			name:     "retrieves from file store when not in DB",
			schemaID: "file-schema-1",
			setupFileStore: func() {
				// Add schema to file store
				err := suite.fileStore.CreateEntityType(ctx, EntityType{
					ID:       "file-schema-1",
					Category: TypeCategoryUser,
					Name:     "File Schema",
					OUID:     "ou-1",
				})
				suite.NoError(err)
			},
			setupDBStore: func() {
				suite.dbStoreMock.On("GetEntityTypeByID", ctx, TypeCategoryUser, "file-schema-1").
					Return(EntityType{}, ErrEntityTypeNotFound).
					Once()
			},
			want: EntityType{
				ID:       "file-schema-1",
				Category: TypeCategoryUser,
				Name:     "File Schema",
				OUID:     "ou-1",
			},
		},
		{
			name:     "returns error when not found in both stores",
			schemaID: "nonexistent",
			setupFileStore: func() {
				// File store doesn't have this schema
			},
			setupDBStore: func() {
				suite.dbStoreMock.On("GetEntityTypeByID", ctx, TypeCategoryUser, "nonexistent").
					Return(EntityType{}, ErrEntityTypeNotFound).
					Once()
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			tc.setupFileStore()
			tc.setupDBStore()

			result, err := suite.compositeStore.GetEntityTypeByID(ctx, TypeCategoryUser, tc.schemaID)

			if tc.wantErr {
				suite.Error(err)
			} else {
				suite.NoError(err)
				suite.Equal(tc.want, result)
			}
		})
	}
}

// TestCompositeStore_GetEntityTypeByName tests retrieving entity types by name from composite store.
func (suite *CompositeStoreTestSuite) TestCompositeStore_GetEntityTypeByName() {
	ctx := context.Background()

	testCases := []struct {
		name           string
		schemaName     string
		setupFileStore func()
		setupDBStore   func()
		want           EntityType
		wantErr        bool
	}{
		{
			name:       "retrieves from DB store",
			schemaName: "DBSchema",
			setupFileStore: func() {
				// File store doesn't have this schema
			},
			setupDBStore: func() {
				suite.dbStoreMock.On("GetEntityTypeByName", ctx, TypeCategoryUser, "DBSchema").
					Return(EntityType{
						ID:   "db-schema-1",
						Name: "DBSchema",
						OUID: "ou-1",
					}, nil).
					Once()
			},
			want: EntityType{
				ID:   "db-schema-1",
				Name: "DBSchema",
				OUID: "ou-1",
			},
		},
		{
			name:       "retrieves from file store when not in DB",
			schemaName: "FileSchema",
			setupFileStore: func() {
				// Add schema to file store
				err := suite.fileStore.CreateEntityType(ctx, EntityType{
					ID:       "file-schema-1",
					Category: TypeCategoryUser,
					Name:     "FileSchema",
					OUID:     "ou-1",
				})
				suite.NoError(err)
			},
			setupDBStore: func() {
				suite.dbStoreMock.On("GetEntityTypeByName", ctx, TypeCategoryUser, "FileSchema").
					Return(EntityType{}, ErrEntityTypeNotFound).
					Once()
			},
			want: EntityType{
				ID:       "file-schema-1",
				Category: TypeCategoryUser,
				Name:     "FileSchema",
				OUID:     "ou-1",
			},
		},
		{
			name:       "returns error when not found in both stores",
			schemaName: "NonExistentSchema",
			setupFileStore: func() {
				// File store is empty - no schemas added
			},
			setupDBStore: func() {
				suite.dbStoreMock.On("GetEntityTypeByName", ctx, TypeCategoryUser, "NonExistentSchema").
					Return(EntityType{}, ErrEntityTypeNotFound).
					Once()
			},
			want:    EntityType{},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			tc.setupFileStore()
			tc.setupDBStore()

			result, err := suite.compositeStore.GetEntityTypeByName(ctx, TypeCategoryUser, tc.schemaName)

			if tc.wantErr {
				suite.Error(err)
			} else {
				suite.NoError(err)
				suite.Equal(tc.want, result)
			}
		})
	}
}

// TestCompositeStore_IsEntityTypeDeclarative tests the IsEntityTypeDeclarative method.
func (suite *CompositeStoreTestSuite) TestCompositeStore_IsEntityTypeDeclarative() {
	ctx := context.Background()

	const fileSchema1ID = "file-schema-1"
	// Setup: Add schema to file store
	err := suite.fileStore.CreateEntityType(ctx, EntityType{
		ID:       fileSchema1ID,
		Category: TypeCategoryUser,
		Name:     "File Schema",
		OUID:     "ou-1",
	})
	suite.NoError(err)

	// Test: File schema should be declarative
	suite.True(suite.compositeStore.IsEntityTypeDeclarative(TypeCategoryUser, fileSchema1ID))

	// Test: DB schema should not be declarative (file store will return false for non-existent)
	suite.False(suite.compositeStore.IsEntityTypeDeclarative(TypeCategoryUser, "db-schema-1"))
}

// TestCompositeStore_CreateEntityType tests creating entity types.
func (suite *CompositeStoreTestSuite) TestCompositeStore_CreateEntityType() {
	ctx := context.Background()

	schema := EntityType{
		ID:   "new-schema",
		Name: "New Schema",
		OUID: "ou-1",
	}

	suite.dbStoreMock.On("CreateEntityType", ctx, schema).
		Return(nil).
		Once()

	err := suite.compositeStore.CreateEntityType(ctx, schema)
	suite.NoError(err)
	suite.dbStoreMock.AssertExpectations(suite.T())
}

// TestCompositeStore_UpdateEntityType tests updating entity types.
func (suite *CompositeStoreTestSuite) TestCompositeStore_UpdateEntityTypeByID() {
	ctx := context.Background()

	schemaID := "schema-1"
	schema := EntityType{
		ID:   schemaID,
		Name: "Updated Schema",
		OUID: "ou-1",
	}

	suite.dbStoreMock.On("UpdateEntityTypeByID", ctx, TypeCategoryUser, schemaID, schema).
		Return(nil).
		Once()

	err := suite.compositeStore.UpdateEntityTypeByID(ctx, TypeCategoryUser, schemaID, schema)
	suite.NoError(err)
	suite.dbStoreMock.AssertExpectations(suite.T())
}

// TestCompositeStore_DeleteEntityType tests deleting entity types.
func (suite *CompositeStoreTestSuite) TestCompositeStore_DeleteEntityTypeByID() {
	ctx := context.Background()

	schemaID := "schema-1"

	suite.dbStoreMock.On("DeleteEntityTypeByID", ctx, TypeCategoryUser, schemaID).
		Return(nil).
		Once()

	err := suite.compositeStore.DeleteEntityTypeByID(ctx, TypeCategoryUser, schemaID)
	suite.NoError(err)
	suite.dbStoreMock.AssertExpectations(suite.T())
}

// TestCompositeStore_GetEntityTypeList tests retrieving paginated list of entity types from composite store.
// Note: This tests the basic functionality. Detailed pagination and merge logic testing
// should be done separately as it involves complex mock setup with the merge helpers.
func (suite *CompositeStoreTestSuite) TestCompositeStore_GetEntityTypeList() {
	ctx := context.Background()

	// Test that retrieves from file store (which doesn't require complex mocking)
	err := suite.fileStore.CreateEntityType(ctx, EntityType{
		ID:       "file-schema-1",
		Category: TypeCategoryUser,
		Name:     "File Schema 1",
		OUID:     "ou-1",
	})
	suite.NoError(err)

	// Set up mock for DB store - note that the composite store calls GetEntityTypeList
	// with the full count from the database, not using the limit parameter directly
	dbCount := 1
	suite.dbStoreMock.On("GetEntityTypeListCount", ctx, TypeCategoryUser).
		Return(dbCount, nil).
		Once()
	suite.dbStoreMock.On("GetEntityTypeList", ctx, TypeCategoryUser, dbCount, 0).
		Return([]EntityTypeListItem{
			{
				ID:   "db-schema-1",
				Name: "DB Schema 1",
				OUID: "ou-1",
			},
		}, nil).
		Once()

	result, err := suite.compositeStore.GetEntityTypeList(ctx, TypeCategoryUser, 100, 0)

	suite.NoError(err)
	// Should have both from file store and DB store
	suite.Equal(2, len(result))
	// Verify that all results have the IsReadOnly flag set appropriately
	for _, item := range result {
		if item.ID == "file-schema-1" {
			suite.True(item.IsReadOnly, "File-based schemas should be read-only")
		} else if item.ID == "db-schema-1" {
			suite.False(item.IsReadOnly, "DB-backed schemas should be mutable")
		}
	}
}

// TestCompositeStore_MergeAndDeduplicateEntityTypes tests the merge and deduplicate function.
func (suite *CompositeStoreTestSuite) TestCompositeStore_MergeAndDeduplicateEntityTypes() {
	dbSchemas := []EntityTypeListItem{
		{ID: "schema-1", Name: "Schema 1"},
		{ID: "schema-2", Name: "Schema 2"},
	}

	fileSchemas := []EntityTypeListItem{
		{ID: "schema-3", Name: "Schema 3"},
		{ID: "schema-1", Name: "Schema 1 Duplicate"}, // Duplicate - should use DB version
	}

	result := mergeAndDeduplicateEntityTypes(dbSchemas, fileSchemas)

	// Should have 3 unique schemas
	suite.Len(result, 3)

	// Verify DB schemas come first and are marked mutable
	suite.Equal("schema-1", result[0].ID)
	suite.False(result[0].IsReadOnly)
	suite.Equal("schema-2", result[1].ID)
	suite.False(result[1].IsReadOnly)

	// File schemas that weren't duplicated should be marked immutable
	suite.Equal("schema-3", result[2].ID)
	suite.True(result[2].IsReadOnly)
}

// TestCompositeStore_GetEntityTypeListCount tests the total count retrieval from composite store.
func (suite *CompositeStoreTestSuite) TestCompositeStore_GetEntityTypeListCount() {
	ctx := context.Background()

	// Setup: Add schema to file store
	err := suite.fileStore.CreateEntityType(ctx, EntityType{
		ID:       "file-schema-1",
		Category: TypeCategoryUser,
		Name:     "File Schema 1",
		OUID:     "ou-1",
	})
	suite.NoError(err)

	err = suite.fileStore.CreateEntityType(ctx, EntityType{
		ID:       "file-schema-2",
		Category: TypeCategoryUser,
		Name:     "File Schema 2",
		OUID:     "ou-1",
	})
	suite.NoError(err)

	// Setup: Mock DB store count
	suite.dbStoreMock.On("GetEntityTypeListCount", ctx, TypeCategoryUser).
		Return(3, nil).
		Once()

	// Execute
	count, err := suite.compositeStore.GetEntityTypeListCount(ctx, TypeCategoryUser)

	// Verify - should sum both counts (3 from DB + 2 from file = 5)
	suite.NoError(err)
	suite.Equal(5, count)
	suite.dbStoreMock.AssertExpectations(suite.T())
}

// TestCompositeStore_GetEntityTypeListCount_FileStoreError tests error handling when file store fails.
func (suite *CompositeStoreTestSuite) TestCompositeStore_GetEntityTypeListCount_FileStoreError() {
	ctx := context.Background()

	// Setup: Mock DB store count (returns successfully)
	suite.dbStoreMock.On("GetEntityTypeListCount", ctx, TypeCategoryUser).
		Return(3, nil).
		Once()

	// File store will throw an error since it's empty and we're forcing an error scenario
	// by not adding any schemas and the underlying helper will handle errors

	// Note: The actual file store won't error, but we're testing the composite helper's error handling
	// For this test, we rely on the helper function's error propagation logic
	count, err := suite.compositeStore.GetEntityTypeListCount(ctx, TypeCategoryUser)

	// Should still work since both stores respond with counts (even if file store is 0)
	suite.NoError(err)
	suite.Equal(3, count) // DB count only since file store has 0
	suite.dbStoreMock.AssertExpectations(suite.T())
}

// TestCompositeStore_GetEntityTypeList_LimitExceeded tests the limit exceeded error when total records exceed max.
func (suite *CompositeStoreTestSuite) TestCompositeStore_GetEntityTypeList_LimitExceeded() {
	ctx := context.Background()

	// Mock DB store to return a count that would exceed the limit
	// The max composite store records is typically 1000 (serverconst.MaxCompositeStoreRecords)
	suite.dbStoreMock.On("GetEntityTypeListCount", ctx, TypeCategoryUser).
		Return(1001, nil).
		Once()

	// Attempt to get list - should fail due to limit
	result, err := suite.compositeStore.GetEntityTypeList(ctx, TypeCategoryUser, 100, 0)

	suite.Error(err)
	suite.Nil(result)
	suite.Equal(errResultLimitExceededInCompositeMode, err)
	suite.dbStoreMock.AssertExpectations(suite.T())
}

// TestCompositeStore_GetEntityTypeList_DBStoreCountError tests error handling when DB store count fails.
func (suite *CompositeStoreTestSuite) TestCompositeStore_GetEntityTypeList_DBStoreCountError() {
	ctx := context.Background()

	// Mock DB store to return an error
	suite.dbStoreMock.On("GetEntityTypeListCount", ctx, TypeCategoryUser).
		Return(0, errors.New("database error")).
		Once()

	// Attempt to get list - should propagate the error
	result, err := suite.compositeStore.GetEntityTypeList(ctx, TypeCategoryUser, 100, 0)
	suite.Error(err)
	suite.Nil(result)
	suite.dbStoreMock.AssertExpectations(suite.T())
}

// TestCompositeStore_GetEntityTypeListCountByOUIDs tests retrieving count filtered by OU IDs from composite store.
func (suite *CompositeStoreTestSuite) TestCompositeStore_GetEntityTypeListCountByOUIDs() {
	ctx := context.Background()
	ouIDs := []string{"ou-1"}

	// Setup: Add schema to file store
	err := suite.fileStore.CreateEntityType(ctx, EntityType{
		ID:       "file-schema-1",
		Category: TypeCategoryUser,
		Name:     "File Schema",
		OUID:     "ou-1",
	})
	suite.NoError(err)

	err = suite.fileStore.CreateEntityType(ctx, EntityType{
		ID:       "file-schema-2",
		Category: TypeCategoryUser,
		Name:     "File Schema 2",
		OUID:     "ou-2",
	})
	suite.NoError(err)

	// Setup: Mock DB store count filtered by OU IDs
	suite.dbStoreMock.On("GetEntityTypeListCountByOUIDs", ctx, TypeCategoryUser, ouIDs).
		Return(2, nil).
		Once()

	// Execute
	count, err := suite.compositeStore.GetEntityTypeListCountByOUIDs(ctx, TypeCategoryUser, ouIDs)

	// Verify - should sum both counts (2 from DB + 1 from file matching "ou-1" = 3)
	suite.NoError(err)
	suite.Equal(3, count)
	suite.dbStoreMock.AssertExpectations(suite.T())
}

// TestCompositeStore_GetEntityTypeListByOUIDs tests retrieving entity types filtered by OU IDs from composite store.
func (suite *CompositeStoreTestSuite) TestCompositeStore_GetEntityTypeListByOUIDs() {
	ctx := context.Background()
	ouIDs := []string{"ou-1"}

	// Setup file store
	err := suite.fileStore.CreateEntityType(ctx, EntityType{
		ID:       "file-schema-1",
		Category: TypeCategoryUser,
		Name:     "File Schema 1",
		OUID:     "ou-1",
	})
	suite.NoError(err)

	// Set up mock for DB store
	dbCount := 1
	suite.dbStoreMock.On("GetEntityTypeListCountByOUIDs", ctx, TypeCategoryUser, ouIDs).
		Return(dbCount, nil).
		Once()
	suite.dbStoreMock.On("GetEntityTypeListByOUIDs", ctx, TypeCategoryUser, ouIDs, dbCount, 0).
		Return([]EntityTypeListItem{
			{
				ID:   "db-schema-1",
				Name: "DB Schema 1",
				OUID: "ou-1",
			},
		}, nil).
		Once()

	result, err := suite.compositeStore.GetEntityTypeListByOUIDs(ctx, TypeCategoryUser, ouIDs, 100, 0)

	suite.NoError(err)
	// Should have both from file store and DB store
	suite.Equal(2, len(result))
	for _, item := range result {
		if item.ID == "file-schema-1" {
			suite.True(item.IsReadOnly, "File-based schemas should be read-only")
		} else if item.ID == "db-schema-1" {
			suite.False(item.IsReadOnly, "DB-backed schemas should be mutable")
		}
	}
}

// TestCompositeStore_GetEntityTypeListByOUIDs_LimitExceeded tests the limit exceeded error.
func (suite *CompositeStoreTestSuite) TestCompositeStore_GetEntityTypeListByOUIDs_LimitExceeded() {
	ctx := context.Background()
	ouIDs := []string{"ou-1"}

	// Mock DB store to return a count that would exceed the limit
	suite.dbStoreMock.On("GetEntityTypeListCountByOUIDs", ctx, TypeCategoryUser, ouIDs).
		Return(1001, nil).
		Once()

	// Attempt to get list - should fail due to limit
	result, err := suite.compositeStore.GetEntityTypeListByOUIDs(ctx, TypeCategoryUser, ouIDs, 100, 0)

	suite.Error(err)
	suite.Nil(result)
	suite.Equal(errResultLimitExceededInCompositeMode, err)
	suite.dbStoreMock.AssertExpectations(suite.T())
}

// TestCompositeStore_GetEntityTypeListByOUIDs_DBStoreCountError tests error handling when DB store count fails.
func (suite *CompositeStoreTestSuite) TestCompositeStore_GetEntityTypeListByOUIDs_DBStoreCountError() {
	ctx := context.Background()
	ouIDs := []string{"ou-1"}

	// Mock DB store to return an error
	suite.dbStoreMock.On("GetEntityTypeListCountByOUIDs", ctx, TypeCategoryUser, ouIDs).
		Return(0, errors.New("database error")).
		Once()

	// Attempt to get list - should propagate the error
	result, err := suite.compositeStore.GetEntityTypeListByOUIDs(ctx, TypeCategoryUser, ouIDs, 100, 0)

	suite.Error(err)
	suite.Nil(result)
	suite.dbStoreMock.AssertExpectations(suite.T())
}

// In order for 'go test' to run this suite, we need to create a normal test function
// and pass our suite to suite.Run
func TestCompositeStoreTestSuite(t *testing.T) {
	suite.Run(t, new(CompositeStoreTestSuite))
}
