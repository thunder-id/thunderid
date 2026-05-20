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

	"github.com/thunder-id/thunderid/internal/system/config"
)

// Test Suite
type ThemeServiceTestSuite struct {
	suite.Suite
	mockStore *themeMgtStoreInterfaceMock
	service   ThemeMgtServiceInterface
}

func TestThemeServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ThemeServiceTestSuite))
}

func (suite *ThemeServiceTestSuite) SetupTest() {
	// Initialize config runtime with default values
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	if err != nil {
		suite.Fail("Failed to initialize runtime", err)
	}

	suite.mockStore = newThemeMgtStoreInterfaceMock(suite.T())
	suite.service = newThemeMgtService(suite.mockStore)
}

// Test GetThemeList - Success
func (suite *ThemeServiceTestSuite) TestGetThemeList_Success() {
	themes := []Theme{
		{
			ID:          "theme-1",
			DisplayName: "Classic Theme",
			Description: "A classic theme",
			Theme:       json.RawMessage(`{"colors": {"primary": "#007bff"}}`),
		},
		{
			ID:          "theme-2",
			DisplayName: "Dark Theme",
			Description: "A dark theme",
			Theme:       json.RawMessage(`{"colors": {"primary": "#000000"}}`),
		},
	}

	suite.mockStore.On("GetThemeListCount").Return(2, nil)
	suite.mockStore.On("GetThemeList", 10, 0).Return(themes, nil)

	result, err := suite.service.GetThemeList(10, 0)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), 2, result.TotalResults)
	assert.Equal(suite.T(), 2, result.Count)
	assert.Equal(suite.T(), 1, result.StartIndex)
	assert.Len(suite.T(), result.Themes, 2)
}

// Test GetThemeList - Store Count Error
func (suite *ThemeServiceTestSuite) TestGetThemeList_CountError() {
	suite.mockStore.On("GetThemeListCount").Return(0, errors.New("database error"))

	result, err := suite.service.GetThemeList(10, 0)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
}

// Test GetThemeList - Store Error
func (suite *ThemeServiceTestSuite) TestGetThemeList_StoreError() {
	suite.mockStore.On("GetThemeListCount").Return(2, nil)
	suite.mockStore.On("GetThemeList", 10, 0).Return(nil, errors.New("database error"))

	result, err := suite.service.GetThemeList(10, 0)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
}

// Test GetThemeList - Invalid Pagination
func (suite *ThemeServiceTestSuite) TestGetThemeList_InvalidLimit() {
	result, err := suite.service.GetThemeList(-1, 0)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), "THM-1008", err.Code)
}

func (suite *ThemeServiceTestSuite) TestGetThemeList_InvalidOffset() {
	result, err := suite.service.GetThemeList(10, -1)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), "THM-1009", err.Code)
}

// Test CreateTheme - Success
func (suite *ThemeServiceTestSuite) TestCreateTheme_Success() {
	themeRequest := CreateThemeRequestWithID{
		Handle:      "new-theme",
		DisplayName: "New Theme",
		Description: "A new theme",
		Theme:       json.RawMessage(`{"colors": {"primary": "#ff0000"}}`),
	}
	storeReq := CreateThemeRequest{
		Handle:      themeRequest.Handle,
		DisplayName: themeRequest.DisplayName,
		Description: themeRequest.Description,
		Theme:       themeRequest.Theme,
	}

	suite.mockStore.On("IsThemeHandleConflict", "new-theme", "").Return(false, nil)
	suite.mockStore.On("CreateTheme", mock.AnythingOfType("string"), storeReq).Return(nil)

	result, err := suite.service.CreateTheme(themeRequest)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "new-theme", result.Handle)
	assert.Equal(suite.T(), "New Theme", result.DisplayName)
	assert.Equal(suite.T(), "A new theme", result.Description)
	assert.NotEmpty(suite.T(), result.ID)
}

// Test CreateTheme - Missing Display Name
func (suite *ThemeServiceTestSuite) TestCreateTheme_MissingDisplayName() {
	themeRequest := CreateThemeRequestWithID{
		Handle:      "my-theme",
		DisplayName: "",
		Description: "A theme without name",
		Theme:       json.RawMessage(`{"colors": {"primary": "#ff0000"}}`),
	}

	result, err := suite.service.CreateTheme(themeRequest)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), "THM-1005", err.Code)
}

// Test CreateTheme - Missing Handle
func (suite *ThemeServiceTestSuite) TestCreateTheme_MissingHandle() {
	themeRequest := CreateThemeRequestWithID{
		Handle:      "",
		DisplayName: "My Theme",
		Description: "A theme without handle",
		Theme:       json.RawMessage(`{"colors": {"primary": "#ff0000"}}`),
	}

	result, err := suite.service.CreateTheme(themeRequest)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), "THM-1016", err.Code)
}

// Test CreateTheme - Duplicate Handle
func (suite *ThemeServiceTestSuite) TestCreateTheme_DuplicateHandle() {
	themeRequest := CreateThemeRequestWithID{
		Handle:      "existing-theme",
		DisplayName: "My Theme",
		Description: "A theme with duplicate handle",
		Theme:       json.RawMessage(`{"colors": {"primary": "#ff0000"}}`),
	}

	suite.mockStore.On("IsThemeHandleConflict", "existing-theme", "").Return(true, nil)

	result, err := suite.service.CreateTheme(themeRequest)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), "THM-1015", err.Code)
}

// Test CreateTheme - Declarative mode enabled
func (suite *ThemeServiceTestSuite) TestCreateTheme_DeclarativeModeEnabled() {
	runtime := config.GetServerRuntime()
	runtime.Config.Theme.Store = "declarative"

	themeRequest := CreateThemeRequestWithID{
		Handle:      "declarative-theme",
		DisplayName: "Declarative Theme",
		Description: "Should be blocked",
		Theme:       json.RawMessage(`{"colors": {"primary": "#ff0000"}}`),
	}

	result, err := suite.service.CreateTheme(themeRequest)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), "THM-1014", err.Code)
}

// Test CreateTheme - Invalid Theme JSON
func (suite *ThemeServiceTestSuite) TestCreateTheme_InvalidJSON() {
	themeRequest := CreateThemeRequestWithID{
		Handle:      "my-theme",
		DisplayName: "Theme",
		Description: "Invalid JSON theme",
		Theme:       json.RawMessage(`{invalid json}`),
	}

	suite.mockStore.On("IsThemeHandleConflict", "my-theme", "").Return(false, nil)

	result, err := suite.service.CreateTheme(themeRequest)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), "THM-1007", err.Code)
}

// Test CreateTheme - Store Error
func (suite *ThemeServiceTestSuite) TestCreateTheme_StoreError() {
	themeRequest := CreateThemeRequestWithID{
		Handle:      "my-theme",
		DisplayName: "Theme",
		Description: "A theme",
		Theme:       json.RawMessage(`{"colors": {"primary": "#ff0000"}}`),
	}
	storeReq := CreateThemeRequest{
		Handle:      themeRequest.Handle,
		DisplayName: themeRequest.DisplayName,
		Description: themeRequest.Description,
		Theme:       themeRequest.Theme,
	}

	suite.mockStore.On("IsThemeHandleConflict", "my-theme", "").Return(false, nil)
	suite.mockStore.On("CreateTheme", mock.AnythingOfType("string"), storeReq).Return(errors.New("database error"))

	result, err := suite.service.CreateTheme(themeRequest)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
}

// Test GetTheme - Success
func (suite *ThemeServiceTestSuite) TestGetTheme_Success() {
	theme := Theme{
		ID:          "theme-123",
		DisplayName: "Test Theme",
		Description: "A test theme",
		Theme:       json.RawMessage(`{"colors": {"primary": "#007bff"}}`),
	}

	suite.mockStore.On("GetTheme", "theme-123").Return(theme, nil)

	result, err := suite.service.GetTheme("theme-123")

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "theme-123", result.ID)
	assert.Equal(suite.T(), "Test Theme", result.DisplayName)
}

// Test GetTheme - Invalid ID
func (suite *ThemeServiceTestSuite) TestGetTheme_InvalidID() {
	result, err := suite.service.GetTheme("")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), "THM-1002", err.Code)
}

// Test GetTheme - Not Found
func (suite *ThemeServiceTestSuite) TestGetTheme_NotFound() {
	suite.mockStore.On("GetTheme", "non-existent").Return(Theme{}, errThemeNotFound)

	result, err := suite.service.GetTheme("non-existent")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), "THM-1003", err.Code)
}

// Test GetTheme - Store Error
func (suite *ThemeServiceTestSuite) TestGetTheme_StoreError() {
	suite.mockStore.On("GetTheme", "theme-123").Return(Theme{}, errors.New("database error"))

	result, err := suite.service.GetTheme("theme-123")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
}

// Test UpdateTheme - Success
func (suite *ThemeServiceTestSuite) TestUpdateTheme_Success() {
	updateRequest := UpdateThemeRequest{
		Handle:      "my-theme",
		DisplayName: "Updated Theme",
		Description: "An updated theme",
		Theme:       json.RawMessage(`{"colors": {"primary": "#00ff00"}}`),
	}
	existingTheme := Theme{
		ID:     "theme-123",
		Handle: "my-theme",
	}

	suite.mockStore.On("IsThemeDeclarative", "theme-123").Return(false)
	suite.mockStore.On("GetTheme", "theme-123").Return(existingTheme, nil)
	suite.mockStore.On("UpdateTheme", "theme-123", updateRequest).Return(nil)

	result, err := suite.service.UpdateTheme("theme-123", updateRequest)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "theme-123", result.ID)
	assert.Equal(suite.T(), "my-theme", result.Handle)
	assert.Equal(suite.T(), "Updated Theme", result.DisplayName)
}

// Test UpdateTheme - Omitted Handle uses existing handle
func (suite *ThemeServiceTestSuite) TestUpdateTheme_OmittedHandle_UsesExisting() {
	updateRequest := UpdateThemeRequest{
		Handle:      "",
		DisplayName: "Updated Theme",
		Description: "An updated theme",
		Theme:       json.RawMessage(`{"colors": {"primary": "#00ff00"}}`),
	}
	existingTheme := Theme{
		ID:     "theme-123",
		Handle: "existing-handle",
	}

	suite.mockStore.On("IsThemeDeclarative", "theme-123").Return(false)
	suite.mockStore.On("GetTheme", "theme-123").Return(existingTheme, nil)
	suite.mockStore.On("UpdateTheme", "theme-123", updateRequest).Return(nil)

	result, err := suite.service.UpdateTheme("theme-123", updateRequest)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "existing-handle", result.Handle)
}

// Test UpdateTheme - Invalid ID
func (suite *ThemeServiceTestSuite) TestUpdateTheme_InvalidID() {
	updateRequest := UpdateThemeRequest{
		Handle:      "my-theme",
		DisplayName: "Theme",
		Description: "A theme",
		Theme:       json.RawMessage(`{"colors": {}}`),
	}

	result, err := suite.service.UpdateTheme("", updateRequest)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), "THM-1002", err.Code)
}

// Test UpdateTheme - Missing Display Name
func (suite *ThemeServiceTestSuite) TestUpdateTheme_MissingDisplayName() {
	updateRequest := UpdateThemeRequest{
		Handle:      "my-theme",
		DisplayName: "",
		Description: "A theme",
		Theme:       json.RawMessage(`{"colors": {}}`),
	}

	result, err := suite.service.UpdateTheme("theme-123", updateRequest)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), "THM-1005", err.Code)
}

// Test UpdateTheme - Immutable Handle
func (suite *ThemeServiceTestSuite) TestUpdateTheme_ImmutableHandle() {
	updateRequest := UpdateThemeRequest{
		Handle:      "different-handle",
		DisplayName: "Theme",
		Description: "A theme",
		Theme:       json.RawMessage(`{"colors": {}}`),
	}
	existingTheme := Theme{
		ID:     "theme-123",
		Handle: "my-theme",
	}

	suite.mockStore.On("IsThemeDeclarative", "theme-123").Return(false)
	suite.mockStore.On("GetTheme", "theme-123").Return(existingTheme, nil)

	result, err := suite.service.UpdateTheme("theme-123", updateRequest)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), "THM-1017", err.Code)
}

// Test UpdateTheme - Not Found
func (suite *ThemeServiceTestSuite) TestUpdateTheme_NotFound() {
	updateRequest := UpdateThemeRequest{
		Handle:      "my-theme",
		DisplayName: "Theme",
		Description: "A theme",
		Theme:       json.RawMessage(`{"colors": {}}`),
	}

	suite.mockStore.On("IsThemeDeclarative", "non-existent").Return(false)
	suite.mockStore.On("GetTheme", "non-existent").Return(Theme{}, errThemeNotFound)

	result, err := suite.service.UpdateTheme("non-existent", updateRequest)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), "THM-1003", err.Code)
}

// Test UpdateTheme - Invalid JSON
func (suite *ThemeServiceTestSuite) TestUpdateTheme_InvalidJSON() {
	updateRequest := UpdateThemeRequest{
		Handle:      "my-theme",
		DisplayName: "Theme",
		Description: "A theme",
		Theme:       json.RawMessage(`{invalid}`),
	}
	existingTheme := Theme{
		ID:     "theme-123",
		Handle: "my-theme",
	}
	suite.mockStore.On("IsThemeDeclarative", "theme-123").Return(false)
	suite.mockStore.On("GetTheme", "theme-123").Return(existingTheme, nil)
	result, err := suite.service.UpdateTheme("theme-123", updateRequest)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), "THM-1007", err.Code)
}

// Test DeleteTheme - Success
func (suite *ThemeServiceTestSuite) TestDeleteTheme_Success() {
	suite.mockStore.On("IsThemeDeclarative", "theme-123").Return(false)
	suite.mockStore.On("IsThemeExist", "theme-123").Return(true, nil)
	suite.mockStore.On("GetApplicationsCountByThemeID", "theme-123").Return(0, nil)
	suite.mockStore.On("DeleteTheme", "theme-123").Return(nil)

	err := suite.service.DeleteTheme("theme-123")

	assert.Nil(suite.T(), err)
}

// Test DeleteTheme - Invalid ID
func (suite *ThemeServiceTestSuite) TestDeleteTheme_InvalidID() {
	err := suite.service.DeleteTheme("")

	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), "THM-1002", err.Code)
}

// Test DeleteTheme - Not Found (idempotent delete returns success)
func (suite *ThemeServiceTestSuite) TestDeleteTheme_NotFound() {
	suite.mockStore.On("IsThemeDeclarative", "non-existent").Return(false)
	suite.mockStore.On("IsThemeExist", "non-existent").Return(false, nil)

	err := suite.service.DeleteTheme("non-existent")

	assert.Nil(suite.T(), err)
}

// Test DeleteTheme - Theme In Use
func (suite *ThemeServiceTestSuite) TestDeleteTheme_InUse() {
	suite.mockStore.On("IsThemeDeclarative", "theme-123").Return(false)
	suite.mockStore.On("IsThemeExist", "theme-123").Return(true, nil)
	suite.mockStore.On("GetApplicationsCountByThemeID", "theme-123").Return(3, nil)

	err := suite.service.DeleteTheme("theme-123")

	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), "THM-1004", err.Code)
}

// Test DeleteTheme - Store Error
func (suite *ThemeServiceTestSuite) TestDeleteTheme_StoreError() {
	suite.mockStore.On("IsThemeDeclarative", "theme-123").Return(false)
	suite.mockStore.On("IsThemeExist", "theme-123").Return(true, nil)
	suite.mockStore.On("GetApplicationsCountByThemeID", "theme-123").Return(0, nil)
	suite.mockStore.On("DeleteTheme", "theme-123").Return(errors.New("database error"))

	err := suite.service.DeleteTheme("theme-123")

	assert.NotNil(suite.T(), err)
}

// Test IsThemeExist - Exists
func (suite *ThemeServiceTestSuite) TestIsThemeExist_True() {
	suite.mockStore.On("IsThemeExist", "theme-123").Return(true, nil)

	exists, err := suite.service.IsThemeExist("theme-123")

	assert.Nil(suite.T(), err)
	assert.True(suite.T(), exists)
}

// Test IsThemeExist - Not Exists
func (suite *ThemeServiceTestSuite) TestIsThemeExist_False() {
	suite.mockStore.On("IsThemeExist", "non-existent").Return(false, nil)

	exists, err := suite.service.IsThemeExist("non-existent")

	assert.Nil(suite.T(), err)
	assert.False(suite.T(), exists)
}

// Test IsThemeExist - Store Error
func (suite *ThemeServiceTestSuite) TestIsThemeExist_StoreError() {
	suite.mockStore.On("IsThemeExist", "theme-123").Return(false, errors.New("database error"))

	exists, err := suite.service.IsThemeExist("theme-123")

	assert.NotNil(suite.T(), err)
	assert.False(suite.T(), exists)
}

// Test CreateTheme - Handle conflict check error
func (suite *ThemeServiceTestSuite) TestCreateTheme_HandleConflictError() {
	themeRequest := CreateThemeRequestWithID{
		Handle:      "my-theme",
		DisplayName: "Theme",
		Description: "A theme",
		Theme:       json.RawMessage(`{"colors": {}}`),
	}

	suite.mockStore.On("IsThemeHandleConflict", "my-theme", "").Return(false, errors.New("database error"))

	result, err := suite.service.CreateTheme(themeRequest)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
}

// Test UpdateTheme - GetTheme store error
func (suite *ThemeServiceTestSuite) TestUpdateTheme_GetThemeError() {
	updateRequest := UpdateThemeRequest{
		Handle:      "my-theme",
		DisplayName: "Theme",
		Description: "A theme",
		Theme:       json.RawMessage(`{"colors": {}}`),
	}

	suite.mockStore.On("IsThemeDeclarative", "theme-123").Return(false)
	suite.mockStore.On("GetTheme", "theme-123").Return(Theme{}, errors.New("database error"))

	result, err := suite.service.UpdateTheme("theme-123", updateRequest)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
}

// Test DeleteTheme - Applications count error
func (suite *ThemeServiceTestSuite) TestDeleteTheme_ApplicationsCountError() {
	suite.mockStore.On("IsThemeDeclarative", "theme-123").Return(false)
	suite.mockStore.On("IsThemeExist", "theme-123").Return(true, nil)
	suite.mockStore.On("GetApplicationsCountByThemeID", "theme-123").Return(0, errors.New("database error"))

	err := suite.service.DeleteTheme("theme-123")

	assert.NotNil(suite.T(), err)
}
