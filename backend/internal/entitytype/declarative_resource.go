// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entitytype

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/entitytype/model"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/log"

	"gopkg.in/yaml.v3"
)

const (
	resourceTypeEntityType = "user_type"
	paramTypEntityType     = "EntityType"
	resourceTypeAgentType  = "agent_type"
	paramTypeAgentType     = "AgentType"
)

// entityTypeExporter implements declarative_resource.ResourceExporter for entity types.
type entityTypeExporter struct {
	service  EntityTypeServiceInterface
	category TypeCategory
}

// newEntityTypeExporter creates a new entity type exporter for the given category.
func newEntityTypeExporter(service EntityTypeServiceInterface, category TypeCategory) *entityTypeExporter {
	return &entityTypeExporter{service: service, category: category}
}

// NewEntityTypeExporterForTest creates a new entity type exporter for testing purposes.
func NewEntityTypeExporterForTest(service EntityTypeServiceInterface, category TypeCategory) *entityTypeExporter {
	return newEntityTypeExporter(service, category)
}

// GetResourceType returns the resource type for entity types.
func (e *entityTypeExporter) GetResourceType() string {
	if e.category == TypeCategoryAgent {
		return resourceTypeAgentType
	}
	return resourceTypeEntityType
}

// GetParameterizerType returns the parameterizer type for entity types.
func (e *entityTypeExporter) GetParameterizerType() string {
	if e.category == TypeCategoryAgent {
		return paramTypeAgentType
	}
	return paramTypEntityType
}

// GetAllResourceIDs retrieves all entity type IDs for the exporter's category.
// In composite mode, this excludes declarative (YAML-based) entity types.
//
// Agent types are included alongside user types: an agent document names its type, and importing it
// into a deployment that does not have that type fails schema validation. The exported document
// carries its category, so the two round-trip through the one resource type.
func (e *entityTypeExporter) GetAllResourceIDs(ctx context.Context) ([]string, *tidcommon.ServiceError) {
	offset := 0
	limit := serverconst.MaxPageSize
	ids := []string{}

	for {
		response, err := e.service.GetEntityTypeList(ctx, e.category, limit, offset, false)
		if err != nil {
			return nil, err
		}

		for _, schema := range response.Types {
			if !schema.IsReadOnly {
				ids = append(ids, schema.ID)
			}
		}

		offset += len(response.Types)
		if len(response.Types) == 0 {
			break
		}
	}

	return ids, nil
}

// GetResourceByID retrieves an entity type of the exporter's category by its ID.
func (e *entityTypeExporter) GetResourceByID(ctx context.Context, id string) (
	interface{}, string, *tidcommon.ServiceError,
) {
	schema, err := e.service.GetEntityType(ctx, e.category, id, false)
	if err != nil {
		return nil, "", err
	}
	return schema, schema.Name, nil
}

// ValidateResource validates a entity type resource.
func (e *entityTypeExporter) ValidateResource(ctx context.Context,
	resource interface{}, id string, logger *log.Logger,
) (string, *declarativeresource.ExportError) {
	schema, ok := resource.(*EntityType)
	if !ok {
		return "", declarativeresource.CreateTypeError(e.GetResourceType(), id)
	}

	err := declarativeresource.ValidateResourceName(ctx,
		schema.Name, e.GetResourceType(), id, "SCHEMA_VALIDATION_ERROR", logger,
	)
	if err != nil {
		return "", err
	}

	if len(schema.Schema) == 0 {
		logger.Warn(ctx, "Entity type has no schema definition",
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
		return fmt.Errorf("ouId or ouHandle is required for entity type '%s'", schemaDTO.Name)
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
