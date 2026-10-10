// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entitytype

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/entitytype/model"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"

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
	return schema, schema.DisplayName, nil
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
		schema.DisplayName, e.GetResourceType(), id, "SCHEMA_VALIDATION_ERROR", logger,
	)
	if err != nil {
		return "", err
	}

	if len(schema.Schema) == 0 {
		logger.Warn(ctx, "Entity type has no schema definition",
			log.String("schemaID", id), log.String("handle", schema.Handle))
	}

	return schema.DisplayName, nil
}

// ViewResource shows an exported entity type as a read of it returns it. The export writes the
// schema as a JSON string, which the read returns as the JSON itself.
func (e *entityTypeExporter) ViewResource(_ context.Context, document *yaml.Node) (interface{}, error) {
	return declarativeresource.DecodeView(document, func(exported *EntityTypeRequestWithID) (
		interface{}, error) {
		var schema json.RawMessage
		var err error
		switch v := exported.Schema.(type) {
		case nil:
		case string:
			schema = json.RawMessage(v)
		default:
			schema, err = json.Marshal(v)
		}
		if schema != nil && !json.Valid(schema) {
			return nil, fmt.Errorf("schema field contains invalid JSON")
		}

		return &EntityType{
			ID:                    exported.ID,
			Category:              e.category,
			Handle:                exported.Handle,
			DisplayName:           exported.DisplayName,
			OUID:                  exported.OUID,
			AllowSelfRegistration: exported.AllowSelfRegistration,
			SystemAttributes:      exported.SystemAttributes,
			Schema:                schema,
		}, err
	})
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
	var dbStore entityTypeStoreInterface

	// Determine store type and extract file store
	switch store := entityTypeStore.(type) {
	case *compositeEntityTypeStore:
		// Composite mode: extract file and DB stores from composite
		fileStore = store.fileStore
		dbStore = store.dbStore
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

	return validateUniqueDeclarativeHandles(fileBasedStore, dbStore)
}

// validateUniqueDeclarativeHandles ensures no two declarative entity types of the same category share
// a handle. In composite mode, it also ensures the handle is not used by an entity type in the DB store.
func validateUniqueDeclarativeHandles(
	fileBasedStore *entityTypeFileBasedStore, dbStore entityTypeStoreInterface) error {
	list, err := fileBasedStore.GenericFileBasedStore.List()
	if err != nil {
		return fmt.Errorf("failed to list entity type resources: %w", err)
	}

	seen := make(map[string]string, len(list))
	for _, item := range list {
		entityType, ok := item.Data.(*EntityType)
		if !ok {
			continue
		}
		key := string(entityType.Category) + ":" + entityType.Handle
		if existingID, exists := seen[key]; exists {
			return fmt.Errorf("duplicate entity type handle %q in declarative resources (ids %s and %s)",
				entityType.Handle, existingID, entityType.ID)
		}
		seen[key] = entityType.ID

		if dbStore == nil {
			continue
		}
		_, err := dbStore.GetEntityTypeByHandle(context.Background(), entityType.Category, entityType.Handle)
		if err == nil {
			return fmt.Errorf("duplicate entity type handle %q: handle already used in the database store",
				entityType.Handle)
		}
		if !errors.Is(err, ErrEntityTypeNotFound) {
			return fmt.Errorf("failed to check for duplicate entity type handle %q: %w", entityType.Handle, err)
		}
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
		Handle:                schemaRequest.Handle,
		DisplayName:           schemaRequest.DisplayName,
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
					schemaDTO.OUHandle, schemaDTO.Handle)
			}
		}
		return validateEntityType(schemaDTO)
	}
}

func validateEntityType(schemaDTO *EntityType) error {
	if !utils.IsValidHandle(schemaDTO.Handle) {
		return fmt.Errorf("entity type handle %q is invalid: it must contain only lowercase letters, "+
			"numbers, hyphens and underscores, and start and end with a letter or a number", schemaDTO.Handle)
	}

	if strings.TrimSpace(schemaDTO.DisplayName) == "" {
		return fmt.Errorf("display name is required for entity type '%s'", schemaDTO.Handle)
	}

	if strings.TrimSpace(schemaDTO.ID) == "" {
		return fmt.Errorf("entity type ID is required")
	}

	if strings.TrimSpace(schemaDTO.OUID) == "" {
		return fmt.Errorf("ouId or ouHandle is required for entity type '%s'", schemaDTO.Handle)
	}

	// Validate schema definition is present and valid.
	if len(schemaDTO.Schema) == 0 {
		return fmt.Errorf("schema definition is required for entity type '%s'", schemaDTO.Handle)
	}

	compiledSchema, compileErr := model.CompileSchema(schemaDTO.Schema)
	if compileErr != nil {
		return fmt.Errorf("invalid schema for entity type '%s': %w", schemaDTO.Handle, compileErr)
	}

	if svcErr := validateSystemAttributes(compiledSchema, schemaDTO.SystemAttributes); svcErr != nil {
		return fmt.Errorf("invalid system attributes for entity type '%s': %s",
			schemaDTO.Handle, svcErr.ErrorDescription)
	}

	return nil
}
