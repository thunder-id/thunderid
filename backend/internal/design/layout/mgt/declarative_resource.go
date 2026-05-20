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
	"fmt"
	"strings"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"

	"gopkg.in/yaml.v3"
)

const (
	resourceTypeLayout = "layout"
	paramTypeLayout    = "Layout"
)

// layoutExporter implements declarativeresource.ResourceExporter for layouts.
type layoutExporter struct {
	service LayoutMgtServiceInterface
}

// newLayoutExporter creates a new layout exporter.
func newLayoutExporter(service LayoutMgtServiceInterface) *layoutExporter {
	return &layoutExporter{service: service}
}

// GetResourceType returns the resource type for layouts.
func (e *layoutExporter) GetResourceType() string {
	return resourceTypeLayout
}

// GetParameterizerType returns the parameterizer type for layouts.
func (e *layoutExporter) GetParameterizerType() string {
	return paramTypeLayout
}

// GetAllResourceIDs retrieves all layout IDs from the database store.
// In composite mode, this excludes declarative (YAML-based) layouts.
func (e *layoutExporter) GetAllResourceIDs(ctx context.Context) ([]string, *serviceerror.ServiceError) {
	const pageSize = 100
	var allIDs []string
	offset := 0

	for {
		layoutList, err := e.service.GetLayoutList(pageSize, offset)
		if err != nil {
			return nil, err
		}

		// Accumulate IDs from this page
		for _, layout := range layoutList.Layouts {
			allIDs = append(allIDs, layout.ID)
		}

		// Stop if we got fewer items than requested (last page)
		if layoutList.Count < pageSize {
			break
		}

		// Move to next page
		offset += pageSize
	}

	return allIDs, nil
}

// GetResourceByID retrieves a layout by its ID.
func (e *layoutExporter) GetResourceByID(ctx context.Context, id string) (
	interface{}, string, *serviceerror.ServiceError,
) {
	layout, err := e.service.GetLayout(id)
	if err != nil {
		return nil, "", err
	}
	return layout, layout.DisplayName, nil
}

// ValidateResource validates a layout resource.
func (e *layoutExporter) ValidateResource(
	resource interface{}, id string, logger *log.Logger,
) (string, *declarativeresource.ExportError) {
	layout, ok := resource.(*Layout)
	if !ok {
		return "", declarativeresource.CreateTypeError(resourceTypeLayout, id)
	}

	err := declarativeresource.ValidateResourceName(
		layout.DisplayName, resourceTypeLayout, id, "LAYOUT_VALIDATION_ERROR", logger,
	)
	if err != nil {
		return "", err
	}

	if len(layout.Layout) == 0 {
		logger.Warn("Layout has no layout configuration",
			log.String("layoutID", id), log.String("displayName", layout.DisplayName))
	}

	return layout.DisplayName, nil
}

// GetResourceRules returns the parameterization rules for layouts.
func (e *layoutExporter) GetResourceRules() *declarativeresource.ResourceRules {
	return &declarativeresource.ResourceRules{}
}

// loadDeclarativeResources loads declarative layout resources from files.
// The dbStore parameter is optional (can be nil) and is used for duplicate checking in composite mode.
func loadDeclarativeResources(fileStore layoutMgtStoreInterface, dbStore layoutMgtStoreInterface) error {
	// Type assert to access Storer interface for resource loading
	fileBasedStore, ok := fileStore.(*layoutFileBasedStore)
	if !ok {
		return fmt.Errorf("failed to assert fileStore to *layoutFileBasedStore")
	}

	resourceConfig := declarativeresource.ResourceConfig{
		ResourceType:  "Layout",
		DirectoryName: "layouts",
		Parser:        parseToLayoutWrapper,
		Validator: func(data interface{}) error {
			return validateLayoutWrapper(data, dbStore)
		},
		IDExtractor: func(data interface{}) string {
			if layout, ok := data.(*Layout); ok {
				return layout.ID
			}
			return ""
		},
	}

	loader := declarativeresource.NewResourceLoader(resourceConfig, fileBasedStore)
	if err := loader.LoadResources(); err != nil {
		return fmt.Errorf("failed to load layout resources: %w", err)
	}

	return nil
}

// parseToLayoutWrapper wraps parseToLayout to match ResourceConfig.Parser signature.
func parseToLayoutWrapper(data []byte) (interface{}, error) {
	return parseToLayout(data)
}

// parseToLayout converts YAML data into a Layout object.
func parseToLayout(data []byte) (*Layout, error) {
	var layoutRequest layoutRequestWithID

	err := yaml.Unmarshal(data, &layoutRequest)
	if err != nil {
		return nil, err
	}

	// Convert layout to JSON bytes
	var layoutJSON json.RawMessage
	if layoutRequest.Layout != nil {
		// Handle both map structure and string format
		switch v := layoutRequest.Layout.(type) {
		case string:
			// JSON string format
			layoutJSON = []byte(v)
		default:
			// Map structure - marshal to JSON
			layoutBytes, err := json.Marshal(layoutRequest.Layout)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal layout to JSON: %w", err)
			}
			layoutJSON = layoutBytes
		}
	}

	layout := &Layout{
		ID:          layoutRequest.ID,
		DisplayName: layoutRequest.DisplayName,
		Description: layoutRequest.Description,
		Layout:      layoutJSON,
		CreatedAt:   "",
		UpdatedAt:   "",
	}

	return layout, nil
}

// validateLayoutWrapper wraps validateLayoutForDeclarativeResource to match ResourceConfig.Validator signature.
// It also checks for duplicates across database stores in composite mode.
func validateLayoutWrapper(dto interface{}, dbStore layoutMgtStoreInterface) error {
	layout, ok := dto.(*Layout)
	if !ok {
		return fmt.Errorf("invalid type: expected *Layout")
	}

	// Basic validation
	if err := validateLayoutForDeclarativeResource(layout); err != nil {
		return err
	}

	// In composite mode, check for duplicates in database store
	if dbStore != nil {
		exists, err := dbStore.IsLayoutExist(layout.ID)
		if err != nil {
			return fmt.Errorf("failed to check for duplicate layout ID '%s': %w", layout.ID, err)
		}
		if exists {
			return fmt.Errorf("layout with ID '%s' already exists in database", layout.ID)
		}
	}

	return nil
}

// validateLayoutForDeclarativeResource validates a layout for declarative resource loading.
func validateLayoutForDeclarativeResource(layout *Layout) error {
	if strings.TrimSpace(layout.DisplayName) == "" {
		return fmt.Errorf("layout display name is required")
	}

	if strings.TrimSpace(layout.ID) == "" {
		return fmt.Errorf("layout ID is required")
	}

	if len(layout.Layout) == 0 {
		return fmt.Errorf("layout configuration is required for '%s'", layout.DisplayName)
	}

	// Validate that layout is valid JSON
	var layoutConfig interface{}
	if err := json.Unmarshal(layout.Layout, &layoutConfig); err != nil {
		return fmt.Errorf("invalid layout JSON for '%s': %w", layout.DisplayName, err)
	}

	return nil
}
