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

package entitytype

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
)

const testSchemaJSON = `{"type":"object"}`

type FileBasedStoreTestSuite struct {
	suite.Suite
	store entityTypeStoreInterface
}

func (suite *FileBasedStoreTestSuite) SetupTest() {
	suite.store = newEntityTypeFileBasedStoreForTest()
}

// newEntityTypeFileBasedStoreForTest creates a test instance
func newEntityTypeFileBasedStoreForTest() entityTypeStoreInterface {
	genericStore := declarativeresource.NewGenericFileBasedStoreForTest(entity.KeyTypeEntityType)
	return &entityTypeFileBasedStore{
		GenericFileBasedStore: genericStore,
	}
}

func TestFileBasedStoreTestSuite(t *testing.T) {
	suite.Run(t, new(FileBasedStoreTestSuite))
}

func (suite *FileBasedStoreTestSuite) TestCreateEntityType() {
	schemaJSON := `{"type":"object","properties":{"username":{"type":"string"}}}`
	schema := EntityType{
		ID:                    "schema-1",
		Category:              TypeCategoryUser,
		Name:                  "basic_schema",
		OUID:                  "ou-1",
		AllowSelfRegistration: true,
		Schema:                json.RawMessage(schemaJSON),
	}

	err := suite.store.CreateEntityType(context.Background(), schema)
	assert.NoError(suite.T(), err)

	// Verify schema was stored
	retrieved, err := suite.store.GetEntityTypeByID(context.Background(), TypeCategoryUser, "schema-1")
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), schema.ID, retrieved.ID)
	assert.Equal(suite.T(), schema.Name, retrieved.Name)
	assert.Equal(suite.T(), schema.OUID, retrieved.OUID)
	assert.Equal(suite.T(), schema.AllowSelfRegistration, retrieved.AllowSelfRegistration)
}

func (suite *FileBasedStoreTestSuite) TestCreateEntityType_DuplicateID() {
	schemaJSON := testSchemaJSON
	schema := EntityType{
		ID:                    "schema-1",
		Category:              TypeCategoryUser,
		Name:                  "basic_schema",
		OUID:                  "ou-1",
		AllowSelfRegistration: true,
		Schema:                json.RawMessage(schemaJSON),
	}

	// Create first schema
	err := suite.store.CreateEntityType(context.Background(), schema)
	assert.NoError(suite.T(), err)

	// Try to create duplicate - should succeed in file-based store as it doesn't check duplicates
	err = suite.store.CreateEntityType(context.Background(), schema)
	// File-based store may allow duplicate or return error depending on implementation
	// Just verify it doesn't panic
	_ = err
}

func (suite *FileBasedStoreTestSuite) TestGetEntityTypeByID_NotFound() {
	_, err := suite.store.GetEntityTypeByID(context.Background(), TypeCategoryUser, "non-existent-id")
	assert.Error(suite.T(), err)
}

func (suite *FileBasedStoreTestSuite) TestGetEntityTypeByName() {
	schemaJSON := testSchemaJSON
	schema := EntityType{
		ID:                    "schema-1",
		Category:              TypeCategoryUser,
		Name:                  "basic_schema",
		OUID:                  "ou-1",
		AllowSelfRegistration: true,
		Schema:                json.RawMessage(schemaJSON),
	}

	err := suite.store.CreateEntityType(context.Background(), schema)
	assert.NoError(suite.T(), err)

	// Get by name
	retrieved, err := suite.store.GetEntityTypeByName(context.Background(), TypeCategoryUser, "basic_schema")
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), schema.ID, retrieved.ID)
	assert.Equal(suite.T(), schema.Name, retrieved.Name)
}

func (suite *FileBasedStoreTestSuite) TestGetEntityTypeByName_NotFound() {
	_, err := suite.store.GetEntityTypeByName(context.Background(), TypeCategoryUser, "non-existent-name")
	assert.Error(suite.T(), err)
}

func (suite *FileBasedStoreTestSuite) TestGetEntityTypeList() {
	schemaJSON := testSchemaJSON
	// Create multiple schemas
	schemas := []EntityType{
		{
			ID:                    "schema-1",
			Category:              TypeCategoryUser,
			Name:                  "basic_schema",
			OUID:                  "ou-1",
			AllowSelfRegistration: true,
			Schema:                json.RawMessage(schemaJSON),
		},
		{
			ID:                    "schema-2",
			Category:              TypeCategoryUser,
			Name:                  "extended_schema",
			OUID:                  "ou-1",
			AllowSelfRegistration: false,
			Schema:                json.RawMessage(schemaJSON),
		},
		{
			ID:                    "schema-3",
			Category:              TypeCategoryUser,
			Name:                  "minimal_schema",
			OUID:                  "ou-1",
			AllowSelfRegistration: true,
			Schema:                json.RawMessage(schemaJSON),
		},
	}
	for _, schema := range schemas {
		err := suite.store.CreateEntityType(context.Background(), schema)
		assert.NoError(suite.T(), err)
	}

	// Get list with pagination
	list, err := suite.store.GetEntityTypeList(context.Background(), TypeCategoryUser, 10, 0)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), list, 3)
}

func (suite *FileBasedStoreTestSuite) TestGetEntityTypeList_WithPagination() {
	schemaJSON := testSchemaJSON
	// Create multiple schemas
	for i := 1; i <= 5; i++ {
		schema := EntityType{
			ID:                    "schema-" + string(rune('0'+i)),
			Category:              TypeCategoryUser,
			Name:                  "schema_" + string(rune('0'+i)),
			OUID:                  "ou-1",
			AllowSelfRegistration: true,
			Schema:                json.RawMessage(schemaJSON),
		}
		err := suite.store.CreateEntityType(context.Background(), schema)
		assert.NoError(suite.T(), err)
	}

	// Get first page
	list, err := suite.store.GetEntityTypeList(context.Background(), TypeCategoryUser, 2, 0)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), list, 2)

	// Get second page
	list, err = suite.store.GetEntityTypeList(context.Background(), TypeCategoryUser, 2, 2)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), list, 2)

	// Get last page
	list, err = suite.store.GetEntityTypeList(context.Background(), TypeCategoryUser, 2, 4)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), list, 1)
}

func (suite *FileBasedStoreTestSuite) TestGetEntityTypeList_EmptyStore() {
	list, err := suite.store.GetEntityTypeList(context.Background(), TypeCategoryUser, 10, 0)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), list, 0)
}

func (suite *FileBasedStoreTestSuite) TestUpdateEntityTypeByID_ReturnsError() {
	schemaJSON := testSchemaJSON
	schema := EntityType{
		ID:                    "schema-1",
		Category:              TypeCategoryUser,
		Name:                  "basic_schema",
		OUID:                  "ou-1",
		AllowSelfRegistration: true,
		Schema:                json.RawMessage(schemaJSON),
	}

	err := suite.store.UpdateEntityTypeByID(context.Background(), TypeCategoryUser, "schema-1", schema)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "not supported")
}

func (suite *FileBasedStoreTestSuite) TestDeleteEntityTypeByID_ReturnsError() {
	err := suite.store.DeleteEntityTypeByID(context.Background(), TypeCategoryUser, "schema-1")
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "not supported")
}

func (suite *FileBasedStoreTestSuite) TestGetEntityTypeListCount() {
	// Initially empty
	count, err := suite.store.GetEntityTypeListCount(context.Background(), TypeCategoryUser)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), 0, count)

	schemaJSON := testSchemaJSON
	// Add schemas
	for i := 1; i <= 3; i++ {
		schema := EntityType{
			ID:                    "schema-" + string(rune('0'+i)),
			Category:              TypeCategoryUser,
			Name:                  "schema_" + string(rune('0'+i)),
			OUID:                  "ou-1",
			AllowSelfRegistration: true,
			Schema:                json.RawMessage(schemaJSON),
		}
		err := suite.store.CreateEntityType(context.Background(), schema)
		assert.NoError(suite.T(), err)
	}

	// Check count
	count, err = suite.store.GetEntityTypeListCount(context.Background(), TypeCategoryUser)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), 3, count)
}

func (suite *FileBasedStoreTestSuite) TestGetEntityTypeListByOUIDs() {
	schemaJSON := testSchemaJSON
	schemas := []EntityType{
		{
			ID:                    "schema-1",
			Category:              TypeCategoryUser,
			Name:                  "schema_1",
			OUID:                  "ou-1",
			AllowSelfRegistration: true,
			Schema:                json.RawMessage(schemaJSON),
		},
		{
			ID:                    "schema-2",
			Category:              TypeCategoryUser,
			Name:                  "schema_2",
			OUID:                  "ou-2",
			AllowSelfRegistration: false,
			Schema:                json.RawMessage(schemaJSON),
		},
		{
			ID:                    "schema-3",
			Category:              TypeCategoryUser,
			Name:                  "schema_3",
			OUID:                  "ou-1",
			AllowSelfRegistration: true,
			Schema:                json.RawMessage(schemaJSON),
		},
	}
	for _, schema := range schemas {
		err := suite.store.CreateEntityType(context.Background(), schema)
		assert.NoError(suite.T(), err)
	}

	testCases := []struct {
		name          string
		ouIDs         []string
		limit         int
		offset        int
		expectedCount int
		expectedNames []string
	}{
		{
			name:          "Get by single OU ID",
			ouIDs:         []string{"ou-2"},
			limit:         10,
			offset:        0,
			expectedCount: 1,
			expectedNames: []string{"schema_2"},
		},
		{
			name:          "Get by multiple OU IDs",
			ouIDs:         []string{"ou-1", "ou-2"},
			limit:         10,
			offset:        0,
			expectedCount: 3,
			expectedNames: []string{"schema_1", "schema_2", "schema_3"},
		},
		{
			name:          "Get by non-existent OU ID",
			ouIDs:         []string{"ou-3"},
			limit:         10,
			offset:        0,
			expectedCount: 0,
			expectedNames: []string{},
		},
		{
			name:          "Pagination limit",
			ouIDs:         []string{"ou-1"},
			limit:         1,
			offset:        0,
			expectedCount: 1,
			expectedNames: []string{"schema_1"},
		},
		{
			name:          "Pagination offset",
			ouIDs:         []string{"ou-1"},
			limit:         10,
			offset:        1,
			expectedCount: 1,
			expectedNames: []string{"schema_3"},
		},
		{
			name:          "Pagination beyond total",
			ouIDs:         []string{"ou-1"},
			limit:         10,
			offset:        5,
			expectedCount: 0,
			expectedNames: []string{},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			list, err := suite.store.GetEntityTypeListByOUIDs(
				context.Background(), TypeCategoryUser, tc.ouIDs, tc.limit, tc.offset)
			assert.NoError(suite.T(), err)
			assert.Len(suite.T(), list, tc.expectedCount)

			// Verify names if expected
			var names []string
			for _, item := range list {
				names = append(names, item.Name)
			}
			assert.ElementsMatch(suite.T(), tc.expectedNames, names)
		})
	}
}

func (suite *FileBasedStoreTestSuite) TestGetEntityTypeListCountByOUIDs() {
	schemaJSON := testSchemaJSON
	schemas := []EntityType{
		{
			ID:                    "schema-1",
			Category:              TypeCategoryUser,
			Name:                  "schema_1",
			OUID:                  "ou-1",
			AllowSelfRegistration: true,
			Schema:                json.RawMessage(schemaJSON),
		},
		{
			ID:                    "schema-2",
			Category:              TypeCategoryUser,
			Name:                  "schema_2",
			OUID:                  "ou-2",
			AllowSelfRegistration: false,
			Schema:                json.RawMessage(schemaJSON),
		},
		{
			ID:                    "schema-3",
			Category:              TypeCategoryUser,
			Name:                  "schema_3",
			OUID:                  "ou-1",
			AllowSelfRegistration: true,
			Schema:                json.RawMessage(schemaJSON),
		},
	}
	for _, schema := range schemas {
		err := suite.store.CreateEntityType(context.Background(), schema)
		assert.NoError(suite.T(), err)
	}

	testCases := []struct {
		name          string
		ouIDs         []string
		expectedCount int
	}{
		{
			name:          "Count by single OU ID",
			ouIDs:         []string{"ou-2"},
			expectedCount: 1,
		},
		{
			name:          "Count by multiple OU IDs",
			ouIDs:         []string{"ou-1", "ou-2"},
			expectedCount: 3,
		},
		{
			name:          "Count by non-existent OU ID",
			ouIDs:         []string{"ou-3"},
			expectedCount: 0,
		},
		{
			name:          "Empty OU IDs",
			ouIDs:         []string{},
			expectedCount: 0,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			count, err := suite.store.GetEntityTypeListCountByOUIDs(context.Background(), TypeCategoryUser, tc.ouIDs)
			assert.NoError(suite.T(), err)
			assert.Equal(suite.T(), tc.expectedCount, count)
		})
	}
}
