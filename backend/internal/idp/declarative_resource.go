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

package idp

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"

	"gopkg.in/yaml.v3"
)

const (
	resourceTypeIdentityProvider = "identity_provider"
	paramTypIdentityProvider     = "IdentityProvider"
)

// idpExporter implements declarativeresource.ResourceExporter for identity providers.
type idpExporter struct {
	service IDPServiceInterface
}

// newIDPExporter creates a new IDP exporter.
func newIDPExporter(service IDPServiceInterface) *idpExporter {
	return &idpExporter{service: service}
}

// NewIDPExporterForTest creates a new IDP exporter for testing purposes.
func NewIDPExporterForTest(service IDPServiceInterface) *idpExporter {
	if !testing.Testing() {
		panic("only for tests!")
	}
	return newIDPExporter(service)
}

// GetResourceType returns the resource type for identity providers.
func (e *idpExporter) GetResourceType() string {
	return resourceTypeIdentityProvider
}

// GetParameterizerType returns the parameterizer type for identity providers.
func (e *idpExporter) GetParameterizerType() string {
	return paramTypIdentityProvider
}

// GetAllResourceIDs retrieves all identity provider IDs.
func (e *idpExporter) GetAllResourceIDs(ctx context.Context) ([]string, *serviceerror.ServiceError) {
	idps, err := e.service.GetIdentityProviderList(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(idps))
	for _, idp := range idps {
		ids = append(ids, idp.ID)
	}
	return ids, nil
}

// GetResourceByID retrieves an identity provider by its ID.
func (e *idpExporter) GetResourceByID(ctx context.Context, id string) (
	interface{}, string, *serviceerror.ServiceError,
) {
	idpDTO, err := e.service.GetIdentityProvider(ctx, id)
	if err != nil {
		return nil, "", err
	}
	return idpDTO, idpDTO.Name, nil
}

// ValidateResource validates an identity provider resource.
func (e *idpExporter) ValidateResource(
	resource interface{}, id string, logger *log.Logger) (string, *declarativeresource.ExportError) {
	idpDTO, ok := resource.(*IDPDTO)
	if !ok {
		return "", declarativeresource.CreateTypeError(resourceTypeIdentityProvider, id)
	}

	err := declarativeresource.ValidateResourceName(
		idpDTO.Name, resourceTypeIdentityProvider, id, "IDP_VALIDATION_ERROR", logger,
	)
	if err != nil {
		return "", err
	}

	if len(idpDTO.Properties) == 0 {
		logger.Warn("Identity provider has no properties",
			log.String("idpID", id), log.String("name", idpDTO.Name))
	}

	return idpDTO.Name, nil
}

// GetResourceRules returns the parameterization rules for identity providers.
func (e *idpExporter) GetResourceRules() *declarativeresource.ResourceRules {
	return &declarativeresource.ResourceRules{
		DynamicPropertyFields: []string{"Properties"},
	}
}

// loadDeclarativeResources loads declarative identity provider resources from files.
func loadDeclarativeResources(idpStore idpStoreInterface) error {
	// Create a storer wrapper since idpStore interface doesn't expose Create directly
	var storer declarativeresource.Storer
	if fileBasedStore, ok := idpStore.(*idpFileBasedStore); ok {
		storer = fileBasedStore
	} else {
		return fmt.Errorf("invalid store type for declarative resources")
	}

	resourceConfig := declarativeresource.ResourceConfig{
		ResourceType:  "IdentityProvider",
		DirectoryName: "identity_providers",
		Parser:        parseToIDPDTOWrapper,
		Validator:     validateIDPWrapper,
		IDExtractor: func(dto interface{}) string {
			return dto.(*IDPDTO).ID
		},
	}

	loader := declarativeresource.NewResourceLoader(resourceConfig, storer)
	if err := loader.LoadResources(); err != nil {
		return fmt.Errorf("failed to load identity provider resources: %w", err)
	}

	return nil
}

// parseToIDPDTOWrapper wraps parseToIDPDTO to match the expected signature.
func parseToIDPDTOWrapper(data []byte) (interface{}, error) {
	return parseToIDPDTO(data)
}

func parseToIDPDTO(data []byte) (*IDPDTO, error) {
	var idpRequest idpRequestWithID
	err := yaml.Unmarshal(data, &idpRequest)
	if err != nil {
		return nil, err
	}

	idpDTO := &IDPDTO{
		ID:          idpRequest.ID,
		Name:        idpRequest.Name,
		Description: idpRequest.Description,
	}

	// Parse IDP type
	idpType, err := parseIDPType(idpRequest.Type)
	if err != nil {
		return nil, err
	}
	idpDTO.Type = idpType

	// Convert PropertyDTO to Property
	if len(idpRequest.Properties) > 0 {
		properties := make([]cmodels.Property, 0, len(idpRequest.Properties))
		for _, propDTO := range idpRequest.Properties {
			prop, err := cmodels.NewProperty(propDTO.Name, propDTO.Value, propDTO.IsSecret)
			if err != nil {
				return nil, err
			}
			properties = append(properties, *prop)
		}
		idpDTO.Properties = properties
	}

	return idpDTO, nil
}

func parseIDPType(typeStr string) (IDPType, error) {
	// Convert string to uppercase for case-insensitive matching
	typeStrUpper := IDPType(strings.ToUpper(typeStr))

	// Check if it's a valid type
	for _, supportedType := range supportedIDPTypes {
		if supportedType == typeStrUpper {
			return supportedType, nil
		}
	}

	return "", fmt.Errorf("unsupported IDP type: %s", typeStr)
}

// validateIDPWrapper wraps validateIDP to match ResourceConfig.Validator signature.
func validateIDPWrapper(dto interface{}) error {
	idpDTO, ok := dto.(*IDPDTO)
	if !ok {
		return fmt.Errorf("invalid type: expected *IDPDTO")
	}

	// Use the full validateIDP function which validates properties and applies defaults
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "IDPDeclarativeResource"))
	svcErr := validateIDP(idpDTO, logger)
	if svcErr != nil {
		return fmt.Errorf("validation failed: %s", svcErr.Error)
	}

	return nil
}
