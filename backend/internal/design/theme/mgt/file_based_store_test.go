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
	"fmt"
	"testing"

	"github.com/thunder-id/thunderid/internal/system/config"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"

	"github.com/stretchr/testify/suite"
)

// ThemeFileBasedStoreTestSuite contains comprehensive tests for the file-based theme store.
type ThemeFileBasedStoreTestSuite struct {
	suite.Suite
	store *themeFileBasedStore
}

// TestThemeFileBasedStoreTestSuite runs the file-based store test suite.
func TestThemeFileBasedStoreTestSuite(t *testing.T) {
	suite.Run(t, new(ThemeFileBasedStoreTestSuite))
}

func (suite *ThemeFileBasedStoreTestSuite) SetupSuite() {
	// Create temporary directory for tests
	tempDir := suite.T().TempDir()

	// Initialize server runtime once for all tests
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime(tempDir, testConfig)
	suite.Require().NoError(err, "Failed to initialize server runtime")
}

func (suite *ThemeFileBasedStoreTestSuite) TearDownSuite() {
	// Clean up server runtime after all tests
	config.ResetServerRuntime()
}

func (suite *ThemeFileBasedStoreTestSuite) SetupTest() {
	genericStore := declarativeresource.NewGenericFileBasedStoreForTest(entity.KeyTypeTheme)
	suite.store = &themeFileBasedStore{
		GenericFileBasedStore: genericStore,
	}
}

func (suite *ThemeFileBasedStoreTestSuite) createTestTheme(displayName string) CreateThemeRequest {
	themeConfig := map[string]interface{}{
		"primaryColor":    "#1976d2",
		"secondaryColor":  "#dc004e",
		"backgroundColor": "#ffffff",
	}
	themeJSON, err := json.Marshal(themeConfig)
	suite.Require().NoError(err, "Failed to marshal test theme config")

	return CreateThemeRequest{
		DisplayName: displayName,
		Description: "Test theme",
		Theme:       themeJSON,
	}
}

func (suite *ThemeFileBasedStoreTestSuite) TestCreateTheme_Success() {
	// Arrange
	themeReq := suite.createTestTheme("Blue Theme")

	// Act
	err := suite.store.CreateTheme("theme-001", themeReq)

	// Assert
	suite.NoError(err)

	// Verify theme was created
	retrieved, err := suite.store.GetTheme("theme-001")
	suite.NoError(err)
	suite.Equal("theme-001", retrieved.ID)
	suite.Equal("Blue Theme", retrieved.DisplayName)
	suite.Equal("Test theme", retrieved.Description)
	suite.NotEmpty(retrieved.Theme)
}

func (suite *ThemeFileBasedStoreTestSuite) TestGetTheme_Success() {
	// Arrange
	themeReq := suite.createTestTheme("Red Theme")
	_ = suite.store.CreateTheme("theme-002", themeReq)

	// Act
	retrieved, err := suite.store.GetTheme("theme-002")

	// Assert
	suite.NoError(err)
	suite.Equal("theme-002", retrieved.ID)
	suite.Equal("Red Theme", retrieved.DisplayName)
}

func (suite *ThemeFileBasedStoreTestSuite) TestGetTheme_NotFound() {
	// Act
	retrieved, err := suite.store.GetTheme("non-existent")

	// Assert
	suite.Error(err)
	suite.Empty(retrieved.ID)
}

func (suite *ThemeFileBasedStoreTestSuite) TestGetThemeList_Success() {
	// Arrange
	theme1 := suite.createTestTheme("Theme 1")
	theme2 := suite.createTestTheme("Theme 2")
	theme3 := suite.createTestTheme("Theme 3")
	_ = suite.store.CreateTheme("theme-003", theme1)
	_ = suite.store.CreateTheme("theme-004", theme2)
	_ = suite.store.CreateTheme("theme-005", theme3)

	// Act
	themes, err := suite.store.GetThemeList(10, 0)

	// Assert
	suite.NoError(err)
	suite.Len(themes, 3)
}

func (suite *ThemeFileBasedStoreTestSuite) TestGetThemeList_WithPagination() {
	// Arrange
	for i := 1; i <= 5; i++ {
		themeReq := suite.createTestTheme(fmt.Sprintf("Theme %d", i))
		_ = suite.store.CreateTheme(fmt.Sprintf("theme-%03d", i), themeReq)
	}

	// Act - Get first 2 themes
	themes, err := suite.store.GetThemeList(2, 0)

	// Assert
	suite.NoError(err)
	suite.Len(themes, 2)

	// Act - Get next 2 themes
	themes, err = suite.store.GetThemeList(2, 2)

	// Assert
	suite.NoError(err)
	suite.Len(themes, 2)
}

func (suite *ThemeFileBasedStoreTestSuite) TestGetThemeList_EmptyStore() {
	// Act
	themes, err := suite.store.GetThemeList(10, 0)

	// Assert
	suite.NoError(err)
	suite.Empty(themes)
}

func (suite *ThemeFileBasedStoreTestSuite) TestGetThemeListCount_Success() {
	// Arrange
	theme1 := suite.createTestTheme("Theme 6")
	theme2 := suite.createTestTheme("Theme 7")
	_ = suite.store.CreateTheme("theme-006", theme1)
	_ = suite.store.CreateTheme("theme-007", theme2)

	// Act
	count, err := suite.store.GetThemeListCount()

	// Assert
	suite.NoError(err)
	suite.Equal(2, count)
}

func (suite *ThemeFileBasedStoreTestSuite) TestIsThemeExist_True() {
	// Arrange
	themeReq := suite.createTestTheme("Existing Theme")
	_ = suite.store.CreateTheme("theme-008", themeReq)

	// Act
	exists, err := suite.store.IsThemeExist("theme-008")

	// Assert
	suite.NoError(err)
	suite.True(exists)
}

func (suite *ThemeFileBasedStoreTestSuite) TestIsThemeExist_False() {
	// Act
	exists, err := suite.store.IsThemeExist("non-existent")

	// Assert
	suite.NoError(err)
	suite.False(exists)
}

func (suite *ThemeFileBasedStoreTestSuite) TestUpdateTheme_NotSupported() {
	// Arrange
	themeReq := suite.createTestTheme("Update Test")

	// Act
	err := suite.store.UpdateTheme("theme-009", UpdateThemeRequest{
		DisplayName: "Updated Name",
		Description: "Updated description",
		Theme:       themeReq.Theme,
	})

	// Assert
	suite.Error(err)
	suite.Contains(err.Error(), "not supported")
}

func (suite *ThemeFileBasedStoreTestSuite) TestDeleteTheme_NotSupported() {
	// Act
	err := suite.store.DeleteTheme("theme-001")

	// Assert
	suite.Error(err)
	suite.Contains(err.Error(), "not supported")
}

func (suite *ThemeFileBasedStoreTestSuite) TestGetApplicationsCountByThemeID() {
	// Act
	count, err := suite.store.GetApplicationsCountByThemeID("theme-001")

	// Assert
	suite.NoError(err)
	suite.Equal(0, count)
}

func (suite *ThemeFileBasedStoreTestSuite) TestCreate_StorerInterface() {
	// Arrange
	themeConfig := map[string]interface{}{
		"primaryColor": "#00ff00",
	}
	themeJSON, _ := json.Marshal(themeConfig)
	theme := &Theme{
		ID:          "theme-010",
		DisplayName: "Green Theme",
		Description: "Test",
		Theme:       themeJSON,
	}

	// Act
	err := suite.store.Create("theme-010", theme)

	// Assert
	suite.NoError(err)

	// Verify
	retrieved, err := suite.store.GetTheme("theme-010")
	suite.NoError(err)
	suite.Equal("theme-010", retrieved.ID)
}

func (suite *ThemeFileBasedStoreTestSuite) TestCreate_InvalidType() {
	// Arrange - pass wrong type to Create
	invalidData := "not a theme"

	// Act
	err := suite.store.Create("theme-invalid", invalidData)

	// Assert
	suite.Error(err)
	suite.Contains(err.Error(), "invalid data type")
}

func (suite *ThemeFileBasedStoreTestSuite) TestGetThemeList_NegativeOffset() {
	// Arrange
	theme1 := suite.createTestTheme("Theme A")
	theme2 := suite.createTestTheme("Theme B")
	_ = suite.store.CreateTheme("theme-a", theme1)
	_ = suite.store.CreateTheme("theme-b", theme2)

	// Act - negative offset should be clamped to 0
	themes, err := suite.store.GetThemeList(10, -5)

	// Assert
	suite.NoError(err)
	suite.Len(themes, 2) // Should return all themes
}

func (suite *ThemeFileBasedStoreTestSuite) TestGetThemeList_ZeroLimit() {
	// Arrange
	theme1 := suite.createTestTheme("Theme C")
	_ = suite.store.CreateTheme("theme-c", theme1)

	// Act - zero limit should return empty slice
	themes, err := suite.store.GetThemeList(0, 0)

	// Assert
	suite.NoError(err)
	suite.Empty(themes)
}

func (suite *ThemeFileBasedStoreTestSuite) TestGetThemeList_NegativeLimit() {
	// Arrange
	theme1 := suite.createTestTheme("Theme D")
	_ = suite.store.CreateTheme("theme-d", theme1)

	// Act - negative limit should return empty slice
	themes, err := suite.store.GetThemeList(-10, 0)

	// Assert
	suite.NoError(err)
	suite.Empty(themes)
}

func (suite *ThemeFileBasedStoreTestSuite) TestGetThemeList_OffsetBeyondList() {
	// Arrange
	theme1 := suite.createTestTheme("Theme E")
	_ = suite.store.CreateTheme("theme-e", theme1)

	// Act - offset beyond list length should return empty slice
	themes, err := suite.store.GetThemeList(10, 100)

	// Assert
	suite.NoError(err)
	suite.Empty(themes)
}

func (suite *ThemeFileBasedStoreTestSuite) TestIsThemeHandleConflict_Conflict() {
	// Arrange
	themeReq := suite.createTestTheme("Handle Conflict Test")
	themeReq.Handle = "conflict-handle"
	_ = suite.store.CreateTheme("theme-hc1", themeReq)

	// Act - different ID with same handle should conflict
	conflict, err := suite.store.IsThemeHandleConflict("conflict-handle", "other-id")

	// Assert
	suite.NoError(err)
	suite.True(conflict)
}

func (suite *ThemeFileBasedStoreTestSuite) TestIsThemeHandleConflict_NoConflict() {
	// Act - non-existent handle should not conflict
	conflict, err := suite.store.IsThemeHandleConflict("non-existent-handle", "")

	// Assert
	suite.NoError(err)
	suite.False(conflict)
}

func (suite *ThemeFileBasedStoreTestSuite) TestIsThemeHandleConflict_SameIDExcluded() {
	// Arrange
	themeReq := suite.createTestTheme("Same ID Exclude Test")
	themeReq.Handle = "same-id-handle"
	_ = suite.store.CreateTheme("theme-hc2", themeReq)

	// Act - same ID should be excluded from conflict check
	conflict, err := suite.store.IsThemeHandleConflict("same-id-handle", "theme-hc2")

	// Assert
	suite.NoError(err)
	suite.False(conflict)
}
