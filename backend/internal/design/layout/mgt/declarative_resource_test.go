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

type DeclarativeResourceTestSuite struct {
	suite.Suite
}

func TestDeclarativeResourceTestSuite(t *testing.T) {
	suite.Run(t, new(DeclarativeResourceTestSuite))
}

func (s *DeclarativeResourceTestSuite) SetupSuite() {
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

func (s *DeclarativeResourceTestSuite) TearDownSuite() {
	// Clean up server runtime after all tests
	config.ResetServerRuntime()
}

func (s *DeclarativeResourceTestSuite) TestLayoutExporter_GetResourceType() {
	exporter := &layoutExporter{}
	s.Equal("layout", exporter.GetResourceType())
}

func (s *DeclarativeResourceTestSuite) TestLayoutExporter_GetParameterizerType() {
	exporter := &layoutExporter{}
	s.Equal("Layout", exporter.GetParameterizerType())
}

func (s *DeclarativeResourceTestSuite) TestLayoutExporter_GetResourceRules() {
	exporter := &layoutExporter{}
	rules := exporter.GetResourceRules()

	s.NotNil(rules)
	// Layout field is exported as JSON string, not expanded to YAML structure
	s.Empty(rules.DynamicPropertyFields)
	s.Nil(rules.Variables)
	s.Nil(rules.ArrayVariables)
}

func (s *DeclarativeResourceTestSuite) TestLayoutExporter_ValidateResource_InvalidType() {
	exporter := &layoutExporter{}

	name, err := exporter.ValidateResource("not a layout", "layout1", nil)

	s.NotNil(err)
	s.Empty(name)
	s.Equal("layout", err.ResourceType)
	s.Equal("layout1", err.ResourceID)
}

func (s *DeclarativeResourceTestSuite) TestLayoutExporter_GetAllResourceIDs_Success() {
	// Arrange
	mockService := NewLayoutMgtServiceInterfaceMock(s.T())
	layoutList := &LayoutList{
		Layouts: []Layout{
			{ID: "layout-001", DisplayName: "Layout 1"},
			{ID: "layout-002", DisplayName: "Layout 2"},
			{ID: "layout-003", DisplayName: "Layout 3"},
		},
	}
	mockService.EXPECT().GetLayoutList(100, 0).Return(layoutList, nil).Once()
	exporter := &layoutExporter{service: mockService}

	// Act
	ids, svcErr := exporter.GetAllResourceIDs(context.Background())

	// Assert
	s.Nil(svcErr)
	s.Len(ids, 3)
	s.Equal("layout-001", ids[0])
	s.Equal("layout-002", ids[1])
	s.Equal("layout-003", ids[2])
}

func (s *DeclarativeResourceTestSuite) TestLayoutExporter_GetAllResourceIDs_ServiceError() {
	// Arrange
	serviceErr := &serviceerror.ServiceError{Error: i18ncore.I18nMessage{DefaultValue: "Database error"}}
	mockService := NewLayoutMgtServiceInterfaceMock(s.T())
	mockService.EXPECT().GetLayoutList(100, 0).Return(&LayoutList{}, serviceErr).Once()
	exporter := &layoutExporter{service: mockService}

	// Act
	ids, svcErr := exporter.GetAllResourceIDs(context.Background())

	// Assert
	s.NotNil(svcErr)
	s.Nil(ids)
	s.Equal(serviceErr, svcErr)
}

func (s *DeclarativeResourceTestSuite) TestLayoutExporter_GetAllResourceIDs_EmptyList() {
	// Arrange
	mockService := NewLayoutMgtServiceInterfaceMock(s.T())
	mockService.EXPECT().GetLayoutList(100, 0).Return(&LayoutList{Layouts: []Layout{}}, nil).Once()
	exporter := &layoutExporter{service: mockService}

	// Act
	ids, svcErr := exporter.GetAllResourceIDs(context.Background())

	// Assert
	s.Nil(svcErr)
	s.Empty(ids)
}

func (s *DeclarativeResourceTestSuite) TestLayoutExporter_GetResourceByID_Success() {
	// Arrange
	layoutConfig := map[string]interface{}{"type": "centered"}
	layoutJSON, _ := json.Marshal(layoutConfig)
	mockService := NewLayoutMgtServiceInterfaceMock(s.T())
	layout := &Layout{
		ID:          "layout-001",
		DisplayName: "Centered Layout",
		Description: "A centered layout",
		Layout:      layoutJSON,
	}
	mockService.EXPECT().GetLayout("layout-001").Return(layout, nil).Once()
	exporter := &layoutExporter{service: mockService}

	// Act
	resource, displayName, svcErr := exporter.GetResourceByID(context.Background(), "layout-001")

	// Assert
	s.Nil(svcErr)
	s.NotNil(resource)
	s.Equal("Centered Layout", displayName)

	retrievedLayout, ok := resource.(*Layout)
	s.True(ok)
	s.Equal("layout-001", retrievedLayout.ID)
	s.Equal("Centered Layout", retrievedLayout.DisplayName)
}

func (s *DeclarativeResourceTestSuite) TestLayoutExporter_GetResourceByID_NotFound() {
	// Arrange
	serviceErr := &serviceerror.ServiceError{Error: i18ncore.I18nMessage{DefaultValue: "Layout not found"}}
	mockService := NewLayoutMgtServiceInterfaceMock(s.T())
	mockService.EXPECT().GetLayout("non-existent").Return(&Layout{}, serviceErr).Once()
	exporter := &layoutExporter{service: mockService}

	// Act
	resource, displayName, svcErr := exporter.GetResourceByID(context.Background(), "non-existent")

	// Assert
	s.NotNil(svcErr)
	s.Nil(resource)
	s.Empty(displayName)
	s.Equal(serviceErr, svcErr)
}

func (s *DeclarativeResourceTestSuite) TestParseToLayout() {
	yamlData := []byte(`
id: layout-001
displayName: Centered Layout
description: A centered layout configuration
layout:
  type: centered
  components:
    - type: logo
      position: top
    - type: form
      position: center
`)

	layout, err := parseToLayout(yamlData)

	s.NoError(err)
	s.NotNil(layout)
	s.Equal("layout-001", layout.ID)
	s.Equal("Centered Layout", layout.DisplayName)
	s.Equal("A centered layout configuration", layout.Description)
	s.NotEmpty(layout.Layout)

	// Verify layout JSON is parseable
	var layoutConfig map[string]interface{}
	err = json.Unmarshal(layout.Layout, &layoutConfig)
	s.NoError(err)
	s.Equal("centered", layoutConfig["type"])
}

func (s *DeclarativeResourceTestSuite) TestParseToLayout_InvalidYAML() {
	yamlData := []byte(`invalid: yaml: data:`)

	layout, err := parseToLayout(yamlData)

	s.Error(err)
	s.Nil(layout)
}

func (s *DeclarativeResourceTestSuite) TestValidateLayoutForDeclarativeResource_Success() {
	layoutConfig := map[string]interface{}{"type": "centered"}
	layoutJSON, _ := json.Marshal(layoutConfig)

	layout := &Layout{
		ID:          "layout1",
		DisplayName: "Test Layout",
		Description: "A test layout",
		Layout:      layoutJSON,
	}

	err := validateLayoutForDeclarativeResource(layout)

	s.NoError(err)
}

func (s *DeclarativeResourceTestSuite) TestValidateLayoutForDeclarativeResource_EmptyDisplayName() {
	layout := &Layout{
		ID:          "layout1",
		DisplayName: "   ",
		Layout:      json.RawMessage(`{"type": "centered"}`),
	}

	err := validateLayoutForDeclarativeResource(layout)

	s.Error(err)
	s.Contains(err.Error(), "display name is required")
}

func (s *DeclarativeResourceTestSuite) TestValidateLayoutForDeclarativeResource_EmptyID() {
	layout := &Layout{
		ID:          "",
		DisplayName: "Layout",
		Layout:      json.RawMessage(`{"type": "centered"}`),
	}

	err := validateLayoutForDeclarativeResource(layout)

	s.Error(err)
	s.Contains(err.Error(), "ID is required")
}

func (s *DeclarativeResourceTestSuite) TestValidateLayoutForDeclarativeResource_EmptyLayout() {
	layout := &Layout{
		ID:          "layout1",
		DisplayName: "Test",
		Layout:      json.RawMessage{},
	}

	err := validateLayoutForDeclarativeResource(layout)

	s.Error(err)
	s.Contains(err.Error(), "configuration is required")
}

func (s *DeclarativeResourceTestSuite) TestValidateLayoutForDeclarativeResource_InvalidJSON() {
	layout := &Layout{
		ID:          "layout1",
		DisplayName: "Test",
		Layout:      json.RawMessage(`{invalid json}`),
	}

	err := validateLayoutForDeclarativeResource(layout)

	s.Error(err)
	s.Contains(err.Error(), "invalid layout JSON")
}

func (s *DeclarativeResourceTestSuite) TestLoadDeclarativeResources_Integration() {
	// Create file-based store with test configuration
	genericStore := declarativeresource.NewGenericFileBasedStoreForTest(entity.KeyTypeLayout)
	store := &layoutFileBasedStore{
		GenericFileBasedStore: genericStore,
	}

	// This should not panic even with empty directory (pass nil for dbStore in test)
	err := loadDeclarativeResources(store, nil)

	// Error is expected if directory doesn't exist, which is acceptable for this test
	// We're verifying that the function can be called without panicking
	_ = err
}

func (s *DeclarativeResourceTestSuite) TestLoadDeclarativeResources_WithDBStore() {
	serverHome := config.GetServerRuntime().ServerHome
	resourceDir := filepath.Join(serverHome, "repository", "resources", "layouts")
	err := os.MkdirAll(resourceDir, 0o750)
	s.Require().NoError(err)

	yamlData := []byte("id: layout-dup\ndisplayName: Layout Dup\nlayout:\n  type: centered\n")
	err = os.WriteFile(filepath.Join(resourceDir, "layout-dup.yaml"), yamlData, 0o600)
	s.Require().NoError(err)

	genericStore := declarativeresource.NewGenericFileBasedStoreForTest(entity.KeyTypeLayout)
	store := &layoutFileBasedStore{GenericFileBasedStore: genericStore}

	dbStore := newLayoutMgtStoreInterfaceMock(s.T())
	dbStore.On("IsLayoutExist", "layout-dup").Return(false, nil)

	err = loadDeclarativeResources(store, dbStore)
	s.NoError(err)
}

func (s *DeclarativeResourceTestSuite) TestLayoutExporter_ValidateResource_EmptyLayoutWarning() {
	// Use the singleton logger which is already initialized in SetupSuite
	testLogger := log.GetLogger()

	exporter := &layoutExporter{}

	layout := &Layout{
		ID:          "layout1",
		DisplayName: "Layout Without Config",
		Description: "This layout has no configuration",
		Layout:      json.RawMessage{},
	}

	// Even with empty layout, validation should succeed (just logs warning)
	name, err := exporter.ValidateResource(layout, "layout1", testLogger)

	// The empty layout should not cause validation error in ValidateResource
	// (it logs a warning instead)
	s.Nil(err)
	s.Equal("Layout Without Config", name)
}

func (s *DeclarativeResourceTestSuite) TestParseToLayoutWrapper() {
	yamlData := []byte(`
id: layout-wrapper-test
displayName: Wrapper Test Layout
description: Testing wrapper
layout:
  type: fullscreen
`)

	result, err := parseToLayoutWrapper(yamlData)

	s.NoError(err)
	s.NotNil(result)

	layout, ok := result.(*Layout)
	s.True(ok)
	s.Equal("layout-wrapper-test", layout.ID)
}

func (s *DeclarativeResourceTestSuite) TestValidateLayoutWrapper() {
	layout := &Layout{
		ID:          "layout1",
		DisplayName: "Test",
		Layout:      json.RawMessage(`{"type": "centered"}`),
	}

	err := validateLayoutWrapper(layout, nil)
	s.NoError(err)
}

func (s *DeclarativeResourceTestSuite) TestValidateLayoutWrapper_DBStoreDuplicate() {
	layout := &Layout{
		ID:          "layout1",
		DisplayName: "Test",
		Layout:      json.RawMessage(`{"type": "centered"}`),
	}

	dbStore := newLayoutMgtStoreInterfaceMock(s.T())
	dbStore.On("IsLayoutExist", "layout1").Return(true, nil)

	err := validateLayoutWrapper(layout, dbStore)
	s.Error(err)
	s.Contains(err.Error(), "already exists in database")
}

func (s *DeclarativeResourceTestSuite) TestValidateLayoutWrapper_DBStoreError() {
	layout := &Layout{
		ID:          "layout1",
		DisplayName: "Test",
		Layout:      json.RawMessage(`{"type": "centered"}`),
	}

	dbStore := newLayoutMgtStoreInterfaceMock(s.T())
	dbStore.On("IsLayoutExist", "layout1").Return(false, errors.New("db error"))

	err := validateLayoutWrapper(layout, dbStore)
	s.Error(err)
	s.Contains(err.Error(), "failed to check for duplicate layout ID")
}

func (s *DeclarativeResourceTestSuite) TestValidateLayoutWrapper_InvalidType() {
	err := validateLayoutWrapper("not a layout", nil)
	s.Error(err)
	s.Contains(err.Error(), "invalid type")
}
