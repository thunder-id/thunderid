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

package definition

import (
	"context"
	"fmt"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/log"

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
	DisplayName          string              `yaml:"displayName"`
	VCT                  string              `yaml:"vct"`
	Format               string              `yaml:"format"`
	RequestedClaims      []string            `yaml:"requestedClaims"`
	MandatoryClaims      []string            `yaml:"mandatoryClaims"`
	OptionalClaims       []string            `yaml:"optionalClaims"`
	ClaimValues          map[string][]string `yaml:"claimValues"`
	EnforceTrustedIssuer *bool               `yaml:"enforceTrustedIssuer"`
	TrustedAuthorities   []string            `yaml:"trustedAuthorities"`
}

// loadDeclarativeResources loads declarative presentation-definition resources from files.
func loadDeclarativeResources(store declarativeresource.Storer) error {
	resourceConfig := declarativeresource.ResourceConfig{
		ResourceType:  "PresentationDefinition",
		DirectoryName: "presentation_definitions",
		Parser:        parseToDefinitionDTOWrapper,
		Validator:     validateDefinitionWrapper,
		IDExtractor: func(dto interface{}) string {
			return dto.(*PresentationDefinitionDTO).ID
		},
	}

	loader := declarativeresource.NewResourceLoader(resourceConfig, store)
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
		DisplayName:          req.DisplayName,
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

// validateDefinitionWrapper wraps validateDefinition to match ResourceConfig.Validator signature.
func validateDefinitionWrapper(dto interface{}) error {
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
	return nil
}
