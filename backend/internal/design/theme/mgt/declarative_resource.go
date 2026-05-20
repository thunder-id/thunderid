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
	"fmt"
	"strings"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"

	"gopkg.in/yaml.v3"
)

const (
	resourceTypeTheme = "theme"
	paramTypeTheme    = "Theme"
)

// themeExporter implements declarativeresource.ResourceExporter for themes.
type themeExporter struct {
	service ThemeMgtServiceInterface
}

// newThemeExporter creates a new theme exporter.
func newThemeExporter(service ThemeMgtServiceInterface) *themeExporter {
	return &themeExporter{service: service}
}

// GetResourceType returns the resource type for themes.
func (e *themeExporter) GetResourceType() string {
	return resourceTypeTheme
}

// GetParameterizerType returns the parameterizer type for themes.
func (e *themeExporter) GetParameterizerType() string {
	return paramTypeTheme
}

// GetAllResourceIDs retrieves all theme IDs from the database store.
// In composite mode, this excludes declarative (YAML-based) themes.
func (e *themeExporter) GetAllResourceIDs(ctx context.Context) ([]string, *serviceerror.ServiceError) {
	const pageSize = 100
	var allIDs []string
	offset := 0

	for {
		themeList, err := e.service.GetThemeList(pageSize, offset)
		if err != nil {
			return nil, err
		}

		// Accumulate IDs from this page
		for _, theme := range themeList.Themes {
			allIDs = append(allIDs, theme.ID)
		}

		// Stop if we got fewer items than requested (last page)
		if themeList.Count < pageSize {
			break
		}

		// Move to next page
		offset += pageSize
	}

	return allIDs, nil
}

// GetResourceByID retrieves a theme by its ID.
func (e *themeExporter) GetResourceByID(ctx context.Context, id string) (
	interface{}, string, *serviceerror.ServiceError,
) {
	theme, err := e.service.GetTheme(id)
	if err != nil {
		return nil, "", err
	}
	return theme, theme.DisplayName, nil
}

// ValidateResource validates a theme resource.
func (e *themeExporter) ValidateResource(
	resource interface{}, id string, logger *log.Logger,
) (string, *declarativeresource.ExportError) {
	theme, ok := resource.(*Theme)
	if !ok {
		return "", declarativeresource.CreateTypeError(resourceTypeTheme, id)
	}

	err := declarativeresource.ValidateResourceName(
		theme.DisplayName, resourceTypeTheme, id, "THEME_VALIDATION_ERROR", logger,
	)
	if err != nil {
		return "", err
	}

	if len(theme.Theme) == 0 {
		logger.Warn("Theme has no theme configuration",
			log.String("themeID", id), log.String("displayName", theme.DisplayName))
	}

	return theme.DisplayName, nil
}

// GetResourceRules returns the parameterization rules for themes.
func (e *themeExporter) GetResourceRules() *declarativeresource.ResourceRules {
	return &declarativeresource.ResourceRules{}
}

// loadDeclarativeResources loads declarative theme resources from files.
// The dbStore parameter is optional (can be nil) and is used for duplicate checking in composite mode.
func loadDeclarativeResources(fileStore themeMgtStoreInterface, dbStore themeMgtStoreInterface) error {
	// Type assert to access Storer interface for resource loading
	fileBasedStore, ok := fileStore.(*themeFileBasedStore)
	if !ok {
		return fmt.Errorf("failed to assert fileStore to *themeFileBasedStore")
	}

	resourceConfig := declarativeresource.ResourceConfig{
		ResourceType:  "Theme",
		DirectoryName: "themes",
		Parser:        parseToThemeWrapper,
		Validator: func(data interface{}) error {
			return validateThemeWrapper(data, dbStore)
		},
		IDExtractor: func(data interface{}) string {
			if theme, ok := data.(*Theme); ok {
				return theme.ID
			}
			return ""
		},
	}

	loader := declarativeresource.NewResourceLoader(resourceConfig, fileBasedStore)
	if err := loader.LoadResources(); err != nil {
		return fmt.Errorf("failed to load theme resources: %w", err)
	}

	return nil
}

// parseToThemeWrapper wraps parseToTheme to match ResourceConfig.Parser signature.
func parseToThemeWrapper(data []byte) (interface{}, error) {
	return parseToTheme(data)
}

// parseToTheme converts YAML data into a Theme object.
func parseToTheme(data []byte) (*Theme, error) {
	var themeRequest themeRequestWithID

	err := yaml.Unmarshal(data, &themeRequest)
	if err != nil {
		return nil, err
	}

	// Convert theme to JSON bytes
	var themeJSON json.RawMessage
	if themeRequest.Theme != nil {
		// Handle both map structure and string format
		switch v := themeRequest.Theme.(type) {
		case string:
			// JSON string format
			themeJSON = []byte(v)
		default:
			// Map structure - marshal to JSON
			themeBytes, err := json.Marshal(themeRequest.Theme)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal theme to JSON: %w", err)
			}
			themeJSON = themeBytes
		}
	}

	theme := &Theme{
		ID:          themeRequest.ID,
		DisplayName: themeRequest.DisplayName,
		Description: themeRequest.Description,
		Theme:       themeJSON,
		CreatedAt:   "",
		UpdatedAt:   "",
	}

	return theme, nil
}

// validateThemeWrapper wraps validateThemeForDeclarativeResource to match ResourceConfig.Validator signature.
// It also checks for duplicates across database stores in composite mode.
func validateThemeWrapper(dto interface{}, dbStore themeMgtStoreInterface) error {
	theme, ok := dto.(*Theme)
	if !ok {
		return fmt.Errorf("invalid type: expected *Theme")
	}

	// Basic validation
	if err := validateThemeForDeclarativeResource(theme); err != nil {
		return err
	}

	// In composite mode, check for duplicates in database store
	if dbStore != nil {
		exists, err := dbStore.IsThemeExist(theme.ID)
		if err != nil {
			return fmt.Errorf("failed to check for duplicate theme ID '%s': %w", theme.ID, err)
		}
		if exists {
			return fmt.Errorf("theme with ID '%s' already exists in database", theme.ID)
		}
	}

	return nil
}

// validateThemeForDeclarativeResource validates a theme for declarative resource loading.
func validateThemeForDeclarativeResource(theme *Theme) error {
	if strings.TrimSpace(theme.DisplayName) == "" {
		return fmt.Errorf("theme display name is required")
	}

	if strings.TrimSpace(theme.ID) == "" {
		return fmt.Errorf("theme ID is required")
	}

	if len(theme.Theme) == 0 {
		return fmt.Errorf("theme configuration is required for '%s'", theme.DisplayName)
	}

	// Validate that theme is valid JSON
	var themeConfig interface{}
	if err := json.Unmarshal(theme.Theme, &themeConfig); err != nil {
		return fmt.Errorf("invalid theme JSON for '%s': %w", theme.DisplayName, err)
	}

	return nil
}
