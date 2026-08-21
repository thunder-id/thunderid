// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package presentation

import (
	"context"
	"errors"
	"fmt"

	"github.com/thunder-id/thunderid/internal/ou"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"gopkg.in/yaml.v3"
)

const (
	resourceTypePresentationDefinition = "presentation_definition"
	paramTypePresentationDefinition    = "PresentationDefinition"
)

// definitionExporter implements declarativeresource.ResourceExporter for
// OpenID4VP presentation definitions, reading them through the service.
type definitionExporter struct {
	service PresentationDefinitionServiceInterface
}

// newDefinitionExporter creates a new presentation-definition exporter.
func newDefinitionExporter(service PresentationDefinitionServiceInterface) *definitionExporter {
	return &definitionExporter{service: service}
}

// GetResourceType returns the resource type identifier for presentation definitions.
func (e *definitionExporter) GetResourceType() string {
	return resourceTypePresentationDefinition
}

// GetParameterizerType returns the parameterizer type name for presentation definitions.
func (e *definitionExporter) GetParameterizerType() string {
	return paramTypePresentationDefinition
}

// GetAllResourceIDs returns the IDs of all mutable (database-backed) presentation
// definitions, excluding any declarative (file-based) definitions.
func (e *definitionExporter) GetAllResourceIDs(ctx context.Context) ([]string, *tidcommon.ServiceError) {
	defs, err := e.service.ListPresentationDefinitions(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(defs))
	for _, dto := range defs {
		isDeclarative, svcErr := e.service.IsPresentationDefinitionDeclarative(ctx, dto.ID)
		if svcErr != nil {
			return nil, svcErr
		}
		if !isDeclarative {
			ids = append(ids, dto.ID)
		}
	}
	return ids, nil
}

// GetResourceByID retrieves a presentation definition by its ID for export.
// The handle is the stable identifier and is returned as the resource name.
func (e *definitionExporter) GetResourceByID(ctx context.Context, id string) (
	interface{}, string, *tidcommon.ServiceError,
) {
	dto, err := e.service.GetPresentationDefinition(ctx, id)
	if err != nil {
		return nil, "", err
	}
	dto.OUHandle = ""
	return dto, dto.Handle, nil
}

// ValidateResource validates a presentation definition resource prior to export,
// extracting its handle as the stable resource name.
func (e *definitionExporter) ValidateResource(ctx context.Context,
	resource interface{}, id string, logger *log.Logger) (string, *declarativeresource.ExportError) {
	dto, ok := resource.(*PresentationDefinitionDTO)
	if !ok {
		return "", declarativeresource.CreateTypeError(resourceTypePresentationDefinition, id)
	}

	if err := declarativeresource.ValidateResourceName(ctx,
		dto.Handle, resourceTypePresentationDefinition, id, "VP_DEFINITION_VALIDATION_ERROR", logger); err != nil {
		return "", err
	}

	return dto.Handle, nil
}

// GetResourceRules returns the parameterization rules for presentation definitions.
// The claim/value constraint blobs (ClaimValues) and the trusted-authority list are
// the free-form, deployment-specific fields treated as dynamic property fields and
// array variables respectively.
func (e *definitionExporter) GetResourceRules() *declarativeresource.ResourceRules {
	return &declarativeresource.ResourceRules{
		ArrayVariables:        []string{"TrustedAuthorities"},
		DynamicPropertyFields: []string{"ClaimValues"},
	}
}

// definitionRequestWithID is the YAML shape of a declarative presentation
// definition: the management request body plus the stable resource ID. The
// claim sets, claim/value constraints and trust fields mirror the JSON request.
type definitionRequestWithID struct {
	ID                   string              `yaml:"id"`
	Handle               string              `yaml:"handle"`
	OUID                 string              `yaml:"ouId"`
	OUHandle             string              `yaml:"ouHandle"`
	Name                 string              `yaml:"name"`
	Description          string              `yaml:"description"`
	VCT                  string              `yaml:"vct"`
	Format               string              `yaml:"format"`
	RequestedClaims      []string            `yaml:"requestedClaims"`
	MandatoryClaims      []string            `yaml:"mandatoryClaims"`
	OptionalClaims       []string            `yaml:"optionalClaims"`
	ClaimValues          map[string][]string `yaml:"claimValues"`
	EnforceTrustedIssuer *bool               `yaml:"enforceTrustedIssuer"`
	TrustedAuthorities   []string            `yaml:"trustedAuthorities"`
}

// loadDeclarativeResources loads declarative presentation-definition resources from YAML files
// into the file store. The dbStore parameter is optional and is used only for duplicate checking
// in composite mode. The ouService parameter is optional and is used to resolve ouHandle to ouId.
func loadDeclarativeResources(
	fileStore *definitionFileBasedStore, dbStore definitionStoreInterface,
	ouService ou.OrganizationUnitServiceInterface,
) error {
	resourceConfig := declarativeresource.ResourceConfig{
		ResourceType:  paramTypePresentationDefinition,
		DirectoryName: "presentation_definitions",
		Parser:        parseToDefinitionDTOWrapper,
		Validator: func(dto interface{}) error {
			return validateDefinitionWrapper(dto, fileStore, dbStore, ouService)
		},
		IDExtractor: func(dto interface{}) string {
			return dto.(*PresentationDefinitionDTO).ID
		},
	}

	loader := declarativeresource.NewResourceLoader(resourceConfig, &definitionStorer{store: fileStore})
	if err := loader.LoadResources(); err != nil {
		return fmt.Errorf("failed to load presentation definition resources: %w", err)
	}

	return nil
}

// parseToDefinitionDTOWrapper wraps parseToDefinitionDTO to match the expected signature.
func parseToDefinitionDTOWrapper(data []byte) (interface{}, error) {
	return parseToDefinitionDTO(data)
}

// parseToDefinitionDTO unmarshals YAML data into a presentation definition DTO.
func parseToDefinitionDTO(data []byte) (*PresentationDefinitionDTO, error) {
	var req definitionRequestWithID
	if err := yaml.Unmarshal(data, &req); err != nil {
		return nil, err
	}
	return buildDefinitionDTOFromRequest(req), nil
}

// buildDefinitionDTOFromRequest maps a parsed YAML request to a managed DTO,
// applying the same field mapping the management API uses in requestToDTO.
func buildDefinitionDTOFromRequest(req definitionRequestWithID) *PresentationDefinitionDTO {
	return &PresentationDefinitionDTO{
		ID:                   req.ID,
		Handle:               req.Handle,
		OUID:                 req.OUID,
		OUHandle:             req.OUHandle,
		Name:                 req.Name,
		Description:          req.Description,
		VCT:                  req.VCT,
		Format:               req.Format,
		RequestedClaims:      req.RequestedClaims,
		MandatoryClaims:      req.MandatoryClaims,
		OptionalClaims:       req.OptionalClaims,
		ClaimValues:          req.ClaimValues,
		EnforceTrustedIssuer: req.EnforceTrustedIssuer,
		TrustedAuthorities:   req.TrustedAuthorities,
	}
}

// validateDefinitionWrapper validates a declarative presentation definition: it must carry an ID,
// pass the same field validation the management API applies, resolve to an existing organization
// unit, and not reuse an ID or handle already claimed by another file.
func validateDefinitionWrapper(
	dto interface{}, fileStore *definitionFileBasedStore, dbStore definitionStoreInterface,
	ouService ou.OrganizationUnitServiceInterface,
) error {
	def, ok := dto.(*PresentationDefinitionDTO)
	if !ok {
		return fmt.Errorf("invalid type: expected *PresentationDefinitionDTO")
	}
	if def.ID == "" {
		return fmt.Errorf("presentation definition ID is required")
	}
	if svcErr := validateDefinition(def); svcErr != nil {
		return fmt.Errorf("validation failed: %s", svcErr.Error.DefaultValue)
	}
	if err := resolveDefinitionOU(context.Background(), def, ouService); err != nil {
		return err
	}
	return checkDuplicateDefinition(context.Background(), def, fileStore, dbStore)
}

// resolveDefinitionOU resolves ouHandle to ouId and requires the result to be non-empty, so a
// declarative definition carries the same owning organization unit the management API demands.
func resolveDefinitionOU(
	ctx context.Context, def *PresentationDefinitionDTO, ouService ou.OrganizationUnitServiceInterface,
) error {
	if ouService != nil && def.OUID == "" && def.OUHandle != "" {
		resolved, svcErr := ouService.GetOrganizationUnitByPath(ctx, def.OUHandle)
		if svcErr != nil {
			return fmt.Errorf("organization unit with handle %q not found for presentation definition '%s'",
				def.OUHandle, def.Handle)
		}
		def.OUID = resolved.ID
	}
	if def.OUID == "" {
		return fmt.Errorf("ouId or ouHandle is required for presentation definition '%s'", def.Handle)
	}
	return nil
}

// checkDuplicateDefinition rejects a definition whose ID or handle another declarative file already
// claimed, which would otherwise silently overwrite or shadow the earlier one.
func checkDuplicateDefinition(
	ctx context.Context, def *PresentationDefinitionDTO,
	fileStore *definitionFileBasedStore, dbStore definitionStoreInterface,
) error {
	if fileStore != nil {
		if _, err := fileStore.GetPresentationDefinitionByID(ctx, def.ID); err == nil {
			return fmt.Errorf(
				"duplicate presentation definition ID '%s': definition already exists in declarative resources",
				def.ID)
		}
		if _, err := fileStore.GetPresentationDefinitionByHandle(ctx, def.Handle); err == nil {
			return fmt.Errorf(
				"duplicate presentation definition handle '%s': handle already used in declarative resources",
				def.Handle)
		}
	}
	if dbStore != nil {
		_, err := dbStore.GetPresentationDefinitionByID(ctx, def.ID)
		if err == nil {
			return fmt.Errorf(
				"duplicate presentation definition ID '%s': definition already exists in the database store",
				def.ID)
		} else if !errors.Is(err, ErrNotFound) {
			return fmt.Errorf("failed to check for duplicate presentation definition ID '%s': %w", def.ID, err)
		}
	}
	return nil
}
