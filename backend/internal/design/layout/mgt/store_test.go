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

package layoutmgt

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

// LayoutStoreTestSuite contains tests for the layout store.
type LayoutStoreTestSuite struct {
	suite.Suite
	mockDBProvider *providermock.DBProviderInterfaceMock
	mockDBClient   *providermock.DBClientInterfaceMock
	store          *layoutMgtStore
}

func TestLayoutStoreTestSuite(t *testing.T) {
	suite.Run(t, new(LayoutStoreTestSuite))
}

func (suite *LayoutStoreTestSuite) SetupTest() {
	suite.mockDBProvider = providermock.NewDBProviderInterfaceMock(suite.T())
	suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())
	suite.store = &layoutMgtStore{
		dbProvider:   suite.mockDBProvider,
		deploymentID: "test-deployment",
	}
}

// Test GetLayoutListCount - Success
func (suite *LayoutStoreTestSuite) TestGetLayoutListCount_Success() {
	results := []map[string]interface{}{
		{"total": int64(5)},
	}
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("Query", mock.Anything, "test-deployment").Return(results, nil)

	count, err := suite.store.GetLayoutListCount()

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), 5, count)
}

// Test GetLayoutListCount - DB client error
func (suite *LayoutStoreTestSuite) TestGetLayoutListCount_DBClientError() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(nil, errors.New("connection error"))

	count, err := suite.store.GetLayoutListCount()

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), 0, count)
}

// Test GetLayoutListCount - Query error
func (suite *LayoutStoreTestSuite) TestGetLayoutListCount_QueryError() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("Query", mock.Anything, "test-deployment").
		Return(nil, errors.New("query error"))

	count, err := suite.store.GetLayoutListCount()

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), 0, count)
}

// Test GetLayoutList - Success
func (suite *LayoutStoreTestSuite) TestGetLayoutList_Success() {
	results := []map[string]interface{}{
		{
			"id":           "layout-1",
			"handle":       "layout-one",
			"display_name": "Layout 1",
			"description":  "Description 1",
			"created_at":   "2024-01-15T10:30:00Z",
			"updated_at":   "2024-01-15T10:30:00Z",
		},
		{
			"id":           "layout-2",
			"handle":       "layout-two",
			"display_name": "Layout 2",
			"description":  "Description 2",
			"created_at":   "2024-01-15T10:30:00Z",
			"updated_at":   "2024-01-15T10:30:00Z",
		},
	}
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("Query", mock.Anything, 10, 0, "test-deployment").Return(results, nil)

	layouts, err := suite.store.GetLayoutList(10, 0)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), layouts, 2)
	assert.Equal(suite.T(), "layout-1", layouts[0].ID)
	assert.Equal(suite.T(), "Layout 1", layouts[0].DisplayName)
}

// Test GetLayoutList - DB client error
func (suite *LayoutStoreTestSuite) TestGetLayoutList_DBClientError() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(nil, errors.New("connection error"))

	layouts, err := suite.store.GetLayoutList(10, 0)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), layouts)
}

// Test CreateLayout - Success
func (suite *LayoutStoreTestSuite) TestCreateLayout_Success() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("Execute", mock.Anything, "layout-1", "classic", "Test", "Desc",
		mock.Anything, "test-deployment").Return(int64(1), nil)

	err := suite.store.CreateLayout("layout-1", CreateLayoutRequest{
		Handle:      "classic",
		DisplayName: "Test",
		Description: "Desc",
		Layout:      json.RawMessage(`{"structure": "grid"}`),
	})

	assert.NoError(suite.T(), err)
}

// Test GetLayout - Success
func (suite *LayoutStoreTestSuite) TestGetLayout_Success() {
	results := []map[string]interface{}{
		{
			"id":           "layout-123",
			"handle":       "classic",
			"display_name": "Test Layout",
			"description":  "A test layout",
			"layout":       `{"structure": "centered"}`,
			"created_at":   "2024-01-15T10:30:00Z",
			"updated_at":   "2024-01-15T10:30:00Z",
		},
	}
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("Query", mock.Anything, "layout-123", "test-deployment").Return(results, nil)

	layout, err := suite.store.GetLayout("layout-123")

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "layout-123", layout.ID)
	assert.Equal(suite.T(), "Test Layout", layout.DisplayName)
}

// Test GetLayout - Not found
func (suite *LayoutStoreTestSuite) TestGetLayout_NotFound() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("Query", mock.Anything, "non-existent", "test-deployment").
		Return([]map[string]interface{}{}, nil)

	_, err := suite.store.GetLayout("non-existent")

	assert.Error(suite.T(), err)
	assert.True(suite.T(), errors.Is(err, errLayoutNotFound))
}

// Test GetLayout - Multiple results (unexpected)
func (suite *LayoutStoreTestSuite) TestGetLayout_MultipleResults() {
	results := []map[string]interface{}{
		{"id": "1", "display_name": "A", "description": "X", "layout": `{}`},
		{"id": "2", "display_name": "B", "description": "Y", "layout": `{}`},
	}
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("Query", mock.Anything, "layout-123", "test-deployment").Return(results, nil)

	_, err := suite.store.GetLayout("layout-123")

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "unexpected number of results")
}

// Test IsLayoutExist - Exists
func (suite *LayoutStoreTestSuite) TestIsLayoutExist_True() {
	results := []map[string]interface{}{
		{"total": int64(1)},
	}
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("Query", mock.Anything, "layout-123", "test-deployment").Return(results, nil)

	exists, err := suite.store.IsLayoutExist("layout-123")

	assert.NoError(suite.T(), err)
	assert.True(suite.T(), exists)
}

// Test IsLayoutExist - Not exists
func (suite *LayoutStoreTestSuite) TestIsLayoutExist_False() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("Query", mock.Anything, "non-existent", "test-deployment").
		Return([]map[string]interface{}{}, nil)

	exists, err := suite.store.IsLayoutExist("non-existent")

	assert.NoError(suite.T(), err)
	assert.False(suite.T(), exists)
}

// Test DeleteLayout - Success
func (suite *LayoutStoreTestSuite) TestDeleteLayout_Success() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("Execute", mock.Anything, "layout-123", "test-deployment").
		Return(int64(1), nil)

	err := suite.store.DeleteLayout("layout-123")

	assert.NoError(suite.T(), err)
}

// Test GetApplicationsCountByLayoutID - Success
func (suite *LayoutStoreTestSuite) TestGetApplicationsCountByLayoutID_Success() {
	results := []map[string]interface{}{
		{"total": int64(3)},
	}
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("Query", mock.Anything, "layout-123", "test-deployment").Return(results, nil)

	count, err := suite.store.GetApplicationsCountByLayoutID("layout-123")

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), 3, count)
}

// Test parseCountResult helper
func (suite *LayoutStoreTestSuite) TestParseCountResult_Int64() {
	results := []map[string]interface{}{{"total": int64(42)}}
	count, err := parseCountResult(results)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), 42, count)
}

func (suite *LayoutStoreTestSuite) TestParseCountResult_Int() {
	results := []map[string]interface{}{{"total": 10}}
	count, err := parseCountResult(results)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), 10, count)
}

func (suite *LayoutStoreTestSuite) TestParseCountResult_Empty() {
	results := []map[string]interface{}{}
	_, err := parseCountResult(results)
	assert.Error(suite.T(), err)
}

func (suite *LayoutStoreTestSuite) TestParseCountResult_MissingField() {
	results := []map[string]interface{}{{"count": int64(1)}}
	_, err := parseCountResult(results)
	assert.Error(suite.T(), err)
}

func (suite *LayoutStoreTestSuite) TestParseCountResult_UnsupportedType() {
	results := []map[string]interface{}{{"total": "invalid"}}
	_, err := parseCountResult(results)
	assert.Error(suite.T(), err)
}

// Test buildLayoutListItemFromResultRow helper
func (suite *LayoutStoreTestSuite) TestBuildLayoutListItemFromResultRow_Success() {
	row := map[string]interface{}{
		"id":           "layout-1",
		"handle":       "classic",
		"display_name": "Test Layout",
		"description":  "A description",
		"created_at":   "2024-01-15T10:30:00Z",
		"updated_at":   "2024-01-15T10:30:00Z",
	}

	layout, err := suite.store.buildLayoutListItemFromResultRow(row)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "layout-1", layout.ID)
	assert.Equal(suite.T(), "Test Layout", layout.DisplayName)
	assert.Equal(suite.T(), "A description", layout.Description)
}

func (suite *LayoutStoreTestSuite) TestBuildLayoutListItemFromResultRow_MissingID() {
	row := map[string]interface{}{
		"display_name": "Test",
		"description":  "Desc",
	}

	_, err := suite.store.buildLayoutListItemFromResultRow(row)
	assert.Error(suite.T(), err)
}

func (suite *LayoutStoreTestSuite) TestBuildLayoutListItemFromResultRow_MissingDisplayName() {
	row := map[string]interface{}{
		"id":          "layout-1",
		"handle":      "classic",
		"description": "Desc",
	}

	_, err := suite.store.buildLayoutListItemFromResultRow(row)
	assert.Error(suite.T(), err)
}

func (suite *LayoutStoreTestSuite) TestBuildLayoutListItemFromResultRow_NilDescription() {
	row := map[string]interface{}{
		"id":           "layout-1",
		"handle":       "classic",
		"display_name": "Test",
		"description":  nil,
		"created_at":   "2024-01-15T10:30:00Z",
		"updated_at":   "2024-01-15T10:30:00Z",
	}

	layout, err := suite.store.buildLayoutListItemFromResultRow(row)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "", layout.Description)
}

// Test buildLayoutFromResultRow helper
func (suite *LayoutStoreTestSuite) TestBuildLayoutFromResultRow_StringLayout() {
	row := map[string]interface{}{
		"id":           "layout-1",
		"display_name": "Test Layout",
		"description":  "A description",
		"layout":       `{"structure": "centered"}`,
		"created_at":   "2024-01-15T10:30:00Z",
		"updated_at":   "2024-01-15T10:30:00Z",
	}

	layout, err := suite.store.buildLayoutFromResultRow(row)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "layout-1", layout.ID)
	assert.NotNil(suite.T(), layout.Layout)
}

func (suite *LayoutStoreTestSuite) TestBuildLayoutFromResultRow_ByteLayout() {
	row := map[string]interface{}{
		"id":           "layout-1",
		"display_name": "Test Layout",
		"description":  "Desc",
		"layout":       []byte(`{"structure": "grid"}`),
		"created_at":   "2024-01-15T10:30:00Z",
		"updated_at":   "2024-01-15T10:30:00Z",
	}

	layout, err := suite.store.buildLayoutFromResultRow(row)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), layout.Layout)
}

func (suite *LayoutStoreTestSuite) TestBuildLayoutFromResultRow_MissingLayout() {
	row := map[string]interface{}{
		"id":           "layout-1",
		"display_name": "Test",
		"description":  "Desc",
	}

	_, err := suite.store.buildLayoutFromResultRow(row)
	assert.Error(suite.T(), err)
}

func (suite *LayoutStoreTestSuite) TestBuildLayoutFromResultRow_UnsupportedLayoutType() {
	row := map[string]interface{}{
		"id":           "layout-1",
		"display_name": "Test",
		"description":  "Desc",
		"layout":       12345,
	}

	_, err := suite.store.buildLayoutFromResultRow(row)
	assert.Error(suite.T(), err)
}

// Test IsLayoutHandleConflict - Conflict found
func (suite *LayoutStoreTestSuite) TestIsLayoutHandleConflict_Conflict() {
	results := []map[string]interface{}{
		{"total": int64(1)},
	}
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("Query", mock.Anything, "classic", "test-deployment", "").Return(results, nil)

	conflict, err := suite.store.IsLayoutHandleConflict("classic", "")

	assert.NoError(suite.T(), err)
	assert.True(suite.T(), conflict)
}

// Test IsLayoutHandleConflict - No conflict
func (suite *LayoutStoreTestSuite) TestIsLayoutHandleConflict_NoConflict() {
	results := []map[string]interface{}{
		{"total": int64(0)},
	}
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("Query", mock.Anything, "unique-handle", "test-deployment", "layout-1").Return(results, nil)

	conflict, err := suite.store.IsLayoutHandleConflict("unique-handle", "layout-1")

	assert.NoError(suite.T(), err)
	assert.False(suite.T(), conflict)
}

// Test IsLayoutHandleConflict - DB client error
func (suite *LayoutStoreTestSuite) TestIsLayoutHandleConflict_DBClientError() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(nil, errors.New("connection error"))

	conflict, err := suite.store.IsLayoutHandleConflict("classic", "")

	assert.Error(suite.T(), err)
	assert.False(suite.T(), conflict)
}

// Test IsLayoutHandleConflict - Query error
func (suite *LayoutStoreTestSuite) TestIsLayoutHandleConflict_QueryError() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("Query", mock.Anything, "classic", "test-deployment", "").
		Return(nil, errors.New("query error"))

	conflict, err := suite.store.IsLayoutHandleConflict("classic", "")

	assert.Error(suite.T(), err)
	assert.False(suite.T(), conflict)
}

// Test UpdateLayout - DB client error
func (suite *LayoutStoreTestSuite) TestUpdateLayout_DBClientError() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(nil, errors.New("connection error"))

	err := suite.store.UpdateLayout("layout-1", UpdateLayoutRequest{
		DisplayName: "Updated",
		Description: "Updated Desc",
		Layout:      json.RawMessage(`{"structure": "grid"}`),
	})

	assert.Error(suite.T(), err)
}

// Test DeleteLayout - DB client error
func (suite *LayoutStoreTestSuite) TestDeleteLayout_DBClientError() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(nil, errors.New("connection error"))

	err := suite.store.DeleteLayout("layout-1")

	assert.Error(suite.T(), err)
}
