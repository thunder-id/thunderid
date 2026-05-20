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

package thememgt

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

// ThemeStoreTestSuite contains tests for the theme store.
type ThemeStoreTestSuite struct {
	suite.Suite
	mockDBProvider *providermock.DBProviderInterfaceMock
	mockDBClient   *providermock.DBClientInterfaceMock
	store          *themeMgtStore
}

func TestThemeStoreTestSuite(t *testing.T) {
	suite.Run(t, new(ThemeStoreTestSuite))
}

func (suite *ThemeStoreTestSuite) SetupTest() {
	suite.mockDBProvider = providermock.NewDBProviderInterfaceMock(suite.T())
	suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())
	suite.store = &themeMgtStore{
		dbProvider:   suite.mockDBProvider,
		deploymentID: "test-deployment",
	}
}

// Test GetThemeListCount - Success
func (suite *ThemeStoreTestSuite) TestGetThemeListCount_Success() {
	results := []map[string]interface{}{
		{"total": int64(5)},
	}
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("Query", mock.Anything, "test-deployment").Return(results, nil)

	count, err := suite.store.GetThemeListCount()

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), 5, count)
}

// Test GetThemeListCount - DB client error
func (suite *ThemeStoreTestSuite) TestGetThemeListCount_DBClientError() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(nil, errors.New("connection error"))

	count, err := suite.store.GetThemeListCount()

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), 0, count)
}

// Test GetThemeListCount - Query error
func (suite *ThemeStoreTestSuite) TestGetThemeListCount_QueryError() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("Query", mock.Anything, "test-deployment").
		Return(nil, errors.New("query error"))

	count, err := suite.store.GetThemeListCount()

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), 0, count)
}

// Test GetThemeList - Success
func (suite *ThemeStoreTestSuite) TestGetThemeList_Success() {
	results := []map[string]interface{}{
		{
			"id":           "theme-1",
			"handle":       "theme-one",
			"display_name": "Theme 1",
			"description":  "Description 1",
			"theme": `{"defaultColorScheme":"light",` +
				`"colorSchemes":{"light":{"palette":{"primary":{"main":"#ff7300"}}}}}`,
			"created_at": "2024-01-15T10:30:00Z",
			"updated_at": "2024-01-15T10:30:00Z",
		},
		{
			"id":           "theme-2",
			"handle":       "theme-two",
			"display_name": "Theme 2",
			"description":  "Description 2",
			"theme": `{"defaultColorScheme":"dark",` +
				`"colorSchemes":{"dark":{"palette":{"primary":{"main":"#bb86fc"}}}}}`,
			"created_at": "2024-01-15T10:30:00Z",
			"updated_at": "2024-01-15T10:30:00Z",
		},
	}
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("Query", mock.Anything, 10, 0, "test-deployment").Return(results, nil)

	themes, err := suite.store.GetThemeList(10, 0)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), themes, 2)
	assert.Equal(suite.T(), "theme-1", themes[0].ID)
	assert.Equal(suite.T(), "Theme 1", themes[0].DisplayName)
	assert.NotNil(suite.T(), themes[0].Theme)
}

// Test GetThemeList - DB client error
func (suite *ThemeStoreTestSuite) TestGetThemeList_DBClientError() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(nil, errors.New("connection error"))

	themes, err := suite.store.GetThemeList(10, 0)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), themes)
}

// Test CreateTheme - Success
func (suite *ThemeStoreTestSuite) TestCreateTheme_Success() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("Execute", mock.Anything, "theme-1", "classic", "Test", "Desc",
		mock.Anything, "test-deployment").Return(int64(1), nil)

	err := suite.store.CreateTheme("theme-1", CreateThemeRequest{
		Handle:      "classic",
		DisplayName: "Test",
		Description: "Desc",
		Theme:       json.RawMessage(`{"colors": {"primary": "#007bff"}}`),
	})

	assert.NoError(suite.T(), err)
}

// Test GetTheme - Success
func (suite *ThemeStoreTestSuite) TestGetTheme_Success() {
	results := []map[string]interface{}{
		{
			"id":           "theme-123",
			"handle":       "classic",
			"display_name": "Test Theme",
			"description":  "A test theme",
			"theme":        `{"colors": {"primary": "#007bff"}}`,
			"created_at":   "2024-01-15T10:30:00Z",
			"updated_at":   "2024-01-15T10:30:00Z",
		},
	}
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("Query", mock.Anything, "theme-123", "test-deployment").Return(results, nil)

	theme, err := suite.store.GetTheme("theme-123")

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "theme-123", theme.ID)
	assert.Equal(suite.T(), "Test Theme", theme.DisplayName)
}

// Test GetTheme - Not found
func (suite *ThemeStoreTestSuite) TestGetTheme_NotFound() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("Query", mock.Anything, "non-existent", "test-deployment").
		Return([]map[string]interface{}{}, nil)

	_, err := suite.store.GetTheme("non-existent")

	assert.Error(suite.T(), err)
	assert.True(suite.T(), errors.Is(err, errThemeNotFound))
}

// Test GetTheme - Multiple results (unexpected)
func (suite *ThemeStoreTestSuite) TestGetTheme_MultipleResults() {
	results := []map[string]interface{}{
		{"id": "1", "display_name": "A", "description": "X", "theme": `{}`},
		{"id": "2", "display_name": "B", "description": "Y", "theme": `{}`},
	}
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("Query", mock.Anything, "theme-123", "test-deployment").Return(results, nil)

	_, err := suite.store.GetTheme("theme-123")

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "unexpected number of results")
}

// Test IsThemeExist - Exists
func (suite *ThemeStoreTestSuite) TestIsThemeExist_True() {
	results := []map[string]interface{}{
		{"total": int64(1)},
	}
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("Query", mock.Anything, "theme-123", "test-deployment").Return(results, nil)

	exists, err := suite.store.IsThemeExist("theme-123")

	assert.NoError(suite.T(), err)
	assert.True(suite.T(), exists)
}

// Test IsThemeExist - Not exists
func (suite *ThemeStoreTestSuite) TestIsThemeExist_False() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("Query", mock.Anything, "non-existent", "test-deployment").
		Return([]map[string]interface{}{}, nil)

	exists, err := suite.store.IsThemeExist("non-existent")

	assert.NoError(suite.T(), err)
	assert.False(suite.T(), exists)
}

// Test DeleteTheme - Success
func (suite *ThemeStoreTestSuite) TestDeleteTheme_Success() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("Execute", mock.Anything, "theme-123", "test-deployment").
		Return(int64(1), nil)

	err := suite.store.DeleteTheme("theme-123")

	assert.NoError(suite.T(), err)
}

// Test GetApplicationsCountByThemeID - Success
func (suite *ThemeStoreTestSuite) TestGetApplicationsCountByThemeID_Success() {
	results := []map[string]interface{}{
		{"total": int64(3)},
	}
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("Query", mock.Anything, "theme-123", "test-deployment").Return(results, nil)

	count, err := suite.store.GetApplicationsCountByThemeID("theme-123")

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), 3, count)
}

// Test parseCountResult helper
func (suite *ThemeStoreTestSuite) TestParseCountResult_Int64() {
	results := []map[string]interface{}{{"total": int64(42)}}
	count, err := parseCountResult(results)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), 42, count)
}

func (suite *ThemeStoreTestSuite) TestParseCountResult_Int() {
	results := []map[string]interface{}{{"total": 10}}
	count, err := parseCountResult(results)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), 10, count)
}

func (suite *ThemeStoreTestSuite) TestParseCountResult_Float64() {
	results := []map[string]interface{}{{"total": float64(7)}}
	count, err := parseCountResult(results)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), 7, count)
}

func (suite *ThemeStoreTestSuite) TestParseCountResult_CountField() {
	results := []map[string]interface{}{{"count": int64(3)}}
	count, err := parseCountResult(results)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), 3, count)
}

func (suite *ThemeStoreTestSuite) TestParseCountResult_Empty() {
	results := []map[string]interface{}{}
	_, err := parseCountResult(results)
	assert.Error(suite.T(), err)
}

func (suite *ThemeStoreTestSuite) TestParseCountResult_MissingField() {
	results := []map[string]interface{}{{"other": int64(1)}}
	_, err := parseCountResult(results)
	assert.Error(suite.T(), err)
}

func (suite *ThemeStoreTestSuite) TestParseCountResult_UnsupportedType() {
	results := []map[string]interface{}{{"total": "invalid"}}
	_, err := parseCountResult(results)
	assert.Error(suite.T(), err)
}

// Test buildThemeListItemFromResultRow helper
func (suite *ThemeStoreTestSuite) TestBuildThemeListItemFromResultRow_Success() {
	row := map[string]interface{}{
		"id":           "theme-1",
		"handle":       "classic",
		"display_name": "Test Theme",
		"description":  "A description",
		"created_at":   "2024-01-15T10:30:00Z",
		"updated_at":   "2024-01-15T10:30:00Z",
	}

	theme, err := suite.store.buildThemeListItemFromResultRow(row)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "theme-1", theme.ID)
	assert.Equal(suite.T(), "Test Theme", theme.DisplayName)
	assert.Equal(suite.T(), "A description", theme.Description)
}

func (suite *ThemeStoreTestSuite) TestBuildThemeListItemFromResultRow_MissingID() {
	row := map[string]interface{}{
		"display_name": "Test",
		"description":  "Desc",
	}

	_, err := suite.store.buildThemeListItemFromResultRow(row)
	assert.Error(suite.T(), err)
}

func (suite *ThemeStoreTestSuite) TestBuildThemeListItemFromResultRow_MissingHandle() {
	row := map[string]interface{}{
		"id":           "theme-1",
		"display_name": "Test",
		"description":  "Desc",
	}

	_, err := suite.store.buildThemeListItemFromResultRow(row)
	assert.Error(suite.T(), err)
}

func (suite *ThemeStoreTestSuite) TestBuildThemeListItemFromResultRow_MissingDisplayName() {
	row := map[string]interface{}{
		"id":          "theme-1",
		"handle":      "classic",
		"description": "Desc",
	}

	_, err := suite.store.buildThemeListItemFromResultRow(row)
	assert.Error(suite.T(), err)
}

func (suite *ThemeStoreTestSuite) TestBuildThemeListItemFromResultRow_MissingDescription() {
	row := map[string]interface{}{
		"id":           "theme-1",
		"handle":       "classic",
		"display_name": "Test",
	}

	_, err := suite.store.buildThemeListItemFromResultRow(row)
	assert.Error(suite.T(), err)
}

func (suite *ThemeStoreTestSuite) TestBuildThemeListItemFromResultRow_WithThemeString() {
	row := map[string]interface{}{
		"id":           "theme-1",
		"handle":       "classic",
		"display_name": "Test Theme",
		"description":  "A description",
		"theme": `{"defaultColorScheme":"light",` +
			`"colorSchemes":{"light":{"palette":{"primary":{"main":"#ff7300"}}}}}`,
		"created_at": "2024-01-15T10:30:00Z",
		"updated_at": "2024-01-15T10:30:00Z",
	}

	theme, err := suite.store.buildThemeListItemFromResultRow(row)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), theme.Theme)
}

func (suite *ThemeStoreTestSuite) TestBuildThemeListItemFromResultRow_WithThemeBytes() {
	row := map[string]interface{}{
		"id":           "theme-1",
		"handle":       "classic",
		"display_name": "Test Theme",
		"description":  "A description",
		"theme":        []byte(`{"defaultColorScheme":"light"}`),
		"created_at":   "2024-01-15T10:30:00Z",
		"updated_at":   "2024-01-15T10:30:00Z",
	}

	theme, err := suite.store.buildThemeListItemFromResultRow(row)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), theme.Theme)
}

func (suite *ThemeStoreTestSuite) TestBuildThemeListItemFromResultRow_WithoutThemeColumn() {
	row := map[string]interface{}{
		"id":           "theme-1",
		"handle":       "classic",
		"display_name": "Test Theme",
		"description":  "A description",
		"created_at":   "2024-01-15T10:30:00Z",
		"updated_at":   "2024-01-15T10:30:00Z",
	}

	theme, err := suite.store.buildThemeListItemFromResultRow(row)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), theme.Theme)
}

func (suite *ThemeStoreTestSuite) TestBuildThemeListItemFromResultRow_NullThemeColumn() {
	row := map[string]interface{}{
		"id":           "theme-1",
		"handle":       "classic",
		"display_name": "Test Theme",
		"description":  "A description",
		"theme":        nil,
		"created_at":   "2024-01-15T10:30:00Z",
		"updated_at":   "2024-01-15T10:30:00Z",
	}

	theme, err := suite.store.buildThemeListItemFromResultRow(row)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), theme.Theme)
}

// Test buildThemeFromResultRow helper
func (suite *ThemeStoreTestSuite) TestBuildThemeFromResultRow_StringTheme() {
	row := map[string]interface{}{
		"id":           "theme-1",
		"handle":       "classic",
		"display_name": "Test Theme",
		"description":  "A description",
		"theme":        `{"colors": {"primary": "#007bff"}}`,
		"created_at":   "2024-01-15T10:30:00Z",
		"updated_at":   "2024-01-15T10:30:00Z",
	}

	theme, err := suite.store.buildThemeFromResultRow(row)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "theme-1", theme.ID)
	assert.NotNil(suite.T(), theme.Theme)
}

func (suite *ThemeStoreTestSuite) TestBuildThemeFromResultRow_ByteTheme() {
	row := map[string]interface{}{
		"id":           "theme-1",
		"handle":       "classic",
		"display_name": "Test Theme",
		"description":  "Desc",
		"theme":        []byte(`{"colors": {}}`),
		"created_at":   "2024-01-15T10:30:00Z",
		"updated_at":   "2024-01-15T10:30:00Z",
	}

	theme, err := suite.store.buildThemeFromResultRow(row)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), theme.Theme)
}

func (suite *ThemeStoreTestSuite) TestBuildThemeFromResultRow_MissingTheme() {
	row := map[string]interface{}{
		"id":           "theme-1",
		"display_name": "Test",
		"description":  "Desc",
	}

	_, err := suite.store.buildThemeFromResultRow(row)
	assert.Error(suite.T(), err)
}

func (suite *ThemeStoreTestSuite) TestBuildThemeFromResultRow_UnsupportedThemeType() {
	row := map[string]interface{}{
		"id":           "theme-1",
		"display_name": "Test",
		"description":  "Desc",
		"theme":        12345,
	}

	_, err := suite.store.buildThemeFromResultRow(row)
	assert.Error(suite.T(), err)
}

// Test IsThemeHandleConflict - Conflict found
func (suite *ThemeStoreTestSuite) TestIsThemeHandleConflict_Conflict() {
	results := []map[string]interface{}{
		{"total": int64(1)},
	}
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("Query", mock.Anything, "classic", "test-deployment", "").Return(results, nil)

	conflict, err := suite.store.IsThemeHandleConflict("classic", "")

	assert.NoError(suite.T(), err)
	assert.True(suite.T(), conflict)
}

// Test IsThemeHandleConflict - No conflict
func (suite *ThemeStoreTestSuite) TestIsThemeHandleConflict_NoConflict() {
	results := []map[string]interface{}{
		{"total": int64(0)},
	}
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("Query", mock.Anything, "unique-handle", "test-deployment", "theme-1").Return(results, nil)

	conflict, err := suite.store.IsThemeHandleConflict("unique-handle", "theme-1")

	assert.NoError(suite.T(), err)
	assert.False(suite.T(), conflict)
}

// Test IsThemeHandleConflict - DB client error
func (suite *ThemeStoreTestSuite) TestIsThemeHandleConflict_DBClientError() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(nil, errors.New("connection error"))

	conflict, err := suite.store.IsThemeHandleConflict("classic", "")

	assert.Error(suite.T(), err)
	assert.False(suite.T(), conflict)
}

// Test IsThemeHandleConflict - Query error
func (suite *ThemeStoreTestSuite) TestIsThemeHandleConflict_QueryError() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("Query", mock.Anything, "classic", "test-deployment", "").
		Return(nil, errors.New("query error"))

	conflict, err := suite.store.IsThemeHandleConflict("classic", "")

	assert.Error(suite.T(), err)
	assert.False(suite.T(), conflict)
}

// Test UpdateTheme - DB client error
func (suite *ThemeStoreTestSuite) TestUpdateTheme_DBClientError() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(nil, errors.New("connection error"))

	err := suite.store.UpdateTheme("theme-1", UpdateThemeRequest{
		DisplayName: "Updated",
		Description: "Updated Desc",
		Theme:       json.RawMessage(`{"colors": {}}`),
	})

	assert.Error(suite.T(), err)
}

// Test DeleteTheme - DB client error
func (suite *ThemeStoreTestSuite) TestDeleteTheme_DBClientError() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(nil, errors.New("connection error"))

	err := suite.store.DeleteTheme("theme-1")

	assert.Error(suite.T(), err)
}

func (suite *ThemeStoreTestSuite) TestBuildThemeFromResultRow_OptionalDescription() {
	row := map[string]interface{}{
		"id":           "theme-1",
		"handle":       "classic",
		"display_name": "Test Theme",
		"theme":        `{"colors": {}}`,
		"created_at":   "2024-01-15T10:30:00Z",
		"updated_at":   "2024-01-15T10:30:00Z",
	}

	theme, err := suite.store.buildThemeFromResultRow(row)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "", theme.Description)
}
