// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package thememgt

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
)

// Test Suite
type ThemeServiceTestSuite struct {
	suite.Suite
	mockStore    *themeMgtStoreInterfaceMock
	mockProjects *projectCheckerMock
	service      ThemeMgtServiceInterface
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
	suite.mockProjects = newProjectCheckerMock(suite.T())
	suite.service = newThemeMgtService(suite.mockStore, discardingBus{}, suite.mockProjects,
		allowAllAuthz(suite.T()))
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

	suite.mockStore.On("GetThemeListCount", testCtx()).Return(2, nil)
	suite.mockStore.On("GetThemeList", testCtx(), 10, 0).Return(themes, nil)

	result, err := suite.service.GetThemeList(testCtx(), 10, 0)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), 2, result.TotalResults)
	assert.Equal(suite.T(), 2, result.Count)
	assert.Equal(suite.T(), 1, result.StartIndex)
	assert.Len(suite.T(), result.Themes, 2)
}

// Test GetThemeList - Store Count Error
func (suite *ThemeServiceTestSuite) TestGetThemeList_CountError() {
	suite.mockStore.On("GetThemeListCount", testCtx()).Return(0, errors.New("database error"))

	result, err := suite.service.GetThemeList(testCtx(), 10, 0)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
}

// Test GetThemeList - Store Error
func (suite *ThemeServiceTestSuite) TestGetThemeList_StoreError() {
	suite.mockStore.On("GetThemeListCount", testCtx()).Return(2, nil)
	suite.mockStore.On("GetThemeList", testCtx(), 10, 0).Return(nil, errors.New("database error"))

	result, err := suite.service.GetThemeList(testCtx(), 10, 0)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
}

// Test GetThemeList - Invalid Pagination
func (suite *ThemeServiceTestSuite) TestGetThemeList_InvalidLimit() {
	result, err := suite.service.GetThemeList(testCtx(), -1, 0)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), "THM-1008", err.Code)
}

func (suite *ThemeServiceTestSuite) TestGetThemeList_InvalidOffset() {
	result, err := suite.service.GetThemeList(testCtx(), 10, -1)

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

	suite.mockStore.On("IsThemeHandleConflict", testCtx(), "new-theme", "").Return(false, nil)
	suite.mockStore.On("CreateTheme", testCtx(), mock.AnythingOfType("string"), storeReq).Return(nil)

	service := newThemeMgtService(suite.mockStore, discardingBus{}, nil, allowAllAuthz(suite.T()))
	result, err := service.CreateTheme(testCtx(), themeRequest)

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

	result, err := suite.service.CreateTheme(testCtx(), themeRequest)

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

	result, err := suite.service.CreateTheme(testCtx(), themeRequest)

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

	suite.mockStore.On("IsThemeHandleConflict", testCtx(), "existing-theme", "").Return(true, nil)

	result, err := suite.service.CreateTheme(testCtx(), themeRequest)

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

	result, err := suite.service.CreateTheme(testCtx(), themeRequest)

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

	suite.mockStore.On("IsThemeHandleConflict", testCtx(), "my-theme", "").Return(false, nil)

	result, err := suite.service.CreateTheme(testCtx(), themeRequest)

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

	suite.mockStore.On("IsThemeHandleConflict", testCtx(), "my-theme", "").Return(false, nil)
	suite.mockStore.On("CreateTheme", testCtx(), mock.AnythingOfType("string"),
		storeReq).Return(errors.New("database error"))

	service := newThemeMgtService(suite.mockStore, discardingBus{}, nil, allowAllAuthz(suite.T()))
	result, err := service.CreateTheme(testCtx(), themeRequest)

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

	suite.mockStore.On("GetTheme", testCtx(), "theme-123").Return(theme, nil)

	result, err := suite.service.GetTheme(testCtx(), "theme-123")

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "theme-123", result.ID)
	assert.Equal(suite.T(), "Test Theme", result.DisplayName)
}

// Test GetTheme - Invalid ID
func (suite *ThemeServiceTestSuite) TestGetTheme_InvalidID() {
	result, err := suite.service.GetTheme(testCtx(), "")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), "THM-1002", err.Code)
}

// Test GetTheme - Not Found
func (suite *ThemeServiceTestSuite) TestGetTheme_NotFound() {
	suite.mockStore.On("GetTheme", testCtx(), "non-existent").Return(Theme{}, errThemeNotFound)

	result, err := suite.service.GetTheme(testCtx(), "non-existent")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), "THM-1003", err.Code)
}

// Test GetTheme - Store Error
func (suite *ThemeServiceTestSuite) TestGetTheme_StoreError() {
	suite.mockStore.On("GetTheme", testCtx(), "theme-123").Return(Theme{}, errors.New("database error"))

	result, err := suite.service.GetTheme(testCtx(), "theme-123")

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

	suite.mockStore.On("IsThemeDeclarative", testCtx(), "theme-123").Return(false)
	suite.mockStore.On("GetTheme", testCtx(), "theme-123").Return(existingTheme, nil)
	suite.mockStore.On("UpdateTheme", testCtx(), "theme-123", updateRequest).Return(nil)

	result, err := suite.service.UpdateTheme(testCtx(), "theme-123", updateRequest)

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

	suite.mockStore.On("IsThemeDeclarative", testCtx(), "theme-123").Return(false)
	suite.mockStore.On("GetTheme", testCtx(), "theme-123").Return(existingTheme, nil)
	suite.mockStore.On("UpdateTheme", testCtx(), "theme-123", updateRequest).Return(nil)

	result, err := suite.service.UpdateTheme(testCtx(), "theme-123", updateRequest)

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

	result, err := suite.service.UpdateTheme(testCtx(), "", updateRequest)

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

	result, err := suite.service.UpdateTheme(testCtx(), "theme-123", updateRequest)

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

	suite.mockStore.On("IsThemeDeclarative", testCtx(), "theme-123").Return(false)
	suite.mockStore.On("GetTheme", testCtx(), "theme-123").Return(existingTheme, nil)

	result, err := suite.service.UpdateTheme(testCtx(), "theme-123", updateRequest)

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

	suite.mockStore.On("IsThemeDeclarative", testCtx(), "non-existent").Return(false)
	suite.mockStore.On("GetTheme", testCtx(), "non-existent").Return(Theme{}, errThemeNotFound)

	result, err := suite.service.UpdateTheme(testCtx(), "non-existent", updateRequest)

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
	suite.mockStore.On("IsThemeDeclarative", testCtx(), "theme-123").Return(false)
	suite.mockStore.On("GetTheme", testCtx(), "theme-123").Return(existingTheme, nil)
	result, err := suite.service.UpdateTheme(testCtx(), "theme-123", updateRequest)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), "THM-1007", err.Code)
}

// Test DeleteTheme - Success
func (suite *ThemeServiceTestSuite) TestDeleteTheme_Success() {
	suite.mockStore.On("IsThemeDeclarative", testCtx(), "theme-123").Return(false)
	suite.mockStore.On("IsThemeExist", testCtx(), "theme-123").Return(true, nil)
	suite.mockStore.On("DeleteTheme", testCtx(), "theme-123").Return(nil)

	err := suite.service.DeleteTheme(testCtx(), "theme-123")

	assert.Nil(suite.T(), err)
}

// Test DeleteTheme - Invalid ID
func (suite *ThemeServiceTestSuite) TestDeleteTheme_InvalidID() {
	err := suite.service.DeleteTheme(testCtx(), "")

	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), "THM-1002", err.Code)
}

// Test DeleteTheme - Not Found (idempotent delete returns success)
func (suite *ThemeServiceTestSuite) TestDeleteTheme_NotFound() {
	suite.mockStore.On("IsThemeDeclarative", testCtx(), "non-existent").Return(false)
	suite.mockStore.On("IsThemeExist", testCtx(), "non-existent").Return(false, nil)

	err := suite.service.DeleteTheme(testCtx(), "non-existent")

	assert.Nil(suite.T(), err)
}

// Test DeleteTheme - Store Error
func (suite *ThemeServiceTestSuite) TestDeleteTheme_StoreError() {
	suite.mockStore.On("IsThemeDeclarative", testCtx(), "theme-123").Return(false)
	suite.mockStore.On("IsThemeExist", testCtx(), "theme-123").Return(true, nil)
	suite.mockStore.On("DeleteTheme", testCtx(), "theme-123").Return(errors.New("database error"))

	err := suite.service.DeleteTheme(testCtx(), "theme-123")

	assert.NotNil(suite.T(), err)
}

// Test IsThemeExist - Exists
func (suite *ThemeServiceTestSuite) TestIsThemeExist_True() {
	suite.mockStore.On("IsThemeExist", testCtx(), "theme-123").Return(true, nil)

	exists, err := suite.service.IsThemeExist(testCtx(), "theme-123")

	assert.Nil(suite.T(), err)
	assert.True(suite.T(), exists)
}

// Test IsThemeExist - Not Exists
func (suite *ThemeServiceTestSuite) TestIsThemeExist_False() {
	suite.mockStore.On("IsThemeExist", testCtx(), "non-existent").Return(false, nil)

	exists, err := suite.service.IsThemeExist(testCtx(), "non-existent")

	assert.Nil(suite.T(), err)
	assert.False(suite.T(), exists)
}

// Test IsThemeExist - Store Error
func (suite *ThemeServiceTestSuite) TestIsThemeExist_StoreError() {
	suite.mockStore.On("IsThemeExist", testCtx(), "theme-123").Return(false, errors.New("database error"))

	exists, err := suite.service.IsThemeExist(testCtx(), "theme-123")

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

	suite.mockStore.On("IsThemeHandleConflict", testCtx(), "my-theme", "").Return(false,
		errors.New("database error"))

	result, err := suite.service.CreateTheme(testCtx(), themeRequest)

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

	suite.mockStore.On("IsThemeDeclarative", testCtx(), "theme-123").Return(false)
	suite.mockStore.On("GetTheme", testCtx(), "theme-123").Return(Theme{}, errors.New("database error"))

	result, err := suite.service.UpdateTheme(testCtx(), "theme-123", updateRequest)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
}

// --- GetThemeUsages tests ---

// stubUsageRegistry is a minimal resourcedependency.Registry for tests.
type stubUsageRegistry struct {
	resp *resourcedependency.DependenciesResponse
	err  error
}

func (s *stubUsageRegistry) RegisterProvider(resourcedependency.Provider) {}

func (s *stubUsageRegistry) GetDependencies(
	_ context.Context, _, _ string) (*resourcedependency.DependenciesResponse, error) {
	return s.resp, s.err
}

func (s *stubUsageRegistry) CascadeDelete(_ context.Context, _, _ string) (int, error) {
	return 0, nil
}

func (s *stubUsageRegistry) ValidateReferenceUpdate(
	_ context.Context, _, _ string) *tidcommon.ServiceError {
	return nil
}

// Test GetThemeUsages - Empty ID
func (suite *ThemeServiceTestSuite) TestGetThemeUsages_EmptyID() {
	result, err := suite.service.GetThemeUsages(testCtx(), "", 10, 0)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorInvalidThemeID.Code, err.Code)
}

// Test GetThemeUsages - Theme not found
func (suite *ThemeServiceTestSuite) TestGetThemeUsages_NotFound() {
	suite.mockStore.On("IsThemeExist", testCtx(), "missing").Return(false, nil)

	result, err := suite.service.GetThemeUsages(testCtx(), "missing", 10, 0)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorThemeNotFound.Code, err.Code)
}

// Test GetThemeUsages - Store error on existence check
func (suite *ThemeServiceTestSuite) TestGetThemeUsages_ExistenceCheckError() {
	suite.mockStore.On("IsThemeExist", testCtx(), "theme-123").Return(false, errors.New("database error"))

	result, err := suite.service.GetThemeUsages(testCtx(), "theme-123", 10, 0)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
}

// Test GetThemeUsages - usage registry not set returns unknown (nil totalResults)
func (suite *ThemeServiceTestSuite) TestGetThemeUsages_RegistryNotSet() {
	suite.mockStore.On("IsThemeExist", testCtx(), "theme-123").Return(true, nil)

	result, err := suite.service.GetThemeUsages(testCtx(), "theme-123", 10, 0)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Nil(suite.T(), result.TotalResults)
	assert.Nil(suite.T(), result.Summary)
	assert.Empty(suite.T(), result.Usages)
}

// Test GetThemeUsages - registry returns usages
func (suite *ThemeServiceTestSuite) TestGetThemeUsages_WithUsages() {
	suite.mockStore.On("IsThemeExist", testCtx(), "theme-123").Return(true, nil)
	total := 2
	suite.service.SetDependencyRegistry(&stubUsageRegistry{
		resp: &resourcedependency.DependenciesResponse{
			TotalResults: &total,
			Count:        2,
			Summary:      map[string]int{resourcedependency.ResourceTypeApplication: 2},
			Usages: []resourcedependency.ResourceDependency{
				{ResourceType: resourcedependency.ResourceTypeApplication, ID: "app-1",
					DisplayName: "App One", BehaviorOnDelete: resourcedependency.BehaviorFallback},
				{ResourceType: resourcedependency.ResourceTypeApplication, ID: "app-2",
					DisplayName: "App Two", BehaviorOnDelete: resourcedependency.BehaviorFallback},
			},
		},
	})

	result, err := suite.service.GetThemeUsages(testCtx(), "theme-123", 10, 0)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.NotNil(suite.T(), result.TotalResults)
	assert.Equal(suite.T(), 2, *result.TotalResults)
	assert.Equal(suite.T(), 2, result.Summary[resourcedependency.ResourceTypeApplication])
	assert.Len(suite.T(), result.Usages, 2)
	assert.Equal(suite.T(), resourcedependency.ResourceTypeApplication, result.Usages[0].ResourceType)
	assert.Equal(suite.T(), resourcedependency.BehaviorFallback, result.Usages[0].BehaviorOnDelete)
	assert.Equal(suite.T(), "App One", result.Usages[0].DisplayName)
}

// Test GetThemeUsages - registry returns no usages
func (suite *ThemeServiceTestSuite) TestGetThemeUsages_NoUsages() {
	suite.mockStore.On("IsThemeExist", testCtx(), "theme-123").Return(true, nil)
	total := 0
	suite.service.SetDependencyRegistry(&stubUsageRegistry{
		resp: &resourcedependency.DependenciesResponse{
			TotalResults: &total, Usages: []resourcedependency.ResourceDependency{},
		},
	})

	result, err := suite.service.GetThemeUsages(testCtx(), "theme-123", 10, 0)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.NotNil(suite.T(), result.TotalResults)
	assert.Equal(suite.T(), 0, *result.TotalResults)
	assert.Empty(suite.T(), result.Usages)
}

// Test GetThemeUsages - registry returns error
func (suite *ThemeServiceTestSuite) TestGetThemeUsages_RegistryError() {
	suite.mockStore.On("IsThemeExist", testCtx(), "theme-123").Return(true, nil)
	suite.service.SetDependencyRegistry(&stubUsageRegistry{err: errors.New("registry error")})

	result, err := suite.service.GetThemeUsages(testCtx(), "theme-123", 10, 0)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
}

// ctxKeyForTest marks a context so a store expectation can assert the service carried the caller's
// context through, rather than substituting one of its own.
type ctxKeyForTest struct{}

// testCtx returns a context distinguishable from any other, including context.Background().
func testCtx() context.Context {
	return context.WithValue(context.Background(), ctxKeyForTest{}, "carried")
}

const testProjectID = "project-1"

// projectThemeRequest returns a create request for a theme in testProjectID.
func projectThemeRequest() CreateThemeRequestWithID {
	return CreateThemeRequestWithID{
		Handle:      "project-theme",
		DisplayName: "Project Theme",
		ProjectID:   testProjectID,
		Theme:       json.RawMessage(`{"colors": {"primary": "#ff0000"}}`),
	}
}

// A theme created in a project that exists is stored with its project.
func (suite *ThemeServiceTestSuite) TestCreateTheme_InProject() {
	req := projectThemeRequest()
	suite.mockStore.On("IsThemeHandleConflict", testCtx(), req.Handle, "").Return(false, nil)
	suite.mockProjects.On("ProjectExists", testCtx(), testProjectID).Return(true, nil)
	suite.mockStore.On("CreateTheme", testCtx(), mock.AnythingOfType("string"),
		mock.MatchedBy(func(r CreateThemeRequest) bool { return r.ProjectID == testProjectID })).Return(nil)

	result, err := suite.service.CreateTheme(testCtx(), req)

	suite.Nil(err)
	suite.Equal(testProjectID, result.ProjectID)
}

// A theme created in a project that does not exist is refused, and nothing is stored.
func (suite *ThemeServiceTestSuite) TestCreateTheme_UnknownProject() {
	req := projectThemeRequest()
	suite.mockStore.On("IsThemeHandleConflict", testCtx(), req.Handle, "").Return(false, nil)
	suite.mockProjects.On("ProjectExists", testCtx(), testProjectID).Return(false, nil)

	result, err := suite.service.CreateTheme(testCtx(), req)

	suite.Nil(result)
	suite.Equal(&ErrorInvalidProject, err)
	suite.mockStore.AssertNotCalled(suite.T(), "CreateTheme", mock.Anything, mock.Anything, mock.Anything)
}

// A failure to check the project is a server error.
func (suite *ThemeServiceTestSuite) TestCreateTheme_ProjectCheckFailure() {
	req := projectThemeRequest()
	suite.mockStore.On("IsThemeHandleConflict", testCtx(), req.Handle, "").Return(false, nil)
	suite.mockProjects.On("ProjectExists", testCtx(), testProjectID).
		Return(false, &tidcommon.InternalServerError)

	result, err := suite.service.CreateTheme(testCtx(), req)

	suite.Nil(result)
	suite.Equal(&tidcommon.InternalServerError, err)
}

// A deployment that serves no projects keeps the project ID without checking it.
func (suite *ThemeServiceTestSuite) TestCreateTheme_ProjectWithoutChecker() {
	service := newThemeMgtService(suite.mockStore, discardingBus{}, nil, allowAllAuthz(suite.T()))
	req := projectThemeRequest()
	suite.mockStore.On("IsThemeHandleConflict", testCtx(), req.Handle, "").Return(false, nil)
	suite.mockStore.On("CreateTheme", testCtx(), mock.AnythingOfType("string"),
		mock.MatchedBy(func(r CreateThemeRequest) bool { return r.ProjectID == testProjectID })).Return(nil)

	result, err := service.CreateTheme(testCtx(), req)

	suite.Nil(err)
	suite.Equal(testProjectID, result.ProjectID)
}

// A deployment that serves projects refuses a theme in none, and nothing is stored.
func (suite *ThemeServiceTestSuite) TestCreateTheme_ProjectRequired() {
	req := projectThemeRequest()
	req.ProjectID = ""
	suite.mockStore.On("IsThemeHandleConflict", testCtx(), req.Handle, "").Return(false, nil)

	result, err := suite.service.CreateTheme(testCtx(), req)

	suite.Nil(result)
	suite.Equal(&ErrorProjectRequired, err)
	suite.mockStore.AssertNotCalled(suite.T(), "CreateTheme", mock.Anything, mock.Anything, mock.Anything)
}

// A project ID longer than the column is refused before anything is checked, with or without projects.
func (suite *ThemeServiceTestSuite) TestCreateTheme_ProjectIDTooLong() {
	req := projectThemeRequest()
	req.ProjectID = strings.Repeat("p", maxProjectIDLength+1)
	suite.mockStore.On("IsThemeHandleConflict", testCtx(), req.Handle, "").Return(false, nil)

	unchecked := newThemeMgtService(suite.mockStore, discardingBus{}, nil, allowAllAuthz(suite.T()))
	for _, service := range []ThemeMgtServiceInterface{suite.service, unchecked} {
		result, err := service.CreateTheme(testCtx(), req)

		suite.Nil(result)
		suite.Equal(&ErrorInvalidProject, err)
	}
	suite.mockProjects.AssertNotCalled(suite.T(), "ProjectExists", mock.Anything, mock.Anything)
	suite.mockStore.AssertNotCalled(suite.T(), "CreateTheme", mock.Anything, mock.Anything, mock.Anything)
}

// A project has themes when its theme count is above zero.
func (suite *ThemeServiceTestSuite) TestProjectHasThemes() {
	suite.mockStore.On("GetThemeListCountByProject", testCtx(), testProjectID).Return(2, nil).Once()
	hasThemes, err := suite.service.ProjectHasThemes(testCtx(), testProjectID)
	suite.Nil(err)
	suite.True(hasThemes)

	suite.mockStore.On("GetThemeListCountByProject", testCtx(), testProjectID).Return(0, nil).Once()
	hasThemes, err = suite.service.ProjectHasThemes(testCtx(), testProjectID)
	suite.Nil(err)
	suite.False(hasThemes)
}

// A failure to count the themes is a server error.
func (suite *ThemeServiceTestSuite) TestProjectHasThemes_StoreError() {
	suite.mockStore.On("GetThemeListCountByProject", testCtx(), testProjectID).Return(0, errors.New("db"))

	hasThemes, err := suite.service.ProjectHasThemes(testCtx(), testProjectID)

	suite.False(hasThemes)
	suite.Equal(&tidcommon.InternalServerError, err)
}

// expectThemeUpdate sets up an update of a theme in the given project.
func (suite *ThemeServiceTestSuite) expectThemeUpdate(existingProjectID string) {
	suite.mockStore.On("IsThemeDeclarative", testCtx(), "theme-123").Return(false)
	suite.mockStore.On("GetTheme", testCtx(), "theme-123").
		Return(Theme{ID: "theme-123", Handle: "my-theme", ProjectID: existingProjectID}, nil)
}

// projectThemeUpdate returns an update request that names the given project.
func projectThemeUpdate(projectID string) UpdateThemeRequest {
	return UpdateThemeRequest{
		DisplayName: "Updated Theme",
		ProjectID:   projectID,
		Theme:       json.RawMessage(`{"colors": {"primary": "#00ff00"}}`),
	}
}

// An update that names no project, or the same one, keeps the theme in its project without checking it.
func (suite *ThemeServiceTestSuite) TestUpdateTheme_KeepsProject() {
	suite.expectThemeUpdate(testProjectID)
	suite.mockStore.On("UpdateTheme", testCtx(), "theme-123", projectThemeUpdate(testProjectID)).Return(nil)

	for _, projectID := range []string{"", testProjectID} {
		result, err := suite.service.UpdateTheme(testCtx(), "theme-123", projectThemeUpdate(projectID))
		suite.Nil(err)
		suite.Equal(testProjectID, result.ProjectID)
	}
	suite.mockProjects.AssertNotCalled(suite.T(), "ProjectExists", mock.Anything, mock.Anything)
}

// An update that names another project moves the theme there once the project is found.
func (suite *ThemeServiceTestSuite) TestUpdateTheme_MovesProject() {
	suite.expectThemeUpdate("")
	suite.mockProjects.On("ProjectExists", testCtx(), testProjectID).Return(true, nil)
	suite.mockStore.On("UpdateTheme", testCtx(), "theme-123", projectThemeUpdate(testProjectID)).Return(nil)

	result, err := suite.service.UpdateTheme(testCtx(), "theme-123", projectThemeUpdate(testProjectID))

	suite.Nil(err)
	suite.Equal(testProjectID, result.ProjectID)
}

// An update that names a project that does not exist, or one that cannot be checked, is refused.
func (suite *ThemeServiceTestSuite) TestUpdateTheme_ProjectErrors() {
	suite.expectThemeUpdate("")
	suite.mockProjects.On("ProjectExists", testCtx(), "missing").Return(false, nil)
	suite.mockProjects.On("ProjectExists", testCtx(), "broken").Return(false, &tidcommon.InternalServerError)

	_, err := suite.service.UpdateTheme(testCtx(), "theme-123", projectThemeUpdate("missing"))
	suite.Equal(&ErrorInvalidProject, err)
	_, err = suite.service.UpdateTheme(testCtx(), "theme-123", projectThemeUpdate("broken"))
	suite.Equal(&tidcommon.InternalServerError, err)
	suite.mockStore.AssertNotCalled(suite.T(), "UpdateTheme", mock.Anything, mock.Anything, mock.Anything)
}

func (suite *ThemeServiceTestSuite) TestGetThemeListByProject_Success() {
	themes := []Theme{{ID: "theme-1", ProjectID: testProjectID}}
	suite.mockStore.On("GetThemeListCountByProject", testCtx(), testProjectID).Return(3, nil)
	suite.mockStore.On("GetThemeListByProject", testCtx(), testProjectID, 1, 1).Return(themes, nil)

	result, err := suite.service.GetThemeListByProject(testCtx(), testProjectID, 1, 1)

	suite.Nil(err)
	suite.Equal(3, result.TotalResults)
	suite.Equal(1, result.Count)
	suite.Equal(2, result.StartIndex)
	suite.Equal(themes, result.Themes)
	suite.Equal([]Link{
		{Href: "/design/themes?offset=0&limit=1&projectId=project-1", Rel: "first"},
		{Href: "/design/themes?offset=0&limit=1&projectId=project-1", Rel: "prev"},
		{Href: "/design/themes?offset=2&limit=1&projectId=project-1", Rel: "next"},
		{Href: "/design/themes?offset=2&limit=1&projectId=project-1", Rel: "last"},
	}, result.Links)
}

// A list without a project ID is empty, and does not reach the store.
func (suite *ThemeServiceTestSuite) TestGetThemeListByProject_EmptyProject() {
	result, err := suite.service.GetThemeListByProject(testCtx(), "", 10, 0)

	suite.Nil(err)
	suite.Zero(result.TotalResults)
	suite.Empty(result.Themes)
}

func (suite *ThemeServiceTestSuite) TestGetThemeListByProject_Errors() {
	_, err := suite.service.GetThemeListByProject(testCtx(), testProjectID, 0, 0)
	suite.Equal(ErrorInvalidLimitValue.Code, err.Code)

	suite.mockStore.On("GetThemeListCountByProject", testCtx(), "count-fails").Return(0, errors.New("db"))
	_, err = suite.service.GetThemeListByProject(testCtx(), "count-fails", 10, 0)
	suite.Equal(&tidcommon.InternalServerError, err)

	suite.mockStore.On("GetThemeListCountByProject", testCtx(), "list-fails").Return(1, nil)
	suite.mockStore.On("GetThemeListByProject", testCtx(), "list-fails", 10, 0).Return(nil, errors.New("db"))
	_, err = suite.service.GetThemeListByProject(testCtx(), "list-fails", 10, 0)
	suite.Equal(&tidcommon.InternalServerError, err)
}

func (suite *ThemeServiceTestSuite) TestWritesRefusedWithoutWriteAccess() {
	tests := []struct {
		name     string
		authzErr *tidcommon.ServiceError
		want     *tidcommon.ServiceError
	}{
		{"Denied", nil, &tidcommon.ErrorUnauthorized},
		{"AuthorizationFailure", &tidcommon.InternalServerError, &tidcommon.InternalServerError},
	}
	for _, tt := range tests {
		suite.Run(tt.name, func() {
			authz := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(suite.T())
			for _, action := range []security.Action{
				security.ActionCreateDesign, security.ActionUpdateDesign, security.ActionDeleteDesign,
			} {
				authz.On("IsActionAllowed", mock.Anything, action,
					&sysauthz.ActionContext{ResourceType: security.ResourceTypeDesign}).
					Return(false, tt.authzErr).Once()
			}
			service := newThemeMgtService(newThemeMgtStoreInterfaceMock(suite.T()), discardingBus{}, nil, authz)

			_, err := service.CreateTheme(testCtx(), CreateThemeRequestWithID{})
			suite.Equal(tt.want, err)
			_, err = service.UpdateTheme(testCtx(), "theme-1", UpdateThemeRequest{})
			suite.Equal(tt.want, err)
			suite.Equal(tt.want, service.DeleteTheme(testCtx(), "theme-1"))
		})
	}
}

// allowAllAuthz returns an authorization service that allows every action.
func allowAllAuthz(t *testing.T) *sysauthzmock.SystemAuthorizationServiceInterfaceMock {
	authz := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
	authz.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).Return(true, nil).Maybe()
	return authz
}
