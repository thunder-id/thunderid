// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package layoutmgt

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

// LayoutFileBasedStoreTestSuite contains comprehensive tests for the file-based layout store.
type LayoutFileBasedStoreTestSuite struct {
	suite.Suite
	store *layoutFileBasedStore
}

// TestLayoutFileBasedStoreTestSuite runs the file-based store test suite.
func TestLayoutFileBasedStoreTestSuite(t *testing.T) {
	suite.Run(t, new(LayoutFileBasedStoreTestSuite))
}

func (suite *LayoutFileBasedStoreTestSuite) SetupSuite() {
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

func (suite *LayoutFileBasedStoreTestSuite) TearDownSuite() {
	// Clean up server runtime after all tests
	config.ResetServerRuntime()
}

func (suite *LayoutFileBasedStoreTestSuite) SetupTest() {
	genericStore := declarativeresource.NewGenericFileBasedStoreForTest(entity.KeyTypeLayout)
	suite.store = &layoutFileBasedStore{
		GenericFileBasedStore: genericStore,
	}
}

func (suite *LayoutFileBasedStoreTestSuite) createTestLayout(displayName string) CreateLayoutRequest {
	layoutConfig := map[string]interface{}{
		"type": "centered",
		"components": []map[string]string{
			{"type": "logo", "position": "top"},
			{"type": "form", "position": "center"},
		},
	}
	layoutJSON, err := json.Marshal(layoutConfig)
	suite.Require().NoError(err, "Failed to marshal test layout config")

	return CreateLayoutRequest{
		DisplayName: displayName,
		Description: "Test layout",
		Layout:      layoutJSON,
	}
}

func (suite *LayoutFileBasedStoreTestSuite) TestCreateLayout_Success() {
	// Arrange
	layoutReq := suite.createTestLayout("Centered Layout")

	// Act
	err := suite.store.CreateLayout(context.Background(), "layout-001", layoutReq)

	// Assert
	suite.NoError(err)

	// Verify layout was created
	retrieved, err := suite.store.GetLayout(context.Background(), "layout-001")
	suite.NoError(err)
	suite.Equal("layout-001", retrieved.ID)
	suite.Equal("Centered Layout", retrieved.DisplayName)
	suite.Equal("Test layout", retrieved.Description)
	suite.NotEmpty(retrieved.Layout)
}

func (suite *LayoutFileBasedStoreTestSuite) TestGetLayout_Success() {
	// Arrange
	layoutReq := suite.createTestLayout("Sidebar Layout")
	_ = suite.store.CreateLayout(context.Background(), "layout-002", layoutReq)

	// Act
	retrieved, err := suite.store.GetLayout(context.Background(), "layout-002")

	// Assert
	suite.NoError(err)
	suite.Equal("layout-002", retrieved.ID)
	suite.Equal("Sidebar Layout", retrieved.DisplayName)
}

func (suite *LayoutFileBasedStoreTestSuite) TestGetLayout_NotFound() {
	// Act
	retrieved, err := suite.store.GetLayout(context.Background(), "non-existent")

	// Assert
	suite.Error(err)
	suite.Empty(retrieved.ID)
}

func (suite *LayoutFileBasedStoreTestSuite) TestGetLayoutList_Success() {
	// Arrange
	layout1 := suite.createTestLayout("Layout 1")
	layout2 := suite.createTestLayout("Layout 2")
	layout3 := suite.createTestLayout("Layout 3")
	_ = suite.store.CreateLayout(context.Background(), "layout-003", layout1)
	_ = suite.store.CreateLayout(context.Background(), "layout-004", layout2)
	_ = suite.store.CreateLayout(context.Background(), "layout-005", layout3)

	// Act
	layouts, err := suite.store.GetLayoutList(context.Background(), 10, 0)

	// Assert
	suite.NoError(err)
	suite.Len(layouts, 3)
}

func (suite *LayoutFileBasedStoreTestSuite) TestGetLayoutList_WithPagination() {
	// Arrange
	for i := 1; i <= 5; i++ {
		layoutReq := suite.createTestLayout(fmt.Sprintf("Layout %d", i))
		_ = suite.store.CreateLayout(context.Background(), fmt.Sprintf("layout-%03d", i), layoutReq)
	}

	// Act - Get first 2 layouts
	layouts, err := suite.store.GetLayoutList(context.Background(), 2, 0)

	// Assert
	suite.NoError(err)
	suite.Len(layouts, 2)

	// Act - Get next 2 layouts
	layouts, err = suite.store.GetLayoutList(context.Background(), 2, 2)

	// Assert
	suite.NoError(err)
	suite.Len(layouts, 2)
}

func (suite *LayoutFileBasedStoreTestSuite) TestGetLayoutList_EmptyStore() {
	// Act
	layouts, err := suite.store.GetLayoutList(context.Background(), 10, 0)

	// Assert
	suite.NoError(err)
	suite.Empty(layouts)
}

func (suite *LayoutFileBasedStoreTestSuite) TestGetLayoutListCount_Success() {
	// Arrange
	layout1 := suite.createTestLayout("Layout 6")
	layout2 := suite.createTestLayout("Layout 7")
	_ = suite.store.CreateLayout(context.Background(), "layout-006", layout1)
	_ = suite.store.CreateLayout(context.Background(), "layout-007", layout2)

	// Act
	count, err := suite.store.GetLayoutListCount(context.Background())

	// Assert
	suite.NoError(err)
	suite.Equal(2, count)
}

func (suite *LayoutFileBasedStoreTestSuite) TestIsLayoutExist_True() {
	// Arrange
	layoutReq := suite.createTestLayout("Existing Layout")
	_ = suite.store.CreateLayout(context.Background(), "layout-008", layoutReq)

	// Act
	exists, err := suite.store.IsLayoutExist(context.Background(), "layout-008")

	// Assert
	suite.NoError(err)
	suite.True(exists)
}

func (suite *LayoutFileBasedStoreTestSuite) TestIsLayoutExist_False() {
	// Act
	exists, err := suite.store.IsLayoutExist(context.Background(), "non-existent")

	// Assert
	suite.NoError(err)
	suite.False(exists)
}

func (suite *LayoutFileBasedStoreTestSuite) TestUpdateLayout_NotSupported() {
	// Arrange
	layoutReq := suite.createTestLayout("Update Test")

	// Act
	err := suite.store.UpdateLayout(context.Background(), "layout-009", UpdateLayoutRequest{
		DisplayName: "Updated Name",
		Description: "Updated description",
		Layout:      layoutReq.Layout,
	})

	// Assert
	suite.Error(err)
	suite.Contains(err.Error(), "not supported")
}

func (suite *LayoutFileBasedStoreTestSuite) TestDeleteLayout_NotSupported() {
	// Act
	err := suite.store.DeleteLayout(context.Background(), "layout-001")

	// Assert
	suite.Error(err)
	suite.Contains(err.Error(), "not supported")
}

func (suite *LayoutFileBasedStoreTestSuite) TestCreate_StorerInterface() {
	// Arrange
	layoutConfig := map[string]interface{}{
		"type": "fullscreen",
	}
	layoutJSON, _ := json.Marshal(layoutConfig)
	layout := &Layout{
		ID:          "layout-010",
		DisplayName: "Fullscreen Layout",
		Description: "Test",
		Layout:      layoutJSON,
	}

	// Act
	err := suite.store.Create("layout-010", layout)

	// Assert
	suite.NoError(err)

	// Verify
	retrieved, err := suite.store.GetLayout(context.Background(), "layout-010")
	suite.NoError(err)
	suite.Equal("layout-010", retrieved.ID)
}

func (suite *LayoutFileBasedStoreTestSuite) TestCreate_InvalidType() {
	// Arrange - pass wrong type to Create
	invalidData := "not a layout"

	// Act
	err := suite.store.Create("layout-invalid", invalidData)

	// Assert
	suite.Error(err)
	suite.Contains(err.Error(), "invalid data type")
}

func (suite *LayoutFileBasedStoreTestSuite) TestGetLayoutList_NegativeOffset() {
	// Arrange
	layout1 := suite.createTestLayout("Layout A")
	layout2 := suite.createTestLayout("Layout B")
	_ = suite.store.CreateLayout(context.Background(), "layout-a", layout1)
	_ = suite.store.CreateLayout(context.Background(), "layout-b", layout2)

	// Act - negative offset should be clamped to 0
	layouts, err := suite.store.GetLayoutList(context.Background(), 10, -5)

	// Assert
	suite.NoError(err)
	suite.Len(layouts, 2) // Should return all layouts
}

func (suite *LayoutFileBasedStoreTestSuite) TestGetLayoutList_ZeroLimit() {
	// Arrange
	layout1 := suite.createTestLayout("Layout C")
	_ = suite.store.CreateLayout(context.Background(), "layout-c", layout1)

	// Act - zero limit should return empty slice
	layouts, err := suite.store.GetLayoutList(context.Background(), 0, 0)

	// Assert
	suite.NoError(err)
	suite.Empty(layouts)
}

func (suite *LayoutFileBasedStoreTestSuite) TestGetLayoutList_NegativeLimit() {
	// Arrange
	layout1 := suite.createTestLayout("Layout D")
	_ = suite.store.CreateLayout(context.Background(), "layout-d", layout1)

	// Act - negative limit should return empty slice
	layouts, err := suite.store.GetLayoutList(context.Background(), -10, 0)

	// Assert
	suite.NoError(err)
	suite.Empty(layouts)
}

func (suite *LayoutFileBasedStoreTestSuite) TestGetLayoutList_OffsetBeyondList() {
	// Arrange
	layout1 := suite.createTestLayout("Layout E")
	_ = suite.store.CreateLayout(context.Background(), "layout-e", layout1)

	// Act - offset beyond list length should return empty slice
	layouts, err := suite.store.GetLayoutList(context.Background(), 10, 100)

	// Assert
	suite.NoError(err)
	suite.Empty(layouts)
}

func (suite *LayoutFileBasedStoreTestSuite) TestIsLayoutHandleConflict_Conflict() {
	// Arrange
	layoutReq := suite.createTestLayout("Handle Conflict Test")
	layoutReq.Handle = "conflict-handle"
	_ = suite.store.CreateLayout(context.Background(), "layout-hc1", layoutReq)

	// Act - different ID with same handle should conflict
	conflict, err := suite.store.IsLayoutHandleConflict(context.Background(), "conflict-handle", "other-id")

	// Assert
	suite.NoError(err)
	suite.True(conflict)
}

func (suite *LayoutFileBasedStoreTestSuite) TestIsLayoutHandleConflict_NoConflict() {
	// Act - non-existent handle should not conflict
	conflict, err := suite.store.IsLayoutHandleConflict(context.Background(), "non-existent-handle", "")

	// Assert
	suite.NoError(err)
	suite.False(conflict)
}

func (suite *LayoutFileBasedStoreTestSuite) TestIsLayoutHandleConflict_SameIDExcluded() {
	// Arrange
	layoutReq := suite.createTestLayout("Same ID Exclude Test")
	layoutReq.Handle = "same-id-handle"
	_ = suite.store.CreateLayout(context.Background(), "layout-hc2", layoutReq)

	// Act - same ID should be excluded from conflict check
	conflict, err := suite.store.IsLayoutHandleConflict(context.Background(), "same-id-handle", "layout-hc2")

	// Assert
	suite.NoError(err)
	suite.False(conflict)
}
