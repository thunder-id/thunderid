/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package entitytype

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/thunder-id/thunderid/internal/entitytype/model"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"

	"gopkg.in/yaml.v3"
)

const (
	resourceTypeEntityType = "user_type"
	paramTypEntityType     = "EntityType"
)

// entityTypeExporter implements declarative_resource.ResourceExporter for entity types.
type entityTypeExporter struct {
	service EntityTypeServiceInterface
}

// newEntityTypeExporter creates a new entity type exporter.
func newEntityTypeExporter(service EntityTypeServiceInterface) *entityTypeExporter {
	return &entityTypeExporter{service: service}
}

// NewEntityTypeExporterForTest creates a new entity type exporter for testing purposes.
func NewEntityTypeExporterForTest(service EntityTypeServiceInterface) *entityTypeExporter {
	return newEntityTypeExporter(service)
}

// GetResourceType returns the resource type for entity types.
func (e *entityTypeExporter) GetResourceType() string {
	return resourceTypeEntityType
}

// GetParameterizerType returns the parameterizer type for entity types.
func (e *entityTypeExporter) GetParameterizerType() string {
	return paramTypEntityType
}

// GetAllResourceIDs retrieves all user-category entity type IDs.
// In composite mode, this excludes declarative (YAML-based) entity types.
func (e *entityTypeExporter) GetAllResourceIDs(ctx context.Context) ([]string, *serviceerror.ServiceError) {
	response, err := e.service.GetEntityTypeList(ctx, TypeCategoryUser, serverconst.MaxPageSize, 0, false)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(response.Types))
	for _, schema := range response.Types {
		if !schema.IsReadOnly {
			ids = append(ids, schema.ID)
		}
	}
	return ids, nil
}

// GetResourceByID retrieves a user-category entity type by its ID.
func (e *entityTypeExporter) GetResourceByID(ctx context.Context, id string) (
	interface{}, string, *serviceerror.ServiceError,
) {
	schema, err := e.service.GetEntityType(ctx, TypeCategoryUser, id, false)
	if err != nil {
		return nil, "", err
	}
	return schema, schema.Name, nil
}

// ValidateResource validates a entity type resource.
func (e *entityTypeExporter) ValidateResource(
	resource interface{}, id string, logger *log.Logger,
) (string, *declarativeresource.ExportError) {
	schema, ok := resource.(*EntityType)
	if !ok {
		return "", declarativeresource.CreateTypeError(resourceTypeEntityType, id)
	}

	err := declarativeresource.ValidateResourceName(
		schema.Name, resourceTypeEntityType, id, "SCHEMA_VALIDATION_ERROR", logger,
	)
	if err != nil {
		return "", err
	}

	if len(schema.Schema) == 0 {
		logger.Warn("Entity type has no schema definition",
			log.String("schemaID", id), log.String("name", schema.Name))
	}

	return schema.Name, nil
}

// GetResourceRules returns the parameterization rules for entity types.
func (e *entityTypeExporter) GetResourceRules() *declarativeresource.ResourceRules {
	return &declarativeresource.ResourceRules{}
}

// loadDeclarativeResources loads declarative entity type resources from files.
// Works in both declarative-only and composite modes:
// - In declarative mode: entityTypeStore is a fileBasedStore
// - In composite mode: entityTypeStore is a compositeEntityTypeStore (contains both file and DB stores)
func loadDeclarativeResources(
	entityTypeStore entityTypeStoreInterface, service EntityTypeServiceInterface) error {
	var fileStore entityTypeStoreInterface

	// Determine store type and extract file store
	switch store := entityTypeStore.(type) {
	case *compositeEntityTypeStore:
		// Composite mode: extract file store from composite
		fileStore = store.fileStore
	case *entityTypeFileBasedStore:
		// Declarative-only mode: only file store available
		fileStore = store
	default:
		return fmt.Errorf("invalid store type for loading declarative resources")
	}

	// Type assert to access Storer interface for resource loading
	fileBasedStore, ok := fileStore.(*entityTypeFileBasedStore)
	if !ok {
		return fmt.Errorf("failed to assert entityTypeStore to *entityTypeFileBasedStore")
	}

	resourceConfig := declarativeresource.ResourceConfig{
		ResourceType:  "EntityType",
		DirectoryName: "user_types",
		Parser:        parseToEntityTypeDTOWrapper,
		Validator:     validateEntityTypeWrapper(service),
		IDExtractor: func(data interface{}) string {
			return data.(*EntityType).ID
		},
	}

	loader := declarativeresource.NewResourceLoader(resourceConfig, fileBasedStore)
	if err := loader.LoadResources(); err != nil {
		return fmt.Errorf("failed to load entity type resources: %w", err)
	}

	return nil
}

// parseToEntityTypeDTOWrapper wraps parseToEntityTypeDTO to match ResourceConfig.Parser signature.
func parseToEntityTypeDTOWrapper(data []byte) (interface{}, error) {
	return parseToEntityTypeDTO(data)
}

func parseToEntityTypeDTO(data []byte) (*EntityType, error) {
	var schemaRequest EntityTypeRequestWithID
	err := yaml.Unmarshal(data, &schemaRequest)
	if err != nil {
		return nil, err
	}

	var schemaBytes []byte
	if schemaRequest.Schema != nil {
		switch v := schemaRequest.Schema.(type) {
		case string:
			schemaBytes = []byte(v)
		default:
			var err error
			schemaBytes, err = json.Marshal(v)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal schema to JSON: %w", err)
			}
		}
	}
	if !json.Valid(schemaBytes) {
		return nil, fmt.Errorf("schema field contains invalid JSON")
	}

	category := schemaRequest.Category
	if category == "" {
		category = TypeCategoryUser
	}
	if !category.IsValid() {
		return nil, fmt.Errorf("invalid entity type category %q", string(category))
	}

	schemaDTO := &EntityType{
		ID:                    schemaRequest.ID,
		Category:              category,
		Name:                  schemaRequest.Name,
		OUID:                  schemaRequest.OUID,
		OUHandle:              schemaRequest.OUHandle,
		AllowSelfRegistration: schemaRequest.AllowSelfRegistration,
		SystemAttributes:      schemaRequest.SystemAttributes,
		Schema:                schemaBytes,
	}

	return schemaDTO, nil
}

// validateEntityTypeWrapper wraps validateEntityType to match ResourceConfig.Validator signature.
// When a service is provided, OU handles are resolved before validation runs.
func validateEntityTypeWrapper(service EntityTypeServiceInterface) func(interface{}) error {
	return func(dto interface{}) error {
		schemaDTO, ok := dto.(*EntityType)
		if !ok {
			return fmt.Errorf("invalid type: expected *EntityType")
		}
		if service != nil {
			if svcErr := service.ResolveEntityTypeHandles(context.Background(), schemaDTO); svcErr != nil {
				return fmt.Errorf("organization unit with handle %q not found for entity type '%s'",
					schemaDTO.OUHandle, schemaDTO.Name)
			}
		}
		return validateEntityType(schemaDTO)
	}
}

func validateEntityType(schemaDTO *EntityType) error {
	if strings.TrimSpace(schemaDTO.Name) == "" {
		return fmt.Errorf("entity type name is required")
	}

	if strings.TrimSpace(schemaDTO.ID) == "" {
		return fmt.Errorf("entity type ID is required")
	}

	if strings.TrimSpace(schemaDTO.OUID) == "" {
		return fmt.Errorf("organization_unit_id or ou_handle is required for entity type '%s'", schemaDTO.Name)
	}

	// Validate schema definition is present and valid.
	if len(schemaDTO.Schema) == 0 {
		return fmt.Errorf("schema definition is required for entity type '%s'", schemaDTO.Name)
	}

	compiledSchema, compileErr := model.CompileSchema(schemaDTO.Schema)
	if compileErr != nil {
		return fmt.Errorf("invalid schema for entity type '%s': %w", schemaDTO.Name, compileErr)
	}

	if svcErr := validateSystemAttributes(compiledSchema, schemaDTO.SystemAttributes); svcErr != nil {
		return fmt.Errorf("invalid system attributes for entity type '%s': %s",
			schemaDTO.Name, svcErr.ErrorDescription)
	}

	return nil
}
