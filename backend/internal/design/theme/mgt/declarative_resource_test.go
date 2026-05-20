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
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/thunder-id/thunderid/internal/system/config"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	i18ncore "github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/system/log"

	"github.com/stretchr/testify/suite"
)

type ThemeDeclarativeSuite struct {
	suite.Suite
}

func TestThemeDeclarativeSuite(t *testing.T) {
	suite.Run(t, new(ThemeDeclarativeSuite))
}

func (s *ThemeDeclarativeSuite) SetupSuite() {
	// Create temporary directory for tests
	tempDir := s.T().TempDir()

	// Initialize server runtime once for all tests
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime(tempDir, testConfig)
	s.Require().NoError(err, "Failed to initialize server runtime")
}

func (s *ThemeDeclarativeSuite) TearDownSuite() {
	// Clean up server runtime after all tests
	config.ResetServerRuntime()
}

func (s *ThemeDeclarativeSuite) TestThemeExporter_GetResourceType() {
	exporter := &themeExporter{}
	s.Equal("theme", exporter.GetResourceType())
}

func (s *ThemeDeclarativeSuite) TestThemeExporter_GetParameterizerType() {
	exporter := &themeExporter{}
	s.Equal("Theme", exporter.GetParameterizerType())
}

func (s *ThemeDeclarativeSuite) TestThemeExporter_GetResourceRules() {
	exporter := &themeExporter{}
	rules := exporter.GetResourceRules()

	s.NotNil(rules)
	// Theme field is exported as JSON string, not expanded to YAML structure
	s.Empty(rules.DynamicPropertyFields)
	s.Nil(rules.Variables)
	s.Nil(rules.ArrayVariables)
}

func (s *ThemeDeclarativeSuite) TestThemeExporter_ValidateResource_InvalidType() {
	exporter := &themeExporter{}

	name, err := exporter.ValidateResource("not a theme", "theme1", nil)

	s.NotNil(err)
	s.Empty(name)
	s.Equal("theme", err.ResourceType)
	s.Equal("theme1", err.ResourceID)
}

func (s *ThemeDeclarativeSuite) TestThemeExporter_GetAllResourceIDs_Success() {
	// Arrange
	mockService := NewThemeMgtServiceInterfaceMock(s.T())
	themeList := &ThemeList{
		Themes: []Theme{
			{ID: "theme-001", DisplayName: "Theme 1"},
			{ID: "theme-002", DisplayName: "Theme 2"},
			{ID: "theme-003", DisplayName: "Theme 3"},
		},
	}
	mockService.EXPECT().GetThemeList(100, 0).Return(themeList, nil).Once()
	exporter := &themeExporter{service: mockService}

	// Act
	ids, svcErr := exporter.GetAllResourceIDs(context.Background())

	// Assert
	s.Nil(svcErr)
	s.Len(ids, 3)
	s.Equal("theme-001", ids[0])
	s.Equal("theme-002", ids[1])
	s.Equal("theme-003", ids[2])
}

func (s *ThemeDeclarativeSuite) TestThemeExporter_GetAllResourceIDs_ServiceError() {
	// Arrange
	serviceErr := &serviceerror.ServiceError{Error: i18ncore.I18nMessage{DefaultValue: "Database error"}}
	mockService := NewThemeMgtServiceInterfaceMock(s.T())
	mockService.EXPECT().GetThemeList(100, 0).Return(&ThemeList{}, serviceErr).Once()
	exporter := &themeExporter{service: mockService}

	// Act
	ids, svcErr := exporter.GetAllResourceIDs(context.Background())

	// Assert
	s.NotNil(svcErr)
	s.Nil(ids)
	s.Equal(serviceErr, svcErr)
}

func (s *ThemeDeclarativeSuite) TestThemeExporter_GetAllResourceIDs_EmptyList() {
	// Arrange
	mockService := NewThemeMgtServiceInterfaceMock(s.T())
	mockService.EXPECT().GetThemeList(100, 0).Return(&ThemeList{Themes: []Theme{}}, nil).Once()
	exporter := &themeExporter{service: mockService}

	// Act
	ids, svcErr := exporter.GetAllResourceIDs(context.Background())

	// Assert
	s.Nil(svcErr)
	s.Empty(ids)
}

func (s *ThemeDeclarativeSuite) TestThemeExporter_GetResourceByID_Success() {
	// Arrange
	themeConfig := map[string]interface{}{"primaryColor": "#1976d2"}
	themeJSON, _ := json.Marshal(themeConfig)
	mockService := NewThemeMgtServiceInterfaceMock(s.T())
	theme := &Theme{
		ID:          "theme-001",
		DisplayName: "Blue Theme",
		Description: "A blue theme",
		Theme:       themeJSON,
	}
	mockService.EXPECT().GetTheme("theme-001").Return(theme, nil).Once()
	exporter := &themeExporter{service: mockService}

	// Act
	resource, displayName, svcErr := exporter.GetResourceByID(context.Background(), "theme-001")

	// Assert
	s.Nil(svcErr)
	s.NotNil(resource)
	s.Equal("Blue Theme", displayName)

	retrievedTheme, ok := resource.(*Theme)
	s.True(ok)
	s.Equal("theme-001", retrievedTheme.ID)
	s.Equal("Blue Theme", retrievedTheme.DisplayName)
}

func (s *ThemeDeclarativeSuite) TestThemeExporter_GetResourceByID_NotFound() {
	// Arrange
	serviceErr := &serviceerror.ServiceError{Error: i18ncore.I18nMessage{DefaultValue: "Theme not found"}}
	mockService := NewThemeMgtServiceInterfaceMock(s.T())
	mockService.EXPECT().GetTheme("non-existent").Return(&Theme{}, serviceErr).Once()
	exporter := &themeExporter{service: mockService}

	// Act
	resource, displayName, svcErr := exporter.GetResourceByID(context.Background(), "non-existent")

	// Assert
	s.NotNil(svcErr)
	s.Nil(resource)
	s.Empty(displayName)
	s.Equal(serviceErr, svcErr)
}

func (s *ThemeDeclarativeSuite) TestParseToTheme() {
	yamlData := []byte(`
id: theme-001
displayName: Blue Theme
description: A beautiful blue theme
theme:
  primaryColor: "#1976d2"
  secondaryColor: "#dc004e"
`)

	theme, err := parseToTheme(yamlData)

	s.NoError(err)
	s.NotNil(theme)
	s.Equal("theme-001", theme.ID)
	s.Equal("Blue Theme", theme.DisplayName)
	s.Equal("A beautiful blue theme", theme.Description)
	s.NotEmpty(theme.Theme)

	// Verify theme JSON is parseable
	var themeConfig map[string]interface{}
	err = json.Unmarshal(theme.Theme, &themeConfig)
	s.NoError(err)
	s.Equal("#1976d2", themeConfig["primaryColor"])
}

func (s *ThemeDeclarativeSuite) TestParseToTheme_InvalidYAML() {
	yamlData := []byte(`invalid: yaml: data:`)

	theme, err := parseToTheme(yamlData)

	s.Error(err)
	s.Nil(theme)
}

func (s *ThemeDeclarativeSuite) TestValidateThemeForDeclarativeResource_Success() {
	themeConfig := map[string]interface{}{"primaryColor": "#1976d2"}
	themeJSON, _ := json.Marshal(themeConfig)

	theme := &Theme{
		ID:          "theme1",
		DisplayName: "Test Theme",
		Description: "A test theme",
		Theme:       themeJSON,
	}

	err := validateThemeForDeclarativeResource(theme)

	s.NoError(err)
}

func (s *ThemeDeclarativeSuite) TestValidateThemeForDeclarativeResource_EmptyDisplayName() {
	theme := &Theme{
		ID:          "theme1",
		DisplayName: "",
		Theme:       json.RawMessage(`{"color": "blue"}`),
	}

	err := validateThemeForDeclarativeResource(theme)

	s.Error(err)
	s.Contains(err.Error(), "display name is required")
}

func (s *ThemeDeclarativeSuite) TestValidateThemeForDeclarativeResource_EmptyID() {
	theme := &Theme{
		ID:          "",
		DisplayName: "Test",
		Theme:       json.RawMessage(`{"color": "blue"}`),
	}

	err := validateThemeForDeclarativeResource(theme)

	s.Error(err)
	s.Contains(err.Error(), "ID is required")
}

func (s *ThemeDeclarativeSuite) TestValidateThemeForDeclarativeResource_EmptyTheme() {
	theme := &Theme{
		ID:          "theme1",
		DisplayName: "Test",
		Theme:       json.RawMessage{},
	}

	err := validateThemeForDeclarativeResource(theme)

	s.Error(err)
	s.Contains(err.Error(), "configuration is required")
}

func (s *ThemeDeclarativeSuite) TestValidateThemeForDeclarativeResource_InvalidJSON() {
	theme := &Theme{
		ID:          "theme1",
		DisplayName: "Test",
		Theme:       json.RawMessage(`{invalid json}`),
	}

	err := validateThemeForDeclarativeResource(theme)

	s.Error(err)
	s.Contains(err.Error(), "invalid theme JSON")
}

func (s *ThemeDeclarativeSuite) TestLoadDeclarativeResources_Integration() {
	// Create file-based store with test configuration
	genericStore := declarativeresource.NewGenericFileBasedStoreForTest(entity.KeyTypeTheme)
	store := &themeFileBasedStore{
		GenericFileBasedStore: genericStore,
	}

	// This should not panic even with empty directory (pass nil for dbStore in test)
	err := loadDeclarativeResources(store, nil)

	// Error is expected if directory doesn't exist, which is acceptable for this test
	// We're verifying that the function can be called without panicking
	_ = err
}

func (s *ThemeDeclarativeSuite) TestLoadDeclarativeResources_WithDBStore() {
	serverHome := config.GetServerRuntime().ServerHome
	resourceDir := filepath.Join(serverHome, "repository", "resources", "themes")
	err := os.MkdirAll(resourceDir, 0o750)
	s.Require().NoError(err)

	yamlData := []byte("id: theme-dup\ndisplayName: Theme Dup\ntheme:\n  color: blue\n")
	err = os.WriteFile(filepath.Join(resourceDir, "theme-dup.yaml"), yamlData, 0o600)
	s.Require().NoError(err)

	genericStore := declarativeresource.NewGenericFileBasedStoreForTest(entity.KeyTypeTheme)
	store := &themeFileBasedStore{GenericFileBasedStore: genericStore}

	dbStore := newThemeMgtStoreInterfaceMock(s.T())
	dbStore.On("IsThemeExist", "theme-dup").Return(false, nil)

	err = loadDeclarativeResources(store, dbStore)
	s.NoError(err)
}

func (s *ThemeDeclarativeSuite) TestThemeExporter_ValidateResource_EmptyThemeWarning() {
	// Use the singleton logger which is already initialized in SetupSuite
	testLogger := log.GetLogger()

	exporter := &themeExporter{}

	theme := &Theme{
		ID:          "theme1",
		DisplayName: "Theme Without Config",
		Description: "This theme has no configuration",
		Theme:       json.RawMessage{},
	}

	// Even with empty theme, validation should succeed (just logs warning)
	name, err := exporter.ValidateResource(theme, "theme1", testLogger)

	// The empty theme should not cause validation error in ValidateResource
	// (it logs a warning instead)
	s.Nil(err)
	s.Equal("Theme Without Config", name)
}

func (s *ThemeDeclarativeSuite) TestParseToThemeWrapper() {
	yamlData := []byte(`
id: theme-wrapper-test
displayName: Wrapper Test Theme
description: Testing wrapper
theme:
  color: red
`)

	result, err := parseToThemeWrapper(yamlData)

	s.NoError(err)
	s.NotNil(result)

	theme, ok := result.(*Theme)
	s.True(ok)
	s.Equal("theme-wrapper-test", theme.ID)
}

func (s *ThemeDeclarativeSuite) TestValidateThemeWrapper() {
	theme := &Theme{
		ID:          "theme1",
		DisplayName: "Test",
		Theme:       json.RawMessage(`{"color": "blue"}`),
	}

	err := validateThemeWrapper(theme, nil)
	s.NoError(err)
}

func (s *ThemeDeclarativeSuite) TestValidateThemeWrapper_DBStoreDuplicate() {
	theme := &Theme{
		ID:          "theme1",
		DisplayName: "Test",
		Theme:       json.RawMessage(`{"color": "blue"}`),
	}

	dbStore := newThemeMgtStoreInterfaceMock(s.T())
	dbStore.On("IsThemeExist", "theme1").Return(true, nil)

	err := validateThemeWrapper(theme, dbStore)
	s.Error(err)
	s.Contains(err.Error(), "already exists in database")
}

func (s *ThemeDeclarativeSuite) TestValidateThemeWrapper_DBStoreError() {
	theme := &Theme{
		ID:          "theme1",
		DisplayName: "Test",
		Theme:       json.RawMessage(`{"color": "blue"}`),
	}

	dbStore := newThemeMgtStoreInterfaceMock(s.T())
	dbStore.On("IsThemeExist", "theme1").Return(false, errors.New("db error"))

	err := validateThemeWrapper(theme, dbStore)
	s.Error(err)
	s.Contains(err.Error(), "failed to check for duplicate theme ID")
}

func (s *ThemeDeclarativeSuite) TestValidateThemeWrapper_InvalidType() {
	err := validateThemeWrapper("not a theme", nil)
	s.Error(err)
	s.Contains(err.Error(), "invalid type")
}
