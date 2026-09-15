// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package thememgt

import (
	"context"
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
	err := suite.store.CreateTheme(context.Background(), "theme-001", themeReq)

	// Assert
	suite.NoError(err)

	// Verify theme was created
	retrieved, err := suite.store.GetTheme(context.Background(), "theme-001")
	suite.NoError(err)
	suite.Equal("theme-001", retrieved.ID)
	suite.Equal("Blue Theme", retrieved.DisplayName)
	suite.Equal("Test theme", retrieved.Description)
	suite.NotEmpty(retrieved.Theme)
}

func (suite *ThemeFileBasedStoreTestSuite) TestGetTheme_Success() {
	// Arrange
	themeReq := suite.createTestTheme("Red Theme")
	_ = suite.store.CreateTheme(context.Background(), "theme-002", themeReq)

	// Act
	retrieved, err := suite.store.GetTheme(context.Background(), "theme-002")

	// Assert
	suite.NoError(err)
	suite.Equal("theme-002", retrieved.ID)
	suite.Equal("Red Theme", retrieved.DisplayName)
}

func (suite *ThemeFileBasedStoreTestSuite) TestGetTheme_NotFound() {
	// Act
	retrieved, err := suite.store.GetTheme(context.Background(), "non-existent")

	// Assert
	suite.Error(err)
	suite.Empty(retrieved.ID)
}

func (suite *ThemeFileBasedStoreTestSuite) TestGetThemeList_Success() {
	// Arrange
	theme1 := suite.createTestTheme("Theme 1")
	theme2 := suite.createTestTheme("Theme 2")
	theme3 := suite.createTestTheme("Theme 3")
	_ = suite.store.CreateTheme(context.Background(), "theme-003", theme1)
	_ = suite.store.CreateTheme(context.Background(), "theme-004", theme2)
	_ = suite.store.CreateTheme(context.Background(), "theme-005", theme3)

	// Act
	themes, err := suite.store.GetThemeList(context.Background(), 10, 0)

	// Assert
	suite.NoError(err)
	suite.Len(themes, 3)
}

func (suite *ThemeFileBasedStoreTestSuite) TestGetThemeList_WithPagination() {
	// Arrange
	for i := 1; i <= 5; i++ {
		themeReq := suite.createTestTheme(fmt.Sprintf("Theme %d", i))
		_ = suite.store.CreateTheme(context.Background(), fmt.Sprintf("theme-%03d", i), themeReq)
	}

	// Act - Get first 2 themes
	themes, err := suite.store.GetThemeList(context.Background(), 2, 0)

	// Assert
	suite.NoError(err)
	suite.Len(themes, 2)

	// Act - Get next 2 themes
	themes, err = suite.store.GetThemeList(context.Background(), 2, 2)

	// Assert
	suite.NoError(err)
	suite.Len(themes, 2)
}

func (suite *ThemeFileBasedStoreTestSuite) TestGetThemeList_EmptyStore() {
	// Act
	themes, err := suite.store.GetThemeList(context.Background(), 10, 0)

	// Assert
	suite.NoError(err)
	suite.Empty(themes)
}

func (suite *ThemeFileBasedStoreTestSuite) TestGetThemeListCount_Success() {
	// Arrange
	theme1 := suite.createTestTheme("Theme 6")
	theme2 := suite.createTestTheme("Theme 7")
	_ = suite.store.CreateTheme(context.Background(), "theme-006", theme1)
	_ = suite.store.CreateTheme(context.Background(), "theme-007", theme2)

	// Act
	count, err := suite.store.GetThemeListCount(context.Background())

	// Assert
	suite.NoError(err)
	suite.Equal(2, count)
}

func (suite *ThemeFileBasedStoreTestSuite) TestIsThemeExist_True() {
	// Arrange
	themeReq := suite.createTestTheme("Existing Theme")
	_ = suite.store.CreateTheme(context.Background(), "theme-008", themeReq)

	// Act
	exists, err := suite.store.IsThemeExist(context.Background(), "theme-008")

	// Assert
	suite.NoError(err)
	suite.True(exists)
}

func (suite *ThemeFileBasedStoreTestSuite) TestIsThemeExist_False() {
	// Act
	exists, err := suite.store.IsThemeExist(context.Background(), "non-existent")

	// Assert
	suite.NoError(err)
	suite.False(exists)
}

func (suite *ThemeFileBasedStoreTestSuite) TestUpdateTheme_NotSupported() {
	// Arrange
	themeReq := suite.createTestTheme("Update Test")

	// Act
	err := suite.store.UpdateTheme(context.Background(), "theme-009", UpdateThemeRequest{
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
	err := suite.store.DeleteTheme(context.Background(), "theme-001")

	// Assert
	suite.Error(err)
	suite.Contains(err.Error(), "not supported")
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
	retrieved, err := suite.store.GetTheme(context.Background(), "theme-010")
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
	_ = suite.store.CreateTheme(context.Background(), "theme-a", theme1)
	_ = suite.store.CreateTheme(context.Background(), "theme-b", theme2)

	// Act - negative offset should be clamped to 0
	themes, err := suite.store.GetThemeList(context.Background(), 10, -5)

	// Assert
	suite.NoError(err)
	suite.Len(themes, 2) // Should return all themes
}

func (suite *ThemeFileBasedStoreTestSuite) TestGetThemeList_ZeroLimit() {
	// Arrange
	theme1 := suite.createTestTheme("Theme C")
	_ = suite.store.CreateTheme(context.Background(), "theme-c", theme1)

	// Act - zero limit should return empty slice
	themes, err := suite.store.GetThemeList(context.Background(), 0, 0)

	// Assert
	suite.NoError(err)
	suite.Empty(themes)
}

func (suite *ThemeFileBasedStoreTestSuite) TestGetThemeList_NegativeLimit() {
	// Arrange
	theme1 := suite.createTestTheme("Theme D")
	_ = suite.store.CreateTheme(context.Background(), "theme-d", theme1)

	// Act - negative limit should return empty slice
	themes, err := suite.store.GetThemeList(context.Background(), -10, 0)

	// Assert
	suite.NoError(err)
	suite.Empty(themes)
}

func (suite *ThemeFileBasedStoreTestSuite) TestGetThemeList_OffsetBeyondList() {
	// Arrange
	theme1 := suite.createTestTheme("Theme E")
	_ = suite.store.CreateTheme(context.Background(), "theme-e", theme1)

	// Act - offset beyond list length should return empty slice
	themes, err := suite.store.GetThemeList(context.Background(), 10, 100)

	// Assert
	suite.NoError(err)
	suite.Empty(themes)
}

func (suite *ThemeFileBasedStoreTestSuite) TestIsThemeHandleConflict_Conflict() {
	// Arrange
	themeReq := suite.createTestTheme("Handle Conflict Test")
	themeReq.Handle = "conflict-handle"
	_ = suite.store.CreateTheme(context.Background(), "theme-hc1", themeReq)

	// Act - different ID with same handle should conflict
	conflict, err := suite.store.IsThemeHandleConflict(context.Background(), "conflict-handle", "other-id")

	// Assert
	suite.NoError(err)
	suite.True(conflict)
}

func (suite *ThemeFileBasedStoreTestSuite) TestIsThemeHandleConflict_NoConflict() {
	// Act - non-existent handle should not conflict
	conflict, err := suite.store.IsThemeHandleConflict(context.Background(), "non-existent-handle", "")

	// Assert
	suite.NoError(err)
	suite.False(conflict)
}

func (suite *ThemeFileBasedStoreTestSuite) TestIsThemeHandleConflict_SameIDExcluded() {
	// Arrange
	themeReq := suite.createTestTheme("Same ID Exclude Test")
	themeReq.Handle = "same-id-handle"
	_ = suite.store.CreateTheme(context.Background(), "theme-hc2", themeReq)

	// Act - same ID should be excluded from conflict check
	conflict, err := suite.store.IsThemeHandleConflict(context.Background(), "same-id-handle", "theme-hc2")

	// Assert
	suite.NoError(err)
	suite.False(conflict)
}
